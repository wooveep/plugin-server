package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	wasmhost "github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransparentTransportPolicyPreservesProviderIndependentOperation(t *testing.T) {
	const (
		unknownTarget = "/api/v1/vendor/jobs/job%2F42:cancel?sig=a%2Bb&dup=1&dup=2"
		forcedTarget  = "/v1/responses?sig=a%2Bb&dup=1&dup=2"
	)

	type testCase struct {
		name              string
		config            string
		target            string
		expectedAuthority string
		expectedAuthName  string
		expectedAuthValue string
	}
	testCases := []testCase{
		{
			name: "claude_code_auto_unknown",
			config: `{"provider":{
				"type":"claude",
				"apiTokens":["claude-provider-token"],
				"claudeCodeMode":true
			}}`,
			target:            unknownTarget,
			expectedAuthority: "api.anthropic.com",
			expectedAuthName:  "authorization",
			expectedAuthValue: "Bearer claude-provider-token",
		},
		{
			name: "claude_code_forced",
			config: `{"protocol_mode":"passthrough","provider":{
				"type":"claude",
				"apiTokens":["claude-provider-token"],
				"claudeCodeMode":true
			}}`,
			target:            forcedTarget,
			expectedAuthority: "api.anthropic.com",
			expectedAuthName:  "authorization",
			expectedAuthValue: "Bearer claude-provider-token",
		},
		{
			name: "openai_direct_custom_url_auto_unknown",
			config: `{"provider":{
				"type":"openai",
				"apiTokens":["openai-provider-token"],
				"openaiCustomUrl":"https://custom.openai.example/v1"
			}}`,
			target:            unknownTarget,
			expectedAuthority: "custom.openai.example",
			expectedAuthName:  "authorization",
			expectedAuthValue: "Bearer openai-provider-token",
		},
		{
			name: "openai_direct_custom_url_forced",
			config: `{"protocol_mode":"passthrough","provider":{
				"type":"openai",
				"apiTokens":["openai-provider-token"],
				"openaiCustomUrl":"https://custom.openai.example/v1"
			}}`,
			target:            forcedTarget,
			expectedAuthority: "custom.openai.example",
			expectedAuthName:  "authorization",
			expectedAuthValue: "Bearer openai-provider-token",
		},
		{
			name: "azure_full_service_url_auto_unknown",
			config: `{"provider":{
				"type":"azure",
				"apiTokens":["azure-provider-token"],
				"azureServiceUrl":"https://fullpath-resource.openai.azure.com/openai/deployments/fullpath-deployment/chat/completions?api-version=2024-02-15-preview"
			}}`,
			target:            unknownTarget,
			expectedAuthority: "fullpath-resource.openai.azure.com",
			expectedAuthName:  "api-key",
			expectedAuthValue: "azure-provider-token",
		},
		{
			name: "azure_full_service_url_forced",
			config: `{"protocol_mode":"passthrough","provider":{
				"type":"azure",
				"apiTokens":["azure-provider-token"],
				"azureServiceUrl":"https://fullpath-resource.openai.azure.com/openai/deployments/fullpath-deployment/chat/completions?api-version=2024-02-15-preview"
			}}`,
			target:            forcedTarget,
			expectedAuthority: "fullpath-resource.openai.azure.com",
			expectedAuthName:  "api-key",
			expectedAuthValue: "azure-provider-token",
		},
	}

	wasmhost.RunGoTest(t, func(t *testing.T) {
		for _, testCase := range testCases {
			testCase := testCase
			t.Run(testCase.name, func(t *testing.T) {
				host, status := wasmhost.NewTestHost(json.RawMessage(testCase.config))
				require.Equal(t, types.OnPluginStartStatusOK, status)
				t.Cleanup(host.Reset)

				requestHeaders := [][2]string{
					{":authority", "client.example"},
					{":path", testCase.target},
					{":method", "PATCH"},
					{"authorization", "Bearer client-token"},
					{"x-api-key", "client-api-key"},
					{"api-key", "client-azure-key"},
					{"content-type", "application/octet-stream"},
					{"content-length", "9"},
					{"anthropic-version", "client-version"},
					{"anthropic-beta", "client-beta"},
					{"user-agent", "client-agent"},
					{"x-app", "client-app"},
					{"x-operation-marker", "preserve-me"},
				}
				action := host.CallOnHttpRequestHeaders(requestHeaders)
				require.Equal(t, types.ActionContinue, action)

				forwardedHeaders := host.GetRequestHeaders()
				requireHeaderValue(t, forwardedHeaders, ":method", "PATCH")
				requireHeaderValue(t, forwardedHeaders, ":path", testCase.target)
				requireHeaderValue(t, forwardedHeaders, ":authority", testCase.expectedAuthority)
				requireHeaderValue(t, forwardedHeaders, testCase.expectedAuthName, testCase.expectedAuthValue)
				requireHeaderValue(t, forwardedHeaders, "content-type", "application/octet-stream")
				requireHeaderValue(t, forwardedHeaders, "content-length", "9")
				requireHeaderValue(t, forwardedHeaders, "anthropic-version", "client-version")
				requireHeaderValue(t, forwardedHeaders, "anthropic-beta", "client-beta")
				requireHeaderValue(t, forwardedHeaders, "user-agent", "client-agent")
				requireHeaderValue(t, forwardedHeaders, "x-app", "client-app")
				requireHeaderValue(t, forwardedHeaders, "x-operation-marker", "preserve-me")

				requestBody := []byte{0x00, 0xff, 'P', 'A', 'T', 'C', 'H', 0x80, 0x7f}
				require.Equal(t, types.ActionContinue, host.CallOnHttpRequestBody(requestBody))
				require.Equal(t, requestBody, host.GetRequestBody())
			})
		}
	})
}

