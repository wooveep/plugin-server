package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/ai/observedusage"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func incidentProtocolRouter(t *testing.T, protocol, mode string, stream bool, request []byte) *managedHost {
	t.Helper()
	supplier, profile := "openai", clientresponse.OpenAI
	if protocol == clientresponse.Messages {
		supplier, profile = "deepseek", clientresponse.Qwen
	}
	if mode == "passthrough" {
		profile = ""
	}
	fixture := responseModeFixture(supplier, protocol, protocol, mode, profile)
	settings := fixture["managed_execution"].(managedSettings)
	settings.SendTransport, settings.StreamProjection = "router", mode == "auto"
	if protocol != clientresponse.Chat {
		settings.StreamProtocol = protocol
	}
	fixture["managed_execution"] = settings
	h := newManagedHost(t, fixture)
	release := fixture["_modelfusion_execution"].(managedRelease)
	require.NoError(t, h.host.SetProperty([]string{"cluster_name"}, []byte(release.Descriptor.ClusterRef)))
	if request == nil {
		request = responseModeRequest(protocol, release.PublicModel, stream)
	}
	h.start(t, request, bytes.Replace(request, []byte(release.PublicModel), []byte("upstream-model"), 1))
	require.NoError(t, h.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
	return h
}

// #12: buffered JSON still waits for durable resources; failed delivery must
// retain original metering even though it cannot disclose the new aliases.
func TestResponsesIncidentJSONAliasFailureKeepsBoundaryAndUsage(t *testing.T) {
	for _, mode := range []string{"auto", "passthrough"} {
		t.Run(mode, func(t *testing.T) {
			h := incidentProtocolRouter(t, clientresponse.Responses, mode, false, nil)
			h.host.CallOnResponseHeaders(h.id, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, false)
			original := responseModePayload(clientresponse.Responses, false)
			h.host.CallOnResponseBody(h.id, original, true)
			writes := h.host.GetRedisCalloutAttributesFromContext(h.id)
			require.Len(t, writes, 1)
			h.host.CallOnRedisCallResponse(writes[0].CalloutID, 0, []byte("-OOM owned alias write unavailable\r\n"))
			status, err := managedSingleHeader(h.host.GetCurrentResponseHeaders(h.id), ":status")
			require.NoError(t, err)
			require.Equal(t, "503", status)
			wire := h.host.GetCurrentResponseBody(h.id)
			require.True(t, gjson.GetBytes(wire, "error").Exists())
			require.NotContains(t, string(wire), "native-response")
			require.NotContains(t, string(wire), "resp_pf_")
			state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
			require.False(t, state.resourcesSaved)
			require.Equal(t, "failed", state.lastDelivery.Envelope().DeliveryState)
			require.Equal(t, "17", state.lastDelivery.Envelope().Usage.Quantities["input"])
			require.Equal(t, "5", state.lastDelivery.Envelope().Usage.Quantities["output"])
			raw, err := h.host.GetProperty([]string{observedusage.Property})
			require.NoError(t, err)
			require.Equal(t, "17", gjson.GetBytes(raw, "input").String())
			require.Equal(t, "5", gjson.GetBytes(raw, "output").String())
		})
	}
}

