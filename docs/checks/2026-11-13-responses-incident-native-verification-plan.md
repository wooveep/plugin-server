# Responses 补修的真实网关验证方案

性质：执行前方案；**尚未创建资源、发请求或注入故障**。使用开发 `192.168.29.200`，不操作灰度。当前已验收基线为 Redis-only Helm104；正式执行以主审后续单写者窗口的实际完整回读为准。账单 b514 及其旧104脚本继续冻结。

## 固定参数与新增资源

产品源码已冻结为 `dacd69e148f61330b713d1a4854578e11b7fdcc3`，两轴复核及完整制品尚待完成。候选为 ai-proxy **2.0.82**，实际 Wasm/archive/manifest/image 摘要届时填入执行回执；其余54项发行选择、已有282制品和正式发布均保留。

| 项目 | 计划值或数量 |
| --- | --- |
| 本次标识 | `responses-incident-20261113`，所有新建ID另记精确清单 |
| Mock工作负载 | `review-responses-incident-20261113` Deployment/Service/ConfigMap 各1，仅集群内8080 |
| Mock接口 | Chat Completions、Responses、Messages 的JSON/SSE及同步Embedding；每次原生ID唯一 |
| Mock计量 | 文本协议报告input17/output5；Embedding wire仅prompt_tokens17/total_tokens17，输出依input-only契约不适用，不伪造completion字段。均为Mock协议字段，不是真实Tokenizer计数 |
| 域名候选 | `incident-20261113.ai.lc.wooveep.com`，先查未占用及门户互斥，再按正常接口登记 |
| 独立身份 | 本次专属租户/用户/有效Key，不复用已过期RT Key；按正常审批/加入/准入流程建立 |
| 业务资源 | 专属供应商接入、三文本协议及Embedding采购绑定；文本与Embedding各自有销售资产/零价价本。路由/入口数量以SDK原生发布计划为准，逐ID记账 |
| Embedding模型 | 本次唯一公开名`incident-embedding-20261113`，映射到Mock上的已支持Embedding模型标识（采用既有qwen3.7-text-embedding-flash输入计量契约）；实际目标始终为Mock Service |
| 授权 | 仅本次Key到本次两个资产的正常授权；不改现有授权或身份资格 |
| 财务 | 销售/采购完整价本均为明确0，保留真实计量/事实；不改既有钱包或追加充值绕过准入 |
| 网络 | 上游仅自有Mock Service，不连接真实付费供应商 |

先确认零价模型按原准入规则可用；不通过手填数据库、伪造准入头、放宽健康/吊销快照或改变Redis配置取得通行。新建资源和后续审计记录单独列出，不能误称环境原始全表完全不变。

## 一次实际验证范围

拟执行6次小输入正常请求（3协议×JSON/SSE）、3次约1MiB大请求、3次定向别名写失败（Responses auto SSE、raw SSE、raw JSON）及1次正常Embedding，合计 **13次Mock生成**。每格一次，客户端关闭自动重试；失败如实留证，不循环补到成功。续接拒绝验证另计最多3次网关请求，必须零Mock生成发送。它们是本次功能验证，不重开旧I22唯一RT采样额度，也不更改原call/attempt记录。

6个正常样本中的Responses JSON兼作合法种子：小输入，Mock返回1090个完整function_call（各有原生item/call ID）、唯一response ID及协议usage。实际新插件先正常持久化，再从真实客户端交付提取1090个public call_id；该请求**不额外增加发送数**。后续Responses大包使用这些真实call aliases（可同时带种子previous_response_id），每项function_call_output约800字节。Mock逐项核验实际收到1090个原生call_id、原生previous_response_id和内容，不能用普通native ID扫描或手填Redis/数据库伪alias替代。2181个种子资源（response+1090 item+1090 call）在既有4096预算内。

Embedding使用正常认证/原生发布和新Wasm，响应为OpenAI同步向量schema，1个index0向量及usage.prompt_tokens=17/total_tokens=17，断言input-only原生事实及精确`/v1/embeddings`操作归因；SSE、Responses别名及相应故障均标N/A。

