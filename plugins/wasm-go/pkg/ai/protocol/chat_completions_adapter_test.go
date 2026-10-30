package protocol

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChatCompletionsAdapterInjectsMemoryIntoMessages(t *testing.T) {
	adapter := ChatCompletionsAdapter{}
	injected, err := adapter.InjectContext([]byte(`{
		"model": "qwen-turbo",
		"messages": [
			{"role": "system", "content": "You are concise."},
			{"role": "user", "content": "What did we decide?"}
		],
		"stream": false,
		"temperature": 0.2,
		"metadata": {"trace_id": "trace-a"}
	}`), InjectableContext{
		Text:      "Memory: ship on Tuesday.",
		Placement: InjectionSystem,
		Messages:  []Message{{Role: "system", Content: "Memory: ship on Tuesday."}},
	})

	require.NoError(t, err)
	var payload struct {
		Model    string `json:"model"`
		Stream   bool   `json:"stream"`
		Messages []struct {
			Role    string      `json:"role"`
			Content interface{} `json:"content"`
		} `json:"messages"`
		Temperature json.RawMessage `json:"temperature"`
		Metadata    struct {
			TraceID string `json:"trace_id"`
		} `json:"metadata"`
	}
	require.NoError(t, json.Unmarshal(injected, &payload))
	require.Equal(t, "qwen-turbo", payload.Model)
	require.False(t, payload.Stream)
	require.Equal(t, "trace-a", payload.Metadata.TraceID)
	require.Len(t, payload.Messages, 3)
	require.Equal(t, "system", payload.Messages[0].Role)
	require.Equal(t, "You are concise.", payload.Messages[0].Content)
	require.Equal(t, "system", payload.Messages[1].Role)
	require.Equal(t, "Memory: ship on Tuesday.", payload.Messages[1].Content)
	require.Equal(t, "user", payload.Messages[2].Role)
	require.JSONEq(t, `0.2`, string(payload.Temperature))
}

func TestChatCompletionsAdapterSkipsOverBudgetMemoryInjection(t *testing.T) {
	adapter := ChatCompletionsAdapter{}
	original := []byte(`{
		"model": "qwen-turbo",
		"messages": [
			{"role": "system", "content": "You are concise."},
			{"role": "user", "content": "What did we decide?"}
		],
		"stream": false
	}`)

	injected, err := adapter.InjectContext(original, InjectableContext{
		Text:          "Memory: this should not fit in the budget.",
		Placement:     InjectionSystem,
		TokenEstimate: 128,
		TokenBudget:   64,
	})

	require.NoError(t, err)
	require.JSONEq(t, string(original), string(injected))
	require.NotContains(t, string(injected), "this should not fit")
}

func TestChatCompletionsAdapterCapturesNonStreamResponse(t *testing.T) {
	adapter := ChatCompletionsAdapter{}
	exchange, err := adapter.CaptureResponse(ResponseCaptureInput{
		StatusCode: 200,
		Body: []byte(`{
			"id": "chatcmpl-fixture",
			"object": "chat.completion",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "We decided to ship on Tuesday.",
					"tool_calls": [{"id":"call-1","type":"function","function":{"name":"lookup","arguments":"{}"}}]
				},
				"finish_reason": "tool_calls"
			}],
			"usage": {"prompt_tokens": 12, "completion_tokens": 8, "total_tokens": 20}
		}`),
	})

	require.NoError(t, err)
	require.Equal(t, ProtocolChatCompletions, exchange.Request.Protocol)
	require.Equal(t, "We decided to ship on Tuesday.", exchange.Response.Text)
	require.Equal(t, "tool_calls", exchange.Response.FinishReason)
	require.True(t, exchange.Response.ContainsToolCalls)
	require.Equal(t, Usage{InputTokens: 12, OutputTokens: 8, TotalTokens: 20}, exchange.Usage)
}

