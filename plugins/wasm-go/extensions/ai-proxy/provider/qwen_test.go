package provider

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func newQwenMediaTestProvider() *qwenProvider {
	return &qwenProvider{
		config: ProviderConfig{
			modelMapping: map[string]string{
				"image-alias": "qwen-image-2.0-pro",
				"tts-alias":   "qwen3-tts-flash",
			},
			capabilities: map[string]string{
				string(ApiNameImageGeneration): qwenMultimodalGenerationPath,
				string(ApiNameAudioSpeech):     qwenMultimodalGenerationPath,
			},
		},
	}
}

func TestChatMessage2QwenMessagePreservesReasoningContent(t *testing.T) {
	t.Run("string content", func(t *testing.T) {
		msg := chatMessage{
			Role:             "assistant",
			Content:          "visible answer",
			ReasoningContent: "preserved reasoning",
			ToolCalls: []toolCall{
				{
					Id:   "call_1",
					Type: "function",
					Function: functionCall{
						Name:      "lookup",
						Arguments: `{"q":"weather"}`,
					},
				},
			},
		}

		qwenMsg := chatMessage2QwenMessage(msg)

		assert.Equal(t, "assistant", qwenMsg.Role)
		assert.Equal(t, "visible answer", qwenMsg.Content)
		assert.Equal(t, "preserved reasoning", qwenMsg.ReasoningContent)
		require.Len(t, qwenMsg.ToolCalls, 1)
		assert.Equal(t, "call_1", qwenMsg.ToolCalls[0].Id)
	})

	t.Run("array content", func(t *testing.T) {
		msg := chatMessage{
			Role: "assistant",
			Content: []any{
				map[string]any{
					"type": "text",
					"text": "visible answer",
				},
			},
			ReasoningContent: "preserved reasoning",
		}

		qwenMsg := chatMessage2QwenMessage(msg)

		assert.Equal(t, "assistant", qwenMsg.Role)
		assert.Equal(t, "preserved reasoning", qwenMsg.ReasoningContent)
		contents, ok := qwenMsg.Content.([]qwenVlMessageContent)
		require.True(t, ok)
		require.Len(t, contents, 1)
		assert.Equal(t, "visible answer", contents[0].Text)
	})

	t.Run("array image content", func(t *testing.T) {
		msg := chatMessage{
			Role: "assistant",
			Content: []any{
				map[string]any{
					"type": "image_url",
					"image_url": map[string]any{
						"url": "https://example.com/image.png",
					},
				},
			},
			ReasoningContent: "preserved reasoning",
		}

		qwenMsg := chatMessage2QwenMessage(msg)

		assert.Equal(t, "preserved reasoning", qwenMsg.ReasoningContent)
		contents, ok := qwenMsg.Content.([]qwenVlMessageContent)
		require.True(t, ok)
		require.Len(t, contents, 1)
		assert.Equal(t, "https://example.com/image.png", contents[0].Image)
	})
}

func TestBuildQwenTextGenerationRequestEnablesPreserveThinkingForReasoningHistory(t *testing.T) {
	provider := &qwenProvider{}
	request := &chatCompletionRequest{
		Model: "qwen3.6-plus",
		Messages: []chatMessage{
			{Role: "assistant", Content: "visible answer", ReasoningContent: "historical reasoning"},
		},
		MaxTokens: 256,
	}

	body, err := provider.buildQwenTextGenerationRequest(nil, request, false)
	require.NoError(t, err)

	var qwenRequest qwenTextGenRequest
	require.NoError(t, json.Unmarshal(body, &qwenRequest))
	assert.True(t, qwenRequest.Parameters.PreserveThinking)
}

func TestBuildQwenTextGenerationRequestOmitsPreserveThinkingForUnsupportedModel(t *testing.T) {
	provider := &qwenProvider{}
	request := &chatCompletionRequest{
		Model: "qwen-plus",
		Messages: []chatMessage{
			{Role: "assistant", Content: "visible answer", ReasoningContent: "historical reasoning"},
		},
		MaxTokens: 256,
	}

	body, err := provider.buildQwenTextGenerationRequest(nil, request, false)
	require.NoError(t, err)

	assert.False(t, gjson.GetBytes(body, "parameters.preserve_thinking").Exists())
}

