package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

func startSemanticContextRequest(t *testing.T, clientProtocol string, body []byte) (test.TestHost, map[string]interface{}) {
	t.Helper()
	cfg, err := sjson.SetBytes(nativeCacheTestConfig(t), "client_protocol", clientProtocol)
	require.NoError(t, err)
	host, status := test.NewTestHost(cfg)
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName("diagnostic-route"))
	require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(nativeCacheTestHeaders(nativeCacheTestCaller())))
	require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(body))
	require.Equal(t, body, host.GetRequestBody(), "semantic projection never rewrites the real request")
	host.CallOnRedisCall(0, test.CreateRedisRespNull())
	calls := host.GetHttpCalloutAttributes()
	require.Len(t, calls, 1)
	return host, nativeCacheDecodeObject(t, calls[0].Body)
}

func TestSemanticContextSelectedSlotsRetainWholeMultimodalContext(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for _, tc := range []struct {
			name, clientProtocol, protocolName, body, projected, slots string
			queryPath, attachmentPath, contextPath, toolPath           string
		}{
			{
				name: "chat", clientProtocol: "openai_chat", protocolName: "chat_completions",
				body:      `{"model":"public-alias","messages":[{"role":"system","content":"shared text"},{"role":"user","content":[{"type":"text","text":"shared text"},{"type":"image_url","image_url":{"url":"data:image/png;base64,QQ==","detail":"high"}},{"type":"text","text":"?"}]}],"tools":[{"type":"function","function":{"name":"shared text","parameters":{"type":"object"}}}],"precision":9007199254740993.0000000001,"stream":false}`,
				projected: `{"model":"public-alias","messages":[{"role":"system","content":"shared text"},{"role":"user","content":[{"type":"text","text":""},{"type":"image_url","image_url":{"url":"data:image/png;base64,QQ==","detail":"high"}},{"type":"text","text":""}]}],"tools":[{"type":"function","function":{"name":"shared text","parameters":{"type":"object"}}}],"precision":9007199254740993.0000000001,"stream":false}`,
				slots:     `["/messages/1/content/0/text","/messages/1/content/2/text"]`,
				queryPath: "messages.1.content.0.text", attachmentPath: "messages.1.content.1.image_url.url", contextPath: "messages.0.content", toolPath: "tools.0.function.name",
			},
			{
				name: "messages", clientProtocol: "anthropic_messages", protocolName: "messages",
				body:      `{"model":"public-alias","system":"shared text","messages":[{"role":"user","content":[{"type":"text","text":"shared text"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"QQ=="}},{"type":"text","text":"?"}]}],"tools":[{"name":"shared text","input_schema":{"type":"object"}}],"precision":9007199254740993.0000000001,"stream":false}`,
				projected: `{"model":"public-alias","system":"shared text","messages":[{"role":"user","content":[{"type":"text","text":""},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"QQ=="}},{"type":"text","text":""}]}],"tools":[{"name":"shared text","input_schema":{"type":"object"}}],"precision":9007199254740993.0000000001,"stream":false}`,
				slots:     `["/messages/0/content/0/text","/messages/0/content/2/text"]`,
				queryPath: "messages.0.content.0.text", attachmentPath: "messages.0.content.1.source.data", contextPath: "system", toolPath: "tools.0.name",
			},
			{
				name: "responses", clientProtocol: "openai_responses", protocolName: "responses",
				body:      `{"model":"public-alias","instructions":"shared text","input":[{"role":"user","content":[{"type":"input_text","text":"shared text"},{"type":"input_image","file_id":"file-A","image_url":"data:image/png;base64,QQ=="},{"type":"input_text","text":"?"}]}],"tools":[{"type":"function","name":"shared text","parameters":{"type":"object"}}],"precision":9007199254740993.0000000001,"stream":false}`,
				projected: `{"model":"public-alias","instructions":"shared text","input":[{"role":"user","content":[{"type":"input_text","text":""},{"type":"input_image","file_id":"file-A","image_url":"data:image/png;base64,QQ=="},{"type":"input_text","text":""}]}],"tools":[{"type":"function","name":"shared text","parameters":{"type":"object"}}],"precision":9007199254740993.0000000001,"stream":false}`,
				slots:     `["/input/0/content/0/text","/input/0/content/2/text"]`,
				queryPath: "input.0.content.0.text", attachmentPath: "input.0.content.1.file_id", contextPath: "instructions", toolPath: "tools.0.name",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				// An independent explicit projection proves that only the selected
				// text values are masked; equal text in system/tools is retained.
				expected, err := mfjson.Digest([]byte(`{"schema_version":"ai-cache-semantic-context.v1","protocol":"` + tc.protocolName + `","query_text_slots":` + tc.slots + `,"request":` + tc.projected + `}`))
				require.NoError(t, err)
				for _, variant := range []string{"original", "paraphrase", "attachment", "system", "tool", "large number", "equivalent number", "canonical order"} {
					t.Run(variant, func(t *testing.T) {
						body := []byte(tc.body)
						switch variant {
						case "paraphrase":
							body, err = sjson.SetBytes(body, tc.queryPath, "a different question")
						case "attachment":
							body, err = sjson.SetBytes(body, tc.attachmentPath, "attachment-B")
						case "system":
							body, err = sjson.SetBytes(body, tc.contextPath, "different instructions")
						case "tool":
							body, err = sjson.SetBytes(body, tc.toolPath, "different tool")
						case "large number":
							body, err = sjson.SetRawBytes(body, "precision", []byte(`9007199254740992.0000000001`))
						case "equivalent number":
							body, err = sjson.SetRawBytes(body, "precision", []byte(`90071992547409930000000001e-10`))
						case "canonical order":
							body, err = mfjson.Canonicalize(body)
						}
						require.NoError(t, err)
						host, lookup := startSemanticContextRequest(t, tc.clientProtocol, body)
						defer host.Reset()
						actual, ok := lookup["semantic_context_digest"].(string)
						require.True(t, ok)
						if variant == "attachment" || variant == "system" || variant == "tool" || variant == "large number" {
							require.NotEqual(t, expected, actual)
						} else {
							require.Equal(t, expected, actual)
						}
						require.NotEmpty(t, lookup["request_digest"])
						if variant == "original" {
							require.Equal(t, "shared text?", lookup["semantic_query_text"])
							host.CallOnHttpCall([][2]string{{":status", "200"}}, []byte(`{"data":{"decision":"miss"}}`))
							host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
							response := map[string]string{
								"chat":      `{"choices":[{"message":{"content":"answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":7,"completion_tokens":3}}`,
								"messages":  `{"content":[{"type":"text","text":"answer"}],"stop_reason":"end_turn","usage":{"input_tokens":7,"output_tokens":3}}`,
								"responses": `{"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"answer"}]}],"usage":{"input_tokens":7,"output_tokens":3}}`,
							}
							host.CallOnHttpResponseBody([]byte(response[tc.name]))
							var event map[string]interface{}
							for _, call := range host.GetRedisCalloutAttributes() {
								command, ok := thinResponseCaptureCommand(t, call.Query)
								if !ok || len(command) < 5 || !strings.EqualFold(command[0], "xadd") {
									continue
								}
								start := thinResponseCaptureXADDFieldStart(t, command)
								for i := start; i+1 < len(command); i += 2 {
									if command[i] == "event" {
										event = nativeCacheDecodeObject(t, []byte(command[i+1]))
									}
								}
							}
							require.NotNil(t, event)
							require.Equal(t, actual, event["semantic_context_digest"])
							require.Equal(t, lookup["request_digest"], event["request_digest"])
							require.Equal(t, lookup["native_identity"], event["native_identity"])
						}
					})
				}
			})
		}
	})
}

