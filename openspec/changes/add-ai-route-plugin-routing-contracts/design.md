## Context

This change is the source-side release gate for Modelfusion's active
`fix-ai-route-lifecycle-and-plugin-routing` change. The downstream Console and
SDK already describe `protocol_mode: auto | passthrough` and
`path_scope: legacy_suffixes | bound_target`, but their current catalog
versions deliberately advertise no compatible runtime contract.

The four independently built plugins have different implementations and
release surfaces:

- ai-proxy `1.0.1` has provider-specific conversion chains and partial
  incidental fallback for unclassified paths, but no route-effective
  protocol mode or stable adapter contract.
- ai-billing `0.1.4` produces truthful missing-usage events only after a request
  passes legacy suffix gating.
- ai-quota `1.0.1` performs Redis balance admission. Its accepted specification
  and resources still describe post-response monetary deduction that the
  runtime does not perform.
- ai-statistics `1.0.0` has request and latency context, but can omit all useful
  metrics when model or token parsing fails.

The repository has root build automation that writes
`build/plugins/<plugin>/<version>/plugin.wasm`, but no formal publishing
workflow. Exact local plugin-server upload plus retrieval and checksum
verification is therefore the supported availability evidence for this
change.

The isolated worktree is intentionally based on
`9995ed5c7d4977a8ddf16ea34faea3d8286a9e56`, which contains the completed
ai-billing internal-cost attribution dependency. Existing complete active
changes remain independent and are neither modified nor archived here.

## Goals / Non-Goals

**Goals:**

- Make protocol transformation an explicit, testable ai-proxy decision.
- Preserve provider-native traffic byte-for-byte when no adapter applies or
  passthrough is forced, within the configured provider trust boundary.
- Give all four plugins one backward-compatible path-scope vocabulary.
- Keep billing, quota, and statistics truthful when model, operation, or token
  facts are unavailable.
- Align each selected patch version across authoritative source metadata,
  schemas, examples, and documentation.
- Produce and verify immutable artifact identities that the downstream catalog
  can consume.

**Non-Goals:**

- Turn an AI Route into an arbitrary outbound URL proxy.
- Let route input replace provider-owned authority, credentials, timeout, or
  failover policy.
- Add new provider adapters merely to increase adapter coverage.
- Move account settlement, price calculation, statements, or post-response
  balance mutation into ai-quota.
- Infer model, operation, token, cost, or error facts from an opaque payload
  without trustworthy evidence.
- Introduce a registry publication workflow that this repository does not
  currently have.

## Decisions

### 1. Use one explicit protocol-mode state machine

ai-proxy will parse `protocol_mode` as `auto` or `passthrough`; absence resolves
to `auto`, and any other value is rejected. In `auto`, an adapter resolver uses
the selected provider, HTTP method, normalized operation path, and declared
provider capabilities to select a stable adapter identity. A match executes the
existing request and response conversion chain. No match enters transparent
fallback.

In transparent fallback, ai-proxy preserves the route-rewritten origin-form
method, path, raw query, request bytes, and response bytes. It still applies
provider-owned authority, authentication, timeout, and failover. It does not
classify the body as another protocol.

In `passthrough`, ai-proxy takes the transparent path even when an adapter would
otherwise match. This is a single route-effective switch rather than a
provider-local `protocol: original` convention.

Alternative considered: infer passthrough from provider type or unknown paths.
That leaves known operations unable to opt out and makes compatibility
incidental rather than versioned.

### 2. Separate origin-form routing from authority

Protocol mode may preserve or transform only the request operation within the
existing route rewrite policy. Absolute URLs, scheme/authority forms, encoded
authority escape, traversal beyond the configured base, or other attempts to
replace the selected provider boundary are rejected before dispatch.

Provider host, authentication, timeout, and failover remain sourced from
provider configuration. Raw query preservation does not grant authority over
the upstream host.

Alternative considered: accept absolute provider-native URLs in route config.
That would cross the established credential and SSRF boundary and is excluded.

### 3. Make adapter metadata trustworthy and bounded

Stable adapter metadata includes adapter ID, normalized operation, usage
profile, provider/capability match, and version. It is set only after an adapter
match. Transparent fallback and forced passthrough do not invent an adapter ID,
operation, or usage profile from an arbitrary path or body.

Existing Responses native and Chat Completions fallback conversions become
registered adapter outcomes. This preserves their current conversion behavior
while allowing provider-native Responses operations without a registered
conversion to pass transparently.

Alternative considered: let downstream plugins reclassify arbitrary paths.
That duplicates proxy logic and creates untrusted, inconsistent facts.

### 4. Use identical path-scope semantics in four independent plugins

Each plugin parses:

- missing `path_scope` as `legacy_suffixes`;
- `legacy_suffixes` with its current default and explicit suffix behavior;
- `bound_target` as permission to process every request already selected by the
  Higress binding target.

A new bound-target projection carries no synthesized suffix list. An explicitly
submitted non-empty suffix list or method list is an additional intersection.
It can narrow but never expand the binding target. In bound-target mode an
absent or empty advanced filter means no additional narrowing; legacy handling
of explicitly empty suffix lists remains unchanged for compatibility.

