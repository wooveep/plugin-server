# Plugin Server TODO

> 本文件只记录 Plugin Server 自身轻量待办。跨子项目专项材料统一索引到根目录 `TASK/projects/plugin-server/`。

## 已完成

- [x] 增加最小 Python 单测，覆盖插件元数据与 properties 解析。
- [x] 纳入仓库级 `./start.sh test --stage unit|all` 测试闸门。

## 待补

- [ ] 梳理插件下载 URL 模式与 Higress / Console 对接配置。
- [ ] 梳理镜像版本、插件版本、HTTP 地址模式之间的统一口径。
- [ ] 补充本地运行、K8s 部署和发布验收的最小检查清单。
