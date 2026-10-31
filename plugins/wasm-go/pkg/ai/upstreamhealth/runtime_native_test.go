//go:build !wasm

package upstreamhealth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/resp"
)

// This adapter executes the production Lua against an owned real Redis. The SDK
// transport is covered separately by the ai-proxy host callback check.
type runtimeRedis struct {
	address         string
	mu              sync.Mutex
	dropMutationAck bool
}

func (c *runtimeRedis) Ready() bool { return true }
func (c *runtimeRedis) exchange(args []interface{}) (resp.Value, error) {
	connection, err := net.DialTimeout("tcp", c.address, 5*time.Second)
	if err != nil {
		return resp.Value{}, err
	}
	defer connection.Close()
	if err = connection.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return resp.Value{}, err
	}
	values := make([]resp.Value, len(args))
	for i, argument := range args {
		values[i] = resp.StringValue(fmt.Sprint(argument))
	}
	if err = resp.NewWriter(connection).WriteArray(values); err != nil {
		return resp.Value{}, err
	}
	value, _, err := resp.NewReader(connection).ReadValue()
	if err == nil {
		err = value.Error()
	}
	return value, err
}
func (c *runtimeRedis) Command(args []interface{}, callback wrapper.RedisResponseCallback) error {
	value, err := c.exchange(args)
	if err != nil {
		return err
	}
	c.mu.Lock()
	drop := c.dropMutationAck && len(args) > 1 && args[0] == "EVAL" && args[1] == mutateRuntime
	if drop {
		c.dropMutationAck = false
	}
	c.mu.Unlock()
	if drop {
		return errors.New("controlled original mutation ACK loss")
	}
	callback(value)
	return nil
}
func (c *runtimeRedis) HMGet(key string, fields []string, callback wrapper.RedisResponseCallback) error {
	args := []interface{}{"HMGET", key}
	for _, field := range fields {
		args = append(args, field)
	}
	return c.Command(args, callback)
}
func (c *runtimeRedis) Eval(script string, count int, keys, args []interface{}, callback wrapper.RedisResponseCallback) error {
	command := []interface{}{"EVAL", script, count}
	command = append(command, keys...)
	command = append(command, args...)
	return c.Command(command, callback)
}

type runtimeHarness struct {
	client              *runtimeRedis
	first, second       *Runtime
	wire                snapshotWire
	key, stream, prefix string
}

