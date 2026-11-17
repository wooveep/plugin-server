package clientresponse

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/higress-group/wasm-go/pkg/iface"
	"github.com/tidwall/gjson"
)

const (
	Completions                = "openai_completions"
	LocalReplyField            = "local_reply_policy"
	LocalReplyDefaultField     = "local_reply_default"
	LocalReplyEntrancesField   = "local_reply_entrances"
	originalReqIDKey           = "modelfusion.original-request-id"
	originalReqAtKey           = "modelfusion.original-request-start"
	maxRequestIDBytes          = 256
)

var (
	ErrLocalReplyAfterResponseCommitted = errors.New("local reply cannot encode after response headers are committed")
	ErrInvalidLocalReplyPolicy          = errors.New("invalid local reply policy")
)

// LocalReplyPolicy holds the frozen client protocol and presentation profile
// validated once at the plugin configuration boundary.
type LocalReplyPolicy struct {
	Protocol string
	Profile  Profile
}

func (p LocalReplyPolicy) Valid() bool {
	switch p.Protocol {
	case Chat, Completions, Responses:
		return p.Profile == DeepSeek || p.Profile == Qwen || p.Profile == OpenAI
	case Messages:
		return p.Profile == DeepSeek || p.Profile == Qwen
	default:
		return false
	}
}

// ParseLocalReplyPolicy validates an input_protocol and optional response_profile
// against the frozen downstream presentation contract.
func ParseLocalReplyPolicy(protocol, profile string) (LocalReplyPolicy, error) {
	if protocol != Chat && protocol != Completions && protocol != Responses && protocol != Messages {
		return LocalReplyPolicy{}, fmt.Errorf("local_reply_policy unsupported input_protocol %q", protocol)
	}
	resolved := Profile(profile)
	if resolved == "" {
		resolved = DeepSeek
	}
	policy := LocalReplyPolicy{Protocol: protocol, Profile: resolved}
	if !policy.Valid() {
		return LocalReplyPolicy{}, fmt.Errorf("local_reply_policy unsupported response_profile %q for %s", profile, protocol)
	}
	return policy, nil
}

