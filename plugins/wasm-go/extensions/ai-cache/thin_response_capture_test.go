package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/resp"
)

const (
	thinResponseCaptureStreamName      = "cache:events"
	thinResponseCaptureStreamField     = "event"
	thinResponseCaptureSensitiveHeader = "x-mse-cache-sensitive"
)

type thinResponseCaptureHost struct {
	test.TestHost
	materializedLookup bool
	lookupSummary      string
}

func (h thinResponseCaptureHost) thinMaterializedLookup() (bool, string) {
	return h.materializedLookup, h.lookupSummary
}

func thinResponseCaptureConfig(t *testing.T) json.RawMessage {
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
		"route_policy": map[string]interface{}{
			"enable_redis_lookup":   true,
			"enable_console_lookup": false,
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

func startThinResponseCaptureRequest(t *testing.T, stream bool, extraHeaders ...[2]string) test.TestHost {
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

	body, err := json.Marshal(map[string]interface{}{
		"model": "qwen-turbo",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "weather?"},
		},
		"stream": stream,
	})
	require.NoError(t, err)

	action = host.CallOnHttpRequestBody(body)
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

func TestThinResponseCapture(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("non-streaming upstream response emits assistant text usage and finish reason", func(t *testing.T) {
			host := startThinResponseCaptureRequest(t, false)
			defer host.Reset()

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{
				"id": "chatcmpl-response-capture",
				"object": "chat.completion",
				"model": "qwen-turbo",
				"choices": [{
					"index": 0,
					"message": {"role": "assistant", "content": "captured non-stream answer"},
					"finish_reason": "stop"
				}],
				"usage": {"prompt_tokens": 9, "completion_tokens": 4, "total_tokens": 13}
			}`))
			require.Equal(t, types.ActionContinue, action)

			event := requireThinResponseCaptureEvent(t, host)
			require.Equal(t, "captured non-stream answer", event["assistant_content"])
			require.Equal(t, "stop", event["finish_reason"])
			require.Equal(t, false, event["is_stream"])
			require.Equal(t, "qwen-turbo", event["model"])
			require.Equal(t, "tenant-a", event["tenant"])
			require.Equal(t, "consumer-a", event["consumer"])
			require.Equal(t, "consumer", event["cache_scope"])
			require.Equal(t, "policy-v1", event["cache_policy_version"])
			require.EqualValues(t, 200, event["status_code"])
			requireThinResponseCaptureUsage(t, event, 9, 4, 13)
			requireThinResponseNoLegacyCacheSet(t, host)
		})

		t.Run("chunked non-streaming upstream response is accumulated before parsing", func(t *testing.T) {
			host := startThinResponseCaptureRequest(t, false)
			defer host.Reset()

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			responseBody := []byte(`{
				"id": "chatcmpl-response-capture-chunked",
				"object": "chat.completion",
				"model": "qwen-turbo",
				"choices": [{
					"index": 0,
					"message": {"role": "assistant", "content": "captured chunked answer"},
					"finish_reason": "stop"
				}],
				"usage": {"prompt_tokens": 11, "completion_tokens": 3, "total_tokens": 14}
			}`)
			splitAt := bytes.Index(responseBody, []byte(`"captured chunked answer"`))
			require.Greater(t, splitAt, 0)

			action := host.CallOnHttpStreamingResponseBody(responseBody[:splitAt], false)
			require.Equal(t, types.ActionContinue, action)
			_, emitted := thinResponseCaptureEvent(t, host)
			require.False(t, emitted)

			action = host.CallOnHttpStreamingResponseBody(responseBody[splitAt:], true)
			require.Equal(t, types.ActionContinue, action)

			event := requireThinResponseCaptureEvent(t, host)
			require.Equal(t, "captured chunked answer", event["assistant_content"])
			require.Equal(t, "stop", event["finish_reason"])
			require.Equal(t, false, event["is_stream"])
			requireThinResponseCaptureUsage(t, event, 11, 3, 14)
			requireThinResponseNoLegacyCacheSet(t, host)
		})

		t.Run("streaming upstream response emits concatenated text usage and finish reason", func(t *testing.T) {
			host := startThinResponseCaptureRequest(t, true)
			defer host.Reset()

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})
			callThinResponseStreamChunks(t, host,
				`data: {"id":"chatcmpl-stream-capture","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}

`,
				`data: {"id":"chatcmpl-stream-capture","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"content":"captured "},"finish_reason":null}]}

data: {"id":"chatcmpl-stream-capture","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"content":"stream answer"},"finish_reason":null}]}

`,
				`data: {"id":"chatcmpl-stream-capture","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}

data: [DONE]

`)

			event := requireThinResponseCaptureEvent(t, host)
			require.Equal(t, "captured stream answer", event["assistant_content"])
			require.Equal(t, "stop", event["finish_reason"])
			require.Equal(t, true, event["is_stream"])
			require.Equal(t, "qwen-turbo", event["model"])
			require.EqualValues(t, 200, event["status_code"])
			requireThinResponseCaptureUsage(t, event, 10, 5, 15)
			requireThinResponseNoLegacyCacheSet(t, host)
		})

		t.Run("tool-call response does not emit cacheable raw assistant content", func(t *testing.T) {
			host := startThinResponseCaptureRequest(t, false)
			defer host.Reset()

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{
				"id": "chatcmpl-tool-call",
				"object": "chat.completion",
				"model": "qwen-turbo",
				"choices": [{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "do not materialize tool-call content",
						"tool_calls": [{"id":"call-1","type":"function","function":{"name":"lookup_weather","arguments":"{}"}}]
					},
					"finish_reason": "tool_calls"
				}],
				"usage": {"prompt_tokens": 8, "completion_tokens": 3, "total_tokens": 11}
			}`))
			require.Equal(t, types.ActionContinue, action)

			requireThinResponseCaptureGate(t, host, 200, "contains_tool_calls")
		})

		t.Run("streaming tool-call response does not emit cacheable raw assistant content", func(t *testing.T) {
			host := startThinResponseCaptureRequest(t, true)
			defer host.Reset()

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})
			callThinResponseStreamChunks(t, host,
				`data: {"id":"chatcmpl-stream-tool-call","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}

`,
				`data: {"id":"chatcmpl-stream-tool-call","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"content":"do not materialize streaming tool content"},"finish_reason":null}]}

`,
				`data: {"id":"chatcmpl-stream-tool-call","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call-1","type":"function","function":{"name":"lookup_weather","arguments":"{}"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-stream-tool-call","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":8,"completion_tokens":3,"total_tokens":11}}

data: [DONE]

`)

			requireThinResponseCaptureGate(t, host, 200, "contains_tool_calls")
		})

		t.Run("no-store response does not emit cacheable raw assistant content", func(t *testing.T) {
			host := startThinResponseCaptureRequest(t, false)
			defer host.Reset()

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
				{"cache-control", "no-store"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{
				"id": "chatcmpl-no-store",
				"object": "chat.completion",
				"model": "qwen-turbo",
				"choices": [{
					"index": 0,
					"message": {"role": "assistant", "content": "do not materialize no-store content"},
					"finish_reason": "stop"
				}],
				"usage": {"prompt_tokens": 7, "completion_tokens": 4, "total_tokens": 11}
			}`))
			require.Equal(t, types.ActionContinue, action)

			requireThinResponseCaptureGate(t, host, 200, "no_store")
		})

		t.Run("sensitive marker does not emit cacheable raw assistant content", func(t *testing.T) {
			host := startThinResponseCaptureRequest(t, false, [2]string{thinResponseCaptureSensitiveHeader, "true"})
			defer host.Reset()

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{
				"id": "chatcmpl-sensitive",
				"object": "chat.completion",
				"model": "qwen-turbo",
				"choices": [{
					"index": 0,
					"message": {"role": "assistant", "content": "do not materialize sensitive content"},
					"finish_reason": "stop"
				}],
				"usage": {"prompt_tokens": 6, "completion_tokens": 4, "total_tokens": 10}
			}`))
			require.Equal(t, types.ActionContinue, action)

			requireThinResponseCaptureGate(t, host, 200, "sensitive")
		})

		t.Run("malformed response does not emit cacheable raw assistant content", func(t *testing.T) {
			host := startThinResponseCaptureRequest(t, false)
			defer host.Reset()

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{not-json`))
			require.Equal(t, types.ActionContinue, action)

			requireThinResponseCaptureGate(t, host, 200)
		})

		t.Run("failed upstream status does not emit cacheable raw assistant content", func(t *testing.T) {
			host := startThinResponseCaptureRequest(t, false)
			defer host.Reset()

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "500"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{
				"id": "chatcmpl-failed",
				"object": "chat.completion",
				"model": "qwen-turbo",
				"choices": [{
					"index": 0,
					"message": {"role": "assistant", "content": "do not materialize failed response content"},
					"finish_reason": "stop"
				}],
				"usage": {"prompt_tokens": 5, "completion_tokens": 4, "total_tokens": 9}
			}`))
			require.Equal(t, types.ActionContinue, action)

			requireThinResponseCaptureGate(t, host, 500)
		})
	})
}

