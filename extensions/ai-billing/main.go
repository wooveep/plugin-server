package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/tokenusage"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/resp"
)

const (
	pluginName = "ai-billing"

	defaultQuotaScope      = "global"
	defaultProvider        = "default"
	defaultTenantHeader    = "x-mse-tenant"
	defaultConsumerHeader  = "x-mse-consumer"
	defaultRedisPort       = 6379
	defaultRedisTimeout    = int64(500)
	defaultRedisStream     = "billing:events"
	gatewayRequestIDHeader = "x-ocmf-gateway-request-id"

	FailPolicyOpen = "open"

	eventKindUsage               = "usage"
	eventKindCustomerUsageLegacy = "customer_usage"
	eventKindInternalCost        = "internal_cost"

	ctxBillingEnabled   = "ai-billing-enabled"
	ctxStartTime        = "ai-billing-start-time"
	ctxEventID          = "ai-billing-event-id"
	ctxIdempotencyKey   = "ai-billing-idempotency-key"
	ctxRequestPath      = "ai-billing-request-path"
	ctxRequestID        = "ai-billing-request-id"
	ctxTenant           = "ai-billing-tenant"
	ctxConsumer         = "ai-billing-consumer"
	ctxRoute            = "ai-billing-route"
	ctxCluster          = "ai-billing-cluster"
	ctxProvider         = "ai-billing-provider"
	ctxQuotaScope       = "ai-billing-quota-scope"
	ctxStatusCode       = "ai-billing-status-code"
	ctxPriceVersion     = "ai-billing-price-version"
	ctxIsStream         = "ai-billing-is-stream"
	ctxBillingDelivered = "ai-billing-delivered"
	ctxInputToken       = "ai-billing-input-token"
	ctxOutputToken      = "ai-billing-output-token"
	ctxTotalToken       = "ai-billing-total-token"
	ctxInputCacheHit    = "ai-billing-input-cache-hit-token"
	ctxInputCacheMiss   = "ai-billing-input-cache-miss-token"
	ctxInputDetails     = "ai-billing-input-details"
	ctxOutputDetails    = "ai-billing-output-details"
	ctxModel            = "ai-billing-model"
	ctxRequestText      = "ai-billing-request-text"
	ctxStreamOutputText = "ai-billing-stream-output-text"
	ctxUsageSource      = "ai-billing-usage-source"
	ctxProviderUsage    = "ai-billing-provider-usage"
	ctxSourceJob        = "ai-billing-source-job"
	ctxUpstreamInvoked  = "ai-billing-upstream-invoked"
	ctxCostSource       = "ai-billing-cost-source"
	ctxWorkerKind       = "ai-billing-worker-kind"
)

const (
	internalCostSourceHeader     = "x-ai-billing-cost-source"
	internalCostWorkerKindHeader = "x-ai-billing-worker-kind"
	maxInternalAttributionLength = 128
)

const (
	usageSourceProvider  = "provider"
	usageSourceEstimated = "estimated"
	usageSourceMissing   = "missing"
)

func main() {}

func init() {
	wrapper.SetCtx(
		pluginName,
		wrapper.ParseOverrideConfig(parseConfig, parseRuleConfig),
		wrapper.ProcessRequestHeaders(onHttpRequestHeaders),
		wrapper.ProcessRequestBody(onHttpRequestBody),
		wrapper.ProcessResponseHeaders(onHttpResponseHeaders),
		wrapper.ProcessStreamingResponseBody(onHttpStreamingResponseBody),
		wrapper.ProcessResponseBody(onHttpResponseBody),
		wrapper.ProcessStreamDone(onHttpStreamDone),
	)
}

type BillingConfig struct {
	RedisStream        RedisStream `yaml:"redis_stream"`
	EventKind          string      `yaml:"event_kind"`
	QuotaScope         string      `yaml:"quota_scope"`
	Provider           string      `yaml:"provider"`
	CostSource         string      `yaml:"cost_source"`
	WorkerKind         string      `yaml:"worker_kind"`
	BillCustomer       *bool       `yaml:"bill_customer"`
	TenantHeader       string      `yaml:"tenant_header"`
	ConsumerHeader     string      `yaml:"consumer_header"`
	EnablePathSuffixes []string    `yaml:"enable_path_suffixes"`
	FailPolicy         string      `yaml:"fail_policy"`
	redisClient        wrapper.RedisClient
}

type RedisStream struct {
	ServiceName string `yaml:"service_name" json:"service_name"`
	ServicePort int    `yaml:"service_port" json:"service_port"`
	Username    string `yaml:"username" json:"username"`
	Password    string `yaml:"password" json:"password"`
	Database    int    `yaml:"database" json:"database"`
	Timeout     int64  `yaml:"timeout" json:"timeout"`
	Stream      string `yaml:"stream" json:"stream"`
}

