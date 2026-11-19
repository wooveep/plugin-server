package provider

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/canonical"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func requireRejectionError(t *testing.T, err error, wantCode, wantField string) canonical.RejectionError {
	t.Helper()
	require.Error(t, err)
	var rej canonical.RejectionError
	require.True(t, errors.As(err, &rej), "expected canonical.RejectionError, got %T: %v", err, err)
	if wantCode != "" {
		assert.Equal(t, wantCode, rej.SafeCode())
	}
	if wantField != "" {
		assert.Equal(t, wantField, rej.SafeField())
	}
	return rej
}

func TestCanonicalClientCodecsASTPurity(t *testing.T) {
	files := []string{
		"canonical_target.go",
		"canonical_chat.go",
		"canonical_responses.go",
		"canonical_messages.go",
	}
	fset := token.NewFileSet()
	for _, filename := range files {
		file, err := parser.ParseFile(fset, filename, nil, parser.AllErrors)
		require.NoError(t, err, "parse %s", filename)

		allowedTypeParamAny := map[*ast.Ident]bool{}
		ast.Inspect(file, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == "init" {
				t.Fatalf("%s declares forbidden init() function", filename)
			}
			if tp, ok := n.(*ast.TypeSpec); ok && tp.TypeParams != nil {
				for _, field := range tp.TypeParams.List {
					if ident, ok := field.Type.(*ast.Ident); ok && ident.Name == "any" {
						allowedTypeParamAny[ident] = true
					}
				}
			}
			if fn, ok := n.(*ast.FuncDecl); ok && fn.Type.TypeParams != nil {
				for _, field := range fn.Type.TypeParams.List {
					if ident, ok := field.Type.(*ast.Ident); ok && ident.Name == "any" {
						allowedTypeParamAny[ident] = true
					}
				}
			}
			if call, ok := n.(*ast.CallExpr); ok {
				if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "panic" {
					t.Fatalf("%s contains forbidden panic() call at %v", filename, fset.Position(call.Pos()))
				}
			}
			return true
		})

		ast.Inspect(file, func(n ast.Node) bool {
			if iface, ok := n.(*ast.InterfaceType); ok {
				if iface.Methods == nil || len(iface.Methods.List) == 0 {
					t.Fatalf("%s contains forbidden empty interface{} at %v", filename, fset.Position(iface.Pos()))
				}
			}
			if ident, ok := n.(*ast.Ident); ok && ident.Name == "any" && !allowedTypeParamAny[ident] {
				t.Fatalf("%s contains forbidden dynamic 'any' at %v", filename, fset.Position(ident.Pos()))
			}
			return true
		})
	}
}

func TestCanonicalStaticDispatchAndPartialMode(t *testing.T) {
	req, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, []byte(`{
		"model":"qwen-plus",
		"messages":[
			{"role":"user","content":"Complete this function"},
			{"role":"assistant","content":"def add(a,b):\n    ","partial":true}
		]
	}`))
	require.NoError(t, err)
	require.Len(t, req.Items, 2)
	assert.True(t, req.Items[0].Partial.IsAbsent())
	assert.True(t, req.Items[1].Partial.IsPresent())
	assert.True(t, req.Items[1].Partial.Val)

	// Round-trip Partial tri-state through Chat encoder.
	chatOut, err := EncodeCanonicalRequest(req, CanonicalTarget{
		WireFormat: WireChatCompletions,
		Model:      "qwen-plus",
	})
	require.NoError(t, err)
	assert.False(t, gjson.GetBytes(chatOut, "messages.0.partial").Exists())
	assert.Equal(t, gjson.True, gjson.GetBytes(chatOut, "messages.1.partial").Type)

	// Static dispatch to Qwen DashScope & Gemini request/response/stream codecs.
	qwenTarget := CanonicalTarget{WireFormat: WireDashScopeText, Supplier: "qwen", Model: "qwen-plus"}
	qwenReqBytes, err := EncodeCanonicalRequest(req, qwenTarget)
	require.NoError(t, err)
	assert.Equal(t, "qwen-plus", gjson.GetBytes(qwenReqBytes, "model").String())

	qwenResp, err := DecodeCanonicalResponse(qwenTarget, []byte(`{"request_id":"r_static","output":{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"return a + b"}}]}}`))
	require.NoError(t, err)
	require.Len(t, qwenResp.Choices, 1)
	assert.Equal(t, canonical.FinishStop, qwenResp.Choices[0].Finish.Category)

	qwenDec, err := NewCanonicalEventDecoder(qwenTarget)
	require.NoError(t, err)
	require.NotNil(t, qwenDec)

	gemTarget := CanonicalTarget{WireFormat: WireGeminiGenerateContent, Supplier: "gemini", Model: "gemini-2.5-flash"}
	gemReqBytes, err := EncodeCanonicalRequest(req, gemTarget)
	require.NoError(t, err)
	assert.Equal(t, "Complete this function", gjson.GetBytes(gemReqBytes, "contents.0.parts.0.text").String())

	gemResp, err := DecodeCanonicalResponse(gemTarget, []byte(`{"responseId":"g_static","candidates":[{"index":0,"finishReason":"STOP","content":{"role":"model","parts":[{"text":"return a + b"}]}}]}`))
	require.NoError(t, err)
	require.Len(t, gemResp.Choices, 1)
	assert.Equal(t, canonical.FinishStop, gemResp.Choices[0].Finish.Category)

	gemDec, err := NewCanonicalEventDecoder(gemTarget)
	require.NoError(t, err)
	require.NotNil(t, gemDec)

	// Unsupported target wire formats & client protocols still return RejectionError.
	for _, wf := range []WireFormat{WireOpenAIResponses, WireAnthropicMessages, WireFormat("unknown")} {
		target := CanonicalTarget{WireFormat: wf, Model: "test-model"}
		_, err := EncodeCanonicalRequest(req, target)
		requireRejectionError(t, err, "unsupported_wire_format", "")

		_, err = DecodeCanonicalResponse(target, []byte(`{}`))
		requireRejectionError(t, err, "unsupported_wire_format", "")

		_, err = NewCanonicalEventDecoder(target)
		requireRejectionError(t, err, "unsupported_wire_format", "")
	}

	_, err = DecodeCanonicalRequest(canonical.Protocol("unsupported"), []byte(`{}`))
	requireRejectionError(t, err, "unsupported_client_protocol", "unsupported")
}

