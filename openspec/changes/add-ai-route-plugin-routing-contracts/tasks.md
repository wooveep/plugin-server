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

- [x] 2.2 Add RED adapter tests proving a stable known Responses/Claude/Qwen
  adapter is selected only when provider, method, path, and capability match
  and performs the expected request/response transformation; cover the
  Qwen-only `openai/v1/responses` sentinel for explicit empty, whitespace,
  omitted, and non-empty paths plus another provider's empty path.

Task 2.2 evidence (2026-07-29):

- The current Graphify report and the focused task 2.2 audit were reviewed
  before source inspection. The audit's upstream impact analysis reports LOW
  risk for the existing ai-proxy request resolver, capability, and converter
  production symbols. The CRITICAL shared `RunGoTest` (58 direct callers) and
  `NewTestHost` (209 direct callers) helpers were only reused and were not
  modified.
- Pre-change focused baselines passed:
  `go test . -run '^TestQwenResponsesFallbackConvertsNonStreamingRequestAndResponse$' -count=1`
  and
  `go test ./provider -run '^TestQwenProvider_ExplicitEmptyResponsesCapabilityDisablesProviderDefault$' -count=1`.
- `extensions/ai-proxy/known_adapter_contract_test.go` adds one top-level
  runtime contract. Its Qwen sentinel matrix passed explicit empty,
  whitespace-only, omitted, and non-empty capability cases plus OpenAI's
  existing empty-path key-presence behavior. The path, provider, and native
  capability mismatch cases also passed without claiming the Qwen
  Responses-to-Chat fallback.
- Focused RED command:
  `go test . -run '^TestKnownQwenResponsesAdapterContract$' -count=1`.
  It compiled and failed only on the two missing contracts. The exact existing
  Qwen request and response transformation assertions passed before the
  positive case reported absent test-local literal property
  `mse_ai_proxy_adapter_metadata`; the independent `GET /v1/responses`
  mismatch expected `/v1/responses` but observed the incorrectly selected
  `/compatible-mode/v1/chat/completions` fallback.
- The future metadata assertion decodes at most 1024 bytes of JSON and requires
  a non-empty stable adapter ID/version, normalized
  `openai/v1/responses`, provider match `qwen`, Chat Completions capability
  dependency, and non-empty usage profile across two identical matches. No
  production source, missing production constant/package, or shared test
  harness was added or modified.
- `graphify update .` refreshed the local AST graph after the test addition.
  Staged GitNexus change detection reported two changed files, zero affected
  execution flows, and LOW risk; its stale line mapping associated only
  documentation sections and found no production or shared-harness symbol
  impact.

- [x] 2.3 Add RED transparent-fallback tests using a non-generic provider,
  nontrivial method, path, raw query, opaque request body, and opaque response
  body; prove exact preservation plus provider-owned host/auth/timeout/failover.

Task 2.3 evidence (2026-07-29):

- The current Graphify report, task 2.3 fast audit, change design/specs, Qwen
  lifecycle fixtures, shared `NewTestHost`/`RunGoTest` harness, and failover
  parser/status behavior were reviewed before the test was added. The audit's
  read-only upstream impact results were LOW for the likely production seams;
  no production symbol or shared harness was modified. The new unindexed
  `TestTransparentQwenUnknownFallbackContract` symbol returned UNKNOWN with
  zero indexed dependents when rechecked before its test-only refinement.
- Pre-change focused baselines passed:
  `go test . -run '^TestQwenResponsesFallbackConvertsNonStreamingRequestAndResponse$' -count=1`
  and
  `go test ./provider -run '^(TestFailover_FromJson_FailoverOnStatus|TestFailover_Validate)$' -count=1`.
- `extensions/ai-proxy/transparent_fallback_contract_test.go` adds one
  non-generic Qwen HostEmulator contract using configured
  `qwenDomain: provider.example`, two provider tokens,
  `firstByteTimeout: 3210`, minimal valid status failover, attacker-controlled
  authority/auth/timeout, `PATCH
  /api/v1/vendor/jobs/job%2F42:cancel?sig=a%2Bb&dup=1&dup=2`, and opaque
  request/response byte slices. It independently asserts exact method, path,
  raw query, request bytes, response bytes, provider authority, provider
  authentication, and timeout authority without inventing fallback metadata.
- Focused RED command:
  `go test . -run '^TestTransparentQwenUnknownFallbackContract$' -count=1`.
  It compiled and reached only the missing transport contract after every
  preceding preservation/authority assertion passed: configured timeout
  expected `3210`, forwarded request-controlled timeout observed `1`.
