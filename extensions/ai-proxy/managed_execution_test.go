package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/resp"
)

const managedOriginalBody = `{"model":"public-model","messages":[{"role":"user","content":"hello"}]}`
const managedCanonicalOriginalBody = `{"messages":[{"content":"hello","role":"user"}],"model":"public-model"}`
const managedMappedBody = `{"model":"upstream-model","messages":[{"role":"user","content":"hello"}]}`
const managedSuccessfulBody = `{"model":"upstream-model","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":9007199254740993,"completion_tokens":2,"prompt_tokens_details":{"cached_tokens":1}}}`

func managedConfigFixture() map[string]interface{} {
	digest := func(char string) string { return "sha256:" + strings.Repeat(char, 64) }
	descriptor := managedDescriptor{
		ReleaseID: "release-1", TargetID: "target-1", BindingID: "binding-1", ProviderID: "provider-1", ProviderAccountID: "purchase-1",
		RegionEndpointRef: "region-1", ServiceRef: "service-1", ClusterRef: "outbound|80||managed-provider.static", CredentialPoolRef: "pool-1", AdapterRef: "adapter-1",
		CredentialPoolRevision: "1", AdapterRevision: "1", MapperDigest: digest("a"), ProxyDigest: digest("b"), DescriptorDigest: "sha256:c8d0b8b684bd9e5e2e10f8526393826ffb7b1fffe6dbcda0644d65d3522363cc",
		UpstreamModel: "upstream-model", ConfigCanonicalization: "mf-json.v1",
		Transport:    managedTransport{Host: "managed-provider.example", SNI: "managed-provider.example", PathAdapterRef: "path-1"},
		Capabilities: map[string]managedCapability{"text_generate": {SendMode: "single_send", ResponseClassifierRef: "openai-text-v1", MaxAttempts: 1, MaxFallbackHops: 0}},
	}
	return map[string]interface{}{
		"provider":      map[string]interface{}{"type": "openai", "apiTokens": []string{"controlled-upstream-key"}},
		"protocol_mode": "auto", "path_scope": "bound_target",
		"managed_execution":      managedSettings{ProducerID: "managed-e1", CallIdentitySource: "admission", PrivateStream: "private-execution-stream", Redis: managedRedisSettings{ServiceName: "redis.static"}, UpstreamPath: "/custom/provider/answer", Authority: "managed-provider.example", CredentialValue: "controlled-upstream-key"},
		"_modelfusion_execution": managedRelease{SchemaVersion: "modelfusion-execution.v1", Descriptor: descriptor, RouteID: "route-1", SalesAssetID: "asset-1", PublicModel: "public-model", CredentialRef: "credential-1", CredentialRevision: "1"},
	}
}

func managedFixtureJSON(t *testing.T, object map[string]interface{}) []byte {
	t.Helper()
	raw, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

type managedHost struct {
	host proxytest.HostEmulator
	id   uint32
	ctx  wrapper.HttpContext
	cfg  config.PluginConfig
}

// These are real Wasm SDK host-call callbacks. They prove transport and send
// count behavior only; they are not evidence of an external upstream execution.
func newManagedHost(t *testing.T, fixtures ...map[string]interface{}) *managedHost {
	t.Helper()
	fixture := managedConfigFixture()
	if len(fixtures) != 0 {
		fixture = fixtures[0]
	}
	h := &managedHost{}
	vm := wrapper.NewCommonVmCtx("ai-proxy",
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
			action, handled := onManagedRequestHeaders(ctx, cfg)
			if !handled {
				t.Error("fixture was not managed")
			}
			return action
		}),
		wrapper.ProcessRequestBody(func(ctx wrapper.HttpContext, cfg config.PluginConfig, body []byte) types.Action {
			action, handled := onManagedRequestBody(ctx, cfg, body)
			if !handled {
				t.Error("fixture body was not managed")
			}
			return action
		}),
		wrapper.ProcessStreamDone(onManagedStreamDone),
	)
	host, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption().WithVMContext(vm).WithPluginConfiguration(managedFixtureJSON(t, fixture)))
	t.Cleanup(reset)
	h.host = host
	host.RegisterForeignFunction("get_log_level", func([]byte) []byte { return []byte{0, 0, 0, 0} })
	if status := host.StartVM(); status != types.OnVMStartStatusOK {
		t.Fatalf("VM start: %v", status)
	}
	if status := host.StartPlugin(); status != types.OnPluginStartStatusOK {
		t.Fatalf("plugin start: %v", status)
	}
	h.id = host.InitializeHttpContext()
	return h
}

