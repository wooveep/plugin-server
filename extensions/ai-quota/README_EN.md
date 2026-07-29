---
title: AI Monetary Quota
keywords: [AI Gateway, AI Quota, Monetary Quota]
description: Configuration reference for read-only Redis balance admission.
---

## Overview

`ai-quota` reads Console- or billing-service-derived Redis hot balance during
request-header processing. A balance greater than `0` allows the request.
Zero and negative balances return the fixed `403 no_balance` response. A
missing key follows `missing_balance_policy`.

This request admission is the plugin's final account enforcement action for
the request. The plugin treats Redis balance hot data as read-only input and
does not mutate account state. Console or billing-service owns authoritative
accounts, settlement, statements, reconciliation, and Redis hot-data rebuilds.

## Request scope

- Missing or explicit `path_scope: legacy_suffixes` preserves legacy suffix
  gating. The default suffixes are `/v1/chat/completions` and `/v1/messages`.
- `path_scope: bound_target` processes every request already selected by the
  Higress binding target. Omitted or literal-empty filters add no narrowing
  and require no synthesized path list.
- In `bound_target`, explicitly non-empty `enable_path_suffixes` and
  `enable_methods` are advanced intersection filters. When both are supplied,
  both must match; neither filter can expand the binding target.

## Configuration

| Name | Type | Default | Description |
| --- | --- | --- | --- |
| `redis` | object | none | Redis connection; `service_name` is required |
| `quota_scope` | string | `global` | Quota scope for the current route or rule |
| `provider` | string | `default` | Provider identifier for the admission context |
| `tenant_header` | string | `x-mse-tenant` | Tenant identity request header |
| `consumer_header` | string | `x-mse-consumer` | Consumer identity request header |
| `balance_key_template` | string | `billing:balance:{tenant}:{quota_scope}:{consumer}` | Balance key template |
| `path_scope` | string | `legacy_suffixes` | `legacy_suffixes` or `bound_target` |
| `enable_path_suffixes` | []string | legacy defaults | Optional path filter |
| `enable_methods` | []string | none | Optional exact-case HTTP method filter |
| `missing_balance_policy` | string | `deny` | Missing balance policy: `deny` or `allow` |

`redis` supports `service_name`, `service_port`, `username`, `password`,
`timeout`, and `database`. The default port is `6379` (`80` for static
services), the default timeout is `1000ms`, and the default database is `0`.

## Example

```yaml
redis:
  service_name: redis-service.default.svc.cluster.local
  service_port: 6379
  username: <redis-username>
  password: <redis-password>
  timeout: 1000
  database: 0
tenant_header: x-mse-tenant
consumer_header: x-mse-consumer
balance_key_template: "billing:balance:{tenant}:{quota_scope}:{consumer}"
path_scope: legacy_suffixes
enable_path_suffixes:
  - /v1/chat/completions
  - /v1/messages
missing_balance_policy: deny
```

Rules override only `quota_scope`, `provider`, `path_scope`, and explicit path
or method filters. They inherit Redis, identity headers, the balance key
template, and the missing-balance policy. A provider-native route with no
additional filtering can use:

```yaml
quota_scope: route:qwen
provider: dashscope
path_scope: bound_target
```

Explicit non-empty filters only narrow the binding target.
`enable_methods` uses exact-case matching; examples use canonical uppercase:

```yaml
path_scope: bound_target
enable_path_suffixes:
  - /v1/responses
enable_methods:
  - POST
```

`ai-quota`, `ai-billing`, and `ai-statistics` share no private runtime state
and can be deployed independently.
