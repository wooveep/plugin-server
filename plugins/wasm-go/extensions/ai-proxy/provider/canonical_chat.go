package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/canonical"
	"github.com/higress-group/wasm-go/pkg/ai/fixedtools"
)

type chatWireCacheControl struct {
	Type string `json:"type"`
	TTL  string `json:"ttl,omitempty"`
}

type chatWireCacheBreakpoint struct {
	Mode string `json:"mode,omitempty"`
	TTL  string `json:"ttl,omitempty"`
}

type chatWireStreamOptions struct {
	IncludeUsage wireOptional[bool] `json:"include_usage"`
}

type chatWireJSONSchema struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Schema      json.RawMessage    `json:"schema,omitempty"`
	Strict      wireOptional[bool] `json:"strict"`
}

type chatWireResponseFormat struct {
	Type       string              `json:"type"`
	JSONSchema *chatWireJSONSchema `json:"json_schema,omitempty"`
}

type chatWireThinking struct {
	Type         string            `json:"type"`
	BudgetTokens wireOptional[int] `json:"budget_tokens"`
	BudgetToken  wireOptional[int] `json:"budget_token"`
}

type chatWireGoogleExtra struct {
	ThoughtSignature string `json:"thought_signature,omitempty"`
}

type chatWireExtraContent struct {
	Google *chatWireGoogleExtra `json:"google,omitempty"`
}

type chatWireFunctionCall struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments"`
}

type chatWireInToolCall struct {
	Index            int                   `json:"index"`
	ID               string                `json:"id"`
	Type             string                `json:"type"`
	Function         chatWireFunctionCall  `json:"function"`
	ThoughtSignature string                `json:"thought_signature"`
	ExtraContent     *chatWireExtraContent `json:"extra_content"`
}

type chatWireImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type chatWireFileRef struct {
	FileID string `json:"file_id,omitempty"`
}

type chatWireInContentPart struct {
	Type                  string                   `json:"type"`
	Text                  wireOptional[string]     `json:"text"`
	Refusal               wireOptional[string]     `json:"refusal"`
	ImageURL              *chatWireImageURL        `json:"image_url,omitempty"`
	File                  *chatWireFileRef         `json:"file,omitempty"`
	Annotations           []json.RawMessage        `json:"annotations,omitempty"`
	CacheControl          *chatWireCacheControl    `json:"cache_control,omitempty"`
	PromptCacheBreakpoint *chatWireCacheBreakpoint `json:"prompt_cache_breakpoint,omitempty"`
}

type chatWireInMessage struct {
	ID                       string                   `json:"id"`
	Name                     string                   `json:"name"`
	Role                     string                   `json:"role"`
	Content                  json.RawMessage          `json:"content"`
	Partial                  wireOptional[bool]       `json:"partial"`
	ReasoningContent         wireOptional[string]     `json:"reasoning_content"`
	Reasoning                wireOptional[string]     `json:"reasoning"`
	ReasoningSignature       string                   `json:"reasoning_signature"`
	ReasoningRedactedContent string                   `json:"reasoning_redacted_content"`
	Refusal                  wireOptional[string]     `json:"refusal"`
	Annotations              []json.RawMessage        `json:"annotations"`
	ToolCalls                []chatWireInToolCall     `json:"tool_calls"`
	FunctionCall             *chatWireFunctionCall    `json:"function_call"`
	ToolCallID               string                   `json:"tool_call_id"`
	CacheControl             *chatWireCacheControl    `json:"cache_control"`
	PromptCacheBreakpoint    *chatWireCacheBreakpoint `json:"prompt_cache_breakpoint"`
}

type chatWireInFunctionDef struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Parameters  json.RawMessage    `json:"parameters"`
	Strict      wireOptional[bool] `json:"strict"`
}

type chatWireInTool struct {
	Type         string                 `json:"type"`
	Name         string                 `json:"name"`
	Function     *chatWireInFunctionDef `json:"function"`
	CacheControl *chatWireCacheControl  `json:"cache_control"`
}

type chatWireInRequest struct {
	Model                 wireOptional[string]                  `json:"model"`
	Messages              json.RawMessage                       `json:"messages"`
	Stream                wireOptional[bool]                    `json:"stream"`
	StreamOptions         wireOptional[chatWireStreamOptions]   `json:"stream_options"`
	Temperature           wireOptional[float64]                 `json:"temperature"`
	TopP                  wireOptional[float64]                 `json:"top_p"`
	TopK                  wireOptional[int]                     `json:"top_k"`
	PresencePenalty       wireOptional[float64]                 `json:"presence_penalty"`
	FrequencyPenalty      wireOptional[float64]                 `json:"frequency_penalty"`
	RepetitionPenalty     wireOptional[float64]                 `json:"repetition_penalty"`
	Seed                  wireOptional[int64]                   `json:"seed"`
	Stop                  json.RawMessage                       `json:"stop"`
	N                     wireOptional[int]                     `json:"n"`
	Logprobs              wireOptional[bool]                    `json:"logprobs"`
	TopLogprobs           wireOptional[int]                     `json:"top_logprobs"`
	MaxTokens             wireOptional[int]                     `json:"max_tokens"`
	MaxCompletionTokens   wireOptional[int]                     `json:"max_completion_tokens"`
	ResponseFormat        json.RawMessage                       `json:"response_format"`
	Tools                 json.RawMessage                       `json:"tools"`
	ToolChoice            json.RawMessage                       `json:"tool_choice"`
	ParallelToolCalls     wireOptional[bool]                    `json:"parallel_tool_calls"`
	EnableSearch          wireOptional[bool]                    `json:"enable_search"`
	EnableCodeInterpreter wireOptional[bool]                    `json:"enable_code_interpreter"`
	SearchOptions         json.RawMessage                       `json:"search_options"`
	WebSearchOptions      json.RawMessage                       `json:"web_search_options"`
	ReasoningEffort       wireOptional[string]                  `json:"reasoning_effort"`
	EnableThinking        wireOptional[bool]                    `json:"enable_thinking"`
	ThinkingBudget        wireOptional[int]                     `json:"thinking_budget"`
	PreserveThinking      wireOptional[bool]                    `json:"preserve_thinking"`
	Thinking              wireOptional[chatWireThinking]        `json:"thinking"`
	Store                 wireOptional[bool]                    `json:"store"`
	ServiceTier           wireOptional[string]                  `json:"service_tier"`
	User                  wireOptional[string]                  `json:"user"`
	Metadata              wireOptional[map[string]string]       `json:"metadata"`
	PromptCacheKey        wireOptional[string]                  `json:"prompt_cache_key"`
	PromptCacheRetention  wireOptional[string]                  `json:"prompt_cache_retention"`
	PromptCacheOptions    wireOptional[chatWireCacheBreakpoint] `json:"prompt_cache_options"`
}

func parseChatCacheBoundary(cc *chatWireCacheControl, bp *chatWireCacheBreakpoint, field string) (*canonical.CacheBoundary, error) {
	if cc != nil && bp != nil {
		return nil, canonical.NewRejectionError("conflicting_cache_breakpoints", field, "cannot specify both cache_control and prompt_cache_breakpoint")
	}
	if cc != nil {
		if cc.Type != "" && cc.Type != "ephemeral" {
			return nil, canonical.NewRejectionError("invalid_cache_control", field, "cache_control.type must be ephemeral")
		}
		mode := cc.Type
		if mode == "" {
			mode = "ephemeral"
		}
		return &canonical.CacheBoundary{
			Origin: canonical.CacheOriginEphemeralControl,
			Mode:   mode,
			TTL:    cc.TTL,
		}, nil
	}
	if bp != nil {
		return &canonical.CacheBoundary{
			Origin: canonical.CacheOriginOpenAIPromptCache,
			Mode:   bp.Mode,
			TTL:    bp.TTL,
		}, nil
	}
	return nil, nil
}

func decodeFixedToolOptions(
	enableSearch wireOptional[bool],
	enableCodeInterpreter wireOptional[bool],
	searchOptions json.RawMessage,
	webSearchOptions json.RawMessage,
) (canonical.FixedToolControls, error) {
	if enableSearch.state == canonical.Null {
		return canonical.FixedToolControls{}, canonical.NewRejectionError("invalid_tool_flag", "enable_search", "enable_search cannot be null")
	}
	if enableCodeInterpreter.state == canonical.Null {
		return canonical.FixedToolControls{}, canonical.NewRejectionError("invalid_tool_flag", "enable_code_interpreter", "enable_code_interpreter cannot be null")
	}
	out := canonical.FixedToolControls{
		EnableSearch:          enableSearch.toCanonical(),
		EnableCodeInterpreter: enableCodeInterpreter.toCanonical(),
	}
	if trimmed := bytes.TrimSpace(searchOptions); len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null")) {
		if err := json.Unmarshal(trimmed, &out.SearchOptions); err != nil {
			return canonical.FixedToolControls{}, canonical.NewRejectionError("invalid_search_options", "search_options", "search_options must be an object")
		}
		var strategy wireOptional[string]
		for name, raw := range out.SearchOptions {
			if !strings.EqualFold(name, "strategy") {
				continue
			}
			if strategy.state != canonical.Absent || json.Unmarshal(raw, &strategy) != nil {
				return canonical.FixedToolControls{}, canonical.NewRejectionError("invalid_search_options", "search_options.strategy", "search_options.strategy must be a single string or null")
			}
			delete(out.SearchOptions, name)
		}
		out.SearchStrategy = strategy.toCanonical()
	}
	if trimmed := bytes.TrimSpace(webSearchOptions); len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null")) {
		out.WebSearchOptionsRaw = bytes.Clone(trimmed)
	}
	return out, nil
}

func encodeFixedToolSearchOptions(tools canonical.FixedToolControls) (json.RawMessage, error) {
	if tools.SearchStrategy.IsAbsent() {
		if tools.SearchOptions == nil {
			return nil, nil
		}
		return json.Marshal(tools.SearchOptions)
	}
	options := maps.Clone(tools.SearchOptions)
	if options == nil {
		options = make(map[string]json.RawMessage, 1)
	}
	options["strategy"] = json.RawMessage("null")
	if tools.SearchStrategy.IsPresent() {
		options["strategy"], _ = json.Marshal(tools.SearchStrategy.Val)
	}
	return json.Marshal(options)
}