type BillingEvent struct {
	EventID         string       `json:"event_id"`
	EventKind       string       `json:"event_kind"`
	CostSource      string       `json:"cost_source,omitempty"`
	WorkerKind      string       `json:"worker_kind,omitempty"`
	SourceJob       string       `json:"source_job,omitempty"`
	BillCustomer    *bool        `json:"bill_customer,omitempty"`
	IdempotencyKey  string       `json:"idempotency_key"`
	RequestID       string       `json:"request_id"`
	Tenant          string       `json:"tenant"`
	Consumer        string       `json:"consumer"`
	Route           BillingFact  `json:"route"`
	Provider        BillingFact  `json:"provider"`
	QuotaScope      string       `json:"quota_scope"`
	Model           BillingFact  `json:"model"`
	RequestPath     string       `json:"request_path"`
	StatusCode      int          `json:"status_code"`
	Usage           BillingUsage `json:"usage"`
	UsageMissing    bool         `json:"usage_missing"`
	UsageSource     string       `json:"usage_source"`
	StartTimeMs     int64        `json:"start_time_ms"`
	EndTimeMs       int64        `json:"end_time_ms"`
	IsStream        bool         `json:"is_stream"`
	Cluster         string       `json:"cluster"`
	PriceVersion    string       `json:"price_version,omitempty"`
	UpstreamInvoked bool         `json:"upstream_invoked"`
}

type BillingFact struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type BillingUsage struct {
	Unit                 string         `json:"unit"`
	Input                int64          `json:"input"`
	Output               int64          `json:"output"`
	InputCacheHitTokens  *int64         `json:"input_cache_hit_tokens,omitempty"`
	InputCacheMissTokens *int64         `json:"input_cache_miss_tokens,omitempty"`
	OutputTokens         *int64         `json:"output_tokens,omitempty"`
	Total                int64          `json:"total"`
	Details              map[string]any `json:"details"`
}

func (usage BillingUsage) MarshalJSON() ([]byte, error) {
	type billingUsageJSON struct {
		Unit                 string          `json:"unit"`
		Input                int64           `json:"input"`
		Output               int64           `json:"output"`
		InputCacheHitTokens  *int64          `json:"input_cache_hit_tokens,omitempty"`
		InputCacheMissTokens *int64          `json:"input_cache_miss_tokens,omitempty"`
		OutputTokens         *int64          `json:"output_tokens,omitempty"`
		Total                int64           `json:"total"`
		Details              *map[string]any `json:"details,omitempty"`
	}
	var details *map[string]any
	if usage.Details != nil {
		details = &usage.Details
	}
	return json.Marshal(billingUsageJSON{
		Unit:                 usage.Unit,
		Input:                usage.Input,
		Output:               usage.Output,
		InputCacheHitTokens:  usage.InputCacheHitTokens,
		InputCacheMissTokens: usage.InputCacheMissTokens,
		OutputTokens:         usage.OutputTokens,
		Total:                usage.Total,
		Details:              details,
	})
}

func parseConfig(configJson gjson.Result, config *BillingConfig) error {
	if err := parseConfigFields(configJson, config); err != nil {
		return err
	}
	if configJson.Get("billing_service").Exists() {
		return errors.New("billing_service is no longer supported; use redis_stream")
	}
	return parseRedisStream(configJson.Get("redis_stream"), config)
}

func parseRuleConfig(configJson gjson.Result, global BillingConfig, config *BillingConfig) error {
	if global.RedisStream.ServiceName == "" {
		return errors.New("missing redis_stream in config")
	}
	if configJson.Get("billing_service").Exists() {
		return errors.New("billing_service is no longer supported; use redis_stream")
	}
	if configJson.Get("redis_stream").Exists() {
		return errors.New("redis_stream must be configured globally")
	}
	*config = global
	if value := configJson.Get("quota_scope"); value.Exists() {
		config.QuotaScope = stringDefault(value.String(), defaultQuotaScope)
	}
	if value := configJson.Get("event_kind"); value.Exists() {
		eventKind, err := normalizeEventKind(value.String())
		if err != nil {
			return err
		}
		config.EventKind = eventKind
	}
	if value := configJson.Get("provider"); value.Exists() {
		config.Provider = stringDefault(value.String(), defaultProvider)
	}
	if value := configJson.Get("cost_source"); value.Exists() {
		config.CostSource = strings.TrimSpace(value.String())
	}
	if value := configJson.Get("worker_kind"); value.Exists() {
		config.WorkerKind = strings.TrimSpace(value.String())
	}
	if value := configJson.Get("bill_customer"); value.Exists() {
		billCustomer := value.Bool()
		config.BillCustomer = &billCustomer
	}
	if value := configJson.Get("tenant_header"); value.Exists() {
		config.TenantHeader = stringDefault(value.String(), defaultTenantHeader)
	}
	if value := configJson.Get("consumer_header"); value.Exists() {
		config.ConsumerHeader = stringDefault(value.String(), defaultConsumerHeader)
	}
	if value := configJson.Get("fail_policy"); value.Exists() {
		config.FailPolicy = stringDefault(value.String(), FailPolicyOpen)
		if config.FailPolicy != FailPolicyOpen {
			return errors.New("fail_policy only supports open")
		}
	}
	if value := configJson.Get("enable_path_suffixes"); value.Exists() {
		suffixes, err := parsePathSuffixes(value)
		if err != nil {
			return err
		}
		config.EnablePathSuffixes = suffixes
	}
	return nil
}

