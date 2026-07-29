package main

import (
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBillingCorrelationLifecycleContract(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		config := mustBillingConfig(t, map[string]interface{}{
			"event_kind":      eventKindUsage,
			"quota_scope":     "tenant",
			"provider":        "vendor-native",
			"tenant_header":   "x-tenant-id",
			"consumer_header": "x-consumer-id",
			"redis_stream": map[string]interface{}{
				"service_name": "redis.static",
				"service_port": 6379,
				"username":     "billing-user-secret-3-3",
				"password":     "billing-password-secret-3-3",
				"timeout":      750,
			},
			"enable_path_suffixes": []string{"/v1/chat/completions"},
		})

		t.Run("non-streaming response replaces upstream values and matches event correlation", func(t *testing.T) {
			host, status := test.NewTestHost(config)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "provider.example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-request-id", "canonical-non-stream-3-3"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			require.Equal(t, types.ActionContinue, action)
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
				{gatewayRequestIDHeader, "upstream-first"},
				{gatewayRequestIDHeader, "upstream-second"},
			})
			require.Equal(t, types.ActionContinue, action)
			assert.Equal(t, "canonical-non-stream-3-3", singleHeaderValue(t, host.GetResponseHeaders(), gatewayRequestIDHeader))

			action = host.CallOnHttpResponseBody([]byte(`{"model":"vendor-model","usage":{"prompt_tokens":4,"completion_tokens":6,"total_tokens":10}}`))
			require.Equal(t, types.ActionContinue, action)
			event := requireRedisBillingEvent(t, host)
			require.Equal(t, "canonical-non-stream-3-3", event["request_id"])
			require.NotEmpty(t, event["event_id"])
			require.Equal(t, event["event_id"], event["idempotency_key"])
			require.Len(t, host.GetRedisCalloutAttributes(), 1)
		})

		t.Run("streaming completion preserves one header and one XADD", func(t *testing.T) {
			host, status := test.NewTestHost(config)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "provider.example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-request-id", "canonical-stream-3-3"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			require.Equal(t, types.ActionContinue, action)
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
				{gatewayRequestIDHeader, "upstream-stream-first"},
				{gatewayRequestIDHeader, "upstream-stream-second"},
			})
			require.Equal(t, types.ActionContinue, action)
			assert.Equal(t, "canonical-stream-3-3", singleHeaderValue(t, host.GetResponseHeaders(), gatewayRequestIDHeader))

			action = host.CallOnHttpStreamingResponseBody([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n"), false)
			require.Equal(t, types.ActionContinue, action)
			action = host.CallOnHttpStreamingResponseBody([]byte("data: {\"model\":\"vendor-model\",\"usage\":{\"prompt_tokens\":4,\"completion_tokens\":6,\"total_tokens\":10}}\n\n"), true)
			require.Equal(t, types.ActionContinue, action)
			event := requireRedisBillingEvent(t, host)
			require.Equal(t, "canonical-stream-3-3", event["request_id"])
			require.Equal(t, event["event_id"], event["idempotency_key"])
			require.Equal(t, true, event["is_stream"])
			require.Len(t, host.GetRedisCalloutAttributes(), 1)

			host.CompleteHttp()
			require.Len(t, host.GetRedisCalloutAttributes(), 1)
			require.Equal(t, "canonical-stream-3-3", singleHeaderValue(t, host.GetResponseHeaders(), gatewayRequestIDHeader))
		})

		for _, testCase := range []struct {
			name        string
			codeDetails string
		}{
			{name: "cache replay", codeDetails: "ai-cache.hit"},
			{name: "gateway local response", codeDetails: "local_reply"},
		} {
			t.Run(testCase.name+" exposes the canonical value and truthful origin", func(t *testing.T) {
				host, status := test.NewTestHost(config)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				require.NoError(t, host.SetProperty([]string{"response", "code_details"}, []byte(testCase.codeDetails)))

				requestID := "canonical-" + strings.ReplaceAll(testCase.name, " ", "-") + "-3-3"
				action := host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "provider.example.com"},
					{":path", "/v1/chat/completions"},
					{":method", "POST"},
					{"x-request-id", requestID},
					{"x-tenant-id", "tenant-a"},
					{"x-consumer-id", "consumer-a"},
				})
				require.Equal(t, types.ActionContinue, action)
				action = host.CallOnHttpResponseHeaders([][2]string{
					{":status", "200"},
					{"content-type", "application/json"},
					{gatewayRequestIDHeader, "upstream-local-first"},
					{gatewayRequestIDHeader, "upstream-local-second"},
				})
				require.Equal(t, types.ActionContinue, action)
				assert.Equal(t, requestID, singleHeaderValue(t, host.GetResponseHeaders(), gatewayRequestIDHeader))

				action = host.CallOnHttpResponseBody([]byte(`{"model":"vendor-model","usage":{"prompt_tokens":4,"completion_tokens":6,"total_tokens":10}}`))
				require.Equal(t, types.ActionContinue, action)
				event := requireRedisBillingEvent(t, host)
				require.Equal(t, requestID, event["request_id"])
				require.Equal(t, false, event["upstream_invoked"])
				require.Equal(t, event["event_id"], event["idempotency_key"])
				require.Len(t, host.GetRedisCalloutAttributes(), 1)
			})
		}

		t.Run("missing canonical context removes every upstream value and invents nothing", func(t *testing.T) {
			host, status := test.NewTestHost(config)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
				{gatewayRequestIDHeader, "upstream-missing-first"},
				{gatewayRequestIDHeader, "upstream-missing-second"},
			})
			require.Equal(t, types.ActionContinue, action)
			assert.False(t, test.HasHeader(host.GetResponseHeaders(), gatewayRequestIDHeader))
			require.Empty(t, host.GetRedisCalloutAttributes())
		})

		t.Run("credentials and submitted header body query values never enter event or logs", func(t *testing.T) {
			host, status := test.NewTestHost(config)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			const (
				headerMarker = "billing-header-secret-3-3"
				bodyMarker   = "billing-body-secret-3-3"
				queryMarker  = "billing-query-secret-3-3"
			)
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "provider.example.com"},
				{":path", "/v1/chat/completions?opaque=" + queryMarker},
				{":method", "POST"},
				{"x-request-id", "canonical-security-3-3"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
				{"authorization", "Bearer " + headerMarker},
				{"x-private-marker", headerMarker},
			})
			require.Equal(t, types.ActionContinue, action)
			action = host.CallOnHttpRequestBody([]byte(`{"model":"vendor-model","messages":[{"role":"user","content":"` + bodyMarker + `"}]}`))
			require.Equal(t, types.ActionContinue, action)
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)
			action = host.CallOnHttpResponseBody([]byte(`{"model":"vendor-model","usage":{"prompt_tokens":4,"completion_tokens":6,"total_tokens":10}}`))
			require.Equal(t, types.ActionContinue, action)

			attrs := host.GetRedisCalloutAttributes()
			require.Len(t, attrs, 1)
			command := redisCommand(t, attrs[0].Query)
			require.Equal(t, []string{"xadd", defaultRedisStream, "*", "event"}, command[:4])
			body := command[4]
			logs := make([]string, 0)
			logs = append(logs, host.GetTraceLogs()...)
			logs = append(logs, host.GetDebugLogs()...)
			logs = append(logs, host.GetInfoLogs()...)
			logs = append(logs, host.GetWarnLogs()...)
			logs = append(logs, host.GetErrorLogs()...)
			logs = append(logs, host.GetCriticalLogs()...)
			allLogs := strings.Join(logs, "\n")

			for _, marker := range []string{
				"billing-user-secret-3-3",
				"billing-password-secret-3-3",
				headerMarker,
				bodyMarker,
				queryMarker,
			} {
				assert.NotContains(t, body, marker, "billing event exposed submitted or credential marker")
				assert.NotContains(t, allLogs, marker, "trace-through-critical logs exposed submitted or credential marker")
			}
		})
	})
}
