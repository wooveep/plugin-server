package clientresponse

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/canonical"
	"github.com/tidwall/gjson"
)

func TestB02SaltedOriginKeysCrossTurnSeedAndTopLevelSkip(t *testing.T) {
	// Turn 1: upstream omits Response.ID, ItemID, and CallID (e.g. DashScope / Gemini).
	turn1Aliases := &ResourceAliases{}
	turn1Policy := Policy{
		Protocol:    Responses,
		Profile:     Qwen,
		PublicModel: "public-qwen",
		RequestID:   "call-req-turn-1",
		Aliases:     turn1Aliases,
	}
	turn1Resp := canonical.Response{
		ID:    "",
		Model: "qwen-plus",
		Choices: []canonical.Choice{
			{
				Index: 0,
				Items: []canonical.Item{
					{
						Kind:  canonical.ItemMessage,
						Role:  canonical.RoleAssistant,
						Parts: []canonical.Part{{Kind: canonical.PartText, Text: "Checking weather."}},
					},
					{
						Kind: canonical.ItemFunctionCall,
						Call: &canonical.FunctionCall{
							Name: "get_weather",
							Args: canonical.Arguments{Kind: canonical.ArgsText, Text: `{"city":"Beijing"}`},
						},
					},
				},
				Finish: canonical.FinishReason{Category: canonical.FinishToolCalls, RawReason: "tool_calls"},
			},
		},
	}

	proj1, err := ProjectCanonical(turn1Policy, turn1Resp, Usage{})
	if err != nil {
		t.Fatalf("ProjectCanonical turn 1 failed: %v", err)
	}
	if !strings.HasPrefix(proj1.PublicID, "resp_pf_") {
		t.Fatalf("expected minted resp_pf_ ID, got %q", proj1.PublicID)
	}
	msgItem1 := proj1.Choices[0].Items[0].PublicItemID
	callItem1 := proj1.Choices[0].Items[1].PublicItemID
	callID1 := proj1.Choices[0].Items[1].PublicCallID
	if !strings.HasPrefix(msgItem1, "item_pf_") || !strings.HasPrefix(callItem1, "item_pf_") || msgItem1 == callItem1 {
		t.Fatalf("expected distinct item_pf_ IDs, got msg=%q callItem=%q", msgItem1, callItem1)
	}
	if !strings.HasPrefix(callID1, "call_pf_") {
		t.Fatalf("expected minted call_pf_ ID, got %q", callID1)
	}

	// Simulate persistManagedResourceAliases (:76): skip Original == "" || Original == s.callID.
	var persisted []ResourceAlias
	for _, rec := range turn1Aliases.Records() {
		if rec.Original == "" || rec.Original == turn1Policy.RequestID {
			if rec.Kind != "response" {
				t.Fatalf("expected only top-level missing response ID to match bare RequestID, got skipped %+v", rec)
			}
			continue
		}
		persisted = append(persisted, rec)
	}
	if len(persisted) != 3 {
		t.Fatalf("expected 3 salted item/call records persisted for Turn 2 MGET, got %+v", persisted)
	}

	// Turn 2: restoreManagedResourceReferences (:183) seeds Turn 1 aliases into Turn 2's table.
	turn2Aliases := &ResourceAliases{}
	for _, rec := range persisted {
		turn2Aliases.Seed(rec)
	}
	turn2Policy := Policy{
		Protocol:    Responses,
		Profile:     Qwen,
		PublicModel: "public-qwen",
		RequestID:   "call-req-turn-2",
		Aliases:     turn2Aliases,
	}
	proj2, err := ProjectCanonical(turn2Policy, turn1Resp, Usage{})
	if err != nil {
		t.Fatalf("ProjectCanonical turn 2 failed: %v", err)
	}
	if proj2.Choices[0].Items[0].PublicItemID == msgItem1 ||
		proj2.Choices[0].Items[1].PublicItemID == callItem1 ||
		proj2.Choices[0].Items[1].PublicCallID == callID1 {
		t.Fatalf("Turn 2 output collided with Turn 1 seeded aliases! turn1=(%s,%s,%s) turn2=(%s,%s,%s)",
			msgItem1, callItem1, callID1,
			proj2.Choices[0].Items[0].PublicItemID,
			proj2.Choices[0].Items[1].PublicItemID,
			proj2.Choices[0].Items[1].PublicCallID)
	}
}