func parseConfigFields(configJson gjson.Result, config *BillingConfig) error {
	eventKind, err := normalizeEventKind(configJson.Get("event_kind").String())
	if err != nil {
		return err
	}
	config.EventKind = eventKind
	config.QuotaScope = stringDefault(configJson.Get("quota_scope").String(), defaultQuotaScope)
	config.Provider = stringDefault(configJson.Get("provider").String(), defaultProvider)
	config.CostSource = strings.TrimSpace(configJson.Get("cost_source").String())
	config.WorkerKind = strings.TrimSpace(configJson.Get("worker_kind").String())
	if value := configJson.Get("bill_customer"); value.Exists() {
		billCustomer := value.Bool()
		config.BillCustomer = &billCustomer
	}
	config.TenantHeader = stringDefault(configJson.Get("tenant_header").String(), defaultTenantHeader)
	config.ConsumerHeader = stringDefault(configJson.Get("consumer_header").String(), defaultConsumerHeader)
	config.FailPolicy = stringDefault(configJson.Get("fail_policy").String(), FailPolicyOpen)
	if config.FailPolicy != FailPolicyOpen {
		return errors.New("fail_policy only supports open")
	}
	suffixes, err := parsePathSuffixes(configJson.Get("enable_path_suffixes"))
	if err != nil {
		return err
	}
	config.EnablePathSuffixes = suffixes
	return nil
}

func validateEventKind(value string) error {
	_, err := normalizeEventKind(value)
	return err
}

func normalizeEventKind(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case "", eventKindUsage, eventKindCustomerUsageLegacy:
		return eventKindUsage, nil
	case eventKindInternalCost:
		return eventKindInternalCost, nil
	default:
		return "", errors.New("event_kind only supports usage or internal_cost")
	}
}

func normalizedEventKindOrDefault(value string) string {
	eventKind, err := normalizeEventKind(value)
	if err == nil {
		return eventKind
	}
	return eventKindUsage
}

func parseRedisStream(redisStream gjson.Result, config *BillingConfig) error {
	if !redisStream.Exists() {
		return errors.New("missing redis_stream in config")
	}
	serviceName := redisStream.Get("service_name").String()
	if serviceName == "" {
		return errors.New("redis_stream.service_name must not be empty")
	}
	servicePort := int(redisStream.Get("service_port").Int())
	if servicePort == 0 {
		servicePort = defaultRedisPort
	}
	timeout := redisStream.Get("timeout").Int()
	if timeout == 0 {
		timeout = defaultRedisTimeout
	}
	stream := stringDefault(redisStream.Get("stream").String(), defaultRedisStream)
	username := redisStream.Get("username").String()
	password := redisStream.Get("password").String()
	database := int(redisStream.Get("database").Int())
	config.RedisStream = RedisStream{
		ServiceName: serviceName,
		ServicePort: servicePort,
		Username:    username,
		Password:    password,
		Database:    database,
		Timeout:     timeout,
		Stream:      stream,
	}
	config.redisClient = wrapper.NewRedisClusterClient(wrapper.FQDNCluster{
		FQDN: serviceName,
		Port: int64(servicePort),
	})
	return config.redisClient.Init(username, password, timeout, wrapper.WithDataBase(database))
}

func onHttpRequestHeaders(ctx wrapper.HttpContext, config BillingConfig) types.Action {
	requestPath := ctx.Path()
	if !isAIPathEnabled(requestPath, config.EnablePathSuffixes) {
		ctx.SetContext(ctxBillingEnabled, false)
		ctx.DontReadRequestBody()
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}
	ctx.SetContext(ctxBillingEnabled, true)
	requestID, _ := proxywasm.GetHttpRequestHeader("x-request-id")
	if requestID == "" {
		requestID = stringProperty([]string{"x_request_id"}, "")
	}
	tenant, _ := proxywasm.GetHttpRequestHeader(config.TenantHeader)
	consumer, _ := proxywasm.GetHttpRequestHeader(config.ConsumerHeader)
	if config.EventKind == eventKindInternalCost {
		tenant = ""
		if !captureInternalCostAttribution(ctx) {
			log.Warn("ai-billing internal attribution rejected; delivery disabled for request")
			ctx.SetContext(ctxBillingEnabled, false)
			ctx.DontReadRequestBody()
			ctx.DontReadResponseBody()
			return types.ActionContinue
		}
		if sourceJob, _ := proxywasm.GetHttpRequestHeader("x-ai-billing-source-job"); sourceJob != "" {
			ctx.SetContext(ctxSourceJob, strings.TrimSpace(sourceJob))
		}
		if upstreamInvoked, _ := proxywasm.GetHttpRequestHeader("x-ai-billing-upstream-invoked"); upstreamInvoked != "" {
			if parsed, ok := parseBoolHeader(upstreamInvoked); ok {
				ctx.SetContext(ctxUpstreamInvoked, parsed)
			}
		}
	}
	priceVersion, _ := proxywasm.GetHttpRequestHeader("x-ai-price-version")
	if _, err := initBillingRequestContext(ctx, requestPath, requestID, tenant, consumer, config.Provider, config.QuotaScope, priceVersion); err != nil {
		log.Warnf("ai-billing event id generation failed open, request_id:%s err:%v", requestID, err)
		ctx.SetContext(ctxBillingEnabled, false)
		ctx.DontReadRequestBody()
		ctx.DontReadResponseBody()
	}
	return types.ActionContinue
}

func internalAttributionValue(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxInternalAttributionLength {
		return "", false
	}
	for _, char := range value {
		if !(char >= 'a' && char <= 'z') &&
			!(char >= '0' && char <= '9') &&
			char != '.' && char != '-' && char != '_' {
			return "", false
		}
	}
	return value, true
}

