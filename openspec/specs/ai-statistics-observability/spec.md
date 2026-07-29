# ai-statistics-observability Specification

## Purpose
Define ai-statistics request scoping, safe base facts, bounded-cardinality metrics, and exactly-once terminal observation for provider-compatible and provider-native traffic.
## Requirements
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

ai-statistics SHALL emit bounded route and path attribution, HTTP method,
response status, latency, and bounded error facts for every eligible completed
request independently of model, operation, or token extraction.

Response headers SHALL capture the bounded status/error projection but SHALL
NOT finalize completed-request duration. Base request count, duration, and
terminal AILog facts SHALL be emitted exactly once at non-streaming
response-body EOS, streaming EOS, or `wrapper.ProcessStreamDone` fallback when
no body callback can finalize the request. The duration counter and AILog
latency SHALL use the same terminal timestamp.

A response status SHALL be accepted only when `:status` contains exactly three
ASCII digits and its numeric value is in `100..599`. Every other value,
including missing, empty, `099`, `600`, `999`, `+200`, `0200`, or non-numeric
input, SHALL emit fixed `status_code=0` and `status_unknown`.

#### Scenario: Opaque successful response completes

- **WHEN** an eligible provider-native request receives a successful opaque
  response without model or token facts
- **THEN** ai-statistics SHALL emit bounded route and path attribution plus
  method, status, and latency base facts
- **AND** it SHALL omit unavailable model, operation, and token dimensions.

#### Scenario: Opaque error response completes

- **WHEN** an eligible provider-native request receives an error response with
  an unrecognized or opaque body
- **THEN** ai-statistics SHALL emit bounded route and path attribution plus
  method, status, latency, and a bounded error classification
- **AND** it SHALL NOT emit raw response bodies or unbounded provider error
  text as metric attributes.

#### Scenario: Usage parsing fails

- **WHEN** model, operation, or token usage parsing fails for an eligible
  request
- **THEN** the failure SHALL NOT suppress base request count or duration facts
- **AND** optional unsupported dimensions SHALL remain absent.

#### Scenario: Terminal completion owns duration

- **WHEN** response headers are received and response-body or streaming
  completion occurs later
- **THEN** `request_duration_ms` and terminal AILog latency SHALL include the
  elapsed time through that later completion
- **AND** response-header, body, streaming, and stream-done callbacks together
  SHALL emit the base facts exactly once.

#### Scenario: Header-only or parser-skipped response completes

- **WHEN** an eligible response has no body callback or its content type is not
  handled by an optional parser
- **THEN** `wrapper.ProcessStreamDone` SHALL finalize the base facts exactly
  once
- **AND** response status, headers, and body bytes SHALL remain transparent.

### Requirement: Bounded Route And Path Attribution

ai-statistics SHALL derive route attribution only from the trusted Higress
`route_name` property. A route name SHALL be non-empty, at most 128 bytes, and
contain no control character; otherwise the emitted value SHALL be the fixed
`route_unknown` sentinel.

Path attribution SHALL NOT use a raw request path, query string, request
header, credential, or body. It SHALL use only `operation_template` from a
supported ai-proxy schema-v2 envelope for the adapter actually selected with
`matched:true`. A valid template SHALL be origin-form, begin with exactly one
`/`, be at most 128 bytes, contain only ASCII letters, digits, `/`, `.`, `_`,
`-`, `~`, `{`, and `}`, and contain no query, fragment, control character, or
authority. Dynamic segments SHALL be adapter-owned `{placeholder}` values
rather than request values.

No metadata, malformed metadata, `matched:false`, schema-v1 metadata, or an
unknown schema version SHALL provide a trusted path template; those cases
SHALL emit the fixed `provider_native_unknown` path sentinel. If a supported
matched schema-v2 envelope supplies an invalid or oversized
`operation_template`, ai-statistics SHALL emit the fixed `path_unknown`
sentinel. The submitted invalid value SHALL never be emitted. Validation and
fallback SHALL be deterministic. Metadata envelopes larger than 1024 bytes
SHALL be treated as malformed. A matched schema-v2 envelope missing either
required operation field SHALL also be malformed and use
`provider_native_unknown`; only a present but invalid or oversized
`operation_template` SHALL use `path_unknown`.

#### Scenario: Dynamic operation uses trusted v2 operation template

