package main

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/mediaexecution"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/resp"
)

const managedMediaRouterPermitHeader = "x-modelfusion-media-permit"
const managedMediaRouterReceiptHeader = "x-modelfusion-media-upstream-receipt"
const managedMediaRouterContextKey = "ai-proxy.media-router.v1"
const managedMediaRouterPermitPrefix = "modelfusion:media:router:"

// This is a one-use handoff of already-consumed JSON media work, never a new
// customer admission or a persisted grant to retry a generation.
type managedMediaRouterPermit struct {
	DescriptorDigest string      `json:"descriptor_digest"`
	Method           string      `json:"method"`
	Path             string      `json:"path"`
	BodyDigest       string      `json:"body_digest"`
	Headers          [][2]string `json:"headers"`
	TimeoutMS        uint32      `json:"timeout_ms"`
	ExpiresAt        time.Time   `json:"expires_at"`
}

type managedMediaRouterRequest struct {
	permit   string
	cluster  string
	started  bool
	accepted bool
}

func (r *managedMediaRuntime) routerCall(request mediaexecution.Request, callback func(mediaexecution.Response)) error {
	var descriptor managedDescriptor
	if decodeManagedJSON(request.Descriptor(), &descriptor, true) != nil || descriptor.DescriptorDigest == "" || descriptor.ServiceRef == "" || r.redis == nil {
		return errors.New("media Router original target is absent")
	}
	service := request.RouterLoopbackService()
	if service != "mfr-"+descriptor.ReleaseID+".static" && service != "mfr-"+descriptor.ReleaseID+".dns" && service != "modelfusion-router-loopback.dns" {
		return errors.New("media Router original loopback registry is absent")
	}
	key := uuid.NewString()
	permit := managedMediaRouterPermit{descriptor.DescriptorDigest, request.Method(), request.Path(), request.OutboundBodyDigest(), request.Headers(), request.TimeoutMilliseconds(), r.now().Add(30 * time.Second)}
	permit.Headers = append(permit.Headers, [2]string{managedCallHeader, request.CallID()}, [2]string{managedAttemptHeader, request.AttemptID()}, [2]string{"accept-encoding", "identity"})
	wire, err := json.Marshal(permit)
	if err != nil {
		return err
	}
	// This bounded JSON control call reaches the local Gateway, whose normal
	// Router owns the supplier connection. SSE never enters this handoff.
	return r.redis.Command([]interface{}{"SET", managedMediaRouterPermitPrefix + key, string(wire), "NX", "PX", "30000"}, func(reply resp.Value) {
		if reply.Error() != nil || reply.String() != "OK" {
			callback(mediaexecution.Response{ObservedAt: r.now().UTC().Format(time.RFC3339Nano), SendState: "not_sent", TransportError: "media_router_handoff_unavailable"})
			return
		}
		headers := [][2]string{{":method", request.Method()}, {":scheme", "http"}, {":authority", wrapper.RouterContinuationHost},
			{":path", wrapper.RouterContinuationPathPrefix + strings.TrimSuffix(descriptor.ServiceRef, ".dns")},
			{"content-type", "application/json"}, {managedMediaRouterPermitHeader, key}}
		cluster := "outbound|80||" + service
		_, err := proxywasm.DispatchHttpCall(cluster, headers, request.Body(), nil, request.TimeoutMilliseconds()+1000, func(numHeaders, bodySize, _ int) {
			callback(readManagedMediaResponse(numHeaders, bodySize, r.settings.MaxResponseBytes, r.now(), key))
		})
		if err != nil {
			callback(mediaexecution.Response{ObservedAt: r.now().UTC().Format(time.RFC3339Nano), SendState: "not_sent", TransportError: "media_router_dispatch_failed"})
		}
	})
}

func acceptManagedMediaRouter(ctx wrapper.HttpContext, c *managedExecutionConfig) types.Action {
	if c.settings.SendTransport != "router" || ctx.Path() != managedRouterTargetPath(c) {
		return rejectManagedRequest(ctx, 403, "media_router_permit_required")
	}
	headers, err := proxywasm.GetHttpRequestHeaders()
	key, headerErr := managedSingleHeader(headers, managedMediaRouterPermitHeader)
	if err != nil || headerErr != nil || uuid.Validate(key) != nil {
		return rejectManagedRequest(ctx, 403, "media_router_permit_required")
	}
	ctx.DisableReroute()
	ctx.SetContext(managedMediaRouterContextKey, &managedMediaRouterRequest{permit: key, cluster: c.release.Descriptor.ClusterRef})
	ctx.SetRequestBodyBufferLimit(managedMediaPrivateLimit)
	if !ctx.HasRequestBody() {
		return executeManagedMediaRouter(ctx, c, nil)
	}
	return types.HeaderStopIteration
}

