> 2026-11-05 迁入说明：本页保留 Go 契约夹具与任务来源，正文状态属于 2026-10-25 历史检查点。当前交付以[2.2.1 发布准备](../../../../docs/plans/2026-11-05-release-2.2.1.md)及当前源码为准；全新安装按现行契约验证。

# WASM插件：G01–G11缺口实施任务卡

| 字段 | 内容 |
| --- | --- |
| 状态 / 更新 | 2026-10-25；10张实施/汇总卡已细化；全部待实施、运行not_run |
| 负责人 / 日期 | 按卡唯一模块责任；自然人、代班、期限待分配 |
| 来源 | 用户要求11类缺口派工、更新并行顺序并建立Issues |

承接[接口计划](2026-10-25-interface-implementation-plan.md)和[跨项目派工与Issue总表](../../../../docs/plans/2026-10-25-gap-dispatch-and-issue-tracking.md)。D0仍以[原任务卡](2026-10-25-p0-d0-task-cards.md)为文件白名单权威；此处不复制20张原卡。业务字段/状态权威仍为IF01–09，本文权威维护本仓实施落点、最小交付片段及共享文件单写责任。旧接口表的宽泛热点不是多人写授权。

本轮仅规划与Issue登记；尚未实施，产品/运行验收均为not_run。人员、代班、日期待分配。Q40首期fresh部署，不做旧业务数据迁移或平滑升级；Q101同修订固定、新修订处理时价；Q41事件故障fail-open并显式记录缺口。媒体创建单发、原账户查询；不增加付费探测、严格RPM/TPM、核心改造或独立恢复系统。

`@片段`通过需给出固定输入/输出、提交和测试回执，供消费者独立验证；它不意味着整卡关闭。最终联合协作不是整卡先后关系，不能把互验双方互相等待“关闭”画成启动依赖。存在目录白名单时，开工PR补登记实际文件，不授权修改整个仓库。

未来测试按独立模块运行：根目录`GOWORK=off go test ./pkg/ai/bodybasis ./pkg/ai/executionfacts ./pkg/wrapper -count=1`（新包创建后）；`extensions/ai-proxy`目录执行`GOWORK=off go test . ./provider ./config -count=1`；billing/quota/cache/memory各进入对应extension目录执行`GOWORK=off go test ./... -count=1`。I05先锁定真实依赖及旧绝对replace；缺依赖明确阻塞，不把根模块命令失败算产品缺陷。D0离线命令还须`GOPROXY=off`，不顺手联网安装。以上本轮均未执行。

## WG-I00：契约消费与执行夹具交付汇总