- The independent failover-status subcase passed with
  `go test . -run '^TestTransparentQwenUnknownFallbackContract$/^go$/^configured_failover_status_removes_the_failed_provider_token$' -count=1`;
  a configured `429` removed the first provider token and the next request used
  the remaining provider token while retaining provider authority. This
  proves the existing failover path independently of the timeout RED.

- [x] 2.4 Add RED forced-passthrough tests proving an otherwise-known adapter
  does not transform request, non-streaming response, or streaming response
  bodies and does not publish the skipped adapter identity.

Task 2.4 evidence (2026-07-29):

- The current Graphify report, focused task 2.4 audit, proposal, design,
  ai-proxy routing and Responses deltas, task plan, existing Qwen Responses
  non-streaming/streaming controls, and shared HostEmulator APIs were reviewed
  before the test was added. GitNexus could not yet resolve the new
  `TestForcedQwenResponsesPassthroughContract` symbol and returned UNKNOWN
  with zero indexed dependents. No production symbol or CRITICAL shared
  `RunGoTest`/`NewTestHost` harness was modified.
- Both automatic-mode controls passed before the RED fixture:
  `go test . -run '^TestQwenResponsesFallbackConvertsNonStreamingRequestAndResponse$' -count=1`
  and
  `go test . -run '^TestQwenResponsesFallbackConvertsStreamingRequestAndSSE$' -count=1`.
- `extensions/ai-proxy/forced_passthrough_contract_test.go` adds one
  compile-safe top-level Go-mode contract with independent non-streaming and
  streaming subtests. Its literal configuration selects
  `protocol_mode: passthrough` for the otherwise-known Qwen
  Responses-to-Chat match without referring to a missing production field or
  getter. Both branches prove provider-owned authority and authentication
  still apply, then compare the `/v1/responses` path, request bytes, upstream
  response/SSE bytes, and skipped adapter identity at observable boundaries.
- Focused RED command:
  `go test . -run '^TestForcedQwenResponsesPassthroughContract$' -count=1`.
  It compiled, reached both independent request/response lifecycles, and
  failed only on the missing forced-passthrough contract. Both branches
  observed `/compatible-mode/v1/chat/completions` instead of
  `/v1/responses` and converted request bytes. The non-streaming branch
  converted the Chat-looking response into a Responses object, while the
  streaming branch converted the exact Chat Completions SSE chunk into
  Responses events. Non-fatal equality assertions recorded all six
  preservation failures in one run; provider host/authentication and the
  currently absent optional adapter-metadata property passed.
- `gofmt`, `git diff --check`, and strict OpenSpec validation pass.
  `graphify update .` refreshed the local AST graph after the test addition.
  Staged GitNexus change detection reported two changed files, zero affected
  execution flows, and LOW risk; the stale index mapped only the task
  sections and did not yet resolve the newly added test symbol.

- [x] 2.5 Add RED URL-boundary security tests for absolute URLs, scheme or
  authority injection, raw/encoded traversal, credential boundary, raw-query
  preservation, and safe diagnostics without secrets or bodies.

Task 2.5 evidence (2026-07-29):

- The current Graphify report, focused task 2.5 audit, design, ai-proxy routing
  delta, task plan, HostEmulator/TestHost lifecycle APIs, and existing Qwen
  fixtures were reviewed before the test was added. Upstream impact for the
  new `TestAIProxyURLBoundarySecurityContract` symbol was UNKNOWN with zero
  indexed dependents. The CRITICAL shared `RunGoTest`/`NewTestHost` harness and
  the MEDIUM `ProviderConfig.applyProviderBasePath` production symbol were
  only inspected or reused and were not modified.
- The existing focused Qwen lifecycle baseline passed:
  `go test . -run '^TestQwenResponsesFallbackConvertsNonStreamingRequestAndResponse$' -count=1`.
  The earlier Task 2.3 transparent-fallback fixture remained intentionally RED
  only on its independently recorded provider-timeout contract.
- `extensions/ai-proxy/url_boundary_security_contract_test.go` adds one
  top-level Go-mode contract with fresh Qwen hosts for six independent unsafe
  request targets. Each case requires local `400..599` rejection, and an
  unrejected target independently proves the provider credential boundary
  while recording the forwarded target and authorization. All cases aggregate
  every HostEmulator diagnostic channel (trace/debug/info/warn/error/critical)
  plus any local body and prohibit provider/client credentials, raw-query
  markers, and submitted body markers.
