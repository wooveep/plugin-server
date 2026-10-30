package protocol

import (
	"encoding/json"
	"strings"
)

type ResponsesAdapter struct{}

func (ResponsesAdapter) InjectContext(body []byte, context InjectableContext) ([]byte, error) {
	if contextExceedsTokenBudget(context) {
		return append([]byte(nil), body...), nil
	}
	fields, err := decodeObject(body)
	if err != nil {
		return nil, err
	}
	switch context.Placement {
	case InjectionInput:
		input, err := responsesInputItems(fields["input"])
		if err != nil {
			return nil, err
		}
		injected := responsesContextInputItems(context)
		if len(injected) == 0 {
			return append([]byte(nil), body...), nil
		}
		input = append(injected, input...)
		inputBody, err := json.Marshal(input)
		if err != nil {
			return nil, err
		}
		fields["input"] = inputBody
	default:
		text := strings.TrimSpace(context.Text)
		if text == "" {
			return append([]byte(nil), body...), nil
		}
		instructions, err := stringField(fields, "instructions")
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(instructions) == "" {
			raw, err := marshalRaw(text)
			if err != nil {
				return nil, err
			}
			fields["instructions"] = raw
		} else {
			raw, err := marshalRaw(instructions + "\n\n" + text)
			if err != nil {
				return nil, err
			}
			fields["instructions"] = raw
		}
	}
	return json.Marshal(fields)
}

func (ResponsesAdapter) CaptureResponse(input ResponseCaptureInput) (NormalizedExchange, error) {
	var response struct {
		OutputText string                `json:"output_text"`
		Output     []responsesOutputItem `json:"output"`
		Status     string                `json:"status"`
		Finish     string                `json:"finish_reason"`
		Usage      responsesUsage        `json:"usage"`
	}
	if err := json.Unmarshal(input.Body, &response); err != nil {
		return NormalizedExchange{}, err
	}
	text := response.OutputText
	if text == "" {
		text = responsesTextFromOutput(response.Output)
	}
	return NormalizedExchange{
		Request: RequestFacts{
			Protocol: ProtocolResponses,
		},
		Response: ResponseText{
			Text:              text,
			FinishReason:      responsesFinishReason(response.Finish, response.Status),
			ContainsToolCalls: responsesOutputContainsToolCall(response.Output),
		},
		Usage:  response.Usage.normalized(),
		Replay: responsesReplay(input.StatusCode, input.Body, nil),
	}, nil
}

func (ResponsesAdapter) CaptureStream(input ResponseStreamInput) (NormalizedExchange, error) {
	parser := newSSEFrameParser()
	capture := responsesStreamCapture{}
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
			Protocol: ProtocolResponses,
			Stream:   true,
		},
		Response: ResponseText{
			Text:              capture.text.String(),
			FinishReason:      capture.finishReason,
			ContainsToolCalls: capture.containsToolCalls,
		},
		Usage:        capture.usage.normalized(),
		StreamChunks: chunks,
		Replay:       responsesReplay(input.StatusCode, nil, chunks),
	}, nil
}

func (ResponsesAdapter) BuildCacheDigest(input RequestParseInput) (CacheDigestResult, error) {
	fields, err := decodeObject(input.Body)
	if err != nil {
		return CacheDigestResult{}, err
	}
	model, err := stringField(fields, "model")
	if err != nil {
		return CacheDigestResult{}, err
	}
	digestInput, payload, err := buildDigestPayload(ProtocolResponses, model, fields, input.ContextPolicyVersion, input.InjectedContextDigest)
	if err != nil {
		return CacheDigestResult{}, err
	}
	digest, err := stableHexDigest(payload)
	if err != nil {
		return CacheDigestResult{}, err
	}
	return CacheDigestResult{Input: digestInput, Digest: digest}, nil
}

