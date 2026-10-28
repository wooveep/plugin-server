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

func historyCaptureConfig(client string) string {
	return strings.Replace(strings.Replace(nativeHistoryConfig, "openai_chat", client, 1), `"redis":`, `"capture_body_basis":true,"redis":`, 1)
}

func requireHistoryContractBody(t *testing.T, expected string, actual []byte) {
	t.Helper()
	if err := mfjson.ValidateRuntimeBody(actual, historyBodyLimit); err != nil {
		t.Fatal(err)
	}
	values := make([]any, 2)
	for i, raw := range [][]byte{[]byte(expected), actual} {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		require.NoError(t, decoder.Decode(&values[i]))
	}
	require.Equal(t, values[0], values[1], "the complete next-stage input retains attachments, systems, tools and exact numbers")
}

func TestFirstBodyHistoryCapturesBeforeBothProcessorContract(t *testing.T) {
	for _, tc := range append(test.BodyBasisContracts(), test.LargeHistoryBodyBasisContracts()...) {
		name := tc.ClientProtocol
		if tc.LargeHistory {
			name += "/large-history"
		}
		t.Run(name, func(t *testing.T) {
			test.RunGoTest(t, func(t *testing.T) {
				raw := historyCaptureConfig(tc.ClientProtocol)
				host, status := test.NewTestHost([]byte(raw))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				original := []byte(tc.Original)
				headers := historyTestHeaders(t, original, historyTestCaller())
				headers = append(headers, [2]string{strings.ToUpper(bodybasis.DigestHeader), "client-spoof"})
				require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(headers))
				_, err := bodybasis.Read(host.GetRequestHeaders())
				require.Error(t, err, "all incoming basis values are cleared before capture")
				require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(original))
				want, err := bodybasis.Capture(original)
				require.NoError(t, err)
				actual, err := bodybasis.Read(host.GetRequestHeaders())
				require.NoError(t, err)
				require.Equal(t, want, actual)
				var config PluginConfig
				require.NoError(t, parseHistoryBinding(gjson.Parse(raw), &config))
				inputs := tc.PriorInputs
				if len(inputs) == 0 {
					inputs = []string{`{"role":"user","content":"previous"}`}
				}
				var turns []historyTurn
				for _, input := range inputs {
					turns = append(turns, historyTurn{Input: []json.RawMessage{json.RawMessage(input)}, Output: []json.RawMessage{json.RawMessage(`{"role":"assistant","content":"previous answer"}`)}})
				}
				stored := nativeHistoryDocument{SchemaVersion: "history.native.v1", Identity: historyNativeIdentity{config.historyBinding.Identity, historyTestCaller()}, Protocol: config.historyBinding.Protocol, PublicModel: "public-model", Turns: turns}
				encoded, err := json.Marshal(stored)
				require.NoError(t, err)
				host.CallOnRedisCall(0, test.CreateRedisRespString(string(encoded)))
				requireHistoryContractBody(t, tc.AfterHistory, host.GetRequestBody())
				after, err := bodybasis.Read(host.GetRequestHeaders())
				require.NoError(t, err)
				require.Equal(t, want, after, "history must not capture its rewritten context")
				require.NotEqual(t, bodybasis.DigestBytes(host.GetRequestBody()), after.Digest())
				if tc.LargeHistory {
					require.LessOrEqual(t, len(original), mfjson.MaxInputBytes)
					require.Greater(t, len(host.GetRequestBody()), mfjson.MaxInputBytes)
				}
			})
		})
	}
}

func TestFirstBodyHistoryCaptureSurvivesPolicyBypasses(t *testing.T) {
	for _, name := range []string{"off", "no-store", "missing-caller"} {
		t.Run(name, func(t *testing.T) {
			test.RunGoTest(t, func(t *testing.T) {
				raw := historyCaptureConfig("openai_chat")
				if name == "off" {
					raw = strings.Replace(raw, `"redis":`, `"enabled":false,"redis":`, 1)
				}
				host, status := test.NewTestHost([]byte(raw))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				body := []byte(test.BodyBasisContracts()[0].Original)
				headers := historyTestHeaders(t, body, historyTestCaller())
				if name == "missing-caller" {
					headers = headers[:4]
				}
				if name == "no-store" {
					headers = append(headers, [2]string{"cache-control", "private, no-store"})
				}
				require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(headers))
				require.Equal(t, types.ActionContinue, host.CallOnHttpRequestBody(body))
				require.Equal(t, string(body), string(host.GetRequestBody()))
				require.Empty(t, host.GetRedisCalloutAttributes())
				want, err := bodybasis.Capture(body)
				require.NoError(t, err)
				got, err := bodybasis.Read(host.GetRequestHeaders())
				require.NoError(t, err)
				require.Equal(t, want, got)
			})
		})
	}
}

func TestFirstBodyHistoryCaptureBoundaryAndCompleteBody(t *testing.T) {
	for _, raw := range []string{
		`{"capture_body_basis":true}`, `{"capture_body_basis":"true"}`,
		strings.Replace(historyCaptureConfig("openai_chat"), "text_generate", "task_query", 1),
		strings.Replace(historyCaptureConfig("openai_chat"), "openai_chat", "unknown", 1),
	} {
		var config PluginConfig
		require.Error(t, parseHistoryBinding(gjson.Parse(raw), &config))
	}
	var config PluginConfig
	require.NoError(t, parseHistoryBinding(gjson.Parse(nativeHistoryConfig), &config))
	require.False(t, config.historyBinding.CaptureBodyBasis)
	test.RunGoTest(t, func(t *testing.T) {
		raw := strings.Replace(historyCaptureConfig("openai_chat"), `"redis":`, `"enabled":false,"redis":`, 1)
		host, status := test.NewTestHost([]byte(raw))
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)
		body := []byte(test.BodyBasisContracts()[0].Original)
		require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(historyTestHeaders(t, body, historyTestCaller())))
		middle := len(body) / 2
		require.Equal(t, types.ActionPause, host.CallOnHttpStreamingRequestBody(body[:middle], false))
		_, err := bodybasis.Read(host.GetRequestHeaders())
		require.Error(t, err)
		require.Equal(t, types.ActionContinue, host.CallOnHttpStreamingRequestBody(body[middle:], true))
		want, err := bodybasis.Capture(body)
		require.NoError(t, err)
		got, err := bodybasis.Read(host.GetRequestHeaders())
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
}
