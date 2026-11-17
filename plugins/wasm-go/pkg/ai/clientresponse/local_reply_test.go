package clientresponse

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/higress-group/wasm-go/pkg/iface"
	"github.com/tidwall/gjson"
)

func TestLocalReplyPolicyValidationMatrixU01U02(t *testing.T) {
	type combo struct {
		protocol string
		profile  string
		valid    bool
		expected Profile
	}
	cases := []combo{
		// 8 canonical standardized combinations + default omissions + Completions
		{Chat, "deepseek", true, DeepSeek},
		{Chat, "qwen", true, Qwen},
		{Chat, "openai", true, OpenAI},
		{Chat, "", true, DeepSeek},
		{Responses, "deepseek", true, DeepSeek},
		{Responses, "qwen", true, Qwen},
		{Responses, "openai", true, OpenAI},
		{Responses, "", true, DeepSeek},
		{Messages, "deepseek", true, DeepSeek},
		{Messages, "qwen", true, Qwen},
		{Messages, "", true, DeepSeek},
		{Completions, "", true, DeepSeek},
		{Completions, "deepseek", true, DeepSeek},
		{Completions, "qwen", true, Qwen},
		{Completions, "openai", true, OpenAI},
		// Illegal combinations (U02)
		{Messages, "openai", false, ""},
		{Chat, "anthropic", false, ""},
		{"openai_embeddings", "deepseek", false, ""},
		{"", "deepseek", false, ""},
	}
	for _, tc := range cases {
		policy, err := ParseLocalReplyPolicy(tc.protocol, tc.profile)
		if tc.valid && err != nil {
			t.Fatalf("expected valid (%s, %s), got error: %v", tc.protocol, tc.profile, err)
		}
		if !tc.valid && err == nil {
			t.Fatalf("expected error for (%s, %s), got %+v", tc.protocol, tc.profile, policy)
		}
		if tc.valid {
			if policy.Profile != tc.expected || !policy.Valid() {
				t.Fatalf("unexpected parsed policy for (%s, %s): %+v", tc.protocol, tc.profile, policy)
			}
			out, encErr := EncodeLocalReply(LocalReplyInput{
				Policy:    policy,
				Status:    401,
				Detail:    "key-auth.no_key",
				RequestID: "req-u01",
			})
			if encErr != nil {
				t.Fatalf("encode failed for (%s, %s): %v", tc.protocol, tc.profile, encErr)
			}
			if gjson.GetBytes(out.Body, "error.type").String() != "authentication_error" ||
				gjson.GetBytes(out.Body, "error.code").String() != "invalid_api_key" ||
				gjson.GetBytes(out.Body, "error.message").String() != "A valid API key is required." {
				t.Fatalf("unexpected envelope for (%s, %s): %s", tc.protocol, tc.profile, string(out.Body))
			}
			hasTopType := gjson.GetBytes(out.Body, "type").String() == "error"
			if hasTopType != (tc.protocol == Messages) {
				t.Fatalf("Messages top-level type mismatch for %s: %s", tc.protocol, string(out.Body))
			}
		}
	}

	// Strict JSON boundary checks (U02)
	invalidJSONs := []string{
		`{}`,
		`{"response_profile":"qwen"}`,
		`{"input_protocol":"anthropic_messages","response_profile":"openai"}`,
		`{"input_protocol":"openai_chat","response_profile":""}`,
		`{"input_protocol":"openai_chat","response_profile":null}`,
		`{"input_protocol":"openai_chat","unknown":"field"}`,
		`{"input_protocol":"openai_chat","input_protocol":"anthropic_messages"}`,
		`null`,
		`[]`,
	}
	for _, raw := range invalidJSONs {
		if got, err := ParseLocalReplyPolicyJSON([]byte(raw)); err == nil {
			t.Fatalf("expected boundary rejection for %s, got %+v", raw, got)
		}
	}
	for _, raw := range []string{`{"input_protocol":"openai_chat"}`, `{"input_protocol":"anthropic_messages"}`} {
		got, err := ParseLocalReplyPolicyJSON([]byte(raw))
		if err != nil || got == nil || got.Profile != DeepSeek {
			t.Fatalf("expected omitted response_profile to default to deepseek for %s, got %+v, err=%v", raw, got, err)
		}
	}
	// Duplicate field in parent object rejected
	if _, err := ParseLocalReplyPolicyField(gjson.Parse(`{"local_reply_policy":{"input_protocol":"openai_chat"},"local_reply_policy":{"input_protocol":"anthropic_messages"}}`)); err == nil {
		t.Fatal("expected duplicate local_reply_policy field rejection")
	}
	// Absent field returns (nil, nil) for backward compatibility (U04)
	if got, err := ParseLocalReplyPolicyField(gjson.Parse(`{"allow":["c1"]}`)); err != nil || got != nil {
		t.Fatalf("expected nil policy when omitted, got %+v, err=%v", got, err)
	}
}

