package executionfacts

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func boolFact(v bool) *bool       { return &v }
func statusFact(v uint16) *uint16 { return &v }
func attemptFixture() Envelope {
	return Envelope{ProducerID: "proxy-e1", CallID: "call-1", AttemptID: "attempt-1",
		ReleaseID: "release-1", RouteID: "route-1", SalesAssetID: "asset-1", TenantID: "tenant-1", PrincipalID: "user-1", AccountID: "account-1", PrincipalKind: "user", BillingMode: "customer", OperationKind: "text_generate",
		OccurredAt: "2026-10-27T10:00:00Z", ObservedAt: "2026-10-27T10:00:01Z",
		TargetID: "target-1", BindingID: "binding-1", ProviderID: "provider-1", ProviderAccountID: "purchase-1", CredentialRef: "credential-ref-1", CredentialRevision: "1", UpstreamModel: "upstream-model-1",
		SelectionDigest: "sha256:" + strings.Repeat("a", 64), OutboundBodyDigest: "sha256:" + strings.Repeat("b", 64), SendTransport: "http_call", RecordingOwner: "ai_proxy", Mechanism: "router",
		Kind: "upstream_attempt", SendState: "observed_sent", Acceptance: "rejected_before_acceptance", ExecutionResult: "failed", HeadersComplete: boolFact(true), BodyComplete: boolFact(false), HTTPStatus: statusFact(503),
		Usage: MissingTokenUsage("provider_usage_missing"), EvidenceRefs: []string{"receipt-1"}}
}
func deliveryFixture() Envelope {
	e := Envelope{SchemaVersion: SchemaVersion, ProducerID: "billing-e1", EventID: "event-delivery-1", SourceFactID: "source-delivery-call-1", SourceRevision: "1", PayloadCanonicalization: Canonicalization,
		Kind: "call_delivery", CallID: "call-1", ReleaseID: "release-1", RouteID: "route-1", SalesAssetID: "asset-1", TenantID: "tenant-1", PrincipalID: "user-1", AccountID: "account-1", PrincipalKind: "user", BillingMode: "customer", OperationKind: "text_generate",
		OccurredAt: "2026-10-27T10:00:00Z", ObservedAt: "2026-10-27T10:00:01Z", EvidenceRefs: []string{}, DeliveryState: "succeeded", ServedFrom: "cache", PublicModel: "model<&\u2028",
		Usage: &Usage{Status: "reported", Unit: "token", Quantities: map[string]string{"input": "0", "output": "0"}, Source: "provider"}}
	return e
}
func mustFreeze(t *testing.T, e Envelope) Snapshot {
	t.Helper()
	s, err := Freeze(e)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestFreezeCanonicalBusinessGolden(t *testing.T) {
	s := mustFreeze(t, deliveryFixture())
	if s.Digest() != "sha256:18b6aa835470f77116d8a137f3490f503f9a1793ade8ff1553c4e863f894da2c" {
		t.Fatalf("digest differs from independent canonical golden: %s", s.Digest())
	}
	if bytes.Contains(s.Bytes(), []byte(`\u003c`)) || bytes.Contains(s.Bytes(), []byte(`\u2028`)) {
		t.Fatal("Unicode/HTML strings were altered")
	}
	if !bytes.Contains(s.Bytes(), []byte(`"evidence_refs":[]`)) {
		t.Fatal("empty evidence is not an array")
	}
	if bytes.Contains(s.Bytes(), []byte(`"attempt_id"`)) {
		t.Fatal("cache delivery fabricated a purchase attempt")
	}
}
func TestFreezeControlsAndDiagnosticProjection(t *testing.T) {
	e := attemptFixture()
	e.Usage.ReasonCode = "missing\nreason"
	e.RequestID = "request-a"
	a := mustFreeze(t, e)
	if !bytes.Contains(a.Bytes(), []byte(`missing\u000areason`)) {
		t.Fatal("controls must use frozen lowercase u00xx escaping")
	}
	e.RequestID = "request-b"
	e.TraceID = "trace-b"
	b := mustFreeze(t, e)
	if a.Digest() != b.Digest() || !bytes.Equal(a.BusinessBytes(), b.BusinessBytes()) {
		t.Fatal("diagnostics changed business digest")
	}
	if bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("original diagnostic fields were lost")
	}
}
func TestSnapshotDeepCopiesAndStableIDs(t *testing.T) {
	e := attemptFixture()
	e.Usage = &Usage{Status: "reported", Unit: "token", Source: "provider", Quantities: map[string]string{"input": "9007199254740994", "cache_hit": "1", "cache_miss": "9007199254740993", "output": "0"}}
	a := mustFreeze(t, e)
	b := mustFreeze(t, e)
	if a.EventID() == "" || a.SourceFactID() == "" || !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("same original snapshot is unstable")
	}
	original := a.Bytes()
	e.Usage.Quantities["input"] = "1"
	e.EvidenceRefs[0] = "mutated"
	*e.HTTPStatus = 200
	copied := a.Envelope()
	copied.Usage.Quantities["input"] = "2"
	copied.EvidenceRefs[0] = "changed"
	returned := a.Bytes()
	returned[0] = 'x'
	if !bytes.Equal(original, a.Bytes()) {
		t.Fatal("caller mutated frozen bytes")
	}
	if !bytes.Contains(original, []byte(`"9007199254740994"`)) {
		t.Fatal("exact large token count changed")
	}
}
func TestUsageMissingReportedZeroAndKnownZeroRemainDistinct(t *testing.T) {
	e := attemptFixture()
	missing := mustFreeze(t, e)
	if bytes.Contains(missing.Bytes(), []byte(`"quantities"`)) {
		t.Fatal("missing usage fabricated zero")
	}
	e.Usage = &Usage{Status: "reported", Unit: "token", Source: "provider", Quantities: map[string]string{"input": "0", "output": "0"}}
	reported := mustFreeze(t, e)
	e.Usage = &Usage{Status: "known_zero", Unit: "token", Source: "protocol_zero", ReasonCode: "protocol_explicit_zero", Quantities: map[string]string{"input": "0", "output": "0"}}
	zero := mustFreeze(t, e)
	if missing.Digest() == reported.Digest() || reported.Digest() == zero.Digest() {
		t.Fatal("usage evidence states collapsed")
	}
}
func TestLedgerConflictsAndCorrectionKeepOneLogicalAttempt(t *testing.T) {
	l := NewLedger(3)
	e := attemptFixture()
	first, err := l.Record(e)
	if err != nil {
		t.Fatal(err)
	}
	e.EventID = first.EventID()
	e.SourceFactID = first.SourceFactID()
	e.RequestID = "retry-diagnostic"
	same, err := l.Record(e)
	if err != nil || !bytes.Equal(same.Bytes(), first.Bytes()) {
		t.Fatal("redelivery did not preserve first bytes")
	}
	e.HTTPStatus = statusFact(502)
	if _, err := l.Record(e); !errors.Is(err, ErrConflict) {
		t.Fatalf("same key different content was not rejected: %v", err)
	}
	replacement := first.Envelope()
	replacement.ObservedAt = "2026-10-27T10:00:02Z"
	replacement.Usage = &Usage{Status: "reported", Unit: "token", Source: "provider", Quantities: map[string]string{"input": "4", "output": "2"}}
	next, err := Correct(first, replacement)
	if err != nil {
		t.Fatal(err)
	}
	if next.SourceFactID() != first.SourceFactID() || next.Revision() != "2" || next.EventID() == first.EventID() || next.Envelope().SupersedesEventID != first.EventID() {
		t.Fatal("correction created another logical purchase or lost predecessor")
	}
	replacement.OutboundBodyDigest = "sha256:" + strings.Repeat("c", 64)
	if _, err := Correct(first, replacement); !errors.Is(err, ErrConflict) {
		t.Fatal("correction changed original outbound body")
	}
	replacement.OutboundBodyDigest = first.Envelope().OutboundBodyDigest
	replacement.AccountID = "another-wallet"
	if _, err := Correct(first, replacement); !errors.Is(err, ErrConflict) {
		t.Fatal("correction changed original ownership")
	}
	if _, err := l.Retain(next); err != nil {
		t.Fatal(err)
	}
}
func TestAttemptSnapshotDoesNotInheritFinalUsageOrStatus(t *testing.T) {
	original := attemptFixture()
	a, err := NewAttempt(original)
	if err != nil {
		t.Fatal(err)
	}
	original.BindingID = "final-binding"
	original.UpstreamModel = "final-model"
	original.Usage = &Usage{Status: "reported", Unit: "token", Source: "provider", Quantities: map[string]string{"input": "999"}}
	failed, err := a.Finish(AttemptOutcome{ObservedAt: "2026-10-27T10:00:01Z", SendState: "handed_off", Acceptance: "unknown", ExecutionResult: "unknown"})
	if err != nil {
		t.Fatal(err)
	}
	e := failed.Envelope()
	if e.BindingID != "binding-1" || e.UpstreamModel != "upstream-model-1" || e.HTTPStatus != nil || e.Usage.Status != "missing" || e.Usage.Quantities != nil {
		t.Fatal("intermediate observation borrowed final execution facts")
	}
}
func TestFactRejectsContradictionsAndOverflow(t *testing.T) {
	cases := []func(*Envelope){
		func(e *Envelope) { e.HeadersComplete = boolFact(true); e.HTTPStatus = nil },
		func(e *Envelope) { e.BodyComplete = boolFact(true); e.ResponseBodyDigest = "" },
		func(e *Envelope) { e.SendState = "not_sent"; e.Acceptance = "accepted" },
		func(e *Envelope) { e.Acceptance = "accepted"; e.EvidenceRefs = nil },
		func(e *Envelope) { e.SendTransport = "router"; e.RecordingOwner = "ai_proxy" },
		func(e *Envelope) { e.Mechanism = "fallback"; e.ParentAttemptID = "" },
		func(e *Envelope) {
			e.Usage = &Usage{Status: "missing", Unit: "token", Source: "none", ReasonCode: "missing", Quantities: map[string]string{}}
		},
		func(e *Envelope) {
			e.Usage = &Usage{Status: "reported", Unit: "token", Source: "provider", Quantities: map[string]string{"input": "340282366920938463463374607431768211456"}}
		},
		func(e *Envelope) { e.SourceRevision = "9223372036854775808"; e.SupersedesEventID = "previous" },
		func(e *Envelope) { e.ObservedAt = "2026-10-27T12:00:00+02:00" },
	}
	for i, change := range cases {
		e := attemptFixture()
		change(&e)
		if _, err := Freeze(e); err == nil {
			t.Errorf("case %d accepted contradictory evidence", i)
		}
	}
	cache := deliveryFixture()
	cache.AttemptID = "invented"
	if _, err := Freeze(cache); err == nil {
		t.Fatal("cache created a purchase attempt")
	}
}

