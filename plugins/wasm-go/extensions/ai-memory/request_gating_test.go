package main

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestMemoryRequestGating(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("supported json suffix stops for memory body inspection", func(t *testing.T) {
			host, headerAction, bodyAction := startMemoryRequestGatingRequest(t, memoryRequestGatingConfig(t, "digest"), nil)
			defer host.Reset()

			require.Equal(t, types.HeaderStopIteration, headerAction)
			require.Equal(t, types.ActionPause, bodyAction)
			require.False(t, test.HasHeader(host.GetRequestHeaders(), "accept-encoding"))
			require.False(t, test.HasHeader(host.GetRequestHeaders(), "content-length"))
		})

		t.Run("unsupported path suffix continues unchanged", func(t *testing.T) {
			host, headerAction, bodyAction := startMemoryRequestGatingRequest(t, memoryRequestGatingConfig(t, "digest"), [][2]string{
				{":path", "/v1/embeddings"},
			})
			defer host.Reset()

			require.Equal(t, types.ActionContinue, headerAction)
			require.Equal(t, types.ActionContinue, bodyAction)
			require.Empty(t, host.GetRedisCalloutAttributes())
			require.Empty(t, host.GetHttpCalloutAttributes())
			require.True(t, test.HasHeader(host.GetRequestHeaders(), "accept-encoding"))
			require.True(t, test.HasHeader(host.GetRequestHeaders(), "content-length"))
		})

		t.Run("non json content type continues unchanged", func(t *testing.T) {
			host, headerAction, bodyAction := startMemoryRequestGatingRequest(t, memoryRequestGatingConfig(t, "digest"), [][2]string{
				{"content-type", "text/plain"},
			})
			defer host.Reset()

			require.Equal(t, types.ActionContinue, headerAction)
			require.Equal(t, types.ActionContinue, bodyAction)
			require.Empty(t, host.GetRedisCalloutAttributes())
			require.Empty(t, host.GetHttpCalloutAttributes())
			require.True(t, test.HasHeader(host.GetRequestHeaders(), "accept-encoding"))
			require.True(t, test.HasHeader(host.GetRequestHeaders(), "content-length"))
		})

		t.Run("missing tenant identity continues without memory behavior", func(t *testing.T) {
			host, headerAction, bodyAction := startMemoryRequestGatingRequest(t, memoryRequestGatingConfig(t, "digest"), [][2]string{
				{"x-mse-tenant", ""},
			})
			defer host.Reset()

			require.Equal(t, types.ActionContinue, headerAction)
			require.Equal(t, types.ActionContinue, bodyAction)
			require.Empty(t, host.GetRedisCalloutAttributes())
			require.Empty(t, host.GetHttpCalloutAttributes())
			require.True(t, test.HasHeader(host.GetRequestHeaders(), "accept-encoding"))
			require.True(t, test.HasHeader(host.GetRequestHeaders(), "content-length"))
		})

		t.Run("missing consumer identity continues without memory behavior", func(t *testing.T) {
			host, headerAction, bodyAction := startMemoryRequestGatingRequest(t, memoryRequestGatingConfig(t, "digest"), [][2]string{
				{"x-mse-consumer", ""},
			})
			defer host.Reset()

			require.Equal(t, types.ActionContinue, headerAction)
			require.Equal(t, types.ActionContinue, bodyAction)
			require.Empty(t, host.GetRedisCalloutAttributes())
			require.Empty(t, host.GetHttpCalloutAttributes())
			require.True(t, test.HasHeader(host.GetRequestHeaders(), "accept-encoding"))
			require.True(t, test.HasHeader(host.GetRequestHeaders(), "content-length"))
		})

		t.Run("memory mode off continues without memory behavior", func(t *testing.T) {
			host, headerAction, bodyAction := startMemoryRequestGatingRequest(t, memoryRequestGatingConfig(t, "off"), nil)
			defer host.Reset()

			require.Equal(t, types.ActionContinue, headerAction)
			require.Equal(t, types.ActionContinue, bodyAction)
			require.Empty(t, host.GetRedisCalloutAttributes())
			require.Empty(t, host.GetHttpCalloutAttributes())
			require.True(t, test.HasHeader(host.GetRequestHeaders(), "accept-encoding"))
			require.True(t, test.HasHeader(host.GetRequestHeaders(), "content-length"))
		})
	})
}

func memoryRequestGatingConfig(t *testing.T, memoryMode string) json.RawMessage {
	t.Helper()
	return mustMemoryConfig(t, map[string]interface{}{
		"redis_stream": map[string]interface{}{
			"service_name": "redis.memory.svc.cluster.local",
		},
		"recent_cache": map[string]interface{}{
			"service_name": "redis.recent.svc.cluster.local",
		},
		"console_internal": map[string]interface{}{
			"service_name": "console.internal.svc.cluster.local",
		},
		"tenant_header":        "x-mse-tenant",
		"consumer_header":      "x-mse-consumer",
		"session_header":       "x-mse-session",
		"request_id_header":    "x-request-id",
		"enable_path_suffixes": []string{"/v1/chat/completions"},
		"fail_policy":          "open",
		"_rules_": []map[string]interface{}{
			{
				"_match_route_":       []string{"memory-route"},
				"memory_mode":         memoryMode,
				"recent_window_turns": 4,
				"capture_response":    true,
			},
		},
	})
}

func startMemoryRequestGatingRequest(t *testing.T, config json.RawMessage, overrides [][2]string) (test.TestHost, types.Action, types.Action) {
	t.Helper()
	host, status := newMemoryConfigTestHost(config)
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName("memory-route"))
	require.NoError(t, host.SetRequestId("request-memory-gating-1"))

	headers := [][2]string{
		{":authority", "example.com"},
		{":path", "/v1/chat/completions"},
		{":method", "POST"},
		{"content-type", "application/json"},
		{"x-mse-tenant", "tenant-a"},
		{"x-mse-consumer", "consumer-a"},
		{"x-mse-session", "session-a"},
		{"x-request-id", "request-header-a"},
		{"accept-encoding", "gzip"},
		{"content-length", "96"},
	}
	headers = applyMemoryRequestGatingHeaderOverrides(headers, overrides)
	headerAction := host.CallOnHttpRequestHeaders(headers)
	bodyAction := host.CallOnHttpRequestBody([]byte(`{
		"model": "qwen-turbo",
		"messages": [
			{"role": "user", "content": "remember my preference"}
		],
		"stream": false
	}`))
	return host, headerAction, bodyAction
}

func applyMemoryRequestGatingHeaderOverrides(headers [][2]string, overrides [][2]string) [][2]string {
	for _, override := range overrides {
		if override[1] == "" {
			headers = removeMemoryRequestGatingHeader(headers, override[0])
			continue
		}
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

func removeMemoryRequestGatingHeader(headers [][2]string, name string) [][2]string {
	for i := 0; i < len(headers); i++ {
		if headers[i][0] == name {
			headers = append(headers[:i], headers[i+1:]...)
			i--
		}
	}
	return headers
}
