# EB-01/02：执行身份与逐发送观测工程边界验证

更新：2026-10-25。状态：CodeGraph静态追链完成，验证设计待实现；所有离线用例、真实网关用例、构建与部署均未执行。本文只定义EB-01可信身份/mapper前选择/fallback/回调，以及EB-02逐发送观测/native retry/http_call两个边界；不扩展业务范围、不修改Higress核心，也不恢复网络门禁、Pod接流或复杂Redis恢复研究。

前置规则：[应用层基线](../../../ocloudmodelfusion/docs/2026-10-25-application-execution-billing-baseline.md)、[IF-01/02](../../../ocloudmodelfusion/docs/contracts/2026-10-25-execution-dispatch-contracts.md)、[IF-03事实](../../../ocloudmodelfusion/docs/contracts/2026-10-25-billing-facts-balance-contracts.md)、[IF-07发布](../../../ocloudmodelfusion/docs/contracts/2026-10-25-release-artifact-contract.md)、[IF-09证据](../../../modelfusion-deploy/docs/plans/2026-10-25-support-evidence-contract.md)。工作包映射见[wasm-go实施计划](2026-10-25-interface-implementation-plan.md)，跨项目合流见[五项边界协调计划](../../../docs/plans/2026-10-25-engineering-boundary-verification.md)。收件、故障、结果封装与独立oracle复用[共享FX夹具计划](../../../modelfusion-deploy/docs/plans/2026-10-25-boundary-fixtures-plan.md)，本文件不重定义上述DTO或业务政策。

## 1. 结论与证据分层

现有扩展点提供可复用候选，但没有源码依据可以直接宣称“可信身份贯穿”和“每次dispatch就是一个attempt”已成立。

| 边界 | 现有可复用 | 明确工程缺口 | 只能运行判定 |
| --- | --- | --- | --- |
| EB-01 | wrapper阶段回调、matcher route/service匹配；宿主Request生命周期属性候选；显式TargetCluster；custom_response重建入口 | 当前Context map非跨插件通道；匹配配置在header阶段缓存；Key仍可二次随机选择；原body与只读call须分阶段；本地grant gate未实现 | 实际镜像ABI、属性重入生命周期、mapper前固定目标是否保持、所有后续插件是否还能改目标/Key、迟到回调是否误恢复 |
| EB-02 | ai-proxy DispatchHttpCall发送/回调边界；宿主每UpstreamRequest上游过滤链、结束日志、上游Wasm工厂候选 | 仅200成功/缺status默认502；无完整attempt事实协议；上游filter共享父streamInfo；测试便利函数固定第一callout；隐藏重试控制仍须收敛 | 工厂是否编进镜像、cluster级链是否覆盖router级链、每次上游尝试能否获得独立关联/usage、callout是否发生额外发送、真实事件是否完整 |

源码结论标`静态已确认`，不是用例PASS；尚未验证的组合保持NOT_RUN。一次实验暴露缺陷可以证明“缺陷复现”，不能将产品验收标PASS。

## 2. CodeGraph实际调用链及定位

以下行号来自2026-10-25当前文件。CodeGraph返回源码或调用关系；少数工厂/头文件未被该查询返回，随后只补读定向片段。Higress源码是候选宿主行为，不等于实际运行镜像；model-mapper位于官方树，发布包来源必须独立核实，不能把其当前源码自动当成独立wasm-go制品。

### EB-01：准入到选路、映射和重入