func TestBuildQwenTextGenerationRequestOmitsPreserveThinkingWithoutReasoningHistory(t *testing.T) {
	provider := &qwenProvider{}
	request := &chatCompletionRequest{
		Model: "qwen-plus",
		Messages: []chatMessage{
			{Role: "assistant", Content: "visible answer"},
		},
		MaxTokens: 256,
	}

	body, err := provider.buildQwenTextGenerationRequest(nil, request, false)
	require.NoError(t, err)

	assert.False(t, gjson.GetBytes(body, "parameters.preserve_thinking").Exists())
}

func TestTransformRequestBodyHeadersCompatibleModeEnablesPreserveThinkingForReasoningHistory(t *testing.T) {
	provider := &qwenProvider{
		config: ProviderConfig{
			qwenEnableCompatible: true,
		},
	}

	body := []byte(`{
		"model":"qwen3.6-plus",
		"messages":[
			{"role":"assistant","content":"visible answer","reasoning_content":"historical reasoning"}
		]
	}`)

	modifiedBody, err := provider.TransformRequestBodyHeaders(nil, ApiNameChatCompletion, body, http.Header{})
	require.NoError(t, err)
	assert.Equal(t, true, gjson.GetBytes(modifiedBody, "preserve_thinking").Bool())
}

func TestTransformRequestBodyHeadersCompatibleModeOmitsPreserveThinkingForUnsupportedModel(t *testing.T) {
	provider := &qwenProvider{
		config: ProviderConfig{
			qwenEnableCompatible: true,
		},
	}

	body := []byte(`{
		"model":"qwen-plus",
		"messages":[
			{"role":"assistant","content":"visible answer","reasoning_content":"historical reasoning"}
		]
	}`)

	modifiedBody, err := provider.TransformRequestBodyHeaders(nil, ApiNameChatCompletion, body, http.Header{})
	require.NoError(t, err)
	assert.False(t, gjson.GetBytes(modifiedBody, "preserve_thinking").Exists())
}

func TestQwenProviderTransformImageGenerationRequestBodyHeaders(t *testing.T) {
	provider := newQwenMediaTestProvider()
	ctx := newMockMultipartHttpContext()
	headers := http.Header{}
	body := []byte(`{
		"model": "image-alias",
		"prompt": "a quiet lake at sunrise",
		"n": 2,
		"seed": 42,
		"size": "1280x720",
		"style": "vivid"
	}`)

	var modifiedBody []byte
	var err error
	require.NotPanics(t, func() {
		modifiedBody, err = provider.TransformRequestBodyHeaders(ctx, ApiNameImageGeneration, body, headers)
	})
	require.NoError(t, err)

	assert.Equal(t, "qwen-image-2.0-pro", gjson.GetBytes(modifiedBody, "model").String())
	assert.Equal(t, "user", gjson.GetBytes(modifiedBody, "input.messages.0.role").String())
	assert.Equal(t, "a quiet lake at sunrise", gjson.GetBytes(modifiedBody, "input.messages.0.content.0.text").String())
	assert.Equal(t, int64(2), gjson.GetBytes(modifiedBody, "parameters.n").Int())
	assert.Equal(t, int64(42), gjson.GetBytes(modifiedBody, "parameters.seed").Int())
	assert.Equal(t, "1280*720", gjson.GetBytes(modifiedBody, "parameters.size").String())
	assert.False(t, gjson.GetBytes(modifiedBody, "style").Exists(), "unsupported OpenAI image fields must not be forwarded")
}

