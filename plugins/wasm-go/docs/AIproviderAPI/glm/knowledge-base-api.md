# GLM 知识库 API

- Category: 知识库-api
- Source root: https://docs.bigmodel.cn/api-reference/知识库-api
- Pages: 15
- Fetched on: 2026-04-23
- Fetch failures: 0

## Sources

- https://docs.bigmodel.cn/api-reference/知识库-api/上传url文档
- https://docs.bigmodel.cn/api-reference/知识库-api/上传文件文档
- https://docs.bigmodel.cn/api-reference/知识库-api/全模态知识库检索
- https://docs.bigmodel.cn/api-reference/知识库-api/创建知识库
- https://docs.bigmodel.cn/api-reference/知识库-api/删除文档
- https://docs.bigmodel.cn/api-reference/知识库-api/删除知识库
- https://docs.bigmodel.cn/api-reference/知识库-api/文档列表
- https://docs.bigmodel.cn/api-reference/知识库-api/文档详情
- https://docs.bigmodel.cn/api-reference/知识库-api/知识库使用量
- https://docs.bigmodel.cn/api-reference/知识库-api/知识库列表
- https://docs.bigmodel.cn/api-reference/知识库-api/知识库检索
- https://docs.bigmodel.cn/api-reference/知识库-api/知识库详情
- https://docs.bigmodel.cn/api-reference/知识库-api/编辑知识库
- https://docs.bigmodel.cn/api-reference/知识库-api/解析文档图片
- https://docs.bigmodel.cn/api-reference/知识库-api/重新向量化

---

> ## Documentation Index
> Fetch the complete documentation index at: https://docs.bigmodel.cn/llms.txt
> Use this file to discover all available pages before exploring further.

# 上传URL文档

> 上传`URL`类型的文档或网页作为内容填充知识库。



## OpenAPI

````yaml /openapi/openapi.json post /llm-application/open/document/upload_url
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
  /llm-application/open/document/upload_url:
    post:
      tags:
        - 知识库 API
      summary: 上传URL文档
      description: 上传`URL`类型的文档或网页作为内容填充知识库。
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/UploadUrlKnowledgeRequest'
        required: true
      responses:
        '200':
          description: 请求成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/UploadUrlKnowledgeResponse'
              example:
                data:
                  successInfos:
                    - documentId: '122121212'
                      url: xxx.com
                    - documentId: '12121212121'
                      url: xxx.com
                  failedInfos:
                    - url: xxx.com
                      failReason: 不支持的文档类型
                code: 200
                message: 请求成功
                timestamp: 1689649504996
        default:
          description: 请求失败
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LlmApplicationError'
components:
  schemas:
    UploadUrlKnowledgeRequest:
      type: object
      properties:
        upload_detail:
          type: array
          items:
            $ref: '#/components/schemas/UrlData'
          description: url列表
        knowledge_id:
          type: string
          description: 知识库id
      required:
        - upload_detail
        - knowledge_id
    UploadUrlKnowledgeResponse:
      type: object
      properties:
        data:
          type: object
          properties:
            successInfos:
              type: array
              items:
                type: object
                properties:
                  documentId:
                    type: string
                    description: 文档id
                  url:
                    type: string
                    description: url
            failedInfos:
              type: array
              items:
                type: object
                properties:
                  url:
                    type: string
                    description: url
                  failReason:
                    type: string
                    description: 失败原因
        code:
          type: integer
          description: 状态码
        message:
          type: string
          description: 返回信息
        timestamp:
          type: integer
          description: 时间戳
    LlmApplicationError:
      type: object
      properties:
        code:
          type: integer
        message:
          type: string
    UrlData:
      type: object
      properties:
        url:
          type: string
          description: url
        knowledge_type:
          type: integer
          description: 文档切片类型
        custom_separator:
          type: array
          items:
            type: string
          description: |
            自定义切片分隔符，仅 knowledge_type=5 时生效，默认 
        sentence_size:
          type: integer
          description: 自定义切片字数，仅 knowledge_type=5 时生效，20-2000，默认300
        callback_url:
          type: string
          description: 回调地址
        callback_header:
          type: object
          description: 回调header k-v
      required:
        - url
        - knowledge_type
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

# 上传文件文档

> 向指定知识库上传文件类型文档，支持多种切片方式和回调。



## OpenAPI

