package wrapper

import (
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/resp"
)

const redisDiagnosticSentinel = "owned-redis-diagnostic-secret-never-log"

func redisDiagnosticHost(t *testing.T, request func() types.Action) (proxytest.HostEmulator, uint32) {
	t.Helper()
	original := proxywasm.GetVMContext()
	vm := NewCommonVmCtx[struct{}]("redis-diagnostics", ProcessRequestHeaders(func(HttpContext, struct{}) types.Action { return request() }))
	host, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption().WithVMContext(vm).WithPluginConfiguration([]byte(`{}`)))
	t.Cleanup(func() { reset(); proxywasm.SetVMContext(original) })
	host.RegisterForeignFunction("declare_property", func([]byte) []byte { return []byte{0} })
	host.RegisterForeignFunction("get_log_level", func([]byte) []byte { return []byte{0, 0, 0, 0} })
	require.Equal(t, types.OnPluginStartStatusOK, host.StartPlugin())
	id := host.InitializeHttpContext()
	return host, id
}

func redisDiagnosticLogs(host proxytest.HostEmulator) string {
	var lines []string
	for _, level := range [][]string{host.GetTraceLogs(), host.GetDebugLogs(), host.GetInfoLogs(), host.GetWarnLogs(), host.GetErrorLogs(), host.GetCriticalLogs()} {
		lines = append(lines, level...)
	}
	return strings.Join(lines, "\n")
}

func requireRedisDiagnosticRedacted(t *testing.T, logs string, query, response []byte) {
	t.Helper()
	for _, forbidden := range []string{redisDiagnosticSentinel, base64.StdEncoding.EncodeToString([]byte(redisDiagnosticSentinel)), base64.StdEncoding.EncodeToString(query)} {
		require.NotContains(t, logs, forbidden)
	}
	if len(response) > 0 {
		require.NotContains(t, logs, base64.StdEncoding.EncodeToString(response))
	}
}

func TestRedisCallDiagnosticsKeepResultsAndOmitAllPayloads(t *testing.T) {
	query := respString([]interface{}{"AUTH", "owned-user", redisDiagnosticSentinel})
	bulk := []byte(fmt.Sprintf("$%d\r\n%s\r\n", len(redisDiagnosticSentinel), redisDiagnosticSentinel))
	dispatchFailure := errors.New(redisDiagnosticSentinel)
	for _, tc := range []struct {
		name, category, stage string
		status                int32
		body                  []byte
		dispatchErr           error
		failed                bool
	}{
		{name: "success-auth", category: "success", stage: "response", body: []byte("+OK\r\n")},
		{name: "success-value", category: "success", stage: "response", body: bulk},
		{name: "dispatch", category: "unknown", stage: "dispatch", dispatchErr: dispatchFailure, failed: true},
		{name: "unknown-host-status", category: "host_status_unknown", stage: "callback", status: 37, failed: true},
		{name: "unmapped-timeout-like-status", category: "host_status_unknown", stage: "callback", status: 2, failed: true},
		{name: "response-read", category: "not_found", stage: "response_read", failed: true},
		{name: "response-parse", category: "invalid_resp", stage: "response_parse", body: []byte("$9\r\nshort"), failed: true},
		{name: "redis-error", category: "redis_error", stage: "response", body: []byte("-ERR " + redisDiagnosticSentinel + "\r\n"), failed: true},
		{name: "noauth", category: "noauth", stage: "response", body: []byte("-NOAUTH Authentication required " + redisDiagnosticSentinel + "\r\n"), failed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			callbacks := 0
			var result resp.Value
			var immediate error
			dispatch := redisDispatchFunc(proxywasm.DispatchRedisCall)
			if tc.dispatchErr != nil {
				dispatch = func(string, []byte, func(int, int)) (uint32, error) { return 0, tc.dispatchErr }
			}
			host, id := redisDiagnosticHost(t, func() types.Action {
				immediate = redisCallWithDispatch("owned-redis", query, func(reply resp.Value) { callbacks++; result = reply }, nil, nil, dispatch)
				return types.ActionContinue
			})
			host.CallOnRequestHeaders(id, [][2]string{{":authority", "owned.example.test"}}, true)
			if tc.dispatchErr != nil {
				require.ErrorIs(t, immediate, tc.dispatchErr)
				require.Zero(t, callbacks)
				require.Empty(t, host.GetRedisCalloutAttributesFromContext(id))
			} else {
				require.NoError(t, immediate)
				calls := host.GetRedisCalloutAttributesFromContext(id)
				require.Len(t, calls, 1)
				time.Sleep(time.Millisecond)
				host.CallOnRedisCallResponse(calls[0].CalloutID, tc.status, tc.body)
				require.Equal(t, 1, callbacks)
				if tc.failed {
					require.Error(t, result.Error())
				} else {
					require.NoError(t, result.Error())
				}
				if tc.status != 0 {
					require.Contains(t, result.Error().Error(), fmt.Sprintf("status=%d", tc.status))
					require.NotContains(t, result.Error().Error(), "cannot connect")
				}
			}
			logs := redisDiagnosticLogs(host)
			requireRedisDiagnosticRedacted(t, logs, query, tc.body)
			require.Contains(t, logs, "stage="+tc.stage+" category="+tc.category)
			if tc.dispatchErr == nil {
				require.Contains(t, logs, fmt.Sprintf("status=%d", tc.status))
				elapsed := regexp.MustCompile(`elapsed_us=(\d+)`).FindAllStringSubmatch(logs, -1)
				positive := false
				for _, match := range elapsed {
					n, err := strconv.ParseInt(match[1], 10, 64)
					require.NoError(t, err)
					positive = positive || n > 0
				}
				require.True(t, positive, "diagnostics must record real elapsed time")
			}
			require.Regexp(t, `request_id=[0-9a-f-]{36}`, logs)
			host.CompleteHttpContext(id)
		})
	}
}

