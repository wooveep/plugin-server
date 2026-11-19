package provider

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/canonical"
	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type parsedSSEFrame struct {
	Event string
	Data  string
}

func parseSSEFrames(t *testing.T, raw []byte) []parsedSSEFrame {
	t.Helper()
	normalized := strings.ReplaceAll(string(raw), "\r\n", "\n")
	blocks := strings.Split(normalized, "\n\n")
	frames := make([]parsedSSEFrame, 0, len(blocks))
	for _, block := range blocks {
		if strings.TrimSpace(block) == "" {
			continue
		}
		var f parsedSSEFrame
		var dataLines []string
		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(line, "event: ") {
				f.Event = strings.TrimPrefix(line, "event: ")
			} else if strings.HasPrefix(line, "data: ") {
				dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
			}
		}
		f.Data = strings.Join(dataLines, "\n")
		frames = append(frames, f)
	}
	return frames
}

func TestCanonicalClientWritersASTPurity(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "canonical_client_writers.go", nil, parser.ParseComments)
	require.NoError(t, err)

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			assert.NotEqual(t, "init", d.Name.Name, "canonical_client_writers.go must not define init()")
		case *ast.GenDecl:
			if d.Tok == token.VAR {
				for _, spec := range d.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if ok {
						assert.Empty(t, vs.Values, "canonical_client_writers.go must not have runtime package-level var initializers")
					}
				}
			}
		}
	}

	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.InterfaceType:
			t.Errorf("forbidden interface{} type in canonical_client_writers.go at %s", fset.Position(x.Pos()))
		case *ast.Ident:
			if x.Name == "any" {
				t.Errorf("forbidden any identifier in canonical_client_writers.go at %s", fset.Position(x.Pos()))
			}
			if x.Name == "HttpContext" {
				t.Errorf("forbidden HttpContext coupling in canonical_client_writers.go at %s", fset.Position(x.Pos()))
			}
		case *ast.CallExpr:
			if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "panic" {
				t.Errorf("forbidden panic call in canonical_client_writers.go at %s", fset.Position(x.Pos()))
			}
		}
		return true
	})
}

func TestEncodeCanonicalClientResponseChat(t *testing.T) {
	t.Run("c02_tool_calls_without_text_keeps_content_null_and_preserves_big_int", func(t *testing.T) {
		policy := clientresponse.Policy{
			PublicModel: "sale-model",
			Protocol:    clientresponse.Chat,
			Profile:     clientresponse.DeepSeek,
			RequestID:   "req_chat_1",
			CreatedAt:   1700000001,
		}
		supResp := canonical.Response{
			ID:        "up_chat_1",
			CreatedAt: 1700000002,
			Model:     "vendor-model",
			Choices: []canonical.Choice{{
				Index: 0,
				Items: []canonical.Item{
					{
						Kind: canonical.ItemReasoning,
						Reasoning: &canonical.ReasoningItem{
							Text: []string{"step 1", " step 2"},
						},
					},
					{
						Kind: canonical.ItemFunctionCall,
						Call: &canonical.FunctionCall{
							CallID: "vendor_call_0",
							Name:   "fetch_order",
							Args: canonical.Arguments{
								Kind:   canonical.ArgsObject,
								Object: json.RawMessage(`{ "order_id" : 9007199254740993 }`),
							},
						},
					},
				},
				Finish:   canonical.FinishReason{Category: canonical.FinishToolCalls, RawReason: "tool_calls"},
				Logprobs: json.RawMessage("null"),
			}},
		}
		proj, err := clientresponse.ProjectCanonical(policy, supResp, clientresponse.Usage{})
		require.NoError(t, err)

		encoded, err := EncodeCanonicalClientResponse(proj)
		require.NoError(t, err)

		assert.Equal(t, "chat.completion", gjson.GetBytes(encoded, "object").String())
		assert.Equal(t, "sale-model", gjson.GetBytes(encoded, "model").String())
		assert.True(t, strings.HasPrefix(gjson.GetBytes(encoded, "id").String(), "chatcmpl_pf_"))
		assert.Equal(t, int64(1700000002), gjson.GetBytes(encoded, "created").Int())
		assert.False(t, gjson.GetBytes(encoded, "usage").Exists(), "nil WireUsage must be omitted in Chat")
		assert.False(t, gjson.GetBytes(encoded, "service_tier").Exists())

		// C02: content: null preserved when tool_calls present without text; empty optional fields omitted.
		assert.True(t, gjson.GetBytes(encoded, "choices.0.message.content").Exists())
		assert.Equal(t, gjson.Null, gjson.GetBytes(encoded, "choices.0.message.content").Type)
		assert.False(t, gjson.GetBytes(encoded, "choices.0.logprobs").Exists())
		assert.False(t, gjson.GetBytes(encoded, "choices.0.message.refusal").Exists())
		assert.False(t, gjson.GetBytes(encoded, "choices.0.message.annotations").Exists())

		assert.Equal(t, "step 1 step 2", gjson.GetBytes(encoded, "choices.0.message.reasoning_content").String())
		assert.Equal(t, "tool_calls", gjson.GetBytes(encoded, "choices.0.finish_reason").String())
		assert.True(t, strings.HasPrefix(gjson.GetBytes(encoded, "choices.0.message.tool_calls.0.id").String(), "call_pf_"))
		assert.Equal(t, `{"order_id":9007199254740993}`, gjson.GetBytes(encoded, "choices.0.message.tool_calls.0.function.arguments").String())
	})

	t.Run("unknown_finish_reason_and_annotations_and_service_tier", func(t *testing.T) {
		usage := clientresponse.ReadSupplierUsage([]byte(`{"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`), "openai_chat", false)
		policy := clientresponse.Policy{
			PublicModel:       "sale-model",
			Protocol:          clientresponse.Chat,
			Profile:           clientresponse.OpenAI,
			RequestID:         "req_chat_2",
			ExposeServiceTier: true,
		}
		supResp := canonical.Response{
			ID:          "up_chat_2",
			CreatedAt:   1700000010,
			ServiceTier: "priority",
			Choices: []canonical.Choice{{
				Index: 0,
				Items: []canonical.Item{{
					Kind: canonical.ItemMessage,
					Role: canonical.RoleAssistant,
					Parts: []canonical.Part{{
						Kind: canonical.PartText,
						Text: "cited answer",
						Annotations: []json.RawMessage{
							json.RawMessage(`{"type":"url_citation","url":"https://example.com","title":"Ex","vendor_debug":"drop"}`),
						},
					}},
				}},
				Finish:   canonical.FinishReason{Category: canonical.FinishUnknown, RawReason: "vendor_custom_stop"},
				Logprobs: json.RawMessage(`{"content":[{"token":"cited","logprob":-0.1}]}`),
			}},
		}
		proj, err := clientresponse.ProjectCanonical(policy, supResp, usage)
		require.NoError(t, err)

		encoded, err := EncodeCanonicalClientResponse(proj)
		require.NoError(t, err)

		assert.Equal(t, "priority", gjson.GetBytes(encoded, "service_tier").String())
		assert.Equal(t, "vendor_custom_stop", gjson.GetBytes(encoded, "choices.0.finish_reason").String())
		assert.Equal(t, "cited answer", gjson.GetBytes(encoded, "choices.0.message.content").String())
		assert.Equal(t, "https://example.com", gjson.GetBytes(encoded, "choices.0.message.annotations.0.url").String())
		assert.False(t, gjson.GetBytes(encoded, "choices.0.message.annotations.0.vendor_debug").Exists())
		assert.True(t, gjson.GetBytes(encoded, "choices.0.logprobs").Exists())
		assert.Equal(t, int64(15), gjson.GetBytes(encoded, "usage.total_tokens").Int())
	})
}

