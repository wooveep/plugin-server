package protocol

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/sessionctx"
)

const (
	defaultJSONContentType = "application/json; charset=utf-8"
	defaultSSEContentType  = "text/event-stream; charset=utf-8"
)

var errAdapterNotImplemented = errors.New("protocol adapter not implemented")

type RequestParseInput struct {
	Method                string
	Path                  string
	Body                  []byte
	ContextPolicyVersion  string
	InjectedContextDigest string
}

type ResponseCaptureInput struct {
	StatusCode int
	Body       []byte
}

type ResponseStreamInput struct {
	StatusCode int
	Chunks     [][]byte
}

type CacheDigestResult struct {
	Input  CacheDigestInput
	Digest string
}

type SerializedReplay struct {
	StatusCode  int
	ContentType string
	Body        []byte
}

type ChatCompletionsAdapter struct{}

func (ChatCompletionsAdapter) InjectContext(body []byte, context InjectableContext) ([]byte, error) {
	if contextExceedsTokenBudget(context) {
		return append([]byte(nil), body...), nil
	}
	request, err := sessionctx.ParseOpenAIChatRequest(body)
	if err != nil {
		return nil, err
	}
	messages := append([]sessionctx.OpenAIMessage(nil), request.Messages...)
	injected := chatContextMessages(context)
	if len(injected) == 0 {
		return append([]byte(nil), body...), nil
	}
	insertAt := chatInjectionIndex(messages)
	messages = append(messages[:insertAt], append(injected, messages[insertAt:]...)...)
	return sessionctx.ReplaceOpenAIChatMessages(body, messages)
}

func (ChatCompletionsAdapter) CaptureResponse(input ResponseCaptureInput) (NormalizedExchange, error) {
	response, err := sessionctx.ParseOpenAIChatResponse(input.Body)
	if err != nil {
		return NormalizedExchange{}, err
	}
	return NormalizedExchange{
		Request: RequestFacts{
			Protocol: ProtocolChatCompletions,
		},
		Response: ResponseText{
			Text:              response.AssistantContent,
			FinishReason:      sessionctx.ResponseFinishReason(response),
			ContainsToolCalls: sessionctx.ContainsToolUse(response),
		},
		Usage:  chatUsage(response.Usage),
		Replay: chatReplay(input.StatusCode, input.Body, nil),
	}, nil
}

func (ChatCompletionsAdapter) CaptureStream(input ResponseStreamInput) (NormalizedExchange, error) {
	capture := sessionctx.NewStreamCapture(sessionctx.StreamCaptureOptions{})
	parser := newSSEFrameParser()
	var streamChunks []StreamChunk
	for _, chunk := range input.Chunks {
		if err := capture.AppendSSE(chunk); err != nil {
			return NormalizedExchange{}, err
		}
		frames, err := parser.Append(chunk)
		if err != nil {
			return NormalizedExchange{}, err
		}
		for _, frame := range frames {
			if strings.TrimSpace(frame.Data) == "[DONE]" {
				continue
			}
			streamChunk, err := chatStreamChunk([]byte(frame.Data))
			if err != nil {
				return NormalizedExchange{}, err
			}
			streamChunks = append(streamChunks, streamChunk)
		}
	}
	return NormalizedExchange{
		Request: RequestFacts{
			Protocol: ProtocolChatCompletions,
			Stream:   true,
		},
		Response: ResponseText{
			Text:              capture.AssistantContent(),
			FinishReason:      capture.FinishReason(),
			ContainsToolCalls: capture.ContainsToolCalls(),
		},
		Usage:        chatUsage(capture.Usage()),
		StreamChunks: streamChunks,
		Replay:       chatReplay(input.StatusCode, nil, streamChunks),
	}, nil
}

