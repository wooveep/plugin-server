package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/tidwall/gjson"
)

const managedQwenFixtureProvider = `{"type":"qwen","protocol":"original","apiTokens":["controlled-upstream-key"],"qwenEnableCompatible":false,"qwenEnableSearch":true}`

func TestManagedQwenCompatibleBindingOverridesPreviouslyInitializedNativeCapability(t *testing.T) {
	fixture := managedConfigFixture()
	fixture["provider"] = json.RawMessage(managedQwenFixtureProvider)
	settings := fixture["managed_execution"].(managedSettings)
	settings.InputProtocol, settings.UpstreamProtocol, settings.UpstreamPath = managedProtocolChat, managedProtocolChat, "/compatible-mode/v1/chat/completions"
	fixture["managed_execution"] = settings
	h := newManagedHost(t, fixture)
	original := []byte(`{"model":"public-model","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"https://example.com/image.png"}},{"type":"text","text":"describe"}]}],"stream":true}`)
	h.start(t, original, bytes.Replace(original, []byte("public-model"), []byte("upstream-model"), 1))
	calls := h.host.GetCalloutAttributesFromContext(h.id)
	if len(calls) != 1 {
		t.Fatal("expected exactly one actual business send")
	}
	path, _ := managedSingleHeader(calls[0].Headers, ":path")
	if path != settings.UpstreamPath || !gjson.GetBytes(calls[0].Body, "messages").IsArray() || gjson.GetBytes(calls[0].Body, "input").Exists() {
		t.Fatalf("frozen compatible binding was overwritten by an earlier provider default: path=%q", path)
	}
}

func TestManagedQwenActualBodyAndOriginalUsageThroughOneSender(t *testing.T) {
	for _, input := range []string{managedProtocolChat, managedProtocolResponses, managedProtocolMessages} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", input, stream), func(t *testing.T) {
				fixture := managedConfigFixture()
				fixture["provider"] = json.RawMessage(managedQwenFixtureProvider)
				settings := fixture["managed_execution"].(managedSettings)
				settings.InputProtocol, settings.UpstreamProtocol = input, managedProtocolChat
				fixture["managed_execution"] = settings
				h := newManagedHost(t, fixture)
				payload := `"messages":[{"role":"user","content":"hello"}],"max_tokens":12`
				if input == managedProtocolResponses {
					payload = `"input":"hello"`
				}
				if input == managedProtocolChat && stream {
					payload += `,"stream_options":{"include_usage":true}`
				}
				original := []byte(fmt.Sprintf(`{"model":"public-model",%s,"stream":%t}`, payload, stream))
				mapped := bytes.Replace(original, []byte("public-model"), []byte("upstream-model"), 1)
				h.start(t, original, mapped)
				calls := h.host.GetCalloutAttributesFromContext(h.id)
				if len(calls) != 1 {
					t.Fatalf("expected one business send, got %d", len(calls))
				}
				actual := bytes.Clone(calls[0].Body)
				if gjson.GetBytes(actual, "input.messages.0.content").String() != "hello" || !gjson.GetBytes(actual, "parameters.enable_search").Bool() {
					t.Fatalf("original supplier fields/body were not used: %s", actual)
				}
				path, err := managedSingleHeader(calls[0].Headers, ":path")
				if err != nil || path != "/api/v1/services/aigc/text-generation/generation" {
					t.Fatalf("supplier path: %q %v", path, err)
				}
				authority, _ := managedSingleHeader(calls[0].Headers, ":authority")
				if authority != "managed-provider.example" {
					t.Fatal("supplier changed the frozen exit authority")
				}
				body := []byte(`{"request_id":"qwen-original","output":{"choices":[{"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]},"usage":{"input_tokens":9007199254740993,"output_tokens":2,"total_tokens":9007199254740995,"prompt_tokens_details":{"cached_tokens":1}}}`)
				headers := [][2]string{{":status", "200"}, {"content-type", "application/json"}, {"etag", "original-qwen-body"}}
				if stream {
					headers[1][1] = "text/event-stream"
					progress := strings.Replace(string(body), `"stop"`, `"null"`, 1)
					body = []byte("event: result\n:HTTP_STATUS/200\ndata: " + progress + "\n\nevent: result\ndata: " + string(body) + "\n\n")
				}
				h.response(t, headers, body)
				response := h.host.GetSentLocalResponse(h.id)
				if response == nil || response.StatusCode != 200 {
					t.Fatal("native response was not converted for the client")
				}
				if _, err := managedSingleHeader(response.Headers, "etag"); err == nil {
					t.Fatal("converted representation retained native etag")
				}
				native, err := newManagedProtocolProfile(input, input)
				if err != nil {
					t.Fatal(err)
				}
				observed := native.ObserveResponse(200, response.Headers, response.Data, true)
				publicUsage := clientresponse.ReadUsage(response.Data, input, stream)
				inputCount, expectedInput := publicUsage.Input, "9007199254740993"
				if input == managedProtocolMessages {
					inputCount, expectedInput = publicUsage.MessagesInput, "9007199254740992"
				}
				if observed.Acceptance != "accepted" || string(inputCount) != expectedInput || string(publicUsage.InputDetails["cached_tokens"]) != "1" {
					t.Fatalf("client conversion lost exact usage: %#v", observed)
				}
				records := h.records(t)
				if len(records) != 2 {
					t.Fatalf("expected registration and one attempt, got %d", len(records))
				}
				for _, record := range records {
					if record.messageType == "execution_registration" {
						continue
					}
					attempt := decodeManagedFact(t, record.raw)
					if attempt.Kind != "upstream_attempt" || attempt.OutboundBodyDigest != managedBodyDigest(actual) || attempt.ResponseBodyDigest != managedBodyDigest(body) || attempt.Usage.Quantities["input"] != "9007199254740993" || attempt.Usage.Quantities["output"] != "2" {
						t.Fatalf("actual wire facts were replaced by conversion data: %#v", attempt)
					}
				}
			})
		}
	}
}

