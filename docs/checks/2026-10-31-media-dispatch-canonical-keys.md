# 媒体票据的规范余额键与原等待

Helm124的原生Qwen2已生成并发布票据，截止后保留unknown、没有物理供应商请求或资金应用。只读重放捕获的真实Rust载荷发现，它在Go构造器的传输契约校验已被拒绝；不能把该任务直接归因为Redis消费。进一步核对确认，媒体网关还自行拼接了无花括号的余额前缀，并把Key映射放到scope下，和IF05原钱包/account/principal层级不一致。原测试直接模拟allow，未核传入的真实键。[规范键红例](health-media-runtime-20261031/ai-proxy-2.0.16/canonical-keys-red-command.json)已在原源码上失败。

网关改为规范的scope花括号、原account及其具体Key映射。实际Lua在projection_unready时确认票据仍prepared、没有写消费事件；这一明确未消费的分支保留原executor和票据，通过已有Root Tick重读资格。其余拒绝、未知或已消费结果保留原行为。原Key、原attempt、invocation和截止时间都不替换。

RESP边界统一返回已有授权错误或明确projection_unready，不在调度层反复解析原始回复。createTaken仅对真正的消费响应及未知状态封闭，明确未消费的投影等待可续接；首次allow后仍只许一次发送。原宿主用例新增用户/Key键断言及投影恢复/原截止到期两案，原生Redis用规范IF05键验证Lua；[分层命令](health-media-runtime-20261031/ai-proxy-2.0.16/media-boundary-commands.json)全部通过。

本批发布版本2.0.16，保留15完整包。尚待统一实际Wasm批次、完整包入库/部署、六媒体与资金链；没有以宿主夹具代替实际供应商发送。

## 原生传输契约

Rust的GatewayExecutionPayload保留完整原供应商endpoint，含API路径；Go却只接受根URL。请求实际路径本来就由已注册profile独立选择，因此移除错误的根路径限制，原endpoint原样保留。另一个源头问题是Release Source为HTTP目标写入TLS SNI；MF的配置生成与私有归档校验改为只在HTTPS使用SNI。SDK原生集群快照本来已按此规则工作。

[十二个协议/传输红例](health-media-runtime-20261031/ai-proxy-2.0.16/media-endpoint-contract-red-command.json)在修复前全部失败；修复后媒体库与实际Redis批次通过，见[命令回执](health-media-runtime-20261031/ai-proxy-2.0.16/media-transport-boundary-green-command.json)。[真实归档只读重放](health-media-runtime-20261031/ai-proxy-2.0.16/native-archive-transport-replay.json)仅按修正后的Source规则将HTTP SNI置为空字符串并重算descriptor digest，保留原完整endpoint和业务身份，构造器通过；没有消费票据或重新发送原任务。它仍是unknown，不能把无账务应用视为已确认零成本。

传输修复前的完整Wasm批次曾通过323顶层/2203含子用例检查，但其制品未发布。当前源代码已变化，最终交付以重建制品与新统一批次为准。

传输修复后的最终候选Wasm `33b2b158a101540bd1c7bdb2b6d1df60837ef708d7f6a34a17bfcb5e2a692bd5` 已通过[完整Go/Wasm批次](health-media-runtime-20261031/ai-proxy-2.0.16/full-go-wasm-command.json)：323顶层、2203含子用例，进程退出0。原始日志中的一个PASS与无换行夹具输出相连，按Go PASS记录统计，未重复执行测试。新旧Schema仅版本/来源元数据变化，route Schema字节一致，见[比较回执](health-media-runtime-20261031/ai-proxy-2.0.16/schema-comparison.json)。
