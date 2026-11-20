package provider

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"strings"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/canonical"
	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// Compile-time verification that all 6 supplier functions match canonical_target.go hook signatures 100%.
var (
	_ func(canonical.Request, CanonicalTarget) ([]byte, error)  = encodeQwenCanonicalRequest
	_ func(CanonicalTarget, []byte) (canonical.Response, error) = decodeQwenCanonicalResponse
	_ func(CanonicalTarget) (CanonicalEventDecoder, error)      = newQwenCanonicalEventDecoder
	_ func(canonical.Request, CanonicalTarget) ([]byte, error)  = encodeGeminiCanonicalRequest
	_ func(CanonicalTarget, []byte) (canonical.Response, error) = decodeGeminiCanonicalResponse
	_ func(CanonicalTarget) (CanonicalEventDecoder, error)      = newGeminiCanonicalEventDecoder
)

func TestCanonicalSupplierCodecsASTPurity(t *testing.T) {
	files := []string{
		"canonical_qwen.go",
		"canonical_gemini.go",
	}
	fset := token.NewFileSet()
	for _, filename := range files {
		file, err := parser.ParseFile(fset, filename, nil, parser.AllErrors)
		require.NoError(t, err, "parse %s", filename)

		for _, imp := range file.Imports {
			impPath := strings.Trim(imp.Path.Value, `"`)
			if strings.Contains(impPath, "wrapper") || strings.Contains(impPath, "proxywasm") {
				t.Fatalf("%s imports forbidden host/context package %q", filename, impPath)
			}
		}

		ast.Inspect(file, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == "init" {
				t.Fatalf("%s declares forbidden init() function", filename)
			}
			if call, ok := n.(*ast.CallExpr); ok {
				if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "panic" {
					t.Fatalf("%s contains forbidden panic() call at %v", filename, fset.Position(call.Pos()))
				}
			}
			if iface, ok := n.(*ast.InterfaceType); ok {
				if iface.Methods == nil || len(iface.Methods.List) == 0 {
					t.Fatalf("%s contains forbidden empty interface{} at %v", filename, fset.Position(iface.Pos()))
				}
			}
			if ident, ok := n.(*ast.Ident); ok {
				if ident.Name == "any" {
					t.Fatalf("%s contains forbidden 'any' identifier at %v", filename, fset.Position(ident.Pos()))
				}
				if ident.Name == "HttpContext" {
					t.Fatalf("%s references forbidden HttpContext at %v", filename, fset.Position(ident.Pos()))
				}
			}
			return true
		})
	}
}

func TestCanonicalSupplierDirectAndPartialMode(t *testing.T) {
	req, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, []byte(`{"model":"qwen-plus","messages":[{"role":"user","content":"ping"}]}`))
	require.NoError(t, err)

	qwenTarget := CanonicalTarget{WireFormat: WireDashScopeText, Supplier: "qwen", Model: "qwen-plus"}
	qwenBytes, err := encodeQwenCanonicalRequest(req, qwenTarget)
	require.NoError(t, err)
	assert.Equal(t, "qwen-plus", gjson.GetBytes(qwenBytes, "model").String())

	qwenResp, err := decodeQwenCanonicalResponse(qwenTarget, []byte(`{"request_id":"r1","output":{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"pong"}}]}}`))
	require.NoError(t, err)
	require.Len(t, qwenResp.Choices, 1)
	assert.Equal(t, canonical.FinishStop, qwenResp.Choices[0].Finish.Category)

	qwenDec, err := newQwenCanonicalEventDecoder(qwenTarget)
	require.NoError(t, err)
	require.NotNil(t, qwenDec)

	gemTarget := CanonicalTarget{WireFormat: WireGeminiGenerateContent, Supplier: "gemini", Model: "gemini-2.5-flash"}
	gemBytes, err := encodeGeminiCanonicalRequest(req, gemTarget)
	require.NoError(t, err)
	assert.Equal(t, "ping", gjson.GetBytes(gemBytes, "contents.0.parts.0.text").String())

	gemResp, err := decodeGeminiCanonicalResponse(gemTarget, []byte(`{"responseId":"g1","candidates":[{"index":0,"finishReason":"STOP","content":{"role":"model","parts":[{"text":"pong"}]}}]}`))
	require.NoError(t, err)
	require.Len(t, gemResp.Choices, 1)
	assert.Equal(t, canonical.FinishStop, gemResp.Choices[0].Finish.Category)

	gemDec, err := newGeminiCanonicalEventDecoder(gemTarget)
	require.NoError(t, err)
	require.NotNil(t, gemDec)

	// C05: Item.Partial tri-state passthrough onto DashScope native assistant messages (text & multimodal).
	partialReq := canonical.Request{
		SourceProtocol: canonical.ProtocolOpenAIChat,
		Model:          "qwen-plus",
		Items: []canonical.Item{
			{
				Kind:  canonical.ItemMessage,
				Role:  canonical.RoleUser,
				Parts: []canonical.Part{{Kind: canonical.PartText, Text: "Complete this function"}},
			},
			{
				Kind:    canonical.ItemMessage,
				Role:    canonical.RoleAssistant,
				Partial: canonical.Some(true),
				Parts:   []canonical.Part{{Kind: canonical.PartText, Text: "def add(a,b):\n    "}},
			},
		},
	}
	for _, wf := range []WireFormat{WireDashScopeText, WireDashScopeMultimodal} {
		encodedPartial, err := encodeQwenCanonicalRequest(partialReq, CanonicalTarget{
			WireFormat: wf,
			Supplier:   "qwen",
			Model:      "qwen-plus",
		})
		require.NoError(t, err)
		assert.False(t, gjson.GetBytes(encodedPartial, "input.messages.0.partial").Exists())
		assert.Equal(t, gjson.True, gjson.GetBytes(encodedPartial, "input.messages.1.partial").Type)
		assert.Equal(t, "assistant", gjson.GetBytes(encodedPartial, "input.messages.1.role").String())
		assert.Equal(t, "def add(a,b):\n    ", gjson.GetBytes(encodedPartial, "input.messages.1.content").String())
	}

	partialFalseReq := partialReq.CloneForAttempt()
	partialFalseReq.Items[1].Partial = canonical.Some(false)
	encodedFalse, err := encodeQwenCanonicalRequest(partialFalseReq, qwenTarget)
	require.NoError(t, err)
	assert.Equal(t, gjson.False, gjson.GetBytes(encodedFalse, "input.messages.1.partial").Type)
}

// T03: Output limits across max_tokens / max_completion_tokens / max_output_tokens and warmup 0 rejection.
func TestCanonicalSupplierOutputLimits(t *testing.T) {
	chatMaxComp, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, []byte(`{
		"model":"m",
		"max_completion_tokens":512,
		"messages":[{"role":"user","content":"hi"}]
	}`))
	require.NoError(t, err)

	respMaxOut, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIResponses, []byte(`{
		"model":"m",
		"max_output_tokens":768,
		"input":"hi"
	}`))
	require.NoError(t, err)

	msgMaxTok, err := DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, []byte(`{
		"model":"m",
		"max_tokens":1024,
		"messages":[{"role":"user","content":"hi"}]
	}`))
	require.NoError(t, err)

	t.Run("dashscope_maps_by_frozen_completion_tokens_capability", func(t *testing.T) {
		// Default DashScope model without max_completion_tokens capability -> parameters.max_tokens.
		out1, err := encodeQwenCanonicalRequest(chatMaxComp, CanonicalTarget{
			WireFormat:               WireDashScopeText,
			Model:                    "qwen-plus",
			SupportsCompletionTokens: false,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(512), gjson.GetBytes(out1, "parameters.max_tokens").Int())
		assert.False(t, gjson.GetBytes(out1, "parameters.max_completion_tokens").Exists())

		// Model supporting max_completion_tokens distinguishes ScopeReasoningAndVisible vs ScopeVisibleOnly.
		target := ResolveCanonicalTarget(nil, "dashscope_multimodal", providerTypeQwen, "", "qwen3.5-plus", false)
		out2, err := EncodeCanonicalRequest(respMaxOut, target)
		require.NoError(t, err)
		assert.Equal(t, int64(768), gjson.GetBytes(out2, "parameters.max_completion_tokens").Int())
		assert.False(t, gjson.GetBytes(out2, "parameters.max_tokens").Exists())

		chatVis, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, []byte(`{
			"model":"m",
			"max_tokens":256,
			"messages":[{"role":"user","content":"hi"}]
		}`))
		require.NoError(t, err)
		out3, err := EncodeCanonicalRequest(chatVis, target)
		require.NoError(t, err)
		assert.Equal(t, int64(256), gjson.GetBytes(out3, "parameters.max_tokens").Int())
		assert.False(t, gjson.GetBytes(out3, "parameters.max_completion_tokens").Exists())
	})

	t.Run("gemini_maps_all_limit_sources_to_maxOutputTokens", func(t *testing.T) {
		target := CanonicalTarget{WireFormat: WireGeminiGenerateContent, Model: "gemini-2.0-flash"}
		for _, tc := range []struct {
			req  canonical.Request
			want int64
		}{
			{chatMaxComp, 512},
			{respMaxOut, 768},
			{msgMaxTok, 1024},
		} {
			encoded, err := encodeGeminiCanonicalRequest(tc.req, target)
			require.NoError(t, err)
			assert.Equal(t, tc.want, gjson.GetBytes(encoded, "generationConfig.maxOutputTokens").Int())
		}
	})

	t.Run("zero_output_tokens_warmup_rejected_on_dashscope_and_gemini", func(t *testing.T) {
		msgZero, err := DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, []byte(`{
			"model":"claude-sonnet-4-6",
			"max_tokens":0,
			"messages":[{"role":"user","content":"warmup"}]
		}`))
		require.NoError(t, err)

		_, err = encodeQwenCanonicalRequest(msgZero, CanonicalTarget{WireFormat: WireDashScopeText, Model: "qwen-plus"})
		requireRejectionError(t, err, "unsupported_zero_output_tokens", "max_tokens")

		_, err = encodeGeminiCanonicalRequest(msgZero, CanonicalTarget{WireFormat: WireGeminiGenerateContent, Model: "gemini-2.5-flash"})
		requireRejectionError(t, err, "unsupported_zero_output_tokens", "max_tokens")
	})
}

