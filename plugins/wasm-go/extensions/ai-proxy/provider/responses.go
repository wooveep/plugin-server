package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/util"
	"github.com/tidwall/gjson"
)

type responsesRequest struct {
	Model                string                       `json:"model"`
	Instructions         string                       `json:"instructions,omitempty"`
	Input                json.RawMessage              `json:"input"`
	Stream               bool                         `json:"stream,omitempty"`
	Temperature          *float64                     `json:"temperature,omitempty"`
	TopP                 *float64                     `json:"top_p,omitempty"`
	PresencePenalty      *float64                     `json:"presence_penalty,omitempty"`
	FrequencyPenalty     *float64                     `json:"frequency_penalty,omitempty"`
	MaxOutputTokens      int                          `json:"max_output_tokens,omitempty"`
	User                 string                       `json:"user,omitempty"`
	Metadata             map[string]string            `json:"metadata,omitempty"`
	Store                *bool                        `json:"store,omitempty"`
	Background           bool                         `json:"background,omitempty"`
	Reasoning            *responsesReasoning          `json:"reasoning,omitempty"`
	PromptCacheKey       string                       `json:"prompt_cache_key,omitempty"`
	PromptCacheRetention string                       `json:"prompt_cache_retention,omitempty"`
	PromptCacheOptions   *responsesPromptCacheOptions `json:"prompt_cache_options,omitempty"`
	Text                 *responsesText               `json:"text,omitempty"`
	Tools                []responsesTool              `json:"tools,omitempty"`
	ToolChoice           json.RawMessage              `json:"tool_choice,omitempty"`
	ParallelToolCalls    *bool                        `json:"parallel_tool_calls,omitempty"`
	PreviousResponseID   string                       `json:"previous_response_id,omitempty"`
	Conversation         any                          `json:"conversation,omitempty"`
}

type responsesReasoning struct {
	Effort          string  `json:"effort,omitempty"`
	Summary         *string `json:"summary,omitempty"`
	GenerateSummary *string `json:"generate_summary,omitempty"`
}

type responsesPromptCacheOptions struct {
	Mode string `json:"mode,omitempty"`
	TTL  string `json:"ttl,omitempty"`
}

