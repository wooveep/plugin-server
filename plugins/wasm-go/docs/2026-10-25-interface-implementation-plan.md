> 2026-11-05 迁入说明：本页保留 Go 契约夹具与任务来源，正文状态属于 2026-10-25 历史检查点。当前交付以[2.2.1 发布准备](../../../../docs/plans/2026-11-05-release-2.2.1.md)及当前源码为准；全新安装按现行契约验证。

# wasm-go 九类接口实施计划

> 2026-10-25缺口派工：[G01–G11实施任务卡](2026-10-25-gap-dispatch-task-cards.md)已固定准确落点、最小前置片段、共享文件唯一合入和验收；与原D0卡共同进入跨项目Issue索引。宽泛热点以新卡的直接白名单/接口贡献区分；所有任务待实施。

> 2026-10-25首批细化：[P0/D0派工卡](2026-10-25-p0-d0-task-cards.md)包含4卡的准确文件落点、输入输出、共享责任、局部依赖与机械验收。下一步按最小公共片段启动，各域独立准备；所有卡待实施，本轮没有创建schema、业务代码、运行夹具或部署。原任务范围及后续运行出口保持不变。

更新：2026-10-25。状态：计划已细化，全部业务实现/测试/发布待执行。本轮只修改文档。人员、开始日期和工期待排期。工作范围为独立wasm-go定制插件，不修改Higress核心。

入口：[九类接口总表](../../../../docs/plans/2026-10-25-nine-interface-contracts.md)、[应用层基线APP-00–05](../../../../ocloudmodelfusion/docs/2026-10-25-application-execution-billing-baseline.md)。本计划落实原WASM执行、计量和准入工作包，字段只引用权威契约，不在插件另定义同名DTO。复杂Redis恢复、Pod接流、网络栅栏均非前置。

## 1. 契约输入与责任

| 契约 | 插件职责 | 权威来源 / 外部唯一owner |
| --- | --- | --- |
| IF-01 | 唯一选择、可信身份传播、精确target/Key、逐发送owner | [执行身份](../../../../ocloudmodelfusion/docs/contracts/2026-10-25-execution-dispatch-contracts.md)；发布语义归Console、编译归SDK |
| IF-02 | 消费客户端、本地单向gate、媒体单发和Outcome分类 | [派发许可](../../../../ocloudmodelfusion/docs/contracts/2026-10-25-execution-dispatch-contracts.md)；持久意图和grant唯一归Console媒体派发模块 |
| IF-03/04 | 输出事实/稳定重投和缺口，不计算销售/采购最终金额 | [事实、账务、余额](../../../../ocloudmodelfusion/docs/contracts/2026-10-25-billing-facts-balance-contracts.md)；事实归并/资金事务归Console |
| IF-05 | ai-quota只读版本化余额投影，未就绪拒绝受影响新调用 | 同上；钱包及投影恢复声明归Console；插件不自建余额、不写钱包 |
| IF-07 | 导出实际加载配置/版本证据，拒绝不支持的能力组合 | [发布与制品](../../../../ocloudmodelfusion/docs/contracts/2026-10-25-release-artifact-contract.md)；发布状态归Console，制品归plugin-server |

同步call身份由可信网关准入模块产生；媒体必须沿Console既有operation/call。`request_id`仅HTTP诊断、`provider_task_id`仅供应商任务，禁止把旧文档歧义`task_id`透传为新协议。主router/native retry事实由上游观察适配器产生，主动http_call由ai-proxy产生；ai-billing只编码/投递与负责同步最终交付，不另建采购attempt。

## 2. 可并行工作包

下列owner为唯一模块主责；具体人员待分配。每包包含未来业务代码落点，路径存在不代表改造完成。“建议新增”不指称现有能力。

