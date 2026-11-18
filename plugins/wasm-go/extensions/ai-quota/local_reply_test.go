package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/iface"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/stretchr/testify/require"
)

type quotaCommittedCtx struct {
	wrapper.HttpContext
}

func (quotaCommittedCtx) GetExecutionPhase() iface.HTTPExecutionPhase { return iface.EncodeHeader }
func (quotaCommittedCtx) GetStringContext(string, string) string      { return "req-quota-committed" }
func (quotaCommittedCtx) GetContext(string) interface{}               { return nil }
func (quotaCommittedCtx) SetUserAttribute(string, interface{})        {}

func TestQuotaLocalReplyConfigValidationU02(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		invalidPolicies := []any{
			map[string]any{"input_protocol": "anthropic_messages", "response_profile": "openai"},
			map[string]any{"response_profile": "deepseek"},
			map[string]any{"input_protocol": "openai_chat", "response_profile": nil},
			map[string]any{"input_protocol": "openai_chat", "response_profile": "deepseek", "extra": true},
			"invalid",
		}
		for idx, bad := range invalidPolicies {
			host, status := test.NewTestHost(quotaConfigWith(map[string]any{
				"local_reply_policy": bad,
			}))
			host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status, "global invalid case %d", idx)

			ruleHost, ruleStatus := test.NewTestHost(quotaConfigWith(map[string]any{
				"_rules_": []map[string]any{
					{
						"_match_route_":      []string{"rel-bad"},
						"local_reply_policy": bad,
					},
				},
			}))
			ruleHost.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, ruleStatus, "rule invalid case %d", idx)
		}

		host, status := test.NewTestHost(quotaConfigWith(map[string]any{
			"local_reply_policy": map[string]any{"input_protocol": "openai_chat"},
		}))
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)
		host.InitHttp()

		policy, err := clientresponse.ParseLocalReplyPolicy(clientresponse.Chat, "deepseek")
		require.NoError(t, err)
		sendQuotaResponse(quotaCommittedCtx{}, &policy, http.StatusForbidden, "ai-quota.no_balance", "legacy body")
		executionAdmissionReject(quotaCommittedCtx{}, &policy, http.StatusForbidden, "check_denied")
		require.Nil(t, host.GetLocalResponse(), "must not send legacy fallback when SendLocalReply fails on committed response")
		joined := strings.Join(host.GetCriticalLogs(), "\n")
		require.Contains(t, joined, "ai-quota failed to send local reply (status=403, detail=ai-quota.no_balance)")
		require.Contains(t, joined, "ai-quota failed to send execution admission local reply (status=403, detail=execution_admission_check_denied)")
		require.Contains(t, joined, clientresponse.ErrLocalReplyAfterResponseCommitted.Error())
	})
}

