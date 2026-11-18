package main

import (
	"bytes"
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

type mapperCommittedCtx struct {
	wrapper.HttpContext
}

func (mapperCommittedCtx) GetExecutionPhase() iface.HTTPExecutionPhase { return iface.EncodeHeader }
func (mapperCommittedCtx) GetStringContext(string, string) string      { return "req-mapper-committed" }
func (mapperCommittedCtx) GetContext(string) interface{}               { return nil }
func (mapperCommittedCtx) SetContext(string, interface{})              {}
func (mapperCommittedCtx) DontReadRequestBody()                        {}

func TestModelMapperLocalReplyConfigValidationU02(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for idx, raw := range []string{
			`{"capture_body_basis":true,"modelMapping":{"a":"b"},"local_reply_policy":{"input_protocol":"anthropic_messages","response_profile":"openai"}}`,
			`{"capture_body_basis":true,"modelMapping":{"a":"b"},"local_reply_policy":{"response_profile":"deepseek"}}`,
			`{"capture_body_basis":true,"modelMapping":{"a":"b"},"local_reply_policy":{"input_protocol":"openai_chat","response_profile":"deepseek"},"local_reply_policy":{"input_protocol":"openai_chat","response_profile":"qwen"}}`,
		} {
			host, status := test.NewTestHost(json.RawMessage(raw))
			host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status, "invalid config %d", idx)
		}

		host, status := test.NewTestHost(json.RawMessage(captureConfig))
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)
		host.InitHttp()

		policy, err := clientresponse.ParseLocalReplyPolicy(clientresponse.Messages, "qwen")
		require.NoError(t, err)
		require.Equal(t, types.ActionPause, reject(mapperCommittedCtx{}, &policy, http.StatusInternalServerError, "header_read_failed"))
		require.Equal(t, types.ActionPause, onStandaloneRequestBody(mapperCommittedCtx{}, standaloneConfig{
			modelKey:         "model",
			localReplyPolicy: &policy,
		}, []byte(`{"model":"a","model":"b"}`)))
		require.Nil(t, host.GetLocalResponse(), "must not send legacy fallback when SendLocalReply fails on committed response")
		joined := strings.Join(host.GetCriticalLogs(), "\n")
		require.Contains(t, joined, "model-mapper failed to send local reply (status=500, detail=modelfusion.local.model-mapper.header_read_failed)")
		require.Contains(t, joined, "model-mapper failed to send local reply (status=400, detail=duplicate_model_member)")
		require.Contains(t, joined, clientresponse.ErrLocalReplyAfterResponseCommitted.Error())
	})
}

func TestModelMapperLocalReplyRejectionsAndLegacyFallbackU03U04(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		anthropicCfg := json.RawMessage(`{
			"capture_body_basis": true,
			"modelMapping": {"public.v1": "provider.v2"},
			"local_reply_policy": {
				"input_protocol": "anthropic_messages",
				"response_profile": "qwen"
			}
		}`)

		t.Run("oversized body keeps 413 and encodes anthropic request_too_large", func(t *testing.T) {
			host, status := test.NewTestHost(anthropicCfg)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			headers := append(mapperHeaders(), [2]string{"x-request-id", "req-mapper-413"})
			require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(headers))
			huge := append([]byte(`{"model":"public.v1","text":"`), bytes.Repeat([]byte("a"), maxRequestBodyBytes)...)
			huge = append(huge, []byte(`"}`)...)
			require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(huge))

			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusRequestEntityTooLarge), resp.StatusCode)
			require.Equal(t, "modelfusion.local.model-mapper.body_too_large", resp.StatusCodeDetail)
			require.Contains(t, resp.Headers, [2]string{"x-request-id", "req-mapper-413"})
			require.JSONEq(t, `{"type":"error","error":{"type":"invalid_request_error","code":"request_too_large","message":"The request is too large."}}`, string(resp.Data))
		})

		t.Run("unmapped model keeps 422 and encodes invalid_parameter", func(t *testing.T) {
			host, status := test.NewTestHost(anthropicCfg)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(mapperHeaders()))
			require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody([]byte(`{"model":"other.v1","messages":[]}`)))

			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusUnprocessableEntity), resp.StatusCode)
			require.Equal(t, "modelfusion.local.model-mapper.model_mapping_missing", resp.StatusCodeDetail)
			require.JSONEq(t, `{"type":"error","error":{"type":"invalid_request_error","code":"invalid_parameter","message":"The request body or model is invalid."}}`, string(resp.Data))
		})

		t.Run("standalone duplicate model encodes openai_chat when policy configured", func(t *testing.T) {
			standaloneCfg := json.RawMessage(`{
				"modelMapping": {"public.v1": "provider.v2"},
				"local_reply_policy": {
					"input_protocol": "openai_chat",
					"response_profile": "openai"
				}
			}`)
			host, status := test.NewTestHost(standaloneCfg)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(mapperHeaders()))
			require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody([]byte(`{"model":"public.v1","model":"other"}`)))

			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusBadRequest), resp.StatusCode)
			require.Equal(t, "duplicate_model_member", resp.StatusCodeDetail)
			require.JSONEq(t, `{"error":{"type":"invalid_request_error","code":"invalid_parameter","message":"The request contains duplicate model fields."}}`, string(resp.Data))
		})

		t.Run("legacy capture config without policy preserves model_mapper_error envelope", func(t *testing.T) {
			host, status := test.NewTestHost(json.RawMessage(captureConfig))
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(mapperHeaders()))
			require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody([]byte(`{"model":"other.v1"}`)))

			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusUnprocessableEntity), resp.StatusCode)
			require.Equal(t, "modelfusion.local.model-mapper.model_mapping_missing", resp.StatusCodeDetail)
			require.JSONEq(t, `{"error":{"code":"model_mapping_missing","type":"model_mapper_error"}}`, string(resp.Data))
		})
	})
}