func decodeChatCanonical(body []byte) (canonical.Request, error) {
	trimmedBody := bytes.TrimSpace(body)
	if len(trimmedBody) == 0 || trimmedBody[0] != '{' {
		return canonical.Request{}, canonical.NewRejectionError("invalid_json_request", "", "Chat request body must be a JSON object")
	}

	var raw chatWireInRequest
	if err := json.Unmarshal(trimmedBody, &raw); err != nil {
		return canonical.Request{}, canonical.NewRejectionError("invalid_request_schema", "", fmt.Sprintf("invalid Chat request: %v", err))
	}
	if raw.Model.state == canonical.Null {
		return canonical.Request{}, canonical.NewRejectionError("invalid_model", "model", "model cannot be null")
	}
	if raw.Stream.state == canonical.Null {
		return canonical.Request{}, canonical.NewRejectionError("invalid_stream_flag", "stream", "stream cannot be null")
	}

	trimmedMessages := bytes.TrimSpace(raw.Messages)
	if len(trimmedMessages) == 0 || bytes.Equal(trimmedMessages, []byte("null")) {
		return canonical.Request{}, canonical.NewRejectionError("missing_messages", "messages", "messages is required")
	}
	var wireMessages []chatWireInMessage
	if err := json.Unmarshal(trimmedMessages, &wireMessages); err != nil {
		return canonical.Request{}, canonical.NewRejectionError("invalid_messages", "messages", fmt.Sprintf("invalid messages array: %v", err))
	}
	if len(wireMessages) == 0 {
		return canonical.Request{}, canonical.NewRejectionError("missing_messages", "messages", "messages must not be empty")
	}

	if raw.MaxTokens.state == canonical.Present && raw.MaxCompletionTokens.state == canonical.Present {
		return canonical.Request{}, canonical.NewRejectionError("conflicting_output_limits", "max_completion_tokens", "max_tokens and max_completion_tokens cannot both be set")
	}

	req := canonical.Request{
		SourceProtocol:    canonical.ProtocolOpenAIChat,
		Model:             raw.Model.toCanonical().ValueOr(""),
		Stream:            raw.Stream.toCanonical().ValueOr(false),
		ParallelToolCalls: raw.ParallelToolCalls.toCanonical(),
		Store:             raw.Store.toCanonical(),
		ServiceTier:       raw.ServiceTier.toCanonical(),
		User:              raw.User.toCanonical(),
		Metadata:          raw.Metadata.toCanonical(),
		Sampling: canonical.Sampling{
			Temperature:       raw.Temperature.toCanonical(),
			TopP:              raw.TopP.toCanonical(),
			TopK:              raw.TopK.toCanonical(),
			RepetitionPenalty: raw.RepetitionPenalty.toCanonical(),
			PresencePenalty:   raw.PresencePenalty.toCanonical(),
			FrequencyPenalty:  raw.FrequencyPenalty.toCanonical(),
			Seed:              raw.Seed.toCanonical(),
			N:                 raw.N.toCanonical(),
			Logprobs:          raw.Logprobs.toCanonical(),
			TopLogprobs:       raw.TopLogprobs.toCanonical(),
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

	if raw.MaxCompletionTokens.state == canonical.Present {
		req.Limits = canonical.OutputLimit{
			Source: canonical.LimitMaxCompletionTokens,
			Scope:  canonical.ScopeReasoningAndVisible,
			Value:  canonical.Some(raw.MaxCompletionTokens.val),
		}
	} else if raw.MaxTokens.state == canonical.Present {
		req.Limits = canonical.OutputLimit{
			Source: canonical.LimitMaxTokens,
			Scope:  canonical.ScopeVisibleOnly,
			Value:  canonical.Some(raw.MaxTokens.val),
		}
	}

	// OA03: stop accepts single string or array of 1..4 strings; reject empty array, >4, or mixed types.
	if trimmedStop := bytes.TrimSpace(raw.Stop); len(trimmedStop) > 0 && !bytes.Equal(trimmedStop, []byte("null")) {
		switch trimmedStop[0] {
		case '"':
			var single string
			if err := json.Unmarshal(trimmedStop, &single); err != nil {
				return canonical.Request{}, canonical.NewRejectionError("invalid_stop", "stop", "invalid stop string")
			}
			req.Sampling.Stop = []string{single}
			req.Sampling.StopPresent = true
		case '[':
			var rawItems []json.RawMessage
			if err := json.Unmarshal(trimmedStop, &rawItems); err != nil {
				return canonical.Request{}, canonical.NewRejectionError("invalid_stop", "stop", "invalid stop array")
			}
			if len(rawItems) == 0 {
				return canonical.Request{}, canonical.NewRejectionError("invalid_stop", "stop", "stop array must not be empty")
			}
			if len(rawItems) > 4 {
				return canonical.Request{}, canonical.NewRejectionError("invalid_stop", "stop", "stop array exceeds maximum of 4 sequences")
			}
			stops := make([]string, len(rawItems))
			for i, elem := range rawItems {
				te := bytes.TrimSpace(elem)
				if len(te) == 0 || te[0] != '"' {
					return canonical.Request{}, canonical.NewRejectionError("invalid_stop", "stop", "stop array elements must all be strings")
				}
				if err := json.Unmarshal(te, &stops[i]); err != nil {
					return canonical.Request{}, canonical.NewRejectionError("invalid_stop", "stop", "invalid stop string element")
				}
			}
			req.Sampling.Stop = stops
			req.Sampling.StopPresent = true
		default:
			return canonical.Request{}, canonical.NewRejectionError("invalid_stop", "stop", "stop must be a string or array of up to 4 strings")
		}
	}

	if trimmedRF := bytes.TrimSpace(raw.ResponseFormat); len(trimmedRF) > 0 && !bytes.Equal(trimmedRF, []byte("null")) {
		var rf chatWireResponseFormat
		if err := json.Unmarshal(trimmedRF, &rf); err != nil {
			return canonical.Request{}, canonical.NewRejectionError("invalid_response_format", "response_format", fmt.Sprintf("invalid response_format: %v", err))
		}
		switch rf.Type {
		case "text":
			req.ResponseFormat = canonical.ResponseFormat{Kind: canonical.ResponseFormatText}
		case "json_object":
			req.ResponseFormat = canonical.ResponseFormat{Kind: canonical.ResponseFormatJSONObject}
		case "json_schema":
			if rf.JSONSchema == nil || rf.JSONSchema.Name == "" {
				return canonical.Request{}, canonical.NewRejectionError("invalid_response_format", "response_format", "json_schema requires name")
			}
			req.ResponseFormat = canonical.ResponseFormat{
				Kind:        canonical.ResponseFormatJSONSchema,
				Name:        rf.JSONSchema.Name,
				Description: rf.JSONSchema.Description,
				Schema:      bytes.Clone(rf.JSONSchema.Schema),
				Strict:      rf.JSONSchema.Strict.toCanonical(),
			}
		default:
			return canonical.Request{}, canonical.NewRejectionError("unsupported_response_format", "response_format", fmt.Sprintf("unsupported response_format type %q", rf.Type))
		}
	}

	// Thinking configuration from Chat / OpenAI-compatible extensions.
	if raw.ReasoningEffort.state == canonical.Present && raw.ReasoningEffort.val != "" {
		req.Thinking.Effort = canonical.Some(raw.ReasoningEffort.val)
		if raw.ReasoningEffort.val == "none" {
			req.Thinking.Mode = canonical.ThinkingDisabled
		} else {
			req.Thinking.Mode = canonical.ThinkingEnabled
		}
	}
	if raw.Thinking.state == canonical.Present {
		switch raw.Thinking.val.Type {
		case "enabled":
			req.Thinking.Mode = canonical.ThinkingEnabled
		case "disabled":
			req.Thinking.Mode = canonical.ThinkingDisabled
		case "adaptive":
			req.Thinking.Mode = canonical.ThinkingAdaptive
		case "":
		default:
			return canonical.Request{}, canonical.NewRejectionError("invalid_thinking_config", "thinking.type", "unsupported thinking.type")
		}
		if raw.Thinking.val.BudgetTokens.state == canonical.Present {
			req.Thinking.BudgetTokens = canonical.Some(raw.Thinking.val.BudgetTokens.val)
		} else if raw.Thinking.val.BudgetToken.state == canonical.Present {
			req.Thinking.BudgetTokens = canonical.Some(raw.Thinking.val.BudgetToken.val)
		}
	}
	if raw.EnableThinking.state == canonical.Present {
		if raw.EnableThinking.val {
			if req.Thinking.Mode == canonical.ThinkingUnset || req.Thinking.Mode == canonical.ThinkingDisabled {
				req.Thinking.Mode = canonical.ThinkingEnabled
			}
		} else {
			req.Thinking.Mode = canonical.ThinkingDisabled
		}
	}
	if raw.ThinkingBudget.state == canonical.Present {
		req.Thinking.BudgetTokens = canonical.Some(raw.ThinkingBudget.val)
	}
	if raw.PreserveThinking.state == canonical.Present {
		req.Thinking.PreserveThinking = canonical.Some(raw.PreserveThinking.val)
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
			var wt chatWireInTool
			if err := json.Unmarshal(rawTool, &wt); err != nil {
				return canonical.Request{}, canonical.NewRejectionError("invalid_tool_declarations", "tools", fmt.Sprintf("invalid tool item: %v", err))
			}
			tCache, err := parseChatCacheBoundary(wt.CacheControl, nil, "tools")
			if err != nil {
				return canonical.Request{}, err
			}
			if wt.Type == "function" {
				if wt.Function == nil || wt.Function.Name == "" {
					return canonical.Request{}, canonical.NewRejectionError("invalid_tool_definition", "tools", "function tool requires function.name")
				}
				req.Tools = append(req.Tools, canonical.ToolDefinition{
					Kind:        canonical.ToolKindFunction,
					Name:        wt.Function.Name,
					Description: wt.Function.Description,
					Schema:      bytes.Clone(wt.Function.Parameters),
					Strict:      wt.Function.Strict.toCanonical(),
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
			return canonical.Request{}, canonical.NewRejectionError("unsupported_tool_type", field, fmt.Sprintf("unsupported Chat tool type %q", wt.Type))
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
				return canonical.Request{}, canonical.NewRejectionError("unsupported_tool_choice", "tool_choice", fmt.Sprintf("unsupported tool_choice %q", choiceStr))
			}
		} else if trimmedTC[0] == '{' {
			var choiceObj struct {
				Type     string                 `json:"type"`
				Name     string                 `json:"name"`
				Function *chatWireInFunctionDef `json:"function"`
			}
			if err := json.Unmarshal(trimmedTC, &choiceObj); err != nil {
				return canonical.Request{}, canonical.NewRejectionError("unsupported_tool_choice", "tool_choice", fmt.Sprintf("invalid tool_choice object: %v", err))
			}
			switch choiceObj.Type {
			case "auto":
				req.ToolChoice = canonical.ToolChoice{Mode: canonical.ToolChoiceAuto}
			case "none":
				req.ToolChoice = canonical.ToolChoice{Mode: canonical.ToolChoiceNone}
			case "required":
				req.ToolChoice = canonical.ToolChoice{Mode: canonical.ToolChoiceRequired}
			case "function":
				fnName := ""
				if choiceObj.Function != nil {
					fnName = choiceObj.Function.Name
				}
				if fnName == "" {
					fnName = choiceObj.Name
				}
				if fnName == "" {
					return canonical.Request{}, canonical.NewRejectionError("unsupported_tool_choice", "tool_choice", "function tool_choice requires function.name")
				}
				req.ToolChoice = canonical.ToolChoice{Mode: canonical.ToolChoiceFunction, Name: fnName}
			default:
				if _, ok := fixedtools.Kind(choiceObj.Type); ok {
					req.ToolChoice = canonical.ToolChoice{Mode: canonical.ToolChoiceFixed, Name: choiceObj.Name, FixedRaw: choiceObj.Type}
				} else {
					return canonical.Request{}, canonical.NewRejectionError("unsupported_tool_choice", "tool_choice", fmt.Sprintf("unsupported tool_choice type %q", choiceObj.Type))
				}
			}
		} else {
			return canonical.Request{}, canonical.NewRejectionError("unsupported_tool_choice", "tool_choice", "invalid tool_choice")
		}
	}

	req.Items = make([]canonical.Item, 0, len(wireMessages))
	toolNamesByCallID := make(map[string]string)

	for _, msg := range wireMessages {
		role := canonical.Role(msg.Role)
		switch role {
		case canonical.RoleSystem, canonical.RoleDeveloper, canonical.RoleUser, canonical.RoleAssistant, canonical.RoleTool:
		default:
			return canonical.Request{}, canonical.NewRejectionError("unsupported_message_role", "messages.role", fmt.Sprintf("unsupported Chat message role %q", msg.Role))
		}

		msgCache, err := parseChatCacheBoundary(msg.CacheControl, msg.PromptCacheBreakpoint, "messages")
		if err != nil {
			return canonical.Request{}, err
		}

		if role == canonical.RoleTool {
			if msg.ToolCallID == "" {
				return canonical.Request{}, canonical.NewRejectionError("invalid_tool_message", "tool_call_id", "tool message requires tool_call_id")
			}
			tc := bytes.TrimSpace(msg.Content)
			if len(tc) == 0 || bytes.Equal(tc, []byte("null")) {
				return canonical.Request{}, canonical.NewRejectionError("invalid_message_content", "messages.content", "tool message requires content")
			}
			var parts []canonical.Part
			isStr := false
			switch tc[0] {
			case '"':
				var s string
				if err := json.Unmarshal(tc, &s); err != nil {
					return canonical.Request{}, canonical.NewRejectionError("invalid_message_content", "messages.content", "invalid tool message string content")
				}
				parts = []canonical.Part{{Kind: canonical.PartText, Text: s}}
				isStr = true
			case '[':
				var wireParts []chatWireInContentPart
				if err := json.Unmarshal(tc, &wireParts); err != nil {
					return canonical.Request{}, canonical.NewRejectionError("invalid_message_content", "messages.content", "invalid tool message content parts")
				}
				if len(wireParts) == 0 {
					parts = []canonical.Part{{Kind: canonical.PartText, Text: ""}}
					isStr = true
				} else {
					parts = make([]canonical.Part, 0, len(wireParts))
					for _, wp := range wireParts {
						if wp.Type != "text" || !wp.Text.toCanonical().IsPresent() {
							return canonical.Request{}, canonical.NewRejectionError("unsupported_tool_content", "messages.content", "Chat tool messages support text parts only")
						}
						pCache, err := parseChatCacheBoundary(wp.CacheControl, wp.PromptCacheBreakpoint, "messages.content")
						if err != nil {
							return canonical.Request{}, err
						}
						parts = append(parts, canonical.Part{Kind: canonical.PartText, Text: wp.Text.val, Cache: pCache})
					}
				}
			default:
				return canonical.Request{}, canonical.NewRejectionError("invalid_message_content", "messages.content", "tool message content must be string or array")
			}

			fnName := msg.Name
			if fnName == "" {
				fnName = toolNamesByCallID[msg.ToolCallID]
			}
			req.Items = append(req.Items, canonical.Item{
				Kind: canonical.ItemFunctionResult,
				Role: canonical.RoleTool,
				Result: &canonical.FunctionResult{
					ItemID:       msg.ID,
					CallID:       msg.ToolCallID,
					Name:         fnName,
					Parts:        parts,
					IsStringForm: isStr,
					Cache:        msgCache,
				},
			})
			continue
		}

		if role == canonical.RoleAssistant {
			reasoningText := msg.ReasoningContent.toCanonical().ValueOr("")
			if reasoningText == "" {
				reasoningText = msg.Reasoning.toCanonical().ValueOr("")
			}
			if reasoningText != "" || msg.ReasoningSignature != "" || msg.ReasoningRedactedContent != "" {
				var rTexts []string
				if reasoningText != "" {
					rTexts = []string{reasoningText}
				}
				req.Items = append(req.Items, canonical.Item{
					Kind: canonical.ItemReasoning,
					Role: canonical.RoleAssistant,
					Reasoning: &canonical.ReasoningItem{
						Text:      rTexts,
						Signature: msg.ReasoningSignature,
						Redacted:  msg.ReasoningRedactedContent,
						Origin:    canonical.OpaqueOrigin{Protocol: canonical.ProtocolOpenAIChat},
					},
				})
			}

			tc := bytes.TrimSpace(msg.Content)
			hasContent := len(tc) > 0 && !bytes.Equal(tc, []byte("null"))
			refusalStr := msg.Refusal.toCanonical().ValueOr("")

			var msgParts []canonical.Part
			isArray := false
			emitMsgItem := false

			if refusalStr != "" && !hasContent {
				msgParts = []canonical.Part{{Kind: canonical.PartRefusal, Refusal: refusalStr}}
				emitMsgItem = true
			} else if hasContent {
				emitMsgItem = true
				switch tc[0] {
				case '"':
					var s string
					if err := json.Unmarshal(tc, &s); err != nil {
						return canonical.Request{}, canonical.NewRejectionError("invalid_message_content", "messages.content", "invalid assistant content string")
					}
					msgParts = []canonical.Part{{Kind: canonical.PartText, Text: s, Annotations: cloneRawSlice(msg.Annotations)}}
				case '[':
					isArray = true
					var wireParts []chatWireInContentPart
					if err := json.Unmarshal(tc, &wireParts); err != nil {
						return canonical.Request{}, canonical.NewRejectionError("invalid_message_content", "messages.content", "invalid assistant content array")
					}
					if len(wireParts) == 0 {
						msgParts = []canonical.Part{{Kind: canonical.PartText, Text: ""}}
						isArray = false
					} else {
						msgParts = make([]canonical.Part, 0, len(wireParts))
						for _, wp := range wireParts {
							pCache, err := parseChatCacheBoundary(wp.CacheControl, wp.PromptCacheBreakpoint, "messages.content")
							if err != nil {
								return canonical.Request{}, err
							}
							switch wp.Type {
							case "text":
								if !wp.Text.toCanonical().IsPresent() {
									return canonical.Request{}, canonical.NewRejectionError("invalid_message_content", "messages.content", "text content part requires string text")
								}
								msgParts = append(msgParts, canonical.Part{
									Kind:        canonical.PartText,
									Text:        wp.Text.val,
									Annotations: cloneRawSlice(wp.Annotations),
									Cache:       pCache,
								})
							case "refusal":
								if !wp.Refusal.toCanonical().IsPresent() {
									return canonical.Request{}, canonical.NewRejectionError("invalid_refusal_content", "refusal", "refusal part requires string refusal")
								}
								msgParts = append(msgParts, canonical.Part{
									Kind:    canonical.PartRefusal,
									Refusal: wp.Refusal.val,
									Cache:   pCache,
								})
							default:
								field := canonical.SafeIdentifier(wp.Type)
								if field == "" {
									field = "messages.content"
								}
								return canonical.Request{}, canonical.NewRejectionError("unsupported_content_part_type", field, fmt.Sprintf("unsupported assistant content part %q", wp.Type))
							}
						}
					}
				default:
					return canonical.Request{}, canonical.NewRejectionError("invalid_message_content", "messages.content", "assistant content must be string, array, or null")
				}
			} else if (len(msg.ToolCalls) == 0 && msg.FunctionCall == nil && reasoningText == "" && msg.ReasoningSignature == "" && msg.ReasoningRedactedContent == "") || msgCache != nil {
				emitMsgItem = true
			}

			partialOpt := msg.Partial.toCanonical()
			if emitMsgItem {
				req.Items = append(req.Items, canonical.Item{
					Kind:           canonical.ItemMessage,
					Role:           canonical.RoleAssistant,
					MessageID:      msg.ID,
					Parts:          msgParts,
					MessageIsArray: isArray,
					Partial:        partialOpt,
					Cache:          msgCache,
				})
			}

			calls := msg.ToolCalls
			if len(calls) == 0 && msg.FunctionCall != nil {
				calls = []chatWireInToolCall{{
					ID:       msg.FunctionCall.ID,
					Type:     "function",
					Function: *msg.FunctionCall,
				}}
			}
			for callIdx, call := range calls {
				if (call.Type != "" && call.Type != "function") || call.Function.Name == "" || call.ID == "" {
					return canonical.Request{}, canonical.NewRejectionError("invalid_tool_call", "tool_calls", "tool_call requires id and function.name")
				}
				sig := call.ThoughtSignature
				var sigOrigin canonical.OpaqueOrigin
				if sig == "" && call.ExtraContent != nil && call.ExtraContent.Google != nil && call.ExtraContent.Google.ThoughtSignature != "" {
					sig = call.ExtraContent.Google.ThoughtSignature
					sigOrigin = canonical.OpaqueOrigin{Protocol: canonical.ProtocolGeminiGenerate, Supplier: "google"}
				} else if sig != "" {
					sigOrigin = canonical.OpaqueOrigin{Protocol: canonical.ProtocolGeminiGenerate, Supplier: "google"}
				}
				toolNamesByCallID[call.ID] = call.Function.Name
				var callPartial canonical.Optional[bool]
				if !emitMsgItem && callIdx == 0 {
					callPartial = partialOpt
				}
				req.Items = append(req.Items, canonical.Item{
					Kind:    canonical.ItemFunctionCall,
					Role:    canonical.RoleAssistant,
					Partial: callPartial,
					Call: &canonical.FunctionCall{
						CallID:           call.ID,
						Name:             call.Function.Name,
						Args:             canonical.Arguments{Kind: canonical.ArgsText, Text: call.Function.Arguments},
						ThoughtSignature: sig,
						SignatureOrigin:  sigOrigin,
					},
				})
			}
			if !emitMsgItem && len(calls) == 0 && partialOpt.IsPresent() && len(req.Items) > 0 {
				req.Items[len(req.Items)-1].Partial = partialOpt
			}
			continue
		}

		// system, developer, user
		tc := bytes.TrimSpace(msg.Content)
		if len(tc) == 0 || bytes.Equal(tc, []byte("null")) {
			return canonical.Request{}, canonical.NewRejectionError("invalid_message_content", "messages.content", "message content is required")
		}
		var parts []canonical.Part
		isArray := false
		switch tc[0] {
		case '"':
			var s string
			if err := json.Unmarshal(tc, &s); err != nil {
				return canonical.Request{}, canonical.NewRejectionError("invalid_message_content", "messages.content", "invalid message content string")
			}
			parts = []canonical.Part{{Kind: canonical.PartText, Text: s}}
		case '[':
			isArray = true
			var wireParts []chatWireInContentPart
			if err := json.Unmarshal(tc, &wireParts); err != nil {
				return canonical.Request{}, canonical.NewRejectionError("invalid_message_content", "messages.content", "invalid message content array")
			}
			if len(wireParts) == 0 {
				return canonical.Request{}, canonical.NewRejectionError("invalid_message_content", "messages.content", "message content array must not be empty")
			}
			parts = make([]canonical.Part, 0, len(wireParts))
			for _, wp := range wireParts {
				pCache, err := parseChatCacheBoundary(wp.CacheControl, wp.PromptCacheBreakpoint, "messages.content")
				if err != nil {
					return canonical.Request{}, err
				}
				switch wp.Type {
				case "text":
					if !wp.Text.toCanonical().IsPresent() {
						return canonical.Request{}, canonical.NewRejectionError("invalid_message_content", "messages.content", "text content part requires string text")
					}
					parts = append(parts, canonical.Part{
						Kind:        canonical.PartText,
						Text:        wp.Text.val,
						Annotations: cloneRawSlice(wp.Annotations),
						Cache:       pCache,
					})
				case "image_url":
					if role != canonical.RoleUser {
						return canonical.Request{}, canonical.NewRejectionError("unsupported_image_content", "image_url", "image_url is supported only on user messages")
					}
					if wp.ImageURL == nil || wp.ImageURL.URL == "" {
						return canonical.Request{}, canonical.NewRejectionError("unsupported_image_content", "image_url", "image_url requires url")
					}
					imgPart, err := parseCanonicalImageURL(wp.ImageURL.URL, canonical.ImageDetail(wp.ImageURL.Detail), "image_url")
					if err != nil {
						return canonical.Request{}, err
					}
					parts = append(parts, canonical.Part{
						Kind:  canonical.PartImage,
						Image: imgPart,
						Cache: pCache,
					})
				case "file":
					if wp.File != nil && wp.File.FileID != "" {
						return canonical.Request{}, canonical.NewRejectionError("unsupported_content_part_type", "file", "file content part is unsupported across protocols")
					}
					return canonical.Request{}, canonical.NewRejectionError("unsupported_content_part_type", "file", "file content part is unsupported")
				default:
					field := canonical.SafeIdentifier(wp.Type)
					if field == "" {
						field = "messages.content"
					}
					return canonical.Request{}, canonical.NewRejectionError("unsupported_content_part_type", field, fmt.Sprintf("content part type %q is unsupported", wp.Type))
				}
			}
		default:
			return canonical.Request{}, canonical.NewRejectionError("invalid_message_content", "messages.content", "message content must be string or array")
		}

		req.Items = append(req.Items, canonical.Item{
			Kind:           canonical.ItemMessage,
			Role:           role,
			MessageID:      msg.ID,
			Parts:          parts,
			MessageIsArray: isArray,
			Partial:        msg.Partial.toCanonical(),
			Cache:          msgCache,
		})
	}

	return req, nil
}

// --- Outbound Chat Request Encoder ---

type chatWireOutStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatWireOutJSONSchema struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Schema      json.RawMessage `json:"schema,omitempty"`
	Strict      *bool           `json:"strict,omitempty"`
}

type chatWireOutResponseFormat struct {
	Type       string                 `json:"type"`
	JSONSchema *chatWireOutJSONSchema `json:"json_schema,omitempty"`
}

type chatWireOutFunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
	Strict      *bool           `json:"strict,omitempty"`
}

type chatWireOutTool struct {
	Type         string                  `json:"type"`
	Function     *chatWireOutFunctionDef `json:"function,omitempty"`
	CacheControl *chatWireCacheControl   `json:"cache_control,omitempty"`
}

type chatWireOutToolCall struct {
	ID           string                `json:"id"`
	Type         string                `json:"type"`
	Function     chatWireFunctionCall  `json:"function"`
	ExtraContent *chatWireExtraContent `json:"extra_content,omitempty"`
}

type chatWireOutContentPart struct {
	Type                  string                   `json:"type"`
	Text                  *string                  `json:"text,omitempty"`
	ImageURL              *chatWireImageURL        `json:"image_url,omitempty"`
	Refusal               *string                  `json:"refusal,omitempty"`
	CacheControl          *chatWireCacheControl    `json:"cache_control,omitempty"`
	PromptCacheBreakpoint *chatWireCacheBreakpoint `json:"prompt_cache_breakpoint,omitempty"`
}

type chatWireOutMessage struct {
	Role                  string                   `json:"role"`
	Name                  string                   `json:"name,omitempty"`
	Content               json.RawMessage          `json:"content,omitempty"`
	Partial               *bool                    `json:"partial,omitempty"`
	ReasoningContent      *string                  `json:"reasoning_content,omitempty"`
	ToolCalls             []chatWireOutToolCall    `json:"tool_calls,omitempty"`
	ToolCallID            string                   `json:"tool_call_id,omitempty"`
	CacheControl          *chatWireCacheControl    `json:"cache_control,omitempty"`
	PromptCacheBreakpoint *chatWireCacheBreakpoint `json:"prompt_cache_breakpoint,omitempty"`
}

type chatWireOutRequest struct {
	Model                 string                     `json:"model"`
	Messages              []chatWireOutMessage       `json:"messages"`
	Stream                bool                       `json:"stream,omitempty"`
	StreamOptions         *chatWireOutStreamOptions  `json:"stream_options,omitempty"`
	Temperature           *float64                   `json:"temperature,omitempty"`
	TopP                  *float64                   `json:"top_p,omitempty"`
	TopK                  *int                       `json:"top_k,omitempty"`
	PresencePenalty       *float64                   `json:"presence_penalty,omitempty"`
	FrequencyPenalty      *float64                   `json:"frequency_penalty,omitempty"`
	RepetitionPenalty     *float64                   `json:"repetition_penalty,omitempty"`
	Seed                  *int64                     `json:"seed,omitempty"`
	Stop                  []string                   `json:"stop,omitempty"`
	N                     *int                       `json:"n,omitempty"`
	Logprobs              *bool                      `json:"logprobs,omitempty"`
	TopLogprobs           *int                       `json:"top_logprobs,omitempty"`
	MaxTokens             *int                       `json:"max_tokens,omitempty"`
	MaxCompletionTokens   *int                       `json:"max_completion_tokens,omitempty"`
	ResponseFormat        *chatWireOutResponseFormat `json:"response_format,omitempty"`
	ReasoningEffort       string                     `json:"reasoning_effort,omitempty"`
	PreserveThinking      *bool                      `json:"preserve_thinking,omitempty"`
	Tools                 []json.RawMessage          `json:"tools,omitempty"`
	ToolChoice            json.RawMessage            `json:"tool_choice,omitempty"`
	ParallelToolCalls     *bool                      `json:"parallel_tool_calls,omitempty"`
	EnableSearch          *bool                      `json:"enable_search,omitempty"`
	EnableCodeInterpreter *bool                      `json:"enable_code_interpreter,omitempty"`
	SearchOptions         json.RawMessage            `json:"search_options,omitempty"`
	WebSearchOptions      json.RawMessage            `json:"web_search_options,omitempty"`
	Store                 *bool                      `json:"store,omitempty"`
	User                  string                     `json:"user,omitempty"`
	Metadata              map[string]string          `json:"metadata,omitempty"`
	PromptCacheKey        string                     `json:"prompt_cache_key,omitempty"`
	PromptCacheRetention  string                     `json:"prompt_cache_retention,omitempty"`
	PromptCacheOptions    json.RawMessage            `json:"prompt_cache_options,omitempty"`
}

type chatAssistantSegment struct {
	parts          []canonical.Part
	messageIsArray bool
	cache          *canonical.CacheBoundary
}

func encodeOuterMessageCacheMarkers(cb *canonical.CacheBoundary, target CanonicalTarget) (*chatWireCacheControl, *chatWireCacheBreakpoint) {
	if cb == nil || target.StripOuterCacheControl {
		return nil, nil
	}
	switch cb.Origin {
	case canonical.CacheOriginEphemeralControl:
		if target.SupportsContentCacheControl {
			mode := cb.Mode
			if mode == "" {
				mode = "ephemeral"
			}
			return &chatWireCacheControl{Type: mode, TTL: cb.TTL}, nil
		}
	case canonical.CacheOriginOpenAIPromptCache:
		if target.SupportsPromptCacheKey {
			return nil, &chatWireCacheBreakpoint{Mode: cb.Mode, TTL: cb.TTL}
		}
	}
	return nil, nil
}

func encodeChatPartsJSON(parts []canonical.Part, target CanonicalTarget, forceArray bool, joinSep string) (json.RawMessage, error) {
	hasRichOrCachedPart := forceArray
	var textBuilder strings.Builder
	outParts := make([]chatWireOutContentPart, 0, len(parts))

	for idx, p := range parts {
		var cc *chatWireCacheControl
		var bp *chatWireCacheBreakpoint
		if p.Cache != nil {
			switch p.Cache.Origin {
			case canonical.CacheOriginEphemeralControl:
				if target.SupportsContentCacheControl {
					mode := p.Cache.Mode
					if mode == "" {
						mode = "ephemeral"
					}
					cc = &chatWireCacheControl{Type: mode, TTL: p.Cache.TTL}
				}
			case canonical.CacheOriginOpenAIPromptCache:
				if target.SupportsPromptCacheKey && !target.StripOuterCacheControl {
					bp = &chatWireCacheBreakpoint{Mode: p.Cache.Mode, TTL: p.Cache.TTL}
				}
			}
		}
		if cc != nil || bp != nil {
			hasRichOrCachedPart = true
		}
		switch p.Kind {
		case canonical.PartText:
			if idx > 0 && joinSep != "" {
				textBuilder.WriteString(joinSep)
			}
			textBuilder.WriteString(p.Text)
			txt := p.Text
			outParts = append(outParts, chatWireOutContentPart{
				Type:                  "text",
				Text:                  &txt,
				CacheControl:          cc,
				PromptCacheBreakpoint: bp,
			})
		case canonical.PartImage:
			if p.Image == nil {
				return nil, canonical.NewRejectionError("invalid_image_content", "image_url", "image part is missing image metadata")
			}
			if p.Image.Source == canonical.ImageFileID || p.Image.FileID != "" {
				return nil, canonical.NewRejectionError("unsupported_image_content", "file_id", "image file_id cannot be resolved across protocols")
			}
			imgURL := p.Image.URL
			if imgURL == "" && p.Image.Source == canonical.ImageDataURL && p.Image.MediaType != "" && p.Image.Base64Ref != "" {
				imgURL = "data:" + p.Image.MediaType + ";base64," + p.Image.Base64Ref
			}
			if imgURL == "" {
				return nil, canonical.NewRejectionError("invalid_image_content", "image_url", "image URL is empty")
			}
			hasRichOrCachedPart = true
			outParts = append(outParts, chatWireOutContentPart{
				Type: "image_url",
				ImageURL: &chatWireImageURL{
					URL:    imgURL,
					Detail: string(p.Image.Detail),
				},
				CacheControl:          cc,
				PromptCacheBreakpoint: bp,
			})
		case canonical.PartRefusal:
			hasRichOrCachedPart = true
			ref := p.Refusal
			outParts = append(outParts, chatWireOutContentPart{
				Type:                  "refusal",
				Refusal:               &ref,
				CacheControl:          cc,
				PromptCacheBreakpoint: bp,
			})
		default:
			return nil, canonical.NewRejectionError("unsupported_content_part_type", "content", "unsupported canonical content part kind")
		}
	}

	if hasRichOrCachedPart {
		encoded, err := json.Marshal(outParts)
		if err != nil {
			return nil, fmt.Errorf("marshal Chat content parts: %w", err)
		}
		return encoded, nil
	}
	encoded, err := json.Marshal(textBuilder.String())
	if err != nil {
		return nil, fmt.Errorf("marshal Chat text content: %w", err)
	}
	return encoded, nil
}

// DashScope rejects output limits above this value for the qwen-plus stable model.
const qwenPlusMaxOutputTokens = 32768

func encodeChatCanonicalRequest(req canonical.Request, target CanonicalTarget) ([]byte, error) {
	model := target.Model
	if model == "" {
		model = req.Model
	}
	streaming := req.Stream || target.Streaming

	out := chatWireOutRequest{
		Model:  model,
		Stream: streaming,
	}
	if streaming {
		out.StreamOptions = &chatWireOutStreamOptions{IncludeUsage: true}
	}

	if req.Sampling.Temperature.IsPresent() {
		v := req.Sampling.Temperature.Val
		out.Temperature = &v
	}
	if req.Sampling.TopP.IsPresent() {
		v := req.Sampling.TopP.Val
		out.TopP = &v
	}
	if req.Sampling.TopK.IsPresent() {
		v := req.Sampling.TopK.Val
		out.TopK = &v
	}
	if req.Sampling.PresencePenalty.IsPresent() {
		v := req.Sampling.PresencePenalty.Val
		out.PresencePenalty = &v
	}
	if req.Sampling.FrequencyPenalty.IsPresent() {
		v := req.Sampling.FrequencyPenalty.Val
		out.FrequencyPenalty = &v
	}
	if req.Sampling.RepetitionPenalty.IsPresent() {
		v := req.Sampling.RepetitionPenalty.Val
		out.RepetitionPenalty = &v
	}
	if req.Sampling.Seed.IsPresent() {
		v := req.Sampling.Seed.Val
		out.Seed = &v
	}
	if req.Sampling.StopPresent && len(req.Sampling.Stop) > 0 {
		out.Stop = slices.Clone(req.Sampling.Stop)
	}
	if req.Sampling.N.IsPresent() {
		v := req.Sampling.N.Val
		out.N = &v
	}
	if req.Sampling.Logprobs.IsPresent() {
		v := req.Sampling.Logprobs.Val
		out.Logprobs = &v
	}
	if req.Sampling.TopLogprobs.IsPresent() && req.SourceProtocol == canonical.ProtocolOpenAIChat {
		v := req.Sampling.TopLogprobs.Val
		out.TopLogprobs = &v
	}

	if req.Limits.Value.IsPresent() {
		v := req.Limits.Value.Val
		if target.Supplier == providerTypeQwen && model == "qwen-plus" {
			v = min(v, qwenPlusMaxOutputTokens)
		}
		if req.Limits.Source == canonical.LimitMaxCompletionTokens || req.Limits.Source == canonical.LimitMaxOutputTokens || target.SupportsCompletionTokens {
			out.MaxCompletionTokens = &v
		} else {
			out.MaxTokens = &v
		}
	}

	switch req.ResponseFormat.Kind {
	case canonical.ResponseFormatText:
		out.ResponseFormat = &chatWireOutResponseFormat{Type: "text"}
	case canonical.ResponseFormatJSONObject:
		out.ResponseFormat = &chatWireOutResponseFormat{Type: "json_object"}
	case canonical.ResponseFormatJSONSchema:
		var strictPtr *bool
		if req.ResponseFormat.Strict.IsPresent() {
			v := req.ResponseFormat.Strict.Val
			strictPtr = &v
		}
		out.ResponseFormat = &chatWireOutResponseFormat{
			Type: "json_schema",
			JSONSchema: &chatWireOutJSONSchema{
				Name:        req.ResponseFormat.Name,
				Description: req.ResponseFormat.Description,
				Schema:      bytes.Clone(req.ResponseFormat.Schema),
				Strict:      strictPtr,
			},
		}
	}

	if req.Thinking.Effort.IsPresent() && req.Thinking.Effort.Val != "" {
		if req.Thinking.Mode != canonical.ThinkingDisabled || req.Thinking.Effort.Val == "none" {
			out.ReasoningEffort = req.Thinking.Effort.Val
		}
	} else {
		switch req.Thinking.Mode {
		case canonical.ThinkingEnabled:
			budget := req.Thinking.BudgetTokens.ValueOr(0)
			switch {
			case budget < 4096:
				out.ReasoningEffort = "low"
			case budget < 16384:
				out.ReasoningEffort = "medium"
			default:
				out.ReasoningEffort = "high"
			}
		case canonical.ThinkingAdaptive:
			out.ReasoningEffort = "medium"
		}
	}

	if target.SupportsStore && req.Store.IsPresent() {
		v := req.Store.Val
		out.Store = &v
	}
	if req.User.IsPresent() && req.User.Val != "" {
		out.User = req.User.Val
	}
	if req.Metadata.IsPresent() && len(req.Metadata.Val) > 0 && req.SourceProtocol != canonical.ProtocolAnthropicMessages {
		out.Metadata = req.Metadata.Val
	}

	if target.SupportsPromptCacheKey {
		if req.Cache.PromptCacheKey.IsPresent() && req.Cache.PromptCacheKey.Val != "" {
			out.PromptCacheKey = req.Cache.PromptCacheKey.Val
		}
		if req.Cache.PromptCacheRetention.IsPresent() && req.Cache.PromptCacheRetention.Val != "" {
			out.PromptCacheRetention = req.Cache.PromptCacheRetention.Val
		}
		if req.Cache.RequestBoundary != nil && req.Cache.RequestBoundary.Origin == canonical.CacheOriginOpenAIPromptCache {
			rawOpts, err := json.Marshal(chatWireCacheBreakpoint{
				Mode: req.Cache.RequestBoundary.Mode,
				TTL:  req.Cache.RequestBoundary.TTL,
			})
			if err != nil {
				return nil, fmt.Errorf("marshal prompt_cache_options: %w", err)
			}
			out.PromptCacheOptions = rawOpts
		}
	}

	if req.FixedTools.EnableSearch.IsPresent() {
		v := req.FixedTools.EnableSearch.Val
		out.EnableSearch = &v
	}
	if req.FixedTools.EnableCodeInterpreter.IsPresent() {
		v := req.FixedTools.EnableCodeInterpreter.Val
		out.EnableCodeInterpreter = &v
	}
	searchOptions, err := encodeFixedToolSearchOptions(req.FixedTools)
	if err != nil {
		return nil, fmt.Errorf("marshal search_options: %w", err)
	}
	out.SearchOptions = searchOptions
	if len(req.FixedTools.WebSearchOptionsRaw) > 0 {
		out.WebSearchOptions = bytes.Clone(req.FixedTools.WebSearchOptionsRaw)
	}

	if len(req.Tools) > 0 {
		out.Tools = make([]json.RawMessage, 0, len(req.Tools))
		for _, t := range req.Tools {
			switch t.Kind {
			case canonical.ToolKindFunction:
				var strictPtr *bool
				if t.Strict.IsPresent() {
					v := t.Strict.Val
					strictPtr = &v
				}
				var toolCC *chatWireCacheControl
				if t.Cache != nil && t.Cache.Origin == canonical.CacheOriginEphemeralControl && target.SupportsContentCacheControl && !target.StripOuterCacheControl {
					mode := t.Cache.Mode
					if mode == "" {
						mode = "ephemeral"
					}
					toolCC = &chatWireCacheControl{Type: mode, TTL: t.Cache.TTL}
				}
				encodedTool, err := json.Marshal(chatWireOutTool{
					Type: "function",
					Function: &chatWireOutFunctionDef{
						Name:        t.Name,
						Description: t.Description,
						Parameters:  bytes.Clone(t.Schema),
						Strict:      strictPtr,
					},
					CacheControl: toolCC,
				})
				if err != nil {
					return nil, fmt.Errorf("marshal Chat function tool: %w", err)
				}
				out.Tools = append(out.Tools, encodedTool)
			case canonical.ToolKindFixedServer:
				if len(t.RawFixed) > 0 {
					rawFixed := bytes.Clone(t.RawFixed)
					if target.StripOuterCacheControl || !target.SupportsContentCacheControl {
						var fixedMap map[string]json.RawMessage
						if json.Unmarshal(rawFixed, &fixedMap) == nil && fixedMap != nil {
							if _, hasCC := fixedMap["cache_control"]; hasCC {
								delete(fixedMap, "cache_control")
								if cleaned, err := json.Marshal(fixedMap); err == nil {
									rawFixed = cleaned
								}
							}
						}
					}
					out.Tools = append(out.Tools, rawFixed)
				} else if t.FixedType != "" {
					encodedFixed, err := json.Marshal(struct {
						Type string `json:"type"`
					}{Type: t.FixedType})
					if err != nil {
						return nil, fmt.Errorf("marshal Chat fixed tool: %w", err)
					}
					out.Tools = append(out.Tools, encodedFixed)
				}
			}
		}
	}

	switch req.ToolChoice.Mode {
	case canonical.ToolChoiceUnset:
	case canonical.ToolChoiceAuto:
		out.ToolChoice = json.RawMessage(`"auto"`)
	case canonical.ToolChoiceNone:
		out.ToolChoice = json.RawMessage(`"none"`)
	case canonical.ToolChoiceRequired, canonical.ToolChoiceAny:
		out.ToolChoice = json.RawMessage(`"required"`)
	case canonical.ToolChoiceFunction:
		encodedChoice, err := json.Marshal(struct {
			Type     string `json:"type"`
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		}{
			Type: "function",
			Function: struct {
				Name string `json:"name"`
			}{Name: req.ToolChoice.Name},
		})
		if err != nil {
			return nil, fmt.Errorf("marshal Chat tool_choice: %w", err)
		}
		out.ToolChoice = encodedChoice
	case canonical.ToolChoiceFixed:
		fixedName := req.ToolChoice.FixedRaw
		if fixedName == "" {
			fixedName = req.ToolChoice.Name
		}
		encodedChoice, err := json.Marshal(struct {
			Type string `json:"type"`
		}{Type: fixedName})
		if err != nil {
			return nil, fmt.Errorf("marshal Chat fixed tool_choice: %w", err)
		}
		out.ToolChoice = encodedChoice
	}

	if req.ParallelToolCalls.IsPresent() {
		v := req.ParallelToolCalls.Val
		out.ParallelToolCalls = &v
	}

	out.Messages = make([]chatWireOutMessage, 0, len(req.Items)+2)

	// Prepend Instructions (OA01) and SystemParts (AN02) while preserving source separation in IR.
	if req.Instructions.IsPresent() && req.Instructions.Val != "" {
		instrRole := roleSystem
		if target.SupportsDeveloperRole {
			instrRole = roleDeveloper
		}
		instrJSON, err := json.Marshal(req.Instructions.Val)
		if err != nil {
			return nil, fmt.Errorf("marshal instructions: %w", err)
		}
		out.Messages = append(out.Messages, chatWireOutMessage{
			Role:    instrRole,
			Content: instrJSON,
		})
	}
	if len(req.SystemParts) > 0 {
		sysContent, err := encodeChatPartsJSON(req.SystemParts, target, req.SystemIsArray, "")
		if err != nil {
			return nil, err
		}
		out.Messages = append(out.Messages, chatWireOutMessage{
			Role:    roleSystem,
			Content: sysContent,
		})
	}

	allowMessageReasoning := req.SourceProtocol != canonical.ProtocolAnthropicMessages ||
		target.SupportsThinking ||
		target.SupportsPreserveThinking ||
		target.TopLevelPreserveThinking ||
		(target.Supplier != "openai" && target.Supplier != "azure")
	hasEmittedMessageReasoning := false

	for i := 0; i < len(req.Items); {
		item := req.Items[i]

		// Assistant turn consolidation across contiguous reasoning, assistant message, and function_call items.
		isAssistantItem := item.Kind == canonical.ItemReasoning ||
			item.Kind == canonical.ItemFunctionCall ||
			(item.Kind == canonical.ItemMessage && item.Role == canonical.RoleAssistant)
		if isAssistantItem {
			var reasoningChunks []string
			hadMessagesReasoning := false
			var segments []chatAssistantSegment
			hasBareEmptyAssistant := false
			var bareEmptyCache *canonical.CacheBoundary
			var partialOpt canonical.Optional[bool]
			var toolCalls []chatWireOutToolCall

			for i < len(req.Items) {
				cur := req.Items[i]
				if cur.Kind == canonical.ItemReasoning {
					if cur.Partial.IsPresent() {
						partialOpt = cur.Partial
					}
					if cur.Reasoning != nil {
						if cur.Reasoning.Cache != nil {
							return nil, canonical.NewRejectionError("invalid_cache_control", "thinking", "thinking blocks cannot carry cache_control")
						}
						if cur.Reasoning.Origin.Protocol == canonical.ProtocolAnthropicMessages {
							hadMessagesReasoning = true
						}
						if allowMessageReasoning && len(cur.Reasoning.Text) > 0 {
							reasoningChunks = append(reasoningChunks, cur.Reasoning.Text...)
						}
					}
					i++
					continue
				}
				if cur.Kind == canonical.ItemFunctionCall {
					if cur.Partial.IsPresent() {
						partialOpt = cur.Partial
					}
					if cur.Call != nil {
						var argsStr string
						switch cur.Call.Args.Kind {
						case canonical.ArgsText:
							argsStr = cur.Call.Args.Text
						case canonical.ArgsObject:
							trimmedObj := bytes.TrimSpace(cur.Call.Args.Object)
							if len(trimmedObj) == 0 {
								argsStr = "{}"
							} else {
								var compactBuf bytes.Buffer
								if err := json.Compact(&compactBuf, trimmedObj); err == nil {
									argsStr = compactBuf.String()
								} else {
									argsStr = string(trimmedObj)
								}
							}
						default:
							argsStr = "{}"
						}
						var extra *chatWireExtraContent
						if cur.Call.ThoughtSignature != "" && !target.StripOuterCacheControl {
							extra = &chatWireExtraContent{
								Google: &chatWireGoogleExtra{ThoughtSignature: cur.Call.ThoughtSignature},
							}
						}
						toolCalls = append(toolCalls, chatWireOutToolCall{
							ID:   cur.Call.CallID,
							Type: "function",
							Function: chatWireFunctionCall{
								Name:      cur.Call.Name,
								Arguments: argsStr,
							},
							ExtraContent: extra,
						})
					}
					i++
					continue
				}
				if cur.Kind == canonical.ItemMessage && cur.Role == canonical.RoleAssistant {
					if len(segments) > 0 && len(toolCalls) == 0 {
						break
					}
					if cur.Partial.IsPresent() {
						partialOpt = cur.Partial
					}
					if len(cur.Parts) == 0 {
						hasBareEmptyAssistant = true
						if cur.Cache != nil {
							bareEmptyCache = cur.Cache
						}
					} else {
						segments = append(segments, chatAssistantSegment{
							parts:          cur.Parts,
							messageIsArray: cur.MessageIsArray,
							cache:          cur.Cache,
						})
					}
					i++
					continue
				}
				break
			}

			var partialPtr *bool
			if partialOpt.IsPresent() {
				pv := partialOpt.Val
				partialPtr = &pv
			}

			var reasoningPtr *string
			if len(reasoningChunks) > 0 {
				sep := ""
				if req.SourceProtocol == canonical.ProtocolAnthropicMessages {
					sep = "\n\n"
				}
				joined := strings.Join(reasoningChunks, sep)
				if joined != "" {
					reasoningPtr = &joined
					hasEmittedMessageReasoning = true
				}
			}

			if len(segments) == 0 && len(toolCalls) == 0 && reasoningPtr == nil {
				if hadMessagesReasoning || hasBareEmptyAssistant || partialPtr != nil {
					cc, bp := encodeOuterMessageCacheMarkers(bareEmptyCache, target)
					out.Messages = append(out.Messages, chatWireOutMessage{
						Role:                  roleAssistant,
						Content:               json.RawMessage(`""`),
						Partial:               partialPtr,
						CacheControl:          cc,
						PromptCacheBreakpoint: bp,
					})
				}
				continue
			}

			outMsg := chatWireOutMessage{
				Role:             roleAssistant,
				Partial:          partialPtr,
				ReasoningContent: reasoningPtr,
				ToolCalls:        toolCalls,
			}

			if len(segments) == 0 {
				cc, bp := encodeOuterMessageCacheMarkers(bareEmptyCache, target)
				outMsg.CacheControl = cc
				outMsg.PromptCacheBreakpoint = bp
				if target.NormalizeEmptyAssistantContent || (len(toolCalls) == 0 && reasoningPtr == nil) {
					outMsg.Content = json.RawMessage(`""`)
				}
			} else if len(segments) == 1 {
				seg := segments[0]
				cc, bp := encodeOuterMessageCacheMarkers(seg.cache, target)
				outMsg.CacheControl = cc
				outMsg.PromptCacheBreakpoint = bp
				forceArr := req.SourceProtocol != canonical.ProtocolOpenAIResponses && seg.messageIsArray && len(toolCalls) == 0
				joinSep := ""
				if req.SourceProtocol == canonical.ProtocolAnthropicMessages && len(toolCalls) > 0 {
					joinSep = "\n\n"
				}
				encodedContent, err := encodeChatPartsJSON(seg.parts, target, forceArr, joinSep)
				if err != nil {
					return nil, err
				}
				outMsg.Content = encodedContent
			} else {
				// Consolidate multiple assistant segments within a tool-calling batch,
				// transferring segment-level cache boundaries onto each segment's final part.
				var mergedParts []canonical.Part
				for _, seg := range segments {
					for _, p := range seg.parts {
						if p.Kind == canonical.PartRefusal {
							return nil, canonical.NewRejectionError("incompatible_refusal_merge", "input.content", "Chat cannot combine refusal and other assistant content parts")
						}
					}
					segParts := make([]canonical.Part, len(seg.parts))
					copy(segParts, seg.parts)
					if seg.cache != nil && len(segParts) > 0 {
						end := &segParts[len(segParts)-1]
						if end.Cache != nil && *end.Cache != *seg.cache {
							field := "cache_control"
							if seg.cache.Origin == canonical.CacheOriginOpenAIPromptCache {
								field = "prompt_cache_breakpoint"
							}
							return nil, canonical.NewRejectionError("conflicting_cache_breakpoints", field, "conflicting cache breakpoints at assistant content boundary")
						}
						cbCopy := *seg.cache
						end.Cache = &cbCopy
					}
					mergedParts = append(mergedParts, segParts...)
				}
				encodedContent, err := encodeChatPartsJSON(mergedParts, target, true, "")
				if err != nil {
					return nil, err
				}
				outMsg.Content = encodedContent
			}

			out.Messages = append(out.Messages, outMsg)
			continue
		}

		// Tool result batch (including Messages multimodal tool_result companion user message).
		if item.Kind == canonical.ItemFunctionResult {
			var companionImages []canonical.Part
			for i < len(req.Items) && req.Items[i].Kind == canonical.ItemFunctionResult {
				res := req.Items[i].Result
				i++
				if res == nil {
					continue
				}
				var textOnlyParts []canonical.Part
				hasImages := false
				for _, p := range res.Parts {
					if p.Kind == canonical.PartImage {
						hasImages = true
						companionImages = append(companionImages, p)
					} else {
						textOnlyParts = append(textOnlyParts, p)
					}
				}

				var toolContent json.RawMessage
				var err error
				if hasImages {
					var texts []string
					for _, tp := range textOnlyParts {
						if tp.Text != "" {
							texts = append(texts, tp.Text)
						}
					}
					toolContent, err = json.Marshal(strings.Join(texts, "\n"))
					if err != nil {
						return nil, fmt.Errorf("marshal tool result text: %w", err)
					}
				} else if res.IsStringForm {
					textVal := ""
					if len(textOnlyParts) == 1 {
						textVal = textOnlyParts[0].Text
					} else if len(textOnlyParts) > 1 {
						var b strings.Builder
						for _, tp := range textOnlyParts {
							b.WriteString(tp.Text)
						}
						textVal = b.String()
					}
					if res.Cache != nil && res.Cache.Origin == canonical.CacheOriginEphemeralControl && target.SupportsContentCacheControl && target.StripOuterCacheControl {
						toolContent, err = encodeChatPartsJSON([]canonical.Part{{
							Kind:  canonical.PartText,
							Text:  textVal,
							Cache: res.Cache,
						}}, target, true, "")
					} else {
						toolContent, err = json.Marshal(textVal)
					}
					if err != nil {
						return nil, fmt.Errorf("marshal tool result string: %w", err)
					}
				} else {
					preserveToolArray := req.SourceProtocol == canonical.ProtocolOpenAIResponses || req.SourceProtocol == canonical.ProtocolOpenAIChat
					toolContent, err = encodeChatPartsJSON(textOnlyParts, target, preserveToolArray, "\n")
					if err != nil {
						return nil, err
					}
				}

				toolName := ""
				if !target.StripToolMessageName && res.Name != "" && req.SourceProtocol == canonical.ProtocolOpenAIChat {
					toolName = res.Name
				}
				cc, bp := encodeOuterMessageCacheMarkers(res.Cache, target)
				out.Messages = append(out.Messages, chatWireOutMessage{
					Role:                  roleTool,
					Name:                  toolName,
					Content:               toolContent,
					ToolCallID:            res.CallID,
					CacheControl:          cc,
					PromptCacheBreakpoint: bp,
				})
			}

			if len(companionImages) > 0 {
				var companionParts []canonical.Part
				if i < len(req.Items) && req.Items[i].Kind == canonical.ItemMessage && req.Items[i].Role == canonical.RoleUser &&
					req.SourceProtocol == canonical.ProtocolAnthropicMessages {
					nextUser := req.Items[i]
					i++
					hasUserText := false
					for _, p := range nextUser.Parts {
						if p.Kind == canonical.PartText && p.Text != "" {
							hasUserText = true
							break
						}
					}
					if !hasUserText {
						companionParts = append(companionParts, canonical.Part{
							Kind: canonical.PartText,
							Text: "Attached image(s) from tool result:",
						})
					}
					companionParts = append(companionParts, nextUser.Parts...)
					companionParts = append(companionParts, companionImages...)
				} else {
					companionParts = make([]canonical.Part, 0, 1+len(companionImages))
					companionParts = append(companionParts, canonical.Part{
						Kind: canonical.PartText,
						Text: "Attached image(s) from tool result:",
					})
					companionParts = append(companionParts, companionImages...)
				}
				companionJSON, err := encodeChatPartsJSON(companionParts, target, true, "")
				if err != nil {
					return nil, err
				}
				out.Messages = append(out.Messages, chatWireOutMessage{
					Role:    roleUser,
					Content: companionJSON,
				})
			}
			continue
		}

		// system, developer, user message items
		outRole := string(item.Role)
		if item.Role == canonical.RoleDeveloper && !target.SupportsDeveloperRole {
			outRole = roleSystem
		}
		forceArr := req.SourceProtocol != canonical.ProtocolOpenAIResponses && item.MessageIsArray
		contentJSON, err := encodeChatPartsJSON(item.Parts, target, forceArr, "")
		if err != nil {
			return nil, err
		}
		var msgPartialPtr *bool
		if item.Partial.IsPresent() {
			pv := item.Partial.Val
			msgPartialPtr = &pv
		}
		cc, bp := encodeOuterMessageCacheMarkers(item.Cache, target)
		out.Messages = append(out.Messages, chatWireOutMessage{
			Role:                  outRole,
			Content:               contentJSON,
			Partial:               msgPartialPtr,
			CacheControl:          cc,
			PromptCacheBreakpoint: bp,
		})
		i++
	}

	// DS06 / S-BOUND-04: top-level preserve_thinking on Qwen compatible targets.
	if req.Thinking.PreserveThinking.IsPresent() {
		if target.TopLevelPreserveThinking || target.SupportsPreserveThinking {
			v := req.Thinking.PreserveThinking.Val
			out.PreserveThinking = &v
		}
	} else if target.TopLevelPreserveThinking && hasEmittedMessageReasoning && req.Thinking.Mode != canonical.ThinkingDisabled {
		v := true
		out.PreserveThinking = &v
	}

	encoded, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("marshal canonical Chat request: %w", err)
	}
	return encoded, nil
}