func TestChatCompletionsAdapterCapturesSSEResponse(t *testing.T) {
	adapter := ChatCompletionsAdapter{}
	exchange, err := adapter.CaptureStream(ResponseStreamInput{
		StatusCode: 200,
		Chunks: [][]byte{
			[]byte("data: {\"id\":\"chatcmpl-stream-fixture\",\"object\":\"chat.completion.chunk\",\"created\":1782420000,\"model\":\"qwen-turbo\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"},\"finish_reason\":null}]}\r\n\r\n"),
			[]byte("data: {\"id\":\"chatcmpl-stream-fixture\",\"object\":\"chat.completion.chunk\",\"created\":1782420000,\"model\":\"qwen-turbo\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Tuesday\"},\"finish_reason\":null}]}\n\n"),
			[]byte("data: {\"id\":\"chatcmpl-stream-fixture\",\"object\":\"chat.completion.chunk\",\"created\":1782420000,\"model\":\"qwen-turbo\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\" launch\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":12,\"completion_tokens\":3,\"total_tokens\":15}}\n\ndata: [DONE]\n\n"),
		},
	})

	require.NoError(t, err)
	require.Equal(t, ProtocolChatCompletions, exchange.Request.Protocol)
	require.Equal(t, "Tuesday launch", exchange.Response.Text)
	require.Equal(t, "stop", exchange.Response.FinishReason)
	require.Equal(t, Usage{InputTokens: 12, OutputTokens: 3, TotalTokens: 15}, exchange.Usage)
	require.Len(t, exchange.StreamChunks, 3)
	require.Equal(t, "chat.completion.chunk", exchange.StreamChunks[1].Event)
	require.Equal(t, "Tuesday", exchange.StreamChunks[1].TextDelta)
	require.True(t, exchange.StreamChunks[1].Replayable)
}

func TestChatCompletionsAdapterCapturesSplitSSEFramesForReplay(t *testing.T) {
	adapter := ChatCompletionsAdapter{}
	exchange, err := adapter.CaptureStream(ResponseStreamInput{
		StatusCode: 200,
		Chunks: [][]byte{
			[]byte("data: {\"id\":\"chatcmpl-split\",\"object\":\"chat.completion.chunk\",\"created\":1782420000,"),
			[]byte("\"model\":\"qwen-turbo\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"split\"},\"finish_reason\":null}]}\n"),
			[]byte("\n"),
			[]byte("data: {\"id\":\"chatcmpl-split\",\"object\":\"chat.completion.chunk\",\"created\":1782420000,\"model\":\"qwen-turbo\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\" frame\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":12,\"completion_tokens\":2,\"total_tokens\":14}}\n\ndata: [DONE]\n\n"),
		},
	})

	require.NoError(t, err)
	require.Equal(t, "split frame", exchange.Response.Text)
	require.Equal(t, "stop", exchange.Response.FinishReason)
	require.Equal(t, Usage{InputTokens: 12, OutputTokens: 2, TotalTokens: 14}, exchange.Usage)
	require.Len(t, exchange.StreamChunks, 2)
	require.Equal(t, "split", exchange.StreamChunks[0].TextDelta)
	require.Equal(t, " frame", exchange.StreamChunks[1].TextDelta)
	frames := chatCompletionsReplaySSEDataFrames(t, dataOnlySSEReplay(exchange.StreamChunks, true))
	require.Len(t, frames, 3)
	requireJSONEqual(t, `{"id":"chatcmpl-split","object":"chat.completion.chunk","created":1782420000,"model":"qwen-turbo","choices":[{"index":0,"delta":{"content":"split"},"finish_reason":null}]}`, frames[0])
	requireJSONEqual(t, `{"id":"chatcmpl-split","object":"chat.completion.chunk","created":1782420000,"model":"qwen-turbo","choices":[{"index":0,"delta":{"content":" frame"},"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":2,"total_tokens":14}}`, frames[1])
	require.Equal(t, "[DONE]", frames[2])
}

