package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
)

func TestManagedDashScopePreservesNativeBodiesAndCumulativeUsage(t *testing.T) {
	for _, protocol := range []string{managedProtocolDashScopeText, managedProtocolDashScopeMultimodal} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", protocol, stream), func(t *testing.T) {
				fixture := managedConfigFixture()
				// The same supplier connection can serve compatible and native bindings.
				fixture["provider"] = json.RawMessage(`{"type":"qwen","apiTokens":["controlled-upstream-key"],"qwenEnableCompatible":true,"qwenEnableSearch":true}`)
				settings := fixture["managed_execution"].(managedSettings)
				settings.InputProtocol, settings.UpstreamProtocol = protocol, protocol
				settings.UpstreamPath = "/api/v1/services/aigc/text-generation/generation"
				if protocol == managedProtocolDashScopeMultimodal {
					settings.UpstreamPath = "/api/v1/services/aigc/multimodal-generation/generation"
				}
				fixture["managed_execution"] = settings
				h := newManagedHost(t, fixture)
				original := []byte(`{"model":"public-model","input":{"messages":[{"role":"user","content":[{"image":"https://example.invalid/triangle.png"},{"text":"describe"}]}]},"parameters":{"enable_search":false,"incremental_output":false,"max_tokens":10}}`)
				mapped := bytes.Replace(original, []byte("public-model"), []byte("upstream-model"), 1)
				headers := managedRequestHeaders(t, original)
				if stream {
					headers = append(headers, [2]string{"X-DashScope-SSE", "enable"})
				}
				if action := h.host.CallOnRequestHeaders(h.id, headers, false); action != types.HeaderStopIteration {
					t.Fatalf("headers action: %v", action)
				}
				h.host.CallOnRequestBody(h.id, mapped, true)
				calls := h.host.GetCalloutAttributesFromContext(h.id)
				if len(calls) != 1 || !bytes.Equal(calls[0].Body, mapped) {
					t.Fatalf("native body must pass unchanged through one sender: %v", calls)
				}
				if path, _ := managedSingleHeader(calls[0].Headers, ":path"); path != settings.UpstreamPath {
					t.Fatalf("actual native path: %s", path)
				}
				if value, _ := managedSingleHeader(calls[0].Headers, "authorization"); value != "Bearer controlled-upstream-key" {
					t.Fatal("native request did not use the selected supplier credential")
				}
				value, _ := managedSingleHeader(calls[0].Headers, "x-dashscope-sse")
				if (value == "enable") != stream {
					t.Fatal("native header streaming intent was lost")
				}
				accept, _ := managedSingleHeader(calls[0].Headers, "accept")
				if !stream && accept != "application/json" {
					t.Fatal("native JSON request advertised SSE to the supplier")
				}
				body := []byte(`{"request_id":"native-receipt","output":{"text":"triangle","finish_reason":"stop"},"usage":{"input_tokens":37,"output_tokens":2,"total_tokens":39}}`)
				responseHeaders := [][2]string{{":status", "200"}, {"content-type", "application/json"}}
				if stream {
					responseHeaders[1][1] = "text/event-stream"
					body = append([]byte("event: result\ndata: {\"output\":{\"text\":\"tri\",\"finish_reason\":\"null\"},\"usage\":{\"input_tokens\":37,\"output_tokens\":1}}\n\nevent: result\ndata: "), append(body, []byte("\n\n")...)...)
				}
				h.response(t, responseHeaders, body)
				response := h.host.GetSentLocalResponse(h.id)
				if response == nil || response.StatusCode != 200 || !bytes.Equal(response.Data, body) {
					t.Fatal("DashScope response was converted or lost")
				}
				records := h.records(t)
				if len(records) != 2 {
					t.Fatalf("expected one registration and one procurement attempt, got %d", len(records))
				}
				for _, record := range records {
					if record.messageType == "execution_registration" {
						continue
					}
					fact := decodeManagedFact(t, record.raw)
					if fact.ExecutionResult != "succeeded" || fact.Usage.Quantities["input"] != "37" || fact.Usage.Quantities["output"] != "2" {
						t.Fatalf("native usage must use the final cumulative counters once: %#v", fact)
					}
				}
			})
		}
	}
}

func TestManagedDashScopeRejectsUnsupportedPairsAndNonTextOutput(t *testing.T) {
	if _, err := newManagedProtocolProfile(managedProtocolDashScopeText, managedProtocolChat); err == nil {
		t.Fatal("unregistered DashScope conversion accepted")
	}
	profile, _ := newManagedProtocolProfile(managedProtocolDashScopeMultimodal, managedProtocolDashScopeMultimodal)
	for _, body := range []string{
		`{"model":"upstream-model","input":{"prompt":"generate an image"}}`,
		`{"model":"upstream-model","input":{"messages":[]}}`,
		`{"model":"upstream-model","input":{"messages":[{}]},"parameters":{"modalities":["audio"]}}`,
	} {
		if _, err := profile.PrepareRequest([]byte(body), "upstream-model", "controlled-key"); err == nil {
			t.Fatalf("non-text operation accepted: %s", body)
		}
	}
}
