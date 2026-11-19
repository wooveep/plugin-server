package fixedtools

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/canonical"
)

func TestInspectCanonicalMatchesByteInspectAndIsolatesClientFunctions(t *testing.T) {
	// Client function named "web_search" + explicit EnableSearch=false over default WebSearch -> empty set.
	req := canonical.Request{
		SourceProtocol: canonical.ProtocolOpenAIChat,
		Tools: []canonical.ToolDefinition{
			{
				Kind:   canonical.ToolKindFunction,
				Name:   "web_search",
				Schema: json.RawMessage(`{"type":"object"}`),
			},
			{
				Kind:   canonical.ToolKindFunction,
				Name:   "code_interpreter",
				Schema: json.RawMessage(`{"type":"object"}`),
			},
		},
		FixedTools: canonical.FixedToolControls{
			EnableSearch: canonical.Some(false),
		},
	}
	got, err := InspectCanonical(req, []Tool{WebSearch})
	if err != nil || len(got) != 0 {
		t.Fatalf("expected client functions and explicit false not to request fixed tools, got %v err=%v", got, err)
	}

	// EnableSearch=true + agent_max strategy + fixed server tools -> WebSearch, WebExtractor, CodeInterpreter.
	reqWithFixed := canonical.Request{
		SourceProtocol: canonical.ProtocolOpenAIResponses,
		Tools: []canonical.ToolDefinition{
			{Kind: canonical.ToolKindFixedServer, FixedType: "code_interpreter"},
			{Kind: canonical.ToolKindFixedServer, FixedType: "web_fetch_20250910", Name: "web_fetch"},
		},
		FixedTools: canonical.FixedToolControls{
			EnableSearch:     canonical.Some(true),
			SearchOptionsRaw: json.RawMessage(`{"strategy":"agent_max","forced_search":true}`),
		},
	}
	got, err = InspectCanonical(reqWithFixed, nil)
	if err != nil || !got[WebSearch] || !got[WebExtractor] || !got[CodeInterpreter] {
		t.Fatalf("expected WebSearch, WebExtractor, and CodeInterpreter, got %v err=%v", got, err)
	}

	// Explicit null enable_search must error identically to byte Inspect;
	// null search_options / null strategy is a benign no-op matching request.go:62.
	if _, err := InspectCanonical(canonical.Request{
		FixedTools: canonical.FixedToolControls{EnableSearch: canonical.NullOf[bool]()},
	}, nil); err == nil {
		t.Fatal("expected error for null enable_search")
	}
	nullSearchOpts, err := InspectCanonical(canonical.Request{
		FixedTools: canonical.FixedToolControls{
			EnableSearch:     canonical.Some(true),
			SearchStrategy:   canonical.NullOf[string](),
			SearchOptionsRaw: json.RawMessage(`null`),
		},
	}, nil)
	if err != nil || !nullSearchOpts[WebSearch] || nullSearchOpts[WebExtractor] {
		t.Fatalf("expected null search_options/strategy to be tolerated as WebSearch only, got %v err=%v", nullSearchOpts, err)
	}
	if _, err := InspectCanonical(canonical.Request{
		FixedTools: canonical.FixedToolControls{
			EnableSearch:     canonical.Some(true),
			SearchOptionsRaw: json.RawMessage(`123`),
		},
	}, nil); err == nil {
		t.Fatal("expected error for non-object search_options")
	}
}

