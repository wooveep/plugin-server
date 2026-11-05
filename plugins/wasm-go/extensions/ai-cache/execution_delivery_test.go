package main

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const nativeCacheOriginalCall = "77777777-7777-4777-8777-777777777777"
const nativeCacheOriginalTime = "2026-10-28T01:02:03.123456000Z"

func TestNativeCacheRejectsInvalidFrozenResponsePolicy(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for _, field := range []string{"protocol_mode", "response_profile"} {
			for _, value := range []string{`null`, `false`, `17`, `[]`, `{}`, `""`, `"typo"`} {
				t.Run(field+"/"+value, func(t *testing.T) {
					raw, err := sjson.SetRawBytes(nativeCacheExecutionConfig(t, true), field, []byte(value))
					require.NoError(t, err)
					host, status := test.NewTestHost(raw)
					defer host.Reset()
					require.Equal(t, types.OnPluginStartStatusFailed, status, "invalid frozen policy must fail configuration")
				})
			}
		}
	})
}

func nativeCacheExecutionConfig(t *testing.T, capture bool) json.RawMessage {
	t.Helper()
	descriptor := map[string]interface{}{
		"release_id": "opaque-release-without-prefix-semantics", "target_id": "original-target",
		"binding_id": "original-binding", "provider_id": "original-provider", "provider_account_id": "original-account",
		"region_endpoint_ref": "original-region", "service_ref": "mft-original.dns", "cluster_ref": "original-cluster",
		"credential_pool_ref": "original-credential-pool", "credential_pool_revision": "9007199254740993",
		"adapter_ref": "original-adapter", "adapter_revision": "1", "mapper_digest": "sha256:" + strings.Repeat("a", 64),
		"proxy_digest": "sha256:" + strings.Repeat("b", 64), "upstream_model": "different-upstream-model", "config_canonicalization": "mf-json.v1",
		"transport_contract": map[string]interface{}{"host": "provider.example.invalid", "sni": "provider.example.invalid", "path_adapter_ref": "original-adapter"},
		"capabilities":       map[string]interface{}{"text_generate": map[string]interface{}{"send_mode": "single_send", "response_classifier_ref": "original-classifier", "max_attempts": 1, "max_fallback_hops": 0}},
	}
	raw, err := json.Marshal(descriptor)
	require.NoError(t, err)
	digest, err := mfjson.Digest(raw)
	require.NoError(t, err)
	descriptor["descriptor_digest"] = digest
	raw = nativeCacheTestConfig(t)
	for path, value := range map[string]interface{}{
		"native_execution": map[string]interface{}{"producer_id": "configured-native-producer", "call_identity_source": "admission"},
		"_modelfusion_execution": map[string]interface{}{"schema_version": "modelfusion-execution.v1", "descriptor": descriptor, "route_id": nativeCacheTestRoute,
			"sales_asset_id": "88888888-8888-4888-8888-888888888888", "public_model": "public-alias", "credential_ref": "original-credential", "credential_revision": "1"},
		"capture_body_basis": capture, "consume_body_basis": !capture,
		"console_lookup":                     map[string]interface{}{"enabled": true, "service_name": thinConsoleService, "service_port": 8080, "path": thinConsolePath, "bearer_token": thinConsoleToken, "timeout": 500},
		"route_policy.enable_console_lookup": true,
	} {
		raw, err = sjson.SetBytes(raw, path, value)
		require.NoError(t, err)
	}
	return raw
}

func nativeCacheExecutionHeaders(capture bool) [][2]string {
	headers := nativeCacheTestHeaders(nativeCacheTestCaller())
	headers = append(headers, [2]string{executionfacts.AdmissionCallHeader, nativeCacheOriginalCall},
		[2]string{executionfacts.AdmissionReleaseHeader, "opaque-release-without-prefix-semantics"},
		[2]string{executionfacts.AdmissionDeadlineHeader, time.Now().UTC().Add(5 * time.Second).Format(time.RFC3339Nano)},
		[2]string{executionfacts.AdmissionOccurredHeader, nativeCacheOriginalTime})
	if capture {
		headers = append(headers, [2]string{bodybasis.DigestHeader, "client-forged"}, [2]string{bodybasis.PublicModelHeader, "client-forged"})
	}
	return headers
}