func (ChatCompletionsAdapter) BuildCacheDigest(input RequestParseInput) (CacheDigestResult, error) {
	fields, err := decodeObject(input.Body)
	if err != nil {
		return CacheDigestResult{}, err
	}
	model, err := stringField(fields, "model")
	if err != nil {
		return CacheDigestResult{}, err
	}
	digestInput, payload, err := buildDigestPayload(ProtocolChatCompletions, model, fields, input.ContextPolicyVersion, input.InjectedContextDigest)
	if err != nil {
		return CacheDigestResult{}, err
	}
	digest, err := stableHexDigest(payload)
	if err != nil {
		return CacheDigestResult{}, err
	}
	return CacheDigestResult{Input: digestInput, Digest: digest}, nil
}

func (ChatCompletionsAdapter) SerializeReplay(payload ReplayPayload) (SerializedReplay, error) {
	if err := validateReplayProtocol(payload, ProtocolChatCompletions); err != nil {
		return SerializedReplay{}, err
	}
	if err := payload.Validate(len(payload.StreamChunks) > 0); err != nil {
		return SerializedReplay{}, err
	}
	contentType := defaultString(payload.ContentType, defaultJSONContentType)
	if len(payload.StreamChunks) == 0 {
		return SerializedReplay{
			StatusCode:  payload.StatusCode,
			ContentType: contentType,
			Body:        append([]byte(nil), payload.Body...),
		}, nil
	}
	return SerializedReplay{
		StatusCode:  payload.StatusCode,
		ContentType: defaultString(payload.ContentType, defaultSSEContentType),
		Body:        dataOnlySSEReplay(payload.StreamChunks, true),
	}, nil
}

func chatContextMessages(context InjectableContext) []sessionctx.OpenAIMessage {
	if len(context.Messages) > 0 {
		out := make([]sessionctx.OpenAIMessage, 0, len(context.Messages))
		for _, message := range context.Messages {
			out = append(out, sessionctx.OpenAIMessage{
				Role:    defaultString(message.Role, "system"),
				Content: message.Content,
			})
		}
		return out
	}
	if strings.TrimSpace(context.Text) == "" {
		return nil
	}
	role := "system"
	if context.Placement == InjectionDeveloper {
		role = "developer"
	}
	return []sessionctx.OpenAIMessage{{Role: role, Content: context.Text}}
}

func chatInjectionIndex(messages []sessionctx.OpenAIMessage) int {
	index := 0
	for index < len(messages) {
		if messages[index].Role != "system" && messages[index].Role != "developer" {
			break
		}
		index++
	}
	return index
}

func chatUsage(usage sessionctx.Usage) Usage {
	total := usage.TotalTokens
	if total == 0 && (usage.PromptTokens != 0 || usage.CompletionTokens != 0) {
		total = usage.PromptTokens + usage.CompletionTokens
	}
	return Usage{
		InputTokens:  usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
		TotalTokens:  total,
	}
}

func chatReplay(statusCode int, body []byte, chunks []StreamChunk) ReplayPayload {
	if len(chunks) > 0 {
		return ReplayPayload{
			Protocol:     ProtocolChatCompletions,
			StatusCode:   statusCode,
			ContentType:  defaultSSEContentType,
			StreamChunks: chunks,
		}
	}
	return ReplayPayload{
		Protocol:    ProtocolChatCompletions,
		StatusCode:  statusCode,
		ContentType: defaultJSONContentType,
		Body:        append(json.RawMessage(nil), body...),
	}
}

