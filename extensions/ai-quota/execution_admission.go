package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"mime"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const (
	executionAdmissionSchema         = "managed-execution-admission.v1"
	executionAdmissionPath           = "/internal/execution-admission"
	executionAdmissionContext        = "ai-quota-execution-admission.v1"
	executionAdmissionCallHeader     = "x-modelfusion-call-id"
	executionAdmissionAttemptHeader  = "x-modelfusion-attempt-id"
	executionAdmissionReleaseHeader  = "x-modelfusion-admission-release-id"
	executionAdmissionDeadlineHeader = "x-modelfusion-admission-valid-until"
	executionAdmissionMaxJSONBytes   = 64 * 1024
)

type executionAdmissionConsole struct {
	ServiceName string `json:"service_name"`
	ServicePort uint32 `json:"service_port"`
	Host        string `json:"host"`
	AuthToken   string `json:"auth_token"`
	TimeoutMS   uint32 `json:"timeout_ms"`
}

type executionAdmissionConfig struct {
	SchemaVersion  string                    `json:"schema_version"`
	ReleaseID      string                    `json:"release_id"`
	RouteID        string                    `json:"route_id"`
	Purpose        string                    `json:"purpose"`
	CombinationKey string                    `json:"combination_key"`
	ProfileID      string                    `json:"profile_id"`
	AccessID       *string                   `json:"access_id"`
	Revision       *string                   `json:"revision"`
	Console        executionAdmissionConsole `json:"console_internal"`
	cluster        string
}

// These are the Console CheckRequest and CheckReceipt wires. Nullable fields
// deliberately have no omitempty: production is not a validation access grant.
type executionAdmissionRequest struct {
	AccessID             *string `json:"access_id"`
	Revision             *string `json:"revision"`
	CallID               string  `json:"call_id"`
	ReleaseID            string  `json:"release_id"`
	CombinationKey       string  `json:"combination_key"`
	ProfileID            string  `json:"profile_id"`
	Purpose              string  `json:"purpose"`
	TenantID             string  `json:"tenant_id"`
	RouteID              string  `json:"route_id"`
	AccountID            string  `json:"account_id"`
	PrincipalID          string  `json:"principal_id"`
	APIKeyID             string  `json:"api_key_id"`
	CredentialGeneration string  `json:"credential_generation"`
}

type executionAdmissionReceipt struct {
	CallID               string  `json:"call_id"`
	AccessID             *string `json:"access_id"`
	Allowed              *bool   `json:"allowed"`
	CurrentRevision      *string `json:"current_revision"`
	ValidUntil           *string `json:"valid_until"`
	Reason               string  `json:"reason"`
	Purpose              string  `json:"purpose"`
	ReleaseID            string  `json:"release_id"`
	CombinationKey       string  `json:"combination_key"`
	ProfileID            string  `json:"profile_id"`
	TenantID             string  `json:"tenant_id"`
	RouteID              string  `json:"route_id"`
	AccountID            string  `json:"account_id"`
	PrincipalID          string  `json:"principal_id"`
	APIKeyID             string  `json:"api_key_id"`
	CredentialGeneration string  `json:"credential_generation"`
}

type executionAdmissionPending struct {
	request  executionAdmissionRequest
	deadline time.Time
	finished bool
}

