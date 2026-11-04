# plugin-server：IF-07 接口实施计划

> 2026-10-25缺口派工：[G01–G11实施任务卡](2026-10-25-gap-dispatch-task-cards.md)已固定准确落点、最小前置片段、共享文件唯一合入和验收；与原D0卡共同进入跨项目Issue索引。宽泛热点以新卡的直接白名单/接口贡献区分；所有任务待实施。

> 2026-10-25首批细化：[P0/D0派工卡](2026-10-25-p0-d0-task-cards.md)包含3卡的准确文件落点、输入输出、共享责任、局部依赖与机械验收。下一步按最小公共片段启动，各域独立准备；所有卡待实施，本轮没有创建schema、业务代码、运行夹具或部署。原任务范围及后续运行出口保持不变。

更新：2026-10-25。状态：实施拆分完成，所有任务与验收待执行；本轮仅新增文档，未修改业务代码、构建、部署或运行测试。功能责任ART，具体人员/日期待排期。[公共接口](../../../docs/plans/2026-10-25-nine-interface-contracts.md)；[IF-07字段、状态与验收权威契约](../../../ocloudmodelfusion/docs/contracts/2026-10-25-release-artifact-contract.md)。本计划细化既有[PS-01至PS-04](2026-10-22-http-plugin-delivery-hardening.md)，不重复另定包schema。

## 1. 当前核对与职责边界

CodeGraph已核对 `plugin_server.py:35–45`元数据只有MD5，`:97–107`直接写入同name/version后写metadata；`upload_plugin.py:14–32`无内容身份核对或unknown恢复；`pull_plugins.py:152–155`文件存在即复用，`:165–187`按首manifest并遍历层导入。不可变版本、完整包、原子可见与来源锁仍是开发任务。

ART独占ArtifactManifest及制品提交状态；Console独占注册、发布计划、配置升级/回滚与业务启用。ART不调用K8s、不改实例配置、不判断ready、不写钱包。每个name/version的所有写入口（PUT、sync、pull、镜像seed导入）遵守同一内容身份；协调跨进程范围必须覆盖实际支持部署模式，不把Python线程锁宣传为全局原子性。

## 2. 可派工任务

| 任务／唯一主责 | 输入→输出与文件热点 | 并行依赖 | 验收出口 |
| --- | --- | --- | --- |
| ART-01／ART schema维护职责 | IF-07＋现有目录→manifest验证器、规范摘要、有效/无效包样例；新公共验证模块；既有metadata/存储/构建入口只交接口给ART-04/02/06 | 即刻可做文档/fixture设计；代码待授权；与Console/SDK均不互等实现 | IF07-V01/V02；docs/default/schema变化不能在相同版本静默覆盖 |
| ART-02／ART存储职责 | ART-01验证结果→私有暂存、完整版本提交、同版本并发协调、upload查询及unknown重放；`plugin_server.py`、`tests/test_plugin_server.py` | 依赖ART-01；先单写进程/实际支持模式，跨入口竞争共同实现 | IF07-V01/V02；不同内容只一方成功，读端无半包，失败保留旧包 |
| ART-03／ART导入职责 | ART-01公共验证器→PUT/CLI/sync/pull统一幂等与来源；`upload_plugin.py`、`pull_plugins.py`、`scripts/sync-plugin-artifacts.sh`、对应tests | 可与ART-02用存储stub并行，最终必须共用同提交接口 | 同键同内容返回既有manifest；ACK未知查后重放；固定OCI manifest/layer/Wasm三摘要 |
| ART-04／ART打包职责 | 已编译Wasm＋说明/schema/defaults→完整包、inventory、来源记录；metadata/打包转换入口；`Makefile`、`Dockerfile`由ART-06接稳定CLI | 依赖ART-01，无需等待实际网关；CON-REL提供环境引用白名单 | 无真实凭据/包内执行脚本；相同锁定输入内容清单一致；历史unknown明确标识 |
| ART-05／ART保留职责 | CON-REL引用/保留清单→可审计删除预检、拒绝引用中版本；导入/同步工具及文档 | 默认保留可先实现；真正prune等待Console完整引用契约，非首期调用闭环前置 | IF07-V05；当前/旧任务/历史事实/回滚所需版本可读，镜像更新不自动删除 |
| ART-06／ART交付职责 | committed包＋构建来源→固定服务器镜像、包manifest与HTTP校验输入，交给DEP | ART-02/03/04单仓库检查完成后；实际运行还依赖DEP环境和CON-REL发布 | IF07-V01/02本仓通过后才交付；IF07-V04/05由真实网关联合判定，不据单测ready |

责任行指功能唯一主责，不代表多人已分配。`plugin_server.py`、manifest公共模块及schema版本只由ART集成职责合入，其他线通过接口协作，避免多份校验规则。旧metadata.txt/MD5可保持兼容展示，新强摘要唯一权威。

