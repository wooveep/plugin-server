## Why

Higress starts the ai-billing Wasm VM with an empty base configuration when
`defaultConfigDisable: true` scopes execution to match rules. Version 0.1.5
rejects that dormant base before it can apply the complete route rule, so the
gateway rejects the plugin and returns HTTP 500.

## What Changes

- Accept an empty base configuration as a safe dormant configuration.
- Allow a matching rule to provide the complete Redis Stream configuration
  only when the base is dormant; keep replacement forbidden when a global
  Redis target exists.
- Publish the corrected ai-billing extension and resource as version `0.1.6`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `ai-billing-events`: Define dormant base configuration and complete
  scoped-rule startup behavior.
- `ai-plugin-resource-docs`: Expose and document scoped delivery configuration.
- `extensions-build-artifacts`: Advance the authoritative ai-billing release to
  `0.1.6`.

## Impact

- ai-billing configuration parsing, request no-op behavior, tests, schemas,
  documentation, and release evidence.
- No change to event payloads, settlement authority, trusted attribution, or
  legacy global configuration behavior.
