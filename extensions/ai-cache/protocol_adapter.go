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

func thinEventUsage(kind protocol.ProtocolKind, usage protocol.Usage) json.RawMessage {
	if usage == (protocol.Usage{}) {
		return nil
	}

	fields := map[string]int{}
	switch kind {
	case protocol.ProtocolChatCompletions:
		addPositiveIntField(fields, "prompt_tokens", usage.InputTokens)
		addPositiveIntField(fields, "completion_tokens", usage.OutputTokens)
	default:
		addPositiveIntField(fields, "input_tokens", usage.InputTokens)
		addPositiveIntField(fields, "output_tokens", usage.OutputTokens)
	}
	addPositiveIntField(fields, "input_cache_hit_tokens", usage.InputCacheHitTokens)
	addPositiveIntField(fields, "input_cache_miss_tokens", usage.InputCacheMissTokens)
	addPositiveIntField(fields, "total_tokens", usage.TotalTokens)
	if len(fields) == 0 {
		return nil
	}
	body, err := json.Marshal(fields)
	if err != nil {
		return nil
	}
	return body
}

func addPositiveIntField(fields map[string]int, name string, value int) {
	if value > 0 {
		fields[name] = value
	}
}

func thinResponseFactsFromExchange(kind protocol.ProtocolKind, exchange protocol.NormalizedExchange) thinResponseFacts {
	return thinResponseFacts{
		assistantContent:  exchange.Response.Text,
		finishReason:      exchange.Response.FinishReason,
		usage:             thinEventUsage(kind, exchange.Usage),
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
