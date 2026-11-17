package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/memorycache"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

func TestNativeCacheLocalReplyConfigValidationU02U08(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		t.Run("invalid local_reply_policy rejected", func(t *testing.T) {
			raw, err := sjson.SetBytes(nativeCacheExecutionConfig(t, true), "local_reply_policy", map[string]any{
				"input_protocol":   "anthropic_messages",
				"response_profile": "openai",
			})
			require.NoError(t, err)
			host, status := test.NewTestHost(raw)
			host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})

		t.Run("passthrough rejects top-level response_profile", func(t *testing.T) {
			raw, err := sjson.SetBytes(nativeCacheExecutionConfig(t, true), "protocol_mode", "passthrough")
			require.NoError(t, err)
			raw, err = sjson.SetBytes(raw, "response_profile", "deepseek")
			require.NoError(t, err)
			host, status := test.NewTestHost(raw)
			host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})

		t.Run("passthrough accepts omitted-profile local_reply_policy", func(t *testing.T) {
			raw, err := sjson.SetBytes(nativeCacheExecutionConfig(t, true), "protocol_mode", "passthrough")
			require.NoError(t, err)
			raw, err = sjson.SetBytes(raw, "local_reply_policy", map[string]any{
				"input_protocol": "openai_chat",
			})
			require.NoError(t, err)
			host, status := test.NewTestHost(raw)
			host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
		})

		t.Run("mismatched client_protocol and local_reply_policy rejected", func(t *testing.T) {
			raw, err := sjson.SetBytes(nativeCacheExecutionConfig(t, true), "local_reply_policy", map[string]any{
				"input_protocol":   "anthropic_messages",
				"response_profile": "qwen",
			})
			require.NoError(t, err)
			host, status := test.NewTestHost(raw)
			host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})
	})
}

func TestNativeCacheLocalReplyRejectionsCacheHitAndLegacyU03U04U08(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		withPolicy, err := sjson.SetBytes(nativeCacheExecutionConfig(t, true), "local_reply_policy", map[string]any{
			"input_protocol":   "openai_chat",
			"response_profile": "openai",
		})
		require.NoError(t, err)

		t.Run("missing caller identity keeps 403 and encodes permission_denied", func(t *testing.T) {
			host, status := test.NewTestHost(withPolicy)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.Equal(t, types.ActionPause, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "api.example.com"},
				{":path", "/native/custom-answer"},
				{":method", http.MethodPost},
				{"content-type", "application/json"},
				{"x-request-id", "req-cache-403"},
			}))
			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusForbidden), resp.StatusCode)
			require.Equal(t, "ai-cache.execution_delivery.authenticated_caller_missing", resp.StatusCodeDetail)
			require.Contains(t, resp.Headers, [2]string{"x-request-id", "req-cache-403"})
			require.JSONEq(t, `{"error":{"type":"permission_error","code":"permission_denied","message":"The account is not permitted to use this service."}}`, string(resp.Data))
		})

		t.Run("invalid body keeps 400 and encodes invalid_parameter", func(t *testing.T) {
			host, status := test.NewTestHost(withPolicy)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			headers := append(nativeCacheExecutionHeaders(true), [2]string{"x-request-id", "req-cache-400"})
			require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(headers))
			require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody([]byte(`{"model":"wrong-model","messages":[{"role":"user","content":"hi"}]}`)))

			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusBadRequest), resp.StatusCode)
			require.Equal(t, "ai-cache.execution_delivery.original_body_invalid", resp.StatusCodeDetail)
			require.JSONEq(t, `{"error":{"type":"invalid_request_error","code":"invalid_parameter","message":"The request body or model is invalid."}}`, string(resp.Data))
		})

		t.Run("cache hit replay is never rewritten as a local error", func(t *testing.T) {
			host, status := test.NewTestHost(withPolicy)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			reqBody := []byte(nativeCacheTestBody)
			require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(nativeCacheExecutionHeaders(true)))
			require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(reqBody))
			replyNativeRegisteredCacheLookup(t, host, reqBody, false)

			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusOK), resp.StatusCode)
			require.Equal(t, memorycache.CacheHitResponseDetail, resp.StatusCodeDetail)
			var parsed map[string]any
			require.NoError(t, json.Unmarshal(resp.Data, &parsed))
			require.Equal(t, "chat.completion", parsed["object"])
		})

		t.Run("passthrough local_reply_policy encodes local rejection while keeping cache hit raw", func(t *testing.T) {
			passthroughCfg, err := sjson.SetBytes(nativeCacheExecutionConfig(t, true), "protocol_mode", "passthrough")
			require.NoError(t, err)
			passthroughCfg, err = sjson.SetBytes(passthroughCfg, "local_reply_policy", map[string]any{
				"input_protocol": "openai_chat",
			})
			require.NoError(t, err)

			t.Run("local rejection encoded", func(t *testing.T) {
				host, status := test.NewTestHost(passthroughCfg)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)

				require.Equal(t, types.ActionPause, host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "api.example.com"},
					{":path", "/native/custom-answer"},
					{":method", http.MethodPost},
					{"content-type", "application/json"},
					{"x-request-id", "req-cache-pass-403"},
				}))
				resp := host.GetLocalResponse()
				require.NotNil(t, resp)
				require.Equal(t, uint32(http.StatusForbidden), resp.StatusCode)
				require.Equal(t, "ai-cache.execution_delivery.authenticated_caller_missing", resp.StatusCodeDetail)
				require.Contains(t, resp.Headers, [2]string{"x-request-id", "req-cache-pass-403"})
				require.JSONEq(t, `{"error":{"type":"permission_error","code":"permission_denied","message":"The account is not permitted to use this service."}}`, string(resp.Data))
			})

			t.Run("valid passthrough retains body basis and leaves upstream payload raw", func(t *testing.T) {
				host, status := test.NewTestHost(passthroughCfg)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)

				reqBody := []byte(nativeCacheTestBody)
				require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(nativeCacheExecutionHeaders(true)))
				require.Equal(t, types.ActionContinue, host.CallOnHttpRequestBody(reqBody))
				require.Nil(t, host.GetLocalResponse())
				require.Empty(t, host.GetRedisCalloutAttributes())
				require.Empty(t, host.GetHttpCalloutAttributes())
				original := []byte("  opaque upstream payload\n")
				host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "text/plain"}})
				host.CallOnHttpResponseBody(original)
				require.Equal(t, original, host.GetResponseBody())
			})
		})

		t.Run("legacy config without local_reply_policy preserves unavailable body", func(t *testing.T) {
			host, status := test.NewTestHost(nativeCacheExecutionConfig(t, true))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.Equal(t, types.ActionPause, host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "api.example.com"},
				{":path", "/native/custom-answer"},
				{":method", http.MethodPost},
				{"content-type", "application/json"},
			}))
			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusForbidden), resp.StatusCode)
			require.Equal(t, "ai-cache.execution_delivery.authenticated_caller_missing", resp.StatusCodeDetail)
			require.JSONEq(t, `{"error":"native cache request unavailable"}`, string(resp.Data))
		})
	})
}
