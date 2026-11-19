package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/higress-group/wasm-go/pkg/ai/canonical"
	"github.com/higress-group/wasm-go/pkg/ai/fixedtools"
)

type qwenWireContentPart struct {
	CacheControl *chatWireCacheControl `json:"cache_control,omitempty"`
	Image        string                `json:"image,omitempty"`
	Text         *string               `json:"text,omitempty"`
}

type qwenWireOutToolCall struct {
	Index    int                  `json:"index"`
	ID       string               `json:"id,omitempty"`
	Type     string               `json:"type"`
	Function chatWireFunctionCall `json:"function"`
}

type qwenWireOutMessage struct {
	Name             string                `json:"name,omitempty"`
	Role             string                `json:"role"`
	Content          json.RawMessage       `json:"content"`
	Partial          *bool                 `json:"partial,omitempty"`
	ReasoningContent *string               `json:"reasoning_content,omitempty"`
	ToolCalls        []qwenWireOutToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string                `json:"tool_call_id,omitempty"`
}

type qwenWireOutInput struct {
	Messages []qwenWireOutMessage `json:"messages"`
}

type qwenWireOutParameters struct {
	ResultFormat          string                     `json:"result_format,omitempty"`
	MaxTokens             *int                       `json:"max_tokens,omitempty"`
	MaxCompletionTokens   *int                       `json:"max_completion_tokens,omitempty"`
	RepetitionPenalty     *float64                   `json:"repetition_penalty,omitempty"`
	PresencePenalty       *float64                   `json:"presence_penalty,omitempty"`
	N                     *int                       `json:"n,omitempty"`
	Seed                  *int64                     `json:"seed,omitempty"`
	Stop                  []string                   `json:"stop,omitempty"`
	Temperature           *float64                   `json:"temperature,omitempty"`
	TopP                  *float64                   `json:"top_p,omitempty"`
	TopK                  *int                       `json:"top_k,omitempty"`
	Logprobs              *bool                      `json:"logprobs,omitempty"`
	TopLogprobs           *int                       `json:"top_logprobs,omitempty"`
	IncrementalOutput     *bool                      `json:"incremental_output,omitempty"`
	EnableSearch          *bool                      `json:"enable_search,omitempty"`
	EnableCodeInterpreter *bool                      `json:"enable_code_interpreter,omitempty"`
	SearchOptions         json.RawMessage            `json:"search_options,omitempty"`
	EnableThinking        *bool                      `json:"enable_thinking,omitempty"`
	ThinkingBudget        *int                       `json:"thinking_budget,omitempty"`
	PreserveThinking      *bool                      `json:"preserve_thinking,omitempty"`
	ResponseFormat        *chatWireOutResponseFormat `json:"response_format,omitempty"`
	Tools                 []json.RawMessage          `json:"tools,omitempty"`
	ToolChoice            json.RawMessage            `json:"tool_choice,omitempty"`
	ParallelToolCalls     *bool                      `json:"parallel_tool_calls,omitempty"`
}

type qwenWireOutRequest struct {
	Model      string                `json:"model"`
	Input      qwenWireOutInput      `json:"input"`
	Parameters qwenWireOutParameters `json:"parameters"`
}

func encodeQwenCacheControl(cb *canonical.CacheBoundary) *chatWireCacheControl {
	if cb == nil || cb.Origin != canonical.CacheOriginEphemeralControl {
		return nil
	}
	mode := cb.Mode
	if mode == "" {
		mode = "ephemeral"
	}
	return &chatWireCacheControl{
		Type: mode,
		TTL:  cb.TTL,
	}
}

func encodeQwenPartsJSON(parts []canonical.Part, multimodal bool, forceArray bool, joinSep string, inToolResult bool) (json.RawMessage, error) {
	useArray := multimodal || forceArray
	var textBuilder strings.Builder
	outParts := make([]qwenWireContentPart, 0, len(parts))

	for idx, p := range parts {
		cc := encodeQwenCacheControl(p.Cache)
		if cc != nil {
			useArray = true
		}
		switch p.Kind {
		case canonical.PartText:
			if idx > 0 && joinSep != "" {
				textBuilder.WriteString(joinSep)
			}
			textBuilder.WriteString(p.Text)
			txt := p.Text
			outParts = append(outParts, qwenWireContentPart{
				CacheControl: cc,
				Text:         &txt,
			})
		case canonical.PartRefusal:
			if idx > 0 && joinSep != "" {
				textBuilder.WriteString(joinSep)
			}
			textBuilder.WriteString(p.Refusal)
			ref := p.Refusal
			outParts = append(outParts, qwenWireContentPart{
				CacheControl: cc,
				Text:         &ref,
			})
		case canonical.PartImage:
			imgField := "image_url"
			if inToolResult {
				imgField = "tool_result"
			}
			if !multimodal {
				return nil, canonical.NewRejectionError("unsupported_image_content", imgField, "dashscope_text does not support image inputs")
			}
			if p.Image == nil {
				return nil, canonical.NewRejectionError("invalid_image_content", imgField, "image part is missing image metadata")
			}
			if p.Image.Source == canonical.ImageFileID || p.Image.FileID != "" {
				return nil, canonical.NewRejectionError("unsupported_image_content", "file_id", "image file_id cannot be resolved across protocols")
			}
			if !p.Image.Detail.Valid() {
				return nil, canonical.NewRejectionError("invalid_image_detail", "detail", "invalid image detail")
			}
			imgURL := p.Image.URL
			if imgURL == "" && p.Image.Source == canonical.ImageDataURL && p.Image.MediaType != "" && p.Image.Base64Ref != "" {
				imgURL = "data:" + p.Image.MediaType + ";base64," + p.Image.Base64Ref
			}
			if imgURL == "" {
				return nil, canonical.NewRejectionError("invalid_image_content", imgField, "image URL is empty")
			}
			useArray = true
			outParts = append(outParts, qwenWireContentPart{
				CacheControl: cc,
				Image:        imgURL,
			})
		default:
			return nil, canonical.NewRejectionError("unsupported_content_part_type", "content", "unsupported canonical content part kind")
		}
	}

	if useArray {
		if len(outParts) == 0 {
			empty := ""
			outParts = []qwenWireContentPart{{Text: &empty}}
		}
		encoded, err := json.Marshal(outParts)
		if err != nil {
			return nil, fmt.Errorf("marshal DashScope content parts: %w", err)
		}
		return encoded, nil
	}

	encoded, err := json.Marshal(textBuilder.String())
	if err != nil {
		return nil, fmt.Errorf("marshal DashScope text content: %w", err)
	}
	return encoded, nil
}

func makeQwenPlainSystemMessage(text string, multimodal bool) (qwenWireOutMessage, error) {
	if multimodal {
		t := text
		raw, err := json.Marshal([]qwenWireContentPart{{Text: &t}})
		if err != nil {
			return qwenWireOutMessage{}, err
		}
		return qwenWireOutMessage{Role: roleSystem, Content: raw}, nil
	}
	raw, err := json.Marshal(text)
	if err != nil {
		return qwenWireOutMessage{}, err
	}
	return qwenWireOutMessage{Role: roleSystem, Content: raw}, nil
}

