---
title: AI可观测
keywords: [higress, AI, observability]
description: AI可观测配置参考
---

> 插件版本：`2.0.4`

## 介绍

提供 AI 可观测基础能力，包括 metric、log 和 trace。安全基础事实不依赖 ai-proxy；ai-proxy schema-v2 元数据只是可选的可信增强，可提供有界的逻辑 operation 和路径模板。

## 运行属性

插件执行阶段：`默认阶段`
插件执行优先级：`900`

## 配置说明

插件默认请求符合 openai 协议格式，并提供了以下基础可观测值，用户无需特殊配置：

- metric：提供了输入 token、输出 token、首个 token 的 rt（流式请求）、请求总 rt 等指标，支持在网关、路由、服务、模型四个维度上进行观测
- log：提供了 input_token, output_token, model, llm_service_duration, llm_first_token_duration 等字段

用户还可以通过配置的方式对可观测的值进行扩展：

| 名称             | 数据类型  | 填写要求 | 默认值 | 描述                     |
|----------------|-------|------|-----|------------------------|
| `use_default_attributes` | bool | 非必填  | false   | 是否使用默认完整属性配置，包含 messages、answer、question 等所有字段。适用于调试、审计场景 |
| `use_default_response_attributes` | bool | 非必填  | false   | 是否使用轻量级默认属性配置（推荐），包含 model 和 token 统计，不缓冲流式响应体。适用于高并发生产环境 |
| `attributes` | []Attribute | 非必填  | -   | 用户希望记录在log/span中的信息 |
| `disable_openai_usage` | bool | 非必填  | false   | 非openai兼容协议时，model、token的支持非标，配置为true时可以避免报错 |
| `value_length_limit` | int | 非必填  | 条件默认值 | 记录的单个 value 的长度限制 |
| `path_scope` | string | 非必填 | `legacy_suffixes` | `legacy_suffixes` 或 `bound_target` |
| `enable_path_suffixes` | []string    | 非必填   | 历史行为见下文 | 仅 legacy_suffixes 的路径门控，可显式配置 "\*"；bound_target 仅允许省略或 [] |
| `enable_methods` | []string | 非必填 | 无 | legacy_suffixes 下按精确大小写匹配；bound_target 仅允许省略或 [] |
| `enable_content_types` | []string    | 非必填   | []     | 只对这些内容类型的响应进行缓冲处理。如果为空数组，则对所有内容类型生效                                                           |
| `session_id_header` | string | 非必填  | -   | 指定读取 session ID 的 header 名称。如果不配置，将按以下优先级自动查找：`x-openclaw-session-key`、`x-clawdbot-session-key`、`x-moltbot-session-key`、`x-agent-session`。session ID 可用于追踪多轮 Agent 对话 |

`value_length_limit` 的条件运行时默认值为：`32000`（普通/自定义配置）、`10485760`（完整默认属性）和 `4000`（轻量默认响应属性）。`enable_content_types` 省略列表时匹配所有 Content-Type；内容解析或缓冲选择不会抑制终态安全基础事实。

### 请求作用域

- 缺省或显式 `path_scope: legacy_suffixes` 保留历史行为：启用任一内置默认属性且省略后缀列表时，有效默认值为 `/completions` 和 `/messages`；未启用内置默认属性时，省略列表保留原有全路径行为。
- `path_scope: bound_target` 使用 Higress 绑定目标。缺省或字面空数组 `enable_path_suffixes: []`、`enable_methods: []` 不额外收窄，也不会合成路径列表。
- bound_target 拒绝任何显式非空路径/方法列表（包括 ["*"]），避免匹配到的 AI 路由漏统计；切换到该作用域时清除继承的 legacy 过滤器。legacy_suffixes 仍保留显式交集行为。
- ai-statistics 对 `enable_methods` 使用精确大小写匹配，示例采用规范的大写方法。路由级字段缺省时继承全局配置。

```yaml
# 兼容历史默认
use_default_response_attributes: true
path_scope: legacy_suffixes
enable_path_suffixes:
  - /completions
  - /messages
```

```yaml
# 绑定目标，无额外过滤
path_scope: bound_target
```

