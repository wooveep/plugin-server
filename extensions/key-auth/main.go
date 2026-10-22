// Copyright (c) 2023 Alibaba Group Holding Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

var (
	ruleSet bool // 插件是否至少在一个 domain 或 route 上生效
)

func main() {}

const (
	defaultProtectionSpace = "MSE Gateway"
	consumerHeaderName     = "X-Mse-Consumer"
	tenantHeaderName       = "X-Mse-Tenant"
)

func init() {
	wrapper.SetCtx(
		"key-auth", // middleware name
		wrapper.ParseOverrideConfigBy(parseGlobalConfig, parseOverrideRuleConfig),
		wrapper.ProcessRequestHeadersBy(onHttpRequestHeaders),
	)
}

type Consumer struct {
	// @Title 名称
	// @Title en-US Name
	// @Description 该调用方的名称。
	// @Description en-US The name of the consumer.
	Name string `yaml:"name"`

	// @Title 访问凭证
	// @Title en-US Credential
	// @Description 该调用方的访问凭证。
	// @Description en-US The credential of the consumer.
	// @Scope GLOBAL
	Credential string `yaml:"credential"`

	// @Title 访问凭证列表
	// @Title en-US Credentials
	// @Description 该调用方的访问凭证列表。
	// @Description en-US The credentials of the consumer.
	// @Scope GLOBAL
	Credentials []string `yaml:"credentials,omitempty"`

	// @Title 租户
	// @Title en-US Tenant
	// @Description 该调用方所属租户。
	// @Description en-US The tenant of the consumer.
	// @Scope GLOBAL
	Tenant string `yaml:"tenant,omitempty"`

	// @Title API Key 的来源字段名称列表
	// @Title en-US The name of the source field of the API Key
	// @Description 当前调用方覆盖全局配置的 API Key 来源字段名称。
	// @Description en-US Consumer-level API Key source field names overriding the global config.
	// @Scope GLOBAL
	Keys []string `yaml:"keys,omitempty"`

	// @Title key是否来源于URL参数
	// @Title en-US the API Key from the URL parameters.
	// @Description 当前调用方覆盖全局配置的 URL 参数来源开关。
	// @Description en-US Consumer-level URL parameter source switch overriding the global config.
	// @Scope GLOBAL
	InQuery *bool `yaml:"in_query,omitempty"`

	// @Title key是否来源于Header
	// @Title en-US the API Key from the HTTP request header name.
	// @Description 当前调用方覆盖全局配置的 HTTP 请求头来源开关。
	// @Description en-US Consumer-level HTTP header source switch overriding the global config.
	// @Scope GLOBAL
	InHeader *bool `yaml:"in_header,omitempty"`
}

type extractionPlan struct {
	Keys     []string
	InHeader bool
	InQuery  bool
}

type credentialIdentity struct {
	Name     string
	Tenant   string
	Consumer bool
	Plan     extractionPlan
}

type credentialCandidate struct {
	Value   string
	Source  string
	Key     string
	Ordinal int
}

