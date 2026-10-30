# Kimi API Quickstart

- Source group: official `platform.kimi.com/docs/api` pages
- Fetched on: 2026-04-23

## Sources

- https://platform.kimi.com/docs/api/quickstart

---

> ## Documentation Index
> Fetch the complete documentation index at: https://platform.kimi.com/docs/llms.txt
> Use this file to discover all available pages before exploring further.

# 快速开始

## 单轮对话

使用 OpenAI SDK 和 cURL 与 Chat Completions API 进行交互：

<CodeGroup>
  ```python Python theme={null}
  from openai import OpenAI

  client = OpenAI(
      api_key = "$MOONSHOT_API_KEY",
      base_url = "https://api.moonshot.cn/v1",
  )

  completion = client.chat.completions.create(
      model = "kimi-k2.6",
      messages = [
          {"role": "system", "content": "你是 Kimi，由 Moonshot AI 提供的人工智能助手，你更擅长中文和英文的对话。你会为用户提供安全，有帮助，准确的回答。同时，你会拒绝一切涉及恐怖主义，种族歧视，黄色暴力等问题的回答。Moonshot AI 为专有名词，不可翻译成其他语言。"},
          {"role": "user", "content": "你好，我叫李雷，1+1等于多少？"}
      ]
  )

  print(completion.choices[0].message.content)
  ```

  ```bash cURL theme={null}
  curl https://api.moonshot.cn/v1/chat/completions \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $MOONSHOT_API_KEY" \
      -d '{
          "model": "kimi-k2.6",
          "messages": [
              {"role": "system", "content": "你是 Kimi，由 Moonshot AI 提供的人工智能助手，你更擅长中文和英文的对话。你会为用户提供安全，有帮助，准确的回答。同时，你会拒绝一切涉及恐怖主义，种族歧视，黄色暴力等问题的回答。Moonshot AI 为专有名词，不可翻译成其他语言。"},
              {"role": "user", "content": "你好，我叫李雷，1+1等于多少？"}
          ]
     }'
  ```

  ```javascript Node.js theme={null}
  const OpenAI = require("openai");

  const client = new OpenAI({
      apiKey: "$MOONSHOT_API_KEY",
      baseURL: "https://api.moonshot.cn/v1",
  });

  async function main() {
      const completion = await client.chat.completions.create({
          model: "kimi-k2.6",
          messages: [
              {role: "system", content: "你是 Kimi，由 Moonshot AI 提供的人工智能助手，你更擅长中文和英文的对话。你会为用户提供安全，有帮助，准确的回答。同时，你会拒绝一切涉及恐怖主义，种族歧视，黄色暴力等问题的回答。Moonshot AI 为专有名词，不可翻译成其他语言。"},
              {role: "user", content: "你好，我叫李雷，1+1等于多少？"}
          ]
      });
      console.log(completion.choices[0].message.content);
  }

  main();
  ```
</CodeGroup>

其中 `$MOONSHOT_API_KEY` 需要替换为您在平台上创建的 API Key。

## 多轮对话

将模型输出的结果继续作为输入的一部分以实现多轮对话：

<CodeGroup>
  ```python Python theme={null}
  from openai import OpenAI

  client = OpenAI(
      api_key = "$MOONSHOT_API_KEY",
      base_url = "https://api.moonshot.cn/v1",
  )

  history = [
      {"role": "system", "content": "你是 Kimi，由 Moonshot AI 提供的人工智能助手，你更擅长中文和英文的对话。你会为用户提供安全，有帮助，准确的回答。同时，你会拒绝一切涉及恐怖主义，种族歧视，黄色暴力等问题的回答。Moonshot AI 为专有名词，不可翻译成其他语言。"}
  ]

  def chat(query, history):
      history.append({
          "role": "user",
          "content": query
      })
      completion = client.chat.completions.create(
          model="kimi-k2.6",
          messages=history
      )
      result = completion.choices[0].message.content
      history.append({
          "role": "assistant",
          "content": result
      })
      return result

  print(chat("地球的自转周期是多少？", history))
  print(chat("月球呢？", history))
  ```

  ```javascript Node.js theme={null}
  const OpenAI = require("openai");

  const client = new OpenAI({
      apiKey: "$MOONSHOT_API_KEY",
      baseURL: "https://api.moonshot.cn/v1",
  });

  let history = [{"role": "system", "content": "你是 Kimi，由 Moonshot AI 提供的人工智能助手，你更擅长中文和英文的对话。你会为用户提供安全，有帮助，准确的回答。同时，你会拒绝一切涉及恐怖主义，种族歧视，黄色暴力等问题的回答。Moonshot AI 为专有名词，不可翻译成其他语言。"}];

  async function chat(prompt) {
      history.push({
          role: "user", content: prompt
      })
      const completion = await client.chat.completions.create({
          model: "kimi-k2.6",
          messages: history,
      });
      history = history.concat(completion.choices[0].message)
      return completion.choices[0].message.content;
  }

  async function main() {
      let reply = await chat("地球的自转周期是多少？")
      console.log(reply);
      reply = await chat("月球呢？")
      console.log(reply);
  }

  main();
  ```
