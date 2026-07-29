## Context

Scoped-only Higress WasmPlugin resources require
`spec.defaultConfigDisable: true`; otherwise the plugin also processes
unmatched traffic. Higress still initializes the base Wasm context with an
empty configuration before selecting a rule. ai-billing 0.1.5 requires
`redis_stream` during that base parse and therefore cannot start safely in the
scoped-only shape.

## Goals / Non-Goals

**Goals:**

- Make an empty base configuration a safe no-op.
- Require every active matching rule to resolve a valid Redis target.
- Preserve strict rejection of partial global config and rule-level target
  replacement when a valid global target exists.

**Non-Goals:**

- Do not enable billing outside Higress match rules.
- Do not permit route users to replace an administrator-owned global target.
- Do not change billing event or fail-open delivery semantics.

## Decisions

An empty base object is parsed into a dormant `BillingConfig`. Request handling
with that config immediately disables body processing and continues without
emitting an event. A rule matched against a dormant base must contain a complete
`redis_stream` configuration and is parsed using the same validation and Redis
initialization as a normal global config.

When the base already has a Redis target, the existing inheritance behavior
remains authoritative and a rule containing `redis_stream` is still rejected.
Any non-empty base object that omits a valid Redis target remains invalid.

## Risks / Trade-offs

- **Risk: an accidentally enabled empty global config silently starts** →
  Dormant behavior is an explicit safe no-op and cannot bill traffic.
- **Risk: route config replaces a global target** → Replacement remains
  rejected whenever the global target is present.
- **Trade-off: scoped rules repeat shared delivery configuration** → The
  duplication is required by Higress to combine strict route isolation with
  valid plugin initialization.

## Migration Plan

Release and upload ai-billing 0.1.7, update the main catalog and minimum version,
project complete route rule configuration with disabled global execution, and
restart the local gateway. Rollback selects 0.1.5 and restores the legacy
globally enabled inheritance shape.

## Open Questions

None.