func TestCanonicalResponseProfilesRC04AndCitationSanitization(t *testing.T) {
	// A-P01: Messages + OpenAI must be rejected; empty profile defaults to DeepSeek.
	if _, err := ProjectCanonical(Policy{Protocol: Messages, Profile: OpenAI, PublicModel: "sale"}, canonical.Response{}, Usage{}); err == nil {
		t.Fatal("expected Messages + OpenAI profile to be rejected")
	}

	// RC-04: Refusal and length-incomplete Responses must still mint non-empty PublicItemID,
	// sanitize private annotation fields, and honor 8-profile usage presentation.
	rawResponsesUsage := []byte(`{"usage":{"input_tokens":100,"output_tokens":5,"total_tokens":105,"input_tokens_details":{"cached_tokens":20,"cache_write_tokens":30},"x_details":[{"input_tokens":100,"output_tokens":5,"total_tokens":105,"secret":"drop"}]}}`)
	usage := ReadUsage(rawResponsesUsage, Responses, false)

	resp := canonical.Response{
		ID:          "up_resp_1",
		CreatedAt:   1700000000,
		ServiceTier: "scale",
		Choices: []canonical.Choice{
			{
				Index: 0,
				Items: []canonical.Item{
					{
						Kind: canonical.ItemMessage,
						Role: canonical.RoleAssistant,
						Parts: []canonical.Part{
							{
								Kind:    canonical.PartRefusal,
								Refusal: "Cannot comply.",
								Annotations: []json.RawMessage{
									json.RawMessage(`{"type":"url_citation","url":"https://example.com","title":"Doc","supplier_debug":"leak"}`),
								},
							},
						},
					},
				},
				Finish:   canonical.FinishReason{Category: canonical.FinishLength, RawReason: "length"},
				Logprobs: json.RawMessage(`null`),
			},
		},
	}

	reqFields := RequestFields([]byte(`{"instructions":"Be brief","store":false,"supplier_internal":"drop"}`))
	projQwen, err := ProjectCanonical(Policy{
		Protocol:          Responses,
		Profile:           Qwen,
		PublicModel:       "public-model",
		RequestID:         "req-1",
		ExposeServiceTier: false,
		RequestFields:     reqFields,
	}, resp, usage)
	if err != nil {
		t.Fatalf("ProjectCanonical failed: %v", err)
	}
	if projQwen.ServiceTier != "" {
		t.Fatalf("expected ServiceTier hidden when ExposeServiceTier=false, got %q", projQwen.ServiceTier)
	}
	if projQwen.IncompleteReason != "max_output_tokens" {
		t.Fatalf("expected IncompleteReason=max_output_tokens, got %q", projQwen.IncompleteReason)
	}
	if projQwen.Choices[0].Logprobs != nil {
		t.Fatalf("expected null logprobs to normalize to nil, got %s", string(projQwen.Choices[0].Logprobs))
	}
	if !strings.HasPrefix(projQwen.Choices[0].Items[0].PublicItemID, "item_pf_") {
		t.Fatalf("RC-04 violated: refusal/incomplete item missing PublicItemID: %+v", projQwen.Choices[0].Items[0])
	}
	ann := string(projQwen.Choices[0].Items[0].Item.Parts[0].Annotations[0])
	if strings.Contains(ann, "supplier_debug") || !strings.Contains(ann, "https://example.com") {
		t.Fatalf("annotation sanitization failed: %s", ann)
	}
	if _, leaked := projQwen.EchoFields["supplier_internal"]; leaked || string(projQwen.EchoFields["store"]) != "false" {
		t.Fatalf("unexpected EchoFields: %+v", projQwen.EchoFields)
	}
	if !gjson.GetBytes(projQwen.WireUsage, "x_details").Exists() {
		t.Fatalf("expected Qwen Responses profile to expose x_details, got %s", string(projQwen.WireUsage))
	}

	projDeepSeek, err := ProjectCanonical(Policy{
		Protocol:    Responses,
		Profile:     "",
		PublicModel: "public-model",
	}, resp, usage)
	if err != nil {
		t.Fatalf("ProjectCanonical default profile failed: %v", err)
	}
	if projDeepSeek.Profile != DeepSeek || gjson.GetBytes(projDeepSeek.WireUsage, "x_details").Exists() {
		t.Fatalf("expected default DeepSeek profile without x_details, got profile=%q usage=%s", projDeepSeek.Profile, string(projDeepSeek.WireUsage))
	}

	// C04/C05: Qwen Chat profile exposes cache_creation_input_tokens / ephemeral_5m_input_tokens / cache_type, while OpenAI/DeepSeek hide them.
	qwenChatUsage := ReadUsage([]byte(`{"usage":{"prompt_tokens":100,"completion_tokens":5,"total_tokens":105,"prompt_tokens_details":{"cached_tokens":20,"cache_creation_input_tokens":30,"cache_creation":{"ephemeral_5m_input_tokens":30},"cache_type":"ephemeral","account":"secret"}}}`), Chat, false)
	projChatQwen, err := ProjectCanonical(Policy{Protocol: Chat, Profile: Qwen, PublicModel: "public-model"}, resp, qwenChatUsage)
	if err != nil {
		t.Fatalf("ProjectCanonical Chat Qwen failed: %v", err)
	}
	if gjson.GetBytes(projChatQwen.WireUsage, "prompt_tokens_details.cache_creation.ephemeral_5m_input_tokens").Int() != 30 ||
		strings.Contains(string(projChatQwen.WireUsage), "secret") {
		t.Fatalf("Qwen Chat C04/C05 profile failed: %s", string(projChatQwen.WireUsage))
	}
	projChatOpenAI, err := ProjectCanonical(Policy{Protocol: Chat, Profile: OpenAI, PublicModel: "public-model"}, resp, qwenChatUsage)
	if err != nil || gjson.GetBytes(projChatOpenAI.WireUsage, "prompt_tokens_details.cache_creation_input_tokens").Exists() {
		t.Fatalf("OpenAI Chat profile leaked Qwen cache_creation_input_tokens: %s err=%v", string(projChatOpenAI.WireUsage), err)
	}
}