```yaml
# 独立 legacy 部署的显式交集
path_scope: legacy_suffixes
enable_path_suffixes:
  - /v1/responses
enable_methods:
  - POST
```

空全局且仅含 _rules_ 的配置中，每个匹配规则独立解析完整统计配置；内置属性、内容类型和禁用 usage 等设置必须在该规则给出。有真实有效全局配置时，规则只覆盖原有三项路由设置，其它完整配置字段必须在根级设置，规则中出现会被拒绝。托管发布下发 path_scope: bound_target 且省略或清空两类过滤器，路由匹配由 Higress 绑定决定。

## 安全基础可观测契约

安全基础事实独立成立；ai-proxy schema-v2 元数据只是可选的可信增强。

每个符合范围且已完成的请求都会输出有界的 `route`、`path`、规范化且有界的 `method`、`status_code`、正值 `latency_ms` 和有界 `error_class`，并恰好输出一次 `request_count` 和 `request_duration_ms`。这些安全基础事实与 model、逻辑 operation、token 或 usage 提取无关；opaque 成功、opaque 错误以及解析失败仍会输出基础事实，可选维度会省略而不是猜测。可信的 provider 解析器可以补充 model/token 信息，但不得从任意路径或 opaque body 推断。原始响应体和无界 provider 错误文本绝不进入指标属性。

托管 bound_target 的 token 扩展只记录有效 JSON/完整 SSE frame 中实际出现的非负整数；input、output、total 分别保持已知或缺失，不把缺字段或无效值补为 0，也不从部分用量推导 total。真实报告的 0 保留。当前复用提取器/指标计数器支持不超过 int64 上限的十进制整数字段，超限值保持未知；正文仍透明透传。SSE 跨块 frame 使用原正文缓冲上限，未完成或超限只停止扩展解析，基本事实仍在原收尾点输出。legacy 提取兼容行为不变。

基础统计覆盖已进入统计回调的匹配请求。前置认证过滤器直接拒绝、未进入统计回调的链路，不能靠本插件补造统计，须在实际过滤顺序和网关原生观测中验证。该源码行为需另构建和登记新精确制品版本，不能由现有版本号推定已生效。

`1.0.2` 还保证每个配置的计数器缓存会在首次使用时初始化，包括规则覆盖生命周期；第一个符合条件的基础指标不会因 nil 缓存而失败。

响应头只捕获有界状态和错误投影，不会冻结完成时延。首次真实终态——非流式响应体 EOS、流式 EOS，或无响应体回调、空 body、不支持 Content-Type、解析器跳过时的 `wrapper.ProcessStreamDone` 回退——使用同一个终态时间戳完成指标和 AILog。请求级完成标记保证所有终态回调合计恰好一次；响应头和 body 字节保持透明。

只有恰好三位 ASCII 数字且数值位于 `100..599` 的状态才可信。其他输入统一为 `status_code=0` 和 `status_unknown`。有效 `2xx..3xx` 使用 `none`，`4xx` 使用 `http_4xx`，`5xx` 使用 `http_5xx`；其他有效状态使用 `status_unknown`。

`route` 只来自可信 Higress `route_name`：它必须非空、是有效 UTF-8、最多 128 字节且不含控制字符，否则使用 `route_unknown`。`path` 只接受实际选中适配器的 `matched:true` schema v2 `operation_template`：它必须是 origin-form、恰好一个前导 `/`、最多 128 字节，并且只包含 ASCII 字母/数字和 `/ . _ - ~ { }`，不得包含 authority、query、fragment、控制字符或 Unicode。

无元数据、格式错误或超过 1024 字节的元数据、`matched:false`、schema v1、未知 schema，以及缺少任一必需 operation 字段的 v2 元数据都使用 `provider_native_unknown`。已提供但非法或超长的选中 v2 模板使用 `path_unknown`，且绝不输出提交值。逻辑 `normalized_operation` 必须非空、不超过 128 字节、不能以 `/` 开头，只能包含 ASCII 字母/数字和 `/ . _ - ~`，且不能包含 query、fragment、authority 或控制字符；它单独用于可选 AILog operation，不会作为路径模板。schema v1 只能保留通过该校验的逻辑 operation，不能信任 adapter、adapter version、provider、capability 或 usage profile。unknown、malformed 或 unmatched 元数据不贡献任何可选 enrichment。