func newRuntimeHarness(t *testing.T) *runtimeHarness {
	t.Helper()
	address := os.Getenv("MODELFUSION_HEALTH_TEST_REDIS_ADDR")
	if address == "" {
		t.Skip("requires owned MODELFUSION_HEALTH_TEST_REDIS_ADDR")
	}
	prefix := "mf-health-native:" + uuid.NewString()
	client := &runtimeRedis{address: address}
	projection := fixtureConfig()
	projection.Prefix = prefix
	config := RuntimeConfig{Projection: projection, Stream: prefix + ":execution", ProducerID: "native-health-test", OwnerInstance: gatewayOwner,
		Policy: RuntimePolicy{BackoffSeconds: []uint32{1, 2}, ClaimLeaseSeconds: 1, InconclusiveWaitSeconds: 1, ClockToleranceSeconds: 30}}
	first, err := NewRuntime(client, config)
	if err != nil {
		t.Fatal(err)
	}
	config.OwnerInstance = "second-pod/second-boot"
	second, err := NewRuntime(client, config)
	if err != nil {
		t.Fatal(err)
	}
	wire := fixtureWire()
	wire.Revision = "2"
	wire.Credentials[0].Revision = "1"
	wire.Scopes[0].Revision = "0"
	wire.Scopes[0].FailureGeneration = "0"
	wire.Scopes[0].RecoveryEpoch = "0"
	h := &runtimeHarness{client: client, first: first, second: second, wire: wire, key: first.reader.key, stream: config.Stream, prefix: prefix}
	h.seed(t)
	t.Cleanup(func() {
		value, err := client.exchange([]interface{}{"KEYS", prefix + ":*"})
		if err != nil {
			t.Error(err)
			return
		}
		args := []interface{}{"DEL"}
		for _, key := range value.Array() {
			args = append(args, key.String())
		}
		if len(args) > 1 {
			if _, err = client.exchange(args); err != nil {
				t.Error(err)
			}
		}
	})
	return h
}
func (h *runtimeHarness) seed(t *testing.T) {
	t.Helper()
	args := []interface{}{"HSET", h.key}
	fields := hashFixture(t, h.wire)
	for i, name := range HashFields() {
		args = append(args, name, fields[i])
	}
	if _, err := h.client.exchange(args); err != nil {
		t.Fatal(err)
	}
}
func runtimeRead(t *testing.T, r *Runtime) ReadResult {
	t.Helper()
	var result ReadResult
	r.Lookup(func(value ReadResult) { result = value })
	if result.Status != ReadReady {
		t.Fatalf("actual runtime lookup: %s", result.Status)
	}
	return result
}
func (h *runtimeHarness) decision(t *testing.T, r *Runtime) Decision {
	t.Helper()
	return r.DecideNewCall(runtimeRead(t, r), Candidate{Target: fixtureTarget()}, time.Now())
}
func runtimeOutcome(event, attempt string) OutcomeRequest {
	request := gatewayOutcome()
	request.EventID = event
	request.Selection.AttemptID = attempt
	request.ObservedScopeRevisions["scope:model"] = "0"
	request.OccurredAt = time.Now().UTC().Format(time.RFC3339Nano)
	return request
}
func reportRuntime(t *testing.T, r *Runtime, request OutcomeRequest) GatewayResult[OutcomeReceipt] {
	t.Helper()
	var result GatewayResult[OutcomeReceipt]
	if err := r.ReportOutcome(request, fixtureTarget(), func(value GatewayResult[OutcomeReceipt]) { result = value }); err != nil {
		t.Fatal(err)
	}
	return result
}
func runtimeRequest(t *testing.T, r *Runtime, command string) ClaimRequest {
	t.Helper()
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(gatewaySelection(t), &fields); err != nil {
		t.Fatal(err)
	}
	delete(fields, "selection_digest")
	fields["gateway_boot_id"], _ = json.Marshal(r.config.OwnerInstance)
	raw, _ := json.Marshal(fields)
	candidate, err := NewRecoveryCandidate(raw, "runtime-1", "9007199254740993", nil)
	if err != nil {
		t.Fatal(err)
	}
	decision := r.DecideNewCall(runtimeRead(t, r), Candidate{Target: fixtureTarget()}, time.Now())
	return ClaimRequest{ClaimRequestID: command, OwnerInstance: r.config.OwnerInstance, ExpectedEpochs: decision.ScopeEpochs, Candidate: candidate, CandidateDigest: candidate.Digest()}
}
func consumeRuntime(t *testing.T, r *Runtime, request ClaimRequest) GatewayResult[RecoveryClaim] {
	t.Helper()
	var result GatewayResult[RecoveryClaim]
	if err := r.ClaimConsumed(request, fixtureTarget(), func(value GatewayResult[RecoveryClaim]) { result = value }); err != nil {
		t.Fatal(err)
	}
	return result
}
func (h *runtimeHarness) expireBase(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano)
	h.wire.Scopes[0].BlockedUntil = &deadline
	h.seed(t)
}
func successFromClaim(t *testing.T, claim RecoveryClaim, event string) OutcomeRequest {
	t.Helper()
	var selected struct{ CallID, AttemptID, SelectionDigest string }
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(claim.ConsumedSelection, &fields); err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(fields["call_id"], &selected.CallID)
	_ = json.Unmarshal(fields["attempt_id"], &selected.AttemptID)
	_ = json.Unmarshal(fields["selection_digest"], &selected.SelectionDigest)
	request := runtimeOutcome(event, selected.AttemptID)
	request.Selection.CallID = selected.CallID
	request.Selection.SelectionDigest = selected.SelectionDigest
	request.ClaimID = &claim.ClaimID
	status := uint16(200)
	request.HTTPStatus = &status
	request.Effect = HealthEffect{Kind: "recovery_succeeded"}
	return request
}

