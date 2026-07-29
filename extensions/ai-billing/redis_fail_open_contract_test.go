package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestBillingRedisFailOpenContract(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		t.Run("XADD dispatch failure returns after one truthful event attempt", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			client := &recordingBillingRedisClient{err: errors.New("network unavailable")}
			const requestSecret = "raw-api-key-dispatch-secret"
			ctx := &mockBillingHttpContext{values: map[string]interface{}{
				ctxStartTime:        int64(1),
				ctxEventID:          "018f4c7c-4444-7abc-8444-444444444444",
				ctxIdempotencyKey:   "018f4c7c-4444-7abc-8444-444444444444",
				ctxRequestID:        "req-dispatch-failure",
				ctxTenant:           "tenant-a",
				ctxConsumer:         "consumer-a",
				ctxRoute:            "route-native",
				ctxCluster:          "llm-vendor-native.dns",
				ctxQuotaScope:       "tenant",
				ctxModel:            "vendor-model-9",
				ctxRequestPath:      "/native/v9/generate",
				ctxStatusCode:       200,
				ctxInputToken:       int64(4),
				ctxOutputToken:      int64(6),
				ctxTotalToken:       int64(10),
				ctxUsageSource:      usageSourceProvider,
				ctxProviderUsage:    map[string]interface{}{"prompt_tokens": 4, "completion_tokens": 6, "total_tokens": 10},
				ctxRequestText:      requestSecret,
				ctxUpstreamInvoked:  true,
				ctxBillingDelivered: false,
			}}
			deliverBillingEvent(ctx, BillingConfig{
				EventKind: eventKindUsage,
				RedisStream: RedisStream{
					Stream: "global:billing:events",
				},
				redisClient: client,
			}, false)

			require.Len(t, client.commands, 1)
			require.True(t, ctx.GetBoolContext(ctxBillingDelivered, false))
			command := client.commands[0]
			require.Equal(t, []interface{}{"xadd", "global:billing:events", "*", "event"}, command[:4])
			body, ok := command[4].(string)
			require.True(t, ok)
			var event map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(body), &event))
			require.Equal(t, eventKindUsage, event["event_kind"])
			require.Equal(t, "req-dispatch-failure", event["request_id"])
			require.EqualValues(t, 200, event["status_code"])
			require.Equal(t, true, event["upstream_invoked"])
			require.Equal(t, false, event["usage_missing"])
			require.Equal(t, usageSourceProvider, event["usage_source"])
			require.NotContains(t, body, requestSecret)

			logs := make([]string, 0)
			logs = append(logs, host.GetTraceLogs()...)
			logs = append(logs, host.GetDebugLogs()...)
			logs = append(logs, host.GetInfoLogs()...)
			logs = append(logs, host.GetWarnLogs()...)
			logs = append(logs, host.GetErrorLogs()...)
			logs = append(logs, host.GetCriticalLogs()...)
			require.NotContains(t, strings.Join(logs, "\n"), requestSecret)
		})

		t.Run("XADD response error preserves at most once completion", func(t *testing.T) {
			config := mustBillingConfig(t, map[string]interface{}{
				"event_kind":      eventKindUsage,
				"quota_scope":     "tenant",
				"provider":        "vendor-native",
				"tenant_header":   "x-tenant-id",
				"consumer_header": "x-consumer-id",
				"redis_stream": map[string]interface{}{
					"service_name": "redis.static",
					"service_port": 6379,
					"stream":       "global:billing:events",
					"timeout":      750,
				},
				"enable_path_suffixes": []string{"/native/v9/generate"},
			})
			host, status := test.NewTestHost(config)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("route-native"))
			require.NoError(t, host.SetClusterName("llm-vendor-native.dns"))
			require.NoError(t, host.SetRequestId("req-response-error"))
			require.NoError(t, host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))

			const requestSecret = "raw-api-key-response-secret"
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "provider.example.com"},
				{":path", "/native/v9/generate"},
				{":method", "POST"},
				{"x-request-id", "req-response-error"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
				{"authorization", "Bearer " + requestSecret},
			})
			require.Equal(t, types.ActionContinue, action)
			action = host.CallOnHttpRequestBody([]byte(`{
				"model":"vendor-model-9",
				"messages":[{"role":"user","content":"` + requestSecret + `"}]
			}`))
			require.Equal(t, types.ActionContinue, action)
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)
			responseBody := []byte(`{
				"model":"vendor-model-9",
				"usage":{"prompt_tokens":4,"completion_tokens":6,"total_tokens":10}
			}`)
			action = host.CallOnHttpResponseBody(responseBody)
			require.Equal(t, types.ActionContinue, action)
			require.Equal(t, responseBody, host.GetResponseBody())

			attrs := host.GetRedisCalloutAttributes()
			require.Len(t, attrs, 1)
			command := redisCommand(t, attrs[0].Query)
			require.Equal(t, []string{"xadd", "global:billing:events", "*", "event"}, command[:4])
			var event map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(command[4]), &event))
			require.Equal(t, eventKindUsage, event["event_kind"])
			require.Equal(t, "req-response-error", event["request_id"])
			require.EqualValues(t, 200, event["status_code"])
			require.Equal(t, true, event["upstream_invoked"])
			require.Equal(t, false, event["usage_missing"])
			require.Equal(t, usageSourceProvider, event["usage_source"])
			require.NotContains(t, command[4], requestSecret)

			host.CallOnRedisCall(0, test.CreateRedisRespError("temporary delivery failure"))
			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			require.Equal(t, responseBody, host.GetResponseBody())
			host.CompleteHttp()
			require.Empty(t, host.GetRedisCalloutAttributes())

			logs := make([]string, 0)
			logs = append(logs, host.GetTraceLogs()...)
			logs = append(logs, host.GetDebugLogs()...)
			logs = append(logs, host.GetInfoLogs()...)
			logs = append(logs, host.GetWarnLogs()...)
			logs = append(logs, host.GetErrorLogs()...)
			logs = append(logs, host.GetCriticalLogs()...)
			require.NotContains(t, strings.Join(logs, "\n"), requestSecret)
		})
	})
}
