package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/mediaexecution"
	"github.com/higress-group/wasm-go/pkg/ai/upstreamhealth"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

func TestManagedMediaNoticeHasOneServiceIdentityAndBoundedDuplicateACK(t *testing.T) {
	headers := [][2]string{{"authorization", "Bearer original-service"}}
	if !authorizeManagedMediaNotice(headers, "original-service") || authorizeManagedMediaNotice(append(headers, [2]string{"Authorization", "Bearer original-service"}), "original-service") ||
		authorizeManagedMediaNotice([][2]string{{"authorization", "Bearer customer-key"}, {"x-modelfusion-gateway-instance", "forged"}}, "original-service") {
		t.Fatal("notice trusted a customer credential or duplicate Authorization")
	}
	r := &managedMediaRuntime{settings: managedMediaSettings{QueueCapacity: 1}, jobs: map[string]*managedMediaJob{}}
	create := []byte(`{"schema_version":"media-dispatch-notice.v1","dispatch_id":"original-dispatch"}`)
	first, err := r.enqueue(managedMediaDispatchPath, create)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := r.enqueue(managedMediaDispatchPath, create)
	if err != nil || duplicate != first || len(r.jobs) != 1 || r.sequence != 1 || !first.Accepted || first.SchemaVersion != "media-dispatch-ack.v1" {
		t.Fatal("duplicate notice became another live invocation or a business result")
	}
	for _, body := range [][]byte{
		[]byte(`{"schema_version":"media-dispatch-notice.v1","dispatch_id":"original-dispatch","producer_id":"forged"}`),
		[]byte(`{"schema_version":"media-dispatch-notice.v1","dispatch_id":"original-dispatch","provider_endpoint":"https://foreign.example"}`),
		[]byte(`{"schema_version":"media-dispatch-notice.v1","dispatch_id":"original-dispatch","dispatch_id":"foreign"}`),
	} {
		if _, err := r.enqueue(managedMediaDispatchPath, body); err == nil {
			t.Fatal("notice body supplied execution authority")
		}
	}
	if _, err := r.enqueue(managedMediaQueryPath, []byte(`{"schema_version":"media-query-notice.v1","operation_id":"original-op","request_id":"original-command"}`)); err == nil {
		t.Fatal("bounded queue silently evicted an original job")
	}
	a := managedMediaQueryAttempt("op-a", strings.Repeat("x", 256))
	if len(a) > 256 || a == managedMediaQueryAttempt("op-b", strings.Repeat("x", 256)) || managedMediaQueryAttempt("a/b", "c") == managedMediaQueryAttempt("a", "b/c") {
		t.Fatal("query attempt lost original operation namespace or ID budget")
	}
}

func TestManagedMediaReportRetriesExactWireUntilMatchingCommittedReceipt(t *testing.T) {
	var wires [][]byte
	var callbacks []func(managedMediaPrivateReply)
	r := &managedMediaRuntime{now: time.Now, settings: managedMediaSettings{Console: managedHealthConsole{TimeoutMS: 1000}}}
	r.call = func(method, path string, body []byte, callback func(managedMediaPrivateReply)) error {
		if method != "POST" || !strings.HasSuffix(path, "/observations") {
			t.Fatal("report invoked a business endpoint")
		}
		wires = append(wires, append([]byte(nil), body...))
		callbacks = append(callbacks, callback)
		return nil
	}
	j := &managedMediaJob{runtime: r, reports: []managedMediaReport{{Wire: []byte(`{"original":"first"}`), EventID: "original-event"}, {Wire: []byte(`{"original":"second"}`), EventID: "next-event"}}}
	j.report()
	callbacks[0](managedMediaPrivateReply{Status: 503, HeadersComplete: true, BodyComplete: true, Body: []byte(`{"error":{}}`)})
	j.report()
	if !bytes.Equal(wires[0], wires[1]) || len(j.reports) != 2 {
		t.Fatal("unknown ACK changed or dropped the immutable original")
	}
	callbacks[1](managedMediaPrivateReply{Status: 200, HeadersComplete: true, BodyComplete: true, Body: []byte(`{"data":{"event_id":"foreign-event"}}`)})
	if len(j.reports) != 2 {
		t.Fatal("foreign receipt acknowledged the original event")
	}
	j.report()
	ack := managedMediaPrivateReply{Status: 200, HeadersComplete: true, BodyComplete: true, Body: []byte(`{"data":{"event_id":"original-event","fact":{},"reason_code":null}}`)}
	callbacks[2](ack)
	callbacks[2](ack)
	if len(j.reports) != 1 || j.reports[0].EventID != "next-event" {
		t.Fatal("duplicate ACK consumed another source revision")
	}
}

