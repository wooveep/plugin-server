---
title: AI Monetary Quota
keywords: [AI Gateway, AI Quota, Monetary Quota]
description: Resource reference for read-only Redis balance admission.
---

> Plugin version: `1.0.2`

## Overview

`ai-quota` reads Console- or billing-service-derived Redis hot balance during
request-header processing. A balance greater than `0` allows the request.
Zero and negative balances return the fixed `403 no_balance` response. A
missing key follows `missing_balance_policy`.

This request admission is the plugin's final account enforcement action for
the request. Redis balance hot data is read-only input to the plugin, which
does not mutate account state. Console or billing-service owns authoritative
accounts, settlement, statements, reconciliation, and Redis hot-data rebuilds.

Recognizable, opaque, streaming, cache, local, error, and missing-usage
responses do not trigger another quota decision, price lookup, Redis write, or
account mutation.

## Runtime properties

Plugin execution phase: `default phase`

Plugin execution priority: `280`

## Request scope

- Missing or explicit `path_scope: legacy_suffixes` preserves legacy suffix
  gating. The default suffixes are `/v1/chat/completions` and `/v1/messages`.
- `path_scope: bound_target` processes every request already selected by the
  Higress binding target. Omitted or literal-empty filters add no narrowing.
- Explicitly non-empty `enable_path_suffixes` and `enable_methods` are
  advanced intersection filters and cannot expand the binding target.
  `enable_methods` uses exact-case matching; examples use canonical uppercase.

## Configuration fields

Global configuration supports `redis`, `quota_scope`, `provider`,
`tenant_header`, `consumer_header`, `balance_key_template`, `path_scope`,
`enable_path_suffixes`, `enable_methods`, and `missing_balance_policy`.

`redis` supports `service_name`, `service_port`, `username`, `password`,
`timeout`, and `database`; `service_name` is required.

Rules override only `quota_scope`, `provider`, `path_scope`,
`enable_path_suffixes`, and `enable_methods`. All other admission
infrastructure is inherited from global configuration.

## Example

```yaml
redis:
  service_name: redis-service.default.svc.cluster.local
  service_port: 6379
  username: <redis-username>
  password: <redis-password>
  timeout: 1000
  database: 0
quota_scope: global
provider: default
tenant_header: x-mse-tenant
consumer_header: x-mse-consumer
balance_key_template: "billing:balance:{tenant}:{quota_scope}:{consumer}"
path_scope: legacy_suffixes
enable_path_suffixes:
  - /v1/chat/completions
  - /v1/messages
missing_balance_policy: deny
```

When the binding target already selects a provider-native route and no
additional filtering is needed, use:

```yaml
quota_scope: route:qwen
provider: dashscope
path_scope: bound_target
```

Explicit intersection example:

```yaml
path_scope: bound_target
enable_path_suffixes:
  - /v1/responses
enable_methods:
  - POST
```

`ai-quota`, `ai-billing`, and `ai-statistics` share no private runtime state
and can be deployed independently.
