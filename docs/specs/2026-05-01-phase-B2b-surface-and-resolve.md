# Phase B.2b — `surface` strategy + `tdx time week resolve`

**Date:** 2026-05-01
**Status:** Approved (brainstorming complete)
**Owner:** ipm
**Type:** Phase B.2b — completes the original B.2 design (surface partial-merge + cell-by-cell resolution)
**Target tag:** v0.11.0 (minor — new top-level CLI verb, new draft schema field)

---

## 0. Decisions log

| # | Decision |
|---|---|
| Q1 | Ship the full B.2b: surface strategy, push refusal, status/show updates, and a flag-driven `resolve` command. Skip the interactive TUI for now. |
| Q2 | Conflict encoding lives **inside the cell**: a new `Conflict *DraftConflictAlt` field on `DraftCell`. The main cell holds **local** intent; the alt holds **remote**. A cell is conflicted iff `Conflict != nil`. YAML reads naturally; `nil` serializes as the field being absent. |
| Q3 | `resolve` UX is flag-driven CLI (option B from brainstorm). Forms: bare → status, `--all-local`/`--all-remote` → bulk, `--row/--day/--pick` → per-cell. Repeatable; user can chip away at conflicts then sweep the rest. No interactive TUI. |
| Q4 | `ComputeSyncState` becomes the single source of truth for `SyncConflicted`: walk cells, bump `Conflict` counter when `cell.Conflict != nil`, set Sync. The `CellConflict` enum becomes a status the cell-state diff returns when it sees `Conflict != nil` on the current cell. |
| Q5 | `--strategy surface` is the third "live" strategy alongside `abort`/`ours`/`theirs`. Under surface, the engine **never aborts** — every conflict becomes a surfaced cell. New `RefreshResult.Surfaced` counter. |
| Q6 | `push` refuses if `SyncConflicted`. Error message points to `tdx time week resolve <date>`. |
| Q7 | MCP gets two changes: `refresh_week_draft` accepts `surface`; new `resolve_week_draft` tool covers all three CLI forms. |

---

## 1. Goal

Close the open B.2 design promise: when `tdx time week refresh` finds a real cell-level conflict, allow the user to land in a state where **both candidates are visible in the draft** and pick winners cell-by-cell, instead of choosing only between aborting or accepting one whole side (`--strategy ours`/`theirs`).

This finally wires up the `CellConflict` and `SyncConflicted` enum values that have been declared-but-unproduced since Phase A.

---

## 2. Surface

### CLI

- `tdx time week refresh <date>[/<name>] --strategy surface` → produces a partial-merge draft with conflicted cells. Always exits 0; never aborts.
- `tdx time week resolve <date>[/<name>]` → new command.
- `tdx time week push <date>[/<name>]` → refuses if conflicted.
- `tdx time week show <date>[/<name>]` → prints a CONFLICTS footer section.
- `tdx time week status <date>[/<name>]` → recommendation updates to point at `resolve`.

### MCP

- `refresh_week_draft.strategy` accepts `"surface"`.
- New tool `resolve_week_draft` with inputs:
  ```json
  {
    "profile": "default",
    "weekStart": "YYYY-MM-DD",
    "name": "default",
    "pickAllLocal": false,
    "pickAllRemote": false,
    "picks": [{"rowID": "abc", "day": "Monday", "choice": "local|remote"}]
  }
  ```

### Schema additions

- `tdx.v1.weekDraftConflicts` — JSON envelope for `tdx time week resolve --json` (no apply). Lists every unresolved conflict with rowID, day, local hours, remote hours, pulled hours.
- `tdx.v1.weekDraftResolveResult` — envelope for apply runs (`--all-X`/`--row ... --pick X`/MCP). Reports counts: `picksApplied`, `conflictsRemaining`, `pickedLocal`, `pickedRemote`.

---

## 3. Domain changes

### 3.1 `internal/domain/draft.go`

```go
type DraftCell struct {
	Day           time.Weekday      `yaml:"day" json:"day"`
	Hours         float64           `yaml:"hours" json:"hours"`
	SourceEntryID int               `yaml:"sourceEntryID,omitempty" json:"sourceEntryID,omitempty"`
	PerCell       *PerCell          `yaml:"perCell,omitempty" json:"perCell,omitempty"`
	Conflict      *DraftConflictAlt `yaml:"conflict,omitempty" json:"conflict,omitempty"` // NEW
}

// DraftConflictAlt is the "other side" candidate when a refresh under
// --strategy surface couldn't auto-merge. The owning cell's main fields
// hold the local intent; this struct holds the remote alternative.
//
// PulledHours is the value at last pull time (if any). Useful context for
// resolve's status output and the show command's CONFLICTS footer; not
// required for the resolve mechanics.
type DraftConflictAlt struct {
	Hours         float64 `yaml:"hours" json:"hours"`
	SourceEntryID int     `yaml:"sourceEntryID,omitempty" json:"sourceEntryID,omitempty"`
	PulledHours   float64 `yaml:"pulledHours,omitempty" json:"pulledHours,omitempty"`
}
```

