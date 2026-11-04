package provider

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestResponsesFallbackImageInput(t *testing.T) {
	for _, detail := range []string{"", "auto", "low", "high", "original"} {
		t.Run(detail, func(t *testing.T) {
			body := `{"model":"test","input":[{"role":"user","content":[{"type":"input_text","text":"Describe"},{"type":"input_image","image_url":"https://example.test/a.png","detail":"` + detail + `","prompt_cache_breakpoint":{"mode":"explicit","ttl":"30m"}},{"type":"input_image","image_url":"data:image/png;base64,AAAA"},{"type":"input_text","text":"End"}]}]}`
			out, err := ConvertResponsesRequestToChatCompletion([]byte(body))
			require.NoError(t, err)
			require.Equal(t, int64(4), gjson.GetBytes(out, "messages.0.content.#").Int())
			require.Equal(t, "Describe", gjson.GetBytes(out, "messages.0.content.0.text").String())
			require.Equal(t, "image_url", gjson.GetBytes(out, "messages.0.content.1.type").String())
			require.Equal(t, "https://example.test/a.png", gjson.GetBytes(out, "messages.0.content.1.image_url.url").String())
			require.Equal(t, detail, gjson.GetBytes(out, "messages.0.content.1.image_url.detail").String())
			require.False(t, gjson.GetBytes(out, "messages.0.content.1.text").Exists())
			require.Equal(t, "30m", gjson.GetBytes(out, "messages.0.content.1.prompt_cache_breakpoint.ttl").String())
			require.Equal(t, "data:image/png;base64,AAAA", gjson.GetBytes(out, "messages.0.content.2.image_url.url").String())
			require.Equal(t, "End", gjson.GetBytes(out, "messages.0.content.3.text").String())
		})
	}
}

func TestResponsesFallbackCompleteFunctionHistory(t *testing.T) {
	body := []byte(`{"model":"test","input":[
		{"role":"user","content":"lookup both"},
		{"type":"message","id":"msg_old","status":"completed","role":"assistant","content":[{"type":"output_text","text":"Checking","annotations":[]}]},
		{"type":"function_call","id":"fc_local_a","status":"completed","call_id":"call_a","name":"lookup","arguments":"{\"n\":9007199254740993}"},
		{"type":"function_call","id":"fc_local_b","call_id":"call_b","name":"lookup","arguments":"{}"},
		{"type":"function_call_output","call_id":"call_b","output":[{"type":"input_text","text":"first","prompt_cache_breakpoint":{"mode":"explicit"}},{"type":"input_text","text":"second"}]},
		{"type":"function_call_output","call_id":"call_a","output":""},
		{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Done"}]},
		{"role":"user","content":"lookup again"},
		{"type":"function_call","call_id":"call_c","name":"lookup","arguments":"{}"},
		{"type":"function_call_output","call_id":"call_c","output":"{\"status\":\"delivered\"}"}
	]}`)
	out, err := ConvertResponsesRequestToChatCompletion(body)
	require.NoError(t, err)
	require.Equal(t, int64(8), gjson.GetBytes(out, "messages.#").Int())
	require.Equal(t, "Checking", gjson.GetBytes(out, "messages.1.content").String())
	require.Equal(t, int64(2), gjson.GetBytes(out, "messages.1.tool_calls.#").Int())
	for index, id := range []string{"call_a", "call_b"} {
		call := gjson.GetBytes(out, "messages.1.tool_calls").Array()[index]
		require.Equal(t, id, call.Get("id").String())
		require.Equal(t, "function", call.Get("type").String())
		require.False(t, call.Get("index").Exists(), "Chat request tool calls do not have streaming indexes")
	}
	require.Equal(t, `{"n":9007199254740993}`, gjson.GetBytes(out, "messages.1.tool_calls.0.function.arguments").String())
	require.Equal(t, "tool", gjson.GetBytes(out, "messages.2.role").String())
	require.Equal(t, "call_b", gjson.GetBytes(out, "messages.2.tool_call_id").String())
	require.Equal(t, int64(2), gjson.GetBytes(out, "messages.2.content.#").Int())
	require.Equal(t, "explicit", gjson.GetBytes(out, "messages.2.content.0.prompt_cache_breakpoint.mode").String())
	require.Equal(t, "second", gjson.GetBytes(out, "messages.2.content.1.text").String())
	require.Equal(t, "call_a", gjson.GetBytes(out, "messages.3.tool_call_id").String())
	require.Equal(t, `""`, gjson.GetBytes(out, "messages.3.content").Raw)
	require.Equal(t, "assistant", gjson.GetBytes(out, "messages.6.role").String())
	require.False(t, gjson.GetBytes(out, "messages.6.content").Exists())
	require.Equal(t, "call_c", gjson.GetBytes(out, "messages.6.tool_calls.0.id").String())
	require.Equal(t, "call_c", gjson.GetBytes(out, "messages.7.tool_call_id").String())
}

