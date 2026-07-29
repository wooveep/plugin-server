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

Task 2.8 review-fix evidence (2026-07-29):

- The independent `NEEDS_FIXES` report was reproduced before correction:
  normal/failover/retry bookkeeping logs exposed provider credentials,
  Claude Code/OpenAI custom/Azure full-service transparent requests still ran
  operation-mutating provider header pipelines, Qwen retry rebuilt an empty
  JSON `POST`, and decoded NUL/CRLF/DEL targets passed validation.
- Provider-owned transparent transport now has an explicit header policy.
  Claude, OpenAI, Azure, and Qwen implementations update only authority and
  credentials; the common layer preserves method, path, raw query,
  non-credential/protocol headers, and binary body while retaining configured
  legacy/provider base-path policy and first-byte timeout behavior. Automatic
  unknown and forced-passthrough contracts cover Claude Code, OpenAI direct
  custom URLs, and Azure full-service URLs.
- Transparent retry now replays the stored method, exact target and raw query,
  headers, and opaque body through a real Qwen callout, applying only the
  replacement credential and provider authority. Successful retry responses
  bypass protocol transforms and retain exact response bytes and headers.
- All selected credential, inventory, cooldown, health-check, failover, and
  retry diagnostics are categorical or count-only. Successful transparent,
  failover, and retry contracts aggregate every HostEmulator log level and
  reject provider credentials, client credentials, and submitted body markers.
- Every decoded validation pass now rejects C0 and DEL bytes. The contract
  covers direct and recursively encoded NUL, CRLF, DEL, backslash, fragment,
  authority, and traversal while retaining legal `job%2F42` classification and
  the exact duplicate/escaped raw query.
- Exact upstream impact was LOW for all behavior edits except
  `modifyApiToken` and `modifyApiTokenRequestCount`, which were HIGH because
  they participate in the established failover/CAS flows. Those two authorized
  edits changed only log strings and removed secret arguments; their shared
  state, CAS, counter, and control-flow behavior is unchanged.
- Focused Azure/Minimax/generic regressions passed in `23.738s`; focused
  Vertex/cooldown/path-scope regressions passed in `36.510s`; config, provider,
  and root wrapper packages passed. The serial aggregate
  `timeout 8m go test -count=1 -p 1 ./...` passed with the root package at
  `249.199s`. Extension and wrapper `go vet`, exact `gofmt`,
  `git diff --check`, and strict OpenSpec validation also passed.
- `graphify update .` rebuilt 11,049 nodes, 14,688 edges, and 876 communities
  with no tracked generated changes. GitNexus incremental analysis rebuilt
  12,818 nodes, 39,396 edges, 287 clusters, and 300 flows. Final change
  detection remains HIGH only for the two authorized log-only failover
  symbols and their ten pre-existing token-availability processes; no new
  unreviewed HIGH/CRITICAL behavior edit was introduced.

Task 2.8 second review-fix evidence (2026-07-29):

- The second independent `NEEDS_FIXES` report was reproduced with new
  HostEmulator contracts before production edits. Vertex Express automatic
  unknown and forced passthrough retained the client `x-goog-api-key`;
  Bedrock AK/SK automatic unknown and forced passthrough retained the client
  Bearer value instead of producing SigV4; Vertex standard OAuth returned
  `ActionContinue` without its token callout; and a focused Bedrock unit RED
  proved query-bearing and query-free targets produced the same signature.
- A transparent body-authentication interface now receives a clone of the
  exact opaque body and request headers. The common policy copies back only
  provider credential/signing headers, so method, exact target/raw query,
  protocol headers, and body cannot be changed by the hook. The live
  transparent body callback stores the exact bytes before authentication, and
  transparent retry invokes the same hook after selecting the replacement
  credential and before dispatch.
- Vertex has an explicit transport-only header policy. Express mode uses the
  provider-supported `x-goog-api-key` header without rewriting the target or
  raw query; standard mode uses the existing OAuth callout/callback and cached
  Bearer-token model. Bedrock has an explicit transport-only header policy for
  bearer mode and signs the exact method, canonical query derived from the
  unchanged request target, provider authority, and opaque body for AK/SK
  mode. Client-supplied Google/AWS authentication and signing headers are
  removed.
- Automatic-unknown and forced-passthrough Go/Wasm contracts cover Vertex
  Express, Vertex standard OAuth with a real `/token` callout, Bedrock bearer,
  and Bedrock AK/SK. A Vertex Express retry contract proves the replay keeps
  exact `PATCH`, target/raw query, non-credential headers, and binary body
  while changing to the remaining request-scoped `x-goog-api-key`.
  Bedrock signing unit coverage independently proves changes to method, query,
  or body change the signature.
- A Go HostEmulator-only, read-only context oracle inspects the actual current
  request context without production hooks or credential logs. Google,
  Anthropic, and Vertex Raw header-to-body lifecycle cases seed two different
  consecutive random choices, capture the exact header-phase selection, and
  require the final key to equal it. The same cases aggregate every
  HostEmulator log level and reject both configured credentials.
- Exact pre-edit upstream impact was LOW for `onHttpRequestBody`,
  `sendRetryRequest`, `setAuthHeaders`, `generateSignatureWithService`,
  `generateSignature`, and the inspected Vertex lifecycle tests. No new
  HIGH/CRITICAL symbol was edited. Post-change GitNexus detection reports HIGH
  only at aggregate scope while listing the ten reviewed existing
  `onHttpRequestBody`/Bedrock `OnRequestBody` and SigV4 process families; the
  exact behavior edits and their direct callers were independently reviewed
  and fully exercised.
- The new contracts pass in Go/Wasm mode, the exact-selection oracle passes in
  Go HostEmulator mode, the complete provider package passes, and combined
  Vertex/Bedrock/Qwen transparent regressions pass. The exact serial aggregate
  `timeout 8m go test -count=1 -p 1 ./...` passes with the root package at
  `258.488s`; extension and root-wrapper vet, root-wrapper tests, exact
  `gofmt`, `git diff --check`, and strict OpenSpec validation also pass.
  Root-wide `go vet ./...` continues to report only the pre-existing unchanged
  `pkg/ai/sessionctx/sessionctx.go:512` unreachable-code finding.
- `graphify update .` rebuilt 11,064 nodes, 14,740 edges, and 873 communities
  without tracked generated changes. GitNexus incremental analysis rebuilt
  12,847 nodes, 39,595 edges, 287 clusters, and 300 flows.

Task 2.8 third review-fix evidence (2026-07-29):

- The third independent `NEEDS_FIXES` report was reproduced with new
  Go/Wasm HostEmulator contracts before production edits. Bodyless Vertex
  Express automatic unknown and forced passthrough requests lacked
  `x-goog-api-key`; bodyless Bedrock AK/SK requests lacked SigV4; and bodyless
  standard Vertex returned `ActionContinue` without a token callout. OAuth
  non-200 and no-status callbacks resumed a terminal stream, malformed 200
  installed an invalid Bearer value without a local error, and the common HTTP
  logger exposed the complete JWT-bearer form plus provider response bodies.
  A synchronous authentication-dispatch error also returned
  `ActionContinue`. The Bedrock bearer exact-replay/rotated-Authorization retry
  fixture passed as an existing-behavior characterization.
- The transparent header lifecycle now executes the same credential-only
  authentication hook with an exact empty body when
  `HasRequestBody()` is false and returns its action. Body-bearing traffic
  continues to authenticate only from the body callback, while retry invokes
  the same hook after credential rotation. Automatic-unknown and forced
  contracts cover bodyless Vertex Express, standard OAuth, Bedrock bearer, and
  Bedrock AK/SK while preserving exact `DELETE`, origin-form target/raw query,
  protocol headers, and empty body.
- Vertex token exchange now uses a dedicated sensitive HTTP callout path
  instead of the common request/response-body logger. The new path never logs
  the JWT assertion/form or provider response body. Non-200, malformed 200,
  no-status timeout, and synchronous dispatch-error contracts require one
  fixed local 500, a terminal paused stream, no invalid Bearer header, and no
  operation/body mutation. Only a valid non-empty header-safe access token is
  installed and followed by `ResumeHttpRequest`. Success and every callback
  failure aggregate trace/debug/info/warn/error/critical logs and reject the
  assertion/form, service-account/project markers, access token, provider
  error body, operation target, and request-body marker.
- Exact pre-edit upstream impact was LOW for `onHttpRequestHeader`,
  `onHttpRequestBody`, Vertex `getAccessToken`, Vertex `getToken`, Bedrock
  `generateSignatureWithService`, and `encodeSigV4Path`; the Vertex provider
  struct was MEDIUM and not modified. The common `HttpCall` symbol was MEDIUM
  with ten direct callers and was deliberately not modified; the sensitive
  callout is a new narrow symbol. No HIGH/CRITICAL production symbol changed.
- The new contracts and all retained body-authentication, exact Vertex
  credential-selection, transparent logging, Qwen retry, and provider retry
  contracts pass in Go/Wasm mode. The combined focused suite passed in
  `25.455s`; complete Vertex/Bedrock/transparent regression passed in
  `78.667s`; provider, config, and root-wrapper packages passed. The exact
  serial aggregate `timeout 8m go test -count=1 -p 1 ./...` passed with the
  root package at `272.956s`. Extension and root-wrapper vet, exact `gofmt`,
  `git diff --check`, and strict OpenSpec validation pass. Root-wide
  `go vet ./...` still reports only the pre-existing unchanged
  `pkg/ai/sessionctx/sessionctx.go:512` unreachable-code finding.
- The pre-commit graph refresh is unambiguously the five-file Task 2.8 third
  review-fix worktree rooted at `c0ad10c8`. `graphify update .` rebuilt
  11,083 nodes, 14,797 edges, and 859 communities. GitNexus incremental
  analysis rebuilt 12,878 nodes, 39,918 edges, 284 clusters, and 300 flows.
  Generated GitNexus instruction-file rewrites were restored before staging;
  the final staged detector and clean-commit refresh are recorded in the
  implementation report.

Task 2.8 fourth review-fix evidence (2026-07-29):

- The fourth independent `NEEDS_FIXES` token fixtures were reproduced before
  production edits. The validator unit suite failed only because the unified
  validator did not exist. Go/Wasm HostEmulator OAuth 200 callbacks accepted
  NUL, non-CR/LF C0, DEL, non-ASCII, invalid punctuation, malformed padding,
  and 8 KiB-plus-one access tokens without a local error; unsafe cached tokens
  were installed immediately instead of remaining paused and re-fetching.
- Vertex now uses one 8 KiB-bounded RFC 6750 `b64token` validator for both
  successful OAuth response data and fresh shared-cache reads. It requires at
  least one ASCII letter, digit, or `-._~+/` character, permits `=` only as a
  trailing suffix, and rejects empty, oversized, whitespace/control, DEL,
  non-ASCII, invalid-punctuation, leading-padding, and data-after-padding
  values. Callback data is validated before header installation or cache
  storage; an invalid cache entry is treated as a miss and triggers the real
  sensitive token callout without installation or premature resume.
- Go/Wasm contracts cover every required unsafe token class, no header/cache
  installation, exact operation/body preservation, terminal pause/local 500
  behavior, cache miss/re-fetch, and all-level token/body log exclusion. A
  Go-only counter proves the invalid-success callback invokes the fixed local
  error handler exactly once. A complete legal RFC 6750 token covering
  `ALPHA`, `DIGIT`, `-._~+/`, and trailing `=` remains successful.
- The SDK HostEmulator's `ProxyHttpCall` implementation always returns
  `StatusOK` and exposes no synchronous dispatch-error injection API. The
  permitted wrapper-level alternative therefore uses a narrow dispatch seam
  below the real `ClusterClient.PostSensitive -> SensitiveHttpCall` path. Its
  contract proves old pseudo-headers are replaced exactly once, URL authority
  and path/raw-query are preserved, the first supplied timeout is selected,
  the exact opaque body reaches dispatch, a synchronous error is returned
  unchanged, and the callback does not run. The retained real Vertex
  HostEmulator contract also covers missing and malformed status callbacks,
  terminal local failure, and all-level response-body secrecy.
- Exact pre-edit upstream impact was LOW for Vertex `getAccessToken`,
  `getToken`, `getCachedAccessToken`, `setCachedAccessToken`, wrapper
  `SensitiveHttpCall`, and the two amended test symbols. The cache writer had
  one direct caller, four total affected symbols, and one existing
  `OnRequestBody` family; every other exact query reported zero direct callers
  or flows. No new HIGH/CRITICAL symbol was edited.
