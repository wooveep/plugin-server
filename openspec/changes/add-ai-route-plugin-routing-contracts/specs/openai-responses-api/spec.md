## ADDED Requirements

### Requirement: Responses Adapter Participates In Automatic Mode

In ai-proxy `auto` mode, supported OpenAI Responses operations SHALL use a
registered native or Chat Completions fallback adapter according to selected
provider capabilities. Existing supported request, response, and streaming
conversion behavior SHALL remain unchanged when that adapter is selected.

#### Scenario: Provider supports native Responses

- **WHEN** an automatic Responses request matches a provider capability for a
  native Responses operation
- **THEN** ai-proxy SHALL select the native Responses adapter
- **AND** it SHALL preserve the client Responses protocol without fallback
  conversion.

#### Scenario: Provider uses supported Chat Completions fallback

- **WHEN** an automatic Responses request matches the reviewed Responses
  fallback adapter and the provider supports its Chat Completions dependency
- **THEN** ai-proxy SHALL apply the established request, non-streaming response,
  or streaming response conversion as applicable
- **AND** it SHALL expose the stable fallback adapter identity as trusted
  metadata.

#### Scenario: No Responses adapter matches

- **WHEN** an admitted provider-native Responses path or method has no
  registered native or fallback adapter
- **THEN** ai-proxy SHALL use transparent fallback
- **AND** it SHALL NOT reinterpret the operation as Create Responses or Chat
  Completions.

### Requirement: Responses Forced Passthrough

When `protocol_mode` is `passthrough`, ai-proxy SHALL bypass all Responses
request, non-streaming response, and SSE response transformations even when a
Responses adapter would otherwise match.

#### Scenario: Known Responses request is forced through

- **WHEN** a known Responses request is handled in passthrough mode
- **THEN** its request body SHALL NOT be converted to Chat Completions
- **AND** its provider response body or SSE bytes SHALL NOT be converted back to
  Responses events.

#### Scenario: Passthrough keeps provider boundary

- **WHEN** a Responses operation is forced through
- **THEN** the selected provider SHALL remain authoritative for host,
  authentication, timeout, and failover
- **AND** route or request data SHALL NOT inject another authority.

### Requirement: Responses Metadata Is Truthful

ai-proxy SHALL emit Responses operation and usage metadata only for the native
or fallback adapter actually selected. Transparent or passthrough Responses
traffic SHALL not inherit metadata from a skipped adapter.

#### Scenario: Transparent stateful operation has no adapter

- **WHEN** a provider-native Responses stateful operation is transparently
  forwarded because no adapter matches
- **THEN** ai-proxy SHALL not label it as Create Responses
- **AND** downstream plugins SHALL receive no fabricated Responses usage
  profile.

#### Scenario: Fallback adapter is selected

- **WHEN** the supported Responses-to-Chat fallback executes
- **THEN** downstream trusted metadata SHALL identify that fallback adapter and
  its normalized operation
- **AND** metadata SHALL not claim native Responses handling.
