package main

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/higress-group/wasm-go/pkg/ai/resourceref"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/resp"
)

func nativeResourceFixture() map[string]interface{} {
	fixture := managedConfigFixture()
	settings := fixture["managed_execution"].(managedSettings)
	settings.InputProtocol, settings.UpstreamProtocol = managedProtocolResponses, managedProtocolResponses
	fixture["managed_execution"] = settings
	return fixture
}

func TestNativeResourceWithoutOptionalAccountUsesSelectedCredentialBoundary(t *testing.T) {
	fixture := nativeResourceFixture()
	release := fixture["_modelfusion_execution"].(managedRelease)
	release.Descriptor.ProviderAccountID = ""
	var descriptor map[string]interface{}
	raw, err := json.Marshal(release.Descriptor)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &descriptor))
	release.Descriptor.DescriptorDigest, err = managedDescriptorDigest(managedFixtureJSON(t, descriptor))
	require.NoError(t, err)
	fixture["_modelfusion_execution"] = release
	h := newManagedHost(t, fixture)
	original := []byte(`{"model":"public-model","input":"a"}`)
	h.start(t, original, bytes.Replace(original, []byte("public-model"), []byte("upstream-model"), 1))
	h.rawResponse(t, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, []byte(`{"id":"native-no-account","object":"response","model":"upstream-model","status":"completed","output":[],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`))
	require.Nil(t, h.host.GetSentLocalResponse(h.id))
	lookups := h.host.GetRedisCalloutAttributesFromContext(h.id)
	require.Len(t, lookups, 1, "a valid descriptor must persist its alias before delivery")
	require.Contains(t, string(lookups[0].Query), "credential:credential-1")
	require.NotContains(t, string(lookups[0].Query), "controlled-upstream-key")
	h.host.CallOnRedisCallResponse(lookups[0].CalloutID, 0, []byte(":1\r\n"))
	require.EqualValues(t, 200, h.host.GetSentLocalResponse(h.id).StatusCode)
}

func TestResourceCredentialBoundaryFiltersPoolWithoutChangingOrdinarySelection(t *testing.T) {
	c, err := decodeManagedPoolFixture(t, managedPoolFixture(t))
	require.NoError(t, err)
	c.release.Descriptor.ProviderAccountID = ""
	c.profile, err = newManagedProtocolProfile(managedProtocolResponses, managedProtocolResponses)
	require.NoError(t, err)
	members := []managedHealthMember{{credential: c.settings.Credentials[0]}, {credential: c.settings.Credentials[1]}}
	state := &managedExecutionState{config: c}
	require.Len(t, managedTextUntried(state, members), 2)
	owner := managedResourceOwner(c)
	owner.Account = "credential:credential-1"
	state.resourceOwner = &owner
	require.NoError(t, selectManagedResourceOwner(state, owner))
	eligible := managedTextUntried(state, members)
	require.Len(t, eligible, 1)
	require.Equal(t, "credential-1", eligible[0].credential.CredentialRef)
	owner.Account = "credential:foreign"
	require.Error(t, selectManagedResourceOwner(state, owner))
}

