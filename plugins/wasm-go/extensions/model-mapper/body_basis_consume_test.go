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
	"github.com/tidwall/gjson"
)

const consumeConfig = `{"consume_body_basis":true,"modelMapping":{"public-model":"upstream-model"}}`

func TestFirstBodyMapperConsumesBothProcessorContract(t *testing.T) {
	for _, tc := range append(test.BodyBasisContracts(), test.LargeHistoryBodyBasisContracts()...) {
		name := tc.ClientProtocol
		if tc.LargeHistory {
			name += "/large-history"
		}
		t.Run(name, func(t *testing.T) {
			test.RunGoTest(t, func(t *testing.T) {
				host, status := test.NewTestHost([]byte(consumeConfig))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				basis, err := bodybasis.Capture([]byte(tc.Original))
				require.NoError(t, err)
				headers := append(mapperHeaders()[:4], basis.Headers()...)
				require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(headers))
				before, err := bodybasis.Read(host.GetRequestHeaders())
				require.NoError(t, err)
				require.Equal(t, basis, before)
				require.Equal(t, types.ActionContinue, host.CallOnHttpRequestBody([]byte(tc.AfterMemory)))
				require.Nil(t, host.GetLocalResponse())
				values := make([]any, 2)
				for i, raw := range [][]byte{[]byte(strings.Replace(tc.AfterMemory, `"model":"public-model"`, `"model":"upstream-model"`, 1)), host.GetRequestBody()} {
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
				require.Equal(t, basis, after, "the exact original snapshot crosses mapper unchanged")
				require.NotEqual(t, bodybasis.DigestBytes(host.GetRequestBody()), after.Digest())
			})
		})
	}
}

func TestFirstBodyMapperRejectsInvalidConsumeConfigurationAndEvidence(t *testing.T) {
	for _, raw := range []string{
		`{"consume_body_basis":true,"capture_body_basis":true}`,
		`{"consume_body_basis":"true"}`, `{"consume_body_basis":null}`,
		`{"consume_body_basis":false,"consume_body_basis":true}`,
	} {
		var config Config
		require.Error(t, parseConfig(gjson.Parse(raw), &config))
	}
	for _, name := range []string{"missing", "duplicate", "invalid-digest", "different-model", "duplicate-body", "truncated-body"} {
		t.Run(name, func(t *testing.T) {
			test.RunGoTest(t, func(t *testing.T) {
				host, status := test.NewTestHost([]byte(consumeConfig))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				basis, err := bodybasis.Capture([]byte(test.BodyBasisContracts()[0].Original))
				require.NoError(t, err)
				headers := mapperHeaders()[:4]
				if name != "missing" {
					headers = append(headers, basis.Headers()...)
				}
				if name == "duplicate" {
					headers = append(headers, basis.Headers()...)
				}
				if name == "invalid-digest" {
					headers[len(headers)-2][1] = "sha256:bad"
				}
				action := host.CallOnHttpRequestHeaders(headers)
				if name == "missing" || name == "duplicate" || name == "invalid-digest" {
					require.Equal(t, types.ActionPause, action)
				} else {
					require.Equal(t, types.HeaderStopIteration, action)
					body := []byte(test.BodyBasisContracts()[0].AfterMemory)
					switch name {
					case "different-model":
						body = []byte(strings.Replace(string(body), "public-model", "other-model", 1))
					case "duplicate-body":
						body = []byte(`{"model":"public-model","x":1,"x":2}`)
					case "truncated-body":
						body = body[:len(body)-1]
					}
					require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(body))
					require.Equal(t, string(body), string(host.GetRequestBody()))
				}
				require.NotNil(t, host.GetLocalResponse())
				require.Equal(t, uint32(400), host.GetLocalResponse().StatusCode)
			})
		})
	}
}
