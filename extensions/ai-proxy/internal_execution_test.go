package main

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/higress-group/wasm-go/pkg/ai/upstreamhealth"
	"github.com/tidwall/resp"
)

const internalOriginalCall = "50000000-0000-4000-8000-000000000009"
const internalOriginalKey = "20000000-0000-4000-8000-000000000009"
const internalOriginalTenant = "30000000-0000-4000-8000-000000000009"

func managedInternalFixture(t *testing.T, embedding bool) map[string]interface{} {
	t.Helper()
	fixture := managedPoolFixture(t)
	settings := fixture["managed_execution"].(map[string]interface{})
	release := fixture["_modelfusion_execution"].(map[string]interface{})
	release["route_id"] = "60000000-0000-4000-8000-000000000009"
	descriptor := release["descriptor"].(map[string]interface{})
	settings["call_identity_source"] = executionfacts.CallIdentitySourceInternal
	if embedding {
		settings["input_protocol"], settings["upstream_protocol"], settings["upstream_path"] = managedProtocolEmbeddings, managedProtocolEmbeddings, "/v1/embeddings"
		descriptor["adapter_ref"] = managedEmbeddingAdapter
		descriptor["transport_contract"].(map[string]interface{})["path_adapter_ref"] = managedEmbeddingAdapter
		descriptor["capabilities"] = map[string]managedCapability{executionfacts.NativeEmbeddingOperation: {SendMode: "single_send", ResponseClassifierRef: managedEmbeddingClassifier, MaxAttempts: 1}}
		for _, target := range settings["upstream_health"].(map[string]interface{})["targets"].([]interface{}) {
			target.(map[string]interface{})["operation_kind"] = executionfacts.NativeEmbeddingOperation
		}
	}
	digest, err := managedDescriptorDigest(managedFixtureJSON(t, descriptor))
	if err != nil {
		t.Fatal(err)
	}
	descriptor["descriptor_digest"] = digest
	return fixture
}

func startManagedInternal(t *testing.T, h *managedHost, original, mapped []byte) bodybasis.Snapshot {
	t.Helper()
	basis, err := bodybasis.Capture(original)
	if err != nil {
		t.Fatal(err)
	}
	headers := [][2]string{{":method", "POST"}, {":path", "/internal-worker-route"}, {":authority", "gateway.example"}, {"content-type", "application/json"},
		{managedInternalCallHeader, internalOriginalCall}, {balanceidentity.APIKeyHeader, internalOriginalKey}, {balanceidentity.GenerationHeader, "9007199254740993"},
		{managedCallHeader, "client-fake-call"}, {managedAdmissionReleaseHeader, "client-fake-release"}, {"x-modelfusion-cost-owner", "client-fake-owner"}}
	headers = append(headers, basis.Headers()...)
	if h.host.CallOnRequestHeaders(h.id, headers, false) != types.HeaderStopIteration {
		t.Fatal("internal route did not wait for complete body")
	}
	if h.host.CallOnRequestBody(h.id, mapped, true) != types.ActionPause {
		t.Fatal("internal route escaped to router")
	}
	return basis
}

func internalCheckReceipt(t *testing.T, h *managedHost, basis bodybasis.Snapshot) managedInternalReceipt {
	t.Helper()
	state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
	return managedInternalReceipt{Allowed: true, CallID: internalOriginalCall, ReleaseID: state.config.release.Descriptor.ReleaseID, RouteID: state.config.release.RouteID, APIKeyID: internalOriginalKey, CredentialGeneration: "9007199254740993",
		Caller:        managedInternalCaller{BillingMode: "internal_cost", TenantID: internalOriginalTenant, PrincipalID: "service/ai-memory/40000000-0000-4000-8000-000000000009", CostOwner: executionfacts.CostOwner{Kind: "tenant", OwnerID: internalOriginalTenant, SourceJobID: "original-memory-job"}},
		OperationKind: managedOperation(state.config), PublicModel: state.config.release.PublicModel, BusinessOccurredAt: "2026-10-29T00:00:01Z", OriginalBodyDigest: basis.Digest(), TargetIDs: []string{state.config.release.Descriptor.TargetID}, ValidUntil: time.Now().UTC().Add(5 * time.Second).Format(time.RFC3339Nano)}
}

