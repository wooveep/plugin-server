package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/util"
	"github.com/tidwall/gjson"
)

type responsesRequest struct {
	Model                string              `json:"model"`
	Instructions         string              `json:"instructions,omitempty"`
	Input                json.RawMessage     `json:"input"`
	Stream               bool                `json:"stream,omitempty"`
	Temperature          *float64            `json:"temperature,omitempty"`
	TopP                 *float64            `json:"top_p,omitempty"`
	PresencePenalty      *float64            `json:"presence_penalty,omitempty"`
	FrequencyPenalty     *float64            `json:"frequency_penalty,omitempty"`
	MaxOutputTokens      int                 `json:"max_output_tokens,omitempty"`
	User                 string              `json:"user,omitempty"`
	Metadata             map[string]string   `json:"metadata,omitempty"`
	Store                *bool               `json:"store,omitempty"`
	Background           bool                `json:"background,omitempty"`
	Reasoning            *responsesReasoning `json:"reasoning,omitempty"`
	PromptCacheKey       string              `json:"prompt_cache_key,omitempty"`
	PromptCacheRetention string              `json:"prompt_cache_retention,omitempty"`
	PromptCacheOptions   json.RawMessage     `json:"prompt_cache_options,omitempty"`
	Text                 *responsesText      `json:"text,omitempty"`
	Tools                []responsesTool     `json:"tools,omitempty"`
	ToolChoice           json.RawMessage     `json:"tool_choice,omitempty"`
	ParallelToolCalls    *bool               `json:"parallel_tool_calls,omitempty"`
	PreviousResponseID   string              `json:"previous_response_id,omitempty"`
	Conversation         any                 `json:"conversation,omitempty"`
}

type responsesReasoning struct {
	Effort          string  `json:"effort,omitempty"`
	Summary         *string `json:"summary,omitempty"`
	GenerateSummary *string `json:"generate_summary,omitempty"`
}

type responsesText struct {
	Format *responsesTextFormat `json:"format,omitempty"`
}

type responsesTextFormat struct {
	Type string `json:"type"`
	responsesJSONSchema
}

type responsesJSONSchema struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Schema      json.RawMessage `json:"schema,omitempty"`
	Strict      *bool           `json:"strict,omitempty"`
}

type responsesChatResponseFormat struct {
	Type       string               `json:"type"`
	JSONSchema *responsesJSONSchema `json:"json_schema,omitempty"`
}

type responsesTool struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Strict      *bool                  `json:"strict,omitempty"`
}

type responsesMessageInput struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type responsesContentPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type responsesResponse struct {
	ID                string                `json:"id,omitempty"`
	Object            string                `json:"object"`
	CreatedAt         int64                 `json:"created_at,omitempty"`
	Status            string                `json:"status"`
	Model             string                `json:"model,omitempty"`
	FinishReason      string                `json:"finish_reason,omitempty"`
	Output            []responsesOutputItem `json:"output"`
	Usage             *responsesUsage       `json:"usage,omitempty"`
	Error             json.RawMessage       `json:"error,omitempty"`
	IncompleteDetails *responsesIncomplete  `json:"incomplete_details,omitempty"`
}

type responsesOutputItem struct {
	Type      string                    `json:"type"`
	ID        string                    `json:"id,omitempty"`
	Status    string                    `json:"status,omitempty"`
	Role      string                    `json:"role,omitempty"`
	Content   *[]responsesOutputContent `json:"content,omitempty"`
	CallID    string                    `json:"call_id,omitempty"`
	Name      string                    `json:"name,omitempty"`
	Arguments *string                   `json:"arguments,omitempty"`
}

// The stream envelope keeps required event metadata in one typed writer.
// https://developers.openai.com/api/reference/resources/responses/streaming-events
type responsesStreamEvent struct {
	Type           string                  `json:"type"`
	SequenceNumber int                     `json:"sequence_number"`
	Response       *responsesResponse      `json:"response,omitempty"`
	Item           *responsesOutputItem    `json:"item,omitempty"`
	Part           *responsesOutputContent `json:"part,omitempty"`
	OutputIndex    *int                    `json:"output_index,omitempty"`
	ContentIndex   *int                    `json:"content_index,omitempty"`
	ItemID         string                  `json:"item_id,omitempty"`
	Delta          *string                 `json:"delta,omitempty"`
	Arguments      *string                 `json:"arguments,omitempty"`
	Text           *string                 `json:"text,omitempty"`
}

