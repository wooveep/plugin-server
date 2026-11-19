package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/canonical"
	"github.com/higress-group/wasm-go/pkg/ai/fixedtools"
)

type messagesWireThinking struct {
	Type         string            `json:"type"`
	BudgetTokens wireOptional[int] `json:"budget_tokens"`
	Display      string            `json:"display"`
}

type messagesWireOutputConfig struct {
	Effort wireOptional[string] `json:"effort"`
	Format json.RawMessage      `json:"format"`
}

type messagesWireOutputFormat struct {
	Type        string             `json:"type"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Schema      json.RawMessage    `json:"schema"`
	Strict      wireOptional[bool] `json:"strict"`
}

type messagesWireTool struct {
	Type         string                `json:"type"`
	Name         string                `json:"name"`
	Description  string                `json:"description"`
	InputSchema  json.RawMessage       `json:"input_schema"`
	Strict       wireOptional[bool]    `json:"strict"`
	CacheControl *chatWireCacheControl `json:"cache_control"`
}

type messagesWireToolChoice struct {
	Type                   string             `json:"type"`
	Name                   string             `json:"name"`
	DisableParallelToolUse wireOptional[bool] `json:"disable_parallel_tool_use"`
}

type messagesWireImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
	URL       string `json:"url"`
	FileID    string `json:"file_id"`
}

type messagesWireBlock struct {
	Type         string                   `json:"type"`
	Text         wireOptional[string]     `json:"text"`
	Thinking     wireOptional[string]     `json:"thinking"`
	Signature    string                   `json:"signature"`
	Data         wireOptional[string]     `json:"data"`
	Source       *messagesWireImageSource `json:"source"`
	ID           string                   `json:"id"`
	Name         string                   `json:"name"`
	Input        json.RawMessage          `json:"input"`
	ToolUseID    string                   `json:"tool_use_id"`
	Content      json.RawMessage          `json:"content"`
	IsError      bool                     `json:"is_error"`
	CacheControl *chatWireCacheControl    `json:"cache_control"`
}

type messagesWireMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type messagesWireInRequest struct {
	Model             wireOptional[string]                   `json:"model"`
	Messages          json.RawMessage                        `json:"messages"`
	System            json.RawMessage                        `json:"system"`
	MaxTokens         wireOptional[int]                      `json:"max_tokens"`
	StopSequences     json.RawMessage                        `json:"stop_sequences"`
	Stream            wireOptional[bool]                     `json:"stream"`
	Temperature       wireOptional[float64]                  `json:"temperature"`
	TopP              wireOptional[float64]                  `json:"top_p"`
	TopK              wireOptional[int]                      `json:"top_k"`
	Tools             json.RawMessage                        `json:"tools"`
	ToolChoice        wireOptional[messagesWireToolChoice]   `json:"tool_choice"`
	ServiceTier       wireOptional[string]                   `json:"service_tier"`
	Metadata          json.RawMessage                        `json:"metadata"`
	ContextManagement json.RawMessage                        `json:"context_management"`
	Speed             wireOptional[string]                   `json:"speed"`
	CacheControl      wireOptional[chatWireCacheControl]     `json:"cache_control"`
	Thinking          wireOptional[messagesWireThinking]     `json:"thinking"`
	OutputConfig      wireOptional[messagesWireOutputConfig] `json:"output_config"`
	AnthropicBeta     wireOptional[[]string]                 `json:"anthropic_beta"`
	AnthropicVersion  wireOptional[string]                   `json:"anthropic_version"`
	Container         json.RawMessage                        `json:"container"`
	MCPServers        json.RawMessage                        `json:"mcp_servers"`
}

// messagesTTLTracker enforces AN05 cache_control rules across tools -> system -> messages:
// valid ephemeral type, allowed TTL ("", "5m", "1h"), and longer TTL (1h) before shorter TTL (5m).
type messagesTTLTracker struct {
	seenShortTTL bool
}

func (t *messagesTTLTracker) parse(cc *chatWireCacheControl, field string) (*canonical.CacheBoundary, error) {
	if cc == nil {
		return nil, nil
	}
	if cc.Type != "ephemeral" {
		return nil, canonical.NewRejectionError("invalid_cache_control", field, "Anthropic cache_control.type must be ephemeral")
	}
	switch cc.TTL {
	case "", "5m":
		t.seenShortTTL = true
	case "1h":
		if t.seenShortTTL {
			return nil, canonical.NewRejectionError("invalid_cache_control_ttl_order", field, "1h cache_control cannot follow a 5m cache_control in tools->system->messages prefix")
		}
	default:
		return nil, canonical.NewRejectionError("invalid_cache_control", field, fmt.Sprintf("unsupported Anthropic cache_control ttl %q", cc.TTL))
	}
	return &canonical.CacheBoundary{
		Origin: canonical.CacheOriginEphemeralControl,
		Mode:   "ephemeral",
		TTL:    cc.TTL,
	}, nil
}

func parseMessagesImageSource(src *messagesWireImageSource) (*canonical.ImagePart, error) {
	if src == nil {
		return nil, canonical.NewRejectionError("invalid_image_content", "image", "Anthropic image block requires source")
	}
	switch src.Type {
	case "base64":
		if src.MediaType == "" || src.Data == "" {
			return nil, canonical.NewRejectionError("invalid_image_content", "image", "Anthropic base64 image source requires media_type and data")
		}
		return &canonical.ImagePart{
			Source:    canonical.ImageDataURL,
			URL:       "data:" + src.MediaType + ";base64," + src.Data,
			MediaType: src.MediaType,
			Base64Ref: src.Data,
		}, nil
	case "url":
		if src.URL == "" {
			return nil, canonical.NewRejectionError("invalid_image_content", "image", "Anthropic url image source requires url")
		}
		return parseCanonicalImageURL(src.URL, canonical.ImageDetailUnset, "image")
	case "file":
		if src.FileID == "" {
			return nil, canonical.NewRejectionError("invalid_image_content", "file_id", "Anthropic file image source requires file_id")
		}
		return &canonical.ImagePart{
			Source: canonical.ImageFileID,
			FileID: src.FileID,
		}, nil
	default:
		return nil, canonical.NewRejectionError("invalid_image_content", "image", fmt.Sprintf("unsupported Anthropic image source type %q", src.Type))
	}
}

func decodeMessagesCanonical(body []byte) (canonical.Request, error) {
	trimmedBody := bytes.TrimSpace(body)
	if len(trimmedBody) == 0 || trimmedBody[0] != '{' {
		return canonical.Request{}, canonical.NewRejectionError("invalid_json_request", "", "Messages request body must be a JSON object")
	}

	var raw messagesWireInRequest
	if err := json.Unmarshal(trimmedBody, &raw); err != nil {
		return canonical.Request{}, canonical.NewRejectionError("invalid_request_schema", "", fmt.Sprintf("unable to unmarshal Messages request: %v", err))
	}
	if raw.Model.state == canonical.Null {
		return canonical.Request{}, canonical.NewRejectionError("invalid_model", "model", "model cannot be null")
	}
	if raw.Stream.state == canonical.Null {
		return canonical.Request{}, canonical.NewRejectionError("invalid_stream_flag", "stream", "stream cannot be null")
	}
	if raw.MaxTokens.state == canonical.Null {
		return canonical.Request{}, canonical.NewRejectionError("invalid_output_limit", "max_tokens", "max_tokens cannot be null")
	}
	if isNonEmptyJSONReference(raw.Container) {
		return canonical.Request{}, canonical.NewRejectionError("unsupported_stateful_reference", "container", "Anthropic container reference is unsupported across protocols")
	}
	if trimmedMCP := bytes.TrimSpace(raw.MCPServers); len(trimmedMCP) > 0 && !bytes.Equal(trimmedMCP, []byte("null")) && !bytes.Equal(trimmedMCP, []byte("[]")) {
		return canonical.Request{}, canonical.NewRejectionError("unsupported_tool_type", "mcp_servers", "Anthropic mcp_servers is unsupported across protocols")
	}

	trimmedMessages := bytes.TrimSpace(raw.Messages)
	if len(trimmedMessages) == 0 || bytes.Equal(trimmedMessages, []byte("null")) {
		return canonical.Request{}, canonical.NewRejectionError("missing_messages", "messages", "messages is required")
	}
	var wireMessages []messagesWireMessage
	if err := json.Unmarshal(trimmedMessages, &wireMessages); err != nil {
		return canonical.Request{}, canonical.NewRejectionError("invalid_messages", "messages", fmt.Sprintf("invalid messages array: %v", err))
	}
	if len(wireMessages) == 0 {
		return canonical.Request{}, canonical.NewRejectionError("missing_messages", "messages", "messages must not be empty")
	}

	req := canonical.Request{
		SourceProtocol: canonical.ProtocolAnthropicMessages,
		Model:          raw.Model.toCanonical().ValueOr(""),
		Stream:         raw.Stream.toCanonical().ValueOr(false),
		ServiceTier:    raw.ServiceTier.toCanonical(),
		Sampling: canonical.Sampling{
			Temperature: raw.Temperature.toCanonical(),
			TopP:        raw.TopP.toCanonical(),
			TopK:        raw.TopK.toCanonical(),
		},
	}

	if raw.MaxTokens.state == canonical.Present {
		if raw.MaxTokens.val < 0 {
			return canonical.Request{}, canonical.NewRejectionError("invalid_output_limit", "max_tokens", "max_tokens must be non-negative")
		}
		req.Limits = canonical.OutputLimit{
			Source: canonical.LimitMaxTokens,
			Scope:  canonical.ScopeSourceDefined,
			Value:  canonical.Some(raw.MaxTokens.val),
		}
	}

	if trimmedStops := bytes.TrimSpace(raw.StopSequences); len(trimmedStops) > 0 && !bytes.Equal(trimmedStops, []byte("null")) {
		var rawStopElems []json.RawMessage
		if err := json.Unmarshal(trimmedStops, &rawStopElems); err != nil {
			return canonical.Request{}, canonical.NewRejectionError("invalid_stop", "stop_sequences", "stop_sequences must be an array of strings")
		}
		stops := make([]string, len(rawStopElems))
		for i, elem := range rawStopElems {
			te := bytes.TrimSpace(elem)
			if len(te) == 0 || te[0] != '"' {
				return canonical.Request{}, canonical.NewRejectionError("invalid_stop", "stop_sequences", "stop_sequences elements must be strings")
			}
			if err := json.Unmarshal(te, &stops[i]); err != nil {
				return canonical.Request{}, canonical.NewRejectionError("invalid_stop", "stop_sequences", "invalid stop_sequences string")
			}
		}
		req.Sampling.Stop = stops
		req.Sampling.StopPresent = true
	}

	if trimmedMeta := bytes.TrimSpace(raw.Metadata); len(trimmedMeta) > 0 {
		if bytes.Equal(trimmedMeta, []byte("null")) {
			req.Metadata = canonical.NullOf[map[string]string]()
		} else {
			var metaRaw map[string]json.RawMessage
			if err := json.Unmarshal(trimmedMeta, &metaRaw); err != nil || metaRaw == nil {
				return canonical.Request{}, canonical.NewRejectionError("invalid_metadata", "metadata", "metadata must be a JSON object or null")
			}
			metaMap := make(map[string]string, len(metaRaw))
			for k, v := range metaRaw {
				tv := bytes.TrimSpace(v)
				if len(tv) > 0 && tv[0] == '"' {
					var sv string
					if err := json.Unmarshal(tv, &sv); err == nil {
						metaMap[k] = sv
					}
				}
			}
			req.Metadata = canonical.Some(metaMap)
		}
	}

	// Thinking & output_config.effort (2.0.91 / 2.0.94 / AN03 / S-BOUND-03).
	if raw.Thinking.state == canonical.Present {
		switch raw.Thinking.val.Type {
		case "enabled":
			req.Thinking.Mode = canonical.ThinkingEnabled
			if raw.Thinking.val.BudgetTokens.state == canonical.Present {
				if raw.Thinking.val.BudgetTokens.val < 0 {
					return canonical.Request{}, canonical.NewRejectionError("invalid_thinking_config", "thinking.budget_tokens", "thinking.budget_tokens must be non-negative")
				}
				req.Thinking.BudgetTokens = canonical.Some(raw.Thinking.val.BudgetTokens.val)
			}
		case "adaptive":
			req.Thinking.Mode = canonical.ThinkingAdaptive
			effort := "medium"
			if raw.OutputConfig.state == canonical.Present && raw.OutputConfig.val.Effort.state == canonical.Present {
				rawEffort := strings.ToLower(strings.TrimSpace(raw.OutputConfig.val.Effort.val))
				switch rawEffort {
				case "":
					effort = "medium"
				case "low":
					effort = "low"
				case "medium":
					effort = "medium"
				case "high", "xhigh", "max":
					effort = "high"
				default:
					return canonical.Request{}, canonical.NewRejectionError("invalid_thinking_effort", "output_config.effort", fmt.Sprintf("unsupported output_config.effort %q", rawEffort))
				}
			}
			req.Thinking.Effort = canonical.Some(effort)
		case "disabled":
			// S-BOUND-04 / T11: explicit disabled thinking is never re-opened by output_config.effort.
			req.Thinking.Mode = canonical.ThinkingDisabled
		default:
			return canonical.Request{}, canonical.NewRejectionError("invalid_thinking_config", "thinking.type", fmt.Sprintf("unsupported thinking.type %q", raw.Thinking.val.Type))
		}
	}

	if raw.OutputConfig.state == canonical.Present {
		if trimmedFmt := bytes.TrimSpace(raw.OutputConfig.val.Format); len(trimmedFmt) > 0 && !bytes.Equal(trimmedFmt, []byte("null")) {
			var wf messagesWireOutputFormat
			if err := json.Unmarshal(trimmedFmt, &wf); err != nil {
				return canonical.Request{}, canonical.NewRejectionError("unsupported_response_format", "output_config.format", fmt.Sprintf("invalid output_config.format: %v", err))
			}
			switch wf.Type {
			case "text":
				req.ResponseFormat = canonical.ResponseFormat{Kind: canonical.ResponseFormatText}
			case "json_object":
				req.ResponseFormat = canonical.ResponseFormat{Kind: canonical.ResponseFormatJSONObject}
			case "json_schema":
				trimmedSchema := bytes.TrimSpace(wf.Schema)
				if len(trimmedSchema) == 0 || bytes.Equal(trimmedSchema, []byte("null")) || !json.Valid(trimmedSchema) || trimmedSchema[0] != '{' {
					return canonical.Request{}, canonical.NewRejectionError("unsupported_response_format", "output_config.format", "output_config.format json_schema requires a valid schema object")
				}
				name := wf.Name
				if name == "" {
					name = "response"
				}
				req.ResponseFormat = canonical.ResponseFormat{
					Kind:        canonical.ResponseFormatJSONSchema,
					Name:        name,
					Description: wf.Description,
					Schema:      bytes.Clone(trimmedSchema),
					Strict:      wf.Strict.toCanonical(),
				}
			default:
				return canonical.Request{}, canonical.NewRejectionError("unsupported_response_format", "output_config.format", fmt.Sprintf("unsupported output_config.format type %q", wf.Type))
			}
		}
	}

	var ttlTracker messagesTTLTracker

	// 1. AN05 prefix stage 1: tools (H2 claudeTool.Type validation + strict + cache_control).
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
			var wt messagesWireTool
			if err := json.Unmarshal(rawTool, &wt); err != nil {
				return canonical.Request{}, canonical.NewRejectionError("invalid_tool_declarations", "tools", fmt.Sprintf("invalid tool definition: %v", err))
			}
			tCache, err := ttlTracker.parse(wt.CacheControl, "tools")
			if err != nil {
				return canonical.Request{}, err
			}
			switch wt.Type {
			case "", "custom":
				if wt.Name == "" {
					return canonical.Request{}, canonical.NewRejectionError("invalid_tool_definition", "tools", "Anthropic function tool requires name")
				}
				req.Tools = append(req.Tools, canonical.ToolDefinition{
					Kind:        canonical.ToolKindFunction,
					Name:        wt.Name,
					Description: wt.Description,
					Schema:      bytes.Clone(wt.InputSchema),
					Strict:      wt.Strict.toCanonical(),
					Cache:       tCache,
				})
			default:
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
				return canonical.Request{}, canonical.NewRejectionError("unsupported_tool_type", "tools", fmt.Sprintf("Anthropic built-in tool type %q is unsupported across protocols", wt.Type))
			}
		}
	}

	// ToolChoice & DEF-04 (never force parallel_tool_calls:true when disable_parallel_tool_use is omitted).
	if raw.ToolChoice.state == canonical.Present {
		tc := raw.ToolChoice.val
		switch tc.Type {
		case "auto":
			req.ToolChoice = canonical.ToolChoice{Mode: canonical.ToolChoiceAuto}
		case "none":
			req.ToolChoice = canonical.ToolChoice{Mode: canonical.ToolChoiceNone}
		case "any":
			req.ToolChoice = canonical.ToolChoice{Mode: canonical.ToolChoiceAny}
		case "tool":
			if tc.Name == "" {
				return canonical.Request{}, canonical.NewRejectionError("unsupported_tool_choice", "tool_choice", "Anthropic tool_choice type tool requires name")
			}
			req.ToolChoice = canonical.ToolChoice{Mode: canonical.ToolChoiceFunction, Name: tc.Name}
		default:
			return canonical.Request{}, canonical.NewRejectionError("unsupported_tool_choice", "tool_choice", fmt.Sprintf("unsupported Anthropic tool_choice type %q", tc.Type))
		}
		if tc.DisableParallelToolUse.state == canonical.Present {
			req.ParallelToolCalls = canonical.Some(!tc.DisableParallelToolUse.val)
		}
	}

	// 2. AN05 prefix stage 2: system (string or text-block array, with stripCchFromBillingHeader).
	if trimmedSys := bytes.TrimSpace(raw.System); len(trimmedSys) > 0 && !bytes.Equal(trimmedSys, []byte("null")) {
		switch trimmedSys[0] {
		case '"':
			var sysStr string
			if err := json.Unmarshal(trimmedSys, &sysStr); err != nil {
				return canonical.Request{}, canonical.NewRejectionError("invalid_system_prompt", "system", "invalid system string")
			}
			processed := stripCchFromBillingHeader(sysStr)
			if processed != "" || sysStr != "" {
				req.SystemParts = []canonical.Part{{Kind: canonical.PartText, Text: processed}}
				req.SystemIsArray = false
			}
		case '[':
			var sysBlocks []messagesWireBlock
			if err := json.Unmarshal(trimmedSys, &sysBlocks); err != nil {
				return canonical.Request{}, canonical.NewRejectionError("invalid_system_prompt", "system", fmt.Sprintf("invalid system blocks: %v", err))
			}
			req.SystemIsArray = true
			req.SystemParts = make([]canonical.Part, 0, len(sysBlocks))
			for _, sb := range sysBlocks {
				if sb.Type != "text" || !sb.Text.toCanonical().IsPresent() {
					return canonical.Request{}, canonical.NewRejectionError("invalid_system_prompt", "system", "Anthropic system blocks must be text blocks")
				}
				sbCache, err := ttlTracker.parse(sb.CacheControl, "system")
				if err != nil {
					return canonical.Request{}, err
				}
				req.SystemParts = append(req.SystemParts, canonical.Part{
					Kind:  canonical.PartText,
					Text:  stripCchFromBillingHeader(sb.Text.val),
					Cache: sbCache,
				})
			}
		default:
			return canonical.Request{}, canonical.NewRejectionError("invalid_system_prompt", "system", "system must be a string or array of text blocks")
		}
	}

	// 3. AN05 prefix stage 3 & AN08 ordering: messages.
	req.Items = make([]canonical.Item, 0, len(wireMessages))
	pendingToolUses := make(map[string]string)
	seenToolUseIDs := make(map[string]bool)

	for _, msg := range wireMessages {
		role := canonical.Role(msg.Role)
		if role != canonical.RoleUser && role != canonical.RoleAssistant {
			return canonical.Request{}, canonical.NewRejectionError("unsupported_message_role", "messages.role", fmt.Sprintf("Anthropic message role %q is unsupported", msg.Role))
		}

		tc := bytes.TrimSpace(msg.Content)
		if len(tc) == 0 || bytes.Equal(tc, []byte("null")) {
			return canonical.Request{}, canonical.NewRejectionError("invalid_message_content", "messages.content", "message content is required")
		}

		if role == canonical.RoleAssistant {
			if len(pendingToolUses) > 0 {
				return canonical.Request{}, canonical.NewRejectionError("missing_tool_result", "tool_result", "assistant tool_use must be immediately followed by user tool_result")
			}

			if tc[0] == '"' {
				var s string
				if err := json.Unmarshal(tc, &s); err != nil {
					return canonical.Request{}, canonical.NewRejectionError("invalid_message_content", "messages.content", "invalid assistant content string")
				}
				req.Items = append(req.Items, canonical.Item{
					Kind:  canonical.ItemMessage,
					Role:  canonical.RoleAssistant,
					Parts: []canonical.Part{{Kind: canonical.PartText, Text: stripCchFromBillingHeader(s)}},
				})
				continue
			}

			if tc[0] != '[' {
				return canonical.Request{}, canonical.NewRejectionError("invalid_message_content", "messages.content", "assistant content must be string or array of blocks")
			}

			var blocks []messagesWireBlock
			if err := json.Unmarshal(tc, &blocks); err != nil {
				return canonical.Request{}, canonical.NewRejectionError("invalid_message_content", "messages.content", fmt.Sprintf("invalid assistant content blocks: %v", err))
			}
			if len(blocks) == 0 {
				req.Items = append(req.Items, canonical.Item{
					Kind:           canonical.ItemMessage,
					Role:           canonical.RoleAssistant,
					Parts:          []canonical.Part{{Kind: canonical.PartText, Text: ""}},
					MessageIsArray: true,
				})
				continue
			}

			var textAccum []canonical.Part
			var toolUseTrailingCache *canonical.CacheBoundary
			flushAssistantText := func() {
				if len(textAccum) == 0 {
					return
				}
				req.Items = append(req.Items, canonical.Item{
					Kind:           canonical.ItemMessage,
					Role:           canonical.RoleAssistant,
					Parts:          textAccum,
					MessageIsArray: true,
				})
				textAccum = nil
			}

			for _, b := range blocks {
				switch b.Type {
				case "thinking":
					if b.CacheControl != nil {
						return canonical.Request{}, canonical.NewRejectionError("invalid_cache_control", "thinking", "thinking blocks cannot carry cache_control")
					}
					thinkText := b.Thinking.toCanonical().ValueOr("")
					if thinkText == "" && b.Signature == "" && !b.Thinking.toCanonical().IsPresent() {
						return canonical.Request{}, canonical.NewRejectionError("invalid_thinking_block", "thinking", "thinking block requires thinking text or signature")
					}
					flushAssistantText()
					var rTexts []string
					if thinkText != "" {
						rTexts = []string{thinkText}
					}
					req.Items = append(req.Items, canonical.Item{
						Kind: canonical.ItemReasoning,
						Role: canonical.RoleAssistant,
						Reasoning: &canonical.ReasoningItem{
							Text:      rTexts,
							Signature: b.Signature,
							Origin:    canonical.OpaqueOrigin{Protocol: canonical.ProtocolAnthropicMessages, Supplier: "anthropic"},
						},
					})

				case "redacted_thinking":
					if b.CacheControl != nil {
						return canonical.Request{}, canonical.NewRejectionError("invalid_cache_control", "thinking", "redacted_thinking blocks cannot carry cache_control")
					}
					redacted := b.Data.toCanonical().ValueOr("")
					if redacted == "" {
						return canonical.Request{}, canonical.NewRejectionError("invalid_thinking_block", "redacted_thinking", "redacted_thinking block requires data")
					}
					flushAssistantText()
					req.Items = append(req.Items, canonical.Item{
						Kind: canonical.ItemReasoning,
						Role: canonical.RoleAssistant,
						Reasoning: &canonical.ReasoningItem{
							Redacted: redacted,
							Origin:   canonical.OpaqueOrigin{Protocol: canonical.ProtocolAnthropicMessages, Supplier: "anthropic"},
						},
					})

				case "text":
					if !b.Text.toCanonical().IsPresent() {
						return canonical.Request{}, canonical.NewRejectionError("invalid_message_content", "messages.content", "text block requires string text")
					}
					bCache, err := ttlTracker.parse(b.CacheControl, "messages")
					if err != nil {
						return canonical.Request{}, err
					}
					textAccum = append(textAccum, canonical.Part{
						Kind:  canonical.PartText,
						Text:  stripCchFromBillingHeader(b.Text.val),
						Cache: bCache,
					})

				case "tool_use":
					tuCache, err := ttlTracker.parse(b.CacheControl, "messages")
					if err != nil {
						return canonical.Request{}, err
					}
					if tuCache != nil {
						toolUseTrailingCache = tuCache
					}
					if b.ID == "" || b.Name == "" {
						return canonical.Request{}, canonical.NewRejectionError("invalid_tool_use", "tool_use", "tool_use requires id and name")
					}
					if seenToolUseIDs[b.ID] {
						return canonical.Request{}, canonical.NewRejectionError("invalid_tool_use", "tool_use", "duplicate tool_use id")
					}
					trimmedInput := bytes.TrimSpace(b.Input)
					var inputObj json.RawMessage
					if len(trimmedInput) == 0 {
						inputObj = json.RawMessage("{}")
					} else {
						if trimmedInput[0] != '{' || !json.Valid(trimmedInput) {
							return canonical.Request{}, canonical.NewRejectionError("invalid_tool_use", "tool_use.input", "tool_use input must be a valid JSON object")
						}
						inputObj = bytes.Clone(trimmedInput)
					}
					flushAssistantText()
					pendingToolUses[b.ID] = b.Name
					seenToolUseIDs[b.ID] = true
					req.Items = append(req.Items, canonical.Item{
						Kind: canonical.ItemFunctionCall,
						Role: canonical.RoleAssistant,
						Call: &canonical.FunctionCall{
							ItemID: b.ID,
							CallID: b.ID,
							Name:   b.Name,
							Args:   canonical.Arguments{Kind: canonical.ArgsObject, Object: inputObj},
						},
					})

				default:
					field := canonical.SafeIdentifier(b.Type)
					if field == "" {
						field = "messages.content"
					}
					return canonical.Request{}, canonical.NewRejectionError("unsupported_content_part_type", field, fmt.Sprintf("unsupported Anthropic assistant block %q", b.Type))
				}
			}
			flushAssistantText()
			if toolUseTrailingCache != nil {
				// Preserve trailing tool_use cache_control on an assistant message marker if no text carried it.
				attached := false
				for j := len(req.Items) - 1; j >= 0; j-- {
					if req.Items[j].Role != canonical.RoleAssistant {
						break
					}
					if req.Items[j].Kind == canonical.ItemMessage && req.Items[j].Cache == nil {
						req.Items[j].Cache = toolUseTrailingCache
						attached = true
						break
					}
				}
				if !attached {
					req.Items = append(req.Items, canonical.Item{
						Kind:  canonical.ItemMessage,
						Role:  canonical.RoleAssistant,
						Cache: toolUseTrailingCache,
					})
				}
			}
			continue
		}

		// User message (AN08: tool_result must immediately follow assistant tool_use and precede ordinary text/image blocks).
		if tc[0] == '"' {
			if len(pendingToolUses) > 0 {
				return canonical.Request{}, canonical.NewRejectionError("missing_tool_result", "tool_result", "user message immediately following tool_use must begin with tool_result blocks")
			}
			var s string
			if err := json.Unmarshal(tc, &s); err != nil {
				return canonical.Request{}, canonical.NewRejectionError("invalid_message_content", "messages.content", "invalid user content string")
			}
			req.Items = append(req.Items, canonical.Item{
				Kind:  canonical.ItemMessage,
				Role:  canonical.RoleUser,
				Parts: []canonical.Part{{Kind: canonical.PartText, Text: stripCchFromBillingHeader(s)}},
			})
			continue
		}

		if tc[0] != '[' {
			return canonical.Request{}, canonical.NewRejectionError("invalid_message_content", "messages.content", "user content must be string or array of blocks")
		}

		var blocks []messagesWireBlock
		if err := json.Unmarshal(tc, &blocks); err != nil {
			return canonical.Request{}, canonical.NewRejectionError("invalid_message_content", "messages.content", fmt.Sprintf("invalid user content blocks: %v", err))
		}
		if len(blocks) == 0 {
			return canonical.Request{}, canonical.NewRejectionError("invalid_message_content", "messages.content", "user content array must not be empty")
		}

		seenNonToolResult := false
		var trailingUserParts []canonical.Part

		for _, b := range blocks {
			switch b.Type {
			case "tool_result":
				if seenNonToolResult {
					return canonical.Request{}, canonical.NewRejectionError("invalid_tool_result_order", "tool_result", "tool_result blocks must precede ordinary text and image blocks in the same user message")
				}
				if b.ToolUseID == "" {
					return canonical.Request{}, canonical.NewRejectionError("invalid_tool_result", "tool_result", "tool_result requires tool_use_id")
				}
				fnName, ok := pendingToolUses[b.ToolUseID]
				if !ok {
					return canonical.Request{}, canonical.NewRejectionError("orphan_tool_result", "tool_result", "tool_result must immediately follow its matching assistant tool_use")
				}
				delete(pendingToolUses, b.ToolUseID)

				trCache, err := ttlTracker.parse(b.CacheControl, "messages")
				if err != nil {
					return canonical.Request{}, err
				}

				trRaw := bytes.TrimSpace(b.Content)
				var resParts []canonical.Part
				isStr := true
				if len(trRaw) == 0 || bytes.Equal(trRaw, []byte("null")) {
					resParts = []canonical.Part{{Kind: canonical.PartText, Text: ""}}
				} else if trRaw[0] == '"' {
					var s string
					if err := json.Unmarshal(trRaw, &s); err != nil {
						return canonical.Request{}, canonical.NewRejectionError("invalid_tool_result", "tool_result", "invalid tool_result string content")
					}
					resParts = []canonical.Part{{Kind: canonical.PartText, Text: stripCchFromBillingHeader(s)}}
				} else if trRaw[0] == '[' {
					var nested []messagesWireBlock
					if err := json.Unmarshal(trRaw, &nested); err != nil {
						return canonical.Request{}, canonical.NewRejectionError("invalid_tool_result", "tool_result", fmt.Sprintf("invalid tool_result content array: %v", err))
					}
					if len(nested) == 0 {
						resParts = []canonical.Part{{Kind: canonical.PartText, Text: ""}}
					} else {
						isStr = false
						resParts = make([]canonical.Part, 0, len(nested))
						for _, nb := range nested {
							switch nb.Type {
							case "text":
								if !nb.Text.toCanonical().IsPresent() {
									return canonical.Request{}, canonical.NewRejectionError("invalid_tool_result", "tool_result", "tool_result text block requires string text")
								}
								nbCache, err := ttlTracker.parse(nb.CacheControl, "tool_result")
								if err != nil {
									return canonical.Request{}, err
								}
								resParts = append(resParts, canonical.Part{
									Kind:  canonical.PartText,
									Text:  stripCchFromBillingHeader(nb.Text.val),
									Cache: nbCache,
								})
							case "image":
								nbCache, err := ttlTracker.parse(nb.CacheControl, "tool_result")
								if err != nil {
									return canonical.Request{}, err
								}
								imgPart, err := parseMessagesImageSource(nb.Source)
								if err != nil {
									return canonical.Request{}, err
								}
								resParts = append(resParts, canonical.Part{
									Kind:  canonical.PartImage,
									Image: imgPart,
									Cache: nbCache,
								})
							default:
								field := canonical.SafeIdentifier(nb.Type)
								if field == "" {
									field = "tool_result"
								}
								return canonical.Request{}, canonical.NewRejectionError("unsupported_tool_result_content", field, fmt.Sprintf("tool_result content block %q is unsupported across protocols", nb.Type))
							}
						}
					}
				} else {
					return canonical.Request{}, canonical.NewRejectionError("invalid_tool_result", "tool_result", "tool_result content must be string or array")
				}

				req.Items = append(req.Items, canonical.Item{
					Kind: canonical.ItemFunctionResult,
					Role: canonical.RoleTool,
					Result: &canonical.FunctionResult{
						CallID:       b.ToolUseID,
						Name:         fnName,
						Parts:        resParts,
						IsStringForm: isStr,
						IsError:      b.IsError,
						Cache:        trCache,
					},
				})

			case "text":
				seenNonToolResult = true
				if !b.Text.toCanonical().IsPresent() {
					return canonical.Request{}, canonical.NewRejectionError("invalid_message_content", "messages.content", "text block requires string text")
				}
				bCache, err := ttlTracker.parse(b.CacheControl, "messages")
				if err != nil {
					return canonical.Request{}, err
				}
				trailingUserParts = append(trailingUserParts, canonical.Part{
					Kind:  canonical.PartText,
					Text:  stripCchFromBillingHeader(b.Text.val),
					Cache: bCache,
				})

			case "image":
				seenNonToolResult = true
				bCache, err := ttlTracker.parse(b.CacheControl, "messages")
				if err != nil {
					return canonical.Request{}, err
				}
				imgPart, err := parseMessagesImageSource(b.Source)
				if err != nil {
					return canonical.Request{}, err
				}
				trailingUserParts = append(trailingUserParts, canonical.Part{
					Kind:  canonical.PartImage,
					Image: imgPart,
					Cache: bCache,
				})

			case "thinking", "redacted_thinking":
				if b.CacheControl != nil {
					return canonical.Request{}, canonical.NewRejectionError("invalid_cache_control", "thinking", "thinking blocks cannot carry cache_control")
				}
				return canonical.Request{}, canonical.NewRejectionError("unsupported_content_part_type", b.Type, "thinking blocks are only valid in assistant messages")

			default:
				field := canonical.SafeIdentifier(b.Type)
				if field == "" {
					field = "messages.content"
				}
				return canonical.Request{}, canonical.NewRejectionError("unsupported_content_part_type", field, fmt.Sprintf("Anthropic content block %q is unsupported across protocols", b.Type))
			}
		}

		if len(pendingToolUses) > 0 {
			return canonical.Request{}, canonical.NewRejectionError("missing_tool_result", "tool_result", "every assistant tool_use must have a matching tool_result in the immediately following user message")
		}
		if len(trailingUserParts) > 0 {
			req.Items = append(req.Items, canonical.Item{
				Kind:           canonical.ItemMessage,
				Role:           canonical.RoleUser,
				Parts:          trailingUserParts,
				MessageIsArray: true,
			})
		}
	}

	if len(pendingToolUses) > 0 {
		return canonical.Request{}, canonical.NewRejectionError("missing_tool_result", "tool_result", "every assistant tool_use requires a following user tool_result")
	}

	if raw.CacheControl.state == canonical.Present {
		cc := raw.CacheControl.val
		reqBoundary, err := ttlTracker.parse(&cc, "cache_control")
		if err != nil {
			return canonical.Request{}, err
		}
		req.Cache.RequestBoundary = reqBoundary
	}

	return req, nil
}
