package tokenusage

import (
	"testing"

	"github.com/higress-group/wasm-go/pkg/iface"
	"github.com/stretchr/testify/require"
)

func TestProviderUsageMergesAcrossStreamingCallbacks(t *testing.T) {
	ctx := newTokenUsageTestContext()

	usage := GetTokenUsage(ctx, []byte(`data: {"model":"gpt-4","usage":{"prompt_tokens":5,"prompt_tokens_details":{"cached_tokens":2}}}`+"\n\n"))
	require.Equal(t, map[string]any{
		"prompt_tokens": float64(5),
		"prompt_tokens_details": map[string]any{
			"cached_tokens": float64(2),
		},
	}, usage.ProviderUsage)

	usage = GetTokenUsage(ctx, []byte(`data: {"model":"gpt-4","usage":{"completion_tokens":8,"prompt_tokens_details":{"audio_tokens":1},"completion_tokens_details":{"reasoning_tokens":3}}}`+"\n\n"))

	require.Equal(t, map[string]any{
		"prompt_tokens": float64(5),
		"prompt_tokens_details": map[string]any{
			"cached_tokens": float64(2),
			"audio_tokens":  float64(1),
		},
		"completion_tokens": float64(8),
		"completion_tokens_details": map[string]any{
			"reasoning_tokens": float64(3),
		},
	}, usage.ProviderUsage)
}

func TestCacheAwareProviderUsageShapes(t *testing.T) {
	tests := []struct {
		name             string
		body             []byte
		wantInputToken   int64
		wantOutputToken  int64
		wantTotalToken   int64
		wantInputDetails map[string]int64
		wantUsage        map[string]any
	}{
		{
			name:            "openai compatible cached prompt tokens",
			body:            []byte(`{"model":"gpt-4o","usage":{"prompt_tokens":10,"prompt_tokens_details":{"cached_tokens":3},"completion_tokens":4,"total_tokens":14}}`),
			wantInputToken:  10,
			wantOutputToken: 4,
			wantTotalToken:  14,
			wantInputDetails: map[string]int64{
				"cached_tokens": 3,
			},
			wantUsage: map[string]any{
				"prompt_tokens": float64(10),
				"prompt_tokens_details": map[string]any{
					"cached_tokens": float64(3),
				},
				"completion_tokens": float64(4),
				"total_tokens":      float64(14),
			},
		},
		{
			name:            "kimi cached prompt tokens",
			body:            []byte(`{"model":"moonshot-v1","usage":{"prompt_tokens":12,"cached_tokens":5,"completion_tokens":6,"total_tokens":18}}`),
			wantInputToken:  12,
			wantOutputToken: 6,
			wantTotalToken:  18,
			wantInputDetails: map[string]int64{
				"cached_tokens": 5,
			},
			wantUsage: map[string]any{
				"prompt_tokens":     float64(12),
				"cached_tokens":     float64(5),
				"completion_tokens": float64(6),
				"total_tokens":      float64(18),
			},
		},
		{
			name:            "deepseek explicit cache split",
			body:            []byte(`{"model":"deepseek-chat","usage":{"prompt_tokens":11,"prompt_cache_hit_tokens":4,"prompt_cache_miss_tokens":7,"completion_tokens":2,"total_tokens":13}}`),
			wantInputToken:  11,
			wantOutputToken: 2,
			wantTotalToken:  13,
			wantInputDetails: map[string]int64{
				InputTokenDetailsKeyDeepSeekPromptCacheHitTokens:  4,
				InputTokenDetailsKeyDeepSeekPromptCacheMissTokens: 7,
			},
			wantUsage: map[string]any{
				"prompt_tokens":            float64(11),
				"prompt_cache_hit_tokens":  float64(4),
				"prompt_cache_miss_tokens": float64(7),
				"completion_tokens":        float64(2),
				"total_tokens":             float64(13),
			},
		},
		{
			name:            "qwen native cached details preserve cache creation",
			body:            []byte(`{"model":"qwen-plus","usage":{"input_tokens":16,"output_tokens":5,"total_tokens":21,"prompt_tokens_details":{"cached_tokens":6,"cache_creation":{"ephemeral_5m_input_tokens":2}}}}`),
			wantInputToken:  16,
			wantOutputToken: 5,
			wantTotalToken:  21,
			wantInputDetails: map[string]int64{
				"cached_tokens": 6,
			},
			wantUsage: map[string]any{
				"input_tokens":  float64(16),
				"output_tokens": float64(5),
				"total_tokens":  float64(21),
				"prompt_tokens_details": map[string]any{
					"cached_tokens": float64(6),
					"cache_creation": map[string]any{
						"ephemeral_5m_input_tokens": float64(2),
					},
				},
			},
		},
		{
			name:            "anthropic cache read and creation tokens",
			body:            []byte(`{"model":"claude-3-5-sonnet","usage":{"input_tokens":10,"output_tokens":7,"cache_creation_input_tokens":4,"cache_read_input_tokens":3}}`),
			wantInputToken:  10,
			wantOutputToken: 7,
			wantTotalToken:  24,
			wantInputDetails: map[string]int64{
				InputTokenDetailsKeyAnthropicMessagesUsageCacheCreationInputTokens: 4,
				InputTokenDetailsKeyAnthropicMessagesUsageCacheReadInputTokens:     3,
			},
			wantUsage: map[string]any{
				"input_tokens":                float64(10),
				"output_tokens":               float64(7),
				"cache_creation_input_tokens": float64(4),
				"cache_read_input_tokens":     float64(3),
			},
		},
		{
			name:            "gemini cached content tokens",
			body:            []byte(`{"modelVersion":"gemini-2.5-pro","usageMetadata":{"promptTokenCount":20,"cachedContentTokenCount":9,"candidatesTokenCount":3,"totalTokenCount":23}}`),
			wantInputToken:  20,
			wantOutputToken: 3,
			wantTotalToken:  23,
			wantInputDetails: map[string]int64{
				InputTokenDetailsKeyGeminiCachedContentTokenCount: 9,
			},
			wantUsage: map[string]any{
				"promptTokenCount":        float64(20),
				"cachedContentTokenCount": float64(9),
				"candidatesTokenCount":    float64(3),
				"totalTokenCount":         float64(23),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			usage := GetTokenUsage(newTokenUsageTestContext(), tc.body)

			require.Equal(t, tc.wantInputToken, usage.InputToken)
			require.Equal(t, tc.wantOutputToken, usage.OutputToken)
			require.Equal(t, tc.wantTotalToken, usage.TotalToken)
			require.Equal(t, tc.wantInputDetails, usage.InputTokenDetails)
			require.Equal(t, tc.wantUsage, usage.ProviderUsage)
		})
	}
}

