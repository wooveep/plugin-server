package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/config"
	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/provider"
	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/test"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	wasmhost "github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

var responsesNativeOpenAIConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"provider": map[string]interface{}{
			"type":      "openai",
			"apiTokens": []string{"sk-openai-responses-native"},
		},
	})
	return data
}()

var responsesFallbackOpenRouterConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"provider": map[string]interface{}{
			"type":      "openrouter",
			"apiTokens": []string{"sk-openrouter-responses-fallback"},
		},
	})
	return data
}()

var responsesFallbackQwenConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"provider": map[string]interface{}{
			"type":                 "qwen",
			"apiTokens":            []string{"sk-qwen-responses-fallback"},
			"qwenEnableCompatible": true,
			"capabilities": map[string]string{
				"openai/v1/responses": "",
			},
		},
	})
	return data
}()

var responsesUnsupportedKlingConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"provider": map[string]interface{}{
			"type":           "kling",
			"klingAccessKey": "kling-ak-test",
			"klingSecretKey": "kling-sk-test",
		},
	})
	return data
}()

func TestResponsesResponseConversionMarker(t *testing.T) {
	ctx := test.NewMockHttpContext()
	require.False(t, needsResponsesResponseConversion(ctx))

	ctx.SetContext("needResponsesResponseConversion", true)
	require.True(t, needsResponsesResponseConversion(ctx))

	ctx.SetContext("needClaudeResponseConversion", true)
	require.True(t, needsResponsesResponseConversion(ctx))
	require.True(t, NeedsClaudeResponseConversionForTest(ctx))
}

func TestResponsesNativeRoutingKeepsResponsesPath(t *testing.T) {
	wasmhost.RunTest(t, func(t *testing.T) {
		host, status := wasmhost.NewTestHost(responsesNativeOpenAIConfig)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "example.com"},
			{":path", "/v1/responses"},
			{":method", "POST"},
			{"Content-Type", "application/json"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		pathValue, hasPath := wasmhost.GetHeaderValue(host.GetRequestHeaders(), ":path")
		require.True(t, hasPath)
		require.Equal(t, "/v1/responses", pathValue)
	})
}

func TestResponsesFallbackRoutingUsesChatCompletions(t *testing.T) {
	wasmhost.RunTest(t, func(t *testing.T) {
		host, status := wasmhost.NewTestHost(responsesFallbackOpenRouterConfig)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "example.com"},
			{":path", "/v1/responses"},
			{":method", "POST"},
			{"Content-Type", "application/json"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		pathValue, hasPath := wasmhost.GetHeaderValue(host.GetRequestHeaders(), ":path")
		require.True(t, hasPath)
		require.Equal(t, "/api/v1/chat/completions", pathValue)
	})
}

func TestResponsesFallbackRequestBodyConvertsToChatCompletions(t *testing.T) {
	wasmhost.RunTest(t, func(t *testing.T) {
		host, status := wasmhost.NewTestHost(responsesFallbackOpenRouterConfig)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "example.com"},
			{":path", "/v1/responses"},
			{":method", "POST"},
			{"Content-Type", "application/json"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		action = host.CallOnHttpRequestBody([]byte(`{"model":"openai/gpt-4o-mini","input":"hello","max_output_tokens":32}`))
		require.Equal(t, types.ActionContinue, action)

		body := host.GetRequestBody()
		require.Equal(t, "user", gjson.GetBytes(body, "messages.0.role").String())
		require.Equal(t, "hello", gjson.GetBytes(body, "messages.0.content").String())
		require.Equal(t, int64(32), gjson.GetBytes(body, "max_completion_tokens").Int())
		require.False(t, gjson.GetBytes(body, "input").Exists(), string(body))
	})
}

