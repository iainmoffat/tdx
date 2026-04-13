# tdx Template Editor — Design Spec

**Goal:** Add an interactive terminal-based grid editor for modifying
template hour values, invoked via `tdx time template edit <name>`.

**Milestone:** A user can open a template, navigate between day cells,
adjust hours, and save — without leaving the terminal or editing YAML.

---

## 1. Scope

### In scope

| Deliverable | Detail |
|---|---|
| `tdx time template edit <name>` | New Cobra command that launches an interactive TUI editor |
| Grid navigation | Arrow keys and Tab/Shift-Tab to move between hour cells |
| Value editing | Type a number (snapped to 0.5) or nudge with up/down arrows (0.5 increments) |
| Live totals | Row totals and day totals recompute on every edit |
| Save/cancel | Ctrl-S saves and exits; Esc cancels (with dirty prompt) |
| bubbletea dependency | Charm's TUI framework for the interactive model |
| Doc updates | Update README command table, user guide Templates section, and MCP tool reference if applicable |

### Out of scope

| Item | Reason |
|---|---|
| Row add/delete | Separate workflow; can be added later |
| Description editing | Rarely needed; edit YAML directly |
| Save-as (new name) | `tdx time template clone` already handles this |
| Inline target/type editing | Complex sub-workflow; out of scope for v1 |

---

## 2. Architecture

Three new files, one new dependency:

### Dependency

`github.com/charmbracelet/bubbletea` — the standard Go TUI framework.
Brings in `lipgloss` (styling) and `bubbles` (common components) transitively.

### Files

| File | Responsibility |
|---|---|
| `internal/tui/editor/model.go` | Bubbletea Model: grid state, cursor (row, col), edit buffer, dirty flag. Init/Update/View. |
| `internal/tui/editor/view.go` | Renders the grid string. Highlights active cell (reverse video). Status bar with key hints. |
| `internal/cli/time/template/edit.go` | Cobra command wiring. Loads template, creates model, runs bubbletea program, saves on clean exit. |

### Why this split

- `tui/editor/` is a self-contained bubbletea program with no Cobra or CLI dependencies. It receives a `[]domain.TemplateRow` and returns an edited copy (or nil on cancel).
- `cli/time/template/edit.go` handles the CLI plumbing: argument parsing, template loading, launching the TUI, and saving the result.
- `view.go` is separated from `model.go` because the View function will be the largest piece (grid layout logic). Keeping it in its own file keeps both files focused.

---

## 3. Interaction Model

### Navigation

| Key | Action |
|---|---|
| Left / Right | Move cursor between day columns (Sun–Sat) |
| Tab | Move to next cell (wraps to next row) |
| Shift-Tab | Move to previous cell (wraps to previous row) |
| Up (no edit active) | Move to row above |
| Down (no edit active) | Move to row below |

Cursor wraps: right from Sat goes to Sun of the next row; left from Sun
goes to Sat of the previous row. Up from first row stays; down from last
row stays.

### Editing

| Key | Action |
|---|---|
| Up (edit active or on cell) | Increment current cell by 0.5 (max 24.0) |
| Down (edit active or on cell) | Decrement current cell by 0.5 (min 0.0) |
| 0–9, `.` | Begin typing a value; replaces cell content |
| Enter | Confirm typed value (snap to nearest 0.5), advance to next cell |
| Backspace | Clear cell to 0.0 |

**Snap logic:** When the user types a value and presses Enter, the value
is rounded to the nearest 0.5. Examples: 1.3 → 1.5, 2.7 → 2.5, 3.0 → 3.0.
Values above 24.0 are clamped to 24.0. Negative values are clamped to 0.0.

**Edit mode detection:** When the user presses a digit or `.`, the editor
enters "typing mode" for that cell. The status bar shows the typed value.
Arrow up/down while in typing mode commits the typed value first, then
nudges. Pressing Esc while typing discards the typed value and reverts
to the previous value.

### Save / Cancel

| Key | Action |
|---|---|
| Ctrl-S | Save modified template and exit |
| Esc / q | Cancel — if dirty, show inline prompt "Unsaved changes. Quit? (y/n)"; if clean, exit immediately |

---

## 4. Grid Layout

The editor renders a grid matching the `template show` layout, with the
active cell highlighted via reverse video (terminal escape codes / lipgloss).

```
Editing: my-week [modified]

  ROW                            SUN   MON   TUE   WED   THU   FRI   SAT   TOTAL
──────────────────────────────────────────────────────────────────────────────────
  Administration (projectTask)   .    [5.0]  5.0   4.0   5.0   5.0   .     24.0
    └ Standard Activities
  Linux OS Platform (projTask)   .     1.0   1.0   1.0   1.0   1.0   .     5.0
    └ Standard Activities
──────────────────────────────────────────────────────────────────────────────────
  DAY TOTAL                      .     6.0   6.0   5.0   6.0   6.0   .     29.0

  ←→/Tab: move  ↑↓: adjust ±0.5  type: set value  Ctrl-S: save  Esc: cancel
```

