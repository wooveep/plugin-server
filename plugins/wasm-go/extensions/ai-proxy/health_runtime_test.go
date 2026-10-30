package main

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/higress-group/wasm-go/pkg/ai/upstreamhealth"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/resp"
)

// Uses the real Wasm host callbacks across Redis lookup, Claim, final Consume,
// original attempt registration and the existing sole provider send. It proves
// native wiring only; it is not live Console/Redis/provider acceptance evidence.
func TestManagedHealthConsumesOriginalRecoveryBeforeSingleSend(t *testing.T) {
	for _, confirmed := range []bool{false, true} {
		name := "unknown_consume"
		if confirmed {
			name = "committed_consume"
		}
		t.Run(name, func(t *testing.T) {
			fixture := managedConfigFixture()
			settings := fixture["managed_execution"].(managedSettings)
			settings.UpstreamHealth = &managedHealthSettings{RuntimeConfigID: "release-1", ConfigRevision: "1",
				Snapshot: managedHealthSnapshot{Prefix: "modelfusion", AuthorityID: "10000000-0000-0000-0000-000000000001", MinimumRevision: "0"},
				Target: upstreamhealth.TargetReference{TargetID: "target-1", BindingID: "binding-1", ProviderAccountID: "purchase-1", RegionEndpointRef: "region-1",
					UpstreamModel: "upstream-model", OperationKind: "text_generate", CredentialRef: "credential-1", CredentialGeneration: "1", ScopeIDs: []string{"model-scope"}},
				Console: managedHealthConsole{ServiceName: "console.static", ServicePort: 8080, Host: "console.internal", AuthToken: "controlled-service-token", TimeoutMS: 1000}}
			fixture["managed_execution"] = settings
			h := &managedHost{}
			vm := wrapper.NewCommonVmCtx("health-managed-text",
				wrapper.PrePluginStartOrReload[config.PluginConfig](initializeManagedHealthRuntime),
				wrapper.ParseConfig(func(value gjson.Result, cfg *config.PluginConfig) error {
					cfg.FromJson(value)
					if err := cfg.Validate(); err != nil {
						return err
					}
					if err := cfg.Complete(); err != nil {
						return err
					}
					if err := configureManagedExecution(value, cfg); err != nil {
						return err
					}
					h.cfg = *cfg
					return nil
				}),
				wrapper.ProcessRequestHeaders(func(ctx wrapper.HttpContext, cfg config.PluginConfig) types.Action {
					h.ctx = ctx
					action, _ := onManagedRequestHeaders(ctx, cfg)
					return action
				}),
				wrapper.ProcessRequestBody(func(ctx wrapper.HttpContext, cfg config.PluginConfig, body []byte) types.Action {
					action, _ := onManagedRequestBody(ctx, cfg, body)
					return action
				}),
				wrapper.ProcessStreamDone(onManagedStreamDone))
			host, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption().WithVMContext(vm).WithPluginConfiguration(managedFixtureJSON(t, fixture)))
			t.Cleanup(reset)
			h.host = host
			host.RegisterForeignFunction("get_log_level", func([]byte) []byte { return []byte{0, 0, 0, 0} })
			if err := host.SetProperty([]string{"node", "id"}, []byte("original-gateway-node")); err != nil {
				t.Fatal(err)
			}
			if host.StartVM() != types.OnVMStartStatusOK || host.StartPlugin() != types.OnPluginStartStatusOK {
				t.Fatal("managed health host did not start")
			}
			h.id = host.InitializeHttpContext()
			h.start(t, []byte(managedOriginalBody), []byte(managedMappedBody))
			lookups := host.GetRedisCalloutAttributesFromContext(h.id)
			if len(lookups) != 1 || len(host.GetCalloutAttributesFromContext(h.id)) != 0 {
				t.Fatal("provider send preceded health lookup")
			}
			now := time.Now().UTC()
			blocked := now.Add(-time.Second).Format(time.RFC3339Nano)
			region, model, operation := "region-1", "upstream-model", "text_generate"
			credentials := []upstreamhealth.CredentialControl{{Identity: upstreamhealth.CredentialIdentity{CredentialRef: "credential-1", ProviderAccountID: "purchase-1", OwnerKind: "platform", OwnerID: "platform-1"}, Generation: "1", Revision: "2", Enabled: true}}
			scopes := []upstreamhealth.ScopeState{{Identity: upstreamhealth.ScopeIdentity{ScopeID: "model-scope", Kind: "model_operation", ProviderAccountID: "purchase-1", RegionEndpointRef: &region, UpstreamModel: &model, OperationKind: &operation}, Revision: "4", FailureGeneration: "2", RecoveryEpoch: "3", BlockedUntil: &blocked}}
			authority := settings.UpstreamHealth.Snapshot.AuthorityID
			projection, _ := json.Marshal([]interface{}{authority, "9", credentials, scopes})
			canonical, err := mfjson.Canonicalize(projection)
			if err != nil {
				t.Fatal(err)
			}
			digest := bodybasis.DigestBytes(canonical)
			observed, expires := now.Add(-time.Second).Format(time.RFC3339Nano), now.Add(time.Minute).Format(time.RFC3339Nano)
			body, _ := json.Marshal(map[string]interface{}{"projection_schema": upstreamhealth.ProjectionSchema, "authority_id": authority, "revision": "9", "state_digest": digest,
				"credentials": credentials, "scopes": scopes, "observed_at": observed, "valid_until": expires, "complete": true})
			values := []resp.Value{}
			for _, value := range []string{authority, "9", digest, string(body), observed, expires, upstreamhealth.ProjectionSchema, "true"} {
				values = append(values, resp.StringValue(value))
			}
			var encoded bytes.Buffer
			if err := resp.NewWriter(&encoded).WriteArray(values); err != nil {
				t.Fatal(err)
			}
			host.CallOnRedisCallResponse(lookups[0].CalloutID, 0, encoded.Bytes())
			findCall := func(path string) proxytest.HttpCalloutAttribute {
				for _, call := range host.GetCalloutAttributesFromContext(h.id) {
					for _, header := range call.Headers {
						if header[0] == ":path" && header[1] == path {
							return call
						}
					}
				}
				t.Fatalf("missing native callout %s", path)
				return proxytest.HttpCalloutAttribute{}
			}
			providerCount := func() int {
				n := 0
				for _, call := range host.GetCalloutAttributesFromContext(h.id) {
					if call.Upstream == "outbound|80||managed-provider.static" {
						n++
					}
				}
				return n
			}
			claimCall := findCall(upstreamhealth.RecoveryClaimPath)
			if providerCount() != 0 {
				t.Fatal("allocated claim preceded provider send gate")
			}
			var original upstreamhealth.ClaimRequest
			if err := json.Unmarshal(claimCall.Body, &original); err != nil {
				t.Fatal(err)
			}
			if original.OwnerInstance != managedGatewayOwnerInstance() || original.CandidateDigest != original.Candidate.Digest() {
				t.Fatal("candidate lost original startup or digest")
			}
			claimID := "20000000-0000-0000-0000-000000000001"
			claim := upstreamhealth.RecoveryClaim{ClaimID: claimID, Request: original, Actor: "service/original-proxy", CredentialGeneration: "1", CredentialControlRevision: "2", CredentialRef: "credential-1",
				Scopes: []upstreamhealth.ClaimedScope{{ScopeID: "model-scope", Epoch: "4", FailureGeneration: "2"}}, CreatedAt: now.Format(time.RFC3339Nano), ExpiresAt: expires, Status: "allocated"}
			raw, _ := json.Marshal(upstreamhealth.ClaimReceipt{Decision: "granted", Claim: &claim})
			host.CallOnHttpCallResponse(claimCall.CalloutID, [][2]string{{":status", "200"}}, nil, raw)
			consumeCall := findCall(upstreamhealth.RecoveryClaimPath + "/" + claimID + "/consume")
			if providerCount() != 0 {
				t.Fatal("provider send preceded committed final consume")
			}
			var consume struct {
				Request   upstreamhealth.ClaimRequest `json:"request"`
				Selection json.RawMessage             `json:"selection"`
			}
			if err := json.Unmarshal(consumeCall.Body, &consume); err != nil {
				t.Fatal(err)
			}
			if !confirmed {
				host.CallOnHttpCallResponse(consumeCall.CalloutID, [][2]string{{":status", "200"}}, nil, []byte(`{"status":`))
				if providerCount() != 0 || host.GetSentLocalResponse(h.id) == nil {
					t.Fatal("unknown Consume sent or failed to reject")
				}
				return
			}
			claim.Status, claim.ConsumedSelection = "consumed", consume.Selection
			raw, _ = json.Marshal(claim)
			host.CallOnHttpCallResponse(consumeCall.CalloutID, [][2]string{{":status", "200"}}, nil, raw)
			if providerCount() != 1 {
				t.Fatal("first committed Consume did not permit exactly one original send")
			}
			state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
			var final map[string]json.RawMessage
			if err := json.Unmarshal(consume.Selection, &final); err != nil {
				t.Fatal(err)
			}
			var finalDigest string
			_ = json.Unmarshal(final["selection_digest"], &finalDigest)
			if state.original.SelectionDigest != finalDigest || state.health.claimID == nil || *state.health.claimID != claimID {
				t.Fatal("actual attempt lost consumed final selection")
			}
		})
	}
}

func TestManagedProviderDeadlineRetainsActualHeaderAndOriginalResponseTime(t *testing.T) {
	for _, raw := range []string{"600", " 600 ", "Thu, 29 Oct 2026 00:10:01 GMT"} {
		deadline, evidence := managedProviderDeadline(raw, "2026-10-29T00:00:01Z")
		if deadline != "2026-10-29T00:10:01Z" || evidence == nil || *evidence != raw {
			t.Fatalf("original provider deadline/header changed: %q %q %#v", raw, deadline, evidence)
		}
	}
	for _, raw := range []string{"", "-1", "provider-secret", "18446744073709551615"} {
		deadline, evidence := managedProviderDeadline(raw, "2026-10-29T00:00:01Z")
		if deadline != "" || evidence == nil || *evidence != "provider_retry_after_invalid" {
			t.Fatal("invalid provider deadline was guessed, shortened or exposed")
		}
	}
}
