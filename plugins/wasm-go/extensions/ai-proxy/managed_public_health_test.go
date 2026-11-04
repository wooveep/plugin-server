package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestHealthGateReturnsPublicProtocolErrorWithoutChangingEligibility(t *testing.T) {
	for _, kind := range []string{managedProtocolChat, managedProtocolResponses, managedProtocolMessages} {
		t.Run(kind, func(t *testing.T) {
			fixture := managedConfigFixture()
			settings := fixture["managed_execution"].(managedSettings)
			settings.InputProtocol, settings.UpstreamProtocol = kind, managedProtocolChat
			fixture["managed_execution"] = settings
			h := newManagedHost(t, fixture)
			h.host.CallOnRequestHeaders(h.id, managedRequestHeaders(t, []byte(managedOriginalBody)), false)
			state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
			seconds := uint64(30)
			rejectManagedHealth(h.ctx, state, "upstream_health_unavailable", &seconds)
			reply := h.host.GetSentLocalResponse(h.id)
			require.NotNil(t, reply)
			require.EqualValues(t, 503, reply.StatusCode)
			require.Equal(t, "service_unavailable", gjson.GetBytes(reply.Data, "error.code").String())
			require.NotEmpty(t, gjson.GetBytes(reply.Data, "error.message").String())
			require.NotContains(t, string(reply.Data), "upstream_health")
			headers := map[string]string{}
			for _, field := range reply.Headers {
				headers[field[0]] = field[1]
			}
			require.Equal(t, state.callID, headers["x-request-id"])
			require.Equal(t, "30", headers["retry-after"])
			require.Empty(t, h.host.GetCalloutAttributesFromContext(h.id))
			require.False(t, state.dispatchStarted)
		})
	}
}
