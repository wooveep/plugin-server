# 上游健康只读消费

本模块消费 Console 已实现的 `upstream-health.v1`，只读取完整快照并逐候选返回资格。权重随机、Key 选择、retry/fallback、失败分类、Outcome 上报、恢复 claim/Consume 和最终发送由各自 owner 接线。当前交付未执行编译、Go/Wasm 测试或真实环境验证，不能据此关闭 WG-I07。

权威 wire 来自 [Redis publisher](../../../../../ocloudmodelfusion/console/src/upstream_health_delivery/redis.rs)、[完整快照及 digest](../../../../../ocloudmodelfusion/console/src/upstream_health_delivery/mod.rs)、[原始 target_decision](../../../../../ocloudmodelfusion/console/src/upstream_health.rs) 和 [健康独立 DDL](../../../../../ocloudmodelfusion/console/src/upstream_health_delivery/schema.sql)。业务范围遵守 [当前恢复设计 REG/MERGE、COOL/STORE](../../../../../ocloudmodelfusion/docs/2026-10-25-simple-upstream-recovery-design.md)、[收敛范围](../../../../../ocloudmodelfusion/docs/2026-10-25-cooldown-media-convergence.md) 和 [WG-I07 原卡](../../../docs/2026-10-25-gap-dispatch-task-cards.md#wg-i07幂等冷却注册健康快照与恢复claim消费)。

## 受控配置与读取

调用方复用现有已初始化 Redis client，传入 `Config{Prefix, AuthorityID, MinimumRevision}`。配置建议落于受控 `upstream_health` 对象：

```json
{
  "prefix": "modelfusion:e2",
  "authority_id": "10000000-0000-0000-0000-000000000001",
  "minimum_revision": "0"
}
```

`AuthorityID` 必须是当前新数据库的实际 authority UUID；示例 UUID 不用于部署。`MinimumRevision` 是 Console 全局健康控制水位，不是 pool revision、credential revision 或 secret generation。前缀/authority 由受控发布配置注入，不能读客户端 header、provider 名称或 URL。Redis endpoint、认证、database 和 timeout 仍归原连接配置，本模块不增加密钥框架。[键及命名约束来源](../../../../../ocloudmodelfusion/console/src/upstream_health_delivery/redis.rs)

`NewReader(RedisReader, Config)` 校验配置；`Lookup(func(ReadResult))` 对以下单个 key 执行一次原子 `HMGET`：

```text
<prefix>:upstream-health:{<authority_id>}:snapshot
authority_id revision state_digest body observed_at valid_until projection_schema complete
```

只读接口仅要求 `Ready` 和 `HMGet`。没有 `EVAL`、写入、过期键操作、后台探测、业务重发或缓存失败兜底。Redis 自身仍可执行既有连接认证；本模块不会调用 `Init`。空 HASH、部分字段、Redis/dispatch 错误、格式/摘要冲突、错 authority、旧 revision、未来观察和控制过期分别返回 typed `ReadStatus`，均不产生可用快照。调用方不能把这些状态改成全健康。[完整原子发布及故障语义来源](../../../../../ocloudmodelfusion/console/src/upstream_health_delivery/redis.rs)

`body` 与 HASH metadata 必须一致；publisher 将 HASH 时间写为固定纳秒、body 使用 chrono serde 时间，因此按 UTC 瞬时比较而非要求字符串相同。`state_digest` 精确对应 `[authority_id, revision, credentials, scopes]` 的 [mf-json.v1](../../../../../ocloudmodelfusion/docs/contracts/2026-10-25-canonical-json-profile.md)，数组顺序不变；仅为该封闭 typed wire 实现规范编码，不导出通用 JSON 算法。控制 revision 保留非负 i64 十进制字符串，不经 float。原 body 保留副本；读取结果不外露可修改状态。[实际 digest 投影](../../../../../ocloudmodelfusion/console/src/upstream_health_delivery/mod.rs)

Reader 只保留已见最大 revision 和对应 digest 作为否决水位，拒绝逆序回调或同版异内容。它不保存一份替代 Console 的状态权威。旧 snapshot 再被使用时仍检查原 `valid_until` 和 reader 已见水位；失败读取不清空水位，也不续期。控制过期不同于长时间没有业务成功：`recovered_at` 的年龄不参与资格，Redis key TTL 不用作健康 TTL。[新 Pod、失败窗口和健康年龄规则](../../../../../ocloudmodelfusion/docs/2026-10-25-simple-upstream-recovery-design.md)

## 发送前的候选决策

```go
reader.Lookup(func(view upstreamhealth.ReadResult) {
    decisions := view.FilterNewCallCandidates(frozenCandidates, time.Now())
    // Parent applies its release policy and routes RecoveryRequired through
    // original claim/Consume authority. No health result is a send grant.
    _ = decisions
})
```

`Candidate.Target` 必须来自 Console 已注册的冻结 `TargetReference`，包含原 target/binding、采购 account、region、upstream model、operation、credential/generation 和确认的 scope IDs。本模块不凭这些自报字段证明 release/pool 身份；发布 owner 必须核原注册及 pool/credential 配置。可选 `ExpectedCredentialControlRevision` 只核已经观察到的同一 credential 控制版本；名称相近的 release credential revision 不得直接填入此字段。[原映射与控制结构](../../../../../ocloudmodelfusion/console/src/upstream_health.rs)

`DecideNewCall` 返回：

- `eligible`：当前完整控制下 credential 启用、原 generation 匹配、scope 映射齐全且没有适用阻断；这不是发送/计费/恢复授权。
- `denied`：缺控制、撤销或错代、误绑定、永久阻断、原绝对期限仍冷却，或调用方带入 `PendingLocalVeto`。本地未 ACK 的真实失败由 outcome owner 保存并设置该否决；配置重载和新快照不会替 owner 清除它。
- `recovery_required`：所有相关阻断均已到期但原行仍存在；携带每个 scope 的原 `recovery_epoch`。必须交 Console 申请并由真实发送权 owner 消费 claim；看到 `active_claim`、租约到期或取得历史 claim 记录均不授予发送权。

以上判断保留当前 `target_decision` 的范围匹配：遍历完整快照里所有适用账户/模型/凭证结构范围，避免新 Key 或旧配置绕过既有账户阻断；quota-group 必须显式 membership。每候选的冷却等待取适用期限最大值并向上取整秒。`FilterNewCallCandidates` 保留输入顺序、返回全部结果，不随机、不修改权重或偷偷换目标；全冷却的新调用由父执行器按原规则快速 503。[原函数及 scope_applies](../../../../../ocloudmodelfusion/console/src/upstream_health.rs)、[全冷却 STORE05](../../../../../ocloudmodelfusion/docs/2026-10-25-simple-upstream-recovery-design.md)

结果携带实际 authority/global revision、credential generation/control revision、适用 scope revision/failure_generation/recovery_epoch。它们是下一步 claim/outcome 的观察输入，不是新建的历史 H_generation 算法。[已移出的历史门槛](../../../../../ocloudmodelfusion/docs/2026-10-25-cooldown-media-convergence.md)

已受理 operation 单独走 `DecideAcceptedOperation(originalOperationID)`，只返回 `operation_authority_required`，没有 `eligible` 分支。原任务查询/Consume 必须由既有操作权威校验原账户、授权、状态和单次发送；生成冷却不能据此取消查询，恢复占位也不能授予原媒体重建。此返回值本身同样不是轮询许可。[STORE06–08 与原任务规则](../../../../../ocloudmodelfusion/docs/2026-10-25-simple-upstream-recovery-design.md)

## 当前 wire 能力边界与验证

当前 snapshot 没有 pool ref/revision、release/runtime-registration 证明或 claim owner/有效期。因此 pool/发布修订核对、登记和服务通道授权、Outcome pending 事件保存与精确 ACK、恢复 claim/Consume、双 Pod 唯一恢复发送、冷却状态变更、retry/fallback 仍由对应 owner 补齐；本模块不以配置字符串或 `active_claim` 伪造这些能力。[真实 snapshot 字段](../../../../../ocloudmodelfusion/console/src/upstream_health_delivery/mod.rs)、[登记/claim DDL](../../../../../ocloudmodelfusion/console/src/upstream_health_delivery/schema.sql)

测试源码包括真实 Wasm SDK Redis callback 的单 HASH 读取、无写/无 provider send；native 正负例覆盖缺失/错代/过期/逆序/同版冲突、撤销、结构账户阻断、新 Key、quota membership、冷却到期仍需恢复、独立 operation 界面和不可变观察。固定向量复用 [Console delivery 测试](../../../../../ocloudmodelfusion/console/tests/upstream_health_delivery.rs) 的原凭据/撤销字段及合成 `9007199254740993` 水位；规范字节与 SHA 独立以 Python hashlib 计算。它是可复现的跨语言静态向量，不是 Console/Go 编译通过或运行 Redis 投递回执。父批次仍需运行双方测试、真实 publisher→Redis→Wasm 及多实例故障验证，绑定最终源码与环境再回填。
