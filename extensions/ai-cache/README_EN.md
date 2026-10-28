---
title: AI Cache
keywords: [higress,ai cache]
description: ai-cache thin gateway plugin configuration reference
---

## Function Description

`ai-cache` is a thin Higress Wasm-Go gateway cache plugin. In the online request
path it only reads Console-materialized cache records, validates them, replays
OpenAI-compatible responses, captures response facts, and emits `CacheEvent`
messages to Redis Stream.

The gateway plugin does not generate embeddings, run vector search, write
PostgreSQL, calculate prices, settle billing, repair or delete durable state, or
own upstream model credentials. Console and backend workers own materialization,
semantic lookup, embeddings, vector indexing, pricing, settlement, repair,
deletion, and reconciliation.

**Tips**

When the request carries `x-higress-skip-ai-cache: on`, the current request does
not use cached content and is forwarded to the upstream service. The response
from that request is not emitted as a cache event.

## Runtime Properties

Plugin execution phase: `default`
Plugin execution priority: `250`

## Runtime Responsibilities

Managed routes choose an existing adapter from trusted publication fields
`client_protocol` and `operation_kind`, independently of URL aliases, client
protocol headers, or later `ai-proxy` context. Omitting both retains legacy path
detection. Supplying just one, an empty value, or a non-string rejects the
configuration. Supported client protocols are `openai_chat`, `openai_responses`,
and `anthropic_messages` for `text_generate`; other protocols and operations
bypass cache without enabling it or falling back to URL detection. Explicit
path suffixes remain an intersection. Managed publication should clear inherited
legacy suffixes while retaining the route's cache policy, no-store, bypass,
identity isolation, policy fingerprints, and replay restrictions.

`text_generate` covers both text input and multimodal input with text output.
The full JSON request, including image URLs, file IDs, inline attachments, and
their other fields, participates in the request digest. Extracted query text
does not replace that digest. The plugin neither downloads nor rewrites
attachments; external content versions or digests must come from the original
request. Non-text output and asynchronous tasks follow their published operation
and bypass this text-output cache.

The plugin is responsible for:

- Applying request gating by JSON `content-type`, `route_policy.enabled_path_suffixes`, and no-store signals.
- Extracting identity and request facts from `tenant_header`, `consumer_header`, `session_header`, and request context.
- Building a materialized lookup key from tenant, consumer, route, model, scope, policy, and request digest.
- Reading Console-generated structured replay records from Redis.
- Optionally calling Console `/internal/cache/lookup` with a short timeout after Redis miss.
- Validating materialized record schema, expiration, scope, tenant, consumer, route, model, policy, and digest.
- Replaying only safe structured responses or stream chunks on hit.
- Capturing eligible upstream response facts and emitting `CacheEvent` after response completion.
- Failing open on Redis, Console, parsing, validation, response capture, and event delivery failures.

The plugin is not responsible for:

- Online embedding, rerank, or vector search.
- PostgreSQL persistence, materialized record generation, repair, or deletion.
- Provider pricing, discounts, balances, billing settlement, or reconciliation.
- Backend model cost attribution.
- Cache management APIs or management-plane authorization.

## Native Publication Identity

A managed rule may declare exactly five static `native_identity` fields:
`release_id`, `target_id`, `route_id`, `policy_id`, and `policy_version`.
Release and target remain original opaque IDs; route and policy are canonical
non-nil UUIDs. The version is a positive i64 JSON integer, and the configured
`cache_policy_version` is its canonical decimal string. This declaration neither
enables cache nor supplies a caller, role, or authorization. Invalid or partial
static tuples and extra fields reject the configuration.

Preceding native key-auth clears client fields and installs the original account,
API key, credential generation, tenant, and principal. Cache reads these once with
`balanceidentity.Read` during request headers and retains that snapshot for both
Console lookup and CacheEvent. Missing native caller facts bypass cache without
falling back to legacy headers. Keys and payloads use the same tenant UUID,
principal UUID, frozen route UUID, and original request public model. Both payloads
add the identical `native_identity.caller`. Response processing never re-reads the
authentication headers or infers identity from Envoy route names, release prefixes,
client labels, or later ai-proxy context. Rules without native identity retain the
existing legacy behavior.

The effective memory-aware cache policy version keeps the original memory
policy/digest fingerprint, separately from the static native policy revision.
The Console consumer must verify both against the frozen policy. Native identity
connectivity does not close the separate multimodal semantic-fallback attachment
isolation work.

## Configuration

The public `ai-cache` configuration keeps only the thin cache path: materialized
Redis lookup, optional Console lookup, CacheEvent Redis Stream, route policy,
identity headers, cache scope, policy version, and fail-open policy. Legacy
`vector`, `embedding`, `cache`, GJSON extraction paths, and response templates
are no longer exposed as resource plugin configuration.

```yaml
materialized_lookup:
  redis:
    enabled: true
    service_name: redis-stack-server.dns
    service_port: 6379
    key_prefix: cache:materialized:
    timeout: 80
console_lookup:
  enabled: true
  service_name: modelfusion-console-api.dns
  service_port: 80
  path: /internal/cache/lookup
  timeout: 120
redis_stream:
  enabled: true
  service_name: redis-stack-server.dns
  service_port: 6379
  stream: cache:events
  field: event
  timeout: 120
route_policy:
  enable_redis_lookup: true
  enable_console_lookup: true
  enable_replay: true
  enable_bypass: false
  enabled_path_suffixes:
  - /v1/chat/completions
  memory:
    enabled: true
    cache_mode: policy_digest
    policy_version: memory-policy-v1
tenant_header: x-mse-tenant
consumer_header: x-mse-consumer
session_header: x-openclaw-session-key
cache_scope: consumer
cache_policy_version: cache-policy-v1
fail_policy: open
```