`Conflict == nil` is the unconflicted default. Existing drafts on disk parse correctly (field absent = nil).

### 3.2 `ComputeCellState` and `ComputeSyncState`

Extend `ComputeCellState` to return `CellConflict` when `current.Conflict != nil` (regardless of pulled diff — the conflict trumps "edited"/"untouched" because it represents an unresolved merge state).

`ComputeSyncState` already promotes any `CellConflict` to `SyncConflicted`. No change needed there beyond ensuring it sees the new state correctly.

### 3.3 Validation

`WeekDraft.Validate` doesn't need to change — `DraftConflictAlt` is just data. Tests confirm round-trip serialization.

---

## 4. Engine changes

### 4.1 `internal/svc/draftsvc/refresh.go`

Add the new strategy:

```go
const (
	StrategyAbort   Strategy = "abort"
	StrategyOurs    Strategy = "ours"
	StrategyTheirs  Strategy = "theirs"
	StrategySurface Strategy = "surface" // NEW
)
```

`Validate()` accepts surface. New outcome:

```go
const (
	// ... existing outcomes ...
	outcomeSurfaced // conflict surfaced as a cell with Conflict != nil
)
```

`makeConflict` extension — under StrategySurface, build the merged cell with local's main fields and `Conflict: &alt{Hours: remote.Hours, SourceEntryID: remote.SourceEntryID, PulledHours: pulled.Hours}`:

```go
case StrategySurface:
	// Build merged cell from local's value (main fields) + remote alt.
	var merged domain.DraftCell
	if local != nil {
		merged = *local
	}
	merged.Conflict = &domain.DraftConflictAlt{
		Hours:         remoteHours(remote),
		SourceEntryID: remoteSourceID(remote),
		PulledHours:   pulledHours(pulled),
	}
	return cellClassification{outcome: outcomeSurfaced, merged: &merged}
```

Helpers `remoteHours`/`remoteSourceID`/`pulledHours` return zero values for nil pointers.

`RefreshResult.Surfaced int` — new counter. Populated from `rowCounts.surfaced`.

### 4.2 Conflict cases under surface

The cases that currently `makeConflict` are:

| Case | Pulled | Local | Remote |
|---|---|---|---|
| All three present, all three diverge | exists | edited | edited |
| Local cleared (delete), remote modified | exists | hours=0 srcID set | edited |
| Local edited, remote deleted | exists | edited | absent |
| Both sides added independently with different hours | absent | exists | exists |

All four route to the same surface logic — encode local in main, remote (or its absence) in `Conflict`.

Edge case: when remote is `absent` (case 3), `Conflict.Hours = 0` and `Conflict.SourceEntryID = 0` represents "deleted on remote." The resolve UX reads this as "the remote candidate is to delete the cell" and `--pick remote` removes the cell entirely (drops it out of the row). UI shows `(deleted on remote)` instead of a numeric value.

### 4.3 `Refresh()` flow under surface

```go
res := classify(pulled, draft, remoteDraft, strategy)

// Surface NEVER aborts. The merged set always lands; conflicts are
// embedded in cells as Conflict alternates.
if res.aborted { ... }  // unchanged abort path stays for StrategyAbort

// Continue with merged save/snapshot — no change.
return RefreshResult{
	Strategy:           strategy,
	Adopted:            res.counts.adopted,
	Preserved:          res.counts.preserved,
	Resolved:           res.counts.resolved,
	ResolvedByStrategy: res.counts.resolvedByStrategy,
	Surfaced:           res.counts.surfaced, // NEW
}, nil
```

Snapshot label: existing `OpPreRefresh` covers it.

---

## 5. CLI: `tdx time week resolve`

### 5.1 Layout

```
tdx time week resolve <date>[/<name>] [flags]
```

Bare invocation (no flags): prints status of unresolved conflicts.

```
WEEK <date>  conflicts: 3
ROW                              DAY     LOCAL          REMOTE         PULLED
proj-1234:billable               Monday  8.0            10.0           7.0
proj-1234:billable               Tuesday 6.0            (deleted)      5.0
maint:non-billable               Wednes  (cleared)      4.0            8.0

Pick:
  tdx time week resolve <date> --row proj-1234:billable --day Monday --pick remote
  tdx time week resolve <date> --all-local
  tdx time week resolve <date> --all-remote
```