- The 15-test combined transparent/OAuth/Bedrock/Vertex focused suite passed
  in `33.805s`; the complete Vertex/Bedrock/transparent group passed in
  `87.169s`; provider, config, and root-wrapper packages passed. The exact
  serial aggregate `timeout 8m go test -count=1 -p 1 ./...` passed with the
  root ai-proxy package at `281.613s`. Extension and root-wrapper vet, exact
  `gofmt`, `git diff --check`, and strict OpenSpec validation also pass.
  Root-wide `go vet ./...` continues to report only the pre-existing
  `pkg/ai/sessionctx/sessionctx.go:512` unreachable-code finding; that file's
  blob remains `e24b0a62e9bccff0023ad0b3e6efd1922693dcbe` at both
  `4451eb84` and this worktree.
- The pre-commit graph refresh is the five-file fourth review-fix worktree
  rooted at `4451eb84`. `graphify update .` rebuilt 11,096 nodes, 14,841
  edges, and 862 communities. GitNexus incremental analysis rebuilt 12,893
  nodes, 40,055 edges, 281 clusters, and 300 flows. Generated GitNexus
  instruction-file rewrites were restored before staging.

- [x] 2.9 Run focused and complete independent ai-proxy unit, proxy-wasm
  lifecycle/integration, streaming, regression, and security suites.

Task 2.9 evidence (2026-07-29):

- The independent verification used final source result
  `5ae7de99960efd41273ea16e0489fee9be40943e`. The fifth Task 2.8
  implementation/security re-review was `APPROVED` with no Critical,
  Important, or Minor finding. Task 2.9 records only the completed independent
  suite gates; the separate formal Task 2.10 review checkbox remains open.
- The 15-contract transparent/OAuth/Vertex/Bedrock focused command passed in
  `33.703s`, and the complete
  `TestVertex|TestBedrock|TestTransparent` regression group passed in
  `86.869s`. The isolated serial
  `go test -count=1 -p 1 ./...` gate passed with exact package results:
  root ai-proxy `282.197s`, config `0.002s`, provider `0.008s`, and util
  `0.002s`.
- Independent provider/config and wrapper gates passed: the complete config
  and provider packages in the serial aggregate; focused
  `TestIsValidOAuthBearerToken` in provider at `0.002s`;
  `go test ./pkg/wrapper -count=1` at `0.001s`; and
  `go test -race ./pkg/wrapper -count=1` at `1.007s`.
- The suites cover protocol defaults and invalid input, stable adapter
  resolution and Qwen empty/whitespace/omitted/non-empty capability
  sentinels, automatic unknown and forced passthrough routing, legacy and
  bound-target path-scope intersections, absolute/scheme/authority/traversal
  rejection, safe exact raw-query preservation, client-credential stripping,
  provider host/authentication/timeout/failover authority, exact opaque
  request/response bytes, streaming and non-streaming lifecycle, Qwen exact
  retry replay, and retained Claude/OpenAI/Azure/Responses regressions.
- Vertex Express, standard OAuth, Bedrock bearer, and Bedrock SigV4 coverage
  includes body-bearing and bodyless authentication, exact method/target/
  query/header/body preservation, credential selection and rotation, OAuth
  callback success and terminal failures, unsafe shared-cache refetch, the
  8 KiB-bounded RFC 6750 bearer-token alphabet/padding contract, sensitive
  synchronous dispatch failure, and all-level credential/token/body log
  exclusion.
- Extension-wide `go vet ./...`, root-wrapper `go vet ./pkg/wrapper`, strict
  OpenSpec validation, exact changed-file `gofmt -l`, and reviewed-range plus
  merge-base `git diff --check` gates passed. Root-wide `go vet ./...`
  continues to report only the pre-existing
  `pkg/ai/sessionctx/sessionctx.go:512:2` unreachable-code finding; that file
  remains blob `e24b0a62e9bccff0023ad0b3e6efd1922693dcbe` at both ends of
  the reviewed range.
- An initial accidental overlap of two ai-proxy processes sharing
  `wasm-unit-test.wasm` was deliberately interrupted with `SIGINT` and is not
  gate evidence. Every result above came from a subsequent isolated serial
  run. Read-only validator and unsafe-cache lifecycle review overlays passed
  and were removed without modifying production or tracked test source.
- The Task 2.9 documentation-only pre-commit GitNexus detector reports LOW
  risk: one changed file, three touched documentation sections, zero affected
  processes, and no production execution-flow change.

- [x] 2.10 Obtain an independent ai-proxy implementation/security review,
  resolve every finding, and rerun the affected RED/GREEN and complete module
  suites before beginning the billing implementation.

Task 2.10 evidence (2026-07-29):

- Independent review was a blocking gate across five exact source results.
  Initial implementation `a4faf8ba25bd8945761e0bf50a125e37f88b5758`
  received `NEEDS_FIXES` for credential disclosure across normal/failover/
  retry bookkeeping, provider-specific transparent operation mutation,
  destructive non-exact Qwen retry replay, and decoded control-octet
  acceptance. Repair
  `db04531cb7116ac03253c45144c8b0c235592342` closed those findings before
  rereview.
- The `db04531cb711` rereview remained `NEEDS_FIXES`: Vertex and Bedrock
  body-stage authentication was absent for forced/automatic transparent
  requests, and three Vertex lifecycle assertions no longer proved exact
  reuse of the header-selected credential. Repair
  `c0ad10c863f2cc7a1d1582390f1a8623047e4bf1` added a credential-only
  opaque-body hook, exact Vertex/Bedrock coverage, SigV4 method/query/body
  binding, retry reauthentication, and a read-only exact-selection oracle.
- The `c0ad10c863f2` rereview remained `NEEDS_FIXES`: bodyless Vertex Express,
  Vertex OAuth, and Bedrock AK/SK skipped authentication; OAuth callback
  failures resumed after a terminal local response; and the token exchange
  exposed its JWT-bearer form and provider response bodies. It also requested
  exact Bedrock bearer retry, corrected SigV4 commentary, and unambiguous graph
  evidence. Repair
  `4451eb843930137a0f7123c2b1aeef5fb363bfe9` closed those lifecycle,
  credential-secrecy, retry, comment, and evidence findings.
- The `4451eb843930` rereview remained `NEEDS_FIXES`: successful OAuth and
  shared-cache paths still accepted header-unsafe or oversized bearer values,
  and the synchronous-dispatch regression did not traverse the real
  `ClusterClient.PostSensitive -> SensitiveHttpCall` path. Repair
  `5ae7de99960efd41273ea16e0489fee9be40943e` added one 8 KiB-bounded
  RFC 6750 validator at both credential boundaries, unsafe-cache miss/refetch,
  exact terminal failure behavior, exhaustive unsafe/legal token contracts,
  and a narrow real sensitive-dispatch seam/oracle.
- The final independent report
  `.superpowers/sdd/add-ai-route-plugin-routing-contracts/task-2.8-rereviewer-v4.md`
  has SHA-256
  `ccb0e4b5d313f0998ee7eeb59bded253c4814222e82437d355e2cd3a161ab76e`.
  It reviewed exact result `5ae7de99960e`, returned `APPROVED`, and reported
  zero Critical, Important, or Minor findings. Its immutable review package
  exactly matched `git diff --binary 4451eb843930..5ae7de99960e`.
- Final GitNexus comparison deliberately retains its `CRITICAL` aggregate
  classification: six files, 43 changed symbols, and 31 affected mappings.
  Thirty mappings are conservative shared `ClusterClient` constructor,
  provider `CreateProvider`, or config `Complete` flows; the remaining mapping
  is the reviewed real sensitive-callout path. The review traced the shared
  type/interface, exact validator/cache/OAuth callers, concrete
  `PostSensitive -> SensitiveHttpCall` edge, sole production Vertex consumer,
  and dispatch construction layer by layer rather than downgrading the risk.
  The original `HttpCall` and all ten ordinary
  `Get/Head/Options/Post/Put/Patch/Delete/Connect/Trace/Call` methods have zero
  source or behavior diff; wrapper full and race gates independently preserve
  that boundary.
- After every finding was resolved, the exclusive serial final gates passed:
  15 focused transparent/OAuth/Vertex/Bedrock contracts in `33.703s`;
  complete `TestVertex|TestBedrock|TestTransparent` coverage in `86.869s`;
  and `go test -count=1 -p 1 ./...` with root ai-proxy `282.197s`, config
  `0.002s`, provider `0.008s`, and util `0.002s`. Focused provider RFC 6750
  validation passed in `0.002s`; root-wrapper full and race suites passed in
  `0.001s` and `1.007s`.
- Extension-wide vet, root-wrapper vet, strict OpenSpec validation, exact
  changed-file formatting, reviewed-range and merge-base diff checks, and
  review-package byte comparison passed. Root-wide vet retains only the
  pre-existing `pkg/ai/sessionctx/sessionctx.go:512:2` unreachable-code
  finding, whose file blob remains
  `e24b0a62e9bccff0023ad0b3e6efd1922693dcbe` at both reviewed endpoints.
  The earlier overlapping ai-proxy runs were interrupted with `SIGINT` and
  excluded; all cited final results are later isolated serial runs.
- No finding was waived, downgraded, or left open: each `NEEDS_FIXES` result
  blocked progression until its repair and independent rereview. The complete
  ai-proxy implementation/security gate is now closed, so billing Task 3.1 may
  begin; no 3.x checkbox or implementation is changed by this closeout. The
  documentation-only pre-commit GitNexus detector reports LOW risk: one
  changed file, four touched documentation sections, zero affected processes,
  and no production execution-flow change.

## 3. AI Billing Red-Green Contracts

- [x] 3.1 Add RED configuration, Redis delivery, and request-gating tests:
  global `redis_stream` requirement/defaults, global and rule-level removed
  `billing_service` rejection, rule-level `redis_stream` rejection, rule
  inheritance without target replacement, omitted/explicit legacy scope,
  bound target without suffixes, provider-native paths, and explicit non-empty
  suffix/method intersections.

Task 3.1 evidence (2026-07-29):

- The current Graphify report, proposal/design/task plan, all seven delta
  capabilities, accepted billing/resource specifications, and the completed
  correlation/internal-cost dependency changes were reviewed before source
  inspection. GitNexus located the real `parseConfig`, `parseRuleConfig`,
  `isAIPathEnabled`, and HostEmulator billing-delivery seams. Pre-edit impact
  for the four new test-only symbols was UNKNOWN with zero indexed dependents;
  no production symbol or CRITICAL shared `RunGoTest`/`NewTestHost` helper was
  modified.
- Pre-change controls passed:
  `go test . -run '^TestParseConfig$' -count=1`,
  `go test . -run '^TestBillingEventDelivery$' -count=1`, and
  `go test . -run
  '^(TestBillingResponseRequestIDHeader|TestUnsafeInternalAttributionDoesNotLogSubmittedValue|TestPathFiltering)$'
  -count=1`. They retain Redis delivery, legacy gating, exact response
  correlation, and internal-attribution behavior.
- `extensions/ai-billing/routing_delivery_contract_test.go` adds independent
  Go HostEmulator configuration and request-lifecycle contracts. The passing
  controls prove the required global Redis target and exact defaults, removed
  global/rule `billing_service` rejection, rule-level `redis_stream`
  rejection, full target inheritance, legacy empty-list rejection, and
  omitted/explicit legacy suffix gating. Positive delivery decodes the real
  inherited `XADD global:billing:events * event <json>` command; no production
  or shared test-harness code changed.
- Focused RED command:
  `go test . -run
  '^(TestBillingRoutingAndDeliveryConfigContract|TestBillingRequestGatingContract)$'
  -count=1`. It compiled and failed only at the missing routing contracts:
  bound target rejected an explicitly empty advanced-filter list; global
  bound target, an empty-method-filter target, and a rule-level bound target
  emitted no Redis event for provider-native paths; and a non-matching `GET`
  incorrectly emitted an event because the non-empty method intersection was
  ignored. The matching `POST` and non-matching suffix controls passed. The
  existing global and rule leaves also retain provider-native runtime `XADD`
  oracles after explicit empty suffix/method filters, so later parser
  acceptance cannot hide a deny-all runtime or global/rule parser divergence.
- The new Redis/configuration and omitted/explicit legacy control subsets pass
  independently. Extension vet, exact changed-file `gofmt`, `git diff
  --check`, and strict OpenSpec validation pass. `graphify update .` rebuilt
  11,101 nodes, 14,858 edges, and 864 communities. Staged GitNexus detection
  reports LOW risk, two changed files, zero affected processes, and only stale
  documentation-section associations; production execution flows are
  unchanged.

