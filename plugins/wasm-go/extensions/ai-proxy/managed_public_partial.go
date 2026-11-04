package main

import (
	"bytes"
	"encoding/json"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/provider"
	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// publicPartialResponse is a content encoder only. It is called after the
// original attempt has been recorded and existing recovery has finished.
// In particular, a content prefix never becomes new procurement evidence.
func publicPartialResponse(ctx wrapper.HttpContext, s *managedExecutionState, headers [][2]string, original []byte) (managedProtocolClientResponse, uint32, bool, error) {
	return encodePublicPartialResponse(ctx, s, headers, original, nil)
}

func encodePublicPartialResponse(ctx wrapper.HttpContext, s *managedExecutionState, headers [][2]string, original []byte, originalUsage *clientresponse.Usage) (managedProtocolClientResponse, uint32, bool, error) {
	if !managedPublicText(s) {
		return managedProtocolClientResponse{}, 0, false, nil
	}
	policy := managedPublicPolicy(s)
	format := s.config.profile.UpstreamProtocol
	if s.config.vendor != nil && s.config.vendor.NativeResponseFormat() != "" {
		format = s.config.vendor.NativeResponseFormat()
	}
	stream, err := managedProtocolContentType(headers)
	if err != nil {
		return managedProtocolClientResponse{}, 0, false, nil
	}
	if !stream && format != clientresponse.Responses && (gjson.GetBytes(original, "error").IsObject() || gjson.GetBytes(original, "code").Exists() && gjson.GetBytes(original, "message").Exists() && !gjson.GetBytes(original, "output").Exists()) {
		f := clientresponse.ClassifyFailure(clientresponse.FailureInput{Status: 200, Format: format, Client: policy.Protocol, Body: original, Request: s.publicRequest})
		return managedProtocolClientResponse{Body: f.Body(policy), Headers: f.Headers(policy, headers, false, false)}, uint32(f.Status(policy.Protocol)), true, nil
	}
	if format == "gemini" && !stream && gjson.GetBytes(original, "promptFeedback.blockReason").String() != "" {
		f := clientresponse.Failure{Kind: clientresponse.FailurePolicy}
		return managedProtocolClientResponse{Body: f.Body(policy), Headers: f.Headers(policy, headers, false, false)}, uint32(f.Status(policy.Protocol)), true, nil
	}
	if format != clientresponse.Chat {
		if stream && (format == "qwen" || format == "gemini") {
			return publicSupplierPartialStream(ctx, s, headers, original, format)
		}
		if stream && (format == clientresponse.Messages || format == clientresponse.Responses) && policy.Protocol == format {
			return publicNativeIncompleteStream(s, headers, original)
		}
		return managedProtocolClientResponse{}, 0, false, nil
	}
	usage := clientresponse.ReadUsage(original, format, stream)
	if originalUsage != nil {
		usage = *originalUsage
	}
	failure := clientresponse.Failure{Kind: clientresponse.FailureIncomplete}
	var body []byte
	status := uint32(200)
	if !stream {
		finish := gjson.GetBytes(original, "choices.0.finish_reason").String()
		switch finish {
		case "aborted":
			failure.Kind = clientresponse.FailureInterrupted
		case "insufficient_system_resource":
			failure.Kind = clientresponse.FailureOverloaded
		case "stop", "length", "content_filter", "tool_calls", "function_call":
			return managedProtocolClientResponse{}, 0, false, nil
		default:
			if !gjson.GetBytes(original, "choices.0.message").IsObject() {
				return managedProtocolClientResponse{}, 0, false, nil
			}
			failure.Kind = clientresponse.FailureInvalid
		}
		if policy.Protocol == clientresponse.Chat && (finish == "aborted" || finish == "insufficient_system_resource") {
			return managedProtocolClientResponse{}, 0, false, nil
		}
		conversion, e := managedProtocolConversionInput(original, false)
		if e != nil {
			return managedProtocolClientResponse{}, 0, true, e
		}
		if policy.Protocol == clientresponse.Responses {
			body, err = provider.ConvertChatCompletionResponseToResponses(conversion)
			if err == nil {
				body, _ = sjson.SetBytes(body, "status", "failed")
				body, _ = sjson.SetRawBytes(body, "error", failure.ResourceError(policy.Protocol))
				// Every item remains incomplete, including partial function arguments.
				for i := range gjson.GetBytes(body, "output").Array() {
					body, _ = sjson.SetBytes(body, "output."+jsonIndex(i)+".status", "incomplete")
				}
			}
		} else {
			var partial []byte
			if policy.Protocol == clientresponse.Messages {
				partial, err = (&provider.ClaudeToOpenAIConverter{}).ConvertOpenAIResponseToClaude(ctx, conversion)
				if err == nil {
					partial, _ = sjson.SetBytes(partial, "stop_reason", nil)
				}
			} else {
				partial = conversion
			}
			if err == nil {
				partial, err = clientresponse.Project(policy, partial, false, usage)
			}
			if err == nil {
				s.publicPartial = partial
				body = failure.Body(policy)
				status = uint32(failure.Status(policy.Protocol))
			}
		}
	} else {
		// Parse only complete frames. A trailing cut-off frame is never silently
		// treated as a terminal; the already complete prefix remains useful.
		normalized := bytes.ReplaceAll(original, []byte("\r\n"), []byte("\n"))
		last := bytes.LastIndex(normalized, []byte("\n\n"))
		if last < 0 {
			return managedProtocolClientResponse{}, 0, false, nil
		}
		events, e := clientresponse.Events(normalized[:last+2])
		if e != nil {
			return managedProtocolClientResponse{}, 0, true, e
		}
		var prefix bytes.Buffer
		terminal, done, failed := false, false, false
		for _, event := range events {
			if bytes.Equal(event.Data, []byte("[DONE]")) {
				done = true
				continue
			}
			object := gjson.ParseBytes(event.Data)
			if event.Name == "error" || object.Get("error").IsObject() || object.Get("type").String() == "error" {
				failure = clientresponse.ClassifyFailure(clientresponse.FailureInput{Body: event.Data, Client: policy.Protocol, Request: s.publicRequest})
				failed = true
				break
			}
			for _, choice := range object.Get("choices").Array() {
				finish := choice.Get("finish_reason").String()
				if finish != "" {
					terminal = true
					switch finish {
					case "stop", "length", "tool_calls", "function_call", "content_filter":
					case "aborted":
						failure.Kind, failed = clientresponse.FailureInterrupted, true
					case "insufficient_system_resource":
						failure.Kind, failed = clientresponse.FailureOverloaded, true
					default:
						failure.Kind, failed = clientresponse.FailureInvalid, true
					}
				}
			}
			data := event.Data
			// Never close a partial tool/text item based on an eventual failed
			// terminal. Normal successful streams use the established converter.
			for i := range object.Get("choices").Array() {
				data, _ = sjson.SetBytes(data, "choices."+jsonIndex(i)+".finish_reason", nil)
			}
			clientresponse.AppendEvent(&prefix, clientresponse.Event{Data: data})
		}
		if terminal && done && !failed && last+2 == len(normalized) {
			return managedProtocolClientResponse{}, 0, false, nil
		}
		if prefix.Len() == 0 {
			return managedProtocolClientResponse{Body: failure.Body(policy), Headers: failure.Headers(policy, headers, false, false)}, uint32(failure.Status(policy.Protocol)), true, nil
		}
		conversion, e := managedProtocolConversionInput(prefix.Bytes(), true)
		if e != nil {
			return managedProtocolClientResponse{}, 0, true, e
		}
		switch policy.Protocol {
		case clientresponse.Responses:
			converter := provider.NewChatCompletionToResponsesStreamConverter()
			body, err = converter.Convert(conversion, false)
			if err == nil {
				var terminal []byte
				terminal, err = converter.Fail(failure.ResourceError(policy.Protocol))
				body = append(body, terminal...)
			}
		case clientresponse.Messages:
			body, err = (&provider.ClaudeToOpenAIConverter{}).ConvertOpenAIStreamResponseToClaude(ctx, conversion)
			var terminal bytes.Buffer
			clientresponse.AppendEvent(&terminal, clientresponse.Event{Name: "error", Data: failure.Body(policy)})
			body = append(body, terminal.Bytes()...)
		default:
			var output bytes.Buffer
			output.Write(prefix.Bytes())
			clientresponse.AppendEvent(&output, clientresponse.Event{Name: "error", Data: failure.Body(policy)})
			body = output.Bytes()
		}
	}
	if err == nil && status == 200 {
		body, err = clientresponse.Project(policy, body, stream, usage)
	}
	return managedProtocolClientResponse{Body: body, Headers: failure.Headers(policy, headers, stream, true)}, status, true, err
}

func jsonIndex(i int) string {
	raw, _ := json.Marshal(i)
	return string(raw)
}
