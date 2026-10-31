package mediaexecution_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/mediaexecution"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
)

type fixture struct {
	archive, descriptor, candidate, history map[string]any
	body                                    []byte
	now                                     time.Time
	gateway                                 mediaexecution.GatewayIdentity
	binding                                 mediaexecution.ProfileBinding
	live                                    mediaexecution.Invocation
}

func raw(t *testing.T, value any) []byte {
	t.Helper()
	result, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func sign(t *testing.T, value map[string]any, key string) {
	t.Helper()
	delete(value, key)
	digest, err := mfjson.Digest(raw(t, value))
	if err != nil {
		t.Fatal(err)
	}
	value[key] = digest
}

// These are private Console-wire fixtures, never proof of actual supplier
// availability or a substitute for the parent's HTTP/PG/K3S acceptance.
func original(t *testing.T) fixture {
	t.Helper()
	f := fixture{now: time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC),
		gateway: mediaexecution.GatewayIdentity{ProducerID: "proxy-original", Actor: "service/gateway", BootID: "gateway-boot"},
		binding: mediaexecution.ProfileBinding{AdapterRef: "qwen-original-adapter", AdapterRevision: "original-revision", CreateClassifierRef: "qwen-native-async-v1", QueryClassifierRef: "qwen-native-async-v1", QueryCapabilityRef: "qwen-original-task-v1"},
		live:    mediaexecution.Invocation{ID: "invocation-create", Generation: "1"},
		body:    []byte(" {\n\"model\":\"public-image\",\"input\":{\"prompt\":\"a tree\"},\"parameters\":{\"n\":4,\"seed\":9007199254740993}\n}")}
	caps := map[string]any{"send_mode": "single_send", "max_attempts": 1, "max_fallback_hops": 0,
		"response_classifier_ref": "qwen-native-async-v1", "query_capability_ref": "qwen-original-task-v1"}
	f.descriptor = map[string]any{"release_id": "release-original", "target_id": "target-original", "binding_id": "binding-original",
		"adapter_ref": f.binding.AdapterRef, "adapter_revision": f.binding.AdapterRevision,
		"provider_id": "provider-original", "provider_account_id": "purchase-original", "upstream_model": mediaexecution.Model,
		"region_endpoint_ref": "region-original", "cluster_ref": "cluster-original", "mapper_digest": "sha256:" + strings.Repeat("a", 64), "proxy_digest": "sha256:" + strings.Repeat("b", 64),
		"config_canonicalization": "mf-json.v1", "capabilities": map[string]any{"image_generate": caps, "task_query": caps},
		"transport_contract": map[string]any{"host": "vendor.example", "sni": "vendor.example", "path_adapter_ref": "qwen-native-async-v1"}}
	sign(t, f.descriptor, "descriptor_digest")
	basis, err := bodybasis.Capture(f.body)
	if err != nil {
		t.Fatal(err)
	}
	f.candidate = map[string]any{"call_id": "call-original", "operation_id": "operation-original", "operation_kind": "image_generate",
		"credential_ref": "credential-original", "credential_revision": "3", "selection_generation": "7",
		"original_body_digest": basis.Digest(), "mechanism": "router", "send_transport": "http_call", "recording_owner": "ai_proxy"}
	for _, key := range []string{"release_id", "target_id", "binding_id", "provider_account_id", "upstream_model", "cluster_ref", "mapper_digest", "proxy_digest"} {
		f.candidate[key] = f.descriptor[key]
	}
	f.history = map[string]any{"history_ref": "history-original", "call_id": "call-original", "operation_id": "operation-original",
		"release_id": "release-original", "route_id": "route-original", "operation_kind": "image_generate", "public_model": "public-image",
		"business_occurred_at": f.now.Format(time.RFC3339), "caller": map[string]any{"billing_mode": "customer", "tenant_id": "tenant-original",
			"principal_id": "user-original", "account_id": "wallet-original", "sales_asset_id": "asset-original"},
		"delivery_writer": map[string]any{"producer_id": "media-original", "role": "media_worker"}, "supporting_writers": []any{}, "attempt": nil}
	f.archive = map[string]any{"dispatch": f.dispatch("prepared"), "descriptor": f.descriptor, "profile": mediaexecution.Profile,
		"request_body": string(f.body), "provider_endpoint": "https://vendor.example", "history": f.history,
		"native_execution": map[string]any{"producer_id": "proxy-original", "authority": "vendor.example", "credential_value": "original-secret",
			"upstream_path": mediaexecution.CreatePath, "timeout_ms": 3000}, "runtime_config_id": "runtime-original", "config_revision": "7"}
	f.archive["procurement"] = struct {
		Identity       map[string]string `json:"identity"`
		Writer         map[string]string `json:"writer"`
		PricingAssetID string            `json:"pricing_asset_id"`
		CostOwner      map[string]string `json:"cost_owner"`
	}{map[string]string{"binding_id": "binding-original", "provider_id": "provider-original", "purchase_account_id": "purchase-original", "region": "beijing-original", "upstream_model_name": mediaexecution.Model},
		map[string]string{"producer_id": "proxy-original", "role": "ai_proxy"}, "original-cost-asset", map[string]string{"kind": "platform", "owner_id": "original-cost-owner"}}
	return f
}

