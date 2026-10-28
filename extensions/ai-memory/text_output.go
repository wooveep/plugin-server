package main

import (
	"encoding/json"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/protocol"
)

// These are output-mode controls, not an input-content filter. Image, audio and
// file input blocks retain their original representation throughout memory use.
func memoryRequestOutputGate(kind protocol.ProtocolKind, body []byte) string {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil || fields == nil {
		return "request-parse-failed"
	}
	if raw := fields["modalities"]; len(raw) > 0 {
		var modalities []string
		if err := json.Unmarshal(raw, &modalities); err != nil {
			return "request-parse-failed"
		}
		for _, modality := range modalities {
			if modality != "text" {
				return "non-text-output"
			}
		}
	}
	if kind == protocol.ProtocolResponses {
		if raw := fields["background"]; len(raw) > 0 {
			var background bool
			if err := json.Unmarshal(raw, &background); err != nil {
				return "request-parse-failed"
			}
			if background {
				return "asynchronous-output"
			}
		}
	}
	return ""
}

// Match History's actual-output boundary without reinterpreting input blocks or
// parsing SSE locally. Streaming callers provide the existing parser's JSON data.
func memoryOutputGate(kind protocol.ProtocolKind, raw []byte) string {
	type block struct {
		Type string `json:"type"`
	}
	type message struct {
		Type    string          `json:"type"`
		Content json.RawMessage `json:"content"`
		Audio   json.RawMessage `json:"audio"`
	}
	var blocks []block
	switch kind {
	case protocol.ProtocolChatCompletions:
		var object struct {
			Choices []struct {
				Index   int     `json:"index"`
				Message message `json:"message"`
				Delta   message `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(raw, &object); err != nil {
			return "response-parse-failed"
		}
		if len(object.Choices) > 1 {
			return "unsupported-response-choices"
		}
		for _, choice := range object.Choices {
			if choice.Index != 0 {
				return "unsupported-response-choices"
			}
			for _, item := range []message{choice.Message, choice.Delta} {
				if len(item.Audio) > 0 && string(item.Audio) != "null" {
					return "non-text-output"
				}
				if len(item.Content) > 0 && item.Content[0] == '[' {
					var content []block
					if err := json.Unmarshal(item.Content, &content); err != nil {
						return "response-parse-failed"
					}
					blocks = append(blocks, content...)
				}
			}
		}
	case protocol.ProtocolMessages:
		var object struct {
			Content []block `json:"content"`
			Block   *block  `json:"content_block"`
		}
		if err := json.Unmarshal(raw, &object); err != nil {
			return "response-parse-failed"
		}
		blocks = object.Content
		if object.Block != nil {
			blocks = append(blocks, *object.Block)
		}
	case protocol.ProtocolResponses:
		var object struct {
			Type     string    `json:"type"`
			Output   []message `json:"output"`
			Item     *message  `json:"item"`
			Response struct {
				Output []message `json:"output"`
			} `json:"response"`
		}
		if err := json.Unmarshal(raw, &object); err != nil {
			return "response-parse-failed"
		}
		if strings.HasPrefix(object.Type, "response.audio.") || strings.HasPrefix(object.Type, "response.output_audio.") || strings.HasPrefix(object.Type, "response.image_generation_call.") {
			return "non-text-output"
		}
		items := append(object.Output, object.Response.Output...)
		if object.Item != nil {
			items = append(items, *object.Item)
		}
		for _, item := range items {
			switch item.Type {
			case "reasoning":
				continue
			case "function_call", "tool_call", "computer_call", "file_search_call", "web_search_call":
				// The existing Responses adapter records these as tool metadata.
				// Keep scanning: a later media item still bypasses memory capture.
				continue
			case "message":
			default:
				return "non-text-output"
			}
			var content []block
			if err := json.Unmarshal(item.Content, &content); err != nil {
				return "response-parse-failed"
			}
			blocks = append(blocks, content...)
		}
	}
	for _, content := range blocks {
		switch content.Type {
		case "text", "output_text", "refusal", "thinking", "redacted_thinking":
		case "tool_use":
			if kind != protocol.ProtocolMessages {
				return "non-text-output"
			}
		default:
			return "non-text-output"
		}
	}
	return ""
}
