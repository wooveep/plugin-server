# `ai-proxy` 跨协议 AI Agent 兼容性修复、错误契约修正与影响范围评估

| 字段 | 内容 |
| --- | --- |
| 文档性质 | 实施计划、开发设计与开发环境（`modelfusion-dev`）验收记录 |
| 状态 | 开发环境（`modelfusion-dev` Helm rev 25）真机 E2E 验收通过；待后续完整版本评审合入 `main` 及发布窗口推进灰度（灰度 `172.16.31.200` 保持零改动） |
| 创建日期 | 2026-11-19 |
| 最近更新 | 2026-11-19 |
| 涉及项目 | 主实施：`plugin-server`（`extensions/ai-proxy` `2.0.93 -> 2.0.94`，本地 `dev` `abc7a34a`）；回归验证：`modelfusion-deploy`（`scripts/test-ai-agent-multiturn.py`）；跨仓契约对照：`ocloudmodelfusion`（无代码修改） |
| 负责人 | 待确认 |
| 优先级 | 待确认 |
| 目标日期 | 待确认（`modelfusion-dev` E2E 已于 2026-11-19 完成；灰度发布窗口待确认） |
| 需求来源 | 灰度 `172.16.31.200` `GitHubCopilotChat/0.74.0` `POST /v1/responses` 跨协议 `400 server_error` / 客户端 `502` 故障溯源、7 大编程 Agent 协议矩阵调研及 42 样本离线红测 |

---

## 1. 目标、非目标与架构演进衔接

### 1.1 目标

1. **修复 `/v1/responses` 跨协议请求转换拦截过度问题（模块 A）**：覆盖 `GitHubCopilotChat/0.74.0`（CAPI/BYOK）、Codex CLI、Pi Agent、OpenClaw、Hermes Agent、LiteLLM Proxy 的合法无状态元字段、`null` 边界、多轮加密/摘要 `reasoning` 回传，以及转发至非 OpenAI 兼容 Chat 上游时的专有字段去噪。
2. **明确并解决 `/v1/responses` 回程 `reasoning` SSE 事件与平台 `R05` 合同冲突（模块 B）**：以显式决策点呈现保持 `R05` 与支持 `response.reasoning_summary_text.delta` 的权衡，严禁静默破坏现有合同。
3. **补齐 `/v1/messages -> OpenAI Chat` 现代 Agent 特性（模块 C）**：支持 `thinking: {type: "adaptive"}` + `output_config.effort` 映射、`tool_result` 多模态图片/空内容边界及 `message_stop` 终态契约校验。
4. **消除 `openai.go` / `qwen.go` 跨客户端桥接残留与 DashScope 零值丢失（模块 D）**：安全清洗 LiteLLM / Hermes 在 `/v1/chat/completions` 注入的非标准桥接字段，修复 `dashscope_text` `Temperature` / `PresencePenalty` 显式 `0` 值被 `omitempty` 吞掉问题，并保护 `tool_choice: "required"`。
5. **以零爆炸半径修正 `ai-proxy` 本地错误包络与脱敏日志（模块 E）**：将可预期 4xx 协议/参数拒绝从 `server_error` 修正为精确的 `invalid_request_error` / `request_too_large` / `permission_error` 并补齐响应头（`x-request-id`、`x-should-retry: false`），同时在 `managed_protocol.go` / `managed_execution.go` 打脱敏字段级日志；严禁修改 `pkg/ai/clientresponse/*.go`。

### 1.2 非目标与严格边界

- **不触碰同协议直通路径**：`input_protocol == upstream_protocol` 继续走原生直通与既定字节投影，不新增字段拦截。
- **不放开真正无状态不可还原的硬红线**：非空 `previous_response_id`、`conversation`、`background: true`、`item_reference`、非 `"function"` 托管工具（`web_search` / `custom` / `tool_search` 等）、`input_file` / `file_id` 等在无状态跨协议 fallback 下仍明确拒绝 `400`。
- **不修改 `pkg/ai/clientresponse/*.go` 与兄弟插件**：避免触发 `plugin-server` 其余 55 项正式制品重编与 `version_conflict`。
- **不修改 `ocloudmodelfusion` 控制面代码**（除非后续显式审批调整 `R05` 契约文档）；灰度环境中顺带排查出的独立运维隐患（见第 7 节）**与本次 `ai-proxy 2.0.94` 彻底解耦**。

### 1.3 与 `canonical-ir` 长期重构的演进衔接

- 本次定位为基于当前主干基线 **`plugin-server` `dev` (`a2036de2` / `v2.2.1-sp004` / `ai-proxy 2.0.93`)** 的 **`2.0.94` 定点修复**。
- 长期架构方案 [ai-proxy 强类型 IR 架构分析与实施方案](2026-11-18-ai-proxy-canonical-ir.md)（v3 草案，代码尚未实施）后续在 `dev` 之上启动开发时，**必须完整继承本文档确立的兼容矩阵、字段分级（`(A)` 硬红线 / `(B)` 保真转换 / `(C)` 安全忽略降级）与 `ai-proxy` 本地错误契约**，并同步并入其 `behavior-deltas.md` 与 `P1–P3` 测试基线。

---

## 2. 故障背景与实证矩阵

### 2.1 灰度 `172.16.31.200` 实证与对照组结论

在灰度环境（`V2.2.1-SP004` / `ai-proxy:2.0.93`）针对客户端报错：

```text
Reason: Failed to get response from the AI model; retried 5 times (total retry wait time: 16.11 seconds)
Last error: 502 Request Failed: 400 {"error":{"code":"server_error","message":"The service could not complete the request. Try again later.","type":"api_error"}}
```

完成网关访问日志、Envoy 配置与控制面数据库全链路交叉印证：

1. **3 批 18 次 100% 复现**：`POST /v1/responses`、`ua=GitHubCopilotChat/0.74.0` 全部由 `ai-proxy:2.0.93` 本地拦截返回 `HTTP 400`、`bytes_sent=125`、`rcd=via_wasm::...::ai-proxy.managed_execution`、`upstream_host="-"`、`reason_codes=["unsupported_or_invalid_protocol_request"]`，未转发上游：
   - `rel-01a1efda-e354-73a0-88ec-eb388258d3b6-text-responses`（`qwen-plus`，`openai_responses -> openai_chat`）：`2026-11-19T05:28:28.595Z–05:28:46.700Z`，1 次首请求 + 5 次重试，`rx=167673`。
   - `rel-01a1efd7-3a18-7e42-9697-a5509fa10113-text-responses`（`qwen-flash`，`openai_responses -> dashscope_text`）：`2026-11-18T18:14:36Z–18:14:54Z`（6 次）及 `18:15:35Z–18:15:53Z`（6 次），`rx=167179`。
2. **双状态码与 5 次重试成因**：`GitHubCopilotChat/0.74.0` 在客户端侧将网关返回的 `400 {"error":{"code":"server_error","type":"api_error",...}}`（正好 `125` 字节）识别为可重试服务端错误（`api_error`/`server_error`），退避重试 5 次耗时约 `16.11s` 后，由客户端自身错误包装器抛成 `502 Request Failed: 400 ...`（工作区源码对 `"502 Request Failed:"` 与 `"retried ... times"` 均为零命中）。
3. **排除 167KB Body 缓冲/大小截断**：
   - **同体积同协议直通对照组**：同为 `GitHubCopilotChat/0.74.0`、同为 ~167KB，在 `rel-01a1efdc-...-text-responses`（`qwen3.5-flash`，`openai_responses -> openai_responses`）上 `rx=167185` 返回 `200`（`upstream_path=/compatible-mode/v1/responses`）；在 `rel-01a1efda-...-text-chat`（`/v1/chat/completions`）上 `rx=167303` 返回 `200`。
   - **其他大包基准**：`rel-01a1ea17-...-text-responses` 上 `rx=1,024,439` 仍正常 `200`；Envoy 无 `envoy.filters.http.buffer` 截断，`ai-proxy` 默认 `defaultMaxBodyBytes = 100 MiB`（`main.go:28`）。
