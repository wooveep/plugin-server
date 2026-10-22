---
title: Key 认证
keywords: [higress,key auth]
description: Key 认证插件配置参考
---

## 功能说明
`key-auth`插件实现了基于 API Key 进行认证鉴权的功能，支持从 HTTP 请求的 URL 参数或者请求头解析 API Key，同时验证该 API Key 是否有权限访问。

## 运行属性

插件执行阶段：`认证阶段`
插件执行优先级：`310`

## 配置字段

**注意：**

- 在一个规则里，鉴权配置和认证配置不可同时存在
- 对于通过 consumer 认证鉴权的请求，请求的 header 会被添加可信的 `X-Mse-Consumer` 字段；如果 consumer 配置了 `tenant`，还会添加可信的 `X-Mse-Tenant` 字段。客户端请求中自带的同名字段不会被作为可信身份。
- 本地 YAML 模式直接从 WasmPlugin 配置读取凭证并在解析后保存在内存中，不需要 Redis、数据库或 HTTP 服务作为外部凭证存储。

### 认证配置
| 名称          | 数据类型        | 填写要求                                    | 默认值 | 描述                                                                                                                                                                            |
| -----------   | --------------- | ------------------------------------------- | ------ | -----------------------------------------------------------                                                                                                                     |
| `global_auth` | bool            | 选填（**仅实例级别配置**）                  | -      | 只能在实例级别配置，若配置为true，则全局生效认证机制; 若配置为false，则只对做了配置的域名和路由生效认证机制，若不配置则仅当没有域名和路由配置时全局生效（兼容老用户使用习惯）。 |
| `consumers`   | array of object | 与 `credentials` 二选一                     | -      | 配置服务的调用者，用于对请求进行认证。使用 consumer 模式时会传播 `X-Mse-Consumer` 和可选 `X-Mse-Tenant`。                                                                       |
| `credentials` | array of string | 与 `consumers` 二选一                       | -      | 顶层凭证列表，只做认证，不绑定 consumer 名称或 tenant，也不会注入身份 header。                                                                                                   |
| `keys`        | array of string | 使用顶层 `credentials` 或 consumer 未配置 `keys` 时必填 | - | API Key 的来源字段名称，可以是 URL 参数或者 HTTP 请求头名称。配置 `Authorization` 时支持从 `Authorization: Bearer <api-key>` 中提取 API Key。                                  |
| `in_query`    | bool            | `in_query` 和 `in_header` 至少有一个为 true | true   | 配置 true 时，网关会尝试从 URL 参数中解析 API Key                                                                                                                               |
| `in_header`   | bool            | `in_query` 和 `in_header` 至少有一个为 true | true   | 配置 true 时，网关会尝试从 HTTP 请求头中解析 API Key                                                                                                                            |
| `realm`       | string          | 选填                                        | `MSE Gateway` | 认证失败时 `WWW-Authenticate` 响应头中的 realm。                                                                                                                               |

显式配置 `consumers: []` 表示当前没有有效调用方；所有需要认证的请求都会被拒绝。路由或域名规则中的 `allow: []` 表示拒绝所有调用方，认证仍然生效。只有未匹配任何规则且 `global_auth: false` 时，请求才不需要认证。单个调用方的 `credentials` 仍必须为非空列表。

`consumers`中每一项的配置字段说明如下：

| 名称          | 数据类型        | 填写要求                                    | 默认值 | 描述 |
| ------------- | --------------- | ------------------------------------------- | ------ | ---- |
| `name`        | string          | 必填                                        | -      | 配置该 consumer 的名称 |
| `credential`  | string          | 与 `credentials` 二选一                     | -      | 配置该 consumer 的单个访问凭证，兼容已有配置 |
| `credentials` | array of string | 与 `credential` 二选一                      | -      | 配置该 consumer 的多个访问凭证，不能为空，且凭证值不能重复 |
| `tenant`      | string          | 选填                                        | -      | 配置该 consumer 的租户。认证成功后会作为 `X-Mse-Tenant` 传递给后端 |
| `keys`        | array of string | 选填                                        | -      | 覆盖全局 `keys`，只使用当前 consumer 的来源字段 |
| `in_query`    | bool            | 选填，解析后至少启用一个来源                | -      | 覆盖全局 `in_query` |
| `in_header`   | bool            | 选填，解析后至少启用一个来源                | -      | 覆盖全局 `in_header` |

### 鉴权配置（非必需）