````yaml /openapi/openapi.json post /llm-application/open/document/upload_document/{id}
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
  /llm-application/open/document/upload_document/{id}:
    post:
      tags:
        - 知识库 API
      summary: 上传文件文档
      description: 向指定知识库上传文件类型文档，支持多种切片方式和回调。
      parameters:
        - name: id
          in: path
          required: true
          description: 知识库id
          schema:
            type: string
      requestBody:
        content:
          multipart/form-data:
            schema:
              $ref: '#/components/schemas/DocumentUploadRequest'
        required: true
      responses:
        '200':
          description: 业务处理成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/DocumentUploadResponse'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LlmApplicationError'
components:
  schemas:
    DocumentUploadRequest:
      type: object
      properties:
        files:
          type: string
          format: binary
          description: 文件
        knowledge_type:
          type: integer
          description: |-
            文档类型，不传则动态解析。
            1: 按标题段落切：支持txt,doc,pdf,url,docx,ppt,pptx,md
            2: 按问答对切片：支持txt,doc,pdf,url,docx,ppt,pptx,md
            3: 按行切片：支持xls,xlsx,csv
            5: 自定义切片：支持txt,doc,pdf,url,docx,ppt,pptx,md
            6: 按页切片：支持pdf,ppt,pptx
            7: 按单个切片：支持xls,xlsx,csv
        custom_separator:
          type: array
          items:
            type: string
          description: |
            自定义切片规则，knowledge_type=5时传，默认
        sentence_size:
          type: integer
          description: 自定义切片大小，knowledge_type=5时传，20-2000，默认300
        parse_image:
          type: boolean
          description: 是否解析图片，默认不解析
        callback_url:
          type: string
          description: 回调地址
        callback_header:
          type: object
          description: 回调时header携带的k-v
        word_num_limit:
          type: string
          description: 文档字数上限，必须为数字
        req_id:
          type: string
          description: 请求唯一id
      required:
        - files
    DocumentUploadResponse:
      type: object
      properties:
        data:
          type: object
          properties:
            successInfos:
              type: array
              items:
                $ref: '#/components/schemas/DocumentUploadSuccessInfo'
              description: 上传成功的文件
            failedInfos:
              type: array
              items:
                $ref: '#/components/schemas/DocumentUploadFailedInfo'
              description: 上传失败的文件
        code:
          type: integer
          description: 响应码，200为成功
        message:
          type: string
          description: 响应信息
        timestamp:
          type: integer
          description: 响应时间戳
    LlmApplicationError:
      type: object
      properties:
        code:
          type: integer
        message:
          type: string
    DocumentUploadSuccessInfo:
      type: object
      properties:
        documentId:
          type: string
          description: 文档ID
        fileName:
          type: string
          description: 文件名
    DocumentUploadFailedInfo:
      type: object
      properties:
        fileName:
          type: string
          description: 文件名
        failReason:
          type: string
          description: 失败原因
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

# 全模态知识库检索

> 用于检索全模态知识库，支持文本、图片、视频等多模态输入检索，支持向量检索、关键词检索、混合检索，支持查询重写、重排、QA干预等高级功能。



## OpenAPI

````yaml /openapi/openapi.json post /zrag/retrieval/retrieve
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
  /zrag/retrieval/retrieve:
    post:
      tags:
        - 知识库 API
      summary: 全模态知识库检索
      description: 用于检索全模态知识库，支持文本、图片、视频等多模态输入检索，支持向量检索、关键词检索、混合检索，支持查询重写、重排、QA干预等高级功能。
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ZragRetrieveRequest'
            example:
              multimodal: true
              knows:
                - id: '1234567890'
              query: 介绍一下小智
              multimodal_parts:
                - type: image_url
                  url: https://example.com/image.png
              top_k: 8
              top_n: 10
              recall_method: mixed
              recall_ratio: 0.8
              messages:
                - role: user
                  content: 你是谁
                - role: assistant
                  content: 我是小智，一个智能助手
        required: true
      responses:
        '200':
          description: 业务处理成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ZragRetrieveResponse'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LlmApplicationError'
