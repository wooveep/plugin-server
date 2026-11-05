# 完全透传异常响应导致运行资源保留

日期：2026-11-05。状态：源码回归通过，完整制品交付与空卷复测待完成。来源：用户要求对最新 dev 再次清理部署、验证 2.2.1；范围仍为单 VM／单节点、全新安装。

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
