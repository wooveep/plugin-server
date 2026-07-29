package main

import (
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"unsafe"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/config"
	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/provider"
	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/util"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	wasmhost "github.com/higress-group/wasm-go/pkg/test"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/stretchr/testify/require"
)

const (
	transparentBodylessUnknownTarget = "/api/v1/vendor/jobs/bodyless%2F42:cancel?sig=a%2Bb&dup=1&dup=2"
	transparentBodylessForcedTarget  = "/v1/chat/completions?sig=a%2Bb&dup=1&dup=2"
)

func TestTransparentBodylessAuthenticationContract(t *testing.T) {
	testCases := []struct {
		name              string
		config            string
		target            string
		expectedAuthority string
		expectedAuthName  string
		expectedAuthValue string
		expectSigV4       bool
	}{
		{
			name: "vertex_express_auto_unknown",
			config: `{"provider":{
				"type":"vertex",
				"apiTokens":["vertex-bodyless-contract-key"]
			}}`,
			target:            transparentBodylessUnknownTarget,
			expectedAuthority: "aiplatform.googleapis.com",
			expectedAuthName:  "x-goog-api-key",
			expectedAuthValue: "vertex-bodyless-contract-key",
		},
		{
			name: "vertex_express_forced",
			config: `{"protocol_mode":"passthrough","provider":{
				"type":"vertex",
				"apiTokens":["vertex-bodyless-contract-key"]
			}}`,
			target:            transparentBodylessForcedTarget,
			expectedAuthority: "aiplatform.googleapis.com",
			expectedAuthName:  "x-goog-api-key",
			expectedAuthValue: "vertex-bodyless-contract-key",
		},
		{
			name: "bedrock_bearer_auto_unknown",
			config: `{"provider":{
				"type":"bedrock",
				"apiTokens":["bedrock-bodyless-bearer-token"],
				"awsRegion":"us-east-1"
			}}`,
			target:            transparentBodylessUnknownTarget,
			expectedAuthority: "bedrock-runtime.us-east-1.amazonaws.com",
			expectedAuthName:  "authorization",
			expectedAuthValue: "Bearer bedrock-bodyless-bearer-token",
		},
		{
			name: "bedrock_bearer_forced",
			config: `{"protocol_mode":"passthrough","provider":{
				"type":"bedrock",
				"apiTokens":["bedrock-bodyless-bearer-token"],
				"awsRegion":"us-east-1"
			}}`,
			target:            transparentBodylessForcedTarget,
			expectedAuthority: "bedrock-runtime.us-east-1.amazonaws.com",
			expectedAuthName:  "authorization",
			expectedAuthValue: "Bearer bedrock-bodyless-bearer-token",
		},
		{
			name: "bedrock_aksk_auto_unknown",
			config: `{"provider":{
				"type":"bedrock",
				"awsAccessKey":"bedrock-bodyless-access-key",
				"awsSecretKey":"bedrock-bodyless-secret-key",
				"awsRegion":"us-east-1"
			}}`,
			target:            transparentBodylessUnknownTarget,
			expectedAuthority: "bedrock-runtime.us-east-1.amazonaws.com",
			expectedAuthName:  "authorization",
			expectSigV4:       true,
		},
		{
			name: "bedrock_aksk_forced",
			config: `{"protocol_mode":"passthrough","provider":{
				"type":"bedrock",
				"awsAccessKey":"bedrock-bodyless-access-key",
				"awsSecretKey":"bedrock-bodyless-secret-key",
				"awsRegion":"us-east-1"
			}}`,
			target:            transparentBodylessForcedTarget,
			expectedAuthority: "bedrock-runtime.us-east-1.amazonaws.com",
			expectedAuthName:  "authorization",
			expectSigV4:       true,
		},
	}

	wasmhost.RunTest(t, func(t *testing.T) {
		for _, testCase := range testCases {
			testCase := testCase
			t.Run(testCase.name, func(t *testing.T) {
				host, status := wasmhost.NewTestHost(json.RawMessage(testCase.config))
				t.Cleanup(host.Reset)
				require.Equal(t, types.OnPluginStartStatusOK, status)

				action := host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "client.example"},
					{":path", testCase.target},
					{":method", "DELETE"},
					{"authorization", "Bearer client-token"},
					{"x-api-key", "client-api-key"},
					{"x-goog-api-key", "client-google-key"},
					{"x-operation-marker", "preserve-bodyless"},
				}, wasmhost.WithEndOfStream(true))
				require.Equal(t, types.ActionContinue, action)

				headers := host.GetRequestHeaders()
				requireHeaderValue(t, headers, ":authority", testCase.expectedAuthority)
				requireHeaderValue(t, headers, ":method", "DELETE")
				requireHeaderValue(t, headers, ":path", testCase.target)
				requireHeaderValue(t, headers, "x-operation-marker", "preserve-bodyless")
				require.Empty(t, host.GetRequestBody())
				if testCase.expectSigV4 {
					authorization, found := wasmhost.GetHeaderValue(headers, testCase.expectedAuthName)
					require.True(t, found)
					require.Contains(t, authorization, "AWS4-HMAC-SHA256")
					require.Contains(t, authorization, "Credential=bedrock-bodyless-access-key/")
					amzDate, found := wasmhost.GetHeaderValue(headers, "x-amz-date")
					require.True(t, found)
					require.NotEmpty(t, amzDate)
				} else {
					requireHeaderValue(
						t,
						headers,
						testCase.expectedAuthName,
						testCase.expectedAuthValue,
					)
				}
			})
		}
	})
}