原计划映射：PS-01→ART-01/04（版本/别名/输入）；PS-02→ART-02/03（可靠提交/所有入口）；PS-03→ART-03/04/06（来源/OCI/镜像证据）；PS-04→ART-01/04（完整包）。ART-05补引用保留边界，不能把原PS范围改成可选。完成状态按这些任务证据回写原计划，不重复计算交付。

## 3. 交付与合流条件

先交ART-01的规范内容样例、错误表、状态样例给Console/SDK/DEP，三方可使用stub并行。ART-02/03内部完成后提交可验证upload结果、同版本冲突与unknown查询样例；ART-04提供真实包及来源锁，Console才注册可启用目录。CON-REL以同一artifact_id/content_digest/wasm_digest引用，DEP按HTTP实读核对；包存储可用不替代插件工厂/ABI、网关加载和规则生效证据。

拟定验证入口沿用Python unittest及同步脚本测试，增加真实并发/进程中断/磁盘失败、不同内容同时提交和历史引用保护；测试命令仅在获授权实施时执行。本轮未运行这些命令。完成记录须附实际分支/commit/dirty摘要、包与镜像digest、IF07案例结果及IF09 run引用；目前全部not_run。

原计划索引维护由协调任务统一完成，已有未提交工作保留。后续更改契约先升schema或明确兼容字段，禁止上传工具与Console各自猜测缺省值。

## 4. EB-05制品验证交付（2026-10-25）

沿[EB-05详细方法](../../../ocloudmodelfusion/docs/plans/2026-10-25-release-boundary-verification-plan.md)和[FX-02/06](../../../modelfusion-deploy/docs/plans/2026-10-25-boundary-fixtures-plan.md)执行，不另建包或证据schema。当前全部not_run。

| 原包 | 验证交付与通过条件 | 并行/阻塞 |
| --- | --- | --- |
| ART-01/02/03 | EB05-V01：同内容/同版本异内容、并发GET/PUT、提交中断/ACK丢失；私有暂存不得位于HTTP可服务目录；完整包一次可见、原版本不变 | 机器manifest与存储/客户端stub先并行；真实并发/HTTP读回在ART公共提交入口合流后完成，不等媒体/资金 |
| ART-04/06 | EB05-V02：冻结非空expected集合，错误字节、缺项、短读、默认版本回落；交真实构建来源及每文件摘要 | 与DEP采集器并行；Console版本透传未完成阻塞新协议发布，不能用catalog默认补版本 |
| ART-05 | EB05-V09：旧任务/事实/回滚引用存在时保留；删除与新引用竞争 | 默认retain先交；真实prune等CON-REL完整引用协调，非首期调用闭环前置 |

ART仅提供committed与可核对HTTP字节，CON-REL才决定业务ready/支持/开放。后续部署模式按实际单/多副本分别取证；现有单副本测试不外推多副本。2026-10-25仅完成验证计划，未上传、构建、改代码或部署。

## 5. 全范围复审的完整包与目录交接

依据[IF07补齐后的上传与注册边界](../../../ocloudmodelfusion/docs/contracts/2026-10-25-release-artifact-contract.md)及[公共规范字节](../../../ocloudmodelfusion/docs/contracts/2026-10-25-canonical-json-profile.md)，ART-01/D0-ART-01须同时交付PackageDescriptor和ArtifactManifest两种机器schema。归档根`package.json`仅列payload，不能把其自身或服务端committed记录列入待哈希files；`archive_digest`、committed时间与upload结果由服务端在归档外生成。ART-02验证/提交，ART-03各入口只调用共同实现。

ART-04把旧`spec.yaml`的包装结构与独立`config.schema.json`分开：确定性转换规则、不可转换项报错、defaults对schema验证和原始来源引用均须交付；不能把OpenAPI包装原样宣称为JSON Schema。公共JSON字节算法只由MF01定义，ART实现内容投影并与独立消费者互验，Unicode/重复键/指数/控制字符必须精确一致；D0文件摘要仍取原字节。

合入出口依次为ART-01协议与golden→ART-02/03统一提交→ART-04完整来源包→ART-06实际HTTP包证据；CON-REL的MF-REL01单独承担catalog注册、版本化schema/defaults读取与幂等查回。ART committed而注册失败时保留已提交包，重试同注册意图；不回退修改包，也不自动启用。SDK不得在新版本缺失时取catalog最新值。注册状态不由ART代写；旧版本/历史任务引用按ART-05默认保留先交，真正删除并非首次闭环前置。

完整跨仓顺序见[全范围依赖规划](../../../docs/plans/2026-10-25-full-scope-dependency-review.md)。以上仍为待实施任务；本轮没有上传、生成机器schema或运行验包。