func TestQwenProviderTransformImageGenerationResponseBody(t *testing.T) {
	provider := newQwenMediaTestProvider()
	body := []byte(`{
		"request_id": "req-image",
		"output": {
			"choices": [
				{
					"finish_reason": "stop",
					"message": {
						"role": "assistant",
						"content": [
							{"image": "https://dashscope.example/one.png"},
							{"image": "https://dashscope.example/two.png", "type": "image"}
						]
					}
				}
			]
		},
		"usage": {
			"input_tokens": 0,
			"output_tokens": 0,
			"total_tokens": 0
		}
	}`)

	modifiedBody, err := provider.TransformResponseBody(newMockMultipartHttpContext(), ApiNameImageGeneration, body)
	require.NoError(t, err)

	assert.Greater(t, gjson.GetBytes(modifiedBody, "created").Int(), int64(0))
	assert.Equal(t, "https://dashscope.example/one.png", gjson.GetBytes(modifiedBody, "data.0.url").String())
	assert.Equal(t, "https://dashscope.example/two.png", gjson.GetBytes(modifiedBody, "data.1.url").String())
	assert.False(t, gjson.GetBytes(modifiedBody, "output").Exists(), "DashScope native output must not leak into OpenAI image response")
}

func TestQwenProviderTransformImageGenerationResponseBodyPreservesProviderError(t *testing.T) {
	provider := newQwenMediaTestProvider()
	body := []byte(`{
		"request_id": "req-image-error",
		"code": "InvalidParameter",
		"message": "image prompt is invalid"
	}`)

	modifiedBody, err := provider.TransformResponseBody(newMockMultipartHttpContext(), ApiNameImageGeneration, body)
	require.NoError(t, err)

	assert.Equal(t, body, modifiedBody)
	assert.Equal(t, "InvalidParameter", gjson.GetBytes(modifiedBody, "code").String())
	assert.Equal(t, "image prompt is invalid", gjson.GetBytes(modifiedBody, "message").String())
	assert.False(t, gjson.GetBytes(modifiedBody, "data").Exists(), "provider errors must not be converted into OpenAI success responses")
}

func TestQwenProviderTransformAudioSpeechRequestBodyHeaders(t *testing.T) {
	provider := newQwenMediaTestProvider()
	ctx := newMockMultipartHttpContext()
	headers := http.Header{}
	body := []byte(`{
		"model": "tts-alias",
		"input": "Today is a wonderful day to build something people love.",
		"voice": "Cherry",
		"language_type": "English",
		"instructions": "Speak warmly.",
		"optimize_instructions": true,
		"response_format": "mp3",
		"speed": 1.25
	}`)

	var modifiedBody []byte
	var err error
	require.NotPanics(t, func() {
		modifiedBody, err = provider.TransformRequestBodyHeaders(ctx, ApiNameAudioSpeech, body, headers)
	})
	require.NoError(t, err)

	assert.Equal(t, "qwen3-tts-flash", gjson.GetBytes(modifiedBody, "model").String())
	assert.Equal(t, "Today is a wonderful day to build something people love.", gjson.GetBytes(modifiedBody, "input.text").String())
	assert.Equal(t, "Cherry", gjson.GetBytes(modifiedBody, "input.voice").String())
	assert.Equal(t, "English", gjson.GetBytes(modifiedBody, "input.language_type").String())
	assert.Equal(t, "Speak warmly.", gjson.GetBytes(modifiedBody, "input.instructions").String())
	assert.True(t, gjson.GetBytes(modifiedBody, "input.optimize_instructions").Bool())
	assert.False(t, gjson.GetBytes(modifiedBody, "response_format").Exists(), "unsupported OpenAI audio fields must not be forwarded")
	assert.False(t, gjson.GetBytes(modifiedBody, "speed").Exists(), "unsupported OpenAI audio fields must not be forwarded")
}