## Configuration Fields

| Name | Type | Default | Description |
| --- | --- | --- | --- |
| client_protocol | string | - | Trusted published client protocol, paired with operation_kind; openai_chat, openai_responses, or anthropic_messages |
| operation_kind | string | - | Trusted published operation; text_generate uses text cache and other operations bypass |
| materialized_lookup.redis.enabled | bool | false | Enables Redis materialized replay lookup |
| materialized_lookup.redis.service_name | string | - | Redis service for materialized records |
| materialized_lookup.redis.service_port | int | 6379 | Redis service port |
| materialized_lookup.redis.key_prefix | string | `cache:materialized:` | Prefix for materialized record keys |
| materialized_lookup.redis.timeout | int | 80 | Redis lookup timeout in milliseconds |
| console_lookup.enabled | bool | false | Enables optional Console lookup after Redis miss |
| console_lookup.service_name | string | - | Console service name |
| console_lookup.service_port | int | 80 | Console service port |
| console_lookup.path | string | `/internal/cache/lookup` | Console lookup path |
| console_lookup.bearer_token | string | - | Existing Console Bearer credential, injected by a trusted secret source and hidden as sensitive configuration |
| console_lookup.timeout | int | 120 | Console lookup timeout in milliseconds |
| redis_stream.enabled | bool | false | Enables `CacheEvent` delivery |
| redis_stream.service_name | string | - | Redis service for events |
| redis_stream.service_port | int | 6379 | Redis event service port |
| redis_stream.stream | string | `cache:events` | Redis Stream name |
| redis_stream.field | string | `event` | Redis Stream field name |
| redis_stream.timeout | int | 120 | Redis event timeout in milliseconds |
| route_policy.enable_redis_lookup | bool | true | Allows Redis materialized lookup |
| route_policy.enable_console_lookup | bool | `console_lookup.enabled` | Allows Console fallback lookup |
| route_policy.enable_replay | bool | true | Allows local replay after validated hit |
| route_policy.enable_bypass | bool | false | Bypasses thin cache for the route |
| route_policy.enabled_path_suffixes | []string | nil | Request path suffixes to cache; empty means all paths |
| route_policy.memory.enabled | bool | false | Enables memory-aware cache policy |
| route_policy.memory.cache_mode | string | `policy_digest` | `policy_digest` includes memory policy and digest in the cache key; `bypass` skips cache |
| route_policy.memory.policy_version | string | - | Memory policy version used in memory-aware cache keys |
| tenant_header | string | `x-mse-tenant` | Tenant identity header |
| consumer_header | string | `x-mse-consumer` | Consumer identity header |
| session_header | string | `x-openclaw-session-key` | Session identity header |
| cache_scope | string | `tenant` | `tenant` or `consumer`; memory-aware routes should use `consumer` |
| cache_policy_version | string | - | Required when thin cache lookup, replay, or event emission is enabled |
| fail_policy | string | `open` | Current production behavior is fail-open |

`policy_digest` receives the digest produced by `ai-memory` through the private
Proxy-Wasm filter-state property `mse_ai_memory_context_digest`, not an HTTP
header. Both plugins remove the deprecated `x-mse-memory-digest` request header.

## Materialized Replay Records

Materialized records are Console-owned Redis values using
`schema_version: ai-cache.materialized.v1`. A record must include tenant,
optional consumer, route, model, cache scope, cache policy version, request
digest, soft and hard expirations, `usage`, `finish_reason`, and either
`response` for non-stream replay or `stream_replayable: true` plus
`stream_chunks` for stream replay.

The plugin rejects unsupported schema versions, expired records, scope, tenant,
consumer, route, model, policy, digest, and malformed payload mismatches, then
continues the upstream request.

## CacheEvent

After an upstream response finishes, the plugin writes:

```text
XADD cache:events * event <CacheEvent JSON>
```

The event contains identity, route, model, request digest, policy, status code,
stream flags, gate flags, timing, and plugin version. When policy allows it, the
event may include user content, assistant content, usage, finish reason, and
safe provider or runtime facts.

The event must not include Authorization headers, API keys, internal bearer
tokens, Redis credentials, upstream provider credentials, or sensitive raw
content. Redis Stream delivery failures keep the user response unchanged.

## Plugin Order and Isolation

When `ai-memory` affects the answer, run `ai-memory` before `ai-cache`.
Recommended cache policy options are consumer-scoped cache, `policy_digest`
keys that include memory policy version and assembled memory digest, or
`bypass` until Console materializes memory-aware replay records.

`ai-cache` and `ai-memory` backend state must remain isolated: separate
PostgreSQL tables, Redis streams, Redis key prefixes, vector namespaces, payload
schemas, retention/deletion policies, management APIs, and authorization
boundaries.

## Billing Facts

On cache replay, `ai-cache` sets trusted gateway facts such as
`upstream_invoked=false` for `ai-billing`. Real upstream provider calls emit
`upstream_invoked=true`. These facts are not added to the user-visible OpenAI
response body, and user-supplied request or response body fields cannot control
them.
