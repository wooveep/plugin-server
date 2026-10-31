package main

import (
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/resp"
)

const balanceProjectionSchema = "balance.v1"

type balanceNamespace struct {
	ScopeID    string
	Generation string
}

func (n balanceNamespace) validate() error {
	if !balanceidentity.ValidID(n.ScopeID) || !balanceidentity.ValidID(n.Generation) {
		return errors.New("balance_scope_id and cache_generation require canonical non-nil UUIDs")
	}
	return nil
}

func (n balanceNamespace) keys(identity balanceidentity.Snapshot) []interface{} {
	base := "mf:balance:{" + n.ScopeID + "}"
	account := base + ":account:" + identity.AccountID
	return []interface{}{base + ":control", account, account + ":principal:key:" + identity.APIKeyID}
}

var legacyBalanceFields = []string{"balance_key_template", "tenant_header", "consumer_header", "quota_scope", "provider"}

func rejectLegacyBalanceConfig(config gjson.Result) error {
	for _, field := range legacyBalanceFields {
		if config.Get(field).Exists() {
			return errors.New(field + " is not supported by versioned balance admission")
		}
	}
	if policy := config.Get("missing_balance_policy"); policy.Exists() && (policy.Type != gjson.String || policy.String() != "deny") {
		return errors.New("missing_balance_policy must be deny")
	}
	return nil
}

func parseBalanceNamespace(config gjson.Result) (balanceNamespace, error) {
	if err := rejectLegacyBalanceConfig(config); err != nil {
		return balanceNamespace{}, err
	}
	for _, field := range []string{"balance_scope_id", "cache_generation", "projection_schema"} {
		if config.Get(field).Type != gjson.String {
			return balanceNamespace{}, errors.New(field + " is required as a string")
		}
	}
	if config.Get("projection_schema").String() != balanceProjectionSchema {
		return balanceNamespace{}, errors.New("projection_schema must be balance.v1")
	}
	namespace := balanceNamespace{config.Get("balance_scope_id").String(), config.Get("cache_generation").String()}
	return namespace, namespace.validate()
}

// This script only reads three keys in the same Redis Cluster slot. Versions and
// signed bigint money stay strings; no float conversion or money mutation occurs.
const balanceAdmissionScript = `
local function unsigned(value, maximum)
    if type(value) ~= 'string' or value == '' or string.find(value, '[^0-9]') then return false end
    if #value > 1 and string.sub(value, 1, 1) == '0' then return false end
    return #value < #maximum or (#value == #maximum and value <= maximum)
end
local function signed(value)
    if type(value) ~= 'string' then return false end
    if string.sub(value, 1, 1) == '-' then
        local magnitude = string.sub(value, 2)
        return magnitude ~= '0' and unsigned(magnitude, '9223372036854775808')
    end
    return unsigned(value, '9223372036854775807')
end
local function deny(reason)
    return cjson.encode({decision='deny', reason_code=reason})
end
local control = redis.call('HMGET', KEYS[1], 'scope_id', 'cache_generation', 'projection_schema', 'projection_state')
if not control[1] or not control[2] then return deny('projection_unready') end
if control[1] ~= ARGV[1] then return deny('account_mismatch') end
if control[3] ~= 'balance.v1' then return deny('projection_schema_mismatch') end
if control[4] ~= 'ready' then return deny('projection_unready') end
if redis.call('EXISTS', KEYS[2]) == 0 then return deny('balance_missing') end
local account = redis.call('HMGET', KEYS[2], 'account_id', 'cache_generation', 'projection_schema',
    'projection_state', 'account_version', 'amount', 'currency', 'scale',
    'mapping_revision', 'admission_state_revision', 'credential_generation')
if account[1] ~= ARGV[3] then return deny('account_mismatch') end
if account[2] ~= control[2] then return deny('wrong_generation') end
if account[3] ~= 'balance.v1' then return deny('projection_schema_mismatch') end
if account[4] ~= 'ready' then return deny('projection_unready') end
if not unsigned(account[5], '9223372036854775807') then return deny('projection_unready') end
if not signed(account[6]) or account[7] ~= 'CNY' or account[8] ~= '10' then return deny('invalid_amount') end
if not unsigned(account[9], '9223372036854775807')
    or not unsigned(account[10], '9223372036854775807')
    or not unsigned(account[11], '9223372036854775807') or account[11] == '0' then
    return deny('authorization_stale')
end
local mapping = redis.call('HMGET', KEYS[3], 'account_id', 'cache_generation', 'projection_schema',
    'projection_state', 'mapping_revision', 'admission_state_revision', 'credential_generation', 'enabled', 'authorized_routes')
if mapping[1] ~= ARGV[3] then return deny('authorization_stale') end
if mapping[2] ~= control[2] then return deny('wrong_generation') end
if mapping[3] ~= 'balance.v1' then return deny('projection_schema_mismatch') end
if mapping[4] ~= 'ready' then return deny('projection_unready') end
if mapping[5] ~= account[9] or mapping[6] ~= account[10]
    or mapping[7] ~= account[11] or mapping[7] ~= ARGV[4] or mapping[8] ~= '1' then
    return deny('authorization_stale')
end
-- Managed routes use the exact Key's grant list from the same atomic identity
-- snapshot. A missing list is an unready projection; an empty list is a denial.
if ARGV[5] and ARGV[5] ~= '' then
    if type(mapping[9]) ~= 'string' or string.sub(mapping[9], 1, 1) ~= '[' then return deny('projection_unready') end
    local valid, routes = pcall(cjson.decode, mapping[9])
    if not valid or type(routes) ~= 'table' then return deny('projection_unready') end
    local authorized = false
    for _, route in ipairs(routes) do
        if type(route) ~= 'string' then return deny('projection_unready') end
        if route == ARGV[5] then authorized = true end
    end
    if not authorized then return deny('route_authorization_missing') end
end
local result = {decision='allow', reason_code='ok', observed_account_version=account[5], observed_generation=control[2]}
if account[6] == '0' or string.sub(account[6], 1, 1) == '-' then
    result.decision = 'deny'
    result.reason_code = 'insufficient_balance'
end
return cjson.encode(result)
`