// configureExecutionAdmission never inherits an admission authority from a
// different scope. The SDK supplies the whole object on every managed rule.
func configureExecutionAdmission(value gjson.Result, cfg *QuotaConfig) error {
	cfg.ExecutionAdmission = nil
	entry := value.Get("execution_admission")
	if !entry.Exists() {
		return nil
	}
	if cfg.PathScope != PathScopeBoundTarget || len(cfg.EnableMethods) != 0 || len(cfg.EnablePathSuffixes) != 0 {
		return errors.New("execution_admission requires bound_target without method or path filters")
	}
	// Check the containing object as well: gjson alone selects one duplicate key.
	if err := executionAdmissionCheckJSON([]byte(value.Raw)); err != nil {
		return fmt.Errorf("execution_admission configuration: %w", err)
	}
	var parsed executionAdmissionConfig
	if err := executionAdmissionDecodeObject([]byte(entry.Raw), []string{
		"schema_version", "release_id", "route_id", "purpose", "combination_key", "profile_id", "console_internal",
	}, []string{"access_id", "revision"}, &parsed); err != nil {
		return fmt.Errorf("execution_admission: %w", err)
	}
	if parsed.SchemaVersion != executionAdmissionSchema || !executionAdmissionText(parsed.ReleaseID, 256) ||
		!balanceidentity.ValidID(parsed.RouteID) || !executionAdmissionText(parsed.ProfileID, 256) || !executionAdmissionDigest(parsed.CombinationKey) {
		return errors.New("execution_admission requires the frozen schema and release scope")
	}
	switch parsed.Purpose {
	case "validation":
		if parsed.AccessID == nil || !executionAdmissionText(*parsed.AccessID, 256) || parsed.Revision == nil || !executionAdmissionRevision(*parsed.Revision) {
			return errors.New("validation execution_admission requires access_id and positive exact revision")
		}
	case "production":
		if parsed.AccessID != nil || parsed.Revision != nil {
			return errors.New("production execution_admission cannot consume a validation access")
		}
	default:
		return errors.New("execution_admission purpose is unsupported")
	}
	var console executionAdmissionConsole
	if err := executionAdmissionDecodeObject([]byte(entry.Get("console_internal").Raw), []string{
		"service_name", "service_port", "host", "auth_token", "timeout_ms",
	}, nil, &console); err != nil {
		return fmt.Errorf("execution_admission console_internal: %w", err)
	}
	if !executionAdmissionServiceName(console.ServiceName) || console.ServicePort == 0 || console.ServicePort > 65535 ||
		!executionAdmissionAuthority(console.Host) || !executionAdmissionToken(console.AuthToken) || console.TimeoutMS == 0 || console.TimeoutMS > 60000 {
		return errors.New("execution_admission requires a complete controlled Console service")
	}
	parsed.Console = console
	cluster := wrapper.FQDNCluster{FQDN: console.ServiceName, Port: int64(console.ServicePort), Host: console.Host}
	parsed.cluster = cluster.ClusterName()
	cfg.ExecutionAdmission = &parsed
	return nil
}

