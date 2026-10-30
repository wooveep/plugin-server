package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVllmProviderInitializer_DefaultCapabilities(t *testing.T) {
	initializer := &vllmProviderInitializer{}

	capabilities := initializer.DefaultCapabilities()
	expected := map[string]string{
		string(ApiNameChatCompletion):       PathOpenAIChatCompletions,
		string(ApiNameCompletion):           PathOpenAICompletions,
		string(ApiNameModels):               PathOpenAIModels,
		string(ApiNameEmbeddings):           PathOpenAIEmbeddings,
		string(ApiNameCohereV1Rerank):       PathCohereV1Rerank,
		string(ApiNameAnthropicMessages):    PathAnthropicMessages,
		string(ApiNameAnthropicCountTokens): PathAnthropicMessagesCountTokens,
		string(ApiNameResponses):            PathOpenAIResponses,
		string(ApiNameAudioTranscription):   PathOpenAIAudioTranscriptions,
		string(ApiNameAudioTranslation):     PathOpenAIAudioTranslations,
	}

	assert.Equal(t, expected, capabilities)
}

func TestVllmProvider_GetApiName(t *testing.T) {
	provider := &vllmProvider{}

	cases := []struct {
		path     string
		expected ApiName
	}{
		{PathOpenAIChatCompletions, ApiNameChatCompletion},
		{PathOpenAICompletions, ApiNameCompletion},
		{PathOpenAIModels, ApiNameModels},
		{PathOpenAIEmbeddings, ApiNameEmbeddings},
		{PathCohereV1Rerank, ApiNameCohereV1Rerank},
		{PathAnthropicMessagesCountTokens, ApiNameAnthropicCountTokens},
		{PathAnthropicMessages, ApiNameAnthropicMessages},
		{PathOpenAIResponses, ApiNameResponses},
		{PathOpenAIAudioTranscriptions, ApiNameAudioTranscription},
		{PathOpenAIAudioTranslations, ApiNameAudioTranslation},
		{"/v1/unknown", ApiName("")},
	}

	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			assert.Equal(t, c.expected, provider.GetApiName(c.path))
		})
	}
}

func TestVllm_isVllmDirectPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/v1/chat/completions", true},
		{"/v1/completions", true},
		{"/v1/rerank", true},
		{"/v1/responses", true},
		{"/v1/messages", true},
		{"/v1/messages/count_tokens", true},
		{"/v1/audio/transcriptions", true},
		{"/v1/audio/translations", true},
		{"/v1", false},
		{"/", false},
		{"/custom", false},
	}

	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			assert.Equal(t, c.want, isVllmDirectPath(c.path))
		})
	}
}

func TestVllmProviderInitializer_CreateProvider_customUrl(t *testing.T) {
	initializer := &vllmProviderInitializer{}

	cases := []struct {
		name       string
		customUrl  string
		wantDirect bool
		wantPath   string
		wantDomain string
		capability ApiName
		wantCap    string
	}{
		{
			name:       "base path v1",
			customUrl:  "http://host:8000/v1",
			wantDirect: false,
			wantDomain: "host:8000",
			capability: ApiNameResponses,
			wantCap:    "/v1/responses",
		},
		{
			name:       "custom base path",
			customUrl:  "http://host:8000/custom",
			wantDirect: false,
			wantDomain: "host:8000",
			capability: ApiNameAnthropicMessages,
			wantCap:    "/custom/messages",
		},
		{
			name:       "direct responses endpoint",
			customUrl:  "http://host:8000/v1/responses",
			wantDirect: true,
			wantPath:   "/v1/responses",
			wantDomain: "host:8000",
		},
		{
			name:       "direct anthropic messages endpoint",
			customUrl:  "http://host:8000/v1/messages",
			wantDirect: true,
			wantPath:   "/v1/messages",
			wantDomain: "host:8000",
		},
		{
			name:       "direct count_tokens endpoint",
			customUrl:  "http://host:8000/v1/messages/count_tokens",
			wantDirect: true,
			wantPath:   "/v1/messages/count_tokens",
			wantDomain: "host:8000",
		},
		{
			name:       "direct audio transcription endpoint",
			customUrl:  "http://host:8000/v1/audio/transcriptions",
			wantDirect: true,
			wantPath:   "/v1/audio/transcriptions",
			wantDomain: "host:8000",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := initializer.CreateProvider(ProviderConfig{vllmCustomUrl: c.customUrl})
			assert.NoError(t, err)
			vp, ok := p.(*vllmProvider)
			assert.True(t, ok)
			assert.Equal(t, c.wantDirect, vp.isDirectCustomPath)
			assert.Equal(t, c.wantDomain, vp.customDomain)
			if c.wantDirect {
				assert.Equal(t, c.wantPath, vp.customPath)
			}
			if c.capability != "" {
				assert.Equal(t, c.wantCap, vp.config.capabilities[string(c.capability)])
			}
		})
	}
}

func TestVllm_passthroughBodyAndSupport(t *testing.T) {
	cfg := &ProviderConfig{}
	assert.False(t, cfg.needToProcessRequestBody(ApiNameAudioTranscription))
	assert.False(t, cfg.needToProcessRequestBody(ApiNameAudioTranslation))
	assert.True(t, cfg.needToProcessRequestBody(ApiNameAnthropicMessages))
	assert.True(t, cfg.needToProcessRequestBody(ApiNameAnthropicCountTokens))
	assert.True(t, cfg.needToProcessRequestBody(ApiNameResponses))

	vllmCfg := &ProviderConfig{}
	vllmCfg.setDefaultCapabilities((&vllmProviderInitializer{}).DefaultCapabilities())
	assert.True(t, vllmCfg.isSupportedAPI(ApiNameAnthropicCountTokens))

	otherCfg := &ProviderConfig{}
	otherCfg.setDefaultCapabilities(map[string]string{
		string(ApiNameChatCompletion): PathOpenAIChatCompletions,
	})
	assert.False(t, otherCfg.isSupportedAPI(ApiNameAnthropicCountTokens))
}
