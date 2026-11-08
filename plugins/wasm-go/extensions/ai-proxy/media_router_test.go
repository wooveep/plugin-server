package main

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/mediaexecution"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/stretchr/testify/require"
)

func TestMediaRouterRequiresOneUseTargetBoundPermitAndRetainsMethod(t *testing.T) {
	for _, method := range []string{"POST", "GET"} {
		for _, upstream := range []bool{true, false} {
			t.Run(method+"/"+map[bool]string{true: "upstream", false: "local_reply"}[upstream], func(t *testing.T) {
				var scoped struct {
					Rules []map[string]interface{} `json:"_rules_"`
				}
				require.NoError(t, json.Unmarshal(nativeMediaProxyConfiguration(t, mediaexecution.Profile, false), &scoped))
				h := newManagedHost(t, scoped.Rules[0])
				c := h.cfg.ManagedExecution.(*managedExecutionConfig)
				require.NoError(t, h.host.SetProperty([]string{"cluster_name"}, []byte(c.release.Descriptor.ClusterRef)))
				key := uuid.NewString()
				body := []byte(`{"model":"qwen-image-3.0","input":{"prompt":"test"}}`)
				path := c.settings.UpstreamPath
				if method == "GET" {
					body = nil
					path = "/api/v1/tasks/original-task"
				}
				headers := [][2]string{{":method", method}, {":authority", wrapper.RouterContinuationHost}, {":path", managedRouterTargetPath(c)}, {"content-type", "application/json"}, {managedMediaRouterPermitHeader, key}}
				action := h.host.CallOnRequestHeaders(h.id, headers, len(body) == 0)
				require.Contains(t, []types.Action{types.HeaderStopIteration, types.ActionPause}, action)
				if len(body) > 0 {
					h.host.CallOnRequestBody(h.id, body, true)
				}
				calls := h.host.GetRedisCalloutAttributesFromContext(h.id)
				require.Equal(t, 1, len(calls))
				permit := managedMediaRouterPermit{c.release.Descriptor.DescriptorDigest, method, path, bodybasis.DigestBytes(body), [][2]string{{"authorization", "Bearer first-controlled-key"}, {"accept", "application/json"}}, 1000, time.Now().Add(time.Second * 30)}
				wire, err := json.Marshal(permit)
				require.NoError(t, err)
				// Redis returns the immutable value only after its atomic GET/DEL.
				reply := append([]byte("$"+strconv.Itoa(len(wire))+"\r\n"), wire...)
				reply = append(reply, []byte("\r\n")...)
				h.host.CallOnRedisCallResponse(calls[0].CalloutID, 0, reply)
				require.Nil(t, h.host.GetSentLocalResponse(h.id))
				require.Empty(t, h.host.GetCalloutAttributesFromContext(h.id))
				actual := h.host.GetCurrentRequestHeaders(h.id)
				actualMethod, err := managedSingleHeader(actual, ":method")
				require.NoError(t, err)
				require.Equal(t, method, actualMethod)
				actualPath, err := managedSingleHeader(actual, ":path")
				require.NoError(t, err)
				require.Equal(t, path, actualPath)
				if upstream {
					require.NoError(t, h.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
				}
				h.host.CallOnResponseHeaders(h.id, [][2]string{{":status", "502"}, {managedMediaRouterReceiptHeader, "provider-forgery"}}, true)
				receipt, err := managedSingleHeader(h.host.GetCurrentResponseHeaders(h.id), managedMediaRouterReceiptHeader)
				if upstream {
					require.NoError(t, err)
					require.Equal(t, key, receipt)
				} else {
					require.Error(t, err)
				}
			})
		}
	}
}

func TestMediaRouterRejectsDirectExternalEntrance(t *testing.T) {
	var scoped struct {
		Rules []map[string]interface{} `json:"_rules_"`
	}
	require.NoError(t, json.Unmarshal(nativeMediaProxyConfiguration(t, mediaexecution.Profile, false), &scoped))
	h := newManagedHost(t, scoped.Rules[0])
	c := h.cfg.ManagedExecution.(*managedExecutionConfig)
	h.host.CallOnRequestHeaders(h.id, [][2]string{{":method", "POST"}, {":authority", wrapper.RouterContinuationHost}, {":path", managedRouterTargetPath(c)}, {"content-type", "application/json"}}, false)
	require.EqualValues(t, 403, h.host.GetSentLocalResponse(h.id).StatusCode)
	require.Empty(t, h.host.GetRedisCalloutAttributesFromContext(h.id))
	require.Empty(t, h.host.GetCalloutAttributesFromContext(h.id))
}
