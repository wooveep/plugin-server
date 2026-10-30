# GLM 文件 API

- Category: 文件-api
- Source root: https://docs.bigmodel.cn/api-reference/文件-api
- Pages: 4
- Fetched on: 2026-04-23
- Fetch failures: 0

## Sources

- https://docs.bigmodel.cn/api-reference/文件-api/上传文件
- https://docs.bigmodel.cn/api-reference/文件-api/删除文件
- https://docs.bigmodel.cn/api-reference/文件-api/文件内容
- https://docs.bigmodel.cn/api-reference/文件-api/文件列表

---

> ## Documentation Index
> Fetch the complete documentation index at: https://docs.bigmodel.cn/llms.txt
> Use this file to discover all available pages before exploring further.

# 上传文件

> 上传用于 `Batch 任务`、`智能体` 等功能的文件。注意 `Try it` 功能仅支持小文件上传，实际支持的文件大小请参见下文 `purpose` 相关说明。



## OpenAPI

````yaml /openapi/openapi.json post /paas/v4/files
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
  /paas/v4/files:
    post:
      tags:
        - 文件 API
      summary: 上传文件
      description: >-
        上传用于 `Batch 任务`、`智能体` 等功能的文件。注意 `Try it` 功能仅支持小文件上传，实际支持的文件大小请参见下文
        `purpose` 相关说明。
      requestBody:
        content:
          multipart/form-data:
            schema:
              $ref: '#/components/schemas/FileUploadRequest'
        required: true
      responses:
        '200':
          description: 业务处理成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/FileObject'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
components:
  schemas:
    FileUploadRequest:
      type: object
      required:
        - file
        - purpose
      properties:
        file:
          type: string
          format: binary
          description: 要上传的文件
        purpose:
          type: string
          enum:
            - batch
            - code-interpreter
            - agent
            - voice-clone-input
          description: >-
            文件的预期用途。

            `batch`：用于批量任务处理，支持 `.jsonl` 文件格式，，单个文件大小限制为`100 MB`，文件数不超过 `1000`
            个。`Batch`指南。

            `code-interpreter`：文件上传给代码沙盒`CI`使用，支持的格式包括：`pdf、docx、doc、xls、xlsx、txt、png、jpg、jpeg、csv`，单个文件大小限制为
            `20M`，图片大小不超过`5M`，文件数不超过 `100` 个。

            `agent`：用于智能体文件上传，支持的格式包括：`pdf、docx、doc、xls、xlsx、txt、png、jpg、jpeg、csv`，单个文件大小限制为
            `20M`，图片大小不超过`5M`，文件数不超过 `1000` 个。

            `voice-clone-input`: 用于音色克隆功能示例音频文件的上传。支持的格式包括`mp3、wav`
    FileObject:
      type: object
      properties:
        id:
          type: string
          description: 文件标识符
        object:
          type: string
          enum:
            - file
        bytes:
          type: integer
          description: 文件大小（字节）
        created_at:
          type: integer
          description: 文件创建的`Unix`时间戳
        filename:
          type: string
          description: 文件名
        purpose:
          type: string
          description: 文件的预期用途
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

# 删除文件

> 永久删除指定文件及其所有关联数据。



## OpenAPI

````yaml /openapi/openapi.json delete /paas/v4/files/{file_id}
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
  /paas/v4/files/{file_id}:
    delete:
      tags:
        - 文件 API
      summary: 删除文件
      description: 永久删除指定文件及其所有关联数据。
      parameters:
        - name: file_id
          in: path
          required: true
          description: 文件唯一标识符
          schema:
            type: string
      responses:
        '200':
          description: 业务处理成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/FileDeletedResponse'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
components:
  schemas:
    FileDeletedResponse:
      allOf:
        - $ref: '#/components/schemas/BaseDeletedResponse'
        - type: object
          properties:
            object:
              type: string
              enum:
                - file
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
    BaseDeletedResponse:
      type: object
      properties:
        id:
          type: string
          description: 删除的资源`ID`
        object:
          type: string
          description: 资源类型
        deleted:
          type: boolean
          description: 是否成功删除
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

# 文件内容

> 获取文件内容。只支持 `batch` 文件类型。



## OpenAPI

````yaml /openapi/openapi.json get /paas/v4/files/{file_id}/content
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
  /paas/v4/files/{file_id}/content:
    get:
      tags:
        - 文件 API
      summary: 文件内容
      description: 获取文件内容。只支持 `batch` 文件类型。
      parameters:
        - name: file_id
          in: path
          required: true
          description: 被请求的文件的唯一标识符，用于指定要获取内容的特定文件。
          schema:
            type: string
      responses:
        '200':
          description: 请求成功，返回文件字节流。
          content:
            application/octet-stream:
              schema:
                type: string
                format: binary
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
components:
  schemas:
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

# 文件列表

> 获取已上传文件的分页列表，支持按用途和排序过滤。



## OpenAPI

````yaml /openapi/openapi.json get /paas/v4/files
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
  /paas/v4/files:
    get:
      tags:
        - 文件 API
      summary: 文件列表
      description: 获取已上传文件的分页列表，支持按用途和排序过滤。
      parameters:
        - name: after
          in: query
          description: 分页游标
          schema:
            type: string
        - name: purpose
          in: query
          description: 按用途过滤文件
          schema:
            type: string
            enum:
              - batch
              - code-interpreter
              - agent
          required: true
        - name: order
          in: query
          description: 排序方式
          schema:
            type: string
            enum:
              - created_at
        - name: limit
          in: query
          description: 每页返回的文件数量
          schema:
            type: integer
            minimum: 1
            maximum: 100
            default: 20
      responses:
        '200':
          description: 业务处理成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/FileListResponse'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
components:
  schemas:
    FileListResponse:
      type: object
      properties:
        object:
          type: string
          enum:
            - list
        data:
          type: array
          items:
            $ref: '#/components/schemas/FileObject'
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
    FileObject:
      type: object
      properties:
        id:
          type: string
          description: 文件标识符
        object:
          type: string
          enum:
            - file
        bytes:
          type: integer
          description: 文件大小（字节）
        created_at:
          type: integer
          description: 文件创建的`Unix`时间戳
        filename:
          type: string
          description: 文件名
        purpose:
          type: string
          description: 文件的预期用途
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      description: >-
        使用以下格式进行身份验证：Bearer [<your api
        key>](https://bigmodel.cn/usercenter/proj-mgmt/apikeys)

````
