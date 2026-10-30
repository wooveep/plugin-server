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
