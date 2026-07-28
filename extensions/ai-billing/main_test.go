package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/iface"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/higress-group/wasm-go/pkg/tokenusage"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/resp"
)

var billingConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"quota_scope":     "global",
		"provider":        "openai",
		"tenant_header":   "x-tenant-id",
		"consumer_header": "x-consumer-id",
		"redis_stream": map[string]interface{}{
			"service_name": "redis.static",
			"service_port": 6379,
			"database":     2,
			"timeout":      750,
			"stream":       "billing:events",
		},
		"enable_path_suffixes": []string{
			"/v1/chat/completions",
			"/v1/messages",
		},
	})
	return data
}()

var billingConfigDefaultConsumer = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"quota_scope":   "global",
		"provider":      "openai",
		"tenant_header": "x-tenant-id",
		"redis_stream": map[string]interface{}{
			"service_name": "redis.static",
			"service_port": 6379,
			"timeout":      750,
		},
		"enable_path_suffixes": []string{
			"/v1/chat/completions",
			"/v1/messages",
		},
	})
	return data
}()

var billingConfigWithRedisDefaults = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"quota_scope":     "global",
		"provider":        "openai",
		"tenant_header":   "x-tenant-id",
		"consumer_header": "x-consumer-id",
		"redis_stream": map[string]interface{}{
			"service_name": "redis.static",
			"service_port": 6379,
			"timeout":      750,
		},
	})
	return data
}()

func mustBillingConfig(t *testing.T, value map[string]interface{}) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return data
}

func TestParseConfig(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		t.Run("redis stream target and default fail policy", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusOK, status)
			config, err := host.GetMatchConfig()
			require.NoError(t, err)

			billingConfig := config.(*BillingConfig)
			require.Equal(t, "redis.static", billingConfig.RedisStream.ServiceName)
			require.Equal(t, 6379, billingConfig.RedisStream.ServicePort)
			require.Equal(t, 2, billingConfig.RedisStream.Database)
			require.Equal(t, int64(750), billingConfig.RedisStream.Timeout)
			require.Equal(t, "billing:events", billingConfig.RedisStream.Stream)
			require.Equal(t, "global", billingConfig.QuotaScope)
			require.Equal(t, "openai", billingConfig.Provider)
			require.Equal(t, "x-tenant-id", billingConfig.TenantHeader)
			require.Equal(t, "x-consumer-id", billingConfig.ConsumerHeader)
			require.Equal(t, FailPolicyOpen, billingConfig.FailPolicy)
		})

		t.Run("redis stream defaults optional fields", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfigWithRedisDefaults)
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusOK, status)
			config, err := host.GetMatchConfig()
			require.NoError(t, err)

			billingConfig := config.(*BillingConfig)
			require.Equal(t, 6379, billingConfig.RedisStream.ServicePort)
			require.Equal(t, 0, billingConfig.RedisStream.Database)
			require.Equal(t, int64(750), billingConfig.RedisStream.Timeout)
			require.Equal(t, defaultRedisStream, billingConfig.RedisStream.Stream)
		})

		t.Run("default consumer header", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfigDefaultConsumer)
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusOK, status)
			config, err := host.GetMatchConfig()
			require.NoError(t, err)

			billingConfig := config.(*BillingConfig)
			require.Equal(t, defaultConsumerHeader, billingConfig.ConsumerHeader)
		})

		t.Run("legacy customer usage event kind and rule override internal cost", func(t *testing.T) {
			host, status := test.NewTestHost(mustBillingConfig(t, map[string]interface{}{
				"event_kind": "customer_usage",
				"redis_stream": map[string]interface{}{
					"service_name": "redis.static",
				},
				"_rules_": []map[string]interface{}{
					{
						"_match_route_": []string{"route-internal-cost"},
						"event_kind":    "internal_cost",
						"quota_scope":   "internal:ai-memory.digest",
						"cost_source":   "ai-memory.embedding",
						"worker_kind":   "embedding",
						"bill_customer": false,
					},
				},
			}))
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusOK, status)
			config, err := host.GetMatchConfig()
			require.NoError(t, err)
			billingConfig := config.(*BillingConfig)
			require.Equal(t, "usage", billingConfig.EventKind)

			require.NoError(t, host.SetRouteName("route-internal-cost"))
			config, err = host.GetMatchConfig()
			require.NoError(t, err)
			billingConfig = config.(*BillingConfig)
			require.Equal(t, eventKindInternalCost, billingConfig.EventKind)
			require.Equal(t, "internal:ai-memory.digest", billingConfig.QuotaScope)
			require.Equal(t, "ai-memory.embedding", billingConfig.CostSource)
			require.Equal(t, "embedding", billingConfig.WorkerKind)
			require.NotNil(t, billingConfig.BillCustomer)
			require.False(t, *billingConfig.BillCustomer)
		})

		t.Run("usage event kind is accepted", func(t *testing.T) {
			host, status := test.NewTestHost(mustBillingConfig(t, map[string]interface{}{
				"event_kind": "usage",
				"redis_stream": map[string]interface{}{
					"service_name": "redis.static",
				},
			}))
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusOK, status)
			config, err := host.GetMatchConfig()
			require.NoError(t, err)
			billingConfig := config.(*BillingConfig)
			require.Equal(t, "usage", billingConfig.EventKind)
		})

		t.Run("unsupported event kind fails parsing", func(t *testing.T) {
			host, status := test.NewTestHost(mustBillingConfig(t, map[string]interface{}{
				"event_kind": "invoice",
				"redis_stream": map[string]interface{}{
					"service_name": "redis.static",
				},
			}))
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})

		t.Run("global redis stream with defaultable fields", func(t *testing.T) {
			host, status := test.NewTestHost(mustBillingConfig(t, map[string]interface{}{
				"redis_stream": map[string]interface{}{
					"service_name": "redis.static",
				},
			}))
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusOK, status)
			config, err := host.GetMatchConfig()
			require.NoError(t, err)

			billingConfig := config.(*BillingConfig)
			require.Equal(t, "redis.static", billingConfig.RedisStream.ServiceName)
			require.Equal(t, defaultRedisPort, billingConfig.RedisStream.ServicePort)
			require.Equal(t, int64(defaultRedisTimeout), billingConfig.RedisStream.Timeout)
			require.Equal(t, defaultRedisStream, billingConfig.RedisStream.Stream)
			require.Equal(t, defaultQuotaScope, billingConfig.QuotaScope)
			require.Equal(t, defaultProvider, billingConfig.Provider)
			require.Equal(t, defaultTenantHeader, billingConfig.TenantHeader)
			require.Equal(t, defaultConsumerHeader, billingConfig.ConsumerHeader)
			require.Equal(t, []string{"/v1/chat/completions", "/v1/messages"}, billingConfig.EnablePathSuffixes)
			require.Equal(t, FailPolicyOpen, billingConfig.FailPolicy)
		})

		t.Run("partial rule inherits redis stream and overrides selected fields", func(t *testing.T) {
			host, status := test.NewTestHost(mustBillingConfig(t, map[string]interface{}{
				"quota_scope":          "global-scope",
				"provider":             "openai",
				"tenant_header":        "x-tenant-id",
				"consumer_header":      "x-consumer-id",
				"enable_path_suffixes": []string{"/v1/chat/completions"},
				"fail_policy":          FailPolicyOpen,
				"redis_stream": map[string]interface{}{
					"service_name": "redis.static",
					"service_port": 6379,
					"timeout":      750,
				},
				"_rules_": []map[string]interface{}{
					{
						"_match_route_": []string{"route-provider"},
						"provider":      "anthropic",
						"quota_scope":   "route-scope",
					},
				},
			}))
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("route-provider"))
			config, err := host.GetMatchConfig()
			require.NoError(t, err)

			billingConfig := config.(*BillingConfig)
			require.Equal(t, "redis.static", billingConfig.RedisStream.ServiceName)
			require.Equal(t, 6379, billingConfig.RedisStream.ServicePort)
			require.Equal(t, int64(750), billingConfig.RedisStream.Timeout)
			require.Equal(t, defaultRedisStream, billingConfig.RedisStream.Stream)
			require.Equal(t, "route-scope", billingConfig.QuotaScope)
			require.Equal(t, "anthropic", billingConfig.Provider)
			require.Equal(t, "x-tenant-id", billingConfig.TenantHeader)
			require.Equal(t, "x-consumer-id", billingConfig.ConsumerHeader)
			require.Equal(t, []string{"/v1/chat/completions"}, billingConfig.EnablePathSuffixes)
			require.Equal(t, FailPolicyOpen, billingConfig.FailPolicy)
		})

		t.Run("partial rule only overrides provider and inherits redis stream", func(t *testing.T) {
			host, status := test.NewTestHost(mustBillingConfig(t, map[string]interface{}{
				"quota_scope":          "global-scope",
				"provider":             "openai",
				"tenant_header":        "x-tenant-id",
				"consumer_header":      "x-consumer-id",
				"enable_path_suffixes": []string{"/v1/chat/completions"},
				"fail_policy":          FailPolicyOpen,
				"redis_stream": map[string]interface{}{
					"service_name": "redis.static",
					"service_port": 6379,
					"timeout":      750,
				},
				"_rules_": []map[string]interface{}{
					{
						"_match_route_": []string{"route-provider-only"},
						"provider":      "anthropic",
					},
				},
			}))
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("route-provider-only"))
			config, err := host.GetMatchConfig()
			require.NoError(t, err)

			billingConfig := config.(*BillingConfig)
			require.Equal(t, "redis.static", billingConfig.RedisStream.ServiceName)
			require.Equal(t, 6379, billingConfig.RedisStream.ServicePort)
			require.Equal(t, int64(750), billingConfig.RedisStream.Timeout)
			require.Equal(t, defaultRedisStream, billingConfig.RedisStream.Stream)
			require.Equal(t, "global-scope", billingConfig.QuotaScope)
			require.Equal(t, "anthropic", billingConfig.Provider)
			require.Equal(t, "x-tenant-id", billingConfig.TenantHeader)
			require.Equal(t, "x-consumer-id", billingConfig.ConsumerHeader)
			require.Equal(t, []string{"/v1/chat/completions"}, billingConfig.EnablePathSuffixes)
			require.Equal(t, FailPolicyOpen, billingConfig.FailPolicy)
		})

		t.Run("partial rule overrides path suffixes and inherits fail policy", func(t *testing.T) {
			host, status := test.NewTestHost(mustBillingConfig(t, map[string]interface{}{
				"provider":             "openai",
				"enable_path_suffixes": []string{"/v1/chat/completions"},
				"fail_policy":          FailPolicyOpen,
				"redis_stream": map[string]interface{}{
					"service_name": "redis.static",
				},
				"_rules_": []map[string]interface{}{
					{
						"_match_route_":        []string{"route-custom-path"},
						"enable_path_suffixes": []string{"/custom/ai"},
						"fail_policy":          FailPolicyOpen,
					},
				},
			}))
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("route-custom-path"))
			config, err := host.GetMatchConfig()
			require.NoError(t, err)

			billingConfig := config.(*BillingConfig)
			require.Equal(t, "openai", billingConfig.Provider)
			require.Equal(t, []string{"/custom/ai"}, billingConfig.EnablePathSuffixes)
			require.Equal(t, FailPolicyOpen, billingConfig.FailPolicy)
		})

		t.Run("unsupported rule fail policy fails rule parsing", func(t *testing.T) {
			host, status := test.NewTestHost(mustBillingConfig(t, map[string]interface{}{
				"provider":    "openai",
				"fail_policy": FailPolicyOpen,
				"redis_stream": map[string]interface{}{
					"service_name": "redis.static",
				},
				"_rules_": []map[string]interface{}{
					{
						"_match_route_": []string{"route-invalid-fail-policy"},
						"fail_policy":   "close",
					},
				},
			}))
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})

		t.Run("rule-level redis stream is rejected", func(t *testing.T) {
			host, status := test.NewTestHost(mustBillingConfig(t, map[string]interface{}{
				"provider": "openai",
				"redis_stream": map[string]interface{}{
					"service_name": "redis.static",
				},
				"_rules_": []map[string]interface{}{
					{
						"_match_route_": []string{"route-full"},
						"provider":      "dashscope",
						"redis_stream": map[string]interface{}{
							"service_name": "redis.route",
							"service_port": 6380,
							"timeout":      900,
						},
					},
				},
			}))
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})

		t.Run("missing global redis stream fails even when rule has redis stream", func(t *testing.T) {
			host, status := test.NewTestHost(mustBillingConfig(t, map[string]interface{}{
				"provider": "openai",
				"_rules_": []map[string]interface{}{
					{
						"_match_route_": []string{"route-full"},
						"redis_stream": map[string]interface{}{
							"service_name": "redis.route",
						},
					},
				},
			}))
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})
	})
}