func captureInternalCostAttribution(ctx wrapper.HttpContext) bool {
	headers, err := proxywasm.GetHttpRequestHeaders()
	if err != nil {
		return false
	}
	var costSourceHeader, workerKindHeader string
	var costSourcePresent, workerKindPresent bool
	for _, header := range headers {
		switch strings.ToLower(header[0]) {
		case internalCostSourceHeader:
			costSourceHeader = header[1]
			costSourcePresent = true
		case internalCostWorkerKindHeader:
			workerKindHeader = header[1]
			workerKindPresent = true
		}
	}
	if !costSourcePresent && !workerKindPresent {
		return true
	}
	costSource, sourceValid := internalAttributionValue(costSourceHeader)
	workerKind, workerValid := internalAttributionValue(workerKindHeader)
	if !sourceValid || !workerValid {
		return false
	}
	ctx.SetContext(ctxCostSource, costSource)
	ctx.SetContext(ctxWorkerKind, workerKind)
	return true
}

func initBillingRequestContext(ctx wrapper.HttpContext, requestPath, requestID, tenant, consumer, provider, quotaScope, priceVersion string) (string, error) {
	eventID, err := newEventID()
	if err != nil {
		return "", err
	}
	ctx.SetContext(ctxStartTime, time.Now().UnixMilli())
	ctx.SetContext(ctxEventID, eventID)
	ctx.SetContext(ctxIdempotencyKey, eventID)
	ctx.SetContext(ctxRequestPath, requestPath)
	ctx.SetContext(ctxRequestID, requestID)
	ctx.SetContext(ctxProvider, provider)
	ctx.SetContext(ctxQuotaScope, quotaScope)
	if tenant != "" {
		ctx.SetContext(ctxTenant, tenant)
	}
	if consumer != "" {
		ctx.SetContext(ctxConsumer, consumer)
	}
	if priceVersion != "" {
		ctx.SetContext(ctxPriceVersion, priceVersion)
	}
	ctx.SetContext(ctxRoute, stringProperty([]string{"route_name"}, "-"))
	ctx.SetContext(ctxCluster, stringProperty([]string{"cluster_name"}, "-"))
	return eventID, nil
}

func onHttpResponseHeaders(ctx wrapper.HttpContext, config BillingConfig) types.Action {
	_ = proxywasm.RemoveHttpResponseHeader(gatewayRequestIDHeader)
	if !ctx.GetBoolContext(ctxBillingEnabled, false) {
		return types.ActionContinue
	}
	if requestID := ctx.GetStringContext(ctxRequestID, ""); requestID != "" {
		_ = proxywasm.AddHttpResponseHeader(gatewayRequestIDHeader, requestID)
	}
	status, _ := proxywasm.GetHttpResponseHeader(":status")
	statusCode, err := strconv.Atoi(status)
	if err != nil {
		statusCode = http.StatusBadGateway
	}
	ctx.SetContext(ctxStatusCode, statusCode)

	contentType, _ := proxywasm.GetHttpResponseHeader("content-type")
	isStream := strings.Contains(contentType, "text/event-stream")
	ctx.SetContext(ctxIsStream, isStream)
	if !isStream {
		ctx.BufferResponseBody()
	}
	return types.ActionContinue
}

func onHttpRequestBody(ctx wrapper.HttpContext, config BillingConfig, body []byte) types.Action {
	if !ctx.GetBoolContext(ctxBillingEnabled, false) {
		return types.ActionContinue
	}
	if text, ok := extractRequestInputText(body); ok {
		ctx.SetContext(ctxRequestText, text)
	}
	return types.ActionContinue
}

func onHttpStreamingResponseBody(ctx wrapper.HttpContext, config BillingConfig, data []byte, endOfStream bool) []byte {
	if !ctx.GetBoolContext(ctxBillingEnabled, false) {
		return data
	}
	ctx.SetContext(ctxIsStream, true)
	recordStreamingOutputText(ctx, data)
	recordUsage(ctx, data)
	if endOfStream {
		if ctx.GetStringContext(ctxUsageSource, "") == "" {
			recordStreamingEstimatedUsage(ctx)
		}
		deliverBillingEvent(ctx, config, true)
	}
	return data
}

func onHttpStreamDone(ctx wrapper.HttpContext, config BillingConfig) {
	if !ctx.GetBoolContext(ctxBillingEnabled, false) {
		return
	}
	if !ctx.GetBoolContext(ctxIsStream, false) {
		return
	}
	if ctx.GetBoolContext(ctxBillingDelivered, false) {
		return
	}
	if ctx.GetStringContext(ctxUsageSource, "") == "" {
		recordStreamingEstimatedUsage(ctx)
	}
	deliverBillingEvent(ctx, config, true)
}

func onHttpResponseBody(ctx wrapper.HttpContext, config BillingConfig, body []byte) types.Action {
	if !ctx.GetBoolContext(ctxBillingEnabled, false) {
		return types.ActionContinue
	}
	ctx.SetContext(ctxIsStream, false)
	recordUsage(ctx, body)
	if ctx.GetStringContext(ctxUsageSource, "") == "" {
		recordNonStreamingEstimatedUsage(ctx, body)
	}
	deliverBillingEvent(ctx, config, false)
	return types.ActionContinue
}

