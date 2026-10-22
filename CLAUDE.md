## Project Documentation

Use [docs/README.md](docs/README.md) for current plugin contracts, design notes, and outstanding verification records.

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **wasm-go** (12728 symbols, 32177 relationships, 300 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

> If any GitNexus tool warns the index is stale, run `npx gitnexus analyze --index-only` in terminal first.

## Always Do

- **MUST run impact analysis before editing any symbol.** Before modifying a function, class, or method, run `gitnexus_impact({target: "symbolName", direction: "upstream"})` and report the blast radius (direct callers, affected processes, risk level) to the user.
- **MUST run `gitnexus_detect_changes()` before committing** to verify your changes only affect expected symbols and execution flows.
- **MUST warn the user** if impact analysis returns HIGH or CRITICAL risk before proceeding with edits.
- When exploring unfamiliar code, use `gitnexus_query({query: "concept"})` to find execution flows instead of grepping. It returns process-grouped results ranked by relevance.
- When you need full context on a specific symbol — callers, callees, which execution flows it participates in — use `gitnexus_context({name: "symbolName"})`.

## Never Do

- NEVER edit a function, class, or method without first running `gitnexus_impact` on it.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis.
- NEVER rename symbols with find-and-replace — use `gitnexus_rename` which understands the call graph.
- NEVER commit changes without running `gitnexus_detect_changes()` to check affected scope.

## Resources

| Resource | Use for |
|----------|---------|
| `gitnexus://repo/wasm-go/context` | Codebase overview, check index freshness |
| `gitnexus://repo/wasm-go/clusters` | All functional areas |
| `gitnexus://repo/wasm-go/processes` | All execution flows |
| `gitnexus://repo/wasm-go/process/{name}` | Step-by-step execution trace |

## CLI

When MCP tools are unavailable, use the corresponding `npx gitnexus` command:
`query`, `context`, `impact`, or `detect-changes`. Pass `--repo <absolute-repository-path>`
to select a repository, and use `npx gitnexus help <command>` for its arguments.
Before committing, use `detect-changes --scope staged` to review the proposed commit.

Refresh indexes with `npx gitnexus analyze --index-only` so indexing preserves the
repository's agent instructions and skill files.

<!-- gitnexus:end -->