func TestSemanticContextAbsentSlotsKeepExactLookupOnly(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for _, tc := range []struct{ name, clientProtocol, body string }{
			{"chat attachment only", "openai_chat", `{"model":"public-alias","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"image-A"}}]}]}`},
			{"messages attachment only", "anthropic_messages", `{"model":"public-alias","messages":[{"role":"user","content":[{"type":"image","source":{"type":"base64","data":"QQ=="}}]}]}`},
			{"responses attachment only", "openai_responses", `{"model":"public-alias","input":[{"role":"user","content":[{"type":"input_image","file_id":"file-A"}]}]}`},
			{"responses latest attachment only after old text", "openai_responses", `{"model":"public-alias","input":[{"role":"user","content":"old question"},{"role":"user","content":[{"type":"input_image","file_id":"file-A"}]}]}`},
			{"blank", "openai_chat", `{"model":"public-alias","messages":[{"role":"user","content":"  "}]}`},
			{"unrecognized content", "openai_chat", `{"model":"public-alias","messages":[{"role":"user","content":{"text":"question"}}]}`},
			{"duplicate key", "openai_chat", `{"model":"public-alias","messages":[{"role":"user","content":"question"}],"precision":1,"precision":2}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				host, lookup := startSemanticContextRequest(t, tc.clientProtocol, []byte(tc.body))
				defer host.Reset()
				require.NotEmpty(t, lookup["request_digest"])
				require.NotContains(t, lookup, "semantic_context_digest")
				require.NotContains(t, lookup, "semantic_query_text")
			})
		}
	})
}

func TestSemanticContextScalarAndLatestUserSelection(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for _, tc := range []struct{ clientProtocol, protocolName, body, projected, slots string }{
			{"openai_chat", "chat_completions", `{"model":"public-alias","messages":[{"role":"user","content":"unchanged history"},{"role":"system","content":"question"},{"role":"user","content":"question"}]}`, `{"model":"public-alias","messages":[{"role":"user","content":"unchanged history"},{"role":"system","content":"question"},{"role":"user","content":""}]}`, `["/messages/2/content"]`},
			{"anthropic_messages", "messages", `{"model":"public-alias","messages":[{"role":"user","content":"question"}]}`, `{"model":"public-alias","messages":[{"role":"user","content":""}]}`, `["/messages/0/content"]`},
			{"openai_responses", "responses", `{"model":"public-alias","instructions":"question","input":"question"}`, `{"model":"public-alias","instructions":"question","input":""}`, `["/input"]`},
		} {
			t.Run(tc.clientProtocol, func(t *testing.T) {
				host, lookup := startSemanticContextRequest(t, tc.clientProtocol, []byte(tc.body))
				defer host.Reset()
				expected, err := mfjson.Digest([]byte(`{"schema_version":"ai-cache-semantic-context.v1","protocol":"` + tc.protocolName + `","query_text_slots":` + tc.slots + `,"request":` + tc.projected + `}`))
				require.NoError(t, err)
				require.Equal(t, "question", lookup["semantic_query_text"])
				require.Equal(t, expected, lookup["semantic_context_digest"])
			})
		}
	})
}

func TestSemanticContextReplayRequiresCurrentProofAndOriginalScope(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for _, variant := range []string{"semantic", "exact without proof", "missing proof", "legacy semantic rewritten digest", "legacy semantic materialized key", "wrong lookup", "wrong context", "missing source context", "other source context", "no current context", "other tenant", "other principal", "other route", "other model", "other policy", "other protocol", "expired", "unsafe", "inactive", "stream mismatch", "materialized key"} {
			t.Run(variant, func(t *testing.T) {
				body := []byte(nativeCacheTestBody)
				if variant == "no current context" {
					body = []byte(`{"model":"public-alias","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"image-A"}}]}]}`)
				}
				host, lookup := startSemanticContextRequest(t, "openai_chat", body)
				defer host.Reset()
				queryDigest := lookup["request_digest"].(string)
				contextDigest, _ := lookup["semantic_context_digest"].(string)
				if variant == "no current context" {
					require.Empty(t, contextDigest)
					contextDigest = "sha256:" + strings.Repeat("c", 64)
				} else {
					require.NotEmpty(t, contextDigest)
				}
				adapter, _ := thinProtocolAdapterForKind(protocol.ProtocolChatCompletions)
				source, err := adapter.BuildCacheDigest(protocol.RequestParseInput{Body: []byte(`{"model":"public-alias","messages":[{"role":"user","content":"how is the weather?"}]}`)})
				require.NoError(t, err)
				require.NotEqual(t, queryDigest, source.Digest)
				caller := nativeCacheTestCaller()
				record := nativeCacheDecodeObject(t, []byte(validThinConsoleReplayRecord(t, map[string]interface{}{
					"gateway_tenant": caller.TenantID, "gateway_consumer": caller.PrincipalID,
					"gateway_route": nativeCacheTestRoute, "gateway_model": "public-alias",
					"cache_policy_version": "9007199254740993", "request_digest": source.Digest,
					"semantic_context_digest": contextDigest,
				})))
				proof := map[string]string{"lookup_request_digest": queryDigest, "semantic_context_digest": contextDigest}
				data := map[string]interface{}{"decision": "hit", "reason": "semantic_replay", "replay_record": record, "semantic_match": proof}
				switch variant {
				case "exact without proof":
					record["request_digest"] = queryDigest
					delete(record, "semantic_context_digest")
					delete(data, "semantic_match")
					data["reason"] = "precomputed_replay"
				case "missing proof":
					delete(data, "semantic_match")
				case "legacy semantic rewritten digest", "legacy semantic materialized key":
					record["request_digest"] = queryDigest
					delete(record, "semantic_context_digest")
					delete(data, "semantic_match")
					if variant == "legacy semantic materialized key" {
						data["materialized_key"] = "cache:legacy-semantic-source"
					}
				case "wrong lookup":
					proof["lookup_request_digest"] = source.Digest
				case "wrong context":
					proof["semantic_context_digest"] = "sha256:" + strings.Repeat("a", 64)
				case "missing source context":
					delete(record, "semantic_context_digest")
				case "other source context":
					record["semantic_context_digest"] = "sha256:" + strings.Repeat("b", 64)
				case "other tenant":
					record["gateway_tenant"] = "other-tenant"
				case "other principal":
					record["gateway_consumer"] = "other-principal"
				case "other route":
					record["gateway_route"] = "other-route"
				case "other model":
					record["gateway_model"] = "other-model"
				case "other policy":
					record["cache_policy_version"] = "other-policy"
				case "other protocol":
					record["protocol"] = "responses"
				case "expired":
					record["soft_expires_at"] = time.Now().Add(-time.Minute).Format(time.RFC3339)
				case "unsafe":
					record["safe_to_replay"] = false
				case "inactive":
					record["replay_status"] = "quarantined"
				case "stream mismatch":
					record["stream_mode"] = "stream"
				case "materialized key":
					data["materialized_key"] = "cache:source-key"
				}
				encoded, err := json.Marshal(map[string]interface{}{"data": data})
				require.NoError(t, err)
				host.CallOnHttpCall([][2]string{{":status", "200"}}, encoded)
				if variant == "semantic" || variant == "exact without proof" {
					require.NotNil(t, host.GetLocalResponse())
					require.Equal(t, uint32(200), host.GetLocalResponse().StatusCode)
				} else {
					requireThinConsoleFailOpen(t, host)
				}
				require.Empty(t, host.GetRedisCalloutAttributes(), "semantic replay cannot follow a source Redis key")
				if variant != "exact without proof" && !strings.HasPrefix(variant, "legacy semantic") {
					require.Equal(t, source.Digest, record["request_digest"], "source provenance remains its original exact request digest")
				}
			})
		}
	})
}
