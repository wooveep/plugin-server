package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/resp"
)

const (
	memoryEventStreamName      = "memory:events"
	memoryEventStreamField     = "event"
	memoryNoStoreHeader        = "x-higress-ai-memory-no-store"
	memoryResponseCaptureModel = "qwen-turbo"
)

func TestMemoryNonStreamingResponseCapture(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("eligible response captures assistant content usage finish reason status and tool flag", func(t *testing.T) {
			host := startMemoryResponseCaptureRequest(t, true)

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{
				"id": "chatcmpl-memory-response-capture",
				"object": "chat.completion",
				"model": "qwen-turbo",
				"choices": [{
					"index": 0,
					"message": {"role": "assistant", "content": "captured memory answer"},
					"finish_reason": "stop"
				}],
				"usage": {"prompt_tokens": 9, "completion_tokens": 4, "total_tokens": 13}
			}`))
			require.Equal(t, types.ActionContinue, action)

			event := requireMemoryResponseCaptureEvent(t, host)
			requireMemoryResponseSafeFacts(t, event, 200)
			require.Equal(t, "current question", event["user_content"])
			require.Equal(t, "captured memory answer", event["assistant_content"])
			require.Equal(t, "stop", event["finish_reason"])
			require.Equal(t, false, event["is_stream"])
			require.Equal(t, false, event["contains_tool_calls"])
			requireMemoryResponseCaptureUsage(t, event, 9, 4, 13)
		})

		t.Run("tool-call response marks contains tool calls", func(t *testing.T) {
			cases := []struct {
				name            string
				body            string
				wantFinish      string
				wantPrompt      int
				wantCompletion  int
				wantTotalTokens int
			}{
				{
					name: "canonical message tool calls",
					body: `{
							"id": "chatcmpl-memory-canonical-tool-call",
							"object": "chat.completion",
							"model": "qwen-turbo",
							"choices": [{
								"index": 0,
								"message": {
									"role": "assistant",
									"content": "canonical tool call response preface",
									"tool_calls": [{"id":"call-1","type":"function","function":{"name":"lookup_weather","arguments":"{}"}}]
								},
								"finish_reason": "stop"
							}],
							"usage": {"prompt_tokens": 8, "completion_tokens": 3, "total_tokens": 11}
						}`,
					wantFinish:      "stop",
					wantPrompt:      8,
					wantCompletion:  3,
					wantTotalTokens: 11,
				},
				{
					name: "legacy function call",
					body: `{
							"id": "chatcmpl-memory-legacy-function-call",
							"object": "chat.completion",
							"model": "qwen-turbo",
							"choices": [{
								"index": 0,
								"message": {
									"role": "assistant",
									"content": "legacy function call response preface",
									"function_call": {"name":"lookup_weather","arguments":"{}"}
								},
								"finish_reason": "stop"
							}],
							"usage": {"prompt_tokens": 9, "completion_tokens": 3, "total_tokens": 12}
						}`,
					wantFinish:      "stop",
					wantPrompt:      9,
					wantCompletion:  3,
					wantTotalTokens: 12,
				},
				{
					name: "configured tool call path",
					body: `{
							"id": "chatcmpl-memory-custom-tool-call-path",
							"object": "chat.completion",
							"model": "qwen-turbo",
							"choices": [{
								"index": 0,
								"message": {
									"role": "assistant",
									"content": "custom tool call path response preface",
									"custom_tool_calls": [{"id":"custom-call-1","name":"lookup_weather"}]
								},
								"finish_reason": "stop"
							}],
							"usage": {"prompt_tokens": 10, "completion_tokens": 3, "total_tokens": 13}
						}`,
					wantFinish:      "stop",
					wantPrompt:      10,
					wantCompletion:  3,
					wantTotalTokens: 13,
				},
				{
					name: "finish reason only",
					body: `{
							"id": "chatcmpl-memory-finish-reason-tool-call",
							"object": "chat.completion",
							"model": "qwen-turbo",
							"choices": [{
								"index": 0,
								"message": {"role": "assistant", "content": "finish reason indicates tool use"},
								"finish_reason": "tool_calls"
							}],
							"usage": {"prompt_tokens": 11, "completion_tokens": 3, "total_tokens": 14}
						}`,
					wantFinish:      "tool_calls",
					wantPrompt:      11,
					wantCompletion:  3,
					wantTotalTokens: 14,
				},
			}

			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					host := startMemoryResponseCaptureRequest(t, true)

					host.CallOnHttpResponseHeaders([][2]string{
						{":status", "200"},
						{"content-type", "application/json"},
					})
					action := host.CallOnHttpResponseBody([]byte(tc.body))
					require.Equal(t, types.ActionContinue, action)

					event := requireMemoryResponseCaptureEvent(t, host)
					requireMemoryResponseSafeFacts(t, event, 200)
					require.Equal(t, true, event["contains_tool_calls"])
					require.Equal(t, tc.wantFinish, event["finish_reason"])
					requireMemoryResponseCaptureUsage(t, event, tc.wantPrompt, tc.wantCompletion, tc.wantTotalTokens)
				})
			}
		})

		t.Run("request no-store omits raw user and assistant content but emits safe facts", func(t *testing.T) {
			host := startMemoryResponseCaptureRequest(t, true, [2]string{memoryNoStoreHeader, "true"})

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{
				"id": "chatcmpl-memory-no-store",
				"object": "chat.completion",
				"model": "qwen-turbo",
				"choices": [{
					"index": 0,
					"message": {"role": "assistant", "content": "no-store assistant content must be omitted"},
					"finish_reason": "stop"
				}],
				"usage": {"prompt_tokens": 7, "completion_tokens": 4, "total_tokens": 11}
			}`))
			require.Equal(t, types.ActionContinue, action)

			event := requireMemoryResponseCaptureEvent(t, host)
			requireMemoryResponseSafeFacts(t, event, 200)
			require.Equal(t, true, event["no_store"])
			require.Equal(t, false, event["is_stream"])
			require.Equal(t, "stop", event["finish_reason"])
			require.Equal(t, false, event["contains_tool_calls"])
			requireMemoryResponseCaptureUsage(t, event, 7, 4, 11)
			requireMemoryEventOmitsField(t, event, "user_content")
			requireMemoryEventOmitsField(t, event, "assistant_content")
			requireMemoryEventJSONExcludes(t, event, "current question", "no-store assistant content must be omitted")
		})

		t.Run("disabled capture omits raw user and assistant content but emits safe facts", func(t *testing.T) {
			host := startMemoryResponseCaptureRequest(t, false)

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{
				"id": "chatcmpl-memory-capture-disabled",
				"object": "chat.completion",
				"model": "qwen-turbo",
				"choices": [{
					"index": 0,
					"message": {"role": "assistant", "content": "disabled capture assistant content must be omitted"},
					"finish_reason": "stop"
				}],
				"usage": {"prompt_tokens": 6, "completion_tokens": 4, "total_tokens": 10}
			}`))
			require.Equal(t, types.ActionContinue, action)

			event := requireMemoryResponseCaptureEvent(t, host)
			requireMemoryResponseSafeFacts(t, event, 200)
			require.Equal(t, false, event["no_store"])
			require.Equal(t, false, event["is_stream"])
			require.Equal(t, "stop", event["finish_reason"])
			require.Equal(t, false, event["contains_tool_calls"])
			requireMemoryResponseCaptureUsage(t, event, 6, 4, 10)
			requireMemoryEventOmitsField(t, event, "user_content")
			requireMemoryEventOmitsField(t, event, "assistant_content")
			requireMemoryEventJSONExcludes(t, event, "current question", "disabled capture assistant content must be omitted")
		})

		t.Run("parse failure omits raw content but preserves upstream response", func(t *testing.T) {
			host := startMemoryResponseCaptureRequest(t, true)
			original := []byte(`{not-json`)

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody(original)
			require.Equal(t, types.ActionContinue, action)
			require.Equal(t, string(original), string(host.GetResponseBody()))

			event := requireMemoryResponseCaptureEvent(t, host)
			requireMemoryResponseSafeFacts(t, event, 200)
			require.Equal(t, false, event["is_stream"])
			require.Equal(t, false, event["contains_tool_calls"])
			requireMemoryEventOmitsField(t, event, "user_content")
			requireMemoryEventOmitsField(t, event, "assistant_content")
			requireMemoryEventJSONExcludes(t, event, "current question")
		})

		t.Run("messages response emits memory event", func(t *testing.T) {
			host := startMemoryProtocolResponseCaptureRequest(t, "/v1/messages", []byte(`{
				"model": "deepseek-v4-flash",
				"messages": [{"role": "user", "content": [{"type":"text","text":"messages memory question"}]}],
				"stream": false
			}`))

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{
				"id": "msg-memory-response-capture",
				"type": "message",
				"role": "assistant",
				"content": [{"type":"text","text":"Messages memory answer"}],
				"stop_reason": "end_turn",
				"usage": {"input_tokens": 14, "output_tokens": 7}
			}`))
			require.Equal(t, types.ActionContinue, action)

			event := requireMemoryResponseCaptureEvent(t, host)
			requireMemoryProtocolResponseSafeFacts(t, event, "/v1/messages", "deepseek-v4-flash")
			require.Equal(t, "messages memory question", event["user_content"])
			require.Equal(t, "Messages memory answer", event["assistant_content"])
			require.Equal(t, "end_turn", event["finish_reason"])
			require.Equal(t, false, event["is_stream"])
			requireMemoryResponseCaptureUsage(t, event, 14, 7, 21)
		})

		t.Run("responses response emits memory event", func(t *testing.T) {
			host := startMemoryProtocolResponseCaptureRequest(t, "/v1/responses", []byte(`{
				"model": "deepseek-v4-flash",
				"input": [{"role":"user","content":[{"type":"input_text","text":"responses memory question"}]}],
				"stream": false
			}`))

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpResponseBody([]byte(`{
				"id": "resp_memory_response_capture",
				"object": "response",
				"model": "deepseek-v4-flash",
				"output_text": "Responses memory answer",
				"status": "completed",
				"usage": {"input_tokens": 13, "output_tokens": 5, "total_tokens": 18}
			}`))
			require.Equal(t, types.ActionContinue, action)

			event := requireMemoryResponseCaptureEvent(t, host)
			requireMemoryProtocolResponseSafeFacts(t, event, "/v1/responses", "deepseek-v4-flash")
			require.Equal(t, "responses memory question", event["user_content"])
			require.Equal(t, "Responses memory answer", event["assistant_content"])
			require.Equal(t, "completed", event["finish_reason"])
			require.Equal(t, false, event["is_stream"])
			requireMemoryResponseCaptureUsage(t, event, 13, 5, 18)
		})
	})
}

