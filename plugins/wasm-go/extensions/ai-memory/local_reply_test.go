package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/iface"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

type memoryCommittedCtx struct {
	wrapper.HttpContext
}

func (memoryCommittedCtx) GetExecutionPhase() iface.HTTPExecutionPhase { return iface.EncodeHeader }
func (memoryCommittedCtx) GetStringContext(string, string) string      { return "req-memory-committed" }
func (memoryCommittedCtx) GetContext(string) interface{}               { return nil }
func (memoryCommittedCtx) SetContext(string, interface{})              {}
func (memoryCommittedCtx) SetUserAttribute(string, interface{})        {}
func (memoryCommittedCtx) WriteUserAttributeToLog() error              { return nil }
func (memoryCommittedCtx) DontReadRequestBody()                        {}

func TestMemoryLocalReplyConfigValidationU02(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		base := memoryBodyBasisConfig(t, "anthropic_messages", "semantic")
		for idx, policy := range []any{
			map[string]any{"input_protocol": "anthropic_messages", "response_profile": "openai"},
			map[string]any{"input_protocol": "openai_chat", "response_profile": "deepseek"},
			map[string]any{"response_profile": "deepseek"},
		} {
			raw, err := sjson.SetBytes(base, "_rules_.0.local_reply_policy", policy)
			require.NoError(t, err)
			host, status := newMemoryConfigTestHost(raw)
			host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status, "invalid policy %d", idx)
		}

		host, status := newMemoryConfigTestHost(base)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)
		host.InitHttp()

		policy, err := clientresponse.ParseLocalReplyPolicy(clientresponse.Messages, "deepseek")
		require.NoError(t, err)
		require.Equal(t, types.ActionPause, rejectMemoryBodyBasis(memoryCommittedCtx{}, &policy, http.StatusUnsupportedMediaType, "content_encoding_unsupported"))
		require.Nil(t, host.GetLocalResponse(), "must not send legacy fallback when SendLocalReply fails on committed response")
		joined := strings.Join(host.GetCriticalLogs(), "\n")
		require.Contains(t, joined, "ai-memory failed to send local reply (status=415, detail=modelfusion.local.ai-memory.content_encoding_unsupported)")
		require.Contains(t, joined, clientresponse.ErrLocalReplyAfterResponseCommitted.Error())
	})
}

func TestMemoryLocalReplyBodyBasisRejectionsAndLegacyFallbackU03U04(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		base := memoryBodyBasisConfig(t, "anthropic_messages", "semantic")
		withPolicy, err := sjson.SetBytes(base, "_rules_.0.local_reply_policy", map[string]any{
			"input_protocol":   "anthropic_messages",
			"response_profile": "deepseek",
		})
		require.NoError(t, err)

		t.Run("unsupported content-encoding keeps 415 and encodes anthropic invalid_parameter", func(t *testing.T) {
			host, status := newMemoryConfigTestHost(withPolicy)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("memory-route"))

			headers := append(memoryConsoleAssembleHeaders(), [2]string{"content-encoding", "gzip"})
			require.Equal(t, types.ActionPause, host.CallOnHttpRequestHeaders(headers))

			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusUnsupportedMediaType), resp.StatusCode)
			require.Equal(t, "modelfusion.local.ai-memory.content_encoding_unsupported", resp.StatusCodeDetail)
			require.Contains(t, resp.Headers, [2]string{"x-request-id", "request-console-assemble-1"})
			require.JSONEq(t, `{"type":"error","error":{"type":"invalid_request_error","code":"invalid_parameter","message":"The request body or model is invalid."}}`, string(resp.Data))
		})

		t.Run("invalid body keeps 400 and encodes anthropic invalid_parameter", func(t *testing.T) {
			host, status := newMemoryConfigTestHost(withPolicy)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("memory-route"))

			require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(memoryConsoleAssembleHeaders()))
			require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody([]byte(`{"messages":[]}`)))

			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusBadRequest), resp.StatusCode)
			require.Equal(t, "modelfusion.local.ai-memory.invalid_model_body", resp.StatusCodeDetail)
			require.JSONEq(t, `{"type":"error","error":{"type":"invalid_request_error","code":"invalid_parameter","message":"The request body or model is invalid."}}`, string(resp.Data))
		})

		t.Run("legacy config without local_reply_policy preserves memory_body_basis_error", func(t *testing.T) {
			host, status := newMemoryConfigTestHost(base)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("memory-route"))

			headers := append(memoryConsoleAssembleHeaders(), [2]string{"content-encoding", "gzip"})
			require.Equal(t, types.ActionPause, host.CallOnHttpRequestHeaders(headers))

			resp := host.GetLocalResponse()
			require.NotNil(t, resp)
			require.Equal(t, uint32(http.StatusUnsupportedMediaType), resp.StatusCode)
			require.Equal(t, "modelfusion.local.ai-memory.content_encoding_unsupported", resp.StatusCodeDetail)
			require.JSONEq(t, `{"error":{"code":"content_encoding_unsupported","type":"memory_body_basis_error"}}`, string(resp.Data))
		})
	})
}
