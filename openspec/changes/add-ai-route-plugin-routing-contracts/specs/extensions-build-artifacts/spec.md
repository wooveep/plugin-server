## ADDED Requirements

### Requirement: Authoritative Plugin Release Versions

The four changed extensions SHALL have one authoritative source version:
ai-proxy `1.0.2`, ai-billing `0.1.5`, ai-quota `1.0.2`, and ai-statistics
`1.0.1`. Resource metadata, option/plugin metadata, schemas, examples, and
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
  `build/plugins/ai-statistics/1.0.1/plugin.wasm`
- **AND** every file SHALL be non-empty and a valid Wasm module.

#### Scenario: Independent module tests fail

- **WHEN** an affected plugin's independent unit, integration, or security suite
  fails
- **THEN** its release artifact SHALL NOT be accepted as tested release
  evidence.

### Requirement: Immutable Artifact Provenance

Release evidence SHALL record the intentional base commit, reviewed result
commit, repository identity, plugin/version, versioned artifact path, SHA-256
checksum, build toolchain, exact build and test commands, and relevant
provenance for each artifact.

#### Scenario: Artifact evidence is recorded

- **WHEN** a release artifact is accepted for availability verification
- **THEN** its evidence SHALL identify base commit
  `9995ed5c7d4977a8ddf16ea34faea3d8286a9e56`
- **AND** it SHALL identify the exact reviewed result commit and the artifact's
  SHA-256 checksum.

#### Scenario: Result commit or bytes change

- **WHEN** source changes after review or an artifact's bytes differ from its
  recorded checksum
- **THEN** the artifact SHALL be rebuilt, retested, and assigned updated
  provenance before it can satisfy the release gate.

### Requirement: Exact Local Plugin Server Availability

The exact tested Wasm bytes SHALL be uploaded to the local plugin server and
verified, because this repository has no formal publication workflow, through
retrieval, availability, and byte-for-byte SHA-256 identity before downstream
catalog consumption.

#### Scenario: Exact bytes are uploaded and retrieved

- **WHEN** a versioned artifact is uploaded to the local plugin server
- **THEN** the verification workflow SHALL retrieve the served artifact
- **AND** the retrieved bytes SHALL have the same SHA-256 checksum as the tested
  local artifact
- **AND** plugin availability SHALL be verified through the supported local
  service interface.

#### Scenario: Only a local build exists

- **WHEN** an artifact has been built but has not been uploaded and retrieved
  with matching checksum
- **THEN** it SHALL NOT count as published or available release evidence
- **AND** downstream catalog gates SHALL remain unchanged.

#### Scenario: Retrieved bytes differ

- **WHEN** the plugin server returns bytes whose SHA-256 differs from the
  recorded tested artifact
- **THEN** verification SHALL fail
- **AND** those bytes SHALL NOT be used by downstream E2E or catalog updates.

### Requirement: Downstream Catalog Gate Evidence

The release evidence SHALL provide enough immutable identity for the
downstream `ocloudmodelfusion` repository to select only the verified plugin
versions and bytes.

#### Scenario: Downstream catalog is ready to update

- **WHEN** all four plugin suites, version checks, builds, provenance records,
  uploads, retrievals, and checksum comparisons pass
- **THEN** the evidence SHALL list exact versions, sibling result commit,
  artifact locations, and SHA-256 checksums for downstream consumption.

#### Scenario: Any plugin gate is incomplete

- **WHEN** any one of the four plugins lacks verified artifact availability
- **THEN** this change SHALL NOT claim the coordinated release gate complete
- **AND** downstream catalog compatibility SHALL remain blocked for that
  contract.
