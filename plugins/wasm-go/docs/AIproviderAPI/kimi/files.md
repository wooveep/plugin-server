# Kimi Files API

- Source group: official `platform.kimi.com/docs/api` pages
- Fetched on: 2026-04-23

## Sources

- https://platform.kimi.com/docs/api/files
- https://platform.kimi.com/docs/api/files-upload
- https://platform.kimi.com/docs/api/files-list
- https://platform.kimi.com/docs/api/files-retrieve
- https://platform.kimi.com/docs/api/files-delete
- https://platform.kimi.com/docs/api/files-content

---

> ## Documentation Index
> Fetch the complete documentation index at: https://platform.kimi.com/docs/llms.txt
> Use this file to discover all available pages before exploring further.

# 文件接口

Kimi API 提供文件管理功能，支持内容抽取、图片理解和视频理解。

<CardGroup cols={2}>
  <Card title="上传文件" icon="upload" href="https://platform.kimi.com/docs/api/files-upload">
    上传文件用于内容抽取或视觉理解
  </Card>

  <Card title="列出文件" icon="list" href="https://platform.kimi.com/docs/api/files-list">
    列举当前用户已上传的所有文件
  </Card>

  <Card title="获取文件信息" icon="circle-info" href="https://platform.kimi.com/docs/api/files-retrieve">
    获取指定文件的元数据
  </Card>

  <Card title="删除文件" icon="trash" href="https://platform.kimi.com/docs/api/files-delete">
    删除不再需要的文件
  </Card>

  <Card title="获取文件内容" icon="file-lines" href="https://platform.kimi.com/docs/api/files-content">
    获取文件内容抽取结果
  </Card>
</CardGroup>
---

> ## Documentation Index
> Fetch the complete documentation index at: https://platform.kimi.com/docs/llms.txt
> Use this file to discover all available pages before exploring further.

# 上传文件

> 上传文件用于内容提取、图片理解或视频理解。

<Note>
  单个用户最多只能上传 1000 个文件，单文件不超过 100MB，同时所有已上传的文件总和不超过 10G 容量。文件解析服务限时免费，请求高峰期平台可能会有限流策略。
</Note>

<Accordion title="支持的格式">
  文件接口支持以下格式：`.pdf`、`.txt`、`.csv`、`.doc`、`.docx`、`.xls`、`.xlsx`、`.ppt`、`.pptx`、`.md`、`.jpeg`、`.png`、`.bmp`、`.gif`、`.svg`、`.svgz`、`.webp`、`.ico`、`.xbm`、`.dib`、`.pjp`、`.tif`、`.pjpeg`、`.avif`、`.dot`、`.apng`、`.epub`、`.tiff`、`.jfif`、`.html`、`.json`、`.mobi`、`.log`、`.go`、`.h`、`.c`、`.cpp`、`.cxx`、`.cc`、`.cs`、`.java`、`.js`、`.css`、`.jsp`、`.php`、`.py`、`.py3`、`.asp`、`.yaml`、`.yml`、`.ini`、`.conf`、`.ts`、`.tsx` 等。
</Accordion>

