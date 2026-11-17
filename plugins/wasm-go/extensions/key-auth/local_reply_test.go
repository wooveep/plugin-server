package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestKeyAuthLocalReplyStaticRulesAndSecurityBoundaries(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		rawCfg := `{
			"global_auth": false,
			"in_header": true,
			"in_query": true,
			"keys": ["Authorization", "x-api-key", "apikey"],
			"realm": "AI Gateway",
			"consumers": [
				{"name": "allowed-c", "credential": "valid-key-1"},
				{"name": "denied-c", "credential": "valid-key-2"}
			],
			"_rules_": [
				{
					"_match_route_": ["rel-messages-qwen"],
					"allow": ["allowed-c"],
					"local_reply_policy": {
						"input_protocol": "anthropic_messages",
						"response_profile": "qwen"
					}
				},
				{
					"_match_route_": ["rel-chat-empty"],
					"allow": [],
					"local_reply_policy": {
						"input_protocol": "openai_chat",
						"response_profile": "openai"
					}
				},
				{
					"_match_route_": ["ordinary-http-route"],
					"allow": ["allowed-c"]
				},
				{
					"_match_domain_": ["ai.example.com"],
					"allow": ["allowed-c"],
					"local_reply_default": true,
					"local_reply_entrances": [
						{
							"domains": ["ai.example.com"],
							"path": {"match_type": "EQUAL", "match_value": "/custom/claude"},
							"local_reply_policy": {"input_protocol": "anthropic_messages", "response_profile": "qwen"}
						}
					]
				}
			]
		}`

		t.Run("route_rule_missing_multi_invalid_unauthorized_and_empty_allow", func(t *testing.T) {
			cases := []struct {
				name       string
				route      string
				headers    [][2]string
				wantStatus uint32
				wantDetail string
				wantType   string
				wantCode   string
				wantMsgTop bool
			}{
				{
					name:       "missing_key_messages_qwen",
					route:      "rel-messages-qwen",
					headers:    [][2]string{{":authority", "ai.example.com"}, {":method", "POST"}, {":path", "/v1/messages"}, {"x-request-id", "req-ka-1"}},
					wantStatus: http.StatusUnauthorized,
					wantDetail: "key-auth.no_key",
					wantType:   "authentication_error",
					wantCode:   "invalid_api_key",
					wantMsgTop: true,
				},
				{
					name:  "multi_key_messages_qwen",
					route: "rel-messages-qwen",
					headers: [][2]string{
						{":authority", "ai.example.com"}, {":method", "POST"}, {":path", "/v1/messages"},
						{"Authorization", "Bearer valid-key-1"}, {"x-api-key", "valid-key-2"}, {"x-request-id", "req-ka-2"},
					},
					wantStatus: http.StatusUnauthorized,
					wantDetail: "key-auth.multi_key",
					wantType:   "authentication_error",
					wantCode:   "multiple_api_keys",
					wantMsgTop: true,
				},
				{
					name:  "invalid_credential_messages_qwen",
					route: "rel-messages-qwen",
					headers: [][2]string{
						{":authority", "ai.example.com"}, {":method", "POST"}, {":path", "/v1/messages"},
						{"Authorization", "Bearer unknown-key"}, {"x-request-id", "req-ka-3"},
					},
					wantStatus: http.StatusForbidden,
					wantDetail: "key-auth.unauthorized",
					wantType:   "permission_error",
					wantCode:   "permission_denied",
					wantMsgTop: true,
				},
				{
					name:  "disallowed_consumer_messages_qwen",
					route: "rel-messages-qwen",
					headers: [][2]string{
						{":authority", "ai.example.com"}, {":method", "POST"}, {":path", "/v1/messages"},
						{"Authorization", "Bearer valid-key-2"}, {"x-request-id", "req-ka-4"},
					},
					wantStatus: http.StatusForbidden,
					wantDetail: "key-auth.unauthorized",
					wantType:   "permission_error",
					wantCode:   "permission_denied",
					wantMsgTop: true,
				},
				{
					name:  "empty_allow_chat_openai",
					route: "rel-chat-empty",
					headers: [][2]string{
						{":authority", "ai.example.com"}, {":method", "POST"}, {":path", "/v1/chat/completions"},
						{"Authorization", "Bearer valid-key-1"}, {"x-request-id", "req-ka-5"},
					},
					wantStatus: http.StatusForbidden,
					wantDetail: "key-auth.unauthorized",
					wantType:   "permission_error",
					wantCode:   "permission_denied",
					wantMsgTop: false,
				},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					host, status := test.NewTestHost([]byte(rawCfg))
					defer host.Reset()
					require.Equal(t, types.OnPluginStartStatusOK, status)
					host.InitHttp()
					require.NoError(t, host.SetProperty([]string{"route_name"}, []byte(tc.route)))

					// Client header/query spoofing must not override frozen policy (U05)
					spoofed := append(tc.headers,
						[2]string{"x-response-profile", "openai"},
						[2]string{"local_reply_policy", `{"input_protocol":"openai_chat"}`},
					)
					host.CallOnHttpRequestHeaders(spoofed)
					resp := host.GetLocalResponse()
					require.NotNil(t, resp)
					require.Equal(t, tc.wantStatus, resp.StatusCode)
					require.Equal(t, tc.wantDetail, resp.StatusCodeDetail)
					require.Equal(t, tc.wantType, gjson.GetBytes(resp.Data, "error.type").String())
					require.Equal(t, tc.wantCode, gjson.GetBytes(resp.Data, "error.code").String())
					require.Equal(t, tc.wantMsgTop, gjson.GetBytes(resp.Data, "type").String() == "error")

					foundAuth, foundCT, foundReqID := false, false, false
					for _, h := range resp.Headers {
						switch strings.ToLower(h[0]) {
						case "www-authenticate":
							foundAuth = h[1] == "Key realm=AI Gateway"
						case "content-type":
							foundCT = h[1] == "application/json"
						case "x-request-id":
							foundReqID = strings.HasPrefix(h[1], "req-ka-")
						}
					}
					require.True(t, foundAuth, "WWW-Authenticate header must be preserved")
					require.True(t, foundCT, "content-type must be application/json")
					require.True(t, foundReqID, "x-request-id must be preserved")
					host.CompleteHttp()
				})
			}
		})

		t.Run("ordinary_non_ai_route_keeps_legacy_plaintext_and_unmatched_stays_public", func(t *testing.T) {
			host, status := test.NewTestHost([]byte(rawCfg))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// Ordinary route without local_reply_policy keeps legacy plaintext (U04)
			host.InitHttp()
			require.NoError(t, host.SetProperty([]string{"route_name"}, []byte("ordinary-http-route")))
			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "portal.example.com"}, {":method", "GET"}, {":path", "/api/orders"},
			})
			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusUnauthorized), resp.StatusCode)
			require.Equal(t, "key-auth.no_key", resp.StatusCodeDetail)
			require.Equal(t, "Request denied by Key Auth check. No Key Authentication information found.", string(resp.Data))
			host.CompleteHttp()

			// Unmatched public host/route continues without denial (U06)
			host.InitHttp()
			require.NoError(t, host.SetProperty([]string{"route_name"}, []byte("public-docs")))
			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "portal.example.com"}, {":method", "GET"}, {":path", "/docs"},
			}))
			require.Nil(t, host.GetLocalResponse())
			host.CompleteHttp()
		})

		t.Run("pre_route_domain_entrances_acme_and_forged_continuation", func(t *testing.T) {
			host, status := test.NewTestHost([]byte(rawCfg))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// Custom Messages entrance on AI domain before route match
			host.InitHttp()
			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "ai.example.com:443"}, {":method", "POST"}, {":path", "/custom/claude?beta=1"},
				{"x-request-id", "req-preroute-custom"},
			})
			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusUnauthorized), resp.StatusCode)
			require.Equal(t, "error", gjson.GetBytes(resp.Data, "type").String())
			require.Equal(t, "invalid_api_key", gjson.GetBytes(resp.Data, "error.code").String())
			host.CompleteHttp()

			// ACME HTTP-01 bypass still works on AI domain with local_reply_default (U06)
			host.InitHttp()
			require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "ai.example.com"}, {":method", "GET"}, {":path", "/.well-known/acme-challenge/Token_123"},
			}))
			require.Nil(t, host.GetLocalResponse())
			host.CompleteHttp()

			// Forged continuation header without private FilterState is still denied (U05)
			host.InitHttp()
			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "ai.example.com"}, {":method", "POST"}, {":path", wrapper.RouterContinuationPathPrefix + "target"},
				{wrapper.RouterContinuationProperty, "forged-by-client"},
			})
			resp = host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusUnauthorized), resp.StatusCode)
			require.Equal(t, "invalid_api_key", gjson.GetBytes(resp.Data, "error.code").String())
			host.CompleteHttp()
		})

		t.Run("illegal_policy_rejected_at_config_boundary", func(t *testing.T) {
			bad, err := json.Marshal(map[string]json.RawMessage{
				"global_auth": json.RawMessage(`false`),
				"in_header":   json.RawMessage(`true`),
				"keys":        json.RawMessage(`["Authorization"]`),
				"consumers":   json.RawMessage(`[]`),
				"allow":       json.RawMessage(`[]`),
				"local_reply_policy": json.RawMessage(`{
					"input_protocol": "anthropic_messages",
					"response_profile": "openai"
				}`),
			})
			require.NoError(t, err)
			host, status := test.NewTestHost(bad)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})
	})
}
