package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractRequestInputTextFromSupportedRequestShapes(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "chat completions messages string content",
			body: `{"model":"gpt-4o-mini","messages":[{"role":"system","content":"answer briefly"},{"role":"user","content":"hello"}]}`,
			want: "answer briefly\nhello",
		},
		{
			name: "chat completions messages content blocks",
			body: `{"messages":[{"role":"user","content":[{"type":"text","text":"describe this"},{"type":"image_url","image_url":{"url":"https://example.test/image.png"}},{"text":"using two words"}]}]}`,
			want: "describe this\nusing two words",
		},
		{
			name: "chat completions content block type gates text",
			body: `{"messages":[{"role":"user","content":[{"type":"file","text":"do-not-tokenize"},{"type":"text","text":"real prompt"}]}]}`,
			want: "real prompt",
		},
		{
			name: "responses instructions and string input",
			body: `{"instructions":"be concise","input":"summarize the release notes","metadata":{"trace":"do-not-tokenize"}}`,
			want: "be concise\nsummarize the release notes",
		},
		{
			name: "responses input content blocks",
			body: `{"input":[{"role":"user","content":[{"type":"input_text","text":"first question"},{"type":"input_image","image_url":"https://example.test/input.png"}]},{"role":"assistant","content":[{"type":"output_text","text":"prior answer"}]}]}`,
			want: "first question\nprior answer",
		},
		{
			name: "completions prompt string",
			body: `{"prompt":"write a haiku","suffix":"do-not-tokenize"}`,
			want: "write a haiku",
		},
		{
			name: "completions prompt preserves whitespace",
			body: "{\"prompt\":\"  write exactly\\n  \"}",
			want: "  write exactly\n  ",
		},
		{
			name: "completions prompt array",
			body: `{"prompt":["first prompt","second prompt"],"model":"legacy-model"}`,
			want: "first prompt\nsecond prompt",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := extractRequestInputText([]byte(tc.body))
			require.True(t, ok)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestExtractRequestInputTextFailsClosedForUnsupportedBodies(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "invalid json", body: `{"messages":`},
		{name: "trailing garbage after json", body: `{"prompt":"hi"} trailing`},
		{name: "non object json", body: `["raw text"]`},
		{name: "empty object", body: `{}`},
		{name: "unsupported metadata only", body: `{"model":"gpt-4o-mini","metadata":{"prompt":"do-not-tokenize"}}`},
		{name: "image content without text", body: `{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"https://example.test/image.png"}}]}]}`},
		{name: "tool call metadata without content text", body: `{"messages":[{"role":"assistant","content":null,"tool_calls":[{"function":{"name":"lookup","arguments":"{\"city\":\"Paris\"}"}}]}]}`},
		{name: "completion token ids are unsupported", body: `{"prompt":[1,2,3]}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := extractRequestInputText([]byte(tc.body))
			require.False(t, ok)
			require.Empty(t, got)
		})
	}
}

func TestExtractResponseOutputTextFromSupportedResponseShapes(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "chat completions message content",
			body: `{"choices":[{"message":{"role":"assistant","content":"hello there"}}],"usage":null}`,
			want: "hello there",
		},
		{
			name: "chat completions message content blocks",
			body: `{"choices":[{"message":{"content":[{"type":"text","text":"first"},{"type":"image_url","image_url":{"url":"https://example.test/out.png"}},{"type":"text","text":"second"}]}}]}`,
			want: "first\nsecond",
		},
		{
			name: "responses output content blocks",
			body: `{"output":[{"type":"message","content":[{"type":"output_text","text":"answer text"},{"type":"refusal","text":"do-not-tokenize"}]}],"metadata":{"trace":"ignore"}}`,
			want: "answer text",
		},
		{
			name: "responses output text shortcut",
			body: `{"output_text":"direct answer","usage":null}`,
			want: "direct answer",
		},
		{
			name: "responses output text shortcut takes precedence",
			body: `{"output_text":"direct answer","output":[{"content":[{"type":"output_text","text":"direct answer"}]}]}`,
			want: "direct answer",
		},
		{
			name: "completions choices text",
			body: `{"choices":[{"text":" completion one"},{"text":"completion two"}],"model":"legacy"}`,
			want: " completion one\ncompletion two",
		},
		{
			name: "output preserves whitespace",
			body: "{\"choices\":[{\"message\":{\"content\":\"  exact output\\n  \"}}]}",
			want: "  exact output\n  ",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := extractResponseOutputText([]byte(tc.body))
			require.True(t, ok)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestExtractResponseOutputTextFailsClosedForUnsupportedBodies(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "invalid json", body: `{"choices":`},
		{name: "trailing garbage after json", body: `{"choices":[{"text":"hi"}]} trailing`},
		{name: "non object json", body: `"raw text"`},
		{name: "empty object", body: `{}`},
		{name: "usage only", body: `{"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`},
		{name: "tool call arguments only", body: `{"choices":[{"message":{"tool_calls":[{"function":{"arguments":"{\"query\":\"Paris\"}"}}]}}]}`},
		{name: "image output without text", body: `{"output":[{"content":[{"type":"output_image","image_url":"https://example.test/out.png"}]}]}`},
		{name: "unsupported output object text", body: `{"output":{"text":"do-not-tokenize"}}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := extractResponseOutputText([]byte(tc.body))
			require.False(t, ok)
			require.Empty(t, got)
		})
	}
}

func TestExtractStreamingResponseOutputTextFromSupportedDeltas(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "openai chat completion content deltas",
			body: "data: {\"choices\":[{\"delta\":{\"content\":\"Hel\"}}]}\r\n\r\ndata: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\r\n\r\n",
			want: "Hello",
		},
		{
			name: "openai responses output text delta",
			body: "data: {\"type\":\"response.output_text.delta\",\"delta\":\"Hi\"}\n\n",
			want: "Hi",
		},
		{
			name: "claude text delta",
			body: "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\" there\"}}\n\n",
			want: " there",
		},
		{
			name: "legacy completions text delta",
			body: "data: {\"choices\":[{\"text\":\" completion\"}]}\n\n",
			want: " completion",
		},
		{
			name: "gemini candidate parts",
			body: "{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"gemini\"}]}}]}",
			want: "gemini",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := extractStreamingResponseOutputText([]byte(tc.body))
			require.True(t, ok)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestExtractStreamingResponseOutputTextFailsClosedForUnsupportedDeltas(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "done marker", body: "data: [DONE]\n\n"},
		{name: "invalid json", body: "data: {\"choices\":\n\n"},
		{name: "usage only", body: "data: {\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2,\"total_tokens\":3}}\n\n"},
		{name: "tool call arguments only", body: "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"function\":{\"arguments\":\"{\\\"city\\\":\\\"Paris\\\"}\"}}]}}]}\n\n"},
		{name: "claude tool input delta", body: "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"city\\\"\"}}\n\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := extractStreamingResponseOutputText([]byte(tc.body))
			require.False(t, ok)
			require.Empty(t, got)
		})
	}
}
