package main

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-memory/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/sessionctx"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/stretchr/testify/require"
)

func TestMemoryRequestFailOpen(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("parse failure continues upstream with original request body", func(t *testing.T) {
			original := []byte(`{"model":"qwen-turbo","messages":[{"role":"user","content":"raw parse failure prompt"}`)
			host, bodyAction := startMemoryFailOpenRequest(t, original)

			require.Equal(t, types.ActionContinue, bodyAction)
			require.Empty(t, host.GetRedisCalloutAttributes())
			require.Empty(t, host.GetHttpCalloutAttributes())
			requireMemoryFailOpenOriginalBody(t, host, original)
			requireMemoryConsoleSafeLogs(t, host, "raw parse failure prompt")
		})

		t.Run("assemble failure after Redis miss continues upstream with original request body", func(t *testing.T) {
			original := memoryConsoleAssembleRequestBody()
			host, bodyAction := startMemoryFailOpenRequest(t, original)
			require.Equal(t, types.ActionPause, bodyAction)
			requireMemoryRecentRedisLookup(t, host)

			host.CallOnRedisCall(0, test.CreateRedisRespNull())
			requireMemoryAssembleCall(t, host)
			host.CallOnHttpCall(memoryAssembleHeaders(), []byte(`{"schema_version":1,"decision":"inject","memory_message":{"role":"system","content":"raw assemble failure memory"}`))

			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			requireMemoryFailOpenOriginalBody(t, host, original)
			requireMemoryConsoleSafeLogs(t, host, "raw assemble failure memory")
		})

	})
}

func TestMemoryRequestReplacementFailureFailsOpen(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		original := memoryConsoleAssembleRequestBody()
		probe := failMemoryRequestBodyReplacement(t)
		host, bodyAction := startMemoryFailOpenRequest(t, original)
		require.Equal(t, types.ActionPause, bodyAction)
		requireMemoryRecentRedisLookup(t, host)

		host.CallOnRedisCall(0, test.CreateRedisRespNull())
		requireMemoryAssembleCall(t, host)
		host.CallOnHttpCall(memoryAssembleHeaders(), memoryAssembleResponse(t, "inject", map[string]interface{}{
			"role":    "system",
			"content": "replacement failure memory",
		}, nil))

		require.Equal(t, 1, probe.calls)
		requireMemoryExpectedMessages(t, requireMemoryRequestMessagesFromBody(t, probe.body), []memoryExpectedMessage{
			{role: "system", content: "replacement failure memory"},
			{role: "user", content: "current question"},
		})
		require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
		requireMemoryFailOpenOriginalBody(t, host, original)
		requireMemoryConsoleSafeLogs(t, host, "replacement failure memory", "Console trace raw value", "Console diagnostic raw value")
	})
}

func TestMemoryRequestReplacementFailureBoundary(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		original := []byte(`null`)
		replaced, err := sessionctx.ReplaceOpenAIChatMessages(original, []sessionctx.OpenAIMessage{
			{Role: "system", Content: "replacement failure memory"},
		})

		require.Error(t, err)
		require.Nil(t, replaced)
		require.Equal(t, `null`, string(original))
	})
}

func TestMemoryRecentLoadDispatchFailureStillAssembles(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		original := memoryConsoleAssembleRequestBody()
		previous := memoryLoadRecentMemory
		memoryLoadRecentMemory = func(ctx wrapper.HttpContext, c config.PluginConfig, log log.Log) error {
			return errors.New("recent redis dispatch failed")
		}
		t.Cleanup(func() {
			memoryLoadRecentMemory = previous
		})

		host, bodyAction := startMemoryFailOpenRequest(t, original)

		require.Equal(t, types.ActionPause, bodyAction)
		require.Empty(t, host.GetRedisCalloutAttributes())
		facts := requireMemoryAssembleCall(t, host)
		require.Equal(t, "semantic", facts["memory_mode"])
		require.Equal(t, "current question", facts["current_question"])
	})
}

func startMemoryFailOpenRequest(t *testing.T, body []byte) (test.TestHost, types.Action) {
	t.Helper()
	host, status := newMemoryConfigTestHost(memoryConsoleAssembleConfig(t, "semantic"))
	t.Cleanup(host.Reset)
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName("memory-route"))
	require.NoError(t, host.SetRequestId("property-request-id"))

	headerAction := host.CallOnHttpRequestHeaders(memoryConsoleAssembleHeaders())
	require.Equal(t, types.HeaderStopIteration, headerAction)

	return host, host.CallOnHttpRequestBody(body)
}

type memoryReplacementProbe struct {
	calls int
	body  []byte
}

func failMemoryRequestBodyReplacement(t *testing.T) *memoryReplacementProbe {
	t.Helper()
	probe := &memoryReplacementProbe{}
	previous := memoryReplaceHTTPRequestBody
	memoryReplaceHTTPRequestBody = func(body []byte) error {
		probe.calls++
		probe.body = append([]byte(nil), body...)
		return errors.New("replace request body failed")
	}
	t.Cleanup(func() {
		memoryReplaceHTTPRequestBody = previous
	})
	return probe
}

func requireMemoryRequestMessagesFromBody(t *testing.T, body []byte) []map[string]interface{} {
	t.Helper()
	var request struct {
		Messages []map[string]interface{} `json:"messages"`
	}
	require.NoErrorf(t, json.Unmarshal(body, &request), "attempted replacement body must be JSON; body=%s", string(body))
	return request.Messages
}

func requireMemoryFailOpenOriginalBody(t *testing.T, host test.TestHost, original []byte) {
	t.Helper()
	require.Equal(t, string(original), string(host.GetRequestBody()))
	require.Nil(t, host.GetLocalResponse(), "memory fail-open paths must not synthesize local responses")
}
