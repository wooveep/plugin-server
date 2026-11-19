package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/util"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// qwenProvider is the provider for Qwen service.

const (
	qwenResultFormatMessage = "message"

	qwenDefaultDomain                     = "dashscope.aliyuncs.com"
	qwenChatCompletionPath                = "/api/v1/services/aigc/text-generation/generation"
	qwenTextEmbeddingPath                 = "/api/v1/services/embeddings/text-embedding/text-embedding"
	qwenTextRerankPath                    = "/api/v1/services/rerank/text-rerank/text-rerank"
	qwenCompatibleTextRerankPath          = "/compatible-api/v1/reranks"
	qwenCompatibleChatCompletionPath      = "/compatible-mode/v1/chat/completions"
	qwenCompatibleCompletionsPath         = "/compatible-mode/v1/completions"
	qwenCompatibleTextEmbeddingPath       = "/compatible-mode/v1/embeddings"
	qwenCompatibleConversationsPath       = "/compatible-mode/v1/conversations"
	qwenCompatibleResponsesPath           = "/compatible-mode/v1/responses"
	qwenCompatibleFilesPath               = "/compatible-mode/v1/files"
	qwenCompatibleRetrieveFilePath        = "/compatible-mode/v1/files/{file_id}"
	qwenCompatibleRetrieveFileContentPath = "/compatible-mode/v1/files/{file_id}/content"
	qwenCompatibleBatchesPath             = "/compatible-mode/v1/batches"
	qwenCompatibleRetrieveBatchPath       = "/compatible-mode/v1/batches/{batch_id}"
	qwenBailianPath                       = "/api/v1/apps"
	qwenMultimodalGenerationPath          = "/api/v1/services/aigc/multimodal-generation/generation"
	qwenAnthropicMessagesPath             = "/apps/anthropic/v1/messages"

	qwenAsyncAIGCPath = "/api/v1/services/aigc/"
	qwenAsyncTaskPath = "/api/v1/tasks/"

	qwenTopPMin = 0.000001
	qwenTopPMax = 0.999999

	qwenDummySystemMessageContent = "You are a helpful assistant."

	qwenLongModelName     = "qwen-long"
	qwenVlModelPrefixName = "qwen-vl"
)

type qwenProviderInitializer struct{}

func (m *qwenProviderInitializer) ValidateConfig(config *ProviderConfig) error {
	if len(config.qwenFileIds) != 0 && config.context != nil {
		return errors.New("qwenFileIds and context cannot be configured at the same time")
	}
	if len(config.apiTokens) == 0 {
		return errors.New("no apiToken found in provider config")
	}
	return nil
}

func (m *qwenProviderInitializer) DefaultCapabilities(qwenEnableCompatible bool) map[string]string {
	if qwenEnableCompatible {
		return map[string]string{
			string(ApiNameChatCompletion):      qwenCompatibleChatCompletionPath,
			string(ApiNameEmbeddings):          qwenCompatibleTextEmbeddingPath,
			string(ApiNameCompletion):          qwenCompatibleCompletionsPath,
			string(ApiNameResponses):           qwenCompatibleResponsesPath,
			string(ApiNameFiles):               qwenCompatibleFilesPath,
			string(ApiNameRetrieveFile):        qwenCompatibleRetrieveFilePath,
			string(ApiNameRetrieveFileContent): qwenCompatibleRetrieveFileContentPath,
			string(ApiNameBatches):             qwenCompatibleBatchesPath,
			string(ApiNameRetrieveBatch):       qwenCompatibleRetrieveBatchPath,
			string(ApiNameQwenAsyncAIGC):       qwenAsyncAIGCPath,
			string(ApiNameQwenAsyncTask):       qwenAsyncTaskPath,
			string(ApiNameQwenV1Rerank):        qwenCompatibleTextRerankPath,
			string(ApiNameQwenV1Conversations): qwenCompatibleConversationsPath,
			string(ApiNameAnthropicMessages):   qwenAnthropicMessagesPath,
		}
	} else {
		return map[string]string{
			string(ApiNameChatCompletion):    qwenChatCompletionPath,
			string(ApiNameEmbeddings):        qwenTextEmbeddingPath,
			string(ApiNameQwenAsyncAIGC):     qwenAsyncAIGCPath,
			string(ApiNameQwenAsyncTask):     qwenAsyncTaskPath,
			string(ApiNameQwenV1Rerank):      qwenTextRerankPath,
			string(ApiNameAnthropicMessages): qwenAnthropicMessagesPath,
		}
	}
}

func (m *qwenProviderInitializer) CreateProvider(config ProviderConfig) (Provider, error) {
	config.setDefaultCapabilities(m.DefaultCapabilities(config.qwenEnableCompatible))
	return &qwenProvider{
		config:       config,
		contextCache: createContextCache(&config),
	}, nil
}

type qwenProvider struct {
	config       ProviderConfig
	contextCache *contextCache
}

func (m *qwenProvider) TransformRequestHeaders(ctx wrapper.HttpContext, apiName ApiName, headers http.Header) {
	if m.config.qwenDomain != "" {
		util.OverwriteRequestHostHeader(headers, m.config.qwenDomain)
	} else {
		util.OverwriteRequestHostHeader(headers, qwenDefaultDomain)
	}
	util.OverwriteRequestAuthorizationHeader(headers, "Bearer "+m.config.GetApiTokenInUse(ctx))

	if !m.config.IsOriginal() {
		m.config.overwriteRequestPathHeaderByCapability(headers, string(apiName))
	}
}

func (m *qwenProvider) TransformTransparentRequestHeaders(ctx wrapper.HttpContext, headers http.Header) {
	if m.config.qwenDomain != "" {
		util.OverwriteRequestHostHeader(headers, m.config.qwenDomain)
	} else {
		util.OverwriteRequestHostHeader(headers, qwenDefaultDomain)
	}
	clearTransparentCredentials(headers)
	headers.Set(util.HeaderAuthorization, "Bearer "+m.config.GetApiTokenInUse(ctx))
}

