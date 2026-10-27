package main

import (
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"strings"
	"testing"
)

type executionTestContext struct {
	values     map[string]interface{}
	attributes map[string]interface{}
}

func newExecutionTestContext() *executionTestContext {
	return &executionTestContext{values: map[string]interface{}{}, attributes: map[string]interface{}{}}
}
func (c *executionTestContext) GetContext(k string) interface{}          { return c.values[k] }
func (c *executionTestContext) SetContext(k string, v interface{})       { c.values[k] = v }
func (c *executionTestContext) SetUserAttribute(k string, v interface{}) { c.attributes[k] = v }
func executionIdentityFixture() executionfacts.Envelope {
	return executionfacts.Envelope{ProducerID: "proxy-e1", CallID: "call-1", AttemptID: "attempt-1", ReleaseID: "release-1", RouteID: "route-1", TargetID: "target-1", BindingID: "binding-1", ProviderID: "provider-1", ProviderAccountID: "purchase-1", CredentialRef: "credential-ref-1", CredentialRevision: "1", SelectionDigest: "sha256:" + strings.Repeat("a", 64), UpstreamModel: "model-a", SendTransport: "http_call", RecordingOwner: "ai_proxy", Mechanism: "router", TenantID: "tenant-1", PrincipalID: "user-1", AccountID: "account-1", SalesAssetID: "asset-1", PrincipalKind: "user", BillingMode: "customer", OperationKind: "text_generate", OccurredAt: "2026-10-27T10:00:00Z"}
}
func TestExecutionAttemptCaptureUsesPrivateOriginalIdentity(t *testing.T) {
	ctx := newExecutionTestContext()
	identity := executionIdentityFixture()
	if err := captureExecutionAttempt(ctx, identity); err != nil {
		t.Fatal(err)
	}
	ctx.SetContext(ctxModel, "final-model")
	ctx.SetContext(ctxInputToken, int64(999))
	ctx.SetContext(ctxStatusCode, 502)
	identity.BindingID = "final-binding"
	snapshot, err := completeExecutionAttempt(ctx, "attempt-1", executionfacts.AttemptOutcome{ObservedAt: "2026-10-27T10:00:01Z", SendState: "handed_off", Acceptance: "unknown", ExecutionResult: "unknown"})
	if err != nil {
		t.Fatal(err)
	}
	e := snapshot.Envelope()
	if e.BindingID != "binding-1" || e.UpstreamModel != "model-a" || e.HTTPStatus != nil || e.Usage.Status != "missing" {
		t.Fatal("final mutable BillingEvent context leaked into attempt")
	}
	if err := captureExecutionAttempt(ctx, identity); err == nil {
		t.Fatal("original attempt identity was overwritten")
	}
	if _, err := completeExecutionAttempt(ctx, "uncaptured", executionfacts.AttemptOutcome{}); err == nil {
		t.Fatal("outcome created history without a send snapshot")
	}
}
func TestExecutionCacheDeliveryDoesNotAllocateAttempt(t *testing.T) {
	ctx := newExecutionTestContext()
	e := executionIdentityFixture()
	e.Kind = "call_delivery"
	e.AttemptID = ""
	e.TargetID = ""
	e.BindingID = ""
	e.SendState = ""
	e.ServedFrom = "cache"
	e.DeliveryState = "succeeded"
	e.PublicModel = "public-model"
	e.ObservedAt = "2026-10-27T10:00:01Z"
	e.Usage = executionfacts.MissingTokenUsage("cache_usage_not_observed")
	snapshot, err := captureExecutionDelivery(ctx, e)
	if err != nil {
		t.Fatal(err)
	}
	state, err := executionState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Envelope().AttemptID != "" || len(state.attempts) != 0 {
		t.Fatal("cache delivery fabricated upstream purchase")
	}
}
