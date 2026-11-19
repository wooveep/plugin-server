// Package canonical defines the strongly typed intermediate representation (IR)
// shared across client protocol codecs, fixed-tool admission/filtering, public
// response projection, and supplier wire codecs.
//
// This package is a pure standard-library leaf package: it contains no dynamic
// interface{} payloads, untyped extension maps, network I/O, or billing logic.
package canonical

// Protocol identifies a registered wire or logical protocol syntax.
type Protocol string

const (
	ProtocolOpenAIChat          Protocol = "openai_chat"
	ProtocolOpenAIResponses     Protocol = "openai_responses"
	ProtocolAnthropicMessages   Protocol = "anthropic_messages"
	ProtocolDashScopeText       Protocol = "dashscope_text"
	ProtocolDashScopeMultimodal Protocol = "dashscope_multimodal"
	ProtocolGeminiGenerate      Protocol = "gemini_generate_content"
)

// Valid reports whether p is one of the six registered canonical protocols.
func (p Protocol) Valid() bool {
	switch p {
	case ProtocolOpenAIChat,
		ProtocolOpenAIResponses,
		ProtocolAnthropicMessages,
		ProtocolDashScopeText,
		ProtocolDashScopeMultimodal,
		ProtocolGeminiGenerate:
		return true
	default:
		return false
	}
}

// IsClientProtocol reports whether p is a public client-facing text protocol.
func (p Protocol) IsClientProtocol() bool {
	return p == ProtocolOpenAIChat || p == ProtocolOpenAIResponses || p == ProtocolAnthropicMessages
}

// Presence distinguishes an omitted JSON member, an explicit JSON null, and an
// explicit value (including zero scalars, false, empty strings, empty slices,
// and empty maps).
type Presence uint8

const (
	Absent Presence = iota
	Null
	Present
)

// Optional preserves tri-state JSON presence without pointer/zero-value ambiguity.
type Optional[T any] struct {
	State Presence
	Val   T
}

// AbsentOf constructs an omitted Optional[T].
func AbsentOf[T any]() Optional[T] {
	return Optional[T]{State: Absent}
}

// NullOf constructs an explicit-null Optional[T].
func NullOf[T any]() Optional[T] {
	return Optional[T]{State: Null}
}

// Some constructs an explicit-value Optional[T].
func Some[T any](v T) Optional[T] {
	return Optional[T]{State: Present, Val: v}
}

// IsAbsent reports whether the field was omitted.
func (o Optional[T]) IsAbsent() bool { return o.State == Absent }

// IsNull reports whether the field was explicitly set to null.
func (o Optional[T]) IsNull() bool { return o.State == Null }

// IsPresent reports whether the field carries an explicit non-null value.
func (o Optional[T]) IsPresent() bool { return o.State == Present }

// ValueOr returns Val when Present, or fallback otherwise.
func (o Optional[T]) ValueOr(fallback T) T {
	if o.State == Present {
		return o.Val
	}
	return fallback
}

// SafeIdentifier returns value if it is a safe, bounded ASCII identifier
// matching [a-zA-Z0-9_.-] of length 1..64, or "" otherwise.
// It guarantees user prompt fragments, tokens, or resource values cannot leak
// into structured rejection codes or field names.
func SafeIdentifier(value string) string {
	if value == "" || len(value) > 64 {
		return ""
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		isValid := (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '_' || c == '.' || c == '-'
		if !isValid {
			return ""
		}
	}
	return value
}

// RejectionError identifies a deterministic request decode, target capability,
// or supplier encode rejection using sanitized identifiers safe for structured
// gateway logging.
type RejectionError struct {
	Code    string
	Field   string
	Message string
}

// NewRejectionError constructs a RejectionError with sanitized Code and Field.
func NewRejectionError(code, field, message string) RejectionError {
	safeCode := SafeIdentifier(code)
	if safeCode == "" {
		safeCode = "invalid_protocol_request"
	}
	return RejectionError{
		Code:    safeCode,
		Field:   SafeIdentifier(field),
		Message: message,
	}
}

// SafeCode returns the sanitized rejection code, falling back to a fixed code
// when Code is empty or contains unsafe characters.
func (e RejectionError) SafeCode() string {
	if code := SafeIdentifier(e.Code); code != "" {
		return code
	}
	return "invalid_protocol_request"
}

// SafeField returns the sanitized field or item type identifier, or "".
func (e RejectionError) SafeField() string {
	return SafeIdentifier(e.Field)
}

// SafeDetail formats "code=<SafeCode>" (and " field=<SafeField>" when present)
// for sanitized gateway warning logs.
func (e RejectionError) SafeDetail() string {
	code := e.SafeCode()
	if field := e.SafeField(); field != "" {
		return "code=" + code + " field=" + field
	}
	return "code=" + code
}

// Error implements error without exposing unsanitized Code or Field values.
func (e RejectionError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	code := e.SafeCode()
	if field := e.SafeField(); field != "" {
		return code + ":" + field
	}
	return code
}
