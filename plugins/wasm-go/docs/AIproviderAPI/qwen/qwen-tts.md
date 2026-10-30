# 非实时语音合成（Qwen-TTS）API参考

- Source: https://help.aliyun.com/zh/model-studio/qwen-tts-api
- Protocol: DashScope native API
- Verified on: 2026-06-14

---

Qwen-TTS supports non-streaming text-to-speech over HTTP. The non-streaming response returns an audio file URL with an expiry time rather than inline audio bytes.

## HTTP 调用

`POST https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation`

Use the Singapore endpoint and API key for Singapore workspaces when applicable.

## Request shape

```json
{
  "model": "qwen3-tts-flash",
  "input": {
    "text": "Today is a wonderful day to build something people love.",
    "voice": "Cherry",
    "language_type": "English",
    "instructions": "Speak warmly.",
    "optimize_instructions": true
  }
}
```

Fields used by ai-proxy:

| Field | Notes |
| --- | --- |
| `model` | Qwen-TTS model name, for example `qwen3-tts-flash`. Instruction control requires a model that supports instructions, such as `qwen3-tts-instruct-flash`. |
| `input.text` | Text to synthesize. |
| `input.voice` | Voice name, for example `Cherry`. |
| `input.language_type` | Optional language hint, for example `English`. |
| `input.instructions` | Optional natural-language voice style instruction. |
| `input.optimize_instructions` | Optional flag for instruction optimization. |

## Success response shape

```json
{
  "status_code": 200,
  "request_id": "req-audio",
  "code": "",
  "message": "",
  "output": {
    "audio": {
      "url": "https://dashscope-result.example/audio.wav",
      "id": "audio_123",
      "expires_at": 1766113409
    }
  },
  "usage": {
    "input_tokens": 76,
    "output_tokens": 1045,
    "characters": 0,
    "total_tokens": 1121
  }
}
```

If the provider returns an error, `code` and `message` identify the failure. ai-proxy preserves provider error bodies for Qwen-TTS native conversion.
