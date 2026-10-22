---
title: Key Authentication
keywords: [higress,key auth]
description: Key Authentication Plugin Configuration Reference
---
## Function Description
The `key-auth` plugin implements authentication based on API Key, supporting the parsing of the API Key from HTTP request URL parameters or request headers, while also verifying whether the API Key has permission to access the resource.

## Runtime Properties
Plugin Execution Phase: `Authentication Phase`
Plugin Execution Priority: `310`

## Configuration Fields
**Note:**
- Authentication and authorization configurations cannot coexist within a single rule.
- For requests authenticated as a consumer, trusted `X-Mse-Consumer` is added. If the consumer has `tenant`, trusted `X-Mse-Tenant` is also added. Client-supplied headers with the same names are not treated as trusted identity.
- Local YAML mode reads credentials from the WasmPlugin configuration and keeps them in memory after parsing. It does not require Redis, a database, or an HTTP service as an external credential store.

### Authentication Configuration
| Name          | Data Type        | Requirements                                    | Default Value | Description                                                                                                                                                                            |
| ------------- | ---------------- | ----------------------------------------------- | ------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `global_auth` | bool             | Optional (**Instance-Level Configuration Only**) | -             | Can only be configured at the instance level; if set to true, the authentication mechanism takes effect globally; if set to false, it only applies to the configured hostnames and routes. If not configured, it will only take effect globally when no hostname and route configurations are present (to maintain compatibility with older user habits). |
| `consumers`   | array of object  | Required unless top-level `credentials` is used | -             | Configures service callers for request authentication. Consumer mode propagates `X-Mse-Consumer` and optional `X-Mse-Tenant`.                                                            |
| `credentials` | array of string  | Required unless `consumers` is used             | -             | Top-level credential list for authentication-only mode. It does not bind credentials to a consumer or tenant and does not inject identity headers.                                      |
| `keys`        | array of string  | Required for top-level `credentials` or consumers without consumer-level `keys` | - | Source field names for the API Key, which can be URL parameters or HTTP request header names. When `Authorization` is configured, `Authorization: Bearer <api-key>` is supported. |
| `in_query`    | bool             | At least one of `in_query` and `in_header` must be true | true          | When configured as true, the gateway will attempt to parse the API Key from URL parameters.                                                                                             |
| `in_header`   | bool             | At least one of `in_query` and `in_header` must be true | true          | When configured as true, the gateway will attempt to parse the API Key from HTTP request headers.                                                                                      |
| `realm`       | string           | Optional                                        | `MSE Gateway` | Realm used in `WWW-Authenticate` response headers when authentication fails.                                                                                                            |

An explicit `consumers: []` means no valid consumers remain, so all requests requiring authentication are rejected. A route or domain rule with `allow: []` denies every consumer while keeping authentication enabled. With `global_auth: false`, requests remain public only when no rule matches. Each consumer's `credentials` list must still be nonempty.

The configuration field descriptions for each item in `consumers` are as follows:
| Name          | Data Type       | Requirements                               | Default Value | Description |
| ------------- | --------------- | ------------------------------------------ | ------------- | ----------- |
| `name`        | string          | Required                                   | -             | Consumer name. |
| `credential`  | string          | Required unless `credentials` is used      | -             | Single access credential for this consumer. This preserves existing configurations. |
| `credentials` | array of string | Required unless `credential` is used       | -             | Multiple access credentials for this consumer. The list cannot be empty, and credential values cannot be duplicated. |
| `tenant`      | string          | Optional                                   | -             | Tenant for this consumer. On successful authentication, it is propagated as `X-Mse-Tenant`. |
| `keys`        | array of string | Optional                                   | -             | Overrides global `keys` for this consumer. |
| `in_query`    | bool            | Optional, with at least one resolved source enabled | - | Overrides global `in_query`. |
| `in_header`   | bool            | Optional, with at least one resolved source enabled | - | Overrides global `in_header`. |

### Authorization Configuration (Optional)
| Name        | Data Type        | Requirements                                    | Default Value | Description                                                                                                                                                           |
| ----------- | ---------------- | ----------------------------------------------- | ------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `allow`     | array of string  | Optional (**Non-Instance Level Configuration**) | -             | Can only be configured on fine-grained rules such as routes or hostnames; specifies the allowed consumers for matching requests, allowing for fine-grained permission control. |

