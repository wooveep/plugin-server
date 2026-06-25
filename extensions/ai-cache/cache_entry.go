package main

const StructuredCacheEntryVersion = 1

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
	PromptTokens        int                         `json:"prompt_tokens"`
	CompletionTokens    int                         `json:"completion_tokens"`
	TotalTokens         int                         `json:"total_tokens"`
	PromptTokensDetails StructuredPromptTokenDetail `json:"prompt_tokens_details,omitempty"`
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