func TestQwenProviderTransformAudioSpeechResponseBody(t *testing.T) {
	provider := newQwenMediaTestProvider()
	body := []byte(`{
		"status_code": 200,
		"request_id": "req-audio",
		"code": "",
		"message": "",
		"output": {
			"text": null,
			"finish_reason": "stop",
			"choices": null,
			"audio": {
				"data": "",
				"url": "https://dashscope.example/audio.wav",
				"id": "audio_123",
				"expires_at": 1766113409
			}
		},
		"usage": {
			"input_tokens": 76,
			"output_tokens": 1045,
			"characters": 0,
			"total_tokens": 1121
		}
	}`)

	modifiedBody, err := provider.TransformResponseBody(newMockMultipartHttpContext(), ApiNameAudioSpeech, body)
	require.NoError(t, err)

	assert.Greater(t, gjson.GetBytes(modifiedBody, "created").Int(), int64(0))
	assert.Equal(t, "https://dashscope.example/audio.wav", gjson.GetBytes(modifiedBody, "data.url").String())
	assert.Equal(t, "audio_123", gjson.GetBytes(modifiedBody, "data.id").String())
	assert.Equal(t, int64(1766113409), gjson.GetBytes(modifiedBody, "data.expires_at").Int())
	assert.Equal(t, int64(76), gjson.GetBytes(modifiedBody, "usage.input_tokens").Int())
	assert.Equal(t, int64(1045), gjson.GetBytes(modifiedBody, "usage.output_tokens").Int())
	assert.Equal(t, int64(1121), gjson.GetBytes(modifiedBody, "usage.total_tokens").Int())
	assert.False(t, gjson.GetBytes(modifiedBody, "output").Exists(), "DashScope native output must not leak into gateway audio response")
}

func TestQwenProviderTransformAudioSpeechResponseBodyPreservesProviderError(t *testing.T) {
	provider := newQwenMediaTestProvider()
	body := []byte(`{
		"status_code": 200,
		"request_id": "req-audio-error",
		"code": "InvalidParameter",
		"message": "audio voice is invalid"
	}`)

	modifiedBody, err := provider.TransformResponseBody(newMockMultipartHttpContext(), ApiNameAudioSpeech, body)
	require.NoError(t, err)

	assert.Equal(t, body, modifiedBody)
	assert.Equal(t, "InvalidParameter", gjson.GetBytes(modifiedBody, "code").String())
	assert.Equal(t, "audio voice is invalid", gjson.GetBytes(modifiedBody, "message").String())
	assert.False(t, gjson.GetBytes(modifiedBody, "data").Exists(), "provider errors must not be converted into audio URL success responses")
}

func TestTransformRequestBodyHeadersCompatibleModeEnablesPreserveThinkingAfterModelMapping(t *testing.T) {
	provider := &qwenProvider{
		config: ProviderConfig{
			qwenEnableCompatible: true,
			modelMapping: map[string]string{
				"alias-model": "qwen3.6-plus-2026-04-02",
			},
		},
	}

	body := []byte(`{
		"model":"alias-model",
		"messages":[
			{"role":"assistant","content":"visible answer","reasoning_content":"historical reasoning"}
		]
	}`)

	modifiedBody, err := provider.TransformRequestBodyHeaders(nil, ApiNameChatCompletion, body, http.Header{})
	require.NoError(t, err)
	assert.Equal(t, "qwen3.6-plus-2026-04-02", gjson.GetBytes(modifiedBody, "model").String())
	assert.Equal(t, true, gjson.GetBytes(modifiedBody, "preserve_thinking").Bool())
}

func TestTransformRequestBodyHeadersCompatibleModeOmitsPreserveThinkingWithoutReasoningHistory(t *testing.T) {
	provider := &qwenProvider{
		config: ProviderConfig{
			qwenEnableCompatible: true,
		},
	}

	body := []byte(`{
		"model":"qwen-plus",
		"messages":[
			{"role":"assistant","content":"visible answer"}
		]
	}`)

	modifiedBody, err := provider.TransformRequestBodyHeaders(nil, ApiNameChatCompletion, body, http.Header{})
	require.NoError(t, err)
	assert.False(t, gjson.GetBytes(modifiedBody, "preserve_thinking").Exists())
}