func chatStreamChunk(data []byte) (StreamChunk, error) {
	chunk := StreamChunk{
		Protocol:   ProtocolChatCompletions,
		Event:      "chat.completion.chunk",
		Data:       append(json.RawMessage(nil), data...),
		Replayable: true,
	}
	var event struct {
		Choices []struct {
			Delta struct {
				Content json.RawMessage `json:"content"`
			} `json:"delta"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage *sessionctx.Usage `json:"usage"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return StreamChunk{}, err
	}
	for _, choice := range event.Choices {
		if chunk.TextDelta == "" {
			chunk.TextDelta = rawString(choice.Delta.Content)
		}
		if choice.FinishReason != "" {
			chunk.FinishReason = choice.FinishReason
		}
	}
	if event.Usage != nil {
		usage := chatUsage(*event.Usage)
		chunk.Usage = &usage
	}
	return chunk, nil
}

func buildDigestPayload(protocol ProtocolKind, model string, fields map[string]json.RawMessage, policyVersion string, contextDigest string) (CacheDigestInput, map[string]interface{}, error) {
	digestFields := make(map[string]json.RawMessage, len(fields))
	canonicalFields := make(map[string]interface{}, len(fields))
	for name, raw := range fields {
		if name == "stream" || name == "model" || len(raw) == 0 {
			continue
		}
		value, err := canonicalValue(raw)
		if err != nil {
			return CacheDigestInput{}, nil, fmt.Errorf("canonicalize %s: %w", name, err)
		}
		canonicalFields[name] = value
		canonicalBody, err := json.Marshal(value)
		if err != nil {
			return CacheDigestInput{}, nil, err
		}
		digestFields[name] = canonicalBody
	}
	input := CacheDigestInput{
		Protocol:              protocol,
		Model:                 model,
		Fields:                digestFields,
		ExcludedFields:        []string{"stream"},
		ContextPolicyVersion:  policyVersion,
		InjectedContextDigest: contextDigest,
	}
	payload := map[string]interface{}{
		"protocol": string(protocol),
		"model":    model,
		"fields":   canonicalFields,
	}
	if policyVersion != "" {
		payload["context_policy_version"] = policyVersion
	}
	if contextDigest != "" {
		payload["injected_context_digest"] = contextDigest
	}
	return input, payload, nil
}

func contextExceedsTokenBudget(context InjectableContext) bool {
	return context.TokenBudget > 0 && context.TokenEstimate > context.TokenBudget
}

func validateReplayProtocol(payload ReplayPayload, expected ProtocolKind) error {
	if payload.Protocol != expected {
		return fmt.Errorf("replay protocol mismatch: expected %s", expected)
	}
	return nil
}

type sseFrame struct {
	Event string
	Data  string
}

type sseFrameParser struct {
	buffer string
}

func newSSEFrameParser() *sseFrameParser {
	return &sseFrameParser{}
}

func (p *sseFrameParser) Append(chunk []byte) ([]sseFrame, error) {
	p.buffer += strings.ReplaceAll(string(chunk), "\r\n", "\n")
	var frames []sseFrame
	for {
		index := strings.Index(p.buffer, "\n\n")
		if index < 0 {
			return frames, nil
		}
		rawFrame := p.buffer[:index]
		p.buffer = p.buffer[index+2:]
		frame := parseSSEFrame(rawFrame)
		if frame.Data != "" {
			frames = append(frames, frame)
		}
	}
}

func parseSSEFrame(rawFrame string) sseFrame {
	var frame sseFrame
	for _, line := range strings.Split(rawFrame, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
			continue
		case strings.HasPrefix(line, "event:"):
			frame.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if frame.Data == "" {
				frame.Data = data
			} else {
				frame.Data += "\n" + data
			}
		}
	}
	return frame
}

func dataOnlySSEReplay(chunks []StreamChunk, includeDone bool) []byte {
	var body strings.Builder
	for _, chunk := range chunks {
		if !chunk.Replayable || len(chunk.Data) == 0 {
			continue
		}
		body.WriteString("data: ")
		body.Write(chunk.Data)
		body.WriteString("\n\n")
	}
	if includeDone {
		body.WriteString("data: [DONE]\n\n")
	}
	return []byte(body.String())
}

func decodeObject(body []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var fields map[string]json.RawMessage
	if err := decoder.Decode(&fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, errors.New("request body must be a JSON object")
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("invalid JSON: multiple values")
		}
		return nil, err
	}
	return fields, nil
}

func stringField(fields map[string]json.RawMessage, name string) (string, error) {
	raw := fields[name]
	if len(raw) == 0 {
		return "", nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string: %w", name, err)
	}
	return value, nil
}

func canonicalValue(raw []byte) (interface{}, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("invalid JSON: multiple values")
		}
		return nil, err
	}
	return value, nil
}

func stableHexDigest(value interface{}) (string, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func rawString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return value
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
