package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

var executionAdmissionCaller = balanceidentity.Snapshot{
	AccountID: "10000000-0000-0000-0000-000000000001", APIKeyID: "20000000-0000-0000-0000-000000000001",
	TenantID: "30000000-0000-0000-0000-000000000001", PrincipalID: "40000000-0000-0000-0000-000000000001",
	CredentialGeneration: "9007199254740993",
}

func executionAdmissionFixture(purpose string) map[string]interface{} {
	entry := map[string]interface{}{
		"schema_version": executionAdmissionSchema, "call_identity_source": "admission", "release_id": "release-original", "route_id": "50000000-0000-0000-0000-000000000001",
		"purpose": purpose, "combination_key": "sha256:" + strings.Repeat("a", 64), "profile_id": "e1-single-text",
		"console_internal": map[string]interface{}{
			"service_name": "modelfusion-console.modelfusion-system.svc.cluster.local", "service_port": 80,
			"host": "console.internal", "auth_token": "controlled-existing-secret", "timeout_ms": 1000,
		},
	}
	if purpose == "validation" {
		entry["access_id"], entry["revision"] = "validation-access-original", "9007199254740993"
	}
	return map[string]interface{}{"path_scope": PathScopeBoundTarget, "execution_admission": entry}
}

func executionAdmissionJSON(t *testing.T, value interface{}) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func executionAdmissionParse(t *testing.T, fixture map[string]interface{}) (*executionAdmissionConfig, error) {
	t.Helper()
	cfg := QuotaConfig{PathScope: PathScopeBoundTarget}
	err := configureExecutionAdmission(gjson.ParseBytes(executionAdmissionJSON(t, fixture)), &cfg)
	if err != nil {
		return nil, err
	}
	parsed, _ := cfg.ExecutionAdmission.(*executionAdmissionConfig)
	return parsed, nil
}

func TestExecutionAdmissionConfigUsesCompleteRuleAuthority(t *testing.T) {
	for _, purpose := range []string{"production", "validation"} {
		t.Run(purpose, func(t *testing.T) {
			parsed, err := executionAdmissionParse(t, executionAdmissionFixture(purpose))
			if err != nil || parsed == nil || parsed.Purpose != purpose || parsed.cluster != "outbound|80||modelfusion-console.modelfusion-system.svc.cluster.local" {
				t.Fatalf("configuration: %#v, %v", parsed, err)
			}
			cfg := QuotaConfig{PathScope: PathScopeBoundTarget, ExecutionAdmission: parsed}
			if err := configureExecutionAdmission(gjson.Parse(`{}`), &cfg); err != nil || cfg.ExecutionAdmission != nil {
				t.Fatalf("absent new scope inherited authority: %#v, %v", cfg.ExecutionAdmission, err)
			}
			cfg.ExecutionAdmission = parsed
			if err := configureExecutionAdmission(gjson.Parse(`{"execution_admission":{"schema_version":"managed-execution-admission.v1"}}`), &cfg); err == nil || cfg.ExecutionAdmission != nil {
				t.Fatal("invalid new scope inherited authority")
			}
		})
	}
}