func TestQwenNativeToolCallMultiTurnAndCumulativeStreamingFidelity(t *testing.T) {
	provider := &qwenProvider{}
	parallel := true
	request := &chatCompletionRequest{
		Model:               "qwen-flash",
		MaxCompletionTokens: 320,
		ClaudeThinking:      &claudeThinkingConfig{Type: "enabled", BudgetTokens: 2048},
		ToolChoice:          "auto",
		ParallelToolCalls:   &parallel,
		Messages: []chatMessage{
			{Role: "user", Content: "Check Hangzhou weather"},
			{
				Role:             "assistant",
				Content:          nil,
				ReasoningContent: "Call weather tool",
				ToolCalls: []toolCall{{
					Id:       "call_weather_1",
					Type:     "function",
					Function: functionCall{Name: "get_weather", Arguments: `{"city":"Hangzhou"}`},
				}},
			},
			{
				Role:       "tool",
				ToolCallId: "call_weather_1",
				Content:    []any{map[string]any{"type": "text", "text": "24C sunny"}},
			},
		},
		Tools: []tool{{Type: "function", Function: function{Name: "get_weather"}}},
	}

	raw, err := provider.buildQwenTextGenerationRequest(nil, request, false)
	require.NoError(t, err)
	assert.Equal(t, int64(320), gjson.GetBytes(raw, "parameters.max_tokens").Int())
	assert.True(t, gjson.GetBytes(raw, "parameters.enable_thinking").Bool())
	assert.Equal(t, int64(2048), gjson.GetBytes(raw, "parameters.thinking_budget").Int())
	assert.Equal(t, "auto", gjson.GetBytes(raw, "parameters.tool_choice").String())
	assert.True(t, gjson.GetBytes(raw, "parameters.parallel_tool_calls").Bool())
	assert.False(t, gjson.GetBytes(raw, "parameters.top_p").Exists())
	assert.Equal(t, gjson.String, gjson.GetBytes(raw, "input.messages.1.content").Type)
	assert.Equal(t, "call_weather_1", gjson.GetBytes(raw, "input.messages.2.tool_call_id").String())
	assert.Equal(t, "24C sunny", gjson.GetBytes(raw, "input.messages.2.content").String())

	ctx := newMockMultipartHttpContext()
	ctx.SetContext(ctxKeyFinalRequestModel, "qwen-flash")
	ctx.SetContext(ctxKeyIncrementalStreaming, false)

	events0, err := provider.OnStreamingEvent(ctx, ApiNameChatCompletion, StreamEvent{
		Event: "result",
		Data:  `{"request_id":"req-1","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","content":"","tool_calls":[{"index":0,"id":"call_weather_1","type":"function","function":{"name":"get_weather","arguments":""}}]}}]},"usage":{"input_tokens":20,"output_tokens":1,"total_tokens":21}}`,
	})
	require.NoError(t, err)
	assert.Empty(t, events0, "empty-arguments intermediate frame in cumulative mode is buffered without error")

	events1, err := provider.OnStreamingEvent(ctx, ApiNameChatCompletion, StreamEvent{
		Event: "result",
		Data:  `{"request_id":"req-1","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","content":"","tool_calls":[{"index":0,"id":"call_weather_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":"}}]}}]},"usage":{"input_tokens":20,"output_tokens":5,"total_tokens":25}}`,
	})
	require.NoError(t, err)
	require.Len(t, events1, 1)
	assert.Empty(t, events1[0].Event)
	assert.Equal(t, "call_weather_1", gjson.Get(events1[0].Data, "choices.0.delta.tool_calls.0.id").String())
	assert.Equal(t, "get_weather", gjson.Get(events1[0].Data, "choices.0.delta.tool_calls.0.function.name").String())
	assert.Equal(t, `{"city":`, gjson.Get(events1[0].Data, "choices.0.delta.tool_calls.0.function.arguments").String())

	events2, err := provider.OnStreamingEvent(ctx, ApiNameChatCompletion, StreamEvent{
		Event: "result",
		Data:  `{"request_id":"req-1","output":{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","tool_calls":[{"index":0,"id":"call_weather_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"Hangzhou\"}"}}]}}]},"usage":{"input_tokens":20,"output_tokens":9,"total_tokens":29}}`,
	})
	require.NoError(t, err)
	require.Len(t, events2, 3)
	assert.Empty(t, gjson.Get(events2[0].Data, "choices.0.delta.tool_calls.0.id").String(), "subsequent delta chunk must not duplicate tool_call id")
	assert.Empty(t, gjson.Get(events2[0].Data, "choices.0.delta.tool_calls.0.function.name").String())
	assert.Equal(t, `"Hangzhou"}`, gjson.Get(events2[0].Data, "choices.0.delta.tool_calls.0.function.arguments").String())
	assert.Equal(t, "tool_calls", gjson.Get(events2[1].Data, "choices.0.finish_reason").String())
}

