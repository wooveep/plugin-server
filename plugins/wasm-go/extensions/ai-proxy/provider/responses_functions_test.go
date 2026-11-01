package provider

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestResponsesFunctionStreamPreservesFragmentedArgumentsAndArrivalOrder(t *testing.T) {
	converter := NewChatCompletionToResponsesStreamConverter()
	var result bytes.Buffer
	for _, frame := range []string{
		`{"id":"chat_1","model":"frozen-model","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_first","type":"function","function":{"name":"first","arguments":"{\"value\":"}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"content":"after","tool_calls":[{"index":0,"function":{"arguments":"9007199254740993}"}},{"index":1,"id":"call_second","type":"function","function":{"name":"second","arguments":"{}"}}]}}]}`,
		`{"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		`[DONE]`,
	} {
		out, err := converter.Convert([]byte("data: "+frame+"\n\n"), false)
		require.NoError(t, err)
		result.Write(out)
	}
	text := result.String()
	require.Equal(t, 2, strings.Count(text, "event: response.output_item.added\n"))
	require.Equal(t, 2, strings.Count(text, "event: response.function_call_arguments.done\n"))
	var completed gjson.Result
	for _, event := range strings.Split(strings.TrimSpace(text), "\n\n") {
		lines := strings.SplitN(event, "\n", 2)
		if lines[0] == "event: response.completed" {
			completed = gjson.Parse(strings.TrimPrefix(lines[1], "data: "))
		}
	}
	require.True(t, completed.Exists())
	require.Equal(t, "function_call", completed.Get("response.output.0.type").String())
	require.Equal(t, "call_first", completed.Get("response.output.0.call_id").String())
	require.Equal(t, `{"value":9007199254740993}`, completed.Get("response.output.0.arguments").String())
	require.Equal(t, "after", completed.Get("response.output.1.content.0.text").String())
	require.Equal(t, "call_second", completed.Get("response.output.2.call_id").String())
	require.Equal(t, "{}", completed.Get("response.output.2.arguments").String())
	require.Contains(t, text, `"output_index":2`)
}