components:
  schemas:
    ZragRetrieveRequest:
      type: object
      properties:
        multimodal:
          type: boolean
          default: true
          description: 是否走多模态路径检索，默认值为true
        knows:
          type: array
          description: 查询的知识库列表
          items:
            type: object
            properties:
              id:
                type: string
                description: 知识库ID
              doc_ids:
                type: array
                description: 知识库下的文档ID列表
                items:
                  type: string
            required:
              - id
        query:
          type: string
          description: 文本查询内容，与多模态查询内容必须传入其中之一
        multimodal_parts:
          type: array
          description: 多模态查询内容，与文本查询内容必须传入其中之一
          items:
            type: object
            properties:
              type:
                type: string
                enum:
                  - image_url
                description: 仅支持图片类型：image_url
              url:
                type: string
                description: 图片URL链接
            required:
              - type
              - url
        top_k:
          type: integer
          default: 8
          description: 最终召回数量，默认为8
        top_n:
          type: integer
          default: 10
          description: 初始召回数量，默认为10
        recall_method:
          type: string
          enum:
            - embedding
            - keyword
            - mixed
          default: mixed
          description: 文本检索方式：embedding（向量检索）、keyword（关键词检索）、mixed（混合检索）
        recall_ratio:
          type: number
          default: 0.8
          description: 混合检索中向量检索的权重，取值范围0~1
        enable_rerank:
          type: boolean
          default: false
          description: 是否开启重排，默认不开启
        enable_rewrite:
          type: boolean
          default: false
          description: 是否开启查询重写，可配合messages参数实现多轮对话改写，默认不开启
        enable_expansion:
          type: boolean
          default: false
          description: 是否开启扩召，默认不开启
        similarity_threshold:
          type: number
          default: 0.2
          description: 相似度阈值，低于该阈值的切片会被过滤
        messages:
          type: array
          description: 当前对话消息列表，用于多轮对话改写
          items:
            type: object
            properties:
              role:
                type: string
                enum:
                  - user
                  - assistant
              content:
                type: string
                description: 消息内容，仅支持文本
            required:
              - role
              - content
        search_filters:
          type: object
          description: 过滤条件
          properties:
            index_types:
              type: array
              description: 索引列表
              items:
                type: object
                properties:
                  know_id:
                    type: string
                    description: 知识库ID
                  index_type_id:
                    type: integer
                    description: 索引ID
                required:
                  - know_id
                  - index_type_id
            tags:
              type: array
              description: 标签列表
              items:
                type: object
                properties:
                  tag_id:
                    type: string
                    description: 标签ID
                  value_type:
                    type: string
                    enum:
                      - fixed
                      - ref
                    description: 固定值：fixed，引用变量：ref
                  filter_type:
                    type: integer
                    enum:
                      - 1
                      - 2
                      - 3
                      - 4
                    description: '过滤类型：1: >=，2: <=，3: 包含，4: 不包含'
                  filter_value:
                    type: string
                    description: 日期/文本/引用值
                  multiple_value:
                    type: array
                    description: 选项值列表
                    items:
                      type: string
                required:
                  - tag_id
                  - value_type
                  - filter_type
                  - filter_value
                  - multiple_value
            qa_intervention:
              type: object
              description: QA干预配置
              properties:
                qa_similarity_threshold:
                  type: number
                  default: 0.6
                  description: QA干预相似度阈值
                qa_intervention_ids:
                  type: array
                  description: QA知识库ID列表
                  items:
                    type: string
              required:
                - qa_similarity_threshold
                - qa_intervention_ids
      required:
        - knows
    ZragRetrieveResponse:
      type: object
      properties:
        data:
          type: object
          properties:
            contents:
              type: array
              description: 检索结果列表
              items:
                type: object
                properties:
                  id:
                    type: string
                    description: 切片ID（UUID）
                  know_id:
                    type: string
                    description: 知识库ID
                  doc_id:
                    type: string
                    description: 文档ID
                  text:
                    type: string
                    description: 文本内容
                  medias:
                    type: array
                    description: 文本中的媒体文件
                    items:
                      type: object
                      properties:
                        id:
                          type: string
                          description: 图片ID
                        url:
                          type: string
                          description: 图片URL
                        description:
                          type: string
                          description: 图片描述
                  image_url:
                    type: object
                    description: 图像URL
                    properties:
                      url:
                        type: string
                        description: URL
                  video_url:
                    type: object
                    description: 视频URL
                    properties:
                      url:
                        type: string
                        description: URL
                  index:
                    type: integer
                    description: 召回位次
                  score:
                    type: number
                    description: 召回分数
                  rerank_index:
                    type: integer
                    description: 重排位次
                  rerank_score:
                    type: number
                    description: 重排分数
                  metadata:
                    type: object
                    description: 元数据
                    properties:
                      doc_type:
                        type: string
                        description: 文档类型，如 pdf、docx、jpeg、png、mp4、mp3 等
                      doc_name:
                        type: string
                        description: 文档名称
                      doc_url:
                        type: string
                        description: 文档URL
                      index:
                        type: integer
                        description: 切片下标
                      page_index:
                        type: integer
                        description: 文档页码
                      clip_index:
                        type: integer
                        description: 视频切片下标
                      start_time:
                        type: integer
                        description: 首帧时间戳
                      end_time:
                        type: integer
                        description: 尾帧时间戳
                      duration:
                        type: integer
                        description: 视频切片时长
                      frames:
                        type: array
                        description: 关键帧列表
                        items:
                          type: string
            rewritten_query:
              type: object
              description: 查询重写结果
              properties:
                original_query:
                  type: string
                  description: 原始查询
                multi_queries:
                  type: array
                  description: 备选查询列表
                  items:
                    type: string
            elapsed_ms:
              type: integer
              description: 请求耗时（毫秒）
            total_tokens:
              type: integer
              description: 消耗的token数量
            request_id:
              type: string
              description: 请求ID
        code:
          type: integer
          description: 错误码，200为成功
        message:
          description: 错误信息
          type: string
    LlmApplicationError:
      type: object
      properties:
        code:
          type: integer
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

