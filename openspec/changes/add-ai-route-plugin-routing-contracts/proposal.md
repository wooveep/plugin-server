## Why

Modelfusion AI Routes need gateway plugins that can safely process provider-native
operations without duplicating route path lists or pretending that unknown
protocol facts are known. The current releases provide only legacy suffix
gating, partial proxy fallback, and incomplete observability, so the main
`fix-ai-route-lifecycle-and-plugin-routing` change cannot enable its catalog
contracts until new sibling artifacts are specified, tested, and made available.

## What Changes

- Define `ai-proxy` protocol mode with a backward-compatible `auto` default,
  versioned known-adapter selection, transparent unknown-operation fallback,
  and forced `passthrough`.
- Preserve the original method, normalized path, raw query, request body, and
  response body for transparent operations while retaining provider-owned
  host, authentication, timeout, and failover policy and rejecting authority
  escape attempts.
- Add backward-compatible `legacy_suffixes` and route-owned `bound_target`
  path scope to ai-proxy, ai-billing, ai-quota, and ai-statistics. Explicit
  non-empty path or method filters narrow a bound target by intersection and
  never expand it.
- Require ai-billing to emit truthful provider usage when available and
  `usage_missing` without invented model, token, or cost facts for opaque
  responses.
- Align ai-quota with its implemented admission-only ownership boundary:
  request admission uses Redis hot balance, but the gateway does not perform
  post-response token deduction or invent completion usage.
- Require ai-statistics to emit safe route/path/method/status/latency/error
  base facts independently of model or token parsing and to omit unsupported
  dimensions rather than fabricate them.
- Publish synchronized patch targets ai-proxy `1.0.2`, ai-billing `0.1.5`,
  ai-quota `1.0.2`, and ai-statistics `1.0.1`, with authoritative versions,
  schemas, resource packages, examples, and bilingual documentation checked
  for alignment.
- Build immutable versioned Wasm files, record source/provenance and SHA-256
  identities, upload the exact tested bytes to the local plugin server (the
  repository has no formal publication workflow), and verify retrieval and
  byte identity before downstream catalog consumption.

## Capabilities

### New Capabilities

- `ai-proxy-routing`: Defines automatic adapter selection, transparent fallback,
  forced passthrough, trusted operation metadata, and outbound URL boundaries.
- `ai-statistics-observability`: Defines route-bound statistics scope and
  truthful base observability for recognized and opaque operations.
- `openai-responses-api`: Records how the existing Responses native/fallback
  adapters participate in protocol-mode selection without weakening their
  established conversion behavior.
- `extensions-build-artifacts`: Defines version alignment, versioned Wasm
  output, immutable provenance, checksum recording, and verified local
  availability for the four release artifacts.

### Modified Capabilities

- `ai-billing-events`: Adds backward-compatible path scope and truthful
  provider-native/opaque-response usage behavior while preserving correlation
  and internal-cost attribution.
- `ai-monetary-quota`: Replaces stale post-response deduction requirements with
  the implemented admission-only contract and adds bound-target filtering.
- `ai-plugin-resource-docs`: Aligns schemas, versions, examples, and bilingual
  documentation for protocol mode, path scope, truthful billing/statistics,
  and admission-only quota behavior.

## Impact

- Source and tests under `extensions/ai-proxy`, `extensions/ai-billing`,
  `extensions/ai-quota`, and `extensions/ai-statistics`.
- Shared provider/protocol and token-usage helpers where the affected plugins
  already depend on them.
- The four extension `VERSION` surfaces, `plugin.yaml`/`option.yaml`, resource
  catalog packages, examples, and bilingual READMEs.
- Root extension build automation, release evidence, and local plugin-server
  upload verification.
- The downstream `ocloudmodelfusion` change
  `fix-ai-route-lifecycle-and-plugin-routing`, which remains gated on these
  exact verified artifacts.
- This change is intentionally based on sibling commit
  `9995ed5c7d4977a8ddf16ea34faea3d8286a9e56` and depends on its completed
  ai-billing internal-cost attribution work. Existing active changes are not
  modified or archived by this proposal.
