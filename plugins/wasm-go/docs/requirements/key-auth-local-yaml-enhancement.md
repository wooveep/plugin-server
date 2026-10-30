# Key Auth Local YAML Enhancement Requirements

## Background

The Go `key-auth` plugin currently supports a simple local YAML model:

- `consumers[].credential`
- global `keys`
- global `in_query` / `in_header`
- route/domain `allow`
- authenticated requests receive `X-Mse-Consumer`

The C++ `key_auth` plugin supports a richer model, including top-level `credentials`, multiple credentials per consumer, consumer-level key extraction settings, `realm`, and `X-HI-ORIGINAL-AUTH` fallback. The Go plugin should close the practical compatibility gap while keeping credential data in local YAML for this phase.

## Decision

This phase keeps API keys in the WasmPlugin YAML configuration.

External Redis or other remote credential storage is explicitly out of scope for this requirement. Redis may be considered later only if API key volume, rotation frequency, or operational requirements make YAML management unsuitable.

## Goals

- Support multiple credentials per consumer through `consumers[].credentials`.
- Preserve compatibility with existing `consumers[].credential`.
- Support consumer-level `keys`, `in_query`, and `in_header` overrides.
- Support top-level `credentials` mode for authentication-only use cases.
- Support `realm` for `WWW-Authenticate` response headers.
- Support API key extraction from `Authorization: Bearer <api-key>` when `Authorization` is configured as a key source.
- Add `tenant` to consumers and inject trusted identity headers after successful authentication.
- Preserve existing `global_auth` and route/domain `allow` behavior.
- Keep all credential lookup local and in-memory after YAML parsing.

## Non-Goals

- Do not add Redis, database, HTTP callout, or dynamic credential loading.
- Do not implement API key lifecycle management, rotation APIs, audit logs, or admin endpoints.
- Do not hash or encrypt YAML credentials in this phase.
- Do not change unrelated auth plugins.
- Do not make `tenant` mandatory for all existing consumers unless a future compatibility decision requires it.

## Required Configuration Model

### Existing Compatible Form

```yaml
global_auth: true
consumers:
  - name: consumer1
    credential: token1
keys:
  - x-api-key
  - apikey
in_header: true
in_query: true
```

### Multiple Credentials And Tenant

```yaml
global_auth: true
consumers:
  - name: consumer-ocloudware-liyuntian-1
    tenant: ocloudware
    credentials:
      - real-api-key-1
      - real-api-key-2
    keys:
      - Authorization
    in_header: true
    in_query: false
keys:
  - apikey
  - x-api-key
in_header: true
in_query: true
```

### Authorization Bearer API Key

```yaml
global_auth: true
consumers:
  - name: consumer1
    tenant: tenant-a
    credentials:
      - real-api-key
keys:
  - apikey
  - x-api-key
  - Authorization
in_header: true
in_query: true
```

Request:

```bash
curl http://example.com/v1/chat/completions \
  -H 'Authorization: Bearer real-api-key'
```

Expected credential value used for matching:

```text
real-api-key
```

### Top-Level Credentials Mode

```yaml
global_auth: true
credentials:
  - real-api-key-1
  - real-api-key-2
keys:
  - Authorization
realm: MSE Gateway
in_header: true
in_query: false
```

Top-level `credentials` mode is authentication-only. It does not define consumer names or tenants, so it must not inject `X-Mse-Consumer` or `X-Mse-Tenant`.

## Validation Requirements

- `consumers` and top-level `credentials` must not appear at the same config level.
- At least one of `consumers` or top-level `credentials` must be configured.
- `consumers[].credential` and `consumers[].credentials` must not both appear on the same consumer.
- Each consumer must have `name`.
- Each consumer must have either non-empty `credential` or non-empty `credentials`.
- A credential value must not be duplicated across consumers.
- `keys` must be non-empty when required by any consumer that does not define consumer-level `keys`.
- If a consumer defines `keys`, those keys override global `keys` for that consumer.
- `in_header` and `in_query` must allow at least one enabled source after applying consumer-level overrides.
- `realm` is optional and defaults to `MSE Gateway`.
- `tenant` is optional for compatibility. If present and authentication succeeds, it must be propagated as `X-Mse-Tenant`.