# 创建知识库

> 用于创建个人知识库，支持绑定向量化模型、设置名称、描述、背景色和图标。



## OpenAPI

````yaml /openapi/openapi.json post /llm-application/open/knowledge
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
  /llm-application/open/knowledge:
    post:
      tags:
        - 知识库 API
      summary: 创建知识库
      description: 用于创建个人知识库，支持绑定向量化模型、设置名称、描述、背景色和图标。
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/KnowledgeCreateRequest'
        required: true
      responses:
        '200':
          description: 业务处理成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/KnowledgeCreateResponse'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LlmApplicationError'
components:
  schemas:
    KnowledgeCreateRequest:
      type: object
      properties:
        embedding_id:
          type: integer
          enum:
            - 3
            - 11
            - 12
          description: |-
            知识库绑定的向量化模型ID。可选值：
            - 3: Embedding-2
            - 11: Embedding-3
            - 12: Embedding-3-pro
        embedding_model:
          type: string
          enum:
            - Embedding-2
            - Embedding-3
            - Embedding-3-pro
          description: |-
            知识库绑定的向量化模型code。可选值：
            - Embedding-2
            - Embedding-3
            - Embedding-3-pro
        contextual:
          type: integer
          enum:
            - 0
            - 1
          description: 是否开启上下文增强
        name:
          type: string
          description: 知识库名称
        description:
          type: string
          description: 知识库描述
        background:
          type: string
          description: 背景颜色，可选：blue, red, orange, purple, sky, green, yellow，默认blue
        icon:
          type: string
          description: 知识库图标，可选：question, book, seal, wrench, tag, horn, house，默认question
      required:
        - embedding_id
        - name
    KnowledgeCreateResponse:
      type: object
      properties:
        data:
          type: object
          properties:
            id:
              type: string
              description: 生成的知识库唯一id
        code:
          type: integer
          description: 响应码，200为成功
        message:
          type: string
          description: 响应信息
        timestamp:
          type: integer
          description: 响应时间戳
    LlmApplicationError:
      type: object
      properties:
        code:
          type: integer
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

# 删除文档

> 根据文档`ID`删除文档。



## OpenAPI

````yaml /openapi/openapi.json delete /llm-application/open/document/{id}
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
  /llm-application/open/document/{id}:
    delete:
      tags:
        - 知识库 API
      summary: 删除文档
      description: 根据文档`ID`删除文档。
      parameters:
        - name: id
          in: path
          required: true
          description: 文档ID
          schema:
            type: string
      responses:
        '200':
          description: 请求成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/DeleteKnowledgeResponse'
              example:
                code: 200
                message: 请求成功
                timestamp: 1689649504996
        default:
          description: 请求失败
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LlmApplicationError'
components:
  schemas:
    DeleteKnowledgeResponse:
      $ref: '#/components/schemas/BaseApiResponse'
    LlmApplicationError:
      type: object
      properties:
        code:
          type: integer
        message:
          type: string
    BaseApiResponse:
      type: object
      properties:
        code:
          type: integer
          description: 响应码，`200`为成功
        message:
          type: string
          description: 响应信息
        timestamp:
          type: integer
          description: 响应时间戳
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

# 删除知识库

> 根据知识库`ID`删除个人知识库。



## OpenAPI

````yaml /openapi/openapi.json delete /llm-application/open/knowledge/{id}
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
  /llm-application/open/knowledge/{id}:
    delete:
      tags:
        - 知识库 API
      summary: 删除知识库
      description: 根据知识库`ID`删除个人知识库。
      parameters:
        - name: id
          in: path
          required: true
          description: 知识库id
          schema:
            type: string
      responses:
        '200':
          description: 业务处理成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/KnowledgeDeleteResponse'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LlmApplicationError'
