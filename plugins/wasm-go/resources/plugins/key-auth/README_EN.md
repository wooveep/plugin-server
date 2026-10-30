---
title: Key Authentication
keywords: [higress,key auth]
description: Resource configuration reference for the Key Authentication plugin.
---

## Overview

`key-auth` authenticates requests with API keys and can authorize matched routes or hostnames by consumer name. It supports API key extraction from HTTP headers and URL query parameters, local YAML multi-credential consumers, `Authorization: Bearer <api-key>` extraction, tenant identity propagation, and top-level `credentials` mode for authentication-only use cases.

Local YAML mode reads credentials from the WasmPlugin configuration and keeps them in memory after parsing. It does not require Redis, a database, or an HTTP service as an external credential store.

## Runtime Properties

Plugin execution phase: `Authentication Phase`
Plugin execution priority: `321`

## Configuration

### Authentication Configuration

| Name | Type | Default | Description |
| --- | --- | --- | --- |
| `global_auth` | bool | - | Instance-level only. `true` enables authentication globally; `false` enables authentication only on configured hostnames or routes; omitted keeps the compatibility behavior. |
| `consumers` | array of object | - | Consumer list. Mutually exclusive with top-level `credentials`. Consumer mode injects `X-Mse-Consumer` after authentication and injects `X-Mse-Tenant` when `tenant` is configured. |
| `credentials` | array of string | - | Top-level credential list for authentication-only mode. It does not bind credentials to a consumer or tenant and does not inject identity headers. Mutually exclusive with `consumers`. |
| `keys` | array of string | - | API key source field names, either URL parameters or HTTP request headers. Required for top-level `credentials` or consumers without consumer-level `keys`. When `Authorization` is configured, `Authorization: Bearer <api-key>` is supported. |
| `in_query` | bool | - | Whether to extract API keys from URL query parameters. At least one of `in_query` or `in_header` must be enabled after resolution. |
| `in_header` | bool | - | Whether to extract API keys from HTTP request headers. At least one of `in_query` or `in_header` must be enabled after resolution. |
| `realm` | string | `MSE Gateway` | Realm used in `WWW-Authenticate` response headers when authentication fails. |

Fields for each `consumers` item:

| Name | Type | Default | Description |
| --- | --- | --- | --- |
| `name` | string | - | Consumer name. Required. |
| `credential` | string | - | Single access credential. Mutually exclusive with `credentials`; preserved for existing configurations. |
| `credentials` | array of string | - | Multiple access credentials. Mutually exclusive with `credential`; cannot be empty; credential values cannot be duplicated. |
| `tenant` | string | - | Tenant for this consumer. On successful authentication, it is propagated as trusted `X-Mse-Tenant`. |
| `keys` | array of string | - | API key source fields for this consumer. Overrides global `keys`. |
| `in_query` | bool | - | Query parameter extraction switch for this consumer. Overrides global `in_query`. |
| `in_header` | bool | - | HTTP header extraction switch for this consumer. Overrides global `in_header`. |

### Authorization Configuration

| Name | Type | Default | Description |
| --- | --- | --- | --- |
| `allow` | array of string | - | Rule-level configuration. Consumer names allowed to access the matched route or hostname. Top-level `credentials` mode has no consumer name and cannot be used for `allow` authorization. |

Client-supplied `X-Mse-Consumer` or `X-Mse-Tenant` headers are not trusted. After consumer authentication succeeds, the plugin removes or overwrites these identity headers and writes the authenticated identity.

## Examples

### Multiple Credentials And Tenant Propagation

```yaml
global_auth: true
consumers:
- name: consumer1
  tenant: tenant-a
  credentials:
  - real-api-key-1
  - real-api-key-2
  keys:
  - Authorization
  in_header: true
  in_query: false
keys:
- apikey
- x-api-key
in_header: true
in_query: true
realm: MSE Gateway
```

Request example:

```bash
curl http://xxx.hello.com/test -H 'Authorization: Bearer real-api-key-1'
```

### Top-Level Credentials Mode

```yaml
global_auth: true
credentials:
- real-api-key-1
- real-api-key-2
keys:
- Authorization
in_header: true
in_query: false
```

### Rule-Level Allow Authorization

Instance-level configuration:

```yaml
global_auth: false
consumers:
- name: consumer1
  credential: token1
- name: consumer2
  credential: token2
keys:
- apikey
- x-api-key
in_header: true
in_query: true
```

Route or hostname rule-level configuration:

```yaml
allow:
- consumer1
```

## Error Codes

| HTTP Status Code | Reason |
| --- | --- |
| 401 | Multiple API keys were provided |
| 401 | No API key was provided |
| 403 | API key is not configured or the consumer is not authorized |
