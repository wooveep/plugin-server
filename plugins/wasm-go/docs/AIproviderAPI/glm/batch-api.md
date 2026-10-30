# GLM 批处理 API

- Category: 批处理-api
- Source root: https://docs.bigmodel.cn/api-reference/批处理-api
- Pages: 4
- Fetched on: 2026-04-23
- Fetch failures: 0

## Sources

- https://docs.bigmodel.cn/api-reference/批处理-api/列出批处理任务
- https://docs.bigmodel.cn/api-reference/批处理-api/创建批处理任务
- https://docs.bigmodel.cn/api-reference/批处理-api/取消批处理任务
- https://docs.bigmodel.cn/api-reference/批处理-api/检索批处理任务

---

> ## Documentation Index
> Fetch the complete documentation index at: https://docs.bigmodel.cn/llms.txt
> Use this file to discover all available pages before exploring further.

# 列出批处理任务

> 获取批量处理任务列表，支持分页。见 [批量服务](https://docs.bigmodel.cn/cn/guide/tools/batch)



## OpenAPI

````yaml /openapi/openapi.json get /paas/v4/batches
openapi: 3.0.1
info:
  title: ZHIPU AI API
  description: ZHIPU AI 接口提供强大的 AI 能力，包括聊天对话、工具调用和视频生成。
  license:
    name: ZHIPU AI 开发者协议和政策
    url: https://chat.z.ai/legal-agreement/terms-of-service
  version: 1.0.0
  contact:
    name: Z.AI 开发者
    url: https://chat.z.ai/legal-agreement/privacy-policy
    email: user_feedback@z.ai
servers:
  - url: https://open.bigmodel.cn/api/
    description: 开放平台服务
security:
  - bearerAuth: []
tags:
  - name: 模型 API
    description: Chat API
  - name: 工具 API
    description: Web Search API
  - name: Agent API
    description: Agent API
  - name: 文件 API
    description: File API
  - name: 知识库 API
    description: Knowledge API
  - name: 实时 API
    description: Realtime API
  - name: 批处理 API
    description: Batch API
  - name: 助理 API
    description: Assistant API
  - name: 智能体 API（旧）
    description: QingLiu Agent API
paths:
  /paas/v4/batches:
    get:
      tags:
        - 批处理 API
      summary: 列出批处理任务
      description: 获取批量处理任务列表，支持分页。见 [批量服务](https://docs.bigmodel.cn/cn/guide/tools/batch)
      parameters:
        - name: after
          in: query
          description: 分页游标，用于获取指定ID之后的结果
          schema:
            type: string
        - name: limit
          in: query
          description: 返回结果的最大数量
          schema:
            type: integer
            default: 20
      responses:
        '200':
          description: 业务处理成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/BatchListResponse'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
components:
  schemas:
    BatchListResponse:
      type: object
      properties:
        object:
          type: string
          enum:
            - list
        data:
          type: array
          items:
            $ref: '#/components/schemas/Batch'
        first_id:
          type: string
          description: 第一个`ID`
        last_id:
          type: string
          description: 最后一个`ID`
        has_more:
          type: boolean
          description: 是否有更多数据
    Error:
      type: object
      properties:
        error:
          required:
            - code
            - message
          type: object
          properties:
            code:
              type: string
            message:
              type: string
    Batch:
      type: object
      properties:
        id:
          type: string
          description: 批处理的唯一标识符。
        object:
          type: string
          description: 对象类型，这里为 `batch`。
        endpoint:
          type: string
          description: 批处理使用的 `API` 端点。
        input_file_id:
          type: string
          description: 批处理使用的输入文件的`ID`。
        completion_window:
          type: string
          description: 批处理应在此时间框架内完成的期限。
        status:
          type: string
          description: 批处理的当前状态。
        output_file_id:
          type: string
          description: 包含成功执行请求的输出的文件`ID`。
        error_file_id:
          type: string
          description: 包含出现错误的请求的输出的文件`ID`。
        created_at:
          type: integer
          description: 创建批处理的`Unix`时间戳（秒）。
        in_progress_at:
          type: integer
          description: 批处理开始处理的`Unix`时间戳（秒）。
        expires_at:
          type: integer
          description: 批处理将过期的`Unix`时间戳（秒）。
        finalizing_at:
          type: integer
          description: 批处理开始最终处理的`Unix`时间戳（秒）。
        completed_at:
          type: integer
          description: 批处理完成的`Unix`时间戳（秒）。
        failed_at:
          type: integer
          description: 批处理失败的`Unix`时间戳（秒）。
        expired_at:
          type: integer
          description: 批处理过期的`Unix`时间戳（秒）。
        cancelling_at:
          type: integer
          description: 批处理开始取消的`Unix`时间戳（秒）。
        cancelled_at:
          type: integer
          description: 批处理取消完成的`Unix`时间戳（秒）。
        request_counts:
          type: integer
          description: batch 请求计数。
        total:
          type: integer
          description: 批处理中的请求总数。
        completed:
          type: integer
          description: 批处理中已成功完成的请求数量。
        failed:
          type: integer
          description: 批处理中失败的请求数量。
        metadata:
          type: object
          description: >-
            可附加到对象上的 `16` 个键值对的集合。这有助于以结构化格式存储对象的附加信息。键的长度最多为 `64` 个字符，值的长度最多为
            `512` 个字符。
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      description: >-
        使用以下格式进行身份验证：Bearer [<your api
        key>](https://bigmodel.cn/usercenter/proj-mgmt/apikeys)

````
---

> ## Documentation Index
> Fetch the complete documentation index at: https://docs.bigmodel.cn/llms.txt
> Use this file to discover all available pages before exploring further.

# 创建批处理任务

> 创建一个新的批量处理任务。见 [批量服务](https://docs.bigmodel.cn/cn/guide/tools/batch)



## OpenAPI

````yaml /openapi/openapi.json post /paas/v4/batches
openapi: 3.0.1
info:
  title: ZHIPU AI API
  description: ZHIPU AI 接口提供强大的 AI 能力，包括聊天对话、工具调用和视频生成。
  license:
    name: ZHIPU AI 开发者协议和政策
    url: https://chat.z.ai/legal-agreement/terms-of-service
  version: 1.0.0
  contact:
    name: Z.AI 开发者
    url: https://chat.z.ai/legal-agreement/privacy-policy
    email: user_feedback@z.ai
servers:
  - url: https://open.bigmodel.cn/api/
    description: 开放平台服务
security:
  - bearerAuth: []
tags:
  - name: 模型 API
    description: Chat API
  - name: 工具 API
    description: Web Search API
  - name: Agent API
    description: Agent API
  - name: 文件 API
    description: File API
  - name: 知识库 API
    description: Knowledge API
  - name: 实时 API
    description: Realtime API
  - name: 批处理 API
    description: Batch API
  - name: 助理 API
    description: Assistant API
  - name: 智能体 API（旧）
    description: QingLiu Agent API
paths:
  /paas/v4/batches:
    post:
      tags:
        - 批处理 API
      summary: 创建批处理任务
      description: 创建一个新的批量处理任务。见 [批量服务](https://docs.bigmodel.cn/cn/guide/tools/batch)
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/BatchCreateRequest'
        required: true
      responses:
        '200':
          description: 业务处理成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Batch'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
components:
  schemas:
    BatchCreateRequest:
      type: object
      properties:
        input_file_id:
          type: string
          description: >-
            上传文件的 `ID`，该文件包含`Batch`的请求。输入文件必须是 `.Jsonl`
            格式，并且文件上传时的目的必须标记为"batch"。
        endpoint:
          type: string
          description: '`Batch` 中所有请求将使用的端点。目前支持 `/v4/chat/completions`。'
          enum:
            - /v4/chat/completions
        auto_delete_input_file:
          type: boolean
          description: 是否自动删除`batch`原始文件，默认为`True`：`True`：执行自动删除。`False`：保留原始`batch`文件。
          default: true
        metadata:
          type: object
          nullable: true
          description: >-
            用于存储与 `Batch` 相关的数据，如客户`I`D、描述或其他任务管理和跟踪所需的额外信息。可附加到对象上的键值对集合最多为
            `16` 个。每个键的长度最多为 `64` 个字符，每个值的长度最多为 `512` 个字符。
      required:
        - input_file_id
        - endpoint
    Batch:
      type: object
      properties:
        id:
          type: string
          description: 批处理的唯一标识符。
        object:
          type: string
          description: 对象类型，这里为 `batch`。
        endpoint:
          type: string
          description: 批处理使用的 `API` 端点。
        input_file_id:
          type: string
          description: 批处理使用的输入文件的`ID`。
        completion_window:
          type: string
          description: 批处理应在此时间框架内完成的期限。
        status:
          type: string
          description: 批处理的当前状态。
        output_file_id:
          type: string
          description: 包含成功执行请求的输出的文件`ID`。
        error_file_id:
          type: string
          description: 包含出现错误的请求的输出的文件`ID`。
        created_at:
          type: integer
          description: 创建批处理的`Unix`时间戳（秒）。
        in_progress_at:
          type: integer
          description: 批处理开始处理的`Unix`时间戳（秒）。
        expires_at:
          type: integer
          description: 批处理将过期的`Unix`时间戳（秒）。
        finalizing_at:
          type: integer
          description: 批处理开始最终处理的`Unix`时间戳（秒）。
        completed_at:
          type: integer
          description: 批处理完成的`Unix`时间戳（秒）。
        failed_at:
          type: integer
          description: 批处理失败的`Unix`时间戳（秒）。
        expired_at:
          type: integer
          description: 批处理过期的`Unix`时间戳（秒）。
        cancelling_at:
          type: integer
          description: 批处理开始取消的`Unix`时间戳（秒）。
        cancelled_at:
          type: integer
          description: 批处理取消完成的`Unix`时间戳（秒）。
        request_counts:
          type: integer
          description: batch 请求计数。
        total:
          type: integer
          description: 批处理中的请求总数。
        completed:
          type: integer
          description: 批处理中已成功完成的请求数量。
        failed:
          type: integer
          description: 批处理中失败的请求数量。
        metadata:
          type: object
          description: >-
            可附加到对象上的 `16` 个键值对的集合。这有助于以结构化格式存储对象的附加信息。键的长度最多为 `64` 个字符，值的长度最多为
            `512` 个字符。
    Error:
      type: object
      properties:
        error:
          required:
            - code
            - message
          type: object
          properties:
            code:
              type: string
            message:
              type: string
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      description: >-
        使用以下格式进行身份验证：Bearer [<your api
        key>](https://bigmodel.cn/usercenter/proj-mgmt/apikeys)

````
---

> ## Documentation Index
> Fetch the complete documentation index at: https://docs.bigmodel.cn/llms.txt
> Use this file to discover all available pages before exploring further.

# 取消批处理任务

> 根据批处理任务`ID`取消正在运行的批量处理任务。见 [批量服务](https://docs.bigmodel.cn/cn/guide/tools/batch)



## OpenAPI

````yaml /openapi/openapi.json post /paas/v4/batches/{batch_id}/cancel
openapi: 3.0.1
info:
  title: ZHIPU AI API
  description: ZHIPU AI 接口提供强大的 AI 能力，包括聊天对话、工具调用和视频生成。
  license:
    name: ZHIPU AI 开发者协议和政策
    url: https://chat.z.ai/legal-agreement/terms-of-service
  version: 1.0.0
  contact:
    name: Z.AI 开发者
    url: https://chat.z.ai/legal-agreement/privacy-policy
    email: user_feedback@z.ai
servers:
  - url: https://open.bigmodel.cn/api/
    description: 开放平台服务
security:
  - bearerAuth: []
tags:
  - name: 模型 API
    description: Chat API
  - name: 工具 API
    description: Web Search API
  - name: Agent API
    description: Agent API
  - name: 文件 API
    description: File API
  - name: 知识库 API
    description: Knowledge API
  - name: 实时 API
    description: Realtime API
  - name: 批处理 API
    description: Batch API
  - name: 助理 API
    description: Assistant API
  - name: 智能体 API（旧）
    description: QingLiu Agent API
paths:
  /paas/v4/batches/{batch_id}/cancel:
    post:
      tags:
        - 批处理 API
      summary: 取消批处理任务
      description: 根据批处理任务`ID`取消正在运行的批量处理任务。见 [批量服务](https://docs.bigmodel.cn/cn/guide/tools/batch)
      parameters:
        - name: batch_id
          in: path
          required: true
          description: 批处理任务ID
          schema:
            type: string
      responses:
        '200':
          description: 业务处理成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Batch'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
components:
  schemas:
    Batch:
      type: object
      properties:
        id:
          type: string
          description: 批处理的唯一标识符。
        object:
          type: string
          description: 对象类型，这里为 `batch`。
        endpoint:
          type: string
          description: 批处理使用的 `API` 端点。
        input_file_id:
          type: string
          description: 批处理使用的输入文件的`ID`。
        completion_window:
          type: string
          description: 批处理应在此时间框架内完成的期限。
        status:
          type: string
          description: 批处理的当前状态。
        output_file_id:
          type: string
          description: 包含成功执行请求的输出的文件`ID`。
        error_file_id:
          type: string
          description: 包含出现错误的请求的输出的文件`ID`。
        created_at:
          type: integer
          description: 创建批处理的`Unix`时间戳（秒）。
        in_progress_at:
          type: integer
          description: 批处理开始处理的`Unix`时间戳（秒）。
        expires_at:
          type: integer
          description: 批处理将过期的`Unix`时间戳（秒）。
        finalizing_at:
          type: integer
          description: 批处理开始最终处理的`Unix`时间戳（秒）。
        completed_at:
          type: integer
          description: 批处理完成的`Unix`时间戳（秒）。
        failed_at:
          type: integer
          description: 批处理失败的`Unix`时间戳（秒）。
        expired_at:
          type: integer
          description: 批处理过期的`Unix`时间戳（秒）。
        cancelling_at:
          type: integer
          description: 批处理开始取消的`Unix`时间戳（秒）。
        cancelled_at:
          type: integer
          description: 批处理取消完成的`Unix`时间戳（秒）。
        request_counts:
          type: integer
          description: batch 请求计数。
        total:
          type: integer
          description: 批处理中的请求总数。
        completed:
          type: integer
          description: 批处理中已成功完成的请求数量。
        failed:
          type: integer
          description: 批处理中失败的请求数量。
        metadata:
          type: object
          description: >-
            可附加到对象上的 `16` 个键值对的集合。这有助于以结构化格式存储对象的附加信息。键的长度最多为 `64` 个字符，值的长度最多为
            `512` 个字符。
    Error:
      type: object
      properties:
        error:
          required:
            - code
            - message
          type: object
          properties:
            code:
              type: string
            message:
              type: string
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      description: >-
        使用以下格式进行身份验证：Bearer [<your api
        key>](https://bigmodel.cn/usercenter/proj-mgmt/apikeys)

````
---

> ## Documentation Index
> Fetch the complete documentation index at: https://docs.bigmodel.cn/llms.txt
> Use this file to discover all available pages before exploring further.

# 检索批处理任务

> 根据批处理任务`ID`获取批量处理任务详情。见 [批量服务](https://docs.bigmodel.cn/cn/guide/tools/batch)



## OpenAPI

````yaml /openapi/openapi.json get /paas/v4/batches/{batch_id}
openapi: 3.0.1
info:
  title: ZHIPU AI API
  description: ZHIPU AI 接口提供强大的 AI 能力，包括聊天对话、工具调用和视频生成。
  license:
    name: ZHIPU AI 开发者协议和政策
    url: https://chat.z.ai/legal-agreement/terms-of-service
  version: 1.0.0
  contact:
    name: Z.AI 开发者
    url: https://chat.z.ai/legal-agreement/privacy-policy
    email: user_feedback@z.ai
servers:
  - url: https://open.bigmodel.cn/api/
    description: 开放平台服务
security:
  - bearerAuth: []
tags:
  - name: 模型 API
    description: Chat API
  - name: 工具 API
    description: Web Search API
  - name: Agent API
    description: Agent API
  - name: 文件 API
    description: File API
  - name: 知识库 API
    description: Knowledge API
  - name: 实时 API
    description: Realtime API
  - name: 批处理 API
    description: Batch API
  - name: 助理 API
    description: Assistant API
  - name: 智能体 API（旧）
    description: QingLiu Agent API
paths:
  /paas/v4/batches/{batch_id}:
    get:
      tags:
        - 批处理 API
      summary: 检索批处理任务
      description: 根据批处理任务`ID`获取批量处理任务详情。见 [批量服务](https://docs.bigmodel.cn/cn/guide/tools/batch)
      parameters:
        - name: batch_id
          in: path
          required: true
          description: 批处理任务的唯一标识符。
          schema:
            type: string
      responses:
        '200':
          description: 请求成功，返回 Batch 对象。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Batch'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
components:
  schemas:
    Batch:
      type: object
      properties:
        id:
          type: string
          description: 批处理的唯一标识符。
        object:
          type: string
          description: 对象类型，这里为 `batch`。
        endpoint:
          type: string
          description: 批处理使用的 `API` 端点。
        input_file_id:
          type: string
          description: 批处理使用的输入文件的`ID`。
        completion_window:
          type: string
          description: 批处理应在此时间框架内完成的期限。
        status:
          type: string
          description: 批处理的当前状态。
        output_file_id:
          type: string
          description: 包含成功执行请求的输出的文件`ID`。
        error_file_id:
          type: string
          description: 包含出现错误的请求的输出的文件`ID`。
        created_at:
          type: integer
          description: 创建批处理的`Unix`时间戳（秒）。
        in_progress_at:
          type: integer
          description: 批处理开始处理的`Unix`时间戳（秒）。
        expires_at:
          type: integer
          description: 批处理将过期的`Unix`时间戳（秒）。
        finalizing_at:
          type: integer
          description: 批处理开始最终处理的`Unix`时间戳（秒）。
        completed_at:
          type: integer
          description: 批处理完成的`Unix`时间戳（秒）。
        failed_at:
          type: integer
          description: 批处理失败的`Unix`时间戳（秒）。
        expired_at:
          type: integer
          description: 批处理过期的`Unix`时间戳（秒）。
        cancelling_at:
          type: integer
          description: 批处理开始取消的`Unix`时间戳（秒）。
        cancelled_at:
          type: integer
          description: 批处理取消完成的`Unix`时间戳（秒）。
        request_counts:
          type: integer
          description: batch 请求计数。
        total:
          type: integer
          description: 批处理中的请求总数。
        completed:
          type: integer
          description: 批处理中已成功完成的请求数量。
        failed:
          type: integer
          description: 批处理中失败的请求数量。
        metadata:
          type: object
          description: >-
            可附加到对象上的 `16` 个键值对的集合。这有助于以结构化格式存储对象的附加信息。键的长度最多为 `64` 个字符，值的长度最多为
            `512` 个字符。
    Error:
      type: object
      properties:
        error:
          required:
            - code
            - message
          type: object
          properties:
            code:
              type: string
            message:
              type: string
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      description: >-
        使用以下格式进行身份验证：Bearer [<your api
        key>](https://bigmodel.cn/usercenter/proj-mgmt/apikeys)

````
