package main

import (
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	mathrand "math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	wasmhost "github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

const (
	transparentAuthUnknownTarget = "/api/v1/vendor/jobs/job%2F42:cancel?sig=a%2Bb&dup=1&dup=2"
	transparentAuthForcedTarget  = "/v1/chat/completions?sig=a%2Bb&dup=1&dup=2"
)

func TestTransparentBodyAuthenticationContract(t *testing.T) {
	requestBody := []byte{0x00, 0xff, 'a', 'u', 't', 'h', 0x80, 0x7f}
	testCases := []struct {
		name              string
		config            string
		target            string
		expectedAuthority string
		expectedAuthName  string
		expectedAuthValue string
	}{
		{
			name: "vertex_express_auto_unknown",
			config: `{"provider":{
				"type":"vertex",
				"apiTokens":["vertex-express-contract-key"]
			}}`,
			target:            transparentAuthUnknownTarget,
			expectedAuthority: "aiplatform.googleapis.com",
			expectedAuthName:  "x-goog-api-key",
			expectedAuthValue: "vertex-express-contract-key",
		},
		{
			name: "vertex_express_forced",
			config: `{"protocol_mode":"passthrough","provider":{
				"type":"vertex",
				"apiTokens":["vertex-express-contract-key"]
			}}`,
			target:            transparentAuthForcedTarget,
			expectedAuthority: "aiplatform.googleapis.com",
			expectedAuthName:  "x-goog-api-key",
			expectedAuthValue: "vertex-express-contract-key",
		},
		{
			name: "bedrock_bearer_auto_unknown",
			config: `{"provider":{
				"type":"bedrock",
				"apiTokens":["bedrock-bearer-contract-token"],
				"awsRegion":"us-east-1"
			}}`,
			target:            transparentAuthUnknownTarget,
			expectedAuthority: "bedrock-runtime.us-east-1.amazonaws.com",
			expectedAuthName:  "authorization",
			expectedAuthValue: "Bearer bedrock-bearer-contract-token",
		},
		{
			name: "bedrock_bearer_forced",
			config: `{"protocol_mode":"passthrough","provider":{
				"type":"bedrock",
				"apiTokens":["bedrock-bearer-contract-token"],
				"awsRegion":"us-east-1"
			}}`,
			target:            transparentAuthForcedTarget,
			expectedAuthority: "bedrock-runtime.us-east-1.amazonaws.com",
			expectedAuthName:  "authorization",
			expectedAuthValue: "Bearer bedrock-bearer-contract-token",
		},
	}

	wasmhost.RunTest(t, func(t *testing.T) {
		for _, testCase := range testCases {
			testCase := testCase
			t.Run(testCase.name, func(t *testing.T) {
				host, status := wasmhost.NewTestHost(json.RawMessage(testCase.config))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)

				require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "client.example"},
					{":path", testCase.target},
					{":method", "PATCH"},
					{"authorization", "Bearer client-token"},
					{"x-api-key", "client-api-key"},
					{"x-goog-api-key", "client-google-key"},
					{"content-type", "application/octet-stream"},
					{"x-operation-marker", "preserve-me"},
				}))
				require.Equal(t, types.ActionContinue, host.CallOnHttpRequestBody(requestBody))

				headers := host.GetRequestHeaders()
				requireHeaderValue(t, headers, ":authority", testCase.expectedAuthority)
				requireHeaderValue(t, headers, ":method", "PATCH")
				requireHeaderValue(t, headers, ":path", testCase.target)
				requireHeaderValue(t, headers, testCase.expectedAuthName, testCase.expectedAuthValue)
				requireHeaderValue(t, headers, "content-type", "application/octet-stream")
				requireHeaderValue(t, headers, "x-operation-marker", "preserve-me")
				require.Equal(t, requestBody, host.GetRequestBody())
			})
		}
	})
}

