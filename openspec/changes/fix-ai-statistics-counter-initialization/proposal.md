## Why

`ai-statistics` 1.0.1 can reach base-metric emission with an
`AIStatisticsConfig` whose counter cache is nil. The first completed request then
panics while assigning the newly defined metric to that map, so the gateway
cannot complete the statistics lifecycle promised by the accepted observability
contract.

## What Changes

- Lazily initialize the per-config counter cache before its first write.
- Add a regression that exercises the zero-value cache through counter
  increment and proves subsequent increments reuse the defined counter.
- Release the corrected `ai-statistics` artifact and authoritative resources as
  version `1.0.2`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `ai-statistics-observability`: Require safe counter-cache initialization for
  eligible completed requests and advance the corrective release to `1.0.2`.
- `extensions-build-artifacts`: Advance the authoritative and versioned
  ai-statistics artifact path to `1.0.2`.
- `ai-plugin-resource-docs`: Keep resource metadata and bilingual documentation
  aligned with the corrected extension release.

## Impact

- `extensions/ai-statistics` metric counter initialization and regression tests.
- ai-statistics resource metadata, examples, documentation, and release
  alignment checks.
- No change to metric names, dimensions, routing scope, or request/response
  behavior.
