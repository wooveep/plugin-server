package main

import (
	"errors"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
)

func TestNativeCallHeaderFailuresAreDiagnosedWithoutFailingTheResponse(t *testing.T) {
	host, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption())
	defer reset()
	for _, tc := range []struct {
		operation string
		err       error
		expected  string
	}{
		{"remove", nil, ""},
		{"remove", types.ErrorStatusNotFound, ""},
		{"remove", errors.New("host internal failure"), "native client call ID response header remove failed"},
		{"write", types.ErrorStatusNotFound, "native client call ID response header write failed"},
		{"write", errors.New("host internal failure"), "native client call ID response header write failed"},
		{"write", nil, ""},
	} {
		before := len(host.GetWarnLogs())
		reportNativeCallHeaderError(tc.operation, tc.err)
		logs := host.GetWarnLogs()
		if tc.expected == "" {
			if len(logs) != before {
				t.Fatalf("unexpected diagnostic for %s: %v", tc.operation, logs)
			}
		} else if len(logs) != before+1 || logs[before] != tc.expected {
			t.Fatalf("missing fixed %s diagnostic: %v", tc.operation, logs)
		}
	}
}
