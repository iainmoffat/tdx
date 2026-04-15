# Web Template Editor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `--web` flag to `tdx time template edit` that opens a browser-based grid editor served by an embedded HTTP server.

**Architecture:** An `internal/web/editor` package owns the HTTP server and embedded HTML. The CLI command adds a `--web` flag that branches to the web editor instead of the TUI. The HTML page is a self-contained file with inline CSS/JS, embedded via `go:embed`. The server starts on a random port, opens the browser, waits for a save or cancel POST, then exits.

**Tech Stack:** Go stdlib (`net/http`, `embed`, `encoding/json`). Vanilla HTML/CSS/JS (no framework). Existing: Cobra, `domain.Template`, `tmplsvc.Store`.

**Design spec:** `docs/specs/2026-04-13-tdx-web-template-editor-design.md`

---

## File Structure

| File | Responsibility |
|---|---|
| Create: `internal/web/editor/server.go` | HTTP server: start, serve HTML, handle `/api/template`, `/api/save`, `/api/cancel`, open browser, shutdown |
| Create: `internal/web/editor/server_test.go` | Tests: template JSON endpoint, save round-trip, cancel without save |
| Create: `internal/web/editor/static/editor.html` | Embedded HTML page: grid editor with CSS and JS |
| Create: `internal/web/editor/embed.go` | `//go:embed` directive and template data injection |
| Modify: `internal/cli/time/template/edit.go` | Add `--web` flag, branch to web editor |
| Modify: `README.md` | Add `--web` flag to template edit row |
| Modify: `docs/guide.md` | Add browser editor section |

---

## Task 1: Embedded HTML page

Create the self-contained editor.html with the validated grid UI. No Go code yet — just the static asset.

**Files:**
- Create: `internal/web/editor/static/editor.html`

- [ ] **Step 1: Create the HTML file**

Create `internal/web/editor/static/editor.html`. This is the full editor page. Template data is injected by the server as a JSON string replacing `"__TEMPLATE_JSON__"`.

