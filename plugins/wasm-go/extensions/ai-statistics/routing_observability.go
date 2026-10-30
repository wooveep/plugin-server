package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const (
	pathScopeLegacySuffixes = "legacy_suffixes"
	pathScopeBoundTarget    = "bound_target"

	routeUnknown          = "route_unknown"
	pathUnknown           = "path_unknown"
	providerNativeUnknown = "provider_native_unknown"
	methodUnknown         = "method_unknown"

	adapterMetadataProperty = "mse_ai_proxy_adapter_metadata"
	maxAttributionBytes     = 128
	maxAdapterMetadataBytes = 1024

	statisticsMethodContext         = "ai-statistics-method"
	statisticsRouteContext          = "ai-statistics-safe-route"
	statisticsPathContext           = "ai-statistics-path"
	statisticsStatusContext         = "ai-statistics-status"
	statisticsErrorClassContext     = "ai-statistics-error-class"
	statisticsBaseMetricsWritten    = "ai-statistics-base-metrics-written"
	statisticsRequestCountMetric    = "request_count"
	statisticsRequestDurationMetric = "request_duration_ms"
)

type statisticsAdapterMetadata struct {
	SchemaVersion       int    `json:"schema_version"`
	Matched             bool   `json:"matched"`
	AdapterID           string `json:"adapter_id"`
	Version             string `json:"version"`
	NormalizedOperation string `json:"normalized_operation"`
	OperationTemplate   string `json:"operation_template"`
	UsageProfile        string `json:"usage_profile"`
	ProviderMatch       string `json:"provider_match"`
}

func parseRuleConfig(configJSON gjson.Result, global AIStatisticsConfig, config *AIStatisticsConfig) error {
	if global.invalidBase {
		return errors.New("invalid non-empty base statistics config")
	}
	if configJSON.Get("_rules_").Exists() {
		return errors.New("nested statistics rules are not supported")
	}
	if !global.initialized {
		return parseCompleteStatisticsConfig(configJSON, config)
	}
	for _, key := range []string{"use_default_attributes", "use_default_response_attributes", "attributes", "disable_openai_usage", "value_length_limit", "enable_content_types", "session_id_header"} {
		if configJSON.Get(key).Exists() {
			return fmt.Errorf("%s must be configured globally when the base statistics config is initialized", key)
		}
	}
	*config = global
	config.enablePathSuffixes = cloneStatisticsStrings(global.enablePathSuffixes)
	config.enableMethods = cloneStatisticsStrings(global.enableMethods)
	config.legacyPathSuffixes = cloneStatisticsStrings(global.legacyPathSuffixes)

	if value := configJSON.Get("path_scope"); value.Exists() {
		pathScope, err := parseStatisticsPathScope(value)
		if err != nil {
			return err
		}
		if pathScope != normalizedStatisticsPathScope(global.pathScope) {
			config.enablePathSuffixes = nil
			config.enableMethods = nil
			if pathScope == pathScopeLegacySuffixes {
				config.enablePathSuffixes = cloneStatisticsStrings(config.legacyPathSuffixes)
			}
		}
		config.pathScope = pathScope
	}
	if config.pathScope == pathScopeBoundTarget {
		for _, key := range []string{"enable_path_suffixes", "enable_methods"} {
			if value := configJSON.Get(key); value.Exists() && (!value.IsArray() || len(value.Array()) != 0) {
				return fmt.Errorf("%s must be omitted or [] for bound_target", key)
			}
		}
		config.enablePathSuffixes = nil
		config.enableMethods = nil
		return nil
	}
	if value := configJSON.Get("enable_path_suffixes"); value.Exists() {
		suffixes, err := parseStatisticsPathSuffixes(value)
		if err != nil {
			return err
		}
		config.enablePathSuffixes = suffixes
	}
	if value := configJSON.Get("enable_methods"); value.Exists() {
		methods, err := parseStatisticsHTTPMethods(value)
		if err != nil {
			return err
		}
		config.enableMethods = methods
	}
	return nil
}

