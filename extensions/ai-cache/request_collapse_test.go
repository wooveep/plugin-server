package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestRequestCollapse(t *testing.T) {
	t.Run("local singleflight follower waits for leader cache fill", func(t *testing.T) {
		host, status := test.NewTestHost(requestCollapseConfig(false, false))
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		leaderID := callCacheableRequestForPrompt(t, host, "介绍杭州西湖")
		respondNextRedisCall(t, host, leaderID, test.CreateRedisRespNull())
		respondNextRedisCall(t, host, leaderID, test.CreateRedisRespNull())
		require.Equal(t, types.ActionContinue, host.GetCurrentHttpStreamAction(leaderID))

		followerID := callCacheableRequestForPrompt(t, host, "介绍杭州西湖")
		respondNextRedisCall(t, host, followerID, test.CreateRedisRespNull())
		respondNextRedisCall(t, host, followerID, test.CreateRedisRespNull())
		require.Equal(t, types.ActionPause, host.GetCurrentHttpStreamAction(followerID))
		requireRedisBLPopQuery(t, nextRedisQuery(t, host, followerID))

		completeSuccessfulUpstreamResponse(t, host, leaderID)
		respondNextRedisCall(t, host, leaderID, test.CreateRedisRespString("OK"))
		require.True(t, hasRedisQueryContaining(t, host, leaderID, "RPUSH"), "expected request collapse notification")

		respondNextRedisCall(t, host, followerID, test.CreateRedisRespArray([]interface{}{"ready", "ready"}))
		require.Contains(t, strings.ToUpper(nextRedisQuery(t, host, followerID)), "GET")
		respondNextRedisCall(t, host, followerID, test.CreateRedisRespArray([]interface{}{structuredHitPayload(t, 4102444800, 4102448400)}))

		localResponse := host.GetSentLocalResponse(followerID)
		require.NotNil(t, localResponse)
		require.Equal(t, uint32(200), localResponse.StatusCode)
		require.Contains(t, string(localResponse.Data), `"model":"qwen-plus"`)
		require.NotContains(t, string(localResponse.Data), "from-cache")
	})

	t.Run("leader notifies followers only after cache write succeeds", func(t *testing.T) {
		host, status := test.NewTestHost(requestCollapseConfig(false, false))
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		leaderID := callCacheableRequestForPrompt(t, host, "介绍杭州西湖")
		respondNextRedisCall(t, host, leaderID, test.CreateRedisRespNull())
		respondNextRedisCall(t, host, leaderID, test.CreateRedisRespNull())
		require.Equal(t, types.ActionContinue, host.GetCurrentHttpStreamAction(leaderID))

		followerID := callCacheableRequestForPrompt(t, host, "介绍杭州西湖")
		respondNextRedisCall(t, host, followerID, test.CreateRedisRespNull())
		respondNextRedisCall(t, host, followerID, test.CreateRedisRespNull())
		requireRedisBLPopQuery(t, nextRedisQuery(t, host, followerID))

		completeSuccessfulUpstreamResponse(t, host, leaderID)
		require.Contains(t, strings.ToUpper(nextRedisQuery(t, host, leaderID)), "SET")
		require.False(t, hasRedisQueryContaining(t, host, leaderID, "RPUSH"), "must not notify before cache write completes")

		respondNextRedisCall(t, host, leaderID, test.CreateRedisRespString("OK"))
		require.True(t, hasRedisQueryContaining(t, host, leaderID, "RPUSH"), "expected notification after cache write succeeds")
	})

	t.Run("leader releases without notification when cache write fails", func(t *testing.T) {
		host, status := test.NewTestHost(requestCollapseConfig(false, false))
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		leaderID := callCacheableRequestForPrompt(t, host, "介绍杭州西湖")
		respondNextRedisCall(t, host, leaderID, test.CreateRedisRespNull())
		respondNextRedisCall(t, host, leaderID, test.CreateRedisRespNull())
		require.Equal(t, types.ActionContinue, host.GetCurrentHttpStreamAction(leaderID))

		followerID := callCacheableRequestForPrompt(t, host, "介绍杭州西湖")
		respondNextRedisCall(t, host, followerID, test.CreateRedisRespNull())
		respondNextRedisCall(t, host, followerID, test.CreateRedisRespNull())
		requireRedisBLPopQuery(t, nextRedisQuery(t, host, followerID))

		completeSuccessfulUpstreamResponse(t, host, leaderID)
		respondNextRedisCall(t, host, leaderID, test.CreateRedisRespError("write failed"))
		require.False(t, hasRedisQueryContaining(t, host, leaderID, "RPUSH"), "must not notify after failed cache write")

		retryID := callCacheableRequestForPrompt(t, host, "介绍杭州西湖")
		respondNextRedisCall(t, host, retryID, test.CreateRedisRespNull())
		respondNextRedisCall(t, host, retryID, test.CreateRedisRespNull())
		require.Equal(t, types.ActionContinue, host.GetCurrentHttpStreamAction(retryID), "failed leader should release local singleflight slot")
	})

	t.Run("leader releases without notification when response is not cacheable", func(t *testing.T) {
		host, status := test.NewTestHost(requestCollapseConfig(false, false))
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		leaderID := callCacheableRequestForPrompt(t, host, "介绍杭州西湖")
		respondNextRedisCall(t, host, leaderID, test.CreateRedisRespNull())
		respondNextRedisCall(t, host, leaderID, test.CreateRedisRespNull())
		require.Equal(t, types.ActionContinue, host.GetCurrentHttpStreamAction(leaderID))

		followerID := callCacheableRequestForPrompt(t, host, "介绍杭州西湖")
		respondNextRedisCall(t, host, followerID, test.CreateRedisRespNull())
		respondNextRedisCall(t, host, followerID, test.CreateRedisRespNull())
		requireRedisBLPopQuery(t, nextRedisQuery(t, host, followerID))

		require.Equal(t, types.ActionContinue, host.CallOnResponseHeaders(leaderID, [][2]string{
			{":status", "500"},
			{"content-type", "application/json"},
		}, true))
		require.False(t, hasRedisQueryContaining(t, host, leaderID, "RPUSH"), "must not notify when no cache write succeeds")

		retryID := callCacheableRequestForPrompt(t, host, "介绍杭州西湖")
		respondNextRedisCall(t, host, retryID, test.CreateRedisRespNull())
		respondNextRedisCall(t, host, retryID, test.CreateRedisRespNull())
		require.Equal(t, types.ActionContinue, host.GetCurrentHttpStreamAction(retryID), "non-cacheable leader should release local singleflight slot")
	})

	t.Run("stream done before cache write callback releases without notification", func(t *testing.T) {
		host, status := test.NewTestHost(requestCollapseConfig(false, false))
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		leaderID := callCacheableRequestForPrompt(t, host, "介绍杭州西湖")
		respondNextRedisCall(t, host, leaderID, test.CreateRedisRespNull())
		respondNextRedisCall(t, host, leaderID, test.CreateRedisRespNull())
		require.Equal(t, types.ActionContinue, host.GetCurrentHttpStreamAction(leaderID))

		followerID := callCacheableRequestForPrompt(t, host, "介绍杭州西湖")
		respondNextRedisCall(t, host, followerID, test.CreateRedisRespNull())
		respondNextRedisCall(t, host, followerID, test.CreateRedisRespNull())
		requireRedisBLPopQuery(t, nextRedisQuery(t, host, followerID))

		completeSuccessfulUpstreamResponse(t, host, leaderID)
		require.Contains(t, strings.ToUpper(nextRedisQuery(t, host, leaderID)), "SET")

		host.CompleteHttpContext(leaderID)
		require.False(t, hasRedisQueryContaining(t, host, leaderID, "RPUSH"), "stream done before SET callback must not notify")

		respondNextRedisCall(t, host, leaderID, test.CreateRedisRespString("OK"))
		require.False(t, hasRedisQueryContaining(t, host, leaderID, "RPUSH"), "late SET callback after stream done must not notify")

		retryID := callCacheableRequestForPrompt(t, host, "介绍杭州西湖")
		respondNextRedisCall(t, host, retryID, test.CreateRedisRespNull())
		respondNextRedisCall(t, host, retryID, test.CreateRedisRespNull())
		require.Equal(t, types.ActionContinue, host.GetCurrentHttpStreamAction(retryID), "stream-done leader should release local singleflight slot")
	})

	t.Run("distributed lock winner proceeds upstream and releases lock", func(t *testing.T) {
		host, status := test.NewTestHost(requestCollapseConfig(true, false))
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		contextID := callCacheableRequestForPrompt(t, host, "介绍杭州西湖")
		respondNextRedisCall(t, host, contextID, test.CreateRedisRespNull())
		respondNextRedisCall(t, host, contextID, test.CreateRedisRespNull())
		requireRedisSetNXQuery(t, nextRedisQuery(t, host, contextID), "higress-ai-cache:")

		respondNextRedisCall(t, host, contextID, test.CreateRedisRespString("OK"))
		require.Equal(t, types.ActionContinue, host.GetCurrentHttpStreamAction(contextID))

		completeSuccessfulUpstreamResponse(t, host, contextID)
		respondNextRedisCall(t, host, contextID, test.CreateRedisRespString("OK"))
		require.True(t, hasRedisQueryContaining(t, host, contextID, "EVAL"), "expected safe lock release script")
	})

	t.Run("distributed lock uses cache key prefix", func(t *testing.T) {
		host, status := test.NewTestHost(requestCollapseConfigWithPrefix(true, false, "custom-ai-cache:"))
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		contextID := callCacheableRequestForPrompt(t, host, "介绍杭州西湖")
		respondNextRedisCall(t, host, contextID, test.CreateRedisRespNull())
		respondNextRedisCall(t, host, contextID, test.CreateRedisRespNull())
		requireRedisSetNXQuery(t, nextRedisQuery(t, host, contextID), "custom-ai-cache:")
	})

	t.Run("distributed lock loser waits for structured cache fill", func(t *testing.T) {
		host, status := test.NewTestHost(requestCollapseConfig(true, false))
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		contextID := callCacheableRequestForPrompt(t, host, "介绍杭州西湖")
		respondNextRedisCall(t, host, contextID, test.CreateRedisRespNull())
		respondNextRedisCall(t, host, contextID, test.CreateRedisRespNull())
		requireRedisSetNXQuery(t, nextRedisQuery(t, host, contextID), "higress-ai-cache:")

		respondNextRedisCall(t, host, contextID, test.CreateRedisRespInt(0))
		require.Equal(t, types.ActionPause, host.GetCurrentHttpStreamAction(contextID))
		requireRedisBLPopQuery(t, nextRedisQuery(t, host, contextID))

		respondNextRedisCall(t, host, contextID, test.CreateRedisRespArray([]interface{}{"ready", "ready"}))
		require.Contains(t, strings.ToUpper(nextRedisQuery(t, host, contextID)), "GET")
		respondNextRedisCall(t, host, contextID, test.CreateRedisRespArray([]interface{}{structuredHitPayload(t, 4102444800, 4102448400)}))
		localResponse := host.GetSentLocalResponse(contextID)
		require.NotNil(t, localResponse)
		require.Equal(t, uint32(200), localResponse.StatusCode)
		require.Contains(t, string(localResponse.Data), `"model":"qwen-plus"`)
	})

	t.Run("wait timeout can fall back to stale structured entry", func(t *testing.T) {
		host, status := test.NewTestHost(requestCollapseConfig(true, true))
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		contextID := callCacheableRequestForPrompt(t, host, "介绍杭州西湖")
		respondNextRedisCall(t, host, contextID, test.CreateRedisRespArray([]interface{}{structuredHitPayload(t, 1, 4102448400)}))
		respondNextRedisCall(t, host, contextID, test.CreateRedisRespNull())
		requireRedisSetNXQuery(t, nextRedisQuery(t, host, contextID), "higress-ai-cache:")

		respondNextRedisCall(t, host, contextID, test.CreateRedisRespInt(0))
		requireRedisBLPopQuery(t, nextRedisQuery(t, host, contextID))
		respondNextRedisCall(t, host, contextID, test.CreateRedisRespNull())
		require.Contains(t, strings.ToUpper(nextRedisQuery(t, host, contextID)), "GET")
		respondNextRedisCall(t, host, contextID, test.CreateRedisRespNull())

		localResponse := host.GetSentLocalResponse(contextID)
		require.NotNil(t, localResponse)
		require.Equal(t, uint32(200), localResponse.StatusCode)
		require.Contains(t, string(localResponse.Data), "今天北京天气晴朗")
	})

	t.Run("no-store miss does not become collapse leader", func(t *testing.T) {
		host, status := test.NewTestHost(requestCollapseConfig(true, false))
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		contextID := callNoStoreRequestForPrompt(t, host, "介绍杭州西湖")
		respondNextRedisCall(t, host, contextID, test.CreateRedisRespNull())
		respondNextRedisCall(t, host, contextID, test.CreateRedisRespNull())

		require.Equal(t, types.ActionContinue, host.GetCurrentHttpStreamAction(contextID))
		require.Empty(t, host.GetRedisCalloutAttributesFromContext(contextID))
	})
}

