package main

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestMemoryStreamingResponseCapture(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("split chunks role-only chunks final content buffer and done capture content usage and finish reason", func(t *testing.T) {
			host := startMemoryStreamingResponseCaptureRequest(t)

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})
			callMemoryStreamingResponseChunks(t, host,
				memoryStreamingSSEFrame("\r\n", `{"id":"chatcmpl-memory-stream","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`),
				memoryStreamingSSEFrame("\r\n", `{"id":"chatcmpl-memory-stream","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"content":"captured "},"finish_reason":null}]}`),
				`data: {"id":"chatcmpl-memory-stream","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"content":"split `,
				`chunk "},"finish_reason":null}]}`+"\r\n\r\n",
				memoryStreamingSSEFrame("\r\n", `{"id":"chatcmpl-memory-stream","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"content":"final content"},"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":5,"total_tokens":17}}`)+
					memoryStreamingSSEFrame("\r\n", `[DONE]`),
			)

			event := requireMemoryResponseCaptureEvent(t, host)
			requireMemoryResponseSafeFacts(t, event, 200)
			require.Equal(t, "current question", event["user_content"])
			require.Equal(t, "captured split chunk final content", event["assistant_content"])
			require.Equal(t, "stop", event["finish_reason"])
			require.Equal(t, true, event["is_stream"])
			require.Equal(t, false, event["contains_tool_calls"])
			requireMemoryResponseCaptureUsage(t, event, 12, 5, 17)
		})

		t.Run("line ending variants are accepted", func(t *testing.T) {
			cases := []struct {
				name       string
				lineEnding string
				content    string
				usage      [3]int
			}{
				{name: "crlf", lineEnding: "\r\n", content: "crlf line ending answer", usage: [3]int{8, 4, 12}},
				{name: "cr", lineEnding: "\r", content: "cr line ending answer", usage: [3]int{9, 4, 13}},
				{name: "lf", lineEnding: "\n", content: "lf line ending answer", usage: [3]int{10, 4, 14}},
			}

			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					host := startMemoryStreamingResponseCaptureRequest(t)

					host.CallOnHttpResponseHeaders([][2]string{
						{":status", "200"},
						{"content-type", "text/event-stream"},
					})
					callMemoryStreamingResponseChunks(t, host,
						memoryStreamingSSEFrame(tc.lineEnding, `{"id":"chatcmpl-memory-line-ending","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`),
						memoryStreamingSSEFrame(tc.lineEnding, `{"id":"chatcmpl-memory-line-ending","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"content":"`+tc.content+`"},"finish_reason":"stop"}],"usage":{"prompt_tokens":`+strconv.Itoa(tc.usage[0])+`,"completion_tokens":`+strconv.Itoa(tc.usage[1])+`,"total_tokens":`+strconv.Itoa(tc.usage[2])+`}}`),
						memoryStreamingSSEFrame(tc.lineEnding, `[DONE]`),
					)

					event := requireMemoryResponseCaptureEvent(t, host)
					requireMemoryResponseSafeFacts(t, event, 200)
					require.Equal(t, tc.content, event["assistant_content"])
					require.Equal(t, "stop", event["finish_reason"])
					require.Equal(t, true, event["is_stream"])
					require.Equal(t, false, event["contains_tool_calls"])
					requireMemoryResponseCaptureUsage(t, event, tc.usage[0], tc.usage[1], tc.usage[2])
				})
			}
		})

		t.Run("tool-call signals mark contains tool calls and preserve safe streaming facts", func(t *testing.T) {
			cases := []struct {
				name      string
				chunks    []string
				finish    string
				usage     [3]int
				wantRaw   string
				forbidden string
			}{
				{
					name: "canonical tool-call delta",
					chunks: []string{
						memoryStreamingSSEFrame("\n", `{"id":"chatcmpl-memory-stream-tool","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`),
						memoryStreamingSSEFrame("\n", `{"id":"chatcmpl-memory-stream-tool","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"content":"canonical tool call preface"},"finish_reason":null}]}`),
						memoryStreamingSSEFrame("\n", `{"id":"chatcmpl-memory-stream-tool","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call-1","type":"function","function":{"name":"lookup_weather","arguments":"{}"}}]},"finish_reason":null}]}`),
						memoryStreamingSSEFrame("\n", `{"id":"chatcmpl-memory-stream-tool","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10}}`) +
							memoryStreamingSSEFrame("\n", `[DONE]`),
					},
					finish:    "stop",
					usage:     [3]int{7, 3, 10},
					wantRaw:   "canonical tool call preface",
					forbidden: "lookup_weather",
				},
				{
					name: "configured content tool-call path",
					chunks: []string{
						memoryStreamingSSEFrame("\r\n", `{"id":"chatcmpl-memory-stream-content-tool","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`),
						memoryStreamingSSEFrame("\r\n", `{"id":"chatcmpl-memory-stream-content-tool","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"content":{"text":"configured path preface","tool_calls":[{"id":"content-call-1","type":"function","function":{"name":"lookup_memory","arguments":"{}"}}]}},"finish_reason":null}]}`),
						memoryStreamingSSEFrame("\r\n", `{"id":"chatcmpl-memory-stream-content-tool","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":8,"completion_tokens":3,"total_tokens":11}}`) +
							memoryStreamingSSEFrame("\r\n", `[DONE]`),
					},
					finish:    "stop",
					usage:     [3]int{8, 3, 11},
					forbidden: "lookup_memory",
				},
				{
					name: "finish reason only",
					chunks: []string{
						memoryStreamingSSEFrame("\n", `{"id":"chatcmpl-memory-stream-finish-tool","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`),
						memoryStreamingSSEFrame("\n", `{"id":"chatcmpl-memory-stream-finish-tool","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"content":"finish reason tool call preface"},"finish_reason":null}]}`),
						memoryStreamingSSEFrame("\n", `{"id":"chatcmpl-memory-stream-finish-tool","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":9,"completion_tokens":3,"total_tokens":12}}`) +
							memoryStreamingSSEFrame("\n", `[DONE]`),
					},
					finish:  "tool_calls",
					usage:   [3]int{9, 3, 12},
					wantRaw: "finish reason tool call preface",
				},
			}

			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					host := startMemoryStreamingResponseCaptureRequest(t)

					host.CallOnHttpResponseHeaders([][2]string{
						{":status", "200"},
						{"content-type", "text/event-stream"},
					})
					callMemoryStreamingResponseChunks(t, host, tc.chunks...)

					event := requireMemoryResponseCaptureEvent(t, host)
					requireMemoryResponseSafeFacts(t, event, 200)
					require.Equal(t, true, event["is_stream"])
					require.Equal(t, true, event["contains_tool_calls"])
					require.Equal(t, tc.finish, event["finish_reason"])
					if tc.wantRaw != "" {
						require.Equal(t, tc.wantRaw, event["assistant_content"])
					} else {
						requireMemoryEventOmitsField(t, event, "assistant_content")
					}
					if tc.forbidden != "" {
						requireMemoryEventJSONExcludes(t, event, tc.forbidden)
					}
					requireMemoryResponseCaptureUsage(t, event, tc.usage[0], tc.usage[1], tc.usage[2])
				})
			}
		})

		t.Run("messages protocol stream emits normalized memory event", func(t *testing.T) {
			host := startMemoryProtocolResponseCaptureRequest(t, "/v1/messages", []byte(`{
				"model": "deepseek-v4-flash",
				"messages": [{"role": "user", "content": [{"type":"text","text":"messages stream question"}]}],
				"stream": true
			}`))

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})
			callMemoryStreamingResponseChunks(t, host,
				memoryStreamingSSEEventFrame("\n", "message_start", `{"type":"message_start","message":{"usage":{"input_tokens":11}}}`),
				memoryStreamingSSEEventFrame("\n", "content_block_delta", `{"type":"content_block_delta","delta":{"type":"text_delta","text":"Messages stream answer"}}`),
				memoryStreamingSSEEventFrame("\n", "message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":6}}`),
			)

			event := requireMemoryResponseCaptureEvent(t, host)
			requireMemoryProtocolResponseSafeFacts(t, event, "/v1/messages", "deepseek-v4-flash")
			require.Equal(t, "messages stream question", event["user_content"])
			require.Equal(t, "Messages stream answer", event["assistant_content"])
			require.Equal(t, "end_turn", event["finish_reason"])
			require.Equal(t, true, event["is_stream"])
			require.Equal(t, false, event["contains_tool_calls"])
			requireMemoryResponseCaptureUsage(t, event, 11, 6, 17)
		})

		t.Run("responses protocol stream emits normalized memory event", func(t *testing.T) {
			host := startMemoryProtocolResponseCaptureRequest(t, "/v1/responses", []byte(`{
				"model": "deepseek-v4-flash",
				"input": [{"role":"user","content":[{"type":"input_text","text":"responses stream question"}]}],
				"stream": true
			}`))

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})
			callMemoryStreamingResponseChunks(t, host,
				memoryStreamingSSEEventFrame("\n", "response.output_text.delta", `{"type":"response.output_text.delta","delta":"Responses stream answer"}`),
				memoryStreamingSSEEventFrame("\n", "response.completed", `{"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":10,"output_tokens":4,"total_tokens":14}}}`),
			)

			event := requireMemoryResponseCaptureEvent(t, host)
			requireMemoryProtocolResponseSafeFacts(t, event, "/v1/responses", "deepseek-v4-flash")
			require.Equal(t, "responses stream question", event["user_content"])
			require.Equal(t, "Responses stream answer", event["assistant_content"])
			require.Equal(t, "completed", event["finish_reason"])
			require.Equal(t, true, event["is_stream"])
			require.Equal(t, false, event["contains_tool_calls"])
			requireMemoryResponseCaptureUsage(t, event, 10, 4, 14)
		})
	})
}

func memoryStreamingResponseCaptureConfig(t *testing.T) json.RawMessage {
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
				"capture_response":    true,
				"no_store_header":     memoryNoStoreHeader,
				"response_value_from": "choices.0.message.content",
				"stream_value_from":   "choices.0.delta.content",
				"tool_calls_from": []string{
					"choices.0.delta.tool_calls",
					"choices.0.delta.content.tool_calls",
				},
				"policy_version": "7",
			},
		},
	})
}

func startMemoryStreamingResponseCaptureRequest(t *testing.T, extraHeaders ...[2]string) test.TestHost {
	t.Helper()
	host, status := newMemoryConfigTestHost(memoryStreamingResponseCaptureConfig(t))
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

	action = host.CallOnHttpRequestBody(memoryStreamingResponseCaptureRequestBody())
	require.Equal(t, types.ActionPause, action)
	requireMemoryRecentRedisLookup(t, host)

	host.CallOnRedisCall(0, test.CreateRedisRespNull())
	require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
	return host
}

func memoryStreamingResponseCaptureRequestBody() []byte {
	return []byte(`{
		"model": "qwen-turbo",
		"messages": [
			{"role": "user", "content": "current question"}
		],
		"stream": true
	}`)
}

func callMemoryStreamingResponseChunks(t *testing.T, host test.TestHost, chunks ...string) {
	t.Helper()
	for i, chunk := range chunks {
		action := host.CallOnHttpStreamingResponseBody([]byte(chunk), i == len(chunks)-1)
		require.Equal(t, types.ActionContinue, action)
	}
}

func memoryStreamingSSEFrame(lineEnding, payload string) string {
	return "data: " + payload + lineEnding + lineEnding
}

func memoryStreamingSSEEventFrame(lineEnding, event string, payload string) string {
	return "event: " + event + lineEnding + "data: " + payload + lineEnding + lineEnding
}