components:
  schemas:
    KnowledgeDeleteResponse:
      $ref: '#/components/schemas/BaseApiResponse'
    LlmApplicationError:
      type: object
      properties:
        code:
          type: integer
        message:
          type: string
    BaseApiResponse:
      type: object
      properties:
        code:
          type: integer
          description: 响应码，`200`为成功
        message:
          type: string
          description: 响应信息
        timestamp:
          type: integer
          description: 响应时间戳
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

# 文档列表

> 获取指定知识库下的文档列表。



## OpenAPI

````yaml /openapi/openapi.json get /llm-application/open/document
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
  /llm-application/open/document:
    get:
      tags:
        - 知识库 API
      summary: 文档列表
      description: 获取指定知识库下的文档列表。
      parameters:
        - name: knowledge_id
          in: query
          required: true
          description: 知识库id
          schema:
            type: string
        - name: page
          in: query
          required: false
          description: 页码，默认1
          schema:
            type: integer
            default: 1
        - name: size
          in: query
          required: false
          description: 每页数量，默认10
          schema:
            type: integer
            default: 10
        - name: word
          in: query
          required: false
          description: 文档名称
          schema:
            type: string
      responses:
        '200':
          description: 请求成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/KnowledgeDocumentListResponse'
              example:
                data:
                  list:
                    - id: '12312121212'
                      knowledge_type: 1
                      custom_separator:
                        - |+

                      sentence_size: 300
                      length: 0
                      word_num: 100
                      name: ''
                      url: ''
                      embedding_stat: 0
                      failInfo:
                        embedding_code: 10002
                        embedding_msg: 字数超出限制
                  total: 1
                code: 200
                message: 请求成功
                timestamp: 1689649504996
        default:
          description: 请求失败
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LlmApplicationError'
components:
  schemas:
    KnowledgeDocumentListResponse:
      type: object
      properties:
        data:
          type: object
          properties:
            list:
              type: array
              items:
                $ref: '#/components/schemas/KnowledgeDocumentItem'
            total:
              type: integer
              description: 总数
        code:
          type: integer
          description: 状态码
        message:
          type: string
          description: 返回信息
        timestamp:
          type: integer
          description: 时间戳
    LlmApplicationError:
      type: object
      properties:
        code:
          type: integer
        message:
          type: string
    KnowledgeDocumentItem:
      type: object
      properties:
        id:
          type: string
          description: 文档id
        knowledge_type:
          type: integer
          description: 文档切片类型
        custom_separator:
          type: array
          items:
            type: string
          description: 自定义分隔符
        sentence_size:
          type: integer
          description: 切片字数
        length:
          type: integer
          description: 文档长度
        word_num:
          type: integer
          description: 字数
        name:
          type: string
          description: 文档名称
        url:
          type: string
          description: 文档URL
        embedding_stat:
          type: integer
          description: 向量化状态
        failInfo:
          $ref: '#/components/schemas/KnowledgeDocumentFailInfo'
    KnowledgeDocumentFailInfo:
      type: object
      properties:
        embedding_code:
          type: integer
          description: 向量化失败状态码
        embedding_msg:
          type: string
          description: 向量化失败信息
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

# 文档详情

> 根据文档`ID`获取文档详情。



## OpenAPI

````yaml /openapi/openapi.json get /llm-application/open/document/{id}
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
  /llm-application/open/document/{id}:
    get:
      tags:
        - 知识库 API
      summary: 文档详情
      description: 根据文档`ID`获取文档详情。
      parameters:
        - name: id
          in: path
          required: true
          description: 文档id
          schema:
            type: string
      responses:
        '200':
          description: 请求成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/KnowledgeDocumentDetailResponse'
              example:
                data:
                  id: '12121212121212'
                  knowledge_type: 1
                  custom_separator:
                    - |+

                  sentence_size: 300
                  length: 0
                  word_num: 100
                  name: ''
                  url: ''
                  embedding_stat: 0
                  failInfo:
                    embedding_code: 10002
                    embedding_msg: 字数超出限制
                code: 200
                message: 请求成功
                timestamp: 1689649504996
        default:
          description: 请求失败
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LlmApplicationError'
components:
  schemas:
    KnowledgeDocumentDetailResponse:
      type: object
      properties:
        data:
          $ref: '#/components/schemas/KnowledgeDocumentItem'
        code:
          type: integer
          description: 状态码
        message:
          type: string
          description: 返回信息
        timestamp:
          type: integer
          description: 时间戳
    LlmApplicationError:
      type: object
      properties:
        code:
          type: integer
        message:
          type: string
    KnowledgeDocumentItem:
      type: object
      properties:
        id:
          type: string
          description: 文档id
        knowledge_type:
          type: integer
          description: 文档切片类型
        custom_separator:
          type: array
          items:
            type: string
          description: 自定义分隔符
        sentence_size:
          type: integer
          description: 切片字数
        length:
          type: integer
          description: 文档长度
        word_num:
          type: integer
          description: 字数
        name:
          type: string
          description: 文档名称
        url:
          type: string
          description: 文档URL
        embedding_stat:
          type: integer
          description: 向量化状态
        failInfo:
          $ref: '#/components/schemas/KnowledgeDocumentFailInfo'
    KnowledgeDocumentFailInfo:
      type: object
      properties:
        embedding_code:
          type: integer
          description: 向量化失败状态码
        embedding_msg:
          type: string
          description: 向量化失败信息
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