func TestEncodeCanonicalClientResponseResponses(t *testing.T) {
	t.Run("r01_r03_r05_r06_echo_fields_null_usage_and_big_int", func(t *testing.T) {
		policy := clientresponse.Policy{
			PublicModel:       "sale-model",
			Protocol:          clientresponse.Responses,
			Profile:           clientresponse.DeepSeek,
			RequestID:         "req_resp_1",
			CreatedAt:         1700000100,
			ExposeServiceTier: false,
			RequestFields: map[string]json.RawMessage{
				"instructions": json.RawMessage(`"Follow strict rules"`),
				"metadata":     json.RawMessage(`{"trace_int":"9007199254740993"}`),
				"error":        json.RawMessage(`null`),
				"service_tier": json.RawMessage(`"auto"`),
			},
		}
		supResp := canonical.Response{
			ID:          "up_resp_1",
			ServiceTier: "scale",
			Choices: []canonical.Choice{{
				Index: 0,
				Items: []canonical.Item{
					{
						Kind: canonical.ItemReasoning,
						Reasoning: &canonical.ReasoningItem{
							Text:      []string{"full chain of thought"},
							Summary:   canonical.Some([]string{"brief summary"}),
							Encrypted: json.RawMessage(`"gAAAA_encrypted"`),
							Origin:    canonical.OpaqueOrigin{Protocol: canonical.ProtocolOpenAIResponses, Supplier: "openai"},
						},
					},
					{
						Kind: canonical.ItemMessage,
						Role: canonical.RoleAssistant,
						Parts: []canonical.Part{{
							Kind: canonical.PartText,
							Text: "final answer",
						}},
					},
					{
						Kind: canonical.ItemFunctionCall,
						Call: &canonical.FunctionCall{
							CallID: "orig_call_1",
							Name:   "transfer",
							Args: canonical.Arguments{
								Kind: canonical.ArgsText,
								Text: `{"tx":9007199254740993}`,
							},
						},
					},
				},
				Finish: canonical.FinishReason{Category: canonical.FinishToolCalls, RawReason: "tool_calls"},
			}},
		}

		proj, err := clientresponse.ProjectCanonical(policy, supResp, clientresponse.Usage{})
		require.NoError(t, err)

		encoded, err := EncodeCanonicalClientResponse(proj)
		require.NoError(t, err)

		assert.Equal(t, "response", gjson.GetBytes(encoded, "object").String())
		assert.Equal(t, "completed", gjson.GetBytes(encoded, "status").String())
		assert.Equal(t, "sale-model", gjson.GetBytes(encoded, "model").String())
		assert.True(t, strings.HasPrefix(gjson.GetBytes(encoded, "id").String(), "resp_pf_"))
		assert.Equal(t, "Follow strict rules", gjson.GetBytes(encoded, "instructions").String())
		assert.Equal(t, "9007199254740993", gjson.GetBytes(encoded, "metadata.trace_int").String())
		// R02: source error:null stays null; unapproved service_tier does not leak.
		assert.True(t, gjson.GetBytes(encoded, "error").Exists())
		assert.Equal(t, gjson.Null, gjson.GetBytes(encoded, "error").Type)
		assert.False(t, gjson.GetBytes(encoded, "service_tier").Exists())
		// R03: nil WireUsage serializes as explicit "usage": null.
		assert.True(t, gjson.GetBytes(encoded, "usage").Exists())
		assert.Equal(t, gjson.Null, gjson.GetBytes(encoded, "usage").Type)

		assert.Equal(t, "reasoning", gjson.GetBytes(encoded, "output.0.type").String())
		assert.Equal(t, "brief summary", gjson.GetBytes(encoded, "output.0.summary.0.text").String())
		assert.Equal(t, "full chain of thought", gjson.GetBytes(encoded, "output.0.content.0.text").String())
		assert.Equal(t, "gAAAA_encrypted", gjson.GetBytes(encoded, "output.0.encrypted_content").String())

		assert.Equal(t, "message", gjson.GetBytes(encoded, "output.1.type").String())
		assert.True(t, strings.HasPrefix(gjson.GetBytes(encoded, "output.1.id").String(), "item_pf_"))
		assert.Equal(t, "final answer", gjson.GetBytes(encoded, "output.1.content.0.text").String())
		assert.True(t, gjson.GetBytes(encoded, "output.1.content.0.annotations").IsArray())

		assert.Equal(t, "function_call", gjson.GetBytes(encoded, "output.2.type").String())
		fcItemID := gjson.GetBytes(encoded, "output.2.id").String()
		fcCallID := gjson.GetBytes(encoded, "output.2.call_id").String()
		assert.True(t, strings.HasPrefix(fcItemID, "item_pf_"))
		assert.True(t, strings.HasPrefix(fcCallID, "call_pf_"))
		assert.NotEqual(t, fcItemID, fcCallID, "R06: item_id and call_id must remain distinct")
		assert.Equal(t, `{"tx":9007199254740993}`, gjson.GetBytes(encoded, "output.2.arguments").String())
	})

	t.Run("rc04_refusal_and_incomplete_messages_carry_non_empty_public_id", func(t *testing.T) {
		policy := clientresponse.Policy{
			PublicModel: "sale-model",
			Protocol:    clientresponse.Responses,
			Profile:     clientresponse.OpenAI,
			RequestID:   "req_rc04",
			CreatedAt:   1700000200,
		}

		// 1. Refusal message
		refProj, err := clientresponse.ProjectCanonical(policy, canonical.Response{
			Choices: []canonical.Choice{{
				Index: 0,
				Items: []canonical.Item{{
					Kind:  canonical.ItemMessage,
					Role:  canonical.RoleAssistant,
					Parts: []canonical.Part{{Kind: canonical.PartRefusal, Refusal: "I cannot assist with that."}},
				}},
				Finish: canonical.FinishReason{Category: canonical.FinishRefusal, RawReason: "refusal"},
			}},
		}, clientresponse.Usage{})
		require.NoError(t, err)
		refBytes, err := EncodeCanonicalClientResponse(refProj)
		require.NoError(t, err)
		assert.NotEmpty(t, gjson.GetBytes(refBytes, "output.0.id").String())
		assert.Equal(t, "refusal", gjson.GetBytes(refBytes, "output.0.content.0.type").String())
		assert.Equal(t, "I cannot assist with that.", gjson.GetBytes(refBytes, "output.0.content.0.refusal").String())

		// 2. Incomplete message (including empty text cut off by max_output_tokens)
		incProj, err := clientresponse.ProjectCanonical(policy, canonical.Response{
			Choices: []canonical.Choice{{
				Index: 0,
				Items: []canonical.Item{{
					Kind:  canonical.ItemMessage,
					Role:  canonical.RoleAssistant,
					Parts: []canonical.Part{{Kind: canonical.PartText, Text: ""}},
				}},
				Finish: canonical.FinishReason{Category: canonical.FinishLength, RawReason: "length"},
			}},
		}, clientresponse.Usage{})
		require.NoError(t, err)
		incBytes, err := EncodeCanonicalClientResponse(incProj)
		require.NoError(t, err)
		assert.Equal(t, "incomplete", gjson.GetBytes(incBytes, "status").String())
		assert.Equal(t, "max_output_tokens", gjson.GetBytes(incBytes, "incomplete_details.reason").String())
		assert.True(t, strings.HasPrefix(gjson.GetBytes(incBytes, "output.0.id").String(), "item_pf_"), "RC-04: incomplete message must carry non-empty public id")
		assert.Equal(t, "incomplete", gjson.GetBytes(incBytes, "output.0.status").String())
	})
}

