package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	wasmhost "github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAIProxyURLBoundarySecurityContract(t *testing.T) {
	const (
		providerSecret = "provider-secret-marker-7f31"
		clientSecret   = "client-secret-marker-8e42"
		querySecret    = "query-secret-marker-9d53"
		bodySecret     = "body-secret-marker-a064"

		providerAuthorization = "Bearer " + providerSecret
		clientAuthorization   = "Bearer " + clientSecret
	)

	authorityConfig := json.RawMessage(`{
		"provider": {
			"type": "qwen",
			"apiTokens": ["provider-secret-marker-7f31"],
			"qwenDomain": "provider.example",
			"qwenEnableCompatible": false
		}
	}`)
	baseConfig := json.RawMessage(`{
		"provider": {
			"type": "qwen",
			"apiTokens": ["provider-secret-marker-7f31"],
			"qwenDomain": "provider.example",
			"qwenEnableCompatible": false,
			"providerBasePath": "/tenant/base"
		}
	}`)
	diagnosticConfig := json.RawMessage(`{
		"provider": {
			"type": "qwen",
			"apiTokens": ["provider-secret-marker-7f31"],
			"qwenDomain": "provider.example",
			"qwenEnableCompatible": true,
			"capabilities": {"openai/v1/responses": ""}
		}
	}`)

	collectDiagnostics := func(host wasmhost.TestHost) string {
		diagnostics := make([]string, 0)
		diagnostics = append(diagnostics, host.GetTraceLogs()...)
		diagnostics = append(diagnostics, host.GetDebugLogs()...)
		diagnostics = append(diagnostics, host.GetInfoLogs()...)
		diagnostics = append(diagnostics, host.GetWarnLogs()...)
		diagnostics = append(diagnostics, host.GetErrorLogs()...)
		diagnostics = append(diagnostics, host.GetCriticalLogs()...)
		if localResponse := host.GetLocalResponse(); localResponse != nil {
			diagnostics = append(diagnostics, string(localResponse.Data))
		}
		return strings.Join(diagnostics, "\n")
	}

	requireSafeDiagnostics := func(t *testing.T, diagnostics string) {
		t.Helper()

		for _, secret := range []string{
			providerSecret,
			clientSecret,
			querySecret,
			bodySecret,
		} {
			assert.NotContains(t, diagnostics, secret,
				"gateway diagnostics and local responses must not expose credentials, queries, or bodies")
		}
	}

	type unsafeCase struct {
		name          string
		requestTarget string
		config        json.RawMessage
	}
	unsafeCases := []unsafeCase{
		{
			name:          "rejects_absolute_url",
			requestTarget: "https://authority-marker.invalid/vendor/jobs?secret=" + querySecret,
			config:        authorityConfig,
		},
		{
			name:          "rejects_scheme_injection",
			requestTarget: "https:/vendor/jobs?secret=" + querySecret,
			config:        authorityConfig,
		},
		{
			name:          "rejects_network_path_authority",
			requestTarget: "//authority-marker.invalid/vendor/jobs?secret=" + querySecret,
			config:        authorityConfig,
		},
		{
			name:          "rejects_encoded_authority_escape",
			requestTarget: "/%2f%2fauthority-marker.invalid/vendor/jobs?secret=" + querySecret,
			config:        authorityConfig,
		},
		{
			name:          "rejects_raw_base_traversal",
			requestTarget: "/../../vendor/jobs?secret=" + querySecret,
			config:        baseConfig,
		},
		{
			name:          "rejects_encoded_base_traversal",
			requestTarget: "/%2e%2e/%2e%2e/vendor/jobs?secret=" + querySecret,
			config:        baseConfig,
		},
	}

	wasmhost.RunGoTest(t, func(t *testing.T) {
		for _, testCase := range unsafeCases {
			t.Run(testCase.name, func(t *testing.T) {
				host, status := wasmhost.NewTestHost(testCase.config)
				require.Equal(t, types.OnPluginStartStatusOK, status)
				t.Cleanup(host.Reset)

				host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "client.example"},
					{":path", testCase.requestTarget},
					{":method", "PATCH"},
					{"authorization", clientAuthorization},
					{"content-type", "application/json"},
				})

				localResponse := host.GetLocalResponse()
				assert.NotNil(t, localResponse,
					"unsafe request target must be rejected locally before dispatch")
				if localResponse != nil {
					assert.GreaterOrEqual(t, localResponse.StatusCode, uint32(400))
					assert.LessOrEqual(t, localResponse.StatusCode, uint32(599))
				} else {
					forwardedHeaders := host.GetRequestHeaders()
					forwardedTarget, hasTarget := wasmhost.GetHeaderValue(forwardedHeaders, ":path")
					require.True(t, hasTarget)
					forwardedAuthorization, hasAuthorization := wasmhost.GetHeaderValue(
						forwardedHeaders,
						"authorization",
					)
					if hasAuthorization {
						assert.NotEqual(t,
							providerAuthorization,
							forwardedAuthorization,
							"unrejected unsafe target %q was forwarded as %q with authorization %q",
							testCase.requestTarget,
							forwardedTarget,
							forwardedAuthorization,
						)
					}

					host.CallOnHttpRequestBody([]byte(
						`{"opaque":"body-secret-marker-a064"}`,
					))
				}

				requireSafeDiagnostics(t, collectDiagnostics(host))
			})
		}

		t.Run("preserves_safe_origin_form_raw_query_with_provider_policy", func(t *testing.T) {
			host, status := wasmhost.NewTestHost(baseConfig)
			require.Equal(t, types.OnPluginStartStatusOK, status)
			t.Cleanup(host.Reset)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "client.example"},
				{":path", "/api/v1/vendor/jobs/job-42?sig=a%2Bb&dup=1&dup=2&empty="},
				{":method", "PATCH"},
				{"authorization", clientAuthorization},
				{"content-type", "application/octet-stream"},
			})

			require.Nil(t, host.GetLocalResponse())
			forwardedHeaders := host.GetRequestHeaders()

			authority, hasAuthority := wasmhost.GetHeaderValue(forwardedHeaders, ":authority")
			require.True(t, hasAuthority)
			require.Equal(t, "provider.example", authority)

			authorization, hasAuthorization := wasmhost.GetHeaderValue(
				forwardedHeaders,
				"authorization",
			)
			require.True(t, hasAuthorization)
			require.Equal(t, providerAuthorization, authorization)

			target, hasTarget := wasmhost.GetHeaderValue(forwardedHeaders, ":path")
			require.True(t, hasTarget)
			path, rawQuery, hasQuery := strings.Cut(target, "?")
			require.True(t, hasQuery)
			require.Equal(t, "/tenant/base/api/v1/vendor/jobs/job-42", path)
			require.Equal(t, "sig=a%2Bb&dup=1&dup=2&empty=", rawQuery)
		})

		t.Run("unsafe_request_diagnostics_exclude_config_query_credentials_and_body", func(t *testing.T) {
			host, status := wasmhost.NewTestHost(diagnosticConfig)
			require.Equal(t, types.OnPluginStartStatusOK, status)
			t.Cleanup(host.Reset)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "client.example"},
				{
					":path",
					"https://authority-marker.invalid/v1/responses?secret=" + querySecret,
				},
				{":method", "POST"},
				{"authorization", clientAuthorization},
				{"content-type", "application/json"},
			})

			if host.GetLocalResponse() == nil {
				host.CallOnHttpRequestBody([]byte(
					`{"model":"deepseek-v4-flash","input":"body-secret-marker-a064"}`,
				))
			}

			requireSafeDiagnostics(t, collectDiagnostics(host))
		})
	})
}
