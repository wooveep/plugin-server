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
