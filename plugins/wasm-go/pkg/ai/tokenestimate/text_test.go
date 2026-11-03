package tokenestimate

import (
	"strings"
	"testing"
)

func TestAvailableTextIncludesToolsAndNativeProtocols(t *testing.T) {
	for _, body := range []string{
		`{"messages":[{"role":"user","content":"hello"}],"tools":[{"type":"function","function":{"name":"weather","description":"city lookup"}}],"max_tokens":9000}`,
		`{"contents":[{"parts":[{"text":"hello"}]}]}`,
		`{"system":"hello","messages":[{"content":[{"type":"text","text":"question"}]}]}`,
	} {
		text := RequestText([]byte(body))
		if !strings.Contains(text, "hello") || strings.Contains(text, "9000") {
			t.Fatal("wrong model input", text)
		}
	}
}

func TestAvailableOutputNormalSSEAndErrors(t *testing.T) {
	for _, body := range []string{
		`{"choices":[{"message":{"content":"hello"}}]}`,
		`{"candidates":[{"content":{"parts":[{"text":"hello"}]}}]}`,
		"data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\ndata: [DONE]\n\n",
	} {
		if got := ResponseText([]byte(body)); got != "hello" {
			t.Fatal("output changed", got)
		}
	}
	if got := ResponseText([]byte(`{"error":{"message":"failed"}}`)); got != "" {
		t.Fatal("provider error became output")
	}
	if count, err := Count("unknown", ""); err != nil || count != 0 {
		t.Fatal("missing output is not zero", count, err)
	}
}

func TestAvailableTextPreservesLargeInputBudget(t *testing.T) {
	if count, err := Count("unknown", strings.Repeat("x", 2<<20)); err != nil || count == 0 {
		t.Fatal("large valid input not estimated", count, err)
	}
}

func TestResponsesSSECountsAvailableOutputOnce(t *testing.T) {
	for name, body := range map[string]string{
		"delta":           "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"hello world\"}\n\n",
		"final":           "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"output\":[{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"hello world\"}]}]}}\n\n",
		"delta_and_final": "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello world\"}\n\ndata: {\"type\":\"response.output_text.done\",\"text\":\"hello world\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"output\":[{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"hello world\"}]}]}}\n\n",
	} {
		t.Run(name, func(t *testing.T) {
			if got := ResponseText([]byte(body)); got != "hello world" {
				t.Fatalf("available output = %q", got)
			}
		})
	}
}
