## MODIFIED Requirements

### Requirement: ai-quota resource documentation reflects monetary quota behavior

The resource catalog documentation for `ai-quota` SHALL describe the current
admission-only Redis monetary balance behavior and SHALL NOT claim
post-response deduction.

#### Scenario: Admission-only quota overview is documented

- **WHEN** an operator reads `resources/plugins/ai-quota/README.md` or
  `resources/plugins/ai-quota/README_EN.md`
- **THEN** the documentation SHALL explain that `ai-quota` reads Redis hot
  balances before forwarding in-scope requests
- **AND** it SHALL explain `legacy_suffixes`, `bound_target`, and explicit
  advanced filter intersections
- **AND** it SHALL state that the plugin does not parse completion usage or
  deduct balance after the response.

#### Scenario: Removed admin and settlement APIs are not presented as supported

- **WHEN** an operator reads the `ai-quota` resource documentation
- **THEN** the documentation SHALL NOT present `/quota`, `/quota/refresh`,
  `/quota/delta`, `admin_consumer`, `admin_path`, or `redis_key_prefix` as
  supported configuration or management interfaces
- **AND** it SHALL NOT present price lookup, token-usage parsing, or
  post-response balance deduction as current behavior.

#### Scenario: Billing ownership is documented

- **WHEN** an operator reads the `ai-quota` resource documentation
- **THEN** the documentation SHALL state that account balances, prices, billing
  statements, settlement, Redis rebuilds, idempotency, and reconciliation are
  owned by Console or billing-service rather than the gateway plugin.

### Requirement: ai-quota resource schema matches monetary quota configuration

The resource catalog `spec.yaml` for `ai-quota` SHALL expose the current
admission-only configuration, backward-compatible path scope, and examples.

#### Scenario: Admission and scope fields are present in schema

- **WHEN** `resources/plugins/ai-quota/spec.yaml` is inspected
- **THEN** its current config schema SHALL include `redis`, `quota_scope`,
  `provider`, `tenant_header`, `consumer_header`, `balance_key_template`,
  `path_scope`, `enable_path_suffixes`, `enable_methods`, and
  `missing_balance_policy`
- **AND** it SHALL define `path_scope` values `legacy_suffixes` and
  `bound_target` with legacy as the compatibility default.

#### Scenario: Redis schema includes database

- **WHEN** `resources/plugins/ai-quota/spec.yaml` is inspected
- **THEN** its Redis object schema SHALL include `service_name`,
  `service_port`, `username`, `password`, `timeout`, and `database`, with
  `service_name` required.

#### Scenario: Obsolete fields are not active schema controls

- **WHEN** `resources/plugins/ai-quota/spec.yaml` and its examples are
  inspected
- **THEN** they SHALL NOT present `redis_key_prefix`, `admin_consumer`,
  `admin_path`, `price_key_template`, `amount_scale`, `price_unit_tokens`,
  `missing_price_policy`, or `missing_usage_policy` as active current
  behavior
- **AND** any documented legacy acceptance SHALL explicitly identify those
  settlement fields as inert compatibility inputs.

#### Scenario: Bound-target example omits synthesized suffixes

- **WHEN** the ai-quota resource shows a `bound_target` example without an
  advanced filter
- **THEN** the example SHALL omit `enable_path_suffixes` and
  `enable_methods`
- **AND** documentation SHALL state that a non-empty filter narrows the
  binding target by intersection.

### Requirement: ai-billing resource package is generated

The resource catalog SHALL include a complete `ai-billing` resource package
that reflects provider-native bound-target processing and the current
`extensions/ai-billing` event contract.

#### Scenario: ai-billing resource files exist

- **WHEN** the resource catalog is inspected
- **THEN** `resources/plugins/ai-billing/README.md`,
  `resources/plugins/ai-billing/README_EN.md`, and
  `resources/plugins/ai-billing/spec.yaml` SHALL exist.

#### Scenario: ai-billing documentation explains event delivery

- **WHEN** an operator reads the `ai-billing` resource documentation
- **THEN** it SHALL explain request-level billing event generation,
  billing-service HTTP callout delivery, Bearer token authorization from
  `billing_service.auth_token`, fail-open behavior, current implemented event
  fields, the global-default plus rule-override configuration model, and that
  settlement and reconciliation belong to billing-service.

#### Scenario: Config inheritance and path scope are documented

- **WHEN** an operator reads `resources/plugins/ai-billing/README.md` or
  `resources/plugins/ai-billing/README_EN.md`