1. [wrapper](../pkg/wrapper/plugin_wrapper.go) `OnHttpRequestHeaders:1021`先缓存path/host/method，再在`:1072–1092`调用matcher并将结果保存到`ctx.config`，之后才进插件header回调。`OnHttpRequestBody:1095–1124`复用这份config；完整body未到时Pause，读取失败却Continue。**body里改选目标，不能假定已经匹配的mapper/proxy会自动换配置。**
2. [matcher](../pkg/matcher/rule_matcher.go) `GetMatchConfig:127–179`先读route_name再读cluster_name，按规则首匹配返回；`serviceMatch:404–419`从cluster字符串拆FQDN/端口。无命中可用global或nil。`:258–288`规则解析失败可载入旧备份，`:429–445`按规则匹配键保存/读取；新release出现旧配置时必须被摘要校验发现。
3. [ai-proxy](../extensions/ai-proxy/main.go) `onHttpRequestHeader:217`取得已匹配provider，`:241–244`调用DisableReroute及initContext；[failover](../extensions/ai-proxy/provider/failover.go) `SetApiTokenInUse:721–730`仍选随机/亲和Key。[wrapper](../pkg/wrapper/plugin_wrapper.go) `DisableReroute:967–969`只是写`clear_route_cache=off`且忽略返回，不能据此证明全链冻结。
4. [model-mapper](../../../higress/plugins/wasm-go/extensions/model-mapper/main.go) `:130–162`按path/body决定是否读取并Pause；`:165–230`做正文模型映射，非法JSON/改写失败存在Continue，header/body替换返回未完整校验。模型选择的前置证据应在matcher读取之前成立；正文基准必须在mapper改变model之前保存。当前代码对重复顶层model在`:182–194`拒绝，可作现有正向防护验证，但不覆盖其余失败分支。
5. [宿主context.cc](../../../higress/envoy/envoy/source/extensions/common/wasm/context.cc) `:629–667`的route_name读取会触发route重求值，cluster_name可取当前route/host。`:1444–1468`SetProperty从**rootContext**取声明原型；`:1521–1529`同名再声明直接BadArgument。[foreign.cc](../../../higress/envoy/envoy/source/extensions/common/wasm/foreign.cc) `:310–341`的`declare_property`接受protobuf参数并在current_context声明；因此写者应在RootContext声明，不能假造JSON参数或在请求Context声明后假定root生效。[cel_state.h](../../../higress/envoy/envoy/source/extensions/filters/common/expr/cel_state.h) `:69–75`只读值初始化后禁止重写；默认生命周期`:47–51`是FilterChain，不是Request。
6. [redirect_policy.cc](../../../higress/envoy/envoy/source/extensions/http/custom_response/redirect_policy/redirect_policy.cc) `encodeHeaders:130`执行fallback响应路径；`:288–305`清route cache、改header、重选route，无route时Continue；`:327–343`写Request生命周期状态后recreateStream，失败仍Continue。**改过header不证明已重入，recreate失败也没有此处可直接复用的“安全回调旧协调器”保证。**媒体跨Context创建fallback首期继续禁用，不把新continuation协议作为本轮前置。
7. [wrapper](../pkg/wrapper/plugin_wrapper.go) `SetContext:799–805`只写当前map；`RouteCall:1234–1276`改当前请求并登记响应callback，返回nil仅表示本地过程返回，不证明派发/供应商接收。既有日志接口也不构成可信身份承载。

**两阶段适配待验证：**在header阶段建立不可变`CallIdentity`（可信销售/发布归属），在原始完整body收齐且任何变换前建立不可变`BodyBasis`（原body摘要、规范化版本、长度及call关联）。这两个名称是实现内部适配，不改变IF-01传输字段的必填要求。发送/Consume之前必须组合成完整ExecutionContext，且校验同call、同正文基准。不得为补body摘要重写只读call；空body有明确基准；旧Context、旧release或其他call的BodyBasis不得用于新发送。跨重入仅复用同call已验证的原基准，每个attempt另存最终上游body摘要，不把变换后正文再次当原正文。

### EB-02：发送、重试及观察

