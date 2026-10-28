package main

import (
	"encoding/json"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/tidwall/gjson"
)

// Output declarations constrain synchronous text auxiliaries. Input attachments
// remain untouched and are not classified by this boundary.
func thinRequestOutputGate(kind protocol.ProtocolKind, request gjson.Result) string {
	modalities := request.Get("modalities")
	if modalities.Exists() && modalities.Raw != "null" {
		if !modalities.IsArray() {
			return "unsupported-output-modalities"
		}
		for _, modality := range modalities.Array() {
			if modality.Type != gjson.String || modality.String() != "text" {
				return "non-text-output"
			}
		}
	}
	if kind == protocol.ProtocolResponses {
		background := request.Get("background")
		if background.Exists() && background.Raw != "null" {
			if background.Type != gjson.True && background.Type != gjson.False {
				return "unsupported-background"
			}
			if background.Bool() {
				return "asynchronous-output"
			}
		}
	}
	return ""
}

func thinReplayOutputGate(kind protocol.ProtocolKind, replay protocol.ReplayPayload) string {
	if len(replay.Body) > 0 {
		return thinOutputGate(kind, replay.Body)
	}
	for _, chunk := range replay.StreamChunks {
		if reason := thinOutputGate(kind, chunk.Data); reason != "" {
			return reason
		}
	}
	return ""
}

// Classify only actual protocol output, matching History's existing boundary.
// This never rejects an image/audio/file attachment in a request.
func thinOutputGate(kind protocol.ProtocolKind, raw []byte) string {
	type block struct {
		Type string `json:"type"`
	}
	type message struct {
		Type    string          `json:"type"`
		Content json.RawMessage `json:"content"`
		Audio   json.RawMessage `json:"audio"`
	}
	var object struct {
		Type    string    `json:"type"`
		Content []block   `json:"content"`
		Output  []message `json:"output"`
		Item    *message  `json:"item"`
		Block   *block    `json:"content_block"`
		Choices []struct {
			Index   int     `json:"index"`
			Message message `json:"message"`
			Delta   message `json:"delta"`
		} `json:"choices"`
		Response struct {
			Output []message `json:"output"`
		} `json:"response"`
	}
	if err := json.Unmarshal(raw, &object); err != nil {
		return "response-parse-failed"
	}
	var blocks []block
	switch kind {
	case protocol.ProtocolChatCompletions:
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
					if json.Unmarshal(item.Content, &content) != nil {
						return "response-parse-failed"
					}
					blocks = append(blocks, content...)
				}
			}
		}
	case protocol.ProtocolMessages:
		blocks = object.Content
		if object.Block != nil {
			blocks = append(blocks, *object.Block)
		}
	case protocol.ProtocolResponses:
		if strings.HasPrefix(object.Type, "response.audio.") || strings.HasPrefix(object.Type, "response.output_audio.") || strings.HasPrefix(object.Type, "response.image_generation_call.") {
			return "non-text-output"
		}
		items := append(object.Output, object.Response.Output...)
		if object.Item != nil {
			items = append(items, *object.Item)
		}
		for _, item := range items {
			if item.Type == "reasoning" {
				continue
			}
			if item.Type != "message" {
				return "non-text-output"
			}
			var content []block
			if json.Unmarshal(item.Content, &content) != nil {
				return "response-parse-failed"
			}
			blocks = append(blocks, content...)
		}
	}
	for _, content := range blocks {
		switch content.Type {
		case "text", "output_text", "refusal", "thinking", "redacted_thinking":
		default:
			return "non-text-output"
		}
	}
	return ""
}
