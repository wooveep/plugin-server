## MODIFIED Requirements

### Requirement: ai-billing resource schema matches plugin configuration

The ai-billing resource schema and documentation SHALL describe both legacy
global-target inheritance and scoped-only delivery. Global configuration SHALL
retain its complete Redis target schema. Route configuration SHALL expose the
same `redis_stream` shape for use only with a dormant disabled base, together
with supported business, identity, path-filter, and fail-policy fields.

#### Scenario: Scoped-only example is complete

- **WHEN** the ai-billing schema shows a `bound_target` scoped-only example
- **THEN** its rule SHALL contain a valid `redis_stream.service_name`
- **AND** it SHALL omit synthesized suffix and method filters
- **AND** documentation SHALL require `defaultConfigDisable: true` for the
  corresponding empty global scope.

#### Scenario: Legacy example keeps global target

- **WHEN** the schema shows legacy global inheritance
- **THEN** Redis delivery SHALL remain in global config
- **AND** route rules SHALL omit `redis_stream`.

#### Scenario: Documentation distinguishes replacement from scoped delivery

- **WHEN** an operator reads ai-billing configuration guidance
- **THEN** it SHALL state that rule-level Redis is accepted only when the base
  is dormant
- **AND** a rule SHALL NOT replace an existing global target.