func TestThinMessagesResponseCapture(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		host := startThinMessagesResponseCaptureRequest(t)
		defer host.Reset()

		host.CallOnHttpResponseHeaders([][2]string{
			{":status", "200"},
			{"content-type", "application/json"},
		})
		action := host.CallOnHttpResponseBody([]byte(`{
			"id": "msg-cache-event",
			"type": "message",
			"role": "assistant",
			"model": "claude-sonnet",
			"content": [
				{"type":"text","text":"Messages cache answer"}
			],
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 14, "output_tokens": 7}
		}`))
		require.Equal(t, types.ActionContinue, action)

		event := requireThinResponseCaptureEvent(t, host)
		require.Equal(t, "tenant-a", event["tenant"])
		require.Equal(t, "consumer-a", event["consumer"])
		require.Equal(t, "session-a", event["session_id"])
		require.Equal(t, "test-route-messages", event["route"])
		require.Equal(t, "claude-sonnet", event["model"])
		require.Equal(t, "request-messages-1", event["request_id"])
		require.Equal(t, "/v1/messages", event["request_path"])
		require.Equal(t, "messages", event["protocol"])
		require.Equal(t, "consumer", event["cache_scope"])
		require.Equal(t, "policy-v1", event["cache_policy_version"])
		require.Equal(t, expectedThinMessagesResponseCaptureRequestDigest(t), event["request_digest"])
		require.Equal(t, "summarize launch plan", event["user_content"])
		require.Equal(t, "Messages cache answer", event["assistant_content"])
		require.Equal(t, "end_turn", event["finish_reason"])
		require.EqualValues(t, 200, event["status_code"])
		require.Equal(t, false, event["is_stream"])
		require.Equal(t, false, event["contains_tool_calls"])
		require.Equal(t, false, event["no_store"])
		require.Equal(t, false, event["sensitive"])
		requireThinMessagesResponseCaptureUsage(t, event, 14, 7)
		requireThinResponseNoLegacyCacheSet(t, host)
	})
}

