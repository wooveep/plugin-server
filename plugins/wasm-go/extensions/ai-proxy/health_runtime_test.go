package main

import (
	"bytes"
	"encoding/json"
	"strconv"
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

// Uses actual SDK host callbacks for Redis read/atomic consume, original attempt
// registration and the sole provider send. No Console callout is acknowledged.
// Native Redis state transitions have their separate owned-service batch.
func TestManagedHealthUsesDurableStateAndConsumesRecoveryBeforeSingleSend(t *testing.T) {
	for _, scenario := range []struct {
		name                   string
		confirmed, agedHealthy bool
	}{{"unknown_consume", false, false}, {"committed_consume", true, false}, {"healthy_console_unavailable", false, true}} {
		t.Run(scenario.name, func(t *testing.T) {
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
			if scenario.agedHealthy {
				scopes[0].BlockedUntil = nil
				projection, _ = json.Marshal([]interface{}{authority, "9", credentials, scopes})
				canonical, err = mfjson.Canonicalize(projection)
				if err != nil {
					t.Fatal(err)
				}
				digest = bodybasis.DigestBytes(canonical)
				observed, expires = now.Add(-24*time.Hour).Format(time.RFC3339Nano), now.Add(-23*time.Hour).Format(time.RFC3339Nano)
			}
			body, _ := json.Marshal(map[string]interface{}{"projection_schema": upstreamhealth.ProjectionSchema, "authority_id": authority, "revision": "9", "state_digest": digest,
				"credentials": credentials, "scopes": scopes, "observed_at": observed, "valid_until": expires, "complete": true})
			values := []resp.Value{}
			for i, value := range []string{authority, "9", digest, string(body), observed, expires, upstreamhealth.ProjectionSchema, "true"} {
				values = append(values, resp.StringValue(upstreamhealth.HashFields()[i]), resp.StringValue(value))
			}
			response := []resp.Value{resp.ArrayValue(values), resp.ArrayValue([]resp.Value{resp.StringValue(strconv.FormatInt(now.Unix(), 10)), resp.StringValue(strconv.FormatInt(int64(now.Nanosecond()/1000), 10))})}
			var encoded bytes.Buffer
			if err := resp.NewWriter(&encoded).WriteArray(response); err != nil {
				t.Fatal(err)
			}
			host.CallOnRedisCallResponse(lookups[0].CalloutID, 0, encoded.Bytes())
			providerCount := func() int {
				n := 0
				for _, call := range host.GetCalloutAttributesFromContext(h.id) {
					if call.Upstream == "outbound|80||managed-provider.static" {
						n++
					}
				}
				return n
			}
			if scenario.agedHealthy {
				if providerCount() != 1 || len(host.GetCalloutAttributesFromContext(h.id)) != 1 || host.GetSentLocalResponse(h.id) != nil {
					t.Fatal("healthy persisted state called Console or blocked the provider send")
				}
				return
			}
			lookups = host.GetRedisCalloutAttributesFromContext(h.id)
			if len(lookups) != 1 || len(host.GetCalloutAttributesFromContext(h.id)) != 0 {
				t.Fatal("recovery requested Console or sent before fresh Redis qualification")
			}
			host.CallOnRedisCallResponse(lookups[0].CalloutID, 0, encoded.Bytes())
			mutations := host.GetRedisCalloutAttributesFromContext(h.id)
			if len(mutations) != 1 || providerCount() != 0 {
				t.Fatal("provider preceded original atomic consume")
			}
			command, _, err := resp.NewReader(bytes.NewReader(mutations[0].Query)).ReadValue()
			if err != nil {
				t.Fatal(err)
			}
			args := command.Array()
			if len(args) != 16 || args[0].String() != "EVAL" {
				t.Fatal("recovery did not use its existing Redis/Stream script")
			}
			var event upstreamhealth.RuntimeEvent
			if err = json.Unmarshal([]byte(args[10].String()), &event); err != nil {
				t.Fatal(err)
			}
			if event.Claim == nil || event.Claim.Request.OwnerInstance != managedGatewayOwnerInstance() || event.Claim.Status != "consumed" || len(event.Claim.ConsumedSelection) == 0 {
				t.Fatal("Redis consume lost original startup/final selection")
			}
			reply := []resp.Value{resp.StringValue("committed"), resp.StringValue(args[11].String())}
			if !scenario.confirmed {
				reply[0] = resp.StringValue("unavailable")
				reply[1] = resp.StringValue("")
			}
			encoded.Reset()
			if err = resp.NewWriter(&encoded).WriteArray(reply); err != nil {
				t.Fatal(err)
			}
			host.CallOnRedisCallResponse(mutations[0].CalloutID, 0, encoded.Bytes())
			if !scenario.confirmed {
				if providerCount() != 0 || host.GetSentLocalResponse(h.id) == nil {
					t.Fatal("unknown atomic consume sent or failed to reject")
				}
				return
			}
			if providerCount() != 1 || len(host.GetCalloutAttributesFromContext(h.id)) != 1 {
				t.Fatal("committed Redis consume did not allow exactly one provider without Console")
			}
			state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
			var final struct {
				Digest string `json:"selection_digest"`
			}
			if err = json.Unmarshal(event.Claim.ConsumedSelection, &final); err != nil {
				t.Fatal(err)
			}
			if state.original.SelectionDigest != final.Digest || state.health.claimID == nil || *state.health.claimID != event.Claim.ClaimID {
				t.Fatal("actual attempt lost its consumed selection")
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
