# tdx — Phase B.2a Design Spec

**Date:** 2026-04-28
**Status:** Approved (brainstorming complete)
**Owner:** ipm
**Builds on:** Phase B.1 (v0.5.0, shipped 2026-04-28)
**Splits framework spec's Phase B.2 into:** B.2a (this) + B.2b (deferred)

---

## 0. Decisions log

| # | Decision | Implication |
|---|---|---|
| Q1 | **Split B.2 into B.2a + B.2b.** B.2a ships the three-way merge engine + auto-merge + three deterministic strategies (`abort` / `ours` / `theirs`). B.2b ships the `surface` strategy + `SyncConflicted` state machine + `resolve` command + (eventually) in-editor conflict markers. | B.2a ends with a draft that is always in a clean (non-conflicted) state. No state-machine complexity in this phase. |
| Q2 | **`--strategy abort` as the default.** When refresh detects any cell-level conflict between local and remote, it aborts cleanly — mutates nothing on disk, exits non-zero, lists the conflicts. User then chooses `--strategy ours` / `theirs` to retry, or breaks the impasse with `reset --yes`. | Smallest possible B.2a surface. No interactive prompt machinery, no `conflicted` state, no YAML conflict markers. Matches the project's "safety first" tone (`--allow-deletes`, `--yes`, `--force`). |
| Q3 | **Cell-level merge rules** confirmed (full table in §4). Key cases: untouched-on-both = pass-through; one-side-changed = adopt that side; both-sides-changed-different = CONFLICT; both-sides-changed-same = silent convergence; local-cleared + remote-deleted = silent reality match (cell drops out); local-edited + remote-deleted = CONFLICT. | Engine is pure: given three views (at-pull-time, current-local, current-remote), classify each cell deterministically. |
| — | **`refresh` and `rebase` are aliases.** Same command, same flags. No behavioral difference. | Hardcore git users reach for `rebase` reflexively. Aliasing avoids the confusion of "which one am I supposed to use?" while requiring only a single line of cobra wiring. |

---

## 1. Goal & scope

### What B.2a adds

The three-way merge primitive — finally lets a user pull a week, edit locally, refresh against the latest remote without losing edits or clobbering remote changes. Closes the deferred half of Phase B's framework-spec'd commitments.

- `tdx time week refresh <date>[/<name>]` (alias: `rebase`)
- `--strategy abort | ours | theirs`, default `abort`
- Pre-refresh auto-snapshot (using the `OpPreRefresh` constant declared but unused since Phase A)
- Watermark update on successful refresh (`.pulled.yaml` rewritten to reflect the now-current remote)
- `refresh_week_draft` MCP tool (mutating, `confirm: true`)
- New `tdx.v1.weekDraftRefreshResult` JSON schema
- README + guide.md "Refresh & rebase" subsection
- Manual walkthrough

### What B.2a explicitly does NOT add (deferred to B.2b)

- The `surface` strategy: produce a partial-merge draft + conflict markers in YAML.
- The `SyncConflicted` state production. (`CellConflict` and `SyncConflicted` enum values stay declared-but-unproduced after B.2a — they wait for B.2b.)
- The `resolve` command for cell-by-cell pick.
- TTY-interactive prompt mode for refresh.
- In-editor conflict marker rendering.

These get their own brainstorming pass once B.2a has shipped and we have real-world miles on the engine.

---

## 2. Domain delta

**None.** `WeekDraft`, `DraftCell`, `DraftSyncState`, `CellState` are all unchanged.

`OpPreRefresh OpTag = "pre-refresh"` already exists in `internal/svc/draftsvc/snapshot.go` (declared in Phase A, never emitted). B.2a starts emitting it.

The `CellConflict` and `SyncConflicted` enum values declared in Phase A's `internal/domain/draft.go` remain declared-but-unproduced after B.2a. B.2b will start emitting them.

---

## 3. Engine

New file: `internal/svc/draftsvc/refresh.go`. The engine is structured as a pure function the Service wraps:

```go
type Strategy string

const (
    StrategyAbort  Strategy = "abort"
    StrategyOurs   Strategy = "ours"
    StrategyTheirs Strategy = "theirs"
)

// MergeConflict describes one cell where local and remote diverged in a way
// the engine cannot resolve without a strategy.
type MergeConflict struct {
    RowID             string
    Day               string  // "Monday", "Tuesday", ... (time.Weekday.String())
    LocalDescription  string  // human-readable summary of local intent
    RemoteDescription string  // human-readable summary of remote state
}

// RefreshResult reports what happened. Aborted=true with a non-empty Conflicts
// list means refresh refused to mutate anything.
type RefreshResult struct {
    Strategy           Strategy
    Adopted            int  // cells whose remote changes were taken
    Preserved          int  // cells where local edits survived
    Resolved           int  // cells where both sides converged on the same value (no conflict)
    ResolvedByStrategy int  // cells where a real conflict was resolved by ours/theirs (always 0 under abort)
    Aborted            bool
    Conflicts          []MergeConflict
}

// Service.Refresh fetches the current remote, computes the three-way merge,
// and either mutates the draft (success / ours / theirs) or aborts (conflicts
// detected under StrategyAbort).
func (s *Service) Refresh(ctx context.Context, profile string, weekStart time.Time, name string, strategy Strategy) (RefreshResult, error)
```