- **THEN** the documentation SHALL explain that `defaultConfig` contains shared
  billing-service and identity defaults, while `matchRules[].config` can
  contain route-specific overrides
- **AND** it SHALL explain legacy suffix defaults, bound-target scope with no
  synthesized suffixes, and explicit non-empty suffix or method filters as
  intersections.

#### Scenario: ai-billing event documentation matches implemented payload

- **WHEN** an operator reads the `ai-billing` resource documentation
- **THEN** it SHALL document the emitted event fields as `event_id`,
  `idempotency_key`, `request_id`, `tenant`, `consumer`, `quota_scope`,
  `route`, `provider`, `model`, `request_path`, `status_code`, `usage`,
  `usage_missing`, `usage_source`, `start_time_ms`, `end_time_ms`,
  `is_stream`, `cluster`, and optional `price_version`
- **AND** it SHALL explain that an opaque response without trustworthy usage
  emits missing usage rather than invented facts.

#### Scenario: ai-billing removed event fields are not documented as emitted

- **WHEN** an operator reads the `ai-billing` resource documentation
- **THEN** it SHALL NOT state that billing events emit top-level
  `input_tokens`, top-level `output_tokens`, top-level `total_tokens`, or
  `gateway_calculated_cost`.

#### Scenario: ai-billing independence is documented

- **WHEN** an operator reads the `ai-billing` resource documentation
- **THEN** it SHALL state that `ai-billing`, `ai-quota`, and `ai-statistics`
  can be deployed independently and do not depend on each other's private
  runtime state.

#### Scenario: Trusted attribution and correlation are documented

- **WHEN** an operator reads either ai-billing README
- **THEN** it SHALL retain the established internal-cost paired-header trust
  boundary and canonical gateway request-ID correlation behavior
- **AND** it SHALL state that path scope does not enable internal-cost mode or
  customer settlement.

### Requirement: ai-billing resource schema matches plugin configuration

The resource catalog `spec.yaml` for `ai-billing` SHALL expose the current
billing event configuration, inherited rule overrides, backward-compatible
path scope, advanced filters, and examples.

#### Scenario: Billing service fields are present in schema

- **WHEN** `resources/plugins/ai-billing/spec.yaml` is inspected
- **THEN** its config schema SHALL include
  `billing_service.service_name`, `billing_service.service_port`,
  `billing_service.path`, `billing_service.timeout`, and
  `billing_service.auth_token`.

#### Scenario: Billing event configuration fields are present in schema

- **WHEN** `resources/plugins/ai-billing/spec.yaml` is inspected
- **THEN** its config schema SHALL include `quota_scope`, `provider`,
  `tenant_header`, `consumer_header`, `path_scope`,
  `enable_path_suffixes`, `enable_methods`, and `fail_policy`
- **AND** it SHALL define `legacy_suffixes` and `bound_target`.

#### Scenario: Route config schema allows partial overrides

- **WHEN** `resources/plugins/ai-billing/spec.yaml` is inspected
- **THEN** its route config schema SHALL allow `provider`, `quota_scope`,
  `tenant_header`, `consumer_header`, `path_scope`,
  `enable_path_suffixes`, `enable_methods`, and `fail_policy` without
  requiring `billing_service`.

#### Scenario: Billing service auth token examples are placeholders

- **WHEN** `resources/plugins/ai-billing/spec.yaml` examples are inspected
- **THEN** every `billing_service.auth_token` example value SHALL use a
  placeholder such as `<shared-secret>` rather than a real secret.

#### Scenario: Billing legacy example matches inherited config model

- **WHEN** `resources/plugins/ai-billing/spec.yaml` shows a legacy-suffix
  example
- **THEN** it SHALL show global billing service and shared identity defaults
- **AND** route rules SHALL contain only route-specific values such as
  `quota_scope` and `provider`
- **AND** the documented legacy suffix defaults SHALL include
  `/v1/chat/completions`, `/v1/messages`, and `/v1/responses`.

#### Scenario: Billing bound-target example omits synthesized filters

- **WHEN** `resources/plugins/ai-billing/spec.yaml` shows a
  `path_scope: bound_target` example without advanced narrowing
- **THEN** it SHALL omit suffix and method filters
- **AND** any non-empty advanced filter example SHALL be documented as an
  intersection with the binding target.

### Requirement: Resource metadata stays aligned with extension examples

