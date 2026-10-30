package executionfacts

import (
	"bytes"
	"errors"
	"testing"
)

func TestStreamACKLossRetryPreservesBytesAndIgnoresOldCallbacks(t *testing.T) {
	snapshot := mustFreeze(t, attemptFixture())
	delivery, err := NewDelivery(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var bodies [][]byte
	var callbacks []func(AppendResult)
	appendFn := func(body []byte, done func(AppendResult)) error {
		bodies = append(bodies, append([]byte(nil), body...))
		callbacks = append(callbacks, done)
		return nil
	}
	if err := delivery.Dispatch(appendFn, nil); err != nil {
		t.Fatal(err)
	}
	if err := delivery.Dispatch(appendFn, nil); err == nil || len(bodies) != 1 {
		t.Fatal("pending dispatch appended twice")
	}
	if state := delivery.MarkUnknown(nil); state.State != "unknown" || state.Reason != "stream_ack_missing" {
		t.Fatal("ACK loss was not explicit")
	}
	if err := delivery.Dispatch(appendFn, nil); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bodies[0], bodies[1]) {
		t.Fatal("resend changed original event bytes")
	}
	callbacks[0](AppendResult{StreamID: "10-0"})
	if delivery.Status().State != "in_flight" {
		t.Fatal("old callback overwrote new append state")
	}
	callbacks[1](AppendResult{StreamID: "11-0"})
	if delivery.Status().State != "acknowledged" {
		t.Fatal("current insertion ACK was lost")
	}
	if err := delivery.Dispatch(appendFn, nil); err != nil || len(bodies) != 2 {
		t.Fatal("acknowledged event appended again")
	}
}
func TestStreamFailuresAreObservableAndNeverMarkPrematureSuccess(t *testing.T) {
	cases := []struct {
		name          string
		append        AppendFunc
		state, reason string
	}{
		{"unconfigured", nil, "gap", "stream_unconfigured"},
		{"dispatch", func([]byte, func(AppendResult)) error { return errors.New("dispatch failed") }, "gap", "append_dispatch_failed"},
		{"response", func(_ []byte, done func(AppendResult)) error {
			done(AppendResult{Err: errors.New("unknown response")})
			return nil
		}, "unknown", "stream_append_not_acknowledged"},
		{"empty_ack", func(_ []byte, done func(AppendResult)) error { done(AppendResult{}); return nil }, "unknown", "stream_append_not_acknowledged"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, _ := NewDelivery(mustFreeze(t, attemptFixture()))
			var seen []DeliveryStatus
			_ = d.Dispatch(tc.append, func(s DeliveryStatus) { seen = append(seen, s) })
			s := d.Status()
			if s.State != tc.state || s.Reason != tc.reason || len(seen) < 2 {
				t.Fatalf("gap not observable: %#v", s)
			}
		})
	}
}
func TestSuccessfulInlineACKCannotBeUndoneByDispatchError(t *testing.T) {
	d, _ := NewDelivery(mustFreeze(t, attemptFixture()))
	_ = d.Dispatch(func(_ []byte, done func(AppendResult)) error {
		done(AppendResult{StreamID: "12-0"})
		return errors.New("late dispatch status")
	}, nil)
	if d.Status().State != "acknowledged" {
		t.Fatal("late error undid actual insertion ACK")
	}
}
