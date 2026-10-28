package main

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/protocol"
)

type thinProtocolAdapter interface {
	BuildCacheDigest(protocol.RequestParseInput) (protocol.CacheDigestResult, error)
	CaptureResponse(protocol.ResponseCaptureInput) (protocol.NormalizedExchange, error)
	CaptureStream(protocol.ResponseStreamInput) (protocol.NormalizedExchange, error)
	SerializeReplay(protocol.ReplayPayload) (protocol.SerializedReplay, error)
}

func thinProtocolAdapterForKind(kind protocol.ProtocolKind) (thinProtocolAdapter, bool) {
	switch kind {
	case protocol.ProtocolChatCompletions:
		return protocol.ChatCompletionsAdapter{}, true
	case protocol.ProtocolMessages:
		return protocol.MessagesAdapter{}, true
	case protocol.ProtocolResponses:
		return protocol.ResponsesAdapter{}, true
	default:
		return nil, false
	}
}

// Usage is projected from original protocol bytes. Normalized integer fields
// cannot distinguish an absent count from an explicitly reported zero.
func thinEventUsage(kind protocol.ProtocolKind, replay protocol.ReplayPayload) json.RawMessage {
	fields := map[string]json.RawMessage{}
	input, output := "input_tokens", "output_tokens"
	if kind == protocol.ProtocolChatCompletions {
		input, output = "prompt_tokens", "completion_tokens"
	}
	copyFields := func(usage map[string]json.RawMessage, names ...string) {
		for _, name := range names {
			if value, exists := usage[name]; exists {
				fields[name] = value
			}
		}
	}
	type frame struct {
		Type    string                     `json:"type"`
		Usage   map[string]json.RawMessage `json:"usage"`
		Message struct {
			Usage map[string]json.RawMessage `json:"usage"`
		} `json:"message"`
		Response struct {
			Usage map[string]json.RawMessage `json:"usage"`
		} `json:"response"`
	}
	if len(replay.Body) > 0 {
		var response frame
		if json.Unmarshal(replay.Body, &response) != nil {
			return nil
		}
		copyFields(response.Usage, input, output, "total_tokens", "input_cache_hit_tokens", "input_cache_miss_tokens")
	} else {
		for _, chunk := range replay.StreamChunks {
			var response frame
			if json.Unmarshal(chunk.Data, &response) != nil {
				return nil
			}
			event := chunk.Event
			if event == "" {
				event = response.Type
			}
			switch kind {
			case protocol.ProtocolChatCompletions:
				if response.Usage != nil {
					// Each Chat usage object is an actual report, not a patch
					// to synthesize missing fields from an earlier frame.
					clear(fields)
					copyFields(response.Usage, input, output, "total_tokens", "input_cache_hit_tokens", "input_cache_miss_tokens")
				}
			case protocol.ProtocolMessages:
				switch event {
				case "message_start":
					copyFields(response.Message.Usage, input, "input_cache_hit_tokens", "input_cache_miss_tokens")
				case "message_delta":
					// The start frame's output zero is not final usage.
					copyFields(response.Usage, output, "total_tokens")
				}
			case protocol.ProtocolResponses:
				if event == "response.completed" {
					clear(fields)
					copyFields(response.Response.Usage, input, output, "total_tokens", "input_cache_hit_tokens", "input_cache_miss_tokens")
				}
			}
		}
	}
	if len(fields) == 0 {
		return nil
	}
	body, err := json.Marshal(fields)
	if err != nil {
		return nil
	}
	return body
}

func thinResponseFactsFromExchange(kind protocol.ProtocolKind, exchange protocol.NormalizedExchange) thinResponseFacts {
	return thinResponseFacts{
		assistantContent:  exchange.Response.Text,
		finishReason:      exchange.Response.FinishReason,
		usage:             thinEventUsage(kind, exchange.Replay),
		containsToolCalls: exchange.Response.ContainsToolCalls,
		parseFailed:       exchange.Response.ParseFailed,
	}
}

func thinReplayPayload(kind protocol.ProtocolKind, record MaterializedReplayRecord, stream bool) protocol.ReplayPayload {
	payload := protocol.ReplayPayload{
		Protocol:   kind,
		StatusCode: 200,
	}
	if stream {
		payload.ContentType = "text/event-stream; charset=utf-8"
		payload.StreamChunks = thinReplayStreamChunks(kind, record.StreamChunks)
		return payload
	}
	payload.ContentType = "application/json; charset=utf-8"
	payload.Body = append(json.RawMessage(nil), record.Response...)
	return payload
}

