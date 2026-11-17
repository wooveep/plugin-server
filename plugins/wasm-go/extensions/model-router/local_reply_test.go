package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestModelRouterLocalReplyConfigValidationU02(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		invalidConfigs := []map[string]any{
			{
				"modelToHeader": "x-higress-llm-model",
				"local_reply_policy": map[string]any{
					"input_protocol":   "anthropic_messages",
					"response_profile": "openai",
				},
			},
			{
				"modelToHeader":       "x-higress-llm-model",
				"local_reply_default": true,
				"local_reply_policy": map[string]any{
					"input_protocol":   "openai_chat",
					"response_profile": "deepseek",
				},
			},
			{
				"modelToHeader": "x-higress-llm-model",
				"local_reply_entrances": []map[string]any{
					{
						"path": map[string]any{
							"match_type":  "REGULAR",
							"match_value": "[unterminated",
						},
						"local_reply_policy": map[string]any{
							"input_protocol":   "openai_chat",
							"response_profile": "deepseek",
						},
					},
				},
			},
		}
		for idx, rawMap := range invalidConfigs {
			raw, err := json.Marshal(rawMap)
			require.NoError(t, err)
			host, status := test.NewTestHost(raw)
			host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status, "invalid config %d", idx)
		}
	})
}

func TestModelRouterLocalReplyDuplicateModelAndCustomEntrancesU10(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		cfg, err := json.Marshal(map[string]any{
			"modelKey":           "model",
			"modelToHeader":      "x-higress-llm-model",
			"enableOnPathSuffix": []string{"*"},
			"_rules_": []map[string]any{
				{
					"_match_domain_":      []string{"api.ai.example.com"},
					"modelKey":            "model",
					"modelToHeader":       "x-higress-llm-model",
					"enableOnPathSuffix":  []string{"*"},
					"local_reply_default": true,
					"local_reply_entrances": []map[string]any{
						{
							"domains": []string{"api.ai.example.com"},
							"path": map[string]any{
								"match_type":  "PRE",
								"match_value": "/custom/claude",
							},
							"local_reply_policy": map[string]any{
								"input_protocol":   "anthropic_messages",
								"response_profile": "qwen",
							},
						},
					},
				},
			},
		})
		require.NoError(t, err)
		dupBody := []byte(`{"model":"qwen-plus","model":"deepseek-chat","messages":[{"role":"user","content":"hi"}]}`)

		t.Run("custom entrance encodes anthropic envelope before ai-proxy", func(t *testing.T) {
			host, status := test.NewTestHost(cfg)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "api.ai.example.com:8080"},
				{":path", "/custom/claude/entry?beta=1"},
				{":method", http.MethodPost},
				{"content-type", "application/json"},
				{"x-request-id", "req-router-custom"},
			}))
			require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(dupBody))
			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusBadRequest), resp.StatusCode)
			require.Equal(t, "duplicate_model_member", resp.StatusCodeDetail)
			require.Contains(t, resp.Headers, [2]string{"content-type", "application/json"})
			require.Contains(t, resp.Headers, [2]string{"x-request-id", "req-router-custom"})
			require.JSONEq(t, `{"type":"error","error":{"type":"invalid_request_error","code":"invalid_parameter","message":"The request contains duplicate model fields."}}`, string(resp.Data))
		})

		t.Run("default suffix rule encodes openai_chat on non-messages path", func(t *testing.T) {
			host, status := test.NewTestHost(cfg)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "api.ai.example.com"},
				{":path", "/v1/chat/completions"},
				{":method", http.MethodPost},
				{"content-type", "application/json"},
			}))
			require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(dupBody))
			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusBadRequest), resp.StatusCode)
			require.Equal(t, "duplicate_model_member", resp.StatusCodeDetail)
			require.JSONEq(t, `{"error":{"type":"invalid_request_error","code":"invalid_parameter","message":"The request contains duplicate model fields."}}`, string(resp.Data))
		})

		t.Run("unmanaged domain preserves legacy text/plain duplicate rejection", func(t *testing.T) {
			host, status := test.NewTestHost(cfg)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "portal.example.com"},
				{":path", "/v1/messages"},
				{":method", http.MethodPost},
				{"content-type", "application/json"},
			}))
			require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(dupBody))
			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusBadRequest), resp.StatusCode)
			require.Equal(t, "duplicate_model_member", resp.StatusCodeDetail)
			require.Contains(t, resp.Headers, [2]string{"content-type", "text/plain"})
		})
	})
}
