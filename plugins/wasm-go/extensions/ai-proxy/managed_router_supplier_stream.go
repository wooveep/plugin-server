package main

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/provider"
	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/tokenestimate"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

// Native counters are observed before each event's content conversion. Neither
// this decoder nor the converter waits for the complete generation response.
type managedRouterSupplierStream struct {
	context        wrapper.HttpContext
	state          *managedExecutionState
	format         string
	decoder        managedSSEDecoder
	usage          *executionfacts.Usage
	priorUsage     *executionfacts.Usage
	publicUsage    clientresponse.Usage
	hasPublicUsage bool
	model          string
	finished       map[string]string
	choiceDone     map[string]bool
	last           managedProtocolObservation
	invalid        bool
	failed         bool
	namedError     bool
	done           bool
	partial        bool
	thinking       bool
	modelConflict  bool
	usageConflict  bool
	collectText    bool
	incremental    bool
	text           strings.Builder
	estimateUsage  json.RawMessage
	textLimited    bool
}

func (s *managedRouterSupplierStream) allChoicesFinished() bool {
	for _, done := range s.choiceDone {
		if !done {
			return false
		}
	}
	return len(s.choiceDone) != 0
}

func (s *managedRouterSupplierStream) estimateBody() []byte {
	text, _ := json.Marshal(s.text.String())
	choices := append([]byte(`[{"message":{"content":`), text...)
	choices = append(choices, []byte(`}}]`)...)
	object := map[string]json.RawMessage{"choices": choices}
	if s.estimateUsage != nil {
		if s.format == "gemini" {
			object["usageMetadata"] = s.estimateUsage
		} else {
			object["usage"] = s.estimateUsage
		}
	}
	body, _ := json.Marshal(object)
	return body
}

