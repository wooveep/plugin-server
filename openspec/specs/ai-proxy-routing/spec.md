# ai-proxy-routing Specification

## Purpose
Define ai-proxy automatic adapter selection, forced passthrough, transparent provider-native fallback, provider-owned upstream authority, and backward-compatible request scoping.
## Requirements
### Requirement: AI Proxy Protocol Mode

`ai-proxy` SHALL support `protocol_mode` values `auto` and `passthrough`.
Missing `protocol_mode` SHALL resolve to `auto` for backward compatibility, and
any other value SHALL fail configuration validation.

#### Scenario: Protocol mode is omitted

- **WHEN** an existing or new ai-proxy configuration omits `protocol_mode`
- **THEN** ai-proxy SHALL use `auto`
- **AND** it SHALL retain compatible existing provider behavior.

#### Scenario: Passthrough is selected

- **WHEN** configuration selects `protocol_mode: passthrough`
- **THEN** ai-proxy SHALL accept the configuration
- **AND** it SHALL use the forced transparent path for matched requests.

#### Scenario: Protocol mode is invalid

- **WHEN** configuration supplies a protocol mode other than `auto` or
  `passthrough`
- **THEN** ai-proxy SHALL reject the configuration with a bounded error
- **AND** it SHALL NOT silently fall back to another mode.

### Requirement: Versioned Automatic Adapter Selection

In `auto` mode, ai-proxy SHALL resolve a stable versioned adapter from the
selected provider, HTTP method, normalized operation path, and declared
provider capabilities. It SHALL apply request and response transformations only
when an adapter matches.

Trusted adapter metadata SHALL use schema version `2`. For a real adapter
match, the schema-v2 envelope SHALL carry two distinct reviewed registry
values: `normalized_operation` as the stable logical operation ID and
`operation_template` as the origin-form path-attribution template. For
example, the reviewed Responses adapter SHALL publish
`normalized_operation: "openai/v1/responses"` and
`operation_template: "/v1/responses"`.

Both values SHALL be bounded registry constants. The logical operation SHALL
be non-empty, at most 128 bytes, contain only ASCII letters, digits, `/`, `.`,
`_`, `-`, and `~`, not begin with `/`, and contain no query, fragment,
authority, or control character. The template SHALL be non-empty and at most
128 bytes. An unmatched or forced-passthrough envelope SHALL NOT claim either
value, an adapter ID, or a usage profile. The complete envelope SHALL be at
most 1024 bytes. The registry schema version, serialized envelope, and adapter
tests SHALL remain aligned.

#### Scenario: Known adapter matches

- **WHEN** an automatic request matches a registered adapter for its provider,
  method, path, and capability
- **THEN** ai-proxy SHALL execute that adapter's request and response
  transformations
- **AND** it SHALL make the stable adapter ID, logical normalized operation,
  origin-form operation template, and usage profile available only as trusted
  gateway metadata
- **AND** the metadata SHALL identify schema version `2`.

#### Scenario: Provider capability does not satisfy an adapter

- **WHEN** the provider, method, and path resemble a known operation but the
  selected provider lacks the adapter's required capability
- **THEN** ai-proxy SHALL NOT claim that adapter identity
- **AND** it SHALL use a different proven adapter or transparent fallback.

#### Scenario: Adapter metadata is bounded

- **WHEN** ai-proxy publishes trusted adapter metadata
- **THEN** every value SHALL come from the reviewed adapter registry
- **AND** arbitrary request paths, query values, headers, or bodies SHALL NOT
  create an adapter ID, normalized operation, operation template, or usage
  profile
- **AND** the complete serialized envelope SHALL be at most 1024 bytes.

#### Scenario: Unmatched metadata does not fabricate adapter facts

- **WHEN** automatic resolution finds no adapter or forced passthrough skips a
  known adapter
- **THEN** any metadata envelope SHALL identify `matched:false`
- **AND** it SHALL omit adapter ID, normalized operation, operation template,
  and usage profile.

### Requirement: Transparent Provider-Native Fallback

When `auto` mode resolves no adapter, ai-proxy SHALL transparently forward the
route-rewritten origin-form HTTP method, path, raw query, and request body and
SHALL preserve the provider response body byte-for-byte. It SHALL continue to
apply only provider-owned authority, authentication, timeout, and failover
behavior.

#### Scenario: Unknown provider-native operation is forwarded