type responsesOutputContent struct {
	Type        string             `json:"type"`
	Text        *string            `json:"text,omitempty"`
	Annotations *[]json.RawMessage `json:"annotations,omitempty"`
	Refusal     string             `json:"refusal,omitempty"`
}

type responsesUsage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
	TotalTokens  int `json:"total_tokens,omitempty"`
}

type responsesIncomplete struct {
	Reason string `json:"reason,omitempty"`
}

type chatCompletionResponseEnvelope struct {
	ID      string                 `json:"id,omitempty"`
	Choices []chatCompletionChoice `json:"choices"`
	Created int64                  `json:"created,omitempty"`
	Model   string                 `json:"model,omitempty"`
	Error   json.RawMessage        `json:"error,omitempty"`
	Usage   *usage                 `json:"usage,omitempty"`
}

// ConvertResponsesRequestToChatCompletion reuses the registered Responses fallback adapter.
func ConvertResponsesRequestToChatCompletion(body []byte) ([]byte, error) {
	return convertResponsesRequestToChatCompletion(body)
}

func convertResponsesRequestToChatCompletion(body []byte) ([]byte, error) {
	if gjson.GetBytes(body, "previous_response_id").Exists() {
		return nil, errors.New("previous_response_id is unsupported in Responses fallback mode")
	}
	if gjson.GetBytes(body, "conversation").Exists() {
		return nil, errors.New("conversation is unsupported in Responses fallback mode")
	}

	var request responsesRequest
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, fmt.Errorf("unable to unmarshal responses request: %w", err)
	}
	if request.Background {
		return nil, errors.New("background is unsupported in Responses fallback mode")
	}
	if options := bytes.TrimSpace(request.PromptCacheOptions); len(options) > 0 && !bytes.Equal(options, []byte("null")) {
		return nil, errors.New("prompt_cache_options is unsupported in Responses fallback mode")
	}
	if request.Reasoning != nil && (request.Reasoning.Summary != nil || request.Reasoning.GenerateSummary != nil) {
		return nil, errors.New("reasoning summary is unsupported in Responses fallback mode")
	}

	messages, err := convertResponsesInputToChatMessages(request.Input)
	if err != nil {
		return nil, err
	}
	if request.Instructions != "" {
		messages = append([]chatMessage{{
			Role:    roleDeveloper,
			Content: request.Instructions,
		}}, messages...)
	}

	chatRequest := chatCompletionRequest{
		Model:                request.Model,
		Messages:             messages,
		Stream:               request.Stream,
		User:                 request.User,
		Metadata:             request.Metadata,
		ParallelToolCalls:    request.ParallelToolCalls,
		PromptCacheKey:       request.PromptCacheKey,
		PromptCacheRetention: request.PromptCacheRetention,
	}
	if request.Reasoning != nil {
		chatRequest.ReasoningEffort = request.Reasoning.Effort
	}
	if request.Stream {
		chatRequest.StreamOptions = &streamOptions{IncludeUsage: true}
	}
	if request.MaxOutputTokens > 0 {
		chatRequest.MaxCompletionTokens = request.MaxOutputTokens
	}
	tools, err := convertResponsesTools(request.Tools)
	if err != nil {
		return nil, err
	}
	chatRequest.Tools = tools
	toolChoice, err := convertResponsesToolChoice(request.ToolChoice)
	if err != nil {
		return nil, err
	}
	chatRequest.ToolChoice = toolChoice
	var responseFormat *responsesChatResponseFormat
	if request.Text != nil && request.Text.Format != nil {
		format := request.Text.Format
		switch format.Type {
		case "text", "json_object":
			responseFormat = &responsesChatResponseFormat{Type: format.Type}
		case "json_schema":
			responseFormat = &responsesChatResponseFormat{Type: format.Type, JSONSchema: &format.responsesJSONSchema}
		default:
			return nil, fmt.Errorf("text.format type %q is unsupported in Responses fallback mode", format.Type)
		}
	}
	// Override the shared Chat scalar fields only at this conversion boundary so
	// explicit zero values survive without changing other providers' defaults.
	return json.Marshal(struct {
		chatCompletionRequest
		Temperature      *float64                     `json:"temperature,omitempty"`
		TopP             *float64                     `json:"top_p,omitempty"`
		PresencePenalty  *float64                     `json:"presence_penalty,omitempty"`
		FrequencyPenalty *float64                     `json:"frequency_penalty,omitempty"`
		ResponseFormat   *responsesChatResponseFormat `json:"response_format,omitempty"`
		Store            *bool                        `json:"store,omitempty"`
	}{
		chatCompletionRequest: chatRequest,
		Temperature:           request.Temperature,
		TopP:                  request.TopP,
		PresencePenalty:       request.PresencePenalty,
		FrequencyPenalty:      request.FrequencyPenalty,
		ResponseFormat:        responseFormat,
		Store:                 request.Store,
	})
}