路径归因绝不使用原始路径、query、请求头、凭据、请求体或响应体，也不使用动态请求片段或原始路径的哈希。`usage_missing` 是 ai-billing 的事件字段，不是 ai-statistics 的事实或指标维度；ai-statistics 在 usage/model/逻辑 operation/token 不可用或禁用时仍输出安全基础 count/duration 和终态 AILog，只省略不可用的可选维度，不会创建合成的 `usage_missing` 指标标签。

Attribute 配置说明:

| 名称                    | 数据类型 | 填写要求 | 默认值 | 描述                                                                                                                                        |
| ----------------------- | -------- | -------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------- |
| `key`                   | string   | 必填     | -      | attribute 名称                                                                                                                              |
| `value_source`          | string   | 条件必填 | -      | 自定义属性需要；内置属性可省略。可选值为 `fixed_value`, `request_header`, `request_body`, `response_header`, `response_body`, `response_streaming_body` |
| `value`                 | string   | 条件必填 | -      | 自定义属性需要；内置属性可省略。配置 attribute 取值 key value/path                                                                          |
| `default_value`         | string   | 非必填   | -      | attribute 默认值                                                                                                                            |
| `rule`                  | string   | 非必填   | -      | 从流式响应中提取 attribute 的规则，可选值为 `first`, `replace`, `append`                                                                    |
| `apply_to_log`          | bool     | 非必填   | false  | 是否将提取的信息记录在日志中                                                                                                                |
| `apply_to_span`         | bool     | 非必填   | false  | 是否将提取的信息记录在链路追踪 span 中                                                                                                      |
| `trace_span_key`        | string   | 非必填   | -      | 链路追踪 attribute key，默认会使用`key`的设置                                                                                               |
| `as_separate_log_field` | bool     | 非必填   | false  | 记录日志时是否作为单独的字段，日志字段名使用`key`的设置                                                                                     |

`value_source` 的各种取值含义如下：

- `fixed_value`：固定值
- `request_header` ： attribute 值通过 http 请求头获取，value 配置为 header key
- `request_body` ：attribute 值通过请求 body 获取，value 配置格式为 gjson 的 jsonpath
- `response_header` ：attribute 值通过 http 响应头获取，value 配置为 header key
- `response_body` ：attribute 值通过响应 body 获取，value 配置格式为 gjson 的 jsonpath
- `response_streaming_body` ：attribute 值通过流式响应 body 获取，value 配置格式为 gjson 的 jsonpath

当 `value_source` 为 `response_streaming_body` 时，应当配置 `rule`，用于指定如何从流式 body 中获取指定值，取值含义如下：

- `first`：多个 chunk 中取第一个有效 chunk 的值
- `replace`：多个 chunk 中取最后一个有效 chunk 的值
- `append`：拼接多个有效 chunk 中的值，可用于获取回答内容

注意：对于 `first` 和 `replace`，有效 chunk 不包含路径不存在、JSON `null` 或空字符串 `""` 的结果。如果所有 chunk 都没有有效值，最终提取结果为 `nil`；依赖空字符串作为“上游显式清空”信号的配置需要注意此行为差异。`append` 规则保持原有拼接行为。

### 内置属性 (Built-in Attributes)

插件提供了一些内置属性键（key），可以直接使用而无需配置 `value_source` 和 `value`。这些内置属性会自动从请求/响应中提取相应的值：

| 内置属性键 | 说明 | 适用场景 |
|---------|------|---------|
| `question` | 用户提问内容 | 支持 OpenAI/Claude 消息格式 |
| `system` | 系统提示词 | 支持 Claude `/v1/messages` 的顶层 system 字段 |
| `answer` | AI 回答内容 | 支持 OpenAI/Claude 消息格式，流式和非流式 |
| `tool_calls` | 工具调用信息 | OpenAI/Claude 工具调用 |
| `reasoning` | 推理过程 | OpenAI o1 等推理模型 |
| `reasoning_tokens` | 推理 token 数（如 o1 模型） | OpenAI Chat Completions，从 `output_token_details.reasoning_tokens` 提取 |
| `cached_tokens` | 缓存命中的 token 数 | OpenAI Chat Completions，从 `input_token_details.cached_tokens` 提取 |
| `input_token_details` | 输入 token 详细信息（完整对象） | OpenAI/Gemini/Anthropic，包含缓存、工具使用等详情 |
| `output_token_details` | 输出 token 详细信息（完整对象） | OpenAI/Gemini/Anthropic，包含推理 token、生成图片数等详情 |