func recordUsage(ctx wrapper.HttpContext, body []byte) {
	usage := normalizeProviderTokenUsage(tokenusage.GetTokenUsage(ctx, body))
	usageSource := usageSourceProvider
	if usage.InputToken <= 0 && len(usage.ProviderUsage) > 0 {
		if estimated, ok := estimateTextTokenUsage(usage.Model, ctx.GetStringContext(ctxRequestText, ""), ""); ok {
			usage.InputToken = estimated.InputToken
			usage.OutputToken = nonNegativeInt64(usage.OutputToken)
			usage.TotalToken = usage.InputToken + usage.OutputToken
			usageSource = usageSourceEstimated
		}
	}
	if usage.InputToken <= 0 && len(usage.ProviderUsage) > 0 {
		return
	}
	if usage.TotalToken <= 0 {
		return
	}
	inputTokens := usage.InputToken
	hasCacheAwareUsage := false
	if hitTokens, missTokens, ok := cacheAwareProviderUsageSplit(usage.InputToken, usage.ProviderUsage); ok {
		hasCacheAwareUsage = true
		inputTokens = hitTokens + missTokens
		ctx.SetContext(ctxInputCacheHit, hitTokens)
		ctx.SetContext(ctxInputCacheMiss, missTokens)
	} else if hitTokens, missTokens, ok := cacheAwareInputTokenSplit(usage.InputToken, usage.InputTokenDetails); ok {
		hasCacheAwareUsage = true
		inputTokens = hitTokens + missTokens
		ctx.SetContext(ctxInputCacheHit, hitTokens)
		ctx.SetContext(ctxInputCacheMiss, missTokens)
	}
	ctx.SetContext(ctxInputToken, inputTokens)
	ctx.SetContext(ctxOutputToken, usage.OutputToken)
	ctx.SetContext(ctxTotalToken, usage.TotalToken)
	if hasCacheAwareUsage && len(usage.InputTokenDetails) > 0 {
		ctx.SetContext(ctxInputDetails, mergeTokenDetailsFromContext(ctx.GetContext(ctxInputDetails), usage.InputTokenDetails))
	}
	if hasCacheAwareUsage && len(usage.OutputTokenDetails) > 0 {
		ctx.SetContext(ctxOutputDetails, mergeTokenDetailsFromContext(ctx.GetContext(ctxOutputDetails), usage.OutputTokenDetails))
	}
	if len(usage.ProviderUsage) > 0 {
		ctx.SetContext(ctxProviderUsage, usage.ProviderUsage)
	}
	ctx.SetContext(ctxModel, usage.Model)
	ctx.SetContext(ctxUsageSource, usageSource)
}

func recordNonStreamingEstimatedUsage(ctx wrapper.HttpContext, body []byte) bool {
	outputText, _ := extractResponseOutputText(body)
	return recordEstimatedUsage(ctx, responseModel(body), ctx.GetStringContext(ctxRequestText, ""), outputText)
}

func normalizeProviderTokenUsage(usage tokenusage.TokenUsage) tokenusage.TokenUsage {
	if inputTokens, ok := firstPositiveProviderUsageInt64(usage.ProviderUsage,
		"prompt_tokens",
		"input_tokens",
		"promptTokenCount",
	); ok {
		usage.InputToken = inputTokens
	}
	if outputTokens, ok := firstPositiveProviderUsageInt64(usage.ProviderUsage,
		"completion_tokens",
		"output_tokens",
		"candidatesTokenCount",
	); ok {
		usage.OutputToken = outputTokens
	}
	if totalTokens, ok := firstPositiveProviderUsageInt64(usage.ProviderUsage,
		"total_tokens",
		"totalTokenCount",
	); ok {
		usage.TotalToken = totalTokens
	}
	if usage.InputToken <= 0 {
		if hitTokens, missTokens, ok := cacheAwareInputTokenSplit(usage.InputToken, usage.InputTokenDetails); ok {
			if inputTokens := hitTokens + missTokens; inputTokens > 0 {
				usage.InputToken = inputTokens
			}
		}
	}
	usage.InputToken, usage.OutputToken, usage.TotalToken = normalizeBillingUsageTotals(
		usage.InputToken,
		usage.OutputToken,
		usage.TotalToken,
	)
	return usage
}

func recordStreamingOutputText(ctx wrapper.HttpContext, data []byte) {
	outputText, ok := extractStreamingResponseOutputText(data)
	if !ok {
		return
	}
	ctx.SetContext(ctxStreamOutputText, ctx.GetStringContext(ctxStreamOutputText, "")+outputText)
}

func recordStreamingEstimatedUsage(ctx wrapper.HttpContext) bool {
	return recordEstimatedUsage(
		ctx,
		ctx.GetStringContext(ctxModel, ""),
		ctx.GetStringContext(ctxRequestText, ""),
		ctx.GetStringContext(ctxStreamOutputText, ""),
	)
}

func recordEstimatedUsage(ctx wrapper.HttpContext, model, inputText, outputText string) bool {
	usage, ok := estimateTextTokenUsage(model, inputText, outputText)
	if !ok {
		return false
	}
	ctx.SetContext(ctxInputToken, usage.InputToken)
	ctx.SetContext(ctxOutputToken, usage.OutputToken)
	ctx.SetContext(ctxTotalToken, usage.TotalToken)
	if strings.TrimSpace(model) == "" {
		model = tokenusage.ModelUnknown
	}
	ctx.SetContext(ctxModel, model)
	ctx.SetContext(ctxUsageSource, usageSourceEstimated)
	return true
}

func responseModel(body []byte) string {
	var response struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return tokenusage.ModelUnknown
	}
	if strings.TrimSpace(response.Model) == "" {
		return tokenusage.ModelUnknown
	}
	return response.Model
}

