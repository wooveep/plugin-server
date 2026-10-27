package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	wasmtest "github.com/higress-group/wasm-go/pkg/test"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/resp"
)

const (
	nativeAccountFixture   = "11111111-1111-1111-1111-111111111111"
	nativeKeyFixture       = "22222222-2222-2222-2222-222222222222"
	nativeTenantFixture    = "33333333-3333-3333-3333-333333333333"
	nativePrincipalFixture = "44444444-4444-4444-4444-444444444444"
	nativeRouteFixture     = "55555555-5555-5555-5555-555555555555"
	nativeAssetFixture     = "66666666-6666-6666-6666-666666666666"
)

type nativeBillingTestContext struct {
	wrapper.HttpContext
	values     map[string]interface{}
	attributes map[string]interface{}
}

func newNativeBillingTestContext() *nativeBillingTestContext {
	return &nativeBillingTestContext{values: map[string]interface{}{}, attributes: map[string]interface{}{}}
}
func (c *nativeBillingTestContext) GetContext(key string) interface{}        { return c.values[key] }
func (c *nativeBillingTestContext) SetContext(key string, value interface{}) { c.values[key] = value }
func (c *nativeBillingTestContext) SetUserAttribute(key string, value interface{}) {
	c.attributes[key] = value
}

type nativeBillingTestRedis struct {
	wrapper.RedisClient
	commands  [][]interface{}
	callbacks []wrapper.RedisResponseCallback
	err       error
}

func (r *nativeBillingTestRedis) Command(command []interface{}, callback wrapper.RedisResponseCallback) error {
	r.commands = append(r.commands, append([]interface{}(nil), command...))
	r.callbacks = append(r.callbacks, callback)
	return r.err
}

func nativeBillingIdentityFixture() balanceidentity.Snapshot {
	return balanceidentity.Snapshot{AccountID: nativeAccountFixture, APIKeyID: nativeKeyFixture, CredentialGeneration: "9007199254740993", TenantID: nativeTenantFixture, PrincipalID: nativePrincipalFixture}
}

func nativeBillingConfigFixture(t *testing.T) (BillingConfig, map[string]interface{}) {
	t.Helper()
	descriptor := map[string]interface{}{
		"release_id": "released-e1", "target_id": "released-target", "binding_id": "released-binding",
		"provider_id": "released-provider", "provider_account_id": "released-purchase-account",
		"region_endpoint_ref": "released-region", "service_ref": "released-service", "cluster_ref": "released-cluster",
		"credential_pool_ref": "released-pool", "credential_pool_revision": "1",
		"adapter_ref": "released-text-adapter", "adapter_revision": "1", "upstream_model": "released-upstream-model",
		"mapper_digest": "sha256:" + strings.Repeat("a", 64), "proxy_digest": "sha256:" + strings.Repeat("b", 64),
		"config_canonicalization": "mf-json.v1",
		"transport_contract":      map[string]interface{}{"host": "controlled-upstream.example.test", "sni": "controlled-upstream.example.test", "path_adapter_ref": "openai-chat"},
		"capabilities":            map[string]interface{}{"text_generate": map[string]interface{}{"send_mode": "single_send", "response_classifier_ref": "openai-status-v1", "max_attempts": 1, "max_fallback_hops": 0}},
	}
	raw, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := nativeBillingDescriptorDigest(raw)
	if err != nil {
		t.Fatal(err)
	}
	descriptor["descriptor_digest"] = digest
	value := map[string]interface{}{
		"path_scope": "bound_target", "native_execution": map[string]interface{}{"producer_id": "gateway-native-e1"},
		"_modelfusion_execution": map[string]interface{}{
			"schema_version": "modelfusion-execution.v1", "descriptor": descriptor,
			"route_id": nativeRouteFixture, "sales_asset_id": nativeAssetFixture,
			"public_model": "released-public-model", "credential_ref": "released-credential", "credential_revision": "9007199254740993",
		},
	}
	cfg := BillingConfig{RedisStream: RedisStream{Stream: "private-execution-e1"}, PathScope: PathScopeLegacySuffixes, EnableMethods: []string{"GET"}, EnablePathSuffixes: []string{"/legacy-only"}}
	raw, err = json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := configureNativeBilling(gjson.ParseBytes(raw), &cfg); err != nil {
		t.Fatal(err)
	}
	return cfg, value
}

