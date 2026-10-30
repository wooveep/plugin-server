package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/memorycache"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestThinMemoryAwareCache(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("consumer-scoped memory route separates materialized lookup keys by consumer", func(t *testing.T) {
			keyA := thinMemoryAwareLookupKey(t, "policy_digest", "memory-policy-v1", "memory-digest-a", "consumer-a")
			keyB := thinMemoryAwareLookupKey(t, "policy_digest", "memory-policy-v1", "memory-digest-a", "consumer-b")
			requireThinMemoryAwareKeysDiffer(t, keyA, keyB, "consumer-scoped memory-aware cache keys must vary by consumer")
		})

		t.Run("memory policy version and digest affect materialized lookup key", func(t *testing.T) {
			keyA := thinMemoryAwareLookupKey(t, "policy_digest", "memory-policy-v1", "memory-digest-a", "consumer-a")
			keySame := thinMemoryAwareLookupKey(t, "policy_digest", "memory-policy-v1", "memory-digest-a", "consumer-a")
			keyDigestChanged := thinMemoryAwareLookupKey(t, "policy_digest", "memory-policy-v1", "memory-digest-b", "consumer-a")
			keyPolicyChanged := thinMemoryAwareLookupKey(t, "policy_digest", "memory-policy-v2", "memory-digest-a", "consumer-a")

			require.True(t, keyA == keySame, "same memory-aware inputs should produce stable materialized key")
			requireThinMemoryAwareKeysDiffer(t, keyA, keyDigestChanged, "memory digest must affect memory-aware cache lookup key")
			requireThinMemoryAwareKeysDiffer(t, keyA, keyPolicyChanged, "memory policy version must affect memory-aware cache lookup key or policy material")
		})

		t.Run("lookup key and materialization event digest are based on final memory-adjusted request body", func(t *testing.T) {
			originalBody := thinMemoryAwareDefaultRequestBody()
			memoryAdjustedBody := []byte(`{
				"model": "qwen-turbo",
				"messages": [
					{"role": "system", "content": "Relevant memory: prefers metric units."},
					{"role": "user", "content": "weather?"}
				],
				"stream": false
			}`)

			originalKey := thinMemoryAwareLookupKeyForBody(t, "policy_digest", "memory-policy-v1", "memory-digest-a", "consumer-a", originalBody)
			host, action := startThinMemoryAwareRequestWithBody(t, "policy_digest", "memory-policy-v1", "memory-digest-a", "consumer-a", memoryAdjustedBody)
			defer host.Reset()
			require.Equal(t, types.ActionPause, action)
			requireThinMemoryAwareNoLegacyCacheCalls(t, host)
			adjustedKey := requireThinMemoryAwareMaterializedGetKey(t, host)

			requireThinMemoryAwareKeysDiffer(t, originalKey, adjustedKey, "ai-cache must compute memory-aware materialized lookup keys from the final memory-adjusted request body")

			originalModel, originalDigest := thinChatRequestDigestForBody(t, originalBody)
			adjustedModel, adjustedDigest := thinChatRequestDigestForBody(t, memoryAdjustedBody)
			require.Equal(t, originalModel, adjustedModel)
			requireThinMemoryAwareKeysDiffer(t, originalDigest, adjustedDigest, "memory injection must affect the digest used for materialization")
			adjustedMaterial, err := BuildScopedCacheKeyMaterial(ScopedCacheKeyInput{
				KeyPrefix:          "cache:materialized:",
				Tenant:             "tenant-a",
				Consumer:           "consumer-a",
				CacheScope:         "consumer",
				Route:              "test-route-default",
				Model:              adjustedModel,
				Protocol:           protocol.ProtocolChatCompletions,
				RequestDigest:      adjustedDigest,
				CachePolicyVersion: strings.Join([]string{"policy-v1", "memory", "memory-policy-v1", "memory-digest-a", "cache-v1"}, "|"),
			})
			require.NoError(t, err)
			require.Equal(t, adjustedMaterial.RedisKey, adjustedKey)

			host.CallOnRedisCall(0, test.CreateRedisRespNull())
			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action = host.CallOnHttpResponseBody([]byte(`{
				"id": "chatcmpl-memory-aware-materialize",
				"object": "chat.completion",
				"model": "qwen-turbo",
				"choices": [{
					"index": 0,
					"message": {"role": "assistant", "content": "metric weather answer"},
					"finish_reason": "stop"
				}],
				"usage": {"prompt_tokens": 12, "completion_tokens": 4, "total_tokens": 16}
			}`))
			require.Equal(t, types.ActionContinue, action)
			event := requireThinMemoryAwareCacheEvent(t, host)
			require.Equal(t, adjustedDigest, event["request_digest"])
			require.NotEqual(t, originalDigest, event["request_digest"])
			require.Equal(t, "weather?", event["user_content"])
			require.Equal(t, "metric weather answer", event["assistant_content"])
		})

		t.Run("configured memory bypass skips cache lookup and continues upstream", func(t *testing.T) {
			host, action := startThinMemoryAwareRequest(t, "bypass", "memory-policy-v1", "memory-digest-a", "consumer-a")
			defer host.Reset()

			require.Equal(t, types.ActionContinue, action)
			require.Empty(t, host.GetRedisCalloutAttributes(), "memory bypass should not issue Redis cache lookup")
			require.Empty(t, host.GetHttpCalloutAttributes(), "memory bypass should not issue Console cache lookup")
			require.Nil(t, host.GetLocalResponse(), "memory bypass should not replay a local cache hit")
		})

		t.Run("memory digest policy skips cache lookup before ai-memory assembles digest", func(t *testing.T) {
			host, action := startThinMemoryAwareRequest(t, "policy_digest", "memory-policy-v1", "", "consumer-a")
			defer host.Reset()

			require.Equal(t, types.ActionContinue, action)
			require.Empty(t, host.GetRedisCalloutAttributes(), "ai-cache must not look up memory-aware records before ai-memory provides a digest")
			require.Empty(t, host.GetHttpCalloutAttributes(), "ai-cache must not call Console cache lookup before ai-memory provides a digest")
			require.Nil(t, host.GetLocalResponse(), "missing memory digest should fail open to upstream")
		})
	})
}