func (m *qwenProvider) TransformRequestBodyHeaders(ctx wrapper.HttpContext, apiName ApiName, body []byte, headers http.Header) ([]byte, error) {
	if m.config.qwenEnableCompatible {
		modifiedBody := body
		model := gjson.GetBytes(modifiedBody, "model")
		mappedModel := ""
		if model.Exists() {
			rawModel := model.String()
			mappedModel = getMappedModel(rawModel, m.config.modelMapping)
			newBody, err := sjson.SetBytes(modifiedBody, "model", mappedModel)
			if err != nil {
				log.Errorf("Replace model error: %v", err)
				return newBody, err
			}
			modifiedBody = newBody
		}
		policy := qwenCompatibleChatPolicy(mappedModel)
		if apiName == ApiNameChatCompletion {
			sanitized, err := sanitizeQwenCompatibleChatRequestWithPolicy(modifiedBody, policy)
			if err != nil {
				return modifiedBody, err
			}
			modifiedBody = sanitized
		}
		if mappedModel != "" && policy.TopLevelPreserveThinking && requestBodyHasMessageReasoningContent(modifiedBody) && !gjson.GetBytes(modifiedBody, "preserve_thinking").Exists() {
			// Qwen OpenAI-compatible mode requires top-level preserve_thinking=true
			// before historical reasoning_content can be reused by supported models.
			var err error
			modifiedBody, err = sjson.SetBytes(modifiedBody, "preserve_thinking", true)
			if err != nil {
				return modifiedBody, err
			}
		}
		return modifiedBody, nil
	}
	switch apiName {
	case ApiNameChatCompletion:
		return m.onChatCompletionRequestBody(ctx, body, headers)
	case ApiNameEmbeddings:
		return m.onEmbeddingsRequestBody(ctx, body)
	case ApiNameImageGeneration:
		return m.onImageGenerationRequestBody(ctx, body, headers)
	case ApiNameAudioSpeech:
		return m.onAudioSpeechRequestBody(ctx, body, headers)
	default:
		return m.config.defaultTransformRequestBody(ctx, apiName, body)
	}
}

func (m *qwenProvider) GetProviderType() string {
	return providerTypeQwen
}

func (m *qwenProvider) OnRequestHeaders(ctx wrapper.HttpContext, apiName ApiName) error {
	m.config.handleRequestHeaders(m, ctx, apiName)

	if m.config.protocol == protocolOriginal {
		ctx.DontReadRequestBody()
		return nil
	}

	return nil
}

func (m *qwenProvider) OnRequestBody(ctx wrapper.HttpContext, apiName ApiName, body []byte) (types.Action, error) {
	if !m.config.isSupportedAPI(apiName) {
		return types.ActionContinue, errUnsupportedApiName
	}
	return m.config.handleRequestBody(m, m.contextCache, ctx, apiName, body)
}

func (m *qwenProvider) onChatCompletionRequestBody(ctx wrapper.HttpContext, body []byte, headers http.Header) ([]byte, error) {
	request := &chatCompletionRequest{}
	err := m.config.parseRequestAndMapModel(ctx, request, body)
	if err != nil {
		return nil, err
	}

	// Use the qwen multimodal model generation API only in unmanaged legacy mode;
	// managed execution binds each target to its exact frozen upstream path.
	if !m.config.managedRequest && strings.HasPrefix(request.Model, qwenVlModelPrefixName) {
		util.OverwriteRequestPathHeader(headers, qwenMultimodalGenerationPath)
	}

	streaming := request.Stream
	if streaming {
		headers.Set("Accept", "text/event-stream")
		headers.Set("X-DashScope-SSE", "enable")
	} else {
		if m.config.managedRequest {
			headers.Set("Accept", "application/json")
		} else {
			headers.Set("Accept", "*/*")
		}
		headers.Del("X-DashScope-SSE")
	}

	return m.buildQwenGenerationRequest(ctx, request, body, streaming, headers.Get(":path") == qwenMultimodalGenerationPath)
}

func (m *qwenProvider) onEmbeddingsRequestBody(ctx wrapper.HttpContext, body []byte) ([]byte, error) {
	request := &embeddingsRequest{}
	if err := m.config.parseRequestAndMapModel(ctx, request, body); err != nil {
		return nil, err
	}

	qwenRequest, err := m.buildQwenTextEmbeddingRequest(request)
	if err != nil {
		return nil, err
	}
	return json.Marshal(qwenRequest)
}

func (m *qwenProvider) onImageGenerationRequestBody(ctx wrapper.HttpContext, body []byte, headers http.Header) ([]byte, error) {
	request := &imageGenerationRequest{}
	if err := m.config.parseRequestAndMapModel(ctx, request, body); err != nil {
		return nil, err
	}

	headers.Set("Accept", "*/*")
	headers.Del("X-DashScope-SSE")

	qwenRequest := m.buildQwenImageGenerationRequest(request, body)
	return json.Marshal(qwenRequest)
}

func (m *qwenProvider) onAudioSpeechRequestBody(ctx wrapper.HttpContext, body []byte, headers http.Header) ([]byte, error) {
	request := &qwenOpenAIAudioSpeechRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		return nil, fmt.Errorf("unable to unmarshal Qwen audio speech request: %v", err)
	}
	if err := m.config.mapModel(ctx, &request.Model); err != nil {
		return nil, err
	}

	headers.Set("Accept", "*/*")
	headers.Del("X-DashScope-SSE")

	qwenRequest := m.buildQwenAudioSpeechRequest(request)
	return json.Marshal(qwenRequest)
}

func (m *qwenProvider) OnStreamingEvent(ctx wrapper.HttpContext, name ApiName, event StreamEvent) ([]StreamEvent, error) {
	if m.config.qwenEnableCompatible || name != ApiNameChatCompletion {
		return nil, nil
	}

	incrementalStreaming := ctx.GetBoolContext(ctxKeyIncrementalStreaming, false)

	qwenResponse := &qwenTextGenResponse{}
	if err := json.Unmarshal([]byte(event.Data), qwenResponse); err != nil {
		log.Errorf("unable to unmarshal Qwen response: %v", err)
		return nil, fmt.Errorf("unable to unmarshal Qwen response: %v", err)
	}

	var outputEvents []StreamEvent
	responses := m.buildChatCompletionStreamingResponse(ctx, qwenResponse, incrementalStreaming)
	for _, response := range responses {
		responseBody, err := json.Marshal(response)
		if err != nil {
			log.Errorf("unable to marshal response: %v", err)
			return nil, fmt.Errorf("unable to marshal response: %v", err)
		}
		modifiedEvent := event
		modifiedEvent.Id = ""
		modifiedEvent.Event = ""
		modifiedEvent.Data = string(responseBody)
		outputEvents = append(outputEvents, modifiedEvent)
	}
	return outputEvents, nil
}

func (m *qwenProvider) TransformResponseBody(ctx wrapper.HttpContext, apiName ApiName, body []byte) ([]byte, error) {
	if m.config.qwenEnableCompatible {
		return body, nil
	}
	if apiName == ApiNameChatCompletion {
		return m.onChatCompletionResponseBody(ctx, body)
	}
	if apiName == ApiNameEmbeddings {
		return m.onEmbeddingsResponseBody(ctx, body)
	}
	if apiName == ApiNameImageGeneration {
		return m.onImageGenerationResponseBody(ctx, body)
	}
	if apiName == ApiNameAudioSpeech {
		return m.onAudioSpeechResponseBody(ctx, body)
	}
	if m.config.isSupportedAPI(apiName) {
		return body, nil
	}
	return nil, errUnsupportedApiName
}