func TestResponsesFallbackResponseBodyConvertsToResponses(t *testing.T) {
	wasmhost.RunTest(t, func(t *testing.T) {
		host, status := wasmhost.NewTestHost(responsesFallbackOpenRouterConfig)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "example.com"},
			{":path", "/v1/responses"},
			{":method", "POST"},
			{"Content-Type", "application/json"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		action = host.CallOnHttpRequestBody([]byte(`{"model":"openai/gpt-4o-mini","input":"hello"}`))
		require.Equal(t, types.ActionContinue, action)

		require.NoError(t, host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
		action = host.CallOnHttpResponseHeaders([][2]string{
			{":status", "200"},
			{"Content-Type", "application/json"},
		})
		require.Equal(t, types.ActionContinue, action)

		action = host.CallOnHttpResponseBody([]byte(`{
			"id":"chatcmpl_1",
			"object":"chat.completion",
			"created":123,
			"model":"openai/gpt-4o-mini",
			"choices":[{
				"index":0,
				"message":{"role":"assistant","content":"hello"},
				"finish_reason":"stop"
			}],
			"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18}
		}`))
		require.Equal(t, types.ActionContinue, action)

		body := host.GetResponseBody()
		require.Equal(t, "response", gjson.GetBytes(body, "object").String(), string(body))
		require.Equal(t, "completed", gjson.GetBytes(body, "status").String())
		require.Equal(t, "output_text", gjson.GetBytes(body, "output.0.content.0.type").String())
		require.Equal(t, "hello", gjson.GetBytes(body, "output.0.content.0.text").String())
		require.Equal(t, int64(11), gjson.GetBytes(body, "usage.input_tokens").Int())
		require.False(t, gjson.GetBytes(body, "choices").Exists(), string(body))
	})
}

func TestQwenResponsesFallbackConvertsNonStreamingRequestAndResponse(t *testing.T) {
	wasmhost.RunTest(t, func(t *testing.T) {
		host, status := wasmhost.NewTestHost(responsesFallbackQwenConfig)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "dashscope.aliyuncs.com"},
			{":path", "/v1/responses"},
			{":method", "POST"},
			{"Content-Type", "application/json"},
		})
		require.Equal(t, types.HeaderStopIteration, action)
		pathValue, hasPath := wasmhost.GetHeaderValue(host.GetRequestHeaders(), ":path")
		require.True(t, hasPath)
		require.Equal(t, "/compatible-mode/v1/chat/completions", pathValue)

		action = host.CallOnHttpRequestBody([]byte(`{"model":"deepseek-v4-flash","input":"hello","max_output_tokens":32}`))
		require.Equal(t, types.ActionContinue, action)
		requestBody := host.GetRequestBody()
		require.Equal(t, "deepseek-v4-flash", gjson.GetBytes(requestBody, "model").String())
		require.Equal(t, "hello", gjson.GetBytes(requestBody, "messages.0.content").String())
		require.Equal(t, int64(32), gjson.GetBytes(requestBody, "max_completion_tokens").Int())

		require.NoError(t, host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
		action = host.CallOnHttpResponseHeaders([][2]string{
			{":status", "200"},
			{"Content-Type", "application/json"},
		})
		require.Equal(t, types.ActionContinue, action)

		action = host.CallOnHttpResponseBody([]byte(`{
			"id":"chatcmpl_qwen",
			"object":"chat.completion",
			"created":123,
			"model":"deepseek-v4-flash",
			"choices":[{"index":0,"message":{"role":"assistant","content":"qwen fallback ok"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":9,"completion_tokens":4,"total_tokens":13}
		}`))
		require.Equal(t, types.ActionContinue, action)
		responseBody := host.GetResponseBody()
		require.Equal(t, "response", gjson.GetBytes(responseBody, "object").String(), string(responseBody))
		require.Equal(t, "qwen fallback ok", gjson.GetBytes(responseBody, "output.0.content.0.text").String())
		require.Equal(t, int64(9), gjson.GetBytes(responseBody, "usage.input_tokens").Int())
	})
}

