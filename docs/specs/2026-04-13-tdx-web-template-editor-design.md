# tdx Web Template Editor — Design Spec

**Goal:** Add a browser-based grid editor for template hours, invoked via
`tdx time template edit --web <name>`. Provides a spreadsheet-like
experience for users who prefer a GUI over the TUI.

**Milestone:** A user runs the command, their browser opens with an
editable grid, they adjust hours, click Save, and the template is updated.

---

## 1. Scope

### In scope

| Deliverable | Detail |
|---|---|
| `--web` flag on `tdx time template edit` | Launches browser editor instead of TUI |
| Embedded HTTP server | Go stdlib `net/http`, random port via `:0`, auto-opens browser |
| `go:embed` HTML/CSS/JS | Single self-contained page embedded in the binary |
| Grouped grid layout | Projects as group headers, tasks indented underneath |
| Click-to-select, type-to-edit | Single click selects cell, typing enters edit mode immediately |
| Shift-click fill | Fill value from selected cell across the row |
| Live totals | Row totals and day totals update on every edit |
| 0.5 snap | Values snapped to nearest 0.5 on commit, clamped to [0, 24] |
| Save/Cancel buttons | Save POSTs data back, Cancel exits without saving |
| Auto-exit | Server shuts down after save or cancel |
| Doc updates | Update README, user guide, and `--web` flag help text |

### Out of scope

| Item | Reason |
|---|---|
| Row add/delete | Same constraint as TUI editor — not in v1 |
| Description editing | Same constraint as TUI editor |
| Persistent web server | Server is ephemeral — starts and exits per edit session |
| Authentication/multi-user | Local-only, single-user tool |
| WebSocket live sync | Not needed — single page, POST on save is sufficient |
| Frontend build step | No npm/node/React — vanilla HTML/CSS/JS via `go:embed` |

---

## 2. Architecture

### Files

| File | Responsibility |
|---|---|
| `internal/web/editor/editor.go` | HTTP server: serves HTML, handles `/save` and `/cancel` endpoints, opens browser |
| `internal/web/editor/static/editor.html` | Embedded HTML page with CSS and JS — the full grid editor UI |
| `internal/cli/time/template/edit.go` | Modified: adds `--web` flag, branches to web editor when set |

### How it works

```
tdx time template edit --web my-week
  │
  ├─ Load template from disk
  ├─ Start HTTP server on localhost:0 (random port)
  ├─ Open browser to http://localhost:{port}
  │
  │  Browser: GET /
  │  ← Server responds with editor.html + template data as JSON
  │
  │  User edits hours in the grid
  │
  │  Browser: POST /save  {rows: [{id, hours: {sun,mon,...}}]}
  │  ← Server writes template, responds 200, signals shutdown
  │  ── prints "saved template "my-week""
  │
  │  OR
  │
  │  Browser: POST /cancel
  │  ← Server responds 200, signals shutdown
  │  ── no output
  │
  └─ Server exits, CLI returns
```

### Dependency: browser open

Reuse the existing `openBrowser` function from `internal/cli/auth/browser.go`
(used by `--sso` login). No new dependencies needed.

---

## 3. HTTP Server

### Endpoints

| Method | Path | Request | Response | Action |
|---|---|---|---|---|
| GET | `/` | — | `editor.html` with template JSON injected | Serve the editor page |
| GET | `/api/template` | — | JSON: template data (rows, name, groups) | Template data for the editor |
| POST | `/api/save` | JSON: `{rows: [{id, hours: {sun,mon,...,sat}}]}` | `200 OK` | Write template to disk, signal shutdown |
| POST | `/api/cancel` | — | `200 OK` | Signal shutdown without saving |

### Template JSON shape (served to browser)

```json
{
  "name": "my-week",
  "rows": [
    {
      "id": "row-01",
      "label": "UFIT Administrative Activities: Administration",
      "group": "UFIT Administration",
      "typeName": "Standard Activities",
      "hours": {"sun": 0, "mon": 4, "tue": 4, "wed": 4, "thu": 4, "fri": 4, "sat": 0}
    }
  ]
}
```

### Save request JSON shape (from browser)

```json
{
  "rows": [
    {"id": "row-01", "hours": {"sun": 0, "mon": 4, "tue": 4, "wed": 4, "thu": 4, "fri": 4, "sat": 0}},
    {"id": "row-02", "hours": {"sun": 0, "mon": 0, "tue": 0, "wed": 0, "thu": 0, "fri": 0, "sat": 0}}
  ]
}
```

The server matches rows by `id` and updates `WeekHours` on the
corresponding `TemplateRow`. Unrecognized IDs are ignored. Missing IDs
keep their original values.

### Lifecycle

1. Server starts on `localhost:0` (OS-assigned port)
2. Port is extracted from the listener address
3. Browser is opened via `openBrowser`
4. Server blocks on a shutdown channel
5. `/api/save` or `/api/cancel` sends to the shutdown channel
6. Server calls `httpServer.Shutdown(ctx)` with a 2-second timeout
7. CLI prints save confirmation (if saved) and returns

### Error handling

