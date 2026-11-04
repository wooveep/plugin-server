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
		if request.Get("store").Bool() || request.Get("previous_response_id").String() != "" || request.Get("conversation").Exists() && request.Get("conversation").Raw != "null" {
			return "non-replayable-context"
		}
		for _, item := range request.Get("input").Array() {
			if item.Get("type").String() == "item_reference" {
				return "non-replayable-context"
			}
		}
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

// Capture retains tool metadata for the auxiliary event. Replay separately
// rejects that metadata; either path still scans every actual output for media.
type thinOutputFacts struct {
	containsToolCalls bool
	blockedReason     string
}

func cacheableTextTerminal(kind protocol.ProtocolKind, finish string) bool {
	switch kind {
	case protocol.ProtocolResponses:
		return finish == "completed"
	case protocol.ProtocolMessages:
		return finish == "end_turn" || finish == "stop_sequence" || finish == "max_tokens" || finish == "refusal"
	case protocol.ProtocolChatCompletions:
		return finish == "stop" || finish == "length" || finish == "content_filter"
	}
	return false
}

// The current materializer rebuilds assistant text, not signed/reasoning state.
// Keep those real outputs on the original response path instead of flattening
// them into a cache entry or adding retained context to the Redis event stream.
func hasNonReplayableContext(kind protocol.ProtocolKind, raw []byte) bool {
	doc := gjson.ParseBytes(raw)
	if kind == protocol.ProtocolChatCompletions {
		for _, choice := range doc.Get("choices").Array() {
			for _, key := range []string{"message.reasoning_content", "delta.reasoning_content", "message.reasoning", "delta.reasoning"} {
				if choice.Get(key).String() != "" {
					return true
				}
			}
		}
	}
	if kind == protocol.ProtocolMessages {
		blocks := doc.Get("content").Array()
		if block := doc.Get("content_block"); block.Exists() {
			blocks = append(blocks, block)
		}
		for _, block := range blocks {
			if (block.Get("type").String() == "thinking" || block.Get("type").String() == "redacted_thinking") && (block.Get("thinking").String() != "" || block.Get("signature").String() != "" || block.Get("data").String() != "") {
				return true
			}
		}
		delta := doc.Get("delta")
		if delta.Get("type").String() == "thinking_delta" && delta.Get("thinking").String() != "" || delta.Get("type").String() == "signature_delta" && delta.Get("signature").String() != "" {
			return true
		}
	}
	if kind == protocol.ProtocolResponses {
		for _, object := range []gjson.Result{doc, doc.Get("response")} {
			if object.Get("store").Bool() || object.Get("previous_response_id").String() != "" || object.Get("conversation").Exists() && object.Get("conversation").Raw != "null" {
				return true
			}
			items := object.Get("output").Array()
			if item := object.Get("item"); item.Exists() {
				items = append(items, item)
			}
			for _, item := range items {
				if item.Get("type").String() == "reasoning" && (item.Get("encrypted_content").String() != "" || len(item.Get("content").Array()) != 0 || len(item.Get("summary").Array()) != 0) {
					return true
				}
			}
		}
		kind := doc.Get("type").String()
		if (strings.HasPrefix(kind, "response.reasoning_text.") || strings.HasPrefix(kind, "response.reasoning_summary")) && (doc.Get("delta").String() != "" || doc.Get("text").String() != "" || doc.Get("part.text").String() != "") {
			return true
		}
	}
	return false
}

func thinReplayOutputGate(kind protocol.ProtocolKind, replay protocol.ReplayPayload) thinOutputFacts {
	if len(replay.Body) > 0 {
		return thinOutputGate(kind, replay.Body)
	}
	var facts thinOutputFacts
	for _, chunk := range replay.StreamChunks {
		current := thinOutputGate(kind, chunk.Data)
		if current.blockedReason != "" {
			return current
		}
		facts.containsToolCalls = facts.containsToolCalls || current.containsToolCalls
	}
	return facts
}

