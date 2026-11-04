package provider

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// The selected artifact exports this declaration to the native route compiler.
// It describes implemented logical adapters, not a model or account's permission.
func TestNativeTextRoutingDeclarationMatchesProviderCapabilities(t *testing.T) {
	raw, err := os.ReadFile("../../../resources/plugins/ai-proxy/spec.yaml")
	require.NoError(t, err)
	var document struct {
		Spec struct {
			ConfigSchema struct {
				Schema struct {
					Routing struct {
						Version   string              `yaml:"schema_version"`
						Providers map[string][]string `yaml:"providers"`
					} `yaml:"x-modelfusion-native-text-routing"`
				} `yaml:"openAPIV3Schema"`
			} `yaml:"configSchema"`
		} `yaml:"spec"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &document))
	declaration := document.Spec.ConfigSchema.Schema.Routing
	require.Equal(t, "native-text-routing.v1", declaration.Version)
	capabilities := map[string]map[string]string{
		"openai":   (&openaiProviderInitializer{}).DefaultCapabilities(),
		"qwen":     (&qwenProviderInitializer{}).DefaultCapabilities(true),
		"deepseek": (&deepseekProviderInitializer{}).DefaultCapabilities(),
		"claude":   (&claudeProviderInitializer{}).DefaultCapabilities(),
	}
	apis := map[string]ApiName{
		"openai_chat":        ApiNameChatCompletion,
		"openai_responses":   ApiNameResponses,
		"anthropic_messages": ApiNameAnthropicMessages,
	}
	require.Len(t, declaration.Providers, len(capabilities))
	for name, protocols := range declaration.Providers {
		for _, protocol := range protocols {
			api, exists := apis[protocol]
			require.True(t, exists, protocol)
			_, exists = capabilities[name][string(api)]
			require.True(t, exists, name+":"+protocol)
		}
	}
	// Managed Claude accepts native Messages only; its legacy Chat converter is
	// not registered as a managed Chat target.
	require.Equal(t, []string{"anthropic_messages"}, declaration.Providers["claude"])
}
