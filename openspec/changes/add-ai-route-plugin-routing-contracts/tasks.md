## 1. Governance And Contract Baseline

- [x] 1.1 Inspect repository instructions, Graphify maps, existing plugin source
  contracts, accepted specs, active changes, version surfaces, build and
  publication policy; record intentional base
  `9995ed5c7d4977a8ddf16ea34faea3d8286a9e56` and selected patch targets.
- [x] 1.2 Create proposal, design, seven delta capability specs, and this task
  plan with `openspec new change`/status/instructions; run strict validation
  without modifying or archiving any pre-existing active change.
- [ ] 1.3 Before every source-symbol edit, use the current Graphify map and run
  GitNexus upstream impact for the exact symbol; report HIGH or CRITICAL blast
  radius before editing and record affected callers/processes in task evidence.

Task 1.1 evidence (2026-07-29):

- The isolated worktree branch `add-ai-route-plugin-routing-contracts` is
  intentionally based on sibling commit
  `9995ed5c7d4977a8ddf16ea34faea3d8286a9e56`, which includes the completed
  ai-billing internal-cost attribution dependency.
- Existing complete active changes
  `expose-ai-billing-request-correlation`,
  `fix-ai-memory-internal-cost-attribution`, and
  `fix-qwen-responses-capability-fallback` do not authorize this routing work
  and remain untouched.
- The repository has versioned root build output but no formal publication
  workflow. Exact local plugin-server upload, retrieval, availability, and
  SHA-256 identity are therefore required.
- Selected targets are ai-proxy `1.0.2`, ai-billing `0.1.5`, ai-quota `1.0.2`,
  and ai-statistics `1.0.1`.

Task 1.2 evidence (2026-07-29):

- `openspec new change add-ai-route-plugin-routing-contracts` created the
  spec-driven change; status reports proposal, design, specs, and tasks
  complete.
- `openspec validate add-ai-route-plugin-routing-contracts --strict` passes for
  seven delta capabilities, and `git diff --check` passes.
- Only this new change directory was modified; the three pre-existing active
  changes remain untouched.

For every RED task below, run the fixture against the pre-implementation state
and record the expected observed failure before production edits. No GREEN,
module verification, review-resolution, or final verification task may be
completed while any required fixture is still failing.

## 2. AI Proxy Red-Green Contracts

- [x] 2.1 Add RED parser tests proving missing `protocol_mode` defaults to
  `auto`, `passthrough` is accepted, invalid modes fail, and missing
  `path_scope` preserves legacy behavior.

Task 2.1 evidence (2026-07-29):

- The Graphify report and focused parser audit were reviewed before source
  inspection. Upstream impact for `PluginConfig`, `FromJson`, `Validate`,
  `parseGlobalConfig`, and `parseOverrideRuleConfig` was LOW with zero indexed
  execution flows. The CRITICAL shared `RunGoTest` and `NewTestHost` helpers
  were not modified.
- Pre-change baselines passed:
  `go test ./config -count=1` and
  `go test . -run '^TestParseGlobalAndOverrideConfig$' -count=1`.
- `extensions/ai-proxy/config/routing_config_test.go` adds independent runtime
  parser assertions for default `auto`, accepted `passthrough`, omitted
  override inheritance, invalid-mode rejection, and default
  `legacy_suffixes`. No production source or shared test harness changed.
- RED command:
  `go test ./config -run '^TestPluginConfigRoutingParserContract$' -count=1`.
  It compiled and failed only for the missing contract: three subtests
  reported that `*PluginConfig` does not implement `GetProtocolMode`, invalid
  mode validation returned `nil`, and the legacy-scope subtest reported that
  `*PluginConfig` does not implement `GetPathScope`.

- [ ] 2.2 Add RED adapter tests proving a stable known Responses/Claude/Qwen
  adapter is selected only when provider, method, path, and capability match
  and performs the expected request/response transformation; cover the
  Qwen-only `openai/v1/responses` sentinel for explicit empty, whitespace,
  omitted, and non-empty paths plus another provider's empty path.
- [ ] 2.3 Add RED transparent-fallback tests using a non-generic provider,
  nontrivial method, path, raw query, opaque request body, and opaque response
  body; prove exact preservation plus provider-owned host/auth/timeout/failover.
- [ ] 2.4 Add RED forced-passthrough tests proving an otherwise-known adapter
  does not transform request, non-streaming response, or streaming response
  bodies and does not publish the skipped adapter identity.
- [ ] 2.5 Add RED URL-boundary security tests for absolute URLs, scheme or
  authority injection, raw/encoded traversal, credential boundary, raw-query
  preservation, and safe diagnostics without secrets or bodies.
- [ ] 2.6 Add RED path-scope tests proving legacy replay, bound-target
  provider-native coverage without synthesized suffixes, and explicit
  non-empty suffix/method intersections.
