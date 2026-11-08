package main

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

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

func TestRouterRetryUsesRemainingCallBudgetAndShorterAttemptLimit(t *testing.T) {
	for _, limit := range []uint32{500, 9000} {
		t.Run(strconv.FormatUint(uint64(limit), 10), func(t *testing.T) {
			h := newManagedTextHost(t, routerTextFixture(t))
			c := h.cfg.ManagedExecution.(*managedExecutionConfig)
			require.NoError(t, h.host.SetProperty([]string{"cluster_name"}, []byte(c.release.Descriptor.ClusterRef)))
			h.start(t, []byte(managedOriginalBody), []byte(managedMappedBody))
			respondManagedTextHealth(t, h, map[string]bool{"credential-1": true})
			state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
			deadline := time.Now().Add(2500 * time.Millisecond)
			state.routerDeadline = deadline
			state.config.settings.TextExecution.Retry.RetryTimeoutMS = limit
			require.NoError(t, h.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
			h.host.CallOnResponseHeaders(h.id, [][2]string{{":status", "500"}, {"content-type", "application/json"}}, false)
			h.host.CallOnResponseBody(h.id, []byte(`{"error":{"message":"retryable"},"usage":{"prompt_tokens":7,"completion_tokens":2}}`), true)
			respondManagedTextHealth(t, h, map[string]bool{"credential-2": true})
			require.Len(t, managedRouterPending, 1)
			for _, entry := range managedRouterPending {
				require.Equal(t, deadline, entry.next.routerDeadline)
			}
			oldID := h.id
			h.id = h.host.InitializeHttpContext()
			headers := [][2]string{{":method", "POST"}, {":authority", wrapper.RouterContinuationHost}, {":path", managedRouterTargetPath(c)}, {"content-type", "application/json"}}
			h.host.CallOnRequestHeaders(h.id, headers, false)
			h.host.CallOnRequestBody(h.id, []byte(managedMappedBody), true)
			value, err := managedSingleHeader(h.host.GetCurrentRequestHeaders(h.id), "x-envoy-upstream-rq-timeout-ms")
			require.NoError(t, err)
			remaining, err := strconv.ParseUint(value, 10, 32)
			require.NoError(t, err)
			require.Positive(t, remaining)
			require.LessOrEqual(t, remaining, uint64(min(limit, 2500)))
			require.Greater(t, remaining, uint64(min(limit, 2500))-100)
			require.Empty(t, h.host.GetCalloutAttributesFromContext(h.id))
			h.host.CompleteHttpContext(oldID)
			h.host.CallOnResponseHeaders(h.id, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, false)
			h.host.CallOnResponseBody(h.id, []byte(managedSuccessfulBody), true)
			h.host.CompleteHttpContext(h.id)
		})
	}
}

func TestRouterRecoveryExpiredDuringQualificationDoesNotRegisterAnotherSend(t *testing.T) {
	h := newManagedTextHost(t, routerTextFixture(t))
	c := h.cfg.ManagedExecution.(*managedExecutionConfig)
	require.NoError(t, h.host.SetProperty([]string{"cluster_name"}, []byte(c.release.Descriptor.ClusterRef)))
	h.start(t, []byte(managedOriginalBody), []byte(managedMappedBody))
	respondManagedTextHealth(t, h, map[string]bool{"credential-1": true})
	state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
	state.routerDeadline = time.Now().Add(time.Minute)
	require.NoError(t, h.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
	h.host.CallOnResponseHeaders(h.id, [][2]string{{":status", "500"}, {"content-type", "application/json"}}, false)
	h.host.CallOnResponseBody(h.id, []byte(`{"error":{"message":"retryable"},"usage":{"prompt_tokens":7,"completion_tokens":2}}`), true)
	state.routerDeadline = time.Now().Add(-time.Second)
	state.text.current.routerDeadline = state.routerDeadline
	respondManagedTextHealth(t, h, map[string]bool{"credential-2": true})
	require.Empty(t, managedRouterPending)
	h.host.CompleteHttpContext(h.id)
	counts := map[string]int{}
	for _, record := range h.records(t) {
		counts[record.messageType]++
	}
	require.Equal(t, 1, counts["router_execution_registration"])
	var attempts, deliveries int
	for _, record := range h.records(t) {
		if record.messageType == "router_execution_registration" {
			continue
		}
		fact := decodeManagedFact(t, record.raw)
		if fact.Kind == "upstream_attempt" {
			attempts++
		} else if fact.Kind == "call_delivery" {
			deliveries++
		}
	}
	require.Equal(t, 1, attempts)
	require.Equal(t, 1, deliveries)
}

func TestRouterAbandonedContinuationRetainsOneSaleAndNoUnsentProcurement(t *testing.T) {
	h := newManagedTextHost(t, routerTextFixture(t))
	c := h.cfg.ManagedExecution.(*managedExecutionConfig)
	require.NoError(t, h.host.SetProperty([]string{"cluster_name"}, []byte(c.release.Descriptor.ClusterRef)))
	h.start(t, []byte(managedOriginalBody), []byte(managedMappedBody))
	respondManagedTextHealth(t, h, map[string]bool{"credential-1": true})
	require.NoError(t, h.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
	h.host.CallOnResponseHeaders(h.id, [][2]string{{":status", "500"}, {"content-type", "application/json"}}, false)
	h.host.CallOnResponseBody(h.id, []byte(`{"error":{"message":"retryable"},"usage":{"prompt_tokens":7,"completion_tokens":2}}`), true)
	respondManagedTextHealth(t, h, map[string]bool{"credential-2": true})
	require.Len(t, managedRouterPending, 1)
	h.host.CompleteHttpContext(h.id)
	for key, entry := range managedRouterPending {
		entry.expires = time.Now().Add(-time.Second)
		managedRouterPending[key] = entry
	}
	h.host.Tick()
	require.Empty(t, managedRouterPending)
	counts := map[string]int{}
	for _, record := range h.records(t) {
		if record.messageType == "router_execution_registration" {
			counts[record.messageType]++
			continue
		}
		fact := decodeManagedFact(t, record.raw)
		counts[fact.Kind]++
	}
	// Qualification reserves the second identity, but does not prove a send.
	require.Equal(t, 2, counts["router_execution_registration"])
	require.Equal(t, 1, counts["upstream_attempt"])
	require.Equal(t, 1, counts["call_delivery"])
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
	continuedLog := h.ctx.GetContext(managedAccessLogContext).(*managedAccessLog)
	require.False(t, continuedLog.closed, "the retired context must not close the continuation's access log")
	require.NotEmpty(t, continuedLog.CallID)
	require.Len(t, continuedLog.Attempts, 1)
	require.Equal(t, originalAttempt, continuedLog.Attempts[0].ParentAttemptID)
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
