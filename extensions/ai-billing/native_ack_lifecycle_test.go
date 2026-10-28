package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	billinglog "github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestNativeBillingACKSurvivesActualHTTPContextDeletion(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		cfg, rule := independentNativeBillingRule(t, "ack-lifecycle", "redis-native.static", "private-native")
		for _, late := range []bool{false, true} {
			for _, result := range []string{"success", "redis_error", "network_timeout"} {
				t.Run(map[bool]string{false: "before_end", true: "after_delete"}[late]+"/"+result, func(t *testing.T) {
					host, status := test.NewTestHost(mustBillingConfig(t, independentNativeBillingRoot(rule)))
					defer host.Reset()
					// Go tests share the package logger; capture this case explicitly
					// instead of depending on the logger left by an earlier test.
					logs := &recordingBillingLog{}
					billinglog.SetPluginLog(logs)
					require.Equal(t, types.OnPluginStartStatusOK, status)
					require.NoError(t, host.SetRouteName("route-ack-lifecycle"))
					require.NoError(t, host.SetClusterName("outbound|8080||ack-lifecycle.dns"))
					headers := append([][2]string{{":authority", "example.test"}, {":path", "/custom/text"}, {":method", http.MethodPost}}, nativeBillingAdmittedHeaders(cfg, uuid.NewSHA1(uuid.NameSpaceURL, []byte("ack-lifecycle")).String())...)
					require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders(headers))
					require.Equal(t, types.ActionContinue, host.CallOnHttpResponseHeaders([][2]string{
						{":status", "200"}, {"content-type", "application/json"},
						{executionfacts.DeliveryBridgeHeader, nativeBillingBridgeFixture(t, independentNativeBillingDelivery(t, cfg, "ack-lifecycle"))},
					}))
					require.Equal(t, types.ActionContinue, host.CallOnHttpResponseBody([]byte(`{"model":"unmodified"}`)))
					calls := host.GetRedisCalloutAttributesFromContext(proxytest.PluginContextID)
					require.Len(t, calls, 1, "event write must be owned by the plugin context")
					if late {
						host.CompleteHttp()
						require.Empty(t, host.GetWarnLogs(), "HTTP completion is not an ACK deadline")
					}
					reply, replyStatus := test.CreateRedisRespString("20-0"), int32(0)
					if result == "redis_error" {
						reply = test.CreateRedisRespError("controlled append failure")
					}
					if result == "network_timeout" {
						reply, replyStatus = nil, 1
					}
					host.CallOnRedisCallResponse(calls[0].CalloutID, replyStatus, reply)
					if !late {
						host.CompleteHttp()
					}
					gapSeen := false
					for _, warning := range append(host.GetWarnLogs(), logs.entries...) {
						require.NotContains(t, warning, "stream_ack_missing")
						gapSeen = gapSeen || strings.Contains(warning, "execution fact delivery gap")
					}
					require.Equal(t, result != "success", gapSeen, "real Redis failures must remain observable")
					require.Empty(t, host.GetRedisCalloutAttributesFromContext(proxytest.PluginContextID))
					require.Nil(t, host.GetLocalResponse())
					require.Equal(t, `{"model":"unmodified"}`, string(host.GetResponseBody()))
				})
			}
		}
	})
}
