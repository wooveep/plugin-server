package main

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func validateManagedRouterProfile(c *managedExecutionConfig) error {
	settings, profile := c.settings, c.profile
	if settings.SendTransport == "" && !settings.StreamProjection && settings.StreamProtocol == "" {
		return nil
	}
	protocol := profile.InputProtocol
	native := protocol == profile.UpstreamProtocol
	dashScope := protocol == managedProtocolDashScopeText || protocol == managedProtocolDashScopeMultimodal
	switch protocol {
	case managedProtocolChat:
		if !native || settings.StreamProtocol != "" {
			return errors.New("unsupported managed Router stream protocol")
		}
	case managedProtocolCompletions:
		if !native || settings.StreamProtocol != protocol {
			return errors.New("unsupported managed Router stream protocol")
		}
	case managedProtocolResponses, managedProtocolMessages:
		if settings.StreamProtocol != protocol || !native && (profile.UpstreamProtocol != managedProtocolChat || !settings.StreamProjection) {
			return errors.New("unsupported managed Router stream protocol")
		}
	case managedProtocolDashScopeText, managedProtocolDashScopeMultimodal:
		if !native || settings.StreamProtocol != protocol || c.vendor == nil || !c.vendor.NativeQwen() {
			return errors.New("unsupported managed Router stream protocol")
		}
	default:
		return errors.New("unsupported managed Router stream protocol")
	}
	if settings.SendTransport != "router" || settings.StreamProjection != (settings.ProtocolMode == "auto" && !dashScope) ||
		settings.CallIdentitySource != executionfacts.CallIdentitySourceAdmission || settings.TextExecution != nil || settings.ToolPolicy != nil ||
		!dashScope && c.vendor != nil && c.vendor.NativeResponseFormat() != "" {
		return errors.New("unsupported managed Router response profile")
	}
	return nil
}

func projectManagedRouterChunk(ctx wrapper.HttpContext, s *managedExecutionState, chunk []byte, end bool) []byte {
	r := s.router
	var output bytes.Buffer
	r.pushStream(chunk, func(event managedProtocolEvent) {
		if r.projectionFailed || r.projectedError || bytes.Equal(bytes.TrimSpace(event.Data), []byte("[DONE]")) {
			return
		}
		if r.chat != nil {
			if r.chat.invalid || r.chat.usageConflict || r.chat.modelConflict {
				r.projectionFailed = true
				return
			}
			if usage := clientresponse.ReadUsage(event.Data, clientresponse.Chat, false); usage.Input != nil {
				r.projectionUsage = usage
			}
		} else {
			r.projectionUsage = r.native.publicUsage
		}

		parsed := gjson.ParseBytes(event.Data)
		kind := parsed.Get("type").String()
		isError := event.Name == "error" || kind == "error" || !managedProtocolNull(json.RawMessage(parsed.Get("error").Raw))
		if r.responsesConverter != nil || r.messagesConverter != nil {
			if isError {
				failure := clientresponse.ClassifyFailure(clientresponse.FailureInput{Format: clientresponse.Chat, Client: managedPublicPolicy(s).Protocol, Body: event.Data, Request: s.publicRequest})
				appendManagedRouterError(s, &output, failure)
				r.projectedError = true
				return
			}
			r.projectConvertedChatEvent(ctx, &output, event, parsed)
			return
		}
		if r.native != nil && r.native.terminal && kind != "error" {
			// The usage and ownership attached to this frame must be complete
			// before a consumer can treat it as a terminal or continue from it.
			r.terminalEvent = clientresponse.Event{Name: event.Name, Data: event.Data}
			return
		}
		r.appendProjection(&output, clientresponse.Event{Name: event.Name, Data: event.Data})
		r.projectedError = isError
	})
	if !end || r.projectedError {
		return output.Bytes()
	}
	observed := r.streamObservation()
	if r.chat != nil && observed.Usage.Status != "reported" {
		r.projectionUsage = clientresponse.Usage{}
	}
	if !r.projectionFailed && observed.BodyComplete && (r.native != nil || observed.ExecutionResult == "succeeded" || observed.ExecutionResult == "partial") {
		switch {
		case r.responsesConverter != nil || r.messagesConverter != nil:
			if observed.ProviderError {
				r.projectionFailed = true
			} else {
				tail := bytes.Join([][]byte{r.conversionFinish, r.conversionUsage, []byte("data: [DONE]\n\n")}, nil)
				r.appendConvertedChat(ctx, &output, tail, true)
			}
		case r.native != nil:
			if len(r.terminalEvent.Data) != 0 {
				r.appendProjection(&output, r.terminalEvent)
			}
		case !observed.ProviderError:
			r.appendProjection(&output, clientresponse.Event{Data: []byte("[DONE]")})
		}
	} else if !observed.ProviderError || r.responsesConverter != nil || r.messagesConverter != nil {
		r.projectionFailed = true
	}
	if r.projectionFailed {
		appendManagedRouterError(s, &output, clientresponse.Failure{Kind: clientresponse.FailureIncomplete})
	}
	return output.Bytes()
}