# 知识库使用量

> 获取个人知识库的使用量详情，包括字数和字节数。



## OpenAPI

````yaml /openapi/openapi.json get /llm-application/open/knowledge/capacity
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
  /llm-application/open/knowledge/capacity:
    get:
      tags:
        - 知识库 API
      summary: 知识库使用量
      description: 获取个人知识库的使用量详情，包括字数和字节数。
      responses:
        '200':
          description: 业务处理成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/KnowledgeCapacityResponse'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LlmApplicationError'
components:
  schemas:
    KnowledgeCapacityResponse:
      type: object
      properties:
        data:
          type: object
          properties:
            used:
              $ref: '#/components/schemas/KnowledgeCapacityItem'
            total:
              $ref: '#/components/schemas/KnowledgeCapacityItem'
        code:
          type: integer
          description: 响应码，200为成功
        message:
          type: string
          description: 响应信息
        timestamp:
          type: integer
          description: 响应时间戳
    LlmApplicationError:
      type: object
      properties:
        code:
          type: integer
        message:
          type: string
    KnowledgeCapacityItem:
      type: object
      properties:
        word_num:
          type: integer
          description: 字数
        length:
          type: integer
          description: 字节数
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

# 知识库列表

> 获取个人知识库列表，支持分页。



## OpenAPI

````yaml /openapi/openapi.json get /llm-application/open/knowledge
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
  /llm-application/open/knowledge:
    get:
      tags:
        - 知识库 API
      summary: 知识库列表
      description: 获取个人知识库列表，支持分页。
      parameters:
        - name: page
          in: query
          description: 页码，默认1
          schema:
            type: integer
            default: 1
        - name: size
          in: query
          description: 每页数量，默认10
          schema:
            type: integer
            default: 10
      responses:
        '200':
          description: 业务处理成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/KnowledgeListResponse'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LlmApplicationError'
components:
  schemas:
    KnowledgeListResponse:
      type: object
      properties:
        data:
          type: object
          properties:
            list:
              type: array
              items:
                $ref: '#/components/schemas/KnowledgeListItem'
              description: 知识库列表
            total:
              type: integer
              description: 知识库总数量
        code:
          type: integer
          description: 响应码，200为成功
        message:
          type: string
          description: 响应信息
        timestamp:
          type: integer
          description: 响应时间戳
    LlmApplicationError:
      type: object
      properties:
        code:
          type: integer
        message:
          type: string
    KnowledgeListItem:
      type: object
      properties:
        id:
          type: string
          description: 知识库id
        embedding_id:
          type: integer
          description: 向量化模型id
        name:
          type: string
          description: 知识库名称
        description:
          type: string
          description: 知识库描述
        contextual:
          type: integer
          description: 是否开启上下文增强
        background:
          type: string
          description: 背景颜色
        icon:
          type: string
          description: 知识库图标
        document_size:
          type: integer
          description: 文档数量
        length:
          type: integer
          description: 分词后总长度
        word_num:
          type: integer
          description: 总字数
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

# 知识库检索

> 用于检索个人知识库，支持向量检索、关键词检索、混合检索，支持自定义重排模型。



## OpenAPI

````yaml /openapi/openapi.json post /llm-application/open/knowledge/retrieve
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
  /llm-application/open/knowledge/retrieve:
    post:
      tags:
        - 知识库 API
      summary: 知识库检索
      description: 用于检索个人知识库，支持向量检索、关键词检索、混合检索，支持自定义重排模型。
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/KnowledgeRetrieveRequest'
        required: true
      responses:
        '200':
          description: 业务处理成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/KnowledgeRetrieveResponse'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LlmApplicationError'
