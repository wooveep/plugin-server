package main

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/tokenestimate"
	"github.com/tidwall/gjson"
)

// managedNativeStream observes the original wire events. The same terminal
// rules apply to a Router stream and an already buffered callout.
type managedNativeStream struct {
	protocol             string
	terminalInput        bool
	decoder              managedSSEDecoder
	observed             managedProtocolObservation
	invalid              bool
	terminal             bool
	thinking             bool
	messageStarted       bool
	rawStopReason        string
	stopReason           string
	finalOutput          bool
	terminalInputSettled bool
	messageUsage         map[string]json.RawMessage
	publicUsage          clientresponse.Usage
	collectText          bool
	text                 strings.Builder
	textLimited          bool
}

func newManagedNativeStream(protocol string, terminalInput, collectText bool) *managedNativeStream {
	return &managedNativeStream{protocol: protocol, terminalInput: terminalInput, collectText: collectText,
		observed: managedProtocolStreamUnknown("stream_terminal_not_observed")}
}

func (s *managedNativeStream) push(chunk []byte, after func(managedProtocolEvent)) {
	if s.invalid {
		return
	}
	err := s.decoder.push(chunk, func(event managedProtocolEvent) {
		if s.invalid {
			return
		}
		s.invalid = !s.observeEvent(event)
		if s.invalid {
			return
		}
		kind := event.Name
		if kind == "" && s.collectText {
			kind = gjson.GetBytes(event.Data, "type").String()
		}
		// Completed Responses items echo the deltas already received.
		if s.collectText && !s.textLimited {
			var text string
			switch kind {
			case "response.output_text.delta", "response.function_call_arguments.delta":
				text = tokenestimate.ResponseText(event.Data)
			case "response.reasoning_text.delta", "response.reasoning_summary_text.delta",
				"response.custom_tool_call_input.delta", "response.refusal.delta":
				if delta := gjson.GetBytes(event.Data, "delta"); delta.Type == gjson.String {
					text = delta.String()
				}
			case "content_block_delta":
				text = tokenestimate.ResponseText(event.Data)
				if text == "" {
					delta := gjson.GetBytes(event.Data, "delta")
					switch delta.Get("type").String() {
					case "input_json_delta":
						text = delta.Get("partial_json").String()
					case "thinking_delta":
						text = delta.Get("thinking").String()
					}
				}
			}
			if text != "" {
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
	})
	s.invalid = s.invalid || err != nil
}

func (s *managedNativeStream) observeEvent(event managedProtocolEvent) bool {
	if s.terminal {
		if s.protocol == managedProtocolResponses && bytes.Equal(event.Data, []byte("[DONE]")) ||
			s.protocol == managedProtocolMessages && s.observed.ProviderError {
			return true
		}
		if s.protocol == managedProtocolMessages || s.protocol == managedProtocolResponses {
			_, kind, err := managedProtocolEventObject(event)
			return err == nil && kind == "ping"
		}
		return false
	}
	object, kind, err := managedProtocolEventObject(event)
	if err != nil {
		return false
	}
	if kind == "error" {
		s.observed.ExecutionResult, s.observed.ProviderError, s.observed.BodyComplete = "failed", true, true
		s.observed.Usage = executionfacts.MissingTokenUsage("stream_provider_error")
		s.terminal = true
		return true
	}
	if s.protocol == managedProtocolResponses {
		return s.observeResponses(object, kind)
	}
	return s.observeMessages(object, kind)
}

func (s *managedNativeStream) observeResponses(object map[string]json.RawMessage, kind string) bool {
	var response map[string]json.RawMessage
	if raw, exists := object["response"]; exists {
		if json.Unmarshal(raw, &response) != nil || response == nil || !managedProtocolMergeModel(&s.observed.ResponseModel, response["model"]) {
			return false
		}
	}
	s.thinking = s.thinking || managedResponsesEventThinking(object, kind)
	expected := map[string]string{"response.completed": "succeeded", "response.incomplete": "partial", "response.failed": "failed"}[kind]
	if expected == "" {
		return true
	}
	if managedResponsesTerminal(response) != expected {
		return false
	}
	s.observed.Usage = managedResponsesUsage(response["usage"])
	s.publicUsage = clientresponse.ReadUsageObject(response["usage"], clientresponse.Responses)
	s.observed.ExecutionResult, s.observed.BodyComplete = expected, true
	if expected == "failed" || !managedProtocolNull(response["error"]) {
		s.observed.ProviderError, s.observed.ExecutionResult = true, "failed"
	}
	s.terminal = true
	return true
}

func (s *managedNativeStream) observeMessages(object map[string]json.RawMessage, kind string) bool {
	switch kind {
	case "ping":
		return true
	case "message_start":
		var message map[string]json.RawMessage
		if s.messageStarted || json.Unmarshal(object["message"], &message) != nil || message == nil ||
			!managedProtocolMergeModel(&s.observed.ResponseModel, message["model"]) {
			return false
		}
		if managedProtocolNull(message["usage"]) {
			s.messageUsage = map[string]json.RawMessage{}
		} else if json.Unmarshal(message["usage"], &s.messageUsage) != nil || s.messageUsage == nil {
			return false
		}
		for key, value := range s.messageUsage {
			if managedProtocolNull(value) {
				delete(s.messageUsage, key)
			}
		}
		s.messageStarted = true
		s.thinking = s.thinking || managedMessagesEventThinking(object, kind)
		s.publicUsage = clientresponse.ReadUsageObject(message["usage"], clientresponse.Messages)
		return true
	case "message_delta":
		if !s.messageStarted {
			return false
		}
		var delta map[string]json.RawMessage
		if json.Unmarshal(object["delta"], &delta) != nil || delta == nil {
			return false
		}
		if !managedProtocolNull(delta["stop_reason"]) {
			var rawReason string
			if json.Unmarshal(delta["stop_reason"], &rawReason) != nil || rawReason == "" {
				return false
			}
			next := managedMessagesTerminal(map[string]json.RawMessage{"stop_reason": delta["stop_reason"]})
			if next == "unknown" || s.rawStopReason != "" && s.rawStopReason != rawReason {
				return false
			}
			s.rawStopReason = rawReason
			s.stopReason = next
		}
		if raw, exists := object["usage"]; exists && !managedProtocolNull(raw) {
			var update map[string]json.RawMessage
			if json.Unmarshal(raw, &update) != nil || update == nil {
				return false
			}
			for key, value := range update {
				if managedProtocolNull(value) {
					continue
				}
				switch key {
				case "output_tokens":
					next, valid := managedCount(value)
					if !valid {
						return false
					}
					if prior, valid := managedCount(s.messageUsage[key]); valid && next.Cmp(prior) < 0 {
						return false
					}
					s.finalOutput = s.finalOutput || s.stopReason != ""
				case "input_tokens":
					next, valid := managedCount(value)
					if !valid {
						return false
					}
					if s.terminalInputSettled {
						if prior, valid := managedCount(s.messageUsage[key]); valid && next.Cmp(prior) != 0 {
							return false
						}
					}
					if s.stopReason != "" {
						s.terminalInputSettled = true
					}
				case "cache_read_input_tokens", "cache_creation_input_tokens", "cache_write_tokens":
					if _, valid := managedCount(value); !valid {
						return false
					}
				case "cache_creation", "cache_write_tokens_details", "server_tool_use":
					var nested map[string]json.RawMessage
					if json.Unmarshal(value, &nested) != nil || nested == nil {
						return false
					}
				}
				s.messageUsage[key] = value
			}
			encoded, _ := json.Marshal(s.messageUsage)
			s.publicUsage = clientresponse.ReadUsageObject(encoded, clientresponse.Messages)
		}
		return true
	case "message_stop":
		if !s.messageStarted || s.stopReason == "" {
			return false
		}
		encoded, _ := json.Marshal(s.messageUsage)
		s.observed.Usage = managedMessagesUsage(encoded)
		if !s.finalOutput {
			s.observed.Usage = executionfacts.MissingTokenUsage("stream_usage_not_observed")
		}
		s.observed.ExecutionResult, s.observed.BodyComplete = s.stopReason, true
		s.terminal = true
		return true
	case "content_block_start", "content_block_delta", "content_block_stop":
		if !s.messageStarted || s.stopReason != "" {
			return false
		}
		s.thinking = s.thinking || managedMessagesEventThinking(object, kind)
		return true
	default:
		return false
	}
}

func (s *managedNativeStream) observation() managedProtocolObservation {
	if s.invalid || !s.terminal || s.decoder.finish() != nil {
		return managedProtocolStreamUnknown("stream_terminal_not_observed")
	}
	return s.observed
}

func (s *managedNativeStream) estimateBody() []byte {
	body, _ := json.Marshal(struct {
		Text string `json:"output_text"`
	}{Text: s.text.String()})
	return body
}