### Algorithm sketch

```
Refresh(profile, weekStart, name, strategy):
  draft        := store.Load(profile, weekStart, name)
  pulled       := store.LoadPulledSnapshot(profile, weekStart, name)
                  -- if absent, abort with error: "draft has no pull watermark; cannot refresh"
  remote       := timesvc.GetWeekReport(profile, weekStart)
  remoteDraft  := buildDraftFromReport(profile, name, remote)
                  -- reuses the existing function from Phase A's pull pipeline

  -- Build cell-level views keyed by (rowID, weekday).
  pulledByKey  := pulledCellsByKey(pulled)
  localByKey   := allCellsByKey(draft)
  remoteByKey  := allCellsByKey(remoteDraft)

  -- Classify every cell visible in any view.
  merged, conflicts := classify(pulledByKey, localByKey, remoteByKey, strategy)

  if strategy == abort && len(conflicts) > 0:
    return RefreshResult{Aborted: true, Conflicts: conflicts}, nil
    -- NO disk mutation

  -- Strategy ours/theirs: every conflict was already resolved by classify.
  -- merged contains the post-merge cell set.

  snapshots.Take(draft, OpPreRefresh, "")    -- pre-refresh rollback point
  newDraft := assembleDraft(draft, merged)   -- preserves identity, notes, tags, etc.
  newDraft.ModifiedAt = time.Now().UTC()

  store.Save(newDraft)
  store.SavePulledSnapshot(remoteDraft)      -- fresh watermark

  return RefreshResult{
    Strategy:           strategy,
    Adopted:            counts.adopted,
    Preserved:          counts.preserved,
    Resolved:           counts.resolved,
    ResolvedByStrategy: counts.resolvedByStrategy,  -- 0 under abort; >0 under ours/theirs when conflicts existed
    Aborted:            false,
    Conflicts:          nil,
  }, nil
```

### Cell-state inputs to classify

For each `(rowID, weekday)` key seen in any of the three views:

- **`pulled`** — what was at the at-pull-time watermark. May be absent if cell was added since pull.
- **`local`** — what's currently in the draft. May be absent if cell was added remotely since pull.
- **`remote`** — what's currently on TD. May be absent if remote deleted the entry.

The classifier produces, per cell:
- a **merged cell** (the post-refresh value), and/or
- a **MergeConflict** if local and remote diverged irreconcilably.

Under `StrategyOurs`, a conflict's merged cell = local. Under `StrategyTheirs`, merged cell = remote. Under `StrategyAbort`, the conflict is reported and the engine aborts before producing any merged set.

---

## 4. Cell-level merge rules

The complete classification table. "any" means the at-pull-time value doesn't matter for that row.

| At pull time | Current local | Current remote | Outcome |
|---|---|---|---|
| any | unchanged from pull | unchanged from pull | **untouched** — pass through |
| any | unchanged from pull | **changed** | **adopt remote** — `Adopted++` |
| any | **changed** (edited or cleared) | unchanged from pull | **keep local** — `Preserved++` |
| any | **changed** | **changed**, same value | **converged** — `Resolved++`, accept (no conflict) |
| any | **changed** | **changed**, different value | **CONFLICT** |
| existed | unchanged (still has sourceEntryID, original hours) | **deleted on remote** | **stale source** — clear `sourceEntryID`, mark as `added`. (Phase A's reconcile already does this; refresh inherits behavior.) |
| existed | **edited** (hours/desc changed) | **deleted on remote** | **CONFLICT** |
| existed | **cleared** (hours=0, sourceEntryID set) | unchanged | **keep local** — user wants entry gone, refresh preserves intent |
| existed | **cleared** | **deleted on remote** | **resolved by reality** — local intent already satisfied; cell drops out of merged set silently |
| existed | **cleared** | **modified** | **CONFLICT** |
| didn't exist | added locally (no source ID, hours>0) | (still doesn't exist) | **keep local** — local-only addition, will Create on push |
| didn't exist | added locally | added with same target/type/billable by remote | **CONFLICT** |
| didn't exist | (cell absent locally) | added remotely | **adopt remote** — `Adopted++` |

### Row-level rules

