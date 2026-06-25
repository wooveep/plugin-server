package main

import (
	"encoding/json"
	"testing"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// 测试配置：基础Redis缓存配置
var basicRedisConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"cache": map[string]interface{}{
			"type":           "redis",
			"serviceName":    "redis.static",
			"servicePort":    6379,
			"timeout":        10000,
			"cacheTTL":       3600,
			"cacheKeyPrefix": "higress-ai-cache:",
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
}()

// 测试配置：完整配置（Redis + DashScope + DashVector）
var completeConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"cache": map[string]interface{}{
			"type":           "redis",
			"serviceName":    "redis.static",
			"servicePort":    6379,
			"timeout":        10000,
			"cacheTTL":       3600,
			"cacheKeyPrefix": "higress-ai-cache:",
		},
		"embedding": map[string]interface{}{
			"type":        "dashscope",
			"serviceName": "dashscope-service",
			"serviceHost": "dashscope.example.com",
			"servicePort": 8080,
			"timeout":     15000,
			"model":       "text-embedding-v1",
			"apiKey":      "test-dashscope-key",
		},
		"vector": map[string]interface{}{
			"type":         "dashvector",
			"serviceName":  "dashvector-service",
			"serviceHost":  "dashvector.example.com",
			"servicePort":  8081,
			"apiKey":       "test-dashvector-key",
			"collectionID": "test-collection",
		},
		"provider":               "dashscope",
		"cacheKeyStrategy":       "lastQuestion",
		"cacheKeyFrom":           "messages.@reverse.0.content",
		"cacheValueFrom":         "choices.0.message.content",
		"cacheStreamValueFrom":   "choices.0.delta.content",
		"enableSemanticCache":    true,
		"responseTemplate":       `{"id":"from-cache","choices":[{"index":0,"message":{"role":"assistant","content":"%s"},"finish_reason":"stop"}],"model":"from-cache","object":"chat.completion","usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`,
		"streamResponseTemplate": `data:{"id":"from-cache","choices":[{"index":0,"delta":{"role":"assistant","content":"%s"},"finish_reason":"stop"}],"model":"from-cache","object":"chat.completion","usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}` + "\n\ndata:[DONE]\n\n",
	})
	return data
}()

// 测试配置：最小配置（使用默认值）
var minimalConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"cache": map[string]interface{}{
			"type":        "redis",
			"serviceName": "redis.static",
			"servicePort": 6379,
		},
		"provider": "dashscope",
	})
	return data
}()

// 测试配置：仅缓存配置（无语义缓存）
var cacheOnlyConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"cache": map[string]interface{}{
			"type":        "redis",
			"serviceName": "redis.static",
			"servicePort": 6379,
			"timeout":     10000,
		},
		"provider":            "dashscope",
		"cacheScope":          "global",
		"cacheKeyStrategy":    "allQuestions",
		"cacheKeyFrom":        "messages.@reverse.0.content",
		"cacheValueFrom":      "choices.0.message.content",
		"enableSemanticCache": false,
	})
	return data
}()

func TestRequestCacheEligibility(t *testing.T) {
	t.Run("english terms match token boundaries", func(t *testing.T) {
		require.True(t, requestHasDynamicDataIntent("what is the weather today?"))
		require.True(t, requestHasDynamicDataIntent("what is the usd exchange rate?"))
		require.True(t, requestHasDynamicDataIntent("今天杭州天气怎么样"))
		require.True(t, requestHasAmbiguousMultiTurnIntent(gjson.Parse(`{
			"messages": [
				{"role": "user", "content": "summarize the proposal"},
				{"role": "assistant", "content": "first draft"},
				{"role": "user", "content": "make it shorter"}
			]
		}`), "make it shorter"))

		require.False(t, requestHasDynamicDataIntent("explain snow formation"))
		require.False(t, requestHasAmbiguousMultiTurnIntent(gjson.Parse(`{
			"messages": [
				{"role": "user", "content": "draft notes"},
				{"role": "assistant", "content": "done"},
				{"role": "user", "content": "write a summary"}
			]
		}`), "write a summary"))
	})
}

// 测试配置：仅嵌入模型配置
var embeddingOnlyConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"embedding": map[string]interface{}{
			"type":        "openai",
			"serviceName": "openai-service",
			"serviceHost": "api.openai.com",
			"servicePort": 443,
			"timeout":     20000,
			"model":       "text-embedding-ada-002",
			"apiKey":      "test-openai-key",
		},
		"cacheKeyStrategy": "lastQuestion",
		"cacheKeyFrom":     "messages.@reverse.0.content",
		"cacheValueFrom":   "choices.0.message.content",
	})
	return data
}()

// 测试配置：仅向量数据库配置
var vectorOnlyConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"vector": map[string]interface{}{
			"type":         "chroma",
			"serviceName":  "chroma-service",
			"serviceHost":  "chroma.example.com",
			"servicePort":  8000,
			"collectionID": "test-collection",
		},
		"cacheKeyStrategy": "lastQuestion",
		"cacheKeyFrom":     "messages.@reverse.0.content",
		"cacheValueFrom":   "choices.0.message.content",
	})
	return data
}()

// 测试配置：禁用缓存策略
var disabledCacheConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"cache": map[string]interface{}{
			"type":        "redis",
			"serviceName": "redis.static",
			"servicePort": 6379,
		},
		"provider":         "dashscope",
		"cacheKeyStrategy": "disabled",
	})
	return data
}()

// 测试配置：无效的缓存键策略
var invalidCacheKeyStrategyConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"cache": map[string]interface{}{
			"type":        "redis",
			"serviceName": "redis.static",
			"servicePort": 6379,
		},
		"provider":         "dashscope",
		"cacheKeyStrategy": "invalidStrategy",
	})
	return data
}()

var missingProviderConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"cache": map[string]interface{}{
			"type":        "redis",
			"serviceName": "redis.static",
			"servicePort": 6379,
		},
	})
	return data
}()

// 测试配置：无效的缓存作用域
var invalidCacheScopeConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"cache": map[string]interface{}{
			"type":        "redis",
			"serviceName": "redis.static",
			"servicePort": 6379,
		},
		"provider":   "dashscope",
		"cacheScope": "unsafe-shared",
	})
	return data
}()

var invalidCacheSoftTTLConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"cache": map[string]interface{}{
			"type":         "redis",
			"serviceName":  "redis.static",
			"servicePort":  6379,
			"cacheTTL":     0,
			"cacheSoftTTL": 60,
		},
		"provider": "dashscope",
	})
	return data
}()

var invalidCacheModeConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"cache": map[string]interface{}{
			"type":        "redis",
			"serviceName": "redis.static",
			"servicePort": 6379,
		},
		"provider":  "dashscope",
		"cacheMode": "unsafe",
	})
	return data
}()

var l1CacheEnabledConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"cache": map[string]interface{}{
			"type":        "redis",
			"serviceName": "redis.static",
			"servicePort": 6379,
		},
		"l1Cache": map[string]interface{}{
			"enabled":    true,
			"maxEntries": 16,
			"ttlSeconds": 10,
		},
		"provider": "dashscope",
	})
	return data
}()

var invalidL1CacheConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"cache": map[string]interface{}{
			"type":        "redis",
			"serviceName": "redis.static",
			"servicePort": 6379,
		},
		"l1Cache": map[string]interface{}{
			"enabled":    true,
			"maxEntries": 0,
			"ttlSeconds": 10,
		},
		"provider": "dashscope",
	})
	return data
}()

var oversizedL1CacheConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"cache": map[string]interface{}{
			"type":        "redis",
			"serviceName": "redis.static",
			"servicePort": 6379,
		},
		"l1Cache": map[string]interface{}{
			"enabled":    true,
			"maxEntries": 1025,
			"ttlSeconds": 61,
		},
		"provider": "dashscope",
	})
	return data
}()

var requestCollapseEnabledConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"cache": map[string]interface{}{
			"type":        "redis",
			"serviceName": "redis.static",
			"servicePort": 6379,
		},
		"requestCollapse": map[string]interface{}{
			"enabled":         true,
			"distributedLock": true,
			"staleFallback":   true,
		},
		"provider": "dashscope",
	})
	return data
}()

var invalidRequestCollapseConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"cache": map[string]interface{}{
			"type":        "redis",
			"serviceName": "redis.static",
			"servicePort": 6379,
		},
		"requestCollapse": map[string]interface{}{
			"enabled":          true,
			"waitAttempts":     0,
			"waitMilliseconds": 0,
			"lockTTLSeconds":   0,
		},
		"provider": "dashscope",
	})
	return data
}()

