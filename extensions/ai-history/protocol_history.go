package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/protocol"
)

type historyAdapter interface {
	CaptureResponse(protocol.ResponseCaptureInput) (protocol.NormalizedExchange, error)
	CaptureStream(protocol.ResponseStreamInput) (protocol.NormalizedExchange, error)
}

func historyAdapterFor(kind protocol.ProtocolKind) historyAdapter {
	switch kind {
	case protocol.ProtocolChatCompletions:
		return protocol.ChatCompletionsAdapter{}
	case protocol.ProtocolMessages:
		return protocol.MessagesAdapter{}
	case protocol.ProtocolResponses:
		return protocol.ResponsesAdapter{}
	default:
		return nil
	}
}

type historyRequest struct {
	Fields    map[string]json.RawMessage
	Container string
	Items     []json.RawMessage
	Turn      []json.RawMessage
	Leading   int
	Users     int
}

// History placement is conversation storage logic, not protocol conversion.
// Preserve raw native items instead of the text-only memory injection contract.
func parseHistoryRequest(kind protocol.ProtocolKind, body []byte, model string) (historyRequest, error) {
	var request historyRequest
	if err := json.Unmarshal(body, &request.Fields); err != nil || request.Fields == nil {
		return historyRequest{}, fmt.Errorf("history request must be a JSON object")
	}
	var actualModel string
	if err := json.Unmarshal(request.Fields["model"], &actualModel); err != nil || actualModel != model {
		return historyRequest{}, fmt.Errorf("history model does not match the published public model")
	}
	if raw := request.Fields["modalities"]; len(raw) > 0 {
		var modalities []string
		if err := json.Unmarshal(raw, &modalities); err != nil {
			return historyRequest{}, err
		}
		for _, modality := range modalities {
			if modality != "text" {
				return historyRequest{}, fmt.Errorf("non-text output does not use conversation history")
			}
		}
	}
	request.Container = "messages"
	if kind == protocol.ProtocolResponses {
		request.Container = "input"
		if raw := request.Fields["background"]; len(raw) > 0 {
			var background bool
			if err := json.Unmarshal(raw, &background); err != nil {
				return historyRequest{}, err
			}
			if background {
				return historyRequest{}, fmt.Errorf("asynchronous responses do not use conversation history")
			}
		}
		// The provider owns continuation state in this form; adding a second
		// history would duplicate it. The original request still passes through.
		for _, field := range []string{"previous_response_id", "conversation"} {
			if raw := request.Fields[field]; len(raw) > 0 && string(raw) != "null" && string(raw) != `""` {
				return historyRequest{}, fmt.Errorf("provider-managed history is not injected")
			}
		}
		var text string
		if json.Unmarshal(request.Fields["input"], &text) == nil {
			item, err := json.Marshal(struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}{"user", text})
			if err != nil {
				return historyRequest{}, err
			}
			request.Items = []json.RawMessage{item}
		}
	}
	if request.Items == nil {
		if err := json.Unmarshal(request.Fields[request.Container], &request.Items); err != nil {
			return historyRequest{}, fmt.Errorf("history conversation must be an array: %w", err)
		}
	}
	lastUser := -1
	for i, raw := range request.Items {
		var item struct {
			Role string `json:"role"`
		}
		if err := json.Unmarshal(raw, &item); err != nil || len(raw) == 0 || raw[0] != '{' {
			return historyRequest{}, fmt.Errorf("history conversation item must be an object")
		}
		if item.Role == "user" {
			request.Users++
			lastUser = i
		}
		if i == request.Leading && (item.Role == "system" || item.Role == "developer") {
			request.Leading++
		}
	}
	if lastUser < 0 {
		return historyRequest{}, fmt.Errorf("history conversation has no user message")
	}
	request.Turn = append([]json.RawMessage(nil), request.Items[lastUser:]...)
	return request, nil
}