## Configuration Example
### Global Configuration for Authentication and Granular Route Authorization
The following configuration will enable Key Auth authentication and authorization for specific routes or hostnames in the gateway. The `credential` field must not repeat.

At the instance level, do the following plugin configuration:
```yaml
global_auth: false
consumers:
- credential: 2bda943c-ba2b-11ec-ba07-00163e1250b5
  name: consumer1
- credential: c8c8e9ca-558e-4a2d-bb62-e700dcc40e35
  name: consumer2
keys:
- apikey
- x-api-key
```

For routes route-a and route-b, do the following configuration:
```yaml
allow:
- consumer1
```

For the hostnames *.example.com and test.com, do the following configuration:
```yaml
allow:
- consumer2
```

**Note:**
The routes route-a and route-b specified in this example refer to the route names filled in when creating the gateway routes. When matched with these two routes, requests from the caller named consumer1 will be allowed while others will be denied.

The specified hostnames *.example.com and test.com are used to match the request's domain name. When a domain name is matched, callers named consumer2 will be allowed while others will be denied.

Based on this configuration, the following requests will be allowed:

Assuming the following request matches route-a:
**Setting API Key in URL Parameters**
```bash
curl  http://xxx.hello.com/test?apikey=2bda943c-ba2b-11ec-ba07-00163e1250b5
```

**Setting API Key in HTTP Request Headers**
```bash
curl  http://xxx.hello.com/test -H 'x-api-key: 2bda943c-ba2b-11ec-ba07-00163e1250b5'
```

After successful authentication and authorization, the request's header will have an added `X-Mse-Consumer` field with the value `consumer1`, to identify the name of the caller.

The following requests will be denied access:
**Request without an API Key returns 401**
```bash
curl  http://xxx.hello.com/test
```

**Request with an invalid API Key returns 401**
```bash
curl  http://xxx.hello.com/test?apikey=926d90ac-ba2e-11ec-ab68-00163e1250b5
```

**Caller matched with provided API Key has no access rights, returns 403**
```bash
# consumer2 is not in the allow list of route-a
curl  http://xxx.hello.com/test?apikey=c8c8e9ca-558e-4a2d-bb62-e700dcc40e35
```

### Enabling at the Instance Level
The following configuration will enable Key Auth authentication at the instance level for the gateway, requiring all requests to pass authentication before accessing.

```yaml
global_auth: true
consumers:
- credential: 2bda943c-ba2b-11ec-ba07-00163e1250b5
  name: consumer1
- credential: c8c8e9ca-558e-4a2d-bb62-e700dcc40e35
  name: consumer2
keys:
- apikey
- x-api-key
```

### Multiple Credentials And Tenant Propagation
The following configuration lets one consumer have multiple local YAML API keys and authenticate through the `Authorization` request header. After successful authentication, upstream services receive trusted `X-Mse-Consumer: consumer1` and `X-Mse-Tenant: tenant-a`.

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

When `Authorization` is configured in `keys`, `Authorization: Bearer <api-key>` matches on the token after `Bearer `. Non-Bearer `Authorization` values are matched as raw values. Bearer stripping only applies to the `Authorization` source and is not applied to other headers.

The same normalized API key may be presented through more than one enabled source, such as `Authorization: Bearer real-api-key-1` and `x-api-key: real-api-key-1`. The gateway treats that request as one credential presentation. If the request presents two or more distinct normalized API keys, it is rejected with `key-auth.multi_key`. Bearer stripping for equality comparison only applies to the configured `Authorization` source; non-Authorization headers are compared as raw values.

### Top-Level Credentials Mode
Use top-level `credentials` when you only need API key authentication and do not need consumer identity or tenant propagation:

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

Top-level `credentials` mode does not inject `X-Mse-Consumer` or `X-Mse-Tenant`, and it is not treated as a named consumer for `allow`. Use `consumers` when fine-grained consumer authorization is required.

## Related Error Codes
| HTTP Status Code | Error Message                                              | Reason Explanation                |
| ---------------- | ---------------------------------------------------------- | --------------------------------- |
| 401              | Request denied by Key Auth check. Multiple distinct API keys found in request | Multiple distinct API keys provided in the request. |
| 401              | Request denied by Key Auth check. No API key found in request | API Key not provided in the request.      |
| 401              | Request denied by Key Auth check. Invalid API key         | The current API Key is not authorized for access. |
| 403              | Request denied by Key Auth check. Unauthorized consumer   | The caller does not have access permissions.  |
