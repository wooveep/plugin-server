package wrapper

import (
	"encoding/binary"
	"errors"
	"maps"
	"strconv"
	"strings"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
)

// RouterContinuationProperty is private downstream-request FilterState. Only
// the original Router executor may install a pending continuation identifier.
const RouterContinuationProperty = "modelfusion.router.continuation.v1"

// RouterContinuationPathPrefix identifies SDK-owned, release-isolated entrances.
const RouterContinuationPathPrefix = "/__modelfusion/router/"

// RouterContinuationHost is an internal redirect authority, never a DNS target.
const RouterContinuationHost = "modelfusion-router.internal"

// RouterContinuationPending reads the private handoff marker. A single dash
// clears it without relying on a host's representation of an empty property.
func RouterContinuationPending() bool {
	value, err := proxywasm.GetProperty([]string{RouterContinuationProperty})
	return err == nil && len(value) > 1
}

// CopyRouterRequestContext restores bounded request metadata for an executor's
// verified continuation. Configuration and response-flow state stay on the new
// context. Map values retain existing typed state without copying request bodies.
func CopyRouterRequestContext[C any](destination, source HttpContext) bool {
	dst, dstOK := destination.(*CommonHttpCtx[C])
	src, srcOK := source.(*CommonHttpCtx[C])
	if !dstOK || !srcOK {
		return false
	}
	dst.userContext, dst.userAttribute = maps.Clone(src.userContext), maps.Clone(src.userAttribute)
	dst.scheme, dst.host, dst.path, dst.method = src.scheme, src.host, src.path, src.method
	dst.requestContentType, dst.requestContentEncoding = src.requestContentType, src.requestContentEncoding
	return true
}

// DeclareRequestProperty gives a private property downstream-request lifetime.
// Unlike ordinary Wasm properties it survives Envoy's internal stream recreation.
func DeclareRequestProperty(name string) error {
	// Envoy DeclarePropertyArguments: name is field 1; span is field 5,
	// DownstreamRequest=1. Bytes, mutable and empty schema are proto3 defaults.
	wire := binary.AppendUvarint([]byte{0x0a}, uint64(len(name)))
	wire = append(wire, name...)
	_, err := proxywasm.CallForeignFunction("declare_property", append(wire, 0x28, 0x01))
	if errors.Is(err, types.ErrorStatusBadArgument) {
		// The host rejects a second declaration of the same immutable prototype.
		return nil
	}
	return err
}

type routerResponseContinuationOption[C any] struct {
	enabled func(C) bool
}

// WithRouterResponseContinuation preserves response observers across a trusted
// internal redirect without repeating their request processors. Add this option
// after the processor's handlers. It is opt-in for an exact native configuration.
func WithRouterResponseContinuation[C any](enabled func(C) bool) CtxOption[C] {
	return &routerResponseContinuationOption[C]{enabled: enabled}
}

