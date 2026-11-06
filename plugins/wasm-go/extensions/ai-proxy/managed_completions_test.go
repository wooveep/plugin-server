package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/stretchr/testify/require"
)

const completionsUsageFixture = `{"prompt_tokens":12,"completion_tokens":4,"total_tokens":16,"prompt_tokens_details":{"cached_tokens":3},"prompt_cache_hit_tokens":3,"prompt_cache_miss_tokens":9}`

func managedCompletionsFixture() map[string]interface{} {
	fixture := responseModeFixture("deepseek", managedProtocolCompletions, managedProtocolCompletions, "passthrough", "")
	settings := fixture["managed_execution"].(managedSettings)
	settings.UpstreamPath = "/beta/completions"
	fixture["managed_execution"] = settings
	return fixture
}

func TestManagedCompletionsRetainsNativeResponseAndBillingUsage(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%t", streaming), func(t *testing.T) {
			h := newManagedHost(t, managedCompletionsFixture())
			request := []byte(fmt.Sprintf(`{"model":"upstream-model","prompt":"def add(a,b):\n    ","suffix":"\n","max_tokens":32,"stream":%t}`, streaming))
			h.start(t, request, request)
			calls := h.host.GetCalloutAttributesFromContext(h.id)
			require.Len(t, calls, 1)
			path, err := managedSingleHeader(calls[0].Headers, ":path")
			require.NoError(t, err)
			require.Equal(t, "/beta/completions", path)
			key, err := managedSingleHeader(calls[0].Headers, "authorization")
			require.NoError(t, err)
			require.Equal(t, "Bearer controlled-upstream-key", key)
			require.Equal(t, request, calls[0].Body)

			contentType := "application/json"
			result := `{"object":"text_completion","model":"upstream-model","choices":[{"index":0,"text":"return a + b","logprobs":null,"finish_reason":"stop"}],"usage":` + completionsUsageFixture + `}`
			if streaming {
				contentType = "text/event-stream"
				result = "data: " + `{"object":"text_completion","model":"upstream-model","choices":[{"index":0,"text":"return a + b","logprobs":null,"finish_reason":null}]}` + "\n\n" +
					"data: " + strings.Replace(result, `"text":"return a + b"`, `"text":""`, 1) + "\n\n" +
					"data: [DONE]\n\n"
			}
			h.response(t, [][2]string{{":status", "200"}, {"content-type", contentType}}, []byte(result))
			reply := h.host.GetSentLocalResponse(h.id)
			require.NotNil(t, reply)
			require.EqualValues(t, 200, reply.StatusCode)
			require.Equal(t, result, string(reply.Data))

			records := h.records(t)
			require.Len(t, records, 2)
			attempt := decodeManagedFact(t, records[1].raw)
			require.Equal(t, "upstream_attempt", attempt.Kind)
			require.Equal(t, "accepted", attempt.Acceptance)
			require.Equal(t, managedBodyDigest([]byte(result)), attempt.ResponseBodyDigest)
			delivery := h.bridge(t).Envelope()
			require.Equal(t, "text_generate", delivery.OperationKind)
			require.Equal(t, "reported", attempt.Usage.Status)
			require.Equal(t, attempt.Usage, delivery.Usage)
			require.Equal(t, map[string]string{"input": "12", "output": "4", "cache_hit": "3", "cache_miss": "9"}, delivery.Usage.Quantities)
		})
	}
}

func TestManagedCompletionsRouterStreamsBeforeWritingOneDelivery(t *testing.T) {
	fixture := managedCompletionsFixture()
	settings := fixture["managed_execution"].(managedSettings)
	settings.SendTransport, settings.StreamProtocol = "router", managedProtocolCompletions
	fixture["managed_execution"] = settings
	h := newManagedHost(t, fixture)
	release := fixture["_modelfusion_execution"].(managedRelease)
	require.NoError(t, h.host.SetProperty([]string{"cluster_name"}, []byte(release.Descriptor.ClusterRef)))
	request := []byte(`{"model":"upstream-model","prompt":"def add(a,b):\n    ","suffix":"\n","stream":true}`)
	h.start(t, request, request)
	require.Empty(t, h.host.GetCalloutAttributesFromContext(h.id))
	path, err := managedSingleHeader(h.host.GetCurrentRequestHeaders(h.id), ":path")
	require.NoError(t, err)
	require.Equal(t, "/beta/completions", path)
	require.Equal(t, request, h.host.GetCurrentRequestBody(h.id))

	require.NoError(t, h.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
	require.Equal(t, types.ActionContinue, h.host.CallOnResponseHeaders(h.id,
		[][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}, false))
	head := []byte("data: " + `{"object":"text_completion","choices":[{"index":0,"text":"return a + b","finish_reason":null}]}` + "\n\n")
	require.Equal(t, types.ActionContinue, h.host.CallOnResponseBody(h.id, head, false))
	require.Equal(t, head, h.host.GetCurrentResponseBody(h.id))
	require.Len(t, h.records(t), 1, "only execution registration precedes the terminal")
	tail := []byte("data: " + `{"object":"text_completion","choices":[{"index":0,"text":"","finish_reason":"stop"}],"usage":` + completionsUsageFixture + "}\n\ndata: [DONE]\n\n")
	require.Equal(t, types.ActionContinue, h.host.CallOnResponseBody(h.id, tail, true))
	require.Equal(t, tail, h.host.GetCurrentResponseBody(h.id))
	records := h.records(t)
	require.Len(t, records, 3)
	attempt := decodeManagedFact(t, records[1].raw)
	delivery := decodeManagedFact(t, records[2].raw)
	require.Equal(t, "upstream_attempt", attempt.Kind)
	require.Equal(t, "call_delivery", delivery.Kind)
	require.Equal(t, attempt.Usage, delivery.Usage)
	require.Equal(t, map[string]string{"input": "12", "output": "4", "cache_hit": "3", "cache_miss": "9"}, delivery.Usage.Quantities)
	h.host.CompleteHttpContext(h.id)
	require.Len(t, h.records(t), 3)
}

func TestManagedCompletionsStreamRequiresTerminalAndPreservesInterruptedUsage(t *testing.T) {
	terminal := "data: " + `{"object":"text_completion","model":"deepseek-flash","choices":[{"index":0,"text":"","finish_reason":"aborted"}],"usage":` + completionsUsageFixture + "}\n\n"
	stream := managedChatStream{completions: true}
	for _, b := range []byte(terminal) {
		stream.push([]byte{b})
	}
	unfinished := stream.observation()
	require.False(t, unfinished.BodyComplete)
	require.Equal(t, "missing", unfinished.Usage.Status)
	stream.push([]byte("data: [DONE]\n\n"))
	observed := stream.observation()
	require.True(t, observed.BodyComplete)
	require.Equal(t, "partial", observed.ExecutionResult)
	require.Equal(t, "4", observed.Usage.Quantities["output"])
}