func TestChatCompletionsAdapterBuildsCacheDigest(t *testing.T) {
	adapter := ChatCompletionsAdapter{}
	bodyWithoutStream := []byte(`{
		"model": "qwen-turbo",
		"messages": [{"role": "user", "content": "weather?"}],
		"temperature": 0.2,
		"stream": false
	}`)
	bodyWithStream := []byte(`{
		"stream": true,
		"temperature": 0.2,
		"messages": [{"content": "weather?", "role": "user"}],
		"model": "qwen-turbo"
	}`)

	digestA, err := adapter.BuildCacheDigest(RequestParseInput{
		Method:                "POST",
		Path:                  "/v1/chat/completions",
		Body:                  bodyWithoutStream,
		ContextPolicyVersion:  "memory-policy-v1",
		InjectedContextDigest: "memory-digest-a",
	})
	require.NoError(t, err)
	digestB, err := adapter.BuildCacheDigest(RequestParseInput{
		Method:                "POST",
		Path:                  "/v1/chat/completions",
		Body:                  bodyWithStream,
		ContextPolicyVersion:  "memory-policy-v1",
		InjectedContextDigest: "memory-digest-a",
	})
	require.NoError(t, err)

	require.Equal(t, ProtocolChatCompletions, digestA.Input.Protocol)
	require.Equal(t, "qwen-turbo", digestA.Input.Model)
	require.Contains(t, digestA.Input.ExcludedFields, "stream")
	require.Equal(t, "memory-policy-v1", digestA.Input.ContextPolicyVersion)
	require.Equal(t, "memory-digest-a", digestA.Input.InjectedContextDigest)
	require.NotEmpty(t, digestA.Digest)
	require.Equal(t, digestA.Digest, digestB.Digest)
	require.JSONEq(t, `[{"role":"user","content":"weather?"}]`, string(digestA.Input.Fields["messages"]))

	digestWithDifferentPolicy, err := adapter.BuildCacheDigest(RequestParseInput{
		Method:                "POST",
		Path:                  "/v1/chat/completions",
		Body:                  bodyWithStream,
		ContextPolicyVersion:  "memory-policy-v2",
		InjectedContextDigest: "memory-digest-a",
	})
	require.NoError(t, err)
	require.NotEqual(t, digestA.Digest, digestWithDifferentPolicy.Digest)

	digestWithDifferentContext, err := adapter.BuildCacheDigest(RequestParseInput{
		Method:                "POST",
		Path:                  "/v1/chat/completions",
		Body:                  bodyWithStream,
		ContextPolicyVersion:  "memory-policy-v1",
		InjectedContextDigest: "memory-digest-b",
	})
	require.NoError(t, err)
	require.NotEqual(t, digestA.Digest, digestWithDifferentContext.Digest)
}

func TestChatCompletionsAdapterSerializesReplayPayload(t *testing.T) {
	adapter := ChatCompletionsAdapter{}
	nonStream, err := adapter.SerializeReplay(ReplayPayload{
		Protocol:    ProtocolChatCompletions,
		StatusCode:  200,
		ContentType: "application/json; charset=utf-8",
		Body:        json.RawMessage(`{"choices":[{"message":{"role":"assistant","content":"cached answer"},"finish_reason":"stop"}]}`),
	})
	require.NoError(t, err)
	require.Equal(t, "application/json; charset=utf-8", nonStream.ContentType)
	require.JSONEq(t, `{"choices":[{"message":{"role":"assistant","content":"cached answer"},"finish_reason":"stop"}]}`, string(nonStream.Body))

	stream, err := adapter.SerializeReplay(ReplayPayload{
		Protocol:    ProtocolChatCompletions,
		ContentType: "text/event-stream; charset=utf-8",
		StreamChunks: []StreamChunk{{
			Protocol:   ProtocolChatCompletions,
			Event:      "chat.completion.chunk",
			Data:       json.RawMessage(`{"choices":[{"delta":{"content":"cached"},"finish_reason":null}]}`),
			TextDelta:  "cached",
			Replayable: true,
		}},
	})
	require.NoError(t, err)
	require.Equal(t, "text/event-stream; charset=utf-8", stream.ContentType)
	frames := chatCompletionsReplaySSEDataFrames(t, stream.Body)
	require.Len(t, frames, 2)
	requireJSONEqual(t, `{"choices":[{"delta":{"content":"cached"},"finish_reason":null}]}`, frames[0])
	require.Equal(t, "[DONE]", frames[1])
}

func TestChatCompletionsAdapterRejectsMismatchedReplayPayloadWithoutLeakingBody(t *testing.T) {
	adapter := ChatCompletionsAdapter{}
	_, err := adapter.SerializeReplay(ReplayPayload{
		Protocol:    ProtocolMessages,
		StatusCode:  200,
		ContentType: "application/json",
		Body:        json.RawMessage(`{"api_key":"sk-should-not-leak","content":"raw prompt should not leak"}`),
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "replay protocol mismatch")
	require.NotContains(t, err.Error(), "sk-should-not-leak")
	require.NotContains(t, err.Error(), "raw prompt")
}

func chatCompletionsReplaySSEDataFrames(t *testing.T, body []byte) []string {
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
			require.Failf(t, "unexpected SSE line", "line=%q body=%s", line, string(body))
		}
	}
	return frames
}

func requireJSONEqual(t *testing.T, expected string, actual string) {
	t.Helper()
	var expectedValue interface{}
	require.NoError(t, json.Unmarshal([]byte(expected), &expectedValue))
	var actualValue interface{}
	require.NoError(t, json.Unmarshal([]byte(actual), &actualValue))
	if !reflect.DeepEqual(expectedValue, actualValue) {
		require.Failf(t, "JSON payload mismatch", "expected=%s actual=%s", expected, actual)
	}
}
