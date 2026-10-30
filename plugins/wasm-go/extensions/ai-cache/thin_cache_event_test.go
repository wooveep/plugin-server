package main

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestThinCacheEvent(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("eligible non-streaming response emits required safe payload", func(t *testing.T) {
			host := startThinCacheEventRequestWithExtraFields(t, false, "weather?", map[string]interface{}{
				"unpermitted_request_field": "request-extra-should-not-appear",
			})
			defer host.Reset()

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{
				"id": "chatcmpl-cache-event",
				"object": "chat.completion",
				"model": "qwen-turbo",
				"choices": [{
					"index": 0,
					"message": {"role": "assistant", "content": "event assistant answer"},
					"finish_reason": "stop"
				}],
				"usage": {"prompt_tokens": 9, "completion_tokens": 4, "total_tokens": 13},
				"unpermitted_response_field": "response-extra-should-not-appear"
			}`))
			require.Equal(t, types.ActionContinue, action)

			event := requireThinResponseCaptureEvent(t, host)
			requireThinCacheEventRequiredFields(t, event)
			require.Equal(t, "tenant-a", requireThinCacheEventString(t, event, "tenant"))
			require.Equal(t, "consumer-a", requireThinCacheEventString(t, event, "consumer"))
			require.Equal(t, "session-a", requireThinCacheEventString(t, event, "session_id"))
			require.Equal(t, "test-route-default", requireThinCacheEventString(t, event, "route"))
			require.Equal(t, "qwen-turbo", requireThinCacheEventString(t, event, "model"))
			require.Equal(t, "request-capture-1", requireThinCacheEventString(t, event, "request_id"))
			require.Equal(t, "/v1/chat/completions", requireThinCacheEventString(t, event, "request_path"))
			require.Equal(t, "chat_completions", requireThinCacheEventString(t, event, "protocol"))
			require.Equal(t, "consumer", requireThinCacheEventString(t, event, "cache_scope"))
			require.Equal(t, "policy-v1", requireThinCacheEventString(t, event, "cache_policy_version"))
			require.Equal(t, "weather?", requireThinCacheEventString(t, event, "user_content"))
			require.Equal(t, "event assistant answer", requireThinCacheEventString(t, event, "assistant_content"))
			require.Equal(t, "stop", requireThinCacheEventString(t, event, "finish_reason"))
			require.EqualValues(t, 200, event["status_code"])
			require.Equal(t, false, event["is_stream"])
			require.Equal(t, false, event["contains_tool_calls"])
			require.Equal(t, false, event["no_store"])
			require.Equal(t, false, event["sensitive"])
			requireThinResponseCaptureUsage(t, event, 9, 4, 13)

			startedAt := requireThinCacheEventMillis(t, event, "started_at_ms")
			endedAt := requireThinCacheEventMillis(t, event, "ended_at_ms")
			require.LessOrEqual(t, startedAt, endedAt)
			require.NotEmpty(t, requireThinCacheEventString(t, event, "event_id"))
			require.NotEmpty(t, requireThinCacheEventString(t, event, "idempotency_key"))
			require.NotEmpty(t, requireThinCacheEventString(t, event, "request_digest"))
			require.NotEmpty(t, requireThinCacheEventString(t, event, "plugin_version"))

			eventJSON := requireThinCacheEventJSON(t, event)
			requireThinCacheEventJSONExcludes(t, event, eventJSON, "unpermitted request value", "request-extra-should-not-appear")
			requireThinCacheEventJSONExcludes(t, event, eventJSON, "unpermitted request field", "unpermitted_request_field")
			requireThinCacheEventJSONExcludes(t, event, eventJSON, "unpermitted response value", "response-extra-should-not-appear")
			requireThinCacheEventJSONExcludes(t, event, eventJSON, "unpermitted response field", "unpermitted_response_field")
			requireThinResponseNoLegacyCacheSet(t, host)
		})

		t.Run("secret headers and sensitive content are redacted from event", func(t *testing.T) {
			const (
				rawUserContent      = "event user content redaction sample"
				rawAssistantContent = "event assistant content redaction sample"
			)
			secretHeaders := [][2]string{
				{thinResponseCaptureSensitiveHeader, "true"},
				{"authorization", "Bearer internal-user-secret"},
				{"x-api-key", "sk-request-secret"},
				{"x-internal-bearer", "Bearer gateway-internal-secret"},
				{"x-redis-password", "redis-password-secret"},
				{"x-provider-api-key", "provider-api-secret"},
			}
			host := startThinCacheEventRequestWithUserContent(t, false, rawUserContent, secretHeaders...)
			defer host.Reset()

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{
				"id": "chatcmpl-cache-event-sensitive",
				"object": "chat.completion",
				"model": "qwen-turbo",
				"choices": [{
					"index": 0,
					"message": {"role": "assistant", "content": "` + rawAssistantContent + `"},
					"finish_reason": "stop"
				}],
				"usage": {"prompt_tokens": 6, "completion_tokens": 4, "total_tokens": 10}
			}`))
			require.Equal(t, types.ActionContinue, action)

			event := requireThinResponseCaptureEvent(t, host)
			require.Equal(t, true, event["sensitive"])
			requireThinCacheEventOmitsField(t, event, "user_content")
			requireThinCacheEventOmitsField(t, event, "assistant_content")

			eventJSON := requireThinCacheEventJSON(t, event)
			eventJSONLower := strings.ToLower(eventJSON)
			for _, secret := range []string{
				rawUserContent,
				rawAssistantContent,
				"Bearer internal-user-secret",
				"sk-request-secret",
				"Bearer gateway-internal-secret",
				"redis-password-secret",
				"provider-api-secret",
			} {
				requireThinCacheEventJSONExcludes(t, event, eventJSON, "secret value", secret)
			}
			for _, headerName := range []string{
				"authorization",
				"x-api-key",
				"x-internal-bearer",
				"x-redis-password",
				"x-provider-api-key",
			} {
				requireThinCacheEventJSONExcludes(t, event, eventJSONLower, "secret header name", headerName)
			}
			requireThinResponseNoLegacyCacheSet(t, host)
		})

		t.Run("redis stream XADD failures fail open and keep response body unchanged", func(t *testing.T) {
			tests := []struct {
				name     string
				status   int32
				response []byte
			}{
				{
					name:     "RESP error",
					status:   0,
					response: test.CreateRedisRespError("temporary"),
				},
				{
					name:   "callout status failure",
					status: 1,
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					requireThinCacheEventXADDFailureFailsOpen(t, tt.status, tt.response)
				})
			}
		})
	})
}

