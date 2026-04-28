# Week Edit Grid Editor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> Each task follows strict TDD where applicable. Never amend commits — always create new ones. Branch: `week-edit-grid` (already created off `main`, has the design spec at `docs/specs/2026-04-28-week-edit-grid-editor.md`).
>
> Do NOT run `go mod tidy` — this fix introduces zero new dependencies.
>
> No `Co-Authored-By` trailer on commit messages.

**Design spec:** `docs/specs/2026-04-28-week-edit-grid-editor.md`

**Goal:** Make `tdx time week edit` use the same grid editor as `tdx time template edit`, including `--web` parity. Replace the YAML-in-`$EDITOR` flow.

**Architecture:** Generalize `internal/tui/editor` and `internal/web/editor` to operate on a small `Sheet` data type. Adapters in the CLI layer convert Template ↔ Sheet and WeekDraft ↔ Sheet. Draft cells preserve `SourceEntryID` and `PerCell` metadata across the round-trip per the rules in spec §8.

**Tech Stack:** Go 1.24, cobra, charmbracelet/bubbletea, gopkg.in/yaml.v3. No new deps.

---

## Task 1: Sheet types

**Files:**
- Create: `internal/tui/editor/sheet.go`
- Create: `internal/tui/editor/sheet_test.go`

- [ ] **Step 1.1 — Failing test for Sheet struct shape**

Create `internal/tui/editor/sheet_test.go`:

```go
package editor

import (
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestSheet_StructHasExpectedFields(t *testing.T) {
	s := Sheet{
		Name: "demo",
		Rows: []SheetRow{
			{
				ID:         "row-01",
				Label:      "Admin",
				GroupName:  "UFIT Administration",
				DisplayRef: "plan/2075/task/2076",
				TypeName:   "Standard Activities",
				Hours:      domain.WeekHours{Mon: 4.0},
			},
		},
	}
	require.Equal(t, "demo", s.Name)
	require.Len(t, s.Rows, 1)
	require.Equal(t, "row-01", s.Rows[0].ID)
	require.Equal(t, "Admin", s.Rows[0].Label)
	require.Equal(t, "UFIT Administration", s.Rows[0].GroupName)
	require.Equal(t, "plan/2075/task/2076", s.Rows[0].DisplayRef)
	require.Equal(t, "Standard Activities", s.Rows[0].TypeName)
	require.InDelta(t, 4.0, s.Rows[0].Hours.Mon, 0.001)
}
```

- [ ] **Step 1.2 — Run test to verify it fails**

Run: `go test ./internal/tui/editor/ -run TestSheet_StructHasExpectedFields -v`
Expected: FAIL with "undefined: Sheet" / "undefined: SheetRow".

- [ ] **Step 1.3 — Create sheet.go**

Create `internal/tui/editor/sheet.go`:

```go
package editor

import "github.com/iainmoffat/tdx/internal/domain"

// Sheet is the editor's view of a hours-by-day grid. The TUI and web
// editors both operate on Sheet; adapters convert from Template, WeekDraft,
// or any other source.
//
// Sheet does not know what it represents. Name is the editor title;
// Rows are presented in display order (caller-determined).
type Sheet struct {
	Name string
	Rows []SheetRow
}

// SheetRow is one row of a Sheet — typically one (target, type, billable)
// triple, but the editor only cares about the ID for matching on save.
type SheetRow struct {
	ID         string
	Label      string
	GroupName  string
	DisplayRef string
	TypeName   string
	Hours      domain.WeekHours
}
```

- [ ] **Step 1.4 — Run test to verify it passes**

Run: `go test ./internal/tui/editor/ -run TestSheet_StructHasExpectedFields -v`
Expected: PASS.

- [ ] **Step 1.5 — Commit**

```bash
git add internal/tui/editor/sheet.go internal/tui/editor/sheet_test.go
git commit -m "feat(editor): Sheet + SheetRow types — domain-agnostic editor input"
```

---

## Task 2: Refactor TUI editor (model + view + tests) to use Sheet

This is the largest task. The model and view are tightly coupled — refactor them together so the package always compiles. Tests move to constructing `Sheet` directly.

**Files:**
- Modify: `internal/tui/editor/model.go`
- Modify: `internal/tui/editor/view.go`
- Modify: `internal/tui/editor/model_test.go`

- [ ] **Step 2.1 — Replace model.go with Sheet-driven version**

Overwrite `internal/tui/editor/model.go`:

```go
package editor

import (
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/iainmoffat/tdx/internal/domain"
)

// cursor tracks the active cell position.
type cursor struct {
	row int // 0-based index into rows
	col int // 0=Sun, 1=Mon, ..., 6=Sat
}

// weekday returns the time.Weekday for the current column.
func (c cursor) weekday() time.Weekday {
	return time.Weekday(c.col)
}

// Model is the bubbletea model for the grid editor. It operates on a Sheet,
// which abstracts over Templates, WeekDrafts, and any other hours-per-day
// data shape.
type Model struct {
	name     string
	rows     []SheetRow
	original []domain.WeekHours
	cursor   cursor
	typing   bool
	input    string
	dirty    bool
	quitting bool
	saved    bool
	confirm  bool
	width    int
	height   int
}

// New creates a new editor Model from a Sheet. Rows are taken in the order
// the caller provides — sorting (e.g. by group then label) is the
// adapter's responsibility.
func New(sheet Sheet) Model {
	rows := make([]SheetRow, len(sheet.Rows))
	copy(rows, sheet.Rows)
	orig := make([]domain.WeekHours, len(rows))
	for i, r := range rows {
		orig[i] = r.Hours
	}
	return Model{
		name:     sheet.Name,
		rows:     rows,
		original: orig,
	}
}

// Saved reports whether the user chose to save.
func (m Model) Saved() bool { return m.saved }

// Sheet returns the (possibly edited) sheet. The Name and row identities
// are unchanged; only Hours per row may have been modified.
func (m Model) Sheet() Sheet {
	rows := make([]SheetRow, len(m.rows))
	copy(rows, m.rows)
	return Sheet{Name: m.name, Rows: rows}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.confirm {
			return m.handleConfirm(msg)
		}

		if m.typing {
			return m.handleTyping(msg)
		}

		switch msg.Type {
		case tea.KeyCtrlS:
			m.commitTyping()
			m.saved = true
			m.quitting = true
			return m, tea.Quit

		case tea.KeyEsc:
			if m.dirty {
				m.confirm = true
				return m, nil
			}
			m.quitting = true
			return m, tea.Quit

		case tea.KeyRunes:
			if len(msg.Runes) == 1 {
				r := msg.Runes[0]
				if r == 'q' {
					if m.dirty {
						m.confirm = true
						return m, nil
					}
					m.quitting = true
					return m, tea.Quit
				}
				if r >= '0' && r <= '9' || r == '.' {
					m.typing = true
					m.input = string(r)
					return m, nil
				}
			}

		case tea.KeyUp:
			if m.cursor.row > 0 {
				m.cursor.row--
			}
			return m, nil

		case tea.KeyDown:
			if m.cursor.row < len(m.rows)-1 {
				m.cursor.row++
			}
			return m, nil

		case tea.KeyLeft:
			m.moveCell(-1)
			return m, nil

		case tea.KeyRight:
			m.moveCell(1)
			return m, nil

		case tea.KeyTab:
			m.moveCell(1)
			return m, nil

		case tea.KeyShiftTab:
			m.moveCell(-1)
			return m, nil

		case tea.KeyBackspace:
			m.setCellValue(0)
			return m, nil

		case tea.KeyEnter:
			m.moveCell(1)
			return m, nil
		}
	}
	return m, nil
}

func (m Model) handleConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'y' {
		m.quitting = true
		return m, tea.Quit
	}
	m.confirm = false
	return m, nil
}

func (m Model) handleTyping(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		m.commitTyping()
		m.moveCell(1)
		return m, nil
	case tea.KeyEsc:
		m.typing = false
		m.input = ""
		return m, nil
	case tea.KeyBackspace:
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
		if len(m.input) == 0 {
			m.typing = false
		}
		return m, nil
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			r := msg.Runes[0]
			if r >= '0' && r <= '9' || r == '.' {
				m.input += string(r)
				return m, nil
			}
		}
	}
	m.commitTyping()
	return m.Update(msg)
}

func (m *Model) commitTyping() {
	m.typing = false
	if m.input == "" {
		return
	}
	v, err := strconv.ParseFloat(m.input, 64)
	if err != nil {
		m.input = ""
		return
	}
	m.input = ""
	m.setCellValue(snapToHalf(v))
}

func (m *Model) setCellValue(v float64) {
	wd := m.cursor.weekday()
	m.rows[m.cursor.row].Hours.SetDay(wd, v)
	m.updateDirty()
}

func (m *Model) updateDirty() {
	m.dirty = false
	for i, r := range m.rows {
		if r.Hours != m.original[i] {
			m.dirty = true
			return
		}
	}
}

func (m *Model) moveCell(dir int) {
	pos := m.cursor.row*7 + m.cursor.col + dir
	total := len(m.rows) * 7
	if pos < 0 {
		pos = 0
	}
	if pos >= total {
		pos = total - 1
	}
	m.cursor.row = pos / 7
	m.cursor.col = pos % 7
}
```

