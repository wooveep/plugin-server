# Responses→Chat 字段与消息 ID 修复验收

2026-11-04。本文保留第一阶段隔离验收：完成[修复计划](../plans/2026-11-04-responses-chat-fidelity.md)的RC-01～05，当时未切换正式业务、未修改开发003工作目录或部署值。用户随后授权的合流与正式交付已完成：**2.0.35已切换43条业务路由并完成小批在线回归**，见[最新正式验收](../../../modelfusion-deploy/docs/checks/2026-11-04-responses-chat-repair-deployed.md)。以下版本及“未切换”描述均指第一阶段历史检查点。

## 修复行为

| 问题 | 修复后的行为 |
| --- | --- |
| text.format 丢失 | text/json_object 转对应 response_format；json_schema 将 name、description、schema、strict 嵌套到 response_format.json_schema。RawMessage 保留 schema 数值，不经过浮点重写 |
| 函数和并行约束丢失 | tools[].strict 转 tools[].function.strict；parallel_tool_calls 保留 true/false；未设置不补默认值 |
| 显式零值丢失 | 转换边界以指针序列化 temperature、top_p、presence_penalty、frequency_penalty，保留 0，不改变共享 Chat 类型的既有默认值 |
| 推理、存储及缓存参数丢失 | reasoning.effort 转 reasoning_effort；store 的 true/false、prompt_cache_key、prompt_cache_retention、prompt_cache_options 保留。消息级和文本块级缓存断点保留；有块级断点时不再合并文本块，避免改变边界位置 |
| 非流式消息缺少 ID | 文本和拒绝消息生成 `msg_` + 上游响应 ID；不同上游响应的 message ID 不同；原 function call 的 call_id、参数关联保持 |
| 无等价处理的选项被忽略 | background=true、推理摘要改为发送前拒绝；未知 text.format 类型也明确拒绝。background=false 保持同步执行 |

这些修复针对已登记的 Responses fallback 子集。图像/文件和函数结果历史的转换仍未覆盖；这是适配范围缺口。服务端状态续接、conversation 与托管搜索缺少等价 Chat 执行环境，按能力边界记录。模型、供应商自己的参数或权限限制不算项目实现缺陷。

## 源码与实际制品

- 稳定基线：`d7153c12` / ai-proxy 2.0.31；最终修复源码提交：`a61ef5330793fb16065136ad3833e93153b7c3bc`。
- 隔离候选包：`ai-proxy 2.0.31-responses.2`，包含完整 Wasm、schema、defaults 和文档，6 个 HTTP payload 均回读验证。
- Wasm SHA256：`d75059ff79dd39d53336b7f497d0048fcf54fcd7f4676293b4bde77aade58d87`。
- 先独立核对构建输入、包摘要及来源回执，再登记到本轮专用制品服务；未绕过来源校验。
- 仅对两条新建审计路由的 WasmPlugin URL 绑定候选制品。原生 publication 元数据仍是 2.0.31；**两个路由的 Envoy 本地缓存 Wasm 均与上述候选 SHA 一致**。本测试不冒充普通业务发布流程或全局交付。

当前主目录的四个修复/测试文件与上述候选源码逐字节相同。为方便后续合流，另保留[仅含这四个文件的补丁](2026-11-04-responses-chat-fidelity.patch)；不携带开发003的 C 批、其他用户改动、业务价册或部署值。

本地修复分支为 `codex/responses-chat-fidelity-20261104`。代码补丁对开发003工作树的 `d0541053` 执行 `git apply --check` 通过；仅检查，未修改该工作树。

## 回归结果

