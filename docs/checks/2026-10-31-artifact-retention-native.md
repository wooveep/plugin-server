# ART-05 引用保留、删除与目录验收

日期：2026-10-31（Asia/Shanghai）。本卡范围验收通过。插件服务器源码为 `2563aaa196e04a87ba0c7e1b18b2fab42f507da6`，Console 消费修复为 `3b02a3153808698ad6a69594f2625c7976c765db`，实际部署为 **Helm102**。[机器总回执](artifact-retention-20261031/acceptance.json)列出证据文件及摘要。本卡完成不代表原 79 卡、余额 Recovery 或完整业务验收完成。

## 责任与实际修复

ART-05 的预检始终默认 retain，独立预检不能授权物理删除。引用决定仍由 Console 的版本化引用协调器负责；ART-02 的实际存储/HTTP 文件由 root 串行集成，没有修改 sync 的删除策略。Console 的目录可选状态由 MF-REL01 接口统一消费。

CodeGraph 核对了预检、存储及 HTTP 调用边界。既有索引对已修改文件标记过期、对新测试未命中，因此这些部分再按当前源码核对，没有重建索引或把缺边当成无影响。

原生测试发现：未引用制品已完成物理删除，目录仍从不可变登记记录中选出它。根因是把“历史登记存在”当成“当前可供新配置选择”。修复后的默认目录和版本选项排除 `dispatching / unknown / deleted`，按可用版本选最高 SemVer；显式历史版本、精确身份的元数据与 schema 仍从原登记记录读取。

删除阶段原先还重复读取、散列已验收包的全部载荷。该读取不能证明当前引用授权，已移除；首次上传继续完整校验。删除仍检查原登记身份、路径归属、协调器决定/修订/有效期及持久回执，并通过隔离目录和原命令查回确认结果。

## 分层验证

| 层次 | 实际结果与边界 |
| --- | --- |
| Python 3.13.5 | 全套 115 通过；同步/保留入口通过。默认 retain、五种引用、过期/旧修订、身份冲突、unknown、持久查回及中断恢复均有相应测试；新增测试证明删除不重新打开载荷。 |
| Console 目录/Schema 集成 | 真实隔离 PostgreSQL 下 22 通过、1 个显式生产 ART opt-in 用例 ignored。新测试覆盖三种删除占用状态、版本回退、不可选与历史读取；该层 ART HTTP 为测试服务。 |
| Console retention 批次 | 12 通过、无跳过：6 个 retention 单元测试、1 个真实 PostgreSQL 双连接删除占用/新引用竞争测试、5 个过滤器附带用例。该竞争测试的 Kubernetes/观察接口是明确测试桩，不作为 K3S 证据。 |
| 静态与构建 | Console 严格 Clippy、API/操作 CLI 构建通过。精确命令、源码和原始日志摘要见[测试回执](artifact-retention-20261031/test-receipt.json)。 |
| K3S 原生 HTTP | 下列保留、拒绝、上传、删除、原命令查回、目录和物理 HTTP 均在实际部署执行。无模型调用、无付费探测。 |

## K3S 原生路径

- 对当前媒体发布引用的 `ai-proxy 2.0.10` 请求删除，实际返回 `retained / referenced_artifact`，引用快照包含 `active_release` 和 `historical_fact`。后者这里是不可变发布审计引用，不是宣称产生了计费 FACT。[原回执](artifact-retention-20261031/referenced-retain.json)保留决定身份、有效期和快照摘要。
- 上传 `uploaded-directory-fixture 2.10.0` 后，以旧 catalog revision 请求删除，返回 **409 `artifact_catalog_revision_conflict`**；真实 revision 下删除成功。原命令在 Helm102 重启后、许可有效期结束后仍查回同一删除结果，没有分配新命令重做。
- Helm102 下上传同名 `2.9.0`，目录准确选择仍可用的较旧版本，schema 绑定其原身份；随后删除成功。两份测试 Wasm 都只有 **8 字节**，source 为 `legacy_unknown`，从未绑定网关，不能作为 Wasm ABI 或模型执行证据。
- 删除后两版均不再出现在可选目录，原始 Wasm HTTP 都返回 **404**；显式历史详情和精确 schema 均返回 **200**。原删除审计和两个 tombstone 保留。实际原生 DTO 在空集合时省略 `artifact_versions`，取证脚本曾按必有数组断言失败；已用只读续验核对契约，没有重做上传或删除。
- 原 **55 类正式默认引用**全部保持。插件服务器原 **79 个有效版本**的 ID、名称、版本、内容/归档摘要逐项一致，镜像更新没有 prune 或复活两个已删测试包；原 PVC UID/PV 保持。当前和旧版 `ai-proxy 2.0.10 / 2.0.9` 实际 Wasm HTTP 均为 **200**。[原生服务回执](artifact-retention-20261031/native-artifact-server.json)、[Console HTTP 回执](artifact-retention-20261031/native-http-verification.json)记录精确身份和结果。

## 部署、来源与业务边界

两张镜像均按 digest 固定，[插件服务器构建](artifact-retention-20261031/plugin-server-build.json)与 [Console 构建](artifact-retention-20261031/console-build.json)记录精确源码及二进制摘要。Console 输入清单额外包含四份未跟踪、未参与编译的模块 README，沿用[上一批来源说明](../../../ocloudmodelfusion/docs/checks/2026-10-31-plugin-directory-consumer.md)；没有将它们描述为编译源码。

server dry-run 比较 80 份清单，只有 Console API 与插件服务器两个 Deployment 改变；沿用 Chart `cef1b629cde0f97b202f478df501d1f681a3303fa2d88acc5d206c4599706384`，配置见 [Helm102 override](../../../modelfusion-deploy/environments/modelfusion-dev/artifact-retention-values.yaml)。16 个部署可用，四个入口 200，单 gateway UID 保持，原 133 份 Ingress/WasmPlugin 最终 UID/spec 一致。`key-auth.internal` generation **302→306**、最终 spec 相同；未捕获中间写入，不能据此宣称所有 generation 或中间状态不变。[运行回执](artifact-retention-20261031/runtime-verification.json)保留该观察。

客户余额保持，FACT、修订、应用和现金流水仍均为零。原 Recovery `3b43fb42-9122-47e5-bdcc-d6b700cc227a` 仍为 `requested / revision 1`，没有绕过原恢复流程、改写 Redis ready 或重置已有失败观察。下一批继续原余额恢复及业务路径；本卡的过期/unknown/竞争故障验证与实际 K3S 正常路径已分别注明，未把测试桩结果写成真实集群故障通过。
