# Go 插件文档入口

维护源码位于 `plugin-server/plugins/wasm-go/`。日常编译、版本选择、完整包校验与 HTTP 分发统一从[插件服务入口](../../../README.md)执行。

- 扩展插件源码与使用说明：[extensions/](../extensions/)。
- 插件规范与默认配置：[resources/plugins/](../resources/plugins/)。
- 发行集合和每个插件的精确版本：[catalog.json](../../release/catalog.json)。
- 跨项目发布准备：[2.2.1 发布计划](../../../../docs/plans/2026-11-05-release-2.2.1.md)。

历史验证文件按各自当时的源码与环境解释；当前开发使用普通 Markdown、dev 分支和适用的代码／运行验证。编译或离线测试通过不代表实际 Gateway、供应商请求和资金结算已经验收。
