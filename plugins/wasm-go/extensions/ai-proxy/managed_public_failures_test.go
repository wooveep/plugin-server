package main

import (
	"bytes"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func publicFailureProbe(t *testing.T, client, upstream, supplier string, status int, stream bool, body string) *managedHost {
	t.Helper()
	fixture := managedConfigFixture()
	fixture["provider"] = json.RawMessage(supplier)
	settings := fixture["managed_execution"].(managedSettings)
	settings.InputProtocol, settings.UpstreamProtocol = client, upstream
	fixture["managed_execution"] = settings
	h := newManagedHost(t, fixture)
	payload := `"messages":[{"role":"user","content":"hello"}],"max_tokens":64`
	if client == managedProtocolResponses {
		payload = `"input":"hello","max_output_tokens":64`
	}
	original := []byte(`{"model":"public-model",` + payload + `,"stream":` + strconv.FormatBool(stream) + `}`)
	h.start(t, original, bytes.Replace(original, []byte("public-model"), []byte("upstream-model"), 1))
	require.Len(t, h.host.GetCalloutAttributesFromContext(h.id), 1)
	typeHeader := "application/json"
	if stream {
		typeHeader = "text/event-stream"
	}
	h.response(t, [][2]string{{":status", strconv.Itoa(status)}, {"content-type", typeHeader}, {"retry-after", "10"}, {"x-supplier-account", "fixture-acc-7"}}, []byte(body))
	for _, call := range h.host.GetRedisCalloutAttributesFromContext(h.id) {
		if bytes.Contains(call.Query, []byte("modelfusion-public-resource-write.v1")) {
			h.host.CallOnRedisCallResponse(call.CalloutID, 0, []byte(":1\r\n"))
		}
	}
	for _, record := range h.records(t) {
		if record.messageType == "execution_registration" {
			continue
		}
		fact := decodeManagedFact(t, record.raw)
		require.NotNil(t, fact.HTTPStatus)
		require.Equal(t, uint16(status), *fact.HTTPStatus, "public policy must not rewrite original procurement HTTP")
		require.Equal(t, managedBodyDigest([]byte(body)), fact.ResponseBodyDigest)
		meters := []string{"input", "output", "cache_hit", "cache_miss"}
		for meter := range fact.Usage.Quantities {
			require.Contains(t, meters, meter)
		}
	}
	return h
}

func TestPublicResponseReviewSevenJSONProbes(t *testing.T) {
	const compatible = `{"type":"deepseek","apiTokens":["controlled-upstream-key"]}`
	const gemini = `{"type":"gemini","apiTokens":["controlled-upstream-key"]}`
	const partial = `{"id":"raw-id","model":"upstream-model","choices":[{"index":0,"message":{"role":"assistant","content":"partial answer"},"finish_reason":"aborted"}],"usage":{"prompt_tokens":17,"completion_tokens":5,"total_tokens":22}}`
	for _, tc := range []struct {
		name, client, provider, body, path, expected, retry string
		http, publicHTTP                                    int
	}{
		{"procurement_auth_is_not_customer_auth", managedProtocolMessages, compatible, `{"error":{"type":"invalid_api_key","code":"invalid_api_key","message":"fixture-acc-7 credential invalid"}}`, "error.code", "service_unavailable", "false", 401, 503},
		{"qwen_rate_quota_has_legal_messages_type", managedProtocolMessages, managedQwenFixtureProvider, `{"code":"Throttling.RateQuota","message":"fixture-acc-7 throttled","request_id":"supplier-trace"}`, "error.type", "rate_limit_error", "true", 429, 429},
		{"gemini_resource_exhaustion_is_not_generic_api_error", managedProtocolMessages, gemini, `{"error":{"code":429,"status":"RESOURCE_EXHAUSTED","message":"fixture-acc-7 throttled"}}`, "error.type", "rate_limit_error", "true", 429, 429},
		{"documented_chat_interruption_extension_control", managedProtocolChat, compatible, partial, "choices.0.finish_reason", "aborted", "", 200, 200},
		{"documented_interruption_keeps_responses_partial_output", managedProtocolResponses, compatible, partial, "status", "failed", "false", 200, 200},
		{"content_filter_uses_legal_messages_refusal", managedProtocolMessages, compatible, `{"id":"raw-id","model":"upstream-model","choices":[{"index":0,"message":{"role":"assistant","content":"partial answer"},"finish_reason":"content_filter"}],"usage":{"prompt_tokens":17,"completion_tokens":5,"total_tokens":22}}`, "stop_reason", "refusal", "", 200, 200},
		{"gemini_input_block_is_not_transport_failure", managedProtocolChat, gemini, `{"promptFeedback":{"blockReason":"SAFETY"},"usageMetadata":{"promptTokenCount":17,"candidatesTokenCount":0,"totalTokenCount":17}}`, "error.code", "content_policy_violation", "false", 200, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := publicFailureProbe(t, tc.client, managedProtocolChat, tc.provider, tc.http, false, tc.body)
			response := h.host.GetSentLocalResponse(h.id)
			require.NotNil(t, response)
			require.EqualValues(t, tc.publicHTTP, response.StatusCode)
			require.Equal(t, tc.expected, gjson.GetBytes(response.Data, tc.path).String())
			require.NotContains(t, string(response.Data), "fixture-acc-7")
			if tc.retry != "" {
				retry, _ := managedSingleHeader(response.Headers, "x-should-retry")
				require.Equal(t, tc.retry, retry)
				wait, _ := managedSingleHeader(response.Headers, "retry-after")
				if tc.retry == "true" {
					require.Equal(t, "10", wait)
				} else {
					require.Empty(t, wait)
				}
			}
			if tc.name == "documented_interruption_keeps_responses_partial_output" {
				require.Equal(t, "partial answer", gjson.GetBytes(response.Data, "output.0.content.0.text").String())
				require.Equal(t, "incomplete", gjson.GetBytes(response.Data, "output.0.status").String())
				require.Equal(t, "generation_interrupted", gjson.GetBytes(response.Data, "error.code").String())
				require.Equal(t, "17", gjson.GetBytes(response.Data, "usage.input_tokens").Raw)
			}
		})
	}
}