func TestBillingEventDelivery(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("non-ai path does not dispatch", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/healthz"},
				{":method", "GET"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpStreamingRequestBody([]byte(`{"secret":"do-not-buffer"}`), false)
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
				{gatewayRequestIDHeader, "provider-spoofed-id"},
			})
			require.Equal(t, types.ActionContinue, action)
			require.False(t, test.HasHeader(host.GetResponseHeaders(), gatewayRequestIDHeader))

			action = host.CallOnHttpResponseBody([]byte(`{"ok":true}`))
			require.Equal(t, types.ActionContinue, action)
			require.Empty(t, host.GetRedisCalloutAttributes())
		})

		t.Run("internal cost preserves task consumer without tenant billing identity", func(t *testing.T) {
			host, status := test.NewTestHost(mustBillingConfig(t, map[string]interface{}{
				"event_kind":      eventKindInternalCost,
				"cost_source":     "internal",
				"worker_kind":     "internal",
				"bill_customer":   false,
				"consumer_header": defaultConsumerHeader,
				"redis_stream": map[string]interface{}{
					"service_name": "redis.static",
					"service_port": 6379,
					"timeout":      750,
				},
				"enable_path_suffixes": []string{"/v1/chat/completions"},
			}))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "internal.example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-mse-tenant", "must-not-be-billed"},
				{"x-mse-consumer", "ai-memory-digest"},
				{"x-ai-billing-source-job", "ai-memory-digest:job-1"},
			})
			require.Equal(t, types.ActionContinue, action)
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
				{gatewayRequestIDHeader, "provider-spoofed-id"},
			})
			require.Equal(t, types.ActionContinue, action)
			responseHeaders := host.GetResponseHeaders()
			responseRequestID := singleHeaderValue(t, responseHeaders, gatewayRequestIDHeader)
			action = host.CallOnHttpResponseBody([]byte(`{"model":"deepseek-v4-flash","usage":{"prompt_tokens":5,"completion_tokens":8,"total_tokens":13}}`))
			require.Equal(t, types.ActionContinue, action)

			event := requireRedisBillingEvent(t, host)
			require.Equal(t, eventKindInternalCost, event["event_kind"])
			require.Equal(t, "ai-memory-digest", event["consumer"])
			require.Equal(t, "", event["tenant"])
			require.Equal(t, "internal", event["cost_source"])
			require.Equal(t, "internal", event["worker_kind"])
			require.Equal(t, "ai-memory-digest:job-1", event["source_job"])
			require.Equal(t, false, event["bill_customer"])
			require.Equal(t, event["request_id"], responseRequestID)

			ackRedisBillingEvent(t, host)
			host.CompleteHttp()
		})

		t.Run("internal cost invocation attribution overrides generic route fallback", func(t *testing.T) {
			config := mustBillingConfig(t, map[string]interface{}{
				"event_kind":    eventKindInternalCost,
				"cost_source":   "internal",
				"worker_kind":   "internal",
				"bill_customer": false,
				"redis_stream": map[string]interface{}{
					"service_name": "redis.static",
					"service_port": 6379,
					"timeout":      750,
				},
				"enable_path_suffixes": []string{"/v1/chat/completions"},
			})
			cases := []struct {
				name       string
				costSource string
				workerKind string
			}{
				{"digest", "ai-memory.digest", "digest"},
				{"extraction", "ai-memory.extraction", "extraction"},
				{"embedding", "ai-memory.embedding", "embedding"},
			}

			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					host, status := test.NewTestHost(config)
					defer host.Reset()
					require.Equal(t, types.OnPluginStartStatusOK, status)

					action := host.CallOnHttpRequestHeaders([][2]string{
						{":authority", "internal.example.com"},
						{":path", "/v1/chat/completions"},
						{":method", "POST"},
						{"x-mse-tenant", "must-not-be-billed"},
						{"x-mse-consumer", "ai-memory-worker"},
						{"x-ai-billing-source-job", "ai-memory:job-1"},
						{"x-ai-billing-cost-source", tc.costSource},
						{"x-ai-billing-worker-kind", tc.workerKind},
					})
					require.Equal(t, types.ActionContinue, action)
					action = host.CallOnHttpResponseHeaders([][2]string{
						{":status", "200"},
						{"content-type", "application/json"},
					})
					require.Equal(t, types.ActionContinue, action)
					action = host.CallOnHttpResponseBody([]byte(`{"model":"deepseek-v4-flash","usage":{"prompt_tokens":5,"completion_tokens":8,"total_tokens":13}}`))
					require.Equal(t, types.ActionContinue, action)

					event := requireRedisBillingEvent(t, host)
					require.Equal(t, tc.costSource, event["cost_source"])
					require.Equal(t, tc.workerKind, event["worker_kind"])
					require.Equal(t, "ai-memory:job-1", event["source_job"])
					require.Equal(t, eventKindInternalCost, event["event_kind"])
					require.Empty(t, event["tenant"])
					require.Equal(t, false, event["bill_customer"])

					ackRedisBillingEvent(t, host)
					host.CompleteHttp()
				})
			}
		})

		t.Run("unsafe internal attribution suppresses delivery without blocking model traffic", func(t *testing.T) {
			config := mustBillingConfig(t, map[string]interface{}{
				"event_kind":    eventKindInternalCost,
				"cost_source":   "internal",
				"worker_kind":   "internal",
				"bill_customer": false,
				"redis_stream": map[string]interface{}{
					"service_name": "redis.static",
					"service_port": 6379,
					"timeout":      750,
				},
				"enable_path_suffixes": []string{"/v1/chat/completions"},
			})
			cases := []struct {
				name       string
				costSource string
				workerKind string
			}{
				{"source only", "ai-memory.digest", ""},
				{"worker only", "", "digest"},
				{"oversized source", strings.Repeat("x", 129), "digest"},
				{"invalid source characters", "ai-memory.digest\nspoof", "digest"},
			}

			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					host, status := test.NewTestHost(config)
					defer host.Reset()
					require.Equal(t, types.OnPluginStartStatusOK, status)

					action := host.CallOnHttpRequestHeaders([][2]string{
						{":authority", "internal.example.com"},
						{":path", "/v1/chat/completions"},
						{":method", "POST"},
						{"x-ai-billing-cost-source", tc.costSource},
						{"x-ai-billing-worker-kind", tc.workerKind},
					})
					require.Equal(t, types.ActionContinue, action)
					action = host.CallOnHttpResponseHeaders([][2]string{
						{":status", "200"},
						{"content-type", "application/json"},
					})
					require.Equal(t, types.ActionContinue, action)
					action = host.CallOnHttpResponseBody([]byte(`{"model":"deepseek-v4-flash","usage":{"prompt_tokens":5,"completion_tokens":8,"total_tokens":13}}`))
					require.Equal(t, types.ActionContinue, action)

					require.Empty(t, host.GetRedisCalloutAttributes())
					host.CompleteHttp()
				})
			}
		})

		t.Run("ordinary customer route ignores internal attribution headers", func(t *testing.T) {
			host, status := test.NewTestHost(mustBillingConfig(t, map[string]interface{}{
				"event_kind":    eventKindUsage,
				"bill_customer": true,
				"redis_stream": map[string]interface{}{
					"service_name": "redis.static",
					"service_port": 6379,
					"timeout":      750,
				},
				"enable_path_suffixes": []string{"/v1/chat/completions"},
			}))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "customer.example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-mse-tenant", "tenant-a"},
				{"x-ai-billing-cost-source", "ai-memory.digest"},
				{"x-ai-billing-worker-kind", "digest"},
			})
			require.Equal(t, types.ActionContinue, action)
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)
			action = host.CallOnHttpResponseBody([]byte(`{"model":"deepseek-v4-flash","usage":{"prompt_tokens":5,"completion_tokens":8,"total_tokens":13}}`))
			require.Equal(t, types.ActionContinue, action)

			event := requireRedisBillingEvent(t, host)
			require.Equal(t, eventKindUsage, event["event_kind"])
			require.Equal(t, "tenant-a", event["tenant"])
			require.NotContains(t, event, "cost_source")
			require.NotContains(t, event, "worker_kind")
			require.NotContains(t, event, "bill_customer")

			ackRedisBillingEvent(t, host)
			host.CompleteHttp()
		})

		t.Run("successful event includes request facts", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("route-a"))
			require.NoError(t, host.SetClusterName("cluster-a"))
			require.NoError(t, host.SetRequestId("req-1"))

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-request-id", "req-1"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
				{"x-ai-price-version", "pv-7"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpResponseBody([]byte(`{"id":"chat-1","model":"gpt-4","usage":{"prompt_tokens":5,"completion_tokens":8,"total_tokens":13}}`))
			require.Equal(t, types.ActionContinue, action)

			event := requireRedisBillingEvent(t, host)
			require.NotEmpty(t, event["event_id"])
			require.Equal(t, "req-1", event["request_id"])
			require.NotEmpty(t, event["idempotency_key"])
			require.NotEqual(t, "req-1", event["idempotency_key"])
			require.Equal(t, "tenant-a", event["tenant"])
			require.Equal(t, "consumer-a", event["consumer"])
			requireObjectFactName(t, event, "provider", "")
			requireObjectFactName(t, event, "model", "gpt-4")
			requireObjectFactName(t, event, "route", "route-a")
			require.Equal(t, "cluster-a", event["cluster"])
			require.Equal(t, "/v1/chat/completions", event["request_path"])
			require.EqualValues(t, 200, event["status_code"])
			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.Equal(t, "token", usage["unit"])
			require.EqualValues(t, 5, usage["input"])
			require.EqualValues(t, 8, usage["output"])
			require.EqualValues(t, 13, usage["total"])
			require.Equal(t, map[string]interface{}{
				"provider_usage": map[string]interface{}{
					"prompt_tokens":     float64(5),
					"completion_tokens": float64(8),
					"total_tokens":      float64(13),
				},
			}, usage["details"])
			require.Equal(t, false, event["usage_missing"])
			require.Equal(t, "provider", event["usage_source"])
			require.Equal(t, false, event["is_stream"])
			require.Equal(t, "pv-7", event["price_version"])
			require.Equal(t, "global", event["quota_scope"])
			require.NotContains(t, event, "input_tokens")
			require.NotContains(t, event, "output_tokens")
			require.NotContains(t, event, "total_tokens")
			require.NotContains(t, event, "gateway_calculated_cost")

			ackRedisBillingEvent(t, host)
			host.CompleteHttp()
		})

		t.Run("provider usage without supported cache fields emits only basic usage details", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{"model":"gpt-4","usage":{"prompt_tokens":5,"prompt_tokens_details":{"audio_tokens":2},"completion_tokens":8,"completion_tokens_details":{"reasoning_tokens":3},"total_tokens":13}}`))
			require.Equal(t, types.ActionContinue, action)
			event := requireRedisBillingEvent(t, host)
			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.EqualValues(t, 5, usage["input"])
			require.EqualValues(t, 8, usage["output"])
			require.EqualValues(t, 13, usage["total"])
			require.NotContains(t, usage, "input_cache_hit_tokens")
			require.NotContains(t, usage, "input_cache_miss_tokens")
			require.NotContains(t, usage, "output_tokens")
			require.Equal(t, map[string]interface{}{
				"provider_usage": map[string]interface{}{
					"prompt_tokens": float64(5),
					"prompt_tokens_details": map[string]interface{}{
						"audio_tokens": float64(2),
					},
					"completion_tokens": float64(8),
					"completion_tokens_details": map[string]interface{}{
						"reasoning_tokens": float64(3),
					},
					"total_tokens": float64(13),
				},
			}, usage["details"])
			require.Equal(t, false, event["usage_missing"])
			require.Equal(t, usageSourceProvider, event["usage_source"])

			ackRedisBillingEvent(t, host)
			host.CompleteHttp()
		})

		t.Run("provider usage prefers prompt and completion tokens over zero input output aliases", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{"model":"deepseek-v4-pro","usage":{"prompt_tokens":9,"completion_tokens":146,"completion_tokens_details":{"reasoning_tokens":136,"text_tokens":0},"input_tokens":0,"output_tokens":0,"total_tokens":155}}`))
			require.Equal(t, types.ActionContinue, action)
			event := requireRedisBillingEvent(t, host)
			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.EqualValues(t, 9, usage["input"])
			require.EqualValues(t, 146, usage["output"])
			require.EqualValues(t, 155, usage["total"])
			require.Equal(t, usageSourceProvider, event["usage_source"])

			ackRedisBillingEvent(t, host)
			host.CompleteHttp()
		})

		t.Run("provider usage derives missing input from total and output tokens", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{"model":"deepseek-v4-pro","usage":{"input_tokens":0,"completion_tokens":146,"output_tokens":0,"total_tokens":155}}`))
			require.Equal(t, types.ActionContinue, action)
			event := requireRedisBillingEvent(t, host)
			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.EqualValues(t, 9, usage["input"])
			require.EqualValues(t, 146, usage["output"])
			require.EqualValues(t, 155, usage["total"])
			require.Equal(t, usageSourceProvider, event["usage_source"])

			ackRedisBillingEvent(t, host)
			host.CompleteHttp()
		})

		t.Run("provider usage estimates missing input when only output tokens are present", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			inputText := "hello input"

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			action := host.CallOnHttpRequestBody([]byte(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"` + inputText + `"}]}`))
			require.Equal(t, types.ActionContinue, action)
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action = host.CallOnHttpResponseBody([]byte(`{"model":"gpt-4o-mini","usage":{"completion_tokens":3}}`))
			require.Equal(t, types.ActionContinue, action)
			event := requireRedisBillingEvent(t, host)
			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.Greater(t, int64(usage["input"].(float64)), int64(0))
			require.EqualValues(t, 3, usage["output"])
			require.Equal(t, usage["input"].(float64)+usage["output"].(float64), usage["total"])
			require.Equal(t, usageSourceEstimated, event["usage_source"])

			ackRedisBillingEvent(t, host)
			host.CompleteHttp()
		})

		t.Run("provider usage takes precedence over estimation and retains raw usage", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			action := host.CallOnHttpRequestBody([]byte(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"a very long prompt that tokenizer estimation could count"}]}`))
			require.Equal(t, types.ActionContinue, action)
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action = host.CallOnHttpResponseBody([]byte(`{"model":"gpt-4o-mini","choices":[{"message":{"content":"a very long assistant answer that is structurally estimable"}}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`))
			require.Equal(t, types.ActionContinue, action)
			event := requireRedisBillingEvent(t, host)
			require.Equal(t, false, event["usage_missing"])
			require.Equal(t, usageSourceProvider, event["usage_source"])

			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
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

			ackRedisBillingEvent(t, host)
			host.CompleteHttp()
		})

		t.Run("matched rules keep provider empty and preserve quota scope", func(t *testing.T) {
			config := mustBillingConfig(t, map[string]interface{}{
				"quota_scope":          "global-scope",
				"provider":             "openai",
				"tenant_header":        "x-tenant-id",
				"consumer_header":      "x-consumer-id",
				"enable_path_suffixes": []string{"/v1/chat/completions"},
				"redis_stream": map[string]interface{}{
					"service_name": "redis.static",
					"service_port": 6379,
					"timeout":      750,
				},
				"_rules_": []map[string]interface{}{
					{
						"_match_route_": []string{"route-openai"},
						"provider":      "openai-route",
					},
					{
						"_match_route_": []string{"route-anthropic"},
						"provider":      "anthropic-route",
						"quota_scope":   "route-scope",
					},
				},
			})

			openaiEvent := deliverTestBillingEvent(t, config, "route-openai", "req-openai")
			requireObjectFactName(t, openaiEvent, "provider", "")
			require.Equal(t, "global-scope", openaiEvent["quota_scope"])

			anthropicEvent := deliverTestBillingEvent(t, config, "route-anthropic", "req-anthropic")
			requireObjectFactName(t, anthropicEvent, "provider", "")
			require.Equal(t, "route-scope", anthropicEvent["quota_scope"])
		})

		t.Run("rule-level redis stream config fails to start", func(t *testing.T) {
			host, status := test.NewTestHost(mustBillingConfig(t, map[string]interface{}{
				"provider":             "openai",
				"tenant_header":        "x-tenant-id",
				"consumer_header":      "x-consumer-id",
				"enable_path_suffixes": []string{"/v1/chat/completions"},
				"redis_stream": map[string]interface{}{
					"service_name": "redis.static",
					"service_port": 6379,
					"timeout":      750,
				},
				"_rules_": []map[string]interface{}{
					{
						"_match_route_": []string{"route-full-runtime"},
						"provider":      "dashscope",
						"redis_stream": map[string]interface{}{
							"service_name": "redis.route",
							"service_port": 6380,
							"timeout":      900,
						},
					},
				},
			}))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status)
			require.Empty(t, host.GetRedisCalloutAttributes())
		})

		t.Run("each ai request gets new event identity", func(t *testing.T) {
			buildEvent := func(requestID string) map[string]interface{} {
				host, status := test.NewTestHost(billingConfig)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)

				host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "example.com"},
					{":path", "/v1/chat/completions"},
					{":method", "POST"},
					{"x-request-id", requestID},
					{"x-tenant-id", "tenant-a"},
					{"x-consumer-id", "consumer-a"},
				})
				host.CallOnHttpResponseHeaders([][2]string{
					{":status", "200"},
					{"content-type", "application/json"},
				})
				host.CallOnHttpResponseBody([]byte(`{"model":"gpt-4","usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
				event := requireRedisBillingEvent(t, host)
				eventID, ok := event["event_id"].(string)
				require.True(t, ok)
				require.NotEmpty(t, eventID)
				require.Equal(t, eventID, event["idempotency_key"])
				parsed, err := uuid.Parse(eventID)
				require.NoError(t, err)
				require.Equal(t, uuid.Version(7), parsed.Version())
				return event
			}

			events := make([]map[string]interface{}, 0, 2)
			for i := 0; i < 2; i++ {
				events = append(events, buildEvent("req-"+strconv.Itoa(i+1)))
			}
			require.NotEqual(t, events[0]["event_id"], events[1]["event_id"])
		})

		t.Run("payload omits credentials and forbidden identity fields", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("route-secure"))
			require.NoError(t, host.SetClusterName("cluster-secure"))
			require.NoError(t, host.SetRequestId("req-sensitive"))

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-request-id", "req-sensitive"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
				{"authorization", "Bearer <raw-api-key>"},
				{"x-api-key", "<raw-api-key>"},
				{"x-user-id", "user-id-sample"},
				{"x-api-key-id", "<api-key-id>"},
				{"x-ai-price-version", "pv-secure"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpResponseBody([]byte(`{"id":"chat-1","model":"gpt-4","usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`))
			require.Equal(t, types.ActionContinue, action)

			bodyBytes := requireRedisBillingEventBody(t, host)
			body := string(bodyBytes)
			var event map[string]interface{}
			require.NoError(t, json.Unmarshal(bodyBytes, &event))

			forbiddenFields := []string{
				"tenant_id",
				"user_id",
				"api_key_id",
				"consumer_id",
			}
			for _, key := range forbiddenFields {
				_, hasKey := event[key]
				require.False(t, hasKey)
			}
			require.Equal(t, "tenant-a", event["tenant"])
			require.NotContains(t, body, "user-id-sample")
			require.NotContains(t, body, "<raw-api-key>")
			require.NotContains(t, body, "<api-key-id>")

			ackRedisBillingEvent(t, host)
			host.CompleteHttp()
		})

		t.Run("x-request-id header takes precedence over x_request_id property", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.NoError(t, host.SetRequestId("property-request-id"))
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-request-id", "header-request-id"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpResponseBody([]byte(`{"id":"chat-1","model":"gpt-4","usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`))
			require.Equal(t, types.ActionContinue, action)
			event := requireRedisBillingEvent(t, host)
			require.Equal(t, "header-request-id", event["request_id"])

			ackRedisBillingEvent(t, host)
			host.CompleteHttp()
		})

		t.Run("x_request_id property is fallback request_id source when header is absent", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.NoError(t, host.SetRequestId("property-request-id"))
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpResponseBody([]byte(`{"id":"chat-1","model":"gpt-4","usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`))
			require.Equal(t, types.ActionContinue, action)
			event := requireRedisBillingEvent(t, host)
			require.Equal(t, "property-request-id", event["request_id"])

			ackRedisBillingEvent(t, host)
			host.CompleteHttp()
		})

		t.Run("default consumer header uses x-mse-consumer", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfigDefaultConsumer)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"X-Mse-Consumer", "consumer-a"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpResponseBody([]byte(`{"model":"gpt-4","usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
			require.Equal(t, types.ActionContinue, action)
			event := requireRedisBillingEvent(t, host)
			require.Equal(t, "consumer-a", event["consumer"])
			ackRedisBillingEvent(t, host)
			host.CompleteHttp()
		})

		t.Run("streaming response sets stream state", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("route-a"))
			require.NoError(t, host.SetClusterName("cluster-a"))
			require.NoError(t, host.SetRequestId("req-2"))

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-request-id", "req-2"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpStreamingResponseBody([]byte("data: {\"model\":\"gpt-4\"}\n\n"), true)
			require.Equal(t, types.ActionContinue, action)

			require.Contains(t, string(requireRedisBillingEventBody(t, host)), `"is_stream":true`)

			ackRedisBillingEvent(t, host)
			host.CompleteHttp()
		})

		t.Run("streaming cache usage maps explicit cache aware usage", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})
			action := host.CallOnHttpStreamingResponseBody([]byte("data: {\"model\":\"gpt-4\",\"usage\":{\"prompt_tokens\":9,\"prompt_tokens_details\":{\"cached_tokens\":4},\"completion_tokens\":6,\"total_tokens\":15}}\n\n"), true)
			require.Equal(t, types.ActionContinue, action)
			event := requireRedisBillingEvent(t, host)
			require.Equal(t, true, event["is_stream"])
			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.EqualValues(t, 9, usage["input"])
			require.EqualValues(t, 6, usage["output"])
			require.EqualValues(t, 15, usage["total"])
			require.EqualValues(t, 4, usage["input_cache_hit_tokens"])
			require.EqualValues(t, 5, usage["input_cache_miss_tokens"])
			require.EqualValues(t, 6, usage["output_tokens"])

			ackRedisBillingEvent(t, host)
			host.CompleteHttp()
		})

		t.Run("kimi top-level cached tokens map cache aware usage", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{"model":"moonshot-v1","usage":{"prompt_tokens":12,"cached_tokens":5,"completion_tokens":6,"total_tokens":18}}`))
			require.Equal(t, types.ActionContinue, action)
			event := requireRedisBillingEvent(t, host)
			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.EqualValues(t, 12, usage["input"])
			require.EqualValues(t, 6, usage["output"])
			require.EqualValues(t, 18, usage["total"])
			require.EqualValues(t, 5, usage["input_cache_hit_tokens"])
			require.EqualValues(t, 7, usage["input_cache_miss_tokens"])
			require.EqualValues(t, 6, usage["output_tokens"])
			require.Equal(t, map[string]interface{}{
				"input": map[string]interface{}{
					"cached_tokens": float64(5),
				},
				"provider_usage": map[string]interface{}{
					"prompt_tokens":     float64(12),
					"cached_tokens":     float64(5),
					"completion_tokens": float64(6),
					"total_tokens":      float64(18),
				},
			}, usage["details"])

			ackRedisBillingEvent(t, host)
			host.CompleteHttp()
		})

		t.Run("cached tokens clamp to provider input tokens", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{"model":"gpt-4","usage":{"prompt_tokens":5,"prompt_tokens_details":{"cached_tokens":8},"completion_tokens":2,"total_tokens":7}}`))
			require.Equal(t, types.ActionContinue, action)
			event := requireRedisBillingEvent(t, host)
			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.EqualValues(t, 5, usage["input"])
			require.EqualValues(t, 5, usage["input_cache_hit_tokens"])
			require.EqualValues(t, 0, usage["input_cache_miss_tokens"])
			require.EqualValues(t, 2, usage["output_tokens"])

			ackRedisBillingEvent(t, host)
			host.CompleteHttp()
		})

		t.Run("deepseek explicit cache split maps hit and miss usage", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{"model":"deepseek-chat","usage":{"prompt_tokens":11,"prompt_cache_hit_tokens":4,"prompt_cache_miss_tokens":7,"completion_tokens":2,"total_tokens":13}}`))
			require.Equal(t, types.ActionContinue, action)
			event := requireRedisBillingEvent(t, host)
			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.EqualValues(t, 11, usage["input"])
			require.EqualValues(t, 2, usage["output"])
			require.EqualValues(t, 13, usage["total"])
			require.EqualValues(t, 4, usage["input_cache_hit_tokens"])
			require.EqualValues(t, 7, usage["input_cache_miss_tokens"])
			require.EqualValues(t, 2, usage["output_tokens"])
			require.Equal(t, map[string]interface{}{
				"input": map[string]interface{}{
					"prompt_cache_hit_tokens":  float64(4),
					"prompt_cache_miss_tokens": float64(7),
				},
				"provider_usage": map[string]interface{}{
					"prompt_tokens":            float64(11),
					"prompt_cache_hit_tokens":  float64(4),
					"prompt_cache_miss_tokens": float64(7),
					"completion_tokens":        float64(2),
					"total_tokens":             float64(13),
				},
			}, usage["details"])

			ackRedisBillingEvent(t, host)
			host.CompleteHttp()
		})

		t.Run("deepseek zero cache split preserves provider input as miss usage", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{"id":"chatcmpl-test","model":"deepseek-v4-pro","usage":{"prompt_tokens":12,"completion_tokens":223,"total_tokens":235,"prompt_tokens_details":{"audio_tokens":0,"cached_tokens":0,"image_tokens":0,"text_tokens":0},"completion_tokens_details":{"audio_tokens":0,"reasoning_tokens":166,"text_tokens":0},"input_tokens":0,"output_tokens":0,"input_tokens_details":null,"prompt_cache_hit_tokens":0,"prompt_cache_miss_tokens":0}}`))
			require.Equal(t, types.ActionContinue, action)
			event := requireRedisBillingEvent(t, host)
			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.EqualValues(t, 12, usage["input"])
			require.EqualValues(t, 223, usage["output"])
			require.EqualValues(t, 235, usage["total"])
			require.EqualValues(t, 0, usage["input_cache_hit_tokens"])
			require.EqualValues(t, 12, usage["input_cache_miss_tokens"])
			require.EqualValues(t, 223, usage["output_tokens"])

			ackRedisBillingEvent(t, host)
			host.CompleteHttp()
		})

		t.Run("streaming provider usage details merge across chunks", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})

			action := host.CallOnHttpStreamingResponseBody([]byte("data: {\"model\":\"gpt-4\",\"usage\":{\"prompt_tokens\":5,\"prompt_tokens_details\":{\"cached_tokens\":2}}}\n\n"), false)
			require.Equal(t, types.ActionContinue, action)
			action = host.CallOnHttpStreamingResponseBody([]byte("data: {\"model\":\"gpt-4\",\"usage\":{\"completion_tokens\":8,\"total_tokens\":13,\"prompt_tokens_details\":{\"audio_tokens\":1},\"completion_tokens_details\":{\"reasoning_tokens\":3}}}\n\n"), true)
			require.Equal(t, types.ActionContinue, action)
			event := requireRedisBillingEvent(t, host)
			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.EqualValues(t, 5, usage["input"])
			require.EqualValues(t, 8, usage["output"])
			require.EqualValues(t, 13, usage["total"])
			details, ok := usage["details"].(map[string]interface{})
			require.True(t, ok)
			require.Equal(t, map[string]interface{}{
				"input": map[string]interface{}{
					"cached_tokens": float64(2),
				},
				"provider_usage": map[string]interface{}{
					"prompt_tokens": float64(5),
					"prompt_tokens_details": map[string]interface{}{
						"cached_tokens": float64(2),
						"audio_tokens":  float64(1),
					},
					"completion_tokens": float64(8),
					"total_tokens":      float64(13),
					"completion_tokens_details": map[string]interface{}{
						"reasoning_tokens": float64(3),
					},
				},
			}, details)

			ackRedisBillingEvent(t, host)
			host.CompleteHttp()
		})

		t.Run("usage missing payload is still emitted", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			host.CallOnHttpResponseBody([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
			body := requireRedisBillingEventBody(t, host)
			var event map[string]interface{}
			require.NoError(t, json.Unmarshal(body, &event))
			require.Equal(t, true, event["usage_missing"])
			require.Equal(t, "missing", event["usage_source"])

			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.Equal(t, "token", usage["unit"])
			require.EqualValues(t, 0, usage["input"])
			require.EqualValues(t, 0, usage["output"])
			require.EqualValues(t, 0, usage["total"])
			require.Equal(t, map[string]interface{}{}, usage["details"])
			require.NotContains(t, string(body), `"input_tokens":`)
			require.NotContains(t, string(body), `"output_tokens":`)
			require.NotContains(t, string(body), `"total_tokens":`)
			ackRedisBillingEvent(t, host)
			host.CompleteHttp()
		})

		t.Run("request text is stored for estimation without raw payload details", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
				{"authorization", "Bearer sk-sensitive"},
			})
			action := host.CallOnHttpRequestBody([]byte(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello input"}],"metadata":{"api_key":"sk-sensitive","trace":"raw-secret"}}`))
			require.Equal(t, types.ActionContinue, action)
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			host.CallOnHttpResponseBody([]byte(`{"model":"gpt-4o-mini","choices":[{"message":{"content":"hello output"}}]}`))

			body := requireRedisBillingEventBody(t, host)
			require.NotContains(t, string(body), "sk-sensitive")
			require.NotContains(t, string(body), "raw-secret")
			require.NotContains(t, string(body), "messages")

			var event map[string]interface{}
			require.NoError(t, json.Unmarshal(body, &event))
			require.Equal(t, false, event["usage_missing"])
			require.Equal(t, usageSourceEstimated, event["usage_source"])
			requireObjectFactName(t, event, "model", "gpt-4o-mini")
			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.ElementsMatch(t, []string{"unit", "input", "output", "total"}, mapKeys(usage))
			require.Greater(t, int64(usage["input"].(float64)), int64(0))
			require.Greater(t, int64(usage["output"].(float64)), int64(0))

			ackRedisBillingEvent(t, host)
			host.CompleteHttp()
		})

		t.Run("structured token usage maps details and total fallback", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			host.CallOnHttpResponseBody([]byte(`{"model":"gpt-4","usage":{"prompt_tokens":5,"prompt_tokens_details":{"cached_tokens":2},"completion_tokens":8,"completion_tokens_details":{"reasoning_tokens":3}}}`))
			event := requireRedisBillingEvent(t, host)

			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.EqualValues(t, 5, usage["input"])
			require.EqualValues(t, 8, usage["output"])
			require.EqualValues(t, 13, usage["total"])
			require.EqualValues(t, 2, usage["input_cache_hit_tokens"])
			require.EqualValues(t, 3, usage["input_cache_miss_tokens"])
			require.EqualValues(t, 8, usage["output_tokens"])
			require.Equal(t, map[string]interface{}{
				"input": map[string]interface{}{
					"cached_tokens": float64(2),
				},
				"output": map[string]interface{}{
					"reasoning_tokens": float64(3),
				},
				"provider_usage": map[string]interface{}{
					"prompt_tokens": float64(5),
					"prompt_tokens_details": map[string]interface{}{
						"cached_tokens": float64(2),
					},
					"completion_tokens": float64(8),
					"completion_tokens_details": map[string]interface{}{
						"reasoning_tokens": float64(3),
					},
				},
			}, usage["details"])
			require.Equal(t, false, event["usage_missing"])

			ackRedisBillingEvent(t, host)
			host.CompleteHttp()
		})

		t.Run("anthropic cache usage maps explicit cache aware usage", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/messages"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			host.CallOnHttpResponseBody([]byte(`{"message":{"model":"claude-3-5-sonnet","usage":{"input_tokens":10,"output_tokens":7}},"usage":{"cache_creation_input_tokens":4,"cache_read_input_tokens":3}}`))
			event := requireRedisBillingEvent(t, host)

			usage, ok := event["usage"].(map[string]interface{})
			require.True(t, ok)
			require.EqualValues(t, 17, usage["input"])
			require.EqualValues(t, 7, usage["output"])
			require.EqualValues(t, 24, usage["total"])
			require.EqualValues(t, 3, usage["input_cache_hit_tokens"])
			require.EqualValues(t, 14, usage["input_cache_miss_tokens"])
			require.EqualValues(t, 7, usage["output_tokens"])
			require.Equal(t, map[string]interface{}{
				"input": map[string]interface{}{
					"cache_creation_input_tokens": float64(4),
					"cache_read_input_tokens":     float64(3),
				},
				"provider_usage": map[string]interface{}{
					"input_tokens":                float64(10),
					"output_tokens":               float64(7),
					"cache_creation_input_tokens": float64(4),
					"cache_read_input_tokens":     float64(3),
				},
			}, usage["details"])
			require.Equal(t, false, event["usage_missing"])

			ackRedisBillingEvent(t, host)
			host.CompleteHttp()
		})

		t.Run("redis delivery response errors are fail open", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{"model":"gpt-4","usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
			require.Equal(t, types.ActionContinue, action)
			require.Len(t, host.GetRedisCalloutAttributes(), 1)

			host.CallOnRedisCall(0, test.CreateRedisRespError("temporary"))
			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			host.CompleteHttp()
		})

		t.Run("billing plugin only appends redis stream event", func(t *testing.T) {
			host, status := test.NewTestHost(billingConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			host.CallOnHttpResponseBody([]byte(`{"model":"gpt-4","usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))

			attrs := host.GetRedisCalloutAttributes()
			require.Len(t, attrs, 1)
			command := redisCommand(t, attrs[0].Query)
			require.Equal(t, "xadd", command[0])
			require.Equal(t, defaultRedisStream, command[1])
			ackRedisBillingEvent(t, host)
			host.CompleteHttp()
		})
	})
}

func TestBillingResponseRequestIDHeader(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for _, testCase := range []struct {
			name        string
			contentType string
		}{
			{name: "non-streaming", contentType: "application/json"},
			{name: "streaming", contentType: "text/event-stream"},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				host, status := test.NewTestHost(billingConfig)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)

				host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "example.com"},
					{":path", "/v1/chat/completions"},
					{":method", "POST"},
					{"x-request-id", "gateway-request-1"},
				})
				host.CallOnHttpResponseHeaders([][2]string{
					{":status", "200"},
					{"content-type", testCase.contentType},
					{gatewayRequestIDHeader, "provider-spoofed-id"},
				})

				require.Equal(
					t,
					"gateway-request-1",
					singleHeaderValue(t, host.GetResponseHeaders(), gatewayRequestIDHeader),
				)
			})
		}

	})
}

func singleHeaderValue(t *testing.T, headers [][2]string, name string) string {
	t.Helper()
	values := make([]string, 0, 1)
	for _, header := range headers {
		if strings.EqualFold(header[0], name) {
			values = append(values, header[1])
		}
	}
	require.Len(t, values, 1)
	return values[0]
}

func deliverTestBillingEvent(t *testing.T, config json.RawMessage, routeName, requestID string) map[string]interface{} {
	t.Helper()
	host, status := test.NewTestHost(config)
	defer host.Reset()
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName(routeName))

	action := host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"},
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
	})
	require.Equal(t, types.ActionContinue, action)

	action = host.CallOnHttpResponseBody([]byte(`{"id":"chat-1","model":"gpt-4","usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`))
	require.Equal(t, types.ActionContinue, action)
	event := requireRedisBillingEvent(t, host)
	ackRedisBillingEvent(t, host)
	host.CompleteHttp()
	return event
}

func requireObjectFactName(t *testing.T, event map[string]interface{}, key, name string) {
	t.Helper()
	value, ok := event[key].(map[string]interface{})
	require.True(t, ok, "%s must be serialized as an object", key)
	if name == "" {
		require.Empty(t, value)
		return
	}
	require.Equal(t, name, value["name"])
	_, hasID := value["id"]
	require.False(t, hasID, "%s.id should be omitted when no stable Console id is available", key)
}

func requireRedisBillingEvent(t *testing.T, host test.TestHost) map[string]interface{} {
	t.Helper()
	body := requireRedisBillingEventBody(t, host)
	var event map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &event))
	return event
}

func requireRedisBillingEventStruct(t *testing.T, host test.TestHost) BillingEvent {
	t.Helper()
	body := requireRedisBillingEventBody(t, host)
	var event BillingEvent
	require.NoError(t, json.Unmarshal(body, &event))
	return event
}

func requireRedisBillingEventBody(t *testing.T, host test.TestHost) []byte {
	t.Helper()
	attrs := host.GetRedisCalloutAttributes()
	require.Len(t, attrs, 1)
	require.Regexp(t, `^outbound\|6379\|\|redis\.static(\?.*)?$`, attrs[0].Upstream)
	command := redisCommand(t, attrs[0].Query)
	require.Equal(t, []string{"xadd", "billing:events", "*", "event"}, command[:4])
	return []byte(command[4])
}

func redisCommand(t *testing.T, query []byte) []string {
	t.Helper()
	value, _, err := resp.NewReader(bytes.NewReader(query)).ReadValue()
	require.NoError(t, err)
	values := value.Array()
	require.Len(t, values, 5)
	command := make([]string, 0, len(values))
	for _, value := range values {
		command = append(command, value.String())
	}
	return command
}

func ackRedisBillingEvent(t *testing.T, host test.TestHost) {
	t.Helper()
	host.CallOnRedisCall(0, test.CreateRedisRespString("1700000000000-0"))
}

func mapKeys(values map[string]interface{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func TestPathFiltering(t *testing.T) {
	require.True(t, isAIPathEnabled("/proxy/v1/chat/completions?x=1", []string{"/v1/chat/completions"}))
	require.False(t, isAIPathEnabled("/proxy/not-ai", []string{"/v1/chat/completions"}))
}

func TestProviderSlugFromCluster(t *testing.T) {
	require.Equal(t, "qwen-019ebb2c", providerSlugFromCluster("llm-qwen-019ebb2c.internal.dns"))
	require.Equal(t, "qwen-019ebb2c", providerSlugFromCluster("outbound|443||llm-qwen-019ebb2c.internal.dns"))
	require.Equal(t, "qwen-019ebb2c", providerSlugFromCluster("llm-qwen-019ebb2c.dns"))
	require.Equal(t, "qwen-019ebb2c", providerSlugFromCluster("  llm-qwen-019ebb2c.internal.dns  "))
	require.Empty(t, providerSlugFromCluster("outbound|bogus||llm-qwen.internal.dns"))
	require.Empty(t, providerSlugFromCluster("llm-qwen|beta.internal.dns"))
	require.Empty(t, providerSlugFromCluster("llm--.dns"))
	require.Empty(t, providerSlugFromCluster("llm-a-.dns"))
	require.Empty(t, providerSlugFromCluster("llm-a-.internal.dns"))
	require.Empty(t, providerSlugFromCluster("outbound|443||llm--.internal.dns"))
	require.Empty(t, providerSlugFromCluster(""))
	require.Empty(t, providerSlugFromCluster("cluster-a"))
	require.Empty(t, providerSlugFromCluster("llm-.internal.dns"))
	require.Empty(t, providerSlugFromCluster("outbound|443||cluster-a"))
}

func TestQwenCacheAwareUsagePreservesCacheCreationDetails(t *testing.T) {
	host, status := test.NewTestHost(billingConfig)
	defer host.Reset()
	require.Equal(t, types.OnPluginStartStatusOK, status)

	host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"},
		{":path", "/v1/chat/completions"},
		{":method", "POST"},
		{"x-tenant-id", "tenant-a"},
		{"x-consumer-id", "consumer-a"},
	})
	host.CallOnHttpResponseHeaders([][2]string{
		{":status", "200"},
		{"content-type", "application/json"},
	})
	action := host.CallOnHttpResponseBody([]byte(`{"model":"qwen-plus","usage":{"input_tokens":16,"output_tokens":5,"total_tokens":21,"prompt_tokens_details":{"cached_tokens":6,"cache_creation":{"ephemeral_5m_input_tokens":2}}}}`))
	require.Equal(t, types.ActionContinue, action)
	event := requireRedisBillingEvent(t, host)
	requireObjectFactName(t, event, "model", "qwen-plus")
	require.Equal(t, usageSourceProvider, event["usage_source"])

	usage, ok := event["usage"].(map[string]interface{})
	require.True(t, ok)
	require.EqualValues(t, 16, usage["input"])
	require.EqualValues(t, 5, usage["output"])
	require.EqualValues(t, 21, usage["total"])
	require.EqualValues(t, 6, usage["input_cache_hit_tokens"])
	require.EqualValues(t, 10, usage["input_cache_miss_tokens"])
	require.Equal(t, map[string]interface{}{
		"input": map[string]interface{}{
			"cached_tokens": float64(6),
		},
		"provider_usage": map[string]interface{}{
			"input_tokens":  float64(16),
			"output_tokens": float64(5),
			"total_tokens":  float64(21),
			"prompt_tokens_details": map[string]interface{}{
				"cached_tokens": float64(6),
				"cache_creation": map[string]interface{}{
					"ephemeral_5m_input_tokens": float64(2),
				},
			},
		},
	}, usage["details"])
}

func TestResponsesCacheReplayPreservesCanonicalCacheAwareUsage(t *testing.T) {
	config := mustBillingConfig(t, map[string]interface{}{
		"quota_scope":     "global",
		"provider":        "openai",
		"tenant_header":   "x-tenant-id",
		"consumer_header": "x-consumer-id",
		"redis_stream": map[string]interface{}{
			"service_name": "redis.static",
			"service_port": 6379,
			"database":     2,
			"timeout":      750,
			"stream":       "billing:events",
		},
		"enable_path_suffixes": []string{"/v1/responses"},
	})
	host, status := test.NewTestHost(config)
	defer host.Reset()
	require.Equal(t, types.OnPluginStartStatusOK, status)

	host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"},
		{":path", "/v1/responses"},
		{":method", "POST"},
		{"x-tenant-id", "tenant-a"},
		{"x-consumer-id", "consumer-a"},
	})
	host.CallOnHttpResponseHeaders([][2]string{
		{":status", "200"},
		{"content-type", "application/json"},
	})
	action := host.CallOnHttpResponseBody([]byte(`{
		"id":"resp_cache_replay_fixture",
		"model":"deepseek-v4-flash",
		"object":"response",
		"output_text":"cached answer",
		"status":"completed",
		"usage":{
			"input_tokens":683,
			"input_cache_hit_tokens":683,
			"input_cache_miss_tokens":0,
			"output_tokens":146,
			"total_tokens":829
		}
	}`))
	require.Equal(t, types.ActionContinue, action)
	event := requireRedisBillingEvent(t, host)
	requireObjectFactName(t, event, "model", "deepseek-v4-flash")
	require.Equal(t, usageSourceProvider, event["usage_source"])

	usage, ok := event["usage"].(map[string]interface{})
	require.True(t, ok)
	require.EqualValues(t, 683, usage["input"])
	require.EqualValues(t, 146, usage["output"])
	require.EqualValues(t, 829, usage["total"])
	require.EqualValues(t, 683, usage["input_cache_hit_tokens"])
	require.EqualValues(t, 0, usage["input_cache_miss_tokens"])
	require.EqualValues(t, 146, usage["output_tokens"])
	require.Equal(t, map[string]interface{}{
		"provider_usage": map[string]interface{}{
			"input_tokens":            float64(683),
			"input_cache_hit_tokens":  float64(683),
			"input_cache_miss_tokens": float64(0),
			"output_tokens":           float64(146),
			"total_tokens":            float64(829),
		},
	}, usage["details"])
}

func TestCacheAwareProviderUsageSplitValidatesCanonicalTotals(t *testing.T) {
	tests := []struct {
		name         string
		inputTokens  int64
		provider     map[string]any
		wantHit      int64
		wantMiss     int64
		wantAccepted bool
	}{
		{
			name:        "valid split",
			inputTokens: 21,
			provider: map[string]any{
				"input_cache_hit_tokens":  float64(13),
				"input_cache_miss_tokens": float64(8),
			},
			wantHit:      13,
			wantMiss:     8,
			wantAccepted: true,
		},
		{
			name:        "missing miss field",
			inputTokens: 21,
			provider: map[string]any{
				"input_cache_hit_tokens": float64(13),
			},
		},
		{
			name:        "negative hit field",
			inputTokens: 21,
			provider: map[string]any{
				"input_cache_hit_tokens":  float64(-1),
				"input_cache_miss_tokens": float64(22),
			},
		},
		{
			name:        "split does not equal input",
			inputTokens: 21,
			provider: map[string]any{
				"input_cache_hit_tokens":  float64(13),
				"input_cache_miss_tokens": float64(7),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hit, miss, accepted := cacheAwareProviderUsageSplit(tc.inputTokens, tc.provider)
			require.Equal(t, tc.wantAccepted, accepted)
			require.Equal(t, tc.wantHit, hit)
			require.Equal(t, tc.wantMiss, miss)
		})
	}
}

func TestGeminiCacheAwareUsageMapsCachedContentTokens(t *testing.T) {
	host, status := test.NewTestHost(billingConfig)
	defer host.Reset()
	require.Equal(t, types.OnPluginStartStatusOK, status)

	host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"},
		{":path", "/v1/chat/completions"},
		{":method", "POST"},
		{"x-tenant-id", "tenant-a"},
		{"x-consumer-id", "consumer-a"},
	})
	host.CallOnHttpResponseHeaders([][2]string{
		{":status", "200"},
		{"content-type", "application/json"},
	})
	action := host.CallOnHttpResponseBody([]byte(`{"modelVersion":"gemini-2.5-pro","usageMetadata":{"promptTokenCount":20,"cachedContentTokenCount":9,"candidatesTokenCount":3,"totalTokenCount":23}}`))
	require.Equal(t, types.ActionContinue, action)
	event := requireRedisBillingEvent(t, host)
	requireObjectFactName(t, event, "model", "gemini-2.5-pro")
	require.Equal(t, usageSourceProvider, event["usage_source"])

	usage, ok := event["usage"].(map[string]interface{})
	require.True(t, ok)
	require.EqualValues(t, 20, usage["input"])
	require.EqualValues(t, 3, usage["output"])
	require.EqualValues(t, 23, usage["total"])
	require.EqualValues(t, 9, usage["input_cache_hit_tokens"])
	require.EqualValues(t, 11, usage["input_cache_miss_tokens"])
	require.Equal(t, map[string]interface{}{
		"input": map[string]interface{}{
			"cached_content_token_count": float64(9),
		},
		"provider_usage": map[string]interface{}{
			"promptTokenCount":        float64(20),
			"cachedContentTokenCount": float64(9),
			"candidatesTokenCount":    float64(3),
			"totalTokenCount":         float64(23),
		},
	}, usage["details"])
}