func respondManagedInternalHealth(t *testing.T, h *managedHost, enabled bool) {
	t.Helper()
	lookups := h.host.GetRedisCalloutAttributesFromContext(h.id)
	if len(lookups) != 1 {
		t.Fatalf("health lookups %d", len(lookups))
	}
	state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
	members, err := managedHealthMembers(state.sourceConfig)
	if err != nil {
		t.Fatal(err)
	}
	credentials := []upstreamhealth.CredentialControl{}
	scopes := []upstreamhealth.ScopeState{}
	seen := map[string]bool{}
	for _, member := range members {
		credentials = append(credentials, upstreamhealth.CredentialControl{Identity: upstreamhealth.CredentialIdentity{CredentialRef: member.target.CredentialRef, ProviderAccountID: member.target.ProviderAccountID, OwnerKind: "platform", OwnerID: "platform-1"}, Generation: member.target.CredentialGeneration, Revision: "2", Enabled: enabled})
		for _, id := range member.target.ScopeIDs {
			if seen[id] {
				continue
			}
			seen[id] = true
			region, model, operation := member.target.RegionEndpointRef, member.target.UpstreamModel, member.target.OperationKind
			scopes = append(scopes, upstreamhealth.ScopeState{Identity: upstreamhealth.ScopeIdentity{ScopeID: id, Kind: "model_operation", ProviderAccountID: member.target.ProviderAccountID, RegionEndpointRef: &region, UpstreamModel: &model, OperationKind: &operation}, Revision: "2", FailureGeneration: "0", RecoveryEpoch: "0"})
		}
	}
	authority := state.config.health.settings.Snapshot.AuthorityID
	projection, _ := json.Marshal([]interface{}{authority, "9", credentials, scopes})
	canonical, err := mfjson.Canonicalize(projection)
	if err != nil {
		t.Fatal(err)
	}
	digest := bodybasis.DigestBytes(canonical)
	now := time.Now().UTC()
	observed, expires := now.Add(-time.Second).Format(time.RFC3339Nano), now.Add(time.Minute).Format(time.RFC3339Nano)
	body, _ := json.Marshal(map[string]interface{}{"projection_schema": upstreamhealth.ProjectionSchema, "authority_id": authority, "revision": "9", "state_digest": digest, "credentials": credentials, "scopes": scopes, "observed_at": observed, "valid_until": expires, "complete": true})
	values := []resp.Value{}
	for _, value := range []string{authority, "9", digest, string(body), observed, expires, upstreamhealth.ProjectionSchema, "true"} {
		values = append(values, resp.StringValue(value))
	}
	var encoded bytes.Buffer
	if err := resp.NewWriter(&encoded).WriteArray(values); err != nil {
		t.Fatal(err)
	}
	h.host.CallOnRedisCallResponse(lookups[0].CalloutID, 0, encoded.Bytes())
}

