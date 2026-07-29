package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	wasmhost "github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestTransparentQwenUnknownFallbackContract(t *testing.T) {
	const (
		unknownOperation = "/api/v1/vendor/jobs/job%2F42:cancel"
		rawQuery         = "sig=a%2Bb&dup=1&dup=2"
		requestTarget    = unknownOperation + "?" + rawQuery
	)

	providerTokens := []string{
		"Bearer provider-token-a",
		"Bearer provider-token-b",
	}
	config := json.RawMessage(`{
		"provider": {
			"type": "qwen",
			"apiTokens": ["provider-token-a", "provider-token-b"],
			"qwenDomain": "provider.example",
			"firstByteTimeout": 3210,
			"failover": {
				"enabled": true,
				"failureThreshold": 1,
				"cooldownDuration": 60000,
				"failoverOnStatus": ["429"]
			}
		}
	}`)

	requestHeaders := func() [][2]string {
		return [][2]string{
			{":authority", "attacker.example"},
			{":path", requestTarget},
			{":method", "PATCH"},
			{"authorization", "Bearer attacker-token"},
			{"content-type", "application/octet-stream"},
			{"x-envoy-upstream-rq-first-byte-timeout-ms", "1"},
		}
	}

	wasmhost.RunGoTest(t, func(t *testing.T) {
		t.Run("preserves_opaque_operation_with_provider_transport_policy", func(t *testing.T) {
			host, status := wasmhost.NewTestHost(config)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders(requestHeaders())

			forwardedHeaders := host.GetRequestHeaders()

			method, hasMethod := wasmhost.GetHeaderValue(forwardedHeaders, ":method")
			require.True(t, hasMethod)
			require.Equal(t, "PATCH", method)

			target, hasTarget := wasmhost.GetHeaderValue(forwardedHeaders, ":path")
			require.True(t, hasTarget)
			path, query, hasQuery := strings.Cut(target, "?")
			require.True(t, hasQuery)
			require.Equal(t, unknownOperation, path)
			require.Equal(t, rawQuery, query)

			authority, hasAuthority := wasmhost.GetHeaderValue(forwardedHeaders, ":authority")
			require.True(t, hasAuthority)
			require.Equal(t, "provider.example", authority)

			authorization, hasAuthorization := wasmhost.GetHeaderValue(forwardedHeaders, "authorization")
			require.True(t, hasAuthorization)
			require.Contains(t, providerTokens, authorization)
			require.NotEqual(t, "Bearer attacker-token", authorization)

			opaqueRequest := []byte{0x00, 0xff, '{', '"', 'x', '"', ':', 0x80, '}'}
			action := host.CallOnHttpRequestBody(opaqueRequest)
			require.Equal(t, types.ActionContinue, action)
			require.Equal(t, opaqueRequest, host.GetRequestBody())

			require.NoError(t, host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/octet-stream"},
			})
			require.Equal(t, types.ActionContinue, action)

			opaqueResponse := []byte{0xfe, 0x00, 'o', 'p', 'a', 'q', 'u', 'e', 0x81}
			action = host.CallOnHttpResponseBody(opaqueResponse)
			require.Equal(t, types.ActionContinue, action)
			require.Equal(t, opaqueResponse, host.GetResponseBody())

			timeout, hasTimeout := wasmhost.GetHeaderValue(
				forwardedHeaders,
				"x-envoy-upstream-rq-first-byte-timeout-ms",
			)
			require.True(t, hasTimeout)
			require.Equal(t, "3210", timeout, "configured provider timeout must override request-controlled timeout")
		})

		t.Run("configured_failover_status_removes_the_failed_provider_token", func(t *testing.T) {
			host, status := wasmhost.NewTestHost(config)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders(requestHeaders())
			firstHeaders := host.GetRequestHeaders()
			firstAuthorization, hasFirstAuthorization := wasmhost.GetHeaderValue(firstHeaders, "authorization")
			require.True(t, hasFirstAuthorization)
			require.Contains(t, providerTokens, firstAuthorization)

			require.NoError(t, host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
			action := host.CallOnHttpResponseHeaders([][2]string{
				{":status", "429"},
				{"content-type", "application/octet-stream"},
			})
			require.Equal(t, types.ActionContinue, action)
			host.CompleteHttp()

			host.CallOnHttpRequestHeaders(requestHeaders())
			secondHeaders := host.GetRequestHeaders()

			secondAuthority, hasSecondAuthority := wasmhost.GetHeaderValue(secondHeaders, ":authority")
			require.True(t, hasSecondAuthority)
			require.Equal(t, "provider.example", secondAuthority)

			secondAuthorization, hasSecondAuthorization := wasmhost.GetHeaderValue(secondHeaders, "authorization")
			require.True(t, hasSecondAuthorization)
			require.Contains(t, providerTokens, secondAuthorization)
			require.NotEqual(t, firstAuthorization, secondAuthorization)
			require.NotEqual(t, "Bearer attacker-token", secondAuthorization)

			allLogs := collectAllHostLogs(host)
			for _, token := range []string{"provider-token-a", "provider-token-b", "attacker-token"} {
				require.NotContains(t, allLogs, token)
			}
		})
	})
}