func managedMediaHostJob(t *testing.T, confirmed bool) (*managedMediaJob, *[]executionfacts.Envelope) {
	t.Helper()
	now := time.Now().UTC()
	fixture := managedConfigFixture()
	d := fixture["_modelfusion_execution"].(managedRelease).Descriptor
	d.UpstreamModel = mediaexecution.Model
	d.AdapterRef, d.AdapterRevision = "original-media-adapter", "1"
	d.Capabilities = map[string]managedCapability{
		"image_generate": {SendMode: "single_send", ResponseClassifierRef: "original-native-classifier", QueryCapabilityRef: "original-task-query", MaxAttempts: 1},
		"task_query":     {SendMode: "single_send", ResponseClassifierRef: "original-query-classifier", QueryCapabilityRef: "original-task-query", MaxAttempts: 1}}
	descriptor, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := managedDescriptorDigest(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	d.DescriptorDigest = digest
	body := []byte(`{"model":"public-model","input":{"prompt":"original prompt"},"parameters":{"seed":9007199254740993}}`)
	basis, err := bodybasis.Capture(body)
	if err != nil {
		t.Fatal(err)
	}
	candidate := map[string]any{"call_id": "original-call", "operation_id": "original-operation", "operation_kind": "image_generate", "release_id": d.ReleaseID,
		"target_id": d.TargetID, "binding_id": d.BindingID, "provider_account_id": d.ProviderAccountID, "upstream_model": d.UpstreamModel, "cluster_ref": d.ClusterRef,
		"mapper_digest": d.MapperDigest, "proxy_digest": d.ProxyDigest, "credential_ref": "original-credential", "credential_revision": "1", "selection_generation": "1",
		"send_transport": "http_call", "recording_owner": "ai_proxy", "mechanism": "router", "original_body_digest": basis.Digest()}
	dispatch := map[string]any{"dispatch_id": "original-dispatch", "attempt_id": "original-create", "operation_id": "original-operation", "revision": "1", "state": "prepared", "deadline": now.Add(time.Minute), "candidate": candidate}
	history := map[string]any{"history_ref": "original-history", "call_id": "original-call", "operation_id": "original-operation", "release_id": d.ReleaseID,
		"route_id": "original-route", "public_model": "public-model", "operation_kind": "image_generate", "business_occurred_at": now.Format(time.RFC3339Nano),
		"caller":          map[string]string{"billing_mode": "customer", "tenant_id": "original-tenant", "principal_id": "original-user", "account_id": "original-wallet", "sales_asset_id": "original-asset"},
		"delivery_writer": map[string]string{"producer_id": "original-producer", "role": "ai_proxy"}, "supporting_writers": []string{}, "attempt": nil}
	archive := map[string]any{"dispatch": dispatch, "descriptor": d, "profile": mediaexecution.Profile, "request_body": string(body), "provider_endpoint": "https://managed-provider.example",
		"native_execution": map[string]any{"producer_id": "original-producer", "authority": "managed-provider.example", "credential_value": "original-provider-secret", "upstream_path": mediaexecution.CreatePath, "timeout_ms": 1000},
		"history":          history, "runtime_config_id": "original-runtime", "config_revision": "1", "output_identity_contract": mediaexecution.NativeImageChoicesContent}
	binding := mediaexecution.ProfileBinding{AdapterRef: d.AdapterRef, AdapterRevision: d.AdapterRevision, CreateClassifierRef: "original-native-classifier", QueryClassifierRef: "original-query-classifier", QueryCapabilityRef: "original-task-query", OutputIdentityContract: mediaexecution.NativeImageChoicesContent}
	executor, err := mediaexecution.FromPrivateArchive(managedFixtureJSON(t, archive), mediaexecution.GatewayIdentity{ProducerID: "original-producer", Actor: "service/gateway/original-producer", BootID: "original-node/original-VM"}, binding)
	if err != nil {
		t.Fatal(err)
	}
	runtime := &managedMediaRuntime{settings: managedMediaSettings{ProducerID: "original-producer", MaxResponseBytes: 1 << 20, Console: managedHealthConsole{TimeoutMS: 1000}}, now: time.Now}
	j := &managedMediaJob{runtime: runtime, executor: executor, notice: managedMediaNotice{DispatchID: "original-dispatch"}, live: mediaexecution.Invocation{ID: "actual-invocation", Generation: "1"}, healthDecision: upstreamhealth.Decision{ScopeEpochs: map[string]upstreamhealth.Revision{}}}
	j.selection, err = executor.Selection(j.live, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	var selection map[string]json.RawMessage
	if err := json.Unmarshal(j.selection, &selection); err != nil {
		t.Fatal(err)
	}
	var selectionDigest string
	_ = json.Unmarshal(selection["selection_digest"], &selectionDigest)
	observed := []executionfacts.Envelope{}
	runtime.call = func(method, path string, requestBody []byte, callback func(managedMediaPrivateReply)) error {
		reply := managedMediaPrivateReply{Status: 200, HeadersComplete: true, BodyComplete: true}
		switch {
		case strings.HasSuffix(path, "/consume"):
			if !confirmed {
				reply.Status = 503
				reply.Body = []byte(`{"error":{}}`)
				callback(reply)
				return nil
			}
			dispatch["state"], dispatch["revision"] = "consumed", "2"
			history["attempt"] = map[string]any{"attempt_id": "original-create", "target_id": d.TargetID, "selection_digest": selectionDigest, "send_transport": "http_call", "recording_owner": "ai_proxy", "mechanism": "router", "credential_ref": "original-credential", "credential_revision": "1", "provider_task_id": nil,
				"writer": map[string]string{"producer_id": "original-producer", "role": "ai_proxy"}, "procurement": map[string]string{"binding_id": d.BindingID, "provider_id": d.ProviderID, "purchase_account_id": d.ProviderAccountID, "region": "original-region", "upstream_model_name": d.UpstreamModel}}
			grant := map[string]any{"schema_version": "dispatch-grant.v1", "grant_id": "first-committed-grant", "dispatch_id": "original-dispatch", "attempt_id": "original-create", "release_id": d.ReleaseID,
				"selection_digest": selectionDigest, "original_body_digest": basis.Digest(), "invocation_id": j.live.ID, "context_generation": j.live.Generation, "gateway_boot_id": "original-node/original-VM", "not_after": now.Add(30 * time.Second)}
			reply.Body = managedFixtureJSON(t, map[string]any{"data": map[string]any{"dispatch": dispatch, "grant": grant, "history": history}})
		case strings.HasSuffix(path, "/observations"):
			var report struct {
				Observation string                 `json:"observation"`
				Projection  managedMediaProjection `json:"projection"`
			}
			if err := json.Unmarshal(requestBody, &report); err != nil {
				t.Fatal(err)
			}
			var fact executionfacts.Envelope
			if err := json.Unmarshal([]byte(report.Observation), &fact); err != nil {
				t.Fatal(err)
			}
			if _, err := executionfacts.Freeze(fact); err != nil {
				t.Fatal(err)
			}
			if fact.Usage.Status != "reported" || fact.Usage.Quantities["image_output_count"] != "1" || len(report.Projection.Outputs) != 1 || len(report.Projection.References) != 1 || report.Projection.OutputManifest == nil {
				t.Fatal("actual output was not projected through the original trusted port")
			}
			observed = append(observed, fact)
			reply.Body = managedFixtureJSON(t, map[string]any{"data": map[string]any{"event_id": fact.EventID, "reason_code": nil, "fact": map[string]string{"decision": "observed"}}})
		default:
			if method != "GET" {
				t.Fatal("unknown consume performed another mutating command")
			}
			reply.Body = []byte(`{"data":{"state":"consumed","grant":null}}`)
		}
		callback(reply)
		return nil
	}
	return j, &observed
}

// Uses actual Wasm SDK provider-call callbacks and the production HTTP evidence
// reader. Console replies are fixtures; this is not live supplier acceptance.
func TestManagedMediaHostNeedsFirstCommittedConsumeAndSendsOriginalAccountOnce(t *testing.T) {
	for _, confirmed := range []bool{false, true} {
		t.Run(map[bool]string{false: "unknown_consume", true: "committed_consume"}[confirmed], func(t *testing.T) {
			job, observed := managedMediaHostJob(t, confirmed)
			job.runtime.provider = job.runtime.providerCall
			type mediaHostConfig struct{}
			vm := wrapper.NewCommonVmCtx("media-original-http", wrapper.ParseConfig(func(gjson.Result, *mediaHostConfig) error { return nil }),
				wrapper.ProcessRequestHeaders(func(wrapper.HttpContext, mediaHostConfig) types.Action {
					job.consume()
					return types.HeaderStopAllIterationAndWatermark
				}))
			host, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption().WithVMContext(vm).WithPluginConfiguration([]byte(`{}`)))
			t.Cleanup(reset)
			host.RegisterForeignFunction("get_log_level", func([]byte) []byte { return []byte{0, 0, 0, 0} })
			if host.StartVM() != types.OnVMStartStatusOK || host.StartPlugin() != types.OnPluginStartStatusOK {
				t.Fatal("media host failed to start")
			}
			id := host.InitializeHttpContext()
			headers := [][2]string{{":method", "POST"}, {":path", "/test-not-public-route"}, {":authority", "gateway.example"}, {"authorization", "Bearer foreign-customer-key"}, {"x-modelfusion-gateway-instance", "forged"}}
			host.CallOnRequestHeaders(id, headers, false)
			calls := host.GetCalloutAttributesFromContext(id)
			if !confirmed {
				if len(calls) != 0 || len(*observed) != 0 {
					t.Fatal("unknown Consume crossed into provider send/FACT")
				}
				return
			}
			if len(calls) != 1 || calls[0].Upstream != "outbound|80||managed-provider.static" {
				t.Fatal("committed media send did not use original cluster exactly once")
			}
			values := map[string]string{}
			for _, header := range calls[0].Headers {
				values[header[0]] = header[1]
			}
			if values[":path"] != mediaexecution.CreatePath || values["Authorization"] != "Bearer original-provider-secret" || values["X-DashScope-Async"] != "enable" || !bytes.Contains(calls[0].Body, []byte("9007199254740993")) || !bytes.Contains(calls[0].Body, []byte(`"messages"`)) {
				t.Fatal("provider body/credential came from a client or wrong protocol")
			}
			body := []byte(`{"request_id":"actual-provider-request","output":{"task_id":"actual-provider-task","task_status":"SUCCEEDED","choices":[{"message":{"content":[{"image":"https://provider.example/output.png?sig=original"}]}}]},"usage":{"image_count":1,"output_width":1024,"output_height":1024}}`)
			host.CallOnHttpCallResponse(calls[0].CalloutID, [][2]string{{":status", "200"}}, nil, body)
			job.runtime.now = func() time.Time { return time.Now().Add(2 * time.Second) }
			if len(job.reports) > 0 {
				job.report()
			}
			if len(*observed) != 2 || (*observed)[0].Kind != "upstream_attempt" || (*observed)[1].Kind != "call_delivery" || (*observed)[0].AttemptID != "original-create" || (*observed)[1].AttemptID != "" || (*observed)[0].CallID != (*observed)[1].CallID {
				t.Fatal("actual media source or sole logical delivery was replaced/duplicated")
			}
			host.CallOnRequestHeaders(id, headers, false)
			if len(host.GetCalloutAttributesFromContext(id)) != 0 {
				t.Fatal("replayed Consume issued another provider creation")
			}
		})
	}
}

func TestManagedMediaPrivateReplyRetainsStrictJSONAndUnknownCompleteness(t *testing.T) {
	for _, reply := range []managedMediaPrivateReply{
		{Status: 200, HeadersComplete: false, BodyComplete: true, Body: []byte(`{"data":{"grant":{}}}`)},
		{Status: 200, HeadersComplete: true, BodyComplete: false, Body: []byte(`{"data":{"grant":{}}}`)},
		{Status: 200, HeadersComplete: true, BodyComplete: true, Body: []byte(`{"data":{},"data":{"grant":{}}}`)},
	} {
		if _, err := managedMediaData(reply); err == nil {
			t.Fatal("incomplete or ambiguous commit became a grant")
		}
	}
}
