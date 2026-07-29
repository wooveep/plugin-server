## MODIFIED Requirements

### Requirement: Billing plugin configuration

`ai-billing` SHALL support either a valid global `redis_stream` target with
inheriting rules or a dormant empty base with complete scoped rules. An empty
base SHALL initialize successfully as a no-op so Higress can use
`defaultConfigDisable: true`. Any non-empty base that omits
`redis_stream.service_name` SHALL remain invalid.

When the base is dormant, every active rule SHALL contain and validate its own
complete `redis_stream` target. When the base contains a valid Redis target,
rules SHALL inherit it and SHALL NOT replace it. Global and rule-level
`billing_service` SHALL always be rejected.

#### Scenario: Empty base is dormant

- **WHEN** Higress initializes ai-billing with an empty base configuration
- **THEN** plugin startup SHALL succeed
- **AND** the base configuration SHALL not read bodies, initialize billing
  request state, or emit events.

#### Scenario: Dormant base uses complete scoped rule

- **WHEN** an empty base is combined with a matching rule that contains valid
  `redis_stream.service_name` and `path_scope: bound_target`
- **THEN** the rule SHALL initialize its Redis client
- **AND** it SHALL process only requests selected by that Higress match rule.

#### Scenario: Dormant base rejects incomplete rule

- **WHEN** an empty base is combined with a matching rule that omits a valid
  `redis_stream.service_name`
- **THEN** ai-billing SHALL reject that rule configuration.

#### Scenario: Existing global target cannot be replaced

- **WHEN** global config contains a valid `redis_stream` and a matching rule
  also contains `redis_stream`
- **THEN** ai-billing SHALL reject the rule
- **AND** route data SHALL NOT replace the global delivery target.

#### Scenario: Non-empty incomplete base remains invalid

- **WHEN** base config contains any billing option but omits a valid
  `redis_stream.service_name`
- **THEN** ai-billing SHALL reject plugin initialization with a clear
  Redis-target configuration error.
