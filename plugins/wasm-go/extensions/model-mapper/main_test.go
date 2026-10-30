package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

const captureConfig = `{"capture_body_basis":true,"modelMapping":{"public.v1":"provider.v2","*":"no-wildcard-fallback"}}`

func mapperHeaders() [][2]string {
	return [][2]string{
		{":authority", "mapper.test"},
		{":path", "/v1/chat/completions"},
		{":method", "POST"},
		{"content-type", "application/json; charset=utf-8"},
		{"x-modelfusion-original-body-digest", "client-spoof-1"},
		{"X-Modelfusion-Original-Body-Digest", "client-spoof-2"},
		{"x-modelfusion-original-public-model", "client-model"},
		{"X-Modelfusion-Original-Public-Model", "client-model-2"},
		{"x-unrelated", "keep"},
	}
}

func requireNoBasis(t *testing.T, headers [][2]string) {
	t.Helper()
	for _, header := range headers {
		require.False(t, strings.EqualFold(header[0], bodybasis.DigestHeader))
		require.False(t, strings.EqualFold(header[0], bodybasis.PublicModelHeader))
	}
}

func TestMapperCapturesOriginalBytesBeforeExactRewrite(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(json.RawMessage(captureConfig))
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)
		raw := []byte(" {\n\"model\" : \"public.v1\", \"amount\":9007199254740993.0000000001, \"messages\":[{\"role\":\"user\",\"content\":\"Hi\"}]}\n")
		headers := append(mapperHeaders(), [2]string{"content-length", fmt.Sprint(len(raw))})
		require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(headers))
		require.Nil(t, host.GetLocalResponse())
		requireNoBasis(t, host.GetRequestHeaders())
		require.Equal(t, types.ActionContinue, host.CallOnHttpRequestBody(raw))
		require.Nil(t, host.GetLocalResponse())
		expectedBody := strings.Replace(string(raw), `"public.v1"`, `"provider.v2"`, 1)
		require.Equal(t, expectedBody, string(host.GetRequestBody()))
		basis, err := bodybasis.Read(host.GetRequestHeaders())
		require.NoError(t, err)
		canonical := []byte(`{"amount":9007199254740993.0000000001,"messages":[{"content":"Hi","role":"user"}],"model":"public.v1"}`)
		require.Equal(t, fmt.Sprintf("sha256:%x", sha256.Sum256(canonical)), basis.Digest())
		require.Equal(t, "public.v1", basis.PublicModel())
		require.NotEqual(t, bodybasis.DigestBytes(host.GetRequestBody()), basis.Digest())
		for _, header := range host.GetRequestHeaders() {
			require.False(t, strings.EqualFold(header[0], "content-length"))
		}
		require.Contains(t, host.GetRequestHeaders(), [2]string{"x-unrelated", "keep"})
		host.CompleteHttp()
	})
}

func TestMapperBasisCaptureRequiresExplicitBooleanOptIn(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for _, config := range []string{
			`{"modelMapping":{"public.v1":"provider.v2"}}`,
			`{"capture_body_basis":false,"modelMapping":{"public.v1":"provider.v2"}}`,
		} {
			t.Run(config, func(t *testing.T) {
				host, status := test.NewTestHost(json.RawMessage(config))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(mapperHeaders()))
				requireNoBasis(t, host.GetRequestHeaders())
				require.Equal(t, types.ActionContinue, host.CallOnHttpRequestBody([]byte(`{"model":"public.v1","messages":[]}`)))
				require.Nil(t, host.GetLocalResponse())
				require.JSONEq(t, `{"model":"provider.v2","messages":[]}`, string(host.GetRequestBody()))
				requireNoBasis(t, host.GetRequestHeaders())
				host.CompleteHttp()
			})
		}
	})
}

func TestMapperWaitsForCompleteBodyBeforePublishingBasis(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(json.RawMessage(captureConfig))
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)
		raw := []byte(`{"model":"public.v1","messages":[{"role":"user","content":"chunked"}]}`)
		require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(mapperHeaders()))
		require.Equal(t, types.ActionPause, host.CallOnHttpStreamingRequestBody(raw[:23], false))
		requireNoBasis(t, host.GetRequestHeaders())
		require.Nil(t, host.GetLocalResponse())
		require.Equal(t, types.ActionContinue, host.CallOnHttpStreamingRequestBody(raw[23:], true))
		basis, err := bodybasis.Read(host.GetRequestHeaders())
		require.NoError(t, err)
		canonical := []byte(`{"messages":[{"content":"chunked","role":"user"}],"model":"public.v1"}`)
		require.Equal(t, fmt.Sprintf("sha256:%x", sha256.Sum256(canonical)), basis.Digest())
		require.Equal(t, "public.v1", basis.PublicModel())
		require.Equal(t, strings.Replace(string(raw), `"public.v1"`, `"provider.v2"`, 1), string(host.GetRequestBody()))
		host.CompleteHttp()
	})
}