func TestQwenStreamingThinkingWithToolsEnablesIncrementalOutput(t *testing.T) {
	provider := &qwenProvider{}
	ctx := newMockMultipartHttpContext()
	ctx.SetContext(ctxKeyFinalRequestModel, "qwen-flash")
	request := &chatCompletionRequest{
		Model:          "qwen-flash",
		Stream:         true,
		ClaudeThinking: &claudeThinkingConfig{Type: "enabled", BudgetTokens: 1024},
		Messages:       []chatMessage{{Role: "user", Content: "Read AGENTS.md"}},
		Tools:          []tool{{Type: "function", Function: function{Name: "read_file"}}},
	}

	raw, err := provider.buildQwenGenerationRequest(ctx, request, nil, true, false)
	require.NoError(t, err)
	assert.True(t, gjson.GetBytes(raw, "parameters.enable_thinking").Bool())
	assert.True(t, gjson.GetBytes(raw, "parameters.incremental_output").Bool(), "DashScope requires incremental_output=true when enable_thinking=true under stream mode")
	assert.True(t, ctx.GetBoolContext(ctxKeyIncrementalStreaming, false))

	evThink, err := provider.OnStreamingEvent(ctx, ApiNameChatCompletion, StreamEvent{
		Event: "result",
		Data:  `{"request_id":"req-inc","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","content":"","reasoning_content":"Reading AGENTS.md..."}}]},"usage":{"input_tokens":50,"output_tokens":4,"total_tokens":54}}`,
	})
	require.NoError(t, err)
	require.Len(t, evThink, 1)
	assert.Equal(t, "Reading AGENTS.md...", gjson.Get(evThink[0].Data, "choices.0.delta.reasoning_content").String())

	evStart, err := provider.OnStreamingEvent(ctx, ApiNameChatCompletion, StreamEvent{
		Event: "result",
		Data:  `{"request_id":"req-inc","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","content":"","reasoning_content":"","tool_calls":[{"index":0,"id":"call_rf_1","type":"function","function":{"name":"read_file","arguments":""}}]}}]},"usage":{"input_tokens":50,"output_tokens":8,"total_tokens":58}}`,
	})
	require.NoError(t, err)
	require.Len(t, evStart, 1)
	assert.Equal(t, "call_rf_1", gjson.Get(evStart[0].Data, "choices.0.delta.tool_calls.0.id").String())
	assert.Equal(t, "read_file", gjson.Get(evStart[0].Data, "choices.0.delta.tool_calls.0.function.name").String())
	assert.Equal(t, "", gjson.Get(evStart[0].Data, "choices.0.delta.tool_calls.0.function.arguments").String())

	evArg, err := provider.OnStreamingEvent(ctx, ApiNameChatCompletion, StreamEvent{
		Event: "result",
		Data:  `{"request_id":"req-inc","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","content":"","reasoning_content":"","tool_calls":[{"index":0,"id":"","type":"function","function":{"arguments":"{\"path\":\"AGENTS.md\"}"}}]}}]},"usage":{"input_tokens":50,"output_tokens":14,"total_tokens":64}}`,
	})
	require.NoError(t, err)
	require.Len(t, evArg, 1)
	assert.Empty(t, gjson.Get(evArg[0].Data, "choices.0.delta.tool_calls.0.id").String())
	assert.Empty(t, gjson.Get(evArg[0].Data, "choices.0.delta.tool_calls.0.function.name").String())
	assert.Equal(t, `{"path":"AGENTS.md"}`, gjson.Get(evArg[0].Data, "choices.0.delta.tool_calls.0.function.arguments").String())

	evFin, err := provider.OnStreamingEvent(ctx, ApiNameChatCompletion, StreamEvent{
		Event: "result",
		Data:  `{"request_id":"req-inc","output":{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","reasoning_content":"","tool_calls":[{"index":0,"type":"function","function":{}}]}}]},"usage":{"input_tokens":50,"output_tokens":14,"total_tokens":64}}`,
	})
	require.NoError(t, err)
	require.Len(t, evFin, 2, "final empty function:{} must not emit a duplicate tool_call chunk before finish_reason")
	assert.Equal(t, "tool_calls", gjson.Get(evFin[0].Data, "choices.0.finish_reason").String())
}

