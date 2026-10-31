package main

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/higress-group/wasm-go/pkg/ai/upstreamhealth"
	"github.com/tidwall/gjson"
)

func managedPoolFixture(t *testing.T) map[string]interface{} {
	t.Helper()
	fixture := managedConfigFixture()
	release := fixture["_modelfusion_execution"].(managedRelease)
	release.SchemaVersion, release.CredentialRef, release.CredentialRevision = "modelfusion-execution.v2", "", ""
	release.Credentials = []managedCredentialIdentity{{"credential-1", "1"}, {"credential-2", "7"}}
	settings := fixture["managed_execution"].(managedSettings)
	settings.CredentialValue = ""
	settings.Credentials = []managedCredential{{"credential-2", "7", "second-controlled-key"}, {"credential-1", "1", "first-controlled-key"}}
	settings.UpstreamHealth = &managedHealthSettings{RuntimeConfigID: "release-1", ConfigRevision: "1",
		Snapshot: managedHealthSnapshot{Prefix: "modelfusion", AuthorityID: "10000000-0000-0000-0000-000000000001", MinimumRevision: "0"}}
	for _, member := range release.Credentials {
		settings.UpstreamHealth.Targets = append(settings.UpstreamHealth.Targets, upstreamhealth.TargetReference{
			TargetID: "target-1", BindingID: "binding-1", ProviderAccountID: "purchase-1", RegionEndpointRef: "region-1", UpstreamModel: "upstream-model",
			OperationKind: "text_generate", CredentialRef: member.CredentialRef, CredentialGeneration: upstreamhealth.Revision(member.CredentialRevision), ScopeIDs: []string{"model-scope"}})
	}
	fixture["_modelfusion_execution"], fixture["managed_execution"] = release, settings
	// Runtime input is an SDK JSON object, not a marshaled zero struct target.
	var document map[string]interface{}
	if json.Unmarshal(managedFixtureJSON(t, fixture), &document) != nil {
		t.Fatal("fixture did not encode")
	}
	delete(document["managed_execution"].(map[string]interface{})["upstream_health"].(map[string]interface{}), "target")
	return document
}

func decodeManagedPoolFixture(t *testing.T, fixture map[string]interface{}) (*managedExecutionConfig, error) {
	t.Helper()
	value := gjson.ParseBytes(managedFixtureJSON(t, fixture))
	settings, release := value.Get("managed_execution"), value.Get("_modelfusion_execution")
	if err := validateManagedCredentialShape(settings, release); err != nil {
		return nil, err
	}
	var result managedExecutionConfig
	if err := decodeManagedJSON([]byte(settings.Raw), &result.settings, true); err != nil {
		return nil, err
	}
	if err := decodeManagedJSON([]byte(release.Raw), &result.release, true); err != nil {
		return nil, err
	}
	if err := result.validate(); err != nil {
		return nil, err
	}
	return &result, nil
}

func TestManagedV2PoolBindsCompleteIdentitiesIndependentlyOfSecretOrder(t *testing.T) {
	fixture := managedPoolFixture(t)
	before := managedFixtureJSON(t, fixture)
	config, err := decodeManagedPoolFixture(t, fixture)
	if err != nil {
		t.Fatal(err)
	}
	members, err := config.credentialMembers()
	if err != nil || len(members) != 2 || members[0].CredentialValue != "first-controlled-key" || members[1].CredentialValue != "second-controlled-key" {
		t.Fatal("pool position replaced the exact original identity directory")
	}
	if !bytes.Equal(before, managedFixtureJSON(t, fixture)) || config.release.CredentialRef != "" || config.settings.CredentialValue != "" {
		t.Fatal("static configuration acquired a selected scalar seed")
	}
}

