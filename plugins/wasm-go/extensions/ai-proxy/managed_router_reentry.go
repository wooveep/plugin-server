package main

import (
	"errors"
	"strings"
	"time"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/config"
	"github.com/google/uuid"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

const managedRouterRedirectHeader = "x-modelfusion-router-redirect"
const managedRouterRedirectKey = "ai-proxy.router-redirect"
const managedRouterPreparedKey = "ai-proxy.router-prepared"
const managedRouterBodyActiveKey = "ai-proxy.router-body-active"

type managedRouterContinuation struct {
	context  wrapper.HttpContext
	previous *managedExecutionState
	next     *managedExecutionState
	request  managedProtocolRequest
	expires  time.Time
}

var managedRouterPending = make(map[string]managedRouterContinuation)
var managedRouterReentryTickRegistered bool

func configureManagedRouterReentry() error {
	if err := wrapper.DeclareRequestProperty(wrapper.RouterContinuationProperty); err != nil {
		return errors.New("Router continuation requires downstream-request FilterState")
	}
	if !managedRouterReentryTickRegistered {
		managedRouterReentryTickRegistered = true
		wrapper.RegisterTickFunc(1000, func() {
			for key, entry := range managedRouterPending {
				if !time.Now().Before(entry.expires) {
					delete(managedRouterPending, key)
					entry.next.closed = true
					if entry.next.text != nil {
						entry.next.text.closed = true
					}
					if entry.previous.router != nil && entry.previous.router.jsonOutcome != nil {
						recordManagedCancelledSale(entry.context, entry.previous, *entry.previous.router.jsonOutcome)
					}
				}
			}
		})
	}
	return nil
}

func managedRouterTargetPath(c *managedExecutionConfig) string {
	return wrapper.RouterContinuationPathPrefix + strings.TrimSuffix(c.release.Descriptor.ServiceRef, ".dns")
}

func acceptManagedRouterContinuation(ctx wrapper.HttpContext, c *managedExecutionConfig) (types.Action, bool) {
	if !strings.HasPrefix(ctx.Path(), wrapper.RouterContinuationPathPrefix) {
		return types.ActionContinue, false
	}
	raw, err := proxywasm.GetProperty([]string{wrapper.RouterContinuationProperty})
	entry, found := managedRouterPending[string(raw)]
	if err != nil || !found || !c.settings.RouterReentry || ctx.Path() != managedRouterTargetPath(c) ||
		!time.Now().Before(entry.expires) || entry.next.config.release.Descriptor.ReleaseID != c.release.Descriptor.ReleaseID ||
		entry.next.config.release.Descriptor.DescriptorDigest != c.release.Descriptor.DescriptorDigest ||
		!entry.next.admissionCurrent(time.Now()) {
		return rejectManagedRequest(ctx, 403, "router_continuation_required"), true
	}
	delete(managedRouterPending, string(raw))
	if !wrapper.CopyRouterRequestContext[config.PluginConfig](ctx, entry.context) ||
		proxywasm.SetProperty([]string{wrapper.RouterContinuationProperty}, []byte("-")) != nil {
		return rejectManagedRequest(ctx, 503, "router_continuation_unavailable"), true
	}
	s := entry.next
	s.sourceConfig = c
	// Request metadata is cloned shallowly. The old context still owns and closes
	// its log, so the continuation needs an independent per-stream log entry.
	ctx.SetContext(managedAccessLogContext, nil)
	beginManagedAccessLog(ctx, c.release.Descriptor.ReleaseID)
	ctx.SetContext(managedExecutionContextKey, s)
	ctx.SetContext(managedRouterRedirectKey, false)
	ctx.SetContext(managedRouterPreparedKey, entry.request)
	ctx.SetContext(managedRouterBodyActiveKey, false)
	ctx.DisableReroute()
	return types.HeaderStopIteration, true
}

func queueManagedRouterContinuation(ctx wrapper.HttpContext, previous, next *managedExecutionState, request managedProtocolRequest) error {
	if !previous.config.settings.RouterReentry || !next.config.settings.RouterReentry ||
		previous.config.release.Descriptor.ReleaseID != next.config.release.Descriptor.ReleaseID ||
		previous.router != nil && previous.router.jsonOutcome == nil {
		return errors.New("Router continuation has no original same-release execution")
	}
	key := uuid.NewString()
	// The host recreates the chain synchronously. Thirty seconds bounds retained
	// request memory if it cannot redirect or the downstream closes during handoff.
	expires := time.Now().Add(30 * time.Second)
	if !next.admissionDeadline.IsZero() && next.admissionDeadline.Before(expires) {
		expires = next.admissionDeadline
	}
	managedRouterPending[key] = managedRouterContinuation{ctx, previous, next, request, expires}
	// The response-stage SRDS lookup retains the original domain scope. The
	// private path is compiled in that scope; FilterState still authorizes it.
	uri := ctx.Scheme() + "://" + ctx.Host() + managedRouterTargetPath(next.config)
	if proxywasm.SetProperty([]string{wrapper.RouterContinuationProperty}, []byte(key)) != nil {
		delete(managedRouterPending, key)
		return errors.New("Router continuation could not be installed")
	}
	ctx.SetContext(managedRouterRedirectKey, true)
	if previous.router == nil {
		// A retained Responses resource can select its owner before the first
		// supplier send. Only this executor-created response carries authority.
		if err := proxywasm.SendHttpResponse(307, [][2]string{{managedRouterRedirectHeader, uri}}, nil, -1); err != nil {
			delete(managedRouterPending, key)
			ctx.SetContext(managedRouterRedirectKey, false)
			return err
		}
		return nil
	}
	if err := proxywasm.ReplaceHttpResponseHeader(managedRouterRedirectHeader, uri); err != nil {
		delete(managedRouterPending, key)
		ctx.SetContext(managedRouterRedirectKey, false)
		return err
	}
	previous.router.ended = true
	if !ctx.GetBoolContext(managedRouterBodyActiveKey, false) {
		if err := proxywasm.ResumeHttpResponse(); err != nil {
			delete(managedRouterPending, key)
			ctx.SetContext(managedRouterRedirectKey, false)
			return err
		}
	}
	return nil
}

func managedRouterRetryOutcome(s *managedExecutionState, outcome executionfacts.AttemptOutcome) bool {
	return s.config.settings.RouterReentry && s.config.settings.TextExecution != nil && outcome.HTTPStatus != nil && *outcome.HTTPStatus >= 400
}