- [ ] **Step 2.2 — Replace view.go with Sheet-driven version**

Overwrite `internal/tui/editor/view.go`:

```go
package editor

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	activeStyle = lipgloss.NewStyle().Reverse(true)
	headerStyle = lipgloss.NewStyle().Bold(true)
	hintStyle   = lipgloss.NewStyle().Faint(true)
	groupStyle  = lipgloss.NewStyle().Bold(true)
)

var dayNames = [7]string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}

const cellWidth = 6

// View implements tea.Model.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	title := "Editing: " + m.name
	if m.dirty {
		title += " [modified]"
	}
	b.WriteString(headerStyle.Render(title))
	b.WriteString("\n\n")

	if m.confirm {
		b.WriteString("  Unsaved changes. Quit? (y/n)")
		return b.String()
	}

	type indexedRow struct {
		flatIdx int
		row     SheetRow
	}
	type group struct {
		name string
		rows []indexedRow
	}
	var groups []group
	groupIdx := map[string]int{}

	for i, r := range m.rows {
		gn := r.GroupName
		if gn == "" {
			groups = append(groups, group{name: "", rows: []indexedRow{{i, r}}})
			continue
		}
		if idx, ok := groupIdx[gn]; ok {
			groups[idx].rows = append(groups[idx].rows, indexedRow{i, r})
		} else {
			groupIdx[gn] = len(groups)
			groups = append(groups, group{name: gn, rows: []indexedRow{{i, r}}})
		}
	}

	labelWidth := len("  ROW")
	for _, g := range groups {
		if g.name != "" {
			label := "  " + g.name
			if len(label) > labelWidth {
				labelWidth = len(label)
			}
		}
		for _, ir := range g.rows {
			prefix := "  "
			if g.name != "" {
				prefix = "    + "
			}
			label := prefix + rowLabel(ir.row)
			if len(label) > labelWidth {
				labelWidth = len(label)
			}
		}
	}

	header := padRight("  ROW", labelWidth)
	for _, d := range dayNames {
		header += "  " + padRight(d, cellWidth-1)
	}
	header += "  TOTAL"
	b.WriteString(header + "\n")

	sepLen := labelWidth + 7*(1+cellWidth) + 2 + 5
	b.WriteString(strings.Repeat("─", sepLen) + "\n")

	var dayTotals [7]float64
	for _, g := range groups {
		if g.name != "" {
			var gDays [7]float64
			for _, ir := range g.rows {
				for ci := 0; ci < 7; ci++ {
					gDays[ci] += ir.row.Hours.ForDay(time.Weekday(ci))
				}
			}
			gTotal := 0.0
			for ci := 0; ci < 7; ci++ {
				gTotal += gDays[ci]
			}
			headerLine := padRight("  "+g.name, labelWidth)
			for ci := 0; ci < 7; ci++ {
				if gDays[ci] == 0 {
					headerLine += "  " + padRight(" ", cellWidth-1)
				} else {
					headerLine += "  " + padRight(fmt.Sprintf("%.1f", gDays[ci]), cellWidth-1)
				}
			}
			if gTotal > 0 {
				headerLine += "  " + padRight(fmt.Sprintf("%.1f", gTotal), cellWidth-1)
			}
			b.WriteString(groupStyle.Render(strings.TrimRight(headerLine, " ")) + "\n")

			for _, ir := range g.rows {
				label := "    + " + rowLabel(ir.row)
				line := padRight(label, labelWidth)
				rowTotal := 0.0
				for ci := 0; ci < 7; ci++ {
					wd := time.Weekday(ci)
					hours := ir.row.Hours.ForDay(wd)
					cell := m.formatCell(ir.flatIdx, ci, hours)
					line += "  " + cell
					dayTotals[ci] += hours
					rowTotal += hours
				}
				line += "  " + padRight(fmt.Sprintf("%.1f", rowTotal), cellWidth-1)
				b.WriteString(strings.TrimRight(line, " ") + "\n")
				if ir.row.TypeName != "" {
					b.WriteString("        " + ir.row.TypeName + "\n")
				}
			}
		} else {
			ir := g.rows[0]
			label := "  " + rowLabel(ir.row)
			line := padRight(label, labelWidth)
			rowTotal := 0.0
			for ci := 0; ci < 7; ci++ {
				wd := time.Weekday(ci)
				hours := ir.row.Hours.ForDay(wd)
				cell := m.formatCell(ir.flatIdx, ci, hours)
				line += "  " + cell
				dayTotals[ci] += hours
				rowTotal += hours
			}
			line += "  " + padRight(fmt.Sprintf("%.1f", rowTotal), cellWidth-1)
			b.WriteString(strings.TrimRight(line, " ") + "\n")
			if ir.row.TypeName != "" {
				b.WriteString("    └ " + ir.row.TypeName + "\n")
			}
		}
	}

	b.WriteString(strings.Repeat("─", sepLen) + "\n")

	totalLine := padRight("  DAY TOTAL", labelWidth)
	grandTotal := 0.0
	for ci := 0; ci < 7; ci++ {
		v := fmt.Sprintf("%.1f", dayTotals[ci])
		if dayTotals[ci] == 0 {
			v = "."
		}
		totalLine += "  " + padRight(v, cellWidth-1)
		grandTotal += dayTotals[ci]
	}
	totalLine += "  " + padRight(fmt.Sprintf("%.1f", grandTotal), cellWidth-1)
	b.WriteString(strings.TrimRight(totalLine, " ") + "\n")

	b.WriteString("\n")
	hints := "  ←→↑↓/Tab: navigate  type: set value  Backspace: clear  Ctrl-S: save  Esc: cancel"
	b.WriteString(hintStyle.Render(hints))

	return b.String()
}

// rowLabel produces the display label for a SheetRow. Falls back to
// DisplayRef when Label is empty.
func rowLabel(r SheetRow) string {
	label := r.Label
	if label == "" {
		label = r.DisplayRef
	}
	return label
}

func (m Model) formatCell(row, col int, hours float64) string {
	isActive := m.cursor.row == row && m.cursor.col == col

	var text string
	if isActive && m.typing {
		text = m.input + "_"
	} else if hours == 0 {
		text = "."
	} else {
		text = fmt.Sprintf("%.1f", hours)
	}

	padded := padRight(text, cellWidth-1)
	if isActive {
		return activeStyle.Render(padded)
	}
	return padded
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
```

