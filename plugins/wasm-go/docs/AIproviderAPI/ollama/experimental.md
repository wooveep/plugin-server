# Ollama Experimental API

- Source page: https://ollama.readthedocs.io/api/
- Source markdown: https://github.com/ollama/ollama/blob/main/docs/api.md
- Fetched on: 2026-04-23

## Included Sections

- Experimental Features

---

## Experimental Features

### Image Generation (Experimental)

> [!WARNING]
> Image generation is experimental and may change in future versions.

Image generation is now supported through the standard `/api/generate` endpoint when using image generation models. The API automatically detects when an image generation model is being used.

See the [Generate a completion](generate.md#generate-a-completion) section for the full API documentation. The experimental image generation parameters (`width`, `height`, `steps`) are documented there.

#### Example

##### Request

```shell
curl http://localhost:11434/api/generate -d '{
  "model": "x/z-image-turbo",
  "prompt": "a sunset over mountains",
  "width": 1024,
  "height": 768
}'
```

##### Response (streaming)

Progress updates during generation:

```json
{
  "model": "x/z-image-turbo",
  "created_at": "2024-01-15T10:30:00.000000Z",
  "completed": 5,
  "total": 20,
  "done": false
}
```

##### Final Response

```json
{
  "model": "x/z-image-turbo",
  "created_at": "2024-01-15T10:30:15.000000Z",
  "image": "iVBORw0KGgoAAAANSUhEUg...",
  "done": true,
  "done_reason": "stop",
  "total_duration": 15000000000,
  "load_duration": 2000000000
}
```
