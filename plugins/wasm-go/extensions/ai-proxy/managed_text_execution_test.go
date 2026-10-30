package main

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/higress-group/wasm-go/pkg/ai/upstreamhealth"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/resp"
)

func managedTextFixture(t *testing.T, fallback bool) map[string]interface{} {
	t.Helper()
	initial := managedPoolFixture(t)
	makeEntry := func(value map[string]interface{}, second bool) managedTextDirectoryEntry {
		native := value["managed_execution"].(map[string]interface{})
		execution := value["_modelfusion_execution"].(map[string]interface{})
		descriptor := execution["descriptor"].(map[string]interface{})
		model := "upstream-model"
		if second {
			model = "second-upstream-model"
			descriptor["target_id"], descriptor["binding_id"], descriptor["provider_account_id"], descriptor["cluster_ref"], descriptor["upstream_model"] = "target-2", "binding-2", "purchase-2", "outbound|80||second-provider.static", model
			execution["credentials"] = []managedCredentialIdentity{{"second-credential", "4"}}
			native["credentials"] = []managedCredential{{"second-credential", "4", "second-target-key"}}
			native["authority"], native["upstream_path"] = "second-provider.example", "/second/path"
			descriptor["transport_contract"] = managedTransport{Host: "second-provider.example", SNI: "second-provider.example", PathAdapterRef: "path-2"}
			native["upstream_health"].(map[string]interface{})["targets"] = []upstreamhealth.TargetReference{{TargetID: "target-2", BindingID: "binding-2", ProviderAccountID: "purchase-2", RegionEndpointRef: "region-1", UpstreamModel: model, OperationKind: "text_generate", CredentialRef: "second-credential", CredentialGeneration: "4", ScopeIDs: []string{"second-model-scope"}}}
		}
		policy := managedTextSettings{Retry: &managedTextRetry{MaxRetries: 1, RetryTimeoutMS: 5000, RetryOnStatus: []string{"5.*"}}}
		cap := managedCapability{SendMode: "observable_retry", ResponseClassifierRef: "openai-text-v1", MaxAttempts: 2}
		if fallback {
			cap.MaxAttempts, cap.MaxFallbackHops = 3, 1
			if !second {
				policy.Fallback = &managedTextFallback{Strategy: "sequence", Targets: []managedTextTarget{{TargetID: "target-2", Weight: 1}}, ResponseCodes: []string{"5.*"}}
			}
		}
		native["text_execution"] = policy
		descriptor["capabilities"] = map[string]managedCapability{"text_generate": cap}
		tokens := []string{"second-controlled-key", "first-controlled-key"}
		if second {
			tokens = []string{"second-target-key"}
		}
		delete(value, "provider")
		value["activeProviderId"] = "runtime-provider-slug"
		value["retryOnFailure"] = map[string]bool{"enabled": false}
		value["providers"] = []map[string]interface{}{{"id": "runtime-provider-slug", "type": "openai", "apiTokens": tokens}}
		mapper := json.RawMessage(managedFixtureJSON(t, map[string]interface{}{"modelMapping": map[string]string{"public-model": model}, "capture_body_basis": false}))
		mapperDigest, err := mfjson.Digest(mapper)
		if err != nil {
			t.Fatal(err)
		}
		descriptor["mapper_digest"] = mapperDigest
		semantic := map[string]interface{}{}
		for key, item := range value {
			if key != "_modelfusion_execution" {
				semantic[key] = item
			}
		}
		proxy := json.RawMessage(managedFixtureJSON(t, semantic))
		proxyDigest, err := mfjson.Digest(proxy)
		if err != nil {
			t.Fatal(err)
		}
		descriptor["proxy_digest"] = proxyDigest
		digest, err := managedDescriptorDigest(managedFixtureJSON(t, descriptor))
		if err != nil {
			t.Fatal(err)
		}
		descriptor["descriptor_digest"] = digest
		return managedTextDirectoryEntry{Execution: json.RawMessage(managedFixtureJSON(t, execution)), Mapper: mapper, Proxy: proxy}
	}
	var peer map[string]interface{}
	if err := json.Unmarshal(managedFixtureJSON(t, initial), &peer); err != nil {
		t.Fatal(err)
	}
	entries := []managedTextDirectoryEntry{makeEntry(initial, false)}
	if fallback {
		entries = append(entries, makeEntry(peer, true))
	}
	initial[managedTextDirectoryField] = managedTextDirectory{SchemaVersion: "modelfusion-execution-targets.v1", Targets: entries}
	return initial
}