func TestTransparentQwenRetryReplaysExactOperationAndResponse(t *testing.T) {
	const (
		requestTarget = "/api/v1/vendor/jobs/job%2F42:cancel?sig=a%2Bb&dup=1&dup=2"
		bodySecret    = "transparent-retry-body-secret"
		tokenA        = "transparent-retry-token-a"
		tokenB        = "transparent-retry-token-b"
	)
	requestBody := append([]byte{0x00, 0xff}, []byte(bodySecret)...)
	requestBody = append(requestBody, 0x80, 0x7f)
	responseBody := []byte{0xfe, 0x00, 'e', 'x', 'a', 'c', 't', 0x81}

	config := json.RawMessage(fmt.Sprintf(`{
		"provider":{
			"type":"qwen",
			"apiTokens":[%q,%q],
			"qwenDomain":"provider.example",
			"retryOnFailure":{
				"enabled":true,
				"maxRetries":1,
				"retryOnStatus":["429"]
			}
		}
	}`, tokenA, tokenB))

	wasmhost.RunGoTest(t, func(t *testing.T) {
		host, status := wasmhost.NewTestHost(config)
		require.Equal(t, types.OnPluginStartStatusOK, status)
		t.Cleanup(host.Reset)

		host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "client.example"},
			{":path", requestTarget},
			{":method", "PATCH"},
			{"authorization", "Bearer client-token"},
			{"content-type", "application/octet-stream"},
			{"content-length", fmt.Sprint(len(requestBody))},
			{"x-replay-marker", "preserve-me"},
		})
		firstAuthorization, hasFirstAuthorization := wasmhost.GetHeaderValue(
			host.GetRequestHeaders(),
			"authorization",
		)
		require.True(t, hasFirstAuthorization)
		require.Equal(t, types.ActionContinue, host.CallOnHttpRequestBody(requestBody))

		require.NoError(t, host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
		action := host.CallOnHttpResponseHeaders([][2]string{
			{":status", "429"},
			{"content-type", "application/octet-stream"},
		})
		require.Equal(t, types.HeaderStopAllIterationAndWatermark, action)

		callouts := host.GetHttpCalloutAttributes()
		require.Len(t, callouts, 1)
		retry := callouts[0]
		require.Equal(t, requestBody, retry.Body)
		requireHeaderValue(t, retry.Headers, ":method", "PATCH")
		requireHeaderValue(t, retry.Headers, ":path", requestTarget)
		requireHeaderValue(t, retry.Headers, ":authority", "provider.example")
		requireHeaderValue(t, retry.Headers, "content-type", "application/octet-stream")
		requireHeaderValue(t, retry.Headers, "content-length", fmt.Sprint(len(requestBody)))
		requireHeaderValue(t, retry.Headers, "x-replay-marker", "preserve-me")
		retryAuthorization, hasRetryAuthorization := wasmhost.GetHeaderValue(
			retry.Headers,
			"authorization",
		)
		require.True(t, hasRetryAuthorization)
		require.NotEqual(t, firstAuthorization, retryAuthorization)
		require.Contains(t, []string{"Bearer " + tokenA, "Bearer " + tokenB}, retryAuthorization)

		host.CallOnHttpCall([][2]string{
			{":status", "200"},
			{"content-type", "application/octet-stream"},
			{"x-upstream-marker", "exact-response"},
		}, responseBody)

		localResponse := host.GetLocalResponse()
		require.NotNil(t, localResponse)
		require.Equal(t, uint32(200), localResponse.StatusCode)
		require.Equal(t, responseBody, localResponse.Data)
		requireHeaderValue(t, localResponse.Headers, "content-type", "application/octet-stream")
		requireHeaderValue(t, localResponse.Headers, "x-upstream-marker", "exact-response")

		allLogs := collectAllHostLogs(host)
		for _, secret := range []string{tokenA, tokenB, bodySecret} {
			assert.NotContains(t, allLogs, secret)
		}
	})
}