func TestQuotaLocalReplyBalanceAndAdmissionReasonsU03U05(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("missing identity uses anthropic envelope and ignores forged headers", func(t *testing.T) {
			host, status := test.NewTestHost(quotaConfigWith(map[string]any{
				"local_reply_policy": map[string]any{
					"input_protocol":   "anthropic_messages",
					"response_profile": "qwen",
				},
			}))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "api.example.com"},
				{":path", "/v1/messages"},
				{":method", http.MethodPost},
				{"x-request-id", "req-quota-missing-id"},
				{"x-modelfusion-response-profile", "openai"},
			})
			require.Equal(t, types.ActionContinue, action)
			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusForbidden), resp.StatusCode)
			require.Equal(t, "ai-quota.missing_identity", resp.StatusCodeDetail)
			require.Contains(t, resp.Headers, [2]string{"content-type", "application/json"})
			require.Contains(t, resp.Headers, [2]string{"x-request-id", "req-quota-missing-id"})
			require.JSONEq(t, `{"type":"error","error":{"type":"permission_error","code":"permission_denied","message":"The account is not permitted to use this service."}}`, string(resp.Data))
		})

		t.Run("insufficient balance keeps 403 and encodes insufficient_balance", func(t *testing.T) {
			host, status := test.NewTestHost(quotaConfigWith(map[string]any{
				"local_reply_policy": map[string]any{
					"input_protocol":   "openai_chat",
					"response_profile": "openai",
				},
			}))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			headers := append(nativeAdmissionHeaders("/v1/chat/completions", http.MethodPost), [2]string{"x-request-id", "req-quota-nobal"})
			require.Equal(t, types.HeaderStopAllIterationAndWatermark, host.CallOnHttpRequestHeaders(headers))
			host.CallOnRedisCall(0, admissionReply("insufficient_balance"))

			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusForbidden), resp.StatusCode)
			require.Equal(t, "ai-quota.no_balance", resp.StatusCodeDetail)
			require.Contains(t, resp.Headers, [2]string{"x-request-id", "req-quota-nobal"})
			require.JSONEq(t, `{"error":{"type":"insufficient_quota","code":"insufficient_balance","message":"The account has insufficient balance."}}`, string(resp.Data))
		})

		t.Run("projection unready keeps 503 and encodes service_unavailable", func(t *testing.T) {
			host, status := test.NewTestHost(quotaConfigWith(map[string]any{
				"local_reply_policy": map[string]any{
					"input_protocol":   "openai_responses",
					"response_profile": "qwen",
				},
			}))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			headers := append(nativeAdmissionHeaders("/v1/chat/completions", http.MethodPost), [2]string{"x-request-id", "req-quota-unready"})
			require.Equal(t, types.HeaderStopAllIterationAndWatermark, host.CallOnHttpRequestHeaders(headers))
			host.CallOnRedisCall(0, admissionReply("projection_unready"))

			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusServiceUnavailable), resp.StatusCode)
			require.Equal(t, "ai-quota.projection_unready", resp.StatusCodeDetail)
			require.JSONEq(t, `{"error":{"type":"api_error","code":"service_unavailable","message":"The service is temporarily unavailable."}}`, string(resp.Data))
		})

		t.Run("execution admission validation check_denied encodes permission_denied", func(t *testing.T) {
			fixture := executionAdmissionFixture("validation")
			cfgBytes := quotaConfigWith(map[string]any{
				"path_scope":           PathScopeBoundTarget,
				"enable_path_suffixes": []string{},
				"execution_admission":  fixture["execution_admission"],
				"local_reply_policy": map[string]any{
					"input_protocol":   "anthropic_messages",
					"response_profile": "deepseek",
				},
			})
			var cleanMap map[string]any
			require.NoError(t, json.Unmarshal(cfgBytes, &cleanMap))
			delete(cleanMap, "enable_path_suffixes")
			rawCfg, err := json.Marshal(cleanMap)
			require.NoError(t, err)

			host, status := test.NewTestHost(rawCfg)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			reqHeaders := append([][2]string{
				{":authority", "api.example.com"},
				{":path", "/v1/messages"},
				{":method", http.MethodPost},
				{"x-request-id", "req-exec-denied"},
			}, executionAdmissionCaller.Headers()...)
			require.Equal(t, types.HeaderStopAllIterationAndWatermark, host.CallOnHttpRequestHeaders(reqHeaders))
			host.CallOnRedisCall(0, admissionReply("ok"))
			host.CallOnHttpCall([][2]string{{":status", "403"}, {"content-type", "application/json"}}, []byte(`{"error":"denied"}`))

			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusForbidden), resp.StatusCode)
			require.Equal(t, "execution_admission_check_denied", resp.StatusCodeDetail)
			require.Contains(t, resp.Headers, [2]string{"x-request-id", "req-exec-denied"})
			require.JSONEq(t, `{"type":"error","error":{"type":"permission_error","code":"permission_denied","message":"The account is not permitted to use this service."}}`, string(resp.Data))
		})
	})
}

func TestQuotaLocalReplyLegacyAndRuleIsolationU04U08(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		cfg := quotaConfigWith(map[string]any{
			"local_reply_policy": map[string]any{
				"input_protocol":   "openai_chat",
				"response_profile": "deepseek",
			},
			"_rules_": []map[string]any{
				{
					"_match_route_": []string{"ai-messages-route"},
					"path_scope":    PathScopeBoundTarget,
					"local_reply_policy": map[string]any{
						"input_protocol":   "anthropic_messages",
						"response_profile": "qwen",
					},
				},
				{
					"_match_route_": []string{"legacy-route"},
					"path_scope":    PathScopeBoundTarget,
				},
			},
		})

		t.Run("matched AI route uses its own anthropic policy", func(t *testing.T) {
			host, status := test.NewTestHost(cfg)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			host.SetRouteName("ai-messages-route")

			require.Equal(t, types.HeaderStopAllIterationAndWatermark, host.CallOnHttpRequestHeaders(nativeAdmissionHeaders("/v1/messages", http.MethodPost)))
			host.CallOnRedisCall(0, admissionReply("insufficient_balance"))
			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusForbidden), resp.StatusCode)
			require.True(t, strings.Contains(string(resp.Data), `"type":"error"`))
		})

		t.Run("sibling legacy route does not inherit global policy", func(t *testing.T) {
			host, status := test.NewTestHost(cfg)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			host.SetRouteName("legacy-route")

			require.Equal(t, types.HeaderStopAllIterationAndWatermark, host.CallOnHttpRequestHeaders(nativeAdmissionHeaders("/v1/chat/completions", http.MethodPost)))
			host.CallOnRedisCall(0, admissionReply("insufficient_balance"))
			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusForbidden), resp.StatusCode)
			require.Equal(t, "ai-quota.no_balance", resp.StatusCodeDetail)
			require.Equal(t, "Request denied by ai quota check. No monetary balance left.", string(resp.Data))
		})

		t.Run("allowed request streams response untouched", func(t *testing.T) {
			host, status := test.NewTestHost(cfg)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			host.SetRouteName("ai-messages-route")

			require.Equal(t, types.HeaderStopAllIterationAndWatermark, host.CallOnHttpRequestHeaders(nativeAdmissionHeaders("/v1/messages", http.MethodPost)))
			host.CallOnRedisCall(0, admissionReply("ok"))
			require.Nil(t, host.GetLocalResponse())
			chunk := []byte("event: message_start\ndata: {}\n\n")
			require.Equal(t, types.ActionContinue, host.CallOnHttpStreamingResponseBody(chunk, true))
			require.Nil(t, host.GetLocalResponse())
		})
	})
}