// 测试配置：缺少必需字段
var missingRequiredFieldsConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"cacheKeyStrategy": "lastQuestion",
		"cacheKeyFrom":     "messages.@reverse.0.content",
		// 缺少cache、embedding、vector配置
	})
	return data
}()

// 测试配置：Redis高级配置
var redisAdvancedConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"cache": map[string]interface{}{
			"type":           "redis",
			"serviceName":    "redis.static",
			"servicePort":    6379,
			"serviceHost":    "redis.example.com",
			"username":       "testuser",
			"password":       "testpass",
			"timeout":        15000,
			"cacheTTL":       7200,
			"cacheKeyPrefix": "custom-prefix:",
			"database":       1,
		},
		"provider":         "dashscope",
		"cacheKeyStrategy": "lastQuestion",
		"cacheKeyFrom":     "messages.@reverse.0.content",
		"cacheValueFrom":   "choices.0.message.content",
	})
	return data
}()

// 测试配置：作用域缓存配置
var scopedCacheConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"cache": map[string]interface{}{
			"type":        "redis",
			"serviceName": "redis.static",
			"servicePort": 6379,
		},
		"cache_scope":          "consumer",
		"tenant_header":        "x-tenant-id",
		"consumer_header":      "x-consumer-id",
		"provider":             "dashscope",
		"cache_policy_version": "policy-20260625",
	})
	return data
}()

func TestParseConfig(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		// 测试基础Redis缓存配置解析
		t.Run("basic redis config", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			configRaw, err := host.GetMatchConfig()
			require.NoError(t, err)
			require.NotNil(t, configRaw)

			config, ok := configRaw.(*config.PluginConfig)
			require.True(t, ok, "config should be of type *PluginConfig")

			// 验证缓存键策略
			require.Equal(t, "lastQuestion", config.CacheKeyStrategy)
			require.Equal(t, "messages.@reverse.0.content", config.CacheKeyFrom)
			require.Equal(t, "choices.0.message.content", config.CacheValueFrom)
			require.Equal(t, "choices.0.delta.content", config.CacheStreamValueFrom)
			require.Equal(t, "global", config.CacheScope)
			require.Equal(t, "x-mse-tenant", config.TenantHeader)
			require.Equal(t, "x-mse-consumer", config.ConsumerHeader)
			require.Equal(t, "v1", config.CachePolicyVersion)
			require.Equal(t, 3600, config.CacheHardTTLSeconds())
			require.Equal(t, 3600, config.CacheSoftTTLSeconds())
			require.False(t, config.StaleWhileRevalidateEnabled())
			require.Equal(t, "exact-only", config.CacheMode)

			// 验证响应模板
			require.Contains(t, config.ResponseTemplate, "from-cache")
			require.Contains(t, config.StreamResponseTemplate, "from-cache")

			// 验证语义缓存默认值
			require.False(t, config.EnableSemanticCache)
			require.False(t, config.L1Cache.Enabled)
			require.Equal(t, 0, config.L1Cache.MaxEntries)
			require.Equal(t, 0, config.L1Cache.TTLSeconds)
			require.False(t, config.RequestCollapse.Enabled)
		})

		// 测试完整配置解析
		t.Run("complete config", func(t *testing.T) {
			host, status := test.NewTestHost(completeConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			configRaw, err := host.GetMatchConfig()
			require.NoError(t, err)
			require.NotNil(t, configRaw)

			config, ok := configRaw.(*config.PluginConfig)
			require.True(t, ok, "config should be of type *PluginConfig")

			// 验证缓存键策略
			require.Equal(t, "lastQuestion", config.CacheKeyStrategy)
			require.Equal(t, "messages.@reverse.0.content", config.CacheKeyFrom)
			require.Equal(t, "choices.0.message.content", config.CacheValueFrom)

			// 验证语义缓存
			require.True(t, config.EnableSemanticCache)
			require.Equal(t, "semantic", config.CacheMode)

			// 验证响应模板
			require.Contains(t, config.ResponseTemplate, "from-cache")
			require.Contains(t, config.StreamResponseTemplate, "from-cache")
		})

		// 测试最小配置解析（使用默认值）
		t.Run("minimal config with defaults", func(t *testing.T) {
			host, status := test.NewTestHost(minimalConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			configRaw, err := host.GetMatchConfig()
			require.NoError(t, err)
			require.NotNil(t, configRaw)

			config, ok := configRaw.(*config.PluginConfig)
			require.True(t, ok, "config should be of type *PluginConfig")

			// 验证默认值
			require.Equal(t, "lastQuestion", config.CacheKeyStrategy)
			require.Equal(t, "messages.@reverse.0.content", config.CacheKeyFrom)
			require.Equal(t, "choices.0.message.content", config.CacheValueFrom)
			require.Equal(t, "choices.0.delta.content", config.CacheStreamValueFrom)
			require.Equal(t, "tenant", config.CacheScope)
			require.Equal(t, "x-mse-tenant", config.TenantHeader)
			require.Equal(t, "x-mse-consumer", config.ConsumerHeader)
			require.Equal(t, "v1", config.CachePolicyVersion)
			require.Equal(t, 3600, config.CacheHardTTLSeconds())
			require.Equal(t, 3600, config.CacheSoftTTLSeconds())
			require.False(t, config.StaleWhileRevalidateEnabled())
			require.Equal(t, "exact-only", config.CacheMode)
			require.False(t, config.EnableSemanticCache)
		})

		// 测试仅缓存配置
		t.Run("cache only config", func(t *testing.T) {
			host, status := test.NewTestHost(cacheOnlyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			configRaw, err := host.GetMatchConfig()
			require.NoError(t, err)
			require.NotNil(t, configRaw)

			config, ok := configRaw.(*config.PluginConfig)
			require.True(t, ok, "config should be of type *PluginConfig")

			// 验证缓存键策略
			require.Equal(t, "allQuestions", config.CacheKeyStrategy)
			require.False(t, config.EnableSemanticCache)
		})

		// 测试仅嵌入模型配置
		t.Run("embedding only config", func(t *testing.T) {
			host, status := test.NewTestHost(embeddingOnlyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			configRaw, err := host.GetMatchConfig()
			require.NoError(t, err)
			require.NotNil(t, configRaw)

			config, ok := configRaw.(*config.PluginConfig)
			require.True(t, ok, "config should be of type *PluginConfig")

			// 验证缓存键策略
			require.Equal(t, "lastQuestion", config.CacheKeyStrategy)
			require.False(t, config.EnableSemanticCache)
		})

		// 测试仅向量数据库配置
		t.Run("vector only config", func(t *testing.T) {
			host, status := test.NewTestHost(vectorOnlyConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			configRaw, err := host.GetMatchConfig()
			require.NoError(t, err)
			require.NotNil(t, configRaw)

			config, ok := configRaw.(*config.PluginConfig)
			require.True(t, ok, "config should be of type *PluginConfig")

			// 验证缓存键策略
			require.Equal(t, "lastQuestion", config.CacheKeyStrategy)
			require.False(t, config.EnableSemanticCache)
		})

		// 测试禁用缓存策略
		t.Run("disabled cache strategy", func(t *testing.T) {
			host, status := test.NewTestHost(disabledCacheConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			configRaw, err := host.GetMatchConfig()
			require.NoError(t, err)
			require.NotNil(t, configRaw)

			config, ok := configRaw.(*config.PluginConfig)
			require.True(t, ok, "config should be of type *PluginConfig")

			// 验证缓存键策略
			require.Equal(t, "disabled", config.CacheKeyStrategy)
		})

		// 测试Redis高级配置
		t.Run("redis advanced config", func(t *testing.T) {
			host, status := test.NewTestHost(redisAdvancedConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			configRaw, err := host.GetMatchConfig()
			require.NoError(t, err)
			require.NotNil(t, configRaw)

			config, ok := configRaw.(*config.PluginConfig)
			require.True(t, ok, "config should be of type *PluginConfig")

			// 验证缓存键策略
			require.Equal(t, "lastQuestion", config.CacheKeyStrategy)
			require.Equal(t, "messages.@reverse.0.content", config.CacheKeyFrom)
			require.Equal(t, 7200, config.CacheHardTTLSeconds())
			require.Equal(t, 7200, config.CacheSoftTTLSeconds())
		})

		t.Run("scoped cache config", func(t *testing.T) {
			host, status := test.NewTestHost(scopedCacheConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			configRaw, err := host.GetMatchConfig()
			require.NoError(t, err)
			require.NotNil(t, configRaw)

			config, ok := configRaw.(*config.PluginConfig)
			require.True(t, ok, "config should be of type *PluginConfig")

			require.Equal(t, "consumer", config.CacheScope)
			require.Equal(t, "x-tenant-id", config.TenantHeader)
			require.Equal(t, "x-consumer-id", config.ConsumerHeader)
			require.Equal(t, "dashscope", config.Provider)
			require.Equal(t, "policy-20260625", config.CachePolicyVersion)
		})

		t.Run("l1 cache config", func(t *testing.T) {
			host, status := test.NewTestHost(l1CacheEnabledConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			configRaw, err := host.GetMatchConfig()
			require.NoError(t, err)
			require.NotNil(t, configRaw)

			config, ok := configRaw.(*config.PluginConfig)
			require.True(t, ok, "config should be of type *PluginConfig")

			require.True(t, config.L1Cache.Enabled)
			require.Equal(t, 16, config.L1Cache.MaxEntries)
			require.Equal(t, 10, config.L1Cache.TTLSeconds)
			require.NotNil(t, config.GetL1CacheStore())
		})

		t.Run("request collapse config", func(t *testing.T) {
			host, status := test.NewTestHost(requestCollapseEnabledConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			configRaw, err := host.GetMatchConfig()
			require.NoError(t, err)
			require.NotNil(t, configRaw)

			config, ok := configRaw.(*config.PluginConfig)
			require.True(t, ok, "config should be of type *PluginConfig")

			require.True(t, config.RequestCollapse.Enabled)
			require.True(t, config.RequestCollapse.DistributedLock)
			require.True(t, config.RequestCollapse.StaleFallback)
			require.Equal(t, 1, config.RequestCollapse.WaitAttempts)
			require.Equal(t, 2500, config.RequestCollapse.WaitMilliseconds)
			require.Equal(t, 5, config.RequestCollapse.LockTTLSeconds)
			require.NotNil(t, config.GetRequestCollapseStore())
		})

		// 测试无效的缓存键策略
		t.Run("invalid cache key strategy", func(t *testing.T) {
			host, status := test.NewTestHost(invalidCacheKeyStrategyConfig)
			defer host.Reset()
			// 由于无效的缓存键策略，配置应该失败
			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})

		t.Run("missing provider config", func(t *testing.T) {
			host, status := test.NewTestHost(missingProviderConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})

		t.Run("invalid cache scope", func(t *testing.T) {
			host, status := test.NewTestHost(invalidCacheScopeConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})

		t.Run("invalid cache soft ttl without hard ttl", func(t *testing.T) {
			host, status := test.NewTestHost(invalidCacheSoftTTLConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})

		t.Run("invalid cache mode", func(t *testing.T) {
			host, status := test.NewTestHost(invalidCacheModeConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})

		t.Run("invalid l1 cache config", func(t *testing.T) {
			host, status := test.NewTestHost(invalidL1CacheConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})

		t.Run("oversized l1 cache config", func(t *testing.T) {
			host, status := test.NewTestHost(oversizedL1CacheConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})

		t.Run("invalid request collapse config", func(t *testing.T) {
			host, status := test.NewTestHost(invalidRequestCollapseConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})

		// 测试缺少必需字段的配置
		t.Run("missing required fields config", func(t *testing.T) {
			host, status := test.NewTestHost(missingRequiredFieldsConfig)
			defer host.Reset()
			// 由于缺少必需的Provider配置，配置应该失败
			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})
	})
}

func TestOnHttpRequestHeaders(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		// 测试基本请求头处理
		t.Run("basic request headers", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 设置请求头
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})

			// 应该返回HeaderStopIteration，因为需要等待请求体
			require.Equal(t, types.HeaderStopIteration, action)
		})

		// 测试跳过缓存请求头
		t.Run("skip cache header", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 设置跳过缓存的请求头
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
				{"x-higress-skip-ai-cache", "on"},
			})

			// 应该返回ActionContinue，因为跳过了缓存
			require.Equal(t, types.ActionContinue, action)
		})

		t.Run("request cache mode skip header", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
				{"x-higress-ai-cache-mode", "skip"},
			})

			require.Equal(t, types.ActionContinue, action)
		})

		// 测试无内容类型的请求头
		t.Run("no content type header", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 设置无内容类型的请求头
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
			})

			// 应该返回ActionContinue，因为没有内容类型
			require.Equal(t, types.ActionContinue, action)
		})

		// 测试非JSON内容类型
		t.Run("non-json content type", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 设置非JSON内容类型的请求头
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "text/plain"},
			})

			// 应该返回ActionContinue，因为内容类型不是JSON
			require.Equal(t, types.ActionContinue, action)
		})
	})
}

