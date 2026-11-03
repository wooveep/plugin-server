package provider

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type managedAdapterContext struct{ *mapCtx }

func (c *managedAdapterContext) GetStringContext(key, fallback string) string {
	if value, ok := c.GetContext(key).(string); ok {
		return value
	}
	return fallback
}

func TestManagedCompatibleSupplierKeepsBusinessParametersAndDeclaredAPIs(t *testing.T) {
	var config ProviderConfig
	config.FromJson(gjson.Parse(`{"type":"openrouter","apiTokens":["pool-a","pool-b"],"providerBasePath":"/egress","modelMapping":{}}`))
	require.NoError(t, config.Validate())
	adapter, err := NewManagedAdapter(config, ApiNameChatCompletion, "/v1/chat/completions?region=owned")
	require.NoError(t, err)
	ctx := &managedAdapterContext{newMapCtx()}
	headers := make(http.Header)
	headers.Set(":path", "/client-alias?region=client")
	body, err := adapter.Prepare(ctx, []byte(`{"model":"frozen-upstream","messages":[],"reasoning_max_tokens":50}`), headers, "frozen-upstream", "pool-b", false)
	require.NoError(t, err)
	require.Equal(t, "/egress/api/v1/chat/completions?region=owned", headers.Get(":path"))
	require.Equal(t, "pool-b", strings.TrimPrefix(headers.Get("authorization"), "Bearer "))
	require.Equal(t, "frozen-upstream", gjson.GetBytes(body, "model").String())
	require.Equal(t, int64(50), gjson.GetBytes(body, "reasoning.max_tokens").Int())
	require.False(t, gjson.GetBytes(body, "reasoning_max_tokens").Exists())
	_, err = NewManagedAdapter(config, ApiNameEmbeddings, "/v1/embeddings")
	require.Error(t, err)
}
func (c *managedAdapterContext) GetBoolContext(key string, fallback bool) bool {
	if value, ok := c.GetContext(key).(bool); ok {
		return value
	}
	return fallback
}

