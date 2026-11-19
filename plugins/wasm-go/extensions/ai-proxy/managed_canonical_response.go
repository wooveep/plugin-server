package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/provider"
	"github.com/higress-group/wasm-go/pkg/ai/canonical"
	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/fixedtools"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

// managedCanonicalRouterStream holds per-attempt typed SSE decoding, public
// identity projection, and client SSE writing state on Router responses without
// intermediate Chat JSON/SSE conversion or HttpContext mutations.
type managedCanonicalRouterStream struct {
	target                provider.CanonicalTarget
	usageFormat           string
	failureFormat         string
	policy                clientresponse.Policy
	decoder               provider.CanonicalEventDecoder
	projection            *clientresponse.CanonicalStreamProjection
	writer                *provider.CanonicalStreamWriter
	observedUsage         clientresponse.Usage
	pendingChoiceFinishes []clientresponse.CanonicalEvent
}

func managedSupplierUsageFormat(c *managedExecutionConfig, target provider.CanonicalTarget) string {
	switch target.WireFormat {
	case provider.WireDashScopeText, provider.WireDashScopeMultimodal:
		return "qwen"
	case provider.WireGeminiGenerateContent:
		return "gemini"
	case provider.WireOpenAIResponses:
		return clientresponse.Responses
	case provider.WireAnthropicMessages:
		return clientresponse.Messages
	default:
		if target.Supplier == "moonshot" || (c != nil && c.vendor != nil && c.vendor.NativeResponseFormat() == "moonshot") {
			return "moonshot"
		}
		return clientresponse.Chat
	}
}

func managedFailureFormat(c *managedExecutionConfig, target provider.CanonicalTarget) string {
	if c != nil && c.vendor != nil && c.vendor.NativeResponseFormat() != "" {
		return c.vendor.NativeResponseFormat()
	}
	switch target.WireFormat {
	case provider.WireDashScopeText, provider.WireDashScopeMultimodal:
		return "qwen"
	case provider.WireGeminiGenerateContent:
		return "gemini"
	case provider.WireOpenAIResponses:
		return clientresponse.Responses
	case provider.WireAnthropicMessages:
		return clientresponse.Messages
	default:
		return clientresponse.Chat
	}
}

func managedCanonicalPolicy(s *managedExecutionState, upstreamModelHint string) clientresponse.Policy {
	policy := managedPublicPolicy(s)
	if !managedPublicText(s) {
		if upstreamModelHint != "" {
			policy.PublicModel = upstreamModelHint
		} else if policy.PublicModel == "" {
			policy.PublicModel = s.config.release.Descriptor.UpstreamModel
		}
		policy.IncludeUsage = true
		policy.Aliases = clientresponse.NewNativeResourceAliases()
	}
	return policy
}

func filterManagedAttemptRequest(s *managedExecutionState, req canonical.Request) canonical.Request {
	p := s.config.settings.ToolPolicy
	if p == nil {
		return req
	}
	effectiveRemoved := make(map[fixedtools.Tool]bool, len(s.removedTools)+len(p.Defaults))
	for tool := range s.removedTools {
		effectiveRemoved[tool] = true
	}
	for _, tool := range p.Defaults {
		if !s.requestedTools[tool] {
			effectiveRemoved[tool] = true
		}
	}
	return fixedtools.FilterCanonical(req, p.Defaults, effectiveRemoved)
}