func TestOnHttpRequestBody(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		// 测试基本请求体处理 - 最后问题策略
		t.Run("basic request body with last question strategy", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 设置请求头
			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})

			// 构造请求体
			requestBody := `{
				"model": "qwen-turbo",
				"messages": [
					{
						"role": "user",
						"content": "今天天气怎么样？"
					},
					{
						"role": "assistant",
						"content": "今天天气晴朗"
					},
					{
						"role": "user",
						"content": "明天呢？"
					}
				],
				"stream": false
			}`

			// 调用请求体处理
			action := host.CallOnHttpRequestBody([]byte(requestBody))

			// 应该返回ActionPause，因为需要等待缓存检查结果
			require.Equal(t, types.ActionPause, action)
		})

		t.Run("route cache mode off bypasses lookup", func(t *testing.T) {
			routeOffConfig := func() json.RawMessage {
				data, _ := json.Marshal(map[string]interface{}{
					"cache": map[string]interface{}{
						"type":        "redis",
						"serviceName": "redis.static",
						"servicePort": 6379,
					},
					"provider":         "dashscope",
					"cacheScope":       "global",
					"cacheMode":        "off",
					"cacheKeyStrategy": "lastQuestion",
					"cacheKeyFrom":     "messages.@reverse.0.content",
					"cacheValueFrom":   "choices.0.message.content",
				})
				return data
			}()
			host, status := test.NewTestHost(routeOffConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpRequestBody([]byte(`{
				"model": "qwen-turbo",
				"messages": [{"role": "user", "content": "今天天气怎么样？"}],
				"stream": false
			}`))

			require.Equal(t, types.ActionContinue, action)
			require.Empty(t, host.GetRedisCalloutAttributes())
		})

		t.Run("route cache mode shadow bypasses replay lookup", func(t *testing.T) {
			shadowConfig := func() json.RawMessage {
				data, _ := json.Marshal(map[string]interface{}{
					"cache": map[string]interface{}{
						"type":        "redis",
						"serviceName": "redis.static",
						"servicePort": 6379,
					},
					"provider":         "dashscope",
					"cacheScope":       "global",
					"cacheMode":        "shadow",
					"cacheKeyStrategy": "lastQuestion",
					"cacheKeyFrom":     "messages.@reverse.0.content",
					"cacheValueFrom":   "choices.0.message.content",
				})
				return data
			}()
			host, status := test.NewTestHost(shadowConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpRequestBody([]byte(`{
				"model": "qwen-turbo",
				"messages": [{"role": "user", "content": "今天天气怎么样？"}],
				"stream": false
			}`))

			require.Equal(t, types.ActionContinue, action)
			require.Nil(t, host.GetLocalResponse())
			require.Empty(t, host.GetRedisCalloutAttributes())
		})

		t.Run("request with tools bypasses cache read and write", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})
			action := host.CallOnHttpRequestBody([]byte(`{
				"model": "qwen-turbo",
				"messages": [{"role": "user", "content": "查一下订单状态"}],
				"tools": [
					{
						"type": "function",
						"function": {
							"name": "lookup_order",
							"parameters": {"type": "object"}
						}
					}
				],
				"stream": false
			}`))

			require.Equal(t, types.ActionContinue, action)
			require.Empty(t, host.GetRedisCalloutAttributes())

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			responseBody := `{
				"id": "chatcmpl-123",
				"choices": [
					{
						"index": 0,
						"message": {
							"role": "assistant",
							"content": "订单正在配送中"
						},
						"finish_reason": "stop"
					}
				],
				"model": "qwen-turbo",
				"object": "chat.completion"
			}`
			require.Equal(t, types.ActionContinue, host.CallOnHttpResponseBody([]byte(responseBody)))
			require.Empty(t, host.GetRedisCalloutAttributes())
		})

		// 测试所有问题策略
		t.Run("request body with all questions strategy", func(t *testing.T) {
			allQuestionsConfig := func() json.RawMessage {
				data, _ := json.Marshal(map[string]interface{}{
					"cache": map[string]interface{}{
						"type":        "redis",
						"serviceName": "redis.static",
						"servicePort": 6379,
					},
					"provider":         "dashscope",
					"cacheScope":       "global",
					"cacheKeyStrategy": "allQuestions",
					"cacheKeyFrom":     "messages.@reverse.0.content",
					"cacheValueFrom":   "choices.0.message.content",
				})
				return data
			}()

			host, status := test.NewTestHost(allQuestionsConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 设置请求头
			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})

			// 构造请求体
			requestBody := `{
				"model": "qwen-turbo",
				"messages": [
					{
						"role": "user",
						"content": "你好"
					},
					{
						"role": "assistant",
						"content": "你好！"
					},
					{
						"role": "user",
						"content": "今天天气怎么样？"
					}
				],
				"stream": false
			}`

			// 调用请求体处理
			action := host.CallOnHttpRequestBody([]byte(requestBody))

			// 应该返回ActionPause，因为需要等待缓存检查结果
			require.Equal(t, types.ActionPause, action)
		})

		// 测试禁用缓存策略
		t.Run("request body with disabled cache strategy", func(t *testing.T) {
			host, status := test.NewTestHost(disabledCacheConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 设置请求头
			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})

			// 构造请求体
			requestBody := `{
				"model": "qwen-turbo",
				"messages": [
					{
						"role": "user",
						"content": "今天天气怎么样？"
					}
				],
				"stream": false
			}`

			// 调用请求体处理
			action := host.CallOnHttpRequestBody([]byte(requestBody))

			// 应该返回ActionContinue，因为缓存被禁用
			require.Equal(t, types.ActionContinue, action)
		})

		// 测试流式请求
		t.Run("stream request body", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 设置请求头
			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})

			// 构造流式请求体
			requestBody := `{
				"model": "qwen-turbo",
				"messages": [
					{
						"role": "user",
						"content": "今天天气怎么样？"
					}
				],
				"stream": true
			}`

			// 调用请求体处理
			action := host.CallOnHttpRequestBody([]byte(requestBody))

			// 应该返回ActionPause，因为需要等待缓存检查结果
			require.Equal(t, types.ActionPause, action)
		})

		// 测试无效的请求体
		t.Run("invalid request body", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 设置请求头
			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})

			// 构造完全无效的请求体（无法解析为JSON）
			invalidBody := []byte(`{invalid json content`)

			// 调用请求体处理
			action := host.CallOnHttpRequestBody(invalidBody)

			// 应该返回ActionContinue，因为JSON解析失败
			require.Equal(t, types.ActionContinue, action)
		})

		// 测试空消息内容
		t.Run("empty message content", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 设置请求头
			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})

			// 构造空内容的请求体
			requestBody := `{
				"model": "qwen-turbo",
				"messages": [
					{
						"role": "user",
						"content": ""
					}
				],
				"stream": false
			}`

			// 调用请求体处理
			action := host.CallOnHttpRequestBody([]byte(requestBody))

			// 应该返回ActionContinue，因为内容为空
			require.Equal(t, types.ActionContinue, action)
		})
	})
}

