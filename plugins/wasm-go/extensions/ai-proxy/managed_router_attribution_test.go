package main

import (
	"encoding/json"
	"testing"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/provider"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/stretchr/testify/require"
)

// #14 R01: an owner handoff recreates the HTTP stream and its Wasm properties.
func TestResponsesIncidentRouterContinuationRestoresOperationMetadata(t *testing.T) {
	for _, authorized := range []bool{true, false} {
		name := "untrusted continuation"
		if authorized {
			name = "verified owner handoff"
		}
		t.Run(name, func(t *testing.T) {
			fixture := responseModeFixture("openai", clientresponse.Responses, clientresponse.Responses, "auto", clientresponse.OpenAI)
			settings := fixture["managed_execution"].(managedSettings)
			settings.SendTransport, settings.StreamProtocol, settings.StreamProjection, settings.RouterReentry = "router", clientresponse.Responses, true, true
			fixture["managed_execution"] = settings
			marker := []byte("untrusted-marker")
			if !t.Run("original stream", func(t *testing.T) {
				managedRouterReentryTickRegistered = false
				h := newManagedTextHost(t, fixture)
				c := h.cfg.ManagedExecution.(*managedExecutionConfig)
				require.NoError(t, h.host.SetProperty([]string{"cluster_name"}, []byte(c.release.Descriptor.ClusterRef)))
				request := responseModeRequest(clientresponse.Responses, c.release.PublicModel, true)
				require.Equal(t, types.HeaderStopIteration, h.host.CallOnRequestHeaders(h.id, managedRequestHeaders(t, request), false))
				if authorized {
					previous := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
					next := *previous
					require.NoError(t, queueManagedRouterContinuation(h.ctx, previous, &next, managedProtocolRequest{Body: request}))
					previous.handedOff = true
					var err error
					marker, err = h.host.GetProperty([]string{wrapper.RouterContinuationProperty})
					require.NoError(t, err)
				}
			}) {
				return
			}

			// The emulator has one property map for all streams and no delete API.
			// A fresh host models recreation; only the declared downstream marker survives.
			managedRouterReentryTickRegistered = false
			h := newManagedTextHost(t, fixture)
			c := h.cfg.ManagedExecution.(*managedExecutionConfig)
			require.NoError(t, h.host.SetProperty([]string{wrapper.RouterContinuationProperty}, marker))
			cleared, err := h.host.GetProperty([]string{adapterMetadataProperty})
			require.Error(t, err)
			require.Empty(t, cleared)
			headers := [][2]string{{":method", "POST"}, {":authority", wrapper.RouterContinuationHost}, {":path", managedRouterTargetPath(c)}, {"content-type", "application/json"}}
			require.Equal(t, types.HeaderStopIteration, h.host.CallOnRequestHeaders(h.id, headers, false))
			raw, err := h.host.GetProperty([]string{adapterMetadataProperty})
			if !authorized {
				require.EqualValues(t, 403, h.host.GetSentLocalResponse(h.id).StatusCode)
				require.Empty(t, raw, "an unverified private URL cannot publish operation metadata")
				return
			}
			require.NoError(t, err)
			require.Nil(t, h.host.GetSentLocalResponse(h.id))
			require.Empty(t, managedRouterPending)
			var metadata adapterMetadata
			require.NoError(t, json.Unmarshal(raw, &metadata))
			require.Equal(t, 2, metadata.SchemaVersion)
			require.True(t, metadata.Matched)
			require.Equal(t, "auto", metadata.Mode)
			require.Equal(t, string(provider.ApiNameResponses), metadata.NormalizedOperation)
			require.Equal(t, provider.PathOpenAIResponses, metadata.OperationTemplate)
			require.Empty(t, metadata.AdapterID)
		})
	}
}