func TestCanonicalSupplierSearchOptionsFollowTypedStrategy(t *testing.T) {
	req, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, []byte(`{
		"model":"m","messages":[{"role":"user","content":"hi"}],
		"enable_search":true,"search_options":{"strategy":"agent_max","forced_search":true}
	}`))
	require.NoError(t, err)
	for _, tc := range []struct {
		wire WireFormat
		path string
	}{
		{WireDashScopeText, "parameters.search_options"},
		{WireChatCompletions, "search_options"},
	} {
		t.Run(string(tc.wire), func(t *testing.T) {
			target := CanonicalTarget{WireFormat: tc.wire, Model: req.Model}
			encoded, err := EncodeCanonicalRequest(req, target)
			require.NoError(t, err)
			assert.Equal(t, "agent_max", gjson.GetBytes(encoded, tc.path+".strategy").String())

			// Simulate the canonical attempt policy removing only the extractor.
			filtered := req.CloneForAttempt()
			filtered.FixedTools.SearchStrategy = canonical.AbsentOf[string]()
			encoded, err = EncodeCanonicalRequest(filtered, target)
			require.NoError(t, err)
			assert.False(t, gjson.GetBytes(encoded, tc.path+".strategy").Exists())
			assert.True(t, gjson.GetBytes(encoded, tc.path+".forced_search").Bool())
		})
	}
}

// T04: Explicit 0 scalars, DashScope top_p==1.0 clamp, float penalties on Gemini, and stop sequences.
func TestCanonicalSupplierSamplingAndStop(t *testing.T) {
	t.Run("dashscope_explicit_zeros_and_top_p_clamp", func(t *testing.T) {
		zeroReq, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, []byte(`{
			"model":"qwen-plus",
			"temperature":0,
			"top_p":0,
			"top_k":0,
			"presence_penalty":0,
			"repetition_penalty":0,
			"seed":0,
			"stop":"\n\n",
			"messages":[{"role":"user","content":"hi"}]
		}`))
		require.NoError(t, err)

		out, err := encodeQwenCanonicalRequest(zeroReq, CanonicalTarget{
			WireFormat: WireDashScopeText,
			Model:      "qwen-plus",
		})
		require.NoError(t, err)
		for _, path := range []string{
			"parameters.temperature",
			"parameters.top_p",
			"parameters.top_k",
			"parameters.presence_penalty",
			"parameters.repetition_penalty",
			"parameters.seed",
		} {
			res := gjson.GetBytes(out, path)
			assert.True(t, res.Exists(), "expected %s=0 in DashScope JSON: %s", path, string(out))
			assert.Equal(t, float64(0), res.Float())
		}
		assert.Equal(t, []string{"\n\n"}, []string{gjson.GetBytes(out, "parameters.stop.0").String()})

		// S-BOUND-01: top_p == 1.0 -> 0.999999 compatible clamp.
		topP1Req, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, []byte(`{
			"model":"qwen-plus",
			"top_p":1.0,
			"messages":[{"role":"user","content":"hi"}]
		}`))
		require.NoError(t, err)
		outClamp, err := encodeQwenCanonicalRequest(topP1Req, CanonicalTarget{WireFormat: WireDashScopeText, Model: "qwen-plus"})
		require.NoError(t, err)
		assert.InDelta(t, 0.999999, gjson.GetBytes(outClamp, "parameters.top_p").Float(), 1e-9)

		// Invalid top_p > 1.0 rejected.
		badTopP := topP1Req.CloneForAttempt()
		badTopP.Sampling.TopP = canonical.Some(1.1)
		_, err = encodeQwenCanonicalRequest(badTopP, CanonicalTarget{WireFormat: WireDashScopeText, Model: "qwen-plus"})
		requireRejectionError(t, err, "invalid_sampling_parameter", "top_p")
	})

	t.Run("gemini_explicit_zeros_and_float_penalties", func(t *testing.T) {
		zeroGemReq, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, []byte(`{
			"model":"gemini-2.0-flash",
			"temperature":0,
			"top_p":0,
			"top_k":0,
			"presence_penalty":0,
			"frequency_penalty":0,
			"seed":0,
			"messages":[{"role":"user","content":"hi"}]
		}`))
		require.NoError(t, err)
		outZero, err := encodeGeminiCanonicalRequest(zeroGemReq, CanonicalTarget{
			WireFormat: WireGeminiGenerateContent,
			Model:      "gemini-2.0-flash",
		})
		require.NoError(t, err)
		for _, path := range []string{
			"generationConfig.temperature",
			"generationConfig.topP",
			"generationConfig.topK",
			"generationConfig.presencePenalty",
			"generationConfig.frequencyPenalty",
			"generationConfig.seed",
		} {
			res := gjson.GetBytes(outZero, path)
			assert.True(t, res.Exists(), "expected %s=0 in Gemini JSON: %s", path, string(outZero))
			assert.Equal(t, float64(0), res.Float())
		}

		// DEF-01: Fractional presencePenalty / frequencyPenalty must not truncate to int64.
		floatReq, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, []byte(`{
			"model":"gemini-2.0-flash",
			"presence_penalty":0.375,
			"frequency_penalty":-0.625,
			"stop":["END","STOP_HERE"],
			"messages":[{"role":"user","content":"hi"}]
		}`))
		require.NoError(t, err)
		outFloat, err := encodeGeminiCanonicalRequest(floatReq, CanonicalTarget{
			WireFormat: WireGeminiGenerateContent,
			Model:      "gemini-2.0-flash",
		})
		require.NoError(t, err)
		assert.InDelta(t, 0.375, gjson.GetBytes(outFloat, "generationConfig.presencePenalty").Float(), 1e-9)
		assert.InDelta(t, -0.625, gjson.GetBytes(outFloat, "generationConfig.frequencyPenalty").Float(), 1e-9)
		assert.Equal(t, "END", gjson.GetBytes(outFloat, "generationConfig.stopSequences.0").String())
		assert.Equal(t, "STOP_HERE", gjson.GetBytes(outFloat, "generationConfig.stopSequences.1").String())
	})
}

// T07: Lossless >53-bit integer precision (9007199254740993) across schemas, tool args, and responses.
func TestCanonicalSupplierBigIntPrecision_9007199254740993(t *testing.T) {
	const bigIntLiteral = "9007199254740993"

	msgReqBody := []byte(`{
		"model":"claude-sonnet-4-6",
		"max_tokens":256,
		"tools":[{
			"name":"lookup_order",
			"description":"Lookup order by 64-bit snowflake",
			"input_schema":{
				"type":"object",
				"properties":{"order_id":{"type":"integer","enum":[9007199254740993]}},
				"required":["order_id"]
			}
		}],
		"messages":[
			{"role":"user","content":"Check order"},
			{"role":"assistant","content":[
				{"type":"tool_use","id":"toolu_1","name":"lookup_order","input":{"order_id":9007199254740993}}
			]},
			{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"toolu_1","content":"{\"order_id\":9007199254740993,\"status\":\"paid\"}"}
			]}
		]
	}`)
	req, err := DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, msgReqBody)
	require.NoError(t, err)
	req.ResponseFormat = canonical.ResponseFormat{
		Kind:   canonical.ResponseFormatJSONSchema,
		Name:   "order_result",
		Schema: json.RawMessage(`{"type":"object","properties":{"anchor":{"const":9007199254740993}}}`),
	}

	// DashScope request encode
	qwenOut, err := encodeQwenCanonicalRequest(req, CanonicalTarget{
		WireFormat:           WireDashScopeText,
		Model:                "qwen-plus",
		SupportsStrictSchema: true,
	})
	require.NoError(t, err)
	assert.Equal(t, 4, strings.Count(string(qwenOut), bigIntLiteral), "DashScope request must preserve all 4 occurrences of 9007199254740993: %s", string(qwenOut))
	assert.NotContains(t, string(qwenOut), "9007199254740992")

	// Gemini request encode
	gemOut, err := encodeGeminiCanonicalRequest(req, CanonicalTarget{
		WireFormat:           WireGeminiGenerateContent,
		Model:                "gemini-2.5-pro",
		SupportsStrictSchema: true,
	})
	require.NoError(t, err)
	assert.Equal(t, 4, strings.Count(string(gemOut), bigIntLiteral), "Gemini request must preserve all 4 occurrences of 9007199254740993: %s", string(gemOut))
	assert.NotContains(t, string(gemOut), "9007199254740992")

	// Gemini non-streaming & streaming functionCall.args decode
	gemRespBody := []byte(`{
		"responseId":"gem-big",
		"modelVersion":"gemini-2.5-pro",
		"candidates":[{
			"index":0,
			"finishReason":"STOP",
			"content":{
				"role":"model",
				"parts":[{
					"functionCall":{
						"id":"call_big_1",
						"name":"lookup_order",
						"args":{"order_id":9007199254740993}
					}
				}]
			}
		}]
	}`)
	gemTarget := CanonicalTarget{WireFormat: WireGeminiGenerateContent, Model: "gemini-2.5-pro"}
	gemResp, err := decodeGeminiCanonicalResponse(gemTarget, gemRespBody)
	require.NoError(t, err)
	require.Len(t, gemResp.Choices, 1)
	require.Len(t, gemResp.Choices[0].Items, 1)
	assert.Equal(t, canonical.ArgsObject, gemResp.Choices[0].Items[0].Call.Args.Kind)
	assert.Contains(t, string(gemResp.Choices[0].Items[0].Call.Args.Object), bigIntLiteral)

	gemStreamDec, err := newGeminiCanonicalEventDecoder(gemTarget)
	require.NoError(t, err)
	var events []canonical.Event
	require.NoError(t, gemStreamDec.Decode("", gemRespBody, func(ev canonical.Event) error {
		events = append(events, ev)
		return nil
	}))
	var argsDeltas []string
	for _, ev := range events {
		if ev.Kind == canonical.EventToolCallArgsDelta {
			argsDeltas = append(argsDeltas, ev.Text)
		}
	}
	assert.Equal(t, `{"order_id":9007199254740993}`, strings.Join(argsDeltas, ""))
}

