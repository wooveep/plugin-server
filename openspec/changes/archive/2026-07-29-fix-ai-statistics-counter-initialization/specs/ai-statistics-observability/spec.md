## ADDED Requirements

### Requirement: Statistics Counter Cache Safety

`ai-statistics` SHALL initialize its per-configuration counter cache before the
first counter definition is stored. Base and optional usage metrics SHALL
remain safe when any supported global or rule-override lifecycle supplies an
otherwise valid configuration with an uninitialized cache.

#### Scenario: First base counter uses an uninitialized cache

- **WHEN** an eligible completed request emits its first base counter through a
  configuration whose counter cache is nil
- **THEN** ai-statistics SHALL define, cache, and increment the counter without
  panicking
- **AND** later increments of the same metric SHALL reuse the cached counter.

## MODIFIED Requirements

### Requirement: AI Statistics Release

The authoritative ai-statistics extension version SHALL advance to `1.0.2`,
and that version SHALL implement bound-target scope, safe base observability,
and counter-cache initialization safety.

#### Scenario: Version 1.0.2 is packaged

- **WHEN** ai-statistics `1.0.2` is built
- **THEN** its versioned Wasm artifact, resource metadata, schema, examples,
  and bilingual documentation SHALL identify `1.0.2`
- **AND** the independent statistics tests SHALL verify the release contract.