// --- Chat Response & SSE Stream Decoders ---

type chatWireRespChoice struct {
	Index        int                `json:"index"`
	Message      *chatWireInMessage `json:"message"`
	Delta        *chatWireInMessage `json:"delta"`
	FinishReason *string            `json:"finish_reason"`
	Logprobs     json.RawMessage    `json:"logprobs"`
}

type chatWireRespEnvelope struct {
	ID          string               `json:"id"`
	Created     int64                `json:"created"`
	Model       string               `json:"model"`
	ServiceTier string               `json:"service_tier"`
	Choices     []chatWireRespChoice `json:"choices"`
	Error       json.RawMessage      `json:"error"`
}

func classifyChatFinishReason(raw string) canonical.FinishReason {
	normalized := strings.TrimSpace(raw)
	if normalized == "" || strings.EqualFold(normalized, "null") {
		return canonical.FinishReason{Category: canonical.FinishNone}
	}
	switch normalized {
	case "stop":
		return canonical.FinishReason{Category: canonical.FinishStop, RawReason: normalized}
	case "tool_calls", "function_call":
		return canonical.FinishReason{Category: canonical.FinishToolCalls, RawReason: normalized}
	case "length":
		return canonical.FinishReason{Category: canonical.FinishLength, RawReason: normalized}
	case "content_filter", "sensitive":
		return canonical.FinishReason{Category: canonical.FinishContentFilter, RawReason: normalized}
	case "refusal":
		return canonical.FinishReason{Category: canonical.FinishRefusal, RawReason: normalized}
	case "pause_turn":
		return canonical.FinishReason{Category: canonical.FinishPauseTurn, RawReason: normalized}
	case "model_context_window_exceeded":
		return canonical.FinishReason{Category: canonical.FinishContextWindow, RawReason: normalized}
	case "aborted":
		return canonical.FinishReason{Category: canonical.FinishAborted, RawReason: normalized}
	case "insufficient_system_resource", "overloaded":
		return canonical.FinishReason{Category: canonical.FinishOverloaded, RawReason: normalized}
	default:
		return canonical.FinishReason{Category: canonical.FinishUnknown, RawReason: normalized}
	}
}

