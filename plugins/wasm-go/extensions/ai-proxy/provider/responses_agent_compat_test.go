package provider

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestResponsesFallback_AllowsStatelessTopLevelAgentFields(t *testing.T) {
	body := []byte(`{
		"model":"qwen-plus",
		"input":[{"type":"message","role":"developer","content":"Be concise."},{"type":"message","role":"user","content":"hello"}],
		"stream":true,
		"truncation":"disabled",
		"include":["reasoning.encrypted_content"],
		"service_tier":"priority",
		"safety_identifier":"tenant-safe-1",
		"max_tool_calls":8,
		"top_logprobs":3,
		"context_management":[{"type":"compaction","compact_threshold":100000}],
		"prompt":null,
		"moderation":{"enabled":false},
		"client_metadata":{"ide":"vscode"},
		"text":{"verbosity":"low"},
		"reasoning":{"effort":"medium","summary":"auto","generate_summary":"concise","context":"all_turns"},
		"stream_options":{"include_usage":true,"reasoning_summary_delivery":"delta"}
	}`)

	out, err := ConvertResponsesRequestToChatCompletionWithOptions(body, ResponsesToChatConvertOptions{
		DeveloperRoleAsSystem: true,
		OmitStore:             true,
	})
	require.NoError(t, err)
	require.Equal(t, "qwen-plus", gjson.GetBytes(out, "model").String())
	require.Equal(t, "system", gjson.GetBytes(out, "messages.0.role").String())
	require.Equal(t, "medium", gjson.GetBytes(out, "reasoning_effort").String())
	for _, stripped := range []string{"truncation", "include", "service_tier", "safety_identifier", "max_tool_calls", "top_logprobs", "context_management", "prompt", "moderation", "client_metadata"} {
		require.False(t, gjson.GetBytes(out, stripped).Exists(), "%s should not leak into Chat body", stripped)
	}
}

func TestResponsesFallback_NullAndEmptyBoundariesVsHardLines(t *testing.T) {
	out, err := ConvertResponsesRequestToChatCompletion([]byte(`{
		"model":"gpt-4o-mini",
		"input":"hello",
		"previous_response_id":null,
		"conversation":"",
		"tool_choice":null,
		"background":null
	}`))
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(out, "tool_choice").Exists(), string(out))
	require.False(t, gjson.GetBytes(out, "previous_response_id").Exists(), string(out))

	out2, err := ConvertResponsesRequestToChatCompletion([]byte(`{
		"model":"gpt-4o-mini",
		"input":"hello",
		"previous_response_id":"",
		"conversation":null,
		"prompt":null
	}`))
	require.NoError(t, err)
	require.Equal(t, "hello", gjson.GetBytes(out2, "messages.0.content").String())
	require.False(t, gjson.GetBytes(out2, "prompt").Exists(), string(out2))

	for _, tc := range []struct {
		name      string
		body      string
		wantCode  string
		wantField string
	}{
		{"non_empty_prev_id", `{"model":"m","input":"hi","previous_response_id":"resp_1"}`, "unsupported_stateful_reference", "previous_response_id"},
		{"non_empty_conversation", `{"model":"m","input":"hi","conversation":"conv_1"}`, "unsupported_stateful_reference", "conversation"},
		{"hosted_prompt_object", `{"model":"m","input":"hi","prompt":{"id":"pmpt_123"}}`, "unsupported_stateful_reference", "prompt"},
		{"hosted_prompt_string", `{"model":"m","input":"hi","prompt":"pmpt_123"}`, "unsupported_stateful_reference", "prompt"},
		{"background_true", `{"model":"m","input":"hi","background":true}`, "unsupported_background_mode", "background"},
		{"hosted_tool_search", `{"model":"m","input":"hi","tools":[{"type":"tool_search","execution":"client"}]}`, "unsupported_tool_type", "tool_search"},
		{"hosted_custom_tool", `{"model":"m","input":"hi","tools":[{"type":"custom","name":"apply_patch"}]}`, "unsupported_tool_type", "custom"},
		{"input_item_reference", `{"model":"m","input":[{"type":"item_reference","id":"item_1"}]}`, "unsupported_input_item_type", "item_reference"},
		{"input_compaction", `{"model":"m","input":[{"type":"compaction","encrypted_content":"abc"}]}`, "unsupported_input_item_type", "compaction"},
		{"input_file_part", `{"model":"m","input":[{"role":"user","content":[{"type":"input_file","file_id":"file_1"}]}]}`, "unsupported_content_part_type", "input_file"},
		{"input_image_file_id", `{"model":"m","input":[{"role":"user","content":[{"type":"input_image","file_id":"file_1"}]}]}`, "unsupported_image_content", "file_id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ConvertResponsesRequestToChatCompletion([]byte(tc.body))
			require.Error(t, err)
			var fallbackErr ResponsesFallbackError
			require.True(t, errors.As(err, &fallbackErr), "expected ResponsesFallbackError, got %T: %v", err, err)
			require.Equal(t, tc.wantCode, fallbackErr.Code)
			require.Equal(t, tc.wantField, fallbackErr.Field)
		})
	}
}