实际请求经过正常认证、额度、原生Router和实际新Wasm。记录请求/调用/尝试ID、Mock接收次数、最早数据与终态、原始usage、日志原因及操作路径、原生账务事实；只按既有价格规则核对零金额。新调用放在本次专属身份，原固定时间窗和旧对象行逐项保持。

Chat/Messages没有Responses别名保存或异步终态Inject调用，不为其制造不存在的alias故障。Redis末尾事实写失败和Inject返回错误目前有单位回归；真实Envoy中未可控触发的分支保持“未验证”。客户断流可验证真实生命周期，但不冒充Inject特定错误码的验证。

## 只对自有alias键触发失败

现有 `resourceref.Save` Lua先GET全部键，发现冲突则返回0且不写任何映射。返回0和OOM均进入同一个“映射未确认”回调，因此可验证补修后的交付分支；这不是业务Redis OOM复现。

Scope固定为本次新建的tenant/principal/sales_asset三元组：`scope_hash = sha256(JSON紧凑数组[tenant_id, principal_id, sales_asset_id])`。

- auto：`modelfusion:public-resources:v1:{scope_hash}:<本次resp_pf_id>`。
- raw：`modelfusion:native-resources:v1:{scope_hash}:sha256("response:" + 本次native_id)`。

Mock发送完整response.created并flush后停在本次nonce屏障。读取本次响应ID，确认目标键不存在，再 `SET <exact_owned_key> <unique_sentinel> NX EX 120`；确认成功后释放Mock屏障发送原正常终态。raw JSON的native ID由Mock预先固定，可在请求前设置同样的自有键冲突。全程不使用MONITOR、OOM、全局断连、全局ACL或Redis配置变更，认证/配额/账务键照常工作。

预期：SSE保留完整终态/usage并记录resource_reference_write_failed，未保存映射不能续接；JSON保持尚未首次交付时的原503门槛，同时保留原supplier usage、observedusage和failed delivery。原生所有权/跨主体约束不变。

auto JSON的public alias由加密随机数生成，首次交付前无法从已有接口取得，不能承诺相同定向注入。若需补充，以独立真实Envoy、同一新Wasm、专属Redis/Mock回放：只对带 `modelfusion-public-resource-write.v1` 标记的EVAL返回未确认，其他命令逐条原样转发一次；不重放非幂等命令。该隔离回放需先固定真实配置/宿主身份与故障中继参数，证据单列，不能计为开发环境完整控制面验收；在方案未完成前保留该格“未验证”。

## finally 与恢复清单

1. 停止新请求、释放全部Mock屏障，确认13次调用及所有保存回调已经结束，保存对象ID、发布revision、call/attempt和审计ID。逐响应收集所有response/item/call/conversation public/native ID与Mock原生ID；种子2181项和普通成功样本的30天映射都列入清理清单。
2. 先通过正常接口撤销本次Key/授权、停用本次发布并确认实际不再接纳新调用，等待在途请求/保存回调全部结束，防止清理后迟到回写；再原生清理发布及Wasm/Bridge引用。
3. 只读取本次两个精确新scope的候选映射。每个键核scope散列、Schema、alias public/original、native/passthrough及完整owner五元组（以本次冻结发布为准），保存已验证原始字节。对sentinel和成功映射分别执行“当前值等于已验证原值才DEL”的CAS；不同值保留并报告。仅允许对本次scope做剩余检查，不扫/删其他scope；稳定回读本次映射及sentinel剩余0。
4. 再删除允许删除的自有路由、绑定、资产和供应商，按正常生命周期停用自有身份。保留不可变调用、账务及审计记录，列明正常接口因历史依赖保留的墓碑，不能硬删业务历史或修改冻结历史发布。
5. 删除本次Mock Deployment/Service/ConfigMap，核对Pod/Service/CM及本次运行引用实际不存在；独立回放容器/网络/临时卷全部清理。
6. 回读原API/FE/Plugin/独立门户工作负载、Secret/PVC、正式配置/价本/授权/钱包和既有制品。原对象逐行/逐资源比较，新自有记录单列；不把正常后台updated_at观测或本次审计新增掩盖成未知业务差异。

只有主审确认冻结源码、完整制品、参数与单写者窗口后才执行上述动作。该文档本身不授权提前创建资源或修改运行配置。