func deliverBillingEvent(ctx wrapper.HttpContext, config BillingConfig, isStream bool) {
	ctx.SetContext(ctxBillingDelivered, true)
	event := buildBillingEvent(ctx, config, isStream)
	sendBillingEvent(config, event)
}

func sendBillingEvent(config BillingConfig, event BillingEvent) {
	body, err := json.Marshal(event)
	if err != nil {
		log.Errorf("ai-billing marshal event failed: %v", err)
		return
	}
	if config.redisClient == nil {
		log.Warnf("ai-billing redis dispatch skipped fail open, request_id:%s err:redis client is nil", event.RequestID)
		return
	}
	err = config.redisClient.Command(
		[]interface{}{"xadd", config.RedisStream.Stream, "*", "event", string(body)},
		func(response resp.Value) {
			if response.Error() != nil {
				log.Warnf("ai-billing redis delivery failed open, request_id:%s err:%v", event.RequestID, response.Error())
				return
			}
			log.Debugf("ai-billing redis delivery accepted, stream:%s request_id:%s id:%s", config.RedisStream.Stream, event.RequestID, response.String())
		},
	)
	if err != nil {
		log.Warnf("ai-billing redis dispatch failed open, request_id:%s err:%v", event.RequestID, err)
	}
}

func buildBillingEvent(ctx wrapper.HttpContext, config BillingConfig, isStream bool) BillingEvent {
	inputTokens := int64FromContext(ctx.GetContext(ctxInputToken))
	outputTokens := int64FromContext(ctx.GetContext(ctxOutputToken))
	totalTokens := int64FromContext(ctx.GetContext(ctxTotalToken))
	usageSource := ctx.GetStringContext(ctxUsageSource, "")
	if usageSource == "" {
		if totalTokens <= 0 {
			usageSource = usageSourceMissing
		} else {
			usageSource = usageSourceProvider
		}
	}
	if usageSource != usageSourceMissing {
		inputTokens, outputTokens, totalTokens = normalizeBillingUsageTotals(inputTokens, outputTokens, totalTokens)
	}
	usageMissing := usageSource == usageSourceMissing || totalTokens <= 0
	if usageMissing {
		inputTokens = 0
		outputTokens = 0
		totalTokens = 0
	}

	var inputCacheHitTokens *int64
	var inputCacheMissTokens *int64
	var nativeOutputTokens *int64
	if usageSource != usageSourceEstimated && !usageMissing {
		if hitTokens, ok := optionalInt64FromContext(ctx.GetContext(ctxInputCacheHit)); ok {
			if missTokens, ok := optionalInt64FromContext(ctx.GetContext(ctxInputCacheMiss)); ok {
				inputCacheHitTokens = int64Ptr(hitTokens)
				inputCacheMissTokens = int64Ptr(missTokens)
				nativeOutputTokens = int64Ptr(outputTokens)
			}
		}
	}
	usageDetails := billingUsageDetails(ctx)
	if usageSource == usageSourceEstimated {
		usageDetails = nil
	} else if usageMissing {
		usageDetails = map[string]any{}
	}

	requestID := ctx.GetStringContext(ctxRequestID, "")
	eventID := ctx.GetStringContext(ctxEventID, "")
	idempotencyKey := ctx.GetStringContext(ctxIdempotencyKey, eventID)
	cluster := ctx.GetStringContext(ctxCluster, "-")
	provider := ""
	if clusterProvider := providerSlugFromCluster(cluster); clusterProvider != "" {
		provider = clusterProvider
	}
	event := BillingEvent{
		EventID:        eventID,
		EventKind:      normalizedEventKindOrDefault(config.EventKind),
		IdempotencyKey: idempotencyKey,
		RequestID:      requestID,
		Tenant:         ctx.GetStringContext(ctxTenant, ""),
		Consumer:       ctx.GetStringContext(ctxConsumer, ""),
		Route:          namedBillingFact(ctx.GetStringContext(ctxRoute, "-")),
		Provider:       namedBillingFact(provider),
		QuotaScope:     ctx.GetStringContext(ctxQuotaScope, config.QuotaScope),
		Model:          namedBillingFact(ctx.GetStringContext(ctxModel, tokenusage.ModelUnknown)),
		RequestPath:    ctx.GetStringContext(ctxRequestPath, ""),
		StatusCode:     intDefault(intFromContext(ctx.GetContext(ctxStatusCode)), http.StatusBadGateway),
		Usage: BillingUsage{
			Unit:                 "token",
			Input:                inputTokens,
			Output:               outputTokens,
			InputCacheHitTokens:  inputCacheHitTokens,
			InputCacheMissTokens: inputCacheMissTokens,
			OutputTokens:         nativeOutputTokens,
			Total:                totalTokens,
			Details:              usageDetails,
		},
		StartTimeMs:     int64FromContext(ctx.GetContext(ctxStartTime)),
		EndTimeMs:       time.Now().UnixMilli(),
		IsStream:        ctx.GetBoolContext(ctxIsStream, isStream),
		UsageMissing:    usageMissing,
		UsageSource:     usageSource,
		Cluster:         cluster,
		PriceVersion:    ctx.GetStringContext(ctxPriceVersion, ""),
		UpstreamInvoked: trustedUpstreamInvoked(ctx),
	}
	if event.EventKind == eventKindInternalCost {
		event.Tenant = ""
		event.CostSource = ctx.GetStringContext(ctxCostSource, strings.TrimSpace(config.CostSource))
		event.WorkerKind = ctx.GetStringContext(ctxWorkerKind, strings.TrimSpace(config.WorkerKind))
		event.SourceJob = ctx.GetStringContext(ctxSourceJob, "")
		event.BillCustomer = config.BillCustomer
	}
	return event
}