- **WHEN** an AI Route target admits an operation for which no adapter matches
- **THEN** ai-proxy SHALL preserve the effective HTTP method, path, raw query,
  and request body bytes through upstream dispatch
- **AND** it SHALL preserve the upstream response body bytes to the client
- **AND** it SHALL NOT rewrite the operation to a known API type.

#### Scenario: Selected provider controls transport policy

- **WHEN** an unknown provider-native operation uses transparent fallback
- **THEN** the configured provider SHALL remain authoritative for upstream
  authority, credentials, timeout, and failover
- **AND** request-controlled values SHALL NOT replace those provider facts.

#### Scenario: Transparent fallback does not invent protocol facts

- **WHEN** no adapter matches a transparent request
- **THEN** ai-proxy SHALL NOT parse or mutate its bodies as another API type
- **AND** it SHALL NOT emit a fabricated adapter ID, normalized operation,
  usage profile, model, or token fact.

### Requirement: Forced Provider-Native Passthrough

When `protocol_mode` is `passthrough`, ai-proxy SHALL bypass request and
response body transformations even if a known adapter would otherwise match.
It SHALL preserve the safe route-rewritten operation while retaining
provider-owned transport policy.

#### Scenario: Known adapter is bypassed

- **WHEN** a request would match a known adapter but its effective protocol
  mode is `passthrough`
- **THEN** ai-proxy SHALL NOT execute the adapter's request or response body
  transformation
- **AND** it SHALL preserve the request and response body bytes.

#### Scenario: Passthrough retains provider policy

- **WHEN** forced passthrough dispatches a request
- **THEN** ai-proxy SHALL still apply the selected provider's authority,
  authentication, timeout, and failover
- **AND** it SHALL NOT accept those values from the route path, query, or body.

#### Scenario: Passthrough metadata is truthful

- **WHEN** forced passthrough skips a known adapter
- **THEN** ai-proxy SHALL identify the effective mode as passthrough
- **AND** it SHALL NOT publish the skipped adapter as the operation actually
  performed.

### Requirement: AI Proxy Bound Target Scope

ai-proxy SHALL support `path_scope` values `legacy_suffixes` and
`bound_target`. Missing `path_scope` SHALL preserve current legacy
classification/default behavior. `bound_target` SHALL treat the Higress
binding target as the complete default request set without generating a suffix
list; explicitly configured non-empty suffix and method filters SHALL only
narrow that set by intersection.

#### Scenario: Legacy scope is replayed

- **WHEN** an existing ai-proxy configuration omits `path_scope` or selects
  `legacy_suffixes`
- **THEN** ai-proxy SHALL retain its existing operation classification and
  filtering behavior
- **AND** replay SHALL NOT silently expand its effective request set.

#### Scenario: Bound target has no advanced filter

- **WHEN** a route-bound ai-proxy instance selects `bound_target` without a
  non-empty suffix or method filter
- **THEN** it SHALL process every request already matched by the binding target
- **AND** its configuration SHALL NOT require a synthesized route path list.

#### Scenario: Advanced filters intersect the target

- **WHEN** a bound-target ai-proxy configuration supplies a non-empty suffix or
  method filter
- **THEN** ai-proxy SHALL process only requests in the intersection of the
  Higress target and every supplied filter
- **AND** no filter SHALL expand the binding target.

### Requirement: Provider Authority And URL Boundary

ai-proxy SHALL accept only safe origin-form operation paths within the selected
provider boundary. Route or request configuration SHALL NOT inject an absolute
URL, scheme, authority, encoded authority escape, or traversal beyond the
configured provider base.

#### Scenario: Absolute URL is supplied

- **WHEN** a route rewrite or provider-native operation attempts to supply an
  absolute URL or another authority
- **THEN** ai-proxy SHALL reject the unsafe operation
- **AND** it SHALL NOT dispatch credentials to the supplied authority.

#### Scenario: Traversal attempts to escape provider base

- **WHEN** a raw or encoded traversal segment would escape the configured
  provider base path
- **THEN** ai-proxy SHALL reject the unsafe operation before upstream dispatch.

#### Scenario: Safe raw query is preserved

- **WHEN** an origin-form path remains within the configured provider boundary
  and contains a raw query
- **THEN** transparent handling SHALL preserve the raw query bytes
- **AND** query values SHALL NOT change upstream authority or credentials.