func TestCanonicalStreamProjectionSlotStabilityM04AndFailContracts(t *testing.T) {
	// 1. Streaming parallel tool calls with omitted CallID on subsequent delta chunks.
	sp := NewCanonicalStreamProjection(Policy{
		Protocol:    Responses,
		Profile:     DeepSeek,
		PublicModel: "sale-model",
		RequestID:   "req-stream-1",
	})
	startEvs, err := sp.Project(canonical.Event{Kind: canonical.EventToolCallStart, ChoiceIndex: 0, ToolIndex: 0, CallID: "call_up_0", ToolName: "read_file"}, Usage{})
	if err != nil || len(startEvs) != 2 || startEvs[0].Event.Kind != canonical.EventResponseStart {
		t.Fatalf("expected synthesized EventResponseStart + EventToolCallStart, got %+v err=%v", startEvs, err)
	}
	deltaEvs, err := sp.Project(canonical.Event{Kind: canonical.EventToolCallArgsDelta, ChoiceIndex: 0, ToolIndex: 0, Text: `{"path":"a.go"}`}, Usage{})
	if err != nil || len(deltaEvs) != 1 {
		t.Fatalf("unexpected delta projection: %+v err=%v", deltaEvs, err)
	}
	stopEvs, err := sp.Project(canonical.Event{Kind: canonical.EventContentBlockStop, ChoiceIndex: 0, ToolIndex: 0}, Usage{})
	if err != nil || len(stopEvs) != 1 {
		t.Fatalf("unexpected block stop projection: %+v err=%v", stopEvs, err)
	}
	if startEvs[1].PublicItemID == "" || startEvs[1].PublicCallID == "" ||
		deltaEvs[0].PublicItemID != startEvs[1].PublicItemID || deltaEvs[0].PublicCallID != startEvs[1].PublicCallID ||
		stopEvs[0].PublicItemID != startEvs[1].PublicItemID || stopEvs[0].PublicCallID != startEvs[1].PublicCallID {
		t.Fatalf("slot IDs drifted across stream chunks: start=%+v delta=%+v stop=%+v", startEvs[1], deltaEvs[0], stopEvs[0])
	}

	// ChoiceFinish on Responses must hold terminal until explicit Finish().
	held, err := sp.Project(canonical.Event{
		Kind:   canonical.EventChoiceFinish,
		Finish: canonical.FinishReason{Category: canonical.FinishToolCalls, RawReason: "tool_calls"},
	}, Usage{})
	if err != nil || len(held) != 0 {
		t.Fatalf("expected Responses ChoiceFinish to hold terminal until Finish(), got %+v err=%v", held, err)
	}
	finEvs, err := sp.Finish(canonical.FinishReason{}, ReadUsage([]byte(`{"usage":{"prompt_tokens":12,"completion_tokens":4,"total_tokens":16}}`), Chat, false))
	if err != nil || len(finEvs) != 1 || finEvs[0].Finish.Category != canonical.FinishToolCalls || gjson.GetBytes(finEvs[0].FinalUsage, "total_tokens").Int() != 16 {
		t.Fatalf("unexpected Finish output: %+v err=%v", finEvs, err)
	}

	// 2. M04: Qwen Messages message_start trims cache counters, message_delta exposes full counters.
	msgUsage := ReadUsage([]byte(`{"usage":{"input_tokens":15,"output_tokens":8,"cache_read_input_tokens":3,"cache_creation_input_tokens":2}}`), Messages, false)
	msgProj := NewCanonicalStreamProjection(Policy{
		Protocol:    Messages,
		Profile:     Qwen,
		PublicModel: "qwen-sale",
		RequestID:   "req-msg-1",
	})
	mStart, err := msgProj.Project(canonical.Event{Kind: canonical.EventResponseStart, ID: "msg_up_1"}, msgUsage)
	if err != nil || len(mStart) != 1 {
		t.Fatalf("Messages start failed: %+v err=%v", mStart, err)
	}
	if gjson.GetBytes(mStart[0].StartUsage, "input_tokens").Int() != 15 ||
		gjson.GetBytes(mStart[0].StartUsage, "output_tokens").Int() != 0 ||
		gjson.GetBytes(mStart[0].StartUsage, "cache_read_input_tokens").Exists() ||
		gjson.GetBytes(mStart[0].StartUsage, "cache_creation_input_tokens").Exists() {
		t.Fatalf("M04 violated on message_start StartUsage: %s", string(mStart[0].StartUsage))
	}
	mFin, err := msgProj.Finish(canonical.FinishReason{Category: canonical.FinishStop, RawReason: "end_turn"}, msgUsage)
	if err != nil || len(mFin) != 1 {
		t.Fatalf("Messages finish failed: %+v err=%v", mFin, err)
	}
	if gjson.GetBytes(mFin[0].FinalUsage, "cache_read_input_tokens").Int() != 3 ||
		gjson.GetBytes(mFin[0].FinalUsage, "cache_creation_input_tokens").Int() != 2 ||
		gjson.GetBytes(mFin[0].FinalUsage, "output_tokens").Int() != 8 {
		t.Fatalf("M04 violated on message_delta FinalUsage: %s", string(mFin[0].FinalUsage))
	}

	// 3. E04 HTTP-only OpenAI overload type must NOT alter streaming Fail() SSE error payload.
	openaiStream := NewCanonicalStreamProjection(Policy{
		Protocol:    Responses,
		Profile:     OpenAI,
		PublicModel: "gpt-sale",
		RequestID:   "req-fail-1",
	})
	failEvs, err := openaiStream.Fail(Failure{Kind: FailureOverloaded, Retryable: true}, Usage{})
	if err != nil || len(failEvs) != 1 {
		t.Fatalf("Fail returned unexpected result: %+v err=%v", failEvs, err)
	}
	if gjson.GetBytes(failEvs[0].FailureBody, "code").String() != "server_is_overloaded" ||
		strings.Contains(string(failEvs[0].FailureBody), "service_unavailable_error") {
		t.Fatalf("HTTP-only E04 leaked into streaming SSE FailureBody: %s", string(failEvs[0].FailureBody))
	}
	if gjson.GetBytes(failEvs[0].Error, "code").String() != "server_is_overloaded" || gjson.GetBytes(failEvs[0].Error, "type").Exists() {
		t.Fatalf("Responses response.failed Error must retain only code and message: %s", string(failEvs[0].Error))
	}
}

