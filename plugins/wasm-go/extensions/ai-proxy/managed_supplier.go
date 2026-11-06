package main

import (
	"bytes"
	"encoding/json"
	"errors"

	supplier "github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/provider"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

func managedSupplierAPIName(profile managedProtocolProfile) supplier.ApiName {
	switch profile.UpstreamProtocol {
	case managedProtocolCompletions:
		return supplier.ApiNameCompletion
	case managedProtocolMessages:
		return supplier.ApiNameAnthropicMessages
	case managedProtocolResponses:
		return supplier.ApiNameResponses
	case managedProtocolEmbeddings:
		return supplier.ApiNameEmbeddings
	case managedProtocolDashScopeText:
		return supplier.ApiNameDashScopeText
	case managedProtocolDashScopeMultimodal:
		return supplier.ApiNameDashScopeMultimodal
	default:
		return supplier.ApiNameChatCompletion
	}
}

// DashScope emits cumulative native counters. Observe the final original result,
// never sum stream frames or read the content converter's machine-int defaults.
// Cache paths: https://help.aliyun.com/en/model-studio/context-cache
func managedQwenObservation(protocol string, status uint16, headers [][2]string, body []byte, complete bool, expectedEmbeddings int) managedProtocolObservation {
	observation := managedProtocolObservation{Usage: executionfacts.MissingTokenUsage("provider_usage_not_observed"), ExecutionResult: "unknown", Acceptance: "unknown", BodyComplete: complete}
	if status >= 400 && status <= 599 {
		observation.ExecutionResult = "failed"
	}
	stream, err := managedProtocolContentType(headers)
	if !complete || err != nil || status < 100 || status > 599 {
		return observation
	}
	last := body
	var streamError bool
	if stream {
		events, err := managedProtocolEvents(body)
		if err != nil || protocol == managedProtocolEmbeddings {
			observation.BodyComplete = false
			return observation
		}
		last = nil
		for _, event := range events {
			if bytes.Equal(event.Data, []byte("[DONE]")) {
				continue
			}
			last = event.Data
			streamError = streamError || event.Name == "error"
		}
	}
	return managedQwenDocumentObservation(protocol, status, headers, last, stream, streamError, expectedEmbeddings)
}

func managedQwenDocumentObservation(protocol string, status uint16, headers [][2]string, body []byte, stream, streamError bool, expectedEmbeddings int) managedProtocolObservation {
	observation := managedProtocolObservation{Usage: executionfacts.MissingTokenUsage("provider_usage_not_observed"), ExecutionResult: "unknown", Acceptance: "unknown", BodyComplete: true}
	if status >= 400 {
		observation.ExecutionResult = "failed"
	}
	var object, output, usage map[string]json.RawMessage
	if decodeManagedJSON(body, &object, false) != nil || object == nil {
		return observation
	}
	if raw, exists := object["output"]; exists {
		if err := json.Unmarshal(raw, &output); err != nil {
			return observation
		}
	}
	if raw, exists := object["usage"]; exists {
		if err := json.Unmarshal(raw, &usage); err != nil {
			return observation
		}
	}
	var code string
	_ = json.Unmarshal(object["code"], &code)
	observation.ProviderError = streamError || code != "" && code != "200"
	managedProtocolMergeModel(&observation.ResponseModel, object["model"])
	wireUsage := make(map[string]json.RawMessage)
	for source, destination := range map[string]string{"input_tokens": "prompt_tokens", "output_tokens": "completion_tokens", "total_tokens": "total_tokens", "prompt_tokens_details": "prompt_tokens_details", "output_tokens_details": "completion_tokens_details"} {
		if value, exists := usage[source]; exists {
			wireUsage[destination] = value
		}
	}
	// Beijing returns prompt_tokens_details; some regions still return cached_tokens.
	if details, exists := wireUsage["prompt_tokens_details"]; !exists || managedProtocolNull(details) {
		if cached, exists := usage["cached_tokens"]; exists {
			wireUsage["prompt_tokens_details"], _ = json.Marshal(map[string]json.RawMessage{"cached_tokens": cached})
		}
	}
	if protocol == managedProtocolEmbeddings {
		if _, exists := wireUsage["prompt_tokens"]; !exists {
			wireUsage["prompt_tokens"] = usage["total_tokens"]
		}
		var embeddings []map[string]json.RawMessage
		_ = json.Unmarshal(output["embeddings"], &embeddings)
		data := make([]map[string]json.RawMessage, 0, len(embeddings))
		for _, item := range embeddings {
			data = append(data, map[string]json.RawMessage{"index": item["text_index"], "embedding": item["embedding"]})
		}
		usageBytes, _ := json.Marshal(wireUsage)
		dataBytes, _ := json.Marshal(data)
		view, _ := json.Marshal(map[string]json.RawMessage{"usage": usageBytes, "data": dataBytes})
		observed := managedEmbeddingObservation(status, headers, view, true, expectedEmbeddings)
		observed.ResponseModel = observation.ResponseModel
		if observation.ProviderError {
			observed.ProviderError, observed.ExecutionResult, observed.Acceptance = true, "failed", "unknown"
		}
		return observed
	}
	usageBytes, _ := json.Marshal(wireUsage)
	observation.Usage = managedOpenAIUsage(map[string]json.RawMessage{"usage": usageBytes})
	observation.ExecutionResult = managedChatTerminal(output)
	if observation.ExecutionResult == "unknown" {
		// DashScope result_format=text uses a top-level finish_reason rather
		// than choices. A text field alone is never proof of completion.
		if reason, exists := output["finish_reason"]; exists {
			choice, _ := json.Marshal([]map[string]json.RawMessage{{"finish_reason": reason}})
			observation.ExecutionResult = managedChatTerminal(map[string]json.RawMessage{"choices": choice})
		}
	}
	if observation.ProviderError || status >= 400 {
		observation.ExecutionResult = "failed"
	}
	if stream && observation.ExecutionResult == "unknown" {
		observation.BodyComplete = false
		observation.Usage = executionfacts.MissingTokenUsage("qwen_stream_terminal_not_observed")
	}
	if status >= 200 && status < 300 && (observation.ExecutionResult == "succeeded" || observation.ExecutionResult == "partial") {
		observation.Acceptance = "accepted"
	}
	return observation
}

// Strip only usage before invoking the original content converters. Preserve
// exact counters separately, and restore them after conversion (or omit unknown).
func managedSupplierConversionInput(body []byte, stream bool, usageField string) ([]byte, error) {
	strip := func(raw []byte) ([]byte, error) {
		var object map[string]json.RawMessage
		if decodeManagedJSON(raw, &object, false) != nil || object == nil {
			return nil, errors.New("invalid native supplier content")
		}
		delete(object, usageField)
		return json.Marshal(object)
	}
	if !stream {
		return strip(body)
	}
	events, err := managedProtocolEvents(body)
	if err != nil {
		return nil, err
	}
	var result bytes.Buffer
	for _, event := range events {
		if bytes.Equal(event.Data, []byte("[DONE]")) {
			continue
		}
		data, err := strip(event.Data)
		if err != nil {
			return nil, err
		}
		managedProtocolWriteEvent(&result, event.Name, data)
	}
	return result.Bytes(), nil
}

func convertManagedSupplierResponse(ctx wrapper.HttpContext, s *managedExecutionState, outcome executionfacts.AttemptOutcome, headers [][2]string, body []byte) (managedProtocolClientResponse, error) {
	profile := s.config.profile
	if s.config.vendor == nil || s.config.vendor.NativeResponseFormat() == "" || *outcome.HTTPStatus >= 400 {
		return profile.ConvertResponse(ctx, *outcome.HTTPStatus, headers, body)
	}
	if !outcome.BodyComplete || outcome.ExecutionResult != "succeeded" && outcome.ExecutionResult != "partial" {
		return managedProtocolClientResponse{}, errors.New("native supplier terminal response is not verified")
	}
	stream, err := managedProtocolContentType(headers)
	if err != nil {
		return managedProtocolClientResponse{}, err
	}
	usageField := "usage"
	if s.config.vendor.NativeResponseFormat() == "gemini" {
		usageField = "usageMetadata"
	}
	conversionInput, err := managedSupplierConversionInput(body, stream, usageField)
	if err != nil {
		return managedProtocolClientResponse{}, err
	}
	converted, err := s.config.vendor.ConvertNativeResponse(ctx, conversionInput, stream)
	if err != nil {
		return managedProtocolClientResponse{}, err
	}
	converted, err = managedProtocolRestoreUsage(converted, stream, profile.UpstreamProtocol, outcome.Usage)
	if err != nil {
		return managedProtocolClientResponse{}, err
	}
	result, err := profile.ConvertResponse(ctx, *outcome.HTTPStatus, headers, converted)
	if err != nil {
		return managedProtocolClientResponse{}, err
	}
	result.Headers = managedProtocolConvertedHeaders(headers, stream)
	return result, nil
}
