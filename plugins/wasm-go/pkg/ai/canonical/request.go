package canonical

import (
	"bytes"
	"encoding/json"
	"maps"
	"slices"
)

// LimitSource records which wire token-budget parameter originated OutputLimit.
type LimitSource uint8

const (
	LimitNone LimitSource = iota
	LimitMaxTokens
	LimitMaxCompletionTokens
	LimitMaxOutputTokens
)

// LimitScope records whether a token limit bounds visible output only or
// reasoning plus visible output together.
type LimitScope uint8

const (
	ScopeSourceDefined LimitScope = iota
	ScopeVisibleOnly
	ScopeReasoningAndVisible
)

// OutputLimit preserves both the originating parameter semantics and explicit 0
// values (so cross-protocol max_tokens:0 warmup requests can be rejected).
type OutputLimit struct {
	Source LimitSource
	Scope  LimitScope
	Value  Optional[int]
}

// Sampling holds decoding and stopping parameters with explicit zero preservation.
type Sampling struct {
	Temperature       Optional[float64]
	TopP              Optional[float64]
	TopK              Optional[int]
	RepetitionPenalty Optional[float64]
	PresencePenalty   Optional[float64]
	FrequencyPenalty  Optional[float64]
	Seed              Optional[int64]
	Stop              []string
	StopPresent       bool
	N                 Optional[int]
	Logprobs          Optional[bool]
	TopLogprobs       Optional[int]
}

// ThinkingMode represents explicit reasoning control modes across protocols.
type ThinkingMode string

const (
	ThinkingUnset    ThinkingMode = ""
	ThinkingEnabled  ThinkingMode = "enabled"
	ThinkingDisabled ThinkingMode = "disabled"
	ThinkingAdaptive ThinkingMode = "adaptive"
)

// ThinkingConfig captures reasoning activation, token budget, effort level, and
// history-preservation intent.
type ThinkingConfig struct {
	Mode             ThinkingMode
	BudgetTokens     Optional[int]
	Effort           Optional[string]
	PreserveThinking Optional[bool]
}

// OpaqueOrigin binds supplier-specific signatures or encrypted reasoning blocks
// to their originating protocol and supplier family.
type OpaqueOrigin struct {
	Protocol Protocol
	Supplier string
}

// CacheOrigin distinguishes Anthropic/DashScope ephemeral cache_control markers
// from OpenAI Responses prompt_cache_options / prompt_cache_breakpoint markers.
type CacheOrigin uint8

const (
	CacheOriginNone CacheOrigin = iota
	CacheOriginEphemeralControl
	CacheOriginOpenAIPromptCache
)

// CacheBoundary represents an explicit prompt-cache breakpoint or request-level
// cache control directive.
type CacheBoundary struct {
	Origin CacheOrigin
	Mode   string
	TTL    string
}

// CachePolicy holds request-level cache keys, retention policies, and top-level
// cache boundary directives (OpenAI prompt_cache_options or Anthropic top-level
// cache_control).
type CachePolicy struct {
	PromptCacheKey       Optional[string]
	PromptCacheRetention Optional[string]
	RequestBoundary      *CacheBoundary
}

// ResponseFormatKind enumerates structured output modes.
type ResponseFormatKind string

const (
	ResponseFormatUnset      ResponseFormatKind = ""
	ResponseFormatText       ResponseFormatKind = "text"
	ResponseFormatJSONObject ResponseFormatKind = "json_object"
	ResponseFormatJSONSchema ResponseFormatKind = "json_schema"
)

// ResponseFormat preserves JSON mode vs JSON Schema constraints without parsing
// schemas through map[string]any (preserving >53-bit integer constants).
type ResponseFormat struct {
	Kind        ResponseFormatKind
	Name        string
	Description string
	Schema      json.RawMessage
	Strict      Optional[bool]
}

// ArgumentsKind distinguishes string-encoded tool arguments (Chat/Responses)
// from raw JSON object arguments (Messages/Gemini).
type ArgumentsKind uint8

const (
	ArgsNone ArgumentsKind = iota
	ArgsText
	ArgsObject
)

// Arguments preserves exact tool-call argument representations across wire shapes.
type Arguments struct {
	Kind   ArgumentsKind
	Text   string
	Object json.RawMessage
}

// ImageDetail enumerates vision resolution hints (including OA09 "original").
type ImageDetail string

const (
	ImageDetailUnset    ImageDetail = ""
	ImageDetailAuto     ImageDetail = "auto"
	ImageDetailLow      ImageDetail = "low"
	ImageDetailHigh     ImageDetail = "high"
	ImageDetailOriginal ImageDetail = "original"
)