```html
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>tdx template editor</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: system-ui, -apple-system, sans-serif; font-size: 14px; background: #fafafa; color: #333; padding: 20px; }
  h1 { font-size: 18px; font-weight: 600; margin-bottom: 16px; }
  h1 span { font-weight: 400; color: #888; }

  .grid-table { border-collapse: collapse; width: 100%; background: white; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
  .grid-table th, .grid-table td { border: 1px solid #ddd; padding: 8px 12px; text-align: center; }
  .grid-table th { background: #f5f5f5; font-weight: 600; font-size: 12px; text-transform: uppercase; position: sticky; top: 0; }
  .grid-table td.group-header { text-align: left; font-weight: 700; background: #f0f0f0; color: #333; border-bottom: 2px solid #ccc; }
  .grid-table td.task-label { text-align: left; padding-left: 24px; color: #555; min-width: 240px; }
  .grid-table .type-name { font-size: 11px; color: #999; font-style: italic; }
  .grid-table td.total-cell { font-weight: 600; background: #f9f9f9; }
  .grid-table tr.total-row td { border-top: 2px solid #333; font-weight: 700; background: #f5f5f5; }

  .cell { cursor: pointer; min-width: 56px; user-select: none; transition: background 0.1s; }
  .cell:hover { background: #f0f7ff; }
  .cell.selected { background: #d4e8ff; outline: 2px solid #4a90d9; outline-offset: -2px; }
  .cell.fill-preview { background: #e8f4e8; outline: 2px dashed #5cb85c; outline-offset: -2px; }
  .cell.filled-flash { animation: fillFlash 0.4s ease-out; }
  .cell .zero-val { color: #ccc; }
  @keyframes fillFlash { 0% { background: #5cb85c; color: white; } 100% { background: white; } }

  .cell-input {
    width: 100%; border: 2px solid #4a90d9; border-radius: 2px;
    padding: 6px 4px; text-align: center; font-size: 13px; outline: none;
    background: #fffde7; margin: -2px;
  }

  .hint-bar {
    margin-top: 12px; padding: 8px 12px; background: #f8f9fa; border-radius: 4px;
    font-size: 13px; color: #666; border: 1px solid #eee;
  }
  .hint-bar kbd { background: #e9ecef; padding: 1px 6px; border-radius: 3px; font-family: monospace; font-size: 12px; }
  .btn-row { margin-top: 16px; display: flex; gap: 10px; justify-content: flex-end; }
  .btn { padding: 8px 20px; border: 1px solid #ccc; border-radius: 4px; cursor: pointer; font-size: 14px; font-family: inherit; }
  .btn-save { background: #4a90d9; color: white; border-color: #4a90d9; }
  .btn-save:hover { background: #3a7bc8; }
  .btn-cancel { background: white; }
  .btn-cancel:hover { background: #f5f5f5; }
  .status { margin-top: 8px; font-size: 13px; color: #888; }
  .status.error { color: #d9534f; }
  .status.success { color: #5cb85c; }
</style>
</head>
<body>

<h1>tdx template editor <span id="tmpl-name"></span></h1>

<table class="grid-table" id="grid">
  <thead>
    <tr>
      <th style="text-align:left; min-width: 260px;">Task</th>
      <th>Sun</th><th>Mon</th><th>Tue</th><th>Wed</th><th>Thu</th><th>Fri</th><th>Sat</th>
      <th>Total</th>
    </tr>
  </thead>
  <tbody id="grid-body"></tbody>
</table>

<div class="hint-bar">
  <kbd>Click</kbd> select
  <kbd>Type</kbd> enter value
  <kbd>Enter</kbd>/<kbd>Tab</kbd> confirm &amp; next
  <kbd>Esc</kbd> cancel edit
  <kbd>Del</kbd> clear to 0
  <kbd>Shift+Click</kbd> fill across row
  <kbd>Arrows</kbd> navigate
</div>

<div class="btn-row">
  <button class="btn btn-cancel" id="btn-cancel">Cancel</button>
  <button class="btn btn-save" id="btn-save">Save Template</button>
</div>
<div class="status" id="status"></div>

<script>
const TEMPLATE_DATA = "__TEMPLATE_JSON__";

(function() {
  const data = typeof TEMPLATE_DATA === 'string' ? JSON.parse(TEMPLATE_DATA) : TEMPLATE_DATA;
  const days = ['sun','mon','tue','wed','thu','fri','sat'];
  let selected = null;
  let editing = false;
  let allCells = [];

  document.getElementById('tmpl-name').textContent = '— ' + data.name;

  // Build grid
  const tbody = document.getElementById('grid-body');
  let currentGroup = null;

  // Sort rows by group then label (match Go display order)
  const sorted = data.rows.slice().sort(function(a, b) {
    if (a.group !== b.group) return (a.group || '').localeCompare(b.group || '');
    return (a.label || '').localeCompare(b.label || '');
  });

  sorted.forEach(function(row, idx) {
    // Group header
    if (row.group && row.group !== currentGroup) {
      currentGroup = row.group;
      const gtr = document.createElement('tr');
      const gtd = document.createElement('td');
      gtd.className = 'group-header';
      gtd.colSpan = 9;
      gtd.textContent = row.group;
      gtr.appendChild(gtd);
      tbody.appendChild(gtr);
    }

    const tr = document.createElement('tr');
    tr.dataset.rowIdx = idx;

    // Label cell
    const labelTd = document.createElement('td');
    labelTd.className = 'task-label';
    labelTd.innerHTML = row.label + (row.typeName ? '<br><span class="type-name">' + row.typeName + '</span>' : '');
    tr.appendChild(labelTd);

    // Day cells
    days.forEach(function(day, colIdx) {
      const td = document.createElement('td');
      td.className = 'cell';
      td.dataset.rowIdx = idx;
      td.dataset.col = colIdx;
      td.dataset.rowId = row.id;
      td.dataset.day = day;
      td.dataset.val = row.hours[day] || 0;
      tr.appendChild(td);
      allCells.push(td);
    });

    // Total cell
    const totalTd = document.createElement('td');
    totalTd.className = 'total-cell';
    totalTd.dataset.rowTotal = idx;
    tr.appendChild(totalTd);

    tbody.appendChild(tr);
  });

  // Totals row
  const totalTr = document.createElement('tr');
  totalTr.className = 'total-row';
  const totalLabel = document.createElement('td');
  totalLabel.style.textAlign = 'left';
  totalLabel.textContent = 'DAY TOTAL';
  totalTr.appendChild(totalLabel);
  for (let c = 0; c < 7; c++) {
    const td = document.createElement('td');
    td.dataset.dayTotal = c;
    totalTr.appendChild(td);
  }
  const grandTd = document.createElement('td');
  grandTd.id = 'grand-total';
  totalTr.appendChild(grandTd);
  tbody.appendChild(totalTr);

  // Helpers
  function snap(v) { return Math.max(0, Math.min(24, Math.round(v * 2) / 2)); }
  function getVal(cell) { return parseFloat(cell.dataset.val) || 0; }
  function setVal(cell, v) { cell.dataset.val = snap(v); renderCell(cell); updateTotals(); }

  function renderCell(cell) {
    var v = getVal(cell);
    cell.innerHTML = v === 0 ? '<span class="zero-val">-</span>' : v.toFixed(1);
  }

  function updateTotals() {
    sorted.forEach(function(row, idx) {
      var t = 0;
      document.querySelectorAll('.cell[data-row-idx="'+idx+'"]').forEach(function(c) { t += getVal(c); });
      var el = document.querySelector('[data-row-total="'+idx+'"]');
      if (el) el.textContent = t.toFixed(1);
    });
    for (var c = 0; c < 7; c++) {
      var t = 0;
      document.querySelectorAll('.cell[data-col="'+c+'"]').forEach(function(cell) { t += getVal(cell); });
      var el = document.querySelector('[data-day-total="'+c+'"]');
      if (el) el.textContent = t.toFixed(1);
    }
    var g = 0;
    sorted.forEach(function(row, idx) {
      var el = document.querySelector('[data-row-total="'+idx+'"]');
      if (el) g += parseFloat(el.textContent) || 0;
    });
    document.getElementById('grand-total').textContent = g.toFixed(1);
  }

  function selectCell(cell) {
    stopEditing(true);
    allCells.forEach(function(c) { c.classList.remove('selected'); });
    cell.classList.add('selected');
    selected = cell;
  }

  function startEditing(firstChar) {
    if (!selected || editing) return;
    editing = true;
    var input = document.createElement('input');
    input.className = 'cell-input';
    input.type = 'text';
    input.inputMode = 'decimal';
    input.value = firstChar || '';
    input.placeholder = getVal(selected) === 0 ? '0' : getVal(selected).toFixed(1);
    selected.textContent = '';
    selected.appendChild(input);
    input.focus();

    input.addEventListener('keydown', function(e) {
      if (e.key === 'Enter' || e.key === 'Tab') {
        e.preventDefault(); e.stopPropagation();
        stopEditing(true); advance(e.shiftKey ? -1 : 1);
      } else if (e.key === 'Escape') {
        e.preventDefault(); e.stopPropagation(); stopEditing(false);
      } else if (e.key.startsWith('Arrow')) {
        e.preventDefault(); e.stopPropagation();
        stopEditing(true); navigate(e.key);
      }
    });
    input.addEventListener('blur', function() {
      setTimeout(function() { if (editing) stopEditing(true); }, 50);
    });
  }

  function stopEditing(commit) {
    if (!editing || !selected) { editing = false; return; }
    var input = selected.querySelector('.cell-input');
    if (input && commit) {
      var raw = input.value.trim();
      if (raw !== '') {
        var v = parseFloat(raw);
        if (!isNaN(v)) setVal(selected, v);
      }
    }
    editing = false;
    renderCell(selected);
  }

  function findCell(rowIdx, col) {
    return document.querySelector('.cell[data-row-idx="'+rowIdx+'"][data-col="'+col+'"]');
  }

  function advance(dir) {
    if (!selected) return;
    var r = parseInt(selected.dataset.rowIdx);
    var c = parseInt(selected.dataset.col) + dir;
    if (c > 6) { c = 0; r++; }
    if (c < 0) { c = 6; r--; }
    if (r >= sorted.length) r = 0;
    if (r < 0) r = sorted.length - 1;
    var next = findCell(r, c);
    if (next) selectCell(next);
  }

  function navigate(key) {
    if (!selected) return;
    var r = parseInt(selected.dataset.rowIdx), c = parseInt(selected.dataset.col);
    if (key === 'ArrowRight') c = Math.min(6, c + 1);
    else if (key === 'ArrowLeft') c = Math.max(0, c - 1);
    else if (key === 'ArrowDown') r = Math.min(sorted.length - 1, r + 1);
    else if (key === 'ArrowUp') r = Math.max(0, r - 1);
    var next = findCell(r, c);
    if (next) selectCell(next);
  }

  // Cell click
  allCells.forEach(function(cell) {
    cell.addEventListener('mousedown', function(e) {
      if (e.shiftKey && selected && selected.dataset.rowIdx === cell.dataset.rowIdx && selected !== cell) {
        e.preventDefault(); stopEditing(true);
        var row = cell.dataset.rowIdx;
        var from = parseInt(selected.dataset.col), to = parseInt(cell.dataset.col);
        var val = getVal(selected), lo = Math.min(from, to), hi = Math.max(from, to);
        for (var c = lo; c <= hi; c++) {
          var t = findCell(row, c);
          if (t) { setVal(t, val); t.classList.add('filled-flash'); setTimeout((function(el){return function(){el.classList.remove('filled-flash');};})(t), 400); }
        }
        return;
      }
      selectCell(cell);
    });
    cell.addEventListener('mouseenter', function(e) {
      allCells.forEach(function(c) { c.classList.remove('fill-preview'); });
      if (!e.shiftKey || !selected || selected.dataset.rowIdx !== cell.dataset.rowIdx || selected === cell) return;
      var from = parseInt(selected.dataset.col), to = parseInt(cell.dataset.col);
      var lo = Math.min(from, to), hi = Math.max(from, to);
      for (var c = lo; c <= hi; c++) {
        var t = findCell(cell.dataset.rowIdx, c);
        if (t && t !== selected) t.classList.add('fill-preview');
      }
    });
    cell.addEventListener('mouseleave', function() { allCells.forEach(function(c) { c.classList.remove('fill-preview'); }); });
  });

  // Global keyboard
  document.addEventListener('keydown', function(e) {
    if (editing) return;
    if (!selected) return;
    if (/^[0-9.]$/.test(e.key)) { e.preventDefault(); startEditing(e.key); return; }
    if (e.key === 'Delete' || e.key === 'Backspace') { e.preventDefault(); setVal(selected, 0); return; }
    if (e.key.startsWith('Arrow')) { e.preventDefault(); navigate(e.key); return; }
    if (e.key === 'Tab') { e.preventDefault(); advance(e.shiftKey ? -1 : 1); return; }
    if (e.key === 'Enter') { e.preventDefault(); advance(1); return; }
  });
  document.addEventListener('keyup', function(e) {
    if (e.key === 'Shift') allCells.forEach(function(c) { c.classList.remove('fill-preview'); });
  });
  document.addEventListener('mousedown', function(e) {
    if (!e.target.closest('.cell') && !e.target.closest('.cell-input')) {
      stopEditing(true); allCells.forEach(function(c) { c.classList.remove('selected'); }); selected = null;
    }
  });

  // Save / Cancel
  function collectData() {
    var result = [];
    sorted.forEach(function(row) {
      var hours = {};
      days.forEach(function(day, ci) {
        var cell = document.querySelector('.cell[data-row-id="'+row.id+'"][data-day="'+day+'"]');
        hours[day] = cell ? getVal(cell) : 0;
      });
      result.push({ id: row.id, hours: hours });
    });
    return { rows: result };
  }

  document.getElementById('btn-save').addEventListener('click', function() {
    var status = document.getElementById('status');
    status.className = 'status';
    status.textContent = 'Saving...';
    fetch('/api/save', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(collectData())
    }).then(function(r) {
      if (r.ok) {
        status.className = 'status success';
        status.textContent = 'Saved! You can close this tab.';
        document.getElementById('btn-save').disabled = true;
        document.getElementById('btn-cancel').disabled = true;
      } else {
        return r.text().then(function(t) { throw new Error(t); });
      }
    }).catch(function(err) {
      status.className = 'status error';
      status.textContent = 'Save failed: ' + err.message;
    });
  });

  document.getElementById('btn-cancel').addEventListener('click', function() {
    fetch('/api/cancel', { method: 'POST' }).then(function() {
      document.getElementById('status').className = 'status';
      document.getElementById('status').textContent = 'Cancelled. You can close this tab.';
      document.getElementById('btn-save').disabled = true;
      document.getElementById('btn-cancel').disabled = true;
    });
  });

  // Initial render
  allCells.forEach(renderCell);
  updateTotals();
})();
</script>
</body>
</html>
```

