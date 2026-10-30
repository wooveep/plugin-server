package mediaexecution

import (
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"strings"
)

// NewAttempt captures this actual HTTP handoff before the host call. A task GET
// keeps its own task_query identity; it cannot become another generation cost.
func (r Request) NewAttempt(producerID string) (executionfacts.Attempt, error) {
	var h historyWire
	if decode(r.history, &h) != nil || h.Attempt == nil || h.Attempt.Writer.ProducerID != producerID {
		return executionfacts.Attempt{}, ErrIdentity
	}
	caller, err := object(h.Caller)
	if err != nil {
		return executionfacts.Attempt{}, err
	}
	selection, err := object(r.selection)
	if err != nil {
		return executionfacts.Attempt{}, err
	}
	d, err := object(r.descriptor)
	if err != nil {
		return executionfacts.Attempt{}, err
	}
	operation, mechanism := r.profile.operation, "router"
	if r.query {
		operation, mechanism = "task_query", "task_query"
	}
	return executionfacts.NewAttempt(executionfacts.Envelope{
		ProducerID: producerID, CallID: h.CallID, OperationID: h.OperationID, AttemptID: r.attemptID,
		ReleaseID: h.ReleaseID, RouteID: h.RouteID, PublicModel: h.PublicModel, OccurredAt: h.OccurredAt,
		BillingMode: "customer", PrincipalKind: "user", TenantID: field(caller, "tenant_id"),
		PrincipalID: field(caller, "principal_id"), AccountID: field(caller, "account_id"), SalesAssetID: field(caller, "sales_asset_id"),
		TargetID: field(d, "target_id"), BindingID: field(d, "binding_id"), ProviderID: field(d, "provider_id"),
		ProviderAccountID: field(d, "provider_account_id"), UpstreamModel: field(d, "upstream_model"),
		CredentialRef: h.Attempt.CredentialRef, CredentialRevision: h.Attempt.CredentialRevision,
		SelectionDigest: field(selection, "selection_digest"), ProviderTaskID: r.taskID,
		OutboundBodyDigest: r.OutboundBodyDigest(), SendTransport: "http_call", RecordingOwner: "ai_proxy",
		OperationKind: operation, Mechanism: mechanism,
	})
}

// GenerationObservation appends actual task evidence to the original creation
// source. The predecessor is the complete original Console snapshot. A restart
// or unknown GET never creates revision one for an already consumed generation.
// Actual query transport evidence remains in its independent NewAttempt result.
func (r Request) GenerationObservation(actual executionfacts.Snapshot, observation Observation, previous *executionfacts.Snapshot) (executionfacts.Snapshot, error) {
	if !r.query {
		if actual.Envelope().AttemptID != r.createAttemptID || (previous != nil && previous.SourceFactID() != actual.SourceFactID()) {
			return executionfacts.Snapshot{}, ErrIdentity
		}
		return actual, nil
	}
	if previous == nil || observation.GenerationAcceptance == "unknown" || observation.GenerationResult == "unknown" {
		return executionfacts.Snapshot{}, ErrIdentity
	}
	old := previous.Envelope()
	var h historyWire
	if decode(r.history, &h) != nil || h.Attempt == nil || old.Kind != "upstream_attempt" ||
		old.CallID != r.callID || old.OperationID != r.operationID || old.AttemptID != r.createAttemptID ||
		old.SelectionDigest != h.Attempt.SelectionDigest || old.ProviderTaskID != r.taskID ||
		old.OperationKind != r.profile.operation || old.ProducerID != h.Attempt.Writer.ProducerID {
		return executionfacts.Snapshot{}, ErrIdentity
	}
	caller, err := object(h.Caller)
	descriptor, descriptorErr := object(r.descriptor)
	if err != nil || descriptorErr != nil || old.ReleaseID != h.ReleaseID || old.RouteID != h.RouteID || old.PublicModel != h.PublicModel ||
		old.OccurredAt != h.OccurredAt || old.BillingMode != "customer" || old.PrincipalKind != "user" ||
		old.AccountID != field(caller, "account_id") || old.TenantID != field(caller, "tenant_id") ||
		old.PrincipalID != field(caller, "principal_id") || old.SalesAssetID != field(caller, "sales_asset_id") ||
		old.TargetID != h.Attempt.TargetID || old.BindingID != field(descriptor, "binding_id") || old.ProviderID != field(descriptor, "provider_id") ||
		old.ProviderAccountID != field(descriptor, "provider_account_id") || old.UpstreamModel != field(descriptor, "upstream_model") ||
		old.CredentialRef != h.Attempt.CredentialRef || old.CredentialRevision != h.Attempt.CredentialRevision {
		return executionfacts.Snapshot{}, ErrIdentity
	}
	// Preserve the create HTTP status, original body digests and all ownership.
	// A complete read contributes cumulative task evidence, never a second POST.
	old.ObservedAt = actual.Envelope().ObservedAt
	old.Acceptance, old.ExecutionResult = observation.GenerationAcceptance, observation.GenerationResult
	if observation.GenerationUsage.Status != "missing" {
		old.Usage = observation.GenerationUsage
	}
	old.EvidenceRefs = append(old.EvidenceRefs, "query-attempt:"+r.attemptID+":"+actual.Digest())
	return executionfacts.Correct(*previous, old)
}

