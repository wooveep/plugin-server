package main

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/serviceidentity"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

const managedInternalCallHeader = executionfacts.InternalCallReferenceHeader
const managedInternalCheckPath = "/internal/internal-execution/check"

type managedInternalCall struct {
	identity                     serviceidentity.Snapshot
	ref, apiKeyID, generation    string
	checkStarted, checkCompleted bool
	receipt                      *managedInternalReceipt
}
type managedInternalRequest struct {
	CallRef              string `json:"call_ref"`
	ReleaseID            string `json:"release_id"`
	RouteID              string `json:"route_id"`
	APIKeyID             string `json:"api_key_id"`
	CredentialGeneration string `json:"credential_generation"`
	OriginalBodyDigest   string `json:"original_body_digest"`
}
type managedInternalCaller struct {
	BillingMode string                   `json:"billing_mode"`
	TenantID    string                   `json:"tenant_id"`
	PrincipalID string                   `json:"principal_id"`
	CostOwner   executionfacts.CostOwner `json:"cost_owner"`
}
type managedInternalReceipt struct {
	Allowed              bool                  `json:"allowed"`
	CallID               string                `json:"call_id"`
	ReleaseID            string                `json:"release_id"`
	RouteID              string                `json:"route_id"`
	APIKeyID             string                `json:"api_key_id"`
	CredentialGeneration string                `json:"credential_generation"`
	Caller               managedInternalCaller `json:"caller"`
	OperationKind        string                `json:"operation_kind"`
	PublicModel          string                `json:"public_model"`
	BusinessOccurredAt   string                `json:"business_occurred_at"`
	OriginalBodyDigest   string                `json:"original_body_digest"`
	TargetIDs            []string              `json:"target_ids"`
	ValidUntil           string                `json:"valid_until"`
}

// These values come from actual key-auth and the server-prepared job request.
// The reference is only a lookup key; it grants no internal-cost authority.
func readManagedInternalCall(headers [][2]string) (*managedInternalCall, error) {
	original, err := executionfacts.ReadInternalCallReference(headers)
	if err != nil {
		return nil, err
	}
	return &managedInternalCall{ref: original.CallID, apiKeyID: original.APIKeyID, generation: original.CredentialGeneration, identity: original.Identity}, nil
}

func decodeManagedInternalReceipt(raw []byte, request managedInternalRequest, configuration *managedExecutionConfig, basis bodybasis.Snapshot, operation string, now time.Time) (managedInternalReceipt, error) {
	var receipt managedInternalReceipt
	if decodeManagedJSON(raw, &receipt, true) != nil || !receipt.Allowed || receipt.CallID != request.CallRef || receipt.ReleaseID != request.ReleaseID || receipt.RouteID != request.RouteID || receipt.APIKeyID != request.APIKeyID || receipt.CredentialGeneration != request.CredentialGeneration || receipt.OriginalBodyDigest != request.OriginalBodyDigest || receipt.OriginalBodyDigest != basis.Digest() || receipt.PublicModel != configuration.release.PublicModel || receipt.PublicModel != basis.PublicModel() || receipt.OperationKind != operation {
		return receipt, errors.New("internal execution receipt differs from the original request")
	}
	caller := receipt.Caller
	if caller.BillingMode != "internal_cost" || !balanceidentity.ValidID(caller.TenantID) || !strings.HasPrefix(caller.PrincipalID, "service/") || !managedID(caller.PrincipalID) || (caller.CostOwner.Kind != "platform" && caller.CostOwner.Kind != "tenant") || !managedID(caller.CostOwner.OwnerID) || !managedID(caller.CostOwner.SourceJobID) {
		return receipt, errors.New("internal execution receipt has no original service cost owner")
	}
	occurred, occurredErr := time.Parse(time.RFC3339Nano, receipt.BusinessOccurredAt)
	deadline, deadlineErr := time.Parse(time.RFC3339Nano, receipt.ValidUntil)
	_, occurredOffset := occurred.Zone()
	_, deadlineOffset := deadline.Zone()
	if occurredErr != nil || deadlineErr != nil || occurredOffset != 0 || deadlineOffset != 0 || !now.Before(deadline) || !occurred.Before(deadline) {
		return receipt, errors.New("internal execution receipt has no current original admission")
	}
	seen := map[string]bool{}
	selected := false
	for _, target := range receipt.TargetIDs {
		if !managedID(target) || seen[target] {
			return receipt, errors.New("internal execution receipt has an invalid frozen target directory")
		}
		seen[target] = true
		selected = selected || target == configuration.release.Descriptor.TargetID
	}
	if !selected {
		return receipt, errors.New("actual internal target was not in the original admitted job")
	}
	return receipt, nil
}