func TestResponsesFallbackReplaysConvertedOutput(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "json", true: "sse"}[stream], func(t *testing.T) {
			chat := []byte(`{"id":"chatcmpl_replay","model":"test","created":123,"choices":[{"message":{"role":"assistant","content":"Checking","tool_calls":[{"id":"call_replay","type":"function","function":{"name":"lookup","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`)
			response, err := ConvertChatCompletionResponseToResponses(chat)
			require.NoError(t, err)
			body := `{"model":"test","input":[{"role":"user","content":"lookup"},` + gjson.GetBytes(response, "output").Raw[1:len(gjson.GetBytes(response, "output").Raw)-1] + `,{"type":"function_call_output","call_id":"call_replay","output":"delivered"}],"stream":` + map[bool]string{false: "false", true: "true"}[stream] + `}`
			out, err := ConvertResponsesRequestToChatCompletion([]byte(body))
			require.NoError(t, err)
			require.Equal(t, "call_replay", gjson.GetBytes(out, "messages.1.tool_calls.0.id").String())
			require.Equal(t, "call_replay", gjson.GetBytes(out, "messages.2.tool_call_id").String())
			require.Equal(t, "delivered", gjson.GetBytes(out, "messages.2.content").String())
			require.Equal(t, stream, gjson.GetBytes(out, "stream").Bool())
		})
	}
}

func TestResponsesFallbackAssistantRefusalHistory(t *testing.T) {
	out, err := ConvertResponsesRequestToChatCompletion([]byte(`{"model":"test","input":[{"type":"message","role":"assistant","content":[{"type":"refusal","refusal":"Cannot comply"}]},{"role":"user","content":"Try another question"}]}`))
	require.NoError(t, err)
	require.JSONEq(t, `[{"type":"refusal","refusal":"Cannot comply"}]`, gjson.GetBytes(out, "messages.0.content").Raw)
}

func TestResponsesFallbackAssistantMessageAfterFunctionCall(t *testing.T) {
	out, err := ConvertResponsesRequestToChatCompletion([]byte(`{"model":"test","input":[{"role":"user","content":"lookup"},{"type":"function_call","call_id":"call_after","name":"lookup","arguments":"{}"},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Checking"}]},{"type":"function_call_output","call_id":"call_after","output":"ok"}]}`))
	require.NoError(t, err)
	require.Equal(t, "Checking", gjson.GetBytes(out, "messages.1.content").String())
	require.Equal(t, "call_after", gjson.GetBytes(out, "messages.1.tool_calls.0.id").String())
	require.Equal(t, "call_after", gjson.GetBytes(out, "messages.2.tool_call_id").String())
}

