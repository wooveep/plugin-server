package canonical

import (
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestProtocolAndImageDetailValidation(t *testing.T) {
	validProtocols := []Protocol{
		ProtocolOpenAIChat,
		ProtocolOpenAIResponses,
		ProtocolAnthropicMessages,
		ProtocolDashScopeText,
		ProtocolDashScopeMultimodal,
		ProtocolGeminiGenerate,
	}
	for _, p := range validProtocols {
		if !p.Valid() {
			t.Fatalf("expected protocol %q to be valid", p)
		}
	}
	if Protocol("unknown_wire").Valid() || Protocol("").Valid() {
		t.Fatal("expected unknown/empty protocol to be invalid")
	}
	if !ProtocolOpenAIChat.IsClientProtocol() || !ProtocolOpenAIResponses.IsClientProtocol() || !ProtocolAnthropicMessages.IsClientProtocol() {
		t.Fatal("expected chat, responses, and messages to be client protocols")
	}
	if ProtocolDashScopeText.IsClientProtocol() || ProtocolDashScopeMultimodal.IsClientProtocol() || ProtocolGeminiGenerate.IsClientProtocol() {
		t.Fatal("expected supplier wire protocols not to be client protocols")
	}

	for _, d := range []ImageDetail{ImageDetailUnset, ImageDetailAuto, ImageDetailLow, ImageDetailHigh, ImageDetailOriginal} {
		if !d.Valid() {
			t.Fatalf("expected image detail %q to be valid", d)
		}
	}
	if ImageDetail("ultra").Valid() {
		t.Fatal("expected unknown image detail to be invalid")
	}
}

func TestOptionalTriStateSemantics(t *testing.T) {
	absentFloat := AbsentOf[float64]()
	if !absentFloat.IsAbsent() || absentFloat.IsNull() || absentFloat.IsPresent() {
		t.Fatalf("unexpected state for AbsentOf: %+v", absentFloat)
	}
	if got := absentFloat.ValueOr(0.7); got != 0.7 {
		t.Fatalf("expected fallback 0.7, got %v", got)
	}

	nullFloat := NullOf[float64]()
	if nullFloat.IsAbsent() || !nullFloat.IsNull() || nullFloat.IsPresent() {
		t.Fatalf("unexpected state for NullOf: %+v", nullFloat)
	}
	if got := nullFloat.ValueOr(0.7); got != 0.7 {
		t.Fatalf("expected fallback 0.7 on null, got %v", got)
	}

	zeroFloat := Some[float64](0)
	if zeroFloat.IsAbsent() || zeroFloat.IsNull() || !zeroFloat.IsPresent() || zeroFloat.Val != 0 {
		t.Fatalf("expected explicit 0.0 to stay Present, got %+v", zeroFloat)
	}
	if got := zeroFloat.ValueOr(0.7); got != 0 {
		t.Fatalf("expected explicit 0.0 from ValueOr, got %v", got)
	}

	falseBool := Some(false)
	if !falseBool.IsPresent() || falseBool.ValueOr(true) != false {
		t.Fatalf("expected explicit false to stay Present, got %+v", falseBool)
	}

	emptySummary := Some([]string{})
	absentSummary := AbsentOf[[]string]()
	if !emptySummary.IsPresent() || emptySummary.Val == nil || len(emptySummary.Val) != 0 {
		t.Fatalf("expected explicit summary:[] to remain Present non-nil empty slice, got %+v", emptySummary)
	}
	if !absentSummary.IsAbsent() {
		t.Fatalf("expected omitted summary to remain Absent, got %+v", absentSummary)
	}
}

func TestRejectionErrorSanitization(t *testing.T) {
	clean := NewRejectionError("unsupported_stateful_reference", "previous_response_id", "previous_response_id is unsupported")
	if clean.Code != "unsupported_stateful_reference" || clean.Field != "previous_response_id" {
		t.Fatalf("unexpected sanitized error: %+v", clean)
	}
	if clean.SafeDetail() != "code=unsupported_stateful_reference field=previous_response_id" {
		t.Fatalf("unexpected SafeDetail: %q", clean.SafeDetail())
	}
	if clean.Error() != "previous_response_id is unsupported" {
		t.Fatalf("unexpected Error(): %q", clean.Error())
	}

	var err error = RejectionError{
		Code:  "leak:SECRET_PROMPT_VALUE",
		Field: "previous_response_id=\"resp_secret_123\"",
	}
	var rej RejectionError
	if !errors.As(err, &rej) {
		t.Fatal("expected errors.As to match RejectionError")
	}
	if strings.Contains(rej.SafeDetail(), "SECRET") || strings.Contains(rej.SafeDetail(), "resp_secret_123") {
		t.Fatalf("SafeDetail leaked secret value: %q", rej.SafeDetail())
	}
	if rej.SafeDetail() != "code=invalid_protocol_request" {
		t.Fatalf("expected fallback safe detail, got %q", rej.SafeDetail())
	}
	if strings.Contains(rej.Error(), "SECRET") || strings.Contains(rej.Error(), "resp_secret_123") {
		t.Fatalf("Error() leaked unsanitized field/code: %q", rej.Error())
	}

	tooLong := strings.Repeat("a", 65)
	if SafeIdentifier(tooLong) != "" {
		t.Fatal("expected >64 char identifier to be rejected")
	}
	if SafeIdentifier("input.messages.0.tool_call-id_1") != "input.messages.0.tool_call-id_1" {
		t.Fatal("expected dotted/dashed/underscored identifier to be accepted")
	}
}

func TestCloneForAttemptDeepIsolation(t *testing.T) {
	base := Request{
		SourceProtocol: ProtocolOpenAIResponses,
		Model:          "public-model",
		Stream:         true,
		IncludeUsage:   Some(false),
		Instructions:   Some("System instructions"),
		SystemParts: []Part{
			{
				Kind:        PartText,
				Text:        "sys-block",
				Annotations: []json.RawMessage{json.RawMessage(`{"type":"url_citation"}`)},
				Cache:       &CacheBoundary{Origin: CacheOriginEphemeralControl, Mode: "ephemeral", TTL: "5m"},
			},
		},
		SystemIsArray: true,
		Items: []Item{
			{
				Kind: ItemMessage,
				Role: RoleUser,
				Parts: []Part{
					{
						Kind:  PartImage,
						Image: &ImagePart{Source: ImageDataURL, URL: "data:image/png;base64,AAAA", MediaType: "image/png", Base64Ref: "AAAA", Detail: ImageDetailOriginal},
						Cache: &CacheBoundary{Origin: CacheOriginEphemeralControl, Mode: "ephemeral"},
					},
				},
				Cache: &CacheBoundary{Origin: CacheOriginOpenAIPromptCache, Mode: "ephemeral", TTL: "1h"},
			},
			{
				Kind: ItemReasoning,
				Reasoning: &ReasoningItem{
					ItemID:    "rs_1",
					Text:      []string{"step 1"},
					Summary:   Some([]string{"sum 1"}),
					Signature: "sig_1",
					Encrypted: json.RawMessage(`"enc_payload"`),
					Origin:    OpaqueOrigin{Protocol: ProtocolOpenAIResponses, Supplier: "openai"},
					Cache:     &CacheBoundary{Origin: CacheOriginEphemeralControl, Mode: "ephemeral"},
				},
			},
			{
				Kind: ItemFunctionCall,
				Call: &FunctionCall{
					ItemID: "fc_1",
					CallID: "call_1",
					Name:   "lookup",
					Args:   Arguments{Kind: ArgsObject, Object: json.RawMessage(`{"id":9007199254740993}`)},
				},
			},
			{
				Kind: ItemFunctionResult,
				Result: &FunctionResult{
					ItemID: "fco_1",
					CallID: "call_1",
					Name:   "lookup",
					Parts:  []Part{{Kind: PartText, Text: "ok"}},
					Cache:  &CacheBoundary{Origin: CacheOriginEphemeralControl, Mode: "ephemeral"},
				},
			},
		},
		Sampling: Sampling{
			Temperature: Some(0.0),
			Seed:        Some(int64(0)),
			Stop:        []string{"STOP_A"},
			StopPresent: true,
		},
		Limits: OutputLimit{
			Source: LimitMaxOutputTokens,
			Scope:  ScopeReasoningAndVisible,
			Value:  Some(1024),
		},
		ResponseFormat: ResponseFormat{
			Kind:   ResponseFormatJSONSchema,
			Name:   "answer",
			Schema: json.RawMessage(`{"type":"object"}`),
			Strict: Some(true),
		},
		Cache: CachePolicy{
			PromptCacheKey:  Some("pck_1"),
			RequestBoundary: &CacheBoundary{Origin: CacheOriginEphemeralControl, Mode: "ephemeral"},
		},
		Tools: []ToolDefinition{
			{
				Kind:   ToolKindFunction,
				Name:   "lookup",
				Schema: json.RawMessage(`{"type":"object","const":9007199254740993}`),
				Strict: Some(true),
				Cache:  &CacheBoundary{Origin: CacheOriginEphemeralControl, Mode: "ephemeral"},
			},
			{
				Kind:      ToolKindFixedServer,
				FixedType: "web_search",
				RawFixed:  json.RawMessage(`{"type":"web_search"}`),
			},
		},
		ToolChoice: ToolChoice{Mode: ToolChoiceAuto},
		FixedTools: FixedToolControls{
			EnableSearch:        Some(true),
			SearchOptionsRaw:    json.RawMessage(`{"strategy":"agent_max"}`),
			WebSearchOptionsRaw: json.RawMessage(`{"search_context_size":"medium"}`),
		},
		Store:    Some(false),
		Metadata: Some(map[string]string{"user_id": "u_1"}),
	}

	attempt := base.CloneForAttempt()
	attempt.Model = "qwen-plus"
	attempt.SystemParts[0].Text = "mutated"
	attempt.SystemParts[0].Annotations[0][0] = '['
	attempt.SystemParts[0].Cache.TTL = "1h"
	attempt.Items[0].Parts[0].Image.Detail = ImageDetailLow
	attempt.Items[0].Parts[0].Cache.Mode = "mutated"
	attempt.Items[0].Cache.TTL = "30m"
	attempt.Items[1].Reasoning.Text[0] = "mutated"
	attempt.Items[1].Reasoning.Summary.Val[0] = "mutated"
	attempt.Items[1].Reasoning.Encrypted[0] = 'X'
	attempt.Items[1].Reasoning.Cache.Mode = "mutated"
	attempt.Items[2].Call.Args.Object[0] = '['
	attempt.Items[3].Result.Parts[0].Text = "mutated"
	attempt.Items[3].Result.Cache.Mode = "mutated"
	attempt.Sampling.Stop[0] = "MUTATED"
	attempt.ResponseFormat.Schema[0] = '['
	attempt.Cache.RequestBoundary.Mode = "mutated"
	attempt.Tools[0].Schema[0] = '['
	attempt.Tools[0].Cache.Mode = "mutated"
	attempt.Tools[1].RawFixed[0] = '['
	attempt.Tools = attempt.Tools[:1]
	attempt.FixedTools.EnableSearch = Some(false)
	attempt.FixedTools.SearchOptionsRaw[0] = '['
	attempt.FixedTools.WebSearchOptionsRaw[0] = '['
	attempt.Metadata.Val["user_id"] = "mutated"
	attempt.Metadata.Val["injected"] = "yes"

	if base.Model != "public-model" {
		t.Fatalf("base Model mutated: %q", base.Model)
	}
	if base.SystemParts[0].Text != "sys-block" || string(base.SystemParts[0].Annotations[0]) != `{"type":"url_citation"}` || base.SystemParts[0].Cache.TTL != "5m" {
		t.Fatalf("base SystemParts mutated: %+v", base.SystemParts[0])
	}
	if base.Items[0].Parts[0].Image.Detail != ImageDetailOriginal || base.Items[0].Parts[0].Cache.Mode != "ephemeral" || base.Items[0].Cache.TTL != "1h" {
		t.Fatalf("base user Item mutated: %+v", base.Items[0])
	}
	if base.Items[1].Reasoning.Text[0] != "step 1" || base.Items[1].Reasoning.Summary.Val[0] != "sum 1" || string(base.Items[1].Reasoning.Encrypted) != `"enc_payload"` {
		t.Fatalf("base ReasoningItem mutated: %+v", base.Items[1].Reasoning)
	}
	if string(base.Items[2].Call.Args.Object) != `{"id":9007199254740993}` {
		t.Fatalf("base FunctionCall Args mutated: %s", string(base.Items[2].Call.Args.Object))
	}
	if base.Items[3].Result.Parts[0].Text != "ok" || base.Items[3].Result.Cache.Mode != "ephemeral" {
		t.Fatalf("base FunctionResult mutated: %+v", base.Items[3].Result)
	}
	if base.Sampling.Stop[0] != "STOP_A" {
		t.Fatalf("base Sampling.Stop mutated: %+v", base.Sampling.Stop)
	}
	if string(base.ResponseFormat.Schema) != `{"type":"object"}` {
		t.Fatalf("base ResponseFormat.Schema mutated: %s", string(base.ResponseFormat.Schema))
	}
	if base.Cache.RequestBoundary.Mode != "ephemeral" {
		t.Fatalf("base Cache.RequestBoundary mutated: %+v", base.Cache.RequestBoundary)
	}
	if len(base.Tools) != 2 || string(base.Tools[0].Schema) != `{"type":"object","const":9007199254740993}` || string(base.Tools[1].RawFixed) != `{"type":"web_search"}` {
		t.Fatalf("base Tools mutated: %+v", base.Tools)
	}
	if !base.FixedTools.EnableSearch.Val || string(base.FixedTools.SearchOptionsRaw) != `{"strategy":"agent_max"}` || string(base.FixedTools.WebSearchOptionsRaw) != `{"search_context_size":"medium"}` {
		t.Fatalf("base FixedTools mutated: %+v", base.FixedTools)
	}
	if len(base.Metadata.Val) != 1 || base.Metadata.Val["user_id"] != "u_1" {
		t.Fatalf("base Metadata mutated: %+v", base.Metadata.Val)
	}
}

func TestPackagePurityAndZeroDynamicAny(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	fset := token.NewFileSet()
	parsedFiles := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		parsedFiles++
		file, err := parser.ParseFile(fset, filepath.Clean(name), nil, parser.AllErrors)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, imp := range file.Imports {
			pkgPath := strings.Trim(imp.Path.Value, `"`)
			if strings.Contains(pkgPath, ".") || pkgPath == "unsafe" || strings.HasPrefix(pkgPath, "net/") {
				t.Fatalf("%s imports forbidden package %q; canonical must remain a pure stdlib leaf package", name, pkgPath)
			}
		}

		allowedTypeParamAny := map[*ast.Ident]bool{}
		ast.Inspect(file, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == "init" {
				t.Fatalf("%s declares forbidden init() function", name)
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
			return true
		})

		ast.Inspect(file, func(n ast.Node) bool {
			if _, ok := n.(*ast.InterfaceType); ok {
				t.Fatalf("%s contains forbidden interface{} type at %v", name, fset.Position(n.Pos()))
			}
			if ident, ok := n.(*ast.Ident); ok && ident.Name == "any" && !allowedTypeParamAny[ident] {
				t.Fatalf("%s contains forbidden dynamic 'any' usage at %v", name, fset.Position(ident.Pos()))
			}
			return true
		})
	}
	if parsedFiles == 0 {
		t.Fatal("expected non-zero production .go files in canonical")
	}

	rootTypes := []reflect.Type{
		reflect.TypeOf(Request{}),
		reflect.TypeOf(Response{}),
		reflect.TypeOf(Choice{}),
		reflect.TypeOf(FinishReason{}),
		reflect.TypeOf(Event{}),
	}
	visited := map[reflect.Type]bool{}
	var checkType func(rt reflect.Type)
	checkType = func(rt reflect.Type) {
		for rt.Kind() == reflect.Pointer || rt.Kind() == reflect.Slice {
			rt = rt.Elem()
		}
		if rt.Kind() == reflect.Interface {
			t.Fatalf("discovered forbidden interface field type %v", rt)
		}
		if rt.Kind() == reflect.Map {
			checkType(rt.Key())
			checkType(rt.Elem())
			return
		}
		if rt.Kind() != reflect.Struct || visited[rt] {
			return
		}
		visited[rt] = true
		for i := 0; i < rt.NumField(); i++ {
			field := rt.Field(i)
			lower := strings.ToLower(field.Name)
			if lower == "extras" || lower == "usage" || lower == "statuscode" || lower == "httpstatus" || lower == "credential" {
				t.Fatalf("struct %s contains forbidden domain-violating field %s", rt.Name(), field.Name)
			}
			checkType(field.Type)
		}
	}
	for _, rt := range rootTypes {
		checkType(rt)
	}
}
