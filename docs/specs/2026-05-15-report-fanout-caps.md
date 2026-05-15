# Security Hardening — Report Fan-Out Caps

**Date:** 2026-05-15
**Goal:** Cap the per-user × per-week API fan-out in `tdx time report status` and `tdx project time` (and their MCP equivalents) so accidental or LLM-driven calls cannot generate thousands of TeamDynamix API requests and trip the 60/min/IP rate limit.

## Motivation

A security audit on 2026-05-15 found that the report runner accepts unbounded `--from`/`--to` ranges and resolves user selectors to as many as 1000 staff. With one TD API request per (user, week), an MCP-driven call like `--from 2010-01-01 --to 2030-01-01 --all` could schedule ~1,000,000 requests, swamping the tenant and tripping rate limits.

This is more than theoretical: v0.19.0 already tripped TD's 60/min/IP rate limit on a 31-resource project before the doubled-call regression was fixed. The runner trusts the input range.

This spec hardens the input boundary with two independent hard caps. It is the first of a multi-phase security hardening rollup (see [Roadmap](#roadmap)).

## Scope

In scope:

- `tdx time report status` — CLI surface
- `get_time_status_report` — MCP equivalent
- `tdx project time` — CLI surface (shares `resolveWeeks` and the user-fan-out)
- `get_project_time_review` — MCP equivalent

Out of scope (deferred to later phases — see [Roadmap](#roadmap)):

- Path traversal in template / draft / profile names (audit finding #1)
- Web editor session protection (audit finding #2)
- Profile revalidation on load (audit finding #3)
- Token storage / OS keychain (audit finding #5)
- CI/release supply-chain hardening (audit finding #6)
- Unsafe token-handling shell snippet in docs (audit finding #7)

## Policy

**Hard caps, refuse over limit.** Same caps on CLI and MCP — simpler and consistent. No override flag; if the user genuinely needs a multi-year range, they issue separate calls.

The point isn't to keep every call below TD's rate limit; it's to refuse pathological ranges (e.g., 20-year `--from`/`--to`) before any IO. Within-cap calls still use the existing concurrency limit of 5 to be courteous to TD.

## Caps

Two independent caps. Either one trips → refuse.

| Constant | Value | What it caps |
|---|---|---|
| `MaxReportWeeks` | **52** | Span between `--from` and `--to` inclusive, in Sunday-anchored weeks. `--week` (single week) trivially passes. |
| `MaxReportUsers` | **1000** | Count of resolved users after selector expansion (`--user`, `--manager`, `--account`, `--resource-pool`, `--all`, `--project`). |

Worst-case allowed call is 52 × 1000 = 52,000 requests — still big, but bounded.

Both exported as package-level consts in `internal/cli/time/report/runner.go` so tests can reference them directly.

## Where the checks live

### Week cap

In `resolveWeeks` (`internal/cli/time/report/runner.go:287`). After parsing `from`/`to`, compute week count by iterating Sunday-anchored `WeekRef`s and refuse before any IO. This function is on the path for both CLI and MCP.

Implementation note: `resolveWeeks` already iterates weeks DST-safely (uses Sunday-anchored construction, not `Sub().Hours()/24`). The cap check is added inline, no algorithm change. See `[[feedback_dst_calendar_arithmetic]]`.

### User cap

Two complementary checks:

1. **Existing flag check (unchanged):** `validateStatusFlags` already refuses `--limit > 1000` with `--limit cannot exceed 1000` (`status.go:144-146`). We keep this exactly as-is for source-compat — both CLI and MCP go through this path. We do upgrade the message to match the new structured error shape so MCP gets a parseable code (see [Error shapes](#error-shapes)).

2. **New resolved-set ceiling:** In `assembleReport` after `resolveUsers`, before the fan-out (`runner.go:77-84`). Today the code silently truncates: `if len(users) > cap { users = users[:cap] }`. We change this so when the *resolved* user set exceeds `MaxReportUsers` (1000), the call refuses with `fanout_limit_exceeded`, limit=users. This is the case where the user typed no explicit `--limit` but a wide selector (e.g. `--all` on a 2000-staff tenant) expands beyond the cap.

After both checks, the existing `--limit N` truncation (for user-explicit narrowing where `N ≤ 1000`) continues unchanged. Net effect: by the time we reach the fan-out loop, both `len(users) ≤ 1000` and `--limit ≤ 1000` are guaranteed.

### `tdx project time`

The project-time path lives in the same `internal/cli/time/report/runner.go` (per v0.19.0 — the `--project` selector was added there). Both checks above cover it because it shares `resolveWeeks` and the user-fan-out.

Will verify during implementation: if any parallel `projectsvc.ListResources` → fan-out exists elsewhere, apply the same guards.

## Error shapes

### CLI (human)

```
$ tdx time report status --from 2020-01-01 --to 2030-01-01
Error: requested 522 weeks; max is 52. Narrow the range with --week or a tighter --from/--to.
```

Exit code 1. No partial output. Same pattern for user-count refusal:

```
$ tdx time report status --all
Error: resolved 1247 users; max is 1000. Narrow with --resource-pool, --account, or --manager.
```

For `--json`: stderr gets the same message; stdout stays empty. Matches existing convention for flag-validation errors.

### MCP

Structured error returned from `RunForMCP` before assembly:

```json
{
  "error": {
    "code": "fanout_limit_exceeded",
    "limit": "weeks",
    "max": 52,
    "requested": 522,
    "hint": "Reduce the from/to range. The 52-week cap protects the TD API from accidental wide-range fan-out."
  }
}
```

User cap returns the same shape with `"limit": "users"`.

## Tests

Unit tests, no live tenant calls.

- `resolveWeeks` table:
  - 1 week (`--week`) ✓
  - 52 weeks (boundary) ✓
  - 53 weeks (over) ✗ — assert error message and `code: fanout_limit_exceeded`
  - Malformed dates: unchanged behavior
- `Run` with mocked services:
  - 1000 users ✓
  - 1001 users ✗ — boundary
- `RunForMCP` mirrors above and asserts the structured error shape (JSON marshaling matches the spec).
- DST clarity: a 52-week range that crosses spring-forward is still 52 weeks. Existing `resolveWeeks` iteration already DST-safe.
- `--limit N > 1000`: assert refusal at flag-validation (existing behavior preserved); assert error message now carries `fanout_limit_exceeded` shape.

Test fixtures: stub `peoplesvc` returns 1001 synthetic users for the over-cap path.

No live walkthrough doc — this is a guardrail, not new functionality.

## Out-of-scope behaviors (explicit)

- **Not changing concurrency** (stays at 5 via existing errgroup).
- **Not retrying on rate-limit responses** (separate concern, future phase).
- **Not adding new MCP confirmation surface** — `confirm:true` semantics unchanged; both affected MCP tools are read-only and don't gate on confirm today.
- **Not differentiating CLI from MCP limits.** Both share the same hard caps.

## Roadmap

This is Phase 1 of a multi-phase security hardening rollup. Each subsequent phase becomes its own spec → plan → PR cycle.

| Phase | Audit # | Severity | Topic |
|---|---|---|---|
| **1 (this spec)** | #4 | Medium | Cap report fan-out. |
| 2 | #1 | **High** | Path traversal: shared `ValidateArtifactName` for template / draft / profile names + weekStart date dir. |
| 3 | #2 | Medium | Web editor session nonce + Origin / Content-Type checks + explicit `127.0.0.1` bind. |
| 4 | #3 | Medium | Revalidate profiles on `Load` / `GetProfile`; enforce HTTPS in `tdx.NewClient` with loopback escape. |
| 5 | #7 | Low | Replace unsafe token-grep snippet in `docs/plans/2026-05-08-tdx-ticket-tasks.md:2034`. |
| 6 | #6 | Low | CI/release hardening: SHA-pin GoReleaser action, pin its version, `permissions: contents: read` on `ci.yml`. |
| 7 | #5 | Low/Med | Token storage: OS keychain default with YAML fallback. Larger refactor — separate brainstorming pass. |

Phase 2 (path traversal) is the highest-severity item and should be planned immediately after this lands.

## Open questions

None. All design decisions settled in the 2026-05-15 brainstorming session.