type tokenUsageTestContext struct {
	values     map[string]any
	attributes map[string]any
}

func newTokenUsageTestContext() *tokenUsageTestContext {
	return &tokenUsageTestContext{
		values:     map[string]any{},
		attributes: map[string]any{},
	}
}

func (c *tokenUsageTestContext) Scheme() string { return "" }
func (c *tokenUsageTestContext) Host() string   { return "" }
func (c *tokenUsageTestContext) Path() string   { return "" }
func (c *tokenUsageTestContext) Method() string { return "" }

func (c *tokenUsageTestContext) SetContext(key string, value any) {
	c.values[key] = value
}

func (c *tokenUsageTestContext) GetContext(key string) any {
	return c.values[key]
}

func (c *tokenUsageTestContext) GetBoolContext(key string, defaultValue bool) bool {
	if value, ok := c.values[key].(bool); ok {
		return value
	}
	return defaultValue
}

func (c *tokenUsageTestContext) GetStringContext(key, defaultValue string) string {
	if value, ok := c.values[key].(string); ok {
		return value
	}
	return defaultValue
}

func (c *tokenUsageTestContext) GetByteSliceContext(key string, defaultValue []byte) []byte {
	if value, ok := c.values[key].([]byte); ok {
		return value
	}
	return defaultValue
}

func (c *tokenUsageTestContext) GetUserAttribute(key string) any {
	return c.attributes[key]
}

func (c *tokenUsageTestContext) SetUserAttribute(key string, value any) {
	c.attributes[key] = value
}

func (c *tokenUsageTestContext) SetUserAttributeMap(kvmap map[string]any) {
	c.attributes = kvmap
}

func (c *tokenUsageTestContext) GetUserAttributeMap() map[string]any {
	return c.attributes
}

func (c *tokenUsageTestContext) WriteUserAttributeToLog() error                  { return nil }
func (c *tokenUsageTestContext) WriteUserAttributeToLogWithKey(key string) error { return nil }
func (c *tokenUsageTestContext) WriteUserAttributeToTrace() error                { return nil }
func (c *tokenUsageTestContext) DontReadRequestBody()                            {}
func (c *tokenUsageTestContext) DontReadResponseBody()                           {}
func (c *tokenUsageTestContext) BufferRequestBody()                              {}
func (c *tokenUsageTestContext) BufferResponseBody()                             {}
func (c *tokenUsageTestContext) NeedPauseStreamingResponse()                     {}
func (c *tokenUsageTestContext) PushBuffer(buffer []byte)                        {}
func (c *tokenUsageTestContext) PopBuffer() []byte                               { return nil }
func (c *tokenUsageTestContext) BufferQueueSize() int                            { return 0 }
func (c *tokenUsageTestContext) DisableReroute()                                 {}
func (c *tokenUsageTestContext) SetRequestBodyBufferLimit(byteSize uint32)       {}
func (c *tokenUsageTestContext) SetResponseBodyBufferLimit(byteSize uint32)      {}
func (c *tokenUsageTestContext) RouteCall(method, url string, headers [][2]string, body []byte, callback iface.RouteResponseCallback) error {
	return nil
}
func (c *tokenUsageTestContext) GetExecutionPhase() iface.HTTPExecutionPhase {
	return iface.EncodeData
}
func (c *tokenUsageTestContext) HasRequestBody() bool       { return false }
func (c *tokenUsageTestContext) HasResponseBody() bool      { return false }
func (c *tokenUsageTestContext) IsWebsocket() bool          { return false }
func (c *tokenUsageTestContext) IsBinaryRequestBody() bool  { return false }
func (c *tokenUsageTestContext) IsBinaryResponseBody() bool { return false }