func TestLocalReplyDifferentialParityAndSupplierSeparationU03U08(t *testing.T) {
	type expectedCase struct {
		detail    string
		status    uint32
		errorType string
		code      string
		message   string
	}
	// Every Wasm producer reason from test-sp003 sdk/src/release_responses.rs
	cases := []expectedCase{
		{"key-auth.no_key", 401, "authentication_error", "invalid_api_key", "A valid API key is required."},
		{"key-auth.multi_key", 401, "authentication_error", "multiple_api_keys", "Multiple distinct API keys were supplied."},
		{"key-auth.unauthorized", 403, "permission_error", "permission_denied", "The account is not permitted to use this service."},
		{"ai-quota.missing_identity", 403, "permission_error", "permission_denied", "The account is not permitted to use this service."},
		{"execution_admission_check_denied", 403, "permission_error", "permission_denied", "The account is not permitted to use this service."},
		{"execution_admission_caller_identity_missing", 403, "permission_error", "permission_denied", "The account is not permitted to use this service."},
		{"ai-quota.no_balance", 403, "insufficient_quota", "insufficient_balance", "The account has insufficient balance."},
		{"ai-quota.insufficient_balance", 403, "insufficient_quota", "insufficient_balance", "The account has insufficient balance."},
		{"execution_admission_insufficient_balance", 403, "insufficient_quota", "insufficient_balance", "The account has insufficient balance."},
		{"ai-quota.dependency_unavailable", 503, "api_error", "service_unavailable", "The service is temporarily unavailable."},
		{"ai-quota.projection_unready", 503, "api_error", "service_unavailable", "The service is temporarily unavailable."},
		{"ai-quota.route_authorization_missing", 403, "api_error", "service_unavailable", "The service is temporarily unavailable."},
		{"execution_admission_check_unavailable", 503, "api_error", "service_unavailable", "The service is temporarily unavailable."},
		{"ai-token-ratelimit.rejected", 429, "rate_limit_error", "rate_limit_exceeded", "The request limit has been exceeded. Try again later."},
		{"ai-token-ratelimit.rejected", 418, "rate_limit_error", "rate_limit_exceeded", "The request limit has been exceeded. Try again later."},
		{"duplicate_model_member", 400, "invalid_request_error", "invalid_parameter", "The request contains duplicate model fields."},
		{"modelfusion.local.model-mapper.body_too_large", 413, "invalid_request_error", "request_too_large", "The request is too large."},
		{"modelfusion.local.ai-history.body_too_large", 413, "invalid_request_error", "request_too_large", "The request is too large."},
		{"modelfusion.local.ai-memory.body_too_large", 413, "invalid_request_error", "request_too_large", "The request is too large."},
		{"modelfusion.local.model-mapper.invalid_model_body", 400, "invalid_request_error", "invalid_parameter", "The request body or model is invalid."},
		{"modelfusion.local.model-mapper.body_required", 400, "invalid_request_error", "invalid_parameter", "The request body or model is invalid."},
		{"modelfusion.local.model-mapper.json_required", 415, "invalid_request_error", "invalid_parameter", "The request body or model is invalid."},
		{"modelfusion.local.model-mapper.content_encoding_unsupported", 415, "invalid_request_error", "invalid_parameter", "The request body or model is invalid."},
		{"modelfusion.local.model-mapper.body_basis_model_differs", 400, "invalid_request_error", "invalid_parameter", "The request body or model is invalid."},
		{"modelfusion.local.model-mapper.original_body_basis_missing", 400, "invalid_request_error", "invalid_parameter", "The request body or model is invalid."},
		{"modelfusion.local.model-mapper.model_mapping_missing", 422, "invalid_request_error", "invalid_parameter", "The request body or model is invalid."},
		{"modelfusion.local.model-mapper.post_required", 405, "invalid_request_error", "invalid_parameter", "The request body or model is invalid."},
		{"modelfusion.local.model-mapper.header_read_failed", 500, "api_error", "service_unavailable", "The service is temporarily unavailable."},
		{"modelfusion.local.ai-history.json_required", 415, "invalid_request_error", "invalid_parameter", "The request body or model is invalid."},
		{"modelfusion.local.ai-history.invalid_model_body", 400, "invalid_request_error", "invalid_parameter", "The request body or model is invalid."},
		{"modelfusion.local.ai-history.header_read_failed", 500, "api_error", "service_unavailable", "The service is temporarily unavailable."},
		{"modelfusion.local.ai-memory.json_required", 415, "invalid_request_error", "invalid_parameter", "The request body or model is invalid."},
		{"modelfusion.local.ai-memory.invalid_model_body", 400, "invalid_request_error", "invalid_parameter", "The request body or model is invalid."},
		{"modelfusion.local.ai-memory.basis_write_failed", 500, "api_error", "service_unavailable", "The service is temporarily unavailable."},
		{"modelfusion.local.accounting_unavailable", 503, "api_error", "service_unavailable", "The service is temporarily unavailable."},
		{"ai-cache.execution_delivery.original_basis_missing", 400, "invalid_request_error", "invalid_parameter", "The request body or model is invalid."},
		{"ai-cache.execution_delivery.json_text_request_required", 400, "invalid_request_error", "invalid_parameter", "The request body or model is invalid."},
		{"ai-cache.execution_delivery.content_encoding_unsupported", 415, "invalid_request_error", "invalid_parameter", "The request body or model is invalid."},
		{"ai-cache.execution_delivery.original_body_invalid", 400, "invalid_request_error", "invalid_parameter", "The request body or model is invalid."},
		{"ai-cache.execution_delivery.current_body_invalid", 400, "invalid_request_error", "invalid_parameter", "The request body or model is invalid."},
		{"ai-cache.execution_delivery.original_model_differs", 400, "invalid_request_error", "invalid_parameter", "The request body or model is invalid."},
		{"ai-cache.execution_delivery.authenticated_caller_missing", 403, "permission_error", "permission_denied", "The account is not permitted to use this service."},
		{"ai-cache.execution_delivery.current_execution_admission_required", 403, "permission_error", "permission_denied", "The account is not permitted to use this service."},
		{"ai-cache.execution_delivery.execution_admission_expired", 403, "permission_error", "permission_denied", "The account is not permitted to use this service."},
		{"ai-cache.execution_delivery.original_call_missing", 409, "api_error", "service_unavailable", "The service is temporarily unavailable."},
		{"ai-cache.execution_delivery.cache_call_ownership_differs", 409, "api_error", "service_unavailable", "The service is temporarily unavailable."},
	}

	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC).Unix()
	for _, protocol := range []string{Chat, Completions, Responses, Messages} {
		policy, err := ParseLocalReplyPolicy(protocol, "qwen")
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range cases {
			out, err := EncodeLocalReply(LocalReplyInput{
				Policy:    policy,
				Status:    tc.status,
				Detail:    tc.detail,
				RequestID: "req-trace-123",
				CreatedAt: now,
				Headers: [][2]string{
					{"WWW-Authenticate", "Key realm=MSE Gateway"},
					{"X-TokenRateLimit-Reset", "42"},
					{"Retry-After", "30"},
					{"Retry-After", "9999999"},
					{"Content-Type", "text/plain"},
					{"X-Internal-Secret", "must-not-leak"},
				},
			})
			if err != nil {
				t.Fatalf("EncodeLocalReply(%s, %s): %v", protocol, tc.detail, err)
			}
			// Producer status and detail preserved (never turned into 503 by ClassifyFailure)
			if out.Status != tc.status || out.Detail != tc.detail {
				t.Fatalf("status/detail mutated: got (%d, %s), want (%d, %s)", out.Status, out.Detail, tc.status, tc.detail)
			}
			var parsed map[string]json.RawMessage
			if err := json.Unmarshal(out.Body, &parsed); err != nil {
				t.Fatalf("invalid JSON body: %v", err)
			}
			if gjson.GetBytes(out.Body, "error.type").String() != tc.errorType ||
				gjson.GetBytes(out.Body, "error.code").String() != tc.code ||
				gjson.GetBytes(out.Body, "error.message").String() != tc.message ||
				gjson.GetBytes(out.Body, "error.param").Exists() {
				t.Fatalf("body mismatch for (%s, %s): %s", protocol, tc.detail, string(out.Body))
			}
			if protocol == Messages {
				if len(parsed) != 2 || gjson.GetBytes(out.Body, "type").String() != "error" {
					t.Fatalf("Messages envelope mismatch: %s", string(out.Body))
				}
			} else if len(parsed) != 1 {
				t.Fatalf("non-Messages envelope has unexpected top-level keys: %s", string(out.Body))
			}
			headerMap := map[string][]string{}
			for _, h := range out.Headers {
				headerMap[strings.ToLower(h[0])] = append(headerMap[strings.ToLower(h[0])], h[1])
			}
			if len(headerMap["content-type"]) != 1 || headerMap["content-type"][0] != "application/json" {
				t.Fatalf("unexpected content-type headers: %+v", out.Headers)
			}
			if len(headerMap["x-request-id"]) != 1 || headerMap["x-request-id"][0] != "req-trace-123" {
				t.Fatalf("missing or duplicate x-request-id: %+v", out.Headers)
			}
			if len(headerMap["www-authenticate"]) != 1 || headerMap["www-authenticate"][0] != "Key realm=MSE Gateway" {
				t.Fatalf("WWW-Authenticate lost: %+v", out.Headers)
			}
			if len(headerMap["x-tokenratelimit-reset"]) != 1 || headerMap["x-tokenratelimit-reset"][0] != "42" {
				t.Fatalf("rate-limit reset lost: %+v", out.Headers)
			}
			if len(headerMap["retry-after"]) != 1 || headerMap["retry-after"][0] != "30" {
				t.Fatalf("valid Retry-After lost or invalid kept: %+v", out.Headers)
			}
			if len(headerMap["x-internal-secret"]) != 0 {
				t.Fatalf("untrusted header leaked: %+v", out.Headers)
			}
		}
	}

	// Prove upstream ClassifyFailure still maps supplier 401 to 503 service_unavailable (A-E01 / U08),
	// while client key-auth 401 stays 401 authentication_error.
	supplier401 := ClassifyFailure(FailureInput{Status: 401, Format: Chat, Client: Chat, Body: []byte(`{"error":{"code":"invalid_api_key","message":"supplier secret"}}`)})
	if supplier401.Kind != FailureAuthentication || supplier401.Status(Chat) != 503 {
		t.Fatalf("supplier 401 classification regressed: %+v status=%d", supplier401, supplier401.Status(Chat))
	}
	// Cache hit & proxy managed_execution details are NOT local error reasons (U08).
	for _, nonLocal := range []string{"ai-proxy.managed_execution", "ai-cache.hit", "route_not_found"} {
		if kind, ok := LocalReplyKindForDetail(nonLocal); ok {
			t.Fatalf("non-local detail %q must not match LocalReplyKindForDetail (got %s)", nonLocal, kind)
		}
	}
}

