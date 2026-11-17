# SP003 插件基于 SP002 的筛选结果

分支：`codex/sp003-plugins-sp002`。基线：`V2.2.1-SP002`（`06518896c1aaa463fed3f23d4ff378784c422346`）。逐提交来源、保留路径和版本清单见 [receipt.json](receipt.json)。

保留插件服务制品管理、原有模型协议和账单修复、原请求 ID 及脱敏 Redis 诊断。key-auth、RPM、TPM、ai-quota 的业务代码恢复 SP002；ai-load-balancer、model-mapper、ai-statistics 保留 SP002 的路由职责。撤回 #100、#105、#92、#129 对应三插件解耦、亲和路由、热态鉴权/限流和托管 key-auth 接管。没有新增双 Redis 读写链路。

共享 Go 框架会改变构建输入，46 个 Go 实现均递增到高于已准备 SP003 的版本，避免复用不同字节的同一版本；8 个 C++、1 个 Rust 正式制品不因该框架重复提版。正式清单仍为 55 个插件。7 项关键插件生产源码相对 SP002 仅有版本声明差异，见 [scope-audit.json](scope-audit.json)。关键版本如下。

| 插件 | 本分支版本 |
| --- | --- |
| ai-load-balancer | 2.0.11 |
| ai-proxy | 2.0.86 |
| ai-quota | 2.0.17 |
| ai-statistics | 2.0.18 |
| ai-token-ratelimit | 2.0.16 |
| cluster-key-rate-limit | 2.0.13 |
| model-mapper | 2.0.13 |
| key-auth | 1.2.13 |

验证：插件服务 Python 测试 132 项通过；上述 9 个 Go 模块与共享框架测试通过。首次验证发现的 quota 额外 HTTP 调用和旧 LB 测试替身不完整问题已修正并复测。

本分支只交付源码和元数据，尚未生成或发布新的 Wasm、完整插件包、镜像或离线包。交付时应由本分支重新构建，并向 Console 提供同次构建的正式插件目录；原 SP003 包不代表本次筛选结果。开发环境部署由用户负责。
