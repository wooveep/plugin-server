# ai-memory Plugin Design

## Summary

`ai-memory` is a thin Higress Wasm-Go gateway plugin. It injects memory into
OpenAI-compatible chat requests, captures the current turn from the upstream
response, and emits one `MemoryEvent` to Redis Stream after the response
finishes.

The plugin does not own durable memory. Console owns PostgreSQL persistence,
recent-window materialization, daily digest generation, semantic recall,
embedding, vector indexing, deletion, repair, management APIs, and backend model
cost attribution.

The paired Console design is:
`/home/cloudyi/CodeWorkspace/ocloudmodelfusion/docs/superpowers/specs/2026-06-26-ai-memory-console-design.md`.

## Relationship to ai-cache Shared Runtime

`ai-memory` and the redesigned thin `ai-cache` plugin should share protocol
helpers, but they should not share business state or a runtime plugin.

Shared helper candidates:

- Tenant, consumer, optional session, and request id extraction.
- Path suffix and JSON content-type gating.
- OpenAI-compatible request message parsing.
- Latest user message and request digest extraction.
- Request body replacement for message injection.
- Streaming and non-streaming assistant text capture.
- Tool-call detection.
- Safe usage and finish reason extraction when present.
- Redis Stream envelope and `XADD` helper.
- Fail-open logging helpers that avoid prompt, answer, token, and credential
  leakage.
- Console internal HTTP callout helper with short timeout.

The helper package must stay policy-free. It can return protocol facts and
perform mechanical body handling, but it must not know cache policy, memory
policy, vector providers, pricing, billing settlement, or deletion rules.

Isolated state:

- `ai-cache` stream: `cache:events`; `ai-memory` stream: `memory:events`.
- `ai-cache` Redis key prefixes remain cache-owned; `ai-memory` uses memory
  prefixes such as `memory:recent:` and `memory:assemble:`.
- `ai-cache` vector collection and payload schema are separate from
  `ai_memory_chunks`.
- Retention and deletion policies are separate.

## Plugin Responsibilities

The plugin SHALL:

- Run only for configured AI path suffixes, defaulting to
  `/v1/chat/completions`, `/v1/messages`, and `/v1/responses`.
- Process only JSON requests.
- Read identity from `x-mse-tenant` and `x-mse-consumer` by default.
- Skip memory behavior when tenant or consumer is missing.
- Parse current request messages and latest user content.
- Read Console-derived Redis recent memory when configured.
- Optionally call `POST /internal/memory/assemble` with a short timeout.
- Inject memory and recent messages into the request body.
- Capture non-streaming and streaming assistant response text.
- Mark tool-call responses with `contains_tool_calls=true`.
- Emit one `MemoryEvent` to Redis Stream field `event` after response
  completion.
- Fail open for Redis, Console API, parsing, body replacement, and event
  delivery failures.

The plugin SHALL NOT:

- Write PostgreSQL.
- Generate daily digests.
- Generate embeddings.
- Call vector databases.
- Call provider model APIs for memory work.
- Apply memory retention or deletion policy.
- Calculate cost, debit balance, or create customer billing statements.
- Log raw prompts, raw answers, credentials, authorization headers, Redis
  passwords, or internal bearer tokens.

## Configuration

The plugin uses global `defaultConfig` plus route-level `matchRules[].config`,
matching existing Wasm-Go plugin practice. External targets are global plugin
instance configuration; route-level rules inherit them and may only override
route behavior.

### Global config

```yaml
redis_stream:
  service_name: redis-stack-server.dns
  service_port: 6379
  database: 0
  timeout: 500
  stream: memory:events
recent_cache:
  service_name: redis-stack-server.dns
  service_port: 6379
  database: 0
  timeout: 50
  key_prefix: memory:recent
console_internal:
  service_name: modelfusion-console-api.dns
  service_port: 80
  assemble_path: /internal/memory/assemble
  timeout_ms: 100
tenant_header: x-mse-tenant
consumer_header: x-mse-consumer
session_header: x-mse-session
request_id_header: x-request-id
enable_path_suffixes:
  - /v1/chat/completions
  - /v1/messages
  - /v1/responses
fail_policy: open
```

Rules:

- `redis_stream.service_name` is required when event emission is enabled.
- `redis_stream.stream` defaults to `memory:events`.
- Rule-level `redis_stream`, `recent_cache`, and `console_internal` overrides
  are not supported in the first version.
- A shared `redis_stream` and `recent_cache` service/port uses one raw Redis
  client. Database and credentials must match, and both startup initializations
  use immediate dispatch plus the smaller positive configured timeout.
- Distinct Redis endpoints retain their independently configured timeouts.
- `fail_policy` currently only supports `open`.
- Credential fields are sensitive and must be redacted in logs and Console UI.

### Route-level config

```yaml
memory_mode: semantic
recent_window_turns: 6
memory_token_budget: 1500
assemble_timeout_ms: 100
inject_role: system
semantic_top_k: 3
capture_response: true
no_store_header: x-higress-ai-memory-no-store
question_from: messages.@reverse.#(role=="user").content
response_value_from: choices.0.message.content
stream_value_from: choices.0.delta.content
tool_calls_from:
  - choices.0.delta.tool_calls
  - choices.0.delta.content.tool_calls
```

