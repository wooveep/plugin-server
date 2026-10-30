package main

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestMemoryEventPayload(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("eligible response emits required and optional safe payload", func(t *testing.T) {
			requestBody := memoryEventRequestBody(t, "event user question", map[string]interface{}{
				"unpermitted_request_field": "request-extra-should-not-appear",
			})
			responseBody := []byte(`{
				"id": "chatcmpl-memory-event",
				"object": "chat.completion",
				"model": "qwen-turbo",
				"choices": [{
					"index": 0,
					"message": {"role": "assistant", "content": "event assistant answer"},
					"finish_reason": "stop"
				}],
				"usage": {"prompt_tokens": 9, "completion_tokens": 4, "total_tokens": 13},
				"unpermitted_response_field": "response-extra-should-not-appear"
			}`)

			host := startMemoryEventRequestWithBody(t, true, requestBody)
			callMemoryEventResponse(t, host, 200, responseBody)

			event := requireMemoryResponseCaptureEvent(t, host)
			requireMemoryEventRequiredFields(t, event)
			requireMemoryResponseSafeFacts(t, event, 200)
			require.Equal(t, "event user question", event["user_content"])
			require.Equal(t, "event assistant answer", event["assistant_content"])
			require.Equal(t, "stop", event["finish_reason"])
			require.Equal(t, false, event["is_stream"])
			require.Equal(t, false, event["contains_tool_calls"])
			requireMemoryResponseCaptureUsage(t, event, 9, 4, 13)

			cmd := requireMemoryEventXADDCommand(t, host)
			require.Equal(t, memoryEventStreamName, cmd[1])
			require.Contains(t, cmd, memoryEventStreamField)

			eventJSON := memoryEventJSON(t, event)
			requireMemoryEventJSONExcludes(t, event, "request-extra-should-not-appear", "unpermitted_request_field")
			requireMemoryEventJSONExcludes(t, event, "response-extra-should-not-appear", "unpermitted_response_field")
			require.NotContains(t, eventJSON, "cache:events")

			host.CallOnRedisCall(0, test.CreateRedisRespString("1700000000000-0"))
			host.CompleteHttp()

			startMemoryEventRequestOnHost(t, host, requestBody)
			callMemoryEventResponse(t, host, 200, responseBody)
			secondEvent := requireMemoryResponseCaptureEvent(t, host)
			require.Equal(t, event["request_digest"], secondEvent["request_digest"])
			require.Equal(t, event["idempotency_key"], secondEvent["idempotency_key"])
			require.NotEmpty(t, secondEvent["event_id"])
		})

		t.Run("raw content omission gates preserve safe facts", func(t *testing.T) {
			tests := []struct {
				name            string
				captureResponse bool
				headers         [][2]string
				statusCode      int
				responseBody    []byte
				wantFinish      string
				wantUsage       [3]int
			}{
				{
					name:            "disabled capture",
					captureResponse: false,
					statusCode:      200,
					responseBody:    memoryEventResponseBody(t, "disabled capture assistant content", "stop", 6, 4, 10),
					wantFinish:      "stop",
					wantUsage:       [3]int{6, 4, 10},
				},
				{
					name:            "request no-store",
					captureResponse: true,
					headers:         [][2]string{{memoryNoStoreHeader, "true"}},
					statusCode:      200,
					responseBody:    memoryEventResponseBody(t, "no-store assistant content", "stop", 7, 4, 11),
					wantFinish:      "stop",
					wantUsage:       [3]int{7, 4, 11},
				},
				{
					name:            "parse failure",
					captureResponse: true,
					statusCode:      200,
					responseBody:    []byte(`{not-json`),
				},
				{
					name:            "ineligible status",
					captureResponse: true,
					statusCode:      500,
					responseBody:    memoryEventResponseBody(t, "failed status assistant content", "stop", 5, 4, 9),
					wantFinish:      "stop",
					wantUsage:       [3]int{5, 4, 9},
				},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					host := startMemoryEventRequestWithBody(t, tt.captureResponse, memoryEventRequestBody(t, "raw content gate user question", nil), tt.headers...)
					callMemoryEventResponse(t, host, tt.statusCode, tt.responseBody)

					event := requireMemoryResponseCaptureEvent(t, host)
					requireMemoryEventRequiredFields(t, event)
					requireMemoryResponseSafeFacts(t, event, tt.statusCode)
					requireMemoryEventOmitsField(t, event, "user_content")
					requireMemoryEventOmitsField(t, event, "assistant_content")
					requireMemoryEventJSONExcludes(t, event, "raw content gate user question")
					requireMemoryEventJSONExcludes(t, event, "disabled capture assistant content", "no-store assistant content", "failed status assistant content")
					if tt.wantFinish != "" {
						require.Equal(t, tt.wantFinish, event["finish_reason"])
						requireMemoryResponseCaptureUsage(t, event, tt.wantUsage[0], tt.wantUsage[1], tt.wantUsage[2])
					}
				})
			}
		})

		t.Run("redis stream dispatch failures fail open and keep response unchanged", func(t *testing.T) {
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
					host := startMemoryEventRequestWithBody(t, true, memoryEventRequestBody(t, "xadd failure user question", nil))
					responseBody := memoryEventResponseBody(t, "xadd failure assistant answer", "stop", 5, 4, 9)
					callMemoryEventResponse(t, host, 200, responseBody)
					require.Equal(t, string(responseBody), string(host.GetResponseBody()))
					requireMemoryEventXADDCommand(t, host)

					host.CallOnRedisCall(tt.status, tt.response)

					require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
					require.Nil(t, host.GetLocalResponse())
					require.Equal(t, string(responseBody), string(host.GetResponseBody()))
				})
			}
		})
	})
}