func TestExecutionAdmissionConfigRejectsAmbiguityAndManualFilters(t *testing.T) {
	for name, mutate := range map[string]func(map[string]interface{}){
		"unknown field":          func(entry map[string]interface{}) { entry["role"] = "validation_service" },
		"schema":                 func(entry map[string]interface{}) { entry["schema_version"] = "future.v2" },
		"missing call origin":    func(entry map[string]interface{}) { delete(entry, "call_identity_source") },
		"wrong call origin":      func(entry map[string]interface{}) { entry["call_identity_source"] = "proxy" },
		"non-string call origin": func(entry map[string]interface{}) { entry["call_identity_source"] = true },
		"missing release":        func(entry map[string]interface{}) { delete(entry, "release_id") },
		"non native route":       func(entry map[string]interface{}) { entry["route_id"] = "client-route" },
		"digest":                 func(entry map[string]interface{}) { entry["combination_key"] = "sha256:" + strings.Repeat("A", 64) },
		"production access":      func(entry map[string]interface{}) { entry["access_id"] = "validation-access" },
		"production revision":    func(entry map[string]interface{}) { entry["revision"] = "1" },
		"credential whitespace": func(entry map[string]interface{}) {
			entry["console_internal"].(map[string]interface{})["auth_token"] = "secret\r\nheader"
		},
		"user authority": func(entry map[string]interface{}) {
			entry["console_internal"].(map[string]interface{})["host"] = "user@console.internal"
		},
		"missing service": func(entry map[string]interface{}) {
			delete(entry["console_internal"].(map[string]interface{}), "service_name")
		},
		"zero timeout": func(entry map[string]interface{}) {
			entry["console_internal"].(map[string]interface{})["timeout_ms"] = 0
		},
		"extra console authority": func(entry map[string]interface{}) {
			entry["console_internal"].(map[string]interface{})["url"] = "http://other.internal"
		},
	} {
		t.Run(name, func(t *testing.T) {
			fixture := executionAdmissionFixture("production")
			mutate(fixture["execution_admission"].(map[string]interface{}))
			if _, err := executionAdmissionParse(t, fixture); err == nil {
				t.Fatal("unsafe configuration accepted")
			}
		})
	}
	for name, cfg := range map[string]QuotaConfig{
		"legacy scope":  {PathScope: PathScopeLegacySuffixes},
		"method filter": {PathScope: PathScopeBoundTarget, EnableMethods: []string{"POST"}},
		"path filter":   {PathScope: PathScopeBoundTarget, EnablePathSuffixes: []string{"/v1/chat/completions"}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := configureExecutionAdmission(gjson.ParseBytes(executionAdmissionJSON(t, executionAdmissionFixture("production"))), &cfg); err == nil {
				t.Fatal("manual managed scope accepted")
			}
		})
	}
	for _, raw := range []string{
		`{"execution_admission":null}`,
		`{"execution_admission":{},"execution_admission":{}}`,
		`{"execution_admission":{"schema_version":"managed-execution-admission.v1","schema_version":"managed-execution-admission.v1"}}`,
	} {
		cfg := QuotaConfig{PathScope: PathScopeBoundTarget}
		if err := configureExecutionAdmission(gjson.Parse(raw), &cfg); err == nil {
			t.Fatalf("ambiguous configuration accepted: %s", raw)
		}
	}
	for _, revision := range []interface{}{nil, "0", "01", "-1", "1.0", "340282366920938463463374607431768211456", int64(9007199254740993)} {
		fixture := executionAdmissionFixture("validation")
		fixture["execution_admission"].(map[string]interface{})["revision"] = revision
		if _, err := executionAdmissionParse(t, fixture); err == nil {
			t.Fatalf("inexact validation revision accepted: %#v", revision)
		}
	}
}

type executionAdmissionHost struct {
	host     proxytest.HostEmulator
	id       uint32
	ctx      wrapper.HttpContext
	cfg      QuotaConfig
	identity balanceidentity.Snapshot
	allowed  int
}

// This runs real Wasm SDK host-call callbacks. It proves control-query and
// resume behavior, not a deployed Console decision or a provider execution.
func newExecutionAdmissionHost(t *testing.T, fixture map[string]interface{}) *executionAdmissionHost {
	t.Helper()
	h := &executionAdmissionHost{}
	vm := wrapper.NewCommonVmCtx("ai-quota-execution-admission-test",
		wrapper.ParseConfig(func(value gjson.Result, cfg *QuotaConfig) error {
			cfg.PathScope = value.Get("path_scope").String()
			if err := configureExecutionAdmission(value, cfg); err != nil {
				return err
			}
			h.cfg = *cfg
			return nil
		}),
		wrapper.ProcessRequestHeaders(func(ctx wrapper.HttpContext, cfg QuotaConfig) types.Action {
			h.ctx = ctx
			headers, _ := proxywasm.GetHttpRequestHeaders()
			h.identity, _ = balanceidentity.Read(headers)
			checkExecutionAdmission(ctx, cfg, h.identity, h.onAllowed)
			return types.HeaderStopAllIterationAndWatermark
		}),
	)
	host, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption().WithVMContext(vm).WithPluginConfiguration(executionAdmissionJSON(t, fixture)))
	t.Cleanup(reset)
	h.host = host
	host.RegisterForeignFunction("get_log_level", func([]byte) []byte { return []byte{0, 0, 0, 0} })
	if host.StartVM() != types.OnVMStartStatusOK || host.StartPlugin() != types.OnPluginStartStatusOK {
		t.Fatal("admission test host did not start")
	}
	h.id = host.InitializeHttpContext()
	return h
}

