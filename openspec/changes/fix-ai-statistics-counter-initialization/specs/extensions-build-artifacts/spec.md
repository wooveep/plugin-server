## MODIFIED Requirements

### Requirement: Authoritative Plugin Release Versions

The four changed extensions SHALL have one authoritative source version:
ai-proxy `1.0.2`, ai-billing `0.1.5`, ai-quota `1.0.2`, and ai-statistics
`1.0.2`. Resource metadata, option/plugin metadata, schemas, examples, and
documentation SHALL be generated from or automatically checked against that
source.

#### Scenario: Version alignment check runs

- **WHEN** release verification compares authoritative extension versions with
  every checked-in version surface
- **THEN** each surface SHALL contain the matching selected version
- **AND** any stale or conflicting version SHALL fail verification.

#### Scenario: Synthetic version is encountered

- **WHEN** tests or unrelated image properties use a synthetic or generic
  version such as `2.0.0`
- **THEN** release evidence SHALL NOT treat it as an authoritative plugin
  version
- **AND** it SHALL NOT replace one of the four selected release versions.

### Requirement: Four Versioned Wasm Artifacts

The reviewed sibling result commit SHALL build non-empty Wasm artifacts at
`build/plugins/<plugin>/<version>/plugin.wasm` for the four selected releases.

#### Scenario: Four-plugin release build completes

- **WHEN** the release build runs from the reviewed result commit
- **THEN** it SHALL produce
  `build/plugins/ai-proxy/1.0.2/plugin.wasm`,
  `build/plugins/ai-billing/0.1.5/plugin.wasm`,
  `build/plugins/ai-quota/1.0.2/plugin.wasm`, and
  `build/plugins/ai-statistics/1.0.2/plugin.wasm`
- **AND** every file SHALL be non-empty and a valid Wasm module.

#### Scenario: Independent module tests fail

- **WHEN** an affected plugin's independent unit, integration, or security suite
  fails
- **THEN** its release artifact SHALL NOT be accepted as tested release
  evidence.