func (m *qwenProvider) onChatCompletionResponseBody(ctx wrapper.HttpContext, body []byte) ([]byte, error) {
	if code := gjson.GetBytes(body, "code").String(); code != "" && code != "200" && !gjson.GetBytes(body, "output").Exists() {
		return body, nil
	}
	qwenResponse := &qwenTextGenResponse{}
	if err := json.Unmarshal(body, qwenResponse); err != nil {
		return nil, fmt.Errorf("unable to unmarshal Qwen response: %v", err)
	}
	response := m.buildChatCompletionResponse(ctx, qwenResponse)
	return json.Marshal(response)
}

func (m *qwenProvider) onEmbeddingsResponseBody(ctx wrapper.HttpContext, body []byte) ([]byte, error) {
	qwenResponse := &qwenTextEmbeddingResponse{}
	if err := json.Unmarshal(body, qwenResponse); err != nil {
		return nil, fmt.Errorf("unable to unmarshal Qwen response: %v", err)
	}
	response := m.buildEmbeddingsResponse(ctx, qwenResponse)
	return json.Marshal(response)
}

func (m *qwenProvider) onImageGenerationResponseBody(ctx wrapper.HttpContext, body []byte) ([]byte, error) {
	qwenResponse := &qwenImageGenerationResponse{}
	if err := json.Unmarshal(body, qwenResponse); err != nil {
		return nil, fmt.Errorf("unable to unmarshal Qwen image response: %v", err)
	}
	if qwenResponse.hasProviderError() {
		return body, nil
	}

	response := m.buildImageGenerationResponse(qwenResponse)
	return json.Marshal(response)
}

func (m *qwenProvider) onAudioSpeechResponseBody(ctx wrapper.HttpContext, body []byte) ([]byte, error) {
	qwenResponse := &qwenAudioSpeechResponse{}
	if err := json.Unmarshal(body, qwenResponse); err != nil {
		return nil, fmt.Errorf("unable to unmarshal Qwen audio speech response: %v", err)
	}
	if qwenResponse.hasProviderError() || qwenResponse.Output.Audio.URL == "" {
		return body, nil
	}

	response := m.buildAudioSpeechResponse(qwenResponse)
	return json.Marshal(response)
}

func (m *qwenProvider) buildQwenTextGenerationRequest(ctx wrapper.HttpContext, origRequest *chatCompletionRequest, streaming bool) ([]byte, error) {
	return m.buildQwenGenerationRequest(ctx, origRequest, nil, streaming, false)
}

func (m *qwenProvider) buildQwenGenerationRequest(ctx wrapper.HttpContext, origRequest *chatCompletionRequest, rawBody []byte, streaming, multimodal bool) ([]byte, error) {
	messages := make([]qwenMessage, 0, len(origRequest.Messages))
	for i := range origRequest.Messages {
		msg := chatMessage2QwenMessage(origRequest.Messages[i])
		if multimodal {
			if (msg.Role == roleSystem || msg.Role == roleUser) && msg.IsStringContent() {
				msg.Content = []qwenVlMessageContent{{Text: msg.StringContent()}}
			}
		} else if msg.Role == roleSystem {
			if contents, ok := msg.Content.([]qwenVlMessageContent); ok {
				plain := true
				var text strings.Builder
				for _, item := range contents {
					if item.Image != "" || item.CacheControl != nil {
						plain = false
						break
					}
					text.WriteString(item.Text)
				}
				if plain {
					msg.Content = text.String()
				}
			}
		}
		messages = append(messages, msg)
	}
	var topP float64
	if origRequest.TopP > 0 {
		topP = math.Max(qwenTopPMin, math.Min(origRequest.TopP, qwenTopPMax))
	}
	var enableThinking *bool
	var thinkingBudget int
	if rawEnable := gjson.GetBytes(rawBody, "enable_thinking"); rawEnable.Type == gjson.True || rawEnable.Type == gjson.False {
		val := rawEnable.Bool()
		enableThinking = &val
	}
	if rawBudget := gjson.GetBytes(rawBody, "thinking_budget"); rawBudget.Type == gjson.Number && rawBudget.Int() > 0 {
		thinkingBudget = int(rawBudget.Int())
	}
	if enableThinking == nil && origRequest.ClaudeThinking != nil {
		switch origRequest.ClaudeThinking.Type {
		case "enabled", "adaptive":
			val := true
			enableThinking = &val
			if thinkingBudget == 0 && origRequest.ClaudeThinking.BudgetTokens > 0 {
				thinkingBudget = origRequest.ClaudeThinking.BudgetTokens
			}
		case "disabled":
			val := false
			enableThinking = &val
		}
	}
	if enableThinking == nil && origRequest.Thinking != nil {
		switch origRequest.Thinking.Type {
		case "enabled", "adaptive":
			val := true
			enableThinking = &val
			if thinkingBudget == 0 && origRequest.Thinking.BudgetToken > 0 {
				thinkingBudget = origRequest.Thinking.BudgetToken
			}
		case "disabled":
			val := false
			enableThinking = &val
		}
	}
	if enableThinking == nil && origRequest.ReasoningEffort != "" {
		val := origRequest.ReasoningEffort != "none"
		enableThinking = &val
	}
	if isDashScopeForcedToolChoice(origRequest.ToolChoice) {
		if len(origRequest.Tools) == 0 {
			return nil, fmt.Errorf("forced tool_choice requires tools in DashScope mode")
		}
		if enableThinking != nil && *enableThinking {
			return nil, fmt.Errorf("forced tool_choice is unsupported when enable_thinking is enabled in DashScope mode")
		}
	}
	temperaturePtr, presencePenaltyPtr := resolveDashScopeOptionalFloatParams(origRequest, rawBody)
	request := &qwenTextGenRequest{
		Model: origRequest.Model,
		Input: qwenTextGenInput{
			Messages: messages,
		},
		Parameters: qwenTextGenParameters{
			ResultFormat:      qwenResultFormatMessage,
			MaxTokens:         origRequest.getMaxTokens(),
			PresencePenalty:   presencePenaltyPtr,
			N:                 origRequest.N,
			Seed:              origRequest.Seed,
			Stop:              origRequest.Stop,
			Temperature:       temperaturePtr,
			TopP:              topP,
			IncrementalOutput: streaming && (len(origRequest.Tools) == 0 || (enableThinking != nil && *enableThinking)),
			EnableSearch:      m.config.qwenEnableSearch,
			EnableThinking:    enableThinking,
			ThinkingBudget:    thinkingBudget,
			PreserveThinking:  shouldEnableQwenPreserveThinking(origRequest.Model, origRequest.Messages),
			ResponseFormat:    origRequest.ResponseFormat,
			Tools:             origRequest.Tools,
			ToolChoice:        origRequest.ToolChoice,
			ParallelToolCalls: origRequest.ParallelToolCalls,
		},
	}

	if streaming && ctx != nil {
		ctx.SetContext(ctxKeyIncrementalStreaming, request.Parameters.IncrementalOutput)
	}

	if len(m.config.qwenFileIds) != 0 && origRequest.Model == qwenLongModelName {
		builder := strings.Builder{}
		for _, fileId := range m.config.qwenFileIds {
			if builder.Len() != 0 {
				builder.WriteRune(',')
			}
			builder.WriteString("fileid://")
			builder.WriteString(fileId)
		}

		body, err := json.Marshal(request)
		if err != nil {
			return nil, fmt.Errorf("unable to marshal request: %v", err)
		}

		return m.insertHttpContextMessage(body, builder.String(), true)
	}
	return json.Marshal(request)
}