func responseResourceDirectoryFixture(t *testing.T, initial int, recovery bool) map[string]interface{} {
	t.Helper()
	fixture := managedTextFixture(t, true)
	directory := fixture[managedTextDirectoryField].(managedTextDirectory)
	directory.ResourceAliases = &managedResourceAliasSettings{SchemaVersion: "modelfusion-public-resources.v1", RetentionSeconds: resourceref.RetentionSeconds}
	for i := range directory.Targets {
		entry := &directory.Targets[i]
		var proxy, envelope map[string]interface{}
		require.NoError(t, json.Unmarshal(entry.Proxy, &proxy))
		require.NoError(t, json.Unmarshal(entry.Execution, &envelope))
		settings := proxy["managed_execution"].(map[string]interface{})
		settings["input_protocol"], settings["upstream_protocol"] = managedProtocolResponses, managedProtocolResponses
		if recovery {
			policy := settings["text_execution"].(map[string]interface{})
			delete(policy, "retry")
			if i == 1 {
				policy["retry"] = map[string]interface{}{"max_retries": 1, "retry_timeout_ms": 5000, "retry_on_status": []string{"503"}}
			}
		} else {
			delete(settings, "text_execution")
			envelope["descriptor"].(map[string]interface{})["capabilities"] = map[string]managedCapability{"text_generate": {SendMode: "single_send", ResponseClassifierRef: "openai-text-v1", MaxAttempts: 1}}
		}
		entry.Proxy = managedFixtureJSON(t, proxy)
		descriptor := envelope["descriptor"].(map[string]interface{})
		digest, err := mfjson.Digest(entry.Proxy)
		require.NoError(t, err)
		descriptor["proxy_digest"] = digest
		descriptor["descriptor_digest"], err = managedDescriptorDigest(managedFixtureJSON(t, descriptor))
		require.NoError(t, err)
		entry.Execution = managedFixtureJSON(t, envelope)
	}
	var selected map[string]interface{}
	require.NoError(t, json.Unmarshal(directory.Targets[initial].Proxy, &selected))
	selected["_modelfusion_execution"] = json.RawMessage(directory.Targets[initial].Execution)
	selected[managedTextDirectoryField] = directory
	return selected
}

func respondResourceLookup(t *testing.T, h *managedHost, alias, original string, owner resourceref.Owner) {
	t.Helper()
	lookups := h.host.GetRedisCalloutAttributesFromContext(h.id)
	require.Len(t, lookups, 1)
	record := resourceref.NewRecord(clientresponse.ResourceAlias{Kind: "response", Public: alias, Original: original}, owner, true)
	raw, err := json.Marshal(record)
	require.NoError(t, err)
	var reply bytes.Buffer
	require.NoError(t, resp.NewWriter(&reply).WriteArray([]resp.Value{resp.StringValue(string(raw))}))
	h.host.CallOnRedisCallResponse(lookups[0].CalloutID, 0, reply.Bytes())
}

func TestNativeResourceOwnerIsSelectedBeforeWeightedTargetSend(t *testing.T) {
	h := newManagedTextHost(t, responseResourceDirectoryFixture(t, 1, false))
	config := h.cfg.ManagedExecution.(*managedExecutionConfig)
	owner := managedResourceOwner(config.targets["target-1"])
	const alias = "resp_pf_11111111111111111111111111111111"
	original := []byte(`{"model":"public-model","input":"next","previous_response_id":"` + alias + `"}`)
	h.start(t, original, bytes.Replace(original, []byte("public-model"), []byte("second-upstream-model"), 1))
	require.Empty(t, h.host.GetCalloutAttributesFromContext(h.id))
	respondResourceLookup(t, h, alias, "private-native-A", owner)
	respondManagedTextHealth(t, h, map[string]bool{"credential-1": true})
	sends := h.host.GetCalloutAttributesFromContext(h.id)
	require.Len(t, sends, 1)
	require.Equal(t, "private-native-A", gjson.GetBytes(sends[0].Body, "previous_response_id").String())
	require.Equal(t, "upstream-model", gjson.GetBytes(sends[0].Body, "model").String())
	require.NotContains(t, sends[0].Upstream, "second-provider")
	state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
	require.Nil(t, state.text, "continuation selection must not enable recovery")
	require.Equal(t, "router", managedTextMechanism(state))
}

func TestNativeReferenceIsNotSentToDifferentFallbackOwner(t *testing.T) {
	h := newManagedTextHost(t, responseResourceDirectoryFixture(t, 0, true))
	config := h.cfg.ManagedExecution.(*managedExecutionConfig)
	owner := managedResourceOwner(config)
	const alias = "resp_pf_22222222222222222222222222222222"
	original := []byte(`{"model":"public-model","input":"next","previous_response_id":"` + alias + `"}`)
	h.start(t, original, bytes.Replace(original, []byte("public-model"), []byte("upstream-model"), 1))
	respondResourceLookup(t, h, alias, "private-native-A", owner)
	respondManagedTextHealth(t, h, map[string]bool{"credential-1": true})
	sends := h.host.GetCalloutAttributesFromContext(h.id)
	require.Len(t, sends, 1)
	h.host.CallOnHttpCallResponse(sends[0].CalloutID, [][2]string{{":status", "503"}, {"content-type", "application/json"}}, nil, []byte(`{"error":{"code":"server_error","message":"private supplier failure"}}`))
	require.Empty(t, h.host.GetRedisCalloutAttributesFromContext(h.id), "foreign owner must not even enter fallback qualification")
	require.Empty(t, h.host.GetCalloutAttributesFromContext(h.id))
	response := h.host.GetSentLocalResponse(h.id)
	require.NotNil(t, response)
	require.EqualValues(t, 503, response.StatusCode)
	require.NotContains(t, string(response.Data), "private")
	state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
	require.Len(t, state.text.attempts, 1)
}

