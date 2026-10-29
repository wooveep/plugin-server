package config

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

type PathScope string

const (
	PathScopeLegacy      PathScope = "legacy"
	PathScopeBoundTarget PathScope = "bound_target"

	MemoryModeOff        = "off"
	MemoryModeRecentOnly = "recent-only"
	MemoryModeDigest     = "digest"
	MemoryModeSemantic   = "semantic"
	MemoryModeFull       = "full"

	FailPolicyOpen = "open"

	InjectRoleSystem    = "system"
	InjectRoleDeveloper = "developer"
)

type RedisStreamConfig struct {
	ServiceName string `yaml:"service_name" json:"service_name"`
	ServicePort int    `yaml:"service_port" json:"service_port"`
	Username    string `yaml:"username" json:"username"`
	Password    string `yaml:"password" json:"password"`
	Database    int    `yaml:"database" json:"database"`
	Timeout     int    `yaml:"timeout" json:"timeout"`
	Stream      string `yaml:"stream" json:"stream"`
}

type RecentCacheConfig struct {
	ServiceName string `yaml:"service_name" json:"service_name"`
	ServicePort int    `yaml:"service_port" json:"service_port"`
	Username    string `yaml:"username" json:"username"`
	Password    string `yaml:"password" json:"password"`
	Database    int    `yaml:"database" json:"database"`
	Timeout     int    `yaml:"timeout" json:"timeout"`
	KeyPrefix   string `yaml:"key_prefix" json:"key_prefix"`
}

type ConsoleInternalConfig struct {
	ServiceName  string `yaml:"service_name" json:"service_name"`
	ServicePort  int    `yaml:"service_port" json:"service_port"`
	AssemblePath string `yaml:"assemble_path" json:"assemble_path"`
	TimeoutMS    int    `yaml:"timeout_ms" json:"timeout_ms"`
	AuthToken    string `yaml:"auth_token" json:"auth_token"`
}

type RouteConfig struct {
	MemoryMode          string   `yaml:"memory_mode" json:"memory_mode"`
	RecentWindowTurns   int      `yaml:"recent_window_turns" json:"recent_window_turns"`
	MemoryTokenBudget   int      `yaml:"memory_token_budget" json:"memory_token_budget"`
	AssembleTimeoutMS   int      `yaml:"assemble_timeout_ms" json:"assemble_timeout_ms"`
	InjectRole          string   `yaml:"inject_role" json:"inject_role"`
	DeveloperCompatible bool     `yaml:"developer_compatible" json:"developer_compatible"`
	SemanticTopK        int      `yaml:"semantic_top_k" json:"semantic_top_k"`
	CaptureResponse     bool     `yaml:"capture_response" json:"capture_response"`
	NoStoreHeader       string   `yaml:"no_store_header" json:"no_store_header"`
	QuestionFrom        string   `yaml:"question_from" json:"question_from"`
	ResponseValueFrom   string   `yaml:"response_value_from" json:"response_value_from"`
	StreamValueFrom     string   `yaml:"stream_value_from" json:"stream_value_from"`
	ToolCallsFrom       []string `yaml:"tool_calls_from" json:"tool_calls_from"`
	PolicyVersion       string   `yaml:"policy_version" json:"policy_version"`
}

// PluginConfig is the ai-memory plugin configuration root.
type PluginConfig struct {
	recentRedis wrapper.RedisClient
	eventRedis  EventRedisClient

	RedisStream        RedisStreamConfig     `yaml:"redis_stream" json:"redis_stream"`
	RecentCache        RecentCacheConfig     `yaml:"recent_cache" json:"recent_cache"`
	ConsoleInternal    ConsoleInternalConfig `yaml:"console_internal" json:"console_internal"`
	TenantHeader       string                `yaml:"tenant_header" json:"tenant_header"`
	ConsumerHeader     string                `yaml:"consumer_header" json:"consumer_header"`
	SessionHeader      string                `yaml:"session_header" json:"session_header"`
	RequestIDHeader    string                `yaml:"request_id_header" json:"request_id_header"`
	EnablePathSuffixes []string              `yaml:"enable_path_suffixes" json:"enable_path_suffixes"`
	PathScope          PathScope             `yaml:"path_scope" json:"path_scope"`
	CaptureBodyBasis   bool                  `yaml:"capture_body_basis" json:"capture_body_basis"`
	NativeIdentity     *NativeIdentityConfig `yaml:"native_identity" json:"native_identity,omitempty"`
	FailPolicy         string                `yaml:"fail_policy" json:"fail_policy"`
	Route              RouteConfig           `yaml:"route" json:"route"`

	unsupportedRouteTarget string
	publishedProtocol      bool
	protocolKind           protocol.ProtocolKind
	requestScopeError      error
	initialized            bool
	baseError              error
}