- [x] 3.2 Add RED event tests proving trustworthy provider usage remains
  authoritative and opaque native responses produce `usage_missing=true`
  without fabricated model, token, provider-usage, price, or cost facts;
  verify the complete configuration/event surface including `event_kind`,
  `cost_source`, `worker_kind`, `source_job`, `bill_customer`, and
  `upstream_invoked`, plus fail-open Redis `XADD` delivery.

Task 3.2 evidence (2026-07-29):

- The current Graphify report, accepted and delta `ai-billing-events`
  specifications, change proposal/design/tasks, and the pre-existing
  correlation and internal-cost changes were reviewed before source
  inspection. GitNexus located the billing event-construction, provider-usage,
  cache-attribution, streaming, and Redis-delivery seams. Pre-edit impact for
  both new test symbols was UNKNOWN with zero indexed dependents; the
  documentation-section impact was LOW with zero affected processes. No
  production symbol or CRITICAL shared `RunGoTest`/`NewTestHost` harness was
  modified.
- The pre-change delivery/correlation/internal-cost/cache/stream baseline
  passed:
  `go test . -run
  '^(TestBillingEventDelivery|TestBillingResponseRequestIDHeader|TestUnsafeInternalAttributionDoesNotLogSubmittedValue|TestBuildBillingEvent_TrustedCacheReplayFacts|TestBuildBillingEvent_UsesLocalResponseCodeDetailsForCacheReplay|TestBillingEventIgnoresUserSuppliedUpstreamInvokedFields|TestStreamDoneFallbackDeliversProviderUsage|TestStreamDoneFallbackDeliversMissingUsage|TestStreamDoneFallbackSkipsAfterNormalDelivery|TestOnHttpStreamingResponseBodyEndOfStreamDeliversAndMarksDelivered|TestDeliverBillingEventDispatchErrorIsFailOpen)$'
  -count=1`.
- `extensions/ai-billing/truthful_event_contract_test.go` uses explicit
  provider-native suffixes so Task 3.1 path-scope RED does not mask the event
  contract. Its passing provider leaf proves provider counts remain
  authoritative over locally estimable text, retains the complete raw usage,
  model/provider/route/quota/status/price-version facts, and omits
  gateway-calculated cost. Its passing event-kind matrix proves configured
  internal `cost_source`, `worker_kind`, `source_job`, `bill_customer=false`,
  and tenant omission, while a customer cache hit retains trusted
  `upstream_invoked=false` and omits internal-only fields.
- Focused RED command:
  `go test . -run '^TestBillingTruthfulEventContract$' -count=1`. The known
  provider, internal-cost, and cache controls pass. Only three opaque-response
  leaves fail: non-streaming HTTP 202, non-streaming HTTP 502, and streaming
  HTTP 200 each serialize `model.name=unknown` where the contract requires an
  empty model fact. Before that final assertion, each leaf proves the original
  response body and status remain truthful, exactly one Redis event is
  attempted, `usage_missing=true`, `usage_source=missing`, all token counts
  are zero, provider usage/details and price/cost fields are absent, and
  `upstream_invoked` remains trusted.
- `extensions/ai-billing/redis_fail_open_contract_test.go` passes with
  `go test . -run '^TestBillingRedisFailOpenContract$' -count=1`. It decodes
  the exact configured
  `XADD global:billing:events * event <serialized-json>` attempt for dispatch
  failure, proves the event is marked delivered after one attempt, and proves
  a Redis command-response error preserves the buffered upstream response,
  `ActionContinue`, and at-most-once stream-done behavior. Raw request API-key
  markers are absent from both serialized events and all host log levels.
- Extension vet, exact changed-file `gofmt`, `git diff --check`, and strict
  OpenSpec validation pass. `graphify update .` rebuilt 11,105 nodes, 14,874
  edges, and 855 communities without tracked generated-file changes. After
  restoring the eight GitNexus-generated instruction files, staged detection
  reports LOW risk across three changed files and ten test/documentation
  symbols, with zero affected production processes.

- [x] 3.3 Add RED regression/security fixtures for all five internal-cost
  scenarios (complete pair, absent-pair fallback, partial suppression,
  empty/oversized/invalid-character suppression, and customer no-spoof) and
  the exact `x-ocmf-gateway-request-id` lifecycle (replace-not-append,
  canonical-absent removal, non-streaming, streaming, and local/cache paths);
  also preserve event identity, idempotent streaming delivery, cache facts,
  Redis credential redaction, and submitted-value non-leakage.

Task 3.3 evidence (2026-07-29):

- The current Graphify report, accepted and delta `ai-billing-events`
  specifications, change design/tasks, and both active correlation and
  internal-cost dependency changes and tests were reviewed before source
  inspection. GitNexus located the real request/response lifecycle,
  attribution, cache-origin, streaming-delivery, and HostEmulator seams.
  Pre-edit impact for both new test-only symbols was UNKNOWN with zero indexed
  dependents; the documentation file was likewise unindexed with zero
  dependents. No production symbol or CRITICAL shared `RunGoTest`/
  `NewTestHost` harness was modified, and every subtest owns a fresh host.
- The pre-change delivery/correlation/internal-cost/cache/stream baseline
  passed:
  `go test . -run
  '^(TestBillingEventDelivery|TestUnsafeInternalAttributionDoesNotLogSubmittedValue|TestBillingResponseRequestIDHeader|TestBuildBillingEvent_TrustedCacheReplayFacts|TestBuildBillingEvent_UsesLocalResponseCodeDetailsForCacheReplay|TestBillingEventIgnoresUserSuppliedUpstreamInvokedFields|TestSendBillingEventReusesEventIdempotencyKey|TestInitBillingRequestContextSetsEventID|TestOnHttpStreamingResponseBodyEndOfStreamDeliversAndMarksDelivered|TestStreamDoneFallbackSkipsAfterNormalDelivery|TestRedisFailOpenContract|TestBillingRedisFailOpenContract)$'
  -count=1`.
- `extensions/ai-billing/internal_cost_security_contract_test.go` passes with
  `go test . -run '^TestBillingInternalCostAndSensitiveDataContract$'
  -count=1`. Real request/response/event observations prove a complete valid
  pair is authoritative, an absent pair retains the configured fallback,
  source-only/worker-only/trimmed-empty/oversized/invalid-character pairs
  suppress event delivery without suppressing model traffic or leaking the
  submitted marker at trace-through-critical host log levels, and customer
  headers cannot spoof internal fields, `event_kind`, or `bill_customer`.
  Independent review repair gives the accepted pair, configured fallback, and
  customer-spoofed cost-source/worker/source-job values unique markers. The
  accepted/fallback markers remain authoritative in the event and the
  customer markers remain absent from it, while a shared test-only log
  collector proves every marker stays out of trace, debug, info, warn, error,
  and critical host logs.
- Focused RED command:
  `go test . -run '^TestBillingCorrelationLifecycleContract$' -count=1`.
  It compiles and fails only at six missing security/lifecycle contracts:
  non-streaming, streaming, cache replay, and gateway-local responses retain
  the second upstream correlation value before the canonical ID instead of
  replacing all values; a response without canonical context retains an
  upstream value instead of removing the header; and a submitted query marker
  enters `request_path` in the serialized event. The same uninterrupted
  lifecycle leaves prove event correlation exactly matches the canonical
  request ID, non-empty `event_id` equals `idempotency_key`, streaming emits
  exactly one `XADD` even after stream completion, and cache/local origin
  remains `upstream_invoked=false`. Redis credentials and submitted
  header/body/internal-attribution values remain absent from serialized events
  and trace-through-critical host logs.
- These are independent RED regression/security fixtures only; no production
  or shared test-harness code changed. Exact changed-file `gofmt`, extension
  vet, `git diff --check`, strict OpenSpec validation, Graphify refresh, and
  staged GitNexus change detection complete the Task 3.3 handoff. `graphify
  update .` rebuilt 11,109 nodes, 14,889 edges, and 877 communities without
  tracked generated-file changes. After restoring the eight
  GitNexus-generated instruction files, staged detection reports LOW risk
  across three changed files and nine test/documentation symbols, with zero
  affected production processes. The review repair likewise reports LOW risk
  across two changed files and three test/documentation symbols, with zero
  affected processes; the shared log collector has one direct test caller and
  no production process participation.

- [x] 3.4 Implement ai-billing path-scope parsing and bound-target/advanced
  filter gating without widening legacy configurations.

Task 3.4 evidence (2026-07-29):

- The current Graphify report, change design/tasks, complete
  `ai-billing-events` delta, accepted billing specification, Task 3.1 RED
  contracts, and the existing ai-proxy routing parser were reviewed before
  production edits. Exact GitNexus upstream impact was LOW for `parseConfig`
  (zero direct callers/processes), `parseRuleConfig` (zero direct
  callers/processes), `isAIPathEnabled` (one direct caller and zero
  processes), `parseConfigFields` (one direct caller and zero processes),
  `parsePathSuffixes` (two direct and one indirect caller, zero processes),
  and `onHttpRequestHeaders` (zero direct callers/processes). `BillingConfig`
  was HIGH with 16 direct event/cache/stream test dependents and zero indexed
  processes; this was reported before edits and explicitly authorized, and
  all 16 dependents pass after the change.
- The focused pre-edit command
  `go test . -run
  '^(TestBillingRoutingAndDeliveryConfigContract|TestBillingRequestGatingContract)$'
  -count=1` failed at exactly the five intended leaves: explicit empty
  bound-target filters, global provider-native bound target, empty
  bound-target method filters, rule bound target without inherited legacy
  suffix gating, and non-empty suffix/method intersection. The identical
  post-edit command passes.
- `BillingConfig` now carries route-effective `path_scope` and
  `enable_methods`. Global and rule parsers default omitted scope to
  `legacy_suffixes`, reject unsupported scope values, trim and validate
  case-sensitive/deduplicated HTTP method tokens, preserve legacy
  default/explicit suffix behavior without new count or length limits, and
  accept absent or literal-empty advanced filters only as non-narrowing
  bound-target projections. A source list that is non-empty but normalizes to
  no usable entry is rejected rather than becoming an allow-all projection. A
  rule changing from legacy to bound-target scope does not inherit the legacy
  suffix projection; rule filters still replace only their corresponding
  request gate, while the complete global Redis target and omitted business
  fields remain inherited. Removed `billing_service` and rule-level
  `redis_stream` rejection are unchanged.
- Request admission remains suffix-required in legacy mode. Bound-target mode
  admits the already selected Higress target when filters are absent/empty,
  while each explicit non-empty suffix or exact-case method list narrows it
  and two lists intersect. The gate continues model traffic when billing is
  out of scope and does not alter event truth, Redis delivery, internal-cost
  attribution, or response-correlation code.
- The 16 HIGH-impact direct-dependent tests pass together, as do the focused
  configuration/delivery/correlation/internal-attribution/cache/stream
  baselines and
  `TestBillingInternalCostAndSensitiveDataContract`. Task 3.2 remains RED at
  exactly its three model-unknown leaves and Task 3.3 remains RED at exactly
  its six response-header/query-lifecycle leaves; `go test ./... -count=1`
  contains only those nine later-task leaves. Extension `go vet ./...`,
  changed-file `gofmt`, `git diff --check`, and
  `openspec validate add-ai-route-plugin-routing-contracts --strict` pass.
  Graphify was refreshed to 11,121 nodes, 14,924 edges, and 874 communities.
  The subsequent GitNexus refresh generated eight repository-instruction
  diffs, all of which were restored. Branch-wide comparison against `main`
  retains the expected CRITICAL result from 63 files of already committed
  change history; the initial task-local detection was LOW across exactly two
  changed files with zero affected processes.
- Independent production review returned `NEEDS_FIXES` for three
  configuration-boundary defects, all repaired with separate RED/GREEN
  cycles. Blank-only bound filters first failed at four global/rule
  suffix/method parser leaves, then passed after non-empty normalized-empty
  lists became errors. Mixed-case method coverage first failed at two leaves
  because `MiXeD` collapsed with `mixed`/`MIXED`, then passed after configured
  token case and exact request matching were preserved. Compatibility
  coverage first failed at three leaves because 65 suffixes, a suffix longer
  than 256 bytes, and a valid method longer than 32 bytes were rejected, then
  passed after the undocumented 64/256/32 limits were removed.
- The repair-round routing regression suite and original Task 3.1 command
  pass, as do all 16 HIGH dependents and the complete focused
  config/delivery/correlation/internal/cache/stream/security baseline.
  Task 3.2 remains exactly three RED leaves, Task 3.3 remains exactly six RED
  leaves, and the complete module contains only those nine later-task
  failures. Repair-round formatting, vet, strict OpenSpec, diff, Graphify,
  GitNexus analyze, and staged change-detection gates were rerun before the
  scoped repair commit. Graphify rebuilt 11,125 nodes, 14,940 edges, and 874
  communities; GitNexus rebuilt 12,952 nodes, 40,595 edges, 292 clusters, and
  300 flows. The eight generated repository-instruction diffs were restored,
  and task-local detection remained LOW with zero affected processes.