func trustedUpstreamInvoked(ctx wrapper.HttpContext) bool {
	if upstreamInvoked, ok := ctx.GetContext(ctxUpstreamInvoked).(bool); ok {
		return upstreamInvoked
	}
	if upstreamInvoked, ok := ctx.GetUserAttribute("upstream_invoked").(bool); ok {
		return upstreamInvoked
	}
	if codeDetails, ok := responseCodeDetails(); ok {
		return codeDetails == "via_upstream"
	}
	return true
}

func responseCodeDetails() (details string, ok bool) {
	defer func() {
		if recover() != nil {
			details = ""
			ok = false
		}
	}()
	raw, err := proxywasm.GetProperty([]string{"response", "code_details"})
	if err != nil || len(raw) == 0 {
		return "", false
	}
	return string(raw), true
}

func parseBoolHeader(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "y":
		return true, true
	case "false", "0", "no", "n":
		return false, true
	default:
		return false, false
	}
}

func normalizeBillingUsageTotals(inputTokens, outputTokens, totalTokens int64) (int64, int64, int64) {
	inputTokens = nonNegativeInt64(inputTokens)
	outputTokens = nonNegativeInt64(outputTokens)
	totalTokens = nonNegativeInt64(totalTokens)
	if totalTokens <= 0 {
		if inputTokens > 0 || outputTokens > 0 {
			totalTokens = inputTokens + outputTokens
		}
		return inputTokens, outputTokens, totalTokens
	}
	if inputTokens == 0 && totalTokens >= outputTokens {
		inputTokens = totalTokens - outputTokens
	}
	if outputTokens == 0 && totalTokens >= inputTokens {
		outputTokens = totalTokens - inputTokens
	}
	return inputTokens, outputTokens, totalTokens
}

func firstPositiveProviderUsageInt64(providerUsage map[string]any, keys ...string) (int64, bool) {
	for _, key := range keys {
		value, ok := providerUsageInt64(providerUsage, key)
		if ok && value > 0 {
			return value, true
		}
	}
	return 0, false
}

func providerUsageInt64(providerUsage map[string]any, key string) (int64, bool) {
	if len(providerUsage) == 0 {
		return 0, false
	}
	switch value := providerUsage[key].(type) {
	case int:
		return nonNegativeProviderUsageInt64(int64(value))
	case int64:
		return nonNegativeProviderUsageInt64(value)
	case int32:
		return nonNegativeProviderUsageInt64(int64(value))
	case float64:
		if value != float64(int64(value)) {
			return 0, false
		}
		return nonNegativeProviderUsageInt64(int64(value))
	case json.Number:
		parsed, err := value.Int64()
		if err != nil {
			return 0, false
		}
		return nonNegativeProviderUsageInt64(parsed)
	default:
		return 0, false
	}
}

func nonNegativeProviderUsageInt64(value int64) (int64, bool) {
	if value < 0 {
		return 0, false
	}
	return value, true
}

func namedBillingFact(name string) BillingFact {
	return BillingFact{Name: strings.TrimSpace(name)}
}

func providerSlugFromCluster(cluster string) string {
	cluster = strings.TrimSpace(cluster)
	if cluster == "" {
		return ""
	}

	if strings.HasPrefix(cluster, "outbound|") {
		parts := strings.Split(cluster, "|")
		if len(parts) != 4 || parts[0] != "outbound" || parts[2] != "" {
			return ""
		}
		if port, err := strconv.Atoi(parts[1]); err != nil || port <= 0 || port > 65535 {
			return ""
		}
		return providerSlugFromCluster(parts[3])
	}

	slug, ok := strings.CutPrefix(cluster, "llm-")
	if !ok {
		return ""
	}
	switch {
	case strings.HasSuffix(slug, ".internal.dns"):
		slug = strings.TrimSuffix(slug, ".internal.dns")
	case strings.HasSuffix(slug, ".dns"):
		slug = strings.TrimSuffix(slug, ".dns")
	default:
		return ""
	}
	if slug == "" || !isValidProviderSlug(slug) {
		return ""
	}
	return slug
}

func isValidProviderSlug(slug string) bool {
	if slug == "" || slug[0] == '-' || slug[len(slug)-1] == '-' {
		return false
	}
	for _, r := range slug {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-':
		default:
			return false
		}
	}
	return true
}

func billingUsageDetails(ctx wrapper.HttpContext) map[string]any {
	details := map[string]any{}
	if inputDetails := tokenDetailsFromContext(ctx.GetContext(ctxInputDetails)); len(inputDetails) > 0 {
		details["input"] = inputDetails
	}
	if outputDetails := tokenDetailsFromContext(ctx.GetContext(ctxOutputDetails)); len(outputDetails) > 0 {
		details["output"] = outputDetails
	}
	if providerUsage := providerUsageFromContext(ctx.GetContext(ctxProviderUsage)); len(providerUsage) > 0 {
		details["provider_usage"] = providerUsage
	}
	return details
}

func providerUsageFromContext(value any) map[string]any {
	if usage, ok := value.(map[string]any); ok {
		return usage
	}
	return nil
}