### 5.2 Flags

| Flag | Purpose |
|---|---|
| `--row ID` | Target row for a per-cell pick. Required with `--day`/`--pick`. |
| `--day NAME` | `Sunday`/`Monday`/.../`Saturday`. Required with `--row`/`--pick`. |
| `--pick local\|remote` | Required with `--row`/`--day`. |
| `--all-local` | Resolve every conflict by keeping local. Mutex with `--all-remote` and `--row/--day/--pick`. |
| `--all-remote` | Resolve every conflict by taking remote. Mutex same as above. |
| `--json` | Switch text → JSON envelope. Works for both status (no apply) and apply runs. |
| `--profile NAME` | Standard. |
| `--yes` | Required if any pick would delete a cell (i.e. `--pick remote` on a conflict where Conflict.Hours=0 / SourceEntryID=0 — "deleted on remote"). Mirrors the existing `--allow-deletes`/`--yes` pattern. Per-pick check. |

### 5.3 Validation rules

- `--row`/`--day`/`--pick` are a triple — all three required together; none allowed alone.
- `--all-local` and `--all-remote` are mutually exclusive with each other and with the `--row/--day/--pick` triple.
- `--pick` value: `local` or `remote` (case-insensitive).
- `--day` value: case-insensitive weekday name.
- If no draft exists → "no draft for date X (use `tdx time week pull` first)".
- If draft exists but is not conflicted → exits 0 with `"no conflicts to resolve"`.

### 5.4 Apply mechanics

