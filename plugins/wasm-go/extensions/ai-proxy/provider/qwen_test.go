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
