# Kimi API Overview

- Source group: official `platform.kimi.com/docs/api` pages
- Fetched on: 2026-04-23

## Sources

- https://platform.kimi.com/docs/api/overview

---

> ## Documentation Index
> Fetch the complete documentation index at: https://platform.kimi.com/docs/llms.txt
> Use this file to discover all available pages before exploring further.

# API 概述

## 服务地址

```
https://api.moonshot.cn
```

Kimi 开放平台提供兼容 OpenAI 协议的 HTTP API，您可以直接使用 OpenAI SDK 接入。

使用 SDK 时，`base_url` 设置为 `https://api.moonshot.cn/v1`；直接调用 HTTP 端点时，完整路径如 `https://api.moonshot.cn/v1/chat/completions`。

## 兼容 OpenAI

我们的 API 在请求/响应格式上兼容 OpenAI Chat Completions API。这意味着：

* 可以直接使用 OpenAI 官方 SDK（Python / Node.js）
* 支持大多数兼容 OpenAI 的第三方工具和框架（LangChain、Dify、Coze 等）
* 只需将 `base_url` 指向 `https://api.moonshot.cn/v1` 即可切换

<Note>
  部分参数为 Kimi 专有扩展：`thinking` 参数需要通过 SDK 的 `extra_body` 传递；`partial` 是写在 messages 中 assistant 消息上的字段（`"partial": true`），不是顶层请求参数。详见[工具调用](https://platform.kimi.com/docs/api/tool-use)和 [Partial Mode](https://platform.kimi.com/docs/api/partial)。
</Note>

## 认证

所有 API 请求需要在 HTTP 头中携带 API Key：

```
Authorization: Bearer $MOONSHOT_API_KEY
```

API Key 可在 [Kimi 开放平台控制台](https://platform.kimi.com/console/api-keys) 创建和管理。

<Warning>
  API Key 是敏感信息，请妥善保管。不要在客户端代码、公开仓库或日志中暴露。建议通过环境变量管理。
</Warning>

## SDK 安装

<CodeGroup>
  ```bash Python theme={null}
  pip install --upgrade 'openai>=1.0'
  ```

  ```bash Node.js theme={null}
  npm install openai
  ```
</CodeGroup>

初始化客户端：

<CodeGroup>
  ```python Python theme={null}
  from openai import OpenAI

  client = OpenAI(
      api_key="$MOONSHOT_API_KEY",
      base_url="https://api.moonshot.cn/v1",
  )
  ```

  ```javascript Node.js theme={null}
  const OpenAI = require("openai");

  const client = new OpenAI({
      apiKey: "$MOONSHOT_API_KEY",
      baseURL: "https://api.moonshot.cn/v1",
  });
  ```
</CodeGroup>

<Note>
  Python 版本需 ≥ 3.7.1，Node.js 版本需 ≥ 18，OpenAI SDK 版本需 ≥ 1.0.0。

  ```bash theme={null}
  python -c 'import openai; print("version =", openai.__version__)'
  ```
</Note>

## 通用请求头

| 请求头             | 值                          | 说明    |
| --------------- | -------------------------- | ----- |
| `Content-Type`  | `application/json`         | 请求体格式 |
| `Authorization` | `Bearer $MOONSHOT_API_KEY` | 认证令牌  |

## 错误处理

请求失败时返回 JSON 格式的错误响应，包含 `error.type` 和 `error.message` 字段。常见的 HTTP 状态码包括 400（请求错误）、401（认证失败）、429（速率限制）、500（服务端错误）等。

完整的错误类型、错误消息和排障建议，请参阅[错误说明](https://platform.kimi.com/docs/api/errors)。

## API 端点一览

| 端点                                    | 方法     | 说明                            |
| ------------------------------------- | ------ | ----------------------------- |
| `/v1/chat/completions`                | POST   | [创建对话补全](https://platform.kimi.com/docs/api/chat)           |
| `/v1/models`                          | GET    | [列出模型](https://platform.kimi.com/docs/api/list-models)      |
| `/v1/tokenizers/estimate-token-count` | POST   | [计算 Token](https://platform.kimi.com/docs/api/estimate)     |
| `/v1/users/me/balance`                | GET    | [查询余额](https://platform.kimi.com/docs/api/balance)          |
| `/v1/files`                           | POST   | [上传文件](https://platform.kimi.com/docs/api/files-upload)     |
| `/v1/files`                           | GET    | [列出文件](https://platform.kimi.com/docs/api/files-list)       |
| `/v1/files/{file_id}`                 | GET    | [获取文件信息](https://platform.kimi.com/docs/api/files-retrieve) |
| `/v1/files/{file_id}`                 | DELETE | [删除文件](https://platform.kimi.com/docs/api/files-delete)     |
| `/v1/files/{file_id}/content`         | GET    | [获取文件内容](https://platform.kimi.com/docs/api/files-content)  |

## 下一步

<CardGroup cols={2}>
  <Card title="快速开始" icon="rocket" href="https://platform.kimi.com/docs/api/quickstart">
    发送第一个 API 请求
  </Card>

  <Card title="模型总览" icon="cubes" href="https://platform.kimi.com/docs/api/models-overview">
    了解各模型的能力和参数差异
  </Card>

  <Card title="工具调用" icon="wrench" href="https://platform.kimi.com/docs/api/tool-use">
    让模型调用外部函数
  </Card>

  <Card title="创建对话补全" icon="code" href="https://platform.kimi.com/docs/api/chat">
    完整的端点参数参考
  </Card>
</CardGroup>
