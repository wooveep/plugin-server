package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestBuildScopedCacheKeyMaterial(t *testing.T) {
	input := ScopedCacheKeyInput{
		KeyPrefix:          "cache:materialized:",
		Tenant:             "tenant-a",
		Consumer:           "consumer-a",
		CacheScope:         config.CACHE_SCOPE_CONSUMER,
		Route:              "route-a",
		Model:              "qwen-turbo",
		Protocol:           protocol.ProtocolChatCompletions,
		RequestDigest:      "digest-a",
		CachePolicyVersion: "policy-v1",
	}

	material, err := BuildScopedCacheKeyMaterial(input)
	require.NoError(t, err)
	require.Equal(t, "digest-a", material.RequestDigest)
	require.Equal(t, "qwen-turbo", material.Model)
	require.Equal(t, "route-a", material.Route)
	require.Equal(t, protocol.ProtocolChatCompletions, material.Protocol)
	require.Equal(t, "policy-v1", material.CachePolicyVersion)
	require.True(t, strings.HasPrefix(material.RedisKey, "cache:materialized:"))
	for _, raw := range []string{"tenant-a", "consumer-a", "route-a", "qwen-turbo", "digest-a", "policy-v1", "weather?"} {
		require.NotContains(t, material.RedisKey, raw)
	}

	changedConsumer := input
	changedConsumer.Consumer = "consumer-b"
	consumerMaterial, err := BuildScopedCacheKeyMaterial(changedConsumer)
	require.NoError(t, err)
	require.NotEqual(t, material.RedisKey, consumerMaterial.RedisKey)

	tenantScopedA := input
	tenantScopedA.CacheScope = config.CACHE_SCOPE_TENANT
	tenantScopedB := tenantScopedA
	tenantScopedB.Consumer = "consumer-b"
	tenantMaterialA, err := BuildScopedCacheKeyMaterial(tenantScopedA)
	require.NoError(t, err)
	tenantMaterialB, err := BuildScopedCacheKeyMaterial(tenantScopedB)
	require.NoError(t, err)
	require.Equal(t, tenantMaterialA.RedisKey, tenantMaterialB.RedisKey)

	for _, mutate := range []func(*ScopedCacheKeyInput){
		func(in *ScopedCacheKeyInput) { in.Tenant = "tenant-b" },
		func(in *ScopedCacheKeyInput) { in.Route = "route-b" },
		func(in *ScopedCacheKeyInput) { in.Model = "qwen-plus" },
		func(in *ScopedCacheKeyInput) { in.Protocol = protocol.ProtocolResponses },
		func(in *ScopedCacheKeyInput) { in.RequestDigest = "digest-b" },
		func(in *ScopedCacheKeyInput) { in.CachePolicyVersion = "policy-v2" },
	} {
		changed := input
		mutate(&changed)
		changedMaterial, err := BuildScopedCacheKeyMaterial(changed)
		require.NoError(t, err)
		require.NotEqual(t, material.RedisKey, changedMaterial.RedisKey)
	}
}

func TestBuildOpenAIRequestDigest(t *testing.T) {
	bodyA := []byte(`{
		"model": "qwen-turbo",
		"messages": [{"role": "user", "content": "weather?"}],
		"temperature": 0.2,
		"stream": false
	}`)
	bodyB := []byte(`{"stream":true,"temperature":0.2,"messages":[{"content":"weather?","role":"user"}],"model":"qwen-turbo"}`)

	modelA, digestA, err := BuildOpenAIRequestDigest(bodyA)
	require.NoError(t, err)
	modelB, digestB, err := BuildOpenAIRequestDigest(bodyB)
	require.NoError(t, err)
	require.Equal(t, "qwen-turbo", modelA)
	require.Equal(t, modelA, modelB)
	require.Equal(t, digestA, digestB, "field order and stream flag should not change the request digest")
	require.NotContains(t, digestA, "weather")

	_, changedPromptDigest, err := BuildOpenAIRequestDigest([]byte(`{
		"model": "qwen-turbo",
		"messages": [{"role": "user", "content": "tomorrow weather?"}],
		"temperature": 0.2
	}`))
	require.NoError(t, err)
	require.NotEqual(t, digestA, changedPromptDigest)

	_, changedTemperatureDigest, err := BuildOpenAIRequestDigest([]byte(`{
		"model": "qwen-turbo",
		"messages": [{"role": "user", "content": "weather?"}],
		"temperature": 0.7
	}`))
	require.NoError(t, err)
	require.NotEqual(t, digestA, changedTemperatureDigest)
}

func TestThinMaterializedLookupUsesDedicatedRedisConfig(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(thinMaterializedLookupOnlyConfig(t))
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)
		require.NoError(t, host.SetRouteName("test-route-default"))

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "example.com"},
			{":path", "/v1/chat/completions"},
			{":method", "POST"},
			{"content-type", "application/json"},
			{"x-mse-tenant", "tenant-a"},
			{"x-mse-consumer", "consumer-a"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		action = host.CallOnHttpRequestBody([]byte(`{
			"model": "qwen-turbo",
			"messages": [{"role": "user", "content": "weather?"}],
			"stream": false
		}`))
		require.Equal(t, types.ActionPause, action)

		calls := host.GetRedisCalloutAttributes()
		require.Len(t, calls, 1)
		require.Contains(t, calls[0].Upstream, "redis-materialized.static")
		require.NotContains(t, calls[0].Upstream, "?")
		cmd, ok := thinResponseCaptureCommand(t, calls[0].Query)
		require.True(t, ok)
		require.Len(t, cmd, 2)
		require.Equal(t, "get", strings.ToLower(cmd[0]))
		require.True(t, strings.HasPrefix(cmd[1], "cache:materialized:"))
		for _, raw := range []string{"higress-ai-cache:", "weather?", "tenant-a", "consumer-a", "test-route-default", "qwen-turbo", "policy-v1"} {
			require.NotContains(t, cmd[1], raw)
		}
	})
}

func thinMaterializedLookupOnlyConfig(t *testing.T) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(map[string]interface{}{
		"materialized_lookup": map[string]interface{}{
			"redis": map[string]interface{}{
				"enabled":      true,
				"service_name": "redis-materialized.static",
				"service_port": 6380,
				"key_prefix":   "cache:materialized:",
				"timeout":      80,
			},
		},
		"route_policy": map[string]interface{}{
			"enable_redis_lookup":   true,
			"enable_console_lookup": false,
			"enable_replay":         true,
			"enabled_path_suffixes": []string{"/v1/chat/completions"},
		},
		"tenant_header":        "x-mse-tenant",
		"consumer_header":      "x-mse-consumer",
		"cache_scope":          "consumer",
		"cache_policy_version": "policy-v1",
	})
	require.NoError(t, err)
	return data
}
