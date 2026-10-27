package bodybasis_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
)

func TestCaptureKeepsOriginalByteBasis(t *testing.T) {
	raw := []byte(" {\n  \"model\" : \"public.v1\", \"amount\":9007199254740993.0000000001, \"messages\":[]\n}\n")
	original := append([]byte(nil), raw...)
	snapshot, err := bodybasis.Capture(raw)
	if err != nil {
		t.Fatal(err)
	}
	canonical := []byte(`{"amount":9007199254740993.0000000001,"messages":[],"model":"public.v1"}`)
	expected := fmt.Sprintf("sha256:%x", sha256.Sum256(canonical))
	if snapshot.Digest() != expected || snapshot.PublicModel() != "public.v1" {
		t.Fatalf("unexpected snapshot: %q / %q", snapshot.Digest(), snapshot.PublicModel())
	}
	compact := []byte(`{"messages":[],"model":"public.v1","amount":9007199254740993.00000000010}`)
	equivalent, err := bodybasis.Capture(compact)
	if err != nil || snapshot.Digest() != equivalent.Digest() {
		t.Fatalf("mf-json.v1 identity must ignore key order, whitespace and equivalent decimal lexemes: %v", err)
	}
	if !bytes.Equal(raw, original) || snapshot.Digest() == bodybasis.DigestBytes(raw) {
		t.Fatal("Capture must retain original body bytes and hash canonical JSON")
	}
	if bodybasis.DigestBytes(raw) != fmt.Sprintf("sha256:%x", sha256.Sum256(original)) {
		t.Fatal("outbound/response raw-bytes.v1 digest changed")
	}
	raw[0] = '!'
	headers := snapshot.Headers()
	headers[0][1] = "changed"
	headers[1][1] = "changed"
	names := bodybasis.HeaderNames()
	names[0] = "changed"
	if snapshot.Digest() != expected || snapshot.PublicModel() != "public.v1" {
		t.Fatal("mutating input bytes or returned headers changed the snapshot")
	}
	if bodybasis.HeaderNames()[0] != bodybasis.DigestHeader {
		t.Fatal("reserved header names must be immutable")
	}
	if err := snapshot.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureRejectsAmbiguousOrInvalidModelBody(t *testing.T) {
	cases := map[string][]byte{
		"empty":            nil,
		"array":            []byte(`[{"model":"a"}]`),
		"missing":          []byte(`{"messages":[]}`),
		"null":             []byte(`{"model":null}`),
		"number":           []byte(`{"model":123}`),
		"blank":            []byte(`{"model":"  "}`),
		"control":          []byte(`{"model":"a\r\nb"}`),
		"duplicate":        []byte(`{"model":"a","model":"b"}`),
		"escaped key":      []byte(`{"model":"a","mo\u0064el":"b"}`),
		"other duplicate":  []byte(`{"model":"a","messages":[],"messages":[1]}`),
		"nested duplicate": []byte(`{"model":"a","messages":[{"role":"user","role":"system"}]}`),
		"surrogate model":  []byte(`{"model":"\ud800"}`),
		"surrogate body":   []byte(`{"model":"a","input":"\udfff"}`),
		"wrong case":       []byte(`{"Model":"a"}`),
		"trailing":         []byte(`{"model":"a"} {"model":"b"}`),
		"truncated":        []byte(`{"model":"a"`),
		"invalid utf8":     append([]byte(`{"model":"`), 0xff, '"', '}'),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := bodybasis.Capture(raw); err == nil {
				t.Fatal("expected body rejection")
			}
		})
	}
}

func TestReadRequiresCompleteUnambiguousCanonicalHeaders(t *testing.T) {
	snapshot, err := bodybasis.Capture([]byte(`{"model":"public/中文"}`))
	if err != nil {
		t.Fatal(err)
	}
	valid := snapshot.Headers()
	valid[0][0] = strings.ToUpper(valid[0][0])
	read, err := bodybasis.Read(valid)
	if err != nil || read.Digest() != snapshot.Digest() || read.PublicModel() != snapshot.PublicModel() {
		t.Fatalf("case-insensitive valid headers failed: %v", err)
	}
	cases := map[string][][2]string{
		"missing":          valid[:1],
		"duplicate digest": append(snapshot.Headers(), valid[0]),
		"duplicate model":  append(snapshot.Headers(), valid[1]),
		"short digest":     {{bodybasis.DigestHeader, "sha256:00"}, {bodybasis.PublicModelHeader, "a"}},
		"uppercase hash":   {{bodybasis.DigestHeader, "sha256:" + strings.Repeat("A", 64)}, {bodybasis.PublicModelHeader, "a"}},
		"wrong prefix":     {{bodybasis.DigestHeader, strings.Repeat("a", 64)}, {bodybasis.PublicModelHeader, "a"}},
		"control model":    {{bodybasis.DigestHeader, snapshot.Digest()}, {bodybasis.PublicModelHeader, "a\nb"}},
	}
	for name, headers := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := bodybasis.Read(headers); err == nil {
				t.Fatal("expected header rejection")
			}
		})
	}
	var zero bodybasis.Snapshot
	if err := zero.Validate(); err == nil {
		t.Fatal("zero snapshot must not be accepted")
	}
}
