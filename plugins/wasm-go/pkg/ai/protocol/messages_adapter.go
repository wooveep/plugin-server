package protocol

import (
	"encoding/json"
	"strings"
)

type MessagesAdapter struct{}

type messagesContentBlock struct {
	Type string          `json:"type,omitempty"`
	Text string          `json:"text,omitempty"`
	Raw  json.RawMessage `json:"-"`
}

func (MessagesAdapter) InjectContext(body []byte, context InjectableContext) ([]byte, error) {
	if contextExceedsTokenBudget(context) {
		return append([]byte(nil), body...), nil
	}
	injected := messagesContextBlocks(context)
	if len(injected) == 0 {
		return append([]byte(nil), body...), nil
	}
	fields, err := decodeObject(body)
	if err != nil {
		return nil, err
	}
	system, err := messagesSystemBlocks(fields["system"])
	if err != nil {
		return nil, err
	}
	system = append(system, injected...)
	systemBody, err := json.Marshal(system)
	if err != nil {
		return nil, err
	}
	fields["system"] = systemBody
	return json.Marshal(fields)
}

func (MessagesAdapter) CaptureResponse(input ResponseCaptureInput) (NormalizedExchange, error) {
	var response struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string        `json:"stop_reason"`
		Usage      messagesUsage `json:"usage"`
	}
	if err := json.Unmarshal(input.Body, &response); err != nil {
		return NormalizedExchange{}, err
	}
	return NormalizedExchange{
		Request: RequestFacts{
			Protocol: ProtocolMessages,
		},
		Response: ResponseText{
			Text:              messagesTextFromContent(response.Content),
			FinishReason:      response.StopReason,
			ContainsToolCalls: messagesContentContainsToolUse(response.Content) || response.StopReason == "tool_use",
		},
		Usage:  response.Usage.normalized(),
		Replay: messagesReplay(input.StatusCode, input.Body, nil),
	}, nil
}

func (MessagesAdapter) CaptureStream(input ResponseStreamInput) (NormalizedExchange, error) {
	parser := newSSEFrameParser()
	capture := messagesStreamCapture{}
	var chunks []StreamChunk
	for _, chunk := range input.Chunks {
		frames, err := parser.Append(chunk)
		if err != nil {
			return NormalizedExchange{}, err
		}
		for _, frame := range frames {
			streamChunk, err := capture.appendFrame(frame)
			if err != nil {
				return NormalizedExchange{}, err
			}
			chunks = append(chunks, streamChunk)
		}
	}
	return NormalizedExchange{
		Request: RequestFacts{
			Protocol: ProtocolMessages,
			Stream:   true,
		},
		Response: ResponseText{
			Text:              capture.text.String(),
			FinishReason:      capture.finishReason,
			ContainsToolCalls: capture.containsToolCalls,
		},
		Usage:        capture.usage.normalized(),
		StreamChunks: chunks,
		Replay:       messagesReplay(input.StatusCode, nil, chunks),
	}, nil
}

func (MessagesAdapter) BuildCacheDigest(input RequestParseInput) (CacheDigestResult, error) {
	fields, err := decodeObject(input.Body)
	if err != nil {
		return CacheDigestResult{}, err
	}
	model, err := stringField(fields, "model")
	if err != nil {
		return CacheDigestResult{}, err
	}
	digestInput, payload, err := buildDigestPayload(ProtocolMessages, model, fields, input.ContextPolicyVersion, input.InjectedContextDigest)
	if err != nil {
		return CacheDigestResult{}, err
	}
	digest, err := stableHexDigest(payload)
	if err != nil {
		return CacheDigestResult{}, err
	}
	return CacheDigestResult{Input: digestInput, Digest: digest}, nil
}

func (MessagesAdapter) SerializeReplay(payload ReplayPayload) (SerializedReplay, error) {
	if err := validateReplayProtocol(payload, ProtocolMessages); err != nil {
		return SerializedReplay{}, err
	}
	if err := payload.Validate(len(payload.StreamChunks) > 0); err != nil {
		return SerializedReplay{}, err
	}
	if len(payload.StreamChunks) == 0 {
		return SerializedReplay{
			StatusCode:  payload.StatusCode,
			ContentType: defaultString(payload.ContentType, defaultJSONContentType),
			Body:        append([]byte(nil), payload.Body...),
		}, nil
	}
	return SerializedReplay{
		StatusCode:  payload.StatusCode,
		ContentType: defaultString(payload.ContentType, defaultSSEContentType),
		Body:        eventSSEReplay(payload.StreamChunks),
	}, nil
}

func messagesSystemBlocks(raw json.RawMessage) ([]messagesContentBlock, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var systemText string
	if err := json.Unmarshal(raw, &systemText); err == nil {
		if systemText == "" {
			return nil, nil
		}
		return []messagesContentBlock{{Type: "text", Text: systemText}}, nil
	}
	var rawBlocks []json.RawMessage
	if err := json.Unmarshal(raw, &rawBlocks); err != nil {
		return nil, err
	}
	blocks := make([]messagesContentBlock, 0, len(rawBlocks))
	for _, rawBlock := range rawBlocks {
		blocks = append(blocks, messagesContentBlock{Raw: append(json.RawMessage(nil), rawBlock...)})
	}
	return blocks, nil
}