func TestManagedV2RejectsPartialMixedOrChangedCredentialDirectories(t *testing.T) {
	for _, mode := range []string{"scalar-ref", "scalar-value", "null-pool", "missing-health", "scalar-health", "duplicate-native", "duplicate-envelope", "wrong-revision", "numeric-revision", "unknown-secret-field", "extra-envelope-secret"} {
		t.Run(mode, func(t *testing.T) {
			fixture := managedPoolFixture(t)
			settings := fixture["managed_execution"].(map[string]interface{})
			release := fixture["_modelfusion_execution"].(map[string]interface{})
			native := settings["credentials"].([]interface{})
			directory := release["credentials"].([]interface{})
			switch mode {
			case "scalar-ref":
				release["credential_ref"] = nil
			case "scalar-value":
				settings["credential_value"] = nil
			case "null-pool":
				settings["credentials"] = nil
			case "missing-health":
				delete(settings, "upstream_health")
			case "scalar-health":
				settings["upstream_health"].(map[string]interface{})["target"] = nil
			case "duplicate-native":
				native[1] = native[0]
			case "duplicate-envelope":
				directory[1] = directory[0]
			case "wrong-revision":
				native[0].(map[string]interface{})["credential_revision"] = "8"
			case "numeric-revision":
				directory[0].(map[string]interface{})["credential_revision"] = 1
			case "unknown-secret-field":
				native[0].(map[string]interface{})["provider_account_id"] = "foreign-account"
			case "extra-envelope-secret":
				directory[0].(map[string]interface{})["credential_value"] = "not-an-identity"
			}
			if _, err := decodeManagedPoolFixture(t, fixture); err == nil {
				t.Fatal("partial or mixed native directory became a selected credential")
			}
		})
	}
}

func TestManagedV1RetainsScalarPathAndRejectsArrayPresence(t *testing.T) {
	if _, err := decodeManagedPoolFixture(t, managedConfigFixture()); err != nil {
		t.Fatal("original v1 scalar path changed")
	}
	for _, location := range []string{"envelope", "native", "health"} {
		fixture := managedConfigFixture()
		var document map[string]interface{}
		if json.Unmarshal(managedFixtureJSON(t, fixture), &document) != nil {
			t.Fatal("fixture did not encode")
		}
		settings := document["managed_execution"].(map[string]interface{})
		switch location {
		case "envelope":
			document["_modelfusion_execution"].(map[string]interface{})["credentials"] = nil
		case "native":
			settings["credentials"] = nil
		case "health":
			settings["upstream_health"] = map[string]interface{}{"targets": nil}
		}
		if _, err := decodeManagedPoolFixture(t, document); err == nil {
			t.Fatal("v1 accepted an array declaration")
		}
	}
}

func TestManagedV2HealthRequiresEveryExactMemberOperation(t *testing.T) {
	for _, mode := range []string{"missing", "duplicate", "account", "target", "binding", "generation", "operation", "scope"} {
		t.Run(mode, func(t *testing.T) {
			fixture := managedPoolFixture(t)
			health := fixture["managed_execution"].(map[string]interface{})["upstream_health"].(map[string]interface{})
			targets := health["targets"].([]interface{})
			second := targets[1].(map[string]interface{})
			switch mode {
			case "missing":
				health["targets"] = targets[:1]
			case "duplicate":
				targets[1] = targets[0]
			case "account":
				second["provider_account_id"] = "foreign-account"
			case "target":
				second["target_id"] = "foreign-target"
			case "binding":
				second["binding_id"] = "foreign-binding"
			case "generation":
				second["credential_generation"] = "6"
			case "operation":
				second["operation_kind"] = "task_query"
			case "scope":
				second["scope_ids"] = []string{"model-scope", "model-scope"}
			}
			if _, err := decodeManagedPoolFixture(t, fixture); err == nil {
				t.Fatal("incomplete or foreign health mapping was accepted")
			}
		})
	}
}