func nativeCacheExecutionRecord(t *testing.T, body []byte, stream bool) string {
	t.Helper()
	adapter, _ := thinProtocolAdapterForKind(protocol.ProtocolChatCompletions)
	digest, err := adapter.BuildCacheDigest(protocol.RequestParseInput{Method: "POST", Path: "/native/custom-answer", Body: body})
	require.NoError(t, err)
	record := MaterializedReplayRecord{
		SchemaVersion: materializedRecordSchemaVersion, Tenant: nativeCacheTestCaller().TenantID, Consumer: nativeCacheTestCaller().PrincipalID,
		Route: nativeCacheTestRoute, Model: "public-alias", Protocol: protocol.ProtocolChatCompletions, CacheScope: "consumer",
		CachePolicyVersion: strconv.FormatInt(nativeCacheTestVersion, 10), RequestDigest: digest.Digest,
		SoftExpiresAt: time.Now().Unix() + 60, HardExpiresAt: time.Now().Unix() + 3600, FinishReason: "stop",
		Usage:            json.RawMessage(`{"prompt_tokens":9007199254740993,"completion_tokens":0,"total_tokens":9007199254740993}`),
		Response:         json.RawMessage(`{"id":"retained-source-id","model":"public-alias","choices":[{"message":{"role":"assistant","content":"cached answer"},"finish_reason":"stop"}]}`),
		StreamReplayable: stream,
	}
	if stream {
		record.StreamChunks = []json.RawMessage{json.RawMessage(`{"choices":[{"delta":{"content":"cached answer"},"finish_reason":"stop"}]}`)}
	}
	raw, err := json.Marshal(record)
	require.NoError(t, err)
	return string(raw)
}

// The real native path registers the original call through its authenticated
// lookup before any Redis reply can bypass the upstream sender.
func completeNativeCacheRegistration(t *testing.T, host test.TestHost) {
	t.Helper()
	calls := host.GetHttpCalloutAttributes()
	require.Len(t, calls, 1)
	require.Empty(t, host.GetRedisCalloutAttributes(), "native registration precedes Redis replay")
	var request consoleLookupRequest
	require.NoError(t, json.Unmarshal(calls[0].Body, &request))
	require.Equal(t, nativeCacheOriginalCall, request.CallID)
	require.NotNil(t, request.CallOrigin)
	require.Equal(t, nativeCacheOriginalTime, request.CallOrigin.OccurredAt)
	require.NotNil(t, request.NativeIdentity)
	require.Equal(t, nativeCacheTestCaller(), request.NativeIdentity.Caller)
	host.CallOnHttpCall([][2]string{{":status", "200"}, {"content-type", "application/json"}}, []byte(`{"data":{"decision":"hit","materialized_key":"cache:materialized:registered-native"}}`))
	require.Len(t, host.GetRedisCalloutAttributes(), 1)
}

func replyNativeRegisteredCacheLookup(t *testing.T, host test.TestHost, body []byte, stream bool) {
	t.Helper()
	completeNativeCacheRegistration(t, host)
	host.CallOnRedisCall(0, test.CreateRedisRespString(nativeCacheExecutionRecord(t, body, stream)))
}

