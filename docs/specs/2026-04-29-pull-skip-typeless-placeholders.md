# Pull skips placeholders that have no TimeType, push validates row TypeID

**Date:** 2026-04-29
**Status:** Approved (brainstorming complete)
**Owner:** ipm
**Type:** Bug fix follow-up to v0.7.1

---

## 0. Decisions log

| # | Decision |
|---|---|
| Q1 | v0.7.1 made pull keep all TD placeholder entries (id=0, minutes=0) as empty editable rows. Some placeholders have `TimeType.ID == 0` — they represent group headers or targets without a default type assignment. New entries created against these rows fail with TD's "Time account 0 was not found" error. |
| Q2 | Skip placeholder entries with `TimeType.ID == 0` at pull time. Real entries (id > 0) always have a valid TimeType.ID, so they're unaffected. Placeholders WITH valid TimeType.ID continue to produce empty editable rows (the v0.7.1 fix's intent). |
| Q3 | Add a defensive guard in reconcile/preview: any cell becoming an `ActionCreate` against a row with `TimeType.ID == 0` produces a clear error before the API call. Catches future cases where bad data slips in via other paths. |
| Q4 | No data migration. Existing drafts with type-less rows on disk stay there until the user resets/re-pulls. The push-side guard prevents bad pushes regardless of when the draft was created. |

---

## 1. Goal

Restore push-correctness after v0.7.1: a draft created by `tdx time week pull` should be safe to edit and push without producing TD errors. Specifically:
- Pull only retains placeholder rows that can actually accept new entries.
- Push refuses early if a row is missing a TimeType (better error than the TD-side "Time account 0 was not found").

---

## 2. Root cause

`buildDraftFromReport` in `internal/svc/draftsvc/pull.go` (post-v0.7.1) reserves a row for every placeholder. The grouping key uses `e.TimeType.ID`, so multiple placeholders with the same target but different TimeType.IDs each produce their own row. Placeholders with `TimeType.ID == 0` collapse into a single per-target "type-less" row.

`reconcile.go:121-129` constructs `EntryInput` with `TimeTypeID: row.TimeType.ID`. When the user types hours into a type-less row, push issues an `ActionCreate` with `TimeTypeID=0` → TD returns the "Time account 0 was not found" error.

The user's report: 16 such failures across 10 distinct rows in week 2026-04-12.

---

## 3. Fix

### 3.1 Pull — skip type-less placeholders

In `buildDraftFromReport` (`internal/svc/draftsvc/pull.go`), extend the placeholder check:

Current:
```go
isPlaceholder := e.ID == 0 && e.Minutes == 0
```

New:
```go
isPlaceholder := e.ID == 0 && e.Minutes == 0
if isPlaceholder && e.TimeType.ID == 0 {
    continue   // unusable for entry creation; skip entirely
}
```

The existing post-row-create `if isPlaceholder { continue }` stays — placeholders WITH a valid TimeType.ID still produce empty editable rows.

### 3.2 Reconcile — refuse to create entries on type-less rows

In `reconcileDraft` (`internal/svc/draftsvc/reconcile.go`), before constructing `entryInput`:

```go
if cell.SourceEntryID == 0 && cell.Hours > 0 && row.TimeType.ID == 0 {
    return domain.ReconcileDiff{}, fmt.Errorf(
        "row %s has no TimeType (TimeType.ID=0); cannot create entries — "+
            "use `tdx time week reset %s --yes` and re-pull, or remove the row before push",
        row.ID, weekStart.Format("2006-01-02"))
}
```

This fires only for the create path. Update path (cell has SourceEntryID) is unaffected — those rows already had valid TimeType when their entry was logged. Delete path doesn't need TypeID either.

The error references the existing recovery path (`reset` + re-pull) so the user can self-serve.

---

## 4. Tests

In `internal/svc/draftsvc/pull_test.go`:
- **Add** `TestBuildDraftFromReport_SkipsPlaceholderWithZeroTypeID`: a placeholder with `e.TimeType.ID == 0` produces no row.
- **Add** `TestBuildDraftFromReport_KeepsPlaceholderWithValidTypeID`: a placeholder with `e.TimeType.ID == 5` (or similar non-zero) still produces an empty editable row (verifies v0.7.1's intent is preserved).

In `internal/svc/draftsvc/reconcile_test.go`:
- **Add** `TestReconcile_RefusesCreateOnTypelessRow`: a draft with a row having `TimeType.ID=0` and a cell with `Hours>0` and `SourceEntryID=0` produces an error from `Reconcile`. Verify the error mentions the row ID and the recovery hint.

---

## 5. Side-effect audit

| Concern | Result |
|---|---|
| Real entries with TimeType.ID=0 | Doesn't happen — real entries always have a valid TimeType. The `e.TimeType.ID == 0` check applies only when `isPlaceholder` is true. |
| Existing drafts with type-less rows on disk | Push refuses with the new guard. User runs `reset --yes` to re-pull cleanly. |
| Three-way refresh (B.2a) | `buildDraftFromReport` is called by refresh too. Type-less placeholders are dropped from the remote view, so they can't show up as "new remote rows". Existing draft rows stay; but if they had TimeType.ID=0, refresh's classifier still aligns by `(Target, TimeType, Billable)` — type-less rows align across pulled/local/remote consistently and produce no actions. Untouched. |
| Sync state, list, status | Type-less rows were rare to begin with and provided no value. Dropping them shrinks the row set slightly; counts and totals are unaffected. |
| MCP `update_week_draft` / `pull_week_draft` | Unaffected — they go through the same `Pull` path, get the same filtered rows. |

---

## 6. Out of scope

- Surfacing a "type picker" UI for type-less placeholders. Could be a future enhancement; for now, those rows just don't appear.
- Migrating existing drafts on disk. The recovery path (`reset --yes`) is well-documented.
- Diagnosing TD's behavior of returning type-less placeholders. We accept whatever TD returns and filter at the boundary.

---

## 7. Estimated work

~3 commits:
1. Pull fix + 2 new tests
2. Reconcile guard + 1 new test
3. Push branch, PR, tag v0.7.2

Single inline execution session. No subagent dispatch.