func (h *executionAdmissionHost) onAllowed() {
	h.allowed++
	_ = proxywasm.ResumeHttpRequest()
}

func (h *executionAdmissionHost) start(t *testing.T, identity balanceidentity.Snapshot) executionAdmissionRequest {
	t.Helper()
	headers := append([][2]string{{":method", "POST"}, {":path", "/custom/customer/alias"}, {":authority", "gateway.example"}}, identity.Headers()...)
	for _, name := range []string{executionAdmissionCallHeader, executionAdmissionAttemptHeader, executionAdmissionReleaseHeader, executionAdmissionDeadlineHeader, executionAdmissionOccurredHeader} {
		headers = append(headers, [2]string{name, "forged-client"}, [2]string{strings.ToUpper(name), "forged-client-duplicate"})
	}
	if action := h.host.CallOnRequestHeaders(h.id, headers, false); action != types.HeaderStopAllIterationAndWatermark {
		t.Fatalf("unexpected headers action: %v", action)
	}
	callouts := h.host.GetCalloutAttributesFromContext(h.id)
	if len(callouts) == 0 {
		return executionAdmissionRequest{}
	}
	if len(callouts) != 1 {
		t.Fatalf("control query count: %d", len(callouts))
	}
	var request executionAdmissionRequest
	if err := json.Unmarshal(callouts[0].Body, &request); err != nil {
		t.Fatal(err)
	}
	return request
}

func executionAdmissionAllowedReceipt(request executionAdmissionRequest) map[string]interface{} {
	return map[string]interface{}{
		"call_id": request.CallID, "access_id": request.AccessID, "allowed": true, "current_revision": request.Revision,
		"valid_until": time.Now().UTC().Add(5 * time.Second).Format(time.RFC3339Nano), "reason": "allowed",
		"purpose": request.Purpose, "release_id": request.ReleaseID, "combination_key": request.CombinationKey, "profile_id": request.ProfileID,
		"tenant_id": request.TenantID, "route_id": request.RouteID, "account_id": request.AccountID,
		"principal_id": request.PrincipalID, "api_key_id": request.APIKeyID, "credential_generation": request.CredentialGeneration,
	}
}

func (h *executionAdmissionHost) response(t *testing.T, headers [][2]string, raw []byte) {
	t.Helper()
	callouts := h.host.GetCalloutAttributesFromContext(h.id)
	if len(callouts) != 1 {
		t.Fatalf("expected one pending query, got %d", len(callouts))
	}
	h.host.CallOnHttpCallResponse(callouts[0].CalloutID, headers, nil, raw)
	if len(h.host.GetCalloutAttributesFromContext(h.id)) != 0 || len(h.host.GetRedisCalloutAttributesFromContext(h.id)) != 0 {
		t.Fatal("control callback sent another HTTP query or changed money in Redis")
	}
}

func executionAdmissionResponseHeaders() [][2]string {
	return [][2]string{{":status", "200"}, {"content-type", "application/json"}, {executionAdmissionOccurredHeader, "2026-10-27T10:00:00Z"}}
}

func (h *executionAdmissionHost) assertDenied(t *testing.T, status uint32) {
	t.Helper()
	response := h.host.GetSentLocalResponse(h.id)
	if h.allowed != 0 || response == nil || response.StatusCode != status || h.host.GetCurrentHttpStreamAction(h.id) == types.ActionContinue {
		t.Fatalf("admission escaped: allows=%d response=%#v action=%v", h.allowed, response, h.host.GetCurrentHttpStreamAction(h.id))
	}
	for _, name := range []string{executionAdmissionAttemptHeader, executionAdmissionReleaseHeader, executionAdmissionDeadlineHeader, executionAdmissionOccurredHeader} {
		if executionAdmissionHeaderCount(h.host.GetCurrentRequestHeaders(h.id), name) != 0 {
			t.Fatalf("failed check installed trusted header %s", name)
		}
	}
}

