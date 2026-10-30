package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func startThinReplayPayloadRequest(t *testing.T, stream bool) test.TestHost {
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
	require.NotEmpty(t, host.GetRedisCalloutAttributes(), "request should issue Redis lookup before replay")
	return host
}

func TestThinReplayPayload(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("non-streaming replay preserves embedded OpenAI response fields", func(t *testing.T) {
			host := startThinReplayPayloadRequest(t, false)
			defer host.Reset()

			response := map[string]interface{}{
				"id":                 "chatcmpl-preserved-42",
				"object":             "chat.completion",
				"created":            float64(1720000123),
				"model":              "qwen-turbo",
				"system_fingerprint": "fp-cache-preserved",
				"choices": []interface{}{
					map[string]interface{}{
						"index": float64(0),
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "cached weather answer with preserved fields",
						},
						"finish_reason": "stop",
					},
				},
				"usage": map[string]interface{}{
					"prompt_tokens":     float64(11),
					"completion_tokens": float64(7),
					"total_tokens":      float64(18),
				},
			}

			host.CallOnRedisCall(0, test.CreateRedisRespString(validThinReplayRecord(t, map[string]interface{}{
				"response": response,
				"usage":    response["usage"],
			})))

			localResponse := host.GetLocalResponse()
			require.NotNil(t, localResponse, "valid materialized record should be replayed")
			require.Equal(t, uint32(200), localResponse.StatusCode)
			require.Equal(t, "application/json; charset=utf-8", thinReplayPayloadHeader(localResponse.Headers, "content-type"))
			thinReplayPayloadRequireJSONEqual(t, thinReplayPayloadMustJSON(t, response), localResponse.Data)
		})

		t.Run("streaming replay emits stored chunks and done marker", func(t *testing.T) {
			host := startThinReplayPayloadRequest(t, true)
			defer host.Reset()

			chunks := []interface{}{
				map[string]interface{}{
					"id":                 "chatcmpl-stream-hit",
					"object":             "chat.completion.chunk",
					"created":            float64(1720000456),
					"model":              "qwen-turbo",
					"system_fingerprint": "fp-cache-stream",
					"choices": []interface{}{
						map[string]interface{}{
							"index":         float64(0),
							"delta":         map[string]interface{}{"role": "assistant"},
							"finish_reason": nil,
						},
					},
				},
				map[string]interface{}{
					"id":                 "chatcmpl-stream-hit",
					"object":             "chat.completion.chunk",
					"created":            float64(1720000456),
					"model":              "qwen-turbo",
					"system_fingerprint": "fp-cache-stream",
					"choices": []interface{}{
						map[string]interface{}{
							"index":         float64(0),
							"delta":         map[string]interface{}{"content": "cached stream answer"},
							"finish_reason": nil,
						},
					},
				},
				map[string]interface{}{
					"id":                 "chatcmpl-stream-hit",
					"object":             "chat.completion.chunk",
					"created":            float64(1720000456),
					"model":              "qwen-turbo",
					"system_fingerprint": "fp-cache-stream",
					"choices": []interface{}{
						map[string]interface{}{
							"index":         float64(0),
							"delta":         map[string]interface{}{},
							"finish_reason": "stop",
						},
					},
				},
			}

			host.CallOnRedisCall(0, test.CreateRedisRespString(validThinReplayRecord(t, map[string]interface{}{
				"stream_replayable": true,
				"stream_chunks":     chunks,
			})))

			localResponse := host.GetLocalResponse()
			require.NotNil(t, localResponse, "stream-replayable materialized record should be replayed")
			require.Equal(t, uint32(200), localResponse.StatusCode)
			require.Equal(t, "text/event-stream; charset=utf-8", thinReplayPayloadHeader(localResponse.Headers, "content-type"))

			frames := thinReplayPayloadSSEDataFrames(t, localResponse.Data)
			if len(frames) != len(chunks)+1 {
				require.Failf(t, "unexpected SSE frame count", "expected=%d actual=%d body=%s", len(chunks)+1, len(frames), thinReplayPayloadSnippet(localResponse.Data, 360))
			}
			for i, chunk := range chunks {
				thinReplayPayloadRequireJSONEqual(t, thinReplayPayloadMustJSON(t, chunk), []byte(frames[i]))
			}
			require.Equal(t, "[DONE]", frames[len(frames)-1])
		})
	})
}