- [ ] **Step 2: Verify the file exists**

```bash
ls -la internal/web/editor/static/editor.html
```

- [ ] **Step 3: Commit**

```bash
git add internal/web/editor/static/editor.html
git commit -m "feat(web): add embedded HTML grid editor for templates"
```

---

## Task 2: Go embed and HTTP server

**Files:**
- Create: `internal/web/editor/embed.go`
- Create: `internal/web/editor/server.go`
- Create: `internal/web/editor/server_test.go`

- [ ] **Step 1: Write server tests**

Create `internal/web/editor/server_test.go`:

```go
package editor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func testTemplate() domain.Template {
	return domain.Template{
		SchemaVersion: 1,
		Name:          "test-tmpl",
		Rows: []domain.TemplateRow{
			{
				ID:       "row-01",
				Label:    "Admin",
				Target:   domain.Target{Kind: domain.TargetProjectTask, GroupName: "UFIT"},
				TimeType: domain.TimeType{ID: 5, Name: "Standard"},
				Hours:    domain.WeekHours{Mon: 8.0, Tue: 8.0, Wed: 8.0, Thu: 8.0, Fri: 8.0},
			},
			{
				ID:       "row-02",
				Label:    "Docker",
				Target:   domain.Target{Kind: domain.TargetProjectTask, GroupName: "Ops"},
				TimeType: domain.TimeType{ID: 5, Name: "Standard"},
				Hours:    domain.WeekHours{Tue: 1.0},
			},
		},
	}
}

func TestGetIndex_ServesHTML(t *testing.T) {
	srv := newServer(testTemplate(), nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/html")
	require.Contains(t, w.Body.String(), "tdx template editor")
	// Template data should be injected
	require.Contains(t, w.Body.String(), "row-01")
	require.Contains(t, w.Body.String(), "Admin")
}

func TestGetTemplate_ReturnsJSON(t *testing.T) {
	srv := newServer(testTemplate(), nil)
	req := httptest.NewRequest(http.MethodGet, "/api/template", nil)
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var resp templateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "test-tmpl", resp.Name)
	require.Len(t, resp.Rows, 2)
	require.Equal(t, "row-01", resp.Rows[0].ID)
	require.Equal(t, 8.0, resp.Rows[0].Hours.Mon)
}

func TestPostSave_UpdatesTemplate(t *testing.T) {
	var saved *domain.Template
	saveFn := func(tmpl domain.Template) error {
		saved = &tmpl
		return nil
	}
	srv := newServer(testTemplate(), saveFn)

	body := `{"rows":[
		{"id":"row-01","hours":{"sun":0,"mon":4,"tue":4,"wed":4,"thu":4,"fri":4,"sat":0}},
		{"id":"row-02","hours":{"sun":0,"mon":0,"tue":2,"wed":0,"thu":0,"fri":0,"sat":0}}
	]}`
	req := httptest.NewRequest(http.MethodPost, "/api/save", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, saved)
	require.InDelta(t, 4.0, saved.Rows[0].Hours.Mon, 0.001)
	require.InDelta(t, 2.0, saved.Rows[1].Hours.Tue, 0.001)
}

func TestPostCancel_NoSave(t *testing.T) {
	saveCalled := false
	saveFn := func(tmpl domain.Template) error {
		saveCalled = true
		return nil
	}
	srv := newServer(testTemplate(), saveFn)

	req := httptest.NewRequest(http.MethodPost, "/api/cancel", nil)
	w := httptest.NewRecorder()
	srv.handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.False(t, saveCalled)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/web/editor/ -count=1
```