// T01 + H1: Presence matrix (absent / null / explicit 0 / false / "" / [] / {}), stream:null reject,
// stateless Agent fields, and H1 non-null prompt rejection.
func TestCanonicalPresenceMatrix(t *testing.T) {
	t.Run("stream_null_rejected_across_all_three_client_decoders", func(t *testing.T) {
		for _, tc := range []struct {
			protocol canonical.Protocol
			body     string
		}{
			{canonical.ProtocolOpenAIChat, `{"model":"gpt-4o","stream":null,"messages":[{"role":"user","content":"hi"}]}`},
			{canonical.ProtocolOpenAIResponses, `{"model":"gpt-4o","stream":null,"input":"hi"}`},
			{canonical.ProtocolAnthropicMessages, `{"model":"claude-sonnet-4-6","max_tokens":64,"stream":null,"messages":[{"role":"user","content":"hi"}]}`},
		} {
			_, err := DecodeCanonicalRequest(tc.protocol, []byte(tc.body))
			requireRejectionError(t, err, "invalid_stream_flag", "stream")
		}
	})

	t.Run("chat_tri_state_presence_and_explicit_zeros", func(t *testing.T) {
		omitted, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`))
		require.NoError(t, err)
		assert.True(t, omitted.Sampling.Temperature.IsAbsent())
		assert.True(t, omitted.Sampling.Seed.IsAbsent())
		assert.True(t, omitted.Metadata.IsAbsent())
		assert.True(t, omitted.ParallelToolCalls.IsAbsent())

		withNulls, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, []byte(`{
			"model":"gpt-4o",
			"temperature":null,
			"top_p":null,
			"seed":null,
			"presence_penalty":null,
			"frequency_penalty":null,
			"metadata":null,
			"parallel_tool_calls":null,
			"search_options":null,
			"messages":[{"role":"user","content":"hi"}]
		}`))
		require.NoError(t, err)
		assert.True(t, withNulls.Sampling.Temperature.IsNull())
		assert.True(t, withNulls.Sampling.TopP.IsNull())
		assert.True(t, withNulls.Sampling.Seed.IsNull())
		assert.True(t, withNulls.Sampling.PresencePenalty.IsNull())
		assert.True(t, withNulls.Sampling.FrequencyPenalty.IsNull())
		assert.True(t, withNulls.Metadata.IsNull())
		assert.True(t, withNulls.ParallelToolCalls.IsNull())
		assert.Nil(t, withNulls.FixedTools.SearchOptionsRaw)

		withZeros, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, []byte(`{
			"model":"gpt-4o",
			"temperature":0,
			"top_p":0,
			"seed":0,
			"presence_penalty":0,
			"frequency_penalty":0,
			"metadata":{},
			"parallel_tool_calls":false,
			"store":false,
			"messages":[{"role":"user","content":""}]
		}`))
		require.NoError(t, err)
		assert.True(t, withZeros.Sampling.Temperature.IsPresent())
		assert.Zero(t, withZeros.Sampling.Temperature.Val)
		assert.True(t, withZeros.Sampling.TopP.IsPresent())
		assert.Zero(t, withZeros.Sampling.TopP.Val)
		assert.True(t, withZeros.Sampling.Seed.IsPresent())
		assert.Zero(t, withZeros.Sampling.Seed.Val)
		assert.True(t, withZeros.Sampling.PresencePenalty.IsPresent())
		assert.True(t, withZeros.Sampling.FrequencyPenalty.IsPresent())
		assert.True(t, withZeros.Metadata.IsPresent())
		assert.Empty(t, withZeros.Metadata.Val)
		assert.True(t, withZeros.ParallelToolCalls.IsPresent())
		assert.False(t, withZeros.ParallelToolCalls.Val)

		encoded, err := EncodeCanonicalRequest(withZeros, CanonicalTarget{
			WireFormat:    WireChatCompletions,
			Model:         "gpt-4o",
			SupportsStore: true,
		})
		require.NoError(t, err)
		for _, scalar := range []string{"temperature", "top_p", "seed", "presence_penalty", "frequency_penalty"} {
			res := gjson.GetBytes(encoded, scalar)
			assert.True(t, res.Exists(), "expected %s=0 in encoded Chat JSON: %s", scalar, string(encoded))
			assert.Equal(t, float64(0), res.Float())
		}
		assert.Equal(t, "false", gjson.GetBytes(encoded, "parallel_tool_calls").Raw)
		assert.Equal(t, "false", gjson.GetBytes(encoded, "store").Raw)
	})

	t.Run("h1_responses_prompt_null_stateless_whitelist_and_stateful_rejections", func(t *testing.T) {
		validAgentBody := []byte(`{
			"model":"qwen-plus",
			"input":[
				{"type":"message","role":"developer","content":"Be concise."},
				{"type":"message","role":"assistant","content":[]},
				{"type":"function_call","id":"fc_1","call_id":"call_1","name":"ping","arguments":"{}"},
				{"type":"function_call_output","id":"fco_1","call_id":"call_1","output":[]},
				{"type":"message","role":"user","content":"hello"}
			],
			"stream":true,
			"prompt":null,
			"previous_response_id":null,
			"conversation":"",
			"background":null,
			"tool_choice":"",
			"store":false,
			"truncation":"disabled",
			"include":["reasoning.encrypted_content"],
			"service_tier":"priority",
			"safety_identifier":"tenant-safe-1",
			"max_tool_calls":8,
			"top_logprobs":3,
			"context_management":[{"type":"compaction","compact_threshold":100000}],
			"moderation":{"enabled":false},
			"client_metadata":{"ide":"vscode"},
			"text":{"verbosity":"low"},
			"reasoning":{"effort":"medium","summary":"auto","generate_summary":"concise","context":"all_turns"},
			"stream_options":{"include_usage":false,"reasoning_summary_delivery":"delta"}
		}`)
		req, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIResponses, validAgentBody)
		require.NoError(t, err)
		require.NoError(t, ValidateCanonicalTarget(req, CanonicalTarget{
			WireFormat:       WireChatCompletions,
			Supplier:         "qwen",
			Model:            "qwen-plus",
			SupportsThinking: true,
		}))

		outNoStore, err := EncodeCanonicalRequest(req, CanonicalTarget{
			WireFormat:    WireChatCompletions,
			Supplier:      "qwen",
			Model:         "qwen-plus",
			SupportsStore: false,
		})
		require.NoError(t, err)
		assert.Equal(t, "system", gjson.GetBytes(outNoStore, "messages.0.role").String())
		assert.Equal(t, "", gjson.GetBytes(outNoStore, "messages.1.content").String())
		assert.Equal(t, `""`, gjson.GetBytes(outNoStore, "messages.2.content").Raw)
		assert.True(t, gjson.GetBytes(outNoStore, "stream_options.include_usage").Bool(), "managed upstream Chat stream must request usage")
		for _, stripped := range []string{"store", "prompt", "truncation", "include", "service_tier", "safety_identifier", "max_tool_calls", "top_logprobs", "context_management", "moderation", "client_metadata", "previous_response_id", "conversation", "background", "tool_choice"} {
			assert.False(t, gjson.GetBytes(outNoStore, stripped).Exists(), "%s must not leak into outbound Chat body", stripped)
		}

		// H1 hard line: non-null prompt MUST be rejected with 400 unsupported_stateful_reference (field="prompt") and leak zero secret IDs.
		for _, badPrompt := range []string{`{"id":"pmpt_SECRET_TOKEN_999"}`, `"pmpt_SECRET_TOKEN_999"`, `{}`} {
			_, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIResponses, []byte(`{"model":"qwen-plus","input":"hi","prompt":`+badPrompt+`}`))
			rej := requireRejectionError(t, err, "unsupported_stateful_reference", "prompt")
			assert.NotContains(t, rej.SafeDetail(), "SECRET_TOKEN_999")
		}

		for _, tc := range []struct {
			name      string
			body      string
			wantCode  string
			wantField string
		}{
			{"prev_resp_id", `{"model":"m","input":"hi","previous_response_id":"resp_secret_abc"}`, "unsupported_stateful_reference", "previous_response_id"},
			{"conversation_str", `{"model":"m","input":"hi","conversation":"conv_secret_abc"}`, "unsupported_stateful_reference", "conversation"},
			{"conversation_obj", `{"model":"m","input":"hi","conversation":{"id":"conv_secret_abc"}}`, "unsupported_stateful_reference", "conversation"},
			{"background_true", `{"model":"m","input":"hi","background":true}`, "unsupported_background_mode", "background"},
			{"unknown_top_field", `{"model":"m","input":"hi","future_agent_knob":true}`, "unsupported_top_level_field", "future_agent_knob"},
			{"user_empty_content_array", `{"model":"m","input":[{"role":"user","content":[]}]}`, "invalid_message_content", "content"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				_, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIResponses, []byte(tc.body))
				rej := requireRejectionError(t, err, tc.wantCode, tc.wantField)
				assert.NotContains(t, rej.SafeDetail(), "secret")
			})
		}
	})
}

// T02: Instructions, SystemParts, and developer/system message ordering.
func TestCanonicalInstructionOrder(t *testing.T) {
	respBody := []byte(`{
		"model":"gpt-4o",
		"instructions":"Top-level instructions",
		"input":[
			{"role":"developer","content":"Developer rule 1"},
			{"role":"system","content":"System rule 2"},
			{"role":"user","content":"Hello"}
		]
	}`)
	req, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIResponses, respBody)
	require.NoError(t, err)
	assert.Equal(t, "Top-level instructions", req.Instructions.Val)
	require.Len(t, req.Items, 3)
	assert.Equal(t, canonical.RoleDeveloper, req.Items[0].Role)
	assert.Equal(t, canonical.RoleSystem, req.Items[1].Role)
	assert.Equal(t, canonical.RoleUser, req.Items[2].Role)

	// OpenAI/Azure target supports developer role.
	outDev, err := EncodeCanonicalRequest(req, CanonicalTarget{
		WireFormat:            WireChatCompletions,
		Model:                 "gpt-4o",
		SupportsDeveloperRole: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "developer", gjson.GetBytes(outDev, "messages.0.role").String())
	assert.Equal(t, "Top-level instructions", gjson.GetBytes(outDev, "messages.0.content").String())
	assert.Equal(t, "developer", gjson.GetBytes(outDev, "messages.1.role").String())
	assert.Equal(t, "Developer rule 1", gjson.GetBytes(outDev, "messages.1.content").String())
	assert.Equal(t, "system", gjson.GetBytes(outDev, "messages.2.role").String())
	assert.Equal(t, "System rule 2", gjson.GetBytes(outDev, "messages.2.content").String())

	// Qwen/DeepSeek target downgrades developer -> system at encode boundary while preserving order.
	outSys, err := EncodeCanonicalRequest(req, CanonicalTarget{
		WireFormat:            WireChatCompletions,
		Model:                 "qwen-plus",
		SupportsDeveloperRole: false,
	})
	require.NoError(t, err)
	assert.Equal(t, "system", gjson.GetBytes(outSys, "messages.0.role").String())
	assert.Equal(t, "Top-level instructions", gjson.GetBytes(outSys, "messages.0.content").String())
	assert.Equal(t, "system", gjson.GetBytes(outSys, "messages.1.role").String())
	assert.Equal(t, "Developer rule 1", gjson.GetBytes(outSys, "messages.1.content").String())
	assert.Equal(t, "system", gjson.GetBytes(outSys, "messages.2.role").String())
}

// T03 + R02/R03a: OutputLimit source/scope distinction & max_tokens:0 warmup rejection.
func TestCanonicalOutputLimitScope(t *testing.T) {
	_, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, []byte(`{"model":"gpt-4o","max_tokens":100,"max_completion_tokens":200,"messages":[{"role":"user","content":"hi"}]}`))
	requireRejectionError(t, err, "conflicting_output_limits", "max_completion_tokens")

	msgZero, err := DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, []byte(`{"model":"claude-sonnet-4-6","max_tokens":0,"messages":[{"role":"user","content":"warmup"}]}`))
	require.NoError(t, err)
	assert.Equal(t, canonical.LimitMaxTokens, msgZero.Limits.Source)
	assert.True(t, msgZero.Limits.Value.IsPresent())
	assert.Equal(t, 0, msgZero.Limits.Value.Val)

	// Cross-protocol targets reject max_tokens:0 warmup with 400 before send.
	for _, wf := range []WireFormat{WireChatCompletions, WireDashScopeText, WireGeminiGenerateContent} {
		err := ValidateCanonicalTarget(msgZero, CanonicalTarget{WireFormat: wf, Model: "target-m"})
		requireRejectionError(t, err, "unsupported_zero_output_tokens", "max_tokens")
	}
	// Native Anthropic Messages supports AN02 max_tokens:0 warmup.
	require.NoError(t, ValidateCanonicalTarget(msgZero, CanonicalTarget{WireFormat: WireAnthropicMessages, Model: "claude-sonnet-4-6"}))

	// Responses max_output_tokens:0 is always rejected.
	respZero, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIResponses, []byte(`{"model":"gpt-4o","max_output_tokens":0,"input":"hi"}`))
	require.NoError(t, err)
	requireRejectionError(t, ValidateCanonicalTarget(respZero, CanonicalTarget{WireFormat: WireChatCompletions, Model: "gpt-4o"}), "unsupported_zero_output_tokens", "max_output_tokens")

	// Scope bitmask enforcement.
	chatComp, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, []byte(`{"model":"o3","max_completion_tokens":512,"messages":[{"role":"user","content":"hi"}]}`))
	require.NoError(t, err)
	assert.Equal(t, canonical.ScopeReasoningAndVisible, chatComp.Limits.Scope)
	requireRejectionError(t, ValidateCanonicalTarget(chatComp, CanonicalTarget{
		WireFormat:          WireDashScopeText,
		Model:               "qwen-turbo",
		AcceptedLimitScopes: LimitScopeMaskVisibleOnly,
	}), "unsupported_output_limit_scope", "max_completion_tokens")
	require.NoError(t, ValidateCanonicalTarget(chatComp, CanonicalTarget{
		WireFormat:          WireDashScopeText,
		Model:               "qwen3-max",
		AcceptedLimitScopes: LimitScopeMaskReasoningAndVisible,
	}))
}

// T04: Sampling zero preservation, range checks, and Chat stop union type (OA03).
func TestCanonicalSamplingAndStop(t *testing.T) {
	singleStop, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, []byte(`{"model":"gpt-4o","stop":"\n","messages":[{"role":"user","content":"hi"}]}`))
	require.NoError(t, err)
	assert.True(t, singleStop.Sampling.StopPresent)
	assert.Equal(t, []string{"\n"}, singleStop.Sampling.Stop)

	fourStops, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, []byte(`{"model":"gpt-4o","stop":["s1","s2","s3","s4"],"messages":[{"role":"user","content":"hi"}]}`))
	require.NoError(t, err)
	assert.Equal(t, []string{"s1", "s2", "s3", "s4"}, fourStops.Sampling.Stop)

	for _, invalidStop := range []string{
		`[]`,
		`["1","2","3","4","5"]`,
		`["ok", 123]`,
		`["ok", null]`,
		`123`,
		`true`,
	} {
		_, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, []byte(`{"model":"gpt-4o","stop":`+invalidStop+`,"messages":[{"role":"user","content":"hi"}]}`))
		requireRejectionError(t, err, "invalid_stop", "stop")
	}

	// Messages explicit zero temperature / top_p / top_k survive decode & encode (DEF-01).
	msgZeros, err := DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, []byte(`{
		"model":"claude-sonnet-4-6",
		"max_tokens":128,
		"temperature":0,
		"top_p":0,
		"top_k":0,
		"stop_sequences":["END"],
		"messages":[{"role":"user","content":"hi"}]
	}`))
	require.NoError(t, err)
	require.NoError(t, ValidateCanonicalTarget(msgZeros, CanonicalTarget{WireFormat: WireChatCompletions, Model: "qwen-plus"}))
	outChat, err := EncodeCanonicalRequest(msgZeros, CanonicalTarget{WireFormat: WireChatCompletions, Model: "qwen-plus"})
	require.NoError(t, err)
	assert.True(t, gjson.GetBytes(outChat, "temperature").Exists())
	assert.Zero(t, gjson.GetBytes(outChat, "temperature").Float())
	assert.True(t, gjson.GetBytes(outChat, "top_p").Exists())
	assert.Zero(t, gjson.GetBytes(outChat, "top_p").Float())
	assert.True(t, gjson.GetBytes(outChat, "top_k").Exists())
	assert.Zero(t, gjson.GetBytes(outChat, "top_k").Int())
	assert.Equal(t, "END", gjson.GetBytes(outChat, "stop.0").String())

	// Out-of-range top_p rejected at ValidateCanonicalTarget.
	badTopP, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, []byte(`{"model":"qwen-plus","top_p":1.5,"messages":[{"role":"user","content":"hi"}]}`))
	require.NoError(t, err)
	requireRejectionError(t, ValidateCanonicalTarget(badTopP, CanonicalTarget{WireFormat: WireDashScopeText, Model: "qwen-plus"}), "invalid_sampling_parameter", "top_p")
}

// T05 + H2 + DEF-04: Function strict tri-state, H2 Anthropic built-in tool rejection,
// parallel_tool_calls preservation, and Thinking + forced tool_choice conflict.
func TestCanonicalFunctionConstraints(t *testing.T) {
	t.Run("h2_messages_tool_type_validation_and_strict_tri_state", func(t *testing.T) {
		msgReq, err := DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, []byte(`{
			"model":"claude-sonnet-4-6",
			"max_tokens":512,
			"tools":[
				{"name":"fn_default","description":"no type","input_schema":{"type":"object"}},
				{"type":"custom","name":"fn_custom","input_schema":{"type":"object"},"strict":true},
				{"type":"custom","name":"fn_non_strict","input_schema":{"type":"object"},"strict":false},
				{"type":"web_search_20250305","name":"web_search"}
			],
			"tool_choice":{"type":"auto"},
			"messages":[{"role":"user","content":"search"}]
		}`))
		require.NoError(t, err)
		require.Len(t, msgReq.Tools, 4)
		assert.Equal(t, canonical.ToolKindFunction, msgReq.Tools[0].Kind)
		assert.True(t, msgReq.Tools[0].Strict.IsAbsent())
		assert.Equal(t, canonical.ToolKindFunction, msgReq.Tools[1].Kind)
		assert.Equal(t, canonical.Some(true), msgReq.Tools[1].Strict)
		assert.Equal(t, canonical.Some(false), msgReq.Tools[2].Strict)
		assert.Equal(t, canonical.ToolKindFixedServer, msgReq.Tools[3].Kind)
		assert.Equal(t, "web_search_20250305", msgReq.Tools[3].FixedType)
		// DEF-04: omitting disable_parallel_tool_use must NOT force ParallelToolCalls=true.
		assert.True(t, msgReq.ParallelToolCalls.IsAbsent())

		// Target without SupportsStrictSchema rejects strict:true tool.
		requireRejectionError(t, ValidateCanonicalTarget(msgReq, CanonicalTarget{
			WireFormat:           WireChatCompletions,
			Model:                "qwen-plus",
			SupportsStrictSchema: false,
		}), "unsupported_strict_schema", "tools")

		// Target with SupportsStrictSchema:true accepts and encodes strict:true and strict:false accurately.
		require.NoError(t, ValidateCanonicalTarget(msgReq, CanonicalTarget{
			WireFormat:           WireChatCompletions,
			Model:                "gpt-4o",
			SupportsStrictSchema: true,
		}))
		encoded, err := EncodeCanonicalRequest(msgReq, CanonicalTarget{
			WireFormat:           WireChatCompletions,
			Model:                "gpt-4o",
			SupportsStrictSchema: true,
		})
		require.NoError(t, err)
		assert.False(t, gjson.GetBytes(encoded, "parallel_tool_calls").Exists())
		assert.False(t, gjson.GetBytes(encoded, "tools.0.function.strict").Exists())
		assert.Equal(t, "true", gjson.GetBytes(encoded, "tools.1.function.strict").Raw)
		assert.Equal(t, "false", gjson.GetBytes(encoded, "tools.2.function.strict").Raw)

		// H2: Anthropic server/computer-use built-in tools rejected with 400 unsupported_tool_type (field="tools").
		for _, builtinType := range []string{
			"bash_20250124",
			"text_editor_20250124",
			"text_editor_20250728",
			"computer_20250124",
			"tool_search_tool_regex_20251119",
		} {
			_, err := DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, []byte(`{
				"model":"claude-sonnet-4-6",
				"max_tokens":256,
				"tools":[{"type":"`+builtinType+`","name":"builtin"}],
				"messages":[{"role":"user","content":"run"}]
			}`))
			requireRejectionError(t, err, "unsupported_tool_type", "tools")
		}
	})

	t.Run("thinking_and_forced_tool_choice_conflict_rejected", func(t *testing.T) {
		msgThinkForced, err := DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, []byte(`{
			"model":"claude-sonnet-4-6",
			"max_tokens":2048,
			"thinking":{"type":"enabled","budget_tokens":1024},
			"tools":[{"name":"ping","input_schema":{"type":"object"}}],
			"tool_choice":{"type":"any","disable_parallel_tool_use":true},
			"messages":[{"role":"user","content":"ping"}]
		}`))
		require.NoError(t, err)
		assert.Equal(t, canonical.Some(false), msgThinkForced.ParallelToolCalls)

		requireRejectionError(t, ValidateCanonicalTarget(msgThinkForced, CanonicalTarget{
			WireFormat:       WireChatCompletions,
			Model:            "qwen-plus",
			SupportsThinking: true,
		}), "conflicting_thinking_tool_choice", "tool_choice")

		chatAdaptiveForced, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, []byte(`{
			"model":"qwen-plus",
			"enable_thinking":true,
			"tools":[{"type":"function","function":{"name":"ping","parameters":{"type":"object"}}}],
			"tool_choice":{"type":"function","function":{"name":"ping"}},
			"messages":[{"role":"user","content":"ping"}]
		}`))
		require.NoError(t, err)
		requireRejectionError(t, ValidateCanonicalTarget(chatAdaptiveForced, CanonicalTarget{
			WireFormat:       WireDashScopeText,
			Model:            "qwen-plus",
			SupportsThinking: true,
		}), "conflicting_thinking_tool_choice", "tool_choice")
	})

	t.Run("ds05_json_schema_capability_check", func(t *testing.T) {
		respSchema, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIResponses, []byte(`{
			"model":"qwen-plus",
			"input":"output json",
			"text":{"format":{"type":"json_schema","name":"item","schema":{"type":"object"},"strict":true}}
		}`))
		require.NoError(t, err)
		requireRejectionError(t, ValidateCanonicalTarget(respSchema, CanonicalTarget{
			WireFormat:           WireDashScopeText,
			Model:                "qwen-turbo",
			SupportsStrictSchema: false,
		}), "unsupported_strict_schema", "response_format")
	})
}

// T06 + AN08: Parallel tool associations, item_id != call_id, and Messages AN08 ordering.
func TestCanonicalToolAssociations(t *testing.T) {
	t.Run("responses_distinct_item_id_and_call_id_parallel_roundtrip", func(t *testing.T) {
		req, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIResponses, []byte(`{
			"model":"gpt-4o",
			"input":[
				{"role":"user","content":"lookup both"},
				{"type":"function_call","id":"fc_item_1","call_id":"call_real_1","name":"lookup","arguments":"{\"k\":\"a\"}"},
				{"type":"function_call","id":"fc_item_2","call_id":"call_real_2","name":"lookup","arguments":"{\"k\":\"b\"}"},
				{"type":"function_call_output","id":"fco_item_2","call_id":"call_real_2","output":"res_b"},
				{"type":"function_call_output","id":"fco_item_1","call_id":"call_real_1","output":"res_a"}
			]
		}`))
		require.NoError(t, err)
		assert.Equal(t, "fc_item_1", req.Items[1].Call.ItemID)
		assert.Equal(t, "call_real_1", req.Items[1].Call.CallID)
		assert.Equal(t, "fco_item_2", req.Items[3].Result.ItemID)
		assert.Equal(t, "call_real_2", req.Items[3].Result.CallID)
		assert.Equal(t, "lookup", req.Items[3].Result.Name)
	})

	t.Run("an08_messages_tool_result_ordering_enforced", func(t *testing.T) {
		// Valid: tool_result immediately follows assistant tool_use and precedes user text in same turn.
		validBody := []byte(`{
			"model":"claude-sonnet-4-6",
			"max_tokens":256,
			"messages":[
				{"role":"user","content":"check both"},
				{"role":"assistant","content":[
					{"type":"tool_use","id":"tu_1","name":"lookup","input":{"q":"1"}},
					{"type":"tool_use","id":"tu_2","name":"lookup","input":{"q":"2"}}
				]},
				{"role":"user","content":[
					{"type":"tool_result","tool_use_id":"tu_1","content":"r1"},
					{"type":"tool_result","tool_use_id":"tu_2","content":"r2"},
					{"type":"text","text":"Now summarize"}
				]}
			]
		}`)
		req, err := DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, validBody)
		require.NoError(t, err)
		require.Len(t, req.Items, 6)

		// Invalid 1: text block precedes tool_result in the same user message.
		_, err = DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, []byte(`{
			"model":"claude-sonnet-4-6",
			"max_tokens":256,
			"messages":[
				{"role":"user","content":"check"},
				{"role":"assistant","content":[{"type":"tool_use","id":"tu_1","name":"lookup","input":{}}]},
				{"role":"user","content":[
					{"type":"text","text":" premature text "},
					{"type":"tool_result","tool_use_id":"tu_1","content":"r1"}
				]}
			]
		}`))
		requireRejectionError(t, err, "invalid_tool_result_order", "tool_result")

		// Invalid 2: unrelated user message inserted before tool_result.
		_, err = DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, []byte(`{
			"model":"claude-sonnet-4-6",
			"max_tokens":256,
			"messages":[
				{"role":"user","content":"check"},
				{"role":"assistant","content":[{"type":"tool_use","id":"tu_1","name":"lookup","input":{}}]},
				{"role":"user","content":"interrupted"},
				{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_1","content":"r1"}]}
			]
		}`))
		requireRejectionError(t, err, "missing_tool_result", "tool_result")

		// Invalid 3: orphan tool_result with no preceding assistant tool_use.
		_, err = DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, []byte(`{
			"model":"claude-sonnet-4-6",
			"max_tokens":256,
			"messages":[
				{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_orphan","content":"r1"}]}
			]
		}`))
		requireRejectionError(t, err, "orphan_tool_result", "tool_result")
	})
}

// T07: >53-bit integer 9007199254740993 precision preservation across schemas, arguments, and responses.
func TestCanonicalArgumentRepresentations(t *testing.T) {
	const bigIntLiteral = "9007199254740993"

	msgBody := []byte(`{
		"model":"claude-sonnet-4-6",
		"max_tokens":256,
		"tools":[{
			"name":"ledger_query",
			"input_schema":{"type":"object","properties":{"tx_id":{"type":"integer","const":9007199254740993}}}
		}],
		"messages":[
			{"role":"user","content":"query"},
			{"role":"assistant","content":[
				{"type":"tool_use","id":"tu_big","name":"ledger_query","input":{"tx_id":9007199254740993,"nested":{"ids":[9007199254740993]}}}
			]},
			{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"tu_big","content":"{\"ok\":true}"}
			]}
		]
	}`)
	req, err := DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, msgBody)
	require.NoError(t, err)
	assert.Contains(t, string(req.Tools[0].Schema), bigIntLiteral)
	assert.Contains(t, string(req.Items[1].Call.Args.Object), bigIntLiteral)

	chatOut, err := EncodeCanonicalRequest(req, CanonicalTarget{WireFormat: WireChatCompletions, Model: "qwen-plus"})
	require.NoError(t, err)
	assert.Contains(t, gjson.GetBytes(chatOut, "tools.0.function.parameters").Raw, bigIntLiteral)
	assert.Equal(t, `{"tx_id":9007199254740993,"nested":{"ids":[9007199254740993]}}`, gjson.GetBytes(chatOut, "messages.1.tool_calls.0.function.arguments").String())

	// ResponseFormat JSON Schema precision in Responses -> Chat.
	respSchemaBody := []byte(`{
		"model":"gpt-4o",
		"input":"check",
		"text":{"format":{"type":"json_schema","name":"tx","schema":{"type":"object","properties":{"id":{"const":9007199254740993}}},"strict":true}}
	}`)
	respReq, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIResponses, respSchemaBody)
	require.NoError(t, err)
	respChatOut, err := EncodeCanonicalRequest(respReq, CanonicalTarget{
		WireFormat:           WireChatCompletions,
		Model:                "gpt-4o",
		SupportsStrictSchema: true,
	})
	require.NoError(t, err)
	assert.Contains(t, gjson.GetBytes(respChatOut, "response_format.json_schema.schema").Raw, bigIntLiteral)
}

// T08 + H2: Tool result empty/error/multimodal split on visual targets vs 400 reject on dashscope_text & Responses.
func TestCanonicalToolResultSemantics(t *testing.T) {
	msgWithImgToolResult := []byte(`{
		"model":"claude-sonnet-4-6",
		"max_tokens":512,
		"messages":[
			{"role":"user","content":"take screenshot"},
			{"role":"assistant","content":[
				{"type":"tool_use","id":"call_nil","name":"noop","input":{}},
				{"type":"tool_use","id":"call_err","name":"fail_op","input":{}},
				{"type":"tool_use","id":"call_img","name":"screenshot","input":{}}
			]},
			{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"call_nil"},
				{"type":"tool_result","tool_use_id":"call_err","is_error":true,"content":"permission denied"},
				{"type":"tool_result","tool_use_id":"call_img","content":[
					{"type":"text","text":"Captured screen"},
					{"type":"image","source":{"type":"base64","media_type":"image/png","data":"iVBORw0KGgo="}}
				]}
			]}
		]
	}`)
	req, err := DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, msgWithImgToolResult)
	require.NoError(t, err)
	assert.True(t, req.Items[5].Result.IsError)
	assert.Equal(t, "permission denied", req.Items[5].Result.Parts[0].Text)

	// H2: Pure-text dashscope_text rejects Messages multimodal tool_result with 400 before send.
	requireRejectionError(t, ValidateCanonicalTarget(req, CanonicalTarget{
		WireFormat: WireDashScopeText,
		Model:      "qwen-plus",
	}), "unsupported_image_content", "tool_result")

	// H2: Visual Chat target accepts and emits role:"tool" + companion role:"user" with "Attached image(s) from tool result:".
	visualTarget := CanonicalTarget{
		WireFormat:        WireChatCompletions,
		Model:             "gpt-4o",
		SupportsURLImages: true,
	}
	require.NoError(t, ValidateCanonicalTarget(req, visualTarget))
	outChat, err := EncodeCanonicalRequest(req, visualTarget)
	require.NoError(t, err)
	require.Equal(t, int64(6), gjson.GetBytes(outChat, "messages.#").Int(), string(outChat))
	assert.Equal(t, "tool", gjson.GetBytes(outChat, "messages.2.role").String())
	assert.Equal(t, "", gjson.GetBytes(outChat, "messages.2.content").String())
	assert.Equal(t, "tool", gjson.GetBytes(outChat, "messages.3.role").String())
	assert.Equal(t, "permission denied", gjson.GetBytes(outChat, "messages.3.content").String())
	assert.Equal(t, "tool", gjson.GetBytes(outChat, "messages.4.role").String())
	assert.Equal(t, "Captured screen", gjson.GetBytes(outChat, "messages.4.content").String())
	assert.Equal(t, "user", gjson.GetBytes(outChat, "messages.5.role").String())
	assert.Equal(t, "Attached image(s) from tool result:", gjson.GetBytes(outChat, "messages.5.content.0.text").String())
	assert.Equal(t, "data:image/png;base64,iVBORw0KGgo=", gjson.GetBytes(outChat, "messages.5.content.1.image_url.url").String())

	// Messages tool_result with document block rejected at decode.
	_, err = DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, []byte(`{
		"model":"claude-sonnet-4-6",
		"max_tokens":128,
		"messages":[
			{"role":"user","content":"doc"},
			{"role":"assistant","content":[{"type":"tool_use","id":"tu_doc","name":"read","input":{}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_doc","content":[{"type":"document","source":{"type":"base64","media_type":"application/pdf","data":"AAAA"}}]}]}
		]
	}`))
	requireRejectionError(t, err, "unsupported_tool_result_content", "document")

	// Responses function_call_output with input_image remains rejected with 400.
	_, err = DecodeCanonicalRequest(canonical.ProtocolOpenAIResponses, []byte(`{
		"model":"gpt-4o",
		"input":[
			{"type":"function_call","call_id":"c1","name":"shot","arguments":"{}"},
			{"type":"function_call_output","call_id":"c1","output":[{"type":"input_image","image_url":"data:image/png;base64,AAAA"}]}
		]
	}`))
	requireRejectionError(t, err, "unsupported_image_content", "input_image")
}

// T09 + OA09: Image URL/DataURL zero-reencode, detail:"original" capability check, file_id & Gemini remote URL reject.
func TestCanonicalImageRepresentations(t *testing.T) {
	respOrig, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIResponses, []byte(`{
		"model":"gpt-4o",
		"input":[{"role":"user","content":[
			{"type":"input_image","image_url":"data:image/jpeg;base64,/9j/4AAQSkZJRg==","detail":"original"},
			{"type":"input_image","image_url":"https://example.com/diagram.png","detail":"high"}
		]}]
	}`))
	require.NoError(t, err)
	img0 := respOrig.Items[0].Parts[0].Image
	require.NotNil(t, img0)
	assert.Equal(t, canonical.ImageDataURL, img0.Source)
	assert.Equal(t, "image/jpeg", img0.MediaType)
	assert.Equal(t, "/9j/4AAQSkZJRg==", img0.Base64Ref)
	assert.Equal(t, canonical.ImageDetailOriginal, img0.Detail)

	// Target without explicit "original" in AllowedImageDetails rejects detail:"original".
	requireRejectionError(t, ValidateCanonicalTarget(respOrig, CanonicalTarget{
		WireFormat:          WireChatCompletions,
		Model:               "qwen-vl-max",
		SupportsURLImages:   true,
		AllowedImageDetails: []canonical.ImageDetail{canonical.ImageDetailAuto, canonical.ImageDetailLow, canonical.ImageDetailHigh},
	}), "unsupported_image_detail", "detail")

	// Target with "original" allowed accepts it.
	require.NoError(t, ValidateCanonicalTarget(respOrig, CanonicalTarget{
		WireFormat:          WireChatCompletions,
		Model:               "gpt-4.1",
		SupportsURLImages:   true,
		AllowedImageDetails: []canonical.ImageDetail{canonical.ImageDetailAuto, canonical.ImageDetailLow, canonical.ImageDetailHigh, canonical.ImageDetailOriginal},
	}))

	// Managed Gemini rejects remote http(s):// image URLs (P05 zero network fetch), while allowing Data URLs.
	requireRejectionError(t, ValidateCanonicalTarget(respOrig, CanonicalTarget{
		WireFormat:          WireGeminiGenerateContent,
		Model:               "gemini-2.5-flash",
		SupportsURLImages:   false,
		AllowedImageDetails: []canonical.ImageDetail{canonical.ImageDetailAuto, canonical.ImageDetailLow, canonical.ImageDetailHigh, canonical.ImageDetailOriginal},
	}), "unsupported_image_url", "image_url")

	dataURLOnly, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, []byte(`{
		"model":"gemini-2.5-flash",
		"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,iVBORw0KGgo="}}]}]
	}`))
	require.NoError(t, err)
	require.NoError(t, ValidateCanonicalTarget(dataURLOnly, CanonicalTarget{
		WireFormat:        WireGeminiGenerateContent,
		Model:             "gemini-2.5-flash",
		SupportsURLImages: false,
	}))

	// dashscope_text rejects any image part.
	requireRejectionError(t, ValidateCanonicalTarget(dataURLOnly, CanonicalTarget{
		WireFormat: WireDashScopeText,
		Model:      "qwen-plus",
	}), "unsupported_image_content", "image_url")

	// file_id image rejected at ValidateCanonicalTarget & EncodeCanonicalRequest.
	respFileID, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIResponses, []byte(`{
		"model":"gpt-4o",
		"input":[{"role":"user","content":[{"type":"input_image","file_id":"file_abc123"}]}]
	}`))
	require.NoError(t, err)
	requireRejectionError(t, ValidateCanonicalTarget(respFileID, CanonicalTarget{
		WireFormat:        WireChatCompletions,
		Model:             "gpt-4o",
		SupportsURLImages: true,
	}), "unsupported_image_content", "file_id")
}

// T10 + H3 + AN05: Cache boundaries, AN05 TTL ordering & Thinking cache forbid,
// Qwen compatible H3 content[].cache_control preservation, and assistant merge breakpoint transfer.
func TestCanonicalCacheBoundaries(t *testing.T) {
	t.Run("an05_ttl_order_and_thinking_cache_control_rejection", func(t *testing.T) {
		// Valid: 1h in tools -> 1h in system -> 5m in messages (image + text) -> default ephemeral top-level.
		validMsg := []byte(`{
			"model":"claude-sonnet-4-6",
			"max_tokens":256,
			"cache_control":{"type":"ephemeral"},
			"tools":[{"name":"t1","input_schema":{"type":"object"},"cache_control":{"type":"ephemeral","ttl":"1h"}}],
			"system":[{"type":"text","text":"sys","cache_control":{"type":"ephemeral","ttl":"1h"}}],
			"messages":[{"role":"user","content":[
				{"type":"image","source":{"type":"base64","media_type":"image/png","data":"AAAA"},"cache_control":{"type":"ephemeral","ttl":"5m"}},
				{"type":"text","text":"hello","cache_control":{"type":"ephemeral"}}
			]}]
		}`)
		req, err := DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, validMsg)
		require.NoError(t, err)
		require.NotNil(t, req.Tools[0].Cache)
		assert.Equal(t, "1h", req.Tools[0].Cache.TTL)
		require.NotNil(t, req.SystemParts[0].Cache)
		assert.Equal(t, "1h", req.SystemParts[0].Cache.TTL)
		require.NotNil(t, req.Items[0].Parts[0].Cache)
		assert.Equal(t, "5m", req.Items[0].Parts[0].Cache.TTL)
		require.NotNil(t, req.Cache.RequestBoundary)

		// Invalid AN05: 5m in system followed by 1h in messages.
		_, err = DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, []byte(`{
			"model":"claude-sonnet-4-6",
			"max_tokens":256,
			"system":[{"type":"text","text":"sys","cache_control":{"type":"ephemeral","ttl":"5m"}}],
			"messages":[{"role":"user","content":[{"type":"text","text":"hi","cache_control":{"type":"ephemeral","ttl":"1h"}}]}]
		}`))
		requireRejectionError(t, err, "invalid_cache_control_ttl_order", "messages")

		// Invalid AN05: cache_control on thinking block.
		_, err = DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, []byte(`{
			"model":"claude-sonnet-4-6",
			"max_tokens":256,
			"messages":[
				{"role":"user","content":"hi"},
				{"role":"assistant","content":[{"type":"thinking","thinking":"hmm","signature":"sig","cache_control":{"type":"ephemeral"}}]},
				{"role":"user","content":"next"}
			]
		}`))
		requireRejectionError(t, err, "invalid_cache_control", "thinking")
	})

	t.Run("h3_qwen_compatible_policy_preserves_content_cache_control_and_strips_outer", func(t *testing.T) {
		chatIn := []byte(`{
			"model":"qwen3.6-plus",
			"store":true,
			"prompt_cache_key":"pck_1",
			"prompt_cache_retention":"24h",
			"tools":[{
				"type":"function",
				"cache_control":{"type":"ephemeral"},
				"function":{"name":"read_file","parameters":{"type":"object"}}
			}],
			"messages":[
				{"role":"developer","content":"Be concise."},
				{"role":"user","content":[{"type":"text","text":"Inspect","cache_control":{"type":"ephemeral"}}]},
				{
					"role":"assistant",
					"content":null,
					"reasoning_content":"Thinking trace",
					"cache_control":{"type":"ephemeral"},
					"tool_calls":[{
						"id":"call_1",
						"type":"function",
						"extra_content":{"google":{"thought_signature":"sig"}},
						"function":{"name":"read_file","arguments":"{}"}
					}]
				},
				{"role":"tool","tool_call_id":"call_1","name":"read_file","content":"ok"}
			]
		}`)
		req, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, chatIn)
		require.NoError(t, err)

		target := CanonicalTarget{
			WireFormat:       WireChatCompletions,
			Supplier:         "qwen",
			Model:            "qwen3.6-plus",
			SupportsThinking: true,
		}
		qwenCompatibleChatPolicy("qwen3.6-plus").applyToTarget(&target)

		out, err := EncodeCanonicalRequest(req, target)
		require.NoError(t, err)

		assert.True(t, gjson.GetBytes(out, "preserve_thinking").Bool())
		assert.False(t, gjson.GetBytes(out, "store").Exists())
		assert.False(t, gjson.GetBytes(out, "prompt_cache_key").Exists())
		assert.False(t, gjson.GetBytes(out, "prompt_cache_retention").Exists())
		assert.False(t, gjson.GetBytes(out, "tools.0.cache_control").Exists(), "outer tool cache_control must be stripped")
		assert.Equal(t, "system", gjson.GetBytes(out, "messages.0.role").String())
		// H3: messages[].content[].cache_control MUST be preserved!
		assert.Equal(t, "ephemeral", gjson.GetBytes(out, "messages.1.content.0.cache_control.type").String())
		assert.Equal(t, `""`, gjson.GetBytes(out, "messages.2.content").Raw)
		assert.Equal(t, "Thinking trace", gjson.GetBytes(out, "messages.2.reasoning_content").String())
		assert.False(t, gjson.GetBytes(out, "messages.2.cache_control").Exists(), "outer message cache_control must be stripped")
		assert.False(t, gjson.GetBytes(out, "messages.2.tool_calls.0.extra_content").Exists())
		assert.False(t, gjson.GetBytes(out, "messages.3.name").Exists())
	})

	t.Run("assistant_segment_merge_transfers_breakpoints_and_rejects_conflict", func(t *testing.T) {
		validMerge, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIResponses, []byte(`{
			"model":"gpt-4o",
			"input":[
				{"role":"user","content":"go"},
				{"role":"assistant","content":"seg1","prompt_cache_breakpoint":{"mode":"explicit","ttl":"30m"}},
				{"type":"function_call","call_id":"c1","name":"fn","arguments":"{}"},
				{"role":"assistant","content":"seg2","prompt_cache_breakpoint":{"mode":"explicit","ttl":"12h"}},
				{"type":"function_call","call_id":"c2","name":"fn","arguments":"{}"},
				{"type":"function_call_output","call_id":"c1","output":"o1"},
				{"type":"function_call_output","call_id":"c2","output":"o2"}
			]
		}`))
		require.NoError(t, err)
		out, err := EncodeCanonicalRequest(validMerge, CanonicalTarget{
			WireFormat:             WireChatCompletions,
			Model:                  "gpt-4o",
			SupportsPromptCacheKey: true,
		})
		require.NoError(t, err)
		assert.Equal(t, "30m", gjson.GetBytes(out, "messages.1.content.0.prompt_cache_breakpoint.ttl").String())
		assert.Equal(t, "12h", gjson.GetBytes(out, "messages.1.content.1.prompt_cache_breakpoint.ttl").String())
		assert.False(t, gjson.GetBytes(out, "messages.1.prompt_cache_breakpoint").Exists())

		conflictMerge, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIResponses, []byte(`{
			"model":"gpt-4o",
			"input":[
				{"role":"user","content":"go"},
				{"role":"assistant","content":[{"type":"output_text","text":"seg1","prompt_cache_breakpoint":{"mode":"explicit","ttl":"30m"}}],"prompt_cache_breakpoint":{"mode":"explicit","ttl":"12h"}},
				{"type":"function_call","call_id":"c1","name":"fn","arguments":"{}"},
				{"role":"assistant","content":"seg2"},
				{"type":"function_call_output","call_id":"c1","output":"o1"}
			]
		}`))
		require.NoError(t, err)
		_, err = EncodeCanonicalRequest(conflictMerge, CanonicalTarget{
			WireFormat:             WireChatCompletions,
			Model:                  "gpt-4o",
			SupportsPromptCacheKey: true,
		})
		requireRejectionError(t, err, "conflicting_cache_breakpoints", "prompt_cache_breakpoint")
	})
}

// T11: Adaptive thinking + output_config.effort, explicit disabled guard, and stripCchFromBillingHeader.
func TestCanonicalThinkingModes(t *testing.T) {
	for _, tc := range []struct {
		effortField string
		wantEffort  string
	}{
		{"", "medium"},
		{`,"output_config":{"effort":"low"}`, "low"},
		{`,"output_config":{"effort":"medium"}`, "medium"},
		{`,"output_config":{"effort":"high"}`, "high"},
		{`,"output_config":{"effort":"max"}`, "high"},
	} {
		req, err := DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, []byte(`{
			"model":"claude-sonnet-4-6",
			"max_tokens":1024,
			"system":"x-anthropic-billing-header: cc_version=2.1.37; cc_entrypoint=cli; cch=9f8e7d6c;",
			"thinking":{"type":"adaptive"}`+tc.effortField+`,
			"messages":[{"role":"user","content":"hi"}]
		}`))
		require.NoError(t, err)
		assert.Equal(t, canonical.ThinkingAdaptive, req.Thinking.Mode)
		assert.Equal(t, canonical.Some(tc.wantEffort), req.Thinking.Effort)
		assert.Equal(t, "x-anthropic-billing-header: cc_version=2.1.37; cc_entrypoint=cli;", req.SystemParts[0].Text)

		out, err := EncodeCanonicalRequest(req, CanonicalTarget{
			WireFormat:       WireChatCompletions,
			Model:            "qwen-plus",
			SupportsThinking: true,
		})
		require.NoError(t, err)
		assert.Equal(t, tc.wantEffort, gjson.GetBytes(out, "reasoning_effort").String())
		assert.Equal(t, "x-anthropic-billing-header: cc_version=2.1.37; cc_entrypoint=cli;", gjson.GetBytes(out, "messages.0.content").String())
	}

	// Explicit disabled thinking must not be re-opened by output_config.effort or historical reasoning.
	disabledReq, err := DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, []byte(`{
		"model":"claude-sonnet-4-6",
		"max_tokens":1024,
		"thinking":{"type":"disabled"},
		"output_config":{"effort":"high"},
		"messages":[
			{"role":"user","content":"q1"},
			{"role":"assistant","content":[{"type":"thinking","thinking":"old","signature":"sig1"},{"type":"text","text":"a1"}]},
			{"role":"user","content":"q2"}
		]
	}`))
	require.NoError(t, err)
	assert.Equal(t, canonical.ThinkingDisabled, disabledReq.Thinking.Mode)
	assert.True(t, disabledReq.Thinking.Effort.IsAbsent())

	target := CanonicalTarget{WireFormat: WireChatCompletions, Supplier: "qwen", Model: "qwen3.6-plus", SupportsThinking: true}
	qwenCompatibleChatPolicy("qwen3.6-plus").applyToTarget(&target)
	outDisabled, err := EncodeCanonicalRequest(disabledReq, target)
	require.NoError(t, err)
	assert.False(t, gjson.GetBytes(outDisabled, "reasoning_effort").Exists())
	assert.False(t, gjson.GetBytes(outDisabled, "preserve_thinking").Exists())
}

// T12 + H1: Opaque reasoning replay & cross-wire skip vs bare {"type":"reasoning"} rejection.
func TestCanonicalOpaqueOwnership(t *testing.T) {
	t.Run("responses_opaque_and_empty_summary_skipped_while_bare_reasoning_rejected", func(t *testing.T) {
		body := []byte(`{
			"model":"qwen-plus",
			"input":[
				{"role":"user","content":"start"},
				{"type":"reasoning","id":"rs_empty_sum","summary":[],"encrypted_content":"gAAAAABopaque"},
				{"type":"message","role":"assistant","content":"Checking"},
				{"type":"function_call","call_id":"c1","name":"ls","arguments":"{}"},
				{"type":"function_call_output","call_id":"c1","output":"a.go"},
				{"type":"reasoning","id":"rs_sum_only","summary":[{"type":"summary_text","text":"summary"}]},
				{"type":"reasoning","id":"rs_plain","summary":[],"content":[{"type":"reasoning_text","text":"explicit thought"}]},
				{"role":"assistant","content":"Done"}
			]
		}`)
		req, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIResponses, body)
		require.NoError(t, err)
		assert.True(t, req.Items[1].Reasoning.Summary.IsPresent())
		assert.Empty(t, req.Items[1].Reasoning.Summary.Val)
		assert.Equal(t, `"gAAAAABopaque"`, string(req.Items[1].Reasoning.Encrypted))

		out, err := EncodeCanonicalRequest(req, CanonicalTarget{
			WireFormat:       WireChatCompletions,
			Supplier:         "qwen",
			Model:            "qwen-plus",
			SupportsThinking: true,
		})
		require.NoError(t, err)
		require.Equal(t, int64(4), gjson.GetBytes(out, "messages.#").Int(), string(out))
		assert.False(t, gjson.GetBytes(out, "messages.1.reasoning_content").Exists())
		assert.Equal(t, "Checking", gjson.GetBytes(out, "messages.1.content").String())
		assert.Equal(t, "explicit thought", gjson.GetBytes(out, "messages.3.reasoning_content").String())
		assert.Equal(t, "Done", gjson.GetBytes(out, "messages.3.content").String())

		// Bare {"type":"reasoning"} without content, encrypted_content, or summary is rejected.
		_, err = DecodeCanonicalRequest(canonical.ProtocolOpenAIResponses, []byte(`{"model":"qwen-plus","input":[{"type":"reasoning"}]}`))
		requireRejectionError(t, err, "unsupported_reasoning_content", "reasoning")
	})

	t.Run("messages_signature_only_and_redacted_thinking_degrade_to_empty_assistant_on_cross_wire", func(t *testing.T) {
		msgOpaque, err := DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, []byte(`{
			"model":"claude-sonnet-4-6",
			"max_tokens":256,
			"messages":[
				{"role":"user","content":"step 1"},
				{"role":"assistant","content":[
					{"type":"thinking","thinking":"","signature":"sig_empty_text"},
					{"type":"redacted_thinking","data":"EgoBopaqueRedacted"}
				]},
				{"role":"user","content":"step 2"}
			]
		}`))
		require.NoError(t, err)
		require.Len(t, msgOpaque.Items, 4)
		assert.Equal(t, "sig_empty_text", msgOpaque.Items[1].Reasoning.Signature)
		assert.Equal(t, "EgoBopaqueRedacted", msgOpaque.Items[2].Reasoning.Redacted)

		out, err := EncodeCanonicalRequest(msgOpaque, CanonicalTarget{
			WireFormat: WireChatCompletions,
			Supplier:   "openai",
			Model:      "gpt-4o",
		})
		require.NoError(t, err)
		require.Equal(t, int64(3), gjson.GetBytes(out, "messages.#").Int(), string(out))
		assert.Equal(t, "assistant", gjson.GetBytes(out, "messages.1.role").String())
		assert.Equal(t, `""`, gjson.GetBytes(out, "messages.1.content").Raw)
	})
}

// T14 + T15: Chat non-streaming response decoder & stateful SSE event decoder.
func TestCanonicalChatResponseAndStream(t *testing.T) {
	t.Run("non_streaming_response_preserves_tool_null_content_big_int_and_unknown_finish_reason", func(t *testing.T) {
		target := CanonicalTarget{WireFormat: WireChatCompletions, Model: "qwen-plus"}
		rawResp := []byte(`{
			"id":"chatcmpl-99",
			"created":1700000000,
			"model":"qwen-plus",
			"service_tier":"default",
			"choices":[
				{
					"index":0,
					"message":{
						"role":"assistant",
						"content":null,
						"reasoning_content":"step-by-step",
						"tool_calls":[{
							"id":"call_big",
							"type":"function",
							"extra_content":{"google":{"thought_signature":"gsig_1"}},
							"function":{"name":"lookup","arguments":"{\"id\":9007199254740993}"}
						}]
					},
					"finish_reason":"tool_calls"
				},
				{
					"index":1,
					"message":{"role":"assistant","refusal":"Cannot assist"},
					"finish_reason":"vendor_custom_halt"
				}
			]
		}`)
		resp, err := DecodeCanonicalResponse(target, rawResp)
		require.NoError(t, err)
		assert.Equal(t, "chatcmpl-99", resp.ID)
		require.Len(t, resp.Choices, 2)

		// Choice 0: reasoning + tool_call only (no empty text ItemMessage from content:null).
		c0 := resp.Choices[0]
		assert.Equal(t, canonical.FinishToolCalls, c0.Finish.Category)
		require.Len(t, c0.Items, 2)
		assert.Equal(t, canonical.ItemReasoning, c0.Items[0].Kind)
		assert.Equal(t, []string{"step-by-step"}, c0.Items[0].Reasoning.Text)
		assert.Equal(t, canonical.ItemFunctionCall, c0.Items[1].Kind)
		assert.Equal(t, `{"id":9007199254740993}`, c0.Items[1].Call.Args.Text)
		assert.Equal(t, "gsig_1", c0.Items[1].Call.ThoughtSignature)

		// Choice 1: refusal + unknown finish_reason preserved as FinishUnknown (never guessed as FinishStop).
		c1 := resp.Choices[1]
		assert.Equal(t, canonical.FinishUnknown, c1.Finish.Category)
		assert.Equal(t, "vendor_custom_halt", c1.Finish.RawReason)
		require.Len(t, c1.Items, 1)
		assert.Equal(t, canonical.PartRefusal, c1.Items[0].Parts[0].Kind)
		assert.Equal(t, "Cannot assist", c1.Items[0].Parts[0].Refusal)
	})

	t.Run("sse_stream_decoder_dual_index_fragmented_tools_role_omitted_and_usage_only", func(t *testing.T) {
		dec, err := NewCanonicalEventDecoder(CanonicalTarget{WireFormat: WireChatCompletions, Streaming: true})
		require.NoError(t, err)

		frames := []string{
			// 1. Role omitted, choice 0 reasoning + choice 1 refusal.
			`{"id":"chatcmpl-s1","created":1700000001,"model":"qwen-plus","choices":[{"index":0,"delta":{"reasoning_content":"think-1"}},{"index":1,"delta":{"refusal":"no"}}]}`,
			// 2. Choice 0 tool 0 split: id arrives first without name.
			`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_0_0","type":"function","function":{"name":"","arguments":""}}]}}]}`,
			// 3. Choice 0 tool 0 name + fragmented args part 1; tool 1 full start; choice 0 text delta; choice 1 tool 0 (same ToolIndex=0 in choice 1!).
			`{"choices":[{"index":0,"delta":{"content":"hello","tool_calls":[{"index":0,"function":{"name":"get_","arguments":"{\"id\":"}},{"index":1,"id":"call_0_1","type":"function","function":{"name":"ping","arguments":"{}"}}]}},{"index":1,"delta":{"tool_calls":[{"index":0,"id":"call_1_0","type":"function","function":{"name":"other","arguments":"{\"x\":1}"}}]}}]}`,
			// 4. Choice 0 tool 0 remaining name + big integer args part 2.
			`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"name":"tx","arguments":"9007199254740993}"}}]}}]}`,
			// 5. Finish reasons (choice 0 tool_calls, choice 1 unknown custom reason).
			`{"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"},{"index":1,"delta":{},"finish_reason":"custom_limit"}]}`,
			// 6. Usage-only frame (choices: []).
			`{"id":"chatcmpl-s1","choices":[],"usage":{"prompt_tokens":12,"completion_tokens":34,"total_tokens":46}}`,
			// 7. Terminal [DONE].
			`[DONE]`,
		}

		var events []canonical.Event
		for _, frame := range frames {
			require.NoError(t, dec.Decode("", []byte(frame), func(ev canonical.Event) error {
				events = append(events, ev)
				return nil
			}))
		}
		require.NoError(t, dec.End())

		require.NotEmpty(t, events)
		assert.Equal(t, canonical.EventResponseStart, events[0].Kind)
		assert.Equal(t, canonical.RoleAssistant, events[0].Role)
		assert.Equal(t, canonical.EventProtocolDone, events[len(events)-1].Kind)

		var c0Tool0Args strings.Builder
		var c0Tool0StartName, c0Tool0LastName, c1Tool0CallID string
		var c0ReasoningItemIdx, c0TextItemIdx, c0Tool0ItemIdx, c0Tool1ItemIdx int
		var c0Finish, c1Finish canonical.FinishReason

		for _, ev := range events {
			switch ev.Kind {
			case canonical.EventReasoningDelta:
				if ev.ChoiceIndex == 0 {
					c0ReasoningItemIdx = ev.ItemIndex
				}
			case canonical.EventTextDelta:
				if ev.ChoiceIndex == 0 {
					c0TextItemIdx = ev.ItemIndex
				}
			case canonical.EventToolCallStart:
				if ev.ChoiceIndex == 0 && ev.ToolIndex == 0 {
					c0Tool0StartName = ev.ToolName
					c0Tool0ItemIdx = ev.ItemIndex
					assert.Equal(t, "call_0_0", ev.CallID)
				}
				if ev.ChoiceIndex == 0 && ev.ToolIndex == 1 {
					c0Tool1ItemIdx = ev.ItemIndex
				}
				if ev.ChoiceIndex == 1 && ev.ToolIndex == 0 {
					c1Tool0CallID = ev.CallID
				}
			case canonical.EventToolCallArgsDelta:
				if ev.ChoiceIndex == 0 && ev.ToolIndex == 0 {
					c0Tool0Args.WriteString(ev.Text)
					c0Tool0LastName = ev.ToolName
				}
			case canonical.EventChoiceFinish:
				if ev.ChoiceIndex == 0 {
					c0Finish = ev.Finish
				} else if ev.ChoiceIndex == 1 {
					c1Finish = ev.Finish
				}
			}
		}

		assert.Equal(t, "get_", c0Tool0StartName)
		assert.Equal(t, "get_tx", c0Tool0LastName)
		assert.Equal(t, `{"id":9007199254740993}`, c0Tool0Args.String())
		assert.Equal(t, "call_1_0", c1Tool0CallID, "choice 1 tool 0 must stay isolated from choice 0 tool 0")
		assert.Len(t, map[int]bool{
			c0ReasoningItemIdx: true,
			c0TextItemIdx:      true,
			c0Tool0ItemIdx:     true,
			c0Tool1ItemIdx:     true,
		}, 4, "reasoning, text, and parallel tool items within choice 0 must receive distinct ItemIndex slots")
		assert.Equal(t, canonical.FinishToolCalls, c0Finish.Category)
		assert.Equal(t, canonical.FinishUnknown, c1Finish.Category)
		assert.Equal(t, "custom_limit", c1Finish.RawReason)
	})
}

