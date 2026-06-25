package config

import (
	"fmt"
	"strings"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/cache"
	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/embedding"
	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/vector"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/tidwall/gjson"
)

const (
	CACHE_KEY_STRATEGY_LAST_QUESTION = "lastQuestion"
	CACHE_KEY_STRATEGY_ALL_QUESTIONS = "allQuestions"
	CACHE_KEY_STRATEGY_DISABLED      = "disabled"

	CACHE_MODE_OFF        = "off"
	CACHE_MODE_EXACT_ONLY = "exact-only"
	CACHE_MODE_SEMANTIC   = "semantic"
	CACHE_MODE_SHADOW     = "shadow"
	CACHE_MODE_SKIP       = "skip"
	CACHE_MODE_NO_STORE   = "no-store"

	CACHE_SCOPE_TENANT   = "tenant"
	CACHE_SCOPE_CONSUMER = "consumer"
	CACHE_SCOPE_GLOBAL   = "global"

	DEFAULT_TENANT_HEADER              = "x-mse-tenant"
	DEFAULT_CONSUMER_HEADER            = "x-mse-consumer"
	DEFAULT_CACHE_POLICY_VERSION       = "v1"
	DEFAULT_L1_CACHE_MAX_ENTRIES       = 128
	DEFAULT_L1_CACHE_TTL_SECONDS       = 30
	MAX_L1_CACHE_MAX_ENTRIES           = 1024
	MAX_L1_CACHE_TTL_SECONDS           = 60
	DEFAULT_COLLAPSE_WAIT_ATTEMPTS     = 1
	DEFAULT_COLLAPSE_WAIT_MILLISECONDS = 2500
	DEFAULT_COLLAPSE_LOCK_TTL_SECONDS  = 5
	MAX_COLLAPSE_WAIT_ATTEMPTS         = 5
	MAX_COLLAPSE_WAIT_MILLISECONDS     = 30000
	MAX_COLLAPSE_LOCK_TTL_SECONDS      = 30
)

type PluginConfig struct {
	// @Title zh-CN 返回 HTTP 响应的模版
	// @Description zh-CN 用 %s 标记需要被 cache value 替换的部分
	ResponseTemplate string
	// @Title zh-CN 返回流式 HTTP 响应的模版
	// @Description zh-CN 用 %s 标记需要被 cache value 替换的部分
	StreamResponseTemplate string

	cacheProvider     cache.Provider
	embeddingProvider embedding.Provider
	vectorProvider    vector.Provider

	embeddingProviderConfig *embedding.ProviderConfig
	vectorProviderConfig    *vector.ProviderConfig
	cacheProviderConfig     *cache.ProviderConfig

	CacheKeyFrom         string
	CacheValueFrom       string
	CacheStreamValueFrom string
	CacheToolCallsFrom   string

	// @Title zh-CN 启用语义化缓存
	// @Description zh-CN 控制是否启用语义化缓存功能。true 表示启用，false 表示禁用。
	EnableSemanticCache bool

	// @Title zh-CN 缓存键策略
	// @Description zh-CN 决定如何生成缓存键的策略。可选值: "lastQuestion" (使用最后一个问题), "allQuestions" (拼接所有问题) 或 "disabled" (禁用缓存)
	CacheKeyStrategy string

	CacheScope         string
	TenantHeader       string
	ConsumerHeader     string
	Provider           string
	CachePolicyVersion string
	CacheMode          string

	// @Title zh-CN L1 网关内存缓存
	// @Description zh-CN 可选短 TTL 本地内存缓存，仅作为 Redis 结构化缓存命中的优化层。
	L1Cache L1CacheConfig `json:"l1Cache,omitempty"`

	// @Title zh-CN 请求折叠
	// @Description zh-CN 对相同 scoped cache key 的并发 miss 进行本地 singleflight，可选 Redis 分布式锁。
	RequestCollapse RequestCollapseConfig `json:"requestCollapse,omitempty"`

	l1CacheStore         any
	requestCollapseStore any
}

type L1CacheConfig struct {
	// @Title zh-CN 启用 L1 缓存
	// @Description zh-CN 是否启用网关本地内存缓存。默认关闭。
	Enabled bool `json:"enabled,omitempty"`
	// @Title zh-CN L1 最大条目数
	// @Description zh-CN L1 内存缓存最多保存的结构化条目数。
	MaxEntries int `json:"maxEntries,omitempty"`
	// @Title zh-CN L1 TTL 秒数
	// @Description zh-CN L1 内存缓存条目的短 TTL 秒数。
	TTLSeconds int `json:"ttlSeconds,omitempty"`
}