<Accordion title="文件内容抽取示例">
  上传文件时选择 `purpose="file-extract"`，随后可以让模型获取文件中的信息作为上下文。

  <Tabs>
    <Tab title="python">
      ```python showLineNumbers expandable theme={null}
      from pathlib import Path
      from openai import OpenAI

      client = OpenAI(
          api_key = "$MOONSHOT_API_KEY",
          base_url = "https://api.moonshot.cn/v1",
      )

      # xlnet.pdf 是一个示例文件, 我们支持 pdf, doc 以及图片等格式
      file_object = client.files.create(file=Path("xlnet.pdf"), purpose="file-extract")

      # 获取结果
      # 注意，之前 retrieve_content api 在最新版本标记了 warning, 可以用下面这行代替
      # 如果是旧版本，可以用 retrieve_content
      file_content = client.files.content(file_id=file_object.id).text

      # 把它放进请求中
      messages = [
          {
              "role": "system",
              "content": "你是 Kimi，由 Moonshot AI 提供的人工智能助手，你更擅长中文和英文的对话。你会为用户提供安全，有帮助，准确的回答。同时，你会拒绝一切涉及恐怖主义，种族歧视，黄色暴力等问题的回答。Moonshot AI 为专有名词，不可翻译成其他语言。",
          },
          {
              "role": "system",
              "content": file_content,
          },
          {"role": "user", "content": "请简单介绍 xlnet.pdf 讲了啥"},
      ]

      # 然后调用 chat-completion, 获取 Kimi 的回答
      completion = client.chat.completions.create(
        model="kimi-k2-turbo-preview",
        messages=messages,
        temperature=0.6,
      )

      print(completion.choices[0].message)
      ```
    </Tab>

    <Tab title="curl">
      ```bash showLineNumbers theme={null}
      # xlnet.pdf 是一个示例文件

      curl https://api.moonshot.cn/v1/files \
        -H "Authorization: Bearer $MOONSHOT_API_KEY" \
        -F purpose="file-extract" \
        -F file="@xlnet.pdf"
      ```
    </Tab>

    <Tab title="node.js">
      ```js showLineNumbers expandable theme={null}
      const OpenAI = require("openai");
      const fs = require("fs")

      const client = new OpenAI({
          apiKey: "$MOONSHOT_API_KEY",
          baseURL: "https://api.moonshot.cn/v1",
      });

      async function main() {
          // xlnet.pdf 是一个示例文件, 我们支持 pdf, doc 以及图片等格式
          let file_object = await client.files.create({
              file: fs.createReadStream("xlnet.pdf"),
              purpose: "file-extract"
          })
          // 注意，之前 retrieve_content api 在最新版本标记了 warning, 可以用下面这行代替
          // 如果是旧版本，可以用 retrieve_content
          let file_content = await (await client.files.content(file_object.id)).text()

          // 把它放进请求中
          let messages = [
              {
                  "role": "system",
                  "content": "你是 Kimi，由 Moonshot AI 提供的人工智能助手，你更擅长中文和英文的对话。你会为用户提供安全，有帮助，准确的回答。同时，你会拒绝一切涉及恐怖主义，种族歧视，黄色暴力等问题的回答。Moonshot AI 为专有名词，不可翻译成其他语言。",
              },
              {
                  "role": "system",
                  "content": file_content,
              },
              {"role": "user", "content": "请简单介绍 xlnet.pdf 讲了啥"},
          ]

          const completion = await client.chat.completions.create({
              model: "kimi-k2-turbo-preview",
              messages: messages,
              temperature: 0.6
          });
          console.log(completion.choices[0].message.content);
      }

      main();
      ```
    </Tab>
  </Tabs>

  其中 `$MOONSHOT_API_KEY` 部分需要替换为您自己的 API Key。或者在调用前给它设置好环境变量。
</Accordion>

<Accordion title="多文件对话示例">
  如果你想一次性上传多个文件，并根据这些文件与 Kimi 对话，你可以参考如下示例：

  ```python expandable theme={null}
  from typing import *

  import os
  import json
  from pathlib import Path

  from openai import OpenAI

  client = OpenAI(
      base_url="https://api.moonshot.cn/v1",
      api_key=os.environ["MOONSHOT_DEMO_API_KEY"],
  )


  def upload_files(files: List[str]) -> List[Dict[str, Any]]:
      """
      upload_files 会将传入的文件（路径）全部通过文件上传接口 '/v1/files' 上传，并获取上传后的
      文件内容生成文件 messages。每个文件会是一个独立的 message，这些 message 的 role 均为
      system，Kimi 大模型会正确识别这些 system messages 中的文件内容。

      :param files: 一个包含要上传文件的路径的列表，路径可以是绝对路径也可以是相对路径，请使用字符串
          的形式传递文件路径。
      :return: 一个包含了文件内容的 messages 列表，请将这些 messages 加入到 Context 中，
          即请求 `/v1/chat/completions` 接口时的 messages 参数中。
      """
      messages = []

      for file in files:
          file_object = client.files.create(file=Path(file), purpose="file-extract")
          file_content = client.files.content(file_id=file_object.id).text
          messages.append({
              "role": "system",
              "content": file_content,
          })

      return messages


  def main():
      file_messages = upload_files(files=["upload_files.py"])

      messages = [
          *file_messages,
          {
              "role": "system",
              "content": "你是 Kimi，由 Moonshot AI 提供的人工智能助手，你更擅长中文和英文的对话。你会为用户提供安全，有帮助，"
                         "准确的回答。同时，你会拒绝一切涉及恐怖主义，种族歧视，黄色暴力等问题的回答。Moonshot AI 为专有名词，不"
                         "可翻译成其他语言。",
          },
          {
              "role": "user",
              "content": "总结一下这些文件的内容。",
          },
      ]

      print(json.dumps(messages, indent=2, ensure_ascii=False))

      completion = client.chat.completions.create(
          model="kimi-k2-turbo-preview",
          messages=messages,
      )

      print(completion.choices[0].message.content)


  if __name__ == '__main__':
      main()
  ```