- **New row on remote** (target/type/billable not present at pull time) → adopt as new row in the merged draft. All its cells get their sourceEntryIDs.
- **New local row** (no sourceEntryID on any cell — came from `new --from-template` or `new` blank or hand-added) → keep local unchanged.

### Stable strategy semantics

- `StrategyOurs`: every CONFLICT row above resolves to local's value. No conflicts surface in the result.
- `StrategyTheirs`: every CONFLICT row above resolves to remote's value. No conflicts surface in the result.
- `StrategyAbort`: any CONFLICT row → engine aborts before producing a merged set.

---

## 5. CLI surface

`tdx time week refresh <date>[/<name>]`:

```
Flags:
  --profile <name>             (default: active)
  --strategy abort|ours|theirs (default: abort)
  --json                        structured output
```

`tdx time week rebase <date>[/<name>]` — alias of `refresh`. Identical behavior, identical flags. Registered as a separate cobra subcommand pointing at the same `runRefresh` function.

### Default-strategy text output (success path)

```
$ tdx time week refresh 2026-04-26
Refresh complete.
  Adopted (remote -> draft):  3 cells
  Preserved (local edits):    5 cells
  Resolved (same on both):    1 cell
```

### Abort path (default strategy, conflicts found)

```
$ tdx time week refresh 2026-04-26
Refresh aborted: 2 cells conflict between local edits and remote changes.

  row-01  Mon  conflict
    local:   updated to 6.0h
    remote:  updated to 8.0h

  row-02  Wed  conflict
    local:   cleared (delete on push)
    remote:  description changed to "Project review"

Choose one:
  --strategy ours        (keep local for both conflicts; refresh succeeds)
  --strategy theirs      (take remote for both conflicts; refresh succeeds)
  tdx time week reset 2026-04-26 --yes  (give up local edits entirely, re-pull fresh)
```

Exit code: 1 on abort, 0 on success.

### Strategy-specified path (ours or theirs, succeeds)

```
$ tdx time week refresh 2026-04-26 --strategy ours
Refresh complete (--strategy ours).
  Adopted (remote -> draft):  3 cells
  Preserved (local edits):    5 cells
  Resolved (same on both):    1 cell
  Resolved by --strategy:     2 cells (local won)
```

The "Resolved by --strategy" counter maps to `RefreshResult.ResolvedByStrategy`. `--strategy theirs` symmetrical: "Resolved by --strategy: N cells (remote won)".

### `--json` output shape

```json
{
  "schema": "tdx.v1.weekDraftRefreshResult",
  "strategy": "abort",
  "aborted": true,
  "adopted": 0,
  "preserved": 0,
  "resolved": 0,
  "resolvedByStrategy": 0,
  "conflicts": [
    {
      "row": "row-01",
      "day": "Monday",
      "local": "updated to 6.0h",
      "remote": "updated to 8.0h"
    }
  ]
}
```

On success: `aborted: false`, `conflicts: []`, count fields populated. Under `--strategy ours` / `theirs`, `resolvedByStrategy` reports how many real conflicts the strategy collapsed.

---

## 6. MCP

One new mutating tool: `refresh_week_draft`. Tool count 37 → 38.

```go
type refreshDraftArgs struct {
    Profile   string `json:"profile,omitempty"`
    WeekStart string `json:"weekStart"`
    Name      string `json:"name,omitempty"`
    Strategy  string `json:"strategy,omitempty" jsonschema:"abort | ours | theirs (default abort)"`
    Confirm   bool   `json:"confirm"`
}
```

Description text:

```
Refresh a draft against the current remote. Three-way merge between
at-pull-time / current-local / current-remote.

strategy: abort (default) - refuse to mutate if any cell-level conflict
          ours              - on conflict, keep local
          theirs            - on conflict, take remote

On strategy=abort with conflicts, returns a successful tool result with
aborted=true and a conflicts[] list. Agent can re-call with strategy=ours
or strategy=theirs after surfacing the conflicts to the user.

Requires confirm=true.
```

Returns `tdx.v1.weekDraftRefreshResult`. Mutating but distinct from the existing draft-mutating tools in two ways:

1. The "refused to mutate" success case (`aborted: true`) is a normal return, not an error. Agents need to inspect `aborted` and `conflicts` to know whether further action is needed.
2. `--strategy abort` produces no observable disk side effects when it aborts.

---

## 7. JSON schemas

- **New:** `tdx.v1.weekDraftRefreshResult` — see §5 for shape.
- Existing `tdx.v1.weekDraftSnapshotList` (used by `tdx time week history`) unchanged; the pre-refresh snapshot just shows up as a new entry.

---

## 8. Docs deliverables

