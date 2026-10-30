package upstreamhealth

import (
	"encoding/json"
	"time"
)

const SnapshotPath = "/internal/upstream-health/snapshot"

// DecodeSnapshot feeds the original closed HTTP document through the same
// complete hash decoder, digest, source, schema and freshness checks as Redis.
// It is a value boundary only, not a send grant or a new authority. Call the
// shared GatewayClient.DecideNewCall to join its local veto/read/ACK watermarks.
func DecodeSnapshot(raw []byte, config Config, now time.Time) ReadResult {
	if len(raw) > MaxSnapshotBytes {
		return ReadResult{Status: ReadInvalid}
	}
	var metadata map[string]json.RawMessage
	if json.Unmarshal(raw, &metadata) != nil || metadata == nil {
		return ReadResult{Status: ReadInvalid}
	}
	fields := make([]string, 8)
	for index, key := range map[int]string{0: "authority_id", 1: "revision", 2: "state_digest", 4: "observed_at", 5: "valid_until", 6: "projection_schema"} {
		if json.Unmarshal(metadata[key], &fields[index]) != nil {
			return ReadResult{Status: ReadInvalid}
		}
	}
	fields[3] = string(raw)
	var complete bool
	if json.Unmarshal(metadata["complete"], &complete) != nil {
		return ReadResult{Status: ReadInvalid}
	}
	fields[7] = "false"
	if complete {
		fields[7] = "true"
	}
	return decodeHash(fields, config, now)
}

// Snapshot is a thin authenticated read. It does not allocate or consume a
// recovery claim, enable a key, or replace the original media /claim authority.
func (c *GatewayClient) Snapshot(config Config, callback func(ReadResult)) error {
	if _, err := config.SnapshotKey(); err != nil {
		return err
	}
	if callback == nil {
		return nil
	}
	return callGatewayWithBudget(c, "GET", SnapshotPath, nil, func(result GatewayResult[ReadResult]) {
		if result.State != GatewayAcknowledged || result.Value == nil {
			callback(ReadResult{Status: ReadRedisUnavailable})
			return
		}
		callback(*result.Value)
	}, func(body []byte) (*ReadResult, error) {
		value := DecodeSnapshot(body, config, time.Now())
		return &value, nil
	}, false, MaxSnapshotBytes)
}
