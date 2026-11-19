package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestManagedCanonicalHTTPCallSinglePassAndUnverifiedUsageGate(t *testing.T) {
	t.Run("json_conflicting_cache_write_never_leaks_raw_usage_and_mints_aliases_once", func(t *testing.T) {
		for _, clientProto := range []string{managedProtocolResponses, managedProtocolMessages} {
			t.Run(clientProto, func(t *testing.T) {
				fixture := managedConfigFixture()
				fixture["provider"] = json.RawMessage(`{"type":"deepseek","apiTokens":["controlled-upstream-key"]}`)
				settings := fixture["managed_execution"].(managedSettings)
				settings.InputProtocol, settings.UpstreamProtocol = clientProto, managedProtocolChat
				fixture["managed_execution"] = settings

				h := newManagedHost(t, fixture)
				reqBody := []byte(`{"model":"public-model","messages":[{"role":"user","content":"lookup"}],"max_tokens":32}`)
				if clientProto == managedProtocolResponses {
					reqBody = []byte(`{"model":"public-model","input":"lookup","max_output_tokens":32}`)
				}
				h.start(t, reqBody, bytes.Replace(reqBody, []byte("public-model"), []byte("upstream-model"), 1))
				require.Len(t, h.host.GetCalloutAttributesFromContext(h.id), 1)

				// cached_tokens (99) > prompt_tokens (10) causes managedOpenAIUsage to reject
				// the report (provider_cache_split_invalid -> outcome.Usage.Status = "estimated"),
				// whereas clientresponse.ReadUsage drops only cached_tokens and keeps 10/5/15.
				upstreamBody := []byte(`{
					"id":"chatcmpl-upstream-raw",
					"model":"upstream-model",
					"choices":[{
						"index":0,
						"message":{
							"role":"assistant",
							"content":null,
							"tool_calls":[{"id":"call_upstream_raw","type":"function","function":{"name":"lookup","arguments":"{\"id\":9007199254740993}"}}]
						},
						"finish_reason":"tool_calls"
					}],
					"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"prompt_tokens_details":{"cached_tokens":99}}
				}`)
				h.response(t, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, upstreamBody)
				drainDashScopeRedis(h)

				state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
				require.NotNil(t, state.lastAttempt)
				require.NotEqual(t, "reported", state.lastAttempt.Envelope().Usage.Status)

				sent := h.host.GetSentLocalResponse(h.id)
				require.NotNil(t, sent)
				require.EqualValues(t, 200, sent.StatusCode)

				if clientProto == managedProtocolResponses {
					require.Equal(t, gjson.Null, gjson.GetBytes(sent.Data, "usage").Type, "unverified usage must stay null: %s", sent.Data)
					respID := gjson.GetBytes(sent.Data, "id").String()
					itemID := gjson.GetBytes(sent.Data, "output.0.id").String()
					callID := gjson.GetBytes(sent.Data, "output.0.call_id").String()
					require.Equal(t, `{"id":9007199254740993}`, gjson.GetBytes(sent.Data, "output.0.arguments").String())

					records := state.publicAliases.Records()
					require.Len(t, records, 3, "response, item, and call aliases must be allocated exactly once")
					publicByKind := map[string]string{}
					for _, rec := range records {
						require.NotEqual(t, rec.Public, rec.Original, "self-seeded alias must not exist")
						publicByKind[rec.Kind] = rec.Public
					}
					require.Equal(t, respID, publicByKind["response"])
					require.Equal(t, itemID, publicByKind["item"])
					require.Equal(t, callID, publicByKind["call"])
				} else {
					require.False(t, gjson.GetBytes(sent.Data, "usage").Exists(), "unverified usage must be omitted in Messages: %s", sent.Data)
					msgID := gjson.GetBytes(sent.Data, "id").String()
					callID := gjson.GetBytes(sent.Data, "content.0.id").String()
					require.Equal(t, "9007199254740993", gjson.GetBytes(sent.Data, "content.0.input.id").Raw)

					records := state.publicAliases.Records()
					require.Len(t, records, 2, "message and call aliases must be allocated exactly once")
					publicByKind := map[string]string{}
					for _, rec := range records {
						require.NotEqual(t, rec.Public, rec.Original)
						publicByKind[rec.Kind] = rec.Public
					}
					require.Equal(t, msgID, publicByKind["message"])
					require.Equal(t, callID, publicByKind["call"])
				}
			})
		}
	})

	t.Run("buffered_sse_stream_usage_conflict_never_leaks_raw_usage_and_mints_aliases_once", func(t *testing.T) {
		fixture := managedConfigFixture()
		fixture["provider"] = json.RawMessage(`{"type":"deepseek","apiTokens":["controlled-upstream-key"]}`)
		settings := fixture["managed_execution"].(managedSettings)
		settings.InputProtocol, settings.UpstreamProtocol = managedProtocolResponses, managedProtocolChat
		fixture["managed_execution"] = settings

		h := newManagedHost(t, fixture)
		reqBody := []byte(`{"model":"public-model","input":"hello","stream":true}`)
		h.start(t, reqBody, bytes.Replace(reqBody, []byte("public-model"), []byte("upstream-model"), 1))
		require.Len(t, h.host.GetCalloutAttributesFromContext(h.id), 1)

		// Conflicting stream usage frames trigger stream_usage_conflict -> estimated outcome.Usage,
		// whereas clientresponse.ReadUsage would otherwise merge the second frame's 20/5/25 counters.
		sseBody := []byte(
			"data: {\"id\":\"chatcmpl-sse-raw\",\"model\":\"upstream-model\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"hello\"},\"finish_reason\":null}],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":2,\"total_tokens\":12}}\n\n" +
				"data: {\"id\":\"chatcmpl-sse-raw\",\"model\":\"upstream-model\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":20,\"completion_tokens\":5,\"total_tokens\":25}}\n\n" +
				"data: [DONE]\n\n",
		)
		h.response(t, [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}, sseBody)
		drainDashScopeRedis(h)

		state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
		require.NotNil(t, state.lastAttempt)
		require.Equal(t, "estimated", state.lastAttempt.Envelope().Usage.Status)

		sent := h.host.GetSentLocalResponse(h.id)
		require.NotNil(t, sent)
		require.EqualValues(t, 200, sent.StatusCode)
		require.Contains(t, string(sent.Data), `"type":"response.completed"`)
		require.Contains(t, string(sent.Data), `"usage":null`)
		require.NotContains(t, string(sent.Data), `"input_tokens":20`)
		require.NotContains(t, string(sent.Data), `"input_tokens":10`)
		require.Len(t, state.publicAliases.Records(), 2, "response and message item aliases must be allocated exactly once")
	})
}