// @Name key-auth
// @Category auth
// @Phase AUTHN
// @Priority 321
// @Title zh-CN Key Auth
// @Description zh-CN 本插件实现了实现了基于 API Key 进行认证鉴权的功能.
// @Description en-US This plugin implements an authentication function based on API Key Auth standard.
// @IconUrl https://img.alicdn.com/imgextra/i4/O1CN01BPFGlT1pGZ2VDLgaH_!!6000000005333-2-tps-42-42.png
// @Version 1.0.3
//
// @Contact.name Higress Team
// @Contact.url http://higress.io/
// @Contact.email admin@higress.io
//
// @Example
// global_auth: false
// consumers:
//   - name: consumer1
//     credential: token1
//   - name: consumer2
//     credential: token2
//
// keys:
//   - x-api-key
//   - token
//
// in_query: true
// @End
type KeyAuthConfig struct {
	// @Title 是否开启全局认证
	// @Title en-US Enable Global Auth
	// @Description 若不开启全局认证，则全局配置只提供凭证信息。只有在域名或路由上进行了配置才会启用认证。
	// @Description en-US If set to false, only consumer info will be accepted from the global config. Auth feature shall only be enabled if the corresponding domain or route is configured.
	// @Scope GLOBAL
	globalAuth *bool `yaml:"global_auth,omitempty"` //是否开启全局认证. 若不开启全局认证，则全局配置只提供凭证信息。只有在域名或路由上进行了配置才会启用认证。

	// @Title API Key 的来源字段名称列表
	// @Title en-US The name of the source field of the API Key
	// @Description API Key 的来源字段名称，可以是 URL 参数或者 HTTP 请求头名称.
	// @Description en-US The name of the source field of the API Key, which can be a URL parameter or an HTTP request header name.
	// @Scope GLOBAL
	Keys []string `yaml:"keys"` // key auth names

	// @Title key是否来源于URL参数
	// @Title en-US the API Key from the URL parameters.
	// @Description 如果配置 true 时，网关会尝试从 URL 参数中解析 API Key
	// @Description en-US When configured true, the gateway will try to parse the API Key from the URL parameters.
	// @Scope GLOBAL
	InQuery bool `yaml:"in_query,omitempty"`

	// @Title key是否来源于Header
	// @Title en-US the API Key from the HTTP request header name.
	// @Description 配置 true 时，网关会尝试从 URL header头中解析 API Key
	// @Description en-US When configured true, the gateway will try to parse the API Key from the HTTP request header name.
	// @Scope GLOBAL
	InHeader bool `yaml:"in_header,omitempty"`

	inQuerySet  bool
	inHeaderSet bool

	// @Title 调用方列表
	// @Title en-US Consumer List
	// @Description 服务调用方列表，用于对请求进行认证。
	// @Description en-US List of service consumers which will be used in request authentication.
	// @Scope GLOBAL
	consumers []Consumer `yaml:"consumers"`

	// @Title 授权访问的调用方列表
	// @Title en-US Allowed Consumers
	// @Description 对于匹配上述条件的请求，允许访问的调用方列表。
	// @Description en-US Consumers to be allowed for matched requests.
	allow           []string `yaml:"allow"`
	allowConfigured bool

	// @Title 认证失败响应 Realm
	// @Title en-US Authentication Failure Response Realm
	// @Description 认证失败时 WWW-Authenticate 响应头中的 realm。
	// @Description en-US The realm in WWW-Authenticate response headers when authentication fails.
	// @Scope GLOBAL
	Realm string `yaml:"realm,omitempty"`

	credentialIdentities map[string]credentialIdentity `yaml:"-"`
}