func thinMemoryAwareLookupKey(t *testing.T, mode, memoryPolicyVersion, memoryDigest, consumer string) string {
	t.Helper()
	return thinMemoryAwareLookupKeyForBody(t, mode, memoryPolicyVersion, memoryDigest, consumer, thinMemoryAwareDefaultRequestBody())
}

func thinMemoryAwareLookupKeyForBody(t *testing.T, mode, memoryPolicyVersion, memoryDigest, consumer string, body []byte) string {
	t.Helper()
	host, action := startThinMemoryAwareRequestWithBody(t, mode, memoryPolicyVersion, memoryDigest, consumer, body)
	defer host.Reset()
	require.Equal(t, types.ActionPause, action)
	requireThinMemoryAwareNoLegacyCacheCalls(t, host)
	return requireThinMemoryAwareMaterializedGetKey(t, host)
}

func requireThinMemoryAwareKeysDiffer(t *testing.T, left, right, message string) {
	t.Helper()
	require.True(t, left != right, message)
}

func thinMemoryAwareConfig(t *testing.T, mode, memoryPolicyVersion string) json.RawMessage {
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
		"redis_stream": map[string]interface{}{
			"enabled":      true,
			"service_name": "redis.static",
			"service_port": 6379,
			"stream":       thinResponseCaptureStreamName,
			"field":        thinResponseCaptureStreamField,
			"timeout":      120,
		},
		"console_lookup": map[string]interface{}{
			"enabled":      false,
			"service_name": thinConsoleService,
			"service_port": 8080,
			"path":         thinConsolePath,
			"timeout":      50,
		},
		"route_policy": map[string]interface{}{
			"enable_redis_lookup":   true,
			"enable_console_lookup": false,
			"enable_replay":         true,
			"enabled_path_suffixes": []string{"/v1/chat/completions"},
			"memory": map[string]interface{}{
				"enabled":        true,
				"cache_mode":     mode,
				"policy_version": memoryPolicyVersion,
			},
		},
		"tenant_header":        "x-mse-tenant",
		"consumer_header":      "x-mse-consumer",
		"session_header":       "x-mse-session",
		"cache_scope":          "consumer",
		"cache_policy_version": "policy-v1",
	})
	require.NoError(t, err)
	return data
}