func TestRuntimeNativeRedisBusinessPaths(t *testing.T) {
	t.Run("429_cross_vm_replay_and_expiry", func(t *testing.T) {
		h := newRuntimeHarness(t)
		if h.decision(t, h.first).Kind != Eligible {
			t.Fatal("persisted healthy state required Console")
		}
		request := runtimeOutcome("original-429", "first-attempt")
		if reportRuntime(t, h.first, request).State != GatewayAcknowledged || h.decision(t, h.second).Kind != Denied {
			t.Fatal("429 did not reach the other VM")
		}
		if reportRuntime(t, h.first, request).State != GatewayAcknowledged {
			t.Fatal("original replay lost receipt")
		}
		count, err := h.client.exchange([]interface{}{"XLEN", h.stream})
		if err != nil || count.Integer() != 1 {
			t.Fatal("replay applied the outcome twice", err, count)
		}
		time.Sleep(1100 * time.Millisecond)
		if h.decision(t, h.second).Kind != RecoveryRequired {
			t.Fatal("cooldown expiry implicitly opened health")
		}
	})
	t.Run("two_vms_one_consume_and_same_attempt_never_renews", func(t *testing.T) {
		h := newRuntimeHarness(t)
		h.expireBase(t)
		requests := []ClaimRequest{runtimeRequest(t, h.first, "first-command"), runtimeRequest(t, h.second, "other-command")}
		results := make(chan GatewayResult[RecoveryClaim], 2)
		for i, r := range []*Runtime{h.first, h.second} {
			go func(r *Runtime, request ClaimRequest) {
				err := r.ClaimConsumed(request, fixtureTarget(), func(v GatewayResult[RecoveryClaim]) { results <- v })
				if err != nil {
					results <- GatewayResult[RecoveryClaim]{State: GatewayRejected, Reason: err.Error()}
				}
			}(r, requests[i])
		}
		granted := 0
		var claim RecoveryClaim
		for range 2 {
			if result := <-results; result.State == GatewayAcknowledged {
				granted++
				claim = *result.Value
			}
		}
		if granted != 1 {
			t.Fatalf("actual Redis permitted %d simultaneous original sends", granted)
		}
		owner := h.first
		if claim.Request.OwnerInstance == h.second.config.OwnerInstance {
			owner = h.second
		}
		time.Sleep(1100 * time.Millisecond)
		changed := runtimeRequest(t, owner, "changed-command-same-attempt")
		if consumeRuntime(t, owner, changed).State == GatewayAcknowledged {
			t.Fatal("changing command key renewed the original attempt")
		}
	})
	t.Run("lost_ack_veto_survives_new_control_until_original_replay", func(t *testing.T) {
		h := newRuntimeHarness(t)
		h.client.dropMutationAck = true
		request := runtimeOutcome("lost-ack", "lost-ack-attempt")
		if reportRuntime(t, h.first, request).State != GatewayUnknown || len(h.first.PendingOutcomes(fixtureTarget())) != 1 {
			t.Fatal("lost ACK discarded original outcome")
		}
		h.wire.Revision = "3"
		h.seed(t)
		if h.decision(t, h.first).Reason != "local_health_failure_pending" {
			t.Fatal("unrelated control revision cleared the local veto")
		}
		h.first.FlushPending()
		if len(h.first.PendingOutcomes(fixtureTarget())) != 0 || h.decision(t, h.second).Kind != Denied {
			t.Fatal("original replay failed to clear only its ACKed veto")
		}
	})
	t.Run("new_failure_and_revocation_fence_old_success", func(t *testing.T) {
		h := newRuntimeHarness(t)
		h.expireBase(t)
		claim := consumeRuntime(t, h.first, runtimeRequest(t, h.first, "recovery-command"))
		if claim.State != GatewayAcknowledged {
			t.Fatal(claim)
		}
		if reportRuntime(t, h.second, runtimeOutcome("new-429", "different-attempt")).State != GatewayAcknowledged {
			t.Fatal("new 429 failed")
		}
		if reportRuntime(t, h.first, successFromClaim(t, *claim.Value, "old-success")).State != GatewayAcknowledged || h.decision(t, h.second).Kind != Denied {
			t.Fatal("old success opened a newer failure")
		}
		h.wire.Credentials[0].Enabled = false
		h.wire.Credentials[0].Revision = "2"
		h.wire.Revision = "3"
		h.seed(t)
		if h.decision(t, h.first).Reason != "credential_unavailable" {
			t.Fatal("temporary health enabled a revoked credential")
		}
	})
	t.Run("future_clock_is_bounded_without_discarding_original_failure", func(t *testing.T) {
		h := newRuntimeHarness(t)
		request := runtimeOutcome("future-clock", "future-attempt")
		request.OccurredAt = time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339Nano)
		retry := time.Now().Add(365 * 24 * time.Hour).UTC().Format(time.RFC3339Nano)
		request.Effect.RetryAfter = &retry
		result := reportRuntime(t, h.first, request)
		if result.State != GatewayAcknowledged || result.Value.Scopes[0].BlockedUntil == nil {
			t.Fatal("anomalous failure was discarded")
		}
		until, _ := utc(*result.Value.Scopes[0].BlockedUntil)
		if until.After(time.Now().Add(2*time.Second)) || h.decision(t, h.second).Kind != Denied {
			t.Fatal("future clock created an unbounded or absent denial")
		}
	})
	t.Run("invalid_consumed_selection_cannot_poison_pending", func(t *testing.T) {
		h := newRuntimeHarness(t)
		h.expireBase(t)
		claim := consumeRuntime(t, h.first, runtimeRequest(t, h.first, "valid-claim"))
		if claim.State != GatewayAcknowledged {
			t.Fatal(claim)
		}
		request := successFromClaim(t, *claim.Value, "wrong-selection")
		request.Selection.AttemptID = "foreign-attempt"
		if err := h.first.ReportOutcome(request, fixtureTarget(), func(GatewayResult[OutcomeReceipt]) {}); err == nil || len(h.first.PendingOutcomes(fixtureTarget())) != 0 {
			t.Fatal("invalid original installed a veto")
		}
	})
	t.Run("manual_scope_clear_does_not_clear_temporary_429", func(t *testing.T) {
		h := newRuntimeHarness(t)
		if reportRuntime(t, h.first, runtimeOutcome("original-temporary", "temporary-attempt")).State != GatewayAcknowledged {
			t.Fatal("429 failed")
		}
		reason := "account_debt"
		h.wire.Scopes[0].PermanentReason = &reason
		h.wire.Scopes[0].Revision = "1"
		h.wire.Scopes[0].FailureGeneration = "1"
		h.wire.Revision = "3"
		h.seed(t)
		if h.decision(t, h.second).Reason != "scope_permanently_blocked" {
			t.Fatal("explicit denial was not authoritative")
		}
		h.wire.Scopes[0].PermanentReason = nil
		h.wire.Scopes[0].Revision = "2"
		h.wire.Revision = "4"
		h.seed(t)
		if h.decision(t, h.second).Reason != "scope_cooling_down" {
			t.Fatal("manual permanent clear opened an unrelated temporary failure")
		}
	})
	t.Run("wrong_stream_type_cannot_partially_consume", func(t *testing.T) {
		h := newRuntimeHarness(t)
		h.expireBase(t)
		if _, err := h.client.exchange([]interface{}{"SET", h.stream, "wrong-type"}); err != nil {
			t.Fatal(err)
		}
		if consumeRuntime(t, h.first, runtimeRequest(t, h.first, "no-stream")).State == GatewayAcknowledged {
			t.Fatal("unrecorded consume granted a send")
		}
		value, err := h.client.exchange([]interface{}{"HGET", h.key, "runtime_revision"})
		if err != nil || !value.IsNull() {
			t.Fatal("failed append partially changed runtime", err, value)
		}
	})
}

