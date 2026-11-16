package executionfacts

import (
	"encoding/json"
	"testing"
)

func TestRuntimeTelemetryPreservesActualSendWithoutBillingUsage(t *testing.T) {
	value := RuntimeTelemetry{ProducerID: "producer-1", Kind: "attempt", CallID: "call-1", AttemptID: "attempt-1", RequestID: "gateway-original", UpstreamRequestID: "upstream-original", SentAt: "2026-11-12T00:00:00Z", ObservedAt: "2026-11-12T00:00:01Z"}
	snapshot, err := FreezeRuntimeTelemetry(value)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(snapshot.wire, &wire); err != nil {
		t.Fatal(err)
	}
	if string(wire["sent_at"]) != `"2026-11-12T00:00:00Z"` || wire["usage"] != nil || snapshot.messageType != "execution_telemetry" {
		t.Fatal("operational evidence changed or introduced billing usage")
	}
	value.SentAt = ""
	snapshot, err = FreezeRuntimeTelemetry(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(snapshot.wire, &wire); err != nil {
		t.Fatal(err)
	}
	var absent RuntimeTelemetry
	if err := json.Unmarshal(snapshot.wire, &absent); err != nil || absent.SentAt != "" {
		t.Fatal("missing send time must remain unknown")
	}
}