// ParseRoot loads a real global configuration or leaves a rules-only carrier dormant.
func (c *PluginConfig) ParseRoot(json gjson.Result, log log.Log) error {
	*c = defaultPluginConfig()
	rulesOnly := true
	for key := range json.Map() {
		if key != "_rules_" {
			rulesOnly = false
			break
		}
	}
	if rulesOnly {
		return nil
	}
	c.FromJson(json, log)
	err := c.Validate()
	if err == nil {
		err = c.Complete(log)
	}
	if err != nil && json.Get("_rules_").IsArray() && len(json.Get("_rules_").Array()) > 0 {
		// The matcher discards a failed base. Retain its error so it cannot be
		// mistaken for the zero base reserved for complete standalone rules.
		c.baseError = err
		return nil
	}
	return err
}

func (c *PluginConfig) FromJson(json gjson.Result, log log.Log) {
	*c = defaultPluginConfig()
	c.initialized = true
	c.RedisStream = parseRedisStreamConfig(c.RedisStream, json.Get("redis_stream"))
	c.RecentCache = parseRecentCacheConfig(c.RecentCache, json.Get("recent_cache"))
	c.ConsoleInternal = parseConsoleInternalConfig(c.ConsoleInternal, json.Get("console_internal"))
	c.TenantHeader = stringValue(json.Get("tenant_header"), c.TenantHeader)
	c.ConsumerHeader = stringValue(json.Get("consumer_header"), c.ConsumerHeader)
	c.SessionHeader = stringValue(json.Get("session_header"), c.SessionHeader)
	c.RequestIDHeader = stringValue(json.Get("request_id_header"), c.RequestIDHeader)
	c.EnablePathSuffixes = stringSliceValue(json.Get("enable_path_suffixes"), c.EnablePathSuffixes)
	c.FailPolicy = stringValue(json.Get("fail_policy"), c.FailPolicy)
	c.parseRequestScope(json)
}

func (c *PluginConfig) FromJsonWithGlobal(json gjson.Result, global PluginConfig, log log.Log) {
	if global.baseError != nil {
		*c = global
		return
	}
	if !global.initialized {
		// The controller's disabled defaultConfig contains only _rules_. Each
		// matched managed rule therefore owns its complete targets and behavior.
		c.FromJson(json, log)
		c.Route = parseRouteConfig(c.Route, json)
		if c.requestScopeError == nil && c.PathScope != PathScopeBoundTarget {
			c.requestScopeError = fmt.Errorf("standalone memory rules require bound_target")
		}
		return
	}
	*c = global
	c.unsupportedRouteTarget = ""
	for _, key := range []string{"redis_stream", "recent_cache", "console_internal"} {
		if json.Get(key).Exists() {
			c.unsupportedRouteTarget = key
			break
		}
	}
	c.Route = parseRouteConfig(c.Route, json)
	c.requestScopeError = nil
	c.parseRequestScope(json)
}

