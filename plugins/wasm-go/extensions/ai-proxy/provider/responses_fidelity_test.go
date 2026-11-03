package provider

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestResponsesFallbackPreservesExplicitSamplingValues(t *testing.T) {
	for _, tc := range []struct {
		name    string
		values  string
		present bool
	}{
		{name: "omitted"},
		{name: "null", values: `,"temperature":null,"top_p":null,"presence_penalty":null,"frequency_penalty":null`},
		{name: "explicit_zero", values: `,"temperature":0,"top_p":0,"presence_penalty":0,"frequency_penalty":0`, present: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := ConvertResponsesRequestToChatCompletion([]byte(`{"model":"test","input":"hello"` + tc.values + `}`))
			require.NoError(t, err)
			for _, field := range []string{"temperature", "top_p", "presence_penalty", "frequency_penalty"} {
				value := gjson.GetBytes(out, field)
				if !tc.present {
					require.False(t, value.Exists(), string(out))
				} else {
					require.True(t, value.Exists(), string(out))
					require.Equal(t, gjson.Number, value.Type)
					require.Zero(t, value.Float())
				}
			}
		})
	}
}

func TestResponsesFallbackAdvancedFields(t *testing.T) {
	for _, value := range []string{"true", "false"} {
		out, err := ConvertResponsesRequestToChatCompletion([]byte(`{"model":"test","input":"hello","store":` + value + `,"background":false,"reasoning":{"effort":"none"},"prompt_cache_key":"cache-audit","prompt_cache_retention":"24h"}`))
		require.NoError(t, err)
		require.Equal(t, value, gjson.GetBytes(out, "store").Raw)
		require.Equal(t, "none", gjson.GetBytes(out, "reasoning_effort").String())
		require.Equal(t, "cache-audit", gjson.GetBytes(out, "prompt_cache_key").String())
		require.Equal(t, "24h", gjson.GetBytes(out, "prompt_cache_retention").String())
		require.False(t, gjson.GetBytes(out, "background").Exists())
		require.False(t, gjson.GetBytes(out, "reasoning").Exists())
	}
	out, err := ConvertResponsesRequestToChatCompletion([]byte(`{"model":"test","input":"hello"}`))
	require.NoError(t, err)
	for _, field := range []string{"store", "reasoning_effort", "prompt_cache_key", "prompt_cache_retention"} {
		require.False(t, gjson.GetBytes(out, field).Exists(), field)
	}
	for _, tc := range []struct{ fields, want string }{
		{`"background":true`, "background"},
		{`"reasoning":{"summary":"auto"}`, "reasoning summary"},
		{`"reasoning":{"generate_summary":"concise"}`, "reasoning summary"},
	} {
		_, err := ConvertResponsesRequestToChatCompletion([]byte(`{"model":"test","input":"hello",` + tc.fields + `}`))
		require.ErrorContains(t, err, tc.want)
	}
}

func TestResponsesFallbackCacheOptionsAndBreakpoints(t *testing.T) {
	for _, mode := range []string{"explicit", "implicit"} {
		body := fmt.Sprintf(`{"model":"test","prompt_cache_options":{"mode":%q,"ttl":"30m"},"input":[{"role":"developer","content":"prefix","prompt_cache_breakpoint":{"mode":"explicit"}},{"role":"user","content":[{"type":"input_text","text":"first","prompt_cache_breakpoint":{"mode":"explicit","ttl":"30m"}},{"type":"input_text","text":"second"},{"type":"input_text","text":"third","prompt_cache_breakpoint":{"mode":"explicit","ttl":"12h"}}]}]}`, mode)
		out, err := ConvertResponsesRequestToChatCompletion([]byte(body))
		require.NoError(t, err)
		require.JSONEq(t, fmt.Sprintf(`{"mode":%q,"ttl":"30m"}`, mode), gjson.GetBytes(out, "prompt_cache_options").Raw)
		require.Equal(t, "explicit", gjson.GetBytes(out, "messages.0.prompt_cache_breakpoint.mode").String())
		require.Equal(t, "prefix", gjson.GetBytes(out, "messages.0.content").String())
		require.Equal(t, int64(3), gjson.GetBytes(out, "messages.1.content.#").Int())
		for i, text := range []string{"first", "second", "third"} {
			path := fmt.Sprintf("messages.1.content.%d", i)
			require.Equal(t, "text", gjson.GetBytes(out, path+".type").String())
			require.Equal(t, text, gjson.GetBytes(out, path+".text").String())
		}
		require.Equal(t, "30m", gjson.GetBytes(out, "messages.1.content.0.prompt_cache_breakpoint.ttl").String())
		require.False(t, gjson.GetBytes(out, "messages.1.content.1.prompt_cache_breakpoint").Exists())
		require.Equal(t, "12h", gjson.GetBytes(out, "messages.1.content.2.prompt_cache_breakpoint.ttl").String())
	}
	for _, option := range []string{``, `,"prompt_cache_options":null`, `,"prompt_cache_options":{}`} {
		out, err := ConvertResponsesRequestToChatCompletion([]byte(`{"model":"test","input":"hello"` + option + `}`))
		require.NoError(t, err)
		if option == `,"prompt_cache_options":{}` {
			require.Equal(t, "{}", gjson.GetBytes(out, "prompt_cache_options").Raw)
		} else {
			require.False(t, gjson.GetBytes(out, "prompt_cache_options").Exists())
		}
	}
}

