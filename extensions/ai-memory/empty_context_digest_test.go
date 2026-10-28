package main

import (
	"strconv"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/memorycache"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

// SHA-256 of {"schema_version":1,"messages":[]}; preserve the existing digest wire.
const memoryEmptyContextDigest = "0c6a35509a3b0c55a295fe39b644b579e83ad2ed60867523fa63f557b89317be"

func TestMemoryKnownEmptyCacheDigest(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for _, protocol := range []struct{ name, body string }{
			{"openai_chat", `{"model":"qwen-turbo","messages":[{"role":"user","content":"weather?"}],"metadata":{"exact":9007199254740993}}`},
			{"anthropic_messages", `{"model":"qwen-turbo","messages":[{"role":"user","content":"weather?"}],"metadata":{"exact":9007199254740993}}`},
			{"openai_responses", `{"model":"qwen-turbo","input":"weather?","metadata":{"exact":9007199254740993}}`},
		} {
			for _, tc := range []struct {
				name     string
				status   int32
				response []byte
				known    bool
			}{
				{"valid_empty_array", 0, test.CreateRedisRespArray(nil), true},
				{"valid_empty_record", 0, test.CreateRedisRespArray([]interface{}{`{"schema_version":1,"messages":[]}`}), true},
				{"error", 0, test.CreateRedisRespError("temporary"), false},
				{"unknown_null", 0, test.CreateRedisRespNull(), false},
				{"wrong_resp_shape", 0, test.CreateRedisRespString("not-an-array"), false},
				{"invalid_record", 0, test.CreateRedisRespArray([]interface{}{`{invalid`}), false},
				{"timeout", 1, nil, false},
			} {
				t.Run(protocol.name+"/"+tc.name, func(t *testing.T) {
					cfg, err := sjson.SetBytes(memoryConsoleAssembleConfig(t, "recent-only"), "client_protocol", protocol.name)
					require.NoError(t, err)
					cfg, err = sjson.SetBytes(cfg, "operation_kind", "text_generate")
					require.NoError(t, err)
					host := startMemoryConsoleAssembleRequestWithConfigNoCleanup(t, cfg, []byte(protocol.body), append(memoryConsoleAssembleHeaders(), [2]string{memorycache.DeprecatedDigestHeader, "client-spoof"}))
					defer host.Reset()
					host.CallOnRedisCall(tc.status, tc.response)
					require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
					require.Equal(t, []byte(protocol.body), host.GetRequestBody(), "no effective context must not rewrite or round the actual request")
					require.Empty(t, host.GetHttpCalloutAttributes())
					require.False(t, test.HasHeader(host.GetRequestHeaders(), memorycache.DeprecatedDigestHeader))
					digest, propertyErr := host.GetProperty([]string{memorycache.TrustedDigestProperty})
					if tc.known {
						require.NoError(t, propertyErr)
						require.Equal(t, memoryEmptyContextDigest, string(digest))
					} else {
						require.Error(t, propertyErr, "unknown or failed dependent reads cannot stand in for known empty context")
					}
				})
			}
		}
		for _, response := range []struct {
			name string
			raw  []byte
		}{
			{"empty", test.CreateRedisRespArray(nil)},
			{"existing_recent", validMemoryRecentZSetResponse(t)},
			{"irrelevant_error", test.CreateRedisRespError("temporary")},
			{"irrelevant_unknown", test.CreateRedisRespNull()},
		} {
			t.Run("client_or_history_complete/"+response.name, func(t *testing.T) {
				body := []byte(`{"model":"qwen-turbo","messages":[{"role":"user","content":"older"},{"role":"assistant","content":"earlier answer"},{"role":"user","content":"weather?"}],"metadata":{"exact":9007199254740993}}`)
				host := startMemoryConsoleAssembleRequestWithConfigNoCleanup(t, memoryConsoleAssembleConfig(t, "recent-only"), body, memoryConsoleAssembleHeaders())
				defer host.Reset()
				host.CallOnRedisCall(0, response.raw)
				require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
				require.Equal(t, body, host.GetRequestBody(), "the original multi-user rule must still suppress consumer-level Recent")
				require.Equal(t, memoryEmptyContextDigest, requireMemoryTrustedDigestProperty(t, host))
			})
		}
	})
}

func TestMemoryConsoleEmptyCacheDigest(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for _, multiUser := range []bool{false, true} {
			for _, recentKnown := range []bool{false, true} {
				for _, tc := range []struct {
					name     string
					headers  [][2]string
					response []byte
					allowed  bool
				}{
					{"recent_only", memoryAssembleHeaders(), memoryAssembleResponse(t, "recent_only", nil, nil), true},
					{"inject", memoryAssembleHeaders(), memoryAssembleResponse(t, "inject", nil, nil), true},
					{"skip", memoryAssembleHeaders(), memoryAssembleResponse(t, "skip", nil, nil), false},
					{"bypass", memoryAssembleHeaders(), memoryAssembleResponse(t, "bypass", nil, nil), false},
					{"http_failure", [][2]string{{":status", "503"}}, []byte(`{"error":"unavailable"}`), false},
					{"invalid_response", memoryAssembleHeaders(), []byte(`{invalid`), false},
				} {
					t.Run(tc.name+"/multi_"+strconv.FormatBool(multiUser)+"/recent_known_"+strconv.FormatBool(recentKnown), func(t *testing.T) {
						body := []byte(`{"model":"qwen-turbo","messages":[{"role":"user","content":"weather?"}]}`)
						if multiUser {
							body = []byte(`{"model":"qwen-turbo","messages":[{"role":"user","content":"older"},{"role":"assistant","content":"earlier answer"},{"role":"user","content":"weather?"}]}`)
						}
						host := startMemoryConsoleAssembleRequestWithConfigNoCleanup(t, memoryConsoleAssembleConfig(t, "semantic"), body, memoryConsoleAssembleHeaders())
						defer host.Reset()
						if recentKnown {
							host.CallOnRedisCall(0, test.CreateRedisRespArray(nil))
						} else {
							host.CallOnRedisCall(0, test.CreateRedisRespError("temporary"))
						}
						requireMemoryAssembleCall(t, host)
						host.CallOnHttpCall(tc.headers, tc.response)
						require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
						require.Equal(t, body, host.GetRequestBody())
						digest, err := host.GetProperty([]string{memorycache.TrustedDigestProperty})
						if tc.allowed && (recentKnown || multiUser) {
							require.NoError(t, err)
							require.Equal(t, memoryEmptyContextDigest, string(digest))
						} else {
							require.Error(t, err, "Console failure/skip or an unknown dependent Recent cannot authorize an empty context digest")
						}
					})
				}
			}
		}
	})
}