func TestOnHttpResponseHeaders(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		// 测试基本响应头处理
		t.Run("basic response headers", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 设置请求头
			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})

			// 设置请求体
			requestBody := `{
				"model": "qwen-turbo",
				"messages": [
					{
						"role": "user",
						"content": "今天天气怎么样？"
					}
				],
				"stream": false
			}`
			host.CallOnHttpRequestBody([]byte(requestBody))

			// 设置响应头
			action := host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})

			// 应该返回ActionContinue
			require.Equal(t, types.ActionContinue, action)
		})

		// 测试跳过缓存的响应头处理
		t.Run("response headers with skip cache", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 设置跳过缓存的请求头
			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
				{"x-higress-skip-ai-cache", "on"},
			})

			// 设置响应头
			action := host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})

			// 应该返回ActionContinue
			require.Equal(t, types.ActionContinue, action)
		})

		// 测试流式响应头
		t.Run("stream response headers", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 设置请求头
			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})

			// 设置流式请求体
			requestBody := `{
				"model": "qwen-turbo",
				"messages": [
					{
						"role": "user",
						"content": "今天天气怎么样？"
					}
				],
				"stream": true
			}`
			host.CallOnHttpRequestBody([]byte(requestBody))

			// 设置流式响应头
			action := host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})

			// 应该返回ActionContinue
			require.Equal(t, types.ActionContinue, action)
		})

		t.Run("non-success response bypasses cache write", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})
			host.CallOnHttpRequestBody([]byte(`{
				"model": "qwen-turbo",
				"messages": [{"role": "user", "content": "介绍杭州西湖"}],
				"stream": false
			}`))
			cacheMissResp := test.CreateRedisRespNull()
			host.CallOnRedisCall(0, cacheMissResp)
			host.CallOnRedisCall(0, cacheMissResp)
			require.Empty(t, host.GetRedisCalloutAttributes())

			action := host.CallOnHttpResponseHeaders([][2]string{
				{":status", "500"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)

			responseBody := `{
				"id": "chatcmpl-123",
				"choices": [
					{
						"index": 0,
						"message": {
							"role": "assistant",
							"content": "这条错误响应不应写入缓存"
						},
						"finish_reason": "stop"
					}
				],
				"model": "qwen-turbo",
				"object": "chat.completion"
			}`
			require.Equal(t, types.ActionContinue, host.CallOnHttpResponseBody([]byte(responseBody)))
			require.Empty(t, host.GetRedisCalloutAttributes())
		})

		t.Run("missing response status bypasses cache write", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})
			host.CallOnHttpRequestBody([]byte(`{
				"model": "qwen-turbo",
				"messages": [{"role": "user", "content": "介绍杭州西湖"}],
				"stream": false
			}`))
			cacheMissResp := test.CreateRedisRespNull()
			host.CallOnRedisCall(0, cacheMissResp)
			host.CallOnRedisCall(0, cacheMissResp)
			require.Empty(t, host.GetRedisCalloutAttributes())

			action := host.CallOnHttpResponseHeaders([][2]string{
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)

			responseBody := `{
				"id": "chatcmpl-123",
				"choices": [
					{
						"index": 0,
						"message": {
							"role": "assistant",
							"content": "缺少状态码的响应不应写入缓存"
						},
						"finish_reason": "stop"
					}
				],
				"model": "qwen-turbo",
				"object": "chat.completion"
			}`
			require.Equal(t, types.ActionContinue, host.CallOnHttpResponseBody([]byte(responseBody)))
			require.Empty(t, host.GetRedisCalloutAttributes())
		})
	})
}

