# Pull resolves a default TimeType for type-less placeholder rows

**Date:** 2026-04-29
**Status:** Approved (brainstorming complete)
**Owner:** ipm
**Type:** Bug fix follow-up to v0.7.2

---

## 0. Decisions log

| # | Decision |
|---|---|
| Q1 | TD's WeekReport returns one placeholder entry per loggable target. For project tasks (and most non-time-off targets), TD does NOT pre-assign a TimeType — those placeholders carry `TimeType.ID == 0`. The web UI handles this by popping a type picker when the user clicks an empty cell. |
| Q2 | We can't replicate TD's modal picker in the grid editor today. Instead, at pull time, look up valid time types per target via `TimeTypesForTarget` and assign the first valid type to each type-less placeholder row (Option C). User gets all rows visible, push works against any of them. If a user wants a non-default type for a specific cell, fallback is `tdx time entry add --type "..."` outside the draft flow. |
| Q3 | Revert v0.7.2's pull-side filter. Type-less placeholders re-appear, but post-process resolves them. |
| Q4 | Keep v0.7.2's reconcile guard as a safety net. If TimeType resolution fails (API error, target kind unsupported), the row stays at `TimeType.ID==0` and the guard refuses to push it with a clear error. |
| Q5 | Fix `componentPathFor` for `TargetProjectTask`. PlanID lives in `Target.ItemID`; TaskID in `Target.TaskID`; ProjectID in `Target.ProjectID`. URL format: `/TDWebApi/api/time/types/component/project/{ProjectID}/plan/{PlanID}/task/{TaskID}`. (Verified by analogy with the ticket-task path; if wrong, the call errors and graceful-degrades.) |

---

## 1. Goal

Pulled drafts should give the user the same row-set semantics as templates: every loggable target visible, every row has a valid TimeType so push works without surprises.

---

## 2. Design

### 2.1 Pull flow

```
Service.Pull():
  report := tsvc.GetWeekReport(...)
  draft := buildDraftFromReport(profile, name, report)        // unchanged
  s.resolveDefaultTimeTypes(ctx, profile, &draft)             // NEW
  s.dedupeRowsByKey(&draft)                                    // NEW (handles real-entry × placeholder collisions)
  store.Save(draft)
  store.SavePulledSnapshot(draft)
```

### 2.2 `resolveDefaultTimeTypes`

```go
// resolveDefaultTimeTypes assigns a default TimeType to each row whose
// TimeType.ID is 0, by querying TD's per-target time-type catalog and
// picking the first valid type. Best-effort: rows whose lookup fails
// keep TimeType.ID=0; the reconcile guard catches them at push time.
//
// Caches lookups by target identity within a single call so repeated
// targets only hit the API once.
func (s *Service) resolveDefaultTimeTypes(ctx context.Context, profile string, draft *domain.WeekDraft)
```

Body sketch:

```go
cache := map[string][]domain.TimeType{}
for i := range draft.Rows {
    row := &draft.Rows[i]
    if row.TimeType.ID > 0 {
        continue
    }
    key := targetCacheKey(row.Target)
    types, ok := cache[key]
    if !ok {
        t, err := s.tsvc.TimeTypesForTarget(ctx, profile, row.Target)
        if err != nil {
            cache[key] = nil  // negative-cache so we don't retry
            continue
        }
        cache[key] = t
        types = t
    }
    if len(types) == 0 {
        continue
    }
    row.TimeType = types[0]
}
```

### 2.3 Dedupe collisions

After resolution, two rows can share a `(Target, TimeType, Billable)` key:
- A real-entry row with `TimeType.ID=5` (existed before resolution)
- A placeholder row that resolution just promoted to `TimeType.ID=5`

The placeholder row has no cells; merge by dropping it.

```go
// dedupeRowsByKey collapses rows sharing (Target, TimeType, Billable).
// When two rows collide, the one with cells wins (the empty placeholder
// is dropped). Stable: input order is preserved among kept rows.
func (s *Service) dedupeRowsByKey(draft *domain.WeekDraft)
```

### 2.4 `componentPathFor` extension

```go
case domain.TargetProjectTask:
    return fmt.Sprintf("/TDWebApi/api/time/types/component/project/%d/plan/%d/task/%d",
        target.ProjectID, target.ItemID, target.TaskID), nil
```

(Replaces the current "ErrUnsupportedTargetKind" stub.)

### 2.5 Interface extension

`timeWriter` interface in `internal/svc/draftsvc/service.go` gains:

```go
TimeTypesForTarget(ctx context.Context, profile string, target domain.Target) ([]domain.TimeType, error)
```

