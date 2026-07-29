## 1. Regression and Fix

- [x] 1.1 Add a failing regression for incrementing a counter with a nil cache.
- [x] 1.2 Lazily initialize the counter cache without changing metric semantics.

## 2. Corrective Release

- [x] 2.1 Advance all authoritative ai-statistics release surfaces to `1.0.2`.
- [ ] 2.2 Build the versioned Wasm artifact and record its size and SHA-256.

## 3. Verification

- [x] 3.1 Run Go formatting and focused/full ai-statistics tests.
- [ ] 3.2 Run strict OpenSpec validation, Graphify update, GitNexus change analysis, and diff checks.
- [ ] 3.3 Deploy `1.0.2` locally and verify the eligible request lifecycle completes without a Wasm panic.
