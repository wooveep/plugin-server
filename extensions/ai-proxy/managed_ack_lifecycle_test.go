package main

import (
	"strings"
	"testing"
)

func TestManagedACKSurvivesActualHTTPContextDeletion(t *testing.T) {
	for _, late := range []bool{false, true} {
		for _, failed := range []bool{false, true} {
			name := map[bool]string{false: "ack_before_end", true: "ack_after_delete"}[late] + "/" + map[bool]string{false: "success", true: "redis_failure"}[failed]
			t.Run(name, func(t *testing.T) {
				h := newManagedHost(t)
				h.start(t, []byte(managedOriginalBody), []byte(managedMappedBody))
				h.host.CallOnRedisCallResponse(h.records(t)[0].calloutID, 0, []byte("$5\r\n100-0\r\n"))
				h.response(t, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, []byte(managedSuccessfulBody))
				pending := h.records(t)
				if len(pending) != 1 {
					t.Fatalf("expected one pending attempt; got %d", len(pending))
				}
				state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
				if late {
					h.host.CompleteHttpContext(h.id)
					if got := h.ctx.GetUserAttribute("execution_fact_gap"); got == true {
						t.Fatal("HTTP completion was treated as an ACK deadline")
					}
				}
				reply := []byte("$5\r\n101-0\r\n")
				if failed {
					reply = []byte("-ERR controlled append failure\r\n")
				}
				h.host.CallOnRedisCallResponse(pending[0].calloutID, 0, reply)
				if !late {
					h.host.CompleteHttpContext(h.id)
				}
				for _, delivery := range state.deliveries {
					got := delivery.Status()
					if got.EventID == state.lastAttempt.EventID() {
						if failed && (got.State != "unknown" || h.ctx.GetUserAttribute("execution_fact_gap") != true) {
							t.Fatalf("real append failure was hidden: %#v", got)
						}
						if !failed && got.State != "acknowledged" {
							t.Fatalf("successful ACK was lost: %#v", got)
						}
					}
				}
				for _, warning := range h.host.GetWarnLogs() {
					if strings.Contains(warning, "stream_ack_missing") {
						t.Fatalf("request completion produced a false ACK gap: %s", warning)
					}
				}
				if !failed && h.ctx.GetUserAttribute("execution_fact_gap") == true {
					t.Fatal("healthy ACK order left a sticky gap")
				}
				if got := h.host.GetSentLocalResponse(h.id); got == nil || got.StatusCode != 200 {
					t.Fatal("event ACK changed the completed business response")
				}
			})
		}
	}
}
