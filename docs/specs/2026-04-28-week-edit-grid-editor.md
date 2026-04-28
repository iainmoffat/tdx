# Week edit uses the grid editor (parity with template edit)

**Date:** 2026-04-28
**Status:** Approved (brainstorming complete)
**Owner:** ipm
**Type:** UX consistency refactor (not a redesign)

---

## 0. Decisions log

| # | Decision |
|---|---|
| Q1 | Generalize the existing TUI grid editor (`internal/tui/editor`) and web editor (`internal/web/editor`) to operate on a small `Sheet` data type. Both Template and WeekDraft adapt to/from `Sheet`. |
| Q2 | `tdx time week edit` supports `--web` parity, mirroring `tdx time template edit --web`. |
| Q3 | Switching from YAML-text editing to the grid editor *removes* arbitrary row add/remove from `tdx time week edit`. The grid edits **hours only** within existing rows. Row-level operations (`new --from-template`, `set`, `copy`, `rename`) cover everything else. |
| Q4 | Adapters live in the CLI layer (`internal/cli/time/template/edit.go`, `internal/cli/time/week/edit.go`). Keeps `tui/editor` focused on the abstraction; keeps `tmplsvc`/`draftsvc` from depending on UI packages. |
| Q5 | Cell-mapping rules for draft → sheet → draft are **non-trivial** because draft cells carry `SourceEntryID` (delete-on-push semantics) and `PerCell` metadata. Spec'd in §8. |

---

## 1. Goal

Replace `tdx time week edit`'s YAML-in-`$EDITOR` flow with the grid editor already used by `tdx time template edit`. Behavior, navigation, validation, save/cancel, and `--web` flag should match templates.

The week-draft model and lifecycle are unchanged. Pull/diff/preview/push semantics are unchanged. Identity guards (profile/weekStart/name immutable) are unchanged. The only behavior change is the editing surface.

---

## 2. Affected files

### Modify

- `internal/tui/editor/model.go` — internal storage switches from `[]domain.TemplateRow` to `[]editor.SheetRow`; constructor `New(name string, rows []domain.TemplateRow)` becomes `New(sheet Sheet)`; `Rows() []domain.TemplateRow` becomes `Sheet() Sheet`.
- `internal/tui/editor/view.go` — render `SheetRow` fields instead of `TemplateRow`.
- `internal/tui/editor/model_test.go`, `internal/tui/editor/snap.go`, `internal/tui/editor/snap_test.go` — refactor to use `Sheet`.
- `internal/web/editor/server.go` — replace `tmpl domain.Template` with `sheet editor.Sheet`; replace `SaveFn` with `func(editor.Sheet) error`; `Run`'s signature changes.
- `internal/web/editor/server_test.go` — refactor.
- `internal/cli/time/template/edit.go` — adapter functions to convert `Template ↔ Sheet`; pass sheet to editor; map sheet back to template on save.
- `internal/cli/time/template/edit_test.go` — update if needed.
- `internal/cli/time/week/edit.go` — completely rewrite: load draft, build sheet from draft, run TUI or web editor, apply sheet back to draft (preserving `SourceEntryID`/`PerCell`), save draft. Drop YAML-in-EDITOR code path.
- `internal/cli/time/week/edit_test.go` — rewrite to test new grid behavior + draft↔sheet round-trip.
- `README.md` — update `tdx time week edit` row to mention grid editor + `--web` flag.
- `docs/guide.md` — update any narrative text about week edit being YAML in `$EDITOR`.

### Create

- `internal/tui/editor/sheet.go` — `Sheet` and `SheetRow` types (no logic, just structs).

### No change

- `internal/web/editor/static/*` — the existing HTML/JS already consumes a flat JSON shape (rows with id/label/group/typeName/hours). The wire format is essentially Sheet-shaped already.
- `internal/svc/draftsvc/*` — domain logic untouched.
- `internal/svc/tmplsvc/*` — domain logic untouched.
- MCP tools — no changes (week edit is CLI-only; MCP `update_week_draft` already takes structured cell-edit args).

---

## 3. Sheet abstraction

New file: `internal/tui/editor/sheet.go`.

```go
package editor

import "github.com/iainmoffat/tdx/internal/domain"

// Sheet is the editor's view of a hours-by-day grid. The TUI and web
// editors both operate on Sheet; adapters convert from Template, WeekDraft,
// or any other source.
//
// Sheet does not know what it represents. The Name field is the editor
// title. Rows are presented in display order (caller-determined).
type Sheet struct {
	Name string
	Rows []SheetRow
}

// SheetRow is one row of a Sheet — typically one (target, type, billable)
// triple, but the editor doesn't care about identity beyond the ID.
type SheetRow struct {
	ID         string            // stable identifier (template row ID, draft row ID)
	Label      string            // primary display label
	GroupName  string            // optional group header (templates use Target.GroupName)
	DisplayRef string            // fallback display when Label is empty
	TypeName   string            // optional time-type sub-label
	Hours      domain.WeekHours  // 7 floats, one per weekday
}
```

