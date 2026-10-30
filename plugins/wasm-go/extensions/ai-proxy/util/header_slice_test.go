package util

import (
	"net/http"
	"reflect"
	"testing"
)

func TestCreateHeaders(t *testing.T) {
	h := CreateHeaders("Content-Type", "application/json", ":status", "200")
	if len(h) != 2 {
		t.Fatalf("len=%d", len(h))
	}
	if h[0][0] != "Content-Type" || h[0][1] != "application/json" {
		t.Fatalf("first pair: %v", h[0])
	}
}

func TestHeaderToSliceAndSliceToHeader_roundTrip(t *testing.T) {
	src := make(http.Header)
	src.Set("A", "1")
	src.Add("A", "2")
	src.Set("B", "3")

	slice := HeaderToSlice(src)
	round := SliceToHeader(slice)

	if !reflect.DeepEqual(src["A"], round["A"]) || !reflect.DeepEqual(src["B"], round["B"]) {
		t.Fatalf("roundTrip mismatch: %#v vs %#v", src, round)
	}
}

func TestOverwriteRequestPathHeader(t *testing.T) {
	h := make(http.Header)
	OverwriteRequestPathHeader(h, "/v1/chat/completions")
	if h.Get(":path") != "/v1/chat/completions" {
		t.Fatalf("path=%q", h.Get(":path"))
	}
}

func TestOverwriteRequestAuthorizationHeaderStripsClientAuthHeaders(t *testing.T) {
	h := make(http.Header)
	h.Set(HeaderAuthorization, "Bearer client-token")
	h.Set("x-api-key", "client-api-key")
	h.Set("anthropic-api-key", "client-anthropic-key")
	h.Set("x-authorization", "Bearer client-alt-token")
	h.Set(HeaderOriginalAuth, "Bearer original-client-token")
	h.Set("x-request-id", "request-1")

	OverwriteRequestAuthorizationHeader(h, "Bearer provider-token")

	if got := h.Get(HeaderAuthorization); got != "Bearer provider-token" {
		t.Fatalf("authorization=%q", got)
	}
	for _, name := range []string{"x-api-key", "anthropic-api-key", "x-authorization"} {
		if got := h.Get(name); got != "" {
			t.Fatalf("%s=%q, want stripped", name, got)
		}
	}
	if got := h.Get(HeaderOriginalAuth); got != "Bearer original-client-token" {
		t.Fatalf("original auth=%q", got)
	}
	if got := h.Get("x-request-id"); got != "request-1" {
		t.Fatalf("x-request-id=%q", got)
	}
}