Expected: compilation error — `newServer` not defined.

- [ ] **Step 3: Create embed.go**

Create `internal/web/editor/embed.go`:

```go
package editor

import (
	"embed"
	"encoding/json"
	"strings"
)

//go:embed static/editor.html
var editorHTML string

// templateResponse is the JSON shape served to the browser and used
// for template data injection into the HTML.
type templateResponse struct {
	Name string            `json:"name"`
	Rows []templateRowJSON `json:"rows"`
}

type templateRowJSON struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Group    string   `json:"group"`
	TypeName string   `json:"typeName"`
	Hours    hoursJSON `json:"hours"`
}

type hoursJSON struct {
	Sun float64 `json:"sun"`
	Mon float64 `json:"mon"`
	Tue float64 `json:"tue"`
	Wed float64 `json:"wed"`
	Thu float64 `json:"thu"`
	Fri float64 `json:"fri"`
	Sat float64 `json:"sat"`
}

// injectTemplateData replaces the placeholder in the HTML with actual
// template JSON data.
func injectTemplateData(html string, resp templateResponse) (string, error) {
	data, err := json.Marshal(resp)
	if err != nil {
		return "", err
	}
	// Escape for embedding inside a JS string literal.
	escaped := strings.ReplaceAll(string(data), `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return strings.Replace(html, `"__TEMPLATE_JSON__"`, `"`+escaped+`"`, 1), nil
}
```

- [ ] **Step 4: Create server.go**

Create `internal/web/editor/server.go`:

```go
package editor

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

