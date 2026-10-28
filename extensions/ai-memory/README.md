---
title: AI Memory
keywords: [AI Gateway, AI Memory, ai-memory]
description: ai-memory thin gateway plugin configuration and runtime contract.
---

## Overview

`ai-memory` is a thin Higress Wasm-Go gateway plugin for OpenAI-compatible chat
traffic. It gates supported JSON AI requests, extracts trusted tenant and
consumer identity, optionally injects Console-derived memory into the request
body, captures assistant response facts, and emits one JSON `MemoryEvent` to
Redis Stream `memory:events` field `event` after the response completes.

The gateway plugin does not own durable memory. Console owns PostgreSQL
persistence, recent-window materialization, daily digest generation, semantic
recall, embeddings, vector indexing, deletion, repair, management APIs, and
backend model cost attribution.

## Responsibilities

Only synchronous text output participates in memory lookup, injection, and capture. Explicit nontext output and Responses background execution bypass these steps; actual mixed text/media responses are not captured as text memory. Image, audio, and file input attachments remain supported for text output.

The plugin handles only request-time and response-time gateway work:

- Request gating by path suffix and JSON `content-type`.
- Tenant, consumer, optional session, request id, route, model, request path,
  memory mode, policy version, stream flag, no-store flag, and request digest
  facts.
- Redis recent-memory fallback from Console-materialized recent-window state.
- Optional Console assemble callout to `POST /internal/memory/assemble` for
  `digest` and `semantic` modes.
- Safe request body replacement that preserves original high-priority client
  messages, injects one memory message when Console returns one, inserts safe
  recent `user` and `assistant` messages, and keeps the current request turns.
- Non-streaming and streaming assistant response capture, including usage,
  finish reason, response status, and tool-call detection.
- One `MemoryEvent` Redis Stream `XADD` after response completion.
- Fail-open behavior for Redis, Console, parsing, replacement, capture, and
  event dispatch failures.

The plugin does not write PostgreSQL, generate daily memory digests, generate
embeddings, call vector databases, call provider models for memory work,
enforce retention or deletion policy, expose memory management APIs, calculate
backend model cost, or create billing statements. It may build a request digest
as a safe protocol fact for Console assemble and MemoryEvent idempotency.

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

## Configuration

Global `defaultConfig` holds external targets and identity settings. Route
`matchRules[].config` holds memory behavior and inherits global external
targets. The first version rejects route-level `redis_stream`, `recent_cache`,
and `console_internal` overrides.

```yaml
defaultConfig:
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
      policy_version: "7"
```

When `redis_stream` and `recent_cache` share the same service and port, Higress
uses one raw Redis client for both. They must use the same database and
credentials. The plugin initializes both roles with immediate dispatch and the
smaller positive configured timeout, so the recent-memory latency bound also
applies to event writes on that shared endpoint. Distinct endpoints retain
their independently configured timeouts.

`memory_mode` accepts:

| Value | Behavior |
| --- | --- |
| `off` | Skips recent memory, Console assemble, request replacement, and raw event content capture. |
| `recent-only` | Uses only Redis recent-memory fallback. |
| `digest` | Allows Console assemble for digest memory. |
| `semantic` | Allows Console assemble for digest and semantic recall. |

`fail_policy` currently supports only `open`.

### Managed route coverage

For a route whose memory mode is already enabled, the control plane publishes
the original client protocol and operation in its `matchRules[].config`:

```yaml
path_scope: bound_target
client_protocol: openai_responses
operation_kind: text_generate
memory_mode: semantic
```

With `defaultConfigDisable: true`, the controller emits a root containing only
`_rules_`. That root stays dormant. Each rule contains the complete managed
configuration: its own `redis_stream`, optional `recent_cache` and
`console_internal`, identity headers, and the flat memory behavior fields above.
Only matching route/service requests use those targets; no global memory mode
is enabled. A real global configuration keeps the existing inherited-target
contract and rejects external-target overrides in its rules.

`bound_target` uses the matched route directly and discards inherited global
`enable_path_suffixes`. Additional nonempty path or method filters are rejected.
The trusted protocol is selected once during configuration loading, then kept
through request assembly and response capture. Custom aliases and upstream
path rewrites do not select another adapter, and client headers cannot select
the protocol. Supported text protocols are `openai_chat`, `openai_responses`,
and `anthropic_messages`. A complete unsupported protocol/operation binding
passes through with `unsupported-protocol-or-operation`; incomplete or nonstring
published fields are configuration errors. A request without the selected
protocol's messages/input shape passes through with `unsupported-request-content`.
Gate reasons are written to the existing custom-log field `ai_memory_gate`.

The release may set `capture_body_basis: true` on a supported published text
binding when memory runs before the model mapper. This designates memory as
the first original-body capture boundary. It removes all client occurrences of
`x-modelfusion-original-body-digest` and `x-modelfusion-original-public-model`
in the header phase, buffers the original JSON even when memory is off or its
identity gates bypass, then uses `bodybasis.Capture` before any memory rewrite.
The digest follows `mf-json.v1`; capture does not rewrite the original bytes.
No-store keeps its existing memory policy and does not disable this basis.
Invalid JSON/model, unsupported encoding, a missing body, or host capture
failure rejects the call instead of forwarding an untrusted basis. The default
is false and keeps legacy behavior. Cache passes the captured basis through.
The control plane must freeze a matching later mapper source that reads and
preserves this prior gateway basis; header presence alone never selects trust.

