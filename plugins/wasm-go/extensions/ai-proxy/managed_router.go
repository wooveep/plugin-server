package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash"
	"strconv"
	"strings"
	"time"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/provider"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/observedusage"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

type managedRouterResponse struct {
	headers            [][2]string
	status             *uint16
	chat               *managedChatStream
	native             *managedNativeStream
	dashScope          *managedDashScopeStream
	supplier           *managedRouterSupplierStream
	toolProtocol       string
	toolUsage          gjson.Result
	digest             hash.Hash
	jsonOutcome        *executionfacts.AttemptOutcome
	attemptRecorded    bool
	transportEnded     bool
	ended              bool
	injectingTerminal  bool
	projection         *clientresponse.StreamProjection
	projectionUsage    clientresponse.Usage
	projectionFailed   bool
	projectedError     bool
	projectionSequence int64
	responsesConverter *provider.ChatCompletionToResponsesStreamConverter
	messagesConverter  *provider.ClaudeToOpenAIConverter
	conversionFinish   []byte
	conversionUsage    []byte
	terminalEvent      clientresponse.Event
	heldRaw            []byte
	rawAliasesInvalid  bool
}

func (r *managedRouterResponse) pushStream(chunk []byte, after func(managedProtocolEvent), comment func(), end bool) {
	if r.toolProtocol != "" {
		forward := after
		after = func(event managedProtocolEvent) {
			r.toolUsage = managedToolEventUsage(r.toolProtocol, event.Data, r.toolUsage)
			if forward != nil {
				forward(event)
			}
		}
	}
	switch {
	case r.supplier != nil:
		r.supplier.push(chunk, end, func(converted []byte) { r.chat.pushEvents(converted, after) }, comment)
	case r.chat != nil:
		r.chat.decoder.onComment = comment
		r.chat.pushEvents(chunk, after)
		r.chat.decoder.onComment = nil
	case r.dashScope != nil:
		r.dashScope.decoder.onComment = comment
		r.dashScope.pushEvents(chunk, after)
		r.dashScope.decoder.onComment = nil
	default:
		r.native.decoder.onComment = comment
		r.native.push(chunk, after)
		r.native.decoder.onComment = nil
	}
}

func (r *managedRouterResponse) streamObservation() managedProtocolObservation {
	switch {
	case r.supplier != nil:
		return r.supplier.observation()
	case r.chat != nil:
		return r.chat.observation()
	case r.dashScope != nil:
		return r.dashScope.observation(*r.status, r.headers)
	default:
		return r.native.observation()
	}
}