// ParseLocalReplyPolicyJSON decodes and validates a standalone local_reply_policy JSON object.
func ParseLocalReplyPolicyJSON(raw []byte) (*LocalReplyPolicy, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	canonical, err := mfjson.Canonicalize(raw)
	if err != nil {
		return nil, fmt.Errorf("local_reply_policy invalid JSON: %w", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(canonical, &fields); err != nil || fields == nil {
		return nil, errors.New("local_reply_policy must be a JSON object")
	}
	for key := range fields {
		if key != "input_protocol" && key != "response_profile" {
			return nil, fmt.Errorf("local_reply_policy unknown field %q", key)
		}
	}
	rawProtocol, hasProtocol := fields["input_protocol"]
	if !hasProtocol || bytes.Equal(rawProtocol, []byte("null")) {
		return nil, errors.New("local_reply_policy.input_protocol is required")
	}
	var protocol string
	if err := json.Unmarshal(rawProtocol, &protocol); err != nil {
		return nil, fmt.Errorf("local_reply_policy.input_protocol must be a string: %w", err)
	}
	var profile string
	if rawProfile, hasProfile := fields["response_profile"]; hasProfile {
		if bytes.Equal(rawProfile, []byte("null")) {
			return nil, errors.New("local_reply_policy.response_profile must be a string when present")
		}
		if err := json.Unmarshal(rawProfile, &profile); err != nil || profile == "" {
			return nil, errors.New("local_reply_policy.response_profile must be a non-empty string when present")
		}
	}
	parsed, err := ParseLocalReplyPolicy(protocol, profile)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

// ParseLocalReplyPolicyField extracts an optional local_reply_policy member from
// a plugin configuration object, rejecting duplicate keys and malformed values.
func ParseLocalReplyPolicyField(parent gjson.Result) (*LocalReplyPolicy, error) {
	if !parent.Exists() {
		return nil, nil
	}
	if !parent.IsObject() {
		return nil, nil
	}
	var count int
	var selected gjson.Result
	parent.ForEach(func(key, value gjson.Result) bool {
		if key.String() == LocalReplyField {
			count++
			selected = value
		}
		return true
	})
	if count == 0 {
		return nil, nil
	}
	if count > 1 {
		return nil, errors.New("duplicate local_reply_policy field")
	}
	if !selected.IsObject() {
		return nil, errors.New("local_reply_policy must be an object")
	}
	return ParseLocalReplyPolicyJSON([]byte(selected.Raw))
}

// LocalReplyEntrance describes a static pre-route entrance rule for domain/global plugins.
type LocalReplyEntrance struct {
	Domains       []string
	MatchType     string
	MatchValue    string
	CaseSensitive bool
	Regex         *regexp.Regexp
	Policy        LocalReplyPolicy
}

// LocalReplySelector resolves either an exact route-level policy or pre-route entrance/default rules.
type LocalReplySelector struct {
	Policy    *LocalReplyPolicy
	Entrances []LocalReplyEntrance
	Default   bool
}

func (s LocalReplySelector) IsConfigured() bool {
	return s.Policy != nil || len(s.Entrances) > 0 || s.Default
}

// Resolve selects the frozen LocalReplyPolicy for the given authority and request path,
// or returns nil when no AI local-reply rule applies (preserving legacy rejections).
func (s LocalReplySelector) Resolve(authority, rawPath string) *LocalReplyPolicy {
	if s.Policy != nil {
		return s.Policy
	}
	pathOnly, _, _ := strings.Cut(rawPath, "?")
	pathOnly, _, _ = strings.Cut(pathOnly, "#")
	if len(s.Entrances) > 0 {
		host := normalizeAuthorityHost(authority)
		for i := range s.Entrances {
			entrance := &s.Entrances[i]
			if len(entrance.Domains) > 0 && !matchesEntranceDomain(host, entrance.Domains) {
				continue
			}
			if matchesEntrancePath(pathOnly, entrance) {
				policy := entrance.Policy
				return &policy
			}
		}
	}
	if s.Default {
		protocol := Chat
		if strings.HasSuffix(pathOnly, "/messages") {
			protocol = Messages
		}
		return &LocalReplyPolicy{Protocol: protocol, Profile: DeepSeek}
	}
	return nil
}

// ParseLocalReplySelector validates optional local_reply_policy, local_reply_entrances,
// and local_reply_default fields on a plugin configuration object at load time.
func ParseLocalReplySelector(parent gjson.Result) (LocalReplySelector, error) {
	if !parent.Exists() || !parent.IsObject() {
		return LocalReplySelector{}, nil
	}
	policy, err := ParseLocalReplyPolicyField(parent)
	if err != nil {
		return LocalReplySelector{}, err
	}
	var defaultCount, entrancesCount int
	var defaultVal, entrancesVal gjson.Result
	parent.ForEach(func(key, value gjson.Result) bool {
		switch key.String() {
		case LocalReplyDefaultField:
			defaultCount++
			defaultVal = value
		case LocalReplyEntrancesField:
			entrancesCount++
			entrancesVal = value
		}
		return true
	})
	if defaultCount > 1 {
		return LocalReplySelector{}, errors.New("duplicate local_reply_default field")
	}
	if entrancesCount > 1 {
		return LocalReplySelector{}, errors.New("duplicate local_reply_entrances field")
	}
	var defaultEnabled bool
	if defaultCount == 1 {
		if defaultVal.Type != gjson.True && defaultVal.Type != gjson.False {
			return LocalReplySelector{}, errors.New("local_reply_default must be a boolean")
		}
		defaultEnabled = defaultVal.Bool()
	}
	if policy != nil && (defaultEnabled || entrancesCount > 0) {
		return LocalReplySelector{}, errors.New("local_reply_policy is mutually exclusive with local_reply_default and local_reply_entrances")
	}
	var entrances []LocalReplyEntrance
	if entrancesCount == 1 {
		if !entrancesVal.IsArray() {
			return LocalReplySelector{}, errors.New("local_reply_entrances must be an array")
		}
		canonical, err := mfjson.Canonicalize([]byte(entrancesVal.Raw))
		if err != nil {
			return LocalReplySelector{}, fmt.Errorf("local_reply_entrances invalid JSON: %w", err)
		}
		var rawItems []json.RawMessage
		if err := json.Unmarshal(canonical, &rawItems); err != nil {
			return LocalReplySelector{}, fmt.Errorf("local_reply_entrances must be an array: %w", err)
		}
		entrances = make([]LocalReplyEntrance, 0, len(rawItems))
		for idx, rawItem := range rawItems {
			entrance, err := parseLocalReplyEntranceJSON(rawItem)
			if err != nil {
				return LocalReplySelector{}, fmt.Errorf("local_reply_entrances[%d]: %w", idx, err)
			}
			entrances = append(entrances, entrance)
		}
	}
	return LocalReplySelector{
		Policy:    policy,
		Entrances: entrances,
		Default:   defaultEnabled,
	}, nil
}

func parseLocalReplyEntranceJSON(raw []byte) (LocalReplyEntrance, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return LocalReplyEntrance{}, errors.New("entrance must be an object")
	}
	for key := range fields {
		if key != "domains" && key != "path" && key != LocalReplyField {
			return LocalReplyEntrance{}, fmt.Errorf("unknown field %q", key)
		}
	}
	rawPath, hasPath := fields["path"]
	rawPolicy, hasPolicy := fields[LocalReplyField]
	if !hasPath || !hasPolicy {
		return LocalReplyEntrance{}, errors.New("path and local_reply_policy are required")
	}
	policy, err := ParseLocalReplyPolicyJSON(rawPolicy)
	if err != nil || policy == nil {
		return LocalReplyEntrance{}, fmt.Errorf("local_reply_policy: %w", err)
	}
	var domains []string
	if rawDomains, hasDomains := fields["domains"]; hasDomains {
		if bytes.Equal(rawDomains, []byte("null")) {
			return LocalReplyEntrance{}, errors.New("domains must be an array")
		}
		if err := json.Unmarshal(rawDomains, &domains); err != nil {
			return LocalReplyEntrance{}, fmt.Errorf("domains must be an array of strings: %w", err)
		}
		for i, d := range domains {
			trimmed := normalizeAuthorityHost(strings.TrimSpace(d))
			if d == "" || trimmed == "" || d != strings.TrimSpace(d) {
				return LocalReplyEntrance{}, errors.New("domains entries must be non-empty hostnames")
			}
			domains[i] = trimmed
		}
	}
	var pathFields map[string]json.RawMessage
	if err := json.Unmarshal(rawPath, &pathFields); err != nil || pathFields == nil {
		return LocalReplyEntrance{}, errors.New("path must be an object")
	}
	for key := range pathFields {
		if key != "match_type" && key != "match_value" && key != "case_sensitive" {
			return LocalReplyEntrance{}, fmt.Errorf("path unknown field %q", key)
		}
	}
	rawMatchType, hasType := pathFields["match_type"]
	rawMatchVal, hasVal := pathFields["match_value"]
	if !hasType || !hasVal || bytes.Equal(rawMatchType, []byte("null")) || bytes.Equal(rawMatchVal, []byte("null")) {
		return LocalReplyEntrance{}, errors.New("path.match_type and path.match_value are required")
	}
	var matchType, matchValue string
	if err := json.Unmarshal(rawMatchType, &matchType); err != nil {
		return LocalReplyEntrance{}, errors.New("path.match_type must be a string")
	}
	if err := json.Unmarshal(rawMatchVal, &matchValue); err != nil || matchValue == "" {
		return LocalReplyEntrance{}, errors.New("path.match_value must be a non-empty string")
	}
	if matchType != "EQUAL" && matchType != "PRE" && matchType != "REGULAR" {
		return LocalReplyEntrance{}, fmt.Errorf("unsupported path.match_type %q", matchType)
	}
	caseSensitive := true
	if rawCS, hasCS := pathFields["case_sensitive"]; hasCS {
		if bytes.Equal(rawCS, []byte("null")) {
			return LocalReplyEntrance{}, errors.New("path.case_sensitive must be a boolean")
		}
		if err := json.Unmarshal(rawCS, &caseSensitive); err != nil {
			return LocalReplyEntrance{}, errors.New("path.case_sensitive must be a boolean")
		}
	}
	var compiled *regexp.Regexp
	if matchType == "REGULAR" {
		pattern := matchValue
		if !caseSensitive {
			pattern = "(?i)" + pattern
		}
		compiled, err = regexp.Compile(pattern)
		if err != nil {
			return LocalReplyEntrance{}, fmt.Errorf("invalid path regex: %w", err)
		}
	}
	return LocalReplyEntrance{
		Domains:       domains,
		MatchType:     matchType,
		MatchValue:    matchValue,
		CaseSensitive: caseSensitive,
		Regex:         compiled,
		Policy:        *policy,
	}, nil
}

func normalizeAuthorityHost(authority string) string {
	host := authority
	if idx := strings.LastIndexByte(host, ':'); idx != -1 {
		v6End := strings.LastIndexByte(host, ']')
		if v6End == -1 || v6End < idx {
			host = host[:idx]
		}
	}
	return strings.ToLower(host)
}

func matchesEntranceDomain(reqHost string, domains []string) bool {
	if reqHost == "" {
		return false
	}
	for _, pattern := range domains {
		switch {
		case strings.HasPrefix(pattern, "*"):
			if strings.HasSuffix(reqHost, pattern[1:]) {
				return true
			}
		case strings.HasSuffix(pattern, "*"):
			if strings.HasPrefix(reqHost, pattern[:len(pattern)-1]) {
				return true
			}
		default:
			if reqHost == pattern {
				return true
			}
		}
	}
	return false
}

func matchesEntrancePath(pathOnly string, entrance *LocalReplyEntrance) bool {
	switch entrance.MatchType {
	case "EQUAL":
		if entrance.CaseSensitive {
			return pathOnly == entrance.MatchValue
		}
		return strings.EqualFold(pathOnly, entrance.MatchValue)
	case "PRE":
		if entrance.CaseSensitive {
			return strings.HasPrefix(pathOnly, entrance.MatchValue)
		}
		return len(pathOnly) >= len(entrance.MatchValue) && strings.EqualFold(pathOnly[:len(entrance.MatchValue)], entrance.MatchValue)
	case "REGULAR":
		return entrance.Regex != nil && entrance.Regex.MatchString(pathOnly)
	default:
		return false
	}
}

// LocalReplyKind identifies a producer-known local rejection cause, completely
// separate from upstream supplier FailureKind / ClassifyFailure.
type LocalReplyKind string

const (
	LocalInvalidAPIKey       LocalReplyKind = "invalid_api_key"
	LocalMultipleAPIKeys     LocalReplyKind = "multiple_api_keys"
	LocalPermissionDenied    LocalReplyKind = "permission_denied"
	LocalInsufficientBalance LocalReplyKind = "insufficient_balance"
	LocalRateLimitExceeded   LocalReplyKind = "rate_limit_exceeded"
	LocalRequestTooLarge     LocalReplyKind = "request_too_large"
	LocalInvalidParameter    LocalReplyKind = "invalid_parameter"
	LocalDuplicateModel      LocalReplyKind = "duplicate_model"
	LocalServiceUnavailable  LocalReplyKind = "service_unavailable"
	LocalNotFound            LocalReplyKind = "not_found"
	LocalRequestTimeout      LocalReplyKind = "request_timeout"
	LocalGatewayError        LocalReplyKind = "gateway_error"
)

type localReplySpec struct {
	errorType string
	code      string
	message   string
}

func (k LocalReplyKind) spec() (localReplySpec, bool) {
	switch k {
	case LocalInvalidAPIKey:
		return localReplySpec{"authentication_error", "invalid_api_key", "A valid API key is required."}, true
	case LocalMultipleAPIKeys:
		return localReplySpec{"authentication_error", "multiple_api_keys", "Multiple distinct API keys were supplied."}, true
	case LocalPermissionDenied:
		return localReplySpec{"permission_error", "permission_denied", "The account is not permitted to use this service."}, true
	case LocalInsufficientBalance:
		return localReplySpec{"insufficient_quota", "insufficient_balance", "The account has insufficient balance."}, true
	case LocalRateLimitExceeded:
		return localReplySpec{"rate_limit_error", "rate_limit_exceeded", "The request limit has been exceeded. Try again later."}, true
	case LocalRequestTooLarge:
		return localReplySpec{"invalid_request_error", "request_too_large", "The request is too large."}, true
	case LocalInvalidParameter:
		return localReplySpec{"invalid_request_error", "invalid_parameter", "The request body or model is invalid."}, true
	case LocalDuplicateModel:
		return localReplySpec{"invalid_request_error", "invalid_parameter", "The request contains duplicate model fields."}, true
	case LocalServiceUnavailable:
		return localReplySpec{"api_error", "service_unavailable", "The service is temporarily unavailable."}, true
	case LocalNotFound:
		return localReplySpec{"not_found_error", "not_found", "The requested endpoint was not found."}, true
	case LocalRequestTimeout:
		return localReplySpec{"api_error", "request_timeout", "The service timed out. Try again later."}, true
	case LocalGatewayError:
		return localReplySpec{"api_error", "gateway_error", "The service could not complete the request. Try again later."}, true
	default:
		return localReplySpec{}, false
	}
}

// LocalReplyKindForDetail maps exact producer SendHttpResponseWithDetail tags
// to their public local error contract without guessing from HTTP status codes.
func LocalReplyKindForDetail(detail string) (LocalReplyKind, bool) {
	switch detail {
	case "key-auth.no_key", "key-auth.invalid_credential":
		return LocalInvalidAPIKey, true
	case "key-auth.multi_key":
		return LocalMultipleAPIKeys, true
	case "key-auth.unauthorized",
		"ai-quota.missing_identity",
		"execution_admission_check_denied",
		"execution_admission_caller_identity_missing",
		"ai-cache.execution_delivery.authenticated_caller_missing",
		"ai-cache.execution_delivery.current_execution_admission_required",
		"ai-cache.execution_delivery.execution_admission_expired":
		return LocalPermissionDenied, true
	case "ai-quota.no_balance",
		"ai-quota.insufficient_balance",
		"execution_admission_insufficient_balance":
		return LocalInsufficientBalance, true
	case "ai-token-ratelimit.rejected":
		return LocalRateLimitExceeded, true
	case "duplicate_model_member":
		return LocalDuplicateModel, true
	case "modelfusion.local.model-mapper.body_too_large",
		"modelfusion.local.ai-history.body_too_large",
		"modelfusion.local.ai-memory.body_too_large":
		return LocalRequestTooLarge, true
	case "modelfusion.local.model-mapper.invalid_model_body",
		"modelfusion.local.model-mapper.body_required",
		"modelfusion.local.model-mapper.json_required",
		"modelfusion.local.model-mapper.content_encoding_unsupported",
		"modelfusion.local.model-mapper.body_basis_model_differs",
		"modelfusion.local.model-mapper.original_body_basis_missing",
		"modelfusion.local.model-mapper.model_mapping_missing",
		"modelfusion.local.model-mapper.post_required",
		"modelfusion.local.ai-history.invalid_model_body",
		"modelfusion.local.ai-history.body_required",
		"modelfusion.local.ai-history.json_required",
		"modelfusion.local.ai-history.content_encoding_unsupported",
		"modelfusion.local.ai-history.body_basis_model_differs",
		"modelfusion.local.ai-history.original_body_basis_missing",
		"modelfusion.local.ai-history.model_mapping_missing",
		"modelfusion.local.ai-history.post_required",
		"modelfusion.local.ai-memory.invalid_model_body",
		"modelfusion.local.ai-memory.body_required",
		"modelfusion.local.ai-memory.json_required",
		"modelfusion.local.ai-memory.content_encoding_unsupported",
		"modelfusion.local.ai-memory.body_basis_model_differs",
		"modelfusion.local.ai-memory.original_body_basis_missing",
		"modelfusion.local.ai-memory.model_mapping_missing",
		"modelfusion.local.ai-memory.post_required",
		"ai-cache.execution_delivery.original_basis_missing",
		"ai-cache.execution_delivery.json_text_request_required",
		"ai-cache.execution_delivery.content_encoding_unsupported",
		"ai-cache.execution_delivery.original_body_invalid",
		"ai-cache.execution_delivery.current_body_invalid",
		"ai-cache.execution_delivery.original_model_differs":
		return LocalInvalidParameter, true
	}
	if strings.HasPrefix(detail, "ai-quota.") ||
		strings.HasPrefix(detail, "execution_admission_") ||
		strings.HasPrefix(detail, "modelfusion.local.") ||
		strings.HasPrefix(detail, "ai-cache.execution_delivery.") {
		return LocalServiceUnavailable, true
	}
	return "", false
}

// LocalReplyInput provides producer-owned rejection facts for direct encoding.
type LocalReplyInput struct {
	Policy            LocalReplyPolicy
	Kind              LocalReplyKind
	Status            uint32
	Detail            string
	RequestID         string
	Headers           [][2]string
	CreatedAt         int64
	ResponseCommitted bool
}

// LocalReplyOutput contains the encoded JSON envelope and safe headers to send.
type LocalReplyOutput struct {
	Status  uint32
	Detail  string
	Headers [][2]string
	Body    []byte
}

// EncodeLocalReply formats a pre-response gateway-local rejection according to
// the frozen protocol envelope, preserving producer status, detail, and safe headers.
func EncodeLocalReply(input LocalReplyInput) (LocalReplyOutput, error) {
	if input.ResponseCommitted {
		return LocalReplyOutput{}, ErrLocalReplyAfterResponseCommitted
	}
	if !input.Policy.Valid() {
		return LocalReplyOutput{}, ErrInvalidLocalReplyPolicy
	}
	kind := input.Kind
	if kind == "" {
		var ok bool
		kind, ok = LocalReplyKindForDetail(input.Detail)
		if !ok {
			return LocalReplyOutput{}, fmt.Errorf("unregistered local reply detail %q", input.Detail)
		}
	}
	spec, ok := kind.spec()
	if !ok {
		return LocalReplyOutput{}, fmt.Errorf("unregistered local reply kind %q", kind)
	}
	headers := make([][2]string, 0, len(input.Headers)+2)
	retryAdded := false
	for _, pair := range input.Headers {
		switch strings.ToLower(pair[0]) {
		case "www-authenticate",
			"x-tokenratelimit-reset",
			"x-ratelimit-reset",
			"x-ratelimit-limit",
			"x-ratelimit-remaining",
			"cache-control",
			"content-language":
			headers = append(headers, pair)
		case "retry-after":
			if !retryAdded && validRetryAfter(pair[1], input.CreatedAt) {
				headers = append(headers, [2]string{"retry-after", pair[1]})
				retryAdded = true
			}
		}
	}
	headers = append(headers, [2]string{"content-type", "application/json"})
	if reqID := cleanRequestID(input.RequestID); reqID != "" {
		headers = append(headers, [2]string{"x-request-id", reqID})
	}
	return LocalReplyOutput{
		Status:  input.Status,
		Detail:  input.Detail,
		Headers: headers,
		Body:    marshalErrorEnvelope(input.Policy.Protocol, spec.errorType, spec.code, spec.message, ""),
	}, nil
}

// ResponseHeadersCommitted reports whether ctx has entered or completed the HTTP response phase.
func ResponseHeadersCommitted(ctx iface.HttpContext) bool {
	if ctx == nil {
		return false
	}
	phase := ctx.GetExecutionPhase()
	return phase == iface.EncodeHeader || phase == iface.EncodeData || phase == iface.Done
}

// SendLocalReply encodes and sends a pre-response local rejection when policy is configured.
// When policy is nil, it returns (false, nil) so legacy configurations keep their existing path.
func SendLocalReply(ctx iface.HttpContext, policy *LocalReplyPolicy, status uint32, detail string, extraHeaders [][2]string) (bool, error) {
	if policy == nil {
		return false, nil
	}
	committed := ResponseHeadersCommitted(ctx)
	var requestID string
	var createdAt int64
	if ctx != nil {
		requestID = ctx.GetStringContext(originalReqIDKey, "")
		if start, ok := ctx.GetContext(originalReqAtKey).(time.Time); ok && !start.IsZero() {
			createdAt = start.Unix()
		}
	}
	if cleanRequestID(requestID) == "" {
		if headerID, err := proxywasm.GetHttpRequestHeader("x-request-id"); err == nil {
			requestID = headerID
		}
	}
	if cleanRequestID(requestID) == "" {
		if propID, err := proxywasm.GetProperty([]string{"x_request_id"}); err == nil && len(propID) > 0 {
			requestID = string(propID)
		}
	}
	out, err := EncodeLocalReply(LocalReplyInput{
		Policy:            *policy,
		Status:            status,
		Detail:            detail,
		RequestID:         requestID,
		Headers:           extraHeaders,
		CreatedAt:         createdAt,
		ResponseCommitted: committed,
	})
	if err != nil {
		return true, err
	}
	if err := proxywasm.SendHttpResponseWithDetail(out.Status, out.Detail, out.Headers, out.Body, -1); err != nil {
		return true, fmt.Errorf("send local reply %s: %w", detail, err)
	}
	return true, nil
}

func cleanRequestID(value string) string {
	if value == "" || value == "-" || len(value) > maxRequestIDBytes || !utf8.ValidString(value) {
		return ""
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return ""
		}
	}
	return value
}