func (c PluginConfig) Validate() error {
	if c.baseError != nil {
		return c.baseError
	}
	if c.requestScopeError != nil {
		return c.requestScopeError
	}
	if c.unsupportedRouteTarget != "" {
		return fmt.Errorf("route-level %s is not supported", c.unsupportedRouteTarget)
	}
	if c.CaptureBodyBasis && (c.PathScope != PathScopeBoundTarget || !c.publishedProtocol || c.protocolKind == protocol.ProtocolUnknown) {
		return fmt.Errorf("capture_body_basis requires a supported published text protocol on bound_target")
	}
	if c.FailPolicy != FailPolicyOpen {
		return fmt.Errorf("unsupported fail_policy %q", c.FailPolicy)
	}
	if c.RedisStream.ServiceName == "" {
		return fmt.Errorf("redis_stream.service_name is required")
	}
	if sharedRedisEndpoint(c.RedisStream, c.RecentCache) &&
		(c.RedisStream.Username != c.RecentCache.Username ||
			c.RedisStream.Password != c.RecentCache.Password ||
			c.RedisStream.Database != c.RecentCache.Database) {
		return fmt.Errorf("redis_stream and recent_cache sharing an endpoint must use the same credentials and database")
	}
	if !validMemoryMode(c.Route.MemoryMode) {
		return fmt.Errorf("unsupported memory_mode %q", c.Route.MemoryMode)
	}
	if c.Route.InjectRole != InjectRoleSystem && c.Route.InjectRole != InjectRoleDeveloper {
		return fmt.Errorf("unsupported inject_role %q", c.Route.InjectRole)
	}
	if c.Route.InjectRole == InjectRoleDeveloper && !c.Route.DeveloperCompatible {
		return fmt.Errorf("inject_role %q requires developer_compatible true", c.Route.InjectRole)
	}
	if err := validatePolicyVersion(c.Route.PolicyVersion); err != nil {
		return err
	}
	return nil
}

// ProtocolForPath selects the published protocol, using path detection only for legacy configuration.
func (c PluginConfig) ProtocolForPath(path string) protocol.ProtocolKind {
	if c.publishedProtocol {
		return c.protocolKind
	}
	return protocol.DetectProtocolKind(path)
}

func (c *PluginConfig) parseRequestScope(json gjson.Result) {
	if identity := json.Get("native_identity"); identity.Exists() {
		var err error
		c.NativeIdentity, err = parseNativeIdentity(identity)
		if err != nil {
			c.requestScopeError = err
			return
		}
	}
	if capture := json.Get("capture_body_basis"); capture.Exists() {
		if capture.Type != gjson.True && capture.Type != gjson.False {
			c.requestScopeError = fmt.Errorf("capture_body_basis must be a boolean")
			return
		}
		c.CaptureBodyBasis = capture.Bool()
	}
	if scope := json.Get("path_scope"); scope.Exists() {
		value := PathScope(scope.String())
		if scope.Type != gjson.String || (value != PathScopeLegacy && value != PathScopeBoundTarget) {
			c.requestScopeError = fmt.Errorf("path_scope must be legacy or bound_target")
			return
		}
		c.PathScope = value
	}
	clientProtocol, operation := json.Get("client_protocol"), json.Get("operation_kind")
	if clientProtocol.Exists() || operation.Exists() {
		if clientProtocol.Type != gjson.String || operation.Type != gjson.String || clientProtocol.String() == "" || operation.String() == "" {
			c.requestScopeError = fmt.Errorf("client_protocol and operation_kind must be declared together as nonempty strings")
			return
		}
		c.publishedProtocol = true
		// A complete unsupported binding stays unsupported; it never falls back to path guessing.
		c.protocolKind, _ = protocol.ParsePublishedTextProtocol(clientProtocol.String(), operation.String())
	}
	if c.PathScope == PathScopeBoundTarget {
		if !c.publishedProtocol {
			c.requestScopeError = fmt.Errorf("bound_target requires published client_protocol and operation_kind")
			return
		}
		for _, field := range []string{"enable_path_suffixes", "enable_methods"} {
			if filter := json.Get(field); filter.Exists() && (!filter.IsArray() || len(filter.Array()) != 0) {
				c.requestScopeError = fmt.Errorf("bound_target does not permit an additional %s filter", field)
				return
			}
		}
		// The matched route owns coverage; legacy global suffixes cannot narrow it.
		c.EnablePathSuffixes = nil
	}
	if c.NativeIdentity != nil {
		if c.PathScope != PathScopeBoundTarget {
			c.requestScopeError = fmt.Errorf("native_identity requires bound_target")
			return
		}
		version := strconv.FormatInt(c.NativeIdentity.PolicyVersion, 10)
		if declared := json.Get("policy_version"); declared.Exists() && (declared.Type != gjson.String || declared.Str != version) {
			c.requestScopeError = fmt.Errorf("policy_version must match native_identity.policy_version as a canonical decimal string")
			return
		}
		c.Route.PolicyVersion = version
	}
}