- [ ] 2.7 Implement validated protocol/path-scope parsing, stable adapter
  resolution, trusted adapter metadata, and Responses adapter registration.
- [ ] 2.8 Implement transparent unknown fallback and forced passthrough while
  preserving origin-form operation bytes and provider-owned transport policy.
- [ ] 2.9 Run focused and complete independent ai-proxy unit, proxy-wasm
  lifecycle/integration, streaming, regression, and security suites.
- [ ] 2.10 Obtain an independent ai-proxy implementation/security review,
  resolve every finding, and rerun the affected RED/GREEN and complete module
  suites before beginning the billing implementation.

## 3. AI Billing Red-Green Contracts

- [ ] 3.1 Add RED configuration, Redis delivery, and request-gating tests:
  global `redis_stream` requirement/defaults, global and rule-level removed
  `billing_service` rejection, rule-level `redis_stream` rejection, rule
  inheritance without target replacement, omitted/explicit legacy scope,
  bound target without suffixes, provider-native paths, and explicit non-empty
  suffix/method intersections.
- [ ] 3.2 Add RED event tests proving trustworthy provider usage remains
  authoritative and opaque native responses produce `usage_missing=true`
  without fabricated model, token, provider-usage, price, or cost facts;
  verify the complete configuration/event surface including `event_kind`,
  `cost_source`, `worker_kind`, `source_job`, `bill_customer`, and
  `upstream_invoked`, plus fail-open Redis `XADD` delivery.
- [ ] 3.3 Add RED regression/security fixtures for all five internal-cost
  scenarios (complete pair, absent-pair fallback, partial suppression,
  empty/oversized/invalid-character suppression, and customer no-spoof) and
  the exact `x-ocmf-gateway-request-id` lifecycle (replace-not-append,
  canonical-absent removal, non-streaming, streaming, and local/cache paths);
  also preserve event identity, idempotent streaming delivery, cache facts,
  Redis credential redaction, and submitted-value non-leakage.
- [ ] 3.4 Implement ai-billing path-scope parsing and bound-target/advanced
  filter gating without widening legacy configurations.
- [ ] 3.5 Implement truthful provider-native/opaque-response event handling
  while preserving global inherited Redis delivery, exact correlation, and
  internal-cost authority.
- [ ] 3.6 Run focused and complete independent ai-billing unit,
  proxy-wasm lifecycle/integration, streaming, Redis delivery,
  internal-attribution, correlation, and security suites.
- [ ] 3.7 Obtain an independent ai-billing implementation/security review,
  resolve every finding, and rerun the affected RED/GREEN and complete module
  suites before beginning the quota implementation.

## 4. AI Quota Red-Green Contracts

- [ ] 4.1 Add RED configuration and admission tests for legacy scope,
  bound-target provider-native requests, no synthesized suffix list, and
  explicit non-empty suffix/method intersections.
- [ ] 4.2 Add RED ownership tests proving the request-header Redis balance
  admission result is final for recognizable, opaque, streaming, and
  non-streaming responses; Redis balance input remains read-only and no
  response lifecycle callback adds account enforcement.
- [ ] 4.3 Implement ai-quota path-scope parsing and bound-target admission
  intersections while preserving legacy request eligibility.
- [ ] 4.4 Converge ai-quota schema, examples, docs, and runtime on the
  admission-only Redis balance surface, removing obsolete settlement controls
  from current configuration and response-stage account behavior from the
  implementation.
- [ ] 4.5 Run focused and complete independent ai-quota unit, Redis admission,
  proxy-wasm lifecycle/integration, no-deduction regression, and security
  suites.
- [ ] 4.6 Obtain an independent ai-quota implementation/ownership review,
  resolve every finding, and rerun the affected RED/GREEN and complete module
  suites before beginning the statistics implementation.

## 5. AI Statistics Red-Green Contracts

- [ ] 5.1 Add RED configuration/gating tests for legacy scope, bound-target
  provider-native requests, no synthesized suffixes, and advanced non-empty
  suffix/method intersections.
- [ ] 5.2 Add RED lifecycle/metric tests proving opaque success and error
  responses emit bounded route/path attribution, method, status, latency,
  bounded error, request count, and duration independently of model/token
  parsing.
- [ ] 5.3 Add RED trust/cardinality fixtures proving: raw
  `/v1/files/<uuid>?api_key=secret` plus trusted
  `/v1/files/{file_id}` emits only the template; unknown
  `/vendor/jobs/<attacker>` emits `provider_native_unknown`; invalid or
  oversized trusted templates emit `path_unknown`; valid trusted
  `route_name` is retained while missing/unsafe/oversized values emit
  `route_unknown`; repeated fallbacks are deterministic; raw queries,
  credentials, headers, bodies, dynamic segments, and raw-path hashes never
  enter metric attributes.