func TestRedisCallDiagnosticsPreserveOneAuthenticationRetryAndCompletion(t *testing.T) {
	query := respString([]interface{}{"GET", redisDiagnosticSentinel})
	noauth := []byte("-NOAUTH Authentication required " + redisDiagnosticSentinel + "\r\n")
	for _, tc := range []struct {
		name                                          string
		authFails, retryDispatchFails, repeatedNoauth bool
	}{
		{name: "retry-success"}, {name: "authentication-fails", authFails: true},
		{name: "retry-dispatch-fails", retryDispatchFails: true}, {name: "second-noauth", repeatedNoauth: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ready, callbacks, auths, dispatches := true, 0, 0, 0
			var result resp.Value
			dispatch := func(cluster string, query []byte, callback func(int, int)) (uint32, error) {
				dispatches++
				if tc.retryDispatchFails && dispatches == 2 {
					return 0, errors.New(redisDiagnosticSentinel)
				}
				return proxywasm.DispatchRedisCall(cluster, query, callback)
			}
			checkReady := func() error {
				auths++
				if tc.authFails {
					return errors.New(redisDiagnosticSentinel)
				}
				ready = true
				return nil
			}
			host, id := redisDiagnosticHost(t, func() types.Action {
				require.NoError(t, redisCallWithDispatch("owned-redis", query, func(reply resp.Value) { callbacks++; result = reply }, &ready, checkReady, dispatch))
				return types.ActionContinue
			})
			host.CallOnRequestHeaders(id, [][2]string{{":authority", "owned.example.test"}}, true)
			calls := host.GetRedisCalloutAttributesFromContext(id)
			require.Len(t, calls, 1)
			host.CallOnRedisCallResponse(calls[0].CalloutID, 0, noauth)
			require.Equal(t, 1, auths)
			if !tc.authFails && !tc.retryDispatchFails {
				require.Zero(t, callbacks)
				calls = host.GetRedisCalloutAttributesFromContext(id)
				require.Len(t, calls, 1)
				require.Equal(t, query, calls[0].Query)
				body := []byte("+OK\r\n")
				if tc.repeatedNoauth {
					body = noauth
				}
				host.CallOnRedisCallResponse(calls[0].CalloutID, 0, body)
			}
			require.Equal(t, 1, callbacks)
			if tc.authFails || tc.retryDispatchFails || tc.repeatedNoauth {
				require.Error(t, result.Error())
			} else {
				require.NoError(t, result.Error())
			}
			if tc.authFails {
				require.Equal(t, 1, dispatches)
				require.False(t, ready)
			} else {
				require.Equal(t, 2, dispatches)
			}
			logs := redisDiagnosticLogs(host)
			requireRedisDiagnosticRedacted(t, logs, query, noauth)
			require.Contains(t, logs, "stage=reauth")
			ids := regexp.MustCompile(`request_id=([0-9a-f-]{36})`).FindAllStringSubmatch(logs, -1)
			require.NotEmpty(t, ids)
			for _, match := range ids {
				require.Equal(t, ids[0][1], match[1], "one logical retry keeps one correlation ID")
			}
			if !tc.authFails {
				require.Contains(t, logs, "attempt=1")
			}
			host.CompleteHttpContext(id)
		})
	}
}
