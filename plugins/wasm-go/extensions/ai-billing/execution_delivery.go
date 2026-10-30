package main

import (
	"errors"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/resp"
)

// executionRedisClient is implemented by the existing configured Redis client.
// The caller supplies a private stream and its server-side producer grant; this
// adapter never treats producer_id in a JSON body as authenticated authority.
type executionRedisClient interface {
	Command(args []interface{}, callback wrapper.RedisResponseCallback) error
}

// dispatchExecutionFact appends the frozen v1 observation on the configured
// private stream. Callers apply Q41: record returned gaps and continue business.
// Redis ACK is insertion only; Console must authenticate, register/verify original
// execution, and commit its receive decision before its separate consumer ACK.
func dispatchExecutionFact(ctx executionFactContext, client executionRedisClient, stream string, snapshot executionfacts.Snapshot) (*executionfacts.Delivery, error) {
	state, err := executionState(ctx)
	if err != nil {
		return nil, err
	}
	retained, err := state.ledger.Retain(snapshot)
	if err != nil {
		return nil, err
	}
	delivery, ok := state.deliveries[retained.EventID()]
	if !ok {
		delivery, err = executionfacts.NewDelivery(retained)
		if err != nil {
			return nil, err
		}
		state.deliveries[retained.EventID()] = delivery
	}
	observe := executionDeliveryObserver(ctx)
	var appendToStream executionfacts.AppendFunc
	if client != nil && stream != "" {
		appendToStream = func(body []byte, complete func(executionfacts.AppendResult)) error {
			return client.Command([]interface{}{"XADD", stream, "*", "message_type", snapshot.MessageType(), "event", string(body)}, func(response resp.Value) {
				if response.Error() != nil {
					complete(executionfacts.AppendResult{Err: errors.New("redis append response error")})
					return
				}
				complete(executionfacts.AppendResult{StreamID: response.String()})
			})
		}
	}
	err = delivery.Dispatch(appendToStream, observe)
	return delivery, err
}
func executionDeliveryObserver(ctx executionFactContext) func(executionfacts.DeliveryStatus) {
	return func(status executionfacts.DeliveryStatus) {
		ctx.SetContext(ExecutionFactDeliveryContextKey, status)
		// The full per-event states remain in executionFactState.deliveries. This
		// attribute summarizes the latest append; success never erases another gap.
		ctx.SetUserAttribute("execution_fact_delivery", status.State)
		if status.State == "gap" || status.State == "unknown" {
			ctx.SetUserAttribute("execution_fact_gap", true)
			log.Warnf("ai-billing execution fact delivery gap fail open, event_id:%s reason:%s", status.EventID, status.Reason)
		}
	}
}

// markExecutionFactDeliveryUnknown is used when the configured append deadline
// expires without ACK. It does not create an event or retry a business request.
func markExecutionFactDeliveryUnknown(ctx executionFactContext, eventID string) (executionfacts.DeliveryStatus, error) {
	state, err := executionState(ctx)
	if err != nil {
		return executionfacts.DeliveryStatus{}, err
	}
	delivery, ok := state.deliveries[eventID]
	if !ok {
		return executionfacts.DeliveryStatus{}, errors.New("execution fact delivery was not dispatched")
	}
	return delivery.MarkUnknown(executionDeliveryObserver(ctx)), nil
}

// dispatchExecutionRegistration uses the same immutable ACK/gap discipline, but
// the receiver routes it to release/key validation, never observation acceptance.
func dispatchExecutionRegistration(ctx executionFactContext, client executionRedisClient, stream string, registration executionfacts.Snapshot) (*executionfacts.Delivery, error) {
	if registration.MessageType() != "execution_registration" {
		return nil, errors.New("expected separate execution registration")
	}
	return dispatchExecutionFact(ctx, client, stream, registration)
}

var _ executionRedisClient = wrapper.RedisClient(nil)
var _ executionFactContext = wrapper.HttpContext(nil)

// reportExecutionFactGap is the Q41 integration path for a pre-dispatch encoding
// or original-snapshot error. Only fixed reason codes enter ordinary logs.
func reportExecutionFactGap(ctx executionFactContext, reason string) {
	switch reason {
	case "snapshot_invalid", "registration_invalid", "original_snapshot_missing", "retention_limit":
	default:
		reason = "fact_encoding_failed"
	}
	executionDeliveryObserver(ctx)(executionfacts.DeliveryStatus{State: "gap", Reason: reason})
}
