## MODIFIED Requirements

### Requirement: ai-quota resource documentation reflects monetary quota behavior

The `ai-quota` resource documentation SHALL describe admission from read-only
Redis balance hot data, backward-compatible path scope, and external account
ownership.

#### Scenario: Admission-only quota overview is documented

- **WHEN** an operator reads either ai-quota resource README
- **THEN** it SHALL explain the request-header Redis balance decision,
  `legacy_suffixes`, `bound_target`, and explicit filter intersections
- **AND** it SHALL identify that admission decision as the plugin's final
  account enforcement action for a request.

#### Scenario: Removed gateway management surface is absent

- **WHEN** an operator reads the ai-quota resource documentation
- **THEN** it SHALL NOT present `/quota`, `/quota/refresh`, `/quota/delta`,
  `admin_consumer`, `admin_path`, or `redis_key_prefix` as supported
  configuration or management interfaces.

#### Scenario: External account ownership is documented

- **WHEN** an operator reads either ai-quota resource README
- **THEN** it SHALL state that Console or billing-service owns authoritative
  accounts, settlement, statements, reconciliation, and Redis hot-data
  rebuilds
- **AND** it SHALL state that ai-quota treats balance hot data as read-only.

### Requirement: ai-quota resource schema matches monetary quota configuration

The ai-quota resource `spec.yaml` SHALL expose only the current admission
configuration, backward-compatible path scope, and truthful examples.

#### Scenario: Admission and scope fields are present

- **WHEN** `resources/plugins/ai-quota/spec.yaml` is inspected
- **THEN** its current schema SHALL include `redis`, `quota_scope`, `provider`,
  `tenant_header`, `consumer_header`, `balance_key_template`, `path_scope`,
  `enable_path_suffixes`, `enable_methods`, and `missing_balance_policy`
- **AND** `path_scope` SHALL define `legacy_suffixes` and `bound_target` with
  legacy compatibility semantics.

#### Scenario: Redis connection fields are complete

- **WHEN** the ai-quota Redis schema is inspected
- **THEN** it SHALL include `service_name`, `service_port`, `username`,
  `password`, `timeout`, and `database`
- **AND** `service_name` SHALL be required and credential examples SHALL use
  placeholders.

#### Scenario: Bound-target example omits synthesized filters

- **WHEN** an ai-quota example selects `bound_target` without advanced
  narrowing
- **THEN** it SHALL omit `enable_path_suffixes` and `enable_methods`
- **AND** any non-empty filter example SHALL be described as an intersection
  with the binding target.

### Requirement: ai-billing resource package is generated

The resource catalog SHALL include complete Chinese and English ai-billing
documentation and schema matching the global Redis Stream delivery and event
contracts.

#### Scenario: ai-billing resource files exist

- **WHEN** the resource catalog is inspected
- **THEN** `resources/plugins/ai-billing/README.md`,
  `resources/plugins/ai-billing/README_EN.md`, and
  `resources/plugins/ai-billing/spec.yaml` SHALL exist.

#### Scenario: Redis Stream delivery is documented

- **WHEN** an operator reads either ai-billing resource README
- **THEN** it SHALL explain global `redis_stream` delivery with
  `XADD <stream> * event <json>` and fail-open behavior
- **AND** it SHALL state that rules inherit and cannot replace the global
  target
- **AND** it SHALL state that removed `billing_service` input is rejected.

#### Scenario: Complete billing event surface is documented

- **WHEN** an operator reads either ai-billing resource README
- **THEN** it SHALL document `event_id`, `event_kind`, `cost_source`,
  `worker_kind`, `source_job`, `bill_customer`, `idempotency_key`,
  `request_id`, `tenant`, `consumer`, `route`, `provider`, `quota_scope`,
  `model`, `request_path`, `status_code`, `usage`, `usage_missing`,
  `usage_source`, `start_time_ms`, `end_time_ms`, `is_stream`, `cluster`,
  optional `price_version`, and trusted `upstream_invoked`
- **AND** it SHALL explain event-kind-specific omission and truthful
  missing-usage behavior.

#### Scenario: Event documentation excludes unsupported fields and secrets

- **WHEN** an operator reads either ai-billing resource README
- **THEN** it SHALL NOT claim emission of top-level `input_tokens`,
  `output_tokens`, `total_tokens`, or `gateway_calculated_cost`
- **AND** it SHALL NOT expose raw API keys or real Redis credentials.

#### Scenario: Inheritance and path scope are documented

- **WHEN** an operator reads either ai-billing resource README
- **THEN** it SHALL explain global shared defaults and allowed route business
  overrides
- **AND** it SHALL explain legacy suffix behavior, bound-target scope without
  synthesized filters, and non-empty filter intersections.

#### Scenario: Trusted internal attribution is documented completely

- **WHEN** an operator reads either ai-billing resource README
- **THEN** it SHALL explain that a complete valid paired header overrides
  configured internal-cost fallback
- **AND** absent pairs use configured fallback
- **AND** partial, empty, oversized, or invalid-character pairs suppress event
  delivery while model traffic continues without submitted-value leakage
- **AND** ordinary customer rules ignore those headers for classification.

#### Scenario: Exact response correlation lifecycle is documented

- **WHEN** an operator reads either ai-billing resource README
- **THEN** it SHALL identify `x-ocmf-gateway-request-id` as the exact internal
  response header
