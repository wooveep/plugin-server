# AI Cache and AI Memory Shared Runtime Design

## Summary

`ai-cache` and `ai-memory` should use the same gateway development pattern:
thin plugins, Console-owned heavy logic, Redis Stream events, and internal AI
Routes for platform model calls.

They should share Go helper code for protocol handling, but they should not
share a runtime plugin or business storage. Shared code reduces duplicated
gateway parsing and fail-open logic without adding plugin-order coupling.

## Decision

Use a shared Go package, not a new standalone "session context" plugin.

The package may live under the wasm-go repository as a reusable helper package
such as `pkg/ai/sessionctx` or `extensions/pkg/chatctx`. The exact path should
follow local repository conventions when implementation starts.

## Shared Gateway Capabilities

The shared package should provide:

- Tenant, consumer, session, and request id extraction.
- Configurable header names with `x-mse-tenant` and `x-mse-consumer` defaults.
- OpenAI-compatible chat message parsing.
- Current user intent extraction and request digest helpers.
- Request body replacement helpers for memory injection.
- Streaming and non-streaming assistant text capture.
- Tool-call detection.
- Safe response usage extraction when present.
- Redis Stream event envelope helpers.
- Fail-open error handling helpers.
- Safe logging helpers that avoid raw secrets and authorization material.

The shared package should not know cache or memory business policy. It should
expose protocol facts and utilities; each plugin decides how to use them.

## AI Cache Use

`ai-cache` uses the shared package to:

- Extract identity and request facts.
- Build exact/materialized cache lookup keys from policy-provided inputs.
- Capture assistant response text and usage.
- Mark tool-call or no-store responses as ineligible.
- Emit `CacheEvent` to Redis Stream.

`ai-cache` does not use the shared package to perform embedding, vector lookup,
or pricing.

## AI Memory Use

`ai-memory` uses the shared package to:

- Extract identity and session facts.
- Parse current request messages.
- Inject memory messages into the request body.
- Capture assistant responses.
- Mark tool-call responses.
- Emit `MemoryEvent` to Redis Stream.

`ai-memory` does not use the shared package to summarize, embed, recall, or
persist durable memory.

## Shared Backend Pattern

Both capabilities use Console workers:

- Redis Stream ingestion.
- Idempotency checks.
- PostgreSQL authoritative persistence.
- Redis derived-state writes.
- Internal AI Route calls for platform-owned model work.
- `internal_cost` billing events for platform cost attribution.
- DLQ and repair workflows.

## Isolation Rules

Shared infrastructure does not mean shared business data.

`ai-cache` and `ai-memory` must use:

- Separate PostgreSQL tables.
- Separate Redis stream names.
- Separate Redis key prefixes.
- Separate vector collections or namespaces.
- Separate payload schemas.
- Separate retention and deletion policies.
- Separate management APIs and authorization checks.

`ai-cache` stores replayable response material. `ai-memory` stores user memory
events, digests, facts, preferences, tasks, and recall chunks.

## Internal AI Route Alignment

Both capabilities should call platform-owned models through Console-managed
internal AI Routes.

Examples:

- `ai-cache.embedding`
- `ai-cache.rerank` in a later phase
- `ai-memory.digest`
- `ai-memory.embedding`
- `ai-memory.recall_enhancement` in a later phase

Internal route calls should emit `event_kind=internal_cost`, never customer
usage statements. Internal service account consumers must not become ordinary
customer billing principals.

## Failure Policy

Both gateway plugins default to fail-open:

- Redis read failure should not block the request.
- Console internal API timeout should not block the request.
- Event delivery failure should not change the client response.
- Worker failure is handled by retry or DLQ.

The shared package should make fail-open behavior easy and consistent, but each
plugin owns the decision of whether a specific failure can safely continue.

## Testing

Shared package tests should cover:

- Header identity extraction.
- Session id extraction.
- OpenAI messages parsing.
- Request digest stability.
- Streaming response fragment accumulation.
- Tool-call detection.
- Safe usage extraction.
- Event envelope construction.
- Log redaction helpers.

Plugin tests should continue to cover plugin-specific policy and runtime
behavior.