// saveFn is a callback to persist the edited template.
type saveFn func(domain.Template) error

// server holds the state for a single edit session.
type server struct {
	tmpl     domain.Template
	save     saveFn
	shutdown chan result
}

// result signals how the server should exit.
type result struct {
	saved bool
	err   error
}

// Result is returned to the CLI after the server exits.
type Result struct {
	Saved bool
}

func newServer(tmpl domain.Template, save saveFn) *server {
	return &server{
		tmpl:     tmpl,
		save:     save,
		shutdown: make(chan result, 1),
	}
}

func (s *server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/template", s.handleGetTemplate)
	mux.HandleFunc("/api/save", s.handleSave)
	mux.HandleFunc("/api/cancel", s.handleCancel)
	return mux
}

func (s *server) toResponse() templateResponse {
	resp := templateResponse{Name: s.tmpl.Name}
	for _, r := range s.tmpl.Rows {
		resp.Rows = append(resp.Rows, templateRowJSON{
			ID:       r.ID,
			Label:    r.Label,
			Group:    r.Target.GroupName,
			TypeName: r.TimeType.Name,
			Hours: hoursJSON{
				Sun: r.Hours.Sun, Mon: r.Hours.Mon, Tue: r.Hours.Tue,
				Wed: r.Hours.Wed, Thu: r.Hours.Thu, Fri: r.Hours.Fri,
				Sat: r.Hours.Sat,
			},
		})
	}
	return resp
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	html, err := injectTemplateData(editorHTML, s.toResponse())
	if err != nil {
		http.Error(w, "failed to render editor", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}

func (s *server) handleGetTemplate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.toResponse())
}