func TestRuntimeNativeArchiveFixture(t *testing.T) {
	h := newRuntimeHarness(t)
	directory := os.Getenv("MODELFUSION_HEALTH_RUNTIME_EVENT_DIR")
	if directory == "" {
		t.Skip("optional Go/Rust original event handoff requires MODELFUSION_HEALTH_RUNTIME_EVENT_DIR")
	}
	old := runtimeOutcome("archived-429", "prior-attempt")
	old.OccurredAt = time.Now().Add(-2 * time.Second).UTC().Format(time.RFC3339Nano)
	if reportRuntime(t, h.first, old).State != GatewayAcknowledged {
		t.Fatal("past original 429 failed")
	}
	claim := consumeRuntime(t, h.first, runtimeRequest(t, h.first, "archived-original-claim"))
	if claim.State != GatewayAcknowledged {
		t.Fatal(claim)
	}
	if reportRuntime(t, h.first, successFromClaim(t, *claim.Value, "archived-success")).State != GatewayAcknowledged {
		t.Fatal("original recovery success failed")
	}
	last := runtimeOutcome("archived-final-429", "final-attempt")
	deadline := time.Now().Add(3 * time.Minute).UTC().Format(time.RFC3339Nano)
	last.Effect.RetryAfter = &deadline
	if reportRuntime(t, h.first, last).State != GatewayAcknowledged {
		t.Fatal("final failure failed")
	}
	value, err := h.client.exchange([]interface{}{"XRANGE", h.stream, "-", "+"})
	if err != nil {
		t.Fatal(err)
	}
	var events []json.RawMessage
	for _, entry := range value.Array() {
		fields := entry.Array()[1].Array()
		for i := 0; i < len(fields); i += 2 {
			if fields[i].String() == "event" {
				events = append(events, json.RawMessage(fields[i+1].String()))
			}
		}
	}
	if len(events) != 4 {
		t.Fatal("unexpected original health record count", len(events))
	}
	seed := hashFixture(t, h.wire)
	wire := json.RawMessage(seed[3])
	raw, err := json.MarshalIndent(struct {
		Snapshot json.RawMessage   `json:"snapshot"`
		Target   TargetReference   `json:"target"`
		Events   []json.RawMessage `json:"events"`
	}{wire, fixtureTarget(), events}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(directory, "original-go-runtime.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Log("original real Redis health messages", strconv.Itoa(len(events)))
}
