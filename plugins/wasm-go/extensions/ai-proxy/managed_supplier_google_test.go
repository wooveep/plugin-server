package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

const managedGoogleFixtureUsage = `{"promptTokenCount":9007199254740993,"toolUsePromptTokenCount":4,"candidatesTokenCount":2,"thoughtsTokenCount":3,"totalTokenCount":9007199254741002,"cachedContentTokenCount":1}`

func TestManagedGeminiOriginalCountersAndTerminalThroughTheSoleSender(t *testing.T) {
	for _, input := range []string{managedProtocolChat, managedProtocolResponses, managedProtocolMessages} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", input, stream), func(t *testing.T) {
				fixture := managedConfigFixture()
				fixture["provider"] = json.RawMessage(`{"type":"gemini","apiTokens":["controlled-upstream-key"],"apiVersion":"v1beta"}`)
				settings := fixture["managed_execution"].(managedSettings)
				settings.InputProtocol, settings.UpstreamProtocol = input, managedProtocolChat
				fixture["managed_execution"] = settings
				h := newManagedHost(t, fixture)
				payload := `"messages":[{"role":"user","content":"hello"}],"max_tokens":12`
				if input == managedProtocolResponses {
					payload = `"input":"hello"`
				}
				original := []byte(fmt.Sprintf(`{"model":"public-model",%s,"stream":%t}`, payload, stream))
				h.start(t, original, bytes.Replace(original, []byte("public-model"), []byte("upstream-model"), 1))
				calls := h.host.GetCalloutAttributesFromContext(h.id)
				if len(calls) != 1 || gjson.GetBytes(calls[0].Body, "contents.0.parts.0.text").String() != "hello" {
					t.Fatal("Gemini SDK content or sole sender was bypassed")
				}
				actual := bytes.Clone(calls[0].Body)
				path, _ := managedSingleHeader(calls[0].Headers, ":path")
				expectedPath := "/v1beta/models/upstream-model:generateContent"
				if stream {
					expectedPath = "/v1beta/models/upstream-model:streamGenerateContent?alt=sse"
				}
				key, _ := managedSingleHeader(calls[0].Headers, "x-goog-api-key")
				authority, _ := managedSingleHeader(calls[0].Headers, ":authority")
				if path != expectedPath || key != "controlled-upstream-key" || authority != "managed-provider.example" {
					t.Fatal("Gemini changed the frozen model, key or exit authority")
				}
				body := []byte(`{"candidates":[{"content":{"parts":[{"text":"hello"}],"role":"model"},"finishReason":"STOP","index":0}],"usageMetadata":` + managedGoogleFixtureUsage + `}`)
				headers := [][2]string{{":status", "200"}, {"content-type", "application/json"}, {"etag", "native-representation"}}
				if stream {
					headers[1][1] = "text/event-stream"
					body = []byte("data: " + strings.Replace(string(body), `,"usageMetadata":`+managedGoogleFixtureUsage, "", 1) + "\n\ndata: {\"usageMetadata\":" + managedGoogleFixtureUsage + "}\n\n")
				}
				h.response(t, headers, body)
				response := h.host.GetSentLocalResponse(h.id)
				if response == nil || response.StatusCode != 200 {
					t.Fatal("verified Gemini response was not converted")
				}
				native, err := newManagedProtocolProfile(input, input)
				if err != nil {
					t.Fatal(err)
				}
				observed := native.ObserveResponse(200, response.Headers, response.Data, true)
				if observed.Acceptance != "accepted" || observed.Usage.Quantities["input"] != "9007199254740997" || observed.Usage.Quantities["output"] != "5" || observed.Usage.Quantities["cache_miss"] != "9007199254740996" {
					t.Fatalf("client lost original thinking/tool/cache counters: %#v", observed)
				}
				records := h.records(t)
				if len(records) != 2 {
					t.Fatal("expected registration and exactly one business attempt")
				}
				fact := decodeManagedFact(t, records[1].raw)
				if fact.OutboundBodyDigest != managedBodyDigest(actual) || fact.ResponseBodyDigest != managedBodyDigest(body) || fact.Usage.Quantities["input"] != "9007199254740997" || fact.Usage.Quantities["output"] != "5" {
					t.Fatal("wire facts were replaced by converted content")
				}
			})
		}
	}
}