func TestTransparentBodylessVertexOAuthContract(t *testing.T) {
	authKey := newTransparentOAuthServiceAccount(t)
	testCases := []struct {
		name         string
		protocolMode string
		target       string
	}{
		{name: "auto_unknown", target: transparentBodylessUnknownTarget},
		{name: "forced", protocolMode: "passthrough", target: transparentBodylessForcedTarget},
	}

	wasmhost.RunTest(t, func(t *testing.T) {
		for _, testCase := range testCases {
			testCase := testCase
			t.Run(testCase.name, func(t *testing.T) {
				host, status := wasmhost.NewTestHost(transparentOAuthConfig(
					t,
					authKey,
					testCase.protocolMode,
				))
				t.Cleanup(host.Reset)
				require.Equal(t, types.OnPluginStartStatusOK, status)

				action := host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "client.example"},
					{":path", testCase.target},
					{":method", "DELETE"},
					{"authorization", "Bearer client-token"},
					{"x-operation-marker", "preserve-oauth-bodyless"},
				}, wasmhost.WithEndOfStream(true))
				require.Equal(t, types.ActionPause, action)

				callouts := host.GetHttpCalloutAttributes()
				require.Len(t, callouts, 1)
				require.Contains(t, callouts[0].Upstream, "transparent-contract-oauth")
				requireHeaderValue(t, callouts[0].Headers, ":method", http.MethodPost)
				requireHeaderValue(t, callouts[0].Headers, ":path", "/token")
				require.Contains(t, string(callouts[0].Body), "assertion=")

				const accessToken = "bodyless-transparent-oauth-access-token"
				host.CallOnHttpCall(
					[][2]string{{":status", "200"}, {"content-type", "application/json"}},
					[]byte(`{"access_token":"`+accessToken+`","expires_in":3600}`),
				)

				require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
				require.Nil(t, host.GetLocalResponse())
				headers := host.GetRequestHeaders()
				requireHeaderValue(t, headers, ":authority", "us-central1-aiplatform.googleapis.com")
				requireHeaderValue(t, headers, ":method", "DELETE")
				requireHeaderValue(t, headers, ":path", testCase.target)
				requireHeaderValue(t, headers, "authorization", "Bearer "+accessToken)
				requireHeaderValue(t, headers, "x-operation-marker", "preserve-oauth-bodyless")
				require.Empty(t, host.GetRequestBody())

				allLogs := collectAllHostLogs(host)
				for _, secret := range []string{
					"assertion=",
					"grant_type=",
					"transparent-service-account@example.iam.gserviceaccount.com",
					"transparent-contract-project",
					accessToken,
					testCase.target,
				} {
					require.NotContains(t, allLogs, secret)
				}
				require.NotContains(t, allLogs, string(callouts[0].Body))
			})
		}
	})
}