使用内置属性时，只需设置 `key`、`apply_to_log` 等参数，无需设置 `value_source` 和 `value`。

**注意**：
- `reasoning_tokens` 和 `cached_tokens` 是从 token details 中提取的便捷字段，适用于 OpenAI Chat Completions API
- `input_token_details` 和 `output_token_details` 会以 JSON 字符串形式记录完整的 token 详情对象

## 配置示例

如果希望在网关访问日志中记录 ai-statistic 相关的统计值，需要修改 log_format，在原 log_format 基础上添加一个新字段，示例如下：

```yaml
'{"ai_log":"%FILTER_STATE(wasm.ai_log:PLAIN)%"}'
```

如果字段设置了 `as_separate_log_field`，例如：

```yaml
attributes:
  - key: consumer
    value_source: request_header
    value: x-mse-consumer
    apply_to_log: true
    as_separate_log_field: true
```

那么要在日志中打印，需要额外设置 log_format：

```
'{"consumer":"%FILTER_STATE(wasm.consumer:PLAIN)%"}'
```

### 空配置

#### 监控

```
# counter 类型，输入 token 数量的累加值
route_upstream_model_consumer_metric_input_token{ai_route="ai-route-aliyun.internal",ai_cluster="outbound|443||llm-aliyun.internal.dns",ai_model="qwen-turbo",ai_consumer="none"} 24

# counter 类型，输出 token 数量的累加值
route_upstream_model_consumer_metric_output_token{ai_route="ai-route-aliyun.internal",ai_cluster="outbound|443||llm-aliyun.internal.dns",ai_model="qwen-turbo",ai_consumer="none"} 507

# counter 类型，流式请求和非流式请求消耗总时间的累加值
route_upstream_model_consumer_metric_llm_service_duration{ai_route="ai-route-aliyun.internal",ai_cluster="outbound|443||llm-aliyun.internal.dns",ai_model="qwen-turbo",ai_consumer="none"} 6470

# counter 类型，流式请求和非流式请求次数的累加值
route_upstream_model_consumer_metric_llm_duration_count{ai_route="ai-route-aliyun.internal",ai_cluster="outbound|443||llm-aliyun.internal.dns",ai_model="qwen-turbo",ai_consumer="none"} 2

# counter 类型，流式请求首个 token 延时的累加值
route_upstream_model_consumer_metric_llm_first_token_duration{ai_route="ai-route-aliyun.internal",ai_cluster="outbound|443||llm-aliyun.internal.dns",ai_model="qwen-turbo",ai_consumer="none"} 340

# counter 类型，流式请求次数的累加值
route_upstream_model_consumer_metric_llm_stream_duration_count{ai_route="ai-route-aliyun.internal",ai_cluster="outbound|443||llm-aliyun.internal.dns",ai_model="qwen-turbo",ai_consumer="none"} 1
```

以下是使用指标的几个示例：

流式请求首个 token 的平均延时：

```
irate(route_upstream_model_consumer_metric_llm_first_token_duration[2m])
/
irate(route_upstream_model_consumer_metric_llm_stream_duration_count[2m])
```

流式请求和非流式请求平均消耗的总时长：

```
irate(route_upstream_model_consumer_metric_llm_service_duration[2m])
/
irate(route_upstream_model_consumer_metric_llm_duration_count[2m])
```

#### 日志

```json
{
  "ai_log": "{\"model\":\"qwen-turbo\",\"input_token\":\"10\",\"output_token\":\"69\",\"llm_first_token_duration\":\"309\",\"llm_service_duration\":\"1955\"}"
}
```

如果请求中携带了 session ID header，日志中会自动添加 `session_id` 字段：

