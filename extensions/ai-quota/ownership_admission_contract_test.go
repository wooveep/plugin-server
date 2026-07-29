package main

import (
	"fmt"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestQuotaAdmissionOwnershipContract(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		testCases := []struct {
			name        string
			path        string
			codeDetails string
			status      string
			contentType string
			chunks      [][]byte
			streaming   bool
		}{
			{
				name:        "recognizable JSON usage response",
				path:        "/v1/chat/completions",
				codeDetails: "via_upstream",
				status:      "200",
				contentType: "application/json",
				chunks: [][]byte{
					[]byte(`{"model":"gpt-4","usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18}}`),
				},
			},
			{
				name:        "opaque provider-native non-streaming response",
				path:        "/native/v9/generate",
				codeDetails: "via_upstream",
				status:      "202",
				contentType: "application/octet-stream",
				chunks: [][]byte{
					{0x00, 0xff, 0x10, 0x80, 0x7f},
				},
			},
			{
				name:        "streaming SSE usage response",
				path:        "/v1/chat/completions",
				codeDetails: "via_upstream",
				status:      "200",
				contentType: "text/event-stream",
				chunks: [][]byte{
					[]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n"),
					[]byte("data: {\"model\":\"gpt-4\",\"usage\":{\"prompt_tokens\":11,\"completion_tokens\":7,\"total_tokens\":18}}\n\n"),
				},
				streaming: true,
			},
			{
				name:        "upstream error response",
				path:        "/v1/chat/completions",
				codeDetails: "via_upstream",
				status:      "502",
				contentType: "application/json",
				chunks: [][]byte{
					[]byte(`{"model":"gpt-4","usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18},"error":{"message":"upstream failed"}}`),
				},
			},
			{
				name:        "gateway local response",
				path:        "/v1/chat/completions",
				codeDetails: "local_reply",
				status:      "429",
				contentType: "application/json",
				chunks: [][]byte{
					[]byte(`{"error":{"message":"local rate limit"}}`),
				},
			},
			{
				name:        "cache replay response",
				path:        "/v1/chat/completions",
				codeDetails: "ai-cache.hit",
				status:      "200",
				contentType: "application/json",
				chunks: [][]byte{
					[]byte(`{"model":"gpt-4","usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18}}`),
				},
			},
			{
				name:        "response without usage",
				path:        "/v1/chat/completions",
				codeDetails: "via_upstream",
				status:      "200",
				contentType: "application/json",
				chunks: [][]byte{
					[]byte(`{"choices":[{"message":{"content":"ok"}}]}`),
				},
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				config := monetaryConfig
				if testCase.path == "/native/v9/generate" {
					config = quotaConfigWith(map[string]interface{}{
						"enable_path_suffixes": []string{"/native/v9/generate"},
					})
				}
				host, status := test.NewTestHost(config)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				require.NoError(t, host.SetProperty(
					[]string{"response", "code_details"},
					[]byte(testCase.codeDetails),
				))

				action := host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "provider.example.com"},
					{":path", testCase.path},
					{":method", "POST"},
					{"x-tenant-id", "tenant-a"},
					{"x-consumer-id", "consumer-a"},
				})
				require.Equal(t, types.HeaderStopAllIterationAndWatermark, action)

				balanceKey := "billing:balance:tenant-a:global:consumer-a"
				admissionCalls := host.GetRedisCalloutAttributes()
				require.Len(t, admissionCalls, 1)
				require.Equal(t, "outbound|6379||redis.static", admissionCalls[0].Upstream)
				require.Equal(
					t,
					fmt.Sprintf("*2\r\n$3\r\nget\r\n$%d\r\n%s\r\n", len(balanceKey), balanceKey),
					string(admissionCalls[0].Query),
				)

				host.CallOnRedisCall(0, test.CreateRedisRespInt(1000))
				require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
				require.Nil(t, host.GetLocalResponse())
				require.Empty(t, host.GetRedisCalloutAttributes())

				responseHeaders := [][2]string{
					{":status", testCase.status},
					{"content-type", testCase.contentType},
				}
				for range 2 {
					action = host.CallOnHttpResponseHeaders(responseHeaders)
					require.Equal(t, types.ActionContinue, action)
					require.Equal(t, responseHeaders, host.GetResponseHeaders())
					require.Nil(t, host.GetLocalResponse())
					require.Empty(t, host.GetRedisCalloutAttributes())
				}

				if testCase.streaming {
					for index, chunk := range testCase.chunks {
						action = host.CallOnHttpStreamingResponseBody(
							chunk,
							index == len(testCase.chunks)-1,
						)
						require.Equal(t, types.ActionContinue, action)
						require.Equal(t, chunk, host.GetResponseBody())
						require.Nil(t, host.GetLocalResponse())
						require.Empty(t, host.GetRedisCalloutAttributes())
					}
					terminalChunk := testCase.chunks[len(testCase.chunks)-1]
					action = host.CallOnHttpStreamingResponseBody(terminalChunk, true)
					require.Equal(t, types.ActionContinue, action)
					require.Equal(t, terminalChunk, host.GetResponseBody())
					require.Nil(t, host.GetLocalResponse())
					require.Empty(t, host.GetRedisCalloutAttributes())
				} else {
					body := testCase.chunks[0]
					for range 2 {
						action = host.CallOnHttpResponseBody(body)
						require.Equal(t, types.ActionContinue, action)
						require.Equal(t, body, host.GetResponseBody())
						require.Nil(t, host.GetLocalResponse())
						require.Empty(t, host.GetRedisCalloutAttributes())
					}
				}

				host.CompleteHttp()
				require.Empty(t, host.GetRedisCalloutAttributes())
			})
		}
	})
}
