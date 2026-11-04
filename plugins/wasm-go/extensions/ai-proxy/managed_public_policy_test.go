package main

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestExplicitQwenPolicyBlockCannotFallbackEvenWhenStatusMatches(t *testing.T) {
	fixture := managedTextFixture(t, true)
	directory := fixture[managedTextDirectoryField].(managedTextDirectory)
	entry := &directory.Targets[0]
	var proxy, execution map[string]interface{}
	require.NoError(t, json.Unmarshal(entry.Proxy, &proxy))
	require.NoError(t, json.Unmarshal(entry.Execution, &execution))
	policy := proxy["managed_execution"].(map[string]interface{})["text_execution"].(map[string]interface{})
	delete(policy, "retry")
	policy["fallback"].(map[string]interface{})["response_codes"] = []string{"400"}
	provider := proxy["providers"].([]interface{})[0].(map[string]interface{})
	provider["type"], provider["qwenEnableCompatible"] = "qwen", true
	entry.Proxy = managedFixtureJSON(t, proxy)
	descriptor := execution["descriptor"].(map[string]interface{})
	digest, err := mfjson.Digest(entry.Proxy)
	require.NoError(t, err)
	descriptor["proxy_digest"] = digest
	descriptor["descriptor_digest"], err = managedDescriptorDigest(managedFixtureJSON(t, descriptor))
	require.NoError(t, err)
	entry.Execution = managedFixtureJSON(t, execution)
	proxy["_modelfusion_execution"], proxy[managedTextDirectoryField] = json.RawMessage(entry.Execution), directory
	h := newManagedTextHost(t, proxy)
	h.start(t, []byte(managedOriginalBody), []byte(managedMappedBody))
	respondManagedTextHealth(t, h, map[string]bool{"credential-1": true})
	sends := h.host.GetCalloutAttributesFromContext(h.id)
	require.Len(t, sends, 1)
	h.host.CallOnHttpCallResponse(sends[0].CalloutID, [][2]string{{":status", "400"}, {"content-type", "application/json"}}, nil, []byte(`{"error":{"code":"DataInspectionFailed","message":"private diagnostic"}}`))
	require.Empty(t, h.host.GetRedisCalloutAttributesFromContext(h.id), "content policy must not be bypassed by another target")
	require.Empty(t, h.host.GetCalloutAttributesFromContext(h.id))
	response := h.host.GetSentLocalResponse(h.id)
	require.NotNil(t, response)
	require.EqualValues(t, 400, response.StatusCode)
	require.Equal(t, "content_policy_violation", gjson.GetBytes(response.Data, "error.code").String())
}

func TestEmbeddedFailureIsNotASuccessfulJSONCompletion(t *testing.T) {
	h := publicFailureProbe(t, managedProtocolChat, managedProtocolChat, `{"type":"deepseek","apiTokens":["controlled-upstream-key"]}`, 200, false, `{"error":{"code":"server_error","message":"fixture-acc-7 private diagnostic"}}`)
	response := h.host.GetSentLocalResponse(h.id)
	require.EqualValues(t, 500, response.StatusCode)
	require.Equal(t, "server_error", gjson.GetBytes(response.Data, "error.code").String())
	require.NotContains(t, string(response.Data), "fixture-acc-7")
}

func TestNativeStreamEOFHasPublicFailureWithoutSuccessTerminal(t *testing.T) {
	for _, tc := range []struct{ protocol, body, forbidden string }{
		{managedProtocolMessages, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"native-id\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"actual\",\"content\":[],\"stop_reason\":null,\"usage\":{\"input_tokens\":17,\"output_tokens\":0}}}\n\nevent: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"partial answer\"}}\n\n", "message_stop"},
		{managedProtocolResponses, "event: response.created\ndata: {\"type\":\"response.created\",\"sequence_number\":0,\"response\":{\"id\":\"native-id\",\"object\":\"response\",\"model\":\"actual\",\"status\":\"in_progress\",\"output\":[]}}\n\nevent: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"sequence_number\":1,\"item_id\":\"native-item\",\"output_index\":0,\"content_index\":0,\"delta\":\"partial answer\"}\n\n", "response.completed"},
	} {
		t.Run(tc.protocol, func(t *testing.T) {
			h := publicFailureProbe(t, tc.protocol, tc.protocol, `{"type":"openai","apiTokens":["controlled-upstream-key"]}`, 200, true, tc.body)
			response := h.host.GetSentLocalResponse(h.id)
			require.NotNil(t, response)
			require.EqualValues(t, 200, response.StatusCode)
			require.Contains(t, string(response.Data), "partial answer")
			require.Contains(t, string(response.Data), `"code":"response_incomplete"`)
			require.NotContains(t, string(response.Data), tc.forbidden)
			retry, _ := managedSingleHeader(response.Headers, "x-should-retry")
			require.Equal(t, "false", retry)
		})
	}
}

func TestNativeSupplierStreamFailureKeepsPrefixAndReportedUsage(t *testing.T) {
	for _, tc := range []struct{ name, provider, body string }{
		{"qwen", managedQwenFixtureProvider, "data: {\"output\":{\"choices\":[{\"message\":{\"role\":\"assistant\",\"content\":\"partial answer\"},\"finish_reason\":\"null\"}]},\"usage\":{\"input_tokens\":17,\"output_tokens\":5,\"total_tokens\":22},\"request_id\":\"private-qwen\"}\n\nevent: error\ndata: {\"code\":\"InternalError\",\"message\":\"fixture-acc-7 private diagnostic\"}\n\n"},
		{"gemini", `{"type":"gemini","apiTokens":["controlled-upstream-key"]}`, "data: {\"candidates\":[{\"index\":0,\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"partial answer\"}]}}],\"usageMetadata\":{\"promptTokenCount\":17,\"candidatesTokenCount\":5,\"totalTokenCount\":22}}\n\nevent: error\ndata: {\"error\":{\"code\":500,\"status\":\"INTERNAL\",\"message\":\"fixture-acc-7 private diagnostic\"}}\n\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := publicFailureProbe(t, managedProtocolResponses, managedProtocolChat, tc.provider, 200, true, tc.body)
			response := h.host.GetSentLocalResponse(h.id)
			require.NotNil(t, response)
			require.EqualValues(t, 200, response.StatusCode)
			require.Contains(t, string(response.Data), "partial answer")
			require.Contains(t, string(response.Data), "response.failed")
			require.Contains(t, string(response.Data), `"input_tokens":17`)
			require.Contains(t, string(response.Data), `"code":"server_error"`)
			require.NotContains(t, string(response.Data), "fixture-acc-7")
			require.NotContains(t, string(response.Data), "response.completed")
		})
	}
}