func resolveManagedStreamTarget(ctx wrapper.HttpContext, s *managedExecutionState, streaming bool) provider.CanonicalTarget {
	target := resolveManagedCanonicalTarget(s.config, streaming)
	if !streaming || (target.WireFormat != provider.WireDashScopeText && target.WireFormat != provider.WireDashScopeMultimodal) {
		return target
	}
	if s.config.profile.InputProtocol == managedProtocolDashScopeText || s.config.profile.InputProtocol == managedProtocolDashScopeMultimodal {
		if gjson.GetBytes(s.publicRequest, "parameters.incremental_output").Bool() {
			target.IncrementalOutputMode = provider.IncrementalForceTrue
		} else {
			target.IncrementalOutputMode = provider.IncrementalForceFalse
		}
		return target
	}
	if s.baseCanonical != nil {
		attemptReq := filterManagedAttemptRequest(s, s.baseCanonical.CloneForAttempt())
		return provider.BindQwenIncrementalMode(attemptReq, target)
	}
	// Standalone stream benchmark or unit test harness that did not run request preparation.
	if ctx != nil {
		if inc, ok := ctx.GetContext("incrementalStreaming").(bool); ok {
			if inc {
				target.IncrementalOutputMode = provider.IncrementalForceTrue
			} else {
				target.IncrementalOutputMode = provider.IncrementalForceFalse
			}
			return target
		}
	}
	if len(s.publicRequest) > 0 {
		if parsed, err := provider.DecodeCanonicalRequest(canonical.Protocol(s.config.profile.InputProtocol), s.publicRequest); err == nil {
			parsed = filterManagedAttemptRequest(s, parsed)
			return provider.BindQwenIncrementalMode(parsed, target)
		}
	}
	target.IncrementalOutputMode = provider.IncrementalForceTrue
	return target
}