// T08 + T09 + DS04 + FixedTools: DashScope text/multimodal shapes, QwenLongFileIDs, cache_control, FixedTools false, and Gemini Data URL vs remote URL rejection.
func TestCanonicalSupplierMultimodalAndToolResults(t *testing.T) {
	const samplePNGDataURL = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
	const samplePNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

	t.Run("dashscope_text_vs_multimodal_shapes_cache_control_and_fixedtools", func(t *testing.T) {
		chatReq, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, []byte(`{
			"model":"qwen-plus",
			"enable_search":false,
			"enable_code_interpreter":false,
			"search_options":{"forced_search":false},
			"messages":[
				{"role":"developer","content":"System rule"},
				{"role":"user","content":[{"type":"text","text":"Cached prompt","cache_control":{"type":"ephemeral","ttl":"5m"}}]},
				{"role":"assistant","content":null,"tool_calls":[{"id":"c1","type":"function","function":{"name":"ping","arguments":"{}"}}]},
				{"role":"tool","tool_call_id":"c1","name":"ping","content":"pong"}
			]
		}`))
		require.NoError(t, err)

		outText, err := encodeQwenCanonicalRequest(chatReq, CanonicalTarget{
			WireFormat:              WireDashScopeText,
			Model:                   "qwen-plus",
			QwenEnableSearchDefault: true, // Must be suppressed by explicit enable_search:false!
		})
		require.NoError(t, err)
		assert.Equal(t, "system", gjson.GetBytes(outText, "input.messages.0.role").String())
		assert.Equal(t, "System rule", gjson.GetBytes(outText, "input.messages.0.content").String())
		assert.Equal(t, "ephemeral", gjson.GetBytes(outText, "input.messages.1.content.0.cache_control.type").String())
		assert.Equal(t, "5m", gjson.GetBytes(outText, "input.messages.1.content.0.cache_control.ttl").String())
		assert.Equal(t, gjson.String, gjson.GetBytes(outText, "input.messages.2.content").Type)
		assert.False(t, gjson.GetBytes(outText, "input.messages.3.name").Exists(), "DashScope tool message must not write tool.name")
		assert.Equal(t, "false", gjson.GetBytes(outText, "parameters.enable_search").Raw)
		assert.Equal(t, "false", gjson.GetBytes(outText, "parameters.enable_code_interpreter").Raw)
		assert.Equal(t, `{"forced_search":false}`, gjson.GetBytes(outText, "parameters.search_options").Raw)

		// Multimodal shape wraps plain system/user strings into [{"text":...}]
		outMM, err := encodeQwenCanonicalRequest(chatReq, CanonicalTarget{
			WireFormat: WireDashScopeMultimodal,
			Model:      "qwen-vl-max",
		})
		require.NoError(t, err)
		assert.Equal(t, "System rule", gjson.GetBytes(outMM, "input.messages.0.content.0.text").String())
	})

	t.Run("qwen_long_file_ids_injection", func(t *testing.T) {
		noSysReq, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, []byte(`{
			"model":"qwen-long",
			"messages":[{"role":"user","content":"Summarize"}]
		}`))
		require.NoError(t, err)

		outNoSys, err := encodeQwenCanonicalRequest(noSysReq, CanonicalTarget{
			WireFormat:      WireDashScopeText,
			Model:           "qwen-long",
			QwenLongFileIDs: []string{"file-111", "file-222"},
		})
		require.NoError(t, err)
		assert.Equal(t, qwenDummySystemMessageContent, gjson.GetBytes(outNoSys, "input.messages.0.content").String())
		assert.Equal(t, "fileid://file-111,fileid://file-222", gjson.GetBytes(outNoSys, "input.messages.1.content").String())
		assert.Equal(t, "Summarize", gjson.GetBytes(outNoSys, "input.messages.2.content").String())

		multiSysReq, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIResponses, []byte(`{
			"model":"qwen-long",
			"instructions":"Rule A",
			"input":[
				{"role":"developer","content":"Rule B"},
				{"role":"user","content":"Summarize"}
			]
		}`))
		require.NoError(t, err)
		outMultiSys, err := encodeQwenCanonicalRequest(multiSysReq, CanonicalTarget{
			WireFormat:      WireDashScopeText,
			Model:           "qwen-long",
			QwenLongFileIDs: []string{"file-999"},
		})
		require.NoError(t, err)
		assert.Equal(t, "Rule A\nRule B", gjson.GetBytes(outMultiSys, "input.messages.0.content").String())
		assert.Equal(t, "fileid://file-999", gjson.GetBytes(outMultiSys, "input.messages.1.content").String())
		assert.Equal(t, "Summarize", gjson.GetBytes(outMultiSys, "input.messages.2.content").String())

		// Non-qwen-long models on a shared connection carrying qwenFileIds must NOT receive fileid:// injection.
		outNonLong, err := encodeQwenCanonicalRequest(noSysReq, CanonicalTarget{
			WireFormat:      WireDashScopeText,
			Model:           "qwen-plus",
			QwenLongFileIDs: []string{"file-111"},
		})
		require.NoError(t, err)
		assert.NotContains(t, string(outNonLong), "fileid://")
	})

	t.Run("messages_multimodal_tool_result_across_dashscope_and_gemini", func(t *testing.T) {
		mmToolReq, err := DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, []byte(`{
			"model":"claude-sonnet-4-6",
			"max_tokens":256,
			"messages":[
				{"role":"user","content":"Take screenshot"},
				{"role":"assistant","content":[{"type":"tool_use","id":"tu_shot","name":"screenshot","input":{}}]},
				{"role":"user","content":[
					{"type":"tool_result","tool_use_id":"tu_shot","content":[
						{"type":"text","text":"Captured screen"},
						{"type":"image","source":{"type":"base64","media_type":"image/png","data":"`+samplePNGBase64+`"}}
					]}
				]}
			]
		}`))
		require.NoError(t, err)

		// WireDashScopeText MUST reject image tool_result with 400.
		_, err = encodeQwenCanonicalRequest(mmToolReq, CanonicalTarget{WireFormat: WireDashScopeText, Model: "qwen-plus"})
		requireRejectionError(t, err, "unsupported_image_content", "tool_result")

		// WireDashScopeMultimodal MUST emit role:"tool" + companion role:"user" image message.
		outMM, err := encodeQwenCanonicalRequest(mmToolReq, CanonicalTarget{WireFormat: WireDashScopeMultimodal, Model: "qwen-vl-max"})
		require.NoError(t, err)
		assert.Equal(t, "tool", gjson.GetBytes(outMM, "input.messages.2.role").String())
		assert.Equal(t, "Captured screen", gjson.GetBytes(outMM, "input.messages.2.content").String())
		assert.Equal(t, "user", gjson.GetBytes(outMM, "input.messages.3.role").String())
		assert.Equal(t, "Attached image(s) from tool result:", gjson.GetBytes(outMM, "input.messages.3.content.0.text").String())
		assert.Equal(t, samplePNGDataURL, gjson.GetBytes(outMM, "input.messages.3.content.1.image").String())

		// WireGeminiGenerateContent MUST emit functionResponse + companion inlineData without base64 re-encode.
		outGem, err := encodeGeminiCanonicalRequest(mmToolReq, CanonicalTarget{WireFormat: WireGeminiGenerateContent, Model: "gemini-2.5-pro"})
		require.NoError(t, err)
		assert.Equal(t, "screenshot", gjson.GetBytes(outGem, "contents.2.parts.0.functionResponse.name").String())
		assert.Equal(t, "Captured screen", gjson.GetBytes(outGem, "contents.2.parts.0.functionResponse.response.output").String())
		assert.Equal(t, "image/png", gjson.GetBytes(outGem, "contents.2.parts.2.inlineData.mimeType").String())
		assert.Equal(t, samplePNGBase64, gjson.GetBytes(outGem, "contents.2.parts.2.inlineData.data").String())
	})

	t.Run("gemini_data_url_inline_and_remote_or_file_id_rejection", func(t *testing.T) {
		dataURLReq, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIResponses, []byte(`{
			"model":"gemini-2.5-flash",
			"input":[{
				"role":"user",
				"content":[
					{"type":"input_text","text":"Describe image"},
					{"type":"input_image","image_url":"`+samplePNGDataURL+`"}
				]
			}]
		}`))
		require.NoError(t, err)
		outGem, err := encodeGeminiCanonicalRequest(dataURLReq, CanonicalTarget{WireFormat: WireGeminiGenerateContent, Model: "gemini-2.5-flash"})
		require.NoError(t, err)
		assert.Equal(t, "image/png", gjson.GetBytes(outGem, "contents.0.parts.1.inlineData.mimeType").String())
		assert.Equal(t, samplePNGBase64, gjson.GetBytes(outGem, "contents.0.parts.1.inlineData.data").String())

		// Remote http(s):// URL MUST be rejected (never placed into inlineData.data!).
		remoteReq, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, []byte(`{
			"model":"gemini-2.5-flash",
			"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"https://example.com/secret.png"}}]}]
		}`))
		require.NoError(t, err)
		_, err = encodeGeminiCanonicalRequest(remoteReq, CanonicalTarget{WireFormat: WireGeminiGenerateContent, Model: "gemini-2.5-flash"})
		rej := requireRejectionError(t, err, "unsupported_image_url", "image_url")
		assert.NotContains(t, rej.SafeDetail(), "example.com")

		// file_id MUST be rejected.
		fileIDReq, err := DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, []byte(`{
			"model":"claude-sonnet-4-6",
			"max_tokens":64,
			"messages":[{"role":"user","content":[{"type":"image","source":{"type":"file","file_id":"file_secret_123"}}]}]
		}`))
		require.NoError(t, err)
		_, err = encodeGeminiCanonicalRequest(fileIDReq, CanonicalTarget{WireFormat: WireGeminiGenerateContent, Model: "gemini-2.5-flash"})
		requireRejectionError(t, err, "unsupported_image_content", "file_id")
	})
}

