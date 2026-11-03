package executionfacts

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

func TestOutboxRetriesOriginalBytesAfterLostACKAndHTTPCompletion(t *testing.T) {
	snapshot := mustFreeze(t, attemptFixture())
	box := NewOutbox()
	var sent [][]byte
	var callbacks []func(AppendResult)
	appendEvent := func(body []byte, done func(AppendResult)) error {
		sent = append(sent, bytes.Clone(body))
		callbacks = append(callbacks, done)
		return nil
	}
	delivery, err := box.Submit(snapshot, appendEvent, nil)
	if err != nil || box.Pending() != 1 {
		t.Fatal("event not retained", err)
	}
	callbacks[0](AppendResult{Err: errors.New("append succeeded but ACK was lost")})
	box.pending[snapshot.EventID()].next = time.Time{}
	box.Retry()
	if len(sent) != 2 || !bytes.Equal(sent[0], sent[1]) {
		t.Fatal("retry changed the original event")
	}
	callbacks[1](AppendResult{StreamID: "23-1"})
	if delivery.Status().State != "acknowledged" || box.Pending() != 0 || box.bytes != 0 {
		t.Fatal("successful append did not release retention")
	}
	box.Retry()
	if len(sent) != 2 {
		t.Fatal("acknowledged event was sent again")
	}
}

func TestOutboxRetriesPreAppendFailureWithoutProviderResend(t *testing.T) {
	snapshot := mustFreeze(t, deliveryFixture())
	box := NewOutbox()
	calls := 0
	delivery, err := box.Submit(snapshot, func(body []byte, done func(AppendResult)) error {
		calls++
		if calls == 1 {
			return errors.New("Redis unavailable before append")
		}
		if !bytes.Equal(body, snapshot.Bytes()) {
			t.Fatal("original sales event changed")
		}
		done(AppendResult{StreamID: "24-1"})
		return nil
	}, nil)
	if err == nil || box.Pending() != 1 {
		t.Fatal("failed dispatch was lost")
	}
	box.pending[snapshot.EventID()].next = time.Time{}
	box.Retry()
	if calls != 2 || delivery.Status().State != "acknowledged" || box.Pending() != 0 {
		t.Fatal("failed sales event was not recovered")
	}
}

func TestOutboxStopsNewAdmissionAtBacklogBudget(t *testing.T) {
	box := NewOutbox()
	box.bytes = MaxRetainedBytes
	if box.Accepting() {
		t.Fatal("backlog budget accepted new business")
	}
}