// Classify only actual protocol output and the existing protocol tool metadata.
// This never rejects an image/audio/file attachment in a request.
func thinOutputGate(kind protocol.ProtocolKind, raw []byte) thinOutputFacts {
	if hasNonReplayableContext(kind, raw) {
		return thinOutputFacts{blockedReason: "non-replayable-context"}
	}
	var facts thinOutputFacts
	type block struct {
		Type string `json:"type"`
	}
	type message struct {
		Type         string            `json:"type"`
		Content      json.RawMessage   `json:"content"`
		Audio        json.RawMessage   `json:"audio"`
		ToolCalls    []json.RawMessage `json:"tool_calls"`
		FunctionCall json.RawMessage   `json:"function_call"`
	}
	var object struct {
		Type       string          `json:"type"`
		Status     string          `json:"status"`
		Error      json.RawMessage `json:"error"`
		Content    []block         `json:"content"`
		Output     []message       `json:"output"`
		Item       *message        `json:"item"`
		Block      *block          `json:"content_block"`
		Delta      json.RawMessage `json:"delta"`
		StopReason string          `json:"stop_reason"`
		Choices    []struct {
			Index        int     `json:"index"`
			FinishReason string  `json:"finish_reason"`
			Message      message `json:"message"`
			Delta        message `json:"delta"`
		} `json:"choices"`
		Response struct {
			Output []message       `json:"output"`
			Status string          `json:"status"`
			Error  json.RawMessage `json:"error"`
		} `json:"response"`
	}
	if err := json.Unmarshal(raw, &object); err != nil {
		return thinOutputFacts{blockedReason: "response-parse-failed"}
	}
	if object.Type == "error" || object.Type == "response.failed" || object.Type == "response.incomplete" ||
		object.Status == "failed" || object.Status == "incomplete" || object.Status == "cancelled" ||
		object.Response.Status == "failed" || object.Response.Status == "incomplete" || object.Response.Status == "cancelled" ||
		len(object.Error) != 0 && string(object.Error) != "null" || len(object.Response.Error) != 0 && string(object.Response.Error) != "null" {
		return thinOutputFacts{blockedReason: "incomplete-response"}
	}
	var blocks []block
	switch kind {
	case protocol.ProtocolChatCompletions:
		if len(object.Choices) > 1 {
			return thinOutputFacts{blockedReason: "unsupported-response-choices"}
		}
		for _, choice := range object.Choices {
			if choice.FinishReason == "tool_calls" || choice.FinishReason == "function_call" {
				facts.containsToolCalls = true
			}
			if choice.Index != 0 {
				return thinOutputFacts{blockedReason: "unsupported-response-choices"}
			}
			for _, item := range []message{choice.Message, choice.Delta} {
				if len(item.ToolCalls) > 0 || len(item.FunctionCall) > 0 && string(item.FunctionCall) != "null" {
					facts.containsToolCalls = true
				}
				if len(item.Audio) > 0 && string(item.Audio) != "null" {
					return thinOutputFacts{blockedReason: "non-text-output"}
				}
				if len(item.Content) > 0 && item.Content[0] == '[' {
					var content []block
					if json.Unmarshal(item.Content, &content) != nil {
						return thinOutputFacts{blockedReason: "response-parse-failed"}
					}
					blocks = append(blocks, content...)
				}
			}
		}
	case protocol.ProtocolMessages:
		var delta struct {
			Type       string `json:"type"`
			StopReason string `json:"stop_reason"`
		}
		if len(object.Delta) > 0 && json.Unmarshal(object.Delta, &delta) != nil {
			return thinOutputFacts{blockedReason: "response-parse-failed"}
		}
		if object.StopReason == "tool_use" || delta.StopReason == "tool_use" || delta.Type == "input_json_delta" {
			facts.containsToolCalls = true
		}
		blocks = object.Content
		if object.Block != nil {
			blocks = append(blocks, *object.Block)
		}
	case protocol.ProtocolResponses:
		if strings.HasPrefix(object.Type, "response.function_call_arguments.") {
			facts.containsToolCalls = true
		}
		if strings.HasPrefix(object.Type, "response.audio.") || strings.HasPrefix(object.Type, "response.output_audio.") || strings.HasPrefix(object.Type, "response.image_generation_call.") {
			return thinOutputFacts{blockedReason: "non-text-output"}
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
				facts.containsToolCalls = true
				continue
			}
			if item.Type != "message" {
				return thinOutputFacts{blockedReason: "non-text-output"}
			}
			var content []block
			if json.Unmarshal(item.Content, &content) != nil {
				return thinOutputFacts{blockedReason: "response-parse-failed"}
			}
			blocks = append(blocks, content...)
		}
	}
	for _, content := range blocks {
		switch content.Type {
		case "text", "output_text", "refusal", "thinking", "redacted_thinking":
		case "tool_use":
			facts.containsToolCalls = true
		default:
			return thinOutputFacts{blockedReason: "non-text-output"}
		}
	}
	return facts
}
