## 1. Safety And Failing Tests

- [x] 1.1 Run Graphify and GitNexus upstream impact gates for `onHttpRequestHeaders` and `buildBillingEvent`
- [x] 1.2 Add failing lifecycle tests for digest, extraction, and embedding invocation pairs overriding generic route attribution
- [x] 1.3 Add failing tests for no-header configuration fallback, partial-pair suppression, malformed-pair suppression, and customer-rule non-spoofing

## 2. Trusted Attribution Implementation

- [x] 2.1 Add bounded paired-header parsing only inside matched `internal_cost` request handling
- [x] 2.2 Store valid source and worker values in request context and use them with route-config fallback during event construction
- [x] 2.3 Suppress unsafe event delivery without blocking model traffic or logging submitted values
- [x] 2.4 Run focused and complete `ai-billing` Go tests and `go vet`

## 3. Release Contract

- [x] 3.1 Document the trusted paired-header boundary in Chinese and English READMEs
- [x] 3.2 Set `VERSION` and the plugin example artifact URL to `0.1.4`
- [x] 3.3 Build and verify the non-empty `build/plugins/ai-billing/0.1.4/plugin.wasm` artifact

## 4. Final Verification

- [x] 4.1 Update Graphify and review GitNexus change detection for the focused plugin flow
  - Graphify rebuilt 10,986 nodes and 14,624 edges. GitNexus comparison with `main` reported LOW risk, 97 changed indexed symbols, 11 files, and no affected execution process.
- [x] 4.2 Validate the OpenSpec change and verify formatting, tests, vet, artifact build, and a clean feature-branch worktree
  - Strict OpenSpec validation, formatting, complete `ai-billing` tests, vet, and the non-empty 0.1.4 artifact passed; the committed feature branch has no source or artifact drift.
- [x] 4.3 Support the Modelfusion focused and full E2E proof for canonical AI Memory costs and zero customer billing
  - Modelfusion run `20260728T071000Z` recorded 164 positive internal-cost records totaling 83,370,500 micro-CNY across digest, extraction, and embedding, with complete source/worker pairs and zero customer-billed internal-cost records.
