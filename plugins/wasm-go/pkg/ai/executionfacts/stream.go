package executionfacts

import (
	"errors"
	"strconv"
	"strings"
	"sync"
)

// AppendResult acknowledges only durable Stream insertion, never T1 or money.
type AppendResult struct {
	StreamID string
	Err      error
}

// AppendFunc is configured by the trusted host. Retries receive identical bytes.
type AppendFunc func(payload []byte, complete func(AppendResult)) error

// DeliveryStatus makes Q41 gaps observable without blocking the business call.
type DeliveryStatus struct {
	State          string
	Reason         string
	Attempts       uint64
	EventID        string
	SourceFactID   string
	SourceRevision string
	PayloadDigest  string
	StreamID       string
}

// Delivery retains the first immutable message through callbacks and ACK loss.
// There is no timer, automatic resend, or claim of lossless delivery here.
type Delivery struct {
	mu       sync.Mutex
	snapshot Snapshot
	state    string
	reason   string
	attempts uint64
	streamID string
}

func NewDelivery(snapshot Snapshot) (*Delivery, error) {
	if len(snapshot.wire) == 0 {
		return nil, errors.New("empty execution fact snapshot")
	}
	return &Delivery{snapshot: snapshot, state: "pending"}, nil
}
func (d *Delivery) statusLocked() DeliveryStatus {
	return DeliveryStatus{State: d.state, Reason: d.reason, Attempts: d.attempts,
		EventID: d.snapshot.EventID(), SourceFactID: d.snapshot.SourceFactID(),
		SourceRevision: d.snapshot.Revision(), PayloadDigest: d.snapshot.Digest(), StreamID: d.streamID}
}
func (d *Delivery) Status() DeliveryStatus { d.mu.Lock(); defer d.mu.Unlock(); return d.statusLocked() }

// Dispatch explicitly retries only pending/gap/unknown messages, preserving IDs,
// timestamps and evidence. Duplicate calls while pending I/O do not append twice.
func (d *Delivery) Dispatch(appendToStream AppendFunc, observe func(DeliveryStatus)) error {
	d.mu.Lock()
	if d.state == "acknowledged" {
		d.mu.Unlock()
		return nil
	}
	if d.state == "in_flight" {
		d.mu.Unlock()
		return errors.New("execution fact append is already in flight")
	}
	d.attempts++
	attempt := d.attempts
	d.state = "in_flight"
	d.reason = "append_ack_pending"
	start := d.statusLocked()
	d.mu.Unlock()
	if observe != nil {
		observe(start)
	}
	if appendToStream == nil {
		d.finish(attempt, AppendResult{Err: errors.New("stream appender unavailable")}, "stream_unconfigured", observe)
		return errors.New("execution fact stream appender unavailable")
	}
	err := appendToStream(d.snapshot.Bytes(), func(result AppendResult) { d.finish(attempt, result, "", observe) })
	if err != nil {
		d.finish(attempt, AppendResult{Err: err}, "append_dispatch_failed", observe)
	}
	return err
}
func (d *Delivery) finish(attempt uint64, result AppendResult, dispatchReason string, observe func(DeliveryStatus)) {
	d.mu.Lock()
	// A callback/dispatch error from an older explicit resend cannot overwrite the
	// current status. A callback followed by a dispatch error cannot undo its ACK.
	if attempt != d.attempts || d.state == "acknowledged" || (d.state != "in_flight" && d.state != "unknown") {
		d.mu.Unlock()
		return
	}
	if result.Err == nil && validStreamID(result.StreamID) {
		d.state = "acknowledged"
		d.reason = "stream_append_acknowledged"
		d.streamID = result.StreamID
	} else if dispatchReason != "" {
		d.state = "gap"
		d.reason = dispatchReason
	} else {
		d.state = "unknown"
		d.reason = "stream_append_not_acknowledged"
	}
	status := d.statusLocked()
	d.mu.Unlock()
	if observe != nil {
		observe(status)
	}
}

// MarkUnknown is the caller's explicit timeout/lifecycle boundary. An unreturned
// callback never becomes a successful delivery merely because HTTP completed.
func (d *Delivery) MarkUnknown(observe func(DeliveryStatus)) DeliveryStatus {
	d.mu.Lock()
	if d.state == "in_flight" {
		d.state = "unknown"
		d.reason = "stream_ack_missing"
	}
	status := d.statusLocked()
	d.mu.Unlock()
	if observe != nil {
		observe(status)
	}
	return status
}
func validStreamID(s string) bool {
	if len(s) > 41 || s == "0-0" {
		return false
	}
	p := strings.Split(s, "-")
	if len(p) != 2 {
		return false
	}
	for _, part := range p {
		if part == "" {
			return false
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
		if _, err := strconv.ParseUint(part, 10, 64); err != nil {
			return false
		}
	}
	return true
}