func TestManagedGeminiMissingUsageAndUnfinishedStreamStayUnknown(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%t", stream), func(t *testing.T) {
			fixture := managedConfigFixture()
			fixture["provider"] = json.RawMessage(`{"type":"gemini","apiTokens":["controlled-upstream-key"]}`)
			h := newManagedHost(t, fixture)
			original := []byte(fmt.Sprintf(`{"model":"public-model","messages":[{"role":"user","content":"hello"}],"stream":%t}`, stream))
			h.start(t, original, bytes.Replace(original, []byte("public-model"), []byte("upstream-model"), 1))
			body := []byte(`{"candidates":[{"content":{"parts":[{"text":"hello"}]},"finishReason":"STOP"}]}`)
			headers := [][2]string{{":status", "200"}, {"content-type", "application/json"}}
			if stream {
				body = []byte("data: " + string(body) + "\n\n")
				headers[1][1] = "text/event-stream"
			}
			h.response(t, headers, body)
			response := h.host.GetSentLocalResponse(h.id)
			if response == nil || response.StatusCode != 200 || bytes.Contains(response.Data, []byte(`"usage"`)) {
				t.Fatal("Gemini content conversion fabricated zero usage")
			}
			fact := decodeManagedFact(t, h.records(t)[1].raw)
			if fact.Usage.Status != "missing" || fact.Usage.Quantities != nil {
				t.Fatal("missing original counters became reported zero")
			}
		})
	}
	progress := []byte("data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"hello\"}]}}],\"usageMetadata\":" + managedGoogleFixtureUsage + "}\n\n")
	observed := managedGoogleObservation(managedProtocolChat, 200, managedProtocolSSEHeaders, progress, true, 0)
	if observed.BodyComplete || observed.Acceptance == "accepted" || observed.Usage.Status != "missing" {
		t.Fatal("Google progress became a verified terminal")
	}
}

func TestManagedGeminiPartialAndCumulativeUsageUseOriginalLastFrame(t *testing.T) {
	progress := `{"candidates":[{"index":0}],"usageMetadata":{"promptTokenCount":17,"candidatesTokenCount":1,"totalTokenCount":18}}`
	terminal := `{"candidates":[{"index":0,"finishReason":"MAX_TOKENS"}],"usageMetadata":{"promptTokenCount":17,"candidatesTokenCount":2,"totalTokenCount":19}}`
	observed := managedGoogleObservation(managedProtocolChat, 200, managedProtocolSSEHeaders, []byte("data: "+progress+"\n\ndata: "+terminal+"\n\n"), true, 0)
	if observed.Acceptance != "accepted" || observed.ExecutionResult != "partial" || observed.Usage.Quantities["output"] != "2" {
		t.Fatalf("Google cumulative counters were summed or terminal lost: %#v", observed)
	}
}