func sharedRedisEndpoint(stream RedisStreamConfig, recent RecentCacheConfig) bool {
	return strings.TrimSpace(recent.ServiceName) != "" &&
		strings.TrimSpace(stream.ServiceName) == strings.TrimSpace(recent.ServiceName) &&
		stream.ServicePort == recent.ServicePort
}

func sharedRedisOperationTimeout(streamTimeout, recentTimeout int) int {
	if streamTimeout <= 0 {
		return recentTimeout
	}
	if recentTimeout <= 0 || streamTimeout < recentTimeout {
		return streamTimeout
	}
	return recentTimeout
}

func (c *PluginConfig) Complete(log log.Log) error {
	recentTimeout := c.RecentCache.Timeout
	eventConfig := c.RedisStream
	if sharedRedisEndpoint(c.RedisStream, c.RecentCache) {
		effectiveTimeout := sharedRedisOperationTimeout(c.RedisStream.Timeout, c.RecentCache.Timeout)
		recentTimeout = effectiveTimeout
		eventConfig.Timeout = effectiveTimeout
	}
	if c.recentRedis == nil && c.RecentCache.ServiceName != "" {
		client := wrapper.NewRedisClusterClient(wrapper.FQDNCluster{
			FQDN: c.RecentCache.ServiceName,
			Port: int64(c.RecentCache.ServicePort),
		})
		if err := client.Init(
			c.RecentCache.Username,
			c.RecentCache.Password,
			int64(recentTimeout),
			wrapper.WithDataBase(c.RecentCache.Database),
			wrapper.WithDisableBuffer(),
		); err != nil {
			return err
		}
		c.recentRedis = client
	}
	if c.eventRedis == nil {
		client := newEventRedisClient(eventConfig)
		if err := client.Init(); err != nil {
			return err
		}
		c.eventRedis = client
	}
	return nil
}

func (c PluginConfig) GetRecentRedisClient() wrapper.RedisClient {
	return c.recentRedis
}

func (c PluginConfig) GetEventRedisClient() EventRedisClient {
	return c.eventRedis
}

func defaultPluginConfig() PluginConfig {
	return PluginConfig{
		RedisStream: RedisStreamConfig{
			ServicePort: 6379,
			Timeout:     500,
			Stream:      "memory:events",
		},
		RecentCache: RecentCacheConfig{
			ServicePort: 6379,
			Timeout:     50,
			KeyPrefix:   "memory:recent",
		},
		ConsoleInternal: ConsoleInternalConfig{
			ServicePort:  80,
			AssemblePath: "/internal/memory/assemble",
			TimeoutMS:    100,
		},
		TenantHeader:       "x-mse-tenant",
		ConsumerHeader:     "x-mse-consumer",
		SessionHeader:      "x-mse-session",
		RequestIDHeader:    "x-request-id",
		EnablePathSuffixes: []string{"/v1/chat/completions", "/v1/messages", "/v1/responses"},
		PathScope:          PathScopeLegacy,
		FailPolicy:         FailPolicyOpen,
		Route:              defaultRouteConfig(),
	}
}

func defaultRouteConfig() RouteConfig {
	return RouteConfig{
		MemoryMode:        MemoryModeOff,
		InjectRole:        InjectRoleSystem,
		CaptureResponse:   true,
		NoStoreHeader:     "x-higress-ai-memory-no-store",
		QuestionFrom:      `messages.@reverse.#(role=="user").content`,
		ResponseValueFrom: "choices.0.message.content",
		StreamValueFrom:   "choices.0.delta.content",
		ToolCallsFrom: []string{
			"choices.0.message.tool_calls",
			"choices.0.message.function_call",
			"choices.0.delta.tool_calls",
			"choices.0.delta.function_call",
		},
	}
}

func parseRedisStreamConfig(base RedisStreamConfig, json gjson.Result) RedisStreamConfig {
	if !json.Exists() {
		return base
	}
	base.ServiceName = stringValue(json.Get("service_name"), base.ServiceName)
	base.ServicePort = intValue(json.Get("service_port"), base.ServicePort)
	base.Username = stringValue(json.Get("username"), base.Username)
	base.Password = stringValue(json.Get("password"), base.Password)
	base.Database = intValue(json.Get("database"), base.Database)
	base.Timeout = intValue(json.Get("timeout"), base.Timeout)
	base.Stream = stringValue(json.Get("stream"), base.Stream)
	return base
}

