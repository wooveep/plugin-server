package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const (
	CACHE_SCOPE_TENANT   = "tenant"
	CACHE_SCOPE_CONSUMER = "consumer"

	MEMORY_CACHE_MODE_POLICY_DIGEST = "policy_digest"
	MEMORY_CACHE_MODE_BYPASS        = "bypass"

	FAIL_POLICY_OPEN = "open"
)

var legacyOnlineCacheConfigKeys = []string{
	"cache",
	"redis",
	"embedding",
	"vector",
	"cacheKeyStrategy",
	"enableSemanticCache",
	"cacheKeyFrom",
	"cacheKeyFrom.requestBody",
	"cacheValueFrom",
	"cacheValueFrom.requestBody",
	"cacheStreamValueFrom",
	"cacheStreamValueFrom.requestBody",
	"cacheToolCallsFrom",
	"responseTemplate",
	"streamResponseTemplate",
	"returnResponseTemplate",
	"returnStreamResponseTemplate",
}

type MaterializedLookupConfig struct {
	Redis RedisEndpointConfig
}

type RedisEndpointConfig struct {
	Enabled     bool
	ServiceName string
	ServicePort int
	Username    string
	Password    string
	Database    int
	KeyPrefix   string
	Timeout     int
}

type ConsoleLookupConfig struct {
	Enabled     bool
	ServiceName string
	ServicePort int
	Path        string
	BearerToken string
	Timeout     int
}

type EventConfig struct {
	RedisStream RedisStreamConfig
}

type RedisStreamConfig struct {
	Enabled     bool
	ServiceName string
	ServicePort int
	Username    string
	Password    string
	Database    int
	Stream      string
	Field       string
	Timeout     int
}

type RoutePolicyConfig struct {
	EnableRedisLookup   bool
	EnableConsoleLookup bool
	EnableReplay        bool
	EnableBypass        bool
	EnabledPathSuffixes []string
	Memory              MemoryPolicyConfig
}

type MemoryPolicyConfig struct {
	Enabled       bool
	CacheMode     string
	PolicyVersion string
}

// NativeIdentityConfig is the five static facts frozen by the original route publication.
// Actual caller identity is captured from native authentication per request, never configured.
type NativeIdentityConfig struct {
	ReleaseID     string `json:"release_id"`
	TargetID      string `json:"target_id"`
	RouteID       string `json:"route_id"`
	PolicyID      string `json:"policy_id"`
	PolicyVersion int64  `json:"policy_version"`
}

type PluginConfig struct {
	materializedRedis wrapper.RedisClient
	eventRedis        wrapper.RedisClient
	legacyConfig      bool
	protocolConfigErr error
	nativeIdentityErr error

	NativeIdentity *NativeIdentityConfig
	// NativeExecution is a private parsed delivery config; never body-supplied.
	NativeExecution any

	PublishedProtocolConfigured bool
	PublishedProtocol           protocol.ProtocolKind
	ProtocolBypassReason        string

	MaterializedLookup MaterializedLookupConfig
	ConsoleLookup      ConsoleLookupConfig
	Event              EventConfig
	RoutePolicy        RoutePolicyConfig
	TenantHeader       string
	ConsumerHeader     string
	SessionHeader      string
	CacheScope         string
	CachePolicyVersion string
	FailPolicy         string
}

