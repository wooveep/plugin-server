# Responses 故障补修：冻结源码与简单回归

产品源码固定为 dev **`dacd69e148f61330b713d1a4854578e11b7fdcc3`**，基线 `940dda90ae09a92de867dee8ca6e0a635cb58ff8`。16个文件包括5个ai-proxy生产文件、7个必要测试及4份版本/发行元数据；SDK、共享Go、依赖锁、Rust、API和账单前端均未修改。本记录属于源码阶段，未构建、未部署，四项插件issue仍OPEN。

| 问题 | 最终修复提交 |
| --- | --- |
| [#11](https://github.com/wooveep/plugin-server/issues/11) 根路径扫描及完整合法Lookup重复复制 | `b3dcd30c`、`3188cd01` |
| [#12](https://github.com/wooveep/plugin-server/issues/12) 流终态/usage与结束顺序 | `880d3a18`、回归`4111e9a9` |
| [#13](https://github.com/wooveep/plugin-server/issues/13) 全部固定访问日志原因码 | `c551a2ef`、`cf4fdbb8` |
| [#14](https://github.com/wooveep/plugin-server/issues/14) 已知原生操作归因及转换元数据保留 | `7a866098`、`462eb50b` |

资源恢复读取已解析字段，完整Lookup仍执行全部record/owner/凭据校验，再按原始字段span排序并一次拼接，public policy仅初始化一次。验证了1090个合法别名、conversation字符串/对象、input项id/call_id、逆序字段、转义public ID、Unicode原ID及无关原字节。未知/跨范围引用、所有者或凭据失败仍零发送。

SSE别名写失败保留已验证终态与usage；在异步end_stream前先完成本地事实/usage，真实注入失败仍沿原事实修订为failed。未持久化的资源不能在下一请求续接。JSON尚未首次交付，保持原503持久化门槛，supplier/observedusage的17/5与failed delivery均保留。曾按200假设得到的JSON红仅是预期差异，未计为产品缺陷。没有增加Redis通用重试、健康/吊销快照宽限或新outbox。

## 影响范围

共享Redis故障和单Worker同步CPU阻塞可影响其他请求，不能将它们解释为只有Responses受影响。直接缺陷与回归边界如下，实际Envoy均尚未验证：

| 协议 | 本次直接缺陷 | 共享路径与本地验证 | 特定不适用项 |
| --- | --- | --- | --- |
| Chat Completions | 已知受管操作缺归因，已修 | 共同prepare、配置了健康目录时的读取、Router finish/事实与usage；JSON/SSE正常及末尾XADD失败、1090项大包通过 | 无Responses alias保存或异步终态Inject |
| Responses | 两段二次方恢复、SSE终态/usage时序、原生归因，已修 | JSON/SSE、两模式alias失败、真实Inject错误模拟、完整Lookup/续接拒绝、大包通过 | JSON503是保留的持久化边界 |
| Messages | 已知受管操作缺归因，已修 | 与Chat相同的共享依赖及JSON/SSE/末尾XADD/大包回归通过 | 不制造Responses alias或异步Inject故障 |
| Embedding | 已知受管操作缺归因，已修 | 共享prepare/健康/Router JSON；既有同步向量、真实大整数input-only/缺测/SSE拒绝及已验证profile归因通过 | SSE与Responses alias故障不适用 |

四类归因使用已验证profile和既有provider operation/path常量，不从外部URL猜测，不虚构AdapterID/Provider/Capability。原同操作已匹配转换metadata逐字保留，不匹配/异常schema保持unknown。原因码测试以GoAST扫描实际生产日志边界调用，不再维护另一份不完整列表；新增11唯一码对应12调用点，非受控tool前缀及外部原文仍被脱敏，不把全部静态码都当成现场已发生故障。

## 原红与回归

原版标准/透传SSE均丢response.completed，固定原因和原生路径真红；注入入口仅转发原SDK，测试可同步关闭流，复现结束先于事实/usage，SDK没有修改。后续补测证明完整合法Lookup也存在同量级退化。Embedding最初fixture缺宿主身份不计产品红；使用既有正确host后，通过Go overlay精确回放tracked7a入口得到0fixture/0panic的真实缺归因红。

冻结源码共有 **39个顶层单元通过，0失败、0跳过、0panic**：相关组37、legacy Responses独立1、ai-statistics归因契约独立1。legacy默认宿主与独立managed VM混在同进程存在原有上下文冲突，该项单独运行；不宣称完整项目suite通过。三文本Router正常/写失败覆盖12格，3种1090项大包；这些宿主回归不冒充真实Envoy。

下表为Native Go三次运行中位数，单位ms/op；Lookup实际执行MGET构造与全部record校验，但Redis传输是stub，不含网络/Envoy延迟。B/op是累计分配，不是RSS。

| input项数 | 原根扫描 | 冻结源无alias扫描 | 原完整合法Lookup | 冻结源完整Lookup |
| --- | ---: | ---: | ---: | ---: |
| 545 | 52.573 | 0.536 | 1070.751 | 4.168 |
| 1090 | 204.570 | 1.173 | 4308.564 | 7.983 |
| 2180 | 798.778 | 2.384 | 16925.757 | 16.807 |

1090合法alias累计分配由约2.769GB降至7.383MB；冻结源翻倍耗时接近翻倍。原件、每次样本、39项名称及完整selector见[机器记录](2026-11-13-responses-incident-source-validation.json)。主要命令在ai-proxy目录执行：

```sh
go test . -run '^TestResponsesIncident' -count=1
go test . -run '^$' -bench '^BenchmarkRestoreManagedResource(References|Lookup)$' -benchmem -benchtime=3x -count=3
go test . -run '^TestResponsesNativeRoutingKeepsResponsesPath$' -count=1
```

ai-statistics独立运行 `go test . -run '^TestStatisticsAttributionCardinalityContract$' -count=1`，生产代码未改。私有冻结回执 `/tmp/modelfusion-responses-incident-20261113/final-source-frozen-private.json`，SHA256 `888ebc3b7a4e52d3a90f22dd9f19aa3784b7a8cbe4e871352c5d4000be08b85b`。主审已独立逐文件核对并重跑39项通过；最终两轴与实际制品/运行验证另记，不能沿用阶段1仍有一个P2的审查作为终审。

## 交付与实际验证

候选新版本 **ai-proxy2.0.82** 尚未构建，原登记最高2.0.81不覆盖。[候选55选择](../../plugins/release/responses-incident-selection.json)只把proxy81换为82，其余54项逐对象保持，原selection文件保留。source_commit固定不随后续证据文档提交移动。

[真实验证方案](2026-11-13-responses-incident-native-verification-plan.md)包含13次专属零价Mock、真实合法种子、Embedding、owned-scope定向alias冲突及完整映射CAS清理，尚未创建资源。auto JSON定向故障和特定Inject宿主错误缺少现成隔离缝，实际验证边界明确保留。源修复与本地回归没有新增网关/供应商请求，也未改开发/灰度环境、旧Key、价格、授权或钱包；旧I22唯一RT额度不重开。
