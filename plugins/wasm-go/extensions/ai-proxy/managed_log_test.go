package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

func readManagedAccessLog(t *testing.T, h *managedHost) (managedAccessLog, []byte) {
	t.Helper()
	raw, err := h.host.GetProperty([]string{managedAccessLogProperty})
	if err != nil || len(raw) == 0 {
		t.Fatalf("actual host log property missing: %v", err)
	}
	var unescaped string
	if err := json.Unmarshal([]byte(`"`+string(raw)+`"`), &unescaped); err != nil {
		t.Fatal(err)
	}
	var entry managedAccessLog
	if err := json.Unmarshal([]byte(unescaped), &entry); err != nil {
		t.Fatal(err)
	}
	return entry, raw
}

func TestManagedAccessLogUsesActualSendAndOnlySafeFields(t *testing.T) {
	h := newManagedHost(t)
	h.start(t, []byte(managedOriginalBody), []byte(managedMappedBody))
	call := h.host.GetCalloutAttributesFromContext(h.id)[0]
	callID, _ := managedSingleHeader(call.Headers, managedCallHeader)
	attemptID, _ := managedSingleHeader(call.Headers, managedAttemptHeader)
	entry, _ := readManagedAccessLog(t, h)
	if entry.CallID != callID || entry.ReleaseID != "release-1" || len(entry.Attempts) != 1 || entry.Attempts[0].AttemptID != attemptID || entry.Attempts[0].ParentAttemptID != "" {
		t.Fatalf("log did not reflect the actual single send: %+v", entry)
	}
	h.ctx.SetUserAttribute("credential", "must-never-log-credential")
	h.ctx.SetUserAttribute("prompt", "must-never-log-prompt")
	h.ctx.SetUserAttribute("managed_call_id", "forged-attribute-call")
	if err := h.host.SetProperty([]string{wrapper.CustomLogKey}, []byte("unrelated-existing-log")); err != nil {
		t.Fatal(err)
	}
	managedFactGap(h.ctx, "stream_ack_missing")
	recordManagedAccessFailure(h.ctx, "must-never-log-arbitrary-provider-error", false)
	entry, raw := readManagedAccessLog(t, h)
	if !entry.FactGap || !entry.Rejected || len(entry.ReasonCodes) != 2 || entry.ReasonCodes[0] != "stream_ack_missing" || entry.ReasonCodes[1] != "other_managed_failure" || entry.CallID != callID {
		t.Fatalf("safe low-cardinality fields differ: %+v", entry)
	}
	for _, forbidden := range []string{"must-never-log", "forged-attribute-call", "controlled-upstream-key", "hello", "run_id"} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("unapproved field entered the native access log: %s", forbidden)
		}
	}
	other, _ := h.host.GetProperty([]string{wrapper.CustomLogKey})
	if string(other) != "unrelated-existing-log" {
		t.Fatal("native log overwrote the shared wrapper log")
	}
	h.host.CompleteHttpContext(h.id)
	_, before := readManagedAccessLog(t, h)
	recordManagedAccessFailure(h.ctx, "fact_dispatch_failed", true)
	_, after := readManagedAccessLog(t, h)
	if !bytes.Equal(before, after) {
		t.Fatal("late callback rewrote a closed HTTP log")
	}
}

