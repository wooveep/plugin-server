package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// Regression: https://github.com/wooveep/plugin-server/issues/14.
func TestResponsesIncidentNativeOperationUsesTrustedProfile(t *testing.T) {
	for _, mode := range []string{"auto", "passthrough"} {
		t.Run(mode, func(t *testing.T) {
			h := responsesIncidentRouter(t, mode)
			raw, err := h.host.GetProperty([]string{adapterMetadataProperty})
			require.NoError(t, err)
			var metadata adapterMetadata
			require.NoError(t, json.Unmarshal(raw, &metadata))
			require.Equal(t, 2, metadata.SchemaVersion)
			require.True(t, metadata.Matched)
			require.Equal(t, mode, metadata.Mode)
			require.Equal(t, "/v1/responses", metadata.OperationTemplate)
			require.Equal(t, "openai/v1/responses", metadata.NormalizedOperation)
			require.Empty(t, metadata.AdapterID, "a native operation must not invent a fallback adapter")
			require.NotContains(t, string(raw), "/custom/customer/")
			require.NotContains(t, string(raw), "controlled-upstream-key")
		})
	}
}