func managedRequestHeaders(t *testing.T, original []byte) [][2]string {
	t.Helper()
	basis, err := bodybasis.Capture(original)
	if err != nil {
		t.Fatal(err)
	}
	headers := [][2]string{{":method", "POST"}, {":path", "/custom/customer/text-alias"}, {":authority", "gateway.example"}, {"content-type", "application/json"}}
	headers = append(headers,
		[2]string{managedCallHeader, "50000000-0000-0000-0000-000000000001"},
		[2]string{managedAdmissionReleaseHeader, "release-1"},
		[2]string{managedAdmissionDeadlineHeader, time.Now().UTC().Add(5 * time.Second).Format(time.RFC3339Nano)},
		[2]string{managedAdmissionOccurredHeader, "2026-10-27T10:00:00Z"})
	headers = append(headers, balanceidentity.Snapshot{
		AccountID: "10000000-0000-0000-0000-000000000001", APIKeyID: "20000000-0000-0000-0000-000000000001", CredentialGeneration: "9007199254740993",
		TenantID: "30000000-0000-0000-0000-000000000001", PrincipalID: "40000000-0000-0000-0000-000000000001",
	}.Headers()...)
	return append(headers, basis.Headers()...)
}

func (h *managedHost) start(t *testing.T, original, mapped []byte) {
	t.Helper()
	headers := managedRequestHeaders(t, original)
	headers = append(headers, [2]string{managedAttemptHeader, "client-attempt"}, [2]string{managedAttemptHeader, "client-attempt-duplicate"})
	if action := h.host.CallOnRequestHeaders(h.id, headers, false); action != types.HeaderStopIteration {
		t.Fatalf("headers action: %v", action)
	}
	if action := h.host.CallOnRequestBody(h.id, mapped, true); action != types.ActionPause {
		t.Fatalf("body action: %v", action)
	}
}

type managedStreamRecord struct {
	messageType string
	raw         []byte
	calloutID   uint32
}

func (h *managedHost) records(t *testing.T) []managedStreamRecord {
	t.Helper()
	var result []managedStreamRecord
	for _, callout := range h.host.GetRedisCalloutAttributesFromContext(h.id) {
		value, _, err := resp.NewReader(bytes.NewReader(callout.Query)).ReadValue()
		if err != nil {
			t.Fatal(err)
		}
		parts := value.Array()
		if len(parts) != 7 || parts[0].String() != "XADD" || parts[1].String() != "private-execution-stream" || parts[3].String() != "message_type" || parts[5].String() != "event" {
			t.Fatalf("unexpected stream command: %#v", parts)
		}
		result = append(result, managedStreamRecord{messageType: parts[4].String(), raw: []byte(parts[6].String()), calloutID: callout.CalloutID})
	}
	return result
}

func (h *managedHost) response(t *testing.T, headers [][2]string, body []byte) {
	t.Helper()
	callouts := h.host.GetCalloutAttributesFromContext(h.id)
	if len(callouts) != 1 {
		t.Fatalf("business send count before callback: %d", len(callouts))
	}
	h.host.CallOnHttpCallResponse(callouts[0].CalloutID, headers, nil, body)
	if len(h.host.GetCalloutAttributesFromContext(h.id)) != 0 {
		t.Fatal("callback retried or resumed a second business send")
	}
}

func decodeManagedFact(t *testing.T, raw []byte) executionfacts.Envelope {
	t.Helper()
	var envelope executionfacts.Envelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope
}

