# 具体 Key 路由授权与生产调用期限候选

2026-10-31。行为提交 `716455b2`、`70b0ee74`，quota最终文档/来源提交 `ce05e989`，其余包按最后触及自身输入的70提交保留来源。本报告记录待集成候选，未入现用制品库、未设为默认、未部署。不能从本批通过推导整个业务链已去耦或 E1 已通过。

quota 在现有一次 Redis 只读脚本中检查具体 Key 的 route grant，并使用稳定账户地址和当前 control/account/mapping 同代校验。旧 Lua 在真实 Redis 上对未授权 Key/其他路由/缺失或损坏授权列表共5例错误放行；候选已转绿。配套 Console 源码为 `c84f7219`，其同事务 revision/outbox 和独立钱包恢复见 MF 的原生候选报告。

普通生产在 Redis 准入后只保留原 call ID 和业务时间，不再签发临时 Console 五秒窗口。明确带 bounded permit 的验证继续检查原到期时间，internal 发送仍须先建立其真实有限许可。Proxy 正文、准备、发送及重试不把未携带临时期限的 production 判作过期；仍受已有上传/上游/重试预算约束。没有续约或延长期限路径。

原合法 headers/body host 回调反例等待6秒后，从403/0次发送转为1次上游 callout且无本地拒绝；另一个聊天在固定70提交上用同一断言独立转绿。共用执行身份包也通过。此处是实际 Wasm SDK host callback，不是在线 Envoy/外部上游证据。

本批5个实际消费者统一固定工具链构建、准备完整包，并用各自实际编译文件显式设置 WASM_FILE_PATH，整包 Go 与支持的 Wasm 宿主用例通过：quota 2.0.6约24秒、proxy 2.0.12约420秒、billing 1.0.2约69秒、cache 2.0.5约70秒、memory 0.1.8约39秒。quota 实际 Redis 环境已配置，未以跳过代替通过。各测试使用各自规定的 Go/宿主边界，不声称每个 Go 用例都在 Wasm 模式重复。源码、工具链、Wasm/完整包/日志摘要见[候选清单](production-admission-20261031/candidates.json)。

准备过程曾因 Go 测试自动生成的 wasm-unit-test.wasm 落入源码目录而被既有来源检查拒绝；已将本次生成物移到 build/test-cache，并显式指定正式编译产物运行宿主测试。未放宽完整包的来源/不可变校验。新包仍只在准备边界；后继接纳只做原上传边界的完整验证，不给列表、选版本或发布追加重复验包。

健康429、待确认outcome、本地veto和半开Claim/Consume、媒体专用派发等 Console 耦合仍开放；本批仅证明上述授权/期限及相关消费者。继续按原scope/epoch/撤销及实际发送/成本规则收敛，不能用健康普通目标通过掩盖特殊路径。

## 容器构建的宿主依赖补正

后继 `a58f15ce` 将默认固定 Go 容器使用的两项缓存归入现有 build/cache，不再先执行宿主 go env；--host 仍使用明确选择的宿主工具链。在只提供 Git/Docker 的 PATH 下，实际固定容器重编 quota2.0.6 成功，输出 Wasm 与上列候选逐字节一致。见[执行摘要](production-admission-20261031/container-no-host-go.json)。测试前后的原候选 Wasm/receipt 已成对保留并恢复；该结果不是新包接纳。构建脚本属于来源输入，最终集成仍须按后继固定源码重新生成 receipt/完整包，不能沿用旧 receipt 冒充新来源。
