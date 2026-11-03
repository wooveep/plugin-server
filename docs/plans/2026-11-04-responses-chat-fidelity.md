# Responses→Chat 字段保真与非流式消息 ID 修复

| 字段 | 内容 |
| --- | --- |
| 状态 | 实施中：修复及隔离回归完成；接续合入开发003、部署VM K3S并切换业务路由 |
| 创建日期 | 2026-11-04 |
| 最近更新 | 2026-11-04 |
| 涉及项目 | plugin-server；ai-proxy Responses fallback 转换器 |
| 负责人 | 本聊天执行；长期维护负责人待确认 |
| 优先级 | 用户要求本轮修复 |
| 目标日期 | 本轮已完成，验收日期 2026-11-04 |
| 需求来源 | 用户要求整理已确认差异、修复字段丢失与缺少消息 ID，完成后在线回归 |

## 目标与范围

以[在线能力报告](../checks/2026-11-04-openai-api-capabilities.md#72-已复现的实现差异)为基线，修复已支持 Responses→Chat 子集中的静默字段丢失及非流式输出消息缺少 ID。保留显式的 false、0 与未设置的区别，不改变共享 Chat 请求类型的既有采样默认值。

| 编号 | 已确认差异 | 本轮交付与验收 |
| --- | --- | --- |
| RC-01 | text.format.json_schema 未映射到 response_format | 映射 text/json_object/json_schema；schema 的 name、description、schema、strict 保真；不认识的 format 类型明确拒绝 |
| RC-02 | tools[].strict、parallel_tool_calls 丢失 | 映射到 Chat 对应字段；true/false 均保留，未设置不补默认值 |
| RC-03 | temperature=0 因 omitempty 丢失 | 转换器对已支持的采样字段保留显式零值；非零及未设置行为不回归 |
| RC-04 | 非流式 Responses output message 没有 id | 文本、拒绝及不完整消息生成非空、可追溯至上游响应的 message ID；保留 function call 的关联 ID |
| RC-05 | reasoning、store、缓存键/配置静默丢失 | 映射 reasoning.effort、显式 store、prompt_cache_key/retention/options；保留消息/文本块缓存断点位置；background=true 和 reasoning 摘要明确拒绝 |
| RC-06 | function_call_output 与图像输入未覆盖 | 登记为后续适配覆盖工作，本轮不扩展输入类型 |

服务端状态续接、conversation、托管搜索缺少等价执行环境继续作为明确边界。协议、模型能力、供应商权限或参数限制单列，不据此认定项目实现缺陷。当前托管 SSE 聚合后交付的传输边界不属于本轮修复。

## 关联资料

- [OpenAI 协议说明](../../../docs/openai-api-protocol.md)
- [修复前脱敏在线回执](../checks/2026-11-04-openai-downstream-live.json)
- [Responses 官方接口](https://developers.openai.com/api/reference/resources/responses/methods/create)
- [Chat Completions 官方接口](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create)
- [本地 K3S 环境与交付流程](../../../modelfusion-deploy/docs/local-test-environment.md)

## 依赖与交付顺序

| 顺序 | 项目 | 交付物 | 前置依赖 | 实施任务或证据链接 |
| --- | --- | --- | --- | --- |
| 1 | plugin-server | 请求/响应语义回归用例 | 原报告的独立复现 | RC-01～05，provider 回归及 fallback 入口回归 |
| 2 | plugin-server | 转换器与函数 strict 修复 | 回归用例确认旧行为失败 | provider/responses.go、provider/model.go |
| 3 | plugin-server | 可追溯完整插件包 | 相关测试、Wasm 编译通过 | 固定源码、版本及 SHA256；不混入开发003未验收的 C 批 |
| 4 | 本地 VM K3S | 隔离路由在线回归与脱敏回执 | 固定修复制品、已有身份与资产 | 对照真实出口字段、JSON message ID、SSE 终态；按模型支持能力取样 |

## 验收条件

- [x] 复现旧版缺字段与缺 ID；覆盖显式 false/0、缺省值、三种格式与未知格式拒绝。
- [x] provider 与 ai-proxy 相关回归通过；函数关联、同协议转发与 SSE 终态不回归。
- [x] Wasm 及完整插件包可构建，实际在线加载字节与修复制品一致。
- [x] 真实网关出站字段与修复后的映射一致；非流式文本具有 ID，拒绝消息 ID 经源码回归；SSE 可解析并正常结束。
- [x] 对支持所测参数的真实模型做小批在线回归；供应商限制与受控收件证据分开标注。
- [x] 临时验证资源清理，能力报告、机器回执及工作区索引同步完成。

## 风险与阻塞

修复启动时开发003正在推进独立 C 批，本轮未修改其工作目录、路由或发布值。使用已验收的 2.0.31 源基线构建隔离修复制品，实际候选 Wasm 仅绑定到本轮临时路由。供应商是否支持 json_schema/strict/采样和缓存参数以实际模型响应为准；实际 OpenAI 服务拒绝块级缓存 TTL、继承请求级 TTL 后通过，已单列供应商接口差异。独立收件器只证明转换后的 HTTP 字段。

## 进展记录

| 日期 | 进展与证据 | 下一步 |
| --- | --- | --- |
| 2026-11-04 | 从修复前在线证据建立 RC-01～06；实现 RC-01～05，最终候选 a61ef533 / 2.0.31-responses.2，通过232顶层/897含子用例检查 | 隔离在线回归 |
| 2026-11-04 | 最终候选17个样本：8成功、8本地400、1供应商参数400；复用3个未改变逻辑的先前真实回执。原生关闭4个临时发布准入后清理运行资源，剩余0 | 本轮完成；正式业务合流/切换按统一交付流程另行执行，RC-06待后续排期 |

## 完成记录

修复与隔离在线回归已完成，见[权威验收报告](../checks/2026-11-04-responses-chat-fidelity.md)及[机器回执](../checks/2026-11-04-responses-chat-fidelity.json)。两条路由实际加载最终候选 SHA256 均为 `d75059ff79dd39d53336b7f497d0048fcf54fcd7f4676293b4bde77aade58d87`。正式业务路由未切换；图像/函数结果历史转换、实时流式等范围保留原边界，不把协议、模型或供应商限制列为项目缺陷。

## 正式合流与 VM K3S 交付（2026-11-04 新增授权）

用户已要求合入开发003最新版本并正式部署、切换当前业务路由，再做小批在线回归。首阶段的隔离候选与“未切换业务”结论作为历史检查点保留。

- 源码基线：开发003插件版本2.0.34（e3c55b59），包含工具准入回调修复；以正式版本2.0.35合入4份转换代码/测试文件，保留C批v5工具事实、计费、缓存及已有改动。
- [ ] 合入开发003最新工作树并核对已有改动。
- [ ] C批基线上的相关回归、Wasm编译及完整包通过。
- [ ] 基于当前活动制品库构建服务器镜像；保留既有版本/校验回执，先部署服务器并逐文件回读。
- [ ] 登记新制品并更新正式目录，经原生发布切换已启用业务路由；保留未发布改动和已有价册/绑定/权限。
- [ ] 核对实际ECDS与Wasm SHA，完成小批同协议/转换JSON、SSE及函数关联、错误/终态和事实回归。
- [ ] 清理本轮临时回归对象，更新部署回执、能力报告和索引。

每阶段重新读取Helm revision和完整values，server dry-run只允许本轮服务器/正式目录相关变化。保留并行API/Web/TLS改动、PVC、Secret与Gateway身份。协议、模型和供应商限制继续单列，不为覆盖率创建无关收费任务。