type phaseStubHttpContext struct {
	iface.HttpContext
	phase iface.HTTPExecutionPhase
}

func (p phaseStubHttpContext) GetExecutionPhase() iface.HTTPExecutionPhase { return p.phase }
func (p phaseStubHttpContext) GetStringContext(string, string) string      { return "req-u09" }
func (p phaseStubHttpContext) GetContext(string) interface{}               { return nil }

func TestLocalReplyRefusesCommittedResponsePhasesU09(t *testing.T) {
	policy, err := ParseLocalReplyPolicy(Chat, "deepseek")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := EncodeLocalReply(LocalReplyInput{
		Policy:            policy,
		Status:            500,
		Detail:            "modelfusion.local.model-mapper.header_read_failed",
		ResponseCommitted: true,
	}); !errors.Is(err, ErrLocalReplyAfterResponseCommitted) {
		t.Fatalf("expected ErrLocalReplyAfterResponseCommitted, got %v", err)
	}
	for _, phase := range []iface.HTTPExecutionPhase{iface.EncodeHeader, iface.EncodeData, iface.Done} {
		ctx := phaseStubHttpContext{phase: phase}
		if !ResponseHeadersCommitted(ctx) {
			t.Fatalf("phase %v must report committed", phase)
		}
		handled, sendErr := SendLocalReply(ctx, &policy, 500, "modelfusion.local.model-mapper.header_read_failed", nil)
		if !handled || !errors.Is(sendErr, ErrLocalReplyAfterResponseCommitted) {
			t.Fatalf("SendLocalReply in phase %v must refuse without host send: handled=%v err=%v", phase, handled, sendErr)
		}
	}
	for _, phase := range []iface.HTTPExecutionPhase{iface.DecodeHeader, iface.DecodeData} {
		if ResponseHeadersCommitted(phaseStubHttpContext{phase: phase}) {
			t.Fatalf("request phase %v must not report committed", phase)
		}
	}
}