components:
  schemas:
    KnowledgeRetrieveRequest:
      type: object
      properties:
        request_id:
          type: string
          description: 请求唯一id，用于定位日志
        query:
          type: string
          description: 查询内容，限制在1000字以内
        knowledge_ids:
          type: array
          description: 知识库ID列表
          items:
            type: string
        document_ids:
          type: array
          description: 文档ID列表
          items:
            type: string
        top_k:
          type: integer
          description: 最终召回数量，取值范围为[1~20]，默认为8
        top_n:
          type: integer
          description: 初始召回数量，取值范围为[1~100]，默认为10
        recall_method:
          type: string
          enum:
            - embedding
            - keyword
            - mixed
          default: mixed
          description: |-
            检索类型 
            - embedding: 向量化检索
            - keyword:关键词检索
            - mixed: 混合检索（默认）
        recall_ratio:
          type: integer
          default: 80
          description: 混合检索中向量检索的权重，取值范围(0~100)，默认为80
        rerank_status:
          type: integer
          enum:
            - 0
            - 1
          description: '是否开启重排，0: 不开启，1: 开启，默认不开启'
        rerank_model:
          type: string
          enum:
            - rerank
            - rerank-pro
          description: 重排模型，支持rerank、rerank-pro
        fractional_threshold:
          type: number
          description: 相似度阈值，低于该阈值的切片会被过滤，取值范围为(0~1)
      required:
        - query
        - knowledge_ids
    KnowledgeRetrieveResponse:
      type: object
      properties:
        data:
          type: array
          description: 检索结果列表
          items:
            type: object
            properties:
              text:
                type: string
                description: 切片内容
              score:
                type: number
                description: 相似度分数
              metadata:
                type: object
                description: 切片元数据
                properties:
                  _id:
                    type: string
                    description: 切片ID
                  knowledge_id:
                    type: string
                    description: 知识库ID
                  doc_id:
                    type: string
                    description: 文档ID
                  doc_name:
                    type: string
                    description: 文档名称
                  doc_url:
                    type: string
                    description: 文档URL
                  contextual_text:
                    type: string
                    description: 上下文增强内容，不开启上下文增强则为空
        code:
          type: integer
          description: 响应码，200为成功
        message:
          type: string
          description: 响应信息
        timestamp:
          type: integer
          description: 响应时间戳
    LlmApplicationError:
      type: object
      properties:
        code:
          type: integer
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

# 知识库详情

> 根据知识库`ID`获取个人知识库详情。



## OpenAPI

````yaml /openapi/openapi.json get /llm-application/open/knowledge/{id}
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
  /llm-application/open/knowledge/{id}:
    get:
      tags:
        - 知识库 API
      summary: 知识库详情
      description: 根据知识库`ID`获取个人知识库详情。
      parameters:
        - name: id
          in: path
          required: true
          description: 知识库id
          schema:
            type: string
      responses:
        '200':
          description: 业务处理成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/KnowledgeDetailResponse'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LlmApplicationError'
components:
  schemas:
    KnowledgeDetailResponse:
      type: object
      properties:
        data:
          $ref: '#/components/schemas/KnowledgeListItem'
        code:
          type: integer
          description: 响应码，200为成功
        message:
          type: string
          description: 响应信息
        timestamp:
          type: integer
          description: 响应时间戳
    LlmApplicationError:
      type: object
      properties:
        code:
          type: integer
        message:
          type: string
    KnowledgeListItem:
      type: object
      properties:
        id:
          type: string
          description: 知识库id
        embedding_id:
          type: integer
          description: 向量化模型id
        name:
          type: string
          description: 知识库名称
        description:
          type: string
          description: 知识库描述
        contextual:
          type: integer
          description: 是否开启上下文增强
        background:
          type: string
          description: 背景颜色
        icon:
          type: string
          description: 知识库图标
        document_size:
          type: integer
          description: 文档数量
        length:
          type: integer
          description: 分词后总长度
        word_num:
          type: integer
          description: 总字数
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

# 编辑知识库

> 用于编辑已经创建好的个人知识库，仅传入要修改的字段。



## OpenAPI

````yaml /openapi/openapi.json put /llm-application/open/knowledge/{id}
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
  /llm-application/open/knowledge/{id}:
    put:
      tags:
        - 知识库 API
      summary: 编辑知识库
      description: 用于编辑已经创建好的个人知识库，仅传入要修改的字段。
      parameters:
        - name: id
          in: path
          required: true
          description: 知识库id
          schema:
            type: string
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/KnowledgeEditRequest'
        required: true
      responses:
        '200':
          description: 业务处理成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/KnowledgeEditResponse'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LlmApplicationError'