| 任务 / 状态 | 唯一owner | 输入及依赖 | 实施落点与输出 | 退出条件 |
| --- | --- | --- | --- | --- |
| WG-I00 契约样例 / 待实施 | wasm-go契约维护模块 | IF-01/02/03/05/07固定字段，Console/SDK共享同一版本样例；无业务实现依赖 | 建立本仓契约样例消费者及反例目录（建议新增docs/contracts-fixtures与测试样例）；先覆盖缺版本、混用task_id、同ID异摘要、未知状态 | 三方DTO解释一致；新协议拒绝歧义，不静默退回provider/model猜测 |
| WG-I01 目标与凭据冻结 / 待实施 | wasm-go执行协调模块 | WG-I00；SDK目标descriptor桩即可开发，真实验收依赖SDK隔离编译及IF-07完整发布 | `extensions/ai-proxy/main.go`、`provider/failover.go`、`provider/provider.go`、`pkg/wrapper/cluster_wrapper.go`；产生唯一ExecutionSelection，固定target/credential，安装而非重新抽Key | IF01-V01/V03/V05；消费后改变目标/Key停止；同provider不同binding不互相覆盖 |
| WG-I02 本地gate与分类 / 待实施 | wasm-go派发消费客户端模块 | WG-I00、Console Consume桩；最终依赖WG-I01及真实Console事务 | `extensions/ai-proxy/provider/retry.go`及建议新增dispatch消费模块；结构化Outcome、一次回调放行、201/202/未知停止创建；与SDK单发配置关联 | IF02-V03/V04/V06；Observe不能变成许可；丢grant零次或最多一次收件且如实unknown |
| WG-I03 attempt事实 / 待实施 | wasm-go事实模块 | WG-I00及IF-03；可与WG-I01/02并行，单独合成事件不代表端到端通过 | `extensions/ai-billing/main.go`、新增共享事实编码/发送助手及只读观察适配器；retry.go由WG-I02接接口；主事实与补证owner固定 | IF01-V02/V04及IF-03全部验收；收件数=独立attempt，重投不变ID/内容，可信更正新revision，不把最终usage复制到中间失败 |
| WG-I04 余额准入 / 待实施 | wasm-go ai-quota模块 | IF-05余额投影DTO/读协议；先用缓存响应桩并行，不依赖媒体页面 | `extensions/ai-quota/main.go`及对应schema；从现有裸整数GET迁移到IF-05就绪/版本/金额校验；缺失、错误、未就绪拒绝受影响新调用 | IF-05验收中读端用例、APP-V9；后台旧事实/查询不被误当新销售准入拦截；不使用allow missing绕过 |
| WG-I05 配置能力与加载证据 / 待实施 | wasm-go配置契约模块 | IF-07制品/配置清单，WG-I00；可先schema/样例并行，实际证据依赖构建制品与SDK输出 | `resources/plugins/ai-proxy/spec.yaml`、`ai-billing/spec.yaml`、`ai-quota/spec.yaml`及相应README；operation、schema版本、单发配置和摘要证据 | 不支持组合阻止发布；真实制品/运行配置可关联；业务测试不能拿仓库源码版本冒充加载版本 |
| WG-I06 应用层联合验收 / 待实施 | wasm-go验证模块 | WG-I01–05/07、Console派发/账务/余额、SDK隔离、deploy独立收件桩、plugin-server不可变包 | 本仓测试＋外部APP-V0–V10证据引用，故障矩阵IF01/IF02；不新增第二套账务oracle | 收件→attempt→销售/采购明细逐笔吻合；全部阻塞能力有通过证据或明确禁用配置；仅单测通过不能关闭工作包 |
| WG-I07 简化冷却与注册 / 待实施 | wasm-go冷却模块 | IF-01身份/IF-02必要恢复claim、[现行简化恢复](../../../../ocloudmodelfusion/docs/2026-10-25-simple-upstream-recovery-design.md)与Console权威状态接口；可先按状态样例并行 | `extensions/ai-proxy/provider/failover.go`由本包拥有健康注册/状态合并区域；完整配置反复加载不清冷却，取消独立付费探测，真实请求恢复；选择消费方接口供WG-I01调用 | 首次429按scope冷却30秒、连续恢复失败退避至300秒且长Retry-After优先；重启/旧成功不复活撤销Key；全冷却新请求503/媒体原期限等待；既有COOL/STORE及IF02-V05通过 |