NOTE: `rowLabel` is now a package-level function (no `(m Model)` receiver) since it's pure. The previous version included a `(kind)` suffix when GroupName was empty — that suffix is dropped because SheetRow doesn't carry the target kind. This is an acceptable cosmetic change; ungrouped rows now show only Label.

- [ ] **Step 2.3 — Update model_test.go to construct Sheet directly**

Overwrite `internal/tui/editor/model_test.go`. The existing test functions are kept but rewritten to use `Sheet` and `SheetRow`. The grouped-rows test now constructs rows in expected display order (sorting moves to the adapter):

```go
package editor

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func testSheet() Sheet {
	return Sheet{
		Name: "test",
		Rows: []SheetRow{
			{
				ID:       "row-01",
				Label:    "Admin",
				TypeName: "Dev",
				Hours:    domain.WeekHours{Mon: 8.0, Tue: 8.0, Wed: 8.0, Thu: 8.0, Fri: 8.0},
			},
			{
				ID:       "row-02",
				Label:    "Project",
				TypeName: "Planning",
				Hours:    domain.WeekHours{Mon: 1.0, Wed: 2.0},
			},
		},
	}
}

func sendKey(m Model, k tea.KeyType) Model {
	updated, _ := m.Update(tea.KeyMsg{Type: k})
	result, _ := updated.(Model)
	return result
}

func sendRune(m Model, r rune) Model {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	result, _ := updated.(Model)
	return result
}

func typeAndCommit(m Model, val string) Model {
	for _, r := range val {
		m = sendRune(m, r)
	}
	m = sendKey(m, tea.KeyEnter)
	return m
}

func TestModel_InitialCursor(t *testing.T) {
	m := New(testSheet())
	require.Equal(t, 0, m.cursor.row)
	require.Equal(t, 0, m.cursor.col)
	require.False(t, m.dirty)
}

func TestModel_NavigateRight(t *testing.T) {
	m := New(testSheet())
	m = sendKey(m, tea.KeyRight)
	require.Equal(t, 0, m.cursor.row)
	require.Equal(t, 1, m.cursor.col)
}

func TestModel_NavigateDown(t *testing.T) {
	m := New(testSheet())
	m = sendKey(m, tea.KeyDown)
	require.Equal(t, 1, m.cursor.row)
	require.Equal(t, 0, m.cursor.col)
}

func TestModel_WrapRight(t *testing.T) {
	m := New(testSheet())
	for i := 0; i < 7; i++ {
		m = sendKey(m, tea.KeyRight)
	}
	require.Equal(t, 1, m.cursor.row)
	require.Equal(t, 0, m.cursor.col)
}

func TestModel_WrapLeft(t *testing.T) {
	m := New(testSheet())
	m = sendKey(m, tea.KeyDown)
	m = sendKey(m, tea.KeyLeft)
	require.Equal(t, 0, m.cursor.row)
	require.Equal(t, 6, m.cursor.col)
}

func TestModel_ClampTop(t *testing.T) {
	m := New(testSheet())
	m = sendKey(m, tea.KeyUp)
	require.Equal(t, 0, m.cursor.row)
}

func TestModel_ClampBottom(t *testing.T) {
	m := New(testSheet())
	for i := 0; i < 5; i++ {
		m = sendKey(m, tea.KeyDown)
	}
	require.Equal(t, 1, m.cursor.row)
}

func TestModel_TypeValue(t *testing.T) {
	m := New(testSheet())
	m = sendKey(m, tea.KeyRight)
	m = typeAndCommit(m, "4")
	require.InDelta(t, 4.0, m.rows[0].Hours.Mon, 0.001)
	require.True(t, m.dirty)
}

func TestModel_TypeReplacesExisting(t *testing.T) {
	m := New(testSheet())
	m = sendKey(m, tea.KeyRight)
	m = typeAndCommit(m, "3")
	require.InDelta(t, 3.0, m.rows[0].Hours.Mon, 0.001)
}

func TestModel_TypeSnaps(t *testing.T) {
	m := New(testSheet())
	m = sendKey(m, tea.KeyRight)
	m = typeAndCommit(m, "3.7")
	require.InDelta(t, 3.5, m.rows[0].Hours.Mon, 0.001)
}

func TestModel_BackspaceClearsCell(t *testing.T) {
	m := New(testSheet())
	m = sendKey(m, tea.KeyRight)
	m = sendKey(m, tea.KeyBackspace)
	require.InDelta(t, 0.0, m.rows[0].Hours.Mon, 0.001)
	require.True(t, m.dirty)
}

func TestModel_DirtyDetection(t *testing.T) {
	m := New(testSheet())
	require.False(t, m.dirty)
	m = sendKey(m, tea.KeyRight)
	m = typeAndCommit(m, "5")
	require.True(t, m.dirty)
}

func TestModel_DirtyRevert(t *testing.T) {
	m := New(testSheet())
	m = sendKey(m, tea.KeyRight)
	m = typeAndCommit(m, "5")
	require.True(t, m.dirty)
	m = typeAndCommit(m, "8")
	require.False(t, m.dirty, "reverting to original value clears dirty")
}

func TestModel_SaveExit(t *testing.T) {
	m := New(testSheet())
	m = sendKey(m, tea.KeyCtrlS)
	require.True(t, m.saved)
	require.True(t, m.quitting)
}

func TestModel_CancelClean(t *testing.T) {
	m := New(testSheet())
	m = sendKey(m, tea.KeyEsc)
	require.False(t, m.saved)
	require.True(t, m.quitting)
}

func TestModel_CancelDirtyPrompt(t *testing.T) {
	m := New(testSheet())
	m = sendKey(m, tea.KeyRight)
	m = typeAndCommit(m, "3")
	m = sendKey(m, tea.KeyEsc)
	require.True(t, m.confirm)
	require.False(t, m.quitting)
}

func TestModel_CancelDirtyDeny(t *testing.T) {
	m := New(testSheet())
	m = sendKey(m, tea.KeyRight)
	m = typeAndCommit(m, "3")
	m = sendKey(m, tea.KeyEsc)
	m = sendRune(m, 'n')
	require.False(t, m.confirm)
	require.False(t, m.quitting)
}

func TestModel_TabWraps(t *testing.T) {
	m := New(testSheet())
	for i := 0; i < 7; i++ {
		m = sendKey(m, tea.KeyTab)
	}
	require.Equal(t, 1, m.cursor.row)
	require.Equal(t, 0, m.cursor.col)
}

func TestModel_CtrlS_WhileTyping(t *testing.T) {
	m := New(testSheet())
	m = sendKey(m, tea.KeyRight)
	m = sendRune(m, '5')
	m = sendKey(m, tea.KeyCtrlS)
	require.True(t, m.saved)
	require.InDelta(t, 5.0, m.rows[0].Hours.Mon, 0.001, "in-progress input should be committed before save")
}

func TestModel_Esc_WhileTyping(t *testing.T) {
	m := New(testSheet())
	m = sendKey(m, tea.KeyRight)
	m = sendRune(m, '5')
	m = sendKey(m, tea.KeyEsc)
	require.False(t, m.typing)
	require.False(t, m.dirty, "Esc while typing should discard the in-progress input")
}

func TestModel_QuitKey_Clean(t *testing.T) {
	m := New(testSheet())
	m = sendRune(m, 'q')
	require.True(t, m.quitting)
}

func TestModel_QuitKey_Dirty(t *testing.T) {
	m := New(testSheet())
	m = sendKey(m, tea.KeyRight)
	m = typeAndCommit(m, "3")
	m = sendRune(m, 'q')
	require.True(t, m.confirm)
	require.False(t, m.quitting)
}

// testGroupedSheet returns rows already in display order. (Display ordering
// moved out of the editor in the Sheet refactor; the adapter sorts.)
func testGroupedSheet() Sheet {
	return Sheet{
		Name: "test",
		Rows: []SheetRow{
			{ID: "row-01", Label: "Admin Task", GroupName: "UFIT Administration", TypeName: "Standard", Hours: domain.WeekHours{Mon: 8.0}},
			{ID: "row-03", Label: "Prof Dev", GroupName: "UFIT Administration", TypeName: "Training"},
			{ID: "row-04", Label: "Docker", GroupName: "UFIT Operations", TypeName: "Standard", Hours: domain.WeekHours{Tue: 1.0}},
			{ID: "row-02", Label: "Linux", GroupName: "UFIT Operations", TypeName: "Standard", Hours: domain.WeekHours{Mon: 1.0}},
		},
	}
}

func TestModel_GroupedNavigation_DownVisitsGivenOrder(t *testing.T) {
	m := New(testGroupedSheet())

	var visited []string
	for i := 0; i < len(m.rows); i++ {
		visited = append(visited, m.rows[m.cursor.row].ID)
		m = sendKey(m, tea.KeyDown)
	}

	require.Equal(t, []string{"row-01", "row-03", "row-04", "row-02"}, visited,
		"navigation follows the order the caller provided (no editor-side sort)")
}

func TestModel_GroupedRows_PreservesAfterEdit(t *testing.T) {
	m := New(testGroupedSheet())
	require.Equal(t, "row-01", m.rows[0].ID)

	m = sendKey(m, tea.KeyRight)
	m = typeAndCommit(m, "4")
	require.InDelta(t, 4.0, m.rows[0].Hours.Mon, 0.001)

	out := m.Sheet()
	require.Equal(t, "row-01", out.Rows[0].ID)
	require.InDelta(t, 4.0, out.Rows[0].Hours.Mon, 0.001)
}
```

