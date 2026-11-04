package clientresponse

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestNativeContentBlocksHideDiagnosticsAndKeepFunctionalPayload(t *testing.T) {
	for _, tc := range []struct{ protocol, field, content string }{
		{Responses, "output", `[{"id":"raw-item","type":"message","role":"assistant","provider_diagnostic":"supplier-brand","content":[{"type":"output_text","text":"real text","annotations":[],"provider_diagnostic":"supplier-brand"}]},{"id":"raw-reasoning","type":"reasoning","encrypted_content":"opaque-signature","summary":[],"provider_diagnostic":"supplier-brand"},{"id":"raw-function","type":"function_call","call_id":"raw-call","name":"lookup","arguments":"{\"provider_diagnostic\":\"customer argument\"}","provider_diagnostic":"supplier-brand"}]`},
		{Messages, "content", `[{"type":"text","text":"real text","provider_diagnostic":"supplier-brand"},{"type":"thinking","thinking":"real thought","signature":"opaque-signature","provider_diagnostic":"supplier-brand"},{"type":"tool_use","id":"raw-call","name":"lookup","input":{"provider_diagnostic":"customer argument"},"provider_diagnostic":"supplier-brand"}]`},
	} {
		for _, stream := range []bool{false, true} {
			body := []byte(`{"id":"raw-root","model":"upstream","object":"response","status":"completed","` + tc.field + `":` + tc.content + `}`)
			if stream {
				kind, key := "response.completed", "response"
				if tc.protocol == Messages {
					kind, key = "message_start", "message"
				}
				event, _ := json.Marshal(map[string]json.RawMessage{"type": json.RawMessage(`"` + kind + `"`), key: body})
				body = []byte("event: " + kind + "\ndata: " + string(event) + "\n\n")
			}
			out, err := Project(Policy{Protocol: tc.protocol, PublicModel: "sales"}, body, stream, Usage{})
			if err != nil || bytes.Contains(out, []byte("supplier-brand")) || !bytes.Contains(out, []byte("real text")) || !bytes.Contains(out, []byte("opaque-signature")) || !bytes.Contains(out, []byte("customer argument")) {
				t.Fatalf("block privacy or functional payload changed (%s %v): %s %v", tc.protocol, stream, out, err)
			}
			// A second projection represents the same public boundary used by a
			// cache replay: diagnostics stay absent, functional payload remains.
			replay, err := Project(Policy{Protocol: tc.protocol, PublicModel: "sales"}, out, stream, Usage{})
			if err != nil || bytes.Contains(replay, []byte("supplier-brand")) || !bytes.Contains(replay, []byte("opaque-signature")) {
				t.Fatal("replay boundary changed content privacy", err)
			}
		}
	}
}

func TestChatToolAndContinuationBlocksHideOnlyDiagnosticSiblings(t *testing.T) {
	raw := []byte(`{"id":"raw","model":"actual","choices":[{"index":0,"message":{"role":"assistant","content":"answer","reasoning_content":"real thought","tool_calls":[{"id":"call","type":"function","provider_diagnostic":"supplier-brand","function":{"name":"lookup","arguments":"{\"provider_diagnostic\":\"customer argument\"}","provider_diagnostic":"supplier-brand"}}],"claude_content_blocks":[{"type":"thinking","thinking":"real thought","signature":"opaque-signature","provider_diagnostic":"supplier-brand"}]},"finish_reason":"tool_calls"}]}`)
	body, err := Project(Policy{Protocol: Chat, PublicModel: "sales"}, raw, false, Usage{})
	if err != nil || bytes.Contains(body, []byte("supplier-brand")) || !bytes.Contains(body, []byte("opaque-signature")) || !bytes.Contains(body, []byte("customer argument")) {
		t.Fatal("diagnostic siblings escaped or functional payload was lost", string(body), err)
	}
}

func TestRequestedChatUsageSurvivesExplicitStreamFailure(t *testing.T) {
	raw := []byte("data: {\"id\":\"raw\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial\"},\"finish_reason\":null}],\"usage\":{\"prompt_tokens\":17,\"completion_tokens\":5,\"total_tokens\":22}}\n\nevent: error\ndata: {\"error\":{\"code\":\"server_error\",\"message\":\"private diagnostic\"}}\n\n")
	body, err := Project(Policy{Protocol: Chat, PublicModel: "sales", IncludeUsage: true}, raw, true, ReadUsage(raw, Chat, true))
	if err != nil || !bytes.Contains(body, []byte(`"prompt_tokens":17`)) || bytes.Contains(body, []byte("[DONE]")) || bytes.Contains(body, []byte("private diagnostic")) {
		t.Fatal("partial usage or failure terminal changed", string(body), err)
	}
}