1. [ai-proxy](../extensions/ai-proxy/main.go) `onHttpResponseHeaders:459–504`在读取正文前，将非200/缺status送`OnRequestFailed`，部分路径随后DontReadResponseBody。[failover](../extensions/ai-proxy/provider/failover.go) `OnRequestFailed:699–713`分别判断冷却与retryOnStatus，后者调用`retryFailedRequest`。
2. [retry.go](../extensions/ai-proxy/provider/retry.go) `:56–61→125–185→192–234→82–121`形成“选替代Key→构建正文→DispatchHttpCall→回调→可能再次发送”的闭环。回调只认200，后续失败按次数继续而不重跑首次状态匹配；缺header/status默认502。`maxRetries=0`在`:33–41`会变为1，不能用0当关闭。需同时验证首次主响应与后续回调的201/202、截断、超时分类。
3. [宿主httpCall](../../../higress/envoy/envoy/source/extensions/common/wasm/context.cc) `:1094–1170`创建新RequestOptions，设置timeout/hash/XFF/parent span，但未设置父filter_state或parent_context.stream_info；[AsyncStreamImpl](../../../higress/envoy/envoy/source/common/http/async_client_impl.cc) `:108–154`在未传入时新建FilterChain状态并用指定cluster的NullRoute。**trace有父子关系不意味着call身份自动继承；callout不是下游完整过滤链重入。**
4. [retry_state_impl.cc](../../../higress/envoy/envoy/source/common/router/retry_state_impl.cc) `:28–59,107–144`允许请求头启用/覆盖重试，即使route没有retry；随后删除header是消费行为，不是阻止启用。[AsyncRequestImpl](../../../higress/envoy/envoy/source/common/http/async_client_impl.h) `:364–395`持有完整request body作为decodingBuffer；[router.cc](../../../higress/envoy/envoy/source/common/router/router.cc) `doRetry:2380`递增attempt_count、`:2437–2485`新建UpstreamRequest并复制正文。一次Wasm callout可能对应多个发送，须验证后才将它作为单attempt边界。
5. [upstream_request.cc](../../../higress/envoy/envoy/source/common/router/upstream_request.cc) `:46–60`的UpstreamFilterManager.streamInfo返回父流共享对象；`:80–160`每个UpstreamRequest新建上游过滤链，**cluster指定链优先于router指定链**，均不存在才默认codec；`:165–250`cleanUp记录单次stream_info及UpstreamEnd，`:380–426`还可能周期日志。每条日志不等于一个新attempt，进程退出也不保证结束日志存在。
6. [上游Wasm工厂](../../../higress/envoy/envoy/source/extensions/filters/http/wasm/config.cc) `:14–15`注册UpstreamHttpFilterConfigFactory。只说明本地源码有候选；实际镜像工厂、配置下发、scope覆盖、callback字段可用性仍须实验。不新增未经源码/实际ABI证明的“native retry callback”或“网络已接收hook”。
7. [ai-billing](../extensions/ai-billing/main.go) `BillingEvent:133`无完整call/attempt/release身份；`:776–804`先标Context已投递再XADD，失败fail-open。下游最终usage和最终cluster无法补回所有中间尝试；Q41可用优先不变，缺口必须单列。
8. [TestHost](../pkg/test/host.go) `:254–257`便利`CallOnHttpCall`总取第一条callout；乱序/两个未完成callout用例须从`:268–269`读取CalloutID后，使用已有底层HostEmulator.CallOnHttpCallResponse指定ID。现有[retry_test.go](../extensions/ai-proxy/provider/retry_test.go) `:23–27`getter返回默认，`:50–52`RouteCall空实现，`:105–125`只测提前返回，不证明真实重试路径通过。

## 3. 验证输入、输出与独立性

以下是**拟新增EB观察字段及共享夹具使用方式**，不表示已存在调试端点。CasePlan、输入冻结、receipt两阶段持久记录、独立oracle及结果封装，以共享FX-01/02/03和IF-09为权威；本表只增加本边界必需的输入维度与阶段观察。字符串ID、revision、时间和摘要编码沿九类公共约定；原始Key、生产请求正文、grant不得进入通用证据。所有额外阶段记录只能在测试构建/隔离验证环境使用，不增加生产绕过鉴权的接口。