func (m *qwenProvider) buildChatCompletionResponse(ctx wrapper.HttpContext, qwenResponse *qwenTextGenResponse) *chatCompletionResponse {
	qwenChoices := qwenResponse.Output.Choices
	if len(qwenChoices) == 0 && (qwenResponse.Output.Text != "" || qwenResponse.Output.FinishReason != "") {
		qwenChoices = []qwenTextGenChoice{{
			FinishReason: qwenResponse.Output.FinishReason,
			Message:      qwenMessage{Role: roleAssistant, Content: qwenResponse.Output.Text},
		}}
	}
	choices := make([]chatCompletionChoice, 0, len(qwenChoices))
	for i, qwenChoice := range qwenChoices {
		message := qwenMessageToChatMessage(qwenChoice.Message, m.config.reasoningContentMode)
		choices = append(choices, chatCompletionChoice{
			Index:        i,
			Message:      &message,
			FinishReason: util.Ptr(qwenChoice.FinishReason),
		})
	}
	return &chatCompletionResponse{
		Id:                qwenResponse.RequestId,
		Created:           time.Now().UnixMilli() / 1000,
		Model:             ctx.GetStringContext(ctxKeyFinalRequestModel, ""),
		SystemFingerprint: "",
		Object:            objectChatCompletion,
		Choices:           choices,
		Usage: &usage{
			PromptTokens:     qwenResponse.Usage.InputTokens,
			CompletionTokens: qwenResponse.Usage.OutputTokens,
			TotalTokens:      qwenResponse.Usage.TotalTokens,
		},
	}
}

func (m *qwenProvider) buildChatCompletionStreamingResponse(ctx wrapper.HttpContext, qwenResponse *qwenTextGenResponse, incrementalStreaming bool) []*chatCompletionResponse {
	qwenChoices := qwenResponse.Output.Choices
	if len(qwenChoices) == 0 && (qwenResponse.Output.Text != "" || qwenResponse.Output.FinishReason != "") {
		qwenChoices = []qwenTextGenChoice{{
			FinishReason: qwenResponse.Output.FinishReason,
			Message:      qwenMessage{Role: roleAssistant, Content: qwenResponse.Output.Text},
		}}
	}
	if len(qwenChoices) == 0 {
		log.Warnf("qwen response has no choices, request_id: %s", qwenResponse.RequestId)
		return nil
	}

	baseMessage := chatCompletionResponse{
		Id:                qwenResponse.RequestId,
		Created:           time.Now().UnixMilli() / 1000,
		Model:             ctx.GetStringContext(ctxKeyFinalRequestModel, ""),
		Choices:           make([]chatCompletionChoice, 0),
		SystemFingerprint: "",
		Object:            objectChatCompletionChunk,
	}

	responses := make([]*chatCompletionResponse, 0)

	qwenChoice := qwenChoices[0]
	// Yes, Qwen uses a string "null" as null.
	finished := qwenChoice.FinishReason != "" && qwenChoice.FinishReason != "null"
	message := qwenChoice.Message
	for i := range message.ToolCalls {
		if message.ToolCalls[i].Index == 0 && i > 0 {
			message.ToolCalls[i].Index = i
		}
	}

	reasoningContentMode := m.config.reasoningContentMode

	log.Debugf("incrementalStreaming: %v", incrementalStreaming)
	currentText := message.StringContent()
	deltaContentMessage := &chatMessage{Role: message.Role, Content: currentText, ReasoningContent: message.ReasoningContent}
	deltaToolCallsMessage := &chatMessage{Role: message.Role, ToolCalls: append([]toolCall{}, message.ToolCalls...)}
	if incrementalStreaming {
		deltaContentMessage.handleStreamingReasoningContent(ctx, reasoningContentMode)
		if len(deltaToolCallsMessage.ToolCalls) > 0 && ctx != nil {
			pushedCalls, _ := ctx.GetContext("qwenPushedIncrementalToolCalls").(map[int]toolCall)
			if pushedCalls == nil {
				pushedCalls = make(map[int]toolCall)
			}
			for i, tc := range deltaToolCallsMessage.ToolCalls {
				if prev, exists := pushedCalls[tc.Index]; exists {
					if prev.Id != "" && tc.Id == prev.Id {
						tc.Id = ""
						if prev.Function.Name != "" && tc.Function.Name == prev.Function.Name {
							tc.Function.Name = ""
						}
					}
					if tc.Id != "" {
						prev.Id = tc.Id
					}
					if tc.Function.Name != "" {
						prev.Function.Name = tc.Function.Name
					}
					pushedCalls[tc.Index] = prev
				} else if tc.Id != "" || tc.Function.Name != "" {
					pushedCalls[tc.Index] = toolCall{Index: tc.Index, Id: tc.Id, Function: functionCall{Name: tc.Function.Name}}
				}
				deltaToolCallsMessage.ToolCalls[i] = tc
			}
			ctx.SetContext("qwenPushedIncrementalToolCalls", pushedCalls)
		}
	} else {
		for _, tc := range message.ToolCalls {
			if tc.Function.Arguments == "" && !finished {
				// We don't push any tool call until its arguments are available.
				return nil
			}
		}
		if pushedMessage, ok := ctx.GetContext(ctxKeyPushedMessage).(qwenMessage); ok {
			pushedText := pushedMessage.StringContent()
			if currentText == "" {
				message.Content = pushedMessage.Content
			} else {
				deltaContentMessage.Content = util.StripPrefix(currentText, pushedText)
			}
			if message.ReasoningContent == "" {
				message.ReasoningContent = pushedMessage.ReasoningContent
			} else {
				deltaContentMessage.ReasoningContent = util.StripPrefix(deltaContentMessage.ReasoningContent, pushedMessage.ReasoningContent)
			}
			deltaContentMessage.handleStreamingReasoningContent(ctx, reasoningContentMode)

			if len(deltaToolCallsMessage.ToolCalls) > 0 && pushedMessage.ToolCalls != nil {
				for i, tc := range deltaToolCallsMessage.ToolCalls {
					if i >= len(pushedMessage.ToolCalls) {
						break
					}
					pushedToolCall := pushedMessage.ToolCalls[i]
					pushedFunction := pushedToolCall.Function
					tc.Id = util.StripPrefix(tc.Id, pushedToolCall.Id)
					tc.Function.Id = util.StripPrefix(tc.Function.Id, pushedFunction.Id)
					tc.Function.Name = util.StripPrefix(tc.Function.Name, pushedFunction.Name)
					tc.Function.Arguments = util.StripPrefix(tc.Function.Arguments, pushedFunction.Arguments)
					deltaToolCallsMessage.ToolCalls[i] = tc
				}
			}
		}
		ctx.SetContext(ctxKeyPushedMessage, message)
	}
	if len(deltaToolCallsMessage.ToolCalls) > 0 {
		filtered := deltaToolCallsMessage.ToolCalls[:0]
		for _, tc := range deltaToolCallsMessage.ToolCalls {
			if !tc.Function.IsEmpty() {
				filtered = append(filtered, tc)
			}
		}
		deltaToolCallsMessage.ToolCalls = filtered
	}

	if !deltaContentMessage.IsEmpty() {
		response := baseMessage
		response.Choices = append(response.Choices, chatCompletionChoice{Delta: deltaContentMessage})
		responses = append(responses, &response)
	}
	if !deltaToolCallsMessage.IsEmpty() {
		response := baseMessage
		response.Choices = append(response.Choices, chatCompletionChoice{Delta: deltaToolCallsMessage})
		responses = append(responses, &response)
	}

	if finished {
		finishResponse := baseMessage
		finishResponse.Choices = append(finishResponse.Choices, chatCompletionChoice{Delta: &chatMessage{}, FinishReason: util.Ptr(qwenChoice.FinishReason)})

		usageResponse := baseMessage
		usageResponse.Choices = []chatCompletionChoice{{Delta: &chatMessage{}}}
		usageResponse.Usage = &usage{
			PromptTokens:     qwenResponse.Usage.InputTokens,
			CompletionTokens: qwenResponse.Usage.OutputTokens,
			TotalTokens:      qwenResponse.Usage.TotalTokens,
		}

		responses = append(responses, &finishResponse, &usageResponse)
	}

	return responses
}