func cloneRawSlice(in []json.RawMessage) []json.RawMessage {
	if len(in) == 0 {
		return nil
	}
	out := make([]json.RawMessage, len(in))
	for i, raw := range in {
		out[i] = bytes.Clone(raw)
	}
	return out
}

func decodeChatCanonicalResponse(body []byte) (canonical.Response, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return canonical.Response{}, fmt.Errorf("invalid Chat completion response JSON")
	}

	var env chatWireRespEnvelope
	if err := json.Unmarshal(trimmed, &env); err != nil {
		return canonical.Response{}, fmt.Errorf("unmarshal Chat completion response: %w", err)
	}

	if errRaw := bytes.TrimSpace(env.Error); len(errRaw) > 0 && !bytes.Equal(errRaw, []byte("null")) && len(env.Choices) == 0 {
		return canonical.Response{}, fmt.Errorf("upstream Chat completion error payload")
	}

	resp := canonical.Response{
		ID:          env.ID,
		CreatedAt:   env.Created,
		Model:       env.Model,
		ServiceTier: env.ServiceTier,
		Choices:     make([]canonical.Choice, 0, len(env.Choices)),
	}

	for _, wc := range env.Choices {
		var finish canonical.FinishReason
		if wc.FinishReason != nil {
			finish = classifyChatFinishReason(*wc.FinishReason)
		}
		choice := canonical.Choice{
			Index:    wc.Index,
			Finish:   finish,
			Logprobs: bytes.Clone(wc.Logprobs),
		}
		msg := wc.Message
		if msg == nil {
			resp.Choices = append(resp.Choices, choice)
			continue
		}

		role := canonical.RoleAssistant
		if msg.Role != "" {
			role = canonical.Role(msg.Role)
		}

		reasoningText := msg.ReasoningContent.toCanonical().ValueOr("")
		if reasoningText == "" {
			reasoningText = msg.Reasoning.toCanonical().ValueOr("")
		}
		hasReasoning := reasoningText != "" || msg.ReasoningSignature != "" || msg.ReasoningRedactedContent != ""
		if hasReasoning {
			var rTexts []string
			if reasoningText != "" {
				rTexts = []string{reasoningText}
			}
			choice.Items = append(choice.Items, canonical.Item{
				Kind: canonical.ItemReasoning,
				Role: role,
				Reasoning: &canonical.ReasoningItem{
					Text:      rTexts,
					Signature: msg.ReasoningSignature,
					Redacted:  msg.ReasoningRedactedContent,
					Origin:    canonical.OpaqueOrigin{Protocol: canonical.ProtocolOpenAIChat},
				},
			})
		}

		calls := msg.ToolCalls
		if len(calls) == 0 && msg.FunctionCall != nil && (msg.FunctionCall.Name != "" || msg.FunctionCall.Arguments != "") {
			calls = []chatWireInToolCall{{
				ID:       msg.FunctionCall.ID,
				Type:     "function",
				Function: *msg.FunctionCall,
			}}
		}

		refusalText := msg.Refusal.toCanonical().ValueOr("")
		tc := bytes.TrimSpace(msg.Content)
		hasNonNullContent := len(tc) > 0 && !bytes.Equal(tc, []byte("null"))

		if refusalText != "" {
			choice.Items = append(choice.Items, canonical.Item{
				Kind:      canonical.ItemMessage,
				Role:      role,
				MessageID: msg.ID,
				Parts:     []canonical.Part{{Kind: canonical.PartRefusal, Refusal: refusalText}},
			})
		} else if hasNonNullContent {
			switch tc[0] {
			case '"':
				var s string
				if err := json.Unmarshal(tc, &s); err != nil {
					return canonical.Response{}, fmt.Errorf("unmarshal Chat response message content: %w", err)
				}
				if s != "" || (len(calls) == 0 && !hasReasoning) {
					choice.Items = append(choice.Items, canonical.Item{
						Kind:      canonical.ItemMessage,
						Role:      role,
						MessageID: msg.ID,
						Parts: []canonical.Part{{
							Kind:        canonical.PartText,
							Text:        s,
							Annotations: cloneRawSlice(msg.Annotations),
						}},
					})
				}
			case '[':
				var wireParts []chatWireInContentPart
				if err := json.Unmarshal(tc, &wireParts); err != nil {
					return canonical.Response{}, fmt.Errorf("unmarshal Chat response message content parts: %w", err)
				}
				parts := make([]canonical.Part, 0, len(wireParts))
				for _, wp := range wireParts {
					switch wp.Type {
					case "text":
						if wp.Text.toCanonical().IsPresent() {
							parts = append(parts, canonical.Part{
								Kind:        canonical.PartText,
								Text:        wp.Text.val,
								Annotations: cloneRawSlice(wp.Annotations),
							})
						}
					case "refusal":
						if wp.Refusal.toCanonical().IsPresent() {
							parts = append(parts, canonical.Part{
								Kind:    canonical.PartRefusal,
								Refusal: wp.Refusal.val,
							})
						}
					}
				}
				if len(parts) > 0 {
					choice.Items = append(choice.Items, canonical.Item{
						Kind:           canonical.ItemMessage,
						Role:           role,
						MessageID:      msg.ID,
						Parts:          parts,
						MessageIsArray: true,
					})
				}
			}
		}

		for _, call := range calls {
			sig := call.ThoughtSignature
			var sigOrigin canonical.OpaqueOrigin
			if sig == "" && call.ExtraContent != nil && call.ExtraContent.Google != nil && call.ExtraContent.Google.ThoughtSignature != "" {
				sig = call.ExtraContent.Google.ThoughtSignature
				sigOrigin = canonical.OpaqueOrigin{Protocol: canonical.ProtocolGeminiGenerate, Supplier: "google"}
			} else if sig != "" {
				sigOrigin = canonical.OpaqueOrigin{Protocol: canonical.ProtocolGeminiGenerate, Supplier: "google"}
			}
			choice.Items = append(choice.Items, canonical.Item{
				Kind: canonical.ItemFunctionCall,
				Role: role,
				Call: &canonical.FunctionCall{
					CallID:           call.ID,
					Name:             call.Function.Name,
					Args:             canonical.Arguments{Kind: canonical.ArgsText, Text: call.Function.Arguments},
					ThoughtSignature: sig,
					SignatureOrigin:  sigOrigin,
				},
			})
		}

		resp.Choices = append(resp.Choices, choice)
	}

	return resp, nil
}

