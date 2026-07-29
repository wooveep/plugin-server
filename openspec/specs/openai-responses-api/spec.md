# openai-responses-api Specification

## Purpose
Define OpenAI Responses adapter selection and provider-specific fallback behavior while preserving supported request, response, streaming, and capability-sentinel contracts.
## Requirements
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

### Requirement: Qwen Responses Native Capability Sentinel

For Qwen only, ai-proxy SHALL treat an explicitly configured empty or
whitespace-only `openai/v1/responses` capability path as meaning native support
is disabled. Existing Responses-to-Chat fallback SHALL remain conditional on
the existing Chat Completions capability. The sentinel SHALL NOT change
provider defaults or other providers' capability semantics.

#### Scenario: Explicit empty Qwen Responses path disables native support

- **WHEN** a Qwen capability map explicitly contains
  `openai/v1/responses` with an empty path
- **THEN** native Qwen Responses SHALL be reported unsupported
- **AND** the existing compatible fallback SHALL be selected when its Chat
  Completions dependency is supported.

#### Scenario: Whitespace-only Qwen Responses path disables native support

- **WHEN** a Qwen capability map explicitly contains
  `openai/v1/responses` with a whitespace-only path
- **THEN** native Qwen Responses SHALL be reported unsupported
- **AND** the whitespace value SHALL NOT be treated as a native endpoint.

#### Scenario: Omitted Qwen Responses path preserves defaults

- **WHEN** the Qwen capability map omits `openai/v1/responses`
- **THEN** normal provider-default merging and native capability behavior SHALL
  remain unchanged.

#### Scenario: Non-empty Qwen Responses path preserves native behavior

- **WHEN** a Qwen capability map contains a non-empty
  `openai/v1/responses` path
- **THEN** existing native Qwen Responses capability behavior SHALL remain
  unchanged.

#### Scenario: Other provider empty path remains supported

- **WHEN** any non-Qwen provider has an explicitly present empty
  `openai/v1/responses` capability path
- **THEN** its existing key-presence capability semantics SHALL remain
  unchanged
- **AND** the Qwen-specific sentinel SHALL NOT apply.