func startMemoryEventRequestWithBody(t *testing.T, captureResponse bool, body []byte, extraHeaders ...[2]string) test.TestHost {
	t.Helper()
	host, status := newMemoryConfigTestHost(memoryResponseCaptureConfig(t, captureResponse))
	t.Cleanup(host.Reset)
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName("memory-route"))
	require.NoError(t, host.SetRequestId("property-request-id"))
	startMemoryEventRequestOnHost(t, host, body, extraHeaders...)
	return host
}

func startMemoryEventRequestOnHost(t *testing.T, host test.TestHost, body []byte, extraHeaders ...[2]string) {
	t.Helper()
	headers := applyMemoryRequestGatingHeaderOverrides(memoryConsoleAssembleHeaders(), [][2]string{
		{"x-request-id", "request-response-capture-1"},
	})
	headers = append(headers, extraHeaders...)
	action := host.CallOnHttpRequestHeaders(headers)
	require.Equal(t, types.HeaderStopIteration, action)

	action = host.CallOnHttpRequestBody(body)
	require.Equal(t, types.ActionPause, action)
	requireMemoryRecentRedisLookup(t, host)

	host.CallOnRedisCall(0, test.CreateRedisRespNull())
	require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
}

func callMemoryEventResponse(t *testing.T, host test.TestHost, statusCode int, responseBody []byte) {
	t.Helper()
	host.CallOnHttpResponseHeaders([][2]string{
		{":status", strconv.Itoa(statusCode)},
		{"content-type", "application/json"},
	})
	action := host.CallOnHttpResponseBody(responseBody)
	require.Equal(t, types.ActionContinue, action)
}

func memoryEventRequestBody(t *testing.T, userContent string, extraFields map[string]interface{}) []byte {
	t.Helper()
	body := map[string]interface{}{
		"model": "qwen-turbo",
		"messages": []map[string]interface{}{
			{"role": "user", "content": userContent},
		},
		"stream": false,
	}
	for key, value := range extraFields {
		body[key] = value
	}
	data, err := json.Marshal(body)
	require.NoError(t, err)
	return data
}

func memoryEventResponseBody(t *testing.T, assistantContent, finishReason string, promptTokens, completionTokens, totalTokens int) []byte {
	t.Helper()
	data, err := json.Marshal(map[string]interface{}{
		"id":     "chatcmpl-memory-event-response",
		"object": "chat.completion",
		"model":  "qwen-turbo",
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": assistantContent,
				},
				"finish_reason": finishReason,
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
			"total_tokens":      totalTokens,
		},
	})
	require.NoError(t, err)
	return data
}

func requireMemoryEventRequiredFields(t *testing.T, event map[string]interface{}) {
	t.Helper()
	for _, field := range []string{
		"schema_version",
		"event_id",
		"idempotency_key",
		"tenant",
		"consumer",
		"request_id",
		"request_path",
		"request_digest",
		"memory_mode",
		"status_code",
		"is_stream",
		"contains_tool_calls",
		"started_at_ms",
		"ended_at_ms",
		"plugin_version",
	} {
		require.Containsf(t, event, field, "MemoryEvent missing required field %q; fields=%s", field, memoryEventFieldList(event))
	}
}

func requireMemoryEventXADDCommand(t *testing.T, host test.TestHost) []string {
	t.Helper()
	var xadds [][]string
	for _, call := range host.GetRedisCalloutAttributes() {
		cmd, ok := memoryResponseCaptureCommand(t, call.Query)
		if !ok || len(cmd) < 5 {
			continue
		}
		if strings.EqualFold(cmd[0], "xadd") && cmd[1] == memoryEventStreamName {
			xadds = append(xadds, cmd)
		}
	}
	require.Lenf(t, xadds, 1, "expected exactly one MemoryEvent XADD; redis calls=%s", memoryResponseCaptureCallSummary(t, host))
	cmd := xadds[0]
	require.Containsf(t, cmd, memoryEventStreamField, "MemoryEvent XADD missing field %q; command=%s", memoryEventStreamField, memoryResponseCaptureCommandSummary(cmd))
	return cmd
}

func memoryEventJSON(t *testing.T, event map[string]interface{}) string {
	t.Helper()
	data, err := json.Marshal(event)
	require.NoError(t, err)
	return string(data)
}

func memoryEventFieldList(event map[string]interface{}) string {
	keys := make([]string, 0, len(event))
	for key := range event {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}
