## Context

`ai-billing` parses matched route configuration into `BillingConfig`, captures
request facts in proxy-wasm request context, and constructs one `BillingEvent`
after response completion. Internal-cost handling already removes tenant
identity and captures `x-ai-billing-source-job` and
`x-ai-billing-upstream-invoked`.

Console workers also send `x-ai-billing-cost-source` and
`x-ai-billing-worker-kind`, but the plugin currently ignores them and always
copies `BillingConfig.CostSource` and `BillingConfig.WorkerKind`. This loses
operation attribution on reusable generic internal routes.

## Goals / Non-Goals

**Goals:**

- Preserve a complete trusted invocation-level source/worker pair.
- Retain legacy route configuration fallback when neither header is present.
- Fail open for model traffic while suppressing an unsafe billing event.
- Keep request values out of logs and user responses.
- Publish and build plugin version `0.1.4`.

**Non-Goals:**

- Let request headers enable internal-cost mode or customer billing.
- Perform Console semantic source mapping, provider pricing, settlement, or
  idempotency.
- Change Redis Stream delivery, usage parsing, event identity, provider
  resolution, or response correlation.

## Decisions

1. Capture attribution only inside the existing
   `config.EventKind == eventKindInternalCost` branch. This uses the matched
   route rule as the trust boundary and prevents customer routes from
   self-declaring internal costs.
2. Treat the two headers as one atomic pair. Both values must be non-empty
   after trimming, no longer than 128 bytes, and contain only lowercase ASCII
   letters, digits, `.`, `-`, or `_`.
3. Store a valid pair in request-scoped contexts. `buildBillingEvent` reads the
   contexts with `BillingConfig` values as defaults, keeping legacy callers
   compatible when both headers are absent.
4. If either header is present but the pair is incomplete or malformed, set
   billing disabled for the request, stop body buffering, log one fixed warning
   without the submitted values, and continue model traffic.
5. Ignore request headers for event kind and customer billing.
   `BillingConfig.EventKind` and `BillingConfig.BillCustomer` remain
   authoritative.
6. Advance `VERSION` and the example plugin URL to `0.1.4`; test the full
   extension and build the versioned Wasm artifact.

## Risks / Trade-offs

- [Suppressing an invalid event loses one operating-cost record] → Preserve
  model availability and make missing canonical costs a blocking Console E2E
  diagnostic.
- [Generic syntactically valid namespaces can pass the gateway] → Keep the
  gateway transport-neutral; Console performs semantic namespace validation.
- [An untrusted public caller might try the headers] → Ignore them unless the
  matched rule is already trusted `internal_cost`.
- [Plugin and Console versions can drift] → Publish a new version and require
  runtime-version verification before E2E.

## Migration Plan

1. Add failing proxy-wasm lifecycle tests.
2. Implement bounded paired-header capture and context fallback.
3. Run focused and complete Go tests and `go vet`.
4. Update bilingual documentation, `VERSION`, and `plugin.yaml`.
5. Build `build/plugins/ai-billing/0.1.4/plugin.wasm`.
6. Deploy through the Modelfusion local E2E workflow and verify emitted Redis
   events and Console audit records.

Rollback deploys the prior plugin artifact. No persisted data migration is
required.

## Open Questions

None.