func TestManagedSupplierRequestUsesOriginalAdaptersAndTheFrozenKey(t *testing.T) {
	for _, test := range []struct {
		name, config, path, authName, authValue string
		apiName                                 ApiName
		nativeQwen                              bool
	}{
		{"qwen-compatible", `{"type":"qwen","protocol":"original","qwenEnableCompatible":true,"qwenEnableSearch":true}`, "/compatible-mode/v1/chat/completions", "authorization", "Bearer pool-b", ApiNameChatCompletion, false},
		{"qwen-native", `{"type":"qwen","protocol":"original","qwenEnableCompatible":false,"qwenEnableSearch":true}`, "/api/v1/services/aigc/text-generation/generation", "authorization", "Bearer pool-b", ApiNameChatCompletion, true},
		{"deepseek", `{"type":"deepseek"}`, "/v1/chat/completions", "authorization", "Bearer pool-b", ApiNameChatCompletion, false},
		{"groq", `{"type":"groq"}`, "/openai/v1/chat/completions", "authorization", "Bearer pool-b", ApiNameChatCompletion, false},
		{"grok", `{"type":"grok"}`, "/v1/chat/completions", "authorization", "Bearer pool-b", ApiNameChatCompletion, false},
		{"yi", `{"type":"yi"}`, "/v1/chat/completions", "authorization", "Bearer pool-b", ApiNameChatCompletion, false},
		{"baichuan", `{"type":"baichuan"}`, "/v1/chat/completions", "authorization", "Bearer pool-b", ApiNameChatCompletion, false},
		{"ai360", `{"type":"ai360"}`, "/v1/chat/completions", "authorization", "pool-b", ApiNameChatCompletion, false},
		{"github", `{"type":"github"}`, "/chat/completions", "authorization", "pool-b", ApiNameChatCompletion, false},
		{"stepfun", `{"type":"stepfun"}`, "/v1/chat/completions", "authorization", "Bearer pool-b", ApiNameChatCompletion, false},
		{"mistral", `{"type":"mistral"}`, "/v1/chat/completions", "authorization", "Bearer pool-b", ApiNameChatCompletion, false},
		{"longcat", `{"type":"longcat"}`, "/v1/chat/completions", "authorization", "Bearer pool-b", ApiNameChatCompletion, false},
		{"fireworks", `{"type":"fireworks"}`, "/v1/chat/completions", "authorization", "Bearer pool-b", ApiNameChatCompletion, false},
		{"together-ai", `{"type":"together-ai"}`, "/v1/chat/completions", "authorization", "Bearer pool-b", ApiNameChatCompletion, false},
		{"doubao", `{"type":"doubao"}`, "/api/v3/chat/completions", "authorization", "Bearer pool-b", ApiNameChatCompletion, false},
		{"openrouter", `{"type":"openrouter"}`, "/api/v1/chat/completions", "authorization", "Bearer pool-b", ApiNameChatCompletion, false},
		{"baidu", `{"type":"baidu"}`, "/v2/chat/completions", "authorization", "Bearer pool-b", ApiNameChatCompletion, false},
		{"ollama", `{"type":"ollama","ollamaServerHost":"frozen-ollama","ollamaServerPort":11434}`, "/v1/chat/completions", "authorization", "", ApiNameChatCompletion, false},
		{"vllm", `{"type":"vllm","vllmCustomUrl":"https://wire.example/egress"}`, "/egress/chat/completions", "authorization", "Bearer pool-b", ApiNameChatCompletion, false},
		{"vllm-full", `{"type":"vllm","vllmCustomUrl":"https://wire.example/egress/custom/completions"}`, "/egress/custom/completions", "authorization", "Bearer pool-b", ApiNameChatCompletion, false},
		{"zhipuai", `{"type":"zhipuai"}`, "/api/paas/v4/chat/completions", "authorization", "Bearer pool-b", ApiNameChatCompletion, false},
		{"zhipuai-code", `{"type":"zhipuai","zhipuCodePlanMode":true,"zhipuDomain":"api.z.ai"}`, "/api/coding/paas/v4/chat/completions", "authorization", "Bearer pool-b", ApiNameChatCompletion, false},
		{"cloudflare", `{"type":"cloudflare","cloudflareAccountId":"frozen-account"}`, "/client/v4/accounts/frozen-account/ai/v1/chat/completions", "authorization", "Bearer pool-b", ApiNameChatCompletion, false},
		{"hunyuan-compatible", `{"type":"hunyuan"}`, "/v1/chat/completions", "authorization", "Bearer pool-b", ApiNameChatCompletion, false},
		{"minimax-v2", `{"type":"minimax","minimaxApiType":"v2"}`, "/v1/text/chatcompletion_v2", "authorization", "Bearer pool-b", ApiNameChatCompletion, false},
		{"azure", `{"type":"azure","azureServiceUrl":"https://wire.example/openai/deployments/frozen-upstream?api-version=2024-10-21"}`, "/openai/deployments/frozen-upstream/chat/completions?api-version=2024-10-21", "api-key", "pool-b", ApiNameChatCompletion, false},
		{"azure-frozen-deployment", `{"type":"azure","azureServiceUrl":"https://wire.example/openai/deployments/other-deployment?api-version=2024-10-21"}`, "/openai/deployments/other-deployment/chat/completions?api-version=2024-10-21", "api-key", "pool-b", ApiNameChatCompletion, false},
		{"azure-frozen-full-url", `{"type":"azure","azureServiceUrl":"https://wire.example/openai/deployments/other-deployment/chat/completions?api-version=2024-10-21"}`, "/openai/deployments/other-deployment/chat/completions?api-version=2024-10-21", "api-key", "pool-b", ApiNameChatCompletion, false},
		{"claude", `{"type":"claude","claudeVersion":"fixture-version","claudeCodeMode":false}`, "/v1/messages", "x-api-key", "pool-b", ApiNameAnthropicMessages, false},
		{"claude-code", `{"type":"claude","claudeVersion":"fixture-version","claudeCodeMode":true}`, "/v1/messages?beta=true", "authorization", "Bearer pool-b", ApiNameAnthropicMessages, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw := strings.TrimSuffix(test.config, "}") + `,"id":"original-provider","apiTokens":["pool-a","pool-b"],"modelMapping":{},"failover":{"enabled":false},"retryOnFailure":{"enabled":false}}`
			var config ProviderConfig
			config.FromJson(gjson.Parse(raw))
			require.NoError(t, config.Validate())
			adapter, err := NewManagedAdapter(config, test.apiName, "/v1/chat/completions")
			require.NoError(t, err)
			require.NotNil(t, adapter)
			ctx := &managedAdapterContext{newMapCtx()}
			headers := make(http.Header)
			headers.Set(":path", "/original-path")
			headers.Set("authorization", "Bearer must-be-replaced")
			body, err := adapter.Prepare(ctx, []byte(`{"model":"frozen-upstream","messages":[{"role":"user","content":"hello"}],"stream":true,"max_tokens":12}`), headers, "frozen-upstream", "pool-b", true)
			require.NoError(t, err)
			require.Equal(t, test.path, headers.Get(":path"))
			require.Equal(t, test.authValue, headers.Get(test.authName))
			require.Equal(t, test.nativeQwen, adapter.NativeQwen())
			if test.nativeQwen {
				require.Equal(t, "hello", gjson.GetBytes(body, "input.messages.0.content").String())
				require.True(t, gjson.GetBytes(body, "parameters.enable_search").Bool())
				require.Equal(t, "enable", headers.Get("X-DashScope-SSE"))
			}
			if strings.HasPrefix(test.name, "azure") {
				require.Empty(t, headers.Get("authorization"))
			}
			if strings.HasPrefix(test.name, "claude") {
				require.Equal(t, "fixture-version", headers.Get("anthropic-version"))
				if test.name == "claude-code" {
					require.NotEmpty(t, headers.Get("anthropic-beta"))
				}
			}
		})
	}
}

