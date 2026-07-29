package main

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	wasmhost "github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForcedQwenResponsesPassthroughContract(t *testing.T) {
	const adapterMetadataProperty = "mse_ai_proxy_adapter_metadata"
	const maxAdapterMetadataBytes = 1024

	config := json.RawMessage(`{
		"protocol_mode":"passthrough",
		"provider":{
			"type":"qwen",
			"apiTokens":["provider-passthrough-token"],
			"qwenDomain":"provider.example",
			"qwenEnableCompatible":true,
			"capabilities":{"openai/v1/responses":""}
		}
	}`)

	type adapterMetadata struct {
		AdapterID           string `json:"adapter_id"`
		NormalizedOperation string `json:"normalized_operation"`
		ProviderMatch       string `json:"provider_match"`
		CapabilityMatch     string `json:"capability_match"`
	}

	requireSkippedAdapterNotClaimed := func(t *testing.T, host wasmhost.TestHost) {
		t.Helper()

		raw, err := host.GetProperty([]string{adapterMetadataProperty})
		if err != nil || len(raw) == 0 {
			return
		}
		require.LessOrEqual(t, len(raw), maxAdapterMetadataBytes,
			"trusted adapter metadata must remain bounded")

		var metadata adapterMetadata
		require.NoError(t, json.Unmarshal(raw, &metadata),
			"trusted adapter metadata must be valid JSON")
		claimsSkippedAdapter := metadata.AdapterID != "" &&
			metadata.ProviderMatch == "qwen" &&
			metadata.NormalizedOperation == "openai/v1/responses" &&
			metadata.CapabilityMatch == "openai/v1/chat/completions"
		assert.False(t, claimsSkippedAdapter,
			"forced passthrough must not claim the skipped Qwen Responses-to-Chat adapter: %+v",
			metadata)
	}

	startHost := func(t *testing.T) wasmhost.TestHost {
		t.Helper()

		host, status := wasmhost.NewTestHost(config)
		require.Equal(t, types.OnPluginStartStatusOK, status)
		t.Cleanup(host.Reset)

		host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "attacker.example"},
			{":path", "/v1/responses"},
			{":method", "POST"},
			{"authorization", "Bearer attacker-token"},
			{"content-type", "application/json"},
		})

		headers := host.GetRequestHeaders()
		authority, hasAuthority := wasmhost.GetHeaderValue(headers, ":authority")
		require.True(t, hasAuthority)
		require.Equal(t, "provider.example", authority,
			"selected provider must remain authoritative for the upstream host")
		authorization, hasAuthorization := wasmhost.GetHeaderValue(headers, "authorization")
		require.True(t, hasAuthorization)
		require.Equal(t, "Bearer provider-passthrough-token", authorization,
			"selected provider must remain authoritative for authentication")
		return host
	}

	wasmhost.RunGoTest(t, func(t *testing.T) {
		t.Run("non_streaming_preserves_known_adapter_bytes", func(t *testing.T) {
			host := startHost(t)

			path, hasPath := wasmhost.GetHeaderValue(host.GetRequestHeaders(), ":path")
			require.True(t, hasPath)
			assert.Equal(t, "/v1/responses", path)

			requestBody := []byte(
				`{"model":"deepseek-v4-flash","input":"hello","max_output_tokens":32}`,
			)
			host.CallOnHttpRequestBody(requestBody)
			assert.Equal(t, requestBody, host.GetRequestBody())

			require.NoError(t,
				host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			responseBody := []byte(
				`{"id":"chatcmpl_passthrough","object":"chat.completion","created":123,"model":"deepseek-v4-flash","choices":[{"index":0,"message":{"role":"assistant","content":"opaque upstream bytes"},"finish_reason":"stop"}],"usage":{"prompt_tokens":9,"completion_tokens":4,"total_tokens":13}}`,
			)
			host.CallOnHttpResponseBody(responseBody)
			assert.Equal(t, responseBody, host.GetResponseBody())
			requireSkippedAdapterNotClaimed(t, host)
		})

		t.Run("streaming_preserves_known_adapter_request_and_sse", func(t *testing.T) {
			host := startHost(t)

			path, hasPath := wasmhost.GetHeaderValue(host.GetRequestHeaders(), ":path")
			require.True(t, hasPath)
			assert.Equal(t, "/v1/responses", path)

			requestBody := []byte(
				`{"model":"deepseek-v4-flash","input":"hello","stream":true}`,
			)
			host.CallOnHttpRequestBody(requestBody)
			assert.Equal(t, requestBody, host.GetRequestBody())

			require.NoError(t,
				host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})
			responseBody := []byte(
				"data: {\"id\":\"chatcmpl_passthrough\",\"object\":\"chat.completion.chunk\",\"created\":123,\"model\":\"deepseek-v4-flash\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"Hello\"}}]}\n\n",
			)
			host.CallOnHttpResponseBody(responseBody)
			assert.Equal(t, responseBody, host.GetResponseBody())
			requireSkippedAdapterNotClaimed(t, host)
		})
	})
}
