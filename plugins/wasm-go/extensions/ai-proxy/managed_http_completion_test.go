package main

import "testing"

func TestManagedActualHTTPDeletionStillRecordsOneCancelledSale(t *testing.T) {
	h := newManagedHost(t)
	h.start(t, []byte(managedOriginalBody), []byte(managedMappedBody))
	call := h.host.GetCalloutAttributesFromContext(h.id)[0]
	h.host.CompleteHttpContext(h.id)
	h.host.CallOnHttpCallResponse(call.CalloutID, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, nil, []byte(managedSuccessfulBody))
	sales := 0
	for _, record := range h.records(t) {
		if record.messageType != "observation" {
			continue
		}
		fact := decodeManagedFact(t, record.raw)
		if fact.Kind == "call_delivery" {
			sales++
			if fact.DeliveryState != "cancelled" || fact.Usage.Quantities["input"] != "9007199254740993" {
				t.Fatal("cancelled call lost its billable original usage")
			}
		}
	}
	if sales != 1 || h.host.GetSentLocalResponse(h.id) != nil || len(h.host.GetCalloutAttributesFromContext(h.id)) != 0 {
		t.Fatal("deleted request lost its sale, responded, or resent")
	}
}