func TestReadSupplierUsageGE02MoonshotQwenAndDeterministicEntropy(t *testing.T) {
	// 1. Gemini GE02 with >53-bit integer 9007199254740993 (exact big.Int, zero float64 truncation,
	// cachedContentTokenCount NOT double-added to promptTokenCount).
	geminiSSE := []byte(
		"data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"Hel\"}]}}],\"usageMetadata\":{\"promptTokenCount\":9007199254740993,\"cachedContentTokenCount\":1000}}\n\n" +
			"data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"lo\"}]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":9007199254740993,\"cachedContentTokenCount\":1000,\"toolUsePromptTokenCount\":7,\"candidatesTokenCount\":20,\"thoughtsTokenCount\":5,\"totalTokenCount\":9007199254741025}}\n\n",
	)
	gu := ReadSupplierUsage(geminiSSE, "gemini", true)
	chatWire := gu.wireProfile(Chat, DeepSeek)
	if gjson.GetBytes(chatWire, "prompt_tokens").Raw != "9007199254741000" ||
		gjson.GetBytes(chatWire, "completion_tokens").Raw != "25" ||
		gjson.GetBytes(chatWire, "total_tokens").Raw != "9007199254741025" ||
		gjson.GetBytes(chatWire, "prompt_tokens_details.cached_tokens").Raw != "1000" ||
		gjson.GetBytes(chatWire, "completion_tokens_details.reasoning_tokens").Raw != "5" {
		t.Fatalf("GE02 Gemini usage mismatch or precision lost: %s", string(chatWire))
	}

	// Conflicting Gemini totalTokenCount must not fabricate output via total - input.
	badGemini := ReadSupplierUsage([]byte(`{"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5,"totalTokenCount":99}}`), "gemini_generate_content", false)
	if badGemini.wireProfile(Chat, DeepSeek) != nil {
		t.Fatal("expected conflicting Gemini totalTokenCount to reject wire projection")
	}

	// Gemini SSE replaces usageMetadata snapshot per frame rather than merging stale fields.
	geminiSnapshotSSE := []byte(
		"data: {\"usageMetadata\":{\"promptTokenCount\":10,\"thoughtsTokenCount\":999,\"totalTokenCount\":1009}}\n\n" +
			"data: {\"usageMetadata\":{\"promptTokenCount\":10,\"candidatesTokenCount\":5,\"totalTokenCount\":15}}\n\n",
	)
	snapUsage := ReadSupplierUsage(geminiSnapshotSSE, "gemini", true)
	snapWire := snapUsage.wireProfile(Chat, DeepSeek)
	if gjson.GetBytes(snapWire, "prompt_tokens").Int() != 10 ||
		gjson.GetBytes(snapWire, "completion_tokens").Int() != 5 ||
		gjson.GetBytes(snapWire, "total_tokens").Int() != 15 ||
		gjson.GetBytes(snapWire, "completion_tokens_details.reasoning_tokens").Exists() {
		t.Fatalf("expected Gemini snapshot replacement without stale thoughtsTokenCount, got %s", string(snapWire))
	}

	// 2. Moonshot nested choices[0].usage in SSE with >53-bit integer.
	moonshotSSE := []byte(
		"data: {\"id\":\"cmpl-1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\",\"usage\":{\"prompt_tokens\":9007199254740993,\"completion_tokens\":2,\"total_tokens\":9007199254740995}}]}\n\n" +
			"data: [DONE]\n\n",
	)
	mu := ReadSupplierUsage(moonshotSSE, "moonshot", true)
	mWire := mu.wireProfile(Responses, DeepSeek)
	if gjson.GetBytes(mWire, "input_tokens").Raw != "9007199254740993" ||
		gjson.GetBytes(mWire, "output_tokens").Raw != "2" ||
		gjson.GetBytes(mWire, "total_tokens").Raw != "9007199254740995" {
		t.Fatalf("Moonshot nested choices[0].usage failed: %s", string(mWire))
	}

	// 3. DashScope / Qwen cumulative SSE usage with Qwen profile text_tokens.
	qwenSSE := []byte(
		"id:1\nevent:result\ndata: {\"output\":{\"choices\":[{\"message\":{\"content\":\"a\"},\"finish_reason\":\"null\"}]},\"usage\":{\"input_tokens\":50,\"output_tokens\":1,\"total_tokens\":51}}\n\n" +
			"id:2\nevent:result\ndata: {\"output\":{\"choices\":[{\"message\":{\"content\":\"b\"},\"finish_reason\":\"stop\"}]},\"usage\":{\"input_tokens\":50,\"output_tokens\":6,\"total_tokens\":56,\"prompt_tokens_details\":{\"cached_tokens\":10},\"output_tokens_details\":{\"text_tokens\":4,\"reasoning_tokens\":2}}}\n\n",
	)
	qu := ReadSupplierUsage(qwenSSE, "dashscope_text", true)
	qWire := qu.wireProfile(Chat, Qwen)
	if gjson.GetBytes(qWire, "prompt_tokens").Int() != 50 ||
		gjson.GetBytes(qWire, "completion_tokens").Int() != 6 ||
		gjson.GetBytes(qWire, "prompt_tokens_details.cached_tokens").Int() != 10 ||
		gjson.GetBytes(qWire, "completion_tokens_details.text_tokens").Int() != 4 ||
		gjson.GetBytes(qWire, "completion_tokens_details.reasoning_tokens").Int() != 2 {
		t.Fatalf("DashScope supplier usage failed: %s", string(qWire))
	}

	// 4. Deterministic entropy injection for benchmarks (B07).
	restore := SetIdentityEntropySource(bytes.NewReader(bytes.Repeat([]byte{0xab}, 64)))
	defer restore()
	detResp, err := ProjectCanonical(Policy{Protocol: Chat, PublicModel: "m", RequestID: "r1"}, canonical.Response{}, Usage{})
	if err != nil || detResp.PublicID != "chatcmpl_pf_"+strings.Repeat("ab", 16) {
		t.Fatalf("unexpected deterministic ID: %q err=%v", detResp.PublicID, err)
	}
}