func startThinMessagesResponseCaptureRequest(t *testing.T) test.TestHost {
	t.Helper()
	host, status := test.NewTestHost(thinMessagesResponseCaptureConfig(t))
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName("test-route-messages"))
	require.NoError(t, host.SetRequestId("request-messages-1"))

	action := host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"},
		{":path", "/v1/messages"},
		{":method", "POST"},
		{"content-type", "application/json"},
		{"x-mse-tenant", "tenant-a"},
		{"x-mse-consumer", "consumer-a"},
		{"x-mse-session", "session-a"},
	})
	require.Equal(t, types.HeaderStopIteration, action)

	action = host.CallOnHttpRequestBody(thinMessagesResponseCaptureRequestBody())
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

func thinMessagesResponseCaptureRequestBody() []byte {
	return []byte(`{
		"model": "claude-sonnet",
		"messages": [{
			"role": "user",
			"content": [{"type":"text","text":"summarize launch plan"}]
		}],
		"stream": false
	}`)
}

func expectedThinMessagesResponseCaptureRequestDigest(t *testing.T) string {
	t.Helper()
	adapter := protocol.MessagesAdapter{}
	digest, err := adapter.BuildCacheDigest(protocol.RequestParseInput{
		Method: "POST",
		Path:   "/v1/messages",
		Body:   thinMessagesResponseCaptureRequestBody(),
	})
	require.NoError(t, err)
	require.NotEmpty(t, digest.Digest)
	require.NotContains(t, digest.Digest, "summarize launch plan")
	return digest.Digest
}