func (m *qwenProvider) insertHttpContextMessage(body []byte, content string, onlyOneSystemBeforeFile bool) ([]byte, error) {
	request := &qwenTextGenRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		return nil, fmt.Errorf("unable to unmarshal request: %v", err)
	}

	fileMessage := qwenMessage{
		Role:    roleSystem,
		Content: content,
	}
	var firstNonSystemMessageIndex int
	messages := request.Input.Messages
	if messages != nil {
		for i, message := range request.Input.Messages {
			if message.Role != roleSystem {
				firstNonSystemMessageIndex = i
				break
			}
		}
	}
	if firstNonSystemMessageIndex == 0 {
		request.Input.Messages = append([]qwenMessage{fileMessage}, request.Input.Messages...)
	} else if !onlyOneSystemBeforeFile {
		request.Input.Messages = append(request.Input.Messages[:firstNonSystemMessageIndex], append([]qwenMessage{fileMessage}, request.Input.Messages[firstNonSystemMessageIndex:]...)...)
	} else {
		builder := strings.Builder{}
		for _, message := range request.Input.Messages[:firstNonSystemMessageIndex] {
			if builder.Len() != 0 {
				builder.WriteString("\n")
			}
			builder.WriteString(message.StringContent())
		}
		request.Input.Messages = append([]qwenMessage{{Role: roleSystem, Content: builder.String()}, fileMessage}, request.Input.Messages[firstNonSystemMessageIndex:]...)
		firstNonSystemMessageIndex = 1
	}

	if firstNonSystemMessageIndex == 0 {
		// The context message cannot come first. We need to add another dummy system message before it.
		request.Input.Messages = append([]qwenMessage{{Role: roleSystem, Content: qwenDummySystemMessageContent}}, request.Input.Messages...)
	}

	return json.Marshal(request)
}

func (m *qwenProvider) buildQwenTextEmbeddingRequest(request *embeddingsRequest) (*qwenTextEmbeddingRequest, error) {
	var texts []string
	if str, isString := request.Input.(string); isString {
		texts = []string{str}
	} else if strs, isArray := request.Input.([]interface{}); isArray {
		texts = make([]string, 0, len(strs))
		for _, item := range strs {
			if str, isString := item.(string); isString {
				texts = append(texts, str)
			} else {
				return nil, errors.New("unsupported input type in array: " + reflect.TypeOf(item).String())
			}
		}
	} else {
		return nil, errors.New("unsupported input type: " + reflect.TypeOf(request.Input).String())
	}
	return &qwenTextEmbeddingRequest{
		Model: request.Model,
		Input: qwenTextEmbeddingInput{
			Texts: texts,
		},
	}, nil
}

func (m *qwenProvider) buildEmbeddingsResponse(ctx wrapper.HttpContext, qwenResponse *qwenTextEmbeddingResponse) *embeddingsResponse {
	data := make([]embedding, 0, len(qwenResponse.Output.Embeddings))
	for _, qwenEmbedding := range qwenResponse.Output.Embeddings {
		data = append(data, embedding{
			Object:    "embedding",
			Index:     qwenEmbedding.TextIndex,
			Embedding: qwenEmbedding.Embedding,
		})
	}
	return &embeddingsResponse{
		Object: "list",
		Data:   data,
		Model:  ctx.GetContext(ctxKeyFinalRequestModel).(string),
		Usage: usage{
			PromptTokens: qwenResponse.Usage.TotalTokens,
			TotalTokens:  qwenResponse.Usage.TotalTokens,
		},
	}
}

func (m *qwenProvider) buildQwenImageGenerationRequest(request *imageGenerationRequest, body []byte) *qwenImageGenerationRequest {
	parameters := qwenImageGenerationParameters{
		N:    request.N,
		Size: qwenNormalizeImageSize(request.Size),
	}
	if seed := gjson.GetBytes(body, "seed"); seed.Exists() {
		seedValue := seed.Int()
		parameters.Seed = &seedValue
	}

	return &qwenImageGenerationRequest{
		Model: request.Model,
		Input: qwenImageGenerationInput{
			Messages: []qwenMessage{
				{
					Role: roleUser,
					Content: []qwenVlMessageContent{
						{Text: request.Prompt},
					},
				},
			},
		},
		Parameters: parameters,
	}
}

func (m *qwenProvider) buildQwenAudioSpeechRequest(request *qwenOpenAIAudioSpeechRequest) *qwenAudioSpeechRequest {
	return &qwenAudioSpeechRequest{
		Model: request.Model,
		Input: qwenAudioSpeechInput{
			Text:                 request.Input,
			Voice:                request.Voice,
			LanguageType:         request.LanguageType,
			Instructions:         request.Instructions,
			OptimizeInstructions: request.OptimizeInstructions,
		},
	}
}

