package main

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tiktoken-go/tokenizer"
)

func tokenizerForModel(model string) (tokenizer.Codec, error) {
	return tokenizer.Get(tokenizerEncodingForModel(model))
}

type estimatedTokenUsage struct {
	InputToken  int64
	OutputToken int64
	TotalToken  int64
}

func estimateTextTokenUsage(model, inputText, outputText string) (estimatedTokenUsage, bool) {
	if inputText == "" {
		return estimatedTokenUsage{}, false
	}
	codec, err := tokenizerForModel(model)
	if err != nil {
		return estimatedTokenUsage{}, false
	}
	inputTokens, err := codec.Count(inputText)
	if err != nil {
		return estimatedTokenUsage{}, false
	}
	outputTokens := 0
	if outputText != "" {
		outputTokens, err = codec.Count(outputText)
		if err != nil {
			return estimatedTokenUsage{}, false
		}
	}
	usage := estimatedTokenUsage{
		InputToken:  int64(inputTokens),
		OutputToken: int64(outputTokens),
	}
	usage.TotalToken = usage.InputToken + usage.OutputToken
	if usage.TotalToken <= 0 {
		return estimatedTokenUsage{}, false
	}
	return usage, true
}

func tokenizerEncodingForModel(model string) tokenizer.Encoding {
	normalizedModel := strings.ToLower(strings.TrimSpace(model))
	if isCl100kBaseModel(normalizedModel) {
		return tokenizer.Cl100kBase
	}
	return tokenizer.O200kBase
}

func isCl100kBaseModel(model string) bool {
	for _, family := range []string{
		"gpt-4",
		"gpt-3.5",
		"gpt-35",
		"ft:gpt-4",
		"ft:gpt-3.5",
		"ft:davinci-002",
		"ft:babbage-002",
		"text-embedding-ada-002",
		"davinci-002",
		"babbage-002",
	} {
		if matchesModelFamily(model, family) {
			return true
		}
	}
	return false
}

func matchesModelFamily(model, family string) bool {
	return model == family || strings.HasPrefix(model, family+"-") || strings.HasPrefix(model, family+":")
}

func extractRequestInputText(body []byte) (string, bool) {
	object, ok := decodeJSONObject(body)
	if !ok {
		return "", false
	}

	parts := make([]string, 0)
	appendTextValue(&parts, object["instructions"])
	appendChatMessagesText(&parts, object["messages"])
	appendTextValue(&parts, object["input"])
	appendTextValue(&parts, object["prompt"])

	return joinTextParts(parts)
}

func extractResponseOutputText(body []byte) (string, bool) {
	object, ok := decodeJSONObject(body)
	if !ok {
		return "", false
	}

	if text, ok := extractTextFromValue(object["output_text"]); ok {
		return text, true
	}
	if text, ok := extractResponseOutputTextValue(object["output"]); ok {
		return text, true
	}
	if text, ok := extractChoiceOutputText(object["choices"]); ok {
		return text, true
	}

	return "", false
}

func extractStreamingResponseOutputText(data []byte) (string, bool) {
	parts := make([]string, 0)
	for _, payload := range streamingJSONPayloads(data) {
		object, ok := decodeJSONObject(payload)
		if !ok {
			continue
		}
		appendStreamingOutputText(&parts, object)
	}
	return concatTextParts(parts)
}

func streamingJSONPayloads(data []byte) [][]byte {
	events := bytes.Split(bytes.TrimSpace(wrapper.UnifySSEChunk(data)), []byte("\n\n"))
	payloads := make([][]byte, 0, len(events))
	for _, event := range events {
		payload, ok := streamingEventDataPayload(event)
		if ok {
			payloads = append(payloads, payload)
		}
	}
	return payloads
}

func streamingEventDataPayload(event []byte) ([]byte, bool) {
	event = bytes.TrimSpace(event)
	if len(event) == 0 {
		return nil, false
	}

	if bytes.HasPrefix(event, []byte("data:")) || bytes.Contains(event, []byte("\ndata:")) {
		dataLines := make([][]byte, 0)
		for _, line := range bytes.Split(event, []byte("\n")) {
			line = bytes.TrimSpace(line)
			if len(line) == 0 || bytes.HasPrefix(line, []byte(":")) {
				continue
			}
			if bytes.HasPrefix(line, []byte("data:")) {
				value := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
				if len(value) > 0 {
					dataLines = append(dataLines, value)
				}
			}
		}
		if len(dataLines) == 0 {
			return nil, false
		}
		payload := bytes.TrimSpace(bytes.Join(dataLines, []byte("\n")))
		if bytes.Equal(payload, []byte("[DONE]")) {
			return nil, false
		}
		return payload, true
	}

	if bytes.Equal(event, []byte("[DONE]")) {
		return nil, false
	}
	return event, true
}