func TestManagedMoonshotNestedStreamUsageUsesOriginalBytes(t *testing.T) {
	fixture := managedConfigFixture()
	fixture["provider"] = json.RawMessage(`{"type":"moonshot","apiTokens":["controlled-upstream-key"]}`)
	h := newManagedHost(t, fixture)
	original := []byte(`{"model":"public-model","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	h.start(t, original, bytes.Replace(original, []byte("public-model"), []byte("upstream-model"), 1))
	calls := h.host.GetCalloutAttributesFromContext(h.id)
	if len(calls) != 1 {
		t.Fatal("Moonshot introduced a second sender")
	}
	body := []byte("data: {\"model\":\"upstream-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"},\"finish_reason\":\"stop\",\"usage\":{\"prompt_tokens\":9007199254740993,\"completion_tokens\":2,\"total_tokens\":9007199254740995}}]}\n\ndata: [DONE]\n\n")
	h.response(t, [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}, body)
	response := h.host.GetSentLocalResponse(h.id)
	if response == nil || response.StatusCode != 200 {
		t.Fatal("Moonshot response conversion failed")
	}
	profile, err := newManagedProtocolProfile(managedProtocolChat, managedProtocolChat)
	if err != nil {
		t.Fatal(err)
	}
	observed := profile.ObserveResponse(200, response.Headers, response.Data, true)
	fact := decodeManagedFact(t, h.records(t)[1].raw)
	if observed.Acceptance != "accepted" || observed.Usage.Quantities["input"] != "9007199254740993" || fact.ResponseBodyDigest != managedBodyDigest(body) || fact.Usage.Quantities["input"] != "9007199254740993" {
		t.Fatal("Moonshot nested original counters were lost or rounded")
	}
}

func TestManagedGeminiBatchEmbeddingPreservesPositionsWithoutFabricatingUsage(t *testing.T) {
	fixture := managedInternalFixture(t, true)
	fixture["provider"] = json.RawMessage(`{"type":"gemini","apiTokens":["controlled-upstream-key"]}`)
	h := newManagedTextHost(t, fixture)
	original := []byte(`{"model":"public-model","input":["first","second"]}`)
	basis := startManagedInternal(t, h, original, bytes.Replace(original, []byte("public-model"), []byte("upstream-model"), 1))
	checks := h.host.GetCalloutAttributesFromContext(h.id)
	if len(checks) != 1 {
		t.Fatal("original job check missing")
	}
	receipt, err := json.Marshal(internalCheckReceipt(t, h, basis))
	if err != nil {
		t.Fatal(err)
	}
	h.host.CallOnHttpCallResponse(checks[0].CalloutID, [][2]string{{":status", "200"}}, nil, receipt)
	respondManagedInternalHealth(t, h, true)
	calls := h.host.GetCalloutAttributesFromContext(h.id)
	if len(calls) != 1 || gjson.GetBytes(calls[0].Body, "requests.1.content.parts.0.text").String() != "second" {
		t.Fatal("original embedding batch was changed")
	}
	path, _ := managedSingleHeader(calls[0].Headers, ":path")
	if path != "/v1beta/models/upstream-model:batchEmbedContents" {
		t.Fatal("Gemini embedding SDK path was bypassed")
	}
	body := []byte(`{"embeddings":[{"values":[0.25,-0.5]},{"values":[0.5,-0.25]}]}`)
	h.response(t, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, body)
	response := h.host.GetSentLocalResponse(h.id)
	if response == nil || response.StatusCode != 200 || gjson.GetBytes(response.Data, "data.1.index").Int() != 1 || gjson.GetBytes(response.Data, "data.1.embedding.1").Float() != -0.25 || gjson.GetBytes(response.Data, "usage").Exists() {
		t.Fatal("Gemini batch positions changed or converter fabricated usage")
	}
	fact := decodeManagedFact(t, h.records(t)[1].raw)
	if fact.Acceptance != "accepted" || fact.ResponseBodyDigest != managedBodyDigest(body) || fact.Usage.Status != "missing" || fact.Usage.Quantities != nil {
		t.Fatal("embedding counters absent upstream became reported zero")
	}
}

func TestManagedGeminiSeparateTerminalAndRealFunctionCallsAcrossClients(t *testing.T) {
	for _, input := range []string{managedProtocolChat, managedProtocolResponses, managedProtocolMessages} {
		for _, test := range []struct {
			name, parts, reason, expected string
			stream                        bool
		}{
			{"separate-stop", `[{"text":"hello"}]`, "STOP", "succeeded", true},
			{"separate-length", `[{"text":"hello"}]`, "MAX_TOKENS", "partial", true},
			{"tools-json", `[{"text":"checking"},{"functionCall":{"name":"first_tool","args":{"value":9007199254740993}}},{"functionCall":{"name":"second_tool","args":{"value":"second"}}}]`, "STOP", "succeeded", false},
			{"tools-stream", `[{"text":"checking"},{"functionCall":{"name":"first_tool","args":{"value":9007199254740993}}},{"functionCall":{"name":"second_tool","args":{"value":"second"}}}]`, "STOP", "succeeded", true},
			{"tool-first-json", `[{"functionCall":{"name":"first_tool","args":{"value":9007199254740993}}}]`, "STOP", "succeeded", false},
		} {
			t.Run(input+"/"+test.name, func(t *testing.T) {
				fixture := managedConfigFixture()
				fixture["provider"] = json.RawMessage(`{"type":"gemini","apiTokens":["controlled-upstream-key"]}`)
				settings := fixture["managed_execution"].(managedSettings)
				settings.InputProtocol, settings.UpstreamProtocol = input, managedProtocolChat
				fixture["managed_execution"] = settings
				h := newManagedHost(t, fixture)
				payload := `"messages":[{"role":"user","content":"hello"}],"max_tokens":12`
				if input == managedProtocolResponses {
					payload = `"input":"hello"`
				}
				original := []byte(fmt.Sprintf(`{"model":"public-model",%s,"stream":%t}`, payload, test.stream))
				h.start(t, original, bytes.Replace(original, []byte("public-model"), []byte("upstream-model"), 1))
				calls := h.host.GetCalloutAttributesFromContext(h.id)
				if len(calls) != 1 {
					t.Fatal("expected exactly one business send")
				}
				body := []byte(`{"candidates":[{"index":0,"content":{"parts":` + test.parts + `},"finishReason":"` + test.reason + `"}],"usageMetadata":` + managedGoogleFixtureUsage + `}`)
				headers := [][2]string{{":status", "200"}, {"content-type", "application/json"}}
				if test.stream {
					body = []byte("data: {\"candidates\":[{\"index\":0,\"content\":{\"parts\":" + test.parts + "}}]}\n\ndata: {\"candidates\":[{\"index\":0,\"finishReason\":\"" + test.reason + "\"}],\"usageMetadata\":" + managedGoogleFixtureUsage + "}\n\n")
					headers[1][1] = "text/event-stream"
				}
				h.response(t, headers, body)
				response := h.host.GetSentLocalResponse(h.id)
				if response == nil || response.StatusCode != 200 {
					t.Fatal("native content or standalone terminal conversion failed")
				}
				profile, err := newManagedProtocolProfile(input, input)
				if err != nil {
					t.Fatal(err)
				}
				observed := profile.ObserveResponse(200, response.Headers, response.Data, true)
				if observed.Acceptance != "accepted" || observed.ExecutionResult != test.expected || observed.Usage.Quantities["output"] != "5" {
					t.Fatalf("separate terminal or original exact usage lost: %#v", observed)
				}
				if strings.Contains(test.name, "tool") {
					for _, content := range []string{"first_tool", "9007199254740993"} {
						if !bytes.Contains(response.Data, []byte(content)) {
							t.Fatal("actual native functionCall content lost or rounded", content)
						}
					}
					if test.name != "tool-first-json" && (!bytes.Contains(response.Data, []byte("checking")) || !bytes.Contains(response.Data, []byte("second_tool"))) {
						t.Fatal("mixed text and multiple native function calls were lost")
					}
				}
				fact := decodeManagedFact(t, h.records(t)[1].raw)
				if fact.Acceptance != "accepted" || fact.ResponseBodyDigest != managedBodyDigest(body) || fact.Usage.Quantities["output"] != "5" {
					t.Fatal("content repair replaced original wire facts")
				}
			})
		}
	}
}