func parseRecentCacheConfig(base RecentCacheConfig, json gjson.Result) RecentCacheConfig {
	if !json.Exists() {
		return base
	}
	base.ServiceName = stringValue(json.Get("service_name"), base.ServiceName)
	base.ServicePort = intValue(json.Get("service_port"), base.ServicePort)
	base.Username = stringValue(json.Get("username"), base.Username)
	base.Password = stringValue(json.Get("password"), base.Password)
	base.Database = intValue(json.Get("database"), base.Database)
	base.Timeout = intValue(json.Get("timeout"), base.Timeout)
	base.KeyPrefix = stringValue(json.Get("key_prefix"), base.KeyPrefix)
	return base
}

func parseConsoleInternalConfig(base ConsoleInternalConfig, json gjson.Result) ConsoleInternalConfig {
	if !json.Exists() {
		return base
	}
	base.ServiceName = stringValue(json.Get("service_name"), base.ServiceName)
	base.ServicePort = intValue(json.Get("service_port"), base.ServicePort)
	base.AssemblePath = stringValue(json.Get("assemble_path"), base.AssemblePath)
	base.TimeoutMS = intValue(json.Get("timeout_ms"), base.TimeoutMS)
	base.AuthToken = stringValue(json.Get("auth_token"), base.AuthToken)
	return base
}

func parseRouteConfig(base RouteConfig, json gjson.Result) RouteConfig {
	base.MemoryMode = stringValue(json.Get("memory_mode"), base.MemoryMode)
	base.RecentWindowTurns = intValue(json.Get("recent_window_turns"), base.RecentWindowTurns)
	base.MemoryTokenBudget = intValue(json.Get("memory_token_budget"), base.MemoryTokenBudget)
	base.AssembleTimeoutMS = intValue(json.Get("assemble_timeout_ms"), base.AssembleTimeoutMS)
	base.InjectRole = stringValue(json.Get("inject_role"), base.InjectRole)
	if json.Get("developer_compatible").Exists() {
		base.DeveloperCompatible = json.Get("developer_compatible").Bool()
	}
	base.SemanticTopK = intValue(json.Get("semantic_top_k"), base.SemanticTopK)
	if json.Get("capture_response").Exists() {
		base.CaptureResponse = json.Get("capture_response").Bool()
	}
	base.NoStoreHeader = stringValue(json.Get("no_store_header"), base.NoStoreHeader)
	base.QuestionFrom = stringValue(json.Get("question_from"), base.QuestionFrom)
	base.ResponseValueFrom = stringValue(json.Get("response_value_from"), base.ResponseValueFrom)
	base.StreamValueFrom = stringValue(json.Get("stream_value_from"), base.StreamValueFrom)
	base.ToolCallsFrom = stringSliceValue(json.Get("tool_calls_from"), base.ToolCallsFrom)
	base.PolicyVersion = stringValue(json.Get("policy_version"), base.PolicyVersion)
	return base
}

func stringValue(value gjson.Result, fallback string) string {
	if !value.Exists() {
		return fallback
	}
	return value.String()
}

func intValue(value gjson.Result, fallback int) int {
	if !value.Exists() {
		return fallback
	}
	return int(value.Int())
}

func stringSliceValue(value gjson.Result, fallback []string) []string {
	if !value.Exists() {
		return fallback
	}
	values := value.Array()
	out := make([]string, 0, len(values))
	for _, item := range values {
		if item.String() == "" {
			continue
		}
		out = append(out, item.String())
	}
	return out
}

func validMemoryMode(value string) bool {
	switch value {
	case MemoryModeOff, MemoryModeRecentOnly, MemoryModeDigest, MemoryModeSemantic, MemoryModeFull:
		return true
	default:
		return false
	}
}

func validatePolicyVersion(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	policyVersion, err := strconv.ParseInt(value, 10, 64)
	if err != nil || policyVersion <= 0 {
		return fmt.Errorf("policy_version must be a positive integer string")
	}
	return nil
}