```json
{
  "ai_log": "{\"session_id\":\"sess_abc123\",\"model\":\"qwen-turbo\",\"input_token\":\"10\",\"output_token\":\"69\",\"llm_first_token_duration\":\"309\",\"llm_service_duration\":\"1955\"}"
}
```

#### 链路追踪

配置为空时，不会在 span 中添加额外的 attribute

### 从非 openai 协议提取 token 使用信息

在 ai-proxy 中设置协议为 original 时，以百炼为例，可作如下配置指定如何提取 model, input_token, output_token

```yaml
attributes:
  - key: model
    value_source: response_body
    value: usage.models.0.model_id
    apply_to_log: true
    apply_to_span: false
  - key: input_token
    value_source: response_body
    value: usage.models.0.input_tokens
    apply_to_log: true
    apply_to_span: false
  - key: output_token
    value_source: response_body
    value: usage.models.0.output_tokens
    apply_to_log: true
    apply_to_span: false
```

#### 监控

```
route_upstream_model_consumer_metric_input_token{ai_route="bailian",ai_cluster="qwen",ai_model="qwen-max"} 343
route_upstream_model_consumer_metric_output_token{ai_route="bailian",ai_cluster="qwen",ai_model="qwen-max"} 153
route_upstream_model_consumer_metric_llm_service_duration{ai_route="bailian",ai_cluster="qwen",ai_model="qwen-max"} 3725
route_upstream_model_consumer_metric_llm_duration_count{ai_route="bailian",ai_cluster="qwen",ai_model="qwen-max"} 1
```

#### 日志

此配置下日志效果如下：

```json
{
  "ai_log": "{\"model\":\"qwen-max\",\"input_token\":\"343\",\"output_token\":\"153\",\"llm_service_duration\":\"19110\"}"
}
```

#### 链路追踪

链路追踪的 span 中可以看到 model, input_token, output_token 三个额外的 attribute

### 配合认证鉴权记录 consumer

举例如下：

```yaml
attributes:
  - key: consumer # 配合认证鉴权记录consumer
    value_source: request_header
    value: x-mse-consumer
    apply_to_log: true
```

### 记录问题与回答

#### 仅记录当前轮次的问题与回答

```yaml
attributes:
  - key: question # 记录当前轮次的问题（最后一条用户消息）
    value_source: request_body
    value: messages.@reverse.0.content
    apply_to_log: true
  - key: answer # 在流式响应中提取大模型的回答
    value_source: response_streaming_body
    value: choices.0.delta.content
    rule: append
    apply_to_log: true
  - key: answer # 在非流式响应中提取大模型的回答
    value_source: response_body
    value: choices.0.message.content
    apply_to_log: true
```

#### 记录完整的多轮对话历史（推荐配置）

对于多轮 Agent 对话场景，使用内置属性可以大幅简化配置：

```yaml
session_id_header: "x-session-id"  # 可选，指定 session ID header
attributes:
  - key: messages     # 完整对话历史
    value_source: request_body
    value: messages
    apply_to_log: true
  - key: question     # 内置属性，自动提取最后一条用户消息
    apply_to_log: true
  - key: answer       # 内置属性，自动提取回答
    apply_to_log: true
  - key: reasoning    # 内置属性，自动提取思考过程
    apply_to_log: true
  - key: tool_calls   # 内置属性，自动提取工具调用
    apply_to_log: true
```

**内置属性说明：**

插件提供以下内置属性 key，无需配置 `value_source` 和 `value` 字段即可自动提取：

| 内置 Key | 说明 | 默认 value_source |
|---------|------|-------------------|
| `question` | 自动提取最后一条用户消息 | `request_body` |
| `answer` | 自动提取回答内容（支持 OpenAI/Claude 协议） | `response_streaming_body` / `response_body` |
| `tool_calls` | 自动提取并拼接工具调用（流式场景自动按 index 拼接 arguments） | `response_streaming_body` / `response_body` |
| `reasoning` | 自动提取思考过程（reasoning_content，如 DeepSeek-R1） | `response_streaming_body` / `response_body` |

> **注意**：如果配置了 `value_source` 和 `value`，将优先使用配置的值，以保持向后兼容。

日志输出示例：

