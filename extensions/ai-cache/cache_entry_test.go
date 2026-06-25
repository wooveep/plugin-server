package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStructuredCacheEntryJSONContract(t *testing.T) {
	payload := []byte(`{
		"version":1,
		"answer":{"role":"assistant","content":"cached answer"},
		"model":"qwen-plus",
		"usage":{
			"prompt_tokens":12,
			"completion_tokens":34,
			"total_tokens":46,
			"prompt_tokens_details":{"cached_tokens":12}
		},
		"finish_reason":"stop",
		"route":{"id":"route-1","name":"chat","api_name":"chat-api"},
		"provider":{"id":"provider-1","name":"qwen","model":"qwen-plus"},
		"upstream":{"provider":"qwen","model":"qwen-plus","cluster":"outbound|443||llm-qwen.internal.dns"},
		"scope":{"type":"tenant","tenant":"deep","consumer":"alice"},
		"policy_version":"pv-20260625",
		"request_fingerprint":{
			"hash":"request-hash",
			"messages_hash":"messages-hash",
			"system_hash":"system-hash",
			"tools_hash":"tools-hash",
			"sampling_hash":"sampling-hash",
			"output_format_hash":"format-hash"
		},
		"semantic":{
			"text_hash":"semantic-text-hash",
			"embedding_model":"text-embedding-v4",
			"embedding_provider":"qwen"
		},
		"tags":["faq","billing"],
		"created_at":1782375000,
		"soft_expires_at":1782378600,
		"hard_expires_at":1782382200
	}`)

	var entry StructuredCacheEntry
	require.NoError(t, json.Unmarshal(payload, &entry))
	require.Equal(t, StructuredCacheEntryVersion, entry.Version)
	require.Equal(t, "cached answer", entry.Answer.Content)
	require.Equal(t, "qwen-plus", entry.Model)
	require.NotNil(t, entry.Usage.PromptTokensDetails)
	require.Equal(t, 12, entry.Usage.PromptTokensDetails.CachedTokens)
	require.Equal(t, "route-1", entry.Route.ID)
	require.Equal(t, "qwen", entry.Provider.Name)
	require.Equal(t, "outbound|443||llm-qwen.internal.dns", entry.Upstream.Cluster)
	require.Equal(t, "tenant", entry.Scope.Type)
	require.Equal(t, "request-hash", entry.RequestFingerprint.Hash)
	require.Equal(t, "semantic-text-hash", entry.Semantic.TextHash)
	require.Equal(t, []string{"faq", "billing"}, entry.Tags)
	require.Equal(t, int64(1782378600), entry.SoftExpiresAt)
	require.Equal(t, int64(1782382200), entry.HardExpiresAt)

	encoded, err := json.Marshal(entry)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"soft_expires_at":1782378600`)
	require.Contains(t, string(encoded), `"hard_expires_at":1782382200`)
	require.NotContains(t, string(encoded), "from-cache")
}

func TestStructuredCacheEntryExpiryHelpers(t *testing.T) {
	entry := StructuredCacheEntry{}
	ApplyStructuredCacheEntryExpiry(&entry, 1000, 300, 600)
	require.Equal(t, int64(1000), entry.CreatedAt)
	require.Equal(t, int64(1300), entry.SoftExpiresAt)
	require.Equal(t, int64(1600), entry.HardExpiresAt)

	require.Equal(t, StructuredCacheFreshnessFresh, StructuredCacheEntryFreshnessAt(entry, 1299))
	require.Equal(t, StructuredCacheFreshnessStale, StructuredCacheEntryFreshnessAt(entry, 1300))
	require.Equal(t, StructuredCacheFreshnessExpired, StructuredCacheEntryFreshnessAt(entry, 1600))

	entry = StructuredCacheEntry{}
	ApplyStructuredCacheEntryExpiry(&entry, 1000, 700, 600)
	require.Equal(t, int64(1600), entry.SoftExpiresAt)
	require.Equal(t, int64(1600), entry.HardExpiresAt)
}

func TestDecodeRedisCacheEntryPayloadClassifiesStructuredEntry(t *testing.T) {
	payload := `{
		"version":1,
		"answer":{"role":"assistant","content":"cached answer"},
		"model":"qwen-plus",
		"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3},
		"finish_reason":"stop",
		"route":{"name":"chat"},
		"provider":{"name":"qwen","model":"qwen-plus"},
		"scope":{"type":"tenant","tenant":"deep"},
		"policy_version":"pv-1",
		"request_fingerprint":{"hash":"req"},
		"semantic":{"text_hash":"sem"},
		"created_at":1782375000,
		"soft_expires_at":1782378600,
		"hard_expires_at":1782382200
	}`

	result, err := DecodeRedisCacheEntryPayload(payload)
	require.NoError(t, err)
	require.True(t, result.IsAuthoritative())
	require.Equal(t, RedisCacheEntryKindStructured, result.Kind)
	require.NotNil(t, result.StructuredEntry)
	require.Equal(t, "cached answer", result.StructuredEntry.Answer.Content)
	require.Empty(t, result.LegacyValue)
}

func TestDecodeRedisCacheEntryPayloadClassifiesLegacyFallback(t *testing.T) {
	result, err := DecodeRedisCacheEntryPayload("legacy answer text")
	require.NoError(t, err)
	require.False(t, result.IsAuthoritative())
	require.Equal(t, RedisCacheEntryKindLegacy, result.Kind)
	require.Nil(t, result.StructuredEntry)
	require.Equal(t, "legacy answer text", result.LegacyValue)
}

func TestDecodeRedisCacheEntryPayloadKeepsLegacyJSONWithVersionFallback(t *testing.T) {
	payloads := []string{
		`{"version":1,"message":"legacy answer"}`,
		`{"version":2,"message":"legacy answer"}`,
		`{"version":1,"answer":{"content":"legacy answer"}}`,
	}

	for _, payload := range payloads {
		result, err := DecodeRedisCacheEntryPayload(payload)
		require.NoError(t, err)
		require.False(t, result.IsAuthoritative())
		require.Equal(t, RedisCacheEntryKindLegacy, result.Kind)
		require.Equal(t, payload, result.LegacyValue)

		text, authoritative, err := CacheHitTextFromRedisPayload(payload)
		require.NoError(t, err)
		require.False(t, authoritative)
		require.Equal(t, payload, text)
	}
}

func TestCacheHitTextFromRedisPayloadUsesStructuredAnswer(t *testing.T) {
	payload := `{
		"version":1,
		"answer":{"role":"assistant","content":"structured answer"},
		"model":"qwen-plus",
		"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3},
		"finish_reason":"stop",
		"scope":{"type":"tenant","tenant":"deep"},
		"request_fingerprint":{"hash":"req"},
		"soft_expires_at":1782378600,
		"hard_expires_at":1782382200
	}`

	text, authoritative, err := CacheHitTextFromRedisPayload(payload)
	require.NoError(t, err)
	require.True(t, authoritative)
	require.Equal(t, "structured answer", text)
}

func TestCacheHitTextFromRedisPayloadKeepsLegacyFallback(t *testing.T) {
	text, authoritative, err := CacheHitTextFromRedisPayload("legacy answer")
	require.NoError(t, err)
	require.False(t, authoritative)
	require.Equal(t, "legacy answer", text)
}

func TestBuildOpenAINonStreamingCacheHitResponsePreservesStructuredFacts(t *testing.T) {
	payload, err := BuildOpenAINonStreamingCacheHitResponse(StructuredCacheEntry{
		Version: StructuredCacheEntryVersion,
		Answer: StructuredCacheAnswer{
			Role:    "assistant",
			Content: `hello "cache"`,
		},
		Model: "qwen-plus",
		Usage: StructuredCacheUsage{
			PromptTokens:     9,
			CompletionTokens: 4,
			TotalTokens:      13,
		},
		FinishReason: "stop",
		RequestFingerprint: RequestFingerprint{
			Hash: "request-hash",
		},
	}, "request-id")
	require.NoError(t, err)
	require.JSONEq(t, `{
		"id":"chatcmpl-cache-request-id",
		"object":"chat.completion",
		"model":"qwen-plus",
		"choices":[
			{
				"index":0,
				"message":{"role":"assistant","content":"hello \"cache\""},
				"finish_reason":"stop"
			}
		],
		"usage":{
			"prompt_tokens":9,
			"completion_tokens":4,
			"total_tokens":13,
			"prompt_tokens_details":{"cached_tokens":9}
		}
	}`, string(payload))
	require.NotContains(t, string(payload), "from-cache")
	require.NotContains(t, string(payload), "upstream_invoked")
}

func TestBuildOpenAINonStreamingCacheHitResponseKeepsExplicitCachedTokensZero(t *testing.T) {
	payload, err := BuildOpenAINonStreamingCacheHitResponse(StructuredCacheEntry{
		Version: StructuredCacheEntryVersion,
		Answer: StructuredCacheAnswer{
			Role:    "assistant",
			Content: "cached answer",
		},
		Model: "qwen-plus",
		Usage: StructuredCacheUsage{
			PromptTokens:     9,
			CompletionTokens: 4,
			TotalTokens:      13,
			PromptTokensDetails: &StructuredPromptTokenDetail{
				CachedTokens: 0,
			},
		},
		FinishReason: "stop",
	}, "request-id")
	require.NoError(t, err)
	require.JSONEq(t, `{
		"id":"chatcmpl-cache-request-id",
		"object":"chat.completion",
		"model":"qwen-plus",
		"choices":[
			{
				"index":0,
				"message":{"role":"assistant","content":"cached answer"},
				"finish_reason":"stop"
			}
		],
		"usage":{
			"prompt_tokens":9,
			"completion_tokens":4,
			"total_tokens":13,
			"prompt_tokens_details":{"cached_tokens":0}
		}
	}`, string(payload))
}

func TestBuildOpenAINonStreamingCacheHitResponsePreservesStructuredChoices(t *testing.T) {
	payload, err := BuildOpenAINonStreamingCacheHitResponse(StructuredCacheEntry{
		Version: StructuredCacheEntryVersion,
		Answer: StructuredCacheAnswer{
			Role:    "assistant",
			Content: "first answer",
		},
		Model: "qwen-plus",
		Choices: []StructuredCacheChoice{
			{
				Index:        0,
				Message:      StructuredCacheAnswer{Role: "assistant", Content: "first answer"},
				FinishReason: "stop",
			},
			{
				Index:        1,
				Message:      StructuredCacheAnswer{Role: "assistant", Content: "second answer"},
				FinishReason: "length",
			},
		},
		Usage:        StructuredCacheUsage{PromptTokens: 9, CompletionTokens: 4, TotalTokens: 13},
		FinishReason: "stop",
	}, "request-id")
	require.NoError(t, err)
	require.JSONEq(t, `{
		"id":"chatcmpl-cache-request-id",
		"object":"chat.completion",
		"model":"qwen-plus",
		"choices":[
			{
				"index":0,
				"message":{"role":"assistant","content":"first answer"},
				"finish_reason":"stop"
			},
			{
				"index":1,
				"message":{"role":"assistant","content":"second answer"},
				"finish_reason":"length"
			}
		],
		"usage":{
			"prompt_tokens":9,
			"completion_tokens":4,
			"total_tokens":13,
			"prompt_tokens_details":{"cached_tokens":9}
		}
	}`, string(payload))
}

func TestBuildOpenAIStreamingCacheHitResponsePreservesStructuredFacts(t *testing.T) {
	payload, err := BuildOpenAIStreamingCacheHitResponse(StructuredCacheEntry{
		Version: StructuredCacheEntryVersion,
		Answer: StructuredCacheAnswer{
			Role:    "assistant",
			Content: `hello "cache"`,
		},
		Model: "qwen-plus",
		Choices: []StructuredCacheChoice{
			{
				Index:        0,
				Message:      StructuredCacheAnswer{Role: "assistant", Content: `hello "cache"`},
				FinishReason: "stop",
			},
		},
		Usage: StructuredCacheUsage{
			PromptTokens:     9,
			CompletionTokens: 4,
			TotalTokens:      13,
			PromptTokensDetails: &StructuredPromptTokenDetail{
				CachedTokens: 9,
			},
		},
		FinishReason: "stop",
	}, "request-id")
	require.NoError(t, err)
	require.Equal(t, "data: {\"id\":\"chatcmpl-cache-request-id\",\"object\":\"chat.completion.chunk\",\"model\":\"qwen-plus\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"hello \\\"cache\\\"\"},\"finish_reason\":null}]}\n\n"+
		"data: {\"id\":\"chatcmpl-cache-request-id\",\"object\":\"chat.completion.chunk\",\"model\":\"qwen-plus\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"+
		"data: {\"id\":\"chatcmpl-cache-request-id\",\"object\":\"chat.completion.chunk\",\"model\":\"qwen-plus\",\"choices\":[],\"usage\":{\"prompt_tokens\":9,\"completion_tokens\":4,\"total_tokens\":13,\"prompt_tokens_details\":{\"cached_tokens\":9}}}\n\n"+
		"data: [DONE]\n\n", string(payload))
	require.NotContains(t, string(payload), "from-cache")
	require.NotContains(t, string(payload), "upstream_invoked")
}

func TestNewStructuredCacheEntryForOpenAIResponsePreservesReplayFacts(t *testing.T) {
	entry := NewStructuredCacheEntryForOpenAIResponse([]byte(`{
		"id":"chatcmpl-123",
		"object":"chat.completion",
		"model":"qwen-plus",
		"choices":[
			{
				"index":0,
				"message":{"role":"assistant","content":"first answer"},
				"finish_reason":"stop"
			},
			{
				"index":1,
				"message":{"role":"assistant","content":"second answer"},
				"finish_reason":"length"
			}
		],
		"usage":{
			"prompt_tokens":9,
			"completion_tokens":4,
			"total_tokens":13,
			"prompt_tokens_details":{"cached_tokens":0}
		}
	}`), "first answer", ScopedCacheKeyMaterial{
		Route:              StructuredRouteFacts{Name: "chat"},
		Provider:           StructuredProviderFacts{Name: "qwen", Model: "qwen-plus"},
		Scope:              StructuredCacheScope{Type: "global"},
		PolicyVersion:      "v1",
		RequestFingerprint: RequestFingerprint{Hash: "request-hash"},
	})

	require.Equal(t, "qwen-plus", entry.Model)
	require.Equal(t, "qwen-plus", entry.Provider.Model)
	require.Equal(t, "stop", entry.FinishReason)
	require.Equal(t, "first answer", entry.Answer.Content)
	require.Len(t, entry.Choices, 2)
	require.Equal(t, "second answer", entry.Choices[1].Message.Content)
	require.Equal(t, "length", entry.Choices[1].FinishReason)
	require.Equal(t, 9, entry.Usage.PromptTokens)
	require.NotNil(t, entry.Usage.PromptTokensDetails)
	require.Equal(t, 0, entry.Usage.PromptTokensDetails.CachedTokens)
}

func TestNewStructuredCacheEntryForAnswerSetsSemanticTextHash(t *testing.T) {
	entry := NewStructuredCacheEntryForAnswer("cached answer", ScopedCacheKeyMaterial{
		LegacyKey:          "介绍杭州西湖",
		Route:              StructuredRouteFacts{Name: "chat"},
		Provider:           StructuredProviderFacts{Name: "qwen", Model: "qwen-plus"},
		Scope:              StructuredCacheScope{Type: "global"},
		PolicyVersion:      "v1",
		RequestFingerprint: RequestFingerprint{Hash: "request-hash"},
	})

	require.Equal(t, stableHash("介绍杭州西湖"), entry.Semantic.TextHash)
}

func TestNewStructuredCacheEntryForOpenAIStreamingResponsePreservesReplayFacts(t *testing.T) {
	var facts StructuredStreamingCacheFacts
	facts.MergeOpenAIStreamingChunkJSON(`{
		"id":"chatcmpl-123",
		"object":"chat.completion.chunk",
		"model":"qwen-plus",
		"choices":[
			{"index":0,"delta":{"role":"assistant","content":"first "},"finish_reason":null},
			{"index":1,"delta":{"role":"assistant","content":"second "},"finish_reason":null}
		]
	}`)
	facts.MergeOpenAIStreamingChunkJSON(`{
		"id":"chatcmpl-123",
		"object":"chat.completion.chunk",
		"model":"qwen-plus",
		"choices":[
			{"index":0,"delta":{"content":"answer"},"finish_reason":"stop"},
			{"index":1,"delta":{"content":"answer"},"finish_reason":"length"}
		]
	}`)
	facts.MergeOpenAIStreamingChunkJSON(`{
		"id":"chatcmpl-123",
		"object":"chat.completion.chunk",
		"model":"qwen-plus",
		"choices":[],
		"usage":{
			"prompt_tokens":9,
			"completion_tokens":4,
			"total_tokens":13,
			"prompt_tokens_details":{"cached_tokens":0}
		}
	}`)

	entry := NewStructuredCacheEntryForOpenAIStreamingResponse(facts, "first answer", ScopedCacheKeyMaterial{
		Route:              StructuredRouteFacts{Name: "chat"},
		Provider:           StructuredProviderFacts{Name: "qwen", Model: "configured-model"},
		Scope:              StructuredCacheScope{Type: "global"},
		PolicyVersion:      "v1",
		RequestFingerprint: RequestFingerprint{Hash: "request-hash"},
	})

	require.Equal(t, "qwen-plus", entry.Model)
	require.Equal(t, "qwen-plus", entry.Provider.Model)
	require.Equal(t, "first answer", entry.Answer.Content)
	require.Equal(t, "stop", entry.FinishReason)
	require.Len(t, entry.Choices, 2)
	require.Equal(t, "second answer", entry.Choices[1].Message.Content)
	require.Equal(t, "length", entry.Choices[1].FinishReason)
	require.Equal(t, 9, entry.Usage.PromptTokens)
	require.NotNil(t, entry.Usage.PromptTokensDetails)
	require.Equal(t, 0, entry.Usage.PromptTokensDetails.CachedTokens)
}

func TestEncodeRedisCacheEntryPayloadRejectsUnsupportedVersion(t *testing.T) {
	entry := StructuredCacheEntry{
		Version:            StructuredCacheEntryVersion + 1,
		Answer:             StructuredCacheAnswer{Role: "assistant", Content: "cached answer"},
		Model:              "qwen-plus",
		Usage:              StructuredCacheUsage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3},
		Scope:              StructuredCacheScope{Type: "tenant", Tenant: "deep"},
		RequestFingerprint: RequestFingerprint{Hash: "req"},
		SoftExpiresAt:      1782378600,
		HardExpiresAt:      1782382200,
	}

	_, err := EncodeRedisCacheEntryPayload(entry)
	require.Error(t, err)
}

func TestEncodeRedisCacheEntryPayloadWritesStructuredJSON(t *testing.T) {
	entry := StructuredCacheEntry{
		Version:            StructuredCacheEntryVersion,
		Answer:             StructuredCacheAnswer{Role: "assistant", Content: "cached answer"},
		Model:              "qwen-plus",
		Usage:              StructuredCacheUsage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3},
		Scope:              StructuredCacheScope{Type: "tenant", Tenant: "deep"},
		RequestFingerprint: RequestFingerprint{Hash: "req"},
		SoftExpiresAt:      1782378600,
		HardExpiresAt:      1782382200,
	}

	payload, err := EncodeRedisCacheEntryPayload(entry)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"version":1,
		"answer":{"role":"assistant","content":"cached answer"},
		"model":"qwen-plus",
		"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3},
		"finish_reason":"",
		"route":{},
		"provider":{},
		"upstream":{},
		"scope":{"type":"tenant","tenant":"deep"},
		"policy_version":"",
		"request_fingerprint":{"hash":"req"},
		"semantic":{},
		"created_at":0,
		"soft_expires_at":1782378600,
		"hard_expires_at":1782382200
	}`, payload)
}
