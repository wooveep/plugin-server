# Ollama Embeddings API

- Source page: https://ollama.readthedocs.io/api/
- Source markdown: https://github.com/ollama/ollama/blob/main/docs/api.md
- Fetched on: 2026-04-23

## Included Sections

- Generate Embeddings
- Generate Embedding

---

## Generate Embeddings

```
POST /api/embed
```

Generate embeddings from a model

### Parameters

- `model`: name of model to generate embeddings from
- `input`: text or list of text to generate embeddings for

Advanced parameters:

- `truncate`: truncates the end of each input to fit within context length. Returns error if `false` and context length is exceeded. Defaults to `true`
- `options`: additional model parameters listed in the documentation for the [Modelfile](https://docs.ollama.com/modelfile#valid-parameters-and-values) such as `temperature`
- `keep_alive`: controls how long the model will stay loaded into memory following the request (default: `5m`)
- `dimensions`: number of dimensions for the embedding

### Examples

#### Request

```shell
curl http://localhost:11434/api/embed -d '{
  "model": "all-minilm",
  "input": "Why is the sky blue?"
}'
```

#### Response

```json
{
  "model": "all-minilm",
  "embeddings": [
    [
      0.010071029, -0.0017594862, 0.05007221, 0.04692972, 0.054916814,
      0.008599704, 0.105441414, -0.025878139, 0.12958129, 0.031952348
    ]
  ],
  "total_duration": 14143917,
  "load_duration": 1019500,
  "prompt_eval_count": 8
}
```

#### Request (Multiple input)

```shell
curl http://localhost:11434/api/embed -d '{
  "model": "all-minilm",
  "input": ["Why is the sky blue?", "Why is the grass green?"]
}'
```

#### Response

```json
{
  "model": "all-minilm",
  "embeddings": [
    [
      0.010071029, -0.0017594862, 0.05007221, 0.04692972, 0.054916814,
      0.008599704, 0.105441414, -0.025878139, 0.12958129, 0.031952348
    ],
    [
      -0.0098027075, 0.06042469, 0.025257962, -0.006364387, 0.07272725,
      0.017194884, 0.09032035, -0.051705178, 0.09951512, 0.09072481
    ]
  ]
}
```

---

## Generate Embedding

> Note: this endpoint has been superseded by `/api/embed`

```
POST /api/embeddings
```

Generate embeddings from a model

### Parameters

- `model`: name of model to generate embeddings from
- `prompt`: text to generate embeddings for

Advanced parameters:

- `options`: additional model parameters listed in the documentation for the [Modelfile](https://docs.ollama.com/modelfile#valid-parameters-and-values) such as `temperature`
- `keep_alive`: controls how long the model will stay loaded into memory following the request (default: `5m`)

### Examples

#### Request

```shell
curl http://localhost:11434/api/embeddings -d '{
  "model": "all-minilm",
  "prompt": "Here is an article about llamas..."
}'
```

#### Response

```json
{
  "embedding": [
    0.5670403838157654, 0.009260174818336964, 0.23178744316101074,
    -0.2916173040866852, -0.8924556970596313, 0.8785552978515625,
    -0.34576427936553955, 0.5742510557174683, -0.04222835972905159,
    -0.137906014919281
  ]
}
```
