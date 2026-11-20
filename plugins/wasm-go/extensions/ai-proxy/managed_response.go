package main

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const managedPublicProtocolKey = "ai-proxy.public-client-protocol"
const managedPublicProfileKey = "ai-proxy.public-response-profile"
const managedPublicTextRejectKey = "ai-proxy.public-text-reject"

func managedPublicPolicy(s *managedExecutionState) clientresponse.Policy {
	if s.publicAliases == nil {
		s.publicAliases = &clientresponse.ResourceAliases{}
	}
	created, _ := time.Parse(time.RFC3339Nano, s.businessUTC)
	return clientresponse.Policy{Protocol: s.config.profile.InputProtocol, Profile: s.config.settings.ResponseProfile, PublicModel: s.config.release.PublicModel, UpstreamModel: s.config.release.Descriptor.UpstreamModel, RequestID: s.callID, CreatedAt: created.Unix(), IncludeUsage: gjson.GetBytes(s.publicRequest, "stream_options.include_usage").Bool(), ExposeServiceTier: gjson.GetBytes(s.publicRequest, "service_tier").String() != "", RequestFields: clientresponse.RequestFields(s.publicRequest), Aliases: s.publicAliases}
}

func managedPublicText(s *managedExecutionState) bool {
	p := s.config.profile.InputProtocol
	return s.internal == nil && managedOperation(s.config) == "text_generate" && (p == clientresponse.Chat || p == clientresponse.Responses || p == clientresponse.Messages)
}

func projectManagedResponse(s *managedExecutionState, originalHeaders [][2]string, original []byte, response managedProtocolClientResponse) (managedProtocolClientResponse, error) {
	if !managedPublicText(s) {
		return response, nil
	}
	stream, err := managedProtocolContentType(response.Headers)
	if err != nil {
		return managedProtocolClientResponse{}, err
	}
	sourceStream, err := managedProtocolContentType(originalHeaders)
	if err != nil {
		return managedProtocolClientResponse{}, err
	}
	format := s.config.profile.UpstreamProtocol
	if s.config.vendor != nil && s.config.vendor.NativeResponseFormat() != "" {
		format = s.config.vendor.NativeResponseFormat()
	}
	usage := clientresponse.ReadUsage(original, format, sourceStream)
	if format != clientresponse.Chat && format != clientresponse.Responses && format != clientresponse.Messages && format != "qwen" {
		// Other registered native codecs already restored their original exact
		// measurements; retain that established subset without inventing details.
		usage = clientresponse.ReadUsage(response.Body, s.config.profile.InputProtocol, stream)
	}
	policy := managedPublicPolicy(s)
	body, err := clientresponse.Project(policy, response.Body, stream, usage)
	if err != nil {
		return managedProtocolClientResponse{}, err
	}
	publicHeaders := clientresponse.Headers(policy, response.Headers, stream)
	if stream {
		if events, e := clientresponse.Events(original); e == nil {
			for _, event := range events {
				kind := gjson.GetBytes(event.Data, "type").String()
				if event.Name == "error" || kind == "error" || kind == "response.failed" {
					failure := clientresponse.ClassifyFailure(clientresponse.FailureInput{Format: format, Client: policy.Protocol, Body: event.Data, Request: s.publicRequest})
					publicHeaders = failure.Headers(policy, response.Headers, true, true)
					break
				}
			}
		}
	}
	return managedProtocolClientResponse{Body: body, Headers: publicHeaders}, nil
}

func managedPublicError(ctx wrapper.HttpContext, status uint32, reason string) ([]byte, [][2]string) {
	headers := [][2]string{{"content-type", "application/json"}}
	if ctx == nil {
		return managedErrorBody(reason), headers
	}
	protocol := ctx.GetStringContext(managedPublicProtocolKey, "")
	if !managedIsPublicTextRejection(ctx, protocol, reason) {
		return managedErrorBody(reason), headers
	}
	if status >= 400 && status < 500 {
		headers = append(headers, [2]string{"x-should-retry", "false"})
	}
	if reqID := managedRejectionRequestID(ctx); reqID != "" {
		headers = append(headers, [2]string{"x-request-id", reqID})
	}
	errorType, code, message := managedLocalErrorSpec(status, reason)
	return marshalManagedLocalError(protocol, errorType, code, message), headers
}