func TestContentBlockStopMultiItemAndParallelToolRouting(t *testing.T) {
	// Scenario A: Parallel tool calls sharing default ItemIndex=0 (ToolIndex=0 and ToolIndex=1).
	spA := NewCanonicalStreamProjection(Policy{
		Protocol:    Responses,
		Profile:     DeepSeek,
		PublicModel: "sale-model",
		RequestID:   "req-scen-a",
	})
	t0Start, err := spA.Project(canonical.Event{Kind: canonical.EventToolCallStart, ChoiceIndex: 0, ItemIndex: 0, ToolIndex: 0, ToolName: "fn_a"}, Usage{})
	if err != nil {
		t.Fatal(err)
	}
	t1Start, err := spA.Project(canonical.Event{Kind: canonical.EventToolCallStart, ChoiceIndex: 0, ItemIndex: 0, ToolIndex: 1, ToolName: "fn_b"}, Usage{})
	if err != nil {
		t.Fatal(err)
	}
	t0Stop, err := spA.Project(canonical.Event{Kind: canonical.EventContentBlockStop, ChoiceIndex: 0, ItemIndex: 0, ToolIndex: 0}, Usage{})
	if err != nil {
		t.Fatal(err)
	}
	t1Stop, err := spA.Project(canonical.Event{Kind: canonical.EventContentBlockStop, ChoiceIndex: 0, ItemIndex: 0, ToolIndex: 1}, Usage{})
	if err != nil {
		t.Fatal(err)
	}
	// t0Start has synthesized EventResponseStart at [0] and ToolCallStart at [1].
	if t0Start[1].PublicItemID == t1Start[0].PublicItemID || t0Start[1].PublicCallID == t1Start[0].PublicCallID {
		t.Fatalf("Scenario A parallel tools collided: t0=%+v t1=%+v", t0Start[1], t1Start[0])
	}
	if t0Stop[0].PublicItemID != t0Start[1].PublicItemID || t0Stop[0].PublicCallID != t0Start[1].PublicCallID {
		t.Fatalf("Scenario A tool 0 stop mismatch: start=%+v stop=%+v", t0Start[1], t0Stop[0])
	}
	if t1Stop[0].PublicItemID != t1Start[0].PublicItemID || t1Stop[0].PublicCallID != t1Start[0].PublicCallID {
		t.Fatalf("Scenario A tool 1 stop mismatch: start=%+v stop=%+v", t1Start[0], t1Stop[0])
	}

	// Scenario B: Text at ItemIndex=0 followed by tool call reusing ItemIndex=0, ToolIndex=0
	// (both interleaved with prior text stop AND unclosed text before tool stop).
	for _, closeTextFirst := range []bool{true, false} {
		spB := NewCanonicalStreamProjection(Policy{
			Protocol:    Responses,
			Profile:     DeepSeek,
			PublicModel: "sale-model",
			RequestID:   "req-scen-b",
		})
		txtEvs, err := spB.Project(canonical.Event{Kind: canonical.EventTextDelta, ChoiceIndex: 0, ItemIndex: 0, Text: "Let me check."}, Usage{})
		if err != nil {
			t.Fatal(err)
		}
		textItemID := txtEvs[1].PublicItemID
		if closeTextFirst {
			txtStop, err := spB.Project(canonical.Event{Kind: canonical.EventContentBlockStop, ChoiceIndex: 0, ItemIndex: 0}, Usage{})
			if err != nil || txtStop[0].PublicItemID != textItemID || txtStop[0].PublicCallID != "" {
				t.Fatalf("Scenario B text stop mismatch: %+v err=%v", txtStop, err)
			}
		}
		toolStart, err := spB.Project(canonical.Event{Kind: canonical.EventToolCallStart, ChoiceIndex: 0, ItemIndex: 0, ToolIndex: 0, ToolName: "lookup"}, Usage{})
		if err != nil {
			t.Fatal(err)
		}
		if toolStart[0].PublicItemID == "" || toolStart[0].PublicItemID == textItemID || toolStart[0].PublicCallID == "" {
			t.Fatalf("Scenario B tool collided with text item: text=%q tool=%+v", textItemID, toolStart[0])
		}
		// Bare EventContentBlockStop without CallID/ToolName must close the open tool slot!
		toolStop, err := spB.Project(canonical.Event{Kind: canonical.EventContentBlockStop, ChoiceIndex: 0, ItemIndex: 0, ToolIndex: 0}, Usage{})
		if err != nil || toolStop[0].PublicItemID != toolStart[0].PublicItemID || toolStop[0].PublicCallID != toolStart[0].PublicCallID {
			t.Fatalf("Scenario B (closeTextFirst=%v) tool stop got text ID instead of tool ID: want=(%s,%s) got=(%s,%s)",
				closeTextFirst, toolStart[0].PublicItemID, toolStart[0].PublicCallID, toolStop[0].PublicItemID, toolStop[0].PublicCallID)
		}
	}

	// Scenario C: Anthropic Messages with thinking at ItemIndex=0 and tool_use at ItemIndex=1, ToolIndex=0.
	spC := NewCanonicalStreamProjection(Policy{
		Protocol:    Messages,
		Profile:     DeepSeek,
		PublicModel: "claude-sale",
		RequestID:   "req-scen-c",
	})
	if _, err := spC.Project(canonical.Event{Kind: canonical.EventReasoningDelta, ChoiceIndex: 0, ItemIndex: 0, Text: "step 1"}, Usage{}); err != nil {
		t.Fatal(err)
	}
	cToolStart, err := spC.Project(canonical.Event{Kind: canonical.EventToolCallStart, ChoiceIndex: 0, ItemIndex: 1, ToolIndex: 0, CallID: "toolu_1", ToolName: "bash"}, Usage{})
	if err != nil {
		t.Fatal(err)
	}
	// Closing thinking block 0 (where ToolIndex defaults to 0) must NOT attach tool_use's PublicCallID!
	thinkStop, err := spC.Project(canonical.Event{Kind: canonical.EventContentBlockStop, ChoiceIndex: 0, ItemIndex: 0, ToolIndex: 0}, Usage{})
	if err != nil || thinkStop[0].PublicItemID != "" || thinkStop[0].PublicCallID != "" {
		t.Fatalf("Scenario C thinking block 0 stop leaked tool_use ID: %+v err=%v", thinkStop[0], err)
	}
	toolUseStop, err := spC.Project(canonical.Event{Kind: canonical.EventContentBlockStop, ChoiceIndex: 0, ItemIndex: 1, ToolIndex: 0}, Usage{})
	if err != nil || toolUseStop[0].PublicCallID != cToolStart[0].PublicCallID || toolUseStop[0].PublicItemID != cToolStart[0].PublicItemID {
		t.Fatalf("Scenario C tool_use block 1 stop mismatch: start=%+v stop=%+v", cToolStart[0], toolUseStop[0])
	}
}
