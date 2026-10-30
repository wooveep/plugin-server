package upstreamhealth

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
)

// AttemptSelection identifies the original frozen attempt, not a newly selected
// credential. It mirrors Console's upstream_health::AttemptSelection.
type AttemptSelection struct {
	RuntimeConfigID      string   `json:"runtime_config_id"`
	ConfigRevision       Revision `json:"config_revision"`
	TargetID             string   `json:"target_id"`
	CredentialRef        string   `json:"credential_ref"`
	CredentialGeneration Revision `json:"credential_generation"`
	OperationKind        string   `json:"operation_kind"`
	CallID               string   `json:"call_id"`
	AttemptID            string   `json:"attempt_id"`
	TaskID               *string  `json:"task_id"`
	SelectionDigest      string   `json:"selection_digest"`
}

// RecoveryCandidate is the exact D0 selection before the new claim exists,
// excluding selection_digest/recovery_claim_refs, plus the registered runtime
// mapping. Its digest is a candidate digest, never a final selection digest.
type RecoveryCandidate struct{ raw []byte }

func (c RecoveryCandidate) MarshalJSON() ([]byte, error) {
	if len(c.raw) == 0 {
		return nil, errors.New("health recovery candidate is absent")
	}
	return append([]byte(nil), c.raw...), nil
}

func (c *RecoveryCandidate) UnmarshalJSON(raw []byte) error {
	parsed, err := parseRecoveryCandidate(raw)
	if err == nil {
		*c = parsed
	}
	return err
}

func (c RecoveryCandidate) Bytes() []byte  { return append([]byte(nil), c.raw...) }
func (c RecoveryCandidate) Digest() string { return bodybasis.DigestBytes(c.raw) }