func TestEncodeCanonicalClientResponseMessages(t *testing.T) {
	policy := clientresponse.Policy{
		PublicModel: "sale-model",
		Protocol:    clientresponse.Messages,
		Profile:     clientresponse.DeepSeek,
		RequestID:   "req_msg_1",
	}
	usage := clientresponse.ReadSupplierUsage([]byte(`{
		"usage":{"input_tokens":15,"cache_read_input_tokens":3,"cache_creation_input_tokens":2,"output_tokens":8}
	}`), "anthropic_messages", false)

	supResp := canonical.Response{
		ID: "msg_up_1",
		Choices: []canonical.Choice{{
			Index: 0,
			Items: []canonical.Item{
				// 0: Signed thinking (including empty text + real signature per T12)
				{
					Kind: canonical.ItemReasoning,
					Reasoning: &canonical.ReasoningItem{
						Text:      []string{""},
						Signature: "sig_real_123",
						Origin:    canonical.OpaqueOrigin{Protocol: canonical.ProtocolAnthropicMessages, Supplier: "anthropic"},
					},
				},
				// 1: Cross-supplier thinking without signature + redacted_thinking (M02: do not fabricate signature)
				{
					Kind: canonical.ItemReasoning,
					Reasoning: &canonical.ReasoningItem{
						Text:     []string{"unsigned reasoning"},
						Redacted: "redacted_blob_xyz",
					},
				},
				// 2: Visible text
				{
					Kind:  canonical.ItemMessage,
					Role:  canonical.RoleAssistant,
					Parts: []canonical.Part{{Kind: canonical.PartText, Text: "Calling tool now."}},
				},
				// 3: Tool use from ArgsText with >53-bit integer
				{
					Kind: canonical.ItemFunctionCall,
					Call: &canonical.FunctionCall{
						CallID: "call_text_1",
						Name:   "ledger_lookup",
						Args:   canonical.Arguments{Kind: canonical.ArgsText, Text: `{"account_id": 9007199254740993}`},
					},
				},
				// 4: Tool use from ArgsObject with >53-bit integer
				{
					Kind: canonical.ItemFunctionCall,
					Call: &canonical.FunctionCall{
						CallID: "call_obj_2",
						Name:   "ledger_commit",
						Args:   canonical.Arguments{Kind: canonical.ArgsObject, Object: json.RawMessage(`{"commit_id":9007199254740993}`)},
					},
				},
			},
			Finish: canonical.FinishReason{
				Category:     canonical.FinishStop,
				RawReason:    "stop_sequence",
				StopSequence: canonical.Some("\n\nHuman:"),
			},
		}},
	}

	proj, err := clientresponse.ProjectCanonical(policy, supResp, usage)
	require.NoError(t, err)

	encoded, err := EncodeCanonicalClientResponse(proj)
	require.NoError(t, err)

	assert.Equal(t, "message", gjson.GetBytes(encoded, "type").String())
	assert.Equal(t, "assistant", gjson.GetBytes(encoded, "role").String())
	assert.Equal(t, "sale-model", gjson.GetBytes(encoded, "model").String())
	assert.True(t, strings.HasPrefix(gjson.GetBytes(encoded, "id").String(), "msg_pf_"))
	assert.Equal(t, "stop_sequence", gjson.GetBytes(encoded, "stop_reason").String())
	assert.Equal(t, "\n\nHuman:", gjson.GetBytes(encoded, "stop_sequence").String())

	// Block 0: empty thinking with real signature preserved
	assert.Equal(t, "thinking", gjson.GetBytes(encoded, "content.0.type").String())
	assert.Equal(t, "", gjson.GetBytes(encoded, "content.0.thinking").String())
	assert.Equal(t, "sig_real_123", gjson.GetBytes(encoded, "content.0.signature").String())

	// Block 1: unsigned thinking omits signature (M02)
	assert.Equal(t, "thinking", gjson.GetBytes(encoded, "content.1.type").String())
	assert.Equal(t, "unsigned reasoning", gjson.GetBytes(encoded, "content.1.thinking").String())
	assert.False(t, gjson.GetBytes(encoded, "content.1.signature").Exists(), "M02: missing signature must not be fabricated")

	// Block 2: redacted_thinking
	assert.Equal(t, "redacted_thinking", gjson.GetBytes(encoded, "content.2.type").String())
	assert.Equal(t, "redacted_blob_xyz", gjson.GetBytes(encoded, "content.2.data").String())

	// Block 3: text
	assert.Equal(t, "text", gjson.GetBytes(encoded, "content.3.type").String())
	assert.Equal(t, "Calling tool now.", gjson.GetBytes(encoded, "content.3.text").String())

	// Block 4 & 5: tool_use with exact 9007199254740993 JSON object input (zero float64 truncation)
	assert.Equal(t, "tool_use", gjson.GetBytes(encoded, "content.4.type").String())
	assert.Equal(t, "9007199254740993", gjson.GetBytes(encoded, "content.4.input.account_id").Raw)
	assert.Equal(t, "tool_use", gjson.GetBytes(encoded, "content.5.type").String())
	assert.Equal(t, "9007199254740993", gjson.GetBytes(encoded, "content.5.input.commit_id").Raw)

	// M03 usage
	assert.Equal(t, int64(15), gjson.GetBytes(encoded, "usage.input_tokens").Int())
	assert.Equal(t, int64(3), gjson.GetBytes(encoded, "usage.cache_read_input_tokens").Int())
	assert.Equal(t, int64(2), gjson.GetBytes(encoded, "usage.cache_creation_input_tokens").Int())
	assert.Equal(t, int64(8), gjson.GetBytes(encoded, "usage.output_tokens").Int())
}