// balanceAdmission is the frozen IF05 result, with string observations preserving
// exact bigint values. It describes one read and never claims current PG equality.
type balanceAdmission struct {
	Decision               string `json:"decision"`
	ReasonCode             string `json:"reason_code"`
	ObservedAccountVersion string `json:"observed_account_version,omitempty"`
	ObservedGeneration     string `json:"observed_generation,omitempty"`
}

func canonicalVersion(value string) bool {
	version, err := strconv.ParseInt(value, 10, 64)
	return err == nil && version >= 0 && strconv.FormatInt(version, 10) == value
}

func decodeBalanceAdmission(response resp.Value, namespace balanceNamespace) (balanceAdmission, error) {
	if err := response.Error(); err != nil {
		return balanceAdmission{}, errors.New("balance projection dependency failed")
	}
	if response.Type() != resp.BulkString && response.Type() != resp.SimpleString {
		return balanceAdmission{}, errors.New("invalid balance projection result type")
	}
	var result balanceAdmission
	decoder := json.NewDecoder(strings.NewReader(response.String()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return result, errors.New("invalid balance projection result")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return result, errors.New("invalid trailing balance projection result")
	}
	if result.Decision == "allow" {
		if result.ReasonCode != "ok" {
			return result, errors.New("invalid allowed projection result")
		}
	} else if result.Decision == "deny" {
		switch result.ReasonCode {
		case "balance_missing", "projection_unready", "wrong_generation", "projection_schema_mismatch",
			"authorization_stale", "route_authorization_missing", "insufficient_balance", "dependency_unavailable", "account_mismatch", "invalid_amount":
		default:
			return result, errors.New("invalid denied projection result")
		}
	} else {
		return result, errors.New("invalid projection decision")
	}
	observed := result.Decision == "allow" || result.ReasonCode == "insufficient_balance"
	if observed || result.ObservedAccountVersion != "" || result.ObservedGeneration != "" {
		if !canonicalVersion(result.ObservedAccountVersion) || !balanceidentity.ValidID(result.ObservedGeneration) {
			return result, errors.New("invalid balance projection observations")
		}
	}
	return result, nil
}

func readBalanceAdmission(client wrapper.RedisClient, namespace balanceNamespace, identity balanceidentity.Snapshot, routeID string, callback func(balanceAdmission, error)) error {
	if err := namespace.validate(); err != nil {
		return err
	}
	if err := identity.Validate(); err != nil {
		return err
	}
	return client.Eval(balanceAdmissionScript, 3, namespace.keys(identity), []interface{}{
		namespace.ScopeID, namespace.Generation, identity.AccountID, identity.CredentialGeneration, routeID,
	}, func(response resp.Value) {
		result, err := decodeBalanceAdmission(response, namespace)
		callback(result, err)
	})
}
