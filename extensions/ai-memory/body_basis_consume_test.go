package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestFirstBodyMemoryConsumesAfterHistoryContract(t *testing.T) {
	for _, tc := range append(test.BodyBasisContracts(), test.LargeHistoryBodyBasisContracts()...) {
		name := tc.ClientProtocol
		if tc.LargeHistory {
			name += "/large-history"
		}
		t.Run(name, func(t *testing.T) {
			test.RunGoTest(t, func(t *testing.T) {
				raw := strings.Replace(string(memoryNativeConfiguration(t, "semantic", false)), `"capture_body_basis":true`, `"capture_body_basis":false`, 1)
				raw = strings.Replace(raw, "openai_chat", tc.ClientProtocol, 1)
				basis, err := bodybasis.Capture([]byte(tc.Original))
				require.NoError(t, err)
				headers := append(memoryNativeCallerFixture().Headers(), basis.Headers()...)
				host := startNativeMemoryHeaders(t, []byte(raw), headers)
				defer host.Reset()
				require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody([]byte(tc.AfterHistory)))
				requireMemoryAssembleCall(t, host)
				host.CallOnHttpCall(memoryAssembleHeaders(), memoryAssembleResponse(t, "inject", map[string]interface{}{"role": "system", "content": "memory-context"}, nil))
				require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
				values := make([]any, 2)
				for i, raw := range [][]byte{[]byte(tc.AfterMemory), host.GetRequestBody()} {
					decoder := json.NewDecoder(bytes.NewReader(raw))
					decoder.UseNumber()
					require.NoError(t, decoder.Decode(&values[i]))
				}
				require.Equal(t, values[0], values[1], "complete handoff retains attachments and exact numbers")
				if tc.LargeHistory {
					require.Greater(t, len(host.GetRequestBody()), mfjson.MaxInputBytes)
				}

				after, err := bodybasis.Read(host.GetRequestHeaders())
				require.NoError(t, err)
				require.Equal(t, basis, after)
				require.NotEqual(t, bodybasis.DigestBytes(host.GetRequestBody()), after.Digest())
			})
		})
	}
}

func TestFirstBodyMemoryMissingBasisDoesNotRewriteOrUseLegacy(t *testing.T) {
	for _, name := range []string{"missing", "duplicate", "wrong-model", "duplicate-body", "invalid-number", "invalid-unicode"} {
		t.Run(name, func(t *testing.T) {
			test.RunGoTest(t, func(t *testing.T) {
				raw := strings.Replace(string(memoryNativeConfiguration(t, "semantic", false)), `"capture_body_basis":true`, `"capture_body_basis":false`, 1)
				body := []byte(test.BodyBasisContracts()[0].AfterHistory)
				headers := memoryNativeCallerFixture().Headers()
				if name != "missing" {
					original := []byte(test.BodyBasisContracts()[0].Original)
					if name == "wrong-model" {
						original = []byte(strings.Replace(string(original), "public-model", "other-model", 1))
					}
					basis, err := bodybasis.Capture(original)
					require.NoError(t, err)
					headers = append(headers, basis.Headers()...)
					switch name {
					case "duplicate-body":
						body = []byte(`{"model":"public-model","nested":{"x":1,"x":2},"messages":[]}`)
					case "invalid-number":
						body = []byte(`{"model":"public-model","number":1e10000,"messages":[]}`)
					case "invalid-unicode":
						body = []byte(`{"model":"public-model","messages":[{"role":"user","content":"\ud800"}]}`)
					}
					if name == "duplicate" {
						headers = append(headers, basis.Headers()...)
					}
				}
				host := startNativeMemoryHeaders(t, []byte(raw), headers)
				defer host.Reset()
				require.Equal(t, types.ActionContinue, host.CallOnHttpRequestBody(body))
				require.Nil(t, host.GetLocalResponse())
				require.Empty(t, host.GetHttpCalloutAttributes())
				require.Empty(t, host.GetRedisCalloutAttributes())
				require.Equal(t, string(body), string(host.GetRequestBody()))
			})
		})
	}
}