func TestMapperIdentityEncodingStillCapturesAndMapsThroughWrapper(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(json.RawMessage(captureConfig))
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)
		headers := append(mapperHeaders(), [2]string{"content-encoding", "identity"})
		require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(headers))
		raw := []byte(`{"model":"public.v1","messages":[]}`)
		require.Equal(t, types.ActionContinue, host.CallOnHttpRequestBody(raw))
		require.Nil(t, host.GetLocalResponse())
		require.Equal(t, `{"model":"provider.v2","messages":[]}`, string(host.GetRequestBody()))
		basis, err := bodybasis.Read(host.GetRequestHeaders())
		require.NoError(t, err)
		require.Equal(t, "public.v1", basis.PublicModel())
		require.Equal(t, fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(`{"messages":[],"model":"public.v1"}`))), basis.Digest())
		host.CompleteHttp()
	})
}

func TestMapperRejectsBodyWithoutAnExactMapping(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		cases := []struct {
			name   string
			body   []byte
			status uint32
		}{
			{"unmapped despite star entry", []byte(`{"model":"unmapped","messages":[]}`), 422},
			{"prefix is not exact", []byte(`{"model":"public","messages":[]}`), 422},
			{"missing model", []byte(`{"messages":[]}`), 400},
			{"nonstring model", []byte(`{"model":1}`), 400},
			{"duplicate model", []byte(`{"model":"public.v1","model":"unmapped"}`), 400},
			{"invalid JSON", []byte(`{"model":"public.v1"`), 400},
			{"empty body", nil, 400},
			{"over budget", []byte(strings.Repeat(" ", maxRequestBodyBytes+1)), 413},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				host, status := test.NewTestHost(json.RawMessage(captureConfig))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(mapperHeaders()))
				requireNoBasis(t, host.GetRequestHeaders())
				require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(tc.body))
				response := host.GetLocalResponse()
				require.NotNil(t, response)
				require.Equal(t, tc.status, response.StatusCode)
				requireNoBasis(t, host.GetRequestHeaders())
				require.Equal(t, string(tc.body), string(host.GetRequestBody()))
				host.CompleteHttp()
			})
		}
	})
}

func TestMapperRejectsUnsupportedRequestsAfterClearingSpoofedBasis(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		cases := []struct {
			name        string
			method      string
			contentType string
			encoding    string
			noBody      bool
			status      uint32
		}{
			{"method", "GET", "application/json", "", false, 405},
			{"media type", "POST", "text/plain", "", false, 415},
			{"encoding", "POST", "application/json", "gzip", false, 415},
			{"no body", "POST", "application/json", "", true, 400},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				host, status := test.NewTestHost(json.RawMessage(captureConfig))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				headers := mapperHeaders()
				headers[2][1] = tc.method
				headers[3][1] = tc.contentType
				if tc.encoding != "" {
					headers = append(headers, [2]string{"content-encoding", tc.encoding})
				}
				require.Equal(t, types.ActionPause, host.CallOnHttpRequestHeaders(headers, test.WithEndOfStream(tc.noBody)))
				requireNoBasis(t, host.GetRequestHeaders())
				require.NotNil(t, host.GetLocalResponse())
				require.Equal(t, tc.status, host.GetLocalResponse().StatusCode)
				host.CompleteHttp()
			})
		}
	})
}

func TestMapperRejectsAmbiguousControlledConfiguration(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for _, config := range []string{
			`{"capture_body_basis":"true","modelMapping":{"a":"b"}}`,
			`{"capture_body_basis":true,"capture_body_basis":false,"modelMapping":{"a":"b"}}`,
			`{"modelMapping":{"a":"b","a":"c"}}`,
			`{"modelMapping":{"a":null}}`,
			`{"modelMapping":{"a":"bad\r\nmodel"}}`,
		} {
			t.Run(config, func(t *testing.T) {
				host, status := test.NewTestHost(json.RawMessage(config))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusFailed, status)
			})
		}
	})
}