func TestNativeResponseAliasesAreSavedBeforePublicDelivery(t *testing.T) {
	h := newManagedHost(t, nativeResourceFixture())
	original := []byte(`{"model":"public-model","input":"hello"}`)
	h.start(t, original, bytes.Replace(original, []byte("public-model"), []byte("upstream-model"), 1))
	h.rawResponse(t, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, []byte(`{"id":"private-native-response","object":"response","model":"upstream-model","status":"completed","output":[{"id":"private-native-item","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"answer","annotations":[]}]}],"usage":{"input_tokens":17,"output_tokens":5,"total_tokens":22}}`))
	require.Nil(t, h.host.GetSentLocalResponse(h.id), "native IDs must be durable before a customer receives them")
	lookups := h.host.GetRedisCalloutAttributesFromContext(h.id)
	require.Len(t, lookups, 1)
	command, _, err := resp.NewReader(bytes.NewReader(lookups[0].Query)).ReadValue()
	require.NoError(t, err)
	args := command.Array()
	require.Equal(t, "EVAL", args[0].String())
	require.Contains(t, args[1].String(), "modelfusion-public-resource-write.v1")
	require.Contains(t, string(lookups[0].Query), "2592000")
	require.NotContains(t, string(lookups[0].Query), "controlled-upstream-key")
	h.host.CallOnRedisCallResponse(lookups[0].CalloutID, 0, []byte(":1\r\n"))
	response := h.host.GetSentLocalResponse(h.id)
	require.NotNil(t, response)
	require.EqualValues(t, 200, response.StatusCode)
	require.True(t, strings.HasPrefix(gjson.GetBytes(response.Data, "id").String(), "resp_pf_"))
	require.NotContains(t, string(response.Data), "private-native")
	require.Equal(t, "public-model", gjson.GetBytes(response.Data, "model").String())
}

func TestUnknownNativeResourceRejectsBeforeSupplierSend(t *testing.T) {
	for _, field := range []string{`"previous_response_id":"resp_pf_00000000000000000000000000000000"`, `"input":[{"type":"item_reference","id":"item_pf_00000000000000000000000000000000"}]`} {
		t.Run(strconv.Itoa(len(field)), func(t *testing.T) {
			h := newManagedHost(t, nativeResourceFixture())
			payload := []byte(`{"model":"public-model",` + field + `}`)
			h.start(t, payload, bytes.Replace(payload, []byte("public-model"), []byte("upstream-model"), 1))
			require.Empty(t, h.host.GetCalloutAttributesFromContext(h.id), "unknown or cross-scope references must not be sent to a supplier")
			lookups := h.host.GetRedisCalloutAttributesFromContext(h.id)
			require.Len(t, lookups, 1)
			command, _, err := resp.NewReader(bytes.NewReader(lookups[0].Query)).ReadValue()
			require.NoError(t, err)
			args := command.Array()
			require.Equal(t, "MGET", args[0].String())
			var reply bytes.Buffer
			reply.WriteString("*" + strconv.Itoa(len(args)-1) + "\r\n")
			for range args[1:] {
				reply.WriteString("$-1\r\n")
			}
			h.host.CallOnRedisCallResponse(lookups[0].CalloutID, 0, reply.Bytes())
			response := h.host.GetSentLocalResponse(h.id)
			require.NotNil(t, response)
			require.EqualValues(t, 400, response.StatusCode)
			require.Equal(t, "resource_reference_unavailable", gjson.GetBytes(response.Data, "error.code").String())
			require.Empty(t, h.host.GetCalloutAttributesFromContext(h.id))
		})
	}
}

