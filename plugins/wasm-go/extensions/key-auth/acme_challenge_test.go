package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/serviceidentity"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func domainScopedKeyAuthConfig(t *testing.T, allow []string) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"global_auth": false,
		"in_header":   true,
		"in_query":    false,
		"keys":        []string{"apikey", "x-api-key", "Authorization"},
		"consumers": []map[string]any{
			{
				"name":       "consumer1",
				"credential": "token1",
			},
		},
		"_rules_": []map[string]any{
			{
				"_match_domain_": []string{"api.test.ocloudmodel.com"},
				"allow":          allow,
			},
		},
	})
	require.NoError(t, err)
	return raw
}

func TestAcmeHTTP01ChallengeBypassAndHeaderScrubbing(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		configs := []struct {
			name string
			cfg  json.RawMessage
		}{
			{"domain_rule_with_consumers", domainScopedKeyAuthConfig(t, []string{"consumer1"})},
			{"domain_rule_empty_allow", domainScopedKeyAuthConfig(t, []string{})},
			{"global_auth_true", basicKeyAuthConfig},
			{"native_balance_bindings", nativeKeyAuthFixture(true)},
		}

		for _, cfg := range configs {
			t.Run(cfg.name, func(t *testing.T) {
				allowedCases := []struct {
					name, method, path string
				}{
					{"get_rfc8555_token", http.MethodGet, "/.well-known/acme-challenge/LoqXcYV8q5ONbJQxbmR7SCTNo3tiAXDfowyjAjEuX3s"},
					{"head_token", http.MethodHead, "/.well-known/acme-challenge/test-probe_1"},
					{"get_token_with_query", http.MethodGet, "/.well-known/acme-challenge/test-probe?check=1"},
				}
				for _, tc := range allowedCases {
					t.Run(tc.name, func(t *testing.T) {
						host, status := test.NewTestHost(cfg.cfg)
						defer host.Reset()
						require.Equal(t, types.OnPluginStartStatusOK, status)
						host.InitHttp()
						require.NoError(t, serviceidentity.Publish(internalServiceIdentity()))

						headers := append([][2]string{
							{":authority", "api.test.ocloudmodel.com"},
							{":method", tc.method},
							{":path", tc.path},
							{consumerHeaderName, "forged-consumer"},
							{tenantHeaderName, "forged-tenant"},
							{serviceidentity.ServiceHeader, "ai-cache"},
						}, firstBalanceIdentity.Headers()...)
						require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders(headers))
						require.Nil(t, host.GetLocalResponse())

						_, err := balanceidentity.Read(host.GetRequestHeaders())
						require.Error(t, err)
						_, err = serviceidentity.Read(host.GetRequestHeaders())
						require.Error(t, err)
						for _, pair := range host.GetRequestHeaders() {
							require.False(t, strings.EqualFold(pair[0], consumerHeaderName))
							require.False(t, strings.EqualFold(pair[0], tenantHeaderName))
							require.False(t, strings.EqualFold(pair[0], balanceidentity.AccountHeader))
							require.False(t, strings.EqualFold(pair[0], serviceidentity.ServiceHeader))
						}
						host.CompleteHttp()
					})
				}

				deniedCases := []struct {
					name, method, path string
				}{
					{"post_challenge_path", http.MethodPost, "/.well-known/acme-challenge/valid-token"},
					{"empty_token", http.MethodGet, "/.well-known/acme-challenge/"},
					{"missing_slash", http.MethodGet, "/.well-known/acme-challenge"},
					{"nested_subpath", http.MethodGet, "/.well-known/acme-challenge/a/b"},
					{"dotdot_segment", http.MethodGet, "/.well-known/acme-challenge/.."},
					{"dot_traversal", http.MethodGet, "/.well-known/acme-challenge/a/../b"},
					{"percent_encoded", http.MethodGet, "/.well-known/acme-challenge/%2e%2e"},
					{"fragment_in_path", http.MethodGet, "/.well-known/acme-challenge/valid#frag"},
					{"oversized_token", http.MethodGet, "/.well-known/acme-challenge/" + strings.Repeat("a", maxAcmeTokenLength+1)},
					{"other_well_known", http.MethodGet, "/.well-known/openid-configuration"},
					{"api_route", http.MethodPost, "/v1/chat/completions"},
				}
				for _, tc := range deniedCases {
					t.Run(tc.name, func(t *testing.T) {
						host, status := test.NewTestHost(cfg.cfg)
						defer host.Reset()
						require.Equal(t, types.OnPluginStartStatusOK, status)
						host.InitHttp()

						host.CallOnHttpRequestHeaders([][2]string{
							{":authority", "api.test.ocloudmodel.com"},
							{":method", tc.method},
							{":path", tc.path},
						})
						resp := host.GetLocalResponse()
						require.NotNil(t, resp)
						require.Equal(t, uint32(http.StatusUnauthorized), resp.StatusCode)
						host.CompleteHttp()
					})
				}
			})
		}
	})
}