**Active cell:** Rendered with reverse video (light text on dark background
or vice versa). When typing, the cell shows the input buffer instead of the
current value.

**Dirty indicator:** Title shows `[modified]` when any cell has been changed
from its original value.

**Totals:** Row TOTAL column and DAY TOTAL row recompute on every keystroke.
These are display-only — not navigable.

---

## 5. Data Flow

1. **Load:** `edit.go` loads the template by name from the template store.
   Errors if the template doesn't exist.

2. **Init:** Creates a `editor.Model` with a deep copy of the template rows.
   The original rows are kept for dirty comparison.

3. **Run:** `bubbletea.NewProgram(model).Run()` takes over the terminal.
   The model processes key events, updates cell values, and re-renders.

4. **Exit — Save (Ctrl-S):**
   - The model signals "save" via its return value.
   - `edit.go` copies the edited `WeekHours` back onto the template rows.
   - Updates `template.ModifiedAt` to `time.Now().UTC()`.
   - Saves via the template store.
   - Prints confirmation: `saved template "my-week"`.

5. **Exit — Cancel (Esc):**
   - If dirty: inline prompt "Unsaved changes. Quit? (y/n)".
     - `y`: exit without saving.
     - `n` / any other key: return to editing.
   - If clean: exit immediately.
   - No output on cancel.

---

## 6. Model State

```go
type Model struct {
    name     string              // template name (for title)
    rows     []domain.TemplateRow // editable copy
    original []domain.WeekHours  // snapshot for dirty detection
    cursor   cursor              // (row, col) — col 0=Sun..6=Sat
    typing   bool                // true when user is typing a value
    input    string              // typed digit buffer
    dirty    bool                // any cell changed from original
    quitting bool                // true after save or confirmed cancel
    saved    bool                // true if exiting via save
    confirm  bool                // true when showing quit confirmation
    width    int                 // terminal width (from WindowSizeMsg)
    height   int                 // terminal height
}

type cursor struct {
    row int // 0-based index into rows
    col int // 0=Sun, 1=Mon, ..., 6=Sat
}
```

---

## 7. Testing Strategy

### Unit tests (no TUI)

- **Snap logic:** Test that values round to nearest 0.5 correctly,
  clamp to [0, 24].
- **Dirty detection:** Modify a cell, verify dirty=true; revert, verify
  dirty=false.
- **Cursor navigation:** Verify wrap behavior (right from Sat → next row
  Sun, etc.), boundary clamping (up from row 0 stays).
- **Nudge:** Verify increment/decrement by 0.5 with clamping.

### Integration test

- Load a template, create a Model, send a sequence of key messages
  via `model.Update()`, verify the final row state matches expectations.
  No actual terminal needed — bubbletea models are testable via direct
  `Update()` calls.

---

## 8. Documentation Updates

Three files need updates:

### README.md

Add `tdx time template edit <name>` back to the Templates command table:

```
| `tdx time template edit <name>` | Edit template hours in a grid editor | |
```

### docs/guide.md

Add a new subsection under **Templates** (after "Show a template"):

**Edit a template**

Describe the interactive editor: `tdx time template edit <name>` opens a
grid where you navigate with arrow keys/Tab, adjust hours with up/down
(0.5 increments) or by typing values, save with Ctrl-S, cancel with Esc.
Include the keybinding table and a brief example of the workflow:
derive → edit → apply.

### docs/guide.md — Templates workflow intro

Update the workflow summary at the top of the Templates section from:

> 1. Derive, 2. Show/compare, 3. Apply

to:

> 1. Derive, 2. Edit (optional), 3. Show/compare, 4. Apply

---

## 9. Decision Log

| # | Decision | Rationale |
|---|---|---|
| D1 | bubbletea, not raw escape codes | Standard Go TUI framework; handles terminal setup, key parsing, resize, alt-screen. Rolling our own would be fragile. |
| D2 | Hours-only editing | Row structure (targets, types) is a separate concern. Keeps the editor simple. |
| D3 | 0.5 increment snap | Matches typical time entry granularity. Prevents impossible values (e.g. 1.333h → non-integer minutes). |
| D4 | Save in place only | `clone` command already exists for making copies. |
| D5 | Separate tui/editor package | Keeps TUI logic decoupled from CLI; testable without Cobra. |
| D6 | No reuse of render/grid.go | The static renderer writes to io.Writer; the TUI needs a string with embedded styling. Similar layout logic but different rendering approach. |