// The route already owns its immutable cluster. Only the validated outbound
// request is installed before the paused decoder is returned to Envoy Router.
func dispatchManagedRouter(ctx wrapper.HttpContext, s *managedExecutionState, request managedProtocolRequest) {
	if previous, _ := ctx.GetContext(managedExecutionContextKey).(*managedExecutionState); previous != nil && previous != s && previous.router != nil {
		if err := queueManagedRouterContinuation(ctx, previous, s, request); err != nil {
			rejectManagedPreparation(ctx, s, 503, "router_continuation_unavailable")
		}
		return
	}
	cluster, err := proxywasm.GetProperty([]string{"cluster_name"})
	if err != nil || string(cluster) != s.config.release.Descriptor.ClusterRef {
		if s.router == nil && s.config.settings.RouterReentry && s.sourceConfig != nil &&
			s.sourceConfig.release.Descriptor.ReleaseID == s.config.release.Descriptor.ReleaseID &&
			s.sourceConfig.release.Descriptor.TargetID != s.config.release.Descriptor.TargetID {
			if err := queueManagedRouterContinuation(ctx, s, s, request); err != nil {
				rejectManagedPreparation(ctx, s, 503, "router_continuation_unavailable")
			}
			return
		}
		rejectManagedPreparation(ctx, s, 503, "router_target_not_verified")
		return
	}
	path := s.config.settings.UpstreamPath
	if s.config.vendor != nil {
		path = request.UpstreamPath
	}
	scheme := "http"
	if s.config.release.Descriptor.Transport.SNI != "" {
		scheme = "https"
	}
	headers := [][2]string{
		{":method", "POST"}, {":path", path}, {":authority", s.config.settings.Authority},
		{":scheme", scheme}, {"content-length", strconv.Itoa(len(request.Body))},
		{managedCallHeader, s.callID}, {managedAttemptHeader, s.attemptID},
	}
	// Envoy requires X-Forwarded-Proto when it resolves an internal redirect.
	// Reconstruct it from the original normalized downstream scheme.
	forwardedProto := ctx.Scheme()
	if forwardedProto == "" {
		forwardedProto = scheme
	}
	headers = append(headers, [2]string{"x-forwarded-proto", forwardedProto})
	for _, header := range request.Headers {
		name := strings.ToLower(header[0])
		if name != "accept-encoding" && !strings.HasPrefix(name, "x-envoy-") {
			headers = append(headers, header)
		}
	}
	// The observer consumes original SSE bytes; downstream compression belongs
	// to Envoy's response filter after observation, never to the supplier codec.
	headers = append(headers, [2]string{"accept-encoding", "identity"})
	// Rebuilt headers exclude caller-owned Envoy controls. The legacy callout
	// default must not impose a total deadline on an active Router stream.
	var timeout uint32
	if s.config.settings.RouterTimeoutMS != nil {
		timeout = *s.config.settings.RouterTimeoutMS
	}
	if s.routerDeadline.IsZero() && timeout != 0 && s.parentAttemptID == "" {
		s.routerDeadline = time.Now().Add(time.Duration(timeout) * time.Millisecond)
	}
	if !s.routerDeadline.IsZero() {
		remaining := time.Until(s.routerDeadline)
		if remaining <= 0 {
			rejectManagedPreparation(ctx, s, 504, "execution_budget_expired")
			return
		}
		remainingMS := (remaining + time.Millisecond - 1) / time.Millisecond
		if timeout == 0 || uint64(remainingMS) < uint64(timeout) {
			timeout = uint32(remainingMS)
		}
	}
	headers = append(headers, [2]string{"x-envoy-upstream-rq-timeout-ms", strconv.FormatUint(uint64(timeout), 10)})
	if proxywasm.ReplaceHttpRequestBody(request.Body) != nil ||
		proxywasm.ReplaceHttpRequestHeaders(headers) != nil ||
		proxywasm.SetProperty([]string{observedusage.RouterDeliveryProperty}, []byte(s.callID)) != nil {
		rejectManagedPreparation(ctx, s, 500, "router_request_install_failed")
		return
	}
	s.router = &managedRouterResponse{}
	s.dispatchStarted = true
	s.sendStartedAt = time.Now()
	markManagedTextAttempt(s)
	recordManagedAccessAttempt(ctx, s)
	ctx.SetUserAttribute("managed_execution_transport", "router")
	ctx.SetUserAttribute("managed_execution_response_mode", "incremental")
	if err := proxywasm.ResumeHttpRequest(); err != nil {
		s.router = nil
		rejectManagedPreparation(ctx, s, 502, "router_resume_failed")
		return
	}
	s.handedOff = true
}