```json
{
  "ai_log": "{\"session_id\":\"sess_abc123\",\"messages\":[{\"role\":\"user\",\"content\":\"北京天气怎么样？\"}],\"question\":\"北京天气怎么样？\",\"reasoning\":\"用户想知道北京的天气，我需要调用天气查询工具。\",\"tool_calls\":[{\"index\":0,\"id\":\"call_abc123\",\"type\":\"function\",\"function\":{\"name\":\"get_weather\",\"arguments\":\"{\\\"location\\\":\\\"Beijing\\\"}\"}}],\"model\":\"deepseek-reasoner\"}"
}
```

**流式响应中的 tool_calls 处理：**

插件会自动按 `index` 字段识别每个独立的工具调用，拼接分片返回的 `arguments` 字符串，最终输出完整的工具调用列表。

### 记录 Token 详情

使用内置属性记录 OpenAI Chat Completions 的 token 详细信息：

```yaml
attributes:
  # 使用便捷的内置属性提取特定字段
  - key: reasoning_tokens  # 推理token数（o1等推理模型）
    apply_to_log: true
  - key: cached_tokens  # 缓存命中的token数
    apply_to_log: true
  # 记录完整的token详情对象
  - key: input_token_details
    apply_to_log: true
  - key: output_token_details
    apply_to_log: true
```

#### 日志示例

对于使用了 prompt caching 和推理模型的请求，日志可能如下：

```json
{
  "ai_log": "{\"model\":\"gpt-4o\",\"input_token\":\"100\",\"output_token\":\"50\",\"reasoning_tokens\":\"25\",\"cached_tokens\":\"80\",\"input_token_details\":\"{\\\"cached_tokens\\\":80}\",\"output_token_details\":\"{\\\"reasoning_tokens\\\":25}\",\"llm_service_duration\":\"2000\"}"
}
```

其中：
- `reasoning_tokens`: 25 - 推理过程产生的 token 数
- `cached_tokens`: 80 - 从缓存中读取的 token 数
- `input_token_details`: 完整的输入 token 详情（JSON 格式）
- `output_token_details`: 完整的输出 token 详情（JSON 格式）

这些详情对于：
1. **成本优化**：了解缓存命中率，优化 prompt caching 策略
2. **性能分析**：分析推理 token 占比，评估推理模型的实际开销
3. **使用统计**：细粒度统计各类 token 的使用情况

## 流式响应观测能力

流式（Streaming）响应是 AI 对话的常见场景，插件提供了完善的流式观测支持，能够正确拼接和提取流式响应中的关键信息。

### 流式响应的挑战

流式响应将完整内容拆分为多个 SSE chunk 逐步返回，例如：

```
data: {"choices":[{"delta":{"content":"Hello"}}]}
data: {"choices":[{"delta":{"content":" 👋"}}]}
data: {"choices":[{"delta":{"content":"!"}}]}
data: [DONE]
```

要获取完整的回答内容，需要将各个 chunk 中的 `delta.content` 拼接起来。

### 自动拼接机制

插件针对不同类型的内容提供了自动拼接能力：

| 内容类型 | 拼接方式 | 说明 |
|---------|---------|------|
| `answer` | 文本追加（append） | 将各 chunk 的 `delta.content` 按顺序拼接成完整回答 |
| `reasoning` | 文本追加（append） | 将各 chunk 的 `delta.reasoning_content` 按顺序拼接 |
| `tool_calls` | 按 index 组装 | 识别每个工具调用的 `index`，分别拼接各自的 `arguments` |

#### answer 和 reasoning 拼接示例

流式响应：
```
data: {"choices":[{"delta":{"content":"你好"}}]}
data: {"choices":[{"delta":{"content":"，我是"}}]}
data: {"choices":[{"delta":{"content":"AI助手"}}]}
```

最终提取的 `answer`：`"你好，我是AI助手"`

#### tool_calls 拼接示例

流式响应（多个并行工具调用）：
```
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_001","function":{"name":"get_weather"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":1,"id":"call_002","function":{"name":"get_time"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"Beijing\"}"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":1,"function":{"arguments":"{\"city\":\"Shanghai\"}"}}]}}]}
```