- [ ] 5.4 Implement bound-target gating and base-fact emission before optional
  trusted adapter/provider enrichment, with 128-byte bounded trusted Higress
  route names and selected-adapter origin-form operation templates.
- [ ] 5.5 Run focused and complete independent ai-statistics unit,
  proxy-wasm lifecycle/integration, metric regression, cardinality, and
  security suites.
- [ ] 5.6 Obtain an independent ai-statistics implementation/cardinality
  review, resolve every finding, and rerun the affected RED/GREEN and complete
  module suites before version/resource work.

## 6. Versions, Schemas, Resources, And Documentation

- [ ] 6.1 Define one shared schema vocabulary for `protocol_mode`,
  `path_scope`, `enable_path_suffixes`, and `enable_methods`, preserving legacy
  defaults and documenting bound-target advanced intersections in all affected
  extension/resource examples.
- [ ] 6.2 Advance authoritative ai-proxy version to `1.0.2` and align
  plugin/option metadata, schema, examples, adapter metadata, and bilingual
  extension/resource documentation.
- [ ] 6.3 Advance authoritative ai-billing version to `0.1.5` and align
  metadata, global Redis Stream schema/examples, complete event/trust and
  correlation documentation, and bilingual extension/resource documentation;
  reject removed `billing_service` on every config surface.
- [ ] 6.4 Advance authoritative ai-quota version to `1.0.2` and align metadata,
  admission-only schema/examples, ownership language, and bilingual
  extension/resource documentation.
- [ ] 6.5 Advance authoritative ai-statistics version to `1.0.1` and align
  metadata, schema, examples, bounded route/path attribution and safe-base-fact
  language, and bilingual extension/resource documentation.
- [ ] 6.6 Add an automated version-alignment/single-source check that fails on
  drift among authoritative extension versions, resource `info.version`,
  plugin/option metadata, example artifact URLs, and documented versions.
- [ ] 6.7 Run schema/example validation, bilingual documentation consistency,
  version-alignment checks, and every affected independent extension module's
  formatting, tests, and vet/static checks.
- [ ] 6.8 Obtain an independent versions/schemas/resources/documentation
  review, resolve every finding, and rerun all affected alignment, schema,
  documentation, and module checks.

## 7. Reviewed Artifacts And Local Availability

- [ ] 7.1 Complete independent implementation review, resolve findings, run
  GitNexus `detect_changes`, and record the reviewed clean source result commit
  from which release bytes will be built.
- [ ] 7.2 Build and validate non-empty Wasm files at
  `build/plugins/ai-proxy/1.0.2/plugin.wasm`,
  `build/plugins/ai-billing/0.1.5/plugin.wasm`,
  `build/plugins/ai-quota/1.0.2/plugin.wasm`, and
  `build/plugins/ai-statistics/1.0.1/plugin.wasm`.
- [ ] 7.3 Record a release evidence manifest for each artifact containing
  repository, intentional base, reviewed result commit, plugin/version,
  versioned path, SHA-256, toolchain, exact build/test commands, and
  provenance.
- [ ] 7.4 Upload the exact four tested Wasm byte sequences to the supported
  local plugin server and verify each version is available through its normal
  local service interface.
- [ ] 7.5 Retrieve all four served artifacts and prove byte-for-byte SHA-256
  identity with the tested local files; reject and replace any mismatch.

## 8. Final Verification, Spec Sync, And Archive

- [ ] 8.1 From a clean reviewed state, rerun all affected independent plugin
  unit/integration/security suites, root tests, formatting, vet/static checks,
  version alignment, and four-plugin versioned build.
- [ ] 8.2 Refresh Graphify after code changes; run GitNexus change detection
  against the intentional base, inspect every affected execution flow, run
  final diff/secret/security review, and verify no unrelated active change was
  modified.
- [ ] 8.3 Run strict OpenSpec validation, verify every task has fresh evidence,
  and preserve the immutable result commit/artifact/upload/checksum manifest
  for downstream catalog consumption.
- [ ] 8.4 Before archive, synchronize the seven validated requirement deltas,
  manually rewrite the accepted `ai-billing-events` Purpose to global Redis
  Stream event delivery, and manually rewrite the accepted
  `ai-monetary-quota` Purpose plus its surviving account-ownership scenario to
  admission-only read-only Redis balance hot data. Archive only
  `add-ai-route-plugin-routing-contracts`; do not modify or archive
  `expose-ai-billing-request-correlation`,
  `fix-ai-memory-internal-cost-attribution`, or
  `fix-qwen-responses-capability-fallback`. Strictly validate the accepted
  tree, verify it contains every complete internal-cost, exact correlation,
  and Qwen sentinel scenario carried here, and perform a scoped grep plus
  semantic review of accepted `ai-monetary-quota` proving no response-stage
  balance change, effective-price key, response token parser, or
  missing-price/usage policy claim survives.
