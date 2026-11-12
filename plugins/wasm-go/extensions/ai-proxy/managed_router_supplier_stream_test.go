package main

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/stretchr/testify/require"
)

func TestRouterCollectsOriginalHeadersBeforeJSONOrStreamBody(t *testing.T) {
	for _, contentType := range []string{"application/json", "text/event-stream"} {
		t.Run(contentType, func(t *testing.T) {
			fixture := managedConfigFixture()
			settings := fixture["managed_execution"].(managedSettings)
			settings.SendTransport, settings.StreamProjection = "router", true
			fixture["managed_execution"] = settings
			h := newManagedHost(t, fixture)
			release := fixture["_modelfusion_execution"].(managedRelease)
			require.NoError(t, h.host.SetProperty([]string{"cluster_name"}, []byte(release.Descriptor.ClusterRef)))
			h.start(t, []byte(managedOriginalBody), []byte(managedMappedBody))
			state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
			require.Nil(t, state.firstByteMS)
			state.sendStartedAt = time.Now().Add(-10 * time.Millisecond)
			require.NoError(t, h.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
			h.host.CallOnResponseHeaders(h.id, [][2]string{{":status", "200"}, {"content-type", contentType}}, false)
			// No body or stream completion callback has run at this point.
			require.NotNil(t, state.firstByteMS)
			require.GreaterOrEqual(t, *state.firstByteMS, int64(10))
		})
	}
}

func TestRouterSupplierStreamsForwardBeforeTerminalAndKeepOriginalCounters(t *testing.T) {
	for _, supplier := range []struct {
		name, config, head, tail, input, output string
	}{
		{"qwen", managedQwenFixtureProvider,
			"event: result\ndata: " + `{"request_id":"native-q","output":{"choices":[{"message":{"role":"assistant","content":"early"},"finish_reason":"null"}]}}` + "\n\n",
			"event: result\ndata: " + `{"request_id":"native-q","output":{"choices":[{"message":{"role":"assistant","content":"early"},"finish_reason":"stop"}]},"usage":{"input_tokens":9007199254740993,"output_tokens":2,"prompt_tokens_details":{"cached_tokens":1}}}` + "\n\n", "9007199254740993", "2"},
		{"gemini", `{"type":"gemini","apiTokens":["controlled-upstream-key"]}`,
			"data: " + `{"candidates":[{"index":0,"content":{"parts":[{"text":"early"}],"role":"model"}}]}` + "\n\n",
			"data: " + `{"candidates":[{"index":0,"finishReason":"STOP"}]}` + "\n\ndata: {\"usageMetadata\":" + managedGoogleFixtureUsage + "}\n\n", "9007199254740997", "5"},
		{"moonshot", `{"type":"moonshot","apiTokens":["controlled-upstream-key"]}`,
			"data: " + `{"model":"upstream-model","choices":[{"index":0,"delta":{"role":"assistant","content":"early"},"finish_reason":null}]}` + "\n\n",
			"data: " + `{"model":"upstream-model","choices":[{"index":0,"delta":{},"finish_reason":"stop","usage":{"prompt_tokens":9007199254740993,"completion_tokens":2}}]}` + "\n\ndata: [DONE]\n\n", "9007199254740993", "2"},
	} {
		for _, protocol := range []string{managedProtocolChat, managedProtocolResponses, managedProtocolMessages} {
			t.Run(supplier.name+"/"+protocol, func(t *testing.T) {
				fixture := managedConfigFixture()
				fixture["provider"] = json.RawMessage(supplier.config)
				settings := fixture["managed_execution"].(managedSettings)
				settings.InputProtocol, settings.UpstreamProtocol = protocol, managedProtocolChat
				settings.SendTransport, settings.StreamProjection = "router", true
				if protocol != managedProtocolChat {
					settings.StreamProtocol = protocol
				}
				fixture["managed_execution"] = settings
				h := newManagedHost(t, fixture)
				release := fixture["_modelfusion_execution"].(managedRelease)
				require.NoError(t, h.host.SetProperty([]string{"cluster_name"}, []byte(release.Descriptor.ClusterRef)))
				payload := `"messages":[{"role":"user","content":"hello"}],"max_tokens":12`
				if protocol == managedProtocolResponses {
					payload = `"input":"hello"`
				}
				original := []byte(`{"model":"public-model",` + payload + `,"stream":true,"stream_options":{"include_usage":true}}`)
				h.start(t, original, bytes.Replace(original, []byte("public-model"), []byte("upstream-model"), 1))
				require.Empty(t, h.host.GetCalloutAttributesFromContext(h.id))
				require.NoError(t, h.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
				require.Equal(t, types.ActionContinue, h.host.CallOnResponseHeaders(h.id, [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}, false))
				// Split inside a UTF-8/event boundary and require early content before
				// the response end or the final usage has arrived.
				for i := range len(supplier.head) - 1 {
					h.host.CallOnResponseBody(h.id, []byte(supplier.head[i:i+1]), false)
				}
				h.host.CallOnResponseBody(h.id, []byte(supplier.head[len(supplier.head)-1:]), false)
				require.Contains(t, string(h.host.GetCurrentResponseBody(h.id)), "early")
				require.Len(t, h.records(t), 1)
				h.host.CallOnResponseBody(h.id, []byte(supplier.tail), true)
				for _, call := range h.host.GetRedisCalloutAttributesFromContext(h.id) {
					h.host.CallOnRedisCallResponse(call.CalloutID, 0, []byte(":1\r\n"))
				}
				records := h.records(t)
				require.Equal(t, 3, len(records))
				attempt, delivery := decodeManagedFact(t, records[1].raw), decodeManagedFact(t, records[2].raw)
				require.NotNil(t, attempt.BodyComplete)
				require.True(t, *attempt.BodyComplete)
				require.Equal(t, "succeeded", attempt.ExecutionResult)
				require.Equal(t, supplier.input, attempt.Usage.Quantities["input"])
				require.Equal(t, supplier.output, attempt.Usage.Quantities["output"])
				require.Equal(t, attempt.Usage, delivery.Usage)
				require.Equal(t, managedBodyDigest([]byte(supplier.head+supplier.tail)), attempt.ResponseBodyDigest)
				h.host.CompleteHttpContext(h.id)
				require.Len(t, h.records(t), 3)
			})
		}
	}
}
