package provider

import (
	"strconv"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
)

const headerUpstreamFirstByteTimeout = "x-envoy-upstream-rq-first-byte-timeout-ms"

func (c *ProviderConfig) ApplyTransparentFirstByteTimeout() error {
	if c.firstByteTimeout == 0 {
		return proxywasm.RemoveHttpRequestHeader(headerUpstreamFirstByteTimeout)
	}
	return proxywasm.ReplaceHttpRequestHeader(
		headerUpstreamFirstByteTimeout,
		strconv.FormatUint(uint64(c.firstByteTimeout), 10),
	)
}
