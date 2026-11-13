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
	for _, tc := range []struct {
		name                             string
		authorized, converted, unmatched bool
	}{
		{"verified owner handoff", true, false, false},
		{"verified matched conversion", true, true, false},
		{"explicitly unmatched stays unknown", true, false, true},
		{"untrusted continuation", false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := clientresponse.Responses
			if tc.converted {
				upstream = clientresponse.Chat
			}
			fixture := responseModeFixture("openai", clientresponse.Responses, upstream, "auto", clientresponse.OpenAI)
			expected := adapterMetadata{SchemaVersion: 2, Matched: true, Mode: "auto", NormalizedOperation: string(provider.ApiNameResponses), OperationTemplate: provider.PathOpenAIResponses}
			if tc.converted {
				expected.AdapterID, expected.Version = "responses.to_chat_completions", "1"
				expected.ProviderMatch, expected.CapabilityMatch, expected.UsageProfile = "openai", "openai/v1/chat/completions", "openai.responses.v1"
			}
			if tc.unmatched {
				expected = adapterMetadata{SchemaVersion: 2, Mode: "auto"}
			}
			expectedRaw, err := json.Marshal(expected)
			require.NoError(t, err)
			settings := fixture["managed_execution"].(managedSettings)
			settings.SendTransport, settings.StreamProtocol, settings.StreamProjection, settings.RouterReentry = "router", clientresponse.Responses, true, true
			fixture["managed_execution"] = settings
			marker := []byte("untrusted-marker")
			if !t.Run("original stream", func(t *testing.T) {
				managedRouterReentryTickRegistered = false
				h := newManagedTextHost(t, fixture)
				c := h.cfg.ManagedExecution.(*managedExecutionConfig)
				require.NoError(t, h.host.SetProperty([]string{"cluster_name"}, []byte(c.release.Descriptor.ClusterRef)))
				if tc.converted {
					publishAdapterMetadata(expected)
				}
				request := responseModeRequest(clientresponse.Responses, c.release.PublicModel, true)
				require.Equal(t, types.HeaderStopIteration, h.host.CallOnRequestHeaders(h.id, managedRequestHeaders(t, request), false))
				if tc.unmatched {
					publishNoAdapterMetadata("auto")
				}
				if tc.authorized {
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
			if !tc.authorized {
				require.EqualValues(t, 403, h.host.GetSentLocalResponse(h.id).StatusCode)
				require.Empty(t, raw, "an unverified private URL cannot publish operation metadata")
				return
			}
			require.NoError(t, err)
			require.Nil(t, h.host.GetSentLocalResponse(h.id))
			require.Empty(t, managedRouterPending)
			var metadata adapterMetadata
			require.NoError(t, json.Unmarshal(raw, &metadata))
			require.Equal(t, expected, metadata)
			require.Equal(t, expectedRaw, raw, "verified conversion identity must survive byte-for-byte")
		})
	}
}
