package main

import (
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestScopedOnlyConfigStartsDormant(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		emptyHost, emptyStatus := test.NewTestHost([]byte(`{}`))

		require.Equal(t, types.OnPluginStartStatusOK, emptyStatus)
		emptyConfig, err := emptyHost.GetMatchConfig()
		require.NoError(t, err)
		dormant := emptyConfig.(*BillingConfig)
		require.True(t, dormant.dormant)
		require.Empty(t, dormant.RedisStream.ServiceName)
		action := emptyHost.CallOnHttpRequestHeaders([][2]string{
			{":authority", "admin.example.test"},
			{":path", "/api/v1/plugin-instances"},
			{":method", "GET"},
		})
		require.Equal(t, types.ActionContinue, action)
		require.Empty(t, emptyHost.GetRedisCalloutAttributes())
		emptyHost.Reset()

		host, status := test.NewTestHost(mustBillingConfig(t, map[string]interface{}{
			"_rules_": []map[string]interface{}{
				{
					"_match_route_": []string{"route-scoped-billing"},
					"redis_stream": map[string]interface{}{
						"service_name": "redis.static",
						"service_port": 6379,
						"stream":       "billing:events",
					},
					"quota_scope": "route:qwen",
					"provider":    "dashscope",
					"path_scope":  "bound_target",
				},
			},
		}))
		defer host.Reset()

		require.Equal(t, types.OnPluginStartStatusOK, status)

		config, err := host.GetMatchConfig()
		require.NoError(t, err)
		require.Nil(t, config)

		require.NoError(t, host.SetRouteName("route-scoped-billing"))
		config, err = host.GetMatchConfig()
		require.NoError(t, err)
		scoped := config.(*BillingConfig)
		require.False(t, scoped.dormant)
		require.Equal(t, "redis.static", scoped.RedisStream.ServiceName)
		require.Equal(t, "route:qwen", scoped.QuotaScope)
		require.Equal(t, "dashscope", scoped.Provider)
		require.Equal(t, PathScopeBoundTarget, scoped.PathScope)
	})
}

func TestScopedOnlyEmptyResponseDeliversOnStreamDone(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(mustBillingConfig(t, map[string]interface{}{
			"_rules_": []map[string]interface{}{
				{
					"_match_route_":   []string{"ai-route-qwen-chat.internal"},
					"_match_service_": []string{"llm-qwen-local.internal.dns"},
					"redis_stream": map[string]interface{}{
						"service_name": "redis.static",
						"service_port": 6379,
						"stream":       "billing:events",
					},
					"quota_scope": "route:qwen-chat",
					"path_scope":  "bound_target",
				},
			},
		}))
		defer host.Reset()

		require.Equal(t, types.OnPluginStartStatusOK, status)
		require.NoError(t, host.SetRouteName("ai-route-qwen-chat.internal"))
		require.NoError(t, host.SetClusterName("outbound|443||llm-qwen-local.internal.dns"))

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "api.ai.local"},
			{":path", "/v1/qwen-e2e"},
			{":method", "POST"},
			{"x-request-id", "scoped-empty-response"},
		})
		require.Equal(t, types.ActionContinue, action)
		action = host.CallOnHttpResponseHeaders([][2]string{
			{":status", "404"},
			{"content-length", "0"},
		})
		require.Equal(t, types.ActionContinue, action)

		host.CompleteHttp()
		event := requireRedisBillingEvent(t, host)
		require.Equal(t, "scoped-empty-response", event["request_id"])
		require.Equal(t, "/v1/qwen-e2e", event["request_path"])
		require.Equal(t, float64(404), event["status_code"])
		require.Equal(t, "route:qwen-chat", event["quota_scope"])
		require.Equal(t, true, event["usage_missing"])
		require.Len(t, host.GetRedisCalloutAttributes(), 1)
	})
}