func TestManagedAccessLogKeepsEveryActualRetryInOriginalOrder(t *testing.T) {
	h := newManagedTextHost(t, managedTextFixture(t, false))
	h.start(t, []byte(managedOriginalBody), []byte(managedMappedBody))
	respondManagedTextHealth(t, h, map[string]bool{"credential-1": true})
	first := h.host.GetCalloutAttributesFromContext(h.id)[0]
	firstID, _ := managedSingleHeader(first.Headers, managedAttemptHeader)
	entry, _ := readManagedAccessLog(t, h)
	if len(entry.Attempts) != 1 || entry.Attempts[0].AttemptID != firstID {
		t.Fatal("first actual attempt is absent")
	}
	h.host.CallOnHttpCallResponse(first.CalloutID, [][2]string{{":status", "500"}, {"content-type", "application/json"}}, nil, []byte(`{"error":{"message":"controlled failure"}}`))
	entry, _ = readManagedAccessLog(t, h)
	if len(entry.Attempts) != 1 {
		t.Fatal("pending HEALTH selection was logged as another actual attempt")
	}
	respondManagedTextHealth(t, h, map[string]bool{"credential-2": true})
	second := h.host.GetCalloutAttributesFromContext(h.id)[0]
	secondID, _ := managedSingleHeader(second.Headers, managedAttemptHeader)
	entry, raw := readManagedAccessLog(t, h)
	if len(entry.Attempts) != 2 || entry.Attempts[0].AttemptID != firstID || entry.Attempts[1].AttemptID != secondID || entry.Attempts[1].ParentAttemptID != firstID || firstID == secondID {
		t.Fatalf("original attempt order/parent was lost: %+v", entry)
	}
	h.host.CallOnHttpCallResponse(second.CalloutID, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, nil, []byte(managedSuccessfulBody))
	final, _ := readManagedAccessLog(t, h)
	if len(final.Attempts) != 2 || final.CallID != entry.CallID || final.ReleaseID != entry.ReleaseID {
		t.Fatal("final response replaced the full attempt chain")
	}
	for _, forbidden := range []string{"controlled-upstream-key", "second-controlled-key", "credential-1", "credential-2"} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatal("credential metadata was added to an access log")
		}
	}
}

func TestManagedAccessLogRejectsClientIdentityAndPreservesUnknown(t *testing.T) {
	h := newManagedHost(t)
	headers := [][2]string{{":method", "POST"}, {":path", "/custom/customer/text-alias"}, {":authority", "gateway.example"}, {"content-type", "application/json"}, {managedCallHeader, "client-invented-call"}, {managedAttemptHeader, "client-invented-attempt"}, {"x-run-id", "client-invented-run"}}
	if action := h.host.CallOnRequestHeaders(h.id, headers, false); action != types.ActionPause {
		t.Fatalf("unauthenticated request did not stop: %v", action)
	}
	entry, raw := readManagedAccessLog(t, h)
	if !entry.Rejected || entry.CallID != "" || entry.ReleaseID != "release-1" || len(entry.Attempts) != 0 || len(h.host.GetCalloutAttributesFromContext(h.id)) != 0 {
		t.Fatalf("rejection invented execution identity: %+v", entry)
	}
	if bytes.Contains(raw, []byte("client-invented")) || bytes.Contains(raw, []byte("run_id")) {
		t.Fatal("customer headers became native log authority")
	}
}

func TestManagedAccessLogBoundsItsOrderedPrefixWithoutReplacingIt(t *testing.T) {
	h := newManagedHost(t)
	h.start(t, []byte(managedOriginalBody), []byte(managedMappedBody))
	original, _ := readManagedAccessLog(t, h)
	state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
	previous := state.attemptID
	// Labelled projection-boundary inputs, not extra business sends.
	for i := 1; i <= managedAccessLogAttempts+1; i++ {
		copy := *state
		copy.attemptID = fmt.Sprintf("50000000-0000-4000-8000-%012d", i)
		copy.parentAttemptID = previous
		recordManagedAccessAttempt(h.ctx, &copy)
		previous = copy.attemptID
	}
	entry, _ := readManagedAccessLog(t, h)
	if !entry.AttemptsTruncated || len(entry.Attempts) != managedAccessLogAttempts || entry.Attempts[0] != original.Attempts[0] {
		t.Fatal("bounded log hid overflow or replaced the original prefix")
	}
	if len(h.host.GetCalloutAttributesFromContext(h.id)) != 1 {
		t.Fatal("access log projection triggered another send")
	}
}