func TestManagedSupplierDoesNotReinterpretNativeModesAsCompatible(t *testing.T) {
	for _, raw := range []string{
		`{"type":"hunyuan","hunyuanAuthId":"123456789012345678901234567890123456","hunyuanAuthKey":"12345678901234567890123456789012"}`,
		`{"type":"minimax","apiTokens":["frozen-key"],"minimaxApiType":"pro","minimaxGroupId":"frozen-group"}`,
	} {
		var config ProviderConfig
		config.FromJson(gjson.Parse(raw))
		require.NoError(t, config.Validate())
		_, err := NewManagedAdapter(config, ApiNameChatCompletion, "/v1/chat/completions")
		require.Error(t, err)
	}
}

func TestManagedZhipuKeepsCodePlanAndConvertsReasoning(t *testing.T) {
	var config ProviderConfig
	config.FromJson(gjson.Parse(`{"type":"zhipuai","apiTokens":["pool-a","pool-b"],"zhipuCodePlanMode":true,"zhipuDomain":"api.z.ai"}`))
	require.NoError(t, config.Validate())
	adapter, err := NewManagedAdapter(config, ApiNameChatCompletion, "/v1/chat/completions?region=owned")
	require.NoError(t, err)
	headers := make(http.Header)
	body, err := adapter.Prepare(&managedAdapterContext{newMapCtx()}, []byte(`{"model":"frozen-upstream","messages":[],"reasoning_effort":"high"}`), headers, "frozen-upstream", "pool-b", false)
	require.NoError(t, err)
	require.Equal(t, "/api/coding/paas/v4/chat/completions?region=owned", headers.Get(":path"))
	require.Equal(t, "frozen-upstream", gjson.GetBytes(body, "model").String())
	require.Equal(t, "enabled", gjson.GetBytes(body, "thinking.type").String())
	require.False(t, gjson.GetBytes(body, "reasoning_effort").Exists())
	require.Equal(t, "Bearer pool-b", headers.Get("authorization"))
}

func TestManagedQwenAndDeepSeekProtocolsCoexistWithoutConnectionMutation(t *testing.T) {
	for _, supplier := range []string{"qwen", "deepseek"} {
		t.Run(supplier, func(t *testing.T) {
			var config ProviderConfig
			config.FromJson(gjson.Parse(`{"type":"` + supplier + `","apiTokens":["supplier-key"],"qwenEnableCompatible":false,"modelMapping":{}}`))
			require.NoError(t, config.Validate())
			paths := []string{"/compatible-mode/v1/chat/completions", "/compatible-mode/v1/responses", "/apps/anthropic/v1/messages"}
			if supplier == "deepseek" {
				paths = []string{"/v1/chat/completions", "/responses", "/anthropic/v1/messages"}
			}
			for i, api := range []ApiName{ApiNameChatCompletion, ApiNameResponses, ApiNameAnthropicMessages} {
				adapter, err := NewManagedAdapter(config, api, paths[i])
				require.NoError(t, err)
				headers := make(http.Header)
				headers.Set("x-api-key", "client-key")
				body := []byte(`{"model":"frozen-model","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"max_tokens":16}`)
				if api == ApiNameResponses {
					body = []byte(`{"model":"frozen-model","input":"hello","max_output_tokens":16}`)
				}
				prepared, err := adapter.Prepare(&managedAdapterContext{newMapCtx()}, body, headers, "frozen-model", "supplier-key", false)
				require.NoError(t, err)
				require.JSONEq(t, string(body), string(prepared))
				require.Equal(t, paths[i], headers.Get(":path"))
				require.Equal(t, "Bearer supplier-key", headers.Get("authorization"))
				require.Empty(t, headers.Get("x-api-key"))
				require.False(t, adapter.NativeQwen())
				if api == ApiNameAnthropicMessages {
					require.Equal(t, "2023-06-01", headers.Get("anthropic-version"))
				}
			}
			require.False(t, config.qwenEnableCompatible)
		})
	}
}