- A follow-up defensive runtime audit found that manually constructed
  `EnablePathSuffixes: []string{""}` bypassed the parser and matched every
  path because an empty string is a suffix of every string. The regression was
  added before production changes and failed at exactly two leaves: a
  literal-empty raw suffix admitted a request, and an empty-plus-usable raw
  list admitted a path that did not match the usable suffix. `isAIPathEnabled`
  now ignores suffixes that are empty after trimming, so a raw list with no
  usable suffix fails closed and a mixed list matches only usable entries.
  Parsed bound-target absent/literal `[]` semantics remain unchanged because
  the suffix matcher is invoked only when the configured slice is non-empty.
  The original Task 3.1 and all repair regressions pass together; all 16
  original HIGH dependents plus the new 17th regression dependent pass,
  focused security/lifecycle baselines pass, and the exact 3+6 later-task RED
  boundary remains unchanged. Follow-up Graphify rebuilt 11,125 nodes, 14,940
  edges, and 879 communities; GitNexus rebuilt 12,953 nodes, 40,595 edges,
  292 clusters, and 300 flows. Eight generated repository-instruction diffs
  were restored. Comparison against `main` retains the expected branch-history
  CRITICAL result, while task-local detection is LOW across exactly three
  changed symbols/files with zero affected processes.

- [x] 3.5 Implement truthful provider-native/opaque-response event handling
  while preserving global inherited Redis delivery, exact correlation, and
  internal-cost authority.

Task 3.5 evidence (2026-07-29):

- The current Graphify report, Higress WASM lifecycle guidance, accepted and
  delta `ai-billing-events` specifications, change proposal/design/tasks, and
  both completed correlation/internal-cost dependency changes were reviewed
  before production inspection. GitNexus was refreshed to the exact
  `990e488be64879c3c2bcbdd6d9639f1c90b19ff0` pre-implementation HEAD; its
  eight generated repository-instruction rewrites were restored before work
  continued.
- The exact pre-edit command
  `go test . -run
  '^(TestBillingTruthfulEventContract|TestBillingCorrelationLifecycleContract)$'
  -count=1` reproduced exactly nine intended RED leaves. Three opaque
  non-streaming-success, non-streaming-error, and streaming-success leaves
  emitted only the fabricated `model.name=unknown`. Four response-origin
  leaves concatenated an upstream duplicate with the canonical correlation
  value, one canonical-absent leaf retained an upstream value, and one
  security leaf exposed the submitted query marker only through
  `request_path`. Provider usage, Redis XADD/fail-open, event identity,
  idempotent stream delivery, cache/local origin, internal-cost authority,
  response bytes/status, and credential/header/body/log secrecy controls all
  passed before implementation.
- Exact upstream impact was HIGH for `buildBillingEvent`: 12 direct and 17
  total dependents across the non-streaming and streaming completion process
  families. This blast radius was reported and explicitly authorized before
  editing. Impact was LOW for `onHttpResponseHeaders` with zero indexed
  dependents/processes and LOW for `initBillingRequestContext` with four
  direct dependents and zero processes.
- The minimal implementation stops defaulting an absent event model to the
  synthetic `unknown` value; estimated usage still stores its established
  unknown-model tokenizer fallback explicitly, and trustworthy provider usage
  still stores and emits its authoritative model, raw provider object,
  cache-aware details, and token counts. No price, cost, provider usage, or
  internal attribution behavior was added to missing-usage events.
- Response-header handling now filters every case-insensitive
  `x-ocmf-gateway-request-id` entry from the complete response header map and
  writes at most one canonical request-context value through whole-map
  replacement. Missing canonical context writes none, so non-streaming,
  streaming, cache, gateway-local, and canonical-absent paths cannot append,
  preserve, or invent an ambiguous value.
- Request context now stores the existing normalized path projection rather
  than the raw query-bearing target. This removes the query and its submitted
  secret from `request_path` without adding statistics-style cardinality
  rewriting or changing legacy/bound-target gating.
- The exact nine-leaf command is GREEN. A focused HIGH-impact gate covering
  every indexed direct event-builder dependent, both response-completion
  families, provider and estimated usage, cache facts, internal cost,
  correlation, Redis fail-open, and stream-done idempotency passed in
  `5.062s`. The complete independent ai-billing module passed with
  `go test ./... -count=1` in `9.473s`. `go vet ./...`, `gofmt -d main.go`,
  `git diff --check`, and strict OpenSpec validation also passed.
- The post-implementation Graphify update indexed 11,125 nodes, 14,942
  edges, and 856 communities. The final GitNexus refresh indexed 12,952
  symbols, 40,595 relationships, 292 clusters, and 300 execution flows; its
  eight generated repository-instruction rewrites were restored. Staged
  task-local change detection was LOW across exactly two files and five
  changed symbols with zero affected processes. The required comparison
  against `main` remained CRITICAL across 64 files, 476 changed symbols, and
  53 affected processes because it includes the change branch's cumulative
  pre-existing history rather than only Task 3.5.

- [x] 3.6 Run focused and complete independent ai-billing unit,
  proxy-wasm lifecycle/integration, streaming, Redis delivery,
  internal-attribution, correlation, and security suites.

Task 3.6 evidence (2026-07-29):

- Independent verification used final source result
  `82238f469f63a9e13b7391b810fd2ff843b2f0e2`. The exact nine-leaf
  RED-to-GREEN command
  `go test . -run
  '^(TestBillingTruthfulEventContract|TestBillingCorrelationLifecycleContract)$'
  -count=1` passed in `0.003s`.
- The focused HIGH-impact completion/stream-done gate passed in `4.876s`.
  It covers every indexed direct event-builder dependent together with
  non-streaming and streaming completion, trusted provider and estimated
  usage, cache and gateway-local facts, Redis dispatch and command-error
  fail-open behavior, at-most-once delivery, and all five internal-attribution
  scenarios: complete valid-pair override, absent-pair configured fallback,
  partial-pair suppression, malformed/blank/oversized/invalid suppression,
  and customer no-self-classification.
- The complete independent `go test ./... -count=1` module gate passed in
  `9.637s`. Unit and proxy-wasm lifecycle/integration coverage includes
  non-streaming, streaming and stream-done delivery; cache/local origins;
  event ID equal to idempotency key and exactly one Redis `XADD`; canonical
  correlation replacement and canonical-absent removal across
  non-streaming, streaming, cache, and local response paths; raw-path query
  removal; and legacy plus bound-target `path_scope` behavior.
- Security assertions cover Redis credential redaction, submitted
  header/body/query/internal-attribution value exclusion from serialized
  events and trace-through-critical logs, opaque missing-usage truthfulness,
  provider-usage authority, internal-attribution suppression without model
  traffic suppression, and customer inability to spoof internal event
  classification.
- Extension `go vet ./...`, exact changed-source `gofmt -l`, reviewed-range
  `git diff --check`, and strict OpenSpec validation pass. A fresh
  documentation-only rerun on the same final source reconfirmed the exact
  nine-leaf gate in `0.003s`, the focused HIGH-impact gate in `4.770s`, and
  the complete module in `9.647s`.
- Exact GitNexus upstream impact for the
  `3. AI Billing Red-Green Contracts` documentation section is LOW with zero
  direct dependents, affected processes, or affected modules. No production
  or test source changed, and the separate Task 3.7 independent
  implementation/security review remains open.
- Documentation-only change detection is LOW across one changed file, three
  touched documentation sections, zero affected symbols, and zero affected
  execution processes.

- [x] 3.7 Obtain an independent ai-billing implementation/security review,
  resolve every finding, and rerun the affected RED/GREEN and complete module
  suites before beginning the quota implementation.

Task 3.7 evidence (2026-07-29):

- Independent review was blocking throughout the complete billing sequence.
  Task 3.1 result `e254760ba220` received `NEEDS_FIXES` because explicit-empty
  suffix filters lacked executable global/rule runtime delivery coverage and
  the tracked stream name was inaccurate. Repair `50d6da6a52ff` added the
  coverage but rereview remained `NEEDS_FIXES` because a nested mock host
  would deadlock after parser GREEN. Final repair `16b4753807ed` removed the
  nested lifecycle; rereview was `APPROVED` with zero Critical, Important, or
  Minor findings and retained exactly the five intended RED leaves.
- Task 3.3 result `712795d0c54c` received `NEEDS_FIXES` because accepted,
  configured-fallback, and customer-spoofed attribution values were not
  excluded from every trace-through-critical plugin log level. Repair
  `2b8e0f2b736d` added distinct accepted/fallback/spoof markers and one shared
  six-level log collector. Rereview was `APPROVED` with no remaining finding,
  while the exact six later correlation/query RED leaves and all retained
  delivery, identity, idempotency, cache/local, Redis fail-open, and
  attribution controls remained intact.
- Task 3.4 result `99b98727c462` received `NEEDS_FIXES` for blank-only
  non-empty filters becoming allow-all, method-token uppercasing violating
  exact matching, and undocumented list/entry caps breaking compatibility.
  Repair `361bb57b7078` closed those parser, method-case, and compatibility
  defects, but rereview remained `NEEDS_FIXES` because raw runtime empty
  suffixes still failed open. Final repair `990e488be648` made unusable raw
  suffixes fail closed while preserving absent/literal-empty bound-target
  semantics; two independent final rereviews were `APPROVED` with no
  blocking or non-blocking finding.
- The final Task 3.5 independent production/security review covered immutable
  range
  `990e488be64879c3c2bcbdd6d9639f1c90b19ff0..82238f469f63a9e13b7391b810fd2ff843b2f0e2`.
  Report
  `.superpowers/sdd/add-ai-route-plugin-routing-contracts/task-3.5-reviewer.md`
  has SHA-256
  `4d58579d5eaf2ff9c6bef71643a56ca78cf8c593f4f4b4b11a7caad5e0c5fa1c`,
  returned `APPROVED`, and reported zero Critical, Important, or Minor
  findings. Its reviewed implementation result is exact source
  `82238f469f63a9e13b7391b810fd2ff843b2f0e2`.
- The reviewer retained the HIGH risk of `buildBillingEvent` and inspected it
  layer by layer: 12 direct and 17 total dependents across four modules and
  both completion families, `onHttpResponseBody` and
  `onHttpStreamingResponseBody` with idempotent `onHttpStreamDone` fallback.
  Every indexed direct dependent, non-streaming and streaming completion,
  provider/estimated/missing/cache/internal event construction, Redis
  fail-open delivery, exact correlation, and one-delivery stream fallback
  passed before the full module gate.
- No finding was waived or downgraded. Every `NEEDS_FIXES` verdict blocked the
  next implementation task until an exact repair range, affected
  RED/GREEN/baseline rerun, static/specification gates, and independent
  rereview closed it. The final Task 3.5 review passed the exact nine contract
  leaves in `0.003s`, the HIGH-impact dependent/completion gate in `4.876s`,
  and `go test ./... -count=1` in `9.637s`, together with extension vet,
  exact formatting, diff, and strict OpenSpec checks.
- Task 3.6 then supplied fresh post-review confirmation on the same final
  source: the exact nine leaves passed in `0.003s`, the HIGH-impact gate in
  `4.770s`, and the complete module in `9.647s`. Unit, proxy-wasm lifecycle,
  non-streaming/streaming/stream-done, cache/local, Redis fail-open and
  at-most-once delivery, all five internal-attribution scenarios,
  correlation replace/remove, event identity, path/query, path-scope, and
  trace-through-critical security-log coverage are GREEN.
- Exact GitNexus upstream impact for this documentation section is LOW with
  zero direct dependents, affected processes, or affected modules. This
  documentation-only detector is likewise LOW across one changed file, four
  shifted documentation-section mappings, zero affected symbols, and zero
  affected processes. This closeout changes no production or test source and
  does not modify any 4.x task. With all independent ai-billing findings
  resolved and the complete final suite reconfirmed, the quota implementation
  may begin at Task 4.1.

## 4. AI Quota Red-Green Contracts

- [x] 4.1 Add RED configuration and admission tests for legacy scope,
  bound-target provider-native requests, no synthesized suffix list, and
  explicit non-empty suffix/method intersections.

Task 4.1 evidence (2026-07-29):

