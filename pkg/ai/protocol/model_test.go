package protocol

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectProtocolKindFromRuntimePath(t *testing.T) {
	require.Equal(t, ProtocolChatCompletions, DetectProtocolKind("/v1/chat/completions"))
	require.Equal(t, ProtocolChatCompletions, DetectProtocolKind("/proxy/v1/chat/completions?api-version=2026-06-30"))
	require.Equal(t, ProtocolMessages, DetectProtocolKind("/v1/messages"))
	require.Equal(t, ProtocolResponses, DetectProtocolKind("/v1/responses"))
	require.Equal(t, ProtocolUnknown, DetectProtocolKind("/v1/embeddings"))
}

func TestNormalizedExchangeModelCarriesSharedAdapterFacts(t *testing.T) {
	usage := Usage{
		InputTokens:          19,
		InputCacheHitTokens:  7,
		InputCacheMissTokens: 12,
		OutputTokens:         11,
		TotalTokens:          30,
	}
	replayChunk := StreamChunk{
		Protocol:     ProtocolMessages,
		Event:        "content_block_delta",
		Data:         json.RawMessage(`{"type":"content_block_delta","delta":{"type":"text_delta","text":"cached"}}`),
		TextDelta:    "cached",
		FinishReason: "end_turn",
		Usage:        &usage,
		Replayable:   true,
	}
	upstreamInvoked := false
	exchange := NormalizedExchange{
		Request: RequestFacts{
			Protocol:  ProtocolMessages,
			Method:    "POST",
			Path:      "/v1/messages",
			Route:     "ai-route-claude",
			Tenant:    "tenant-a",
			Consumer:  "consumer-a",
			SessionID: "session-a",
			RequestID: "request-a",
			Model:     "claude-sonnet",
			Stream:    true,
			NoStore:   false,
		},
		CurrentUserPrompt: CurrentUserPrompt{
			Text:       "What did we decide?",
			SourcePath: "messages[1].content[0].text",
		},
		InjectableContext: InjectableContext{
			Text:      "Remember the launch date.",
			Placement: InjectionSystem,
			Messages:  []Message{{Role: "system", Content: "Remember the launch date."}},
			ContentBlocks: []ContentBlock{{
				Type: "text",
				Text: "Remember the launch date.",
			}},
			TokenEstimate: 6,
			Digest:        "memory-digest-a",
			Source:        "console-memory",
		},
		CacheDigest: CacheDigestInput{
			Protocol:              ProtocolMessages,
			Model:                 "claude-sonnet",
			Fields:                map[string]json.RawMessage{"messages": json.RawMessage(`[{"role":"user","content":"What did we decide?"}]`)},
			ExcludedFields:        []string{"stream"},
			ContextPolicyVersion:  "memory-policy-v1",
			InjectedContextDigest: "memory-digest-a",
		},
		Response: ResponseText{
			Text:              "cached answer",
			FinishReason:      "end_turn",
			ContainsToolCalls: false,
			ParseFailed:       false,
			UnsafeContent:     false,
		},
		Usage:        usage,
		StreamChunks: []StreamChunk{replayChunk},
		Replay: ReplayPayload{
			Protocol:        ProtocolMessages,
			StatusCode:      200,
			ContentType:     "text/event-stream; charset=utf-8",
			Body:            json.RawMessage(`{"type":"message","content":[{"type":"text","text":"cached answer"}]}`),
			StreamChunks:    []StreamChunk{replayChunk},
			UpstreamInvoked: &upstreamInvoked,
		},
	}

	body, err := json.Marshal(exchange)
	require.NoError(t, err)

	var decoded NormalizedExchange
	require.NoError(t, json.Unmarshal(body, &decoded))
	require.Equal(t, ProtocolMessages, decoded.Request.Protocol)
	require.Equal(t, "What did we decide?", decoded.CurrentUserPrompt.Text)
	require.Equal(t, "memory-digest-a", decoded.InjectableContext.Digest)
	require.Equal(t, InjectionSystem, decoded.InjectableContext.Placement)
	require.Equal(t, "Remember the launch date.", decoded.InjectableContext.ContentBlocks[0].Text)
	require.Equal(t, "memory-policy-v1", decoded.CacheDigest.ContextPolicyVersion)
	require.Equal(t, "memory-digest-a", decoded.CacheDigest.InjectedContextDigest)
	require.Equal(t, json.RawMessage(`[{"role":"user","content":"What did we decide?"}]`), decoded.CacheDigest.Fields["messages"])
	require.Equal(t, usage, decoded.Usage)
	require.Len(t, decoded.StreamChunks, 1)
	require.Equal(t, "content_block_delta", decoded.StreamChunks[0].Event)
	require.Equal(t, usage, *decoded.StreamChunks[0].Usage)
	require.NotNil(t, decoded.Replay.UpstreamInvoked)
	require.False(t, *decoded.Replay.UpstreamInvoked)
}