Issue：[WG-I00](https://github.com/wooveep/wasm-go/issues/2)。

**唯一责任：**wasm-go契约维护模块；任务文件集成责任由`WG-I00`承担。自然人/代班/期限待分配。
**阶段与状态：**D0交付汇总（不重复实施）；待实施，验收`not_run`。
**最小前置：**[D0-WG-01](https://github.com/wooveep/wasm-go/issues/1) @ `loader`

依赖中的`@片段`是可单独合入/验收的交付，不等上游Issue关闭。消费前记录上游提交、路径、摘要、测试回执；字段未冻结可先准备案例/桩，不能据此接真实产品。

**可直接修改的文件落点（相对本仓）：**

- `docs/2026-10-25-interface-implementation-plan.md` — 现有文件。

**输入：**IF-01/02/03/05/07固定字段，Console/SDK共享同一版本样例；无业务实现依赖
**输出：**汇总D0-WG-01–04的契约消费、语料、离线调度及测试回执；不另造第二份loader

**验收条件：**三方DTO解释一致；新协议拒绝歧义，不静默退回provider/model猜测

**共享文件责任：**汇总D0-WG01–04，不另造契约/loader。
**实施/合入边界：**纯离线通过不关闭任何真实宿主工程边界。

**最终联合验收协作项：**[D0-WG-01](https://github.com/wooveep/wasm-go/issues/1)、[D0-WG-02](https://github.com/wooveep/wasm-go/issues/9)、[D0-WG-03](https://github.com/wooveep/wasm-go/issues/4)、[D0-WG-04](https://github.com/wooveep/wasm-go/issues/10)。这里要求相应接口/证据就绪，不要求对方整卡先关闭；对等验证可以同一轮取证，各方分别记录回执。

**交付检查：**

- [ ] PR仅含本卡直接白名单；共享文件变更需求由指定任务串行合入，未登记新文件先更新本卡。
- [ ] 输入锁/字段映射、正例与拒绝反例、预期和实际结果可复核；注明测试命令、cwd、版本及证据路径。
- [ ] 先交独立模块/片段，再接共享文件，最后按本卡实际范围执行联合验收；桩结果与真实运行结果分开。
- [ ] 未运行保持not_run，环境/ABI不足保持blocked；不以文档、schema、旧测试或模拟宿主冒充真实通过。
- [ ] 关闭时回填PR/提交、测试结果和依赖片段回执；汇总卡须子卡全部通过，不能据部分profile关闭完整范围。

来源：`wooveep/wasm-go:docs/2026-10-25-interface-implementation-plan.md`；本卡细化见本仓`docs/2026-10-25-gap-dispatch-task-cards.md`。

## WG-I01：可信身份、目标凭据冻结与发送主链接线

Issue：[WG-I01](https://github.com/wooveep/wasm-go/issues/3)。

**唯一责任：**wasm-go执行协调模块；任务文件集成责任由`WG-I01`承担。自然人/代班/期限待分配。
**阶段与状态：**P1产品/工具实现；待实施，验收`not_run`。
**最小前置：**[D0-WG-01](https://github.com/wooveep/wasm-go/issues/1) @ `loader`；[D0-MF-02](https://github.com/wooveep/ocloudmodelfusion/issues/7) @ `identity`

依赖中的`@片段`是可单独合入/验收的交付，不等上游Issue关闭。消费前记录上游提交、路径、摘要、测试回执；字段未冻结可先准备案例/桩，不能据此接真实产品。

**可直接修改的文件落点（相对本仓）：**

- `extensions/ai-proxy/main.go` — 现有文件。
- `extensions/ai-proxy/execution_selection.go` — 拟新增文件/目录。
- `extensions/ai-proxy/execution_selection_test.go` — 拟新增文件/目录。
- `pkg/wrapper/cluster_wrapper.go` — 现有文件。

**输入：**WG-I00；SDK目标descriptor桩即可开发，真实验收依赖SDK隔离编译及IF-07完整发布
**输出：**`extensions/ai-proxy/main.go`、`provider/failover.go`、`provider/provider.go`、`pkg/wrapper/cluster_wrapper.go`；产生唯一ExecutionSelection，固定target/credential，安装而非重新抽Key

**验收条件：**IF01-V01/V03/V05；消费后改变目标/Key停止；同provider不同binding不互相覆盖

**共享文件责任：**main.go/init/StreamDone注册唯一集成；provider.go/failover结构由I07先交；retry由I02。
**实施/合入边界：**先WG-I01.selection-api供其余卡；main最终按wrapper、body、validation、facts接口串行合入，不能四分支抢main。

**最终联合验收协作项：**[WG-I07](https://github.com/wooveep/wasm-go/issues/12)、[WG-I08](https://github.com/wooveep/wasm-go/issues/13)、[WG-I09](https://github.com/wooveep/wasm-go/issues/14)、[MF-SDK01](https://github.com/wooveep/ocloudmodelfusion/issues/19)、[MF-REL02](https://github.com/wooveep/ocloudmodelfusion/issues/21)。这里要求相应接口/证据就绪，不要求对方整卡先关闭；对等验证可以同一轮取证，各方分别记录回执。

**交付检查：**

- [ ] PR仅含本卡直接白名单；共享文件变更需求由指定任务串行合入，未登记新文件先更新本卡。
- [ ] 输入锁/字段映射、正例与拒绝反例、预期和实际结果可复核；注明测试命令、cwd、版本及证据路径。
- [ ] 先交独立模块/片段，再接共享文件，最后按本卡实际范围执行联合验收；桩结果与真实运行结果分开。
- [ ] 未运行保持not_run，环境/ABI不足保持blocked；不以文档、schema、旧测试或模拟宿主冒充真实通过。
- [ ] 关闭时回填PR/提交、测试结果和依赖片段回执；汇总卡须子卡全部通过，不能据部分profile关闭完整范围。

来源：`wooveep/wasm-go:docs/2026-10-25-interface-implementation-plan.md`；本卡细化见本仓`docs/2026-10-25-gap-dispatch-task-cards.md`。

## WG-I02：BODY正文接缝、Outcome分类与一次派发决策

Issue：[WG-I02](https://github.com/wooveep/wasm-go/issues/5)。

**唯一责任：**wasm-go派发消费客户端模块；任务文件集成责任由`WG-I02`承担。自然人/代班/期限待分配。
**阶段与状态：**P1产品/工具实现；待实施，验收`not_run`。
**最小前置：**[D0-WG-03](https://github.com/wooveep/wasm-go/issues/4) @ `host-adapter`；[D0-MF-02](https://github.com/wooveep/ocloudmodelfusion/issues/7) @ `outcome`

依赖中的`@片段`是可单独合入/验收的交付，不等上游Issue关闭。消费前记录上游提交、路径、摘要、测试回执；字段未冻结可先准备案例/桩，不能据此接真实产品。

**可直接修改的文件落点（相对本仓）：**

- `pkg/wrapper/plugin_wrapper.go` — 现有文件。
- `pkg/iface/context.go` — 现有文件。
- `extensions/ai-proxy/provider/retry.go` — 现有文件。
- `extensions/ai-proxy/response_decision.go` — 拟新增文件/目录。
- `extensions/ai-proxy/response_decision_test.go` — 拟新增文件/目录。
- `pkg/wrapper/response_body_observation_test.go` — 拟新增文件/目录。
- `extensions/ai-proxy/provider/retry_decision.go` — 拟新增文件/目录。
- `extensions/ai-proxy/provider/retry_decision_test.go` — 拟新增文件/目录。

**输入：**WG-I00、Console Consume桩；最终依赖WG-I01及真实Console事务
**输出：**`extensions/ai-proxy/provider/retry.go`及建议新增dispatch消费模块；结构化Outcome、一次回调放行、201/202/未知停止创建；与SDK单发配置关联

**验收条件：**IF02-V03/V04/V06；Observe不能变成许可；丢grant零次或最多一次收件且如实unknown

**共享文件责任：**wrapper/iface正文接缝和retry唯一归I02；main注册交I01；事实只调I03.codec。
**实施/合入边界：**先I02.wrapper接口；真实BODY-V01–07验headers暂停/EOS/64KiB/截断/StreamDone/迟到回调，无额外发送；失败仅禁对应retry。

**CodeGraph复核后的验证细节：**

**验收：**BODY-V01–07全矩阵：EOS前零次第二发送；EOS后至多一次；无正文不挂起；64KiB±1、读失败、设置失败、截断、StreamDone/晚callback不双Resume/派发；首次失败与后次成功usage分别记账；媒体201/202/unknown仍单发；成功SSE保持增量。依赖能力无法证明时关闭该正文依赖retry profile，不扩展为全局暂停或修改Higress核心。

**源码定位：**ai-proxy/main.go459–503在headers直接OnRequestFailed/DontReadResponseBody；wrapper976–978忽略buffer属性结果，1179–1182/1211–1214读失败直接Continue；main106–116无StreamDone注册。BODY设计锚点：ocloudmodelfusion/docs/2026-10-24-attempt-execution-event-design.md §APP-02 首次失败正文与重试时序。

**最终联合验收协作项：**[WG-I01](https://github.com/wooveep/wasm-go/issues/3)、[WG-I03](https://github.com/wooveep/wasm-go/issues/6)、[MF-MED01](https://github.com/wooveep/ocloudmodelfusion/issues/8)、[DEP-IF01](https://github.com/wooveep/modelfusion-deploy/issues/4)、[DEP-IF04](https://github.com/wooveep/modelfusion-deploy/issues/8)。这里要求相应接口/证据就绪，不要求对方整卡先关闭；对等验证可以同一轮取证，各方分别记录回执。

**交付检查：**

- [ ] PR仅含本卡直接白名单；共享文件变更需求由指定任务串行合入，未登记新文件先更新本卡。
- [ ] 输入锁/字段映射、正例与拒绝反例、预期和实际结果可复核；注明测试命令、cwd、版本及证据路径。
- [ ] 先交独立模块/片段，再接共享文件，最后按本卡实际范围执行联合验收；桩结果与真实运行结果分开。
- [ ] 未运行保持not_run，环境/ABI不足保持blocked；不以文档、schema、旧测试或模拟宿主冒充真实通过。
- [ ] 关闭时回填PR/提交、测试结果和依赖片段回执；汇总卡须子卡全部通过，不能据部分profile关闭完整范围。

来源：`wooveep/wasm-go:docs/2026-10-25-interface-implementation-plan.md`；本卡细化见本仓`docs/2026-10-25-gap-dispatch-task-cards.md`。

## WG-I03：逐attempt事实编码与同步交付事件

Issue：[WG-I03](https://github.com/wooveep/wasm-go/issues/6)。

**唯一责任：**wasm-go事实模块；任务文件集成责任由`WG-I03`承担。自然人/代班/期限待分配。
**阶段与状态：**P1产品/工具实现；待实施，验收`not_run`。
**最小前置：**[D0-MF-03](https://github.com/wooveep/ocloudmodelfusion/issues/9) @ `facts`；[D0-WG-01](https://github.com/wooveep/wasm-go/issues/1) @ `loader`

依赖中的`@片段`是可单独合入/验收的交付，不等上游Issue关闭。消费前记录上游提交、路径、摘要、测试回执；字段未冻结可先准备案例/桩，不能据此接真实产品。

**可直接修改的文件落点（相对本仓）：**

- `pkg/ai/executionfacts/codec.go` — 拟新增文件/目录。
- `pkg/ai/executionfacts/codec_test.go` — 拟新增文件/目录。
- `extensions/ai-billing/main.go` — 现有文件。
- `extensions/ai-billing/execution_facts_test.go` — 拟新增文件/目录。
- `pkg/ai/executionfacts/envelope.go` — 拟新增文件/目录。
- `pkg/ai/executionfacts/stream.go` — 拟新增文件/目录。
- `pkg/ai/executionfacts/stream_test.go` — 拟新增文件/目录。
- `extensions/ai-billing/execution_fact.go` — 拟新增文件/目录。
- `extensions/ai-billing/execution_delivery.go` — 拟新增文件/目录。
- `extensions/ai-billing/execution_delivery_test.go` — 拟新增文件/目录。

**输入：**WG-I00及IF-03；可与WG-I01/02并行，单独合成事件不代表端到端通过
**输出：**生产共享事实编码、稳定投递、billing接入和只读观察适配；retry由WG-I02、main由WG-I01接线

**验收条件：**IF01-V02/V04及IF-03全部验收；收件数=独立attempt，重投不变ID/内容，可信更正新revision，不把最终usage复制到中间失败

**共享文件责任：**codec只读不可变snapshot；不改retry/main发送判定；发送点由I01/I02集成。
**实施/合入边界：**先I03.codec交付；真实router observer覆盖另需DEP实际ABI；事件投递失败按Q41显式缺口。

**CodeGraph复核后的验证细节：**

**输出/验收：**同事实重投ID/字节稳定、同ID异内容拒绝、补证不新建采购、cache delivery无采购、各attempt失败usage独立，缺status保留缺失；事件ACK丢失/重复不多计，未持久故障按Q41明确缺口。正常声明完整的路径少事实failed；故障案例如实暴露缺口且不重复资金可通过故障行为，不要求绝不丢失。跨仓固定样例可被Console接受/隔离；最终真实receipt+facts+账本联合后才关闭相应运行出口。

**源码定位：**ai-billing/main.go518–541每Context新event、保存当时cluster；776–804标delivered后XADD；807–895最终model/cluster构建旧事实；sessionctx572–596为另一cache/memory事件语义。不要在同一PR无差别更换所有旧事件格式。

**最终联合验收协作项：**[WG-I01](https://github.com/wooveep/wasm-go/issues/3)、[WG-I02](https://github.com/wooveep/wasm-go/issues/5)、[WG-I08](https://github.com/wooveep/wasm-go/issues/13)、[MF-FACT01](https://github.com/wooveep/ocloudmodelfusion/issues/11)、[DEP-IF01](https://github.com/wooveep/modelfusion-deploy/issues/4)、[DEP-IF03](https://github.com/wooveep/modelfusion-deploy/issues/7)。这里要求相应接口/证据就绪，不要求对方整卡先关闭；对等验证可以同一轮取证，各方分别记录回执。

**交付检查：**

- [ ] PR仅含本卡直接白名单；共享文件变更需求由指定任务串行合入，未登记新文件先更新本卡。
- [ ] 输入锁/字段映射、正例与拒绝反例、预期和实际结果可复核；注明测试命令、cwd、版本及证据路径。
- [ ] 先交独立模块/片段，再接共享文件，最后按本卡实际范围执行联合验收；桩结果与真实运行结果分开。
- [ ] 未运行保持not_run，环境/ABI不足保持blocked；不以文档、schema、旧测试或模拟宿主冒充真实通过。
- [ ] 关闭时回填PR/提交、测试结果和依赖片段回执；汇总卡须子卡全部通过，不能据部分profile关闭完整范围。

来源：`wooveep/wasm-go:docs/2026-10-25-interface-implementation-plan.md`；本卡细化见本仓`docs/2026-10-25-gap-dispatch-task-cards.md`。

## WG-I04：余额版本投影读取与未就绪拒绝

Issue：[WG-I04](https://github.com/wooveep/wasm-go/issues/7)。

**唯一责任：**wasm-go ai-quota模块；任务文件集成责任由`WG-I04`承担。自然人/代班/期限待分配。
**阶段与状态：**P1产品/工具实现；待实施，验收`not_run`。
**最小前置：**[D0-MF-03](https://github.com/wooveep/ocloudmodelfusion/issues/9) @ `balance`

依赖中的`@片段`是可单独合入/验收的交付，不等上游Issue关闭。消费前记录上游提交、路径、摘要、测试回执；字段未冻结可先准备案例/桩，不能据此接真实产品。

**可直接修改的文件落点（相对本仓）：**

- `extensions/ai-quota/main.go` — 现有文件。
- `extensions/ai-quota/balance_projection.go` — 拟新增文件/目录。
- `extensions/ai-quota/balance_projection_test.go` — 拟新增文件/目录。

**输入：**IF-05余额投影DTO/读协议；先用缓存响应桩并行，不依赖媒体页面
**输出：**`extensions/ai-quota/main.go`及对应schema；从现有裸整数GET迁移到IF-05就绪/版本/金额校验；缺失、错误、未就绪拒绝受影响新调用

**验收条件：**IF-05验收中读端用例、APP-V9；后台旧事实/查询不被误当新销售准入拦截；不使用allow missing绕过

**共享文件责任：**余额只读，spec/go.mod归I05；维护状态唯一Console。
**实施/合入边界：**

**最终联合验收协作项：**[MF-CACHE01](https://github.com/wooveep/ocloudmodelfusion/issues/15)、[DEP-IF06](https://github.com/wooveep/modelfusion-deploy/issues/11)。这里要求相应接口/证据就绪，不要求对方整卡先关闭；对等验证可以同一轮取证，各方分别记录回执。

**交付检查：**

- [ ] PR仅含本卡直接白名单；共享文件变更需求由指定任务串行合入，未登记新文件先更新本卡。
- [ ] 输入锁/字段映射、正例与拒绝反例、预期和实际结果可复核；注明测试命令、cwd、版本及证据路径。
- [ ] 先交独立模块/片段，再接共享文件，最后按本卡实际范围执行联合验收；桩结果与真实运行结果分开。
- [ ] 未运行保持not_run，环境/ABI不足保持blocked；不以文档、schema、旧测试或模拟宿主冒充真实通过。
- [ ] 关闭时回填PR/提交、测试结果和依赖片段回执；汇总卡须子卡全部通过，不能据部分profile关闭完整范围。

来源：`wooveep/wasm-go:docs/2026-10-25-interface-implementation-plan.md`；本卡细化见本仓`docs/2026-10-25-gap-dispatch-task-cards.md`。

## WG-I05：插件配置契约与可追溯构建依赖

Issue：[WG-I05](https://github.com/wooveep/wasm-go/issues/8)。

**唯一责任：**wasm-go配置契约模块；任务文件集成责任由`WG-I05`承担。自然人/代班/期限待分配。
**阶段与状态：**P1产品/工具实现；待实施，验收`not_run`。
**最小前置：**[D0-MF-05](https://github.com/wooveep/ocloudmodelfusion/issues/16) @ `release`

依赖中的`@片段`是可单独合入/验收的交付，不等上游Issue关闭。消费前记录上游提交、路径、摘要、测试回执；字段未冻结可先准备案例/桩，不能据此接真实产品。

**可直接修改的文件落点（相对本仓）：**

- `resources/plugins/ai-proxy/spec.yaml` — 现有文件。
- `resources/plugins/ai-billing/spec.yaml` — 现有文件。
- `resources/plugins/ai-quota/spec.yaml` — 现有文件。
- `resources/plugins/ai-cache/spec.yaml` — 现有文件。
- `resources/plugins/ai-memory/spec.yaml` — 现有文件。
- `extensions/ai-proxy/go.mod` — 现有文件。
- `extensions/ai-billing/go.mod` — 现有文件。
- `docs/release-evidence/build-inputs.md` — 拟新增文件/目录。

**输入：**IF-07制品/配置清单，WG-I00；可先schema/样例并行，实际证据依赖构建制品与SDK输出
**输出：**`resources/plugins/ai-proxy/spec.yaml`、`ai-billing/spec.yaml`、`ai-quota/spec.yaml`及相应README；operation、schema版本、单发配置和摘要证据

**验收条件：**不支持组合阻止发布；真实制品/运行配置可关联；业务测试不能拿仓库源码版本冒充加载版本

**共享文件责任：**公共spec/实际涉及模块go.mod/go.sum及工具链锁唯一I05，具体新增依赖先登记；其他卡禁止各修replace。
**实施/合入边界：**preflight逐模块GOWORK=off go list -m -json all；修正旧绝对replace并冻结来源；所有实际模块版本和Wasm摘要进入证据。

**最终联合验收协作项：**[ART-04](https://github.com/wooveep/plugin-server/issues/5)、[MF-SDK01](https://github.com/wooveep/ocloudmodelfusion/issues/19)、[DEP-IF04](https://github.com/wooveep/modelfusion-deploy/issues/8)。这里要求相应接口/证据就绪，不要求对方整卡先关闭；对等验证可以同一轮取证，各方分别记录回执。

**交付检查：**

- [ ] PR仅含本卡直接白名单；共享文件变更需求由指定任务串行合入，未登记新文件先更新本卡。
- [ ] 输入锁/字段映射、正例与拒绝反例、预期和实际结果可复核；注明测试命令、cwd、版本及证据路径。
- [ ] 先交独立模块/片段，再接共享文件，最后按本卡实际范围执行联合验收；桩结果与真实运行结果分开。
- [ ] 未运行保持not_run，环境/ABI不足保持blocked；不以文档、schema、旧测试或模拟宿主冒充真实通过。
- [ ] 关闭时回填PR/提交、测试结果和依赖片段回执；汇总卡须子卡全部通过，不能据部分profile关闭完整范围。

来源：`wooveep/wasm-go:docs/2026-10-25-interface-implementation-plan.md`；本卡细化见本仓`docs/2026-10-25-gap-dispatch-task-cards.md`。

## WG-I06：执行计量与全插件组合联合验收

Issue：[WG-I06](https://github.com/wooveep/wasm-go/issues/11)。

**唯一责任：**wasm-go验证模块；任务文件集成责任由`WG-I06`承担。自然人/代班/期限待分配。
**阶段与状态：**P1产品/工具实现；待实施，验收`not_run`。
**最小前置：**[D0-WG-04](https://github.com/wooveep/wasm-go/issues/10) @ `offline-report`

依赖中的`@片段`是可单独合入/验收的交付，不等上游Issue关闭。消费前记录上游提交、路径、摘要、测试回执；字段未冻结可先准备案例/桩，不能据此接真实产品。

**可直接修改的文件落点（相对本仓）：**

- `docs/release-evidence/platform-execution-matrix.md` — 拟新增文件/目录。
- `tests/platform_execution/README.md` — 拟新增文件/目录。

**输入：**WG-I01–05/07、Console派发/账务/余额、SDK隔离、deploy独立收件桩、plugin-server不可变包
**输出：**本仓测试＋外部APP-V0–V10证据引用，故障矩阵IF01/IF02；不新增第二套账务oracle

**验收条件：**收件→attempt→销售/采购明细逐笔吻合；全部阻塞能力有通过证据或明确禁用配置；仅单测通过不能关闭工作包

**共享文件责任：**仅验证/证据引用；产品修改仍I01–09原owner；独立oracle归DEP。
**实施/合入边界：**真实出口等待所声明profile的I01–09、Console/SDK/ART/DEP；对应required_cases未齐不得关闭。

按DEP-IF08预先冻结的W00–W12/profile required_cases合流；覆盖全部已确认范围。逐profile采证，不等全局全绿才开始；缺项保持not_run/blocked，不因某单发supported关闭完整任务。

**交付检查：**

- [ ] PR仅含本卡直接白名单；共享文件变更需求由指定任务串行合入，未登记新文件先更新本卡。
- [ ] 输入锁/字段映射、正例与拒绝反例、预期和实际结果可复核；注明测试命令、cwd、版本及证据路径。
- [ ] 先交独立模块/片段，再接共享文件，最后按本卡实际范围执行联合验收；桩结果与真实运行结果分开。
- [ ] 未运行保持not_run，环境/ABI不足保持blocked；不以文档、schema、旧测试或模拟宿主冒充真实通过。
- [ ] 关闭时回填PR/提交、测试结果和依赖片段回执；汇总卡须子卡全部通过，不能据部分profile关闭完整范围。

来源：`wooveep/wasm-go:docs/2026-10-25-interface-implementation-plan.md`；本卡细化见本仓`docs/2026-10-25-gap-dispatch-task-cards.md`。

## WG-I07：幂等冷却注册、健康快照与恢复claim消费

Issue：[WG-I07](https://github.com/wooveep/wasm-go/issues/12)。

**唯一责任：**wasm-go冷却模块；任务文件集成责任由`WG-I07`承担。自然人/代班/期限待分配。
**阶段与状态：**P1产品/工具实现；待实施，验收`not_run`。
**最小前置：**[D0-MF-02](https://github.com/wooveep/ocloudmodelfusion/issues/7) @ `identity`；[MF-HEALTH01](https://github.com/wooveep/ocloudmodelfusion/issues/31) @ `state-api`

依赖中的`@片段`是可单独合入/验收的交付，不等上游Issue关闭。消费前记录上游提交、路径、摘要、测试回执；字段未冻结可先准备案例/桩，不能据此接真实产品。

**可直接修改的文件落点（相对本仓）：**

- `extensions/ai-proxy/provider/failover.go` — 现有文件。
- `extensions/ai-proxy/provider/provider.go` — 现有文件。
- `extensions/ai-proxy/config/config.go` — 现有文件。
- `extensions/ai-proxy/provider/health_snapshot.go` — 拟新增文件/目录。
- `extensions/ai-proxy/provider/health_snapshot_test.go` — 拟新增文件/目录。

**输入：**IF-01身份/IF-02必要恢复claim、[现行简化恢复](../../../../ocloudmodelfusion/docs/2026-10-25-simple-upstream-recovery-design.md)与Console权威状态接口；可先按状态样例并行
**输出：**`extensions/ai-proxy/provider/failover.go`由本包拥有健康注册/状态合并区域；完整配置反复加载不清冷却，取消独立付费探测，真实请求恢复；选择消费方接口供WG-I01调用

**验收条件：**首次429按scope冷却30秒、连续恢复失败退避至300秒且长Retry-After优先；重启/旧成功不复活撤销Key；全冷却新请求503/媒体原期限等待；既有COOL/STORE及IF02-V05通过

**共享文件责任：**failover/provider/config健康公共结构由I07先交；I01提交选择/安装需求，不同时改结构；main由I01接。
**实施/合入边界：**snapshot可用固定响应桩开发，真实分发依HEALTH02；不要求MED完整页面，媒体claim联合依MED01。

**CodeGraph复核后的验证细节：**

**输出/验收：**幂等配置注册、权威快照+pending本地否决合并、有效候选查询（不再次随机）、恢复claim归属及事件ACK接口。重复Complete不清429；撤销/旧成功/旧outbox不复活；首次30秒后30/60/120/240/300退避和更长Retry-After；全冷却新请求503；两个Pod同epoch唯一恢复者、租约到期不重发原媒体。已允许的分发窗口如实记录，不宣称撤销瞬间零发送。取消独立付费探测。

**源码定位：**failover141–223 reset/timer/probe；672–730从本地available列表选Key；686–696清状态；config.Complete调用SetApiTokensFailover。REG-M1–5/MERGE-M1–4是设计权威，不重建新恢复系统。

**最终联合验收协作项：**[MF-HEALTH01](https://github.com/wooveep/ocloudmodelfusion/issues/31)、[MF-HEALTH02](https://github.com/wooveep/ocloudmodelfusion/issues/32)、[MF-MED01](https://github.com/wooveep/ocloudmodelfusion/issues/8)、[WG-I01](https://github.com/wooveep/wasm-go/issues/3)。这里要求相应接口/证据就绪，不要求对方整卡先关闭；对等验证可以同一轮取证，各方分别记录回执。

**交付检查：**

- [ ] PR仅含本卡直接白名单；共享文件变更需求由指定任务串行合入，未登记新文件先更新本卡。
- [ ] 输入锁/字段映射、正例与拒绝反例、预期和实际结果可复核；注明测试命令、cwd、版本及证据路径。
- [ ] 先交独立模块/片段，再接共享文件，最后按本卡实际范围执行联合验收；桩结果与真实运行结果分开。
- [ ] 未运行保持not_run，环境/ABI不足保持blocked；不以文档、schema、旧测试或模拟宿主冒充真实通过。
- [ ] 关闭时回填PR/提交、测试结果和依赖片段回执；汇总卡须子卡全部通过，不能据部分profile关闭完整范围。

来源：`wooveep/wasm-go:docs/2026-10-25-interface-implementation-plan.md`；本卡细化见本仓`docs/2026-10-25-gap-dispatch-task-cards.md`。

## WG-I08：响应缓存、记忆正文与交付计量接缝

Issue：[WG-I08](https://github.com/wooveep/wasm-go/issues/13)。

**唯一责任：**wasm-go上下文集成；任务文件集成责任由`WG-I08`承担。自然人/代班/期限待分配。
**阶段与状态：**P1产品/工具实现；待实施，验收`not_run`。
**最小前置：**[D0-MF-02](https://github.com/wooveep/ocloudmodelfusion/issues/7) @ `body`；[D0-MF-03](https://github.com/wooveep/ocloudmodelfusion/issues/9) @ `facts`

依赖中的`@片段`是可单独合入/验收的交付，不等上游Issue关闭。消费前记录上游提交、路径、摘要、测试回执；字段未冻结可先准备案例/桩，不能据此接真实产品。

**可直接修改的文件落点（相对本仓）：**

- `pkg/ai/bodybasis/bodybasis.go` — 拟新增文件/目录。
- `pkg/ai/bodybasis/bodybasis_test.go` — 拟新增文件/目录。
- `extensions/ai-cache/cache_key.go` — 现有文件。
- `extensions/ai-cache/main.go` — 现有文件。
- `extensions/ai-memory/request_injection.go` — 现有文件。
- `extensions/ai-memory/trusted_cache_digest.go` — 现有文件。
- `extensions/ai-cache/core.go` — 现有文件。
- `extensions/ai-cache/materialized_record.go` — 现有文件。
- `extensions/ai-cache/console_lookup.go` — 现有文件。
- `extensions/ai-cache/cache_event.go` — 现有文件。
- `extensions/ai-cache/execution_delivery.go` — 拟新增文件/目录。
- `extensions/ai-cache/execution_delivery_test.go` — 拟新增文件/目录。
- `extensions/ai-memory/main.go` — 现有文件。
- `extensions/ai-memory/cache_replay.go` — 现有文件。
- `extensions/ai-memory/body_basis.go` — 拟新增文件/目录。
- `extensions/ai-memory/body_basis_test.go` — 拟新增文件/目录。

**输入：**IF01/03和SDK01策略描述即可stub开发；WG-I01拥有目标安装，WG-I03拥有事实编码；Console内部任务由MF-INTERNAL01另行生产事实
**输出：**`extensions/ai-cache/cache_key.go`及缓存命中入口、`extensions/ai-memory/request_injection.go`、`extensions/ai-memory/trusted_cache_digest.go`及共享正文上下文；开工锁定新增模块文件清单。输入IF01 BodyBasis、SDK策略指纹和IF03，输出原正文基准、最终outbound摘要及cache delivery

**验收条件：**首次变换前保存BodyBasis，记忆/mapper后记录实际出站正文；策略变化隔离缓存。命中产生delivery且零上游attempt；已经发送后本地返回不冒充零采购缓存命中；stream/nonstream、注入开关、重入/重试均有精确正文与计量证据

**共享文件责任：**新增pkg/ai/bodybasis独占，不另改CommonHttpCtx；proxy main由I01接，billing编码I03，spec/I05。
**实施/合入边界：**I08.body先交；原BodyBasis首次变换前固定，最终出站另摘要；cache命中有delivery无主call采购，内部调用另计，不能把已发送本地返回当纯命中。

**CodeGraph复核后的验证细节：**

**验收：**原body三chunk/空body/错误读取、注入有无、map/slice后改、同call重入、不同call复用Context编号；策略/目标映射变化不误命中旧缓存；cache命中只有delivery且无采购attempt，已实际发送后本地返回保留采购；stream/nonstream与普通响应的销售用量解释一致。独立桩核对最终发送字节；模型mock仅验证本模块，实际插件顺序仍需DEP/SDK证据。对应MU07–09、APP-V6、EB01-V02/V03。

**源码定位：**cache_key.go42–73只组合route/model/requestDigest/cachePolicyVersion；cache main170–242在body阶段lookup；core29–64可Resume或replay；memory request_injection12–70实际改body并写digest；trusted_cache_digest28–70使用独立内容摘要。现有shared map不能自然贯穿这些插件。

**最终联合验收协作项：**[WG-I01](https://github.com/wooveep/wasm-go/issues/3)、[WG-I03](https://github.com/wooveep/wasm-go/issues/6)、[MF-SDK01](https://github.com/wooveep/ocloudmodelfusion/issues/19)、[DEP-IF04](https://github.com/wooveep/modelfusion-deploy/issues/8)。这里要求相应接口/证据就绪，不要求对方整卡先关闭；对等验证可以同一轮取证，各方分别记录回执。

**交付检查：**

- [ ] PR仅含本卡直接白名单；共享文件变更需求由指定任务串行合入，未登记新文件先更新本卡。
- [ ] 输入锁/字段映射、正例与拒绝反例、预期和实际结果可复核；注明测试命令、cwd、版本及证据路径。
- [ ] 先交独立模块/片段，再接共享文件，最后按本卡实际范围执行联合验收；桩结果与真实运行结果分开。
- [ ] 未运行保持not_run，环境/ABI不足保持blocked；不以文档、schema、旧测试或模拟宿主冒充真实通过。
- [ ] 关闭时回填PR/提交、测试结果和依赖片段回执；汇总卡须子卡全部通过，不能据部分profile关闭完整范围。

来源：`wooveep/wasm-go:docs/2026-10-25-interface-implementation-plan.md`；本卡细化见本仓`docs/2026-10-25-gap-dispatch-task-cards.md`。

## WG-I09：validation许可客户端与可信主体准入

Issue：[WG-I09](https://github.com/wooveep/wasm-go/issues/14)。

**唯一责任：**wasm-go准入集成；任务文件集成责任由`WG-I09`承担。自然人/代班/期限待分配。
**阶段与状态：**P1产品/工具实现；待实施，验收`not_run`。
**最小前置：**[D0-MF-05](https://github.com/wooveep/ocloudmodelfusion/issues/16) @ `validation`

依赖中的`@片段`是可单独合入/验收的交付，不等上游Issue关闭。消费前记录上游提交、路径、摘要、测试回执；字段未冻结可先准备案例/桩，不能据此接真实产品。

**可直接修改的文件落点（相对本仓）：**

- `extensions/ai-proxy/validation_access.go` — 拟新增文件/目录。
- `extensions/ai-proxy/validation_access_test.go` — 拟新增文件/目录。
- `extensions/ai-proxy/validation_lifecycle.go` — 拟新增文件/目录。
- `extensions/ai-proxy/validation_lifecycle_test.go` — 拟新增文件/目录。

**输入：**MF-REL02逐调用权威校验接口、MF-ID02测试主体；可用固定响应桩并行；实际验证要先完成此门再运行候选链
**输出：**拟新增共享validation准入客户端，由`extensions/ai-proxy/main.go`唯一接线；SDK01生成受控入口。消费IF07 ValidationAccess和已鉴权主体，输出本call准入结果

**验收条件：**在任何Consume/实际发送前校验主体、release/组合/profile/route_scope、revision和有效期；撤销/过期/接口不可达/伪造头拒绝，禁止跨call缓存允许结果；普通客户不进入validation。production按自己的支持/发布门开放，不复用测试许可

**共享文件责任：**本卡只独立客户端/拒绝逻辑；main唯一I01接，许可签发和撤销归MF-REL02。
**实施/合入边界：**桩可先行；真实联调依REL02.validation-check和ID02.principal；Consume/任何发送前验证，过期/撤销/断联/伪造均拒绝，不跨call缓存。

**CodeGraph复核后的验证细节：**

**验收：**普通客户/错测试主体/伪造header/旧revision/错release/profile/route_scope/过期/撤销/服务不可达全部拒绝；两pending反序、旧Context回调不得恢复新call；通过才允许当前call进入Consume/发送。候选插件链中的早返回（cache命中、provider为空、bound filter不匹配）必须在完整配置验证中核对，不能把校验放在可能被跳过的后段并称所有validation入口受保护。validation仍经过真实认证和计量链。对应IF07 §4.1、EB05-V07。

**源码定位：**ai-proxy/main.go217–239提前跳过，250–263按path/透明分类；wrapper1072先匹配后调用插件header。新模块本身测试通过不等于真实链路校验点已正确安装。

**最终联合验收协作项：**[MF-REL02](https://github.com/wooveep/ocloudmodelfusion/issues/21)、[MF-ID02](https://github.com/wooveep/ocloudmodelfusion/issues/6)、[MF-API01](https://github.com/wooveep/ocloudmodelfusion/issues/22)、[WG-I01](https://github.com/wooveep/wasm-go/issues/3)。这里要求相应接口/证据就绪，不要求对方整卡先关闭；对等验证可以同一轮取证，各方分别记录回执。

**交付检查：**

- [ ] PR仅含本卡直接白名单；共享文件变更需求由指定任务串行合入，未登记新文件先更新本卡。
- [ ] 输入锁/字段映射、正例与拒绝反例、预期和实际结果可复核；注明测试命令、cwd、版本及证据路径。
- [ ] 先交独立模块/片段，再接共享文件，最后按本卡实际范围执行联合验收；桩结果与真实运行结果分开。
- [ ] 未运行保持not_run，环境/ABI不足保持blocked；不以文档、schema、旧测试或模拟宿主冒充真实通过。
- [ ] 关闭时回填PR/提交、测试结果和依赖片段回执；汇总卡须子卡全部通过，不能据部分profile关闭完整范围。

来源：`wooveep/wasm-go:docs/2026-10-25-interface-implementation-plan.md`；本卡细化见本仓`docs/2026-10-25-gap-dispatch-task-cards.md`。