func TestThinResponsesReplayPayload(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("non-streaming replay preserves Responses response object", func(t *testing.T) {
			host := startThinResponsesReplayPayloadRequest(t, false)
			defer host.Reset()

			response := thinResponsesReplayPayloadResponse()
			host.CallOnRedisCall(0, test.CreateRedisRespString(validThinResponsesReplayRecord(t, false, map[string]interface{}{
				"response": response,
			})))

			localResponse := host.GetLocalResponse()
			require.NotNil(t, localResponse, "valid Responses materialized record should be replayed")
			require.Equal(t, uint32(200), localResponse.StatusCode)
			require.Equal(t, "application/json; charset=utf-8", thinReplayPayloadHeader(localResponse.Headers, "content-type"))
			thinReplayPayloadRequireJSONEqual(t, thinReplayPayloadMustJSON(t, response), localResponse.Data)
		})

		t.Run("streaming replay emits Responses event frames", func(t *testing.T) {
			host := startThinResponsesReplayPayloadRequest(t, true)
			defer host.Reset()

			chunks := thinResponsesReplayPayloadStreamChunks()
			host.CallOnRedisCall(0, test.CreateRedisRespString(validThinResponsesReplayRecord(t, true, map[string]interface{}{
				"stream_chunks": chunks,
			})))

			localResponse := host.GetLocalResponse()
			require.NotNil(t, localResponse, "stream-replayable Responses materialized record should be replayed")
			require.Equal(t, uint32(200), localResponse.StatusCode)
			require.Equal(t, "text/event-stream; charset=utf-8", thinReplayPayloadHeader(localResponse.Headers, "content-type"))
			require.NotContains(t, string(localResponse.Data), "[DONE]", "Responses adapter replay should not add Chat Completions done markers")

			frames := thinResponsesReplayPayloadSSEFrames(t, localResponse.Data)
			requireThinResponsesReplayPayloadEventOrder(t, frames, []string{
				"response.created",
				"response.output_item.added",
				"response.output_text.delta",
				"response.output_text.done",
				"response.completed",
			})
			delta := requireThinResponsesReplayPayloadFrame(t, frames, "response.output_text.delta")
			thinReplayPayloadRequireJSONEqual(t, `{"type":"response.output_text.delta","item_id":"msg_1","output_index":0,"content_index":0,"delta":"cached"}`, []byte(delta.Data))
		})
	})
}