func TestNativeCacheReplayBridgesOnlyTheOriginalCallDelivery(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for _, stream := range []bool{false, true} {
			t.Run(strconv.FormatBool(stream), func(t *testing.T) {
				host, status := test.NewTestHost(nativeCacheExecutionConfig(t, true))
				require.Equal(t, types.OnPluginStartStatusOK, status)
				defer host.Reset()
				require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(nativeCacheExecutionHeaders(true)))
				body, err := sjson.SetBytes([]byte(nativeCacheTestBody), "stream", stream)
				require.NoError(t, err)
				require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(body))
				basis, err := bodybasis.Read(host.GetRequestHeaders())
				require.NoError(t, err)
				expected, err := bodybasis.Capture(body)
				require.NoError(t, err)
				require.Equal(t, expected, basis)
				replyNativeRegisteredCacheLookup(t, host, body, stream)
				response := host.GetLocalResponse()
				require.NotNil(t, response)
				require.Equal(t, uint32(200), response.StatusCode)
				bridge := thinReplayPayloadHeader(response.Headers, executionfacts.DeliveryBridgeHeader)
				require.NotEmpty(t, bridge)
				snapshot, err := executionfacts.DecodeDeliveryBridge(bridge)
				require.NoError(t, err)
				fact := snapshot.Envelope()
				require.Equal(t, "call_delivery", fact.Kind)
				require.Equal(t, nativeCacheOriginalCall, fact.CallID)
				require.Equal(t, nativeCacheOriginalTime, fact.OccurredAt)
				require.Equal(t, "cache", fact.ServedFrom)
				require.Equal(t, "estimator", fact.Usage.Source)
				require.Equal(t, "2", fact.Usage.Quantities["input"])
				require.Equal(t, "2", fact.Usage.Quantities["output"])
				require.Equal(t, "0", fact.Usage.Quantities["cache_hit"])
				require.Empty(t, fact.AttemptID)
				require.Empty(t, fact.TargetID)
				require.Empty(t, fact.BindingID)
				require.Empty(t, fact.SendState)
				require.Empty(t, host.GetHttpCalloutAttributes(), "cache hit must not reach the upstream sender")
				require.Empty(t, host.GetRedisCalloutAttributes(), "only billing appends the frozen bridge, cache must not double-write a FACT")
				require.False(t, bytes.Contains(response.Data, []byte("billing-facts.v1")))
			})
		}
	})
}

func TestNativeCacheCaptureStillRunsForAUserRequestedBypass(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(nativeCacheExecutionConfig(t, true))
		require.Equal(t, types.OnPluginStartStatusOK, status)
		defer host.Reset()
		headers := append(nativeCacheExecutionHeaders(true), [2]string{SKIP_CACHE_HEADER, "true"})
		require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(headers))
		require.Equal(t, types.ActionContinue, host.CallOnHttpRequestBody([]byte(nativeCacheTestBody)))
		basis, err := bodybasis.Read(host.GetRequestHeaders())
		require.NoError(t, err)
		require.Equal(t, "public-alias", basis.PublicModel())
		require.Empty(t, host.GetRedisCalloutAttributes())
		require.Empty(t, host.GetHttpCalloutAttributes())
	})
}

func TestNativeCachePassthroughRetainsBodyBasisAndBypassesResponseCache(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		raw := nativeCacheExecutionConfig(t, true)
		var err error
		raw, err = sjson.SetBytes(raw, "protocol_mode", "passthrough")
		require.NoError(t, err)
		raw, err = sjson.SetBytes(raw, "_modelfusion_execution.descriptor.upstream_model", "public-alias")
		require.NoError(t, err)
		descriptor, err := sjson.DeleteBytes([]byte(gjson.GetBytes(raw, "_modelfusion_execution.descriptor").Raw), "descriptor_digest")
		require.NoError(t, err)
		digest, err := mfjson.Digest(descriptor)
		require.NoError(t, err)
		raw, err = sjson.SetBytes(raw, "_modelfusion_execution.descriptor.descriptor_digest", digest)
		require.NoError(t, err)
		host, status := test.NewTestHost(raw)
		require.Equal(t, types.OnPluginStartStatusOK, status)
		defer host.Reset()
		require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(nativeCacheExecutionHeaders(true)))
		require.Equal(t, types.ActionContinue, host.CallOnHttpRequestBody([]byte(nativeCacheTestBody)))
		basis, err := bodybasis.Read(host.GetRequestHeaders())
		require.NoError(t, err)
		require.Equal(t, "public-alias", basis.PublicModel())
		require.Empty(t, host.GetRedisCalloutAttributes())
		require.Empty(t, host.GetHttpCalloutAttributes())
		original := []byte("  opaque upstream payload\n")
		host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "text/plain"}})
		host.CallOnHttpResponseBody(original)
		require.Equal(t, original, host.GetResponseBody())
		require.Empty(t, host.GetRedisCalloutAttributes())
	})
}