func TestCacheAwareInputTokenSplit(t *testing.T) {
	tests := []struct {
		name       string
		input      int64
		details    map[string]int64
		wantHit    int64
		wantMiss   int64
		wantMapped bool
	}{
		{
			name:  "cached tokens clamp to input",
			input: 10,
			details: map[string]int64{
				tokenusage.InputTokenDetailsKeyCachedTokens: 12,
			},
			wantHit:    10,
			wantMiss:   0,
			wantMapped: true,
		},
		{
			name:  "negative cached tokens become zero",
			input: 10,
			details: map[string]int64{
				tokenusage.InputTokenDetailsKeyCachedTokens: -2,
			},
			wantHit:    0,
			wantMiss:   10,
			wantMapped: true,
		},
		{
			name:  "gemini cached content clamps to input",
			input: 10,
			details: map[string]int64{
				tokenusage.InputTokenDetailsKeyGeminiCachedContentTokenCount: 12,
			},
			wantHit:    10,
			wantMiss:   0,
			wantMapped: true,
		},
		{
			name:  "deepseek explicit split preserves provider hit and miss",
			input: 10,
			details: map[string]int64{
				tokenusage.InputTokenDetailsKeyDeepSeekPromptCacheHitTokens:  8,
				tokenusage.InputTokenDetailsKeyDeepSeekPromptCacheMissTokens: 7,
			},
			wantHit:    8,
			wantMiss:   7,
			wantMapped: true,
		},
		{
			name:  "deepseek zero explicit split falls back to input as miss",
			input: 10,
			details: map[string]int64{
				tokenusage.InputTokenDetailsKeyDeepSeekPromptCacheHitTokens:  0,
				tokenusage.InputTokenDetailsKeyDeepSeekPromptCacheMissTokens: 0,
			},
			wantHit:    0,
			wantMiss:   10,
			wantMapped: true,
		},
		{
			name:  "deepseek negative explicit split falls back to input as miss",
			input: 10,
			details: map[string]int64{
				tokenusage.InputTokenDetailsKeyDeepSeekPromptCacheHitTokens:  -2,
				tokenusage.InputTokenDetailsKeyDeepSeekPromptCacheMissTokens: -3,
			},
			wantHit:    0,
			wantMiss:   10,
			wantMapped: true,
		},
		{
			name:  "deepseek miss only preserves provider miss",
			input: 10,
			details: map[string]int64{
				tokenusage.InputTokenDetailsKeyDeepSeekPromptCacheMissTokens: 12,
			},
			wantHit:    0,
			wantMiss:   12,
			wantMapped: true,
		},
		{
			name:  "anthropic negative cache values become zero",
			input: -10,
			details: map[string]int64{
				tokenusage.InputTokenDetailsKeyAnthropicMessagesUsageCacheCreationInputTokens: -4,
				tokenusage.InputTokenDetailsKeyAnthropicMessagesUsageCacheReadInputTokens:     -3,
			},
			wantHit:    0,
			wantMiss:   0,
			wantMapped: true,
		},
		{
			name:       "unsupported details do not map cache split",
			input:      10,
			details:    map[string]int64{"audio_tokens": 2},
			wantMapped: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hitTokens, missTokens, ok := cacheAwareInputTokenSplit(tc.input, tc.details)

			require.Equal(t, tc.wantMapped, ok)
			require.EqualValues(t, tc.wantHit, hitTokens)
			require.EqualValues(t, tc.wantMiss, missTokens)
		})
	}
}