- Focused RED command:
  `go test . -run '^TestAIProxyURLBoundarySecurityContract$' -count=1`.
  It compiled and failed each of the absolute URL, scheme injection, network
  authority, encoded authority, raw traversal, and encoded traversal subtests:
  no local response was produced and every unsafe target carried the configured
  provider Bearer credential. Raw and encoded traversal were forwarded under
  `/tenant/base` without containment enforcement.
- The dedicated diagnostics subtest failed independently because the current
  implementation logged the complete provider configuration/token and the
  submitted Responses body. Its client-token and raw-query assertions passed.
  The positive guard passed independently with
  `go test . -run '^TestAIProxyURLBoundarySecurityContract$/^go$/^preserves_safe_origin_form_raw_query_with_provider_policy$' -count=1`,
  proving provider host/authentication plus exact duplicate ordering, escaping,
  and empty-value preservation for
  `sig=a%2Bb&dup=1&dup=2&empty=` after provider-base prefixing.
- Independent review identified three initially omitted HostEmulator log
  channels. The fixture now executes trace, debug, info, warn, error, and
  critical getters for the complete diagnostics aggregate without weakening
  any marker assertion. The exact focused rerun retained seven failing leaf
  subtests and 20 real assertion failures, while the safe-query guard and
  existing Qwen Responses lifecycle baseline still passed independently.
- No production source, shared test harness, protocol/path-scope fields, or
  missing production constants were added or modified.
- `gofmt`, `git diff --check`, and strict OpenSpec validation pass.
  `graphify update .` completed without tracked generated changes. Staged
  GitNexus change detection reported two changed files, zero affected
  execution flows, and LOW risk; the stale index mapped only adjacent task
  sections and did not yet resolve the new test symbol.

- [x] 2.6 Add RED path-scope tests proving legacy replay, bound-target
  provider-native coverage without synthesized suffixes, and explicit
  non-empty suffix/method intersections.

Task 2.6 evidence (2026-07-29):

- The current Graphify report, focused task 2.6 audit, design, ai-proxy routing
  and Responses deltas, task plan, existing Qwen Responses fallback control,
  and HostEmulator lifecycle patterns were reviewed before the test was added.
  Upstream impact for the new `TestAIProxyPathScopeContract` symbol was UNKNOWN
  with zero indexed dependents. The CRITICAL shared `RunGoTest` and
  `NewTestHost` helpers were only reused and were not modified.
- The existing legacy Qwen fallback baseline passed:
  `go test . -run '^TestQwenResponsesFallbackConvertsNonStreamingRequestAndResponse$' -count=1`.
  `extensions/ai-proxy/path_scope_contract_test.go` then added one compile-safe
  Go-mode contract using only literal `json.RawMessage` configuration and a
  fresh HostEmulator for every request. It does not reference future
  protocol/path-scope production fields, getters, enums, or shared helpers.
- Independent positive guards passed for omitted and explicit
  `legacy_suffixes` Qwen Responses classification, `bound_target` without
  advanced filters, `bound_target` with both filter arrays empty, and the
  non-empty suffix/method both-match case. They prove the current
  `/compatible-mode/v1/chat/completions` legacy replay, provider-native path
  coverage without synthesized suffixes, empty-filter no-narrowing, and
  provider authority/authentication on an admitted match. Opaque request bytes
  are checked only as an auxiliary positive guard.
- Focused RED command:
  `go test . -run '^TestAIProxyPathScopeContract$' -count=1`.
  It compiled and failed only for the missing non-empty filter intersection.
  The suffix-mismatch, method-mismatch, and both-mismatch requests all changed
  authority from `client.example` to `provider.example` and authorization from
  `Bearer client-token` to `Bearer provider-path-scope-token`; the contract
  requires those skipped requests to retain the client values. A dedicated
  mismatch-only rerun reproduced the same six real assertions independently.
- The HostEmulator represents traffic already selected by the Higress binding
  target, so the fixture tests only in-plugin narrowing and does not claim that
  a filter can expand an external binding. No exact callback action is asserted,
  and no production source or shared test harness changed.
- `gofmt`, `git diff --check`, and strict OpenSpec validation pass.
  `graphify update .` refreshed the local AST graph without adding tracked
  generated output. Staged GitNexus change detection reported two changed
  files, zero affected execution flows, and LOW risk; its stale symbol mapping
  resolved only adjacent task sections and not the newly added test symbol.

- [x] 2.7 Implement validated protocol/path-scope parsing, stable adapter
  resolution, trusted adapter metadata, and Responses adapter registration.

Task 2.7 evidence (2026-07-29):