## Extraction Requirements

Credential extraction must support both header and query sources when both are enabled.

```text
configured keys
  ├─ header lookup, when in_header=true
  │    └─ if key is Authorization and value starts with "Bearer ", strip prefix
  └─ query lookup, when in_query=true
```

Important behavior:

- Header matching must keep current proxy-wasm header behavior and should not introduce case-sensitive surprises beyond the host API behavior.
- `Authorization` Bearer stripping applies only to the configured `Authorization` key source.
- Bearer prefix matching should accept the standard `Bearer ` form.
- Non-Bearer `Authorization` values should be treated as raw credential values for compatibility.
- Multiple presented credential values should continue to be rejected as ambiguous.

## Authentication And Header Injection

After successful consumer authentication:

- Add or overwrite `X-Mse-Consumer` with the authenticated consumer `name`.
- If the authenticated consumer has `tenant`, add or overwrite `X-Mse-Tenant` with the configured tenant.
- Header values must come only from the matched local YAML consumer, not from incoming client-supplied identity headers.

Recommended request flow:

```text
request
  |
  v
extract candidate credential(s)
  |
  v
reject none / reject multiple
  |
  v
local YAML credential lookup
  |
  +-- top-level credentials match --> authenticated, no identity headers
  |
  +-- consumer credentials match
        |
        v
     check allow list
        |
        v
     inject X-Mse-Consumer and optional X-Mse-Tenant
```

## Authorization Requirements

- Existing `global_auth` semantics must be preserved.
- Existing route/domain `allow` semantics must be preserved for named consumers.
- `allow` applies to consumer `name`.
- Top-level `credentials` mode has no consumer name and therefore must not be used for consumer-level authorization decisions.
- Empty or missing `allow` behavior must remain compatible with the current Go plugin unless a separate proposal explicitly changes it.

## Error Response Requirements

- Missing credential: return the existing no-key response behavior.
- Multiple credentials in one request: return the existing multi-key response behavior.
- Unknown credential: return unauthorized/invalid credential behavior.
- Authenticated consumer not in `allow`: return unauthorized consumer behavior.
- `WWW-Authenticate` must use configured `realm`, defaulting to `MSE Gateway`.

## Backward Compatibility

Existing configurations using only `consumers[].credential`, global `keys`, global `in_header`, global `in_query`, `global_auth`, and `allow` must continue to work.

Existing downstream plugins that read `X-Mse-Consumer` must continue to work. New `X-Mse-Tenant` enables downstream plugins such as `ai-quota` to use tenant identity without parsing raw API keys.

## Acceptance Criteria

- Existing key-auth tests continue to pass.
- New tests cover `consumers[].credentials` with multiple keys for one consumer.
- New tests cover duplicate credentials across consumers.
- New tests cover consumer-level `keys`, `in_header`, and `in_query`.
- New tests cover global key fallback when a consumer has no consumer-level keys.
- New tests cover `Authorization: Bearer <api-key>` extraction.
- New tests cover non-Bearer `Authorization` raw credential compatibility.
- New tests cover `tenant` header injection.
- New tests verify incoming `X-Mse-Consumer` and `X-Mse-Tenant` cannot spoof authenticated identity.
- New tests cover top-level `credentials` authentication without identity header injection.
- Documentation examples are updated for multi-credential consumers, Bearer Authorization, and tenant propagation.

## Future Consideration: External Credential Store

If API keys grow beyond what is operationally reasonable in YAML, a future change may introduce Redis-backed credential lookup. That future design should consider:

- storing hashed or HMACed API keys instead of plaintext;
- local short-lived cache to avoid Redis on every request;
- fail-closed default for Redis errors;
- explicit timeout and fallback policy;
- compatibility with `X-Mse-Consumer` and `X-Mse-Tenant` injection;
- migration path from YAML to external storage.

This future storage work is not part of the current local YAML enhancement.