func managedIsPublicTextRejection(ctx wrapper.HttpContext, protocol, reason string) bool {
	if explicit, ok := ctx.GetContext(managedPublicTextRejectKey).(bool); ok && !explicit {
		return false
	}
	if protocol != clientresponse.Chat && protocol != clientresponse.Responses && protocol != clientresponse.Messages {
		return false
	}
	if strings.HasPrefix(reason, "media_") || strings.HasPrefix(reason, "internal_execution_") || reason == "registered_internal_service_call_required" {
		return false
	}
	return true
}

func managedRejectionRequestID(ctx wrapper.HttpContext) string {
	if state, ok := ctx.GetContext(managedExecutionContextKey).(*managedExecutionState); ok && state != nil && managedID(state.callID) {
		return state.callID
	}
	if reqID, err := proxywasm.GetHttpRequestHeader("x-request-id"); err == nil && reqID != "-" && managedID(reqID) {
		return reqID
	}
	return ""
}

func managedLocalErrorSpec(status uint32, reason string) (string, string, string) {
	switch reason {
	case "resource_reference_unavailable":
		return "invalid_request_error", "resource_reference_unavailable", "The resource reference is unavailable or expired. Resend the full conversation history."
	case "resource_reference_incompatible":
		return "invalid_request_error", "resource_reference_incompatible", "These resource references cannot be used together on this service. Resend the full conversation history."
	}
	switch status {
	case 400:
		return "invalid_request_error", "invalid_parameter", "A request parameter is invalid. Check the indicated parameter."
	case 403:
		return "permission_error", "permission_denied", "The account is not permitted to use this service."
	case 409:
		return "invalid_request_error", "invalid_parameter", "The request could not be completed due to a request state conflict."
	case 413:
		return "invalid_request_error", "request_too_large", "The request is too large."
	case 502:
		return "api_error", "gateway_error", "The service could not complete the request. Try again later."
	case 503:
		return "api_error", "service_unavailable", "The service is currently unavailable. Contact support with the request ID."
	case 504:
		return "api_error", "request_timeout", "The service timed out. Try again later."
	default:
		return "api_error", "server_error", "The service could not complete the request. Try again later."
	}
}

func marshalManagedLocalError(protocol, errorType, code, message string) []byte {
	inner, _ := json.Marshal(map[string]string{
		"type":    errorType,
		"code":    code,
		"message": message,
	})
	envelope := map[string]json.RawMessage{
		"error": inner,
	}
	if protocol == clientresponse.Messages {
		envelope["type"] = json.RawMessage(`"error"`)
	}
	raw, _ := json.Marshal(envelope)
	return raw
}

// configureManagedResponse validates the only response branch for this target.
func configureManagedResponse(c *managedExecutionConfig, value gjson.Result) error {
	c.settings.ProtocolMode = value.Get("protocol_mode").String()
	if c.settings.ProtocolMode == "" {
		c.settings.ProtocolMode = "auto"
	}
	if c.settings.ProtocolMode == "passthrough" {
		switch c.profile.InputProtocol {
		case clientresponse.Chat, clientresponse.Responses, clientresponse.Messages, managedProtocolCompletions:
		default:
			if !executionfacts.IsRetrieval(managedOperation(c)) {
				return errors.New("passthrough requires a registered native wire protocol")
			}
		}
		if c.settings.CallIdentitySource != executionfacts.CallIdentitySourceAdmission || c.release.PublicModel != c.release.Descriptor.UpstreamModel || c.profile.InputProtocol != c.profile.UpstreamProtocol || (c.vendor != nil && c.vendor.NativeResponseFormat() != "") || c.settings.ResponseProfile != "" {
			return errors.New("passthrough requires the exact model and native wire protocol without a response profile")
		}
		return nil
	}
	if c.settings.ProtocolMode != "auto" || c.profile.InputProtocol == managedProtocolCompletions || executionfacts.IsRetrieval(managedOperation(c)) {
		return errors.New("unsupported managed response mode")
	}
	if c.profile.InputProtocol != clientresponse.Chat && c.profile.InputProtocol != clientresponse.Responses && c.profile.InputProtocol != clientresponse.Messages {
		if c.settings.ResponseProfile != "" {
			return errors.New("response profiles are only available for public text protocols")
		}
		return nil
	}
	profile, err := clientresponse.ParseProfile(string(c.settings.ResponseProfile), c.profile.InputProtocol)
	if err != nil {
		return err
	}
	c.settings.ResponseProfile = profile
	return nil
}
