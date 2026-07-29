## ADDED Requirements

### Requirement: AI Statistics Path Scope

`ai-statistics` SHALL support `path_scope` values `legacy_suffixes` and
`bound_target`. Missing `path_scope` SHALL preserve current legacy suffix
defaults and explicit suffix behavior. A bound-target instance SHALL observe
every request already matched by its Higress binding target unless an
explicitly submitted non-empty suffix or method filter narrows that target by
intersection.

#### Scenario: Legacy statistics configuration is replayed

- **WHEN** an existing statistics configuration omits `path_scope` or selects
  `legacy_suffixes`
- **THEN** ai-statistics SHALL preserve its current default and explicitly
  configured suffix semantics
- **AND** replay SHALL NOT widen the effective request set.

#### Scenario: Bound target observes provider-native operation

- **WHEN** a bound-target statistics instance has no non-empty advanced filter
  and its Higress target matches a provider-native operation
- **THEN** ai-statistics SHALL observe that request without a synthesized
  suffix list.

#### Scenario: Advanced path and method filters narrow statistics

- **WHEN** a bound-target statistics instance has an explicit non-empty suffix
  or method filter
- **THEN** it SHALL emit statistics only for the intersection of the target and
  every configured filter
- **AND** the filter SHALL NOT expand the target.

### Requirement: Safe Statistics Base Facts

ai-statistics SHALL emit safe route, request path, HTTP method, response status,
latency, and bounded error facts for every eligible completed request
independently of model, operation, or token extraction.

#### Scenario: Opaque successful response completes

- **WHEN** an eligible provider-native request receives a successful opaque
  response without model or token facts
- **THEN** ai-statistics SHALL emit the route, path, method, status, and latency
  base facts
- **AND** it SHALL omit unavailable model, operation, and token dimensions.

#### Scenario: Opaque error response completes

- **WHEN** an eligible provider-native request receives an error response with
  an unrecognized or opaque body
- **THEN** ai-statistics SHALL emit the route, path, method, status, latency,
  and a bounded error classification
- **AND** it SHALL NOT emit raw response bodies or unbounded provider error
  text as metric attributes.

#### Scenario: Usage parsing fails

- **WHEN** model, operation, or token usage parsing fails for an eligible
  request
- **THEN** the failure SHALL NOT suppress base request count or duration facts
- **AND** optional unsupported dimensions SHALL remain absent.

### Requirement: Trusted Statistics Enrichment

ai-statistics SHALL add model, normalized operation, adapter, or token fields
only from trusted adapter metadata or a supported provider body parser. It
SHALL NOT infer those facts from arbitrary paths or opaque bodies.

#### Scenario: Trusted adapter metadata is available

- **WHEN** ai-proxy provides reviewed trusted metadata for the adapter actually
  executed
- **THEN** ai-statistics SHALL emit its adapter ID, normalized operation, and
  usage profile
- **AND** it SHALL keep the safe base facts.

#### Scenario: Provider usage is trustworthy

- **WHEN** a supported provider parser extracts valid model or token facts
- **THEN** ai-statistics SHALL emit those fields with their trusted source
- **AND** it SHALL not replace them with guessed values.

#### Scenario: Request claims internal metadata

- **WHEN** a caller supplies headers or body fields resembling trusted adapter,
  operation, model, token, route, or error metadata
- **THEN** ai-statistics SHALL ignore those values unless the gateway trust
  boundary explicitly established them.

### Requirement: AI Statistics Release

The authoritative ai-statistics extension version SHALL advance to `1.0.1`,
and that version SHALL implement bound-target scope and safe base
observability.

#### Scenario: Version 1.0.1 is packaged

- **WHEN** ai-statistics `1.0.1` is built
- **THEN** its versioned Wasm artifact, resource metadata, schema, examples,
  and bilingual documentation SHALL identify `1.0.1`
- **AND** the independent statistics tests SHALL verify the release contract.