// Regression for #75: Messages output_config.format (DS05 / RC-01) and capability-driven
// Thinking + forced tool_choice target validation (AN07 / DS02 vs OpenAI/Gemini).
func TestCanonicalMessagesOutputConfigFormatAndForcedToolThink(t *testing.T) {
	t.Run("messages_output_config_format_json_schema_default_name_and_ds05_gate", func(t *testing.T) {
		msgSchemaBody := []byte(`{
			"model":"claude-sonnet-4-6",
			"max_tokens":512,
			"output_config":{
				"format":{
					"type":"json_schema",
					"schema":{"type":"object","properties":{"id":{"const":9007199254740993}},"required":["id"]}
				}
			},
			"messages":[{"role":"user","content":"return id"}]
		}`)
		req, err := DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, msgSchemaBody)
		require.NoError(t, err)
		assert.Equal(t, canonical.ResponseFormatJSONSchema, req.ResponseFormat.Kind)
		assert.Equal(t, "response", req.ResponseFormat.Name)
		assert.Contains(t, string(req.ResponseFormat.Schema), "9007199254740993")

		// DS05: target with SupportsStrictSchema=false must reject 400 unsupported_strict_schema.
		requireRejectionError(t, ValidateCanonicalTarget(req, CanonicalTarget{
			WireFormat:           WireChatCompletions,
			Model:                "deepseek-chat",
			SupportsStrictSchema: false,
		}), "unsupported_strict_schema", "response_format")

		// Target with SupportsStrictSchema=true encodes response_format.json_schema cleanly.
		strictTarget := CanonicalTarget{
			WireFormat:           WireChatCompletions,
			Model:                "gpt-4o",
			SupportsStrictSchema: true,
		}
		require.NoError(t, ValidateCanonicalTarget(req, strictTarget))
		encoded, err := EncodeCanonicalRequest(req, strictTarget)
		require.NoError(t, err)
		assert.Equal(t, "json_schema", gjson.GetBytes(encoded, "response_format.type").String())
		assert.Equal(t, "response", gjson.GetBytes(encoded, "response_format.json_schema.name").String())
		assert.Equal(t, "9007199254740993", gjson.GetBytes(encoded, "response_format.json_schema.schema.properties.id.const").Raw)

		// Invalid output_config.format type rejected with field="output_config.format".
		_, err = DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, []byte(`{
			"model":"claude-sonnet-4-6",
			"max_tokens":128,
			"output_config":{"format":{"type":"xml"}},
			"messages":[{"role":"user","content":"hi"}]
		}`))
		requireRejectionError(t, err, "unsupported_response_format", "output_config.format")
	})

	t.Run("forced_tool_choice_with_reasoning_effort_respects_target_capability", func(t *testing.T) {
		respReq, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIResponses, []byte(`{
			"model":"o3",
			"reasoning":{"effort":"high"},
			"tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}],
			"tool_choice":"required",
			"input":"run lookup"
		}`))
		require.NoError(t, err)
		assert.Equal(t, canonical.ThinkingEnabled, respReq.Thinking.Mode)

		// OpenAI Chat / Gemini target with ForbidsForcedToolThink=false allows reasoning.effort + forced tool_choice.
		require.NoError(t, ValidateCanonicalTarget(respReq, CanonicalTarget{
			WireFormat:             WireChatCompletions,
			Model:                  "o3",
			SupportsThinking:       true,
			ForbidsForcedToolThink: false,
		}))
		require.NoError(t, ValidateCanonicalTarget(respReq, CanonicalTarget{
			WireFormat:             WireGeminiGenerateContent,
			Model:                  "gemini-2.5-pro",
			SupportsThinking:       true,
			ForbidsForcedToolThink: false,
		}))

		// ForbidsForcedToolThink=true or DashScope still rejects 400 conflicting_thinking_tool_choice.
		requireRejectionError(t, ValidateCanonicalTarget(respReq, CanonicalTarget{
			WireFormat:             WireChatCompletions,
			Model:                  "qwen-plus",
			SupportsThinking:       true,
			ForbidsForcedToolThink: true,
		}), "conflicting_thinking_tool_choice", "tool_choice")
		requireRejectionError(t, ValidateCanonicalTarget(respReq, CanonicalTarget{
			WireFormat:       WireDashScopeText,
			Model:            "qwen-plus",
			SupportsThinking: true,
		}), "conflicting_thinking_tool_choice", "tool_choice")

		// Anthropic manual thinking.type:"enabled" + forced tool_choice remains rejected even if ForbidsForcedToolThink=false (AN07).
		anthManual, err := DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, []byte(`{
			"model":"claude-sonnet-4-6",
			"max_tokens":2048,
			"thinking":{"type":"enabled","budget_tokens":1024},
			"tools":[{"name":"lookup","input_schema":{"type":"object"}}],
			"tool_choice":{"type":"any"},
			"messages":[{"role":"user","content":"run lookup"}]
		}`))
		require.NoError(t, err)
		requireRejectionError(t, ValidateCanonicalTarget(anthManual, CanonicalTarget{
			WireFormat:             WireChatCompletions,
			Model:                  "gpt-4o",
			SupportsThinking:       true,
			ForbidsForcedToolThink: false,
		}), "conflicting_thinking_tool_choice", "tool_choice")
	})
}
