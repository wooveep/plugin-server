# Ollama API Overview

- Source page: https://ollama.readthedocs.io/api/
- Source markdown: https://github.com/ollama/ollama/blob/main/docs/api.md
- Fetched on: 2026-04-23

## Included Sections

- Endpoints
- Conventions

---

# API

> Note: Ollama's API docs are moving to https://docs.ollama.com/api

---

## Endpoints

- [Generate a completion](generate.md#generate-a-completion)
- [Generate a chat completion](chat.md#generate-a-chat-completion)
- [Create a Model](model-creation.md#create-a-model)
- [List Local Models](models.md#list-local-models)
- [Show Model Information](models.md#show-model-information)
- [Copy a Model](models.md#copy-a-model)
- [Delete a Model](models.md#delete-a-model)
- [Pull a Model](models.md#pull-a-model)
- [Push a Model](models.md#push-a-model)
- [Generate Embeddings](embeddings.md#generate-embeddings)
- [List Running Models](system.md#list-running-models)
- [Version](system.md#version)
- [Experimental: Image Generation](experimental.md#image-generation-experimental)

---

## Conventions

### Model names

Model names follow a `model:tag` format, where `model` can have an optional namespace such as `example/model`. Some examples are `orca-mini:3b-q8_0` and `llama3:70b`. The tag is optional and, if not provided, will default to `latest`. The tag is used to identify a specific version.

### Durations

All durations are returned in nanoseconds.

### Streaming responses

Certain endpoints stream responses as JSON objects. Streaming can be disabled by providing `{"stream": false}` for these endpoints.
