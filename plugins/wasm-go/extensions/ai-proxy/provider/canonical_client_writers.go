package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/canonical"
	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
)

type chatClientFunctionWire struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatClientToolCallWire struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function chatClientFunctionWire `json:"function"`
}

type chatClientMessageWire struct {
	ID               string                   `json:"id,omitempty"`
	Role             string                   `json:"role"`
	Content          json.RawMessage          `json:"content"`
	ReasoningContent string                   `json:"reasoning_content,omitempty"`
	Refusal          string                   `json:"refusal,omitempty"`
	Annotations      []json.RawMessage        `json:"annotations,omitempty"`
	ToolCalls        []chatClientToolCallWire `json:"tool_calls,omitempty"`
}

type chatClientChoiceWire struct {
	Index        int                   `json:"index"`
	Message      chatClientMessageWire `json:"message"`
	Logprobs     json.RawMessage       `json:"logprobs,omitempty"`
	FinishReason *string               `json:"finish_reason"`
}

type chatClientResponseWire struct {
	ID          string                 `json:"id,omitempty"`
	Object      string                 `json:"object"`
	Created     int64                  `json:"created,omitempty"`
	Model       string                 `json:"model,omitempty"`
	ServiceTier string                 `json:"service_tier,omitempty"`
	Choices     []chatClientChoiceWire `json:"choices"`
	Usage       json.RawMessage        `json:"usage,omitempty"`
	Error       json.RawMessage        `json:"error,omitempty"`
}

type chatStreamFunctionWire struct {
	Name      *string `json:"name,omitempty"`
	Arguments *string `json:"arguments,omitempty"`
}

type chatStreamToolCallWire struct {
	Index    int                    `json:"index"`
	ID       string                 `json:"id,omitempty"`
	Type     string                 `json:"type,omitempty"`
	Function chatStreamFunctionWire `json:"function"`
}

type chatStreamDeltaWire struct {
	Role             string                   `json:"role,omitempty"`
	Content          *string                  `json:"content,omitempty"`
	ReasoningContent string                   `json:"reasoning_content,omitempty"`
	Refusal          string                   `json:"refusal,omitempty"`
	Annotations      []json.RawMessage        `json:"annotations,omitempty"`
	ToolCalls        []chatStreamToolCallWire `json:"tool_calls,omitempty"`
}

type chatStreamChoiceWire struct {
	Index        int                 `json:"index"`
	Delta        chatStreamDeltaWire `json:"delta"`
	Logprobs     json.RawMessage     `json:"logprobs,omitempty"`
	FinishReason *string             `json:"finish_reason"`
}

type chatStreamChunkWire struct {
	ID          string                 `json:"id,omitempty"`
	Object      string                 `json:"object"`
	Created     int64                  `json:"created,omitempty"`
	Model       string                 `json:"model,omitempty"`
	ServiceTier string                 `json:"service_tier,omitempty"`
	Choices     []chatStreamChoiceWire `json:"choices"`
	Usage       json.RawMessage        `json:"usage,omitempty"`
}

type responsesClientContentWire struct {
	Type        string             `json:"type"`
	Text        *string            `json:"text,omitempty"`
	Annotations *[]json.RawMessage `json:"annotations,omitempty"`
	Refusal     *string            `json:"refusal,omitempty"`
}

type responsesClientOutputItemWire struct {
	ID               string                        `json:"id,omitempty"`
	Type             string                        `json:"type"`
	Status           string                        `json:"status,omitempty"`
	Role             string                        `json:"role,omitempty"`
	Content          *[]responsesClientContentWire `json:"content,omitempty"`
	Summary          *[]responsesClientContentWire `json:"summary,omitempty"`
	EncryptedContent json.RawMessage               `json:"encrypted_content,omitempty"`
	CallID           string                        `json:"call_id,omitempty"`
	Name             string                        `json:"name,omitempty"`
	Arguments        *string                       `json:"arguments,omitempty"`
}

type responsesIncompleteDetailsWire struct {
	Reason string `json:"reason"`
}

type responsesStreamEventWire struct {
	Type           string                         `json:"type"`
	SequenceNumber int                            `json:"sequence_number"`
	Response       json.RawMessage                `json:"response,omitempty"`
	OutputIndex    *int                           `json:"output_index,omitempty"`
	ContentIndex   *int                           `json:"content_index,omitempty"`
	ItemID         string                         `json:"item_id,omitempty"`
	Item           *responsesClientOutputItemWire `json:"item,omitempty"`
	Part           *responsesClientContentWire    `json:"part,omitempty"`
	Delta          *string                        `json:"delta,omitempty"`
	Text           *string                        `json:"text,omitempty"`
	Refusal        *string                        `json:"refusal,omitempty"`
	Arguments      *string                        `json:"arguments,omitempty"`
}