func TestReplayPayloadValidationRequiresProtocolSpecificPayload(t *testing.T) {
	require.NoError(t, ReplayPayload{
		Protocol:    ProtocolChatCompletions,
		StatusCode:  200,
		ContentType: "application/json",
		Body:        json.RawMessage(`{"choices":[{"message":{"content":"cached"}}]}`),
	}.Validate(false))

	require.NoError(t, ReplayPayload{
		Protocol: ProtocolResponses,
		StreamChunks: []StreamChunk{{
			Protocol:   ProtocolResponses,
			Event:      "response.output_text.delta",
			Data:       json.RawMessage(`{"type":"response.output_text.delta","delta":"cached"}`),
			TextDelta:  "cached",
			Replayable: true,
		}},
	}.Validate(true))

	require.EqualError(t, ReplayPayload{Protocol: ProtocolUnknown, Body: json.RawMessage(`{}`)}.Validate(false), "replay protocol is required")
	require.EqualError(t, ReplayPayload{Protocol: ProtocolChatCompletions}.Validate(false), "non-stream replay body is required")
	require.EqualError(t, ReplayPayload{Protocol: ProtocolResponses}.Validate(true), "stream replay chunks are required")
}

func TestStreamChunkOmitsEmptyUsage(t *testing.T) {
	body, err := json.Marshal(StreamChunk{
		Protocol:   ProtocolResponses,
		Event:      "response.output_text.delta",
		Data:       json.RawMessage(`{"type":"response.output_text.delta","delta":"cached"}`),
		TextDelta:  "cached",
		Replayable: true,
	})

	require.NoError(t, err)
	require.NotContains(t, string(body), `"usage"`)
}

func TestNormalizedExchangeGoldenPayloadsCoverSupportedProtocols(t *testing.T) {
	chat := NormalizedExchange{
		Request: RequestFacts{
			Protocol: ProtocolChatCompletions,
			Method:   "POST",
			Path:     "/v1/chat/completions",
			Model:    "qwen-turbo",
		},
		CurrentUserPrompt: CurrentUserPrompt{Text: "weather?"},
		InjectableContext: InjectableContext{
			Text:      "Use Shanghai context.",
			Placement: InjectionSystem,
			Messages:  []Message{{Role: "system", Content: "Use Shanghai context."}},
		},
		CacheDigest: CacheDigestInput{
			Protocol:       ProtocolChatCompletions,
			Model:          "qwen-turbo",
			Fields:         map[string]json.RawMessage{"messages": json.RawMessage(`[{"role":"user","content":"weather?"}]`)},
			ExcludedFields: []string{"stream"},
		},
		Response: ResponseText{Text: "sunny", FinishReason: "stop"},
		Replay: ReplayPayload{
			Protocol:    ProtocolChatCompletions,
			StatusCode:  200,
			ContentType: "application/json",
			Body:        json.RawMessage(`{"choices":[{"message":{"role":"assistant","content":"sunny"},"finish_reason":"stop"}]}`),
		},
	}

	messages := NormalizedExchange{
		Request: RequestFacts{
			Protocol: ProtocolMessages,
			Method:   "POST",
			Path:     "/v1/messages",
			Model:    "claude-sonnet",
			Stream:   true,
		},
		CurrentUserPrompt: CurrentUserPrompt{Text: "summarize"},
		InjectableContext: InjectableContext{
			Text:      "Use account context.",
			Placement: InjectionSystem,
			ContentBlocks: []ContentBlock{{
				Type: "text",
				Text: "Use account context.",
			}},
		},
		CacheDigest: CacheDigestInput{
			Protocol:              ProtocolMessages,
			Model:                 "claude-sonnet",
			Fields:                map[string]json.RawMessage{"messages": json.RawMessage(`[{"role":"user","content":[{"type":"text","text":"summarize"}]}]`)},
			ExcludedFields:        []string{"stream"},
			ContextPolicyVersion:  "context-policy-v1",
			InjectedContextDigest: "context-digest-a",
		},
		Response: ResponseText{Text: "summary", FinishReason: "end_turn"},
		StreamChunks: []StreamChunk{{
			Protocol:     ProtocolMessages,
			Event:        "content_block_delta",
			Data:         json.RawMessage(`{"type":"content_block_delta","delta":{"type":"text_delta","text":"summary"}}`),
			TextDelta:    "summary",
			FinishReason: "end_turn",
			Replayable:   true,
		}},
		Replay: ReplayPayload{Protocol: ProtocolMessages, StreamChunks: []StreamChunk{{
			Protocol:   ProtocolMessages,
			Event:      "content_block_delta",
			Data:       json.RawMessage(`{"type":"content_block_delta","delta":{"type":"text_delta","text":"summary"}}`),
			TextDelta:  "summary",
			Replayable: true,
		}}},
	}

	responses := NormalizedExchange{
		Request: RequestFacts{
			Protocol: ProtocolResponses,
			Method:   "POST",
			Path:     "/v1/responses",
			Model:    "gpt-4.1",
		},
		CurrentUserPrompt: CurrentUserPrompt{Text: "draft reply", SourcePath: "input[0].content[0].text"},
		InjectableContext: InjectableContext{
			Text:      "Prefer concise language.",
			Placement: InjectionInstructions,
		},
		CacheDigest: CacheDigestInput{
			Protocol:       ProtocolResponses,
			Model:          "gpt-4.1",
			Fields:         map[string]json.RawMessage{"input": json.RawMessage(`[{"role":"user","content":[{"type":"input_text","text":"draft reply"}]}]`)},
			ExcludedFields: []string{"stream"},
		},
		Response: ResponseText{Text: "drafted", FinishReason: "completed"},
		Replay: ReplayPayload{
			Protocol:    ProtocolResponses,
			StatusCode:  200,
			ContentType: "application/json",
			Body:        json.RawMessage(`{"output_text":"drafted"}`),
		},
	}

	requireNormalizedExchangeJSON(t, chat, `"protocol":"chat_completions"`, `"messages"`, `"replay"`)
	requireNormalizedExchangeJSON(t, messages, `"protocol":"messages"`, `"content_blocks"`, `"content_block_delta"`)
	requireNormalizedExchangeJSON(t, responses, `"protocol":"responses"`, `"instructions"`, `"output_text"`)
}