`Sheet` is pure data. It carries the only domain dependency (`WeekHours`), which is a leaf type with no further dependencies. The editor reads/writes `Hours.SetDay`/`Hours.ForDay`.

---

## 4. TUI editor refactor

**Constructor:**

```go
// Before
func New(name string, rows []domain.TemplateRow) Model

// After
func New(sheet Sheet) Model
```

**Internal storage:** `Model.rows` becomes `[]SheetRow`. `Model.original` stays `[]domain.WeekHours` (still used for dirty-tracking).

**Sort order:** the existing sort by `Target.GroupName` then `Label` moves into the adapter (caller-determined display order). The editor stops sorting.

**Accessor:**

```go
// Before
func (m Model) Rows() []domain.TemplateRow

// After
func (m Model) Sheet() Sheet
```

**View:** `view.go` reads `r.GroupName`, `r.Label`, `r.DisplayRef`, `r.TypeName`, `r.Hours.ForDay(d)`. The current view.go already uses these fields via `TemplateRow.Target.GroupName` etc. — straightforward rename.

**Behavior:** unchanged. Same key bindings, same dirty/confirm/typing model, same snap-to-half on numeric input.

---

## 5. Web editor refactor

`internal/web/editor/server.go`:

**Run signature:**

```go
// Before
func Run(tmpl domain.Template, save SaveFn) (Result, error)
type SaveFn func(domain.Template) error

// After
func Run(sheet editor.Sheet, save SaveFn) (Result, error)
type SaveFn func(editor.Sheet) error
```

`server` struct switches from `tmpl domain.Template` to `sheet editor.Sheet`. `toResponse()` reads `sheet.Name` and `sheet.Rows[].Hours/Label/GroupName/DisplayRef/TypeName`. `handleSave()` decodes the same `saveRequest` shape (rows by ID with hours), updates `sheet.Rows[i].Hours`, calls `save(sheet)`.

The HTML/JS in `static/` is unchanged — it already consumes a flat row-with-hours JSON shape.

---

## 6. Template edit.go update

Add small adapter functions:

```go
// templateToSheet builds a Sheet from a template, with rows in display order
// (grouped by Target.GroupName, then Label).
func templateToSheet(t domain.Template) editor.Sheet { ... }

// applySheetToTemplate copies hours from sheet rows back into t.Rows
// (matching by row ID). Mutates t in place.
func applySheetToTemplate(sheet editor.Sheet, t *domain.Template) { ... }
```

`runTUIEditor` builds the sheet, runs the editor, applies the result back to a copy of the template, saves. Same for `runWebEditor`.

The display-order sort that previously lived in `editor.New` moves into `templateToSheet`.

---

## 7. Week edit.go rewrite

The new `tdx time week edit [date[/name]]`:

