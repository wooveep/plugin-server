package main

import (
	"encoding/json"
	"strings"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/provider"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

const (
	adapterMetadataProperty = "mse_ai_proxy_adapter_metadata"
	maxAdapterMetadataBytes = 1024
	anyProviderMatch        = "*"
)

type protocolAdapter struct {
	id                        string
	version                   string
	provider                  string
	method                    string
	operation                 provider.ApiName
	requiredCapability        provider.ApiName
	requiresUnsupportedNative provider.ApiName
	normalizedOperation       string
	capabilityMetadata        string
	usageProfile              string
}

type resolvedProtocolAdapter struct {
	adapter       protocolAdapter
	providerMatch string
}

type adapterMetadata struct {
	SchemaVersion       int    `json:"schema_version"`
	Matched             bool   `json:"matched"`
	Mode                string `json:"mode"`
	AdapterID           string `json:"adapter_id,omitempty"`
	Version             string `json:"version,omitempty"`
	NormalizedOperation string `json:"normalized_operation,omitempty"`
	ProviderMatch       string `json:"provider_match,omitempty"`
	CapabilityMatch     string `json:"capability_match,omitempty"`
	UsageProfile        string `json:"usage_profile,omitempty"`
}

var reviewedProtocolAdapters = [...]protocolAdapter{
	{
		id:                        "qwen.responses_to_chat_completions",
		version:                   "1",
		provider:                  "qwen",
		method:                    "POST",
		operation:                 provider.ApiNameResponses,
		requiredCapability:        provider.ApiNameChatCompletion,
		requiresUnsupportedNative: provider.ApiNameResponses,
		normalizedOperation:       "openai/v1/responses",
		capabilityMetadata:        "openai/v1/chat/completions",
		usageProfile:              "openai.responses.v1",
	},
	{
		id:                        "responses.to_chat_completions",
		version:                   "1",
		provider:                  anyProviderMatch,
		method:                    "POST",
		operation:                 provider.ApiNameResponses,
		requiredCapability:        provider.ApiNameChatCompletion,
		requiresUnsupportedNative: provider.ApiNameResponses,
		normalizedOperation:       "openai/v1/responses",
		capabilityMetadata:        "openai/v1/chat/completions",
		usageProfile:              "openai.responses.v1",
	},
}

func resolveProtocolAdapter(
	activeProvider provider.Provider,
	providerConfig *provider.ProviderConfig,
	method string,
	operation provider.ApiName,
) *resolvedProtocolAdapter {
	if activeProvider == nil || providerConfig == nil {
		return nil
	}

	providerType := activeProvider.GetProviderType()
	normalizedMethod := strings.ToUpper(strings.TrimSpace(method))
	for _, adapter := range reviewedProtocolAdapters {
		if adapter.provider != anyProviderMatch && adapter.provider != providerType {
			continue
		}
		if adapter.method != normalizedMethod || adapter.operation != operation {
			continue
		}
		if adapter.requiresUnsupportedNative != "" &&
			providerConfig.IsSupportedAPI(adapter.requiresUnsupportedNative) {
			continue
		}
		if !providerConfig.IsSupportedAPI(adapter.requiredCapability) {
			continue
		}
		return &resolvedProtocolAdapter{
			adapter:       adapter,
			providerMatch: providerType,
		}
	}
	return nil
}

func publishNoAdapterMetadata(mode string) {
	publishAdapterMetadata(adapterMetadata{
		SchemaVersion: 1,
		Matched:       false,
		Mode:          mode,
	})
}

func publishMatchedAdapterMetadata(mode string, resolved *resolvedProtocolAdapter) {
	if resolved == nil {
		return
	}
	publishAdapterMetadata(adapterMetadata{
		SchemaVersion:       1,
		Matched:             true,
		Mode:                mode,
		AdapterID:           resolved.adapter.id,
		Version:             resolved.adapter.version,
		NormalizedOperation: resolved.adapter.normalizedOperation,
		ProviderMatch:       resolved.providerMatch,
		CapabilityMatch:     resolved.adapter.capabilityMetadata,
		UsageProfile:        resolved.adapter.usageProfile,
	})
}

func publishAdapterMetadata(metadata adapterMetadata) {
	payload, err := json.Marshal(metadata)
	if err != nil || len(payload) > maxAdapterMetadataBytes {
		log.Errorf("[Adapter Registry] failed to encode bounded adapter metadata")
		return
	}
	if err := proxywasm.SetProperty([]string{adapterMetadataProperty}, payload); err != nil {
		log.Errorf("[Adapter Registry] failed to publish adapter metadata")
	}
}

func applyResponsesFallbackAdapter(
	ctx wrapper.HttpContext,
	path string,
	mode string,
	resolved *resolvedProtocolAdapter,
) provider.ApiName {
	newPath := strings.Replace(
		path,
		provider.PathOpenAIResponses,
		provider.PathOpenAIChatCompletions,
		1,
	)
	_ = proxywasm.ReplaceHttpRequestHeader(":path", newPath)
	ctx.SetContext(provider.CtxKeyNeedResponsesResponseConversion, true)
	publishMatchedAdapterMetadata(mode, resolved)
	log.Debugf(
		"[Auto Protocol] Responses request detected, selected adapter and converted path from %s to %s, apiName: %s",
		path,
		newPath,
		provider.ApiNameChatCompletion,
	)
	return provider.ApiNameChatCompletion
}