func (f fixture) dispatch(state string) map[string]any {
	return map[string]any{"dispatch_id": "dispatch-original", "attempt_id": "attempt-create", "operation_id": "operation-original",
		"state": state, "revision": "2", "deadline": f.now.Add(time.Minute).Format(time.RFC3339), "candidate": f.candidate}
}
func (f fixture) selection(t *testing.T, query bool) map[string]any {
	s := make(map[string]any)
	for key, value := range f.candidate {
		s[key] = value
	}
	s["attempt_id"], s["dispatch_id"], s["descriptor_digest"] = "attempt-create", "dispatch-original", f.descriptor["descriptor_digest"]
	s["selection_canonicalization"], s["invocation_id"], s["gateway_boot_id"], s["context_generation"] = "selection-json.v1", f.live.ID, f.gateway.BootID, f.live.Generation
	if query {
		delete(s, "dispatch_id")
		s["attempt_id"], s["operation_kind"], s["mechanism"], s["original_body_digest"] = "attempt-query", "task_query", "task_query", bodybasis.DigestBytes(nil)
	}
	sign(t, s, "selection_digest")
	return s
}
func (f fixture) sealed(selection map[string]any, task string) map[string]any {
	h := make(map[string]any)
	for key, value := range f.history {
		h[key] = value
	}
	h["attempt"] = map[string]any{"attempt_id": "attempt-create", "target_id": "target-original", "selection_digest": selection["selection_digest"],
		"send_transport": "http_call", "recording_owner": "ai_proxy", "mechanism": "router", "credential_ref": "credential-original", "credential_revision": "3",
		"provider_task_id": task, "pricing_asset_id": "original-cost-asset", "cost_owner": map[string]any{"kind": "platform", "owner_id": "original-cost-owner"},
		"writer": map[string]any{"producer_id": "proxy-original", "role": "ai_proxy"}, "procurement": map[string]any{"binding_id": "binding-original",
			"provider_id": "provider-original", "purchase_account_id": "purchase-original", "region": "beijing-original", "upstream_model_name": mediaexecution.Model}}
	return h
}
func (f fixture) commit(t *testing.T, selection map[string]any) map[string]any {
	return map[string]any{"dispatch": f.dispatch("consumed"), "history": f.sealed(selection, ""), "grant": map[string]any{
		"schema_version": "dispatch-grant.v1", "grant_id": "grant-first", "dispatch_id": "dispatch-original", "attempt_id": "attempt-create",
		"invocation_id": f.live.ID, "gateway_boot_id": f.gateway.BootID, "context_generation": f.live.Generation,
		"release_id": "release-original", "selection_digest": selection["selection_digest"], "original_body_digest": f.candidate["original_body_digest"],
		"not_after": f.now.Add(time.Minute).Format(time.RFC3339)}}
}
func (f fixture) query(t *testing.T, create map[string]any) map[string]any {
	parameters := map[string]json.RawMessage{}
	if err := json.Unmarshal(f.body, &parameters); err != nil {
		t.Fatal(err)
	}
	delete(parameters, "model")
	expires := f.now.Add(30 * time.Minute).Format(time.RFC3339)
	selection := map[string]any{"query_attempt_id": "attempt-query", "operation_id": "operation-original", "provider_task_id": "task-original", "method": "GET",
		"path": "/api/v1/tasks/task-original", "descriptor": f.descriptor, "profile": mediaexecution.Profile, "effective_parameters": parameters,
		"execution_selection": f.selection(t, true), "credential_ref": "credential-original", "credential_revision": "3", "capability_ref": "qwen-original-task-v1",
		"owner_actor": f.gateway.Actor, "owner_instance": f.gateway.BootID, "expires_at": expires}
	return map[string]any{"history": f.sealed(create, "task-original"), "receipt": map[string]any{"query_attempt_id": "attempt-query", "operation_id": "operation-original", "expires_at": expires, "selection": selection}}
}
func createRequest(t *testing.T) (mediaexecution.Request, fixture, *mediaexecution.Executor) {
	t.Helper()
	f := original(t)
	e, err := mediaexecution.FromPrivateArchive(raw(t, f.archive), f.gateway, f.binding)
	if err != nil {
		t.Fatal(err)
	}
	s := f.selection(t, false)
	r, err := e.PrepareCreate(raw(t, f.commit(t, s)), raw(t, s), f.live, f.now)
	if err != nil {
		t.Fatal(err)
	}
	return r, f, e
}