| 对象 / 唯一生产者 | 必填输入/输出字段 | 用途与限制 |
| --- | --- | --- |
| CasePlan附加维度 / DEP-VERIFY | send_transport,mechanism,route_form,filter_chain_order[],upstream_protocol,worker_count,concurrency,seed；制品/config/release使用IF-09既有引用字段 | route_form=single_target/weighted/fallback；与FX冻结输入一并版本化，不能读被测事件后生成期望 |
| 驱动输入附加维度 / FX-01驱动器 | fixture_request_id使用共享编号；client_request_id,public_model,body_fixture_ref,original_body_digest,forged_headers[],chunk_schedule[] | fixture_request_id仅测试关联；客户端不能自选真实call/attempt归属。固定body序列，包含空body/重复model/截断/跨chunk；测试Key为固定无真实权限值 |
| PhaseSnapshot / 测试插件或测试构建的对应阶段 | sample_id,case_id,plugin_artifact_digest,phase,local_context_id,context_generation,gateway_boot_id,call_id?,selection_generation?,selection_digest?,route_name?,cluster_name?,mapper_digest?,credential_marker?,body_basis_digest?,outbound_body_digest?,hostcall_result | phase候选：entry_after_sanitize、before_match、after_match、body_basis_ready、before_consume、before_dispatch、callback、stream_done。**采样点是待实现测试能力**，用既有插件回调/显式发送调用处采样，不假定宿主提供上述名字的hook；读不到返回unavailable，不填期望值；credential_marker沿FX合成标识 |
| DispatchTrace / 发送者 | dispatch_trace_id,call_id,attempt_id,send_transport,mechanism,callout_id?,local_send_seq,context_generation,captured_selection_digest,dispatch_return,callback_count,send_state,acceptance,execution_result,headers_complete,body_complete | callout_id仅宿主局部关联，不能替代attempt；回调必须引用发送前私有快照，不能事后读取当前全局selection |
| Receipt引用 / FX-01独立上游桩 | 完整复用FX-01的receipt_id、receipt_sequence、endpoint_id、credential_marker及接收/结束字段；EB附加authority,tls_sni?,model解析结果 | 非法JSON仍保留共享收件记录；产品身份仅在真实受保护关联可读时记录；无receipt不能直接证明not_sent；零额外发送判据复用FX观察窗口及结束条件 |
| Observation / 对应实际owner | IF-03完整事件、sample_ref、producer/owner、send_transport、原snapshot引用、发送/接受/结果三维 | router观察器与ai-proxy不得同时报两份主采购事实；补证只能引用同attempt；记录真实缺status而非自动填502 |
| CaseResult / DEP-VERIFY经FX-02封装 | 使用IF-09案例结果与证据引用；追加断言计数observed_receipts,observed_primary_attempts,known_not_sent,unknown_attempts及block_scope | 一个判定记录；正文PASS/FAIL/BLOCKED/NOT_RUN对应共享序列化passed/failed/blocked/not_run；WASM机制解释与Console账务结果不能替代独立收件判定 |

受控响应脚本至少支持：200/201/202/503；有/无usage；202含任务ID/缺任务ID；headers完整但body截断；收件后reset/超时；按receipt序号延迟响应；流式首chunk后reset。fault_schedule明确屏障位置与释放顺序，随机kill仅压力补充。

共用三目标：P1/B1/M1、P1/B2/M2、P2/B3/M3；B1/B2同DNS但测试Key标识、model及采购绑定不同。初始控制组关闭缓存/记忆注入、粘性、健康变化、原生retry、凭据retry、fallback；一次只改变一个因素。HTTP/1.1、HTTP/2分别记录，HTTP/3只在计划支持时另列组合，不能从POST/H1结果外推其安全请求自动重试。

## 4. EB-01具体用例

执行顺序为01→02/03→04/05→06；各行L=离线Go/Wasm模拟，R=实际网关。所有行当前NOT_RUN。