const consumeMediaRouterPermit = `local raw=redis.call('GET',KEYS[1]); if not raw then return false end
local p=cjson.decode(raw)
if p.descriptor_digest~=ARGV[1] or p.body_digest~=ARGV[2] then return false end
redis.call('DEL',KEYS[1]); return raw`

func executeManagedMediaRouter(ctx wrapper.HttpContext, c *managedExecutionConfig, body []byte) types.Action {
	state, _ := ctx.GetContext(managedMediaRouterContextKey).(*managedMediaRouterRequest)
	if state == nil || state.started {
		return types.ActionPause
	}
	state.started = true
	cluster, err := proxywasm.GetProperty([]string{"cluster_name"})
	if err != nil || string(cluster) != state.cluster {
		return rejectManagedRequest(ctx, 503, "media_router_target_not_verified")
	}
	body = append([]byte(nil), body...)
	err = c.priceRedis.Command([]interface{}{"EVAL", consumeMediaRouterPermit, 1, managedMediaRouterPermitPrefix + state.permit, c.release.Descriptor.DescriptorDigest, bodybasis.DigestBytes(body)}, func(reply resp.Value) {
		var permit managedMediaRouterPermit
		if reply.Error() != nil || decodeManagedJSON([]byte(reply.String()), &permit, true) != nil ||
			!time.Now().Before(permit.ExpiresAt) || permit.TimeoutMS == 0 || permit.TimeoutMS > 600000 ||
			permit.DescriptorDigest != c.release.Descriptor.DescriptorDigest || permit.BodyDigest != bodybasis.DigestBytes(body) ||
			!(permit.Method == "POST" && permit.Path == c.settings.UpstreamPath || permit.Method == "GET" &&
				c.release.Descriptor.Capabilities["task_query"].QueryCapabilityRef != "" &&
				strings.HasPrefix(permit.Path, "/api/v1/tasks/") && managedID(strings.TrimPrefix(permit.Path, "/api/v1/tasks/"))) {
			rejectManagedRequest(ctx, 403, "media_router_permit_invalid")
			return
		}
		scheme := "http"
		if c.release.Descriptor.Transport.SNI != "" {
			scheme = "https"
		}
		headers := [][2]string{{":method", permit.Method}, {":path", permit.Path}, {":authority", c.settings.Authority}, {":scheme", scheme},
			{"content-length", strconv.Itoa(len(body))}, {"x-forwarded-proto", "http"}, {"x-envoy-upstream-rq-timeout-ms", strconv.FormatUint(uint64(permit.TimeoutMS), 10)}}
		for _, header := range permit.Headers {
			if !strings.EqualFold(header[0], "host") {
				headers = append(headers, header)
			}
		}
		if proxywasm.ReplaceHttpRequestHeaders(headers) != nil || len(body) > 0 && proxywasm.ReplaceHttpRequestBody(body) != nil {
			rejectManagedRequest(ctx, 500, "media_router_install_failed")
			return
		}
		state.accepted = true
		if proxywasm.ResumeHttpRequest() != nil {
			rejectManagedRequest(ctx, 502, "media_router_resume_failed")
		}
	})
	if err != nil {
		return rejectManagedRequest(ctx, 503, "media_router_handoff_unavailable")
	}
	return types.ActionPause
}

func observeManagedMediaRouterHeaders(ctx wrapper.HttpContext) (types.Action, bool) {
	state, _ := ctx.GetContext(managedMediaRouterContextKey).(*managedMediaRouterRequest)
	if state == nil {
		return types.ActionContinue, false
	}
	_ = proxywasm.RemoveHttpResponseHeader(managedMediaRouterReceiptHeader)
	_ = proxywasm.RemoveHttpResponseHeader(managedRouterRedirectHeader)
	_ = proxywasm.RemoveHttpResponseHeader(executionfacts.DeliveryBridgeHeader)
	cluster, err := proxywasm.GetProperty([]string{"cluster_name"})
	if state.accepted && wrapper.IsResponseFromUpstream() && err == nil && string(cluster) == state.cluster {
		_ = proxywasm.AddHttpResponseHeader(managedMediaRouterReceiptHeader, state.permit)
	}
	ctx.DontReadResponseBody()
	return types.ActionContinue, true
}