func (h *managedHost) bridge(t *testing.T) executionfacts.Snapshot {
	t.Helper()
	response := h.host.GetSentLocalResponse(h.id)
	if response == nil {
		t.Fatal("client response missing")
	}
	encoded, err := managedSingleHeader(response.Headers, executionfacts.DeliveryBridgeHeader)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := executionfacts.DecodeDeliveryBridge(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestManagedRouteBoundConfigurationDoesNotInheritTarget(t *testing.T) {
	h := newManagedHost(t)
	inherited := h.cfg
	if err := configureManagedExecution(gjson.Parse(`{"path_scope":"bound_target"}`), &inherited); err != nil || inherited.ManagedExecution != nil {
		t.Fatal("rule inherited another target")
	}
	cases := []func(map[string]interface{}){
		func(o map[string]interface{}) { delete(o, "_modelfusion_execution") },
		func(o map[string]interface{}) { delete(o, "managed_execution") },
		func(o map[string]interface{}) {
			s := o["managed_execution"].(managedSettings)
			s.CallIdentitySource = ""
			o["managed_execution"] = s
		},
		func(o map[string]interface{}) {
			s := o["managed_execution"].(managedSettings)
			s.CallIdentitySource = "proxy"
			o["managed_execution"] = s
		},
		func(o map[string]interface{}) { o["path_scope"] = "legacy_suffixes" },
		func(o map[string]interface{}) { o["enable_methods"] = []string{"POST"} },
		func(o map[string]interface{}) { o["enable_path_suffixes"] = []string{"/v1/chat/completions"} },
		func(o map[string]interface{}) {
			s := o["managed_execution"].(managedSettings)
			s.Authority = "different.example"
			o["managed_execution"] = s
		},
		func(o map[string]interface{}) {
			s := o["managed_execution"].(managedSettings)
			s.UpstreamPath = "https://other.example/path"
			o["managed_execution"] = s
		},
		func(o map[string]interface{}) {
			s := o["managed_execution"].(managedSettings)
			s.CredentialValue = "key\r\ninjected:value"
			o["managed_execution"] = s
		},
		func(o map[string]interface{}) {
			r := o["_modelfusion_execution"].(managedRelease)
			r.Descriptor.ConfigCanonicalization = "target-config-json.v1"
			o["_modelfusion_execution"] = r
		},
	}
	for i, change := range cases {
		object := managedConfigFixture()
		change(object)
		cfg := h.cfg
		if err := configureManagedExecution(gjson.ParseBytes(managedFixtureJSON(t, object)), &cfg); err == nil || cfg.ManagedExecution != nil {
			t.Errorf("case %d accepted an incomplete or unsafe release", i)
		}
	}
}

func TestManagedDescriptorRejectsStaleOriginalFieldDigests(t *testing.T) {
	h := newManagedHost(t)
	cases := []struct {
		name   string
		change func(*managedDescriptor)
	}{
		{"cluster", func(d *managedDescriptor) { d.ClusterRef = "outbound|80||another-target.dns" }},
		{"transport", func(d *managedDescriptor) { d.Transport.PathAdapterRef = "another-path-adapter" }},
		{"capabilities", func(d *managedDescriptor) {
			capability := d.Capabilities["text_generate"]
			capability.ResponseClassifierRef = "another-response-classifier"
			d.Capabilities["text_generate"] = capability
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			object := managedConfigFixture()
			release := object["_modelfusion_execution"].(managedRelease)
			test.change(&release.Descriptor)
			object["_modelfusion_execution"] = release
			cfg := h.cfg
			err := configureManagedExecution(gjson.ParseBytes(managedFixtureJSON(t, object)), &cfg)
			if err == nil || err.Error() != "managed execution target descriptor digest differs" || cfg.ManagedExecution != nil {
				t.Fatal("changed original descriptor retained stale release authority")
			}
		})
	}
}

func TestManagedDescriptorAcceptsCompleteSDKShape(t *testing.T) {
	h := newManagedHost(t)
	object := managedConfigFixture()
	release := object["_modelfusion_execution"].(managedRelease)
	release.Descriptor.Transport.SNI = ""
	text := release.Descriptor.Capabilities["text_generate"]
	text.QueryCapabilityRef, text.ProviderIdempotencyRef = "query-text-v1", "text-idempotency-v1"
	release.Descriptor.Capabilities["text_generate"] = text
	release.Descriptor.Capabilities["image_generate"] = managedCapability{
		SendMode: "observable_retry", ResponseClassifierRef: "image-status-v1",
		QueryCapabilityRef: "query-v1", ProviderIdempotencyRef: "provider-idempotency-v1",
		MaxAttempts: 4294967295, MaxFallbackHops: 0,
	}
	// Independent mf-json.v1 golden: all original fields, excluding only self.
	release.Descriptor.DescriptorDigest = "sha256:cfdac010d8406da62720b588a5a5463312c3be8a3c35a5b8812aa91a8a8ab009"
	object["_modelfusion_execution"] = release
	cfg := h.cfg
	if err := configureManagedExecution(gjson.ParseBytes(managedFixtureJSON(t, object)), &cfg); err != nil {
		t.Fatal(err)
	}
	configured, ok := cfg.ManagedExecution.(*managedExecutionConfig)
	if !ok || configured.release.Descriptor.DescriptorDigest != release.Descriptor.DescriptorDigest || configured.release.Descriptor.Capabilities["image_generate"].MaxAttempts != 4294967295 {
		t.Fatal("complete released SDK descriptor lost fields or exact integer limits")
	}
}

func TestManagedDescriptorDigestPreservesRawFieldsAndRejectsAmbiguousInput(t *testing.T) {
	raw := []byte(`{"descriptor_digest":"ignored","capabilities":{"text_generate":{"query_capability_ref":""}},"transport_contract":{"sni":""}}`)
	canonical := []byte(`{"capabilities":{"text_generate":{"query_capability_ref":""}},"transport_contract":{"sni":""}}`)
	got, err := managedDescriptorDigest(raw)
	if err != nil || got != managedBodyDigest(canonical) {
		t.Fatal("descriptor digest dropped original explicitly empty fields")
	}
	for _, raw := range []string{
		`{"descriptor_digest":"one","descriptor_digest":"two","a":1}`,
		`{"descriptor_digest":"one","nested":{"a":1,"a":2}}`,
		`{"descriptor_digest":"one","a":"\uD800"}`,
		`[]`,
	} {
		if _, err := managedDescriptorDigest([]byte(raw)); err == nil {
			t.Fatal("ambiguous original descriptor was accepted before removing self")
		}
	}
	h := newManagedHost(t)
	raw = managedFixtureJSON(t, managedConfigFixture())
	raw = bytes.Replace(raw, []byte(`"config_canonicalization":"mf-json.v1"`), []byte(`"config_canonicalization":"mf-json.v1","unknown_descriptor_field":true`), 1)
	cfg := h.cfg
	if err := configureManagedExecution(gjson.ParseBytes(raw), &cfg); err == nil || err.Error() != "invalid managed execution release descriptor" || cfg.ManagedExecution != nil {
		t.Fatal("strict descriptor decoding no longer rejects unknown fields")
	}
}

func TestManagedIdentityEncodingUsesRealWrapperBodyCallback(t *testing.T) {
	h := newManagedHost(t)
	headers := append(managedRequestHeaders(t, []byte(managedOriginalBody)), [2]string{"content-encoding", "identity"})
	if action := h.host.CallOnRequestHeaders(h.id, headers, false); action != types.HeaderStopIteration {
		t.Fatalf("headers action: %v", action)
	}
	if action := h.host.CallOnRequestBody(h.id, []byte(managedMappedBody), true); action != types.ActionPause {
		t.Fatalf("body action: %v", action)
	}
	callouts := h.host.GetCalloutAttributesFromContext(h.id)
	if len(callouts) != 1 || !bytes.Equal(callouts[0].Body, []byte(managedMappedBody)) || h.host.GetSentLocalResponse(h.id) != nil {
		t.Fatal("identity encoding bypassed the controlled request-body callback")
	}
	records := h.records(t)
	if len(records) != 1 || records[0].messageType != "execution_registration" {
		t.Fatal("real wrapper body callback did not register the original execution")
	}
	var registration executionfacts.Registration
	if json.Unmarshal(records[0].raw, &registration) != nil || !bytes.Contains(registration.Selection, []byte(managedBodyDigest([]byte(managedCanonicalOriginalBody)))) {
		t.Fatal("registration did not retain the independently canonical original body")
	}
	h.response(t, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, []byte(managedSuccessfulBody))
	if len(h.records(t)) != 2 {
		t.Fatal("the one controlled send did not produce its attempt observation")
	}
}

func TestManagedSingleSendFreezesOriginalBodyAndOverridesClientIDs(t *testing.T) {
	h := newManagedHost(t)
	h.start(t, []byte(managedOriginalBody), []byte(managedMappedBody))
	callouts := h.host.GetCalloutAttributesFromContext(h.id)
	if len(callouts) != 1 {
		t.Fatalf("actual HTTP callout count: %d", len(callouts))
	}
	call := callouts[0]
	if call.Upstream != "outbound|80||managed-provider.static" || !bytes.Equal(call.Body, []byte(managedMappedBody)) {
		t.Fatal("send did not use the released target and exact mapped bytes")
	}
	for name, want := range map[string]string{":method": "POST", ":path": "/custom/provider/answer", ":authority": "managed-provider.example", "authorization": "Bearer controlled-upstream-key"} {
		value, err := managedSingleHeader(call.Headers, name)
		if err != nil || value != want {
			t.Fatalf("outbound %s differs", name)
		}
	}
	for _, header := range []string{managedCallHeader, managedAttemptHeader} {
		outgoing, err := managedSingleHeader(call.Headers, header)
		if err != nil || !balanceidentity.ValidID(outgoing) {
			t.Fatal("business send has no native execution ID")
		}
		installed, err := managedSingleHeader(h.host.GetCurrentRequestHeaders(h.id), header)
		if err != nil || installed != outgoing {
			t.Fatal("client IDs were retained or execution IDs drifted")
		}
	}
	records := h.records(t)
	if len(records) != 1 || records[0].messageType != "execution_registration" {
		t.Fatal("independent registration was not dispatched before the business response")
	}
	var registration executionfacts.Registration
	if err := json.Unmarshal(records[0].raw, &registration); err != nil {
		t.Fatal(err)
	}
	if registration.BusinessOccurredAt != "2026-10-27T10:00:00Z" {
		t.Fatal("proxy replaced the original committed call time")
	}
	var selection map[string]string
	if err := json.Unmarshal(registration.Selection, &selection); err != nil {
		t.Fatal(err)
	}
	if selection["original_body_digest"] != managedBodyDigest([]byte(managedCanonicalOriginalBody)) || selection["original_body_digest"] == managedBodyDigest([]byte(managedOriginalBody)) || selection["original_body_digest"] == managedBodyDigest([]byte(managedMappedBody)) {
		t.Fatal("selection did not use the canonical original body")
	}
	if selection["send_transport"] != "http_call" || selection["recording_owner"] != "ai_proxy" || selection["attempt_id"] != registration.AttemptID {
		t.Fatal("original send ownership is incomplete")
	}
	if _, err := executionfacts.CanonicalSelection(registration.Selection); err != nil {
		t.Fatal(err)
	}
	if _, handled := onManagedRequestBody(h.ctx, h.cfg, []byte(managedMappedBody)); !handled {
		t.Fatal("duplicate body stopped being managed")
	}
	if len(h.host.GetCalloutAttributesFromContext(h.id)) != 1 || len(h.records(t)) != 1 {
		t.Fatal("duplicate body allocated another send or registration")
	}
}

func TestManagedCallbackPublishesAttemptAndBridgesOneDelivery(t *testing.T) {
	h := newManagedHost(t)
	h.start(t, []byte(managedOriginalBody), []byte(managedMappedBody))
	h.response(t, [][2]string{{":status", "200"}, {"content-type", "application/json"}, {executionfacts.DeliveryBridgeHeader, "upstream-forgery"}, {executionfacts.DeliveryBridgeHeader, "another-forgery"}}, []byte(managedSuccessfulBody))
	response := h.host.GetSentLocalResponse(h.id)
	if response == nil || response.StatusCode != 200 || !bytes.Equal(response.Data, []byte(managedSuccessfulBody)) {
		t.Fatal("billing changed the client response")
	}
	bridge := h.bridge(t)
	delivery := bridge.Envelope()
	if delivery.Kind != "call_delivery" || delivery.DeliveryState != "succeeded" || delivery.AttemptID != "" || delivery.PublicModel != "public-model" {
		t.Fatal("bridge is not the one original call delivery")
	}
	if delivery.Usage.Status != "reported" || delivery.Usage.Quantities["input"] != "9007199254740993" || delivery.Usage.Quantities["cache_hit"] != "1" || delivery.Usage.Quantities["cache_miss"] != "9007199254740992" {
		t.Fatal("provider usage lost exactness or cache split")
	}
	records := h.records(t)
	if len(records) != 2 {
		t.Fatalf("proxy should emit registration plus attempt, got %d", len(records))
	}
	var registration executionfacts.Registration
	var attempt executionfacts.Envelope
	for _, record := range records {
		if bytes.Contains(record.raw, []byte("controlled-upstream-key")) {
			t.Fatal("stream evidence leaked an upstream credential")
		}
		if record.messageType == "execution_registration" {
			if json.Unmarshal(record.raw, &registration) != nil {
				t.Fatal("registration invalid")
			}
		} else {
			attempt = decodeManagedFact(t, record.raw)
			if attempt.Kind != "upstream_attempt" {
				t.Fatal("proxy emitted a second delivery writer")
			}
		}
	}
	if attempt.HTTPStatus == nil || *attempt.HTTPStatus != 200 || attempt.SendState != "observed_sent" || attempt.Acceptance != "accepted" || attempt.OccurredAt != registration.BusinessOccurredAt || delivery.OccurredAt != registration.BusinessOccurredAt || attempt.CallID != delivery.CallID {
		t.Fatal("callback borrowed another execution or fabricated evidence")
	}
	if attempt.OutboundBodyDigest != managedBodyDigest([]byte(managedMappedBody)) {
		t.Fatal("actual outbound bytes were not frozen")
	}
}

func TestManagedMissingUpstreamStatusRemainsMissing(t *testing.T) {
	h := newManagedHost(t)
	h.start(t, []byte(managedOriginalBody), []byte(managedMappedBody))
	h.response(t, [][2]string{{"content-type", "application/json"}}, []byte(managedSuccessfulBody))
	response := h.host.GetSentLocalResponse(h.id)
	if response == nil || response.StatusCode != 502 {
		t.Fatal("missing upstream status did not produce the local response policy")
	}
	for _, record := range h.records(t) {
		if record.messageType != "observation" {
			continue
		}
		fact := decodeManagedFact(t, record.raw)
		if fact.HTTPStatus != nil || fact.HeadersComplete == nil || *fact.HeadersComplete || fact.Usage.Status != "missing" || bytes.Contains(record.raw, []byte(`"http_status":502`)) {
			t.Fatal("local 502 became an upstream fact")
		}
	}
	if delivery := h.bridge(t).Envelope(); delivery.DeliveryState != "failed" || delivery.Usage.Status != "missing" {
		t.Fatal("delivery borrowed unverified provider usage")
	}
}

func TestManagedRedisFailureFailsOpenAndNeverResendsBusiness(t *testing.T) {
	h := newManagedHost(t)
	h.start(t, []byte(managedOriginalBody), []byte(managedMappedBody))
	registration := h.records(t)[0]
	h.host.CallOnRedisCallResponse(registration.calloutID, 0, []byte("-ERR unavailable\r\n"))
	if h.ctx.GetUserAttribute("execution_fact_gap") != true || len(h.host.GetCalloutAttributesFromContext(h.id)) != 1 {
		t.Fatal("registration failure blocked or retried the business send")
	}
	h.response(t, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, []byte(managedSuccessfulBody))
	if h.host.GetSentLocalResponse(h.id).StatusCode != 200 {
		t.Fatal("fact append failure replaced the successful business response")
	}
	for _, record := range h.records(t) {
		h.host.CallOnRedisCallResponse(record.calloutID, 0, []byte("$5\r\n123-0\r\n"))
	}
	if h.ctx.GetUserAttribute("execution_fact_gap") != true {
		t.Fatal("a later append ACK erased an earlier gap")
	}
	if len(h.host.GetCalloutAttributesFromContext(h.id)) != 0 {
		t.Fatal("a Redis callback resent the business request")
	}
}

func TestManagedClosedStreamRecordsUnknownAttemptWithoutInventingDelivery(t *testing.T) {
	h := newManagedHost(t)
	h.start(t, []byte(managedOriginalBody), []byte(managedMappedBody))
	onManagedStreamDone(h.ctx, h.cfg)
	var first executionfacts.Envelope
	for _, record := range h.records(t) {
		if record.messageType == "observation" {
			first = decodeManagedFact(t, record.raw)
		}
	}
	if first.Kind != "upstream_attempt" || first.ExecutionResult != "unknown" || first.Usage.Status != "missing" || first.HTTPStatus != nil {
		t.Fatal("closed stream invented an upstream result")
	}
	if h.host.GetSentLocalResponse(h.id) != nil {
		t.Fatal("stream close fabricated a final delivery")
	}
	h.response(t, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, []byte(managedSuccessfulBody))
	var corrected executionfacts.Envelope
	for _, record := range h.records(t) {
		if record.messageType == "observation" {
			fact := decodeManagedFact(t, record.raw)
			if fact.SourceRevision == "2" {
				corrected = fact
			}
		}
	}
	if corrected.SourceFactID != first.SourceFactID || corrected.SupersedesEventID != first.EventID || corrected.OccurredAt != first.OccurredAt || corrected.ExecutionResult != "succeeded" {
		t.Fatal("late evidence was not a correction of the original attempt")
	}
	if h.host.GetSentLocalResponse(h.id) != nil {
		t.Fatal("late callback fabricated delivery after stream close")
	}
}

func TestManagedInvalidBoundRequestNeverFallsThroughToRouter(t *testing.T) {
	cases := []struct {
		name       string
		change     func([][2]string) [][2]string
		body       []byte
		headerOnly bool
	}{
		{"method", func(h [][2]string) [][2]string { h[0][1] = "GET"; return h }, []byte(managedMappedBody), true},
		{"content_type", func(h [][2]string) [][2]string { h[3][1] = "text/plain"; return h }, []byte(managedMappedBody), true},
		{"missing_auth", func(h [][2]string) [][2]string { return append(h[:4], h[5:]...) }, []byte(managedMappedBody), true},
		{"wrong_mapped_model", nil, []byte(managedOriginalBody), false},
		{"duplicate_model", nil, []byte(`{"model":"upstream-model","model":"other","messages":[]}`), false},
		{"missing_original_basis", func(h [][2]string) [][2]string {
			var r [][2]string
			for _, p := range h {
				if p[0] != bodybasis.DigestHeader && p[0] != bodybasis.PublicModelHeader {
					r = append(r, p)
				}
			}
			return r
		}, []byte(managedMappedBody), false},
		{"invalid_stream_flag", nil, []byte(`{"model":"upstream-model","messages":[],"stream":"true"}`), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newManagedHost(t)
			headers := managedRequestHeaders(t, []byte(managedOriginalBody))
			if tc.change != nil {
				headers = tc.change(headers)
			}
			action := h.host.CallOnRequestHeaders(h.id, headers, false)
			if !tc.headerOnly {
				if action != types.HeaderStopIteration {
					t.Fatalf("header action: %v", action)
				}
				action = h.host.CallOnRequestBody(h.id, tc.body, true)
			}
			if action != types.ActionPause || h.host.GetSentLocalResponse(h.id) == nil || len(h.host.GetCalloutAttributesFromContext(h.id)) != 0 || len(h.records(t)) != 0 {
				t.Fatal("invalid managed request fell through or allocated an upstream purchase")
			}
		})
	}
}

func TestManagedUsageRequiresExactCompleteProviderCounts(t *testing.T) {
	cases := []struct {
		usage  string
		status string
	}{
		{`null`, "missing"}, {`{}`, "missing"},
		{`{"prompt_tokens":0,"completion_tokens":0}`, "reported"},
		{`{"prompt_tokens":9007199254740993,"completion_tokens":1}`, "reported"},
		{`{"prompt_tokens":340282366920938463463374607431768211455,"completion_tokens":0}`, "reported"},
		{`{"prompt_tokens":340282366920938463463374607431768211456,"completion_tokens":0}`, "missing"},
		{`{"prompt_tokens":-1,"completion_tokens":0}`, "missing"},
		{`{"prompt_tokens":1.0,"completion_tokens":0}`, "missing"},
		{`{"prompt_tokens":1e1,"completion_tokens":0}`, "missing"},
		{`{"prompt_tokens":"1","completion_tokens":0}`, "missing"},
		{`{"prompt_tokens":1,"completion_tokens":0,"prompt_tokens_details":{"cached_tokens":2}}`, "missing"},
		{`{"prompt_tokens":1,"completion_tokens":0,"prompt_cache_hit_tokens":1}`, "missing"},
		{`{"prompt_tokens":1,"completion_tokens":2,"total_tokens":4}`, "missing"},
	}
	for i, tc := range cases {
		var object map[string]json.RawMessage
		if err := json.Unmarshal([]byte(`{"usage":`+tc.usage+`}`), &object); err != nil {
			t.Fatal(err)
		}
		usage := managedOpenAIUsage(object)
		if usage.Status != tc.status {
			t.Errorf("case %d: got %s", i, usage.Status)
		}
		if tc.status == "missing" && usage.Quantities != nil {
			t.Fatal("missing usage fabricated quantities")
		}
	}
}

func managedSSEFixture(ending string) []byte {
	return []byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"},\"finish_reason\":null}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"" + ending + "\"}]}\n\n" +
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":3,\"prompt_tokens_details\":{\"cached_tokens\":2}}}\n\n" +
		"data: [DONE]\n\n")
}

