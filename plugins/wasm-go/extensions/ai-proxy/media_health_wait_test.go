package main

import (
	"encoding/json"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/higress-group/wasm-go/pkg/ai/upstreamhealth"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/resp"
	"strconv"
	"testing"
	"time"
)

// The notifying HTTP context ends while health is blocked. Only the live root
// tick and Redis may subsequently send the same original job; no Console exists
// in this transport fixture. The source/classifier has its separate fact tests.
func TestManagedMediaHealthWaitSurvivesNoticeEndAndKeepsOriginalTicket(t *testing.T) {
	for _, resume := range []bool{true, false} {
		t.Run(map[bool]string{true: "health_recovers", false: "original_deadline_expires"}[resume], func(t *testing.T) {
			job, _ := managedMediaHostJob(t, true)
			runtime := job.runtime
			clock := time.Now().UTC()
			runtime.now = func() time.Time { return clock }
			runtime.owner = "original-node/original-VM"
			job.key = job.notice.TicketKey
			runtime.jobs = map[string]*managedMediaJob{job.key: job}
			var descriptor struct {
				TargetID  string `json:"target_id"`
				BindingID string `json:"binding_id"`
				Account   string `json:"provider_account_id"`
				Region    string `json:"region_endpoint_ref"`
				Model     string `json:"upstream_model"`
			}
			if err := json.Unmarshal(job.executor.Descriptor(), &descriptor); err != nil {
				t.Fatal(err)
			}
			operation := "image_generate"
			authority := "10000000-0000-0000-0000-000000000001"
			target := upstreamhealth.TargetReference{TargetID: descriptor.TargetID, BindingID: descriptor.BindingID,
				ProviderAccountID: descriptor.Account, RegionEndpointRef: descriptor.Region, UpstreamModel: descriptor.Model,
				OperationKind: operation, CredentialRef: "original-credential", CredentialGeneration: "1", ScopeIDs: []string{"media-scope"}}
			job.archive.Native.Health = &managedHealthSettings{RuntimeConfigID: "original-runtime", ConfigRevision: "1",
				Snapshot: managedHealthSnapshot{Prefix: "media-wait", AuthorityID: authority, MinimumRevision: "0"}}
			job.archive.HealthTarget = target
			previousOwner, previousAuthorities := managedHealthStartup.owner, managedHealthStartup.byAuthority
			managedHealthStartup.owner = runtime.owner
			managedHealthStartup.byAuthority = map[string]*managedHealthShared{}
			t.Cleanup(func() {
				managedHealthStartup.owner, managedHealthStartup.byAuthority = previousOwner, previousAuthorities
			})
			initial := clock
			healthy := false
			reads := 0
			originalRedis := runtime.redis
			runtime.redis = managedMediaTestRedis{command: originalRedis.Command, eval: func(_ string, _ int, _ []interface{}, _ []interface{}, callback wrapper.RedisResponseCallback) error {
				reads++
				revision := upstreamhealth.Revision("9")
				scopeRevision := upstreamhealth.Revision("4")
				blocked := initial.Add(30 * time.Second).Format(time.RFC3339Nano)
				scopes := []upstreamhealth.ScopeState{{Identity: upstreamhealth.ScopeIdentity{ScopeID: "media-scope", Kind: "model_operation",
					ProviderAccountID: descriptor.Account, RegionEndpointRef: &descriptor.Region, UpstreamModel: &descriptor.Model, OperationKind: &operation},
					Revision: scopeRevision, FailureGeneration: "2", RecoveryEpoch: "3", BlockedUntil: &blocked}}
				if healthy {
					revision = "10"
					scopes[0].Revision = "5"
					scopes[0].BlockedUntil = nil
				}
				credentials := []upstreamhealth.CredentialControl{{Identity: upstreamhealth.CredentialIdentity{CredentialRef: "original-credential",
					ProviderAccountID: descriptor.Account, OwnerKind: "platform", OwnerID: "platform-1"}, Generation: "1", Revision: "2", Enabled: true}}
				authorityJSON, _ := json.Marshal(authority)
				revisionJSON, _ := json.Marshal(revision)
				credentialsJSON, _ := json.Marshal(credentials)
				scopesJSON, _ := json.Marshal(scopes)
				projection, _ := json.Marshal([]json.RawMessage{authorityJSON, revisionJSON, credentialsJSON, scopesJSON})
				canonical, err := mfjson.Canonicalize(projection)
				if err != nil {
					t.Fatal(err)
				}
				digest := bodybasis.DigestBytes(canonical)
				observed, expires := clock.Add(-time.Second).Format(time.RFC3339Nano), clock.Add(time.Minute).Format(time.RFC3339Nano)
				body, err := json.Marshal(struct {
					Schema      string                             `json:"projection_schema"`
					Authority   string                             `json:"authority_id"`
					Revision    upstreamhealth.Revision            `json:"revision"`
					Digest      string                             `json:"state_digest"`
					Credentials []upstreamhealth.CredentialControl `json:"credentials"`
					Scopes      []upstreamhealth.ScopeState        `json:"scopes"`
					Observed    string                             `json:"observed_at"`
					Expires     string                             `json:"valid_until"`
					Complete    bool                               `json:"complete"`
				}{upstreamhealth.ProjectionSchema, authority, revision, digest, credentials, scopes, observed, expires, true})
				if err != nil {
					t.Fatal(err)
				}
				fields := []resp.Value{}
				for i, value := range []string{authority, string(revision), digest, string(body), observed, expires, upstreamhealth.ProjectionSchema, "true"} {
					fields = append(fields, resp.StringValue(upstreamhealth.HashFields()[i]), resp.StringValue(value))
				}
				callback(resp.ArrayValue([]resp.Value{resp.ArrayValue(fields), resp.ArrayValue([]resp.Value{
					resp.StringValue(strconv.FormatInt(clock.Unix(), 10)), resp.StringValue(strconv.FormatInt(int64(clock.Nanosecond()/1000), 10)),
				})}))
				return nil
			}}
			runtime.provider = runtime.providerCall
			type tickConfig struct{}
			vm := wrapper.NewCommonVmCtx("media-waiting-original", wrapper.ParseConfig(func(gjson.Result, *tickConfig) error {
				wrapper.RegisterTickFunc(100, runtime.tick)
				return nil
			}), wrapper.ProcessRequestHeaders(func(wrapper.HttpContext, tickConfig) types.Action {
				job.qualify()
				_ = proxywasm.SendHttpResponse(202, nil, []byte(`{"accepted":true}`), -1)
				return types.ActionPause
			}))
			host, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption().WithVMContext(vm).WithPluginConfiguration([]byte(`{}`)))
			t.Cleanup(reset)
			host.RegisterForeignFunction("get_log_level", func([]byte) []byte { return []byte{0, 0, 0, 0} })
			if host.StartVM() != types.OnVMStartStatusOK || host.StartPlugin() != types.OnPluginStartStatusOK {
				t.Fatal("media root did not start")
			}
			id := host.InitializeHttpContext()
			host.CallOnRequestHeaders(id, [][2]string{{":method", "POST"}, {":path", managedMediaDispatchPath}, {":authority", "gateway.example"}}, true)
			if job.stage != "waiting_health" || reads != 1 || len(host.GetCalloutAttributesFromContext(id)) != 0 {
				t.Fatal("blocked job escaped its original wait")
			}
			host.CompleteHttpContext(id)
			if resume {
				healthy = true
				clock = initial.Add(31 * time.Second)
			} else {
				clock = job.notice.ExpiresAt.Add(time.Second)
			}
			host.Tick()
			calls := host.GetCalloutAttributesFromContext(proxytest.PluginContextID)
			if resume {
				if len(calls) != 1 || job.request.AttemptID() != "original-create" || job.notice.TicketKey != "mf:media:ticket:original" || reads != 2 {
					t.Fatal("health recovery required another notice, ticket or attempt")
				}
				host.Tick()
				after := host.GetCalloutAttributesFromContext(proxytest.PluginContextID)
				if len(after) != 1 || after[0].CalloutID != calls[0].CalloutID || reads != 2 {
					t.Fatal("root tick replayed provider creation")
				}
			} else if len(calls) != 0 || reads != 1 {
				t.Fatal("expired ticket was renewed or sent")
			}
		})
	}
}