func parseStatisticsRouting(
	configJSON gjson.Result,
	config *AIStatisticsConfig,
	useLegacyDefaultSuffixes bool,
) error {
	pathScope, err := parseStatisticsPathScope(configJSON.Get("path_scope"))
	if err != nil {
		return err
	}
	config.pathScope = pathScope

	legacyDefaults := []string(nil)
	if useLegacyDefaultSuffixes {
		legacyDefaults = []string{"/completions", "/messages"}
	}
	config.legacyPathSuffixes = cloneStatisticsStrings(legacyDefaults)

	if pathScope == pathScopeBoundTarget {
		for _, key := range []string{"enable_path_suffixes", "enable_methods"} {
			if value := configJSON.Get(key); value.Exists() && (!value.IsArray() || len(value.Array()) != 0) {
				return fmt.Errorf("%s must be omitted or [] for bound_target", key)
			}
		}
		config.enablePathSuffixes = nil
		config.enableMethods = nil
		return nil
	}

	suffixResult := configJSON.Get("enable_path_suffixes")
	if suffixResult.Exists() {
		suffixes, err := parseStatisticsPathSuffixes(suffixResult)
		if err != nil {
			return err
		}
		config.enablePathSuffixes = suffixes
		if pathScope == pathScopeLegacySuffixes {
			config.legacyPathSuffixes = cloneStatisticsStrings(suffixes)
		}
	} else if pathScope == pathScopeLegacySuffixes {
		config.enablePathSuffixes = cloneStatisticsStrings(legacyDefaults)
	} else {
		config.enablePathSuffixes = nil
	}

	methods, err := parseStatisticsHTTPMethods(configJSON.Get("enable_methods"))
	if err != nil {
		return err
	}
	config.enableMethods = methods
	return nil
}

func parseStatisticsPathScope(result gjson.Result) (string, error) {
	if !result.Exists() {
		return pathScopeLegacySuffixes, nil
	}
	if result.Type != gjson.String {
		return "", errors.New("path_scope must be legacy_suffixes or bound_target")
	}
	pathScope := strings.TrimSpace(result.String())
	if pathScope != pathScopeLegacySuffixes && pathScope != pathScopeBoundTarget {
		return "", errors.New("path_scope must be legacy_suffixes or bound_target")
	}
	return pathScope, nil
}

func normalizedStatisticsPathScope(pathScope string) string {
	if pathScope == "" {
		return pathScopeLegacySuffixes
	}
	return pathScope
}

func parseStatisticsPathSuffixes(result gjson.Result) ([]string, error) {
	values, err := parseStatisticsRoutingStringList(result, false)
	if err != nil {
		return nil, errors.New("enable_path_suffixes must be an array of strings")
	}
	for _, value := range values {
		if value == "*" {
			return nil, nil
		}
	}
	return values, nil
}

func parseStatisticsHTTPMethods(result gjson.Result) ([]string, error) {
	if !result.Exists() {
		return nil, nil
	}
	values, err := parseStatisticsRoutingStringList(result, true)
	if err != nil {
		return nil, errors.New("enable_methods must be an array of HTTP method strings")
	}
	return values, nil
}

