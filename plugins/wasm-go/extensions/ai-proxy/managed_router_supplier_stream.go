package main

import (
	"bytes"
	"encoding/json"
	"strconv"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/provider"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

// Native counters are observed before each event's content conversion. Neither
// this decoder nor the converter waits for the complete generation response.
type managedRouterSupplierStream struct {
	context  wrapper.HttpContext
	state    *managedExecutionState
	format   string
	decoder  managedSSEDecoder
	usage    *executionfacts.Usage
	model    string
	finished map[string]string
	last     managedProtocolObservation
	invalid  bool
	failed   bool
	done     bool
	thinking bool
}

func (s *managedRouterSupplierStream) push(chunk []byte, end bool, forward func([]byte), comment func()) {
	s.decoder.onComment = comment
	if !s.invalid {
		s.invalid = s.decoder.push(chunk, func(event managedProtocolEvent) {
			if bytes.Equal(bytes.TrimSpace(event.Data), []byte("[DONE]")) {
				s.done = true
				return
			}
			var object map[string]json.RawMessage
			if s.done || decodeManagedJSON(event.Data, &object, false) != nil || object == nil {
				s.invalid = true
				return
			}
			headers := [][2]string{{"content-type", "application/json"}}
			var observed managedProtocolObservation
			usageKey := "usage"
			switch s.format {
			case "qwen":
				observed = managedQwenDocumentObservation(managedProtocolChat, 200, headers, event.Data, true, event.Name == "error", 0)
				s.thinking = s.thinking || managedQwenReasoning(event.Data)
			case "gemini":
				observed = managedGoogleObservation(managedProtocolChat, 200, headers, event.Data, true, 0)
				usageKey = "usageMetadata"
				var candidates []struct {
					Index  *uint32 `json:"index"`
					Finish string  `json:"finishReason"`
				}
				if raw, exists := object["candidates"]; exists {
					if json.Unmarshal(raw, &candidates) != nil {
						s.invalid = true
						return
					}
				}
				for ordinal, candidate := range candidates {
					index := strconv.Itoa(ordinal)
					if candidate.Index != nil {
						index = strconv.FormatUint(uint64(*candidate.Index), 10)
					}
					if _, exists := s.finished[index]; !exists || candidate.Finish != "" {
						s.finished[index] = candidate.Finish
					}
				}
			default:
				// Moonshot's original nested counters precede its lossless
				// relocation to the OpenAI root.
				var choices []map[string]json.RawMessage
				if raw, exists := object["choices"]; exists && json.Unmarshal(raw, &choices) != nil {
					s.invalid = true
					return
				}
				for _, choice := range choices {
					if raw, exists := choice["usage"]; exists {
						object[usageKey] = raw
						delete(choice, "usage")
					}
				}
				if len(choices) != 0 {
					object["choices"], _ = json.Marshal(choices)
				}
				observed.Usage = managedOpenAIUsage(object)
			}
			if _, exists := object[usageKey]; exists {
				s.usage = observed.Usage
			}
			if s.state.router.toolProtocol != "" {
				s.state.router.toolUsage = managedToolEventUsage(s.state.router.toolProtocol, event.Data, s.state.router.toolUsage)
			}
			if observed.ResponseModel != "" {
				if s.model != "" && s.model != observed.ResponseModel {
					s.invalid = true
				}
				s.model = observed.ResponseModel
			}
			s.failed = s.failed || observed.ProviderError || event.Name == "error"
			s.last = observed
			delete(object, usageKey)
			stripped, err := json.Marshal(object)
			if err != nil {
				s.invalid = true
				return
			}
			converted, err := s.state.config.vendor.ConvertNativeStreamEvent(s.context, provider.StreamEvent{Event: event.Name, Data: string(stripped)})
			if err != nil {
				s.invalid = true
				return
			}
			// Legacy content codecs may emit zero-valued SDK usage after their
			// native counters were removed. Only the original observer supplies it.
			events, err := managedProtocolEvents(converted)
			if err != nil {
				s.invalid = true
				return
			}
			var wire bytes.Buffer
			for _, convertedEvent := range events {
				if bytes.Equal(bytes.TrimSpace(convertedEvent.Data), []byte("[DONE]")) {
					continue
				}
				var content map[string]json.RawMessage
				if decodeManagedJSON(convertedEvent.Data, &content, false) != nil || content == nil {
					s.invalid = true
					return
				}
				delete(content, "usage")
				data, _ := json.Marshal(content)
				managedProtocolWriteEvent(&wire, convertedEvent.Name, data)
			}
			if s.failed {
				managedProtocolWriteEvent(&wire, "error", []byte(`{"error":{"code":"upstream_error","message":"Upstream request failed"}}`))
			}
			forward(wire.Bytes())
		}) != nil || s.invalid
	}
	s.decoder.onComment = nil
	if end {
		observed := s.observation()
		if observed.BodyComplete && !observed.ProviderError {
			if usage := managedProtocolWireUsage(managedProtocolChat, observed.Usage); usage != nil {
				body, _ := json.Marshal(struct {
					Choices []json.RawMessage `json:"choices"`
					Usage   json.RawMessage   `json:"usage"`
				}{[]json.RawMessage{}, usage})
				var wire bytes.Buffer
				managedProtocolWriteEvent(&wire, "", body)
				forward(wire.Bytes())
			}
			forward([]byte("data: [DONE]\n\n"))
		}
	}
}

func (s *managedRouterSupplierStream) observation() managedProtocolObservation {
	o := managedProtocolObservation{Usage: s.usage, ResponseModel: s.model, ExecutionResult: "unknown", Acceptance: "unknown", ProviderError: s.failed}
	if o.Usage == nil {
		o.Usage = executionfacts.MissingTokenUsage("native_stream_usage_not_observed")
	}
	if s.invalid || s.decoder.finish() != nil {
		return o
	}
	switch s.format {
	case "qwen":
		o.ExecutionResult = s.last.ExecutionResult
	case "gemini":
		if len(s.finished) != 0 {
			o.ExecutionResult = "succeeded"
			for _, reason := range s.finished {
				switch reason {
				case "STOP":
				case "MAX_TOKENS", "SAFETY", "RECITATION", "LANGUAGE", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII", "IMAGE_SAFETY", "IMAGE_PROHIBITED_CONTENT", "IMAGE_RECITATION":
					if o.ExecutionResult != "unknown" {
						o.ExecutionResult = "partial"
					}
				default:
					o.ExecutionResult = "unknown"
				}
			}
		}
	default:
		observed := s.state.router.chat.observation()
		o.ExecutionResult = observed.ExecutionResult
		// Moonshot is a Chat stream with the same explicit terminal contract.
		if s.done && s.state.router.chat.allChoicesFinished() && !s.state.router.chat.invalid {
			o.ExecutionResult = "succeeded"
			if s.state.router.chat.partial {
				o.ExecutionResult = "partial"
			}
		}
		if o.ResponseModel == "" {
			o.ResponseModel = s.state.router.chat.responseModel
		}
	}
	if o.ProviderError {
		o.ExecutionResult = "failed"
	}
	o.BodyComplete = o.ExecutionResult == "succeeded" || o.ExecutionResult == "partial" || o.ExecutionResult == "failed"
	if o.ExecutionResult == "succeeded" || o.ExecutionResult == "partial" {
		o.Acceptance = "accepted"
	}
	return o
}