func parseGlobalConfig(json gjson.Result, global *KeyAuthConfig, log log.Log) error {
	log.Debug("global config")

	// init
	ruleSet = false
	global.Realm = defaultProtectionSpace
	global.credentialIdentities = make(map[string]credentialIdentity)

	// global_auth
	globalAuth := json.Get("global_auth")
	if globalAuth.Exists() {
		ga := globalAuth.Bool()
		global.globalAuth = &ga
	}

	realm := json.Get("realm")
	if realm.Exists() && realm.String() != "" {
		global.Realm = realm.String()
	}

	// keys
	names := json.Get("keys")
	if names.Exists() {
		keys, err := parseStringArray(names, "keys")
		if err != nil {
			return err
		}
		global.Keys = keys
	}

	// in_query and in_header
	in_query := json.Get("in_query")
	in_header := json.Get("in_header")
	if in_query.Exists() {
		global.InQuery = in_query.Bool()
		global.inQuerySet = true
	}
	if in_header.Exists() {
		global.InHeader = in_header.Bool()
		global.inHeaderSet = true
	}

	topLevelCredentials := json.Get("credentials")
	consumers := json.Get("consumers")
	if consumers.Exists() && topLevelCredentials.Exists() {
		return errors.New("consumers and credentials cannot both be configured")
	}
	if !consumers.Exists() && !topLevelCredentials.Exists() {
		return errors.New("consumers or credentials is required")
	}

	if topLevelCredentials.Exists() {
		plan, err := resolveTopLevelExtractionPlan(*global)
		if err != nil {
			return err
		}
		credentials, err := parseCredentials(topLevelCredentials, "credentials")
		if err != nil {
			return err
		}
		for _, credential := range credentials {
			if err := addCredentialIdentity(global.credentialIdentities, credential, credentialIdentity{
				Consumer: false,
				Plan:     plan,
			}); err != nil {
				return err
			}
		}
		if json.Get("allow").Exists() {
			return parseOverrideRuleConfig(json, *global, global, log)
		}
		return nil
	}

	if !consumers.IsArray() {
		return errors.New("consumers must be an array")
	}

	for _, item := range consumers.Array() {
		name := item.Get("name")
		if !name.Exists() || name.String() == "" {
			return errors.New("consumer name is required")
		}

		consumer := Consumer{
			Name:   name.String(),
			Tenant: item.Get("tenant").String(),
		}
		if keys := item.Get("keys"); keys.Exists() {
			parsedKeys, err := parseStringArray(keys, "consumer keys")
			if err != nil {
				return err
			}
			consumer.Keys = parsedKeys
		}
		if inHeader := item.Get("in_header"); inHeader.Exists() {
			value := inHeader.Bool()
			consumer.InHeader = &value
		}
		if inQuery := item.Get("in_query"); inQuery.Exists() {
			value := inQuery.Bool()
			consumer.InQuery = &value
		}
		credentials, err := parseConsumerCredentials(item, &consumer)
		if err != nil {
			return err
		}
		plan, err := resolveConsumerExtractionPlan(*global, consumer)
		if err != nil {
			return err
		}

		global.consumers = append(global.consumers, consumer)
		for _, credential := range credentials {
			if err := addCredentialIdentity(global.credentialIdentities, credential, credentialIdentity{
				Name:     consumer.Name,
				Tenant:   consumer.Tenant,
				Consumer: true,
				Plan:     plan,
			}); err != nil {
				return err
			}
		}
	}
	// Higress can merge a matched rule into the top-level plugin configuration.
	if json.Get("allow").Exists() {
		return parseOverrideRuleConfig(json, *global, global, log)
	}
	return nil
}

func parseOverrideRuleConfig(json gjson.Result, global KeyAuthConfig, config *KeyAuthConfig, log log.Log) error {
	log.Debug("domain/route config")

	*config = global

	allow := json.Get("allow")
	if !allow.Exists() {
		return errors.New("allow is required")
	}
	if !allow.IsArray() {
		return errors.New("allow must be an array")
	}
	// An explicit empty rule denies every consumer; it must not become a public route.
	config.allowConfigured = true
	config.allow = nil

	for _, item := range allow.Array() {
		config.allow = append(config.allow, item.String())
	}
	ruleSet = true

	return nil
}