</Accordion>

<Accordion title="用于图片或视频理解">
  上传文件时，选择 `purpose="image"` 或 `purpose="video"`，上传后的图片或视频可以用于模型的原生理解。

  请参阅[使用视觉模型](https://platform.kimi.com/docs/guide/use-kimi-vision-model)了解完整示例。
</Accordion>


## OpenAPI

````yaml POST /v1/files
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
  /v1/files:
    post:
      tags:
        - Files
      summary: 上传文件
      description: 上传文件用于内容提取、图片理解或视频理解。
      requestBody:
        required: true
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                file:
                  type: string
                  format: binary
                  description: 要上传的文件
                purpose:
                  type: string
                  enum:
                    - file-extract
                    - image
                    - video
                    - batch
                  description: >-
                    指定上传文件的处理方式。file-extract：抽取文件内容；image：上传图片，用于视觉理解；video：上传视频，用于视频理解；batch：上传
                    JSONL 文件，用于批处理任务
              required:
                - file
                - purpose
      responses:
        '200':
          description: 已上传文件的元数据
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/FileObject'
        '400':
          description: 请求错误 - 上传参数无效
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
    FileObject:
      type: object
      properties:
        id:
          type: string
          description: 文件唯一标识符
        object:
          type: string
          description: 对象类型
          example: file
        bytes:
          type: integer
          description: 文件大小（字节）
        created_at:
          type: integer
          description: 文件创建时的 Unix 时间戳
        filename:
          type: string
          description: 原始文件名
        purpose:
          type: string
          description: >-
            上传文件时指定的用途。file-extract：抽取文件内容；image：上传图片，用于视觉理解；video：上传视频，用于视频理解；batch：上传
            JSONL 文件，用于批处理任务
          enum:
            - file-extract
            - image
            - video
            - batch
        status:
          type: string
          description: 文件处理状态
          example: ready
        status_details:
          type: string
          description: 处理失败或返回警告时的额外状态详情
      required:
        - id
        - object
        - bytes
        - created_at
        - filename
        - purpose
        - status
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

# 列出文件

> 列出当前用户上传的所有文件。

<Accordion title="调用示例">
  ```python theme={null}
  file_list = client.files.list()

  for file in file_list.data:
      print(file) # 查看每个文件的信息
  ```
</Accordion>


## OpenAPI

````yaml GET /v1/files
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
  /v1/files:
    get:
      tags:
        - Files
      summary: 文件列表
      description: 列出当前用户上传的所有文件。
      responses:
        '200':
          description: 已上传文件列表
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/FileListResponse'
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
    FileListResponse:
      type: object
      properties:
        object:
          type: string
          example: list
        data:
          type: array
          items:
            $ref: '#/components/schemas/FileObject'
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
    FileObject:
      type: object
      properties:
        id:
          type: string
          description: 文件唯一标识符
        object:
          type: string
          description: 对象类型
          example: file
        bytes:
          type: integer
          description: 文件大小（字节）
        created_at:
          type: integer
          description: 文件创建时的 Unix 时间戳
        filename:
          type: string
          description: 原始文件名
        purpose:
          type: string
          description: >-
            上传文件时指定的用途。file-extract：抽取文件内容；image：上传图片，用于视觉理解；video：上传视频，用于视频理解；batch：上传
            JSONL 文件，用于批处理任务
          enum:
            - file-extract
            - image
            - video
            - batch
        status:
          type: string
          description: 文件处理状态
          example: ready
        status_details:
          type: string
          description: 处理失败或返回警告时的额外状态详情
      required:
        - id
        - object
        - bytes
        - created_at
        - filename
        - purpose
        - status
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

# 获取文件信息

> 获取指定已上传文件的元数据。

<Accordion title="调用示例">
  ```python theme={null}
  client.files.retrieve(file_id=file_id)
  # FileObject(
  #     id='clg681objj8g9m7n4je0',
  #     bytes=761790,
  #     created_at=1700815879,
  #     filename='xlnet.pdf',
  #     object='file',
  #     purpose='file-extract',
  #     status='ok', status_details='')
  ```
</Accordion>


## OpenAPI

````yaml GET /v1/files/{file_id}
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
  /v1/files/{file_id}:
    get:
      tags:
        - Files
      summary: 获取文件信息
      description: 获取指定已上传文件的元数据。
      parameters:
        - name: file_id
          in: path
          required: true
          description: 文件标识符
          schema:
            type: string
      responses:
        '200':
          description: 文件元数据
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/FileObject'
        '401':
          description: 未授权 - API 密钥无效或缺失
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '404':
          description: 文件未找到
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
    FileObject:
      type: object
      properties:
        id:
          type: string
          description: 文件唯一标识符
        object:
          type: string
          description: 对象类型
          example: file
        bytes:
          type: integer
          description: 文件大小（字节）
        created_at:
          type: integer
          description: 文件创建时的 Unix 时间戳
        filename:
          type: string
          description: 原始文件名
        purpose:
          type: string
          description: >-
            上传文件时指定的用途。file-extract：抽取文件内容；image：上传图片，用于视觉理解；video：上传视频，用于视频理解；batch：上传
            JSONL 文件，用于批处理任务
          enum:
            - file-extract
            - image
            - video
            - batch
        status:
          type: string
          description: 文件处理状态
          example: ready
        status_details:
          type: string
          description: 处理失败或返回警告时的额外状态详情
      required:
        - id
        - object
        - bytes
        - created_at
        - filename
        - purpose
        - status
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

# 删除文件

> 删除一个已上传的文件。

<Accordion title="调用示例">
  ```python theme={null}
  client.files.delete(file_id=file_id)
  ```
</Accordion>


## OpenAPI

````yaml DELETE /v1/files/{file_id}
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
  /v1/files/{file_id}:
    delete:
      tags:
        - Files
      summary: 删除文件
      description: 删除一个已上传的文件。
      parameters:
        - name: file_id
          in: path
          required: true
          description: 文件标识符
          schema:
            type: string
      responses:
        '200':
          description: 删除结果
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/FileDeleteResponse'
        '401':
          description: 未授权 - API 密钥无效或缺失
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '404':
          description: 文件未找到
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
    FileDeleteResponse:
      type: object
      properties:
        id:
          type: string
          description: 已删除文件的标识符
        object:
          type: string
          example: file
        deleted:
          type: boolean
          description: 文件是否删除成功
      required:
        - id
        - object
        - deleted
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

# 获取文件内容

> 获取以 `file-extract` 用途上传的文件的提取文本内容。

<Accordion title="调用示例">
  <Tabs>
    <Tab title="python">
      ```python theme={null}
      # 注意，之前 retrieve_content api 在最新版本标记了 warning, 可以用下面这行代替
      # 如果是旧版本，可以用 retrieve_content
      file_content = client.files.content(file_id=file_object.id).text
      ```
    </Tab>

    <Tab title="curl">
      ```bash theme={null}
      curl https://api.moonshot.cn/v1/files/{file_id}/content \
        -H "Authorization: Bearer $MOONSHOT_API_KEY"
      ```
    </Tab>
  </Tabs>
</Accordion>


## OpenAPI

````yaml GET /v1/files/{file_id}/content
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
  /v1/files/{file_id}/content:
    get:
      tags:
        - Files
      summary: 获取文件内容
      description: 获取以 `file-extract` 用途上传的文件的提取文本内容。
      parameters:
        - name: file_id
          in: path
          required: true
          description: 文件标识符
          schema:
            type: string
      responses:
        '200':
          description: 提取的文件内容
          content:
            text/plain:
              schema:
                type: string
                description: 提取的文件内容（纯文本）
        '401':
          description: 未授权 - API 密钥无效或缺失
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '404':
          description: 文件未找到
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
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      description: >-
        Authorization 请求头需要一个 Bearer 令牌。使用 MOONSHOT_API_KEY 作为令牌。这是一个服务端密钥，请在
        [API 密钥页面](https://platform.kimi.com/console/api-keys) 生成。

````
