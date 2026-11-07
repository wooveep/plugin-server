package main

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/tokenestimate"
)

// Token estimation must not retain an entire long stream. Crossing this limit
// disables the local estimate, while forwarding and provider usage continue.
const managedEstimateTextBytes = 1024 * 1024

type managedChatStream struct {
	completions   bool
	decoder       managedSSEDecoder
	finished      map[string]bool
	finalUsage    *executionfacts.Usage
	priorUsage    *executionfacts.Usage
	responseModel string
	framingError  bool
	done          bool
	invalid       bool
	providerError bool
	namedError    bool
	modelConflict bool
	usageConflict bool
	partial       bool
	thinking      bool
	collectText   bool
	text          strings.Builder
	estimateUsage json.RawMessage
	textLimited   bool
}

func (s *managedChatStream) push(chunk []byte) {
	s.pushEvents(chunk, nil)
}

func (s *managedChatStream) pushEvents(chunk []byte, after func(managedProtocolEvent)) {
	if !s.framingError {
		s.framingError = s.decoder.push(chunk, func(event managedProtocolEvent) {
			s.observeEvent(event)
			if after != nil {
				after(event)
			}
		}) != nil
	}
}

func (s *managedChatStream) allChoicesFinished() bool {
	for _, finished := range s.finished {
		if !finished {
			return false
		}
	}
	return len(s.finished) != 0
}

func (s *managedChatStream) observeEvent(event managedProtocolEvent) {
	if !s.modelConflict && !s.usageConflict && event.Name == "error" {
		s.namedError = true
	}
	if s.done {
		s.invalid = true
		return
	}
	if bytes.Equal(bytes.TrimSpace(event.Data), []byte("[DONE]")) {
		s.done = true
		return
	}
	var object map[string]json.RawMessage
	if decodeManagedJSON(event.Data, &object, false) != nil || object == nil {
		s.invalid = true
		return
	}
	if event.Name == "error" || !managedProtocolNull(object["error"]) {
		s.providerError = true
	}
	if s.collectText {
		s.thinking = s.thinking || managedQwenReasoning(event.Data)
		if !s.textLimited {
			text := tokenestimate.ResponseText(event.Data)
			if s.text.Len()+len(text) <= managedEstimateTextBytes {
				s.text.WriteString(text)
			} else {
				s.textLimited = true
				s.text.Reset()
			}
		}
		if raw := object["usage"]; !managedProtocolNull(raw) {
			s.estimateUsage = raw
		}
	}
	var usage *executionfacts.Usage
	if !managedProtocolNull(object["usage"]) {
		usage = managedOpenAIUsage(object)
	}
	if !s.modelConflict && !s.usageConflict {
		s.modelConflict = !managedProtocolMergeModel(&s.responseModel, object["model"])
		if !s.modelConflict && usage != nil {
			s.usageConflict = s.priorUsage != nil && !managedProtocolSameUsage(s.priorUsage, usage)
			if !s.usageConflict {
				s.priorUsage = usage
			}
		}
	}
	if raw, exists := object["choices"]; exists || s.completions && !s.providerError {
		var choices []map[string]json.RawMessage
		if json.Unmarshal(raw, &choices) != nil ||
			s.completions && (string(object["object"]) != `"text_completion"` || len(choices) != 1) {
			s.invalid = true
			return
		}
		if s.finished == nil {
			s.finished = make(map[string]bool)
		}
		for _, choice := range choices {
			index, ok := managedCount(choice["index"])
			if !ok || index.BitLen() > 32 || s.finished[index.String()] {
				s.invalid = true
				return
			}
			key := index.String()
			s.finished[key] = false
			if s.completions {
				switch managedCompletionChoice(choice) {
				case "unknown":
					s.invalid = true
					return
				case "partial":
					s.partial = true
					s.finished[key] = true
				case "succeeded":
					s.finished[key] = true
				}
				continue
			}
			if raw := choice["finish_reason"]; !managedProtocolNull(raw) {
				var reason string
				if json.Unmarshal(raw, &reason) != nil || reason == "" {
					s.invalid = true
					return
				}
				s.finished[key] = true
				s.partial = s.partial || reason == "length" || reason == "content_filter"
			}
		}
	}
	if usage != nil && s.allChoicesFinished() {
		s.finalUsage = usage
	}
}

func (s *managedChatStream) estimateBody() []byte {
	text, _ := json.Marshal(s.text.String())
	choices := append([]byte(`[{"message":{"content":`), text...)
	choices = append(choices, []byte(`}}]`)...)
	object := map[string]json.RawMessage{"choices": choices}
	if s.estimateUsage != nil {
		object["usage"] = s.estimateUsage
	}
	body, _ := json.Marshal(object)
	return body
}

func (s *managedChatStream) result() (*executionfacts.Usage, string, bool) {
	if s.framingError || s.decoder.finish() != nil || s.invalid || !s.done || !s.allChoicesFinished() {
		return executionfacts.MissingTokenUsage("stream_terminal_not_observed"), "unknown", s.providerError
	}
	if s.providerError {
		return executionfacts.MissingTokenUsage("stream_provider_error"), "failed", true
	}
	usage := s.finalUsage
	if usage == nil {
		usage = executionfacts.MissingTokenUsage("stream_usage_not_observed")
	}
	if s.partial {
		return usage, "partial", false
	}
	return usage, "succeeded", false
}

func (s *managedChatStream) observation() managedProtocolObservation {
	usage, result, providerError := s.result()
	if !s.framingError && s.decoder.finish() == nil {
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
	return managedProtocolObservation{
		Usage: usage, ExecutionResult: result, ProviderError: providerError, Acceptance: "unknown",
		ResponseModel: s.responseModel, BodyComplete: result != "unknown",
	}
}
