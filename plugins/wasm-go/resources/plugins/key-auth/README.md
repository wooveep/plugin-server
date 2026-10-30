---
title: Key 认证
keywords: [higress,key auth]
description: Key 认证插件资源配置参考
---

## 功能说明

`key-auth` 基于 API Key 对请求进行认证，并可在路由或域名等规则级配置中按 consumer 名称做鉴权。插件支持从 HTTP 请求头和 URL 参数中提取 API Key，支持本地 YAML 多凭证 consumer、`Authorization: Bearer <api-key>` 提取、租户身份透传，以及不绑定 consumer 的顶层 `credentials` 认证模式。

本地 YAML 模式直接从 WasmPlugin 配置读取凭证并在解析后保存在内存中，不需要 Redis、数据库或 HTTP 服务作为外部凭证存储。

## 运行属性

插件执行阶段：`认证阶段`
插件执行优先级：`321`

## 配置说明

### 认证配置

| 名称 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `global_auth` | bool | - | 仅实例级配置。`true` 表示全局开启认证；`false` 表示只对配置了规则的域名或路由启用认证；未配置时保持兼容行为。 |
| `consumers` | array of object | - | consumer 列表。与顶层 `credentials` 二选一。consumer 模式认证成功后会注入 `X-Mse-Consumer`，并在配置 `tenant` 时注入 `X-Mse-Tenant`。 |
| `credentials` | array of string | - | 顶层凭证列表，只做认证，不绑定 consumer 名称或 tenant，也不会注入身份 header。与 `consumers` 二选一。 |
| `keys` | array of string | - | API Key 来源字段名，可以是 URL 参数或 HTTP 请求头。使用顶层 `credentials` 或 consumer 未配置 `keys` 时必填。配置 `Authorization` 时支持 `Authorization: Bearer <api-key>`。 |
| `in_query` | bool | - | 是否从 URL 参数提取 API Key。解析后必须至少启用 `in_query` 或 `in_header` 中的一个来源。 |
| `in_header` | bool | - | 是否从 HTTP 请求头提取 API Key。解析后必须至少启用 `in_query` 或 `in_header` 中的一个来源。 |
| `realm` | string | `MSE Gateway` | 认证失败时 `WWW-Authenticate` 响应头中的 realm。 |

`consumers` 中每一项的字段：

| 名称 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `name` | string | - | consumer 名称，必填。 |
| `credential` | string | - | 单个访问凭证。与 `credentials` 二选一，保留已有配置兼容性。 |
| `credentials` | array of string | - | 多个访问凭证。与 `credential` 二选一，不能为空，且凭证值不能重复。 |
| `tenant` | string | - | consumer 所属租户。认证成功后作为可信 `X-Mse-Tenant` 传递。 |
| `keys` | array of string | - | 当前 consumer 的 API Key 来源字段，配置后覆盖全局 `keys`。 |
| `in_query` | bool | - | 当前 consumer 的 URL 参数提取开关，配置后覆盖全局 `in_query`。 |
| `in_header` | bool | - | 当前 consumer 的 HTTP 请求头提取开关，配置后覆盖全局 `in_header`。 |

### 鉴权配置

| 名称 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `allow` | array of string | - | 规则级配置。允许访问当前路由或域名的 consumer 名称列表。顶层 `credentials` 模式没有 consumer 名称，不能用于 `allow` 鉴权。 |

客户端请求中自带的 `X-Mse-Consumer` 或 `X-Mse-Tenant` 不会被作为可信身份；consumer 认证成功后插件会覆盖或移除这些身份头，再写入认证得到的身份。

## 配置示例

### 多凭证 Consumer 和租户透传

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

请求示例：

```bash
curl http://xxx.hello.com/test -H 'Authorization: Bearer real-api-key-1'
```

### 顶层 Credentials 认证模式

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

### 规则级 Allow 鉴权

实例级配置：

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

路由或域名规则级配置：

```yaml
allow:
- consumer1
```

## 相关错误码

| HTTP 状态码 | 原因说明 |
| --- | --- |
| 401 | 请求提供多个 API Key |
| 401 | 请求未提供 API Key |
| 403 | API Key 未配置或 consumer 无权访问 |