func TestDeliverBillingEventDispatchErrorIsFailOpen(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		client := &recordingBillingRedisClient{err: errors.New("network unavailable")}
		ctx := &mockBillingHttpContext{values: map[string]interface{}{}}
		eventID, err := initBillingRequestContext(ctx, "/v1/chat/completions", "req-dispatch", "tenant-a", "consumer-a", "openai", "global", "")
		require.NoError(t, err)
		ctx.SetContext(ctxEventID, eventID)
		ctx.SetContext(ctxStatusCode, http.StatusOK)
		ctx.SetContext(ctxInputToken, int64(1))
		ctx.SetContext(ctxOutputToken, int64(1))
		ctx.SetContext(ctxTotalToken, int64(2))

		deliverBillingEvent(ctx, BillingConfig{
			Provider: "openai",
			RedisStream: RedisStream{
				Stream: defaultRedisStream,
			},
			redisClient: client,
		}, false)

		require.Len(t, client.commands, 1)
		require.True(t, ctx.GetBoolContext(ctxBillingDelivered, false))
	})
}

func TestSendBillingEventReusesEventIdempotencyKey(t *testing.T) {
	client := &recordingBillingRedisClient{}
	event := BillingEvent{
		EventID:        "018f4c7c-1111-7abc-8111-111111111111",
		IdempotencyKey: "018f4c7c-1111-7abc-8111-111111111111",
		RequestID:      "req-retry",
		Consumer:       "consumer-a",
		Route:          BillingFact{Name: "route-a"},
		Provider:       BillingFact{Name: "openai"},
		Model:          BillingFact{Name: "gpt-4"},
		RequestPath:    "/v1/chat/completions",
		StatusCode:     http.StatusOK,
		Usage: BillingUsage{
			Unit:    "token",
			Input:   1,
			Output:  2,
			Total:   3,
			Details: map[string]any{},
		},
		StartTimeMs: 1,
		EndTimeMs:   2,
		Cluster:     "cluster-a",
	}
	config := BillingConfig{
		RedisStream: RedisStream{
			Stream: defaultRedisStream,
		},
		redisClient: client,
	}

	sendBillingEvent(config, event)
	sendBillingEvent(config, event)

	require.Len(t, client.commands, 2)
	for _, command := range client.commands {
		require.Equal(t, []interface{}{"xadd", defaultRedisStream, "*", "event"}, command[:4])
		body, ok := command[4].(string)
		require.True(t, ok)
		var sent BillingEvent
		require.NoError(t, json.Unmarshal([]byte(body), &sent))
		require.Equal(t, event.IdempotencyKey, sent.IdempotencyKey)
		require.Equal(t, event.EventID, sent.EventID)
	}
}