func extractQwenMessageText(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return ""
	}
	if trimmed[0] == '"' {
		var s string
		if json.Unmarshal(trimmed, &s) == nil {
			return s
		}
		return ""
	}
	if trimmed[0] == '[' {
		var parts []struct {
			Text wireOptional[string] `json:"text"`
		}
		if json.Unmarshal(trimmed, &parts) == nil {
			var b strings.Builder
			for _, p := range parts {
				if p.Text.state == canonical.Present {
					b.WriteString(p.Text.val)
				}
			}
			return b.String()
		}
	}
	return ""
}

func applyQwenLongFileIDs(messages []qwenWireOutMessage, fileIDs []string, multimodal bool) ([]qwenWireOutMessage, error) {
	var fileBuilder strings.Builder
	for _, id := range fileIDs {
		if id == "" {
			continue
		}
		if fileBuilder.Len() > 0 {
			fileBuilder.WriteByte(',')
		}
		fileBuilder.WriteString("fileid://")
		fileBuilder.WriteString(id)
	}
	if fileBuilder.Len() == 0 {
		return messages, nil
	}

	fileMsg, err := makeQwenPlainSystemMessage(fileBuilder.String(), multimodal)
	if err != nil {
		return nil, fmt.Errorf("marshal DashScope file context message: %w", err)
	}

	firstNonSystemIdx := len(messages)
	for i, m := range messages {
		if m.Role != roleSystem {
			firstNonSystemIdx = i
			break
		}
	}

	if firstNonSystemIdx == 0 {
		dummyMsg, err := makeQwenPlainSystemMessage(qwenDummySystemMessageContent, multimodal)
		if err != nil {
			return nil, fmt.Errorf("marshal DashScope dummy system message: %w", err)
		}
		out := make([]qwenWireOutMessage, 0, len(messages)+2)
		out = append(out, dummyMsg, fileMsg)
		out = append(out, messages...)
		return out, nil
	}

	if firstNonSystemIdx == 1 {
		out := make([]qwenWireOutMessage, 0, len(messages)+1)
		out = append(out, messages[0], fileMsg)
		out = append(out, messages[1:]...)
		return out, nil
	}

	var sysBuilder strings.Builder
	for i := 0; i < firstNonSystemIdx; i++ {
		if sysBuilder.Len() > 0 {
			sysBuilder.WriteByte('\n')
		}
		sysBuilder.WriteString(extractQwenMessageText(messages[i].Content))
	}
	mergedSys, err := makeQwenPlainSystemMessage(sysBuilder.String(), multimodal)
	if err != nil {
		return nil, fmt.Errorf("marshal DashScope consolidated system message: %w", err)
	}
	out := make([]qwenWireOutMessage, 0, 2+len(messages)-firstNonSystemIdx)
	out = append(out, mergedSys, fileMsg)
	out = append(out, messages[firstNonSystemIdx:]...)
	return out, nil
}

func resolveQwenThinking(req canonical.Request) (*bool, *int) {
	var enableThinking *bool
	switch req.Thinking.Mode {
	case canonical.ThinkingEnabled, canonical.ThinkingAdaptive:
		v := true
		enableThinking = &v
	case canonical.ThinkingDisabled:
		v := false
		enableThinking = &v
	case canonical.ThinkingUnset:
		if req.Thinking.Effort.IsPresent() && req.Thinking.Effort.Val != "" {
			v := req.Thinking.Effort.Val != "none"
			enableThinking = &v
		}
	}

	var budgetPtr *int
	if req.Thinking.Mode != canonical.ThinkingDisabled && req.Thinking.BudgetTokens.IsPresent() && req.Thinking.BudgetTokens.Val > 0 {
		b := req.Thinking.BudgetTokens.Val
		budgetPtr = &b
	}
	return enableThinking, budgetPtr
}

func qwenHasDeclaredWireTools(req canonical.Request) bool {
	for _, t := range req.Tools {
		switch t.Kind {
		case canonical.ToolKindFunction:
			return true
		case canonical.ToolKindFixedServer:
			if len(t.RawFixed) > 0 || t.FixedType != "" {
				return true
			}
		}
	}
	return false
}

func resolveQwenIncrementalOutput(mode IncrementalMode, hasTools bool, enableThinking *bool) bool {
	switch mode {
	case IncrementalForceTrue:
		return true
	case IncrementalForceFalse:
		return false
	default:
		return !hasTools || (enableThinking != nil && *enableThinking)
	}
}

// BindQwenIncrementalMode resolves IncrementalAuto into an explicit IncrementalForceTrue
// or IncrementalForceFalse on target for DashScope wire formats based on the effective
// post-FilterCanonical canonical.Request so request encoding and SSE event decoding
// always bind to the exact same streaming mode (R05 / DS03).
func BindQwenIncrementalMode(req canonical.Request, target CanonicalTarget) CanonicalTarget {
	if target.WireFormat != WireDashScopeText && target.WireFormat != WireDashScopeMultimodal {
		return target
	}
	if target.IncrementalOutputMode != IncrementalAuto {
		return target
	}
	enableThinking, _ := resolveQwenThinking(req)
	if resolveQwenIncrementalOutput(IncrementalAuto, qwenHasDeclaredWireTools(req), enableThinking) {
		target.IncrementalOutputMode = IncrementalForceTrue
	} else {
		target.IncrementalOutputMode = IncrementalForceFalse
	}
	return target
}

