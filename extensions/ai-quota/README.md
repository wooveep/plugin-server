# AI Quota：版本化余额只读准入

版本 `2.0.0`，协议 `balance.v1`。仅给受管的新模型调用入口绑定本插件。PostgreSQL原钱包是资金权威；Console是余额/资格投影和维护状态的唯一写者。ai-quota不计价、不预扣、不结算、不写余额或ready；响应正文、缓存命中、错误和迟到用量均不触发第二次配额判断。

配置全局必填 `balance_scope_id`、`cache_generation`、`projection_schema: balance.v1` 和 `redis`。两个UUID必须来自Console本次受控发布，不能使用示例值代表真实生效配置。`missing_balance_policy`只接受 `deny`。旧用户名余额key、`balance_key_template`、`tenant_header`、`consumer_header`、`quota_scope`和`provider`配置已退役；旧裸整数GET不会作为新协议后备。

受管链必须先执行支持原生 `credential_bindings` 的key-auth（本批版本1.1.0），它按实际成功认证的credential安装原PG `account_id`、`api_key_id`和`credential_generation`，先清除客户端同名字段。不得把任意客户端钱包header直接当可信身份，也不得把旧consumer别名替代具体Key。没有原生绑定、重复或非法身份字段均拒绝。Console/CUT/SDK生成并观察该配置后才可发布真实组合；仅升级二进制不会补出原账户绑定。

单个Redis脚本同次读取同slot的control、account和具体Key mapping。必须scope/generation/schema一致、三者ready、账户与Key映射的授权版本和凭据代际一致且Key enabled。金额、account_version和授权版本均按规范整数字符串精确读取，支持超过2^53与有符号i64边界。只有严格正余额允许；零余额和负余额拒绝。缺失、旧代、blocked/building、旧资格、格式/单位错误分别保留原因；Redis故障或不明结果503，不继续请求。

`path_scope: legacy_suffixes`默认只匹配 `/v1/chat/completions` 和 `/v1/messages`；`bound_target`用于已经绑定为新调用的执行入口。`enable_path_suffixes`与`enable_methods`是交集过滤器，不改变账户、代际或Redis连接，规则不能覆盖这些全局值。已受理任务查询通过路由绑定或方法过滤排除；GET/HEAD若被分类为新调用仍执行准入；内部事实、结算与已受理任务查询入口不绑定本插件。模型创建入口应显式限定相应方法，例如POST，不能拿任意任务管理路由当新销售入口。

所有三类key使用 `mf:balance:{scope}:...` 同一hash slot。普通资金通知不能为缺失记录开户或解除冲突。维护时拒绝新调用；恢复开放由Console/REL/DEP完成真实配置和受控请求观察，插件没有维护或ready写入口。

本批源码与原生/模拟宿主结果、Wasm构建、实际K3S加载必须分别记录。参见[实施计划](../../docs/plans/2026-10-27-versioned-balance-admission.md)；未取得实际联合证据前，不把本地Redis或模拟宿主称为生产链通过。
