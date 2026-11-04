package provider

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestResponsesPlaintextReasoningAndFunctionHistoryRoundTrip(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(map[bool]string{false: "json", true: "sse"}[streaming], func(t *testing.T) {
			var response []byte
			if !streaming {
				var err error
				response, err = ConvertChatCompletionResponseToResponses([]byte(`{"id":"c1","model":"upstream","choices":[{"index":0,"message":{"role":"assistant","content":"checking","reasoning_content":"look carefully","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"n\":9007199254740993}"}}]},"finish_reason":"tool_calls"}]}`))
				require.NoError(t, err)
			} else {
				converter := NewChatCompletionToResponsesStreamConverter()
				var stream bytes.Buffer
				for _, data := range []string{
					`{"id":"c1","model":"upstream","choices":[{"index":0,"delta":{"reasoning_content":"look "}}]}`,
					`{"choices":[{"index":0,"delta":{"reasoning_content":"carefully","content":"checking","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"n\":9007199254740993}"}}]}}]}`,
					`{"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
					`[DONE]`,
				} {
					out, err := converter.Convert([]byte("data: "+data+"\n\n"), false)
					require.NoError(t, err)
					stream.Write(out)
				}
				events, err := clientresponse.Events(stream.Bytes())
				require.NoError(t, err)
				for _, event := range events {
					if gjson.GetBytes(event.Data, "type").String() == "response.completed" {
						response = []byte(gjson.GetBytes(event.Data, "response").Raw)
					}
				}
				require.Contains(t, stream.String(), "response.reasoning_text.delta")
				require.Contains(t, stream.String(), "response.reasoning_text.done")
			}
			require.Equal(t, "look carefully", gjson.GetBytes(response, "output.0.content.0.text").String())
			require.Equal(t, "[]", gjson.GetBytes(response, "output.0.summary").Raw)
			require.False(t, gjson.GetBytes(response, "output.0.encrypted_content").Exists())
			var output []json.RawMessage
			require.NoError(t, json.Unmarshal([]byte(gjson.GetBytes(response, "output").Raw), &output))
			input := append([]json.RawMessage{json.RawMessage(`{"role":"user","content":"lookup"}`)}, output...)
			input = append(input, json.RawMessage(`{"type":"function_call_output","call_id":"call_1","output":"found"}`))
			raw, err := json.Marshal(struct {
				Model string            `json:"model"`
				Input []json.RawMessage `json:"input"`
			}{"sales", input})
			require.NoError(t, err)
			next, err := ConvertResponsesRequestToChatCompletion(raw)
			require.NoError(t, err)
			require.Equal(t, "look carefully", gjson.GetBytes(next, "messages.1.reasoning_content").String())
			require.Equal(t, "checking", gjson.GetBytes(next, "messages.1.content").String())
			require.Equal(t, "call_1", gjson.GetBytes(next, "messages.1.tool_calls.0.id").String())
			require.Equal(t, `{"n":9007199254740993}`, gjson.GetBytes(next, "messages.1.tool_calls.0.function.arguments").String())
			require.Equal(t, "call_1", gjson.GetBytes(next, "messages.2.tool_call_id").String())
		})
	}
}

func TestResponsesStreamRefusalAndSupplierInterruption(t *testing.T) {
	for _, reason := range []string{"stop", "insufficient_system_resource", "aborted"} {
		converter := NewChatCompletionToResponsesStreamConverter()
		out, err := converter.Convert([]byte("data: {\"id\":\"r\",\"choices\":[{\"index\":0,\"delta\":{\"refusal\":\"cannot comply\"},\"finish_reason\":\""+reason+"\"}]}\n\ndata: [DONE]\n\n"), true)
		require.NoError(t, err)
		events, err := clientresponse.Events(out)
		require.NoError(t, err)
		last := events[len(events)-1].Data
		require.Contains(t, string(out), "response.refusal.delta")
		require.Contains(t, string(out), "response.refusal.done")
		require.Equal(t, "cannot comply", gjson.GetBytes(last, "response.output.0.content.0.refusal").String())
		if reason == "stop" {
			require.Equal(t, "completed", gjson.GetBytes(last, "response.status").String())
		} else {
			require.Equal(t, "failed", gjson.GetBytes(last, "response.status").String())
			require.Equal(t, "incomplete", gjson.GetBytes(last, "response.output.0.status").String())
		}
	}
	_, err := NewChatCompletionToResponsesStreamConverter().Convert([]byte("data: {\"id\":\"r\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial\"}}]}\n\ndata: [DONE]\n\n"), true)
	require.Error(t, err)
}
