package executionfacts

import (
	"encoding/json"
	"errors"
	"time"
)

// RuntimeTelemetry is operational evidence, separate from immutable billing facts.
// It does not authorize a call or contribute money or token usage.
type RuntimeTelemetry struct {
	SchemaVersion     string `json:"schema_version"`
	ProducerID        string `json:"producer_id"`
	Kind              string `json:"kind"`
	CallID            string `json:"call_id"`
	AttemptID         string `json:"attempt_id,omitempty"`
	RequestID         string `json:"request_id,omitempty"`
	UpstreamRequestID string `json:"upstream_request_id,omitempty"`
	SentAt            string `json:"sent_at,omitempty"`
	DurationMS        *int64 `json:"duration_ms,omitempty"`
	FirstByteMS       *int64 `json:"first_byte_ms,omitempty"`
	ObservedAt        string `json:"observed_at"`
}

// FreezeRuntimeTelemetry retains one immutable operational observation for the existing private outbox.
func FreezeRuntimeTelemetry(value RuntimeTelemetry) (Snapshot, error) {
	value.SchemaVersion = "execution-telemetry.v1"
	if !id(value.ProducerID) || !id(value.CallID) || (value.Kind != "attempt" && value.Kind != "call") || value.Kind == "attempt" && !id(value.AttemptID) {
		return Snapshot{}, errors.New("invalid execution telemetry identity")
	}
	if _, err := time.Parse(time.RFC3339Nano, value.ObservedAt); err != nil {
		return Snapshot{}, errors.New("invalid telemetry observation time")
	}
	if value.SentAt != "" {
		if _, err := time.Parse(time.RFC3339Nano, value.SentAt); err != nil {
			return Snapshot{}, errors.New("invalid actual send time")
		}
	}
	if value.FirstByteMS != nil && (*value.FirstByteMS < 0 || value.Kind != "attempt" || value.SentAt == "") {
		return Snapshot{}, errors.New("invalid upstream first response time")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return Snapshot{}, err
	}
	eventID := stableID("runtime-telemetry", value.ProducerID, value.Kind, value.CallID, value.AttemptID, value.ObservedAt)
	return Snapshot{wire: raw, business: raw, messageType: "execution_telemetry", envelope: Envelope{ProducerID: value.ProducerID, EventID: eventID, SourceFactID: eventID, SourceRevision: "1", PayloadDigest: digestBytes(raw)}}, nil
}
