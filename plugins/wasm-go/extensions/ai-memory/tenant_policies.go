package main

import (
	"errors"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-memory/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/ai/tenantpolicy"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const selectedMemoryPolicyKey = "ai-memory.selected-tenant-policy"

func parseTenantMemoryPolicies(input gjson.Result, c *config.PluginConfig, logger log.Log) error {
	localReplyPolicy, err := clientresponse.ParseLocalReplyPolicyField(input)
	if err != nil {
		return err
	}
	if localReplyPolicy != nil && input.Get("client_protocol").Exists() && localReplyPolicy.Protocol != input.Get("client_protocol").String() {
		return errors.New("ai-memory local_reply_policy differs from client_protocol")
	}
	directory, err := tenantpolicy.Parse(input, func(policy gjson.Result) (config.PluginConfig, error) {
		var selected config.PluginConfig
		err := parseOverrideConfig(policy, config.PluginConfig{}, &selected, logger)
		if err == nil && selected.LocalReplyPolicy == nil {
			selected.LocalReplyPolicy = localReplyPolicy
		}
		return selected, err
	})
	if err != nil {
		return err
	}
	// The common carrier owns no policy, Redis client, or materialized prefix.
	// It retains body capture even for an admitted tenant without memory.
	*c = config.PluginConfig{
		RouterContinuation: input.Get("router_continuation").Bool(),
		TenantPolicies:     directory,
		CaptureBodyBasis:   directory.CaptureBodyBasis,
		PathScope:          config.PathScopeBoundTarget,
		LocalReplyPolicy:   localReplyPolicy,
		Route:              config.RouteConfig{MemoryMode: config.MemoryModeOff},
	}
	return nil
}

func selectedMemoryPolicy(ctx wrapper.HttpContext, c config.PluginConfig) config.PluginConfig {
	if selected, ok := ctx.GetContext(selectedMemoryPolicyKey).(*config.PluginConfig); ok {
		return *selected
	}
	if c.TenantPolicies == nil {
		return c
	}
	headers, err := proxywasm.GetHttpRequestHeaders()
	selected, found := c.TenantPolicies.Select(headers)
	if err == nil && found {
		c = selected
	}
	ctx.SetContext(selectedMemoryPolicyKey, &c)
	return c
}