- The current Graphify report, accepted ai-monetary-quota specification,
  complete quota delta, shared path-scope design, task plan, existing quota
  parser/admission tests, resource schema, and real HostEmulator Redis
  lifecycle were reviewed before the fixture was added. Upstream impact for
  each of the five new test/helper symbols and this evidence section was
  UNKNOWN with zero indexed dependents or execution flows because none was
  present in the two-commit-stale index. Existing production and shared
  harness symbols were not modified.
- Before the test edit, `go test ./... -count=1` passed the complete ai-quota
  module in `3.710s`, `go vet ./...`, `git diff --check`, and strict OpenSpec
  validation passed. The independent
  `extensions/ai-quota/routing_admission_contract_test.go` uses one fresh
  HostEmulator per request and literal JSON configuration, without referencing
  future path-scope or method-filter production fields.
- The fixture retains positive guards for omitted legacy default suffixes and
  explicit `legacy_suffixes`, including both eligible and ineligible paths. It
  adds global and rule-level `bound_target` requests with absent and literal
  empty filter lists, proves that a rule inherits the complete global Redis,
  identity-header, balance-template, and missing-balance configuration while
  overriding `quota_scope` and `provider`, and requires non-empty suffix and
  exact-case method filters to intersect. Blank-only global and rule filter
  arrays must fail closed.
- Every eligible runtime case must dispatch a real Redis `GET` containing the
  route-effective balance key. A positive balance must resume the request; a
  non-positive balance must produce the existing 403 local no-balance response
  while the HostEmulator retains its original
  `HeaderStopAllIterationAndWatermark` action rather than observing
  `ActionContinue`. Out-of-scope filter mismatches must continue without a
  Redis callout or local response. This tests the admission lifecycle rather
  than parser state alone.
- Focused RED command:
  `go test . -run
  '^(TestQuotaRoutingConfigurationContract|TestQuotaRoutingAdmissionContract)$'
  -count=1`. It compiles and fails at exactly nine leaves: explicit-empty
  bound-target configuration, blank-only global method rejection, rule
  infrastructure inheritance, global omitted-filter provider-native
  admission, global literal-empty filters, rule omitted filters, rule
  literal-empty filters, non-empty suffix/method intersection, and
  case-sensitive method matching. Existing blank-only suffix rejection,
  rule-level blank-filter guards, and omitted/explicit legacy eligibility
  remain GREEN.
- The initial complete `go test ./... -count=1` module run contained only
  those nine routing RED leaves. Dedicated legacy and blank-filter guard
  reruns passed. Changed-file `gofmt`, extension `go vet ./...`,
  `git diff --check`, and strict OpenSpec validation passed. `graphify update
  .` rebuilt 11,131 nodes, 14,958 edges, and 891 communities without tracked
  output. The GitNexus refresh generated eight repository-instruction diffs,
  all restored before staging; staged detection reported LOW risk across
  exactly two changed files and six test/documentation symbols, with zero
  affected processes.
- Independent review returned `NEEDS_FIXES` because the retained
  `TestRequestAdmission` required zero balance to resume even though both the
  accepted and change specifications require `<= 0` denial, and the new
  provider-native lifecycle covered only `-1`. Before repair, the stale
  zero-allow suite passed in `1.778s`, while the nine routing RED leaves were
  reproduced unchanged.
- The repair renames the retained case to `zero balance denies request` and
  requires the real Redis zero callback to produce the existing 403
  no-balance response without request resume in both Go and Wasm modes. The
  bound-target provider-native lifecycle now exercises positive, zero, and
  negative balances through the same Redis callback; both denied branches
  require 403 plus the HostEmulator's retained header-stop action. The
  positive and negative retained controls pass together in both modes.
- Combined pre-production command:
  `go test . -run
  '^(TestRequestAdmission|TestQuotaRoutingConfigurationContract|TestQuotaRoutingAdmissionContract)$'
  -count=1`. It contains ten semantic RED contracts: the zero-balance boundary
  plus the unchanged nine routing contracts. Because `TestRequestAdmission`
  executes in both Go and Wasm modes, the output has eleven executable failing
  leaves: two zero-balance leaves and the original nine Go-mode routing
  leaves. No prior routing RED moved or disappeared.
- The complete repaired `go test ./... -count=1` module run has that same
  ten-contract/eleven-executable-leaf boundary and no other failure. Fresh
  positive/negative retained admission controls pass in both Go and Wasm
  modes. Changed-file `gofmt`, extension `go vet ./...`, `git diff --check`,
  and strict OpenSpec validation pass. The repair Graphify refresh rebuilt
  11,131 nodes, 14,958 edges, and 879 communities without tracked output.
  Final staged GitNexus detection is LOW across exactly three changed
  test/task files and five mapped symbols, with zero affected processes.

- [x] 4.2 Add RED ownership tests proving the request-header Redis balance
  admission result is final for recognizable, opaque, streaming, and
  non-streaming responses; Redis balance input remains read-only and no
  response lifecycle callback adds account enforcement.

Task 4.2 evidence (2026-07-29):

- The accepted and delta ai-monetary-quota specifications, shared design,
  task plan, quota source/tests/schema, current Graphify report, and real
  HostEmulator Redis protocol were reviewed before the fixture was added.
  Current runtime ownership had already converged at `7c657e1`: `init`
  registers only request headers, eligible requests call
  `DontReadResponseBody`, and the sole live Redis operation is the admission
  `Get`. Task 4.2 therefore records the missing regression boundary without
  manufacturing a current-source RED. Task 4.4 remains responsible for
  schema/docs and obsolete settlement-control convergence.
- The independent
  `extensions/ai-quota/ownership_admission_contract_test.go` uses one fresh,
  non-parallel HostEmulator per leaf. Its seven response classes are
  recognizable JSON usage, explicitly eligible opaque provider-native
  non-streaming bytes, streaming SSE usage, upstream error, gateway local,
  cache replay, and success without usage. Each positive admission requires
  exactly one callout to `outbound|6379||redis.static` with the complete RESP
  body `GET billing:balance:tenant-a:global:consumer-a`.
- After the positive Redis callback resumes the request, duplicate response
  header/body or terminal streaming callbacks and stream completion must
  create no additional Redis callout of any kind. This stronger zero-callout
  oracle rejects price reads as well as `EVAL`, `DECRBY`, `SET`, or other
  balance mutation. Every lifecycle also requires no quota local response and
  exact preservation of the supplied response status, headers, and opaque or
  structured bytes.
- The same test was overlaid, without commits or shared-worktree changes, on
  detached pre-admission-only source `7c657e1^`
  (`ad1e61a2dd295e40c9aee852096e58e5b5042ab4`). It failed at exactly four
  semantic/eight Go-Wasm leaves: recognizable JSON usage, streaming SSE
  usage, upstream error with recognizable usage, and cache replay with
  recognizable usage. Each failure exposed one response-stage Redis `EVAL`;
  its exact callout body contains input/output price `GET`s, `DECRBY` of the
  admitted balance key, both effective-price keys, token arguments `11` and
  `7`, and the price-unit argument. Opaque native, gateway-local without
  usage, and no-usage controls stayed GREEN. The detached worktree was then
  removed.
- On current HEAD,
  `go test . -run '^TestQuotaAdmissionOwnershipContract$' -count=1` passes all
  seven cases in both Go and Wasm modes. The retained positive and negative
  admission controls also pass in both modes with an exact focused filter.
  The zero-balance denial remains only the already-recorded Task 4.1 RED;
  Task 4.2 does not duplicate or mask that pending `<= 0` implementation
  boundary. The complete module still fails at exactly the same eleven
  executable Task 4.1 leaves (two zero-boundary leaves plus nine routing
  leaves), while the util package remains GREEN.
- Pre-edit GitNexus impact for the new test and this evidence section was
  UNKNOWN with zero indexed dependents or execution flows. No production,
  accepted specification, existing test, shared harness, schema, or
  documentation symbol was modified.
- Changed-file `gofmt`, extension `go vet ./...`, `git diff --check`, and
  strict OpenSpec validation pass. `graphify update .` rebuilt 11,133 nodes,
  14,963 edges, and 873 communities without tracked generated changes.
  GitNexus incremental analysis rebuilt 12,966 nodes, 40,774 edges, 290
  clusters, and 300 flows; its eight generated instruction-file rewrites were
  restored before staging. Final staged detection is LOW across exactly two
  changed files and ten test/documentation symbols, with zero affected
  execution flows.

- [x] 4.3 Implement ai-quota path-scope parsing and bound-target admission
  intersections while preserving legacy request eligibility.

Task 4.3 evidence (2026-07-29):

- The current Graphify report, change design/tasks, accepted and delta
  ai-monetary-quota specifications, Task 4.1/4.2 fixtures, quota production
  source, and the reviewed ai-billing routing implementation were inspected
  before production edits. Exact pre-edit GitNexus impact was LOW for
  `QuotaConfig` (one direct test dependent and zero flows), `init`,
  `parseConfig`, and `onHttpRequestHeaders` (zero direct dependents/flows),
  `isAIPathEnabled` (two direct callers and zero flows), and
  `parsePathSuffixes` (one direct caller and the existing
  `ParseConfig -> RedisClusterClient` flow). The new rule parser and routing
  helpers had no pre-existing symbols; no HIGH/CRITICAL pre-edit impact was
  ignored.
- Before production edits, a separate
  `extensions/ai-quota/routing_regression_test.go` focused command failed at
  11 observed points: seven global scope/filter validation boundaries, the
  raw empty-suffix fail-open matcher, a valid method longer than 32 bytes that
  was not gated, rule infrastructure replacement, and bound-to-legacy default
  restoration. Existing non-array/empty legacy suffix rejection and query
  stripping remained GREEN controls. The unchanged Task 4.1 focused command
  independently reproduced exactly its original nine routing RED leaves.
- `QuotaConfig` now carries route-effective `path_scope` and
  `enable_methods`, and plugin registration uses a dedicated override parser.
  Global and rule parsing accepts only `legacy_suffixes` and `bound_target`,
  defaults omission to legacy, rejects blank/invalid/non-string scope,
  rejects non-array or non-string filters and invalid RFC HTTP tokens,
  preserves exact method case, and deduplicates exact entries without adding
  count or length caps. A non-empty source list that trims to no usable entry
  is rejected, while a literal empty bound-target list remains
  non-narrowing.
- Legacy missing/explicit suffix behavior remains suffix-gated and a method
  filter can only narrow it. Bound-target mode admits the already selected
  Higress target when both advanced filters are absent or literal empty;
  non-empty suffix and exact-case method lists intersect. Suffix matching
  excludes the query, ignores unusable raw entries, fails closed when none is
  usable, and uses only usable entries in a mixed raw slice.
- Rules clone inherited slices and retain the complete global Redis
  information/client, tenant and consumer headers, balance template,
  missing-balance policy, and every other base field. Only `quota_scope`,
  `provider`, `path_scope`, and explicitly submitted filters are parsed as
  rule overrides. A legacy-to-bound scope change clears inherited legacy
  filters, and switching back to legacy restores legacy defaults. Submitted
  rule Redis/header/template/policy fields cannot replace global
  infrastructure.
- The new regression command is GREEN. All nine original routing defects are
  crossed before the retained zero-balance boundary, the other seven
  Task 4.1 admission subtests pass independently, and all 14 Task 4.2
  ownership leaves pass. The implementation deliberately retains
  `balance < 0`, response ownership, and every obsolete settlement control
  for Task 4.4. Consequently the complete module has only three known
  executable zero-balance RED leaves: the retained `TestRequestAdmission`
  Go/Wasm pair plus the now-unmasked bound-target Go fixture. This corrects
  the earlier two-leaf estimate without editing or deleting the Task 4.1
  assertion.
- Existing parser, key-builder, admission-only/no-deduction, non-zero
  admission, Redis ownership, configuration, and routing regression gates
  pass. Extension `go vet ./...`, exact changed-file `gofmt`, `git diff
  --check`, and strict OpenSpec validation pass. `graphify update .` rebuilt
  11,150 nodes and 15,014 edges and regenerated the community map. GitNexus incremental
  analysis rebuilt 12,995 nodes, 40,919 edges, 295 clusters, and 300 flows;
  its eight generated repository-instruction diffs were restored. Change
  detection retains HIGH risk for the reviewed five plugin-registration
  option flows and the existing `ParseConfig -> RedisClusterClient` flow;
  those exact initialization/configuration boundaries are covered by the
  configuration, rule-inheritance, and real Redis admission tests.

- [x] 4.4 Converge ai-quota schema, examples, docs, and runtime on the
  admission-only Redis balance surface, removing obsolete settlement controls
  from current configuration and response-stage account behavior from the
  implementation.

Task 4.4 evidence (2026-07-29):

