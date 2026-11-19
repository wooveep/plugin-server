package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func managedResponseHeaderValue(headers [][2]string, name string) string {
	for _, h := range headers {
		if strings.EqualFold(h[0], name) {
			return h[1]
		}
	}
	return ""
}

func TestManagedLocalErrorContract_ResponsesProtocolRejectionAndHeaders(t *testing.T) {
	fixture := managedConfigFixture()
	settings := fixture["managed_execution"].(managedSettings)
	settings.InputProtocol = managedProtocolResponses
	settings.UpstreamProtocol = managedProtocolChat
	fixture["managed_execution"] = settings

	h := newManagedHost(t, fixture)
	original := []byte(`{"model":"public-model","input":"secret user prompt should never leak","previous_response_id":"resp_secret_id_123"}`)
	mapped := bytes.Replace(original, []byte(`"public-model"`), []byte(`"upstream-model"`), 1)
	headers := managedRequestHeaders(t, original)

	require.Equal(t, types.HeaderStopIteration, h.host.CallOnRequestHeaders(h.id, headers, false))
	require.Equal(t, types.ActionPause, h.host.CallOnRequestBody(h.id, mapped, true))

	reply := h.host.GetSentLocalResponse(h.id)
	require.NotNil(t, reply)
	require.EqualValues(t, 400, reply.StatusCode)
	require.Equal(t, "invalid_request_error", gjson.GetBytes(reply.Data, "error.type").String(), string(reply.Data))
	require.NotEqual(t, "server_error", gjson.GetBytes(reply.Data, "error.code").String())
	require.NotEmpty(t, gjson.GetBytes(reply.Data, "error.message").String())
	require.NotContains(t, string(reply.Data), "resp_secret_id_123")
	require.NotContains(t, string(reply.Data), "secret user prompt")

	require.Equal(t, "false", managedResponseHeaderValue(reply.Headers, "x-should-retry"))
	state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
	require.Equal(t, state.callID, managedResponseHeaderValue(reply.Headers, "x-request-id"))
}

func TestManagedLocalErrorContract_AllStatusClassesAndProtocols(t *testing.T) {
	for _, proto := range []string{managedProtocolChat, managedProtocolResponses, managedProtocolMessages} {
		for _, tc := range []struct {
			name      string
			status    uint32
			reason    string
			wantType  string
			wantCode  string
		}{
			{"400_protocol", 400, "unsupported_or_invalid_protocol_request", "invalid_request_error", "invalid_parameter"},
			{"400_json", 400, "invalid_json_body", "invalid_request_error", "invalid_parameter"},
			{"400_resource_unavailable", 400, "resource_reference_unavailable", "invalid_request_error", "resource_reference_unavailable"},
			{"400_resource_incompatible", 400, "resource_reference_incompatible", "invalid_request_error", "resource_reference_incompatible"},
			{"413_too_large", 413, "request_body_out_of_bounds", "invalid_request_error", "request_too_large"},
			{"403_admission", 403, "current_execution_admission_required", "permission_error", "permission_denied"},
			{"403_tool", 403, "tool_not_open_for_selected_target", "permission_error", "permission_denied"},
			{"503_unavailable", 503, "accounting_delivery_unavailable", "api_error", "service_unavailable"},
		} {
			t.Run(proto+"/"+tc.name, func(t *testing.T) {
				fixture := managedConfigFixture()
				settings := fixture["managed_execution"].(managedSettings)
				settings.InputProtocol = proto
				settings.UpstreamProtocol = proto
				fixture["managed_execution"] = settings

				h := newManagedHost(t, fixture)
				headers := managedRequestHeaders(t, []byte(managedOriginalBody))
				headers = append(headers, [2]string{"x-request-id", "req-trace-local-1"})
				require.Equal(t, types.HeaderStopIteration, h.host.CallOnRequestHeaders(h.id, headers, false))

				action := rejectManagedRequest(h.ctx, tc.status, tc.reason)
				require.Equal(t, types.ActionPause, action)

				reply := h.host.GetSentLocalResponse(h.id)
				require.NotNil(t, reply)
				require.EqualValues(t, tc.status, reply.StatusCode)
				require.Equal(t, tc.wantType, gjson.GetBytes(reply.Data, "error.type").String(), string(reply.Data))
				require.Equal(t, tc.wantCode, gjson.GetBytes(reply.Data, "error.code").String(), string(reply.Data))
				require.Equal(t, "false", managedResponseHeaderValue(reply.Headers, "x-should-retry"))
				require.NotEmpty(t, managedResponseHeaderValue(reply.Headers, "x-request-id"))
				if proto == managedProtocolMessages {
					require.Equal(t, "error", gjson.GetBytes(reply.Data, "type").String(), string(reply.Data))
				} else {
					require.False(t, gjson.GetBytes(reply.Data, "type").Exists(), string(reply.Data))
				}
			})
		}
	}
}

func TestManagedLocalErrorContract_MediaAndInternalKeepManagedExecutionError(t *testing.T) {
	h := newManagedTextHost(t, managedInternalFixture(t, false))
	action := h.host.CallOnRequestHeaders(h.id, [][2]string{{":method", "POST"}, {":path", "/v1/chat/completions"}, {":authority", "gateway.example"}, {"content-type", "application/json"}}, false)
	require.Equal(t, types.ActionPause, action)

	reply := h.host.GetSentLocalResponse(h.id)
	require.NotNil(t, reply)
	require.EqualValues(t, 403, reply.StatusCode)
	require.Equal(t, "managed_execution_error", gjson.GetBytes(reply.Data, "error.type").String())
	require.Equal(t, "registered_internal_service_call_required", gjson.GetBytes(reply.Data, "error.code").String())
}

func TestManagedProtocolPrepareRequest_StructuredSanitizedError(t *testing.T) {
	newManagedHost(t)
	profile := managedProtocolFixture(t, managedProtocolResponses, managedProtocolChat)
	payload, err := json.Marshal(map[string]any{
		"model":                "frozen-upstream",
		"input":                "SECRET_USER_PROMPT_VALUE",
		"previous_response_id": "SECRET_RESP_ID_TOKEN",
	})
	require.NoError(t, err)

	_, err = profile.PrepareRequest(payload, "frozen-upstream", "server-key")
	require.Error(t, err)
	detail := managedSafeProtocolDetail(err)
	require.Contains(t, detail, "previous_response_id")
	require.NotContains(t, detail, "SECRET_RESP_ID_TOKEN")
	require.NotContains(t, detail, "SECRET_USER_PROMPT_VALUE")
	require.NotContains(t, err.Error(), "SECRET_RESP_ID_TOKEN")
	require.NotContains(t, err.Error(), "SECRET_USER_PROMPT_VALUE")
}
