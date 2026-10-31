package main

import (
	"testing"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
)

func TestProductionUploadCanExceedTemporaryPermitWindow(t *testing.T) {
	h := newManagedHost(t)
	if action := h.host.CallOnRequestHeaders(h.id, managedRequestHeaders(t, []byte(managedOriginalBody)), false); action != types.HeaderStopIteration {
		t.Fatalf("production headers action: %v", action)
	}
	// A normal body upload can outlast the old validation permit window.
	time.Sleep(6 * time.Second)
	h.host.CallOnRequestBody(h.id, []byte(managedMappedBody), true)
	callouts := h.host.GetCalloutAttributesFromContext(h.id)
	response := h.host.GetSentLocalResponse(h.id)
	t.Logf("business callouts after slow upload: %d; local response: %#v", len(callouts), response)
	if len(callouts) != 1 {
		t.Fatalf("an already Redis-admitted production request must retain its configured upload/send budget, got %d business sends", len(callouts))
	}
}