func convertResponsesInputToChatMessages(input json.RawMessage) ([]chatMessage, error) {
	if len(bytes.TrimSpace(input)) == 0 {
		return nil, errors.New("input is required for Responses fallback mode")
	}
	var text string
	if err := json.Unmarshal(input, &text); err == nil {
		return []chatMessage{{Role: roleUser, Content: text}}, nil
	}

	var items []responsesMessageInput
	if err := json.Unmarshal(input, &items); err != nil {
		return nil, fmt.Errorf("unsupported responses input: %w", err)
	}
	messages := make([]chatMessage, 0, len(items))
	for _, item := range items {
		content, err := convertResponsesMessageContent(item.Content)
		if err != nil {
			return nil, err
		}
		messages = append(messages, chatMessage{
			Role:    item.Role,
			Content: content,
		})
	}
	return messages, nil
}

func convertResponsesMessageContent(content json.RawMessage) (string, error) {
	var text string
	if err := json.Unmarshal(content, &text); err == nil {
		return text, nil
	}

	var parts []responsesContentPart
	if err := json.Unmarshal(content, &parts); err != nil {
		return "", fmt.Errorf("unsupported responses message content: %w", err)
	}
	var builder bytes.Buffer
	for _, part := range parts {
		switch part.Type {
		case "input_text", "output_text":
			builder.WriteString(part.Text)
		default:
			return "", fmt.Errorf("%s is unsupported in Responses fallback mode", part.Type)
		}
	}
	return builder.String(), nil
}

func convertResponsesTools(tools []responsesTool) ([]tool, error) {
	if len(tools) == 0 {
		return nil, nil
	}
	chatTools := make([]tool, 0, len(tools))
	for _, responsesTool := range tools {
		if responsesTool.Type != "function" {
			return nil, fmt.Errorf("%s is unsupported in Responses fallback mode", responsesTool.Type)
		}
		chatTools = append(chatTools, tool{
			Type: "function",
			Function: function{
				Name:        responsesTool.Name,
				Description: responsesTool.Description,
				Parameters:  responsesTool.Parameters,
				Strict:      responsesTool.Strict,
			},
		})
	}
	return chatTools, nil
}

func convertResponsesToolChoice(raw json.RawMessage) (interface{}, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	var choiceString string
	if err := json.Unmarshal(raw, &choiceString); err == nil {
		switch choiceString {
		case "auto", "none", "required":
			return choiceString, nil
		default:
			return nil, fmt.Errorf("tool_choice %q is unsupported in Responses fallback mode", choiceString)
		}
	}

	var choice struct {
		Type string `json:"type"`
		Name string `json:"name,omitempty"`
	}
	if err := json.Unmarshal(raw, &choice); err != nil {
		return nil, fmt.Errorf("unsupported tool_choice in Responses fallback mode: %w", err)
	}
	switch choice.Type {
	case "auto", "none", "required":
		return choice.Type, nil
	case "function":
		if choice.Name == "" {
			return nil, errors.New("function tool_choice requires name in Responses fallback mode")
		}
		return &toolChoice{
			Type: "function",
			Function: function{
				Name: choice.Name,
			},
		}, nil
	default:
		return nil, fmt.Errorf("tool_choice type %q is unsupported in Responses fallback mode", choice.Type)
	}
}

func ConvertChatCompletionResponseToResponses(body []byte) ([]byte, error) {
	return convertChatCompletionResponseToResponses(body)
}

