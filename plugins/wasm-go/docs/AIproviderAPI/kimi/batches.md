# Kimi Batch API

- Source group: official `platform.kimi.com/docs/api` pages
- Fetched on: 2026-04-23

## Sources

- https://platform.kimi.com/docs/api/batch-create
- https://platform.kimi.com/docs/api/batch-retrieve
- https://platform.kimi.com/docs/api/batch-list
- https://platform.kimi.com/docs/api/batch-cancel

---

> ## Documentation Index
> Fetch the complete documentation index at: https://platform.kimi.com/docs/llms.txt
> Use this file to discover all available pages before exploring further.

# 创建批处理任务

> 创建一个批处理任务。需要先通过文件接口上传一个 purpose="batch" 的 JSONL 文件，然后使用返回的 file_id 创建任务。

<Note>
  **限制：**

  | 限制项         | 说明                       |
  | ----------- | ------------------------ |
  | 文件格式        | 必须为 `.jsonl` 扩展名         |
  | 文件大小        | 不能为空，最大 100MB            |
  | 组织文件配额      | 每个组织最多 1000 个 batch 类型文件 |
  | 模型一致性       | 同一批次内所有请求必须使用相同模型        |
  | `custom_id` | 文件内必须唯一                  |
  | 模型权限        | 指定的模型必须存在且用户有访问权限        |
</Note>