func TestTransparentBedrockSigV4BodyAuthenticationContract(t *testing.T) {
	requestBody := []byte{0x00, 0xff, 's', 'i', 'g', 'v', '4', 0x80, 0x7f}
	testCases := []struct {
		name   string
		config string
		target string
	}{
		{
			name: "auto_unknown",
			config: `{"provider":{
				"type":"bedrock",
				"awsAccessKey":"bedrock-contract-access-key",
				"awsSecretKey":"bedrock-contract-secret-key",
				"awsRegion":"us-east-1"
			}}`,
			target: transparentAuthUnknownTarget,
		},
		{
			name: "forced",
			config: `{"protocol_mode":"passthrough","provider":{
				"type":"bedrock",
				"awsAccessKey":"bedrock-contract-access-key",
				"awsSecretKey":"bedrock-contract-secret-key",
				"awsRegion":"us-east-1"
			}}`,
			target: transparentAuthForcedTarget,
		},
	}

	wasmhost.RunTest(t, func(t *testing.T) {
		for _, testCase := range testCases {
			testCase := testCase
			t.Run(testCase.name, func(t *testing.T) {
				host, status := wasmhost.NewTestHost(json.RawMessage(testCase.config))
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)

				require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "client.example"},
					{":path", testCase.target},
					{":method", "PATCH"},
					{"authorization", "Bearer client-token"},
					{"content-type", "application/octet-stream"},
					{"x-operation-marker", "preserve-me"},
				}))
				require.Equal(t, types.ActionContinue, host.CallOnHttpRequestBody(requestBody))

				headers := host.GetRequestHeaders()
				requireHeaderValue(t, headers, ":authority", "bedrock-runtime.us-east-1.amazonaws.com")
				requireHeaderValue(t, headers, ":method", "PATCH")
				requireHeaderValue(t, headers, ":path", testCase.target)
				requireHeaderValue(t, headers, "content-type", "application/octet-stream")
				requireHeaderValue(t, headers, "x-operation-marker", "preserve-me")
				authorization, hasAuthorization := wasmhost.GetHeaderValue(headers, "authorization")
				require.True(t, hasAuthorization)
				require.Contains(t, authorization, "AWS4-HMAC-SHA256")
				require.Contains(t, authorization, "Credential=bedrock-contract-access-key/")
				amzDate, hasAmzDate := wasmhost.GetHeaderValue(headers, "x-amz-date")
				require.True(t, hasAmzDate)
				require.NotEmpty(t, amzDate)
				require.Equal(t, requestBody, host.GetRequestBody())
			})
		}
	})
}

func TestTransparentVertexOAuthBodyAuthenticationContract(t *testing.T) {
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
		"client_email": "transparent-contract@example.iam.gserviceaccount.com",
		"private_key":  privateKeyPEM,
		"token_uri":    "https://oauth2.googleapis.com/token",
	})
	require.NoError(t, err)

	testCases := []struct {
		name         string
		protocolMode string
		target       string
	}{
		{name: "auto_unknown", target: transparentAuthUnknownTarget},
		{name: "forced", protocolMode: "passthrough", target: transparentAuthForcedTarget},
	}

	wasmhost.RunTest(t, func(t *testing.T) {
		for _, testCase := range testCases {
			testCase := testCase
			t.Run(testCase.name, func(t *testing.T) {
				rawConfig := map[string]any{
					"provider": map[string]any{
						"type":                  "vertex",
						"vertexAuthKey":         string(authKey),
						"vertexRegion":          "us-central1",
						"vertexProjectId":       "transparent-contract-project",
						"vertexAuthServiceName": "transparent-contract-oauth",
					},
				}
				if testCase.protocolMode != "" {
					rawConfig["protocol_mode"] = testCase.protocolMode
				}
				config, marshalErr := json.Marshal(rawConfig)
				require.NoError(t, marshalErr)

				host, status := wasmhost.NewTestHost(config)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)

				requestBody := []byte{0x00, 0xff, 'o', 'a', 'u', 't', 'h', 0x80, 0x7f}
				require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "client.example"},
					{":path", testCase.target},
					{":method", "PATCH"},
					{"authorization", "Bearer client-token"},
					{"content-type", "application/octet-stream"},
					{"x-operation-marker", "preserve-me"},
				}))
				require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(requestBody))

				callouts := host.GetHttpCalloutAttributes()
				require.Len(t, callouts, 1)
				require.Contains(t, callouts[0].Upstream, "transparent-contract-oauth")
				host.CallOnHttpCall(
					[][2]string{{":status", "200"}, {"content-type", "application/json"}},
					[]byte(`{"access_token":"transparent-contract-oauth-token","expires_in":3600}`),
				)

				headers := host.GetRequestHeaders()
				requireHeaderValue(t, headers, ":authority", "us-central1-aiplatform.googleapis.com")
				requireHeaderValue(t, headers, ":method", "PATCH")
				requireHeaderValue(t, headers, ":path", testCase.target)
				requireHeaderValue(t, headers, "authorization", "Bearer transparent-contract-oauth-token")
				requireHeaderValue(t, headers, "content-type", "application/octet-stream")
				requireHeaderValue(t, headers, "x-operation-marker", "preserve-me")
				require.Equal(t, requestBody, host.GetRequestBody())
			})
		}
	})
}

