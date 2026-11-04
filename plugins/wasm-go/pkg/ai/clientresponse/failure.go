package clientresponse

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

// FailureKind is an internal cause. It is never serialized to the public API.
type FailureKind string

const (
	FailureAuthentication FailureKind = "procurement_authentication"
	FailurePermission     FailureKind = "procurement_permission"
	FailureConfiguration  FailureKind = "service_configuration"
	FailureQuota          FailureKind = "procurement_quota"
	FailureRate           FailureKind = "rate_limit"
	FailureOverloaded     FailureKind = "overloaded"
	FailureParameter      FailureKind = "customer_parameter"
	FailurePolicy         FailureKind = "input_policy"
	FailureInterrupted    FailureKind = "generation_interrupted"
	FailureIncomplete     FailureKind = "response_incomplete"
	FailureInvalid        FailureKind = "invalid_response"
	FailureReference      FailureKind = "resource_reference_unavailable"
	FailureReferenceMode  FailureKind = "resource_reference_incompatible"
	FailureServer         FailureKind = "server_failure"
)

// FailureInput describes original supplier evidence, not an encoded public body.
type FailureInput struct {
	Status         uint16
	Format, Client string
	Body, Request  []byte
}

type Failure struct {
	Kind            FailureKind
	Param           string
	Retryable       bool
	RetryCredential bool
	AllowFallback   bool
}

// ClassifyFailure keeps recovery decisions private. Public messages are chosen
// from fixed platform descriptions rather than cleaned supplier diagnostics.
func ClassifyFailure(input FailureInput) Failure {
	object := gjson.ParseBytes(input.Body)
	fields := object.Get("error")
	if !fields.IsObject() {
		fields = object
	}
	code := strings.ToLower(fields.Get("code").String())
	kind := strings.ToLower(fields.Get("type").String())
	status := strings.ToLower(fields.Get("status").String())
	failure := Failure{Kind: FailureServer, Retryable: true, RetryCredential: true, AllowFallback: true}
	switch code {
	case "resource_reference_unavailable":
		return Failure{Kind: FailureReference}
	case "resource_reference_incompatible":
		return Failure{Kind: FailureReferenceMode}
	case "service_unavailable", "resource_owner_unavailable", "resource_mapping_unavailable":
		return Failure{Kind: FailureConfiguration}
	}
	if input.Status == 401 || code == "invalid_api_key" || code == "invalidapikey" || kind == "authentication_error" || kind == "invalid_api_key" {
		return Failure{Kind: FailureAuthentication, RetryCredential: true, AllowFallback: true}
	}
	if input.Status == 403 {
		return Failure{Kind: FailurePermission, AllowFallback: true}
	}
	// Supplier billing/account limits can share HTTP status with request errors
	// or transient rate limits. Classify documented business codes first.
	for _, account := range []string{"arrearage", "allocationquota", "insufficient_quota", "insufficient_balance", "prepaidbilloverdue", "budgetlimitexceeded", "billing_error", "creditbalance", "spend_limit"} {
		if strings.Contains(code, account) || strings.Contains(kind, account) {
			return Failure{Kind: FailureQuota, AllowFallback: true}
		}
	}
	if input.Status == 402 {
		return Failure{Kind: FailureQuota, AllowFallback: true}
	}
	if code == "content_policy_violation" || code == "datainspectionfailed" || code == "data_inspection_failed" || object.Get("promptFeedback.blockReason").String() != "" {
		return Failure{Kind: FailurePolicy}
	}
	if code == "generation_interrupted" || code == "aborted" {
		return Failure{Kind: FailureInterrupted}
	}
	if code == "response_incomplete" || code == "upstream_protocol_response_incomplete" || code == "upstream_response_unavailable" {
		return Failure{Kind: FailureIncomplete}
	}
	if code == "invalid_response" {
		return Failure{Kind: FailureInvalid}
	}
	if input.Status == 503 || input.Status == 529 || strings.Contains(code, "serviceoverloaded") || code == "overloaded_error" || kind == "overloaded_error" || code == "serviceunavailable" || code == "insufficient_system_resource" || code == "server_is_overloaded" {
		return Failure{Kind: FailureOverloaded, Retryable: true, RetryCredential: true, AllowFallback: true}
	}
	if input.Status == 429 || strings.Contains(code, "ratequota") || strings.Contains(code, "concurrency") || code == "rate_limit_exceeded" || kind == "rate_limit_error" || status == "resource_exhausted" {
		return Failure{Kind: FailureRate, Retryable: true, RetryCredential: true, AllowFallback: true}
	}
	if input.Status == 400 || input.Status == 422 || code == "invalidparameter" || kind == "invalid_request_error" {
		if param := customerParameter(input, fields); param != "" {
			return Failure{Kind: FailureParameter, Param: param}
		}
		// A supplier/converted request may be invalid even though the customer
		// supplied a valid platform request. Do not blame an unproven parameter.
		return Failure{Kind: FailureConfiguration, AllowFallback: true}
	}
	if code == "service_unavailable" || input.Status == 404 {
		return Failure{Kind: FailureConfiguration, AllowFallback: true}
	}
	return failure
}

