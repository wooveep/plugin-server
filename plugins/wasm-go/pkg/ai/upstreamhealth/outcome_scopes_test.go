package upstreamhealth

import "testing"

func TestRateLimitProjectionKeepsModelScopeSeparateFromAccountAndCredential(t *testing.T) {
	wire := fixtureWire()
	wire.Scopes = append(wire.Scopes, ScopeState{Identity: ScopeIdentity{ScopeID: "scope:account", Kind: "account", ProviderAccountID: "account:original"}, Revision: "6", FailureGeneration: "0", RecoveryEpoch: "0"})
	other := wire.Scopes[0]
	other.Identity.ScopeID = "scope:other-model"
	other.Identity.UpstreamModel = pointer("different-model")
	wire.Scopes = append(wire.Scopes, other)
	snapshot := readyFixture(t, wire).Snapshot
	target := fixtureTarget()
	target.ScopeIDs = append(target.ScopeIDs, "scope:account")
	ids, revisions, err := snapshot.ModelOperationScopes(target, fixtureNow)
	if err != nil || len(ids) != 1 || ids[0] != "scope:model" || len(revisions) != 1 || revisions["scope:model"] != "4" {
		t.Fatalf("unscoped 429 escaped its original model domain: %#v %#v %v", ids, revisions, err)
	}
	target.ScopeIDs = []string{"scope:account"}
	if _, _, err := snapshot.ModelOperationScopes(target, fixtureNow); err == nil {
		t.Fatal("missing registered model scope borrowed an unrelated account or model domain")
	}
}
