# 完全透传异常响应导致运行资源保留

日期：2026-11-05。状态：源码回归、完整制品交付和修复版空卷运行复测通过。来源：用户要求对最新 dev 再次清理部署、验证 2.2.1；范围仍为单 VM／单节点、全新安装。

## 现象与证据

真实 K3S 验收中，正常36形态及原文／状态／响应头检查通过。删除受控完全透传路由时，入口关闭但6个Wasm及3个EnvoyFilter持续保留。原生回收报告 `deleted route still has live execution or retained references`；8条异常调用的计量事实已accepted、用量estimated，但call_delivery仍为unknown。原始供应商attempt未知状态被保留，未伪造发送证明。

负例包括200 text/plain、无法证明来自供应商的500／503，以及HTTP回调已结束、缺原生终态的SSE。带gzip的有效JSON和Responses的in_progress对象仍可能确实未知，不能因为需要清理就将它们终结。

## 根因与修复

透传分支直接采用上游ExecutionResult作为客户端DeliveryState，并仅在上游HTTPStatus已被确认为采购证据时把HTTP错误标为failed。为防止将Envoy本地错误伪称供应商发送，原采购观测会正确移除不可信500／503状态；这一保护又导致已经返回的原始错误被记为未结束交付。

ai-proxy 2.0.47 将两项含义分别处理：已返回的HTTP错误、2xx原生接口不支持的媒体类型、完整HTTP回调内缺终态的SSE，将客户端交付记为failed；供应商attempt、用量来源和响应原文保持。合法的未结束Responses和不能解析的编码正文继续unknown，不放宽原生回收引用保护。

## 验证

新增 `TestPassthroughDeliveryFinalityKeepsUnknownProcurement` 在修复前4个负例均失败，修复后通过；同时验证编码JSON和异步Responses保留unknown。状态、原始正文保持，500／503的采购HTTP状态仍为空、未宣称observed_sent。ai-proxy完整Go包回归通过。

待完成：固定工具链编译、独立来源与完整包核对、55项镜像；部署新目录后复测异常事实、原生删除及API／VM恢复。本次负例的原账本与冻结计划先备份，使用全新业务存储复测，不手改旧事实或冻结计划。

本轮私有审计：`/home/cloudyi/.local/state/modelfusion-release/2026-11-05-2.2.1-dev-clean-redeploy-1917/`。原始配置、凭据和完整dump不进入Git。

## main基线双轴复核补充

固定main `974e4694` 至首个修复 `3ad46687` 的双轴复核另定位两项：非origin目录候选在组装后丢失supplier，使OpenAI Messages→Chat的max_completion_tokens以及Qwen Messages终态输入修订失效；AI Cache边界将非法protocol_mode或null／非字符串profile静默当缺省。

增加真实目录fallback用例，先分别复现OpenAI参数错误、Qwen用量退为unknown，再在provider校验边界保留完整profile；不再用未填supplier的局部profile覆盖。新增14个配置负例，在原生解析入口拒绝非法类型及枚举，缺省和已登记值保持。两组回归先失败后通过，代理／缓存完整包测试通过；缓存版本升为2.0.20，同名同版本不覆盖。

## 修复版空卷复测

来源 `4cb49b1a` 的 ai-proxy 2.0.47／ai-cache 2.0.20 已独立核对完整包并部署。真实网关 6 组原始异常交付均接受终态 failed；500／503 的供应商 execution_result=unknown、send_state=handed_off、HTTP 状态为空保持原样。14 条测试路由的原生资源全部自然清完，再次 API 重建后没有复活；17 份冻结计划及账本保留。完整结果见[清理部署验收](../../../modelfusion-deploy/docs/checks/2026-11-05-dev-clean-redeploy.md)。

同轮双轴复审还修复真实候补供应商丢失 supplier profile、缓存配置边界静默接受无效值的问题，分别增加2个实际 fallback 回归和14个无效配置回归。Standards、Spec 剩余均0项，代理与缓存完整 Go 包、插件服务131项检查通过。