</CodeGroup>

<Note>
  随着对话的进行，模型每次需要传入的 token 都会线性增加，必要时，需要一些策略进行优化，例如只保留最近几轮对话。
</Note>

## 流式输出

使用 `stream: true` 启用流式返回，获得更好的用户体验：

<CodeGroup>
  ```python Python theme={null}
  from openai import OpenAI

  client = OpenAI(
      api_key = "$MOONSHOT_API_KEY",
      base_url = "https://api.moonshot.cn/v1",
  )

  response = client.chat.completions.create(
      model="kimi-k2.6",
      messages=[
          {
              "role": "system",
              "content": "你是 Kimi，由 Moonshot AI 提供的人工智能助手，你更擅长中文和英文的对话。你会为用户提供安全，有帮助，准确的回答。同时，你会拒绝一切涉及恐怖主义，种族歧视，黄色暴力等问题的回答。Moonshot AI 为专有名词，不可翻译成其他语言。",
          },
          {"role": "user", "content": "你好，我叫李雷，1+1等于多少？"},
      ],
      stream=True,
  )

  collected_messages = []
  for idx, chunk in enumerate(response):
      chunk_message = chunk.choices[0].delta
      if not chunk_message.content:
          continue
      collected_messages.append(chunk_message)
      print(f"#{idx}: {''.join([m.content for m in collected_messages])}")
  print(f"Full conversation received: {''.join([m.content for m in collected_messages])}")
  ```

  ```bash cURL theme={null}
  curl https://api.moonshot.cn/v1/chat/completions \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $MOONSHOT_API_KEY" \
    -d '{
      "model": "kimi-k2.6",
      "messages": [
        {
          "role": "system",
          "content": "你是 Kimi，由 Moonshot AI 提供的人工智能助手，你更擅长中文和英文的对话。你会为用户提供安全，有帮助，准确的回答。同时，你会拒绝一切涉及恐怖主义，种族歧视，黄色暴力等问题的回答。Moonshot AI 为专有名词，不可翻译成其他语言。"
        },
        {
          "role": "user",
          "content": "你好，我叫李雷，1+1等于多少？"
        }
      ],
      "stream": true
    }'
  ```

  ```javascript Node.js theme={null}
  const OpenAI = require("openai");

  const client = new OpenAI({
      apiKey: "$MOONSHOT_API_KEY",
      baseURL: "https://api.moonshot.cn/v1",
  });

  async function main() {
      const stream = await client.chat.completions.create({
          model: "kimi-k2.6",
          messages: [
              {
                  role: "system",
                  content: "你是 Kimi，由 Moonshot AI 提供的人工智能助手，你更擅长中文和英文的对话。你会为用户提供安全，有帮助，准确的回答。同时，你会拒绝一切涉及恐怖主义，种族歧视，黄色暴力等问题的回答。Moonshot AI 为专有名词，不可翻译成其他语言。"
              },
              {
                  role: "user",
                  content: "你好，我叫李雷，1+1等于多少？"
              }
          ],
          stream: true,
      });

      for await (const chunk of stream) {
          const delta = chunk.choices[0].delta;
          if (delta.content) {
              process.stdout.write(delta.content);
          }
      }
  }

  main();
  ```
</CodeGroup>