func messagesContextBlocks(context InjectableContext) []messagesContentBlock {
	if len(context.ContentBlocks) > 0 {
		blocks := make([]messagesContentBlock, 0, len(context.ContentBlocks))
		for _, block := range context.ContentBlocks {
			blocks = append(blocks, messagesContentBlock{
				Type: defaultString(block.Type, "text"),
				Text: block.Text,
				Raw:  block.Raw,
			})
		}
		return blocks
	}
	if strings.TrimSpace(context.Text) == "" {
		return nil
	}
	return []messagesContentBlock{{Type: "text", Text: context.Text}}
}

func (block messagesContentBlock) MarshalJSON() ([]byte, error) {
	if len(block.Raw) > 0 {
		return block.Raw, nil
	}
	fields := map[string]string{}
	if block.Type != "" {
		fields["type"] = block.Type
	}
	if block.Text != "" {
		fields["text"] = block.Text
	}
	return json.Marshal(fields)
}

type messagesUsage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
	TotalTokens  int `json:"total_tokens,omitempty"`
}

func (usage messagesUsage) normalized() Usage {
	total := usage.TotalTokens
	if total == 0 && (usage.InputTokens != 0 || usage.OutputTokens != 0) {
		total = usage.InputTokens + usage.OutputTokens
	}
	return Usage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		TotalTokens:  total,
	}
}

func messagesTextFromContent(content []struct {
	Type string `json:"type"`
	Text string `json:"text"`
}) string {
	var text strings.Builder
	for _, block := range content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	return text.String()
}

func messagesContentContainsToolUse(content []struct {
	Type string `json:"type"`
	Text string `json:"text"`
}) bool {
	for _, block := range content {
		if block.Type == "tool_use" {
			return true
		}
	}
	return false
}

type messagesStreamCapture struct {
	text              strings.Builder
	usage             messagesUsage
	finishReason      string
	containsToolCalls bool
}

func (capture *messagesStreamCapture) appendFrame(frame sseFrame) (StreamChunk, error) {
	chunk := StreamChunk{
		Protocol:   ProtocolMessages,
		Event:      frame.Event,
		Data:       json.RawMessage(frame.Data),
		Replayable: true,
	}
	var event struct {
		Type    string `json:"type"`
		Message struct {
			Usage messagesUsage `json:"usage"`
		} `json:"message"`
		ContentBlock struct {
			Type string `json:"type"`
		} `json:"content_block"`
		Delta struct {
			Type       string `json:"type"`
			Text       string `json:"text"`
			StopReason string `json:"stop_reason"`
		} `json:"delta"`
		Usage messagesUsage `json:"usage"`
	}
	if err := json.Unmarshal([]byte(frame.Data), &event); err != nil {
		return StreamChunk{}, err
	}
	switch event.Type {
	case "message_start":
		capture.usage.InputTokens = event.Message.Usage.InputTokens
		capture.usage.OutputTokens = event.Message.Usage.OutputTokens
	case "content_block_start":
		if event.ContentBlock.Type == "tool_use" {
			capture.containsToolCalls = true
		}
	case "content_block_delta":
		if event.Delta.Type == "text_delta" {
			capture.text.WriteString(event.Delta.Text)
			chunk.TextDelta = event.Delta.Text
		}
		if strings.Contains(event.Delta.Type, "tool") || strings.Contains(event.Delta.Type, "input_json") {
			capture.containsToolCalls = true
		}
	case "message_delta":
		if event.Delta.StopReason != "" {
			capture.finishReason = event.Delta.StopReason
			chunk.FinishReason = event.Delta.StopReason
			if event.Delta.StopReason == "tool_use" {
				capture.containsToolCalls = true
			}
		}
		if event.Usage.OutputTokens != 0 {
			capture.usage.OutputTokens = event.Usage.OutputTokens
		}
	}
	if event.Usage.InputTokens != 0 {
		capture.usage.InputTokens = event.Usage.InputTokens
	}
	if event.Usage.TotalTokens != 0 {
		capture.usage.TotalTokens = event.Usage.TotalTokens
	}
	if event.Usage.OutputTokens != 0 {
		capture.usage.OutputTokens = event.Usage.OutputTokens
	}
	normalizedUsage := capture.usage.normalized()
	if normalizedUsage != (Usage{}) && (event.Type == "message_start" || event.Type == "message_delta") {
		chunk.Usage = &normalizedUsage
	}
	return chunk, nil
}

func messagesReplay(statusCode int, body []byte, chunks []StreamChunk) ReplayPayload {
	if len(chunks) > 0 {
		return ReplayPayload{
			Protocol:     ProtocolMessages,
			StatusCode:   statusCode,
			ContentType:  defaultSSEContentType,
			StreamChunks: chunks,
		}
	}
	return ReplayPayload{
		Protocol:    ProtocolMessages,
		StatusCode:  statusCode,
		ContentType: defaultJSONContentType,
		Body:        append(json.RawMessage(nil), body...),
	}
}

func eventSSEReplay(chunks []StreamChunk) []byte {
	var body strings.Builder
	for _, chunk := range chunks {
		if !chunk.Replayable || len(chunk.Data) == 0 {
			continue
		}
		if chunk.Event != "" {
			body.WriteString("event: ")
			body.WriteString(chunk.Event)
			body.WriteString("\n")
		}
		body.WriteString("data: ")
		body.Write(chunk.Data)
		body.WriteString("\n\n")
	}
	return []byte(body.String())
}