func (ResponsesAdapter) SerializeReplay(payload ReplayPayload) (SerializedReplay, error) {
	if err := validateReplayProtocol(payload, ProtocolResponses); err != nil {
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

type responsesInputItem struct {
	Role    string                  `json:"role,omitempty"`
	Content []responsesContentBlock `json:"content,omitempty"`
	Raw     json.RawMessage         `json:"-"`
}

type responsesContentBlock struct {
	Type string          `json:"type,omitempty"`
	Text string          `json:"text,omitempty"`
	Raw  json.RawMessage `json:"-"`
}

func responsesInputItems(raw json.RawMessage) ([]responsesInputItem, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if text == "" {
			return nil, nil
		}
		return []responsesInputItem{{
			Role:    "user",
			Content: []responsesContentBlock{{Type: "input_text", Text: text}},
		}}, nil
	}
	var rawItems []json.RawMessage
	if err := json.Unmarshal(raw, &rawItems); err != nil {
		return nil, err
	}
	items := make([]responsesInputItem, 0, len(rawItems))
	for _, rawItem := range rawItems {
		items = append(items, responsesInputItem{Raw: append(json.RawMessage(nil), rawItem...)})
	}
	return items, nil
}

func responsesContextInputItems(context InjectableContext) []responsesInputItem {
	blocks := responsesContextContentBlocks(context)
	if len(blocks) == 0 {
		return nil
	}
	return []responsesInputItem{{
		Role:    "system",
		Content: blocks,
	}}
}

func responsesContextContentBlocks(context InjectableContext) []responsesContentBlock {
	if len(context.ContentBlocks) > 0 {
		blocks := make([]responsesContentBlock, 0, len(context.ContentBlocks))
		for _, block := range context.ContentBlocks {
			blocks = append(blocks, responsesContentBlock{
				Type: defaultString(block.Type, "input_text"),
				Text: block.Text,
				Raw:  block.Raw,
			})
		}
		return blocks
	}
	if strings.TrimSpace(context.Text) == "" {
		return nil
	}
	return []responsesContentBlock{{Type: "input_text", Text: context.Text}}
}

func (item responsesInputItem) MarshalJSON() ([]byte, error) {
	if len(item.Raw) > 0 {
		return item.Raw, nil
	}
	type alias responsesInputItem
	return json.Marshal(alias(item))
}

func (block responsesContentBlock) MarshalJSON() ([]byte, error) {
	if len(block.Raw) > 0 {
		return block.Raw, nil
	}
	type alias responsesContentBlock
	return json.Marshal(alias(block))
}

type responsesOutputItem struct {
	Type    string `json:"type"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func responsesTextFromOutput(output []responsesOutputItem) string {
	var text strings.Builder
	for _, item := range output {
		for _, block := range item.Content {
			if block.Type == "output_text" {
				text.WriteString(block.Text)
			}
		}
	}
	return text.String()
}

func responsesOutputContainsToolCall(output []responsesOutputItem) bool {
	for _, item := range output {
		switch item.Type {
		case "function_call", "tool_call", "computer_call", "file_search_call", "web_search_call":
			return true
		}
	}
	return false
}

type responsesUsage struct {
	InputTokens          int `json:"input_tokens,omitempty"`
	InputCacheHitTokens  int `json:"input_cache_hit_tokens,omitempty"`
	InputCacheMissTokens int `json:"input_cache_miss_tokens,omitempty"`
	OutputTokens         int `json:"output_tokens,omitempty"`
	TotalTokens          int `json:"total_tokens,omitempty"`
}

func (usage responsesUsage) normalized() Usage {
	total := usage.TotalTokens
	if total == 0 && (usage.InputTokens != 0 || usage.OutputTokens != 0) {
		total = usage.InputTokens + usage.OutputTokens
	}
	cacheHitTokens := 0
	cacheMissTokens := 0
	if usage.InputTokens >= 0 &&
		usage.InputCacheHitTokens >= 0 &&
		usage.InputCacheMissTokens >= 0 &&
		usage.InputCacheHitTokens <= usage.InputTokens &&
		usage.InputCacheMissTokens == usage.InputTokens-usage.InputCacheHitTokens {
		cacheHitTokens = usage.InputCacheHitTokens
		cacheMissTokens = usage.InputCacheMissTokens
	}
	return Usage{
		InputTokens:          usage.InputTokens,
		InputCacheHitTokens:  cacheHitTokens,
		InputCacheMissTokens: cacheMissTokens,
		OutputTokens:         usage.OutputTokens,
		TotalTokens:          total,
	}
}

type responsesStreamCapture struct {
	text              strings.Builder
	usage             responsesUsage
	finishReason      string
	containsToolCalls bool
}

func (capture *responsesStreamCapture) appendFrame(frame sseFrame) (StreamChunk, error) {
	chunk := StreamChunk{
		Protocol:   ProtocolResponses,
		Event:      frame.Event,
		Data:       json.RawMessage(frame.Data),
		Replayable: true,
	}
	var event struct {
		Type string `json:"type"`
		Item struct {
			Type string `json:"type"`
		} `json:"item"`
		Delta    string `json:"delta"`
		Text     string `json:"text"`
		Response struct {
			Status            string                `json:"status"`
			Finish            string                `json:"finish_reason"`
			Output            []responsesOutputItem `json:"output"`
			IncompleteDetails struct {
				Reason string `json:"reason"`
			} `json:"incomplete_details"`
			Usage responsesUsage `json:"usage"`
		} `json:"response"`
	}
	if err := json.Unmarshal([]byte(frame.Data), &event); err != nil {
		return StreamChunk{}, err
	}
	if event.Type == "" {
		event.Type = frame.Event
	}
	switch event.Type {
	case "response.output_item.added":
		if responsesStreamItemIsToolCall(event.Item.Type) {
			capture.containsToolCalls = true
		}
	case "response.output_text.delta":
		capture.text.WriteString(event.Delta)
		chunk.TextDelta = event.Delta
	case "response.output_text.done":
		if capture.text.Len() == 0 && event.Text != "" {
			capture.text.WriteString(event.Text)
		}
	case "response.function_call_arguments.delta", "response.function_call_arguments.done":
		capture.containsToolCalls = true
	case "response.completed":
		if capture.text.Len() == 0 {
			capture.text.WriteString(responsesTextFromOutput(event.Response.Output))
		}
		capture.finishReason = responsesFinishReason(event.Response.Finish, event.Response.Status)
		chunk.FinishReason = capture.finishReason
		capture.usage = event.Response.Usage
	case "response.failed", "response.cancelled", "response.incomplete":
		capture.finishReason = responsesFinishReason(event.Response.IncompleteDetails.Reason, event.Response.Status)
		chunk.FinishReason = capture.finishReason
	}
	if event.Response.Usage != (responsesUsage{}) {
		usage := event.Response.Usage.normalized()
		chunk.Usage = &usage
	}
	return chunk, nil
}

func responsesStreamItemIsToolCall(itemType string) bool {
	switch itemType {
	case "function_call", "tool_call", "computer_call", "file_search_call", "web_search_call":
		return true
	default:
		return false
	}
}

func responsesFinishReason(reason string, status string) string {
	if reason != "" {
		return reason
	}
	return status
}

func responsesReplay(statusCode int, body []byte, chunks []StreamChunk) ReplayPayload {
	if len(chunks) > 0 {
		return ReplayPayload{
			Protocol:     ProtocolResponses,
			StatusCode:   statusCode,
			ContentType:  defaultSSEContentType,
			StreamChunks: chunks,
		}
	}
	return ReplayPayload{
		Protocol:    ProtocolResponses,
		StatusCode:  statusCode,
		ContentType: defaultJSONContentType,
		Body:        append(json.RawMessage(nil), body...),
	}
}

func marshalRaw(value interface{}) (json.RawMessage, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return body, nil
}
