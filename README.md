# Higress Plugin Server

本仓库统一维护 Higress 插件源码、固定工具链编译、完整制品入库和 HTTP 分发。日常源码位于 `plugins/`；编译中间文件位于 `build/`；已验证的不可变交付库位于 `dist/plugins/`。原独立 Go 插件/SDK 目录仅保留历史，`higress/` 保留上游参考。

需求、计划和验收使用普通 Markdown 与 Git/PR；本项目不使用 Superpowers 或 OpenSpec，历史资料中的执行指令不再生效。当前实施及验收以[统一源码与交付计划](docs/plans/2026-10-31-unified-plugin-source-and-delivery.md)为准；[原生媒体交付记录](docs/checks/2026-10-31-native-media-plugin-contract.md)单独维护实际版本和下载证据。

## 源码和默认集合

[来源锁定记录](plugins/upstream-lock.json)保留上游提交、原始树及 Git 跟踪分支；[定制整合记录](plugins/release/custom-integration.json)说明逐插件取舍和合入。后续上游快照沿既有跟踪分支形成父链，通过 `git merge -s ort -Xsubtree=plugins <新快照提交>` 三方合并，保留本地定制。

[唯一发行清单](plugins/release/catalog.json)列出 72 个源码实现，按真实 `VERSION` 的 SemVer 选择 55 个正式 Wasm 插件。无版本旧实现、示例、废弃插件和原生 `.so` 单独分类；不生成伪造旧版本别名。查看当前版本：

```bash
make list-plugins
```

## 构建与完整包

安装固定 Python 依赖，默认使用[固定本地容器工具链](plugins/release/toolchains.json)：

```bash
make bootstrap
make build-plugin PLUGIN=model-router
make build-all
make prepare-packages
```

`HOST_TOOLCHAIN=1` 显式选择宿主工具链。示例或原生模块须同时指定 `PLUGIN` 与 `IMPLEMENTATION`；原生 `.so` 通过网关镜像集成，不作为 Wasm 下载。Go 模块锁变更使用 `make update-go-locks`，常规编译只读依赖锁。

完整包包含 Wasm、独立配置 schema、默认模板、文档、环境引用和来源。准备阶段输出 `build/packages/review.json`，记录源码输入、编译回执与完整内容摘要。独立核对后提供受信策略，首次入库时执行完整包校验：

```bash
make package-plugins TRUSTED_POLICY=/absolute/path/to/reviewed-policy.json
make docker-build TRUSTED_POLICY=/absolute/path/to/reviewed-policy.json TAG=higress-plugin-server:local
```

同名同版本不允许覆盖不同内容。已登记版本经当前编译、全部载荷摘要和环境引用比对完全相同时，复用原归档、artifact_id 与来源；当前编译提交另记入 review。列表、镜像构建与 PVC 初始化读取已登记元数据，不反复校验完整包。镜像独立对照正式清单，缺少任一选定版本即构建失败。

## HTTP 与上传

- `GET /artifacts`：批量已登记 manifest 与 `x-higress` 元数据，可供消费端缓存及分页。
- `GET /plugins/{name}/{version}/manifest.json`：精确版本登记记录。
- `GET /plugins/{name}/{version}/plugin.wasm`：Wasm 下载；同时支持已登记的配置、模板和文档。
- `GET /plugins/{name}/{version}/archive.tgz`：原完整归档下载。
- `PUT /plugins/{name}/{version}/package.tgz`：鉴权上传完整包，使用持久化 upload ID 查询和重试未知结果。

本地上传 CLI：

```bash
.venv/bin/python upload_plugin.py \
  --server http://localhost:8080 \
  --file ./package.tgz --name demo --version 1.2.3 \
  --upload-id demo-release-1.2.3 --policy /absolute/path/to/reviewed-policy.json
```

Token 由 `PLUGIN_SERVER_UPLOAD_TOKEN` 提供；服务端策略由 `PLUGIN_SERVER_POLICY_FILE` 指定。客户端和服务端使用完整包协议；旧的裸 Wasm PUT 不是当前入库方式。

## 运行与 PVC

镜像服务监听 8080。Kubernetes 部署资产由 `modelfusion-deploy/` 管理，工作区[本地环境与部署流程](../modelfusion-deploy/docs/local-test-environment.md)是环境入口。挂载 `/usr/share/plugin-server` 同时保留交付文件和相邻的制品状态目录。

镜像内提供已登记库、`/opt/plugin-server/plugin-catalog.json` 和制品 inventory。PVC 初始化使用：

```bash
python3 /opt/plugin-server/scripts/seed-registered-store.py \
  --source /usr/share/plugin-server/plugins \
  --source-state /usr/share/plugin-server/.plugins-artifact-state \
  --destination /data/plugins \
  --destination-state /data/.plugins-artifact-state
```

该入口复制已验证登记记录；同内容保留既有 ID，不同内容的同版本拒绝覆盖。HTTP URL 模板为 `http://higress-plugin-server.higress-system.svc:80/plugins/${name}/${version}/plugin.wasm`，消费端必须使用实际已登记的精确版本。上传新版本本身不改变已有生效绑定。

## 验证与历史资料

`make test` 统一运行 Python 契约、存储、发布清单和入口测试。各语言编译和 Go/编译后 Wasm 回归记录由实施计划链接；本地测试通过不代表真实 Envoy 或供应商调用已验收。

以下是历史设计入口，其阶段描述保留原文；当前进展以上述实施计划和验收回执为准。

