## Context

Base observability introduced two counters that are emitted independently of
optional usage parsing. The counter registry is cached on
`AIStatisticsConfig`, but some rule-override lifecycle paths provide a config
whose cache was not initialized by the global parse path. Reading a nil map is
safe in Go; assigning the first newly defined counter is not.

## Goals / Non-Goals

**Goals:**

- Make every counter increment safe when the cache is nil.
- Keep lazy metric definition and reuse semantics unchanged.
- Publish a traceable corrective artifact.

**Non-Goals:**

- Do not change base metric names or cardinality.
- Do not change statistics target filtering or attribution.
- Do not introduce a shared/global counter registry.

## Decisions

`AIStatisticsConfig.incrementCounter` will initialize `counterMetrics` on
demand immediately before lookup and assignment. This is the narrowest
lifecycle-independent invariant: every caller is protected, including base and
usage metrics, while already initialized maps retain their entries.

The regression will construct a config with a nil cache and call the method
twice for the same metric. It must demonstrate that the first call does not
panic and that only one cached metric entry exists afterward. Existing
proxy-wasm test plumbing verifies the counter increments.

## Risks / Trade-offs

- **[Risk] A copied config value receives a private map** → This matches existing
  value-copy behavior; the method's pointer receiver initializes the active
  config instance and does not introduce shared mutable state.
- **[Risk] Reinitialization loses counters** → Initialization occurs only when
  the map is nil, so populated caches are preserved.
- **[Trade-off] The invariant is defensive** → It intentionally protects all
  lifecycle paths instead of relying on every parser to initialize the cache.

## Migration Plan

Build and upload `ai-statistics` 1.0.2, update consumers to the new catalog
version, and refresh plugin configuration. Roll back by selecting 1.0.1, noting
that the prior nil-map panic will return on affected lifecycle paths.

## Open Questions

None.
