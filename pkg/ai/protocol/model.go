package protocol

import (
	"encoding/json"
	"errors"
	"strings"
)

type ProtocolKind string

const (
	ProtocolUnknown         ProtocolKind = "unknown"
	ProtocolChatCompletions ProtocolKind = "chat_completions"
	ProtocolMessages        ProtocolKind = "messages"
	ProtocolResponses       ProtocolKind = "responses"
)

// ParsePublishedTextProtocol validates trusted route configuration once.
// It consumes the original published client protocol and operation, never a
// request path, a client header, or a later plugin's private context. Callers
// retain the returned kind for their existing adapters; unsupported operations
// and protocols must not fall back to legacy path detection.
func ParsePublishedTextProtocol(clientProtocol, operationKind string) (ProtocolKind, error) {
	if operationKind != "text_generate" {
		return ProtocolUnknown, errors.New("published operation_kind does not support text protocol adapters")
	}
	switch clientProtocol {
	case "openai_chat":
		return ProtocolChatCompletions, nil
	case "openai_responses":
		return ProtocolResponses, nil
	case "anthropic_messages":
		return ProtocolMessages, nil
	default:
		return ProtocolUnknown, errors.New("published client_protocol is not supported by text protocol adapters")
	}
}

// DetectProtocolKind provides legacy suffix-based compatibility only.
// Managed route bindings use ParsePublishedTextProtocol at configuration time.
func DetectProtocolKind(path string) ProtocolKind {
	pathOnly := strings.TrimSpace(strings.Split(path, "?")[0])
	switch {
	case strings.HasSuffix(pathOnly, "/v1/chat/completions"):
		return ProtocolChatCompletions
	case strings.HasSuffix(pathOnly, "/v1/messages"):
		return ProtocolMessages
	case strings.HasSuffix(pathOnly, "/v1/responses"):
		return ProtocolResponses
	default:
		return ProtocolUnknown
	}
}

type NormalizedExchange struct {
	Request           RequestFacts      `json:"request"`
	CurrentUserPrompt CurrentUserPrompt `json:"current_user_prompt"`
	InjectableContext InjectableContext `json:"injectable_context"`
	CacheDigest       CacheDigestInput  `json:"cache_digest"`
	Response          ResponseText      `json:"response"`
	Usage             Usage             `json:"usage"`
	StreamChunks      []StreamChunk     `json:"stream_chunks,omitempty"`
	Replay            ReplayPayload     `json:"replay"`
}

type RequestFacts struct {
	Protocol  ProtocolKind `json:"protocol"`
	Method    string       `json:"method,omitempty"`
	Path      string       `json:"path,omitempty"`
	Route     string       `json:"route,omitempty"`
	Tenant    string       `json:"tenant,omitempty"`
	Consumer  string       `json:"consumer,omitempty"`
	SessionID string       `json:"session_id,omitempty"`
	RequestID string       `json:"request_id,omitempty"`
	Model     string       `json:"model,omitempty"`
	Stream    bool         `json:"stream,omitempty"`
	NoStore   bool         `json:"no_store,omitempty"`
}

type CurrentUserPrompt struct {
	Text       string `json:"text,omitempty"`
	SourcePath string `json:"source_path,omitempty"`
}

type InjectableContext struct {
	Text          string             `json:"text,omitempty"`
	Placement     InjectionPlacement `json:"placement,omitempty"`
	Messages      []Message          `json:"messages,omitempty"`
	ContentBlocks []ContentBlock     `json:"content_blocks,omitempty"`
	TokenEstimate int                `json:"token_estimate,omitempty"`
	TokenBudget   int                `json:"token_budget,omitempty"`
	Digest        string             `json:"digest,omitempty"`
	Source        string             `json:"source,omitempty"`
}

type Message struct {
	Role    string          `json:"role,omitempty"`
	Content string          `json:"content,omitempty"`
	Raw     json.RawMessage `json:"raw,omitempty"`
}

type InjectionPlacement string

const (
	InjectionSystem       InjectionPlacement = "system"
	InjectionDeveloper    InjectionPlacement = "developer"
	InjectionMessages     InjectionPlacement = "messages"
	InjectionInstructions InjectionPlacement = "instructions"
	InjectionInput        InjectionPlacement = "input"
)

type ContentBlock struct {
	Type string          `json:"type,omitempty"`
	Text string          `json:"text,omitempty"`
	Raw  json.RawMessage `json:"raw,omitempty"`
}

type CacheDigestInput struct {
	Protocol              ProtocolKind               `json:"protocol"`
	Model                 string                     `json:"model,omitempty"`
	Fields                map[string]json.RawMessage `json:"fields,omitempty"`
	ExcludedFields        []string                   `json:"excluded_fields,omitempty"`
	ContextPolicyVersion  string                     `json:"context_policy_version,omitempty"`
	InjectedContextDigest string                     `json:"injected_context_digest,omitempty"`
}

type ResponseText struct {
	Text              string `json:"text,omitempty"`
	FinishReason      string `json:"finish_reason,omitempty"`
	ContainsToolCalls bool   `json:"contains_tool_calls,omitempty"`
	ParseFailed       bool   `json:"parse_failed,omitempty"`
	UnsafeContent     bool   `json:"unsafe_content,omitempty"`
}

type Usage struct {
	InputTokens          int `json:"input_tokens,omitempty"`
	InputCacheHitTokens  int `json:"input_cache_hit_tokens,omitempty"`
	InputCacheMissTokens int `json:"input_cache_miss_tokens,omitempty"`
	OutputTokens         int `json:"output_tokens,omitempty"`
	TotalTokens          int `json:"total_tokens,omitempty"`
}

type StreamChunk struct {
	Protocol     ProtocolKind    `json:"protocol"`
	Event        string          `json:"event,omitempty"`
	Data         json.RawMessage `json:"data,omitempty"`
	TextDelta    string          `json:"text_delta,omitempty"`
	FinishReason string          `json:"finish_reason,omitempty"`
	Usage        *Usage          `json:"usage,omitempty"`
	Replayable   bool            `json:"replayable,omitempty"`
}

type ReplayPayload struct {
	Protocol        ProtocolKind    `json:"protocol"`
	StatusCode      int             `json:"status_code,omitempty"`
	ContentType     string          `json:"content_type,omitempty"`
	Body            json.RawMessage `json:"body,omitempty"`
	StreamChunks    []StreamChunk   `json:"stream_chunks,omitempty"`
	UpstreamInvoked *bool           `json:"upstream_invoked,omitempty"`
}

func (payload ReplayPayload) Validate(stream bool) error {
	if payload.Protocol == "" || payload.Protocol == ProtocolUnknown {
		return errors.New("replay protocol is required")
	}
	if stream {
		if len(payload.StreamChunks) == 0 {
			return errors.New("stream replay chunks are required")
		}
		return nil
	}
	if len(payload.Body) == 0 {
		return errors.New("non-stream replay body is required")
	}
	return nil
}