The behavior is implemented inside each independent extension module rather
than introducing a new shared runtime package. The same table-driven contract
fixtures and schema vocabulary keep the modules aligned.

Alternative considered: use an empty suffix list to mean all paths everywhere.
That would silently widen replayed legacy instances and violates backward
compatibility.

### 5. Preserve billing truth and existing trust boundaries

For every eligible bound-target request, ai-billing first consumes supported,
trustworthy provider usage. Existing provider usage authority, cache-aware
details, event identity/idempotency, response correlation, Redis/event delivery,
and internal-cost attribution stay intact.

If a provider-native or opaque response has no trustworthy usage and cannot be
safely estimated from a recognized structure, the event records
`usage_missing=true` and `usage_source=missing`. It does not invent model,
token, provider-usage, price, cost, or settlement facts. Route binding only
controls invocation; it cannot enable internal-cost classification or customer
billing.

Alternative considered: estimate from arbitrary bytes. That can turn opaque
media or vendor payloads into false billable usage.

### 6. Make ai-quota admission-only

ai-quota checks the configured Redis hot balance before forwarding every
eligible request and applies the existing missing-balance policy. It performs no
response-body token parsing, price lookup, cost calculation, or post-response
deduction.

Legacy price and missing-usage configuration fields may remain parse-compatible
as inert inputs during the patch transition, but schemas and examples no longer
present them as active behavior. Console or billing-service owns settlement and
Redis balance rebuild.

Alternative considered: finish the stale post-response deduction design.
That would duplicate billing-service ownership and depend on usage facts that
provider-native operations may not expose.

### 7. Emit statistics base facts before optional AI enrichment

ai-statistics emits route, request path, HTTP method, status, latency, and a
bounded error classification independently of model/operation/token parsing.
Trusted adapter/provider metadata may enrich the record. Missing or opaque
payloads leave optional fields absent rather than blocking base count/duration
or fabricating values.

Alternative considered: keep model/usage parsing as the metric gate. That makes
unknown provider-native traffic operationally invisible.

### 8. Treat version alignment and exact bytes as one release gate

The selected targets are:

- ai-proxy `1.0.2`
- ai-billing `0.1.5`
- ai-quota `1.0.2`
- ai-statistics `1.0.1`

Each plugin will have one authoritative extension version. Generated metadata,
resource `info.version`, examples, and documentation are checked against it.
Stale `option.yaml` or resource versions must be generated from or validated
against that source rather than acting as independent authorities.

After review and all plugin tests, the four Wasm files are built from one
recorded result commit. A release evidence manifest records intentional base,
result commit, plugin/version, versioned path, SHA-256, toolchain, build/test
commands, and provenance. The exact bytes are uploaded to the local plugin
server, retrieved, and hashed again. A mere local build is not availability
evidence.

Alternative considered: update downstream catalog versions as soon as tests
pass. That can point the catalog at absent or different bytes.

### 9. Use TDD and independent module/security gates

RED fixtures are committed before behavior changes. Each plugin implementation
has focused unit tests, proxy-wasm lifecycle/integration tests, and relevant
security tests. Final verification covers all independent extension modules,
root build automation, formatting/vetting, URL-boundary and secret/log safety,
Graphify refresh, GitNexus change detection, clean diff, and strict OpenSpec.

## Risks / Trade-offs

- [Transparent fallback could expose an unintended provider endpoint] →
  Restrict it to an already matched Higress target and enforce the provider
  authority and origin-form URL boundary.
- [Stable adapter IDs can drift from current handlers] → Add table-driven
  adapter resolution tests and derive resource metadata from the reviewed
  registry.
- [Legacy stored configs can be widened] → Default missing scope to
  `legacy_suffixes` and never treat a legacy empty/default list as
  `bound_target`.
- [Base statistics can have high cardinality] → Use the existing normalized
  route/path conventions and bounded error classes; never add raw query,
  payload, credentials, or unbounded error text.
- [Inert quota fields can confuse operators] → Keep parse compatibility only,
  remove them from current examples, and document that settlement is external.
- [Local plugin-server availability is environment-specific] → Record exact
  retrieval commands and byte hashes so the downstream E2E can prove it uses
  the same artifacts.

## Migration Plan

1. Add RED contract fixtures for proxy mode/adapter/fallback/safety and for the
   four plugins' legacy/bound-target scope behavior.
2. Implement ai-proxy mode resolution and transparent fallback, then implement
   billing, quota, and statistics scope/truthfulness independently.
3. Synchronize schemas, resources, examples, bilingual docs, and authoritative
   versions.
4. Run every affected independent module and security suite; refresh Graphify
   and inspect GitNexus impact/change scope.
5. Build the four versioned Wasm files from the reviewed result commit and
   record SHA-256/provenance evidence.
6. Upload those exact bytes to the local plugin server, retrieve them, and
   verify byte-for-byte checksum identity.
7. Synchronize accepted specs and archive this sibling change only after all
   tasks and strict gates pass. Do not archive the pre-existing active changes.
8. Allow the downstream repository to update its catalog only to the exact
   verified versions and hashes.

Rollback keeps the previous plugin artifacts and catalog versions. No database
migration is required. Legacy instances continue to resolve missing
`path_scope` to `legacy_suffixes`.

## Open Questions

None.