func memoryResponseCaptureConfig(t *testing.T, captureResponse bool) json.RawMessage {
	t.Helper()
	return mustMemoryConfig(t, map[string]interface{}{
		"redis_stream": map[string]interface{}{
			"service_name": "redis.memory.svc.cluster.local",
			"stream":       memoryEventStreamName,
			"field":        memoryEventStreamField,
		},
		"recent_cache": map[string]interface{}{
			"service_name": "redis.recent.svc.cluster.local",
		},
		"console_internal": map[string]interface{}{
			"service_name":  memoryConsoleService,
			"service_port":  8080,
			"assemble_path": memoryConsolePath,
			"timeout_ms":    120,
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
				"memory_mode":         "recent-only",
				"recent_window_turns": 6,
				"capture_response":    captureResponse,
				"no_store_header":     memoryNoStoreHeader,
				"response_value_from": "choices.0.message.content",
				"tool_calls_from":     []string{"choices.0.message.custom_tool_calls"},
				"policy_version":      "7",
			},
		},
	})
}

func startMemoryResponseCaptureRequest(t *testing.T, captureResponse bool, extraHeaders ...[2]string) test.TestHost {
	t.Helper()
	host, status := newMemoryConfigTestHost(memoryResponseCaptureConfig(t, captureResponse))
	t.Cleanup(host.Reset)
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName("memory-route"))
	require.NoError(t, host.SetRequestId("property-request-id"))

	headers := applyMemoryRequestGatingHeaderOverrides(memoryConsoleAssembleHeaders(), [][2]string{
		{"x-request-id", "request-response-capture-1"},
	})
	headers = append(headers, extraHeaders...)
	action := host.CallOnHttpRequestHeaders(headers)
	require.Equal(t, types.HeaderStopIteration, action)

	action = host.CallOnHttpRequestBody(memoryConsoleAssembleRequestBody())
	require.Equal(t, types.ActionPause, action)
	requireMemoryRecentRedisLookup(t, host)

	host.CallOnRedisCall(0, test.CreateRedisRespNull())
	require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
	return host
}

