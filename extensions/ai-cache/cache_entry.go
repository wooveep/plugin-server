package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/tidwall/gjson"
)

const StructuredCacheEntryVersion = 1

type RedisCacheEntryKind string
type StructuredCacheFreshness string

const (
	RedisCacheEntryKindStructured RedisCacheEntryKind = "structured"
	RedisCacheEntryKindLegacy     RedisCacheEntryKind = "legacy"
	RedisCacheEntryKindEmpty      RedisCacheEntryKind = "empty"

	StructuredCacheFreshnessFresh   StructuredCacheFreshness = "fresh"
	StructuredCacheFreshnessStale   StructuredCacheFreshness = "stale"
	StructuredCacheFreshnessExpired StructuredCacheFreshness = "expired"
)

type RedisCacheEntryPayload struct {
	Kind            RedisCacheEntryKind
	StructuredEntry *StructuredCacheEntry
	LegacyValue     string
}

type OpenAINonStreamingCacheHitResponse struct {
	ID      string                  `json:"id"`
	Object  string                  `json:"object"`
	Model   string                  `json:"model"`
	Choices []StructuredCacheChoice `json:"choices"`
	Usage   StructuredCacheUsage    `json:"usage"`
}

type StructuredCacheChoice struct {
	Index        int                   `json:"index"`
	Message      StructuredCacheAnswer `json:"message"`
	FinishReason string                `json:"finish_reason"`
}

type OpenAIStreamingCacheHitChunk struct {
	ID      string                       `json:"id"`
	Object  string                       `json:"object"`
	Model   string                       `json:"model"`
	Choices []OpenAIStreamingChunkChoice `json:"choices"`
	Usage   *StructuredCacheUsage        `json:"usage,omitempty"`
}

type OpenAIStreamingChunkChoice struct {
	Index        int                       `json:"index"`
	Delta        OpenAIStreamingChunkDelta `json:"delta"`
	FinishReason *string                   `json:"finish_reason"`
}

type OpenAIStreamingChunkDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type StructuredStreamingCacheFacts struct {
	Model   string
	Choices []StructuredCacheChoice
	Usage   StructuredCacheUsage
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

func BuildOpenAINonStreamingCacheHitResponse(entry StructuredCacheEntry, requestID string) ([]byte, error) {
	model := cacheHitModel(entry)
	choices := cacheHitChoices(entry)
	usage := entry.Usage
	if usage.PromptTokens > 0 && usage.PromptTokensDetails == nil {
		usage.PromptTokensDetails = &StructuredPromptTokenDetail{CachedTokens: usage.PromptTokens}
	}

	return json.Marshal(OpenAINonStreamingCacheHitResponse{
		ID:      cacheHitResponseID(entry, requestID),
		Object:  "chat.completion",
		Model:   model,
		Choices: choices,
		Usage:   usage,
	})
}

func BuildOpenAIStreamingCacheHitResponse(entry StructuredCacheEntry, requestID string) ([]byte, error) {
	model := cacheHitModel(entry)
	responseID := cacheHitResponseID(entry, requestID)
	choices := cacheHitChoices(entry)

	contentChoices := make([]OpenAIStreamingChunkChoice, 0, len(choices))
	finalChoices := make([]OpenAIStreamingChunkChoice, 0, len(choices))
	for _, choice := range choices {
		contentChoices = append(contentChoices, OpenAIStreamingChunkChoice{
			Index: choice.Index,
			Delta: OpenAIStreamingChunkDelta{
				Role:    normalizeOr(choice.Message.Role, "assistant"),
				Content: choice.Message.Content,
			},
			FinishReason: nil,
		})
		finishReason := normalizeOr(choice.FinishReason, entry.FinishReason)
		if finishReason == "" {
			finishReason = "stop"
		}
		finalChoices = append(finalChoices, OpenAIStreamingChunkChoice{
			Index:        choice.Index,
			Delta:        OpenAIStreamingChunkDelta{},
			FinishReason: &finishReason,
		})
	}

	var out strings.Builder
	if err := writeSSEJSON(&out, OpenAIStreamingCacheHitChunk{
		ID:      responseID,
		Object:  "chat.completion.chunk",
		Model:   model,
		Choices: contentChoices,
	}); err != nil {
		return nil, err
	}

	finalChunk := OpenAIStreamingCacheHitChunk{
		ID:      responseID,
		Object:  "chat.completion.chunk",
		Model:   model,
		Choices: finalChoices,
	}
	usage := entry.Usage
	if usage.PromptTokens > 0 && usage.PromptTokensDetails == nil {
		usage.PromptTokensDetails = &StructuredPromptTokenDetail{CachedTokens: usage.PromptTokens}
	}
	if err := writeSSEJSON(&out, finalChunk); err != nil {
		return nil, err
	}
	if hasStructuredUsage(usage) {
		if err := writeSSEJSON(&out, OpenAIStreamingCacheHitChunk{
			ID:      responseID,
			Object:  "chat.completion.chunk",
			Model:   model,
			Choices: []OpenAIStreamingChunkChoice{},
			Usage:   &usage,
		}); err != nil {
			return nil, err
		}
	}
	out.WriteString("data: [DONE]\n\n")
	return []byte(out.String()), nil
}

func writeSSEJSON(out *strings.Builder, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	out.WriteString("data: ")
	out.Write(data)
	out.WriteString("\n\n")
	return nil
}

func cacheHitModel(entry StructuredCacheEntry) string {
	model := strings.TrimSpace(entry.Model)
	if model == "" {
		model = strings.TrimSpace(entry.Provider.Model)
	}
	if model == "" {
		model = strings.TrimSpace(entry.Upstream.Model)
	}
	if model == "" {
		model = "unknown-model"
	}
	return model
}

func cacheHitChoices(entry StructuredCacheEntry) []StructuredCacheChoice {
	role := strings.TrimSpace(entry.Answer.Role)
	if role == "" {
		role = "assistant"
	}

	finishReason := strings.TrimSpace(entry.FinishReason)
	if finishReason == "" {
		finishReason = "stop"
	}

	choices := entry.Choices
	if len(choices) == 0 {
		choices = []StructuredCacheChoice{
			{
				Index: 0,
				Message: StructuredCacheAnswer{
					Role:    role,
					Content: entry.Answer.Content,
				},
				FinishReason: finishReason,
			},
		}
	}
	return choices
}

func cacheHitResponseID(entry StructuredCacheEntry, requestID string) string {
	responseID := "chatcmpl-cache"
	if requestID = strings.TrimSpace(requestID); requestID != "" {
		responseID += "-" + requestID
	} else if entry.RequestFingerprint.Hash != "" {
		responseID += "-" + entry.RequestFingerprint.Hash
	}
	return responseID
}

func hasStructuredUsage(usage StructuredCacheUsage) bool {
	return usage.PromptTokens != 0 ||
		usage.CompletionTokens != 0 ||
		usage.TotalTokens != 0 ||
		usage.PromptTokensDetails != nil
}

func (facts StructuredStreamingCacheFacts) HasReplayFacts() bool {
	return strings.TrimSpace(facts.Model) != "" ||
		len(facts.Choices) > 0 ||
		hasStructuredUsage(facts.Usage)
}

func (facts *StructuredStreamingCacheFacts) MergeOpenAIStreamingChunkJSON(bodyJSON string) {
	bodyJSON = strings.TrimSpace(bodyJSON)
	if bodyJSON == "" || !strings.HasPrefix(bodyJSON, "{") {
		return
	}

	body := gjson.Parse(bodyJSON)
	if model := strings.TrimSpace(body.Get("model").String()); model != "" {
		facts.Model = model
	}
	if usageRaw := strings.TrimSpace(body.Get("usage").Raw); usageRaw != "" && usageRaw != "null" {
		var usage StructuredCacheUsage
		if err := json.Unmarshal([]byte(usageRaw), &usage); err == nil {
			facts.Usage = usage
		}
	}

	for _, choiceJSON := range body.Get("choices").Array() {
		choice := facts.ensureChoice(int(choiceJSON.Get("index").Int()))
		if role := strings.TrimSpace(choiceJSON.Get("delta.role").String()); role != "" {
			choice.Message.Role = role
		}
		if content := choiceJSON.Get("delta.content"); content.Exists() {
			choice.Message.Content += content.String()
		}
		if finishReason := strings.TrimSpace(choiceJSON.Get("finish_reason").String()); finishReason != "" {
			choice.FinishReason = finishReason
		}
	}
}

func (facts *StructuredStreamingCacheFacts) ensureChoice(index int) *StructuredCacheChoice {
	for i := range facts.Choices {
		if facts.Choices[i].Index == index {
			return &facts.Choices[i]
		}
	}

	facts.Choices = append(facts.Choices, StructuredCacheChoice{
		Index: index,
		Message: StructuredCacheAnswer{
			Role: "assistant",
		},
	})
	return &facts.Choices[len(facts.Choices)-1]
}

func NewStructuredCacheEntryForOpenAIStreamingResponse(facts StructuredStreamingCacheFacts, answer string, keyMaterial ScopedCacheKeyMaterial) StructuredCacheEntry {
	entry := NewStructuredCacheEntryForAnswer(answer, keyMaterial)
	if model := strings.TrimSpace(facts.Model); model != "" {
		entry.Model = model
		entry.Provider.Model = model
	}
	if len(facts.Choices) > 0 {
		entry.Choices = make([]StructuredCacheChoice, 0, len(facts.Choices))
		for _, choice := range facts.Choices {
			if strings.TrimSpace(choice.Message.Role) == "" {
				choice.Message.Role = "assistant"
			}
			entry.Choices = append(entry.Choices, choice)
		}
		sort.Slice(entry.Choices, func(i, j int) bool {
			return entry.Choices[i].Index < entry.Choices[j].Index
		})
		if strings.TrimSpace(entry.Choices[0].Message.Content) == "" {
			entry.Choices[0].Message.Content = answer
		}
		entry.Answer = entry.Choices[0].Message
		if strings.TrimSpace(entry.Choices[0].FinishReason) != "" {
			entry.FinishReason = entry.Choices[0].FinishReason
		}
	}
	if hasStructuredUsage(facts.Usage) {
		entry.Usage = facts.Usage
	}
	return entry
}

func NewStructuredCacheEntryForOpenAIResponse(responseBody []byte, answer string, keyMaterial ScopedCacheKeyMaterial) StructuredCacheEntry {
	entry := NewStructuredCacheEntryForAnswer(answer, keyMaterial)
	body := gjson.ParseBytes(responseBody)
	if model := strings.TrimSpace(body.Get("model").String()); model != "" {
		entry.Model = model
		entry.Provider.Model = model
	}
	if usageRaw := body.Get("usage").Raw; usageRaw != "" {
		_ = json.Unmarshal([]byte(usageRaw), &entry.Usage)
	}

	choices := body.Get("choices").Array()
	if len(choices) > 0 {
		entry.Choices = make([]StructuredCacheChoice, 0, len(choices))
		for _, choice := range choices {
			cacheChoice := StructuredCacheChoice{
				Index: int(choice.Get("index").Int()),
				Message: StructuredCacheAnswer{
					Role:    normalizeOr(choice.Get("message.role").String(), "assistant"),
					Content: choice.Get("message.content").String(),
				},
				FinishReason: choice.Get("finish_reason").String(),
			}
			entry.Choices = append(entry.Choices, cacheChoice)
		}
		firstChoice := entry.Choices[0]
		if strings.TrimSpace(firstChoice.Message.Content) != "" {
			entry.Answer = firstChoice.Message
		}
		if strings.TrimSpace(firstChoice.FinishReason) != "" {
			entry.FinishReason = firstChoice.FinishReason
		}
	}

	return entry
}

func NewStructuredCacheEntryForAnswer(answer string, keyMaterial ScopedCacheKeyMaterial) StructuredCacheEntry {
	return StructuredCacheEntry{
		Version: StructuredCacheEntryVersion,
		Answer: StructuredCacheAnswer{
			Role:    "assistant",
			Content: answer,
		},
		Model:              keyMaterial.Provider.Model,
		Route:              keyMaterial.Route,
		Provider:           keyMaterial.Provider,
		Scope:              keyMaterial.Scope,
		PolicyVersion:      keyMaterial.PolicyVersion,
		RequestFingerprint: keyMaterial.RequestFingerprint,
	}
}

func ApplyStructuredCacheEntryExpiry(entry *StructuredCacheEntry, nowUnix int64, softTTLSeconds int, hardTTLSeconds int) {
	if entry == nil || nowUnix <= 0 {
		return
	}
	entry.CreatedAt = nowUnix
	if hardTTLSeconds > 0 {
		entry.HardExpiresAt = nowUnix + int64(hardTTLSeconds)
	}
	if softTTLSeconds > 0 {
		softExpiresAt := nowUnix + int64(softTTLSeconds)
		if entry.HardExpiresAt > 0 && softExpiresAt > entry.HardExpiresAt {
			softExpiresAt = entry.HardExpiresAt
		}
		entry.SoftExpiresAt = softExpiresAt
	}
}

func StructuredCacheEntryFreshnessAt(entry StructuredCacheEntry, nowUnix int64) StructuredCacheFreshness {
	if nowUnix <= 0 {
		return StructuredCacheFreshnessFresh
	}
	if entry.HardExpiresAt > 0 && nowUnix >= entry.HardExpiresAt {
		return StructuredCacheFreshnessExpired
	}
	if entry.SoftExpiresAt > 0 && nowUnix >= entry.SoftExpiresAt {
		return StructuredCacheFreshnessStale
	}
	return StructuredCacheFreshnessFresh
}

type StructuredCacheEntry struct {
	Version            int                     `json:"version"`
	Answer             StructuredCacheAnswer   `json:"answer"`
	Model              string                  `json:"model"`
	Choices            []StructuredCacheChoice `json:"choices,omitempty"`
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
	CachedTokens int `json:"cached_tokens"`
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