func TestQwenCompatibleSanitizesAgentBridgingArtifacts(t *testing.T) {
	provider := &qwenProvider{
		config: ProviderConfig{
			qwenEnableCompatible: true,
			modelMapping: map[string]string{
				"public-qwen": "qwen3.6-plus",
			},
		},
	}

	body := []byte(`{
		"model": "public-qwen",
		"stream": true,
		"stream_options": {"include_usage": true},
		"store": false,
		"prompt_cache_key": "pck_hermes_1",
		"prompt_cache_retention": "24h",
		"prompt_cache_options": {"mode": "explicit"},
		"reasoning_summary": "auto",
		"safety_identifier": "user_1",
		"context_management": [{"type": "compaction"}],
		"max_completion_tokens": 1800,
		"reasoning_effort": "medium",
		"tools": [{
			"type": "function",
			"cache_control": {"type": "ephemeral"},
			"function": {"name": "read_file", "parameters": {"type": "object"}}
		}],
		"messages": [
			{"role": "developer", "content": "Be concise."},
			{"role": "user", "content": [{"type": "text", "text": "Inspect", "cache_control": {"type": "ephemeral"}}]},
			{
				"role": "assistant",
				"content": null,
				"reasoning_content": "Thinking trace",
				"reasoning_details": [{"type": "reasoning.text", "text": "Thinking trace"}],
				"thinking_blocks": [{"type": "thinking", "thinking": "Thinking trace", "signature": "sig_1"}],
				"cache_control": {"type": "ephemeral"},
				"tool_calls": [{
					"id": "call_1",
					"type": "function",
					"extra_content": {"google": {"thought_signature": "sig"}},
					"function": {"name": "read_file", "arguments": "{}"}
				}]
			},
			{"role": "tool", "tool_call_id": "call_1", "name": "read_file", "content": "ok"}
		]
	}`)

	out, err := provider.TransformRequestBodyHeaders(nil, ApiNameChatCompletion, body, http.Header{})
	require.NoError(t, err)

	assert.Equal(t, "qwen3.6-plus", gjson.GetBytes(out, "model").String())
	assert.True(t, gjson.GetBytes(out, "preserve_thinking").Bool())
	assert.Equal(t, int64(1800), gjson.GetBytes(out, "max_completion_tokens").Int())
	assert.Equal(t, "medium", gjson.GetBytes(out, "reasoning_effort").String())

	for _, droppedTop := range []string{"store", "prompt_cache_key", "prompt_cache_retention", "prompt_cache_options", "reasoning_summary", "safety_identifier", "context_management"} {
		assert.False(t, gjson.GetBytes(out, droppedTop).Exists(), "compatible mode must strip %s", droppedTop)
	}
	assert.False(t, gjson.GetBytes(out, "tools.0.cache_control").Exists())
	assert.Equal(t, "system", gjson.GetBytes(out, "messages.0.role").String())
	assert.False(t, gjson.GetBytes(out, "messages.1.content.0.cache_control").Exists())
	assert.Equal(t, gjson.String, gjson.GetBytes(out, "messages.2.content").Type, "assistant null content must normalize to empty string")
	assert.Equal(t, "", gjson.GetBytes(out, "messages.2.content").String())
	assert.Equal(t, "Thinking trace", gjson.GetBytes(out, "messages.2.reasoning_content").String())
	assert.False(t, gjson.GetBytes(out, "messages.2.reasoning_details").Exists())
	assert.False(t, gjson.GetBytes(out, "messages.2.thinking_blocks").Exists())
	assert.False(t, gjson.GetBytes(out, "messages.2.cache_control").Exists())
	assert.False(t, gjson.GetBytes(out, "messages.2.tool_calls.0.extra_content").Exists())
	assert.False(t, gjson.GetBytes(out, "messages.3.name").Exists(), "tool message name must be stripped")
}

