## 1. Regression and Fix

- [x] 1.1 Add a failing regression for incrementing a counter with a nil cache.
- [x] 1.2 Lazily initialize the counter cache without changing metric semantics.

## 2. Corrective Release

- [x] 2.1 Advance all authoritative ai-statistics release surfaces to `1.0.2`.
- [x] 2.2 Build the versioned Wasm artifact and record its size and SHA-256.

## 3. Verification

- [x] 3.1 Run Go formatting and focused/full ai-statistics tests.
- [x] 3.2 Run strict OpenSpec validation, Graphify update, GitNexus change analysis, and diff checks.
- [x] 3.3 Deploy `1.0.2` locally and verify the eligible request lifecycle completes without a Wasm panic.

## Release Evidence

- Artifact: `build/plugins/ai-statistics/1.0.2/plugin.wasm`
- Source commit: `e6e47215ddd1f1b304b69ff7bdf295e191086b1a`
- Size: `6915319` bytes
- SHA-256: `4bd6fb6c52ad68eb1957a688c9e02b8c6791c9db16d6bc143e83db3a54e7aa44`
- Upload: HTTP `201`; loopback retrieval was byte-identical.
- Runtime: the deployed `ai-statistics-1.0.2` emitted route-scoped counters for terminal HTTP `200`, `400`, `404`, and `429` responses without a nil-map panic.
- Manifest: `docs/release-evidence/fix-ai-statistics-counter-initialization.json`