完整的调用示例请参考 [Batch API 指南](https://platform.kimi.com/docs/guide/use-batch-api)。


## OpenAPI

````yaml POST /v1/batches
openapi: 3.1.0
info:
  title: Moonshot AI API
  version: 1.0.0
  description: Moonshot AI / Kimi 大语言模型服务 API
servers:
  - url: https://api.moonshot.cn
    description: 生产环境
security: []
paths:
  /v1/batches:
    post:
      tags:
        - Batch
      summary: 创建批处理任务
      description: 创建一个批处理任务。需要先通过文件接口上传一个 purpose="batch" 的 JSONL 文件，然后使用返回的 file_id 创建任务。
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/BatchCreateRequest'
      responses:
        '200':
          description: 已创建的批处理任务
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/BatchObject'
        '400':
          description: 请求错误 - 参数无效
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '401':
          description: 未授权 - API 密钥无效或缺失
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '500':
          description: 服务器错误
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
      security:
        - bearerAuth: []
components:
  schemas:
    BatchCreateRequest:
      type: object
      properties:
        input_file_id:
          type: string
          description: 输入文件的 ID，必须是通过 purpose="batch" 上传的 .jsonl 文件
        endpoint:
          type: string
          description: 请求端点，目前仅支持 /v1/chat/completions
          enum:
            - /v1/chat/completions
        completion_window:
          type: string
          description: 任务处理的时间窗口，支持语义化格式如 12h、1d、3d，最小 12h，最大 7d
        metadata:
          type: object
          description: 自定义元数据，最多 16 个键值对，key 最长 64 字符，value 最长 512 字符
          additionalProperties:
            type: string
            maxLength: 512
      required:
        - input_file_id
        - endpoint
        - completion_window
    BatchObject:
      type: object
      properties:
        id:
          type: string
          description: 批处理任务的唯一标识符
        object:
          type: string
          description: 对象类型，固定为 batch
          example: batch
        endpoint:
          type: string
          description: 请求端点
        input_file_id:
          type: string
          description: 输入文件 ID
        completion_window:
          type: string
          description: 任务处理时间窗口
        status:
          type: string
          description: >-
            当前状态：validating（校验中）、failed（校验失败）、in_progress（执行中）、finalizing（准备结果中）、completed（已完成）、expired（已过期）、cancelling（取消中）、cancelled（已取消）
          enum:
            - validating
            - failed
            - in_progress
            - finalizing
            - completed
            - expired
            - cancelling
            - cancelled
        output_file_id:
          type:
            - string
            - 'null'
          description: 处理成功的结果文件 ID
        error_file_id:
          type:
            - string
            - 'null'
          description: 处理失败的错误文件 ID
        created_at:
          type: integer
          description: 创建时间（Unix 时间戳）
        in_progress_at:
          type:
            - integer
            - 'null'
          description: 开始执行时间（Unix 时间戳）
        expires_at:
          type:
            - integer
            - 'null'
          description: 过期时间（Unix 时间戳）
        finalizing_at:
          type:
            - integer
            - 'null'
          description: 开始准备结果的时间（Unix 时间戳）
        completed_at:
          type:
            - integer
            - 'null'
          description: 完成时间（Unix 时间戳）
        failed_at:
          type:
            - integer
            - 'null'
          description: 校验失败时间（Unix 时间戳）
        cancelling_at:
          type:
            - integer
            - 'null'
          description: 发起取消时间（Unix 时间戳）
        cancelled_at:
          type:
            - integer
            - 'null'
          description: 取消完成时间（Unix 时间戳）
        request_counts:
          $ref: '#/components/schemas/BatchRequestCounts'
        metadata:
          type:
            - object
            - 'null'
          description: 自定义元数据
          additionalProperties:
            type: string
      required:
        - id
        - object
        - endpoint
        - input_file_id
        - completion_window
        - status
        - created_at
        - request_counts
    ErrorResponse:
      type: object
      properties:
        error:
          type: object
          properties:
            message:
              type: string
              description: 描述错误原因的错误消息
            type:
              type: string
              description: 错误类型
            code:
              type: string
              description: 错误码
          required:
            - message
      required:
        - error
    BatchRequestCounts:
      type: object
      properties:
        completed:
          type: integer
          description: 已完成的请求数量
        failed:
          type: integer
          description: 失败的请求数量
        total:
          type: integer
          description: 总请求数量
      required:
        - completed
        - failed
        - total
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      description: >-
        Authorization 请求头需要一个 Bearer 令牌。使用 MOONSHOT_API_KEY 作为令牌。这是一个服务端密钥，请在
        [API 密钥页面](https://platform.kimi.com/console/api-keys) 生成。

````
---

> ## Documentation Index
> Fetch the complete documentation index at: https://platform.kimi.com/docs/llms.txt
> Use this file to discover all available pages before exploring further.

# 获取批处理任务详情

> 获取指定批处理任务的状态和详细信息。

完整的调用示例请参考 [Batch API 指南](https://platform.kimi.com/docs/guide/use-batch-api)。


## OpenAPI

````yaml GET /v1/batches/{batch_id}
openapi: 3.1.0
info:
  title: Moonshot AI API
  version: 1.0.0
  description: Moonshot AI / Kimi 大语言模型服务 API
servers:
  - url: https://api.moonshot.cn
    description: 生产环境
security: []
paths:
  /v1/batches/{batch_id}:
    get:
      tags:
        - Batch
      summary: 获取批处理任务详情
      description: 获取指定批处理任务的状态和详细信息。
      parameters:
        - name: batch_id
          in: path
          required: true
          description: 批处理任务的 ID
          schema:
            type: string
      responses:
        '200':
          description: 批处理任务详情
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/BatchObject'
        '401':
          description: 未授权 - API 密钥无效或缺失
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '404':
          description: 批处理任务未找到
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '500':
          description: 服务器错误
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
      security:
        - bearerAuth: []
components:
  schemas:
    BatchObject:
      type: object
      properties:
        id:
          type: string
          description: 批处理任务的唯一标识符
        object:
          type: string
          description: 对象类型，固定为 batch
          example: batch
        endpoint:
          type: string
          description: 请求端点
        input_file_id:
          type: string
          description: 输入文件 ID
        completion_window:
          type: string
          description: 任务处理时间窗口
        status:
          type: string
          description: >-
            当前状态：validating（校验中）、failed（校验失败）、in_progress（执行中）、finalizing（准备结果中）、completed（已完成）、expired（已过期）、cancelling（取消中）、cancelled（已取消）
          enum:
            - validating
            - failed
            - in_progress
            - finalizing
            - completed
            - expired
            - cancelling
            - cancelled
        output_file_id:
          type:
            - string
            - 'null'
          description: 处理成功的结果文件 ID
        error_file_id:
          type:
            - string
            - 'null'
          description: 处理失败的错误文件 ID
        created_at:
          type: integer
          description: 创建时间（Unix 时间戳）
        in_progress_at:
          type:
            - integer
            - 'null'
          description: 开始执行时间（Unix 时间戳）
        expires_at:
          type:
            - integer
            - 'null'
          description: 过期时间（Unix 时间戳）
        finalizing_at:
          type:
            - integer
            - 'null'
          description: 开始准备结果的时间（Unix 时间戳）
        completed_at:
          type:
            - integer
            - 'null'
          description: 完成时间（Unix 时间戳）
        failed_at:
          type:
            - integer
            - 'null'
          description: 校验失败时间（Unix 时间戳）
        cancelling_at:
          type:
            - integer
            - 'null'
          description: 发起取消时间（Unix 时间戳）
        cancelled_at:
          type:
            - integer
            - 'null'
          description: 取消完成时间（Unix 时间戳）
        request_counts:
          $ref: '#/components/schemas/BatchRequestCounts'
        metadata:
          type:
            - object
            - 'null'
          description: 自定义元数据
          additionalProperties:
            type: string
      required:
        - id
        - object
        - endpoint
        - input_file_id
        - completion_window
        - status
        - created_at
        - request_counts
    ErrorResponse:
      type: object
      properties:
        error:
          type: object
          properties:
            message:
              type: string
              description: 描述错误原因的错误消息
            type:
              type: string
              description: 错误类型
            code:
              type: string
              description: 错误码
          required:
            - message
      required:
        - error
    BatchRequestCounts:
      type: object
      properties:
        completed:
          type: integer
          description: 已完成的请求数量
        failed:
          type: integer
          description: 失败的请求数量
        total:
          type: integer
          description: 总请求数量
      required:
        - completed
        - failed
        - total
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      description: >-
        Authorization 请求头需要一个 Bearer 令牌。使用 MOONSHOT_API_KEY 作为令牌。这是一个服务端密钥，请在
        [API 密钥页面](https://platform.kimi.com/console/api-keys) 生成。

````
---

> ## Documentation Index
> Fetch the complete documentation index at: https://platform.kimi.com/docs/llms.txt
> Use this file to discover all available pages before exploring further.

# 列出批处理任务

> 列出当前组织的批处理任务。

完整的调用示例请参考 [Batch API 指南](https://platform.kimi.com/docs/guide/use-batch-api)。


## OpenAPI

````yaml GET /v1/batches
openapi: 3.1.0
info:
  title: Moonshot AI API
  version: 1.0.0
  description: Moonshot AI / Kimi 大语言模型服务 API
servers:
  - url: https://api.moonshot.cn
    description: 生产环境
security: []
paths:
  /v1/batches:
    get:
      tags:
        - Batch
      summary: 列出批处理任务
      description: 列出当前组织的批处理任务。
      parameters:
        - name: after
          in: query
          required: false
          description: 分页游标，传入上一页最后一个 batch 的 ID
          schema:
            type: string
        - name: limit
          in: query
          required: false
          description: 每页数量，默认 20
          schema:
            type: integer
            default: 20
      responses:
        '200':
          description: 批处理任务列表
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/BatchListResponse'
        '401':
          description: 未授权 - API 密钥无效或缺失
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '500':
          description: 服务器错误
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
      security:
        - bearerAuth: []
components:
  schemas:
    BatchListResponse:
      type: object
      properties:
        object:
          type: string
          example: list
        data:
          type: array
          items:
            $ref: '#/components/schemas/BatchObject'
        has_more:
          type: boolean
          description: 是否还有更多数据
      required:
        - object
        - data
    ErrorResponse:
      type: object
      properties:
        error:
          type: object
          properties:
            message:
              type: string
              description: 描述错误原因的错误消息
            type:
              type: string
              description: 错误类型
            code:
              type: string
              description: 错误码
          required:
            - message
      required:
        - error
    BatchObject:
      type: object
      properties:
        id:
          type: string
          description: 批处理任务的唯一标识符
        object:
          type: string
          description: 对象类型，固定为 batch
          example: batch
        endpoint:
          type: string
          description: 请求端点
        input_file_id:
          type: string
          description: 输入文件 ID
        completion_window:
          type: string
          description: 任务处理时间窗口
        status:
          type: string
          description: >-
            当前状态：validating（校验中）、failed（校验失败）、in_progress（执行中）、finalizing（准备结果中）、completed（已完成）、expired（已过期）、cancelling（取消中）、cancelled（已取消）
          enum:
            - validating
            - failed
            - in_progress
            - finalizing
            - completed
            - expired
            - cancelling
            - cancelled
        output_file_id:
          type:
            - string
            - 'null'
          description: 处理成功的结果文件 ID
        error_file_id:
          type:
            - string
            - 'null'
          description: 处理失败的错误文件 ID
        created_at:
          type: integer
          description: 创建时间（Unix 时间戳）
        in_progress_at:
          type:
            - integer
            - 'null'
          description: 开始执行时间（Unix 时间戳）
        expires_at:
          type:
            - integer
            - 'null'
          description: 过期时间（Unix 时间戳）
        finalizing_at:
          type:
            - integer
            - 'null'
          description: 开始准备结果的时间（Unix 时间戳）
        completed_at:
          type:
            - integer
            - 'null'
          description: 完成时间（Unix 时间戳）
        failed_at:
          type:
            - integer
            - 'null'
          description: 校验失败时间（Unix 时间戳）
        cancelling_at:
          type:
            - integer
            - 'null'
          description: 发起取消时间（Unix 时间戳）
        cancelled_at:
          type:
            - integer
            - 'null'
          description: 取消完成时间（Unix 时间戳）
        request_counts:
          $ref: '#/components/schemas/BatchRequestCounts'
        metadata:
          type:
            - object
            - 'null'
          description: 自定义元数据
          additionalProperties:
            type: string
      required:
        - id
        - object
        - endpoint
        - input_file_id
        - completion_window
        - status
        - created_at
        - request_counts
    BatchRequestCounts:
      type: object
      properties:
        completed:
          type: integer
          description: 已完成的请求数量
        failed:
          type: integer
          description: 失败的请求数量
        total:
          type: integer
          description: 总请求数量
      required:
        - completed
        - failed
        - total
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      description: >-
        Authorization 请求头需要一个 Bearer 令牌。使用 MOONSHOT_API_KEY 作为令牌。这是一个服务端密钥，请在
        [API 密钥页面](https://platform.kimi.com/console/api-keys) 生成。

````
---

> ## Documentation Index
> Fetch the complete documentation index at: https://platform.kimi.com/docs/llms.txt
> Use this file to discover all available pages before exploring further.

# 取消批处理任务

> 取消一个正在进行的批处理任务。取消后，任务状态将先变为 cancelling，最终变为 cancelled。仅 validating、in_progress、finalizing 状态的任务可以取消。

完整的调用示例请参考 [Batch API 指南](https://platform.kimi.com/docs/guide/use-batch-api)。


## OpenAPI

````yaml POST /v1/batches/{batch_id}/cancel
openapi: 3.1.0
info:
  title: Moonshot AI API
  version: 1.0.0
  description: Moonshot AI / Kimi 大语言模型服务 API
servers:
  - url: https://api.moonshot.cn
    description: 生产环境
security: []
paths:
  /v1/batches/{batch_id}/cancel:
    post:
      tags:
        - Batch
      summary: 取消批处理任务
      description: >-
        取消一个正在进行的批处理任务。取消后，任务状态将先变为 cancelling，最终变为 cancelled。仅
        validating、in_progress、finalizing 状态的任务可以取消。
      parameters:
        - name: batch_id
          in: path
          required: true
          description: 批处理任务的 ID
          schema:
            type: string
      responses:
        '200':
          description: 已取消的批处理任务
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/BatchObject'
        '400':
          description: 请求错误 - 任务状态不允许取消
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '401':
          description: 未授权 - API 密钥无效或缺失
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '404':
          description: 批处理任务未找到
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '500':
          description: 服务器错误
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
      security:
        - bearerAuth: []
components:
  schemas:
    BatchObject:
      type: object
      properties:
        id:
          type: string
          description: 批处理任务的唯一标识符
        object:
          type: string
          description: 对象类型，固定为 batch
          example: batch
        endpoint:
          type: string
          description: 请求端点
        input_file_id:
          type: string
          description: 输入文件 ID
        completion_window:
          type: string
          description: 任务处理时间窗口
        status:
          type: string
          description: >-
            当前状态：validating（校验中）、failed（校验失败）、in_progress（执行中）、finalizing（准备结果中）、completed（已完成）、expired（已过期）、cancelling（取消中）、cancelled（已取消）
          enum:
            - validating
            - failed
            - in_progress
            - finalizing
            - completed
            - expired
            - cancelling
            - cancelled
        output_file_id:
          type:
            - string
            - 'null'
          description: 处理成功的结果文件 ID
        error_file_id:
          type:
            - string
            - 'null'
          description: 处理失败的错误文件 ID
        created_at:
          type: integer
          description: 创建时间（Unix 时间戳）
        in_progress_at:
          type:
            - integer
            - 'null'
          description: 开始执行时间（Unix 时间戳）
        expires_at:
          type:
            - integer
            - 'null'
          description: 过期时间（Unix 时间戳）
        finalizing_at:
          type:
            - integer
            - 'null'
          description: 开始准备结果的时间（Unix 时间戳）
        completed_at:
          type:
            - integer
            - 'null'
          description: 完成时间（Unix 时间戳）
        failed_at:
          type:
            - integer
            - 'null'
          description: 校验失败时间（Unix 时间戳）
        cancelling_at:
          type:
            - integer
            - 'null'
          description: 发起取消时间（Unix 时间戳）
        cancelled_at:
          type:
            - integer
            - 'null'
          description: 取消完成时间（Unix 时间戳）
        request_counts:
          $ref: '#/components/schemas/BatchRequestCounts'
        metadata:
          type:
            - object
            - 'null'
          description: 自定义元数据
          additionalProperties:
            type: string
      required:
        - id
        - object
        - endpoint
        - input_file_id
        - completion_window
        - status
        - created_at
        - request_counts
    ErrorResponse:
      type: object
      properties:
        error:
          type: object
          properties:
            message:
              type: string
              description: 描述错误原因的错误消息
            type:
              type: string
              description: 错误类型
            code:
              type: string
              description: 错误码
          required:
            - message
      required:
        - error
    BatchRequestCounts:
      type: object
      properties:
        completed:
          type: integer
          description: 已完成的请求数量
        failed:
          type: integer
          description: 失败的请求数量
        total:
          type: integer
          description: 总请求数量
      required:
        - completed
        - failed
        - total
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      description: >-
        Authorization 请求头需要一个 Bearer 令牌。使用 MOONSHOT_API_KEY 作为令牌。这是一个服务端密钥，请在
        [API 密钥页面](https://platform.kimi.com/console/api-keys) 生成。

````