func convertAndProjectManagedCanonicalResponse(
	ctx wrapper.HttpContext,
	s *managedExecutionState,
	outcome executionfacts.AttemptOutcome,
	headers [][2]string,
	body []byte,
) (managedProtocolClientResponse, error) {
	if !outcome.BodyComplete || outcome.Acceptance != "accepted" ||
		(outcome.ExecutionResult != "succeeded" && outcome.ExecutionResult != "partial") ||
		outcome.ProviderError != "" {
		return managedProtocolClientResponse{}, errors.New("canonical supplier terminal response is not verified")
	}
	stream, err := managedProtocolContentType(headers)
	if err != nil {
		return managedProtocolClientResponse{}, err
	}
	if outcome.Usage != nil && outcome.Usage.Status == "reported" {
		if managedProtocolWireUsage(s.config.profile.InputProtocol, outcome.Usage) == nil {
			return managedProtocolClientResponse{}, errors.New("exact managed usage cannot be represented by client profile")
		}
	}

	target := resolveManagedStreamTarget(ctx, s, stream)
	usageFormat := managedSupplierUsageFormat(s.config, target)
	var usage clientresponse.Usage
	if outcome.Usage != nil && outcome.Usage.Status == "reported" {
		usage = clientresponse.ReadSupplierUsage(body, usageFormat, stream)
	}

	convertedHeaders := managedProtocolConvertedHeaders(headers, stream)

	if !stream {
		if target.WireFormat == provider.WireChatCompletions && !gjson.GetBytes(body, "choices.0.message").IsObject() {
			return managedProtocolClientResponse{}, errors.New("canonical chat response assistant message missing")
		}
		canonicalResp, err := provider.DecodeCanonicalResponse(target, body)
		if err != nil {
			return managedProtocolClientResponse{}, fmt.Errorf("decode canonical supplier response: %w", err)
		}
		policy := managedCanonicalPolicy(s, canonicalResp.Model)
		projectedResp, err := clientresponse.ProjectCanonical(policy, canonicalResp, usage)
		if err != nil {
			return managedProtocolClientResponse{}, fmt.Errorf("project canonical response: %w", err)
		}
		wireBody, err := provider.EncodeCanonicalClientResponse(projectedResp)
		if err != nil {
			return managedProtocolClientResponse{}, fmt.Errorf("encode canonical client response: %w", err)
		}
		if managedPublicText(s) {
			convertedHeaders = clientresponse.Headers(policy, convertedHeaders, false)
		}
		return managedProtocolClientResponse{Body: wireBody, Headers: convertedHeaders}, nil
	}

	events, err := managedProtocolEvents(body)
	if err != nil || len(events) == 0 {
		return managedProtocolClientResponse{}, errors.New("canonical buffered stream contains no complete events")
	}
	decoder, err := provider.NewCanonicalEventDecoder(target)
	if err != nil {
		return managedProtocolClientResponse{}, fmt.Errorf("new canonical event decoder: %w", err)
	}
	policy := managedCanonicalPolicy(s, "")
	projection := clientresponse.NewCanonicalStreamProjection(policy)
	writer, err := provider.NewCanonicalStreamWriter(canonical.Protocol(policy.Protocol))
	if err != nil {
		return managedProtocolClientResponse{}, fmt.Errorf("new canonical stream writer: %w", err)
	}

	var outBuf bytes.Buffer
	var pendingFinishes []clientresponse.CanonicalEvent
	for _, rawEv := range events {
		if err := decoder.Decode(rawEv.Name, rawEv.Data, func(ev canonical.Event) error {
			if ev.Kind == canonical.EventProtocolDone {
				return nil
			}
			if ev.Kind == canonical.EventProviderError {
				return errors.New("unexpected provider error event in verified stream")
			}
			projFrames, err := projection.Project(ev, usage)
			if err != nil {
				return err
			}
			for _, pf := range projFrames {
				if pf.Event.Kind == canonical.EventChoiceFinish {
					pendingFinishes = append(pendingFinishes, pf)
					continue
				}
				encoded, err := writer.Write(pf)
				if err != nil {
					return err
				}
				outBuf.Write(encoded)
			}
			return nil
		}); err != nil {
			return managedProtocolClientResponse{}, fmt.Errorf("decode buffered canonical stream event: %w", err)
		}
	}
	if err := decoder.End(); err != nil {
		return managedProtocolClientResponse{}, fmt.Errorf("end buffered canonical stream decoder: %w", err)
	}
	for _, pf := range pendingFinishes {
		encoded, err := writer.Write(pf)
		if err != nil {
			return managedProtocolClientResponse{}, fmt.Errorf("write buffered canonical choice finish: %w", err)
		}
		outBuf.Write(encoded)
	}
	finFrames, err := projection.Finish(canonical.FinishReason{}, usage)
	if err != nil {
		return managedProtocolClientResponse{}, fmt.Errorf("finish canonical stream projection: %w", err)
	}
	for _, ff := range finFrames {
		var encoded []byte
		if ff.Event.Kind == canonical.EventProtocolDone {
			encoded, err = writer.Finish(ff)
		} else {
			encoded, err = writer.Write(ff)
		}
		if err != nil {
			return managedProtocolClientResponse{}, fmt.Errorf("finish canonical stream writer: %w", err)
		}
		outBuf.Write(encoded)
	}
	if managedPublicText(s) {
		convertedHeaders = clientresponse.Headers(policy, convertedHeaders, true)
	}
	return managedProtocolClientResponse{Body: outBuf.Bytes(), Headers: convertedHeaders}, nil
}

func newManagedCanonicalRouterStream(ctx wrapper.HttpContext, s *managedExecutionState) (*managedCanonicalRouterStream, error) {
	target := resolveManagedStreamTarget(ctx, s, true)
	policy := managedCanonicalPolicy(s, "")
	decoder, decErr := provider.NewCanonicalEventDecoder(target)
	projection := clientresponse.NewCanonicalStreamProjection(policy)
	writer, wrtErr := provider.NewCanonicalStreamWriter(canonical.Protocol(policy.Protocol))
	rc := &managedCanonicalRouterStream{
		target:        target,
		usageFormat:   managedSupplierUsageFormat(s.config, target),
		failureFormat: managedFailureFormat(s.config, target),
		policy:        policy,
		decoder:       decoder,
		projection:    projection,
		writer:        writer,
	}
	if decErr != nil {
		return rc, fmt.Errorf("new router canonical event decoder: %w", decErr)
	}
	if wrtErr != nil {
		return rc, fmt.Errorf("new router canonical stream writer: %w", wrtErr)
	}
	return rc, nil
}