type responsesPromptCacheBreakpoint struct {
	Mode string `json:"mode,omitempty"`
	TTL  string `json:"ttl,omitempty"`
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

type responsesInputItem struct {
	Type                  string                          `json:"type"`
	Role                  string                          `json:"role"`
	Content               json.RawMessage                 `json:"content"`
	CallID                string                          `json:"call_id"`
	Name                  string                          `json:"name"`
	Arguments             *string                         `json:"arguments"`
	Output                json.RawMessage                 `json:"output"`
	Summary               []json.RawMessage               `json:"summary,omitempty"`
	EncryptedContent      json.RawMessage                 `json:"encrypted_content,omitempty"`
	PromptCacheBreakpoint *responsesPromptCacheBreakpoint `json:"prompt_cache_breakpoint,omitempty"`
}

type responsesContentPart struct {
	Type                  string                          `json:"type"`
	Text                  *string                         `json:"text,omitempty"`
	ImageURL              string                          `json:"image_url,omitempty"`
	FileID                string                          `json:"file_id,omitempty"`
	Detail                string                          `json:"detail,omitempty"`
	Refusal               *string                         `json:"refusal,omitempty"`
	PromptCacheBreakpoint *responsesPromptCacheBreakpoint `json:"prompt_cache_breakpoint,omitempty"`
}

type responsesChatContentPart struct {
	Type                  string                          `json:"type"`
	Text                  *string                         `json:"text,omitempty"`
	ImageURL              *chatMessageContentImageUrl     `json:"image_url,omitempty"`
	Refusal               *string                         `json:"refusal,omitempty"`
	PromptCacheBreakpoint *responsesPromptCacheBreakpoint `json:"prompt_cache_breakpoint,omitempty"`
}

// Request history has no streaming tool indexes or provider-specific message fields.
type responsesChatInputMessage struct {
	Role                  string                          `json:"role"`
	Content               json.RawMessage                 `json:"content,omitempty"`
	ToolCalls             []responsesChatFunctionCall     `json:"tool_calls,omitempty"`
	ToolCallID            string                          `json:"tool_call_id,omitempty"`
	ReasoningContent      *string                         `json:"reasoning_content,omitempty"`
	PromptCacheBreakpoint *responsesPromptCacheBreakpoint `json:"prompt_cache_breakpoint,omitempty"`
}

type responsesChatFunctionCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function functionCall `json:"function"`
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
	Summary   *[]responsesOutputContent `json:"summary,omitempty"`
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
	Refusal        *string                 `json:"refusal,omitempty"`
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
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, fmt.Errorf("invalid Responses fallback request: %w", err)
	}
	const supported = " model instructions input stream stream_options temperature top_p presence_penalty frequency_penalty max_output_tokens user metadata store background reasoning prompt_cache_key prompt_cache_retention prompt_cache_options text tools tool_choice parallel_tool_calls previous_response_id conversation enable_search search_options web_search_options "
	for name := range fields {
		if !strings.Contains(supported, " "+name+" ") {
			return nil, fmt.Errorf("%s is unsupported in Responses fallback mode", name)
		}
	}
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
	if request.Reasoning != nil && (request.Reasoning.Summary != nil || request.Reasoning.GenerateSummary != nil) {
		return nil, errors.New("reasoning summary is unsupported in Responses fallback mode")
	}

	messages, err := convertResponsesInputToChatMessages(request.Input)
	if err != nil {
		return nil, err
	}
	if request.Instructions != "" {
		content, err := json.Marshal(request.Instructions)
		if err != nil {
			return nil, fmt.Errorf("unable to marshal responses instructions: %w", err)
		}
		messages = append([]responsesChatInputMessage{{
			Role:    roleDeveloper,
			Content: content,
		}}, messages...)
	}

	chatRequest := chatCompletionRequest{
		Model:                request.Model,
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
		Messages           []responsesChatInputMessage  `json:"messages"`
		Temperature        *float64                     `json:"temperature,omitempty"`
		TopP               *float64                     `json:"top_p,omitempty"`
		PresencePenalty    *float64                     `json:"presence_penalty,omitempty"`
		FrequencyPenalty   *float64                     `json:"frequency_penalty,omitempty"`
		ResponseFormat     *responsesChatResponseFormat `json:"response_format,omitempty"`
		Store              *bool                        `json:"store,omitempty"`
		PromptCacheOptions *responsesPromptCacheOptions `json:"prompt_cache_options,omitempty"`
	}{
		chatCompletionRequest: chatRequest,
		Messages:              messages,
		Temperature:           request.Temperature,
		TopP:                  request.TopP,
		PresencePenalty:       request.PresencePenalty,
		FrequencyPenalty:      request.FrequencyPenalty,
		ResponseFormat:        responseFormat,
		Store:                 request.Store,
		PromptCacheOptions:    request.PromptCacheOptions,
	})
}