func TestManagedSelectedMemberFreezesRequestSelectionAndFactWithoutMutatingConfig(t *testing.T) {
	c, err := decodeManagedPoolFixture(t, managedPoolFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	c.profile, err = newManagedProtocolProfile("", "")
	if err != nil {
		t.Fatal(err)
	}
	c.health = &managedHealthRuntime{settings: *c.settings.UpstreamHealth, owner: "original-node/original-vm"}
	before, _ := json.Marshal([]interface{}{c.release, c.settings, c.health.settings})
	members, err := managedHealthMembers(c)
	if err != nil {
		t.Fatal(err)
	}
	s := &managedExecutionState{config: c, sourceConfig: c, callID: "50000000-0000-0000-0000-000000000001", attemptID: "original-attempt", businessUTC: "2026-10-29T00:00:00Z",
		identity: balanceidentity.Snapshot{AccountID: "10000000-0000-0000-0000-000000000001", APIKeyID: "20000000-0000-0000-0000-000000000001", CredentialGeneration: "1", TenantID: "30000000-0000-0000-0000-000000000001", PrincipalID: "40000000-0000-0000-0000-000000000001"}}
	if err := freezeManagedHealthMember(s, members[1]); err != nil {
		t.Fatal(err)
	}
	if s.config == c || s.config.health == c.health || s.config.release.CredentialRef != "credential-2" || s.config.settings.CredentialValue != "second-controlled-key" || s.config.health.settings.Target.CredentialRef != "credential-2" {
		t.Fatal("request did not retain its exact selected member")
	}
	basis, err := bodybasis.Capture([]byte(managedOriginalBody))
	if err != nil {
		t.Fatal(err)
	}
	if err := prepareManagedExecution(s, basis, []byte(managedMappedBody)); err != nil {
		t.Fatal(err)
	}
	var selection map[string]interface{}
	if json.Unmarshal(stateSelection(s, basis), &selection) != nil || selection["credential_ref"] != "credential-2" || selection["credential_revision"] != "7" || s.original.CredentialRef != "credential-2" || s.original.CredentialRevision != "7" || s.original.OccurredAt != s.businessUTC {
		t.Fatal("fact or selection substituted a scalar seed or changed admission time")
	}
	after, _ := json.Marshal([]interface{}{c.release, c.settings, c.health.settings})
	if !bytes.Equal(before, after) || s.config.release.Descriptor.ProxyDigest != c.release.Descriptor.ProxyDigest {
		t.Fatal("selection rewrote original config bytes or proxy digest")
	}
	if freezeManagedHealthMember(s, members[0]) == nil || s.config.release.CredentialRef != "credential-2" {
		t.Fatal("a later callback reselected a credential")
	}
	s.config.health.settings.Target.ScopeIDs[0] = "request-local-only"
	if c.settings.UpstreamHealth.Targets[1].ScopeIDs[0] != "model-scope" {
		t.Fatal("request target scopes alias shared configuration")
	}
	if _, err := managedUniformMember(0); err == nil {
		t.Fatal("empty eligible set selected a member")
	}
}

func TestManagedPoolQualificationPreservesDisabledAndGenerationDenials(t *testing.T) {
	c, err := decodeManagedPoolFixture(t, managedPoolFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	members, err := managedHealthMembers(c)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
	for _, mode := range []string{"disabled", "old-generation"} {
		client, err := upstreamhealth.NewGatewayClient(func(string, string, [][2]string, []byte, uint32, func(upstreamhealth.GatewayResponse)) error {
			return nil
		}, [][2]string{{"authorization", "Bearer controlled-service-token"}, {"x-modelfusion-gateway-instance", "original-node/original-vm"}}, "original-node/original-vm", 1000)
		if err != nil {
			t.Fatal(err)
		}
		controls := []upstreamhealth.CredentialControl{
			{Identity: upstreamhealth.CredentialIdentity{CredentialRef: "credential-1", ProviderAccountID: "purchase-1", OwnerKind: "platform", OwnerID: "original-platform"}, Generation: "1", Revision: "1", Enabled: true},
			{Identity: upstreamhealth.CredentialIdentity{CredentialRef: "credential-2", ProviderAccountID: "purchase-1", OwnerKind: "platform", OwnerID: "original-platform"}, Generation: "7", Revision: "1", Enabled: true}}
		if mode == "disabled" {
			controls[0].Enabled = false
		} else {
			controls[0].Generation = "2"
		}
		region, model, operation := "region-1", "upstream-model", "text_generate"
		scopes := []upstreamhealth.ScopeState{{Identity: upstreamhealth.ScopeIdentity{ScopeID: "model-scope", Kind: "model_operation", ProviderAccountID: "purchase-1", RegionEndpointRef: &region, UpstreamModel: &model, OperationKind: &operation}, Revision: "1", FailureGeneration: "0", RecoveryEpoch: "0"}}
		authority := c.settings.UpstreamHealth.Snapshot.AuthorityID
		projection, _ := json.Marshal([]interface{}{authority, "1", controls, scopes})
		canonical, err := mfjson.Canonicalize(projection)
		if err != nil {
			t.Fatal(err)
		}
		wire, _ := json.Marshal(map[string]interface{}{"projection_schema": upstreamhealth.ProjectionSchema, "authority_id": authority, "revision": "1", "state_digest": bodybasis.DigestBytes(canonical), "credentials": controls, "scopes": scopes, "observed_at": now.Add(-time.Second).Format(time.RFC3339Nano), "valid_until": now.Add(20 * time.Second).Format(time.RFC3339Nano), "complete": true})
		read := upstreamhealth.DecodeSnapshot(wire, upstreamhealth.Config{Prefix: "modelfusion", AuthorityID: authority, MinimumRevision: "0"}, now)
		eligible, _ := eligibleManagedHealthMembers(client, read, members, now)
		if len(eligible) != 1 || eligible[0].member.credential.CredentialRef != "credential-2" {
			t.Fatal("disabled or old-generation member survived qualification")
		}
	}
}