func callCacheableRequestForPrompt(t *testing.T, host test.TestHost, prompt string) uint32 {
	t.Helper()
	contextID := host.InitializeHttpContext()
	require.Equal(t, types.ActionPause, host.CallOnRequestHeaders(contextID, [][2]string{
		{":authority", "example.com"},
		{":path", "/api/chat"},
		{":method", "POST"},
		{"content-type", "application/json"},
	}, false))
	body := `{
		"model": "qwen-turbo",
		"messages": [{"role": "user", "content": "` + prompt + `"}],
		"stream": false
	}`
	require.Equal(t, types.ActionPause, host.CallOnRequestBody(contextID, []byte(body), true))
	return contextID
}

func callNoStoreRequestForPrompt(t *testing.T, host test.TestHost, prompt string) uint32 {
	t.Helper()
	contextID := host.InitializeHttpContext()
	require.Equal(t, types.ActionPause, host.CallOnRequestHeaders(contextID, [][2]string{
		{":authority", "example.com"},
		{":path", "/api/chat"},
		{":method", "POST"},
		{"content-type", "application/json"},
		{"x-higress-ai-cache-mode", "no-store"},
	}, false))
	body := `{
		"model": "qwen-turbo",
		"messages": [{"role": "user", "content": "` + prompt + `"}],
		"stream": false
	}`
	require.Equal(t, types.ActionPause, host.CallOnRequestBody(contextID, []byte(body), true))
	return contextID
}