func TestQwenDashScopeExplicitZerosAndToolChoiceThinkingGuard(t *testing.T) {
	provider := &qwenProvider{
		config: ProviderConfig{
			typ:                  providerTypeQwen,
			qwenEnableCompatible: false,
			managedRequest:       true,
		},
	}

	t.Run("preserves_explicit_zero_temperature_and_presence_penalty", func(t *testing.T) {
		ctx := newMockMultipartHttpContext()
		body := []byte(`{
			"model": "qwen-plus",
			"temperature": 0,
			"presence_penalty": 0,
			"messages": [
				{"role": "developer", "content": "System policy"},
				{"role": "user", "content": "Hello"}
			],
			"tools": [{"type": "function", "function": {"name": "ping", "parameters": {"type": "object"}}}],
			"tool_choice": "required"
		}`)
		out, err := provider.TransformRequestBodyHeaders(ctx, ApiNameChatCompletion, body, http.Header{})
		require.NoError(t, err)
		assert.True(t, gjson.GetBytes(out, "parameters.temperature").Exists(), "explicit temperature=0 must survive")
		assert.Equal(t, float64(0), gjson.GetBytes(out, "parameters.temperature").Float())
		assert.True(t, gjson.GetBytes(out, "parameters.presence_penalty").Exists(), "explicit presence_penalty=0 must survive")
		assert.Equal(t, float64(0), gjson.GetBytes(out, "parameters.presence_penalty").Float())
		assert.Equal(t, "system", gjson.GetBytes(out, "input.messages.0.role").String())
		assert.Equal(t, "required", gjson.GetBytes(out, "parameters.tool_choice").String())
	})

	t.Run("omits_absent_temperature_and_presence_penalty", func(t *testing.T) {
		ctx := newMockMultipartHttpContext()
		body := []byte(`{
			"model": "qwen-plus",
			"messages": [{"role": "user", "content": "Hello"}]
		}`)
		out, err := provider.TransformRequestBodyHeaders(ctx, ApiNameChatCompletion, body, http.Header{})
		require.NoError(t, err)
		assert.False(t, gjson.GetBytes(out, "parameters.temperature").Exists())
		assert.False(t, gjson.GetBytes(out, "parameters.presence_penalty").Exists())
	})

	t.Run("rejects_forced_tool_choice_when_thinking_enabled", func(t *testing.T) {
		for _, tc := range []string{`"required"`, `{"type":"function","function":{"name":"ping"}}`} {
			ctx := newMockMultipartHttpContext()
			body := []byte(`{
				"model": "qwen-plus",
				"enable_thinking": true,
				"messages": [{"role": "user", "content": "Hello"}],
				"tools": [{"type": "function", "function": {"name": "ping", "parameters": {"type": "object"}}}],
				"tool_choice": ` + tc + `
			}`)
			_, err := provider.TransformRequestBodyHeaders(ctx, ApiNameChatCompletion, body, http.Header{})
			require.Error(t, err, "DashScope thinking + forced tool_choice %s must be rejected explicitly", tc)
		}
	})
}