func TestCreateUsesOnlyFirstCommittedGrantAndOriginalTransport(t *testing.T) {
	r, f, executor := createRequest(t)
	if r.Method() != "POST" || r.Endpoint() != "https://vendor.example" || r.Path() != mediaexecution.CreatePath ||
		r.CreateAttemptID() != "attempt-create" || r.AttemptID() != "attempt-create" || r.TimeoutMilliseconds() != 3000 || len(r.Descriptor()) == 0 {
		t.Fatal("changed original send target")
	}
	expected := bytes.Replace(f.body, []byte(`"public-image"`), []byte(`"qwen-image-3.0"`), 1)
	expected = bytes.Replace(expected, []byte(`{"prompt":"a tree"}`), []byte(`{"messages":[{"role":"user","content":[{"text":"a tree"}]}]}`), 1)
	if !bytes.Equal(r.Body(), expected) {
		t.Fatalf("rewrote native parameters/precision: %s", r.Body())
	}
	copyBody := r.Body()
	copyBody[0] = '!'
	if !bytes.Equal(r.Body(), expected) || !strings.Contains(string(r.Body()), "9007199254740993") {
		t.Fatal("request aliases caller bytes")
	}
	if strings.Contains(fmt.Sprintf("%+v", r), "original-secret") || strings.Contains(fmt.Sprintf("%+v", executor), "original-secret") {
		t.Fatal("secret in diagnostic format")
	}
	s := f.selection(t, false)
	if _, err := executor.PrepareCreate(raw(t, f.commit(t, s)), raw(t, s), f.live, f.now); !errors.Is(err, mediaexecution.ErrReplay) {
		t.Fatal("replayed generation grant")
	}
	var secret, async bool
	for _, h := range r.Headers() {
		secret = secret || h[0] == "Authorization" && h[1] == "Bearer original-secret"
		async = async || h[0] == "X-DashScope-Async" && h[1] == "enable"
	}
	if !secret || !async {
		t.Fatal("missing original credential or native async header")
	}
}

func TestUnknownConsumeAndForeignContextNeverPrepareCreate(t *testing.T) {
	for _, mode := range []string{"unknown", "replay reply", "wrong boot", "wrong account", "wrong writer", "expired"} {
		t.Run(mode, func(t *testing.T) {
			f := original(t)
			e, err := mediaexecution.FromPrivateArchive(raw(t, f.archive), f.gateway, f.binding)
			if err != nil {
				t.Fatal(err)
			}
			s := f.selection(t, false)
			reply := f.commit(t, s)
			now := f.now
			switch mode {
			case "unknown":
				e.ConsumeUnknown()
			case "replay reply":
				reply["grant"] = nil
			case "wrong boot":
				reply["grant"].(map[string]any)["gateway_boot_id"] = "new-boot"
			case "wrong account":
				reply["history"].(map[string]any)["caller"].(map[string]any)["account_id"] = "other-wallet"
			case "wrong writer":
				reply["history"].(map[string]any)["attempt"].(map[string]any)["writer"] = map[string]any{"producer_id": "proxy-original", "role": "media_worker"}
			case "expired":
				now = now.Add(time.Minute)
			}
			if _, err := e.PrepareCreate(raw(t, reply), raw(t, s), f.live, now); err == nil {
				t.Fatal("prepared unauthorized or unknown create")
			}
		})
	}
}

