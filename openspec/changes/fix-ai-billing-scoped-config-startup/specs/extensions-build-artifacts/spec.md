## MODIFIED Requirements

### Requirement: AI plugin artifacts use authoritative versioned release paths

The coordinated plugin artifact set SHALL identify ai-proxy `1.0.2`,
ai-billing `0.1.6`, ai-quota `1.0.2`, and ai-statistics `1.0.2`.
ai-billing SHALL be built from the reviewed scoped-startup fix and written to
`build/plugins/ai-billing/0.1.6/plugin.wasm`.

#### Scenario: Corrective ai-billing artifact is built

- **WHEN** the scoped-startup correction is released
- **THEN** the authoritative extension and resource version SHALL be `0.1.6`
- **AND** the exact artifact size, SHA-256, source commit, build command, test
  command, and availability evidence SHALL be recorded.