func TestTransparentSuccessfulRequestLogsExcludeCredentialsAndBodies(t *testing.T) {
	const (
		providerToken = "transparent-success-provider-token"
		clientToken   = "transparent-success-client-token"
		bodySecret    = "transparent-success-body-secret"
	)
	config := json.RawMessage(fmt.Sprintf(`{"provider":{
		"type":"qwen",
		"apiTokens":[%q],
		"qwenDomain":"provider.example"
	}}`, providerToken))

	wasmhost.RunGoTest(t, func(t *testing.T) {
		host, status := wasmhost.NewTestHost(config)
		require.Equal(t, types.OnPluginStartStatusOK, status)
		t.Cleanup(host.Reset)

		host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "client.example"},
			{":path", "/vendor/jobs/job-42"},
			{":method", "PATCH"},
			{"authorization", "Bearer " + clientToken},
			{"content-type", "application/octet-stream"},
		})
		host.CallOnHttpRequestBody([]byte(bodySecret))

		require.NoError(t, host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
		host.CallOnHttpResponseHeaders([][2]string{
			{":status", "200"},
			{"content-type", "application/octet-stream"},
		})

		allLogs := collectAllHostLogs(host)
		for _, secret := range []string{providerToken, clientToken, bodySecret} {
			assert.NotContains(t, allLogs, secret)
		}
	})
}

func collectAllHostLogs(host wasmhost.TestHost) string {
	logs := make([]string, 0)
	logs = append(logs, host.GetTraceLogs()...)
	logs = append(logs, host.GetDebugLogs()...)
	logs = append(logs, host.GetInfoLogs()...)
	logs = append(logs, host.GetWarnLogs()...)
	logs = append(logs, host.GetErrorLogs()...)
	logs = append(logs, host.GetCriticalLogs()...)
	return strings.Join(logs, "\n")
}

func requireHeaderValue(t *testing.T, headers [][2]string, name, expected string) {
	t.Helper()
	value, ok := wasmhost.GetHeaderValue(headers, name)
	require.True(t, ok, "expected header %q", name)
	require.Equal(t, expected, value, "unexpected value for header %q", name)
}
