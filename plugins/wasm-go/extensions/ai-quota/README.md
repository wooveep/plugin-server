# AI Quota：版本化余额只读准入

版本 `2.0.6`，协议 `balance.v1`。仅给受管的新模型调用入口绑定本插件。PostgreSQL原钱包是资金权威；Console是余额/资格投影和维护状态的唯一写者。ai-quota不计价、不预扣、不结算、不写余额或ready；响应正文、缓存命中、错误和迟到用量均不触发第二次配额判断。

## 配置作用域

每份独立生效的余额配置必须有 `balance_scope_id`、`cache_generation`、`projection_schema: balance.v1` 和完整 `redis`。字段以 [资源 schema](../../resources/plugins/ai-quota/spec.yaml) 为准；两个 UUID 必须来自 Console 本次受控发布，不能使用示例值代表真实生效配置。`missing_balance_policy` 只接受 `deny`。旧用户名余额 key、`balance_key_template`、`tenant_header`、`consumer_header`、`quota_scope` 和 `provider` 配置已退役；旧裸整数 GET 不会作为新协议后备。

支持两种配置形态：

- **有效全局配置与继承规则：** 全局提供上述余额配置；规则继承 Redis、scope、generation、schema 和 missing policy，不能覆盖这些字段。规则可调整普通路径作用域及交集过滤条件；托管规则仍须完整提供自己的 `execution_admission`。
- **仅路由完整规则：** `defaultConfigDisable: true` 且控制器下发的根只有 `_rules_` 时，基础配置休眠，未匹配流量不执行准入。每条匹配规则各自提供完整 `redis`、余额命名空间和托管字段，按独立配置校验及初始化。没有有效全局连接时，规则不能仅靠局部字段假定继承。非空且无效的全局配置也不能由规则补救。

托管发布使用第二种形态；Console/SDK 将完整配置写入每个目标规则，不能假设被禁用的 `defaultConfig` 仍会传到插件。Redis 的 `service_name` 必填；认证信息来自受控 Secret，文档不包含真实凭据。

## 可信身份与余额读取

受管链必须先执行支持原生 `credential_bindings` 的key-auth（本批版本1.1.0），它按实际成功认证的credential安装原PG `account_id`、`api_key_id`和`credential_generation`，先清除客户端同名字段。不得把任意客户端钱包header直接当可信身份，也不得把旧consumer别名替代具体Key。没有原生绑定、重复或非法身份字段均拒绝。Console/CUT/SDK生成并观察该配置后才可发布真实组合；仅升级二进制不会补出原账户绑定。

单个Redis脚本同次读取同slot的control、account和具体Key mapping。必须scope/generation/schema一致、三者ready、账户与Key映射的授权版本和凭据代际一致且Key enabled。金额、account_version和授权版本均按规范整数字符串精确读取，支持超过2^53与有符号i64边界。只有严格正余额允许；零余额和负余额拒绝。缺失、旧代、blocked/building、旧资格、格式/单位错误分别保留原因；Redis故障或不明结果503，不继续请求。

## 逐调用托管准入

`execution_admission` 使用 `schema_version: managed-execution-admission.v1`。认证成功且 Redis 余额/资格检查允许后，production 在网关本地生成调用 UUID 和原业务时间，不签发临时发送许可，直接继续到上游处理。它不向 Console 查询，不预扣资金，不把客户端的 call/attempt/准入头当作权限。ai-proxy 通过既有事件 Stream 登记同一调用身份、原始时间和实际尝试，后台据此结算、记账并更新 Redis。后台停止时允许事件积压、余额更新延迟和随后结算出现负余额；Redis 已知的零或负余额仍拒绝新请求。

规则内保留冻结的 `release_id`、`route_id`、`purpose`、`combination_key` 和 `profile_id`。production 的 `access_id`、`revision` 缺省或为 null，不需要 `console_internal`。旧完整配置保留该字段时仍校验其结构，但 production 不调用它。

仅 validation 使用完整 `console_internal` 连接对象和原许可 `access_id/revision`，向 `POST /internal/execution-admission` 查询受控许可，核对完整回显、原业务时间和有效期。该测试权限不能用于 production。依赖失败或不完整响应保持拒绝；重复及迟到回调不创建第二个调用。

## 路由范围与维护

托管配置必须使用 `path_scope: bound_target`，`enable_path_suffixes` 与 `enable_methods` 省略或为空数组；非空手工过滤器会使 `execution_admission` 配置无效。范围随绑定路由覆盖其全部方法，无需手填 URL 白名单。方法属于新调用时就执行准入；这不表示下游支持任意方法或操作，E1 实际文本执行仍校验自己的 POST JSON 边界。已受理任务查询、内部事实和结算使用各自入口，不绑定为新的模型销售调用。

无托管准入对象的兼容配置保留 `path_scope: legacy_suffixes` 默认后缀 `/v1/chat/completions`、`/v1/messages`；普通 `bound_target` 下的非空 suffix/method 仅作为交集。该兼容能力不能代替托管发布的逐调用许可。

所有三类key使用 `mf:balance:{scope}:...` 同一hash slot。普通资金通知不能为缺失记录开户或解除冲突。维护时拒绝新调用；恢复开放由 Console 的原生钱包维护、扫描和 Redis 读回核验控制；路由配置与业务验收独立进行，插件没有维护或ready写入口。

本批源码与原生/模拟宿主结果、Wasm构建、实际K3S加载必须分别记录。参见[实施计划](../../docs/plans/2026-10-27-versioned-balance-admission.md)；未取得实际联合证据前，不把本地Redis或模拟宿主称为生产链通过。

生产托管路由还须在同一次 Redis 原子读中核对当前 Key 的 `authorized_routes`。空列表返回 `route_authorization_missing`，缺失或损坏的列表返回 `projection_unready`；不会以同用户另一把 Key 的权限代替。授权变更与原 identity revision/outbox 同事务提交，映射刷新不写资金。账户 key 使用稳定的 `mf:balance:{scope}:account:<id>`，当前代际由 control/account/mapping 一致性校验；缓存恢复无需重发路由，旧代写者仍被拒绝。配置中的 `cache_generation` 保留冻结来源标识，不作为永久的运行地址或就绪心跳。

普通生产调用的原始身份与业务时间由前置 Redis 准入产生，不使用临时 Console 许可的5秒期限；上传、发送和重试遵守各自既有预算。带明确到期时间的 validation/internal 许可继续按原期限校验，不创建续约或延长许可路径。