func (c *PluginConfig) FromJson(json gjson.Result, _ log.Log) {
	*c = PluginConfig{}
	c.legacyConfig = containsLegacyOnlineCacheConfig(json)
	c.parsePublishedProtocol(json)
	c.parseNativeIdentity(json)
	c.MaterializedLookup.Redis = RedisEndpointConfig{
		Enabled:     json.Get("materialized_lookup.redis.enabled").Bool(),
		ServiceName: json.Get("materialized_lookup.redis.service_name").String(),
		ServicePort: int(json.Get("materialized_lookup.redis.service_port").Int()),
		Username:    json.Get("materialized_lookup.redis.username").String(),
		Password:    json.Get("materialized_lookup.redis.password").String(),
		Database:    int(json.Get("materialized_lookup.redis.database").Int()),
		KeyPrefix:   json.Get("materialized_lookup.redis.key_prefix").String(),
		Timeout:     int(json.Get("materialized_lookup.redis.timeout").Int()),
	}
	c.ConsoleLookup = ConsoleLookupConfig{
		Enabled:     json.Get("console_lookup.enabled").Bool(),
		ServiceName: json.Get("console_lookup.service_name").String(),
		ServicePort: int(json.Get("console_lookup.service_port").Int()),
		Path:        json.Get("console_lookup.path").String(),
		BearerToken: json.Get("console_lookup.bearer_token").String(),
		Timeout:     int(json.Get("console_lookup.timeout").Int()),
	}
	if c.ConsoleLookup.Path == "" {
		c.ConsoleLookup.Path = "/internal/cache/lookup"
	}
	c.Event.RedisStream = RedisStreamConfig{
		Enabled:     json.Get("redis_stream.enabled").Bool(),
		ServiceName: json.Get("redis_stream.service_name").String(),
		ServicePort: int(json.Get("redis_stream.service_port").Int()),
		Username:    json.Get("redis_stream.username").String(),
		Password:    json.Get("redis_stream.password").String(),
		Database:    int(json.Get("redis_stream.database").Int()),
		Stream:      json.Get("redis_stream.stream").String(),
		Field:       json.Get("redis_stream.field").String(),
		Timeout:     int(json.Get("redis_stream.timeout").Int()),
	}
	if c.Event.RedisStream.Field == "" {
		c.Event.RedisStream.Field = "event"
	}
	if c.Event.RedisStream.Stream == "" {
		c.Event.RedisStream.Stream = "cache:events"
	}
	c.RoutePolicy = RoutePolicyConfig{
		EnableRedisLookup:   true,
		EnableConsoleLookup: c.ConsoleLookup.Enabled,
		EnableReplay:        true,
		EnableBypass:        false,
		EnabledPathSuffixes: jsonStringArray(json.Get("route_policy.enabled_path_suffixes")),
		Memory:              parseMemoryPolicy(json.Get("route_policy.memory")),
	}
	if json.Get("route_policy.enable_redis_lookup").Exists() {
		c.RoutePolicy.EnableRedisLookup = json.Get("route_policy.enable_redis_lookup").Bool()
	}
	if json.Get("route_policy.enable_console_lookup").Exists() {
		c.RoutePolicy.EnableConsoleLookup = json.Get("route_policy.enable_console_lookup").Bool()
	}
	if json.Get("route_policy.enable_replay").Exists() {
		c.RoutePolicy.EnableReplay = json.Get("route_policy.enable_replay").Bool()
	}
	if json.Get("route_policy.enable_bypass").Exists() {
		c.RoutePolicy.EnableBypass = json.Get("route_policy.enable_bypass").Bool()
	}
	c.TenantHeader = json.Get("tenant_header").String()
	if c.TenantHeader == "" {
		c.TenantHeader = "x-mse-tenant"
	}
	c.ConsumerHeader = json.Get("consumer_header").String()
	if c.ConsumerHeader == "" {
		c.ConsumerHeader = "x-mse-consumer"
	}
	c.SessionHeader = json.Get("session_header").String()
	if c.SessionHeader == "" {
		c.SessionHeader = "x-openclaw-session-key"
	}
	c.CacheScope = json.Get("cache_scope").String()
	if c.CacheScope == "" {
		c.CacheScope = CACHE_SCOPE_TENANT
	}
	c.CachePolicyVersion = json.Get("cache_policy_version").String()
	c.FailPolicy = json.Get("fail_policy").String()
	if c.FailPolicy == "" {
		c.FailPolicy = FAIL_POLICY_OPEN
	}
}