// key-auth 插件认证逻辑：
// - global_auth == true 开启全局生效：
//   - 若当前 domain/route 未配置 allow 列表，即未配置该插件：则在所有 consumers 中查找，如果找到则认证通过，否则认证失败 (1*)
//   - 若当前 domain/route 配置了该插件：则在 allow 列表中查找，如果找到则认证通过，否则认证失败
//
// - global_auth == false 非全局生效：(2*)
//   - 若当前 domain/route 未配置该插件：则直接放行
//   - 若当前 domain/route 配置了该插件：则在 allow 列表中查找，如果找到则认证通过，否则认证失败
//
// - global_auth 未设置：
//   - 若没有一个 domain/route 配置该插件：则遵循 (1*)
//   - 若有至少一个 domain/route 配置该插件：则遵循 (2*)
//
// 显式配置 allow: [] 仍启用认证，并拒绝所有调用方。
func onHttpRequestHeaders(ctx wrapper.HttpContext, config KeyAuthConfig, log log.Log) types.Action {
	var (
		noAllow            = !config.allowConfigured // No matching rule leaves scoped authentication disabled.
		globalAuthNoSet    = config.globalAuth == nil
		globalAuthSetTrue  = !globalAuthNoSet && *config.globalAuth
		globalAuthSetFalse = !globalAuthNoSet && !*config.globalAuth
	)
	// 不需要认证而直接放行的情况：
	// - global_auth == false 且 当前 domain/route 未配置该插件
	// - global_auth 未设置 且 有至少一个 domain/route 配置该插件 且 当前 domain/route 未配置该插件
	if globalAuthSetFalse || (globalAuthNoSet && ruleSet) {
		if noAllow {
			log.Info("authorization is not required")
			return types.ActionContinue
		}
	}

	// 以下需要认证：
	// - 从 header 中获取 tokens 信息
	// - 从 query 中获取 tokens 信息
	candidates := extractCredentialCandidates(config)
	candidatesByValue := groupCredentialCandidatesByValue(candidates)

	// header/query
	if len(candidatesByValue) > 1 {
		return deniedMultiKeyAuthData(config.Realm)
	} else if len(candidatesByValue) <= 0 {
		return deniedNoKeyAuthData(config.Realm)
	}
	credential, sameValueCandidates := singleCredentialCandidateGroup(candidatesByValue)

	// 验证token
	identity, ok := config.credentialIdentities[credential]
	if !ok {
		log.Warnf("credential %q is not configured", credential)
		return deniedUnauthorizedConsumer(config.Realm)
	}
	if !hasAllowedCredentialCandidate(sameValueCandidates, identity.Plan) {
		log.Warnf("credential %q is not allowed from configured credential sources", credential)
		return deniedUnauthorizedConsumer(config.Realm)
	}
	if !identity.Consumer {
		removeTrustedIdentityHeaders()
		if !noAllow {
			return deniedUnauthorizedConsumer(config.Realm)
		}
		return authenticated("")
	}

	name := identity.Name
	propagateTrustedIdentity(identity)

	// 全局生效：
	// - global_auth == true 且 当前 domain/route 未配置该插件
	// - global_auth 未设置 且 没有任何一个 domain/route 配置该插件
	if (globalAuthSetTrue && noAllow) || (globalAuthNoSet && !ruleSet) {
		log.Infof("consumer %q authenticated", name)
		return authenticated(name)
	}

	// 全局生效，但当前 domain/route 配置了 allow 列表
	if globalAuthSetTrue && !noAllow {
		if !contains(config.allow, name) {
			log.Warnf("consumer %q is not allowed", name)
			return deniedUnauthorizedConsumer(config.Realm)
		}
		log.Infof("consumer %q authenticated", name)
		return authenticated(name)
	}

	// 非全局生效
	if globalAuthSetFalse || (globalAuthNoSet && ruleSet) {
		if !noAllow { // 配置了 allow 列表
			if !contains(config.allow, name) {
				log.Warnf("consumer %q is not allowed", name)
				return deniedUnauthorizedConsumer(config.Realm)
			}
			log.Infof("consumer %q authenticated", name)
			return authenticated(name)
		}
	}

	return types.ActionContinue
}

func parseStringArray(result gjson.Result, field string) ([]string, error) {
	items := result.Array()
	if len(items) == 0 {
		return nil, errors.New(field + " cannot be empty")
	}
	values := make([]string, 0, len(items))
	for _, item := range items {
		if item.String() == "" {
			return nil, errors.New(field + " cannot contain empty item")
		}
		values = append(values, item.String())
	}
	return values, nil
}

func parseCredentials(result gjson.Result, field string) ([]string, error) {
	values, err := parseStringArray(result, field)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			return nil, errors.New("duplicate consumer credential: " + value)
		}
		seen[value] = struct{}{}
	}
	return values, nil
}

func parseConsumerCredentials(item gjson.Result, consumer *Consumer) ([]string, error) {
	credential := item.Get("credential")
	credentials := item.Get("credentials")
	if credential.Exists() && credentials.Exists() {
		return nil, errors.New("consumer credential and credentials cannot both be configured")
	}
	if credential.Exists() {
		if credential.String() == "" {
			return nil, errors.New("consumer credential is required")
		}
		consumer.Credential = credential.String()
		return []string{credential.String()}, nil
	}
	if credentials.Exists() {
		parsedCredentials, err := parseCredentials(credentials, "consumer credentials")
		if err != nil {
			return nil, err
		}
		consumer.Credentials = parsedCredentials
		return parsedCredentials, nil
	}
	return nil, errors.New("consumer credential is required")
}

