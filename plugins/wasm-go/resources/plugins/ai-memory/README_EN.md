---
title: AI Memory
keywords: [AI Gateway, AI Memory, ai-memory]
description: Configuration reference for the ai-memory thin gateway plugin.
---

## Overview

`ai-memory` is a thin Higress Wasm-Go gateway plugin. It handles request-time
memory injection, response capture, and event delivery for OpenAI-compatible
JSON AI traffic. It performs request gating, reads tenant and consumer identity,
loads Console-materialized Redis recent memory, optionally calls
`POST /internal/memory/assemble`, rewrites the request body with safe memory
messages, and emits one `MemoryEvent` to Redis Stream `memory:events` field
`event` after the response completes.

Console owns durable memory responsibilities: PostgreSQL persistence,
recent-window materialization, daily digest generation, semantic recall,
embedding, vector indexing, deletion, repair, management APIs, and backend model
cost attribution are outside the gateway plugin.

## Runtime Responsibilities

Only synchronous text output participates in memory lookup, injection, and capture. Explicit nontext output and Responses background execution bypass these steps; actual mixed text/media responses are not captured as text memory. Image, audio, and file input attachments remain supported for text output.

The plugin:

- Runs request gating by `enable_path_suffixes` and JSON `content-type`.
- Extracts tenant identity, consumer identity, optional session, and request id
  through `tenant_header`, `consumer_header`, `session_header`, and
  `request_id_header`.
- Reads Redis recent memory with `recent_cache.key_prefix`, defaulting to
  `memory:recent`.
- Calls Console assemble only for `digest` and `semantic` memory modes.
- Injects a Console memory message and safe recent messages into the
  OpenAI-compatible request body.
- Captures assistant response facts, usage, finish reason, status code, and
  tool-call markers.
- Sends `XADD memory:events * event <MemoryEvent JSON>` after response
  completion.
- Uses fail-open behavior for Redis, Console, parsing, replacement, response
  capture, and event dispatch failures.

The plugin does not write PostgreSQL, generate daily memory digests, generate
embeddings, call vector databases, call provider models for memory work, apply
retention or deletion policy, expose management APIs, calculate backend model
cost, or create billing settlement records. It may build a request digest as a
safe protocol fact for Console assemble and MemoryEvent idempotency.

## Ownership Boundary

Console owns every durable or policy-heavy memory concern: PostgreSQL records,
recent-window materialization, daily digests, semantic recall, embeddings,
vector indexes, retention, deletion, repair, management APIs, authorization
checks, and backend model cost attribution.

The gateway plugin is intentionally not a memory system of record. It does not
persist conversations, generate memory artifacts, choose retention policy,
operate vector storage, repair missing turns, expose operator APIs, or settle
billing. It only consumes Console-derived runtime state and emits one
best-effort event for Console workers.

## Privacy Boundary

`ai-memory` does not log raw prompts, raw answers, credentials, Authorization
headers, Redis passwords, or internal bearer tokens. Redis Stream dispatch
failure logs include only safe diagnostics such as request id, stream, status,
and fixed reason codes.

When `capture_response` is false, a no-store header is present, parsing fails,
content is unsafe, or the status code is ineligible, the MemoryEvent keeps safe
facts but omits `user_content` and `assistant_content`.

## Configuration

The global and route schemas declare the three external service targets for the control plane: `redis_stream`, `recent_cache`, and `console_internal`. These declarations identify existing Redis/DNS and HTTP callout dependencies; they do not create a new event channel.

Global `defaultConfig` contains Redis Stream, Redis recent memory, Console
internal service, identity headers, path suffixes, and `fail_policy`.
`matchRules[].config` contains route memory behavior and inherits the global
external targets. The first version rejects route-level `redis_stream`,
`recent_cache`, and `console_internal` overrides.

