package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/resp"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-quota/util"
)

const (
	pluginName = "ai-quota"

	defaultQuotaScope         = "global"
	defaultProvider           = "default"
	defaultTenantHeader       = "x-mse-tenant"
	defaultConsumerHeader     = "x-mse-consumer"
	defaultBalanceKeyTemplate = "billing:balance:{tenant}:{quota_scope}:{consumer}"
)

const (
	MissingPolicyDeny  = "deny"
	MissingPolicyAllow = "allow"

	PathScopeLegacySuffixes = "legacy_suffixes"
	PathScopeBoundTarget    = "bound_target"
)

var removedSettlementConfigFields = []string{
	"price_key_template",
	"amount_scale",
	"price_unit_tokens",
	"missing_price_policy",
	"missing_usage_policy",
}

func main() {}

func init() {
	wrapper.SetCtx(
		pluginName,
		wrapper.ParseOverrideConfig(parseConfig, parseRuleConfig),
		wrapper.ProcessRequestHeaders(onHttpRequestHeaders),
	)
}

type QuotaConfig struct {
	redisInfo            RedisInfo `yaml:"redis"`
	QuotaScope           string    `yaml:"quota_scope"`
	Provider             string    `yaml:"provider"`
	TenantHeader         string    `yaml:"tenant_header"`
	ConsumerHeader       string    `yaml:"consumer_header"`
	BalanceKeyTemplate   string    `yaml:"balance_key_template"`
	PathScope            string    `yaml:"path_scope"`
	EnablePathSuffixes   []string  `yaml:"enable_path_suffixes"`
	EnableMethods        []string  `yaml:"enable_methods"`
	MissingBalancePolicy string    `yaml:"missing_balance_policy"`
	redisClient          wrapper.RedisClient
}

type RedisInfo struct {
	ServiceName string `required:"true" yaml:"service_name" json:"service_name"`
	ServicePort int    `required:"false" yaml:"service_port" json:"service_port"`
	Username    string `required:"false" yaml:"username" json:"username"`
	Password    string `required:"false" yaml:"password" json:"password"`
	Timeout     int    `required:"false" yaml:"timeout" json:"timeout"`
	Database    int    `required:"false" yaml:"database" json:"database"`
}

func parseConfig(json gjson.Result, config *QuotaConfig) error {
	if err := rejectRemovedSettlementConfig(json); err != nil {
		return err
	}
	config.QuotaScope = stringDefault(json.Get("quota_scope").String(), defaultQuotaScope)
	config.Provider = stringDefault(json.Get("provider").String(), defaultProvider)
	config.TenantHeader = stringDefault(json.Get("tenant_header").String(), defaultTenantHeader)
	config.ConsumerHeader = stringDefault(json.Get("consumer_header").String(), defaultConsumerHeader)
	config.BalanceKeyTemplate = stringDefault(json.Get("balance_key_template").String(), defaultBalanceKeyTemplate)
	config.MissingBalancePolicy = stringDefault(json.Get("missing_balance_policy").String(), MissingPolicyDeny)

	if err := validatePolicy(config.MissingBalancePolicy, MissingPolicyDeny, MissingPolicyAllow); err != nil {
		return fmt.Errorf("invalid missing_balance_policy: %w", err)
	}

	pathScope, err := parsePathScope(json.Get("path_scope"))
	if err != nil {
		return err
	}
	config.PathScope = pathScope
	suffixes, err := parsePathSuffixesForScope(json.Get("enable_path_suffixes"), pathScope)
	if err != nil {
		return err
	}
	config.EnablePathSuffixes = suffixes
	methods, err := parseHTTPMethods(json.Get("enable_methods"))
	if err != nil {
		return err
	}
	config.EnableMethods = methods

	redisConfig := json.Get("redis")
	if !redisConfig.Exists() {
		return errors.New("missing redis in config")
	}
	serviceName := redisConfig.Get("service_name").String()
	if serviceName == "" {
		return errors.New("redis service name must not be empty")
	}
	servicePort := int(redisConfig.Get("service_port").Int())
	if servicePort == 0 {
		if strings.HasSuffix(serviceName, ".static") {
			servicePort = 80
		} else {
			servicePort = 6379
		}
	}
	username := redisConfig.Get("username").String()
	password := redisConfig.Get("password").String()
	timeout := int(redisConfig.Get("timeout").Int())
	if timeout == 0 {
		timeout = 1000
	}
	database := int(redisConfig.Get("database").Int())
	config.redisInfo = RedisInfo{
		ServiceName: serviceName,
		ServicePort: servicePort,
		Username:    username,
		Password:    password,
		Timeout:     timeout,
		Database:    database,
	}
	config.redisClient = wrapper.NewRedisClusterClient(wrapper.FQDNCluster{
		FQDN: serviceName,
		Port: int64(servicePort),
	})

	return config.redisClient.Init(username, password, int64(timeout), wrapper.WithDataBase(database))
}

