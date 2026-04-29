# `tdx time week pull` keeps empty rows for editing

**Date:** 2026-04-28
**Status:** Approved (brainstorming complete)
**Owner:** ipm
**Type:** Bug fix (single-site behavioral change)

---

## 0. Decisions log

| # | Decision |
|---|---|
| Q1 | Pull keeps a row for every `(target, type, billable)` tuple TD's WeekReport advertises, even when the user has not logged any hours against it. Previously these "placeholder" entries (`id=0, minutes=0`) were silently dropped. |
| Q2 | Placeholder entries do NOT add a cell to their row. The row is created (so the editor sees it), but `Cells` stays empty/sparse. The user types into the grid → the editor adds a fresh `DraftCell{Day, Hours, SourceEntryID: 0}` per the existing v0.7.0 cell-mapping rules → push creates a new TD entry. |
| Q3 | Update `TestBuildDraftFromReport_FiltersZeroPlaceholders` to its inverse: `TestBuildDraftFromReport_KeepsPlaceholderRowsAsEmpty`. Add a collapse-test to confirm a placeholder + a real entry for the same key produce one row with one cell. |
| Q4 | No additional filtering (e.g., archived projects). TD already scopes the placeholder list to the current user; treat all of them as legitimate rows the user wants to see. If a need to filter emerges later, add it as a follow-up. |

---

## 1. Goal

Restore parity with `tdx time week show` and template editing: a pulled week draft contains the full row set TD considers available, not just rows where time has been logged. The grid editor introduced in v0.7.0 then renders all rows with editable zero cells, matching template-edit's mental model.

---

## 2. Change

Single site: `buildDraftFromReport` in `internal/svc/draftsvc/pull.go`.

Current code:
```go
for _, e := range report.Entries {
    if e.ID == 0 && e.Minutes == 0 {
        continue
    }
    // ... build rowGroupKey, create row, add cell
}
```

New code:
```go
for _, e := range report.Entries {
    isPlaceholder := e.ID == 0 && e.Minutes == 0

    k := rowGroupKey{ /* unchanged */ }
    row, ok := groups[k]
    if !ok {
        row = &domain.DraftRow{ /* unchanged */ }
        groups[k] = row
        order = append(order, k)
    }

    if isPlaceholder {
        continue   // row reserved; no cell to add
    }

    // ... existing cell-add code (DST-safe day calc, append DraftCell)
}
```

---

## 3. Test updates

In `internal/svc/draftsvc/pull_test.go`:

- **Invert** `TestBuildDraftFromReport_FiltersZeroPlaceholders`. Rename to `TestBuildDraftFromReport_KeepsPlaceholderRowsAsEmpty`. Assert that a placeholder produces a row with `len(Cells) == 0` and the row's Target/TimeType/Billable match the placeholder's.
- **Add** `TestBuildDraftFromReport_PlaceholderAndRealEntryCollapseToOneRow`: a placeholder followed by a real entry on the same key produces exactly one row with one cell.
- **Audit** `TestBuildDraftFromReport_GroupsByTargetTypeBillable`: its existing fixture has only real entries, so no change needed unless a placeholder fixture was implicitly relied on. Verify by reading the test before editing.

---

## 4. Side-effect audit

| Concern | Result |
|---|---|
| `.pulled.yaml` size | Grows by one row per available target. Negligible (~30 bytes/row). |
| Sync state counts | Empty rows have zero cells → contribute 0 to untouched/edited/added/conflict. Unchanged. |
| `list` / `status` totals | Total hours come from cells, not rows. Unchanged. |
| Refresh (B.2a `classify`) | Aligns rows by `(Target, TimeType, Billable)` key. Two empty rows on the same key classify as untouched — no work. New remote rows since pull → adopt. Correct. |
| Push reconcile | Empty rows have no cells → no actions. Once user edits, cells are added with `SourceEntryID=0` → ActionCreate. Correct. |
| Three-way merge edge cases | Same as refresh — adding extra empty rows doesn't introduce new outcomes. |

---

## 5. Out of scope

- Filtering placeholder rows (none today; deferred until a real need surfaces).
- Editor-side changes — the grid editor and adapters already handle empty rows correctly.
- Migrating existing on-disk drafts — pre-fix drafts are missing rows; users get the new behavior on next pull. Acceptable.

---

## 6. Estimated work

~3 tasks, ~3 commits:

1. Update `buildDraftFromReport` + invert/add tests.
2. Verify the full module is green; quality gate (`go test`, `vet`, `gofmt`, `golangci-lint`).
3. Push branch + open PR + tag v0.7.1 (patch — bug fix).