func (c *PluginConfig) FromJsonWithGlobal(json gjson.Result, global PluginConfig, log log.Log) {
	if !global.HasThinConfig() && global.CachePolicyVersion == "" && !global.PublishedProtocolConfigured && global.NativeIdentity == nil {
		c.FromJson(json, log)
		return
	}
	*c = global
	c.legacyConfig = c.legacyConfig || containsLegacyOnlineCacheConfig(json)
	c.parsePublishedProtocol(json)
	c.parseNativeIdentity(json)
	consoleLookupEnabledOverride := json.Get("console_lookup.enabled").Exists()
	routePolicyConsoleOverride := json.Get("route_policy.enable_console_lookup").Exists()
	c.MaterializedLookup.Redis = mergeRedisEndpoint(c.MaterializedLookup.Redis, json.Get("materialized_lookup.redis"))
	c.ConsoleLookup = mergeConsoleLookup(c.ConsoleLookup, json.Get("console_lookup"))
	c.Event.RedisStream = mergeRedisStream(c.Event.RedisStream, json.Get("redis_stream"))
	c.RoutePolicy = mergeRoutePolicy(c.RoutePolicy, json.Get("route_policy"))
	if consoleLookupEnabledOverride && !routePolicyConsoleOverride {
		c.RoutePolicy.EnableConsoleLookup = c.ConsoleLookup.Enabled
	}
	if json.Get("tenant_header").Exists() && json.Get("tenant_header").String() != "" {
		c.TenantHeader = json.Get("tenant_header").String()
	}
	if json.Get("consumer_header").Exists() && json.Get("consumer_header").String() != "" {
		c.ConsumerHeader = json.Get("consumer_header").String()
	}
	if json.Get("session_header").Exists() && json.Get("session_header").String() != "" {
		c.SessionHeader = json.Get("session_header").String()
	}
	if json.Get("cache_scope").Exists() {
		c.CacheScope = json.Get("cache_scope").String()
	}
	if json.Get("cache_policy_version").Exists() {
		c.CachePolicyVersion = json.Get("cache_policy_version").String()
	}
	if json.Get("fail_policy").Exists() {
		c.FailPolicy = json.Get("fail_policy").String()
	}
}

func (c *PluginConfig) Validate() error {
	if c.nativeIdentityErr != nil {
		return c.nativeIdentityErr
	}
	if c.protocolConfigErr != nil {
		return c.protocolConfigErr
	}
	if c.legacyConfig {
		return fmt.Errorf("legacy online cache configuration is not supported")
	}
	if !c.HasThinConfig() {
		return fmt.Errorf("thin cache behavior is required")
	}
	if c.CacheScope != CACHE_SCOPE_TENANT && c.CacheScope != CACHE_SCOPE_CONSUMER {
		return fmt.Errorf("invalid cache_scope: %s", c.CacheScope)
	}
	if c.FailPolicy != FAIL_POLICY_OPEN {
		return fmt.Errorf("invalid fail_policy: %s", c.FailPolicy)
	}
	if c.CachePolicyVersion == "" {
		return fmt.Errorf("cache_policy_version is required when thin cache behavior is enabled")
	}
	if c.NativeIdentity != nil && c.CachePolicyVersion != strconv.FormatInt(c.NativeIdentity.PolicyVersion, 10) {
		return fmt.Errorf("cache_policy_version must equal the canonical native_identity.policy_version")
	}
	if c.MaterializedLookup.Redis.Enabled {
		if err := validateRedisEndpoint(c.MaterializedLookup.Redis); err != nil {
			return err
		}
	}
	if c.ConsoleLookup.Enabled {
		if err := validateConsoleLookup(c.ConsoleLookup); err != nil {
			return err
		}
	}
	if c.Event.RedisStream.Enabled {
		if err := validateRedisStream(c.Event.RedisStream); err != nil {
			return err
		}
	}
	if c.RoutePolicy.Memory.Enabled {
		switch c.RoutePolicy.Memory.CacheMode {
		case MEMORY_CACHE_MODE_POLICY_DIGEST:
			if c.RoutePolicy.Memory.PolicyVersion == "" {
				return fmt.Errorf("route_policy.memory.policy_version is required when memory policy digest mode is enabled")
			}
		case MEMORY_CACHE_MODE_BYPASS:
		default:
			return fmt.Errorf("invalid route_policy.memory.cache_mode: %s", c.RoutePolicy.Memory.CacheMode)
		}
	}
	return nil
}