func requireThinCacheEventXADDFailureFailsOpen(t *testing.T, status int32, response []byte) {
	t.Helper()
	host := startThinResponseCaptureRequest(t, false)
	defer host.Reset()

	responseBody := []byte(`{"id":"chatcmpl-cache-event-xadd-error","object":"chat.completion","model":"qwen-turbo","choices":[{"index":0,"message":{"role":"assistant","content":"response survives xadd failure"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":4,"total_tokens":9}}`)
	host.CallOnHttpResponseHeaders([][2]string{
		{":status", "200"},
		{"content-type", "application/json"},
	})
	action := host.CallOnHttpResponseBody(responseBody)
	require.Equal(t, types.ActionContinue, action)
	require.Equal(t, string(responseBody), string(host.GetResponseBody()))

	requireThinCacheEventXADDCallout(t, host)
	host.CallOnRedisCall(status, response)

	require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
	require.Nil(t, host.GetLocalResponse())
	require.Equal(t, string(responseBody), string(host.GetResponseBody()))
	requireThinResponseNoLegacyCacheSet(t, host)
}

func startThinCacheEventRequestWithUserContent(t *testing.T, stream bool, userContent string, extraHeaders ...[2]string) test.TestHost {
	t.Helper()
	return startThinCacheEventRequestWithExtraFields(t, stream, userContent, nil, extraHeaders...)
}

