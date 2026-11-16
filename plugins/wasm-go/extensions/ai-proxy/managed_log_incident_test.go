package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Regression: https://github.com/wooveep/plugin-server/issues/13.
func TestResponsesIncidentControlledReasonsRemainSafeAndExact(t *testing.T) {
	for _, reason := range []string{
		"resource_reference_write_failed", "resource_reference_unavailable", "resource_reference_incompatible", "resource_owner_unavailable",
		"router_continuation_unavailable", "router_target_not_verified", "execution_budget_expired", "router_request_install_failed",
		"router_resume_failed", "router_observed_target_mismatch", "router_public_headers_unavailable", "router_public_response_unavailable",
		"native_resource_observation_failed", "telemetry_invalid",
	} {
		t.Run(reason, func(t *testing.T) {
			require.Equal(t, reason, safeManagedAccessReason(reason))
		})
	}
	require.Equal(t, "other_managed_failure", safeManagedAccessReason("private supplier error with prompt or credential"))
}