func startMemoryProtocolResponseCaptureRequest(t *testing.T, path string, body []byte) test.TestHost {
	t.Helper()
	host, status := newMemoryConfigTestHost(memoryProtocolResponseCaptureConfig(t, path))
	t.Cleanup(host.Reset)
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName("memory-route"))
	require.NoError(t, host.SetRequestId("property-request-id"))

	headers := applyMemoryRequestGatingHeaderOverrides(memoryConsoleAssembleHeaders(), [][2]string{
		{":path", path},
		{"x-request-id", "request-response-capture-1"},
	})
	action := host.CallOnHttpRequestHeaders(headers)
	require.Equal(t, types.HeaderStopIteration, action)

	action = host.CallOnHttpRequestBody(body)
	require.Equal(t, types.ActionPause, action)
	requireMemoryRecentRedisLookup(t, host)

	host.CallOnRedisCall(0, test.CreateRedisRespNull())
	require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
	return host
}

func memoryProtocolResponseCaptureConfig(t *testing.T, path string) json.RawMessage {
	t.Helper()
	return mustMemoryConfig(t, map[string]interface{}{
		"redis_stream": map[string]interface{}{
			"service_name": "redis.memory.svc.cluster.local",
			"stream":       memoryEventStreamName,
			"field":        memoryEventStreamField,
		},
		"recent_cache": map[string]interface{}{
			"service_name": "redis.recent.svc.cluster.local",
		},
		"console_internal": map[string]interface{}{
			"service_name":  memoryConsoleService,
			"service_port":  8080,
			"assemble_path": memoryConsolePath,
			"timeout_ms":    120,
		},
		"tenant_header":        "x-mse-tenant",
		"consumer_header":      "x-mse-consumer",
		"session_header":       "x-mse-session",
		"request_id_header":    "x-request-id",
		"enable_path_suffixes": []string{path},
		"fail_policy":          "open",
		"_rules_": []map[string]interface{}{
			{
				"_match_route_":       []string{"memory-route"},
				"memory_mode":         "recent-only",
				"recent_window_turns": 6,
				"capture_response":    true,
				"no_store_header":     memoryNoStoreHeader,
				"policy_version":      "7",
			},
		},
	})
}

