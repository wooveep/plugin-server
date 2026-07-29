## MODIFIED Requirements

### Requirement: ai-billing resource schema matches plugin configuration

The ai-billing resource `spec.yaml` SHALL expose the current global Redis
target, complete business configuration, allowed rule overrides,
backward-compatible path scope, scoped-only delivery, and truthful examples.
Global configuration SHALL retain its complete Redis target schema. Route
configuration SHALL expose the same `redis_stream` shape only for use with a
dormant disabled base.

#### Scenario: Global Redis Stream fields are complete

- **WHEN** `resources/plugins/ai-billing/spec.yaml` is inspected
- **THEN** global config SHALL require `redis_stream.service_name` and expose
  `service_port`, `username`, `password`, `database`, `timeout`, and `stream`
- **AND** defaults SHALL match `6379`, `0`, `500`, and `billing:events`
- **AND** examples SHALL use credential placeholders.

#### Scenario: Complete billing configuration fields are present

- **WHEN** the ai-billing schema is inspected
- **THEN** it SHALL expose `event_kind`, `quota_scope`, `provider`,
  `cost_source`, `worker_kind`, `bill_customer`, `tenant_header`,
  `consumer_header`, `path_scope`, `enable_path_suffixes`, `enable_methods`,
  and `fail_policy`
- **AND** it SHALL document `usage` and `internal_cost`, legacy
  `customer_usage` normalization, and both path-scope values.

#### Scenario: Route schema allows supported scoped configuration

- **WHEN** the ai-billing route config schema is inspected
- **THEN** it SHALL allow supported business, header, path-filter, and
  fail-policy overrides
- **AND** it SHALL expose complete `redis_stream` delivery for a dormant base
- **AND** it SHALL reject removed `billing_service`.

#### Scenario: Legacy example uses global target and rule business fields

- **WHEN** the schema shows a legacy-suffix example
- **THEN** it SHALL place Redis delivery and shared defaults in global config
- **AND** route rules SHALL contain only supported route-specific differences
- **AND** documented legacy defaults SHALL include exactly
  `/v1/chat/completions` and `/v1/messages`.

#### Scenario: Bound-target example omits synthesized filters

- **WHEN** the schema shows `path_scope: bound_target` without advanced
  narrowing
- **THEN** it SHALL omit suffix and method filters
- **AND** any non-empty filter example SHALL describe target intersection.

#### Scenario: Scoped-only example is complete

- **WHEN** the schema shows a scoped-only `bound_target` example
- **THEN** its rule SHALL contain a valid `redis_stream.service_name`
- **AND** documentation SHALL require `defaultConfigDisable: true` for the
  corresponding empty global scope
- **AND** it SHALL state that a rule cannot replace an existing global target.

### Requirement: Resource metadata stays aligned with extension examples

Resource metadata SHALL stay aligned for ai-proxy, ai-billing, ai-quota, and
ai-statistics with authoritative extension versions, schemas, examples, and
runtime contracts.

#### Scenario: Resource and extension examples agree

- **WHEN** resource examples are compared with extension examples
- **THEN** they SHALL use equivalent fields, defaults, placeholders,
  inheritance, protocol/path-scope semantics, and route values
- **AND** legacy ai-billing examples SHALL keep `redis_stream` global
- **AND** scoped-only examples SHALL keep complete delivery configuration in
  the matched rule.

#### Scenario: Resource documentation follows tested runtime

- **WHEN** resource and extension READMEs are compared with implementation
  tests
- **THEN** they SHALL agree on payloads, metrics, admission, routing,
  inheritance, and ownership boundaries
- **AND** they SHALL NOT reintroduce behavior that tests assert is absent.

#### Scenario: Resource versions match authoritative extensions

- **WHEN** the coordinated alignment check runs
- **THEN** ai-proxy, ai-billing, ai-quota, and ai-statistics resources SHALL
  identify `1.0.2`, `0.1.7`, `1.0.2`, and `1.0.2`, respectively
- **AND** any mismatch SHALL fail verification.
