package provider

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/higress-group/wasm-go/pkg/wrapper"
)

// ManagedAdapter reuses supplier conversions after the native executor has
// selected its immutable model and credential. It never invokes an OnRequest
// handler, selects another key, installs failover state, or sends an HTTP call.
type ManagedAdapter struct {
	provider Provider
	config   ProviderConfig
	apiName  ApiName
}

// NewManagedAdapter receives the already parsed provider configuration. The API
// name describes the prepared logical payload; the old provider protocol flag
// cannot reinterpret that payload as an opaque original client request.
func NewManagedAdapter(config ProviderConfig, apiName ApiName, upstreamPath string) (*ManagedAdapter, error) {
	switch config.typ {
	case providerTypeOpenAI, providerTypeGeneric:
		return nil, nil
	case providerTypeQwen:
		if apiName == ApiNameAnthropicMessages || !config.qwenEnableCompatible && apiName != ApiNameChatCompletion && apiName != ApiNameEmbeddings {
			return nil, errors.New("managed Qwen API is unsupported")
		}
	case providerTypeAzure:
		if apiName == ApiNameAnthropicMessages {
			return nil, errors.New("managed Azure API is unsupported")
		}
	case providerTypeClaude:
		if apiName != ApiNameAnthropicMessages {
			return nil, errors.New("managed Claude requires its Messages payload")
		}
	case providerTypeDeepSeek, providerTypeGroq, providerTypeGrok, providerTypeYi,
		providerTypeBaichuan, providerTypeAi360, providerTypeGithub, providerTypeStepfun,
		providerTypeMistral, providerTypeLongcat, providerTypeFireworks,
		providerTypeTogetherAI, providerTypeDoubao, providerTypeOpenRouter:
		initializer, declared := providerInitializers[config.typ].(interface{ DefaultCapabilities() map[string]string })
		if !declared {
			return nil, errors.New("managed supplier has no declared API capabilities")
		}
		if _, supported := initializer.DefaultCapabilities()[string(apiName)]; !supported {
			return nil, errors.New("managed supplier does not support this API")
		}
	default:
		return nil, errors.New("managed supplier adapter is unsupported")
	}
	if config.failover.enabled || config.retryOnFailure.enabled {
		return nil, errors.New("managed supplier cannot enable another sender")
	}
	config.managedRequestPath = upstreamPath
	config.managedRequest = true
	config.protocol = protocolOpenAI
	if apiName == ApiNameAnthropicMessages {
		config.protocol = protocolOriginal
	}
	config.initVariable()
	active, err := CreateProvider(config)
	if err != nil {
		return nil, fmt.Errorf("create managed supplier adapter: %w", err)
	}
	return &ManagedAdapter{provider: active, config: config, apiName: apiName}, nil
}

func (a *ManagedAdapter) NativeQwen() bool {
	return a.config.typ == providerTypeQwen && !a.config.qwenEnableCompatible
}

// Prepare transforms the actual wire body and headers using the selected key.
// The caller freezes these resulting bytes and path before its sole send.
func (a *ManagedAdapter) Prepare(ctx wrapper.HttpContext, body []byte, headers http.Header, model, credential string, streaming bool) ([]byte, error) {
	headers.Set(":path", a.config.managedRequestPath)
	ctx.SetContext(a.config.failover.ctxApiTokenInUse, credential)
	ctx.SetContext(ctxKeyOriginalRequestModel, model)
	ctx.SetContext(ctxKeyFinalRequestModel, model)
	ctx.SetContext(ctxKeyIsStreaming, streaming)
	if streaming {
		headers.Set("accept", "text/event-stream")
	}
	for _, name := range []string{"authorization", "x-api-key", "api-key"} {
		headers.Del(name)
	}
	if handler, ok := a.provider.(TransformRequestHeadersHandler); ok {
		handler.TransformRequestHeaders(ctx, a.apiName, headers)
	}
	var err error
	if handler, ok := a.provider.(TransformRequestBodyHandler); ok {
		body, err = handler.TransformRequestBody(ctx, a.apiName, body)
	} else if handler, ok := a.provider.(TransformRequestBodyHeadersHandler); ok {
		body, err = handler.TransformRequestBodyHeaders(ctx, a.apiName, body, headers)
	} else {
		body, err = a.config.defaultTransformRequestBody(ctx, a.apiName, body)
	}
	if err != nil {
		return nil, err
	}
	headers.Set(":path", a.config.applyProviderBasePath(headers.Get(":path")))
	return body, nil
}

// ConvertQwenResponse only shapes the client payload. Native usage is observed
// separately from the original response before this content conversion.
func (a *ManagedAdapter) ConvertQwenResponse(ctx wrapper.HttpContext, body []byte, streaming bool) ([]byte, error) {
	if !a.NativeQwen() {
		return body, nil
	}
	if !streaming {
		handler, ok := a.provider.(TransformResponseBodyHandler)
		if !ok {
			return nil, errors.New("native Qwen response conversion is unavailable")
		}
		return handler.TransformResponseBody(ctx, a.apiName, body)
	}
	handler, ok := a.provider.(StreamingEventHandler)
	if !ok {
		return nil, errors.New("native Qwen stream conversion is unavailable")
	}
	var result strings.Builder
	for _, event := range ExtractStreamingEvents(ctx, body) {
		if event.IsEndData() {
			continue
		}
		events, err := handler.OnStreamingEvent(ctx, a.apiName, event)
		if err != nil {
			return nil, fmt.Errorf("convert native Qwen stream: %w", err)
		}
		for _, converted := range events {
			result.WriteString(converted.ToHttpString())
		}
	}
	// The caller invokes this only after verifying the original native terminal.
	result.WriteString("data: [DONE]\n\n")
	return []byte(result.String()), nil
}