type saveRequest struct {
	Rows []saveRow `json:"rows"`
}

type saveRow struct {
	ID    string    `json:"id"`
	Hours hoursJSON `json:"hours"`
}

func (s *server) handleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	var req saveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Build lookup by row ID.
	byID := make(map[string]hoursJSON, len(req.Rows))
	for _, row := range req.Rows {
		byID[row.ID] = row.Hours
	}

	// Apply edits to the template.
	for i, row := range s.tmpl.Rows {
		if h, ok := byID[row.ID]; ok {
			s.tmpl.Rows[i].Hours = domain.WeekHours{
				Sun: h.Sun, Mon: h.Mon, Tue: h.Tue,
				Wed: h.Wed, Thu: h.Thu, Fri: h.Fri,
				Sat: h.Sat,
			}
		}
	}
	s.tmpl.ModifiedAt = time.Now().UTC()

	// Save via callback.
	if s.save != nil {
		if err := s.save(s.tmpl); err != nil {
			http.Error(w, "save failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)

	// Signal shutdown.
	select {
	case s.shutdown <- result{saved: true}:
	default:
	}
}

func (s *server) handleCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	select {
	case s.shutdown <- result{saved: false}:
	default:
	}
}

// Run starts the HTTP server, opens the browser, and blocks until save
// or cancel is received. Returns whether the template was saved.
func Run(tmpl domain.Template, save saveFn) (Result, error) {
	srv := newServer(tmpl, save)

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return Result{}, fmt.Errorf("listen: %w", err)
	}

	addr := listener.Addr().String()
	url := "http://" + addr

	httpSrv := &http.Server{Handler: srv.handler()}
	go func() { _ = httpSrv.Serve(listener) }()

	// Open browser (best effort).
	if err := openBrowser(url); err != nil {
		fmt.Printf("Could not open browser: %v\nOpen %s manually.\n", err, url)
	}

	// Wait for save or cancel.
	res := <-srv.shutdown

	// Graceful shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)

	return Result{Saved: res.saved}, res.err
}

