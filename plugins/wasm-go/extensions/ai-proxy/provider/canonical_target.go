package provider

import (
	"bytes"
	"encoding/json"
	"math"
	"slices"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/canonical"
)

// WireFormat identifies the concrete upstream wire protocol of a selected target.
type WireFormat string

const (
	WireChatCompletions       WireFormat = "chat_completions"
	WireOpenAIResponses       WireFormat = "openai_responses"
	WireAnthropicMessages     WireFormat = "anthropic_messages"
	WireDashScopeText         WireFormat = "dashscope_text"
	WireDashScopeMultimodal   WireFormat = "dashscope_multimodal"
	WireGeminiGenerateContent WireFormat = "gemini_generate_content"
)

// Valid reports whether w is one of the six registered supplier wire formats.
func (w WireFormat) Valid() bool {
	switch w {
	case WireChatCompletions,
		WireOpenAIResponses,
		WireAnthropicMessages,
		WireDashScopeText,
		WireDashScopeMultimodal,
		WireGeminiGenerateContent:
		return true
	default:
		return false
	}
}

// IncrementalMode controls upstream incremental vs cumulative streaming output.
type IncrementalMode uint8

const (
	IncrementalAuto IncrementalMode = iota
	IncrementalForceTrue
	IncrementalForceFalse
)

// LimitScopeMask encodes accepted canonical.LimitScope values as a bitmask.
type LimitScopeMask uint8

const (
	LimitScopeMaskNone                LimitScopeMask = 0
	LimitScopeMaskSourceDefined       LimitScopeMask = 1 << canonical.ScopeSourceDefined
	LimitScopeMaskVisibleOnly         LimitScopeMask = 1 << canonical.ScopeVisibleOnly
	LimitScopeMaskReasoningAndVisible LimitScopeMask = 1 << canonical.ScopeReasoningAndVisible
	LimitScopeMaskAll                 LimitScopeMask = LimitScopeMaskSourceDefined | LimitScopeMaskVisibleOnly | LimitScopeMaskReasoningAndVisible
)

// Allows reports whether m permits the given canonical.LimitScope.
func (m LimitScopeMask) Allows(scope canonical.LimitScope) bool {
	if scope > canonical.ScopeReasoningAndVisible {
		return false
	}
	return (m & (1 << scope)) != 0
}

// CanonicalTarget captures the immutable supplier wire format, model identity,
// API revision, and capability contract frozen prior to request encoding.
type CanonicalTarget struct {
	WireFormat                     WireFormat
	Supplier                       string
	Model                          string
	APIVersion                     string
	Streaming                      bool
	SupportsDeveloperRole          bool
	AcceptedLimitScopes            LimitScopeMask
	MaxOutputTokens                int
	SupportsCompletionTokens       bool
	SupportsThinking               bool
	SupportsAdaptiveThinking       bool
	SupportsPreserveThinking       bool
	ForbidsForcedToolThink         bool
	IncrementalOutputMode          IncrementalMode
	SupportsStrictSchema           bool
	SupportsURLImages              bool
	AllowedImageDetails            []canonical.ImageDetail
	SupportsStore                  bool
	SupportsPromptCacheKey         bool
	SupportsContentCacheControl    bool
	NormalizeEmptyAssistantContent bool
	StripToolMessageName           bool
	StripOuterCacheControl         bool
	TopLevelPreserveThinking       bool
	QwenEnableSearchDefault        bool
	QwenLongFileIDs                []string
	GeminiSafetySettings           map[string]string
	GeminiThinkingBudget           int64
}

// CanonicalEventDecoder normalizes delimited supplier SSE payloads into ordered
// canonical.Event values on a per-attempt basis without touching HttpContext.
type CanonicalEventDecoder interface {
	Decode(name string, payload []byte, emit func(canonical.Event) error) error
	End() error
}

// qwenChatCompatPolicy is the single frozen policy for Qwen OpenAI-compatible
// Chat endpoints (B03 / H3), shared between cross-wire EncodeCanonicalRequest
// and same-wire sanitizeQwenCompatibleChatRequest.
type qwenChatCompatPolicy struct {
	SupportsDeveloperRole          bool
	SupportsStore                  bool
	SupportsPromptCacheKey         bool
	SupportsContentCacheControl    bool
	NormalizeEmptyAssistantContent bool
	StripToolMessageName           bool
	StripOuterCacheControl         bool
	TopLevelPreserveThinking       bool
}

func qwenCompatibleChatPolicy(mappedModel string) qwenChatCompatPolicy {
	return qwenChatCompatPolicy{
		SupportsDeveloperRole:          false,
		SupportsStore:                  false,
		SupportsPromptCacheKey:         false,
		SupportsContentCacheControl:    true,
		NormalizeEmptyAssistantContent: true,
		StripToolMessageName:           true,
		StripOuterCacheControl:         true,
		TopLevelPreserveThinking:       qwenSupportsPreserveThinking(mappedModel),
	}
}

