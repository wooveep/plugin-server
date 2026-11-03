package fixedtools

import (
	"encoding/json"
	"testing"
)

func TestFixedToolFilteringKeepsFunctionsAndValidChoicesAcrossRetries(t *testing.T) {
	body := []byte(`{"tools":[{"type":"web_search"},{"type":"code_interpreter"},{"type":"function","name":"code_interpreter","parameters":{}}],"tool_choice":{"type":"code_interpreter"},"input":"hello"}`)
	requested, err := Inspect(body, "openai_responses", nil)
	if err != nil || len(requested) != 2 {
		t.Fatalf("unexpected fixed tools: %v %v", requested, err)
	}
	removed := map[Tool]bool{CodeInterpreter: true}
	for i := 0; i < 2; i++ {
		filtered, err := Filter(body, "openai_responses", nil, removed)
		if err != nil {
			t.Fatal(err)
		}
		var parsed struct {
			Tools      []struct{ Type, Name string }
			ToolChoice json.RawMessage `json:"tool_choice"`
		}
		if json.Unmarshal(filtered, &parsed) != nil || len(parsed.Tools) != 2 || parsed.Tools[1].Type != "function" || parsed.Tools[1].Name != "code_interpreter" || len(parsed.ToolChoice) != 0 {
			t.Fatalf("lost functions or retained dangling choice: %s", filtered)
		}
	}
	for _, choice := range []string{`"required"`, `{"type":"any"}`} {
		filtered, err := Filter([]byte(`{"tools":[{"type":"web_search_20250305","name":"web_search"}],"tool_choice":`+choice+`}`), "anthropic_messages", nil, map[Tool]bool{WebSearch: true})
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		json.Unmarshal(filtered, &fields)
		if _, exists := fields["tool_choice"]; exists {
			t.Fatalf("dangling choice retained: %s", filtered)
		}
	}
}

func TestExplicitFalseOverridesDefaultsAndIntentIsNotAFunctionName(t *testing.T) {
	for _, body := range []string{`{"enable_search":false}`, `{"tools":[{"type":"function","function":{"name":"web_search"}}],"enable_search":false}`} {
		tools, err := Inspect([]byte(body), "openai_chat", []Tool{WebSearch})
		if err != nil || len(tools) != 0 {
			t.Fatalf("false/function became a server tool: %v %v", tools, err)
		}
	}
	tools, err := Inspect([]byte(`{"parameters":{"enable_search":true,"search_options":{"strategy":"agent_max"}}}`), "dashscope_multimodal", nil)
	if err != nil || !tools[WebSearch] || !tools[WebExtractor] {
		t.Fatalf("native flags not recognized: %v %v", tools, err)
	}
}