// NewRecoveryCandidate preserves every strict D0 field. Existing nonempty claim
// references cannot be recycled into another recovery command.
func NewRecoveryCandidate(selection []byte, runtimeConfigID string, configRevision Revision, taskID *string) (RecoveryCandidate, error) {
	digest, err := healthSelectionDigest(selection)
	if err != nil {
		return RecoveryCandidate{}, err
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(selection, &fields); err != nil {
		return RecoveryCandidate{}, err
	}
	if raw, exists := fields["selection_digest"]; exists {
		var supplied string
		if json.Unmarshal(raw, &supplied) != nil || supplied != digest {
			return RecoveryCandidate{}, errors.New("health original selection digest differs")
		}
	}
	if raw, exists := fields["recovery_claim_refs"]; exists {
		var refs []string
		if json.Unmarshal(raw, &refs) != nil || len(refs) != 0 {
			return RecoveryCandidate{}, errors.New("health candidate already belongs to recovery claims")
		}
	}
	delete(fields, "selection_digest")
	delete(fields, "recovery_claim_refs")
	fields["runtime_config_id"], _ = json.Marshal(runtimeConfigID)
	fields["config_revision"], _ = json.Marshal(configRevision)
	if taskID != nil {
		fields["task_id"], _ = json.Marshal(*taskID)
	}
	raw, err := json.Marshal(fields)
	if err != nil {
		return RecoveryCandidate{}, err
	}
	return parseRecoveryCandidate(raw)
}

func parseRecoveryCandidate(raw []byte) (RecoveryCandidate, error) {
	canonical, err := mfjson.Canonicalize(raw)
	if err != nil {
		return RecoveryCandidate{}, err
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(canonical, &fields) != nil || fields == nil {
		return RecoveryCandidate{}, errors.New("health candidate requires an object")
	}
	var runtimeID string
	var revision Revision
	var task *string
	var boot string
	if json.Unmarshal(fields["runtime_config_id"], &runtimeID) != nil || !identity(runtimeID) ||
		json.Unmarshal(fields["config_revision"], &revision) != nil ||
		json.Unmarshal(fields["gateway_boot_id"], &boot) != nil || !identity(boot) {
		return RecoveryCandidate{}, errors.New("health runtime mapping is invalid")
	}
	if _, err := ParseRevision(string(revision)); err != nil {
		return RecoveryCandidate{}, err
	}
	if raw, exists := fields["task_id"]; exists {
		if bytes.Equal(raw, []byte("null")) || json.Unmarshal(raw, &task) != nil || task == nil || !identity(*task) {
			return RecoveryCandidate{}, errors.New("health original task is invalid")
		}
	}
	delete(fields, "runtime_config_id")
	delete(fields, "config_revision")
	delete(fields, "task_id")
	if fields["selection_digest"] != nil || fields["recovery_claim_refs"] != nil {
		return RecoveryCandidate{}, errors.New("health candidate is not a final selection")
	}
	base, _ := json.Marshal(fields)
	if _, err := healthSelectionDigest(base); err != nil {
		return RecoveryCandidate{}, err
	}
	return RecoveryCandidate{raw: canonical}, nil
}

// FinalSelection adds only this newly allocated claim to the retained candidate.
// Constructing it is not a send grant; the first Consume must still commit.
func (c RecoveryCandidate) FinalSelection(claimID string) ([]byte, error) {
	if len(c.raw) == 0 || !claimUUID(claimID) {
		return nil, errors.New("health final selection requires an allocated claim")
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(c.raw, &fields)
	delete(fields, "runtime_config_id")
	delete(fields, "config_revision")
	delete(fields, "task_id")
	fields["recovery_claim_refs"], _ = json.Marshal([]string{claimID})
	raw, _ := json.Marshal(fields)
	digest, err := healthSelectionDigest(raw)
	if err != nil {
		return nil, err
	}
	fields["selection_digest"], _ = json.Marshal(digest)
	raw, _ = json.Marshal(fields)
	return canonicalHealthSelection(raw)
}

// Native:v2 embedding has its own registered selection schema. All other
// operations still take the original v1 codec and its unchanged rejections.
func embeddingHealthSelection(raw []byte) bool {
	var operation struct {
		OperationKind string `json:"operation_kind"`
	}
	return json.Unmarshal(raw, &operation) == nil && operation.OperationKind == executionfacts.NativeEmbeddingOperation
}

func healthSelectionDigest(raw []byte) (string, error) {
	if embeddingHealthSelection(raw) {
		return executionfacts.EmbeddingSelectionDigest(raw)
	}
	return executionfacts.SelectionDigest(raw)
}

func canonicalHealthSelection(raw []byte) ([]byte, error) {
	if embeddingHealthSelection(raw) {
		return executionfacts.CanonicalEmbeddingSelection(raw)
	}
	return executionfacts.CanonicalSelection(raw)
}

type ClaimRequest struct {
	ClaimRequestID  string              `json:"claim_request_id"`
	OwnerInstance   string              `json:"owner_instance"`
	ExpectedEpochs  map[string]Revision `json:"expected_epochs"`
	Candidate       RecoveryCandidate   `json:"candidate"`
	CandidateDigest string              `json:"candidate_digest"`
}

type ClaimedScope struct {
	ScopeID           string   `json:"scope_id"`
	Epoch             Revision `json:"epoch"`
	FailureGeneration Revision `json:"failure_generation"`
}

type RecoveryClaim struct {
	ClaimID                   string          `json:"claim_id"`
	Request                   ClaimRequest    `json:"request"`
	Actor                     string          `json:"actor"`
	CredentialGeneration      Revision        `json:"credential_generation"`
	CredentialControlRevision Revision        `json:"credential_control_revision"`
	CredentialRef             string          `json:"credential_ref"`
	Scopes                    []ClaimedScope  `json:"scopes"`
	CreatedAt                 string          `json:"created_at"`
	ExpiresAt                 string          `json:"expires_at"`
	Status                    string          `json:"status"`
	ConsumedSelection         json.RawMessage `json:"consumed_selection,omitempty"`
}

// ClaimReceipt retains the original allocation decision. A granted allocation
// or its GET replay is never permission to send a business request.
type ClaimReceipt struct {
	Decision string         `json:"decision"`
	Claim    *RecoveryClaim `json:"claim,omitempty"`
	Reason   *string        `json:"reason,omitempty"`
}

type HealthEffect struct {
	Kind               string  `json:"kind"`
	RetryAfter         *string `json:"retry_after,omitempty"`
	RetryAfterEvidence *string `json:"retry_after_evidence,omitempty"`
	ProviderTaskID     *string `json:"provider_task_id,omitempty"`
	Reason             *string `json:"reason,omitempty"`
}

type OutcomeRequest struct {
	EventID                string              `json:"event_id"`
	ObservationPhase       string              `json:"observation_phase"`
	Selection              AttemptSelection    `json:"selection"`
	AffectedScopeIDs       []string            `json:"affected_scope_ids"`
	ObservedScopeRevisions map[string]Revision `json:"observed_scope_revisions"`
	OutcomeEvidenceRef     string              `json:"outcome_evidence_ref"`
	OccurredAt             string              `json:"occurred_at"`
	HTTPStatus             *uint16             `json:"http_status"`
	Effect                 HealthEffect        `json:"effect"`
	ClaimID                *string             `json:"claim_id"`
}

type OutcomeReceipt struct {
	EventID    string       `json:"event_id"`
	Revision   Revision     `json:"revision"`
	Decision   string       `json:"decision"`
	Scopes     []ScopeState `json:"scopes"`
	ReceivedAt string       `json:"received_at"`
}

// OutcomeFromAttempt classifies only an immutable real upstream observation.
// Credentials and permanent denials require their existing explicit authority;
// an ordinary 400/401/403 cannot disable a credential here. The supplied scopes
// and evidence reference are the retained runtime/history mapping, not headers.
func OutcomeFromAttempt(snapshot executionfacts.Snapshot, selection AttemptSelection, affectedScopes []string, revisions map[string]Revision, evidenceRef string, claimID *string) (OutcomeRequest, error) {
	e := snapshot.Envelope()
	if e.Kind != "upstream_attempt" || e.EventID == "" || e.CallID != selection.CallID || e.AttemptID != selection.AttemptID ||
		e.TargetID != selection.TargetID || e.CredentialRef != selection.CredentialRef || e.CredentialRevision != string(selection.CredentialGeneration) ||
		e.OperationKind != selection.OperationKind || e.SelectionDigest != selection.SelectionDigest ||
		selection.TaskID == nil && e.ProviderTaskID != "" || selection.TaskID != nil && *selection.TaskID != e.ProviderTaskID {
		return OutcomeRequest{}, errors.New("health observation differs from its original attempt")
	}
	request := OutcomeRequest{
		EventID: "health:" + bodybasis.DigestBytes([]byte(e.EventID)), ObservationPhase: "upstream_response",
		Selection: selection, AffectedScopeIDs: append([]string{}, affectedScopes...),
		ObservedScopeRevisions: map[string]Revision{}, OutcomeEvidenceRef: evidenceRef,
		OccurredAt: e.ObservedAt, HTTPStatus: e.HTTPStatus, Effect: HealthEffect{Kind: "neutral"}, ClaimID: claimID,
	}
	for scope, revision := range revisions {
		request.ObservedScopeRevisions[scope] = revision
	}
	if e.SendState == "observed_sent" && e.HTTPStatus != nil {
		if *e.HTTPStatus == 429 {
			request.Effect = HealthEffect{Kind: "rate_limited"}
			if e.RetryAfter != "" {
				// IF03 has already frozen an absolute UTC deadline, not a raw
				// HTTP delta/date. Do not reinterpret it at delivery time.
				deadline, evidence := e.RetryAfter, "original_fact_retry_after"
				request.Effect.RetryAfter, request.Effect.RetryAfterEvidence = &deadline, &evidence
			}
		} else if *e.HTTPStatus >= 200 && *e.HTTPStatus < 300 && e.Acceptance == "accepted" &&
			e.ExecutionResult == "succeeded" && e.BodyComplete != nil && *e.BodyComplete {
			request.Effect = HealthEffect{Kind: "recovery_succeeded"}
		}
	}
	if err := request.validate(); err != nil {
		return OutcomeRequest{}, err
	}
	// Seal all caller-owned pointers/maps before an asynchronous callback can run.
	raw, err := json.Marshal(request)
	if err != nil {
		return OutcomeRequest{}, err
	}
	var frozen OutcomeRequest
	if err := gatewayJSON(raw, &frozen); err != nil {
		return OutcomeRequest{}, err
	}
	return frozen, nil
}

func sameJSON(left, right interface{}) bool {
	a, err := json.Marshal(left)
	if err != nil {
		return false
	}
	b, err := json.Marshal(right)
	if err != nil {
		return false
	}
	a, err = mfjson.Canonicalize(a)
	if err != nil {
		return false
	}
	b, err = mfjson.Canonicalize(b)
	return err == nil && bytes.Equal(a, b)
}

func validTime(value string) bool { _, ok := utc(value); return ok }

func (r ClaimRequest) validate(owner string) error {
	if !identity(r.ClaimRequestID) || !identity(owner) || r.OwnerInstance != owner || len(r.Candidate.raw) == 0 ||
		r.CandidateDigest != r.Candidate.Digest() || r.ExpectedEpochs == nil || len(r.ExpectedEpochs) > 128 {
		return errors.New("health recovery request differs from its original owner or candidate")
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(r.Candidate.raw, &fields)
	var boot string
	if json.Unmarshal(fields["gateway_boot_id"], &boot) != nil || boot != owner {
		return errors.New("health recovery requires the authenticated gateway startup")
	}
	for scope, epoch := range r.ExpectedEpochs {
		if !identity(scope) {
			return errors.New("health recovery scope is invalid")
		}
		if _, err := ParseRevision(string(epoch)); err != nil {
			return err
		}
	}
	return nil
}

func (s AttemptSelection) validate() error {
	for _, value := range []string{s.RuntimeConfigID, s.TargetID, s.CredentialRef, s.OperationKind, s.CallID, s.AttemptID} {
		if !identity(value) {
			return errors.New("health original attempt identity is incomplete")
		}
	}
	if s.TaskID != nil && !identity(*s.TaskID) {
		return errors.New("health original task is invalid")
	}
	for _, revision := range []Revision{s.ConfigRevision, s.CredentialGeneration} {
		if _, err := ParseRevision(string(revision)); err != nil {
			return err
		}
	}
	if !validDigest(s.SelectionDigest) {
		return errors.New("health original selection digest is invalid")
	}
	return nil
}

func (r OutcomeRequest) validate() error {
	if err := r.Selection.validate(); err != nil {
		return err
	}
	if !identity(r.EventID) || !identity(r.ObservationPhase) || !identity(r.OutcomeEvidenceRef) || !validTime(r.OccurredAt) ||
		r.AffectedScopeIDs == nil || r.ObservedScopeRevisions == nil || len(r.AffectedScopeIDs) > 128 || len(r.ObservedScopeRevisions) > 128 {
		return errors.New("health original outcome is incomplete")
	}
	if r.HTTPStatus != nil && (*r.HTTPStatus < 100 || *r.HTTPStatus > 599) {
		return errors.New("health upstream HTTP status is invalid")
	}
	if r.ClaimID != nil && !claimUUID(*r.ClaimID) {
		return errors.New("health claim reference is invalid")
	}
	seen := map[string]bool{}
	for _, scope := range r.AffectedScopeIDs {
		if !identity(scope) || seen[scope] {
			return errors.New("health outcome scopes are invalid")
		}
		seen[scope] = true
	}
	for scope, revision := range r.ObservedScopeRevisions {
		if !identity(scope) {
			return errors.New("health observed scope is invalid")
		}
		if _, err := ParseRevision(string(revision)); err != nil {
			return err
		}
	}
	e := r.Effect
	if e.RetryAfter != nil && !validTime(*e.RetryAfter) {
		return errors.New("health provider deadline is invalid")
	}
	if e.RetryAfterEvidence != nil && !identity(*e.RetryAfterEvidence) {
		return errors.New("health deadline evidence is invalid")
	}
	if e.Kind != "rate_limited" && (e.RetryAfter != nil || e.RetryAfterEvidence != nil) ||
		e.Kind != "recovery_accepted" && e.ProviderTaskID != nil || e.Kind != "permanently_blocked" && e.Reason != nil {
		return errors.New("health effect contains fields from another classification")
	}
	switch e.Kind {
	case "rate_limited":
		if r.HTTPStatus == nil || *r.HTTPStatus != 429 || len(r.AffectedScopeIDs) == 0 {
			return errors.New("health rate limit requires the actual upstream 429 and registered scopes")
		}
	case "recovery_succeeded", "neutral", "credential_invalid":
	case "recovery_accepted":
		if e.ProviderTaskID == nil || !identity(*e.ProviderTaskID) {
			return errors.New("health acceptance requires the original provider task")
		}
	case "permanently_blocked":
		if e.Reason == nil || !identity(*e.Reason) || len(r.AffectedScopeIDs) == 0 {
			return errors.New("health permanent block requires a classified reason and registered scopes")
		}
	default:
		return errors.New("health effect is not supported")
	}
	return nil
}

func (c RecoveryClaim) validate(request ClaimRequest) error {
	if !claimUUID(c.ClaimID) || !sameJSON(c.Request, request) || !identity(c.Actor) || !identity(c.CredentialRef) ||
		!validTime(c.CreatedAt) || !validTime(c.ExpiresAt) || len(c.Scopes) == 0 || len(c.Scopes) > 128 {
		return errors.New("health claim does not bind the original request")
	}
	created, _ := utc(c.CreatedAt)
	expires, _ := utc(c.ExpiresAt)
	if !expires.After(created) {
		return errors.New("health claim lifetime is invalid")
	}
	for _, revision := range []Revision{c.CredentialGeneration, c.CredentialControlRevision} {
		if _, err := ParseRevision(string(revision)); err != nil {
			return err
		}
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(request.Candidate.raw, &fields)
	var credential string
	var generation Revision
	if json.Unmarshal(fields["credential_ref"], &credential) != nil || credential != c.CredentialRef ||
		json.Unmarshal(fields["credential_revision"], &generation) != nil || generation != c.CredentialGeneration {
		return errors.New("health claim changed the original credential")
	}
	seen := map[string]bool{}
	for _, scope := range c.Scopes {
		if !identity(scope.ScopeID) || seen[scope.ScopeID] {
			return errors.New("health claim repeats a scope")
		}
		if _, ok := request.ExpectedEpochs[scope.ScopeID]; !ok {
			return errors.New("health claim contains an unrequested scope")
		}
		seen[scope.ScopeID] = true
		for _, revision := range []Revision{scope.Epoch, scope.FailureGeneration} {
			if _, err := ParseRevision(string(revision)); err != nil {
				return err
			}
		}
	}
	return nil
}

func claimUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}
