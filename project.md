# Plugin Server 项目说明

## 定位

`plugin-server` 是 Wasm 插件分发与插件元数据服务，负责为 Higress / Console 提供可访问的插件下载地址与版本产物。

## 职责边界

- 提供插件二进制或静态资源的 HTTP 分发入口
- 为 Console / Higress 提供可配置的插件 URL 模式
- 配合发布流程输出可被运行时消费的插件版本地址

## 与其他子项目关系

- 与 `higress`：Higress 运行时通过插件地址拉取 Wasm 产物
- 与 `aigateway-console`：Console 负责配置插件地址模式与管理插件元数据
- 与根目录发布链路：正式版本镜像与地址口径需服从根目录 `TASK/release/` 与 `docs/release/1.0.0/`
