package provider

import (
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/util"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

const headerUpstreamFirstByteTimeout = "x-envoy-upstream-rq-first-byte-timeout-ms"

var transparentCredentialHeaders = []string{
	util.HeaderAuthorization,
	util.HeaderXApiKey,
	util.HeaderXAuthorization,
	util.HeaderAnthropicApiKey,
	"api-key",
	"x-goog-api-key",
	"x-amz-content-sha256",
	"x-amz-date",
	"x-amz-security-token",
}

// BuildTransparentRequestHeaders applies provider-owned authority and credentials
// while preserving the request operation and all non-credential headers.
func (c *ProviderConfig) BuildTransparentRequestHeaders(
	activeProvider Provider,
	ctx wrapper.HttpContext,
	source http.Header,
) http.Header {
	headers := source.Clone()
	if handler, ok := activeProvider.(TransparentRequestHeadersHandler); ok {
		handler.TransformTransparentRequestHeaders(ctx, headers)
	} else if handler, ok := activeProvider.(TransformRequestHeadersHandler); ok {
		candidate := source.Clone()
		handler.TransformRequestHeaders(ctx, "", candidate)
		copyTransparentTransportPolicy(headers, candidate)
	}

	requestTarget := source.Get(util.HeaderPath)
	if c.basePath != "" {
		switch c.basePathHandling {
		case basePathHandlingRemovePrefix:
			requestTarget = strings.TrimPrefix(requestTarget, c.basePath)
		case basePathHandlingPrepend:
			if !strings.HasPrefix(requestTarget, c.basePath) {
				requestTarget = path.Join(c.basePath, requestTarget)
			}
		}
	}
	if c.providerBasePath != "" {
		requestTarget = c.applyProviderBasePath(requestTarget)
	}
	headers.Set(util.HeaderPath, requestTarget)
	if c.providerDomain != "" {
		util.OverwriteRequestHostHeader(headers, c.providerDomain)
	}
	return headers
}

func copyTransparentTransportPolicy(destination, candidate http.Header) {
	if authority := candidate.Get(util.HeaderAuthority); authority != "" {
		destination.Set(util.HeaderAuthority, authority)
	}
	for _, name := range transparentCredentialHeaders {
		destination.Del(name)
		for _, value := range candidate.Values(name) {
			destination.Add(name, value)
		}
	}
}

func clearTransparentCredentials(headers http.Header) {
	for _, name := range transparentCredentialHeaders {
		headers.Del(name)
	}
}

// BuildTransparentRequestBodyAuthHeaders applies body-dependent provider
// authentication while copying back only credential/signing headers. This
// prevents a provider hook from changing the opaque transport operation.
func (c *ProviderConfig) BuildTransparentRequestBodyAuthHeaders(
	activeProvider Provider,
	ctx wrapper.HttpContext,
	source http.Header,
	body []byte,
) (http.Header, types.Action, error) {
	headers := source.Clone()
	handler, ok := activeProvider.(TransparentRequestBodyAuthHandler)
	if !ok {
		return headers, types.ActionContinue, nil
	}

	candidate := source.Clone()
	action, err := handler.AuthenticateTransparentRequest(
		ctx,
		append([]byte(nil), body...),
		candidate,
	)
	if err != nil {
		return headers, action, err
	}
	for _, name := range transparentCredentialHeaders {
		headers.Del(name)
		for _, value := range candidate.Values(name) {
			headers.Add(name, value)
		}
	}
	return headers, action, nil
}

func (c *ProviderConfig) ApplyTransparentTransportPolicy(
	activeProvider Provider,
	ctx wrapper.HttpContext,
) error {
	headers := c.BuildTransparentRequestHeaders(activeProvider, ctx, util.GetRequestHeaders())
	util.ReplaceRequestHeaders(headers)
	return c.ApplyTransparentFirstByteTimeout()
}

func (c *ProviderConfig) ApplyTransparentFirstByteTimeout() error {
	if c.firstByteTimeout == 0 {
		return proxywasm.RemoveHttpRequestHeader(headerUpstreamFirstByteTimeout)
	}
	return proxywasm.ReplaceHttpRequestHeader(
		headerUpstreamFirstByteTimeout,
		strconv.FormatUint(uint64(c.firstByteTimeout), 10),
	)
}