func thinMessagesResponseCaptureConfig(t *testing.T) json.RawMessage {
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
		"route_policy": map[string]interface{}{
			"enable_redis_lookup":   true,
			"enable_console_lookup": false,
			"enable_replay":         true,
			"enabled_path_suffixes": []string{"/v1/messages"},
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

func TestThinResponsesResponseCapture(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("non-streaming upstream response emits Responses output usage and finish reason", func(t *testing.T) {
			host := startThinResponsesResponseCaptureRequest(t, false)
			defer host.Reset()

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{
				"id": "resp-cache-event",
				"object": "response",
				"model": "gpt-4.1",
				"output_text": "Responses cache answer",
				"output": [{
					"id":"msg_1",
					"type":"message",
					"role":"assistant",
					"content":[{"type":"output_text","text":"Responses cache answer"}]
				}],
				"status": "completed",
				"usage": {"input_tokens": 13, "output_tokens": 5, "total_tokens": 18}
			}`))
			require.Equal(t, types.ActionContinue, action)

			event := requireThinResponseCaptureEvent(t, host)
			require.Equal(t, "tenant-a", event["tenant"])
			require.Equal(t, "consumer-a", event["consumer"])
			require.Equal(t, "session-a", event["session_id"])
			require.Equal(t, "test-route-responses", event["route"])
			require.Equal(t, "gpt-4.1", event["model"])
			require.Equal(t, "request-responses-1", event["request_id"])
			require.Equal(t, "/v1/responses", event["request_path"])
			require.Equal(t, "responses", event["protocol"])
			require.Equal(t, "consumer", event["cache_scope"])
			require.Equal(t, "policy-v1", event["cache_policy_version"])
			require.Equal(t, expectedThinResponsesResponseCaptureRequestDigest(t, false), event["request_digest"])
			require.Equal(t, "Responses cache answer", event["assistant_content"])
			require.Equal(t, "completed", event["finish_reason"])
			require.EqualValues(t, 200, event["status_code"])
			require.Equal(t, false, event["is_stream"])
			require.Equal(t, false, event["contains_tool_calls"])
			require.Equal(t, false, event["no_store"])
			require.Equal(t, false, event["sensitive"])
			requireThinResponsesResponseCaptureUsage(t, event, 13, 5, 18)
			requireThinResponseNoLegacyCacheSet(t, host)
		})

		t.Run("streaming upstream response emits Responses deltas usage and finish reason", func(t *testing.T) {
			host := startThinResponsesResponseCaptureRequest(t, true)
			defer host.Reset()

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})
			callThinResponseStreamChunks(t, host,
				"event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp-stream-cache-event\",\"object\":\"response\",\"model\":\"gpt-4.1\",\"status\":\"in_progress\"}}\n\n",
				"event: response.output_item.added\ndata: {\"type\":\"response.output_item.added\",\"output_index\":0,\"item\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[]}}\n\n",
				"event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"item_id\":\"msg_1\",\"output_index\":0,\"content_index\":0,\"delta\":\"Responses \"}\n\n",
				"event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"item_id\":\"msg_1\",\"output_index\":0,\"content_index\":0,\"delta\":\"stream answer\"}\n\n",
				"event: response.output_text.done\ndata: {\"type\":\"response.output_text.done\",\"item_id\":\"msg_1\",\"output_index\":0,\"content_index\":0,\"text\":\"Responses stream answer\"}\n\n",
				"event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp-stream-cache-event\",\"status\":\"completed\",\"usage\":{\"input_tokens\":11,\"output_tokens\":4,\"total_tokens\":15}}}\n\n",
			)

			event := requireThinResponseCaptureEvent(t, host)
			require.Equal(t, "tenant-a", event["tenant"])
			require.Equal(t, "consumer-a", event["consumer"])
			require.Equal(t, "session-a", event["session_id"])
			require.Equal(t, "test-route-responses", event["route"])
			require.Equal(t, "gpt-4.1", event["model"])
			require.Equal(t, "request-responses-1", event["request_id"])
			require.Equal(t, "/v1/responses", event["request_path"])
			require.Equal(t, "responses", event["protocol"])
			require.Equal(t, "consumer", event["cache_scope"])
			require.Equal(t, "policy-v1", event["cache_policy_version"])
			require.Equal(t, expectedThinResponsesResponseCaptureRequestDigest(t, true), event["request_digest"])
			require.Equal(t, "Responses stream answer", event["assistant_content"])
			require.Equal(t, "completed", event["finish_reason"])
			require.EqualValues(t, 200, event["status_code"])
			require.Equal(t, true, event["is_stream"])
			require.Equal(t, false, event["contains_tool_calls"])
			require.Equal(t, false, event["no_store"])
			require.Equal(t, false, event["sensitive"])
			requireThinResponsesResponseCaptureUsage(t, event, 11, 4, 15)
			requireThinResponseNoLegacyCacheSet(t, host)
		})
	})
}

func startThinResponsesResponseCaptureRequest(t *testing.T, stream bool) test.TestHost {
	t.Helper()
	host, status := test.NewTestHost(thinResponsesResponseCaptureConfig(t))
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName("test-route-responses"))
	require.NoError(t, host.SetRequestId("request-responses-1"))

	action := host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"},
		{":path", "/v1/responses"},
		{":method", "POST"},
		{"content-type", "application/json"},
		{"x-mse-tenant", "tenant-a"},
		{"x-mse-consumer", "consumer-a"},
		{"x-mse-session", "session-a"},
	})
	require.Equal(t, types.HeaderStopIteration, action)

	action = host.CallOnHttpRequestBody(thinResponsesResponseCaptureRequestBody(stream))
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

func thinResponsesResponseCaptureRequestBody(stream bool) []byte {
	streamLiteral := "false"
	if stream {
		streamLiteral = "true"
	}
	return []byte(fmt.Sprintf(`{
		"model": "gpt-4.1",
		"instructions": "Use project context.",
		"input": [{"role":"user","content":[{"type":"input_text","text":"draft reply"}]}],
		"temperature": 0.1,
		"stream": %s
	}`, streamLiteral))
}

func expectedThinResponsesResponseCaptureRequestDigest(t *testing.T, stream bool) string {
	t.Helper()
	adapter := protocol.ResponsesAdapter{}
	digest, err := adapter.BuildCacheDigest(protocol.RequestParseInput{
		Method: "POST",
		Path:   "/v1/responses",
		Body:   thinResponsesResponseCaptureRequestBody(stream),
	})
	require.NoError(t, err)
	require.NotEmpty(t, digest.Digest)
	require.NotContains(t, digest.Digest, "draft reply")
	return digest.Digest
}

func thinResponsesResponseCaptureConfig(t *testing.T) json.RawMessage {
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
		"route_policy": map[string]interface{}{
			"enable_redis_lookup":   true,
			"enable_console_lookup": false,
			"enable_replay":         true,
			"enabled_path_suffixes": []string{"/v1/responses"},
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

func callThinResponseStreamChunks(t *testing.T, host test.TestHost, chunks ...string) {
	t.Helper()
	for i, chunk := range chunks {
		action := host.CallOnHttpStreamingResponseBody([]byte(chunk), i == len(chunks)-1)
		require.Equal(t, types.ActionContinue, action)
	}
}

func requireThinResponseCaptureEvent(t *testing.T, host test.TestHost) map[string]interface{} {
	t.Helper()
	event, ok := thinResponseCaptureEvent(t, host)
	if !ok {
		require.Failf(t, "missing thin CacheEvent XADD", "redis calls=%s", thinResponseCaptureCallSummary(t, host))
	}
	requireThinResponseCapturedMaterializedLookup(t, host)
	return event
}

func requireThinResponseCaptureGate(t *testing.T, host test.TestHost, statusCode int, markers ...string) {
	t.Helper()
	requireThinResponseNoLegacyCacheSet(t, host)
	event := requireThinResponseCaptureEvent(t, host)
	if _, exists := event["assistant_content"]; exists {
		require.Failf(t, "gated response emitted cacheable assistant content", "event=%s", thinResponseCaptureEventSnippet(t, event))
	}
	require.EqualValuesf(t, statusCode, event["status_code"], "event should record gated response status; event=%s", thinResponseCaptureEventSnippet(t, event))
	for _, marker := range markers {
		require.Equalf(t, true, event[marker], "event should mark %s gate; event=%s", marker, thinResponseCaptureEventSnippet(t, event))
	}
}

func requireThinResponseNoLegacyCacheSet(t *testing.T, host test.TestHost) {
	t.Helper()
	for _, call := range host.GetRedisCalloutAttributes() {
		cmd, ok := thinResponseCaptureCommand(t, call.Query)
		if !ok || len(cmd) < 3 {
			continue
		}
		if strings.EqualFold(cmd[0], "set") && strings.HasPrefix(cmd[1], "higress-ai-cache:") {
			require.Failf(t, "thin response capture emitted legacy cache SET", "command=%s", thinResponseCaptureCommandSummary(cmd))
		}
	}
}

func requireThinResponseCapturedMaterializedLookup(t *testing.T, host test.TestHost) {
	t.Helper()
	capture, ok := host.(interface {
		thinMaterializedLookup() (bool, string)
	})
	if !ok {
		require.Fail(t, "CacheEvent assertion requires host with captured materialized lookup metadata")
	}
	lookupWasMaterialized, lookupSummary := capture.thinMaterializedLookup()
	require.Truef(t, lookupWasMaterialized, "missing thin materialized Redis lookup before upstream capture; redis calls=%s", lookupSummary)
}

func requireThinResponseMaterializedLookup(t *testing.T, host test.TestHost) {
	t.Helper()
	if thinResponseHasMaterializedLookup(t, host) {
		return
	}
	require.Failf(t, "missing thin materialized Redis lookup", "redis calls=%s", thinResponseCaptureCallSummary(t, host))
}

func thinResponseHasMaterializedLookup(t *testing.T, host test.TestHost) bool {
	t.Helper()
	for _, call := range host.GetRedisCalloutAttributes() {
		cmd, ok := thinResponseCaptureCommand(t, call.Query)
		if !ok || len(cmd) < 2 {
			continue
		}
		if strings.EqualFold(cmd[0], "get") && strings.HasPrefix(cmd[1], "cache:materialized:") {
			return true
		}
	}
	return false
}

func requireThinResponseCaptureUsage(t *testing.T, event map[string]interface{}, promptTokens, completionTokens, totalTokens int) {
	t.Helper()
	usage, ok := event["usage"].(map[string]interface{})
	require.Truef(t, ok, "event usage must be an object; event=%s", thinResponseCaptureEventSnippet(t, event))
	require.EqualValues(t, promptTokens, usage["prompt_tokens"])
	require.EqualValues(t, completionTokens, usage["completion_tokens"])
	require.EqualValues(t, totalTokens, usage["total_tokens"])
}

func requireThinMessagesResponseCaptureUsage(t *testing.T, event map[string]interface{}, inputTokens, outputTokens int) {
	t.Helper()
	usage, ok := event["usage"].(map[string]interface{})
	require.Truef(t, ok, "event usage must be an object; event=%s", thinResponseCaptureEventSnippet(t, event))
	require.EqualValues(t, inputTokens, usage["input_tokens"])
	require.EqualValues(t, outputTokens, usage["output_tokens"])
	require.NotContains(t, usage, "total_tokens", "the original Messages response did not report a total")
}

func requireThinResponsesResponseCaptureUsage(t *testing.T, event map[string]interface{}, inputTokens, outputTokens, totalTokens int) {
	t.Helper()
	usage, ok := event["usage"].(map[string]interface{})
	require.Truef(t, ok, "event usage must be an object; event=%s", thinResponseCaptureEventSnippet(t, event))
	require.EqualValues(t, inputTokens, usage["input_tokens"])
	require.EqualValues(t, outputTokens, usage["output_tokens"])
	require.EqualValues(t, totalTokens, usage["total_tokens"])
}

func thinResponseCaptureEvent(t *testing.T, host test.TestHost) (map[string]interface{}, bool) {
	t.Helper()
	var events []map[string]interface{}
	for _, call := range host.GetRedisCalloutAttributes() {
		cmd, ok := thinResponseCaptureCommand(t, call.Query)
		if !ok || len(cmd) < 5 {
			continue
		}
		if !strings.EqualFold(cmd[0], "xadd") || cmd[1] != thinResponseCaptureStreamName {
			continue
		}
		fieldStart := thinResponseCaptureXADDFieldStart(t, cmd)
		require.Equalf(t, 0, (len(cmd)-fieldStart)%2, "cache event XADD has incomplete field/value pair; command=%s", thinResponseCaptureCommandSummary(cmd))
		eventFound := false
		for i := fieldStart; i+1 < len(cmd); i += 2 {
			if cmd[i] != thinResponseCaptureStreamField {
				continue
			}
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(cmd[i+1]), &event); err != nil {
				require.Failf(t, "CacheEvent field is not valid JSON", "err=%v event_bytes=%d", err, len(cmd[i+1]))
			}
			events = append(events, event)
			eventFound = true
			break
		}
		if !eventFound {
			require.Failf(t, "cache event XADD missing event field", "command=%s", thinResponseCaptureCommandSummary(cmd))
		}
	}
	if len(events) == 0 {
		return nil, false
	}
	require.Lenf(t, events, 1, "expected exactly one CacheEvent XADD; redis calls=%s", thinResponseCaptureCallSummary(t, host))
	return events[0], true
}

func thinResponseCaptureXADDFieldStart(t *testing.T, cmd []string) int {
	t.Helper()
	for i := 2; i < len(cmd); {
		switch strings.ToLower(cmd[i]) {
		case "nomkstream":
			i++
		case "maxlen", "minid":
			i++
			if i < len(cmd) && (cmd[i] == "=" || cmd[i] == "~") {
				i++
			}
			require.Lessf(t, i, len(cmd), "cache event XADD missing trim threshold; command=%s", thinResponseCaptureCommandSummary(cmd))
			i++
			if i < len(cmd) && strings.EqualFold(cmd[i], "limit") {
				i++
				require.Lessf(t, i, len(cmd), "cache event XADD missing LIMIT count; command=%s", thinResponseCaptureCommandSummary(cmd))
				i++
			}
		default:
			streamID := cmd[i]
			require.Truef(t, streamID == "*" || strings.Contains(streamID, "-"), "cache event XADD has invalid stream id %q; command=%s", streamID, thinResponseCaptureCommandSummary(cmd))
			return i + 1
		}
	}
	require.Failf(t, "cache event XADD missing stream id", "command=%s", thinResponseCaptureCommandSummary(cmd))
	return len(cmd)
}

func thinResponseCaptureCommand(t *testing.T, query []byte) ([]string, bool) {
	t.Helper()
	value, _, err := resp.NewReader(bytes.NewReader(query)).ReadValue()
	if err != nil {
		return nil, false
	}
	array := value.Array()
	if len(array) == 0 {
		return nil, false
	}
	cmd := make([]string, 0, len(array))
	for _, item := range array {
		cmd = append(cmd, item.String())
	}
	return cmd, true
}

func thinResponseCaptureCallSummary(t *testing.T, host test.TestHost) string {
	t.Helper()
	var parts []string
	for _, call := range host.GetRedisCalloutAttributes() {
		cmd, ok := thinResponseCaptureCommand(t, call.Query)
		if ok {
			parts = append(parts, fmt.Sprintf("%s %s", call.Upstream, thinResponseCaptureCommandSummary(cmd)))
			continue
		}
		parts = append(parts, fmt.Sprintf("%s <unparseable:%d>", call.Upstream, len(call.Query)))
	}
	return thinResponseCaptureSnippet(strings.Join(parts, " | "), 600)
}

func thinResponseCaptureCommandSummary(cmd []string) string {
	if len(cmd) == 0 {
		return "<empty>"
	}
	switch strings.ToLower(cmd[0]) {
	case "get":
		return fmt.Sprintf("get %s", thinResponseCaptureArgSummary(cmd, 1, "key"))
	case "set":
		return fmt.Sprintf("set %s %s", thinResponseCaptureArgSummary(cmd, 1, "key"), thinResponseCaptureArgSummary(cmd, 2, "value"))
	case "xadd":
		parts := []string{"xadd", thinResponseCaptureArgSummary(cmd, 1, "stream")}
		for i := 2; i < len(cmd); i++ {
			parts = append(parts, thinResponseCaptureArgSummary(cmd, i, fmt.Sprintf("arg%d", i)))
		}
		return strings.Join(parts, " ")
	default:
		parts := []string{cmd[0]}
		for i := 1; i < len(cmd); i++ {
			parts = append(parts, thinResponseCaptureArgSummary(cmd, i, fmt.Sprintf("arg%d", i)))
		}
		return strings.Join(parts, " ")
	}
}

func thinResponseCaptureArgSummary(cmd []string, index int, label string) string {
	if index >= len(cmd) {
		return "<missing>"
	}
	value := cmd[index]
	switch label {
	case "stream":
		return value
	case "arg2":
		if strings.EqualFold(cmd[0], "xadd") && (value == "*" || strings.Contains(value, "-")) {
			return value
		}
	}
	return fmt.Sprintf("<%s:%d>", label, len(value))
}

func thinResponseCaptureEventSnippet(t *testing.T, event map[string]interface{}) string {
	t.Helper()
	data, err := json.Marshal(event)
	require.NoError(t, err)
	return thinResponseCaptureSnippet(string(data), 600)
}

func thinResponseCaptureSnippet(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}