func TestCanonicalClientWritersEightProfilesWireUsage(t *testing.T) {
	// 1-3: openai_chat x (deepseek, qwen, openai) with C04/C05 Qwen usage fixture
	qwenChatUsage := clientresponse.ReadSupplierUsage([]byte(`{
		"usage":{
			"prompt_tokens":100,
			"completion_tokens":5,
			"total_tokens":105,
			"prompt_tokens_details":{
				"cached_tokens":20,
				"cache_creation_input_tokens":30,
				"cache_creation":{"ephemeral_5m_input_tokens":30},
				"cache_type":"ephemeral"
			},
			"completion_tokens_details":{"text_tokens":3,"reasoning_tokens":2}
		}
	}`), "openai_chat", false)

	baseChatResp := canonical.Response{
		ID: "up_c",
		Choices: []canonical.Choice{{
			Index:  0,
			Items:  []canonical.Item{{Kind: canonical.ItemMessage, Role: canonical.RoleAssistant, Parts: []canonical.Part{{Kind: canonical.PartText, Text: "ok"}}}},
			Finish: canonical.FinishReason{Category: canonical.FinishStop, RawReason: "stop"},
		}},
	}

	for _, prof := range []clientresponse.Profile{clientresponse.DeepSeek, clientresponse.Qwen, clientresponse.OpenAI} {
		proj, err := clientresponse.ProjectCanonical(clientresponse.Policy{
			PublicModel: "sale-model",
			Protocol:    clientresponse.Chat,
			Profile:     prof,
			RequestID:   "req_8p_chat_" + string(prof),
		}, baseChatResp, qwenChatUsage)
		require.NoError(t, err)
		out, err := EncodeCanonicalClientResponse(proj)
		require.NoError(t, err)
		assert.Equal(t, int64(100), gjson.GetBytes(out, "usage.prompt_tokens").Int())
		assert.Equal(t, int64(20), gjson.GetBytes(out, "usage.prompt_tokens_details.cached_tokens").Int())
		if prof == clientresponse.Qwen {
			assert.Equal(t, int64(30), gjson.GetBytes(out, "usage.prompt_tokens_details.cache_creation_input_tokens").Int())
			assert.Equal(t, int64(30), gjson.GetBytes(out, "usage.prompt_tokens_details.cache_creation.ephemeral_5m_input_tokens").Int())
			assert.Equal(t, "ephemeral", gjson.GetBytes(out, "usage.prompt_tokens_details.cache_type").String())
		} else {
			assert.False(t, gjson.GetBytes(out, "usage.prompt_tokens_details.cache_creation_input_tokens").Exists())
			assert.False(t, gjson.GetBytes(out, "usage.prompt_tokens_details.cache_creation").Exists())
		}
	}

	// 4-6: openai_responses x (deepseek, qwen, openai) with R04 x_details[] fixture
	qwenRespUsage := clientresponse.ReadSupplierUsage([]byte(`{
		"usage":{
			"input_tokens":100,
			"output_tokens":5,
			"total_tokens":105,
			"x_details":[{
				"input_tokens":100,
				"output_tokens":5,
				"total_tokens":105,
				"x_billing_type":"response_api",
				"input_tokens_details":{
					"text_tokens":100,
					"cached_tokens":20,
					"cache_creation_input_tokens":30,
					"cache_creation":{"ephemeral_5m_input_tokens":30},
					"cache_type":"ephemeral"
				},
				"output_tokens_details":{"text_tokens":3,"reasoning_tokens":2}
			}]
		}
	}`), "openai_responses", false)

	for _, prof := range []clientresponse.Profile{clientresponse.DeepSeek, clientresponse.Qwen, clientresponse.OpenAI} {
		proj, err := clientresponse.ProjectCanonical(clientresponse.Policy{
			PublicModel: "sale-model",
			Protocol:    clientresponse.Responses,
			Profile:     prof,
			RequestID:   "req_8p_resp_" + string(prof),
		}, baseChatResp, qwenRespUsage)
		require.NoError(t, err)
		out, err := EncodeCanonicalClientResponse(proj)
		require.NoError(t, err)
		assert.Equal(t, int64(105), gjson.GetBytes(out, "usage.total_tokens").Int())
		if prof == clientresponse.Qwen {
			assert.True(t, gjson.GetBytes(out, "usage.x_details").IsArray())
			assert.Equal(t, int64(30), gjson.GetBytes(out, "usage.x_details.0.input_tokens_details.cache_creation.ephemeral_5m_input_tokens").Int())
			assert.False(t, gjson.GetBytes(out, "usage.x_details.0.x_billing_type").Exists())
		} else {
			assert.False(t, gjson.GetBytes(out, "usage.x_details").Exists())
		}
	}

	// 7-8: anthropic_messages x (deepseek, qwen) streaming M04 message_start cropping vs message_delta full usage
	msgUsage := clientresponse.ReadSupplierUsage([]byte(`{
		"usage":{"input_tokens":15,"cache_read_input_tokens":3,"cache_creation_input_tokens":2,"output_tokens":8}
	}`), "anthropic_messages", false)

	for _, prof := range []clientresponse.Profile{clientresponse.DeepSeek, clientresponse.Qwen} {
		sp := clientresponse.NewCanonicalStreamProjection(clientresponse.Policy{
			PublicModel: "sale-model",
			Protocol:    clientresponse.Messages,
			Profile:     prof,
			RequestID:   "req_8p_msg_" + string(prof),
		})
		writer, err := NewCanonicalStreamWriter(canonical.ProtocolAnthropicMessages)
		require.NoError(t, err)

		var streamBuf bytes.Buffer
		evs, err := sp.Project(canonical.Event{Kind: canonical.EventTextDelta, Text: "hello"}, msgUsage)
		require.NoError(t, err)
		for _, ev := range evs {
			b, err := writer.Write(ev)
			require.NoError(t, err)
			streamBuf.Write(b)
		}
		finEvs, err := sp.Finish(canonical.FinishReason{Category: canonical.FinishStop, RawReason: "end_turn"}, msgUsage)
		require.NoError(t, err)
		for _, ev := range finEvs {
			b, err := writer.Write(ev)
			require.NoError(t, err)
			streamBuf.Write(b)
		}

		frames := parseSSEFrames(t, streamBuf.Bytes())
		require.NotEmpty(t, frames)
		assert.Equal(t, "message_start", frames[0].Event)
		assert.Equal(t, int64(15), gjson.Get(frames[0].Data, "message.usage.input_tokens").Int())
		assert.Equal(t, int64(0), gjson.Get(frames[0].Data, "message.usage.output_tokens").Int())
		if prof == clientresponse.Qwen {
			assert.False(t, gjson.Get(frames[0].Data, "message.usage.cache_read_input_tokens").Exists(), "M04: Qwen message_start must omit cache_read_input_tokens")
			assert.False(t, gjson.Get(frames[0].Data, "message.usage.cache_creation_input_tokens").Exists(), "M04: Qwen message_start must omit cache_creation_input_tokens")
		} else {
			assert.Equal(t, int64(3), gjson.Get(frames[0].Data, "message.usage.cache_read_input_tokens").Int())
			assert.Equal(t, int64(2), gjson.Get(frames[0].Data, "message.usage.cache_creation_input_tokens").Int())
		}

		var deltaFrame parsedSSEFrame
		for _, f := range frames {
			if f.Event == "message_delta" {
				deltaFrame = f
			}
		}
		assert.Equal(t, "end_turn", gjson.Get(deltaFrame.Data, "delta.stop_reason").String())
		assert.Equal(t, int64(15), gjson.Get(deltaFrame.Data, "usage.input_tokens").Int())
		assert.Equal(t, int64(8), gjson.Get(deltaFrame.Data, "usage.output_tokens").Int())
		assert.Equal(t, int64(3), gjson.Get(deltaFrame.Data, "usage.cache_read_input_tokens").Int())
		assert.Equal(t, int64(2), gjson.Get(deltaFrame.Data, "usage.cache_creation_input_tokens").Int())
	}
}

