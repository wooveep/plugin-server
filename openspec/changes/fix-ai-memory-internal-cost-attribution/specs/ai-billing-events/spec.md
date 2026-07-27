## ADDED Requirements

### Requirement: Trusted Internal Cost Invocation Attribution

`ai-billing` SHALL preserve a complete bounded invocation-level `cost_source`
and `worker_kind` pair only when the matched plugin configuration is already
`event_kind=internal_cost`.

The plugin SHALL treat request attribution as event metadata only. Request
headers SHALL NOT enable internal-cost mode, set `bill_customer`, or alter
customer-route settlement.

#### Scenario: Complete internal pair overrides route fallback

- **WHEN** a request matches an `internal_cost` rule and supplies complete valid
  `x-ai-billing-cost-source` and `x-ai-billing-worker-kind` values
- **THEN** the emitted event SHALL contain those invocation-level values
- **AND** event kind and customer billing SHALL remain configuration-derived.

#### Scenario: Both headers absent use route fallback

- **WHEN** a request matches an `internal_cost` rule and supplies neither
  attribution header
- **THEN** the emitted event SHALL retain the configured `cost_source` and
  `worker_kind`.

#### Scenario: Partial pair suppresses event delivery

- **WHEN** an internal-cost request supplies only one attribution header
- **THEN** the plugin SHALL suppress billing-event delivery for that request
- **AND** it SHALL continue model traffic
- **AND** it SHALL NOT log the submitted value.

#### Scenario: Malformed pair suppresses event delivery

- **WHEN** either attribution value is empty after trimming, exceeds 128 bytes,
  or contains characters outside lowercase ASCII letters, digits, `.`, `-`,
  and `_`
- **THEN** the plugin SHALL suppress billing-event delivery without blocking
  model traffic or exposing the submitted value.

#### Scenario: Customer rule ignores internal attribution headers

- **WHEN** a request matches an ordinary customer billing rule and supplies
  internal-cost attribution headers
- **THEN** the plugin SHALL NOT classify the request as internal cost
- **AND** it SHALL retain customer-rule event semantics.

### Requirement: AI Billing Internal Attribution Release

The `ai-billing` extension SHALL publish version `0.1.4` with bilingual
documentation for the paired trusted-header contract and a buildable versioned
Wasm artifact.

#### Scenario: Versioned artifact builds

- **WHEN** the repository builds `ai-billing` with plugin version `0.1.4`
- **THEN** it SHALL produce a non-empty
  `build/plugins/ai-billing/0.1.4/plugin.wasm` artifact.

#### Scenario: Documentation explains trust boundary

- **WHEN** an operator reads either `ai-billing` README
- **THEN** it SHALL explain that both attribution headers are required for an
  invocation override
- **AND** it SHALL explain that the matched internal-cost configuration remains
  authoritative.