func convertChatCompletionResponseToResponses(body []byte) ([]byte, error) {
	var chatResponse chatCompletionResponseEnvelope
	if err := json.Unmarshal(body, &chatResponse); err != nil {
		return nil, fmt.Errorf("unable to unmarshal chat completions response: %w", err)
	}

	response := responsesResponse{
		ID:        chatResponse.ID,
		Object:    "response",
		CreatedAt: chatResponse.Created,
		Status:    "completed",
		Model:     chatResponse.Model,
		Usage:     convertChatCompletionUsageToResponsesUsage(chatResponse.Usage),
		Output:    make([]responsesOutputItem, 0),
	}

	if len(bytes.TrimSpace(chatResponse.Error)) > 0 && !bytes.Equal(bytes.TrimSpace(chatResponse.Error), []byte("null")) {
		response.Status = "failed"
		response.Error = chatResponse.Error
		return json.Marshal(response)
	}

	if len(chatResponse.Choices) == 0 || chatResponse.Choices[0].Message == nil {
		return nil, errors.New("chat completions response has no assistant message")
	}

	choice := chatResponse.Choices[0]
	if choice.FinishReason != nil {
		applyResponsesFinishStatus(&response, *choice.FinishReason)
	}

	output := convertChatCompletionMessageToResponsesOutput(choice.Message)
	if output.Content != nil && len(*output.Content) > 0 {
		output.ID = "msg_" + chatResponse.ID
		response.Output = []responsesOutputItem{output}
	}
	for _, call := range choice.Message.ToolCalls {
		response.Output = append(response.Output, responsesOutputItem{
			Type: "function_call", ID: "fc_" + call.Id, Status: "completed", CallID: call.Id,
			Name: call.Function.Name, Arguments: &call.Function.Arguments,
		})
	}
	return json.Marshal(response)
}

func convertChatCompletionUsageToResponsesUsage(chatUsage *usage) *responsesUsage {
	if chatUsage == nil {
		return nil
	}
	return &responsesUsage{
		InputTokens:  chatUsage.PromptTokens,
		OutputTokens: chatUsage.CompletionTokens,
		TotalTokens:  chatUsage.TotalTokens,
	}
}

func applyResponsesFinishStatus(response *responsesResponse, finishReason string) {
	switch finishReason {
	case "length":
		response.Status = "incomplete"
		response.IncompleteDetails = &responsesIncomplete{Reason: "max_output_tokens"}
	case "content_filter":
		response.Status = "incomplete"
		response.IncompleteDetails = &responsesIncomplete{Reason: "content_filter"}
	default:
		response.Status = "completed"
	}
}

func convertChatCompletionMessageToResponsesOutput(message *chatMessage) responsesOutputItem {
	content := make([]responsesOutputContent, 0, 1)
	output := responsesOutputItem{Type: "message", Role: roleAssistant, Status: "completed", Content: &content}
	if message.Role != "" {
		output.Role = message.Role
	}
	if message.Refusal != "" {
		content = append(content, responsesOutputContent{Type: "refusal", Refusal: message.Refusal})
	} else if text, ok := chatCompletionContentText(message.Content); ok && text != "" {
		content = append(content, responsesOutputContent{Type: "output_text", Text: &text, Annotations: util.Ptr(make([]json.RawMessage, 0))})
	}
	return output
}

func chatCompletionContentText(content any) (string, bool) {
	switch value := content.(type) {
	case string:
		return value, true
	default:
		return "", false
	}
}

type chatCompletionToResponsesStreamConverter struct {
	buffer         []byte
	id             string
	createdAt      int64
	model          string
	text           bytes.Buffer
	hasText        bool
	textDone       bool
	finishReason   string
	usage          *responsesUsage
	created        bool
	completed      bool
	outputs        []responsesOutputItem
	toolIndexes    map[int]int
	textIndex      int
	sequenceNumber int
}

type ChatCompletionToResponsesStreamConverter = chatCompletionToResponsesStreamConverter

func newChatCompletionToResponsesStreamConverter() *chatCompletionToResponsesStreamConverter {
	return &chatCompletionToResponsesStreamConverter{outputs: make([]responsesOutputItem, 0)}
}

func NewChatCompletionToResponsesStreamConverter() *ChatCompletionToResponsesStreamConverter {
	return newChatCompletionToResponsesStreamConverter()
}

func (c *chatCompletionToResponsesStreamConverter) Convert(chunk []byte, isLastChunk bool) ([]byte, error) {
	return c.convert(chunk, isLastChunk)
}