最终提取的 `tool_calls`：
```json
[
  {"index":0,"id":"call_001","function":{"name":"get_weather","arguments":"{\"city\":\"Beijing\"}"}},
  {"index":1,"id":"call_002","function":{"name":"get_time","arguments":"{\"city\":\"Shanghai\"}"}}
]
```

### 使用默认配置快速启用

插件提供两种默认配置模式：

#### 轻量模式（推荐用于生产环境）

通过 `use_default_response_attributes: true` 启用轻量模式：

```yaml
use_default_response_attributes: true
```

此配置是**推荐的生产环境配置**，特别适合高并发、高延迟的场景：

| 字段 | 说明 |
|------|------|
| `model` | 模型名称（从请求体提取） |
| `reasoning_tokens` | 推理 token 数 |
| `cached_tokens` | 缓存命中 token 数 |
| `input_token_details` | 输入 token 详情 |
| `output_token_details` | 输出 token 详情 |

**为什么推荐轻量模式？**

LLM 请求有两个显著特点：**延迟高**（通常数秒到数十秒）和**请求体大**（多轮对话可能达到数百 KB 甚至 MB 级别）。

在高并发场景下，如果请求体和响应体都被缓存在内存中，积压的请求会占用大量内存：
- 假设 QPS=100，平均延迟=10秒，请求体=500KB
- 同时在处理的请求数 ≈ 100 × 10 = 1000 个
- 如果缓存完整请求体+响应体：1000 × 1.5MB ≈ **1.5GB 内存**

轻量模式通过以下方式降低内存占用：
- **缓冲请求体**：仅用于提取 `model` 字段（很小），不提取 `question`、`system`、`messages` 等大字段
- **不缓冲流式响应体**：不提取 `answer`、`reasoning`、`tool_calls` 等需要完整响应的字段
- **只统计 token**：从响应的 usage 字段提取 token 信息

**内存对比：**

| 场景 | 完整模式 | 轻量模式 |
|------|----------|----------|
| 单次请求 (1MB 请求 + 500KB 响应) | ~1.5MB | ~1MB（请求体） |
| 高并发 (100 QPS, 10s 延迟) | ~1.5GB | ~1GB |
| 超高并发 (1000 QPS, 10s 延迟) | ~15GB | ~10GB |

**注意**：轻量模式下 `chat_round` 字段会正常计算，`model` 会从请求体正常提取。

#### 完整模式

通过 `use_default_attributes: true` 可以一键启用完整的流式观测能力：

```yaml
use_default_attributes: true
```

此配置会自动记录以下字段，**但会缓冲完整的请求体和流式响应体**：

| 字段 | 说明 | 内存影响 |
|------|------|----------|
| `messages` | 完整对话历史 | ⚠️ 可能很大 |
| `question` | 最后一条用户消息 | 需要缓冲请求体 |
| `system` | 系统提示词 | 需要缓冲请求体 |
| `answer` | AI 回答（自动拼接流式 chunk） | ⚠️ 需要缓冲响应体 |
| `reasoning` | 推理过程（自动拼接流式 chunk） | ⚠️ 需要缓冲响应体 |
| `tool_calls` | 工具调用（自动按 index 组装） | 需要缓冲响应体 |
| `reasoning_tokens` | 推理 token 数 | 无 |
| `cached_tokens` | 缓存命中 token 数 | 无 |
| `input_token_details` | 输入 token 详情 | 无 |
| `output_token_details` | 输出 token 详情 | 无 |

**注意**：完整模式适用于调试、审计等需要完整对话记录的场景，但在高并发生产环境可能消耗大量内存。

### 流式日志示例

启用默认配置后，一个流式请求的日志输出示例：

```json
{
  "answer": "2 plus 2 equals 4.",
  "question": "What is 2+2?",
  "response_type": "stream",
  "tool_calls": null,
  "reasoning": null,
  "model": "glm-4-flash",
  "input_token": 10,
  "output_token": 8,
  "llm_first_token_duration": 425,
  "llm_service_duration": 985,
  "chat_id": "chat_abc123"
}
```

包含工具调用的流式日志示例：

