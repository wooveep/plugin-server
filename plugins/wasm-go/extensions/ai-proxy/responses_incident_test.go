package main

import (
	"bytes"
	"errors"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/ai/observedusage"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// Regressions for https://github.com/wooveep/plugin-server/issues/12.
func responsesIncidentRouter(t *testing.T, mode string) *managedHost {
	t.Helper()
	profile := clientresponse.OpenAI
	if mode == "passthrough" {
		profile = ""
	}
	fixture := responseModeFixture("openai", clientresponse.Responses, clientresponse.Responses, mode, profile)
	settings := fixture["managed_execution"].(managedSettings)
	settings.SendTransport, settings.StreamProjection = "router", mode == "auto"
	settings.StreamProtocol = clientresponse.Responses
	fixture["managed_execution"] = settings
	h := newManagedHost(t, fixture)
	release := fixture["_modelfusion_execution"].(managedRelease)
	require.NoError(t, h.host.SetProperty([]string{"cluster_name"}, []byte(release.Descriptor.ClusterRef)))
	h.start(t, responseModeRequest(clientresponse.Responses, release.PublicModel, true), responseModeRequest(clientresponse.Responses, "upstream-model", true))
	require.NoError(t, h.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
	return h
}

func TestResponsesIncidentFactsAndUsagePrecedeTerminalClose(t *testing.T) {
	for _, result := range []struct{ name, reply string }{
		{"aliases_saved", ":1\r\n"},
		{"alias_write_failed", "-OOM alias write unavailable\r\n"},
	} {
		t.Run(result.name, func(t *testing.T) {
			h := responsesIncidentRouter(t, "auto")
			state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
			inject := injectManagedRouterTerminal
			t.Cleanup(func() { injectManagedRouterTerminal = inject })
			var injections int
			injectManagedRouterTerminal = func(tail []byte, end bool) error {
				injections++
				require.True(t, end)
				require.False(t, state.closed)
				require.NotNil(t, state.lastAttempt, "upstream facts must exist before a synchronous stream close")
				require.NotNil(t, state.lastDelivery, "local delivery accounting must precede end_stream")
				raw, err := h.host.GetProperty([]string{observedusage.Property})
				require.NoError(t, err)
				require.Equal(t, "17", gjson.GetBytes(raw, "input").String())
				require.Equal(t, "5", gjson.GetBytes(raw, "output").String())
				if result.name == "alias_write_failed" {
					entry, _ := readManagedAccessLog(t, h)
					require.Contains(t, entry.ReasonCodes, "resource_reference_write_failed")
				}
				err = inject(tail, end)
				h.host.CompleteHttpContext(h.id)
				return err
			}
			h.host.CallOnResponseHeaders(h.id, [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}, false)
			h.host.CallOnResponseBody(h.id, responseModePayload(clientresponse.Responses, true), true)
			writes := h.host.GetRedisCalloutAttributesFromContext(h.id)
			require.Len(t, writes, 1)
			h.host.CallOnRedisCallResponse(writes[0].CalloutID, 0, []byte(result.reply))
			require.Equal(t, 1, injections)
			require.True(t, state.closed)
			require.Equal(t, "succeeded", state.lastDelivery.Envelope().DeliveryState)
		})
	}
}

func TestResponsesIncidentInjectionFailureRetainsFailedDelivery(t *testing.T) {
	h := responsesIncidentRouter(t, "auto")
	state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
	inject := injectManagedRouterTerminal
	t.Cleanup(func() { injectManagedRouterTerminal = inject })
	injectManagedRouterTerminal = func([]byte, bool) error { return errors.New("terminal injection unavailable") }
	h.host.CallOnResponseHeaders(h.id, [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}, false)
	h.host.CallOnResponseBody(h.id, responseModePayload(clientresponse.Responses, true), true)
	writes := h.host.GetRedisCalloutAttributesFromContext(h.id)
	require.Len(t, writes, 1)
	h.host.CallOnRedisCallResponse(writes[0].CalloutID, 0, []byte(":1\r\n"))
	require.NotNil(t, state.lastDelivery)
	require.Equal(t, "failed", state.lastDelivery.Envelope().DeliveryState)
	require.Equal(t, "17", state.lastDelivery.Envelope().Usage.Quantities["input"])
	require.Equal(t, "5", state.lastDelivery.Envelope().Usage.Quantities["output"])
}

func TestResponsesIncidentResourceWriteFailurePreservesCompletedUsage(t *testing.T) {
	for _, mode := range []string{"auto", "passthrough"} {
		t.Run(mode, func(t *testing.T) {
			h := responsesIncidentRouter(t, mode)
			h.host.CallOnResponseHeaders(h.id, [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}, false)
			original := responseModePayload(clientresponse.Responses, true)
			h.host.CallOnResponseBody(h.id, original, true)
			writes := h.host.GetRedisCalloutAttributesFromContext(h.id)
			require.Len(t, writes, 1, "only resource persistence remains pending")
			h.host.CallOnRedisCallResponse(writes[0].CalloutID, 0, []byte("-OOM command not allowed when used memory exceeds maxmemory\r\n"))
			wire := h.host.GetCurrentResponseBody(h.id)
			require.Contains(t, string(wire), "event: response.completed", "an auxiliary alias write must not erase the verified terminal")
			require.NotContains(t, string(wire), "event: error")
			events, err := clientresponse.Events(wire)
			require.NoError(t, err)
			var completed int
			for _, event := range events {
				if event.Name == "response.completed" {
					completed++
					require.Equal(t, int64(17), gjson.GetBytes(event.Data, "response.usage.input_tokens").Int())
					require.Equal(t, int64(5), gjson.GetBytes(event.Data, "response.usage.output_tokens").Int())
				}
			}
			require.Equal(t, 1, completed)
			if mode == "passthrough" {
				require.True(t, bytes.Equal(original, wire), "native passthrough bytes must stay intact")
			}
			state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
			require.False(t, state.resourcesSaved)
			require.NotNil(t, state.lastDelivery)
			delivery := state.lastDelivery.Envelope()
			require.Equal(t, "succeeded", delivery.DeliveryState)
			require.Equal(t, "17", delivery.Usage.Quantities["input"])
			require.Equal(t, "5", delivery.Usage.Quantities["output"])
			entry, _ := readManagedAccessLog(t, h)
			require.True(t, entry.FactGap)
			require.Contains(t, entry.ReasonCodes, "resource_reference_write_failed")
		})
	}
}