func TestManagedNativeQwenMissingUsageNeverUsesConverterZeros(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%t", stream), func(t *testing.T) {
			fixture := managedConfigFixture()
			fixture["provider"] = json.RawMessage(managedQwenFixtureProvider)
			h := newManagedHost(t, fixture)
			original := []byte(fmt.Sprintf(`{"model":"public-model","messages":[{"role":"user","content":"hello"}],"stream":%t}`, stream))
			h.start(t, original, bytes.Replace(original, []byte("public-model"), []byte("upstream-model"), 1))
			body := []byte(`{"output":{"choices":[{"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}}`)
			headers := [][2]string{{":status", "200"}, {"content-type", "application/json"}}
			if stream {
				body = []byte("data: " + string(body) + "\n\n")
				headers[1][1] = "text/event-stream"
			}
			h.response(t, headers, body)
			response := h.host.GetSentLocalResponse(h.id)
			if response == nil || response.StatusCode != 200 || bytes.Contains(response.Data, []byte(`"usage"`)) {
				t.Fatal("content converter fabricated usage")
			}
			for _, record := range h.records(t) {
				if record.messageType == "execution_registration" {
					continue
				}
				attempt := decodeManagedFact(t, record.raw)
				if attempt.Usage.Status != "estimated" || attempt.Usage.Source != "estimator" || attempt.Usage.Quantities["input"] == "0" {
					t.Fatal("missing native counters were not estimated from real input")
				}
			}
		})
	}
}