func TestResponsesFallbackAssistantSegmentsWithinFunctionBatch(t *testing.T) {
	out, err := ConvertResponsesRequestToChatCompletion([]byte(`{"model":"test","input":[{"role":"user","content":"lookup"},{"role":"assistant","content":"first","prompt_cache_breakpoint":{"mode":"explicit","ttl":"30m"}},{"type":"function_call","call_id":"call_a","name":"lookup","arguments":"{}"},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"second"}],"prompt_cache_breakpoint":{"mode":"explicit","ttl":"12h"}},{"type":"function_call","call_id":"call_b","name":"lookup","arguments":"{}"},{"role":"assistant","content":"third"},{"type":"function_call_output","call_id":"call_b","output":"b"},{"type":"function_call_output","call_id":"call_a","output":"a"}]}`))
	require.NoError(t, err)
	require.Equal(t, int64(4), gjson.GetBytes(out, "messages.#").Int())
	require.Equal(t, int64(2), gjson.GetBytes(out, "messages.1.tool_calls.#").Int())
	require.Equal(t, "first", gjson.GetBytes(out, "messages.1.content.0.text").String())
	require.Equal(t, "30m", gjson.GetBytes(out, "messages.1.content.0.prompt_cache_breakpoint.ttl").String())
	require.Equal(t, "second", gjson.GetBytes(out, "messages.1.content.1.text").String())
	require.Equal(t, "12h", gjson.GetBytes(out, "messages.1.content.1.prompt_cache_breakpoint.ttl").String())
	require.Equal(t, "third", gjson.GetBytes(out, "messages.1.content.2.text").String())
	require.False(t, gjson.GetBytes(out, "messages.1.prompt_cache_breakpoint").Exists())
}

func TestResponsesFallbackRejectsUnrepresentableOrInvalidHistory(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"null_input", `null`, "input"},
		{"unknown_item", `[{"type":"item_reference","id":"msg_stored"}]`, "item_reference"},
		{"reasoning", `[{"type":"reasoning","encrypted_content":"opaque"}]`, "reasoning"},
		{"image_file_id", `[{"role":"user","content":[{"type":"input_image","file_id":"file_1"}]}]`, "file_id"},
		{"missing_image_url", `[{"role":"user","content":[{"type":"input_image"}]}]`, "image_url"},
		{"invalid_image_detail", `[{"role":"user","content":[{"type":"input_image","image_url":"https://example.test/a.png","detail":"invalid"}]}]`, "detail"},
		{"assistant_image", `[{"role":"assistant","content":[{"type":"input_image","image_url":"https://example.test/a.png"}]}]`, "user"},
		{"missing_role", `[{"type":"message","content":"hello"}]`, "role"},
		{"missing_content", `[{"role":"user"}]`, "content"},
		{"missing_arguments", `[{"type":"function_call","call_id":"call_1","name":"lookup"}]`, "arguments"},
		{"missing_call_id", `[{"type":"function_call","name":"lookup","arguments":"{}"}]`, "call_id"},
		{"missing_name", `[{"type":"function_call","call_id":"call_1","arguments":"{}"}]`, "name"},
		{"orphan_output", `[{"type":"function_call_output","call_id":"call_other","output":"ok"}]`, "call_id"},
		{"missing_output", `[{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{}"},{"type":"function_call_output","call_id":"call_1"}]`, "content"},
		{"pending_output", `[{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{}"}]`, "output"},
		{"interrupted_output", `[{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{}"},{"role":"user","content":"next"}]`, "output"},
		{"duplicate_call_id", `[{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{}"},{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{}"}]`, "duplicate"},
		{"duplicate_output", `[{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{}"},{"type":"function_call_output","call_id":"call_1","output":"ok"},{"type":"function_call_output","call_id":"call_1","output":"again"}]`, "call_id"},
		{"tool_image", `[{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{}"},{"type":"function_call_output","call_id":"call_1","output":[{"type":"input_image","image_url":"https://example.test/a.png"}]}]`, "text"},
		{"tool_file", `[{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{}"},{"type":"function_call_output","call_id":"call_1","output":[{"type":"input_file","file_id":"file_1"}]}]`, "input_file"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ConvertResponsesRequestToChatCompletion([]byte(`{"model":"test","input":` + tc.input + `}`))
			require.ErrorContains(t, err, tc.want)
		})
	}
}
