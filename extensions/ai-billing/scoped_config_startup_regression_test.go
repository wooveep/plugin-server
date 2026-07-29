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
