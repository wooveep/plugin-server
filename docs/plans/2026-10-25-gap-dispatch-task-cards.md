# 制品服务：G01–G11缺口实施任务卡

| 字段 | 内容 |
| --- | --- |
| 状态 / 更新 | 2026-10-25；6张实施/汇总卡已细化；全部待实施、运行not_run |
| 负责人 / 日期 | 按卡唯一模块责任；自然人、代班、期限待分配 |
| 来源 | 用户要求11类缺口派工、更新并行顺序并建立Issues |

承接[接口计划](2026-10-25-interface-implementation-plan.md)和[跨项目派工与Issue总表](../../../docs/plans/2026-10-25-gap-dispatch-and-issue-tracking.md)。D0仍以[原任务卡](2026-10-25-p0-d0-task-cards.md)为文件白名单权威；此处不复制20张原卡。业务字段/状态权威仍为IF01–09，本文权威维护本仓实施落点、最小交付片段及共享文件单写责任。旧接口表的宽泛热点不是多人写授权。

本轮仅规划与Issue登记；尚未实施，产品/运行验收均为not_run。人员、代班、日期待分配。Q40首期fresh部署，不做旧业务数据迁移或平滑升级；Q101同修订固定、新修订处理时价；Q41事件故障fail-open并显式记录缺口。媒体创建单发、原账户查询；不增加付费探测、严格RPM/TPM、核心改造或独立恢复系统。

`@片段`通过需给出固定输入/输出、提交和测试回执，供消费者独立验证；它不意味着整卡关闭。最终联合协作不是整卡先后关系，不能把互验双方互相等待“关闭”画成启动依赖。存在目录白名单时，开工PR补登记实际文件，不授权修改整个仓库。

## ART-01：生产级完整包契约与规范化验证器