An optional `native_identity` on the same managed rule contains exactly the
original `release_id`, `target_id`, `route_id`, `policy_id`, and positive i64
integer `policy_version`. The existing string policy version, when supplied,
must be its canonical decimal equivalent. Caller fields are not configuration.
After native key authentication, memory captures the five original
`balanceidentity` fields once and adds that same caller snapshot to both its
Console assemble request and `MemoryEvent`. Missing or ambiguous native caller
identity bypasses memory without a legacy-name lookup or auxiliary call.

Native tenant and consumer facts use the authenticated tenant and principal
UUIDs. The route context and `route.id` use the original frozen route UUID;
`route.name` keeps the Envoy route only as a diagnostic. The original request's
public model remains a diagnostic for Console's frozen-target resolution.
Native recent-memory Redis keys use Console's existing
`consumer-name-sha256:<sha256(principal UUID)>` component; event and context
consumer values retain the UUID itself. Legacy keys and identity behavior stay
unchanged. This wire requires the matching Console native identity boundary and
new exact artifact schema before enabling it. Attachment-only requests without
a text question still have the previously identified Console semantic-assembly
limitation; this change does not implement that path.

Without published fields, legacy configuration keeps its suffix-based protocol
selection. Memory remains off unless the route enables its original memory
mode. Existing no-store, tenant/consumer/session isolation, tool-call capture,
and cache-replay suppression continue to apply. Background extraction and
embedding stay in Console and retain their separate internal costs.

## Request Flow

1. Header phase uses the matched route and published text protocol for managed
   bindings, or configured suffixes for legacy bindings, then checks JSON
   content type, request body, tenant identity, and consumer identity.
   Unsupported or unauthenticated requests continue unchanged.
2. Body phase uses the selected protocol adapter, extracts the latest user
   content, stores request facts, and builds a stable request digest.
3. Recent-memory phase reads Console-materialized Redis sorted set key
   `memory:recent:<tenant>:<consumer>` using `ZREVRANGE 0 <recent_window_turns-1>`.
   Invalid JSON, unsupported roles, oversized records, or Redis failures are
   treated as misses.
4. Console assemble phase runs only for `digest` and `semantic` modes. The
   plugin sends safe request facts to `POST /internal/memory/assemble` and
   applies valid `inject`, `recent_only`, `skip`, or `bypass` decisions.
5. Replacement phase rewrites the request body only after memory assembly is
   valid. Parse, assemble, validation, and replacement failures continue
   upstream with the original request body.

## Response Capture and MemoryEvent

For non-streaming responses, the plugin captures assistant content, status,
usage, finish reason, and tool-call presence from the configured paths. For
streaming SSE responses, it accumulates content across split chunks, supports
`\r\n`, `\r`, and `\n` line endings, handles `[DONE]`, and ignores role-only
chunks.

After the final response chunk, the plugin emits exactly one JSON
`MemoryEvent` with safe facts such as schema version, event id, idempotency key,
tenant, consumer, request id, path, digest, route, model, memory mode, policy
version, response status, stream flag, tool-call flag, usage, finish reason,
timestamps, plugin version, and no-store state.

Raw `user_content` and `assistant_content` are included only when response
capture is enabled, no-store is absent, parsing succeeded, the content is safe,
and the response status is eligible. Disabled capture, no-store, parse failure,
unsafe content, and ineligible status preserve safe facts while omitting raw
content.

## Fail-Open and Privacy

`ai-memory` is fail-open. Redis recent read failures, Console assemble failures,
request body replacement errors, response parsing errors, and Redis Stream
`XADD` dispatch failures do not change or block the user response.

The plugin does not log raw prompts, raw answers, credentials, authorization
headers, Redis passwords, or internal bearer tokens. Redis Stream dispatch
failure logs contain only safe diagnostics such as request id, stream name,
status, and fixed reason codes.

## Ordering and Isolation

Run `ai-memory` after identity and quota plugins so trusted tenant and consumer
identity are available. Run it before `ai-cache` and `ai-proxy` when memory
injection can change the model request.

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

Keep `ai-memory` isolated from `ai-cache`: memory events use `memory:events`,
recent state uses `memory:recent`, MemoryEvent payload schemas are separate
from cache payload schemas, and Console owns any memory vector namespace,
retention, deletion, management API, authorization, and repair policy.

## Cache and Memory Isolation

| Concern | `ai-memory` boundary |
| --- | --- |
| Redis Stream | Uses `memory:events`; never writes cache streams. |
| Redis key prefix | Reads Console materialized recent memory under `memory:recent`; never reads or writes cache key prefixes. |
| Vector namespace | Does not create, read, or mutate vector namespaces; Console owns memory vector indexing. |
| Payload schema | Emits `MemoryEvent`; never reuses cache request, response, replay, or settlement payload schemas. |
| Retention and deletion | Does not enforce retention or deletion; Console owns memory lifecycle policy. |
| Management APIs | Exposes no memory management APIs from the gateway. |
| Authorization checks | Trusts upstream identity headers and leaves memory authorization checks to Console APIs and workers. |

When `ai-memory` and `ai-cache` are both enabled, use a conservative cache
policy such as consumer-scoped cache, memory-policy or digest-aware cache keys,
or cache bypass until Console materializes memory-aware replay records.

For managed native identity, `capture_body_basis: false` consumes and validates the original basis before auxiliary lookup or injection; a missing or mismatched basis bypasses memory without legacy fallback. Trusted publication assigns capture to the earliest actual history/memory rewriter using exact artifact phase/order, not incoming header presence. A later mapper must explicitly consume that basis. Independent legacy memory mode keeps its existing behavior.

Managed consume validation uses the existing 100 MiB runtime HTTP limit after history injection. It retains the original basis unchanged and applies the shared strict parser without imposing the single canonical document byte budget on the combined conversation.