- The focused production audit, Tasks 2.1/2.2/2.6 RED fixtures and reviews,
  current config/main/provider/Qwen/Responses lifecycle code, active design and
  delta specs, and Graphify report were reviewed before implementation.
  Exact upstream impact was rerun against the linked-worktree index for
  `PluginConfig`, `FromJson`, `Validate`, and `onHttpRequestHeader`; all four
  reported LOW risk, zero indexed direct callers, and zero execution flows.
  Manual review retained the known generic/callback caveat: these values flow
  through global/override parsing and every ai-proxy request-header callback.
- The three approved RED commands were replayed before production edits.
  Parser state lacked both getters and accepted an invalid mode; the known
  adapter contract lacked stable metadata and allowed GET to enter the old
  fallback branch; the three bound-target filter mismatch leaves all replaced
  client authority/authentication with provider values.
- `PluginConfig` now owns inherited route-effective `protocol_mode`,
  `path_scope`, `enable_path_suffixes`, and `enable_methods` state. Missing mode
  and scope resolve to `auto` and `legacy_suffixes`; explicit invalid or empty
  mode/scope and malformed/oversized filter input receive fixed bounded errors.
  A rule overrides only fields present in its JSON, copied filters receive
  independent backing arrays, list getters return clones, methods are
  normalized to uppercase HTTP tokens, and absent/empty bound-target filters
  add no narrowing while non-empty suffix/method lists intersect.
- A reviewed local adapter registry now resolves the existing
  Responses-to-Chat-Completions fallback from selected provider identity,
  uppercase POST, normalized Responses operation, unsupported native Responses
  capability, and supported Chat Completions capability. It preserves the Qwen
  empty/whitespace sentinel and the established converters, while GET, native
  Responses, provider/capability mismatch, and unrelated paths claim no
  adapter.
- Each request first receives a bounded no-match envelope in
  `mse_ai_proxy_adapter_metadata` and clears the existing Responses conversion
  marker. A matched adapter publishes at most 1024 JSON bytes containing only
  registry/provider/capability values for stable ID, version, normalized
  operation, provider match, capability match, and usage profile. Bound-target
  mismatch returns before reroute, context, token, host/authentication, or body
  work and preserves client authority/authentication/body ownership.
- Independent review found that `DontReadResponseBody` did not suppress the
  registered response-header callback, so a bound-target mismatch could still
  remove `Content-Length`, update failover token inventory, and issue a retry.
  A real HostEmulator RED reproduced all three effects, including same-context
  request re-entry. Every request-header entry now clears a request-scope skip
  marker plus both Claude and Responses conversion markers; a bound-target
  mismatch sets the skip marker, and the response-header callback returns
  before upstream classification, failover/retry, header transformation, or
  response-body setup. The regression proves exact response header/body
  preservation and unchanged two-token availability on the next admitted
  request, with no retry callout.
- A second independent lifecycle review found that keeping `DontReadRequestBody`
  and `DontReadResponseBody` in that mismatch branch permanently disabled the
  wrapper's body callbacks for later same-context re-entry. Those calls were
  removed from the mismatch branch. Request body, response headers, streaming
  response body, and buffered response body now all check the same request-scope
  skip marker and immediately preserve the supplied bytes/headers without
  provider, conversion, failover, retry, token, or logging work. HostEmulator
  RED cases proved the old failure and now prove admitted → mismatch → admitted
  re-entry restores Qwen Responses request conversion plus both non-streaming
  response and SSE conversion, while JSON mismatch callbacks remain exact
  no-ops.
- GREEN commands passed independently:
  `go test ./config -run '^TestPluginConfigRoutingParserContract$' -count=1`,
  `go test . -run '^TestKnownQwenResponsesAdapterContract$' -count=1`, and
  `go test . -run '^TestAIProxyPathScopeContract$' -count=1`. The complete
  config and provider packages passed, as did global/override parsing,
  Qwen non-streaming/streaming fallback, OpenRouter fallback, and the explicit
  empty Qwen capability sentinel. `go vet ./...` passed.
- The Task 2.3 transparent fallback contract remains RED only at the recorded
  provider-timeout authority assertion (`want 3210`, `got 1`). Task 2.4 remains
  RED because passthrough still executes and identifies the registered
  conversion for path, request body, non-streaming response, and SSE. Task 2.5
  retains the recorded unsafe URL/authority/traversal dispatch and
  credential/body diagnostic leakage failures; its safe raw-query guard
  remains outside this task.
- `graphify update .` rebuilt the local AST graph without tracked generated
  changes. `gofmt`, strict OpenSpec validation, diff checks, and staged
  GitNexus change detection were run before the task commit.