func TestNativeResponsePreservesAFullParallelFunctionBatch(t *testing.T) {
	h := newManagedHost(t, nativeResourceFixture())
	request := []byte(`{"model":"public-model","input":"look up the batch","tools":[{"type":"function","name":"lookup","parameters":{"type":"object","properties":{}}}]}`)
	h.start(t, request, bytes.Replace(request, []byte("public-model"), []byte("upstream-model"), 1))
	items := make([]map[string]interface{}, 128)
	for i := range items {
		items[i] = map[string]interface{}{"type": "function_call", "id": "native-item-" + strconv.Itoa(i), "call_id": "native-call-" + strconv.Itoa(i), "name": "lookup", "arguments": "{}", "status": "completed"}
	}
	raw, err := json.Marshal(map[string]interface{}{"id": "native-batch", "object": "response", "status": "completed", "model": "upstream-model", "output": items, "usage": map[string]int{"input_tokens": 17, "output_tokens": 512, "total_tokens": 529}})
	require.NoError(t, err)
	h.rawResponse(t, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, raw)
	require.Nil(t, h.host.GetSentLocalResponse(h.id), "the batch must remain pending until all associations are durable")
	lookups := h.host.GetRedisCalloutAttributesFromContext(h.id)
	require.Len(t, lookups, 1)
	h.host.CallOnRedisCallResponse(lookups[0].CalloutID, 0, []byte(":1\r\n"))
	response := h.host.GetSentLocalResponse(h.id)
	require.NotNil(t, response)
	require.EqualValues(t, 200, response.StatusCode)
	require.Len(t, gjson.GetBytes(response.Data, "output").Array(), 128)
	seen := map[string]bool{}
	for _, item := range gjson.GetBytes(response.Data, "output").Array() {
		id := item.Get("call_id").String()
		require.False(t, seen[id])
		seen[id] = true
		require.Equal(t, "{}", item.Get("arguments").String())
	}
}

func TestClosingWhileResourceMappingIsPendingKeepsOneOriginalSale(t *testing.T) {
	h := newManagedHost(t, nativeResourceFixture())
	original := []byte(`{"model":"public-model","input":"hello"}`)
	h.start(t, original, bytes.Replace(original, []byte("public-model"), []byte("upstream-model"), 1))
	h.rawResponse(t, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, []byte(`{"id":"native-response","object":"response","status":"completed","model":"upstream-model","output":[],"usage":{"input_tokens":17,"output_tokens":5,"total_tokens":22}}`))
	lookups := h.host.GetRedisCalloutAttributesFromContext(h.id)
	require.Len(t, lookups, 1)
	require.Nil(t, h.host.GetSentLocalResponse(h.id))
	onManagedStreamDone(h.ctx, h.cfg)
	state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
	require.NotNil(t, state.lastDelivery, "a known original result must not lose its cancelled sale while waiting for ID persistence")
	fact := state.lastDelivery.Envelope()
	require.Equal(t, "cancelled", fact.DeliveryState)
	require.Equal(t, "17", fact.Usage.Quantities["input"])
	require.Equal(t, "5", fact.Usage.Quantities["output"])
	id := state.lastDelivery.EventID()
	h.host.CallOnRedisCallResponse(lookups[0].CalloutID, 0, []byte(":1\r\n"))
	require.Equal(t, id, state.lastDelivery.EventID())
	require.Nil(t, h.host.GetSentLocalResponse(h.id))
	require.Empty(t, h.host.GetCalloutAttributesFromContext(h.id))
}

func TestRouterStreamDoneDuringTerminalInjectionDefersFinalization(t *testing.T) {
	h := newManagedHost(t, nativeResourceFixture())
	original := []byte(`{"model":"public-model","input":"hello"}`)
	h.start(t, original, bytes.Replace(original, []byte("public-model"), []byte("upstream-model"), 1))
	state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
	state.router = &managedRouterResponse{transportEnded: true, injectingTerminal: true}
	onManagedStreamDone(h.ctx, h.cfg)
	require.True(t, state.closed)
	require.False(t, state.router.ended)
	require.Nil(t, state.lastDelivery)
}
