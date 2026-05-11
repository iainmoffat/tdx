# Per-user threshold via WorkableHours (v0.16.5)

**Date:** 2026-05-11
**Goal:** `tdx time report status --incomplete` uses each user's `WorkableHours` (from TD) as the threshold by default, rather than a global `--threshold 40`. Explicit `--threshold N` still works as a global override. Closes a deferred item from v0.12.0.

> **Post-implementation correction (2026-05-11):** Live probing of Sample revealed that TD returns `WorkableHours` as **hours per day**, not per week (e.g. `8.0` for a FT staff member, not `40`). The runner multiplies by 5 (`workdaysPerWeek`) to compute the weekly threshold. All test fixtures, examples, and prose in this spec that show `WorkableHours: 40 / 32` as weekly values should be read as the equivalent daily values (`8.0 / 6.4`); the final code and documentation reflect the correct daily semantic.

## Motivation

TD already stores per-user expected weekly hours (`WorkableHours`) — admins set them when onboarding (40 FT, 32 PT, etc.). The current `tdx time report status --incomplete` command can't see that field; it applies one global threshold to everyone, so PT staff at 32 hours show as "incomplete" against the 40-default unless the operator runs separate commands per group or manually sifts the output. Closing this gap was explicitly deferred from the v0.12.0 incomplete-filter spec (`docs/specs/2026-05-04-time-report-status-incomplete-filter.md` § Q1 + deferred section).

## Decisions

Settled during brainstorming on 2026-05-11:

1. **Default behavior changes** from "global 40" to "per-user `WorkableHours`". Users who actually want 40-for-everyone now pass `--threshold 40` explicitly.
2. **`--threshold N` explicit** keeps the existing global-threshold behavior. Same flag, same value, same outcome — just no longer auto-applied when omitted.
3. **Fallback when `WorkableHours <= 0`**: use the literal `40.0`. Documented in the runner code and the user-facing docs.
4. **JSON envelope gains per-row `threshold` + filter-level `thresholdMode`** so consumers can see what was applied.
5. **MCP `get_time_status_report` semantics mirror the CLI:** `threshold` omitted → per-user; `threshold N` → global.
6. **Domain + wire change is additive:** `User.WorkableHours float64` + `wireUser.WorkableHours float64`. Existing callers ignore the new field.

## Decisions deferred / out of scope

- **Per-row "expected" column in the text output.** Could be useful (`Sarah 30/32 PT`) but adds noise and requires column-width re-balancing. Defer.
- **Threshold customization beyond global vs per-user** (e.g. per-group, per-role) — TD doesn't expose those concepts cleanly. Defer.
- **Status of users with `WorkableHours == 0`:** they fall back to the 40 default. We do not show a "no threshold configured" indicator, since it'd add noise for the typical case.
- **Backfill for tenants where TD doesn't populate `WorkableHours`:** if every user has 0 there, the new default is effectively the same as the old (`40 for everyone`). No degradation.

## Behavior

### Before (today)

```bash
tdx time report status --manager me --week 2026-04-19 --incomplete
# Uses 40 for every user. PT staff at 32h show "incomplete" even when they hit target.
```

### After

```bash
tdx time report status --manager me --week 2026-04-19 --incomplete
# Uses each user's WorkableHours (32 for PT, 40 for FT). PT staff who hit 32 are NOT in the result.

tdx time report status --manager me --week 2026-04-19 --incomplete --threshold 40
# Explicit global 40 — pre-v0.16.5 behavior restored.

tdx time report status --manager me --week 2026-04-19 --incomplete --threshold 20
# Global 20 for everyone — useful for "anyone with under 20 hours logged".
```

### Fallback when `WorkableHours <= 0`

Per-user threshold is computed as:

```
threshold(user) =
    globalThreshold            if --threshold was set explicitly
    user.WorkableHours         if user.WorkableHours > 0
    40.0                       otherwise (fallback default)
```

The 40-fallback is the literal documented in the runner. Configurable later if anyone wants it; YAGNI for now.

## Wire format