func TestFilterCanonicalImmutabilityExplicitFalseAndDanglingChoices(t *testing.T) {
	base := canonical.Request{
		SourceProtocol: canonical.ProtocolOpenAIResponses,
		Model:          "public-model",
		Tools: []canonical.ToolDefinition{
			{Kind: canonical.ToolKindFixedServer, FixedType: "web_search", RawFixed: json.RawMessage(`{"type":"web_search"}`)},
			{Kind: canonical.ToolKindFixedServer, FixedType: "code_interpreter", RawFixed: json.RawMessage(`{"type":"code_interpreter"}`)},
			{Kind: canonical.ToolKindFunction, Name: "code_interpreter", Schema: json.RawMessage(`{"type":"object"}`)},
		},
		ToolChoice: canonical.ToolChoice{
			Mode:     canonical.ToolChoiceFixed,
			FixedRaw: "code_interpreter",
		},
		FixedTools: canonical.FixedToolControls{
			SearchStrategy:      canonical.Some("agent_max"),
			SearchOptionsRaw:    json.RawMessage(`{"strategy":"agent_max","freshness":"day"}`),
			WebSearchOptionsRaw: json.RawMessage(`{"search_context_size":"high"}`),
		},
	}

	// Removing WebSearch sets EnableSearch=false and clears WebSearchOptionsRaw.
	noSearch := FilterCanonical(base, []Tool{WebSearch}, map[Tool]bool{WebSearch: true})
	if !noSearch.FixedTools.EnableSearch.IsPresent() || noSearch.FixedTools.EnableSearch.Val || noSearch.FixedTools.WebSearchOptionsRaw != nil {
		t.Fatalf("expected removed WebSearch to set EnableSearch=false and nil WebSearchOptionsRaw, got %+v", noSearch.FixedTools)
	}

	// Attempt 1: remove CodeInterpreter & WebExtractor, default WebSearch=true.
	attempt1 := FilterCanonical(base, []Tool{WebSearch, CodeInterpreter}, map[Tool]bool{
		CodeInterpreter: true,
		WebExtractor:    true,
	})
	if !attempt1.FixedTools.EnableSearch.IsPresent() || !attempt1.FixedTools.EnableSearch.Val {
		t.Fatalf("expected default WebSearch to materialize as Some(true), got %+v", attempt1.FixedTools.EnableSearch)
	}
	if !attempt1.FixedTools.EnableCodeInterpreter.IsPresent() || attempt1.FixedTools.EnableCodeInterpreter.Val {
		t.Fatalf("expected removed CodeInterpreter to write explicit Some(false), got %+v", attempt1.FixedTools.EnableCodeInterpreter)
	}
	if attempt1.FixedTools.SearchStrategy.IsPresent() {
		t.Fatalf("expected agent_max SearchStrategy to be cleared, got %+v", attempt1.FixedTools.SearchStrategy)
	}
	var opts map[string]json.RawMessage
	if err := json.Unmarshal(attempt1.FixedTools.SearchOptionsRaw, &opts); err != nil || opts["strategy"] != nil || string(opts["freshness"]) != `"day"` {
		t.Fatalf("expected strategy removed while preserving freshness, got %s", string(attempt1.FixedTools.SearchOptionsRaw))
	}
	if len(attempt1.Tools) != 2 || attempt1.Tools[0].Kind != canonical.ToolKindFixedServer || attempt1.Tools[1].Kind != canonical.ToolKindFunction || attempt1.Tools[1].Name != "code_interpreter" {
		t.Fatalf("expected web_search server tool + code_interpreter client function to survive, got %+v", attempt1.Tools)
	}
	if attempt1.ToolChoice.Mode != canonical.ToolChoiceUnset {
		t.Fatalf("expected dangling fixed tool_choice to be cleared, got %+v", attempt1.ToolChoice)
	}

	// Base request must remain completely untouched for Attempt 2 fallback target!
	if len(base.Tools) != 3 || base.ToolChoice.Mode != canonical.ToolChoiceFixed || !base.FixedTools.SearchStrategy.IsPresent() {
		t.Fatalf("FilterCanonical mutated input base Request: %+v", base)
	}

	// Named client function choice must survive when同-named server tool is removed.
	namedFuncReq := base.CloneForAttempt()
	namedFuncReq.ToolChoice = canonical.ToolChoice{Mode: canonical.ToolChoiceFunction, Name: "code_interpreter"}
	keptChoice := FilterCanonical(namedFuncReq, nil, map[Tool]bool{CodeInterpreter: true})
	if keptChoice.ToolChoice.Mode != canonical.ToolChoiceFunction || keptChoice.ToolChoice.Name != "code_interpreter" {
		t.Fatalf("expected client function tool_choice to survive server tool removal, got %+v", keptChoice.ToolChoice)
	}

	// Required/any and Anthropic named server tool choice must clear when sole server tool is removed.
	for _, tc := range []canonical.ToolChoice{
		{Mode: canonical.ToolChoiceRequired},
		{Mode: canonical.ToolChoiceAny},
		{Mode: canonical.ToolChoiceFunction, Name: "web_search"},
	} {
		soleServer := canonical.Request{
			SourceProtocol: canonical.ProtocolAnthropicMessages,
			Tools: []canonical.ToolDefinition{
				{Kind: canonical.ToolKindFixedServer, FixedType: "web_search_20250305", Name: "web_search"},
			},
			ToolChoice: tc,
		}
		filtered := FilterCanonical(soleServer, nil, map[Tool]bool{WebSearch: true})
		if len(filtered.Tools) != 0 || filtered.ToolChoice.Mode != canonical.ToolChoiceUnset {
			t.Fatalf("expected empty tools and cleared tool_choice for %+v, got tools=%+v choice=%+v", tc, filtered.Tools, filtered.ToolChoice)
		}
	}
}
