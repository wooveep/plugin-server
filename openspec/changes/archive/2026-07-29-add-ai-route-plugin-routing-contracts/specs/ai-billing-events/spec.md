## MODIFIED Requirements

### Requirement: Billing plugin configuration

`ai-billing` SHALL require one global `redis_stream` delivery target and SHALL
support `event_kind`, `quota_scope`, `provider`, `cost_source`, `worker_kind`,
`bill_customer`, `tenant_header`, `consumer_header`, `path_scope`,
`enable_path_suffixes`, `enable_methods`, and `fail_policy`.

`redis_stream` SHALL contain required `service_name` and optional
`service_port`, `username`, `password`, `database`, `timeout`, and `stream`.
Rules SHALL inherit the global target and SHALL NOT replace it. Global and
rule-level `billing_service` SHALL be rejected. Rule-level `redis_stream` SHALL
also be rejected.

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

#### Scenario: Global Redis Stream target is required

- **WHEN** global config omits `redis_stream` or provides an empty
  `redis_stream.service_name`
- **THEN** ai-billing SHALL reject plugin initialization with a clear
  Redis-target configuration error.

#### Scenario: Removed global billing service is rejected

- **WHEN** global config contains `billing_service`
- **THEN** ai-billing SHALL reject plugin initialization
- **AND** it SHALL direct the operator to use global `redis_stream`.

#### Scenario: Rules inherit Redis Stream target

- **WHEN** global config contains a valid `redis_stream` and a matching rule
  omits delivery configuration
- **THEN** the rule SHALL use the global Redis target and stream.

#### Scenario: Rules cannot replace delivery target

- **WHEN** a matching rule contains `redis_stream` or `billing_service`
- **THEN** ai-billing SHALL reject that rule configuration
- **AND** route data SHALL NOT choose a Redis service, credential, database, or
  stream.

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

### Requirement: Billing events are generated per completed AI request

`ai-billing` SHALL generate one request-level billing event for every enabled,
in-scope request after response completion or stream termination. It SHALL
generate `event_id` at request start, retain it in request context, and default
`idempotency_key` to the same value.

The serialized event SHALL retain `event_id`, `event_kind`, `cost_source`,
`worker_kind`, `source_job`, `bill_customer`, `idempotency_key`, `request_id`,
`tenant`, `consumer`, `route`, `provider`, `quota_scope`, `model`,
`request_path`, `status_code`, `usage`, `usage_missing`, `usage_source`,
`start_time_ms`, `end_time_ms`, `is_stream`, `cluster`, optional
`price_version`, and trusted `upstream_invoked` according to their established
event-kind and omission rules.

Binding and path scope SHALL NOT prove model, token, provider usage, price, or
cost facts. Provider-native or opaque responses SHALL use only trustworthy
provider usage or safe recognized estimation. If neither is available, the
event SHALL contain `usage_missing=true`, `usage_source=missing`, and zero
token counts without fabricated model, provider usage, price, or cost.

#### Scenario: Completed request contains implemented event facts

- **WHEN** an enabled in-scope request completes
- **THEN** ai-billing SHALL serialize the complete applicable event surface
  defined by this requirement
- **AND** `request_id` SHALL come from canonical `x-request-id` or the trusted
  Higress `x_request_id` property and be used only for correlation
- **AND** raw API keys and `tenant_id`, `user_id`, `api_key_id`, or
  `consumer_id` UUID fields SHALL NOT be emitted.

#### Scenario: Internal-cost event contains configured attribution

- **WHEN** a request matches an `internal_cost` configuration and no valid
  invocation override is supplied
- **THEN** its event SHALL use configured `cost_source`, `worker_kind`, and
  optional `bill_customer`
- **AND** customer settlement classification SHALL remain
  configuration-derived.

#### Scenario: Upstream invocation fact is retained

- **WHEN** an enabled request completes from either upstream invocation or a
  trusted gateway-local/cache path
- **THEN** its event SHALL contain the trusted boolean `upstream_invoked`
- **AND** request or response payload fields SHALL NOT spoof that value.

#### Scenario: Provider usage is structured

- **WHEN** an enabled request completes with trustworthy provider usage
- **THEN** `usage.unit` SHALL be `token`, numeric input/output/total facts
  SHALL be emitted, and the complete provider usage object SHALL be retained
  under `usage.details.provider_usage`
- **AND** `usage_missing` SHALL be `false` and `usage_source` SHALL be
  `provider`.

#### Scenario: Safe estimation is structured

- **WHEN** provider usage is absent and tokenizer estimation from a recognized
  request and response structure succeeds
- **THEN** basic numeric token facts SHALL be emitted with
  `usage_source=estimated` and `usage_missing=false`
- **AND** provider-only details SHALL NOT be fabricated.

#### Scenario: Opaque provider-native usage is missing

- **WHEN** an enabled bound-target response has no trustworthy provider usage
  and cannot be safely estimated