```yaml
defaultConfig:
  redis_stream:
    service_name: redis-stack-server.dns
    service_port: 6379
    username: ""
    password: <redis-password>
    database: 0
    timeout: 500
    stream: memory:events
  recent_cache:
    service_name: redis-stack-server.dns
    service_port: 6379
    username: ""
    password: <redis-password>
    database: 0
    timeout: 50
    key_prefix: memory:recent
  console_internal:
    service_name: modelfusion-console-api.dns
    service_port: 80
    assemble_path: /internal/memory/assemble
    timeout_ms: 100
    auth_token: <internal-bearer-token>
  tenant_header: x-mse-tenant
  consumer_header: x-mse-consumer
  session_header: x-mse-session
  request_id_header: x-request-id
  enable_path_suffixes:
    - /v1/chat/completions
    - /v1/messages
    - /v1/responses
  fail_policy: open
matchRules:
  - ingress:
      - qwen
    config:
      memory_mode: semantic
      recent_window_turns: 6
      memory_token_budget: 1500
      assemble_timeout_ms: 100
      inject_role: system
      developer_compatible: false
      semantic_top_k: 3
      capture_response: true
      no_store_header: x-higress-ai-memory-no-store
      question_from: messages.@reverse.#(role=="user").content
      response_value_from: choices.0.message.content
      stream_value_from: choices.0.delta.content
      tool_calls_from:
        - choices.0.message.tool_calls
        - choices.0.message.function_call
        - choices.0.delta.tool_calls
        - choices.0.delta.function_call
      policy_version: memory-policy-v1
```

`memory_mode` accepts `off`, `recent-only`, `digest`, and `semantic`.
`fail_policy` currently supports only `open`.

## Schema Examples

The `spec.yaml` `configSchema.openAPIV3Schema.example` shows the global
`defaultConfig`: Redis Stream `redis_stream`, Redis recent memory
`recent_cache`, Console internal service `console_internal`, identity headers,
`enable_path_suffixes`, and `fail_policy: open` are deployment-level settings.

The `routeConfigSchema.openAPIV3Schema.example` shows route memory behavior for
`matchRules[].config`: `memory_mode`, `recent_window_turns`,
`memory_token_budget`, `assemble_timeout_ms`, `semantic_top_k`,
`capture_response`, the no-store header, and request/response extraction paths.
The route example does not include Redis or Console external targets; those are
inherited from `defaultConfig`.

## Request and Response Flow

The header phase checks path suffix, JSON content type, tenant, and consumer
identity. Missing identity or unsupported paths continue upstream unchanged.

The body phase parses OpenAI-compatible messages, extracts the latest user
content, builds a request digest, loads Redis recent memory, and optionally
calls Console assemble for `digest` and `semantic` modes. Console may return
`inject`, `recent_only`, `skip`, or `bypass`. Request body replacement failures
continue upstream with the original body.

The response phase supports non-streaming and streaming SSE response capture.
It handles split chunks, `\r\n`, `\r`, `\n`, `[DONE]`, and role-only chunks, and
sends exactly one MemoryEvent after the final chunk.

## Ordering and Isolation

Run `ai-memory` after identity plugins and `ai-quota` so trusted tenant and
consumer identity are available. Run it before `ai-cache` and `ai-proxy` when
memory injection can affect the model request.

## Ordering Recommendation

Recommended request-path order:

1. Identity or authentication plugins populate trusted tenant and consumer
   headers.
2. `ai-quota` runs with the trusted identity and can reject over-quota traffic
   before memory work starts.
3. `ai-memory` injects memory and records response facts.
4. `ai-cache` runs only with a conservative policy for memory-enabled routes:
   consumer-scoped cache, memory-policy or digest-aware keys, or bypass.
5. `ai-proxy` forwards the final memory-adjusted request to the provider.

`ai-memory` is isolated from `ai-cache`: memory events use `memory:events`,
recent keys use `memory:recent`, and MemoryEvent payload schemas are separate
from cache payload schemas. Vector namespaces, retention, deletion, management
APIs, authorization checks, and repair policy belong to Console.

## Cache and Memory Isolation

| Concern | `ai-memory` boundary |
| --- | --- |
| Redis Stream | Uses `memory:events`; never writes cache streams. |
| Redis key prefix | Reads Console-materialized recent memory under `memory:recent`; never reads or writes cache key prefixes. |
| Vector namespace | Does not create, read, or mutate vector namespaces; Console owns memory vector indexing. |
| Payload schema | Emits `MemoryEvent`; never reuses cache request, response, replay, or settlement payload schemas. |
| Retention and deletion | Does not enforce retention or deletion; Console owns memory lifecycle policy. |
| Management APIs | Exposes no memory management APIs from the gateway. |
| Authorization checks | Trusts upstream identity headers and leaves memory authorization checks to Console APIs and workers. |

Ordinary production retains the original Redis-admitted call identity and business time without a temporary Console permit deadline. Upload, send and retry keep their existing budgets. Explicit bounded validation/internal permits retain their expiry; no renewal or extension path is introduced.