func TestNativeCacheRedisAdmissionHasNoSyntheticExpiry(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for _, capture := range []bool{true, false} {
			t.Run(strconv.FormatBool(capture), func(t *testing.T) {
				host, status := test.NewTestHost(nativeCacheExecutionConfig(t, capture))
				require.Equal(t, types.OnPluginStartStatusOK, status)
				defer host.Reset()
				body := []byte(nativeCacheTestBody)
				var headers [][2]string
				for _, header := range nativeCacheExecutionHeaders(capture) {
					if header[0] != executionfacts.AdmissionDeadlineHeader {
						headers = append(headers, header)
					}
				}
				if !capture {
					basis, err := bodybasis.Capture(body)
					require.NoError(t, err)
					headers = append(headers, basis.Headers()...)
				}
				require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(headers))
				require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(body))
				require.Nil(t, host.GetLocalResponse(), "an admitted Redis call can reach cache lookup")
				replyNativeRegisteredCacheLookup(t, host, body, false)
				response := host.GetLocalResponse()
				require.NotNil(t, response)
				require.Equal(t, uint32(200), response.StatusCode)
				snapshot, err := executionfacts.DecodeDeliveryBridge(thinReplayPayloadHeader(response.Headers, executionfacts.DeliveryBridgeHeader))
				require.NoError(t, err)
				require.Equal(t, nativeCacheOriginalCall, snapshot.Envelope().CallID)
				require.Equal(t, nativeCacheOriginalTime, snapshot.Envelope().OccurredAt)
				require.Empty(t, host.GetHttpCalloutAttributes())
				require.Equal(t, types.ActionContinue, host.CallOnHttpResponseHeaders(append(response.Headers, [2]string{":status", "200"})))
				require.Equal(t, types.ActionContinue, host.CallOnHttpStreamingResponseBody(response.Data, true))
				require.Empty(t, host.GetRedisCalloutAttributes(), "local cache replay cannot become a new upstream cache event")
			})
		}
	})
}

func TestNativeCacheConsumesTheEarlierOriginalBasisWithoutRecapturingInjectedBody(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(nativeCacheExecutionConfig(t, false))
		require.Equal(t, types.OnPluginStartStatusOK, status)
		defer host.Reset()
		basis, err := bodybasis.Capture([]byte(nativeCacheTestBody))
		require.NoError(t, err)
		headers := append(nativeCacheExecutionHeaders(false), basis.Headers()...)
		require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(headers))
		rewritten := []byte(`{"model":"public-alias","messages":[{"role":"system","content":"actual injected context"},{"role":"user","content":"weather?"}]}`)
		require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(rewritten))
		actual, err := bodybasis.Read(host.GetRequestHeaders())
		require.NoError(t, err)
		require.Equal(t, basis, actual)
		require.Equal(t, rewritten, host.GetRequestBody())
	})
}

func TestNativeCacheRejectsMissingCurrentAdmissionBeforeLookup(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for _, field := range []string{executionfacts.AdmissionCallHeader, executionfacts.AdmissionReleaseHeader, executionfacts.AdmissionOccurredHeader} {
			t.Run(field, func(t *testing.T) {
				host, status := test.NewTestHost(nativeCacheExecutionConfig(t, true))
				require.Equal(t, types.OnPluginStartStatusOK, status)
				defer host.Reset()
				var headers [][2]string
				for _, header := range nativeCacheExecutionHeaders(true) {
					if !strings.EqualFold(header[0], field) {
						headers = append(headers, header)
					}
				}
				require.Equal(t, types.ActionPause, host.CallOnHttpRequestHeaders(headers))
				require.NotNil(t, host.GetLocalResponse())
				require.Empty(t, host.GetRedisCalloutAttributes())
				require.Empty(t, host.GetHttpCalloutAttributes())
			})
		}
	})
}