// checkExecutionAdmission is called only after authentication and the existing
// balance projection allow. It pauses until this single call's controlled
// Console query succeeds; it never resumes or sends a model request itself.
func checkExecutionAdmission(ctx wrapper.HttpContext, cfg QuotaConfig, identity balanceidentity.Snapshot, allowed func()) {
	if cfg.ExecutionAdmission == nil {
		if allowed != nil {
			allowed()
		}
		return
	}
	parsed, ok := cfg.ExecutionAdmission.(*executionAdmissionConfig)
	if !ok || parsed == nil || allowed == nil {
		executionAdmissionReject(ctx, 503, "configuration_unavailable")
		return
	}
	if existing := ctx.GetContext(executionAdmissionContext); existing != nil {
		if pending, ok := existing.(*executionAdmissionPending); ok && !pending.finished {
			pending.finished = true
			executionAdmissionReject(ctx, 503, "duplicate_check")
		}
		return
	}
	pending := &executionAdmissionPending{}
	ctx.SetContext(executionAdmissionContext, pending)
	reject := func(status uint32, reason string) {
		if !pending.finished {
			pending.finished = true
			executionAdmissionReject(ctx, status, reason)
		}
	}
	// Remove every occurrence, including case variants, before minting any call.
	if err := executionAdmissionInstallHeaders("", "", ""); err != nil {
		reject(503, "request_headers_unavailable")
		return
	}
	if err := identity.Validate(); err != nil || identity.TenantID == "" || identity.PrincipalID == "" {
		reject(403, "caller_identity_missing")
		return
	}
	callID, err := uuid.NewRandom()
	if err != nil {
		reject(503, "call_id_unavailable")
		return
	}
	pending.request = executionAdmissionRequest{
		AccessID: parsed.AccessID, Revision: parsed.Revision, CallID: callID.String(), ReleaseID: parsed.ReleaseID,
		CombinationKey: parsed.CombinationKey, ProfileID: parsed.ProfileID, Purpose: parsed.Purpose,
		TenantID: identity.TenantID, RouteID: parsed.RouteID, AccountID: identity.AccountID,
		PrincipalID: identity.PrincipalID, APIKeyID: identity.APIKeyID, CredentialGeneration: identity.CredentialGeneration,
	}
	if err := executionAdmissionInstallHeaders(pending.request.CallID, "", ""); err != nil {
		reject(503, "request_headers_unavailable")
		return
	}
	body, err := json.Marshal(pending.request)
	if err != nil {
		reject(503, "request_encoding_failed")
		return
	}
	pending.deadline = time.Now().Add(time.Duration(parsed.Console.TimeoutMS) * time.Millisecond)
	headers := [][2]string{{":method", "POST"}, {":path", executionAdmissionPath}, {":authority", parsed.Console.Host},
		{"content-type", "application/json"}, {"accept", "application/json"}, {"authorization", "Bearer " + parsed.Console.AuthToken}}
	_, err = proxywasm.DispatchHttpCall(parsed.cluster, headers, body, nil, parsed.Console.TimeoutMS, func(numHeaders, bodySize, numTrailers int) {
		if pending.finished {
			return
		}
		if !time.Now().Before(pending.deadline) {
			reject(503, "check_expired")
			return
		}
		responseHeaders, headerErr := proxywasm.GetHttpCallResponseHeaders()
		if headerErr != nil || len(responseHeaders) != numHeaders {
			reject(503, "response_headers_missing")
			return
		}
		status, statusOK := executionAdmissionSingleHeader(responseHeaders, ":status")
		if !statusOK || status != "200" {
			if statusOK && status == "403" {
				reject(403, "check_denied")
			} else {
				reject(503, "check_unavailable")
			}
			return
		}
		contentType, typeOK := executionAdmissionSingleHeader(responseHeaders, "content-type")
		media, parameters, mediaErr := mime.ParseMediaType(contentType)
		if !typeOK || mediaErr != nil || media != "application/json" || (len(parameters) != 0 &&
			(len(parameters) != 1 || !strings.EqualFold(parameters["charset"], "utf-8"))) || bodySize < 1 || bodySize > executionAdmissionMaxJSONBytes {
			reject(503, "response_incomplete")
			return
		}
		if encoding, found := executionAdmissionSingleHeader(responseHeaders, "content-encoding"); executionAdmissionHeaderCount(responseHeaders, "content-encoding") > 1 || found && encoding != "identity" {
			reject(503, "response_encoding_unsupported")
			return
		}
		if count, countOK := executionAdmissionOptionalLength(responseHeaders, bodySize); !countOK || count != bodySize {
			reject(503, "response_incomplete")
			return
		}
		responseBody, bodyErr := proxywasm.GetHttpCallResponseBody(0, bodySize)
		if bodyErr != nil || len(responseBody) != bodySize {
			reject(503, "response_incomplete")
			return
		}
		var receipt executionAdmissionReceipt
		if err := executionAdmissionDecodeObject(responseBody, []string{
			"call_id", "access_id", "allowed", "current_revision", "valid_until", "reason", "purpose", "release_id",
			"combination_key", "profile_id", "tenant_id", "route_id", "account_id", "principal_id", "api_key_id", "credential_generation",
		}, nil, &receipt); err != nil || receipt.Allowed == nil || !executionAdmissionEcho(pending.request, receipt) {
			reject(503, "receipt_invalid")
			return
		}
		if !*receipt.Allowed {
			reject(403, "check_denied")
			return
		}
		if !time.Now().Before(pending.deadline) {
			reject(503, "check_expired")
			return
		}
		if receipt.Reason != "allowed" || !executionAdmissionSameString(parsed.Revision, receipt.CurrentRevision) || receipt.ValidUntil == nil ||
			!executionAdmissionFutureUTC(*receipt.ValidUntil, time.Now()) {
			reject(503, "permit_invalid")
			return
		}
		// Install only the original returned deadline, never an extended local TTL.
		if err := executionAdmissionInstallHeaders(pending.request.CallID, parsed.ReleaseID, *receipt.ValidUntil); err != nil {
			reject(503, "request_headers_unavailable")
			return
		}
		pending.finished = true
		ctx.SetUserAttribute("execution_admission_reason", "allowed")
		allowed()
	})
	if err != nil {
		reject(503, "check_dispatch_failed")
	}
}

