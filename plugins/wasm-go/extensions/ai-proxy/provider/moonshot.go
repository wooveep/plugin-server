package provider

import (
	"errors"
	"net/http"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/util"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// moonshotProvider is the provider for Moonshot AI service.

const (
	moonshotDomain = "api.moonshot.cn"
)

type moonshotProviderInitializer struct{}

func (m *moonshotProviderInitializer) ValidateConfig(config *ProviderConfig) error {
	if config.moonshotFileId != "" && config.context != nil {
		return errors.New("moonshotFileId and context cannot be configured at the same time")
	}
	if len(config.apiTokens) == 0 {
		return errors.New("no apiToken found in provider config")
	}
	return nil
}

func (m *moonshotProviderInitializer) DefaultCapabilities() map[string]string {
	return map[string]string{
		string(ApiNameChatCompletion): PathOpenAIChatCompletions,
		string(ApiNameModels):         PathOpenAIModels,
	}
}

func (m *moonshotProviderInitializer) CreateProvider(config ProviderConfig) (Provider, error) {
	config.setDefaultCapabilities(m.DefaultCapabilities())
	return &moonshotProvider{
		config:       config,
		contextCache: createContextCache(&config),
	}, nil
}

type moonshotProvider struct {
	config       ProviderConfig
	contextCache *contextCache
}

func (m *moonshotProvider) GetProviderType() string {
	return providerTypeMoonshot
}

func (m *moonshotProvider) OnRequestHeaders(ctx wrapper.HttpContext, apiName ApiName) error {
	m.config.handleRequestHeaders(m, ctx, apiName)
	return nil
}

func (m *moonshotProvider) TransformRequestHeaders(ctx wrapper.HttpContext, apiName ApiName, headers http.Header) {
	util.OverwriteRequestPathHeaderByCapability(headers, string(apiName), m.config.capabilities)
	util.OverwriteRequestHostHeader(headers, moonshotDomain)
	util.OverwriteRequestAuthorizationHeader(headers, "Bearer "+m.config.GetApiTokenInUse(ctx))
	headers.Del("Content-Length")
}

// moonshot 有自己获取 context 的配置（moonshotFileId），因此无法复用 handleRequestBody 方法
// moonshot 的 body 没有修改，无须实现TransformRequestBody，使用默认的 defaultTransformRequestBody 方法
func (m *moonshotProvider) OnRequestBody(ctx wrapper.HttpContext, apiName ApiName, body []byte) (types.Action, error) {
	if !m.config.isSupportedAPI(apiName) {
		return types.ActionContinue, errUnsupportedApiName
	}
	return m.config.handleRequestBody(m, m.contextCache, ctx, apiName, body)
}

func (m *moonshotProvider) OnStreamingEvent(ctx wrapper.HttpContext, name ApiName, event StreamEvent) ([]StreamEvent, error) {
	if name != ApiNameChatCompletion {
		return nil, nil
	}

	if gjson.Get(event.Data, "choices.0.usage").Exists() {
		usageStr := gjson.Get(event.Data, "choices.0.usage").Raw
		newData, err := sjson.Delete(event.Data, "choices.0.usage")
		if err != nil {
			log.Errorf("convert usage event error: %v", err)
			return nil, err
		}
		newData, err = sjson.SetRaw(newData, "usage", usageStr)
		if err != nil {
			log.Errorf("convert usage event error: %v", err)
			return nil, err
		}
		event.Data = newData
	}
	return []StreamEvent{event}, nil
}
