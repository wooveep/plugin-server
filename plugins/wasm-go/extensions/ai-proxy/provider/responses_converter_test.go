package provider

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestConvertResponsesRequestToChatCompletion_StringInputAndFields(t *testing.T) {
	body := []byte(`{
		"model":"gpt-4o-mini",
		"instructions":"Answer tersely.",
		"input":"hello",
		"stream":true,
		"temperature":0.7,
		"top_p":0.9,
		"presence_penalty":0.2,
		"frequency_penalty":0.3,
		"max_output_tokens":64,
		"user":"user-1",
		"metadata":{"trace":"abc"},
		"tools":[{
			"type":"function",
			"name":"lookup",
			"description":"Lookup a value.",
			"parameters":{"type":"object","properties":{"q":{"type":"string"}}}
		}],
		"tool_choice":{"type":"function","name":"lookup"}
	}`)

	out, err := convertResponsesRequestToChatCompletion(body)
	require.NoError(t, err)

	require.Equal(t, "gpt-4o-mini", gjson.GetBytes(out, "model").String())
	require.Equal(t, "developer", gjson.GetBytes(out, "messages.0.role").String())
	require.Equal(t, "Answer tersely.", gjson.GetBytes(out, "messages.0.content").String())
	require.Equal(t, "user", gjson.GetBytes(out, "messages.1.role").String())
	require.Equal(t, "hello", gjson.GetBytes(out, "messages.1.content").String())
	require.True(t, gjson.GetBytes(out, "stream").Bool())
	require.Equal(t, int64(64), gjson.GetBytes(out, "max_completion_tokens").Int())
	require.Equal(t, "user-1", gjson.GetBytes(out, "user").String())
	require.Equal(t, "abc", gjson.GetBytes(out, "metadata.trace").String())
	require.Equal(t, "function", gjson.GetBytes(out, "tools.0.type").String())
	require.Equal(t, "lookup", gjson.GetBytes(out, "tools.0.function.name").String())
	require.Equal(t, "function", gjson.GetBytes(out, "tool_choice.type").String())
	require.Equal(t, "lookup", gjson.GetBytes(out, "tool_choice.function.name").String())
	require.False(t, gjson.GetBytes(out, "input").Exists(), string(out))
}

func TestConvertResponsesRequestToChatCompletion_MessageArrayInput(t *testing.T) {
	body := []byte(`{
		"model":"gpt-4o-mini",
		"input":[
			{"role":"user","content":[{"type":"input_text","text":"first"}]},
			{"role":"assistant","content":[{"type":"output_text","text":"second"}]},
			{"role":"user","content":"third"}
		]
	}`)

	out, err := convertResponsesRequestToChatCompletion(body)
	require.NoError(t, err)
	require.Equal(t, "user", gjson.GetBytes(out, "messages.0.role").String())
	require.Equal(t, "first", gjson.GetBytes(out, "messages.0.content").String())
	require.Equal(t, "assistant", gjson.GetBytes(out, "messages.1.role").String())
	require.Equal(t, "second", gjson.GetBytes(out, "messages.1.content").String())
	require.Equal(t, "user", gjson.GetBytes(out, "messages.2.role").String())
	require.Equal(t, "third", gjson.GetBytes(out, "messages.2.content").String())
}

