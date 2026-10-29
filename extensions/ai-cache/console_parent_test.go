package main

import (
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

func TestNativeCacheLookupAndEventRetainOriginalAdmissionParent(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		for _, mode := range []string{"original", "missing-native-state", "single-client-id", "duplicate", "foreign-release", "legacy"} {
			t.Run(mode, func(t *testing.T) {
				cfg := nativeCacheExecutionConfig(t, true)
				headers := nativeCacheExecutionHeaders(true)
				if mode != "original" {
					cfg = nativeCacheTestConfig(t)
					switch mode {
					case "single-client-id":
						headers = append(nativeCacheTestHeaders(nativeCacheTestCaller()), [2]string{executionfacts.AdmissionCallHeader, nativeCacheOriginalCall})
					case "duplicate":
						headers = append(headers, [2]string{executionfacts.AdmissionCallHeader, nativeCacheOriginalCall})
					case "foreign-release":
						for index := range headers {
							if headers[index][0] == executionfacts.AdmissionReleaseHeader {
								headers[index][1] = "foreign-release"
							}
						}
					case "legacy":
						var err error
						cfg, err = sjson.DeleteBytes(cfg, "native_identity")
						require.NoError(t, err)
					}
				}
				host, status := test.NewTestHost(cfg)
				require.Equal(t, types.OnPluginStartStatusOK, status)
				defer host.Reset()
				require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(headers))
				for _, name := range []string{executionfacts.AdmissionCallHeader, executionfacts.AdmissionReleaseHeader, executionfacts.AdmissionOccurredHeader, executionfacts.AdmissionDeadlineHeader} {
					require.NoError(t, proxywasm.RemoveHttpRequestHeader(name))
				}
				for _, name := range balanceidentity.HeaderNames() {
					require.NoError(t, proxywasm.RemoveHttpRequestHeader(name))
				}
				require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody([]byte(nativeCacheTestBody)))
				require.Len(t, host.GetRedisCalloutAttributes(), 1)
				host.CallOnRedisCall(0, test.CreateRedisRespNull())
				calls := host.GetHttpCalloutAttributes()
				require.Len(t, calls, 1)
				lookup := nativeCacheDecodeObject(t, calls[0].Body)
				if mode == "original" {
					require.Equal(t, nativeCacheOriginalCall, lookup["call_id"])
				} else {
					require.NotContains(t, lookup, "call_id", "missing original state cannot be filled by client headers")
				}
				if mode != "legacy" {
					native := lookup["native_identity"].(map[string]interface{})
					require.NotContains(t, native, "call_id")
					caller := native["caller"].(map[string]interface{})
					require.Equal(t, nativeCacheTestCaller().APIKeyID, caller["api_key_id"])
					require.Equal(t, "9007199254740993", caller["credential_generation"])
				}
				host.CallOnHttpCall([][2]string{{":status", "200"}, {"content-type", "application/json"}}, []byte(`{"data":{"decision":"miss"}}`))
				host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
				host.CallOnHttpResponseBody([]byte(`{"choices":[{"message":{"content":"answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":7,"completion_tokens":3}}`))
				var event map[string]interface{}
				for _, call := range host.GetRedisCalloutAttributes() {
					command, ok := thinResponseCaptureCommand(t, call.Query)
					if !ok || len(command) < 5 || !strings.EqualFold(command[0], "XADD") {
						continue
					}
					for index := thinResponseCaptureXADDFieldStart(t, command); index+1 < len(command); index += 2 {
						if command[index] == "event" {
							require.Nil(t, event)
							event = nativeCacheDecodeObject(t, []byte(command[index+1]))
						}
					}
				}
				require.NotNil(t, event)
				if mode == "original" {
					require.Equal(t, nativeCacheOriginalCall, event["call_id"])
				} else {
					require.NotContains(t, event, "call_id")
				}
				require.Equal(t, lookup["native_identity"], event["native_identity"])
				for _, field := range []string{"tenant", "consumer", "route", "model", "request_digest", "cache_policy_version"} {
					require.Equal(t, lookup[field], event[field])
				}
			})
		}
	})
}
