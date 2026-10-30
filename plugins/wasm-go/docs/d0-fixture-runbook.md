# WASM D0 离线运行与交接

2026-10-25。范围：D0-WG-01/02/03/04 的批次 A；权威范围与共享文件责任见 [任务卡](2026-10-25-p0-d0-task-cards.md)。固定输入为 common `mf-common-2026-10-25.3` 与 `mf-execution-2026-10-25.1`，精确摘要见 [消费者说明](../testdata/contracts/README.md)。仅交付测试资产，没有业务或部署改动。

## 运行

预先准备 Go ≥1.24.1 与 [固定 Python hash lock](../tools/contractcheck/python/scripts/contracts/requirements.lock) 对应的环境。依赖 bootstrap 与离线验收分开；脚本不安装、不联网。已有环境可通过 `D0_PYTHON` 显式选择，缺失或版本不符报告 blocked。

```bash
D0_PYTHON=/tmp/modelfusion-d0-venv/bin/python GOWORK=off GOPROXY=off \
  bash scripts/verify-d0-contracts.sh --batch A --output /tmp/wasm-d0-evidence
```

驱动运行 559 个公共正负契约案例、固定消费者测试、39 个身份 / matcher / callback 案例、既有 matcher 回归及 provider 回归。测试集为空、必需测试名漏选、错 lock、丢夹具、错 CalloutID 或任一步非零均不能被后续命令覆盖。`driver_test.go` 经真实命令验证这四类故障。`--run '^DoesNotExist$'` 可重现零选集拒绝；`--lock` 与 `--cases-root` 支持显式故障输入。B/C 未安装时 `E_SCOPE_BLOCKED`，不成功退出零例。

输出 `summary.json` 包含提交 / 工作区状态、lock / 语料 / 工具的摘要、实际命令 / cwd / 退出码、案例数、断言与未覆盖项。`public-contracts.json` 保存权威验证器逐例结果；`fixture-tests.log` 保存真实 CalloutID 绑定与明确事件次序；`imported-files.json` 准确列出全部 629 个导入文件及实际摘要并被 summary 绑定。两个同输入运行只比较 `assertions` 与案例数，run ID、时间、路径和耗时允许不同。

## provider 既有模块路径适配

原卡命令 `(cd extensions/ai-proxy && GOWORK=off GOPROXY=off go test ./provider -count=1)` 被已有绝对 SDK replace `/home/cloudyi/CodeWorkspace/higress-plugin/proxy-wasm-go-sdk` 阻塞。驱动保留其真实非零退出和 `blocked_existing_module_path`，不把该命令记录为 passed。

默认驱动随后在显式输出目录生成临时 `provider-offline.mod/.sum`：只移除失效的 SDK replace，保持 SDK `v0.0.0-20251103120604-77e9cce339d2`；SDK 两条校验和必须等于本仓根 go.sum 的固定值；将 wasm-go 的相对 replace 解析到本仓实际根。provider 的原始模块文件始终只读。输出保存原始 / 生成文件摘要、准确 patch、SDK 校验和与实际 `go test -mod=readonly -modfile ...` 命令。`GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOWORK=off` 禁止隐式下载；缺缓存阻塞。此适配仅用于验证，不修改业务依赖。

## CodeGraph 与边界

实施前 CodeGraph 核对 NewTestHost、CallOnHttpCall、attributes、matcher ParseRuleConfig / GetMatchConfig、header/body 缓存与旧 retry mock。现有 NewTestHost 有 263 callers，故保持 host/matcher/wrapper/retry 源码只读。新增接缝未进入现有图；GitNexus 对 NewHostAdapter 返回 target not found / UNKNOWN，没有伪称影响分析零风险。提交前由集成 owner 对准确暂存范围运行 GitNexus detect-changes。

D0 实际层级为 `contract_fixture` 与 `host_emulator`，`evidence_level=offline_fixture`、`run_kind=offline_fixture`。provider 回归只说明旧测试未被破坏。真实可信 writer / selection / Consume / guard、Envoy 重入、生产 retry、真实 EB 和 supported / 发布均未验收。批次 C / IF09 未安装，正式 FX 证据封装为 not_run，不复制协议。

## 已执行验收