func TestProductionAdmissionMintsOriginalCallWithoutConsole(t *testing.T) {
	for _, retainEndpoint := range []bool{false, true} {
		t.Run(fmt.Sprint(retainEndpoint), func(t *testing.T) {
			fixture := executionAdmissionFixture("production")
			if !retainEndpoint {
				delete(fixture["execution_admission"].(map[string]interface{}), "console_internal")
			}
			h := newExecutionAdmissionHost(t, fixture)
			before := time.Now().UTC()
			h.start(t, executionAdmissionCaller)
			if h.allowed != 1 || h.host.GetSentLocalResponse(h.id) != nil || len(h.host.GetCalloutAttributesFromContext(h.id)) != 0 {
				t.Fatal("production still depends on a Console response")
			}
			origin, err := executionfacts.ReadAdmittedCall(h.host.GetCurrentRequestHeaders(h.id), "release-original", time.Now())
			if err != nil {
				t.Fatal(err)
			}
			callID, err := uuid.Parse(origin.CallID)
			occurred, timeErr := time.Parse(time.RFC3339Nano, origin.OccurredAt)
			if err != nil || callID.Version() != 4 || timeErr != nil || occurred.Before(before) || !origin.ValidUntil.Equal(occurred.Add(5*time.Second)) {
				t.Fatal("production did not retain its original local admission identity and time")
			}
			if executionAdmissionHeaderCount(h.host.GetCurrentRequestHeaders(h.id), executionAdmissionAttemptHeader) != 0 {
				t.Fatal("quota invented a provider attempt")
			}
			checkExecutionAdmission(h.ctx, h.cfg, h.identity, h.onAllowed)
			if h.allowed != 1 || len(h.host.GetCalloutAttributesFromContext(h.id)) != 0 || len(h.host.GetRedisCalloutAttributesFromContext(h.id)) != 0 {
				t.Fatal("repeated callback created another call or changed a balance")
			}
		})
	}
}

