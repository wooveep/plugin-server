# AI Cache Thin Plugin Design

## Summary

The `ai-cache` gateway plugin should be a lightweight online cache executor. It
should not own production semantic cache materialization, embedding generation,
vector search, persistent replay records, or final billing settlement.

The plugin reads already materialized cache results, optionally asks Console for
a short-timeout lookup over precomputed records, replays safe hits, fails open on
misses or errors, captures response facts, and emits cache events to Redis
Stream. Console workers perform the heavy asynchronous work.

## Goals

- Keep gateway latency predictable.
- Avoid default online embedding and vector search.
- Preserve OpenAI-compatible cache-hit responses.
- Support cache-hit billing facts through `upstream_invoked=false`.
- Emit enough safe event data for Console to materialize future cache hits.
- Share protocol helpers with `ai-memory` without introducing another runtime
  plugin.

## Non-Goals

- Do not write PostgreSQL from the plugin.
- Do not generate embeddings in the plugin.
- Do not call vector databases from the plugin.
- Do not calculate customer prices, discounts, upstream costs, or balances.
- Do not hold upstream provider credentials.
- Do not make event delivery failures affect the client response.

## Online Request Flow

```text
request headers
  -> extract tenant, consumer, session, request id
  -> validate content type and path
request body
  -> parse messages and current user intent
  -> build scoped request digest
  -> Redis materialized-result lookup
  -> optional Console lookup with short timeout
  -> hit: send OpenAI-compatible replay
  -> miss/error/timeout: resume upstream request
response body
  -> capture assistant text and usage when eligible
  -> emit CacheEvent to Redis Stream
```

The plugin must treat every external dependency as optional for the user
request. Redis lookup, Console lookup, and Redis Stream event write all fail open
unless a future explicit policy says otherwise.

## Redis Materialized Lookup

Redis is the plugin's fastest replay source. Keys are projected or configured
from Console policy and must include scope-relevant inputs:

- tenant
- consumer when consumer-scoped
- route
- model
- request digest
- policy version

Values are structured replay records, not plain assistant text. A valid replay
record includes:

- schema version
- OpenAI-compatible response
- usage
- finish reason
- scope
- policy version
- `soft_expires_at`
- `hard_expires_at`

The plugin must miss on invalid JSON, expired records, scope mismatch, route
mismatch, model mismatch, or policy version mismatch.

## Console Lookup

After Redis miss, the plugin may call Console `/internal/cache/lookup` when
enabled by route policy.

Rules:

- Timeout should be short, normally 50-100 ms.
- The default Console lookup reads only precomputed or materialized results.
- The plugin must not rely on Console lookup for correctness.
- Timeout, failure, or miss resumes the upstream request.
- Online embedding/vector lookup is not part of the default plugin contract.

## Cache Hit Replay

Cache hits must return OpenAI-compatible responses.

The plugin should preserve:

- user-visible model
- choices
- assistant content
- finish reason
- usage

The plugin must not add proprietary billing fields to the user response body.
Safe diagnostics may be exposed through allowed headers or logs.

On replay, the plugin must set the trusted gateway facts needed for `ai-billing`
to emit `upstream_invoked=false`.

## Cache Event

After an upstream response finishes, the plugin emits a JSON `CacheEvent` to
Redis Stream `cache:events`, field `event`.

The event includes:

- `event_id`
- `idempotency_key`
- `tenant`
- `consumer`
- `session_id`, when available
- `route`
- `model`
- `request_id`
- `request_path`
- `request_digest`
- `cache_scope`
- `cache_policy_version`
- `status_code`
- `is_stream`
- `contains_tool_calls`
- `no_store`
- `sensitive`
- `started_at_ms`
- `ended_at_ms`
- `plugin_version`

When policy permits, it may include:

- `user_content`
- `assistant_content`
- response `usage`
- `finish_reason`
- safe provider or runtime cluster facts

The plugin must not include authorization headers, API keys, internal bearer
tokens, Redis credentials, or upstream provider credentials.

## Shared Go Package

`ai-cache` should share protocol helpers with `ai-memory` through a Go package,
not a separate runtime plugin.

The shared package should cover:

- tenant and consumer extraction
- session id extraction
- OpenAI message parsing
- current user intent extraction
- request digest helpers
- streaming and non-streaming assistant text capture
- tool-call detection
- event envelope helpers
- safe logging and fail-open helpers

The package should not contain cache policy, memory policy, vector code, or
billing settlement logic.

## Interaction With ai-memory

When `ai-cache` and `ai-memory` are both enabled on a route, cache replay must
not ignore the memory input that changes the upstream model request.

Acceptable first-version policies are:

- Consumer-scoped cache using the same tenant and consumer identity as
  `ai-memory`.
- Cache keys or policy versions that include the memory policy version and
  assembled memory digest.
- Cache bypass for memory-enabled routes until Console can materialize
  memory-aware replay records.

`ai-memory` should run before cache lookup when memory is expected to affect the
answer. If a cache hit short-circuits before memory injection, the response is
not memory-aware and should be treated as an explicit route policy choice, not a
default behavior.

## Failure Policy

- Redis lookup failure: continue upstream.
- Console lookup failure or timeout: continue upstream.
- Invalid replay record: continue upstream.
- Redis Stream event failure: keep the user response unchanged and log safely.
- Response parsing failure: do not emit cacheable raw content.

## Compatibility

Existing text-only cache behavior may remain as a local compatibility path, but
Modelfusion production cache replay should use structured materialized records
from Console.

Existing direct embedding/vector provider code is not part of the production
thin-plugin path. It can remain only as legacy or experimental compatibility
until removed or moved behind explicit non-production configuration.

## Tests

Plugin tests should cover:

- Config parsing for Redis, Console lookup, stream, and event settings.
- Missing tenant or consumer fail-open behavior.
- Redis hit/miss behavior.
- Invalid, expired, or scope-mismatched materialized result misses.
- Console lookup hit, miss, timeout, and failure.
- OpenAI-compatible replay body.
- Streaming replay body where supported.
- Response capture for stream and non-stream responses.
- Tool-call, no-store, sensitive, and failed-response gates.
- CacheEvent payload shape and redaction.
- Redis Stream dispatch failure fail-open behavior.