// Valid reports whether d is a recognized image detail value.
func (d ImageDetail) Valid() bool {
	switch d {
	case ImageDetailUnset, ImageDetailAuto, ImageDetailLow, ImageDetailHigh, ImageDetailOriginal:
		return true
	default:
		return false
	}
}

// ImageSourceKind identifies how an input image is referenced.
type ImageSourceKind uint8

const (
	ImageSourceNone ImageSourceKind = iota
	ImageURL
	ImageDataURL
	ImageFileID
)

// ImagePart holds multimodal image metadata without decoding or re-encoding
// underlying Base64 payloads.
type ImagePart struct {
	Source    ImageSourceKind
	URL       string
	MediaType string
	Base64Ref string
	FileID    string
	Detail    ImageDetail
}

// PartKind enumerates content part variants within a message or tool result.
type PartKind uint8

const (
	PartText PartKind = iota + 1
	PartImage
	PartRefusal
)

// Part represents a single ordered content block within an Item or FunctionResult.
type Part struct {
	Kind        PartKind
	Text        string
	Image       *ImagePart
	Refusal     string
	Annotations []json.RawMessage
	Cache       *CacheBoundary
}

// ItemKind enumerates conversation and output item variants.
type ItemKind uint8

const (
	ItemMessage ItemKind = iota + 1
	ItemFunctionCall
	ItemFunctionResult
	ItemReasoning
)

// Role enumerates canonical message participant roles.
type Role string

