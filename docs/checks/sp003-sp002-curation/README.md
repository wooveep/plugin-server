# SP003 插件基于 SP002 的筛选结果

分支：`codex/sp003-plugins-sp002`。基线：`V2.2.1-SP002`（`a2f1ada99be5ee86e0eca2db5df9608dc281fb03`）。逐提交来源、保留路径和版本清单见 [receipt.json](receipt.json)。

保留插件服务制品管理、原有模型协议和账单修复、原请求 ID 及脱敏 Redis 诊断。key-auth、RPM、TPM、ai-quota 的业务代码恢复 SP002；ai-load-balancer、model-mapper、ai-statistics 保留 SP002 的路由职责。撤回 #100、#105、#92、#129 对应三插件解耦、亲和路由、热态鉴权/限流和托管 key-auth 接管。没有新增双 Redis 读写链路。

共享 Go 框架会改变构建输入，46 个 Go 实现均递增到高于已准备 SP003 的版本，避免复用不同字节的同一版本；8 个 C++ 正式制品复用 SP002 原归档和制品 ID；Rust ai-data-masking 重建字节与已登记 2.0.6 不同，独立使用 2.0.7。正式清单仍为 55 个插件。7 项关键插件生产源码相对 SP002 仅有版本声明差异，见 [scope-audit.json](scope-audit.json)。关键版本如下。

| 插件 | 本分支版本 |
| --- | --- |
| ai-load-balancer | 2.0.11 |
| ai-proxy | 2.0.86 |
| ai-quota | 2.0.17 |
| ai-statistics | 2.0.18 |
| ai-token-ratelimit | 2.0.16 |
| cluster-key-rate-limit | 2.0.13 |
| model-mapper | 2.0.13 |
| key-auth | 1.2.15 |

验证：插件服务 Python 测试 132 项通过；上述 9 个 Go 模块与共享框架测试通过。首次验证发现的 quota 额外 HTTP 调用和旧 LB 测试替身不完整问题已修正并复测。2026-11-17 按用户要求合入 `key-auth` `1.2.15` 自动 TLS 证书签发（ACME HTTP-01 `/.well-known/acme-challenge/<token>` `GET`/`HEAD` 校验放行及伪造身份请求头清理）优化，保持静态 SP002 鉴权模型，未引入 #129 热态 Redis 鉴权。

已从固定源码重新编译全部 55 项正式插件，完成完整包校验、登记、镜像构建与推送，并在开发环境 Helm revision 3 部署。两条既有已发布路由通过 SP002 原生发布流程采纳新插件，修订分别为 14 和 6；其余草稿保持原状态。

保留 Responses 终态流读取修复、ai-proxy 200 MiB 和 ai-billing 128 MiB 内存阈值重建。ai-billing 事件每秒重试，后端只对满足消费组确认边界的消息执行 7 天回收。保留 #76 调用观测及 #122 第一个 wrapper 的有界原始 UA/XFF/请求 ID、Router 前当前 ID 快照和 JSON/TEXT 两类日志字段；model-router 2.0.10 已在现网采纳。XFF 仅保留原声明值，不据此推断真实客户端 IP。

运行镜像：`registry.lc.wooveep.com:9980/modelfusion/plugin-server:V2.2.1-SP003-sp002-20261117-rc1@sha256:556ec4f505951494021aae3a0f58e37a35921f883b27c4f4491955a6c8f74e03`。55 项清单摘要 `sha256:a2e0d9a351f8329d20ab26a5ee66926fdac993fcf825dd823bfb5e601ccaea09`。未生成新的完整离线交付包，未执行付费供应商调用或把单元测试当作完整业务验收。
