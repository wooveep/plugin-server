package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

const (
	thinConsoleService  = "console.static"
	thinConsoleUpstream = "outbound|8080||console.static"
	thinConsolePath     = "/internal/cache/lookup"
	thinConsoleToken    = "console-service-token"
)

func thinConsoleLookupConfig(t *testing.T, routeConsoleEnabled bool) json.RawMessage {
	t.Helper()
	return thinConsoleLookupConfigWithToken(t, routeConsoleEnabled, thinConsoleToken)
}

func thinConsoleLookupConfigWithToken(t *testing.T, routeConsoleEnabled bool, bearerToken string) json.RawMessage {
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
			"bearer_token": bearerToken,
			"timeout":      50,
		},
		"route_policy": map[string]interface{}{
			"enable_redis_lookup":   true,
			"enable_console_lookup": routeConsoleEnabled,
			"enable_replay":         true,
			"enabled_path_suffixes": []string{"/v1/chat/completions"},
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

func startThinConsoleLookupRequest(t *testing.T, routeConsoleEnabled bool) test.TestHost {
	t.Helper()
	return startThinConsoleLookupRequestWithConfig(t, thinConsoleLookupConfig(t, routeConsoleEnabled))
}

func startThinConsoleLookupRequestWithConfig(t *testing.T, rawConfig json.RawMessage) test.TestHost {
	t.Helper()
	host, status := test.NewTestHost(rawConfig)
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName("test-route-default"))
	action := host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"},
		{":path", "/v1/chat/completions"},
		{":method", "POST"},
		{"content-type", "application/json"},
		{"x-mse-tenant", "tenant-a"},
		{"x-mse-consumer", "consumer-a"},
		{"x-mse-session", "session-a"},
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
	require.NotEmpty(t, host.GetRedisCalloutAttributes(), "request should issue Redis lookup before Console lookup is considered")
	return host
}

func requireThinConsoleLookupCall(t *testing.T, host test.TestHost) map[string]interface{} {
	t.Helper()
	callouts := host.GetHttpCalloutAttributes()
	require.Len(t, callouts, 1, "Redis miss with Console lookup enabled should issue exactly one Console callout")
	callout := callouts[0]
	require.Equal(t, thinConsoleUpstream, callout.Upstream)
	method, ok := test.GetHeaderValue(callout.Headers, ":method")
	require.True(t, ok, "Console lookup call should set :method")
	require.Equal(t, "POST", method)
	path, ok := test.GetHeaderValue(callout.Headers, ":path")
	require.True(t, ok, "Console lookup call should set :path")
	require.Equal(t, thinConsolePath, path)
	require.True(t, test.HasHeaderWithValue(callout.Headers, "content-type", "application/json"), "Console lookup call should send a JSON body")
	require.True(t, test.HasHeaderWithValue(callout.Headers, "authorization", "Bearer "+thinConsoleToken), "Console lookup call should send configured bearer authentication")

	var facts map[string]interface{}
	if err := json.Unmarshal(callout.Body, &facts); err != nil {
		require.NoErrorf(t, err, "Console lookup call body must be JSON; body=%s", boundedThinConsoleBody(callout.Body))
	}
	require.Equal(t, "tenant-a", facts["tenant"])
	require.Equal(t, "consumer-a", facts["consumer"])
	require.Equal(t, "session-a", facts["session"])
	require.Equal(t, "test-route-default", facts["route"])
	require.Equal(t, "qwen-turbo", facts["model"])
	require.Equal(t, "chat_completions", facts["protocol"])
	require.Equal(t, "consumer", facts["cache_scope"])
	require.Equal(t, "policy-v1", facts["cache_policy_version"])
	require.Equal(t, "non_stream", facts["stream_mode"])
	require.Equal(t, "weather?", facts["semantic_query_text"])
	requestDigest, ok := facts["request_digest"].(string)
	require.True(t, ok, "Console lookup facts should include string request_digest")
	require.NotEmpty(t, requestDigest)
	return facts
}

func requireThinMaterializedRedisLookup(t *testing.T, host test.TestHost, materializedKey string) {
	t.Helper()
	calls := host.GetRedisCalloutAttributes()
	require.Lenf(t, calls, 1, "wrapped Console hit should fetch exactly one returned materialized Redis key")
	cmd, ok := thinResponseCaptureCommand(t, calls[0].Query)
	require.Truef(t, ok && len(cmd) == 2, "pending Redis lookup is not parseable GET; query=%q", string(calls[0].Query))
	require.Equal(t, "get", strings.ToLower(cmd[0]))
	require.Equal(t, materializedKey, cmd[1])
}