func requireMemoryResponseCaptureEvent(t *testing.T, host test.TestHost) map[string]interface{} {
	t.Helper()
	var events []map[string]interface{}
	for _, call := range host.GetRedisCalloutAttributes() {
		cmd, ok := memoryResponseCaptureCommand(t, call.Query)
		if !ok || len(cmd) < 5 {
			continue
		}
		if !strings.EqualFold(cmd[0], "xadd") || cmd[1] != memoryEventStreamName {
			continue
		}
		for i := 2; i+1 < len(cmd); i++ {
			if cmd[i] != memoryEventStreamField {
				continue
			}
			var event map[string]interface{}
			require.NoErrorf(t, json.Unmarshal([]byte(cmd[i+1]), &event), "MemoryEvent must be JSON; command=%s", memoryResponseCaptureCommandSummary(cmd))
			events = append(events, event)
			break
		}
	}
	require.Lenf(t, events, 1, "expected one MemoryEvent XADD; redis calls=%s", memoryResponseCaptureCallSummary(t, host))
	return events[0]
}

func requireMemoryResponseSafeFacts(t *testing.T, event map[string]interface{}, statusCode int) {
	t.Helper()
	requireMemoryResponseSafeFactsFor(t, event, statusCode, "/v1/chat/completions", memoryResponseCaptureModel)
}