- The accepted and delta ai-monetary-quota specifications, shared design and
  task plan, quota source/tests/schema/examples, bilingual extension and
  resource documentation, and current Graphify/GitNexus reports were reviewed
  before implementation. The accepted ai-monetary-quota Purpose remains
  byte-for-byte untouched, and the plugin/resource versions remain `1.0.1` for
  Task 6.4.
- Pre-edit GitNexus upstream impact was LOW for `QuotaConfig` (one direct test
  dependent, zero flows), `buildPriceKey` (one direct test dependent, zero
  flows), and `int64Default` (one direct caller and the existing
  `ParseConfig -> RedisClusterClient` flow). `parseConfig`,
  `parseRuleConfig`, `onHttpRequestHeaders`,
  `onHttpStreamingResponseBody`, and `getOperationMode` had zero direct
  dependents or affected flows; `ChatMode` was unresolved by the index and a
  scoped source search confirmed its only remaining reference was the removed
  mode helper. No HIGH or CRITICAL pre-edit impact was ignored.
- The inherited RED boundary contained exactly three executable zero-balance
  failures: the Go and Wasm `TestRequestAdmission` leaves plus the bound-target
  routing admission leaf. Positive balance, negative balance, and all 14
  request/response ownership leaves remained independent controls. New
  test-first parser coverage then failed all ten global/rule cases for
  `price_key_template`, `amount_scale`, `price_unit_tokens`,
  `missing_price_policy`, and `missing_usage_policy`; the new structured
  resource contract failed its four intended schema, examples, bilingual docs,
  and obsolete-surface leaves before production/resource changes.
- Request admission now denies every Redis balance `<= 0` with fixed
  `403`/`ai-quota.no_balance` behavior and continues only for a strictly
  positive balance. The parser explicitly rejects each removed settlement
  field at both global and route-rule scope. Obsolete effective-price,
  input/output token-price, missing usage/price policy, response body mode, and
  response-stage account-state helpers were removed; the only live Redis
  account operation remains the request-stage read-only balance `GET`.
- The resource schema uses `additionalProperties: false` and exposes only the
  admission surface globally plus the supported business/scope route
  overrides. It documents both legacy and unfiltered bound-target examples,
  preserves rule inheritance/intersection semantics, and excludes global
  infrastructure from route overrides. The plugin example and four bilingual
  READMEs now state strict `balance > 0`, fixed zero/negative denial,
  request-header final account enforcement, read-only Redis ownership, and the
  Console/billing-service settlement boundary.
- The final exact core contract command passes in 3.795s, including removed
  field rejection, Go/Wasm balance admission, route admission, all 14
  ownership leaves, and structured schema/example/doc assertions. The complete
  module passes in 5.471s (`util` in 0.003s); extension `go vet ./...`, exact
  changed-file `gofmt`, `git diff --check`, scoped obsolete-current-surface
  searches, accepted-spec diff, and strict OpenSpec validation also pass.
- Controller diff review found no unresolved implementation, ownership,
  schema, documentation, or test finding. `graphify update .` rebuilt 11,166
  nodes, 15,049 edges, and 881 communities. GitNexus incremental analysis
  rebuilt 12,986 nodes, 40,897 edges, 291 clusters, and 300 flows; its eight
  generated repository-instruction rewrites were restored. Final staged
  detection is MEDIUM across exactly 11 files and 38 indexed symbols, affecting
  only `ParseConfig -> RedisClusterClient`; that configuration boundary is
  covered by the parser and real Redis admission suites. The mandatory
  compare-to-`main` result is CRITICAL across the cumulative 75-file change
  branch (547 symbols and 59 flows), rather than the isolated Task 4.4 delta.

- [x] 4.5 Run focused and complete independent ai-quota unit, Redis admission,
  proxy-wasm lifecycle/integration, no-deduction regression, and security
  suites.

Task 4.5 evidence (2026-07-29):

- Independent verification used reviewed source result
  `aec0cdfab4f9b796f97da29b06741bf6dd191b3c`; the separate Task 4.4
  implementation/ownership review was `APPROVED`. This task changes only this
  checkbox and evidence. It does not modify production, test, schema,
  example, or documentation files, and Task 4.6 remains open.
- The focused parser command
  `go test . -run
  '^(TestParseConfig|TestRemovedSettlementConfigIsRejected)$' -count=1`
  passed 12 leaves in `0.171s`. It covers the admission defaults plus
  global/rule rejection of all five removed settlement fields. The combined
  routing configuration, admission, validation, raw fail-closed, no-cap,
  rule-isolation, query, and scope-switch regression command passed 37 leaves
  in `0.174s`.
- Real Redis admission passed in both Go and Wasm modes with
  `go test . -run '^TestRequestAdmission$' -count=1`: 14 executable leaves,
  zero failures, `1.987s`. The lifecycle covers positive, zero, negative,
  malformed, and missing balances, missing identity, exact balance-key
  dispatch, fixed denial/error responses, request resume only when allowed,
  and legacy admin-path exclusion.
- The combined ownership/no-deduction command
  `go test . -run
  '^(TestQuotaAdmissionOwnershipContract|TestAdmissionOnlyDoesNotDeduct)$'
  -count=1` passed 18 leaves in `3.733s`. Seven response classes in both Go
  and Wasm modes provide the 14 ownership leaves; four additional leaves
  preserve no-deduction behavior. Recognizable, opaque, streaming, error,
  local, cache, and missing-usage responses remain byte/status/header
  transparent and dispatch no response-stage Redis callout or account
  mutation.
- The structured resource/schema/example/bilingual-documentation contract
  passed four leaves in `0.176s`, and `go test ./util -count=1` passed six
  leaves in `0.128s`. Scoped scans passed with no credential-like literals or
  credential-bearing log calls, no removed settlement fields/wording in the
  current schema/examples/docs, and no response callback registration or
  Redis mutation command in the ai-quota runtime.
- The complete isolated module gate `go test ./... -count=1` passed both
  packages and all 92 leaves in `5.410s`. The exact race gate
  `go test -race ./... -count=1` also passed without a Wasm/harness
  exception: ai-quota `54.148s`, util `1.005s`, wall `56.989s`.
- Verification used Go `1.26.3` on `linux/amd64`. `go vet ./...` passed in
  `0.036s`; exact `gofmt -l` inspection passed all eight Go files;
  `git diff --check` passed; and
  `openspec validate add-ai-route-plugin-routing-contracts --strict` passed
  in `1.098s`.
- `graphify update .` refreshed 11,166 nodes, 15,049 edges, and 880
  communities without tracked generated output. GitNexus incremental analysis
  refreshed 12,986 nodes, 40,897 edges, 291 clusters, and 300 flows; its
  eight generated repository-instruction rewrites were restored. Final staged
  detection is LOW across exactly one changed documentation file and two
  indexed documentation sections (the quota section plus the line-shifted
  following statistics section), with zero affected execution processes.

- [x] 4.6 Obtain an independent ai-quota implementation/ownership review,
  resolve every finding, and rerun the affected RED/GREEN and complete module
  suites before beginning the statistics implementation.

Task 4.6 evidence (2026-07-29):

- Independent review of exact immutable range
  `85d943350b98ab913b6f0b4056e75b14a1b426f5..39299baefa59910c877a4b8344184adcae644c64`
  is `APPROVED` with no unresolved finding. The byte-identical review package
  `review-ai-quota-85d9433..39299ba.diff` has SHA-256
  `bba11d994777e44de99aac1f88e6cc3b3dfb040c56cc51c6ba25db2212917d5e`.
  The ignored reviewer report
  `.superpowers/sdd/add-ai-route-plugin-routing-contracts/task-4.6-reviewer.md`
  has SHA-256
  `d83bdb204bd8062a44e9fb4a16a8da806491ba49e6cadcf80cdfaf217d36a0d1`.
- Runtime and contract review confirmed legacy and unfiltered bound-target
  routing, non-empty suffix/method intersection, exact-case method matching,
  query exclusion and invalid/raw fail-closed behavior. Eligible requests
  perform exactly one real Redis `GET`; strict `balance > 0`, zero/negative,
  missing, malformed, Redis-error, and missing-identity outcomes match the
  contract. No response callback is registered and all 14 ownership leaves
  preserve response bytes/status/headers with zero response-stage Redis
  callout, mutation, or settlement state.
- Global and rule parsing reject all five removed settlement fields. The
  plugin schema, resource schema, examples, and four bilingual READMEs are
  aligned on read-only admission and Console/billing-service settlement
  ownership; route overrides cannot replace Redis infrastructure. Credential
  examples remain placeholders, source/config errors do not expose submitted
  credentials, versions remain `1.0.1`, and the accepted specs/Purpose are
  untouched for their later assigned tasks.
- Historical Task 4.1 RED was independently reproduced at detached commit
  `033be91`: the focused routing contract exited non-zero with nine visible
  failing semantic leaves; the current same contract is GREEN. Task 4.2's
  historical ownership RED is traceable to pre-admission-only source
  `ad1e61a2`, where response settlement/EVAL behavior failed the documented
  four semantic/eight Go-Wasm leaves. Task 4.4's APPROVED report SHA and Task
  4.5 evidence were checked and are consistent with the reviewed range.
- The focused parser, removed-field, real Redis admission, routing,
  regression, ownership, no-deduction, and resource contract command passed.
  `go test ./... -count=1` passed (`ai-quota` `5.471s`, `util` `0.002s`);
  `go test -race ./... -count=1` passed (`54.427s` and `1.005s`);
  `go vet ./...`, all-module `gofmt -l`, exact-range `git diff --check`, and
  strict OpenSpec validation passed. Direct YAML parsing passed both YAML
  files and all eight README examples (`10` documents total).
- GitNexus exact-range detection is HIGH across 13 files, 81 symbols, and six
  processes: five `init` registration fanout flows plus
  `ParseConfig -> RedisClusterClient`, all directly covered by the reviewed
  suites. Compare-to-`main` is CRITICAL across the cumulative 76-file branch
  (555 symbols and 59 flows), not an isolated ai-quota finding. Task 5.1 may
  begin after this evidence-only closeout commit.

## 5. AI Statistics Red-Green Contracts

- [x] 5.1 Add RED configuration/gating tests for legacy scope, bound-target
  provider-native requests, no synthesized suffixes, and advanced non-empty
  suffix/method intersections.

Task 5.1 evidence (2026-07-29):

- The current Graphify report, change proposal/design/tasks, all seven delta
  capabilities, accepted resource specification, ai-statistics source/tests,
  wrapper route-matching behavior, and HostEmulator lifecycle were reviewed
  before the fixture was added. No accepted
  `openspec/specs/ai-statistics-observability/spec.md` exists yet, so the
  validated statistics delta and shared routing design are the current
  authority. The pre-edit Graphify report contained 11,166 nodes, 15,049
  edges, and 880 communities and was two commits behind the clean starting
  HEAD `f4e852d656f7d23c3dd25a810aef194afdaac19c`.
- Pre-edit controls passed:
  `go test . -run
  '^(TestParseConfig|TestUnmatchedPathsAndContentTypes|TestIsPathEnabled)$'
  -count=1` in `2.153s`, complete `go test ./... -count=1` in `33.616s`,
  and `go vet ./...`. Upstream impact for the new
  `TestStatisticsRoutingScopeContract` symbol was UNKNOWN with zero indexed
  dependents; this documentation section was LOW with zero callers, affected
  processes, or modules. No production symbol, existing test, shared harness,
  schema, documentation, or version surface was modified.
- `extensions/ai-statistics/routing_scope_contract_test.go` adds one
  compile-safe Go HostEmulator contract and no top-level helper. Every request
  receives a fresh host and literal JSON configuration. Eligibility is
  observed through the real request-body lifecycle and
  `wrapper.AILogKey`: an admitted request writes the bounded test attribute,
  while `SkipProcessing` plus `DontReadRequestBody` leaves the property
  absent. The fixture does not reference future fields, getters, constants,
  or Task 5.2 base metrics.
- Global and rule cases retain omitted and explicit `legacy_suffixes`
  behavior; exercise `bound_target` with filters omitted and with both lists
  literal empty; require provider-native coverage without a synthesized
  default suffix; and require non-empty suffix and exact-case method filters
  to intersect. Rule cases prove omitted-scope inheritance, explicit legacy
  suffix replacement, legacy-to-bound-target projection, literal-empty
  filters, non-empty intersections, and preservation of inherited global
  attributes. Unknown global/rule `path_scope` and blank-only global/rule
  suffix/method lists must fail configuration.
