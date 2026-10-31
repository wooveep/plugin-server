# Memory 与 Cache 的原调用时间

本批串行负责 WG-I08 的请求侧，与 [MF 原始调用记录](../../../ocloudmodelfusion/docs/checks/2026-11-01-processing-call-origin.md)接线。当前旧生产路径仅发送 call_id，而服务端的语义查询早于实际模型注册，发生缺原历史的503；实际失败和七次后台成功的边界由 [DEP 运行记录](../../../modelfusion-deploy/docs/checks/2026-11-01-internal-runtime.md#internal-native-helm136)维护。

ai-memory 的 assemble 和 ai-cache 的 lookup 从 header 阶段已捕获、经过 native 身份与原 release 匹配的 AdmittedCall，向原认证 RPC 附带 `call_origin.occurred_at`。仅原 bounded 调用携带已有 valid_until；生产 Redis 调用保持无此字段。原时间字符串直接传递，后续过滤器删除 header 后仍使用同一份捕获值。缺失、重复、跨 release 或旧标签路径均不补原始时间。Cache 事件继续只携带原 call_id，其历史来源由处理 RPC 负责。

类型位于各插件私有 RPC 结构旁，正式版本提升为 **ai-memory0.1.9、ai-cache2.0.6**；VERSION、spec 与唯一 catalog 一致。[输入闭包核对](processing-origin-20261101/source-input-closure.json)确认55类中只有这两类发生变化，共享 framework 输入与本批基线相同。

[最终 Go 批次](processing-origin-20261101/green-commands.json)覆盖两个模块及其 config 包，全部通过；memory 增加原 bounded 与生产 unbounded 的传递检查，两个入口继续拒绝缺失/重复/跨发布的假 parent，且校验 header 删除后来源保持。早期公共类型试验及日志已留在本地诊断目录，最终采用各 RPC 的私有类型，只以最终源码摘要作为交付依据。

MF 的准确 PG RED、10项分层GREEN及严格Clippy见其唯一记录。完整制品构建、入库/HTTP读回、实际配置加载与语义/缓存请求仍待完成；Go 宿主检查不等同实际网关通过。

完整 Wasm 批次已通过：Memory 72 个顶层用例（含子项607），Cache 65 个顶层用例（含子项555），均无跳过。首次打包被来源校验正确阻止：Wasm 测试框架在源码旁生成的 `wasm-unit-test.wasm` 被目录输入模式误收为源码，无法在记录的 Git 提交中找到。源码输入枚举现在只排除这个已知测试输出，仍保留实际嵌入的 Wasm 资产；[五项 catalog 检查](processing-origin-20261101/catalog-generated-output-green.json)覆盖生成测试产物不改变发布输入、真实源码改动仍改变摘要。旧构建与测试记录保留，重新构建后核对二进制及实际源码输入，再制作完整包；不会手工改写旧 receipt。