// T11 + DS02 + DS06: Thinking parameters, preserve_thinking, forced tool_choice conflicts, and Gemini thinkingConfig/toolConfig.
func TestCanonicalSupplierThinkingAndToolChoice(t *testing.T) {
	t.Run("dashscope_thinking_preserve_thinking_and_forced_tool_conflict", func(t *testing.T) {
		msgAdaptive, err := DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, []byte(`{
			"model":"qwen3.6-plus",
			"max_tokens":1024,
			"thinking":{"type":"adaptive"},
			"output_config":{"effort":"high"},
			"tools":[{"name":"read_file","input_schema":{"type":"object"}}],
			"tool_choice":{"type":"auto"},
			"messages":[
				{"role":"user","content":"Step 1"},
				{"role":"assistant","content":[
					{"type":"thinking","thinking":"Prior thought","signature":"anth_sig_1"},
					{"type":"text","text":"Intermediate"}
				]},
				{"role":"user","content":"Step 2"}
			]
		}`))
		require.NoError(t, err)

		outStream, err := encodeQwenCanonicalRequest(msgAdaptive, CanonicalTarget{
			WireFormat: WireDashScopeText,
			Model:      "qwen3.6-plus",
			Streaming:  true,
		})
		require.NoError(t, err)
		assert.True(t, gjson.GetBytes(outStream, "parameters.enable_thinking").Bool())
		assert.True(t, gjson.GetBytes(outStream, "parameters.preserve_thinking").Bool(), "qwen3.6-plus must set parameters.preserve_thinking=true")
		assert.True(t, gjson.GetBytes(outStream, "parameters.incremental_output").Bool(), "thinking+tools under stream requires incremental_output=true")
		assert.False(t, gjson.GetBytes(outStream, "preserve_thinking").Exists(), "DashScope native must not write top-level preserve_thinking")

		// Unsupported model omits preserve_thinking.
		outUnsupported, err := encodeQwenCanonicalRequest(msgAdaptive, CanonicalTarget{
			WireFormat: WireDashScopeText,
			Model:      "qwen-plus",
		})
		require.NoError(t, err)
		assert.False(t, gjson.GetBytes(outUnsupported, "parameters.preserve_thinking").Exists())

		// Explicit disabled thinking does not re-open preserve_thinking.
		disabledReq := msgAdaptive.CloneForAttempt()
		disabledReq.Thinking.Mode = canonical.ThinkingDisabled
		outDisabled, err := encodeQwenCanonicalRequest(disabledReq, CanonicalTarget{
			WireFormat: WireDashScopeText,
			Model:      "qwen3.6-plus",
		})
		require.NoError(t, err)
		assert.Equal(t, "false", gjson.GetBytes(outDisabled, "parameters.enable_thinking").Raw)
		assert.False(t, gjson.GetBytes(outDisabled, "parameters.preserve_thinking").Exists())

		// DS02: Forced tool_choice + enable_thinking rejected.
		conflictReq := msgAdaptive.CloneForAttempt()
		conflictReq.ToolChoice = canonical.ToolChoice{Mode: canonical.ToolChoiceFunction, Name: "read_file"}
		_, err = encodeQwenCanonicalRequest(conflictReq, CanonicalTarget{WireFormat: WireDashScopeText, Model: "qwen3.6-plus"})
		requireRejectionError(t, err, "conflicting_thinking_tool_choice", "tool_choice")
	})

	t.Run("gemini_thinking_config_and_tool_config_modes", func(t *testing.T) {
		gem25Req, err := DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, []byte(`{
			"model":"gemini-2.5-pro",
			"max_tokens":512,
			"thinking":{"type":"enabled","budget_tokens":2048},
			"tools":[{"name":"ping","input_schema":{"type":"object"}}],
			"tool_choice":{"type":"auto"},
			"messages":[{"role":"user","content":"hi"}]
		}`))
		require.NoError(t, err)
		out25, err := encodeGeminiCanonicalRequest(gem25Req, CanonicalTarget{
			WireFormat: WireGeminiGenerateContent,
			Model:      "gemini-2.5-pro",
		})
		require.NoError(t, err)
		assert.True(t, gjson.GetBytes(out25, "generationConfig.thinkingConfig.includeThoughts").Bool())
		assert.Equal(t, int64(2048), gjson.GetBytes(out25, "generationConfig.thinkingConfig.thinkingBudget").Int())
		assert.Equal(t, "AUTO", gjson.GetBytes(out25, "toolConfig.functionCallingConfig.mode").String())

		// Gemini 3 Responses reasoning.effort="high" + forced named function tool_choice MUST succeed (ForbidsForcedToolThink=false)
		gem3Req, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIResponses, []byte(`{
			"model":"gemini-3-pro",
			"reasoning":{"effort":"high"},
			"tools":[{"type":"function","name":"calc","parameters":{"type":"object"}}],
			"tool_choice":{"type":"function","name":"calc"},
			"input":"solve"
		}`))
		require.NoError(t, err)
		outGem3ForcedWithThink, err := encodeGeminiCanonicalRequest(gem3Req, CanonicalTarget{
			WireFormat: WireGeminiGenerateContent,
			Model:      "gemini-3-pro",
		})
		require.NoError(t, err)
		assert.True(t, gjson.GetBytes(outGem3ForcedWithThink, "generationConfig.thinkingConfig.includeThoughts").Bool())
		assert.Equal(t, "HIGH", gjson.GetBytes(outGem3ForcedWithThink, "generationConfig.thinkingConfig.thinkingLevel").String())
		assert.Equal(t, "ANY", gjson.GetBytes(outGem3ForcedWithThink, "toolConfig.functionCallingConfig.mode").String())
		assert.Equal(t, "calc", gjson.GetBytes(outGem3ForcedWithThink, "toolConfig.functionCallingConfig.allowedFunctionNames.0").String())

		// Anthropic Messages manual thinking.type="enabled" + forced tool_choice MUST still reject per AN07!
		msgManualForced := gem25Req.CloneForAttempt()
		msgManualForced.ToolChoice = canonical.ToolChoice{Mode: canonical.ToolChoiceAny}
		_, err = encodeGeminiCanonicalRequest(msgManualForced, CanonicalTarget{
			WireFormat: WireGeminiGenerateContent,
			Model:      "gemini-2.5-pro",
		})
		requireRejectionError(t, err, "conflicting_thinking_tool_choice", "tool_choice")

		// Disabled thinking + named ANY
		gem3Named := gem3Req.CloneForAttempt()
		gem3Named.Thinking = canonical.ThinkingConfig{Mode: canonical.ThinkingDisabled}
		outNamed, err := encodeGeminiCanonicalRequest(gem3Named, CanonicalTarget{
			WireFormat: WireGeminiGenerateContent,
			Model:      "gemini-3-pro",
		})
		require.NoError(t, err)
		assert.Equal(t, "false", gjson.GetBytes(outNamed, "generationConfig.thinkingConfig.includeThoughts").Raw)
		assert.Equal(t, "MINIMAL", gjson.GetBytes(outNamed, "generationConfig.thinkingConfig.thinkingLevel").String())
		assert.Equal(t, "ANY", gjson.GetBytes(outNamed, "toolConfig.functionCallingConfig.mode").String())
		assert.Equal(t, "calc", gjson.GetBytes(outNamed, "toolConfig.functionCallingConfig.allowedFunctionNames.0").String())

		gem3ThinkOnly := gem3Req.CloneForAttempt()
		gem3ThinkOnly.ToolChoice = canonical.ToolChoice{Mode: canonical.ToolChoiceNone}
		outThink3, err := encodeGeminiCanonicalRequest(gem3ThinkOnly, CanonicalTarget{
			WireFormat: WireGeminiGenerateContent,
			Model:      "gemini-3-pro",
		})
		require.NoError(t, err)
		assert.True(t, gjson.GetBytes(outThink3, "generationConfig.thinkingConfig.includeThoughts").Bool())
		assert.Equal(t, "HIGH", gjson.GetBytes(outThink3, "generationConfig.thinkingConfig.thinkingLevel").String())
		assert.Equal(t, "NONE", gjson.GetBytes(outThink3, "toolConfig.functionCallingConfig.mode").String())

		// Non-2.5/non-3 Gemini model with SupportsThinking=true and ThinkingUnset must not fabricate thinkingConfig.
		plainGem20, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, []byte(`{
			"model":"gemini-2.0-flash",
			"messages":[{"role":"user","content":"hi"}]
		}`))
		require.NoError(t, err)
		outPlain20, err := encodeGeminiCanonicalRequest(plainGem20, CanonicalTarget{
			WireFormat:       WireGeminiGenerateContent,
			Supplier:         "gemini",
			Model:            "gemini-2.0-flash",
			SupportsThinking: true,
		})
		require.NoError(t, err)
		assert.False(t, gjson.GetBytes(outPlain20, "generationConfig.thinkingConfig").Exists())
	})
}

