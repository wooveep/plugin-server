# Responses→Chat 字段保真与非流式消息 ID 修复

| 字段 | 内容 |
| --- | --- |
| 状态 | 实施中 |
| 创建日期 | 2026-11-04 |
| 最近更新 | 2026-11-04 |
| 涉及项目 | plugin-server；ai-proxy Responses fallback 转换器 |
| 负责人 | 本聊天执行；长期维护负责人待确认 |
| 优先级 | 用户要求本轮修复 |
| 目标日期 | 待确认；本轮接续实施与在线回归 |
| 需求来源 | 用户要求整理已确认差异、修复字段丢失与缺少消息 ID，完成后在线回归 |

## 目标与范围

以[在线能力报告](../checks/2026-11-04-openai-api-capabilities.md#72-已复现的实现差异)为基线，修复已支持 Responses→Chat 子集中的静默字段丢失及非流式输出消息缺少 ID。保留显式的 false、0 与未设置的区别，不改变共享 Chat 请求类型的既有采样默认值。

| 编号 | 已确认差异 | 本轮交付与验收 |
| --- | --- | --- |
| RC-01 | text.format.json_schema 未映射到 response_format | 映射 text/json_object/json_schema；schema 的 name、description、schema、strict 保真；不认识的 format 类型明确拒绝 |
| RC-02 | tools[].strict、parallel_tool_calls 丢失 | 映射到 Chat 对应字段；true/false 均保留，未设置不补默认值 |
| RC-03 | temperature=0 因 omitempty 丢失 | 转换器对已支持的采样字段保留显式零值；非零及未设置行为不回归 |
| RC-04 | 非流式 Responses output message 没有 id | 文本、拒绝及不完整消息生成非空、可追溯至上游响应的 message ID；保留 function call 的关联 ID |
| RC-05 | reasoning、store、缓存键/配置静默丢失 | 映射 reasoning.effort、显式 store、prompt_cache_key/retention；对 background=true、reasoning 摘要、prompt_cache_options 明确拒绝，不伪造等价语义 |
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
| 1 | plugin-server | 请求/响应语义回归用例 | 原报告的独立复现 | RC-01～04，provider 回归及 fallback 入口回归 |
| 2 | plugin-server | 转换器与函数 strict 修复 | 回归用例确认旧行为失败 | provider/responses.go、provider/model.go |
| 3 | plugin-server | 可追溯完整插件包 | 相关测试、Wasm 编译通过 | 固定源码、版本及 SHA256；不混入开发003未验收的 C 批 |
| 4 | 本地 VM K3S | 隔离路由在线回归与脱敏回执 | 固定修复制品、已有身份与资产 | 对照真实出口字段、JSON message ID、SSE 终态；按模型支持能力取样 |

## 验收条件

- [ ] 复现旧版缺字段与缺 ID；覆盖显式 false/0、缺省值、三种格式与未知格式拒绝。
- [ ] provider 与 ai-proxy 相关回归通过；函数关联、同协议转发与 SSE 终态不回归。
- [ ] Wasm 及完整插件包可构建，实际在线加载字节与修复制品一致。
- [ ] 真实网关出站字段与修复后的映射一致；非流式文本/拒绝消息具有 ID；SSE 可解析并正常结束。
- [ ] 对支持所测参数的真实模型做小批在线回归；供应商限制与受控收件证据分开标注。
- [ ] 临时验证资源清理，能力报告、机器回执及工作区索引同步完成。

## 风险与阻塞

开发003正推进独立 C 批，不修改其工作目录、路由或发布值。本轮使用已验收的 2.0.31 源基线构建隔离修复制品，避免提前发送新事实版本。供应商是否支持 json_schema/strict/采样参数以实际模型和响应为准；独立收件器用于证明转换后的真实 HTTP 字段，不作为模型遵循 schema 的证据。

## 进展记录

| 日期 | 进展与证据 | 下一步 |
| --- | --- | --- |
| 2026-11-04 | 从修复前在线证据建立 RC-01～05；确认现用转换器与 2.0.31 对应源文件一致 | 实施回归、修复及隔离在线验收 |

## 完成记录

待填实际修复、测试与在线制品证据；未取得在线回执前不标记完成。