WG-I01与WG-I03都会触及ai-proxy发送边界：执行协调模块拥有选择/dispatch修改，事实模块提供共享编码接口及插桩契约；合入时由执行协调模块落发送点集成，禁止两条工作线分别重写重试循环。WG-I02拥有回调分类和gate，事实模块只消费结构化Outcome，不另写一套HTTP状态判断。WG-I07拥有failover健康注册与状态区域，WG-I01拥有凭据选择/安装区域；公共结构调整由WG-I07先交接口，WG-I01再集成，避免同时修改同一结构定义。

首批并行：WG-I00冻结样例后，WG-I01、WG-I02（桩）、WG-I03、WG-I04、WG-I05、WG-I07分别启动。首次合流为目标身份＋单发/Consume＋逐attempt事实；第二次合流为Console销售/采购差额及余额；媒体/门户最终验收随后接入，不等待页面才验证事务。

## 3. 当前代码与工程边界

2026-10-25通过CodeGraph读取当前代码，未运行产品测试：

| 代码证据 | 当前行为 | 需要改变的契约边界 |
| --- | --- | --- |
| [retryCall / sendRetryRequest / dispatchRetryRequest](../extensions/ai-proxy/provider/retry.go)（82/125/192） | 仅200成功；其他回调可继续重试；随机Key；无状态默认502 | operation分类＋完整性＋accepted/unknown分离；媒体创建单发；响应不明不能伪造拒绝后重试 |
| [RouteCluster / TargetCluster](../pkg/wrapper/cluster_wrapper.go)（29/48） | RouteCluster运行时读cluster_name；TargetCluster可显式固定名称 | 固定目标复用候选有源码基础；实际mapper前选择和重入时序仍需收件/xDS证明 |
| [BillingEvent / deliverBillingEvent / buildBillingEvent](../extensions/ai-billing/main.go)（133/776/807） | 无call/attempt/release；Context标delivered后XADD；provider由cluster推断 | 新协议精确归属、稳定事件身份、ACK重放/缺口；不得把fail-open称可靠落盘 |
| [ai-quota onHttpRequestHeaders](../extensions/ai-quota/main.go)（202） | 读取租户/consumer header；GET裸余额整数；存在missing allow配置 | 校验可信来源和IF-05就绪/版本；新受管路径不能只凭整数>0或missing allow放行 |
| [SetApiTokensFailover / SetApiTokenInUse](../extensions/ai-proxy/provider/failover.go)（141/721） | Complete调用时resetSharedData；定时恢复仍可发真实模型探测；请求入口另选随机Key | 注册幂等且不清权威冷却；停止未授权独立探测；将选择/安装与状态合并拆分，不用关冷却来关媒体重试 |

当前尚未证明的工程项是：可信属性能否跨实际fallback重入、真实镜像逐上游观察覆盖、mapper读取前可否固定target、所有主动调用是否禁隐藏重试、跨插件同版本配置是否实际加载。每一项由对应WG包取得证据；不能以字段齐全或代码中存在TargetCluster就标完成。

最小能力退化规则：不可观测native retry关闭；媒体创建禁所有隐藏重发及跨Context fallback；复杂媒体组合若无法保证固定目标/可信传递则不能发布，保留经验证单target单发组合。不得为绕过证据缺口修改Higress核心，或恢复已退出的nft/Pod生命周期门禁方案。

## 4. 验收与交付证据

每份运行记录至少包含契约版本、Console/SDK提交、插件包SHA-256、实际镜像摘要、渲染及实际加载配置摘要、operation/call/dispatch/attempt关联、独立收件记录、事件去重/冲突记录和对应账本/余额版本。对故障测试记录注入点与事务/回调先后，敏感Key、原始正文及grant不进入普通日志。