type chatStreamToolState struct {
	itemIndex        int
	callID           string
	name             string
	thoughtSignature string
	sigOrigin        canonical.OpaqueOrigin
	started          bool
	pendingArgs      strings.Builder
}

type chatStreamChoiceState struct {
	role             canonical.Role
	nextItemIndex    int
	hasReasoningItem bool
	reasoningItemIdx int
	hasRefusalItem   bool
	refusalItemIdx   int
	hasMessageItem   bool
	messageItemIdx   int
	tools            map[int]*chatStreamToolState
}

func (cs *chatStreamChoiceState) allocItemIndex() int {
	idx := cs.nextItemIndex
	cs.nextItemIndex++
	return idx
}

type chatCanonicalEventDecoder struct {
	started     bool
	done        bool
	id          string
	createdAt   int64
	model       string
	serviceTier string
	choices     map[int]*chatStreamChoiceState
}

func newChatCanonicalEventDecoder() CanonicalEventDecoder {
	return &chatCanonicalEventDecoder{
		choices: make(map[int]*chatStreamChoiceState),
	}
}

func (d *chatCanonicalEventDecoder) flushPendingTools(choiceIdx int, cs *chatStreamChoiceState, emit func(canonical.Event) error) error {
	if len(cs.tools) == 0 {
		return nil
	}
	indices := make([]int, 0, len(cs.tools))
	for tIdx, ts := range cs.tools {
		if !ts.started && (ts.callID != "" || ts.name != "" || ts.pendingArgs.Len() > 0) {
			indices = append(indices, tIdx)
		}
	}
	slices.Sort(indices)
	for _, tIdx := range indices {
		ts := cs.tools[tIdx]
		ts.started = true
		if err := emit(canonical.Event{
			Kind:             canonical.EventToolCallStart,
			ChoiceIndex:      choiceIdx,
			ItemIndex:        ts.itemIndex,
			ToolIndex:        tIdx,
			CallID:           ts.callID,
			ID:               d.id,
			Model:            d.model,
			CreatedAt:        d.createdAt,
			ServiceTier:      d.serviceTier,
			Role:             cs.role,
			ToolName:         ts.name,
			ThoughtSignature: ts.thoughtSignature,
			SignatureOrigin:  ts.sigOrigin,
		}); err != nil {
			return err
		}
		if ts.pendingArgs.Len() > 0 {
			buffered := ts.pendingArgs.String()
			ts.pendingArgs.Reset()
			if err := emit(canonical.Event{
				Kind:             canonical.EventToolCallArgsDelta,
				ChoiceIndex:      choiceIdx,
				ItemIndex:        ts.itemIndex,
				ToolIndex:        tIdx,
				CallID:           ts.callID,
				ID:               d.id,
				Model:            d.model,
				CreatedAt:        d.createdAt,
				ServiceTier:      d.serviceTier,
				Role:             cs.role,
				ToolName:         ts.name,
				Text:             buffered,
				ThoughtSignature: ts.thoughtSignature,
				SignatureOrigin:  ts.sigOrigin,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *chatCanonicalEventDecoder) Decode(name string, payload []byte, emit func(canonical.Event) error) error {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		if name == "ping" {
			return emit(canonical.Event{Kind: canonical.EventPing})
		}
		return nil
	}
	if bytes.Equal(trimmed, []byte(streamEndDataValue)) {
		for cIdx, cs := range d.choices {
			if err := d.flushPendingTools(cIdx, cs, emit); err != nil {
				return err
			}
		}
		d.done = true
		return emit(canonical.Event{
			Kind:        canonical.EventProtocolDone,
			ID:          d.id,
			Model:       d.model,
			CreatedAt:   d.createdAt,
			ServiceTier: d.serviceTier,
		})
	}

	var chunk chatWireRespEnvelope
	if err := json.Unmarshal(trimmed, &chunk); err != nil {
		if name == "error" {
			return emit(canonical.Event{
				Kind:        canonical.EventProviderError,
				ID:          d.id,
				Model:       d.model,
				CreatedAt:   d.createdAt,
				ServiceTier: d.serviceTier,
				RawError:    bytes.Clone(trimmed),
			})
		}
		return fmt.Errorf("unmarshal Chat stream chunk: %w", err)
	}

	if chunk.ID != "" {
		d.id = chunk.ID
	}
	if chunk.Created != 0 {
		d.createdAt = chunk.Created
	}
	if chunk.Model != "" {
		d.model = chunk.Model
	}
	if chunk.ServiceTier != "" {
		d.serviceTier = chunk.ServiceTier
	}

	errTrimmed := bytes.TrimSpace(chunk.Error)
	if name == "error" || (len(errTrimmed) > 0 && !bytes.Equal(errTrimmed, []byte("null"))) {
		rawErr := errTrimmed
		if len(rawErr) == 0 {
			rawErr = trimmed
		}
		return emit(canonical.Event{
			Kind:        canonical.EventProviderError,
			ID:          d.id,
			Model:       d.model,
			CreatedAt:   d.createdAt,
			ServiceTier: d.serviceTier,
			RawError:    bytes.Clone(rawErr),
		})
	}

	if !d.started {
		d.started = true
		startRole := canonical.RoleAssistant
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil && chunk.Choices[0].Delta.Role != "" {
			startRole = canonical.Role(chunk.Choices[0].Delta.Role)
		}
		if err := emit(canonical.Event{
			Kind:        canonical.EventResponseStart,
			ID:          d.id,
			Model:       d.model,
			CreatedAt:   d.createdAt,
			ServiceTier: d.serviceTier,
			Role:        startRole,
		}); err != nil {
			return err
		}
	}

	// Usage-only terminal chunks carry choices: [] and emit no content/finish events.
	if len(chunk.Choices) == 0 {
		return nil
	}

	for _, wc := range chunk.Choices {
		cIdx := wc.Index
		cs, exists := d.choices[cIdx]
		if !exists {
			cs = &chatStreamChoiceState{
				role:  canonical.RoleAssistant,
				tools: make(map[int]*chatStreamToolState),
			}
			d.choices[cIdx] = cs
		}

		if wc.Delta != nil {
			delta := wc.Delta
			if delta.Role != "" {
				cs.role = canonical.Role(delta.Role)
			}

			reasoningText := delta.ReasoningContent.toCanonical().ValueOr("")
			if reasoningText == "" {
				reasoningText = delta.Reasoning.toCanonical().ValueOr("")
			}
			if reasoningText != "" {
				if !cs.hasReasoningItem {
					cs.hasReasoningItem = true
					cs.reasoningItemIdx = cs.allocItemIndex()
				}
				if err := emit(canonical.Event{
					Kind:        canonical.EventReasoningDelta,
					ChoiceIndex: cIdx,
					ItemIndex:   cs.reasoningItemIdx,
					PartIndex:   0,
					ID:          d.id,
					Model:       d.model,
					CreatedAt:   d.createdAt,
					ServiceTier: d.serviceTier,
					Role:        cs.role,
					Text:        reasoningText,
				}); err != nil {
					return err
				}
			}

			if delta.ReasoningSignature != "" {
				if !cs.hasReasoningItem {
					cs.hasReasoningItem = true
					cs.reasoningItemIdx = cs.allocItemIndex()
				}
				if err := emit(canonical.Event{
					Kind:        canonical.EventReasoningSignatureDelta,
					ChoiceIndex: cIdx,
					ItemIndex:   cs.reasoningItemIdx,
					ID:          d.id,
					Model:       d.model,
					CreatedAt:   d.createdAt,
					ServiceTier: d.serviceTier,
					Role:        cs.role,
					Text:        delta.ReasoningSignature,
				}); err != nil {
					return err
				}
			}

			if delta.ReasoningRedactedContent != "" {
				redactedItemIdx := cs.allocItemIndex()
				if err := emit(canonical.Event{
					Kind:        canonical.EventReasoningRedacted,
					ChoiceIndex: cIdx,
					ItemIndex:   redactedItemIdx,
					ID:          d.id,
					Model:       d.model,
					CreatedAt:   d.createdAt,
					ServiceTier: d.serviceTier,
					Role:        cs.role,
					Text:        delta.ReasoningRedactedContent,
				}); err != nil {
					return err
				}
			}

			if refusalDelta := delta.Refusal.toCanonical().ValueOr(""); refusalDelta != "" {
				if !cs.hasRefusalItem {
					cs.hasRefusalItem = true
					cs.refusalItemIdx = cs.allocItemIndex()
				}
				if err := emit(canonical.Event{
					Kind:        canonical.EventRefusalDelta,
					ChoiceIndex: cIdx,
					ItemIndex:   cs.refusalItemIdx,
					PartIndex:   0,
					ItemID:      delta.ID,
					ID:          d.id,
					Model:       d.model,
					CreatedAt:   d.createdAt,
					ServiceTier: d.serviceTier,
					Role:        cs.role,
					Text:        refusalDelta,
				}); err != nil {
					return err
				}
			}

			tc := bytes.TrimSpace(delta.Content)
			var deltaText string
			if len(tc) > 0 && tc[0] == '"' {
				if err := json.Unmarshal(tc, &deltaText); err != nil {
					return fmt.Errorf("unmarshal Chat delta content: %w", err)
				}
			}
			if deltaText != "" || len(delta.Annotations) > 0 {
				if !cs.hasMessageItem {
					cs.hasMessageItem = true
					cs.messageItemIdx = cs.allocItemIndex()
				}
				if err := emit(canonical.Event{
					Kind:        canonical.EventTextDelta,
					ChoiceIndex: cIdx,
					ItemIndex:   cs.messageItemIdx,
					PartIndex:   0,
					ItemID:      delta.ID,
					ID:          d.id,
					Model:       d.model,
					CreatedAt:   d.createdAt,
					ServiceTier: d.serviceTier,
					Role:        cs.role,
					Text:        deltaText,
					Annotations: cloneRawSlice(delta.Annotations),
					Logprobs:    bytes.Clone(wc.Logprobs),
				}); err != nil {
					return err
				}
			}

			calls := delta.ToolCalls
			if len(calls) == 0 && delta.FunctionCall != nil {
				calls = []chatWireInToolCall{{
					Index:    0,
					ID:       delta.FunctionCall.ID,
					Type:     "function",
					Function: *delta.FunctionCall,
				}}
			}
			for _, call := range calls {
				tIdx := call.Index
				ts, ok := cs.tools[tIdx]
				if !ok {
					ts = &chatStreamToolState{
						itemIndex: cs.allocItemIndex(),
					}
					cs.tools[tIdx] = ts
				}
				if call.ID != "" {
					ts.callID = call.ID
				}
				if call.Function.Name != "" {
					if !ts.started {
						ts.name += call.Function.Name
					} else if call.Function.Name != ts.name {
						ts.name += call.Function.Name
					}
				}
				if call.ThoughtSignature != "" {
					ts.thoughtSignature = call.ThoughtSignature
					ts.sigOrigin = canonical.OpaqueOrigin{Protocol: canonical.ProtocolGeminiGenerate, Supplier: "google"}
				} else if call.ExtraContent != nil && call.ExtraContent.Google != nil && call.ExtraContent.Google.ThoughtSignature != "" {
					ts.thoughtSignature = call.ExtraContent.Google.ThoughtSignature
					ts.sigOrigin = canonical.OpaqueOrigin{Protocol: canonical.ProtocolGeminiGenerate, Supplier: "google"}
				}

				if !ts.started {
					readyToStart := (ts.callID != "" && ts.name != "") ||
						(ts.name != "" && (call.Function.Arguments != "" || ts.pendingArgs.Len() > 0))
					if readyToStart {
						ts.started = true
						if err := emit(canonical.Event{
							Kind:             canonical.EventToolCallStart,
							ChoiceIndex:      cIdx,
							ItemIndex:        ts.itemIndex,
							ToolIndex:        tIdx,
							CallID:           ts.callID,
							ID:               d.id,
							Model:            d.model,
							CreatedAt:        d.createdAt,
							ServiceTier:      d.serviceTier,
							Role:             cs.role,
							ToolName:         ts.name,
							ThoughtSignature: ts.thoughtSignature,
							SignatureOrigin:  ts.sigOrigin,
						}); err != nil {
							return err
						}
						if ts.pendingArgs.Len() > 0 {
							buffered := ts.pendingArgs.String()
							ts.pendingArgs.Reset()
							if err := emit(canonical.Event{
								Kind:             canonical.EventToolCallArgsDelta,
								ChoiceIndex:      cIdx,
								ItemIndex:        ts.itemIndex,
								ToolIndex:        tIdx,
								CallID:           ts.callID,
								ID:               d.id,
								Model:            d.model,
								CreatedAt:        d.createdAt,
								ServiceTier:      d.serviceTier,
								Role:             cs.role,
								ToolName:         ts.name,
								Text:             buffered,
								ThoughtSignature: ts.thoughtSignature,
								SignatureOrigin:  ts.sigOrigin,
							}); err != nil {
								return err
							}
						}
					} else if call.Function.Arguments != "" {
						ts.pendingArgs.WriteString(call.Function.Arguments)
						continue
					}
				}

				if ts.started && call.Function.Arguments != "" {
					if err := emit(canonical.Event{
						Kind:             canonical.EventToolCallArgsDelta,
						ChoiceIndex:      cIdx,
						ItemIndex:        ts.itemIndex,
						ToolIndex:        tIdx,
						CallID:           ts.callID,
						ID:               d.id,
						Model:            d.model,
						CreatedAt:        d.createdAt,
						ServiceTier:      d.serviceTier,
						Role:             cs.role,
						ToolName:         ts.name,
						Text:             call.Function.Arguments,
						ThoughtSignature: ts.thoughtSignature,
						SignatureOrigin:  ts.sigOrigin,
					}); err != nil {
						return err
					}
				}
			}
		}

		if wc.FinishReason != nil {
			finish := classifyChatFinishReason(*wc.FinishReason)
			if finish.Category != canonical.FinishNone {
				if err := d.flushPendingTools(cIdx, cs, emit); err != nil {
					return err
				}
				if err := emit(canonical.Event{
					Kind:        canonical.EventChoiceFinish,
					ChoiceIndex: cIdx,
					ID:          d.id,
					Model:       d.model,
					CreatedAt:   d.createdAt,
					ServiceTier: d.serviceTier,
					Finish:      finish,
					Logprobs:    bytes.Clone(wc.Logprobs),
				}); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (d *chatCanonicalEventDecoder) End() error {
	return nil
}
