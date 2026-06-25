package main

import (
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestStructuredCacheHitResponse(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("non-streaming hit preserves structured OpenAI response facts", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})

			requestBody := `{
				"model": "qwen-plus",
				"messages": [
					{
						"role": "user",
						"content": "今天天气怎么样？"
					}
				],
				"stream": false
			}`
			require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody([]byte(requestBody)))

			payload, err := EncodeRedisCacheEntryPayload(StructuredCacheEntry{
				Version: StructuredCacheEntryVersion,
				Answer: StructuredCacheAnswer{
					Role:    "assistant",
					Content: "今天北京天气晴朗，温度25度",
				},
				Model: "qwen-plus",
				Usage: StructuredCacheUsage{
					PromptTokens:     42,
					CompletionTokens: 17,
					TotalTokens:      59,
					PromptTokensDetails: &StructuredPromptTokenDetail{
						CachedTokens: 42,
					},
				},
				FinishReason: "stop",
				Scope: StructuredCacheScope{
					Type: "global",
				},
				RequestFingerprint: RequestFingerprint{
					Hash: "fingerprint-123",
				},
			})
			require.NoError(t, err)

			host.CallOnRedisCall(0, test.CreateRedisRespArray([]interface{}{payload}))

			localResponse := host.GetLocalResponse()
			require.NotNil(t, localResponse)
			require.Equal(t, uint32(200), localResponse.StatusCode)
			require.Equal(t, "ai-cache.hit", localResponse.StatusCodeDetail)
			require.Contains(t, localResponse.Headers, [2]string{"content-type", "application/json; charset=utf-8"})
			require.JSONEq(t, `{
				"id": "chatcmpl-cache-test-request-id-default",
				"object": "chat.completion",
				"model": "qwen-plus",
				"choices": [
					{
						"index": 0,
						"message": {
							"role": "assistant",
							"content": "今天北京天气晴朗，温度25度"
						},
						"finish_reason": "stop"
					}
				],
				"usage": {
					"prompt_tokens": 42,
					"completion_tokens": 17,
					"total_tokens": 59,
					"prompt_tokens_details": {
						"cached_tokens": 42
					}
				}
			}`, string(localResponse.Data))
			require.NotContains(t, string(localResponse.Data), "from-cache")
			require.NotContains(t, string(localResponse.Data), "upstream_invoked")
		})

		t.Run("streaming hit replays structured OpenAI SSE chunks", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})

			requestBody := `{
				"model": "qwen-plus",
				"messages": [
					{
						"role": "user",
						"content": "今天天气怎么样？"
					}
				],
				"stream": true
			}`
			require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody([]byte(requestBody)))

			payload, err := EncodeRedisCacheEntryPayload(StructuredCacheEntry{
				Version: StructuredCacheEntryVersion,
				Answer: StructuredCacheAnswer{
					Role:    "assistant",
					Content: "今天北京天气晴朗，温度25度",
				},
				Model: "qwen-plus",
				Choices: []StructuredCacheChoice{
					{
						Index:        0,
						Message:      StructuredCacheAnswer{Role: "assistant", Content: "今天北京天气晴朗，温度25度"},
						FinishReason: "stop",
					},
				},
				Usage: StructuredCacheUsage{
					PromptTokens:     42,
					CompletionTokens: 17,
					TotalTokens:      59,
					PromptTokensDetails: &StructuredPromptTokenDetail{
						CachedTokens: 42,
					},
				},
				FinishReason: "stop",
				Scope: StructuredCacheScope{
					Type: "global",
				},
				RequestFingerprint: RequestFingerprint{
					Hash: "fingerprint-123",
				},
			})
			require.NoError(t, err)

			host.CallOnRedisCall(0, test.CreateRedisRespArray([]interface{}{payload}))

			localResponse := host.GetLocalResponse()
			require.NotNil(t, localResponse)
			require.Equal(t, uint32(200), localResponse.StatusCode)
			require.Equal(t, "ai-cache.hit", localResponse.StatusCodeDetail)
			require.Contains(t, localResponse.Headers, [2]string{"content-type", "text/event-stream; charset=utf-8"})

			body := string(localResponse.Data)
			require.Contains(t, body, "data: ")
			require.Contains(t, body, "data: [DONE]")
			require.Contains(t, body, `"object":"chat.completion.chunk"`)
			require.Contains(t, body, `"model":"qwen-plus"`)
			require.Contains(t, body, `"delta":{"role":"assistant","content":"今天北京天气晴朗，温度25度"}`)
			require.Contains(t, body, `"finish_reason":"stop"`)
			require.Contains(t, body, `"usage":{"prompt_tokens":42,"completion_tokens":17,"total_tokens":59,"prompt_tokens_details":{"cached_tokens":42}}`)
			require.NotContains(t, body, "from-cache")
			require.NotContains(t, body, "upstream_invoked")
		})
	})
}