type RequestCollapseConfig struct {
	// @Title zh-CN 启用请求折叠
	// @Description zh-CN 启用后，相同 scoped cache key 的并发 miss 会折叠到一个上游请求。
	Enabled bool `json:"enabled,omitempty"`
	// @Title zh-CN 启用 Redis 分布式锁
	// @Description zh-CN 开启后，使用 scoped lock key 进行跨网关实例请求折叠。
	DistributedLock bool `json:"distributedLock,omitempty"`
	// @Title zh-CN 允许陈旧缓存兜底
	// @Description zh-CN 等待折叠结果超限时，允许回放未硬过期的结构化 stale 条目。
	StaleFallback bool `json:"staleFallback,omitempty"`
	// @Title zh-CN 等待复查次数
	// @Description zh-CN follower 或分布式锁失败后最多复查 Redis 结构化条目的次数。
	WaitAttempts int `json:"waitAttempts,omitempty"`
	// @Title zh-CN 单次等待毫秒数
	// @Description zh-CN follower 或分布式锁失败后每次等待新缓存写入通知的最大毫秒数。
	WaitMilliseconds int `json:"waitMilliseconds,omitempty"`
	// @Title zh-CN 锁 TTL 秒数
	// @Description zh-CN Redis 分布式锁的短 TTL，避免上游请求异常时长期阻塞。
	LockTTLSeconds int `json:"lockTTLSeconds,omitempty"`
}