func TestThinMessagesReplayPayload(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("non-streaming replay preserves Messages response object", func(t *testing.T) {
			host := startThinMessagesReplayPayloadRequest(t, false)
			defer host.Reset()

			response := thinMessagesReplayPayloadResponse()
			host.CallOnRedisCall(0, test.CreateRedisRespString(validThinMessagesReplayRecord(t, false, map[string]interface{}{
				"response": response,
			})))

			localResponse := host.GetLocalResponse()
			require.NotNil(t, localResponse, "valid Messages materialized record should be replayed")
			require.Equal(t, uint32(200), localResponse.StatusCode)
			require.Equal(t, "application/json; charset=utf-8", thinReplayPayloadHeader(localResponse.Headers, "content-type"))
			thinReplayPayloadRequireJSONEqual(t, thinReplayPayloadMustJSON(t, response), localResponse.Data)
			require.NotContains(t, string(localResponse.Data), "choices")
		})

		t.Run("streaming replay emits Messages event frames", func(t *testing.T) {
			host := startThinMessagesReplayPayloadRequest(t, true)
			defer host.Reset()

			chunks := []interface{}{
				map[string]interface{}{"type": "message_start", "message": map[string]interface{}{"id": "msg-cache", "type": "message", "role": "assistant", "content": []interface{}{}}},
				map[string]interface{}{"type": "content_block_delta", "index": float64(0), "delta": map[string]interface{}{"type": "text_delta", "text": "cached Messages answer"}},
				map[string]interface{}{"type": "message_stop"},
			}
			host.CallOnRedisCall(0, test.CreateRedisRespString(validThinMessagesReplayRecord(t, true, map[string]interface{}{
				"stream_chunks": chunks,
			})))

			localResponse := host.GetLocalResponse()
			require.NotNil(t, localResponse, "stream-replayable Messages record should be replayed")
			require.Equal(t, uint32(200), localResponse.StatusCode)
			require.Equal(t, "text/event-stream; charset=utf-8", thinReplayPayloadHeader(localResponse.Headers, "content-type"))
			require.Contains(t, string(localResponse.Data), "event: message_start")
			require.Contains(t, string(localResponse.Data), "event: content_block_delta")
			require.Contains(t, string(localResponse.Data), "event: message_stop")
			require.NotContains(t, string(localResponse.Data), "[DONE]")
			require.NotContains(t, string(localResponse.Data), "choices")
		})
	})
}

func startThinMessagesReplayPayloadRequest(t *testing.T, stream bool) test.TestHost {
	t.Helper()
	host, status := test.NewTestHost(thinMessagesResponseCaptureConfig(t))
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName("test-route-messages"))
	action := host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"},
		{":path", "/v1/messages"},
		{":method", "POST"},
		{"content-type", "application/json"},
		{"x-mse-tenant", "tenant-a"},
		{"x-mse-consumer", "consumer-a"},
	})
	require.Equal(t, types.HeaderStopIteration, action)
	action = host.CallOnHttpRequestBody(thinMessagesReplayPayloadRequestBody(t, stream))
	require.Equal(t, types.ActionPause, action)
	require.NotEmpty(t, host.GetRedisCalloutAttributes(), "request should issue Redis lookup before Messages replay")
	return host
}

func thinMessagesReplayPayloadRequestBody(t *testing.T, stream bool) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]interface{}{
		"model": "claude-sonnet",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "summarize launch plan"},
		},
		"stream": stream,
	})
	require.NoError(t, err)
	return body
}

func validThinMessagesReplayRecord(t *testing.T, stream bool, overrides map[string]interface{}) string {
	t.Helper()
	adapter := protocol.MessagesAdapter{}
	digest, err := adapter.BuildCacheDigest(protocol.RequestParseInput{
		Method: "POST",
		Path:   "/v1/messages",
		Body:   thinMessagesReplayPayloadRequestBody(t, stream),
	})
	require.NoError(t, err)
	now := time.Now().Unix()
	record := map[string]interface{}{
		"schema_version":       "ai-cache.materialized.v1",
		"tenant":               "tenant-a",
		"consumer":             "consumer-a",
		"route":                "test-route-messages",
		"model":                "claude-sonnet",
		"protocol":             "messages",
		"cache_scope":          "consumer",
		"cache_policy_version": "policy-v1",
		"request_digest":       digest.Digest,
		"soft_expires_at":      now + 60,
		"hard_expires_at":      now + 3600,
		"response":             thinMessagesReplayPayloadResponse(),
		"usage":                map[string]interface{}{"input_tokens": float64(8), "output_tokens": float64(3)},
		"finish_reason":        "end_turn",
	}
	if stream {
		record["stream_replayable"] = true
	}
	for key, value := range overrides {
		record[key] = value
	}
	body, err := json.Marshal(record)
	require.NoError(t, err)
	return string(body)
}

