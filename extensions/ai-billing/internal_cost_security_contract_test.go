package main

import (
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestBillingInternalCostAndSensitiveDataContract(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		internalConfig := mustBillingConfig(t, map[string]interface{}{
			"event_kind":    eventKindInternalCost,
			"cost_source":   "route.fallback",
			"worker_kind":   "route-worker",
			"bill_customer": false,
			"redis_stream": map[string]interface{}{
				"service_name": "redis.static",
				"service_port": 6379,
				"timeout":      750,
			},
			"enable_path_suffixes": []string{"/v1/chat/completions"},
		})

		t.Run("complete valid pair is authoritative but cannot change event kind or bill customer", func(t *testing.T) {
			host, status := test.NewTestHost(internalConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "internal.example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-request-id", "internal-complete-3-3"},
				{"x-mse-tenant", "must-not-be-billed"},
				{"x-mse-consumer", "ai-memory-worker"},
				{internalCostSourceHeader, "ai-memory.digest"},
				{internalCostWorkerKindHeader, "digest"},
				{"x-ai-billing-source-job", "job-complete-3-3"},
				{"x-ai-billing-event-kind", eventKindUsage},
				{"x-ai-billing-bill-customer", "true"},
			})
			require.Equal(t, types.ActionContinue, action)
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)
			responseBody := []byte(`{"model":"vendor-model","usage":{"prompt_tokens":4,"completion_tokens":6,"total_tokens":10}}`)
			action = host.CallOnHttpResponseBody(responseBody)
			require.Equal(t, types.ActionContinue, action)
			require.Equal(t, responseBody, host.GetResponseBody())

			event := requireRedisBillingEvent(t, host)
			require.Equal(t, eventKindInternalCost, event["event_kind"])
			require.Equal(t, "ai-memory.digest", event["cost_source"])
			require.Equal(t, "digest", event["worker_kind"])
			require.Equal(t, "job-complete-3-3", event["source_job"])
			require.Equal(t, false, event["bill_customer"])
			require.Equal(t, "", event["tenant"])
			require.Equal(t, "ai-memory-worker", event["consumer"])
		})

		t.Run("absent pair retains route fallback", func(t *testing.T) {
			host, status := test.NewTestHost(internalConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "internal.example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-request-id", "internal-fallback-3-3"},
				{"x-mse-consumer", "ai-memory-worker"},
			})
			require.Equal(t, types.ActionContinue, action)
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)
			action = host.CallOnHttpResponseBody([]byte(`{"model":"vendor-model","usage":{"prompt_tokens":4,"completion_tokens":6,"total_tokens":10}}`))
			require.Equal(t, types.ActionContinue, action)

			event := requireRedisBillingEvent(t, host)
			require.Equal(t, eventKindInternalCost, event["event_kind"])
			require.Equal(t, "route.fallback", event["cost_source"])
			require.Equal(t, "route-worker", event["worker_kind"])
			require.Equal(t, false, event["bill_customer"])
		})

		for _, testCase := range []struct {
			name       string
			costSource string
			workerKind string
			marker     string
		}{
			{
				name:       "partial source-only pair",
				costSource: "partial-source-marker-3-3",
				marker:     "partial-source-marker-3-3",
			},
			{
				name:       "partial worker-only pair",
				workerKind: "partial-worker-marker-3-3",
				marker:     "partial-worker-marker-3-3",
			},
			{
				name:       "trimmed-empty value",
				costSource: " \t ",
				workerKind: "empty-worker-marker-3-3",
				marker:     "empty-worker-marker-3-3",
			},
			{
				name:       "oversized value",
				costSource: strings.Repeat("oversized-marker-", 9),
				workerKind: "oversized-worker-marker-3-3",
				marker:     "oversized-marker-",
			},
			{
				name:       "invalid-character value",
				costSource: "invalid/marker-3-3",
				workerKind: "invalid-worker-marker-3-3",
				marker:     "invalid/marker-3-3",
			},
		} {
			t.Run(testCase.name+" suppresses only event delivery without value leakage", func(t *testing.T) {
				host, status := test.NewTestHost(internalConfig)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)

				headers := [][2]string{
					{":authority", "internal.example.com"},
					{":path", "/v1/chat/completions"},
					{":method", "POST"},
					{"x-request-id", "internal-suppressed-3-3"},
					{"x-mse-consumer", "ai-memory-worker"},
				}
				if testCase.costSource != "" {
					headers = append(headers, [2]string{internalCostSourceHeader, testCase.costSource})
				}
				if testCase.workerKind != "" {
					headers = append(headers, [2]string{internalCostWorkerKindHeader, testCase.workerKind})
				}
				action := host.CallOnHttpRequestHeaders(headers)
				require.Equal(t, types.ActionContinue, action)
				action = host.CallOnHttpResponseHeaders([][2]string{
					{":status", "200"},
					{"content-type", "application/json"},
				})
				require.Equal(t, types.ActionContinue, action)
				responseBody := []byte(`{"model":"vendor-model","usage":{"prompt_tokens":4,"completion_tokens":6,"total_tokens":10}}`)
				action = host.CallOnHttpResponseBody(responseBody)
				require.Equal(t, types.ActionContinue, action)
				require.Equal(t, responseBody, host.GetResponseBody())
				require.Empty(t, host.GetRedisCalloutAttributes())

				logs := make([]string, 0)
				logs = append(logs, host.GetTraceLogs()...)
				logs = append(logs, host.GetDebugLogs()...)
				logs = append(logs, host.GetInfoLogs()...)
				logs = append(logs, host.GetWarnLogs()...)
				logs = append(logs, host.GetErrorLogs()...)
				logs = append(logs, host.GetCriticalLogs()...)
				require.NotContains(t, strings.Join(logs, "\n"), testCase.marker)
			})
		}

		t.Run("customer route cannot spoof internal fields or bill customer", func(t *testing.T) {
			customerConfig := mustBillingConfig(t, map[string]interface{}{
				"event_kind":    eventKindUsage,
				"bill_customer": true,
				"redis_stream": map[string]interface{}{
					"service_name": "redis.static",
					"service_port": 6379,
					"timeout":      750,
				},
				"enable_path_suffixes": []string{"/v1/chat/completions"},
			})
			host, status := test.NewTestHost(customerConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "customer.example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-request-id", "customer-no-spoof-3-3"},
				{"x-mse-tenant", "tenant-a"},
				{"x-mse-consumer", "consumer-a"},
				{internalCostSourceHeader, "ai-memory.digest"},
				{internalCostWorkerKindHeader, "digest"},
				{"x-ai-billing-source-job", "spoofed-source-job-3-3"},
				{"x-ai-billing-event-kind", eventKindInternalCost},
				{"x-ai-billing-bill-customer", "false"},
			})
			require.Equal(t, types.ActionContinue, action)
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)
			action = host.CallOnHttpResponseBody([]byte(`{"model":"vendor-model","usage":{"prompt_tokens":4,"completion_tokens":6,"total_tokens":10}}`))
			require.Equal(t, types.ActionContinue, action)

			event := requireRedisBillingEvent(t, host)
			require.Equal(t, eventKindUsage, event["event_kind"])
			require.Equal(t, "tenant-a", event["tenant"])
			require.Equal(t, "consumer-a", event["consumer"])
			require.NotContains(t, event, "cost_source")
			require.NotContains(t, event, "worker_kind")
			require.NotContains(t, event, "source_job")
			require.NotContains(t, event, "bill_customer")
		})
	})
}
