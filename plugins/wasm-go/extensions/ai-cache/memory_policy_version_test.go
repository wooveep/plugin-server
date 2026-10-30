package main

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestMemoryAwareEventPolicyVersionAcceptedByConsole(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		host, _ := startThinMemoryAwareRequest(t, "policy_digest", "memory-policy-v1", "memory-digest-a", "consumer-a")
		defer host.Reset()
		lookupKey := requireThinMemoryAwareMaterializedGetKey(t, host)
		host.CallOnRedisCall(0, test.CreateRedisRespNull())
		host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
		host.CallOnHttpResponseBody([]byte(`{"choices":[{"message":{"content":"answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":4,"total_tokens":16}}`))
		event := requireThinMemoryAwareCacheEvent(t, host)
		material, err := BuildScopedCacheKeyMaterial(ScopedCacheKeyInput{
			KeyPrefix: "cache:materialized:", Tenant: event["tenant"].(string), Consumer: event["consumer"].(string),
			CacheScope: "consumer", Route: event["route"].(string), Model: event["model"].(string),
			Protocol: protocol.ProtocolChatCompletions, RequestDigest: event["request_digest"].(string),
			CachePolicyVersion: event["cache_policy_version"].(string),
		})
		require.NoError(t, err)
		require.Equal(t, lookupKey, material.RedisKey,
			"Console must materialize into the same isolated key that the next request reads")
		encoded, err := json.Marshal(event["cache_policy_version"])
		require.NoError(t, err)
		var version string
		require.NoError(t, json.Unmarshal(encoded, &version))
		// Console accepts positive integer versions or a final -v<integer> suffix.
		_, suffix, found := strings.Cut(version, "-v")
		if found {
			suffix = version[strings.LastIndex(version, "-v")+2:]
		} else {
			suffix = version
		}
		value, err := strconv.ParseInt(suffix, 10, 64)
		require.NoError(t, err, "memory-aware events must resolve the authoritative numeric cache policy")
		require.Equal(t, int64(1), value)
	})
}