func convertResponsesInputToChatMessages(input json.RawMessage) ([]responsesChatInputMessage, error) {
	if len(bytes.TrimSpace(input)) == 0 || bytes.Equal(bytes.TrimSpace(input), []byte("null")) {
		return nil, errors.New("input is required for Responses fallback mode")
	}
	var text string
	if err := json.Unmarshal(input, &text); err == nil {
		return []responsesChatInputMessage{{Role: roleUser, Content: input}}, nil
	}

	var items []responsesInputItem
	if err := json.Unmarshal(input, &items); err != nil {
		return nil, fmt.Errorf("unsupported responses input: %w", err)
	}
	messages := make([]responsesChatInputMessage, 0, len(items))
	pending := make(map[string]struct{})
	seen := make(map[string]struct{})
	for _, item := range items {
		switch item.Type {
		case "reasoning":
			if len(item.EncryptedContent) > 0 && !bytes.Equal(bytes.TrimSpace(item.EncryptedContent), []byte("null")) || len(item.Summary) > 0 {
				return nil, errors.New("opaque or summary-only reasoning is unsupported in Responses fallback mode")
			}
			var parts []responsesContentPart
			if json.Unmarshal(item.Content, &parts) != nil || len(parts) == 0 {
				return nil, errors.New("reasoning requires plaintext reasoning_text content in Responses fallback mode")
			}
			var text strings.Builder
			for _, part := range parts {
				if part.Type != "reasoning_text" || part.Text == nil {
					return nil, errors.New("reasoning content cannot be converted without loss")
				}
				text.WriteString(*part.Text)
			}
			if len(messages) == 0 || messages[len(messages)-1].Role != roleAssistant {
				messages = append(messages, responsesChatInputMessage{Role: roleAssistant})
			}
			last := &messages[len(messages)-1]
			value := text.String()
			if last.ReasoningContent != nil {
				value = *last.ReasoningContent + value
			}
			last.ReasoningContent = &value
		case "", "message":
			if len(pending) > 0 && (item.Role != roleAssistant || messages[len(messages)-1].Role != roleAssistant) {
				return nil, errors.New("function_call_output is required before the next message in Responses fallback mode")
			}
			switch item.Role {
			case roleUser, roleAssistant, roleDeveloper, roleSystem:
			default:
				return nil, fmt.Errorf("message role %q is unsupported in Responses fallback mode", item.Role)
			}
			content, err := convertResponsesMessageContent(item.Content, item.Role)
			if err != nil {
				return nil, err
			}
			if item.Role == roleAssistant && len(messages) > 0 {
				last := &messages[len(messages)-1]
				if last.Role == roleAssistant && last.ReasoningContent != nil && last.Content == nil {
					last.Content, last.PromptCacheBreakpoint = content, item.PromptCacheBreakpoint
					continue
				}
			}
			if len(pending) > 0 {
				last := &messages[len(messages)-1]
				if last.Content == nil {
					last.Content = content
					last.PromptCacheBreakpoint = item.PromptCacheBreakpoint
					continue
				}
				// Responses can split one assistant turn around parallel calls. Chat
				// needs one assistant message; keep each original cache boundary on
				// its final content part when consolidating those segments.
				var merged []responsesChatContentPart
				for _, segment := range []responsesChatInputMessage{*last, {Content: content, PromptCacheBreakpoint: item.PromptCacheBreakpoint}} {
					var parts []responsesChatContentPart
					var text string
					if err := json.Unmarshal(segment.Content, &text); err == nil {
						parts = []responsesChatContentPart{{Type: "text", Text: &text}}
					} else if err := json.Unmarshal(segment.Content, &parts); err != nil {
						return nil, fmt.Errorf("unable to combine assistant history: %w", err)
					}
					for _, part := range parts {
						if part.Type == "refusal" {
							return nil, errors.New("Chat cannot combine refusal and other assistant content parts in Responses fallback mode")
						}
					}
					if segment.PromptCacheBreakpoint != nil {
						end := &parts[len(parts)-1]
						if end.PromptCacheBreakpoint != nil && *end.PromptCacheBreakpoint != *segment.PromptCacheBreakpoint {
							return nil, errors.New("conflicting cache breakpoints at one assistant content boundary in Responses fallback mode")
						}
						end.PromptCacheBreakpoint = segment.PromptCacheBreakpoint
					}
					merged = append(merged, parts...)
				}
				last.Content, err = json.Marshal(merged)
				if err != nil {
					return nil, fmt.Errorf("unable to marshal combined assistant history: %w", err)
				}
				last.PromptCacheBreakpoint = nil
				continue
			}
			messages = append(messages, responsesChatInputMessage{Role: item.Role, Content: content, PromptCacheBreakpoint: item.PromptCacheBreakpoint})
		case "function_call":
			if item.CallID == "" || item.Name == "" || item.Arguments == nil {
				return nil, errors.New("function_call requires call_id, name and string arguments in Responses fallback mode")
			}
			if _, exists := seen[item.CallID]; exists {
				return nil, fmt.Errorf("duplicate function_call call_id %q in Responses fallback mode", item.CallID)
			}
			if len(pending) > 0 && messages[len(messages)-1].Role != roleAssistant {
				return nil, errors.New("all function_call_output items must precede the next function_call in Responses fallback mode")
			}
			if len(messages) == 0 || messages[len(messages)-1].Role != roleAssistant {
				messages = append(messages, responsesChatInputMessage{Role: roleAssistant})
			}
			last := &messages[len(messages)-1]
			last.ToolCalls = append(last.ToolCalls, responsesChatFunctionCall{ID: item.CallID, Type: "function", Function: functionCall{Name: item.Name, Arguments: *item.Arguments}})
			pending[item.CallID] = struct{}{}
			seen[item.CallID] = struct{}{}
		case "function_call_output":
			if _, exists := pending[item.CallID]; !exists {
				return nil, fmt.Errorf("function_call_output call_id %q has no matching pending function_call in Responses fallback mode", item.CallID)
			}
			content, err := convertResponsesMessageContent(item.Output, roleTool)
			if err != nil {
				return nil, err
			}
			messages = append(messages, responsesChatInputMessage{Role: roleTool, Content: content, ToolCallID: item.CallID})
			delete(pending, item.CallID)
		default:
			return nil, fmt.Errorf("input item type %q is unsupported in Responses fallback mode", item.Type)
		}
	}
	if len(pending) > 0 {
		return nil, errors.New("function_call_output is required for every function_call in Responses fallback mode")
	}
	return messages, nil
}

