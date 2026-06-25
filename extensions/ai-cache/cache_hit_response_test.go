package main

import (
	"encoding/json"
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

		t.Run("soft-expired structured hit misses unless stale replay is enabled", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			requestStructuredCacheHit(t, host, false)
			payload := structuredHitPayload(t, 1, 4102444800)
			host.CallOnRedisCall(0, test.CreateRedisRespArray([]interface{}{payload}))

			require.Nil(t, host.GetLocalResponse())
			redisCalls := host.GetRedisCalloutAttributes()
			require.GreaterOrEqual(t, len(redisCalls), 1)
			require.Contains(t, string(redisCalls[len(redisCalls)-1].Query), "higress-ai-cache:今天天气怎么样？")
		})

		t.Run("soft-expired structured hit replays when stale replay is enabled", func(t *testing.T) {
			host, status := test.NewTestHost(staleReplayConfig())
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			requestStructuredCacheHit(t, host, false)
			payload := structuredHitPayload(t, 1, 4102444800)
			host.CallOnRedisCall(0, test.CreateRedisRespArray([]interface{}{payload}))

			localResponse := host.GetLocalResponse()
			require.NotNil(t, localResponse)
			require.Equal(t, uint32(200), localResponse.StatusCode)
			require.Contains(t, string(localResponse.Data), `"model":"qwen-plus"`)
			require.NotContains(t, string(localResponse.Data), "from-cache")
		})

		t.Run("hard-expired structured hit misses even when stale replay is enabled", func(t *testing.T) {
			host, status := test.NewTestHost(staleReplayConfig())
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			requestStructuredCacheHit(t, host, false)
			payload := structuredHitPayload(t, 1, 1)
			host.CallOnRedisCall(0, test.CreateRedisRespArray([]interface{}{payload}))

			require.Nil(t, host.GetLocalResponse())
			redisCalls := host.GetRedisCalloutAttributes()
			require.GreaterOrEqual(t, len(redisCalls), 1)
			require.Contains(t, string(redisCalls[len(redisCalls)-1].Query), "higress-ai-cache:今天天气怎么样？")
		})
	})
}

func requestStructuredCacheHit(t *testing.T, host test.TestHost, stream bool) {
	t.Helper()
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
	if stream {
		requestBody = `{
			"model": "qwen-plus",
			"messages": [
				{
					"role": "user",
					"content": "今天天气怎么样？"
				}
			],
			"stream": true
		}`
	}
	require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody([]byte(requestBody)))
}

func structuredHitPayload(t *testing.T, softExpiresAt int64, hardExpiresAt int64) string {
	t.Helper()
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
		CreatedAt:     1,
		SoftExpiresAt: softExpiresAt,
		HardExpiresAt: hardExpiresAt,
	})
	require.NoError(t, err)
	return payload
}

func staleReplayConfig() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"cache": map[string]interface{}{
			"type":                 "redis",
			"serviceName":          "redis.static",
			"servicePort":          6379,
			"staleWhileRevalidate": true,
			"cacheTTL":             3600,
			"cacheSoftTTL":         1800,
			"cacheKeyPrefix":       "higress-ai-cache:",
		},
		"provider":               "dashscope",
		"cacheScope":             "global",
		"cacheKeyStrategy":       "lastQuestion",
		"cacheKeyFrom":           "messages.@reverse.0.content",
		"cacheValueFrom":         "choices.0.message.content",
		"cacheStreamValueFrom":   "choices.0.delta.content",
		"responseTemplate":       `{"id":"from-cache","choices":[{"index":0,"message":{"role":"assistant","content":"%s"},"finish_reason":"stop"}],"model":"from-cache","object":"chat.completion","usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`,
		"streamResponseTemplate": `data:{"id":"from-cache","choices":[{"index":0,"delta":{"role":"assistant","content":"%s"},"finish_reason":"stop"}],"model":"from-cache","object":"chat.completion","usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}` + "\n\ndata:[DONE]\n\n",
	})
	return data
}
