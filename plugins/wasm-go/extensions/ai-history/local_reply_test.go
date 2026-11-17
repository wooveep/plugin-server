package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

func TestHistoryLocalReplyConfigValidationU02(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		base := historyCaptureConfig("anthropic_messages")
		for idx, policy := range []any{
			map[string]any{"input_protocol": "anthropic_messages", "response_profile": "openai"},
			map[string]any{"input_protocol": "openai_chat", "response_profile": "deepseek"},
			map[string]any{"input_protocol": "anthropic_messages"},
		} {
			raw, err := sjson.Set(base, "local_reply_policy", policy)
			require.NoError(t, err)
			host, status := test.NewTestHost([]byte(raw))
			host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status, "invalid policy %d", idx)
		}
	})
}

func TestHistoryLocalReplyBodyBasisRejectionsAndLegacyFallbackU03U04(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		rawCfg, err := sjson.Set(historyCaptureConfig("anthropic_messages"), "local_reply_policy", map[string]any{
			"input_protocol":   "anthropic_messages",
			"response_profile": "qwen",
		})
		require.NoError(t, err)

		t.Run("non-JSON content-type keeps 415 and encodes anthropic invalid_parameter", func(t *testing.T) {
			host, status := test.NewTestHost([]byte(rawCfg))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.Equal(t, types.ActionPause, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "api.example.com"},
				{":path", "/v1/messages"},
				{":method", http.MethodPost},
				{"content-type", "text/plain"},
				{"x-request-id", "req-history-415"},
			}))
			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusUnsupportedMediaType), resp.StatusCode)
			require.Equal(t, "modelfusion.local.ai-history.json_required", resp.StatusCodeDetail)
			require.Contains(t, resp.Headers, [2]string{"x-request-id", "req-history-415"})
			require.JSONEq(t, `{"type":"error","error":{"type":"invalid_request_error","code":"invalid_parameter","message":"The request body or model is invalid."}}`, string(resp.Data))
		})

		t.Run("invalid JSON body keeps 400 and encodes anthropic invalid_parameter", func(t *testing.T) {
			host, status := test.NewTestHost([]byte(rawCfg))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			headers := historyTestHeaders(t, []byte(`{"model":"public-model"}`), historyTestCaller())
			require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(headers))
			require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody([]byte(`{"messages":[]}`)))

			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusBadRequest), resp.StatusCode)
			require.Equal(t, "modelfusion.local.ai-history.invalid_model_body", resp.StatusCodeDetail)
			require.JSONEq(t, `{"type":"error","error":{"type":"invalid_request_error","code":"invalid_parameter","message":"The request body or model is invalid."}}`, string(resp.Data))
		})

		t.Run("legacy config without local_reply_policy preserves history_body_basis_error", func(t *testing.T) {
			host, status := test.NewTestHost(json.RawMessage(historyCaptureConfig("openai_chat")))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.Equal(t, types.ActionPause, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "api.example.com"},
				{":path", "/v1/chat/completions"},
				{":method", http.MethodPost},
				{"content-type", "text/plain"},
			}))
			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusUnsupportedMediaType), resp.StatusCode)
			require.Equal(t, "modelfusion.local.ai-history.json_required", resp.StatusCodeDetail)
			require.JSONEq(t, `{"error":{"code":"json_required","type":"history_body_basis_error"}}`, string(resp.Data))
		})
	})
}
