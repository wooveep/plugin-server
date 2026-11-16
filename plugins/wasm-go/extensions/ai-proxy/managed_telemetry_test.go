package main

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/stretchr/testify/require"
)

func TestManagedClientMetadataIgnoresForgedForwardingHeaders(t *testing.T) {
	h := newManagedHost(t)
	require.NoError(t, h.host.SetProperty([]string{"source", "address"}, []byte("[2001:db8::7]:443")))
	headers := append(managedRequestHeaders(t, []byte(managedOriginalBody)),
		[2]string{"x-forwarded-for", "192.0.2.99"}, [2]string{"x-real-ip", "192.0.2.98"},
		[2]string{"user-agent", strings.Repeat("客", 200)})
	h.host.CallOnRequestHeaders(h.id, headers, false)
	state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
	require.Equal(t, "2001:db8::7", state.client.ClientIP)
	require.Equal(t, executionfacts.ClientIPDownstreamConnection, state.client.ClientIPSource)
	require.LessOrEqual(t, len(state.client.UserAgent), executionfacts.MaxUserAgentBytes)
	require.True(t, utf8.ValidString(state.client.UserAgent))
	require.NotEmpty(t, state.client.UserAgent)
}

func TestManagedPortalMarkerNeverMakesConsoleTheOriginalClient(t *testing.T) {
	h := newManagedHost(t)
	require.NoError(t, h.host.SetProperty([]string{"source", "address"}, []byte("10.42.0.8:8080")))
	headers := append(managedRequestHeaders(t, []byte(managedOriginalBody)),
		[2]string{"via", "1.1 modelfusion-portal"}, [2]string{"x-forwarded-for", "192.0.2.99"})
	h.host.CallOnRequestHeaders(h.id, headers, false)
	state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
	require.Empty(t, state.client.ClientIP)
	require.Empty(t, state.client.UserAgent)
	require.Equal(t, executionfacts.ClientIPUnconfirmedPortal, state.client.ClientIPSource)
}
