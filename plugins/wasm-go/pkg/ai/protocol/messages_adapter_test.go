package protocol

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMessagesAdapterInjectsMemoryIntoSystemContentBlocks(t *testing.T) {
	adapter := MessagesAdapter{}
	injected, err := adapter.InjectContext([]byte(`{
		"model": "claude-sonnet",
		"system": [{"type":"text","text":"You are concise.","cache_control":{"type":"ephemeral"}}],
		"messages": [
			{"role": "user", "content": [{"type":"text","text":"What did we decide?"}]}
		],
		"stream": false,
		"metadata": {"trace_id":"trace-messages-a"}
	}`), InjectableContext{
		Text:      "Memory: ship on Tuesday.",
		Placement: InjectionSystem,
		ContentBlocks: []ContentBlock{{
			Type: "text",
			Text: "Memory: ship on Tuesday.",
		}},
	})

	require.NoError(t, err)
	var payload struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
		System []struct {
			Type         string `json:"type"`
			Text         string `json:"text"`
			CacheControl struct {
				Type string `json:"type"`
			} `json:"cache_control"`
		} `json:"system"`
		Messages []struct {
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"messages"`
		Metadata struct {
			TraceID string `json:"trace_id"`
		} `json:"metadata"`
	}
	require.NoError(t, json.Unmarshal(injected, &payload))
	require.Equal(t, "claude-sonnet", payload.Model)
	require.False(t, payload.Stream)
	require.Equal(t, "trace-messages-a", payload.Metadata.TraceID)
	require.Len(t, payload.System, 2)
	require.Equal(t, "You are concise.", payload.System[0].Text)
	require.Equal(t, "ephemeral", payload.System[0].CacheControl.Type)
	require.Equal(t, "Memory: ship on Tuesday.", payload.System[1].Text)
	require.Len(t, payload.Messages, 1)
	require.Equal(t, "user", payload.Messages[0].Role)
	require.Equal(t, "What did we decide?", payload.Messages[0].Content[0].Text)
}

func TestMessagesAdapterDoesNotMutateRequestWhenContextIsEmpty(t *testing.T) {
	adapter := MessagesAdapter{}
	original := []byte(`{"model":"claude-sonnet","messages":[{"role":"user","content":[{"type":"text","text":"ping"}]}],"stream":false}`)
	injected, err := adapter.InjectContext(original, InjectableContext{})

	require.NoError(t, err)
	require.JSONEq(t, string(original), string(injected))
	require.NotContains(t, string(injected), `"system":null`)
}

func TestMessagesAdapterSkipsOverBudgetMemoryInjection(t *testing.T) {
	adapter := MessagesAdapter{}
	original := []byte(`{
		"model": "claude-sonnet",
		"system": [{"type":"text","text":"You are concise."}],
		"messages": [
			{"role": "user", "content": [{"type":"text","text":"What did we decide?"}]}
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

func TestMessagesAdapterCapturesNonStreamResponse(t *testing.T) {
	adapter := MessagesAdapter{}
	exchange, err := adapter.CaptureResponse(ResponseCaptureInput{
		StatusCode: 200,
		Body: []byte(`{
			"id": "msg_fixture",
			"type": "message",
			"role": "assistant",
			"model": "claude-sonnet",
			"content": [
				{"type":"text","text":"We decided to ship on Tuesday."},
				{"type":"tool_use","id":"toolu_1","name":"lookup","input":{"topic":"launch"}}
			],
			"stop_reason": "tool_use",
			"usage": {"input_tokens": 14, "output_tokens": 7}
		}`),
	})

	require.NoError(t, err)
	require.Equal(t, ProtocolMessages, exchange.Request.Protocol)
	require.Equal(t, "We decided to ship on Tuesday.", exchange.Response.Text)
	require.Equal(t, "tool_use", exchange.Response.FinishReason)
	require.True(t, exchange.Response.ContainsToolCalls)
	require.Equal(t, Usage{InputTokens: 14, OutputTokens: 7, TotalTokens: 21}, exchange.Usage)
}

func TestMessagesAdapterCapturesMessagesStreamEvents(t *testing.T) {
	adapter := MessagesAdapter{}
	exchange, err := adapter.CaptureStream(ResponseStreamInput{
		StatusCode: 200,
		Chunks: [][]byte{
			[]byte("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_stream_fixture\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-sonnet\",\"content\":[],\"usage\":{\"input_tokens\":14,\"output_tokens\":0}}}\n\n"),
			[]byte("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n"),
			[]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Tuesday\"}}\n\n"),
			[]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\" launch\"}}\n\n"),
			[]byte("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n"),
			[]byte("event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":3}}\n\n"),
			[]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"),
		},
	})

	require.NoError(t, err)
	require.Equal(t, ProtocolMessages, exchange.Request.Protocol)
	require.Equal(t, "Tuesday launch", exchange.Response.Text)
	require.Equal(t, "end_turn", exchange.Response.FinishReason)
	require.Equal(t, Usage{InputTokens: 14, OutputTokens: 3, TotalTokens: 17}, exchange.Usage)
	require.Len(t, exchange.StreamChunks, 7)
	require.Equal(t, "content_block_delta", exchange.StreamChunks[2].Event)
	require.Equal(t, "Tuesday", exchange.StreamChunks[2].TextDelta)
	require.True(t, exchange.StreamChunks[2].Replayable)
}

func TestMessagesAdapterBuildsCacheDigest(t *testing.T) {
	adapter := MessagesAdapter{}
	bodyWithoutStream := []byte(`{
		"model": "claude-sonnet",
		"system": [{"type":"text","text":"Use project context."}],
		"messages": [{"role":"user","content":[{"type":"text","text":"summarize"}]}],
		"temperature": 0.1,
		"stream": false
	}`)
	bodyWithStream := []byte(`{
		"stream": true,
		"temperature": 0.1,
		"messages": [{"content":[{"text":"summarize","type":"text"}],"role":"user"}],
		"system": [{"text":"Use project context.","type":"text"}],
		"model": "claude-sonnet"
	}`)

	digestA, err := adapter.BuildCacheDigest(RequestParseInput{
		Method:                "POST",
		Path:                  "/v1/messages",
		Body:                  bodyWithoutStream,
		ContextPolicyVersion:  "memory-policy-v1",
		InjectedContextDigest: "memory-digest-a",
	})
	require.NoError(t, err)
	digestB, err := adapter.BuildCacheDigest(RequestParseInput{
		Method:                "POST",
		Path:                  "/v1/messages",
		Body:                  bodyWithStream,
		ContextPolicyVersion:  "memory-policy-v1",
		InjectedContextDigest: "memory-digest-a",
	})
	require.NoError(t, err)

	require.Equal(t, ProtocolMessages, digestA.Input.Protocol)
	require.Equal(t, "claude-sonnet", digestA.Input.Model)
	require.Contains(t, digestA.Input.ExcludedFields, "stream")
	require.Equal(t, "memory-policy-v1", digestA.Input.ContextPolicyVersion)
	require.Equal(t, "memory-digest-a", digestA.Input.InjectedContextDigest)
	require.NotEmpty(t, digestA.Digest)
	require.Equal(t, digestA.Digest, digestB.Digest)
	require.JSONEq(t, `[{"type":"text","text":"Use project context."}]`, string(digestA.Input.Fields["system"]))
	require.JSONEq(t, `[{"role":"user","content":[{"type":"text","text":"summarize"}]}]`, string(digestA.Input.Fields["messages"]))

	digestWithDifferentContext, err := adapter.BuildCacheDigest(RequestParseInput{
		Method:                "POST",
		Path:                  "/v1/messages",
		Body:                  bodyWithStream,
		ContextPolicyVersion:  "memory-policy-v1",
		InjectedContextDigest: "memory-digest-b",
	})
	require.NoError(t, err)
	require.NotEqual(t, digestA.Digest, digestWithDifferentContext.Digest)

	digestWithDifferentPolicy, err := adapter.BuildCacheDigest(RequestParseInput{
		Method:                "POST",
		Path:                  "/v1/messages",
		Body:                  bodyWithStream,
		ContextPolicyVersion:  "memory-policy-v2",
		InjectedContextDigest: "memory-digest-a",
	})
	require.NoError(t, err)
	require.NotEqual(t, digestA.Digest, digestWithDifferentPolicy.Digest)
}

func TestMessagesAdapterSerializesReplayPayload(t *testing.T) {
	adapter := MessagesAdapter{}
	nonStream, err := adapter.SerializeReplay(ReplayPayload{
		Protocol:    ProtocolMessages,
		StatusCode:  200,
		ContentType: "application/json; charset=utf-8",
		Body:        json.RawMessage(`{"type":"message","role":"assistant","content":[{"type":"text","text":"cached answer"}],"stop_reason":"end_turn"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "application/json; charset=utf-8", nonStream.ContentType)
	require.JSONEq(t, `{"type":"message","role":"assistant","content":[{"type":"text","text":"cached answer"}],"stop_reason":"end_turn"}`, string(nonStream.Body))

	stream, err := adapter.SerializeReplay(ReplayPayload{
		Protocol:    ProtocolMessages,
		ContentType: "text/event-stream; charset=utf-8",
		StreamChunks: []StreamChunk{{
			Protocol:   ProtocolMessages,
			Event:      "message_start",
			Data:       json.RawMessage(`{"type":"message_start","message":{"id":"msg_cached","type":"message","role":"assistant","model":"claude-sonnet","content":[],"usage":{"input_tokens":14,"output_tokens":0}}}`),
			Replayable: true,
		}, {
			Protocol:   ProtocolMessages,
			Event:      "content_block_start",
			Data:       json.RawMessage(`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`),
			Replayable: true,
		}, {
			Protocol:   ProtocolMessages,
			Event:      "content_block_delta",
			Data:       json.RawMessage(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"cached"}}`),
			TextDelta:  "cached",
			Replayable: true,
		}, {
			Protocol:   ProtocolMessages,
			Event:      "content_block_stop",
			Data:       json.RawMessage(`{"type":"content_block_stop","index":0}`),
			Replayable: true,
		}, {
			Protocol:   ProtocolMessages,
			Event:      "message_delta",
			Data:       json.RawMessage(`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":3}}`),
			Replayable: true,
		}, {
			Protocol:   ProtocolMessages,
			Event:      "message_stop",
			Data:       json.RawMessage(`{"type":"message_stop"}`),
			Replayable: true,
		}},
	})
	require.NoError(t, err)
	require.Equal(t, "text/event-stream; charset=utf-8", stream.ContentType)
	frames := messagesReplaySSEFrames(t, stream.Body)
	require.Len(t, frames, 6)
	require.Equal(t, "message_start", frames[0].Event)
	require.Equal(t, "content_block_start", frames[1].Event)
	require.Equal(t, "content_block_delta", frames[2].Event)
	require.Equal(t, "content_block_stop", frames[3].Event)
	require.Equal(t, "message_delta", frames[4].Event)
	require.Equal(t, "message_stop", frames[5].Event)
	requireJSONEqual(t, `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"cached"}}`, frames[2].Data)
}

func TestMessagesAdapterRejectsMismatchedReplayPayloadWithoutLeakingBody(t *testing.T) {
	adapter := MessagesAdapter{}
	_, err := adapter.SerializeReplay(ReplayPayload{
		Protocol:    ProtocolResponses,
		StatusCode:  200,
		ContentType: "application/json",
		Body:        json.RawMessage(`{"api_key":"sk-should-not-leak","content":"raw prompt should not leak"}`),
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "replay protocol mismatch")
	require.NotContains(t, err.Error(), "sk-should-not-leak")
	require.NotContains(t, err.Error(), "raw prompt")
}

type messagesReplaySSEFrame struct {
	Event string
	Data  string
}

func messagesReplaySSEFrames(t *testing.T, body []byte) []messagesReplaySSEFrame {
	t.Helper()
	rawFrames := strings.Split(strings.TrimSpace(string(body)), "\n\n")
	frames := make([]messagesReplaySSEFrame, 0, len(rawFrames))
	for _, raw := range rawFrames {
		var frame messagesReplaySSEFrame
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
