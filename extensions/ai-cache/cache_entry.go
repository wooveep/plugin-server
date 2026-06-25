package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

const StructuredCacheEntryVersion = 1

type RedisCacheEntryKind string

const (
	RedisCacheEntryKindStructured RedisCacheEntryKind = "structured"
	RedisCacheEntryKindLegacy     RedisCacheEntryKind = "legacy"
	RedisCacheEntryKindEmpty      RedisCacheEntryKind = "empty"
)

type RedisCacheEntryPayload struct {
	Kind            RedisCacheEntryKind
	StructuredEntry *StructuredCacheEntry
	LegacyValue     string
}

type redisCacheEntryProbe struct {
	Version *int `json:"version"`
	Answer  *struct {
		Content string `json:"content"`
	} `json:"answer"`
	Scope *struct {
		Type string `json:"type"`
	} `json:"scope"`
	RequestFingerprint *struct {
		Hash string `json:"hash"`
	} `json:"request_fingerprint"`
}

func (p redisCacheEntryProbe) hasStructuredCacheEntryShape() bool {
	return p.Version != nil &&
		p.Answer != nil &&
		strings.TrimSpace(p.Answer.Content) != "" &&
		p.Scope != nil &&
		strings.TrimSpace(p.Scope.Type) != "" &&
		p.RequestFingerprint != nil &&
		strings.TrimSpace(p.RequestFingerprint.Hash) != ""
}

func (p RedisCacheEntryPayload) IsAuthoritative() bool {
	return p.Kind == RedisCacheEntryKindStructured && p.StructuredEntry != nil
}

func DecodeRedisCacheEntryPayload(payload string) (RedisCacheEntryPayload, error) {
	trimmed := strings.TrimSpace(payload)
	if trimmed == "" {
		return RedisCacheEntryPayload{Kind: RedisCacheEntryKindEmpty}, nil
	}

	var probe redisCacheEntryProbe
	if err := json.Unmarshal([]byte(trimmed), &probe); err != nil {
		return RedisCacheEntryPayload{
			Kind:        RedisCacheEntryKindLegacy,
			LegacyValue: payload,
		}, nil
	}
	if !probe.hasStructuredCacheEntryShape() {
		return RedisCacheEntryPayload{
			Kind:        RedisCacheEntryKindLegacy,
			LegacyValue: payload,
		}, nil
	}
	if *probe.Version != StructuredCacheEntryVersion {
		return RedisCacheEntryPayload{}, fmt.Errorf("unsupported structured cache entry version: %d", *probe.Version)
	}

	var entry StructuredCacheEntry
	if err := json.Unmarshal([]byte(trimmed), &entry); err != nil {
		return RedisCacheEntryPayload{}, err
	}
	return RedisCacheEntryPayload{
		Kind:            RedisCacheEntryKindStructured,
		StructuredEntry: &entry,
	}, nil
}

func EncodeRedisCacheEntryPayload(entry StructuredCacheEntry) (string, error) {
	if entry.Version != StructuredCacheEntryVersion {
		return "", fmt.Errorf("unsupported structured cache entry version: %d", entry.Version)
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func CacheHitTextFromRedisPayload(payload string) (string, bool, error) {
	decoded, err := DecodeRedisCacheEntryPayload(payload)
	if err != nil {
		return "", false, err
	}
	if decoded.IsAuthoritative() {
		return decoded.StructuredEntry.Answer.Content, true, nil
	}
	return decoded.LegacyValue, false, nil
}

func NewStructuredCacheEntryForAnswer(key string, answer string) StructuredCacheEntry {
	return StructuredCacheEntry{
		Version: StructuredCacheEntryVersion,
		Answer: StructuredCacheAnswer{
			Role:    "assistant",
			Content: answer,
		},
		Scope: StructuredCacheScope{
			Type: "legacy",
		},
		RequestFingerprint: RequestFingerprint{
			Hash: key,
		},
	}
}

type StructuredCacheEntry struct {
	Version            int                     `json:"version"`
	Answer             StructuredCacheAnswer   `json:"answer"`
	Model              string                  `json:"model"`
	Usage              StructuredCacheUsage    `json:"usage"`
	FinishReason       string                  `json:"finish_reason"`
	Route              StructuredRouteFacts    `json:"route"`
	Provider           StructuredProviderFacts `json:"provider"`
	Upstream           StructuredUpstreamFacts `json:"upstream"`
	Scope              StructuredCacheScope    `json:"scope"`
	PolicyVersion      string                  `json:"policy_version"`
	RequestFingerprint RequestFingerprint      `json:"request_fingerprint"`
	Semantic           SemanticCacheMetadata   `json:"semantic"`
	Tags               []string                `json:"tags,omitempty"`
	CreatedAt          int64                   `json:"created_at"`
	SoftExpiresAt      int64                   `json:"soft_expires_at"`
	HardExpiresAt      int64                   `json:"hard_expires_at"`
}

type StructuredCacheAnswer struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type StructuredCacheUsage struct {
	PromptTokens        int                          `json:"prompt_tokens"`
	CompletionTokens    int                          `json:"completion_tokens"`
	TotalTokens         int                          `json:"total_tokens"`
	PromptTokensDetails *StructuredPromptTokenDetail `json:"prompt_tokens_details,omitempty"`
}

type StructuredPromptTokenDetail struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
}

type StructuredRouteFacts struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	APIName string `json:"api_name,omitempty"`
}

type StructuredProviderFacts struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Model string `json:"model,omitempty"`
}

type StructuredUpstreamFacts struct {
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
	Cluster  string `json:"cluster,omitempty"`
}

type StructuredCacheScope struct {
	Type     string `json:"type"`
	Tenant   string `json:"tenant,omitempty"`
	Consumer string `json:"consumer,omitempty"`
}

type RequestFingerprint struct {
	Hash             string `json:"hash"`
	MessagesHash     string `json:"messages_hash,omitempty"`
	SystemHash       string `json:"system_hash,omitempty"`
	ToolsHash        string `json:"tools_hash,omitempty"`
	SamplingHash     string `json:"sampling_hash,omitempty"`
	OutputFormatHash string `json:"output_format_hash,omitempty"`
}

type SemanticCacheMetadata struct {
	TextHash          string `json:"text_hash,omitempty"`
	EmbeddingModel    string `json:"embedding_model,omitempty"`
	EmbeddingProvider string `json:"embedding_provider,omitempty"`
}