func (c *PluginConfig) FromJson(json gjson.Result, log log.Log) {
	c.embeddingProviderConfig = &embedding.ProviderConfig{}
	c.vectorProviderConfig = &vector.ProviderConfig{}
	c.cacheProviderConfig = &cache.ProviderConfig{}
	c.vectorProviderConfig.FromJson(json.Get("vector"))
	c.embeddingProviderConfig.FromJson(json.Get("embedding"))
	c.cacheProviderConfig.FromJson(json.Get("cache"))
	if json.Get("redis").Exists() {
		// compatible with legacy config
		c.cacheProviderConfig.ConvertLegacyJson(json)
	}

	c.CacheKeyStrategy = json.Get("cacheKeyStrategy").String()
	if c.CacheKeyStrategy == "" {
		c.CacheKeyStrategy = CACHE_KEY_STRATEGY_LAST_QUESTION // set default value
	}
	c.CacheScope = firstJSONValue(json, "cacheScope", "cache_scope")
	if c.CacheScope == "" {
		c.CacheScope = CACHE_SCOPE_TENANT
	}
	c.TenantHeader = firstJSONValue(json, "tenantHeader", "tenant_header")
	if c.TenantHeader == "" {
		c.TenantHeader = DEFAULT_TENANT_HEADER
	}
	c.ConsumerHeader = firstJSONValue(json, "consumerHeader", "consumer_header")
	if c.ConsumerHeader == "" {
		c.ConsumerHeader = DEFAULT_CONSUMER_HEADER
	}
	c.Provider = firstJSONValue(json, "provider")
	c.CachePolicyVersion = firstJSONValue(json, "cachePolicyVersion", "cache_policy_version")
	if c.CachePolicyVersion == "" {
		c.CachePolicyVersion = DEFAULT_CACHE_POLICY_VERSION
	}
	c.CacheMode = normalizeCacheMode(firstJSONValue(json, "cacheMode", "cache_mode"))
	c.l1CacheStore = nil
	c.L1Cache = L1CacheConfig{}
	if l1Config := firstJSONResult(json, "l1Cache", "l1_cache"); l1Config.Exists() {
		c.L1Cache.Enabled = firstJSONBool(l1Config, false, "enabled")
		if c.L1Cache.Enabled {
			if value := firstJSONResult(l1Config, "maxEntries", "max_entries"); value.Exists() {
				c.L1Cache.MaxEntries = int(value.Int())
			} else {
				c.L1Cache.MaxEntries = DEFAULT_L1_CACHE_MAX_ENTRIES
			}
			if value := firstJSONResult(l1Config, "ttlSeconds", "ttl_seconds"); value.Exists() {
				c.L1Cache.TTLSeconds = int(value.Int())
			} else {
				c.L1Cache.TTLSeconds = DEFAULT_L1_CACHE_TTL_SECONDS
			}
		}
	}
	c.requestCollapseStore = nil
	c.RequestCollapse = RequestCollapseConfig{}
	if collapseConfig := firstJSONResult(json, "requestCollapse", "request_collapse"); collapseConfig.Exists() {
		c.RequestCollapse.Enabled = firstJSONBool(collapseConfig, false, "enabled")
		if c.RequestCollapse.Enabled {
			c.RequestCollapse.DistributedLock = firstJSONBool(collapseConfig, false, "distributedLock", "distributed_lock")
			c.RequestCollapse.StaleFallback = firstJSONBool(collapseConfig, false, "staleFallback", "stale_fallback")
			if value := firstJSONResult(collapseConfig, "waitAttempts", "wait_attempts"); value.Exists() {
				c.RequestCollapse.WaitAttempts = int(value.Int())
			} else {
				c.RequestCollapse.WaitAttempts = DEFAULT_COLLAPSE_WAIT_ATTEMPTS
			}
			if value := firstJSONResult(collapseConfig, "waitMilliseconds", "wait_milliseconds"); value.Exists() {
				c.RequestCollapse.WaitMilliseconds = int(value.Int())
			} else {
				c.RequestCollapse.WaitMilliseconds = DEFAULT_COLLAPSE_WAIT_MILLISECONDS
			}
			if value := firstJSONResult(collapseConfig, "lockTTLSeconds", "lock_ttl_seconds"); value.Exists() {
				c.RequestCollapse.LockTTLSeconds = int(value.Int())
			} else {
				c.RequestCollapse.LockTTLSeconds = DEFAULT_COLLAPSE_LOCK_TTL_SECONDS
			}
		}
	}
	c.CacheKeyFrom = json.Get("cacheKeyFrom").String()
	if c.CacheKeyFrom == "" {
		c.CacheKeyFrom = "messages.@reverse.0.content"
	}
	c.CacheValueFrom = json.Get("cacheValueFrom").String()
	if c.CacheValueFrom == "" {
		c.CacheValueFrom = "choices.0.message.content"
	}
	c.CacheStreamValueFrom = json.Get("cacheStreamValueFrom").String()
	if c.CacheStreamValueFrom == "" {
		c.CacheStreamValueFrom = "choices.0.delta.content"
	}
	c.CacheToolCallsFrom = json.Get("cacheToolCallsFrom").String()
	if c.CacheToolCallsFrom == "" {
		c.CacheToolCallsFrom = "choices.0.delta.content.tool_calls"
	}

	c.StreamResponseTemplate = json.Get("streamResponseTemplate").String()
	if c.StreamResponseTemplate == "" {
		c.StreamResponseTemplate = `data:{"id":"from-cache","choices":[{"index":0,"delta":{"role":"assistant","content":"%s"},"finish_reason":"stop"}],"model":"from-cache","object":"chat.completion","usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}` + "\n\ndata:[DONE]\n\n"
	}
	c.ResponseTemplate = json.Get("responseTemplate").String()
	if c.ResponseTemplate == "" {
		c.ResponseTemplate = `{"id":"from-cache","choices":[{"index":0,"message":{"role":"assistant","content":"%s"},"finish_reason":"stop"}],"model":"from-cache","object":"chat.completion","usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`
	}

	if json.Get("enableSemanticCache").Exists() {
		c.EnableSemanticCache = json.Get("enableSemanticCache").Bool()
	} else if c.GetVectorProvider() == nil {
		c.EnableSemanticCache = false // set value to false when no vector provider
	} else {
		c.EnableSemanticCache = true // set default value to true
	}
	if c.CacheMode == "" {
		if c.EnableSemanticCache {
			c.CacheMode = CACHE_MODE_SEMANTIC
		} else {
			c.CacheMode = CACHE_MODE_EXACT_ONLY
		}
	}

	// compatible with legacy config
	convertLegacyMapFields(c, json, log)
}