func observeManagedRouterHeaders(ctx wrapper.HttpContext, s *managedExecutionState) types.Action {
	// This internal bridge belongs to the gateway. It can never come from a
	// provider response, even when the rest of that response is transparent.
	_ = proxywasm.RemoveHttpResponseHeader(executionfacts.DeliveryBridgeHeader)
	_ = proxywasm.RemoveHttpResponseHeader(managedRouterRedirectHeader)
	if !wrapper.IsResponseFromUpstream() {
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}
	cluster, err := proxywasm.GetProperty([]string{"cluster_name"})
	if err != nil || string(cluster) != s.config.release.Descriptor.ClusterRef {
		managedFactGap(ctx, "router_observed_target_mismatch")
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}
	headers, err := proxywasm.GetHttpResponseHeaders()
	statusText, statusErr := managedSingleHeader(headers, ":status")
	status, parseErr := strconv.ParseUint(statusText, 10, 16)
	if err != nil || statusErr != nil || parseErr != nil || status < 100 || status > 599 {
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}
	r := s.router
	code := uint16(status)
	r.headers, r.status = headers, &code
	captureManagedUpstreamID(s, headers)
	if !ctx.HasResponseBody() {
		r.transportEnded = true
		finishManagedRouter(ctx, s, true)
		return types.ActionContinue
	}
	if code >= 400 && s.config.settings.TextExecution != nil && s.config.settings.RouterReentry {
		ctx.BufferResponseBody()
		ctx.SetResponseBodyBufferLimit(defaultMaxBodyBytes)
		return types.HeaderStopIteration
	}
	stream, err := managedProtocolContentType(headers)
	if err == nil && stream {
		profile := s.config.profile
		if s.config.settings.RouterToolUsage && len(s.requestedTools) > 0 {
			r.toolProtocol = profile.UpstreamProtocol
		}
		switch profile.UpstreamProtocol {
		case managedProtocolChat:
			r.chat = &managedChatStream{collectText: true}
		case managedProtocolCompletions:
			r.chat = &managedChatStream{completions: true}
		case managedProtocolDashScopeText, managedProtocolDashScopeMultimodal:
			r.dashScope = &managedDashScopeStream{
				protocol:    profile.UpstreamProtocol,
				incremental: gjson.GetBytes(s.publicRequest, "parameters.incremental_output").Bool(),
			}
		default:
			r.native = newManagedNativeStream(profile.UpstreamProtocol, profile.supplier == "qwen", true)
		}
		if s.config.vendor != nil && s.config.vendor.NativeResponseFormat() != "" {
			r.chat = &managedChatStream{collectText: true}
			r.supplier = &managedRouterSupplierStream{context: ctx, state: s, format: s.config.vendor.NativeResponseFormat(), finished: make(map[string]string)}
		}
		r.digest = sha256.New()
		if s.config.settings.StreamProjection {
			if profile.InputProtocol != profile.UpstreamProtocol {
				switch profile.InputProtocol {
				case managedProtocolResponses:
					r.responsesConverter = provider.NewChatCompletionToResponsesStreamConverter()
				case managedProtocolMessages:
					r.messagesConverter = &provider.ClaudeToOpenAIConverter{}
				}
			}
			policy := managedPublicPolicy(s)
			r.projection = clientresponse.NewStreamProjection(policy)
			headers := clientresponse.Headers(policy, headers, true)
			headers = append([][2]string{{":status", statusText}}, headers...)
			if proxywasm.ReplaceHttpResponseHeaders(headers) != nil {
				r.projectionFailed = true
				managedFactGap(ctx, "router_public_headers_unavailable")
			}
		} else if managedNativeResponses(s.config) {
			s.publicAliases = clientresponse.NewNativeResourceAliases()
		}
		return types.ActionContinue
	}
	ctx.BufferResponseBody()
	ctx.SetResponseBodyBufferLimit(defaultMaxBodyBytes)
	if s.config.settings.ProtocolMode == "auto" || s.config.profile.InputProtocol == clientresponse.Responses {
		return types.HeaderStopIteration
	}
	return types.ActionContinue
}

