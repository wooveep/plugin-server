## Why

`ai-billing` currently emits route-configured `cost_source` and `worker_kind`
for internal-cost requests even when a trusted Console worker supplies more
specific invocation attribution. Generic internal routes therefore erase the
distinction between AI Memory digest, extraction, and embedding costs.

## What Changes

- Read a complete bounded `x-ai-billing-cost-source` and
  `x-ai-billing-worker-kind` pair only for a matched `internal_cost` rule.
- Use the valid invocation pair in the emitted billing event while retaining
  configuration fallback when both headers are absent.
- Suppress event delivery for partial or malformed pairs without blocking model
  traffic or logging submitted values.
- Keep route configuration authoritative for event kind and customer billing.
- Add lifecycle tests, bilingual documentation, and the versioned `0.1.4`
  plugin artifact contract.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `ai-billing-events`: Preserve trusted invocation-level internal-cost
  attribution without allowing request headers to enable internal billing or
  customer settlement.

## Impact

Affected code is limited to `extensions/ai-billing` request-header capture,
request context, event construction, lifecycle tests, documentation, and
plugin version metadata. Redis Stream delivery, provider usage parsing,
pricing, customer settlement, event identity, and idempotency remain
unchanged.