// T12 + T14: Response endings, finish_reason null/unknown, DashScope output.text, Gemini promptFeedback & thought separation.
func TestCanonicalSupplierOpaqueAndResponseEndings(t *testing.T) {
	t.Run("dashscope_response_shapes_and_finish_reasons", func(t *testing.T) {
		targetText := CanonicalTarget{WireFormat: WireDashScopeText, Model: "qwen-plus"}

		// output.text fallback + unknown finish_reason
		respText, err := decodeQwenCanonicalResponse(targetText, []byte(`{
			"request_id":"req-txt-1",
			"output":{"text":"Fallback answer","finish_reason":"custom_vendor_halt"}
		}`))
		require.NoError(t, err)
		assert.Equal(t, "req-txt-1", respText.ID)
		require.Len(t, respText.Choices, 1)
		assert.Equal(t, canonical.FinishUnknown, respText.Choices[0].Finish.Category)
		assert.Equal(t, "custom_vendor_halt", respText.Choices[0].Finish.RawReason)
		require.Len(t, respText.Choices[0].Items, 1)
		assert.Equal(t, "Fallback answer", respText.Choices[0].Items[0].Parts[0].Text)

		// finish_reason: "null", "", and JSON null -> FinishNone
		for _, rawFin := range []string{`"null"`, `""`, `null`} {
			r, err := decodeQwenCanonicalResponse(targetText, []byte(`{
				"request_id":"req-n",
				"output":{"choices":[{"finish_reason":`+rawFin+`,"message":{"role":"assistant","content":"partial"}}]}
			}`))
			require.NoError(t, err)
			assert.Equal(t, canonical.FinishNone, r.Choices[0].Finish.Category, "finish_reason=%s must map to FinishNone", rawFin)
		}

		// Multimodal response with reasoning + array text/image + tool_calls
		respMM, err := decodeQwenCanonicalResponse(CanonicalTarget{WireFormat: WireDashScopeMultimodal, Model: "qwen-vl-max"}, []byte(`{
			"request_id":"req-mm-1",
			"output":{"choices":[{
				"finish_reason":"tool_calls",
				"message":{
					"role":"assistant",
					"reasoning_content":"Inspecting visual",
					"content":[{"text":"Caption"},{"image":"https://dashscope.example/out.png"}],
					"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"tag","arguments":"{\"label\":\"cat\"}"}}]
				}
			}]}
		}`))
		require.NoError(t, err)
		require.Len(t, respMM.Choices[0].Items, 3)
		assert.Equal(t, canonical.ItemReasoning, respMM.Choices[0].Items[0].Kind)
		assert.Equal(t, canonical.ItemMessage, respMM.Choices[0].Items[1].Kind)
		assert.Len(t, respMM.Choices[0].Items[1].Parts, 2)
		assert.Equal(t, canonical.PartImage, respMM.Choices[0].Items[1].Parts[1].Kind)
		assert.Equal(t, canonical.ItemFunctionCall, respMM.Choices[0].Items[2].Kind)
		assert.Equal(t, `{"label":"cat"}`, respMM.Choices[0].Items[2].Call.Args.Text)

		// Provider error payload rejected
		_, err = decodeQwenCanonicalResponse(targetText, []byte(`{"request_id":"req-err","code":"InvalidParameter","message":"bad param"}`))
		require.Error(t, err)
	})

	t.Run("gemini_finish_reasons_prompt_feedback_multi_candidate_and_interactions_guard", func(t *testing.T) {
		gemTarget := CanonicalTarget{WireFormat: WireGeminiGenerateContent, Model: "gemini-2.5-pro"}

		// Candidateless promptFeedback.blockReason -> FinishContentFilter + RawReason
		blockedResp, err := decodeGeminiCanonicalResponse(gemTarget, []byte(`{
			"responseId":"gem-blocked",
			"promptFeedback":{"blockReason":"PROHIBITED_CONTENT"}
		}`))
		require.NoError(t, err)
		require.Len(t, blockedResp.Choices, 1)
		assert.Empty(t, blockedResp.Choices[0].Items)
		assert.Equal(t, canonical.FinishContentFilter, blockedResp.Choices[0].Finish.Category)
		assert.Equal(t, "PROHIBITED_CONTENT", blockedResp.Choices[0].Finish.RawReason)

		// Multi-candidate finishReasons including SAFETY, MAX_TOKENS, SPII, BLOCKLIST, RECITATION, and UNKNOWN
		multiCandResp, err := decodeGeminiCanonicalResponse(gemTarget, []byte(`{
			"responseId":"gem-multi",
			"candidates":[
				{"index":0,"finishReason":"STOP","content":{"role":"model","parts":[{"text":"ok"}]}},
				{"index":1,"finishReason":"MAX_TOKENS","content":{"role":"model","parts":[{"text":"cut"}]}},
				{"index":2,"finishReason":"SAFETY","content":{"role":"model","parts":[]}},
				{"index":3,"finishReason":"RECITATION","content":{"role":"model","parts":[]}},
				{"index":4,"finishReason":"BLOCKLIST","content":{"role":"model","parts":[]}},
				{"index":5,"finishReason":"SPII","content":{"role":"model","parts":[]}},
				{"index":6,"finishReason":"MALFORMED_FUNCTION_CALL","content":{"role":"model","parts":[]}}
			]
		}`))
		require.NoError(t, err)
		require.Len(t, multiCandResp.Choices, 7)
		assert.Equal(t, canonical.FinishStop, multiCandResp.Choices[0].Finish.Category)
		assert.Equal(t, canonical.FinishLength, multiCandResp.Choices[1].Finish.Category)
		for _, idx := range []int{2, 3, 4, 5} {
			assert.Equal(t, canonical.FinishContentFilter, multiCandResp.Choices[idx].Finish.Category)
			assert.Equal(t, idx, multiCandResp.Choices[idx].Index)
		}
		assert.Equal(t, canonical.FinishUnknown, multiCandResp.Choices[6].Finish.Category)
		assert.Equal(t, "MALFORMED_FUNCTION_CALL", multiCandResp.Choices[6].Finish.RawReason)

		// Interactions API payload MUST be rejected
		_, err = decodeGeminiCanonicalResponse(gemTarget, []byte(`{"object":"interaction","id":"int_123","outputs":[]}`))
		require.Error(t, err)
	})
}

