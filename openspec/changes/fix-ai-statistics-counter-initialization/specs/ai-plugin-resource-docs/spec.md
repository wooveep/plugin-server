## MODIFIED Requirements

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
  identify `1.0.2`, `0.1.5`, `1.0.2`, and `1.0.2`, respectively
- **AND** any mismatch SHALL fail verification.