- `--pick local`: clear `cell.Conflict` (the local value is already in the main fields).
- `--pick remote`:
  - If `Conflict.Hours == 0 && Conflict.SourceEntryID == 0` → drop the cell entirely (remote's intent was "delete"); requires `--yes`.
  - Else → copy `Conflict.Hours` / `Conflict.SourceEntryID` into the cell, clear `Conflict`.

### 5.5 Snapshot

Take an `OpResolve` snapshot before save. Add `OpResolve = "resolve"` to the snapshot Op enum (or reuse `OpPreRefresh` with a different reason — but a distinct op is clearer in `tdx time week history`).

---

## 6. CLI updates to existing commands

### 6.1 `tdx time week refresh`

Update flag help to list `surface` alongside `abort|ours|theirs`. No code change beyond `Strategy.Validate()` accepting it.

### 6.2 `tdx time week push`

Pre-flight: load draft + watermark, compute `ComputeSyncState`, refuse if `Sync == SyncConflicted`:

```
draft has N unresolved conflicts; tdx time week resolve <date> to pick winners
```

Exit non-zero. No mutation.

### 6.3 `tdx time week status`

Update `recommendedAction`:

```go
case sync == domain.SyncConflicted:
	return "tdx time week resolve <date>"
```

Existing test `status_test.go` line 18 needs updating.

### 6.4 `tdx time week show`

Append a CONFLICTS section when `state.Sync == SyncConflicted`. Format:

```
CONFLICTS
ROW                DAY      LOCAL      REMOTE     PULLED
proj-1234:bill     Monday   8.0        10.0       7.0
...
```

### 6.5 `tdx time week list --conflicted` filter

Already exists (line 97 of list.go). Verify it now finds drafts produced by `--strategy surface`.

---

## 7. MCP

### 7.1 `refresh_week_draft`

Accept `"surface"` in the strategy enum. Update tool description.

### 7.2 New tool `resolve_week_draft`

```go
type resolveWeekDraftArgs struct {
	Profile       string `json:"profile,omitempty"`
	WeekStart     string `json:"weekStart"`
	Name          string `json:"name,omitempty"`
	PickAllLocal  bool   `json:"pickAllLocal,omitempty"`
	PickAllRemote bool   `json:"pickAllRemote,omitempty"`
	Picks         []picker `json:"picks,omitempty"`
}
type picker struct {
	RowID  string `json:"rowID"`
	Day    string `json:"day"`
	Choice string `json:"choice"` // "local"|"remote"
}
```

Same validation as the CLI. Exposes both status (no inputs beyond date → returns conflict envelope) and apply (any of the pick fields). Returns `tdx.v1.weekDraftResolveResult` envelope.

MCP count: 39 → 40.

---

## 8. Tests

### Domain

- `TestDraftConflictAlt_RoundTripYAML` — confirm a cell with `Conflict` round-trips through YAML and back.
- `TestComputeCellState_Conflict` — `ComputeCellState(pulled, current{Conflict: &x})` returns `CellConflict`.
- `TestComputeSyncState_PromotesToConflicted` — a draft with one conflicted cell yields Sync=SyncConflicted.

### Engine

- `TestClassify_SurfaceProducesSurfacedCells` — three-divergent case under surface emits a merged cell with Conflict populated; counters update.
- `TestClassify_SurfaceNeverAborts` — a draft with multiple conflicts, surface strategy returns `aborted=false` and full merged set.
- `TestClassify_SurfaceCases` — table-driven: each of the 4 conflict cases produces correctly-shaped surfaced cell.

### Service

- `TestRefresh_SurfaceWritesConflictedDraft` — end-to-end: pull, edit conflictingly, mock remote, refresh --strategy surface, assert draft on disk has Conflict set.

### Resolve CLI

- `TestResolve_BarePrintsConflictList`
- `TestResolve_AllLocalClearsAllConflicts`
- `TestResolve_AllRemoteCopiesAllRemotes`
- `TestResolve_PerCellPick_Local`
- `TestResolve_PerCellPick_Remote`
- `TestResolve_PickRemoteDeleteRequiresYes`
- `TestResolve_NoConflictsExitsCleanly`
- `TestResolve_FlagValidation_TripleRequired`
- `TestResolve_FlagValidation_Mutex`
- `TestResolve_JSONEnvelopeShape`

### Push refusal

- `TestPush_RefusesOnConflictedDraft` — assert exit non-zero, message points at resolve.

### Show

- `TestShow_ConflictsFooterAppears`

### MCP

- `TestRefresh_StrategySurface_TC` — MCP path
- `TestResolve_MCP_TableDriven`

---

## 9. Side-effect audit

| Concern | Result |
|---|---|
| Existing drafts on disk | Unaffected. `Conflict` field is `omitempty` and absent in old YAML; loaded as nil. |
| `tdx time week pull` | Already refuses when local has unpushed changes. A conflicted draft is "dirty enough" to block pull → no behavior change beyond the existing refusal. |
| `tdx time week refresh --strategy abort` (default) | Unchanged. Still aborts on conflicts. |
| `tdx time week refresh --strategy ours/theirs` | Unchanged. Still collapses conflicts inline. |
| Watermark behavior under surface | Same as ours/theirs: refresh writes the new pull-time watermark on success (success here = "merged set saved"). User runs `resolve` against the post-refresh draft + watermark. |
| Reconcile on push | Push refuses before reconcile under SyncConflicted; reconcile never sees a conflicted draft. |
| Editor (`tdx time week edit`, `tdx time template edit`) | The editor reads cells but doesn't display the Conflict field. A conflicted draft loaded into the editor shows local's hours; editing rewrites to draft, preserving Conflict (we don't strip it on save). User can't resolve via editor in this round — by design. Add a banner: "draft has N conflicts; tdx time week resolve to pick winners" before the editor opens. |
| `tdx time week show --json` | New output: `state.conflicts: [...]` array when SyncConflicted. Schema bump? No — additive field; existing consumers ignore. |
| Snapshot enum | New `OpResolve`. Existing `tdx time week history` lists snapshots by Op; new value just appears. |

---

## 10. Out of scope

- Interactive TUI for resolve. Layer on top later if demand shows.
- Showing conflicts in the grid editor. Two-value column is a real lift; defer.
- Multi-pick batch picker (`--pick rowA:Monday=local,rowB:Tuesday=remote`). YAGNI; users can repeat the command.
- Conflict markers in YAML for `<<<<<<< local / >>>>>>> remote` style. We have a typed `Conflict` field; no need for textual markers.
- Auto-resolve heuristics (e.g. "if local hours equals pulled, prefer remote"). Surface lets the user see and decide.

---

## 11. Estimated work

6 commits, target tag v0.11.0:

1. **Domain:** `DraftConflictAlt` field on `DraftCell`; extend `ComputeCellState` to return `CellConflict` on `Conflict != nil`; YAML round-trip + state tests.
2. **Engine:** `StrategySurface` constant + Validate; outcomeSurfaced + makeConflict surface branch; `RefreshResult.Surfaced` counter; classify tests.
3. **Refresh CLI + Push refusal + status/show updates:** wire `--strategy surface` flag help, refuse push on conflicted draft, update `recommendedAction`, add show CONFLICTS footer.
4. **Resolve CLI:** new `tdx time week resolve` command (status + bulk + per-cell + JSON envelope + delete-on-pick safety + tests).
5. **Snapshot Op + Editor banner:** new `OpResolve` snapshot reason; pre-edit banner for conflicted drafts.
6. **MCP + docs:** `refresh_week_draft` accepts surface; new `resolve_week_draft` tool; README + guide updates; live verification on the test tenant; PR; merge; tag v0.11.0.

Inline execution.
