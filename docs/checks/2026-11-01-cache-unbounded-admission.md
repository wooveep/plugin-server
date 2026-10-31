# 缓存交付遵循原生产准入期限

Helm138 首次组合 Memory/Cache 的真实客户请求返回403；Gateway 原访问记录固定在 `ai-cache.execution_delivery`，Memory assemble 已为200，客户模型供应商未收到发送。CodeGraph 未索引该 Go 模块，随后读取实际 `execution_delivery.go` 和共享 `executionfacts/admission.go` 核对：原 Redis 准入以零 `ValidUntil` 表示当前 HTTP 上下文无显式期限，缓存正文和命中交付却直接把零值判为过期。

两处期限判断现在仅检查原准入显式携带的期限。原 header 阶段的身份、发布、调用和时间校验继续由既有 `ReadAdmittedCall` 完成；没有生成新期限或更改共享 framework。新正式版本为 ai-cache2.0.7。

[RED](cache-unbounded-20261101/red-command.json)使用实际已交付2.0.6 Wasm，在 Go 和 Wasm 两种宿主上均复现403，覆盖自身捕获原正文与消费 Memory 已捕获正文两条路径。回归同时核对成功缓存交付保留原 call/occurred_at，且不发送模型上游。修复制品的完整测试、入库和实际请求验收继续，以最终 GREEN 和部署回执为准。

[完整 GREEN](cache-unbounded-20261101/full-wasm-command.json)覆盖66个顶层用例（含子项562），无失败/跳过；[catalog五项检查](cache-unbounded-20261101/catalog-green-command.json)通过。[完整包资格](cache-unbounded-20261101/qualification.json)绑定提交、原始源码输入和已测试 Wasm，版本2.0.7已入库，原92份不可变manifest保持。实际加载和业务回执继续由DEP维护。