func (c *PluginConfig) parseNativeIdentity(input gjson.Result) {
	value := input.Get("native_identity")
	if !value.Exists() {
		return
	}
	c.NativeIdentity = nil
	c.nativeIdentityErr = nil
	var identity NativeIdentityConfig
	decoder := json.NewDecoder(strings.NewReader(value.Raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&identity); err != nil {
		c.nativeIdentityErr = fmt.Errorf("native_identity requires only its five published identity fields: %w", err)
		return
	}
	if identity.ReleaseID == "" || len(identity.ReleaseID) > 256 ||
		identity.TargetID == "" || len(identity.TargetID) > 256 ||
		!balanceidentity.ValidID(identity.RouteID) || !balanceidentity.ValidID(identity.PolicyID) || identity.PolicyVersion <= 0 {
		c.nativeIdentityErr = fmt.Errorf("native_identity requires bounded original release/target IDs, canonical non-nil route/policy UUIDs, and a positive integer policy_version")
		return
	}
	c.NativeIdentity = &identity
}

func (c *PluginConfig) parsePublishedProtocol(json gjson.Result) {
	clientProtocol := json.Get("client_protocol")
	operationKind := json.Get("operation_kind")
	if !clientProtocol.Exists() && !operationKind.Exists() {
		return
	}
	c.PublishedProtocolConfigured = true
	c.PublishedProtocol = protocol.ProtocolUnknown
	c.ProtocolBypassReason = ""
	c.protocolConfigErr = nil
	if clientProtocol.Type != gjson.String || operationKind.Type != gjson.String ||
		clientProtocol.String() == "" || operationKind.String() == "" {
		c.protocolConfigErr = fmt.Errorf("client_protocol and operation_kind must be supplied together as non-empty strings")
		return
	}
	kind, err := protocol.ParsePublishedTextProtocol(clientProtocol.String(), operationKind.String())
	if err != nil {
		c.ProtocolBypassReason = err.Error()
		return
	}
	c.PublishedProtocol = kind
}

// ProtocolForPath uses the route's published client protocol. Path detection is
// retained only for configurations that omit both published fields.
func (c PluginConfig) ProtocolForPath(path string) protocol.ProtocolKind {
	if c.PublishedProtocolConfigured {
		return c.PublishedProtocol
	}
	return protocol.DetectProtocolKind(path)
}

func (c *PluginConfig) HasThinConfig() bool {
	return c.MaterializedLookup.Redis.Enabled ||
		c.ConsoleLookup.Enabled ||
		c.Event.RedisStream.Enabled
}

func (c *PluginConfig) Complete(_ log.Log) error {
	if c.MaterializedLookup.Redis.Enabled && c.Event.RedisStream.Enabled &&
		sameRedisEndpoint(c.MaterializedLookup.Redis, c.Event.RedisStream) &&
		(c.MaterializedLookup.Redis.Username != c.Event.RedisStream.Username ||
			c.MaterializedLookup.Redis.Password != c.Event.RedisStream.Password ||
			c.MaterializedLookup.Redis.Database != c.Event.RedisStream.Database) {
		return fmt.Errorf("shared cache Redis endpoint requires matching credentials and database")
	}
	if c.MaterializedLookup.Redis.Enabled && c.Event.RedisStream.Enabled &&
		sameRedisEndpoint(c.MaterializedLookup.Redis, c.Event.RedisStream) {
		client := wrapper.NewRedisClusterClient(wrapper.FQDNCluster{
			FQDN: c.Event.RedisStream.ServiceName,
			Port: int64(c.Event.RedisStream.ServicePort),
		})
		timeout := c.Event.RedisStream.Timeout
		if c.MaterializedLookup.Redis.Timeout > 0 && (timeout <= 0 || c.MaterializedLookup.Redis.Timeout < timeout) {
			timeout = c.MaterializedLookup.Redis.Timeout
		}
		if err := client.Init(c.Event.RedisStream.Username, c.Event.RedisStream.Password, int64(timeout),
			wrapper.WithDisableBuffer(), wrapper.WithDataBase(c.Event.RedisStream.Database)); err != nil {
			return err
		}
		c.materializedRedis = client
		c.eventRedis = client
		return nil
	}
	if c.MaterializedLookup.Redis.Enabled {
		c.materializedRedis = wrapper.NewRedisClusterClient(wrapper.FQDNCluster{
			FQDN: c.MaterializedLookup.Redis.ServiceName,
			Port: int64(c.MaterializedLookup.Redis.ServicePort),
		})
		if err := c.materializedRedis.Init(c.MaterializedLookup.Redis.Username, c.MaterializedLookup.Redis.Password,
			int64(c.MaterializedLookup.Redis.Timeout), wrapper.WithDataBase(c.MaterializedLookup.Redis.Database)); err != nil {
			return err
		}
	} else {
		c.materializedRedis = nil
	}
	if c.Event.RedisStream.Enabled {
		c.eventRedis = wrapper.NewRedisClusterClient(wrapper.FQDNCluster{
			FQDN: c.Event.RedisStream.ServiceName,
			Port: int64(c.Event.RedisStream.ServicePort),
		})
		if err := c.eventRedis.Init(c.Event.RedisStream.Username, c.Event.RedisStream.Password,
			int64(c.Event.RedisStream.Timeout), wrapper.WithDisableBuffer(), wrapper.WithDataBase(c.Event.RedisStream.Database)); err != nil {
			return err
		}
	} else {
		c.eventRedis = nil
	}
	return nil
}

func sameRedisEndpoint(materialized RedisEndpointConfig, event RedisStreamConfig) bool {
	return materialized.ServiceName == event.ServiceName &&
		materialized.ServicePort == event.ServicePort
}

func (c *PluginConfig) GetMaterializedRedisClient() wrapper.RedisClient {
	return c.materializedRedis
}

func (c *PluginConfig) GetEventRedisClient() wrapper.RedisClient {
	return c.eventRedis
}

func containsLegacyOnlineCacheConfig(json gjson.Result) bool {
	for _, key := range legacyOnlineCacheConfigKeys {
		if json.Get(key).Exists() {
			return true
		}
	}
	return false
}

func jsonStringArray(value gjson.Result) []string {
	if !value.Exists() {
		return nil
	}
	items := value.Array()
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item.String() == "" {
			continue
		}
		out = append(out, item.String())
	}
	return out
}

