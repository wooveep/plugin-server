package main

import (
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestBillingTruthfulEventContract(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		t.Run("trustworthy provider usage remains authoritative with complete customer facts", func(t *testing.T) {
			config := mustBillingConfig(t, map[string]interface{}{
				"event_kind":      eventKindUsage,
				"quota_scope":     "tenant",
				"provider":        "vendor-native",
				"tenant_header":   "x-tenant-id",
				"consumer_header": "x-consumer-id",
				"redis_stream": map[string]interface{}{
					"service_name": "redis.static",
					"service_port": 6379,
					"stream":       defaultRedisStream,
					"timeout":      750,
				},
				"enable_path_suffixes": []string{"/native/v9/generate"},
			})
			host, status := test.NewTestHost(config)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("route-native"))
			require.NoError(t, host.SetClusterName("llm-vendor-native.dns"))
			require.NoError(t, host.SetRequestId("req-provider-usage"))
			require.NoError(t, host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "provider.example.com"},
				{":path", "/native/v9/generate"},
				{":method", "POST"},
				{"x-request-id", "req-provider-usage"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
				{"x-ai-price-version", "price-2026-07"},
			})
			require.Equal(t, types.ActionContinue, action)
			action = host.CallOnHttpRequestBody([]byte(`{
				"model":"vendor-model-9",
				"messages":[{"role":"user","content":"a deliberately long prompt that must not replace provider counts"}]
			}`))
			require.Equal(t, types.ActionContinue, action)
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)
			action = host.CallOnHttpResponseBody([]byte(`{
				"model":"vendor-model-9",
				"choices":[{"message":{"content":"a long answer that is also locally estimable"}}],
				"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}
			}`))
			require.Equal(t, types.ActionContinue, action)

			event := requireRedisBillingEvent(t, host)
			require.NotEmpty(t, event["event_id"])
			require.Equal(t, event["event_id"], event["idempotency_key"])
			require.Equal(t, eventKindUsage, event["event_kind"])
			require.Equal(t, "req-provider-usage", event["request_id"])
			require.Equal(t, "tenant-a", event["tenant"])
			require.Equal(t, "consumer-a", event["consumer"])
			require.Equal(t, "tenant", event["quota_scope"])
			requireObjectFactName(t, event, "route", "route-native")
			requireObjectFactName(t, event, "provider", "vendor-native")
			requireObjectFactName(t, event, "model", "vendor-model-9")
			require.Equal(t, "/native/v9/generate", event["request_path"])
			require.EqualValues(t, 200, event["status_code"])
			require.Equal(t, "llm-vendor-native.dns", event["cluster"])
			require.Equal(t, false, event["is_stream"])
			require.Equal(t, "price-2026-07", event["price_version"])
			require.Equal(t, true, event["upstream_invoked"])
			require.NotContains(t, event, "cost_source")
			require.NotContains(t, event, "worker_kind")
			require.NotContains(t, event, "source_job")
			require.NotContains(t, event, "bill_customer")
			require.Equal(t, false, event["usage_missing"])
			require.Equal(t, usageSourceProvider, event["usage_source"])

			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.Equal(t, "token", usage["unit"])
			require.EqualValues(t, 2, usage["input"])
			require.EqualValues(t, 3, usage["output"])
			require.EqualValues(t, 5, usage["total"])
			require.Equal(t, map[string]interface{}{
				"provider_usage": map[string]interface{}{
					"prompt_tokens":     float64(2),
					"completion_tokens": float64(3),
					"total_tokens":      float64(5),
				},
			}, usage["details"])
			require.NotContains(t, event, "input_tokens")
			require.NotContains(t, event, "output_tokens")
			require.NotContains(t, event, "total_tokens")
			require.NotContains(t, event, "gateway_calculated_cost")

			ackRedisBillingEvent(t, host)
			host.CompleteHttp()
		})

		t.Run("configured internal attribution and cache invocation facts remain distinct", func(t *testing.T) {
			t.Run("internal cost retains configured attribution surface", func(t *testing.T) {
				config := mustBillingConfig(t, map[string]interface{}{
					"event_kind":      eventKindInternalCost,
					"quota_scope":     "internal",
					"provider":        "vendor-native",
					"cost_source":     "ai-memory.digest",
					"worker_kind":     "digest",
					"bill_customer":   false,
					"consumer_header": "x-consumer-id",
					"redis_stream": map[string]interface{}{
						"service_name": "redis.static",
						"service_port": 6379,
						"stream":       defaultRedisStream,
						"timeout":      750,
					},
					"enable_path_suffixes": []string{"/native/v9/generate"},
				})
				host, status := test.NewTestHost(config)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				require.NoError(t, host.SetRouteName("route-internal"))
				require.NoError(t, host.SetClusterName("llm-vendor-native.dns"))
				require.NoError(t, host.SetRequestId("req-internal"))
				require.NoError(t, host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))

				action := host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "internal.example.com"},
					{":path", "/native/v9/generate"},
					{":method", "POST"},
					{"x-request-id", "req-internal"},
					{"x-tenant-id", "must-not-be-billed"},
					{"x-consumer-id", "ai-memory-worker"},
					{"x-ai-billing-source-job", "digest:job-42"},
				})
				require.Equal(t, types.ActionContinue, action)
				action = host.CallOnHttpResponseHeaders([][2]string{
					{":status", "200"},
					{"content-type", "application/json"},
				})
				require.Equal(t, types.ActionContinue, action)
				action = host.CallOnHttpResponseBody([]byte(`{
					"model":"vendor-model-9",
					"usage":{"prompt_tokens":4,"completion_tokens":6,"total_tokens":10}
				}`))
				require.Equal(t, types.ActionContinue, action)

				event := requireRedisBillingEvent(t, host)
				require.Equal(t, eventKindInternalCost, event["event_kind"])
				require.Equal(t, "ai-memory.digest", event["cost_source"])
				require.Equal(t, "digest", event["worker_kind"])
				require.Equal(t, "digest:job-42", event["source_job"])
				require.Equal(t, false, event["bill_customer"])
				require.Equal(t, "", event["tenant"])
				require.Equal(t, "ai-memory-worker", event["consumer"])
				require.Equal(t, "internal", event["quota_scope"])
				require.Equal(t, true, event["upstream_invoked"])
				require.Equal(t, false, event["usage_missing"])
				require.Equal(t, usageSourceProvider, event["usage_source"])

				ackRedisBillingEvent(t, host)
				host.CompleteHttp()
			})

			t.Run("customer cache hit retains trusted non-upstream fact", func(t *testing.T) {
				config := mustBillingConfig(t, map[string]interface{}{
					"event_kind":      eventKindUsage,
					"quota_scope":     "tenant",
					"provider":        "vendor-native",
					"tenant_header":   "x-tenant-id",
					"consumer_header": "x-consumer-id",
					"redis_stream": map[string]interface{}{
						"service_name": "redis.static",
						"service_port": 6379,
						"stream":       defaultRedisStream,
						"timeout":      750,
					},
					"enable_path_suffixes": []string{"/native/v9/generate"},
				})
				host, status := test.NewTestHost(config)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				require.NoError(t, host.SetRouteName("route-cache"))
				require.NoError(t, host.SetClusterName("llm-vendor-native.dns"))
				require.NoError(t, host.SetRequestId("req-cache"))
				require.NoError(t, host.SetProperty([]string{"response", "code_details"}, []byte("ai-cache.hit")))

				action := host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "provider.example.com"},
					{":path", "/native/v9/generate"},
					{":method", "POST"},
					{"x-request-id", "req-cache"},
					{"x-tenant-id", "tenant-a"},
					{"x-consumer-id", "consumer-a"},
				})
				require.Equal(t, types.ActionContinue, action)
				action = host.CallOnHttpRequestBody([]byte(`{
					"model":"vendor-model-9",
					"upstream_invoked":true,
					"messages":[{"role":"user","content":"cached request"}]
				}`))
				require.Equal(t, types.ActionContinue, action)
				action = host.CallOnHttpResponseHeaders([][2]string{
					{":status", "200"},
					{"content-type", "application/json"},
				})
				require.Equal(t, types.ActionContinue, action)
				action = host.CallOnHttpResponseBody([]byte(`{
					"model":"vendor-model-9",
					"upstream_invoked":true,
					"usage":{"prompt_tokens":4,"completion_tokens":6,"total_tokens":10}
				}`))
				require.Equal(t, types.ActionContinue, action)

				event := requireRedisBillingEvent(t, host)
				require.Equal(t, eventKindUsage, event["event_kind"])
				require.Equal(t, false, event["upstream_invoked"])
				require.NotContains(t, event, "cost_source")
				require.NotContains(t, event, "worker_kind")
				require.NotContains(t, event, "source_job")
				require.NotContains(t, event, "bill_customer")
				require.Equal(t, false, event["usage_missing"])
				require.Equal(t, usageSourceProvider, event["usage_source"])

				ackRedisBillingEvent(t, host)
				host.CompleteHttp()
			})
		})

		opaqueCases := []struct {
			name        string
			status      string
			statusCode  float64
			contentType string
			body        []byte
			stream      bool
		}{
			{
				name:        "non streaming success keeps unknown provider-native facts missing",
				status:      "202",
				statusCode:  202,
				contentType: "application/vnd.vendor.binary",
				body:        []byte{0x00, 0x01, 0x02, 0x03},
			},
			{
				name:        "non streaming error keeps unknown provider-native facts missing",
				status:      "502",
				statusCode:  502,
				contentType: "application/problem+vendor",
				body:        []byte(`{"vendor_error":{"opaque_ref":"failure-7"}}`),
			},
			{
				name:        "streaming success keeps unknown provider-native facts missing",
				status:      "200",
				statusCode:  200,
				contentType: "text/event-stream",
				body:        []byte("event: vendor\ndata: {\"binary_ref\":\"blob-7\"}\n\n"),
				stream:      true,
			},
		}
		for _, testCase := range opaqueCases {
			t.Run(testCase.name, func(t *testing.T) {
				config := mustBillingConfig(t, map[string]interface{}{
					"event_kind":      eventKindUsage,
					"quota_scope":     "tenant",
					"provider":        "vendor-native",
					"tenant_header":   "x-tenant-id",
					"consumer_header": "x-consumer-id",
					"redis_stream": map[string]interface{}{
						"service_name": "redis.static",
						"service_port": 6379,
						"stream":       defaultRedisStream,
						"timeout":      750,
					},
					"enable_path_suffixes": []string{"/native/v9/jobs/job-42"},
				})
				host, status := test.NewTestHost(config)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				require.NoError(t, host.SetRouteName("route-native"))
				require.NoError(t, host.SetClusterName("llm-vendor-native.dns"))
				require.NoError(t, host.SetRequestId("req-opaque"))
				require.NoError(t, host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))

				action := host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "provider.example.com"},
					{":path", "/native/v9/jobs/job-42"},
					{":method", "POST"},
					{"x-request-id", "req-opaque"},
					{"x-tenant-id", "tenant-a"},
					{"x-consumer-id", "consumer-a"},
				})
				require.Equal(t, types.ActionContinue, action)
				action = host.CallOnHttpRequestBody([]byte{0x10, 0x11, 0x12})
				require.Equal(t, types.ActionContinue, action)
				action = host.CallOnHttpResponseHeaders([][2]string{
					{":status", testCase.status},
					{"content-type", testCase.contentType},
				})
				require.Equal(t, types.ActionContinue, action)
				if testCase.stream {
					action = host.CallOnHttpStreamingResponseBody(testCase.body, true)
				} else {
					action = host.CallOnHttpResponseBody(testCase.body)
				}
				require.Equal(t, types.ActionContinue, action)
				require.Equal(t, testCase.body, host.GetResponseBody())

				event := requireRedisBillingEvent(t, host)
				require.NotEmpty(t, event["event_id"])
				require.Equal(t, event["event_id"], event["idempotency_key"])
				require.Equal(t, eventKindUsage, event["event_kind"])
				require.Equal(t, "req-opaque", event["request_id"])
				require.Equal(t, "tenant-a", event["tenant"])
				require.Equal(t, "consumer-a", event["consumer"])
				requireObjectFactName(t, event, "route", "route-native")
				requireObjectFactName(t, event, "provider", "vendor-native")
				require.Equal(t, "/native/v9/jobs/job-42", event["request_path"])
				require.Equal(t, testCase.statusCode, event["status_code"])
				require.Equal(t, testCase.stream, event["is_stream"])
				require.Equal(t, true, event["upstream_invoked"])
				require.Equal(t, true, event["usage_missing"])
				require.Equal(t, usageSourceMissing, event["usage_source"])
				require.NotContains(t, event, "price_version")
				require.NotContains(t, event, "gateway_calculated_cost")
				require.NotContains(t, event, "cost")
				require.NotContains(t, event, "unit_price")

				usage, ok := event["usage"].(map[string]interface{})
				require.True(t, ok)
				require.Equal(t, "token", usage["unit"])
				require.EqualValues(t, 0, usage["input"])
				require.EqualValues(t, 0, usage["output"])
				require.EqualValues(t, 0, usage["total"])
				require.Equal(t, map[string]interface{}{}, usage["details"])
				require.NotContains(t, usage, "provider_usage")
				require.NotContains(t, usage, "input_tokens")
				require.NotContains(t, usage, "output_tokens")
				require.NotContains(t, usage, "total_tokens")
				requireObjectFactName(t, event, "model", "")

				ackRedisBillingEvent(t, host)
				host.CompleteHttp()
			})
		}
	})
}
