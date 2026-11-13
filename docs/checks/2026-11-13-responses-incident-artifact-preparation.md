# Responses 补修制品准备记录

状态：固定源码的完整包、候选镜像、克隆及Helm部署前检查完成；开发环境仍为Helm104。**尚未部署新镜像，也未创建本次验证身份、模型、Mock或发送新模型请求。** 本记录不作为真实Envoy验收或issue关闭依据。

源码固定为 `dacd69e148f61330b713d1a4854578e11b7fdcc3`，候选为 ai-proxy **2.0.82**。源码范围、39项简单回归及最终两轴结果见[源码验证记录](2026-11-13-responses-incident-source-validation.md)；后续真实请求及清理顺序见[网关验证方案](2026-11-13-responses-incident-native-verification-plan.md)。对应机器可读摘要为[制品记录JSON](2026-11-13-responses-incident-artifact-preparation.json)。

| 项目 | 固定值 |
| --- | --- |
| 源码archive SHA256 | `a80dce4047a195e52b286c747a57aa7d9180a4ca128196e4c5a106f8f058b76f` |
| Wasm SHA256／字节 | `sha256:3b6b0ed80a2699c2b51f4f4e53c510a9df2b5b9dd7fbaaaa67e6b774f1f804cf`／24716504 |
| 完整包archive SHA256 | `sha256:b5ab18383b44916221a4479c1c283a167f401542eb85c7a2ec984ce5d6feb8bd` |
| content digest | `sha256:da66b8617fb78307415661e21e8f8fa9173b9f3783c922efcae13c8f5ae408e6` |
| artifact ID | `10f364bd-59f8-4906-922e-d60e00f7c779` |
| manifest SHA256 | `88df2169e8b0911143850c9015340489df13fe8f55176937f6f652f4ba5506d1` |
| 镜像index digest | `sha256:4405a58a09958b5177d74b10ea3bbe34a4c17deb9d6c1e34818b57dfcb13d794` |
| Linux amd64 manifest digest | `sha256:fb9bc293031c7935a369125970939aa6dbd78c5a33e9aad819fc312fc98823c4` |
| 实际镜像catalog SHA256 | `03b71dc5ef01caf168e5afe8f1f6369a90869da87815acd109272e239775d2df` |
| 实际镜像policy SHA256 | `4e0a16fbfab402c033cb6cd802a59217bfcd4a4d9acd689dadeab8a5ebe1cd02` |

候选镜像：`registry.lc.wooveep.com:9980/modelfusion/plugin-server:V2.2.1-dev-20261113-responses-dacd69e1@sha256:4405a58a09958b5177d74b10ea3bbe34a4c17deb9d6c1e34818b57dfcb13d794`。

编译输入来自 `git archive` 的固定提交，636个编译输入逐字节匹配。使用既有固定Go工具链和正式构建、打包、ArtifactStore入口，只构建并新增本次82完整包。独立复核确认旧策略拒绝未批准的新包，新策略完整校验通过；保留原262条有序provenance receipts及环境引用策略，仅追加本包精确source/Wasm/content三元组。

原 `.dockerignore` 未允许新的固定selection文件。本机构建使用单独 `Dockerfile.dockerignore`，保留原规则字节并只追加该文件的allow，摘要 `ce45cd89d124de05d382a22781807cc64f7b99bc91543b9402084544ce6ae393`。Dockerfile、selection和全部tracked源码仍为固定archive原字节；该构建上下文覆盖不改产品源码。

实际镜像含55个完整选择包，546个文件、606944452字节与登记输入逐字节相等。其余54项完整选择、ID、manifest、archive及payload不变。镜像Python运行源码也逐字节吻合固定提交。

独立克隆旧282登记库，以新镜像的正常初始化脚本seed：新增1、保留54，得到283个登记制品。重启前后HTTP目录、当前64个Wasm引用及旧81／新82的29个唯一下载路径均返回200且摘要相等；旧282全部ID、manifest、archive、payload及282条完整上传历史均保留。本机验证容器及匿名卷已移除。

部署前已基于实际104重新备份PostgreSQL、Redis、插件卷／一致性SQLite、完整values／manifest／chart／Secrets及资源清单。PG完整archive读取、RDB检查、完整gzip／TAR读取和SQLite282项quick_check均通过；原RT Key再次确认已于00:00Z过期，明文未读取，有效期未更改。

仅准备104→105的单值变更 `/higress-core/pluginServer/tag`：73个资源中仅插件Deployment的main/init镜像变化。Helm lint、正向server dry-run及普通逆向server dry-run通过；完整values及73个资源可逆向精确恢复104。旧282及新增82在同一PVC中的不可变字节和登记历史均须继续保留。实际升级和真实Wasm请求验证待独立执行窗口。
