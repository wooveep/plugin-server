package clientresponse

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/tidwall/gjson"
)

func TestSupplierUsageMapsWithoutInventingUnknownDetails(t *testing.T) {
	for _, tc := range []struct {
		name, format, body string
	}{
		{"openai", Chat, `{"usage":{"prompt_tokens":17,"completion_tokens":5,"total_tokens":22,"prompt_tokens_details":{"cache_write_tokens":3,"cached_tokens":2},"completion_tokens_details":{"reasoning_tokens":4}}}`},
		{"deepseek", Chat, `{"usage":{"prompt_tokens":17,"completion_tokens":5,"total_tokens":22,"prompt_cache_hit_tokens":2,"prompt_cache_miss_tokens":15,"completion_tokens_details":{"reasoning_tokens":4}}}`},
		{"qwen-native", "qwen", `{"usage":{"input_tokens":17,"output_tokens":5,"total_tokens":22,"cached_tokens":2,"output_tokens_details":{"reasoning_tokens":4}}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := ReadUsage([]byte(tc.body), tc.format, false)
			wire := u.wire(Responses)
			if gjson.GetBytes(wire, "input_tokens").Int() != 17 || gjson.GetBytes(wire, "output_tokens").Int() != 5 || gjson.GetBytes(wire, "input_tokens_details.cached_tokens").Int() != 2 || gjson.GetBytes(wire, "output_tokens_details.reasoning_tokens").Int() != 4 {
				t.Fatal("supplier measurement changed", string(wire))
			}
			if tc.name != "openai" && gjson.GetBytes(wire, "input_tokens_details.cache_write_tokens").Exists() {
				t.Fatal("missing write detail became zero")
			}
		})
	}
	for _, raw := range []string{`{"usage":{"prompt_tokens":1}}`, `{"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":4}}`} {
		body, err := Project(Policy{Protocol: Responses, PublicModel: "sales"}, []byte(`{"id":"r","object":"response","status":"completed","output":[]}`), false, ReadUsage([]byte(raw), Chat, false))
		if err != nil || string(gjson.GetBytes(body, "usage").Raw) != "null" {
			t.Fatal("incomplete or conflicting base usage was fabricated", string(body), err)
		}
	}
}

func TestPublicIdentityAndRealReasoningAreSeparateFromSupplierDiagnostics(t *testing.T) {
	raw := []byte(`{"id":"opaque","object":"chat.completion","created":1,"model":"supplier-model","system_fingerprint":"private-fingerprint","vendor_private":"hidden","choices":[{"index":0,"message":{"role":"assistant","content":"supplier-model is quoted content","reasoning_content":"actual reasoning","tool_calls":[{"id":"c","type":"function","function":{"name":"f","arguments":"{\"n\":9007199254740993}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":9007199254740993,"completion_tokens":2}}`)
	before := bytes.Clone(raw)
	body, err := Project(Policy{Protocol: Chat, PublicModel: "sales"}, raw, false, ReadUsage(raw, Chat, false))
	if err != nil || !bytes.Equal(raw, before) || gjson.GetBytes(body, "model").String() != "sales" || gjson.GetBytes(body, "choices.0.message.reasoning_content").String() != "actual reasoning" || gjson.GetBytes(body, "choices.0.message.content").String() != "supplier-model is quoted content" || gjson.GetBytes(body, "usage.prompt_tokens").Raw != "9007199254740993" {
		t.Fatal("public projection changed content or exact quantities", string(body), err)
	}
	if gjson.GetBytes(body, "system_fingerprint").Exists() || gjson.GetBytes(body, "vendor_private").Exists() {
		t.Fatal("supplier diagnostics escaped")
	}
}

func TestDeepSeekTerminalUsageIsSplitOnlyForClientRequestedUsage(t *testing.T) {
	body := []byte("data: {\"id\":\"r\",\"model\":\"supplier\",\"choices\":[{\"index\":0,\"delta\":{\"reasoning_content\":\"thinking\"},\"finish_reason\":null}]}\n\ndata: {\"id\":\"r\",\"model\":\"supplier\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":17,\"completion_tokens\":5,\"total_tokens\":22}}\n\ndata: [DONE]\n\n")
	for _, include := range []bool{false, true} {
		out, err := Project(Policy{Protocol: Chat, PublicModel: "sales", IncludeUsage: include}, body, true, ReadUsage(body, Chat, true))
		if err != nil {
			t.Fatal(err)
		}
		events, _ := Events(out)
		terminal, usageFrames, reasoning := false, 0, false
		for _, event := range events {
			if bytes.Equal(event.Data, []byte("[DONE]")) {
				continue
			}
			if gjson.GetBytes(event.Data, "model").String() != "sales" {
				t.Fatal("stream model leaked")
			}
			terminal = terminal || gjson.GetBytes(event.Data, "choices.0.finish_reason").String() == "stop"
			reasoning = reasoning || gjson.GetBytes(event.Data, "choices.0.delta.reasoning_content").String() == "thinking"
			if gjson.GetBytes(event.Data, "usage.prompt_tokens").Exists() {
				usageFrames++
				if gjson.GetBytes(event.Data, "choices").Raw != "[]" {
					t.Fatal("usage still attached to the terminal content chunk")
				}
			}
			if !include && gjson.GetBytes(event.Data, "usage").Exists() {
				t.Fatal("forced upstream usage changed the public preference")
			}
		}
		if !terminal || !reasoning || include && usageFrames != 1 || !include && usageFrames != 0 {
			t.Fatal("stream content, terminal or usage policy changed", string(out))
		}
	}
}

func TestMessagesThinkingAndResponsesFailureRetainTheirProtocolMeaning(t *testing.T) {
	raw := []byte(`{"id":"m","type":"message","role":"assistant","model":"upstream","content":[{"type":"thinking","thinking":"real","signature":"opaque-signature"},{"type":"text","text":"answer"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":10,"cache_read_input_tokens":2,"cache_creation_input_tokens":3,"output_tokens":4,"service_tier":"standard"}}`)
	out, err := Project(Policy{Protocol: Messages, PublicModel: "sales"}, raw, false, ReadUsage(raw, Messages, false))
	if err != nil || gjson.GetBytes(out, "content.0.signature").String() != "opaque-signature" || gjson.GetBytes(out, "usage.input_tokens").Int() != 10 || gjson.GetBytes(out, "usage.cache_creation_input_tokens").Int() != 3 || gjson.GetBytes(out, "usage.service_tier").Exists() {
		t.Fatal("native Messages content or cache semantics changed", string(out), err)
	}
	failure := []byte(`{"id":"r","object":"response","model":"upstream","status":"failed","output":[],"error":{"code":"server_error","message":"upstream interrupted"},"usage":null}`)
	out, err = Project(Policy{Protocol: Responses, PublicModel: "sales"}, failure, false, Usage{})
	if err != nil || gjson.GetBytes(out, "status").String() != "failed" || gjson.GetBytes(out, "object").String() != "response" {
		t.Fatal("failed Response became a generic success or lost its resource", string(out), err)
	}
	var o map[string]json.RawMessage
	_ = json.Unmarshal(Error(Policy{Protocol: Messages}, []byte(`{"error":{"message":"https://private.example/key=secret","code":"bad"}}`)), &o)
	if string(o["type"]) != `"error"` || bytes.Contains(o["error"], []byte("private.example")) {
		t.Fatal("Messages error shape or diagnostic privacy changed")
	}
}