func (r *managedRouterResponse) projectConvertedChatEvent(ctx wrapper.HttpContext, output *bytes.Buffer, event managedProtocolEvent, parsed gjson.Result) {
	body, err := managedProtocolConversionObject(event.Data, true)
	if err != nil {
		r.projectionFailed = true
		return
	}
	if r.messagesConverter != nil && parsed.Get("choices.0.delta").IsObject() && !parsed.Get("choices.0.delta.role").Exists() {
		// Chat deltas without a role still belong to the same assistant.
		body, err = sjson.SetBytes(body, "choices.0.delta.role", "assistant")
		if err != nil {
			r.projectionFailed = true
			return
		}
	}
	var wire bytes.Buffer
	managedProtocolWriteEvent(&wire, event.Name, body)
	if !managedProtocolNull(json.RawMessage(parsed.Get("choices.0.finish_reason").Raw)) {
		// The converters close text and tool arguments on finish_reason.
		// Keep those events pending until the Chat terminal is verified.
		r.conversionFinish = wire.Bytes()
		return
	}
	if !managedProtocolNull(json.RawMessage(parsed.Get("usage").Raw)) && r.chat.allChoicesFinished() {
		r.conversionUsage = wire.Bytes()
		return
	}
	r.appendConvertedChat(ctx, output, wire.Bytes(), false)
}

func (r *managedRouterResponse) appendConvertedChat(ctx wrapper.HttpContext, output *bytes.Buffer, body []byte, end bool) {
	var converted []byte
	var err error
	if r.responsesConverter != nil {
		converted, err = r.responsesConverter.Convert(body, end)
	} else {
		converted, err = r.messagesConverter.ConvertOpenAIStreamResponseToClaude(ctx, body)
	}
	if err != nil {
		r.projectionFailed = true
		return
	}
	if len(converted) == 0 {
		return
	}
	events, err := managedProtocolEvents(converted)
	if err != nil {
		r.projectionFailed = true
		return
	}
	for _, event := range events {
		if r.messagesConverter != nil && event.Name == "message_delta" {
			// The converter's bounded input uses placeholder counts. Its
			// output count must not override the real Chat terminal usage.
			event.Data, err = sjson.DeleteBytes(event.Data, "usage")
			if err != nil {
				r.projectionFailed = true
				return
			}
		}
		if !r.appendProjection(output, clientresponse.Event{Name: event.Name, Data: event.Data}) {
			return
		}
	}
}

func (r *managedRouterResponse) appendProjection(output *bytes.Buffer, event clientresponse.Event) bool {
	if err := r.projection.Append(output, event, r.projectionUsage); err != nil {
		r.projectionFailed = true
		return false
	}
	if sequence := gjson.GetBytes(event.Data, "sequence_number"); sequence.Exists() {
		r.projectionSequence = max(r.projectionSequence, sequence.Int()+1)
	}
	return true
}

func appendManagedRouterError(s *managedExecutionState, output *bytes.Buffer, failure clientresponse.Failure) {
	r := s.router
	policy := managedPublicPolicy(s)
	if r.responsesConverter != nil {
		wire, err := r.responsesConverter.Fail(failure.ResourceError(policy.Protocol))
		if err == nil {
			events, err := managedProtocolEvents(wire)
			if err == nil {
				var projected bytes.Buffer
				for _, event := range events {
					if !r.appendProjection(&projected, clientresponse.Event{Name: event.Name, Data: event.Data}) {
						break
					}
				}
				if projected.Len() != 0 {
					output.Write(projected.Bytes())
					return
				}
			}
		}
	}
	body := failure.Body(policy)
	name := "error"
	switch policy.Protocol {
	case clientresponse.Chat:
		name = ""
	case clientresponse.Responses:
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(failure.ResourceError(policy.Protocol), &fields)
		fields["type"] = json.RawMessage(`"error"`)
		fields["sequence_number"], _ = json.Marshal(s.router.projectionSequence)
		body, _ = json.Marshal(fields)
	}
	clientresponse.AppendEvent(output, clientresponse.Event{Name: name, Data: body})
}

func managedRouterError(s *managedExecutionState, failure clientresponse.Failure) []byte {
	var output bytes.Buffer
	appendManagedRouterError(s, &output, failure)
	return output.Bytes()
}
