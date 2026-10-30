package upstreamhealth

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
)

const gatewayOwner = "original-pod/original-boot"
const gatewayClaimID = "20000000-0000-0000-0000-000000000001"

type gatewayCallRecord struct {
	method, path string
	headers      [][2]string
	body         []byte
	callback     func(GatewayResponse)
}

type gatewayRecorder struct {
	calls []gatewayCallRecord
	err   error
}

func (r *gatewayRecorder) call(method, path string, headers [][2]string, body []byte, _ uint32, callback func(GatewayResponse)) error {
	r.calls = append(r.calls, gatewayCallRecord{method, path, headers, append([]byte(nil), body...), callback})
	return r.err
}

func gatewayClientFixture(t *testing.T, recorder *gatewayRecorder) *GatewayClient {
	t.Helper()
	client, err := NewGatewayClient(recorder.call, [][2]string{{"authorization", "Bearer controlled-service-token"},
		{"x-modelfusion-gateway-instance", gatewayOwner}}, gatewayOwner, 1000)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func gatewayReply(t *testing.T, value interface{}) GatewayResponse {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return GatewayResponse{StatusCode: 200, HeadersComplete: true, BodyComplete: true, Body: raw}
}

func gatewaySelection(t *testing.T) []byte {
	t.Helper()
	digest := "sha256:" + strings.Repeat("a", 64)
	fields := map[string]string{
		"call_id": "call-1", "release_id": "release-1", "target_id": "target:original", "binding_id": "binding:original",
		"attempt_id": "attempt-1", "operation_kind": "text_generate", "provider_account_id": "account:original",
		"upstream_model": "model:original", "cluster_ref": "outbound|443||provider.dns", "credential_ref": "upstream:original-key",
		"credential_revision": "1", "selection_generation": "9007199254740993", "selection_canonicalization": "selection-json.v1",
		"original_body_digest": digest, "mapper_digest": digest, "proxy_digest": digest, "descriptor_digest": digest,
		"mechanism": "router", "send_transport": "http_call", "recording_owner": "ai_proxy", "gateway_boot_id": gatewayOwner,
	}
	raw, _ := json.Marshal(fields)
	actual, err := executionfacts.SelectionDigest(raw)
	if err != nil {
		t.Fatal(err)
	}
	fields["selection_digest"] = actual
	raw, _ = json.Marshal(fields)
	return raw
}

func gatewayClaimRequest(t *testing.T) ClaimRequest {
	t.Helper()
	candidate, err := NewRecoveryCandidate(gatewaySelection(t), "runtime-1", "9007199254740993", nil)
	if err != nil {
		t.Fatal(err)
	}
	return ClaimRequest{ClaimRequestID: "original-command", OwnerInstance: gatewayOwner,
		ExpectedEpochs: map[string]Revision{"scope:model": "3"}, Candidate: candidate, CandidateDigest: candidate.Digest()}
}

func gatewayAllocation(request ClaimRequest) RecoveryClaim {
	now := time.Now().UTC()
	return RecoveryClaim{ClaimID: gatewayClaimID, Request: request, Actor: "service/original-ai-proxy",
		CredentialGeneration: "1", CredentialControlRevision: "9007199254740993", CredentialRef: "upstream:original-key",
		Scopes:    []ClaimedScope{{ScopeID: "scope:model", Epoch: "4", FailureGeneration: "2"}},
		CreatedAt: now.Format(time.RFC3339Nano), ExpiresAt: now.Add(time.Minute).Format(time.RFC3339Nano), Status: "allocated"}
}

func gatewayOutcome() OutcomeRequest {
	status := uint16(429)
	return OutcomeRequest{EventID: "health-original-event", ObservationPhase: "upstream_response", OutcomeEvidenceRef: "original-history/attempt-1",
		Selection: AttemptSelection{RuntimeConfigID: "runtime-1", ConfigRevision: "9007199254740993", TargetID: "target:original",
			CredentialRef: "upstream:original-key", CredentialGeneration: "1", OperationKind: "text_generate", CallID: "call-1", AttemptID: "attempt-1",
			SelectionDigest: "sha256:" + strings.Repeat("a", 64)},
		AffectedScopeIDs: []string{"scope:model"}, ObservedScopeRevisions: map[string]Revision{"scope:model": "4"},
		OccurredAt: "2026-10-29T00:00:01Z", HTTPStatus: &status, Effect: HealthEffect{Kind: "rate_limited"}}
}

func TestRecoveryCandidatePreservesOriginalSemanticsBeforeAndAfterClaim(t *testing.T) {
	request := gatewayClaimRequest(t)
	if bytes.Contains(request.Candidate.Bytes(), []byte(`"selection_digest"`)) || bytes.Contains(request.Candidate.Bytes(), []byte(`"task_id"`)) {
		t.Fatal("pre-claim candidate retained a final digest or fabricated a null task")
	}
	final, err := request.Candidate.FinalSelection(gatewayClaimID)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(final, &fields)
	if string(fields["recovery_claim_refs"]) != `["`+gatewayClaimID+`"]` || string(fields["selection_generation"]) != `"9007199254740993"` ||
		string(fields["gateway_boot_id"]) != `"`+gatewayOwner+`"` || fields["runtime_config_id"] != nil {
		t.Fatalf("final selection lost original semantics: %s", final)
	}
	other := request
	other.OwnerInstance = "replacement-boot"
	if other.validate(other.OwnerInstance) == nil {
		t.Fatal("claim survived a different startup owner")
	}
	raw := bytes.Replace(request.Candidate.Bytes(), []byte(`"runtime_config_id"`), []byte(`"task_id":null,"runtime_config_id"`), 1)
	var invalid RecoveryCandidate
	if json.Unmarshal(raw, &invalid) == nil {
		t.Fatal("null provider task was accepted")
	}
}

func TestUnknownClaimsAndOriginalReadbackNeverGrantAnotherSend(t *testing.T) {
	r := &gatewayRecorder{}
	client := gatewayClientFixture(t, r)
	request := gatewayClaimRequest(t)
	var result GatewayResult[ClaimReceipt]
	if err := client.Claim(request, func(value GatewayResult[ClaimReceipt]) { result = value }); err != nil {
		t.Fatal(err)
	}
	request.ExpectedEpochs["scope:model"] = "99"
	var original ClaimRequest
	if err := gatewayJSON(r.calls[0].body, &original); err != nil {
		t.Fatal(err)
	}
	r.calls[0].callback(GatewayResponse{StatusCode: 200, HeadersComplete: true, BodyComplete: false, Body: []byte(`{"decision":"granted"`)})
	if result.State != GatewayUnknown || result.Value != nil {
		t.Fatal("truncated allocation granted a send")
	}
	allocation := gatewayAllocation(original)
	if err := client.GetClaim(original, func(value GatewayResult[ClaimReceipt]) { result = value }); err != nil {
		t.Fatal(err)
	}
	r.calls[1].callback(gatewayReply(t, ClaimReceipt{Decision: "granted", Claim: &allocation}))
	if result.State != GatewayAcknowledged || result.Value.Claim.Status != "allocated" || len(r.calls) != 2 || strings.HasSuffix(r.calls[1].path, "/consume") {
		t.Fatal("original claim readback consumed or changed an allocation")
	}
	final, err := original.Candidate.FinalSelection(allocation.ClaimID)
	if err != nil {
		t.Fatal(err)
	}
	var consumed GatewayResult[RecoveryClaim]
	callbacks := 0
	if err := client.Consume(allocation, final, func(value GatewayResult[RecoveryClaim]) { consumed = value; callbacks++ }); err != nil {
		t.Fatal(err)
	}
	allocation.Status = "consumed"
	allocation.ConsumedSelection = final
	reply := gatewayReply(t, allocation)
	r.calls[2].callback(reply)
	r.calls[2].callback(reply)
	if consumed.State != GatewayAcknowledged || callbacks != 1 {
		t.Fatalf("first consume not confirmed once: %#v callbacks=%d", consumed, callbacks)
	}
	if err := client.Consume(allocation, final, func(GatewayResult[RecoveryClaim]) {}); err == nil {
		t.Fatal("consumed receipt permitted reuse")
	}
}

func TestFailuresRemainVetoedUntilTheirOwnDurableAcknowledgement(t *testing.T) {
	r := &gatewayRecorder{}
	client := gatewayClientFixture(t, r)
	first := gatewayOutcome()
	var last GatewayResult[OutcomeReceipt]
	if err := client.ReportOutcome(first, func(value GatewayResult[OutcomeReceipt]) { last = value }); err != nil {
		t.Fatal(err)
	}
	otherKey := fixtureTarget()
	otherKey.CredentialRef = "different-key-same-model-scope"
	if !client.Vetoes(otherKey) {
		t.Fatal("account/model cooldown lost its cross-key scope")
	}
	r.calls[0].callback(GatewayResponse{StatusCode: 503, HeadersComplete: true, BodyComplete: true})
	if last.State != GatewayUnknown || !client.Vetoes(otherKey) {
		t.Fatal("dependency failure cleared the local failure")
	}
	second := gatewayOutcome()
	second.EventID, second.OccurredAt = "newer-failure", "2026-10-29T00:00:02Z"
	if err := client.ReportOutcome(second, func(GatewayResult[OutcomeReceipt]) {}); err != nil {
		t.Fatal(err)
	}
	ack := func(id string) GatewayResponse {
		return gatewayReply(t, OutcomeReceipt{EventID: id, Revision: "9007199254740993", Decision: "applied",
			Scopes: []ScopeState{}, ReceivedAt: "2026-10-29T00:00:03Z"})
	}
	// Retry the same original, rather than creating a fresh observation time.
	pending := client.PendingOutcomes(fixtureTarget())
	if len(pending) != 2 {
		t.Fatalf("pending originals: %d", len(pending))
	}
	if err := client.ReportOutcome(first, func(GatewayResult[OutcomeReceipt]) {}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(r.calls[0].body, r.calls[2].body) {
		t.Fatal("retry rewrote original occurrence or evidence")
	}
	r.calls[2].callback(ack(first.EventID))
	if !client.Vetoes(otherKey) {
		t.Fatal("old ACK cleared a newer failure")
	}
	r.calls[1].callback(ack(second.EventID))
	if client.Vetoes(otherKey) {
		t.Fatal("both exact ACKs did not clear their own vetoes")
	}
	// An ACK is durable before Redis publication. Its newer watermark must fence
	// an otherwise valid older view even after both pending originals are gone.
	third := gatewayOutcome()
	third.EventID = "ack-before-projection"
	if err := client.ReportOutcome(third, func(GatewayResult[OutcomeReceipt]) {}); err != nil {
		t.Fatal(err)
	}
	newer := gatewayReply(t, OutcomeReceipt{EventID: third.EventID, Revision: "9007199254740994", Decision: "applied", Scopes: []ScopeState{}, ReceivedAt: "2026-10-29T00:00:03Z"})
	r.calls[3].callback(newer)
	if client.Vetoes(fixtureTarget()) || client.DecideNewCall(readyFixture(t, fixtureWire()), Candidate{Target: fixtureTarget()}, fixtureNow).ControlStatus != ReadStale {
		t.Fatal("durable ACK reopened an unpublished older control snapshot")
	}
	changed := first
	changed.EventID = "credential-revoked"
	changed.Effect = HealthEffect{Kind: "credential_invalid"}
	if err := client.ReportOutcome(changed, func(GatewayResult[OutcomeReceipt]) {}); err != nil {
		t.Fatal(err)
	}
	if client.Vetoes(otherKey) || !client.Vetoes(fixtureTarget()) {
		t.Fatal("credential revocation leaked into a different credential")
	}
}

func TestDispatchFailureAndMalformedAcknowledgementStayUnknown(t *testing.T) {
	r := &gatewayRecorder{err: errors.New("host call unavailable")}
	client := gatewayClientFixture(t, r)
	var result GatewayResult[OutcomeReceipt]
	if err := client.ReportOutcome(gatewayOutcome(), func(value GatewayResult[OutcomeReceipt]) { result = value }); err != nil {
		t.Fatal(err)
	}
	if result.State != GatewayUnknown || !client.Vetoes(fixtureTarget()) {
		t.Fatal("dispatch failure cleared a pending failure")
	}
	r.err = nil
	if err := client.ReportOutcome(gatewayOutcome(), func(value GatewayResult[OutcomeReceipt]) { result = value }); err != nil {
		t.Fatal(err)
	}
	r.calls[1].callback(GatewayResponse{StatusCode: 200, HeadersComplete: true, BodyComplete: true,
		Body: []byte(`{"event_id":"wrong","revision":"9007199254740993","decision":"applied","scopes":[],"received_at":"2026-10-29T00:00:03Z"}`)})
	if result.State != GatewayUnknown || !client.Vetoes(fixtureTarget()) {
		t.Fatal("another event's ACK cleared the original failure")
	}
}

func TestRecoverySuccessIsRetainedThroughHistoryArrivalWithoutInventingAFailure(t *testing.T) {
	r := &gatewayRecorder{}
	client := gatewayClientFixture(t, r)
	request := gatewayOutcome()
	request.EventID = "original-recovery-success"
	request.HTTPStatus = pointer(uint16(200))
	request.Effect = HealthEffect{Kind: "recovery_succeeded"}
	request.ClaimID = pointer(gatewayClaimID)
	if err := client.ReportOutcome(request, func(GatewayResult[OutcomeReceipt]) {}); err != nil {
		t.Fatal(err)
	}
	r.calls[0].callback(GatewayResponse{StatusCode: 503, HeadersComplete: true, BodyComplete: true, Body: []byte(`{"error":{"code":"upstream_health_history_pending"}}`)})
	pending := client.PendingOutcomes(fixtureTarget())
	if len(pending) != 1 || client.Vetoes(fixtureTarget()) {
		t.Fatal("original recovery success was dropped or turned into a failure")
	}
	if err := client.ReportOutcome(pending[0], func(GatewayResult[OutcomeReceipt]) {}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(r.calls[0].body, r.calls[1].body) {
		t.Fatal("history arrival retry changed the original success")
	}
	r.calls[1].callback(gatewayReply(t, OutcomeReceipt{EventID: request.EventID, Revision: "9007199254740993", Decision: "recovered", Scopes: []ScopeState{}, ReceivedAt: "2026-10-29T00:00:03Z"}))
	if len(client.PendingOutcomes(fixtureTarget())) != 0 {
		t.Fatal("durable recovery success ACK did not remove its own pending original")
	}
}

func TestOutcomeUsesRealAttemptAndDoesNotGuessCredentialInvalidity(t *testing.T) {
	selection := gatewayOutcome().Selection
	for _, status := range []uint16{429, 400, 401, 403, 503, 200} {
		e := executionfacts.Envelope{ProducerID: "original-proxy", Kind: "upstream_attempt", CallID: selection.CallID, AttemptID: selection.AttemptID,
			ReleaseID: "release-1", RouteID: "route-1", SalesAssetID: "asset-1", TenantID: "tenant-1", PrincipalID: "principal-1", AccountID: "wallet-1",
			PrincipalKind: "user", BillingMode: "customer", OperationKind: selection.OperationKind, OccurredAt: "2026-10-29T00:00:00Z", ObservedAt: "2026-10-29T00:00:01Z",
			TargetID: selection.TargetID, BindingID: "binding:original", ProviderID: "provider-1", ProviderAccountID: "account:original",
			CredentialRef: selection.CredentialRef, CredentialRevision: string(selection.CredentialGeneration), UpstreamModel: "model:original",
			SelectionDigest: selection.SelectionDigest, OutboundBodyDigest: "sha256:" + strings.Repeat("b", 64), SendTransport: "http_call", RecordingOwner: "ai_proxy", Mechanism: "router",
			SendState: "observed_sent", Acceptance: "unknown", ExecutionResult: "failed", HTTPStatus: &status, HeadersComplete: pointer(true), BodyComplete: pointer(true),
			ResponseBodyDigest: "sha256:" + strings.Repeat("c", 64),
			RetryAfter:         "2026-10-29T00:10:01Z", Usage: executionfacts.MissingTokenUsage("provider_usage_missing"), EvidenceRefs: []string{"original-attempt"}}
		if status == 200 {
			e.Acceptance, e.ExecutionResult = "accepted", "succeeded"
		}
		snapshot, err := executionfacts.Freeze(e)
		if err != nil {
			t.Fatal(err)
		}
		outcome, err := OutcomeFromAttempt(snapshot, selection, []string{"scope:model"}, map[string]Revision{"scope:model": "4"}, "original-history/attempt-1", nil)
		if err != nil {
			t.Fatal(err)
		}
		wanted := "neutral"
		if status == 429 {
			wanted = "rate_limited"
		} else if status == 200 {
			wanted = "recovery_succeeded"
		}
		if outcome.Effect.Kind != wanted || *outcome.HTTPStatus != status || outcome.OccurredAt != e.ObservedAt {
			t.Fatalf("status %d was reclassified: %#v", status, outcome)
		}
		if status == 429 && (outcome.Effect.RetryAfter == nil || *outcome.Effect.RetryAfter != "2026-10-29T00:10:01Z") {
			t.Fatal("provider's longer Retry-After was truncated")
		}
	}
}
