package main

import (
	"encoding/json"
	"fmt"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/ai/tenantpolicy"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const selectedCachePolicyKey = "ai-cache.selected-tenant-policy"

func parseTenantCachePolicies(input gjson.Result, c *config.PluginConfig, logger log.Log) error {
	directory, err := tenantpolicy.Parse(input, func(policy gjson.Result) (config.PluginConfig, error) {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(policy.Raw), &fields); err != nil {
			return config.PluginConfig{}, fmt.Errorf("cache tenant configuration: %w", err)
		}
		// The SDK injects one target envelope. Every tenant policy shares that
		// exact envelope without converting integer identities through float64.
		fields["_modelfusion_execution"] = json.RawMessage(input.Get("_modelfusion_execution").Raw)
		raw, err := json.Marshal(fields)
		if err != nil {
			return config.PluginConfig{}, fmt.Errorf("cache tenant execution envelope: %w", err)
		}
		var selected config.PluginConfig
		err = parseConfig(gjson.ParseBytes(raw), &selected, logger)
		return selected, err
	})
	if err != nil {
		return err
	}
	*c = config.PluginConfig{TenantPolicies: directory, PublishedProtocolConfigured: true, PublishedProtocol: directory.Protocol}
	return configureNativeCacheDelivery(input, c)
}

func selectedCachePolicy(ctx wrapper.HttpContext, c config.PluginConfig) config.PluginConfig {
	if selected, ok := ctx.GetContext(selectedCachePolicyKey).(*config.PluginConfig); ok {
		return *selected
	}
	if c.TenantPolicies == nil {
		return c
	}
	headers, err := proxywasm.GetHttpRequestHeaders()
	selected, found := c.TenantPolicies.Select(headers)
	if err == nil && found {
		c = selected
	} else {
		markCacheGate(ctx, "no-tenant-policy")
	}
	ctx.SetContext(selectedCachePolicyKey, &c)
	return c
}