// A single read-only native RPC checks the already committed job/call. Neither
// retrying an unknown check under a new reference nor a client cost-owner header
// can create another call. The callback may proceed only in this live context.
func checkManagedInternalCall(ctx wrapper.HttpContext, state *managedExecutionState, basis bodybasis.Snapshot, operation string, ready func()) {
	original := state.internal
	if original == nil || original.checkStarted || managedTextStopped(state) {
		return
	}
	original.checkStarted = true
	health := state.config.settings.UpstreamHealth
	if health == nil {
		rejectManagedRequest(ctx, 503, "internal_execution_endpoint_missing")
		return
	}
	endpoint := health.Console
	request := managedInternalRequest{CallRef: original.ref, ReleaseID: state.config.release.Descriptor.ReleaseID, RouteID: state.config.release.RouteID, APIKeyID: original.apiKeyID, CredentialGeneration: original.generation, OriginalBodyDigest: basis.Digest()}
	body, err := json.Marshal(request)
	if err != nil {
		rejectManagedRequest(ctx, 503, "internal_execution_request_invalid")
		return
	}
	cluster := wrapper.FQDNCluster{FQDN: endpoint.ServiceName, Port: int64(endpoint.ServicePort), Host: endpoint.Host}
	headers := [][2]string{{":method", "POST"}, {":scheme", "http"}, {":authority", endpoint.Host}, {":path", managedInternalCheckPath}, {"authorization", "Bearer " + endpoint.AuthToken}, {"content-type", "application/json"}, {"accept", "application/json"}}
	_, err = proxywasm.DispatchHttpCall(cluster.ClusterName(), headers, body, nil, endpoint.TimeoutMS, func(count, size, _ int) {
		if original.checkCompleted || managedTextStopped(state) || state.dispatchStarted {
			return
		}
		original.checkCompleted = true
		response := readHealthGatewayResponse(count, size, 128*1024)
		if !response.HeadersComplete || !response.BodyComplete || response.StatusCode != 200 {
			rejectManagedRequest(ctx, 503, "internal_execution_check_unconfirmed")
			return
		}
		receipt, err := decodeManagedInternalReceipt(response.Body, request, state.config, basis, operation, time.Now())
		if err != nil {
			rejectManagedRequest(ctx, 403, "internal_execution_not_authorized")
			return
		}
		if receipt.Caller.TenantID != original.identity.TenantID || receipt.Caller.PrincipalID != original.identity.QualifiedPrincipal() {
			rejectManagedRequest(ctx, 403, "internal_execution_service_mismatch")
			return
		}
		original.receipt = &receipt
		state.callID, state.businessUTC = receipt.CallID, receipt.BusinessOccurredAt
		state.admissionDeadline, _ = time.Parse(time.RFC3339Nano, receipt.ValidUntil)
		if proxywasm.ReplaceHttpRequestHeader(managedCallHeader, state.callID) != nil {
			rejectManagedRequest(ctx, 500, "execution_identity_install_failed")
			return
		}
		ctx.SetUserAttribute("managed_call_id", state.callID)
		ready()
	})
	if err != nil && !original.checkCompleted {
		original.checkCompleted = true
		rejectManagedRequest(ctx, 503, "internal_execution_check_unconfirmed")
	}
}

// The static SDK asset is catalogue attribution only on an internal route. The
// original service/job caller comes solely from the authenticated Console check.
func setManagedOriginalCaller(state *managedExecutionState, envelope *executionfacts.Envelope) error {
	if state.internal == nil {
		return nil
	}
	receipt := state.internal.receipt
	if receipt == nil || !receipt.Allowed || receipt.CallID != state.callID || receipt.OperationKind != managedOperation(state.config) {
		return errors.New("original internal call receipt is absent")
	}
	authorizedTarget := false
	for _, target := range receipt.TargetIDs {
		authorizedTarget = authorizedTarget || target == state.config.release.Descriptor.TargetID
	}
	if !authorizedTarget {
		return errors.New("actual internal attempt is outside the original job target set")
	}
	owner := receipt.Caller.CostOwner
	envelope.TenantID, envelope.PrincipalID = receipt.Caller.TenantID, receipt.Caller.PrincipalID
	envelope.PrincipalKind, envelope.BillingMode, envelope.CostOwner = "service", "internal_cost", &owner
	envelope.AccountID, envelope.SalesAssetID = "", ""
	return nil
}

func managedRegistration(state *managedExecutionState, basis bodybasis.Snapshot) (executionfacts.Snapshot, error) {
	configuration := state.config
	if state.internal != nil {
		if state.internal.receipt == nil {
			return executionfacts.Snapshot{}, errors.New("original internal call check is absent")
		}
		return executionfacts.FreezeInternalRegistration(executionfacts.InternalRegistration{
			ProducerID: configuration.settings.ProducerID, CallID: state.callID, AttemptID: state.attemptID,
			ReleaseID: configuration.release.Descriptor.ReleaseID, RouteID: configuration.release.RouteID, TargetID: configuration.release.Descriptor.TargetID,
			APIKeyID: state.internal.apiKeyID, CredentialGeneration: state.internal.generation,
			BusinessOccurredAt: state.businessUTC, Selection: stateSelection(state, basis),
		})
	}
	return executionfacts.FreezeRegistration(executionfacts.Registration{
		ProducerID: configuration.settings.ProducerID, CallID: state.callID, AttemptID: state.attemptID,
		ReleaseID: configuration.release.Descriptor.ReleaseID, RouteID: configuration.release.RouteID, TargetID: configuration.release.Descriptor.TargetID,
		AccountID: state.identity.AccountID, APIKeyID: state.identity.APIKeyID, CredentialGeneration: state.identity.CredentialGeneration,
		BusinessOccurredAt: state.businessUTC, Selection: stateSelection(state, basis),
	})
}