func TestQwenResponsesFallbackConvertsStreamingRequestAndSSE(t *testing.T) {
	wasmhost.RunTest(t, func(t *testing.T) {
		host, status := wasmhost.NewTestHost(responsesFallbackQwenConfig)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "dashscope.aliyuncs.com"},
			{":path", "/v1/responses"},
			{":method", "POST"},
			{"Content-Type", "application/json"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		action = host.CallOnHttpRequestBody([]byte(`{"model":"deepseek-v4-flash","input":"hello","stream":true}`))
		require.Equal(t, types.ActionContinue, action)
		pathValue, hasPath := wasmhost.GetHeaderValue(host.GetRequestHeaders(), ":path")
		require.True(t, hasPath)
		require.Equal(t, "/compatible-mode/v1/chat/completions", pathValue)
		require.True(t, gjson.GetBytes(host.GetRequestBody(), "stream").Bool())

		require.NoError(t, host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
		action = host.CallOnHttpResponseHeaders([][2]string{
			{":status", "200"},
			{"Content-Type", "text/event-stream"},
		})
		require.Equal(t, types.ActionContinue, action)

		action = host.CallOnHttpResponseBody([]byte("data: {\"id\":\"chatcmpl_qwen\",\"object\":\"chat.completion.chunk\",\"created\":123,\"model\":\"deepseek-v4-flash\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"Hello\"}}]}\n\n"))
		require.Equal(t, types.ActionContinue, action)
		responseBody := string(host.GetResponseBody())
		require.Contains(t, responseBody, "event: response.output_text.delta")
		require.Contains(t, responseBody, `"delta":"Hello"`)
		require.NotContains(t, responseBody, "chat.completion.chunk")
	})
}

func TestQwenResponsesFallbackSurvivesMultiProviderOverrideMerge(t *testing.T) {
	wasmhost.RunGoTest(t, func(t *testing.T) {
		bootstrap, status := wasmhost.NewTestHost(responsesFallbackQwenConfig)
		defer bootstrap.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		var global config.PluginConfig
		global.FromJson(gjson.Parse(`{
			"providers":[
				{"id":"deepseek-local","type":"deepseek","apiTokens":["sk-deepseek"]},
				{"id":"qwen-local","type":"qwen","apiTokens":["sk-qwen"],"qwenEnableCompatible":true,
				 "capabilities":{"openai/v1/responses":""}}
			]
		}`))
		require.NoError(t, global.Validate())
		require.NoError(t, global.Complete())

		override := global
		override.FromJson(gjson.Parse(`{"activeProviderId":"qwen-local"}`))
		require.NoError(t, override.Validate())
		require.NoError(t, override.Complete())
		require.Equal(t, "qwen", override.GetProvider().GetProviderType())
		require.False(t, override.GetProviderConfig().IsSupportedAPI(provider.ApiNameResponses))
		require.True(t, override.GetProviderConfig().IsSupportedAPI(provider.ApiNameChatCompletion))
	})
}

func TestResponsesUnsupportedProviderDoesNotFallbackToUnrelatedAPI(t *testing.T) {
	wasmhost.RunTest(t, func(t *testing.T) {
		host, status := wasmhost.NewTestHost(responsesUnsupportedKlingConfig)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "example.com"},
			{":path", "/v1/responses"},
			{":method", "POST"},
			{"Content-Type", "application/json"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		pathValue, hasPath := wasmhost.GetHeaderValue(host.GetRequestHeaders(), ":path")
		require.True(t, hasPath)
		require.Equal(t, "/v1/responses", pathValue)

		_ = host.CallOnHttpRequestBody([]byte(`{"model":"gpt-4o-mini","input":"hello"}`))
		allLogs := strings.Join(append(append([]string{}, host.GetErrorLogs()...), host.GetWarnLogs()...), "\n")
		require.Contains(t, allLogs, "unsupported API name")
	})
}

func TestResponsesFallbackDoesNotRegressMessagesOrChatCompletions(t *testing.T) {
	wasmhost.RunTest(t, func(t *testing.T) {
		host, status := wasmhost.NewTestHost(responsesFallbackOpenRouterConfig)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "example.com"},
			{":path", "/v1/messages"},
			{":method", "POST"},
			{"Content-Type", "application/json"},
		})
		require.Equal(t, types.HeaderStopIteration, action)
		pathValue, hasPath := wasmhost.GetHeaderValue(host.GetRequestHeaders(), ":path")
		require.True(t, hasPath)
		require.Equal(t, "/api/v1/chat/completions", pathValue)
	})

	wasmhost.RunTest(t, func(t *testing.T) {
		host, status := wasmhost.NewTestHost(responsesFallbackOpenRouterConfig)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "example.com"},
			{":path", "/v1/chat/completions"},
			{":method", "POST"},
			{"Content-Type", "application/json"},
		})
		require.Equal(t, types.HeaderStopIteration, action)
		pathValue, hasPath := wasmhost.GetHeaderValue(host.GetRequestHeaders(), ":path")
		require.True(t, hasPath)
		require.Equal(t, "/api/v1/chat/completions", pathValue)
	})
}
