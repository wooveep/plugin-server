## MODIFIED Requirements

### Requirement: Monetary quota configuration

`ai-quota` SHALL support admission-only monetary quota configuration using
Redis connection settings, `quota_scope`, `provider`, `tenant_header`,
`consumer_header`, `balance_key_template`, `path_scope`,
`enable_path_suffixes`, `enable_methods`, and `missing_balance_policy`.

Missing `path_scope` SHALL resolve to `legacy_suffixes` and preserve the
existing default and explicit suffix behavior. `bound_target` SHALL use the
Higress binding target as the default request set without requiring a suffix
list; explicitly supplied non-empty suffix or method filters SHALL only narrow
that set by intersection.

Legacy `price_key_template`, `amount_scale`, `price_unit_tokens`,
`missing_price_policy`, and `missing_usage_policy` fields SHALL remain accepted as
inert compatibility inputs during the patch migration, but they SHALL NOT
enable price lookup, usage parsing, cost calculation, or post-response
deduction and SHALL NOT appear in current examples as active controls.

#### Scenario: Default admission configuration is applied

- **WHEN** `ai-quota` is configured with Redis settings and omits optional
  admission fields
- **THEN** the plugin SHALL default `quota_scope` to `global`,
  `tenant_header` to `x-mse-tenant`, `consumer_header` to
  `x-mse-consumer`,
  `balance_key_template` to
  `billing:balance:{tenant}:{quota_scope}:{consumer}`,
  `path_scope` to `legacy_suffixes`, and `missing_balance_policy` to `deny`
- **AND** it SHALL retain the existing legacy default path suffixes.

#### Scenario: Route-level quota scope is selected

- **WHEN** a Higress WasmPlugin `matchRules` entry matches an AI route and
  provides `quota_scope`
- **THEN** `ai-quota` SHALL use the matched `quota_scope` for the request's
  balance key construction.

#### Scenario: Bound target omits suffixes

- **WHEN** a rule selects `path_scope: bound_target` and omits
  `enable_path_suffixes` and `enable_methods`
- **THEN** ai-quota SHALL accept the configuration
- **AND** it SHALL perform admission for every request already matched by the
  Higress target.

#### Scenario: Bound target has advanced filters

- **WHEN** a bound-target rule supplies a non-empty suffix or method filter
- **THEN** ai-quota SHALL perform admission only for the intersection of the
  target and every supplied filter
- **AND** the filter SHALL NOT expand the target.

#### Scenario: Legacy settlement fields are present

- **WHEN** a replayed configuration contains legacy price, amount-scale, or
  missing-usage policy fields
- **THEN** ai-quota SHALL parse them for compatibility
- **AND** it SHALL treat them as inert
- **AND** it SHALL NOT perform post-response monetary deduction.

### Requirement: Request admission uses Redis monetary balance

`ai-quota` SHALL perform request admission for every in-scope request by reading
a Redis monetary balance key built from tenant, quota scope, and consumer
identity. Legacy-suffix instances SHALL retain existing path eligibility;
bound-target instances SHALL use the Higress target plus any explicit
non-empty advanced path or method intersections.

#### Scenario: Positive balance allows request

- **WHEN** an in-scope request has tenant and consumer headers and Redis returns
  a balance greater than `0`
- **THEN** `ai-quota` SHALL allow the request to continue.

#### Scenario: Non-positive balance denies request

- **WHEN** an in-scope request has tenant and consumer headers and Redis returns
  a balance less than or equal to `0`
- **THEN** `ai-quota` SHALL deny the request.

#### Scenario: Missing balance follows policy

- **WHEN** an in-scope request has tenant and consumer headers and the Redis
  balance key is missing
- **THEN** `ai-quota` SHALL deny the request when `missing_balance_policy` is
  `deny`
- **AND** it SHALL allow the request when `missing_balance_policy` is `allow`.

#### Scenario: Missing identity denies request

- **WHEN** an in-scope request does not include the configured tenant or
  consumer header
- **THEN** `ai-quota` SHALL deny the request.

#### Scenario: Bound provider-native request is admitted

- **WHEN** a provider-native operation is matched by a bound-target quota
  instance and satisfies any explicit advanced filters
- **THEN** ai-quota SHALL apply the same Redis admission decision without
  requiring a recognized AI API suffix or response usage shape.

## REMOVED Requirements

### Requirement: Monetary deduction uses token usage and effective prices

**Reason**: The gateway implementation and platform ownership boundary are
admission-only. Console or billing-service owns pricing, settlement, balance
mutation, reconciliation, and Redis rebuild; post-response gateway deduction
would duplicate that authority and cannot be truthful for opaque responses.

**Migration**: Keep request admission from Redis hot balance. Treat legacy
price and missing-usage config fields as inert parse-compatible inputs during
the patch transition, remove them from current examples, and perform all
settlement through Console or billing-service.

### Requirement: Token usage parser is shared but runtime state is not

**Reason**: Admission-only ai-quota does not parse response usage or perform
post-response deduction, so it no longer needs `pkg/tokenusage` as a normative
quota dependency.

**Migration**: ai-billing remains independently responsible for trustworthy
usage events. ai-quota bases admission only on its own Redis balance read and
does not depend on ai-billing or ai-statistics private runtime state.

## ADDED Requirements

### Requirement: Quota Enforcement Is Admission-Only

ai-quota SHALL NOT parse response bodies for token usage, read effective price
keys, calculate request cost, decrement Redis balances after response
completion, or mutate authoritative account state.

#### Scenario: Admitted request completes with usage

- **WHEN** a request admitted by ai-quota completes with recognizable token
  usage
- **THEN** ai-quota SHALL NOT calculate a cost or deduct the Redis balance
- **AND** billing-service or Console SHALL retain settlement ownership.

#### Scenario: Admitted request completes without usage

- **WHEN** a provider-native or opaque response contains no recognizable token
  usage
- **THEN** ai-quota SHALL NOT invent completion usage
- **AND** it SHALL NOT perform a post-response balance mutation.

#### Scenario: Billing or statistics plugin is disabled

- **WHEN** ai-billing, ai-statistics, or both are disabled for a route
- **THEN** ai-quota SHALL continue to make its independent admission decision
  from Redis hot balance
- **AND** it SHALL not depend on either plugin's private runtime state.

### Requirement: AI Quota Admission Release

The authoritative ai-quota extension version SHALL advance to `1.0.2` and
SHALL implement admission-only bound-target enforcement.

#### Scenario: Version 1.0.2 is packaged

- **WHEN** ai-quota `1.0.2` is built
- **THEN** its versioned Wasm artifact, resource metadata, schema, examples,
  and bilingual documentation SHALL identify `1.0.2`
- **AND** independent quota tests SHALL verify legacy scope, bound target,
  advanced filters, Redis admission, and absence of post-response deduction.
