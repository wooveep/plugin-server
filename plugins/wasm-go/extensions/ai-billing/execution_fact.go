package main

import (
	"errors"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
)

// These are private host-context keys. No request header or user attribute is
// deserialized as authority. I01/I02 install trusted original identity here.
const ExecutionFactsContextKey = "ai-billing-execution-facts.v1"
const ExecutionFactDeliveryContextKey = "ai-billing-execution-delivery.v1"

// executionFactContext is the small context surface needed by this adapter.
type executionFactContext interface {
	GetContext(key string) interface{}
	SetContext(key string, value interface{})
	SetUserAttribute(key string, value interface{})
}
type executionFactState struct {
	ledger     *executionfacts.Ledger
	attempts   map[string]executionfacts.Attempt
	deliveries map[string]*executionfacts.Delivery
}

func executionState(ctx executionFactContext) (*executionFactState, error) {
	if v := ctx.GetContext(ExecutionFactsContextKey); v != nil {
		state, ok := v.(*executionFactState)
		if !ok {
			return nil, errors.New("execution fact context has incompatible state")
		}
		return state, nil
	}
	// A call retains at most 128 observations; an overflow is an explicit gap,
	// never an unbounded VM-global event registry or a business retry mechanism.
	state := &executionFactState{ledger: executionfacts.NewLedger(128), attempts: map[string]executionfacts.Attempt{}, deliveries: map[string]*executionfacts.Delivery{}}
	ctx.SetContext(ExecutionFactsContextKey, state)
	return state, nil
}

// captureExecutionAttempt is called at the actual selection/send boundary, once
// per new attempt_id. It snapshots the original identity before later mutation.
func captureExecutionAttempt(ctx executionFactContext, identity executionfacts.Envelope) error {
	state, err := executionState(ctx)
	if err != nil {
		return err
	}
	if _, ok := state.attempts[identity.AttemptID]; ok {
		return errors.New("attempt identity was already captured")
	}
	if len(state.attempts) >= 128 {
		return executionfacts.ErrCapacity
	}
	attempt, err := executionfacts.NewAttempt(identity)
	if err != nil {
		return err
	}
	state.attempts[identity.AttemptID] = attempt
	return nil
}

// completeExecutionAttempt accepts only this attempt's observer outcome. It
// never reads BillingEvent's final model, cluster, usage, or default HTTP status.
func completeExecutionAttempt(ctx executionFactContext, attemptID string, outcome executionfacts.AttemptOutcome) (executionfacts.Snapshot, error) {
	state, err := executionState(ctx)
	if err != nil {
		return executionfacts.Snapshot{}, err
	}
	attempt, ok := state.attempts[attemptID]
	if !ok {
		return executionfacts.Snapshot{}, errors.New("original send snapshot is missing")
	}
	snapshot, err := attempt.Finish(outcome)
	if err != nil {
		return executionfacts.Snapshot{}, err
	}
	return state.ledger.Retain(snapshot)
}

// captureExecutionDelivery records exactly one logical final delivery, including
// cache/local delivery with no fabricated upstream_attempt or purchase identity.
func captureExecutionDelivery(ctx executionFactContext, delivery executionfacts.Envelope) (executionfacts.Snapshot, error) {
	if delivery.Kind != "call_delivery" {
		return executionfacts.Snapshot{}, errors.New("expected final call delivery observation")
	}
	state, err := executionState(ctx)
	if err != nil {
		return executionfacts.Snapshot{}, err
	}
	return state.ledger.Record(delivery)
}

// captureExecutionRegistration retains an independent original registration
// before send; it does not mark historical execution as accepted or persisted.
func captureExecutionRegistration(ctx executionFactContext, registration executionfacts.Registration) (executionfacts.Snapshot, error) {
	state, err := executionState(ctx)
	if err != nil {
		return executionfacts.Snapshot{}, err
	}
	snapshot, err := executionfacts.FreezeRegistration(registration)
	if err != nil {
		return executionfacts.Snapshot{}, err
	}
	return state.ledger.Retain(snapshot)
}