| 用例 / 映射 | 步骤、正负例与控制变量 | 观测点及通过标准 | 失败范围 |
| --- | --- | --- | --- |
| EB01-V01 属性建立与外部伪造；WG-I00/01，IF01-V01/V05 | L：声明protobuf参数、RootContext成功/重复声明/错Context；R：固定单target，两插件写/读，外部伪造call/target/credential/retry字段；另先由错误插件用默认生命周期抢写同名值 | hostcall返回、写后读、接收插件值；可信call不受伪造改变；声明失败/类型冲突不得发送；同名声明不能因同内容误认为成功。R未做不能以mock属性Map作为PASS | 只阻塞依赖该可信载体的执行组合；不能偷偷改用裸header；可另验证受认证内部信封路径 |
| EB01-V02 两阶段正文；WG-I01/02，IF01-V03/V04 | L：先header后分3块body，空body、重复model、正文读取/改写失败；R：同输入经mapper/协议转换，再模拟同call重入及不同call复用Context编号 | CallIdentity只写一次；完整原body前没有Consume/发送；BodyBasis同call不可变，最终outbound摘要单存；错call/旧Context/不同正文基准拒绝；原body为空与未收到可区分 | 阻塞该body/operation profile；不必阻塞无body且已独立通过的查询 |
| EB01-V03 mapper前固定目标；WG-I01/05，APP-V1/V2，IF01-V01/V03 | L：route＋service首匹配、缺规则/global/旧backup；R：先单target各跑B1/B2/B3，再同DNS双绑定交替；分别在matcher前后改变目标header，实际配置固定 | before_match、after_match、mapper/proxy摘要、before_dispatch及receipt六点一致；后改选不得沿旧config误发；错配受管路径拒绝，旧备份被识别；保留Host/SNI/path正确性 | multi-target未证实时仅关闭该组合，保留经过同样核验的single-target；单target也错配则该profile不发布 |
| EB01-V04 route cache与fallback；WG-I01/03，APP-V5，IF01-V02/V03 | R：文本A失败→B成功；固定原body，B用不同mapper；旧target header残留、B无route、recreate失败、两跳/循环分别测；媒体只测fallback关闭负例 | call/销售资产不变，新实际发送才新attempt；B receipt匹配B；无route/recreate失败无虚构B采购；有界跳数；媒体201/202/未知零额外创建 | 文本fallback失败局部关闭fallback；媒体跨Context创建fallback保持禁用，本用例不承担将其开放的目标 |
| EB01-V05 异步回调快照与gate；WG-I02/03，IF02-V03/V06 | L：两callout按明确CalloutID反序回调；关闭Context后回调、重复grant、同Context编号新generation；R：Consume提交后丢响应/延迟到gate关闭，并重启原worker | 每callback只引用其发送前call/selection/body快照；旧generation不得Resume新Context；grant至多一次本地放行，Read(consumed)不能恢复；供应商至多一次创建，可能零且保持unknown | 阻塞受影响的async/gate路径；不冻结Console事务开发或同步单发验证 |
| EB01-V06 同call版本一致性；WG-I01/05，IF01-V03、IF07-V05 | R：A发送后切新release/重载mapper代理规则，A晚回；新call走新发布；把旧backup或不一致proxy摘要引入负例 | A回调/事实仍原release/target/Key版本；新call完整新集合；不混用当前配置给旧attempt定归属；身份冲突拒绝/隔离而非改账掩盖 | 阻塞该在线切换组合，可使用IF-07已定义维护发布；不引入Pod门禁替代 |

EB01-V01的readonly只防误写，不是任意插件恶意写入的安全沙箱。受管插件发布边界必须成立；没有可验证入口清理、写者及接收方校验，不把“SetProperty成功”标成可信身份通过。

## 5. EB-02具体用例

执行顺序为01→02/03→04→05/06；先一请求确定性序列，再并发。所有行当前NOT_RUN。