func TestCanonicalStreamWriterChat(t *testing.T) {
	t.Run("include_usage_null_deltas_single_terminal_usage_and_done", func(t *testing.T) {
		sp := clientresponse.NewCanonicalStreamProjection(clientresponse.Policy{
			PublicModel:  "sale-model",
			Protocol:     clientresponse.Chat,
			Profile:      clientresponse.DeepSeek,
			RequestID:    "req_cs_1",
			CreatedAt:    1700000500,
			IncludeUsage: true,
		})
		writer, err := NewCanonicalStreamWriter(canonical.ProtocolOpenAIChat)
		require.NoError(t, err)

		var out bytes.Buffer
		feed := []canonical.Event{
			{Kind: canonical.EventReasoningDelta, ChoiceIndex: 0, Text: "think "},
			{Kind: canonical.EventToolCallStart, ChoiceIndex: 0, ToolIndex: 0, CallID: "c0", ToolName: "calc"},
			{Kind: canonical.EventToolCallArgsDelta, ChoiceIndex: 0, ToolIndex: 0, Text: `{"n":9007199254740993}`},
			{Kind: canonical.EventTextDelta, ChoiceIndex: 1, Text: "alt choice"},
			{Kind: canonical.EventChoiceFinish, ChoiceIndex: 0, Finish: canonical.FinishReason{Category: canonical.FinishToolCalls}},
			{Kind: canonical.EventChoiceFinish, ChoiceIndex: 1, Finish: canonical.FinishReason{Category: canonical.FinishStop}},
		}
		for _, raw := range feed {
			evs, err := sp.Project(raw, clientresponse.Usage{})
			require.NoError(t, err)
			for _, ev := range evs {
				chunk, err := writer.Write(ev)
				require.NoError(t, err)
				out.Write(chunk)
			}
		}

		finalUsage := clientresponse.ReadSupplierUsage([]byte(`{"usage":{"prompt_tokens":20,"completion_tokens":10,"total_tokens":30}}`), "openai_chat", false)
		finEvs, err := sp.Finish(canonical.FinishReason{Category: canonical.FinishToolCalls}, finalUsage)
		require.NoError(t, err)
		for _, ev := range finEvs {
			chunk, err := writer.Finish(ev)
			require.NoError(t, err)
			out.Write(chunk)
		}

		frames := parseSSEFrames(t, out.Bytes())
		require.GreaterOrEqual(t, len(frames), 7)
		assert.Equal(t, "[DONE]", frames[len(frames)-1].Data)

		// Second-to-last frame is the single terminal usage-only chunk (choices: []).
		usageFrame := frames[len(frames)-2]
		assert.Equal(t, 0, len(gjson.Get(usageFrame.Data, "choices").Array()))
		assert.Equal(t, int64(30), gjson.Get(usageFrame.Data, "usage.total_tokens").Int())

		// Earlier chunks carry "usage": null when IncludeUsage=true.
		assert.True(t, gjson.Get(frames[0].Data, "usage").Exists())
		assert.Equal(t, gjson.Null, gjson.Get(frames[0].Data, "usage").Type)
		assert.Contains(t, out.String(), `9007199254740993`)

		// Fail after completed stream must return an error.
		_, err = writer.Fail(clientresponse.CanonicalEvent{Protocol: canonical.ProtocolOpenAIChat})
		require.Error(t, err)
	})

	t.Run("mid_stream_fail_emits_error_without_done", func(t *testing.T) {
		sp := clientresponse.NewCanonicalStreamProjection(clientresponse.Policy{
			PublicModel:  "sale-model",
			Protocol:     clientresponse.Chat,
			Profile:      clientresponse.DeepSeek,
			RequestID:    "req_cs_fail",
			IncludeUsage: true,
		})
		writer, err := NewCanonicalStreamWriter(canonical.ProtocolOpenAIChat)
		require.NoError(t, err)

		var out bytes.Buffer
		evs, err := sp.Project(canonical.Event{Kind: canonical.EventTextDelta, Text: "partial "}, clientresponse.Usage{})
		require.NoError(t, err)
		for _, ev := range evs {
			b, err := writer.Write(ev)
			require.NoError(t, err)
			out.Write(b)
		}

		partialUsage := clientresponse.ReadSupplierUsage([]byte(`{"usage":{"prompt_tokens":8,"completion_tokens":2,"total_tokens":10}}`), "openai_chat", false)
		failure := clientresponse.ClassifyFailure(clientresponse.FailureInput{
			Status: 502,
			Body:   []byte(`{"error":{"message":"upstream broken"}}`),
			Client: clientresponse.Chat,
		})
		failEvs, err := sp.Fail(failure, partialUsage)
		require.NoError(t, err)
		for _, ev := range failEvs {
			b, err := writer.Fail(ev)
			require.NoError(t, err)
			out.Write(b)
		}

		frames := parseSSEFrames(t, out.Bytes())
		require.NotEmpty(t, frames)
		assert.NotEqual(t, "[DONE]", frames[len(frames)-1].Data)
		assert.True(t, gjson.Get(frames[len(frames)-1].Data, "error.code").Exists())
	})
}

