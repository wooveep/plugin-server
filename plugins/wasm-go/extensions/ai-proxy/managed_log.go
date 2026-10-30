package main

import (
	"encoding/json"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

// Envoy access logging reads wasm.modelfusion_execution_log:PLAIN. This separate
// escaped-JSON slot never serializes the wrapper's general user attributes.
const managedAccessLogProperty = "modelfusion_execution_log"
const managedAccessLogContext = "ai-proxy.native-access-log.v1"

// Access logs retain a bounded ordered prefix, with an explicit overflow flag.
// The original FACT/selection chain remains the complete execution authority.
const managedAccessLogAttempts = 64
const managedAccessLogReasons = 8

type managedAccessAttempt struct {
	AttemptID       string `json:"attempt_id"`
	ParentAttemptID string `json:"parent_attempt_id,omitempty"`
}

type managedAccessLog struct {
	CallID            string                 `json:"call_id,omitempty"`
	ReleaseID         string                 `json:"release_id,omitempty"`
	Attempts          []managedAccessAttempt `json:"attempts,omitempty"`
	FactGap           bool                   `json:"fact_gap"`
	Rejected          bool                   `json:"rejected"`
	ReasonCodes       []string               `json:"reason_codes,omitempty"`
	AttemptsTruncated bool                   `json:"attempts_truncated,omitempty"`
	closed            bool
}

func beginManagedAccessLog(ctx wrapper.HttpContext, releaseID string) {
	if ctx.GetContext(managedAccessLogContext) != nil || !managedID(releaseID) {
		return
	}
	ctx.SetContext(managedAccessLogContext, &managedAccessLog{ReleaseID: releaseID})
	writeManagedAccessLog(ctx)
}

func managedAccessOrigin(ctx wrapper.HttpContext) *managedAccessLog {
	entry, ok := ctx.GetContext(managedAccessLogContext).(*managedAccessLog)
	if !ok || entry == nil || entry.closed {
		return nil
	}
	if state, ok := ctx.GetContext(managedExecutionContextKey).(*managedExecutionState); ok && state != nil && state.config != nil && state.config.release.Descriptor.ReleaseID == entry.ReleaseID && managedID(state.callID) {
		if entry.CallID == "" || entry.CallID == state.callID {
			entry.CallID = state.callID
		}
	}
	return entry
}

func recordManagedAccessAttempt(ctx wrapper.HttpContext, state *managedExecutionState) {
	entry := managedAccessOrigin(ctx)
	if entry == nil || state == nil || state.config == nil || !state.prepared || !state.dispatchStarted || entry.CallID == "" || entry.CallID != state.callID || entry.ReleaseID != state.config.release.Descriptor.ReleaseID || !managedID(state.attemptID) {
		return
	}
	for _, original := range entry.Attempts {
		if original.AttemptID == state.attemptID {
			return
		}
	}
	if len(entry.Attempts) == managedAccessLogAttempts {
		entry.AttemptsTruncated = true
	} else {
		entry.Attempts = append(entry.Attempts, managedAccessAttempt{AttemptID: state.attemptID, ParentAttemptID: state.parentAttemptID})
	}
	writeManagedAccessLog(ctx)
}

func recordManagedAccessFailure(ctx wrapper.HttpContext, reason string, gap bool) {
	entry := managedAccessOrigin(ctx)
	if entry == nil {
		return
	}
	if gap {
		entry.FactGap = true
	} else {
		entry.Rejected = true
	}
	code := safeManagedAccessReason(reason)
	for _, previous := range entry.ReasonCodes {
		if previous == code {
			writeManagedAccessLog(ctx)
			return
		}
	}
	if len(entry.ReasonCodes) < managedAccessLogReasons {
		entry.ReasonCodes = append(entry.ReasonCodes, code)
	}
	writeManagedAccessLog(ctx)
}

func writeManagedAccessLog(ctx wrapper.HttpContext) {
	entry := managedAccessOrigin(ctx)
	if entry == nil {
		return
	}
	// Only this fixed struct crosses the host boundary. No incoming header,
	// prompt, body, provider error, credential or arbitrary attribute is read.
	raw, err := json.Marshal(entry)
	if err == nil {
		_ = proxywasm.SetProperty([]string{managedAccessLogProperty}, []byte(wrapper.MarshalStr(string(raw))))
	}
}

func closeManagedAccessLog(ctx wrapper.HttpContext) {
	entry := managedAccessOrigin(ctx)
	if entry == nil {
		return
	}
	writeManagedAccessLog(ctx)
	// Later plugin-owned Redis ACK callbacks cannot write into a deleted HTTP
	// context or accidentally update a different invocation's filter state.
	entry.closed = true
}

// Unknown strings collapse to a constant; formatting never turns them into log data.
func safeManagedAccessReason(reason string) string {
	switch reason {
	case "append_dispatch_failed",
		"attempt_fact_invalid",
		"authenticated_identity_missing",
		"body_context_unavailable",
		"current_execution_admission_required",
		"delivery_bridge_invalid",
		"delivery_fact_invalid",
		"execution_admission_expired",
		"execution_context_reentry",
		"execution_identity_install_failed",
		"execution_identity_unavailable",
		"fact_dispatch_failed",
		"fact_retention_failed",
		"fact_snapshot_invalid",
		"health_candidate_invalid",
		"health_claim_invalid",
		"health_consume_invalid",
		"health_consume_unconfirmed",
		"health_final_selection_invalid",
		"health_member_choice_unavailable",
		"health_member_directory_invalid",
		"health_or_admission_expired",
		"health_rate_limit_scope_unknown",
		"health_recovery_not_granted",
		"health_revision_below_registered_minimum",
		"identity_header_cleanup_failed",
		"internal_execution_check_unconfirmed",
		"internal_execution_endpoint_missing",
		"internal_execution_not_authorized",
		"internal_execution_request_invalid",
		"invalid_json_body",
		"invalid_stream_flag",
		"json_body_required",
		"local_response_not_acknowledged",
		"mapped_model_differs_from_selection",
		"mapped_request_body_out_of_bounds",
		"media_operation_api_required",
		"original_body_basis_missing",
		"original_execution_missing",
		"original_selection_invalid",
		"protocol_conversion_changed_selection",
		"protocol_response_conversion_failed",
		"registered_internal_service_call_required",
		"registration_invalid",
		"request_body_out_of_bounds",
		"stream_ack_missing",
		"stream_append_not_acknowledged",
		"stream_unconfigured",
		"text_execution_body_invalid",
		"unsupported_or_invalid_protocol_request",
		"unsupported_text_request",
		"upstream_callback_missing",
		"upstream_health_unavailable":
		return reason
	default:
		return "other_managed_failure"
	}
}