func (o *routerResponseContinuationOption[C]) Apply(vm *CommonVmCtx[C]) {
	headers, body, done := vm.onHttpRequestHeaders, vm.onHttpRequestBody, vm.onHttpStreamDone
	responseHeaders, responseBody, responseStream := vm.onHttpResponseHeaders, vm.onHttpResponseBody, vm.onHttpStreamingResponseBody
	contexts := make(map[string]*CommonHttpCtx[C])
	retired := make(map[*CommonHttpCtx[C]]bool)
	waiting := make(map[*CommonHttpCtx[C]]time.Time)
	property := "modelfusion.router.observer." + vm.pluginName
	const contextKey = "wrapper.router.observer-key"
	// A redirect recreates its chain immediately on the same worker. Keep only
	// the exceptional client-close/failed-redirect handoff for a bounded interval.
	collect := func() {
		for ctx, at := range waiting {
			if time.Since(at) >= 30*time.Second {
				key := ctx.GetStringContext(contextKey, "")
				if contexts[key] == ctx {
					delete(contexts, key)
				}
				delete(waiting, ctx)
			}
		}
	}
	parse := vm.parseConfig
	vm.parseConfig = func(context PluginContext, raw []byte, config *C) error {
		// OnPluginStart clears tick registrations before parsing each reload.
		RegisterTickFunc(1000, collect)
		if parse != nil {
			return parse(context, raw, config)
		}
		return nil
	}
	vm.onHttpRequestHeaders = func(context HttpContext, config C) types.Action {
		if !o.enabled(config) {
			if headers != nil {
				return headers(context, config)
			}
			return types.ActionContinue
		}
		ctx := context.(*CommonHttpCtx[C])
		if strings.HasPrefix(ctx.path, RouterContinuationPathPrefix) {
			// These entrances intentionally omit customer admission. No observer
			// may run a cache lookup or request processor for an external caller.
			if !RouterContinuationPending() {
				_ = proxywasm.SendHttpResponse(403, [][2]string{{"content-type", "application/json"}}, []byte(`{"error":{"code":"router_continuation_required","message":"Forbidden"}}`), -1)
				return types.ActionPause
			}
			key, _ := proxywasm.GetProperty([]string{property})
			previous := contexts[string(key)]
			if previous == nil {
				return unavailableRouterObserver()
			}
			if _, alreadyDone := waiting[previous]; alreadyDone {
				delete(waiting, previous)
			} else {
				retired[previous] = true
			}
			ctx.config = previous.config
			CopyRouterRequestContext[C](ctx, previous)
			ctx.needRequestBody, ctx.streamingRequestBody = false, false
			ctx.needResponseBody = responseBody != nil || responseStream != nil
			ctx.streamingResponseBody = responseStream != nil
			contexts[string(key)] = ctx
			return types.ActionContinue
		}
		if err := DeclareRequestProperty(property); err != nil {
			return unavailableRouterObserver()
		}
		key := strconv.FormatUint(uint64(ctx.contextID), 10)
		if proxywasm.SetProperty([]string{property}, []byte(key)) != nil {
			return unavailableRouterObserver()
		}
		ctx.SetContext(contextKey, key)
		contexts[key] = ctx
		if headers != nil {
			return headers(context, config)
		}
		return types.ActionContinue
	}
	vm.onHttpRequestBody = func(context HttpContext, config C, input []byte) types.Action {
		if !context.(*CommonHttpCtx[C]).needRequestBody || body == nil {
			return types.ActionContinue
		}
		return body(context, config, input)
	}
	vm.onHttpResponseHeaders = func(context HttpContext, config C) types.Action {
		if o.enabled(config) && RouterContinuationPending() || responseHeaders == nil {
			return types.ActionContinue
		}
		return responseHeaders(context, config)
	}
	if responseBody != nil {
		vm.onHttpResponseBody = func(context HttpContext, config C, input []byte) types.Action {
			if o.enabled(config) && RouterContinuationPending() {
				return types.ActionContinue
			}
			return responseBody(context, config, input)
		}
	}
	if responseStream != nil {
		vm.onHttpStreamingResponseBody = func(context HttpContext, config C, input []byte, end bool) []byte {
			if o.enabled(config) && RouterContinuationPending() {
				return input
			}
			return responseStream(context, config, input, end)
		}
	}
	vm.onHttpStreamDone = func(context HttpContext, config C) {
		ctx := context.(*CommonHttpCtx[C])
		if retired[ctx] {
			delete(retired, ctx)
			return
		}
		key := ctx.GetStringContext(contextKey, "")
		if key != "" {
			if RouterContinuationPending() {
				waiting[ctx] = time.Now()
				return
			}
			if contexts[key] == ctx {
				delete(contexts, key)
			}
		}
		if done != nil {
			done(context, config)
		}
	}
}

func unavailableRouterObserver() types.Action {
	_ = proxywasm.SendHttpResponse(503, [][2]string{{"content-type", "application/json"}}, []byte(`{"error":{"code":"execution_unavailable","message":"Service temporarily unavailable"}}`), -1)
	return types.ActionPause
}