func parseRuleConfig(json gjson.Result, global QuotaConfig, config *QuotaConfig) error {
	if global.redisInfo.ServiceName == "" || global.redisClient == nil {
		return errors.New("missing redis in config")
	}
	if err := rejectRemovedSettlementConfig(json); err != nil {
		return err
	}

	*config = global
	config.EnablePathSuffixes = cloneStrings(global.EnablePathSuffixes)
	config.EnableMethods = cloneStrings(global.EnableMethods)

	if value := json.Get("quota_scope"); value.Exists() {
		config.QuotaScope = stringDefault(value.String(), defaultQuotaScope)
	}
	if value := json.Get("provider"); value.Exists() {
		config.Provider = stringDefault(value.String(), defaultProvider)
	}
	if value := json.Get("path_scope"); value.Exists() {
		pathScope, err := parsePathScope(value)
		if err != nil {
			return err
		}
		if pathScope != normalizedPathScope(global.PathScope) {
			config.EnablePathSuffixes = nil
			config.EnableMethods = nil
			if pathScope == PathScopeLegacySuffixes {
				config.EnablePathSuffixes = defaultPathSuffixes()
			}
		}
		config.PathScope = pathScope
	}
	if value := json.Get("enable_path_suffixes"); value.Exists() {
		suffixes, err := parsePathSuffixesForScope(
			value,
			normalizedPathScope(config.PathScope),
		)
		if err != nil {
			return err
		}
		config.EnablePathSuffixes = suffixes
	}
	if value := json.Get("enable_methods"); value.Exists() {
		methods, err := parseHTTPMethods(value)
		if err != nil {
			return err
		}
		config.EnableMethods = methods
	}
	return nil
}

func onHttpRequestHeaders(context wrapper.HttpContext, config QuotaConfig) types.Action {
	context.DisableReroute()

	rawPath := context.Path()
	if !isQuotaRequestEnabled(rawPath, context.Method(), config) {
		context.DontReadResponseBody()
		return types.ActionContinue
	}
	context.DontReadRequestBody()
	context.DontReadResponseBody()

	tenant, _ := proxywasm.GetHttpRequestHeader(config.TenantHeader)
	consumer, _ := proxywasm.GetHttpRequestHeader(config.ConsumerHeader)
	if tenant == "" || consumer == "" {
		return deniedMissingIdentity()
	}

	balanceKey := config.buildBalanceKey(tenant, consumer)
	log.Debugf("ai-quota admission tenant:%s consumer:%s balance_key:%s", tenant, consumer, balanceKey)

	err := config.redisClient.Get(balanceKey, func(response resp.Value) {
		if err := response.Error(); err != nil {
			log.Errorf("ai-quota redis balance read failed (service=%s timeout_ms=%d): %v", config.redisInfo.ServiceName, config.redisInfo.Timeout, err)
			util.SendResponse(http.StatusServiceUnavailable, "ai-quota.redis_error", "text/plain", "Request denied by ai quota check. Redis balance read failed.")
			return
		}
		if response.IsNull() {
			if config.MissingBalancePolicy == MissingPolicyAllow {
				log.Warnf("ai-quota balance missing, allowing by policy, key:%s", balanceKey)
				proxywasm.ResumeHttpRequest()
				return
			}
			deniedMissingBalance()
			return
		}
		balance, err := redisInteger(response)
		if err != nil {
			log.Errorf("ai-quota invalid balance value for key %s: %v", balanceKey, err)
			util.SendResponse(http.StatusServiceUnavailable, "ai-quota.redis_error", "text/plain", "Request denied by ai quota check. Invalid Redis balance.")
			return
		}
		if balance <= 0 {
			deniedNoBalance()
			return
		}
		proxywasm.ResumeHttpRequest()
	})
	if err != nil {
		log.Errorf("ai-quota redis balance dispatch failed (service=%s timeout_ms=%d): %v", config.redisInfo.ServiceName, config.redisInfo.Timeout, err)
		util.SendResponse(http.StatusServiceUnavailable, "ai-quota.redis_error", "text/plain", "Request denied by ai quota check. Redis balance read failed.")
		return types.ActionContinue
	}
	return types.HeaderStopAllIterationAndWatermark
}

func deniedMissingIdentity() types.Action {
	util.SendResponse(http.StatusForbidden, "ai-quota.missing_identity", "text/plain", "Request denied by ai quota check. Missing tenant or consumer identity.")
	return types.ActionContinue
}

func deniedMissingBalance() {
	util.SendResponse(http.StatusForbidden, "ai-quota.missing_balance", "text/plain", "Request denied by ai quota check. Monetary balance is missing.")
}

func deniedNoBalance() {
	util.SendResponse(http.StatusForbidden, "ai-quota.no_balance", "text/plain", "Request denied by ai quota check. No monetary balance left.")
}

func isAIPathEnabled(requestPath string, enabledSuffixes []string) bool {
	pathWithoutQuery := requestPath
	if parsed, err := url.Parse(requestPath); err == nil && parsed.Path != "" {
		pathWithoutQuery = parsed.Path
	} else if queryPos := strings.Index(requestPath, "?"); queryPos != -1 {
		pathWithoutQuery = requestPath[:queryPos]
	}
	for _, suffix := range enabledSuffixes {
		if strings.TrimSpace(suffix) == "" {
			continue
		}
		if strings.HasSuffix(pathWithoutQuery, suffix) {
			return true
		}
	}
	return false
}