// pushRaw observes raw supplier SSE frames (billing counters, Bailian request_id,
// model/usage conflicts, bounded text estimation, and terminal completion) without
// stripping usage or converting through intermediate Chat SSE.
func (s *managedRouterSupplierStream) pushRaw(chunk []byte, after func(managedProtocolEvent), comment func()) {
	s.decoder.onComment = comment
	defer func() {
		s.decoder.onComment = nil
	}()
	if s.invalid {
		return
	}
	s.invalid = s.decoder.push(chunk, func(event managedProtocolEvent) {
		if !s.modelConflict && !s.usageConflict && event.Name == "error" {
			s.namedError = true
		}
		if s.done {
			s.invalid = true
			return
		}
		if bytes.Equal(bytes.TrimSpace(event.Data), []byte("[DONE]")) {
			s.done = true
			if after != nil {
				after(event)
			}
			return
		}
		var object map[string]json.RawMessage
		if decodeManagedJSON(event.Data, &object, false) != nil || object == nil {
			if event.Name == "error" {
				s.failed = true
				if after != nil {
					after(event)
				}
				return
			}
			s.invalid = true
			return
		}

		headers := [][2]string{{"content-type", "application/json"}}
		switch s.format {
		case "qwen":
			var requestID string
			_ = json.Unmarshal(object["request_id"], &requestID)
			recordManagedBailianRequestID(s.state, requestID)
			observed := managedQwenDocumentObservation(managedProtocolChat, 200, headers, event.Data, true, event.Name == "error", 0)
			s.thinking = s.thinking || managedQwenReasoning(event.Data)
			if raw, exists := object["usage"]; exists {
				s.usage = observed.Usage
				if !managedProtocolNull(raw) {
					s.estimateUsage = raw
					if parsed := clientresponse.ReadSupplierUsage(event.Data, "qwen", false); parsed.Input != nil {
						s.publicUsage = parsed
						s.hasPublicUsage = true
					}
				}
			}
			if observed.ResponseModel != "" {
				if s.model != "" && s.model != observed.ResponseModel {
					s.modelConflict = true
					s.invalid = true
				}
				s.model = observed.ResponseModel
			}
			s.failed = s.failed || observed.ProviderError || event.Name == "error"
			s.last = observed

		case "gemini":
			observed := managedGoogleObservation(managedProtocolChat, 200, headers, event.Data, true, 0)
			s.thinking = s.thinking || managedGoogleThinking(event.Data)
			if raw, exists := object["usageMetadata"]; exists {
				s.usage = observed.Usage
				if !managedProtocolNull(raw) {
					s.estimateUsage = raw
					if parsed := clientresponse.ReadSupplierUsage(event.Data, "gemini", false); parsed.Input != nil {
						s.publicUsage = parsed
						s.hasPublicUsage = true
					}
				}
			}
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
			if observed.ResponseModel != "" {
				if s.model != "" && s.model != observed.ResponseModel {
					s.modelConflict = true
					s.invalid = true
				}
				s.model = observed.ResponseModel
			}
			s.failed = s.failed || observed.ProviderError || event.Name == "error"
			s.last = observed

		default:
			s.thinking = s.thinking || managedQwenReasoning(event.Data)
			if event.Name == "error" || !managedProtocolNull(object["error"]) {
				s.failed = true
			}
			var choices []map[string]json.RawMessage
			if raw, exists := object["choices"]; exists && json.Unmarshal(raw, &choices) != nil {
				s.invalid = true
				return
			}
			var rawUsage json.RawMessage
			if raw, exists := object["usage"]; exists && !managedProtocolNull(raw) {
				rawUsage = raw
			}
			for _, choice := range choices {
				if raw, exists := choice["usage"]; exists && !managedProtocolNull(raw) {
					rawUsage = raw
				}
			}
			var usage *executionfacts.Usage
			if len(rawUsage) > 0 {
				s.estimateUsage = rawUsage
				usage = managedOpenAIUsage(map[string]json.RawMessage{"usage": rawUsage})
				if parsed := clientresponse.ReadSupplierUsage(event.Data, "moonshot", false); parsed.Input != nil {
					s.publicUsage = parsed
					s.hasPublicUsage = true
				}
			}
			if !s.modelConflict && !s.usageConflict {
				s.modelConflict = !managedProtocolMergeModel(&s.model, object["model"])
				if !s.modelConflict && usage != nil {
					s.usageConflict = s.priorUsage != nil && !managedProtocolSameUsage(s.priorUsage, usage)
					if !s.usageConflict {
						s.priorUsage = usage
					}
				}
			}
			if len(choices) > 0 {
				if s.choiceDone == nil {
					s.choiceDone = make(map[string]bool)
				}
				for _, choice := range choices {
					index, ok := managedCount(choice["index"])
					if !ok || index.BitLen() > 32 || s.choiceDone[index.String()] {
						s.invalid = true
						return
					}
					key := index.String()
					s.choiceDone[key] = false
					if raw := choice["finish_reason"]; !managedProtocolNull(raw) {
						var reason string
						if json.Unmarshal(raw, &reason) != nil {
							s.invalid = true
							return
						}
						if reason != "" {
							s.choiceDone[key] = true
							s.partial = s.partial || reason == "length" || reason == "content_filter" || reason == "sensitive" || reason == "model_context_window_exceeded"
						}
					}
				}
			}
			if usage != nil && s.allChoicesFinished() {
				s.usage = usage
			}
		}

		if s.collectText && !s.textLimited && !s.failed {
			text := tokenestimate.ResponseText(event.Data)
			if text != "" {
				if s.format == "qwen" && !s.incremental {
					s.text.Reset()
				}
				if s.text.Len()+len(text) <= managedEstimateTextBytes {
					s.text.WriteString(text)
				} else {
					s.textLimited = true
					s.text.Reset()
				}
			}
		}

		if after != nil {
			after(event)
		}
	}) != nil || s.invalid
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
				var requestID string
				_ = json.Unmarshal(object["request_id"], &requestID)
				recordManagedBailianRequestID(s.state, requestID)
				observed = managedQwenDocumentObservation(managedProtocolChat, 200, headers, event.Data, true, event.Name == "error", 0)
				s.thinking = s.thinking || managedQwenReasoning(event.Data)
			case "gemini":
				observed = managedGoogleObservation(managedProtocolChat, 200, headers, event.Data, true, 0)
				s.thinking = s.thinking || managedGoogleThinking(event.Data)
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
				s.thinking = s.thinking || managedQwenReasoning(event.Data)
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
			if raw, exists := object[usageKey]; exists {
				s.usage = observed.Usage
				if !managedProtocolNull(raw) {
					if parsed := clientresponse.ReadSupplierUsage(event.Data, s.format, false); parsed.Input != nil {
						s.publicUsage = parsed
						s.hasPublicUsage = true
					}
				}
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
			if s.failed {
				errPayload := event.Data
				if managedProtocolNull(object["error"]) {
					errPayload, _ = json.Marshal(map[string]json.RawMessage{"error": event.Data})
				}
				var wire bytes.Buffer
				managedProtocolWriteEvent(&wire, "error", errPayload)
				forward(wire.Bytes())
				return
			}
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
			if len(converted) == 0 {
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
				_, hadUsage := content["usage"]
				delete(content, "usage")
				if hadUsage {
					var choices []struct {
						Delta        map[string]json.RawMessage `json:"delta"`
						FinishReason *string                    `json:"finish_reason"`
					}
					if err := json.Unmarshal(content["choices"], &choices); err != nil {
						s.invalid = true
						return
					}
					// Qwen's content converter emits an empty delta for its generated
					// usage after finish_reason. Omit it so Chat choice finality holds.
					if len(choices) == 0 || len(choices) == 1 && len(choices[0].Delta) == 0 && choices[0].FinishReason == nil {
						continue
					}
				}
				data, _ := json.Marshal(content)
				managedProtocolWriteEvent(&wire, "", data)
			}
			if wire.Len() != 0 {
				forward(wire.Bytes())
			}
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
	if s.format != "qwen" && s.format != "gemini" && (s.state == nil || s.state.router == nil || s.state.router.chat == nil) {
		finishErr := s.decoder.finish()
		providerError := s.failed || s.namedError
		usage := s.usage
		result := "unknown"
		if finishErr != nil || s.invalid || !s.done || !s.allChoicesFinished() {
			usage = executionfacts.MissingTokenUsage("stream_terminal_not_observed")
		} else if providerError {
			usage, result = executionfacts.MissingTokenUsage("stream_provider_error"), "failed"
		} else {
			if usage == nil {
				usage = executionfacts.MissingTokenUsage("stream_usage_not_observed")
			}
			if s.partial {
				result = "partial"
			} else {
				result = "succeeded"
			}
		}
		if finishErr == nil {
			if s.namedError {
				result, providerError = "failed", true
			}
			switch {
			case s.modelConflict:
				usage, result = executionfacts.MissingTokenUsage("stream_model_conflict"), "unknown"
			case s.usageConflict:
				usage = executionfacts.MissingTokenUsage("stream_usage_conflict")
			case providerError && result == "failed" && s.priorUsage != nil && s.priorUsage.Status == "reported":
				usage = s.priorUsage
			}
		}
		acceptance := "unknown"
		if result == "succeeded" || result == "partial" {
			acceptance = "accepted"
		}
		return managedProtocolObservation{
			Usage:           usage,
			ExecutionResult: result,
			ProviderError:   providerError,
			Acceptance:      acceptance,
			ResponseModel:   s.model,
			BodyComplete:    result != "unknown",
		}
	}

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
