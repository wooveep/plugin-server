# Higress 上游基线与独立插件维护边界

登记日期：2026-10-22。用户已确定 Higress 基础跟随上游，Modelfusion 插件在本项目维护。跨项目执行与验收见[工作区计划](../../../docs/plans/2026-10-22-higress-upstream-baseline.md)。

## 现行来源

- Go 插件权威源码是本仓库 `extensions/`、`pkg/` 及对应资源 schema；[当前插件契约](README.md#当前插件契约)记录行为与职责。
- [Makefile](../Makefile)从 `extensions/` 发现模块，输出 `build/plugins/<name>/<VERSION>/plugin.wasm` 与 metadata。
- [plugin-server](../../../plugin-server/Makefile)同步本仓库 `build/` 中的制品；默认目录依据两个仓库的相邻位置解析，仍支持显式 `PLUGIN_ARTIFACTS_DIR`。
- [部署离线包构建脚本](../../../modelfusion-deploy/offline/build-offline-bundle.sh)显式传入相邻 `higress-plugin/wasm-go/build`，环境 values、插件 URL 和运行验证在部署仓库维护。
- Higress 官方仓库的 `plugins/` 用于参考上游实现；Modelfusion 构建和分发不以该目录为源码入口。

## 历史副本处理

清理前 Higress 提交为 `703d35256ffbd5d06d9469610ce334be14bf9101`。其相对官方 `60383934` 的 Go 插件差异涉及 35 个文件；改名后的 `extensions/key-auth` 计入对应路径后，24 个文件在本仓库存在对应路径，但内容均已不同。因此本次保留现行实现，未用旧副本覆盖。

| 旧 Higress 副本 | 现行维护位置 / 处理 |
| --- | --- |
| `examples/key-auth` | 本仓库 `extensions/key-auth`；以现行鉴权契约为准 |
| Qwen 媒体转换 | 本仓库 `extensions/ai-proxy/provider/qwen.go` 及测试；真实 provider 验收另行登记 |
| 新增 `volcengine.go` | 当前已有 `provider/doubao.go`，覆盖 Ark 域名及 chat、embeddings、images、responses；旧副本的额外私有请求头不宣称已迁移 |
| quota / statistics / token-ratelimit 中的重复 usage 和金额计算 | 当前职责以 `ai-quota` 的只读余额准入、`ai-statistics` 的可观测事实、`ai-billing` 的请求事件及平台结算为准；旧金额扣算逻辑不重新加入 |
| 旧 Rust 脱敏、阻断审计扩展 | 位于 Higress 历史提交；独立 `../wasm-rust` 有不同实现，本轮未覆盖，也未将其视为当前 Go 制品交付的一部分 |
| C++ OAuth、Go 原生 `.so` | 非本仓库当前 Go/Wasm 构建链，保留在历史提交与备份中 |

这些是副本去向与边界核对，不能据此宣称旧副本的全部能力与现行插件等价。确需恢复某项历史能力时，在所属独立插件项目按当前契约移植并补验证。

## 恢复入口与验证边界

完整历史由 Higress 原提交保留，另有可独立恢复的 Git bundle；备份路径、SHA256、逐文件清单及运行证据见[部署核对记录](../../../modelfusion-deploy/docs/checks/2026-10-22-higress-upstream-baseline.md)。例如从 Higress 仓库查阅旧文件：

```bash
git show 703d3525:plugins/wasm-go/extensions/ai-proxy/provider/volcengine.go
```

本轮只更改本仓库的维护说明，插件代码、配置契约、版本和已构建 Wasm 均保持现状。旧绝对 `go.mod replace` 路径、插件服务运行镜像与现行源码的制品对应关系仍由[后续构建与分发计划](../../../docs/plans/2026-10-22-http-plugin-upstream-adoption.md)处理；本次社区镜像的 McpBridge 验证不作为插件计费或真实模型调用验收。