func thinMessagesReplayPayloadResponse() map[string]interface{} {
	return map[string]interface{}{
		"id":          "msg-cache",
		"type":        "message",
		"role":        "assistant",
		"model":       "claude-sonnet",
		"content":     []interface{}{map[string]interface{}{"type": "text", "text": "cached Messages answer"}},
		"stop_reason": "end_turn",
		"usage":       map[string]interface{}{"input_tokens": float64(8), "output_tokens": float64(3)},
	}
}

func startThinResponsesReplayPayloadRequest(t *testing.T, stream bool) test.TestHost {
	t.Helper()
	host, status := test.NewTestHost(thinResponsesResponseCaptureConfig(t))
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName("test-route-responses"))
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
	require.NotEmpty(t, host.GetRedisCalloutAttributes(), "request should issue Redis lookup before Responses replay")
	return host
}

func validThinResponsesReplayRecord(t *testing.T, stream bool, overrides map[string]interface{}) string {
	t.Helper()
	now := time.Now().Unix()
	usage := map[string]interface{}{
		"input_tokens":  float64(15),
		"output_tokens": float64(2),
		"total_tokens":  float64(17),
	}
	if stream {
		usage = map[string]interface{}{
			"input_tokens":  float64(15),
			"output_tokens": float64(1),
			"total_tokens":  float64(16),
		}
	}
	record := map[string]interface{}{
		"schema_version":       "ai-cache.materialized.v1",
		"tenant":               "tenant-a",
		"consumer":             "consumer-a",
		"route":                "test-route-responses",
		"model":                "gpt-4.1",
		"protocol":             "responses",
		"cache_scope":          "consumer",
		"cache_policy_version": "policy-v1",
		"request_digest":       expectedThinResponsesResponseCaptureRequestDigest(t, stream),
		"soft_expires_at":      now + 60,
		"hard_expires_at":      now + 3600,
		"response":             thinResponsesReplayPayloadResponse(),
		"usage":                usage,
		"finish_reason":        "completed",
	}
	if stream {
		record["stream_replayable"] = true
		record["stream_chunks"] = thinResponsesReplayPayloadStreamChunks()
	}
	for key, value := range overrides {
		record[key] = value
	}
	body, err := json.Marshal(record)
	require.NoError(t, err)
	return string(body)
}

func thinResponsesReplayPayloadResponse() map[string]interface{} {
	return map[string]interface{}{
		"id":          "resp-cache-hit",
		"object":      "response",
		"model":       "gpt-4.1",
		"status":      "completed",
		"output_text": "cached Responses answer",
		"output": []interface{}{
			map[string]interface{}{
				"id":   "msg_1",
				"type": "message",
				"role": "assistant",
				"content": []interface{}{
					map[string]interface{}{"type": "output_text", "text": "cached Responses answer"},
				},
			},
		},
		"usage": map[string]interface{}{
			"input_tokens":  float64(15),
			"output_tokens": float64(2),
			"total_tokens":  float64(17),
		},
	}
}

func thinResponsesReplayPayloadStreamChunks() []interface{} {
	return []interface{}{
		map[string]interface{}{
			"type":     "response.created",
			"response": map[string]interface{}{"id": "resp_cached", "object": "response", "model": "gpt-4.1", "status": "in_progress"},
		},
		map[string]interface{}{
			"type":         "response.output_item.added",
			"output_index": float64(0),
			"item":         map[string]interface{}{"id": "msg_1", "type": "message", "role": "assistant", "content": []interface{}{}},
		},
		map[string]interface{}{
			"type":          "response.output_text.delta",
			"item_id":       "msg_1",
			"output_index":  float64(0),
			"content_index": float64(0),
			"delta":         "cached",
		},
		map[string]interface{}{
			"type":          "response.output_text.done",
			"item_id":       "msg_1",
			"output_index":  float64(0),
			"content_index": float64(0),
			"text":          "cached",
		},
		map[string]interface{}{
			"type":     "response.completed",
			"response": map[string]interface{}{"id": "resp_cached", "status": "completed", "usage": map[string]interface{}{"input_tokens": float64(15), "output_tokens": float64(1), "total_tokens": float64(16)}},
		},
	}
}

