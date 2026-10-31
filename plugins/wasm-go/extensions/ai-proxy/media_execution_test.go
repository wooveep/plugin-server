package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/mediaexecution"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/higress-group/wasm-go/pkg/ai/upstreamhealth"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/resp"
)

type managedMediaTestRedis struct {
	upstreamhealth.RuntimeRedis
	command func([]interface{}, wrapper.RedisResponseCallback) error
	eval    func(string, int, []interface{}, []interface{}, wrapper.RedisResponseCallback) error
}

func (r managedMediaTestRedis) Command(args []interface{}, callback wrapper.RedisResponseCallback) error {
	return r.command(args, callback)
}
func (r managedMediaTestRedis) Ready() bool { return true }
func (r managedMediaTestRedis) Eval(script string, count int, keys, args []interface{}, callback wrapper.RedisResponseCallback) error {
	if r.eval == nil {
		return errors.New("unexpected media test Redis read")
	}
	return r.eval(script, count, keys, args, callback)
}

func mediaNoticeWire(t *testing.T, notice managedMediaNotice) []byte {
	t.Helper()
	var fields map[string]json.RawMessage
	raw, err := json.Marshal(notice)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	delete(fields, "ticket_digest")
	original, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := mfjson.Digest(original)
	if err != nil {
		t.Fatal(err)
	}
	notice.TicketDigest = digest
	raw, err = json.Marshal(notice)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func TestManagedMediaNoticeHasOneServiceIdentityAndBoundedDuplicateACK(t *testing.T) {
	headers := [][2]string{{"authorization", "Bearer original-service"}}
	if !authorizeManagedMediaNotice(headers, "original-service") || authorizeManagedMediaNotice(append(headers, [2]string{"Authorization", "Bearer original-service"}), "original-service") || authorizeManagedMediaNotice([][2]string{{"authorization", "Bearer customer-key"}}, "original-service") {
		t.Fatal("notice trusted another service identity")
	}
	r := &managedMediaRuntime{settings: managedMediaSettings{QueueCapacity: 1}, jobs: map[string]*managedMediaJob{}}
	notice := managedMediaNotice{SchemaVersion: "media-dispatch-notice.v2", DispatchID: "original-dispatch", OperationID: "original-op", RequestID: "original-dispatch", TicketKey: "mf:media:ticket:original", ExpiresAt: time.Now().Add(time.Minute), Admission: managedMediaAdmission{ScopeID: "original-scope", AccountID: "original-wallet"}, Archive: json.RawMessage(`{}`)}
	create := mediaNoticeWire(t, notice)
	first, err := r.enqueue(managedMediaDispatchPath, create)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := r.enqueue(managedMediaDispatchPath, create)
	if err != nil || duplicate != first || len(r.jobs) != 1 || r.sequence != 1 || !first.Accepted || first.SchemaVersion != "media-dispatch-ack.v2" {
		t.Fatal("duplicate notice allocated another invocation")
	}
	notice.Archive = json.RawMessage(`{"provider_endpoint":"https://changed.example"}`)
	if _, err := r.enqueue(managedMediaDispatchPath, mediaNoticeWire(t, notice)); err == nil {
		t.Fatal("same ticket replaced original archive")
	}
	notice.TicketKey = "mf:media:ticket:other"
	if _, err := r.enqueue(managedMediaDispatchPath, mediaNoticeWire(t, notice)); err == nil {
		t.Fatal("full queue evicted original work")
	}
	for _, wire := range [][]byte{bytes.Replace(create, []byte(`"ticket_digest":`), []byte(`"ticket_digest":"forged","unknown":`), 1), append(create, []byte(`{}`)...)} {
		if _, err := r.enqueue(managedMediaDispatchPath, wire); err == nil {
			t.Fatal("ambiguous notice crossed input boundary")
		}
	}
	a := managedMediaQueryAttempt("op-a", strings.Repeat("x", 256))
	if len(a) > 256 || a == managedMediaQueryAttempt("op-b", strings.Repeat("x", 256)) {
		t.Fatal("query attempt lost original namespace")
	}
}
func TestManagedMediaReportRetriesExactWireOnTheExistingStream(t *testing.T) {
	var wires [][]byte
	var callbacks []wrapper.RedisResponseCallback
	r := &managedMediaRuntime{now: time.Now, settings: managedMediaSettings{PrivateStream: "original-execution-stream", Redis: managedRedisSettings{Timeout: 1000}}}
	r.redis = managedMediaTestRedis{command: func(args []interface{}, callback wrapper.RedisResponseCallback) error {
		if args[0] != "XADD" || args[1] != "original-execution-stream" || args[4] != "media_observation" {
			t.Fatal("report used another event path")
		}
		wires = append(wires, []byte(args[6].(string)))
		callbacks = append(callbacks, callback)
		return nil
	}}
	j := &managedMediaJob{runtime: r, reports: []managedMediaReport{{Wire: []byte(`{"original":"first"}`), EventID: "first"}, {Wire: []byte(`{"original":"second"}`), EventID: "second"}}}
	j.report()
	callbacks[0](resp.ErrorValue(errors.New("lost ACK")))
	j.report()
	if !bytes.Equal(wires[0], wires[1]) || len(j.reports) != 2 {
		t.Fatal("lost ACK changed the original event")
	}
	callbacks[1](resp.StringValue("123-0"))
	callbacks[1](resp.StringValue("123-0"))
	if len(j.reports) != 1 || j.reports[0].EventID != "second" {
		t.Fatal("duplicate ACK advanced another source")
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
		"procurement":      map[string]interface{}{"identity": map[string]string{"binding_id": d.BindingID, "provider_id": d.ProviderID, "purchase_account_id": d.ProviderAccountID, "region": "original-region", "upstream_model_name": d.UpstreamModel}, "pricing_asset_id": "original-cost-asset", "cost_owner": map[string]string{"kind": "platform", "owner_id": "original-cost-owner"}, "writer": map[string]string{"producer_id": "original-producer", "role": "ai_proxy"}},
		"history":          history, "runtime_config_id": "original-runtime", "config_revision": "1", "output_identity_contract": mediaexecution.NativeImageChoicesContent}
	binding := mediaexecution.ProfileBinding{AdapterRef: d.AdapterRef, AdapterRevision: d.AdapterRevision, CreateClassifierRef: "original-native-classifier", QueryClassifierRef: "original-query-classifier", QueryCapabilityRef: "original-task-query", OutputIdentityContract: mediaexecution.NativeImageChoicesContent}
	executor, err := mediaexecution.FromPrivateArchive(managedFixtureJSON(t, archive), mediaexecution.GatewayIdentity{ProducerID: "original-producer", Actor: "service/gateway/original-producer", BootID: "original-node/original-VM"}, binding)
	if err != nil {
		t.Fatal(err)
	}
	runtime := &managedMediaRuntime{settings: managedMediaSettings{ProducerID: "original-producer", MaxResponseBytes: 1 << 20, PrivateStream: "original-execution-stream", Redis: managedRedisSettings{Timeout: 1000}}, now: time.Now}
	j := &managedMediaJob{runtime: runtime, executor: executor, notice: managedMediaNotice{SchemaVersion: "media-dispatch-notice.v2", DispatchID: "original-dispatch", OperationID: "original-operation", RequestID: "original-dispatch", TicketKey: "mf:media:ticket:original", TicketDigest: "original-ticket", ExpiresAt: now.Add(time.Minute)}, live: mediaexecution.Invocation{ID: "actual-invocation", Generation: "1"}, healthDecision: upstreamhealth.Decision{ScopeEpochs: map[string]upstreamhealth.Revision{}}}
	j.notice.Admission = managedMediaAdmission{ScopeID: "original-scope", AccountID: "original-wallet"}
	j.selection, err = executor.Selection(j.live, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	observed := []executionfacts.Envelope{}
	consumed := false
	runtime.redis = managedMediaTestRedis{command: func(args []interface{}, callback wrapper.RedisResponseCallback) error {
		switch args[0] {
		case "EVAL":
			mapping := "mf:balance:{original-scope}:account:original-wallet:principal:key:"
			if j.notice.Admission.KeyID != nil {
				mapping += *j.notice.Admission.KeyID
			}
			if args[4] != "mf:balance:{original-scope}:control" || args[5] != "mf:balance:{original-scope}:account:original-wallet" || args[6] != mapping {
				t.Fatal("media consumption must use the canonical IF05 control, original wallet and account-scoped Key mapping")
			}
			if !confirmed {
				callback(resp.ErrorValue(errors.New("unconfirmed consume")))
				return nil
			}
			if consumed {
				callback(resp.ArrayValue([]resp.Value{resp.StringValue("deny"), resp.StringValue("media_ticket_consumed")}))
				return nil
			}
			consumed = true
			callback(resp.ArrayValue([]resp.Value{resp.StringValue("allow"), resp.StringValue("123-0")}))
		case "XADD":
			if args[1] != "original-execution-stream" || args[4] != "media_observation" {
				t.Fatal("media report used another Stream")
			}
			var report struct {
				SchemaVersion string                 `json:"schema_version"`
				ProducerID    string                 `json:"producer_id"`
				OperationID   string                 `json:"operation_id"`
				Observation   string                 `json:"observation"`
				Projection    managedMediaProjection `json:"projection"`
			}
			if err := json.Unmarshal([]byte(args[6].(string)), &report); err != nil {
				t.Fatal(err)
			}
			var fact executionfacts.Envelope
			if err := json.Unmarshal([]byte(report.Observation), &fact); err != nil {
				t.Fatal(err)
			}
			if _, err := executionfacts.Freeze(fact); err != nil {
				t.Fatal(err)
			}
			if fact.Usage.Status != "reported" || fact.Usage.Quantities["image_output_count"] != "1" || len(report.Projection.Outputs) != 1 || report.Projection.OutputManifest == nil {
				t.Fatal("original output measurement missing")
			}
			observed = append(observed, fact)
			callback(resp.StringValue("124-0"))
		default:
			t.Fatal("unexpected Redis command")
		}
		return nil
	}}
	return j, &observed
}

// Uses actual Wasm SDK provider-call callbacks and the production HTTP evidence
// reader. Console replies are fixtures; this is not live supplier acceptance.
func TestManagedMediaHostNeedsFirstCommittedConsumeAndSendsOriginalAccountOnce(t *testing.T) {
	for _, test := range []struct {
		name                 string
		confirmed, keyScoped bool
	}{
		{"unknown_consume", false, false},
		{"committed_user", true, false},
		{"committed_key", true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			job, observed := managedMediaHostJob(t, test.confirmed)
			if test.keyScoped {
				key := "original-key"
				job.notice.Admission.KeyID = &key
			}
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
			if !test.confirmed {
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