- **WHEN** the raw request is
  `/v1/files/550e8400-e29b-41d4-a716-446655440000?api_key=secret` and the
  selected trusted adapter publishes schema-v2
  `operation_template: "/v1/files/{file_id}"`
- **THEN** path attribution SHALL be `/v1/files/{file_id}`
- **AND** it SHALL contain neither the UUID, query, nor credential.

#### Scenario: Opaque provider-native path uses fixed fallback

- **WHEN** an opaque provider-native request such as
  `/vendor/jobs/<attacker-controlled-value>` has no trusted adapter template
- **THEN** path attribution SHALL be `provider_native_unknown`
- **AND** no request-controlled segment or hash SHALL enter the attribute.

#### Scenario: Invalid trusted template uses invalid-template fallback

- **WHEN** a supported matched schema-v2 `operation_template` is not
  origin-form, does not begin with exactly one `/`, contains a query, fragment,
  authority, control character, or character outside the allowed set, or
  exceeds 128 bytes
- **THEN** path attribution SHALL be `path_unknown`
- **AND** the invalid value SHALL NOT be emitted.

#### Scenario: Schema-v1 logical operation has no trusted template

- **WHEN** matched schema-v1 metadata contains
  `normalized_operation: "openai/v1/responses"` but no v2 operation template
- **THEN** path attribution SHALL be `provider_native_unknown`
- **AND** the logical operation SHALL NOT be reinterpreted as a path.

#### Scenario: Unknown or unmatched metadata has no trusted template

- **WHEN** metadata has an unknown schema version, is malformed, or declares
  `matched:false`
- **THEN** path attribution SHALL be `provider_native_unknown`
- **AND** it SHALL NOT fabricate adapter, operation, usage, or path facts.

#### Scenario: Trusted route name is bounded

- **WHEN** the Higress `route_name` property is non-empty, at most 128 bytes,
  and contains no control character
- **THEN** route attribution SHALL equal that trusted property
- **AND** request-supplied route-like values SHALL be ignored.

#### Scenario: Unsafe route name uses fixed fallback

- **WHEN** trusted `route_name` is missing, empty, oversized, or contains a
  control character
- **THEN** route attribution SHALL be `route_unknown`.

#### Scenario: Fallback attribution is deterministic

- **WHEN** the same missing or invalid trusted metadata condition is processed
  repeatedly
- **THEN** ai-statistics SHALL emit the same documented fixed sentinel
- **AND** it SHALL NOT synthesize a value from raw traffic.

### Requirement: Trusted Statistics Enrichment

ai-statistics SHALL add model, normalized operation, adapter, or token fields
only from trusted adapter metadata or a supported provider body parser. It
SHALL NOT infer those facts from arbitrary paths or opaque bodies.

For schema v2, optional AILog `operation` SHALL use the separately validated
logical `normalized_operation`, while path attribution SHALL use the separately
validated origin-form `operation_template`. The operation value SHALL be
non-empty, at most 128 bytes, contain only ASCII letters, digits, `/`, `.`,
`_`, `-`, and `~`, not begin with `/`, contain no control character, query,
fragment, or authority, and SHALL NOT be replaced by the path template.

A matched schema-v1 envelope MAY retain only a logical
`normalized_operation` that passes that bound; it SHALL use
`provider_native_unknown` for path attribution and SHALL NOT trust its adapter,
adapter-version, provider, capability, or usage-profile fields. Unknown
schemas, malformed envelopes, and unmatched metadata SHALL contribute no
adapter, operation, or usage enrichment.

#### Scenario: Trusted adapter metadata is available

- **WHEN** ai-proxy provides reviewed trusted metadata for the adapter actually
  executed
- **THEN** ai-statistics SHALL emit its adapter ID, bounded logical normalized
  operation, and usage profile
- **AND** for schema v2 it SHALL derive path attribution separately from the
  bounded origin-form operation template
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

The authoritative ai-statistics extension version SHALL advance to `1.0.2`,
and that version SHALL implement bound-target scope, safe base observability,
and counter-cache initialization safety.

#### Scenario: Version 1.0.2 is packaged

- **WHEN** ai-statistics `1.0.2` is built
- **THEN** its versioned Wasm artifact, resource metadata, schema, examples,
  and bilingual documentation SHALL identify `1.0.2`
- **AND** the independent statistics tests SHALL verify the release contract.

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
