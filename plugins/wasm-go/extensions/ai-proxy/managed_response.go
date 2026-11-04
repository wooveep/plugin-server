package main

import (
	"time"

	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const managedPublicProtocolKey = "ai-proxy.public-client-protocol"

func managedPublicPolicy(s *managedExecutionState) clientresponse.Policy {
	if s.publicAliases == nil {
		s.publicAliases = &clientresponse.ResourceAliases{}
	}
	created, _ := time.Parse(time.RFC3339Nano, s.businessUTC)
	return clientresponse.Policy{Protocol: s.config.profile.InputProtocol, PublicModel: s.config.release.PublicModel, UpstreamModel: s.config.release.Descriptor.UpstreamModel, RequestID: s.callID, CreatedAt: created.Unix(), IncludeUsage: gjson.GetBytes(s.publicRequest, "stream_options.include_usage").Bool(), ExposeServiceTier: gjson.GetBytes(s.publicRequest, "service_tier").String() != "", RequestFields: clientresponse.RequestFields(s.publicRequest), Aliases: s.publicAliases}
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
	return clientresponse.Error(clientresponse.Policy{Protocol: ctx.GetStringContext(managedPublicProtocolKey, "")}, managedErrorBody(reason))
}