func nativeBillingDeliveryFixture(t *testing.T, cfg BillingConfig) executionfacts.Snapshot {
	t.Helper()
	configured := cfg.NativeExecution.(*nativeBillingConfig)
	release, identity := configured.release, nativeBillingIdentityFixture()
	snapshot, err := executionfacts.Freeze(executionfacts.Envelope{
		ProducerID: configured.producerID, Kind: "call_delivery", CallID: "proxy-generated-original-call",
		ReleaseID: release.Descriptor.ReleaseID, RouteID: release.RouteID, SalesAssetID: release.SalesAssetID,
		TenantID: identity.TenantID, PrincipalID: identity.PrincipalID, AccountID: identity.AccountID,
		PrincipalKind: "user", BillingMode: "customer", OperationKind: "text_generate",
		OccurredAt: "2026-10-27T10:00:00Z", ObservedAt: "2026-10-27T10:00:01Z", EvidenceRefs: []string{},
		DeliveryState: "succeeded", ServedFrom: "upstream", PublicModel: release.PublicModel,
		Usage: executionfacts.MissingTokenUsage("provider_usage_missing"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func nativeBillingBridgeFixture(t *testing.T, snapshot executionfacts.Snapshot) string {
	t.Helper()
	value, err := executionfacts.EncodeDeliveryBridge(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func nativeBillingRequestFixture(t *testing.T, cfg BillingConfig) (*nativeBillingTestContext, *nativeBillingState) {
	t.Helper()
	ctx := newNativeBillingTestContext()
	state := &nativeBillingState{config: cfg.NativeExecution.(*nativeBillingConfig)}
	ctx.SetContext(nativeBillingContextKey, state)
	headers := append(nativeBillingIdentityFixture().Headers(), [2]string{"x-modelfusion-call-id", "client-invented-call"}, [2]string{"x-modelfusion-attempt-id", "client-invented-attempt"})
	captureNativeBillingIdentity(ctx, state, headers)
	if !state.valid || state.identity.CredentialGeneration != "9007199254740993" {
		t.Fatal("original exact authenticated caller was not retained")
	}
	return ctx, state
}

func TestNativeBillingRequiresPairedFrozenConfigAndClearsInheritedLegacyScope(t *testing.T) {
	cfg, value := nativeBillingConfigFixture(t)
	if cfg.NativeExecution == nil || cfg.PathScope != PathScopeBoundTarget || len(cfg.EnableMethods) != 0 || len(cfg.EnablePathSuffixes) != 0 {
		t.Fatal("native route inherited legacy suffix or method restrictions")
	}
	changes := []func(map[string]interface{}){
		func(v map[string]interface{}) { delete(v, "native_execution") },
		func(v map[string]interface{}) { delete(v, "_modelfusion_execution") },
		func(v map[string]interface{}) { v["path_scope"] = "legacy_suffixes" },
		func(v map[string]interface{}) { v["enable_methods"] = []string{"POST"} },
		func(v map[string]interface{}) { v["enable_path_suffixes"] = []string{"/chat/completions"} },
		func(v map[string]interface{}) {
			v["native_execution"].(map[string]interface{})["producer_role"] = "ai_proxy"
		},
		func(v map[string]interface{}) {
			v["_modelfusion_execution"].(map[string]interface{})["sales_asset_id"] = "not-an-asset-uuid"
		},
		func(v map[string]interface{}) {
			v["_modelfusion_execution"].(map[string]interface{})["descriptor"].(map[string]interface{})["target_id"] = "different-target-with-stale-digest"
		},
		func(v map[string]interface{}) {
			v["_modelfusion_execution"].(map[string]interface{})["credential_revision"] = float64(9007199254740993)
		},
	}
	original, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	for index, change := range changes {
		var changed map[string]interface{}
		if err := json.Unmarshal(original, &changed); err != nil {
			t.Fatal(err)
		}
		change(changed)
		raw, err := json.Marshal(changed)
		if err != nil {
			t.Fatal(err)
		}
		if err := configureNativeBilling(gjson.ParseBytes(raw), &cfg); err == nil || cfg.NativeExecution != nil {
			t.Fatalf("configuration case %d retained inherited authority", index)
		}
	}
	if err := configureNativeBilling(gjson.Parse(`{}`), &cfg); err != nil || cfg.NativeExecution != nil {
		t.Fatal("unmatched/global configuration retained native route authority")
	}
}

func TestNativeBillingCanonicalDescriptorUsesUTF8AndExactIntegerLimits(t *testing.T) {
	raw := []byte("{\"z\":4294967295,\"descriptor_digest\":\"ignored\",\"nested\":{\"limit\":0},\"a\":\"<&\u2028\"}")
	canonical := []byte("{\"a\":\"<&\u2028\",\"nested\":{\"limit\":0},\"z\":4294967295}")
	sum := sha256.Sum256(canonical)
	want := "sha256:" + hex.EncodeToString(sum[:])
	got, err := nativeBillingDescriptorDigest(raw)
	if err != nil || got != want {
		t.Fatal("descriptor digest changed UTF-8 escaping or integer bytes")
	}
	for _, raw := range []string{`{"a":1,"a":2}`, `{"a":"\uD800"}`, `{"a":null}`, `{"a":1} {"b":2}`} {
		if validateNativeBillingJSON([]byte(raw)) == nil {
			t.Fatal("ambiguous native config was accepted")
		}
	}
}

func TestNativeBillingDescriptorRejectsStaleOriginalFieldDigests(t *testing.T) {
	cases := []struct {
		name   string
		change func(map[string]interface{})
	}{
		{"cluster", func(d map[string]interface{}) { d["cluster_ref"] = "another-released-cluster" }},
		{"transport", func(d map[string]interface{}) {
			d["transport_contract"].(map[string]interface{})["path_adapter_ref"] = "another-path-adapter"
		}},
		{"capabilities", func(d map[string]interface{}) {
			d["capabilities"].(map[string]interface{})["text_generate"].(map[string]interface{})["response_classifier_ref"] = "another-response-classifier"
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			cfg, value := nativeBillingConfigFixture(t)
			descriptor := value["_modelfusion_execution"].(map[string]interface{})["descriptor"].(map[string]interface{})
			test.change(descriptor)
			raw, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if err := configureNativeBilling(gjson.ParseBytes(raw), &cfg); err == nil || cfg.NativeExecution != nil {
				t.Fatal("billing retained stale descriptor authority after original fields changed")
			}
		})
	}
}

func TestNativeBillingDescriptorAcceptsCompleteSDKShape(t *testing.T) {
	cfg, value := nativeBillingConfigFixture(t)
	descriptor := value["_modelfusion_execution"].(map[string]interface{})["descriptor"].(map[string]interface{})
	descriptor["transport_contract"].(map[string]interface{})["sni"] = ""
	capabilities := descriptor["capabilities"].(map[string]interface{})
	text := capabilities["text_generate"].(map[string]interface{})
	text["query_capability_ref"], text["provider_idempotency_ref"] = "query-text-v1", "text-idempotency-v1"
	capabilities["image_generate"] = map[string]interface{}{
		"send_mode": "observable_retry", "response_classifier_ref": "image-status-v1",
		"query_capability_ref": "query-v1", "provider_idempotency_ref": "provider-idempotency-v1",
		"max_attempts": uint32(4294967295), "max_fallback_hops": 0,
	}
	// Independent mf-json.v1 golden includes the complete original SDK object.
	descriptor["descriptor_digest"] = "sha256:dbf45ae172612dc85c0ffba4f7abc61a5739e5a5aacffeed584f51ede7cb2461"
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := configureNativeBilling(gjson.ParseBytes(raw), &cfg); err != nil {
		t.Fatal(err)
	}
	configured, ok := cfg.NativeExecution.(*nativeBillingConfig)
	if !ok || configured.release.Descriptor.DescriptorDigest != descriptor["descriptor_digest"] || configured.release.Descriptor.Capabilities["image_generate"].MaxAttempts != 4294967295 {
		t.Fatal("billing changed the full released descriptor or its exact integer limits")
	}
}

func TestNativeBillingDescriptorDigestPreservesRawFieldsBeforeRemovingSelf(t *testing.T) {
	raw := []byte(`{"descriptor_digest":"ignored","capabilities":{"text_generate":{"query_capability_ref":""}},"transport_contract":{"sni":""}}`)
	canonical := []byte(`{"capabilities":{"text_generate":{"query_capability_ref":""}},"transport_contract":{"sni":""}}`)
	sum := sha256.Sum256(canonical)
	got, err := nativeBillingDescriptorDigest(raw)
	if err != nil || got != "sha256:"+hex.EncodeToString(sum[:]) {
		t.Fatal("billing descriptor digest dropped original explicitly empty fields")
	}
	for _, raw := range []string{
		`{"descriptor_digest":"one","descriptor_digest":"two","a":1}`,
		`{"descriptor_digest":"one","nested":{"a":1,"a":2}}`,
		`[]`,
	} {
		if _, err := nativeBillingDescriptorDigest([]byte(raw)); err == nil {
			t.Fatal("billing swallowed ambiguous original descriptor fields")
		}
	}
}

func TestNativeBillingBridgePublishesExactlyOriginalBytesAndSameACKLedger(t *testing.T) {
	wasmtest.RunGoTest(t, func(t *testing.T) {
		cfg, _ := nativeBillingConfigFixture(t)
		redis := &nativeBillingTestRedis{}
		cfg.redisClient = redis
		ctx, state := nativeBillingRequestFixture(t, cfg)
		snapshot := nativeBillingDeliveryFixture(t, cfg)
		header := nativeBillingBridgeFixture(t, snapshot)
		consumeNativeBillingBridge(ctx, cfg, state, header)
		if len(redis.commands) != 1 || state.delivery == nil || state.delivery.Status().State != "in_flight" {
			t.Fatal("delivery skipped XADD or claimed an ACK early")
		}
		command := redis.commands[0]
		if len(command) != 7 || command[0] != "XADD" || command[1] != cfg.RedisStream.Stream || command[3] != "message_type" || command[4] != "observation" || command[5] != "event" || !bytes.Equal([]byte(command[6].(string)), snapshot.Bytes()) {
			t.Fatal("billing changed the proxy's event or emitted a legacy BillingEvent")
		}
		if !bytes.Contains(snapshot.Bytes(), []byte(`"status":"missing"`)) || bytes.Contains(snapshot.Bytes(), []byte(`"quantities"`)) {
			t.Fatal("missing provider usage became a free reported zero")
		}
		consumeNativeBillingBridge(ctx, cfg, state, header)
		if len(redis.commands) != 1 {
			t.Fatal("same in-flight bridge appended twice")
		}
		state.delivery.MarkUnknown(executionDeliveryObserver(ctx))
		if _, err := dispatchExecutionFact(ctx, redis, cfg.RedisStream.Stream, state.snapshot); err != nil {
			t.Fatal(err)
		}
		if len(redis.commands) != 2 || redis.commands[0][6] != redis.commands[1][6] {
			t.Fatal("explicit ACK-unknown resend changed original bytes")
		}
		redis.callbacks[0](resp.StringValue("10-0"))
		if state.delivery.Status().State != "in_flight" {
			t.Fatal("stale callback overwrote the new append")
		}
		redis.callbacks[1](resp.StringValue("11-0"))
		consumeNativeBillingBridge(ctx, cfg, state, header)
		onNativeBillingStreamDone(ctx, cfg)
		if len(redis.commands) != 2 || state.delivery.Status().State != "acknowledged" {
			t.Fatal("completed stream redelivered or lost a real insertion ACK")
		}
	})
}

func TestNativeBillingRejectsBridgeCallerReleaseAndProducerDrift(t *testing.T) {
	wasmtest.RunGoTest(t, func(t *testing.T) {
		cfg, _ := nativeBillingConfigFixture(t)
		changes := []func(*executionfacts.Envelope){
			func(e *executionfacts.Envelope) { e.ProducerID = "another-configured-producer" },
			func(e *executionfacts.Envelope) { e.AccountID = "99999999-9999-9999-9999-999999999999" },
			func(e *executionfacts.Envelope) { e.TenantID = "99999999-9999-9999-9999-999999999999" },
			func(e *executionfacts.Envelope) { e.PrincipalID = "99999999-9999-9999-9999-999999999999" },
			func(e *executionfacts.Envelope) { e.ReleaseID = "another-release" },
			func(e *executionfacts.Envelope) { e.RouteID = "99999999-9999-9999-9999-999999999999" },
			func(e *executionfacts.Envelope) { e.SalesAssetID = "99999999-9999-9999-9999-999999999999" },
			func(e *executionfacts.Envelope) { e.PublicModel = "final-response-model-is-not-sale-owner" },
			func(e *executionfacts.Envelope) { e.AttemptID = "unsupported-purchase-attribution" },
		}
		for index, change := range changes {
			redis := &nativeBillingTestRedis{}
			cfg.redisClient = redis
			ctx, state := nativeBillingRequestFixture(t, cfg)
			envelope := nativeBillingDeliveryFixture(t, cfg).Envelope()
			change(&envelope)
			envelope.PayloadDigest = ""
			snapshot, err := executionfacts.Freeze(envelope)
			if err != nil {
				t.Fatal(err)
			}
			consumeNativeBillingBridge(ctx, cfg, state, nativeBillingBridgeFixture(t, snapshot))
			if len(redis.commands) != 0 || !state.gap || ctx.attributes["execution_fact_gap"] != true {
				t.Fatalf("bridge drift case %d emitted a sale", index)
			}
		}
	})
}

func TestNativeBillingMissingBridgeOrRedisFailureRecordsQ41WithoutFabricatedFacts(t *testing.T) {
	wasmtest.RunGoTest(t, func(t *testing.T) {
		cfg, _ := nativeBillingConfigFixture(t)
		ctx, state := nativeBillingRequestFixture(t, cfg)
		onNativeBillingStreamDone(ctx, cfg)
		if !state.gap || state.delivery != nil || ctx.attributes["execution_fact_gap_reason"] != "delivery_bridge_missing" {
			t.Fatal("missing bridge fabricated a successful delivery")
		}
		for _, dispatchError := range []error{nil, errors.New("sensitive redis endpoint failure")} {
			redis := &nativeBillingTestRedis{err: dispatchError}
			cfg.redisClient = redis
			ctx, state := nativeBillingRequestFixture(t, cfg)
			consumeNativeBillingBridge(ctx, cfg, state, nativeBillingBridgeFixture(t, nativeBillingDeliveryFixture(t, cfg)))
			onNativeBillingStreamDone(ctx, cfg)
			if state.delivery == nil || (state.delivery.Status().State != "unknown" && state.delivery.Status().State != "gap") || ctx.attributes["execution_fact_gap"] != true {
				t.Fatal("Redis failure/ACK loss was treated as successful billing")
			}
			if strings.Contains(state.delivery.Status().Reason, "sensitive") {
				t.Fatal("raw dependency error entered ordinary gap diagnostics")
			}
		}
		ctx, state = nativeBillingRequestFixture(t, cfg)
		headers := nativeBillingIdentityFixture().Headers()
		headers = append(headers, [2]string{balanceidentity.AccountHeader, nativeAccountFixture})
		state.valid = false
		captureNativeBillingIdentity(ctx, state, headers)
		if state.valid || !state.gap {
			t.Fatal("duplicate caller metadata granted original authority")
		}
	})
}

func TestNativeBillingAlwaysStripsEveryBridgeHeaderBeforeValidation(t *testing.T) {
	headers := [][2]string{{":status", "200"}, {"content-type", "application/json"}, {executionfacts.DeliveryBridgeHeader, "not-a-valid-bridge"}, {strings.ToUpper(executionfacts.DeliveryBridgeHeader), "another-untrusted-value"}}
	_, clean, found, err := splitNativeDeliveryBridge(headers)
	if !found || err == nil || len(clean) != 2 || clean[0] != headers[0] || clean[1] != headers[1] {
		t.Fatal("duplicate bridge metadata escaped stripping or changed the actual response")
	}
	for _, header := range clean {
		if strings.EqualFold(header[0], executionfacts.DeliveryBridgeHeader) {
			t.Fatal("private delivery header reached the client")
		}
	}
}