func TestNativeCacheDeliveryRejectsPartialOrChangedPublicationConfiguration(t *testing.T) {
	for name, change := range map[string]func([]byte) []byte{
		"missing call origin": func(raw []byte) []byte {
			out, _ := sjson.DeleteBytes(raw, "native_execution.call_identity_source")
			return out
		},
		"wrong call origin": func(raw []byte) []byte {
			out, _ := sjson.SetBytes(raw, "native_execution.call_identity_source", "proxy")
			return out
		},
		"non-string call origin": func(raw []byte) []byte {
			out, _ := sjson.SetBytes(raw, "native_execution.call_identity_source", true)
			return out
		},
		"missing envelope": func(raw []byte) []byte { out, _ := sjson.DeleteBytes(raw, "_modelfusion_execution"); return out },
		"both body roles":  func(raw []byte) []byte { out, _ := sjson.SetBytes(raw, "consume_body_basis", true); return out },
		"no body role":     func(raw []byte) []byte { out, _ := sjson.SetBytes(raw, "capture_body_basis", false); return out },
		"wrong target": func(raw []byte) []byte {
			out, _ := sjson.SetBytes(raw, "native_identity.target_id", "another-target")
			return out
		},
		"stale descriptor": func(raw []byte) []byte {
			out, _ := sjson.SetBytes(raw, "_modelfusion_execution.descriptor.cluster_ref", "different-cluster")
			return out
		},
		"URL narrowing": func(raw []byte) []byte {
			out, _ := sjson.SetBytes(raw, "route_policy.enabled_path_suffixes", []string{"/v1/chat/completions"})
			return out
		},
	} {
		t.Run(name, func(t *testing.T) {
			raw := change(nativeCacheExecutionConfig(t, true))
			var cfg config.PluginConfig
			cfg.FromJson(gjson.ParseBytes(raw), nil)
			require.Error(t, configureNativeCacheDelivery(gjson.ParseBytes(raw), &cfg))
			require.Nil(t, cfg.NativeExecution, "failed parse must not inherit authority")
		})
	}
}

func nativeCachePoolExecutionConfig(t *testing.T) json.RawMessage {
	t.Helper()
	raw := nativeCacheExecutionConfig(t, true)
	for path, value := range map[string]interface{}{
		"_modelfusion_execution.schema_version": "modelfusion-execution.v2",
		"_modelfusion_execution.credentials": []map[string]interface{}{
			{"credential_ref": "original-credential", "credential_revision": "9007199254740993"},
			{"credential_ref": "original-credential-two", "credential_revision": "2"},
		},
	} {
		var err error
		raw, err = sjson.SetBytes(raw, path, value)
		require.NoError(t, err)
	}
	for _, name := range []string{"credential_ref", "credential_revision"} {
		var err error
		raw, err = sjson.DeleteBytes(raw, "_modelfusion_execution."+name)
		require.NoError(t, err)
	}
	return raw
}