| 用例 / 映射 | 步骤、正负例与控制变量 | 观测点及通过标准 | 失败范围 |
| --- | --- | --- | --- |
| EB02-V01 单发观察覆盖；WG-I03/05，APP-V0/V1，IF01-V02 | L：事件编码与owner验证；R：加载上游Wasm候选，导出router及cluster上游过滤链；单发200、上游503、本地拒绝、连接前失败各跑，另加入cluster自定义链作为优先级负例 | 实际工厂/配置接受与行为证据齐全；每receipt一主attempt；本地拒绝不报供应商已发；观察器缺失/不可加载记BLOCKED而非默认成功；cluster链覆盖router补丁时能识别缺口 | 先阻塞该观察组合；原生retry不可观测则关闭；普通单发仍需独立成立的实际发送归属证明 |
| EB02-V02 原生retry私有快照；WG-I03，APP-V4，IF01-V02、IF03-V01 | R：原生503→200，再per-try超时A晚到/B先回；仅在计划支持hedge时增加重叠尝试；有不同usage、endpoint、返回model；其他重发关闭 | 每观察实例在创建/回调时捕获私有attempt/target/模型/usage来源；故意让父streamInfo变成后次attempt，前次结果仍不被覆盖；绝不能在最终onLog从最新共享streamInfo回填前次。每receipt关联不同attempt，失败成本独立 | 失败关闭native retry/hedge相应组合，不回退为最终usage乘次数；可保留已通过单发/应用retry |
| EB02-V03 callout隐藏重试；WG-I02/03/05，APP-V4/V10，IF02-V04 | R：固定TargetCluster POST，关闭显式应用retry；无header控制组与注入x-envoy-retry-on/max-retries/retriable-status/per-try/hedge头负例；503→200/收件reset脚本；普通/透明路径分别跑；H1/H2分别列组合 | sanitize后调用参数、真实cluster配置、callout返回/回调与receipt数；首期每dispatch最多一receipt，header不能启用第二层重发。无receipt时不虚构sent；出现额外发送即FAIL，不将同callout两收件合成一attempt | 不可关闭的隐藏retry阻塞该callout组合，不能只改计量标签继续开放媒体；无需阻塞可验证router单发 |
| EB02-V04 受理/传输分类；WG-I02/03，APP-V10，IF01-V04、IF02-V04 | L：主响应及指定callout分别201/202/200业务受理、缺status、正文截断/超限、callback读错；R：媒体single_send同一矩阵，另文本503→200应用retry组合 | acceptance、send_state、execution_result三维和完整性独立；201/202不是失败重试，未知不重建；缺status无真实502；媒体单次创建；文本每次实际发送各有事实，不因最后本地响应删成本 | 哪个operation/adapter分类失败就禁用其重发或发布，不扩大为所有供应商全部不可用 |
| EB02-V05 双观察与日志阶段；WG-I03，IF01-V02、IF03-V02/V04 | R：ai-proxy callout同时被上游链观察；打开PoolReady/Periodic/End核对，重复投递及ACK丢失；查询原provider_task结果多次 | 主owner仍ai-proxy，观察器补证同attempt；同日志阶段不额外采购；查询发送是自己的attempt，查询返回累计生成usage不新建生成attempt；源重投同ID/内容；无法安全关联补证则隔离，不按时间/相同正文自动合并 | 阻塞双观测关联方案；允许先只用已证明完整的唯一owner通路，不能关掉缺口监测 |
| EB02-V06 事件缺口与完成故障；WG-I03/06，APP-V6/V7，IF03-V05 | R：上游先持久receipt，延迟/丢XADD ACK、使投递失败、在receipt后结束记录前重启；另缓存命中对照 | receipt/attempt缺口报告可见，已存事实重放不多扣；业务按Q41继续但不能报告100%事实完整；真正缓存无receipt/attempt且有缓存证据，不能将丢事件算缓存/已证实零采购 | 故意注入Q41允许的不可恢复窗口时，缺口正确暴露且不重复资金可通过本故障行为案例，但不能声明采购事实完整；正常或声称完整路径缺事实、或故障缺口被隐藏则FAIL。不新增绝不丢失/全量调用拒绝前置 |

对重置/连接失败：收到receipt可证明供应商桩已经接收；未收到只表示观察中没有收件，须结合桩健康、完整采集区间和发送层证据分类，不能从“零行日志”推导“永远未发送”。对进程崩溃丢事件，记录残余不可恢复窗口，不承诺Q41下绝不丢失。

## 6. 离线、真实网关与复用入口

离线层复用`pkg/test/host.go`的明确Context/CalloutID及Go/Wasm模式，验证DTO、状态、摘要、操作分类、错误分支和callback不可重入。增加假宿主能力时必须单独标注模拟语义；不能让mock返回已通过预期值后声称真实Envoy支持Request属性、路由重算或upstream过滤链。

真实网关层使用独立供应商桩，不使用付费模型。未来执行前按[本地环境权威入口](../../../modelfusion-deploy/docs/local-test-environment.md)选默认VM K3S并盘点真实镜像/包；本轮不连接环境。镜像、Wasm、controller、Go依赖最终解析路径和提交均进入CasePlan；旧路径replace或“源码存在但包不是该来源”必须先修正基线，不能从源码标签替代加载摘要。

第一轮B只记录现状：预期缺口复现为FAIL，无法加载所需夹具/能力为BLOCKED，未执行为NOT_RUN。第二轮A使用同CasePlan语义与相同负例，实施后复验。修改能力范围必须新组合键/计划版本，不能删除失败场景后沿用原PASS。日志、XADD ACK、网关最终200均只是一层证据，不代替独立receipt＋事实接受结果。

