package protocol

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResponsesAdapterInjectsMemoryIntoInstructionsAndInput(t *testing.T) {
	adapter := ResponsesAdapter{}
	injected, err := adapter.InjectContext([]byte(`{
		"model": "gpt-4.1",
		"instructions": "You are concise.",
		"input": [
			{"role":"user","content":[{"type":"input_text","text":"What did we decide?"}]}
		],
		"stream": false,
		"metadata": {"trace_id":"trace-responses-a"}
	}`), InjectableContext{
		Text:      "Memory: ship on Tuesday.",
		Placement: InjectionInstructions,
	})

	require.NoError(t, err)
	var payload struct {
		Model        string `json:"model"`
		Instructions string `json:"instructions"`
		Stream       bool   `json:"stream"`
		Input        []struct {
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"input"`
		Metadata struct {
			TraceID string `json:"trace_id"`
		} `json:"metadata"`
	}
	require.NoError(t, json.Unmarshal(injected, &payload))
	require.Equal(t, "gpt-4.1", payload.Model)
	require.False(t, payload.Stream)
	require.Equal(t, "trace-responses-a", payload.Metadata.TraceID)
	require.Equal(t, "You are concise.\n\nMemory: ship on Tuesday.", payload.Instructions)
	require.Len(t, payload.Input, 1)
	require.Equal(t, "user", payload.Input[0].Role)
	require.Equal(t, "input_text", payload.Input[0].Content[0].Type)
	require.Equal(t, "What did we decide?", payload.Input[0].Content[0].Text)
}

func TestResponsesAdapterInjectsMemoryIntoInputContent(t *testing.T) {
	adapter := ResponsesAdapter{}
	injected, err := adapter.InjectContext([]byte(`{
		"model": "gpt-4.1",
		"input": [
			{"role":"user","content":[{"type":"input_text","text":"Draft a reply."}]}
		],
		"stream": false
	}`), InjectableContext{
		Text:      "Memory: mention the Tuesday launch.",
		Placement: InjectionInput,
		ContentBlocks: []ContentBlock{{
			Type: "input_text",
			Text: "Memory: mention the Tuesday launch.",
		}},
	})

	require.NoError(t, err)
	var payload struct {
		Input []struct {
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"input"`
	}
	require.NoError(t, json.Unmarshal(injected, &payload))
	require.Len(t, payload.Input, 2)
	require.Equal(t, "system", payload.Input[0].Role)
	require.Equal(t, "input_text", payload.Input[0].Content[0].Type)
	require.Equal(t, "Memory: mention the Tuesday launch.", payload.Input[0].Content[0].Text)
	require.Equal(t, "user", payload.Input[1].Role)
	require.Equal(t, "Draft a reply.", payload.Input[1].Content[0].Text)
}

func TestResponsesAdapterSkipsOverBudgetMemoryInjection(t *testing.T) {
	adapter := ResponsesAdapter{}
	original := []byte(`{
		"model": "gpt-4.1",
		"instructions": "You are concise.",
		"input": [
			{"role":"user","content":[{"type":"input_text","text":"What did we decide?"}]}
		],
		"stream": false
	}`)

	injected, err := adapter.InjectContext(original, InjectableContext{
		Text:          "Memory: this should not fit in the budget.",
		Placement:     InjectionInstructions,
		TokenEstimate: 128,
		TokenBudget:   64,
	})

	require.NoError(t, err)
	require.JSONEq(t, string(original), string(injected))
	require.NotContains(t, string(injected), "this should not fit")
}

func TestResponsesAdapterCapturesNonStreamOutputTextAndItems(t *testing.T) {
	adapter := ResponsesAdapter{}
	exchange, err := adapter.CaptureResponse(ResponseCaptureInput{
		StatusCode: 200,
		Body: []byte(`{
			"id": "resp_fixture",
			"object": "response",
			"model": "gpt-4.1",
			"output_text": "We decided to ship on Tuesday.",
			"output": [
				{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"output_text","text":"We decided to ship on Tuesday."}]},
				{"id":"call_1","type":"function_call","name":"lookup","arguments":"{}"}
			],
			"status": "completed",
			"finish_reason": "tool_calls",
			"usage": {"input_tokens": 15, "output_tokens": 6, "total_tokens": 21}
		}`),
	})

	require.NoError(t, err)
	require.Equal(t, ProtocolResponses, exchange.Request.Protocol)
	require.Equal(t, "We decided to ship on Tuesday.", exchange.Response.Text)
	require.Equal(t, "tool_calls", exchange.Response.FinishReason)
	require.True(t, exchange.Response.ContainsToolCalls)
	require.Equal(t, Usage{InputTokens: 15, OutputTokens: 6, TotalTokens: 21}, exchange.Usage)

	itemOnlyExchange, err := adapter.CaptureResponse(ResponseCaptureInput{
		StatusCode: 200,
		Body: []byte(`{
			"id": "resp_item_only_fixture",
			"object": "response",
			"model": "gpt-4.1",
			"output": [
				{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"output_text","text":"Item-only answer."}]}
			],
			"status": "completed",
			"usage": {"input_tokens": 9, "output_tokens": 3, "total_tokens": 12}
		}`),
	})

	require.NoError(t, err)
	require.Equal(t, ProtocolResponses, itemOnlyExchange.Request.Protocol)
	require.Equal(t, "Item-only answer.", itemOnlyExchange.Response.Text)
	require.Equal(t, "completed", itemOnlyExchange.Response.FinishReason)
	require.False(t, itemOnlyExchange.Response.ContainsToolCalls)
	require.Equal(t, Usage{InputTokens: 9, OutputTokens: 3, TotalTokens: 12}, itemOnlyExchange.Usage)
}

func TestResponsesAdapterCapturesCacheAwareUsageFromReplayBody(t *testing.T) {
	adapter := ResponsesAdapter{}
	exchange, err := adapter.CaptureResponse(ResponseCaptureInput{
		StatusCode: 200,
		Body: []byte(`{
			"id": "resp_cache_replay_fixture",
			"object": "response",
			"model": "deepseek-v4-flash",
			"output_text": "cached answer",
			"status": "completed",
			"usage": {
				"input_tokens": 683,
				"input_cache_hit_tokens": 683,
				"input_cache_miss_tokens": 0,
				"output_tokens": 146,
				"total_tokens": 829
			}
		}`),
	})

	require.NoError(t, err)
	require.Equal(t, Usage{
		InputTokens:          683,
		InputCacheHitTokens:  683,
		InputCacheMissTokens: 0,
		OutputTokens:         146,
		TotalTokens:          829,
	}, exchange.Usage)
}

func TestResponsesAdapterCapturesResponsesStreamEvents(t *testing.T) {
	adapter := ResponsesAdapter{}
	exchange, err := adapter.CaptureStream(ResponseStreamInput{
		StatusCode: 200,
		Chunks: [][]byte{
			[]byte("event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_stream_fixture\",\"object\":\"response\",\"model\":\"gpt-4.1\",\"status\":\"in_progress\"}}\n\n"),
			[]byte("event: response.output_item.added\ndata: {\"type\":\"response.output_item.added\",\"output_index\":0,\"item\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[]}}\n\n"),
			[]byte("event: response.content_part.added\ndata: {\"type\":\"response.content_part.added\",\"item_id\":\"msg_1\",\"output_index\":0,\"content_index\":0,\"part\":{\"type\":\"output_text\",\"text\":\"\"}}\n\n"),
			[]byte("event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"item_id\":\"msg_1\",\"output_index\":0,\"content_index\":0,\"delta\":\"Tuesday\"}\n\n"),
			[]byte("event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"item_id\":\"msg_1\",\"output_index\":0,\"content_index\":0,\"delta\":\" launch\"}\n\n"),
			[]byte("event: response.output_text.done\ndata: {\"type\":\"response.output_text.done\",\"item_id\":\"msg_1\",\"output_index\":0,\"content_index\":0,\"text\":\"Tuesday launch\"}\n\n"),
			[]byte("event: response.output_item.added\ndata: {\"type\":\"response.output_item.added\",\"output_index\":1,\"item\":{\"id\":\"call_1\",\"type\":\"function_call\",\"name\":\"lookup\",\"arguments\":\"\",\"call_id\":\"call_fixture\"}}\n\n"),
			[]byte("event: response.function_call_arguments.delta\ndata: {\"type\":\"response.function_call_arguments.delta\",\"item_id\":\"call_1\",\"output_index\":1,\"delta\":\"{\\\"topic\\\":\"}\n\n"),
			[]byte("event: response.function_call_arguments.delta\ndata: {\"type\":\"response.function_call_arguments.delta\",\"item_id\":\"call_1\",\"output_index\":1,\"delta\":\"\\\"launch\\\"}\"}\n\n"),
			[]byte("event: response.function_call_arguments.done\ndata: {\"type\":\"response.function_call_arguments.done\",\"item_id\":\"call_1\",\"output_index\":1,\"arguments\":\"{\\\"topic\\\":\\\"launch\\\"}\"}\n\n"),
			[]byte("event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_stream_fixture\",\"status\":\"completed\",\"usage\":{\"input_tokens\":15,\"output_tokens\":3,\"total_tokens\":18}}}\n\n"),
		},
	})

	require.NoError(t, err)
	require.Equal(t, ProtocolResponses, exchange.Request.Protocol)
	require.Equal(t, "Tuesday launch", exchange.Response.Text)
	require.Equal(t, "completed", exchange.Response.FinishReason)
	require.True(t, exchange.Response.ContainsToolCalls)
	require.Equal(t, Usage{InputTokens: 15, OutputTokens: 3, TotalTokens: 18}, exchange.Usage)
	textDelta := requireResponsesStreamChunk(t, exchange.StreamChunks, "response.output_text.delta", "Tuesday")
	require.True(t, textDelta.Replayable)
	functionCallDelta := requireResponsesStreamChunk(t, exchange.StreamChunks, "response.function_call_arguments.delta", "")
	require.True(t, functionCallDelta.Replayable)
	require.JSONEq(t, `{"type":"response.function_call_arguments.delta","item_id":"call_1","output_index":1,"delta":"{\"topic\":"}`, string(functionCallDelta.Data))
	requireResponsesStreamEvent(t, exchange.StreamChunks, "response.function_call_arguments.done")
	requireResponsesStreamEvent(t, exchange.StreamChunks, "response.completed")
}

func TestResponsesAdapterCapturesCacheAwareUsageFromCompletedStream(t *testing.T) {
	adapter := ResponsesAdapter{}
	exchange, err := adapter.CaptureStream(ResponseStreamInput{
		StatusCode: 200,
		Chunks: [][]byte{
			[]byte("event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_stream_cache_fixture\",\"status\":\"completed\",\"usage\":{\"input_tokens\":21,\"input_cache_hit_tokens\":13,\"input_cache_miss_tokens\":8,\"output_tokens\":5,\"total_tokens\":26}}}\n\n"),
		},
	})

	require.NoError(t, err)
	require.Equal(t, Usage{
		InputTokens:          21,
		InputCacheHitTokens:  13,
		InputCacheMissTokens: 8,
		OutputTokens:         5,
		TotalTokens:          26,
	}, exchange.Usage)
	completed := requireResponsesStreamChunk(t, exchange.StreamChunks, "response.completed", "")
	require.NotNil(t, completed.Usage)
	require.Equal(t, exchange.Usage, *completed.Usage)
}

func TestResponsesUsageRejectsInvalidCacheAwareSplits(t *testing.T) {
	tests := []struct {
		name  string
		usage responsesUsage
	}{
		{
			name: "negative cache tokens",
			usage: responsesUsage{
				InputTokens:          21,
				InputCacheHitTokens:  -1,
				InputCacheMissTokens: 22,
			},
		},
		{
			name: "cache split differs from input",
			usage: responsesUsage{
				InputTokens:          21,
				InputCacheHitTokens:  13,
				InputCacheMissTokens: 7,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			normalized := tc.usage.normalized()
			require.Zero(t, normalized.InputCacheHitTokens)
			require.Zero(t, normalized.InputCacheMissTokens)
		})
	}
}

func TestResponsesAdapterCapturesCompletedStreamOutputFallback(t *testing.T) {
	adapter := ResponsesAdapter{}
	exchange, err := adapter.CaptureStream(ResponseStreamInput{
		StatusCode: 200,
		Chunks: [][]byte{
			[]byte("event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_stream_fixture\",\"object\":\"response\",\"model\":\"gpt-4.1\",\"status\":\"in_progress\"}}\n\n"),
			[]byte("event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_stream_fixture\",\"status\":\"completed\",\"finish_reason\":\"stop\",\"output\":[{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"completed fallback\"}]}],\"usage\":{\"input_tokens\":11,\"output_tokens\":2,\"total_tokens\":13}}}\n\n"),
		},
	})

	require.NoError(t, err)
	require.Equal(t, ProtocolResponses, exchange.Request.Protocol)
	require.Equal(t, "completed fallback", exchange.Response.Text)
	require.Equal(t, "stop", exchange.Response.FinishReason)
	require.Equal(t, Usage{InputTokens: 11, OutputTokens: 2, TotalTokens: 13}, exchange.Usage)
}

func TestResponsesAdapterBuildsCacheDigest(t *testing.T) {
	adapter := ResponsesAdapter{}
	bodyWithoutStream := []byte(`{
		"model": "gpt-4.1",
		"instructions": "Use project context.",
		"input": [{"role":"user","content":[{"type":"input_text","text":"draft reply"}]}],
		"temperature": 0.1,
		"stream": false
	}`)
	bodyWithStream := []byte(`{
		"stream": true,
		"temperature": 0.1,
		"input": [{"content":[{"text":"draft reply","type":"input_text"}],"role":"user"}],
		"instructions": "Use project context.",
		"model": "gpt-4.1"
	}`)

	digestA, err := adapter.BuildCacheDigest(RequestParseInput{
		Method:                "POST",
		Path:                  "/v1/responses",
		Body:                  bodyWithoutStream,
		ContextPolicyVersion:  "memory-policy-v1",
		InjectedContextDigest: "memory-digest-a",
	})
	require.NoError(t, err)
	digestB, err := adapter.BuildCacheDigest(RequestParseInput{
		Method:                "POST",
		Path:                  "/v1/responses",
		Body:                  bodyWithStream,
		ContextPolicyVersion:  "memory-policy-v1",
		InjectedContextDigest: "memory-digest-a",
	})
	require.NoError(t, err)

	require.Equal(t, ProtocolResponses, digestA.Input.Protocol)
	require.Equal(t, "gpt-4.1", digestA.Input.Model)
	require.Contains(t, digestA.Input.ExcludedFields, "stream")
	require.Equal(t, "memory-policy-v1", digestA.Input.ContextPolicyVersion)
	require.Equal(t, "memory-digest-a", digestA.Input.InjectedContextDigest)
	require.NotEmpty(t, digestA.Digest)
	require.Equal(t, digestA.Digest, digestB.Digest)
	require.JSONEq(t, `"Use project context."`, string(digestA.Input.Fields["instructions"]))
	require.JSONEq(t, `[{"role":"user","content":[{"type":"input_text","text":"draft reply"}]}]`, string(digestA.Input.Fields["input"]))

	digestWithDifferentContext, err := adapter.BuildCacheDigest(RequestParseInput{
		Method:                "POST",
		Path:                  "/v1/responses",
		Body:                  bodyWithStream,
		ContextPolicyVersion:  "memory-policy-v1",
		InjectedContextDigest: "memory-digest-b",
	})
	require.NoError(t, err)
	require.NotEqual(t, digestA.Digest, digestWithDifferentContext.Digest)

	digestWithDifferentPolicy, err := adapter.BuildCacheDigest(RequestParseInput{
		Method:                "POST",
		Path:                  "/v1/responses",
		Body:                  bodyWithStream,
		ContextPolicyVersion:  "memory-policy-v2",
		InjectedContextDigest: "memory-digest-a",
	})
	require.NoError(t, err)
	require.NotEqual(t, digestA.Digest, digestWithDifferentPolicy.Digest)
}

func TestResponsesAdapterSerializesReplayPayload(t *testing.T) {
	adapter := ResponsesAdapter{}
	nonStream, err := adapter.SerializeReplay(ReplayPayload{
		Protocol:    ProtocolResponses,
		StatusCode:  200,
		ContentType: "application/json; charset=utf-8",
		Body:        json.RawMessage(`{"object":"response","status":"completed","output_text":"cached answer"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "application/json; charset=utf-8", nonStream.ContentType)
	require.JSONEq(t, `{"object":"response","status":"completed","output_text":"cached answer"}`, string(nonStream.Body))

	stream, err := adapter.SerializeReplay(ReplayPayload{
		Protocol:    ProtocolResponses,
		ContentType: "text/event-stream; charset=utf-8",
		StreamChunks: []StreamChunk{{
			Protocol:   ProtocolResponses,
			Event:      "response.created",
			Data:       json.RawMessage(`{"type":"response.created","response":{"id":"resp_cached","object":"response","model":"gpt-4.1","status":"in_progress"}}`),
			Replayable: true,
		}, {
			Protocol:   ProtocolResponses,
			Event:      "response.output_item.added",
			Data:       json.RawMessage(`{"type":"response.output_item.added","output_index":0,"item":{"id":"msg_1","type":"message","role":"assistant","content":[]}}`),
			Replayable: true,
		}, {
			Protocol:   ProtocolResponses,
			Event:      "response.output_text.delta",
			Data:       json.RawMessage(`{"type":"response.output_text.delta","item_id":"msg_1","output_index":0,"content_index":0,"delta":"cached"}`),
			TextDelta:  "cached",
			Replayable: true,
		}, {
			Protocol:   ProtocolResponses,
			Event:      "response.output_text.done",
			Data:       json.RawMessage(`{"type":"response.output_text.done","item_id":"msg_1","output_index":0,"content_index":0,"text":"cached"}`),
			Replayable: true,
		}, {
			Protocol:   ProtocolResponses,
			Event:      "response.completed",
			Data:       json.RawMessage(`{"type":"response.completed","response":{"id":"resp_cached","status":"completed","usage":{"input_tokens":15,"output_tokens":1,"total_tokens":16}}}`),
			Replayable: true,
		}},
	})
	require.NoError(t, err)
	require.Equal(t, "text/event-stream; charset=utf-8", stream.ContentType)
	frames := responsesReplaySSEFrames(t, stream.Body)
	requireResponsesReplayEventOrder(t, frames, []string{
		"response.created",
		"response.output_item.added",
		"response.output_text.delta",
		"response.output_text.done",
		"response.completed",
	})
	deltaFrame := requireResponsesReplayFrame(t, frames, "response.output_text.delta")
	requireJSONEqual(t, `{"type":"response.output_text.delta","item_id":"msg_1","output_index":0,"content_index":0,"delta":"cached"}`, deltaFrame.Data)
}

func TestResponsesAdapterRejectsMismatchedReplayPayloadWithoutLeakingBody(t *testing.T) {
	adapter := ResponsesAdapter{}
	_, err := adapter.SerializeReplay(ReplayPayload{
		Protocol:    ProtocolChatCompletions,
		StatusCode:  200,
		ContentType: "application/json",
		Body:        json.RawMessage(`{"api_key":"sk-should-not-leak","content":"raw prompt should not leak"}`),
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "replay protocol mismatch")
	require.NotContains(t, err.Error(), "sk-should-not-leak")
	require.NotContains(t, err.Error(), "raw prompt")
}

type responsesReplaySSEFrame struct {
	Event string
	Data  string
}

func responsesReplaySSEFrames(t *testing.T, body []byte) []responsesReplaySSEFrame {
	t.Helper()
	rawFrames := strings.Split(strings.TrimSpace(string(body)), "\n\n")
	frames := make([]responsesReplaySSEFrame, 0, len(rawFrames))
	for _, raw := range rawFrames {
		var frame responsesReplaySSEFrame
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "event:"):
				frame.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				frame.Data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			default:
				require.Failf(t, "unexpected SSE line", "line=%q body=%s", line, string(body))
			}
		}
		require.NotEmpty(t, frame.Event)
		require.NotEmpty(t, frame.Data)
		frames = append(frames, frame)
	}
	return frames
}

func requireResponsesStreamChunk(t *testing.T, chunks []StreamChunk, event string, textDelta string) StreamChunk {
	t.Helper()
	for _, chunk := range chunks {
		if chunk.Event == event && (textDelta == "" || chunk.TextDelta == textDelta) {
			return chunk
		}
	}
	require.Failf(t, "missing Responses stream chunk", "event=%q text_delta=%q chunks=%+v", event, textDelta, chunks)
	return StreamChunk{}
}

func requireResponsesStreamEvent(t *testing.T, chunks []StreamChunk, event string) StreamChunk {
	t.Helper()
	for _, chunk := range chunks {
		if chunk.Event == event {
			return chunk
		}
	}
	require.Failf(t, "missing Responses stream event", "event=%q chunks=%+v", event, chunks)
	return StreamChunk{}
}

func requireResponsesReplayFrame(t *testing.T, frames []responsesReplaySSEFrame, event string) responsesReplaySSEFrame {
	t.Helper()
	for _, frame := range frames {
		if frame.Event == event {
			return frame
		}
	}
	require.Failf(t, "missing Responses replay frame", "event=%q frames=%+v", event, frames)
	return responsesReplaySSEFrame{}
}

func requireResponsesReplayEventOrder(t *testing.T, frames []responsesReplaySSEFrame, expected []string) {
	t.Helper()
	next := 0
	for _, frame := range frames {
		if next < len(expected) && frame.Event == expected[next] {
			next++
		}
	}
	require.Equalf(t, len(expected), next, "missing replay event sequence %v in frames %+v", expected, frames)
}