func TestManagedAggregatedSSERequiresTerminalAndFinalUsage(t *testing.T) {
	complete := managedSSEFixture("stop")
	cases := []struct {
		name           string
		body           []byte
		status, result string
	}{
		{"complete", complete, "reported", "succeeded"},
		{"partial", managedSSEFixture("length"), "reported", "partial"},
		{"crlf", bytes.ReplaceAll(complete, []byte("\n"), []byte("\r\n")), "reported", "succeeded"},
		{"cut_event", complete[:len(complete)-1], "missing", "unknown"},
		{"missing_done", bytes.ReplaceAll(complete, []byte("data: [DONE]\n\n"), nil), "missing", "unknown"},
		{"missing_finish", bytes.ReplaceAll(complete, []byte(`"finish_reason":"stop"`), []byte(`"finish_reason":null`)), "missing", "unknown"},
		{"duplicate_frame_key", bytes.ReplaceAll(complete, []byte(`"prompt_tokens":7`), []byte(`"prompt_tokens":7,"prompt_tokens":8`)), "missing", "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			usage, result, _ := managedSSEObservation(tc.body)
			if usage.Status != tc.status || result != tc.result {
				t.Fatalf("got usage=%s result=%s", usage.Status, result)
			}
		})
	}
}

func TestManagedStreamResponsePreservesAggregatedWireBytes(t *testing.T) {
	for _, complete := range []bool{true, false} {
		t.Run(fmt.Sprint(complete), func(t *testing.T) {
			h := newManagedHost(t)
			original := []byte(strings.TrimSuffix(managedOriginalBody, "}") + `,"stream":true}`)
			mapped := []byte(strings.TrimSuffix(managedMappedBody, "}") + `,"stream":true}`)
			h.start(t, original, mapped)
			body := managedSSEFixture("stop")
			if !complete {
				body = body[:len(body)-1]
			}
			h.response(t, [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}, body)
			if response := h.host.GetSentLocalResponse(h.id); response.StatusCode != 200 || !bytes.Equal(response.Data, body) {
				t.Fatal("aggregate SSE response was changed")
			}
			delivery := h.bridge(t).Envelope()
			if complete && delivery.Usage.Status != "reported" || !complete && (delivery.Usage.Status != "missing" || delivery.DeliveryState != "unknown") {
				t.Fatal("truncated stream claimed final usage or completion")
			}
			if h.ctx.GetUserAttribute("managed_execution_response_mode") != "buffered_http_call" {
				t.Fatal("aggregate callout was mislabeled as live frame streaming")
			}
		})
	}
}