func TestOnHttpResponseBody(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		// 测试基本响应体处理
		t.Run("basic response body", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 设置请求头
			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})

			// 设置请求体
			requestBody := `{
				"model": "qwen-turbo",
				"messages": [
					{
						"role": "user",
						"content": "今天天气怎么样？"
					}
				],
				"stream": false
			}`
			host.CallOnHttpRequestBody([]byte(requestBody))

			// 设置响应头
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})

			// 构造响应体
			expectedResponseBody := `{
				"id": "chatcmpl-123",
				"choices": [
					{
						"index": 0,
						"message": {
							"role": "assistant",
							"content": "今天北京天气晴朗，温度25度"
						},
						"finish_reason": "stop"
					}
				],
				"model": "qwen-turbo",
				"object": "chat.completion",
				"usage": {
					"prompt_tokens": 16,
					"completion_tokens": 9,
					"total_tokens": 25,
					"prompt_tokens_details": {
						"cached_tokens": 0
					}
				}
			}`

			// 调用响应体处理
			action := host.CallOnHttpResponseBody([]byte(expectedResponseBody))

			// 应该返回ActionContinue
			require.Equal(t, types.ActionContinue, action)
			actualResponseBody := string(host.GetResponseBody())
			require.JSONEq(t, expectedResponseBody, actualResponseBody)
		})

		// 测试流式响应体处理
		t.Run("stream response body", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 设置请求头
			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})

			// 设置流式请求体
			requestBody := `{
				"model": "qwen-turbo",
				"messages": [
					{
						"role": "user",
						"content": "今天天气怎么样？"
					}
				],
				"stream": true
			}`
			host.CallOnHttpRequestBody([]byte(requestBody))

			// 设置流式响应头
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})

			// 构造流式响应体
			expectedStreamResponseBody := `data: {"id":"chatcmpl-123","choices":[{"index":0,"delta":{"role":"assistant","content":"今天"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","choices":[{"index":0,"delta":{"role":"assistant","content":"北京"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","choices":[{"index":0,"delta":{"role":"assistant","content":"天气晴朗"},"finish_reason":null}]}

data: [DONE]`

			// 调用响应体处理
			action := host.CallOnHttpStreamingResponseBody([]byte(expectedStreamResponseBody), true)

			// 应该返回ActionContinue
			require.Equal(t, types.ActionContinue, action)
			actualStreamResponseBody := string(host.GetResponseBody())
			require.Equal(t, expectedStreamResponseBody, actualStreamResponseBody)
		})

		t.Run("non-stream response with tool call is not cached", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})
			host.CallOnHttpRequestBody([]byte(`{
				"model": "qwen-turbo",
				"messages": [{"role": "user", "content": "查一下订单状态"}],
				"stream": false
			}`))
			cacheMissResp := test.CreateRedisRespNull()
			host.CallOnRedisCall(0, cacheMissResp)
			host.CallOnRedisCall(0, cacheMissResp)

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			responseBody := `{
				"id": "chatcmpl-123",
				"choices": [
					{
						"index": 0,
						"message": {
							"role": "assistant",
							"content": "我需要查询订单。",
							"tool_calls": [
								{
									"id": "call_1",
									"type": "function",
									"function": {"name": "lookup_order", "arguments": "{}"}
								}
							]
						},
						"finish_reason": "tool_calls"
					}
				],
				"model": "qwen-turbo",
				"object": "chat.completion"
			}`

			require.Equal(t, types.ActionContinue, host.CallOnHttpResponseBody([]byte(responseBody)))
			require.Empty(t, host.GetRedisCalloutAttributes())
		})

		t.Run("missing final assistant content is not cached", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})
			host.CallOnHttpRequestBody([]byte(`{
				"model": "qwen-turbo",
				"messages": [{"role": "user", "content": "介绍杭州西湖"}],
				"stream": false
			}`))
			cacheMissResp := test.CreateRedisRespNull()
			host.CallOnRedisCall(0, cacheMissResp)
			host.CallOnRedisCall(0, cacheMissResp)

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			responseBody := `{
				"id": "chatcmpl-123",
				"choices": [
					{
						"index": 0,
						"message": {
							"role": "assistant",
							"content": ""
						},
						"finish_reason": "stop"
					}
				],
				"model": "qwen-turbo",
				"object": "chat.completion"
			}`

			require.Equal(t, types.ActionContinue, host.CallOnHttpResponseBody([]byte(responseBody)))
			require.Empty(t, host.GetRedisCalloutAttributes())
		})

		t.Run("stream response with tool call is not cached", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})
			host.CallOnHttpRequestBody([]byte(`{
				"model": "qwen-turbo",
				"messages": [{"role": "user", "content": "查一下订单状态"}],
				"stream": true
			}`))
			cacheMissResp := test.CreateRedisRespNull()
			host.CallOnRedisCall(0, cacheMissResp)
			host.CallOnRedisCall(0, cacheMissResp)
			require.Empty(t, host.GetRedisCalloutAttributes())

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})
			contentChunk := `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"role":"assistant","content":"我需要查询订单。"},"finish_reason":null}]}

`
			doneChunk := `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup_order","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}

data: [DONE]

`

			require.Equal(t, types.ActionContinue, host.CallOnHttpStreamingResponseBody([]byte(contentChunk), false))
			require.Equal(t, types.ActionContinue, host.CallOnHttpStreamingResponseBody([]byte(doneChunk), true))
			require.Empty(t, host.GetRedisCalloutAttributes())
		})

		t.Run("stream response missing final assistant content is not uploaded to vector", func(t *testing.T) {
			completeGlobalConfig := func() json.RawMessage {
				var data map[string]interface{}
				require.NoError(t, json.Unmarshal(completeConfig, &data))
				data["cacheScope"] = "global"
				raw, err := json.Marshal(data)
				require.NoError(t, err)
				return raw
			}()
			host, status := test.NewTestHost(completeGlobalConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})
			host.CallOnHttpRequestBody([]byte(`{
				"model": "qwen-turbo",
				"messages": [{"role": "user", "content": "介绍杭州西湖"}],
				"stream": true
			}`))
			cacheMissResp := test.CreateRedisRespNull()
			host.CallOnRedisCall(0, cacheMissResp)
			host.CallOnRedisCall(0, cacheMissResp)

			embeddingResponse := `{
				"output": {
					"embeddings": [
						{
							"embedding": [0.1, 0.2, 0.3, 0.4, 0.5]
						}
					]
				}
			}`
			host.CallOnHttpCall([][2]string{
				{"Content-Type", "application/json"},
				{":status", "200"},
			}, []byte(embeddingResponse))

			vectorQueryResponse := `{
				"code": 200,
				"request_id": "test-request-123",
				"message": "success",
				"output": []
			}`
			host.CallOnHttpCall([][2]string{
				{"Content-Type", "application/json"},
				{":status", "200"},
			}, []byte(vectorQueryResponse))
			require.Empty(t, host.GetRedisCalloutAttributes())
			require.Empty(t, host.GetHttpCalloutAttributes())

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})
			doneChunk := `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","model":"qwen-turbo","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":"stop"}]}

data: [DONE]

`
			require.Equal(t, types.ActionContinue, host.CallOnHttpStreamingResponseBody([]byte(doneChunk), true))
			require.Empty(t, host.GetRedisCalloutAttributes())
			require.Empty(t, host.GetHttpCalloutAttributes())
		})

		// 测试流式响应体处理：首 chunk 只带 role 无 content（混元大模型场景，#3953）
		// 验证：
		//   1) processSSEMessage 不会因为首 chunk 没有 content 而报错，
		//      避免后续 chunk 被 ERROR_PARTIAL_MESSAGE_KEY 短路。
		//   2) 全部 content chunk 累积完成后，data: [DONE] 触发的 processStreamLastChunk
		//      必须把累积内容写入 Redis —— 而不是像修复前那样返回空字符串导致 cacheResponse
		//      直接吞掉。
		t.Run("stream response body with role-only first chunk", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 设置请求头
			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})

			// 设置流式请求体
			requestBody := `{
				"model": "hy-mt2-pro",
				"messages": [
					{
						"role": "user",
						"content": "介绍天津"
					}
				],
				"stream": true
			}`
			host.CallOnHttpRequestBody([]byte(requestBody))

			// 模拟 Redis 缓存未命中（CallOnRedisCall 会消费 callout 列表中的第 0 项，
			// 即上一步 CheckCacheForKey 产生的 GET）。响应后该条目被移除，
			// 流式响应便会走完整的 cacheResponse 写缓存路径。
			host.CallOnRedisCall(0, test.CreateRedisRespNull())

			// 设置流式响应头
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})

			// 首 chunk：只有 delta.role，没有 delta.content（混元/hy-mt2-pro 行为）
			roleOnlyChunk := `data: {"id":"0b8ed3c9-3710-920b-abd9-581a0a77fd28","object":"chat.completion.chunk","created":1781169426,"model":"hy-mt2-pro","choices":[{"index":0,"delta":{"role":"assistant"}}]}

`

			// 中间 chunk：包含实际 content
			contentChunk := `data: {"id":"0b8ed3c9-3710-920b-abd9-581a0a77fd28","object":"chat.completion.chunk","created":1781169426,"model":"hy-mt2-pro","choices":[{"index":0,"delta":{"content":"天津"}}]}

data: {"id":"0b8ed3c9-3710-920b-abd9-581a0a77fd28","object":"chat.completion.chunk","created":1781169426,"model":"hy-mt2-pro","choices":[{"index":0,"delta":{"content":"是历史文化名城"}}]}

`

			// 末 chunk：finish_reason、usage 与流结束标记。usage-only chunk 是
			// OpenAI-compatible stream_options.include_usage 的常见收尾形态。
			doneChunk := `data: {"id":"0b8ed3c9-3710-920b-abd9-581a0a77fd28","object":"chat.completion.chunk","created":1781169426,"model":"hy-mt2-pro","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: {"id":"0b8ed3c9-3710-920b-abd9-581a0a77fd28","object":"chat.completion.chunk","created":1781169426,"model":"hy-mt2-pro","choices":[],"usage":{"prompt_tokens":16,"completion_tokens":9,"total_tokens":25,"prompt_tokens_details":{"cached_tokens":0}}}

data: [DONE]

`

			// 三次调用模拟上游分片推送：首 chunk 不带 content，后续 chunk 带 content
			// 修复前：第一次调用会因首 chunk 无 content 报错，ERROR_PARTIAL_MESSAGE_KEY 被设置，
			// 后续所有 chunk 在 onHttpResponseBody 入口被短路，缓存永远写不进去。
			// 修复后：首 chunk 静默跳过，content chunk 正常累积并写入缓存。
			action := host.CallOnHttpStreamingResponseBody([]byte(roleOnlyChunk), false)
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpStreamingResponseBody([]byte(contentChunk), false)
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpStreamingResponseBody([]byte(doneChunk), true)
			require.Equal(t, types.ActionContinue, action)

			// 在响应 SET 之前先快照 callout 属性 —— 框架在 CallOnRedisCallResponse
			// 时会同步把条目从 map 里清掉，所以必须在消费前抓取。
			redisCalls := host.GetRedisCalloutAttributes()
			require.GreaterOrEqual(t, len(redisCalls), 1,
				"expected a SET redis callout after [DONE], got %d", len(redisCalls))

			// 模拟 Redis SET 响应。
			host.CallOnRedisCall(0, test.CreateRedisRespString("OK"))

			// 断言：上游流式响应走到末尾后，ai-cache 必须真的发出一次 SET，
			// 且写入的值是前面 content chunk 累积出的完整文本。
			setCall := redisCalls[len(redisCalls)-1]
			setQuery := string(setCall.Query)
			require.Contains(t, setQuery, "higress-ai-cache:{aiscope:",
				"SET command should target the scoped cache key derived from the request, got query: %q", setQuery)
			require.NotContains(t, setQuery, "higress-ai-cache:介绍天津",
				"SET command must not use the raw prompt as the Redis key, got query: %q", setQuery)
			require.Contains(t, setQuery, "天津是历史文化名城",
				"SET command should contain the accumulated value, got query: %q", setQuery)
			require.Contains(t, setQuery, `"model":"hy-mt2-pro"`,
				"SET command should preserve upstream stream model, got query: %q", setQuery)
			require.Contains(t, setQuery, `"choices"`,
				"SET command should preserve structured stream choices, got query: %q", setQuery)
			require.Contains(t, setQuery, `"finish_reason":"stop"`,
				"SET command should preserve stream finish reason, got query: %q", setQuery)
			require.Contains(t, setQuery, `"usage":{"prompt_tokens":16,"completion_tokens":9,"total_tokens":25,"prompt_tokens_details":{"cached_tokens":0}}`,
				"SET command should preserve stream final usage, got query: %q", setQuery)
		})

		// 测试流式响应体处理：最后一个 buffer 同时包含最后一段 content 与 data: [DONE]
		// 验证：
		//   1) 同一 buffer 内的最后一段 content 不会被 [DONE] 早 return 丢掉；
		//   2) cacheResponse SET 写入的值是先前 chunk 与本次合并后的完整累积文本。
		// 回归保护：PR #3962 review 指出当 content 与 [DONE] 在同一 lastChunk 到达时，
		// processSSEMessage 会在 [DONE] 处直接 return ctx 旧值，导致最后一段 content 丢失。
		t.Run("stream response body with final content and [DONE] in same chunk", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 设置请求头
			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})

			// 设置流式请求体
			requestBody := `{
				"model": "hy-mt2-pro",
				"messages": [
					{
						"role": "user",
						"content": "介绍天津"
					}
				],
				"stream": true
			}`
			host.CallOnHttpRequestBody([]byte(requestBody))

			// 模拟 Redis 缓存未命中（消费 CheckCacheForKey 的 GET）
			host.CallOnRedisCall(0, test.CreateRedisRespNull())

			// 设置流式响应头
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})

			// 非末次 chunk：先累积 "天津"
			firstChunk := `data: {"id":"0b8ed3c9-3710-920b-abd9-581a0a77fd28","object":"chat.completion.chunk","created":1781169426,"model":"hy-mt2-pro","choices":[{"index":0,"delta":{"content":"天津"}}]}

`

			// 末次 chunk：最后一段 content + [DONE] 同一个 buffer 到达
			finalChunk := `data: {"id":"0b8ed3c9-3710-920b-abd9-581a0a77fd28","object":"chat.completion.chunk","created":1781169426,"model":"hy-mt2-pro","choices":[{"index":0,"delta":{"content":"是历史文化名城"}}]}

data: [DONE]

`

			action := host.CallOnHttpStreamingResponseBody([]byte(firstChunk), false)
			require.Equal(t, types.ActionContinue, action)

			action = host.CallOnHttpStreamingResponseBody([]byte(finalChunk), true)
			require.Equal(t, types.ActionContinue, action)

			// 在消费前快照 callout —— 框架在 CallOnRedisCall 同步把条目从 map 里清掉。
			redisCalls := host.GetRedisCalloutAttributes()
			require.GreaterOrEqual(t, len(redisCalls), 1,
				"expected a SET redis callout after [DONE], got %d", len(redisCalls))

			// 模拟 Redis SET 响应
			host.CallOnRedisCall(0, test.CreateRedisRespString("OK"))

			// 断言 SET value 必须是完整累积文本，而非被 [DONE] 截断后的 "天津"
			setCall := redisCalls[len(redisCalls)-1]
			setQuery := string(setCall.Query)
			require.Contains(t, setQuery, "higress-ai-cache:{aiscope:",
				"SET command should target the scoped cache key derived from the request, got query: %q", setQuery)
			require.NotContains(t, setQuery, "higress-ai-cache:介绍天津",
				"SET command must not use the raw prompt as the Redis key, got query: %q", setQuery)
			require.Contains(t, setQuery, "天津是历史文化名城",
				"SET command should contain the full accumulated value (final chunk content must not be dropped by [DONE]), got query: %q", setQuery)
		})

		// 测试无缓存键的响应体处理
		t.Run("response body without cache key", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 设置响应头（不经过请求处理）
			action := host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})

			// 应该返回ActionContinue
			require.Equal(t, types.ActionContinue, action)

			// 构造响应体
			expectedResponseBody := `{
				"id": "chatcmpl-123",
				"choices": [
					{
						"index": 0,
						"message": {
							"role": "assistant",
							"content": "测试响应"
						},
						"finish_reason": "stop"
					}
				]
			}`

			// 调用响应体处理
			action = host.CallOnHttpStreamingResponseBody([]byte(expectedResponseBody), true)

			// 应该返回ActionContinue
			require.Equal(t, types.ActionContinue, action)

			actualResponseBody := string(host.GetResponseBody())
			require.JSONEq(t, expectedResponseBody, actualResponseBody)
		})
	})
}