func convertResponsesMessageContent(content json.RawMessage, role string) (json.RawMessage, error) {
	if len(bytes.TrimSpace(content)) == 0 || bytes.Equal(bytes.TrimSpace(content), []byte("null")) {
		return nil, errors.New("message or function output content is required in Responses fallback mode")
	}
	var text string
	if err := json.Unmarshal(content, &text); err == nil {
		return json.Marshal(text)
	}

	var parts []responsesContentPart
	if err := json.Unmarshal(content, &parts); err != nil {
		return nil, fmt.Errorf("unsupported responses message content: %w", err)
	}
	if len(parts) == 0 {
		return nil, errors.New("message or function output content must not be empty in Responses fallback mode")
	}
	var builder bytes.Buffer
	chatParts := make([]responsesChatContentPart, 0, len(parts))
	preserveParts := role == roleTool
	for _, part := range parts {
		switch part.Type {
		case "input_text", "output_text":
			if part.Text == nil {
				return nil, errors.New("text content requires a string text field in Responses fallback mode")
			}
			builder.WriteString(*part.Text)
			chatParts = append(chatParts, responsesChatContentPart{Type: "text", Text: part.Text, PromptCacheBreakpoint: part.PromptCacheBreakpoint})
			preserveParts = preserveParts || part.PromptCacheBreakpoint != nil
		case "input_image":
			if role != roleUser {
				return nil, errors.New("input_image is supported only on user messages; Chat tool outputs support text only in Responses fallback mode")
			}
			if part.FileID != "" {
				return nil, errors.New("input_image file_id cannot be resolved to image_url in Responses fallback mode")
			}
			if part.ImageURL == "" {
				return nil, errors.New("input_image requires image_url in Responses fallback mode")
			}
			switch part.Detail {
			case "", "auto", "low", "high", "original":
			default:
				return nil, fmt.Errorf("image detail %q is unsupported in Responses fallback mode", part.Detail)
			}
			chatParts = append(chatParts, responsesChatContentPart{Type: "image_url", ImageURL: &chatMessageContentImageUrl{Url: part.ImageURL, Detail: part.Detail}, PromptCacheBreakpoint: part.PromptCacheBreakpoint})
			preserveParts = true
		case "refusal":
			if role != roleAssistant || part.Refusal == nil || len(parts) != 1 {
				return nil, errors.New("refusal requires a single assistant refusal content part in Responses fallback mode")
			}
			chatParts = append(chatParts, responsesChatContentPart{Type: "refusal", Refusal: part.Refusal})
			preserveParts = true
		default:
			return nil, fmt.Errorf("%s is unsupported in Responses fallback mode", part.Type)
		}
	}
	if preserveParts {
		return json.Marshal(chatParts)
	}
	return json.Marshal(builder.String())
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
	if choice.Message.ReasoningContent != "" {
		response.Output = append(response.Output, reasoningOutput("rs_"+chatResponse.ID, choice.Message.ReasoningContent, response.Status))
	}
	if response.Status != "completed" {
		output.Status = "incomplete"
	}
	if output.Content != nil && len(*output.Content) > 0 {
		output.ID = "msg_" + chatResponse.ID
		response.Output = append(response.Output, output)
	}
	for _, call := range choice.Message.ToolCalls {
		response.Output = append(response.Output, responsesOutputItem{
			Type: "function_call", ID: "fc_" + call.Id, Status: output.Status, CallID: call.Id,
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
	case "insufficient_system_resource", "aborted":
		response.Status = "failed"
		response.Error = json.RawMessage(`{"code":"server_error","message":"The upstream generation was interrupted."}`)
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
		annotations := message.Annotations
		if annotations == nil {
			annotations = util.Ptr(make([]json.RawMessage, 0))
		}
		content = append(content, responsesOutputContent{Type: "output_text", Text: &text, Annotations: annotations})
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
	reasoningIndex *int
	refusalIndex   *int
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

// Fail encodes only the content observed so far. It never closes partial tool
// arguments as executable or invents a successful completion after an error.
func (c *chatCompletionToResponsesStreamConverter) Fail(reason json.RawMessage) ([]byte, error) {
	if c.completed {
		return nil, errors.New("cannot fail an already completed response")
	}
	var output bytes.Buffer
	if err := c.writeCreatedEvent(&output); err != nil {
		return nil, err
	}
	for index := range c.outputs {
		c.outputs[index].Status = "incomplete"
	}
	response := responsesResponse{ID: c.id, Object: "response", CreatedAt: c.createdAt, Status: "failed", Model: c.model, Output: c.outputs, Error: reason, Usage: c.usage}
	if response.Output == nil {
		response.Output = make([]responsesOutputItem, 0)
	}
	c.completed = true
	if err := c.writeEvent(&output, responsesStreamEvent{Type: "response.failed", Response: &response}); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
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
			if refusal := choice.Delta.Refusal; refusal != "" {
				if c.refusalIndex == nil {
					index := len(c.outputs)
					c.refusalIndex = &index
					content := []responsesOutputContent{{Type: "refusal"}}
					c.outputs = append(c.outputs, responsesOutputItem{Type: "message", ID: fmt.Sprintf("msg_%s_%d", c.id, index), Role: roleAssistant, Status: "in_progress", Content: &content})
					if err := c.writeEvent(&output, responsesStreamEvent{Type: "response.output_item.added", OutputIndex: &index, Item: &c.outputs[index]}); err != nil {
						return nil, err
					}
					if err := c.writeEvent(&output, responsesStreamEvent{Type: "response.content_part.added", OutputIndex: &index, ContentIndex: util.Ptr(0), ItemID: c.outputs[index].ID, Part: &content[0]}); err != nil {
						return nil, err
					}
				}
				item := &c.outputs[*c.refusalIndex]
				(*item.Content)[0].Refusal += refusal
				if err := c.writeEvent(&output, responsesStreamEvent{Type: "response.refusal.delta", OutputIndex: c.refusalIndex, ContentIndex: util.Ptr(0), ItemID: item.ID, Delta: &refusal}); err != nil {
					return nil, err
				}
			}
			if text := choice.Delta.ReasoningContent; text != "" {
				if c.reasoningIndex == nil {
					index := len(c.outputs)
					c.reasoningIndex = &index
					c.outputs = append(c.outputs, reasoningOutput(fmt.Sprintf("rs_%s_%d", c.id, index), "", "in_progress"))
					if err := c.writeEvent(&output, responsesStreamEvent{Type: "response.output_item.added", OutputIndex: c.reasoningIndex, Item: &c.outputs[index]}); err != nil {
						return nil, err
					}
				}
				item := &c.outputs[*c.reasoningIndex]
				*(*item.Content)[0].Text += text
				if err := c.writeEvent(&output, responsesStreamEvent{Type: "response.reasoning_text.delta", OutputIndex: c.reasoningIndex, ContentIndex: util.Ptr(0), ItemID: item.ID, Delta: &text}); err != nil {
					return nil, err
				}
			}
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
	if isLastChunk && !c.completed && c.finishReason != "" && len(bytes.TrimSpace(c.buffer)) == 0 {
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
	if c.finishReason == "length" || c.finishReason == "content_filter" || c.finishReason == "insufficient_system_resource" || c.finishReason == "aborted" {
		item.Status = "incomplete"
	}
	return c.writeEvent(output, responsesStreamEvent{Type: "response.output_item.done", OutputIndex: &c.textIndex, Item: item})
}

func (c *chatCompletionToResponsesStreamConverter) writeCompletedEvent(output *bytes.Buffer) error {
	if c.completed {
		return nil
	}
	if c.finishReason == "" {
		return errors.New("chat stream ended without a terminal finish_reason")
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
		if c.refusalIndex != nil && index == *c.refusalIndex {
			item.Status = "completed"
			if response.Status != "completed" {
				item.Status = "incomplete"
			}
			part := &(*item.Content)[0]
			if err := c.writeEvent(output, responsesStreamEvent{Type: "response.refusal.done", OutputIndex: &index, ContentIndex: util.Ptr(0), ItemID: item.ID, Refusal: &part.Refusal}); err != nil {
				return err
			}
			if err := c.writeEvent(output, responsesStreamEvent{Type: "response.content_part.done", OutputIndex: &index, ContentIndex: util.Ptr(0), ItemID: item.ID, Part: part}); err != nil {
				return err
			}
			if err := c.writeEvent(output, responsesStreamEvent{Type: "response.output_item.done", OutputIndex: &index, Item: item}); err != nil {
				return err
			}
			continue
		}
		if item.Type == "reasoning" {
			item.Status = "completed"
			if response.Status != "completed" {
				item.Status = "incomplete"
			}
			if err := c.writeEvent(output, responsesStreamEvent{Type: "response.reasoning_text.done", OutputIndex: &index, ContentIndex: util.Ptr(0), ItemID: item.ID, Text: (*item.Content)[0].Text}); err != nil {
				return err
			}
			if err := c.writeEvent(output, responsesStreamEvent{Type: "response.output_item.done", OutputIndex: &index, Item: item}); err != nil {
				return err
			}
			continue
		}
		if item.Type != "function_call" {
			continue
		}
		item.Status = "completed"
		if response.Status != "completed" {
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

func reasoningOutput(id, text, status string) responsesOutputItem {
	if status == "failed" || status == "cancelled" {
		status = "incomplete"
	}
	if status != "in_progress" && status != "incomplete" {
		status = "completed"
	}
	return responsesOutputItem{Type: "reasoning", ID: id, Status: status, Summary: util.Ptr(make([]responsesOutputContent, 0)), Content: util.Ptr([]responsesOutputContent{{Type: "reasoning_text", Text: &text}})}
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