func appendManagedCanonicalStreamFailure(
	s *managedExecutionState,
	rc *managedCanonicalRouterStream,
	output *bytes.Buffer,
	failure clientresponse.Failure,
	usage clientresponse.Usage,
) {
	if rc != nil && rc.projection != nil && rc.writer != nil {
		if failFrames, err := rc.projection.Fail(failure, usage); err == nil {
			var encoded bytes.Buffer
			writeOK := true
			for _, ff := range failFrames {
				chunk, wErr := rc.writer.Fail(ff)
				if wErr != nil {
					writeOK = false
					break
				}
				encoded.Write(chunk)
			}
			if writeOK && encoded.Len() > 0 {
				output.Write(encoded.Bytes())
				return
			}
		}
	}
	appendManagedRouterError(s, output, failure)
}

func projectManagedCanonicalRouterChunk(ctx wrapper.HttpContext, s *managedExecutionState, chunk []byte, end bool) []byte {
	r := s.router
	rc := r.canonical
	var output bytes.Buffer

	onComment := func() {
		if !r.projectionFailed && !r.projectedError {
			output.WriteString(": keep-alive\n\n")
		}
	}

	onEvent := func(event managedProtocolEvent) {
		if r.projectionFailed || r.projectedError || rc == nil || rc.decoder == nil || rc.writer == nil {
			return
		}
		if r.chat != nil && (r.chat.invalid || r.chat.usageConflict || r.chat.modelConflict) {
			r.projectionFailed = true
			return
		}
		if r.supplier != nil && (r.supplier.invalid || r.supplier.usageConflict || r.supplier.modelConflict) {
			r.projectionFailed = true
			return
		}
		if r.dashScope != nil && r.dashScope.framingError {
			r.projectionFailed = true
			return
		}

		if r.supplier != nil && r.supplier.hasPublicUsage {
			rc.observedUsage = r.supplier.publicUsage
		} else if !bytes.Equal(bytes.TrimSpace(event.Data), []byte("[DONE]")) {
			if parsedUsage := clientresponse.ReadSupplierUsage(event.Data, rc.usageFormat, false); parsedUsage.Input != nil {
				rc.observedUsage = parsedUsage
			}
		}

		isError := event.Name == "error" ||
			(r.chat != nil && r.chat.providerError) ||
			(r.supplier != nil && r.supplier.failed) ||
			(r.dashScope != nil && r.dashScope.providerError)
		if !isError && !bytes.Equal(bytes.TrimSpace(event.Data), []byte("[DONE]")) {
			parsed := gjson.ParseBytes(event.Data)
			if parsed.Get("type").String() == "error" || !managedProtocolNull(json.RawMessage(parsed.Get("error").Raw)) {
				isError = true
			}
		}

		if isError {
			var failure clientresponse.Failure
			if rc.target.WireFormat == provider.WireGeminiGenerateContent {
				if br := gjson.GetBytes(event.Data, "promptFeedback.blockReason").String(); br != "" && br != "BLOCK_REASON_UNSPECIFIED" {
					failure = clientresponse.Failure{Kind: clientresponse.FailurePolicy}
				}
			}
			if failure.Kind == "" {
				failure = clientresponse.ClassifyFailure(clientresponse.FailureInput{
					Format:  rc.failureFormat,
					Client:  rc.policy.Protocol,
					Body:    event.Data,
					Request: s.publicRequest,
				})
			}
			appendManagedCanonicalStreamFailure(s, rc, &output, failure, rc.observedUsage)
			r.projectedError = true
			return
		}

		decErr := rc.decoder.Decode(event.Name, event.Data, func(ev canonical.Event) error {
			if ev.Kind == canonical.EventProtocolDone {
				return nil
			}
			if ev.Kind == canonical.EventProviderError {
				errPayload := ev.RawError
				if len(errPayload) == 0 {
					errPayload = event.Data
				}
				failure := clientresponse.ClassifyFailure(clientresponse.FailureInput{
					Format:  rc.failureFormat,
					Client:  rc.policy.Protocol,
					Body:    errPayload,
					Request: s.publicRequest,
				})
				appendManagedCanonicalStreamFailure(s, rc, &output, failure, rc.observedUsage)
				r.projectedError = true
				return nil
			}
			projFrames, pErr := rc.projection.Project(ev, clientresponse.Usage{})
			if pErr != nil {
				return pErr
			}
			for _, pf := range projFrames {
				if pf.Event.Kind == canonical.EventChoiceFinish {
					rc.pendingChoiceFinishes = append(rc.pendingChoiceFinishes, pf)
					continue
				}
				encoded, wErr := rc.writer.Write(pf)
				if wErr != nil {
					return wErr
				}
				output.Write(encoded)
			}
			return nil
		})
		if decErr != nil {
			r.projectionFailed = true
		}
	}

	if r.toolProtocol != "" {
		baseOnEvent := onEvent
		onEvent = func(event managedProtocolEvent) {
			r.toolUsage = managedToolEventUsage(r.toolProtocol, event.Data, r.toolUsage)
			baseOnEvent(event)
		}
	}

	switch {
	case r.supplier != nil:
		r.supplier.pushRaw(chunk, onEvent, onComment)
	case r.chat != nil:
		r.chat.decoder.onComment = onComment
		r.chat.pushEvents(chunk, onEvent)
		r.chat.decoder.onComment = nil
	case r.dashScope != nil:
		r.dashScope.decoder.onComment = onComment
		r.dashScope.pushEvents(chunk, onEvent)
		r.dashScope.decoder.onComment = nil
	}

	if !end || r.projectedError {
		return output.Bytes()
	}

	if rc == nil || rc.decoder == nil || rc.writer == nil {
		r.projectionFailed = true
	} else if err := rc.decoder.End(); err != nil {
		r.projectionFailed = true
	}

	observed := r.streamObservation()
	finalUsage := rc.observedUsage
	if observed.Usage == nil || observed.Usage.Status != "reported" {
		finalUsage = clientresponse.Usage{}
	} else if r.supplier != nil && r.supplier.hasPublicUsage {
		finalUsage = r.supplier.publicUsage
	}

	if !r.projectionFailed && observed.BodyComplete && !observed.ProviderError &&
		(observed.ExecutionResult == "succeeded" || observed.ExecutionResult == "partial") {
		if rc.policy.Protocol != clientresponse.Chat {
			for _, pf := range rc.pendingChoiceFinishes {
				fin := pf.Finish
				if fin.Category == canonical.FinishNone {
					fin = pf.Event.Finish
				}
				switch fin.Category {
				case canonical.FinishAborted:
					appendManagedCanonicalStreamFailure(s, rc, &output, clientresponse.Failure{Kind: clientresponse.FailureInterrupted}, finalUsage)
					r.projectedError = true
					return output.Bytes()
				case canonical.FinishOverloaded:
					appendManagedCanonicalStreamFailure(s, rc, &output, clientresponse.Failure{Kind: clientresponse.FailureOverloaded}, finalUsage)
					r.projectedError = true
					return output.Bytes()
				case canonical.FinishUnknown:
					appendManagedCanonicalStreamFailure(s, rc, &output, clientresponse.Failure{Kind: clientresponse.FailureInvalid}, finalUsage)
					r.projectedError = true
					return output.Bytes()
				}
			}
		}

		for _, pf := range rc.pendingChoiceFinishes {
			encoded, wErr := rc.writer.Write(pf)
			if wErr != nil {
				r.projectionFailed = true
				break
			}
			output.Write(encoded)
		}
		if !r.projectionFailed {
			finFrames, fErr := rc.projection.Finish(canonical.FinishReason{}, finalUsage)
			if fErr != nil {
				r.projectionFailed = true
			} else {
				for _, ff := range finFrames {
					var encoded []byte
					var wErr error
					if ff.Event.Kind == canonical.EventProtocolDone {
						encoded, wErr = rc.writer.Finish(ff)
					} else {
						encoded, wErr = rc.writer.Write(ff)
					}
					if wErr != nil {
						r.projectionFailed = true
						break
					}
					output.Write(encoded)
				}
			}
		}
	} else {
		r.projectionFailed = true
	}

	if r.projectionFailed {
		appendManagedCanonicalStreamFailure(s, rc, &output, clientresponse.Failure{Kind: clientresponse.FailureIncomplete}, clientresponse.Usage{})
	}
	return output.Bytes()
}