func TestTransparentVertexOAuthTerminalFailureContract(t *testing.T) {
	authKey := newTransparentOAuthServiceAccount(t)
	requestBody := []byte("transparent-oauth-request-body-marker")
	const providerErrorMarker = "oauth-provider-secret-marker"
	testCases := []struct {
		name            string
		responseHeaders [][2]string
		responseBody    []byte
	}{
		{
			name:            "non_200",
			responseHeaders: [][2]string{{":status", "400"}, {"content-type", "application/json"}},
			responseBody:    []byte(`{"error":"` + providerErrorMarker + `"}`),
		},
		{
			name:            "malformed_200",
			responseHeaders: [][2]string{{":status", "200"}, {"content-type", "application/json"}},
			responseBody:    []byte(`{"access_token":"` + providerErrorMarker),
		},
		{
			name:            "timeout_without_status",
			responseHeaders: nil,
			responseBody:    []byte(providerErrorMarker),
		},
	}

	wasmhost.RunTest(t, func(t *testing.T) {
		for _, testCase := range testCases {
			testCase := testCase
			t.Run(testCase.name, func(t *testing.T) {
				host, status := wasmhost.NewTestHost(transparentOAuthConfig(t, authKey, ""))
				t.Cleanup(host.Reset)
				require.Equal(t, types.OnPluginStartStatusOK, status)

				require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "client.example"},
					{":path", transparentAuthUnknownTarget},
					{":method", "PATCH"},
					{"authorization", "Bearer client-token"},
					{"content-type", "application/octet-stream"},
					{"x-operation-marker", "preserve-oauth-failure"},
				}))
				require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(requestBody))

				callouts := host.GetHttpCalloutAttributes()
				require.Len(t, callouts, 1)
				require.Contains(t, string(callouts[0].Body), "assertion=")
				host.CallOnHttpCall(testCase.responseHeaders, testCase.responseBody)

				localResponse := host.GetLocalResponse()
				require.NotNil(t, localResponse)
				require.Equal(t, uint32(http.StatusInternalServerError), localResponse.StatusCode)
				require.Equal(t, "ai-proxy.vertex.load_ak_failed", localResponse.StatusCodeDetail)
				require.Equal(t, []byte("failed to load vertex ak"), localResponse.Data)
				require.Equal(t, types.ActionPause, host.GetHttpStreamAction())

				headers := host.GetRequestHeaders()
				requireHeaderValue(t, headers, ":authority", "us-central1-aiplatform.googleapis.com")
				requireHeaderValue(t, headers, ":method", "PATCH")
				requireHeaderValue(t, headers, ":path", transparentAuthUnknownTarget)
				requireHeaderValue(t, headers, "content-type", "application/octet-stream")
				requireHeaderValue(t, headers, "x-operation-marker", "preserve-oauth-failure")
				_, hasAuthorization := wasmhost.GetHeaderValue(headers, "authorization")
				require.False(t, hasAuthorization)
				require.Equal(t, requestBody, host.GetRequestBody())

				allLogs := collectAllHostLogs(host)
				for _, secret := range []string{
					"assertion=",
					"grant_type=",
					"transparent-service-account@example.iam.gserviceaccount.com",
					"transparent-contract-project",
					providerErrorMarker,
					transparentAuthUnknownTarget,
					string(requestBody),
				} {
					require.NotContains(t, allLogs, secret)
				}
				require.NotContains(t, allLogs, string(callouts[0].Body))
			})
		}
	})
}

func TestTransparentVertexOAuthSuccessLogsExcludeSensitiveCalloutData(t *testing.T) {
	authKey := newTransparentOAuthServiceAccount(t)
	requestBody := []byte("transparent-oauth-success-request-body-marker")
	const accessToken = "transparent-oauth-success-access-token"

	wasmhost.RunTest(t, func(t *testing.T) {
		host, status := wasmhost.NewTestHost(transparentOAuthConfig(t, authKey, "passthrough"))
		t.Cleanup(host.Reset)
		require.Equal(t, types.OnPluginStartStatusOK, status)

		require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "client.example"},
			{":path", transparentAuthForcedTarget},
			{":method", "PATCH"},
			{"authorization", "Bearer client-token"},
			{"content-type", "application/octet-stream"},
			{"x-operation-marker", "preserve-oauth-success"},
		}))
		require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(requestBody))
		callouts := host.GetHttpCalloutAttributes()
		require.Len(t, callouts, 1)
		require.Contains(t, string(callouts[0].Body), "assertion=")

		host.CallOnHttpCall(
			[][2]string{{":status", "200"}, {"content-type", "application/json"}},
			[]byte(`{"access_token":"`+accessToken+`","expires_in":3600}`),
		)
		require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
		require.Nil(t, host.GetLocalResponse())
		requireHeaderValue(t, host.GetRequestHeaders(), "authorization", "Bearer "+accessToken)
		require.Equal(t, requestBody, host.GetRequestBody())

		allLogs := collectAllHostLogs(host)
		for _, secret := range []string{
			"assertion=",
			"grant_type=",
			"transparent-service-account@example.iam.gserviceaccount.com",
			"transparent-contract-project",
			accessToken,
			transparentAuthForcedTarget,
			string(requestBody),
		} {
			require.NotContains(t, allLogs, secret)
		}
		require.NotContains(t, allLogs, string(callouts[0].Body))
	})
}