func startThinMemoryAwareRequest(t *testing.T, mode, memoryPolicyVersion, memoryDigest, consumer string) (test.TestHost, types.Action) {
	t.Helper()
	return startThinMemoryAwareRequestWithBody(t, mode, memoryPolicyVersion, memoryDigest, consumer, thinMemoryAwareDefaultRequestBody())
}

func startThinMemoryAwareRequestWithBody(t *testing.T, mode, memoryPolicyVersion, memoryDigest, consumer string, body []byte) (test.TestHost, types.Action) {
	t.Helper()
	host, status := test.NewTestHost(thinMemoryAwareConfig(t, mode, memoryPolicyVersion))
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName("test-route-default"))
	require.NoError(t, host.SetRequestId("request-memory-aware-1"))
	if memoryDigest != "" {
		require.NoError(t, host.SetProperty([]string{memorycache.TrustedDigestProperty}, []byte(memoryDigest)))
	}

	headers := [][2]string{
		{":authority", "example.com"},
		{":path", "/v1/chat/completions"},
		{":method", "POST"},
		{"content-type", "application/json"},
		{"x-mse-tenant", "tenant-a"},
		{"x-mse-consumer", consumer},
		{"x-mse-session", "session-a"},
		{memorycache.DeprecatedDigestHeader, "caller-supplied-memory-digest"},
	}
	action := host.CallOnHttpRequestHeaders(headers)
	require.Equal(t, types.HeaderStopIteration, action)
	require.False(t, test.HasHeader(host.GetRequestHeaders(), memorycache.DeprecatedDigestHeader), "ai-cache must strip the caller-visible legacy digest header")

	action = host.CallOnHttpRequestBody(body)
	return host, action
}

func thinMemoryAwareDefaultRequestBody() []byte {
	return []byte(`{
		"model": "qwen-turbo",
		"messages": [
			{"role": "user", "content": "weather?"}
		],
		"stream": false
	}`)
}

func requireThinMemoryAwareMaterializedGetKey(t *testing.T, host test.TestHost) string {
	t.Helper()
	for _, call := range host.GetRedisCalloutAttributes() {
		cmd, ok := thinResponseCaptureCommand(t, call.Query)
		if !ok || len(cmd) < 2 || !strings.EqualFold(cmd[0], "get") {
			continue
		}
		require.Truef(t, strings.HasPrefix(cmd[1], "cache:materialized:"), "memory-aware cache lookup should use materialized Redis key; redis calls=%s", thinResponseCaptureCallSummary(t, host))
		return cmd[1]
	}
	require.Failf(t, "missing memory-aware materialized Redis lookup", "redis calls=%s", thinResponseCaptureCallSummary(t, host))
	return ""
}

func requireThinMemoryAwareCacheEvent(t *testing.T, host test.TestHost) map[string]interface{} {
	t.Helper()
	event, ok := thinResponseCaptureEvent(t, host)
	if !ok {
		require.Failf(t, "missing memory-aware CacheEvent XADD", "redis calls=%s", thinResponseCaptureCallSummary(t, host))
	}
	return event
}

func requireThinMemoryAwareNoLegacyCacheCalls(t *testing.T, host test.TestHost) {
	t.Helper()
	for _, call := range host.GetRedisCalloutAttributes() {
		cmd, ok := thinResponseCaptureCommand(t, call.Query)
		if !ok || len(cmd) < 2 {
			continue
		}
		if strings.EqualFold(cmd[0], "get") || strings.EqualFold(cmd[0], "set") {
			require.Falsef(t, strings.HasPrefix(cmd[1], "higress-ai-cache:"), "memory-aware route must not use legacy cache lookup/write; redis calls=%s", thinResponseCaptureCallSummary(t, host))
		}
	}
}