func TestCanonicalStreamWriterResponsesLifecycleAndFail(t *testing.T) {
	t.Run("t18_r05_completed_lifecycle_sequence_and_snapshot_without_duplication", func(t *testing.T) {
		sp := clientresponse.NewCanonicalStreamProjection(clientresponse.Policy{
			PublicModel: "sale-model",
			Protocol:    clientresponse.Responses,
			Profile:     clientresponse.DeepSeek,
			RequestID:   "req_rs_ok",
			CreatedAt:   1700000600,
			RequestFields: map[string]json.RawMessage{
				"instructions": json.RawMessage(`"Be precise"`),
			},
		})
		writer, err := NewCanonicalStreamWriter(canonical.ProtocolOpenAIResponses)
		require.NoError(t, err)

		var out bytes.Buffer
		feed := []canonical.Event{
			{Kind: canonical.EventReasoningDelta, ItemIndex: 0, Text: "reason A "},
			{Kind: canonical.EventReasoningDelta, ItemIndex: 0, Text: "reason B"},
			{Kind: canonical.EventContentBlockStop, ItemIndex: 0},
			{Kind: canonical.EventTextDelta, ItemIndex: 1, Text: "Hello "},
			{Kind: canonical.EventTextDelta, ItemIndex: 1, Text: "world!"},
			{Kind: canonical.EventContentBlockStop, ItemIndex: 1},
			{Kind: canonical.EventToolCallStart, ItemIndex: 2, ToolIndex: 0, CallID: "c_0", ToolName: "pay"},
			{Kind: canonical.EventToolCallArgsDelta, ItemIndex: 2, ToolIndex: 0, Text: `{"amount":`},
			{Kind: canonical.EventToolCallArgsDelta, ItemIndex: 2, ToolIndex: 0, Text: `9007199254740993}`},
		}
		for _, raw := range feed {
			evs, err := sp.Project(raw, clientresponse.Usage{})
			require.NoError(t, err)
			for _, ev := range evs {
				b, err := writer.Write(ev)
				require.NoError(t, err)
				out.Write(b)
			}
		}

		finalUsage := clientresponse.ReadSupplierUsage([]byte(`{"usage":{"input_tokens":50,"output_tokens":25,"total_tokens":75}}`), "openai_responses", false)
		finEvs, err := sp.Finish(canonical.FinishReason{Category: canonical.FinishToolCalls}, finalUsage)
		require.NoError(t, err)
		for _, ev := range finEvs {
			b, err := writer.Finish(ev)
			require.NoError(t, err)
			out.Write(b)
		}

		frames := parseSSEFrames(t, out.Bytes())
		require.NotEmpty(t, frames)
		assert.NotContains(t, out.String(), "[DONE]")

		var eventTypes []string
		for idx, f := range frames {
			assert.Equal(t, int64(idx), gjson.Get(f.Data, "sequence_number").Int(), "sequence_number must be strictly 0..N-1")
			assert.Equal(t, f.Event, gjson.Get(f.Data, "type").String())
			eventTypes = append(eventTypes, f.Event)
		}
		assert.Equal(t, "response.created", eventTypes[0])
		assert.Equal(t, "response.in_progress", eventTypes[1])
		assert.Contains(t, eventTypes, "response.reasoning_text.delta")
		assert.Contains(t, eventTypes, "response.reasoning_text.done")
		assert.Contains(t, eventTypes, "response.output_text.delta")
		assert.Contains(t, eventTypes, "response.output_text.done")
		assert.Contains(t, eventTypes, "response.function_call_arguments.delta")
		assert.Contains(t, eventTypes, "response.function_call_arguments.done")
		assert.Equal(t, "response.completed", eventTypes[len(eventTypes)-1])

		terminal := frames[len(frames)-1].Data
		assert.Equal(t, "completed", gjson.Get(terminal, "response.status").String())
		assert.Equal(t, "Be precise", gjson.Get(terminal, "response.instructions").String())
		assert.Equal(t, int64(75), gjson.Get(terminal, "response.usage.total_tokens").Int())
		// Snapshot aggregates deltas without duplicating!
		assert.Equal(t, "reason A reason B", gjson.Get(terminal, "response.output.0.content.0.text").String())
		assert.True(t, gjson.Get(terminal, "response.output.0.summary").IsArray())
		assert.Equal(t, 0, len(gjson.Get(terminal, "response.output.0.summary").Array()))
		assert.Equal(t, "Hello world!", gjson.Get(terminal, "response.output.1.content.0.text").String())
		assert.Equal(t, `{"amount":9007199254740993}`, gjson.Get(terminal, "response.output.2.arguments").String())
	})

	t.Run("b04_e05_mid_stream_fail_marks_open_outputs_incomplete_and_emits_response_failed", func(t *testing.T) {
		sp := clientresponse.NewCanonicalStreamProjection(clientresponse.Policy{
			PublicModel: "sale-model",
			Protocol:    clientresponse.Responses,
			Profile:     clientresponse.OpenAI,
			RequestID:   "req_rs_fail",
			CreatedAt:   1700000601,
			RequestFields: map[string]json.RawMessage{
				"instructions": json.RawMessage(`"Keep safe"`),
			},
		})
		writer, err := NewCanonicalStreamWriter(canonical.ProtocolOpenAIResponses)
		require.NoError(t, err)

		var out bytes.Buffer
		feed := []canonical.Event{
			{Kind: canonical.EventReasoningDelta, ItemIndex: 0, Text: "partial thought"},
			{Kind: canonical.EventTextDelta, ItemIndex: 1, Text: "partial answer"},
			{Kind: canonical.EventToolCallStart, ItemIndex: 2, ToolIndex: 0, CallID: "c_partial", ToolName: "write_db"},
			{Kind: canonical.EventToolCallArgsDelta, ItemIndex: 2, ToolIndex: 0, Text: `{"id":9007199`},
		}
		for _, raw := range feed {
			evs, err := sp.Project(raw, clientresponse.Usage{})
			require.NoError(t, err)
			for _, ev := range evs {
				b, err := writer.Write(ev)
				require.NoError(t, err)
				out.Write(b)
			}
		}

		failure := clientresponse.ClassifyFailure(clientresponse.FailureInput{
			Status: 503,
			Body:   []byte(`{"error":{"type":"overloaded_error","message":"vendor overloaded"}}`),
			Client: clientresponse.Responses,
		})
		failEvs, err := sp.Fail(failure, clientresponse.Usage{})
		require.NoError(t, err)
		for _, ev := range failEvs {
			b, err := writer.Fail(ev)
			require.NoError(t, err)
			out.Write(b)
		}

		frames := parseSSEFrames(t, out.Bytes())
		require.NotEmpty(t, frames)
		for _, f := range frames {
			assert.NotEqual(t, "response.completed", f.Event)
			assert.NotEqual(t, "response.function_call_arguments.done", f.Event, "Fail must not close truncated tool args as done")
		}

		last := frames[len(frames)-1]
		assert.Equal(t, "response.failed", last.Event)
		assert.Equal(t, "failed", gjson.Get(last.Data, "response.status").String())
		assert.Equal(t, "Keep safe", gjson.Get(last.Data, "response.instructions").String())
		assert.True(t, gjson.Get(last.Data, "response.error.code").Exists())

		outputs := gjson.Get(last.Data, "response.output").Array()
		require.Len(t, outputs, 3)
		assert.Equal(t, "incomplete", outputs[0].Get("status").String())
		assert.Equal(t, "partial thought", outputs[0].Get("content.0.text").String())
		assert.Equal(t, "incomplete", outputs[1].Get("status").String())
		assert.Equal(t, "partial answer", outputs[1].Get("content.0.text").String())
		assert.Equal(t, "incomplete", outputs[2].Get("status").String())
		assert.Equal(t, `{"id":9007199`, outputs[2].Get("arguments").String())
	})

	t.Run("incomplete_terminal_emits_response_incomplete", func(t *testing.T) {
		sp := clientresponse.NewCanonicalStreamProjection(clientresponse.Policy{
			PublicModel: "sale-model",
			Protocol:    clientresponse.Responses,
			Profile:     clientresponse.DeepSeek,
			RequestID:   "req_rs_inc",
		})
		writer, err := NewCanonicalStreamWriter(canonical.ProtocolOpenAIResponses)
		require.NoError(t, err)

		var out bytes.Buffer
		evs, err := sp.Project(canonical.Event{Kind: canonical.EventRefusalDelta, ItemIndex: 0, Text: "Refused cut"}, clientresponse.Usage{})
		require.NoError(t, err)
		for _, ev := range evs {
			b, err := writer.Write(ev)
			require.NoError(t, err)
			out.Write(b)
		}
		finEvs, err := sp.Finish(canonical.FinishReason{Category: canonical.FinishLength, RawReason: "length"}, clientresponse.Usage{})
		require.NoError(t, err)
		for _, ev := range finEvs {
			b, err := writer.Finish(ev)
			require.NoError(t, err)
			out.Write(b)
		}

		frames := parseSSEFrames(t, out.Bytes())
		last := frames[len(frames)-1]
		assert.Equal(t, "response.incomplete", last.Event)
		assert.Equal(t, "incomplete", gjson.Get(last.Data, "response.status").String())
		assert.Equal(t, "max_output_tokens", gjson.Get(last.Data, "response.incomplete_details.reason").String())
		assert.Equal(t, "incomplete", gjson.Get(last.Data, "response.output.0.status").String())
	})
}

