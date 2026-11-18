package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/iface"
	wasmtest "github.com/higress-group/wasm-go/pkg/test"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/stretchr/testify/require"
)

type billingCommittedCtx struct {
	wrapper.HttpContext
}

func (billingCommittedCtx) GetExecutionPhase() iface.HTTPExecutionPhase { return iface.EncodeHeader }
func (billingCommittedCtx) GetStringContext(string, string) string      { return "req-billing-committed" }
func (billingCommittedCtx) GetContext(string) interface{}               { return nil }

func saturateBillingOutbox(t *testing.T, cfg BillingConfig) {
	t.Helper()
	configured := cfg.NativeExecution.(*nativeBillingConfig)
	base := nativeBillingDeliveryFixture(t, cfg).Envelope()
	for i := 0; i < 4096; i++ {
		env := base
		env.EventID = fmt.Sprintf("70000000-0000-4000-8000-%012d", i+1)
		env.CallID = env.EventID
		env.PayloadDigest = ""
		snap, err := executionfacts.Freeze(env)
		require.NoError(t, err)
		_, err = configured.outbox.Submit(snap, func([]byte, func(executionfacts.AppendResult)) error { return nil }, nil)
		require.NoError(t, err)
	}
	require.False(t, configured.outbox.Accepting())
}

func TestBillingLocalReplyConfigValidationAndRuleIsolationU02U04(t *testing.T) {
	wasmtest.RunGoTest(t, func(t *testing.T) {
		_, ruleFixture := nativeBillingConfigFixture(t)
		ruleFixture["_match_route_"] = []string{"rel-billing-ai"}

		t.Run("invalid local_reply_policy rejected on global and rule", func(t *testing.T) {
			badGlobal, err := json.Marshal(map[string]any{
				"redis_stream": map[string]any{"service_name": "redis.static", "service_port": 6379, "stream": "billing:events"},
				"local_reply_policy": map[string]any{
					"input_protocol":   "anthropic_messages",
					"response_profile": "openai",
				},
			})
			require.NoError(t, err)
			host, status := wasmtest.NewTestHost(badGlobal)
			host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status)

			badRule := make(map[string]any, len(ruleFixture)+1)
			for k, v := range ruleFixture {
				badRule[k] = v
			}
			badRule["local_reply_policy"] = map[string]any{"response_profile": "deepseek"}
			badRuleDoc, err := json.Marshal(map[string]any{
				"redis_stream": map[string]any{"service_name": "redis.static", "service_port": 6379, "stream": "billing:events"},
				"_rules_":      []any{badRule},
			})
			require.NoError(t, err)
			ruleHost, ruleStatus := wasmtest.NewTestHost(badRuleDoc)
			ruleHost.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, ruleStatus)
		})

		t.Run("rule without local_reply_policy does not inherit global policy", func(t *testing.T) {
			doc, err := json.Marshal(map[string]any{
				"redis_stream": map[string]any{"service_name": "redis.static", "service_port": 6379, "stream": "billing:events"},
				"local_reply_policy": map[string]any{
					"input_protocol":   "openai_chat",
					"response_profile": "deepseek",
				},
				"_rules_": []any{ruleFixture},
			})
			require.NoError(t, err)
			host, status := wasmtest.NewTestHost(doc)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			host.SetRouteName("rel-billing-ai")
			matched, err := host.GetMatchConfig()
			require.NoError(t, err)
			require.Nil(t, matched.(*BillingConfig).LocalReplyPolicy)
		})

		t.Run("committed response phase logs critical without legacy double send ST01", func(t *testing.T) {
			doc, err := json.Marshal(map[string]any{
				"redis_stream": map[string]any{"service_name": "redis.static", "service_port": 6379, "stream": "billing:events"},
				"_rules_":      []any{ruleFixture},
			})
			require.NoError(t, err)
			host, status := wasmtest.NewTestHost(doc)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			host.InitHttp()

			policy, err := clientresponse.ParseLocalReplyPolicy(clientresponse.Messages, "qwen")
			require.NoError(t, err)
			require.Equal(t, types.ActionPause, rejectNativeBillingUnavailable(billingCommittedCtx{}, &policy, []byte(`{"error":"legacy"}`)))
			require.Nil(t, host.GetLocalResponse(), "must not send legacy fallback when SendLocalReply fails on committed response")
			joined := strings.Join(host.GetCriticalLogs(), "\n")
			require.Contains(t, joined, "ai-billing failed to send local reply (status=503, detail=modelfusion.local.accounting_unavailable)")
			require.Contains(t, joined, clientresponse.ErrLocalReplyAfterResponseCommitted.Error())
		})
	})
}

