package main

import (
	"encoding/json"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/ai/sessionctx"
)

type memoryProtocolAdapter interface {
	InjectContext([]byte, protocol.InjectableContext) ([]byte, error)
	BuildCacheDigest(protocol.RequestParseInput) (protocol.CacheDigestResult, error)
	CaptureResponse(protocol.ResponseCaptureInput) (protocol.NormalizedExchange, error)
	CaptureStream(protocol.ResponseStreamInput) (protocol.NormalizedExchange, error)
}

func memoryProtocolAdapterForKind(kind protocol.ProtocolKind) memoryProtocolAdapter {
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

func memoryProtocolCurrentUserPrompt(kind protocol.ProtocolKind, body []byte) (string, error) {
	switch kind {
	case protocol.ProtocolChatCompletions, protocol.ProtocolMessages:
		fields, err := memoryDecodeJSONObject(body)
		if err != nil {
			return "", err
		}
		return memoryCurrentUserPromptFromMessages(fields["messages"])
	case protocol.ProtocolResponses:
		fields, err := memoryDecodeJSONObject(body)
		if err != nil {
			return "", err
		}
		return memoryCurrentUserPromptFromResponsesInput(fields["input"])
	default:
		return "", nil
	}
}

func memoryCurrentUserPromptFromMessages(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	var messages []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &messages); err != nil {
		return "", err
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" {
			continue
		}
		return memoryTextFromRawContent(messages[i].Content, "text"), nil
	}
	return "", nil
}

func memoryCurrentUserPromptFromResponsesInput(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	var inputText string
	if err := json.Unmarshal(raw, &inputText); err == nil {
		return inputText, nil
	}
	var items []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return "", err
	}
	for i := len(items) - 1; i >= 0; i-- {
		if items[i].Role != "" && items[i].Role != "user" {
			continue
		}
		if text := memoryTextFromRawContent(items[i].Content, "input_text"); text != "" {
			return text, nil
		}
	}
	return "", nil
}

func memoryTextFromRawContent(raw json.RawMessage, textType string) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	var parts []string
	for _, block := range blocks {
		if block.Type != textType {
			continue
		}
		if block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "")
}

func memoryDecodeJSONObject(body []byte) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		fields = map[string]json.RawMessage{}
	}
	return fields, nil
}

func memorySessionUsageFromProtocol(usage protocol.Usage) sessionctx.Usage {
	total := usage.TotalTokens
	if total == 0 && (usage.InputTokens != 0 || usage.OutputTokens != 0) {
		total = usage.InputTokens + usage.OutputTokens
	}
	return sessionctx.Usage{
		PromptTokens:     usage.InputTokens,
		CompletionTokens: usage.OutputTokens,
		TotalTokens:      total,
	}
}

func memoryProtocolInjectableContext(input memoryMessageAssemblyInput) protocol.InjectableContext {
	messages := make([]sessionctx.OpenAIMessage, 0, 1+len(input.Recent))
	if input.MemoryMessage != nil {
		messages = append(messages, *input.MemoryMessage)
	}
	messages = append(messages, input.Recent...)

	contextMessages := make([]protocol.Message, 0, len(messages))
	parts := make([]string, 0, len(messages))
	placement := protocol.InjectionSystem
	for _, message := range messages {
		content := strings.TrimSpace(memoryTextContent(message.Content))
		if content == "" {
			continue
		}
		role := strings.TrimSpace(message.Role)
		if role == "" {
			role = "system"
		}
		if role == "developer" {
			placement = protocol.InjectionDeveloper
		}
		contextMessages = append(contextMessages, protocol.Message{Role: role, Content: content})
		parts = append(parts, role+": "+content)
	}
	return protocol.InjectableContext{
		Text:      strings.Join(parts, "\n"),
		Placement: placement,
		Messages:  contextMessages,
		Source:    "ai-memory",
	}
}
