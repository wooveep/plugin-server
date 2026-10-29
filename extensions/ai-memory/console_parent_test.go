package main

import (
	"testing"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestManagedMemoryAssembleRetainsOriginalAdmissionParent(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		callID := "99999999-9999-4999-8999-999999999999"
		for _, mode := range []string{"original", "single-client-id", "duplicate", "foreign-release", "legacy"} {
			t.Run(mode, func(t *testing.T) {
				admission := [][2]string{{executionfacts.AdmissionCallHeader, callID}, {executionfacts.AdmissionReleaseHeader, "original/release:opaque"}, {executionfacts.AdmissionOccurredHeader, time.Now().UTC().Add(-time.Second).Format(time.RFC3339Nano)}, {executionfacts.AdmissionDeadlineHeader, time.Now().UTC().Add(time.Minute).Format(time.RFC3339Nano)}}
				switch mode {
				case "single-client-id":
					admission = admission[:1]
				case "duplicate":
					admission = append(admission, admission[0])
				case "foreign-release":
					admission[1][1] = "foreign-release"
				}
				body := []byte(`{"model":"public.model","messages":[{"role":"user","content":"current question"}]}`)
				if mode == "legacy" {
					facts := memoryConsoleAssembleFactsWithConfig(t, memoryConsoleAssembleCustomConfig(t, "full", nil, false, true), body, append(memoryConsoleAssembleHeaders(), admission...))
					require.NotContains(t, facts, "call_id", "legacy labels cannot acquire a native call authority")
					require.NotContains(t, facts, "native_identity")
					return
				}
				headers := append(memoryNativeCallerFixture().Headers(), admission...)
				host := startNativeMemoryHeaders(t, memoryNativeConfiguration(t, "full", false), headers)
				defer host.Reset()
				// Later filters remove these headers. Parentage must already be frozen
				// with the original authenticated caller at request-header time.
				for _, name := range []string{executionfacts.AdmissionCallHeader, executionfacts.AdmissionReleaseHeader, executionfacts.AdmissionOccurredHeader, executionfacts.AdmissionDeadlineHeader} {
					require.NoError(t, proxywasm.RemoveHttpRequestHeader(name))
				}
				for _, name := range balanceidentity.HeaderNames() {
					require.NoError(t, proxywasm.RemoveHttpRequestHeader(name))
				}
				require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(body))
				facts := requireMemoryAssembleCall(t, host)
				if mode == "original" {
					require.Equal(t, callID, facts["call_id"])
				} else {
					require.NotContains(t, facts, "call_id", "incomplete/duplicate/foreign admission remains unknown")
				}
				native := facts["native_identity"].(map[string]interface{})
				require.NotContains(t, native, "call_id", "runtime parent does not mutate the frozen static identity")
				caller := native["caller"].(map[string]interface{})
				require.Equal(t, memoryNativeCallerFixture().APIKeyID, caller["api_key_id"])
				require.Equal(t, "9007199254740993", caller["credential_generation"])
				require.Equal(t, memoryNativeRouteID, facts["route"].(map[string]interface{})["id"])
				require.Equal(t, "full", facts["memory_mode"])
			})
		}
	})
}
