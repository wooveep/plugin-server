package executionfacts

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRuntimeTelemetryPreservesActualSendWithoutBillingUsage(t *testing.T) {
	firstByte := int64(0)
	value := RuntimeTelemetry{ProducerID: "producer-1", Kind: "attempt", CallID: "call-1", AttemptID: "attempt-1", RequestID: "gateway-original", UpstreamRequestID: "upstream-original", SentAt: "2026-11-12T00:00:00Z", FirstByteMS: &firstByte, ObservedAt: "2026-11-12T00:00:01Z"}
	snapshot, err := FreezeRuntimeTelemetry(value)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(snapshot.wire, &wire); err != nil {
		t.Fatal(err)
	}
	if string(wire["sent_at"]) != `"2026-11-12T00:00:00Z"` || string(wire["first_byte_ms"]) != "0" || wire["usage"] != nil || snapshot.messageType != "execution_telemetry" {
		t.Fatal("operational evidence changed or introduced billing usage")
	}
	value.SentAt = ""
	value.FirstByteMS = nil
	snapshot, err = FreezeRuntimeTelemetry(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(snapshot.wire, &wire); err != nil {
		t.Fatal(err)
	}
	var absent RuntimeTelemetry
	if err := json.Unmarshal(snapshot.wire, &absent); err != nil || absent.SentAt != "" || absent.FirstByteMS != nil {
		t.Fatal("missing send time must remain unknown")
	}
	value.SentAt = "2026-11-12T00:00:00Z"
	firstByte = -1
	value.FirstByteMS = &firstByte
	if _, err := FreezeRuntimeTelemetry(value); err == nil {
		t.Fatal("negative upstream timing must be rejected at the boundary")
	}
}

func TestRuntimeTelemetryClientMetadataBoundary(t *testing.T) {
	for _, tc := range []struct {
		name    string
		info    ClientInfo
		invalid bool
	}{
		{"missing", ClientInfo{}, false},
		{"direct", ClientInfo{ClientIP: "2001:db8::7", ClientIPSource: ClientIPDownstreamConnection, UserAgent: "client/1.0"}, false},
		{"portal-unknown", ClientInfo{ClientIPSource: ClientIPUnconfirmedPortal}, false},
		{"bad-ip", ClientInfo{ClientIP: "forged", ClientIPSource: ClientIPDownstreamConnection}, true},
		{"unproven-ip", ClientInfo{ClientIP: "192.0.2.1", ClientIPSource: ClientIPUnconfirmedPortal}, true},
		{"long-agent", ClientInfo{UserAgent: strings.Repeat("x", MaxUserAgentBytes+1)}, true},
		{"control-agent", ClientInfo{UserAgent: "client\r\nsecret"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := RuntimeTelemetry{ClientInfo: tc.info, ProducerID: "producer-1", Kind: "call", CallID: "call-1", ObservedAt: "2026-11-13T00:00:00Z"}
			snapshot, err := FreezeRuntimeTelemetry(value)
			if (err != nil) != tc.invalid {
				t.Fatalf("invalid=%t, error=%v", tc.invalid, err)
			}
			if err == nil {
				var decoded RuntimeTelemetry
				if err := json.Unmarshal(snapshot.Bytes(), &decoded); err != nil || decoded.ClientInfo != tc.info {
					t.Fatal("original client evidence was changed")
				}
			}
		})
	}
}