**Probe required** before locking the wire change. The implementer must verify (live, once the token's refreshed):

1. Does `GET /api/people/{uid}` return a field named exactly `WorkableHours`?
2. What is the JSON type — number, string, integer?
3. Is the value weekly or daily? (Common values would clarify: 40, 32 → weekly; 8, 6.4 → daily.)
4. Is the field always present, or omitted when unset?
5. Same probe on `POST /api/people/search` rows — does it survive the search-list wire shape?

Earlier probing during v0.9.x people-search work did NOT find a `WorkableHours` field in the response on the test tenant — that probe was during v0.9.1 (2026-04-30). The field may have been added since, or may live under a different shape. Live probe is mandatory before writing the wire struct.

**Documented expectation (per TD's published schema):** `WorkableHours` is a `Double` field on `User`, weekly hours, `0.0` when unset. If the live response differs, the wire-format fix is mechanical: rename the wire tag, change the type, adjust `decodeUser`.

## Domain types

`internal/domain/user.go` (additive):

```go
type User struct {
    // ... existing fields ...

    // WorkableHours is the user's expected weekly hours from TD.
    // 0 indicates "unset" — the report-status command falls back to a
    // global 40 default when computing per-user --incomplete thresholds.
    WorkableHours float64
}
```

## Service layer

`internal/svc/peoplesvc/types.go` adds the wire field:

```go
type wireUser struct {
    // ... existing fields ...
    WorkableHours float64 `json:"WorkableHours"`
}
```

`decodeUser` (in `internal/svc/peoplesvc/users.go`) plumbs it through:

```go
return domain.User{
    // ... existing fields ...
    WorkableHours: w.WorkableHours,
}
```

No new methods. `GetUser`, `SearchUsers`, `LookupPeople` all already deserialize via `wireUser`; they get the field for free.

## CLI layer — runner change

In `internal/cli/time/report/runner.go`, the `--incomplete` filter currently looks something like:

```go
if filter.Incomplete && row.TotalHours >= threshold {
    continue // skip rows that meet the threshold
}
```

After the change:

```go
// thresholdMode tells JSON consumers what was applied.
thresholdMode := "per-user"
if cmd.Flags().Changed("threshold") {
    thresholdMode = "global"
}

// thresholdFor returns the threshold to use for a given user row.
// - If --threshold was explicitly set, use the global value.
// - Else use user.WorkableHours when > 0.
// - Else fall back to 40.0.
const defaultThresholdFallback = 40.0
thresholdFor := func(u domain.User) float64 {
    if cmd.Flags().Changed("threshold") {
        return globalThreshold
    }
    if u.WorkableHours > 0 {
        return u.WorkableHours
    }
    return defaultThresholdFallback
}

// In the row loop:
rowThreshold := thresholdFor(row.User)
if filter.Incomplete && row.TotalHours >= rowThreshold {
    continue
}
```

The exact factoring depends on the existing `runner.go` shape — the implementer should preserve the existing variable names and threading. Goal: every row carries a `rowThreshold` that gets used both for filtering AND for the JSON envelope (next section).

### Edge case: `--incomplete` not set

When `--incomplete` is not set, no row-level threshold computation runs. `thresholdMode` is `""` (omitted from the JSON envelope). The per-row JSON `threshold` field is also omitted.

### Edge case: explicit `--threshold 0`

A user passing `--threshold 0` explicitly is saying "global threshold 0 → no row meets `TotalHours < 0` → all rows pass the incomplete filter." That's a degenerate filter (effectively a no-op for `--incomplete`). We pass it through without special handling — the filter just becomes a no-op, which is the intuitive result.

## JSON envelope

Schema name unchanged (`tdx.v1.timeStatusReport`).

Additive changes:

**Per row** (when `--incomplete` is set):

```json
{
  "userUID": "...",
  "userName": "...",
  "totalHours": 30.5,
  "threshold": 32.0,
  "status": "permission-granted",
  ...
}
```

`threshold` is the value used to filter THIS row. Omitted when `--incomplete` is not set.

**Filter section:**

```json
"filter": {
  "incomplete": true,
  "thresholdMode": "per-user"  // or "global"
}
```

`thresholdMode` is omitted when `--incomplete` is not set. The existing `filter.threshold` field stays for backwards compat — it's the global value when `thresholdMode=global`, and the literal 40.0 fallback when `thresholdMode=per-user` (or even better, omit it entirely in per-user mode — implementer's call, document either way in the field's `json:"-,omitempty"` tag and in the docs).

## Text output

No layout change. The existing OVERALL footer and per-row table stay identical.

**Optional one-line hint** above or below the table when `--incomplete` is on AND `thresholdMode=per-user`:

```
filter: incomplete (per-user WorkableHours; pass --threshold N to override)
```

Skip the hint if the existing renderer doesn't have a natural place for it. YAGNI.

## MCP

`get_time_status_report` tool's `threshold float64` input semantics:

- Omitted (zero value) → per-user mode (mirrors CLI `--threshold` not set).
- Set explicitly > 0 → global threshold for all rows.
- Set to 0 explicitly via the JSON arg → currently indistinguishable from "omitted" because JSON omits zero values; the MCP handler treats `threshold == 0` as "per-user mode". Acceptable for v0.16.5; not worth a `thresholdSet bool` flag for an edge case nobody asks for.

## Tests

1. **Domain test:** `User.WorkableHours` zero value is valid; round-trip through JSON marshal/unmarshal preserves it. (One test, very small.)
2. **Service-layer test:** httptest fixture returns a `wireUser` with `WorkableHours: 32.0`; `GetUser` and `SearchUsers` decode it correctly. (Add to existing `peoplesvc/users_test.go`.)
3. **Runner-level tests** (in `internal/cli/time/report/runner_test.go`):
   - `TestRunStatusPerUserThresholdFromWorkableHours` — stub returns 2 users (Alice 40h workable / 35h logged; Bob 32h workable / 30h logged). `--incomplete` (no `--threshold`) → only Alice is in result (35 < 40). Bob (30 ≥ 32 false… wait, 30 < 32, so Bob IS incomplete). Re-state: Alice 40/35 → incomplete; Bob 32/30 → incomplete; both appear. Better test: Alice 40/40 → NOT incomplete; Bob 32/30 → incomplete; only Bob in result.
   - `TestRunStatusPerUserThresholdFallback` — user with `WorkableHours: 0` and `TotalHours: 30` → falls back to 40 → 30 < 40 → row is incomplete.
   - `TestRunStatusGlobalThresholdOverridesPerUser` — `--threshold 20`: Alice 40/35 → not incomplete (35 ≥ 20); Bob 32/30 → not incomplete (30 ≥ 20). Both filtered out. Global overrides.
   - `TestRunStatusIncompleteNotSetIgnoresThreshold` — `--incomplete` not set → all rows pass through regardless of threshold/WorkableHours.
   - `TestRunStatusJSONEnvelopeHasThresholdMode` — JSON output includes `filter.thresholdMode = "per-user"` when `--incomplete` is on and `--threshold` is not set; `"global"` otherwise.
   - `TestRunStatusJSONEnvelopePerRowThreshold` — JSON rows include `threshold` field matching the per-user value used.

## Documentation

- `docs/guide/time.md` `## tdx time report status` section: document the new per-user behavior, the fallback, and the `--threshold N` global override. Add a bash example showing both modes.
- No README or tree changes.

## Acceptance criteria

1. `tdx time report status --manager me --week ... --incomplete` (no `--threshold`) uses each user's `WorkableHours` when > 0, else falls back to 40.
2. `tdx time report status --manager me --week ... --incomplete --threshold N` applies N globally (preserves pre-v0.16.5 behavior).
3. JSON output includes per-row `threshold` and filter-level `thresholdMode = "per-user"|"global"` when `--incomplete` is set.
4. `domain.User.WorkableHours` field exists; `peoplesvc` deserializes it.
5. Live-verified on the test tenant: at least one user with FT WorkableHours (40) and one with PT (32 or other) — the `--incomplete` result should differ from the pre-v0.16.5 global-40 behavior.
6. All existing tests pass; new tests pass; `go vet` + `golangci-lint` clean.
7. Doc updated.
8. Released as v0.16.5 (PR + squash + tag + Goreleaser).

## Risks and mitigations

- **`WorkableHours` field may not exist or may be named differently on the test tenant.** Mitigation: probe live before locking the wire struct. Earlier probe (v0.9.1, 2026-04-30) didn't surface it — that may have been a different endpoint or a field added since. Concretely, the implementer must:
  1. Refresh auth (token expired during planning).
  2. `curl /api/people/<my-uid>` and grep for `Workable`/`Hours`/`Capacity`/`Expected` — if no match, dump all fields and pick the closest. If genuinely missing, escalate (this becomes a "no WorkableHours data" gap, which means we fall back to 40 for everyone anyway — equivalent to current behavior — and the feature still ships as "no-op when data missing").
  3. Same probe against a `POST /api/people/search` row.
- **Backwards-compat for users who relied on the old default-40 behavior.** Mitigation: document in the release notes; `--threshold 40` restores the old behavior verbatim.
- **Floating-point comparisons.** `WorkableHours == 32.0` and `TotalHours == 32.0` should be considered "not incomplete." We use `>=` which is exact for typical values. No epsilon needed.
- **Service layer fetches `WorkableHours` even when the report doesn't need it.** Acceptable — the field is already on every `wireUser` response and adds zero API calls.