func TestLocalReplySelectorPreRouteAndCustomMessagesPathU10(t *testing.T) {
	// 1. Route-level local_reply_policy wins directly and forbids mixing with local_reply_default:true or local_reply_entrances
	routeSel, err := ParseLocalReplySelector(gjson.Parse(`{"local_reply_policy":{"input_protocol":"anthropic_messages","response_profile":"qwen"}}`))
	if err != nil {
		t.Fatal(err)
	}
	resolved := routeSel.Resolve("api.example.com:8443", "/v1/chat/completions?foo=1")
	if resolved == nil || resolved.Protocol != Messages || resolved.Profile != Qwen {
		t.Fatalf("route-level policy must win unconditionally, got %+v", resolved)
	}
	for _, conflict := range []string{
		`{"local_reply_policy":{"input_protocol":"openai_chat"},"local_reply_default":true}`,
		`{"local_reply_policy":{"input_protocol":"openai_chat"},"local_reply_entrances":[]}`,
	} {
		if _, err := ParseLocalReplySelector(gjson.Parse(conflict)); err == nil {
			t.Fatalf("expected conflict error for %s", conflict)
		}
	}

	// 2. Domain/pre-route selector with custom Messages path (EQUAL, PRE, REGULAR) + local_reply_default fallback
	domainSel, err := ParseLocalReplySelector(gjson.Parse(`{
		"local_reply_default": true,
		"local_reply_entrances": [
			{
				"domains": ["ai.example.com"],
				"path": {"match_type": "EQUAL", "match_value": "/custom/claude-entry"},
				"local_reply_policy": {"input_protocol": "anthropic_messages", "response_profile": "qwen"}
			},
			{
				"domains": ["ai.example.com"],
				"path": {"match_type": "PRE", "match_value": "/Api/V2/Responses", "case_sensitive": false},
				"local_reply_policy": {"input_protocol": "openai_responses", "response_profile": "openai"}
			},
			{
				"path": {"match_type": "REGULAR", "match_value": "^/tenant-[0-9]+/messages-custom$"},
				"local_reply_policy": {"input_protocol": "anthropic_messages", "response_profile": "deepseek"}
			}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	// Custom Messages path without /messages suffix resolves to anthropic_messages + qwen
	if got := domainSel.Resolve("AI.EXAMPLE.COM:443", "/custom/claude-entry?beta=true"); got == nil || got.Protocol != Messages || got.Profile != Qwen {
		t.Fatalf("custom EQUAL Messages entrance failed: %+v", got)
	}
	// Case-sensitive EQUAL must not match wrong case, falling through to local_reply_default (openai_chat)
	if got := domainSel.Resolve("ai.example.com", "/Custom/Claude-Entry"); got == nil || got.Protocol != Chat || got.Profile != DeepSeek {
		t.Fatalf("case-sensitive EQUAL should fall through to default Chat, got %+v", got)
	}
	// Case-insensitive PRE matches
	if got := domainSel.Resolve("ai.example.com:8080", "/api/v2/responses/sub?x=1"); got == nil || got.Protocol != Responses || got.Profile != OpenAI {
		t.Fatalf("case-insensitive PRE entrance failed: %+v", got)
	}
	// Regex custom Messages path matches
	if got := domainSel.Resolve("other.example.com", "/tenant-42/messages-custom"); got == nil || got.Protocol != Messages || got.Profile != DeepSeek {
		t.Fatalf("REGULAR custom Messages entrance failed: %+v", got)
	}
	// Default pre-route suffix rule: /messages -> anthropic_messages/deepseek, else -> openai_chat/deepseek
	if got := domainSel.Resolve("other.example.com", "/v1/messages?stream=true"); got == nil || got.Protocol != Messages || got.Profile != DeepSeek {
		t.Fatalf("default /messages suffix fallback failed: %+v", got)
	}
	if got := domainSel.Resolve("other.example.com", "/v1/chat/completions"); got == nil || got.Protocol != Chat || got.Profile != DeepSeek {
		t.Fatalf("default non-messages fallback failed: %+v", got)
	}

	// 3. Entrances without local_reply_default leave unmatched paths (portal, ACME, MCP) nil
	scopedOnly, err := ParseLocalReplySelector(gjson.Parse(`{
		"local_reply_entrances": [
			{
				"domains": ["ai.example.com"],
				"path": {"match_type": "EQUAL", "match_value": "/v1/messages"},
				"local_reply_policy": {"input_protocol": "anthropic_messages", "response_profile": "qwen"}
			}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := scopedOnly.Resolve("portal.example.com", "/v1/messages"); got != nil {
		t.Fatalf("non-AI portal domain must stay nil, got %+v", got)
	}
	if got := scopedOnly.Resolve("ai.example.com", "/mcp/sse"); got != nil {
		t.Fatalf("unmatched MCP path without local_reply_default must stay nil, got %+v", got)
	}
}