func (m *qwenProvider) buildImageGenerationResponse(qwenResponse *qwenImageGenerationResponse) *imageGenerationResponse {
	data := make([]imageGenerationData, 0)
	for _, choice := range qwenResponse.Output.Choices {
		for _, content := range choice.Message.Content {
			if content.Image == "" {
				continue
			}
			data = append(data, imageGenerationData{
				URL: content.Image,
			})
		}
	}

	var usage *imageGenerationUsage
	if qwenResponse.Usage != nil {
		usage = &imageGenerationUsage{
			TotalTokens:  qwenResponse.Usage.TotalTokens,
			InputTokens:  qwenResponse.Usage.InputTokens,
			OutputTokens: qwenResponse.Usage.OutputTokens,
		}
	}

	return &imageGenerationResponse{
		Created: time.Now().UnixMilli() / 1000,
		Data:    data,
		Usage:   usage,
	}
}

func (m *qwenProvider) buildAudioSpeechResponse(qwenResponse *qwenAudioSpeechResponse) *qwenAudioSpeechGatewayResponse {
	return &qwenAudioSpeechGatewayResponse{
		Created: time.Now().UnixMilli() / 1000,
		Data: qwenAudioSpeechGatewayData{
			URL:       qwenResponse.Output.Audio.URL,
			ID:        qwenResponse.Output.Audio.ID,
			ExpiresAt: qwenResponse.Output.Audio.ExpiresAt,
		},
		Usage: qwenResponse.Usage,
	}
}

func qwenNormalizeImageSize(size string) string {
	return strings.NewReplacer("x", "*", "X", "*").Replace(size)
}

type qwenTextGenRequest struct {
	Model      string                `json:"model"`
	Input      qwenTextGenInput      `json:"input"`
	Parameters qwenTextGenParameters `json:"parameters,omitempty"`
}

type qwenTextGenInput struct {
	Messages []qwenMessage `json:"messages"`
}

type qwenTextGenParameters struct {
	ResultFormat      string                 `json:"result_format,omitempty"`
	MaxTokens         int                    `json:"max_tokens,omitempty"`
	RepetitionPenalty float64                `json:"repetition_penalty,omitempty"`
	PresencePenalty   *float64               `json:"presence_penalty,omitempty"`
	N                 int                    `json:"n,omitempty"`
	Seed              int                    `json:"seed,omitempty"`
	Stop              []string               `json:"stop,omitempty"`
	Temperature       *float64               `json:"temperature,omitempty"`
	TopP              float64                `json:"top_p,omitempty"`
	IncrementalOutput bool                   `json:"incremental_output,omitempty"`
	EnableSearch      bool                   `json:"enable_search,omitempty"`
	EnableThinking    *bool                  `json:"enable_thinking,omitempty"`
	ThinkingBudget    int                    `json:"thinking_budget,omitempty"`
	PreserveThinking  bool                   `json:"preserve_thinking,omitempty"`
	ResponseFormat    map[string]interface{} `json:"response_format,omitempty"`
	Tools             []tool                 `json:"tools,omitempty"`
	ToolChoice        interface{}            `json:"tool_choice,omitempty"`
	ParallelToolCalls *bool                  `json:"parallel_tool_calls,omitempty"`
}

type qwenTextGenResponse struct {
	RequestId string            `json:"request_id"`
	Output    qwenTextGenOutput `json:"output"`
	Usage     qwenUsage         `json:"usage"`
}

type qwenTextGenOutput struct {
	Text         string              `json:"text,omitempty"`
	FinishReason string              `json:"finish_reason"`
	Choices      []qwenTextGenChoice `json:"choices"`
}

type qwenTextGenChoice struct {
	FinishReason string      `json:"finish_reason"`
	Message      qwenMessage `json:"message"`
}

type qwenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type qwenImageGenerationRequest struct {
	Model      string                        `json:"model"`
	Input      qwenImageGenerationInput      `json:"input"`
	Parameters qwenImageGenerationParameters `json:"parameters,omitempty"`
}

type qwenImageGenerationInput struct {
	Messages []qwenMessage `json:"messages"`
}

type qwenImageGenerationParameters struct {
	N    int    `json:"n,omitempty"`
	Seed *int64 `json:"seed,omitempty"`
	Size string `json:"size,omitempty"`
}

type qwenImageGenerationResponse struct {
	RequestID string                    `json:"request_id"`
	Code      string                    `json:"code,omitempty"`
	Message   string                    `json:"message,omitempty"`
	Output    qwenImageGenerationOutput `json:"output"`
	Usage     *qwenUsage                `json:"usage,omitempty"`
}

func (r *qwenImageGenerationResponse) hasProviderError() bool {
	return r.Code != ""
}

type qwenImageGenerationOutput struct {
	Choices []qwenImageGenerationChoice `json:"choices"`
}

type qwenImageGenerationChoice struct {
	FinishReason string                     `json:"finish_reason"`
	Message      qwenImageGenerationMessage `json:"message"`
}

type qwenImageGenerationMessage struct {
	Role    string                 `json:"role"`
	Content []qwenVlMessageContent `json:"content"`
}

type qwenOpenAIAudioSpeechRequest struct {
	Model                string `json:"model"`
	Input                string `json:"input"`
	Voice                string `json:"voice"`
	LanguageType         string `json:"language_type,omitempty"`
	Instructions         string `json:"instructions,omitempty"`
	OptimizeInstructions *bool  `json:"optimize_instructions,omitempty"`
}

type qwenAudioSpeechRequest struct {
	Model string               `json:"model"`
	Input qwenAudioSpeechInput `json:"input"`
}

type qwenAudioSpeechInput struct {
	Text                 string `json:"text"`
	Voice                string `json:"voice"`
	LanguageType         string `json:"language_type,omitempty"`
	Instructions         string `json:"instructions,omitempty"`
	OptimizeInstructions *bool  `json:"optimize_instructions,omitempty"`
}

type qwenAudioSpeechResponse struct {
	StatusCode int                   `json:"status_code,omitempty"`
	RequestID  string                `json:"request_id"`
	Code       string                `json:"code,omitempty"`
	Message    string                `json:"message,omitempty"`
	Output     qwenAudioSpeechOutput `json:"output"`
	Usage      *qwenAudioSpeechUsage `json:"usage,omitempty"`
}

func (r *qwenAudioSpeechResponse) hasProviderError() bool {
	return r.Code != "" || r.StatusCode >= http.StatusBadRequest
}

type qwenAudioSpeechOutput struct {
	Audio qwenAudioSpeechAudio `json:"audio"`
}

type qwenAudioSpeechAudio struct {
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
	ID        string `json:"id,omitempty"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
}

type qwenAudioSpeechUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	Characters   int `json:"characters"`
	TotalTokens  int `json:"total_tokens"`
}

type qwenAudioSpeechGatewayResponse struct {
	Created int64                      `json:"created"`
	Data    qwenAudioSpeechGatewayData `json:"data"`
	Usage   *qwenAudioSpeechUsage      `json:"usage,omitempty"`
}

type qwenAudioSpeechGatewayData struct {
	URL       string `json:"url"`
	ID        string `json:"id,omitempty"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
}