- [ ] **Step 2.4 — Run editor package tests**

Run: `go test ./internal/tui/editor/ -count=1 -v 2>&1 | tail -40`
Expected: all PASS. The package no longer imports `domain.TemplateRow`.

- [ ] **Step 2.5 — Run the full module to find compile errors in dependents**

Run: `go build ./...`
Expected: build FAILS in `internal/cli/time/template/edit.go` and `internal/web/editor/server.go` (they still call `editor.New(name, []domain.TemplateRow)`). That's fine — Tasks 3 and 4 fix those.

- [ ] **Step 2.6 — Commit**

```bash
git add internal/tui/editor/model.go internal/tui/editor/view.go internal/tui/editor/model_test.go
git commit -m "refactor(editor): TUI editor operates on Sheet (was []TemplateRow)"
```

This commit intentionally breaks the build of dependents — Tasks 3 and 4 land the fixes immediately after. If you want to keep main always green, run Tasks 2/3/4 in one cycle (don't push between them).

---

## Task 3: Refactor web editor to use Sheet

**Files:**
- Modify: `internal/web/editor/server.go`
- Modify: `internal/web/editor/server_test.go`

- [ ] **Step 3.1 — Read the current server_test.go to understand the test surface**

Run: `cat internal/web/editor/server_test.go`

This shows what the existing handlers test. Refactor the test to construct a Sheet rather than a Template, and to expect a sheet-shaped save callback.

- [ ] **Step 3.2 — Replace server.go with Sheet-driven version**

Overwrite `internal/web/editor/server.go`:

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

	"github.com/iainmoffat/tdx/internal/tui/editor"
)

// SaveFn is a callback to persist the edited sheet.
type SaveFn func(editor.Sheet) error

// server holds the state for a single edit session.
type server struct {
	sheet    editor.Sheet
	save     SaveFn
	shutdown chan result
}

type result struct {
	saved bool
	err   error
}

// Result is returned to the CLI after the server exits.
type Result struct {
	Saved bool
}

func newServer(sheet editor.Sheet, save SaveFn) *server {
	return &server{
		sheet:    sheet,
		save:     save,
		shutdown: make(chan result, 1),
	}
}

func (s *server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/template", s.handleGetSheet)
	mux.HandleFunc("/api/save", s.handleSave)
	mux.HandleFunc("/api/cancel", s.handleCancel)
	return mux
}

type sheetResponse struct {
	Name string         `json:"name"`
	Rows []sheetRowJSON `json:"rows"`
}