func parseMemoryPolicy(value gjson.Result) MemoryPolicyConfig {
	if !value.Exists() {
		return MemoryPolicyConfig{}
	}
	cfg := MemoryPolicyConfig{
		Enabled:       value.Get("enabled").Bool(),
		CacheMode:     value.Get("cache_mode").String(),
		PolicyVersion: value.Get("policy_version").String(),
	}
	if cfg.CacheMode == "" {
		cfg.CacheMode = MEMORY_CACHE_MODE_POLICY_DIGEST
	}
	return cfg
}

func mergeRedisEndpoint(base RedisEndpointConfig, value gjson.Result) RedisEndpointConfig {
	if !value.Exists() {
		return base
	}
	if value.Get("enabled").Exists() {
		base.Enabled = value.Get("enabled").Bool()
	}
	if value.Get("service_name").Exists() {
		base.ServiceName = value.Get("service_name").String()
	}
	if value.Get("service_port").Exists() {
		base.ServicePort = int(value.Get("service_port").Int())
	}
	if value.Get("username").Exists() {
		base.Username = value.Get("username").String()
	}
	if value.Get("password").Exists() {
		base.Password = value.Get("password").String()
	}
	if value.Get("database").Exists() {
		base.Database = int(value.Get("database").Int())
	}
	if value.Get("key_prefix").Exists() {
		base.KeyPrefix = value.Get("key_prefix").String()
	}
	if value.Get("timeout").Exists() {
		base.Timeout = int(value.Get("timeout").Int())
	}
	return base
}

func mergeConsoleLookup(base ConsoleLookupConfig, value gjson.Result) ConsoleLookupConfig {
	if !value.Exists() {
		return base
	}
	if value.Get("enabled").Exists() {
		base.Enabled = value.Get("enabled").Bool()
	}
	if value.Get("service_name").Exists() {
		base.ServiceName = value.Get("service_name").String()
	}
	if value.Get("service_port").Exists() {
		base.ServicePort = int(value.Get("service_port").Int())
	}
	if value.Get("path").Exists() {
		base.Path = value.Get("path").String()
	}
	if value.Get("bearer_token").Exists() {
		base.BearerToken = value.Get("bearer_token").String()
	}
	if value.Get("timeout").Exists() {
		base.Timeout = int(value.Get("timeout").Int())
	}
	return base
}