- WG-I01：IF01-V01/V03/V05，实际B1/B2/B3身份100%相符；APP-V3权重10000次独立调用各比例绝对偏差≤2个百分点，失败调查而非重跑至通过。
- WG-I02：IF02-V01/V02由Console事务证据提供，插件负责IF02-V03–V06真实收件证据；201/202、缺status和截断须分别测。
- WG-I03：IF01-V02/V04与IF-03；ACK丢失/乱序/双观察不多计，无法恢复事件缺口明确可见。
- WG-I04：IF-05与APP-V9；数据库旧outbox不能覆盖新投影，未就绪入口拒绝，既有事实可继续入账。
- WG-I05/06：IF-07及APP-V0–V10；单元夹具、实际网关、完整账务三层通过记录分别保存，不互相替代。

本轮交付是本计划及IF-01/02契约；以上实现、故障注入、容量测试、构建及部署均未执行。

## 5. 工程边界验证交付（2026-10-25）

详细方法见[EB-01/02执行验证](2026-10-25-execution-boundary-verification-plan.md)，共同输入与收件/oracle见[FX计划](../../../../modelfusion-deploy/docs/plans/2026-10-25-boundary-fixtures-plan.md)，跨仓顺序见[工程边界协调](../../../../docs/plans/2026-10-25-engineering-boundary-verification.md)。运行均not_run。

| 原任务 | 细化交付 | 并行/硬前置 |
| --- | --- | --- |
| WG-I00/01 | EB-W01/02，两阶段call/body、真实matcher前target/Key一致、旧backup/跨重入负例 | 可先host/matcher样例；真实出口依SDK完整配置、实际包及FX-01 |
| WG-I02 | EB-W03/05，指定CalloutID逆序回调、旧Context gate、隐藏retry与201/202/未知 | Consume桩/模拟host先行；实际grant丢失依Console事务，单发证明依独立receipt |
| WG-I03 | EB-W04，每观察实例私有快照，router/callout唯一owner，Periodic/End补证不多计 | 事件样例/观察器独立；native retry能力由实际镜像及上游链覆盖证明，不从最终streamInfo回填 |
| WG-I04 | [EB-04](../../../../ocloudmodelfusion/docs/plans/2026-10-25-cache-maintenance-verification-plan.md)的所有路径/schema/代际/维护拒绝 | 响应桩可先；真实维护依MF-CACHE01及完整发布，legacy只覆盖文本不算媒体已拒绝 |
| WG-I05/06 | 加载链/工厂/ABI、全实例与EB01/02真实案例，接[EB-05](../../../../ocloudmodelfusion/docs/plans/2026-10-25-release-boundary-verification-plan.md) | 先冻结依赖解析和Wasm来源；下发成功不等于加载；media跨Context创建fallback保持关闭 |

测试host便利函数若总回调第一callout，不能用于两个pending回调顺序的证明；须采用已确认的显式CalloutID底层接口并记录实际调度。工厂源码存在不代表镜像可用；只阻塞依赖失败能力的profile，不能让native retry未知阻塞所有资金/单发开发。

本轮进展仅为深入静态追链及验证设计，未构建/运行；下一步按EB-W01/03/04与共享FX先并行准备，再合流实际单发→多target→已证明可观测的retry/fallback。

## 6. 全范围复审补齐：响应缓存、记忆与验证入口

跨仓总顺序见[完整依赖规划](../../../../docs/plans/2026-10-25-full-scope-dependency-review.md)。以下为W02/W04/W09已有范围的任务落点，全部待实施，不新增首批D0卡；D0只登记其接口语料，产品接线在P1以后。

