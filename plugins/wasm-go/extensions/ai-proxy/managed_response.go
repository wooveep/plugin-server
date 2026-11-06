package main

import (
	"errors"
	"time"

	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const managedPublicProtocolKey = "ai-proxy.public-client-protocol"
const managedPublicProfileKey = "ai-proxy.public-response-profile"

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

func managedPublicError(ctx wrapper.HttpContext, reason string) []byte {
	return clientresponse.Error(clientresponse.Policy{Protocol: ctx.GetStringContext(managedPublicProtocolKey, ""), Profile: clientresponse.Profile(ctx.GetStringContext(managedPublicProfileKey, ""))}, managedErrorBody(reason))
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