func (c *chatCompletionToResponsesStreamConverter) convert(chunk []byte, isLastChunk bool) ([]byte, error) {
	payloads := c.extractSSEPayloads(chunk, isLastChunk)
	var output bytes.Buffer
	for _, payload := range payloads {
		switch payload {
		case "":
			continue
		case streamEndDataValue:
			if err := c.writeCompletedEvent(&output); err != nil {
				return nil, err
			}
			continue
		}
		var chatChunk chatCompletionResponseEnvelope
		if err := json.Unmarshal([]byte(payload), &chatChunk); err != nil {
			return nil, fmt.Errorf("unable to unmarshal chat completions stream chunk: %w", err)
		}
		c.captureChunkMetadata(chatChunk)
		if err := c.writeCreatedEvent(&output); err != nil {
			return nil, err
		}
		if chatChunk.Usage != nil {
			c.usage = convertChatCompletionUsageToResponsesUsage(chatChunk.Usage)
		}
		if len(chatChunk.Choices) == 0 {
			continue
		}
		choice := chatChunk.Choices[0]
		if choice.Delta != nil {
			if delta, ok := chatCompletionContentText(choice.Delta.Content); ok && delta != "" {
				if !c.hasText {
					c.textIndex = len(c.outputs)
					content := make([]responsesOutputContent, 0, 1)
					c.outputs = append(c.outputs, responsesOutputItem{Type: "message", ID: fmt.Sprintf("msg_%d", c.textIndex), Role: roleAssistant, Status: "in_progress", Content: &content})
					item := &c.outputs[c.textIndex]
					if err := c.writeEvent(&output, responsesStreamEvent{Type: "response.output_item.added", OutputIndex: &c.textIndex, Item: item}); err != nil {
						return nil, err
					}
					part := responsesOutputContent{Type: "output_text", Text: util.Ptr(""), Annotations: util.Ptr(make([]json.RawMessage, 0))}
					content = append(content, part)
					if err := c.writeEvent(&output, responsesStreamEvent{Type: "response.content_part.added", OutputIndex: &c.textIndex, ContentIndex: util.Ptr(0), ItemID: item.ID, Part: &part}); err != nil {
						return nil, err
					}
					c.hasText = true
				}
				c.text.WriteString(delta)
				item := &c.outputs[c.textIndex]
				*(*item.Content)[0].Text = c.text.String()
				if err := c.writeEvent(&output, responsesStreamEvent{Type: "response.output_text.delta", OutputIndex: &c.textIndex, ContentIndex: util.Ptr(0), ItemID: item.ID, Delta: &delta}); err != nil {
					return nil, err
				}
			}
			for _, call := range choice.Delta.ToolCalls {
				if c.toolIndexes == nil {
					c.toolIndexes = make(map[int]int)
				}
				index, exists := c.toolIndexes[call.Index]
				if !exists {
					index = len(c.outputs)
					c.toolIndexes[call.Index] = index
					id := "fc_" + call.Id
					if call.Id == "" {
						id = fmt.Sprintf("fc_%s_%d", c.id, call.Index)
					}
					c.outputs = append(c.outputs, responsesOutputItem{Type: "function_call", ID: id, Status: "in_progress", CallID: call.Id, Name: call.Function.Name, Arguments: util.Ptr("")})
					if err := c.writeEvent(&output, responsesStreamEvent{Type: "response.output_item.added", OutputIndex: &index, Item: &c.outputs[index]}); err != nil {
						return nil, err
					}
				}
				item := &c.outputs[index]
				if call.Id != "" {
					item.CallID = call.Id
				}
				if call.Function.Name != "" {
					item.Name = call.Function.Name
				}
				*item.Arguments += call.Function.Arguments
				if call.Function.Arguments != "" {
					if err := c.writeEvent(&output, responsesStreamEvent{Type: "response.function_call_arguments.delta", OutputIndex: &index, ItemID: item.ID, Delta: &call.Function.Arguments}); err != nil {
						return nil, err
					}
				}
			}
		}
		if choice.FinishReason != nil {
			c.finishReason = *choice.FinishReason
			if err := c.writeTextDoneEvent(&output); err != nil {
				return nil, err
			}
		}
	}
	if isLastChunk && !c.completed && len(bytes.TrimSpace(c.buffer)) == 0 {
		if err := c.writeCompletedEvent(&output); err != nil {
			return nil, err
		}
	}
	return output.Bytes(), nil
}

func (c *chatCompletionToResponsesStreamConverter) extractSSEPayloads(chunk []byte, isLastChunk bool) []string {
	c.buffer = append(c.buffer, chunk...)
	c.buffer = bytes.ReplaceAll(c.buffer, []byte("\r\n"), []byte("\n"))
	c.buffer = bytes.ReplaceAll(c.buffer, []byte("\r"), []byte("\n"))

	var payloads []string
	for {
		eventEnd := bytes.Index(c.buffer, []byte("\n\n"))
		if eventEnd < 0 {
			break
		}
		eventBlock := c.buffer[:eventEnd]
		c.buffer = c.buffer[eventEnd+2:]
		payloads = append(payloads, sseDataPayload(eventBlock))
	}

	if isLastChunk && len(bytes.TrimSpace(c.buffer)) > 0 {
		payloads = append(payloads, sseDataPayload(c.buffer))
		c.buffer = nil
	}
	return payloads
}

