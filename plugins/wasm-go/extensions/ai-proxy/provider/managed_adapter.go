package provider

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/util"
	"github.com/higress-group/wasm-go/pkg/ai/canonical"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
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
	if apiName == ApiNameCompletion {
		if config.typ != providerTypeDeepSeek || upstreamPath != deepseekFIMPath ||
			config.failover.enabled || config.retryOnFailure.enabled {
			return nil, errors.New("managed Completions requires the registered DeepSeek FIM path")
		}
		// The managed sender already owns the frozen path, Bearer key and body.
		return nil, nil
	}
	if apiName == ApiNameDashScopeText || apiName == ApiNameDashScopeMultimodal {
		expected := "/api/v1/services/aigc/text-generation/generation"
		if apiName == ApiNameDashScopeMultimodal {
			expected = "/api/v1/services/aigc/multimodal-generation/generation"
		}
		if config.typ != providerTypeQwen || upstreamPath != expected || config.failover.enabled || config.retryOnFailure.enabled {
			return nil, errors.New("managed DashScope requires its registered synchronous Qwen text path")
		}
		config.managedRequestPath = upstreamPath
		return &ManagedAdapter{config: config, apiName: apiName}, nil
	}
	switch config.typ {
	case providerTypeOpenAI, providerTypeGeneric:
		return nil, nil
	case providerTypeQwen:
		// The frozen binding selects the wire protocol independently of the
		// shared connection's historical compatible-mode default.
		if apiName == ApiNameAnthropicMessages || apiName == ApiNameResponses || strings.HasPrefix(upstreamPath, "/compatible-mode/v1/") {
			config.qwenEnableCompatible = true
			config.providerBasePath = ""
			// Complete() may already have populated the connection's native
			// defaults. This binding owns its wire path and a private map copy.
			capabilities := make(map[string]string, len(config.capabilities)+1)
			for name, path := range config.capabilities {
				capabilities[name] = path
			}
			capabilities[string(apiName)] = upstreamPath
			config.capabilities = capabilities
		} else if apiName == ApiNameChatCompletion && (upstreamPath == qwenChatCompletionPath || upstreamPath == qwenMultimodalGenerationPath) {
			config.qwenEnableCompatible = false
			config.providerBasePath = ""
			capabilities := make(map[string]string, len(config.capabilities)+1)
			for name, path := range config.capabilities {
				capabilities[name] = path
			}
			capabilities[string(apiName)] = upstreamPath
			config.capabilities = capabilities
		}
		if !config.qwenEnableCompatible && apiName != ApiNameChatCompletion && apiName != ApiNameEmbeddings {
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
	case providerTypeZhipuAi:
		if apiName != ApiNameChatCompletion && apiName != ApiNameEmbeddings {
			return nil, errors.New("managed ZhipuAI API is unsupported")
		}
	case providerTypeMinimax:
		if config.minimaxApiType == minimaxApiTypePro || apiName != ApiNameChatCompletion {
			return nil, errors.New("managed Minimax API is unsupported")
		}
	case providerTypeGemini:
		if apiName != ApiNameChatCompletion && apiName != ApiNameEmbeddings {
			return nil, errors.New("managed Gemini API is unsupported")
		}
	case providerTypeHunyuan:
		if config.hunyuanAuthId != "" || config.hunyuanAuthKey != "" {
			return nil, errors.New("managed Hunyuan native signing is unsupported")
		}
		fallthrough
	case providerTypeDeepSeek, providerTypeGroq, providerTypeGrok, providerTypeYi,
		providerTypeBaichuan, providerTypeAi360, providerTypeGithub, providerTypeStepfun,
		providerTypeMistral, providerTypeLongcat, providerTypeFireworks,
		providerTypeTogetherAI, providerTypeDoubao, providerTypeOpenRouter,
		providerTypeBaidu, providerTypeOllama, providerTypeVllm, providerTypeCloudflare,
		providerTypeMoonshot:
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
	return a.config.typ == providerTypeQwen && (!a.config.qwenEnableCompatible || a.apiName == ApiNameDashScopeText || a.apiName == ApiNameDashScopeMultimodal)
}

func (a *ManagedAdapter) NativeResponseFormat() string {
	if a.apiName == ApiNameDashScopeText || a.apiName == ApiNameDashScopeMultimodal {
		return ""
	}
	if a.NativeQwen() {
		return providerTypeQwen
	}
	if a.config.typ == providerTypeGemini || a.config.typ == providerTypeMoonshot {
		return a.config.typ
	}
	return ""
}

// ResolveCanonicalTarget freezes the immutable supplier wire format, API revision,
// model identity, and capability contract for a selected target without network I/O.
func ResolveCanonicalTarget(adapter *ManagedAdapter, upstreamProtocol, supplierType, upstreamPath, model string, streaming bool) CanonicalTarget {
	supplier := supplierType
	if adapter != nil && adapter.config.typ != "" {
		supplier = adapter.config.typ
	}

	var wireFormat WireFormat
	switch upstreamProtocol {
	case "openai_responses":
		wireFormat = WireOpenAIResponses
	case "anthropic_messages":
		wireFormat = WireAnthropicMessages
	case "dashscope_text":
		wireFormat = WireDashScopeText
	case "dashscope_multimodal":
		wireFormat = WireDashScopeMultimodal
	case "openai_chat", "":
		switch {
		case adapter != nil && adapter.config.typ == providerTypeGemini:
			wireFormat = WireGeminiGenerateContent
		case adapter != nil && adapter.NativeQwen():
			if adapter.apiName == ApiNameDashScopeMultimodal ||
				adapter.config.managedRequestPath == qwenMultimodalGenerationPath ||
				upstreamPath == qwenMultimodalGenerationPath {
				wireFormat = WireDashScopeMultimodal
			} else {
				wireFormat = WireDashScopeText
			}
		default:
			wireFormat = WireChatCompletions
		}
	default:
		wireFormat = WireFormat(upstreamProtocol)
	}

	var apiVersion string
	switch {
	case supplier == providerTypeGemini:
		if adapter != nil && adapter.config.apiVersion != "" {
			apiVersion = adapter.config.apiVersion
		} else {
			apiVersion = geminiDefaultApiVersion
		}
	case wireFormat == WireAnthropicMessages || supplier == providerTypeClaude:
		if adapter != nil && adapter.config.apiVersion != "" {
			apiVersion = adapter.config.apiVersion
		} else {
			apiVersion = claudeDefaultVersion
		}
	case supplier == providerTypeAzure && adapter != nil:
		if az, ok := adapter.provider.(*azureProvider); ok && az.serviceUrl != nil {
			apiVersion = az.serviceUrl.Query().Get(queryAzureApiVersion)
		}
	}

	isOpenAIOrAzure := supplier == providerTypeOpenAI || supplier == providerTypeAzure
	allowedDetails := []canonical.ImageDetail{
		canonical.ImageDetailAuto,
		canonical.ImageDetailLow,
		canonical.ImageDetailHigh,
	}
	if isOpenAIOrAzure || supplier == "" || wireFormat == WireGeminiGenerateContent {
		allowedDetails = append(allowedDetails, canonical.ImageDetailOriginal)
	}

	target := CanonicalTarget{
		WireFormat:               wireFormat,
		Supplier:                 supplier,
		Model:                    model,
		APIVersion:               apiVersion,
		Streaming:                streaming,
		SupportsDeveloperRole:    wireFormat == WireChatCompletions && (isOpenAIOrAzure || supplier == ""),
		AcceptedLimitScopes:      LimitScopeMaskAll,
		SupportsCompletionTokens: wireFormat == WireChatCompletions && supplier == providerTypeOpenAI,
		SupportsThinking: supplier == providerTypeQwen ||
			supplier == providerTypeDeepSeek ||
			supplier == providerTypeZhipuAi ||
			supplier == providerTypeMoonshot ||
			supplier == providerTypeOpenRouter ||
			supplier == providerTypeGemini ||
			supplier == providerTypeClaude,
		SupportsAdaptiveThinking: supplier == providerTypeClaude ||
			supplier == providerTypeQwen ||
			supplier == providerTypeGemini,
		SupportsPreserveThinking: supplier == providerTypeQwen && qwenSupportsPreserveThinking(model),
		ForbidsForcedToolThink: wireFormat == WireDashScopeText ||
			wireFormat == WireDashScopeMultimodal ||
			wireFormat == WireAnthropicMessages,
		IncrementalOutputMode: IncrementalAuto,
		SupportsStrictSchema: wireFormat != WireAnthropicMessages &&
			supplier != providerTypeDeepSeek &&
			model != "qwen-turbo" &&
			model != "qwen-long",
		SupportsURLImages:           wireFormat != WireDashScopeText && wireFormat != WireGeminiGenerateContent,
		AllowedImageDetails:         allowedDetails,
		SupportsStore:               wireFormat == WireChatCompletions && (isOpenAIOrAzure || supplier == ""),
		SupportsPromptCacheKey:      wireFormat == WireChatCompletions && supplier != providerTypeQwen,
		SupportsContentCacheControl: wireFormat == WireChatCompletions || wireFormat == WireDashScopeMultimodal || wireFormat == WireAnthropicMessages,
	}

	if supplier == providerTypeQwen && wireFormat == WireChatCompletions {
		qwenCompatibleChatPolicy(model).applyToTarget(&target)
	}
	if wireFormat == WireDashScopeText || wireFormat == WireDashScopeMultimodal {
		target.NormalizeEmptyAssistantContent = true
		target.StripToolMessageName = true
		target.StripOuterCacheControl = true
		if adapter != nil {
			target.QwenEnableSearchDefault = adapter.config.qwenEnableSearch
			if len(adapter.config.qwenFileIds) > 0 {
				target.QwenLongFileIDs = slices.Clone(adapter.config.qwenFileIds)
			}
		}
	}
	if wireFormat == WireGeminiGenerateContent && adapter != nil {
		if len(adapter.config.geminiSafetySetting) > 0 {
			target.GeminiSafetySettings = maps.Clone(adapter.config.geminiSafetySetting)
		}
		target.GeminiThinkingBudget = adapter.config.geminiThinkingBudget
	}
	return target
}

// ResolveCanonicalTarget resolves the frozen CanonicalTarget for this ManagedAdapter.
func (a *ManagedAdapter) ResolveCanonicalTarget(upstreamProtocol string, streaming bool, model string) CanonicalTarget {
	var supplierType, upstreamPath string
	if a != nil {
		supplierType = a.config.typ
		upstreamPath = a.config.managedRequestPath
	}
	return ResolveCanonicalTarget(a, upstreamProtocol, supplierType, upstreamPath, model, streaming)
}

// PrepareCanonical encodes req directly into target's wire format and configures
// upstream path, authentication, and SSE negotiation headers without intermediate
// Chat JSON decoding or wrapper.HttpContext state mutations.
func (a *ManagedAdapter) PrepareCanonical(_ wrapper.HttpContext, req canonical.Request, target CanonicalTarget, headers http.Header, credential string) ([]byte, error) {
	streaming := req.Stream || target.Streaming
	if a == nil {
		for _, name := range []string{"authorization", "x-api-key", "api-key", geminiApiKeyHeader} {
			headers.Del(name)
		}
		headers.Set("authorization", "Bearer "+credential)
		if streaming {
			headers.Set("accept", "text/event-stream")
		}
		return EncodeCanonicalRequest(req, target)
	}

	headers.Set(":path", a.config.managedRequestPath)
	for _, name := range []string{"authorization", "x-api-key", "api-key", geminiApiKeyHeader} {
		headers.Del(name)
	}

	if target.WireFormat == WireDashScopeText || target.WireFormat == WireDashScopeMultimodal ||
		a.apiName == ApiNameDashScopeText || a.apiName == ApiNameDashScopeMultimodal {
		headers.Set("authorization", "Bearer "+credential)
		if streaming {
			headers.Set("accept", "text/event-stream")
			headers.Set("x-dashscope-sse", "enable")
		} else {
			headers.Set("accept", "application/json")
			headers.Del("x-dashscope-sse")
		}
		if a.apiName != ApiNameDashScopeText && a.apiName != ApiNameDashScopeMultimodal {
			if !a.config.IsOriginal() {
				a.config.overwriteRequestPathHeaderByCapability(headers, string(a.apiName))
			}
			headers.Set(":path", a.config.applyProviderBasePath(headers.Get(":path")))
		}
	} else {
		if streaming {
			headers.Set("accept", "text/event-stream")
		}
		switch a.config.typ {
		case providerTypeGemini:
			headers.Set(geminiApiKeyHeader, credential)
			if gp, ok := a.provider.(*geminiProvider); ok {
				headers.Set(":path", a.config.applyProviderBasePath(gp.getRequestPath(a.apiName, target.Model, streaming)))
			}
		case providerTypeAzure:
			headers.Set("api-key", credential)
			if az, ok := a.provider.(*azureProvider); ok && !a.config.IsOriginal() {
				var resolvedPath string
				if az.serviceUrlType == azureServiceUrlTypeFull {
					resolvedPath = az.serviceUrlFullPath
				} else {
					resolvedPath = util.MapRequestPathByCapability(string(a.apiName), a.config.pathMappingInput(), a.config.capabilities)
					if strings.Contains(resolvedPath, pathAzureModelPlaceholder) {
						deployModel := target.Model
						if az.serviceUrlType == azureServiceUrlTypeWithDeployment || deployModel == "" {
							deployModel = az.defaultModel
						}
						resolvedPath = strings.ReplaceAll(resolvedPath, pathAzureModelPlaceholder, deployModel)
					}
					if az.serviceUrl != nil {
						resolvedPath = appendAzureServiceURLRawQuery(resolvedPath, az.serviceUrl.RawQuery)
					}
				}
				if resolvedPath != "" {
					headers.Set(":path", a.config.applyProviderBasePath(resolvedPath))
				}
			}
		case providerTypeClaude:
			if a.config.claudeCodeMode {
				headers.Set("authorization", "Bearer "+credential)
			} else {
				headers.Set("x-api-key", credential)
			}
			if headers.Get("anthropic-version") == "" {
				ver := a.config.apiVersion
				if ver == "" {
					ver = claudeDefaultVersion
				}
				headers.Set("anthropic-version", ver)
			}
			if !a.config.IsOriginal() {
				a.config.overwriteRequestPathHeaderByCapability(headers, string(a.apiName))
			}
			headers.Set(":path", a.config.applyProviderBasePath(headers.Get(":path")))
		case providerTypeMinimax:
			headers.Set("authorization", "Bearer "+credential)
			headers.Set(":path", a.config.applyProviderBasePath(minimaxChatCompletionV2Path))
		case providerTypeCloudflare:
			headers.Set("authorization", "Bearer "+credential)
			headers.Set(":path", a.config.applyProviderBasePath(strings.Replace(cloudflareChatCompletionPath, "{account_id}", a.config.cloudflareAccountId, 1)))
		case providerTypeMistral:
			headers.Set("authorization", "Bearer "+credential)
			headers.Set(":path", a.config.applyProviderBasePath(headers.Get(":path")))
		case providerTypeOllama:
			if !a.config.IsOriginal() {
				a.config.overwriteRequestPathHeaderByCapability(headers, string(a.apiName))
			}
			headers.Set(":path", a.config.applyProviderBasePath(headers.Get(":path")))
		case providerTypeVllm:
			if len(a.config.apiTokens) > 0 {
				headers.Set("authorization", "Bearer "+credential)
			}
			if vp, ok := a.provider.(*vllmProvider); ok && vp.isDirectCustomPath {
				headers.Set(":path", a.config.applyProviderBasePath(vp.customPath))
			} else if !a.config.IsOriginal() {
				a.config.overwriteRequestPathHeaderByCapability(headers, string(a.apiName))
				headers.Set(":path", a.config.applyProviderBasePath(headers.Get(":path")))
			}
		case providerTypeAi360, providerTypeGithub:
			headers.Set("authorization", credential)
			if !a.config.IsOriginal() {
				a.config.overwriteRequestPathHeaderByCapability(headers, string(a.apiName))
			}
			headers.Set(":path", a.config.applyProviderBasePath(headers.Get(":path")))
		default:
			headers.Set("authorization", "Bearer "+credential)
			if !a.config.IsOriginal() {
				a.config.overwriteRequestPathHeaderByCapability(headers, string(a.apiName))
			}
			headers.Set(":path", a.config.applyProviderBasePath(headers.Get(":path")))
		}
	}

	body, err := EncodeCanonicalRequest(req, target)
	if err != nil {
		return nil, err
	}

	if target.WireFormat == WireChatCompletions {
		switch a.config.typ {
		case providerTypeZhipuAi:
			if gjson.GetBytes(body, "reasoning_effort").String() != "" {
				body, _ = sjson.SetBytes(body, "thinking", map[string]string{"type": "enabled"})
				body, _ = sjson.DeleteBytes(body, "reasoning_effort")
			} else if req.SourceProtocol == canonical.ProtocolAnthropicMessages &&
				req.Thinking.Mode != canonical.ThinkingEnabled &&
				req.Thinking.Mode != canonical.ThinkingAdaptive {
				body, _ = sjson.SetBytes(body, "thinking", map[string]string{"type": "disabled"})
			}
			if requestBodyHasMessageReasoningContent(body) {
				body, _ = sjson.SetBytes(body, "thinking.clear_thinking", false)
			}
		case providerTypeOpenRouter:
			if req.SourceProtocol == canonical.ProtocolAnthropicMessages &&
				req.Thinking.Mode == canonical.ThinkingEnabled &&
				req.Thinking.BudgetTokens.ValueOr(0) > 0 {
				body, _ = sjson.DeleteBytes(body, "reasoning_effort")
				body, err = sjson.SetBytes(body, "reasoning.max_tokens", req.Thinking.BudgetTokens.Val)
				if err != nil {
					return nil, fmt.Errorf("set openrouter reasoning.max_tokens: %w", err)
				}
			}
		}
	}
	return body, nil
}

// Prepare transforms the actual wire body and headers using the selected key.
// The caller freezes these resulting bytes and path before its sole send.
func (a *ManagedAdapter) Prepare(ctx wrapper.HttpContext, body []byte, headers http.Header, model, credential string, streaming bool) ([]byte, error) {
	search := gjson.GetBytes(body, "enable_search")
	codeInterpreter := gjson.GetBytes(body, "enable_code_interpreter")
	searchOptions := gjson.GetBytes(body, "search_options")
	headers.Set(":path", a.config.managedRequestPath)
	if a.apiName == ApiNameDashScopeText || a.apiName == ApiNameDashScopeMultimodal {
		headers.Del("x-api-key")
		headers.Del("api-key")
		headers.Set("authorization", "Bearer "+credential)
		if streaming {
			headers.Set("accept", "text/event-stream")
			headers.Set("x-dashscope-sse", "enable")
		} else {
			// DashScope also negotiates SSE from Accept, independently of its
			// control header. A JSON request must not advertise event streams.
			headers.Set("accept", "application/json")
			headers.Del("x-dashscope-sse")
		}
		return body, nil
	}
	ctx.SetContext(a.config.failover.ctxApiTokenInUse, credential)
	ctx.SetContext(ctxKeyOriginalRequestModel, model)
	ctx.SetContext(ctxKeyFinalRequestModel, model)
	ctx.SetContext(ctxKeyIsStreaming, streaming)
	if streaming {
		headers.Set("accept", "text/event-stream")
	}
	if a.apiName == ApiNameAnthropicMessages && headers.Get("anthropic-version") == "" {
		// Both registered Messages suppliers accept this protocol revision.
		headers.Set("anthropic-version", "2023-06-01")
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
	if a.NativeQwen() {
		if search.Exists() {
			// A request-level false overrides the connection's legacy default.
			body, err = sjson.SetBytes(body, "parameters.enable_search", search.Bool())
			if err != nil {
				return nil, err
			}
		}
		if codeInterpreter.Exists() {
			body, err = sjson.SetBytes(body, "parameters.enable_code_interpreter", codeInterpreter.Bool())
			if err != nil {
				return nil, err
			}
		}
		if searchOptions.IsObject() {
			body, err = sjson.SetRawBytes(body, "parameters.search_options", []byte(searchOptions.Raw))
			if err != nil {
				return nil, err
			}
		}
	}
	headers.Set(":path", a.config.applyProviderBasePath(headers.Get(":path")))
	return body, nil
}

// ConvertNativeResponse only shapes the client payload. Native usage is observed
// separately from the original response before this content conversion.
func (a *ManagedAdapter) ConvertNativeResponse(ctx wrapper.HttpContext, body []byte, streaming bool) ([]byte, error) {
	if a.NativeResponseFormat() == "" {
		return body, nil
	}
	if !streaming {
		handler, ok := a.provider.(TransformResponseBodyHandler)
		if !ok {
			if a.config.typ == providerTypeMoonshot {
				return body, nil
			}
			return nil, errors.New("native supplier response conversion is unavailable")
		}
		return handler.TransformResponseBody(ctx, a.apiName, body)
	}
	handler, ok := a.provider.(StreamingEventHandler)
	if !ok {
		if streamHandler, ok := a.provider.(StreamingResponseBodyHandler); ok {
			converted, err := streamHandler.OnStreamingResponseBody(ctx, a.apiName, body, false)
			if err != nil {
				return nil, err
			}
			return append(converted, []byte("data: [DONE]\n\n")...), nil
		}
		return nil, errors.New("native supplier stream conversion is unavailable")
	}
	var result strings.Builder
	for _, event := range ExtractStreamingEvents(ctx, body) {
		if event.IsEndData() {
			continue
		}
		events, err := handler.OnStreamingEvent(ctx, a.apiName, event)
		if err != nil {
			return nil, fmt.Errorf("convert native supplier stream: %w", err)
		}
		for _, converted := range events {
			result.WriteString(converted.ToHttpString())
		}
	}
	// The caller invokes this only after verifying the original native terminal.
	result.WriteString("data: [DONE]\n\n")
	return []byte(result.String()), nil
}

// ConvertNativeStreamEvent converts one already delimited native SSE event.
// Completion and exact usage remain owned by the caller's original observer.
func (a *ManagedAdapter) ConvertNativeStreamEvent(ctx wrapper.HttpContext, event StreamEvent) ([]byte, error) {
	if handler, ok := a.provider.(StreamingEventHandler); ok {
		events, err := handler.OnStreamingEvent(ctx, a.apiName, event)
		if err != nil {
			return nil, err
		}
		var result strings.Builder
		for _, converted := range events {
			result.WriteString(converted.ToHttpString())
		}
		return []byte(result.String()), nil
	}
	if handler, ok := a.provider.(StreamingResponseBodyHandler); ok {
		return handler.OnStreamingResponseBody(ctx, a.apiName, []byte(event.ToHttpString()), false)
	}
	return nil, errors.New("native supplier stream conversion is unavailable")
}
