package main

import (
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/fixedtools"
)

func TestQwenChatPlatformSearchCountFromTerminalUsage(t *testing.T) {
	state := &managedExecutionState{
		config: &managedExecutionConfig{
			settings: managedSettings{ToolPolicy: &managedToolSettings{Supplier: "qwen"}},
			profile:  managedProtocolProfile{UpstreamProtocol: managedProtocolChat},
		},
		requestedTools: map[fixedtools.Tool]bool{fixedtools.WebSearch: true, fixedtools.WebExtractor: true},
	}
	for _, stream := range []bool{false, true} {
		for _, tc := range []struct {
			name, usage, status, count string
			interrupted                bool
		}{
			{"one", `{"x_tools":{"search":{"count":1}}}`, "reported", "1", false},
			{"zero", `{"x_tools":{"search":{"count":0}}}`, "known_zero", "0", false},
			{"missing", `{}`, "missing", "", false},
			{"negative", `{"x_tools":{"search":{"count":-1}}}`, "missing", "", false},
			{"fraction", `{"x_tools":{"search":{"count":1.5}}}`, "missing", "", false},
			{"invalid", `{"x_tools":{"search":{"count":"invalid"}}}`, "missing", "", false},
			{"null", `{"x_tools":{"search":{"count":null}}}`, "missing", "", false},
			{"interrupted", `{"x_tools":{"search":{"count":1}}}`, "missing", "", true},
		} {
			t.Run(tc.name+map[bool]string{false: "/json", true: "/sse"}[stream], func(t *testing.T) {
				body := []byte(`{"choices":[{"finish_reason":"stop"}],"usage":` + tc.usage + `}`)
				contentType := "application/json"
				if stream {
					contentType = "text/event-stream"
					body = []byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: {\"choices\":[],\"usage\":" + tc.usage + "}\n\n")
					if !tc.interrupted {
						body = append(body, []byte("data: [DONE]\n\n")...)
					}
				}
				outcome := executionfacts.AttemptOutcome{SendState: "observed_sent", ExecutionResult: "succeeded", BodyComplete: !tc.interrupted}
				value := managedToolUsage(state, body, outcome, [][2]string{{"content-type", contentType}})
				search := value[fixedtools.WebSearch]
				if search.Status != tc.status || search.Count != tc.count {
					t.Fatalf("terminal platform count mismatch: %#v", search)
				}
				if tc.count != "" && (search.Source != "provider" || search.ReasonCode != "platform_qwen_openai_chat_usage_x_tools_search_count") {
					t.Fatalf("platform rule provenance lost: %#v", search)
				}
				if value[fixedtools.WebExtractor].Status != "missing" {
					t.Fatal("Chat search metering changed another tool's unknown state")
				}
			})
		}
	}
	outcome := executionfacts.AttemptOutcome{SendState: "observed_sent", ExecutionResult: "succeeded", BodyComplete: true}
	early := "data: {\"choices\":[{\"delta\":{\"content\":\"working\"}}],\"usage\":{\"x_tools\":{\"search\":{\"count\":7}}}}\n\n"
	for _, tc := range []struct{ tail, count string }{
		{`{"choices":[{"finish_reason":"stop"}]}`, ""},
		{`{"choices":[],"usage":{"x_tools":{"search":{"count":1}}}}`, "1"},
	} {
		value := managedToolUsage(state, []byte(early+"data: "+tc.tail+"\n\ndata: [DONE]\n\n"), outcome, [][2]string{{"content-type", "text/event-stream"}})[fixedtools.WebSearch]
		if value.Count != tc.count {
			t.Fatalf("incremental count was retained or summed: %#v", value)
		}
	}
}