func validThinConsoleReplayRecord(t *testing.T, overrides map[string]interface{}) string {
	t.Helper()
	now := time.Now().UTC()
	record := map[string]interface{}{
		"id":                   "11111111-1111-4111-8111-111111111111",
		"source_event_id":      "22222222-2222-4222-8222-222222222222",
		"tenant_id":            "33333333-3333-4333-8333-333333333333",
		"gateway_tenant":       "tenant-a",
		"consumer_id":          "44444444-4444-4444-8444-444444444444",
		"gateway_consumer":     "consumer-a",
		"route_id":             "55555555-5555-4555-8555-555555555555",
		"gateway_route":        "test-route-default",
		"model_asset_id":       "66666666-6666-4666-8666-666666666666",
		"gateway_model":        "qwen-turbo",
		"protocol":             "chat_completions",
		"policy_id":            "77777777-7777-4777-8777-777777777777",
		"policy_version":       1,
		"cache_policy_version": "policy-v1",
		"request_digest":       thinRedisReplayRequestDigest(t),
		"cache_scope":          "consumer",
		"replay_status":        "active",
		"response_object": map[string]interface{}{
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
		"usage_snapshot": map[string]interface{}{
			"prompt_tokens":     7,
			"completion_tokens": 3,
			"total_tokens":      10,
		},
		"finish_reason":    "stop",
		"stream_mode":      "non_stream",
		"materialized_key": nil,
		"soft_expires_at":  now.Add(time.Minute).Format(time.RFC3339),
		"hard_expires_at":  now.Add(time.Hour).Format(time.RFC3339),
		"safe_to_replay":   true,
		"created_at":       now.Format(time.RFC3339),
		"updated_at":       now.Format(time.RFC3339),
	}
	for key, value := range overrides {
		record[key] = value
	}
	body, err := json.Marshal(record)
	require.NoError(t, err)
	return string(body)
}

func requireThinConsoleFailOpen(t *testing.T, host test.TestHost) {
	t.Helper()
	if localResponse := host.GetLocalResponse(); localResponse != nil {
		require.Failf(
			t,
			"Console lookup should fail open to upstream",
			"got local replay status=%d detail=%s body=%s",
			localResponse.StatusCode,
			localResponse.StatusCodeDetail,
			boundedThinConsoleBody(localResponse.Data),
		)
	}
	require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
}

func boundedThinConsoleBody(body []byte) string {
	text := string(body)
	if len(text) > 240 {
		return text[:240] + "..."
	}
	return text
}

func TestThinConsoleLookup(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("disabled lookup fails open after Redis miss without HTTP callout", func(t *testing.T) {
			host := startThinConsoleLookupRequest(t, false)
			defer host.Reset()

			host.CallOnRedisCall(0, test.CreateRedisRespNull())

			require.Empty(t, host.GetHttpCalloutAttributes(), "route-disabled Console lookup should not issue an HTTP callout")
			requireThinConsoleFailOpen(t, host)
		})

		t.Run("missing bearer token fails open after Redis miss without HTTP callout", func(t *testing.T) {
			host := startThinConsoleLookupRequestWithConfig(t, thinConsoleLookupConfigWithToken(t, true, ""))
			defer host.Reset()

			host.CallOnRedisCall(0, test.CreateRedisRespNull())

			require.Empty(t, host.GetHttpCalloutAttributes(), "Console lookup without configured bearer token should not issue an unauthenticated HTTP callout")
			requireThinConsoleFailOpen(t, host)
		})

		t.Run("enabled lookup sends facts after Redis miss", func(t *testing.T) {
			host := startThinConsoleLookupRequest(t, true)
			defer host.Reset()

			host.CallOnRedisCall(0, test.CreateRedisRespNull())

			requireThinConsoleLookupCall(t, host)
		})

		t.Run("wrapped hit fetches materialized key and replays validated OpenAI response", func(t *testing.T) {
			host := startThinConsoleLookupRequest(t, true)
			defer host.Reset()
			host.CallOnRedisCall(0, test.CreateRedisRespNull())
			requireThinConsoleLookupCall(t, host)
			materializedKey := "cache:materialized:console-hit"

			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"data":{"decision":"hit","materialized_key":"`+materializedKey+`"}}`))
			requireThinMaterializedRedisLookup(t, host, materializedKey)

			host.CallOnRedisCall(0, test.CreateRedisRespString(validThinReplayRecord(t, nil)))

			localResponse := host.GetLocalResponse()
			require.NotNil(t, localResponse, "valid Console replay record should be replayed")
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

		t.Run("wrapped inline replay record replays validated OpenAI response", func(t *testing.T) {
			host := startThinConsoleLookupRequest(t, true)
			defer host.Reset()
			host.CallOnRedisCall(0, test.CreateRedisRespNull())
			requireThinConsoleLookupCall(t, host)

			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"data":{"decision":"hit","reason":"precomputed_replay","replay_record":`+validThinConsoleReplayRecord(t, nil)+`}}`))

			localResponse := host.GetLocalResponse()
			require.NotNil(t, localResponse, "valid Console inline replay record should be replayed")
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

		t.Run("wrapped inline replay record stream mismatch fails open", func(t *testing.T) {
			host := startThinConsoleLookupRequest(t, true)
			defer host.Reset()
			host.CallOnRedisCall(0, test.CreateRedisRespNull())
			requireThinConsoleLookupCall(t, host)

			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"data":{"decision":"hit","reason":"precomputed_replay","replay_record":`+validThinConsoleReplayRecord(t, map[string]interface{}{
				"stream_mode": "stream",
			})+`}}`))

			requireThinConsoleFailOpen(t, host)
		})

		t.Run("invalid hit record fails open", func(t *testing.T) {
			host := startThinConsoleLookupRequest(t, true)
			defer host.Reset()
			host.CallOnRedisCall(0, test.CreateRedisRespNull())
			requireThinConsoleLookupCall(t, host)
			materializedKey := "cache:materialized:console-invalid"

			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"data":{"decision":"hit","materialized_key":"`+materializedKey+`"}}`))
			requireThinMaterializedRedisLookup(t, host, materializedKey)

			host.CallOnRedisCall(0, test.CreateRedisRespString(validThinReplayRecord(t, map[string]interface{}{"schema_version": "ai-cache.materialized.v0"})))

			requireThinConsoleFailOpen(t, host)
		})

		t.Run("miss response fails open", func(t *testing.T) {
			host := startThinConsoleLookupRequest(t, true)
			defer host.Reset()
			host.CallOnRedisCall(0, test.CreateRedisRespNull())
			requireThinConsoleLookupCall(t, host)

			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"data":{"decision":"miss","reason":"not_materialized"}}`))

			requireThinConsoleFailOpen(t, host)
		})

		t.Run("top-level error envelope fails open", func(t *testing.T) {
			host := startThinConsoleLookupRequest(t, true)
			defer host.Reset()
			host.CallOnRedisCall(0, test.CreateRedisRespNull())
			requireThinConsoleLookupCall(t, host)

			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"error":{"code":"UNAUTHENTICATED","message":"invalid service token"}}`))

			requireThinConsoleFailOpen(t, host)
		})

		t.Run("malformed wrapped body fails open", func(t *testing.T) {
			host := startThinConsoleLookupRequest(t, true)
			defer host.Reset()
			host.CallOnRedisCall(0, test.CreateRedisRespNull())
			requireThinConsoleLookupCall(t, host)

			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"data":`))

			requireThinConsoleFailOpen(t, host)
		})

		t.Run("timeout no-status callback fails open", func(t *testing.T) {
			host := startThinConsoleLookupRequest(t, true)
			defer host.Reset()
			host.CallOnRedisCall(0, test.CreateRedisRespNull())
			requireThinConsoleLookupCall(t, host)

			host.CallOnHttpCall(nil, nil)

			requireThinConsoleFailOpen(t, host)
		})

		t.Run("failure response fails open", func(t *testing.T) {
			host := startThinConsoleLookupRequest(t, true)
			defer host.Reset()
			host.CallOnRedisCall(0, test.CreateRedisRespNull())
			requireThinConsoleLookupCall(t, host)

			host.CallOnHttpCall([][2]string{
				{":status", "503"},
				{"content-type", "application/json"},
			}, []byte(`{"error":"console unavailable"}`))

			requireThinConsoleFailOpen(t, host)
		})
	})
}

func TestThinConsoleLookupInternalAuthAndWrappedResponseContract(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		host := startThinConsoleLookupRequest(t, true)
		defer host.Reset()

		host.CallOnRedisCall(0, test.CreateRedisRespNull())
		facts := requireThinConsoleLookupCall(t, host)
		require.Equal(t, "tenant-a", facts["tenant"])
		require.Equal(t, "consumer-a", facts["consumer"])
		require.Equal(t, "test-route-default", facts["route"])
		require.Equal(t, "qwen-turbo", facts["model"])
		require.Equal(t, "non_stream", facts["stream_mode"])

		materializedKey := "cache:materialized:console-contract-hit"
		host.CallOnHttpCall([][2]string{
			{":status", "200"},
			{"content-type", "application/json"},
		}, []byte(`{"data":{"decision":"hit","reason":"fresh","materialized_key":"`+materializedKey+`"},"error":null}`))
		requireThinMaterializedRedisLookup(t, host, materializedKey)

		host.CallOnRedisCall(0, test.CreateRedisRespString(validThinReplayRecord(t, map[string]interface{}{
			"upstream_invoked": false,
		})))

		localResponse := host.GetLocalResponse()
		require.NotNil(t, localResponse, "wrapped Console lookup hit should replay the validated materialized record")
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