func TestInitBillingRequestContextSetsEventID(t *testing.T) {
	ctx := &mockBillingHttpContext{values: map[string]interface{}{}}

	eventID, err := initBillingRequestContext(ctx, "/v1/chat/completions", "req-1", "tenant-a", "consumer-a", "openai", "global", "pv-7")

	require.NoError(t, err)
	require.NotEmpty(t, eventID)
	require.Equal(t, eventID, ctx.values[ctxEventID])
	require.Equal(t, eventID, ctx.values[ctxIdempotencyKey])
	require.Equal(t, "openai", ctx.values[ctxProvider])
	require.Equal(t, "global", ctx.values[ctxQuotaScope])
	require.Equal(t, "/v1/chat/completions", ctx.values[ctxRequestPath])
	require.Equal(t, "req-1", ctx.values[ctxRequestID])
	require.Equal(t, "tenant-a", ctx.values[ctxTenant])
	require.Equal(t, "consumer-a", ctx.values[ctxConsumer])
	require.NotZero(t, ctx.values[ctxStartTime])
	parsed, err := uuid.Parse(eventID)
	require.NoError(t, err)
	require.Equal(t, uuid.Version(7), parsed.Version())
}

func TestBuildBillingEventUsesOnlyRequestIdSources(t *testing.T) {
	ctx := &mockBillingHttpContext{values: map[string]interface{}{}}
	eventID, err := initBillingRequestContext(ctx, "/v1/chat/completions", "", "tenant-a", "consumer-a", "openai", "global", "pv-7")
	require.NoError(t, err)

	ctx.SetContext(ctxStatusCode, http.StatusOK)

	event := buildBillingEvent(ctx, BillingConfig{Provider: "openai"}, false)
	require.Equal(t, eventID, event.EventID)
	require.Equal(t, "", event.RequestID)
	require.Equal(t, "consumer-a", event.Consumer)
	require.NotEqual(t, event.RequestID, "tenant-a")
	require.NotEqual(t, event.RequestID, "consumer-a")
	require.NotEqual(t, event.RequestID, "200")
}

