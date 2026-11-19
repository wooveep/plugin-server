package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/canonical"
	"github.com/higress-group/wasm-go/pkg/ai/fixedtools"
)

type responsesWireStreamOptions struct {
	IncludeUsage             wireOptional[bool]   `json:"include_usage"`
	ReasoningSummaryDelivery wireOptional[string] `json:"reasoning_summary_delivery"`
}

type responsesWireReasoningConfig struct {
	Effort          wireOptional[string] `json:"effort"`
	Summary         wireOptional[string] `json:"summary"`
	GenerateSummary wireOptional[string] `json:"generate_summary"`
	Context         wireOptional[string] `json:"context"`
}

type responsesWireTextFormat struct {
	Type        string             `json:"type"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Schema      json.RawMessage    `json:"schema"`
	Strict      wireOptional[bool] `json:"strict"`
}

type responsesWireTextConfig struct {
	Verbosity wireOptional[string]                  `json:"verbosity"`
	Format    wireOptional[responsesWireTextFormat] `json:"format"`
}

type responsesWireTool struct {
	Type         string                `json:"type"`
	Name         string                `json:"name"`
	Description  string                `json:"description"`
	Parameters   json.RawMessage       `json:"parameters"`
	Strict       wireOptional[bool]    `json:"strict"`
	CacheControl *chatWireCacheControl `json:"cache_control"`
}

type responsesWireSummaryPart struct {
	Type string               `json:"type"`
	Text wireOptional[string] `json:"text"`
}

type responsesWireContentPart struct {
	Type                  string                   `json:"type"`
	Text                  wireOptional[string]     `json:"text"`
	ImageURL              string                   `json:"image_url"`
	FileID                string                   `json:"file_id"`
	Detail                string                   `json:"detail"`
	Refusal               wireOptional[string]     `json:"refusal"`
	Annotations           []json.RawMessage        `json:"annotations"`
	CacheControl          *chatWireCacheControl    `json:"cache_control"`
	PromptCacheBreakpoint *chatWireCacheBreakpoint `json:"prompt_cache_breakpoint"`
}

type responsesWireInputItem struct {
	Type                  string                                   `json:"type"`
	ID                    string                                   `json:"id"`
	Status                string                                   `json:"status"`
	Phase                 string                                   `json:"phase"`
	Role                  string                                   `json:"role"`
	Content               json.RawMessage                          `json:"content"`
	CallID                string                                   `json:"call_id"`
	Name                  string                                   `json:"name"`
	Arguments             wireOptional[string]                     `json:"arguments"`
	Output                json.RawMessage                          `json:"output"`
	Summary               wireOptional[[]responsesWireSummaryPart] `json:"summary"`
	EncryptedContent      json.RawMessage                          `json:"encrypted_content"`
	CacheControl          *chatWireCacheControl                    `json:"cache_control"`
	PromptCacheBreakpoint *chatWireCacheBreakpoint                 `json:"prompt_cache_breakpoint"`
}

type responsesWireInRequest struct {
	Model                 wireOptional[string]                       `json:"model"`
	Instructions          wireOptional[string]                       `json:"instructions"`
	Input                 json.RawMessage                            `json:"input"`
	Stream                wireOptional[bool]                         `json:"stream"`
	StreamOptions         wireOptional[responsesWireStreamOptions]   `json:"stream_options"`
	Temperature           wireOptional[float64]                      `json:"temperature"`
	TopP                  wireOptional[float64]                      `json:"top_p"`
	PresencePenalty       wireOptional[float64]                      `json:"presence_penalty"`
	FrequencyPenalty      wireOptional[float64]                      `json:"frequency_penalty"`
	MaxOutputTokens       wireOptional[int]                          `json:"max_output_tokens"`
	User                  wireOptional[string]                       `json:"user"`
	Metadata              wireOptional[map[string]string]            `json:"metadata"`
	Store                 wireOptional[bool]                         `json:"store"`
	Background            wireOptional[bool]                         `json:"background"`
	Reasoning             wireOptional[responsesWireReasoningConfig] `json:"reasoning"`
	PromptCacheKey        wireOptional[string]                       `json:"prompt_cache_key"`
	PromptCacheRetention  wireOptional[string]                       `json:"prompt_cache_retention"`
	PromptCacheOptions    wireOptional[chatWireCacheBreakpoint]      `json:"prompt_cache_options"`
	Text                  wireOptional[responsesWireTextConfig]      `json:"text"`
	Tools                 json.RawMessage                            `json:"tools"`
	ToolChoice            json.RawMessage                            `json:"tool_choice"`
	ParallelToolCalls     wireOptional[bool]                         `json:"parallel_tool_calls"`
	EnableSearch          wireOptional[bool]                         `json:"enable_search"`
	EnableCodeInterpreter wireOptional[bool]                         `json:"enable_code_interpreter"`
	SearchOptions         json.RawMessage                            `json:"search_options"`
	WebSearchOptions      json.RawMessage                            `json:"web_search_options"`
	ServiceTier           wireOptional[string]                       `json:"service_tier"`
	MaxToolCalls          wireOptional[int]                          `json:"max_tool_calls"`
	TopLogprobs           wireOptional[int]                          `json:"top_logprobs"`
}

const responsesSupportedTopLevelFields = " model instructions input stream stream_options temperature top_p presence_penalty frequency_penalty max_output_tokens user metadata store background reasoning prompt_cache_key prompt_cache_retention prompt_cache_options text tools tool_choice parallel_tool_calls previous_response_id conversation enable_search enable_code_interpreter search_options web_search_options truncation include service_tier safety_identifier max_tool_calls top_logprobs context_management prompt moderation client_metadata "

func decodeResponsesContentParts(raw json.RawMessage, role canonical.Role, allowEmptyArray bool) ([]canonical.Part, bool, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, false, canonical.NewRejectionError("invalid_message_content", "content", "message or function output content is required")
	}

	switch trimmed[0] {
	case '"':
		var text string
		if err := json.Unmarshal(trimmed, &text); err != nil {
			return nil, false, canonical.NewRejectionError("invalid_message_content", "content", fmt.Sprintf("invalid string content: %v", err))
		}
		return []canonical.Part{{Kind: canonical.PartText, Text: text}}, false, nil
	case '[':
		var wireParts []responsesWireContentPart
		if err := json.Unmarshal(trimmed, &wireParts); err != nil {
			return nil, false, canonical.NewRejectionError("invalid_message_content", "content", fmt.Sprintf("invalid content array: %v", err))
		}
		if len(wireParts) == 0 {
			if allowEmptyArray {
				return []canonical.Part{{Kind: canonical.PartText, Text: ""}}, false, nil
			}
			return nil, false, canonical.NewRejectionError("invalid_message_content", "content", "message or function output content must not be empty")
		}
		parts := make([]canonical.Part, 0, len(wireParts))
		for _, wp := range wireParts {
			pCache, err := parseChatCacheBoundary(wp.CacheControl, wp.PromptCacheBreakpoint, "input.content")
			if err != nil {
				return nil, false, err
			}
			switch wp.Type {
			case "input_text", "output_text":
				if !wp.Text.toCanonical().IsPresent() {
					return nil, false, canonical.NewRejectionError("invalid_message_content", "content", "text content part requires a string text field")
				}
				parts = append(parts, canonical.Part{
					Kind:        canonical.PartText,
					Text:        wp.Text.val,
					Annotations: cloneRawSlice(wp.Annotations),
					Cache:       pCache,
				})
			case "input_image":
				if role != canonical.RoleUser {
					return nil, false, canonical.NewRejectionError("unsupported_image_content", "input_image", "input_image is supported only on user messages; tool outputs support text only across protocols")
				}
				detail := canonical.ImageDetail(wp.Detail)
				if !detail.Valid() {
					return nil, false, canonical.NewRejectionError("unsupported_image_content", "input_image", fmt.Sprintf("image detail %q is unsupported", wp.Detail))
				}
				if wp.FileID != "" {
					parts = append(parts, canonical.Part{
						Kind: canonical.PartImage,
						Image: &canonical.ImagePart{
							Source: canonical.ImageFileID,
							URL:    wp.ImageURL,
							FileID: wp.FileID,
							Detail: detail,
						},
						Cache: pCache,
					})
					continue
				}
				if wp.ImageURL == "" {
					return nil, false, canonical.NewRejectionError("unsupported_image_content", "input_image", "input_image requires image_url")
				}
				imgPart, err := parseCanonicalImageURL(wp.ImageURL, detail, "input_image")
				if err != nil {
					return nil, false, err
				}
				parts = append(parts, canonical.Part{
					Kind:  canonical.PartImage,
					Image: imgPart,
					Cache: pCache,
				})
			case "refusal":
				if role != canonical.RoleAssistant || !wp.Refusal.toCanonical().IsPresent() || len(wireParts) != 1 {
					return nil, false, canonical.NewRejectionError("invalid_refusal_content", "refusal", "refusal requires a single assistant refusal content part")
				}
				parts = append(parts, canonical.Part{
					Kind:    canonical.PartRefusal,
					Refusal: wp.Refusal.val,
					Cache:   pCache,
				})
			default:
				field := canonical.SafeIdentifier(wp.Type)
				if field == "" {
					field = "content"
				}
				return nil, false, canonical.NewRejectionError("unsupported_content_part_type", field, fmt.Sprintf("content part type %q is unsupported", wp.Type))
			}
		}
		return parts, true, nil
	default:
		return nil, false, canonical.NewRejectionError("invalid_message_content", "content", "content must be a string or array of content parts")
	}
}

func decodeResponsesCanonical(body []byte) (canonical.Request, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil || fields == nil {
		return canonical.Request{}, canonical.NewRejectionError("invalid_json_request", "", fmt.Sprintf("invalid Responses request JSON: %v", err))
	}

	for name := range fields {
		if !strings.Contains(responsesSupportedTopLevelFields, " "+name+" ") {
			return canonical.Request{}, canonical.NewRejectionError("unsupported_top_level_field", name, fmt.Sprintf("%s is unsupported in Responses cross-protocol mode", name))
		}
	}

	// H1: prompt: null is ignored; any non-null prompt is rejected as a stateful hosted template reference.
	if rawPrompt, exists := fields["prompt"]; exists {
		trimmedPrompt := bytes.TrimSpace(rawPrompt)
		if len(trimmedPrompt) > 0 && !bytes.Equal(trimmedPrompt, []byte("null")) {
			return canonical.Request{}, canonical.NewRejectionError("unsupported_stateful_reference", "prompt", "prompt template reference is unsupported across protocols")
		}
	}

	if isNonEmptyJSONReference(fields["previous_response_id"]) {
		return canonical.Request{}, canonical.NewRejectionError("unsupported_stateful_reference", "previous_response_id", "previous_response_id is unsupported across protocols")
	}
	if isNonEmptyJSONReference(fields["conversation"]) {
		return canonical.Request{}, canonical.NewRejectionError("unsupported_stateful_reference", "conversation", "conversation is unsupported across protocols")
	}

	var raw responsesWireInRequest
	if err := json.Unmarshal(body, &raw); err != nil {
		return canonical.Request{}, canonical.NewRejectionError("invalid_request_schema", "", fmt.Sprintf("unable to unmarshal Responses request: %v", err))
	}
	if raw.Model.state == canonical.Null {
		return canonical.Request{}, canonical.NewRejectionError("invalid_model", "model", "model cannot be null")
	}
	if raw.Stream.state == canonical.Null {
		return canonical.Request{}, canonical.NewRejectionError("invalid_stream_flag", "stream", "stream cannot be null")
	}
	if raw.Background.state == canonical.Present && raw.Background.val {
		return canonical.Request{}, canonical.NewRejectionError("unsupported_background_mode", "background", "background mode is unsupported across protocols")
	}

	req := canonical.Request{
		SourceProtocol:    canonical.ProtocolOpenAIResponses,
		Model:             raw.Model.toCanonical().ValueOr(""),
		Stream:            raw.Stream.toCanonical().ValueOr(false),
		Instructions:      raw.Instructions.toCanonical(),
		ParallelToolCalls: raw.ParallelToolCalls.toCanonical(),
		Store:             raw.Store.toCanonical(),
		ServiceTier:       raw.ServiceTier.toCanonical(),
		User:              raw.User.toCanonical(),
		Metadata:          raw.Metadata.toCanonical(),
		Sampling: canonical.Sampling{
			Temperature:      raw.Temperature.toCanonical(),
			TopP:             raw.TopP.toCanonical(),
			PresencePenalty:  raw.PresencePenalty.toCanonical(),
			FrequencyPenalty: raw.FrequencyPenalty.toCanonical(),
			TopLogprobs:      raw.TopLogprobs.toCanonical(),
		},
		Cache: canonical.CachePolicy{
			PromptCacheKey:       raw.PromptCacheKey.toCanonical(),
			PromptCacheRetention: raw.PromptCacheRetention.toCanonical(),
		},
	}

	if raw.StreamOptions.state == canonical.Present {
		req.IncludeUsage = raw.StreamOptions.val.IncludeUsage.toCanonical()
	}
	if raw.PromptCacheOptions.state == canonical.Present {
		req.Cache.RequestBoundary = &canonical.CacheBoundary{
			Origin: canonical.CacheOriginOpenAIPromptCache,
			Mode:   raw.PromptCacheOptions.val.Mode,
			TTL:    raw.PromptCacheOptions.val.TTL,
		}
	}
	if raw.MaxOutputTokens.state == canonical.Present {
		req.Limits = canonical.OutputLimit{
			Source: canonical.LimitMaxOutputTokens,
			Scope:  canonical.ScopeReasoningAndVisible,
			Value:  canonical.Some(raw.MaxOutputTokens.val),
		}
	}

	if raw.Reasoning.state == canonical.Present {
		if raw.Reasoning.val.Effort.state == canonical.Present && raw.Reasoning.val.Effort.val != "" {
			effort := raw.Reasoning.val.Effort.val
			req.Thinking.Effort = canonical.Some(effort)
			if effort == "none" {
				req.Thinking.Mode = canonical.ThinkingDisabled
			} else {
				req.Thinking.Mode = canonical.ThinkingEnabled
			}
		}
	}

	if raw.Text.state == canonical.Present && raw.Text.val.Format.state == canonical.Present {
		format := raw.Text.val.Format.val
		switch format.Type {
		case "text":
			req.ResponseFormat = canonical.ResponseFormat{Kind: canonical.ResponseFormatText}
		case "json_object":
			req.ResponseFormat = canonical.ResponseFormat{Kind: canonical.ResponseFormatJSONObject}
		case "json_schema":
			req.ResponseFormat = canonical.ResponseFormat{
				Kind:        canonical.ResponseFormatJSONSchema,
				Name:        format.Name,
				Description: format.Description,
				Schema:      bytes.Clone(format.Schema),
				Strict:      format.Strict.toCanonical(),
			}
		default:
			return canonical.Request{}, canonical.NewRejectionError("unsupported_text_format", "text.format", fmt.Sprintf("text.format type %q is unsupported", format.Type))
		}
	}

	fixedControls, err := decodeFixedToolOptions(raw.EnableSearch, raw.EnableCodeInterpreter, raw.SearchOptions, raw.WebSearchOptions)
	if err != nil {
		return canonical.Request{}, err
	}
	req.FixedTools = fixedControls

	if trimmedTools := bytes.TrimSpace(raw.Tools); len(trimmedTools) > 0 {
		if bytes.Equal(trimmedTools, []byte("null")) {
			return canonical.Request{}, canonical.NewRejectionError("invalid_tool_declarations", "tools", "tools cannot be null")
		}
		var rawToolItems []json.RawMessage
		if err := json.Unmarshal(trimmedTools, &rawToolItems); err != nil {
			return canonical.Request{}, canonical.NewRejectionError("invalid_tool_declarations", "tools", fmt.Sprintf("invalid tools array: %v", err))
		}
		req.Tools = make([]canonical.ToolDefinition, 0, len(rawToolItems))
		for _, rawTool := range rawToolItems {
			var wt responsesWireTool
			if err := json.Unmarshal(rawTool, &wt); err != nil {
				return canonical.Request{}, canonical.NewRejectionError("invalid_tool_declarations", "tools", fmt.Sprintf("invalid tool definition: %v", err))
			}
			tCache, err := parseChatCacheBoundary(wt.CacheControl, nil, "tools")
			if err != nil {
				return canonical.Request{}, err
			}
			if wt.Type == "function" {
				if wt.Name == "" {
					return canonical.Request{}, canonical.NewRejectionError("invalid_tool_definition", "tools", "function tool requires name")
				}
				req.Tools = append(req.Tools, canonical.ToolDefinition{
					Kind:        canonical.ToolKindFunction,
					Name:        wt.Name,
					Description: wt.Description,
					Schema:      bytes.Clone(wt.Parameters),
					Strict:      wt.Strict.toCanonical(),
					Cache:       tCache,
				})
				continue
			}
			if _, ok := fixedtools.Kind(wt.Type); ok {
				req.Tools = append(req.Tools, canonical.ToolDefinition{
					Kind:      canonical.ToolKindFixedServer,
					FixedType: wt.Type,
					Name:      wt.Name,
					RawFixed:  bytes.Clone(rawTool),
					Cache:     tCache,
				})
				continue
			}
			field := canonical.SafeIdentifier(wt.Type)
			if field == "" {
				field = "tools"
			}
			return canonical.Request{}, canonical.NewRejectionError("unsupported_tool_type", field, fmt.Sprintf("%s is unsupported in Responses cross-protocol mode", wt.Type))
		}
	}

	if trimmedTC := bytes.TrimSpace(raw.ToolChoice); len(trimmedTC) > 0 && !bytes.Equal(trimmedTC, []byte("null")) {
		if trimmedTC[0] == '"' {
			var choiceStr string
			if err := json.Unmarshal(trimmedTC, &choiceStr); err != nil {
				return canonical.Request{}, canonical.NewRejectionError("unsupported_tool_choice", "tool_choice", "invalid tool_choice string")
			}
			switch choiceStr {
			case "":
				req.ToolChoice = canonical.ToolChoice{Mode: canonical.ToolChoiceUnset}
			case "auto":
				req.ToolChoice = canonical.ToolChoice{Mode: canonical.ToolChoiceAuto}
			case "none":
				req.ToolChoice = canonical.ToolChoice{Mode: canonical.ToolChoiceNone}
			case "required":
				req.ToolChoice = canonical.ToolChoice{Mode: canonical.ToolChoiceRequired}
			default:
				return canonical.Request{}, canonical.NewRejectionError("unsupported_tool_choice", "tool_choice", fmt.Sprintf("tool_choice %q is unsupported", choiceStr))
			}
		} else if trimmedTC[0] == '{' {
			var choiceObj struct {
				Type string `json:"type"`
				Name string `json:"name"`
			}
			if err := json.Unmarshal(trimmedTC, &choiceObj); err != nil {
				return canonical.Request{}, canonical.NewRejectionError("unsupported_tool_choice", "tool_choice", fmt.Sprintf("invalid tool_choice: %v", err))
			}
			switch choiceObj.Type {
			case "auto":
				req.ToolChoice = canonical.ToolChoice{Mode: canonical.ToolChoiceAuto}
			case "none":
				req.ToolChoice = canonical.ToolChoice{Mode: canonical.ToolChoiceNone}
			case "required":
				req.ToolChoice = canonical.ToolChoice{Mode: canonical.ToolChoiceRequired}
			case "function":
				if choiceObj.Name == "" {
					return canonical.Request{}, canonical.NewRejectionError("unsupported_tool_choice", "tool_choice", "function tool_choice requires name")
				}
				req.ToolChoice = canonical.ToolChoice{Mode: canonical.ToolChoiceFunction, Name: choiceObj.Name}
			default:
				if _, ok := fixedtools.Kind(choiceObj.Type); ok {
					req.ToolChoice = canonical.ToolChoice{Mode: canonical.ToolChoiceFixed, Name: choiceObj.Name, FixedRaw: choiceObj.Type}
				} else {
					return canonical.Request{}, canonical.NewRejectionError("unsupported_tool_choice", "tool_choice", fmt.Sprintf("tool_choice type %q is unsupported", choiceObj.Type))
				}
			}
		} else {
			return canonical.Request{}, canonical.NewRejectionError("unsupported_tool_choice", "tool_choice", "invalid tool_choice")
		}
	}

	trimmedInput := bytes.TrimSpace(raw.Input)
	if len(trimmedInput) == 0 || bytes.Equal(trimmedInput, []byte("null")) {
		return canonical.Request{}, canonical.NewRejectionError("missing_input", "input", "input is required")
	}

	if trimmedInput[0] == '"' {
		var textInput string
		if err := json.Unmarshal(trimmedInput, &textInput); err != nil {
			return canonical.Request{}, canonical.NewRejectionError("invalid_input_schema", "input", fmt.Sprintf("invalid string input: %v", err))
		}
		req.Items = []canonical.Item{{
			Kind:  canonical.ItemMessage,
			Role:  canonical.RoleUser,
			Parts: []canonical.Part{{Kind: canonical.PartText, Text: textInput}},
		}}
		return req, nil
	}

	if trimmedInput[0] != '[' {
		return canonical.Request{}, canonical.NewRejectionError("invalid_input_schema", "input", "input must be a string or array of items")
	}

	var wireItems []responsesWireInputItem
	if err := json.Unmarshal(trimmedInput, &wireItems); err != nil {
		return canonical.Request{}, canonical.NewRejectionError("invalid_input_schema", "input", fmt.Sprintf("invalid input items: %v", err))
	}
	if len(wireItems) == 0 && (!req.Instructions.IsPresent() || req.Instructions.Val == "") {
		return canonical.Request{}, canonical.NewRejectionError("missing_input", "input", "input must not be empty")
	}

	req.Items = make([]canonical.Item, 0, len(wireItems))
	pendingCalls := make(map[string]string)
	seenCallIDs := make(map[string]bool)
	inAssistantBatch := false

	for _, wi := range wireItems {
		switch wi.Type {
		case "reasoning":
			tc := bytes.TrimSpace(wi.Content)
			hasContent := len(tc) > 0 && !bytes.Equal(tc, []byte("null")) && !bytes.Equal(tc, []byte("[]"))
			var reasoningTexts []string
			if hasContent {
				var rParts []responsesWireContentPart
				if err := json.Unmarshal(tc, &rParts); err != nil || len(rParts) == 0 {
					return canonical.Request{}, canonical.NewRejectionError("unsupported_reasoning_content", "reasoning", "reasoning requires plaintext reasoning_text content")
				}
				var b strings.Builder
				for _, rp := range rParts {
					if rp.Type != "reasoning_text" || !rp.Text.toCanonical().IsPresent() {
						return canonical.Request{}, canonical.NewRejectionError("unsupported_reasoning_content", "reasoning", "reasoning content cannot be converted without loss")
					}
					b.WriteString(rp.Text.val)
				}
				reasoningTexts = []string{b.String()}
			}

			var summaryOpt canonical.Optional[[]string]
			switch wi.Summary.state {
			case canonical.Null:
				summaryOpt = canonical.NullOf[[]string]()
			case canonical.Present:
				sumTexts := make([]string, 0, len(wi.Summary.val))
				for _, sp := range wi.Summary.val {
					if sp.Text.toCanonical().IsPresent() {
						sumTexts = append(sumTexts, sp.Text.val)
					}
				}
				summaryOpt = canonical.Some(sumTexts)
			default:
				summaryOpt = canonical.AbsentOf[[]string]()
			}

			trimmedEnc := bytes.TrimSpace(wi.EncryptedContent)
			hasEncrypted := len(trimmedEnc) > 0 && !bytes.Equal(trimmedEnc, []byte("null")) && !bytes.Equal(trimmedEnc, []byte(`""`))
			var encRaw json.RawMessage
			if hasEncrypted {
				encRaw = bytes.Clone(trimmedEnc)
			}

			if len(reasoningTexts) == 0 && !hasEncrypted && !summaryOpt.IsPresent() {
				return canonical.Request{}, canonical.NewRejectionError("unsupported_reasoning_content", "reasoning", "bare reasoning item requires content, summary, or encrypted_content")
			}

			if len(reasoningTexts) > 0 {
				inAssistantBatch = true
			}
			req.Items = append(req.Items, canonical.Item{
				Kind: canonical.ItemReasoning,
				Role: canonical.RoleAssistant,
				Reasoning: &canonical.ReasoningItem{
					ItemID:    wi.ID,
					Text:      reasoningTexts,
					Summary:   summaryOpt,
					Encrypted: encRaw,
					Origin:    canonical.OpaqueOrigin{Protocol: canonical.ProtocolOpenAIResponses, Supplier: "openai"},
				},
			})

		case "", "message":
			role := canonical.Role(wi.Role)
			switch role {
			case canonical.RoleUser, canonical.RoleAssistant, canonical.RoleDeveloper, canonical.RoleSystem:
			default:
				return canonical.Request{}, canonical.NewRejectionError("unsupported_message_role", "input.role", fmt.Sprintf("message role %q is unsupported", wi.Role))
			}
			if len(pendingCalls) > 0 && (role != canonical.RoleAssistant || !inAssistantBatch) {
				return canonical.Request{}, canonical.NewRejectionError("pending_function_call_output", "function_call_output", "function_call_output is required before the next message")
			}

			parts, isArray, err := decodeResponsesContentParts(wi.Content, role, role == canonical.RoleAssistant)
			if err != nil {
				return canonical.Request{}, err
			}
			itemCache, err := parseChatCacheBoundary(wi.CacheControl, wi.PromptCacheBreakpoint, "input")
			if err != nil {
				return canonical.Request{}, err
			}
			req.Items = append(req.Items, canonical.Item{
				Kind:           canonical.ItemMessage,
				Role:           role,
				MessageID:      wi.ID,
				Parts:          parts,
				MessageIsArray: isArray,
				Cache:          itemCache,
			})
			inAssistantBatch = role == canonical.RoleAssistant

		case "function_call":
			if wi.CallID == "" || wi.Name == "" || !wi.Arguments.toCanonical().IsPresent() {
				return canonical.Request{}, canonical.NewRejectionError("invalid_function_call", "function_call", "function_call requires call_id, name, and string arguments")
			}
			if seenCallIDs[wi.CallID] {
				return canonical.Request{}, canonical.NewRejectionError("invalid_function_call", "function_call", "duplicate function_call call_id")
			}
			if len(pendingCalls) > 0 && !inAssistantBatch && len(req.Items) > 0 {
				return canonical.Request{}, canonical.NewRejectionError("invalid_function_call", "function_call", "all function_call_output items must precede the next function_call")
			}
			pendingCalls[wi.CallID] = wi.Name
			seenCallIDs[wi.CallID] = true
			inAssistantBatch = true
			req.Items = append(req.Items, canonical.Item{
				Kind: canonical.ItemFunctionCall,
				Role: canonical.RoleAssistant,
				Call: &canonical.FunctionCall{
					ItemID: wi.ID,
					CallID: wi.CallID,
					Name:   wi.Name,
					Args:   canonical.Arguments{Kind: canonical.ArgsText, Text: wi.Arguments.val},
				},
			})

		case "function_call_output":
			fnName, exists := pendingCalls[wi.CallID]
			if !exists {
				return canonical.Request{}, canonical.NewRejectionError("orphan_function_call_output", "function_call_output", "function_call_output call_id has no matching pending function_call")
			}
			parts, isArray, err := decodeResponsesContentParts(wi.Output, canonical.RoleTool, true)
			if err != nil {
				return canonical.Request{}, err
			}
			outCache, err := parseChatCacheBoundary(wi.CacheControl, wi.PromptCacheBreakpoint, "function_call_output")
			if err != nil {
				return canonical.Request{}, err
			}
			delete(pendingCalls, wi.CallID)
			inAssistantBatch = false
			req.Items = append(req.Items, canonical.Item{
				Kind: canonical.ItemFunctionResult,
				Role: canonical.RoleTool,
				Result: &canonical.FunctionResult{
					ItemID:       wi.ID,
					CallID:       wi.CallID,
					Name:         fnName,
					Parts:        parts,
					IsStringForm: !isArray,
					Cache:        outCache,
				},
			})

		default:
			field := canonical.SafeIdentifier(wi.Type)
			if field == "" {
				field = "input"
			}
			return canonical.Request{}, canonical.NewRejectionError("unsupported_input_item_type", field, fmt.Sprintf("input item type %q is unsupported", wi.Type))
		}
	}

	if len(pendingCalls) > 0 {
		return canonical.Request{}, canonical.NewRejectionError("pending_function_call_output", "function_call_output", "function_call_output is required for every function_call")
	}

	return req, nil
}