`memory_mode` values:

- `off`: skip injection and event capture.
- `recent-only`: use Redis recent messages only.
- `digest`: allow Console assemble for daily digest memory.
- `semantic`: allow Console assemble for daily digest and semantic recall.

`inject_role` defaults to `system`. `developer` is valid only when Console
policy marks the upstream route as developer-message compatible.

## Request Flow

1. Request headers phase:
   - Disable reroute if consistent with local plugin practice.
   - Check path suffix and JSON content type.
   - Read tenant, consumer, optional session, and request id.
   - Skip memory when identity is missing.
   - Remove `Accept-Encoding` and `Content-Length` before request body
     replacement.
   - Stop header iteration so request body can be inspected.

2. Request body phase:
   - Parse JSON and detect `stream`.
   - Extract current messages and latest user content.
   - Build a stable request digest over identity, route, model, and current
     request facts.
   - If the current client request already contains multiple user turns, avoid
     blindly injecting duplicate recent-window history. Console assemble may
     still return digest or semantic memory.
   - Read Redis recent messages using the Console-derived key prefix.
   - If mode is `digest` or `semantic`, call Console assemble with the configured
     timeout.
   - Build the final `messages` array:
     1. Original client `system` or `developer` messages.
     2. One Console-returned memory message, when present.
     3. Recent `user` and `assistant` messages that are safe to inject.
     4. Remaining current request messages.
   - Replace the request body and resume upstream.

3. Fallback behavior:
   - Redis recent read failure: continue without recent memory.
   - Console assemble timeout or failure: use Redis recent messages when
     available, otherwise continue without memory.
   - JSON parse or body replacement failure: continue with original request.

## Console Assemble Contract

The plugin calls `POST /internal/memory/assemble` only when route policy permits
digest or semantic memory. This call is optional for request correctness.

Request body:

```json
{
  "schema_version": 1,
  "tenant": "tenant-slug",
  "consumer": "consumer-name",
  "session_id": "optional-session-id",
  "route": { "name": "runtime-route" },
  "model": { "name": "runtime-model" },
  "request_id": "gateway-request-id",
  "request_path": "/v1/chat/completions",
  "current_question": "latest user input",
  "messages_digest": "sha256:...",
  "memory_mode": "semantic",
  "recent_window_turns": 6,
  "memory_token_budget": 1500,
  "semantic_top_k": 3,
  "policy_version": "7"
}
```

Response body:

```json
{
  "schema_version": 1,
  "decision": "inject",
  "memory_message": {
    "role": "system",
    "content": "Relevant long-term memory..."
  },
  "recent_messages": [
    { "role": "user", "content": "..." },
    { "role": "assistant", "content": "..." }
  ],
  "trace": {
    "policy_version": "7",
    "recent_source": "redis",
    "digest_count": 1,
    "semantic_count": 3
  },
  "diagnostics": {
    "safe_reason": "assembled"
  }
}
```

`decision` values are `inject`, `recent_only`, `skip`, and `bypass`. The plugin
must not log raw `memory_message` or raw `recent_messages`.

## Response Capture

For non-streaming responses, the plugin extracts assistant content from
`response_value_from`, default `choices.0.message.content`.

For streaming responses, the plugin accumulates complete SSE messages and
extracts content from `stream_value_from`, default `choices.0.delta.content`.
It must handle:

- `\r\n`, `\r`, and `\n` line endings.
- Partial SSE fragments split across chunks.
- `[DONE]` messages that arrive alone or in the same buffer as the final
  content chunk.
- Role-only chunks with no content.
- Tool-call deltas.

Tool calls set `contains_tool_calls=true`. They do not block event creation, but
Console policy may decide not to persist or summarize tool-call content.

## Memory Event Contract

After the response is complete, the plugin writes:

```text
XADD memory:events * event <MemoryEvent JSON>
```

The event JSON fields are:

```json
{
  "schema_version": 1,
  "event_id": "uuid-or-stable-generated-id",
  "idempotency_key": "stable-event-key",
  "tenant": "tenant-slug",
  "consumer": "consumer-name",
  "session_id": "optional-session-id",
  "route": { "name": "runtime-route" },
  "model": { "name": "runtime-model" },
  "request_id": "gateway-request-id",
  "request_path": "/v1/chat/completions",
  "request_digest": "sha256:...",
  "policy_version": "7",
  "memory_mode": "semantic",
  "user_content": "latest user input when capture is allowed",
  "assistant_content": "assistant output when capture is allowed",
  "status_code": 200,
  "is_stream": true,
  "contains_tool_calls": false,
  "usage": {
    "unit": "token",
    "input": 0,
    "output": 0,
    "total": 0
  },
  "finish_reason": "stop",
  "started_at_ms": 1782420000000,
  "ended_at_ms": 1782420000500,
  "plugin_version": "0.1.1"
}
```

