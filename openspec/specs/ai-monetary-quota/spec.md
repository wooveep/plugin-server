# ai-monetary-quota Specification

## Purpose
Define admission-only monetary quota enforcement for `ai-quota`, including request admission from read-only Redis balance hot data, missing-balance policy, and external ownership boundaries with Console or billing-service.
## Requirements
### Requirement: Monetary quota configuration

`ai-quota` SHALL support admission-only configuration using Redis connection
settings, `quota_scope`, `provider`, `tenant_header`, `consumer_header`,
`balance_key_template`, `path_scope`, `enable_path_suffixes`,
`enable_methods`, and `missing_balance_policy`.

Missing `path_scope` SHALL resolve to `legacy_suffixes` and preserve existing
default and explicit suffix behavior. `bound_target` SHALL use the Higress
binding target as its default request set without requiring a suffix list.
Explicit non-empty suffix or method filters SHALL only narrow that target by
intersection.

#### Scenario: Default admission configuration is applied

- **WHEN** ai-quota is configured with Redis settings and omits optional
  admission fields
- **THEN** `quota_scope`, `tenant_header`, `consumer_header`,
  `balance_key_template`, `path_scope`, and `missing_balance_policy` SHALL
  default to `global`, `x-mse-tenant`, `x-mse-consumer`,
  `billing:balance:{tenant}:{quota_scope}:{consumer}`, `legacy_suffixes`, and
  `deny`
- **AND** existing legacy default suffix behavior SHALL be retained.

#### Scenario: Route-level quota scope is selected

- **WHEN** a matching Higress WasmPlugin rule provides `quota_scope` and
  `provider`
- **THEN** ai-quota SHALL use those matched values for that request's
  admission context.

#### Scenario: Bound target omits filters

- **WHEN** a rule selects `path_scope: bound_target` and omits
  `enable_path_suffixes` and `enable_methods`
- **THEN** ai-quota SHALL accept the configuration
- **AND** it SHALL perform admission for every request already selected by
  that Higress target.

#### Scenario: Bound target has advanced filters

- **WHEN** a bound-target rule supplies a non-empty suffix or method filter
- **THEN** ai-quota SHALL perform admission only for the intersection of the
  target and every supplied filter
- **AND** the filter SHALL NOT expand the target.

### Requirement: Legacy quota management is removed

`ai-quota` SHALL NOT expose gateway-hosted quota management endpoints or require admin quota configuration.

#### Scenario: Admin quota paths are not handled

- **WHEN** a request path ends with `/quota`, `/quota/refresh`, or `/quota/delta`
- **THEN** `ai-quota` SHALL NOT execute quota query, refresh, delta, admin identity, or request-body quota mutation logic

#### Scenario: Admin configuration is absent

- **WHEN** `ai-quota` configuration omits `admin_consumer` and `admin_path`
- **THEN** the plugin SHALL parse configuration successfully if the required Redis settings are valid

### Requirement: Request admission uses Redis monetary balance

`ai-quota` SHALL perform request admission for every in-scope request by
reading a Redis monetary balance key built from tenant, quota scope, and
consumer identity. Legacy instances SHALL retain existing path eligibility;
bound-target instances SHALL use the Higress target plus explicit non-empty
filter intersections.

#### Scenario: Positive balance allows request

- **WHEN** an in-scope request has tenant and consumer headers and Redis
  returns a balance greater than `0`
- **THEN** ai-quota SHALL allow the request to continue.

#### Scenario: Non-positive balance denies request

- **WHEN** an in-scope request has tenant and consumer headers and Redis
  returns a balance less than or equal to `0`
- **THEN** ai-quota SHALL deny the request.

#### Scenario: Missing balance follows policy

- **WHEN** an in-scope request has tenant and consumer headers and the Redis
  balance key is missing
- **THEN** ai-quota SHALL deny the request when `missing_balance_policy` is
  `deny`
- **AND** it SHALL allow the request when `missing_balance_policy` is `allow`.

#### Scenario: Missing identity denies request

- **WHEN** an in-scope request lacks the configured tenant or consumer header
- **THEN** ai-quota SHALL deny the request.

#### Scenario: Bound provider-native request is admitted

- **WHEN** a provider-native operation is matched by a bound-target quota
  instance and satisfies every explicit filter
- **THEN** ai-quota SHALL apply the same Redis admission decision without
  requiring a recognized AI API suffix or response usage shape.

### Requirement: Billing-service remains the account source of truth

`ai-quota` SHALL treat Redis balance hot data as read-only request-admission
input. Console or billing-service SHALL own authoritative accounts,
settlement, statements, reconciliation, and rebuilding that hot data.

#### Scenario: Redis balance hot data is rebuilt externally

- **WHEN** Redis balance hot data is lost or stale
- **THEN** Console or billing-service SHALL rebuild it from authoritative
  account state
- **AND** ai-quota SHALL apply `missing_balance_policy` until the balance key
  is available.

### Requirement: Quota Enforcement Is Admission-Only

The request-header admission result SHALL be ai-quota's final account
enforcement action for that request. Redis balance input SHALL be read-only to
the plugin, and response lifecycle handling SHALL NOT add another account
enforcement action.

#### Scenario: Admitted request completes

- **WHEN** an admitted request reaches any non-streaming or streaming response
  completion path
- **THEN** the earlier admission result SHALL remain final
- **AND** later response facts SHALL NOT cause ai-quota to mutate account
  state.

#### Scenario: Billing or statistics plugin is disabled

- **WHEN** ai-billing, ai-statistics, or both are disabled for a route
- **THEN** ai-quota SHALL continue to make its independent admission decision
  from Redis balance hot data
- **AND** it SHALL NOT depend on either plugin's private runtime state.

### Requirement: AI Quota Admission Release

The authoritative ai-quota extension version SHALL advance to `1.0.2` and
SHALL implement admission-only bound-target enforcement.

#### Scenario: Version 1.0.2 is packaged

- **WHEN** ai-quota `1.0.2` is built
- **THEN** its versioned Wasm artifact, resource metadata, schema, examples,
  and bilingual documentation SHALL identify `1.0.2`
- **AND** independent quota tests SHALL verify legacy scope, bound target,
  advanced filters, and Redis admission-only behavior.