- Exact RED command:
  `go test . -run '^TestStatisticsRoutingScopeContract$' -count=1`.
  It compiles and fails at exactly 15 semantic/executable Go leaves: one
  global unfiltered bound-target provider-native observation; two global
  exact-case method mismatches; four rule inheritance/legacy/bound-target
  leaves; two rule non-empty intersection leaves; two invalid path-scope
  leaves; and four blank-only suffix/method validation leaves. The complete
  `go test ./... -count=1` module boundary fails at those same 15 leaves in
  `33.599s` and no pre-existing test.
- Independent controls remain GREEN: the original three-test parser/gating
  command passes in `2.042s`; omitted and explicit legacy behavior plus
  literal-empty bound-target filters pass together in `0.003s`; global
  exact-match/suffix-mismatch/both-mismatch intersections pass in `0.003s`;
  and the rule suffix-mismatch control passes in `0.003s`. Changed-file
  `gofmt`, extension `go vet ./...`, `git diff --check`, and strict OpenSpec
  validation pass. The final `graphify update .` rebuilt 11,168 nodes, 15,053
  edges, and 866 communities. GitNexus refreshed 12,992 nodes, 40,944 edges, 290
  clusters, and 300 flows; the post-index test-symbol impact is LOW with zero
  callers/processes/modules. Final staged detection is LOW across exactly two
  files and seven mapped test/documentation symbols with zero affected
  processes. All eight generated repository-instruction rewrites were
  restored.

Task 5.1 review-fix evidence (2026-07-29):

- Independent review of exact result `77590c22a4373534b84a08761e5ce2fa96e1ba13`
  returned `NEEDS_FIXES`. Its ignored report
  `.superpowers/sdd/add-ai-route-plugin-routing-contracts/task-5.1-reviewer.md`
  has SHA-256
  `cc053177f4e9cd4cb4309a02bec70bc3485971bb507f9ce1c083b9d3361b50e1`.
  The review found that the two rule legacy tests had only positive
  observations, permitting a future omitted-scope rule to widen into
  unfiltered bound-target behavior or an explicit rule suffix to union with
  the inherited global suffix. It also found no control for explicit
  `legacy_suffixes` with an omitted suffix list. Task 5.2 remained blocked.
- Exact pre-edit upstream impact for
  `TestStatisticsRoutingScopeContract` and this Task 5 section was LOW with
  zero callers, processes, or modules. The omitted-rule-scope subtest now
  rejects a provider-native path on the same matched route after its positive
  inherited suffix/marker assertion. The explicit-legacy-rule subtest now
  rejects the inherited global suffix on the same matched route after its
  positive route-suffix/marker assertion. Neither existing assertion was
  weakened or reordered.
- A new compile-safe GREEN control selects explicit `legacy_suffixes` with
  `enable_path_suffixes` omitted. It observes `/v1/chat/completions` and skips
  `/vendor/jobs/job-42`, proving the existing default suffix projection is
  retained rather than widened. Its isolated command passes in `0.002s`.
- The exact focused RED command still compiles and fails at the same 15 leaves
  with no new failure. Both strengthened rule subtests still stop at their
  original missing-inheritance positive assertion on current production; when
  GREEN implementation satisfies that assertion, the new same-route negative
  observations become mandatory. Complete `go test ./... -count=1` fails only
  at those 15 leaves in `33.060s`.
- The existing parser/gating controls pass in `1.993s`; omitted, explicit
  default, explicit configured legacy, and literal-empty bound-target controls
  pass together in `0.003s`; global intersection controls pass in `0.003s`;
  and the rule suffix-mismatch control passes in `0.002s`. Extension vet,
  changed-file `gofmt`, `git diff --check`, and strict OpenSpec validation
  pass. Graphify and GitNexus staged change detection were refreshed before
  the review-fix commit, and all generated repository-instruction rewrites
  were restored.

- [x] 5.2 Add RED lifecycle/metric tests proving opaque success and error
  responses emit bounded route/path attribution, method, status, latency,
  bounded error, request count, and duration independently of model/token
  parsing.

Task 5.2 evidence (2026-07-29):

- The current Graphify report, change proposal/design/tasks, all seven delta
  capabilities, accepted resource specification, ai-statistics lifecycle and
  metric tests, and the real HostEmulator AILog/filter-state and counter
  surfaces were reviewed before the fixture was added. GitNexus located the
  request, non-streaming response, streaming end-of-stream, AILog, and
  `writeMetric` usage-gate boundaries. Pre-edit impact for the new
  `TestStatisticsOpaqueBaseFactsContract` symbol was UNKNOWN with zero indexed
  dependents or processes; the AI Statistics task section was LOW with zero
  dependents, processes, or modules. No production, existing test, shared
  harness, schema, documentation, or version surface was modified.
- Pre-edit baselines reproduced the exact existing Task 5.1 boundary:
  `go test . -run '^TestStatisticsRoutingScopeContract$' -count=1` failed at
  its documented 15 leaves, and `go test ./... -count=1` failed only at those
  same 15 leaves in `32.911s`.
- `extensions/ai-statistics/opaque_base_facts_contract_test.go` defines its
  reader-facing AILog and metric contract locally rather than importing future
  production constants or getters. Every leaf owns a fresh Go HostEmulator,
  selects `bound_target` with literal-empty advanced filters, uses the trusted
  route `statistics-base-facts-route`, and leaves adapter metadata absent so
  path attribution must be the fixed `provider_native_unknown` sentinel.
- The three independent lifecycle leaves cover an opaque `202` success with
  `disable_openai_usage: true`, an opaque `201` success with usage/model facts
  missing, and an opaque streaming `503` error completed at end of stream.
  Each leaf asserts six base fields (`route`, `path`, `method`, `status_code`,
  positive numeric `latency_ms`, and bounded `error_class`), six unavailable
  optional dimensions remain absent, exact response headers/body remain
  transparent, and the exact reader-facing `request_count=1` and positive
  `request_duration_ms` counter metrics exist. Counter names contain only the
  bounded route/path/method/status/error projection.
- Each leaf also rejects response-body/provider-error text, request
  authorization, provider trace metadata, and the raw provider-native path
  from AILog; the body and authorization markers are rejected across every
  HostEmulator log level. The two expected metric names independently reject
  the raw path and submitted markers. This supplies the fixed opaque fallback
  and error-secrecy boundary without adding Task 5.3's route/template
  cardinality matrix.
- Focused RED command:
  `go test . -run '^TestStatisticsOpaqueBaseFactsContract$' -count=1`. It
  compiles and fails at exactly three executable leaves. Current AILog lacks
  all six safe base fields and emits the fabricated optional `api:"-"`; both
  exact counters are absent because `writeMetric` returns for disabled or
  missing usage. Response transparency, positive observable legacy duration,
  optional model/token absence, and every submitted-value secrecy control pass
  before those intended failures.
- The retained metric, non-streaming, streaming, and disabled-usage controls
  pass together in `7.710s`. Task 5.1 remains exactly 15 RED leaves. The
  complete module fails only at the union of those 15 leaves and the three new
  Task 5.2 leaves (`18` total) in a final-source rerun of `33.228s`. Exact
  changed-file formatting, extension vet, diff checks, and strict OpenSpec
  validation pass. `graphify update .` rebuilt 11,170 nodes, 15,059 edges, and
  854 communities without tracked generated output. GitNexus incremental
  analysis rebuilt 13,008 nodes, 40,989 edges, 292 clusters, and 300 flows; its
  eight generated repository-instruction rewrites were restored. Final staged
  detection is LOW across exactly two files and 14 test/documentation symbols,
  with zero affected execution processes.

Task 5.2 review-fix evidence (2026-07-29):

- Independent review of commit
  `285615914cfbfaa77cdc43502f792976795fbecb` produced one HIGH finding and no
  other findings; the immutable review report SHA-256 is
  `ea51ce0e45ede45e7e56b4de52772a616baa10e46c58c3361bb65d124851e520`.
  The finding was technically reproduced: the six-level HostEmulator log
  snapshot preceded `CompleteHttp`, so it omitted terminal callback logs, and
  it rejected only response-body and authorization markers rather than the
  complete secrecy set.
- Pre-edit upstream impact for
  `TestStatisticsOpaqueBaseFactsContract` was LOW with zero direct callers,
  affected processes, or affected modules. The fixture now invokes
  `CompleteHttp` before aggregating all trace, debug, info, warn, error, and
  critical logs. Both decoded AILog and the post-terminal six-level aggregate
  reject the response-body/provider-error marker, request authorization,
  `opaque-provider-trace`, and the raw provider-native path. Existing response
  transparency, exact metric-name, optional-dimension, and bounded base-fact
  assertions remain intact.
- The focused Task 5.2 command still fails at exactly its three intended RED
  leaves, and Task 5.1 still fails at exactly its documented 15 leaves. A
  final-source full-module run failed in `33.345s`; its JSON event stream
  proves the executable leaves are exactly the 18-leaf union, with no
  additional failure. Retained metric, non-streaming, streaming, and
  disabled-usage controls pass together in `8.265s`.
- Changed-file `gofmt`, extension `go vet ./...`, `git diff --check`, and
  strict OpenSpec validation pass. No production code, Task 5.1 fixture, Task
  5.3 fixture, schema, documentation, or version surface was changed.
- `graphify update .` rebuilt 11,170 nodes, 15,059 edges, and 851 communities
  without tracked generated output. GitNexus incremental analysis rebuilt
  13,008 nodes, 40,989 edges, 292 clusters, and 300 flows; its eight generated
  repository-instruction rewrites were restored. Final staged detection is LOW
  across exactly two files and two test/documentation symbols, with zero
  affected execution processes.

- [x] 5.3 Add RED trust/cardinality fixtures proving: raw
  `/v1/files/<uuid>?api_key=secret` plus trusted
  `/v1/files/{file_id}` emits only the template; unknown
  `/vendor/jobs/<attacker>` emits `provider_native_unknown`; invalid or
  oversized trusted templates emit `path_unknown`; valid trusted
  `route_name` is retained while missing/unsafe/oversized values emit
  `route_unknown`; repeated fallbacks are deterministic; raw queries,
  credentials, headers, bodies, dynamic segments, and raw-path hashes never
  enter metric attributes.

Task 5.3 evidence (2026-07-29):

- `extensions/ai-statistics/attribution_cardinality_contract_test.go` adds one
  independent HostEmulator matrix with literal bound-target configuration,
  literal-empty advanced filters, empty attributes/defaults, opaque response
  controls, test-local bounded adapter metadata, and the trusted
  `mse_ai_proxy_adapter_metadata` property. It enumerates actual counter keys
  read-only through same-package `GetMatchConfig`, verifies response
  transparency, and aggregates every terminal HostEmulator log level.
- The bounded focused command
  `timeout 45s go test . -run '^TestStatisticsAttributionCardinalityContract$' -count=1`
  compiles and fails at exactly 19 intentional leaves: trusted template, no
  metadata, matched-false metadata, nine invalid templates, five route cases,
  determinism, and spoofing. Each intended failure is missing bounded base
  attribution/metrics; no setup, panic, or empty-property error is present.
  The selected full boundary is exactly 37 leaves: existing Task 5.1 15 + Task
  5.2 3 + these 19 only.
- The isolated missing-route command
  `timeout 45s go test . -run '^TestStatisticsAttributionCardinalityContract$/^go$/^missing_trusted_route_name_uses_fixed_fallback$' -count=1 -v`
  bootstraps and resets its own ordinary host before the raw no-default host.
  It fails only the intended attribution/metric assertions, with no context,
  setup, or panic failure.
- The SDK HostEmulator rejects zero-length `SetProperty` values and dereferences
  element zero while reading an injected empty slice, so it cannot honestly
  represent an empty `route_name`. This fixture uses a raw no-default-property
  HostEmulator for the genuine missing route case and an exact 128-byte valid
  route control instead of fabricating empty data. Before Task 5.4 production
  work, a pure route-validator RED test must cover the exact empty-value case.
- [x] 5.4 Implement bound-target gating and base-fact emission before optional
  trusted adapter/provider enrichment, with 128-byte bounded trusted Higress
  route names and selected-adapter origin-form operation templates.

Task 5.4 evidence (2026-07-29):

- Before production implementation, the mandatory pure-validator command
  `go test . -run '^TestNormalizeTrustedRouteNameExactEmpty$' -count=1`
  compiled all existing code and failed only at
  `route_validator_contract_test.go:10:36: undefined:
  normalizeTrustedRouteName`. After implementation, the same exact command
  passed in `0.002s`. Pre-edit GitNexus upstream impact was LOW for the
  existing lifecycle/config symbols and `writeMetric` (two direct callback
  callers, zero indexed execution flows); planned new helpers/tests were
  UNKNOWN/not-found with zero dependents. No HIGH or CRITICAL pre-edit impact
  was reported.