func (p qwenChatCompatPolicy) applyToTarget(target *CanonicalTarget) {
	if target == nil {
		return
	}
	target.SupportsDeveloperRole = p.SupportsDeveloperRole
	target.SupportsStore = p.SupportsStore
	target.SupportsPromptCacheKey = p.SupportsPromptCacheKey
	target.SupportsContentCacheControl = p.SupportsContentCacheControl
	target.NormalizeEmptyAssistantContent = p.NormalizeEmptyAssistantContent
	target.StripToolMessageName = p.StripToolMessageName
	target.StripOuterCacheControl = p.StripOuterCacheControl
	target.TopLevelPreserveThinking = p.TopLevelPreserveThinking
}

// wireOptional decodes JSON tri-state presence (omitted vs null vs explicit value)
// directly into typed Go values without dynamic empty interfaces.
type wireOptional[T any] struct {
	state canonical.Presence
	val   T
}

func (w *wireOptional[T]) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		w.state = canonical.Null
		var zero T
		w.val = zero
		return nil
	}
	var v T
	if err := json.Unmarshal(trimmed, &v); err != nil {
		return err
	}
	w.state = canonical.Present
	w.val = v
	return nil
}

func (w wireOptional[T]) toCanonical() canonical.Optional[T] {
	switch w.state {
	case canonical.Null:
		return canonical.NullOf[T]()
	case canonical.Present:
		return canonical.Some(w.val)
	default:
		return canonical.AbsentOf[T]()
	}
}

func parseCanonicalImageURL(rawURL string, detail canonical.ImageDetail, errField string) (*canonical.ImagePart, error) {
	if !detail.Valid() {
		return nil, canonical.NewRejectionError("invalid_image_detail", "detail", "unsupported image detail")
	}
	if rawURL == "" {
		return nil, canonical.NewRejectionError("unsupported_image_content", errField, "image URL must not be empty")
	}
	if strings.HasPrefix(rawURL, "data:") {
		semiIdx := strings.Index(rawURL, ";base64,")
		if semiIdx <= len("data:") {
			return nil, canonical.NewRejectionError("invalid_data_url", errField, "data URL must match data:<media_type>;base64,<data>")
		}
		mediaType := rawURL[len("data:"):semiIdx]
		base64Ref := rawURL[semiIdx+len(";base64,"):]
		if mediaType == "" || base64Ref == "" {
			return nil, canonical.NewRejectionError("invalid_data_url", errField, "data URL requires non-empty media type and base64 payload")
		}
		return &canonical.ImagePart{
			Source:    canonical.ImageDataURL,
			URL:       rawURL,
			MediaType: mediaType,
			Base64Ref: base64Ref,
			Detail:    detail,
		}, nil
	}
	return &canonical.ImagePart{
		Source: canonical.ImageURL,
		URL:    rawURL,
		Detail: detail,
	}, nil
}

// DecodeCanonicalRequest decodes a client protocol JSON body into a canonical.Request.
func DecodeCanonicalRequest(protocol canonical.Protocol, body []byte) (canonical.Request, error) {
	switch protocol {
	case canonical.ProtocolOpenAIChat:
		return decodeChatCanonical(body)
	case canonical.ProtocolOpenAIResponses:
		return decodeResponsesCanonical(body)
	case canonical.ProtocolAnthropicMessages:
		return decodeMessagesCanonical(body)
	default:
		return canonical.Request{}, canonical.NewRejectionError("unsupported_client_protocol", string(protocol), "unsupported client protocol for canonical decode")
	}
}