func newManagedTextHost(t *testing.T, fixture map[string]interface{}) *managedHost {
	t.Helper()
	h := &managedHost{}
	vm := wrapper.NewCommonVmCtx("managed-e2-text",
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
	if err := host.SetProperty([]string{"node", "id"}, []byte("e2-original-gateway")); err != nil {
		t.Fatal(err)
	}
	if host.StartVM() != types.OnVMStartStatusOK || host.StartPlugin() != types.OnPluginStartStatusOK {
		t.Fatal("E2 host failed to parse its exact target directory")
	}
	h.id = host.InitializeHttpContext()
	return h
}

func respondManagedTextHealth(t *testing.T, h *managedHost, enabled map[string]bool) {
	t.Helper()
	lookups := h.host.GetRedisCalloutAttributesFromContext(h.id)
	if len(lookups) != 1 {
		t.Fatalf("expected one fresh HEALTH lookup, got %d", len(lookups))
	}
	state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
	source := state.sourceConfig
	revision, _ := h.ctx.GetContext("e2-health-revision").(uint64)
	revision++
	h.ctx.SetContext("e2-health-revision", revision)
	version := strconv.FormatUint(revision+8, 10)
	credentials := []upstreamhealth.CredentialControl{}
	scopes := []upstreamhealth.ScopeState{}
	seen := map[string]bool{}
	for _, target := range source.targets {
		members, err := managedHealthMembers(target)
		if err != nil {
			t.Fatal(err)
		}
		for _, member := range members {
			credentials = append(credentials, upstreamhealth.CredentialControl{Identity: upstreamhealth.CredentialIdentity{CredentialRef: member.target.CredentialRef, ProviderAccountID: member.target.ProviderAccountID, OwnerKind: "platform", OwnerID: "platform-1"}, Generation: member.target.CredentialGeneration, Revision: "2", Enabled: enabled[member.target.CredentialRef]})
			for _, id := range member.target.ScopeIDs {
				if seen[id] {
					continue
				}
				seen[id] = true
				region, model, operation := member.target.RegionEndpointRef, member.target.UpstreamModel, member.target.OperationKind
				scopes = append(scopes, upstreamhealth.ScopeState{Identity: upstreamhealth.ScopeIdentity{ScopeID: id, Kind: "model_operation", ProviderAccountID: member.target.ProviderAccountID, RegionEndpointRef: &region, UpstreamModel: &model, OperationKind: &operation}, Revision: "2", FailureGeneration: "0", RecoveryEpoch: "0"})
			}
		}
	}
	authority := source.health.settings.Snapshot.AuthorityID
	projection, _ := json.Marshal([]interface{}{authority, version, credentials, scopes})
	canonical, err := mfjson.Canonicalize(projection)
	if err != nil {
		t.Fatal(err)
	}
	digest := bodybasis.DigestBytes(canonical)
	now := time.Now().UTC()
	observed, expires := now.Add(-time.Second).Format(time.RFC3339Nano), now.Add(time.Minute).Format(time.RFC3339Nano)
	body, _ := json.Marshal(map[string]interface{}{"projection_schema": upstreamhealth.ProjectionSchema, "authority_id": authority, "revision": version, "state_digest": digest, "credentials": credentials, "scopes": scopes, "observed_at": observed, "valid_until": expires, "complete": true})
	values := []resp.Value{}
	for _, value := range []string{authority, version, digest, string(body), observed, expires, upstreamhealth.ProjectionSchema, "true"} {
		values = append(values, resp.StringValue(value))
	}
	var encoded bytes.Buffer
	if err := resp.NewWriter(&encoded).WriteArray(values); err != nil {
		t.Fatal(err)
	}
	h.host.CallOnRedisCallResponse(lookups[0].CalloutID, 0, encoded.Bytes())
}

func TestManagedTextCredentialRetryKeepsActualAttemptsAndOneDelivery(t *testing.T) {
	h := newManagedTextHost(t, managedTextFixture(t, false))
	h.start(t, []byte(managedOriginalBody), []byte(managedMappedBody))
	respondManagedTextHealth(t, h, map[string]bool{"credential-1": true})
	sends := h.host.GetCalloutAttributesFromContext(h.id)
	if len(sends) != 1 {
		t.Fatalf("first sends %d", len(sends))
	}
	first := sends[0]
	firstBody := []byte(`{"error":{"message":"controlled failure"},"usage":{"prompt_tokens":7,"completion_tokens":2}}`)
	h.host.CallOnHttpCallResponse(first.CalloutID, [][2]string{{":status", "500"}, {"content-type", "application/json"}}, nil, firstBody)
	if h.host.GetSentLocalResponse(h.id) != nil || len(h.host.GetCalloutAttributesFromContext(h.id)) != 0 {
		t.Fatal("retry sent before fresh HEALTH or leaked the first response")
	}
	respondManagedTextHealth(t, h, map[string]bool{"credential-2": true})
	sends = h.host.GetCalloutAttributesFromContext(h.id)
	if len(sends) != 1 {
		t.Fatalf("retry sends %d", len(sends))
	}
	second := sends[0]
	auth, _ := managedSingleHeader(second.Headers, "authorization")
	if auth != "Bearer second-controlled-key" || second.Upstream != first.Upstream || !bytes.Equal(second.Body, first.Body) {
		t.Fatal("retry changed target/account/input or reused the failed credential")
	}
	h.host.CallOnHttpCallResponse(second.CalloutID, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, nil, []byte(managedSuccessfulBody))
	facts := []executionfacts.Envelope{}
	registrations := 0
	for _, record := range h.records(t) {
		if record.messageType == "observation" {
			facts = append(facts, decodeManagedFact(t, record.raw))
		} else {
			registrations++
		}
	}
	if len(facts) != 2 || registrations != 2 {
		t.Fatalf("attempt fact/registration count %d/%d", len(facts), registrations)
	}
	a, b := facts[0], facts[1]
	if a.AttemptID == b.AttemptID || b.ParentAttemptID != a.AttemptID || b.Mechanism != "credential_retry" || a.CredentialRef != "credential-1" || b.CredentialRef != "credential-2" || a.BindingID != b.BindingID || a.CallID != b.CallID || a.OccurredAt != b.OccurredAt || a.Usage.Quantities["input"] != "7" || b.Usage.Quantities["input"] != "9007199254740993" {
		t.Fatal("immutable attempt lineage or independent usage was lost")
	}
	if bridge := h.bridge(t).Envelope(); bridge.Kind != "call_delivery" || bridge.CallID != a.CallID || bridge.Usage.Quantities["input"] != b.Usage.Quantities["input"] {
		t.Fatal("final delivery borrowed failed usage or changed call")
	}
	root := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
	before := len(h.records(t))
	onManagedHTTPResponse(h.ctx, root, 2, len(firstBody))
	if len(h.records(t)) != before || len(h.host.GetCalloutAttributesFromContext(h.id)) != 0 {
		t.Fatal("duplicate callback created a send or fact")
	}
	if root.sourceConfig.release.CredentialRef != "" || root.sourceConfig.settings.CredentialValue != "" {
		t.Fatal("request selection mutated shared pool configuration")
	}
}

func TestManagedTextFallbackUsesOriginalEffectiveInputAndExactPeer(t *testing.T) {
	h := newManagedTextHost(t, managedTextFixture(t, true))
	mapped := []byte(`{"model":"upstream-model","messages":[{"role":"system","content":"actual memory"},{"role":"user","content":[{"type":"text","text":"hello"},{"type":"image_url","image_url":{"url":"https://input.invalid/image"}}]}]}`)
	h.start(t, []byte(managedOriginalBody), mapped)
	respondManagedTextHealth(t, h, map[string]bool{"credential-1": true})
	first := h.host.GetCalloutAttributesFromContext(h.id)[0]
	h.host.CallOnHttpCallResponse(first.CalloutID, [][2]string{{":status", "503"}, {"content-type", "application/json"}}, nil, []byte(`{"error":{"message":"controlled unavailable"}}`))
	// No remaining eligible credential at the first target, then choose the
	// original fallback. Neither lookup alone is a business send.
	respondManagedTextHealth(t, h, map[string]bool{"second-credential": true})
	respondManagedTextHealth(t, h, map[string]bool{"second-credential": true})
	sends := h.host.GetCalloutAttributesFromContext(h.id)
	if len(sends) != 1 {
		t.Fatalf("fallback send count %d", len(sends))
	}
	second := sends[0]
	if second.Upstream != "outbound|80||second-provider.static" || !bytes.Contains(second.Body, []byte(`"model":"second-upstream-model"`)) || !bytes.Contains(second.Body, []byte("actual memory")) || !bytes.Contains(second.Body, []byte("image_url")) {
		t.Fatal("fallback reused the first target/model or dropped effective input attachments")
	}
	auth, _ := managedSingleHeader(second.Headers, "authorization")
	if auth != "Bearer second-target-key" {
		t.Fatal("fallback reused old credentials")
	}
	h.host.CallOnHttpCallResponse(second.CalloutID, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, nil, []byte(strings.ReplaceAll(managedSuccessfulBody, "upstream-model", "second-upstream-model")))
	var attempts []executionfacts.Envelope
	for _, record := range h.records(t) {
		if record.messageType == "observation" {
			attempts = append(attempts, decodeManagedFact(t, record.raw))
		}
	}
	if len(attempts) != 2 || attempts[1].Mechanism != "fallback" || attempts[1].ParentAttemptID != attempts[0].AttemptID || attempts[1].CallID != attempts[0].CallID || attempts[1].BindingID != "binding-2" || attempts[1].ProviderAccountID != "purchase-2" {
		t.Fatal("fallback did not freeze independent purchase lineage")
	}
	if h.bridge(t).Envelope().PublicModel != "public-model" {
		t.Fatal("fallback changed the sale model")
	}
}

func TestManagedTextUnknownIncompleteAcceptedAndClosedDoNotRetry(t *testing.T) {
	for _, mode := range []string{"missing-status", "truncated", "accepted", "large-error", "closed"} {
		t.Run(mode, func(t *testing.T) {
			h := newManagedTextHost(t, managedTextFixture(t, false))
			h.start(t, []byte(managedOriginalBody), []byte(managedMappedBody))
			respondManagedTextHealth(t, h, map[string]bool{"credential-1": true})
			call := h.host.GetCalloutAttributesFromContext(h.id)[0]
			headers := [][2]string{{":status", "500"}, {"content-type", "application/json"}}
			body := []byte(`{"error":{"message":"failure"}}`)
			switch mode {
			case "missing-status":
				headers = headers[1:]
			case "truncated":
				body = []byte(`{"error":`)
			case "accepted":
				headers[0][1] = "202"
				body = []byte(`{"id":"accepted-task","status":"in_progress"}`)
			case "large-error":
				body = []byte(`{"error":{"message":"` + strings.Repeat("x", managedTextFailureBodyLimit) + `"}}`)
			case "closed":
				h.host.CompleteHttpContext(h.id)
			}
			h.host.CallOnHttpCallResponse(call.CalloutID, headers, nil, body)
			if len(h.host.GetCalloutAttributesFromContext(h.id)) != 0 || len(h.host.GetRedisCalloutAttributesFromContext(h.id)) != 0 {
				t.Fatal("unknown/incomplete/accepted/closed response reopened a sender")
			}
		})
	}
}

func TestManagedTextFrozenDirectoryRejectsWrongSemanticsAndIdentity(t *testing.T) {
	fixture := managedTextFixture(t, true)
	h := newManagedTextHost(t, fixture)
	for _, mode := range []string{"missing", "recursive", "duplicate", "wrong-proxy-digest", "wrong-mapper-digest", "wrong-descriptor-digest", "foreign-call", "foreign-ref", "unbound-path", "legacy-sender", "null-policy"} {
		t.Run(mode, func(t *testing.T) {
			var changed map[string]interface{}
			if err := json.Unmarshal(managedFixtureJSON(t, fixture), &changed); err != nil {
				t.Fatal(err)
			}
			directory := changed[managedTextDirectoryField].(map[string]interface{})
			targets := directory["targets"].([]interface{})
			peer := targets[1].(map[string]interface{})
			proxy := peer["proxy_config"].(map[string]interface{})
			execution := peer["execution"].(map[string]interface{})
			switch mode {
			case "missing":
				delete(changed, managedTextDirectoryField)
			case "recursive":
				proxy[managedTextDirectoryField] = map[string]interface{}{}
			case "duplicate":
				directory["targets"] = append(targets, targets[1])
			case "wrong-proxy-digest":
				proxy["managed_execution"].(map[string]interface{})["timeout_ms"] = 1234
			case "wrong-mapper-digest":
				peer["mapper_config"].(map[string]interface{})["capture_body_basis"] = true
			case "wrong-descriptor-digest":
				execution["descriptor"].(map[string]interface{})["cluster_ref"] = "foreign-cluster"
			case "foreign-call":
				execution["sales_asset_id"] = "foreign-sales-asset"
			case "foreign-ref":
				execution["credentials"].([]interface{})[0].(map[string]interface{})["credential_ref"] = "foreign-ref"
			case "unbound-path":
				proxy["path_scope"] = "legacy_suffixes"
			case "legacy-sender":
				proxy["retryOnFailure"] = map[string]bool{"enabled": true}
			case "null-policy":
				changed["managed_execution"].(map[string]interface{})["text_execution"] = nil
			}
			copy := h.cfg
			if err := configureManagedExecution(gjson.ParseBytes(managedFixtureJSON(t, changed)), &copy); err == nil {
				t.Fatal("changed/partial/recursive native target directory was accepted")
			}
		})
	}
}

func TestManagedTextRevocationExhaustionAndBudgetKeepLastOriginalFailure(t *testing.T) {
	for _, mode := range []string{"revoked-next-key", "budget-exhausted", "stream-closed-before-qualification"} {
		t.Run(mode, func(t *testing.T) {
			h := newManagedTextHost(t, managedTextFixture(t, false))
			h.start(t, []byte(managedOriginalBody), []byte(managedMappedBody))
			respondManagedTextHealth(t, h, map[string]bool{"credential-1": true})
			call := h.host.GetCalloutAttributesFromContext(h.id)[0]
			failure := []byte(`{"error":{"message":"original failure"},"usage":{"prompt_tokens":3,"completion_tokens":1}}`)
			h.host.CallOnHttpCallResponse(call.CalloutID, [][2]string{{":status", "500"}, {"content-type", "application/json"}}, nil, failure)
			if mode == "stream-closed-before-qualification" {
				h.host.CompleteHttpContext(h.id)
				respondManagedTextHealth(t, h, map[string]bool{"credential-2": true})
				if h.host.GetSentLocalResponse(h.id) != nil {
					t.Fatal("closed stream fabricated a final client delivery")
				}
			} else if mode == "revoked-next-key" {
				respondManagedTextHealth(t, h, map[string]bool{})
				if h.bridge(t).Envelope().Usage.Quantities["input"] != "3" {
					t.Fatal("revoked pool replaced the original failure usage")
				}
			} else {
				respondManagedTextHealth(t, h, map[string]bool{"credential-2": true})
				second := h.host.GetCalloutAttributesFromContext(h.id)[0]
				h.host.CallOnHttpCallResponse(second.CalloutID, [][2]string{{":status", "500"}, {"content-type", "application/json"}}, nil, failure)
				if h.bridge(t).Envelope().DeliveryState != "failed" {
					t.Fatal("exhausted budget did not keep final failure")
				}
			}
			if len(h.host.GetCalloutAttributesFromContext(h.id)) != 0 || len(h.host.GetRedisCalloutAttributesFromContext(h.id)) != 0 {
				t.Fatal("revoked/closed/exhausted chain scheduled another sender")
			}
		})
	}
}

func TestManagedTextClosedRetryKeepsLateEvidenceOnItsOwnAttempt(t *testing.T) {
	h := newManagedTextHost(t, managedTextFixture(t, false))
	h.start(t, []byte(managedOriginalBody), []byte(managedMappedBody))
	respondManagedTextHealth(t, h, map[string]bool{"credential-1": true})
	first := h.host.GetCalloutAttributesFromContext(h.id)[0]
	h.host.CallOnHttpCallResponse(first.CalloutID, [][2]string{{":status", "500"}, {"content-type", "application/json"}}, nil, []byte(`{"error":{"message":"first failure"},"usage":{"prompt_tokens":7,"completion_tokens":2}}`))
	respondManagedTextHealth(t, h, map[string]bool{"credential-2": true})
	second := h.host.GetCalloutAttributesFromContext(h.id)[0]
	// Close the business hook while retaining the emulator context so the
	// explicitly late callback remains callable. Real host deletion need not
	// deliver it; the separate closed-context cases assert no new send.
	onManagedStreamDone(h.ctx, h.cfg)
	h.host.CallOnHttpCallResponse(second.CalloutID, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, nil, []byte(managedSuccessfulBody))
	var facts []executionfacts.Envelope
	for _, record := range h.records(t) {
		if record.messageType == "observation" {
			facts = append(facts, decodeManagedFact(t, record.raw))
		}
	}
	if len(facts) != 3 || facts[0].CredentialRef != "credential-1" || facts[1].CredentialRef != "credential-2" || facts[1].ExecutionResult != "unknown" || facts[2].AttemptID != facts[1].AttemptID || facts[2].SourceRevision != "2" || facts[2].SupersedesEventID != facts[1].EventID || facts[2].ParentAttemptID != facts[0].AttemptID || facts[0].Usage.Quantities["input"] != "7" {
		for _, fact := range facts {
			t.Logf("attempt=%s credential=%s revision=%s result=%s supersedes=%s parent=%s", fact.AttemptID, fact.CredentialRef, fact.SourceRevision, fact.ExecutionResult, fact.SupersedesEventID, fact.ParentAttemptID)
		}
		t.Fatalf("late response changed another attempt or erased the original failed usage: %d facts", len(facts))
	}
	if h.host.GetSentLocalResponse(h.id) != nil || len(h.host.GetCalloutAttributesFromContext(h.id)) != 0 {
		t.Fatal("late retry response fabricated delivery or dispatched again")
	}
}

func TestManagedTextKeepsExistingLargeEffectiveInputBudget(t *testing.T) {
	h := newManagedTextHost(t, managedTextFixture(t, false))
	mapped := []byte(`{"model":"upstream-model","messages":[{"role":"system","content":"` + strings.Repeat("x", (1<<20)+1) + `"},{"role":"user","content":"hello"}]}`)
	h.start(t, []byte(managedOriginalBody), mapped)
	respondManagedTextHealth(t, h, map[string]bool{"credential-1": true})
	calls := h.host.GetCalloutAttributesFromContext(h.id)
	if len(calls) != 1 || !bytes.Equal(calls[0].Body, mapped) {
		t.Fatal("effective runtime input inherited immutable configuration limits")
	}
	h.host.CallOnHttpCallResponse(calls[0].CalloutID, [][2]string{{":status", "500"}, {"content-type", "application/json"}}, nil, []byte(`{"error":{"message":"retry this complete failure"}}`))
	respondManagedTextHealth(t, h, map[string]bool{"credential-2": true})
	retry := h.host.GetCalloutAttributesFromContext(h.id)
	if len(retry) != 1 || !bytes.Equal(retry[0].Body, mapped) {
		t.Fatal("Key retry changed the original effective input bytes")
	}
	h.host.CompleteHttpContext(h.id)
}