func TestManagedInternalCheckPrecedesHealthAndSingleOriginalProviderSend(t *testing.T) {
	for _, embedding := range []bool{false, true} {
		name := "text"
		if embedding {
			name = "embedding"
		}
		t.Run(name, func(t *testing.T) {
			h := newManagedTextHost(t, managedInternalFixture(t, embedding))
			original, mapped, response := []byte(managedOriginalBody), []byte(managedMappedBody), []byte(managedSuccessfulBody)
			if embedding {
				original = []byte(`{"model":"public-model","input":"hello"}`)
				mapped = []byte(`{"model":"upstream-model","input":"hello"}`)
				response = []byte(`{"model":"upstream-model","data":[{"index":0,"embedding":[0.25,-0.125]}],"usage":{"prompt_tokens":9007199254740993,"total_tokens":9007199254740993}}`)
			}
			basis := startManagedInternal(t, h, original, mapped)
			calls := h.host.GetCalloutAttributesFromContext(h.id)
			if len(calls) != 1 || len(h.host.GetRedisCalloutAttributesFromContext(h.id)) != 0 || len(h.records(t)) != 0 {
				t.Fatal("provider/history/health preceded the registered job check")
			}
			path, _ := managedSingleHeader(calls[0].Headers, ":path")
			if path != managedInternalCheckPath {
				t.Fatal("wrong native Console namespace", path)
			}
			var request managedInternalRequest
			if decodeManagedJSON(calls[0].Body, &request, true) != nil || request.CallRef != internalOriginalCall || request.APIKeyID != internalOriginalKey || request.CredentialGeneration != "9007199254740993" || request.OriginalBodyDigest != basis.Digest() {
				t.Fatal("RPC replaced the original service key/body/call")
			}
			receipt := internalCheckReceipt(t, h, basis)
			raw, _ := json.Marshal(receipt)
			h.host.CallOnHttpCallResponse(calls[0].CalloutID, [][2]string{{":status", "200"}}, nil, raw)
			if len(h.host.GetCalloutAttributesFromContext(h.id)) != 0 || len(h.records(t)) != 0 {
				t.Fatal("job check bypassed original HEALTH qualification")
			}
			respondManagedInternalHealth(t, h, true)
			calls = h.host.GetCalloutAttributesFromContext(h.id)
			if len(calls) != 1 || calls[0].Upstream != "outbound|80||managed-provider.static" || !bytes.Equal(calls[0].Body, mapped) {
				t.Fatal("actual single send changed source input/target")
			}
			state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
			if state.original.CallID != receipt.CallID || state.original.OccurredAt != receipt.BusinessOccurredAt || state.original.BillingMode != "internal_cost" || state.original.AccountID != "" || state.original.SalesAssetID != "" || state.original.CostOwner.SourceJobID != receipt.Caller.CostOwner.SourceJobID {
				t.Fatal("actual attempt lost original internal ownership")
			}
			records := h.records(t)
			if len(records) != 1 || records[0].messageType != "execution_registration" {
				t.Fatal("pre-send registration missing")
			}
			var registration map[string]json.RawMessage
			_ = json.Unmarshal(records[0].raw, &registration)
			if string(registration["schema_version"]) != `"execution-registration.v2"` || registration["account_id"] != nil || len(registration) != 12 {
				t.Fatal("service registration invented an account or wrong carrier")
			}
			h.host.CallOnHttpCallResponse(calls[0].CalloutID, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, nil, response)
			records = h.records(t)
			if len(records) != 2 {
				t.Fatal("proxy must only write one registration and one actual attempt")
			}
			var fact executionfacts.Envelope
			if json.Unmarshal(records[1].raw, &fact) != nil || fact.Kind != "upstream_attempt" || fact.Acceptance != "accepted" || fact.ExecutionResult != "succeeded" || fact.CredentialRef != state.config.release.CredentialRef {
				t.Fatal("actual attempt evidence incomplete")
			}
			if embedding && (fact.SchemaVersion != "billing-facts.v2" || fact.Usage.Quantities["input"] != "9007199254740993" || len(fact.Usage.Quantities) != 1) {
				t.Fatal("embedding token precision/version changed")
			}
			local := h.host.GetSentLocalResponse(h.id)
			if local == nil || local.StatusCode != 200 || !bytes.Equal(local.Data, response) {
				t.Fatal("original provider response was changed")
			}
			bridge, _ := managedSingleHeader(local.Headers, executionfacts.DeliveryBridgeHeader)
			var delivery executionfacts.Snapshot
			var err error
			if embedding {
				delivery, err = executionfacts.DecodeEmbeddingDeliveryBridge(bridge)
			} else {
				delivery, err = executionfacts.DecodeDeliveryBridge(bridge)
			}
			if err != nil || delivery.Envelope().CallID != receipt.CallID || delivery.Envelope().AccountID != "" || delivery.Envelope().SalesAssetID != "" || delivery.Envelope().CostOwner.SourceJobID != receipt.Caller.CostOwner.SourceJobID {
				t.Fatal("delivery bridge lost original service/job identity", err)
			}
		})
	}
}