func addCredentialIdentity(lookup map[string]credentialIdentity, credential string, identity credentialIdentity) error {
	if _, ok := lookup[credential]; ok {
		return errors.New("duplicate consumer credential: " + credential)
	}
	lookup[credential] = identity
	return nil
}

func resolveTopLevelExtractionPlan(config KeyAuthConfig) (extractionPlan, error) {
	if len(config.Keys) == 0 {
		return extractionPlan{}, errors.New("keys is required")
	}
	plan := extractionPlan{
		Keys:     append([]string(nil), config.Keys...),
		InHeader: config.inHeaderSet && config.InHeader,
		InQuery:  config.inQuerySet && config.InQuery,
	}
	if !plan.InHeader && !plan.InQuery {
		return extractionPlan{}, errors.New("must one of in_query/in_header required")
	}
	return plan, nil
}

func resolveConsumerExtractionPlan(global KeyAuthConfig, consumer Consumer) (extractionPlan, error) {
	keys := consumer.Keys
	if len(keys) == 0 {
		keys = global.Keys
	}
	if len(keys) == 0 {
		return extractionPlan{}, errors.New("keys is required")
	}

	inHeader := global.inHeaderSet && global.InHeader
	inQuery := global.inQuerySet && global.InQuery
	if consumer.InHeader != nil {
		inHeader = *consumer.InHeader
	}
	if consumer.InQuery != nil {
		inQuery = *consumer.InQuery
	}

	if !inHeader && !inQuery {
		return extractionPlan{}, errors.New("must one of in_query/in_header required")
	}

	return extractionPlan{
		Keys:     append([]string(nil), keys...),
		InHeader: inHeader,
		InQuery:  inQuery,
	}, nil
}

func extractCredentialCandidates(config KeyAuthConfig) []credentialCandidate {
	plans := uniqueExtractionPlans(config)
	seen := make(map[string]struct{})
	var candidates []credentialCandidate
	for _, plan := range plans {
		if plan.InHeader {
			candidates = append(candidates, extractHeaderCredentialCandidates(plan, seen)...)
		}
		if plan.InQuery {
			candidates = append(candidates, extractQueryCredentialCandidates(plan, seen)...)
		}
	}
	return candidates
}

func groupCredentialCandidatesByValue(candidates []credentialCandidate) map[string][]credentialCandidate {
	groups := make(map[string][]credentialCandidate)
	for _, candidate := range candidates {
		groups[candidate.Value] = append(groups[candidate.Value], candidate)
	}
	return groups
}

func singleCredentialCandidateGroup(groups map[string][]credentialCandidate) (string, []credentialCandidate) {
	for value, candidates := range groups {
		return value, candidates
	}
	return "", nil
}

func uniqueExtractionPlans(config KeyAuthConfig) []extractionPlan {
	seen := make(map[string]struct{})
	var plans []extractionPlan
	for _, identity := range config.credentialIdentities {
		key := identity.Plan.key()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		plans = append(plans, identity.Plan)
	}
	return plans
}

func (plan extractionPlan) key() string {
	return fmt.Sprintf("%t|%t|%s", plan.InHeader, plan.InQuery, strings.Join(plan.Keys, "\x00"))
}

func extractHeaderCredentialCandidates(plan extractionPlan, seen map[string]struct{}) []credentialCandidate {
	var candidates []credentialCandidate
	for _, key := range plan.Keys {
		value, err := proxywasm.GetHttpRequestHeader(key)
		if err != nil || value == "" {
			continue
		}
		value = normalizeHeaderCredential(key, value)
		if value == "" {
			continue
		}
		candidates = appendCredentialCandidate(candidates, seen, credentialCandidate{
			Value:   value,
			Source:  "header",
			Key:     key,
			Ordinal: 0,
		})
	}
	return candidates
}

