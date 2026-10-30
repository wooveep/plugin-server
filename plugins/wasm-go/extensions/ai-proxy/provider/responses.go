package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/tidwall/gjson"
)

type responsesRequest struct {
	Model              string            `json:"model"`
	Instructions       string            `json:"instructions,omitempty"`
	Input              json.RawMessage   `json:"input"`
	Stream             bool              `json:"stream,omitempty"`
	Temperature        *float64          `json:"temperature,omitempty"`
	TopP               *float64          `json:"top_p,omitempty"`
	PresencePenalty    *float64          `json:"presence_penalty,omitempty"`
	FrequencyPenalty   *float64          `json:"frequency_penalty,omitempty"`
	MaxOutputTokens    int               `json:"max_output_tokens,omitempty"`
	User               string            `json:"user,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
	Tools              []responsesTool   `json:"tools,omitempty"`
	ToolChoice         json.RawMessage   `json:"tool_choice,omitempty"`
	PreviousResponseID string            `json:"previous_response_id,omitempty"`
	Conversation       any               `json:"conversation,omitempty"`
}

type responsesTool struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
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
	ID                string                   `json:"id,omitempty"`
	Object            string                   `json:"object"`
	CreatedAt         int64                    `json:"created_at,omitempty"`
	Status            string                   `json:"status"`
	Model             string                   `json:"model,omitempty"`
	FinishReason      string                   `json:"finish_reason,omitempty"`
	Output            []responsesOutputMessage `json:"output,omitempty"`
	Usage             *responsesUsage          `json:"usage,omitempty"`
	Error             json.RawMessage          `json:"error,omitempty"`
	IncompleteDetails *responsesIncomplete     `json:"incomplete_details,omitempty"`
}

type responsesOutputMessage struct {
	Type    string                   `json:"type"`
	Role    string                   `json:"role"`
	Content []responsesOutputContent `json:"content"`
}

type responsesOutputContent struct {
	Type    string `json:"type"`
	Text    string `json:"text,omitempty"`
	Refusal string `json:"refusal,omitempty"`
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
		Model:      request.Model,
		Messages:   messages,
		Stream:     request.Stream,
		User:       request.User,
		Metadata:   request.Metadata,
		ToolChoice: nil,
		Tools:      nil,
	}
	if request.Stream {
		chatRequest.StreamOptions = &streamOptions{IncludeUsage: true}
	}
	if request.Temperature != nil {
		chatRequest.Temperature = *request.Temperature
	}
	if request.TopP != nil {
		chatRequest.TopP = *request.TopP
	}
	if request.PresencePenalty != nil {
		chatRequest.PresencePenalty = *request.PresencePenalty
	}
	if request.FrequencyPenalty != nil {
		chatRequest.FrequencyPenalty = *request.FrequencyPenalty
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
	return json.Marshal(chatRequest)
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
	if len(output.Content) > 0 {
		response.Output = []responsesOutputMessage{output}
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

func convertChatCompletionMessageToResponsesOutput(message *chatMessage) responsesOutputMessage {
	output := responsesOutputMessage{
		Type:    "message",
		Role:    roleAssistant,
		Content: make([]responsesOutputContent, 0, 1),
	}
	if message.Role != "" {
		output.Role = message.Role
	}
	if message.Refusal != "" {
		output.Content = append(output.Content, responsesOutputContent{
			Type:    "refusal",
			Refusal: message.Refusal,
		})
		return output
	}
	if text, ok := chatCompletionContentText(message.Content); ok {
		output.Content = append(output.Content, responsesOutputContent{
			Type: "output_text",
			Text: text,
		})
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
	buffer       []byte
	id           string
	createdAt    int64
	model        string
	text         bytes.Buffer
	hasText      bool
	textDone     bool
	finishReason string
	usage        *responsesUsage
	completed    bool
}

type ChatCompletionToResponsesStreamConverter = chatCompletionToResponsesStreamConverter

func newChatCompletionToResponsesStreamConverter() *chatCompletionToResponsesStreamConverter {
	return &chatCompletionToResponsesStreamConverter{}
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
		if chatChunk.Usage != nil {
			c.usage = convertChatCompletionUsageToResponsesUsage(chatChunk.Usage)
		}
		if len(chatChunk.Choices) == 0 {
			continue
		}

		choice := chatChunk.Choices[0]
		if choice.Delta != nil {
			if delta, ok := chatCompletionContentText(choice.Delta.Content); ok && delta != "" {
				c.text.WriteString(delta)
				c.hasText = true
				if err := writeResponsesStreamEvent(&output, "response.output_text.delta", map[string]any{
					"type":          "response.output_text.delta",
					"item_id":       "msg_0",
					"output_index":  0,
					"content_index": 0,
					"delta":         delta,
				}); err != nil {
					return nil, err
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

func (c *chatCompletionToResponsesStreamConverter) writeTextDoneEvent(output *bytes.Buffer) error {
	if !c.hasText || c.textDone {
		return nil
	}
	c.textDone = true
	return writeResponsesStreamEvent(output, "response.output_text.done", map[string]any{
		"type":          "response.output_text.done",
		"item_id":       "msg_0",
		"output_index":  0,
		"content_index": 0,
		"text":          c.text.String(),
	})
}

func (c *chatCompletionToResponsesStreamConverter) writeCompletedEvent(output *bytes.Buffer) error {
	if c.completed {
		return nil
	}
	if err := c.writeTextDoneEvent(output); err != nil {
		return err
	}

	response := responsesResponse{
		ID:           c.id,
		Object:       "response",
		CreatedAt:    c.createdAt,
		Status:       "completed",
		Model:        c.model,
		FinishReason: c.finishReason,
		Usage:        c.usage,
	}
	if c.finishReason != "" {
		applyResponsesFinishStatus(&response, c.finishReason)
	}
	if c.hasText {
		response.Output = []responsesOutputMessage{{
			Type: "message",
			Role: roleAssistant,
			Content: []responsesOutputContent{{
				Type: "output_text",
				Text: c.text.String(),
			}},
		}}
	}

	c.completed = true
	return writeResponsesStreamEvent(output, "response.completed", map[string]any{
		"type":     "response.completed",
		"response": response,
	})
}

func writeResponsesStreamEvent(output *bytes.Buffer, eventType string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	output.WriteString("event: ")
	output.WriteString(eventType)
	output.WriteString("\n")
	output.WriteString("data: ")
	output.Write(data)
	output.WriteString("\n\n")
	return nil
}