Issue：[ART-01](https://github.com/wooveep/plugin-server/issues/2)。

**唯一责任：**ART schema维护职责；任务文件集成责任由`ART-01`承担。自然人/代班/期限待分配。
**阶段与状态：**P1产品/工具实现；待实施，验收`not_run`。
**最小前置：**[D0-ART-01](https://github.com/wooveep/plugin-server/issues/1) @ `manifest`

依赖中的`@片段`是可单独合入/验收的交付，不等上游Issue关闭。消费前记录上游提交、路径、摘要、测试回执；字段未冻结可先准备案例/桩，不能据此接真实产品。

**本卡须先交的片段：**

- `ART-01@validator`：无导入副作用的生产验包/规范化模块。

**可直接修改的文件落点（相对本仓）：**

- `artifact_contract.py` — 拟新增文件/目录。
- `tests/test_artifact_contract.py` — 拟新增文件/目录。

**输入：**冻结的IF07/common规范、D0-ART-01包类型及D0-ART-02正负语料
**输出：**无副作用的生产validator；描述/manifest投影、payload/schema/default/ref验证与明确错误

**验收条件：**IF07-V01/V02；docs/default/schema变化不能在相同版本静默覆盖

**共享文件责任：**只公共validator，plugin_server.py归ART02、metadata归ART04；不把tests/contracts helper直接作为产品依赖。
**实施/合入边界：**PackageDescriptor/manifest两个类型，mf-json.v1 exact golden，重复键/路径/文件角色/摘要/未知算法明确拒绝。

**CodeGraph复核后的验证细节：**

- 通过：生产模块跑同一golden；schema/default/ref验证一致；verified来源必须有独立核验结果；无网络/目录写副作用的解析入口可单测。

**最终联合验收协作项：**[D0-ART-02](https://github.com/wooveep/plugin-server/issues/8)、[ART-04](https://github.com/wooveep/plugin-server/issues/5)。这里要求相应接口/证据就绪，不要求对方整卡先关闭；对等验证可以同一轮取证，各方分别记录回执。

**交付检查：**

- [ ] PR仅含本卡直接白名单；共享文件变更需求由指定任务串行合入，未登记新文件先更新本卡。
- [ ] 输入锁/字段映射、正例与拒绝反例、预期和实际结果可复核；注明测试命令、cwd、版本及证据路径。
- [ ] 先交独立模块/片段，再接共享文件，最后按本卡实际范围执行联合验收；桩结果与真实运行结果分开。
- [ ] 未运行保持not_run，环境/ABI不足保持blocked；不以文档、schema、旧测试或模拟宿主冒充真实通过。
- [ ] 关闭时回填PR/提交、测试结果和依赖片段回执；汇总卡须子卡全部通过，不能据部分profile关闭完整范围。

来源：`wooveep/plugin-server:docs/plans/2026-10-25-interface-implementation-plan.md`；本卡细化见本仓`docs/plans/2026-10-25-gap-dispatch-task-cards.md`。

## ART-02：原子不可变提交、并发协调与unknown查回

Issue：[ART-02](https://github.com/wooveep/plugin-server/issues/3)。

**唯一责任：**ART存储职责；任务文件集成责任由`ART-02`承担。自然人/代班/期限待分配。
**阶段与状态：**P1产品/工具实现；待实施，验收`not_run`。
**最小前置：**[ART-01](https://github.com/wooveep/plugin-server/issues/2) @ `validator`

依赖中的`@片段`是可单独合入/验收的交付，不等上游Issue关闭。消费前记录上游提交、路径、摘要、测试回执；字段未冻结可先准备案例/桩，不能据此接真实产品。

**本卡须先交的片段：**

- `ART-02@committed`：不可变commit/query/逐文件读取接口及并发自测。

**可直接修改的文件落点（相对本仓）：**

- `plugin_server.py` — 现有文件。
- `artifact_storage.py` — 拟新增文件/目录。
- `tests/test_plugin_server.py` — 现有文件。
- `tests/test_artifact_storage.py` — 拟新增文件/目录。

**输入：**ART-01验证结果、包原始字节与幂等upload键
**输出：**私有stage、单次原子可见commit、版本不可变、upload query及逐文件读取

**验收条件：**IF07-V01/V02；不同内容只一方成功，读端无半包，失败保留旧包

**共享文件责任：**plugin_server.py唯一集成；ART03/05交客户端/保留接口，不能重复写存储。
**实施/合入边界：**

**CodeGraph复核后的验证细节：**

- 通过：同版本不同内容并发只一赢家；GET无半包；失败保留旧内容；同键异内容拒绝；提交后丢ACK可查回；stage在HTTP服务根外；unknown不自动失败/换key。

- 验证：既有标准库ThreadingHTTPServer测试可扩展，但旧假Wasm样本不证明新包有效；并发/进程退出/磁盘失败测试是P1真实存储测试，非D0完成证明。

**交付检查：**

- [ ] PR仅含本卡直接白名单；共享文件变更需求由指定任务串行合入，未登记新文件先更新本卡。
- [ ] 输入锁/字段映射、正例与拒绝反例、预期和实际结果可复核；注明测试命令、cwd、版本及证据路径。
- [ ] 先交独立模块/片段，再接共享文件，最后按本卡实际范围执行联合验收；桩结果与真实运行结果分开。
- [ ] 未运行保持not_run，环境/ABI不足保持blocked；不以文档、schema、旧测试或模拟宿主冒充真实通过。
- [ ] 关闭时回填PR/提交、测试结果和依赖片段回执；汇总卡须子卡全部通过，不能据部分profile关闭完整范围。

来源：`wooveep/plugin-server:docs/plans/2026-10-25-interface-implementation-plan.md`；本卡细化见本仓`docs/plans/2026-10-25-gap-dispatch-task-cards.md`。

## ART-03：上传、同步、OCI拉取统一提交协议

Issue：[ART-03](https://github.com/wooveep/plugin-server/issues/4)。

**唯一责任：**ART导入职责；任务文件集成责任由`ART-03`承担。自然人/代班/期限待分配。
**阶段与状态：**P1产品/工具实现；待实施，验收`not_run`。
**最小前置：**[ART-01](https://github.com/wooveep/plugin-server/issues/2) @ `validator`

依赖中的`@片段`是可单独合入/验收的交付，不等上游Issue关闭。消费前记录上游提交、路径、摘要、测试回执；字段未冻结可先准备案例/桩，不能据此接真实产品。

**本卡须先交的片段：**

- `ART-03@import`：统一上传/同步/OCI导入CLI与来源记录。

**可直接修改的文件落点（相对本仓）：**

- `upload_plugin.py` — 现有文件。
- `pull_plugins.py` — 现有文件。
- `scripts/sync-plugin-artifacts.sh` — 现有文件。
- `tests/test_upload_plugin.py` — 现有文件。
- `tests/test_pull_plugins.py` — 现有文件。
- `tests/test_sync_plugin_artifacts.sh` — 现有文件。
- `artifact_import.py` — 拟新增文件/目录。
- `tests/test_artifact_import.py` — 拟新增文件/目录。

**输入：**完整包、固定来源摘要及ART-01/02协议
**输出：**上传/同步/OCI/seed统一提交与查回客户端，来源锁、异常分类

**验收条件：**同键同内容返回既有manifest；ACK未知查后重放；固定OCI manifest/layer/Wasm三摘要

**共享文件责任：**存储HTTP入口由ART02；构建metadata由ART04；同协议不另实现摘要。
**实施/合入边界：**可用存储stub先做；真实闭环依ART02.committed/query；保留原ref/manifest/layer/Wasm来源。

**CodeGraph复核后的验证细节：**

- 验收：四入口同版本身份一致，任一旁路均不能替换已提交schema/default/docs；超时同键查回；fixture验证registry端口/tag@digest/多manifest/非Wasm层；token不落日志。

**最终联合验收协作项：**[ART-02](https://github.com/wooveep/plugin-server/issues/3)。这里要求相应接口/证据就绪，不要求对方整卡先关闭；对等验证可以同一轮取证，各方分别记录回执。

**交付检查：**

- [ ] PR仅含本卡直接白名单；共享文件变更需求由指定任务串行合入，未登记新文件先更新本卡。
- [ ] 输入锁/字段映射、正例与拒绝反例、预期和实际结果可复核；注明测试命令、cwd、版本及证据路径。
- [ ] 先交独立模块/片段，再接共享文件，最后按本卡实际范围执行联合验收；桩结果与真实运行结果分开。
- [ ] 未运行保持not_run，环境/ABI不足保持blocked；不以文档、schema、旧测试或模拟宿主冒充真实通过。
- [ ] 关闭时回填PR/提交、测试结果和依赖片段回执；汇总卡须子卡全部通过，不能据部分profile关闭完整范围。

来源：`wooveep/plugin-server:docs/plans/2026-10-25-interface-implementation-plan.md`；本卡细化见本仓`docs/plans/2026-10-25-gap-dispatch-task-cards.md`。

## ART-04：完整包打包、配置schema转换和来源锁

Issue：[ART-04](https://github.com/wooveep/plugin-server/issues/5)。

**唯一责任：**ART打包职责；任务文件集成责任由`ART-04`承担。自然人/代班/期限待分配。
**阶段与状态：**P1产品/工具实现；待实施，验收`not_run`。
**最小前置：**[ART-01](https://github.com/wooveep/plugin-server/issues/2) @ `validator`

依赖中的`@片段`是可单独合入/验收的交付，不等上游Issue关闭。消费前记录上游提交、路径、摘要、测试回执；字段未冻结可先准备案例/桩，不能据此接真实产品。

**本卡须先交的片段：**

- `ART-04@package`：完整包生成CLI、转换保真表及锁定输入。

**可直接修改的文件落点（相对本仓）：**

- `generate_metadata.py` — 现有文件。
- `artifact_packaging.py` — 拟新增文件/目录。
- `tests/test_artifact_packaging.py` — 拟新增文件/目录。
- `plugin_schema_conversion.py` — 拟新增文件/目录。
- `tests/test_plugin_schema_conversion.py` — 拟新增文件/目录。
- `tests/fixtures/schema-conversion/` — 拟新增文件/目录。
- `docs/contracts/plugin-schema-conversion.md` — 拟新增文件/目录。

**输入：**已锁定Wasm、现有spec及docs/defaults/资源声明、ART-01包协议
**输出：**完整包生成CLI、schema转换映射/报告、来源记录；构建接线交ART-06

**验收条件：**无真实凭据/包内执行脚本；相同锁定输入内容清单一致；历史unknown明确标识

**共享文件责任：**generate_metadata.py与打包/转换模块唯一ART-04；Makefile/Dockerfile/plugins.properties由ART-06接线，ART-04只交稳定CLI和锁定输入。
**实施/合入边界：**global/route schema、phase/priority、externalResources必须逐字段保真映射或显式拒绝；不能只抽openAPIV3Schema。 第一出口为版本化转换映射表，由ART-01发布受控扩展schema、SDK01消费；必须覆盖global/route、phase/priority、externalResources与defaults并纳入payload摘要。不能保真时显式unsupported_schema_conversion。此片段先于打包器验收，双方不等待整卡。 第一出口为版本化转换映射表，由ART-01发布受控扩展schema、SDK01消费；必须覆盖global/route、phase/priority、externalResources与defaults并纳入payload摘要。不能保真时显式unsupported_schema_conversion。此片段先于打包器验收，双方不等待整卡。

**CodeGraph复核后的验证细节：**

- 必须先形成逐字段表：info身份/展示、phase/priority、configSchema、routeConfigSchema、external-resource(s)、路由能力扩展、受控环境引用、defaults。每字段声明目标位置、摘要覆盖、消费者；不能丢弃后回catalog latest补回。

- 验收：至少key-auth及一个含route/global差异和外部资源的真实元资料fixture；转换前后合法/非法配置判定一致，schema与默认文件有效；未知关键字/歧义nullable/ref/无法保真项拒绝；源码/输入/转换器版本与结果锁定；同输入payload确定。

**最终联合验收协作项：**[ART-01](https://github.com/wooveep/plugin-server/issues/2)。这里要求相应接口/证据就绪，不要求对方整卡先关闭；对等验证可以同一轮取证，各方分别记录回执。

**交付检查：**

- [ ] PR仅含本卡直接白名单；共享文件变更需求由指定任务串行合入，未登记新文件先更新本卡。
- [ ] 输入锁/字段映射、正例与拒绝反例、预期和实际结果可复核；注明测试命令、cwd、版本及证据路径。
- [ ] 先交独立模块/片段，再接共享文件，最后按本卡实际范围执行联合验收；桩结果与真实运行结果分开。
- [ ] 未运行保持not_run，环境/ABI不足保持blocked；不以文档、schema、旧测试或模拟宿主冒充真实通过。
- [ ] 关闭时回填PR/提交、测试结果和依赖片段回执；汇总卡须子卡全部通过，不能据部分profile关闭完整范围。

来源：`wooveep/plugin-server:docs/plans/2026-10-25-interface-implementation-plan.md`；本卡细化见本仓`docs/plans/2026-10-25-gap-dispatch-task-cards.md`。

## ART-05：制品引用保留与删除预检

Issue：[ART-05](https://github.com/wooveep/plugin-server/issues/6)。

**唯一责任：**ART保留职责；任务文件集成责任由`ART-05`承担。自然人/代班/期限待分配。
**阶段与状态：**P1产品/工具实现；待实施，验收`not_run`。
**最小前置：**[ART-01](https://github.com/wooveep/plugin-server/issues/2) @ `validator`

依赖中的`@片段`是可单独合入/验收的交付，不等上游Issue关闭。消费前记录上游提交、路径、摘要、测试回执；字段未冻结可先准备案例/桩，不能据此接真实产品。

**可直接修改的文件落点（相对本仓）：**

- `artifact_retention.py` — 拟新增文件/目录。
- `tests/test_artifact_retention.py` — 拟新增文件/目录。

**输入：**CON-REL版本化引用/删除预检及ART-02存储接口
**输出：**默认retain策略、明确删除拒绝/执行/unknown和审计记录

**验收条件：**IF07-V05；当前/旧任务/历史事实/回滚所需版本可读，镜像更新不自动删除

**共享文件责任：**实际存储删/HTTP接线交ART02，引用决定CON-REL；不私改sync删除策略。
**实施/合入边界：**默认retain先交；实际prune依REL01完整引用协调，非首期单发前置。

**CodeGraph复核后的验证细节：**

- 通过：镜像/seed/sync不prune上传包；过期/旧revision删除许可拒绝；活跃、在途媒体、历史事实、回滚引用均保留；删除占用和新引用竞争可判定；unknown不能声称删除成功。

**最终联合验收协作项：**[MF-REL01](https://github.com/wooveep/ocloudmodelfusion/issues/17)、[MF-REL02](https://github.com/wooveep/ocloudmodelfusion/issues/21)、[ART-02](https://github.com/wooveep/plugin-server/issues/3)。这里要求相应接口/证据就绪，不要求对方整卡先关闭；对等验证可以同一轮取证，各方分别记录回执。

**交付检查：**

- [ ] PR仅含本卡直接白名单；共享文件变更需求由指定任务串行合入，未登记新文件先更新本卡。
- [ ] 输入锁/字段映射、正例与拒绝反例、预期和实际结果可复核；注明测试命令、cwd、版本及证据路径。
- [ ] 先交独立模块/片段，再接共享文件，最后按本卡实际范围执行联合验收；桩结果与真实运行结果分开。
- [ ] 未运行保持not_run，环境/ABI不足保持blocked；不以文档、schema、旧测试或模拟宿主冒充真实通过。
- [ ] 关闭时回填PR/提交、测试结果和依赖片段回执；汇总卡须子卡全部通过，不能据部分profile关闭完整范围。

来源：`wooveep/plugin-server:docs/plans/2026-10-25-interface-implementation-plan.md`；本卡细化见本仓`docs/plans/2026-10-25-gap-dispatch-task-cards.md`。

## ART-06：制品构建与HTTP交付证据

Issue：[ART-06](https://github.com/wooveep/plugin-server/issues/7)。

**唯一责任：**ART交付职责；任务文件集成责任由`ART-06`承担。自然人/代班/期限待分配。
**阶段与状态：**P1产品/工具实现；待实施，验收`not_run`。
**最小前置：**[ART-02](https://github.com/wooveep/plugin-server/issues/3) @ `committed`；[ART-03](https://github.com/wooveep/plugin-server/issues/4) @ `import`；[ART-04](https://github.com/wooveep/plugin-server/issues/5) @ `package`

依赖中的`@片段`是可单独合入/验收的交付，不等上游Issue关闭。消费前记录上游提交、路径、摘要、测试回执；字段未冻结可先准备案例/桩，不能据此接真实产品。

**可直接修改的文件落点（相对本仓）：**

- `docs/checks/artifact-delivery.md` — 拟新增文件/目录。
- `tests/test_artifact_delivery.py` — 拟新增文件/目录。
- `Makefile` — 现有文件。
- `Dockerfile` — 现有文件。
- `plugins.properties` — 现有文件。
- `scripts/export-artifact-inventory.py` — 拟新增文件/目录。
- `tests/test_artifact_inventory.py` — 拟新增文件/目录。

**输入：**ART-02已提交内容、ART-03导入CLI、ART-04完整包及来源锁
**输出：**锁定镜像构建输入、非空逐文件inventory、HTTP交付清单和本仓验证证据

**验收条件：**IF07-V01/02本仓通过后才交付；IF07-V04/05由真实网关联合判定，不据单测ready

**共享文件责任：**Makefile/Dockerfile/plugins.properties、镜像inventory和构建依赖入口唯一ART-06；打包CLI归ART-04，存储归ART-02；不写ready/supported。
**实施/合入边界：**交精确包/镜像digest与源锁给DEP；不写ready/supported。

**CodeGraph复核后的验证细节：**

- 通过：最终镜像inventory与已提交包一致、历史目录保留、无遗漏/默认latest、实际HTTP逐文件由DEP验证。当前建卡不执行镜像构建/推送。

**最终联合验收协作项：**[DEP-IF05](https://github.com/wooveep/modelfusion-deploy/issues/9)。这里要求相应接口/证据就绪，不要求对方整卡先关闭；对等验证可以同一轮取证，各方分别记录回执。

**交付检查：**

- [ ] PR仅含本卡直接白名单；共享文件变更需求由指定任务串行合入，未登记新文件先更新本卡。
- [ ] 输入锁/字段映射、正例与拒绝反例、预期和实际结果可复核；注明测试命令、cwd、版本及证据路径。
- [ ] 先交独立模块/片段，再接共享文件，最后按本卡实际范围执行联合验收；桩结果与真实运行结果分开。
- [ ] 未运行保持not_run，环境/ABI不足保持blocked；不以文档、schema、旧测试或模拟宿主冒充真实通过。
- [ ] 关闭时回填PR/提交、测试结果和依赖片段回执；汇总卡须子卡全部通过，不能据部分profile关闭完整范围。

来源：`wooveep/plugin-server:docs/plans/2026-10-25-interface-implementation-plan.md`；本卡细化见本仓`docs/plans/2026-10-25-gap-dispatch-task-cards.md`。

## 2026-10-26 开工登记

代班 Codex；分支 `codex/artifact-p1-20261026`；输入 ART `e9a42d9b7c67ff7bced5cfbda75df5e36a8b1989`、固定 common/domains/ART `.3` 保持原字节。依序 ART01→ART04/02→ART03/05→ART06，单一写者串行接线。ART01 新增生产 `mf_canonical.py`（从固定 MF 工具逐函数无语义更改提取）、`artifact_schemas/{common,package-descriptor,artifact-manifest}.schema.json`；依赖入口 `requirements.in/requirements.lock` 归 ART06；本轮证据 `docs/checks/2026-10-26-artifact-p1.md` 及 `docs/checks/artifact-p1/` 归 ART06。生产模块不依赖测试目录；Wasm 静态有效性不代表网关 ABI。原用户改动保持。

ART06 追加准确路径：`.dockerignore`、`.gitignore` 的 packages 生成物规则、`packages/policy.json`、`scripts/build-upload-cli.py`、`scripts/seed-artifact-packages.py`、`scripts/export-artifact-inventory.py`、`tests/test_artifact_inventory.py`、`tests/test_artifact_delivery.py`、`tests/test_artifact_retention.py`。ART05 先交默认 retain/删除预检；真实删除等待 CON-REL 事务式引用占用，不伪造删除完成。生产镜像固定 Python digest，停止构建中按 mutable OCI tag 拉取。

ART06 2026-10-26补充：Dockerfile的LEGACY_PLUGINS_DIR显式输入；packages/legacy-empty/.keep、docs/checks/2026-10-26-artifact-k3s-image.md、docs/checks/artifact-k3s/归ART06。默认K3S实际125历史版本与镜像14版本Wasm冲突，切换镜像仅携完整包增量，旧PVC由DEP只读保留并导入，版本拒绝规则不放宽。