func TestManagedJSONRejectsAmbiguousInput(t *testing.T) {
	for _, raw := range []string{`{"model":"a","model":"b"}`, `{"nested":{"x":1,"x":2}}`, `{"model":"\ud800"}`, `{"model":"ok"} {}`, `{"value":NaN}`} {
		if validateManagedJSON([]byte(raw), 1024) == nil {
			t.Fatalf("ambiguous JSON accepted: %s", raw)
		}
	}
	if validateManagedJSON([]byte(`{"model":"\ud83d\ude00"}`), 1024) != nil {
		t.Fatal("valid surrogate pair rejected")
	}
}

func TestManagedHTTPErrorDoesNotInventPreacceptanceRejection(t *testing.T) {
	profile, err := newManagedProtocolProfile("", "")
	if err != nil {
		t.Fatal(err)
	}
	s := &managedExecutionState{attemptID: "original-attempt", config: &managedExecutionConfig{profile: profile}}
	failed := managedResponseOutcome(s, [][2]string{{":status", "503"}, {"content-type", "application/json"}}, nil, []byte(managedSuccessfulBody), nil)
	if failed.HTTPStatus == nil || *failed.HTTPStatus != 503 || failed.Acceptance != "unknown" || failed.ExecutionResult != "failed" || failed.Usage.Status != "reported" {
		t.Fatal("HTTP failure fabricated rejection or erased actual provider usage")
	}
	body := []byte(`{"model":"upstream-model","choices":[{"index":0,"finish_reason":null}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`)
	incomplete := managedResponseOutcome(s, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, nil, body, nil)
	if incomplete.Acceptance != "unknown" || incomplete.ExecutionResult != "unknown" {
		t.Fatal("HTTP 200 alone invented a successful completed text execution")
	}
}
