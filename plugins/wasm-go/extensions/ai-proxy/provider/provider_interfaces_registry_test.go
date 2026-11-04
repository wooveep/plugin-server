package provider

import (
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"os"
	"testing"
)

func implementedManagedInterfaces() map[string][]string {
	allowed := map[string]bool{}
	for _, kind := range []string{providerTypeOpenAI, providerTypeQwen, providerTypeDeepSeek, providerTypeClaude, providerTypeAzure, providerTypeZhipuAi, providerTypeMinimax, providerTypeGemini, providerTypeHunyuan, providerTypeGroq, providerTypeGrok, providerTypeYi, providerTypeBaichuan, providerTypeAi360, providerTypeGithub, providerTypeStepfun, providerTypeMistral, providerTypeLongcat, providerTypeFireworks, providerTypeTogetherAI, providerTypeDoubao, providerTypeOpenRouter, providerTypeBaidu, providerTypeOllama, providerTypeVllm, providerTypeCloudflare, providerTypeMoonshot} {
		allowed[kind] = true
	}
	result := map[string][]string{}
	for kind, initializer := range providerInitializers {
		if !allowed[kind] {
			continue
		}
		var capabilities map[string]string
		if kind == "qwen" {
			capabilities = (&qwenProviderInitializer{}).DefaultCapabilities(true)
		} else if kind == providerTypeZhipuAi {
			capabilities = (&zhipuAiProviderInitializer{}).DefaultCapabilities(true)
		} else if declaration, ok := initializer.(interface{ DefaultCapabilities() map[string]string }); ok {
			capabilities = declaration.DefaultCapabilities()
		} else {
			continue
		}
		for _, entry := range []struct {
			api      ApiName
			protocol string
		}{{ApiNameChatCompletion, "openai_chat"}, {ApiNameResponses, "openai_responses"}, {ApiNameAnthropicMessages, "anthropic_messages"}, {ApiNameEmbeddings, "openai_embeddings"}} {
			if _, exists := capabilities[string(entry.api)]; !exists {
				continue
			}
			if kind == "claude" && entry.api != ApiNameAnthropicMessages {
				continue
			}
			if kind == "gemini" || kind == providerTypeZhipuAi {
				if entry.api != ApiNameChatCompletion && entry.api != ApiNameEmbeddings {
					continue
				}
			}
			if kind == "minimax" && entry.api != ApiNameChatCompletion {
				continue
			}
			if kind == "azure" && entry.api == ApiNameAnthropicMessages {
				continue
			}
			result[kind] = append(result[kind], entry.protocol)
		}
		if kind == "qwen" {
			result[kind] = append(result[kind], "dashscope_text", "dashscope_multimodal")
		}
	}
	return result
}

func TestManagedProviderInterfaceDeclaration(t *testing.T) {
	raw, err := os.ReadFile("../../../resources/plugins/ai-proxy/spec.yaml")
	require.NoError(t, err)
	var document struct {
		Spec struct {
			ConfigSchema struct {
				Schema struct {
					Interfaces struct {
						Version       string                       `yaml:"schema_version"`
						Providers     map[string][]string          `yaml:"providers"`
						WireProtocols map[string]map[string]string `yaml:"wire_protocols"`
					} `yaml:"x-modelfusion-provider-interfaces"`
				} `yaml:"openAPIV3Schema"`
			} `yaml:"configSchema"`
		} `yaml:"spec"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &document))
	declaration := document.Spec.ConfigSchema.Schema.Interfaces
	require.Equal(t, "provider-interfaces.v1", declaration.Version)
	require.Equal(t, implementedManagedInterfaces(), declaration.Providers)
	require.Equal(t, map[string]map[string]string{"gemini": {"openai_chat": "gemini_generate_content", "openai_embeddings": "gemini_embeddings"}}, declaration.WireProtocols)
}