func TestExecutionAdmissionControlledQueryAndExactValidationPermit(t *testing.T) {
	h := newExecutionAdmissionHost(t, executionAdmissionFixture("validation"))
	request := h.start(t, executionAdmissionCaller)
	parsed, err := uuid.Parse(request.CallID)
	if err != nil || parsed.Version() != 4 || request.CallID == "forged-client" || request.CredentialGeneration != "9007199254740993" {
		t.Fatalf("fresh call and exact authenticated identity missing: %#v", request)
	}
	callout := h.host.GetCalloutAttributesFromContext(h.id)[0]
	if callout.Upstream != "outbound|80||modelfusion-console.modelfusion-system.svc.cluster.local" {
		t.Fatalf("uncontrolled upstream: %s", callout.Upstream)
	}
	for name, expected := range map[string]string{":method": "POST", ":path": executionAdmissionPath, ":authority": "console.internal", "authorization": "Bearer controlled-existing-secret"} {
		if actual, unique := executionAdmissionSingleHeader(callout.Headers, name); !unique || actual != expected {
			t.Fatalf("control query %s: %q unique=%v", name, actual, unique)
		}
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(callout.Body, &wire); err != nil || len(wire) != 13 || string(wire["access_id"]) != `"validation-access-original"` || string(wire["revision"]) != `"9007199254740993"` {
		t.Fatalf("validation wire lost its exact grant: %s", callout.Body)
	}
	if len(h.host.GetRedisCalloutAttributesFromContext(h.id)) != 0 {
		t.Fatal("license query performed a monetary write")
	}
	before := h.host.GetCurrentRequestHeaders(h.id)
	if value, unique := executionAdmissionSingleHeader(before, executionAdmissionCallHeader); !unique || value != request.CallID {
		t.Fatal("fresh call header missing")
	}
	for _, name := range []string{executionAdmissionAttemptHeader, executionAdmissionReleaseHeader, executionAdmissionDeadlineHeader, executionAdmissionOccurredHeader} {
		if executionAdmissionHeaderCount(before, name) != 0 {
			t.Fatalf("client value survived before receipt: %s", name)
		}
	}
	receipt := executionAdmissionAllowedReceipt(request)
	h.response(t, executionAdmissionResponseHeaders(), executionAdmissionJSON(t, receipt))
	if h.allowed != 1 || h.host.GetSentLocalResponse(h.id) != nil || h.host.GetCurrentHttpStreamAction(h.id) != types.ActionContinue {
		t.Fatal("complete current validation permit did not invoke allow exactly once")
	}
	after := h.host.GetCurrentRequestHeaders(h.id)
	for name, expected := range map[string]string{executionAdmissionCallHeader: request.CallID, executionAdmissionReleaseHeader: request.ReleaseID, executionAdmissionDeadlineHeader: receipt["valid_until"].(string), executionAdmissionOccurredHeader: "2026-10-27T10:00:00Z"} {
		if actual, unique := executionAdmissionSingleHeader(after, name); !unique || actual != expected {
			t.Fatalf("trusted permit header %s: %q unique=%v", name, actual, unique)
		}
	}
	if executionAdmissionHeaderCount(after, executionAdmissionAttemptHeader) != 0 {
		t.Fatal("license query minted a provider attempt")
	}
	checkExecutionAdmission(h.ctx, h.cfg, h.identity, h.onAllowed)
	if h.allowed != 1 || len(h.host.GetCalloutAttributesFromContext(h.id)) != 0 {
		t.Fatal("completed call reused its permit callback")
	}
}

func TestExecutionAdmissionValidationUsesExactCurrentRevision(t *testing.T) {
	h := newExecutionAdmissionHost(t, executionAdmissionFixture("validation"))
	request := h.start(t, executionAdmissionCaller)
	if request.AccessID == nil || *request.AccessID != "validation-access-original" || request.Revision == nil || *request.Revision != "9007199254740993" {
		t.Fatalf("validation permit lost exact source revision: %#v", request)
	}
	h.response(t, executionAdmissionResponseHeaders(), executionAdmissionJSON(t, executionAdmissionAllowedReceipt(request)))
	if h.allowed != 1 {
		t.Fatal("current scoped validation permit did not allow")
	}
}

func TestExecutionAdmissionRejectsEveryMismatchedEcho(t *testing.T) {
	for _, field := range []string{"call_id", "access_id", "purpose", "release_id", "combination_key", "profile_id", "tenant_id", "route_id", "account_id", "principal_id", "api_key_id", "credential_generation", "current_revision"} {
		t.Run(field, func(t *testing.T) {
			h := newExecutionAdmissionHost(t, executionAdmissionFixture("validation"))
			request := h.start(t, executionAdmissionCaller)
			receipt := executionAdmissionAllowedReceipt(request)
			receipt[field] = "different-original-scope"
			h.response(t, executionAdmissionResponseHeaders(), executionAdmissionJSON(t, receipt))
			h.assertDenied(t, 503)
		})
	}
}

func TestExecutionAdmissionRequiresFreshCompleteStrictReceipt(t *testing.T) {
	for name, mutate := range map[string]func(map[string]interface{}){
		"denied":                    func(receipt map[string]interface{}) { receipt["allowed"], receipt["reason"] = false, "permit_revoked" },
		"nullable allowed":          func(receipt map[string]interface{}) { receipt["allowed"] = nil },
		"nullable current revision": func(receipt map[string]interface{}) { receipt["current_revision"] = nil },
		"numeric revision":          func(receipt map[string]interface{}) { receipt["current_revision"] = int64(9007199254740993) },
		"old revision":              func(receipt map[string]interface{}) { receipt["current_revision"] = "9007199254740992" },
		"missing deadline":          func(receipt map[string]interface{}) { delete(receipt, "valid_until") },
		"nullable deadline":         func(receipt map[string]interface{}) { receipt["valid_until"] = nil },
		"expired deadline": func(receipt map[string]interface{}) {
			receipt["valid_until"] = time.Now().UTC().Add(-time.Second).Format(time.RFC3339Nano)
		},
		"non UTC deadline": func(receipt map[string]interface{}) {
			receipt["valid_until"] = time.Now().In(time.FixedZone("offset", 3600)).Add(time.Second).Format(time.RFC3339Nano)
		},
		"unknown offset":          func(receipt map[string]interface{}) { receipt["valid_until"] = "2099-01-01T00:00:00-00:00" },
		"unapproved reason":       func(receipt map[string]interface{}) { receipt["reason"] = "pending" },
		"extra self granted role": func(receipt map[string]interface{}) { receipt["role"] = "validation_service" },
		"missing echo":            func(receipt map[string]interface{}) { delete(receipt, "principal_id") },
	} {
		t.Run(name, func(t *testing.T) {
			h := newExecutionAdmissionHost(t, executionAdmissionFixture("validation"))
			receipt := executionAdmissionAllowedReceipt(h.start(t, executionAdmissionCaller))
			mutate(receipt)
			h.response(t, executionAdmissionResponseHeaders(), executionAdmissionJSON(t, receipt))
			status := uint32(503)
			if name == "denied" {
				status = 403
			}
			h.assertDenied(t, status)
		})
	}
	for name, transform := range map[string]func([]byte) []byte{
		"duplicate allowed": func(raw []byte) []byte { return append([]byte(`{"allowed":true,`), raw[1:]...) },
		"alias scope": func(raw []byte) []byte {
			return []byte(strings.Replace(string(raw), `"release_id":`, `"Release_ID":`, 1))
		},
		"truncated JSON":      func(raw []byte) []byte { return raw[:len(raw)-1] },
		"trailing JSON":       func(raw []byte) []byte { return append(raw, []byte(` {}`)...) },
		"ApiResponse wrapper": func(raw []byte) []byte { return append(append([]byte(`{"code":0,"data":`), raw...), '}') },
		"oversized body": func(raw []byte) []byte {
			return append(raw, []byte(strings.Repeat(" ", executionAdmissionMaxJSONBytes))...)
		},
		"lone surrogate": func(raw []byte) []byte { return []byte(strings.Replace(string(raw), `"allowed"`, `"\ud800"`, 1)) },
	} {
		t.Run(name, func(t *testing.T) {
			h := newExecutionAdmissionHost(t, executionAdmissionFixture("validation"))
			raw := executionAdmissionJSON(t, executionAdmissionAllowedReceipt(h.start(t, executionAdmissionCaller)))
			h.response(t, executionAdmissionResponseHeaders(), transform(raw))
			h.assertDenied(t, 503)
		})
	}
}

func TestExecutionAdmissionDoesNotInventMissingHTTPStatusOrAcceptTruncation(t *testing.T) {
	for name, headers := range map[string][][2]string{
		"missing status":           {{"content-type", "application/json"}},
		"duplicate status":         {{":status", "200"}, {":status", "200"}, {"content-type", "application/json"}},
		"real forbidden":           {{":status", "403"}, {"content-type", "application/json"}},
		"real unavailable":         {{":status", "503"}, {"content-type", "application/json"}},
		"missing content type":     {{":status", "200"}},
		"duplicate content type":   {{":status", "200"}, {"content-type", "application/json"}, {"Content-Type", "application/json"}},
		"compressed":               {{":status", "200"}, {"content-type", "application/json"}, {"content-encoding", "gzip"}},
		"duplicate encoding":       {{":status", "200"}, {"content-type", "application/json"}, {"content-encoding", "identity"}, {"Content-Encoding", "identity"}},
		"short declared body":      {{":status", "200"}, {"content-type", "application/json"}, {"content-length", "1"}},
		"incomplete declared body": {{":status", "200"}, {"content-type", "application/json"}, {"content-length", "99999"}},
	} {
		t.Run(name, func(t *testing.T) {
			h := newExecutionAdmissionHost(t, executionAdmissionFixture("validation"))
			raw := executionAdmissionJSON(t, executionAdmissionAllowedReceipt(h.start(t, executionAdmissionCaller)))
			h.response(t, headers, raw)
			status := uint32(503)
			if name == "real forbidden" {
				status = 403
			}
			h.assertDenied(t, status)
		})
	}
}

func TestExecutionAdmissionTimeoutAndDuplicateCannotResumeOrQueryAgain(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		name := "timeout"
		if duplicate {
			name = "duplicate"
		}
		t.Run(name, func(t *testing.T) {
			h := newExecutionAdmissionHost(t, executionAdmissionFixture("validation"))
			receipt := executionAdmissionAllowedReceipt(h.start(t, executionAdmissionCaller))
			if duplicate {
				checkExecutionAdmission(h.ctx, h.cfg, h.identity, h.onAllowed)
			} else {
				h.ctx.GetContext(executionAdmissionContext).(*executionAdmissionPending).deadline = time.Now().Add(-time.Second)
			}
			if len(h.host.GetCalloutAttributesFromContext(h.id)) != 1 {
				t.Fatal("second callback created a second query")
			}
			h.response(t, executionAdmissionResponseHeaders(), executionAdmissionJSON(t, receipt))
			h.assertDenied(t, 503)
		})
	}
}