// 测试外部服务调用的模拟
func TestExternalServiceCalls(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		// 测试完整的缓存命中流程
		t.Run("complete cache hit flow", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 设置请求头
			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})

			// 设置请求体
			requestBody := `{
				"model": "qwen-turbo",
				"messages": [
					{
						"role": "user",
						"content": "今天天气怎么样？"
					}
				],
				"stream": false
			}`
			host.CallOnHttpRequestBody([]byte(requestBody))
			redisCalls := host.GetRedisCalloutAttributes()
			require.GreaterOrEqual(t, len(redisCalls), 1)
			getQuery := string(redisCalls[len(redisCalls)-1].Query)
			require.Contains(t, getQuery, "higress-ai-cache:{aiscope:")
			require.NotContains(t, getQuery, "今天天气怎么样？")

			// 模拟Redis缓存命中响应
			cacheHitResp := test.CreateRedisRespArray([]interface{}{`{"temperature": 25, "condition": "晴朗", "humidity": 60}`})
			host.CallOnRedisCall(0, cacheHitResp)

			// 完成HTTP请求
			host.CompleteHttp()
		})

		t.Run("legacy raw key fallback flow", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
				{"x-mse-tenant", "tenant-a"},
			})

			requestBody := `{
				"model": "qwen-turbo",
				"messages": [
					{
						"role": "user",
						"content": "今天天气怎么样？"
					}
				],
				"stream": false
			}`
			host.CallOnHttpRequestBody([]byte(requestBody))

			redisCalls := host.GetRedisCalloutAttributes()
			require.GreaterOrEqual(t, len(redisCalls), 1)
			scopedGetQuery := string(redisCalls[len(redisCalls)-1].Query)
			require.Contains(t, scopedGetQuery, "higress-ai-cache:{aiscope:")

			host.CallOnRedisCall(0, test.CreateRedisRespNull())

			redisCalls = host.GetRedisCalloutAttributes()
			require.GreaterOrEqual(t, len(redisCalls), 1)
			legacyGetQuery := string(redisCalls[len(redisCalls)-1].Query)
			require.Contains(t, legacyGetQuery, "higress-ai-cache:今天天气怎么样？")

			cacheHitResp := test.CreateRedisRespArray([]interface{}{"legacy answer"})
			host.CallOnRedisCall(0, cacheHitResp)
			host.CompleteHttp()
		})

		// 测试语义缓存流程（embedding + vector查询）
		t.Run("semantic cache flow with embedding and vector", func(t *testing.T) {
			semanticConfig := func() json.RawMessage {
				data, _ := json.Marshal(map[string]interface{}{
					"cache": map[string]interface{}{
						"type":        "redis",
						"serviceName": "redis.static",
						"servicePort": 6379,
					},
					"embedding": map[string]interface{}{
						"type":        "dashscope",
						"apiKey":      "test-dashscope-key",
						"serviceName": "dashscope.static",
						"servicePort": 8080,
					},
					"vector": map[string]interface{}{
						"type":              "dashvector",
						"serviceName":       "dashvector-service",
						"serviceHost":       "dashvector.example.com",
						"servicePort":       8081,
						"apiKey":            "test-dashvector-key",
						"collectionID":      "test-collection",
						"threshold":         0.8,
						"thresholdRelation": "gt",
					},
					"provider":            "dashscope",
					"cacheScope":          "global",
					"enableSemanticCache": true,
					"cacheKeyStrategy":    "lastQuestion",
					"cacheKeyFrom":        "messages.@reverse.0.content",
					"cacheValueFrom":      "choices.0.message.content",
				})
				return data
			}()

			host, status := test.NewTestHost(semanticConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 设置请求头
			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})

			// 设置请求体
			requestBody := `{
				"model": "qwen-turbo",
				"messages": [
					{
						"role": "user",
						"content": "介绍杭州西湖"
					}
				],
				"stream": false
			}`
			host.CallOnHttpRequestBody([]byte(requestBody))

			// 模拟 scoped Redis 缓存未命中（返回 null）和 legacy raw-key 兼容查询未命中
			cacheMissResp := test.CreateRedisRespNull()
			host.CallOnRedisCall(0, cacheMissResp)
			host.CallOnRedisCall(0, cacheMissResp)

			// 模拟DashScope embedding服务响应
			embeddingResponse := `{
				"output": {
					"embeddings": [
						{
							"embedding": [0.1, 0.2, 0.3, 0.4, 0.5]
						}
					]
				}
			}`
			host.CallOnHttpCall([][2]string{
				{"Content-Type", "application/json"},
				{":status", "200"},
			}, []byte(embeddingResponse))

			// 模拟DashVector向量查询响应
			vectorQueryResponse := `{
				"code": 200,
				"request_id": "test-request-123",
				"message": "success",
				"output": [
					{
						"id": "1",
						"fields": {
							"query": "介绍杭州西湖",
							"answer": "杭州西湖是中国著名的风景名胜，兼具自然景观与历史文化。"
						},
						"score": 0.95
					}
				]
			}`
			host.CallOnHttpCall([][2]string{
				{"Content-Type", "application/json"},
				{":status", "200"},
			}, []byte(vectorQueryResponse))

			redisCalls := host.GetRedisCalloutAttributes()
			require.GreaterOrEqual(t, len(redisCalls), 1,
				"expected a compatibility SET redis callout after accepted vector answer, got %d", len(redisCalls))
			setQuery := string(redisCalls[len(redisCalls)-1].Query)
			require.Contains(t, setQuery, "higress-ai-cache:{aiscope:")
			require.Contains(t, setQuery, "杭州西湖是中国著名的风景名胜")
			require.NotContains(t, setQuery, `"version":1`,
				"vector answer payloads are not authoritative structured cache entries")
			require.NotContains(t, setQuery, `"usage"`,
				"vector answer payloads must not synthesize zero-usage structured entries")

			localResponse := host.GetLocalResponse()
			require.NotNil(t, localResponse)
			require.Contains(t, string(localResponse.Data), "from-cache")
			host.CallOnRedisCall(0, test.CreateRedisRespString("OK"))

			// 完成HTTP请求
			host.CompleteHttp()
		})

		runSemanticBypassCase := func(name string, requestBody string) {
			t.Run(name, func(t *testing.T) {
				semanticConfig := func() json.RawMessage {
					data, _ := json.Marshal(map[string]interface{}{
						"cache": map[string]interface{}{
							"type":        "redis",
							"serviceName": "redis.static",
							"servicePort": 6379,
						},
						"embedding": map[string]interface{}{
							"type":        "dashscope",
							"apiKey":      "test-dashscope-key",
							"serviceName": "dashscope.static",
							"servicePort": 8080,
						},
						"vector": map[string]interface{}{
							"type":              "dashvector",
							"serviceName":       "dashvector-service",
							"serviceHost":       "dashvector.example.com",
							"servicePort":       8081,
							"apiKey":            "test-dashvector-key",
							"collectionID":      "test-collection",
							"threshold":         0.8,
							"thresholdRelation": "gt",
						},
						"provider":            "dashscope",
						"cacheScope":          "global",
						"enableSemanticCache": true,
						"cacheKeyStrategy":    "lastQuestion",
						"cacheKeyFrom":        "messages.@reverse.0.content",
						"cacheValueFrom":      "choices.0.message.content",
					})
					return data
				}()

				host, status := test.NewTestHost(semanticConfig)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)

				host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "example.com"},
					{":path", "/api/chat"},
					{":method", "POST"},
					{"content-type", "application/json"},
				})

				host.CallOnHttpRequestBody([]byte(requestBody))
				cacheMissResp := test.CreateRedisRespNull()
				host.CallOnRedisCall(0, cacheMissResp)

				require.Empty(t, host.GetRedisCalloutAttributes())
				require.Empty(t, host.GetHttpCalloutAttributes())
				require.Nil(t, host.GetLocalResponse())
				host.CompleteHttp()
			})
		}

		runSemanticBypassCase("sampling settings bypass semantic lookup", `{
			"model": "qwen-turbo",
			"messages": [{"role": "user", "content": "介绍杭州西湖"}],
			"temperature": 0.7,
			"stream": false
		}`)
		runSemanticBypassCase("dynamic data intent bypasses semantic lookup", `{
			"model": "qwen-turbo",
			"messages": [{"role": "user", "content": "what is the weather today?"}],
			"stream": false
		}`)
		runSemanticBypassCase("weak intent bypasses semantic lookup", `{
			"model": "qwen-turbo",
			"messages": [{"role": "user", "content": "continue"}],
			"stream": false
		}`)
		runSemanticBypassCase("ambiguous multi-turn prompt bypasses semantic lookup", `{
			"model": "qwen-turbo",
			"messages": [
				{"role": "user", "content": "给我三个缓存策略"},
				{"role": "assistant", "content": "第一是精确缓存，第二是语义缓存，第三是旁路。"},
				{"role": "user", "content": "第二个"}
			],
			"stream": false
		}`)

		t.Run("sampling settings bypass legacy raw-key fallback", func(t *testing.T) {
			semanticConfig := func() json.RawMessage {
				data, _ := json.Marshal(map[string]interface{}{
					"cache": map[string]interface{}{
						"type":        "redis",
						"serviceName": "redis.static",
						"servicePort": 6379,
					},
					"embedding": map[string]interface{}{
						"type":        "dashscope",
						"apiKey":      "test-dashscope-key",
						"serviceName": "dashscope.static",
						"servicePort": 8080,
					},
					"vector": map[string]interface{}{
						"type":              "dashvector",
						"serviceName":       "dashvector-service",
						"serviceHost":       "dashvector.example.com",
						"servicePort":       8081,
						"apiKey":            "test-dashvector-key",
						"collectionID":      "test-collection",
						"threshold":         0.8,
						"thresholdRelation": "gt",
					},
					"provider":            "dashscope",
					"cacheScope":          "global",
					"enableSemanticCache": true,
					"cacheKeyStrategy":    "lastQuestion",
					"cacheKeyFrom":        "messages.@reverse.0.content",
					"cacheValueFrom":      "choices.0.message.content",
				})
				return data
			}()
			host, status := test.NewTestHost(semanticConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})
			host.CallOnHttpRequestBody([]byte(`{
				"model": "qwen-turbo",
				"messages": [{"role": "user", "content": "介绍杭州西湖"}],
				"temperature": 0.7,
				"stream": false
			}`))

			redisCalls := host.GetRedisCalloutAttributes()
			require.Len(t, redisCalls, 1)
			require.NotContains(t, string(redisCalls[0].Query), "higress-ai-cache:介绍杭州西湖")

			host.CallOnRedisCall(0, test.CreateRedisRespNull())
			require.Empty(t, host.GetRedisCalloutAttributes())
			require.Empty(t, host.GetHttpCalloutAttributes())
			require.Nil(t, host.GetLocalResponse())
		})

		t.Run("request cache mode exact-only bypasses semantic lookup", func(t *testing.T) {
			semanticConfig := func() json.RawMessage {
				data, _ := json.Marshal(map[string]interface{}{
					"cache": map[string]interface{}{
						"type":        "redis",
						"serviceName": "redis.static",
						"servicePort": 6379,
					},
					"embedding": map[string]interface{}{
						"type":        "dashscope",
						"apiKey":      "test-dashscope-key",
						"serviceName": "dashscope.static",
						"servicePort": 8080,
					},
					"vector": map[string]interface{}{
						"type":              "dashvector",
						"serviceName":       "dashvector-service",
						"serviceHost":       "dashvector.example.com",
						"servicePort":       8081,
						"apiKey":            "test-dashvector-key",
						"collectionID":      "test-collection",
						"threshold":         0.8,
						"thresholdRelation": "gt",
					},
					"provider":            "dashscope",
					"cacheScope":          "global",
					"enableSemanticCache": true,
					"cacheKeyStrategy":    "lastQuestion",
					"cacheKeyFrom":        "messages.@reverse.0.content",
					"cacheValueFrom":      "choices.0.message.content",
				})
				return data
			}()

			host, status := test.NewTestHost(semanticConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
				{"x-higress-ai-cache-mode", "exact-only"},
			})

			requestBody := `{
				"model": "qwen-turbo",
				"messages": [
					{
						"role": "user",
						"content": "今天天气怎么样？"
					}
				],
				"stream": false
			}`
			host.CallOnHttpRequestBody([]byte(requestBody))
			cacheMissResp := test.CreateRedisRespNull()
			host.CallOnRedisCall(0, cacheMissResp)
			host.CallOnRedisCall(0, cacheMissResp)

			require.Empty(t, host.GetRedisCalloutAttributes())
			require.Empty(t, host.GetHttpCalloutAttributes())
			require.Nil(t, host.GetLocalResponse())
			host.CompleteHttp()
		})

		// 测试流式响应的缓存流程
		t.Run("streaming response cache flow", func(t *testing.T) {
			streamConfig := func() json.RawMessage {
				data, _ := json.Marshal(map[string]interface{}{
					"cache": map[string]interface{}{
						"type":        "redis",
						"serviceName": "redis.static",
						"servicePort": 6379,
					},
					"provider":         "dashscope",
					"cacheScope":       "global",
					"cacheKeyStrategy": "lastQuestion",
					"cacheKeyFrom":     "messages.@reverse.0.content",
					"cacheValueFrom":   "choices.0.message.content",
					"streamResponseTemplate": `data: {"id":"chatcmpl-123","choices":[{"index":0,"delta":{"role":"assistant","content":"%s"},"finish_reason":"stop"}]}

data: [DONE]`,
				})
				return data
			}()

			host, status := test.NewTestHost(streamConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 设置请求头
			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})

			// 设置流式请求体
			requestBody := `{
				"model": "qwen-turbo",
				"messages": [
					{
						"role": "user",
						"content": "今天天气怎么样？"
					}
				],
				"stream": true
			}`
			host.CallOnHttpRequestBody([]byte(requestBody))

			// 模拟Redis缓存命中响应
			cacheHitResp := test.CreateRedisRespArray([]interface{}{`{"temperature": 25, "condition": "晴朗", "humidity": 60}`})
			host.CallOnRedisCall(0, cacheHitResp)

			// 完成HTTP请求
			host.CompleteHttp()
		})

		t.Run("request cache mode no-store skips response cache write", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
				{"x-higress-ai-cache-mode", "no-store"},
			})

			requestBody := `{
				"model": "qwen-turbo",
				"messages": [
					{
						"role": "user",
						"content": "今天天气怎么样？"
					}
				],
				"stream": false
			}`
			host.CallOnHttpRequestBody([]byte(requestBody))
			cacheMissResp := test.CreateRedisRespNull()
			host.CallOnRedisCall(0, cacheMissResp)
			host.CallOnRedisCall(0, cacheMissResp)
			require.Empty(t, host.GetRedisCalloutAttributes())

			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			responseBody := `{
				"id": "chatcmpl-123",
				"choices": [
					{
						"index": 0,
						"message": {
							"role": "assistant",
							"content": "今天北京天气晴朗，温度25度"
						},
						"finish_reason": "stop"
					}
				],
				"model": "qwen-turbo",
				"object": "chat.completion",
				"usage": {
					"prompt_tokens": 16,
					"completion_tokens": 9,
					"total_tokens": 25
				}
			}`
			action := host.CallOnHttpResponseBody([]byte(responseBody))

			require.Equal(t, types.ActionContinue, action)
			require.Empty(t, host.GetRedisCalloutAttributes())
		})

		// 测试缓存存储流程
		t.Run("cache storage flow", func(t *testing.T) {
			host, status := test.NewTestHost(basicRedisConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 设置请求头
			host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/api/chat"},
				{":method", "POST"},
				{"content-type", "application/json"},
			})

			// 设置请求体
			requestBody := `{
				"model": "qwen-turbo",
				"messages": [
					{
						"role": "user",
						"content": "今天天气怎么样？"
					}
				],
				"stream": false
			}`
			host.CallOnHttpRequestBody([]byte(requestBody))

			// 模拟 scoped Redis 缓存未命中和 legacy raw-key 兼容查询未命中
			cacheMissResp := test.CreateRedisRespNull()
			host.CallOnRedisCall(0, cacheMissResp)
			host.CallOnRedisCall(0, cacheMissResp)

			// 设置响应头
			host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})

			// 构造响应体
			responseBody := `{
				"id": "chatcmpl-123",
				"choices": [
					{
						"index": 0,
						"message": {
							"role": "assistant",
							"content": "今天北京天气晴朗，温度25度"
						},
						"finish_reason": "stop"
					}
				],
				"model": "qwen-turbo",
				"object": "chat.completion",
				"usage": {
					"prompt_tokens": 16,
					"completion_tokens": 9,
					"total_tokens": 25,
					"prompt_tokens_details": {
						"cached_tokens": 0
					}
				}
			}`

			// 调用响应体处理，这会触发缓存存储
			action := host.CallOnHttpResponseBody([]byte(responseBody))

			// 应该返回ActionContinue
			require.Equal(t, types.ActionContinue, action)

			redisCalls := host.GetRedisCalloutAttributes()
			require.GreaterOrEqual(t, len(redisCalls), 1,
				"expected a SET redis callout after response body, got %d", len(redisCalls))
			setQuery := string(redisCalls[len(redisCalls)-1].Query)
			require.Contains(t, setQuery, "higress-ai-cache:{aiscope:")
			require.NotContains(t, setQuery, "今天天气怎么样？")
			require.Contains(t, setQuery, `"version":1`)
			require.Contains(t, setQuery, `"answer"`)
			require.Contains(t, setQuery, `"model":"qwen-turbo"`)
			require.Contains(t, setQuery, `"choices"`)
			require.Contains(t, setQuery, `"usage":{"prompt_tokens":16,"completion_tokens":9,"total_tokens":25,"prompt_tokens_details":{"cached_tokens":0}}`)
			require.Contains(t, setQuery, `"finish_reason":"stop"`)
			require.Contains(t, setQuery, "今天北京天气晴朗，温度25度")
			require.NotContains(t, setQuery, `"created_at":0`)
			require.NotContains(t, setQuery, `"soft_expires_at":0`)
			require.NotContains(t, setQuery, `"hard_expires_at":0`)

			// 模拟Redis存储操作
			storeResp := test.CreateRedisRespArray([]interface{}{"OK"})
			host.CallOnRedisCall(0, storeResp)

			// 完成HTTP请求
			host.CompleteHttp()
		})
	})
}