func TestResponsesIncidentTextRouterProtocolDeliveryMatrix(t *testing.T) {
	for _, protocol := range []string{clientresponse.Chat, clientresponse.Responses, clientresponse.Messages} {
		for _, stream := range []bool{false, true} {
			for _, failedWrite := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/stream=%t/write_failure=%t", protocol, stream, failedWrite), func(t *testing.T) {
					h := incidentProtocolRouter(t, protocol, "auto", stream, nil)
					state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
					inject := injectManagedRouterTerminal
					t.Cleanup(func() { injectManagedRouterTerminal = inject })
					var injections int
					injectManagedRouterTerminal = func(body []byte, end bool) error {
						injections++
						return inject(body, end)
					}
					contentType := "application/json"
					if stream {
						contentType = "text/event-stream"
					}
					h.host.CallOnResponseHeaders(h.id, [][2]string{{":status", "200"}, {"content-type", contentType}}, false)
					h.host.CallOnResponseBody(h.id, responseModePayload(protocol, stream), true)
					aliases := h.host.GetRedisCalloutAttributesFromContext(h.id)
					if protocol == clientresponse.Responses {
						require.Len(t, aliases, 1)
						reply := ":1\r\n"
						if failedWrite {
							reply = "-OOM owned alias write unavailable\r\n"
						}
						h.host.CallOnRedisCallResponse(aliases[0].CalloutID, 0, []byte(reply))
					} else {
						require.Empty(t, aliases, "this protocol must not manufacture a Responses alias operation")
					}
					var appends int
					for _, call := range h.host.GetRedisCalloutAttributesFromContext(proxytest.PluginContextID) {
						if !bytes.Contains(call.Query, []byte(state.lastDelivery.EventID())) {
							continue
						}
						appends++
						reply := "$3\r\n1-0\r\n"
						if failedWrite {
							reply = "-OOM owned fact append unavailable\r\n"
						}
						h.host.CallOnRedisCallResponse(call.CalloutID, 0, []byte(reply))
						break
					}
					require.Positive(t, appends)
					require.NotNil(t, state.lastDelivery)
					delivery := state.lastDelivery.Envelope()
					bufferedAliasFailure := protocol == clientresponse.Responses && !stream && failedWrite
					expectedDelivery := "succeeded"
					if bufferedAliasFailure {
						expectedDelivery = "failed"
					}
					require.Equal(t, expectedDelivery, delivery.DeliveryState)
					require.Equal(t, "17", delivery.Usage.Quantities["input"])
					require.Equal(t, "5", delivery.Usage.Quantities["output"])
					if failedWrite {
						require.Equal(t, "unknown", state.deliveries[state.lastDelivery.EventID()].Status().State)
					}
					raw, err := h.host.GetProperty([]string{observedusage.Property})
					require.NoError(t, err)
					require.Equal(t, "17", gjson.GetBytes(raw, "input").String())
					require.Equal(t, "5", gjson.GetBytes(raw, "output").String())
					wire := string(h.host.GetCurrentResponseBody(h.id))
					if bufferedAliasFailure {
						require.True(t, gjson.Get(wire, "error").Exists())
					} else {
						require.Contains(t, wire, "hello")
					}
					if stream {
						terminal := map[string]string{clientresponse.Chat: "[DONE]", clientresponse.Responses: "event: response.completed", clientresponse.Messages: "event: message_stop"}[protocol]
						require.Contains(t, wire, terminal)
					}
					expectedInjections := 0
					if protocol == clientresponse.Responses && stream {
						expectedInjections = 1
					}
					require.Equal(t, expectedInjections, injections)
					h.host.CompleteHttpContext(h.id)
					require.True(t, state.closed)
				})
			}
		}
	}
}

func TestResponsesIncidentLargeTextRequestsPreserveBodies(t *testing.T) {
	for _, protocol := range []string{clientresponse.Chat, clientresponse.Responses, clientresponse.Messages} {
		t.Run(protocol, func(t *testing.T) {
			var messages strings.Builder
			for i := range 1090 {
				if i != 0 {
					messages.WriteByte(',')
				}
				fmt.Fprintf(&messages, `{"role":"user","content":%q}`, strings.Repeat("x", 800))
			}
			field, tail := "messages", `,"max_tokens":16`
			if protocol == clientresponse.Responses {
				field, tail = "input", ""
			}
			request := []byte(fmt.Sprintf(`{"model":"public-model","%s":[%s],"stream":true%s}`, field, messages.String(), tail))
			h := incidentProtocolRouter(t, protocol, "auto", true, request)
			require.Nil(t, h.host.GetSentLocalResponse(h.id))
			actual := h.host.GetCurrentRequestBody(h.id)
			require.Greater(t, len(actual), 800000)
			items := gjson.GetBytes(actual, field).Array()
			require.Len(t, items, 1090)
			require.Equal(t, strings.Repeat("x", 800), items[0].Get("content").String())
			require.Equal(t, strings.Repeat("x", 800), items[1089].Get("content").String())
			require.Equal(t, "upstream-model", gjson.GetBytes(actual, "model").String())
			state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
			require.True(t, state.dispatchStarted)
		})
	}
}