- **THEN** it SHALL emit zero token counts with `usage_missing=true` and
  `usage_source=missing`
- **AND** it SHALL NOT fabricate model, provider usage, price, or cost from a
  path or opaque body.

#### Scenario: Event identity and retries remain stable

- **WHEN** two requests are processed or one event delivery is retried
- **THEN** each request SHALL have a distinct `event_id`
- **AND** all attempts for one event SHALL reuse its original
  `idempotency_key`.

### Requirement: Billing event delivery is fail-open by default

`ai-billing` SHALL deliver each event with Redis
`XADD <configured-stream> * event <serialized-json>`. Redis initialization,
dispatch, or command-response failure SHALL NOT block the user response.

#### Scenario: Redis event is appended

- **WHEN** an enabled in-scope request completes and Redis accepts delivery
- **THEN** ai-billing SHALL issue `XADD` to the inherited global stream with
  field name `event` and the serialized billing event
- **AND** it SHALL record the returned stream entry identity only in bounded
  diagnostics.

#### Scenario: Redis dispatch fails open

- **WHEN** the Redis client is unavailable or command dispatch fails
- **THEN** ai-billing SHALL allow the user response to complete
- **AND** it SHALL record a bounded delivery failure without event or
  credential values.

#### Scenario: Redis command response fails open

- **WHEN** Redis returns an error response for `XADD`
- **THEN** ai-billing SHALL allow the user response to complete
- **AND** it SHALL retain request-scoped at-most-once delivery lifecycle
  behavior.

### Requirement: Billing plugin does not expose secrets or raw credentials

`ai-billing` SHALL NOT log the Redis password, Redis username, raw API keys, or
trusted internal attribution values. Events and documentation examples SHALL
not expose real credentials.

#### Scenario: Redis credentials are configured

- **WHEN** ai-billing logs initialization, request handling, delivery success,
  or delivery failure
- **THEN** logs SHALL NOT contain configured Redis username or password.

#### Scenario: Request contains raw credentials

- **WHEN** an enabled request contains an API key or internal attribution
  value
- **THEN** ai-billing SHALL NOT include raw credentials in the event
- **AND** rejected attribution values SHALL NOT be written to logs.

#### Scenario: Documentation shows Redis authentication

- **WHEN** documentation or examples show `redis_stream.username` or
  `redis_stream.password`
- **THEN** values SHALL be placeholders and SHALL NOT be real credentials.

## REMOVED Requirements

### Requirement: Billing event callouts are authenticated

**Reason**: The selected base delivers billing events through an authenticated
global Redis Stream client and no longer implements billing-service HTTP
callouts.

**Migration**: Replace removed `billing_service` input with one global
`redis_stream` object. Rules inherit that object and cannot configure transport
or credentials.

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
- **THEN** the emitted event SHALL retain configured `cost_source` and
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

### Requirement: Billing Response Correlation Header

`ai-billing` SHALL expose the canonical request ID already captured in trusted
request context only as `x-ocmf-gateway-request-id`. It SHALL replace rather
than append that response header, and if canonical context is absent it SHALL
remove any upstream value and SHALL NOT invent an identity.

#### Scenario: Non-streaming response replaces upstream value

- **WHEN** a non-streaming billed response enters response-header handling
  with canonical request context
- **THEN** ai-billing SHALL replace any existing
  `x-ocmf-gateway-request-id` value with the canonical request ID
- **AND** it SHALL expose exactly one unambiguous value.

#### Scenario: Streaming response exposes canonical value

- **WHEN** a streaming billed response enters response-header handling
- **THEN** ai-billing SHALL set `x-ocmf-gateway-request-id` to the same
  canonical request ID used by the billing event
- **AND** stream completion SHALL NOT append a second value.

#### Scenario: Gateway-local or cache response exposes canonical value

- **WHEN** a billed response is produced by a gateway-local or cache lifecycle
  path without upstream invocation
- **THEN** response-header handling SHALL expose the canonical request ID
- **AND** response origin SHALL NOT weaken replacement semantics.

#### Scenario: Missing canonical context removes upstream value

- **WHEN** response-header handling has no canonical request ID in request
  context
- **THEN** ai-billing SHALL remove `x-ocmf-gateway-request-id` from the
  response
- **AND** it SHALL NOT forward an upstream value or invent another identity.

### Requirement: AI Billing Routing Release

The authoritative ai-billing extension version SHALL advance to `0.1.5` and
SHALL combine Redis Stream delivery, trusted internal attribution, exact
response correlation, bound-target scope, and truthful opaque-response usage.

#### Scenario: Version 0.1.5 is packaged

- **WHEN** ai-billing `0.1.5` is built
- **THEN** its versioned Wasm artifact, resource metadata, schema, examples,
  and bilingual documentation SHALL identify `0.1.5`
- **AND** independent tests SHALL cover the complete release contract.