func (request historyRequest) inject(turns []historyTurn, count int, original []byte) ([]byte, error) {
	if request.Users > 1 || count <= 0 || len(turns) == 0 {
		return original, nil
	}
	if count > len(turns) {
		count = len(turns)
	}
	items := append([]json.RawMessage(nil), request.Items[:request.Leading]...)
	for _, turn := range turns[len(turns)-count:] {
		items = append(items, turn.Input...)
		items = append(items, turn.Output...)
	}
	items = append(items, request.Items[request.Leading:]...)
	encoded, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}
	request.Fields[request.Container] = encoded
	return json.Marshal(request.Fields)
}

// The shared adapter determines the actual text/terminal/tool-call facts. Raw
// JSON output is retained when available; SSE text is collected by that adapter,
// not by a second local SSE parser. Pending tools never become a fake answer.
func historyResponseOutput(kind protocol.ProtocolKind, status int, stream bool, chunks [][]byte) ([]json.RawMessage, string) {
	if status < 200 || status >= 300 {
		return nil, "upstream-http-error"
	}
	adapter := historyAdapterFor(kind)
	var exchange protocol.NormalizedExchange
	var err error
	if stream {
		exchange, err = adapter.CaptureStream(protocol.ResponseStreamInput{StatusCode: status, Chunks: chunks})
	} else {
		var body []byte
		for _, chunk := range chunks {
			body = append(body, chunk...)
		}
		exchange, err = adapter.CaptureResponse(protocol.ResponseCaptureInput{StatusCode: status, Body: body})
	}
	if err != nil || exchange.Response.ParseFailed || exchange.Response.UnsafeContent {
		return nil, "response-parse-failed"
	}
	if exchange.Response.ContainsToolCalls {
		return nil, "pending-tool-result"
	}
	if exchange.Response.Text == "" {
		return nil, "unsupported-or-empty-text-output"
	}
	terminal := false
	switch kind {
	case protocol.ProtocolChatCompletions:
		terminal = exchange.Response.FinishReason == "stop" || exchange.Response.FinishReason == "length"
	case protocol.ProtocolMessages:
		terminal = exchange.Response.FinishReason == "end_turn" || exchange.Response.FinishReason == "stop_sequence" || exchange.Response.FinishReason == "max_tokens"
	case protocol.ProtocolResponses:
		terminal = exchange.Response.FinishReason == "completed" || exchange.Response.FinishReason == "stop"
	}
	if !terminal {
		return nil, "incomplete-text-output"
	}
	if stream {
		for _, frame := range exchange.StreamChunks {
			if reason := historyOutputGate(kind, frame.Data); reason != "" {
				return nil, reason
			}
		}
	} else if reason := historyOutputGate(kind, exchange.Replay.Body); reason != "" {
		return nil, reason
	}
	if !stream {
		var body []byte
		for _, chunk := range chunks {
			body = append(body, chunk...)
		}
		var object map[string]json.RawMessage
		if err := json.Unmarshal(body, &object); err != nil {
			return nil, "response-parse-failed"
		}
		switch kind {
		case protocol.ProtocolChatCompletions:
			var response struct {
				Choices []struct {
					Message json.RawMessage `json:"message"`
				} `json:"choices"`
			}
			if err := json.Unmarshal(body, &response); err != nil || len(response.Choices) != 1 {
				return nil, "unsupported-response-choices"
			}
			return []json.RawMessage{response.Choices[0].Message}, ""
		case protocol.ProtocolMessages:
			output, err := json.Marshal(struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			}{"assistant", object["content"]})
			if err != nil {
				return nil, "response-parse-failed"
			}
			return []json.RawMessage{output}, ""
		case protocol.ProtocolResponses:
			var items []json.RawMessage
			if err := json.Unmarshal(object["output"], &items); err == nil && len(items) > 0 {
				return items, ""
			}
		}
	}
	output, err := json.Marshal(struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}{"assistant", exchange.Response.Text})
	if err != nil {
		return nil, "response-parse-failed"
	}
	return []json.RawMessage{output}, ""
}

// This boundary classifies actual output only. Input image/audio/file blocks
// are deliberately not filtered: their full raw representation belongs to the
// stored conversation. No local protocol conversion or SSE parsing occurs here.
func historyOutputGate(kind protocol.ProtocolKind, raw []byte) string {
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