- [x] 2.8 Implement transparent unknown fallback and forced passthrough while
  preserving origin-form operation bytes and provider-owned transport policy.

Task 2.8 evidence (2026-07-29):

- The focused Task 2.8 audit, current Graphify report, active design/deltas,
  provider lifecycle, and all three approved RED fixtures were reviewed before
  production edits. Exact upstream impact was LOW for
  `parseGlobalConfig`, `parseOverrideRuleConfig`, `onHttpRequestHeader`,
  `onHttpRequestBody`, `onHttpResponseHeaders`, `onHttpResponseBody`, and
  `applyResponsesFallbackAdapter`; `onStreamingResponseBody` was also LOW with
  one direct and three total test dependents. No HIGH/CRITICAL symbol,
  `provider.replaceRequestBody`, shared harness, or converter was modified.
- The required pre-edit RED runs reproduced only their approved gaps.
  Transparent unknown Qwen traffic preserved opaque operation/query and bytes
  plus provider host/auth/failover, but expected configured timeout `3210` and
  observed request-controlled `1`. Forced Qwen Responses passthrough still
  rewrote the path and converted request, JSON response, and SSE bytes while
  publishing the skipped adapter identity. All six unsafe URL/authority/
  traversal targets were forwarded with provider credentials, and startup/body
  diagnostics exposed configured/submitted material.
- A bounded origin-form validator now rejects empty, relative, absolute,
  scheme/authority, raw or nested encoded authority/traversal, malformed
  escape, backslash, fragment, control, and over-decode targets before
  provider logging, context capture, token selection, or credential mutation.
  It validates up to six decode passes without rebuilding the request target
  or raw query, returns the established single-decoded classification path,
  and emits only fixed local `400`,
  `ai-proxy.invalid_request_target`, and `invalid request target` values.
- Explicit passthrough and true automatic no-match traffic now share a
  request-scoped transparent marker and bounded no-match metadata. Provider
  header processing receives an empty API name so authority,
  authentication, base-path policy, token inventory, and failover remain
  provider-owned while request/response adapters and body classification are
  skipped. Configured first-byte timeout replaces the client header at the
  header stage; an omitted provider timeout removes the client-controlled
  header. Response status/failover and success failure-count reset execute
  before transparent response adapters are suppressed.
- TDD added direct validator, fixed local-response/timeout, same-context
  transparent-to-known re-entry, and provider-original classification
  contracts. The validator first failed to compile, the transport contract
  then retained the injected timeout and lacked a local rejection, and the
  re-entry/provider-original contracts initially failed because wrapper
  `DontRead*` state is one-way and provider classification occurred after the
  transparent decision. Request-scoped callback guards now preserve bytes
  without permanently disabling later re-entry; auto original-provider
  classification remains authoritative, and legacy original/generic
  body-capable traffic retains its established header-stop action while the
  transparent guard prevents conversion. Explicit passthrough remains the
  unconditional transparent override.
- Lifecycle diagnostics touched by this task are categorical or byte-count
  only. Full config JSON, target/path/query values, request bodies, streaming
  chunks/events, and error chunks were removed from those logs. The legacy
  fixed substring `unsupported path` is retained without any submitted value.
- Required Task 2.3, 2.4, and 2.5 GREEN commands pass, as do their failover and
  safe raw-query positive guards. Tasks 2.1, 2.2, and 2.6 remain GREEN, along
  with Qwen Responses non-streaming/streaming controls and the complete config
  and provider packages. Serial provider regression groups covering every
  listed top-level provider suite passed after resolving Vertex/Bedrock
  original classification and Azure/Minimax/generic lifecycle compatibility.
- The exact aggregate gate passed:
  `go test ./... -count=1 -timeout=8m` (root package `245.483s`, config,
  provider, and util packages GREEN). Exact changed-file `gofmt`/`gofmt -d`,
  `go vet ./...`, `git diff --check`, and
  `openspec validate add-ai-route-plugin-routing-contracts --strict` also pass.
- `graphify update .` rebuilt 11,027 nodes, 14,631 edges, and 872 communities
  with no tracked generated changes. GitNexus compare-to-`main` reported the
  intentional cumulative branch as MEDIUM (32 files, one pre-existing
  `FromJson` flow from earlier work); isolated staged detection for Task 2.8
  reported LOW risk, six files, eight indexed existing symbols, zero affected
  processes, and no unexpected module, shared harness, or converter. Final
  staged detection including this task evidence remained LOW with seven files,
  12 indexed symbols/sections, and zero affected processes.

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