- **AND** it SHALL document replacement on non-streaming, streaming, and
  gateway-local/cache responses
- **AND** it SHALL document removal, without invention, when canonical request
  context is absent.

#### Scenario: Plugin independence is documented

- **WHEN** an operator reads the ai-billing resource documentation
- **THEN** it SHALL state that ai-billing, ai-quota, and ai-statistics can be
  deployed independently without sharing private runtime state.

### Requirement: ai-billing resource schema matches plugin configuration

The ai-billing resource `spec.yaml` SHALL expose the current global Redis
target, complete business configuration, allowed rule overrides,
backward-compatible path scope, and truthful examples.

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

#### Scenario: Route schema allows only supported partial overrides

- **WHEN** the ai-billing route config schema is inspected
- **THEN** it SHALL allow supported business, header, path-filter, and
  fail-policy overrides without requiring a delivery target
- **AND** it SHALL reject rule-level `redis_stream` and removed
  `billing_service`.

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

### Requirement: Resource metadata stays aligned with extension examples

Resource metadata SHALL stay aligned for ai-proxy, ai-billing, ai-quota, and
ai-statistics with authoritative extension versions, schemas, examples, and
runtime contracts.

#### Scenario: Resource and extension examples agree

- **WHEN** resource examples are compared with extension examples
- **THEN** they SHALL use equivalent fields, defaults, placeholders,
  inheritance, protocol/path-scope semantics, and route values
- **AND** the ai-billing extension example SHALL keep `redis_stream` global
  and route configuration transport-free.

#### Scenario: Resource documentation follows tested runtime

- **WHEN** resource and extension READMEs are compared with implementation
  tests
- **THEN** they SHALL agree on payloads, metrics, admission, routing,
  inheritance, and ownership boundaries
- **AND** they SHALL NOT reintroduce behavior that tests assert is absent.

#### Scenario: Resource versions match authoritative extensions

- **WHEN** the coordinated alignment check runs
- **THEN** ai-proxy, ai-billing, ai-quota, and ai-statistics resources SHALL
  identify `1.0.2`, `0.1.5`, `1.0.2`, and `1.0.1`, respectively
- **AND** any mismatch SHALL fail verification.

## ADDED Requirements

### Requirement: ai-proxy routing resources describe protocol and scope

The ai-proxy resource package and extension documentation SHALL describe
automatic adapter selection, transparent fallback, forced passthrough,
backward-compatible path scope, trusted metadata, provider URL boundaries, and
the Qwen Responses capability sentinel.

#### Scenario: Protocol and path scope are documented

- **WHEN** an operator inspects ai-proxy resources
- **THEN** schema and documentation SHALL define `protocol_mode` values `auto`
  and `passthrough`, with `auto` as compatibility default
- **AND** they SHALL define `path_scope` values `legacy_suffixes` and
  `bound_target`.

#### Scenario: Transparent fallback and passthrough are documented

- **WHEN** an operator reads either ai-proxy README
- **THEN** it SHALL explain operation/body preservation when no adapter
  matches and full transformation bypass in forced passthrough
- **AND** provider configuration SHALL remain authoritative for host,
  authentication, timeout, and failover
- **AND** route data SHALL NOT inject an absolute URL or authority.

#### Scenario: Qwen capability sentinel is documented

- **WHEN** an operator reads either ai-proxy README
- **THEN** it SHALL explain that an explicitly empty or whitespace-only Qwen
  `openai/v1/responses` path disables native Responses capability
- **AND** omitted or non-empty Qwen paths and every other provider's empty-path
  semantics SHALL remain unchanged.

### Requirement: ai-statistics resources describe safe base observability

The ai-statistics resource package and extension documentation SHALL describe
legacy/bound-target scope, advanced intersections, bounded base facts, trusted
optional enrichment, and deterministic route/path attribution.

#### Scenario: Statistics scope schema is inspected

- **WHEN** an operator inspects the ai-statistics resource schema
- **THEN** it SHALL define `path_scope`, `enable_path_suffixes`, and
  `enable_methods` with backward-compatible legacy behavior and bound-target
  intersection behavior.

#### Scenario: Bounded route and path attribution is documented

- **WHEN** an operator reads either ai-statistics README
- **THEN** it SHALL document trusted `route_name` and selected-adapter
  operation-template validation with a 128-byte maximum
- **AND** it SHALL document `route_unknown`, `provider_native_unknown`, and
  `path_unknown` fallbacks
- **AND** it SHALL prohibit raw query, dynamic request segments, credentials,
  headers, bodies, and raw-path hashes from metric attributes.

#### Scenario: Opaque operation base facts are documented

- **WHEN** model, operation, or token facts are unavailable
- **THEN** documentation SHALL state that bounded route/path attribution,
  method, status, latency, and bounded error facts are still emitted
- **AND** unavailable optional facts are omitted rather than invented.

### Requirement: Coordinated Resource Version Alignment

The four selected extension and resource version surfaces SHALL have an
automated alignment check or single-source generation mechanism.

#### Scenario: Alignment check passes

- **WHEN** the coordinated resource check runs
- **THEN** every authoritative extension version, resource `info.version`,
  metadata version, artifact URL, and documented version SHALL match.

#### Scenario: Stale duplicate version is found

- **WHEN** a checked-in duplicate differs from its authoritative extension
  version
- **THEN** verification SHALL fail
- **AND** no artifact or downstream catalog update SHALL use that release.