const (
	RoleSystem    Role = "system"
	RoleDeveloper Role = "developer"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// FunctionCall represents an assistant tool invocation.
type FunctionCall struct {
	ItemID           string
	CallID           string
	WireCallID       Optional[string] // The recorded provider ID, which can be explicitly empty.
	Name             string
	Args             Arguments
	ThoughtSignature string
	SignatureOrigin  OpaqueOrigin
}

// FunctionResult represents a client-supplied tool execution output.
type FunctionResult struct {
	ItemID       string
	CallID       string
	Name         string
	Parts        []Part
	IsStringForm bool
	IsError      bool
	Cache        *CacheBoundary
}

// ReasoningItem represents assistant thinking, summary, redacted, or encrypted
// reasoning blocks. Summary uses Optional[[]string] to distinguish explicit
// summary:[] (skipped on stateless cross-protocol targets) from bare {"type":"reasoning"}.
type ReasoningItem struct {
	ItemID    string
	Text      []string
	Summary   Optional[[]string]
	Signature string
	Redacted  string
	Encrypted json.RawMessage
	Origin    OpaqueOrigin
	Cache     *CacheBoundary
}

// Item is an ordered conversation turn element in Request.Items or Choice.Items.
type Item struct {
	Kind           ItemKind
	Role           Role
	MessageID      string
	Parts          []Part
	MessageIsArray bool
	Partial        Optional[bool]
	Call           *FunctionCall
	Result         *FunctionResult
	Reasoning      *ReasoningItem
	Cache          *CacheBoundary
}

// ToolKind separates client function tools from gateway-managed fixed server tools.
type ToolKind uint8

const (
	ToolKindFunction ToolKind = iota + 1
	ToolKindFixedServer
)

// ToolDefinition describes a declared function or fixed server tool.
type ToolDefinition struct {
	Kind        ToolKind
	FixedType   string
	Name        string
	Description string
	Schema      json.RawMessage
	Strict      Optional[bool]
	RawFixed    json.RawMessage
	Cache       *CacheBoundary
}

// ToolChoiceMode enumerates tool selection strategies.
type ToolChoiceMode string

const (
	ToolChoiceUnset    ToolChoiceMode = ""
	ToolChoiceAuto     ToolChoiceMode = "auto"
	ToolChoiceNone     ToolChoiceMode = "none"
	ToolChoiceRequired ToolChoiceMode = "required"
	ToolChoiceAny      ToolChoiceMode = "any"
	ToolChoiceFunction ToolChoiceMode = "function"
	ToolChoiceFixed    ToolChoiceMode = "fixed"
)

// ToolChoice captures the normalized tool selection policy.
type ToolChoice struct {
	Mode     ToolChoiceMode
	Name     string
	FixedRaw string
}

// FixedToolControls captures top-level built-in tool switches and options.
type FixedToolControls struct {
	EnableSearch          Optional[bool]
	EnableCodeInterpreter Optional[bool]
	SearchStrategy        Optional[string]
	SearchOptions         map[string]json.RawMessage // Supplier options other than the normalized SearchStrategy.
	WebSearchOptionsRaw   json.RawMessage
}

// Request is the read-only canonical representation of a decoded client request.
type Request struct {
	SourceProtocol      Protocol
	Model               string
	Stream              bool
	IncludeUsage        Optional[bool]
	Instructions        Optional[string]
	SystemParts         []Part
	SystemIsArray       bool
	Items               []Item
	Sampling            Sampling
	Limits              OutputLimit
	ResponseFormat      ResponseFormat
	Thinking            ThinkingConfig
	Cache               CachePolicy
	Tools               []ToolDefinition
	ToolChoice          ToolChoice
	ParallelToolCalls   Optional[bool]
	FixedTools          FixedToolControls
	Store               Optional[bool]
	Background          bool
	ServiceTier         Optional[string]
	User                Optional[string]
	Metadata            Optional[map[string]string]
	PreviousResponseID  Optional[string]
	ConversationPresent bool
}

// CloneForAttempt returns a deep copy of r so per-attempt model substitution,
// fixed-tool filtering, or target normalization never mutates a cached base Request.
func (r Request) CloneForAttempt() Request {
	out := r
	out.SystemParts = cloneParts(r.SystemParts)
	if r.Items != nil {
		out.Items = make([]Item, len(r.Items))
		for i, item := range r.Items {
			out.Items[i] = cloneItem(item)
		}
	}
	out.Sampling.Stop = slices.Clone(r.Sampling.Stop)
	out.ResponseFormat.Schema = bytes.Clone(r.ResponseFormat.Schema)
	if r.Cache.RequestBoundary != nil {
		b := *r.Cache.RequestBoundary
		out.Cache.RequestBoundary = &b
	}
	if r.Tools != nil {
		out.Tools = make([]ToolDefinition, len(r.Tools))
		for i, t := range r.Tools {
			tc := t
			tc.Schema = bytes.Clone(t.Schema)
			tc.RawFixed = bytes.Clone(t.RawFixed)
			if t.Cache != nil {
				b := *t.Cache
				tc.Cache = &b
			}
			out.Tools[i] = tc
		}
	}
	out.FixedTools.SearchOptions = maps.Clone(r.FixedTools.SearchOptions)
	for key, value := range out.FixedTools.SearchOptions {
		out.FixedTools.SearchOptions[key] = bytes.Clone(value)
	}
	out.FixedTools.WebSearchOptionsRaw = bytes.Clone(r.FixedTools.WebSearchOptionsRaw)
	if r.Metadata.IsPresent() && r.Metadata.Val != nil {
		out.Metadata = Some(maps.Clone(r.Metadata.Val))
	}
	return out
}

func cloneItem(item Item) Item {
	out := item
	out.Parts = cloneParts(item.Parts)
	if item.Call != nil {
		call := *item.Call
		call.Args.Object = bytes.Clone(item.Call.Args.Object)
		out.Call = &call
	}
	if item.Result != nil {
		res := *item.Result
		res.Parts = cloneParts(item.Result.Parts)
		if item.Result.Cache != nil {
			b := *item.Result.Cache
			res.Cache = &b
		}
		out.Result = &res
	}
	if item.Reasoning != nil {
		rs := *item.Reasoning
		rs.Text = slices.Clone(item.Reasoning.Text)
		if item.Reasoning.Summary.IsPresent() && item.Reasoning.Summary.Val != nil {
			rs.Summary = Some(slices.Clone(item.Reasoning.Summary.Val))
		}
		rs.Encrypted = bytes.Clone(item.Reasoning.Encrypted)
		if item.Reasoning.Cache != nil {
			b := *item.Reasoning.Cache
			rs.Cache = &b
		}
		out.Reasoning = &rs
	}
	if item.Cache != nil {
		b := *item.Cache
		out.Cache = &b
	}
	return out
}

func cloneParts(parts []Part) []Part {
	if parts == nil {
		return nil
	}
	out := make([]Part, len(parts))
	for i, p := range parts {
		pc := p
		if p.Image != nil {
			img := *p.Image
			pc.Image = &img
		}
		if p.Annotations != nil {
			pc.Annotations = make([]json.RawMessage, len(p.Annotations))
			for j, ann := range p.Annotations {
				pc.Annotations[j] = bytes.Clone(ann)
			}
		}
		if p.Cache != nil {
			b := *p.Cache
			pc.Cache = &b
		}
		out[i] = pc
	}
	return out
}