func customerParameter(input FailureInput, fields gjson.Result) string {
	param := fields.Get("param").String()
	allowed := strings.Fields("model messages input max_tokens max_completion_tokens max_output_tokens temperature top_p stream stop tools tool_choice parallel_tool_calls response_format text reasoning presence_penalty frequency_penalty seed logprobs top_logprobs n")
	if param == "" && strings.EqualFold(fields.Get("code").String(), "InvalidParameter") {
		message := fields.Get("message").String()
		for _, candidate := range allowed {
			if strings.HasPrefix(message, candidate+" ") || strings.Contains(message, "`"+candidate+"`") || strings.Contains(message, "'"+candidate+"'") {
				param = candidate
				break
			}
		}
	}
	if input.Client == Responses && (param == "max_tokens" || param == "max_completion_tokens") {
		param = "max_output_tokens"
	}
	for _, candidate := range allowed {
		if candidate == param && gjson.GetBytes(input.Request, param).Exists() {
			return param
		}
	}
	return ""
}

type publicFailure struct {
	status                uint16
	errorType, code, text string
}

func (f Failure) public(protocol string) publicFailure {
	switch f.Kind {
	case FailureAuthentication, FailurePermission, FailureConfiguration, FailureQuota:
		return publicFailure{503, "api_error", "service_unavailable", "The service is currently unavailable. Contact support with the request ID."}
	case FailureRate:
		return publicFailure{429, "rate_limit_error", "rate_limit_exceeded", "The service is receiving too many requests. Try again later."}
	case FailureOverloaded:
		status := uint16(503)
		if protocol == Messages {
			status = 529
		}
		return publicFailure{status, "overloaded_error", "server_is_overloaded", "The service is temporarily busy. Try again later."}
	case FailureParameter:
		return publicFailure{400, "invalid_request_error", "invalid_parameter", "A request parameter is invalid. Check the indicated parameter."}
	case FailurePolicy:
		return publicFailure{400, "invalid_request_error", "content_policy_violation", "The request could not be processed under the service content policy."}
	case FailureInterrupted:
		return publicFailure{502, "api_error", "generation_interrupted", "Generation was interrupted. Partial output may be incomplete."}
	case FailureIncomplete:
		return publicFailure{502, "api_error", "response_incomplete", "The response ended without a complete terminal result."}
	case FailureInvalid:
		return publicFailure{502, "api_error", "invalid_response", "The service returned an invalid response."}
	case FailureReference:
		return publicFailure{400, "invalid_request_error", "resource_reference_unavailable", "The resource reference is unavailable or expired. Resend the full conversation history."}
	case FailureReferenceMode:
		return publicFailure{400, "invalid_request_error", "resource_reference_incompatible", "These resource references cannot be used together on this service. Resend the full conversation history."}
	default:
		return publicFailure{500, "api_error", "server_error", "The service could not complete the request. Try again later."}
	}
}

func (f Failure) Status(protocol string) uint16 { return f.public(protocol).status }

func (f Failure) ResourceError(protocol string) json.RawMessage {
	public := f.public(protocol)
	raw, _ := json.Marshal(map[string]string{"code": public.code, "message": public.text})
	return raw
}

func (f Failure) Body(policy Policy) []byte {
	public := f.public(policy.Protocol)
	fields := map[string]string{"type": public.errorType, "code": public.code, "message": public.text}
	if f.Param != "" {
		fields["param"] = f.Param
	}
	result := map[string]json.RawMessage{}
	result["error"], _ = json.Marshal(fields)
	if policy.Protocol == Messages {
		result["type"] = json.RawMessage(`"error"`)
	}
	raw, _ := json.Marshal(result)
	return raw
}

func (f Failure) Headers(policy Policy, original [][2]string, stream, partial bool) [][2]string {
	var clean [][2]string
	for _, header := range original {
		if !strings.EqualFold(header[0], "retry-after") {
			clean = append(clean, header)
		}
	}
	result := Headers(policy, clean, stream)
	retry := "false"
	if f.Retryable && !partial {
		retry = "true"
		for _, header := range original {
			if strings.EqualFold(header[0], "retry-after") && validRetryAfter(header[1], policy.CreatedAt) {
				result = append(result, [2]string{"retry-after", header[1]})
				break
			}
		}
	}
	return append(result, [2]string{"x-should-retry", retry})
}

func validRetryAfter(value string, created int64) bool {
	if value == "" || len(value) > 64 || strings.TrimSpace(value) != value {
		return false
	}
	if seconds, err := strconv.ParseUint(value, 10, 32); err == nil {
		return seconds <= 86400
	}
	when, err := http.ParseTime(value)
	if err != nil || created <= 0 {
		return false
	}
	delta := when.Sub(time.Unix(created, 0))
	return delta >= 0 && delta <= 24*time.Hour
}