`user_content` and `assistant_content` are omitted when `capture_response` is
false, the no-store header is present, parsing failed, content is unsafe by
policy, or status is not eligible. The event still carries safe operational
facts where possible.

Console ingestion is idempotent by `idempotency_key` and body digest. The
plugin does not trim the producer stream and does not write DLQ entries.

## Redis Recent Read

Recent memory in Redis is Console-derived runtime state. The plugin can read it
for low-latency fallback, but it must treat Redis as optional. Console stores
recent windows as a Redis sorted set at `<recent_cache.key_prefix>:<tenant>:<consumer>`;
the plugin reads the latest window with `ZREVRANGE 0 <recent_window_turns-1>`.

Recommended sorted-set member shape:

```json
{
  "schema_version": 1,
  "messages": [
    { "role": "user", "content": "..." },
    { "role": "assistant", "content": "..." }
  ],
  "captured_at_ms": 1782420000000
}
```

The plugin must miss on Redis errors, invalid JSON, unsupported roles, or
oversized values. Tenant and consumer isolation come from the Redis key parts.

## Failure and Latency Policy

Default behavior is fail-open:

- Missing identity: continue without memory.
- Redis recent read failure: continue without recent memory.
- Console assemble timeout: use Redis recent memory or continue without memory.
- Body parse or replacement failure: continue original request.
- Response capture failure: do not emit raw content.
- Redis Stream event failure: keep the user response unchanged and log safely.

Default timeout budgets:

- Redis recent read: 50 ms.
- Console assemble: 100 ms.
- Redis Stream XADD: 500 ms.

Route-level config can lower or raise assemble timeout, but production defaults
should favor user request latency over guaranteed memory recall.

## Security and Privacy

- Tenant and consumer headers are the memory isolation boundary.
- Memory must not cross consumers in the first version.
- Raw `Authorization` headers are never used as memory identity.
- No prompt, answer, secret, bearer token, API key, Redis password, or internal
  auth token may be written to plugin logs.
- Safe logs may include request id, stream name, status code, mode, timeout
  reason, and redacted route or model name.
- Internal Console call credentials are sensitive configuration.
- The plugin must not expose memory diagnostics in user response bodies.

## Plugin Ordering

For memory-enabled routes, `ai-memory` should run before any plugin that may
short-circuit the request with a generated or cached response. Otherwise a cache
hit can bypass memory injection and produce an answer that ignores user memory.

Recommended logical order:

```text
client request
  -> key-auth or identity plugins
  -> ai-quota
  -> ai-memory
  -> ai-cache
  -> ai-billing response observation
  -> ai-proxy
```

`ai-memory` should run after identity and quota checks so it can read
`x-mse-tenant` and `x-mse-consumer`, and before `ai-proxy` so it can rewrite the
request body before upstream invocation.

When `ai-cache` and `ai-memory` are both enabled, cache materialization and
lookup policy must account for the memory boundary. Acceptable first-version
options are:

- Consumer-scoped cache with the same tenant and consumer identity as memory.
- Cache key or policy version including memory policy version and assembled
  memory digest.
- Cache bypass for memory-enabled routes until Console can materialize
  memory-aware replay records.

`ai-memory` may or may not observe a response generated by a later local-reply
cache hit depending on filter-chain behavior. Console should not require cache
hit turns to be memory source events in the first version.

## Tests

Plugin tests should cover:

- Global and route-level config parsing.
- Rule-level rejection of `redis_stream`, `recent_cache`, and
  `console_internal` overrides.
- `fail_policy` accepting only `open`.
- Path suffix and content-type gating.
- Missing tenant or consumer fail-open.
- Request body parsing and latest user extraction.
- Multi-user-turn request avoids duplicate recent injection.
- Redis recent hit, miss, invalid JSON, unsupported role, and timeout.
- Console assemble inject, recent-only, skip, timeout, and failure.
- Request body replacement ordering.
- Non-streaming response extraction.
- Streaming SSE accumulation across split chunks.
- `[DONE]` handling with and without final content in the same buffer.
- Tool-call detection from both canonical OpenAI path
  `choices.0.delta.tool_calls` and legacy-compatible path
  `choices.0.delta.content.tool_calls`.
- No-store header omits raw content from events.
- `MemoryEvent` payload shape and redaction.
- Redis Stream dispatch failure fail-open behavior.

## Implementation Notes

- Start from existing Wasm-Go plugin lifecycle patterns:
  `ParseConfigBy`, `ProcessRequestHeadersBy`, `ProcessRequestBodyBy`,
  `ProcessResponseHeadersBy`, `ProcessStreamingResponseBodyBy`, and
  `ProcessStreamDone` where needed.
- Prefer a shared helper package for protocol parsing and Redis Stream delivery
  before duplicating ai-cache or ai-billing logic.
- Keep event delivery asynchronous and fail-open. The callback may log accepted
  stream id safely, but must not alter the already-sent response.
- Keep plugin docs and future catalog schema aligned with Console projection
  fields so managed runtime config can be generated without hand editing.