// T16 + T23: DashScope cumulative vs incremental streaming, UTF-8 non-prefix rollback, terminal frame with final delta, and per-attempt isolation.
func TestCanonicalDashScopeStreamModesAndIsolation(t *testing.T) {
	t.Run("incremental_mode_dedups_repeated_tool_id_name_and_empty_tail_frame", func(t *testing.T) {
		dec, err := newQwenCanonicalEventDecoder(CanonicalTarget{
			WireFormat:            WireDashScopeText,
			Model:                 "qwen-flash",
			IncrementalOutputMode: IncrementalForceTrue,
		})
		require.NoError(t, err)

		frames := []string{
			`{"request_id":"req-inc-1","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","content":"","reasoning_content":"Thinking 1..."}}]}}`,
			`{"request_id":"req-inc-1","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","content":"","tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"read_file","arguments":""}},{"index":1,"id":"call_b","type":"function","function":{"name":"list_dir","arguments":""}}]}}]}}`,
			// Repeated id and name on subsequent delta frame MUST NOT re-emit EventToolCallStart!
			`{"request_id":"req-inc-1","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","content":"","tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"read_file","arguments":"{\"p\":\"a\"}"}},{"index":1,"id":"call_b","type":"function","function":{"name":"list_dir","arguments":"{\"d\":"}}]}}]}}`,
			// Terminal frame simultaneously carries final delta for tool 1 AND empty function:{} for tool 0 AND finish_reason!
			`{"request_id":"req-inc-1","output":{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","tool_calls":[{"index":0,"type":"function","function":{}},{"index":1,"id":"call_b","type":"function","function":{"name":"list_dir","arguments":"\".\"}"}}]}}]}}`,
		}

		var events []canonical.Event
		for _, f := range frames {
			require.NoError(t, dec.Decode("result", []byte(f), func(ev canonical.Event) error {
				events = append(events, ev)
				return nil
			}))
		}
		require.NoError(t, dec.End())

		var kinds []canonical.EventKind
		toolStarts := 0
		argsByTool := map[int]string{}
		for _, ev := range events {
			kinds = append(kinds, ev.Kind)
			if ev.Kind == canonical.EventToolCallStart {
				toolStarts++
			}
			if ev.Kind == canonical.EventToolCallArgsDelta {
				argsByTool[ev.ToolIndex] += ev.Text
			}
		}
		assert.Equal(t, 2, toolStarts, "each parallel tool must start exactly once despite repeated id/name")
		assert.Equal(t, `{"p":"a"}`, argsByTool[0])
		assert.Equal(t, `{"d":"."}`, argsByTool[1])
		// Last two events MUST be the final EventToolCallArgsDelta followed by EventChoiceFinish!
		require.GreaterOrEqual(t, len(kinds), 2)
		assert.Equal(t, canonical.EventToolCallArgsDelta, kinds[len(kinds)-2])
		assert.Equal(t, canonical.EventChoiceFinish, kinds[len(kinds)-1])
	})

	t.Run("cumulative_mode_utf8_diff_non_prefix_rollback_unready_tools_and_terminal_delta", func(t *testing.T) {
		dec, err := newQwenCanonicalEventDecoder(CanonicalTarget{
			WireFormat:            WireDashScopeText,
			Model:                 "qwen-plus",
			IncrementalOutputMode: IncrementalForceFalse,
		})
		require.NoError(t, err)

		var emittedText strings.Builder
		var emittedReasoning strings.Builder
		var events []canonical.Event

		push := func(payload string) {
			events = nil
			require.NoError(t, dec.Decode("result", []byte(payload), func(ev canonical.Event) error {
				events = append(events, ev)
				if ev.Kind == canonical.EventTextDelta {
					emittedText.WriteString(ev.Text)
				}
				if ev.Kind == canonical.EventReasoningDelta {
					emittedReasoning.WriteString(ev.Text)
				}
				return nil
			}))
		}

		// Frame 0: cumulative CJK + emoji prefix growth
		push(`{"request_id":"req-cum","output":{"choices":[{"finish_reason":null,"message":{"role":"assistant","reasoning_content":"思考","content":"你好，🚀"}}]}}`)
		assert.Equal(t, "思考", emittedReasoning.String())
		assert.Equal(t, "你好，🚀", emittedText.String())

		// Frame 1: non-prefix rollback ("你好" shorter than "你好，🚀", divergent reasoning "重来") -> MUST NOT duplicate or retract!
		push(`{"request_id":"req-cum","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","reasoning_content":"重来","content":"你好"}}]}}`)
		assert.Empty(t, events, "non-prefix rollback must emit zero duplicate or retraction events")
		assert.Equal(t, "你好，🚀", emittedText.String())
		assert.Equal(t, "思考", emittedReasoning.String())

		// Frame 2: recovery extending original prefix + unready tool_call (arguments=="") held back!
		push(`{"request_id":"req-cum","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","reasoning_content":"思考完毕","content":"你好，🚀世界","tool_calls":[{"index":0,"id":"c_w","type":"function","function":{"name":"weather","arguments":""}}]}}]}}`)
		assert.Equal(t, "思考完毕", emittedReasoning.String())
		assert.Equal(t, "你好，🚀世界", emittedText.String())
		for _, ev := range events {
			assert.NotEqual(t, canonical.EventToolCallStart, ev.Kind, "unready cumulative tool_call must not emit before arguments arrive")
		}

		// Frame 3: partial cumulative tool arguments
		push(`{"request_id":"req-cum","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","reasoning_content":"思考完毕","content":"你好，🚀世界","tool_calls":[{"index":0,"id":"c_w","type":"function","function":{"name":"weather","arguments":"{\"city\":"}}]}}]}}`)
		require.Len(t, events, 2)
		assert.Equal(t, canonical.EventToolCallStart, events[0].Kind)
		assert.Equal(t, canonical.EventToolCallArgsDelta, events[1].Kind)
		assert.Equal(t, `{"city":`, events[1].Text)

		// Frame 4: terminal frame carrying both final text suffix "！" + final tool args suffix + finish_reason!
		push(`{"request_id":"req-cum","output":{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","reasoning_content":"思考完毕","content":"你好，🚀世界！","tool_calls":[{"index":0,"id":"c_w","type":"function","function":{"name":"weather","arguments":"{\"city\":\"HZ\"}"}}]}}]}}`)
		require.Len(t, events, 3)
		assert.Equal(t, canonical.EventTextDelta, events[0].Kind)
		assert.Equal(t, "！", events[0].Text)
		assert.Equal(t, canonical.EventToolCallArgsDelta, events[1].Kind)
		assert.Equal(t, `"HZ"}`, events[1].Text)
		assert.Equal(t, canonical.EventChoiceFinish, events[2].Kind)
		assert.Equal(t, canonical.FinishToolCalls, events[2].Finish.Category)
		assert.Equal(t, "你好，🚀世界！", emittedText.String())
	})

	t.Run("interleaved_per_attempt_decoders_and_provider_error", func(t *testing.T) {
		d1, err := newQwenCanonicalEventDecoder(CanonicalTarget{WireFormat: WireDashScopeText, Model: "qwen-plus", IncrementalOutputMode: IncrementalForceFalse})
		require.NoError(t, err)
		d2, err := newQwenCanonicalEventDecoder(CanonicalTarget{WireFormat: WireDashScopeMultimodal, Model: "qwen-vl-max", IncrementalOutputMode: IncrementalForceTrue})
		require.NoError(t, err)

		var out1, out2 []canonical.Event
		require.NoError(t, d1.Decode("result", []byte(`{"request_id":"a1","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","content":"Alpha"}}]}}`), func(ev canonical.Event) error {
			out1 = append(out1, ev)
			return nil
		}))
		require.NoError(t, d2.Decode("result", []byte(`{"request_id":"a2","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","content":[{"text":"Beta"}]}}]}}`), func(ev canonical.Event) error {
			out2 = append(out2, ev)
			return nil
		}))
		require.NoError(t, d1.Decode("result", []byte(`{"request_id":"a1","output":{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"AlphaBeta"}}]}}`), func(ev canonical.Event) error {
			out1 = append(out1, ev)
			return nil
		}))
		require.NoError(t, d2.Decode("error", []byte(`{"request_id":"a2","code":"Throttling","message":"rate limit"}`), func(ev canonical.Event) error {
			out2 = append(out2, ev)
			return nil
		}))

		assert.Equal(t, "Beta", out1[2].Text, "d1 cumulative prefix must remain isolated from d2")
		assert.Equal(t, canonical.EventProviderError, out2[len(out2)-1].Kind)
	})

	t.Run("bind_qwen_incremental_mode_pairs_request_and_cumulative_tool_decoder_under_auto", func(t *testing.T) {
		toolNoThinkReq, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, []byte(`{
			"model":"qwen-plus",
			"stream":true,
			"tools":[{"type":"function","function":{"name":"get_weather","parameters":{"type":"object"}}}],
			"messages":[{"role":"user","content":"Weather in Hangzhou?"}]
		}`))
		require.NoError(t, err)

		autoTarget := CanonicalTarget{
			WireFormat:            WireDashScopeText,
			Supplier:              providerTypeQwen,
			Model:                 "qwen-plus",
			Streaming:             true,
			IncrementalOutputMode: IncrementalAuto,
		}
		boundTarget := BindQwenIncrementalMode(toolNoThinkReq, autoTarget)
		assert.Equal(t, IncrementalForceFalse, boundTarget.IncrementalOutputMode)

		encodedReq, err := encodeQwenCanonicalRequest(toolNoThinkReq, boundTarget)
		require.NoError(t, err)
		assert.Equal(t, "false", gjson.GetBytes(encodedReq, "parameters.incremental_output").Raw)

		dec, err := newQwenCanonicalEventDecoder(boundTarget)
		require.NoError(t, err)

		var gotArgs strings.Builder
		var gotText strings.Builder
		for _, frame := range []string{
			`{"request_id":"r-auto-cum","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","content":"Checking ","tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"get_weather","arguments":""}}]}}]}}`,
			`{"request_id":"r-auto-cum","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","content":"Checking weather","tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":"}}]}}]}}`,
			`{"request_id":"r-auto-cum","output":{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"Checking weather!","tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"HZ\"}"}}]}}]}}`,
		} {
			require.NoError(t, dec.Decode("result", []byte(frame), func(ev canonical.Event) error {
				if ev.Kind == canonical.EventTextDelta {
					gotText.WriteString(ev.Text)
				}
				if ev.Kind == canonical.EventToolCallArgsDelta {
					gotArgs.WriteString(ev.Text)
				}
				return nil
			}))
		}
		assert.Equal(t, "Checking weather!", gotText.String(), "cumulative text must not duplicate prefix under IncrementalAuto+tools")
		assert.Equal(t, `{"city":"HZ"}`, gotArgs.String(), "cumulative tool args must not duplicate prefix under IncrementalAuto+tools")

		// Conversely, tools + active thinking under IncrementalAuto binds IncrementalForceTrue.
		toolWithThinkReq := toolNoThinkReq.CloneForAttempt()
		toolWithThinkReq.Thinking = canonical.ThinkingConfig{Mode: canonical.ThinkingEnabled}
		boundThinkTarget := BindQwenIncrementalMode(toolWithThinkReq, autoTarget)
		assert.Equal(t, IncrementalForceTrue, boundThinkTarget.IncrementalOutputMode)
	})
}