func requireNormalizedExchangeJSON(t *testing.T, exchange NormalizedExchange, required ...string) {
	t.Helper()
	body, err := json.Marshal(exchange)
	require.NoError(t, err)
	for _, value := range required {
		require.Contains(t, string(body), value)
	}
}

func TestParsePublishedTextProtocolKeepsDeclaredKindAcrossRouteAliases(t *testing.T) {
	cases := []struct {
		clientProtocol string
		kind           ProtocolKind
		body           string
		build          func(RequestParseInput) (CacheDigestResult, error)
	}{
		{"openai_chat", ProtocolChatCompletions, `{"model":"original-model","messages":[{"role":"user","content":"hello"}]}`, ChatCompletionsAdapter{}.BuildCacheDigest},
		{"openai_responses", ProtocolResponses, `{"model":"original-model","input":"hello"}`, ResponsesAdapter{}.BuildCacheDigest},
		{"anthropic_messages", ProtocolMessages, `{"model":"original-model","messages":[{"role":"user","content":"hello"}]}`, MessagesAdapter{}.BuildCacheDigest},
	}
	for _, example := range cases {
		t.Run(example.clientProtocol, func(t *testing.T) {
			kind, err := ParsePublishedTextProtocol(example.clientProtocol, "text_generate")
			require.NoError(t, err)
			require.Equal(t, example.kind, kind)
			var originalDigest string
			for _, path := range []string{"/custom/sales-alias", "/provider/private/remapped", "/v1/embeddings"} {
				// Existing adapters consume the configured protocol. Even a path
				// associated with another operation cannot reinterpret this input.
				result, err := example.build(RequestParseInput{Method: "POST", Path: path, Body: []byte(example.body)})
				require.NoError(t, err)
				require.Equal(t, kind, result.Input.Protocol)
				if originalDigest == "" {
					originalDigest = result.Digest
				}
				require.Equal(t, originalDigest, result.Digest)
			}
		})
	}
}

func TestParsePublishedTextProtocolDoesNotTreatOtherOperationsAsText(t *testing.T) {
	for _, operation := range []string{"", "embedding", "image_generate", "video_generate", "task_query", "unknown", " text_generate", "text_generate "} {
		kind, err := ParsePublishedTextProtocol("openai_chat", operation)
		require.Error(t, err, operation)
		require.Equal(t, ProtocolUnknown, kind, operation)
	}
}

func TestParsePublishedTextProtocolDoesNotInferOrNormalizeUnknownProtocols(t *testing.T) {
	for _, clientProtocol := range []string{"", "unknown", "chat_completions", "messages", "responses", "OPENAI_CHAT", "openai_chat ", "/v1/chat/completions"} {
		kind, err := ParsePublishedTextProtocol(clientProtocol, "text_generate")
		require.Error(t, err, clientProtocol)
		require.Equal(t, ProtocolUnknown, kind, clientProtocol)
	}
	// The compatibility detector remains available to explicit legacy callers.
	require.Equal(t, ProtocolChatCompletions, DetectProtocolKind("/legacy/v1/chat/completions"))
	require.Equal(t, ProtocolUnknown, DetectProtocolKind("/custom/sales-alias"))
}
