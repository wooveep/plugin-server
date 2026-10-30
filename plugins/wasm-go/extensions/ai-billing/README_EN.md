---
title: AI Billing Events
keywords: [AI Gateway, AI Billing, ai-billing]
description: Configuration reference for request-level AI billing event delivery.
---

## Frozen Credential Directory

The `0.1.11` configuration contract keeps `_modelfusion_execution` v1's scalar `credential_ref`/`credential_revision` and adds `modelfusion-execution.v2` with a complete, nonempty `credentials` array. Each member contains only those two fields; references are unique, revisions are canonical positive decimal strings, and secrets or mixed scalar/array shapes are rejected. The directory is not a selected attempt: actual D0/FACT attribution still contains one selected credential tuple. Billing and cache never select the first directory member.

> Plugin version: `1.0.1`

## Overview

`ai-billing` creates one request-level event when an enabled AI request completes or its stream terminates, then runs `XADD <stream> * event <BillingEvent JSON>` through the Redis Stream target selected by its valid global or scoped configuration. Delivery is fail-open: Redis initialization, network, timeout, or command errors do not block the model response. The plugin does not calculate final authoritative cost, apply discounts, debit balances, or update account databases. Idempotent settlement, statements, balance projection, and reconciliation belong to Console/billing-service.

## Billing event contract

The complete event surface includes `event_id`, `event_kind`, `cost_source`, `worker_kind`, `source_job`, `bill_customer`, `idempotency_key`, `request_id`, `tenant`, `consumer`, `route`, `provider`, `quota_scope`, `model`, `request_path`, `status_code`, `usage`, `usage_missing`, `usage_source`, `start_time_ms`, `end_time_ms`, `is_stream`, `cluster`, optional `price_version`, and the trusted gateway lifecycle fact `upstream_invoked`. Request, response, or user payload cannot spoof that fact. `event_id` is created at request start, defaults `idempotency_key` to the same value, and remains stable across retries.

The canonical event kinds are `usage` and `internal_cost`. Legacy `customer_usage` is accepted only as a compatibility alias and normalizes to `usage`. `cost_source`, `worker_kind`, `source_job`, and `bill_customer` are internal attribution fields on `internal_cost`; ordinary `usage` events omit them. `price_version` is optional and appears only when a trustworthy price fact exists.

Trustworthy provider usage remains authoritative, sets `usage_source=provider` and `usage_missing=false`, and retains the complete provider object in `usage.details.provider_usage`. When provider usage is absent but a recognized structure allows safe estimation, the event uses `usage_source=estimated` with `usage_missing=false` without inventing provider-only detail. When neither source is trustworthy, it uses zero token counts, `usage_missing=true`, and `usage_source=missing`; this does not fabricate model, provider usage, price, cost, or settlement.

`usage` carries canonical `unit=token`, `input`, `output`, and `total` counts plus optional cache detail. The event has no top-level `input_tokens`, `output_tokens`, `total_tokens`, or `gateway_calculated_cost`, and events and logs contain no raw API keys or real Redis credentials. `route`, `provider`, and `model` use `{ "id"?: "...", "name"?: "..." }` fact objects. A binding target or path alone cannot prove model, usage, price, or cost.

## Trusted internal-cost attribution

Only configured `internal_cost` routes are eligible for invocation-level attribution headers. A trusted caller must submit both `x-ai-billing-cost-source` and `x-ai-billing-worker-kind`: a complete pair overrides the configured fallback, while the configured fallback applies when both are absent. A partial, empty, oversized, invalid-character, or duplicate pair suppresses only billing-event delivery; model traffic continues without being blocked.

Accepted and rejected submitted values are never logged. Customer routes ignore these headers for event classification. They cannot enable `internal_cost`, cannot set `bill_customer`, and cannot change configuration-derived customer settlement classification.

## Response correlation contract

For billing-enabled requests, `x-ocmf-gateway-request-id` uses the final canonical request ID and replaces rather than appends any same-named upstream value. It exactly matches the event `request_id` for non-streaming, streaming, gateway-local, and cache responses. Stream completion does not append a second `x-ocmf-gateway-request-id` value.

When canonical request context is missing, the plugin removes the upstream value without inventing one. The response header proves request correlation only and does not prove Redis persistence; Console/billing-service consumption determines persistence.

## Configuration and migration

A global delivery target requires `redis_stream.service_name`. The defaults for `service_port`, `database`, `timeout`, and `stream` are `6379`, `0`, `500`, and `billing:events`. Credential examples use only the `<redis-username>` and `<redis-password>` placeholders.

```yaml
defaultConfig:
  redis_stream:
    service_name: redis-stack-server.dns
    service_port: 6379
    username: <redis-username>
    password: <redis-password>
    database: 0
    timeout: 500
    stream: billing:events
  event_kind: usage
  quota_scope: global
  provider: default
  tenant_header: x-mse-tenant
  consumer_header: x-mse-consumer
  path_scope: legacy_suffixes
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
      path_scope: bound_target
  - ingress:
      - ai-memory-digest
    config:
      event_kind: internal_cost
      quota_scope: internal:ai-memory.digest
      provider: dashscope
      cost_source: ai-memory
      worker_kind: digest
      bill_customer: false
      path_scope: bound_target
```

For route-scoped-only delivery, leave the base empty, set `defaultConfigDisable: true`, and place a complete Redis target in every active match rule:

```yaml
defaultConfigDisable: true
matchRules:
  - ingress:
      - qwen
    config:
      redis_stream:
        service_name: redis-stack-server.dns
        service_port: 6379
        username: <redis-username>
        password: <redis-password>
        database: 0
        timeout: 500
        stream: billing:events
      event_kind: usage
      quota_scope: route:qwen
      provider: dashscope
      path_scope: bound_target
```

The empty base starts in a dormant no-op state, so unmatched traffic does not read bodies, initialize billing state, or emit events. Any non-empty base still requires a valid global `redis_stream.service_name`.

Global and route configuration support `event_kind`, `quota_scope`, `provider`, `cost_source`, `worker_kind`, `bill_customer`, `tenant_header`, `consumer_header`, `path_scope`, `enable_path_suffixes`, `enable_methods`, and `fail_policy`. With a valid global Redis target, omitted route fields inherit global values and a rule cannot replace that target. With a dormant base, each active rule must be complete, including `redis_stream.service_name`.

Missing or explicit `legacy_suffixes` retains exactly the two effective legacy suffixes `/v1/chat/completions` and `/v1/messages`. With `bound_target`, the Higress binding defines the request set, and omitted or literal-empty filters add no narrowing. For advanced narrowing, non-empty suffix and method filters form an exact intersection with the binding target and cannot expand it; the following example intersects `/v1/responses` with `POST`:

```yaml
path_scope: bound_target
enable_path_suffixes:
  - /v1/responses
enable_methods:
  - POST
```

The old HTTP-shaped transport is removed from the current configuration surface. Global and rule-level `billing_service` are rejected. Rule-level `redis_stream` is accepted only with a dormant empty base; it cannot replace an existing global target. To upgrade old objects, remove `billing_service`, `path`, and `auth_token`, then recreate the `WasmPlugin` or let Console reproject the managed binding.

## Independent deployment boundary

`ai-billing`, `ai-quota`, and `ai-statistics` can be deployed independently. `ai-billing` only publishes billing events; the plugins do not depend on or share one another's private runtime state. Balance admission and observability stay within their respective plugins, while settlement and persistence belong to Console/billing-service.
