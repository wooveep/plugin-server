## MODIFIED Requirements

### Requirement: Billing plugin configuration

`ai-billing` SHALL support either one valid global `redis_stream` delivery
target with inheriting rules or a dormant empty base with complete scoped
rules. It SHALL support `event_kind`, `quota_scope`, `provider`, `cost_source`,
`worker_kind`, `bill_customer`, `tenant_header`, `consumer_header`,
`path_scope`, `enable_path_suffixes`, `enable_methods`, and `fail_policy`.

`redis_stream` SHALL contain required `service_name` and optional
`service_port`, `username`, `password`, `database`, `timeout`, and `stream`.
Rules SHALL inherit a configured global target and SHALL NOT replace it. When
the base is dormant, every active rule SHALL contain and validate its own
complete `redis_stream` target. Global and rule-level `billing_service` SHALL
always be rejected.

Missing `path_scope` SHALL resolve to `legacy_suffixes`. In
`legacy_suffixes`, ai-billing SHALL retain its existing default and explicit
suffix behavior. In `bound_target`, the Higress binding target SHALL define
the default request set; absent suffix and method lists SHALL be accepted, and
explicit non-empty filters SHALL only narrow that target by intersection.

#### Scenario: Global Redis Stream target is configured

- **WHEN** global config contains `redis_stream.service_name`
- **THEN** ai-billing SHALL initialize a Redis client for that service
- **AND** omitted `service_port`, `database`, `timeout`, and `stream` SHALL
  default to `6379`, `0`, `500`, and `billing:events`
- **AND** optional `username` and `password` SHALL be used only for Redis
  connection authentication.

#### Scenario: Non-empty incomplete base is rejected

- **WHEN** non-empty global config omits `redis_stream` or provides an empty
  `redis_stream.service_name`
- **THEN** ai-billing SHALL reject plugin initialization with a clear
  Redis-target configuration error.

#### Scenario: Empty base is dormant

- **WHEN** Higress initializes ai-billing with an empty base configuration
- **THEN** plugin startup SHALL succeed
- **AND** the base configuration SHALL not read bodies, initialize billing
  request state, or emit events.

#### Scenario: Removed global billing service is rejected

- **WHEN** global config contains `billing_service`
- **THEN** ai-billing SHALL reject plugin initialization
- **AND** it SHALL direct the operator to use global `redis_stream`.

#### Scenario: Rules inherit Redis Stream target

- **WHEN** global config contains a valid `redis_stream` and a matching rule
  omits delivery configuration
- **THEN** the rule SHALL use the global Redis target and stream.

#### Scenario: Rules cannot replace delivery target

- **WHEN** global config contains a valid `redis_stream` and a matching rule
  contains `redis_stream` or `billing_service`
- **THEN** ai-billing SHALL reject that rule configuration
- **AND** route data SHALL NOT choose a Redis service, credential, database, or
  stream.

#### Scenario: Dormant base uses complete scoped rule

- **WHEN** an empty base is combined with a matching rule that contains valid
  `redis_stream.service_name` and `path_scope: bound_target`
- **THEN** the rule SHALL initialize its Redis client
- **AND** it SHALL process only requests selected by that Higress match rule.

#### Scenario: Dormant base rejects incomplete scoped rule

- **WHEN** an empty base is combined with a matching rule that omits a valid
  `redis_stream.service_name`
- **THEN** ai-billing SHALL reject that rule configuration.

#### Scenario: Event kind and business attribution are configured

- **WHEN** global or matched rule config supplies supported business fields
- **THEN** `event_kind` SHALL accept `usage` and `internal_cost`
- **AND** the legacy `customer_usage` value SHALL normalize to `usage`
- **AND** `cost_source`, `worker_kind`, and `bill_customer` SHALL remain
  configuration-derived internal-cost metadata
- **AND** omitted business fields SHALL inherit from global config.

#### Scenario: Default request configuration is applied

- **WHEN** optional request fields are omitted
- **THEN** `quota_scope`, `provider`, `tenant_header`, `consumer_header`,
  `path_scope`, and `fail_policy` SHALL default to `global`, `default`,
  `x-mse-tenant`, `x-mse-consumer`, `legacy_suffixes`, and `open`
- **AND** legacy default suffix behavior SHALL be retained.

#### Scenario: Rule business fields override global defaults

- **WHEN** a matching rule supplies `event_kind`, `quota_scope`, `provider`,
  `cost_source`, `worker_kind`, `bill_customer`, header names, eligible path
  filters, or `fail_policy`
- **THEN** the explicit rule values SHALL apply to that request
- **AND** all omitted fields SHALL inherit from global config.

#### Scenario: Bound target omits filters

- **WHEN** a matching rule selects `path_scope: bound_target` and omits
  `enable_path_suffixes` and `enable_methods`
- **THEN** ai-billing SHALL accept the configuration
- **AND** it SHALL process every request already selected by that Higress
  binding target.

#### Scenario: Bound target has advanced filters

- **WHEN** a bound-target rule supplies a non-empty suffix or method filter
- **THEN** ai-billing SHALL process only the intersection of the binding
  target and every supplied filter
- **AND** the filters SHALL NOT expand the target.

### Requirement: AI Billing Routing Release

The authoritative ai-billing extension version SHALL advance to `0.1.7` and
SHALL combine Redis Stream delivery, trusted internal attribution, exact
response correlation, bound-target scope, and truthful opaque-response usage.

#### Scenario: Version 0.1.7 is packaged

- **WHEN** ai-billing `0.1.7` is built
- **THEN** its versioned Wasm artifact, resource metadata, schema, examples,
  and bilingual documentation SHALL identify `0.1.7`
- **AND** independent tests SHALL cover the complete release contract.