// DeliveryObservation is a separate logical call source, owned only by the
// delivery writer already sealed in the original history. A task acknowledgement
// is unknown delivery; complete actual output evidence determines the result.
func (r Request) DeliveryObservation(generation executionfacts.Snapshot, observation Observation, previous *executionfacts.Snapshot) (executionfacts.Snapshot, error) {
	var h historyWire
	if decode(r.history, &h) != nil || h.DeliveryWriter.ProducerID != generation.ProducerID() || h.DeliveryWriter.Role != "ai_proxy" {
		return executionfacts.Snapshot{}, ErrIdentity
	}
	g := generation.Envelope()
	delivery := executionfacts.Envelope{Kind: "call_delivery", ProducerID: g.ProducerID, CallID: g.CallID, OperationID: g.OperationID,
		ReleaseID: g.ReleaseID, RouteID: g.RouteID, SalesAssetID: g.SalesAssetID, TenantID: g.TenantID, PrincipalID: g.PrincipalID,
		AccountID: g.AccountID, PrincipalKind: g.PrincipalKind, BillingMode: g.BillingMode, OperationKind: g.OperationKind,
		OccurredAt: g.OccurredAt, ObservedAt: g.ObservedAt, PublicModel: g.PublicModel, ServedFrom: "upstream", DeliveryState: observation.DeliveryState,
		Usage: observation.GenerationUsage, EvidenceRefs: []string{"generation-fact:" + generation.EventID() + ":" + generation.Digest()}}
	if r.query {
		found := false
		for _, ref := range g.EvidenceRefs {
			if strings.HasPrefix(ref, "query-attempt:"+r.attemptID+":") {
				delivery.EvidenceRefs = append(delivery.EvidenceRefs, ref)
				found = true
			}
		}
		if !found {
			return executionfacts.Snapshot{}, ErrIdentity
		}
	}
	if previous != nil {
		old := previous.Envelope()
		if old.Kind != "call_delivery" || old.CallID != delivery.CallID || old.OperationID != delivery.OperationID || old.ProducerID != delivery.ProducerID {
			return executionfacts.Snapshot{}, ErrIdentity
		}
		if delivery.Usage.Status == "missing" {
			delivery.Usage = old.Usage
		}
		return executionfacts.Correct(*previous, delivery)
	}
	return executionfacts.Freeze(delivery)
}

// DecodeOriginalObservation validates the complete retained FACT wire, including
// its supplied digest, before it can be the predecessor of a query correction.
func DecodeOriginalObservation(raw []byte) (executionfacts.Snapshot, error) {
	var envelope executionfacts.Envelope
	if err := decode(raw, &envelope); err != nil {
		return executionfacts.Snapshot{}, err
	}
	// Decode at this transport boundary once; Freeze reuses the original codec's
	// identity, integer, usage and digest checks rather than a second FACT dialect.
	if envelope.PayloadDigest == "" || envelope.SourceFactID == "" ||
		envelope.SourceRevision == "" || envelope.EventID == "" || (envelope.Kind != "upstream_attempt" && envelope.Kind != "call_delivery") {
		return executionfacts.Snapshot{}, ErrIdentity
	}
	return executionfacts.Freeze(envelope)
}