// T19 + C05: Gemini multi-system, 2-turn functionCall/functionResponse, thoughtSignature Google ownership,
// missing CallID salted projection, stream thought separation, and Moonshot nested choices[0].usage.
func TestCanonicalGeminiMultiTurnStreamAndMoonshot(t *testing.T) {
	gemTarget := CanonicalTarget{
		WireFormat: WireGeminiGenerateContent,
		Supplier:   "gemini",
		Model:      "gemini-2.5-pro",
		GeminiSafetySettings: map[string]string{
			"HARM_CATEGORY_DANGEROUS_CONTENT": "BLOCK_ONLY_HIGH",
			"HARM_CATEGORY_HATE_SPEECH":       "BLOCK_MEDIUM_AND_ABOVE",
		},
	}

	t.Run("multi_system_concatenation_and_two_turn_tool_history_with_signature_guard", func(t *testing.T) {
		// Turn 1 upstream Gemini response: thought=true + functionCall (no native id) + Google thoughtSignature.
		turn1Upstream := []byte(`{
			"responseId":"gem-turn-1",
			"modelVersion":"gemini-2.5-pro",
			"candidates":[{
				"index":0,
				"finishReason":"STOP",
				"content":{
					"role":"model",
					"parts":[
						{"text":"Checking database first.","thought":true},
						{
							"functionCall":{
								"name":"query_user",
								"args":{"uid":9007199254740993}
							},
							"thoughtSignature":"google_sig_abc_123"
						}
					]
				}
			}]
		}`)
		resp1, err := decodeGeminiCanonicalResponse(gemTarget, turn1Upstream)
		require.NoError(t, err)
		require.Len(t, resp1.Choices, 1)
		assert.Equal(t, canonical.FinishToolCalls, resp1.Choices[0].Finish.Category, "STOP with functionCall must map to FinishToolCalls")
		require.Len(t, resp1.Choices[0].Items, 2)

		// Verify thought:true is separated into ItemReasoning and NOT mixed into visible text!
		assert.Equal(t, canonical.ItemReasoning, resp1.Choices[0].Items[0].Kind)
		assert.Equal(t, []string{"Checking database first."}, resp1.Choices[0].Items[0].Reasoning.Text)
		assert.Equal(t, "google", resp1.Choices[0].Items[0].Reasoning.Origin.Supplier)

		// Verify native missing id stays "" for F07 B02 salted slot alias!
		fcItem := resp1.Choices[0].Items[1]
		assert.Equal(t, canonical.ItemFunctionCall, fcItem.Kind)
		assert.Equal(t, "", fcItem.Call.CallID)
		assert.Equal(t, "google_sig_abc_123", fcItem.Call.ThoughtSignature)
		assert.Equal(t, "google", fcItem.Call.SignatureOrigin.Supplier)

		// Verify B02 salted slot alias minting through ProjectCanonical.
		projected, err := clientresponse.ProjectCanonical(clientresponse.Policy{
			Protocol:    clientresponse.Responses,
			PublicModel: "public-gemini",
			RequestID:   "req-trace-1",
		}, resp1, clientresponse.Usage{})
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(projected.Choices[0].Items[1].PublicCallID, "call_pf_"))

		// Turn 2 canonical request: multiple system instructions + Google signed call + Anthropic signed thinking (must NOT forge signature) + error tool result!
		turn2Req := canonical.Request{
			SourceProtocol: canonical.ProtocolAnthropicMessages,
			Model:          "gemini-2.5-pro",
			Instructions:   canonical.Some("Instruction 0"),
			SystemParts:    []canonical.Part{{Kind: canonical.PartText, Text: "System Part 1"}},
			Items: []canonical.Item{
				{Kind: canonical.ItemMessage, Role: canonical.RoleDeveloper, Parts: []canonical.Part{{Kind: canonical.PartText, Text: "Developer Rule 2"}}},
				{Kind: canonical.ItemMessage, Role: canonical.RoleSystem, Parts: []canonical.Part{{Kind: canonical.PartText, Text: "System Rule 3"}}},
				{Kind: canonical.ItemMessage, Role: canonical.RoleUser, Parts: []canonical.Part{{Kind: canonical.PartText, Text: "Find user 9007199254740993"}}},
				// Foreign Anthropic signature must NOT leak into Gemini thoughtSignature:
				{
					Kind: canonical.ItemReasoning,
					Role: canonical.RoleAssistant,
					Reasoning: &canonical.ReasoningItem{
						Text:      []string{"Foreign thought"},
						Signature: "FOREIGN_ANTHROPIC_SIG_DO_NOT_SEND",
						Origin:    canonical.OpaqueOrigin{Protocol: canonical.ProtocolAnthropicMessages, Supplier: "anthropic"},
					},
				},
				{
					Kind: canonical.ItemFunctionCall,
					Role: canonical.RoleAssistant,
					Call: &canonical.FunctionCall{
						CallID:           "call_restored_1",
						Name:             "query_user",
						Args:             canonical.Arguments{Kind: canonical.ArgsText, Text: `{"uid":9007199254740993}`},
						ThoughtSignature: "google_sig_abc_123",
						SignatureOrigin:  canonical.OpaqueOrigin{Protocol: canonical.ProtocolGeminiGenerate, Supplier: "google"},
					},
				},
				{
					Kind: canonical.ItemFunctionResult,
					Role: canonical.RoleTool,
					Result: &canonical.FunctionResult{
						CallID:       "call_restored_1",
						Parts:        []canonical.Part{{Kind: canonical.PartText, Text: "Database timeout"}},
						IsStringForm: true,
						IsError:      true,
					},
				},
			},
		}

		turn2Wire, err := encodeGeminiCanonicalRequest(turn2Req, gemTarget)
		require.NoError(t, err)

		// 1. All 4 system/developer/instructions segments concatenated in order without overwrite!
		assert.Equal(t, "Instruction 0", gjson.GetBytes(turn2Wire, "systemInstruction.parts.0.text").String())
		assert.Equal(t, "System Part 1", gjson.GetBytes(turn2Wire, "systemInstruction.parts.1.text").String())
		assert.Equal(t, "Developer Rule 2", gjson.GetBytes(turn2Wire, "systemInstruction.parts.2.text").String())
		assert.Equal(t, "System Rule 3", gjson.GetBytes(turn2Wire, "systemInstruction.parts.3.text").String())

		// 2. Foreign signature stripped, Google signature preserved, 9007199254740993 preserved!
		assert.NotContains(t, string(turn2Wire), "FOREIGN_ANTHROPIC_SIG_DO_NOT_SEND")
		assert.Equal(t, "google_sig_abc_123", gjson.GetBytes(turn2Wire, "contents.1.parts.1.thoughtSignature").String())
		assert.Equal(t, "call_restored_1", gjson.GetBytes(turn2Wire, "contents.1.parts.1.functionCall.id").String())
		assert.Equal(t, "9007199254740993", gjson.GetBytes(turn2Wire, "contents.1.parts.1.functionCall.args.uid").Raw)

		// 3. FunctionResult recovers name="query_user", encodes role:"user" functionResponse with is_error:true, zero role:"tool"!
		assert.NotContains(t, string(turn2Wire), `"role":"tool"`)
		assert.Equal(t, "user", gjson.GetBytes(turn2Wire, "contents.2.role").String())
		assert.Equal(t, "call_restored_1", gjson.GetBytes(turn2Wire, "contents.2.parts.0.functionResponse.id").String())
		assert.Equal(t, "query_user", gjson.GetBytes(turn2Wire, "contents.2.parts.0.functionResponse.name").String())
		assert.Equal(t, "Database timeout", gjson.GetBytes(turn2Wire, "contents.2.parts.0.functionResponse.response.error").String())
		assert.True(t, gjson.GetBytes(turn2Wire, "contents.2.parts.0.functionResponse.response.is_error").Bool())

		// 4. Deterministic sorted safetySettings!
		assert.Equal(t, "HARM_CATEGORY_DANGEROUS_CONTENT", gjson.GetBytes(turn2Wire, "safetySettings.0.category").String())
		assert.Equal(t, "HARM_CATEGORY_HATE_SPEECH", gjson.GetBytes(turn2Wire, "safetySettings.1.category").String())

		// 5. Missing historical function_call in Chat role:"tool" -> cannot resolve function name -> RejectionError!
		orphanReq, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIChat, []byte(`{
			"model":"gemini-2.5-pro",
			"messages":[
				{"role":"user","content":"hi"},
				{"role":"tool","tool_call_id":"orphan_call_1","content":"ok"}
			]
		}`))
		require.NoError(t, err)
		_, err = encodeGeminiCanonicalRequest(orphanReq, gemTarget)
		requireRejectionError(t, err, "missing_tool_call_association", "tool_call_id")
	})

	t.Run("gemini_streaming_thought_separation_prompt_feedback_and_interactions_guard", func(t *testing.T) {
		dec, err := newGeminiCanonicalEventDecoder(gemTarget)
		require.NoError(t, err)

		var events []canonical.Event
		emit := func(ev canonical.Event) error {
			events = append(events, ev)
			return nil
		}

		require.NoError(t, dec.Decode("", []byte(`{"responseId":"gs-1","modelVersion":"gemini-2.5-pro","candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"Step A","thought":true,"thoughtSignature":"gsig_1"}]}}]}`), emit))
		require.NoError(t, dec.Decode("", []byte(`{"responseId":"gs-1","modelVersion":"gemini-2.5-pro","candidates":[{"index":0,"finishReason":"STOP","content":{"role":"model","parts":[{"text":"Visible B"}]}}]}`), emit))

		var reasoningTexts, visibleTexts, sigs []string
		for _, ev := range events {
			switch ev.Kind {
			case canonical.EventReasoningDelta:
				reasoningTexts = append(reasoningTexts, ev.Text)
			case canonical.EventReasoningSignatureDelta:
				sigs = append(sigs, ev.Text)
			case canonical.EventTextDelta:
				visibleTexts = append(visibleTexts, ev.Text)
			}
		}
		assert.Equal(t, []string{"Step A"}, reasoningTexts)
		assert.Equal(t, []string{"gsig_1"}, sigs)
		assert.Equal(t, []string{"Visible B"}, visibleTexts)
		assert.Equal(t, canonical.EventChoiceFinish, events[len(events)-1].Kind)
		assert.Equal(t, canonical.FinishStop, events[len(events)-1].Finish.Category)

		// Candidateless promptFeedback in stream
		decBlocked, err := newGeminiCanonicalEventDecoder(gemTarget)
		require.NoError(t, err)
		var blockedEvents []canonical.Event
		require.NoError(t, decBlocked.Decode("", []byte(`{"promptFeedback":{"blockReason":"SAFETY"}}`), func(ev canonical.Event) error {
			blockedEvents = append(blockedEvents, ev)
			return nil
		}))
		require.Len(t, blockedEvents, 2)
		assert.Equal(t, canonical.EventChoiceFinish, blockedEvents[1].Kind)
		assert.Equal(t, canonical.FinishContentFilter, blockedEvents[1].Finish.Category)
		assert.Equal(t, "SAFETY", blockedEvents[1].Finish.RawReason)

		// Interactions API SSE event name rejected
		require.Error(t, decBlocked.Decode("interaction.start", []byte(`{"id":"int_1"}`), emit))
	})

	t.Run("moonshot_c05_nested_choice_usage_stream_and_response_compat", func(t *testing.T) {
		moonshotTarget := CanonicalTarget{
			WireFormat: WireChatCompletions,
			Supplier:   "moonshot",
			Model:      "moonshot-v1-32k",
		}
		dec, err := NewCanonicalEventDecoder(moonshotTarget)
		require.NoError(t, err)

		sseFrames := [][]byte{
			[]byte(`{"id":"cmpl-moon-1","model":"moonshot-v1-32k","choices":[{"index":0,"delta":{"role":"assistant","content":"Moonshot ok"}}],"created":1700000000}`),
			[]byte(`{"id":"cmpl-moon-1","model":"moonshot-v1-32k","choices":[{"index":0,"delta":{},"finish_reason":"stop","usage":{"prompt_tokens":9007199254740993,"completion_tokens":12,"total_tokens":9007199254741005}}],"created":1700000000}`),
			[]byte(`[DONE]`),
		}

		var gotEvents []canonical.Event
		var rawSSE bytes.Buffer
		for _, frame := range sseFrames {
			rawSSE.WriteString("data: ")
			rawSSE.Write(frame)
			rawSSE.WriteString("\n\n")
			require.NoError(t, dec.Decode("", frame, func(ev canonical.Event) error {
				gotEvents = append(gotEvents, ev)
				return nil
			}))
		}
		assert.Equal(t, canonical.EventTextDelta, gotEvents[1].Kind)
		assert.Equal(t, "Moonshot ok", gotEvents[1].Text)
		assert.Equal(t, canonical.EventChoiceFinish, gotEvents[2].Kind)
		assert.Equal(t, canonical.FinishStop, gotEvents[2].Finish.Category)
		assert.Equal(t, canonical.EventProtocolDone, gotEvents[3].Kind)

		usage := clientresponse.ReadSupplierUsage(rawSSE.Bytes(), "moonshot", true)
		assert.Equal(t, "9007199254740993", string(usage.Input))
		assert.Equal(t, "12", string(usage.Output))
		assert.Equal(t, "9007199254741005", string(usage.Total))
	})
}

