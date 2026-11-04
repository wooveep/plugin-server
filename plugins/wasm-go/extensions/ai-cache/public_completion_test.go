package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestNativeCacheHitExposesItsOriginalAdmittedTrace(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(nativeCacheExecutionConfig(t, true))
		require.Equal(t, types.OnPluginStartStatusOK, status)
		defer host.Reset()
		host.CallOnHttpRequestHeaders(nativeCacheExecutionHeaders(true))
		body := []byte(nativeCacheTestBody)
		host.CallOnHttpRequestBody(body)
		replyNativeRegisteredCacheLookup(t, host, body, false)
		reply := host.GetLocalResponse()
		require.NotNil(t, reply)
		require.Equal(t, nativeCacheOriginalCall, thinReplayPayloadHeader(reply.Headers, "x-request-id"))
	})
}

func TestIncompletePublicResponseCannotBecomeCacheReplay(t *testing.T) {
	for _, finish := range []string{"failed", "incomplete", "cancelled"} {
		t.Run(finish, func(t *testing.T) {
			raw := validThinResponsesReplayRecord(t, false, map[string]interface{}{
				"finish_reason": finish,
				"response":      map[string]interface{}{"object": "response", "status": finish, "output": []interface{}{map[string]interface{}{"type": "message", "role": "assistant", "content": []interface{}{map[string]interface{}{"type": "output_text", "text": "partial"}}}}},
			})
			var record MaterializedReplayRecord
			require.NoError(t, json.Unmarshal([]byte(raw), &record))
			material := ScopedCacheKeyMaterial{Tenant: record.Tenant, Consumer: record.Consumer, Route: record.Route, Model: record.Model, Protocol: record.Protocol, CacheScope: record.CacheScope, CachePolicyVersion: record.CachePolicyVersion, RequestDigest: record.RequestDigest}
			require.Error(t, validateReplayRecordScopeAndPayload(record, material, false, time.Now().Unix()))
		})
	}
}

func TestFailedHTTP200ResponsesDoNotCapturePartialCacheAnswers(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		host := startThinResponsesResponseCaptureRequest(t, false)
		defer host.Reset()
		host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
		host.CallOnHttpResponseBody([]byte(`{"id":"resp_partial","object":"response","model":"gpt-4.1","status":"failed","error":{"code":"response_incomplete","message":"cut off"},"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"PARTIAL_MUST_NOT_REPLAY"}]}],"usage":{"input_tokens":13,"output_tokens":5,"total_tokens":18}}`))
		if event, ok := thinResponseCaptureEvent(t, host); ok {
			require.Empty(t, event["assistant_content"])
		}
	})
}

func TestCacheOutputGateRejectsProtocolErrorEvents(t *testing.T) {
	for _, kind := range []protocol.ProtocolKind{protocol.ProtocolChatCompletions, protocol.ProtocolResponses, protocol.ProtocolMessages} {
		t.Run(string(kind), func(t *testing.T) {
			require.NotEmpty(t, thinOutputGate(kind, []byte(`{"type":"error","error":{"type":"api_error","message":"incomplete"}}`)).blockedReason)
		})
	}
}

func TestTextCacheDoesNotFlattenRealContinuationContent(t *testing.T) {
	for _, raw := range []string{
		`{"store":true,"input":"hello"}`,
		`{"previous_response_id":"resp_pf_reference","input":"next"}`,
		`{"conversation":{"id":"conv_pf_reference"},"input":"next"}`,
	} {
		require.NotEmpty(t, thinRequestOutputGate(protocol.ProtocolResponses, gjson.Parse(raw)))
	}
	for _, tc := range []struct {
		kind protocol.ProtocolKind
		raw  string
	}{
		{protocol.ProtocolChatCompletions, `{"choices":[{"message":{"content":"answer","reasoning_content":"real thinking"},"finish_reason":"stop"}]}`},
		{protocol.ProtocolMessages, `{"content":[{"type":"thinking","thinking":"real thinking","signature":"original-signature"}]}`},
		{protocol.ProtocolMessages, `{"type":"content_block_delta","delta":{"type":"signature_delta","signature":"original-signature"}}`},
		{protocol.ProtocolResponses, `{"object":"response","status":"completed","store":true,"output":[]}`},
		{protocol.ProtocolResponses, `{"output":[{"type":"reasoning","encrypted_content":"original-encrypted-context"}]}`},
		{protocol.ProtocolResponses, `{"type":"response.reasoning_text.delta","delta":"real thinking"}`},
	} {
		require.NotEmpty(t, thinOutputGate(tc.kind, []byte(tc.raw)).blockedReason)
	}
}