func TestExecutionAdmissionCannotUseMissingNativeCallerOrCrossCallPermit(t *testing.T) {
	t.Run("missing tenant", func(t *testing.T) {
		h := newExecutionAdmissionHost(t, executionAdmissionFixture("validation"))
		identity := executionAdmissionCaller
		identity.TenantID, identity.PrincipalID = "", ""
		h.start(t, identity)
		h.assertDenied(t, 403)
		if len(h.host.GetCalloutAttributesFromContext(h.id)) != 0 {
			t.Fatal("unbound caller queried a permit")
		}
	})
	t.Run("different call", func(t *testing.T) {
		h := newExecutionAdmissionHost(t, executionAdmissionFixture("validation"))
		first := h.start(t, executionAdmissionCaller)
		firstID := h.id
		h.id = h.host.InitializeHttpContext()
		second := h.start(t, executionAdmissionCaller)
		if first.CallID == second.CallID {
			t.Fatal("new call reused an old call ID")
		}
		h.response(t, executionAdmissionResponseHeaders(), executionAdmissionJSON(t, executionAdmissionAllowedReceipt(first)))
		h.assertDenied(t, 503)
		if len(h.host.GetCalloutAttributesFromContext(firstID)) != 1 {
			t.Fatal("second call consumed first call query")
		}
		secondID := h.id
		h.id = firstID
		h.response(t, executionAdmissionResponseHeaders(), executionAdmissionJSON(t, executionAdmissionAllowedReceipt(first)))
		if h.allowed != 1 || h.host.GetSentLocalResponse(firstID) != nil || h.host.GetCurrentHttpStreamAction(firstID) != types.ActionContinue ||
			h.host.GetSentLocalResponse(secondID) == nil || h.host.GetSentLocalResponse(secondID).StatusCode != 503 {
			t.Fatal("one original permit did not remain confined to its own call")
		}
	})
}

