package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

var monetaryConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"quota_scope":            "global",
		"provider":               "openai",
		"tenant_header":          "x-tenant-id",
		"consumer_header":        "x-consumer-id",
		"balance_key_template":   "billing:balance:{tenant}:{quota_scope}:{consumer}",
		"missing_balance_policy": "deny",
		"enable_path_suffixes": []string{
			"/v1/chat/completions",
			"/v1/messages",
		},
		"redis": map[string]interface{}{
			"service_name": "redis.static",
			"service_port": 6379,
			"timeout":      1000,
			"database":     0,
		},
	})
	return data
}()

func quotaConfigWith(overrides map[string]interface{}) json.RawMessage {
	var cfg map[string]interface{}
	_ = json.Unmarshal(monetaryConfig, &cfg)
	for k, v := range overrides {
		cfg[k] = v
	}
	data, _ := json.Marshal(cfg)
	return data
}

func TestParseConfig(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		t.Run("default admission config", func(t *testing.T) {
			data, _ := json.Marshal(map[string]interface{}{
				"redis": map[string]interface{}{
					"service_name": "redis.static",
				},
			})
			host, status := test.NewTestHost(data)
			defer host.Reset()

			require.Equal(t, types.OnPluginStartStatusOK, status)
			config, err := host.GetMatchConfig()
			require.NoError(t, err)

			quotaConfig := config.(*QuotaConfig)
			require.Equal(t, "global", quotaConfig.QuotaScope)
			require.Equal(t, "default", quotaConfig.Provider)
			require.Equal(t, "x-mse-tenant", quotaConfig.TenantHeader)
			require.Equal(t, "x-mse-consumer", quotaConfig.ConsumerHeader)
			require.Equal(t, "billing:balance:{tenant}:{quota_scope}:{consumer}", quotaConfig.BalanceKeyTemplate)
			require.Equal(t, MissingPolicyDeny, quotaConfig.MissingBalancePolicy)
			require.Equal(t, []string{"/v1/chat/completions", "/v1/messages"}, quotaConfig.EnablePathSuffixes)
		})

		t.Run("admin config no longer required", func(t *testing.T) {
			host, status := test.NewTestHost(quotaConfigWith(map[string]interface{}{
				"admin_consumer": nil,
				"admin_path":     nil,
			}))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
		})
	})
}

func TestRemovedSettlementConfigIsRejected(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		removedFields := map[string]interface{}{
			"price_key_template":   "billing:effective_price:{tenant}:{provider}:{model}:{token_type}",
			"amount_scale":         1000000,
			"price_unit_tokens":    1000000,
			"missing_price_policy": "skip",
			"missing_usage_policy": "skip",
		}

		for field, value := range removedFields {
			t.Run("global rejects "+field, func(t *testing.T) {
				host, status := test.NewTestHost(quotaConfigWith(map[string]interface{}{
					field: value,
				}))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusFailed, status)
			})

			t.Run("rule rejects "+field, func(t *testing.T) {
				host, status := test.NewTestHost(quotaConfigWith(map[string]interface{}{
					"_rules_": []map[string]interface{}{
						{
							"_match_route_": []string{"route-removed-settlement"},
							field:           value,
						},
					},
				}))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusFailed, status)
			})
		}
	})
}