```json
{
  "answer": null,
  "question": "What's the weather in Beijing?",
  "response_type": "stream",
  "tool_calls": [
    {
      "id": "call_abc123",
      "type": "function",
      "function": {
        "name": "get_weather",
        "arguments": "{\"location\": \"Beijing\"}"
      }
    }
  ],
  "model": "glm-4-flash",
  "input_token": 50,
  "output_token": 15,
  "llm_first_token_duration": 300,
  "llm_service_duration": 500
}
```

### 流式特有指标

流式响应会额外记录以下指标：

- `llm_first_token_duration`：从请求发出到收到首个 token 的时间（首字延迟）
- `llm_stream_duration_count`：流式请求次数

可用于监控流式响应的用户体验：

```promql
# 平均首字延迟
irate(route_upstream_model_consumer_metric_llm_first_token_duration[5m])
/
irate(route_upstream_model_consumer_metric_llm_stream_duration_count[5m])
```

## 调试

### 验证 ai_log 内容

在测试或调试过程中，可以通过开启 Higress 的 debug 日志来验证 ai_log 的内容：

```bash
# 日志格式示例
2026/01/31 23:29:30 proxy_debug_log: [ai-statistics] [nil] [test-request-id] [ai_log] attributes to be written: {"question":"What is 2+2?","answer":"4","reasoning":"...","tool_calls":[...],"session_id":"sess_123","model":"gpt-4","input_token":20,"output_token":10}
```

通过这个debug日志可以验证：
- question/answer/reasoning 是否正确提取
- tool_calls 是否正确拼接（特别是流式场景下的arguments）
- session_id 是否正确识别
- 各个字段是否符合预期

## 进阶

配合阿里云 SLS 数据加工，可以将 ai 相关的字段进行提取加工，例如原始日志为：

````
ai_log:{"question":"用python计算2的3次方","answer":"你可以使用 Python 的乘方运算符 `**` 来计算一个数的次方。计算2的3次方，即2乘以自己2次，可以用以下代码表示：\n\n```python\nresult = 2 ** 3\nprint(result)\n```\n\n运行这段代码，你会得到输出结果为8，因为2乘以自己两次等于8。","model":"qwen-max","input_token":"16","output_token":"76","llm_service_duration":"5913"}
````

使用如下数据加工脚本，可以提取出 question 和 answer：

```
e_regex("ai_log", grok("%{EXTRACTJSON}"))
e_set("question", json_select(v("json"), "question", default="-"))
e_set("answer", json_select(v("json"), "answer", default="-"))
```

提取后，SLS 中会添加 question 和 answer 两个字段，示例如下：

````
ai_log:{"question":"用python计算2的3次方","answer":"你可以使用 Python 的乘方运算符 `**` 来计算一个数的次方。计算2的3次方，即2乘以自己2次，可以用以下代码表示：\n\n```python\nresult = 2 ** 3\nprint(result)\n```\n\n运行这段代码，你会得到输出结果为8，因为2乘以自己两次等于8。","model":"qwen-max","input_token":"16","output_token":"76","llm_service_duration":"5913"}

question:用python计算2的3次方

answer:你可以使用 Python 的乘方运算符 `**` 来计算一个数的次方。计算2的3次方，即2乘以自己2次，可以用以下代码表示：

result = 2 ** 3
print(result)

运行这段代码，你会得到输出结果为8，因为2乘以自己两次等于8。

````

### 路径和内容类型过滤配置示例

#### 只处理特定 AI 路径

```yaml
enable_path_suffixes:
  - "/v1/chat/completions"
  - "/v1/embeddings"
  - "/generateContent"
```

#### 只处理特定内容类型

```yaml
enable_content_types:
  - "text/event-stream"
  - "application/json"
```

#### 处理所有路径（通配符）

```yaml
enable_path_suffixes:
  - "*"
```

#### 处理所有内容类型（空数组）

```yaml
enable_content_types: []
```

#### 完整配置示例

```yaml
enable_path_suffixes:
  - "/v1/chat/completions"
  - "/v1/embeddings"
  - "/generateContent"
enable_content_types:
  - "text/event-stream"
  - "application/json"
attributes:
  - key: model
    value_source: request_body
    value: model
    apply_to_log: true
  - key: consumer
    value_source: request_header
    value: x-mse-consumer
    apply_to_log: true
```