// T10 + T24 + H3: Dual-track parity between same-wire openai_chat -> qwen compatible
// (sanitizeQwenCompatibleChatRequest + top-level preserve_thinking) and cross-wire
// responses/messages -> qwen compatible (EncodeCanonicalRequest) via shared qwenCompatibleChatPolicy.
func TestCanonicalQwenCompatibleDualTrackParity_H3(t *testing.T) {
	const mappedModel = "qwen3.6-plus"

	// Track 1: Same-wire openai_chat -> Qwen OpenAI-compatible (TransformRequestBodyHeaders / sanitizeQwenCompatibleChatRequest)
	qwenCompatProvider := &qwenProvider{
		config: ProviderConfig{
			typ:                  providerTypeQwen,
			qwenEnableCompatible: true,
			modelMapping: map[string]string{
				"public-qwen": mappedModel,
			},
		},
	}
	sameWireChatInput := []byte(`{
		"model": "public-qwen",
		"stream": true,
		"stream_options": {"include_usage": true},
		"store": false,
		"prompt_cache_key": "pck_1",
		"prompt_cache_retention": "24h",
		"prompt_cache_options": {"mode": "explicit"},
		"max_completion_tokens": 1800,
		"reasoning_effort": "medium",
		"tools": [{
			"type": "function",
			"cache_control": {"type": "ephemeral"},
			"function": {"name": "read_file", "parameters": {"type": "object", "properties": {"id": {"const": 9007199254740993}}}}
		}],
		"messages": [
			{"role": "developer", "content": "Be concise."},
			{"role": "user", "content": [{"type": "text", "text": "Inspect", "cache_control": {"type": "ephemeral", "ttl": "5m"}}]},
			{
				"role": "assistant",
				"content": null,
				"reasoning_content": "Thinking trace",
				"cache_control": {"type": "ephemeral"},
				"tool_calls": [{
					"id": "call_1",
					"type": "function",
					"extra_content": {"google": {"thought_signature": "sig"}},
					"function": {"name": "read_file", "arguments": "{\"id\":9007199254740993}"}
				}]
			},
			{"role": "tool", "tool_call_id": "call_1", "name": "read_file", "content": "ok"}
		]
	}`)
	sameWireOut, err := qwenCompatProvider.TransformRequestBodyHeaders(nil, ApiNameChatCompletion, sameWireChatInput, http.Header{})
	require.NoError(t, err)

	qwenCompatTarget := CanonicalTarget{
		WireFormat:       WireChatCompletions,
		Supplier:         providerTypeQwen,
		Model:            mappedModel,
		Streaming:        true,
		SupportsThinking: true,
	}
	qwenCompatibleChatPolicy(mappedModel).applyToTarget(&qwenCompatTarget)

	// Track 2: Cross-wire openai_responses -> Qwen OpenAI-compatible (EncodeCanonicalRequest)
	// Note per OA08/DS04: OpenAI Responses uses prompt_cache_breakpoint (not Anthropic cache_control),
	// which is stripped rather than silently rewritten across cache systems.
	responsesInput := []byte(`{
		"model": "public-qwen",
		"stream": true,
		"store": false,
		"prompt_cache_key": "pck_1",
		"prompt_cache_retention": "24h",
		"prompt_cache_options": {"mode": "explicit"},
		"max_output_tokens": 1800,
		"reasoning": {"effort": "medium"},
		"tools": [{
			"type": "function",
			"name": "read_file",
			"parameters": {"type": "object", "properties": {"id": {"const": 9007199254740993}}}
		}],
		"input": [
			{"type": "message", "role": "developer", "content": "Be concise."},
			{"type": "message", "role": "user", "content": [{"type": "input_text", "text": "Inspect", "prompt_cache_breakpoint": {"mode": "explicit", "ttl": "30m"}}]},
			{"type": "reasoning", "id": "rs_1", "summary": [], "content": [{"type": "reasoning_text", "text": "Thinking trace"}]},
			{"type": "function_call", "id": "fc_1", "call_id": "call_1", "name": "read_file", "arguments": "{\"id\":9007199254740993}"},
			{"type": "function_call_output", "call_id": "call_1", "output": "ok"}
		]
	}`)
	respCanonical, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIResponses, responsesInput)
	require.NoError(t, err)
	crossResponsesOut, err := EncodeCanonicalRequest(respCanonical, qwenCompatTarget)
	require.NoError(t, err)

	// Track 2b: Cross-wire openai_responses carrying Bailian cache_control -> Qwen OpenAI-compatible
	responsesWithCC := bytes.Replace(responsesInput, []byte(`"prompt_cache_breakpoint": {"mode": "explicit", "ttl": "30m"}`), []byte(`"cache_control": {"type": "ephemeral", "ttl": "5m"}`), 1)
	respCCCanonical, err := DecodeCanonicalRequest(canonical.ProtocolOpenAIResponses, responsesWithCC)
	require.NoError(t, err)
	crossResponsesCCOut, err := EncodeCanonicalRequest(respCCCanonical, qwenCompatTarget)
	require.NoError(t, err)

	// Track 3: Cross-wire anthropic_messages -> Qwen OpenAI-compatible (EncodeCanonicalRequest)
	messagesInput := []byte(`{
		"model": "public-qwen",
		"stream": true,
		"max_tokens": 1800,
		"thinking": {"type": "adaptive"},
		"output_config": {"effort": "medium"},
		"system": "Be concise.",
		"tools": [{
			"name": "read_file",
			"cache_control": {"type": "ephemeral"},
			"input_schema": {"type": "object", "properties": {"id": {"const": 9007199254740993}}}
		}],
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "Inspect", "cache_control": {"type": "ephemeral", "ttl": "5m"}}]},
			{"role": "assistant", "content": [
				{"type": "thinking", "thinking": "Thinking trace", "signature": "anth_sig_1"},
				{"type": "tool_use", "id": "call_1", "name": "read_file", "input": {"id": 9007199254740993}, "cache_control": {"type": "ephemeral"}}
			]},
			{"role": "user", "content": [
				{"type": "tool_result", "tool_use_id": "call_1", "content": "ok"}
			]}
		]
	}`)
	msgCanonical, err := DecodeCanonicalRequest(canonical.ProtocolAnthropicMessages, messagesInput)
	require.NoError(t, err)
	crossMessagesOut, err := EncodeCanonicalRequest(msgCanonical, qwenCompatTarget)
	require.NoError(t, err)

	for _, track := range []struct {
		name                  string
		body                  []byte
		expectEphemeralMarker bool
	}{
		{"same_wire_chat", sameWireOut, true},
		{"cross_wire_responses_prompt_cache_bp", crossResponsesOut, false},
		{"cross_wire_responses_cache_control", crossResponsesCCOut, true},
		{"cross_wire_messages", crossMessagesOut, true},
	} {
		t.Run(track.name, func(t *testing.T) {
			out := track.body
			assert.Equal(t, mappedModel, gjson.GetBytes(out, "model").String())
			assert.True(t, gjson.GetBytes(out, "stream").Bool())
			assert.True(t, gjson.GetBytes(out, "stream_options.include_usage").Bool())
			assert.True(t, gjson.GetBytes(out, "preserve_thinking").Bool(), "top-level preserve_thinking=true must match across tracks")
			assert.Equal(t, "medium", gjson.GetBytes(out, "reasoning_effort").String())

			for _, strippedTop := range []string{"store", "prompt_cache_key", "prompt_cache_retention", "prompt_cache_options"} {
				assert.False(t, gjson.GetBytes(out, strippedTop).Exists(), "%s must be stripped", strippedTop)
			}

			// Outer cache_control stripped on tools[] and messages[]
			assert.False(t, gjson.GetBytes(out, "tools.0.cache_control").Exists())
			assert.False(t, gjson.GetBytes(out, "messages.2.cache_control").Exists())
			assert.False(t, gjson.GetBytes(out, "messages.2.tool_calls.0.extra_content").Exists())
			assert.NotContains(t, string(out), "prompt_cache_breakpoint")

			if track.expectEphemeralMarker {
				// H3: messages[].content[].cache_control MUST survive on same-wire Chat and cross-wire Messages (enabling s.cacheMode == explicit)!
				assert.Equal(t, "ephemeral", gjson.GetBytes(out, "messages.1.content.0.cache_control.type").String())
				assert.Equal(t, "5m", gjson.GetBytes(out, "messages.1.content.0.cache_control.ttl").String())
				assert.True(t, bytes.Contains(out, []byte(`"cache_control"`)))
			} else {
				assert.False(t, bytes.Contains(out, []byte(`"cache_control"`)), "OA08/DS04: OpenAI prompt_cache_breakpoint must not forge ephemeral cache_control")
			}

			// Core message normalization parity
			assert.Equal(t, "system", gjson.GetBytes(out, "messages.0.role").String())
			assert.Equal(t, "Be concise.", gjson.GetBytes(out, "messages.0.content").String())
			assert.Equal(t, gjson.String, gjson.GetBytes(out, "messages.2.content").Type)
			assert.Equal(t, "", gjson.GetBytes(out, "messages.2.content").String())
			assert.Equal(t, "Thinking trace", gjson.GetBytes(out, "messages.2.reasoning_content").String())
			assert.Equal(t, `{"id":9007199254740993}`, gjson.GetBytes(out, "messages.2.tool_calls.0.function.arguments").String())
			assert.Equal(t, "tool", gjson.GetBytes(out, "messages.3.role").String())
			assert.False(t, gjson.GetBytes(out, "messages.3.name").Exists())
			assert.Equal(t, "ok", gjson.GetBytes(out, "messages.3.content").String())
			assert.Contains(t, gjson.GetBytes(out, "tools.0.function.parameters").Raw, "9007199254740993")
		})
	}
}