- Template not found: error before server starts (same as TUI path)
- Browser fails to open: print the URL to stderr so user can open manually
- Save fails (disk error): return 500, print error to stderr, don't shutdown
- Server port conflict: `:0` avoids this — OS always assigns a free port

---

## 4. Embedded HTML Page

The HTML page is a single self-contained file with inline CSS and JS.
It is embedded in the Go binary via `//go:embed static/editor.html`.

### Template data injection

The server injects template data into the HTML via a `<script>` tag
replacement. The HTML contains a placeholder:

```html
<script>const TEMPLATE_DATA = "__TEMPLATE_JSON__";</script>
```

The server replaces `__TEMPLATE_JSON__` with the actual JSON before
serving. This avoids a separate API call on page load.

Alternatively, the page can fetch `/api/template` on load. Both approaches
work — the placeholder injection is simpler (one request, no loading state).

### Grid UI

The grid layout and interaction model match the validated v3 mockup:

**Layout:**
- Group header rows (bold, spanning full width, non-editable)
- Task rows indented with `+` prefix, cells are editable
- Row total column (computed, non-editable)
- Day total row at bottom (computed)
- Hint bar with keyboard shortcuts
- Save / Cancel buttons

**Interaction:**
- Click a cell to select it (blue outline)
- Type digits/dot to enter a value — replaces cell content, shows input
- Enter or Tab commits and advances to next cell
- Escape cancels the current edit, reverts to previous value
- Arrow keys navigate between cells (commits any active edit first)
- Delete/Backspace clears selected cell to 0
- Shift-click fills value from selected cell to clicked cell within the same row
- Shift-hover shows green dashed preview of the fill range
- All values snap to nearest 0.5, clamped to [0, 24]
- Row totals and day totals update live on every commit

**Visual feedback:**
- Selected cell: blue outline
- Editing cell: yellow background with input field
- Fill preview: green dashed outline
- Filled cells: green flash animation (0.4s)
- Zero-value cells: grey dash (`-`)

---

## 5. CLI Integration

### Flag addition

Add `--web` boolean flag to the existing `edit` command:

```go
cmd.Flags().BoolVar(&webFlag, "web", false, "open the template editor in your browser")
```

### Branching logic

```go
if webFlag {
    // Start web editor server
    return web.RunEditor(tmpl, store)
} else {
    // Existing TUI editor path
    m := editor.New(tmpl.Name, tmpl.Rows)
    p := tea.NewProgram(m, tea.WithAltScreen())
    // ...
}
```

### Browser opening

Reuse `openBrowser()` from `internal/cli/auth/browser.go`. That function
is currently unexported and in the `auth` package. Two options:

1. Export it and move to a shared package (e.g. `internal/cli/util`)
2. Duplicate the 5-line function in the web editor package

Option 2 is simpler and avoids restructuring for a small utility.

---

## 6. Testing Strategy

### Go tests

- **Server endpoints:** Use `httptest.Server` to test GET `/`, GET `/api/template`,
  POST `/api/save`, POST `/api/cancel`. Verify correct JSON serialization,
  template data injection, save writes to disk, cancel doesn't write.
- **Snap logic:** Reuse existing `snapToHalf` tests (same 0.5 rounding).
- **Save round-trip:** Load template, start server, POST modified hours,
  verify saved template has the new values.

### Manual testing

- Run `tdx time template edit --web <name>` with a real template
- Verify browser opens, grid displays correctly
- Edit values, shift-click fill, verify totals
- Save and verify template file is updated
- Cancel and verify template file is unchanged

### No browser automation tests

Testing the HTML/JS interaction model requires a browser. The v3 mockup
served as the validation. The Go tests cover the server-side behavior.
Browser-level testing (Playwright/Cypress) is out of scope for a CLI tool.

---

## 7. Documentation Updates

### README.md

Update the template edit row in the command table:

```
| `tdx time template edit <name>` | Edit template hours in a grid editor | `--web` |
```

### docs/guide.md

Add to the "Edit a template" section:

```markdown
#### Browser editor

For a GUI experience, add `--web` to open the editor in your browser:

    tdx time template edit --web my-week

This starts a local server and opens a spreadsheet-like grid. Click cells
to select, type to enter values, shift-click to fill across a row. Click
Save when done — the server exits automatically.
```

---

## 8. Decision Log

| # | Decision | Rationale |
|---|---|---|
| D1 | Embedded HTTP server, not static file | Avoids CORS/file:// issues; clean save via POST |
| D2 | `go:embed` for HTML, not template rendering | Single file, no build step, embedded in binary |
| D3 | Random port via `:0` | No port conflicts, no configuration needed |
| D4 | `--web` flag on existing command, not separate command | Same operation, different interface |
| D5 | Vanilla HTML/CSS/JS, no framework | Zero frontend dependencies; the grid is simple enough |
| D6 | Click-to-select then type (not double-click) | Matches spreadsheet convention; validated in mockup |
| D7 | Shift-click fill within row only | Covers the main use case (Mon-Fri same hours); cross-row adds complexity |
| D8 | Server auto-exits after save/cancel | No orphan processes; clean lifecycle |
| D9 | Duplicate openBrowser rather than refactor | 5-line function; not worth a shared package |
| D10 | JSON placeholder injection, not separate fetch | One request, no loading state, simpler |
