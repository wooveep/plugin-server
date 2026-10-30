package main

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestThinRequestGating(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("unsupported path skips cache lookup", func(t *testing.T) {
			host, headerAction, bodyAction := startThinRequestGatingRequest(t, thinRequestGatingConfig(t, "consumer", false), [][2]string{
				{":path", "/v1/embeddings"},
			})
			defer host.Reset()

			require.Equal(t, types.ActionContinue, headerAction)
			require.Equal(t, types.ActionContinue, bodyAction)
			require.Empty(t, host.GetRedisCalloutAttributes())
			require.Empty(t, host.GetHttpCalloutAttributes())
		})

		t.Run("non json content type skips cache lookup", func(t *testing.T) {
			host, headerAction, bodyAction := startThinRequestGatingRequest(t, thinRequestGatingConfig(t, "consumer", false), [][2]string{
				{"content-type", "text/plain"},
			})
			defer host.Reset()

			require.Equal(t, types.ActionContinue, headerAction)
			require.Equal(t, types.ActionContinue, bodyAction)
			require.Empty(t, host.GetRedisCalloutAttributes())
			require.Empty(t, host.GetHttpCalloutAttributes())
		})

		t.Run("missing tenant skips cache lookup", func(t *testing.T) {
			host, headerAction, bodyAction := startThinRequestGatingRequest(t, thinRequestGatingConfig(t, "consumer", false), [][2]string{
				{"x-mse-tenant", ""},
			})
			defer host.Reset()

			require.Equal(t, types.ActionContinue, headerAction)
			require.Equal(t, types.ActionContinue, bodyAction)
			require.Empty(t, host.GetRedisCalloutAttributes())
			require.Empty(t, host.GetHttpCalloutAttributes())
		})

		t.Run("missing consumer on consumer scoped cache skips cache lookup", func(t *testing.T) {
			host, headerAction, bodyAction := startThinRequestGatingRequest(t, thinRequestGatingConfig(t, "consumer", false), [][2]string{
				{"x-mse-consumer", ""},
			})
			defer host.Reset()

			require.Equal(t, types.ActionContinue, headerAction)
			require.Equal(t, types.ActionContinue, bodyAction)
			require.Empty(t, host.GetRedisCalloutAttributes())
			require.Empty(t, host.GetHttpCalloutAttributes())
		})

		t.Run("tenant scoped cache allows missing consumer", func(t *testing.T) {
			host, headerAction, bodyAction := startThinRequestGatingRequest(t, thinRequestGatingConfig(t, "tenant", false), [][2]string{
				{"x-mse-consumer", ""},
			})
			defer host.Reset()

			require.Equal(t, types.HeaderStopIteration, headerAction)
			require.Equal(t, types.ActionPause, bodyAction)
			require.NotEmpty(t, host.GetRedisCalloutAttributes())
		})

		t.Run("request no store skips cache lookup", func(t *testing.T) {
			host, headerAction, bodyAction := startThinRequestGatingRequest(t, thinRequestGatingConfig(t, "consumer", false), [][2]string{
				{"cache-control", "private, no-store"},
			})
			defer host.Reset()

			require.Equal(t, types.ActionContinue, headerAction)
			require.Equal(t, types.ActionContinue, bodyAction)
			require.Empty(t, host.GetRedisCalloutAttributes())
			require.Empty(t, host.GetHttpCalloutAttributes())
		})

		t.Run("sensitive marker does not skip request lookup", func(t *testing.T) {
			host, headerAction, bodyAction := startThinRequestGatingRequest(t, thinRequestGatingConfig(t, "consumer", false), [][2]string{
				{"x-mse-cache-sensitive", "true"},
			})
			defer host.Reset()

			require.Equal(t, types.HeaderStopIteration, headerAction)
			require.Equal(t, types.ActionPause, bodyAction)
			require.NotEmpty(t, host.GetRedisCalloutAttributes())
		})

		t.Run("route policy bypass skips cache lookup", func(t *testing.T) {
			host, headerAction, bodyAction := startThinRequestGatingRequest(t, thinRequestGatingConfig(t, "consumer", true), nil)
			defer host.Reset()

			require.Equal(t, types.ActionContinue, headerAction)
			require.Equal(t, types.ActionContinue, bodyAction)
			require.Empty(t, host.GetRedisCalloutAttributes())
			require.Empty(t, host.GetHttpCalloutAttributes())
		})

		t.Run("supported json suffix content type proceeds to cache lookup", func(t *testing.T) {
			host, headerAction, bodyAction := startThinRequestGatingRequest(t, thinRequestGatingConfig(t, "consumer", false), [][2]string{
				{"content-type", "application/problem+json; charset=utf-8"},
			})
			defer host.Reset()

			require.Equal(t, types.HeaderStopIteration, headerAction)
			require.Equal(t, types.ActionPause, bodyAction)
			require.NotEmpty(t, host.GetRedisCalloutAttributes())
		})

	})
}

func thinRequestGatingConfig(t *testing.T, cacheScope string, enableBypass bool) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(map[string]interface{}{
		"materialized_lookup": map[string]interface{}{
			"redis": map[string]interface{}{
				"enabled":      true,
				"service_name": "redis.static",
				"service_port": 6379,
				"key_prefix":   "cache:materialized:",
				"timeout":      80,
			},
		},
		"console_lookup": map[string]interface{}{
			"enabled":      true,
			"service_name": thinConsoleService,
			"service_port": 8080,
			"path":         thinConsolePath,
			"timeout":      50,
		},
		"route_policy": map[string]interface{}{
			"enable_redis_lookup":   true,
			"enable_console_lookup": true,
			"enable_replay":         true,
			"enable_bypass":         enableBypass,
			"enabled_path_suffixes": []string{"/v1/chat/completions"},
		},
		"tenant_header":        "x-mse-tenant",
		"consumer_header":      "x-mse-consumer",
		"cache_scope":          cacheScope,
		"cache_policy_version": "policy-v1",
	})
	require.NoError(t, err)
	return data
}

func startThinRequestGatingRequest(t *testing.T, cfg json.RawMessage, overrides [][2]string) (test.TestHost, types.Action, types.Action) {
	t.Helper()
	host, status := test.NewTestHost(cfg)
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName("test-route-default"))

	headers := [][2]string{
		{":authority", "example.com"},
		{":path", "/v1/chat/completions"},
		{":method", "POST"},
		{"content-type", "application/json"},
		{"x-mse-tenant", "tenant-a"},
		{"x-mse-consumer", "consumer-a"},
	}
	headers = applyThinRequestGatingHeaderOverrides(headers, overrides)
	headerAction := host.CallOnHttpRequestHeaders(headers)
	bodyAction := host.CallOnHttpRequestBody([]byte(`{
		"model": "qwen-turbo",
		"messages": [
			{"role": "user", "content": "weather?"}
		],
		"stream": false
	}`))
	return host, headerAction, bodyAction
}

func applyThinRequestGatingHeaderOverrides(headers [][2]string, overrides [][2]string) [][2]string {
	for _, override := range overrides {
		replaced := false
		for i := range headers {
			if headers[i][0] == override[0] {
				headers[i][1] = override[1]
				replaced = true
				break
			}
		}
		if !replaced {
			headers = append(headers, override)
		}
	}
	return headers
}