- Global and rule configuration now use route-effective
  `legacy_suffixes`/`bound_target` scope, clone inherited filter slices, retain
  omitted and explicit legacy defaults, accept literal-empty bound-target
  filters, reject blank-only/invalid values, and intersect non-empty suffix
  and exact-case method filters. Fixed skip logs no longer print raw request
  paths or response content types.
- Request admission records only a UTF-8, no-control, at-most-128-byte trusted
  route (otherwise `route_unknown`) and a fixed-cardinality method. Response
  headers refresh bounded adapter metadata, record bounded status/error and a
  positive latency, then emit `request_count` and `request_duration_ms` before
  any optional content-type/body/usage parser. Missing, malformed,
  schema-mismatched, or `matched:false` metadata uses
  `provider_native_unknown`; schema-v1 `matched:true` metadata with an invalid
  or oversized origin-form operation uses `path_unknown`. Raw request paths,
  query strings, credentials, headers, bodies, dynamic segments, and hashes
  are never included in the new metric key.
- Five completion-seam regressions prove response-header EOS, unsupported
  parser content type, empty body, malformed status, and missing status all
  emit exactly the two base counters once, preserve headers/body, and retain
  bounded AILog facts. The Task 5.1/5.2/5.3 contract union
  (`15 + 3 + 19 = 37` leaves), exact-empty validator, completion seams, and
  final cardinality regression pass together in `0.023s`. Complete
  `go test ./... -count=1 -timeout=3m` passes in `33.821s`; final-source
  `go test -race ./... -count=1 -timeout=10m` passes in `345.568s` with no
  race finding.
- Independent implementation review initially returned one HIGH finding:
  legacy token/duration metrics still consumed the raw `RouteName` when valid
  usage existed. A new 129-byte-route plus real-usage regression reproduced
  one exact RED (the `route.route_unknown...input_token` counter was absent).
  `RouteName` now derives from the normalized statistics route, while a valid
  `api@...` name may still fold to its compatible first-three-part value only
  after the full route passes validation. The exact regression passes in
  `0.002s`, legacy metric/API controls pass in `8.176s`, every generated
  counter key rejects the unsafe route, and the same reviewer returned
  APPROVED with no remaining finding.
- Final extension `go vet ./...`, changed-file `gofmt -l` (empty output),
  `git diff --check`, and
  `openspec validate add-ai-route-plugin-routing-contracts --strict` all
  pass. The known ai-proxy metadata value without a leading slash remains a
  documented Task 5.6 cross-module follow-up; Task 5.4 does not modify proxy,
  Task 5.5, version, schema, or resource surfaces.

- [x] 5.5 Run focused and complete independent ai-statistics unit,
  proxy-wasm lifecycle/integration, metric regression, cardinality, and
  security suites.

Task 5.5 evidence (2026-07-29):

- Independent behavior verification ran on exact source
  `07ada49e45a4f22bc0fb0cbcde7c092ef1465273`. The module-hygiene result
  `24f9a1ed3966a5cbe6a08a0ac9cd47ffbb7cb1a2` changes only
  `extensions/ai-proxy/go.sum` and `extensions/ai-statistics/go.sum`, deleting
  four stale checksum lines from each. It has zero additions, no `go.mod`,
  Go-source, test, or OpenSpec change, and the combined ai-proxy/
  ai-statistics `.go` plus `go.mod` blob-tree SHA-256 is identical at both
  commits:
  `3e3c45c9f33199d442e0f7721412a573f59f5aade2e39abcfb22f581962ed0c2`.
  The behavior evidence therefore exercises byte-identical Go source, while
  fresh post-hygiene focused and complete ordinary checks confirm the cleaned
  module state.
- The individual Task 5.1 routing, Task 5.2 opaque facts, Task 5.3
  attribution/cardinality, exact-empty route, completion/status, and unsafe
  legacy-usage route regressions all pass. Their combined seven-contract gate,
  including schema-v2 terminal lifecycle, passes in `2.281s`.
  Configuration/helper controls pass in `4.005s`; metric/terminal/legacy
  controls pass in `7.912s`; and the secrecy/cardinality/response-transparency
  matrix passes in `9.940s`.
- Fresh temporary ai-proxy and ai-statistics guests are non-empty Wasm modules
  with magic `00 61 73 6d`. Their final sizes and SHA-256 values are
  ai-proxy 9,903,117 bytes /
  `dbe09dd5a07230eea8cb73f195fd338022f6ed6400dc741fede11d9eac87df4a`
  and ai-statistics 6,923,564 bytes /
  `2c9eaca0d95b33644ebcbf52ec68b3ed4b6ae28fd268190d805fa025026bf7f8`.
  Those exact values are unchanged after the checksum-only commit.
- The ai-proxy schema-v2 producer contract passes in real `/go` and `/wasm`
  modes, compares the matched envelope to the exact 306-byte shared golden,
  and observes zero skip or missing/auto-compile marker. The direct
  ai-statistics terminal consumer contract likewise passes in real `/go` and
  `/wasm` modes with zero skip. A fresh post-hygiene smoke rerun reconfirms
  both producer and consumer pairs with zero skip/failure.
- The complete explicit ai-statistics dual-runtime matrix passes 18 top-level
  entrances, 18 `/go` branches, and 18 `/wasm` branches in `35.303s`, with
  zero skip, failure, missing-Wasm, or auto-compilation marker. It covers
  existing request/response, streaming, metric, session, tool-call, attribute,
  legacy, and edge-case lifecycles plus the new terminal contract.
- Fixed shuffle seeds `17`, `314159`, and `8675309` pass in `34.662s`,
  `35.070s`, and `34.730s`. The complete independent ai-statistics JSON suite
  passes in `34.779s`: 34 top-level tests, 24 `/go` branches, 18 `/wasm`
  branches, one package result, and zero skip/failure. The post-hygiene
  implementer rerun independently records a complete ordinary pass in
  `36.892s`.
- The complete ai-statistics race gate
  `CGO_ENABLED=1 go test -race ./... -count=1 -timeout=10m` passes in
  `362.955s` (`363.204s` wall) with no race report. This instruments the Go
  process and Go-mode path; it does not race-instrument guest Wasm bytecode.
- The supporting ai-proxy ordinary serial suite passes on the behavior source
  with root package `281.838s` plus config/provider/util passes; the
  post-hygiene rerun passes with root package `285.296s` and every subpackage
  green. The all-Go-mode proxy race gate passes 1,534 non-Wasm run/pass events
  across four tested packages with zero failure, unexpected test skip, or race
  marker. The changed producer race contract separately passes real `/go` and
  `/wasm`, and the known-adapter Go control passes, in `30.643s`.
- A complete ai-proxy Go+Wasm race result is deliberately **not** claimed.
  Prior 15-minute and 30-minute attempts emitted no race or assertion finding
  but timed out while Wazero JIT-compiled unrelated existing
  `TestMinimax/wasm` and `TestGroq/wasm` cases. The true split evidence is the
  complete all-Go-mode race plus the changed real Go/Wasm producer race
  contract; the JIT limitation remains explicit.
- On final source `24f9a1e`, both extension-local `go mod tidy -diff`
  commands exit zero with empty output, both `go mod verify` commands report
  all modules verified, and both `go vet ./...` commands pass. `gofmt -l`
  across 136 Go files is empty; worktree and exact checksum-only range
  `git diff --check` pass; and
  `openspec validate add-ai-route-plugin-routing-contracts --strict` passes.
  Every temporary Wasm and JSON file and its exact temporary directory was
  removed after verification.
- The fresh Task 5.6 rereview of the behavior source is `APPROVED` with no
  remaining correctness, security, compatibility, response-transparency, or
  test-execution finding. Task 5.5 records the independent suite result only;
  the separate Task 5.6 checkbox remains unchanged for its own closeout.
- Exact upstream impact for the AI Statistics task section is LOW with zero
  direct dependents, affected processes, or affected modules. After GitNexus
  refreshed to 13,110 nodes, 41,521 edges, 295 clusters, and 300 flows, staged
  Task 5.5 detection remained LOW: exactly one changed documentation file and
  one touched task section, with zero affected execution process. The eight
  generated repository-instruction rewrites were restored before staging.

- [ ] 5.6 Obtain an independent ai-statistics implementation/cardinality
  review, resolve every finding, and rerun the affected RED/GREEN and complete
  module suites before version/resource work.

Task 5.6 open finding-resolution scope (2026-07-29):

- The initial independent review of result `b8f4cb60dbc9` is
  `NEEDS_FIXES`. It found that schema-v1 `normalized_operation` was produced as
  the logical ID `openai/v1/responses` but consumed as an origin-form path
  template, that `request_duration_ms` was frozen at response headers rather
  than terminal completion, that non-HTTP status forms and `600..999` were
  accepted, and that the new routing/base/cardinality branches lacked direct
  Wasm-mode contract coverage.
- Resolution SHALL first align the active design and deltas on schema v2:
  matched metadata separates bounded logical `normalized_operation` from
  bounded origin-form `operation_template`; unmatched/passthrough metadata
  claims neither. ai-statistics uses only matched schema-v2
  `operation_template` for path attribution, keeps logical operation
  enrichment separate, treats v1/unknown/malformed/unmatched metadata as
  having no trusted template, and never emits submitted unsafe values.
- Base facts SHALL finalize exactly once at non-streaming body EOS, streaming
  EOS, or `wrapper.ProcessStreamDone` fallback. Response headers may capture
  status/error state but do not freeze duration. Status accepts only exact
  three-ASCII-digit `100..599`; all other forms use
  `0`/`status_unknown`.
- RED/GREEN evidence SHALL include exact producer bytes, v1 and future-schema
  compatibility, invalid/oversized v2 templates, matched-false behavior,
  bounded logical operation versus path attribution, observable terminal
  delay, header-only/parser-skipped fallback, exact-once behavior, and at
  least one direct `test.RunTest` Go/Wasm lifecycle matrix for the new branch.
- This checkbox remains open until all findings are fixed, both modules pass
  focused/full/race/vet plus explicit temporary-Wasm verification, strict
  OpenSpec/Graphify/GitNexus gates pass, and a fresh independent review returns
  `APPROVED`.

Task 5.6 finding-resolution implementation evidence (2026-07-29):

- Exact RED proved that ai-proxy emitted schema v1 without
  `operation_template` in both Go and Wasm modes. Statistics RED proved that
  schema-v2 metadata fell back to `provider_native_unknown`, schema-v1 logical
  operation was treated as a path, duration was frozen at response headers,
  and invalid HTTP status forms were accepted.
- ai-proxy now publishes a bounded schema-v2 envelope with distinct reviewed
  logical `normalized_operation` and origin-form `operation_template` values.
  Matched Responses metadata is byte-for-byte equal to the shared golden;
  unmatched and forced-passthrough envelopes omit adapter and operation
  claims. ai-statistics caps the envelope at 1024 bytes, accepts only supported
  schema/match states, keeps schema-v1 compatibility logical-only, and rejects
  unknown, malformed, missing-field, unsafe, or oversized values to fixed
  fallbacks without optional enrichment.
- Response headers now capture only bounded status/error state. Body EOS,
  streaming EOS, or `ProcessStreamDone` owns one terminal timestamp and one
  request-count/duration emission. The terminal marker prevents later
  callbacks from duplicating facts. Status accepts only exact three-ASCII-digit
  values in `100..599`.
- Focused proxy metadata and statistics attribution/completion/lifecycle
  contracts pass. Explicit temporary Wasm builds ran the new producer and
  consumer contracts in both `go` and `wasm` modes without skip, then their
  exact temporary files were removed. Complete ordinary suites pass for both
  modules; both `go vet ./...` runs pass; complete ai-statistics race passes;
  all 349 ai-proxy Go-mode race tests/subtests plus the changed Go/Wasm
  metadata race contract pass.
- Complete ai-proxy race was additionally attempted with 15-minute and
  30-minute limits. Both produced no race or assertion finding but timed out
  while Wazero compiled unrelated existing provider Wasm cases
  (`TestMinimax/wasm`, then `TestGroq/wasm`). This harness limitation is
  recorded rather than reported as a passing complete race suite.
- Strict OpenSpec validation and `git diff --check` pass. Graphify rebuilt
  11216 nodes, 15158 edges, and 873 communities with the new lifecycle callback
  in the statistics community. Final GitNexus indexing reports 13110 nodes,
  41521 edges, and 300 flows. Its aggregate HIGH change result was reviewed:
  six flows are expected wrapper initialization/option registration and five
  are bounded local stream-done context/log flows; no unexpected business,
  security, network, credential, billing, or persistence flow is introduced.
- Task 5.6 remains unchecked pending a fresh independent review of the fix
  commit and an explicit `APPROVED` result.

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