func respondNextRedisCall(t *testing.T, host test.TestHost, contextID uint32, response []byte) {
	t.Helper()
	attrs := host.GetRedisCalloutAttributesFromContext(contextID)
	require.NotEmpty(t, attrs)
	host.CallOnRedisCallResponse(attrs[0].CalloutID, 0, response)
}

func nextRedisQuery(t *testing.T, host test.TestHost, contextID uint32) string {
	t.Helper()
	attrs := host.GetRedisCalloutAttributesFromContext(contextID)
	require.NotEmpty(t, attrs)
	return string(attrs[0].Query)
}

func hasRedisQueryContaining(t *testing.T, host test.TestHost, contextID uint32, fragment string) bool {
	t.Helper()
	for _, attr := range host.GetRedisCalloutAttributesFromContext(contextID) {
		if strings.Contains(strings.ToUpper(string(attr.Query)), strings.ToUpper(fragment)) {
			return true
		}
	}
	return false
}

func requireRedisSetNXQuery(t *testing.T, query string, prefix string) {
	t.Helper()
	upper := strings.ToUpper(query)
	require.Contains(t, upper, "SET")
	require.Contains(t, upper, "NX")
	require.Contains(t, upper, "EX")
	require.Contains(t, query, prefix)
	require.Contains(t, query, ":lock:")
}