func parseStatisticsRoutingStringList(
	result gjson.Result,
	validateMethod bool,
) ([]string, error) {
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
		if validateMethod && !isStatisticsHTTPToken(entry) {
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

func isStatisticsHTTPToken(value string) bool {
	if value == "" {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') {
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

func isStatisticsRequestEnabled(requestPath, requestMethod string, config AIStatisticsConfig) bool {
	pathScope := normalizedStatisticsPathScope(config.pathScope)
	switch pathScope {
	case pathScopeLegacySuffixes:
		if !isPathEnabled(requestPath, config.enablePathSuffixes) {
			return false
		}
	case pathScopeBoundTarget:
		return true
	default:
		return false
	}
	if len(config.enableMethods) > 0 &&
		!matchesStatisticsHTTPMethod(config.enableMethods, requestMethod) {
		return false
	}
	return true
}

func matchesStatisticsHTTPMethod(methods []string, requestMethod string) bool {
	for _, method := range methods {
		if method == requestMethod {
			return true
		}
	}
	return false
}

func cloneStatisticsStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string(nil), values...)
}

func normalizeTrustedRouteName(value string) string {
	if value == "" || len(value) > maxAttributionBytes || !utf8.ValidString(value) {
		return routeUnknown
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return routeUnknown
		}
	}
	return value
}

func statisticsPathAttribution() (string, *statisticsAdapterMetadata) {
	raw, err := proxywasm.GetProperty([]string{adapterMetadataProperty})
	if err != nil || len(raw) == 0 || len(raw) > maxAdapterMetadataBytes {
		return providerNativeUnknown, nil
	}
	var metadata statisticsAdapterMetadata
	if err := json.Unmarshal(raw, &metadata); err != nil ||
		!metadata.Matched {
		return providerNativeUnknown, nil
	}
	if !isValidNormalizedOperation(metadata.NormalizedOperation) {
		return providerNativeUnknown, nil
	}
	switch metadata.SchemaVersion {
	case 1:
		metadata.AdapterID = ""
		metadata.Version = ""
		metadata.UsageProfile = ""
		metadata.ProviderMatch = ""
		return providerNativeUnknown, &metadata
	case 2:
		if metadata.OperationTemplate == "" {
			return providerNativeUnknown, nil
		}
		if !isValidOperationTemplate(metadata.OperationTemplate) {
			return pathUnknown, &metadata
		}
		return metadata.OperationTemplate, &metadata
	default:
		return providerNativeUnknown, nil
	}
}

func isValidNormalizedOperation(value string) bool {
	if len(value) == 0 ||
		len(value) > maxAttributionBytes ||
		value[0] == '/' {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') {
			continue
		}
		switch character {
		case '/', '.', '_', '-', '~':
			continue
		default:
			return false
		}
	}
	return true
}

func isValidOperationTemplate(value string) bool {
	if len(value) == 0 ||
		len(value) > maxAttributionBytes ||
		value[0] != '/' ||
		(len(value) > 1 && value[1] == '/') {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') {
			continue
		}
		switch character {
		case '/', '.', '_', '-', '~', '{', '}':
			continue
		default:
			return false
		}
	}
	return true
}

func setStatisticsRequestBaseFacts(ctx wrapper.HttpContext) {
	rawRoute, err := proxywasm.GetProperty([]string{"route_name"})
	route := routeUnknown
	if err == nil {
		route = normalizeTrustedRouteName(string(rawRoute))
	}
	path, metadata := statisticsPathAttribution()
	method, _ := proxywasm.GetHttpRequestHeader(":method")
	method = normalizeStatisticsMethod(method)

	ctx.SetContext(statisticsRouteContext, route)
	ctx.SetContext(statisticsPathContext, path)
	ctx.SetContext(statisticsMethodContext, method)
	ctx.SetUserAttribute("route", route)
	ctx.SetUserAttribute("path", path)
	ctx.SetUserAttribute("method", method)

	if metadata == nil {
		return
	}
	ctx.SetUserAttribute("operation", metadata.NormalizedOperation)
	if value := boundedMetadataValue(metadata.AdapterID); value != "" {
		ctx.SetUserAttribute("adapter", value)
	}
	if value := boundedMetadataValue(metadata.Version); value != "" {
		ctx.SetUserAttribute("adapter_version", value)
	}
	if value := boundedMetadataValue(metadata.UsageProfile); value != "" {
		ctx.SetUserAttribute("usage_profile", value)
	}
	if value := boundedMetadataValue(metadata.ProviderMatch); value != "" {
		ctx.SetUserAttribute("provider", value)
	}
}