func encodeQwenCanonicalRequest(req canonical.Request, target CanonicalTarget) ([]byte, error) {
	if target.WireFormat != WireDashScopeText && target.WireFormat != WireDashScopeMultimodal {
		return nil, canonical.NewRejectionError("unsupported_wire_format", string(target.WireFormat), "expected dashscope_text or dashscope_multimodal")
	}
	multimodal := target.WireFormat == WireDashScopeMultimodal

	if req.PreviousResponseID.IsPresent() && req.PreviousResponseID.Val != "" {
		return nil, canonical.NewRejectionError("unsupported_stateful_reference", "previous_response_id", "previous_response_id is unsupported on DashScope")
	}
	if req.ConversationPresent {
		return nil, canonical.NewRejectionError("unsupported_stateful_reference", "conversation", "conversation is unsupported on DashScope")
	}
	if req.Background {
		return nil, canonical.NewRejectionError("unsupported_background_mode", "background", "background execution is unsupported on DashScope")
	}

	model := target.Model
	if model == "" {
		model = req.Model
	}
	streaming := req.Stream || target.Streaming

	out := qwenWireOutRequest{
		Model: model,
		Parameters: qwenWireOutParameters{
			ResultFormat: qwenResultFormatMessage,
		},
	}

	// R02 / DS01: Output token budget mapping according to frozen target capability.
	if req.Limits.Value.IsPresent() {
		limitField := "max_tokens"
		switch req.Limits.Source {
		case canonical.LimitMaxCompletionTokens:
			limitField = "max_completion_tokens"
		case canonical.LimitMaxOutputTokens:
			limitField = "max_output_tokens"
		}
		v := req.Limits.Value.Val
		if v < 0 {
			return nil, canonical.NewRejectionError("invalid_output_limit", limitField, "output token limit must be non-negative")
		}
		if v == 0 {
			return nil, canonical.NewRejectionError("unsupported_zero_output_tokens", limitField, "zero output token warmup is unsupported on DashScope")
		}
		if target.AcceptedLimitScopes != LimitScopeMaskNone && !target.AcceptedLimitScopes.Allows(req.Limits.Scope) {
			return nil, canonical.NewRejectionError("unsupported_output_limit_scope", limitField, "output token limit scope is unsupported on target")
		}
		if target.SupportsCompletionTokens && (req.Limits.Source == canonical.LimitMaxCompletionTokens || req.Limits.Source == canonical.LimitMaxOutputTokens || req.Limits.Scope == canonical.ScopeReasoningAndVisible) {
			out.Parameters.MaxCompletionTokens = &v
		} else {
			out.Parameters.MaxTokens = &v
		}
	}

	// R03b / DEF-01 / S-BOUND-01: Sampling scalars with pointer zero preservation and top_p==1.0 clamp.
	if req.Sampling.Temperature.IsPresent() {
		v := req.Sampling.Temperature.Val
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return nil, canonical.NewRejectionError("invalid_sampling_parameter", "temperature", "temperature must be non-negative")
		}
		out.Parameters.Temperature = &v
	}
	if req.Sampling.TopP.IsPresent() {
		v := req.Sampling.TopP.Val
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
			return nil, canonical.NewRejectionError("invalid_sampling_parameter", "top_p", "top_p must be within [0, 1]")
		}
		if v > qwenTopPMax {
			v = qwenTopPMax
		}
		out.Parameters.TopP = &v
	}
	if req.Sampling.TopK.IsPresent() {
		v := req.Sampling.TopK.Val
		if v < 0 {
			return nil, canonical.NewRejectionError("invalid_sampling_parameter", "top_k", "top_k must be non-negative")
		}
		out.Parameters.TopK = &v
	}
	if req.Sampling.PresencePenalty.IsPresent() {
		v := req.Sampling.PresencePenalty.Val
		out.Parameters.PresencePenalty = &v
	}
	if req.Sampling.RepetitionPenalty.IsPresent() {
		v := req.Sampling.RepetitionPenalty.Val
		out.Parameters.RepetitionPenalty = &v
	}
	if req.Sampling.Seed.IsPresent() {
		v := req.Sampling.Seed.Val
		out.Parameters.Seed = &v
	}
	if req.Sampling.N.IsPresent() {
		v := req.Sampling.N.Val
		if v <= 0 {
			return nil, canonical.NewRejectionError("invalid_sampling_parameter", "n", "n must be at least 1")
		}
		out.Parameters.N = &v
	}
	if req.Sampling.StopPresent {
		if len(req.Sampling.Stop) > 4 {
			return nil, canonical.NewRejectionError("invalid_stop", "stop", "stop sequences exceed maximum of 4")
		}
		if len(req.Sampling.Stop) > 0 {
			out.Parameters.Stop = slices.Clone(req.Sampling.Stop)
		}
	}
	if req.Sampling.Logprobs.IsPresent() {
		v := req.Sampling.Logprobs.Val
		out.Parameters.Logprobs = &v
	}
	if req.Sampling.TopLogprobs.IsPresent() && req.SourceProtocol == canonical.ProtocolOpenAIChat {
		v := req.Sampling.TopLogprobs.Val
		out.Parameters.TopLogprobs = &v
	}

	// DS05: Structured output format without schema mutation or float64 loss.
	switch req.ResponseFormat.Kind {
	case canonical.ResponseFormatUnset:
	case canonical.ResponseFormatText:
		out.Parameters.ResponseFormat = &chatWireOutResponseFormat{Type: "text"}
	case canonical.ResponseFormatJSONObject:
		out.Parameters.ResponseFormat = &chatWireOutResponseFormat{Type: "json_object"}
	case canonical.ResponseFormatJSONSchema:
		trimmedSchema := bytes.TrimSpace(req.ResponseFormat.Schema)
		if len(trimmedSchema) == 0 || bytes.Equal(trimmedSchema, []byte("null")) {
			return nil, canonical.NewRejectionError("invalid_response_format", "response_format", "json_schema requires a non-empty schema")
		}
		if !target.SupportsStrictSchema {
			return nil, canonical.NewRejectionError("unsupported_strict_schema", "response_format", "json_schema structured output is unsupported on target")
		}
		var strictPtr *bool
		if req.ResponseFormat.Strict.IsPresent() {
			v := req.ResponseFormat.Strict.Val
			strictPtr = &v
		}
		out.Parameters.ResponseFormat = &chatWireOutResponseFormat{
			Type: "json_schema",
			JSONSchema: &chatWireOutJSONSchema{
				Name:        req.ResponseFormat.Name,
				Description: req.ResponseFormat.Description,
				Schema:      bytes.Clone(req.ResponseFormat.Schema),
				Strict:      strictPtr,
			},
		}
	default:
		return nil, canonical.NewRejectionError("unsupported_response_format", "response_format", "unsupported response_format")
	}

	enableThinking, budgetPtr := resolveQwenThinking(req)
	out.Parameters.EnableThinking = enableThinking
	out.Parameters.ThinkingBudget = budgetPtr

	// Tools & FixedTools (preserving explicit false to suppress QwenEnableSearchDefault).
	hasSearchTool := false
	if len(req.Tools) > 0 {
		out.Parameters.Tools = make([]json.RawMessage, 0, len(req.Tools))
		for _, t := range req.Tools {
			switch t.Kind {
			case canonical.ToolKindFunction:
				if t.Strict.IsPresent() && t.Strict.Val && !target.SupportsStrictSchema {
					return nil, canonical.NewRejectionError("unsupported_strict_schema", "tools", "strict tool schema is unsupported on target")
				}
				var strictPtr *bool
				if t.Strict.IsPresent() {
					v := t.Strict.Val
					strictPtr = &v
				}
				encodedTool, err := json.Marshal(chatWireOutTool{
					Type: "function",
					Function: &chatWireOutFunctionDef{
						Name:        t.Name,
						Description: t.Description,
						Parameters:  bytes.Clone(t.Schema),
						Strict:      strictPtr,
					},
				})
				if err != nil {
					return nil, fmt.Errorf("marshal DashScope function tool: %w", err)
				}
				out.Parameters.Tools = append(out.Parameters.Tools, encodedTool)
			case canonical.ToolKindFixedServer:
				if fk, ok := fixedtools.Kind(t.FixedType); ok && fk == fixedtools.WebSearch {
					hasSearchTool = true
				}
				if len(t.RawFixed) > 0 {
					rawFixed := bytes.Clone(t.RawFixed)
					var fixedMap map[string]json.RawMessage
					if json.Unmarshal(rawFixed, &fixedMap) == nil && fixedMap != nil {
						if _, hasCC := fixedMap["cache_control"]; hasCC {
							delete(fixedMap, "cache_control")
							if cleaned, err := json.Marshal(fixedMap); err == nil {
								rawFixed = cleaned
							}
						}
					}
					out.Parameters.Tools = append(out.Parameters.Tools, rawFixed)
				} else if t.FixedType != "" {
					encodedFixed, err := json.Marshal(struct {
						Type string `json:"type"`
					}{Type: t.FixedType})
					if err != nil {
						return nil, fmt.Errorf("marshal DashScope fixed tool: %w", err)
					}
					out.Parameters.Tools = append(out.Parameters.Tools, encodedFixed)
				}
			}
		}
	}

	if req.FixedTools.EnableSearch.IsPresent() {
		v := req.FixedTools.EnableSearch.Val
		out.Parameters.EnableSearch = &v
	} else if target.QwenEnableSearchDefault || hasSearchTool {
		v := true
		out.Parameters.EnableSearch = &v
	}

	if req.FixedTools.EnableCodeInterpreter.IsPresent() {
		v := req.FixedTools.EnableCodeInterpreter.Val
		out.Parameters.EnableCodeInterpreter = &v
	}
	if len(req.FixedTools.SearchOptionsRaw) > 0 {
		out.Parameters.SearchOptions = bytes.Clone(req.FixedTools.SearchOptionsRaw)
	} else if req.FixedTools.SearchStrategy.IsPresent() && req.FixedTools.SearchStrategy.Val != "" {
		rawOpts, err := json.Marshal(struct {
			Strategy string `json:"strategy"`
		}{Strategy: req.FixedTools.SearchStrategy.Val})
		if err != nil {
			return nil, fmt.Errorf("marshal DashScope search_options: %w", err)
		}
		out.Parameters.SearchOptions = rawOpts
	}

	// DS02: Forced tool_choice validation & conflict guard with active thinking.
	forcedToolChoice := req.ToolChoice.Mode == canonical.ToolChoiceRequired ||
		req.ToolChoice.Mode == canonical.ToolChoiceAny ||
		req.ToolChoice.Mode == canonical.ToolChoiceFunction ||
		req.ToolChoice.Mode == canonical.ToolChoiceFixed
	if forcedToolChoice {
		if len(out.Parameters.Tools) == 0 {
			return nil, canonical.NewRejectionError("invalid_tool_choice", "tool_choice", "forced tool_choice requires declared tools in DashScope mode")
		}
		if enableThinking != nil && *enableThinking {
			return nil, canonical.NewRejectionError("conflicting_thinking_tool_choice", "tool_choice", "forced tool_choice cannot be combined with active thinking in DashScope mode")
		}
	}

	switch req.ToolChoice.Mode {
	case canonical.ToolChoiceUnset:
	case canonical.ToolChoiceAuto:
		out.Parameters.ToolChoice = json.RawMessage(`"auto"`)
	case canonical.ToolChoiceNone:
		out.Parameters.ToolChoice = json.RawMessage(`"none"`)
	case canonical.ToolChoiceRequired, canonical.ToolChoiceAny:
		out.Parameters.ToolChoice = json.RawMessage(`"required"`)
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
			return nil, fmt.Errorf("marshal DashScope tool_choice: %w", err)
		}
		out.Parameters.ToolChoice = encodedChoice
	case canonical.ToolChoiceFixed:
		fixedName := req.ToolChoice.FixedRaw
		if fixedName == "" {
			fixedName = req.ToolChoice.Name
		}
		encodedChoice, err := json.Marshal(struct {
			Type string `json:"type"`
		}{Type: fixedName})
		if err != nil {
			return nil, fmt.Errorf("marshal DashScope fixed tool_choice: %w", err)
		}
		out.Parameters.ToolChoice = encodedChoice
	}

	if req.ParallelToolCalls.IsPresent() {
		v := req.ParallelToolCalls.Val
		out.Parameters.ParallelToolCalls = &v
	}

	// R05 / DS03: Explicit incremental_output aligned with BindQwenIncrementalMode & 2.0.91 rule.
	if streaming {
		bound := BindQwenIncrementalMode(req, target)
		inc := bound.IncrementalOutputMode == IncrementalForceTrue
		out.Parameters.IncrementalOutput = &inc
	}

	// Build input.messages directly from canonical IR with zero intermediate Chat JSON.
	messages := make([]qwenWireOutMessage, 0, len(req.Items)+2)

	if req.Instructions.IsPresent() && req.Instructions.Val != "" {
		sysMsg, err := makeQwenPlainSystemMessage(req.Instructions.Val, multimodal)
		if err != nil {
			return nil, fmt.Errorf("marshal DashScope instructions: %w", err)
		}
		messages = append(messages, sysMsg)
	}
	if len(req.SystemParts) > 0 {
		// In WireDashScopeText, plain system content flattens to string unless part-level cache_control is present.
		sysContent, err := encodeQwenPartsJSON(req.SystemParts, multimodal, false, "", false)
		if err != nil {
			return nil, err
		}
		messages = append(messages, qwenWireOutMessage{
			Role:    roleSystem,
			Content: sysContent,
		})
	}

	hasEmittedMessageReasoning := false

	for i := 0; i < len(req.Items); {
		item := req.Items[i]

		isAssistantItem := item.Kind == canonical.ItemReasoning ||
			item.Kind == canonical.ItemFunctionCall ||
			(item.Kind == canonical.ItemMessage && item.Role == canonical.RoleAssistant)
		if isAssistantItem {
			var reasoningChunks []string
			hadMessagesReasoning := false
			var segments []chatAssistantSegment
			hasBareEmptyAssistant := false
			var partialPtr *bool
			var toolCalls []qwenWireOutToolCall

			for i < len(req.Items) {
				cur := req.Items[i]
				if cur.Kind == canonical.ItemReasoning {
					if cur.Reasoning != nil {
						if cur.Reasoning.Cache != nil {
							return nil, canonical.NewRejectionError("invalid_cache_control", "thinking", "thinking blocks cannot carry cache_control")
						}
						if cur.Reasoning.Origin.Protocol == canonical.ProtocolAnthropicMessages {
							hadMessagesReasoning = true
						}
						if len(cur.Reasoning.Text) > 0 {
							reasoningChunks = append(reasoningChunks, cur.Reasoning.Text...)
						}
					}
					i++
					continue
				}
				if cur.Kind == canonical.ItemFunctionCall {
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
						toolCalls = append(toolCalls, qwenWireOutToolCall{
							Index: len(toolCalls),
							ID:    cur.Call.CallID,
							Type:  "function",
							Function: chatWireFunctionCall{
								Name:      cur.Call.Name,
								Arguments: argsStr,
							},
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
						v := cur.Partial.Val
						partialPtr = &v
					}
					if len(cur.Parts) == 0 {
						hasBareEmptyAssistant = true
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
					messages = append(messages, qwenWireOutMessage{
						Role:    roleAssistant,
						Content: json.RawMessage(`""`),
						Partial: partialPtr,
					})
				}
				continue
			}

			outMsg := qwenWireOutMessage{
				Role:             roleAssistant,
				Partial:          partialPtr,
				ReasoningContent: reasoningPtr,
				ToolCalls:        toolCalls,
			}

			if len(segments) == 0 {
				outMsg.Content = json.RawMessage(`""`)
			} else if len(segments) == 1 {
				seg := segments[0]
				forceArr := req.SourceProtocol != canonical.ProtocolOpenAIResponses && seg.messageIsArray && len(toolCalls) == 0
				joinSep := ""
				if req.SourceProtocol == canonical.ProtocolAnthropicMessages && len(toolCalls) > 0 {
					joinSep = "\n\n"
				}
				encodedContent, err := encodeQwenPartsJSON(seg.parts, false, forceArr, joinSep, false)
				if err != nil {
					if !multimodal {
						return nil, err
					}
					encodedContent, err = encodeQwenPartsJSON(seg.parts, true, forceArr, joinSep, false)
					if err != nil {
						return nil, err
					}
				}
				outMsg.Content = encodedContent
			} else {
				var mergedParts []canonical.Part
				for _, seg := range segments {
					for _, p := range seg.parts {
						if p.Kind == canonical.PartRefusal {
							return nil, canonical.NewRejectionError("incompatible_refusal_merge", "input.content", "cannot combine refusal and other assistant content parts")
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
				encodedContent, err := encodeQwenPartsJSON(mergedParts, multimodal, true, "", false)
				if err != nil {
					return nil, err
				}
				outMsg.Content = encodedContent
			}

			messages = append(messages, outMsg)
			continue
		}

		if item.Kind == canonical.ItemFunctionResult {
			var companionImages []canonical.Part
			for i < len(req.Items) && req.Items[i].Kind == canonical.ItemFunctionResult {
				res := req.Items[i].Result
				i++
				if res == nil {
					continue
				}
				var textParts []string
				for _, p := range res.Parts {
					if p.Kind == canonical.PartImage {
						if req.SourceProtocol != canonical.ProtocolAnthropicMessages {
							return nil, canonical.NewRejectionError("unsupported_image_content", "function_call_output", "multimodal function_call_output is unsupported")
						}
						if !multimodal {
							return nil, canonical.NewRejectionError("unsupported_image_content", "tool_result", "dashscope_text does not support image inputs")
						}
						companionImages = append(companionImages, p)
					} else if p.Kind == canonical.PartText {
						textParts = append(textParts, p.Text)
					}
				}
				var joinedText string
				if res.IsStringForm && len(textParts) == 1 {
					joinedText = textParts[0]
				} else if res.IsStringForm {
					joinedText = strings.Join(textParts, "")
				} else {
					var nonEmpty []string
					for _, t := range textParts {
						if t != "" || len(textParts) == 1 {
							nonEmpty = append(nonEmpty, t)
						}
					}
					joinedText = strings.Join(nonEmpty, "\n")
				}
				toolContent, err := json.Marshal(joinedText)
				if err != nil {
					return nil, fmt.Errorf("marshal DashScope tool result: %w", err)
				}
				// Never write tool.name on DashScope tool messages.
				messages = append(messages, qwenWireOutMessage{
					Role:       roleTool,
					Content:    toolContent,
					ToolCallID: res.CallID,
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
				companionJSON, err := encodeQwenPartsJSON(companionParts, true, true, "", true)
				if err != nil {
					return nil, err
				}
				messages = append(messages, qwenWireOutMessage{
					Role:    roleUser,
					Content: companionJSON,
				})
			}
			continue
		}

		// system, developer (-> system), user
		outRole := string(item.Role)
		if item.Role == canonical.RoleDeveloper {
			outRole = roleSystem
		}
		forceArr := false
		if outRole != roleSystem {
			forceArr = req.SourceProtocol != canonical.ProtocolOpenAIResponses && item.MessageIsArray
		}
		contentJSON, err := encodeQwenPartsJSON(item.Parts, multimodal, forceArr, "", false)
		if err != nil {
			return nil, err
		}
		messages = append(messages, qwenWireOutMessage{
			Role:    outRole,
			Content: contentJSON,
		})
		i++
	}

	if len(target.QwenLongFileIDs) > 0 && (model == qwenLongModelName || strings.HasPrefix(model, qwenLongModelName+"-")) {
		var err error
		messages, err = applyQwenLongFileIDs(messages, target.QwenLongFileIDs, multimodal)
		if err != nil {
			return nil, err
		}
	}
	out.Input.Messages = messages

	// DS06 / S-BOUND-04: parameters.preserve_thinking on DashScope native targets.
	supportsPreserve := target.SupportsPreserveThinking || qwenSupportsPreserveThinking(model)
	if req.Thinking.PreserveThinking.IsPresent() {
		v := req.Thinking.PreserveThinking.Val
		if !v || (supportsPreserve && req.Thinking.Mode != canonical.ThinkingDisabled) {
			out.Parameters.PreserveThinking = &v
		}
	} else if supportsPreserve && hasEmittedMessageReasoning && req.Thinking.Mode != canonical.ThinkingDisabled {
		v := true
		out.Parameters.PreserveThinking = &v
	}

	encoded, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("marshal canonical DashScope request: %w", err)
	}
	return encoded, nil
}

// --- DashScope Native Response & Per-Attempt SSE Event Decoders ---

type qwenWireRespToolCall struct {
	Index    int                  `json:"index"`
	ID       string               `json:"id"`
	Type     string               `json:"type"`
	Function chatWireFunctionCall `json:"function"`
}

type qwenWireRespContentPart struct {
	Text  wireOptional[string] `json:"text"`
	Image string               `json:"image"`
}

type qwenWireRespMessage struct {
	Name             string                 `json:"name"`
	Role             string                 `json:"role"`
	Content          json.RawMessage        `json:"content"`
	ReasoningContent wireOptional[string]   `json:"reasoning_content"`
	ToolCalls        []qwenWireRespToolCall `json:"tool_calls"`
}

type qwenWireRespChoice struct {
	Index        *int                 `json:"index"`
	FinishReason *string              `json:"finish_reason"`
	Message      *qwenWireRespMessage `json:"message"`
}

type qwenWireRespOutput struct {
	Text         wireOptional[string] `json:"text"`
	FinishReason *string              `json:"finish_reason"`
	Choices      []qwenWireRespChoice `json:"choices"`
}

type qwenWireRespEnvelope struct {
	StatusCode int                 `json:"status_code"`
	RequestID  string              `json:"request_id"`
	Code       string              `json:"code"`
	Message    string              `json:"message"`
	Model      string              `json:"model"`
	Output     *qwenWireRespOutput `json:"output"`
}

func classifyQwenFinishReason(raw string) canonical.FinishReason {
	return classifyChatFinishReason(raw)
}

func qwenCanonicalProtocol(wf WireFormat) canonical.Protocol {
	if wf == WireDashScopeMultimodal {
		return canonical.ProtocolDashScopeMultimodal
	}
	return canonical.ProtocolDashScopeText
}

func decodeQwenResponseParts(raw json.RawMessage) ([]canonical.Part, bool, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, false, nil
	}
	switch trimmed[0] {
	case '"':
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return nil, false, fmt.Errorf("unmarshal DashScope message content string: %w", err)
		}
		return []canonical.Part{{Kind: canonical.PartText, Text: s}}, false, nil
	case '[':
		var wireParts []qwenWireRespContentPart
		if err := json.Unmarshal(trimmed, &wireParts); err != nil {
			return nil, false, fmt.Errorf("unmarshal DashScope message content array: %w", err)
		}
		parts := make([]canonical.Part, 0, len(wireParts))
		for _, wp := range wireParts {
			if wp.Image != "" {
				imgPart, err := parseCanonicalImageURL(wp.Image, canonical.ImageDetailUnset, "image")
				if err != nil {
					imgPart = &canonical.ImagePart{
						Source: canonical.ImageURL,
						URL:    wp.Image,
					}
				}
				parts = append(parts, canonical.Part{
					Kind:  canonical.PartImage,
					Image: imgPart,
				})
				continue
			}
			if wp.Text.state == canonical.Present {
				parts = append(parts, canonical.Part{
					Kind: canonical.PartText,
					Text: wp.Text.val,
				})
			}
		}
		return parts, true, nil
	default:
		return nil, false, fmt.Errorf("unsupported DashScope message content shape")
	}
}

func decodeQwenCanonicalResponse(target CanonicalTarget, body []byte) (canonical.Response, error) {
	if target.WireFormat != WireDashScopeText && target.WireFormat != WireDashScopeMultimodal {
		return canonical.Response{}, canonical.NewRejectionError("unsupported_wire_format", string(target.WireFormat), "expected dashscope_text or dashscope_multimodal")
	}

	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return canonical.Response{}, fmt.Errorf("invalid DashScope response JSON")
	}

	var env qwenWireRespEnvelope
	if err := json.Unmarshal(trimmed, &env); err != nil {
		return canonical.Response{}, fmt.Errorf("unmarshal DashScope response: %w", err)
	}
	if (env.Code != "" && env.Code != "200") || env.StatusCode >= 400 {
		return canonical.Response{}, fmt.Errorf("upstream DashScope error payload")
	}

	model := env.Model
	if model == "" {
		model = target.Model
	}
	resp := canonical.Response{
		ID:    env.RequestID,
		Model: model,
	}
	if env.Output == nil {
		return resp, nil
	}

	wireChoices := env.Output.Choices
	if len(wireChoices) == 0 && (env.Output.Text.state == canonical.Present || env.Output.FinishReason != nil) {
		var textRaw json.RawMessage
		if env.Output.Text.state == canonical.Present {
			encodedText, err := json.Marshal(env.Output.Text.val)
			if err != nil {
				return canonical.Response{}, err
			}
			textRaw = encodedText
		}
		wireChoices = []qwenWireRespChoice{{
			FinishReason: env.Output.FinishReason,
			Message: &qwenWireRespMessage{
				Role:    roleAssistant,
				Content: textRaw,
			},
		}}
	}

	originProto := qwenCanonicalProtocol(target.WireFormat)
	resp.Choices = make([]canonical.Choice, 0, len(wireChoices))

	for ordinal, wc := range wireChoices {
		cIdx := ordinal
		if wc.Index != nil {
			cIdx = *wc.Index
		}
		var finish canonical.FinishReason
		if wc.FinishReason != nil {
			finish = classifyQwenFinishReason(*wc.FinishReason)
		} else if len(wireChoices) == 1 && env.Output.FinishReason != nil {
			finish = classifyQwenFinishReason(*env.Output.FinishReason)
		}

		choice := canonical.Choice{
			Index:  cIdx,
			Finish: finish,
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
		hasReasoning := reasoningText != ""
		if hasReasoning {
			choice.Items = append(choice.Items, canonical.Item{
				Kind: canonical.ItemReasoning,
				Role: role,
				Reasoning: &canonical.ReasoningItem{
					Text:   []string{reasoningText},
					Origin: canonical.OpaqueOrigin{Protocol: originProto, Supplier: "qwen"},
				},
			})
		}

		parts, isArr, err := decodeQwenResponseParts(msg.Content)
		if err != nil {
			return canonical.Response{}, err
		}
		if len(parts) > 0 {
			if !isArr && len(parts) == 1 && parts[0].Kind == canonical.PartText && parts[0].Text == "" {
				if len(msg.ToolCalls) == 0 && !hasReasoning {
					choice.Items = append(choice.Items, canonical.Item{
						Kind:  canonical.ItemMessage,
						Role:  role,
						Parts: parts,
					})
				}
			} else {
				nonEmptyParts := parts
				if isArr && (len(msg.ToolCalls) > 0 || hasReasoning) {
					filtered := make([]canonical.Part, 0, len(parts))
					for _, p := range parts {
						if p.Kind != canonical.PartText || p.Text != "" {
							filtered = append(filtered, p)
						}
					}
					nonEmptyParts = filtered
				}
				if len(nonEmptyParts) > 0 {
					choice.Items = append(choice.Items, canonical.Item{
						Kind:           canonical.ItemMessage,
						Role:           role,
						Parts:          nonEmptyParts,
						MessageIsArray: isArr,
					})
				}
			}
		}

		for _, tc := range msg.ToolCalls {
			if tc.ID == "" && tc.Function.Name == "" && tc.Function.Arguments == "" {
				continue
			}
			choice.Items = append(choice.Items, canonical.Item{
				Kind: canonical.ItemFunctionCall,
				Role: role,
				Call: &canonical.FunctionCall{
					CallID: tc.ID,
					Name:   tc.Function.Name,
					Args:   canonical.Arguments{Kind: canonical.ArgsText, Text: tc.Function.Arguments},
				},
			})
		}

		resp.Choices = append(resp.Choices, choice)
	}

	return resp, nil
}

type qwenStreamToolState struct {
	itemIndex   int
	callID      string
	name        string
	pushedID    string
	pushedName  string
	pushedArgs  string
	started     bool
	pendingArgs strings.Builder
}

type qwenStreamChoiceState struct {
	role             canonical.Role
	nextItemIndex    int
	hasReasoningItem bool
	reasoningItemIdx int
	hasMessageItem   bool
	messageItemIdx   int
	pushedText       string
	pushedReasoning  string
	tools            map[int]*qwenStreamToolState
}

func (cs *qwenStreamChoiceState) allocItemIndex() int {
	idx := cs.nextItemIndex
	cs.nextItemIndex++
	return idx
}

type qwenCanonicalEventDecoder struct {
	incremental bool
	started     bool
	requestID   string
	model       string
	choices     map[int]*qwenStreamChoiceState
}

func newQwenCanonicalEventDecoder(target CanonicalTarget) (CanonicalEventDecoder, error) {
	if target.WireFormat != WireDashScopeText && target.WireFormat != WireDashScopeMultimodal {
		return nil, canonical.NewRejectionError("unsupported_wire_format", string(target.WireFormat), "expected dashscope_text or dashscope_multimodal")
	}
	incremental := true
	if target.IncrementalOutputMode == IncrementalForceFalse {
		incremental = false
	}
	return &qwenCanonicalEventDecoder{
		incremental: incremental,
		model:       target.Model,
		choices:     make(map[int]*qwenStreamChoiceState),
	}, nil
}

// diffCumulativeUTF8 computes the exact UTF-8 suffix added to pushed.
// On empty frames or non-prefix rollbacks/divergences, it emits "" and keeps
// pushed unchanged so previously sent text is neither duplicated nor retracted.
func diffCumulativeUTF8(current, pushed string) (string, string) {
	if current == "" {
		return "", pushed
	}
	if pushed == "" {
		return current, current
	}
	if strings.HasPrefix(current, pushed) {
		if len(current) == len(pushed) {
			return "", pushed
		}
		if utf8.RuneStart(current[len(pushed)]) {
			return current[len(pushed):], current
		}
	}
	return "", pushed
}

func (d *qwenCanonicalEventDecoder) ensureStarted(role canonical.Role, emit func(canonical.Event) error) error {
	if d.started {
		return nil
	}
	d.started = true
	if role == "" {
		role = canonical.RoleAssistant
	}
	return emit(canonical.Event{
		Kind:  canonical.EventResponseStart,
		ID:    d.requestID,
		Model: d.model,
		Role:  role,
	})
}

func (d *qwenCanonicalEventDecoder) flushPendingIncrementalTools(cIdx int, cs *qwenStreamChoiceState, emit func(canonical.Event) error) error {
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
		if err := d.ensureStarted(cs.role, emit); err != nil {
			return err
		}
		if err := emit(canonical.Event{
			Kind:        canonical.EventToolCallStart,
			ChoiceIndex: cIdx,
			ItemIndex:   ts.itemIndex,
			ToolIndex:   tIdx,
			CallID:      ts.callID,
			ID:          d.requestID,
			Model:       d.model,
			Role:        cs.role,
			ToolName:    ts.name,
		}); err != nil {
			return err
		}
		if ts.pendingArgs.Len() > 0 {
			buffered := ts.pendingArgs.String()
			ts.pendingArgs.Reset()
			if err := emit(canonical.Event{
				Kind:        canonical.EventToolCallArgsDelta,
				ChoiceIndex: cIdx,
				ItemIndex:   ts.itemIndex,
				ToolIndex:   tIdx,
				CallID:      ts.callID,
				ID:          d.requestID,
				Model:       d.model,
				Role:        cs.role,
				ToolName:    ts.name,
				Text:        buffered,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *qwenCanonicalEventDecoder) Decode(name string, payload []byte, emit func(canonical.Event) error) error {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		if name == "ping" {
			return emit(canonical.Event{Kind: canonical.EventPing})
		}
		return nil
	}
	if bytes.Equal(trimmed, []byte(streamEndDataValue)) {
		for cIdx, cs := range d.choices {
			if err := d.flushPendingIncrementalTools(cIdx, cs, emit); err != nil {
				return err
			}
		}
		return emit(canonical.Event{
			Kind:  canonical.EventProtocolDone,
			ID:    d.requestID,
			Model: d.model,
		})
	}

	var env qwenWireRespEnvelope
	if err := json.Unmarshal(trimmed, &env); err != nil {
		if name == "error" {
			return emit(canonical.Event{
				Kind:     canonical.EventProviderError,
				ID:       d.requestID,
				Model:    d.model,
				RawError: bytes.Clone(trimmed),
			})
		}
		return fmt.Errorf("unmarshal DashScope stream event: %w", err)
	}

	if env.RequestID != "" {
		d.requestID = env.RequestID
	}
	if env.Model != "" {
		d.model = env.Model
	}

	if name == "error" || (env.Code != "" && env.Code != "200") || env.StatusCode >= 400 {
		return emit(canonical.Event{
			Kind:     canonical.EventProviderError,
			ID:       d.requestID,
			Model:    d.model,
			RawError: bytes.Clone(trimmed),
		})
	}

	if env.Output == nil {
		return nil
	}

	wireChoices := env.Output.Choices
	if len(wireChoices) == 0 && (env.Output.Text.state == canonical.Present || env.Output.FinishReason != nil) {
		var textRaw json.RawMessage
		if env.Output.Text.state == canonical.Present {
			encodedText, err := json.Marshal(env.Output.Text.val)
			if err != nil {
				return err
			}
			textRaw = encodedText
		}
		wireChoices = []qwenWireRespChoice{{
			FinishReason: env.Output.FinishReason,
			Message: &qwenWireRespMessage{
				Role:    roleAssistant,
				Content: textRaw,
			},
		}}
	}
	if len(wireChoices) == 0 {
		return nil
	}

	for ordinal, wc := range wireChoices {
		cIdx := ordinal
		if wc.Index != nil {
			cIdx = *wc.Index
		}
		cs, exists := d.choices[cIdx]
		if !exists {
			cs = &qwenStreamChoiceState{
				role:  canonical.RoleAssistant,
				tools: make(map[int]*qwenStreamToolState),
			}
			d.choices[cIdx] = cs
		}

		var finish canonical.FinishReason
		if wc.FinishReason != nil {
			finish = classifyQwenFinishReason(*wc.FinishReason)
		} else if len(wireChoices) == 1 && env.Output.FinishReason != nil {
			finish = classifyQwenFinishReason(*env.Output.FinishReason)
		}
		finished := finish.Category != canonical.FinishNone

		if wc.Message != nil {
			msg := wc.Message
			if msg.Role != "" {
				cs.role = canonical.Role(msg.Role)
			}

			currentReasoning := msg.ReasoningContent.toCanonical().ValueOr("")
			currentText := extractQwenMessageText(msg.Content)

			calls := make([]qwenWireRespToolCall, len(msg.ToolCalls))
			copy(calls, msg.ToolCalls)
			for ti := range calls {
				if calls[ti].Index == 0 && ti > 0 {
					calls[ti].Index = ti
				}
			}

			if d.incremental {
				if currentReasoning != "" {
					if !cs.hasReasoningItem {
						cs.hasReasoningItem = true
						cs.reasoningItemIdx = cs.allocItemIndex()
					}
					if err := d.ensureStarted(cs.role, emit); err != nil {
						return err
					}
					if err := emit(canonical.Event{
						Kind:        canonical.EventReasoningDelta,
						ChoiceIndex: cIdx,
						ItemIndex:   cs.reasoningItemIdx,
						PartIndex:   0,
						ID:          d.requestID,
						Model:       d.model,
						Role:        cs.role,
						Text:        currentReasoning,
					}); err != nil {
						return err
					}
				}

				if currentText != "" {
					if !cs.hasMessageItem {
						cs.hasMessageItem = true
						cs.messageItemIdx = cs.allocItemIndex()
					}
					if err := d.ensureStarted(cs.role, emit); err != nil {
						return err
					}
					if err := emit(canonical.Event{
						Kind:        canonical.EventTextDelta,
						ChoiceIndex: cIdx,
						ItemIndex:   cs.messageItemIdx,
						PartIndex:   0,
						ID:          d.requestID,
						Model:       d.model,
						Role:        cs.role,
						Text:        currentText,
					}); err != nil {
						return err
					}
				}

				for _, tc := range calls {
					tIdx := tc.Index
					ts, ok := cs.tools[tIdx]
					if !ok {
						ts = &qwenStreamToolState{
							itemIndex: cs.allocItemIndex(),
						}
						cs.tools[tIdx] = ts
					}

					// 2.0.91 qwenPushedIncrementalToolCalls deduplication on per-attempt decoder state:
					// suppress repeated id/name on subsequent frames and drop empty function:{} tail frames.
					dedupID := tc.ID
					dedupName := tc.Function.Name
					if ts.pushedID != "" || ts.pushedName != "" {
						if ts.pushedID != "" && dedupID == ts.pushedID {
							dedupID = ""
							if ts.pushedName != "" && dedupName == ts.pushedName {
								dedupName = ""
							}
						}
						if dedupID != "" {
							ts.pushedID = dedupID
						}
						if dedupName != "" {
							ts.pushedName = dedupName
						}
					} else if dedupID != "" || dedupName != "" {
						ts.pushedID = dedupID
						ts.pushedName = dedupName
					}

					if dedupID != "" {
						ts.callID = dedupID
					}
					if dedupName != "" {
						if !ts.started {
							ts.name += dedupName
						} else if dedupName != ts.name {
							ts.name += dedupName
						}
					}

					if dedupName == "" && tc.Function.Arguments == "" {
						continue
					}

					if !ts.started {
						readyToStart := ts.name != ""
						if readyToStart {
							ts.started = true
							if err := d.ensureStarted(cs.role, emit); err != nil {
								return err
							}
							if err := emit(canonical.Event{
								Kind:        canonical.EventToolCallStart,
								ChoiceIndex: cIdx,
								ItemIndex:   ts.itemIndex,
								ToolIndex:   tIdx,
								CallID:      ts.callID,
								ID:          d.requestID,
								Model:       d.model,
								Role:        cs.role,
								ToolName:    ts.name,
							}); err != nil {
								return err
							}
							if ts.pendingArgs.Len() > 0 {
								buffered := ts.pendingArgs.String()
								ts.pendingArgs.Reset()
								if err := emit(canonical.Event{
									Kind:        canonical.EventToolCallArgsDelta,
									ChoiceIndex: cIdx,
									ItemIndex:   ts.itemIndex,
									ToolIndex:   tIdx,
									CallID:      ts.callID,
									ID:          d.requestID,
									Model:       d.model,
									Role:        cs.role,
									ToolName:    ts.name,
									Text:        buffered,
								}); err != nil {
									return err
								}
							}
						} else if tc.Function.Arguments != "" {
							ts.pendingArgs.WriteString(tc.Function.Arguments)
							continue
						}
					}

					if ts.started && tc.Function.Arguments != "" {
						if err := emit(canonical.Event{
							Kind:        canonical.EventToolCallArgsDelta,
							ChoiceIndex: cIdx,
							ItemIndex:   ts.itemIndex,
							ToolIndex:   tIdx,
							CallID:      ts.callID,
							ID:          d.requestID,
							Model:       d.model,
							Role:        cs.role,
							ToolName:    ts.name,
							Text:        tc.Function.Arguments,
						}); err != nil {
							return err
						}
					}
				}
			} else {
				// Cumulative mode: hold tool_call emission until arguments are ready or stream finishes.
				unreadyToolArgs := false
				for _, tc := range calls {
					if tc.Function.Arguments == "" && !finished {
						unreadyToolArgs = true
						break
					}
				}

				deltaReasoning, nextReasoning := diffCumulativeUTF8(currentReasoning, cs.pushedReasoning)
				cs.pushedReasoning = nextReasoning
				if deltaReasoning != "" {
					if !cs.hasReasoningItem {
						cs.hasReasoningItem = true
						cs.reasoningItemIdx = cs.allocItemIndex()
					}
					if err := d.ensureStarted(cs.role, emit); err != nil {
						return err
					}
					if err := emit(canonical.Event{
						Kind:        canonical.EventReasoningDelta,
						ChoiceIndex: cIdx,
						ItemIndex:   cs.reasoningItemIdx,
						PartIndex:   0,
						ID:          d.requestID,
						Model:       d.model,
						Role:        cs.role,
						Text:        deltaReasoning,
					}); err != nil {
						return err
					}
				}

				deltaText, nextText := diffCumulativeUTF8(currentText, cs.pushedText)
				cs.pushedText = nextText
				if deltaText != "" {
					if !cs.hasMessageItem {
						cs.hasMessageItem = true
						cs.messageItemIdx = cs.allocItemIndex()
					}
					if err := d.ensureStarted(cs.role, emit); err != nil {
						return err
					}
					if err := emit(canonical.Event{
						Kind:        canonical.EventTextDelta,
						ChoiceIndex: cIdx,
						ItemIndex:   cs.messageItemIdx,
						PartIndex:   0,
						ID:          d.requestID,
						Model:       d.model,
						Role:        cs.role,
						Text:        deltaText,
					}); err != nil {
						return err
					}
				}

				if !unreadyToolArgs {
					for _, tc := range calls {
						if tc.ID == "" && tc.Function.Name == "" && tc.Function.Arguments == "" {
							continue
						}
						tIdx := tc.Index
						ts, ok := cs.tools[tIdx]
						if !ok {
							ts = &qwenStreamToolState{
								itemIndex: cs.allocItemIndex(),
							}
							cs.tools[tIdx] = ts
						}
						if tc.ID != "" && ts.callID == "" {
							ts.callID = tc.ID
						}
						if tc.Function.Name != "" && ts.name == "" {
							ts.name = tc.Function.Name
						}

						if !ts.started {
							if ts.name == "" && ts.callID == "" && tc.Function.Arguments == "" {
								continue
							}
							ts.started = true
							if err := d.ensureStarted(cs.role, emit); err != nil {
								return err
							}
							if err := emit(canonical.Event{
								Kind:        canonical.EventToolCallStart,
								ChoiceIndex: cIdx,
								ItemIndex:   ts.itemIndex,
								ToolIndex:   tIdx,
								CallID:      ts.callID,
								ID:          d.requestID,
								Model:       d.model,
								Role:        cs.role,
								ToolName:    ts.name,
							}); err != nil {
								return err
							}
						}

						deltaArgs, nextArgs := diffCumulativeUTF8(tc.Function.Arguments, ts.pushedArgs)
						ts.pushedArgs = nextArgs
						if deltaArgs != "" {
							if err := emit(canonical.Event{
								Kind:        canonical.EventToolCallArgsDelta,
								ChoiceIndex: cIdx,
								ItemIndex:   ts.itemIndex,
								ToolIndex:   tIdx,
								CallID:      ts.callID,
								ID:          d.requestID,
								Model:       d.model,
								Role:        cs.role,
								ToolName:    ts.name,
								Text:        deltaArgs,
							}); err != nil {
								return err
							}
						}
					}
				}
			}
		}

		// When a terminal frame carries final content/tool deltas, deltas have already
		// been emitted above; emit EventChoiceFinish strictly afterwards.
		if finished {
			if d.incremental {
				if err := d.flushPendingIncrementalTools(cIdx, cs, emit); err != nil {
					return err
				}
			}
			if err := d.ensureStarted(cs.role, emit); err != nil {
				return err
			}
			if err := emit(canonical.Event{
				Kind:        canonical.EventChoiceFinish,
				ChoiceIndex: cIdx,
				ID:          d.requestID,
				Model:       d.model,
				Finish:      finish,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *qwenCanonicalEventDecoder) End() error {
	return nil
}