func TestManagedInternalUnconfirmedOrChangedCheckNeverSends(t *testing.T) {
	for _, mode := range []string{"unknown", "wrong_key", "wrong_generation", "wrong_call", "wrong_body", "wrong_target", "expired", "customer", "extra_owner"} {
		t.Run(mode, func(t *testing.T) {
			h := newManagedTextHost(t, managedInternalFixture(t, false))
			basis := startManagedInternal(t, h, []byte(managedOriginalBody), []byte(managedMappedBody))
			calls := h.host.GetCalloutAttributesFromContext(h.id)
			receipt := internalCheckReceipt(t, h, basis)
			switch mode {
			case "wrong_key":
				receipt.APIKeyID = "20000000-0000-4000-8000-000000000008"
			case "wrong_generation":
				receipt.CredentialGeneration = "9007199254740994"
			case "wrong_call":
				receipt.CallID = "50000000-0000-4000-8000-000000000008"
			case "wrong_body":
				receipt.OriginalBodyDigest = "sha256:" + string(bytes.Repeat([]byte("a"), 64))
			case "wrong_target":
				receipt.TargetIDs = []string{"foreign-target"}
			case "expired":
				receipt.ValidUntil = time.Now().UTC().Add(-time.Second).Format(time.RFC3339Nano)
			case "customer":
				receipt.Caller.BillingMode = "customer"
			}
			raw, _ := json.Marshal(receipt)
			if mode == "unknown" {
				raw = []byte(`{"allowed":`)
			}
			if mode == "extra_owner" {
				raw = append(raw[:len(raw)-1], []byte(`,"cost_owner":{"kind":"platform"}}`)...)
			}
			h.host.CallOnHttpCallResponse(calls[0].CalloutID, [][2]string{{":status", "200"}}, nil, raw)
			if len(h.host.GetCalloutAttributesFromContext(h.id)) != 0 || len(h.host.GetRedisCalloutAttributesFromContext(h.id)) != 0 || len(h.records(t)) != 0 || h.host.GetSentLocalResponse(h.id) == nil {
				t.Fatal("unconfirmed job authority reached HEALTH/provider/facts")
			}
		})
	}
}

func TestManagedInternalHealthDenialAndMissingCapabilityStayClosed(t *testing.T) {
	t.Run("disabled_pool", func(t *testing.T) {
		h := newManagedTextHost(t, managedInternalFixture(t, true))
		basis := startManagedInternal(t, h, []byte(`{"model":"public-model","input":"hello"}`), []byte(`{"model":"upstream-model","input":"hello"}`))
		calls := h.host.GetCalloutAttributesFromContext(h.id)
		raw, _ := json.Marshal(internalCheckReceipt(t, h, basis))
		h.host.CallOnHttpCallResponse(calls[0].CalloutID, [][2]string{{":status", "200"}}, nil, raw)
		respondManagedInternalHealth(t, h, false)
		if len(h.host.GetCalloutAttributesFromContext(h.id)) != 0 || len(h.records(t)) != 0 {
			t.Fatal("service call bypassed disabled health control")
		}
	})
	for _, mode := range []string{"missing_health", "embedding_customer", "embedding_retry", "embedding_wrong_classifier"} {
		t.Run(mode, func(t *testing.T) {
			fixture := managedInternalFixture(t, true)
			settings := fixture["managed_execution"].(map[string]interface{})
			descriptor := fixture["_modelfusion_execution"].(map[string]interface{})["descriptor"].(map[string]interface{})
			switch mode {
			case "missing_health":
				delete(settings, "upstream_health")
			case "embedding_customer":
				settings["call_identity_source"] = "admission"
			case "embedding_retry":
				settings["text_execution"] = managedTextSettings{Retry: &managedTextRetry{MaxRetries: 1, RetryTimeoutMS: 1000, RetryOnStatus: []string{"5.*"}}}
			case "embedding_wrong_classifier":
				descriptor["adapter_ref"] = "text-adapter"
			}
			if _, err := decodeManagedPoolFixture(t, fixture); err == nil {
				t.Fatal("incompatible native capability accepted")
			}
		})
	}
}