func extractQueryCredentialCandidates(plan extractionPlan, seen map[string]struct{}) []credentialCandidate {
	requestUrl, _ := proxywasm.GetHttpRequestHeader(":path")
	parsedURL, err := url.Parse(requestUrl)
	if err != nil {
		return nil
	}
	queryValues := parsedURL.Query()
	var candidates []credentialCandidate
	for _, key := range plan.Keys {
		values, ok := queryValues[key]
		if !ok {
			continue
		}
		for index, value := range values {
			if value == "" {
				continue
			}
			candidates = appendCredentialCandidate(candidates, seen, credentialCandidate{
				Value:   value,
				Source:  "query",
				Key:     key,
				Ordinal: index,
			})
		}
	}
	return candidates
}

func appendCredentialCandidate(candidates []credentialCandidate, seen map[string]struct{}, candidate credentialCandidate) []credentialCandidate {
	key := fmt.Sprintf("%s\x00%s\x00%d\x00%s", candidate.Source, candidate.Key, candidate.Ordinal, candidate.Value)
	if _, ok := seen[key]; ok {
		return candidates
	}
	seen[key] = struct{}{}
	return append(candidates, candidate)
}

func normalizeHeaderCredential(key string, value string) string {
	if strings.EqualFold(key, "Authorization") && strings.HasPrefix(value, "Bearer ") {
		return strings.TrimPrefix(value, "Bearer ")
	}
	return value
}

func hasAllowedCredentialCandidate(candidates []credentialCandidate, plan extractionPlan) bool {
	for _, candidate := range candidates {
		if candidateAllowedByPlan(candidate, plan) {
			return true
		}
	}
	return false
}

func candidateAllowedByPlan(candidate credentialCandidate, plan extractionPlan) bool {
	if candidate.Source == "header" && !plan.InHeader {
		return false
	}
	if candidate.Source == "query" && !plan.InQuery {
		return false
	}
	return contains(plan.Keys, candidate.Key)
}

func propagateTrustedIdentity(identity credentialIdentity) {
	removeTrustedIdentityHeaders()
	_ = proxywasm.AddHttpRequestHeader(consumerHeaderName, identity.Name)
	if identity.Tenant != "" {
		_ = proxywasm.AddHttpRequestHeader(tenantHeaderName, identity.Tenant)
	}
}

func removeTrustedIdentityHeaders() {
	_ = proxywasm.RemoveHttpRequestHeader(consumerHeaderName)
	_ = proxywasm.RemoveHttpRequestHeader(tenantHeaderName)
}

func deniedMultiKeyAuthData(realm string) types.Action {
	_ = proxywasm.SendHttpResponseWithDetail(http.StatusUnauthorized, "key-auth.multi_key", WWWAuthenticateHeader(realm),
		[]byte("Request denied by Key Auth check. Multiple distinct API keys found."), -1)
	return types.ActionContinue
}

func deniedNoKeyAuthData(realm string) types.Action {
	_ = proxywasm.SendHttpResponseWithDetail(http.StatusUnauthorized, "key-auth.no_key", WWWAuthenticateHeader(realm),
		[]byte("Request denied by Key Auth check. No Key Authentication information found."), -1)
	return types.ActionContinue
}

func deniedUnauthorizedConsumer(realm string) types.Action {
	_ = proxywasm.SendHttpResponseWithDetail(http.StatusForbidden, "key-auth.unauthorized", WWWAuthenticateHeader(realm),
		[]byte("Request denied by Key Auth check. Unauthorized consumer."), -1)
	return types.ActionContinue
}

func authenticated(name string) types.Action {
	return types.ActionContinue
}

func contains(arr []string, item string) bool {
	for _, i := range arr {
		if i == item {
			return true
		}
	}
	return false
}

func WWWAuthenticateHeader(realm string) [][2]string {
	if realm == "" {
		realm = defaultProtectionSpace
	}
	return [][2]string{
		{"WWW-Authenticate", fmt.Sprintf("Key realm=%s", realm)},
	}
}
