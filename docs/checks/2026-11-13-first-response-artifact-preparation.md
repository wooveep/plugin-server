# 首响应采集制品候选准备

2026-11-13，当前 dev。采集源码基于 b1df05c7776b9465923f5877c19ab6b833100bd2，新增首响应头耗时沿[采集说明](2026-11-13-first-response-telemetry.md)。当前权威 ai-proxy VERSION/catalog/spec 为2.0.80；旧option.yaml不是发行版本来源。本批推进为2.0.81，保留旧2.0.79/2.0.80归档与活动路由原引用。

共享executionfacts输入依既有source_inputs不可变规则影响46个Go发行实现，只增补丁号并保持原alpha性质；元数据仅改各VERSION、spec的info.version及catalog的对应declaredVersion。8个C++与1个Rust原制品保持。构建使用仓库固定Go1.26.3容器（toolchains.json的不可变digest），不拿临时宿主Wasm替代正式完整包，不增加业务代码、依赖、推理、价格或资金修改。

| 插件 | 原版本 | 候选版本 |
| --- | --- | --- |
| ai-agent | 2.0.7 | 2.0.8 |
| ai-billing | 1.0.16 | 1.0.17 |
| ai-cache | 2.0.29 | 2.0.30 |
| ai-context-limit | 1.0.4 | 1.0.5 |
| ai-endpoint-picker | 2.0.6 | 2.0.7 |
| ai-history | 2.0.17 | 2.0.18 |
| ai-intent | 2.0.7 | 2.0.8 |
| ai-json-resp | 2.0.7 | 2.0.8 |
| ai-load-balancer | 2.0.7 | 2.0.8 |
| ai-memory | 0.1.21 | 0.1.22 |
| ai-prompt-decorator | 2.0.7 | 2.0.8 |
| ai-prompt-template | 2.0.7 | 2.0.8 |
| ai-proxy | 2.0.80 | 2.0.81 |
| ai-quota | 2.0.13 | 2.0.14 |
| ai-rag | 2.0.7 | 2.0.8 |
| ai-search | 2.0.7 | 2.0.8 |
| ai-security-guard | 2.0.7 | 2.0.8 |
| ai-statistics | 2.0.14 | 2.0.15 |
| ai-token-ratelimit | 2.0.11 | 2.0.12 |
| ai-transformer | 2.0.7 | 2.0.8 |
| cache-control | 2.0.7 | 2.0.8 |
| cluster-key-rate-limit | 2.0.8 | 2.0.9 |
| cors | 2.0.8 | 2.0.9 |
| de-graphql | 2.0.7 | 2.0.8 |
| ext-auth | 2.0.7 | 2.0.8 |
| frontend-gray | 2.0.7 | 2.0.8 |
| geo-ip | 2.0.7 | 2.0.8 |
| gw-error-format | 1.0.6 | 1.0.7 |
| hmac-auth-apisix | 2.0.7 | 2.0.8 |
| ip-restriction | 2.0.7 | 2.0.8 |
| json-converter | 2.0.7 | 2.0.8 |
| key-auth | 1.2.7 | 1.2.8 |
| log-request-response | 1.0.4 | 1.0.5 |
| mcp-router | 2.0.7 | 2.0.8 |
| mcp-server | 2.0.7 | 2.0.8 |
| model-mapper | 2.0.9 | 2.0.10 |
| model-router | 2.0.7 | 2.0.8 |
| nginx-rewrite-compatible | 1.0.4 | 1.0.5 |
| oidc | 2.0.7 | 2.0.8 |
| opa | 2.0.7 | 2.0.8 |
| replay-protection | 1.0.4-alpha | 1.0.5-alpha |
| request-validation | 2.0.7 | 2.0.8 |
| response-cache | 1.0.7 | 1.0.8 |
| traffic-tag | 2.0.7 | 2.0.8 |
| transformer | 2.0.7 | 2.0.8 |
| waf | 2.0.7 | 2.0.8 |

当前仅源码/版本准备，完整构建、包摘要、来源策略独立核对、入库与PluginServer镜像仍待执行。发行前备份本地登记库及原策略，原版本不覆盖；未来镜像/PVC只按原种子流程追加已审完整版本。新增目录不改变已冻结路由引用，正式Qwen/Deep须另获明确原生发布范围后才能切换。Console接收列与认证ingest应先部署；真实首响应采集仍需后续真实请求验证，不凭包构建通过关闭I22。