type sheetRowJSON struct {
	ID       string    `json:"id"`
	Label    string    `json:"label"`
	Group    string    `json:"group,omitempty"`
	TypeName string    `json:"typeName,omitempty"`
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

func (s *server) toResponse() sheetResponse {
	resp := sheetResponse{Name: s.sheet.Name}
	for _, r := range s.sheet.Rows {
		label := r.Label
		if label == "" {
			label = r.DisplayRef
		}
		resp.Rows = append(resp.Rows, sheetRowJSON{
			ID:       r.ID,
			Label:    label,
			Group:    r.GroupName,
			TypeName: r.TypeName,
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

func (s *server) handleGetSheet(w http.ResponseWriter, r *http.Request) {
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

	byID := make(map[string]hoursJSON, len(req.Rows))
	for _, row := range req.Rows {
		byID[row.ID] = row.Hours
	}

	for i, row := range s.sheet.Rows {
		if h, ok := byID[row.ID]; ok {
			s.sheet.Rows[i].Hours.Sun = h.Sun
			s.sheet.Rows[i].Hours.Mon = h.Mon
			s.sheet.Rows[i].Hours.Tue = h.Tue
			s.sheet.Rows[i].Hours.Wed = h.Wed
			s.sheet.Rows[i].Hours.Thu = h.Thu
			s.sheet.Rows[i].Hours.Fri = h.Fri
			s.sheet.Rows[i].Hours.Sat = h.Sat
		}
	}

	if s.save != nil {
		if err := s.save(s.sheet); err != nil {
			http.Error(w, "save failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
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
// or cancel. Returns whether the sheet was saved.
func Run(sheet editor.Sheet, save SaveFn) (Result, error) {
	srv := newServer(sheet, save)

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return Result{}, fmt.Errorf("listen: %w", err)
	}

	addr := listener.Addr().String()
	url := "http://" + addr

	httpSrv := &http.Server{Handler: srv.handler()}
	go func() { _ = httpSrv.Serve(listener) }()

	if err := openBrowser(url); err != nil {
		_, _ = fmt.Printf("Could not open browser: %v\nOpen %s manually.\n", err, url)
	}

	res := <-srv.shutdown

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)

	return Result{Saved: res.saved}, res.err
}

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

NOTE: The handler routes `/api/template`, `/api/save`, `/api/cancel` are kept as-is — the static HTML/JS in `internal/web/editor/static/` posts to those paths. The wire JSON shape (rows with id/label/group/hours) is unchanged. Only Go-side types change.

- [ ] **Step 3.3 — Update server_test.go to use Sheet**

Replace the test fixtures in `internal/web/editor/server_test.go` with sheet-shaped equivalents. Construct `editor.Sheet` directly. Use the existing test patterns (httptest.NewServer, JSON decode of the response, POST to /api/save, verify save callback received correct sheet).

Open the file, identify each test function, and adapt:
- Replace `tmpl domain.Template` setup → `sheet editor.Sheet`
- Replace `SaveFn func(domain.Template) error` → `SaveFn func(editor.Sheet) error`
- Replace assertions on `tmpl.Rows[0].Hours.Mon` → `sheet.Rows[0].Hours.Mon`
- The handler-route test for `/api/template` still hits `/api/template` (route name unchanged); response field is now `sheetResponse` shape with same JSON keys, so most assertions don't change.

Add `"github.com/iainmoffat/tdx/internal/tui/editor"` to the imports if not already present.

- [ ] **Step 3.4 — Run web editor tests**

Run: `go test ./internal/web/editor/ -count=1 -v 2>&1 | tail -20`
Expected: all PASS.

- [ ] **Step 3.5 — Confirm full build still fails only in template/edit.go (next task)**

Run: `go build ./...`
Expected: FAIL only in `internal/cli/time/template/edit.go` (still calls editor.New with old signature).

- [ ] **Step 3.6 — Commit**

```bash
git add internal/web/editor/server.go internal/web/editor/server_test.go
git commit -m "refactor(web): web editor operates on Sheet (was Template)"
```

---

## Task 4: Update template edit.go to use Sheet

**Files:**
- Modify: `internal/cli/time/template/edit.go`
- Modify: `internal/cli/time/template/edit_test.go`

- [ ] **Step 4.1 — Replace template/edit.go with Sheet-driven version**

Overwrite `internal/cli/time/template/edit.go`:

```go
package template

import (
	"fmt"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/tmplsvc"
	"github.com/iainmoffat/tdx/internal/tui/editor"
	webeditor "github.com/iainmoffat/tdx/internal/web/editor"
)

func newEditCmd() *cobra.Command {
	var (
		webFlag     bool
		profileFlag string
	)

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
			auth := authsvc.New(paths)
			profile, err := auth.ResolveProfile(profileFlag)
			if err != nil {
				return err
			}
			store := tmplsvc.NewStore(paths)
			tmpl, err := store.Load(profile, args[0])
			if err != nil {
				return err
			}

			if webFlag {
				return runWebEditor(cmd, profile, tmpl, store)
			}
			return runTUIEditor(cmd, profile, tmpl, store)
		},
	}

	cmd.Flags().BoolVar(&webFlag, "web", false, "open the editor in your browser")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runTUIEditor(cmd *cobra.Command, profile string, tmpl domain.Template, store *tmplsvc.Store) error {
	sheet := templateToSheet(tmpl)
	m := editor.New(sheet)
	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("editor: %w", err)
	}

	final, _ := result.(editor.Model)
	if !final.Saved() {
		return nil
	}

	applySheetToTemplate(final.Sheet(), &tmpl)
	tmpl.ModifiedAt = time.Now().UTC()
	if err := store.Save(profile, tmpl); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "saved template %q\n", tmpl.Name)
	return nil
}

func runWebEditor(cmd *cobra.Command, profile string, tmpl domain.Template, store *tmplsvc.Store) error {
	sheet := templateToSheet(tmpl)

	saveFn := func(s editor.Sheet) error {
		applySheetToTemplate(s, &tmpl)
		tmpl.ModifiedAt = time.Now().UTC()
		return store.Save(profile, tmpl)
	}

	res, err := webeditor.Run(sheet, saveFn)
	if err != nil {
		return fmt.Errorf("web editor: %w", err)
	}

	if res.Saved {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "saved template %q\n", tmpl.Name)
	}
	return nil
}

// templateToSheet builds a Sheet from a template, with rows sorted by
// (GroupName, then Label or DisplayRef) for stable display order.
func templateToSheet(t domain.Template) editor.Sheet {
	rows := make([]editor.SheetRow, 0, len(t.Rows))
	for _, r := range t.Rows {
		rows = append(rows, editor.SheetRow{
			ID:         r.ID,
			Label:      r.Label,
			GroupName:  r.Target.GroupName,
			DisplayRef: r.Target.DisplayRef,
			TypeName:   r.TimeType.Name,
			Hours:      r.Hours,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].GroupName != rows[j].GroupName {
			return rows[i].GroupName < rows[j].GroupName
		}
		li := rows[i].Label
		if li == "" {
			li = rows[i].DisplayRef
		}
		lj := rows[j].Label
		if lj == "" {
			lj = rows[j].DisplayRef
		}
		return li < lj
	})
	return editor.Sheet{Name: t.Name, Rows: rows}
}

// applySheetToTemplate copies hours from sheet rows back into t.Rows
// (matched by row ID). Mutates t in place. Rows in t that aren't in the
// sheet are untouched (defensive — shouldn't happen since the editor can't
// drop rows).
func applySheetToTemplate(sheet editor.Sheet, t *domain.Template) {
	hoursByID := make(map[string]domain.WeekHours, len(sheet.Rows))
	for _, r := range sheet.Rows {
		hoursByID[r.ID] = r.Hours
	}
	for i := range t.Rows {
		if h, ok := hoursByID[t.Rows[i].ID]; ok {
			t.Rows[i].Hours = h
		}
	}
}
```

- [ ] **Step 4.2 — Update template/edit_test.go**

Read the existing `internal/cli/time/template/edit_test.go` and adapt any tests that reference the editor's old constructor or Template-based call shape. Add a unit test for the new adapter:

```go
func TestTemplateToSheet_RoundTrip(t *testing.T) {
	tmpl := domain.Template{
		Name: "demo",
		Rows: []domain.TemplateRow{
			{ID: "row-01", Label: "B", Target: domain.Target{GroupName: "Group A"}, TimeType: domain.TimeType{Name: "Type"}, Hours: domain.WeekHours{Mon: 4}},
			{ID: "row-02", Label: "A", Target: domain.Target{GroupName: "Group A"}, TimeType: domain.TimeType{Name: "Type"}, Hours: domain.WeekHours{Tue: 2}},
			{ID: "row-03", Label: "C", Target: domain.Target{GroupName: "Group B"}, TimeType: domain.TimeType{Name: "Type"}, Hours: domain.WeekHours{Wed: 1}},
		},
	}

	sheet := templateToSheet(tmpl)
	require.Equal(t, "demo", sheet.Name)
	// Sorted by GroupName then Label: Group A:A, Group A:B, Group B:C
	require.Equal(t, []string{"row-02", "row-01", "row-03"}, []string{sheet.Rows[0].ID, sheet.Rows[1].ID, sheet.Rows[2].ID})

	// Edit a sheet row's hours, apply back.
	sheet.Rows[0].Hours.Tue = 99
	applySheetToTemplate(sheet, &tmpl)

	// row-02 (Label "A") should now have Tue=99.
	for _, r := range tmpl.Rows {
		if r.ID == "row-02" {
			require.InDelta(t, 99.0, r.Hours.Tue, 0.001)
		}
	}
}
```

Add `"github.com/stretchr/testify/require"` and `"github.com/iainmoffat/tdx/internal/domain"` to imports if not already present.

- [ ] **Step 4.3 — Run template tests + full build**

```bash
go build ./... && go test ./internal/cli/time/template/ -count=1 -v 2>&1 | tail -20
```

Expected: build succeeds; all template tests pass.

- [ ] **Step 4.4 — Commit**

```bash
git add internal/cli/time/template/edit.go internal/cli/time/template/edit_test.go
git commit -m "refactor(template): edit threads template through Sheet adapter"
```

---

## Task 5: Rewrite week edit.go to use the grid editor

**Files:**
- Modify: `internal/cli/time/week/edit.go`
- Modify: `internal/cli/time/week/edit_test.go`

- [ ] **Step 5.1 — Failing tests for week edit adapters**

Replace `internal/cli/time/week/edit_test.go` contents with adapter-focused tests covering each row of the spec §8 cell-mapping table:

```go
package week

import (
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/tui/editor"
	"github.com/stretchr/testify/require"
)

func TestNewEditCmd_FlagsRegistered(t *testing.T) {
	cmd := newEditCmd()
	require.NotNil(t, cmd)
	require.Equal(t, "edit [date[/name]]", cmd.Use)
	require.NotNil(t, cmd.Flags().Lookup("web"), "--web flag missing")
	require.NotNil(t, cmd.Flags().Lookup("profile"), "--profile flag missing")
	require.NoError(t, cmd.Args(cmd, []string{}), "should accept zero args")
	require.NoError(t, cmd.Args(cmd, []string{"2026-05-04"}), "should accept one arg")
}

func TestDraftToSheet_DenseHoursFromSparseCells(t *testing.T) {
	weekStart := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	draft := domain.WeekDraft{
		Name: "default", WeekStart: weekStart,
		Rows: []domain.DraftRow{
			{
				ID:    "row-01",
				Label: "Admin",
				Target: domain.Target{
					GroupName: "UFIT Administration", DisplayRef: "plan/2075/task/2076",
				},
				TimeType: domain.TimeType{Name: "Standard Activities"},
				Cells: []domain.DraftCell{
					{Day: time.Monday, Hours: 4, SourceEntryID: 100},
					{Day: time.Tuesday, Hours: 2}, // local-only addition
				},
			},
		},
	}

	sheet := draftToSheet(draft)
	require.Equal(t, "default", sheet.Name)
	require.Len(t, sheet.Rows, 1)
	require.Equal(t, "row-01", sheet.Rows[0].ID)
	require.Equal(t, "Admin", sheet.Rows[0].Label)
	require.Equal(t, "UFIT Administration", sheet.Rows[0].GroupName)
	require.Equal(t, "Standard Activities", sheet.Rows[0].TypeName)
	require.InDelta(t, 4.0, sheet.Rows[0].Hours.Mon, 0.001)
	require.InDelta(t, 2.0, sheet.Rows[0].Hours.Tue, 0.001)
	require.InDelta(t, 0.0, sheet.Rows[0].Hours.Wed, 0.001, "absent cells map to 0")
}

func TestApplySheetToDraft_AddNewCell(t *testing.T) {
	draft := domain.WeekDraft{
		Rows: []domain.DraftRow{
			{ID: "row-01", Cells: []domain.DraftCell{
				{Day: time.Monday, Hours: 4, SourceEntryID: 100},
			}},
		},
	}
	sheet := editor.Sheet{
		Rows: []editor.SheetRow{
			{ID: "row-01", Hours: domain.WeekHours{Mon: 4, Wed: 3}},
		},
	}
	applySheetToDraft(sheet, &draft)

	cells := draft.Rows[0].Cells
	require.Len(t, cells, 2)
	// Cells sorted by Day.
	require.Equal(t, time.Monday, cells[0].Day)
	require.InDelta(t, 4.0, cells[0].Hours, 0.001)
	require.Equal(t, 100, cells[0].SourceEntryID)
	require.Equal(t, time.Wednesday, cells[1].Day)
	require.InDelta(t, 3.0, cells[1].Hours, 0.001)
	require.Equal(t, 0, cells[1].SourceEntryID, "new cell has no source ID")
}

func TestApplySheetToDraft_PreserveSourceIDOnZero(t *testing.T) {
	draft := domain.WeekDraft{
		Rows: []domain.DraftRow{
			{ID: "row-01", Cells: []domain.DraftCell{
				{Day: time.Monday, Hours: 4, SourceEntryID: 100},
			}},
		},
	}
	sheet := editor.Sheet{
		Rows: []editor.SheetRow{
			{ID: "row-01", Hours: domain.WeekHours{Mon: 0}},
		},
	}
	applySheetToDraft(sheet, &draft)

	require.Len(t, draft.Rows[0].Cells, 1)
	require.InDelta(t, 0.0, draft.Rows[0].Cells[0].Hours, 0.001)
	require.Equal(t, 100, draft.Rows[0].Cells[0].SourceEntryID,
		"zeroing a pulled cell preserves SourceEntryID (delete-on-push)")
}

func TestApplySheetToDraft_DropLocalOnlyCellOnZero(t *testing.T) {
	draft := domain.WeekDraft{
		Rows: []domain.DraftRow{
			{ID: "row-01", Cells: []domain.DraftCell{
				{Day: time.Monday, Hours: 4, SourceEntryID: 100},
				{Day: time.Tuesday, Hours: 2}, // local-only add
			}},
		},
	}
	sheet := editor.Sheet{
		Rows: []editor.SheetRow{
			{ID: "row-01", Hours: domain.WeekHours{Mon: 4, Tue: 0}},
		},
	}
	applySheetToDraft(sheet, &draft)

	require.Len(t, draft.Rows[0].Cells, 1, "local-only cell zeroed → dropped")
	require.Equal(t, time.Monday, draft.Rows[0].Cells[0].Day)
	require.Equal(t, 100, draft.Rows[0].Cells[0].SourceEntryID)
}

func TestApplySheetToDraft_PreservePerCellMetadata(t *testing.T) {
	desc := "review meeting"
	draft := domain.WeekDraft{
		Rows: []domain.DraftRow{
			{ID: "row-01", Cells: []domain.DraftCell{
				{
					Day: time.Monday, Hours: 4, SourceEntryID: 100,
					PerCell: &domain.PerCell{Description: &desc},
				},
			}},
		},
	}
	sheet := editor.Sheet{
		Rows: []editor.SheetRow{
			{ID: "row-01", Hours: domain.WeekHours{Mon: 6}},
		},
	}
	applySheetToDraft(sheet, &draft)

	cell := draft.Rows[0].Cells[0]
	require.InDelta(t, 6.0, cell.Hours, 0.001)
	require.NotNil(t, cell.PerCell)
	require.NotNil(t, cell.PerCell.Description)
	require.Equal(t, "review meeting", *cell.PerCell.Description, "PerCell metadata preserved")
}

func TestApplySheetToDraft_ZeroAbsentCellNoOp(t *testing.T) {
	draft := domain.WeekDraft{
		Rows: []domain.DraftRow{
			{ID: "row-01", Cells: []domain.DraftCell{
				{Day: time.Monday, Hours: 4, SourceEntryID: 100},
			}},
		},
	}
	sheet := editor.Sheet{
		Rows: []editor.SheetRow{
			{ID: "row-01", Hours: domain.WeekHours{Mon: 4, Wed: 0}},
		},
	}
	applySheetToDraft(sheet, &draft)

	require.Len(t, draft.Rows[0].Cells, 1, "absent cell + zero hours = no-op")
}
```

- [ ] **Step 5.2 — Run tests to verify they fail**

Run: `go test ./internal/cli/time/week/ -run 'TestDraftToSheet|TestApplySheetToDraft|TestNewEditCmd_FlagsRegistered' -v 2>&1 | tail -10`
Expected: FAIL — `draftToSheet` / `applySheetToDraft` undefined; new edit command shape differs.

- [ ] **Step 5.3 — Replace edit.go with the grid-editor version**

Overwrite `internal/cli/time/week/edit.go`:

```go
package week

import (
	"fmt"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
	"github.com/iainmoffat/tdx/internal/tui/editor"
	webeditor "github.com/iainmoffat/tdx/internal/web/editor"
)

type editFlags struct {
	profile string
	web     bool
}

func newEditCmd() *cobra.Command {
	var f editFlags
	cmd := &cobra.Command{
		Use:   "edit [date[/name]]",
		Short: "Edit a draft in an interactive grid (defaults to the current week)",
		Long: `Edit a draft's hours in an interactive grid (the same editor used by ` +
			"`tdx time template edit`" + `).

Use --web to open the editor in your browser instead of the terminal.

Only hours within existing rows can be edited. To add or remove rows, use
` + "`tdx time week new --from-template`" + ` or ` + "`tdx time week set`" + `.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := ""
			if len(args) > 0 {
				ref = args[0]
			}
			return runEdit(cmd, f, ref)
		},
	}
	cmd.Flags().BoolVar(&f.web, "web", false, "open the editor in your browser")
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
	return cmd
}

func runEdit(cmd *cobra.Command, f editFlags, ref string) error {
	weekStart, name, err := ResolveWeekRef(ref)
	if err != nil {
		return err
	}

	paths, err := config.ResolvePaths()
	if err != nil {
		return err
	}
	auth := authsvc.New(paths)
	tsvc := timesvc.New(paths)
	drafts := draftsvc.NewService(paths, tsvc)

	profileName, err := auth.ResolveProfile(f.profile)
	if err != nil {
		return err
	}

	d, err := drafts.Store().Load(profileName, weekStart, name)
	if err != nil {
		return err
	}

	if f.web {
		return runWebEditor(cmd, drafts, d)
	}
	return runTUIEditor(cmd, drafts, d)
}

func runTUIEditor(cmd *cobra.Command, drafts *draftsvc.Service, d domain.WeekDraft) error {
	sheet := draftToSheet(d)
	m := editor.New(sheet)
	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("editor: %w", err)
	}

	final, _ := result.(editor.Model)
	if !final.Saved() {
		return nil
	}

	applySheetToDraft(final.Sheet(), &d)
	d.ModifiedAt = time.Now().UTC()
	if err := drafts.Store().Save(d); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Saved draft %s/%s.\n",
		d.WeekStart.Format("2006-01-02"), d.Name)
	return nil
}

func runWebEditor(cmd *cobra.Command, drafts *draftsvc.Service, d domain.WeekDraft) error {
	sheet := draftToSheet(d)
	saveFn := func(s editor.Sheet) error {
		applySheetToDraft(s, &d)
		d.ModifiedAt = time.Now().UTC()
		return drafts.Store().Save(d)
	}
	res, err := webeditor.Run(sheet, saveFn)
	if err != nil {
		return fmt.Errorf("web editor: %w", err)
	}
	if res.Saved {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Saved draft %s/%s.\n",
			d.WeekStart.Format("2006-01-02"), d.Name)
	}
	return nil
}

// draftToSheet builds a Sheet from a draft, with rows in display order
// (group, then label) and dense WeekHours assembled from sparse cells.
func draftToSheet(d domain.WeekDraft) editor.Sheet {
	rows := make([]editor.SheetRow, 0, len(d.Rows))
	for _, r := range d.Rows {
		var h domain.WeekHours
		for _, c := range r.Cells {
			h.SetDay(c.Day, c.Hours)
		}
		rows = append(rows, editor.SheetRow{
			ID:         r.ID,
			Label:      r.Label,
			GroupName:  r.Target.GroupName,
			DisplayRef: r.Target.DisplayRef,
			TypeName:   r.TimeType.Name,
			Hours:      h,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].GroupName != rows[j].GroupName {
			return rows[i].GroupName < rows[j].GroupName
		}
		li := rows[i].Label
		if li == "" {
			li = rows[i].DisplayRef
		}
		lj := rows[j].Label
		if lj == "" {
			lj = rows[j].DisplayRef
		}
		return li < lj
	})
	return editor.Sheet{Name: d.Name, Rows: rows}
}

// applySheetToDraft writes hours from sheet rows back into draft cells,
// preserving SourceEntryID and PerCell metadata. Per spec §8:
//   - cell exists, hours unchanged → no-op
//   - cell exists with SourceEntryID, hours=0 → set to 0 (delete-on-push)
//   - cell exists without SourceEntryID, hours=0 → drop the cell
//   - cell exists, hours>0 → update Hours (preserves PerCell)
//   - cell absent, hours=0 → no-op
//   - cell absent, hours>0 → add new {Day, Hours, SourceEntryID=0}
//
// Cells in each row are sorted by Day after the apply.
func applySheetToDraft(sheet editor.Sheet, d *domain.WeekDraft) {
	hoursByID := make(map[string]domain.WeekHours, len(sheet.Rows))
	for _, r := range sheet.Rows {
		hoursByID[r.ID] = r.Hours
	}

	for ri := range d.Rows {
		row := &d.Rows[ri]
		newHours, ok := hoursByID[row.ID]
		if !ok {
			continue
		}

		cellsByDay := make(map[time.Weekday]int, len(row.Cells))
		for ci, c := range row.Cells {
			cellsByDay[c.Day] = ci
		}

		var rebuilt []domain.DraftCell
		for ci := 0; ci < 7; ci++ {
			wd := time.Weekday(ci)
			h := newHours.ForDay(wd)
			idx, exists := cellsByDay[wd]
			switch {
			case exists:
				cell := row.Cells[idx]
				if h == 0 && cell.SourceEntryID == 0 {
					continue // drop local-only cell
				}
				cell.Hours = h
				rebuilt = append(rebuilt, cell)
			case !exists && h > 0:
				rebuilt = append(rebuilt, domain.DraftCell{Day: wd, Hours: h})
			}
		}
		sort.SliceStable(rebuilt, func(i, j int) bool {
			return rebuilt[i].Day < rebuilt[j].Day
		})
		row.Cells = rebuilt
	}
}
```

- [ ] **Step 5.4 — Run tests to verify they pass**

Run: `go test ./internal/cli/time/week/ -run 'TestDraftToSheet|TestApplySheetToDraft|TestNewEditCmd_FlagsRegistered' -count=1 -v 2>&1 | tail -15`
Expected: PASS — all 7 sub-tests.

- [ ] **Step 5.5 — Run full module tests**

Run: `go test ./... -count=1 2>&1 | grep -E "FAIL|ok\s" | tail -15`
Expected: every package green.

- [ ] **Step 5.6 — Commit**

```bash
git add internal/cli/time/week/edit.go internal/cli/time/week/edit_test.go
git commit -m "feat(week): edit uses grid editor (TUI + --web), drops YAML flow"
```

---

## Task 6: Docs + version bump + final verification

**Files:**
- Modify: `README.md`
- Modify: `docs/guide.md`

- [ ] **Step 6.1 — Update README.md**

Locate the Time Week Drafts table (around lines 84–106). Change the `edit` row description and key flags:

```diff
-| `tdx time week edit [date[/name]]` | Edit a draft as YAML in $EDITOR | (vi fallback) |
+| `tdx time week edit [date[/name]]` | Edit a draft in an interactive grid | `--web`, `--profile` |
```

- [ ] **Step 6.2 — Update docs/guide.md**

Search `docs/guide.md` for narrative about week edit being a YAML-in-`$EDITOR` flow. Replace with a brief note that the grid editor matches `tdx time template edit`. Specifically check the "Editing" subsection under "Week drafts".

```bash
grep -n "EDITOR\|YAML\|edit" docs/guide.md | head
```

For any line that describes YAML-text editing, replace with text like:
"`tdx time week edit` opens the same interactive grid used by `tdx time template edit`. Use `--web` for a browser-based editor. Only hours within existing rows can be edited."

- [ ] **Step 6.3 — Sanity check by building and running --help**

```bash
go build -o /tmp/tdx ./cmd/tdx && /tmp/tdx time week edit --help | head -10
```

Expected: `--web` and `--profile` flags listed; no mention of `$EDITOR` or YAML.

- [ ] **Step 6.4 — Full quality gate**

```bash
go test ./... && go vet ./... && gofmt -l . && golangci-lint run ./...
```

Expected: all green; `gofmt -l .` empty; `golangci-lint` 0 issues.

- [ ] **Step 6.5 — Commit docs**

```bash
git add README.md docs/guide.md
git commit -m "docs: README + guide.md — week edit uses interactive grid"
```

- [ ] **Step 6.6 — Push branch + open PR**

```bash
git push -u origin week-edit-grid
gh pr create --title "feat(week): edit uses grid editor (parity with template edit)" --body "$(cat <<'EOF'
## Summary
- Generalized internal/tui/editor and internal/web/editor to operate on a Sheet abstraction
- tdx time week edit now uses the same grid editor as tdx time template edit (TUI + --web)
- Drops the YAML-in-EDITOR flow
- Cell metadata (SourceEntryID, PerCell) preserved across draft↔sheet round-trip per spec §8

## Spec
docs/specs/2026-04-28-week-edit-grid-editor.md

## Out of scope
- Row add/remove via editor (use new --from-template or set instead)
- PerCell metadata edits (Phase C)

## Test plan
- [x] go test ./...
- [x] go vet ./...
- [x] gofmt -l .
- [x] golangci-lint run ./...
- [x] tdx time week edit --help shows interactive grid + --web
EOF
)"
```

- [ ] **Step 6.7 — After PR merges: tag v0.7.0**

This is a user-visible behavior change (different editor surface) and a small API change in internal editor packages — minor version bump.

```bash
git checkout main
git pull
git tag -a v0.7.0 -m "feat(week): edit uses grid editor (parity with template edit)"
git push origin v0.7.0
```

Goreleaser auto-publishes.

---

## Notes for the implementer

- **Task 2 + Task 3 + Task 4 are a chain** — Task 2's commit intentionally breaks the build; Tasks 3 and 4 land the fixes in adjacent commits. If you're using subagent-driven execution, dispatch them serially without pushing between.
- **Sort order moved from editor to adapter.** Both `templateToSheet` and `draftToSheet` sort by GroupName then Label. The editor takes rows in the order it's given.
- **`(kind)` suffix dropped** from ungrouped row labels in the editor view — minor cosmetic loss, acceptable.
- **Per-cell metadata** is preserved by the apply function. The editor never sees PerCell.
- **Identity guards** (profile/weekStart/name not editable) are inherent to the new flow — those fields aren't exposed to the editor at all.
- **MCP unaffected** — week edit is CLI-only; `update_week_draft` MCP tool is untouched.