func TestExecutionAdmissionStandaloneRetainsExistingMode(t *testing.T) {
	h := newExecutionAdmissionHost(t, map[string]interface{}{})
	h.start(t, executionAdmissionCaller)
	if h.allowed != 1 || len(h.host.GetCalloutAttributesFromContext(h.id)) != 0 || len(h.host.GetRedisCalloutAttributesFromContext(h.id)) != 0 {
		t.Fatal("standalone mode gained a managed permit query")
	}
}

func TestExecutionAdmissionJSONRejectsDuplicateEscapesAndInvalidUnicode(t *testing.T) {
	for _, raw := range [][]byte{
		[]byte(`{"a":1,"\u0061":2}`), []byte(`{"a":{"x":1,"x":2}}`), []byte(`{"x":"\ud800"}`),
		[]byte(`{"x":"\udc00"}`), []byte(`{"x":"\ud800\u0041"}`), {'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'},
	} {
		if err := executionAdmissionCheckJSON(raw); err == nil {
			t.Fatalf("ambiguous JSON accepted: %q", raw)
		}
	}
	for _, raw := range []string{`{"x":"\ud83d\ude00"}`, `{"x":"escaped \\\" quote"}`, `{"array":[1,true,null,{"x":"UTF8 文本"}]}`} {
		if err := executionAdmissionCheckJSON([]byte(raw)); err != nil {
			t.Fatalf("valid JSON rejected: %s: %v", raw, err)
		}
	}
}

func TestExecutionAdmissionRequiresThisResponsesUniqueOriginalCallTime(t *testing.T) {
	for _, malformed := range []string{"absent", "duplicate", "future", "non-UTC"} {
		t.Run(malformed, func(t *testing.T) {
			h := newExecutionAdmissionHost(t, executionAdmissionFixture("validation"))
			receipt := executionAdmissionAllowedReceipt(h.start(t, executionAdmissionCaller))
			headers := executionAdmissionResponseHeaders()
			switch malformed {
			case "absent":
				headers = headers[:2]
			case "duplicate":
				headers = append(headers, [2]string{strings.ToUpper(executionAdmissionOccurredHeader), "2026-10-27T10:00:00Z"})
			case "future":
				headers[2][1] = time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano)
			case "non-UTC":
				headers[2][1] = "2026-10-27T10:00:00+00:00"
			}
			h.response(t, headers, executionAdmissionJSON(t, receipt))
			h.assertDenied(t, 503)
		})
	}
}
