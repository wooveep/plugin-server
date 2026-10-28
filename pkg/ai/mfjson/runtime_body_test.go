package mfjson_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
)

func TestRuntimeBodyValidationDoesNotChangeFrozenCanonicalBudgets(t *testing.T) {
	raw := []byte(`{"model":"public","attachment":"` + strings.Repeat("A", mfjson.MaxInputBytes) + `","integer":9007199254740993,"decimal":9007199254740993.0000000001}`)
	original := append([]byte(nil), raw...)
	if err := mfjson.ValidateRuntimeBody(raw, 100*1024*1024); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, original) {
		t.Fatal("runtime validation changed the actual wire bytes")
	}
	for name, err := range map[string]error{
		"Canonicalize": func() error { _, err := mfjson.Canonicalize(raw); return err }(),
		"Digest":       func() error { _, err := mfjson.Digest(raw); return err }(),
		"body bound":   mfjson.ValidateRuntimeBody(raw, len(raw)-1),
	} {
		var rejected *mfjson.Error
		if !errors.As(err, &rejected) || rejected.Code != "E_LIMIT" {
			t.Fatalf("%s must retain its own explicit byte budget: %v", name, err)
		}
	}
}

func TestRuntimeBodyValidationRetainsFrozenStrictRules(t *testing.T) {
	for name, raw := range map[string][]byte{
		"nested duplicate": []byte(`{"messages":[{"content":{"n":1,"n":2}}]}`),
		"invalid UTF8":     {'"', 0xff, '"'},
		"surrogate":        []byte(`{"value":"\ud800"}`),
		"expanded number":  []byte(`{"value":1e10000}`),
		"deep":             []byte(strings.Repeat("[", mfjson.MaxDepth+1) + "0" + strings.Repeat("]", mfjson.MaxDepth+1)),
		"many items":       []byte("[" + strings.Repeat("0,", mfjson.MaxCollectionItems) + "0]"),
		"trailing value":   []byte(`{"value":1} {"value":2}`),
	} {
		t.Run(name, func(t *testing.T) {
			_, frozenErr := mfjson.Canonicalize(raw)
			runtimeErr := mfjson.ValidateRuntimeBody(raw, 100*1024*1024)
			var frozen, runtime *mfjson.Error
			if !errors.As(frozenErr, &frozen) || !errors.As(runtimeErr, &runtime) || frozen.Code != runtime.Code {
				t.Fatalf("runtime validation relaxed a frozen rule: canonical=%v runtime=%v", frozenErr, runtimeErr)
			}
		})
	}
}