1. **`README.md`:** add `tdx time week refresh / rebase --strategy` row to the Time Week Drafts table; expand MCP table with `refresh_week_draft`; add `tdx.v1.weekDraftRefreshResult` to the schema list.
2. **`docs/guide.md`:** new "Refresh & rebase" subsection inside the "Week drafts" section. Cover:
   - The three-way merge model (at-pull-time / current-local / current-remote).
   - Strategy semantics with concrete examples.
   - Abort behavior + recovery (retry with `--strategy ours/theirs`, or `reset --yes`).
   - Worked example: pull → edit Mon to 6h → meanwhile remote changed Mon to 8h → refresh aborts → `--strategy ours` to keep 6h.
3. **`docs/manual-tests/phase-B2a-week-drafts-refresh-walkthrough.md`** — runnable end-to-end against UFL tenant. Exercises:
   - Refresh with no remote changes (no-op success).
   - Refresh with local edits + non-conflicting remote changes (auto-merge, all three counters non-zero).
   - Refresh with conflicts under `--strategy abort` (verify error + zero disk mutation + `tdx time week status` unchanged).
   - Same conflicts under `--strategy ours` (verify local won).
   - Same conflicts under `--strategy theirs` (verify remote won).
   - `tdx time week history` shows pre-refresh snapshots from the ours/theirs runs.
4. **Inline `--help`** for `refresh`, `rebase`, and `--strategy` flag.
5. **MCP tool description** carries the strategy semantics inline.

---

## 9. Recommended slice + sequence

B.2a is one phase. Estimated **~15 tasks**, ~20 commits. Suggested order:

1. Three-way merge classifier (pure function) + comprehensive table-driven tests covering every row of §4.
2. `MergeConflict` + `RefreshResult` types + `Strategy` enum.
3. `Service.Refresh` with `StrategyAbort` (success-path classifier wiring + the "no conflicts" merged set).
4. `StrategyAbort` conflict-abort path: returns conflicts, mutates nothing.
5. `StrategyOurs` / `StrategyTheirs` resolution paths — engine collapses conflicts deterministically.
6. Pre-refresh snapshot integration (`OpPreRefresh`).
7. Watermark write on success (`.pulled.yaml` rewrite to current remote).
8. Edge case: stale source ID (cell points at remotely-deleted entry).
9. Edge case: new local rows / new remote rows.
10. `tdx time week refresh` CLI (default-strategy, ours, theirs paths).
11. `tdx time week rebase` alias registration.
12. `--json` output for refresh result.
13. MCP `refresh_week_draft` tool + confirm-gate test + `wantCount` bump.
14. README + guide.md + walkthrough doc.
15. Final verification + tag v0.6.0.

(Task count at writing-plans time may compress as related steps merge.)

---

## 10. Open questions and risks

### Risks

- **The merge engine is the densest piece of code in the whole iteration.** Cell-level state classification has many edge cases. Test matrix needs to be exhaustive — every row of §4's table needs at least one explicit test.
- **Watermark consistency.** After a successful refresh, `.pulled.yaml` MUST reflect the post-refresh remote state, not the pre-refresh remote. Otherwise next refresh thinks pristine cells are conflicts. The walkthrough should explicitly verify this with two consecutive refreshes (second one is a no-op).
- **Pre-refresh snapshot timing.** Snapshot fires before any disk write. If the merge math has a bug that corrupts the draft, `tdx time week restore --snapshot N` is the recovery path.

### Open questions deferred to B.2b

- The `surface` strategy: produce partial-merge + `conflicted` state.
- A `tdx time week resolve <date>` command for cell-by-cell pick.
- In-editor conflict marker rendering.
- TTY-interactive prompt mode.
- Whether `refresh` without a strategy should default-abort vs default-prompt (currently locked to default-abort; reopen if real usage shows this is wrong).

---

## 11. After B.2a: what B.2b will need

B.2b will reuse B.2a's pure merge classifier as-is. The classifier already returns `(merged_cells, conflicts)`. B.2b's additions:

- A new `Strategy = "surface"` value that uses the existing `(merged, conflicts)` output to produce a partial-merge draft with the conflicts encoded somehow (the design choice between in-YAML markers and a sidecar conflicts file is B.2b's to make).
- Production of `SyncConflicted` state and `CellConflict` cell state.
- `tdx time week resolve <date>` CLI for cell-by-cell pick + the corresponding `resolve_week_draft_cell` MCP tool.
- Updated push, status, list to handle the conflicted state correctly (push refuses, status/list show it).

None of this requires changes to B.2a's engine. The engine produces what both phases need.

---

## 12. Summary

B.2a ships in one focused PR: three-way merge engine + `refresh` / `rebase` CLI + three deterministic strategies + `refresh_week_draft` MCP tool. Engine is pure. Drafts always end up in a clean (non-conflicted) state. ~14 tasks, ~20 commits. Tag v0.6.0 on completion. B.2b (`surface` strategy + `conflicted` state machine + `resolve` UX) follows in a separate cycle once B.2a has shipped.