| 验证 | 结果 |
| --- | --- |
| 修复前定向测试 | 复现零值、工具约束、text.format 丢失及 message ID 缺失 |
| 2.0.31 修复候选的 provider/config/util | 188 顶层、741 含子用例通过项；0失败、0跳过 |
| Responses、SSE 及托管文本/协议/供应商/HTTP 定向回归 | 44 顶层、156 含子用例通过项；0失败、0跳过 |
| 合计 | **232 顶层、897 含子用例通过项** |
| 最终候选的受控真实网关出口 | 7 次 HTTP200：JSON Schema true/false、显式 0/false、JSON object、text、SSE、未设置参数、缓存选项与消息/文本块断点。独立 HTTP 收件逐字段符合预期，摘要与实际 attempt 事实一致 |
| 最终候选的真实 OpenAI gpt-6-luna | 请求级缓存 TTL 与块级断点、JSON Schema、temperature=0、strict 函数定义等组合请求 HTTP200，返回 `{"ok":true}`；非流式 message ID 存在 |
| 复用先前修复版本的真实回执 | `2.0.31-responses.1` 的 3 个 HTTP200：JSON Schema、真实 SSE、strict 函数调用。仅复用未改变的逻辑；不冒充在最终 .2 版本重复发送 |
| 最终候选的本地明确拒绝 | 7 个合法但未支持的请求、1 个未知格式非法请求，均 HTTP400，未发送上游 |
| 供应商参数限制 | 1 个请求被实际 OpenAI 服务以 unknown_parameter 拒绝块级 TTL，原生错误正文摘要未变；见下文 |
| 事实与账务关联 | 最终候选的 8 个成功调用对应 16 条 accepted 事实与 16 笔账务应用；此处只核调用关联，不将专用 fixture 价册当作官方采购报价 |

最终候选在线共 **17 个样本：8 成功、8 本地明确拒绝、1 供应商参数拒绝**，另复用先前修复版本的 3 个真实回执。两个修复版本合计实际访问真实 OpenAI 5 次，4 成功、1 参数拒绝；未重复付费发送前三项已证明且逻辑未变的真实样本。受控服务只证明网关字段映射，不冒充模型遵循 schema 的证据。普通 SSE 的事件结构与终态通过；托管分支仍聚合后交付，不宣称实时逐 token 等价。

对 previous_response_id、conversation、托管搜索、后台执行和推理摘要的拒绝按当前无状态 Chat 范围解释；图像、完整函数历史按转换覆盖缺口解释。未知格式是请求校验用例。上述拒绝未计为实现失败，也未通过无提示丢字段来获得 HTTP200。

官方 [Chat Schema](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create)同样声明请求级 prompt_cache_options 和内容块缓存断点。本次实际上游却拒绝 `messages[0].content[0].prompt_cache_breakpoint.ttl`，返回 unknown_parameter；代理保留了合法输入字段及上游错误正文。去掉块级 TTL、由断点继承请求级 `ttl=30m` 后 HTTP200。该差异单列为供应商实际接口与文档的差异，不能称为 Chat 协议没有缓存选项，也不算项目实现缺陷。没有据本批短输入承诺实际缓存命中、写入或缓存费用验证。

## 环境与清理

临时真实模型路由使用本轮独立资产、绑定和 fixture 价册；既有价册保持，fixture 数值不代表 OpenAI 官方报价。设置阶段的缺价、命令键冲突单列在私有原件，均发生在模型调用前，不认作协议缺陷。历史投影异步到达导致部分瞬时新增 call_id 集合不适于逐案关联，最终使用独立 HTTP 收件的 call_id 与实际出站摘要核对。

两个版本各自的临时审计路由经产品 API 删除；首条未发布成功的临时路由及过渡绑定也已清理。新资产/绑定/供应商按产品语义禁用或软删除，保留历史引用。复查发现业务软删除仍保留活动 publication，后台曾重新生成运行资源；随后通过原生 `/releases/{id}/commands` 的 disable 操作关闭本轮4个发布的准入，再按唯一 release 前缀清理。清理后延时35秒复查，Ingress/WasmPlugin和专用制品Pod/Service/ConfigMap均剩余0，4个发布仍为disabled；保留历史发布记录，不写 SQL 或改动业务删除代码。本任务没有重部署共享服务或切换原业务路由；并行工作将全局 Helm 从68推进到70，清理阶段回读为70。

完整脱敏结果、call/attempt/release ID、实际出口参数、用量、逐案版本与清理回读见[机器回执](2026-11-04-responses-chat-fidelity.json)。原请求、响应头、登录及账本原件保留在本地忽略目录 `modelfusion-deploy/.local/responses-chat-fix-20261104/` 与其 `-v2/` 目录，未纳入公开文件。