func (c *PluginConfig) Validate() error {
	// if cache provider is configured, validate it
	if c.cacheProviderConfig.GetProviderType() != "" {
		if err := c.cacheProviderConfig.Validate(); err != nil {
			return err
		}
	}
	if c.embeddingProviderConfig.GetProviderType() != "" {
		if err := c.embeddingProviderConfig.Validate(); err != nil {
			return err
		}
	}
	if c.vectorProviderConfig.GetProviderType() != "" {
		if err := c.vectorProviderConfig.Validate(); err != nil {
			return err
		}
	}

	// cache, vector, and embedding cannot all be empty
	if c.vectorProviderConfig.GetProviderType() == "" &&
		c.embeddingProviderConfig.GetProviderType() == "" &&
		c.cacheProviderConfig.GetProviderType() == "" {
		return fmt.Errorf("vector, embedding and cache provider cannot be all empty")
	}

	// Validate the value of CacheKeyStrategy
	if c.CacheKeyStrategy != CACHE_KEY_STRATEGY_LAST_QUESTION &&
		c.CacheKeyStrategy != CACHE_KEY_STRATEGY_ALL_QUESTIONS &&
		c.CacheKeyStrategy != CACHE_KEY_STRATEGY_DISABLED {
		return fmt.Errorf("invalid CacheKeyStrategy: %s", c.CacheKeyStrategy)
	}
	if c.CacheScope != CACHE_SCOPE_TENANT &&
		c.CacheScope != CACHE_SCOPE_CONSUMER &&
		c.CacheScope != CACHE_SCOPE_GLOBAL {
		return fmt.Errorf("invalid CacheScope: %s", c.CacheScope)
	}
	if c.CacheMode != CACHE_MODE_OFF &&
		c.CacheMode != CACHE_MODE_EXACT_ONLY &&
		c.CacheMode != CACHE_MODE_SEMANTIC &&
		c.CacheMode != CACHE_MODE_SHADOW {
		return fmt.Errorf("invalid CacheMode: %s", c.CacheMode)
	}
	if c.CacheKeyStrategy != CACHE_KEY_STRATEGY_DISABLED &&
		c.cacheProviderConfig.GetProviderType() != "" &&
		c.Provider == "" {
		return fmt.Errorf("provider is required when cache provider is configured")
	}
	if c.L1Cache.Enabled {
		if c.cacheProviderConfig.GetProviderType() == "" {
			return fmt.Errorf("l1Cache requires cache provider")
		}
		if c.L1Cache.MaxEntries <= 0 {
			return fmt.Errorf("invalid l1Cache maxEntries: %d", c.L1Cache.MaxEntries)
		}
		if c.L1Cache.MaxEntries > MAX_L1_CACHE_MAX_ENTRIES {
			return fmt.Errorf("l1Cache maxEntries must be less than or equal to %d", MAX_L1_CACHE_MAX_ENTRIES)
		}
		if c.L1Cache.TTLSeconds <= 0 {
			return fmt.Errorf("invalid l1Cache ttlSeconds: %d", c.L1Cache.TTLSeconds)
		}
		if c.L1Cache.TTLSeconds > MAX_L1_CACHE_TTL_SECONDS {
			return fmt.Errorf("l1Cache ttlSeconds must be less than or equal to %d", MAX_L1_CACHE_TTL_SECONDS)
		}
	}
	if c.RequestCollapse.Enabled {
		if c.cacheProviderConfig.GetProviderType() == "" {
			return fmt.Errorf("requestCollapse requires cache provider")
		}
		if c.RequestCollapse.WaitAttempts <= 0 {
			return fmt.Errorf("invalid requestCollapse waitAttempts: %d", c.RequestCollapse.WaitAttempts)
		}
		if c.RequestCollapse.WaitAttempts > MAX_COLLAPSE_WAIT_ATTEMPTS {
			return fmt.Errorf("requestCollapse waitAttempts must be less than or equal to %d", MAX_COLLAPSE_WAIT_ATTEMPTS)
		}
		if c.RequestCollapse.WaitMilliseconds <= 0 {
			return fmt.Errorf("invalid requestCollapse waitMilliseconds: %d", c.RequestCollapse.WaitMilliseconds)
		}
		if c.RequestCollapse.WaitMilliseconds > MAX_COLLAPSE_WAIT_MILLISECONDS {
			return fmt.Errorf("requestCollapse waitMilliseconds must be less than or equal to %d", MAX_COLLAPSE_WAIT_MILLISECONDS)
		}
		if c.RequestCollapse.DistributedLock {
			if c.RequestCollapse.LockTTLSeconds <= 0 {
				return fmt.Errorf("invalid requestCollapse lockTTLSeconds: %d", c.RequestCollapse.LockTTLSeconds)
			}
			if c.RequestCollapse.LockTTLSeconds > MAX_COLLAPSE_LOCK_TTL_SECONDS {
				return fmt.Errorf("requestCollapse lockTTLSeconds must be less than or equal to %d", MAX_COLLAPSE_LOCK_TTL_SECONDS)
			}
		}
	}

	// If semantic cache is enabled, ensure necessary components are configured
	// if c.EnableSemanticCache {
	// 	if c.embeddingProviderConfig.GetProviderType() == "" {
	// 		return fmt.Errorf("semantic cache is enabled but embedding provider is not configured")
	// 	}
	// 	// if only configure cache, just warn the user
	// }
	return nil
}

func firstJSONValue(json gjson.Result, keys ...string) string {
	result := firstJSONResult(json, keys...)
	if result.Exists() {
		return result.String()
	}
	return ""
}

func firstJSONResult(json gjson.Result, keys ...string) gjson.Result {
	for _, key := range keys {
		if value := json.Get(key); value.Exists() {
			return value
		}
	}
	return gjson.Result{}
}