// openBrowser opens the given URL in the default browser.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/web/editor/ -count=1
```

Expected: all 4 tests pass.

- [ ] **Step 6: Verify vet**

```bash
go vet ./internal/web/editor/
```

- [ ] **Step 7: Commit**

```bash
git add internal/web/editor/
git commit -m "feat(web): add embedded HTTP server for browser template editor"
```

---

## Task 3: CLI integration — add --web flag

**Files:**
- Modify: `internal/cli/time/template/edit.go`

- [ ] **Step 1: Update edit.go to add --web flag**

Replace the contents of `internal/cli/time/template/edit.go` with:

```go
package template

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/tmplsvc"
	"github.com/iainmoffat/tdx/internal/tui/editor"
	webeditor "github.com/iainmoffat/tdx/internal/web/editor"
)

func newEditCmd() *cobra.Command {
	var webFlag bool

	cmd := &cobra.Command{
		Use:   "edit <name>",
		Short: "Edit template hours in an interactive grid",
		Long:  "Edit template hours in an interactive grid.\nUse --web to open the editor in your browser instead of the terminal.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			store := tmplsvc.NewStore(paths)
			tmpl, err := store.Load(args[0])
			if err != nil {
				return err
			}

			if webFlag {
				return runWebEditor(cmd, tmpl, store)
			}
			return runTUIEditor(cmd, tmpl, store)
		},
	}

	cmd.Flags().BoolVar(&webFlag, "web", false, "open the editor in your browser")
	return cmd
}