func requireRedisBLPopQuery(t *testing.T, query string) {
	t.Helper()
	upper := strings.ToUpper(query)
	require.Contains(t, upper, "BLPOP")
	require.Contains(t, upper, ":NOTIFY:")
	require.NotContains(t, query, "介绍杭州西湖")
}

func completeSuccessfulUpstreamResponse(t *testing.T, host test.TestHost, contextID uint32) {
	t.Helper()
	require.Equal(t, types.ActionContinue, host.CallOnResponseHeaders(contextID, [][2]string{
		{":status", "200"},
		{"content-type", "application/json"},
	}, false))
	responseBody := `{
		"id": "chatcmpl-123",
		"choices": [
			{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "西湖位于杭州，是著名风景名胜。"
				},
				"finish_reason": "stop"
			}
		],
		"model": "qwen-plus",
		"object": "chat.completion",
		"usage": {"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30}
	}`
	require.Equal(t, types.ActionContinue, host.CallOnResponseBody(contextID, []byte(responseBody), true))
}

func requestCollapseConfig(distributedLock bool, staleFallback bool) json.RawMessage {
	return requestCollapseConfigWithPrefix(distributedLock, staleFallback, "higress-ai-cache:")
}

func requestCollapseConfigWithPrefix(distributedLock bool, staleFallback bool, cacheKeyPrefix string) json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"cache": map[string]interface{}{
			"type":           "redis",
			"serviceName":    "redis.static",
			"servicePort":    6379,
			"cacheTTL":       3600,
			"cacheKeyPrefix": cacheKeyPrefix,
		},
		"requestCollapse": map[string]interface{}{
			"enabled":          true,
			"distributedLock":  distributedLock,
			"staleFallback":    staleFallback,
			"waitAttempts":     1,
			"waitMilliseconds": 2500,
			"lockTTLSeconds":   5,
		},
		"provider":               "dashscope",
		"cacheScope":             "global",
		"cacheKeyStrategy":       "lastQuestion",
		"cacheKeyFrom":           "messages.@reverse.0.content",
		"cacheValueFrom":         "choices.0.message.content",
		"cacheStreamValueFrom":   "choices.0.delta.content",
		"responseTemplate":       `{"id":"from-cache","choices":[{"index":0,"message":{"role":"assistant","content":"%s"},"finish_reason":"stop"}],"model":"from-cache","object":"chat.completion","usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`,
		"streamResponseTemplate": `data:{"id":"from-cache","choices":[{"index":0,"delta":{"role":"assistant","content":"%s"},"finish_reason":"stop"}],"model":"from-cache","object":"chat.completion","usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}` + "\n\ndata:[DONE]\n\n",
	})
	return data
}