// ValidateCanonicalTarget validates whether req can be safely and losslessly
// (or via approved compatible rules) encoded for target before any upstream send.
func ValidateCanonicalTarget(req canonical.Request, target CanonicalTarget) error {
	if !target.WireFormat.Valid() {
		return canonical.NewRejectionError("unsupported_wire_format", string(target.WireFormat), "unsupported target wire format")
	}

	if req.PreviousResponseID.IsPresent() && req.PreviousResponseID.Val != "" {
		return canonical.NewRejectionError("unsupported_stateful_reference", "previous_response_id", "previous_response_id is unsupported on target")
	}
	if req.ConversationPresent {
		return canonical.NewRejectionError("unsupported_stateful_reference", "conversation", "conversation is unsupported on target")
	}
	if req.Background {
		return canonical.NewRejectionError("unsupported_background_mode", "background", "background execution is unsupported on target")
	}

	if req.Limits.Value.IsPresent() {
		limitField := "max_tokens"
		switch req.Limits.Source {
		case canonical.LimitMaxCompletionTokens:
			limitField = "max_completion_tokens"
		case canonical.LimitMaxOutputTokens:
			limitField = "max_output_tokens"
		}
		val := req.Limits.Value.Val
		if val < 0 {
			return canonical.NewRejectionError("invalid_output_limit", limitField, "output token limit must be non-negative")
		}
		if val == 0 && (target.WireFormat != WireAnthropicMessages || req.Limits.Source == canonical.LimitMaxOutputTokens) {
			return canonical.NewRejectionError("unsupported_zero_output_tokens", limitField, "zero output token warmup is unsupported on target")
		}
		if target.AcceptedLimitScopes != LimitScopeMaskNone && !target.AcceptedLimitScopes.Allows(req.Limits.Scope) {
			return canonical.NewRejectionError("unsupported_output_limit_scope", limitField, "output token limit scope is unsupported on target")
		}
	}

	if req.Sampling.Temperature.IsPresent() {
		v := req.Sampling.Temperature.Val
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return canonical.NewRejectionError("invalid_sampling_parameter", "temperature", "temperature must be non-negative")
		}
	}
	if req.Sampling.TopP.IsPresent() {
		v := req.Sampling.TopP.Val
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
			return canonical.NewRejectionError("invalid_sampling_parameter", "top_p", "top_p must be within [0, 1]")
		}
	}
	if req.Sampling.TopK.IsPresent() && req.Sampling.TopK.Val < 0 {
		return canonical.NewRejectionError("invalid_sampling_parameter", "top_k", "top_k must be non-negative")
	}
	if req.Sampling.N.IsPresent() && req.Sampling.N.Val <= 0 {
		return canonical.NewRejectionError("invalid_sampling_parameter", "n", "n must be at least 1")
	}
	if req.Sampling.StopPresent && len(req.Sampling.Stop) > 4 &&
		(target.WireFormat == WireChatCompletions || target.WireFormat == WireDashScopeText || target.WireFormat == WireDashScopeMultimodal) {
		return canonical.NewRejectionError("invalid_stop", "stop", "stop sequences exceed maximum of 4")
	}

	switch req.ResponseFormat.Kind {
	case canonical.ResponseFormatUnset, canonical.ResponseFormatText:
	case canonical.ResponseFormatJSONObject:
		if target.WireFormat == WireAnthropicMessages {
			return canonical.NewRejectionError("unsupported_response_format", "response_format", "json_object response_format is unsupported on target")
		}
	case canonical.ResponseFormatJSONSchema:
		trimmedSchema := bytes.TrimSpace(req.ResponseFormat.Schema)
		if len(trimmedSchema) == 0 || bytes.Equal(trimmedSchema, []byte("null")) {
			return canonical.NewRejectionError("invalid_response_format", "response_format", "json_schema requires a non-empty schema")
		}
		if !target.SupportsStrictSchema || target.WireFormat == WireAnthropicMessages {
			return canonical.NewRejectionError("unsupported_strict_schema", "response_format", "json_schema structured output is unsupported on target")
		}
	default:
		return canonical.NewRejectionError("unsupported_response_format", "response_format", "unsupported response_format")
	}

	for _, t := range req.Tools {
		if t.Kind == canonical.ToolKindFunction && t.Strict.IsPresent() && t.Strict.Val && !target.SupportsStrictSchema {
			return canonical.NewRejectionError("unsupported_strict_schema", "tools", "strict tool schema is unsupported on target")
		}
	}

	forcedToolChoice := req.ToolChoice.Mode == canonical.ToolChoiceRequired ||
		req.ToolChoice.Mode == canonical.ToolChoiceAny ||
		req.ToolChoice.Mode == canonical.ToolChoiceFunction ||
		req.ToolChoice.Mode == canonical.ToolChoiceFixed
	if forcedToolChoice {
		if len(req.Tools) == 0 {
			return canonical.NewRejectionError("invalid_tool_choice", "tool_choice", "forced tool_choice requires declared tools")
		}
		thinkingActive := req.Thinking.Mode == canonical.ThinkingEnabled ||
			req.Thinking.Mode == canonical.ThinkingAdaptive ||
			(req.Thinking.Mode == canonical.ThinkingUnset && req.Thinking.Effort.IsPresent() && req.Thinking.Effort.Val != "" && req.Thinking.Effort.Val != "none")
		if thinkingActive && (target.ForbidsForcedToolThink ||
			target.WireFormat == WireDashScopeText ||
			target.WireFormat == WireDashScopeMultimodal ||
			target.WireFormat == WireAnthropicMessages ||
			(req.SourceProtocol == canonical.ProtocolAnthropicMessages && req.Thinking.Mode == canonical.ThinkingEnabled)) {
			return canonical.NewRejectionError("conflicting_thinking_tool_choice", "tool_choice", "forced tool_choice cannot be combined with active thinking")
		}
	}

	validatePart := func(p canonical.Part, inToolResult bool) error {
		if p.Kind != canonical.PartImage && p.Image == nil {
			return nil
		}
		imgField := "image_url"
		if inToolResult {
			imgField = "tool_result"
		}
		if p.Image == nil {
			return canonical.NewRejectionError("invalid_image_content", imgField, "image part is missing image metadata")
		}
		if inToolResult && req.SourceProtocol != canonical.ProtocolAnthropicMessages {
			return canonical.NewRejectionError("unsupported_image_content", "function_call_output", "multimodal function_call_output is unsupported")
		}
		if target.WireFormat == WireDashScopeText {
			return canonical.NewRejectionError("unsupported_image_content", imgField, "dashscope_text does not support image inputs")
		}
		if p.Image.Source == canonical.ImageFileID || p.Image.FileID != "" {
			return canonical.NewRejectionError("unsupported_image_content", "file_id", "image file_id cannot be resolved across protocols")
		}
		lowerURL := strings.ToLower(p.Image.URL)
		isRemoteURL := p.Image.Source == canonical.ImageURL || strings.HasPrefix(lowerURL, "http://") || strings.HasPrefix(lowerURL, "https://")
		if isRemoteURL && (!target.SupportsURLImages || target.WireFormat == WireGeminiGenerateContent) {
			return canonical.NewRejectionError("unsupported_image_url", "image_url", "remote image URLs are unsupported on target")
		}
		if !p.Image.Detail.Valid() {
			return canonical.NewRejectionError("invalid_image_detail", "detail", "invalid image detail")
		}
		if p.Image.Detail != canonical.ImageDetailUnset {
			if target.AllowedImageDetails != nil {
				if !slices.Contains(target.AllowedImageDetails, p.Image.Detail) {
					return canonical.NewRejectionError("unsupported_image_detail", "detail", "image detail is unsupported on target")
				}
			} else if p.Image.Detail == canonical.ImageDetailOriginal {
				return canonical.NewRejectionError("unsupported_image_detail", "detail", "image detail original requires explicit target capability")
			}
		}
		return nil
	}

	for _, p := range req.SystemParts {
		if err := validatePart(p, false); err != nil {
			return err
		}
	}
	for _, item := range req.Items {
		if item.Reasoning != nil && item.Reasoning.Cache != nil {
			return canonical.NewRejectionError("invalid_cache_control", "thinking", "thinking blocks cannot carry cache_control")
		}
		for _, p := range item.Parts {
			if err := validatePart(p, false); err != nil {
				return err
			}
		}
		if item.Result != nil {
			for _, p := range item.Result.Parts {
				if err := validatePart(p, true); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// EncodeCanonicalRequest encodes req into the upstream wire format designated by target.
func EncodeCanonicalRequest(req canonical.Request, target CanonicalTarget) ([]byte, error) {
	if target.MaxOutputTokens > 0 && req.Limits.Value.IsPresent() {
		req.Limits.Value.Val = min(req.Limits.Value.Val, target.MaxOutputTokens)
	}
	switch target.WireFormat {
	case WireChatCompletions:
		return encodeChatCanonicalRequest(req, target)
	case WireDashScopeText, WireDashScopeMultimodal:
		return encodeQwenCanonicalRequest(req, target)
	case WireGeminiGenerateContent:
		return encodeGeminiCanonicalRequest(req, target)
	default:
		return nil, canonical.NewRejectionError("unsupported_wire_format", string(target.WireFormat), "unsupported target wire format for canonical request encode")
	}
}

// DecodeCanonicalResponse decodes a non-streaming upstream response body into canonical.Response.
func DecodeCanonicalResponse(target CanonicalTarget, body []byte) (canonical.Response, error) {
	switch target.WireFormat {
	case WireChatCompletions:
		return decodeChatCanonicalResponse(body)
	case WireDashScopeText, WireDashScopeMultimodal:
		return decodeQwenCanonicalResponse(target, body)
	case WireGeminiGenerateContent:
		return decodeGeminiCanonicalResponse(target, body)
	default:
		return canonical.Response{}, canonical.NewRejectionError("unsupported_wire_format", string(target.WireFormat), "unsupported target wire format for canonical response decode")
	}
}

// NewCanonicalEventDecoder constructs a stateful per-attempt SSE event decoder for target.
func NewCanonicalEventDecoder(target CanonicalTarget) (CanonicalEventDecoder, error) {
	switch target.WireFormat {
	case WireChatCompletions:
		return newChatCanonicalEventDecoder(), nil
	case WireDashScopeText, WireDashScopeMultimodal:
		return newQwenCanonicalEventDecoder(target)
	case WireGeminiGenerateContent:
		return newGeminiCanonicalEventDecoder(target)
	default:
		return nil, canonical.NewRejectionError("unsupported_wire_format", string(target.WireFormat), "unsupported target wire format for canonical event decoder")
	}
}
