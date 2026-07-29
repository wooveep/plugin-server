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
  `legacy_suffixes`, reject unsupported scope values, normalize and validate
  bounded/deduplicated HTTP method tokens, preserve legacy default/explicit
  suffix behavior, and accept absent or explicitly empty advanced filters
  only as non-narrowing bound-target projections. A rule changing from legacy
  to bound-target scope does not inherit the legacy suffix projection; rule
  filters still replace only their corresponding request gate, while the
  complete global Redis target and omitted business fields remain inherited.
  Removed `billing_service` and rule-level `redis_stream` rejection are
  unchanged.
- Request admission remains suffix-required in legacy mode. Bound-target mode
  admits the already selected Higress target when filters are absent/empty,
  while each explicit non-empty suffix or normalized method list narrows it
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
  change history; task-local detection is LOW across exactly two changed
  files with zero affected processes. The retained source/spec delta is
  limited to this routing gate and its Task 3.4 evidence.

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