func TestNativeCachePoolKeepsOriginalCallOwnershipAndNoUpstreamAttempt(t *testing.T) {
	original, pool := nativeCacheExecutionConfig(t, true), nativeCachePoolExecutionConfig(t)
	var prior, current config.PluginConfig
	prior.FromJson(gjson.ParseBytes(original), nil)
	current.FromJson(gjson.ParseBytes(pool), nil)
	require.NoError(t, configureNativeCacheDelivery(gjson.ParseBytes(original), &prior))
	require.NoError(t, configureNativeCacheDelivery(gjson.ParseBytes(pool), &current))
	require.Equal(t, *prior.NativeExecution.(*nativeCacheDeliveryConfig), *current.NativeExecution.(*nativeCacheDeliveryConfig),
		"a delivery has identical call/target ownership and no selected upstream seed")
	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(pool)
		require.Equal(t, types.OnPluginStartStatusOK, status)
		defer host.Reset()
		require.Equal(t, types.HeaderStopIteration, host.CallOnHttpRequestHeaders(nativeCacheExecutionHeaders(true)))
		body := []byte(nativeCacheTestBody)
		require.Equal(t, types.ActionPause, host.CallOnHttpRequestBody(body))
		replyNativeRegisteredCacheLookup(t, host, body, false)
		response := host.GetLocalResponse()
		require.NotNil(t, response)
		require.Equal(t, uint32(200), response.StatusCode)
		snapshot, err := executionfacts.DecodeDeliveryBridge(thinReplayPayloadHeader(response.Headers, executionfacts.DeliveryBridgeHeader))
		require.NoError(t, err)
		fact := snapshot.Envelope()
		require.Equal(t, "call_delivery", fact.Kind)
		require.Equal(t, nativeCacheOriginalCall, fact.CallID)
		require.Empty(t, fact.AttemptID)
		require.Empty(t, fact.CredentialRef)
		require.Empty(t, fact.CredentialRevision)
		require.Empty(t, host.GetHttpCalloutAttributes())
		require.Empty(t, host.GetRedisCalloutAttributes(), "cache must not append a second FACT")
	})
}

func TestNativeCachePoolRejectsMixedIncompleteOrUnversionedDirectories(t *testing.T) {
	original := nativeCachePoolExecutionConfig(t)
	changes := map[string]func(map[string]interface{}){
		"unversioned pool":  func(e map[string]interface{}) { e["schema_version"] = "modelfusion-execution.v1" },
		"unknown version":   func(e map[string]interface{}) { e["schema_version"] = "modelfusion-execution.v3" },
		"scalar mixed":      func(e map[string]interface{}) { e["credential_ref"] = "" },
		"scalar null mixed": func(e map[string]interface{}) { e["credential_revision"] = nil },
		"empty":             func(e map[string]interface{}) { e["credentials"] = []interface{}{} },
		"null":              func(e map[string]interface{}) { e["credentials"] = nil },
		"missing":           func(e map[string]interface{}) { delete(e, "credentials") },
		"duplicate ref": func(e map[string]interface{}) {
			e["credentials"].([]interface{})[1].(map[string]interface{})["credential_ref"] = "original-credential"
		},
		"missing ref": func(e map[string]interface{}) {
			delete(e["credentials"].([]interface{})[0].(map[string]interface{}), "credential_ref")
		},
		"secret": func(e map[string]interface{}) {
			e["credentials"].([]interface{})[0].(map[string]interface{})["secret"] = "not-an-allowed-field"
		},
		"number revision": func(e map[string]interface{}) {
			e["credentials"].([]interface{})[0].(map[string]interface{})["credential_revision"] = json.Number("9007199254740993")
		},
	}
	for _, revision := range []string{"0", "01", "-1", " 1", "340282366920938463463374607431768211456"} {
		value := revision
		changes["revision "+value] = func(e map[string]interface{}) {
			e["credentials"].([]interface{})[0].(map[string]interface{})["credential_revision"] = value
		}
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			var value map[string]interface{}
			require.NoError(t, json.Unmarshal(original, &value))
			change(value["_modelfusion_execution"].(map[string]interface{}))
			raw, err := json.Marshal(value)
			require.NoError(t, err)
			var cfg config.PluginConfig
			cfg.FromJson(gjson.ParseBytes(original), nil)
			require.NoError(t, configureNativeCacheDelivery(gjson.ParseBytes(original), &cfg))
			require.Error(t, configureNativeCacheDelivery(gjson.ParseBytes(raw), &cfg))
			require.Nil(t, cfg.NativeExecution, "failed parse must not retain authority")
		})
	}
	raw, err := sjson.SetBytes(nativeCacheExecutionConfig(t, true), "_modelfusion_execution.credentials", nil)
	require.NoError(t, err)
	var cfg config.PluginConfig
	cfg.FromJson(gjson.ParseBytes(raw), nil)
	require.Error(t, configureNativeCacheDelivery(gjson.ParseBytes(raw), &cfg))
	require.Nil(t, cfg.NativeExecution, "v1 must reject a pool alongside its original scalar")
}