func normalizeStatisticsMethod(method string) string {
	switch method {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "CONNECT", "OPTIONS", "TRACE":
		return method
	default:
		return methodUnknown
	}
}

func boundedMetadataValue(value string) string {
	if value == "" || len(value) > maxAttributionBytes || !utf8.ValidString(value) {
		return ""
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return ""
		}
	}
	return value
}

func setStatisticsResponseStatusFact(ctx wrapper.HttpContext) {
	rawStatus, _ := proxywasm.GetHttpResponseHeader(":status")
	status := 0
	if len(rawStatus) == 3 &&
		rawStatus[0] >= '0' && rawStatus[0] <= '9' &&
		rawStatus[1] >= '0' && rawStatus[1] <= '9' &&
		rawStatus[2] >= '0' && rawStatus[2] <= '9' {
		parsedStatus := int(rawStatus[0]-'0')*100 +
			int(rawStatus[1]-'0')*10 +
			int(rawStatus[2]-'0')
		if parsedStatus >= 100 && parsedStatus <= 599 {
			status = parsedStatus
		}
	}
	errorClass := "status_unknown"
	switch {
	case status >= 200 && status < 400:
		errorClass = "none"
	case status >= 400 && status < 500:
		errorClass = "http_4xx"
	case status >= 500 && status < 600:
		errorClass = "http_5xx"
	}
	ctx.SetContext(statisticsStatusContext, status)
	ctx.SetContext(statisticsErrorClassContext, errorClass)
	ctx.SetUserAttribute("status_code", status)
	ctx.SetUserAttribute("error_class", errorClass)
}

func completeStatisticsBaseFacts(ctx wrapper.HttpContext, completedAtMillis int64) {
	startedAtMillis, ok := ctx.GetContext(StatisticsRequestStartTime).(int64)
	latencyMillis := int64(1)
	if ok && completedAtMillis > startedAtMillis {
		latencyMillis = completedAtMillis - startedAtMillis
	}
	ctx.SetUserAttribute("latency_ms", latencyMillis)
}

func finalizeStatisticsBaseFacts(
	ctx wrapper.HttpContext,
	config AIStatisticsConfig,
	completedAtMillis int64,
) bool {
	if ctx.GetBoolContext(statisticsBaseMetricsWritten, false) {
		return false
	}
	completeStatisticsBaseFacts(ctx, completedAtMillis)
	writeBaseMetrics(ctx, config)
	return true
}

func writeBaseMetrics(ctx wrapper.HttpContext, config AIStatisticsConfig) {
	if ctx.GetBoolContext(statisticsBaseMetricsWritten, false) {
		return
	}
	route := ctx.GetStringContext(statisticsRouteContext, routeUnknown)
	path := ctx.GetStringContext(statisticsPathContext, providerNativeUnknown)
	method := ctx.GetStringContext(statisticsMethodContext, methodUnknown)
	status, _ := ctx.GetContext(statisticsStatusContext).(int)
	errorClass := ctx.GetStringContext(statisticsErrorClassContext, "status_unknown")
	latency, ok := convertToUInt(ctx.GetUserAttribute("latency_ms"))
	if !ok || latency == 0 {
		latency = 1
	}
	metricPrefix := generateBaseMetricName(route, path, method, status, errorClass)
	config.incrementCounter(metricPrefix+statisticsRequestCountMetric, 1)
	config.incrementCounter(metricPrefix+statisticsRequestDurationMetric, latency)
	ctx.SetContext(statisticsBaseMetricsWritten, true)
}

func generateBaseMetricName(route, path, method string, status int, errorClass string) string {
	return fmt.Sprintf(
		"route.%s.path.%s.method.%s.status.%d.error.%s.metric.",
		route,
		path,
		method,
		status,
		errorClass,
	)
}