func TestTransparentAuthenticationDispatchErrorIsTerminal(t *testing.T) {
	wasmhost.RunGoTest(t, func(t *testing.T) {
		host, status := wasmhost.NewTestHost(json.RawMessage(`{"provider":{
			"type":"generic",
			"genericHost":"https://provider.example",
			"apiTokens":["provider-token"]
		}}`))
		t.Cleanup(host.Reset)
		require.Equal(t, types.OnPluginStartStatusOK, status)

		originalErrorHandler := util.ErrorHandler
		errorHandlerCalls := 0
		util.ErrorHandler = func(statusCodeDetails string, err error) error {
			errorHandlerCalls++
			return originalErrorHandler(statusCodeDetails, err)
		}
		t.Cleanup(func() {
			util.ErrorHandler = originalErrorHandler
		})

		requestBody := []byte("dispatch-error-body-marker")
		require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "client.example"},
			{":path", transparentAuthUnknownTarget},
			{":method", "PATCH"},
			{"content-type", "application/octet-stream"},
			{"x-operation-marker", "preserve-dispatch-error"},
		}))
		currentContext := requireCurrentRequestHttpContext(t, host)
		contextValue := reflect.ValueOf(currentContext)
		require.Equal(t, reflect.Pointer, contextValue.Kind())
		configField := contextValue.Elem().FieldByName("config")
		require.True(t, configField.IsValid())
		require.False(t, configField.IsNil())
		pluginConfig := (*config.PluginConfig)(unsafe.Pointer(configField.Pointer()))
		pluginConfig.SetActiveProviderForTest(&transparentAuthDispatchErrorProvider{})

		require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(requestBody))
		require.Equal(t, 1, errorHandlerCalls)

		localResponse := host.GetLocalResponse()
		require.NotNil(t, localResponse)
		require.Equal(t, uint32(http.StatusInternalServerError), localResponse.StatusCode)
		require.Equal(
			t,
			"ai-proxy.transparent_request_authentication_failed",
			localResponse.StatusCodeDetail,
		)
		require.Equal(
			t,
			[]byte("failed to apply transparent request authentication"),
			localResponse.Data,
		)
		require.Equal(t, types.ActionPause, host.GetHttpStreamAction())
		requireHeaderValue(t, host.GetRequestHeaders(), ":method", "PATCH")
		requireHeaderValue(t, host.GetRequestHeaders(), ":path", transparentAuthUnknownTarget)
		requireHeaderValue(
			t,
			host.GetRequestHeaders(),
			"x-operation-marker",
			"preserve-dispatch-error",
		)
		require.Equal(t, requestBody, host.GetRequestBody())
	})
}

func TestTransparentBedrockBearerRetryReauthenticatesExactReplay(t *testing.T) {
	const (
		tokenA = "transparent-bedrock-retry-token-a"
		tokenB = "transparent-bedrock-retry-token-b"
	)
	requestBody := []byte{0x00, 0xff, 'b', 'e', 'd', 'r', 'o', 'c', 'k', 0x80, 0x7f}
	rawConfig := json.RawMessage(fmt.Sprintf(`{"provider":{
		"type":"bedrock",
		"apiTokens":[%q,%q],
		"awsRegion":"us-east-1",
		"retryOnFailure":{
			"enabled":true,
			"maxRetries":1,
			"retryOnStatus":["429"]
		}
	}}`, tokenA, tokenB))

	wasmhost.RunTest(t, func(t *testing.T) {
		host, status := wasmhost.NewTestHost(rawConfig)
		t.Cleanup(host.Reset)
		require.Equal(t, types.OnPluginStartStatusOK, status)

		require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "client.example"},
			{":path", transparentAuthUnknownTarget},
			{":method", "PATCH"},
			{"authorization", "Bearer client-token"},
			{"content-type", "application/octet-stream"},
			{"x-operation-marker", "preserve-bedrock-retry"},
		}))
		require.Equal(t, types.ActionContinue, host.CallOnHttpRequestBody(requestBody))
		firstAuthorization, found := wasmhost.GetHeaderValue(
			host.GetRequestHeaders(),
			"authorization",
		)
		require.True(t, found)
		require.Contains(t, []string{"Bearer " + tokenA, "Bearer " + tokenB}, firstAuthorization)

		require.NoError(t, host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
		require.Equal(t, types.HeaderStopAllIterationAndWatermark, host.CallOnHttpResponseHeaders(
			[][2]string{{":status", "429"}, {"content-type", "application/octet-stream"}},
		))

		callouts := host.GetHttpCalloutAttributes()
		require.Len(t, callouts, 1)
		retry := callouts[0]
		require.Equal(t, requestBody, retry.Body)
		requireHeaderValue(t, retry.Headers, ":authority", "bedrock-runtime.us-east-1.amazonaws.com")
		requireHeaderValue(t, retry.Headers, ":method", "PATCH")
		requireHeaderValue(t, retry.Headers, ":path", transparentAuthUnknownTarget)
		requireHeaderValue(t, retry.Headers, "content-type", "application/octet-stream")
		requireHeaderValue(t, retry.Headers, "x-operation-marker", "preserve-bedrock-retry")
		retryAuthorization, found := wasmhost.GetHeaderValue(retry.Headers, "authorization")
		require.True(t, found)
		require.Contains(t, []string{"Bearer " + tokenA, "Bearer " + tokenB}, retryAuthorization)
		require.NotEqual(t, firstAuthorization, retryAuthorization)
	})
}