func thinReplayStreamChunks(kind protocol.ProtocolKind, chunks []json.RawMessage) []protocol.StreamChunk {
	out := make([]protocol.StreamChunk, 0, len(chunks))
	for _, chunk := range chunks {
		out = append(out, protocol.StreamChunk{
			Protocol:   kind,
			Event:      thinReplayStreamEvent(kind, chunk),
			Data:       append(json.RawMessage(nil), chunk...),
			Replayable: true,
		})
	}
	return out
}

func thinReplayStreamEvent(kind protocol.ProtocolKind, raw json.RawMessage) string {
	if kind == protocol.ProtocolChatCompletions {
		return ""
	}
	var event struct {
		Event string `json:"event"`
		Type  string `json:"type"`
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		return ""
	}
	if strings.TrimSpace(event.Event) != "" {
		return event.Event
	}
	return event.Type
}

// The slot paths are generated only by the protocol query extractor below.
// They refer to the exact text values used to build Text, never matching text
// elsewhere in the request (for example system instructions or tool metadata).
type thinQueryTextSlot struct {
	pointer string
	path    string
}

type thinQueryTextSelection struct {
	Text  string
	Slots []thinQueryTextSlot
}

func thinCurrentUserPrompt(kind protocol.ProtocolKind, body []byte) (thinQueryTextSelection, error) {
	fields, err := decodeJSONObject(body)
	if err != nil {
		return thinQueryTextSelection{}, err
	}
	switch kind {
	case protocol.ProtocolChatCompletions, protocol.ProtocolMessages:
		return thinCurrentUserPromptFromMessages(fields["messages"])
	case protocol.ProtocolResponses:
		return thinCurrentUserPromptFromResponsesInput(fields["input"])
	default:
		return thinQueryTextSelection{}, nil
	}
}

func thinCurrentUserPromptFromMessages(raw json.RawMessage) (thinQueryTextSelection, error) {
	if len(raw) == 0 {
		return thinQueryTextSelection{}, nil
	}
	var messages []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &messages); err != nil {
		return thinQueryTextSelection{}, err
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" {
			continue
		}
		index := strconv.Itoa(i)
		return thinTextFromRawContent(messages[i].Content, "text", thinQueryTextSlot{
			pointer: "/messages/" + index + "/content", path: "messages." + index + ".content",
		}), nil
	}
	return thinQueryTextSelection{}, nil
}

func thinCurrentUserPromptFromResponsesInput(raw json.RawMessage) (thinQueryTextSelection, error) {
	if len(raw) == 0 {
		return thinQueryTextSelection{}, nil
	}
	var inputText string
	if err := json.Unmarshal(raw, &inputText); err == nil {
		return thinQueryTextSelection{Text: inputText, Slots: []thinQueryTextSlot{{pointer: "/input", path: "input"}}}, nil
	}
	var items []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return thinQueryTextSelection{}, err
	}
	latestCandidate := true
	for i := len(items) - 1; i >= 0; i-- {
		if items[i].Role != "" && items[i].Role != "user" {
			continue
		}
		index := strconv.Itoa(i)
		selection := thinTextFromRawContent(items[i].Content, "input_text", thinQueryTextSlot{
			pointer: "/input/" + index + "/content", path: "input." + index + ".content",
		})
		if selection.Text != "" {
			if !latestCandidate {
				// Preserve the existing diagnostic text fallback, but an older
				// question is not a semantic query for a later attachment-only
				// or otherwise unrecognized current input.
				selection.Slots = nil
			}
			return selection, nil
		}
		latestCandidate = false
	}
	return thinQueryTextSelection{}, nil
}

func thinTextFromRawContent(raw json.RawMessage, textType string, contentSlot thinQueryTextSlot) thinQueryTextSelection {
	if len(raw) == 0 {
		return thinQueryTextSelection{}
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return thinQueryTextSelection{Text: text, Slots: []thinQueryTextSlot{contentSlot}}
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return thinQueryTextSelection{}
	}
	var parts []string
	var slots []thinQueryTextSlot
	for i, block := range blocks {
		if block.Type != textType || block.Text == "" {
			continue
		}
		parts = append(parts, block.Text)
		index := strconv.Itoa(i)
		slots = append(slots, thinQueryTextSlot{
			pointer: contentSlot.pointer + "/" + index + "/text", path: contentSlot.path + "." + index + ".text",
		})
	}
	return thinQueryTextSelection{Text: strings.Join(parts, ""), Slots: slots}
}