func startThinCacheEventRequestWithExtraFields(t *testing.T, stream bool, userContent string, extraFields map[string]interface{}, extraHeaders ...[2]string) test.TestHost {
	t.Helper()
	host, status := test.NewTestHost(thinResponseCaptureConfig(t))
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName("test-route-default"))
	require.NoError(t, host.SetRequestId("request-capture-1"))

	headers := [][2]string{
		{":authority", "example.com"},
		{":path", "/v1/chat/completions"},
		{":method", "POST"},
		{"content-type", "application/json"},
		{"x-mse-tenant", "tenant-a"},
		{"x-mse-consumer", "consumer-a"},
		{"x-mse-session", "session-a"},
	}
	headers = append(headers, extraHeaders...)

	action := host.CallOnHttpRequestHeaders(headers)
	require.Equal(t, types.HeaderStopIteration, action)

	body := map[string]interface{}{
		"model": "qwen-turbo",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": userContent},
		},
		"stream": stream,
	}
	for key, value := range extraFields {
		body[key] = value
	}
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	action = host.CallOnHttpRequestBody(bodyBytes)
	require.Equal(t, types.ActionPause, action)
	require.NotEmpty(t, host.GetRedisCalloutAttributes(), "request should issue Redis lookup before upstream capture")
	materializedLookup := thinResponseHasMaterializedLookup(t, host)
	lookupSummary := thinResponseCaptureCallSummary(t, host)

	host.CallOnRedisCall(0, test.CreateRedisRespNull())
	require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
	return thinResponseCaptureHost{
		TestHost:           host,
		materializedLookup: materializedLookup,
		lookupSummary:      lookupSummary,
	}
}

func requireThinCacheEventRequiredFields(t *testing.T, event map[string]interface{}) {
	t.Helper()
	for _, field := range []string{
		"event_id",
		"idempotency_key",
		"tenant",
		"consumer",
		"route",
		"model",
		"request_id",
		"request_path",
		"protocol",
		"request_digest",
		"cache_scope",
		"cache_policy_version",
		"status_code",
		"is_stream",
		"contains_tool_calls",
		"no_store",
		"sensitive",
		"started_at_ms",
		"ended_at_ms",
		"plugin_version",
	} {
		require.Containsf(t, event, field, "CacheEvent missing required field %q; fields=%s", field, thinCacheEventFieldList(event))
	}
}

func requireThinCacheEventJSON(t *testing.T, event map[string]interface{}) string {
	t.Helper()
	eventBytes, err := json.Marshal(event)
	require.NoError(t, err)
	return string(eventBytes)
}

func requireThinCacheEventOmitsField(t *testing.T, event map[string]interface{}, field string) {
	t.Helper()
	_, exists := event[field]
	require.Falsef(t, exists, "CacheEvent included %s; fields=%s", field, thinCacheEventFieldList(event))
}

func requireThinCacheEventJSONExcludes(t *testing.T, event map[string]interface{}, eventJSON, label, value string) {
	t.Helper()
	require.Falsef(t, strings.Contains(eventJSON, value), "CacheEvent leaked %s; fields=%s", label, thinCacheEventFieldList(event))
}

func thinCacheEventFieldList(event map[string]interface{}) string {
	keys := make([]string, 0, len(event))
	for key := range event {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func requireThinCacheEventString(t *testing.T, event map[string]interface{}, key string) string {
	t.Helper()
	value, ok := event[key].(string)
	require.Truef(t, ok, "CacheEvent field %q must be a string; fields=%s", key, thinCacheEventFieldList(event))
	require.NotEmptyf(t, value, "CacheEvent field %q must not be empty; fields=%s", key, thinCacheEventFieldList(event))
	return value
}

func requireThinCacheEventMillis(t *testing.T, event map[string]interface{}, key string) float64 {
	t.Helper()
	value, ok := event[key].(float64)
	require.Truef(t, ok, "CacheEvent field %q must be a JSON number; fields=%s", key, thinCacheEventFieldList(event))
	require.Greaterf(t, value, float64(0), "CacheEvent field %q must be a positive epoch millis; fields=%s", key, thinCacheEventFieldList(event))
	return value
}

func requireThinCacheEventXADDCallout(t *testing.T, host test.TestHost) {
	t.Helper()
	calls := host.GetRedisCalloutAttributes()
	require.Lenf(t, calls, 1, "expected exactly one pending CacheEvent XADD with no legacy response writes; redis calls=%s", thinResponseCaptureCallSummary(t, host))
	cmd, ok := thinResponseCaptureCommand(t, calls[0].Query)
	require.Truef(t, ok && len(cmd) >= 2, "pending Redis call is not parseable RESP; redis calls=%s", thinResponseCaptureCallSummary(t, host))
	require.Truef(t, strings.EqualFold(cmd[0], "xadd") && cmd[1] == thinResponseCaptureStreamName, "pending Redis call should be CacheEvent XADD; redis calls=%s", thinResponseCaptureCallSummary(t, host))
}