func registrationFixture(t *testing.T) Registration {
	t.Helper()
	selection := map[string]interface{}{"call_id": "call-1", "release_id": "release-1", "target_id": "target-1", "binding_id": "binding-1", "attempt_id": "attempt-1", "operation_kind": "text_generate", "provider_account_id": "purchase-1", "upstream_model": "upstream-model-1", "cluster_ref": "cluster-1", "credential_ref": "credential-ref-1", "credential_revision": "1", "selection_generation": "1", "selection_canonicalization": "selection-json.v1", "original_body_digest": "sha256:" + strings.Repeat("a", 64), "mapper_digest": "sha256:" + strings.Repeat("b", 64), "proxy_digest": "sha256:" + strings.Repeat("c", 64), "descriptor_digest": "sha256:" + strings.Repeat("d", 64), "mechanism": "router", "send_transport": "http_call", "recording_owner": "ai_proxy"}
	raw, _ := json.Marshal(selection)
	digest, err := SelectionDigest(raw)
	if err != nil {
		t.Fatal(err)
	}
	selection["selection_digest"] = digest
	raw, _ = json.Marshal(selection)
	return Registration{ProducerID: "proxy-e1", CallID: "call-1", AttemptID: "attempt-1", ReleaseID: "release-1", RouteID: "route-1", TargetID: "target-1", AccountID: "account-1", APIKeyID: "api-key-1", CredentialGeneration: "9007199254740993", BusinessOccurredAt: "2026-10-27T10:00:00Z", Selection: raw}
}
func TestRegistrationIsSeparateImmutableAndPreservesSelection(t *testing.T) {
	r := registrationFixture(t)
	a, err := FreezeRegistration(r)
	if err != nil {
		t.Fatal(err)
	}
	b, err := FreezeRegistration(r)
	if err != nil {
		t.Fatal(err)
	}
	if a.MessageType() != "execution_registration" || a.EventID() != b.EventID() || !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("registration identity/bytes are unstable")
	}
	if bytes.Contains(a.Bytes(), []byte(`"usage"`)) || bytes.Contains(a.Bytes(), []byte(`"history"`)) || bytes.Contains(a.Bytes(), []byte(`"payload_digest"`)) {
		t.Fatal("registration self-certified historical usage")
	}
	if !bytes.Contains(a.Bytes(), []byte(`"credential_generation":"9007199254740993"`)) {
		t.Fatal("key generation lost precision")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(a.Bytes(), &object); err != nil {
		t.Fatal(err)
	}
	expected, err := CanonicalSelection(r.Selection)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(object["selection"], expected) {
		t.Fatal("complete original selection changed")
	}
	r.Selection[0] = 'x'
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("registration retained mutable input")
	}
	l := NewLedger(4)
	if _, err := l.Retain(a); err != nil {
		t.Fatal(err)
	}
	changed := registrationFixture(t)
	changed.AccountID = "another-wallet"
	bad, err := FreezeRegistration(changed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.Retain(bad); !errors.Is(err, ErrConflict) {
		t.Fatal("same original registration was overwritten")
	}
}
func TestRegistrationRejectsChangedOrAmbiguousOriginalSelection(t *testing.T) {
	changes := []func(*Registration){
		func(r *Registration) { r.AttemptID = "another-attempt" },
		func(r *Registration) {
			r.Selection = []byte(strings.Replace(string(r.Selection), `"target_id":"target-1"`, `"target_id":"target-other"`, 1))
		},
		func(r *Registration) { r.Selection = []byte(`{"call_id":"evil",` + string(r.Selection[1:])) },
		func(r *Registration) {
			r.Selection = []byte(strings.Replace(string(r.Selection), `"upstream-model-1"`, `"\ud800"`, 1))
		},
		func(r *Registration) { r.Selection = append(r.Selection, []byte(` {}`)...) },
		func(r *Registration) { r.CredentialGeneration = "01" },
	}
	for i, change := range changes {
		r := registrationFixture(t)
		change(&r)
		if _, err := FreezeRegistration(r); err == nil {
			t.Errorf("registration case %d accepted ambiguous original", i)
		}
	}
}

func TestLedgerBoundedRetentionDoesNotDiscardFirstBytes(t *testing.T) {
	l := NewLedger(1)
	first, err := l.Record(attemptFixture())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.Record(deliveryFixture()); !errors.Is(err, ErrCapacity) {
		t.Fatal("per-call count limit was ignored")
	}
	same, err := l.Retain(first)
	if err != nil || !bytes.Equal(same.Bytes(), first.Bytes()) {
		t.Fatal("capacity prevented identical first-message redelivery")
	}
	byteLimited := NewLedger(128)
	byteLimited.retainedBytes = MaxRetainedBytes - 1
	if _, err := byteLimited.Retain(first); !errors.Is(err, ErrCapacity) {
		t.Fatal("per-call byte limit was ignored")
	}
}
