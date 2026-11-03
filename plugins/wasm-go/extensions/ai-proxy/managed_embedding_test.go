package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
)

func TestManagedEmbeddingProtocolKeepsActualInputAndProviderUsage(t *testing.T) {
	for _, input := range []string{`"hello"`, `["hello","world"]`, `[1,2,3]`, `[[1,2],[3,4]]`} {
		t.Run(input, func(t *testing.T) {
			body := []byte(`{"model":"upstream-model","input":` + input + `}`)
			request, err := prepareManagedEmbeddingRequest(body, "upstream-model", "actual-key")
			if err != nil || !bytes.Equal(request.Body, body) || request.Streaming || request.ExpectedEmbeddings < 1 {
				t.Fatal("embedding input changed", err)
			}
		})
	}
	for _, input := range []string{`null`, `""`, `[]`, `["hello",[1,2]]`, `[{"type":"image_url","image_url":"opaque"}]`, `[-1]`, `[1.5]`} {
		t.Run("invalid_"+input, func(t *testing.T) {
			if _, err := prepareManagedEmbeddingRequest([]byte(`{"model":"upstream-model","input":`+input+`}`), "upstream-model", "actual-key"); err == nil {
				t.Fatal("unsupported embedding input accepted")
			}
		})
	}
	for _, field := range []string{`"stream":true`, `"background":true`, `"stream":null`} {
		t.Run(field, func(t *testing.T) {
			if _, err := prepareManagedEmbeddingRequest([]byte(`{"model":"upstream-model","input":"hello",`+field+`}`), "upstream-model", "actual-key"); err == nil {
				t.Fatal("asynchronous embedding accepted")
			}
		})
	}
	object := map[string]json.RawMessage{"usage": json.RawMessage(`{"prompt_tokens":9007199254740993,"total_tokens":9007199254740993}`)}
	usage := managedEmbeddingUsage(object)
	if usage.Status != "reported" || usage.Source != "provider" || len(usage.Quantities) != 1 || usage.Quantities["input"] != "9007199254740993" {
		t.Fatal("input count rounded or output invented")
	}
	for _, raw := range []string{`null`, `{}`, `{"total_tokens":10}`, `{"prompt_tokens":"10"}`, `{"prompt_tokens":2,"total_tokens":3}`} {
		object["usage"] = json.RawMessage(raw)
		usage = managedEmbeddingUsage(object)
		if usage.Status != "missing" || usage.Quantities != nil || usage.Source != "none" {
			t.Fatal("invalid/missing input inferred from another field")
		}
	}
}

func TestManagedEmbeddingResponseRequiresActualSynchronousVectors(t *testing.T) {
	headers := [][2]string{{"content-type", "application/json"}}
	for _, raw := range []string{`{"data":[{"index":0,"embedding":[1,-0.25]}],"usage":{"prompt_tokens":0,"total_tokens":0}}`, `{"data":[{"index":0,"embedding":"AACAPw=="}]}`} {
		observation := managedEmbeddingObservation(200, headers, []byte(raw), true, 1)
		if observation.Acceptance != "accepted" || observation.ExecutionResult != "succeeded" {
			t.Fatal("actual embedding response rejected")
		}
	}
	for _, raw := range []string{`{}`, `{"data":[]}`, `{"data":[{"index":0,"embedding":[]}]}`, `{"data":[{"index":0,"embedding":["text"]}]}`, `{"data":[{"index":1,"embedding":[1]}]}`, `{"data":[{"index":0,"embedding":[1]},{"index":0,"embedding":[1]}]}`, `{"error":{"message":"provider failure"}}`} {
		observation := managedEmbeddingObservation(200, headers, []byte(raw), true, 1)
		if observation.Acceptance == "accepted" || observation.ExecutionResult == "succeeded" {
			t.Fatal("HTTP200 fabricated embedding success")
		}
	}
	body := []byte(`{"data":[{"index":0,"embedding":[1]}],"usage":{"prompt_tokens":7,"total_tokens":7}}`)
	observation := managedEmbeddingObservation(503, headers, body, true, 1)
	if observation.ExecutionResult != "failed" || observation.Acceptance == "accepted" || observation.Usage.Quantities["input"] != "7" {
		t.Fatal("failure lost independent real input usage")
	}
	observation = managedEmbeddingObservation(200, [][2]string{{"content-type", "text/event-stream"}}, body, true, 1)
	if observation.Acceptance == "accepted" || observation.Usage.Status != "missing" {
		t.Fatal("SSE invented synchronous embedding evidence")
	}
	if _, err := newManagedProtocolProfile(managedProtocolEmbeddings, managedProtocolChat); err == nil {
		t.Fatal("embedding converted through text")
	}
	if executionfacts.NativeEmbeddingOperation == "text_generate" {
		t.Fatal("embedding operation is aliased")
	}
}