components:
  schemas:
    KnowledgeEditRequest:
      type: object
      properties:
        embedding_id:
          type: integer
          enum:
            - 3
            - 11
            - 12
          description: |-
            知识库绑定的向量化模型ID。可选值：
            - 3: Embedding-2
            - 11: Embedding-3
            - 12: Embedding-3-pro
        embedding_model:
          type: string
          enum:
            - Embedding-2
            - Embedding-3
            - Embedding-3-pro
          description: |-
            知识库绑定的向量化模型code。可选值：
            - Embedding-2
            - Embedding-3
            - Embedding-3-pro
        contextual:
          type: integer
          enum:
            - 0
            - 1
          description: 是否开启上下文增强
        name:
          type: string
          description: 知识库名称
        description:
          type: string
          description: 知识库描述
        background:
          type: string
          description: 背景颜色，可选：blue, red, orange, purple, sky, green, yellow
        icon:
          type: string
          description: 知识库图标，可选：question, book, seal, wrench, tag, horn, house
        callback_url:
          type: string
          description: 回调地址（若修改向量模型，则需要重新构建知识，由客户决定是否单独配置回调）
        callback_header:
          type: object
          description: 回调时header携带的k-v（若修改向量模型，则需要重新构建知识，由客户决定是否单独配置回调）
    KnowledgeEditResponse:
      type: object
      properties:
        code:
          type: integer
          description: 响应码，200为成功
        message:
          type: string
          description: 响应信息
        timestamp:
          type: integer
          description: 响应时间戳
    LlmApplicationError:
      type: object
      properties:
        code:
          type: integer
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

# 解析文档图片

> 用于获取文件下解析到的图片序号和图片链接映射关系。



## OpenAPI

````yaml /openapi/openapi.json post /llm-application/open/document/slice/image_list/{id}
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
  /llm-application/open/document/slice/image_list/{id}:
    post:
      tags:
        - 知识库 API
      summary: 解析文档图片
      description: 用于获取文件下解析到的图片序号和图片链接映射关系。
      parameters:
        - name: id
          in: path
          required: true
          description: 文档ID
          schema:
            type: string
      responses:
        '200':
          description: 请求成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/KnowledgeImageListResponse'
              example:
                data:
                  images:
                    - text: 【示意图序号_1829473032620613632_103】
                      cos_url: >-
                        https://cdn.bigmodel.cn/knowledge_pdf_image/de7163a7-c67f-4e33-8810-97a6d5a83378.png
                code: 200
                message: 请求成功
                timestamp: 1725070689634
        default:
          description: 请求失败
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LlmApplicationError'
components:
  schemas:
    KnowledgeImageListResponse:
      type: object
      properties:
        data:
          type: object
          properties:
            images:
              type: array
              items:
                type: object
                properties:
                  text:
                    type: string
                    description: 图片序号文本
                  cos_url:
                    type: string
                    description: 图片链接
        code:
          type: integer
          description: 状态码
        message:
          type: string
          description: 返回信息
        timestamp:
          type: integer
          description: 时间戳
    LlmApplicationError:
      type: object
      properties:
        code:
          type: integer
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

# 重新向量化

> 用于重新向量化文档（重试等操作）。同步返回成功表示调用成功，向量化完成后调用`callback_url`进行通知，也可调用知识详情接口获取结果。多用于`url`知识场景。



## OpenAPI

````yaml /openapi/openapi.json post /llm-application/open/document/embedding/{id}
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
  /llm-application/open/document/embedding/{id}:
    post:
      tags:
        - 知识库 API
      summary: 重新向量化
      description: >-
        用于重新向量化文档（重试等操作）。同步返回成功表示调用成功，向量化完成后调用`callback_url`进行通知，也可调用知识详情接口获取结果。多用于`url`知识场景。
      parameters:
        - name: id
          in: path
          required: true
          description: 文档ID
          schema:
            type: string
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ReEmbeddingRequest'
        required: false
      responses:
        '200':
          description: 请求成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ReEmbeddingResponse'
              example:
                code: 200
                message: 请求成功
                timestamp: 1689649504996
        default:
          description: 请求失败
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/LlmApplicationError'
components:
  schemas:
    ReEmbeddingRequest:
      type: object
      properties:
        callback_url:
          type: string
          description: 回调地址
        callback_header:
          type: object
          description: 回调header k-v
    ReEmbeddingResponse:
      type: object
      properties:
        code:
          type: integer
          description: 状态码
        message:
          type: string
          description: 返回信息
        timestamp:
          type: integer
          description: 时间戳
    LlmApplicationError:
      type: object
      properties:
        code:
          type: integer
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
