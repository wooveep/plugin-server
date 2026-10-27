package main

import (
	"bytes"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/resp"
	"testing"
)

type executionRecordingRedis struct {
	commands  [][]interface{}
	callbacks []wrapper.RedisResponseCallback
}

func (r *executionRecordingRedis) Command(cmd []interface{}, callback wrapper.RedisResponseCallback) error {
	r.commands = append(r.commands, append([]interface{}(nil), cmd...))
	r.callbacks = append(r.callbacks, callback)
	return nil
}
func TestExecutionRedisAdapterUsesFrozenObservationAndPrivateStream(t *testing.T) {
	ctx := newExecutionTestContext()
	if err := captureExecutionAttempt(ctx, executionIdentityFixture()); err != nil {
		t.Fatal(err)
	}
	snapshot, err := completeExecutionAttempt(ctx, "attempt-1", executionfacts.AttemptOutcome{ObservedAt: "2026-10-27T10:00:01Z", SendState: "handed_off", Acceptance: "unknown", ExecutionResult: "unknown"})
	if err != nil {
		t.Fatal(err)
	}
	redis := &executionRecordingRedis{}
	d, err := dispatchExecutionFact(ctx, redis, "private-execution-stream", snapshot)
	if err != nil {
		t.Fatal(err)
	}
	cmd := redis.commands[0]
	if len(cmd) != 7 || cmd[0] != "XADD" || cmd[1] != "private-execution-stream" || cmd[3] != "message_type" || cmd[4] != "observation" || cmd[5] != "event" || !bytes.Equal([]byte(cmd[6].(string)), snapshot.Bytes()) {
		t.Fatalf("unexpected private stream command: %#v", cmd)
	}
	if d.Status().State != "in_flight" {
		t.Fatal("event marked delivered before ACK")
	}
	redis.callbacks[0](resp.StringValue("123-0"))
	if d.Status().State != "acknowledged" {
		t.Fatal("stream insertion ACK not retained")
	}
	if _, err := dispatchExecutionFact(ctx, redis, "private-execution-stream", snapshot); err != nil || len(redis.commands) != 1 {
		t.Fatal("same acknowledged fact duplicated insertion")
	}
}