func encodeManagedCanonicalPartialResponse(
	ctx wrapper.HttpContext,
	s *managedExecutionState,
	headers [][2]string,
	original []byte,
) (managedProtocolClientResponse, uint32, bool, error) {
	if !managedPublicText(s) {
		return managedProtocolClientResponse{}, 0, false, nil
	}
	stream, err := managedProtocolContentType(headers)
	if err != nil {
		return managedProtocolClientResponse{}, 0, false, nil
	}

	target := resolveManagedStreamTarget(ctx, s, stream)
	policy := managedPublicPolicy(s)
	failureFormat := managedFailureFormat(s.config, target)
	usageFormat := managedSupplierUsageFormat(s.config, target)

	if !stream {
		if failureFormat != clientresponse.Responses &&
			(gjson.GetBytes(original, "error").IsObject() ||
				(gjson.GetBytes(original, "code").Exists() && gjson.GetBytes(original, "message").Exists() && !gjson.GetBytes(original, "output").Exists())) {
			f := clientresponse.ClassifyFailure(clientresponse.FailureInput{
				Status:  200,
				Format:  failureFormat,
				Client:  policy.Protocol,
				Body:    original,
				Request: s.publicRequest,
			})
			return managedProtocolClientResponse{
				Body:    f.Body(policy),
				Headers: f.Headers(policy, headers, false, false),
			}, uint32(f.Status(policy.Protocol)), true, nil
		}
		if target.WireFormat == provider.WireGeminiGenerateContent {
			if br := gjson.GetBytes(original, "promptFeedback.blockReason").String(); br != "" && br != "BLOCK_REASON_UNSPECIFIED" {
				f := clientresponse.Failure{Kind: clientresponse.FailurePolicy}
				return managedProtocolClientResponse{
					Body:    f.Body(policy),
					Headers: f.Headers(policy, headers, false, false),
				}, uint32(f.Status(policy.Protocol)), true, nil
			}
		}
		if target.WireFormat != provider.WireChatCompletions {
			return managedProtocolClientResponse{}, 0, false, nil
		}

		finish := gjson.GetBytes(original, "choices.0.finish_reason").String()
		failure := clientresponse.Failure{Kind: clientresponse.FailureIncomplete}
		switch finish {
		case "aborted":
			failure.Kind = clientresponse.FailureInterrupted
		case "insufficient_system_resource":
			failure.Kind = clientresponse.FailureOverloaded
		case "stop", "length", "content_filter", "tool_calls", "function_call":
			return managedProtocolClientResponse{}, 0, false, nil
		default:
			if !gjson.GetBytes(original, "choices.0.message").IsObject() {
				return managedProtocolClientResponse{}, 0, false, nil
			}
			failure.Kind = clientresponse.FailureInvalid
		}
		if policy.Protocol == clientresponse.Chat && (finish == "aborted" || finish == "insufficient_system_resource") {
			return managedProtocolClientResponse{}, 0, false, nil
		}

		canonicalResp, decErr := provider.DecodeCanonicalResponse(target, original)
		if decErr != nil {
			return managedProtocolClientResponse{}, 0, true, fmt.Errorf("decode partial canonical response: %w", decErr)
		}
		usage := clientresponse.ReadSupplierUsage(original, usageFormat, false)

		if policy.Protocol == clientresponse.Responses {
			projResp, pErr := clientresponse.ProjectCanonical(policy, canonicalResp, usage)
			if pErr != nil {
				return managedProtocolClientResponse{}, 0, true, pErr
			}
			projResp.Error = failure.ResourceError(policy.Protocol)
			encoded, eErr := provider.EncodeCanonicalClientResponse(projResp)
			if eErr != nil {
				return managedProtocolClientResponse{}, 0, true, eErr
			}
			return managedProtocolClientResponse{
				Body:    encoded,
				Headers: failure.Headers(policy, headers, false, true),
			}, 200, true, nil
		}

		if policy.Protocol == clientresponse.Messages {
			for i := range canonicalResp.Choices {
				canonicalResp.Choices[i].Finish = canonical.FinishReason{}
			}
		}
		projResp, pErr := clientresponse.ProjectCanonical(policy, canonicalResp, usage)
		if pErr != nil {
			return managedProtocolClientResponse{}, 0, true, pErr
		}
		partialBody, eErr := provider.EncodeCanonicalClientResponse(projResp)
		if eErr != nil {
			return managedProtocolClientResponse{}, 0, true, eErr
		}
		s.publicPartial = partialBody
		return managedProtocolClientResponse{
			Body:    failure.Body(policy),
			Headers: failure.Headers(policy, headers, false, true),
		}, uint32(failure.Status(policy.Protocol)), true, nil
	}

	normalized := bytes.ReplaceAll(original, []byte("\r\n"), []byte("\n"))
	last := bytes.LastIndex(normalized, []byte("\n\n"))
	if last < 0 {
		return managedProtocolClientResponse{}, 0, false, nil
	}
	events, evErr := clientresponse.Events(normalized[:last+2])
	if evErr != nil {
		if target.WireFormat == provider.WireChatCompletions {
			return managedProtocolClientResponse{}, 0, true, evErr
		}
		return managedProtocolClientResponse{}, 0, false, nil
	}

	failure := clientresponse.Failure{Kind: clientresponse.FailureIncomplete}
	var prefixEvents []clientresponse.Event
	terminal, done, failed := false, false, false

	for _, event := range events {
		if bytes.Equal(bytes.TrimSpace(event.Data), []byte("[DONE]")) {
			done = true
			continue
		}
		object := gjson.ParseBytes(event.Data)
		if target.WireFormat == provider.WireGeminiGenerateContent {
			if br := object.Get("promptFeedback.blockReason").String(); br != "" && br != "BLOCK_REASON_UNSPECIFIED" {
				f := clientresponse.Failure{Kind: clientresponse.FailurePolicy}
				return managedProtocolClientResponse{
					Body:    f.Body(policy),
					Headers: f.Headers(policy, headers, false, false),
				}, uint32(f.Status(policy.Protocol)), true, nil
			}
		}

		isErrFrame := event.Name == "error" ||
			object.Get("error").IsObject() ||
			object.Get("type").String() == "error" ||
			(target.WireFormat != provider.WireChatCompletions &&
				object.Get("code").Exists() &&
				object.Get("message").Exists() &&
				!object.Get("output").Exists())
		if isErrFrame {
			failure = clientresponse.ClassifyFailure(clientresponse.FailureInput{
				Format:  failureFormat,
				Client:  policy.Protocol,
				Body:    event.Data,
				Request: s.publicRequest,
			})
			failed = true
			break
		}

		switch target.WireFormat {
		case provider.WireGeminiGenerateContent:
			if fin := object.Get("candidates.0.finishReason").String(); fin != "" && fin != "null" {
				terminal = true
			}
		case provider.WireDashScopeText, provider.WireDashScopeMultimodal:
			fin := object.Get("output.choices.0.finish_reason").String()
			if fin == "" {
				fin = object.Get("output.finish_reason").String()
			}
			if fin != "" && fin != "null" {
				terminal = true
				switch fin {
				case "stop", "length", "tool_calls", "function_call", "content_filter":
				case "aborted":
					failure.Kind, failed = clientresponse.FailureInterrupted, true
				case "insufficient_system_resource":
					failure.Kind, failed = clientresponse.FailureOverloaded, true
				default:
					failure.Kind, failed = clientresponse.FailureInvalid, true
				}
			}
		default:
			for _, choice := range object.Get("choices").Array() {
				if fin := choice.Get("finish_reason").String(); fin != "" {
					terminal = true
					switch fin {
					case "stop", "length", "tool_calls", "function_call", "content_filter":
					case "aborted":
						failure.Kind, failed = clientresponse.FailureInterrupted, true
					case "insufficient_system_resource":
						failure.Kind, failed = clientresponse.FailureOverloaded, true
					default:
						failure.Kind, failed = clientresponse.FailureInvalid, true
					}
				}
			}
		}

		prefixEvents = append(prefixEvents, event)
	}

	requiresDone := target.WireFormat == provider.WireChatCompletions
	if terminal && (!requiresDone || done) && !failed && last+2 == len(normalized) {
		return managedProtocolClientResponse{}, 0, false, nil
	}
	if len(prefixEvents) == 0 {
		return managedProtocolClientResponse{
			Body:    failure.Body(policy),
			Headers: failure.Headers(policy, headers, false, false),
		}, uint32(failure.Status(policy.Protocol)), true, nil
	}

	decoder, decErr := provider.NewCanonicalEventDecoder(target)
	if decErr != nil {
		return managedProtocolClientResponse{}, 0, true, fmt.Errorf("new partial canonical event decoder: %w", decErr)
	}
	projection := clientresponse.NewCanonicalStreamProjection(policy)
	writer, wrtErr := provider.NewCanonicalStreamWriter(canonical.Protocol(policy.Protocol))
	if wrtErr != nil {
		return managedProtocolClientResponse{}, 0, true, fmt.Errorf("new partial canonical stream writer: %w", wrtErr)
	}

	partialUsage := clientresponse.ReadSupplierUsage(original, usageFormat, true)
	var outBuf bytes.Buffer
	for _, ev := range prefixEvents {
		if dErr := decoder.Decode(ev.Name, ev.Data, func(cEv canonical.Event) error {
			// Never close partial text or tool items based on an unverified or failed terminal.
			if cEv.Kind == canonical.EventChoiceFinish || cEv.Kind == canonical.EventProtocolDone || cEv.Kind == canonical.EventProviderError {
				return nil
			}
			projFrames, pErr := projection.Project(cEv, partialUsage)
			if pErr != nil {
				return pErr
			}
			for _, pf := range projFrames {
				encoded, wErr := writer.Write(pf)
				if wErr != nil {
					return wErr
				}
				outBuf.Write(encoded)
			}
			return nil
		}); dErr != nil {
			return managedProtocolClientResponse{}, 0, true, fmt.Errorf("decode partial canonical stream event: %w", dErr)
		}
	}

	failFrames, fErr := projection.Fail(failure, partialUsage)
	if fErr != nil {
		return managedProtocolClientResponse{}, 0, true, fErr
	}
	for _, ff := range failFrames {
		encoded, wErr := writer.Fail(ff)
		if wErr != nil {
			return managedProtocolClientResponse{}, 0, true, wErr
		}
		outBuf.Write(encoded)
	}
	return managedProtocolClientResponse{
		Body:    outBuf.Bytes(),
		Headers: failure.Headers(policy, headers, true, true),
	}, 200, true, nil
}