func executionAdmissionEcho(request executionAdmissionRequest, receipt executionAdmissionReceipt) bool {
	return request.CallID == receipt.CallID && executionAdmissionSameString(request.AccessID, receipt.AccessID) &&
		request.Purpose == receipt.Purpose && request.ReleaseID == receipt.ReleaseID && request.CombinationKey == receipt.CombinationKey &&
		request.ProfileID == receipt.ProfileID && request.TenantID == receipt.TenantID && request.RouteID == receipt.RouteID &&
		request.AccountID == receipt.AccountID && request.PrincipalID == receipt.PrincipalID && request.APIKeyID == receipt.APIKeyID &&
		request.CredentialGeneration == receipt.CredentialGeneration
}

func executionAdmissionSameString(left, right *string) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func executionAdmissionInstallHeaders(callID, releaseID, deadline string) error {
	headers, err := proxywasm.GetHttpRequestHeaders()
	if err != nil {
		return err
	}
	clean := make([][2]string, 0, len(headers)+3)
	for _, pair := range headers {
		switch strings.ToLower(pair[0]) {
		case executionAdmissionCallHeader, executionAdmissionAttemptHeader, executionAdmissionReleaseHeader, executionAdmissionDeadlineHeader:
			continue
		}
		clean = append(clean, pair)
	}
	if callID != "" {
		clean = append(clean, [2]string{executionAdmissionCallHeader, callID})
	}
	if releaseID != "" && deadline != "" {
		clean = append(clean, [2]string{executionAdmissionReleaseHeader, releaseID}, [2]string{executionAdmissionDeadlineHeader, deadline})
	}
	return proxywasm.ReplaceHttpRequestHeaders(clean)
}

func executionAdmissionReject(ctx wrapper.HttpContext, status uint32, reason string) {
	ctx.SetUserAttribute("execution_admission_reason", reason)
	_ = proxywasm.SendHttpResponseWithDetail(status, "execution_admission_"+reason,
		[][2]string{{"content-type", "application/json"}}, []byte(`{"error":"execution admission unavailable"}`), -1)
}

func executionAdmissionSingleHeader(headers [][2]string, name string) (string, bool) {
	value, count := "", 0
	for _, pair := range headers {
		if strings.EqualFold(pair[0], name) {
			value, count = pair[1], count+1
		}
	}
	return value, count == 1
}

func executionAdmissionHeaderCount(headers [][2]string, name string) int {
	count := 0
	for _, pair := range headers {
		if strings.EqualFold(pair[0], name) {
			count++
		}
	}
	return count
}

func executionAdmissionOptionalLength(headers [][2]string, actual int) (int, bool) {
	count, length := 0, actual
	for _, pair := range headers {
		if strings.EqualFold(pair[0], "content-length") {
			count++
			parsed, err := strconv.Atoi(pair[1])
			if err != nil || parsed < 0 || strconv.Itoa(parsed) != pair[1] {
				return 0, false
			}
			length = parsed
		}
	}
	return length, count <= 1
}

func executionAdmissionFutureUTC(value string, now time.Time) bool {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || strings.HasSuffix(value, "-00:00") {
		return false
	}
	_, offset := parsed.Zone()
	return offset == 0 && parsed.After(now)
}

func executionAdmissionText(value string, limit int) bool {
	if value == "" || len(value) > limit || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, char := range value {
		if char < 0x20 || char == 0x7f {
			return false
		}
	}
	return true
}

func executionAdmissionToken(value string) bool {
	if len(value) < 1 || len(value) > 8192 {
		return false
	}
	for _, char := range value {
		if char < 0x21 || char > 0x7e {
			return false
		}
	}
	return true
}

func executionAdmissionDigest(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, char := range value[7:] {
		if !(char >= '0' && char <= '9' || char >= 'a' && char <= 'f') {
			return false
		}
	}
	return true
}