```go
func newEditCmd() *cobra.Command {
    var (
        webFlag     bool
        profileFlag string
    )

    cmd := &cobra.Command{
        Use:   "edit [date[/name]]",
        Short: "Edit a draft in an interactive grid (defaults to the current week)",
        Long: `Edit a draft's hours in an interactive grid (the same editor used by ` +
              "`tdx time template edit`" + `).

Use --web to open the editor in your browser instead of the terminal.

Only hours within existing rows can be edited. To add or remove rows, use
` + "`tdx time week new --from-template`" + ` or ` + "`tdx time week set`" + `.`,
        Args: cobra.MaximumNArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error { ... },
    }
    cmd.Flags().BoolVar(&webFlag, "web", false, "open the editor in your browser")
    cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
    return cmd
}
```

`runEdit` flow:
1. Resolve week ref via `ResolveWeekRef`.
2. Load draft via `drafts.Store().Load(profile, weekStart, name)`.
3. Build sheet via `draftToSheet(draft)`.
4. Run TUI editor or web editor. Drop out early if not saved.
5. Apply sheet back via `applySheetToDraft(sheet, &draft)`.
6. `draft.ModifiedAt = time.Now().UTC()`.
7. `drafts.Store().Save(draft)`.

Identity guards (profile/weekStart/name) are unaffected — none of those fields are exposed in the sheet.

---

## 8. Cell-mapping rules (draft ↔ sheet)

This is the non-trivial part. Drafts have **sparse cells** with metadata; the sheet has **dense WeekHours**. Round-tripping must preserve `SourceEntryID` (drives push delete-vs-update behavior) and `PerCell` metadata.

### `draftToSheet(draft) → Sheet`

For each `DraftRow`, produce a `SheetRow`:
- `ID` = row.ID
- `Label` = row.Label, or row.Target.DisplayRef if Label is empty
- `GroupName` = row.Target.GroupName
- `DisplayRef` = row.Target.DisplayRef
- `TypeName` = row.TimeType.Name
- `Hours` = a `WeekHours` where `Hours.ForDay(d)` = the cell.Hours of the cell with Day=d, or 0 if no cell exists for that day

Rows are returned in display order (group, then label) — same as `templateToSheet`.

### `applySheetToDraft(sheet, &draft)`

For each `SheetRow`, find the matching `DraftRow` by `ID` (rows count and IDs are unchanged from `draftToSheet` — the editor cannot add/remove rows). For each weekday `d ∈ Sun..Sat`:

| Cell exists? | Has SourceEntryID? | Cell.Hours | Sheet hours | Outcome |
|---|---|---|---|---|
| no | – | – | 0 | no-op (don't add zero cell) |
| no | – | – | >0 | **add** new cell `{Day=d, Hours=h, SourceEntryID=0}` |
| yes | yes | any | 0 | **set Hours=0** (preserves SourceEntryID; push will issue ActionDelete) |
| yes | yes | any | >0 | **set Hours=h** (preserves SourceEntryID + PerCell) |
| yes | no | any | 0 | **drop the cell** (was a local-only addition, now deleted) |
| yes | no | any | >0 | **set Hours=h** (preserves PerCell) |

After applying, sort cells in each row by `Day` for canonical order (matches `pull.go`'s convention).

### Edge cases

- **Float comparison:** "did this change" is detected by the editor's dirty flag, not by the adapter. The adapter just writes the values it receives.
- **PerCell metadata:** preserved untouched. The editor can't see or modify `PerCell`.
- **Row-level fields (Description, ResolverHints, etc.):** the editor doesn't see them. They're preserved on the original DraftRow.
- **Empty draft (zero rows):** sheet has zero rows. Editor renders an empty grid; user can save (no-op) or cancel. No special-casing needed.

---

## 9. Tests

### TUI editor (`internal/tui/editor`)

- `model_test.go` — refactor existing tests to use `Sheet` constructor + `Sheet()` accessor. Tests cover navigation, typing, dirty, confirm. Substantively unchanged behavior.

### Web editor (`internal/web/editor`)

- `server_test.go` — refactor handlers tests for sheet input/output. The wire format JSON is unchanged.

### Template edit (`internal/cli/time/template/edit_test.go`)

- Existing test continues to pass (template edit's external behavior is unchanged).
- Add a small unit test for `templateToSheet` ↔ `applySheetToTemplate` round-trip.

### Week edit (`internal/cli/time/week/edit_test.go`)

- Replace YAML-flow tests with adapter-focused tests:
  - `draftToSheet` produces correct rows in display order, dense WeekHours from sparse cells
  - `applySheetToDraft` covers each row of §8's table:
    - Add new cell when sheet has hours and draft has none
    - Preserve SourceEntryID when zeroing a pulled cell
    - Drop a cell entirely when zeroing a local-only addition
    - Update Hours preserving PerCell metadata
  - Smoke test: `newEditCmd()` registers `--web`, `--profile`, accepts zero/one args
- Drop tests that asserted YAML editor behavior.

### Smoke verification

End-to-end: build the binary, run `tdx time week edit --help` — should mention "interactive grid", not "YAML in $EDITOR".

---

## 10. Docs

- **`README.md`** Time Week Drafts table: `tdx time week edit [date[/name]]` description: "Edit a draft in an interactive grid" with key flags `--web`, `--profile`.
- **`docs/guide.md`** "Week drafts → Editing" subsection: replace YAML-editing description with a brief note that the grid editor is the same as `tdx time template edit`.

---

## 11. Out of scope

- MCP changes — the existing `update_week_draft` tool is structured (per-cell edits) and orthogonal to interactive editing.
- New editor features (e.g., multi-select, bulk fill) — keep current keybindings + behavior.
- Row-level edits (add/delete/rename rows via editor) — explicitly removed; covered by other commands.
- Rename `internal/tui/editor` package — the package stays at its current path. Even though it now serves both templates and drafts, `editor` is still the right name.
- Per-cell metadata edits (Description, Billable override) — Phase C feature; not addressable from the grid editor.
- Web editor styling/UX changes — visual unchanged.

---

## 12. Estimated work

~7 tasks, ~10 commits:

1. Add `editor.Sheet` + `SheetRow` types
2. Refactor TUI editor model + view + tests to use Sheet
3. Refactor web editor server + tests to use Sheet
4. Update template edit.go with adapter + thread sheet through
5. Rewrite week edit.go (TUI + web paths, adapter, identity guards preserved)
6. Tests for week edit's adapter + smoke
7. README + guide.md + final verification + version bump v0.7.0 (minor — UX behavior change is user-visible)
