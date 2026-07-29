package main

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	wasmhost "github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAIProxyPathScopeContract(t *testing.T) {
	legacyOmittedConfig := json.RawMessage(`{
		"provider": {
			"type": "qwen",
			"apiTokens": ["provider-path-scope-token"],
			"qwenDomain": "provider.example",
			"qwenEnableCompatible": true,
			"capabilities": {
				"openai/v1/responses": ""
			}
		}
	}`)
	legacyExplicitConfig := json.RawMessage(`{
		"path_scope": "legacy_suffixes",
		"provider": {
			"type": "qwen",
			"apiTokens": ["provider-path-scope-token"],
			"qwenDomain": "provider.example",
			"qwenEnableCompatible": true,
			"capabilities": {
				"openai/v1/responses": ""
			}
		}
	}`)
	boundTargetConfig := json.RawMessage(`{
		"path_scope": "bound_target",
		"provider": {
			"type": "qwen",
			"apiTokens": ["provider-path-scope-token"],
			"qwenDomain": "provider.example",
			"qwenEnableCompatible": true,
			"capabilities": {
				"openai/v1/responses": ""
			}
		}
	}`)
	boundTargetEmptyFiltersConfig := json.RawMessage(`{
		"path_scope": "bound_target",
		"enable_path_suffixes": [],
		"enable_methods": [],
		"provider": {
			"type": "qwen",
			"apiTokens": ["provider-path-scope-token"],
			"qwenDomain": "provider.example",
			"qwenEnableCompatible": true,
			"capabilities": {
				"openai/v1/responses": ""
			}
		}
	}`)
	boundTargetIntersectionConfig := json.RawMessage(`{
		"path_scope": "bound_target",
		"enable_path_suffixes": ["/vendor/jobs"],
		"enable_methods": ["PATCH"],
		"provider": {
			"type": "qwen",
			"apiTokens": ["provider-path-scope-token"],
			"qwenDomain": "provider.example",
			"qwenEnableCompatible": true,
			"capabilities": {
				"openai/v1/responses": ""
			}
		}
	}`)

	startRequest := func(
		t *testing.T,
		config json.RawMessage,
		method string,
		path string,
	) wasmhost.TestHost {
		t.Helper()

		host, status := wasmhost.NewTestHost(config)
		require.Equal(t, types.OnPluginStartStatusOK, status)
		t.Cleanup(host.Reset)
		host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "client.example"},
			{":path", path},
			{":method", method},
			{"authorization", "Bearer client-token"},
			{"content-type", "application/octet-stream"},
		})
		return host
	}

	requireProcessed := func(t *testing.T, host wasmhost.TestHost) {
		t.Helper()

		authority, hasAuthority := wasmhost.GetHeaderValue(
			host.GetRequestHeaders(),
			":authority",
		)
		require.True(t, hasAuthority)
		require.Equal(t, "provider.example", authority)

		authorization, hasAuthorization := wasmhost.GetHeaderValue(
			host.GetRequestHeaders(),
			"authorization",
		)
		require.True(t, hasAuthorization)
		require.Equal(t, "Bearer provider-path-scope-token", authorization)
	}

	assertSkipped := func(t *testing.T, host wasmhost.TestHost) {
		t.Helper()

		authority, hasAuthority := wasmhost.GetHeaderValue(
			host.GetRequestHeaders(),
			":authority",
		)
		require.True(t, hasAuthority)
		assert.Equal(t, "client.example", authority)

		authorization, hasAuthorization := wasmhost.GetHeaderValue(
			host.GetRequestHeaders(),
			"authorization",
		)
		require.True(t, hasAuthorization)
		assert.Equal(t, "Bearer client-token", authorization)
	}

	wasmhost.RunGoTest(t, func(t *testing.T) {
		t.Run("legacy_missing_and_explicit_replay_current_classification", func(t *testing.T) {
			configs := []struct {
				name   string
				config json.RawMessage
			}{
				{name: "omitted", config: legacyOmittedConfig},
				{name: "explicit", config: legacyExplicitConfig},
			}

			for _, testCase := range configs {
				t.Run(testCase.name, func(t *testing.T) {
					host := startRequest(t, testCase.config, "POST", "/v1/responses")
					requireProcessed(t, host)

					path, hasPath := wasmhost.GetHeaderValue(
						host.GetRequestHeaders(),
						":path",
					)
					require.True(t, hasPath)
					require.Equal(t, "/compatible-mode/v1/chat/completions", path)
				})
			}
		})

		t.Run("bound_target_without_advanced_filters_processes_provider_native_path", func(t *testing.T) {
			host := startRequest(t, boundTargetConfig, "PATCH", "/vendor/jobs/job-1")
			requireProcessed(t, host)

			path, hasPath := wasmhost.GetHeaderValue(host.GetRequestHeaders(), ":path")
			require.True(t, hasPath)
			require.Equal(t, "/vendor/jobs/job-1", path)

			body := []byte{0x00, 0xff, 0x7f, 0x01}
			host.CallOnHttpRequestBody(body)
			require.Equal(t, body, host.GetRequestBody())
		})

		t.Run("bound_target_empty_filters_do_not_narrow", func(t *testing.T) {
			host := startRequest(
				t,
				boundTargetEmptyFiltersConfig,
				"PATCH",
				"/vendor/jobs/job-1",
			)
			requireProcessed(t, host)

			path, hasPath := wasmhost.GetHeaderValue(host.GetRequestHeaders(), ":path")
			require.True(t, hasPath)
			require.Equal(t, "/vendor/jobs/job-1", path)

			body := []byte{0x00, 0xff, 0x7f, 0x02}
			host.CallOnHttpRequestBody(body)
			require.Equal(t, body, host.GetRequestBody())
		})

		t.Run("bound_target_nonempty_suffix_and_method_filters_intersect", func(t *testing.T) {
			cases := []struct {
				name      string
				method    string
				path      string
				processed bool
			}{
				{
					name:      "both_match",
					method:    "PATCH",
					path:      "/api/v1/vendor/jobs",
					processed: true,
				},
				{
					name:   "suffix_mismatch",
					method: "PATCH",
					path:   "/api/v1/vendor/other",
				},
				{
					name:   "method_mismatch",
					method: "POST",
					path:   "/api/v1/vendor/jobs",
				},
				{
					name:   "both_mismatch",
					method: "POST",
					path:   "/api/v1/vendor/other",
				},
			}

			for _, testCase := range cases {
				t.Run(testCase.name, func(t *testing.T) {
					host := startRequest(
						t,
						boundTargetIntersectionConfig,
						testCase.method,
						testCase.path,
					)
					if !testCase.processed {
						assertSkipped(t, host)
						return
					}

					requireProcessed(t, host)
					path, hasPath := wasmhost.GetHeaderValue(
						host.GetRequestHeaders(),
						":path",
					)
					require.True(t, hasPath)
					require.Equal(t, testCase.path, path)

					body := []byte{0x00, 0xff, 0x7f, 0x03}
					host.CallOnHttpRequestBody(body)
					require.Equal(t, body, host.GetRequestBody())
				})
			}
		})
	})
}