| 名称        | 数据类型        | 填写要求                                    | 默认值 | 描述                                                                                                                                                           |
| ----------- | --------------- | ------------------------------------------- | ------ | -----------------------------------------------------------                                                                                                    |
| `allow`     | array of string | 选填(**非实例级别配置**)                    | -      | 只能在路由或域名等细粒度规则上配置，对于符合匹配条件的请求，配置允许访问的 consumer，从而实现细粒度的权限控制 |

## 配置示例

### 全局配置认证和路由粒度进行鉴权

以下配置将对网关特定路由或域名开启Key Auth认证和鉴权。credential字段不能重复。

在实例级别做如下插件配置：

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

对 route-a 和 route-b 这两个路由做如下配置：

```yaml
allow: 
- consumer1
```

对 *.example.com 和 test.com 在这两个域名做如下配置:

```yaml
allow:
- consumer2
```

**说明：**

此例指定的route-a和route-b即在创建网关路由时填写的路由名称，当匹配到这两个路由时，将允许name为consumer1的调用者访问，其他调用者不允许访问。

此例指定的*.example.com和test.com用于匹配请求的域名，当发现域名匹配时，将允许name为consumer2的调用者访问，其他调用者不被允许访问。

根据该配置，下列请求可以允许访问：

假设以下请求会匹配到route-a这条路由
n
**将 API Key 设置在 url 参数中**
```bash
curl  http://xxx.hello.com/test?apikey=2bda943c-ba2b-11ec-ba07-00163e1250b5
```
**将 API Key 设置在 http 请求头中**
```bash
curl  http://xxx.hello.com/test -H 'x-api-key: 2bda943c-ba2b-11ec-ba07-00163e1250b5'
```

认证鉴权通过后，请求的header中会被添加一个`X-Mse-Consumer`字段，在此例中其值为`consumer1`，用以标识调用方的名称

下列请求将拒绝访问：

**请求未提供 API Key，返回401**
```bash
curl  http://xxx.hello.com/test
```
**请求提供的 API Key 无权访问，返回401**
```bash
curl  http://xxx.hello.com/test?apikey=926d90ac-ba2e-11ec-ab68-00163e1250b5
```

**根据请求提供的 API Key匹配到的调用者无访问权限，返回403**
```bash
# consumer2不在route-a的allow列表里
curl  http://xxx.hello.com/test?apikey=c8c8e9ca-558e-4a2d-bb62-e700dcc40e35
```

### 网关实例级别开启

以下配置将对网关实例级别开启 Key Auth 认证，所有请求均需要经过认证后才能访问。

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

### 多凭证 Consumer 和租户透传

以下配置允许一个 consumer 拥有多个本地 YAML API Key，并通过 `Authorization` 请求头认证。认证成功后，后端会收到可信的 `X-Mse-Consumer: consumer1` 和 `X-Mse-Tenant: tenant-a`。

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

当 `Authorization` 配置在 `keys` 中时，`Authorization: Bearer <api-key>` 会使用 Bearer 后面的 `<api-key>` 进行匹配；非 Bearer 的 `Authorization` 值会按原始值匹配。Bearer 前缀剥离只适用于 `Authorization` 来源，不会应用到其他请求头。

同一个规范化后的 API Key 可以同时出现在多个已启用来源中，例如 `Authorization: Bearer real-api-key-1` 和 `x-api-key: real-api-key-1`。网关会按一个凭证处理该请求；如果请求中出现两个或更多不同的规范化 API Key，则会拒绝请求并返回 `key-auth.multi_key`。用于判断是否相同的 Bearer 前缀剥离只适用于配置为 `Authorization` 的来源，其他请求头会按原始值比较。

### 顶层 Credentials 认证模式

如果只需要认证 API Key，不需要 consumer 身份和租户透传，可以使用顶层 `credentials`：

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

顶层 `credentials` 模式不会注入 `X-Mse-Consumer` 或 `X-Mse-Tenant`，也不会作为 `allow` 中的命名 consumer 使用。如需按 consumer 做细粒度授权，请使用 `consumers` 配置。


## 相关错误码

| HTTP 状态码 | 出错信息                                                  | 原因说明                |
| ----------- | --------------------------------------------------------- | ----------------------- |
| 401         | Request denied by Key Auth check. Multiple distinct API keys found in request | 请求提供多个不同的 API Key |
| 401         | Request denied by Key Auth check. No API key found in request | 请求未提供 API Key      |
| 401         | Request denied by Key Auth check. Invalid API key         | 不允许当前 API Key 访问 |
| 403         | Request denied by Key Auth check. Unauthorized consumer   | 请求的调用方无访问权限  |