func TestBuildBillingEventIncludesConfiguredEventKind(t *testing.T) {
	ctx := &mockBillingHttpContext{values: map[string]interface{}{}}
	ctx.SetContext(ctxEventID, "018f4c7c-3333-7abc-8333-333333333333")
	ctx.SetContext(ctxIdempotencyKey, "018f4c7c-3333-7abc-8333-333333333333")
	ctx.SetContext(ctxRequestPath, "/v1/chat/completions")
	ctx.SetContext(ctxRequestID, "req-1")
	ctx.SetContext(ctxTenant, "tenant-a")
	ctx.SetContext(ctxConsumer, "consumer-a")
	ctx.SetContext(ctxProvider, "openai")
	ctx.SetContext(ctxQuotaScope, "internal:ai-memory.digest")
	ctx.SetContext(ctxRoute, "internal-ai-memory-digest")
	ctx.SetContext(ctxCluster, "llm-qwen-019ebb2c.internal.dns")
	ctx.SetContext(ctxPriceVersion, "pv-7")
	ctx.SetContext(ctxStatusCode, http.StatusOK)
	ctx.SetContext(ctxInputToken, int64(3))
	ctx.SetContext(ctxOutputToken, int64(5))
	ctx.SetContext(ctxUsageSource, usageSourceProvider)

	require.Equal(t, "usage", buildBillingEvent(ctx, BillingConfig{}, false).EventKind)
	require.Equal(t, "usage", buildBillingEvent(ctx, BillingConfig{EventKind: "customer_usage"}, false).EventKind)

	body, err := json.Marshal(buildBillingEvent(ctx, BillingConfig{EventKind: eventKindInternalCost}, false))
	require.NoError(t, err)

	var event map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &event))
	require.Equal(t, eventKindInternalCost, event["event_kind"])
}

func TestBuildBillingEventIncludesInternalCostMetadata(t *testing.T) {
	ctx := &mockBillingHttpContext{values: map[string]interface{}{}}
	ctx.SetContext(ctxEventID, "018f4c7c-3333-7abc-8333-333333333333")
	ctx.SetContext(ctxIdempotencyKey, "018f4c7c-3333-7abc-8333-333333333333")
	ctx.SetContext(ctxRequestPath, "/v1/embeddings")
	ctx.SetContext(ctxRequestID, "req-internal-embedding")
	ctx.SetContext(ctxTenant, "tenant-a")
	ctx.SetContext(ctxConsumer, "ai-memory-embedding")
	ctx.SetContext(ctxProvider, "gemini-local")
	ctx.SetContext(ctxQuotaScope, "route:ai-memory-gemini-embedding")
	ctx.SetContext(ctxRoute, "ai-route-ai-memory-gemini-embedding.internal")
	ctx.SetContext(ctxCluster, "outbound|443||llm-gemini-local.internal.dns")
	ctx.SetContext(ctxStatusCode, http.StatusOK)
	ctx.SetContext(ctxInputToken, int64(21))
	ctx.SetContext(ctxOutputToken, int64(0))
	ctx.SetContext(ctxTotalToken, int64(21))
	ctx.SetContext(ctxUsageSource, usageSourceEstimated)
	ctx.SetContext(ctxModel, "gemini-embedding-001")
	ctx.SetContext(ctxSourceJob, "ai-memory-chunk:4069d615-f6e0-8e8b-8250-73860818c96a")
	ctx.SetContext(ctxUpstreamInvoked, true)
	billCustomer := false

	event := buildBillingEvent(ctx, BillingConfig{
		EventKind:    eventKindInternalCost,
		CostSource:   "ai-memory.embedding",
		WorkerKind:   "embedding",
		BillCustomer: &billCustomer,
	}, false)

	require.Equal(t, eventKindInternalCost, event.EventKind)
	require.Equal(t, "ai-memory.embedding", event.CostSource)
	require.Equal(t, "embedding", event.WorkerKind)
	require.Equal(t, "ai-memory-chunk:4069d615-f6e0-8e8b-8250-73860818c96a", event.SourceJob)
	require.NotNil(t, event.BillCustomer)
	require.False(t, *event.BillCustomer)
	require.Empty(t, event.Tenant)
	require.Equal(t, "ai-memory-embedding", event.Consumer)
	require.True(t, event.UpstreamInvoked)

	streamEvent := buildBillingEvent(ctx, BillingConfig{
		EventKind:    eventKindInternalCost,
		CostSource:   "ai-memory.embedding",
		WorkerKind:   "embedding",
		BillCustomer: &billCustomer,
	}, true)
	require.Equal(t, "ai-memory-embedding", streamEvent.Consumer)
	require.Empty(t, streamEvent.Tenant)
	require.True(t, streamEvent.IsStream)
}

func TestBuildBillingEventDerivesMissingInputFromTotalAndOutput(t *testing.T) {
	ctx := &mockBillingHttpContext{values: map[string]interface{}{}}
	ctx.SetContext(ctxInputToken, int64(0))
	ctx.SetContext(ctxOutputToken, int64(146))
	ctx.SetContext(ctxTotalToken, int64(155))
	ctx.SetContext(ctxUsageSource, usageSourceProvider)
	ctx.SetContext(ctxStatusCode, http.StatusOK)

	event := buildBillingEvent(ctx, BillingConfig{Provider: "openai"}, false)

	require.EqualValues(t, 9, event.Usage.Input)
	require.EqualValues(t, 146, event.Usage.Output)
	require.EqualValues(t, 155, event.Usage.Total)
}

func TestBuildBillingEventUsesClusterDerivedProviderAndPreservesRawCluster(t *testing.T) {
	ctx := &mockBillingHttpContext{values: map[string]interface{}{}}
	ctx.SetContext(ctxProvider, "deepseek-019ebb2c")
	ctx.SetContext(ctxCluster, "outbound|443||llm-qwen-019ebb2c.internal.dns")
	ctx.SetContext(ctxStatusCode, http.StatusOK)

	event := buildBillingEvent(ctx, BillingConfig{Provider: "openai"}, false)

	require.Equal(t, "qwen-019ebb2c", event.Provider.Name)
	require.Equal(t, "outbound|443||llm-qwen-019ebb2c.internal.dns", event.Cluster)
}

func TestBuildBillingEventLeavesProviderEmptyWhenClusterIdentityIsUnknown(t *testing.T) {
	ctx := &mockBillingHttpContext{values: map[string]interface{}{}}
	ctx.SetContext(ctxProvider, "deepseek-019ebb2c")
	ctx.SetContext(ctxCluster, "cluster-a")
	ctx.SetContext(ctxStatusCode, http.StatusOK)

	event := buildBillingEvent(ctx, BillingConfig{Provider: "deepseek-019ebb2c"}, false)

	require.Empty(t, event.Provider.Name)
	require.Equal(t, "cluster-a", event.Cluster)
}