func observeManagedRouterChunk(ctx wrapper.HttpContext, s *managedExecutionState, chunk []byte, end bool) []byte {
	r := s.router
	if r.digest == nil {
		if end {
			r.transportEnded = true
			finishManagedRouter(ctx, s, true)
		}
		return chunk
	}
	_, _ = r.digest.Write(chunk)
	tracksResources := s.config.profile.InputProtocol == clientresponse.Responses
	if r.projection != nil {
		chunk = projectManagedRouterChunk(ctx, s, chunk, end)
	} else if tracksResources {
		r.pushStream(chunk, func(event managedProtocolEvent) {
			if !r.rawAliasesInvalid {
				err := s.publicAliases.ObserveEvent(clientresponse.Responses, clientresponse.Event{Name: event.Name, Data: event.Data})
				r.rawAliasesInvalid = err != nil
			}
		}, nil, end)
		if r.native.terminal {
			if r.native.invalid || r.rawAliasesInvalid || len(r.heldRaw)+len(chunk) > int(defaultMaxBodyBytes) {
				chunk = append(r.heldRaw, chunk...)
				r.heldRaw = nil
				r.rawAliasesInvalid = true
			} else {
				// Keep the provider's terminal bytes intact until the native
				// reference can be used. Earlier chunks pass through unchanged.
				r.heldRaw = append(r.heldRaw, chunk...)
				chunk = nil
			}
		}
	} else if r.supplier != nil && s.config.settings.ProtocolMode == "auto" {
		var converted []byte
		r.supplier.push(chunk, end, func(value []byte) {
			r.chat.pushEvents(value, nil)
			converted = append(converted, value...)
		}, nil)
		chunk = converted
	} else {
		r.pushStream(chunk, nil, nil, end)
	}
	if !end {
		return chunk
	}
	r.transportEnded = true
	if len(r.heldRaw) != 0 {
		chunk, r.heldRaw = r.heldRaw, nil
	}
	if tracksResources && !r.rawAliasesInvalid && !r.projectionFailed && r.streamObservation().BodyComplete {
		// Only the final delivery waits on the mapping. The upstream fact
		// already has terminal evidence, even if the client cancels.
		pending, err := saveManagedRouterResources(ctx, s, func(err error) {
			final := managedRouterResourceTail(ctx, s, chunk, err)
			// Envoy may call stream-done synchronously while injecting end-of-stream.
			// The injection result owns delivery finalization during that callback.
			r.injectingTerminal = true
			injectErr := proxywasm.InjectEncodedDataToFilterChain(final, true)
			r.injectingTerminal = false
			if injectErr != nil {
				r.projectionFailed = true
				managedFactGap(ctx, "router_public_response_unavailable")
			}
			finishManagedRouter(ctx, s, true)
		})
		if pending {
			ctx.NeedPauseStreamingResponse()
			return nil
		}
		chunk = managedRouterResourceTail(ctx, s, chunk, err)
	}
	finishManagedRouter(ctx, s, true)
	return chunk
}

func managedRouterResourceTail(ctx wrapper.HttpContext, s *managedExecutionState, tail []byte, err error) []byte {
	if err == nil {
		return tail
	}
	s.router.projectionFailed = true
	managedFactGap(ctx, "resource_reference_write_failed")
	if s.router.projection == nil {
		return nil
	}
	return managedRouterError(s, clientresponse.Failure{Kind: clientresponse.FailureConfiguration})
}

func saveManagedRouterResources(ctx wrapper.HttpContext, s *managedExecutionState, done func(error)) (bool, error) {
	// Save can fail before returning. Only an outstanding callback
	// needs to re-enter the encoder filter chain.
	var synchronousError error
	var completed, dispatched bool
	saving := persistManagedResourceAliases(ctx, s, func(err error) {
		if !dispatched {
			completed, synchronousError = true, err
			return
		}
		done(err)
	})
	dispatched = true
	return saving && !completed, synchronousError
}

func observeManagedRouterBody(ctx wrapper.HttpContext, s *managedExecutionState, body []byte) types.Action {
	r := s.router
	r.transportEnded = true
	if r.status != nil {
		outcome := managedResponseOutcome(s, r.headers, nil, body, nil)
		r.jsonOutcome = &outcome
		if managedRouterRetryOutcome(s, outcome) {
			recordManagedAttempt(ctx, s, outcome)
			r.attemptRecorded = true
			ctx.SetContext(managedRouterBodyActiveKey, true)
			var returned, finishRequested bool
			finish := func() {
				if !returned {
					finishRequested = true
					return
				}
				if finishManagedRouterJSON(ctx, s, body) == types.ActionContinue {
					_ = proxywasm.ResumeHttpResponse()
				}
			}
			continuing := continueManagedText(ctx, s, outcome, r.headers, body, finish)
			returned = true
			ctx.SetContext(managedRouterBodyActiveKey, false)
			if continuing && !finishRequested {
				if ctx.GetBoolContext(managedRouterRedirectKey, false) {
					return types.ActionContinue
				}
				return types.ActionPause
			}
		}
	}
	return finishManagedRouterJSON(ctx, s, body)
}