func cacheAwareProviderUsageSplit(inputTokens int64, providerUsage map[string]any) (int64, int64, bool) {
	hitTokens, hasHit := providerUsageInt64(providerUsage, "input_cache_hit_tokens")
	missTokens, hasMiss := providerUsageInt64(providerUsage, "input_cache_miss_tokens")
	if !hasHit || !hasMiss {
		return 0, 0, false
	}
	inputTokens = nonNegativeInt64(inputTokens)
	if hitTokens > inputTokens || missTokens != inputTokens-hitTokens {
		return 0, 0, false
	}
	return hitTokens, missTokens, true
}

func cacheAwareInputTokenSplit(inputTokens int64, inputDetails map[string]int64) (int64, int64, bool) {
	if len(inputDetails) == 0 {
		return 0, 0, false
	}
	deepSeekHitTokens, hasDeepSeekHit := inputDetails[tokenusage.InputTokenDetailsKeyDeepSeekPromptCacheHitTokens]
	deepSeekMissTokens, hasDeepSeekMiss := inputDetails[tokenusage.InputTokenDetailsKeyDeepSeekPromptCacheMissTokens]
	if hasDeepSeekHit || hasDeepSeekMiss {
		hitTokens := nonNegativeInt64(deepSeekHitTokens)
		if hasDeepSeekMiss {
			missTokens := nonNegativeInt64(deepSeekMissTokens)
			if hitTokens == 0 && missTokens == 0 && inputTokens > 0 {
				return 0, nonNegativeInt64(inputTokens), true
			}
			return hitTokens, missTokens, true
		}
		inputTokens = nonNegativeInt64(inputTokens)
		if hitTokens > inputTokens {
			hitTokens = inputTokens
		}
		return hitTokens, inputTokens - hitTokens, true
	}

	cacheCreationTokens, hasCacheCreation := inputDetails[tokenusage.InputTokenDetailsKeyAnthropicMessagesUsageCacheCreationInputTokens]
	cacheReadTokens, hasCacheRead := inputDetails[tokenusage.InputTokenDetailsKeyAnthropicMessagesUsageCacheReadInputTokens]
	if hasCacheCreation || hasCacheRead {
		hitTokens := nonNegativeInt64(cacheReadTokens)
		missTokens := nonNegativeInt64(inputTokens) + nonNegativeInt64(cacheCreationTokens)
		return hitTokens, missTokens, true
	}

	cachedTokens := int64(0)
	hasCachedTokens := false
	for _, key := range []string{
		tokenusage.InputTokenDetailsKeyCachedTokens,
		tokenusage.InputTokenDetailsKeyGeminiCachedContentTokenCount,
	} {
		value, ok := inputDetails[key]
		if !ok {
			continue
		}
		cachedTokens += nonNegativeInt64(value)
		hasCachedTokens = true
	}
	if !hasCachedTokens {
		return 0, 0, false
	}
	inputTokens = nonNegativeInt64(inputTokens)
	if cachedTokens > inputTokens {
		cachedTokens = inputTokens
	}
	return cachedTokens, inputTokens - cachedTokens, true
}

func mergeTokenDetailsFromContext(value any, updates map[string]int64) map[string]int64 {
	merged := map[string]int64{}
	for key, existingValue := range tokenDetailsFromContext(value) {
		merged[key] = existingValue
	}
	for key, updateValue := range updates {
		merged[key] = updateValue
	}
	return merged
}

func tokenDetailsFromContext(value any) map[string]int64 {
	if details, ok := value.(map[string]int64); ok {
		return details
	}
	return nil
}

func isAIPathEnabled(requestPath string, enabledSuffixes []string) bool {
	pathWithoutQuery := requestPath
	if parsed, err := url.Parse(requestPath); err == nil && parsed.Path != "" {
		pathWithoutQuery = parsed.Path
	} else if queryPos := strings.Index(requestPath, "?"); queryPos != -1 {
		pathWithoutQuery = requestPath[:queryPos]
	}
	for _, suffix := range enabledSuffixes {
		if strings.HasSuffix(pathWithoutQuery, suffix) {
			return true
		}
	}
	return false
}

func parsePathSuffixes(result gjson.Result) ([]string, error) {
	if !result.Exists() {
		return []string{"/v1/chat/completions", "/v1/messages"}, nil
	}
	if !result.IsArray() {
		return nil, errors.New("enable_path_suffixes must be an array")
	}
	values := result.Array()
	suffixes := make([]string, 0, len(values))
	for _, suffix := range values {
		suffixStr := strings.TrimSpace(suffix.String())
		if suffixStr != "" {
			suffixes = append(suffixes, suffixStr)
		}
	}
	if len(suffixes) == 0 {
		return nil, errors.New("enable_path_suffixes must not be empty")
	}
	return suffixes, nil
}

func stringProperty(path []string, fallback string) string {
	raw, err := proxywasm.GetProperty(path)
	if err != nil || len(raw) == 0 {
		return fallback
	}
	return string(raw)
}

func newEventID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func stringDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func int64FromContext(value interface{}) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	default:
		return 0
	}
}

func optionalInt64FromContext(value interface{}) (int64, bool) {
	switch v := value.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	default:
		return 0, false
	}
}

func int64Ptr(value int64) *int64 {
	return &value
}

func nonNegativeInt64(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func intFromContext(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	default:
		return 0
	}
}

func intDefault(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}
