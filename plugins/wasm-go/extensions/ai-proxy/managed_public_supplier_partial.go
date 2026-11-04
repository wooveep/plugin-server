package main

import (
	"bytes"
	"encoding/json"

	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

// Native prefix conversion shares the existing supplier codec, but never its
// success observation. Original facts were retained before this encoder runs.
func publicSupplierPartialStream(ctx wrapper.HttpContext, s *managedExecutionState, headers [][2]string, original []byte, format string) (managedProtocolClientResponse, uint32, bool, error) {
	normalized := bytes.ReplaceAll(original, []byte("\r\n"), []byte("\n"))
	last := bytes.LastIndex(normalized, []byte("\n\n"))
	if last < 0 {
		return managedProtocolClientResponse{}, 0, false, nil
	}
	events, err := clientresponse.Events(normalized[:last+2])
	if err != nil {
		return managedProtocolClientResponse{}, 0, false, nil
	}
	failure := clientresponse.Failure{Kind: clientresponse.FailureIncomplete}
	var prefix bytes.Buffer
	terminal, failed := false, false
	for _, event := range events {
		if bytes.Equal(event.Data, []byte("[DONE]")) {
			continue
		}
		object := gjson.ParseBytes(event.Data)
		if object.Get("promptFeedback.blockReason").String() != "" {
			policy := managedPublicPolicy(s)
			f := clientresponse.Failure{Kind: clientresponse.FailurePolicy}
			return managedProtocolClientResponse{Body: f.Body(policy), Headers: f.Headers(policy, headers, false, false)}, uint32(f.Status(policy.Protocol)), true, nil
		}
		if event.Name == "error" || object.Get("error").IsObject() || object.Get("code").Exists() && object.Get("message").Exists() && !object.Get("output").Exists() {
			failure = clientresponse.ClassifyFailure(clientresponse.FailureInput{Body: event.Data, Format: format, Client: s.config.profile.InputProtocol, Request: s.publicRequest})
			failed = true
			break
		}
		var finish string
		if format == "gemini" {
			finish = object.Get("candidates.0.finishReason").String()
		} else {
			finish = object.Get("output.choices.0.finish_reason").String()
			if finish == "" {
				finish = object.Get("output.finish_reason").String()
			}
		}
		if finish != "" && finish != "null" {
			terminal = true
		}
		clientresponse.AppendEvent(&prefix, clientresponse.Event{Name: event.Name, Data: event.Data})
	}
	if terminal && !failed && last+2 == len(normalized) {
		return managedProtocolClientResponse{}, 0, false, nil
	}
	policy := managedPublicPolicy(s)
	if prefix.Len() == 0 {
		return managedProtocolClientResponse{Body: failure.Body(policy), Headers: failure.Headers(policy, headers, false, false)}, uint32(failure.Status(policy.Protocol)), true, nil
	}
	field := "usage"
	if format == "gemini" {
		field = "usageMetadata"
	}
	conversion, err := managedSupplierConversionInput(prefix.Bytes(), true, field)
	if err != nil {
		return managedProtocolClientResponse{}, 0, true, err
	}
	chat, err := s.config.vendor.ConvertNativeResponse(ctx, conversion, true)
	if err != nil {
		return managedProtocolClientResponse{}, 0, true, err
	}
	// The native codec normally appends DONE after a verified terminal. Here
	// the Chat partial encoder strips all terminal claims and retains an error.
	var output bytes.Buffer
	output.Write(chat)
	clientresponse.AppendEvent(&output, clientresponse.Event{Name: "error", Data: failure.Body(policy)})
	state, configuration := *s, *s.config
	configuration.vendor = nil
	configuration.profile.UpstreamProtocol = clientresponse.Chat
	state.config = &configuration
	usage := clientresponse.ReadUsage(original, format, true)
	if format == "gemini" {
		// Reuse the existing exact native-counter projection. This does not
		// consume converted default zeros or modify the original observation.
		var metadata json.RawMessage
		for _, event := range events {
			if value := gjson.GetBytes(event.Data, "usageMetadata"); value.IsObject() {
				metadata = json.RawMessage(value.Raw)
			}
		}
		wire := managedGoogleUsage(metadata)
		if thoughts := gjson.GetBytes(metadata, "thoughtsTokenCount"); thoughts.Exists() {
			wire["completion_tokens_details"], _ = json.Marshal(map[string]json.RawMessage{"reasoning_tokens": json.RawMessage(thoughts.Raw)})
		}
		encoded, _ := json.Marshal(map[string]interface{}{"usage": wire})
		usage = clientresponse.ReadUsage(encoded, clientresponse.Chat, false)
	}
	response, status, handled, err := encodePublicPartialResponse(ctx, &state, headers, output.Bytes(), &usage)
	s.publicPartial = state.publicPartial
	return response, status, handled, err
}