type messagesClientContentWire struct {
	Type      string          `json:"type"`
	Text      *string         `json:"text,omitempty"`
	Thinking  *string         `json:"thinking,omitempty"`
	Signature *string         `json:"signature,omitempty"`
	Data      *string         `json:"data,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
}

type messagesClientResponseWire struct {
	ID           string                      `json:"id"`
	Type         string                      `json:"type"`
	Role         string                      `json:"role"`
	Model        string                      `json:"model"`
	Content      []messagesClientContentWire `json:"content"`
	StopReason   *string                     `json:"stop_reason"`
	StopSequence json.RawMessage             `json:"stop_sequence"`
	Usage        json.RawMessage             `json:"usage,omitempty"`
}

type messagesStreamDeltaWire struct {
	Type         string          `json:"type,omitempty"`
	Text         *string         `json:"text,omitempty"`
	Thinking     *string         `json:"thinking,omitempty"`
	Signature    *string         `json:"signature,omitempty"`
	PartialJSON  *string         `json:"partial_json,omitempty"`
	StopReason   *string         `json:"stop_reason,omitempty"`
	StopSequence json.RawMessage `json:"stop_sequence,omitempty"`
}

type messagesStreamEventWire struct {
	Type         string                      `json:"type"`
	Message      *messagesClientResponseWire `json:"message,omitempty"`
	Index        *int                        `json:"index,omitempty"`
	ContentBlock *messagesClientContentWire  `json:"content_block,omitempty"`
	Delta        *messagesStreamDeltaWire    `json:"delta,omitempty"`
	Usage        json.RawMessage             `json:"usage,omitempty"`
}

// EncodeCanonicalClientResponse encodes a projected CanonicalResponse directly
// into the target client protocol JSON without intermediate Chat JSON conversion,
// dynamic any payloads, or secondary ID minting.
func EncodeCanonicalClientResponse(resp clientresponse.CanonicalResponse) ([]byte, error) {
	switch resp.Protocol {
	case canonical.ProtocolOpenAIChat:
		return encodeChatClientResponse(resp)
	case canonical.ProtocolOpenAIResponses:
		return encodeResponsesClientResponse(resp)
	case canonical.ProtocolAnthropicMessages:
		return encodeMessagesClientResponse(resp)
	default:
		return nil, fmt.Errorf("unsupported canonical client protocol %q", resp.Protocol)
	}
}

func nonNullJSONRaw(raw json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	return raw
}

func formatArgumentsString(args canonical.Arguments) string {
	switch args.Kind {
	case canonical.ArgsText:
		return args.Text
	case canonical.ArgsObject:
		trimmed := bytes.TrimSpace(args.Object)
		if len(trimmed) == 0 {
			return "{}"
		}
		var compact bytes.Buffer
		if json.Compact(&compact, trimmed) == nil {
			return compact.String()
		}
		return string(trimmed)
	default:
		return ""
	}
}

func formatMessagesToolInput(args canonical.Arguments) json.RawMessage {
	var raw []byte
	switch args.Kind {
	case canonical.ArgsObject:
		raw = bytes.TrimSpace(args.Object)
	case canonical.ArgsText:
		raw = bytes.TrimSpace([]byte(args.Text))
	}
	if len(raw) > 0 && raw[0] == '{' && json.Valid(raw) {
		var compact bytes.Buffer
		if json.Compact(&compact, raw) == nil {
			return json.RawMessage(compact.Bytes())
		}
		return json.RawMessage(bytes.Clone(raw))
	}
	return json.RawMessage("{}")
}

func mapChatFinishReason(finish canonical.FinishReason) *string {
	if finish.Category == canonical.FinishNone {
		return nil
	}
	var reason string
	switch finish.Category {
	case canonical.FinishStop:
		reason = "stop"
	case canonical.FinishToolCalls:
		reason = "tool_calls"
	case canonical.FinishLength:
		reason = "length"
	case canonical.FinishContextWindow:
		if finish.RawReason == "model_context_window_exceeded" {
			reason = "model_context_window_exceeded"
		} else {
			reason = "length"
		}
	case canonical.FinishContentFilter:
		reason = "content_filter"
	case canonical.FinishRefusal:
		if finish.RawReason != "" {
			reason = finish.RawReason
		} else {
			reason = "content_filter"
		}
	case canonical.FinishPauseTurn:
		if finish.RawReason != "" {
			reason = finish.RawReason
		} else {
			reason = "pause_turn"
		}
	case canonical.FinishAborted:
		if finish.RawReason != "" {
			reason = finish.RawReason
		} else {
			reason = "aborted"
		}
	case canonical.FinishOverloaded:
		if finish.RawReason != "" {
			reason = finish.RawReason
		} else {
			reason = "insufficient_system_resource"
		}
	case canonical.FinishUnknown:
		if finish.RawReason != "" {
			reason = finish.RawReason
		} else {
			reason = "unknown"
		}
	default:
		reason = "unknown"
	}
	return &reason
}

func mapMessagesStopReasonAndSeq(finish canonical.FinishReason) (*string, json.RawMessage, error) {
	stopSeq := json.RawMessage("null")
	if finish.StopSequence.IsPresent() {
		encodedSeq, err := json.Marshal(finish.StopSequence.Val)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal messages stop_sequence: %w", err)
		}
		stopSeq = encodedSeq
	}
	if finish.Category == canonical.FinishNone {
		return nil, stopSeq, nil
	}
	var reason string
	if finish.StopSequence.IsPresent() && (finish.Category == canonical.FinishStop || finish.RawReason == "stop_sequence") {
		reason = "stop_sequence"
		return &reason, stopSeq, nil
	}
	switch finish.Category {
	case canonical.FinishStop:
		if finish.RawReason == "stop_sequence" {
			reason = "stop_sequence"
		} else {
			reason = "end_turn"
		}
	case canonical.FinishToolCalls:
		reason = "tool_use"
	case canonical.FinishLength:
		reason = "max_tokens"
	case canonical.FinishContextWindow:
		if finish.RawReason == "model_context_window_exceeded" {
			reason = "model_context_window_exceeded"
		} else {
			reason = "max_tokens"
		}
	case canonical.FinishContentFilter, canonical.FinishRefusal:
		reason = "refusal"
	case canonical.FinishPauseTurn:
		reason = "pause_turn"
	case canonical.FinishAborted:
		if finish.RawReason != "" {
			reason = finish.RawReason
		} else {
			reason = "aborted"
		}
	case canonical.FinishOverloaded:
		if finish.RawReason != "" {
			reason = finish.RawReason
		} else {
			reason = "overloaded"
		}
	case canonical.FinishUnknown:
		if finish.RawReason != "" {
			reason = finish.RawReason
		} else {
			reason = "unknown"
		}
	default:
		reason = "unknown"
	}
	return &reason, stopSeq, nil
}

func resolveResponsesStatus(finish canonical.FinishReason, incompleteHint string, rawErr json.RawMessage) (string, string, json.RawMessage) {
	cleanErr := nonNullJSONRaw(rawErr)
	if len(cleanErr) > 0 {
		return "failed", "", cleanErr
	}
	switch finish.Category {
	case canonical.FinishAborted, canonical.FinishOverloaded:
		return "failed", "", json.RawMessage(`{"code":"server_error","message":"The upstream generation was interrupted."}`)
	case canonical.FinishLength, canonical.FinishContextWindow:
		if incompleteHint != "" {
			return "incomplete", incompleteHint, nil
		}
		return "incomplete", "max_output_tokens", nil
	case canonical.FinishContentFilter:
		if incompleteHint != "" {
			return "incomplete", incompleteHint, nil
		}
		return "incomplete", "content_filter", nil
	case canonical.FinishPauseTurn:
		if incompleteHint != "" {
			return "incomplete", incompleteHint, nil
		}
		return "incomplete", "pause_turn", nil
	}
	if incompleteHint != "" {
		return "incomplete", incompleteHint, nil
	}
	return "completed", "", nil
}

func buildResponsesResourceJSON(
	publicID string,
	createdAt int64,
	status string,
	publicModel string,
	serviceTier string,
	output []responsesClientOutputItemWire,
	wireUsage json.RawMessage,
	incompleteReason string,
	errRaw json.RawMessage,
	echoFields map[string]json.RawMessage,
) (json.RawMessage, error) {
	envelope := make(map[string]json.RawMessage, len(echoFields)+10)
	for k, v := range echoFields {
		if len(bytes.TrimSpace(v)) > 0 {
			envelope[k] = v
		}
	}
	if output == nil {
		output = make([]responsesClientOutputItemWire, 0)
	}
	encodedOutput, err := json.Marshal(output)
	if err != nil {
		return nil, fmt.Errorf("marshal responses output: %w", err)
	}
	encodedStatus, err := json.Marshal(status)
	if err != nil {
		return nil, fmt.Errorf("marshal responses status: %w", err)
	}
	envelope["object"] = json.RawMessage(`"response"`)
	envelope["status"] = encodedStatus
	envelope["output"] = encodedOutput

	if publicID != "" {
		encodedID, err := json.Marshal(publicID)
		if err != nil {
			return nil, fmt.Errorf("marshal responses id: %w", err)
		}
		envelope["id"] = encodedID
	} else {
		delete(envelope, "id")
	}

	if createdAt > 0 {
		envelope["created_at"] = json.RawMessage(strconv.FormatInt(createdAt, 10))
	} else {
		delete(envelope, "created_at")
	}

	if publicModel != "" {
		encodedModel, err := json.Marshal(publicModel)
		if err != nil {
			return nil, fmt.Errorf("marshal responses model: %w", err)
		}
		envelope["model"] = encodedModel
	} else {
		delete(envelope, "model")
	}

	if serviceTier != "" {
		encodedTier, err := json.Marshal(serviceTier)
		if err != nil {
			return nil, fmt.Errorf("marshal responses service_tier: %w", err)
		}
		envelope["service_tier"] = encodedTier
	} else {
		delete(envelope, "service_tier")
	}

	if cleanUsage := nonNullJSONRaw(wireUsage); len(cleanUsage) > 0 {
		envelope["usage"] = cleanUsage
	} else {
		envelope["usage"] = json.RawMessage("null")
	}

	if incompleteReason != "" {
		encodedInc, err := json.Marshal(responsesIncompleteDetailsWire{Reason: incompleteReason})
		if err != nil {
			return nil, fmt.Errorf("marshal responses incomplete_details: %w", err)
		}
		envelope["incomplete_details"] = encodedInc
	} else {
		delete(envelope, "incomplete_details")
	}

	if cleanErr := nonNullJSONRaw(errRaw); len(cleanErr) > 0 {
		envelope["error"] = cleanErr
	}

	encoded, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("marshal responses envelope: %w", err)
	}
	return encoded, nil
}

func encodeChatClientResponse(resp clientresponse.CanonicalResponse) ([]byte, error) {
	choices := make([]chatClientChoiceWire, 0, len(resp.Choices))
	for _, ch := range resp.Choices {
		var textBuf, reasoningBuf, refusalBuf strings.Builder
		var hasTextPart bool
		var msgID string
		role := string(canonical.RoleAssistant)
		var annotations []json.RawMessage
		var toolCalls []chatClientToolCallWire

		for _, pi := range ch.Items {
			switch pi.Item.Kind {
			case canonical.ItemReasoning:
				if pi.Item.Reasoning != nil {
					for _, seg := range pi.Item.Reasoning.Text {
						reasoningBuf.WriteString(seg)
					}
				}
			case canonical.ItemMessage:
				if pi.Item.Role != "" {
					role = string(pi.Item.Role)
				}
				if msgID == "" {
					if pi.PublicItemID != "" {
						msgID = pi.PublicItemID
					} else if pi.Item.MessageID != "" {
						msgID = pi.Item.MessageID
					}
				}
				for _, part := range pi.Item.Parts {
					switch part.Kind {
					case canonical.PartText:
						hasTextPart = true
						textBuf.WriteString(part.Text)
						for _, ann := range part.Annotations {
							if cleanAnn := nonNullJSONRaw(ann); len(cleanAnn) > 0 {
								annotations = append(annotations, cleanAnn)
							}
						}
					case canonical.PartRefusal:
						if part.Refusal != "" {
							refusalBuf.WriteString(part.Refusal)
						}
					}
				}
			case canonical.ItemFunctionCall:
				if pi.Item.Call != nil {
					callID := pi.PublicCallID
					if callID == "" {
						callID = pi.Item.Call.CallID
					}
					toolCalls = append(toolCalls, chatClientToolCallWire{
						ID:   callID,
						Type: "function",
						Function: chatClientFunctionWire{
							Name:      pi.Item.Call.Name,
							Arguments: formatArgumentsString(pi.Item.Call.Args),
						},
					})
				}
			}
		}

		refusalStr := refusalBuf.String()
		var contentRaw json.RawMessage
		if !hasTextPart && (len(toolCalls) > 0 || refusalStr != "") {
			contentRaw = json.RawMessage("null")
		} else {
			encodedText, err := json.Marshal(textBuf.String())
			if err != nil {
				return nil, fmt.Errorf("marshal chat message content: %w", err)
			}
			contentRaw = encodedText
		}

		choices = append(choices, chatClientChoiceWire{
			Index: ch.Index,
			Message: chatClientMessageWire{
				ID:               msgID,
				Role:             role,
				Content:          contentRaw,
				ReasoningContent: reasoningBuf.String(),
				Refusal:          refusalStr,
				Annotations:      annotations,
				ToolCalls:        toolCalls,
			},
			Logprobs:     nonNullJSONRaw(ch.Logprobs),
			FinishReason: mapChatFinishReason(ch.Finish),
		})
	}

	wire := chatClientResponseWire{
		ID:          resp.PublicID,
		Object:      "chat.completion",
		Created:     resp.CreatedAt,
		Model:       resp.PublicModel,
		ServiceTier: resp.ServiceTier,
		Choices:     choices,
		Usage:       nonNullJSONRaw(resp.WireUsage),
		Error:       nonNullJSONRaw(resp.Error),
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("marshal chat client response: %w", err)
	}
	return encoded, nil
}

func encodeResponsesClientResponse(resp clientresponse.CanonicalResponse) ([]byte, error) {
	var primaryFinish canonical.FinishReason
	if len(resp.Choices) > 0 {
		primaryFinish = resp.Choices[0].Finish
	}
	status, incReason, errRaw := resolveResponsesStatus(primaryFinish, resp.IncompleteReason, resp.Error)
	itemStatus := "completed"
	if status != "completed" {
		itemStatus = "incomplete"
	}

	output := make([]responsesClientOutputItemWire, 0)
	for _, ch := range resp.Choices {
		hasFunctionCall := false
		for _, pi := range ch.Items {
			if pi.Item.Kind == canonical.ItemFunctionCall && pi.Item.Call != nil {
				hasFunctionCall = true
				break
			}
		}

		for _, pi := range ch.Items {
			switch pi.Item.Kind {
			case canonical.ItemReasoning:
				r := pi.Item.Reasoning
				if r == nil {
					continue
				}
				summary := make([]responsesClientContentWire, 0)
				if r.Summary.IsPresent() {
					for _, s := range r.Summary.Val {
						sc := s
						summary = append(summary, responsesClientContentWire{
							Type: "summary_text",
							Text: &sc,
						})
					}
				}
				var contentPtr *[]responsesClientContentWire
				if len(r.Text) > 0 {
					contentSlice := make([]responsesClientContentWire, 0, len(r.Text))
					for _, seg := range r.Text {
						tc := seg
						contentSlice = append(contentSlice, responsesClientContentWire{
							Type: "reasoning_text",
							Text: &tc,
						})
					}
					contentPtr = &contentSlice
				}
				var encRaw json.RawMessage
				if r.Origin.Protocol == canonical.ProtocolOpenAIResponses {
					encRaw = nonNullJSONRaw(r.Encrypted)
				}
				if contentPtr == nil && len(summary) == 0 && len(encRaw) == 0 {
					continue
				}
				itemID := pi.PublicItemID
				if itemID == "" {
					itemID = r.ItemID
				}
				output = append(output, responsesClientOutputItemWire{
					ID:               itemID,
					Type:             "reasoning",
					Status:           itemStatus,
					Summary:          &summary,
					Content:          contentPtr,
					EncryptedContent: encRaw,
				})

			case canonical.ItemMessage:
				itemID := pi.PublicItemID
				if itemID == "" {
					itemID = pi.Item.MessageID
				}
				role := string(pi.Item.Role)
				if role == "" {
					role = string(canonical.RoleAssistant)
				}
				parts := make([]responsesClientContentWire, 0, len(pi.Item.Parts))
				fallbackParts := make([]responsesClientContentWire, 0, 1)
				hasMeaningful := false
				for _, p := range pi.Item.Parts {
					switch p.Kind {
					case canonical.PartRefusal:
						refCopy := p.Refusal
						parts = append(parts, responsesClientContentWire{
							Type:    "refusal",
							Refusal: &refCopy,
						})
						hasMeaningful = true
					case canonical.PartText:
						anns := make([]json.RawMessage, 0, len(p.Annotations))
						for _, ann := range p.Annotations {
							if cleanAnn := nonNullJSONRaw(ann); len(cleanAnn) > 0 {
								anns = append(anns, cleanAnn)
							}
						}
						textCopy := p.Text
						partWire := responsesClientContentWire{
							Type:        "output_text",
							Text:        &textCopy,
							Annotations: &anns,
						}
						if p.Text != "" || len(anns) > 0 {
							parts = append(parts, partWire)
							hasMeaningful = true
						} else if len(fallbackParts) == 0 {
							fallbackParts = append(fallbackParts, partWire)
						}
					}
				}
				if hasMeaningful {
					output = append(output, responsesClientOutputItemWire{
						ID:      itemID,
						Type:    "message",
						Status:  itemStatus,
						Role:    role,
						Content: &parts,
					})
				} else if !hasFunctionCall && (status == "incomplete" || len(output) == 0) {
					output = append(output, responsesClientOutputItemWire{
						ID:      itemID,
						Type:    "message",
						Status:  itemStatus,
						Role:    role,
						Content: &fallbackParts,
					})
				}

			case canonical.ItemFunctionCall:
				if pi.Item.Call == nil {
					continue
				}
				itemID := pi.PublicItemID
				if itemID == "" {
					itemID = pi.Item.Call.ItemID
				}
				callID := pi.PublicCallID
				if callID == "" {
					callID = pi.Item.Call.CallID
				}
				argsStr := formatArgumentsString(pi.Item.Call.Args)
				output = append(output, responsesClientOutputItemWire{
					ID:        itemID,
					Type:      "function_call",
					Status:    itemStatus,
					CallID:    callID,
					Name:      pi.Item.Call.Name,
					Arguments: &argsStr,
				})
			}
		}
	}

	return buildResponsesResourceJSON(
		resp.PublicID,
		resp.CreatedAt,
		status,
		resp.PublicModel,
		resp.ServiceTier,
		output,
		resp.WireUsage,
		incReason,
		errRaw,
		resp.EchoFields,
	)
}

func encodeMessagesClientResponse(resp clientresponse.CanonicalResponse) ([]byte, error) {
	content := make([]messagesClientContentWire, 0)
	var finish canonical.FinishReason
	if len(resp.Choices) > 0 {
		ch := resp.Choices[0]
		finish = ch.Finish
		hasNonEmptyBlock := false
		for _, pi := range ch.Items {
			switch pi.Item.Kind {
			case canonical.ItemFunctionCall:
				if pi.Item.Call != nil {
					hasNonEmptyBlock = true
				}
			case canonical.ItemReasoning:
				if pi.Item.Reasoning != nil && (len(pi.Item.Reasoning.Text) > 0 || pi.Item.Reasoning.Signature != "" || pi.Item.Reasoning.Redacted != "") {
					hasNonEmptyBlock = true
				}
			case canonical.ItemMessage:
				for _, p := range pi.Item.Parts {
					if (p.Kind == canonical.PartText && p.Text != "") || (p.Kind == canonical.PartRefusal && p.Refusal != "") {
						hasNonEmptyBlock = true
					}
				}
			}
		}

		for _, pi := range ch.Items {
			switch pi.Item.Kind {
			case canonical.ItemReasoning:
				r := pi.Item.Reasoning
				if r == nil {
					continue
				}
				if len(r.Text) > 0 || r.Signature != "" {
					thinkingStr := strings.Join(r.Text, "")
					var sigPtr *string
					if r.Signature != "" {
						sigCopy := r.Signature
						sigPtr = &sigCopy
					}
					content = append(content, messagesClientContentWire{
						Type:      "thinking",
						Thinking:  &thinkingStr,
						Signature: sigPtr,
					})
				}
				if r.Redacted != "" {
					redactedCopy := r.Redacted
					content = append(content, messagesClientContentWire{
						Type: "redacted_thinking",
						Data: &redactedCopy,
					})
				}

			case canonical.ItemMessage:
				for _, p := range pi.Item.Parts {
					switch p.Kind {
					case canonical.PartText:
						if p.Text != "" || (!hasNonEmptyBlock && len(content) == 0) {
							textCopy := p.Text
							content = append(content, messagesClientContentWire{
								Type: "text",
								Text: &textCopy,
							})
						}
					case canonical.PartRefusal:
						if p.Refusal != "" {
							refCopy := p.Refusal
							content = append(content, messagesClientContentWire{
								Type: "text",
								Text: &refCopy,
							})
						}
					}
				}

			case canonical.ItemFunctionCall:
				if pi.Item.Call == nil {
					continue
				}
				callID := pi.PublicCallID
				if callID == "" {
					callID = pi.Item.Call.CallID
				}
				content = append(content, messagesClientContentWire{
					Type:  "tool_use",
					ID:    callID,
					Name:  pi.Item.Call.Name,
					Input: formatMessagesToolInput(pi.Item.Call.Args),
				})
			}
		}
	}

	stopReason, stopSeq, err := mapMessagesStopReasonAndSeq(finish)
	if err != nil {
		return nil, err
	}

	wire := messagesClientResponseWire{
		ID:           resp.PublicID,
		Type:         "message",
		Role:         string(canonical.RoleAssistant),
		Model:        resp.PublicModel,
		Content:      content,
		StopReason:   stopReason,
		StopSequence: stopSeq,
		Usage:        nonNullJSONRaw(resp.WireUsage),
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("marshal messages client response: %w", err)
	}
	return encoded, nil
}

type respStreamPart struct {
	partType    string // "output_text" or "refusal"
	textBuf     strings.Builder
	annotations []json.RawMessage
	closed      bool
}

type respStreamItem struct {
	outputIndex   int
	kind          canonical.ItemKind
	id            string
	callID        string
	name          string
	role          string
	status        string
	closed        bool
	reasoningText strings.Builder
	argsBuf       strings.Builder
	parts         []respStreamPart
}

func (it *respStreamItem) toWire() responsesClientOutputItemWire {
	switch it.kind {
	case canonical.ItemReasoning:
		summary := make([]responsesClientContentWire, 0)
		text := it.reasoningText.String()
		content := []responsesClientContentWire{{
			Type: "reasoning_text",
			Text: &text,
		}}
		return responsesClientOutputItemWire{
			ID:      it.id,
			Type:    "reasoning",
			Status:  it.status,
			Summary: &summary,
			Content: &content,
		}
	case canonical.ItemFunctionCall:
		args := it.argsBuf.String()
		return responsesClientOutputItemWire{
			ID:        it.id,
			Type:      "function_call",
			Status:    it.status,
			CallID:    it.callID,
			Name:      it.name,
			Arguments: &args,
		}
	default:
		content := make([]responsesClientContentWire, 0, len(it.parts))
		for _, p := range it.parts {
			val := p.textBuf.String()
			if p.partType == "refusal" {
				content = append(content, responsesClientContentWire{
					Type:    "refusal",
					Refusal: &val,
				})
			} else {
				anns := make([]json.RawMessage, len(p.annotations))
				copy(anns, p.annotations)
				content = append(content, responsesClientContentWire{
					Type:        "output_text",
					Text:        &val,
					Annotations: &anns,
				})
			}
		}
		return responsesClientOutputItemWire{
			ID:      it.id,
			Type:    "message",
			Status:  it.status,
			Role:    it.role,
			Content: &content,
		}
	}
}

type msgActiveBlockKind uint8

const (
	msgBlockNone msgActiveBlockKind = iota
	msgBlockThinking
	msgBlockRedacted
	msgBlockText
	msgBlockTool
)

type msgStreamToolState struct {
	toolIndex    int
	callID       string
	name         string
	blockIndex   int
	started      bool
	stopped      bool
	pendingStop  bool
	bufferedArgs strings.Builder
}

// CanonicalStreamWriter maintains per-attempt client SSE serialization state
// without global state, HttpContext, ID minting, or secondary JSON re-parsing.
type CanonicalStreamWriter struct {
	protocol    canonical.Protocol
	publicID    string
	publicModel string
	createdAt   int64
	serviceTier string
	echoFields  map[string]json.RawMessage
	wireUsage   json.RawMessage
	startUsage  json.RawMessage
	completed   bool
	failed      bool

	// openai_chat state
	chatRoleSent       map[int]bool
	chatChoiceFinished map[int]bool

	// openai_responses state
	respSeq             int
	respCreated         bool
	respInProgress      bool
	respItems           []*respStreamItem
	respReasoningBySlot map[[2]int]int
	respMessageBySlot   map[[2]int]int
	respToolBySlot      map[[2]int]int

	// anthropic_messages state
	msgStarted        bool
	msgNextBlockIdx   int
	msgHasActiveBlock bool
	msgActiveKind     msgActiveBlockKind
	msgActiveBlockIdx int
	msgActiveItemIdx  int
	msgActiveToolIdx  int
	msgTools          map[int]*msgStreamToolState
}

// NewCanonicalStreamWriter creates a fresh per-attempt streaming SSE writer for
// one of the three supported client protocols.
func NewCanonicalStreamWriter(protocol canonical.Protocol) (*CanonicalStreamWriter, error) {
	switch protocol {
	case canonical.ProtocolOpenAIChat, canonical.ProtocolOpenAIResponses, canonical.ProtocolAnthropicMessages:
		return &CanonicalStreamWriter{
			protocol:            protocol,
			chatRoleSent:        make(map[int]bool),
			chatChoiceFinished:  make(map[int]bool),
			respReasoningBySlot: make(map[[2]int]int),
			respMessageBySlot:   make(map[[2]int]int),
			respToolBySlot:      make(map[[2]int]int),
			msgTools:            make(map[int]*msgStreamToolState),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported canonical client stream protocol %q", protocol)
	}
}

func (w *CanonicalStreamWriter) captureEnvelope(ev clientresponse.CanonicalEvent) error {
	if ev.Protocol != "" && ev.Protocol != w.protocol {
		return fmt.Errorf("canonical event protocol %q does not match writer protocol %q", ev.Protocol, w.protocol)
	}
	if ev.PublicID != "" {
		w.publicID = ev.PublicID
	} else if ev.Event.ID != "" && w.publicID == "" {
		w.publicID = ev.Event.ID
	}
	if ev.PublicModel != "" {
		w.publicModel = ev.PublicModel
	} else if ev.Event.Model != "" && w.publicModel == "" {
		w.publicModel = ev.Event.Model
	}
	if ev.CreatedAt > 0 {
		w.createdAt = ev.CreatedAt
	} else if ev.Event.CreatedAt > 0 && w.createdAt == 0 {
		w.createdAt = ev.Event.CreatedAt
	}
	if ev.ServiceTier != "" {
		w.serviceTier = ev.ServiceTier
	} else if ev.Event.ServiceTier != "" && w.serviceTier == "" {
		w.serviceTier = ev.Event.ServiceTier
	}
	if len(ev.EchoFields) > 0 {
		w.echoFields = ev.EchoFields
	}
	if len(ev.WireUsage) > 0 {
		w.wireUsage = ev.WireUsage
	}
	if len(ev.StartUsage) > 0 {
		w.startUsage = ev.StartUsage
	}
	return nil
}

// Write serializes a projected streaming CanonicalEvent into zero or more SSE frames.
func (w *CanonicalStreamWriter) Write(ev clientresponse.CanonicalEvent) ([]byte, error) {
	if ev.Event.Kind == canonical.EventProtocolDone {
		return w.Finish(ev)
	}
	if ev.Event.Kind == canonical.EventProviderError {
		return w.Fail(ev)
	}
	if w.completed || w.failed {
		return nil, nil
	}
	if err := w.captureEnvelope(ev); err != nil {
		return nil, err
	}

	switch w.protocol {
	case canonical.ProtocolOpenAIChat:
		return w.writeChatEvent(ev)
	case canonical.ProtocolOpenAIResponses:
		return w.writeResponsesEvent(ev)
	case canonical.ProtocolAnthropicMessages:
		return w.writeMessagesEvent(ev)
	default:
		return nil, fmt.Errorf("unsupported writer protocol %q", w.protocol)
	}
}

// Finish closes any open streaming items/blocks and emits the terminal SSE frames.
func (w *CanonicalStreamWriter) Finish(ev clientresponse.CanonicalEvent) ([]byte, error) {
	if w.failed {
		return nil, errors.New("cannot finish an already failed canonical stream")
	}
	if w.completed {
		return nil, nil
	}
	if err := w.captureEnvelope(ev); err != nil {
		return nil, err
	}

	switch w.protocol {
	case canonical.ProtocolOpenAIChat:
		return w.finishChatStream(ev)
	case canonical.ProtocolOpenAIResponses:
		return w.finishResponsesStream(ev)
	case canonical.ProtocolAnthropicMessages:
		return w.finishMessagesStream(ev)
	default:
		return nil, fmt.Errorf("unsupported writer protocol %q", w.protocol)
	}
}

// Fail emits the protocol-appropriate terminal error SSE frame without fabricating
// completion or closing incomplete Responses output items as executable.
func (w *CanonicalStreamWriter) Fail(ev clientresponse.CanonicalEvent) ([]byte, error) {
	if w.completed {
		return nil, errors.New("cannot fail an already completed canonical stream")
	}
	if w.failed {
		return nil, nil
	}
	if err := w.captureEnvelope(ev); err != nil {
		return nil, err
	}

	switch w.protocol {
	case canonical.ProtocolOpenAIChat:
		return w.failChatStream(ev)
	case canonical.ProtocolOpenAIResponses:
		return w.failResponsesStream(ev)
	case canonical.ProtocolAnthropicMessages:
		return w.failMessagesStream(ev)
	default:
		return nil, fmt.Errorf("unsupported writer protocol %q", w.protocol)
	}
}

func appendChatSSEChunk(buf *bytes.Buffer, chunk chatStreamChunkWire) error {
	encoded, err := json.Marshal(chunk)
	if err != nil {
		return fmt.Errorf("marshal chat stream chunk: %w", err)
	}
	buf.WriteString("data: ")
	buf.Write(encoded)
	buf.WriteString("\n\n")
	return nil
}

func (w *CanonicalStreamWriter) newChatChunk(choices []chatStreamChoiceWire, usage json.RawMessage) chatStreamChunkWire {
	return chatStreamChunkWire{
		ID:          w.publicID,
		Object:      "chat.completion.chunk",
		Created:     w.createdAt,
		Model:       w.publicModel,
		ServiceTier: w.serviceTier,
		Choices:     choices,
		Usage:       usage,
	}
}

func (w *CanonicalStreamWriter) writeChatEvent(ev clientresponse.CanonicalEvent) ([]byte, error) {
	var buf bytes.Buffer
	choiceIdx := ev.Event.ChoiceIndex

	switch ev.Event.Kind {
	case canonical.EventResponseStart:
		if w.chatRoleSent[0] {
			return nil, nil
		}
		w.chatRoleSent[0] = true
		chunk := w.newChatChunk([]chatStreamChoiceWire{{
			Index: 0,
			Delta: chatStreamDeltaWire{Role: string(canonical.RoleAssistant)},
		}}, w.wireUsage)
		if err := appendChatSSEChunk(&buf, chunk); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil

	case canonical.EventTextDelta:
		var anns []json.RawMessage
		for _, ann := range ev.Event.Annotations {
			if cleanAnn := nonNullJSONRaw(ann); len(cleanAnn) > 0 {
				anns = append(anns, cleanAnn)
			}
		}
		if ev.Event.Text == "" && len(anns) == 0 && w.chatRoleSent[choiceIdx] {
			return nil, nil
		}
		delta := chatStreamDeltaWire{Annotations: anns}
		if !w.chatRoleSent[choiceIdx] {
			delta.Role = string(canonical.RoleAssistant)
			w.chatRoleSent[choiceIdx] = true
		}
		textCopy := ev.Event.Text
		delta.Content = &textCopy
		chunk := w.newChatChunk([]chatStreamChoiceWire{{
			Index:    choiceIdx,
			Delta:    delta,
			Logprobs: nonNullJSONRaw(ev.Event.Logprobs),
		}}, w.wireUsage)
		if err := appendChatSSEChunk(&buf, chunk); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil

	case canonical.EventRefusalDelta:
		if ev.Event.Text == "" {
			return nil, nil
		}
		delta := chatStreamDeltaWire{Refusal: ev.Event.Text}
		if !w.chatRoleSent[choiceIdx] {
			delta.Role = string(canonical.RoleAssistant)
			w.chatRoleSent[choiceIdx] = true
		}
		chunk := w.newChatChunk([]chatStreamChoiceWire{{
			Index: choiceIdx,
			Delta: delta,
		}}, w.wireUsage)
		if err := appendChatSSEChunk(&buf, chunk); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil

	case canonical.EventReasoningDelta:
		if ev.Event.Text == "" {
			return nil, nil
		}
		delta := chatStreamDeltaWire{ReasoningContent: ev.Event.Text}
		if !w.chatRoleSent[choiceIdx] {
			delta.Role = string(canonical.RoleAssistant)
			w.chatRoleSent[choiceIdx] = true
		}
		chunk := w.newChatChunk([]chatStreamChoiceWire{{
			Index: choiceIdx,
			Delta: delta,
		}}, w.wireUsage)
		if err := appendChatSSEChunk(&buf, chunk); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil

	case canonical.EventToolCallStart:
		callID := ev.PublicCallID
		if callID == "" {
			callID = ev.Event.CallID
		}
		nameCopy := ev.Event.ToolName
		argsCopy := ev.Event.Text
		delta := chatStreamDeltaWire{
			ToolCalls: []chatStreamToolCallWire{{
				Index: ev.Event.ToolIndex,
				ID:    callID,
				Type:  "function",
				Function: chatStreamFunctionWire{
					Name:      &nameCopy,
					Arguments: &argsCopy,
				},
			}},
		}
		if !w.chatRoleSent[choiceIdx] {
			delta.Role = string(canonical.RoleAssistant)
			w.chatRoleSent[choiceIdx] = true
		}
		chunk := w.newChatChunk([]chatStreamChoiceWire{{
			Index: choiceIdx,
			Delta: delta,
		}}, w.wireUsage)
		if err := appendChatSSEChunk(&buf, chunk); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil

	case canonical.EventToolCallArgsDelta:
		if ev.Event.Text == "" {
			return nil, nil
		}
		argsCopy := ev.Event.Text
		delta := chatStreamDeltaWire{
			ToolCalls: []chatStreamToolCallWire{{
				Index: ev.Event.ToolIndex,
				Function: chatStreamFunctionWire{
					Arguments: &argsCopy,
				},
			}},
		}
		if !w.chatRoleSent[choiceIdx] {
			delta.Role = string(canonical.RoleAssistant)
			w.chatRoleSent[choiceIdx] = true
		}
		chunk := w.newChatChunk([]chatStreamChoiceWire{{
			Index: choiceIdx,
			Delta: delta,
		}}, w.wireUsage)
		if err := appendChatSSEChunk(&buf, chunk); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil

	case canonical.EventChoiceFinish:
		fin := ev.Finish
		if fin.Category == canonical.FinishNone {
			fin = ev.Event.Finish
		}
		w.chatChoiceFinished[choiceIdx] = true
		var delta chatStreamDeltaWire
		if !w.chatRoleSent[choiceIdx] {
			delta.Role = string(canonical.RoleAssistant)
			w.chatRoleSent[choiceIdx] = true
		}
		chunk := w.newChatChunk([]chatStreamChoiceWire{{
			Index:        choiceIdx,
			Delta:        delta,
			Logprobs:     nonNullJSONRaw(ev.Event.Logprobs),
			FinishReason: mapChatFinishReason(fin),
		}}, w.wireUsage)
		if err := appendChatSSEChunk(&buf, chunk); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil

	default:
		return nil, nil
	}
}

func (w *CanonicalStreamWriter) finishChatStream(ev clientresponse.CanonicalEvent) ([]byte, error) {
	var buf bytes.Buffer
	fin := ev.Finish
	if fin.Category == canonical.FinishNone {
		fin = ev.Event.Finish
	}
	if len(w.chatChoiceFinished) == 0 && fin.Category != canonical.FinishNone {
		w.chatChoiceFinished[0] = true
		var delta chatStreamDeltaWire
		if !w.chatRoleSent[0] {
			delta.Role = string(canonical.RoleAssistant)
			w.chatRoleSent[0] = true
		}
		chunk := w.newChatChunk([]chatStreamChoiceWire{{
			Index:        0,
			Delta:        delta,
			FinishReason: mapChatFinishReason(fin),
		}}, w.wireUsage)
		if err := appendChatSSEChunk(&buf, chunk); err != nil {
			return nil, err
		}
	}

	if cleanUsage := nonNullJSONRaw(ev.FinalUsage); len(cleanUsage) > 0 {
		usageChunk := w.newChatChunk(make([]chatStreamChoiceWire, 0), cleanUsage)
		if err := appendChatSSEChunk(&buf, usageChunk); err != nil {
			return nil, err
		}
	}

	buf.WriteString("data: [DONE]\n\n")
	w.completed = true
	return buf.Bytes(), nil
}

func (w *CanonicalStreamWriter) failChatStream(ev clientresponse.CanonicalEvent) ([]byte, error) {
	var buf bytes.Buffer
	if cleanUsage := nonNullJSONRaw(ev.FinalUsage); len(cleanUsage) > 0 {
		usageChunk := w.newChatChunk(make([]chatStreamChoiceWire, 0), cleanUsage)
		if err := appendChatSSEChunk(&buf, usageChunk); err != nil {
			return nil, err
		}
	}
	if cleanBody := nonNullJSONRaw(ev.FailureBody); len(cleanBody) > 0 {
		buf.WriteString("data: ")
		buf.Write(cleanBody)
		buf.WriteString("\n\n")
	} else if cleanErr := nonNullJSONRaw(ev.Error); len(cleanErr) > 0 {
		buf.WriteString("data: {\"error\":")
		buf.Write(cleanErr)
		buf.WriteString("}\n\n")
	}
	w.failed = true
	return buf.Bytes(), nil
}

func (w *CanonicalStreamWriter) appendResponsesSSEEvent(buf *bytes.Buffer, event responsesStreamEventWire) error {
	event.SequenceNumber = w.respSeq
	encoded, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal responses stream event %s: %w", event.Type, err)
	}
	w.respSeq++
	buf.WriteString("event: ")
	buf.WriteString(event.Type)
	buf.WriteString("\ndata: ")
	buf.Write(encoded)
	buf.WriteString("\n\n")
	return nil
}

func (w *CanonicalStreamWriter) ensureResponsesCreated(buf *bytes.Buffer, includeInProgress bool) error {
	if !w.respCreated {
		resJSON, err := buildResponsesResourceJSON(
			w.publicID, w.createdAt, "in_progress", w.publicModel, w.serviceTier,
			make([]responsesClientOutputItemWire, 0), nil, "", nil, w.echoFields,
		)
		if err != nil {
			return err
		}
		if err := w.appendResponsesSSEEvent(buf, responsesStreamEventWire{
			Type:     "response.created",
			Response: resJSON,
		}); err != nil {
			return err
		}
		w.respCreated = true
	}
	if includeInProgress && !w.respInProgress {
		resJSON, err := buildResponsesResourceJSON(
			w.publicID, w.createdAt, "in_progress", w.publicModel, w.serviceTier,
			make([]responsesClientOutputItemWire, 0), nil, "", nil, w.echoFields,
		)
		if err != nil {
			return err
		}
		if err := w.appendResponsesSSEEvent(buf, responsesStreamEventWire{
			Type:     "response.in_progress",
			Response: resJSON,
		}); err != nil {
			return err
		}
		w.respInProgress = true
	}
	return nil
}

func (w *CanonicalStreamWriter) closeResponsesItem(buf *bytes.Buffer, it *respStreamItem, status string) error {
	if it.closed {
		return nil
	}
	outIdx := it.outputIndex
	switch it.kind {
	case canonical.ItemReasoning:
		contentIdx := 0
		fullText := it.reasoningText.String()
		if err := w.appendResponsesSSEEvent(buf, responsesStreamEventWire{
			Type:         "response.reasoning_text.done",
			OutputIndex:  &outIdx,
			ContentIndex: &contentIdx,
			ItemID:       it.id,
			Text:         &fullText,
		}); err != nil {
			return err
		}

	case canonical.ItemMessage:
		for ci := range it.parts {
			p := &it.parts[ci]
			if p.closed {
				continue
			}
			contentIdx := ci
			fullVal := p.textBuf.String()
			if p.partType == "refusal" {
				if err := w.appendResponsesSSEEvent(buf, responsesStreamEventWire{
					Type:         "response.refusal.done",
					OutputIndex:  &outIdx,
					ContentIndex: &contentIdx,
					ItemID:       it.id,
					Refusal:      &fullVal,
				}); err != nil {
					return err
				}
				partWire := responsesClientContentWire{Type: "refusal", Refusal: &fullVal}
				if err := w.appendResponsesSSEEvent(buf, responsesStreamEventWire{
					Type:         "response.content_part.done",
					OutputIndex:  &outIdx,
					ContentIndex: &contentIdx,
					ItemID:       it.id,
					Part:         &partWire,
				}); err != nil {
					return err
				}
			} else {
				if err := w.appendResponsesSSEEvent(buf, responsesStreamEventWire{
					Type:         "response.output_text.done",
					OutputIndex:  &outIdx,
					ContentIndex: &contentIdx,
					ItemID:       it.id,
					Text:         &fullVal,
				}); err != nil {
					return err
				}
				anns := make([]json.RawMessage, len(p.annotations))
				copy(anns, p.annotations)
				partWire := responsesClientContentWire{Type: "output_text", Text: &fullVal, Annotations: &anns}
				if err := w.appendResponsesSSEEvent(buf, responsesStreamEventWire{
					Type:         "response.content_part.done",
					OutputIndex:  &outIdx,
					ContentIndex: &contentIdx,
					ItemID:       it.id,
					Part:         &partWire,
				}); err != nil {
					return err
				}
			}
			p.closed = true
		}

	case canonical.ItemFunctionCall:
		fullArgs := it.argsBuf.String()
		if err := w.appendResponsesSSEEvent(buf, responsesStreamEventWire{
			Type:        "response.function_call_arguments.done",
			OutputIndex: &outIdx,
			ItemID:      it.id,
			Arguments:   &fullArgs,
		}); err != nil {
			return err
		}
	}

	it.status = status
	it.closed = true
	itemWire := it.toWire()
	return w.appendResponsesSSEEvent(buf, responsesStreamEventWire{
		Type:        "response.output_item.done",
		OutputIndex: &outIdx,
		Item:        &itemWire,
	})
}

func (w *CanonicalStreamWriter) ensureResponsesMessageItem(buf *bytes.Buffer, ev clientresponse.CanonicalEvent) (*respStreamItem, error) {
	slot := [2]int{ev.Event.ChoiceIndex, ev.Event.ItemIndex}
	if idx, exists := w.respMessageBySlot[slot]; exists && !w.respItems[idx].closed {
		return w.respItems[idx], nil
	}
	itemID := ev.PublicItemID
	if itemID == "" {
		itemID = ev.Event.ItemID
	}
	role := string(ev.Event.Role)
	if role == "" {
		role = string(canonical.RoleAssistant)
	}
	outIdx := len(w.respItems)
	it := &respStreamItem{
		outputIndex: outIdx,
		kind:        canonical.ItemMessage,
		id:          itemID,
		role:        role,
		status:      "in_progress",
	}
	w.respItems = append(w.respItems, it)
	w.respMessageBySlot[slot] = outIdx
	itemWire := it.toWire()
	if err := w.appendResponsesSSEEvent(buf, responsesStreamEventWire{
		Type:        "response.output_item.added",
		OutputIndex: &outIdx,
		Item:        &itemWire,
	}); err != nil {
		return nil, err
	}
	return it, nil
}

func (w *CanonicalStreamWriter) writeResponsesEvent(ev clientresponse.CanonicalEvent) ([]byte, error) {
	var buf bytes.Buffer

	switch ev.Event.Kind {
	case canonical.EventResponseStart:
		if err := w.ensureResponsesCreated(&buf, true); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil

	case canonical.EventReasoningDelta:
		if ev.Event.Text == "" {
			return nil, nil
		}
		if err := w.ensureResponsesCreated(&buf, true); err != nil {
			return nil, err
		}
		slot := [2]int{ev.Event.ChoiceIndex, ev.Event.ItemIndex}
		idx, exists := w.respReasoningBySlot[slot]
		if !exists || w.respItems[idx].closed {
			itemID := ev.PublicItemID
			if itemID == "" {
				itemID = ev.Event.ItemID
			}
			idx = len(w.respItems)
			it := &respStreamItem{
				outputIndex: idx,
				kind:        canonical.ItemReasoning,
				id:          itemID,
				status:      "in_progress",
			}
			w.respItems = append(w.respItems, it)
			w.respReasoningBySlot[slot] = idx
			itemWire := it.toWire()
			if err := w.appendResponsesSSEEvent(&buf, responsesStreamEventWire{
				Type:        "response.output_item.added",
				OutputIndex: &idx,
				Item:        &itemWire,
			}); err != nil {
				return nil, err
			}
		}
		it := w.respItems[idx]
		it.reasoningText.WriteString(ev.Event.Text)
		outIdx := it.outputIndex
		contentIdx := 0
		deltaCopy := ev.Event.Text
		if err := w.appendResponsesSSEEvent(&buf, responsesStreamEventWire{
			Type:         "response.reasoning_text.delta",
			OutputIndex:  &outIdx,
			ContentIndex: &contentIdx,
			ItemID:       it.id,
			Delta:        &deltaCopy,
		}); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil

	case canonical.EventTextDelta:
		var anns []json.RawMessage
		for _, ann := range ev.Event.Annotations {
			if cleanAnn := nonNullJSONRaw(ann); len(cleanAnn) > 0 {
				anns = append(anns, cleanAnn)
			}
		}
		if ev.Event.Text == "" && len(anns) == 0 {
			return nil, nil
		}
		if err := w.ensureResponsesCreated(&buf, true); err != nil {
			return nil, err
		}
		it, err := w.ensureResponsesMessageItem(&buf, ev)
		if err != nil {
			return nil, err
		}
		partIdx := -1
		if n := len(it.parts); n > 0 && !it.parts[n-1].closed && it.parts[n-1].partType == "output_text" {
			partIdx = n - 1
		} else {
			partIdx = len(it.parts)
			it.parts = append(it.parts, respStreamPart{partType: "output_text"})
			outIdx := it.outputIndex
			contentIdx := partIdx
			emptyStr := ""
			emptyAnns := make([]json.RawMessage, 0)
			partWire := responsesClientContentWire{Type: "output_text", Text: &emptyStr, Annotations: &emptyAnns}
			if err := w.appendResponsesSSEEvent(&buf, responsesStreamEventWire{
				Type:         "response.content_part.added",
				OutputIndex:  &outIdx,
				ContentIndex: &contentIdx,
				ItemID:       it.id,
				Part:         &partWire,
			}); err != nil {
				return nil, err
			}
		}
		it.parts[partIdx].textBuf.WriteString(ev.Event.Text)
		if len(anns) > 0 {
			it.parts[partIdx].annotations = append(it.parts[partIdx].annotations, anns...)
		}
		if ev.Event.Text != "" {
			outIdx := it.outputIndex
			contentIdx := partIdx
			deltaCopy := ev.Event.Text
			if err := w.appendResponsesSSEEvent(&buf, responsesStreamEventWire{
				Type:         "response.output_text.delta",
				OutputIndex:  &outIdx,
				ContentIndex: &contentIdx,
				ItemID:       it.id,
				Delta:        &deltaCopy,
			}); err != nil {
				return nil, err
			}
		}
		return buf.Bytes(), nil

	case canonical.EventRefusalDelta:
		if ev.Event.Text == "" {
			return nil, nil
		}
		if err := w.ensureResponsesCreated(&buf, true); err != nil {
			return nil, err
		}
		it, err := w.ensureResponsesMessageItem(&buf, ev)
		if err != nil {
			return nil, err
		}
		partIdx := -1
		if n := len(it.parts); n > 0 && !it.parts[n-1].closed && it.parts[n-1].partType == "refusal" {
			partIdx = n - 1
		} else {
			partIdx = len(it.parts)
			it.parts = append(it.parts, respStreamPart{partType: "refusal"})
			outIdx := it.outputIndex
			contentIdx := partIdx
			emptyStr := ""
			partWire := responsesClientContentWire{Type: "refusal", Refusal: &emptyStr}
			if err := w.appendResponsesSSEEvent(&buf, responsesStreamEventWire{
				Type:         "response.content_part.added",
				OutputIndex:  &outIdx,
				ContentIndex: &contentIdx,
				ItemID:       it.id,
				Part:         &partWire,
			}); err != nil {
				return nil, err
			}
		}
		it.parts[partIdx].textBuf.WriteString(ev.Event.Text)
		outIdx := it.outputIndex
		contentIdx := partIdx
		deltaCopy := ev.Event.Text
		if err := w.appendResponsesSSEEvent(&buf, responsesStreamEventWire{
			Type:         "response.refusal.delta",
			OutputIndex:  &outIdx,
			ContentIndex: &contentIdx,
			ItemID:       it.id,
			Delta:        &deltaCopy,
		}); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil

	case canonical.EventToolCallStart, canonical.EventToolCallArgsDelta:
		if err := w.ensureResponsesCreated(&buf, true); err != nil {
			return nil, err
		}
		slot := [2]int{ev.Event.ChoiceIndex, ev.Event.ToolIndex}
		idx, exists := w.respToolBySlot[slot]
		itemID := ev.PublicItemID
		if itemID == "" {
			itemID = ev.Event.ItemID
		}
		callID := ev.PublicCallID
		if callID == "" {
			callID = ev.Event.CallID
		}
		if !exists || w.respItems[idx].closed {
			idx = len(w.respItems)
			it := &respStreamItem{
				outputIndex: idx,
				kind:        canonical.ItemFunctionCall,
				id:          itemID,
				callID:      callID,
				name:        ev.Event.ToolName,
				status:      "in_progress",
			}
			w.respItems = append(w.respItems, it)
			w.respToolBySlot[slot] = idx
			itemWire := it.toWire()
			if err := w.appendResponsesSSEEvent(&buf, responsesStreamEventWire{
				Type:        "response.output_item.added",
				OutputIndex: &idx,
				Item:        &itemWire,
			}); err != nil {
				return nil, err
			}
		}
		it := w.respItems[idx]
		if it.id == "" && itemID != "" {
			it.id = itemID
		}
		if it.callID == "" && callID != "" {
			it.callID = callID
		}
		if it.name == "" && ev.Event.ToolName != "" {
			it.name = ev.Event.ToolName
		}
		if ev.Event.Text != "" {
			it.argsBuf.WriteString(ev.Event.Text)
			outIdx := it.outputIndex
			deltaCopy := ev.Event.Text
			if err := w.appendResponsesSSEEvent(&buf, responsesStreamEventWire{
				Type:        "response.function_call_arguments.delta",
				OutputIndex: &outIdx,
				ItemID:      it.id,
				Delta:       &deltaCopy,
			}); err != nil {
				return nil, err
			}
		}
		return buf.Bytes(), nil

	case canonical.EventContentBlockStop:
		var target *respStreamItem
		callID := ev.PublicCallID
		if callID == "" {
			callID = ev.Event.CallID
		}
		itemID := ev.PublicItemID
		if itemID == "" {
			itemID = ev.Event.ItemID
		}
		if callID != "" {
			for _, it := range w.respItems {
				if !it.closed && it.kind == canonical.ItemFunctionCall && it.callID == callID {
					target = it
					break
				}
			}
		}
		if target == nil && itemID != "" {
			for _, it := range w.respItems {
				if !it.closed && it.id == itemID {
					target = it
					break
				}
			}
		}
		if target == nil {
			itemSlot := [2]int{ev.Event.ChoiceIndex, ev.Event.ItemIndex}
			toolSlot := [2]int{ev.Event.ChoiceIndex, ev.Event.ToolIndex}
			if ev.Event.ToolIndex > 0 || ev.Event.ToolName != "" {
				if idx, ok := w.respToolBySlot[toolSlot]; ok && !w.respItems[idx].closed {
					target = w.respItems[idx]
				}
			} else if idx, ok := w.respReasoningBySlot[itemSlot]; ok && !w.respItems[idx].closed {
				target = w.respItems[idx]
			} else if idx, ok := w.respMessageBySlot[itemSlot]; ok && !w.respItems[idx].closed {
				target = w.respItems[idx]
			} else if idx, ok := w.respToolBySlot[toolSlot]; ok && !w.respItems[idx].closed {
				target = w.respItems[idx]
			}
		}
		if target == nil {
			return nil, nil
		}
		if err := w.closeResponsesItem(&buf, target, "completed"); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil

	default:
		return nil, nil
	}
}

func (w *CanonicalStreamWriter) finishResponsesStream(ev clientresponse.CanonicalEvent) ([]byte, error) {
	var buf bytes.Buffer
	if err := w.ensureResponsesCreated(&buf, true); err != nil {
		return nil, err
	}

	fin := ev.Finish
	if fin.Category == canonical.FinishNone {
		fin = ev.Event.Finish
	}
	status, incReason, errRaw := resolveResponsesStatus(fin, ev.IncompleteReason, ev.Error)
	itemStatus := "completed"
	if status != "completed" {
		itemStatus = "incomplete"
	}

	for _, it := range w.respItems {
		if !it.closed {
			if err := w.closeResponsesItem(&buf, it, itemStatus); err != nil {
				return nil, err
			}
		}
	}

	output := make([]responsesClientOutputItemWire, len(w.respItems))
	for i, it := range w.respItems {
		output[i] = it.toWire()
	}

	resJSON, err := buildResponsesResourceJSON(
		w.publicID, w.createdAt, status, w.publicModel, w.serviceTier,
		output, ev.FinalUsage, incReason, errRaw, w.echoFields,
	)
	if err != nil {
		return nil, err
	}
	w.completed = true
	if err := w.appendResponsesSSEEvent(&buf, responsesStreamEventWire{
		Type:     "response." + status,
		Response: resJSON,
	}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (w *CanonicalStreamWriter) failResponsesStream(ev clientresponse.CanonicalEvent) ([]byte, error) {
	var buf bytes.Buffer
	if err := w.ensureResponsesCreated(&buf, false); err != nil {
		return nil, err
	}

	output := make([]responsesClientOutputItemWire, len(w.respItems))
	for i, it := range w.respItems {
		if !it.closed {
			it.status = "incomplete"
		}
		output[i] = it.toWire()
	}

	errRaw := nonNullJSONRaw(ev.Error)
	if len(errRaw) == 0 {
		errRaw = json.RawMessage(`{"code":"server_error","message":"The upstream generation was interrupted."}`)
	}

	resJSON, err := buildResponsesResourceJSON(
		w.publicID, w.createdAt, "failed", w.publicModel, w.serviceTier,
		output, ev.FinalUsage, "", errRaw, w.echoFields,
	)
	if err != nil {
		return nil, err
	}
	w.failed = true
	if err := w.appendResponsesSSEEvent(&buf, responsesStreamEventWire{
		Type:     "response.failed",
		Response: resJSON,
	}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func appendMessagesSSEEvent(buf *bytes.Buffer, event messagesStreamEventWire) error {
	encoded, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal messages stream event %s: %w", event.Type, err)
	}
	buf.WriteString("event: ")
	buf.WriteString(event.Type)
	buf.WriteString("\ndata: ")
	buf.Write(encoded)
	buf.WriteString("\n\n")
	return nil
}

func (w *CanonicalStreamWriter) ensureMessagesStarted(buf *bytes.Buffer) error {
	if w.msgStarted {
		return nil
	}
	w.msgStarted = true
	msg := messagesClientResponseWire{
		ID:           w.publicID,
		Type:         "message",
		Role:         string(canonical.RoleAssistant),
		Model:        w.publicModel,
		Content:      make([]messagesClientContentWire, 0),
		StopReason:   nil,
		StopSequence: json.RawMessage("null"),
		Usage:        nonNullJSONRaw(w.startUsage),
	}
	return appendMessagesSSEEvent(buf, messagesStreamEventWire{
		Type:    "message_start",
		Message: &msg,
	})
}

func (w *CanonicalStreamWriter) closeMessagesActiveBlock(buf *bytes.Buffer) error {
	if !w.msgHasActiveBlock {
		return nil
	}
	idx := w.msgActiveBlockIdx
	if w.msgActiveKind == msgBlockTool {
		if ts, ok := w.msgTools[w.msgActiveToolIdx]; ok {
			ts.stopped = true
		}
	}
	w.msgHasActiveBlock = false
	w.msgActiveKind = msgBlockNone
	return appendMessagesSSEEvent(buf, messagesStreamEventWire{
		Type:  "content_block_stop",
		Index: &idx,
	})
}

func (w *CanonicalStreamWriter) startMessagesToolBlock(buf *bytes.Buffer, ts *msgStreamToolState) error {
	if ts.started {
		return nil
	}
	if err := w.closeMessagesActiveBlock(buf); err != nil {
		return err
	}
	idx := w.msgNextBlockIdx
	w.msgNextBlockIdx++
	ts.started = true
	ts.blockIndex = idx
	w.msgHasActiveBlock = true
	w.msgActiveKind = msgBlockTool
	w.msgActiveBlockIdx = idx
	w.msgActiveToolIdx = ts.toolIndex

	block := messagesClientContentWire{
		Type:  "tool_use",
		ID:    ts.callID,
		Name:  ts.name,
		Input: json.RawMessage("{}"),
	}
	if err := appendMessagesSSEEvent(buf, messagesStreamEventWire{
		Type:         "content_block_start",
		Index:        &idx,
		ContentBlock: &block,
	}); err != nil {
		return err
	}
	if ts.bufferedArgs.Len() > 0 {
		partial := ts.bufferedArgs.String()
		ts.bufferedArgs.Reset()
		delta := messagesStreamDeltaWire{
			Type:        "input_json_delta",
			PartialJSON: &partial,
		}
		if err := appendMessagesSSEEvent(buf, messagesStreamEventWire{
			Type:  "content_block_delta",
			Index: &idx,
			Delta: &delta,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (w *CanonicalStreamWriter) sortedMsgToolIndices() []int {
	indices := make([]int, 0, len(w.msgTools))
	for k := range w.msgTools {
		indices = append(indices, k)
	}
	slices.Sort(indices)
	return indices
}

func (w *CanonicalStreamWriter) promoteQueuedMessagesTools(buf *bytes.Buffer) error {
	if w.msgHasActiveBlock {
		return nil
	}
	for _, nextIdx := range w.sortedMsgToolIndices() {
		nextTS := w.msgTools[nextIdx]
		if !nextTS.started && nextTS.callID != "" && nextTS.name != "" {
			if err := w.startMessagesToolBlock(buf, nextTS); err != nil {
				return err
			}
			if nextTS.pendingStop {
				if err := w.closeMessagesActiveBlock(buf); err != nil {
					return err
				}
				continue
			}
			break
		}
	}
	return nil
}

func (w *CanonicalStreamWriter) closeAndDrainMessagesBlocks(buf *bytes.Buffer) error {
	if err := w.closeMessagesActiveBlock(buf); err != nil {
		return err
	}
	for _, nextIdx := range w.sortedMsgToolIndices() {
		nextTS := w.msgTools[nextIdx]
		if !nextTS.started && nextTS.callID != "" && nextTS.name != "" {
			if err := w.startMessagesToolBlock(buf, nextTS); err != nil {
				return err
			}
			if err := w.closeMessagesActiveBlock(buf); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *CanonicalStreamWriter) writeMessagesEvent(ev clientresponse.CanonicalEvent) ([]byte, error) {
	var buf bytes.Buffer

	switch ev.Event.Kind {
	case canonical.EventPing:
		buf.WriteString("event: ping\ndata: {\"type\":\"ping\"}\n\n")
		return buf.Bytes(), nil

	case canonical.EventResponseStart:
		if err := w.ensureMessagesStarted(&buf); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil

	case canonical.EventReasoningDelta, canonical.EventReasoningSignatureDelta:
		sig := ev.Event.ThoughtSignature
		if sig == "" && ev.Event.Kind == canonical.EventReasoningSignatureDelta {
			sig = ev.Event.Text
		}
		if ev.Event.Kind == canonical.EventReasoningDelta && ev.Event.Text == "" {
			return nil, nil
		}
		if ev.Event.Kind == canonical.EventReasoningSignatureDelta && sig == "" {
			return nil, nil
		}
		if err := w.ensureMessagesStarted(&buf); err != nil {
			return nil, err
		}
		if !w.msgHasActiveBlock || w.msgActiveKind != msgBlockThinking || w.msgActiveItemIdx != ev.Event.ItemIndex {
			if err := w.closeAndDrainMessagesBlocks(&buf); err != nil {
				return nil, err
			}
			idx := w.msgNextBlockIdx
			w.msgNextBlockIdx++
			w.msgHasActiveBlock = true
			w.msgActiveKind = msgBlockThinking
			w.msgActiveBlockIdx = idx
			w.msgActiveItemIdx = ev.Event.ItemIndex
			emptyStr := ""
			block := messagesClientContentWire{
				Type:     "thinking",
				Thinking: &emptyStr,
			}
			if err := appendMessagesSSEEvent(&buf, messagesStreamEventWire{
				Type:         "content_block_start",
				Index:        &idx,
				ContentBlock: &block,
			}); err != nil {
				return nil, err
			}
		}
		idx := w.msgActiveBlockIdx
		if ev.Event.Kind == canonical.EventReasoningDelta {
			thinkCopy := ev.Event.Text
			delta := messagesStreamDeltaWire{
				Type:     "thinking_delta",
				Thinking: &thinkCopy,
			}
			if err := appendMessagesSSEEvent(&buf, messagesStreamEventWire{
				Type:  "content_block_delta",
				Index: &idx,
				Delta: &delta,
			}); err != nil {
				return nil, err
			}
		} else {
			sigCopy := sig
			delta := messagesStreamDeltaWire{
				Type:      "signature_delta",
				Signature: &sigCopy,
			}
			if err := appendMessagesSSEEvent(&buf, messagesStreamEventWire{
				Type:  "content_block_delta",
				Index: &idx,
				Delta: &delta,
			}); err != nil {
				return nil, err
			}
		}
		return buf.Bytes(), nil

	case canonical.EventReasoningRedacted:
		if err := w.ensureMessagesStarted(&buf); err != nil {
			return nil, err
		}
		if err := w.closeAndDrainMessagesBlocks(&buf); err != nil {
			return nil, err
		}
		idx := w.msgNextBlockIdx
		w.msgNextBlockIdx++
		w.msgHasActiveBlock = true
		w.msgActiveKind = msgBlockRedacted
		w.msgActiveBlockIdx = idx
		w.msgActiveItemIdx = ev.Event.ItemIndex
		dataCopy := ev.Event.Text
		block := messagesClientContentWire{
			Type: "redacted_thinking",
			Data: &dataCopy,
		}
		if err := appendMessagesSSEEvent(&buf, messagesStreamEventWire{
			Type:         "content_block_start",
			Index:        &idx,
			ContentBlock: &block,
		}); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil

	case canonical.EventTextDelta, canonical.EventRefusalDelta:
		if ev.Event.Text == "" {
			return nil, nil
		}
		if err := w.ensureMessagesStarted(&buf); err != nil {
			return nil, err
		}
		if !w.msgHasActiveBlock || w.msgActiveKind != msgBlockText || w.msgActiveItemIdx != ev.Event.ItemIndex {
			if err := w.closeAndDrainMessagesBlocks(&buf); err != nil {
				return nil, err
			}
			idx := w.msgNextBlockIdx
			w.msgNextBlockIdx++
			w.msgHasActiveBlock = true
			w.msgActiveKind = msgBlockText
			w.msgActiveBlockIdx = idx
			w.msgActiveItemIdx = ev.Event.ItemIndex
			emptyStr := ""
			block := messagesClientContentWire{
				Type: "text",
				Text: &emptyStr,
			}
			if err := appendMessagesSSEEvent(&buf, messagesStreamEventWire{
				Type:         "content_block_start",
				Index:        &idx,
				ContentBlock: &block,
			}); err != nil {
				return nil, err
			}
		}
		idx := w.msgActiveBlockIdx
		textCopy := ev.Event.Text
		delta := messagesStreamDeltaWire{
			Type: "text_delta",
			Text: &textCopy,
		}
		if err := appendMessagesSSEEvent(&buf, messagesStreamEventWire{
			Type:  "content_block_delta",
			Index: &idx,
			Delta: &delta,
		}); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil

	case canonical.EventToolCallStart, canonical.EventToolCallArgsDelta:
		if err := w.ensureMessagesStarted(&buf); err != nil {
			return nil, err
		}
		tIdx := ev.Event.ToolIndex
		ts, exists := w.msgTools[tIdx]
		if !exists {
			ts = &msgStreamToolState{toolIndex: tIdx}
			w.msgTools[tIdx] = ts
		}
		callID := ev.PublicCallID
		if callID == "" {
			callID = ev.Event.CallID
		}
		if ts.callID == "" && callID != "" {
			ts.callID = callID
		}
		if ts.name == "" && ev.Event.ToolName != "" {
			ts.name = ev.Event.ToolName
		}

		if (!w.msgHasActiveBlock || w.msgActiveKind != msgBlockTool) && !ts.started && ts.callID != "" && ts.name != "" {
			if err := w.startMessagesToolBlock(&buf, ts); err != nil {
				return nil, err
			}
		}

		if ev.Event.Text != "" {
			if w.msgHasActiveBlock && w.msgActiveKind == msgBlockTool && w.msgActiveToolIdx == tIdx && ts.started && !ts.stopped {
				idx := ts.blockIndex
				partialCopy := ev.Event.Text
				delta := messagesStreamDeltaWire{
					Type:        "input_json_delta",
					PartialJSON: &partialCopy,
				}
				if err := appendMessagesSSEEvent(&buf, messagesStreamEventWire{
					Type:  "content_block_delta",
					Index: &idx,
					Delta: &delta,
				}); err != nil {
					return nil, err
				}
			} else {
				ts.bufferedArgs.WriteString(ev.Event.Text)
			}
		}
		return buf.Bytes(), nil

	case canonical.EventContentBlockStop:
		callID := ev.PublicCallID
		if callID == "" {
			callID = ev.Event.CallID
		}
		var targetTool *msgStreamToolState
		if callID != "" {
			for _, ts := range w.msgTools {
				if ts.callID == callID {
					targetTool = ts
					break
				}
			}
		} else if w.msgHasActiveBlock && w.msgActiveKind == msgBlockTool {
			if ts, ok := w.msgTools[ev.Event.ToolIndex]; ok && (ev.Event.ToolIndex > 0 || ev.Event.ItemIndex != w.msgActiveItemIdx || w.msgActiveToolIdx == ev.Event.ToolIndex) {
				targetTool = ts
			}
		}
		if targetTool != nil {
			if targetTool.stopped {
				return nil, nil
			}
			if !targetTool.started {
				if w.msgHasActiveBlock {
					targetTool.pendingStop = true
					return nil, nil
				}
				if err := w.startMessagesToolBlock(&buf, targetTool); err != nil {
					return nil, err
				}
			}
			if w.msgHasActiveBlock && w.msgActiveKind == msgBlockTool && w.msgActiveToolIdx == targetTool.toolIndex {
				if err := w.closeMessagesActiveBlock(&buf); err != nil {
					return nil, err
				}
			} else {
				idx := targetTool.blockIndex
				targetTool.stopped = true
				if err := appendMessagesSSEEvent(&buf, messagesStreamEventWire{
					Type:  "content_block_stop",
					Index: &idx,
				}); err != nil {
					return nil, err
				}
			}
			if err := w.promoteQueuedMessagesTools(&buf); err != nil {
				return nil, err
			}
			return buf.Bytes(), nil
		}

		if w.msgHasActiveBlock && w.msgActiveKind != msgBlockTool {
			if err := w.closeMessagesActiveBlock(&buf); err != nil {
				return nil, err
			}
			if err := w.promoteQueuedMessagesTools(&buf); err != nil {
				return nil, err
			}
			return buf.Bytes(), nil
		}
		return nil, nil

	default:
		return nil, nil
	}
}

func (w *CanonicalStreamWriter) finishMessagesStream(ev clientresponse.CanonicalEvent) ([]byte, error) {
	var buf bytes.Buffer
	if err := w.ensureMessagesStarted(&buf); err != nil {
		return nil, err
	}
	if err := w.closeMessagesActiveBlock(&buf); err != nil {
		return nil, err
	}

	for _, tIdx := range w.sortedMsgToolIndices() {
		ts := w.msgTools[tIdx]
		if !ts.started {
			if err := w.startMessagesToolBlock(&buf, ts); err != nil {
				return nil, err
			}
		}
		if !ts.stopped {
			if err := w.closeMessagesActiveBlock(&buf); err != nil {
				return nil, err
			}
		}
	}

	fin := ev.Finish
	if fin.Category == canonical.FinishNone {
		fin = ev.Event.Finish
	}
	stopReason, stopSeq, err := mapMessagesStopReasonAndSeq(fin)
	if err != nil {
		return nil, err
	}

	delta := messagesStreamDeltaWire{
		StopReason:   stopReason,
		StopSequence: stopSeq,
	}
	if err := appendMessagesSSEEvent(&buf, messagesStreamEventWire{
		Type:  "message_delta",
		Delta: &delta,
		Usage: nonNullJSONRaw(ev.FinalUsage),
	}); err != nil {
		return nil, err
	}
	if err := appendMessagesSSEEvent(&buf, messagesStreamEventWire{
		Type: "message_stop",
	}); err != nil {
		return nil, err
	}

	w.completed = true
	return buf.Bytes(), nil
}

func (w *CanonicalStreamWriter) failMessagesStream(ev clientresponse.CanonicalEvent) ([]byte, error) {
	var buf bytes.Buffer
	payload := nonNullJSONRaw(ev.FailureBody)
	if len(payload) == 0 {
		if cleanErr := nonNullJSONRaw(ev.Error); len(cleanErr) > 0 {
			payload = json.RawMessage(`{"type":"error","error":` + string(cleanErr) + `}`)
		} else {
			payload = json.RawMessage(`{"type":"error","error":{"type":"api_error","message":"The upstream generation was interrupted."}}`)
		}
	}
	buf.WriteString("event: error\ndata: ")
	buf.Write(payload)
	buf.WriteString("\n\n")
	w.failed = true
	return buf.Bytes(), nil
}
