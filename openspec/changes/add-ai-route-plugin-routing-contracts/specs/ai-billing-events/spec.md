## MODIFIED Requirements

### Requirement: Billing plugin configuration

`ai-billing` SHALL support configuration for `billing_service`,
`quota_scope`, `provider`, `tenant_header`, `consumer_header`, `path_scope`,
`enable_path_suffixes`, `enable_methods`, and `fail_policy`.
`billing_service` SHALL include `service_name`, `service_port`, `path`,
`timeout`, and `auth_token`. `ai-billing` SHALL parse global config as a
complete default config and SHALL parse matched rule config as an override of
that global config.

Missing `path_scope` SHALL resolve to `legacy_suffixes`. In
`legacy_suffixes`, ai-billing SHALL retain its existing default and explicit
suffix behavior. In `bound_target`, the Higress binding target SHALL define
the default request set and the plugin SHALL accept an absent suffix list;
explicitly supplied non-empty suffix or method filters SHALL only narrow that
target by intersection.

#### Scenario: Billing service target is configured

- **WHEN** `ai-billing` global config is configured with
  `billing_service.service_name`, `billing_service.service_port`,
  `billing_service.path`, `billing_service.timeout`, and
  `billing_service.auth_token`
- **THEN** the plugin SHALL create HTTP callouts to the configured
  billing-service endpoint using the configured timeout
- **AND** it SHALL retain the configured auth token for callout authorization.

#### Scenario: Default fail policy is open

- **WHEN** `fail_policy` is omitted from global config
- **THEN** `ai-billing` SHALL use `open` as the fail policy.

#### Scenario: Default consumer header is configured

- **WHEN** `consumer_header` is omitted
- **THEN** `ai-billing` SHALL use `x-mse-consumer` as the consumer header.

#### Scenario: Path scope is omitted

- **WHEN** an existing or new config omits `path_scope`
- **THEN** `ai-billing` SHALL resolve it as `legacy_suffixes`
- **AND** it SHALL retain current legacy default suffix behavior.

#### Scenario: Bound target omits suffixes

- **WHEN** a matching rule selects `path_scope: bound_target` and omits
  `enable_path_suffixes` and `enable_methods`
- **THEN** `ai-billing` SHALL accept the configuration
- **AND** it SHALL process every request already selected by that Higress
  binding target.

#### Scenario: Bound target has advanced filters

- **WHEN** a bound-target rule supplies a non-empty `enable_path_suffixes` or
  `enable_methods` filter
- **THEN** `ai-billing` SHALL process only the intersection of the binding
  target and every supplied filter
- **AND** the filter SHALL NOT expand the target.

#### Scenario: Rule config inherits billing service

- **WHEN** global config includes `billing_service` and a matching rule config
  omits `billing_service`
- **THEN** `ai-billing` SHALL initialize successfully
- **AND** the matched rule SHALL use the inherited billing-service endpoint.

#### Scenario: Rule config overrides provider

- **WHEN** global config defines a default `provider` and a matching rule config
  defines a different `provider`
- **THEN** `ai-billing` SHALL use the rule-level provider for requests matched
  by that rule.

#### Scenario: Rule config overrides quota scope

- **WHEN** global config defines a default `quota_scope` and a matching rule
  config defines a different `quota_scope`
- **THEN** `ai-billing` SHALL use the rule-level quota scope for requests
  matched by that rule.

#### Scenario: Rule config overrides path filters and fail policy

- **WHEN** global config defines legacy path suffixes or fail policy and a
  matching rule config explicitly defines an eligible override
- **THEN** `ai-billing` SHALL use the rule-level value for the matched request
- **AND** it SHALL inherit omitted fields from global config
- **AND** a bound-target filter SHALL remain only an intersection.

#### Scenario: Full rule config remains valid

- **WHEN** a matching rule config provides a complete config including
  `billing_service`
- **THEN** `ai-billing` SHALL parse the rule config successfully
- **AND** it SHALL use the rule-level billing-service endpoint and rule-level
  fields.

#### Scenario: Global billing service is required

- **WHEN** global config omits `billing_service`
- **THEN** `ai-billing` SHALL fail plugin initialization with a clear missing
  `billing_service` error.

### Requirement: Billing events are generated per completed AI request

`ai-billing` SHALL generate a request-level billing event for every enabled,
in-scope AI request after response completion or stream termination. The plugin
SHALL generate an `event_id` at request start, store it in request context, and
reuse it when building the response-completion event. The event
`idempotency_key` SHALL default to the same value as `event_id`.

Binding and path scope SHALL determine whether the request is processed; path
shape alone SHALL NOT prove model, token, provider usage, price, or cost facts.
For provider-native or opaque responses, ai-billing SHALL use only trustworthy
provider usage or safe recognized estimation. If neither is available, it
SHALL emit `usage_missing=true`, `usage_source=missing`, and zero token counts
without fabricating model, provider usage, price, or cost.

#### Scenario: Successful event includes implemented request facts

- **WHEN** an enabled in-scope AI request completes and `X-Mse-Consumer` or the
  configured consumer header is present
- **THEN** `ai-billing` SHALL build an event containing `event_id`,
  `idempotency_key`, `request_id`, `tenant`, `consumer`, `quota_scope`,
  `route`, `provider`, `model`, `request_path`, `status_code`, `usage`,
  `usage_missing`, `usage_source`, `start_time_ms`, `end_time_ms`,
  `is_stream`, `cluster`, and optional `price_version`