func TestRequestAdmission(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("positive balance allows request", func(t *testing.T) {
			host, status := test.NewTestHost(monetaryConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			require.Equal(t, types.HeaderStopAllIterationAndWatermark, action)
			require.Contains(t, string(host.GetRedisCalloutAttributes()[0].Query), "billing:balance:tenant-a:global:consumer-a")

			host.CallOnRedisCall(0, test.CreateRedisRespString("100"))
			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			host.CompleteHttp()
		})

		t.Run("zero balance denies request", func(t *testing.T) {
			host, status := test.NewTestHost(monetaryConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			require.Equal(t, types.HeaderStopAllIterationAndWatermark, action)

			host.CallOnRedisCall(0, test.CreateRedisRespInt(0))
			response := host.GetLocalResponse()
			require.NotNil(t, response)
			require.Equal(t, uint32(http.StatusForbidden), response.StatusCode)
			require.Equal(t, "ai-quota.no_balance", response.StatusCodeDetail)
			require.Contains(t, string(response.Data), "No monetary balance left")
			require.Equal(t, types.HeaderStopAllIterationAndWatermark, host.GetHttpStreamAction())
			require.NotEqual(t, types.ActionContinue, host.GetHttpStreamAction())
			host.CompleteHttp()
		})

		t.Run("negative balance denies request", func(t *testing.T) {
			host, status := test.NewTestHost(monetaryConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			require.Equal(t, types.HeaderStopAllIterationAndWatermark, action)

			host.CallOnRedisCall(0, test.CreateRedisRespInt(-1))
			response := host.GetLocalResponse()
			require.Equal(t, uint32(http.StatusForbidden), response.StatusCode)
			require.Equal(t, "ai-quota.no_balance", response.StatusCodeDetail)
			require.Contains(t, string(response.Data), "No monetary balance left")
			require.Equal(t, types.HeaderStopAllIterationAndWatermark, host.GetHttpStreamAction())
			require.NotEqual(t, types.ActionContinue, host.GetHttpStreamAction())
			host.CompleteHttp()
		})

		t.Run("malformed balance returns redis error", func(t *testing.T) {
			host, status := test.NewTestHost(monetaryConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			require.Equal(t, types.HeaderStopAllIterationAndWatermark, action)

			host.CallOnRedisCall(0, test.CreateRedisRespString("not-a-number"))
			response := host.GetLocalResponse()
			require.Equal(t, uint32(http.StatusServiceUnavailable), response.StatusCode)
			require.Contains(t, string(response.Data), "Invalid Redis balance")
			host.CompleteHttp()
		})

		t.Run("missing balance follows allow policy", func(t *testing.T) {
			host, status := test.NewTestHost(quotaConfigWith(map[string]interface{}{
				"missing_balance_policy": "allow",
			}))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			require.Equal(t, types.HeaderStopAllIterationAndWatermark, action)

			host.CallOnRedisCall(0, test.CreateRedisRespNull())
			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			host.CompleteHttp()
		})

		t.Run("missing identity denies request", func(t *testing.T) {
			host, status := test.NewTestHost(monetaryConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
			})
			require.Equal(t, types.ActionContinue, action)
			response := host.GetLocalResponse()
			require.Equal(t, uint32(http.StatusForbidden), response.StatusCode)
			require.Contains(t, string(response.Data), "Missing tenant or consumer identity")
		})

		t.Run("legacy admin paths are not handled", func(t *testing.T) {
			host, status := test.NewTestHost(monetaryConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions/quota?consumer=consumer-a"},
				{":method", "GET"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			require.Equal(t, types.ActionContinue, action)
			require.Empty(t, host.GetRedisCalloutAttributes())
		})
	})
}

func TestAdmissionOnlyDoesNotDeduct(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("response usage does not dispatch redis mutation", func(t *testing.T) {
			host, status := test.NewTestHost(monetaryConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			host.CallOnRedisCall(0, test.CreateRedisRespInt(1000))

			responseBody := []byte(`{"model":"gpt-4","usage":{"prompt_tokens":1001,"completion_tokens":2000,"total_tokens":3001}}`)
			action := host.CallOnHttpStreamingResponseBody(responseBody, true)
			require.Equal(t, types.ActionContinue, action)

			attrs := host.GetRedisCalloutAttributes()
			require.Empty(t, attrs)
			assertNoRedisMutationQueries(t, attrs)
			host.CompleteHttp()
		})

		t.Run("missing usage has no response redis callout", func(t *testing.T) {
			host, status := test.NewTestHost(monetaryConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-tenant-id", "tenant-a"},
				{"x-consumer-id", "consumer-a"},
			})
			host.CallOnRedisCall(0, test.CreateRedisRespInt(1000))

			action := host.CallOnHttpStreamingResponseBody([]byte(`{"choices":[{"message":{"content":"ok"}}]}`), true)
			require.Equal(t, types.ActionContinue, action)
			require.Empty(t, host.GetRedisCalloutAttributes())
			host.CompleteHttp()
		})
	})
}

func TestKeyBuilders(t *testing.T) {
	config := QuotaConfig{
		BalanceKeyTemplate: "billing:balance:{tenant}:{quota_scope}:{consumer}",
		QuotaScope:         "team-a",
	}

	require.Equal(t, "billing:balance:t1:team-a:c1", config.buildBalanceKey("t1", "c1"))
	require.False(t, isAIPathEnabled("/v1/chat/completions/quota", []string{"/v1/chat/completions"}))
	require.True(t, isAIPathEnabled("/proxy/v1/messages?debug=true", []string{"/v1/messages"}))
}

func assertNoRedisMutationQueries(t *testing.T, attrs []proxytest.RedisCalloutAttribute) {
	t.Helper()
	for _, attr := range attrs {
		query := strings.ToLower(string(attr.Query))
		for _, forbidden := range []string{"eval", "decrby", "incrby", " set ", " del "} {
			require.NotContains(t, query, forbidden)
		}
	}
}