4. **控制面路由编译分叉根因**：7 条 `enabled=true` 路由中共有 2 条在 `provider_model_bindings.execution.choices` 中关闭 `inherit_provider` 并移除了 `"openai_responses"` 候选（`bailian-qwen-plus` rev 8 仅保留 `["openai_chat"]`；`bailian-qwen-flash` rev 10 仅保留 `["dashscope_text", "openai_chat", "anthropic_messages"]`），被迫降级进跨协议转换而命中 `responses.go` 严格校验；其余 5 条（含 `rel-01a1efdc`）走原生直通不受影响。

### 2.2 7 大真实编程 Agent 客户端协议行为与当前 `ai-proxy:2.0.93` 命中对照

基于 `microsoft/vscode`（`0.74.0` `03d396a6`）、`openai/codex`、`badlogic/pi-mono`、`openclaw/openclaw`、`NousResearch/hermes-agent`、`BerriAI/litellm` 一手源码及本地 `provider/` 核实：

| 客户端 | 目标入口 | 典型高频请求与多轮特征 | `ai-proxy:2.0.93` 命中行为与源码位置 | 判定 |
| --- | --- | --- | --- | --- |
| **Copilot Chat `0.74.0`**（CAPI / BYOK） | `/v1/responses` | 默认必发 `truncation: "disabled"`/`"auto"`、`include: ["reasoning.encrypted_content"]`；BYOK 非 ZDR 第 2 轮+发 `previous_response_id`；回传 `reasoning` 含 `encrypted_content` + `summary: []`（无 `content`）；可选 `top_logprobs`、`context_management`、`tools[].type="tool_search"`、`input_file`；SSE 仅读 `reasoning_summary_text.delta` | `responses.go:222-227` 拒 `truncation`/`include`/`top_logprobs`/`context_management`；`228` 拒 `previous_response_id`；`352-366` 拒无明文 `reasoning`；`545` 拒 `tool_search`；`834` 输出 `reasoning_text.delta` | **首轮 100% 400**（`server_error`）；多轮 400；SSE 思维链不可见 |
| **OpenAI Codex CLI** | `/v1/responses` | 必发 `include`、`store: false`、`reasoning: {effort, summary, context}`、`service_tier`、`client_metadata`、`text.verbosity`、`stream_options.reasoning_summary_delivery`；多轮 `message` 带 `role:"developer"`/`id`/`status`/`phase`、`reasoning` 无明文 `content`；另含 `custom_tool_call(_output)`、`local_shell_call`、`compaction`、`tools[].type="custom"/"web_search"/"tool_search"` | `222-227` 拒 `include`/`service_tier`/`client_metadata`；`242-244` 拒 `reasoning.summary`；`352-366` 拒加密/摘要 `reasoning`；`468` / `545` 拒非 function item/tool | **首轮 100% 400**（L1/L2/L3 全命中） |
| **Claude Code** | `/v1/messages` | `anthropic-beta` 头；`system` block 数组（末块 `cache_control`）；`thinking: {type:"enabled", budget_tokens}` 或 `{type:"adaptive"}` + `output_config.effort`；`metadata.user_id`；`tool_result` 含文本+图片数组或空内容 | `claude_to_openai.go:330-353` 不处理 `adaptive` + `output_config.effort`；`232` `GetStringValue()` 丢 `tool_result` 图片 | `adaptive` 推理等级丢失；多模态工具返回丢图 |
| **Pi Agent (`pi-ai`)** | `/v1/responses`<br>`/v1/chat/completions`<br>`/v1/messages` | Responses 发 `include`、`reasoning.summary`、`service_tier`、`prompt_cache_retention: "24h"`、带 `id`/`status`/`phase` 的 `message`、`reasoning`；Chat 发 `developer`、`max_completion_tokens`、`store: false`、`reasoning_details`、`cache_control`、工具 `name`；Messages 发 `adaptive` + `output_config.effort` | Responses 首轮中 `include` / `reasoning.summary` / `service_tier`，Turn 2 中 `reasoning`；Messages 丢 `adaptive` 强度；Chat `store`/`max_completion_tokens`/`name` 透传到兼容上游 | Responses **100% 400**；Messages 行为漂移；Chat 视上游兼容度 |
| **OpenClaw** | 三协议（自研 `@openclaw/ai`） | 在 `pi-ai` 基础上对 `gpt-5*` 注入 `parallel_tool_calls: true`、`text.verbosity: "low"`、`service_tier`；自定义网关仍保留 `include` + `reasoning.summary` + `reasoning` 回传；Messages 含 `context_management` 与 `adaptive` | 同 Pi Agent；对 `max_completion_tokens`、`store: false`、Messages `adaptive` 与 Responses L1/L2/L3 行为一致 | Responses **100% 400**；Messages 丢 `adaptive` |
| **Hermes Agent** | 三协议 | Responses 发 `include`、`reasoning: {effort, summary:"auto", context:"all_turns"}`、`service_tier`、`context_management`、`prompt_cache_retention`、`compaction`、`web_search`；Chat 发 `role:"developer"`、`max_completion_tokens`、`prompt_cache_key`、`reasoning_details`、OpenRouter `cache_control`、`extra_body.*`；Messages 发 `adaptive` + `output_config.effort` + 含图 `tool_result` | Responses 全命中 L1/L2/L3；Chat 兼容模式 `qwen.go:136-160` 未清洗 `prompt_cache_key`/`cache_control`/`reasoning_details`/`extra_body` 等；Messages 丢 `adaptive` 与 `tool_result` 图片 | Responses **100% 400**；Chat 二次 400 风险；Messages 丢强度/图 |
| **LiteLLM Proxy**（作客户端/桥接上游） | 三协议 | Responses 透传 `include`、`truncation`、`service_tier`、`safety_identifier`、`max_tool_calls`、`top_logprobs`、`context_management`、`prompt`、`moderation`；**Chat 桥接**：将 Claude/Responses 转为 `role:"assistant", content: null` + `reasoning_content` + `thinking_blocks` + `messages[].cache_control` + `tools[].cache_control` + `max_completion_tokens` + `tool_choice:"required"` | Responses 首轮命中 L1；`openai.go` 与 `qwen.go:136-160` 兼容模式**原样透传 `thinking_blocks` / `cache_control` / `content:null` / `store`**；`dashscope_text` `461` 透传 `tool_choice:"required"` 且 `854,858` 丢显式 `0` 采样值 | Responses **100% 400**；Chat 桥接字段引发严格上游二次 400 |

### 2.3 42 份样本离线确定性红测实证

基于 `modelfusion-deploy/scripts/test-ai-agent-multiturn.py`（含 `copy.deepcopy(p1)` Turn 1/Turn 2 彻底隔离，单元测试 **93/93 OK** / `test_ai_agent_multiturn.py` **14/14 OK**）导出 **7 画像 × 3 协议 × 2 轮 = 42 份离线样本**，直接输入当前 `ai-proxy:2.0.93` Go 转换函数实测结果：

1. **`/v1/responses`（`ConvertResponsesRequestToChatCompletionWithOptions`）**：7 个非 `generic` 画像中除 `claude-code` 外其余 **6 个画像（`copilot-0.74`、`copilot-byok-0.74`、`codex-cli`、`pi-openclaw`、`hermes-agent`、`litellm-proxy`）100% 稳定复现失败**：
   - **L1（首轮 & 第 2 轮 100% 命中）**：6/6 命中 `include` / `truncation` / `service_tier is unsupported in Responses fallback mode`。
   - **L2（剥除 L1 顶层元字段后）**：`codex-cli`、`pi-openclaw`、`hermes-agent` 100% 命中 `reasoning summary is unsupported in Responses fallback mode`。
   - **L3（再剥除 `reasoning.summary` 后）**：6/6 画像 Turn 1 均变为通过，但 **6/6 画像 Turn 2 100% 命中** `opaque or summary-only reasoning is unsupported in Responses fallback mode`。