func TestManagedSupplierHeadersAndBodyReachTheSoleBusinessSender(t *testing.T) {
	for _, test := range []struct {
		name, config, protocol, path, authName, authValue string
	}{
		{"qwen-compatible", `{"type":"qwen","qwenEnableCompatible":true}`, managedProtocolChat, "/compatible-mode/v1/chat/completions", "authorization", "Bearer controlled-upstream-key"},
		{"deepseek", `{"type":"deepseek"}`, managedProtocolChat, "/v1/chat/completions", "authorization", "Bearer controlled-upstream-key"},
		{"groq", `{"type":"groq"}`, managedProtocolChat, "/openai/v1/chat/completions", "authorization", "Bearer controlled-upstream-key"},
		{"grok", `{"type":"grok"}`, managedProtocolChat, "/v1/chat/completions", "authorization", "Bearer controlled-upstream-key"},
		{"yi", `{"type":"yi"}`, managedProtocolChat, "/v1/chat/completions", "authorization", "Bearer controlled-upstream-key"},
		{"baichuan", `{"type":"baichuan"}`, managedProtocolChat, "/v1/chat/completions", "authorization", "Bearer controlled-upstream-key"},
		{"ai360", `{"type":"ai360"}`, managedProtocolChat, "/v1/chat/completions", "authorization", "controlled-upstream-key"},
		{"github", `{"type":"github"}`, managedProtocolChat, "/chat/completions", "authorization", "controlled-upstream-key"},
		{"stepfun", `{"type":"stepfun"}`, managedProtocolChat, "/v1/chat/completions", "authorization", "Bearer controlled-upstream-key"},
		{"mistral", `{"type":"mistral"}`, managedProtocolChat, "/custom/provider/answer", "authorization", "Bearer controlled-upstream-key"},
		{"longcat", `{"type":"longcat"}`, managedProtocolChat, "/v1/chat/completions", "authorization", "Bearer controlled-upstream-key"},
		{"fireworks", `{"type":"fireworks"}`, managedProtocolChat, "/v1/chat/completions", "authorization", "Bearer controlled-upstream-key"},
		{"together-ai", `{"type":"together-ai"}`, managedProtocolChat, "/v1/chat/completions", "authorization", "Bearer controlled-upstream-key"},
		{"doubao", `{"type":"doubao"}`, managedProtocolChat, "/api/v3/chat/completions", "authorization", "Bearer controlled-upstream-key"},
		{"openrouter", `{"type":"openrouter"}`, managedProtocolChat, "/api/v1/chat/completions", "authorization", "Bearer controlled-upstream-key"},
		{"baidu", `{"type":"baidu"}`, managedProtocolChat, "/v2/chat/completions", "authorization", "Bearer controlled-upstream-key"},
		{"ollama", `{"type":"ollama","ollamaServerHost":"frozen-ollama","ollamaServerPort":11434}`, managedProtocolChat, "/v1/chat/completions", "authorization", ""},
		{"vllm", `{"type":"vllm","vllmCustomUrl":"https://wire.example/egress"}`, managedProtocolChat, "/egress/chat/completions", "authorization", "Bearer controlled-upstream-key"},
		{"vllm-full", `{"type":"vllm","vllmCustomUrl":"https://wire.example/egress/custom/completions"}`, managedProtocolChat, "/egress/custom/completions", "authorization", "Bearer controlled-upstream-key"},
		{"zhipuai", `{"type":"zhipuai"}`, managedProtocolChat, "/api/paas/v4/chat/completions", "authorization", "Bearer controlled-upstream-key"},
		{"zhipuai-code", `{"type":"zhipuai","zhipuCodePlanMode":true,"zhipuDomain":"api.z.ai"}`, managedProtocolChat, "/api/coding/paas/v4/chat/completions", "authorization", "Bearer controlled-upstream-key"},
		{"cloudflare", `{"type":"cloudflare","cloudflareAccountId":"frozen-account"}`, managedProtocolChat, "/client/v4/accounts/frozen-account/ai/v1/chat/completions", "authorization", "Bearer controlled-upstream-key"},
		{"hunyuan-compatible", `{"type":"hunyuan"}`, managedProtocolChat, "/v1/chat/completions", "authorization", "Bearer controlled-upstream-key"},
		{"minimax-v2", `{"type":"minimax","minimaxApiType":"v2"}`, managedProtocolChat, "/v1/text/chatcompletion_v2", "authorization", "Bearer controlled-upstream-key"},
		{"azure", `{"type":"azure","azureServiceUrl":"https://wire.example/openai/deployments/upstream-model?api-version=2024-10-21"}`, managedProtocolChat, "/openai/deployments/upstream-model/chat/completions?api-version=2024-10-21", "api-key", "controlled-upstream-key"},
		{"azure-frozen-deployment", `{"type":"azure","azureServiceUrl":"https://wire.example/openai/deployments/other-deployment?api-version=2024-10-21"}`, managedProtocolChat, "/openai/deployments/other-deployment/chat/completions?api-version=2024-10-21", "api-key", "controlled-upstream-key"},
		{"azure-frozen-full-url", `{"type":"azure","azureServiceUrl":"https://wire.example/openai/deployments/other-deployment/chat/completions?api-version=2024-10-21"}`, managedProtocolChat, "/openai/deployments/other-deployment/chat/completions?api-version=2024-10-21", "api-key", "controlled-upstream-key"},
		{"claude", `{"type":"claude","claudeVersion":"fixture-version"}`, managedProtocolMessages, "/v1/messages", "x-api-key", "controlled-upstream-key"},
		{"claude-code", `{"type":"claude","claudeCodeMode":true,"claudeVersion":"fixture-version"}`, managedProtocolMessages, "/v1/messages?beta=true", "authorization", "Bearer controlled-upstream-key"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := managedConfigFixture()
			fixture["provider"] = json.RawMessage(strings.TrimSuffix(test.config, "}") + `,"apiTokens":["controlled-upstream-key"]}`)
			settings := fixture["managed_execution"].(managedSettings)
			settings.InputProtocol, settings.UpstreamProtocol = test.protocol, test.protocol
			fixture["managed_execution"] = settings
			h := newManagedHost(t, fixture)
			h.start(t, []byte(managedOriginalBody), []byte(managedMappedBody))
			calls := h.host.GetCalloutAttributesFromContext(h.id)
			if len(calls) != 1 || gjson.GetBytes(calls[0].Body, "model").String() != "upstream-model" {
				t.Fatal("supplier changed the selected model or send count")
			}
			actual := bytes.Clone(calls[0].Body)
			for name, expected := range map[string]string{":path": test.path, ":authority": "managed-provider.example", test.authName: test.authValue} {
				value, err := managedSingleHeader(calls[0].Headers, name)
				if (err != nil && expected != "") || value != expected {
					t.Fatalf("%s was not prepared by the original supplier adapter", name)
				}
			}
			if test.protocol == managedProtocolMessages {
				version, _ := managedSingleHeader(calls[0].Headers, "anthropic-version")
				if version != "fixture-version" {
					t.Fatal("supplier-specific version was lost")
				}
			}
			body := []byte(managedSuccessfulBody)
			if test.protocol == managedProtocolMessages {
				body = []byte(`{"type":"message","model":"upstream-model","stop_reason":"end_turn","content":[{"type":"text","text":"hello"}],"usage":{"input_tokens":9007199254740993,"output_tokens":2}}`)
			}
			h.response(t, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, body)
			response := h.host.GetSentLocalResponse(h.id)
			if response == nil || response.StatusCode != 200 || gjson.GetBytes(response.Data, "model").String() != "public-model" || !bytes.Contains(response.Data, []byte("hello")) || bytes.Contains(response.Data, []byte(`"model":"upstream-model"`)) {
				t.Fatal("public identity or actual content changed")
			}
			records := h.records(t)
			if len(records) != 2 {
				t.Fatal("expected registration and exactly one attempt")
			}
			fact := decodeManagedFact(t, records[1].raw)
			if fact.Acceptance != "accepted" || fact.OutboundBodyDigest != managedBodyDigest(actual) || fact.ResponseBodyDigest != managedBodyDigest(body) || fact.Usage.Quantities["input"] != "9007199254740993" {
				t.Fatal("supplier wire bytes or original counters were lost")
			}
		})
	}
}