func mergeRedisStream(base RedisStreamConfig, value gjson.Result) RedisStreamConfig {
	if !value.Exists() {
		return base
	}
	if value.Get("enabled").Exists() {
		base.Enabled = value.Get("enabled").Bool()
	}
	if value.Get("service_name").Exists() {
		base.ServiceName = value.Get("service_name").String()
	}
	if value.Get("service_port").Exists() {
		base.ServicePort = int(value.Get("service_port").Int())
	}
	if value.Get("username").Exists() {
		base.Username = value.Get("username").String()
	}
	if value.Get("password").Exists() {
		base.Password = value.Get("password").String()
	}
	if value.Get("database").Exists() {
		base.Database = int(value.Get("database").Int())
	}
	if value.Get("stream").Exists() {
		base.Stream = value.Get("stream").String()
	}
	if value.Get("field").Exists() {
		base.Field = value.Get("field").String()
	}
	if value.Get("timeout").Exists() {
		base.Timeout = int(value.Get("timeout").Int())
	}
	return base
}

func mergeRoutePolicy(base RoutePolicyConfig, value gjson.Result) RoutePolicyConfig {
	if !value.Exists() {
		return base
	}
	if value.Get("enable_redis_lookup").Exists() {
		base.EnableRedisLookup = value.Get("enable_redis_lookup").Bool()
	}
	if value.Get("enable_console_lookup").Exists() {
		base.EnableConsoleLookup = value.Get("enable_console_lookup").Bool()
	}
	if value.Get("enable_replay").Exists() {
		base.EnableReplay = value.Get("enable_replay").Bool()
	}
	if value.Get("enable_bypass").Exists() {
		base.EnableBypass = value.Get("enable_bypass").Bool()
	}
	if value.Get("enabled_path_suffixes").Exists() {
		base.EnabledPathSuffixes = jsonStringArray(value.Get("enabled_path_suffixes"))
	}
	base.Memory = mergeMemoryPolicy(base.Memory, value.Get("memory"))
	return base
}

func mergeMemoryPolicy(base MemoryPolicyConfig, value gjson.Result) MemoryPolicyConfig {
	if !value.Exists() {
		return base
	}
	if value.Get("enabled").Exists() {
		base.Enabled = value.Get("enabled").Bool()
	}
	if value.Get("cache_mode").Exists() {
		base.CacheMode = value.Get("cache_mode").String()
	}
	if value.Get("policy_version").Exists() {
		base.PolicyVersion = value.Get("policy_version").String()
	}
	if base.CacheMode == "" {
		base.CacheMode = MEMORY_CACHE_MODE_POLICY_DIGEST
	}
	return base
}

func validateRedisEndpoint(cfg RedisEndpointConfig) error {
	if cfg.ServiceName == "" {
		return fmt.Errorf("materialized_lookup.redis.service_name is required when enabled")
	}
	if cfg.ServicePort <= 0 {
		return fmt.Errorf("materialized_lookup.redis.service_port must be positive when enabled")
	}
	if cfg.KeyPrefix == "" {
		return fmt.Errorf("materialized_lookup.redis.key_prefix is required when enabled")
	}
	if cfg.Timeout <= 0 {
		return fmt.Errorf("materialized_lookup.redis.timeout must be positive when enabled")
	}
	return nil
}

func validateConsoleLookup(cfg ConsoleLookupConfig) error {
	if cfg.ServiceName == "" {
		return fmt.Errorf("console_lookup.service_name is required when enabled")
	}
	if cfg.ServicePort <= 0 {
		return fmt.Errorf("console_lookup.service_port must be positive when enabled")
	}
	if cfg.Path == "" {
		return fmt.Errorf("console_lookup.path is required when enabled")
	}
	if cfg.Timeout <= 0 {
		return fmt.Errorf("console_lookup.timeout must be positive when enabled")
	}
	return nil
}

func validateRedisStream(cfg RedisStreamConfig) error {
	if cfg.ServiceName == "" {
		return fmt.Errorf("redis_stream.service_name is required when enabled")
	}
	if cfg.ServicePort <= 0 {
		return fmt.Errorf("redis_stream.service_port must be positive when enabled")
	}
	if cfg.Stream == "" {
		return fmt.Errorf("redis_stream.stream is required when enabled")
	}
	if cfg.Field == "" {
		return fmt.Errorf("redis_stream.field is required when enabled")
	}
	if cfg.Timeout <= 0 {
		return fmt.Errorf("redis_stream.timeout must be positive when enabled")
	}
	return nil
}