Resource catalog metadata SHALL stay aligned for ai-proxy, ai-billing,
ai-quota, and ai-statistics with the corresponding extension versions,
schemas, examples, and runtime contracts.

#### Scenario: Resource examples follow extension plugin examples

- **WHEN** resource `spec.yaml` examples are compared with the four extension
  `plugin.yaml` or equivalent examples
- **THEN** they SHALL use equivalent config field names, defaults, secret
  placeholders, protocol/path-scope semantics, and representative route-level
  values.

#### Scenario: ai-billing extension example uses recommended override shape

- **WHEN** `extensions/ai-billing/plugin.yaml` is inspected
- **THEN** it SHALL put common billing-service, identity, legacy enabled
  suffixes, and fail policy in `defaultConfig`
- **AND** route rules SHALL contain only route-specific differences such as
  quota scope and provider
- **AND** a separate bound-target example SHALL not synthesize suffixes.

#### Scenario: Resource docs follow extension README content

- **WHEN** resource README files are compared with extension README files for
  the same plugin
- **THEN** they SHALL describe the same runtime responsibilities, supported
  configuration fields, inheritance behavior, protocol/path-scope semantics,
  and operational ownership boundaries.

#### Scenario: Resource docs follow implementation tests

- **WHEN** resource documentation is compared with the four plugin
  implementations and release tests
- **THEN** it SHALL follow tested serialized payloads, metrics, admission
  behavior, and routing semantics
- **AND** it SHALL NOT reintroduce fields or behavior that tests assert are
  absent.

#### Scenario: Resource versions match authoritative extensions

- **WHEN** the coordinated release alignment check runs
- **THEN** ai-proxy resources SHALL identify `1.0.2`, ai-billing resources
  SHALL identify `0.1.5`, ai-quota resources SHALL identify `1.0.2`, and
  ai-statistics resources SHALL identify `1.0.1`
- **AND** mismatches SHALL fail verification.

## ADDED Requirements

### Requirement: ai-proxy routing resources describe protocol and scope

The ai-proxy resource package and extension documentation SHALL describe
automatic adapter selection, transparent fallback, forced passthrough,
backward-compatible path scope, trusted metadata, and provider URL boundaries.

#### Scenario: Protocol mode schema is inspected

- **WHEN** an operator inspects the ai-proxy resource schema
- **THEN** it SHALL define `protocol_mode` values `auto` and `passthrough`
  with `auto` as the compatibility default
- **AND** it SHALL define `path_scope` values `legacy_suffixes` and
  `bound_target`.

#### Scenario: Transparent fallback is documented

- **WHEN** an operator reads either ai-proxy README
- **THEN** it SHALL explain exact operation/body preservation when no adapter
  matches, provider-owned host/auth/timeout/failover, and the prohibition on
  absolute URL or authority injection.

#### Scenario: Forced passthrough is documented

- **WHEN** an operator reads either ai-proxy README
- **THEN** it SHALL explain that forced passthrough bypasses request and
  response body transformation even for a known adapter
- **AND** it SHALL not imply that provider credentials or authority become
  route-controlled.

### Requirement: ai-statistics resources describe safe base observability

The ai-statistics resource package and extension documentation SHALL describe
legacy/bound-target scope, advanced filter intersections, safe base facts, and
trusted optional enrichment.

#### Scenario: Statistics schema is inspected

- **WHEN** an operator inspects the ai-statistics resource schema
- **THEN** it SHALL define `path_scope`, `enable_path_suffixes`, and
  `enable_methods` with backward-compatible legacy semantics and bound-target
  intersection behavior.

#### Scenario: Opaque operation behavior is documented

- **WHEN** an operator reads either ai-statistics README
- **THEN** it SHALL state that route, path, method, status, latency, and bounded
  error facts are emitted even when model, operation, or token facts are
  unavailable
- **AND** it SHALL state that unavailable facts are omitted rather than
  invented.

### Requirement: Coordinated Resource Version Alignment

The resource and extension version surfaces for the four selected plugins SHALL
have an automated alignment check or a single-source generation mechanism.

#### Scenario: Alignment check passes

- **WHEN** the coordinated resource check runs
- **THEN** each extension version, resource `info.version`,
  plugin/option metadata version, example artifact URL, and documented version
  SHALL match its authoritative selected version.

#### Scenario: Stale duplicate version is found

- **WHEN** any checked-in duplicate version differs from the authoritative
  extension version
- **THEN** verification SHALL fail
- **AND** no artifact or downstream catalog update SHALL use that release.
