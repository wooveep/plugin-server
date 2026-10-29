package main

import (
	"errors"
	"strconv"
	"strings"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/ai/upstreamhealth"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

// healthGatewayEndpoint is supplied by the same trusted Console service
// registration as execution_admission. It is not decoded from a public body,
// client header, provider URL, or a recovered claim receipt.
type healthGatewayEndpoint struct {
	ServiceName string
	ServicePort uint32
	Host        string
	AuthToken   string
	TimeoutMS   uint32
}

// The startup owner must be the original registered runtime instance/boot value.
// This leaf neither mints a second UUID nor reads an external HTTP assertion.
func newHealthGatewayClient(endpoint healthGatewayEndpoint, ownerInstance string) (*upstreamhealth.GatewayClient, error) {
	if !managedID(endpoint.ServiceName) || strings.ContainsAny(endpoint.ServiceName, " /:@?#[]") ||
		endpoint.ServicePort == 0 || endpoint.ServicePort > 65535 || !managedAuthority(endpoint.Host) ||
		endpoint.TimeoutMS == 0 || endpoint.TimeoutMS > 60000 || !healthHeaderValue(endpoint.AuthToken) || !healthHeaderValue(ownerInstance) {
		return nil, errors.New("upstream health requires the registered Console endpoint and startup owner")
	}
	cluster := wrapper.FQDNCluster{FQDN: endpoint.ServiceName, Port: int64(endpoint.ServicePort), Host: endpoint.Host}
	headers := [][2]string{{"authorization", "Bearer " + endpoint.AuthToken},
		{"x-modelfusion-gateway-instance", ownerInstance}, {"content-type", "application/json"}, {"accept", "application/json"}}
	call := func(method, path string, trusted [][2]string, body []byte, timeout uint32, callback func(upstreamhealth.GatewayResponse)) error {
		// GatewayClient owns these paths; preserve escaped opaque IDs instead of
		// decoding/rebuilding them through url.URL.Path in a generic HTTP wrapper.
		if !strings.HasPrefix(path, "/internal/upstream-health/") || strings.ContainsAny(path, "\r\n\x00?#") {
			return errors.New("upstream health destination is not its registered internal service")
		}
		requestHeaders := [][2]string{{":method", method}, {":scheme", "http"}, {":authority", endpoint.Host}, {":path", path}}
		requestHeaders = append(requestHeaders, trusted...)
		_, err := proxywasm.DispatchHttpCall(cluster.ClusterName(), requestHeaders, body, nil, timeout,
			func(numHeaders, bodySize, _ int) {
				limit := upstreamhealth.MaxGatewayBodyBytes
				if path == upstreamhealth.SnapshotPath {
					limit = upstreamhealth.MaxSnapshotBytes
				}
				callback(readHealthGatewayResponse(numHeaders, bodySize, limit))
			})
		return err
	}
	return upstreamhealth.NewGatewayClient(call, headers, ownerInstance, endpoint.TimeoutMS)
}

func healthHeaderValue(value string) bool {
	if len(value) == 0 || len(value) > 8192 {
		return false
	}
	for _, char := range value {
		if char <= 0x20 || char >= 0x7f {
			return false
		}
	}
	return true
}

// The callback reads its own callout response only. It never consults or
// resumes a customer HTTP context; deleted-request callbacks cannot recreate a
// business send. A missing SDK callback leaves the client's exact local veto.
func readHealthGatewayResponse(numHeaders, bodySize, maxBodyBytes int) upstreamhealth.GatewayResponse {
	var response upstreamhealth.GatewayResponse
	headers, err := proxywasm.GetHttpCallResponseHeaders()
	response.HeadersComplete = err == nil && len(headers) == numHeaders
	statuses := 0
	for _, header := range headers {
		if header[0] == ":status" {
			statuses++
			status, err := strconv.Atoi(header[1])
			if err == nil && len(header[1]) == 3 && status >= 100 && status <= 599 {
				response.StatusCode = status
			}
		}
	}
	if statuses != 1 {
		response.HeadersComplete = false
		response.StatusCode = 0
	}
	if bodySize < 0 || bodySize > maxBodyBytes {
		return response
	}
	if bodySize == 0 {
		response.BodyComplete = true
		return response
	}
	body, err := proxywasm.GetHttpCallResponseBody(0, bodySize)
	response.BodyComplete = err == nil && len(body) == bodySize
	response.Body = append([]byte(nil), body...)
	return response
}
