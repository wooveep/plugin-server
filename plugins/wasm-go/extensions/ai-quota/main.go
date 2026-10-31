package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-quota/util"
)

const (
	pluginName = "ai-quota"
)

const (
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
	dormant            bool
	invalidBase        error
	redisInfo          RedisInfo `yaml:"redis"`
	namespace          balanceNamespace
	PathScope          string   `yaml:"path_scope"`
	EnablePathSuffixes []string `yaml:"enable_path_suffixes"`
	EnableMethods      []string `yaml:"enable_methods"`
	redisClient        wrapper.RedisClient
	ExecutionAdmission any
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
	*config = QuotaConfig{}
	if isRuleOnlyQuotaConfig(json) {
		config.dormant = true
		return nil
	}
	if err := parseQuotaConfigFields(json, config); err != nil {
		// The matcher continues parsing rules after a failed non-empty root.
		// Preserve that failure so its zero fallback cannot authorize a rule.
		if rules := json.Get("_rules_"); rules.IsArray() && len(rules.Array()) > 0 {
			*config = QuotaConfig{invalidBase: err}
			return nil
		}
		return err
	}
	return nil
}

func isRuleOnlyQuotaConfig(json gjson.Result) bool {
	if !json.IsObject() {
		return false
	}
	count, onlyRules := 0, true
	json.ForEach(func(key, value gjson.Result) bool {
		count++
		if key.String() != "_rules_" || !value.IsArray() {
			onlyRules = false
			return false
		}
		return true
	})
	return onlyRules && count == 1
}

func parseQuotaConfigFields(json gjson.Result, config *QuotaConfig) error {
	if err := rejectRemovedSettlementConfig(json); err != nil {
		return err
	}
	namespace, err := parseBalanceNamespace(json)
	if err != nil {
		return err
	}
	config.namespace = namespace

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

	if err := configureExecutionAdmission(json, config); err != nil {
		return err
	}
	return config.redisClient.Init(username, password, int64(timeout), wrapper.WithDataBase(database))
}

func parseRuleConfig(json gjson.Result, global QuotaConfig, config *QuotaConfig) error {
	if global.invalidBase != nil {
		return global.invalidBase
	}
	if json.Get("_rules_").Exists() {
		return errors.New("nested quota rules are not supported")
	}
	// A root containing only _rules_ skips parseConfig in the real matcher,
	// which passes a zero global config instead of the explicit dormant value.
	if global.dormant || (global.redisInfo.ServiceName == "" && global.redisClient == nil) {
		return parseConfig(json, config)
	}
	if global.redisInfo.ServiceName == "" || global.redisClient == nil {
		return errors.New("missing redis in config")
	}
	if err := rejectRemovedSettlementConfig(json); err != nil {
		return err
	}
	if err := rejectLegacyBalanceConfig(json); err != nil {
		return err
	}
	for _, field := range []string{"redis", "balance_scope_id", "cache_generation", "projection_schema", "missing_balance_policy"} {
		if json.Get(field).Exists() {
			return errors.New(field + " must be inherited from the global balance configuration")
		}
	}

	*config = global
	config.EnablePathSuffixes = cloneStrings(global.EnablePathSuffixes)
	config.EnableMethods = cloneStrings(global.EnableMethods)

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
	return configureExecutionAdmission(json, config)
}

func onHttpRequestHeaders(context wrapper.HttpContext, config QuotaConfig) types.Action {
	if config.dormant {
		context.DontReadRequestBody()
		context.DontReadResponseBody()
		return types.ActionContinue
	}
	context.DisableReroute()
	if !isQuotaRequestEnabled(context.Path(), context.Method(), config) {
		context.DontReadResponseBody()
		return types.ActionContinue
	}
	context.DontReadRequestBody()
	context.DontReadResponseBody()
	headers, err := proxywasm.GetHttpRequestHeaders()
	if err != nil {
		return deniedMissingIdentity()
	}
	identity, err := balanceidentity.Read(headers)
	if err != nil {
		return deniedMissingIdentity()
	}
	routeID := ""
	if admission, ok := config.ExecutionAdmission.(*executionAdmissionConfig); ok && admission != nil {
		routeID = admission.RouteID
	}
	err = readBalanceAdmission(config.redisClient, config.namespace, identity, routeID, func(result balanceAdmission, err error) {
		if err != nil {
			context.SetUserAttribute("balance_admission_reason", "dependency_unavailable")
			log.Warn("ai-quota versioned balance read failed")
			util.SendResponse(http.StatusServiceUnavailable, "ai-quota.dependency_unavailable", "text/plain", "Model request is temporarily unavailable.")
			return
		}
		context.SetUserAttribute("balance_admission_reason", result.ReasonCode)
		context.SetUserAttribute("balance_account_version", result.ObservedAccountVersion)
		context.SetUserAttribute("balance_cache_generation", result.ObservedGeneration)
		if result.Decision == "allow" {
			checkExecutionAdmission(context, config, identity, func() { proxywasm.ResumeHttpRequest() })
			return
		}
		if result.ReasonCode == "insufficient_balance" {
			deniedNoBalance()
			return
		}
		status := uint32(http.StatusForbidden)
		message := "Model request is not allowed for this account."
		switch result.ReasonCode {
		case "projection_unready", "wrong_generation", "projection_schema_mismatch", "dependency_unavailable", "invalid_amount":
			status = http.StatusServiceUnavailable
			message = "Model request is temporarily unavailable."
		}
		util.SendResponse(status, "ai-quota."+result.ReasonCode, "text/plain", message)
	})
	if err != nil {
		context.SetUserAttribute("balance_admission_reason", "dependency_unavailable")
		util.SendResponse(http.StatusServiceUnavailable, "ai-quota.dependency_unavailable", "text/plain", "Model request is temporarily unavailable.")
		return types.ActionContinue
	}
	return types.HeaderStopAllIterationAndWatermark
}

func deniedMissingIdentity() types.Action {
	util.SendResponse(http.StatusForbidden, "ai-quota.missing_identity", "text/plain", "Request denied by ai quota check. Missing authenticated wallet identity.")
	return types.ActionContinue
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

func rejectRemovedSettlementConfig(json gjson.Result) error {
	for _, field := range removedSettlementConfigFields {
		if json.Get(field).Exists() {
			return fmt.Errorf("%s has been removed; ai-quota is admission-only", field)
		}
	}
	return nil
}
