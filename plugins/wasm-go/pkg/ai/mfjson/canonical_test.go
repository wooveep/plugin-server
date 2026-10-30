package mfjson_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
)

// Use the existing frozen vendor corpus; do not maintain another vector copy.
var corpus = filepath.Join("..", "..", "..", "testdata", "contracts", "vendor", "ocloudmodelfusion",
	"mf-common-2026-10-25.3", "fixtures", "mf", "common", "canonical")

func vector(t *testing.T, name, suffix string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(corpus, name+suffix))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func requireCode(t *testing.T, raw []byte, code string) {
	t.Helper()
	canonical, err := mfjson.Canonicalize(raw)
	var rejection *mfjson.Error
	if !errors.As(err, &rejection) || rejection.Code != code || canonical != nil {
		t.Fatalf("expected %s and no output, got %v / %q", code, err, canonical)
	}
	if digest, err := mfjson.Digest(raw); err == nil || digest != "" {
		t.Fatalf("invalid input produced digest %q / %v", digest, err)
	}
}

func TestFrozenCanonicalVectors(t *testing.T) {
	for _, name := range []string{
		"object-order", "decimals", "string-numbers", "controls", "scalar-order",
		"unicode-not-normalized", "absence-null", "array-order",
	} {
		t.Run(name, func(t *testing.T) {
			raw := vector(t, name, ".input.bin")
			expected := vector(t, name, ".expected.bin")
			canonical, err := mfjson.Canonicalize(raw)
			if err != nil || !bytes.Equal(canonical, expected) {
				t.Fatalf("canonical bytes differ: %v / %q, want %q", err, canonical, expected)
			}
			var metadata struct {
				ExpectedDigest string `json:"expected_digest"`
			}
			if err := json.Unmarshal(vector(t, name, ".json"), &metadata); err != nil {
				t.Fatal(err)
			}
			digest, err := mfjson.Digest(raw)
			if err != nil || digest != metadata.ExpectedDigest {
				t.Fatalf("digest differs: %v / %q, want %q", err, digest, metadata.ExpectedDigest)
			}
		})
	}
	for name, code := range map[string]string{
		"duplicate-key": "E_DUPLICATE_KEY", "surrogate": "E_SURROGATE", "bad-utf8": "E_UTF8",
		"nan": "E_JSON", "infinity": "E_JSON", "expansion": "E_LIMIT",
		"output-limit": "E_LIMIT", "container-limit": "E_LIMIT",
	} {
		t.Run(name, func(t *testing.T) { requireCode(t, vector(t, name, ".input.bin"), code) })
	}
}

func TestCanonicalizePreservesExactValuesAndOriginalBytes(t *testing.T) {
	raw := []byte(" \n{\"z\":[{\"b\":null,\"a\":1.20e1}],\"amount\":9007199254740993.0000000001,\"negative\":-0.0}\t")
	original := append([]byte(nil), raw...)
	expected := []byte(`{"amount":9007199254740993.0000000001,"negative":0,"z":[{"a":12,"b":null}]}`)
	canonical, err := mfjson.Canonicalize(raw)
	if err != nil || !bytes.Equal(canonical, expected) || !bytes.Equal(raw, original) {
		t.Fatalf("canonicalization changed a value or original bytes: %v / %q", err, canonical)
	}
	first, err := mfjson.Digest(raw)
	if err != nil {
		t.Fatal(err)
	}
	second, err := mfjson.Digest(expected)
	if err != nil || first != second {
		t.Fatalf("key order/whitespace/equivalent numeric lexemes changed identity: %v", err)
	}
	canonical[0] = '!'
	if !bytes.Equal(raw, original) {
		t.Fatal("canonical output aliases the input")
	}
	for input, expected := range map[string]string{
		`18446744073709551616`:             `18446744073709551616`,
		`9007199254740993.0000000001`:      `9007199254740993.0000000001`,
		`-1.23000E-3`:                      `-0.00123`,
		`"0001.200e+1"`:                    `"0001.200e+1"`,
		`[null,true,false,"\ud834\udd1e"]`: `[null,true,false,"𝄞"]`,
		`{"literal":"\\ud800"}`:            `{"literal":"\\ud800"}`,
	} {
		actual, err := mfjson.Canonicalize([]byte(input))
		if err != nil || string(actual) != expected {
			t.Fatalf("exact value %s: got %q / %v, want %s", input, actual, err, expected)
		}
	}
}

func TestRejectsAmbiguousAndInvalidJSON(t *testing.T) {
	for _, raw := range []string{
		`{"outer":{"x":1,"\u0078":2}}`, `[{"x":1,"x":2}]`,
		`{"\ud834\udd1e":1,"𝄞":2}`,
	} {
		requireCode(t, []byte(raw), "E_DUPLICATE_KEY")
	}
	for _, raw := range []string{`{"x":"\ud800"}`, `"\udc00"`, `"\ud800\u0041"`, `{"\udfff":null}`} {
		requireCode(t, []byte(raw), "E_SURROGATE")
	}
	for _, raw := range []string{"", "{}[]", "01", "1.", "+1", "[1,]", `{"x":}`, `"\x20"`, `"\uZZZZ"`, "\ufeff{}"} {
		requireCode(t, []byte(raw), "E_JSON")
	}
	requireCode(t, []byte{'"', 0xff, '"'}, "E_UTF8")
}

func TestFrozenResourceBoundaries(t *testing.T) {
	valid := []string{
		strings.Repeat("[", mfjson.MaxDepth) + "0" + strings.Repeat("]", mfjson.MaxDepth),
		strings.Repeat("[", mfjson.MaxDepth+1) + strings.Repeat("]", mfjson.MaxDepth+1),
		"[" + strings.Repeat("0,", mfjson.MaxCollectionItems-1) + "0]",
		"1e9999", "1e-9999",
	}
	for _, raw := range valid {
		if _, err := mfjson.Canonicalize([]byte(raw)); err != nil {
			t.Fatalf("rejected value on a frozen budget boundary: %v", err)
		}
	}
	invalid := []string{
		strings.Repeat("[", mfjson.MaxDepth+1) + "0" + strings.Repeat("]", mfjson.MaxDepth+1),
		strings.Repeat("[", mfjson.MaxDepth) + `{"a":0}` + strings.Repeat("]", mfjson.MaxDepth),
		"[" + strings.Repeat("0,", mfjson.MaxCollectionItems) + "0]",
		"1e10000", "1e-10000", "1e9223372036854775808", "1e-9223372036854775808",
		strings.Repeat(" ", mfjson.MaxInputBytes+1),
	}
	for _, raw := range invalid {
		requireCode(t, []byte(raw), "E_LIMIT")
	}
}