func isQuotaRequestEnabled(requestPath string, requestMethod string, config QuotaConfig) bool {
	pathScope := normalizedPathScope(config.PathScope)
	if pathScope == PathScopeLegacySuffixes &&
		!isAIPathEnabled(requestPath, config.EnablePathSuffixes) {
		return false
	}
	if pathScope == PathScopeBoundTarget &&
		len(config.EnablePathSuffixes) > 0 &&
		!isAIPathEnabled(requestPath, config.EnablePathSuffixes) {
		return false
	}
	if len(config.EnableMethods) > 0 &&
		!matchesHTTPMethod(config.EnableMethods, requestMethod) {
		return false
	}
	return pathScope == PathScopeLegacySuffixes || pathScope == PathScopeBoundTarget
}

func (config QuotaConfig) buildBalanceKey(tenant, consumer string) string {
	replacer := strings.NewReplacer(
		"{tenant}", tenant,
		"{quota_scope}", config.QuotaScope,
		"{consumer}", consumer,
	)
	return replacer.Replace(config.BalanceKeyTemplate)
}

func parsePathSuffixes(result gjson.Result) ([]string, error) {
	return parsePathSuffixesForScope(result, PathScopeLegacySuffixes)
}

func parsePathSuffixesForScope(result gjson.Result, pathScope string) ([]string, error) {
	if !result.Exists() {
		if pathScope == PathScopeBoundTarget {
			return nil, nil
		}
		return defaultPathSuffixes(), nil
	}
	suffixes, err := parseRoutingStringList(result, false)
	if err != nil {
		return nil, errors.New("enable_path_suffixes must be an array of strings")
	}
	if pathScope == PathScopeLegacySuffixes && len(suffixes) == 0 {
		return nil, errors.New("enable_path_suffixes must not be empty")
	}
	return suffixes, nil
}

func parsePathScope(result gjson.Result) (string, error) {
	if !result.Exists() {
		return PathScopeLegacySuffixes, nil
	}
	if result.Type != gjson.String {
		return "", errors.New("path_scope must be legacy_suffixes or bound_target")
	}
	pathScope := strings.TrimSpace(result.String())
	if pathScope != PathScopeLegacySuffixes && pathScope != PathScopeBoundTarget {
		return "", errors.New("path_scope must be legacy_suffixes or bound_target")
	}
	return pathScope, nil
}

func normalizedPathScope(pathScope string) string {
	if pathScope == "" {
		return PathScopeLegacySuffixes
	}
	return pathScope
}

func parseHTTPMethods(result gjson.Result) ([]string, error) {
	if !result.Exists() {
		return nil, nil
	}
	methods, err := parseRoutingStringList(result, true)
	if err != nil {
		return nil, errors.New("enable_methods must be an array of HTTP method strings")
	}
	return methods, nil
}

func parseRoutingStringList(result gjson.Result, validateMethod bool) ([]string, error) {
	if !result.IsArray() {
		return nil, errors.New("routing filter must be an array")
	}
	values := result.Array()
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value.Type != gjson.String {
			return nil, errors.New("routing filter entries must be strings")
		}
		entry := strings.TrimSpace(value.String())
		if entry == "" {
			continue
		}
		if validateMethod && !isHTTPToken(entry) {
			return nil, errors.New("routing filter entry is not an HTTP token")
		}
		if _, exists := seen[entry]; exists {
			continue
		}
		seen[entry] = struct{}{}
		normalized = append(normalized, entry)
	}
	if len(values) > 0 && len(normalized) == 0 {
		return nil, errors.New("routing filter must contain a usable entry")
	}
	return normalized, nil
}

func isHTTPToken(value string) bool {
	if value == "" {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character >= '0' && character <= '9' ||
			character >= 'A' && character <= 'Z' ||
			character >= 'a' && character <= 'z' {
			continue
		}
		switch character {
		case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
			continue
		default:
			return false
		}
	}
	return true
}

func matchesHTTPMethod(methods []string, requestMethod string) bool {
	for _, method := range methods {
		if method == requestMethod {
			return true
		}
	}
	return false
}

func defaultPathSuffixes() []string {
	return []string{"/v1/chat/completions", "/v1/messages"}
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string(nil), values...)
}

func validatePolicy(policy string, allowed ...string) error {
	for _, value := range allowed {
		if policy == value {
			return nil
		}
	}
	return fmt.Errorf("must be one of %s", strings.Join(allowed, ", "))
}

func redisInteger(response resp.Value) (int64, error) {
	if response.Type() == resp.Integer {
		return int64(response.Integer()), nil
	}
	return strconv.ParseInt(strings.TrimSpace(response.String()), 10, 64)
}

func rejectRemovedSettlementConfig(json gjson.Result) error {
	for _, field := range removedSettlementConfigFields {
		if json.Get(field).Exists() {
			return fmt.Errorf("%s has been removed; ai-quota is admission-only", field)
		}
	}
	return nil
}

func stringDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