func TestTransparentVertexExpressRetryReauthenticatesOpaqueReplay(t *testing.T) {
	const (
		tokenA = "transparent-vertex-retry-key-a"
		tokenB = "transparent-vertex-retry-key-b"
	)
	requestBody := []byte{0x00, 0xff, 'r', 'e', 't', 'r', 'y', 0x80, 0x7f}
	config := json.RawMessage(fmt.Sprintf(`{"provider":{
		"type":"vertex",
		"apiTokens":[%q,%q],
		"retryOnFailure":{
			"enabled":true,
			"maxRetries":1,
			"retryOnStatus":["429"]
		}
	}}`, tokenA, tokenB))

	wasmhost.RunTest(t, func(t *testing.T) {
		host, status := wasmhost.NewTestHost(config)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		require.Equal(t, types.ActionContinue, host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "client.example"},
			{":path", transparentAuthUnknownTarget},
			{":method", "PATCH"},
			{"content-type", "application/octet-stream"},
			{"x-operation-marker", "preserve-me"},
		}))
		require.Equal(t, types.ActionContinue, host.CallOnHttpRequestBody(requestBody))
		firstKey, hasFirstKey := wasmhost.GetHeaderValue(host.GetRequestHeaders(), "x-goog-api-key")
		require.True(t, hasFirstKey)
		require.Contains(t, []string{tokenA, tokenB}, firstKey)

		require.NoError(t, host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
		require.Equal(t, types.HeaderStopAllIterationAndWatermark, host.CallOnHttpResponseHeaders(
			[][2]string{{":status", "429"}, {"content-type", "application/octet-stream"}},
		))

		callouts := host.GetHttpCalloutAttributes()
		require.Len(t, callouts, 1)
		retry := callouts[0]
		require.Equal(t, requestBody, retry.Body)
		requireHeaderValue(t, retry.Headers, ":authority", "aiplatform.googleapis.com")
		requireHeaderValue(t, retry.Headers, ":method", "PATCH")
		requireHeaderValue(t, retry.Headers, ":path", transparentAuthUnknownTarget)
		requireHeaderValue(t, retry.Headers, "content-type", "application/octet-stream")
		requireHeaderValue(t, retry.Headers, "x-operation-marker", "preserve-me")
		retryKey, hasRetryKey := wasmhost.GetHeaderValue(retry.Headers, "x-goog-api-key")
		require.True(t, hasRetryKey)
		require.Contains(t, []string{tokenA, tokenB}, retryKey)
		require.NotEqual(t, firstKey, retryKey)
	})
}