func runTUIEditor(cmd *cobra.Command, tmpl domain.Template, store *tmplsvc.Store) error {
	m := editor.New(tmpl.Name, tmpl.Rows)
	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("editor: %w", err)
	}

	final, _ := result.(editor.Model)
	if !final.Saved() {
		return nil
	}

	tmpl.Rows = final.Rows()
	tmpl.ModifiedAt = time.Now().UTC()
	if err := store.Save(tmpl); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "saved template %q\n", tmpl.Name)
	return nil
}

func runWebEditor(cmd *cobra.Command, tmpl domain.Template, store *tmplsvc.Store) error {
	saveFn := func(t domain.Template) error {
		return store.Save(t)
	}

	res, err := webeditor.Run(tmpl, saveFn)
	if err != nil {
		return fmt.Errorf("web editor: %w", err)
	}

	if res.Saved {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "saved template %q\n", tmpl.Name)
	}
	return nil
}
```

Note: You will need to add the `domain` import:

```go
import (
	"github.com/iainmoffat/tdx/internal/domain"
)
```

Add it alongside the existing imports.

- [ ] **Step 2: Verify build**

```bash
go build ./cmd/tdx
./tdx time template edit --help
```

Expected: help output shows `--web` flag.

- [ ] **Step 3: Run all tests**

```bash
go test ./... -count=1
```

Expected: all pass.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/time/template/edit.go
git commit -m "feat(cli): add --web flag to template edit command"
```

---

## Task 4: Documentation updates

**Files:**
- Modify: `README.md`
- Modify: `docs/guide.md`

- [ ] **Step 1: Update README.md**

In the Templates command table, change the `template edit` row from:

```
| `tdx time template edit <name>` | Edit template hours in a grid editor | |
```

to:

```
| `tdx time template edit <name>` | Edit template hours in a grid editor | `--web` |
```

- [ ] **Step 2: Update docs/guide.md**

In the "Edit a template" section, after the keybinding table and the closing paragraph about adjusting derived templates, add:

```markdown
#### Browser editor

For a GUI experience, add `--web` to open the editor in your browser:

```bash
tdx time template edit --web my-week
```

This starts a local server and opens a spreadsheet-like grid. Click cells
to select, type to enter values, shift-click to fill across a row. Click
Save when done — the server exits automatically.
```

- [ ] **Step 3: Commit**

```bash
git add README.md docs/guide.md
git commit -m "docs: add --web flag to README and user guide"
```

---

## Task 5: Final verification

- [ ] **Step 1: Run all tests**

```bash
go test ./... -count=1
```

Expected: all packages pass.

- [ ] **Step 2: Run vet and lint**

```bash
go vet ./...
golangci-lint run ./...
```

Expected: clean.

- [ ] **Step 3: Verify build**

```bash
make build
./tdx time template edit --help
```

Expected: help text shows `--web` flag with description "open the editor in your browser".

- [ ] **Step 4: Manual smoke test**

If you have an existing template:

```bash
./tdx time template edit --web my-week
```

Verify:
- Browser opens with the grid editor
- Template data is displayed correctly (grouped, with hours)
- Click a cell, type a value, press Enter — value commits
- Shift-click fills across a row
- Totals update live
- Save button writes the template and server exits
- Cancel button exits without saving

- [ ] **Step 5: Check commit log**

```bash
git log --oneline -5
```

Expected: 4 commits (HTML, server, CLI flag, docs).