func TestCanonicalStreamWriterMessagesInterleavedToolsAndSignature(t *testing.T) {
	t.Run("an04_signature_delta_redacted_ping_and_interleaved_parallel_tools", func(t *testing.T) {
		sp := clientresponse.NewCanonicalStreamProjection(clientresponse.Policy{
			PublicModel: "sale-model",
			Protocol:    clientresponse.Messages,
			Profile:     clientresponse.DeepSeek,
			RequestID:   "req_ms_1",
		})
		writer, err := NewCanonicalStreamWriter(canonical.ProtocolAnthropicMessages)
		require.NoError(t, err)

		var out bytes.Buffer
		feed := []canonical.Event{
			{Kind: canonical.EventPing},
			{Kind: canonical.EventReasoningDelta, ItemIndex: 0, Text: "Let me calculate."},
			{Kind: canonical.EventReasoningSignatureDelta, ItemIndex: 0, ThoughtSignature: "sig_part_1"},
			{Kind: canonical.EventContentBlockStop, ItemIndex: 0},
			{Kind: canonical.EventReasoningRedacted, ItemIndex: 1, Text: "redacted_payload"},
			{Kind: canonical.EventTextDelta, ItemIndex: 2, Text: "Invoking two tools."},
			// Interleaved parallel tool deltas (like DashScope / OpenAI Chat):
			{Kind: canonical.EventToolCallStart, ItemIndex: 3, ToolIndex: 0, CallID: "call_a", ToolName: "tool_alpha"},
			{Kind: canonical.EventToolCallStart, ItemIndex: 4, ToolIndex: 1, CallID: "call_b", ToolName: "tool_beta"},
			{Kind: canonical.EventToolCallArgsDelta, ItemIndex: 3, ToolIndex: 0, Text: `{"a":`},
			{Kind: canonical.EventToolCallArgsDelta, ItemIndex: 4, ToolIndex: 1, Text: `{"b":`},
			{Kind: canonical.EventToolCallArgsDelta, ItemIndex: 3, ToolIndex: 0, Text: `9007199254740993}`},
			{Kind: canonical.EventToolCallArgsDelta, ItemIndex: 4, ToolIndex: 1, Text: `9007199254740993}`},
		}
		for _, raw := range feed {
			evs, err := sp.Project(raw, clientresponse.Usage{})
			require.NoError(t, err)
			for _, ev := range evs {
				b, err := writer.Write(ev)
				require.NoError(t, err)
				out.Write(b)
			}
		}

		finalUsage := clientresponse.ReadSupplierUsage([]byte(`{"usage":{"input_tokens":25,"output_tokens":12}}`), "anthropic_messages", false)
		finEvs, err := sp.Finish(canonical.FinishReason{Category: canonical.FinishToolCalls, RawReason: "tool_use"}, finalUsage)
		require.NoError(t, err)
		for _, ev := range finEvs {
			b, err := writer.Finish(ev)
			require.NoError(t, err)
			out.Write(b)
		}

		frames := parseSSEFrames(t, out.Bytes())
		require.NotEmpty(t, frames)
		assert.Equal(t, "ping", frames[0].Event)
		assert.Equal(t, "message_start", frames[1].Event)
		assert.Equal(t, "message_stop", frames[len(frames)-1].Event)
		assert.NotContains(t, out.String(), "[DONE]")

		// Verify strict block start -> deltas -> stop serialization with no overlapping open blocks!
		openIdx := -1
		nextExpectedBlock := 0
		blockTypes := make(map[int]string)
		toolPartials := make(map[int]string)
		sawSignatureDelta := false

		for _, f := range frames {
			switch f.Event {
			case "content_block_start":
				assert.Equal(t, -1, openIdx, "Claude requires closing the previous content_block before starting a new one")
				idx := int(gjson.Get(f.Data, "index").Int())
				assert.Equal(t, nextExpectedBlock, idx)
				nextExpectedBlock++
				openIdx = idx
				blockTypes[idx] = gjson.Get(f.Data, "content_block.type").String()
			case "content_block_delta":
				idx := int(gjson.Get(f.Data, "index").Int())
				assert.Equal(t, openIdx, idx, "content_block_delta must target the currently open block")
				dt := gjson.Get(f.Data, "delta.type").String()
				if dt == "signature_delta" {
					sawSignatureDelta = true
					assert.Equal(t, "sig_part_1", gjson.Get(f.Data, "delta.signature").String())
				}
				if dt == "input_json_delta" {
					toolPartials[idx] += gjson.Get(f.Data, "delta.partial_json").String()
				}
			case "content_block_stop":
				idx := int(gjson.Get(f.Data, "index").Int())
				assert.Equal(t, openIdx, idx, "content_block_stop must close the currently open block")
				openIdx = -1
			}
		}

		assert.Equal(t, -1, openIdx, "all content blocks must be closed before message_stop")
		assert.True(t, sawSignatureDelta, "AN04: signature_delta must be emitted")
		assert.Equal(t, "thinking", blockTypes[0])
		assert.Equal(t, "redacted_thinking", blockTypes[1])
		assert.Equal(t, "text", blockTypes[2])
		assert.Equal(t, "tool_use", blockTypes[3])
		assert.Equal(t, "tool_use", blockTypes[4])
		assert.Equal(t, `{"a":9007199254740993}`, toolPartials[3])
		assert.Equal(t, `{"b":9007199254740993}`, toolPartials[4])
	})

	t.Run("mid_stream_fail_emits_error_event_without_message_stop", func(t *testing.T) {
		sp := clientresponse.NewCanonicalStreamProjection(clientresponse.Policy{
			PublicModel: "sale-model",
			Protocol:    clientresponse.Messages,
			Profile:     clientresponse.Qwen,
			RequestID:   "req_ms_fail",
		})
		writer, err := NewCanonicalStreamWriter(canonical.ProtocolAnthropicMessages)
		require.NoError(t, err)

		var out bytes.Buffer
		evs, err := sp.Project(canonical.Event{Kind: canonical.EventTextDelta, Text: "partial"}, clientresponse.Usage{})
		require.NoError(t, err)
		for _, ev := range evs {
			b, err := writer.Write(ev)
			require.NoError(t, err)
			out.Write(b)
		}

		failure := clientresponse.ClassifyFailure(clientresponse.FailureInput{
			Status: 500,
			Body:   []byte(`{"type":"error","error":{"type":"api_error","message":"crash"}}`),
			Client: clientresponse.Messages,
		})
		failEvs, err := sp.Fail(failure, clientresponse.Usage{})
		require.NoError(t, err)
		for _, ev := range failEvs {
			b, err := writer.Fail(ev)
			require.NoError(t, err)
			out.Write(b)
		}

		frames := parseSSEFrames(t, out.Bytes())
		last := frames[len(frames)-1]
		assert.Equal(t, "error", last.Event)
		assert.Equal(t, "error", gjson.Get(last.Data, "type").String())
		assert.True(t, gjson.Get(last.Data, "error.type").Exists())
		for _, f := range frames {
			assert.NotEqual(t, "message_stop", f.Event)
		}
	})

	t.Run("out_of_order_tool_stop_preserves_active_tool_tail_and_trailing_text_drains_queued_tools", func(t *testing.T) {
		sp := clientresponse.NewCanonicalStreamProjection(clientresponse.Policy{
			PublicModel: "sale-model",
			Protocol:    clientresponse.Messages,
			Profile:     clientresponse.DeepSeek,
			RequestID:   "req_ms_ooo_stop",
		})
		writer, err := NewCanonicalStreamWriter(canonical.ProtocolAnthropicMessages)
		require.NoError(t, err)

		var out bytes.Buffer
		feed := []canonical.Event{
			// Tool 0 opens and starts streaming; Tool 1 and Tool 2 buffer in parallel.
			{Kind: canonical.EventToolCallStart, ItemIndex: 0, ToolIndex: 0, CallID: "call_0", ToolName: "first_tool"},
			{Kind: canonical.EventToolCallArgsDelta, ItemIndex: 0, ToolIndex: 0, Text: `{"id":900719925`},
			{Kind: canonical.EventToolCallStart, ItemIndex: 1, ToolIndex: 1, CallID: "call_1", ToolName: "second_tool"},
			{Kind: canonical.EventToolCallArgsDelta, ItemIndex: 1, ToolIndex: 1, Text: `{"ok":true}`},
			{Kind: canonical.EventToolCallStart, ItemIndex: 2, ToolIndex: 2, CallID: "call_2", ToolName: "third_tool"},
			{Kind: canonical.EventToolCallArgsDelta, ItemIndex: 2, ToolIndex: 2, Text: `{"step":3}`},
			// Out-of-order stop: Tool 1 finishes BEFORE Tool 0! Tool 0 must NOT be preempted/closed.
			{Kind: canonical.EventContentBlockStop, ItemIndex: 1, ToolIndex: 1, CallID: "call_1"},
			// Tool 0 emits its remaining JSON tail after Tool 1 stopped!
			{Kind: canonical.EventToolCallArgsDelta, ItemIndex: 0, ToolIndex: 0, Text: `4740993}`},
			// Tool 0 now stops -> closes Tool 0 (idx 0), starts+closes pendingStop Tool 1 (idx 1), promotes Tool 2 (idx 2).
			{Kind: canonical.EventContentBlockStop, ItemIndex: 0, ToolIndex: 0, CallID: "call_0"},
			// Trailing text block (e.g. Gemini parts: [functionCall_0..2, text]) closes Tool 2 before opening text (idx 3).
			{Kind: canonical.EventTextDelta, ItemIndex: 3, Text: "All tools scheduled."},
		}
		for _, raw := range feed {
			evs, err := sp.Project(raw, clientresponse.Usage{})
			require.NoError(t, err)
			for _, ev := range evs {
				b, err := writer.Write(ev)
				require.NoError(t, err)
				out.Write(b)
			}
		}

		// Also verify non-tool transition when a queued tool hasn't even been promoted yet:
		sp2 := clientresponse.NewCanonicalStreamProjection(clientresponse.Policy{
			PublicModel: "sale-model",
			Protocol:    clientresponse.Messages,
			Profile:     clientresponse.DeepSeek,
			RequestID:   "req_ms_gemini_parts",
		})
		writer2, err := NewCanonicalStreamWriter(canonical.ProtocolAnthropicMessages)
		require.NoError(t, err)
		var out2 bytes.Buffer
		for _, raw := range []canonical.Event{
			{Kind: canonical.EventToolCallStart, ItemIndex: 0, ToolIndex: 0, CallID: "g_0", ToolName: "fn0", Text: `{"x":1}`},
			{Kind: canonical.EventToolCallStart, ItemIndex: 1, ToolIndex: 1, CallID: "g_1", ToolName: "fn1", Text: `{"y":2}`},
			{Kind: canonical.EventTextDelta, ItemIndex: 2, Text: "trailing summary"},
		} {
			evs, err := sp2.Project(raw, clientresponse.Usage{})
			require.NoError(t, err)
			for _, ev := range evs {
				b, err := writer2.Write(ev)
				require.NoError(t, err)
				out2.Write(b)
			}
		}
		finEvs2, err := sp2.Finish(canonical.FinishReason{Category: canonical.FinishToolCalls}, clientresponse.Usage{})
		require.NoError(t, err)
		for _, ev := range finEvs2 {
			b, err := writer2.Finish(ev)
			require.NoError(t, err)
			out2.Write(b)
		}

		finEvs, err := sp.Finish(canonical.FinishReason{Category: canonical.FinishToolCalls}, clientresponse.Usage{})
		require.NoError(t, err)
		for _, ev := range finEvs {
			b, err := writer.Finish(ev)
			require.NoError(t, err)
			out.Write(b)
		}

		// Assert Stream 1: Tool 0 full big-int JSON intact, strictly sequential blocks [tool_0, tool_1, tool_2, text]
		frames := parseSSEFrames(t, out.Bytes())
		openIdx := -1
		blockTypes := make(map[int]string)
		blockNames := make(map[int]string)
		toolPartials := make(map[int]string)
		textContent := make(map[int]string)
		for _, f := range frames {
			switch f.Event {
			case "content_block_start":
				assert.Equal(t, -1, openIdx)
				idx := int(gjson.Get(f.Data, "index").Int())
				openIdx = idx
				blockTypes[idx] = gjson.Get(f.Data, "content_block.type").String()
				blockNames[idx] = gjson.Get(f.Data, "content_block.name").String()
			case "content_block_delta":
				idx := int(gjson.Get(f.Data, "index").Int())
				assert.Equal(t, openIdx, idx)
				if gjson.Get(f.Data, "delta.type").String() == "input_json_delta" {
					toolPartials[idx] += gjson.Get(f.Data, "delta.partial_json").String()
				}
				if gjson.Get(f.Data, "delta.type").String() == "text_delta" {
					textContent[idx] += gjson.Get(f.Data, "delta.text").String()
				}
			case "content_block_stop":
				idx := int(gjson.Get(f.Data, "index").Int())
				assert.Equal(t, openIdx, idx)
				openIdx = -1
			}
		}
		assert.Equal(t, -1, openIdx)
		assert.Equal(t, "first_tool", blockNames[0])
		assert.Equal(t, `{"id":9007199254740993}`, toolPartials[0], "Tool 0 tail args after out-of-order Tool 1 stop must not be lost")
		assert.Equal(t, "second_tool", blockNames[1])
		assert.Equal(t, `{"ok":true}`, toolPartials[1])
		assert.Equal(t, "third_tool", blockNames[2])
		assert.Equal(t, `{"step":3}`, toolPartials[2])
		assert.Equal(t, "text", blockTypes[3])
		assert.Equal(t, "All tools scheduled.", textContent[3])

		// Assert Stream 2: [fn0 (idx 0), fn1 (idx 1), text (idx 2)] drained before trailing text!
		frames2 := parseSSEFrames(t, out2.Bytes())
		var blockOrder2 []string
		for _, f := range frames2 {
			if f.Event == "content_block_start" {
				bt := gjson.Get(f.Data, "content_block.type").String()
				if bt == "tool_use" {
					blockOrder2 = append(blockOrder2, bt+":"+gjson.Get(f.Data, "content_block.name").String())
				} else {
					blockOrder2 = append(blockOrder2, bt)
				}
			}
		}
		assert.Equal(t, []string{"tool_use:fn0", "tool_use:fn1", "text"}, blockOrder2)
	})
}
