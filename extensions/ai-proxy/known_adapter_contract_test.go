package main

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	wasmhost "github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestKnownQwenResponsesAdapterContract(t *testing.T) {
	const adapterMetadataProperty = "mse_ai_proxy_adapter_metadata"
	const maxAdapterMetadataBytes = 1024

	type adapterMetadata struct {
		AdapterID           string `json:"adapter_id"`
		Version             string `json:"version"`
		NormalizedOperation string `json:"normalized_operation"`
		ProviderMatch       string `json:"provider_match"`
		CapabilityMatch     string `json:"capability_match"`
		UsageProfile        string `json:"usage_profile"`
	}

	readAdapterMetadata := func(t *testing.T, host wasmhost.TestHost) (adapterMetadata, bool) {
		t.Helper()

		raw, err := host.GetProperty([]string{adapterMetadataProperty})
		if err != nil || len(raw) == 0 {
			return adapterMetadata{}, false
		}
		require.LessOrEqual(t, len(raw), maxAdapterMetadataBytes, "trusted adapter metadata must remain bounded")

		var metadata adapterMetadata
		require.NoError(t, json.Unmarshal(raw, &metadata), "trusted adapter metadata must be valid JSON")
		return metadata, true
	}

	claimsQwenResponsesFallback := func(metadata adapterMetadata) bool {
		return metadata.ProviderMatch == "qwen" &&
			metadata.NormalizedOperation == "openai/v1/responses" &&
			metadata.CapabilityMatch == "openai/v1/chat/completions"
	}

	requireNoQwenResponsesFallback := func(t *testing.T, host wasmhost.TestHost) {
		t.Helper()

		metadata, exists := readAdapterMetadata(t, host)
		if !exists {
			return
		}
		require.False(t, claimsQwenResponsesFallback(metadata),
			"request mismatch must not claim the Qwen Responses-to-Chat fallback: %+v", metadata)
	}

	wasmhost.RunGoTest(t, func(t *testing.T) {
		t.Run("qwen_capability_sentinel_matrix", func(t *testing.T) {
			tests := []struct {
				name     string
				config   json.RawMessage
				wantPath string
			}{
				{
					name: "qwen_explicit_empty_uses_chat_completions_fallback",
					config: json.RawMessage(`{"provider":{
						"type":"qwen",
						"apiTokens":["sk-qwen-empty"],
						"qwenEnableCompatible":true,
						"capabilities":{"openai/v1/responses":""}
					}}`),
					wantPath: "/compatible-mode/v1/chat/completions",
				},
				{
					name: "qwen_whitespace_only_uses_chat_completions_fallback",
					config: json.RawMessage(`{"provider":{
						"type":"qwen",
						"apiTokens":["sk-qwen-whitespace"],
						"qwenEnableCompatible":true,
						"capabilities":{"openai/v1/responses":"  \t "}
					}}`),
					wantPath: "/compatible-mode/v1/chat/completions",
				},
				{
					name: "qwen_omitted_capability_preserves_native_default",
					config: json.RawMessage(`{"provider":{
						"type":"qwen",
						"apiTokens":["sk-qwen-omitted"],
						"qwenEnableCompatible":true
					}}`),
					wantPath: "/compatible-mode/v1/responses",
				},
				{
					name: "qwen_non_empty_capability_preserves_native_override",
					config: json.RawMessage(`{"provider":{
						"type":"qwen",
						"apiTokens":["sk-qwen-non-empty"],
						"qwenEnableCompatible":true,
						"capabilities":{"openai/v1/responses":"/custom/v1/responses"}
					}}`),
					wantPath: "/custom/v1/responses",
				},
				{
					name: "openai_empty_capability_keeps_key_presence_semantics",
					config: json.RawMessage(`{"provider":{
						"type":"openai",
						"apiTokens":["sk-openai-empty"],
						"capabilities":{"openai/v1/responses":""}
					}}`),
					wantPath: "/v1/responses",
				},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					host, status := wasmhost.NewTestHost(tt.config)
					defer host.Reset()
					require.Equal(t, types.OnPluginStartStatusOK, status)

					action := host.CallOnHttpRequestHeaders([][2]string{
						{":authority", "provider.example"},
						{":path", "/v1/responses"},
						{":method", "POST"},
						{"Content-Type", "application/json"},
					})
					require.Equal(t, types.HeaderStopIteration, action)

					pathValue, hasPath := wasmhost.GetHeaderValue(host.GetRequestHeaders(), ":path")
					require.True(t, hasPath)
					require.Equal(t, tt.wantPath, pathValue)
				})
			}
		})

		t.Run("exact_positive_match_transforms_and_publishes_stable_metadata", func(t *testing.T) {
			exerciseMatch := func(t *testing.T) adapterMetadata {
				t.Helper()

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

				action = host.CallOnHttpRequestBody(
					[]byte(`{"model":"deepseek-v4-flash","input":"hello","max_output_tokens":32}`),
				)
				require.Equal(t, types.ActionContinue, action)

				requestBody := host.GetRequestBody()
				require.Equal(t, "deepseek-v4-flash", gjson.GetBytes(requestBody, "model").String())
				require.Equal(t, "user", gjson.GetBytes(requestBody, "messages.0.role").String())
				require.Equal(t, "hello", gjson.GetBytes(requestBody, "messages.0.content").String())
				require.Equal(t, int64(32), gjson.GetBytes(requestBody, "max_completion_tokens").Int())
				require.False(t, gjson.GetBytes(requestBody, "input").Exists(), string(requestBody))

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
					"choices":[{
						"index":0,
						"message":{"role":"assistant","content":"qwen fallback ok"},
						"finish_reason":"stop"
					}],
					"usage":{"prompt_tokens":9,"completion_tokens":4,"total_tokens":13}
				}`))
				require.Equal(t, types.ActionContinue, action)

				responseBody := host.GetResponseBody()
				require.Equal(t, "response", gjson.GetBytes(responseBody, "object").String(), string(responseBody))
				require.Equal(t, "output_text", gjson.GetBytes(responseBody, "output.0.content.0.type").String())
				require.Equal(t, "qwen fallback ok", gjson.GetBytes(responseBody, "output.0.content.0.text").String())
				require.Equal(t, int64(9), gjson.GetBytes(responseBody, "usage.input_tokens").Int())
				require.False(t, gjson.GetBytes(responseBody, "choices").Exists(), string(responseBody))

				metadata, exists := readAdapterMetadata(t, host)
				require.True(t, exists, "matched adapter must publish trusted adapter metadata")
				require.NotEmpty(t, metadata.AdapterID)
				require.NotEmpty(t, metadata.Version)
				require.Equal(t, "openai/v1/responses", metadata.NormalizedOperation)
				require.Equal(t, "qwen", metadata.ProviderMatch)
				require.Equal(t, "openai/v1/chat/completions", metadata.CapabilityMatch)
				require.NotEmpty(t, metadata.UsageProfile)
				return metadata
			}

			first := exerciseMatch(t)
			second := exerciseMatch(t)
			require.Equal(t, first.AdapterID, second.AdapterID, "adapter identity must be stable across identical matches")
			require.Equal(t, first.Version, second.Version, "adapter version must be stable across identical matches")
		})

		t.Run("method_mismatch_does_not_select_responses_fallback", func(t *testing.T) {
			host, status := wasmhost.NewTestHost(responsesFallbackQwenConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "dashscope.aliyuncs.com"},
				{":path", "/v1/responses"},
				{":method", "GET"},
				{"Content-Type", "application/json"},
			})
			require.Equal(t, types.HeaderStopIteration, action)

			pathValue, hasPath := wasmhost.GetHeaderValue(host.GetRequestHeaders(), ":path")
			require.True(t, hasPath)
			require.Equal(t, "/v1/responses", pathValue,
				"GET must not select the POST Responses-to-Chat fallback")
			requireNoQwenResponsesFallback(t, host)
		})

		t.Run("path_mismatch_does_not_select_responses_fallback", func(t *testing.T) {
			host, status := wasmhost.NewTestHost(responsesFallbackQwenConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "dashscope.aliyuncs.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"Content-Type", "application/json"},
			})
			require.Equal(t, types.HeaderStopIteration, action)
			requireNoQwenResponsesFallback(t, host)
		})

		t.Run("capability_mismatch_does_not_select_responses_fallback", func(t *testing.T) {
			nativeQwenConfig := json.RawMessage(`{"provider":{
				"type":"qwen",
				"apiTokens":["sk-qwen-native"],
				"qwenEnableCompatible":true,
				"capabilities":{"openai/v1/responses":"/custom/v1/responses"}
			}}`)
			host, status := wasmhost.NewTestHost(nativeQwenConfig)
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
			require.Equal(t, "/custom/v1/responses", pathValue)
			requireNoQwenResponsesFallback(t, host)
		})

		t.Run("provider_mismatch_does_not_select_qwen_fallback", func(t *testing.T) {
			host, status := wasmhost.NewTestHost(responsesNativeOpenAIConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "api.openai.com"},
				{":path", "/v1/responses"},
				{":method", "POST"},
				{"Content-Type", "application/json"},
			})
			require.Equal(t, types.HeaderStopIteration, action)

			pathValue, hasPath := wasmhost.GetHeaderValue(host.GetRequestHeaders(), ":path")
			require.True(t, hasPath)
			require.Equal(t, "/v1/responses", pathValue)
			requireNoQwenResponsesFallback(t, host)
		})
	})
}
