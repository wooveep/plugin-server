package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func thinRedisReplayConfig(t *testing.T) json.RawMessage {
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

func validThinReplayRecord(t *testing.T, overrides map[string]interface{}) string {
	t.Helper()
	now := time.Now().Unix()
	record := map[string]interface{}{
		"schema_version":       "ai-cache.materialized.v1",
		"tenant":               "tenant-a",
		"consumer":             "consumer-a",
		"route":                "test-route-default",
		"model":                "qwen-turbo",
		"protocol":             "chat_completions",
		"cache_scope":          "consumer",
		"cache_policy_version": "policy-v1",
		"request_digest":       thinRedisReplayRequestDigest(t),
		"soft_expires_at":      now + 60,
		"hard_expires_at":      now + 3600,
		"response": map[string]interface{}{
			"id":     "chatcmpl-cache-hit",
			"object": "chat.completion",
			"model":  "qwen-turbo",
			"choices": []interface{}{
				map[string]interface{}{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "cached answer",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     7,
				"completion_tokens": 3,
				"total_tokens":      10,
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     7,
			"completion_tokens": 3,
			"total_tokens":      10,
		},
		"finish_reason": "stop",
	}
	for key, value := range overrides {
		record[key] = value
	}
	body, err := json.Marshal(record)
	require.NoError(t, err)
	return string(body)
}

func startThinRedisReplayRequest(t *testing.T) test.TestHost {
	t.Helper()
	host, status := test.NewTestHost(thinRedisReplayConfig(t))
	require.Equal(t, types.OnPluginStartStatusOK, status)
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
		"messages": [
			{"role": "user", "content": "weather?"}
		],
		"stream": false
	}`))
	require.Equal(t, types.ActionPause, action)
	require.NotEmpty(t, host.GetRedisCalloutAttributes(), "request should issue Redis lookup before replay validation")
	return host
}

func TestThinRedisReplayValidation(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("valid materialized record replays OpenAI response", func(t *testing.T) {
			host := startThinRedisReplayRequest(t)
			defer host.Reset()

			host.CallOnRedisCall(0, test.CreateRedisRespString(validThinReplayRecord(t, nil)))

			localResponse := host.GetLocalResponse()
			require.NotNil(t, localResponse, "valid materialized record should be replayed")
			require.Equal(t, uint32(200), localResponse.StatusCode)
			require.JSONEq(t, `{
				"id": "chatcmpl-cache-hit",
				"object": "chat.completion",
				"model": "qwen-turbo",
				"choices": [{
					"index": 0,
					"message": {"role": "assistant", "content": "cached answer"},
					"finish_reason": "stop"
				}],
				"usage": {"prompt_tokens": 7, "completion_tokens": 3, "total_tokens": 10}
			}`, string(localResponse.Data))
		})

		tests := []struct {
			name      string
			status    int32
			redisResp []byte
		}{
			{
				name:      "miss",
				redisResp: test.CreateRedisRespNull(),
			},
			{
				name:      "RESP error",
				redisResp: test.CreateRedisRespError("temporary"),
			},
			{
				name:   "callout status failure",
				status: 1,
			},
			{
				name:      "invalid JSON",
				redisResp: test.CreateRedisRespString(`{not-json`),
			},
			{
				name:      "unsupported schema version",
				redisResp: test.CreateRedisRespString(validThinReplayRecord(t, map[string]interface{}{"schema_version": "ai-cache.materialized.v0"})),
			},
			{
				name:      "missing protocol",
				redisResp: test.CreateRedisRespString(validThinReplayRecord(t, map[string]interface{}{"protocol": ""})),
			},
			{
				name:      "protocol mismatch",
				redisResp: test.CreateRedisRespString(validThinReplayRecord(t, map[string]interface{}{"protocol": "responses"})),
			},
			{
				name:      "expired hard expiration",
				redisResp: test.CreateRedisRespString(validThinReplayRecord(t, map[string]interface{}{"hard_expires_at": int64(1)})),
			},
			{
				name:      "scope mismatch",
				redisResp: test.CreateRedisRespString(validThinReplayRecord(t, map[string]interface{}{"consumer": "consumer-b"})),
			},
			{
				name:      "route mismatch",
				redisResp: test.CreateRedisRespString(validThinReplayRecord(t, map[string]interface{}{"route": "other-route"})),
			},
			{
				name:      "model mismatch",
				redisResp: test.CreateRedisRespString(validThinReplayRecord(t, map[string]interface{}{"model": "other-model"})),
			},
			{
				name:      "policy version mismatch",
				redisResp: test.CreateRedisRespString(validThinReplayRecord(t, map[string]interface{}{"cache_policy_version": "policy-v2"})),
			},
			{
				name:      "request digest mismatch",
				redisResp: test.CreateRedisRespString(validThinReplayRecord(t, map[string]interface{}{"request_digest": "digest-mismatch"})),
			},
		}
		for _, tt := range tests {
			t.Run(tt.name+" fails open", func(t *testing.T) {
				host := startThinRedisReplayRequest(t)
				defer host.Reset()

				host.CallOnRedisCall(tt.status, tt.redisResp)

				if localResponse := host.GetLocalResponse(); localResponse != nil {
					body := string(localResponse.Data)
					if len(body) > 240 {
						body = body[:240] + "..."
					}
					require.Failf(t, "invalid or missing materialized record should fail open to upstream", "got local replay status=%d detail=%s body=%s", localResponse.StatusCode, localResponse.StatusCodeDetail, body)
				}
				require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			})
		}
	})
}

func TestThinRedisReplayConsoleGeneratedCanonicalFixture(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		host := startThinRedisReplayRequest(t)
		defer host.Reset()

		require.Equal(t, expectedThinRedisReplayMaterializedKey(t), requireThinRedisReplayMaterializedGetKey(t, host))
		record := validThinReplayRecord(t, map[string]interface{}{"upstream_invoked": false})
		requireThinRedisReplayCanonicalRecordShape(t, record)

		host.CallOnRedisCall(0, test.CreateRedisRespString(record))

		localResponse := host.GetLocalResponse()
		require.NotNil(t, localResponse, "Console-generated materialized Redis value should replay locally")
		require.Equal(t, uint32(200), localResponse.StatusCode)
		require.JSONEq(t, `{
			"id": "chatcmpl-cache-hit",
			"object": "chat.completion",
			"model": "qwen-turbo",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "cached answer"},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 7, "completion_tokens": 3, "total_tokens": 10}
		}`, string(localResponse.Data))

		attrs := thinCacheReplayAILogAttributes(t, host)
		require.Equal(t, "hit", attrs["cache_status"])
		require.Equal(t, false, attrs["upstream_invoked"])
	})
}

func thinRedisReplayRequestDigest(t *testing.T) string {
	t.Helper()
	_, digest := thinChatRequestDigestForBody(t, []byte(`{
		"model": "qwen-turbo",
		"messages": [
			{"role": "user", "content": "weather?"}
		],
		"stream": false
	}`))
	return digest
}

func thinChatRequestDigestForBody(t *testing.T, body []byte) (string, string) {
	t.Helper()
	adapter := protocol.ChatCompletionsAdapter{}
	digest, err := adapter.BuildCacheDigest(protocol.RequestParseInput{
		Method: "POST",
		Path:   "/v1/chat/completions",
		Body:   body,
	})
	require.NoError(t, err)
	return digest.Input.Model, digest.Digest
}

func expectedThinRedisReplayMaterializedKey(t *testing.T) string {
	t.Helper()
	material, err := BuildScopedCacheKeyMaterial(ScopedCacheKeyInput{
		KeyPrefix:          "cache:materialized:",
		Tenant:             "tenant-a",
		Consumer:           "consumer-a",
		CacheScope:         "consumer",
		Route:              "test-route-default",
		Model:              "qwen-turbo",
		Protocol:           protocol.ProtocolChatCompletions,
		RequestDigest:      thinRedisReplayRequestDigest(t),
		CachePolicyVersion: "policy-v1",
	})
	require.NoError(t, err)
	return material.RedisKey
}

func requireThinRedisReplayMaterializedGetKey(t *testing.T, host test.TestHost) string {
	t.Helper()
	for _, call := range host.GetRedisCalloutAttributes() {
		cmd, ok := thinResponseCaptureCommand(t, call.Query)
		if !ok || len(cmd) != 2 || !strings.EqualFold(cmd[0], "get") {
			continue
		}
		require.Truef(t, strings.HasPrefix(cmd[1], "cache:materialized:"), "Redis replay should query a materialized cache key; redis calls=%s", thinResponseCaptureCallSummary(t, host))
		return cmd[1]
	}
	require.Failf(t, "missing materialized Redis lookup", "redis calls=%s", thinResponseCaptureCallSummary(t, host))
	return ""
}

func requireThinRedisReplayCanonicalRecordShape(t *testing.T, raw string) {
	t.Helper()
	var record map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &record))

	requiredFields := []string{
		"schema_version",
		"tenant",
		"consumer",
		"route",
		"model",
		"protocol",
		"cache_scope",
		"cache_policy_version",
		"request_digest",
		"soft_expires_at",
		"hard_expires_at",
		"response",
		"usage",
		"finish_reason",
		"upstream_invoked",
	}
	for _, field := range requiredFields {
		require.Contains(t, record, field)
	}
	require.Equal(t, materializedRecordSchemaVersion, record["schema_version"])
	require.Equal(t, "tenant-a", record["tenant"])
	require.Equal(t, "consumer-a", record["consumer"])
	require.Equal(t, "test-route-default", record["route"])
	require.Equal(t, "qwen-turbo", record["model"])
	require.Equal(t, "chat_completions", record["protocol"])
	require.Equal(t, "consumer", record["cache_scope"])
	require.Equal(t, "policy-v1", record["cache_policy_version"])
	require.Equal(t, thinRedisReplayRequestDigest(t), record["request_digest"])
	require.Equal(t, false, record["upstream_invoked"])

	for _, legacyField := range []string{
		"replay_id",
		"source_event_id",
		"tenant_id",
		"consumer_id",
		"route_id",
		"model_asset_id",
		"policy_id",
		"policy_version",
		"response_object",
		"usage_snapshot",
		"gateway_tenant",
		"gateway_consumer",
		"gateway_route",
		"gateway_model",
	} {
		require.NotContains(t, record, legacyField)
	}
}
