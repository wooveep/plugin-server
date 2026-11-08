package main

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/stretchr/testify/require"
)

func routerTextFixture(t *testing.T) map[string]interface{} {
	t.Helper()
	fixture := managedTextFixture(t, false)
	directory := fixture[managedTextDirectoryField].(managedTextDirectory)
	for i := range directory.Targets {
		entry := &directory.Targets[i]
		var proxy, envelope map[string]interface{}
		require.NoError(t, json.Unmarshal(entry.Proxy, &proxy))
		require.NoError(t, json.Unmarshal(entry.Execution, &envelope))
		settings := proxy["managed_execution"].(map[string]interface{})
		settings["send_transport"], settings["router_reentry"], settings["stream_projection"] = "router", true, true
		entry.Proxy = managedFixtureJSON(t, proxy)
		descriptor := envelope["descriptor"].(map[string]interface{})
		digest, err := mfjson.Digest(entry.Proxy)
		require.NoError(t, err)
		descriptor["proxy_digest"] = digest
		digest, err = managedDescriptorDigest(managedFixtureJSON(t, descriptor))
		require.NoError(t, err)
		descriptor["descriptor_digest"] = digest
		entry.Execution = managedFixtureJSON(t, envelope)
	}
	var root map[string]interface{}
	require.NoError(t, json.Unmarshal(directory.Targets[0].Proxy, &root))
	root["_modelfusion_execution"] = json.RawMessage(directory.Targets[0].Execution)
	root[managedTextDirectoryField] = directory
	managedRouterReentryTickRegistered = false
	return root
}

func TestRouterCredentialRetryReentersWithoutAnotherAdmissionOrCallout(t *testing.T) {
	h := newManagedTextHost(t, routerTextFixture(t))
	c := h.cfg.ManagedExecution.(*managedExecutionConfig)
	require.NoError(t, h.host.SetProperty([]string{"cluster_name"}, []byte(c.release.Descriptor.ClusterRef)))
	h.start(t, []byte(managedOriginalBody), []byte(managedMappedBody))
	respondManagedTextHealth(t, h, map[string]bool{"credential-1": true})
	forwardedProto, err := managedSingleHeader(h.host.GetCurrentRequestHeaders(h.id), "x-forwarded-proto")
	require.NoError(t, err)
	require.NotEmpty(t, forwardedProto, "Envoy must be able to resolve the next route after rebuilding headers")
	originalAttempt, err := managedSingleHeader(h.host.GetCurrentRequestHeaders(h.id), managedAttemptHeader)
	require.NoError(t, err)
	require.Empty(t, h.host.GetCalloutAttributesFromContext(h.id))
	require.NoError(t, h.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
	require.Equal(t, types.HeaderStopIteration, h.host.CallOnResponseHeaders(h.id, [][2]string{{":status", "500"}, {"content-type", "application/json"}}, false))
	h.host.CallOnResponseBody(h.id, []byte(`{"error":{"message":"retryable"},"usage":{"prompt_tokens":7,"completion_tokens":2}}`), true)
	respondManagedTextHealth(t, h, map[string]bool{"credential-2": true})
	retainedAttempt, err := managedSingleHeader(h.host.GetCurrentRequestHeaders(h.id), managedAttemptHeader)
	require.NoError(t, err)
	require.Equal(t, originalAttempt, retainedAttempt, "response qualification must not mutate the sealed original request")
	require.Nil(t, h.host.GetSentLocalResponse(h.id))
	require.Empty(t, h.host.GetCalloutAttributesFromContext(h.id))
	require.Equal(t, 1, len(managedRouterPending))
	oldID := h.id
	h.id = h.host.InitializeHttpContext()
	headers := [][2]string{{":method", "POST"}, {":authority", wrapper.RouterContinuationHost}, {":path", managedRouterTargetPath(c)}, {"content-type", "application/json"}}
	require.Equal(t, types.HeaderStopIteration, h.host.CallOnRequestHeaders(h.id, headers, false))
	require.Empty(t, managedRouterPending)
	h.host.CallOnRequestBody(h.id, []byte(managedMappedBody), true)
	auth, err := managedSingleHeader(h.host.GetCurrentRequestHeaders(h.id), "authorization")
	require.NoError(t, err)
	require.Equal(t, "Bearer second-controlled-key", auth)
	h.host.CompleteHttpContext(oldID)
	h.host.CallOnResponseHeaders(h.id, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, false)
	h.host.CallOnResponseBody(h.id, []byte(managedSuccessfulBody), true)
	var attempts, deliveries []executionfacts.Envelope
	registrations := 0
	for _, record := range h.records(t) {
		if record.messageType == "router_execution_registration" {
			registrations++
			continue
		}
		fact := decodeManagedFact(t, record.raw)
		if fact.Kind == "upstream_attempt" {
			attempts = append(attempts, fact)
		} else if fact.Kind == "call_delivery" {
			deliveries = append(deliveries, fact)
		}
	}
	require.Equal(t, 2, registrations)
	require.Equal(t, 2, len(attempts))
	require.Equal(t, 1, len(deliveries))
	require.Equal(t, attempts[0].CallID, attempts[1].CallID)
	require.Equal(t, attempts[0].AttemptID, attempts[1].ParentAttemptID)
	require.Equal(t, "credential_retry", attempts[1].Mechanism)
	require.Equal(t, "credential-2", attempts[1].CredentialRef)
	require.Equal(t, attempts[1].Usage, deliveries[0].Usage)
	require.Empty(t, h.host.GetCalloutAttributesFromContext(h.id))
	h.host.CompleteHttpContext(h.id)
	// A public caller cannot claim the private entrance after its one use.
	rogue := h.host.InitializeHttpContext()
	h.host.CallOnRequestHeaders(rogue, headers, false)
	require.EqualValues(t, 403, h.host.GetSentLocalResponse(rogue).StatusCode)
}