func TestConvertResponsesRequestToChatCompletion_UnsupportedFallbackFields(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "previous_response_id",
			body: `{"model":"gpt-4o-mini","input":"hello","previous_response_id":"resp_1"}`,
			want: "previous_response_id",
		},
		{
			name: "conversation",
			body: `{"model":"gpt-4o-mini","input":"hello","conversation":"conv_1"}`,
			want: "conversation",
		},
		{
			name: "hosted_tool",
			body: `{"model":"gpt-4o-mini","input":"hello","tools":[{"type":"web_search_preview"}]}`,
			want: "web_search_preview",
		},
		{
			name: "unmappable_multimodal_input",
			body: `{"model":"gpt-4o-mini","input":[{"role":"user","content":[{"type":"input_image","image_url":"https://example.test/a.png"}]}]}`,
			want: "input_image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := convertResponsesRequestToChatCompletion([]byte(tt.body))
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestConvertChatCompletionResponseToResponses(t *testing.T) {
	body := []byte(`{
		"id":"chatcmpl_1",
		"object":"chat.completion",
		"created":123,
		"model":"gpt-4o-mini",
		"choices":[{
			"index":0,
			"message":{"role":"assistant","content":"hello"},
			"finish_reason":"stop"
		}],
		"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18}
	}`)

	out, err := convertChatCompletionResponseToResponses(body)
	require.NoError(t, err)
	require.Equal(t, "chatcmpl_1", gjson.GetBytes(out, "id").String())
	require.Equal(t, "response", gjson.GetBytes(out, "object").String())
	require.Equal(t, int64(123), gjson.GetBytes(out, "created_at").Int())
	require.Equal(t, "gpt-4o-mini", gjson.GetBytes(out, "model").String())
	require.Equal(t, "completed", gjson.GetBytes(out, "status").String())
	require.Equal(t, "message", gjson.GetBytes(out, "output.0.type").String())
	require.Equal(t, "assistant", gjson.GetBytes(out, "output.0.role").String())
	require.Equal(t, "output_text", gjson.GetBytes(out, "output.0.content.0.type").String())
	require.Equal(t, "hello", gjson.GetBytes(out, "output.0.content.0.text").String())
	require.Equal(t, int64(11), gjson.GetBytes(out, "usage.input_tokens").Int())
	require.Equal(t, int64(7), gjson.GetBytes(out, "usage.output_tokens").Int())
	require.Equal(t, int64(18), gjson.GetBytes(out, "usage.total_tokens").Int())
}

func TestConvertChatCompletionResponseToResponses_Refusal(t *testing.T) {
	body := []byte(`{
		"id":"chatcmpl_1",
		"created":123,
		"model":"gpt-4o-mini",
		"choices":[{
			"message":{"role":"assistant","refusal":"I cannot help with that."},
			"finish_reason":"stop"
		}]
	}`)

	out, err := convertChatCompletionResponseToResponses(body)
	require.NoError(t, err)
	require.Equal(t, "refusal", gjson.GetBytes(out, "output.0.content.0.type").String())
	require.Equal(t, "I cannot help with that.", gjson.GetBytes(out, "output.0.content.0.refusal").String())
}

func TestChatCompletionToResponsesStreamConverter_TextDeltaAndCompletion(t *testing.T) {
	converter := newChatCompletionToResponsesStreamConverter()
	chunk := []byte("data: {\"id\":\"chatcmpl_1\",\"object\":\"chat.completion.chunk\",\"created\":123,\"model\":\"gpt-4o-mini\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"Hel\"}}]}\n\n")

	out, err := converter.convert(chunk, false)
	require.NoError(t, err)
	require.Contains(t, string(out), "event: response.output_text.delta")
	require.Contains(t, string(out), `"type":"response.output_text.delta"`)
	require.Contains(t, string(out), `"delta":"Hel"`)
	require.NotContains(t, string(out), "chat.completion.chunk")

	finish := []byte("data: {\"id\":\"chatcmpl_1\",\"object\":\"chat.completion.chunk\",\"created\":124,\"model\":\"gpt-4o-mini\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":1,\"total_tokens\":4}}\n\ndata: [DONE]\n\n")
	out, err = converter.convert(finish, true)
	require.NoError(t, err)
	text := string(out)
	require.Contains(t, text, "event: response.output_text.done")
	require.Contains(t, text, "event: response.completed")
	require.Contains(t, text, `"finish_reason":"stop"`)
	require.Contains(t, text, `"input_tokens":3`)
	require.NotContains(t, text, "data: [DONE]")

	for _, event := range strings.Split(strings.TrimSpace(text), "\n\n") {
		if strings.Contains(event, "response.completed") {
			data := strings.TrimPrefix(strings.Split(event, "\n")[1], "data: ")
			var decoded map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(data), &decoded))
		}
	}
}