func TestBuildBillingEventDerivesProviderFromClusterForms(t *testing.T) {
	tests := []struct {
		name         string
		cluster      string
		wantProvider string
		wantCluster  string
	}{
		{
			name:         "direct host",
			cluster:      "llm-qwen-019ebb2c.internal.dns",
			wantProvider: "qwen-019ebb2c",
			wantCluster:  "llm-qwen-019ebb2c.internal.dns",
		},
		{
			name:         "envoy outbound cluster",
			cluster:      "outbound|443||llm-qwen-019ebb2c.internal.dns",
			wantProvider: "qwen-019ebb2c",
			wantCluster:  "outbound|443||llm-qwen-019ebb2c.internal.dns",
		},
		{
			name:         "short host",
			cluster:      "llm-qwen-019ebb2c.dns",
			wantProvider: "qwen-019ebb2c",
			wantCluster:  "llm-qwen-019ebb2c.dns",
		},
		{
			name:         "whitespace trimming",
			cluster:      "  llm-qwen-019ebb2c.internal.dns  ",
			wantProvider: "qwen-019ebb2c",
			wantCluster:  "  llm-qwen-019ebb2c.internal.dns  ",
		},
		{
			name:         "unknown cluster",
			cluster:      "cluster-a",
			wantProvider: "",
			wantCluster:  "cluster-a",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &mockBillingHttpContext{values: map[string]interface{}{}}
			ctx.SetContext(ctxProvider, "route-provider")
			ctx.SetContext(ctxCluster, tc.cluster)
			ctx.SetContext(ctxStatusCode, http.StatusOK)

			event := buildBillingEvent(ctx, BillingConfig{Provider: "openai"}, false)

			require.Equal(t, tc.wantProvider, event.Provider.Name)
			require.Equal(t, tc.wantCluster, event.Cluster)
		})
	}
}

func TestBuildBillingEventEstimatedUsageIsBasicOnly(t *testing.T) {
	ctx := &mockBillingHttpContext{values: map[string]interface{}{}}
	ctx.SetContext(ctxStartTime, int64(1))
	ctx.SetContext(ctxEventID, "018f4c7c-2222-7abc-8222-222222222222")
	ctx.SetContext(ctxIdempotencyKey, "018f4c7c-2222-7abc-8222-222222222222")
	ctx.SetContext(ctxRequestPath, "/v1/chat/completions")
	ctx.SetContext(ctxRequestID, "req-estimated")
	ctx.SetContext(ctxTenant, "tenant-a")
	ctx.SetContext(ctxConsumer, "consumer-a")
	ctx.SetContext(ctxProvider, "openai")
	ctx.SetContext(ctxQuotaScope, "global")
	ctx.SetContext(ctxRoute, "route-a")
	ctx.SetContext(ctxCluster, "cluster-a")
	ctx.SetContext(ctxStatusCode, http.StatusOK)

	require.True(t, recordEstimatedUsage(ctx, "provider-new-model", "hello input", "hello output"))
	ctx.SetContext(ctxInputCacheHit, int64(1))
	ctx.SetContext(ctxInputCacheMiss, int64(2))
	ctx.SetContext(ctxInputDetails, map[string]int64{"cached_tokens": 1})
	ctx.SetContext(ctxOutputDetails, map[string]int64{"reasoning_tokens": 2})
	ctx.SetContext(ctxProviderUsage, map[string]any{"prompt_tokens": 99})

	body, err := json.Marshal(buildBillingEvent(ctx, BillingConfig{Provider: "openai"}, false))
	require.NoError(t, err)

	var event map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &event))
	require.Equal(t, false, event["usage_missing"])
	require.Equal(t, usageSourceEstimated, event["usage_source"])
	requireObjectFactName(t, event, "model", "provider-new-model")

	usage, ok := event["usage"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "token", usage["unit"])
	input := int64(usage["input"].(float64))
	output := int64(usage["output"].(float64))
	total := int64(usage["total"].(float64))
	require.Greater(t, input, int64(0))
	require.Greater(t, output, int64(0))
	require.Equal(t, input+output, total)
	require.ElementsMatch(t, []string{"unit", "input", "output", "total"}, mapKeys(usage))
}

func TestNonStreamingEstimatedUsageUsesUnknownModelFallback(t *testing.T) {
	host, status := test.NewTestHost(billingConfig)
	defer host.Reset()
	require.Equal(t, types.OnPluginStartStatusOK, status)

	inputText := "estimate this prompt with the fallback vocabulary"
	outputText := "estimated answer text"

	host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"},
		{":path", "/v1/chat/completions"},
		{":method", "POST"},
		{"x-tenant-id", "tenant-a"},
		{"x-consumer-id", "consumer-a"},
	})
	action := host.CallOnHttpRequestBody([]byte(`{"messages":[{"role":"user","content":"` + inputText + `"}]}`))
	require.Equal(t, types.ActionContinue, action)
	host.CallOnHttpResponseHeaders([][2]string{
		{":status", "200"},
		{"content-type", "application/json"},
	})
	action = host.CallOnHttpResponseBody([]byte(`{"choices":[{"message":{"content":"` + outputText + `"}}]}`))
	require.Equal(t, types.ActionContinue, action)

	body := requireRedisBillingEventBody(t, host)

	expected, ok := estimateTextTokenUsage(tokenusage.ModelUnknown, inputText, outputText)
	require.True(t, ok)

	var event map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &event))
	require.Equal(t, false, event["usage_missing"])
	require.Equal(t, usageSourceEstimated, event["usage_source"])
	requireObjectFactName(t, event, "model", tokenusage.ModelUnknown)

	usage, ok := event["usage"].(map[string]interface{})
	require.True(t, ok)
	require.EqualValues(t, expected.InputToken, usage["input"])
	require.EqualValues(t, expected.OutputToken, usage["output"])
	require.EqualValues(t, expected.TotalToken, usage["total"])
	require.ElementsMatch(t, []string{"unit", "input", "output", "total"}, mapKeys(usage))
	require.NotContains(t, string(body), "provider_usage")
	require.NotContains(t, string(body), "input_cache_hit_tokens")
	require.NotContains(t, string(body), "input_cache_miss_tokens")
	require.NotContains(t, string(body), "output_tokens")
}

func TestBuildBillingEventMissingUsageFallbackIsZero(t *testing.T) {
	ctx := &mockBillingHttpContext{values: map[string]interface{}{}}
	ctx.SetContext(ctxStartTime, int64(1))
	ctx.SetContext(ctxEventID, "018f4c7c-3333-7abc-8333-333333333333")
	ctx.SetContext(ctxIdempotencyKey, "018f4c7c-3333-7abc-8333-333333333333")
	ctx.SetContext(ctxRequestPath, "/v1/chat/completions")
	ctx.SetContext(ctxRequestID, "req-missing")
	ctx.SetContext(ctxConsumer, "consumer-a")
	ctx.SetContext(ctxProvider, "openai")
	ctx.SetContext(ctxQuotaScope, "global")
	ctx.SetContext(ctxRoute, "route-a")
	ctx.SetContext(ctxCluster, "cluster-a")
	ctx.SetContext(ctxStatusCode, http.StatusOK)

	ctx.SetContext(ctxInputToken, int64(12))
	ctx.SetContext(ctxOutputToken, int64(8))
	ctx.SetContext(ctxInputCacheHit, int64(1))
	ctx.SetContext(ctxInputCacheMiss, int64(2))
	ctx.SetContext(ctxInputDetails, map[string]int64{"cached_tokens": 1})
	ctx.SetContext(ctxProviderUsage, map[string]any{"prompt_tokens": 99})

	body, err := json.Marshal(buildBillingEvent(ctx, BillingConfig{Provider: "openai"}, false))
	require.NoError(t, err)

	var event map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &event))
	require.Equal(t, true, event["usage_missing"])
	require.Equal(t, usageSourceMissing, event["usage_source"])

	usage, ok := event["usage"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "token", usage["unit"])
	require.EqualValues(t, 0, usage["input"])
	require.EqualValues(t, 0, usage["output"])
	require.EqualValues(t, 0, usage["total"])
	require.Equal(t, map[string]interface{}{}, usage["details"])
	require.NotContains(t, usage, "input_cache_hit_tokens")
	require.NotContains(t, usage, "input_cache_miss_tokens")
	require.NotContains(t, usage, "output_tokens")
}

type recordingBillingRedisClient struct {
	wrapper.RedisClient
	err      error
	commands [][]interface{}
}

func (r *recordingBillingRedisClient) Command(cmds []interface{}, callback wrapper.RedisResponseCallback) error {
	r.commands = append(r.commands, append([]interface{}(nil), cmds...))
	return r.err
}

func TestOnHttpStreamingResponseBodyAccumulatesSentAssistantDeltas(t *testing.T) {
	ctx := &mockBillingHttpContext{values: map[string]interface{}{
		ctxBillingEnabled: true,
	}}

	firstChunk := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hel\"}}]}\n\n")
	returned := onHttpStreamingResponseBody(ctx, BillingConfig{}, firstChunk, false)
	require.Equal(t, firstChunk, returned)

	secondChunk := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n")
	returned = onHttpStreamingResponseBody(ctx, BillingConfig{}, secondChunk, false)
	require.Equal(t, secondChunk, returned)

	require.True(t, ctx.GetBoolContext(ctxIsStream, false))
	require.Equal(t, "Hello", ctx.GetStringContext(ctxStreamOutputText, ""))
	require.Empty(t, ctx.GetStringContext(ctxUsageSource, ""))
}

func TestOnHttpStreamingResponseBodyEndOfStreamDeliversAndMarksDelivered(t *testing.T) {
	client := &recordingBillingRedisClient{}
	ctx := &mockBillingHttpContext{values: map[string]interface{}{
		ctxBillingEnabled: true,
		ctxStartTime:      int64(1),
		ctxEventID:        "018f4c7c-3333-7abc-8333-333333333333",
		ctxIdempotencyKey: "018f4c7c-3333-7abc-8333-333333333333",
		ctxRequestPath:    "/v1/chat/completions",
		ctxRequestID:      "req-stream-end",
		ctxTenant:         "tenant-a",
		ctxConsumer:       "consumer-a",
		ctxProvider:       "openai",
		ctxQuotaScope:     "global",
		ctxRoute:          "route-a",
		ctxCluster:        "cluster-a",
		ctxStatusCode:     http.StatusOK,
	}}

	chunk := []byte("data: {\"model\":\"gpt-4\",\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2,\"total_tokens\":3}}\n\n")
	returned := onHttpStreamingResponseBody(ctx, BillingConfig{
		Provider: "openai",
		RedisStream: RedisStream{
			Stream: defaultRedisStream,
		},
		redisClient: client,
	}, chunk, true)

	require.Equal(t, chunk, returned)
	require.True(t, ctx.GetBoolContext(ctxBillingDelivered, false))
	require.Len(t, client.commands, 1)

	var event BillingEvent
	body, ok := client.commands[0][4].(string)
	require.True(t, ok)
	require.NoError(t, json.Unmarshal([]byte(body), &event))
	require.True(t, event.IsStream)
	require.False(t, event.UsageMissing)
	require.Equal(t, usageSourceProvider, event.UsageSource)
	require.EqualValues(t, 1, event.Usage.Input)
	require.EqualValues(t, 2, event.Usage.Output)
	require.EqualValues(t, 3, event.Usage.Total)
}

func TestStreamingNormalEndWithoutProviderUsageIsEstimated(t *testing.T) {
	host, status := test.NewTestHost(billingConfig)
	defer host.Reset()
	require.Equal(t, types.OnPluginStartStatusOK, status)

	inputText := "streaming prompt text"
	outputText := "streaming answer text"

	host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"},
		{":path", "/v1/chat/completions"},
		{":method", "POST"},
		{"x-tenant-id", "tenant-a"},
		{"x-consumer-id", "consumer-a"},
	})
	action := host.CallOnHttpRequestBody([]byte(`{"messages":[{"role":"user","content":"` + inputText + `"}]}`))
	require.Equal(t, types.ActionContinue, action)
	host.CallOnHttpResponseHeaders([][2]string{
		{":status", "200"},
		{"content-type", "text/event-stream"},
	})
	action = host.CallOnHttpStreamingResponseBody([]byte("data: {\"choices\":[{\"delta\":{\"content\":\""+outputText+"\"}}]}\n\n"), true)
	require.Equal(t, types.ActionContinue, action)

	expected, ok := estimateTextTokenUsage("", inputText, outputText)
	require.True(t, ok)

	event := requireRedisBillingEventStruct(t, host)
	require.True(t, event.IsStream)
	require.False(t, event.UsageMissing)
	require.Equal(t, usageSourceEstimated, event.UsageSource)
	require.Equal(t, expected.InputToken, event.Usage.Input)
	require.Equal(t, expected.OutputToken, event.Usage.Output)
	require.Equal(t, expected.TotalToken, event.Usage.Total)
	require.Nil(t, event.Usage.Details)

	host.CompleteHttp()
	require.Len(t, host.GetRedisCalloutAttributes(), 1)
}