func TestOriginalQueryUsesIndependentAttemptWithoutNewGenerationAdmission(t *testing.T) {
	_, f, executor := createRequest(t)
	s := f.selection(t, false)
	q := f.query(t, s)
	r, err := executor.PrepareQuery(raw(t, q), f.now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if !r.IsQuery() || r.Method() != "GET" || len(r.Body()) != 0 || r.AttemptID() != "attempt-query" || r.CreateAttemptID() != "attempt-create" ||
		r.Path() != "/api/v1/tasks/task-original" || !bytes.Contains(r.OriginalHistory(), []byte("wallet-original")) || !bytes.Contains(r.OriginalHistory(), []byte("purchase-original")) {
		t.Fatal("query changed original ownership or became a create")
	}
	if _, err := executor.PrepareQuery(raw(t, q), f.now); !errors.Is(err, mediaexecution.ErrReplay) {
		t.Fatal("reused first query selection")
	}
	for _, mode := range []string{"replay", "foreign task", "foreign credential", "foreign owner", "expired"} {
		t.Run(mode, func(t *testing.T) {
			e, err := mediaexecution.FromPrivateArchive(raw(t, f.archive), f.gateway, f.binding)
			if err != nil {
				t.Fatal(err)
			}
			q := f.query(t, s)
			receipt := q["receipt"].(map[string]any)
			selection := receipt["selection"].(map[string]any)
			now := f.now
			switch mode {
			case "replay":
				receipt["selection"] = nil
			case "foreign task":
				selection["provider_task_id"] = "task-other"
			case "foreign credential":
				selection["credential_ref"] = "current-rotated-key"
			case "foreign owner":
				selection["owner_actor"] = "service/scheduler"
			case "expired":
				now = now.Add(time.Hour)
			}
			if _, err := e.PrepareQuery(raw(t, q), now); err == nil {
				t.Fatal("query escaped original committed read permission")
			}
		})
	}
}

func observed(body string, status uint16) mediaexecution.Response {
	return mediaexecution.Response{HTTPStatus: &status, HeadersComplete: true, BodyComplete: true, BodyObserved: true, Body: []byte(body), SendState: "observed_sent", ObservedAt: "2026-10-28T01:00:00Z"}
}

func TestCreateAcceptanceAndQueryCompletionAreSeparateFacts(t *testing.T) {
	r, f, e := createRequest(t)
	o, err := r.Observe(observed(`{"request_id":"provider-create","output":{"task_id":"task-original","task_status":"PENDING"}}`, 202))
	if err != nil || o.GenerationAcceptance != "accepted" || o.GenerationResult != "pending" || o.Terminal || o.GenerationUsage.Status != "missing" {
		t.Fatal("202 invented completion or free usage")
	}
	q, err := e.PrepareQuery(raw(t, f.query(t, f.selection(t, false))), f.now)
	if err != nil {
		t.Fatal(err)
	}
	conflict, err := q.Observe(observed(`{"output":{"task_id":"task-other","task_status":"SUCCEEDED"},"usage":{"image_count":9}}`, 200))
	if err != nil || conflict.Terminal || conflict.ReasonCode != "media_provider_task_conflict" || conflict.GenerationUsage.Status != "missing" {
		t.Fatal("foreign task became original generation fact")
	}
	notSent, err := q.Observe(mediaexecution.Response{SendState: "not_sent", ObservedAt: "2026-10-28T01:00:00Z"})
	if err != nil || notSent.GenerationAcceptance == "not_sent" || notSent.Terminal || notSent.ActualAttempt.Acceptance != "not_sent" {
		t.Fatal("unsent query erased original generation acceptance")
	}
	running, err := q.Observe(observed(`{"output":{"task_id":"task-original","task_status":"RUNNING","results":[{"url":"https://results.example/a?sig=first"}]}}`, 200))
	if err != nil || running.Terminal || running.GenerationResult != "pending" || running.ActualAttempt.ExecutionResult != "succeeded" || running.ActualAttempt.Usage.Status != "not_applicable" {
		t.Fatal("HTTP GET success became terminal image generation")
	}
	complete, err := q.Observe(observed(`{"request_id":"provider-query","output":{"task_id":"task-original","task_status":"SUCCEEDED","results":[{"url":"https://results.example/a?sig=second"},{"code":"FailedSubtask"}]},"usage":{"image_count":1,"width":1024,"height":768}}`, 200))
	if err != nil || !complete.Terminal || complete.GenerationResult != "partial" || complete.DeliveryState != "partial" || complete.GenerationUsage.Quantities["image_count"] != "1" || !complete.MeasurementsPending || complete.CreateAttemptID != "attempt-create" || complete.QueryAttemptID != "attempt-query" {
		t.Fatal("lost partial output, reliable usage, or original generation ownership")
	}
	if complete.AggregateSpecification["width"] != "1024" || len(complete.Outputs) != 2 || !complete.Outputs[0].Successful {
		t.Fatal("did not retain actual result evidence")
	}
}

func TestUnknownAndLateResponsesDoNotResendOrLoseProvenance(t *testing.T) {
	r, f, e := createRequest(t)
	for _, response := range []mediaexecution.Response{
		{SendState: "handed_off", ObservedAt: "2026-10-28T02:00:00Z"},
		{HTTPStatus: ptr(202), HeadersComplete: true, BodyObserved: true, BodyComplete: false, Body: []byte(`{"output":{"task_id":"task-original"`), SendState: "observed_sent"},
		observed(`{"output":{"task_id":"first","task_id":"second","task_status":"PENDING"}}`, 202),
	} {
		o, err := r.Observe(response)
		if err != nil || o.GenerationAcceptance != "unknown" || o.Terminal || o.ActualAttempt.Usage.Quantities != nil {
			t.Fatal("unknown response invented acceptance/zero")
		}
	}
	late, err := r.Observe(observed(`{"output":{"task_id":"task-original","task_status":"SUCCEEDED","results":[{"url":"https://results.example/a"}]},"usage":{"image_count":1}}`, 200))
	if err != nil || !late.Terminal || late.CallID != "call-original" || !bytes.Contains(late.OriginalHistory, []byte("purchase-original")) || !late.MeasurementsPending {
		t.Fatal("late response lost original ownership or invented per-output identity")
	}
	if _, err := e.PrepareCreate(raw(t, f.commit(t, f.selection(t, false))), raw(t, f.selection(t, false)), f.live, f.now); err == nil {
		t.Fatal("late/unknown observation renewed create grant")
	}
}
func ptr(value uint16) *uint16 { return &value }

func TestNativeShapeAndCanonicalOriginalBodyCannotSilentlyDowngrade(t *testing.T) {
	for _, mode := range []string{"raw digest", "compatible shape", "other model", "other producer", "unsupported revision"} {
		t.Run(mode, func(t *testing.T) {
			f := original(t)
			switch mode {
			case "raw digest":
				f.candidate["original_body_digest"] = bodybasis.DigestBytes(f.body)
			case "compatible shape":
				f.archive["request_body"] = `{"model":"public-image","prompt":"a tree"}`
			case "other model":
				f.descriptor["upstream_model"] = "qwen-image-plus"
				sign(t, f.descriptor, "descriptor_digest")
			case "other producer":
				f.gateway.ProducerID = "scheduler"
			case "unsupported revision":
				f.descriptor["adapter_revision"] = "new-unregistered-revision"
				sign(t, f.descriptor, "descriptor_digest")
			}
			if _, err := mediaexecution.FromPrivateArchive(raw(t, f.archive), f.gateway, f.binding); err == nil {
				t.Fatal("unsupported original wire was silently remapped")
			}
		})
	}
}

func explicitPool(t *testing.T) fixture {
	t.Helper()
	f := original(t)
	native := f.archive["native_execution"].(map[string]any)
	delete(native, "credential_value")
	native["credentials"] = []any{
		map[string]any{"credential_ref": "another-credential", "credential_revision": "9", "credential_value": "another-secret"},
		map[string]any{"credential_ref": "credential-original", "credential_revision": "3", "credential_value": "original-secret"},
	}
	return f
}

func TestExplicitCredentialPoolUsesOnlyPreparedMemberAndRetainsItForQuery(t *testing.T) {
	f := explicitPool(t)
	before := raw(t, f.archive)
	e, err := mediaexecution.FromPrivateArchive(before, f.gateway, f.binding)
	if err != nil {
		t.Fatal(err)
	}
	s := f.selection(t, false)
	r, err := e.PrepareCreate(raw(t, f.commit(t, s)), raw(t, s), f.live, f.now)
	if err != nil {
		t.Fatal(err)
	}
	authorization := func(r mediaexecution.Request) string {
		for _, header := range r.Headers() {
			if header[0] == "Authorization" {
				return header[1]
			}
		}
		return ""
	}
	if authorization(r) != "Bearer original-secret" || !bytes.Equal(before, raw(t, f.archive)) {
		t.Fatal("prepared member was replaced by pool position or config rewriting")
	}
	// Mutating a caller's current input cannot rotate an already retained task.
	members := f.archive["native_execution"].(map[string]any)["credentials"].([]any)
	members[0], members[1] = members[1], members[0]
	members[0].(map[string]any)["credential_value"] = "rotated-current-secret"
	query, err := e.PrepareQuery(raw(t, f.query(t, s)), f.now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if authorization(query) != "Bearer original-secret" {
		t.Fatal("query selected a current pool member")
	}
	if _, err = e.PrepareCreate(raw(t, f.commit(t, s)), raw(t, s), f.live, f.now); !errors.Is(err, mediaexecution.ErrReplay) {
		t.Fatal("consumed multi-member operation authorized another POST")
	}
}

func TestExplicitPoolMissingDuplicateChangedOrForeignMemberFailsClosed(t *testing.T) {
	for _, mode := range []string{"missing", "duplicate", "wrong-revision", "numeric-revision", "extra-member-field", "empty-secret", "ambiguous-scalar", "null-pool"} {
		t.Run(mode, func(t *testing.T) {
			f := explicitPool(t)
			native := f.archive["native_execution"].(map[string]any)
			members := native["credentials"].([]any)
			selected := members[1].(map[string]any)
			switch mode {
			case "missing":
				native["credentials"] = members[:1]
			case "duplicate":
				members[0] = members[1]
			case "wrong-revision":
				selected["credential_revision"] = "4"
			case "numeric-revision":
				selected["credential_revision"] = 3
			case "extra-member-field":
				selected["provider_account_id"] = "foreign-account"
			case "empty-secret":
				selected["credential_value"] = ""
			case "ambiguous-scalar":
				native["credential_value"] = "first-member-seed"
			case "null-pool":
				native["credentials"] = nil
			}
			if _, err := mediaexecution.FromPrivateArchive(raw(t, f.archive), f.gateway, f.binding); err == nil {
				t.Fatal("invalid pool substituted another member")
			}
		})
	}
}

func TestCredentialPoolProviderBindingAndUnknownConsumeDoNotAuthorizeReselection(t *testing.T) {
	f := explicitPool(t)
	native := raw(t, f.archive["native_execution"])
	if err := mediaexecution.ValidateCredentialConfiguration(native, []string{"original-secret", "another-secret"}); err != nil {
		t.Fatal(err)
	}
	for _, values := range [][]string{{"original-secret"}, {"original-secret", "foreign-secret"}, {"original-secret", "original-secret"}} {
		if mediaexecution.ValidateCredentialConfiguration(native, values) == nil {
			t.Fatal("partial or foreign provider pool accepted")
		}
	}
	e, err := mediaexecution.FromPrivateArchive(raw(t, f.archive), f.gateway, f.binding)
	if err != nil {
		t.Fatal(err)
	}
	e.ConsumeUnknown()
	s := f.selection(t, false)
	if _, err = e.PrepareCreate(raw(t, f.commit(t, s)), raw(t, s), f.live, f.now); !errors.Is(err, mediaexecution.ErrReplay) {
		t.Fatal("unknown consume retried with another credential")
	}
}