func requireMemoryResponseSafeFactsFor(t *testing.T, event map[string]interface{}, statusCode int, path string, model string) {
	t.Helper()
	require.EqualValues(t, 1, event["schema_version"])
	require.Equal(t, "tenant-a", event["tenant"])
	require.Equal(t, "consumer-a", event["consumer"])
	require.Equal(t, "session-a", event["session_id"])
	require.Equal(t, "request-response-capture-1", event["request_id"])
	require.Equal(t, path, event["request_path"])
	require.Equal(t, map[string]interface{}{"name": "memory-route"}, event["route"])
	require.Equal(t, map[string]interface{}{"name": model}, event["model"])
	require.Equal(t, "recent-only", event["memory_mode"])
	require.Equal(t, "7", event["policy_version"])
	require.EqualValues(t, statusCode, event["status_code"])
	require.NotEmpty(t, event["event_id"])
	require.NotEmpty(t, event["idempotency_key"])
	require.NotEmpty(t, event["request_digest"])
	require.NotEmpty(t, event["started_at_ms"])
	require.NotEmpty(t, event["ended_at_ms"])
	require.NotEmpty(t, event["plugin_version"])
}

func requireMemoryProtocolResponseSafeFacts(t *testing.T, event map[string]interface{}, path string, model string) {
	t.Helper()
	requireMemoryResponseSafeFactsFor(t, event, 200, path, model)
}

func requireMemoryResponseCaptureUsage(t *testing.T, event map[string]interface{}, input, output, total int) {
	t.Helper()
	usage, ok := event["usage"].(map[string]interface{})
	require.Truef(t, ok, "MemoryEvent usage must be an object; event=%s", memoryEventSnippet(t, event))
	require.Equal(t, "token", usage["unit"])
	require.EqualValues(t, input, usage["input"])
	require.EqualValues(t, output, usage["output"])
	require.EqualValues(t, total, usage["total"])
}

func requireMemoryEventOmitsField(t *testing.T, event map[string]interface{}, field string) {
	t.Helper()
	_, exists := event[field]
	require.Falsef(t, exists, "MemoryEvent included %s; event=%s", field, memoryEventSnippet(t, event))
}

func requireMemoryEventJSONExcludes(t *testing.T, event map[string]interface{}, forbidden ...string) {
	t.Helper()
	body, err := json.Marshal(event)
	require.NoError(t, err)
	eventJSON := string(body)
	for _, value := range forbidden {
		require.NotContainsf(t, eventJSON, value, "MemoryEvent leaked forbidden content; event=%s", memoryEventSnippet(t, event))
	}
}

func memoryResponseCaptureCommand(t *testing.T, query []byte) ([]string, bool) {
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

func memoryResponseCaptureCallSummary(t *testing.T, host test.TestHost) string {
	t.Helper()
	var parts []string
	for _, call := range host.GetRedisCalloutAttributes() {
		cmd, ok := memoryResponseCaptureCommand(t, call.Query)
		if ok {
			parts = append(parts, fmt.Sprintf("%s %s", call.Upstream, memoryResponseCaptureCommandSummary(cmd)))
			continue
		}
		parts = append(parts, fmt.Sprintf("%s <unparseable:%d>", call.Upstream, len(call.Query)))
	}
	return strings.Join(parts, " | ")
}

func memoryResponseCaptureCommandSummary(cmd []string) string {
	if len(cmd) == 0 {
		return "<empty>"
	}
	limit := len(cmd)
	if limit > 6 {
		limit = 6
	}
	summary := append([]string(nil), cmd[:limit]...)
	for i := range summary {
		if len(summary[i]) > 96 {
			summary[i] = fmt.Sprintf("<arg:%d>", len(summary[i]))
		}
	}
	if len(cmd) > limit {
		summary = append(summary, "...")
	}
	return strings.Join(summary, " ")
}

func memoryEventSnippet(t *testing.T, event map[string]interface{}) string {
	t.Helper()
	body, err := json.Marshal(event)
	require.NoError(t, err)
	text := string(body)
	if len(text) > 600 {
		return text[:600] + "..."
	}
	return text
}