func finishManagedRouterJSON(ctx wrapper.HttpContext, s *managedExecutionState, body []byte) types.Action {
	r := s.router
	if r.jsonOutcome != nil {
		outcome := *r.jsonOutcome
		if s.config.settings.ProtocolMode == "auto" {
			r.projectionFailed = !projectManagedRouterJSON(ctx, s, body, outcome)
		} else if managedNativeResponses(s.config) && *r.status >= 200 && *r.status < 300 {
			aliases, err := clientresponse.ObserveNativeResources(clientresponse.Responses, body, false)
			if err == nil {
				s.publicAliases = aliases
			}
		}
		if s.config.profile.InputProtocol == clientresponse.Responses && !r.projectionFailed && *r.status >= 200 && *r.status < 300 {
			handleError := func(err error) {
				if err != nil {
					r.projectionFailed = true
					managedFactGap(ctx, "resource_reference_write_failed")
					replaceManagedRouterFailure(ctx, s, clientresponse.Failure{Kind: clientresponse.FailureConfiguration})
				}
			}
			pending, err := saveManagedRouterResources(ctx, s, func(err error) {
				handleError(err)
				finishManagedRouter(ctx, s, true)
				_ = proxywasm.ResumeHttpResponse()
			})
			if pending {
				return types.ActionPause
			}
			handleError(err)
		}
	}
	finishManagedRouter(ctx, s, true)
	return types.ActionContinue
}

func finishManagedRouter(ctx wrapper.HttpContext, s *managedExecutionState, end bool) {
	r := s.router
	if r.ended {
		return
	}
	r.ended = true
	var outcome executionfacts.AttemptOutcome
	if r.digest != nil {
		observed := r.streamObservation()
		outcome = executionfacts.AttemptOutcome{
			ObservedAt: managedNow(), SendState: "observed_sent", HTTPStatus: r.status,
			HeadersComplete: true, Acceptance: "unknown", ExecutionResult: observed.ExecutionResult,
			Usage: observed.Usage, BodyComplete: r.transportEnded && observed.BodyComplete,
			ResponseModel: observed.ResponseModel,
			EvidenceRefs:  []string{"router:" + s.attemptID + ":upstream-response"},
		}
		if r.dashScope != nil {
			recordManagedBailianRequestID(s, gjson.GetBytes(r.dashScope.last, "request_id").String())
		}
		outcome.EvidenceRefs = appendManagedBailianEvidence(s, r.headers, outcome.EvidenceRefs)
		if *r.status >= 400 {
			outcome.ExecutionResult, outcome.ErrorClass = "failed", "upstream_http_error"
		} else if r.transportEnded && *r.status >= 200 && *r.status < 300 &&
			(observed.ExecutionResult == "succeeded" || observed.ExecutionResult == "partial") {
			outcome.Acceptance = "accepted"
			if s.config.profile.supplier == "qwen" && outcome.Usage.Status == "reported" {
				switch {
				case r.supplier != nil:
					applyManagedQwenThinking(s, outcome.Usage, r.supplier.thinking)
				case r.chat != nil:
					applyManagedQwenThinking(s, outcome.Usage, r.chat.thinking)
				case r.dashScope != nil:
					applyManagedQwenThinking(s, outcome.Usage, r.dashScope.thinking)
				}
			}
		}
		if observed.ProviderError {
			outcome.ProviderError = "provider_error_response"
		}
		if outcome.BodyComplete {
			outcome.ResponseBodyDigest = "sha256:" + hex.EncodeToString(r.digest.Sum(nil))
			outcome.EvidenceRefs = append(outcome.EvidenceRefs, "response-body:"+outcome.ResponseBodyDigest)
		}
		if outcome.Usage.Status == "missing" {
			switch {
			case r.chat != nil && !r.chat.textLimited:
				outcome.Usage = availableManagedUsage(s, outcome, r.chat.estimateBody())
			case r.native != nil && !r.native.textLimited:
				outcome.Usage = availableManagedUsage(s, outcome, r.native.estimateBody())
			case r.dashScope != nil && !r.dashScope.textLimited:
				outcome.Usage = availableManagedUsage(s, outcome, r.dashScope.estimateBody())
			}
		}
		outcome.ToolUsage = managedToolMeasurements(s, r.toolUsage, outcome)
	} else if r.jsonOutcome != nil {
		outcome = *r.jsonOutcome
	} else {
		var headerErr, bodyErr error
		if r.status == nil {
			headerErr = errors.New("router upstream headers not observed")
		}
		if !r.transportEnded {
			bodyErr = errors.New("router upstream body not observed")
		}
		outcome = managedResponseOutcome(s, r.headers, headerErr, nil, bodyErr)
	}
	if !r.attemptRecorded {
		recordManagedAttempt(ctx, s, outcome)
		r.attemptRecorded = true
	}
	if !billableManagedSend(s, outcome) {
		return
	}
	delivery := outcome.ExecutionResult
	if !end {
		delivery = "cancelled"
	} else if outcome.HTTPStatus == nil || outcome.ExecutionResult == "unknown" || r.projectionFailed {
		delivery = "failed"
	}
	snapshot, err := managedDeliverySnapshot(s, outcome, delivery)
	if err != nil {
		managedFactGap(ctx, "delivery_fact_invalid")
		return
	}
	s.lastDelivery = &snapshot
	dispatchManagedSnapshot(ctx, s, snapshot)
	_ = observedusage.Publish(snapshot.Envelope())
}