func TestResponsesFallback_MultiTurnReasoningReplayHandling(t *testing.T) {
	t.Run("skips_opaque_and_summary_reasoning_keeps_assistant_and_tool_call", func(t *testing.T) {
		body := []byte(`{
			"model":"qwen-plus",
			"input":[
				{"type":"message","role":"system","content":[{"type":"input_text","text":"sys"}]},
				{"type":"message","role":"user","content":[{"type":"input_text","text":"read file"}]},
				{"type":"reasoning","id":"rs_1","summary":[],"encrypted_content":"gAAAAABopaque"},
				{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Let me check."}],"phase":"commentary","status":"completed"},
				{"type":"function_call","call_id":"call_1","name":"read_file","arguments":"{\"path\":\"a.go\"}"},
				{"type":"function_call_output","call_id":"call_1","output":"package main"},
				{"type":"reasoning","id":"rs_2","summary":[{"type":"summary_text","text":"thinking summary"}]},
				{"type":"function_call","call_id":"call_2","name":"list_dir","arguments":"{}"},
				{"type":"function_call_output","call_id":"call_2","output":"main.go"}
			]
		}`)
		out, err := ConvertResponsesRequestToChatCompletion(body)
		require.NoError(t, err)
		require.Equal(t, int64(6), gjson.GetBytes(out, "messages.#").Int(), string(out))
		require.Equal(t, "Let me check.", gjson.GetBytes(out, "messages.2.content").String())
		require.Equal(t, "call_1", gjson.GetBytes(out, "messages.2.tool_calls.0.id").String())
		require.Equal(t, "call_1", gjson.GetBytes(out, "messages.3.tool_call_id").String())
		require.Equal(t, "assistant", gjson.GetBytes(out, "messages.4.role").String())
		require.Equal(t, "call_2", gjson.GetBytes(out, "messages.4.tool_calls.0.id").String())
		require.Equal(t, "call_2", gjson.GetBytes(out, "messages.5.tool_call_id").String())
	})

	t.Run("extracts_plaintext_reasoning_and_ignores_encrypted_or_summary", func(t *testing.T) {
		body := []byte(`{
			"model":"qwen-plus",
			"input":[
				{"role":"user","content":"check"},
				{"type":"reasoning","summary":[{"type":"summary_text","text":"short"}],"encrypted_content":"opaque","content":[{"type":"reasoning_text","text":"full plaintext thought"}]},
				{"role":"assistant","content":"done"}
			]
		}`)
		out, err := ConvertResponsesRequestToChatCompletion(body)
		require.NoError(t, err)
		require.Equal(t, "full plaintext thought", gjson.GetBytes(out, "messages.1.reasoning_content").String())
		require.Equal(t, "done", gjson.GetBytes(out, "messages.1.content").String())
	})
}

func TestResponsesFallback_OmitStoreOptionForNonOpenAIUpstreams(t *testing.T) {
	body := []byte(`{"model":"qwen-plus","input":"hello","store":false}`)

	withStore, err := ConvertResponsesRequestToChatCompletionWithOptions(body, ResponsesToChatConvertOptions{OmitStore: false})
	require.NoError(t, err)
	require.True(t, gjson.GetBytes(withStore, "store").Exists())
	require.False(t, gjson.GetBytes(withStore, "store").Bool())

	withoutStore, err := ConvertResponsesRequestToChatCompletionWithOptions(body, ResponsesToChatConvertOptions{OmitStore: true})
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(withoutStore, "store").Exists(), string(withoutStore))
}