| 任务 / 唯一owner | 文件、输入输出 | 硬依赖 / 并行界面 | 通过条件 |
| --- | --- | --- | --- |
| WG-I08 响应缓存/记忆上下文 / wasm-go上下文集成 | `extensions/ai-cache/cache_key.go`及缓存命中入口、`extensions/ai-memory/request_injection.go`、`extensions/ai-memory/trusted_cache_digest.go`及共享正文上下文；开工锁定新增模块文件清单。输入IF01 BodyBasis、SDK策略指纹和IF03，输出原正文基准、最终outbound摘要及cache delivery | IF01/03和SDK01策略描述即可stub开发；WG-I01拥有目标安装，WG-I03拥有事实编码；Console内部任务由MF-INTERNAL01另行生产事实 | 首次变换前保存BodyBasis，记忆/mapper后记录实际出站正文；策略变化隔离缓存。命中产生delivery且零上游attempt；已经发送后本地返回不冒充零采购缓存命中；stream/nonstream、注入开关、重入/重试均有精确正文与计量证据 |
| WG-I09 validation准入 / wasm-go准入集成 | 拟新增共享validation准入客户端，由`extensions/ai-proxy/main.go`唯一接线；SDK01生成受控入口。消费IF07 ValidationAccess和已鉴权主体，输出本call准入结果 | MF-REL02逐调用权威校验接口、MF-ID02测试主体；可用固定响应桩并行；实际验证要先完成此门再运行候选链 | 在任何Consume/实际发送前校验主体、release/组合/profile/route_scope、revision和有效期；撤销/过期/接口不可达/伪造头拒绝，禁止跨call缓存允许结果；普通客户不进入validation。production按自己的支持/发布门开放，不复用测试许可 |

WG-I08修改公共上下文结构时先由本卡交小接口；proxy发送处统一交WG-I01接入，billing编码统一交WG-I03。WG-I09只实现许可消费，不拥有许可状态；该状态唯一归CON-REL。WG-I05负责spec/README的一致配置描述，两卡不得各自修改公共spec。`pkg/wrapper/`的公共响应正文接缝由WG-I02独占合入，其他卡只交要求和反例。

### 6.1 首次失败正文与唯一重试决策

WG-I02完整承接[APP-02 BODY-01–03/BODY-V01–07](../../../../ocloudmodelfusion/docs/2026-10-24-attempt-execution-event-design.md#app-02-首次失败正文与重试时序2026-10-25)：headers暂停、正文可读/读取失败/超过上限/截断/StreamDone分别归并为一次终结决策，随后才允许该profile已验证的retry。WG-I03只保存原始usage与结构化Outcome，不再次判定是否重试。WG-I01实际发送前消费该单次决策；不能由headers路径和body路径各发一次。

CodeGraph核对`ai-proxy/main.go:459–503`的headers重试/非200不读body，以及wrapper正文读取失败的Continue路径，说明“修改retry callback”不足以覆盖首次响应。离线host正负例与真实宿主暂停、EOS、64KiB边界、截断、流结束分别取证；若实际宿主不能满足，只关闭依赖该能力的重试profile，单发、账务和其他产品研发继续。DEP-IF01收件、DEP-IF04真实ABI配合；模拟host通过不代替BODY运行结论。

### 6.2 冷却生产者及联合出口

WG-I07的Console依赖具体为MF-HEALTH01持久状态/claim、MF-HEALTH02版本outbox/快照；WG-I01候选过滤消费该状态，媒体恢复claim还接MF-MED01联合Consume。它们可按固定scope/Outcome各自实现，健康快照/事件最终必须双Pod合流；不等待完整媒体UI，不恢复已退出的独立付费探测。

WG-I06最终验收还包括本组合启用的WG-I08、受控验收所需WG-I09、MF-INTERNAL01和BODY案例。15类插件原矩阵逐项声明哪些会改变正文、产生内部调用或提前返回；不能只验证ai-quota余额缓存就宣布响应缓存/记忆已验收。CodeGraph所见cache key按route/model/requestDigest/cachePolicyVersion构造，记忆注入会修改body，内部任务由Console reqwest发送；因此三条路径必须在call/attempt及正文基准处合流。仍保留各profile不支持/未验证的明确记录。