func projectManagedRouterJSON(ctx wrapper.HttpContext, s *managedExecutionState, body []byte, outcome executionfacts.AttemptOutcome) bool {
	r := s.router
	policy := managedPublicPolicy(s)
	if *r.status >= 400 {
		return replaceManagedRouterFailure(ctx, s, clientresponse.ClassifyFailure(clientresponse.FailureInput{
			Status: *r.status, Format: s.config.profile.UpstreamProtocol, Client: policy.Protocol,
			Body: body, Request: s.publicRequest,
		}))
	}
	if outcome.ExecutionResult != "succeeded" {
		partial, status, handled, err := publicPartialResponse(ctx, s, r.headers, body)
		if handled && err == nil {
			return replaceManagedRouterJSON(ctx, uint16(status), partial.Headers, partial.Body)
		}
	}
	converted, err := convertManagedSupplierResponse(ctx, s, outcome, r.headers, body)
	if err == nil {
		projected, err := projectManagedResponse(s, r.headers, body, converted)
		if err == nil {
			return replaceManagedRouterJSON(ctx, *r.status, projected.Headers, projected.Body)
		}
	}
	replaceManagedRouterFailure(ctx, s, clientresponse.Failure{Kind: clientresponse.FailureIncomplete})
	return false
}

func replaceManagedRouterFailure(ctx wrapper.HttpContext, s *managedExecutionState, failure clientresponse.Failure) bool {
	policy := managedPublicPolicy(s)
	return replaceManagedRouterJSON(ctx, failure.Status(policy.Protocol), failure.Headers(policy, s.router.headers, false, false), failure.Body(policy))
}

func replaceManagedRouterJSON(ctx wrapper.HttpContext, status uint16, headers [][2]string, body []byte) bool {
	headers = append([][2]string{{":status", strconv.Itoa(int(status))}}, headers...)
	if proxywasm.ReplaceHttpResponseHeaders(headers) != nil || proxywasm.ReplaceHttpResponseBody(body) != nil {
		managedFactGap(ctx, "router_public_response_unavailable")
		return false
	}
	return true
}