- **AND** `consumer` SHALL equal the configured consumer header value
- **AND** `request_id` SHALL be read from `x-request-id` or the Higress
  `x_request_id` property and used only for trace correlation
- **AND** the event SHALL NOT include raw API-key values, `tenant_id`,
  `user_id`, `api_key_id`, or `consumer_id` UUID fields.

#### Scenario: Event identity is generated by the billing plugin

- **WHEN** an enabled in-scope AI request starts
- **THEN** `ai-billing` SHALL generate a new event identity
- **AND** it SHALL store it for response completion and default
  `idempotency_key` to that event identity.

#### Scenario: Each AI request gets a new event identity

- **WHEN** two enabled in-scope AI requests are processed
- **THEN** `ai-billing` SHALL generate a different `event_id` for each request
- **AND** each `idempotency_key` SHALL equal that request's `event_id` by
  default.

#### Scenario: Retry reuses idempotency key

- **WHEN** `ai-billing` retries delivery for the same billing event
- **THEN** every retry attempt SHALL reuse the original `idempotency_key`.

#### Scenario: Consumer is sourced from configured consumer header

- **WHEN** an enabled in-scope AI request includes the configured consumer
  header
- **THEN** `ai-billing` SHALL set event `consumer` to that header value.

#### Scenario: Rule-level provider is emitted

- **WHEN** two different match rules configure different `provider` values and
  each rule handles an enabled in-scope AI request
- **THEN** each generated billing event SHALL contain the provider resolved from
  the matched rule.

#### Scenario: Rule-level quota scope is emitted

- **WHEN** a matching rule config overrides `quota_scope` for an enabled
  in-scope AI request
- **THEN** the generated billing event SHALL contain the rule-level quota
  scope.

#### Scenario: Non-emitted cost fields are excluded

- **WHEN** `ai-billing` serializes a billing event
- **THEN** the event SHALL NOT contain top-level `input_tokens`, top-level
  `output_tokens`, top-level `total_tokens`, or
  `gateway_calculated_cost`.

#### Scenario: Provider usage is structured

- **WHEN** an enabled in-scope AI request completes with trustworthy provider
  token usage
- **THEN** `ai-billing` SHALL emit `usage.unit` as `token`, numeric
  `usage.input`, `usage.output`, `usage.total`, and
  `usage.details.provider_usage`
- **AND** `usage_missing` SHALL be `false`
- **AND** `usage_source` SHALL be `provider`.

#### Scenario: Estimated usage is structured

- **WHEN** an enabled in-scope AI request completes without provider usage and
  tokenizer estimation from a recognized safe structure succeeds
- **THEN** `ai-billing` SHALL emit `usage.unit` as `token`, numeric
  `usage.input`, `usage.output`, and `usage.total`
- **AND** `usage_missing` SHALL be `false`
- **AND** `usage_source` SHALL be `estimated`
- **AND** the event SHALL NOT include `usage.details.provider_usage`.

#### Scenario: Opaque provider-native usage is missing

- **WHEN** an enabled bound-target provider-native request completes without
  trustworthy provider usage and its request or response cannot be safely
  estimated
- **THEN** `ai-billing` SHALL emit `usage_missing` as `true`,
  `usage_source` as `missing`, `usage.unit` as `token`, and zero token counts
- **AND** it SHALL NOT fabricate a model, provider usage object, positive token
  count, price, or cost from the path or opaque body.

## ADDED Requirements

### Requirement: Billing Routing Preserves Trust Boundaries

The new protocol and path-scope behavior SHALL NOT change billing event
identity, response correlation, customer/internal-cost classification, trusted
internal attribution, delivery idempotency, or settlement ownership.

#### Scenario: Correlation header is exposed

- **WHEN** ai-billing exposes its internal gateway correlation response header
- **THEN** it SHALL use the canonical request ID already captured in trusted
  request context
- **AND** upstream or caller values SHALL NOT spoof that identity.

#### Scenario: Complete internal attribution pair is supplied

- **WHEN** a request matches an `internal_cost` rule and supplies a complete
  valid trusted cost-source and worker-kind pair
- **THEN** ai-billing SHALL retain the established invocation attribution
  behavior
- **AND** path scope SHALL NOT change event kind or customer-billing authority.

#### Scenario: Customer rule supplies internal headers

- **WHEN** a customer billing rule receives headers resembling internal-cost
  attribution
- **THEN** ai-billing SHALL ignore them for internal-cost classification
- **AND** route binding SHALL NOT enable internal-cost mode.

#### Scenario: Missing usage event is delivered

- **WHEN** an in-scope event has `usage_missing=true`
- **THEN** normal fail-open delivery, event identity, and idempotency semantics
  SHALL still apply
- **AND** downstream settlement ownership SHALL decide how to handle the
  missing facts.

### Requirement: AI Billing Routing Release

The authoritative ai-billing extension version SHALL advance to `0.1.5` and
SHALL combine the existing `0.1.4` trusted internal attribution contract with
bound-target scope and truthful opaque-response usage behavior.

#### Scenario: Version 0.1.5 is packaged

- **WHEN** ai-billing `0.1.5` is built
- **THEN** its versioned Wasm artifact, resource metadata, schema, examples,
  and bilingual documentation SHALL identify `0.1.5`
- **AND** independent billing tests SHALL cover legacy scope, bound target,
  advanced filters, provider usage, missing usage, correlation, and internal
  attribution.