非隔离完整驱动回执 `/tmp/wasm-d0-evidence-followon-second/summary.json`：公共 559 / 身份 13 / matcher 11 / callback 15；固定消费者 pass 事件 13，fixture pass 事件 54，provider pass 事件 631。原 provider 命令 blocked，临时固定 modfile 回归 passed。整数跨 resource 红绿回归证明 10 与 9007199254740993 接受，10.5 拒绝；固定 core 字节未改。

独立 checkout 最终验收位于 `/tmp/wasm-d0-isolation-rp1l5e2m/`：从本仓 HEAD `1102a601c461113bc7cb2c8792bed941393674e2` 独立 clone，仅覆盖上述 D0 白名单；新 user/mount/net namespace 将 `/home/cloudyi/CodeWorkspace` 及 `/tmp/d0-follow-on-input` 挂为独立 tmpfs，检查源仓和导出包均不可见、网络接口仅 `lo`。该环境的两次完整驱动均退出 0，559 公共例 / 39 本地例及全部业务断言完全相同。随后 `go test ./internal/executionfixture -count=1` 完整通过，含错 lock、丢 fixture、错 CalloutID、零选集四个真实驱动故障守卫；无网络安装或生产模块改动。

| 回执 | SHA-256 |
| --- | --- |
| `isolation-environment.json` | `b4f1bdd81b781f0fef4c3fa6ab7a4d22b9391b761951f9766a2dc86b8cf88a75` |
| `run-1/summary.json` | `93cc7c48769056f130cdac233031670b1f007a1bda24c6bfcbda40a7e107d3a9` |
| `run-2/summary.json` | `f8714e140d2e3f5893c3926ac77d46b4b811d4ceb5c82c77e47750b996007d9d` |
| `determinism.json` | `e3bdf6e625fd70ad382daa33fda3d496d94245a23b2d07bcc8a19e2c43aeaaab` |
| `full-fixture-tests.log` | `0e241d63fe63583b9b96895a4aee27b765c069308efc924c6a27e0820e43ea18` |

验收脚本保存在同一临时目录的 `run-isolated.sh`，环境和导入文件的细节由运行回执绑定；临时目录仅供复核，版本化的输入、测试与运行命令可重新生成回执。仓库既有 graphify 约定的 AST 更新亦已执行（无需 LLM）；原 CodeGraph / GitNexus 索引未升级。

## 复审后的最终验收

WG04 的 `--run` 限定顶层测试名，包含 `/` 的子例筛选以 `E_RUN_SCOPE` 拒绝。驱动还按冻结 catalog 的稳定 ID 逐个核对11个 matcher与15个 callback 的实际 Go JSON 子例 pass 事件；逐例 `proof_test` 指向真实子例，不能用顶层 pass 推定全部39例执行。

最终复验目录为 `/tmp/wasm-d0-review-isolation-679tdh42/`。新 user/mount/net namespace 隐藏原工作区及导出快照，网络仅 `lo`；两次完整驱动均559个公共案例＋39个本地案例匹配，逐例断言一致。完整 fixture suite 包含六类驱动故障守卫通过，其中新回归覆盖子例筛选，以及仅删除一个子例 pass 事件、保留顶层 pass 和命令 exit0 的真实 wrapper 故障（必须 `E_NO_CHECKS`）。此前隔离记录保留作实施过程追溯，本节为最终结果。

| 最终回执 | SHA-256 |
| --- | --- |
| `run-1/summary.json` | `4a40617a4d31d6bc2f1f679927757814b07865f94296e93f2dfba177a9e4df80` |
| `run-2/summary.json` | `83b7d72815c2c504e8a283f2bff0d26182895e5d2b82a8de053f3bc5c4fafdfb` |
| `full-fixture-tests.log` | `1ee825a7a235608417eec6ae77c818803ad8b8e934140153d69b18a2c31cb9ac` |

修复后脚本 SHA-256 为 `86d1f2f218a513be54b1c7e6426a0cdec5c11d8fae727e508c4066d1bf23cdcf`，driver test 为 `12674f4b149a945ea3350594792dbb90e356ecb3d46b1e9b24130711051f41dc`。上述回执不改变原 provider 直接命令的失效路径结论；临时固定 modfile 的回归与原命令阻塞仍分别记录。