## 7. 并行任务、硬依赖与唯一判定

| 任务 / 当前状态 | 唯一实施owner | 输入与并行范围 | 交付及硬依赖 |
| --- | --- | --- | --- |
| EB-W01 身份/两阶段body夹具 / 待实施 | WG-I01执行协调模块 | IF-01样例、宿主声明协议；可与Console事务及SDK编译并行 | EB01-V01/V02离线结果；真实通过硬依赖实际包/ABI及可信入口，不依赖完整账务 |
| EB-W02 匹配/冻结/重入夹具 / 待实施 | WG-I01执行协调模块 | SDK三目标运行配置、WG-I05加载记录；离线matcher测试可先行 | EB01-V03/V04/V06；真实通过依EB-W01身份载体、独立桩及发布完整性；媒体跨Context仍关闭 |
| EB-W03 回调/gate/Outcome / 待实施 | WG-I02派发客户端模块 | IF-02 Consume桩、指定CalloutID helper；可与EB-W01和Console事务并行 | EB01-V05/EB02-V04；真实gate需Console实际Consume唯一性，不能只用返回固定grant的桩验端到端 |
| EB-W04 上游观察与主事实 / 待实施 | WG-I03事实模块 | 上游工厂候选、IF-03、私有快照结构；独立开发观察器，不改核心 | EB02-V01/V02/V05/V06；真实通过依实际工厂/链覆盖；不能从共享streamInfo补所有attempt |
| EB-W05 单dispatch单发送 / 待实施 | WG-I02派发客户端模块 | 关闭隐藏retry的SDK完整配置、透明/普通参数捕获；与EB-W04并行 | EB02-V03；是否多发由receipt裁决；无法单发只阻塞该callout能力 |
| EB-D01 共享夹具接入/结果裁决 / 待实施 | DEP-VERIFY | 直接复用FX-01收件脚本、FX-02证据封装、FX-03独立oracle、FX-06发布观察；EB仅追加本文件阶段观察；最先并行准备 | 独立持久receipt、故障屏障、合并证据索引、每case判定；不另建收件格式或验证器，不从插件event生成期望 |
| EB-C01 事实接受检查 / 待实施 | CON-BILL事实接受模块 | IF-03及合成样例可独立 | 接受/重复/隔离、采购聚合结果；端到端通过依真实发送链，计算器开发不等待EB全部通过 |

文件所有权延续WG计划：WG-I01拥有选择/安装，WG-I02拥有retry回调/gate，WG-I03提供事件助手；共享发送点由执行模块集成，避免三个包分别重写retry循环。SDK拥有配置编译，DEP拥有真实收件与证据索引。每case的**验收判定唯一owner是DEP-VERIFY**；对应WG模块对技术修复负责，CON-REL只消费结果决定可发布组合，不能将UNKNOWN改为ready。事实生产者仍按send_transport唯一：router_observer或ai_proxy，采集者不得改变主事实owner。

`PASS`须该case全部断言成立、证据齐全且版本匹配；`FAIL`有已执行反例；`BLOCKED`已尝试执行但缺必要工具/ABI/实现/证据，列缺口和解阻owner；`NOT_RUN`未执行。离线PASS与真实网关PASS分别登记，不互相覆盖；没有实际运行时，本文件所有case继续NOT_RUN。

局部阻塞的最小单位为`operation/profile × send_transport × retry/fallback能力 × 发布版本 × 协议`。EB-01多目标失败不默认取消单目标；EB-02原生retry失败不默认取消已证明单发；但同一profile的可信归属/单发底线失败时不得开放该profile。默认动作是缩减到已验证组合或保持该入口不可用，不新增底层门禁工程，不修改核心来凑通过。

## 8. 本轮产物与下一步

本轮新增此验证计划；静态发现包括header先匹配后body转换、只读call与body阶段不一致、callout不自动继承身份、父streamInfo非attempt私有、cluster链优先和CallOnHttpCall固定首callout。以上均已转成可判定正负例。下一步获实施授权后先交付EB-W01/03/04及EB-D01最小夹具，再按单target/单发→多target→可观测retry/fallback逐层验证；本轮没有构建、执行测试、连接集群或部署。