func newTransparentOAuthServiceAccount(t *testing.T) string {
	t.Helper()
	privateKey, err := rsa.GenerateKey(cryptorand.Reader, 1024)
	require.NoError(t, err)
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	privateKeyPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	}))
	authKey, err := json.Marshal(map[string]string{
		"type":         "service_account",
		"client_email": "transparent-service-account@example.iam.gserviceaccount.com",
		"private_key":  privateKeyPEM,
		"token_uri":    "https://oauth2.googleapis.com/token",
	})
	require.NoError(t, err)
	return string(authKey)
}

func transparentOAuthConfig(t *testing.T, authKey, protocolMode string) json.RawMessage {
	t.Helper()
	rawConfig := map[string]any{
		"provider": map[string]any{
			"type":                  "vertex",
			"vertexAuthKey":         authKey,
			"vertexRegion":          "us-central1",
			"vertexProjectId":       "transparent-contract-project",
			"vertexAuthServiceName": "transparent-contract-oauth",
		},
	}
	if protocolMode != "" {
		rawConfig["protocol_mode"] = protocolMode
	}
	data, err := json.Marshal(rawConfig)
	require.NoError(t, err)
	return data
}

func requireCurrentRequestHttpContext(
	t *testing.T,
	host wasmhost.TestHost,
) wrapper.HttpContext {
	t.Helper()
	hostValue := reflect.ValueOf(host)
	require.Equal(t, reflect.Pointer, hostValue.Kind())
	hostValue = hostValue.Elem()
	contextIDField := hostValue.FieldByName("currentContextID")
	require.True(t, contextIDField.IsValid())
	contextID := uint32(contextIDField.Uint())
	httpContext := proxywasm.GetHttpContext(contextID)
	require.NotNil(t, httpContext)
	context, ok := httpContext.(wrapper.HttpContext)
	require.True(t, ok)
	return context
}

type transparentAuthDispatchErrorProvider struct{}

func (*transparentAuthDispatchErrorProvider) GetProviderType() string {
	return "vertex-oauth-dispatch-error"
}

func (*transparentAuthDispatchErrorProvider) OnRequestHeaders(
	ctx wrapper.HttpContext,
	apiName provider.ApiName,
) error {
	return nil
}

func (*transparentAuthDispatchErrorProvider) TransformTransparentRequestHeaders(
	ctx wrapper.HttpContext,
	headers http.Header,
) {
}

func (*transparentAuthDispatchErrorProvider) AuthenticateTransparentRequest(
	ctx wrapper.HttpContext,
	body []byte,
	headers http.Header,
) (types.Action, error) {
	return types.ActionContinue, errors.New("vertex oauth dispatch failed")
}

var _ provider.Provider = (*transparentAuthDispatchErrorProvider)(nil)
var _ provider.RequestHeadersHandler = (*transparentAuthDispatchErrorProvider)(nil)
var _ provider.TransparentRequestHeadersHandler = (*transparentAuthDispatchErrorProvider)(nil)
var _ provider.TransparentRequestBodyAuthHandler = (*transparentAuthDispatchErrorProvider)(nil)