func TestPublicResponseReviewTwoSSEProbes(t *testing.T) {
	t.Run("partial_chat_then_explicit_error_to_responses", func(t *testing.T) {
		body := "data: {\"id\":\"raw-id\",\"model\":\"upstream-model\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"partial answer\"},\"finish_reason\":null}],\"usage\":{\"prompt_tokens\":17,\"completion_tokens\":5,\"total_tokens\":22}}\n\nevent: error\ndata: {\"error\":{\"type\":\"server_error\",\"code\":\"server_error\",\"message\":\"fixture-acc-7 failed\"}}\n\n"
		h := publicFailureProbe(t, managedProtocolResponses, managedProtocolChat, `{"type":"deepseek","apiTokens":["controlled-upstream-key"]}`, 200, true, body)
		response := h.host.GetSentLocalResponse(h.id)
		require.EqualValues(t, 200, response.StatusCode)
		require.Contains(t, string(response.Data), "partial answer")
		require.Contains(t, string(response.Data), "event: response.failed")
		require.Contains(t, string(response.Data), `"code":"server_error"`)
		require.Contains(t, string(response.Data), `"input_tokens":17`)
		require.NotContains(t, string(response.Data), "response.completed")
		require.NotContains(t, string(response.Data), "fixture-acc-7")
	})
	t.Run("native_messages_error_remains_a_terminal_error_control", func(t *testing.T) {
		body := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"raw-id\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"upstream-model\",\"content\":[],\"stop_reason\":null,\"usage\":{\"input_tokens\":17,\"output_tokens\":0}}}\n\nevent: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"Service busy\"}}\n\n"
		h := publicFailureProbe(t, managedProtocolMessages, managedProtocolMessages, `{"type":"claude","apiTokens":["controlled-upstream-key"]}`, 200, true, body)
		response := h.host.GetSentLocalResponse(h.id)
		require.EqualValues(t, 200, response.StatusCode)
		require.Contains(t, string(response.Data), `"type":"overloaded_error"`)
		require.NotContains(t, string(response.Data), "message_stop")
		require.NotContains(t, string(response.Data), "[DONE]")
	})
}