func TestResponsesFallbackPreservesFunctionConstraints(t *testing.T) {
	for _, value := range []string{"true", "false", "null", ""} {
		t.Run(value, func(t *testing.T) {
			constraint := ""
			if value != "" {
				constraint = `,"strict":` + value
			}
			body := `{"model":"test","input":"hello","tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}` + constraint + `}],"tool_choice":{"type":"function","name":"lookup"}`
			if value != "" {
				body += `,"parallel_tool_calls":` + value
			}
			out, err := ConvertResponsesRequestToChatCompletion([]byte(body + `}`))
			require.NoError(t, err)
			for _, field := range []string{"tools.0.function.strict", "parallel_tool_calls"} {
				got := gjson.GetBytes(out, field)
				if value == "" || value == "null" {
					require.False(t, got.Exists(), string(out))
				} else {
					require.Equal(t, value, got.Raw, string(out))
				}
			}
			require.Equal(t, "lookup", gjson.GetBytes(out, "tool_choice.function.name").String())
		})
	}
}

func TestResponsesFallbackMapsTextFormat(t *testing.T) {
	for _, format := range []string{
		`{"type":"text"}`,
		`{"type":"json_object"}`,
		`{"type":"json_schema","name":"audit","description":"Audit schema","schema":{"type":"object","properties":{"value":{"const":9007199254740993}},"required":["value"],"additionalProperties":false},"strict":true}`,
		`{"type":"json_schema","name":"audit","schema":{"type":"object"},"strict":false}`,
		`{"type":"json_schema","name":"audit","schema":{"type":"object"}}`,
	} {
		t.Run(format, func(t *testing.T) {
			out, err := ConvertResponsesRequestToChatCompletion([]byte(`{"model":"test","input":"Return JSON","text":{"format":` + format + `}}`))
			require.NoError(t, err)
			want := gjson.Parse(format)
			got := gjson.GetBytes(out, "response_format")
			require.Equal(t, want.Get("type").String(), got.Get("type").String(), string(out))
			if want.Get("type").String() == "json_schema" {
				for _, field := range []string{"name", "description", "schema", "strict"} {
					require.Equal(t, want.Get(field).Raw, got.Get("json_schema."+field).Raw, field)
				}
				require.False(t, got.Get("schema").Exists(), string(out))
			} else {
				require.False(t, got.Get("json_schema").Exists(), string(out))
			}
			require.False(t, gjson.GetBytes(out, "text").Exists(), string(out))
		})
	}
	for _, text := range []string{``, `,"text":{}`, `,"text":{"format":null}`} {
		out, err := ConvertResponsesRequestToChatCompletion([]byte(`{"model":"test","input":"hello"` + text + `}`))
		require.NoError(t, err)
		require.False(t, gjson.GetBytes(out, "response_format").Exists(), string(out))
	}
	_, err := ConvertResponsesRequestToChatCompletion([]byte(`{"model":"test","input":"hello","text":{"format":{"type":"unknown_format"}}}`))
	require.ErrorContains(t, err, "text.format")
}

func TestResponsesFallbackMessageIDs(t *testing.T) {
	for _, message := range []string{
		`{"role":"assistant","content":"hello"}`,
		`{"role":"assistant","refusal":"Cannot comply"}`,
		`{"role":"assistant","content":"partial","tool_calls":[{"id":"call_42","type":"function","function":{"name":"lookup","arguments":"{}"}}]}`,
	} {
		t.Run(message, func(t *testing.T) {
			ids := make(map[string]bool)
			for _, responseID := range []string{"chatcmpl_first", "chatcmpl_second"} {
				body := []byte(fmt.Sprintf(`{"id":%q,"created":123,"model":"test","choices":[{"index":0,"message":%s,"finish_reason":"length"}]}`, responseID, message))
				out, err := ConvertChatCompletionResponseToResponses(body)
				require.NoError(t, err)
				id := gjson.GetBytes(out, "output.0.id").String()
				require.NotEmpty(t, id, string(out))
				require.Contains(t, id, responseID)
				require.NotEqual(t, responseID, id)
				require.False(t, ids[id], "different responses must have different message IDs")
				ids[id] = true
				require.Equal(t, "incomplete", gjson.GetBytes(out, "status").String())
				if gjson.Parse(message).Get("tool_calls").Exists() {
					require.Equal(t, "call_42", gjson.GetBytes(out, "output.1.call_id").String())
					require.Equal(t, "{}", gjson.GetBytes(out, "output.1.arguments").String())
				}
			}
		})
	}
}