func executionAdmissionRevision(value string) bool {
	if len(value) < 1 || len(value) > 39 || value[0] == '0' {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	parsed, ok := new(big.Int).SetString(value, 10)
	return ok && parsed.Sign() > 0 && parsed.BitLen() <= 128
}

func executionAdmissionServiceName(value string) bool {
	if len(value) < 1 || len(value) > 253 {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) < 1 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-') {
				return false
			}
		}
	}
	return true
}

func executionAdmissionAuthority(value string) bool {
	if !executionAdmissionText(value, 512) || strings.ContainsAny(value, " /\\?#@%") {
		return false
	}
	parsed, err := url.Parse("http://" + value)
	if err != nil || parsed.Host != value || parsed.Hostname() == "" || parsed.User != nil || parsed.Path != "" {
		return false
	}
	if port := parsed.Port(); port != "" {
		number, err := strconv.Atoi(port)
		return err == nil && number > 0 && number <= 65535 && strconv.Itoa(number) == port
	}
	return !strings.HasSuffix(value, ":")
}

func executionAdmissionDecodeObject(raw []byte, required, optional []string, destination interface{}) error {
	if len(raw) > executionAdmissionMaxJSONBytes {
		return errors.New("JSON exceeds admission bound")
	}
	if err := executionAdmissionCheckJSON(raw); err != nil {
		return err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return errors.New("JSON object required")
	}
	keys := make(map[string]bool, len(required)+len(optional))
	for _, key := range required {
		keys[key] = true
		if _, exists := object[key]; !exists {
			return errors.New("required admission field missing")
		}
	}
	for _, key := range optional {
		keys[key] = true
	}
	for key := range object {
		if !keys[key] {
			return errors.New("unknown admission field")
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return errors.New("admission field type invalid")
	}
	return nil
}

// Check the whole JSON token stream before unmarshalling, because encoding/json
// otherwise accepts duplicate keys and silently replaces lone UTF-16 surrogates.
func executionAdmissionCheckJSON(raw []byte) error {
	if !utf8.Valid(raw) || !executionAdmissionUnicode(raw) {
		return errors.New("admission JSON encoding invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var scan func(int) error
	scan = func(depth int) error {
		if depth > 64 {
			return errors.New("admission JSON nesting exceeds bound")
		}
		token, err := decoder.Token()
		if err != nil {
			return errors.New("admission JSON incomplete")
		}
		delimiter, compound := token.(json.Delim)
		if !compound {
			return nil
		}
		switch delimiter {
		case '{':
			seen := make(map[string]bool)
			for decoder.More() {
				key, err := decoder.Token()
				name, ok := key.(string)
				if err != nil || !ok || seen[name] {
					return errors.New("admission JSON duplicate or invalid field")
				}
				seen[name] = true
				if err := scan(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for decoder.More() {
				if err := scan(depth + 1); err != nil {
					return err
				}
			}
		default:
			return errors.New("admission JSON token invalid")
		}
		closing, err := decoder.Token()
		if err != nil || delimiter == '{' && closing != json.Delim('}') || delimiter == '[' && closing != json.Delim(']') {
			return errors.New("admission JSON incomplete")
		}
		return nil
	}
	if err := scan(0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("admission JSON has trailing data")
	}
	return nil
}

func executionAdmissionUnicode(raw []byte) bool {
	inside := false
	for index := 0; index < len(raw); index++ {
		if raw[index] == '"' {
			inside = !inside
			continue
		}
		if !inside || raw[index] != '\\' {
			continue
		}
		index++
		if index >= len(raw) {
			return false
		}
		if raw[index] != 'u' {
			continue
		}
		if index+4 >= len(raw) {
			return false
		}
		value, err := strconv.ParseUint(string(raw[index+1:index+5]), 16, 16)
		if err != nil {
			return false
		}
		index += 4
		if value >= 0xdc00 && value <= 0xdfff {
			return false
		}
		if value >= 0xd800 && value <= 0xdbff {
			if index+6 >= len(raw) || raw[index+1] != '\\' || raw[index+2] != 'u' {
				return false
			}
			low, err := strconv.ParseUint(string(raw[index+3:index+7]), 16, 16)
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return false
			}
			index += 6
		}
	}
	return !inside
}
