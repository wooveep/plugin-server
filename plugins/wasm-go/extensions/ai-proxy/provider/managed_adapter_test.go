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