func firstJSONBool(json gjson.Result, defaultValue bool, keys ...string) bool {
	result := firstJSONResult(json, keys...)
	if result.Exists() {
		return result.Bool()
	}
	return defaultValue
}

func normalizeCacheMode(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func (c *PluginConfig) Complete(log log.Log) error {
	var err error
	if c.embeddingProviderConfig.GetProviderType() != "" {
		log.Debugf("embedding provider is set to %s", c.embeddingProviderConfig.GetProviderType())
		c.embeddingProvider, err = embedding.CreateProvider(*c.embeddingProviderConfig)
		if err != nil {
			return err
		}
	} else {
		log.Info("embedding provider is not configured")
		c.embeddingProvider = nil
	}
	if c.cacheProviderConfig.GetProviderType() != "" {
		log.Debugf("cache provider is set to %s", c.cacheProviderConfig.GetProviderType())
		c.cacheProvider, err = cache.CreateProvider(*c.cacheProviderConfig, log)
		if err != nil {
			return err
		}
	} else {
		log.Info("cache provider is not configured")
		c.cacheProvider = nil
	}
	if c.vectorProviderConfig.GetProviderType() != "" {
		log.Debugf("vector provider is set to %s", c.vectorProviderConfig.GetProviderType())
		c.vectorProvider, err = vector.CreateProvider(*c.vectorProviderConfig)
		if err != nil {
			return err
		}
	} else {
		log.Info("vector provider is not configured")
		c.vectorProvider = nil
	}
	return nil
}

func (c *PluginConfig) GetEmbeddingProvider() embedding.Provider {
	return c.embeddingProvider
}

func (c *PluginConfig) GetVectorProvider() vector.Provider {
	return c.vectorProvider
}

func (c *PluginConfig) GetVectorProviderConfig() vector.ProviderConfig {
	return *c.vectorProviderConfig
}

func (c *PluginConfig) GetCacheProvider() cache.Provider {
	return c.cacheProvider
}

func (c *PluginConfig) SetL1CacheStore(store any) {
	c.l1CacheStore = store
}

func (c *PluginConfig) GetL1CacheStore() any {
	return c.l1CacheStore
}

func (c *PluginConfig) SetRequestCollapseStore(store any) {
	c.requestCollapseStore = store
}

func (c *PluginConfig) GetRequestCollapseStore() any {
	return c.requestCollapseStore
}

func (c *PluginConfig) CacheHardTTLSeconds() int {
	if c.cacheProviderConfig == nil || c.cacheProviderConfig.GetProviderType() == "" {
		return 0
	}
	return c.cacheProviderConfig.GetCacheTTL()
}

func (c *PluginConfig) CacheSoftTTLSeconds() int {
	if c.cacheProviderConfig == nil || c.cacheProviderConfig.GetProviderType() == "" {
		return 0
	}
	return c.cacheProviderConfig.GetCacheSoftTTL()
}

func (c *PluginConfig) StaleWhileRevalidateEnabled() bool {
	if c.cacheProviderConfig == nil || c.cacheProviderConfig.GetProviderType() == "" {
		return false
	}
	return c.cacheProviderConfig.StaleWhileRevalidateEnabled()
}

func convertLegacyMapFields(c *PluginConfig, json gjson.Result, log log.Log) {
	keyMap := map[string]string{
		"cacheKeyFrom.requestBody":         "cacheKeyFrom",
		"cacheValueFrom.requestBody":       "cacheValueFrom",
		"cacheStreamValueFrom.requestBody": "cacheStreamValueFrom",
		"returnResponseTemplate":           "responseTemplate",
		"returnStreamResponseTemplate":     "streamResponseTemplate",
	}

	for oldKey, newKey := range keyMap {
		if json.Get(oldKey).Exists() {
			log.Debugf("[convertLegacyMapFields] mapping %s to %s", oldKey, newKey)
			setField(c, newKey, json.Get(oldKey).String(), log)
		} else {
			log.Debugf("[convertLegacyMapFields] %s not exists", oldKey)
		}
	}
}

func setField(c *PluginConfig, fieldName string, value string, log log.Log) {
	switch fieldName {
	case "cacheKeyFrom":
		c.CacheKeyFrom = value
	case "cacheValueFrom":
		c.CacheValueFrom = value
	case "cacheStreamValueFrom":
		c.CacheStreamValueFrom = value
	case "responseTemplate":
		c.ResponseTemplate = value
	case "streamResponseTemplate":
		c.StreamResponseTemplate = value
	}
	log.Debugf("[setField] set %s to %s", fieldName, value)
}