func decodeJSONObject(body []byte) (map[string]any, bool) {
	var payload any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, false
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, false
	}

	object, ok := payload.(map[string]any)
	if !ok {
		return nil, false
	}
	return object, true
}

func appendChatMessagesText(parts *[]string, value any) {
	messages, ok := value.([]any)
	if !ok {
		return
	}
	for _, message := range messages {
		messageObject, ok := message.(map[string]any)
		if !ok {
			continue
		}
		appendTextValue(parts, messageObject["content"])
	}
}

func appendResponseOutputText(parts *[]string, value any) {
	outputItems, ok := value.([]any)
	if !ok {
		return
	}
	for _, item := range outputItems {
		itemObject, ok := item.(map[string]any)
		if !ok {
			continue
		}
		appendTextValue(parts, itemObject["content"])
	}
}

func extractResponseOutputTextValue(value any) (string, bool) {
	parts := make([]string, 0)
	appendResponseOutputText(&parts, value)
	return joinTextParts(parts)
}

func appendChoiceOutputText(parts *[]string, value any) {
	choices, ok := value.([]any)
	if !ok {
		return
	}
	for _, choice := range choices {
		choiceObject, ok := choice.(map[string]any)
		if !ok {
			continue
		}
		appendTextValue(parts, choiceObject["text"])
		messageObject, ok := choiceObject["message"].(map[string]any)
		if ok {
			appendTextValue(parts, messageObject["content"])
		}
	}
}

func extractChoiceOutputText(value any) (string, bool) {
	parts := make([]string, 0)
	appendChoiceOutputText(&parts, value)
	return joinTextParts(parts)
}

func appendStreamingOutputText(parts *[]string, object map[string]any) {
	appendStreamingChoiceOutputText(parts, object["choices"])
	appendStreamingDeltaText(parts, object["delta"])
	appendStreamingGeminiCandidateText(parts, object["candidates"])
}

func appendStreamingChoiceOutputText(parts *[]string, value any) {
	choices, ok := value.([]any)
	if !ok {
		return
	}
	for _, choice := range choices {
		choiceObject, ok := choice.(map[string]any)
		if !ok {
			continue
		}
		appendTextValue(parts, choiceObject["text"])
		appendStreamingDeltaText(parts, choiceObject["delta"])
	}
}

func appendStreamingDeltaText(parts *[]string, value any) {
	switch typed := value.(type) {
	case string:
		*parts = append(*parts, typed)
	case map[string]any:
		appendTextValue(parts, typed["content"])
		appendTextValue(parts, typed["text"])
	}
}

func appendStreamingGeminiCandidateText(parts *[]string, value any) {
	candidates, ok := value.([]any)
	if !ok {
		return
	}
	for _, candidate := range candidates {
		candidateObject, ok := candidate.(map[string]any)
		if !ok {
			continue
		}
		contentObject, ok := candidateObject["content"].(map[string]any)
		if ok {
			appendTextValue(parts, contentObject["parts"])
		}
	}
}

func extractTextFromValue(value any) (string, bool) {
	parts := make([]string, 0)
	appendTextValue(&parts, value)
	return joinTextParts(parts)
}

func appendTextValue(parts *[]string, value any) {
	switch typed := value.(type) {
	case string:
		*parts = append(*parts, typed)
	case []any:
		for _, item := range typed {
			appendTextValue(parts, item)
		}
	case map[string]any:
		if isTextContentBlock(typed) {
			appendTextValue(parts, typed["text"])
		}
		appendTextValue(parts, typed["content"])
	}
}

func isTextContentBlock(object map[string]any) bool {
	value, hasType := object["type"]
	if !hasType {
		return true
	}
	blockType, ok := value.(string)
	if !ok {
		return false
	}
	switch blockType {
	case "text", "input_text", "output_text":
		return true
	default:
		return false
	}
}

func joinTextParts(parts []string) (string, bool) {
	joinedParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			joinedParts = append(joinedParts, part)
		}
	}
	if len(joinedParts) == 0 {
		return "", false
	}
	return strings.Join(joinedParts, "\n"), true
}

func concatTextParts(parts []string) (string, bool) {
	var builder strings.Builder
	for _, part := range parts {
		if part != "" {
			builder.WriteString(part)
		}
	}
	if builder.Len() == 0 {
		return "", false
	}
	return builder.String(), true
}
