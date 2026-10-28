package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestStatisticsPreAuthMetadataOrder(t *testing.T) {
	type metadata struct {
		Spec struct {
			Phase    string `yaml:"phase"`
			Priority int    `yaml:"priority"`
		} `yaml:"spec"`
	}
	read := func(plugin string) metadata {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join("..", "..", "resources", "plugins", plugin, "spec.yaml"))
		require.NoError(t, err)
		var value metadata
		require.NoError(t, yaml.Unmarshal(raw, &value))
		return value
	}
	statistics, authentication := read("ai-statistics"), read("key-auth")
	require.Equal(t, "AUTHN", statistics.Spec.Phase)
	require.Equal(t, authentication.Spec.Phase, statistics.Spec.Phase)
	require.Greater(t, statistics.Spec.Priority, authentication.Spec.Priority,
		"statistics must enter before key-auth can send a local rejection")
}

func TestStatisticsPreAuthLifecycle(t *testing.T) {
	// This host runs the real statistics handlers in their callback order. The
	// changed map models key-auth's existing clear/install boundary; it does not
	// claim to run a multi-filter Envoy chain or authenticate these fixture IDs.
	test.RunTest(t, func(t *testing.T) {
		identity := balanceidentity.Snapshot{
			AccountID: "11111111-1111-4111-8111-111111111111", APIKeyID: "22222222-2222-4222-8222-222222222222",
			CredentialGeneration: "1", TenantID: "33333333-3333-4333-8333-333333333333", PrincipalID: "44444444-4444-4444-8444-444444444444",
		}
		cases := []struct {
			name          string
			status        string
			installNative bool
			requestBody   bool
			legacy        bool
			binary        bool
			usage         bool
			consumer      string
		}{
			{name: "missing credential local reply", status: "401", consumer: "none"},
			{name: "allow-list denial after identity installation", status: "403", installNative: true, consumer: "none"},
			{name: "allowed body uses authenticated principal", status: "200", installNative: true, requestBody: true, usage: true, consumer: identity.PrincipalID},
			{name: "public rule clears native fields but leaves legacy spoof", status: "200", requestBody: true, usage: true, consumer: "none"},
			{name: "unmanaged legacy cannot promote client native fields", status: "200", requestBody: true, usage: true, legacy: true, consumer: "none"},
			{name: "binary body remains unsubscribed", status: "200", installNative: true, requestBody: true, binary: true, usage: true, consumer: "none"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				config := json.RawMessage(`{"path_scope":"bound_target","enable_path_suffixes":[],"enable_methods":[],"enable_content_types":[],"use_default_attributes":false,"use_default_response_attributes":true,"disable_openai_usage":false}`)
				if tc.legacy {
					config = json.RawMessage(`{"path_scope":"legacy_suffixes","enable_path_suffixes":["/aliased/text"],"enable_methods":[],"enable_content_types":[],"use_default_attributes":false,"use_default_response_attributes":true,"disable_openai_usage":false}`)
				}
				host, start := test.NewTestHost(config)
				t.Cleanup(host.Reset)
				require.Equal(t, types.OnPluginStartStatusOK, start)
				id := host.InitializeHttpContext()
				require.NoError(t, host.SetRouteName("statistics-auth-route"))
				require.NoError(t, host.SetClusterName("statistics-auth-cluster"))
				contentType := "application/json"
				if tc.binary {
					contentType = "application/octet-stream"
				}
				incoming := [][2]string{{":authority", "fixture.example"}, {":method", "POST"}, {":path", "/aliased/text"}, {"content-type", contentType}, {ConsumerKey, "spoof-before-auth"}}
				// Even a syntactically valid native identity sent by the client must
				// not be frozen by statistics before key-auth runs.
				incoming = append(incoming, identity.Headers()...)
				require.Equal(t, types.ActionContinue, host.CallOnRequestHeaders(id, incoming, false))
				postAuth := [][2]string{{":authority", "fixture.example"}, {":method", "POST"}, {":path", "/aliased/text"}, {"content-type", contentType}, {ConsumerKey, "spoof-before-auth"}}
				if tc.installNative {
					postAuth[len(postAuth)-1][1] = "authenticated-consumer-name"
					postAuth = append(postAuth, identity.Headers()...)
				}
				if !tc.legacy {
					host.SetHttpRequestHeaders(id, postAuth)
				}
				if tc.requestBody {
					require.Equal(t, types.ActionContinue, host.CallOnRequestBody(id, []byte(`{"model":"public-model","messages":[]}`), true))
				}
				require.Equal(t, types.ActionContinue, host.CallOnResponseHeaders(id, [][2]string{{":status", tc.status}, {"content-type", "application/json"}}, false))
				body := []byte(`{"error":{"message":"fixture rejection"}}`)
				if tc.usage {
					body = []byte(`{"model":"provider-model","choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10}}`)
				}
				require.Equal(t, types.ActionContinue, host.CallOnResponseBody(id, body, true))
				host.CompleteHttpContext(id)
				errorClass := "none"
				if tc.status != "200" {
					errorClass = "http_4xx"
				}
				base := fmt.Sprintf("route.statistics-auth-route.path.provider_native_unknown.method.POST.status.%s.error.%s.metric.", tc.status, errorClass)
				count, err := host.GetCounterMetric(base + "request_count")
				require.NoError(t, err)
				require.Equal(t, uint64(1), count, "local replies and allowed calls each finish exactly once")
				duration, err := host.GetCounterMetric(base + "request_duration_ms")
				require.NoError(t, err)
				require.Positive(t, duration)
				if tc.usage {
					metric := "route.statistics-auth-route.upstream.statistics-auth-cluster.model.provider-model.consumer." + tc.consumer + ".metric.input_token"
					input, err := host.GetCounterMetric(metric)
					require.NoError(t, err)
					require.Equal(t, uint64(7), input)
				}
				_, err = host.GetCounterMetric("route.statistics-auth-route.upstream.statistics-auth-cluster.model.provider-model.consumer.spoof-before-auth.metric.input_token")
				require.Error(t, err, "client consumer must never become an authenticated usage label")
			})
		}
	})
}
