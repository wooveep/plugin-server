package canonical

import "encoding/json"

// FinishCategory classifies supplier generation termination semantics without
// conflating unknown reasons with normal stop.
type FinishCategory uint8

const (
	FinishNone FinishCategory = iota
	FinishStop
	FinishToolCalls
	FinishLength
	FinishContentFilter
	FinishRefusal
	FinishPauseTurn
	FinishContextWindow
	FinishAborted
	FinishOverloaded
	FinishUnknown
)

// FinishReason carries normalized termination category, supplier raw reason, and
// optional matched stop sequence. It intentionally excludes HTTP status, billing
// usage, and credentials.
type FinishReason struct {
	Category     FinishCategory
	RawReason    string
	StopSequence Optional[string]
}

// Choice represents one candidate output branch in a non-streaming Response.
type Choice struct {
	Index    int
	Items    []Item
	Finish   FinishReason
	Logprobs json.RawMessage
}

// Response is the canonical representation of a non-streaming supplier response's
// protocol content. Raw billing usage and HTTP transport facts are observed
// separately outside this package.
type Response struct {
	ID          string
	CreatedAt   int64
	Model       string
	ServiceTier string
	Choices     []Choice
}

// EventKind enumerates normalized streaming content and lifecycle events.
type EventKind uint8

const (
	EventResponseStart EventKind = iota + 1
	EventTextDelta
	EventRefusalDelta
	EventReasoningDelta
	EventReasoningSignatureDelta
	EventReasoningRedacted
	EventToolCallStart
	EventToolCallArgsDelta
	EventContentBlockStop
	EventChoiceFinish
	EventProtocolDone
	EventPing
	EventProviderError
)

// Event represents a single normalized streaming event emitted by a supplier
// event decoder before public projection and client SSE serialization.
type Event struct {
	Kind             EventKind
	ChoiceIndex      int
	ItemIndex        int
	PartIndex        int
	ToolIndex        int
	ItemID           string
	CallID           string
	ID               string
	Model            string
	CreatedAt        int64
	ServiceTier      string
	Role             Role
	ToolName         string
	Text             string
	ThoughtSignature string
	SignatureOrigin  OpaqueOrigin
	Annotations      []json.RawMessage
	Logprobs         json.RawMessage
	Finish           FinishReason
	RawError         json.RawMessage
}
