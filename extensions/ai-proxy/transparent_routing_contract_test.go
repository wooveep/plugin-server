package main

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	wasmhost "github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestSafeOriginFormTargetValidatorContract(t *testing.T) {
	validTargets := map[string]string{
		"/v1/responses": "/v1/responses",
		"/api/v1/vendor/jobs/job%2F42:cancel?sig=a%2Bb&dup=1&dup=2": "/api/v1/vendor/jobs/job/42:cancel",
		"/ordinary/.../segment": "/ordinary/.../segment",
	}
	for target, expectedPath := range validTargets {
		target := target
		expectedPath := expectedPath
		t.Run("accepts_"+target, func(t *testing.T) {
			classificationPath, err := validateSafeOriginFormTarget(target)
			require.NoError(t, err)
			require.Equal(t, expectedPath, classificationPath)
		})
	}

	unsafeTargets := []string{
		"",
		"relative/path",
		"https://authority.invalid/vendor/jobs",
		"https:/vendor/jobs",
		"//authority.invalid/vendor/jobs",
		"/%2f%2fauthority.invalid/vendor/jobs",
		"/%252f%252fauthority.invalid/vendor/jobs",
		"/../../vendor/jobs",
		"/%2e%2e/%2e%2e/vendor/jobs",
		"/%252e%252e/vendor/jobs",
		"/bad%zz",
		"/backslash\\authority",
		"/decoded%5cauthority",
		"/nested%255cauthority",
		"/fragment#value",
		"/decoded%23fragment",
		"/nested%2523fragment",
		"/decoded%00nul",
		"/decoded%0d%0ainjected",
		"/decoded%7fdelete",
		"/nested%2500nul",
		"/nested%250d%250ainjected",
		"/nested%257fdelete",
		"/%25252525252525252f%25252525252525252fauthority.invalid",
	}
	for _, target := range unsafeTargets {
		target := target
		t.Run("rejects_"+target, func(t *testing.T) {
			_, err := validateSafeOriginFormTarget(target)
			require.Error(t, err)
		})
	}
}

func TestTransparentProviderTimeoutAndLocalRejectContract(t *testing.T) {
	config := json.RawMessage(`{
		"provider": {
			"type": "qwen",
			"apiTokens": ["provider-timeout-token"],
			"qwenDomain": "provider.example"
		}
	}`)

	wasmhost.RunGoTest(t, func(t *testing.T) {
		t.Run("removes_request_controlled_timeout_when_provider_omits_one", func(t *testing.T) {
			host, status := wasmhost.NewTestHost(config)
			require.Equal(t, types.OnPluginStartStatusOK, status)
			t.Cleanup(host.Reset)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "client.example"},
				{":path", "/vendor/jobs/job-42"},
				{":method", "PATCH"},
				{"authorization", "Bearer client-token"},
				{"content-type", "application/octet-stream"},
				{"x-envoy-upstream-rq-first-byte-timeout-ms", "9000"},
			})

			_, hasTimeout := wasmhost.GetHeaderValue(
				host.GetRequestHeaders(),
				"x-envoy-upstream-rq-first-byte-timeout-ms",
			)
			require.False(t, hasTimeout)
		})

		t.Run("returns_fixed_local_response_for_unsafe_target", func(t *testing.T) {
			host, status := wasmhost.NewTestHost(config)
			require.Equal(t, types.OnPluginStartStatusOK, status)
			t.Cleanup(host.Reset)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "client.example"},
				{":path", "https://authority.invalid/vendor/jobs?secret=hidden"},
				{":method", "PATCH"},
				{"authorization", "Bearer client-token"},
			})

			localResponse := host.GetLocalResponse()
			require.NotNil(t, localResponse)
			require.Equal(t, uint32(400), localResponse.StatusCode)
			require.Equal(t, "ai-proxy.invalid_request_target", localResponse.StatusCodeDetail)
			require.Equal(t, []byte("invalid request target"), localResponse.Data)
		})
	})
}

func TestTransparentLifecycleKeepsKnownAdapterReentryEnabled(t *testing.T) {
	config := json.RawMessage(`{
		"provider": {
			"type": "qwen",
			"apiTokens": ["provider-reentry-token"],
			"qwenDomain": "provider.example",
			"qwenEnableCompatible": true,
			"capabilities": {"openai/v1/responses": ""}
		}
	}`)

	wasmhost.RunGoTest(t, func(t *testing.T) {
		host, status := wasmhost.NewTestHost(config)
		require.Equal(t, types.OnPluginStartStatusOK, status)
		t.Cleanup(host.Reset)

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "client.example"},
			{":path", "/vendor/jobs/job-42"},
			{":method", "PATCH"},
			{"authorization", "Bearer client-token"},
			{"content-type", "application/json"},
		})
		require.Equal(t, types.ActionContinue, action)

		action = host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "client.example"},
			{":path", "/v1/responses"},
			{":method", "POST"},
			{"authorization", "Bearer client-token"},
			{"content-type", "application/json"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		action = host.CallOnHttpRequestBody([]byte(
			`{"model":"deepseek-v4-flash","input":"reentered request"}`,
		))
		require.Equal(t, types.ActionContinue, action)
		require.JSONEq(t,
			`{"messages":[{"role":"user","content":"reentered request"}],"model":"deepseek-v4-flash"}`,
			string(host.GetRequestBody()),
		)

		require.NoError(t,
			host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
		action = host.CallOnHttpResponseHeaders([][2]string{
			{":status", "200"},
			{"content-type", "application/json"},
		})
		require.Equal(t, types.ActionContinue, action)

		action = host.CallOnHttpResponseBody([]byte(
			`{"id":"chatcmpl_reentry","object":"chat.completion","created":123,"model":"deepseek-v4-flash","choices":[{"index":0,"message":{"role":"assistant","content":"reentered response"},"finish_reason":"stop"}]}`,
		))
		require.Equal(t, types.ActionContinue, action)
		require.Contains(t, string(host.GetResponseBody()), `"object":"response"`)
		require.Contains(t, string(host.GetResponseBody()), `"reentered response"`)
	})
}

func TestAutoOriginalProviderClassificationPrecedesTransparentFallback(t *testing.T) {
	config := json.RawMessage(`{
		"provider": {
			"type": "vertex",
			"apiTokens": ["provider-original-token"],
			"protocol": "original"
		}
	}`)

	wasmhost.RunGoTest(t, func(t *testing.T) {
		host, status := wasmhost.NewTestHost(config)
		require.Equal(t, types.OnPluginStartStatusOK, status)
		t.Cleanup(host.Reset)

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "client.example"},
			{":path", "/v1/projects/test-project/locations/us-east5/publishers/anthropic/models/claude-sonnet-4@20250514:rawPredict"},
			{":method", "POST"},
			{"content-type", "application/json"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		requestBody := []byte(`{"anthropic_version":"vertex-2023-10-16","messages":[{"role":"user","content":"Hello"}]}`)
		action = host.CallOnHttpRequestBody(requestBody)
		require.Equal(t, types.ActionContinue, action)
		require.Equal(t, requestBody, host.GetRequestBody())

		requestHeaders := host.GetRequestHeaders()
		path, hasPath := wasmhost.GetHeaderValue(requestHeaders, ":path")
		require.True(t, hasPath)
		require.Contains(t, path, "?key=provider-original-token")
	})
}
