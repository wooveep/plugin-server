---
title: AI Billing Events
keywords: [AI Gateway, AI Billing, ai-billing]
description: Configuration reference for request-level AI billing event delivery.
---

## Overview

`ai-billing` parses token usage and model independently after an enabled AI response completes, builds a customer usage billing event by default, and writes it to Console/billing-service settlement through Redis Streams. Internal AI Routes can set `event_kind=internal_cost` so events are used only for platform cost attribution and not for ordinary customer statements. The plugin uses Redis `XADD <stream> * event <BillingEvent JSON>` and defaults to `billing:events`. Delivery is fail-open by default: Redis timeouts, network failures, and error responses are logged but do not block the user response.

The plugin does not calculate final authoritative costs, apply tenant discounts, apply override prices, deduct Redis balances, or update account databases. Idempotency, settlement, statements, balance projection, and reconciliation belong to Console/billing-service.

## Example

```yaml
defaultConfig:
  redis_stream:
    service_name: redis-stack-server.dns
    service_port: 6379
    database: 0
    timeout: 500
    stream: billing:events
  quota_scope: global
  event_kind: usage
  provider: default
  tenant_header: x-mse-tenant
  consumer_header: x-mse-consumer
  enable_path_suffixes:
    - /v1/chat/completions
    - /v1/messages
  fail_policy: open
matchRules:
  - ingress:
      - qwen
    config:
      quota_scope: route:qwen
      provider: dashscope
  - ingress:
      - ai-memory-digest
    config:
      event_kind: internal_cost
      quota_scope: internal:ai-memory.digest
      provider: dashscope
```

Events include `event_id`, `event_kind`, `idempotency_key`, `request_id`, `tenant`, `consumer`, `quota_scope`, `route`, `provider`, `model`, `cluster`, request path, status code, timing, stream flag, `usage`, `usage_missing`, and optional price version.

When provider usage exposes cache details, `usage` also includes `input_cache_hit_tokens`, `input_cache_miss_tokens`, and `output_tokens`. Legacy `usage.input`, `usage.output`, and `usage.total` stay present for compatibility.

`route`, `provider`, and `model` use Console-native object facts: `{ "id"?: "...", "name"?: "..." }`. Gateways usually do not know Console UUIDs, so the plugin populates runtime `name` values by default.

For billing-enabled AI requests, the plugin overwrites `x-ocmf-gateway-request-id` on the response with the gateway's final `x-request-id`. This value exactly matches the event `request_id`, allowing Console and UI billing correlation; a same-named header supplied by the upstream provider is not retained. Paths where billing is not enabled do not expose this correlation header. The response header identifies the gateway request but does not by itself prove that the Redis event has been persisted.

Configuration fields:

| Name | Type | Default | Description |
| --- | --- | --- | --- |
| `redis_stream` | object | none | Redis Stream delivery target |
| `redis_stream.service_name` | string | none | Redis service name or Console-managed registry name |
| `redis_stream.service_port` | integer | `6379` | Redis service port |
| `redis_stream.username` | string | empty | Redis username |
| `redis_stream.password` | string | empty | Redis password; treat as sensitive |
| `redis_stream.database` | integer | `0` | Redis database |
| `redis_stream.timeout` | integer | `500` | Redis callout timeout in milliseconds |
| `redis_stream.stream` | string | `billing:events` | Redis Stream name |
| `event_kind` | string | `usage` | Event type: `usage` or `internal_cost` |
| `quota_scope` | string | `global` | Quota scope for the current route or rule |
| `provider` | string | `default` | AI provider identifier |
| `tenant_header` | string | `x-mse-tenant` | Tenant identity request header |
| `consumer_header` | string | `x-mse-consumer` | Consumer identity request header |
| `enable_path_suffixes` | []string | `/v1/chat/completions`, `/v1/messages` | Enabled path suffixes |
| `fail_policy` | string | `open` | Delivery failure policy; currently only `open` is supported |

The global `defaultConfig` must contain `redis_stream.service_name`, and should hold shared tenant, consumer, path suffix, and fail-open defaults. `matchRules[].config` can contain only route-specific differences such as `event_kind`, `provider`, `quota_scope`, `enable_path_suffixes`, or `fail_policy`; omitted fields inherit from the global config. Rule-level `redis_stream` is not supported because the Redis Stream target is shared plugin-instance configuration.

Platform-owned internal AI Routes, such as `ai-cache.embedding` and `ai-memory.digest`, should set `event_kind=internal_cost` plus an internal `quota_scope`. These events still carry usage, provider, model, and route facts so Console/billing-service can calculate platform costs, but downstream billing must not turn them into ordinary customer usage statements.

### Trusted internal-cost attribution headers

For a request that matches an `internal_cost` rule, a trusted internal caller may provide both `x-ai-billing-cost-source` and `x-ai-billing-worker-kind`. The plugin uses the pair for that request and otherwise falls back to the route configuration. A partial or invalid pair suppresses billing-event delivery for that request without blocking model traffic. These headers cannot enable `internal_cost` mode or set `bill_customer`.

Console-managed `ai-billing` configuration projects the Redis service to an McpBridge registry name, such as `redis-stack-server.dns`. The old `billing_service`, `path`, and `auth_token` configuration has been removed; when upgrading from the HTTP transport, delete and recreate existing HTTP-shaped `WasmPlugin` objects or reproject Console-managed bindings.

`ai-billing`, `ai-quota`, and `ai-statistics` can be deployed independently. Balance debit and Redis projection refresh remain owned by Console/billing-service.