func TestManagedNativeQwenEmbeddingUsesOriginalWireAndInputOnlyUsage(t *testing.T) {
	fixture := managedInternalFixture(t, true)
	fixture["provider"] = json.RawMessage(managedQwenFixtureProvider)
	h := newManagedTextHost(t, fixture)
	original := []byte(`{"model":"public-model","input":"hello"}`)
	mapped := bytes.Replace(original, []byte("public-model"), []byte("upstream-model"), 1)
	basis := startManagedInternal(t, h, original, mapped)
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
	if len(calls) != 1 || gjson.GetBytes(calls[0].Body, "input.texts.0").String() != "hello" || gjson.GetBytes(calls[0].Body, "model").String() != "upstream-model" {
		t.Fatal("native embedding body was not prepared")
	}
	actual := bytes.Clone(calls[0].Body)
	path, _ := managedSingleHeader(calls[0].Headers, ":path")
	if path != "/api/v1/services/embeddings/text-embedding/text-embedding" {
		t.Fatal("native embedding path was lost")
	}
	body := []byte(`{"output":{"embeddings":[{"text_index":0,"embedding":[0.25,-0.5]}]},"usage":{"total_tokens":9007199254740993}}`)
	h.response(t, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, body)
	response := h.host.GetSentLocalResponse(h.id)
	if response == nil || response.StatusCode != 200 || gjson.GetBytes(response.Data, "data.0.index").Int() != 0 || gjson.GetBytes(response.Data, "data.0.embedding.1").Float() != -0.5 || gjson.GetBytes(response.Data, "usage.prompt_tokens").Raw != "9007199254740993" || gjson.GetBytes(response.Data, "usage.completion_tokens").Exists() {
		t.Fatal("native vectors/index or exact input-only usage changed")
	}
	records := h.records(t)
	if len(records) != 2 {
		t.Fatal("expected registration and exactly one embedding attempt")
	}
	fact := decodeManagedFact(t, records[1].raw)
	if fact.SchemaVersion != "billing-facts.v2" || fact.Acceptance != "accepted" || fact.OutboundBodyDigest != managedBodyDigest(actual) || fact.ResponseBodyDigest != managedBodyDigest(body) || fact.Usage.Quantities["input"] != "9007199254740993" || len(fact.Usage.Quantities) != 1 {
		t.Fatal("original embedding wire facts were lost")
	}
}