func sseDataPayload(eventBlock []byte) string {
	var dataLines [][]byte
	for _, line := range bytes.Split(eventBlock, []byte("\n")) {
		if !bytes.HasPrefix(line, []byte(streamDataItemKey)) {
			continue
		}
		value := bytes.TrimSpace(line[len(streamDataItemKey):])
		dataLines = append(dataLines, value)
	}
	return string(bytes.Join(dataLines, []byte("\n")))
}

func (c *chatCompletionToResponsesStreamConverter) captureChunkMetadata(chunk chatCompletionResponseEnvelope) {
	if chunk.ID != "" {
		c.id = chunk.ID
	}
	if chunk.Created != 0 {
		c.createdAt = chunk.Created
	}
	if chunk.Model != "" {
		c.model = chunk.Model
	}
}

func (c *chatCompletionToResponsesStreamConverter) writeCreatedEvent(output *bytes.Buffer) error {
	if c.created {
		return nil
	}
	response := responsesResponse{ID: c.id, Object: "response", CreatedAt: c.createdAt, Status: "in_progress", Model: c.model, Output: make([]responsesOutputItem, 0)}
	if err := c.writeEvent(output, responsesStreamEvent{Type: "response.created", Response: &response}); err != nil {
		return err
	}
	c.created = true
	return nil
}

func (c *chatCompletionToResponsesStreamConverter) writeTextDoneEvent(output *bytes.Buffer) error {
	if !c.hasText || c.textDone {
		return nil
	}
	c.textDone = true
	item := &c.outputs[c.textIndex]
	part := &(*item.Content)[0]
	if err := c.writeEvent(output, responsesStreamEvent{Type: "response.output_text.done", OutputIndex: &c.textIndex, ContentIndex: util.Ptr(0), ItemID: item.ID, Text: part.Text}); err != nil {
		return err
	}
	if err := c.writeEvent(output, responsesStreamEvent{Type: "response.content_part.done", OutputIndex: &c.textIndex, ContentIndex: util.Ptr(0), ItemID: item.ID, Part: part}); err != nil {
		return err
	}
	item.Status = "completed"
	if c.finishReason == "length" || c.finishReason == "content_filter" {
		item.Status = "incomplete"
	}
	return c.writeEvent(output, responsesStreamEvent{Type: "response.output_item.done", OutputIndex: &c.textIndex, Item: item})
}

func (c *chatCompletionToResponsesStreamConverter) writeCompletedEvent(output *bytes.Buffer) error {
	if c.completed {
		return nil
	}
	if err := c.writeCreatedEvent(output); err != nil {
		return err
	}
	if err := c.writeTextDoneEvent(output); err != nil {
		return err
	}
	response := responsesResponse{ID: c.id, Object: "response", CreatedAt: c.createdAt, Status: "completed", Model: c.model, FinishReason: c.finishReason, Usage: c.usage}
	if c.finishReason != "" {
		applyResponsesFinishStatus(&response, c.finishReason)
	}
	for index := range c.outputs {
		item := &c.outputs[index]
		if item.Type != "function_call" {
			continue
		}
		item.Status = "completed"
		if response.Status == "incomplete" {
			item.Status = "incomplete"
		}
		if err := c.writeEvent(output, responsesStreamEvent{Type: "response.function_call_arguments.done", OutputIndex: &index, ItemID: item.ID, Arguments: item.Arguments}); err != nil {
			return err
		}
		if err := c.writeEvent(output, responsesStreamEvent{Type: "response.output_item.done", OutputIndex: &index, Item: item}); err != nil {
			return err
		}
	}
	response.Output = c.outputs
	c.completed = true
	return c.writeEvent(output, responsesStreamEvent{Type: "response." + response.Status, Response: &response})
}

func (c *chatCompletionToResponsesStreamConverter) writeEvent(output *bytes.Buffer, event responsesStreamEvent) error {
	event.SequenceNumber = c.sequenceNumber
	encoded, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal responses stream event: %w", err)
	}
	c.sequenceNumber++
	output.WriteString("event: " + event.Type + "\ndata: ")
	output.Write(encoded)
	output.WriteString("\n\n")
	return nil
}