func thinReplayPayloadHeader(headers [][2]string, name string) string {
	for _, header := range headers {
		if strings.EqualFold(header[0], name) {
			return header[1]
		}
	}
	return ""
}

type thinResponsesReplayPayloadSSEFrame struct {
	Event string
	Data  string
}

func thinResponsesReplayPayloadSSEFrames(t *testing.T, body []byte) []thinResponsesReplayPayloadSSEFrame {
	t.Helper()
	rawFrames := strings.Split(strings.TrimSpace(string(body)), "\n\n")
	frames := make([]thinResponsesReplayPayloadSSEFrame, 0, len(rawFrames))
	for _, raw := range rawFrames {
		var frame thinResponsesReplayPayloadSSEFrame
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "event:"):
				frame.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				frame.Data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			default:
				require.Failf(t, "unexpected Responses SSE line", "line=%q body=%s", line, thinReplayPayloadSnippet(body, 360))
			}
		}
		require.NotEmptyf(t, frame.Event, "Responses replay frame must include an event line; frame=%q body=%s", raw, thinReplayPayloadSnippet(body, 360))
		require.NotEmptyf(t, frame.Data, "Responses replay frame must include a data line; frame=%q body=%s", raw, thinReplayPayloadSnippet(body, 360))
		frames = append(frames, frame)
	}
	return frames
}

func requireThinResponsesReplayPayloadFrame(t *testing.T, frames []thinResponsesReplayPayloadSSEFrame, event string) thinResponsesReplayPayloadSSEFrame {
	t.Helper()
	for _, frame := range frames {
		if frame.Event == event {
			return frame
		}
	}
	require.Failf(t, "missing Responses replay frame", "event=%q frames=%+v", event, frames)
	return thinResponsesReplayPayloadSSEFrame{}
}

func requireThinResponsesReplayPayloadEventOrder(t *testing.T, frames []thinResponsesReplayPayloadSSEFrame, expected []string) {
	t.Helper()
	next := 0
	for _, frame := range frames {
		if next < len(expected) && frame.Event == expected[next] {
			next++
		}
	}
	require.Equalf(t, len(expected), next, "missing Responses replay event sequence %v in frames %+v", expected, frames)
}

func thinReplayPayloadSSEDataFrames(t *testing.T, body []byte) []string {
	t.Helper()
	lines := strings.Split(string(body), "\n")
	frames := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
			continue
		case strings.HasPrefix(line, "data:"):
			frames = append(frames, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		default:
			require.Failf(t, "unexpected SSE line", "line=%q body=%s", line, thinReplayPayloadSnippet(body, 360))
		}
	}
	return frames
}

func thinReplayPayloadRequireJSONEqual(t *testing.T, expected string, actual []byte) {
	t.Helper()
	var expectedValue interface{}
	require.NoError(t, json.Unmarshal([]byte(expected), &expectedValue))

	var actualValue interface{}
	if err := json.Unmarshal(actual, &actualValue); err != nil {
		require.Failf(t, "invalid JSON body", "err=%v body=%s", err, thinReplayPayloadSnippet(actual, 360))
	}
	if !reflect.DeepEqual(expectedValue, actualValue) {
		require.Failf(t, "JSON payload mismatch", "expected=%s actual=%s", expected, thinReplayPayloadSnippet(actual, 360))
	}
}

func thinReplayPayloadMustJSON(t *testing.T, value interface{}) string {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return string(data)
}

func thinReplayPayloadSnippet(data []byte, limit int) string {
	body := string(data)
	if len(body) <= limit {
		return body
	}
	return body[:limit] + "..."
}
