package executionfacts

import (
	"sync"
	"time"
)

// Outbox retains original events in the plugin context across HTTP completion.
// Redis is the durable store; retries never repeat a provider request or change IDs.
type Outbox struct {
	mu      sync.Mutex
	pending map[string]*outboxEntry
	bytes   int
}

type outboxEntry struct {
	delivery *Delivery
	append   AppendFunc
	observe  func(DeliveryStatus)
	next     time.Time
}

// NewOutbox creates a plugin-owned queue; callers drive Retry from their root tick.
func NewOutbox() *Outbox { return &Outbox{pending: make(map[string]*outboxEntry)} }

// Submit retains a frozen event before its first asynchronous Redis append.
func (o *Outbox) Submit(snapshot Snapshot, appendToStream AppendFunc, observe func(DeliveryStatus)) (*Delivery, error) {
	o.mu.Lock()
	entry := o.pending[snapshot.EventID()]
	if entry == nil {
		delivery, err := NewDelivery(snapshot)
		if err != nil {
			o.mu.Unlock()
			return nil, err
		}
		entry = &outboxEntry{delivery: delivery, append: appendToStream, observe: observe}
		o.bytes += len(snapshot.wire)
		o.pending[snapshot.EventID()] = entry
	}
	o.mu.Unlock()
	return entry.delivery, o.dispatch(snapshot.EventID(), entry)
}

func (o *Outbox) dispatch(id string, entry *outboxEntry) error {
	return entry.delivery.Dispatch(entry.append, func(status DeliveryStatus) {
		o.mu.Lock()
		if status.State == "acknowledged" {
			if o.pending[id] == entry {
				o.bytes -= len(entry.delivery.snapshot.wire)
				delete(o.pending, id)
			}
		} else {
			// One second prevents a Redis outage from becoming a tight retry loop.
			entry.next = time.Now().Add(time.Second)
		}
		o.mu.Unlock()
		if entry.observe != nil {
			entry.observe(status)
		}
	})
}

// Retry resumes failed/unknown writes using original bytes, independently of HTTP.
// The Redis event client supplies a timeout callback for in-flight writes.
func (o *Outbox) Retry() {
	o.mu.Lock()
	now := time.Now()
	ready := make(map[string]*outboxEntry)
	for id, entry := range o.pending {
		status := entry.delivery.Status()
		if status.State != "in_flight" && !now.Before(entry.next) {
			ready[id] = entry
		}
	}
	o.mu.Unlock()
	for id, entry := range ready {
		_ = o.dispatch(id, entry)
	}
}

// Pending counts events whose durable Redis append has not been acknowledged.
func (o *Outbox) Pending() int { o.mu.Lock(); defer o.mu.Unlock(); return len(o.pending) }

// Accepting applies backpressure before new provider sends during a long outage.
// Already dispatched results remain retained. The eight-MiB event budget and
// 4096-event admission limit bound the backlog accepted from new requests.
func (o *Outbox) Accepting() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.pending) < 4096 && o.bytes < MaxRetainedBytes
}