`timesvc.Service` already implements this. The test mock (`mockTimeWriter` in `apply_test.go`) needs a `TimeTypesForTarget` implementation, default empty slice + nil error.

### 2.6 Refresh flow

`Service.Refresh` calls `buildDraftFromReport` to construct `remoteDraft` for the merge. Apply the same enrichment: call `resolveDefaultTimeTypes` and `dedupeRowsByKey` on `remoteDraft` before passing to `classify`. Otherwise the merge would see `TimeType.ID=0` rows on remote and produce conflicts against rows the user already enriched.

### 2.7 v0.7.2 changes status

- **Pull-side filter** (drop `TimeType.ID==0` placeholders): **REVERTED**. Replaced by resolution + dedup.
- **Reconcile guard** (refuse `ActionCreate` on `TimeType.ID==0` rows): **KEPT** as a safety net for cases where resolution fails (API error, unsupported target kind, etc).

---

## 3. Tests

In `internal/svc/draftsvc/`:

**New tests:**

- `TestPull_ResolvesDefaultTimeTypeForPlaceholders` (in `pull_test.go` or a new `service_test.go`): mock `TimeTypesForTarget` returns `[{ID: 5, Name: "Standard"}, {ID: 6, Name: "Training"}]` for a placeholder target; assert the resulting draft row has `TimeType.ID == 5`.

- `TestPull_KeepsTypeIDZeroIfLookupFails`: mock returns an error; the row keeps `TimeType.ID == 0`. Push-side guard handles it.

- `TestPull_DedupesPlaceholderCollidingWithRealEntry`: real entry on target X with TypeID=5 + placeholder on target X with TypeID=0 (resolution picks TypeID=5) → final draft has 1 row, not 2.

- `TestPull_CachesTimeTypesPerTarget`: two placeholder rows on the same target → mock's `TimeTypesForTarget` is called once.

**Updated tests:**

- Existing `TestBuildDraftFromReport_*` tests stay (they test the pure builder). Just remove `TestBuildDraftFromReport_SkipsPlaceholderWithZeroTypeID` (no longer applies).
- Existing `TestBuildDraftFromReport_KeepsPlaceholderWithValidTypeID` stays — placeholders WITH valid TypeIDs continue to produce empty rows directly.

In `internal/svc/timesvc/`:
- `TestComponentPathFor_ProjectTask`: assert the new URL format for project task targets.

---

## 4. Side-effect audit

| Concern | Result |
|---|---|
| Pull latency | One `TimeTypesForTarget` call per *unique* target with `TypeID=0`. Typical ~10–20 unique targets per week → ~1–2 sec extra latency on first pull. Cached within the call. Acceptable. |
| Refresh latency | Same as pull. Acceptable. |
| Dedupe correctness | Real-entry rows always have non-empty cells; placeholder rows have empty cells. When they collide on `(Target, TimeType, Billable)`, the cell-bearing row wins. |
| Network errors | Best-effort: if `TimeTypesForTarget` fails for a target, the row stays at `TypeID=0`. Push-side guard refuses to push it with the existing actionable error. User can manually fix or ignore. |
| `TargetProjectTask` URL | If my guess is wrong, the call returns an HTTP error → row stays at `TypeID=0` → push guard refuses. User sees actionable error pointing to `reset --yes`. No correctness damage; just no enrichment. Will verify the URL during live walkthrough. |
| MCP `pull_week_draft` | Same enrichment path (it calls Service.Pull). |
| MCP `update_week_draft` | Unaffected — works at the cell level on already-resolved rows. |
| Existing on-disk drafts | Unaffected. New behavior applies on next pull. v0.7.2 reset-and-repull recovery still works. |

---

## 5. Out of scope

- In-editor TimeType picker. The user would have to pick during edit. Future enhancement.
- Letting the user see/choose alternate TimeTypes per row at pull time (Option B from the brainstorm). Adds row count without obvious win for most cases.
- Resolving "ambiguous" cases — when `TimeTypesForTarget` returns multiple types, we always pick the first. If that turns out to be wrong frequently, we can sort the list (e.g., by frequency-of-use across past entries) in a follow-up.

---

## 6. Estimated work

~5 commits:
1. Extend `timeWriter` interface; update `mockTimeWriter`. Keep tests green.
2. Fix `componentPathFor` for `TargetProjectTask`.
3. Revert v0.7.2's pull-side filter; add `resolveDefaultTimeTypes` + `dedupeRowsByKey`; call them from `Service.Pull` and `Service.Refresh`.
4. New tests covering resolution, dedup, cache, error fallback.
5. Quality gate, push, PR, tag v0.8.0 (minor bump because pull semantics change meaningfully).

Inline execution. No subagent dispatch.
