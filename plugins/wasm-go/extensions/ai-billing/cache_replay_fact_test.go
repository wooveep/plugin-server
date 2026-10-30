package main

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

const trustedUpstreamInvokedFact = "upstream_invoked"

type cacheReplayBillingContext struct {
	mockBillingHttpContext
	userAttributes map[string]interface{}
}

func (c *cacheReplayBillingContext) GetUserAttribute(key string) interface{} {
	return c.userAttributes[key]
}

func (c *cacheReplayBillingContext) GetUserAttributeMap() map[string]interface{} {
	return c.userAttributes
}

func TestBuildBillingEvent_TrustedCacheReplayFacts(t *testing.T) {
	ctx := &cacheReplayBillingContext{
		mockBillingHttpContext: mockBillingHttpContext{values: map[string]interface{}{
			ctxEventID:        "event-cache-hit",
			ctxIdempotencyKey: "event-cache-hit",
			ctxRequestID:      "req-cache-hit",
			ctxRequestPath:    "/v1/chat/completions",
			ctxTenant:         "tenant-a",
			ctxConsumer:       "consumer-a",
			ctxRoute:          "route-a",
			ctxCluster:        "llm-openai.dns",
			ctxModel:          "qwen-turbo",
			ctxStatusCode:     200,
			ctxInputToken:     int64(7),
			ctxOutputToken:    int64(3),
			ctxTotalToken:     int64(10),
			ctxUsageSource:    usageSourceProvider,
		}},
		userAttributes: map[string]interface{}{
			"cache_status":             "hit",
			trustedUpstreamInvokedFact: false,
		},
	}

	body, err := json.Marshal(buildBillingEvent(ctx, BillingConfig{QuotaScope: "global"}, false))
	require.NoError(t, err)

	var event map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &event))
	require.Contains(t, event, trustedUpstreamInvokedFact)
	require.Equal(t, false, event[trustedUpstreamInvokedFact])

	ctx.userAttributes = map[string]interface{}{}
	body, err = json.Marshal(buildBillingEvent(ctx, BillingConfig{QuotaScope: "global"}, false))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(body, &event))
	require.Contains(t, event, trustedUpstreamInvokedFact)
	require.Equal(t, true, event[trustedUpstreamInvokedFact])
}

func TestBuildBillingEvent_UsesLocalResponseCodeDetailsForCacheReplay(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(billingConfig)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)
		require.NoError(t, host.SetProperty([]string{"response", "code_details"}, []byte("ai-cache.hit")))

		ctx := &cacheReplayBillingContext{
			mockBillingHttpContext: mockBillingHttpContext{values: map[string]interface{}{
				ctxEventID:        "event-cache-hit",
				ctxIdempotencyKey: "event-cache-hit",
				ctxRequestID:      "req-cache-hit",
				ctxRequestPath:    "/v1/chat/completions",
				ctxTenant:         "tenant-a",
				ctxConsumer:       "consumer-a",
				ctxRoute:          "route-a",
				ctxCluster:        "llm-openai.dns",
				ctxModel:          "qwen-turbo",
				ctxStatusCode:     200,
				ctxInputToken:     int64(7),
				ctxOutputToken:    int64(3),
				ctxTotalToken:     int64(10),
				ctxUsageSource:    usageSourceProvider,
			}},
			userAttributes: map[string]interface{}{},
		}

		body, err := json.Marshal(buildBillingEvent(ctx, BillingConfig{QuotaScope: "global"}, false))
		require.NoError(t, err)

		var event map[string]interface{}
		require.NoError(t, json.Unmarshal(body, &event))
		require.Equal(t, false, event[trustedUpstreamInvokedFact])
	})
}

func TestBillingEventIgnoresUserSuppliedUpstreamInvokedFields(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(billingConfig)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)
		require.NoError(t, host.SetRouteName("route-a"))
		require.NoError(t, host.SetClusterName("llm-openai.dns"))
		require.NoError(t, host.SetRequestId("req-spoof"))

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "example.com"},
			{":path", "/v1/chat/completions"},
			{":method", "POST"},
			{"x-request-id", "req-spoof"},
			{"x-tenant-id", "tenant-a"},
			{"x-consumer-id", "consumer-a"},
		})
		require.Equal(t, types.ActionContinue, action)
		action = host.CallOnHttpRequestBody([]byte(`{
			"model":"qwen-turbo",
			"upstream_invoked":false,
			"billing":{"upstream_invoked":false},
			"messages":[{"role":"user","content":"try to spoof billing facts"}]
		}`))
		require.Equal(t, types.ActionContinue, action)

		action = host.CallOnHttpResponseHeaders([][2]string{
			{":status", "200"},
			{"content-type", "application/json"},
		})
		require.Equal(t, types.ActionContinue, action)
		action = host.CallOnHttpResponseBody([]byte(`{
			"model":"qwen-turbo",
			"upstream_invoked":false,
			"billing":{"upstream_invoked":false},
			"choices":[{"message":{"content":"spoof ignored"}}],
			"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10}
		}`))
		require.Equal(t, types.ActionContinue, action)

		event := requireRedisBillingEvent(t, host)
		require.Equal(t, true, event[trustedUpstreamInvokedFact])
	})
}