func TestBillingLocalReplyAccountingUnavailableU03U04(t *testing.T) {
	wasmtest.RunGoTest(t, func(t *testing.T) {
		_, ruleBase := nativeBillingConfigFixture(t)

		t.Run("saturated outbox encodes anthropic service_unavailable when policy configured", func(t *testing.T) {
			ruleWithPolicy := make(map[string]any, len(ruleBase)+2)
			for k, v := range ruleBase {
				ruleWithPolicy[k] = v
			}
			ruleWithPolicy["_match_route_"] = []string{"rel-billing-policy"}
			ruleWithPolicy["local_reply_policy"] = map[string]any{
				"input_protocol":   "anthropic_messages",
				"response_profile": "qwen",
			}
			doc, err := json.Marshal(map[string]any{
				"redis_stream": map[string]any{"service_name": "redis.static", "service_port": 6379, "stream": "billing:events"},
				"_rules_":      []any{ruleWithPolicy},
			})
			require.NoError(t, err)

			host, status := wasmtest.NewTestHostWithForeignFuncs(doc, map[string]func([]byte) []byte{
				"set_property": func([]byte) []byte { return nil },
			})
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			host.SetRouteName("rel-billing-policy")
			matched, err := host.GetMatchConfig()
			require.NoError(t, err)
			saturateBillingOutbox(t, *matched.(*BillingConfig))

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "api.example.com"},
				{":path", "/v1/messages"},
				{":method", http.MethodPost},
				{"x-request-id", "req-billing-503"},
			})
			require.Equal(t, types.ActionPause, action)

			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusServiceUnavailable), resp.StatusCode)
			require.Equal(t, "modelfusion.local.accounting_unavailable", resp.StatusCodeDetail)
			require.Contains(t, resp.Headers, [2]string{"x-request-id", "req-billing-503"})
			require.JSONEq(t, `{"type":"error","error":{"type":"api_error","code":"service_unavailable","message":"The service is temporarily unavailable."}}`, string(resp.Data))
		})

		t.Run("saturated outbox preserves legacy accounting_delivery_unavailable without policy", func(t *testing.T) {
			ruleLegacy := make(map[string]any, len(ruleBase)+1)
			for k, v := range ruleBase {
				ruleLegacy[k] = v
			}
			ruleLegacy["_match_route_"] = []string{"rel-billing-legacy"}
			doc, err := json.Marshal(map[string]any{
				"redis_stream": map[string]any{"service_name": "redis.static", "service_port": 6379, "stream": "billing:events"},
				"_rules_":      []any{ruleLegacy},
			})
			require.NoError(t, err)

			host, status := wasmtest.NewTestHostWithForeignFuncs(doc, map[string]func([]byte) []byte{
				"set_property": func([]byte) []byte { return nil },
			})
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			host.SetRouteName("rel-billing-legacy")
			matched, err := host.GetMatchConfig()
			require.NoError(t, err)
			saturateBillingOutbox(t, *matched.(*BillingConfig))

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "api.example.com"},
				{":path", "/v1/chat/completions"},
				{":method", http.MethodPost},
			})
			require.Equal(t, types.ActionPause, action)

			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusServiceUnavailable), resp.StatusCode)
			require.Equal(t, "modelfusion.local.accounting_unavailable", resp.StatusCodeDetail)
			require.JSONEq(t, `{"error":{"code":"accounting_delivery_unavailable","message":"Accounting temporarily unavailable"}}`, string(resp.Data))
		})
	})
}