type qwenMessage struct {
	Name             string     `json:"name,omitempty"`
	Role             string     `json:"role"`
	Content          any        `json:"content"`
	Partial          *bool      `json:"partial,omitempty"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []toolCall `json:"tool_calls,omitempty"`
	ToolCallId       string     `json:"tool_call_id,omitempty"`
}

type qwenVlMessageContent struct {
	CacheControl map[string]interface{} `json:"cache_control,omitempty"`
	Image        string                 `json:"image,omitempty"`
	Text         string                 `json:"text,omitempty"`
}

type qwenTextEmbeddingRequest struct {
	Model      string                      `json:"model"`
	Input      qwenTextEmbeddingInput      `json:"input"`
	Parameters qwenTextEmbeddingParameters `json:"parameters,omitempty"`
}

type qwenTextEmbeddingInput struct {
	Texts []string `json:"texts"`
}

type qwenTextEmbeddingParameters struct {
	TextType string `json:"text_type,omitempty"`
}

type qwenTextEmbeddingResponse struct {
	RequestId string                  `json:"request_id"`
	Output    qwenTextEmbeddingOutput `json:"output"`
	Usage     qwenUsage               `json:"usage"`
}

type qwenTextEmbeddingOutput struct {
	RequestId  string               `json:"request_id"`
	Embeddings []qwenTextEmbeddings `json:"embeddings"`
}

type qwenTextEmbeddings struct {
	TextIndex int       `json:"text_index"`
	Embedding []float64 `json:"embedding"`
}

func qwenMessageToChatMessage(qwenMessage qwenMessage, reasoningContentMode string) chatMessage {
	var content any
	if qwenMessage.Content != nil {
		text := qwenMessage.StringContent()
		if text != "" || len(qwenMessage.ToolCalls) == 0 {
			content = text
		}
	}
	toolCalls := qwenMessage.ToolCalls
	if len(toolCalls) > 1 {
		toolCalls = append([]toolCall(nil), toolCalls...)
		for i := range toolCalls {
			if toolCalls[i].Index == 0 && i > 0 {
				toolCalls[i].Index = i
			}
		}
	}
	msg := chatMessage{
		Name:             qwenMessage.Name,
		Role:             qwenMessage.Role,
		Content:          content,
		ReasoningContent: qwenMessage.ReasoningContent,
		ToolCalls:        toolCalls,
		ToolCallId:       qwenMessage.ToolCallId,
	}
	msg.handleNonStreamingReasoningContent(reasoningContentMode)
	return msg
}

func (m *qwenMessage) IsStringContent() bool {
	_, ok := m.Content.(string)
	return ok
}

func (m *qwenMessage) StringContent() string {
	if content, ok := m.Content.(string); ok {
		return content
	}
	if vlList, ok := m.Content.([]qwenVlMessageContent); ok {
		var contentStr strings.Builder
		for _, item := range vlList {
			contentStr.WriteString(item.Text)
		}
		return contentStr.String()
	}
	if contentList, ok := m.Content.([]any); ok {
		var contentStr strings.Builder
		for _, contentItem := range contentList {
			contentMap, ok := contentItem.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := contentMap["text"].(string); ok {
				contentStr.WriteString(text)
			}
		}
		return contentStr.String()
	}
	return ""
}

func chatMessagesHaveReasoningContent(messages []chatMessage) bool {
	for _, message := range messages {
		if message.ReasoningContent != "" {
			return true
		}
	}
	return false
}

func shouldEnableQwenPreserveThinking(model string, messages []chatMessage) bool {
	return chatMessagesHaveReasoningContent(messages) && qwenSupportsPreserveThinking(model)
}

func qwenSupportsPreserveThinking(model string) bool {
	switch {
	case model == "qwen3.6-max-preview":
		return true
	case strings.HasPrefix(model, "qwen3.6-plus"),
		strings.HasPrefix(model, "qwen3.6-flash"),
		strings.HasPrefix(model, "qwen3.7-"),
		strings.HasPrefix(model, "qwen3.8-"):
		return true
	case model == "kimi-k2.6", strings.HasPrefix(model, "kimi-k2.7"):
		return true
	default:
		return false
	}
}

func requestBodyHasMessageReasoningContent(body []byte) bool {
	for _, message := range gjson.GetBytes(body, "messages").Array() {
		if message.Get("reasoning_content").String() != "" {
			return true
		}
	}
	return false
}

func sanitizeQwenCompatibleChatRequest(body []byte) ([]byte, error) {
	mappedModel := gjson.GetBytes(body, "model").String()
	return sanitizeQwenCompatibleChatRequestWithPolicy(body, qwenCompatibleChatPolicy(mappedModel))
}

func sanitizeQwenCompatibleChatRequestWithPolicy(body []byte, policy qwenChatCompatPolicy) ([]byte, error) {
	modified := body
	var err error
	topFieldsToStrip := []string{
		"reasoning_summary",
		"safety_identifier",
		"context_management",
	}
	if !policy.SupportsStore {
		topFieldsToStrip = append(topFieldsToStrip, "store")
	}
	if !policy.SupportsPromptCacheKey {
		topFieldsToStrip = append(topFieldsToStrip, "prompt_cache_key", "prompt_cache_retention", "prompt_cache_options")
	}
	for _, field := range topFieldsToStrip {
		if gjson.GetBytes(modified, field).Exists() {
			if modified, err = sjson.DeleteBytes(modified, field); err != nil {
				return nil, err
			}
		}
	}

	if policy.StripOuterCacheControl {
		if tools := gjson.GetBytes(modified, "tools"); tools.IsArray() {
			for idx, toolItem := range tools.Array() {
				if toolItem.Get("cache_control").Exists() {
					if modified, err = sjson.DeleteBytes(modified, fmt.Sprintf("tools.%d.cache_control", idx)); err != nil {
						return nil, err
					}
				}
			}
		}
	}

	if messages := gjson.GetBytes(modified, "messages"); messages.IsArray() {
		msgFieldsToStrip := []string{"thinking_blocks", "reasoning_details", "extra_content"}
		if policy.StripOuterCacheControl {
			msgFieldsToStrip = append(msgFieldsToStrip, "cache_control")
		}
		if !policy.SupportsPromptCacheKey {
			msgFieldsToStrip = append(msgFieldsToStrip, "prompt_cache_breakpoint")
		}
		for idx, msg := range messages.Array() {
			for _, field := range msgFieldsToStrip {
				if msg.Get(field).Exists() {
					if modified, err = sjson.DeleteBytes(modified, fmt.Sprintf("messages.%d.%s", idx, field)); err != nil {
						return nil, err
					}
				}
			}
			role := msg.Get("role").String()
			if role == roleDeveloper && !policy.SupportsDeveloperRole {
				if modified, err = sjson.SetBytes(modified, fmt.Sprintf("messages.%d.role", idx), roleSystem); err != nil {
					return nil, err
				}
			} else if role == roleTool && policy.StripToolMessageName && msg.Get("name").Exists() {
				if modified, err = sjson.DeleteBytes(modified, fmt.Sprintf("messages.%d.name", idx)); err != nil {
					return nil, err
				}
			} else if role == roleAssistant && policy.NormalizeEmptyAssistantContent && msg.Get("content").Type == gjson.Null {
				if modified, err = sjson.SetBytes(modified, fmt.Sprintf("messages.%d.content", idx), ""); err != nil {
					return nil, err
				}
			}
			// H3: policy.SupportsContentCacheControl == true preserves messages[].content[].cache_control
			// so Bailian explicit prompt caching and s.cacheMode == explicit work identically on same-wire and cross-wire paths.
			if content := msg.Get("content"); content.IsArray() {
				for partIdx, part := range content.Array() {
					if !policy.SupportsContentCacheControl && part.Get("cache_control").Exists() {
						if modified, err = sjson.DeleteBytes(modified, fmt.Sprintf("messages.%d.content.%d.cache_control", idx, partIdx)); err != nil {
							return nil, err
						}
					}
					if !policy.SupportsPromptCacheKey && part.Get("prompt_cache_breakpoint").Exists() {
						if modified, err = sjson.DeleteBytes(modified, fmt.Sprintf("messages.%d.content.%d.prompt_cache_breakpoint", idx, partIdx)); err != nil {
							return nil, err
						}
					}
				}
			}
			if policy.StripOuterCacheControl {
				if toolCalls := msg.Get("tool_calls"); toolCalls.IsArray() {
					for tcIdx, tc := range toolCalls.Array() {
						if tc.Get("extra_content").Exists() {
							if modified, err = sjson.DeleteBytes(modified, fmt.Sprintf("messages.%d.tool_calls.%d.extra_content", idx, tcIdx)); err != nil {
								return nil, err
							}
						}
					}
				}
			}
		}
	}
	return modified, nil
}

func resolveDashScopeOptionalFloatParams(origRequest *chatCompletionRequest, rawBody []byte) (*float64, *float64) {
	var temperature, presencePenalty *float64
	if rawBody != nil {
		if t := gjson.GetBytes(rawBody, "temperature"); t.Type == gjson.Number {
			v := t.Float()
			temperature = &v
		}
		if p := gjson.GetBytes(rawBody, "presence_penalty"); p.Type == gjson.Number {
			v := p.Float()
			presencePenalty = &v
		}
		return temperature, presencePenalty
	}
	if origRequest.Temperature != 0 {
		v := origRequest.Temperature
		temperature = &v
	}
	if origRequest.PresencePenalty != 0 {
		v := origRequest.PresencePenalty
		presencePenalty = &v
	}
	return temperature, presencePenalty
}

func isDashScopeForcedToolChoice(toolChoice interface{}) bool {
	if toolChoice == nil {
		return false
	}
	switch v := toolChoice.(type) {
	case string:
		return v == "required"
	case map[string]interface{}:
		if t, _ := v["type"].(string); t == "function" {
			return true
		}
		_, hasFunc := v["function"]
		return hasFunc
	default:
		return false
	}
}

func chatMessage2QwenMessage(chatMessage chatMessage) qwenMessage {
	if chatMessage.Role == roleDeveloper {
		chatMessage.Role = roleSystem
	}
	if chatMessage.Role == roleTool {
		var content string
		if s, ok := chatMessage.Content.(string); ok {
			content = s
		} else {
			var builder strings.Builder
			for _, part := range chatMessage.ParseContent() {
				if part.Type == contentTypeText {
					builder.WriteString(part.Text)
				}
			}
			content = builder.String()
		}
		return qwenMessage{
			Role:       chatMessage.Role,
			Content:    content,
			ToolCallId: chatMessage.ToolCallId,
		}
	}
	if chatMessage.Content == nil {
		return qwenMessage{
			Name:             chatMessage.Name,
			Role:             chatMessage.Role,
			Content:          "",
			Partial:          chatMessage.Partial,
			ReasoningContent: chatMessage.ReasoningContent,
			ToolCalls:        chatMessage.ToolCalls,
			ToolCallId:       chatMessage.ToolCallId,
		}
	}
	if chatMessage.IsStringContent() {
		return qwenMessage{
			Name:             chatMessage.Name,
			Role:             chatMessage.Role,
			Content:          chatMessage.StringContent(),
			Partial:          chatMessage.Partial,
			ReasoningContent: chatMessage.ReasoningContent,
			ToolCalls:        chatMessage.ToolCalls,
			ToolCallId:       chatMessage.ToolCallId,
		}
	}
	var contents []qwenVlMessageContent
	openaiContent := chatMessage.ParseContent()
	for _, part := range openaiContent {
		var content qwenVlMessageContent
		content.CacheControl = part.CacheControl
		if part.Type == contentTypeText {
			content.Text = part.Text
		} else if part.Type == contentTypeImageUrl {
			content.Image = part.ImageUrl.Url
		}
		contents = append(contents, content)
	}
	return qwenMessage{
		Name:             chatMessage.Name,
		Role:             chatMessage.Role,
		Content:          contents,
		Partial:          chatMessage.Partial,
		ReasoningContent: chatMessage.ReasoningContent,
		ToolCalls:        chatMessage.ToolCalls,
		ToolCallId:       chatMessage.ToolCallId,
	}
}

func (m *qwenProvider) GetApiName(path string) ApiName {
	switch {
	case strings.Contains(path, qwenChatCompletionPath),
		strings.Contains(path, qwenMultimodalGenerationPath),
		strings.Contains(path, qwenBailianPath),
		strings.Contains(path, qwenCompatibleChatCompletionPath):
		return ApiNameChatCompletion
	case strings.Contains(path, qwenTextEmbeddingPath),
		strings.Contains(path, qwenCompatibleTextEmbeddingPath):
		return ApiNameEmbeddings
	case strings.Contains(path, qwenCompatibleResponsesPath):
		return ApiNameResponses
	case strings.Contains(path, qwenAsyncAIGCPath):
		return ApiNameQwenAsyncAIGC
	case strings.Contains(path, qwenAsyncTaskPath):
		return ApiNameQwenAsyncTask
	case strings.Contains(path, qwenTextRerankPath),
		strings.Contains(path, qwenCompatibleTextRerankPath):
		return ApiNameQwenV1Rerank
	case strings.Contains(path, qwenCompatibleConversationsPath):
		return ApiNameQwenV1Conversations
	default:
		return ""
	}
}