func TestCustomerEmbeddingAvailableUsageCountsOnlyOriginalInput(t *testing.T) {
	state := &managedExecutionState{config: &managedExecutionConfig{}}
	state.config.settings.InputProtocol = managedProtocolEmbeddings
	state.config.release.Descriptor.UpstreamModel = "qwen3.7-text-embedding-flash"
	for _, tc := range []struct{ input, want string }{
		{`"hello"`, "1"}, {`["hello","world"]`, "2"}, {`[1,2,3]`, "3"}, {`[[1,2],[3,4,5]]`, "5"},
	} {
		state.embeddingInput = json.RawMessage(tc.input)
		outcome := executionfacts.AttemptOutcome{SendState: "observed_sent", Usage: executionfacts.MissingTokenUsage("omitted")}
		usage := availableManagedUsage(state, outcome, []byte(`{"data":[{"embedding":[1,2,3,4]}]}`))
		if usage.Status != "estimated" || usage.Quantities["input"] != tc.want || len(usage.Quantities) != 1 || usage.EstimatorVersion == "" {
			t.Fatalf("wrong input estimate: %+v", usage)
		}
		zero := &executionfacts.Usage{Status: "reported", Unit: "token", Source: "provider", Quantities: map[string]string{"input": "0"}}
		outcome.Usage = zero
		if availableManagedUsage(state, outcome, nil) != zero {
			t.Fatal("provider zero overwritten")
		}
		outcome.SendState, outcome.Usage = "not_sent", executionfacts.MissingTokenUsage("not_sent")
		if availableManagedUsage(state, outcome, nil).Status != "missing" {
			t.Fatal("pre-send rejection became billable")
		}
	}
}

func TestCustomerEmbeddingSuccessfulHandoffUsesAvailableInput(t *testing.T) {
	state := &managedExecutionState{config: &managedExecutionConfig{}, handedOff: true}
	state.config.settings.InputProtocol = managedProtocolEmbeddings
	state.config.release.Descriptor.UpstreamModel = "qwen3.7-text-embedding-flash"
	state.embeddingInput = json.RawMessage(`"hello"`)
	outcome := executionfacts.AttemptOutcome{SendState: "handed_off", Usage: executionfacts.MissingTokenUsage("upstream_response_unverified")}
	usage := availableManagedUsage(state, outcome, nil)
	if usage.Status != "estimated" || usage.Quantities["input"] != "1" {
		t.Fatalf("successful dispatch did not retain its input charge: %+v", usage)
	}
	state.handedOff = false
	if availableManagedUsage(state, outcome, nil).Status != "missing" {
		t.Fatal("unsuccessful dispatch became billable")
	}
	state.handedOff = true
	outcome.SendState = "not_sent"
	if availableManagedUsage(state, outcome, nil).Status != "missing" {
		t.Fatal("pre-send rejection became billable")
	}
}

func TestCustomerEmbeddingEstimatedUsageReachesClientObservers(t *testing.T) {
	for name, code := range map[string]uint32{"success": 200, "local-error": 502} {
		t.Run(name, func(t *testing.T) {
			h := newManagedHost(t)
			h.start(t, []byte(managedOriginalBody), []byte(managedMappedBody))
			state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
			state.config.settings.InputProtocol = managedProtocolEmbeddings
			state.config.release.PublicModel = "model"
			usage := &executionfacts.Usage{Status: "estimated", Unit: "token", Source: "estimator", EstimatorVersion: "input-estimator.v1", Quantities: map[string]string{"input": "3"}}
			outcome := executionfacts.AttemptOutcome{ObservedAt: managedNow(), SendState: "handed_off", Usage: usage}
			body := []byte(`{"object":"list","model":"model","data":[{"index":0,"embedding":[0.1,0.2]}]}`)
			if code == 502 {
				body = []byte(`{"error":{"code":"upstream_response_unavailable"}}`)
			}
			sendManagedFinal(h.ctx, state, outcome, "failed", code, [][2]string{{"content-type", "application/json"}}, body)
			response := h.host.GetSentLocalResponse(h.id)
			if response == nil || response.StatusCode != code {
				t.Fatal("missing original response policy")
			}
			var object struct {
				Model string `json:"model"`
				Usage struct {
					Input  int    `json:"prompt_tokens"`
					Total  int    `json:"total_tokens"`
					Source string `json:"source"`
				} `json:"usage"`
				Data []struct {
					Embedding []float64 `json:"embedding"`
				} `json:"data"`
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if json.Unmarshal(response.Data, &object) != nil || object.Usage.Input != 3 || object.Usage.Total != 3 || object.Usage.Source != "estimator" || object.Model != "model" {
				t.Fatal("client, TPM and statistics lost the billed estimate")
			}
			if code == 200 && (len(object.Data) != 1 || len(object.Data[0].Embedding) != 2) {
				t.Fatal("usage normalization changed vectors")
			}
			if code == 502 && object.Error.Code != "upstream_response_unavailable" {
				t.Fatal("usage normalization changed transport error")
			}
		})
	}
}
