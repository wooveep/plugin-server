package main

import (
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/ai/tenantpolicy"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const selectedHistoryPolicyKey = "ai-history.selected-tenant-policy"

func parseTenantHistoryPolicies(input gjson.Result, c *PluginConfig, logger log.Log) error {
	directory, err := tenantpolicy.Parse(input, func(policy gjson.Result) (PluginConfig, error) {
		var selected PluginConfig
		err := parseConfig(policy, &selected, logger)
		return selected, err
	})
	if err != nil {
		return err
	}
	*c = PluginConfig{tenantPolicies: directory, historyEnabled: false}
	return nil
}

func selectedHistoryPolicy(ctx wrapper.HttpContext, c PluginConfig) PluginConfig {
	if selected, ok := ctx.GetContext(selectedHistoryPolicyKey).(*PluginConfig); ok {
		return *selected
	}
	if c.tenantPolicies == nil {
		return c
	}
	headers, err := proxywasm.GetHttpRequestHeaders()
	selected, found := c.tenantPolicies.Select(headers)
	if err == nil && found {
		c = selected
	}
	ctx.SetContext(selectedHistoryPolicyKey, &c)
	return c
}
