# 原始上游响应头耗时采集

2026-11-13，当前 `dev`，对应 [I22 调用监控补充](https://github.com/wooveep/ocloudmodelfusion/issues/83#issuecomment-6829965467)。本记录说明采集实现；页面与聚合验收以 [Console 逐项记录](../../../ocloudmodelfusion/docs/checks/2026-11-12-feature-issues-followup.md) 为准。

每个 managed Router attempt 在原始上游响应头到达、确认 upstream/cluster 且状态头有效后，记录从该 attempt `sendStartedAt` 到回调的毫秒数。采样位于 JSON 缓冲和 SSE 转换之前，不等待正文、终止事件或完整请求结束。它是上游响应头 RT，不是客户端端到端耗时或首正文 Token 时间。

`execution-telemetry.v1` 增加可选 `first_byte_ms`，随既有 attempt telemetry/outbox 发送，不增加一次业务发送、计费事实或用量。边界要求非负、attempt 身份及可靠发送时间；已知 0 保留，缺测不发该字段。重试使用现有新 state，样本不继承前次 attempt。只有完整响应回调的旧 callout 路径保持缺测。

Console 对应可空列及认证 ingest 必须先具备接收能力，再启用新的不可变插件版本；旧行不回填，原计费时间和 FACT 不变。本批只有源码及临时编译检查，未发布新目录、覆盖现有 2.0.79 工件或重发正式路由。

验证结果：

- `executionfacts` 既有单元通过，覆盖真实 0、字段缺失和负值拒绝。
- `ai-proxy` 与 config/provider/util 四个相关 Go 包通过；新增一个 host 回调用例含 JSON/SSE 两子场景，确认尚无 body/stream completion 时已取得首响应头样本。该测试使用 SDK host emulator，不称为真实上游请求。
- 本机 Go 1.26.3 的 `wasip1/wasm`、`c-shared` 编译通过。临时工件 `/tmp/modelfusion-calls-first-byte-unreleased.wasm`，摘要 `sha256:bd5a4ae634011b810d2b45e38d535059d10eeaf60a0e76c3ea97652a4cba03b9`；它未登记或部署，不代替项目发布构建器的正式工件验证。
- 日志为 `/tmp/modelfusion-calls-first-byte-{proxy-unit-final,telemetry-unit,wasm-build}-private.log`。首次新增测试的 Router fixture 漏配置 StreamProjection，已按既有合法 fixture 修正，生产逻辑未因此变更，原失败日志保留。

两轴源码复核无剩余确定 P1/P2。VM、完整采集传输及真实 LLM 样本仍待后续明确部署窗口和实际验证，不据此关闭 I22。