2. **`/v1/messages`（`ClaudeToOpenAIConverter.ConvertClaudeRequestToOpenAIWithOptions` + `stripClaudeInternalMessageFields`）**：
   - `pi-openclaw`、`hermes-agent`、`litellm-proxy` 第 1/2 轮 **100% 复现 `adaptive` 静默丢失**：输入 `thinking: {"type": "adaptive"}` + `output_config: {"effort": "medium"}`，转换产出 `reasoning_effort: ""`，随后 `claude_output_config` 被剥离导致推理强度彻底丢失（且 `cache_control` / `metadata` 均被丢弃）。
3. **`/v1/chat/completions`（`qwenProvider.TransformRequestBodyHeaders` 在 `qwenEnableCompatible=true` 下对比）**：
   - `pi-openclaw` 透传残留 `store`、`max_completion_tokens`、`role:"tool".name`；
   - `hermes-agent` 透传残留 `prompt_cache_key`、`content[].cache_control`、`msg.reasoning_details`；
   - `litellm-proxy` Turn 2 透传残留 `tools[].cache_control`、`msg.thinking_blocks`、`msg.cache_control`、`msg.content: null`。

---

## 3. 五大修复模块（A–E）详细技术方案

### 模块 A：`provider/responses.go` 请求侧转换（`openai_responses -> openai_chat / dashscope_*`）

- **目标文件**：`plugin-server/plugins/wasm-go/extensions/ai-proxy/provider/responses.go`（`convertResponsesRequestToChatCompletionWithOptions:217-598` 及请求结构体 `39-215`），关联调用点 `managed_protocol.go:154-157`。
- **完整拒绝分支与字段分级矩阵**：

| 源码位置与条件 | 当前行为 | 分级 | 修复后目标行为与实现要点 |
| --- | --- | --- | --- |
| `219-221` 外层非 JSON Object；`236-238` 类型错误；`301-303` 非法 `text.format.type` | 拒绝 | **(A) 硬红线** | 保留拒绝（合法性底线）。 |
| `228-233` `previous_response_id` / `conversation` 存在即拒；`239-241` `background: true` | 拒绝 | **(A)/(B)** | `previous_response_id` / `conversation` **仅在值非 `null` 且非空字符串 `""` 时拒绝**（修复当前 `gjson.Exists()` 误拒 `:null`）；`background: true` 继续硬拒绝（`background: false` / `null` 放行）。 |
| `222-227` 顶层白名单拦截：`include`、`truncation`、`service_tier`、`safety_identifier`、`max_tool_calls`、`context_management`、`client_metadata`、`top_logprobs`、`logprobs`、`prompt_cache_key`（已放行）/ `prompt` / `moderation` | 白名单外直接报错 `unsupported` | **(C)/(B)** | 1. **安全忽略 (C)**：`truncation`、`include`、`service_tier`、`safety_identifier`、`max_tool_calls`、`context_management`、`client_metadata`（均属无状态运行提示/客户端策略头，不改变语义，Chat 上游本不执行服务端截断或生成密文）；<br>2. **按 (B) 转换**：`top_logprobs`（整数 `>0`）可按选项映射为 Chat `logprobs: true, top_logprobs: N`（不支持时按配置忽略）；`stream_options` 允许合法扩展子键反序列化忽略；<br>3. **`prompt`（Hosted Prompt 模板 ID）与 `moderation`**：保持 **(A)** 拒绝（无法无状态等价展开）。 |
| `242-244` `reasoning.summary != nil \|\| reasoning.generate_summary != nil`；新增 `reasoning.context` | 直接拒绝 `reasoning summary is unsupported` | **(C)** | 移除硬拒绝：忽略 `reasoning.summary` / `reasoning.generate_summary` / `reasoning.context`，保留 `reasoning.effort` 映射至 Chat `ReasoningEffort`（前提：`effort != ""`）。 |
| `352-366` `input[]` 中 `type: "reasoning"` 含 `encrypted_content` / `summary` 或缺明文 `content` | 直接报错 `opaque or summary-only reasoning is unsupported` | **(B)/(C)** | 1. 若同时含非空明文 `content[].type == "reasoning_text"`：提取文本为 `ReasoningContent` 并安全忽略 `encrypted_content` / `summary`；<br>2. 若仅有 `encrypted_content`（不透明密文）且/或非空 `summary[].text`（摘要）而无明文 `content`：优先将 `summary` 文本拼接（或若均为空密文且同轮随后有 `message`/`function_call`，则在 fallback 模式下**优雅跳过该仅密文 reasoning item**），确保同轮及后续 `assistant message` / `function_call` 不被整条拒死；仅当 `reasoning` 结构畸形时才报错。 |
| `416-428, 377-384, 441-467, 472-474` 工具配对、`refusal` 混用、合法 `role` | 拒绝 | **(A) 硬红线** | 继续保留（工具调用闭环与 role 合法性红线）。 |
| `479-493` `content` / `output` 为 `[]` 或 `null` | 直接拒绝 | **(A)/(B)** | `assistant` 空 `content: []` 或 `function_call_output.output` 为 `[]`（或合法空数组）在已存在前置/同轮语义时可容错为 `""`，畸形缺失仍按 **(A)** 拒绝。 |
| `507-520, 530` `input_file`、`file_id`、非 `user` 含 `input_image` | 拒绝 | **(A) 硬红线** | 保持 **(A)** 拒绝（无状态路径无法解析 `file_id`，且非视觉/tool 消息图片不在本轮放开）；明确提示安全字段键名用于日志定位。 |
| `468-470, 545-555` `item_reference`、`custom_tool_call`、`compaction`、`tools[].type != "function"`（`web_search` / `tool_search` / `custom`） | 拒绝 | **(A) 硬红线** | 继续保留 **(A)** 拒绝（无状态不可展开、hosted/custom 工具无 Chat 等价执行环境）。 |
| `561-597` `tool_choice` 处理 | `"tool_choice": null` 在 `571` 行误拒 | **(B) 修复** | 修复 `tool_choice: null` 解析路径按未设置处理（与字段缺失一致）；`web_search` 等 hosted `tool_choice` 保持 **(A)** 拒绝。 |
| `315-316` 转出 Chat 专有字段（`store`、`prompt_cache_key`、`prompt_cache_retention`、`prompt_cache_options`） | 无条件输出到 `chatCompletionRequest` | **(B) 目标敏感处理** | 在 `ResponsesFallbackOptions` 中传入目标形态标志（或由 `managed_protocol.go:154-157` / `qwen.go` 消费端配合模块 D 处理）：转发至非 OpenAI 原生 Chat 上游（如 `qwenEnableCompatible=true` / `dashscope_*`）时剥离 `store` 等仅 OpenAI Stateful API 识别的字段，避免上游二次 400。 |

---

### 模块 B：`provider/responses.go` 回程响应/SSE 转换与 `R05` 合同决策点

- **目标文件**：`plugin-server/plugins/wasm-go/extensions/ai-proxy/provider/responses.go`（`ConvertChatCompletionResponseToResponses:600-657`、`chatCompletionToResponsesStreamConverter:713-1076`）。
- **核心问题**：
  1. `responses.go:824-837` 与 `1023-1035` 将上游 Chat / DashScope 的 `reasoning_content` 转换为 SSE 事件 **`response.reasoning_text.delta`** / **`response.reasoning_text.done`**（`summary: []`），不发出 `response.reasoning_summary_part.*` 与 `response.reasoning_summary_text.delta`。
  2. `GitHubCopilotChat/0.74.0` 的 `OpenAIResponsesProcessor` 仅监听 `response.reasoning_summary_text.delta` / `response.reasoning_summary_part.done`，未监听 `response.reasoning_text.delta`（注：Codex CLI / Pi Agent / OpenClaw / Hermes 均已模糊兼容 `response.reasoning_text.delta`）。