func TestStreamDoneFallbackDeliversEstimatedUsage(t *testing.T) {
	host, status := test.NewTestHost(billingConfig)
	defer host.Reset()
	require.Equal(t, types.OnPluginStartStatusOK, status)

	action := host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"},
		{":path", "/v1/chat/completions"},
		{":method", "POST"},
		{"x-tenant-id", "tenant-a"},
		{"x-consumer-id", "consumer-a"},
	})
	require.Equal(t, types.ActionContinue, action)

	action = host.CallOnHttpRequestBody([]byte(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello input"}]}`))
	require.Equal(t, types.ActionContinue, action)

	action = host.CallOnHttpResponseHeaders([][2]string{
		{":status", "200"},
		{"content-type", "text/event-stream"},
	})
	require.Equal(t, types.ActionContinue, action)

	action = host.CallOnHttpStreamingResponseBody([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n"), false)
	require.Equal(t, types.ActionContinue, action)

	host.CompleteHttp()
	event := requireRedisBillingEventStruct(t, host)
	require.True(t, event.IsStream)
	require.False(t, event.UsageMissing)
	require.Equal(t, usageSourceEstimated, event.UsageSource)
	require.Greater(t, event.Usage.Input, int64(0))
	require.Greater(t, event.Usage.Output, int64(0))
	require.Equal(t, event.Usage.Input+event.Usage.Output, event.Usage.Total)
	require.Nil(t, event.Usage.Details)
}

func TestStreamDoneFallbackEstimatesInputWhenInterruptedBeforeOutput(t *testing.T) {
	host, status := test.NewTestHost(billingConfig)
	defer host.Reset()
	require.Equal(t, types.OnPluginStartStatusOK, status)

	inputText := "hello input"

	action := host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"},
		{":path", "/v1/chat/completions"},
		{":method", "POST"},
		{"x-tenant-id", "tenant-a"},
		{"x-consumer-id", "consumer-a"},
	})
	require.Equal(t, types.ActionContinue, action)

	action = host.CallOnHttpRequestBody([]byte(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"` + inputText + `"}]}`))
	require.Equal(t, types.ActionContinue, action)

	action = host.CallOnHttpResponseHeaders([][2]string{
		{":status", "200"},
		{"content-type", "text/event-stream"},
	})
	require.Equal(t, types.ActionContinue, action)

	host.CompleteHttp()
	event := requireRedisBillingEventStruct(t, host)
	require.True(t, event.IsStream)
	require.False(t, event.UsageMissing)
	require.Equal(t, usageSourceEstimated, event.UsageSource)
	require.Greater(t, event.Usage.Input, int64(0))
	require.EqualValues(t, 0, event.Usage.Output)
	require.Equal(t, event.Usage.Input, event.Usage.Total)
	require.Nil(t, event.Usage.Details)
}

func TestStreamDoneFallbackEstimatesOnlySentDeltas(t *testing.T) {
	host, status := test.NewTestHost(billingConfig)
	defer host.Reset()
	require.Equal(t, types.OnPluginStartStatusOK, status)

	requestText := "hello input"
	sentText := "Hello"
	unsentText := " unreachable generated continuation with many distinct words and numbers 12345"

	action := host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"},
		{":path", "/v1/chat/completions"},
		{":method", "POST"},
		{"x-tenant-id", "tenant-a"},
		{"x-consumer-id", "consumer-a"},
	})
	require.Equal(t, types.ActionContinue, action)

	action = host.CallOnHttpRequestBody([]byte(`{"messages":[{"role":"user","content":"` + requestText + `"}]}`))
	require.Equal(t, types.ActionContinue, action)

	action = host.CallOnHttpResponseHeaders([][2]string{
		{":status", "200"},
		{"content-type", "text/event-stream"},
	})
	require.Equal(t, types.ActionContinue, action)

	action = host.CallOnHttpStreamingResponseBody([]byte("data: {\"choices\":[{\"delta\":{\"content\":\""+sentText+"\"}}]}\n\n"), false)
	require.Equal(t, types.ActionContinue, action)

	host.CompleteHttp()

	sentUsage, ok := estimateTextTokenUsage("", requestText, sentText)
	require.True(t, ok)
	unsentUsage, ok := estimateTextTokenUsage("", requestText, sentText+unsentText)
	require.True(t, ok)
	require.Greater(t, unsentUsage.OutputToken, sentUsage.OutputToken)

	event := requireRedisBillingEventStruct(t, host)
	require.Equal(t, usageSourceEstimated, event.UsageSource)
	require.Equal(t, sentUsage.InputToken, event.Usage.Input)
	require.Equal(t, sentUsage.OutputToken, event.Usage.Output)
	require.Equal(t, sentUsage.TotalToken, event.Usage.Total)
	require.NotEqual(t, unsentUsage.OutputToken, event.Usage.Output)
}

func TestStreamDoneFallbackDeliversProviderUsage(t *testing.T) {
	host, status := test.NewTestHost(billingConfig)
	defer host.Reset()
	require.Equal(t, types.OnPluginStartStatusOK, status)

	host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"},
		{":path", "/v1/chat/completions"},
		{":method", "POST"},
		{"x-tenant-id", "tenant-a"},
		{"x-consumer-id", "consumer-a"},
	})
	host.CallOnHttpResponseHeaders([][2]string{
		{":status", "200"},
		{"content-type", "text/event-stream"},
	})
	action := host.CallOnHttpStreamingResponseBody([]byte("data: {\"model\":\"gpt-4\",\"usage\":{\"prompt_tokens\":4,\"completion_tokens\":5,\"total_tokens\":9}}\n\n"), false)
	require.Equal(t, types.ActionContinue, action)

	host.CompleteHttp()
	event := requireRedisBillingEventStruct(t, host)
	require.True(t, event.IsStream)
	require.False(t, event.UsageMissing)
	require.Equal(t, usageSourceProvider, event.UsageSource)
	require.EqualValues(t, 4, event.Usage.Input)
	require.EqualValues(t, 5, event.Usage.Output)
	require.EqualValues(t, 9, event.Usage.Total)
	require.Contains(t, event.Usage.Details, "provider_usage")
}

func TestStreamDoneFallbackDeliversMissingUsage(t *testing.T) {
	host, status := test.NewTestHost(billingConfig)
	defer host.Reset()
	require.Equal(t, types.OnPluginStartStatusOK, status)

	host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"},
		{":path", "/v1/chat/completions"},
		{":method", "POST"},
		{"x-tenant-id", "tenant-a"},
		{"x-consumer-id", "consumer-a"},
	})
	host.CallOnHttpResponseHeaders([][2]string{
		{":status", "200"},
		{"content-type", "text/event-stream"},
	})
	action := host.CallOnHttpStreamingResponseBody([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"function\":{\"arguments\":\"{\\\"city\\\":\\\"Paris\\\"}\"}}]}}]}\n\n"), false)
	require.Equal(t, types.ActionContinue, action)

	host.CompleteHttp()
	event := requireRedisBillingEventStruct(t, host)
	require.True(t, event.IsStream)
	require.True(t, event.UsageMissing)
	require.Equal(t, usageSourceMissing, event.UsageSource)
	require.EqualValues(t, 0, event.Usage.Input)
	require.EqualValues(t, 0, event.Usage.Output)
	require.EqualValues(t, 0, event.Usage.Total)
	require.Empty(t, event.Usage.Details)
}

func TestStreamDoneFallbackSkipsAfterNormalDelivery(t *testing.T) {
	host, status := test.NewTestHost(billingConfig)
	defer host.Reset()
	require.Equal(t, types.OnPluginStartStatusOK, status)

	host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"},
		{":path", "/v1/chat/completions"},
		{":method", "POST"},
		{"x-tenant-id", "tenant-a"},
		{"x-consumer-id", "consumer-a"},
	})
	host.CallOnHttpResponseHeaders([][2]string{
		{":status", "200"},
		{"content-type", "text/event-stream"},
	})
	action := host.CallOnHttpStreamingResponseBody([]byte("data: {\"model\":\"gpt-4\",\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2,\"total_tokens\":3}}\n\n"), true)
	require.Equal(t, types.ActionContinue, action)
	require.Len(t, host.GetRedisCalloutAttributes(), 1)

	host.CompleteHttp()

	require.Len(t, host.GetRedisCalloutAttributes(), 1)
}

type mockBillingHttpContext struct {
	values map[string]interface{}
}

func (m *mockBillingHttpContext) Scheme() string { return "" }
func (m *mockBillingHttpContext) Host() string   { return "" }
func (m *mockBillingHttpContext) Path() string   { return "" }
func (m *mockBillingHttpContext) Method() string { return "" }
func (m *mockBillingHttpContext) SetContext(key string, value interface{}) {
	m.values[key] = value
}
func (m *mockBillingHttpContext) GetContext(key string) interface{} { return m.values[key] }
func (m *mockBillingHttpContext) GetBoolContext(key string, defaultValue bool) bool {
	if v, ok := m.values[key].(bool); ok {
		return v
	}
	return defaultValue
}
func (m *mockBillingHttpContext) GetStringContext(key, defaultValue string) string {
	if v, ok := m.values[key].(string); ok {
		return v
	}
	return defaultValue
}
func (m *mockBillingHttpContext) GetByteSliceContext(key string, defaultValue []byte) []byte {
	if v, ok := m.values[key].([]byte); ok {
		return v
	}
	return defaultValue
}
func (m *mockBillingHttpContext) GetUserAttribute(key string) interface{} { return nil }
func (m *mockBillingHttpContext) SetUserAttribute(key string, value interface{}) {
}
func (m *mockBillingHttpContext) SetUserAttributeMap(kvmap map[string]interface{}) {}
func (m *mockBillingHttpContext) GetUserAttributeMap() map[string]interface{}      { return nil }
func (m *mockBillingHttpContext) WriteUserAttributeToLog() error                   { return nil }
func (m *mockBillingHttpContext) WriteUserAttributeToLogWithKey(key string) error  { return nil }
func (m *mockBillingHttpContext) WriteUserAttributeToTrace() error                 { return nil }
func (m *mockBillingHttpContext) DontReadRequestBody()                             {}
func (m *mockBillingHttpContext) DontReadResponseBody()                            {}
func (m *mockBillingHttpContext) BufferRequestBody()                               {}
func (m *mockBillingHttpContext) BufferResponseBody()                              {}
func (m *mockBillingHttpContext) NeedPauseStreamingResponse()                      {}
func (m *mockBillingHttpContext) PushBuffer(buffer []byte)                         {}
func (m *mockBillingHttpContext) PopBuffer() []byte                                { return nil }
func (m *mockBillingHttpContext) BufferQueueSize() int                             { return 0 }
func (m *mockBillingHttpContext) DisableReroute()                                  {}
func (m *mockBillingHttpContext) SetRequestBodyBufferLimit(byteSize uint32)        {}
func (m *mockBillingHttpContext) SetResponseBodyBufferLimit(byteSize uint32)       {}
func (m *mockBillingHttpContext) RouteCall(method, url string, headers [][2]string, body []byte, callback iface.RouteResponseCallback) error {
	return nil
}
func (m *mockBillingHttpContext) GetExecutionPhase() iface.HTTPExecutionPhase {
	return iface.DecodeHeader
}
func (m *mockBillingHttpContext) HasRequestBody() bool       { return false }
func (m *mockBillingHttpContext) HasResponseBody() bool      { return false }
func (m *mockBillingHttpContext) IsWebsocket() bool          { return false }
func (m *mockBillingHttpContext) IsBinaryRequestBody() bool  { return false }
func (m *mockBillingHttpContext) IsBinaryResponseBody() bool { return false }