func TestVertexBodyLifecycleUsesHeaderSelectedCredentialExactly(t *testing.T) {
	testCases := []struct {
		name   string
		target string
		body   string
		tokens []string
		raw    bool
	}{
		{
			name:   "google",
			target: "/v1/chat/completions",
			body:   `{"model":"gemini-2.5-flash","messages":[{"role":"user","content":"exact selection"}]}`,
			tokens: []string{"vertex-google-selection-a", "vertex-google-selection-b"},
		},
		{
			name:   "anthropic",
			target: "/v1/chat/completions",
			body:   `{"model":"claude-sonnet-4@20250514","messages":[{"role":"user","content":"exact selection"}]}`,
			tokens: []string{"vertex-anthropic-selection-a", "vertex-anthropic-selection-b"},
		},
		{
			name:   "raw",
			target: "/v1/projects/test/locations/us-central1/publishers/google/models/gemini-2.0-flash:generateContent",
			body:   `{"contents":[{"role":"user","parts":[{"text":"exact selection"}]}]}`,
			tokens: []string{"vertex-raw-selection-a", "vertex-raw-selection-b"},
			raw:    true,
		},
	}

	wasmhost.RunGoTest(t, func(t *testing.T) {
		for _, testCase := range testCases {
			testCase := testCase
			t.Run(testCase.name, func(t *testing.T) {
				providerConfig := map[string]any{
					"id":        "exact-selection-" + testCase.name,
					"type":      "vertex",
					"apiTokens": testCase.tokens,
				}
				if testCase.raw {
					providerConfig["protocol"] = "original"
				}
				config, err := json.Marshal(map[string]any{"provider": providerConfig})
				require.NoError(t, err)

				seed := int64(1)
				for {
					random := mathrand.New(mathrand.NewSource(seed))
					if random.Intn(len(testCase.tokens)) != random.Intn(len(testCase.tokens)) {
						break
					}
					seed++
				}
				//lint:ignore SA1019 The regression target uses package-level math/rand token selection.
				mathrand.Seed(seed)

				host, status := wasmhost.NewTestHost(config)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "client.example"},
					{":path", testCase.target},
					{":method", "POST"},
					{"content-type", "application/json"},
				})

				contextKey := fmt.Sprintf("vertex-%s-apiTokenInUse", "exact-selection-"+testCase.name)
				headerSelected := requireCurrentRequestContextString(t, host, contextKey)
				require.Contains(t, testCase.tokens, headerSelected)
				require.Equal(t, types.ActionContinue, host.CallOnHttpRequestBody([]byte(testCase.body)))

				pathValue, hasPath := wasmhost.GetHeaderValue(host.GetRequestHeaders(), ":path")
				require.True(t, hasPath)
				keyMarker := "key="
				keyIndex := strings.Index(pathValue, keyMarker)
				require.NotEqual(t, -1, keyIndex)
				keyValue := pathValue[keyIndex+len(keyMarker):]
				if nextParameter := strings.IndexByte(keyValue, '&'); nextParameter >= 0 {
					keyValue = keyValue[:nextParameter]
				}
				require.Equal(t, headerSelected, keyValue,
					"body-stage authentication must reuse the exact credential selected during headers")
				allLogs := collectAllHostLogs(host)
				for _, credential := range testCase.tokens {
					require.NotContains(t, allLogs, credential)
				}
			})
		}
	})
}

func requireCurrentRequestContextString(t *testing.T, host wasmhost.TestHost, key string) string {
	t.Helper()
	hostValue := reflect.ValueOf(host)
	require.Equal(t, reflect.Pointer, hostValue.Kind())
	hostValue = hostValue.Elem()
	contextIDField := hostValue.FieldByName("currentContextID")
	require.True(t, contextIDField.IsValid())
	contextID := uint32(contextIDField.Uint())

	httpContext := proxywasm.GetHttpContext(contextID)
	require.NotNil(t, httpContext)
	contextReader, ok := httpContext.(interface {
		GetContext(string) interface{}
	})
	require.True(t, ok)
	value, ok := contextReader.GetContext(key).(string)
	require.True(t, ok)
	require.NotEmpty(t, value)
	return value
}