- **【强契约冲突警告 — 严禁静默修改】**：
  - [下游响应规范契约](../../../ocloudmodelfusion/docs/contracts/downstream-response-profiles.md#L90)（`R05` / `A-R05`）与[下游响应契约方案](../../../ocloudmodelfusion/docs/plans/2026-11-05-openai-downstream-response-contract.md#L86)（及[第 6.1 节](../../../ocloudmodelfusion/docs/plans/2026-11-05-openai-downstream-response-contract.md#L179-L180)）明确规定：**Chat `reasoning_content` 全文推理必须映射到 `content:[{type:"reasoning_text", text}]` 与 `response.reasoning_text.delta/done`，"全文不改名摘要"（`summary: []`）**。
  - 下游 `managed_router_projection.go:176-210`、`pkg/ai/clientresponse/response.go:226`（`summary_index`）、`pkg/ai/clientresponse/content.go:17`（`summary_text`）、`managed_metering.go:263-266`、`managed_native_stream.go:63` 与 `ai-cache/text_output.go:111` 在代码层面已同时支持 `reasoning_text.*` 与 `reasoning_summary_text.*`，**无计费与观测断链风险**；但直接将默认输出改为 `reasoning_summary_text.delta` 会违背 `R05` 并打破现有单测 `responses_reasoning_test.go:41-46`、`managed_multiturn_agent_test.go:182` 与 `managed_dashscope_conversion_test.go:210`。
- **模块 B 方案对比（供用户拍板，详见第 5.5 节决策清单）**：
  - **方案 B1（默认严守 `R05`，推荐作为稳妥默认）**：默认继续输出 `response.reasoning_text.delta/done` + `summary: []`，**零破坏 `R05` 合同与现有 3 组单测**；对于 `GitHubCopilotChat` 的已知差异，要么在备注/契约中确认仅影响 Copilot UI 思维链预览（不阻塞请求 `200` 与 `output_text`/`tool_calls`/计费），要么仅在显式配置或特定兼容性策略开启时按兼容模式输出 `summary_text`。
  - **方案 B2（升级/修订 `R05` 契约，或镜像输出 `reasoning_summary_text.delta`）**：若用户明确拍板要求 Copilot Chat 在 `/v1/responses` 跨协议路由上实时展示思维链，须先修订 `ocloudmodelfusion` 下 `R05` 合同文件，并同步更新 `responses_reasoning_test.go:41-46`、`managed_multiturn_agent_test.go:182`、`managed_dashscope_conversion_test.go:210` 断言。
- **其他非冲突 SSE 细节确认**：`function_call` 合成 `item_id="fc_<id>_<idx>"`（`responses.go:871, 880`）、多 Choice 限制（`managed_protocol.go:593-599`）与无 `finish_reason` 提前 `[DONE]` 时注入 `response.failed`（`991-993` + `managed_router_projection.go:141-143`）均符合 E05 契约，保持不变。

---

### 模块 C：`provider/claude_to_openai.go` & `provider/claude.go`（`anthropic_messages -> openai_chat / dashscope_*`）

- **目标文件**：
  - `plugin-server/plugins/wasm-go/extensions/ai-proxy/provider/claude_to_openai.go`
  - `plugin-server/plugins/wasm-go/extensions/ai-proxy/provider/claude.go`
- **具体修复点**：
  1. **`thinking: {type: "adaptive"}` + `output_config.effort` 映射（`claude_to_openai.go:330-353`）**：
     - 现状：仅 `claudeRequest.Thinking.Type == "enabled"` 时按 `BudgetTokens` 计算 `openaiRequest.ReasoningEffort`；面对 Claude Code / Pi / OpenClaw / Hermes / LiteLLM 高频发送的 `{"thinking": {"type": "adaptive"}, "output_config": {"effort": "low|medium|high|max"}}`，`ReasoningEffort` 保持 `""`，而 `claude_output_config` 随后会被 `stripClaudeInternalMessageFields`（`provider.go:1367`）剥离，导致推理强度彻底丢失（注意：`qwen.go:412` 虽然在直接转 `dashscope_text` 时读到了 `ClaudeThinking.Type == "adaptive"`，但 `openai_chat` 兼容路由仍丢失 `ReasoningEffort`）。
     - 修复：当 `claudeRequest.Thinking.Type == "adaptive"` 时，读取 `claudeRequest.OutputConfig.Effort`（映射规则：`low -> "low"`，`medium -> "medium"`，`high` / `max` -> `"high"`，未传 `OutputConfig.Effort` 时默认 `"medium"` 或由适配器策略决定）写入 `openaiRequest.ReasoningEffort`；同时保留 `ClaudeThinking` / `ClaudeOutputConfig` 供 `qwen.go` 与 `stripClaudeInternalMessageFields`（`provider_test.go:842-870`）原有契约不变。
  2. **`tool_result` 空内容与多模态图片边界（`claude_to_openai.go:228-237`、`claude.go:126-138`）**：
     - 现状：`toolResult.Content == nil` 时需防空指针（`232` 行当前先判空再调 `GetStringValue()` 或通过指针解引用），且数组型 `tool_result.content` 含 `type: "image"` 块时 `GetStringValue()` 仅拼接 `.Text`，静默丢失图片块。
     - 修复：显式判空 `toolResult.Content == nil` 输出 `""`；当 `toolResult.Content` 含 `type: "image"`（且下游目标/模型支持多模态或按 OpenAI 多段 user 注入约定）时，参照 Hermes / Pi 的成熟约定：文本留在 `role: "tool"`，`image` 块保留提示或按目标能力转换/拆出随附多模态消息（若目标明确不支持图片工具输出则返回显式结构化原因而非静默吞图，具体开关与模块 D 协同确认）。
  3. **`message_stop` 终态与异常截断契约（`claude_to_openai.go:481-489`）**：
     - 核查确认：当前仅在 `c.messageStartSent && !c.messageStopSent` 且收到上游 `[DONE]` 时发 `event: message_stop`；上游异常断流时**不伪造** `message_stop`（严格符合 E05/M06 契约），本次保持该正确行为不变并纳入回归验证。
  4. **`claudeTool` 扩展结构兼容（`claude.go:31-35, 234-251`）**：
     - 为请求结构补充识别 `claudeTextGenRequest.Metadata`、`ContextManagement`、`Speed` 与 `claudeTool.Type` / `CacheControl`，区分无害元数据（可忽略）与无等价执行环境的内置工具（显式拒绝而非静默丢失工具定义）。

---

### 模块 D：`provider/openai.go` & `provider/qwen.go`（Chat 桥接降噪与 `dashscope_text` 修复）

- **目标文件**：
  - `plugin-server/plugins/wasm-go/extensions/ai-proxy/provider/openai.go`（`245-256`）
  - `plugin-server/plugins/wasm-go/extensions/ai-proxy/provider/qwen.go`（`136-160`, `371-488`, `850-869`）
- **具体修复点**：
  1. **`dashscope_text` 显式零值丢失修复（`qwen.go:850-869`）**：
     - 现状：`qwenTextGenParameters.Temperature`（`858`）与 `PresencePenalty`（`854`）使用值类型 `float64` + `omitempty`，导致 `responses.go:307-327` 好不容易通过 `*float64` 保留的显式 `temperature: 0` / `presence_penalty: 0` 在第二跳被重新吞掉（违反 `RC-03` 与 `canonical-ir` `R03`）。
     - 修复：将 `qwenTextGenParameters.Temperature`、`PresencePenalty` 改为 `*float64`（若 `chatCompletionRequest` 侧相应字段仍为值类型或已有 `*float64`，同步保持 presence 语义不丢失）；同步核查 `Responses -> Chat -> DashScope` 全链路下显式 `0` 值保真。
  2. **`dashscope_text` `tool_choice: "required"` 与非支持参数处理（`qwen.go:461`）**：
     - 现状：`responses.go:568` 放行 `tool_choice: "required"` 并透传至 `qwenTextGenParameters.ToolChoice`（`461`），部分 DashScope 原生模型接口不接受字符串 `"required"`（尤其开启 `enable_thinking` 时），易触发上游 `400`。
     - 修复：按 DashScope 官方模型/能力约定校验 `enable_thinking` + 强制 `tool_choice` 冲突与 `"required"` 兼容性，不可支持的组合前置明确返回 `400 invalid_request_error`（不静默降级为 `"auto"`，与 `canonical-ir` `R04` 对齐）。
  3. **LiteLLM / Hermes `/v1/chat/completions` 桥接字段清理（`qwen.go:136-160` 兼容模式 & `openai.go:245-256`）**【需用户拍板边界】：
     - 现状：LiteLLM Proxy（跨协议桥接）及 Hermes Agent 发往 `/v1/chat/completions` 时会携带 `thinking_blocks`、`reasoning_details`、`messages[].cache_control`、`tools[].cache_control`、`role:"assistant", content: null`（仅有 `reasoning_content`）、`extra_body.*`、`store`、`reasoning_summary` 等。当前 `qwen.go` 兼容模式（`qwenEnableCompatible=true`）与 `openai.go` 在无 `responseJsonSchema` 时几乎原样透传上述字段（而 Qwen Native `qwen.go:1147-1171` 会清理并将 `content: null` 置 `""`）。
     - 修复策略（见第 5.5 节决策 2）：
       - 在 `qwenEnableCompatible=true`（及可控兼容目标）清理明确属于桥接伪影的非标准字段（`thinking_blocks`、`reasoning_details`、`extra_content`）及目标不支持的 `store` / `messages[].cache_control` / `tools[].cache_control`；对 `role:"assistant"` 且 `content: null` + 含 `reasoning_content` / `tool_calls` 的消息按上游兼容要求规范化为 `""`（避免严格上游报 `content` 类型错误）；
       - 在 `openai.go` 仅对**跨协议/托管兼容目标**按策略清理已知桥接伪影，**严禁在同协议 OpenAI 原生透传路径（`C06`）上剥离合法字段**。

---

### 模块 E：本地错误包络与脱敏字段级日志（零爆炸半径方案）

- **目标文件（严格限定在 `extensions/ai-proxy/` 内）**：
  - `plugin-server/plugins/wasm-go/extensions/ai-proxy/managed_execution.go`（`721-724`, `1518-1528`）
  - `plugin-server/plugins/wasm-go/extensions/ai-proxy/managed_response.go`（`72-74` 及新增错误映射辅助）
  - `plugin-server/plugins/wasm-go/extensions/ai-proxy/managed_protocol.go`（`186-189`）
- **为什么严禁改动 `pkg/ai/clientresponse/*.go`（核心架构红线）**：
  1. `pkg/ai/clientresponse/local_reply_test.go:249-253`（U08 门禁测试）**明文禁止**把 `"ai-proxy.managed_execution"` 放入 `LocalReplyKindForDetail`；
  2. 工作区实查显示共有 61 个文件引用 `pkg/ai/clientresponse`，改写共享包会强制 ~10+ 个兄弟 Wasm 插件的 `.wasm` SHA256 发生变化，若未全量同步升版所有兄弟插件并在发布目录登记，将在 `scripts/package-plugins.py:118-120` 触发 `StorageConflict("version_conflict")`；
  3. 本身只有 `ai-proxy` 的 `managedPublicError` 错误地把“自己生成的拒绝 reason 字符串”塞进无 HTTP Status 的 `clientresponse.Error`（`response.go:352-354` -> `ClassifyFailure` `failure.go:61,126,188` 兜底成 `{500, "api_error", "server_error", "The service could not complete the request. Try again later."}`），**根因完全在 `extensions/ai-proxy/` 内部**。
- **零爆炸半径实现设计**：
  1. **显式本地错误包络映射（`managed_response.go` + `managed_execution.go:1518-1528`）**：
     - 改造 `managedPublicError(ctx, status, reason)`（不再经 `clientresponse.Error` 对未映射 `reason` 落入 `FailureServer`）：按 `(status, reason)` 与当前 `managedPublicProtocolKey` / `managedPublicProfileKey`（`managed_execution.go:543-544` 已正确采集）构造精准本地错误 JSON：
       - **协议/请求参数类 `400`**（`unsupported_or_invalid_protocol_request`、`unsupported_text_request`、`json_body_required`、`invalid_json_body`、`mapped_model_differs_from_selection`、`invalid_stream_flag`、`invalid_effective_tool_request`、`unsupported_or_invalid_supplier_request`、`text_execution_body_invalid`、`invalid_fixed_tools` 等）-> HTTP `400`、`error.type = "invalid_request_error"`、`error.code = "invalid_parameter"`（或明确子码）、脱敏固定安全 `message`（例如明确提示参数/协议不受当前执行路径支持，不回显用户输入内容）。
       - **资源引用类 `400`**（`resource_reference_unavailable`、`resource_reference_incompatible` 等）-> HTTP `400`、`error.type = "invalid_request_error"`，**严格保留原 `error.code == "resource_reference_unavailable"` / `"resource_reference_incompatible"`**（满足 `managed_public_resources_test.go:207-208`）。
       - **`413`**（`request_body_out_of_bounds` / `mapped_request_body_out_of_bounds`）-> HTTP `413`、`error.type = "invalid_request_error"`、`error.code = "request_too_large"`。
       - **`403`**（`authenticated_identity_missing` / `current_execution_admission_required` / `execution_admission_expired` / `tool_not_open_*` 等）-> HTTP `403`、`error.type = "permission_error"`、`error.code = "permission_denied"`。
       - **`500 / 502 / 503 / 504`**（`accounting_delivery_unavailable` 等）-> `error.type = "api_error"`、`error.code = "service_unavailable"` / `"server_error"`。
     - **多协议响应形状与安全头（对齐 E06）**：
       - OpenAI Chat / Responses 客户端输出 `{"error":{"message":...,"type":...,"param":null,"code":...}}`（可依 `param` 仅返回安全的脱敏结构字段名，如 `"previous_response_id"`，不带用户值）；
       - Anthropic Messages 客户端输出顶层 `{"type":"error","error":{"type":...,"message":...}}`；
       - 在 `rejectManagedRequest`（`managed_execution.go:1521`）响应头中补充 `x-request-id`（取 `callID` / 请求追踪 ID）及针对不可重试 4xx 的 `x-should-retry: false`，彻底杜绝客户端把我方 400 当成 `500/502 server_error` 盲目重试 5 次。
  2. **脱敏结构化字段级 Warn 日志（`managed_protocol.go:186-189` + `managed_execution.go:721-724`）**：
     - 严禁直接打印原始 `err.Error()`（部分底层 JSON/解码错误可能附带用户输入片段），也不在生产路径泄露请求体或凭据；
     - 让转换器与 `managed_protocol.go:186-189` 通过自定义类型化错误（如 `managedProtocolError{ReasonCode, SafeField}`）返回**固定枚举原因 + 纯 JSON 字段名/Item 类型**（如 `kind=unsupported_top_level_field field=include`、`kind=unsupported_input_item_type item_type=compaction`）；
     - 在 `managed_execution.go:721-724` 拒绝分支打印 `log.Warnf("ai-proxy managed protocol preparation rejected, call_id:%s input:%s upstream:%s reason:%s field:%s", ...)`，使网关日志与 Envoy `reason_codes` 可直接一一对应排障。

---

## 4. 跨仓影响范围、既有单测调整清单与发布回滚评估

### 4.1 跨仓源码与调用链影响

| 仓库 / 模块 | 影响状态 | 具体说明 |
| --- | --- | --- |
| `plugin-server`: `extensions/ai-proxy/` | **核心修改与重编** | 仅修改 `provider/responses.go`、`provider/claude_to_openai.go`、`provider/claude.go`、`provider/openai.go`、`provider/qwen.go`、`managed_protocol.go`、`managed_response.go`、`managed_execution.go`、`managed_log.go`（按需登记安全详情）及版本文件。 |
| `plugin-server`: `pkg/ai/clientresponse/` | **零修改** | 严格不改动，确保 61 个引用文件及兄弟插件不受任何影响。 |
| `plugin-server`: 其余 Go/JS 插件 | **零修改、零重编** | `scripts/package-plugins.py:67-77` 比对包内容忽略 `"source"`，仅 `ai-proxy` 产出新制品，其余 55 项正式插件自动复用已有制品，**无 `version_conflict`**。 |
| `ocloudmodelfusion`: Rust 控制面与 E1 | **零代码破坏**（仅 `R05` 视决策确认是否更新契约文档） | 核实 `console/src/release_runtime/e1_cases/media/scenarios.rs` 与 `internal.rs` 对 `"managed_execution_error"` 的断言**全部仅针对媒体/内部接口**；公开文本出口走 `managedPublicError`，计费、余额投影、SLS 补采与路由配置逻辑均无依赖冲突。 |
| `modelfusion-deploy`: 测试脚本与离线包发布 | **无 Chart 源码破坏** | `#48/#49` 已完成 `scripts/test-ai-agent-multiturn.py` 扩展（支持全部 7 画像）；后续 `ai-proxy 2.0.94` 构建完成后，按常规更新 `plugin-release-catalog.json`（参考 `offline/build-offline-bundle.sh:665-666`）并经原生发布切换即可。 |

### 4.2 必须同步审视/调整断言的既有单测清单

以下测试文件已核实与模块 A–E 存在直接断言关联，实施时需逐一按新语义更新或保留守卫：

| 测试文件与精确行号 | 当前断言内容 | 归属模块 | 处置方式 |
| --- | --- | --- | --- |
| `extensions/ai-proxy/provider/responses_fidelity_test.go:54-61` | 断言 `reasoning.summary` / `generate_summary` 返回错误 `unsupported` | 模块 A | **同步更新**：改为断言忽略 `summary`/`generate_summary` 并保留 `reasoning_effort` |
| `extensions/ai-proxy/provider/responses_history_test.go:115` | 断言 `{"type":"reasoning","encrypted_content":"opaque"}` 返回错误 `unsupported` | 模块 A | **同步更新**：改为断言优雅跳过不透明 reasoning item 或从 `summary` 提取 |
| `extensions/ai-proxy/provider/responses_converter_test.go:73-108` | 断言非空 `previous_response_id`、`conversation`、非 function tool、`file_id` 报错 | 模块 A | **保留不变**（硬红线；新增 `:null` / `"tool_choice": null` 正向放行用例） |
| `extensions/ai-proxy/provider/responses_reasoning_test.go:41-46` | 断言 SSE 包含 `response.reasoning_text.delta` | 模块 B | **视 `R05` 拍板**：若选 B1 则**保持通过**；若选 B2 改契约则同步调整 |
| `extensions/ai-proxy/managed_multiturn_agent_test.go:182` | 断言 `ConvertResponse` 包含 `response.reasoning_text.delta` | 模块 B | **视 `R05` 拍板**：同上 |
| `extensions/ai-proxy/managed_dashscope_conversion_test.go:210` | 断言 DashScope -> Responses SSE 包含 `response.reasoning_text.delta` | 模块 B / D | **视 `R05` 拍板**：同上；另增显式 `0` 值与 `tool_choice` 回归用例 |
| `extensions/ai-proxy/provider/provider_test.go:842-870` | `TestStripClaudeInternalMessageFields` | 模块 C | **保留不变**（确保新 `adaptive` 映射不破坏旧内部字段清理） |
| `extensions/ai-proxy/provider/qwen_test.go` & `claude_to_openai_test.go` | Qwen 兼容/DashScope 参数转换与 Claude->OpenAI 转换断言 | 模块 C / D | **扩展补强**：覆盖 `adaptive`、`tool_result` 多模态/nil、`Temperature=0`、桥接字段清洗 |
| `extensions/ai-proxy/managed_public_resources_test.go:207-208` | 断言 `400` 且 `error.code == "resource_reference_unavailable"` | 模块 E | **必须保持通过**（显式映射保留 `resource_reference_*`） |
| `extensions/ai-proxy/managed_router_reentry_test.go:228` | 断言非法重入返回 `403` | 模块 E | **必须保持通过** |
| `extensions/ai-proxy/managed_execution_test.go:606` | 断言前置拒绝产生 `ActionPause` + local response | 模块 E | **必须保持通过** |
| `pkg/ai/clientresponse/local_reply_test.go:249-253` | U08 禁止将 `"ai-proxy.managed_execution"` 加入 `LocalReplyKindForDetail` | 模块 E | **必须保持通过**（零修改 `clientresponse` 即天然满足） |
| `extensions/ai-proxy/managed_public_error_matrix_test.go` & `managed_public_failures_test.go` | 托管公开错误矩阵及上游失败透传映射测试 | 模块 E | **核查并扩展**：确认上游真 5xx 仍走 `ClassifyFailure`，本地 4xx 改走明确 `invalid_request_error` + `x-should-retry: false` |

### 4.3 安全、防泄密与发布/回滚链路评估

- **安全与防泄密**：模块 E 的结构化 Warn 日志只允许记录固定原因码与白名单/纯键名（`field` / `item_type`），严禁打印 `rawBody`、`err.Error()` 原文（以防携带请求正文碎片）或任何 `Authorization` / `api_key` 值；对外错误响应禁止包含内部供应商名称、上游路径、路由 ID 或栈信息。
- **发布版本与文件同步**：正式实施升版 **`2.0.93 -> 2.0.94`** 时，须同步更新：
  - `plugin-server/plugins/wasm-go/extensions/ai-proxy/VERSION`（当前 `2.0.93`）
  - `plugin-server/plugins/wasm-go/extensions/ai-proxy/option.yaml`（第 3 行 `version: 2.0.93`）
  - `plugin-server/plugins/wasm-go/extensions/ai-proxy/config/config.go`（第 17 行 `// @Version 2.0.93`）
  - `plugin-server/plugins/release/catalog.json`（第 1756 行 `"declaredVersion": "2.0.93"`）
- **无缝回滚**：发布目录与数据库注册保留 `ai-proxy 2.0.93` 原版不可变制品；若 `2.0.94` 发现回归，控制面直接将路由制品引用切回 `2.0.93` 并重新发布即可无缝回滚，无需重编其他插件或回滚数据库 schema。

### 4.4 用户拍板决策结论（2026-11-19 已确认并按此实施）

| 决策编号 | 议题 | 拍板结论 | 落地结果 |
| --- | --- | --- | --- |
| **D1（`R05` 合同）** | `/v1/responses` 回程 `reasoning` SSE 事件是否改名为 `response.reasoning_summary_text.delta` | **B1（严守 `R05`）** | 默认保持 `response.reasoning_text.delta/done`（`responses_reasoning_test.go`、`managed_multiturn_agent_test.go`、`managed_dashscope_conversion_test.go` 全绿）；同步在 `test-ai-agent-multiturn.py` 默认将 Copilot/Codex `missing_reasoning_summary_delta` 记为非阻塞 warning（显式 `--strict-copilot-reasoning-summary` 才判 FAIL）。 |
| **D2（Chat 桥接清洗边界）** | `qwen.go` 兼容模式与 `openai.go` 对 LiteLLM/Hermes Chat 桥接字段的清洗范围 | **D2-A（精准兼容清洗）** | 仅在跨协议（`OmitStore`）及 `qwenEnableCompatible=true`（`ApiNameChatCompletion`）/ `dashscope_*` 清洗桥接字段与 `store`/`content:null`，`provider/openai.go` 同协议 `openai -> openai` 直通完全不动。 |
| **D3（实施批次顺序）** | 五个模块（A–E）是合并单版 `2.0.94` 发布还是拆分验证 | **S1（TDD 合并单版发布）** | 按 Wave 1（模块 E + A）-> Wave 2（模块 C + D + 升版 `2.0.94`）顺序一次性合入 `dev` 提交 `abc7a34a`，仅重编并升级 `ai-proxy 2.0.94`。 |

---

## 5. 分阶段实施计划与 TDD 红-绿验收标准

### 5.1 实施阶段划分

| 阶段 | 范围 | 前置条件 | 核心验收门槛 |
| --- | --- | --- | --- |
| **Phase 0：基线冻结与 Red 测试前置** | 基于 `plugin-server` `dev` (`a2036de2`) 建独立开发工作树；导出 42 份离线红测样本；在 Go 测试中写入对应 Red 失败用例（覆盖 L1/L2/L3、`adaptive`、Chat 桥接残余字段、`Temperature=0`、`400 server_error`） | D1–D3 决策拍板 | Red 用例精确复现失败；基线现有单测全绿 |
| **Phase 1：模块 E（错误契约与脱敏日志）** | 实现 `managed_response.go` / `managed_execution.go` / `managed_protocol.go` 显式映射、响应头与安全 Warn 日志；不动 `clientresponse` | Phase 0 完成 | 非法/硬红线 400 均返回 `invalid_request_error` + `x-request-id` + `x-should-retry: false`；U08 与 `resource_reference_unavailable` 保持通过 |
| **Phase 2：模块 A & B（Responses 兼容与 SSE）** | 落实 `(A)/(B)/(C)` 字段分级、`null` 修复、`reasoning` 回放处理及 `R05` 决策落点 | Phase 1 通过 | 42 份样本中全部 6 个受阻 `/v1/responses` 画像（Turn 1 & Turn 2）在本地 Go 转换器 Turn 1/2 均通过；硬红线用例继续精准返回 `400 invalid_request_error` |
| **Phase 3：模块 C & D（Messages & Chat 兼容）** | 落实 `adaptive` + `output_config.effort`、`tool_result` 边界、`dashscope_text` `*float64` 零值、`tool_choice` 及桥接字段清理 | Phase 2 通过 | 42 份样本中 `/v1/messages` `adaptive` 保留推理强度；`/v1/chat/completions` 兼容模式无非标准桥接残留字段 |
| **Phase 4：升版构建与 `modelfusion-dev` 真机并发回归** | 同步 `VERSION`/`option.yaml`/`config.go`/`catalog.json` 至 `2.0.94`，编译 Wasm 与插件包（确认仅 `ai-proxy` 更新），部署 `modelfusion-dev`（`192.168.29.200`）并执行全画像并发回归 | Phase 3 通过且 Go/Python 全量测试通过 | 7 客户端画像 × 3 协议 × 多轮并发回归全部 PASS；`client_retry_risk_count == 0`；负向合同 PASS；控制面账务/计费零缺失 |

> **灰度环境推进纪律**：`172.16.31.200` 仅随后续正式发布窗口推进（严禁在未验收或开发阶段直推灰度）；灰度发布前可按需对 2 条受影响路由评估是否同时在控制面恢复其 `provider_model_bindings.execution.choices` 的原生直通候选。

### 5.2 可复制执行的验证命令清单

#### A. 离线红测样本导出与脚本自检（`modelfusion-deploy/` 目录执行）

```bash
# 1. 运行测试脚本单元测试（确保测试夹具自身健康）
../plugin-server/.venv/bin/python -m unittest discover -s scripts/tests -p "test_*.py" -v

# 2. 导出全部 7 个非 generic 客户端画像 × 3 协议 × 2 轮（共 42 份确定性离线载荷）
python3 scripts/test-ai-agent-multiturn.py \
  --client-profile all \
  --protocols chat,responses,messages \
  --stream-modes stream \
  --context-target-bytes 170000 \
  --dump-sample-payloads /tmp/ai-agent-samples
```

#### B. `plugin-server` Go 全量回归与单包构建（`plugin-server/plugins/wasm-go/` 目录执行）

```bash
# 1. 运行 ai-proxy 与 clientresponse（守卫 U08）全量单元测试
go test -v ./extensions/ai-proxy/... ./pkg/ai/clientresponse/...

# 2. 仅构建 ai-proxy Wasm 产物并校验版本目录
make build-plugin PLUGIN=ai-proxy
```

在 `plugin-server/` 根目录校验单包打包无版本冲突（正式提交前）：

```bash
python3 scripts/package-plugins.py prepare --plugin ai-proxy
```

#### C. `modelfusion-dev`（`192.168.29.200`）真机跨协议并发回归（`modelfusion-deploy/` 目录执行）

> 说明：测试前先从本地非入库秘钥文件读取网关密钥（如 `<API_KEY_FILE>`，严禁写入文档或日志），并确认参考环境配置 [本地测试环境与部署流程](../../../modelfusion-deploy/docs/local-test-environment.md)。

```bash
python3 scripts/test-ai-agent-multiturn.py \
  --base-url https://api.ai.lc.wooveep.com \
  --api-key-file <API_KEY_FILE> \
  --ssh-target root@192.168.29.200 \
  --client-profile all \
  --protocols chat,responses,messages \
  --stream-modes stream,nonstream \
  --include-thinking-variant \
  --include-negative-contracts \
  --expect-upstream-paths "responses=/compatible-mode/v1/chat/completions" \
  --context-target-bytes 170000 \
  --turns 3 \
  --concurrency 4 \
  --output /tmp/modelfusion-dev-compat-regression.json
```

**真机验收通过硬条件**：
1. 正向会话：`failed_sessions == 0`、`client_retry_risk_count == 0`（无任何 `400` 携带 `server_error` / `api_error` 或缺失 `x-should-retry: false`）；
2. 负向合同（`--include-negative-contracts`）：全部不可转换硬红线（非空 `previous_response_id`、`conversation`、`hosted tool`、`file_id` 等）均返回 `400` 且 `error.type == "invalid_request_error"`；
3. `--expect-upstream-paths` 校验确认流量确实经过跨协议转换路径而非误走原生直通；
4. 协同监控（Gateway / Console）确认零插件 panic/OOM、每条成功请求均有唯一对应计费观测记录。

---

## 6. 风险与阻塞

1. **决策依赖阻塞**：`R05` 合同是否修订（D1）与 Chat 桥接字段清理边界（D2）需用户在进入 Phase 2/3 编码前确认。
2. **多目标上游差异**：不同百炼/第三方兼容模型对 `store`、`prompt_cache_*`、`tool_choice: "required"` 与多模态 `tool` 消息的容忍度不一致，必须在真机回归中覆盖 `openai_chat`（兼容模式）与 `dashscope_text` 两类真实上游路径。
3. **发布窗口内存水位**：结合灰度排查实证，若单次全量并发重载多个路由的 `WasmPlugin`，Envoy V8 VM 重建峰值会显著抬高内存（见第 7 节），正式发版建议避开高并发流量峰或提前治理网关资源限额与 VM 挂载规模。

---

## 7. 附录：排查与真机回归顺带发现的独立平台/运维隐患（与本案解耦，另立专项）

以下问题在排查灰度 `172.16.31.200` 及开展 `modelfusion-dev` 真机 E2E 时顺带实证发现，**均与本次 `ai-proxy 2.0.94` 协议转换及 `400 server_error` 修复零代码耦合**，建议另立专项并行治理：

| 序号 | 隐患领域 | 实证现象与根因定位 | 建议专项治理方向 |
| --- | --- | --- | --- |
| **O-01** | 网关 `kube::Api::exec` 进程泄漏 | `ocloudmodelfusion/sdk/src/kubernetes/capture.rs:120-146` 调用 `curl` 采集 Envoy `/config_dump` 时未设 `--max-time`；当 30s exec 超时断开后，容器内 `curl` 写满管道永久阻塞在内核 `pipe_write` 并悬停 Envoy 连接缓冲（OOM 前累积 4 个，当前仍驻留 `11h49m`）。 | 为 `capture.rs` 中所有 `curl` 采集命令补充 `--max-time 25`（或统一 timeout 包装）。 |
| **O-02** | `console-api` 空载 ~34% CPU 与 PG 高频扫表 | ① Prometheus 每 15s 抓 `/internal/observability/metrics`（`observability_status/metrics.rs:111-129`）触发全量账务关联扫描（`billing_revision_cost_reviews` 单窗 `seq_scan=28,588`）；② `projection_worker` / `email` / `balance_projection_worker` 1s 空轮询与 60s 全量 `native_domain_identity::sample` 事务回滚；③ `model_observations` / `bailian_sls` 偶发 >1s 慢 SQL。 | 调整指标抓取/预聚合缓存、拉长空轮询或改事件唤醒、优化 `model_observations` 临时表与 `bailian_sls` JSONB 查询索引。 |
| **O-03** | `higress-gateway` 5Gi OOM 风险与 LDS 14–18s 阻塞 | 两个 listener 均全量挂载全部 39 个 `WasmPlugin`（共 78 个 filter 槽位，对应 9 个 `.wasm` 共 `333.2MiB`），`lds.update_duration` P50=`14.2s` / P100=`18.0s`、`watchdog_mega_miss=71`；批量发布期间累计创建 306 个 V8 VM 且受 `drainDuration=600s` 延迟释放，前实例曾触及 `5Gi` OOMKilled（重启后峰值仍达 `3.20GiB`）。 | 评估提升 Gateway 内存限额、减少全量并发重发布批次、以及控制面按路由/域收敛 ECDS/LDS WasmFilter 挂载粒度。 |
| **O-04** | Prometheus 存储与内存余量告警 | `higress-console-prometheus` PVC 声明 `1Gi` 实际已用 `1483.8 MiB`（日增 `75–184 MiB/天`，设 `retention.time=15d` 但 `retention.size=0B`）；内存 Limit `768Mi` 历史 `memory.peak=736.4Mi`（`95.9%`）。另有 `higress-console-loki` 缺内存 limit 及历史未绑定旧目录待清理。 | 扩容 Prometheus PVC 并设 `--storage.tsdb.retention.size` 与合理内存 Limit，补齐 Loki 内存 Limit。 |
| **O-05** | `route` `disabled -> enabled` 未同步 bump `authorization_revision`（`console` 缺陷） | `modelfusion-dev` 真机测试中在既有 v1 租户（`01a1f312-4332-76d2-be7d-0e01ccdad363` / 账户 `01a1f312-4333-74b0-9e4b-8268d7121359`）已授权状态下启用 `qwen-flash`，因未同步递增 `identity_assignments.authorization_revision`（仍为 `7`），触发 `mappings.lua` 在相同 `(mapping_revision=7, admission_state_revision=7)` 下发现 `authorized_routes` 变为 3 条并判 `conflict()`，将账户在 Redis 置为 `projection_state=blocked`（网关返回 `503 ai-quota.projection_unready`），并在 PG 残留 1 条 pending `wallet_balance_outbox`（`068b9061-8e0a-4cd8-8717-9cec980d7fc0`）与 1 条 `balance_projection_dirty`（`authorization_revision=13, dirty_revision=8`）证据。 | 修复 `ocloudmodelfusion` 在切换路由 `enabled` 状态时对受影响授权关系同步 bump `authorization_revision` 并刷新 Redis 投影，另立 Console 缺陷单修复验收。 |

---

## 进展记录

| 日期 | 进展与证据 | 下一步 |
| --- | --- | --- |
| 2026-11-19 | 完成灰度 `172.16.31.200` 实证溯源、7 大客户端协议对照、[多轮 Agent 测试脚本](../../../modelfusion-deploy/scripts/test-ai-agent-multiturn.py)（93/93 单测通过）与 42 份离线确定性红测验证；定稿开发设计与影响评估。 | 提交用户评审决策 D1–D3。 |
| 2026-11-19 | 用户拍板 **D1 = B1（严守 `R05`）、D2 = D2-A、D3 = S1**；按 TDD 红-绿完成 Wave 1（模块 E + A）与 Wave 2（模块 C + D + `2.0.94`），合入本地 `plugin-server` `dev` `abc7a34a` 并构建 `V2.2.1-dev-20261119-aiproxy2094`。 | 部署至 `modelfusion-dev` 执行全矩阵真机回归。 |
| 2026-11-19 | `modelfusion-dev`（`192.168.29.200`）平滑升至 Helm rev 25 并完成全矩阵真机 E2E 与独立网关/计费复核（见下文完成记录），随后还原临时测试路由并清理临时测试资产；灰度 `172.16.31.200` 全程零触碰。 | 待后续完整版本评审合入 `main` 及发布窗口推进灰度；另立专项修复 **O-05** 及运维项 **O-01~O-04**。 |

## 完成记录

### 1. 源码提交、制品与镜像基线

- **本地提交**：`plugin-server` `dev` `abc7a34a05f3cb9d1fc34d3e2c6a032696b48163`（`fix(ai-proxy): cross-protocol agent compatibility and 400 error contract (2.0.94)`，共改动/新增 19 个文件；`pkg/ai/clientresponse/*.go`、`provider/openai.go` 与 `R05` SSE 事件命名零改动）。
- **Wasm 制品与目录校验**：固定容器 `golang:1.26.3-bookworm` 编译产出 `ai-proxy 2.0.94` `plugin.wasm`（`24,961,439` bytes，`sha256:8479cb1f1dc66836f104d307365056df09818b8f13ae819eaeaf9df446eb2cc6`，`artifact_id=c953ce19-d5de-4fa0-ba07-e334135882bd`，`content_digest=sha256:6e14f0e7843d26b3af7dedf61c1bab622de2de821a66af77555210a919d4a5a9`）；`verify-release-store.py` 校验 55 项正式目录零 `version_conflict`，原 `ai-proxy 2.0.93` 制品保留供回滚。
- **验证镜像与部署**：`registry.lc.wooveep.com:9980/modelfusion/plugin-server:V2.2.1-dev-20261119-aiproxy2094`（`sha256:76ba8d933b067db7875dc9a4d24eb5dfbb5facb18da648c1a1e5c105b7c5d522`）部署至 `modelfusion-dev` Helm Revision `25`（`12/12` Pod Ready，`artifact_catalog_entries` `139/139` `registered`）。

### 2. 离线验证证据

- **Go 单元测试**：`extensions/ai-proxy` `go test -count=1 ./...`（`ai-proxy`、`config`、`provider`、`util`）、`pkg/ai/clientresponse` `go test -count=1 ./...` 及 `plugins/wasm-go` 版本对齐契约测试全绿。
- **离线样本红->绿复测**：`42/42` 导出样本（7 客户端画像 × 3 协议 × 2 轮）跨 `ConvertResponsesRequestToChatCompletionWithOptions`、`ClaudeToOpenAIConverter` + `StripClaudeInternalMessageFields`、`NewManagedAdapter.Prepare`（Qwen compatible 与 DashScope 原生）全部转绿；`6/6` 负向硬红线契约精确返回结构化 `ResponsesFallbackError`。
- **Python 测试脚本**：`modelfusion-deploy` 全量单测 `93/93` PASS（含 `test_ai_agent_multiturn.py` `14/14` PASS，按 `D1=B1` 将 `R05` `missing_reasoning_summary_delta` 默认归为非阻塞 warning）。

### 3. `modelfusion-dev`（`192.168.29.200`）真机 E2E 与独立复核证据（#81 / #82）

- **167KB+ `GitHubCopilotChat/0.74.0` 复现对照**：`qwen-plus`（`openai_responses -> openai_chat`）`copilot-byok-0.74` 在 clean v2 租户下 **2/2 PASS**（stream & nonstream，单包 `bytes_received=176,233–180,682`，彻底消除原 `400 server_error` / 5 次重试）。
- **`qwen-plus -> openai_chat` 全画像矩阵**：**42/42 PASS + 5/5 负面契约 PASS**（7 画像 × 3 协议 × 2 模式，`concurrency=4`，`cum_tokens=983,435`）。
- **`qwen-flash -> dashscope_text` 全画像矩阵**：**30/30 PASS + 5/5 负面契约 PASS**（5 画像 × 3 协议 × 2 模式，`concurrency=3`，`cum_tokens=782,716`，含 3 条非阻塞 `R05` `codex_missing_reasoning_summary_delta` warning）。
- **`glm-5.3` 原生直通对照**：**3/3 PASS**（`responses/chat/messages`，`cum_tokens=43,773`）。
- **网关、日志与计费独立复核**：
  - 负面契约 `10/10` 在网关本地返回 `HTTP 400 invalid_request_error` + `x-should-retry: false`；`ai-proxy` 输出 4 条 `[warning][wasm]` 日志仅包含 `call_id`、`input`、`upstream`、`reason`、`code` 与安全字段名（`background`/`tool_search`），**零用户正文或凭据泄露**；
  - `model_execution_telemetry`（`attempt=228, call=238`）与 `billing_fact_observations`（`call_delivery=228 accepted, upstream_attempt=228 accepted`）100% 对齐、`fact_gaps=0`；
  - `billing:events` `pending=0, lag=0`、`lds/cds/rds update_rejected=0`、`0 Wasm crash`；
  - 基线还原与清理已核对（临时测试路由/租户/Key/用户均已退场还原，仅保留真实复现 Console 缺陷 **O-05** 的 v1 账户现场证据；灰度 `172.16.31.200` 未做任何改动）。