func TestManagedNativeQwenUsageUsesLastFrameAndPreservesUnknownTruncation(t *testing.T) {
	progress := `{"output":{"choices":[{"finish_reason":"null"}]},"usage":{"input_tokens":17,"output_tokens":1,"total_tokens":18,"cached_tokens":5}}`
	terminal := strings.Replace(progress, `"null"`, `"stop"`, 1)
	terminal = strings.Replace(terminal, `"output_tokens":1,"total_tokens":18`, `"output_tokens":2,"total_tokens":19`, 1)
	body := []byte("data: " + progress + "\n\ndata: " + terminal + "\n\n")
	observed := managedQwenObservation(managedProtocolChat, 200, managedProtocolSSEHeaders, body, true, 0)
	if observed.Acceptance != "accepted" || observed.Usage.Quantities["input"] != "17" || observed.Usage.Quantities["output"] != "2" || observed.Usage.Quantities["cache_hit"] != "5" {
		t.Fatalf("cumulative counters were added or lost: %#v", observed)
	}
	incomplete := managedQwenObservation(managedProtocolChat, 200, managedProtocolSSEHeaders, []byte("data: "+progress+"\n\n"), true, 0)
	if incomplete.BodyComplete || incomplete.Usage.Status != "missing" || incomplete.Acceptance == "accepted" {
		t.Fatal("unterminated native result became complete")
	}
}

func TestManagedNativeQwenEmbeddingFactsKeepOriginalIndexesAndInput(t *testing.T) {
	body := []byte(`{"output":{"embeddings":[{"text_index":0,"embedding":[0.25,-0.5]}]},"usage":{"total_tokens":9007199254740993}}`)
	observed := managedQwenObservation(managedProtocolEmbeddings, 200, managedProtocolJSONHeaders, body, true, 1)
	if observed.Acceptance != "accepted" || observed.Usage.Quantities["input"] != "9007199254740993" || len(observed.Usage.Quantities) != 1 {
		t.Fatalf("native embedding evidence changed: %#v", observed)
	}
	bad := bytes.Replace(body, []byte(`"text_index":0`), []byte(`"text_index":1`), 1)
	if managedQwenObservation(managedProtocolEmbeddings, 200, managedProtocolJSONHeaders, bad, true, 1).Acceptance == "accepted" {
		t.Fatal("wrong original embedding index accepted")
	}
	if observed.Usage.Source != "provider" || observed.Usage.Unit != "token" || executionfacts.NativeEmbeddingOperation != "text_embedding" {
		t.Fatal("embedding source changed")
	}
}
