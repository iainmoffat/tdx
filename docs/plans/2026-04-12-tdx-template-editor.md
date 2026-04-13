# Template Editor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `tdx time template edit <name>` — an interactive terminal grid editor for adjusting template hour values.

**Architecture:** New `internal/tui/editor` package implements a bubbletea Model that renders a navigable grid of hour cells. The CLI command in `internal/cli/time/template/edit.go` loads the template, runs the TUI, and saves on exit. The editor only modifies `WeekHours` per row — no row add/delete or description editing.

**Tech Stack:** Go 1.26+, bubbletea (Charm TUI framework), lipgloss (styling). Existing: Cobra, domain.Template, tmplsvc.Store.

**Design spec:** `docs/specs/2026-04-12-tdx-template-editor-design.md`

---

## File Structure

| File | Responsibility |
|---|---|
| Create: `internal/tui/editor/model.go` | Bubbletea Model: state, cursor, edit buffer, Init/Update |
| Create: `internal/tui/editor/view.go` | View function: grid rendering with active cell highlight, status bar |
| Create: `internal/tui/editor/snap.go` | Value snapping (round to 0.5, clamp 0–24) |
| Create: `internal/tui/editor/model_test.go` | Unit tests for model Update logic |
| Create: `internal/tui/editor/snap_test.go` | Unit tests for snap/clamp logic |
| Create: `internal/cli/time/template/edit.go` | Cobra command: load template, run TUI, save result |
| Create: `internal/cli/time/template/edit_test.go` | Test: template not found error |
| Modify: `internal/cli/time/template/template.go` | Register `newEditCmd()` |
| Modify: `README.md` | Add `template edit` to command table |
| Modify: `docs/guide.md` | Add editor section + update workflow intro |

---

## Task 1: Add bubbletea dependency

- [ ] **Step 1: Install bubbletea and lipgloss**

```bash
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/lipgloss@latest
```

- [ ] **Step 2: Verify go.mod updated**

```bash
grep charmbracelet go.mod
```

Expected: lines for `bubbletea` and `lipgloss`.

- [ ] **Step 3: Tidy**

```bash
go mod tidy
```

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add bubbletea and lipgloss for template editor TUI"
```

---

## Task 2: Snap logic

Pure functions with no dependencies on bubbletea. Build and test these first.

- [ ] **Step 1: Write snap tests**

Create `internal/tui/editor/snap_test.go`:

```go
package editor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSnapToHalf(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{0.0, 0.0},
		{0.3, 0.5},
		{0.7, 0.5},
		{0.8, 1.0},
		{1.0, 1.0},
		{1.25, 1.5},
		{1.333, 1.5},
		{2.7, 2.5},
		{2.9, 3.0},
		{8.0, 8.0},
		{24.5, 24.0},
		{-1.0, 0.0},
		{100.0, 24.0},
	}
	for _, tt := range tests {
		got := snapToHalf(tt.input)
		require.InDelta(t, tt.expected, got, 0.001, "snapToHalf(%v)", tt.input)
	}
}

func TestNudge(t *testing.T) {
	require.InDelta(t, 1.0, nudge(0.5, 1), 0.001)   // up
	require.InDelta(t, 0.0, nudge(0.5, -1), 0.001)   // down
	require.InDelta(t, 0.0, nudge(0.0, -1), 0.001)   // clamp at 0
	require.InDelta(t, 24.0, nudge(24.0, 1), 0.001)  // clamp at 24
	require.InDelta(t, 23.5, nudge(24.0, -1), 0.001) // down from max
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/tui/editor/ -run "TestSnap|TestNudge" -count=1
```

Expected: compilation error — functions not defined.

- [ ] **Step 3: Implement snap logic**

Create `internal/tui/editor/snap.go`:

```go
package editor

import "math"

// snapToHalf rounds v to the nearest 0.5 and clamps to [0, 24].
func snapToHalf(v float64) float64 {
	v = math.Round(v*2) / 2
	if v < 0 {
		return 0
	}
	if v > 24 {
		return 24
	}
	return v
}

// nudge adds dir * 0.5 to v and clamps to [0, 24].
// dir should be +1 or -1.
func nudge(v float64, dir int) float64 {
	v += float64(dir) * 0.5
	if v < 0 {
		return 0
	}
	if v > 24 {
		return 24
	}
	return v
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/tui/editor/ -run "TestSnap|TestNudge" -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/editor/snap.go internal/tui/editor/snap_test.go
git commit -m "feat(tui): add snap and nudge functions for template editor"
```

---

## Task 3: Editor model — core state and update logic

- [ ] **Step 1: Write model tests**

Create `internal/tui/editor/model_test.go`:

```go
package editor

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ipm/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func testRows() []domain.TemplateRow {
	return []domain.TemplateRow{
		{
			ID:       "row-01",
			Label:    "Admin",
			TimeType: domain.TimeType{ID: 5, Name: "Dev"},
			Hours:    domain.WeekHours{Mon: 8.0, Tue: 8.0, Wed: 8.0, Thu: 8.0, Fri: 8.0},
		},
		{
			ID:       "row-02",
			Label:    "Project",
			TimeType: domain.TimeType{ID: 6, Name: "Planning"},
			Hours:    domain.WeekHours{Mon: 1.0, Wed: 2.0},
		},
	}
}

func sendKey(m Model, k tea.KeyType) Model {
	updated, _ := m.Update(tea.KeyMsg{Type: k})
	return updated.(Model)
}

func sendRune(m Model, r rune) Model {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return updated.(Model)
}

func TestModel_InitialCursor(t *testing.T) {
	m := New("test", testRows())
	require.Equal(t, 0, m.cursor.row)
	require.Equal(t, 0, m.cursor.col) // Sun
	require.False(t, m.dirty)
}

func TestModel_NavigateRight(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyRight)
	require.Equal(t, 0, m.cursor.row)
	require.Equal(t, 1, m.cursor.col) // Mon
}

func TestModel_NavigateDown(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyDown)
	require.Equal(t, 1, m.cursor.row) // row-02
	require.Equal(t, 0, m.cursor.col)
}

func TestModel_WrapRight(t *testing.T) {
	m := New("test", testRows())
	// Move to Sat (col 6), then right → next row Sun (col 0)
	for i := 0; i < 6; i++ {
		m = sendKey(m, tea.KeyRight)
	}
	require.Equal(t, 6, m.cursor.col) // Sat
	m = sendKey(m, tea.KeyRight)
	require.Equal(t, 1, m.cursor.row)
	require.Equal(t, 0, m.cursor.col) // Sun of row-02
}

func TestModel_WrapLeft(t *testing.T) {
	m := New("test", testRows())
	// Move to row 1 first
	m = sendKey(m, tea.KeyDown)
	require.Equal(t, 1, m.cursor.row)
	// Left from Sun → Sat of previous row
	m = sendKey(m, tea.KeyLeft)
	require.Equal(t, 0, m.cursor.row)
	require.Equal(t, 6, m.cursor.col)
}

func TestModel_ClampTop(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyUp) // already at row 0
	require.Equal(t, 0, m.cursor.row)
}

func TestModel_ClampBottom(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyDown)
	m = sendKey(m, tea.KeyDown) // already at last row
	require.Equal(t, 1, m.cursor.row)
}

func TestModel_NudgeUp(t *testing.T) {
	m := New("test", testRows())
	// Move to Mon (col 1) which has 8.0
	m = sendKey(m, tea.KeyRight)
	// Nudge up → 8.5
	m = sendRune(m, '+')
	require.InDelta(t, 8.5, m.rows[0].Hours.Mon, 0.001)
	require.True(t, m.dirty)
}

func TestModel_NudgeDown(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyRight) // Mon = 8.0
	m = sendRune(m, '-')
	require.InDelta(t, 7.5, m.rows[0].Hours.Mon, 0.001)
	require.True(t, m.dirty)
}

func TestModel_TypeValue(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyRight) // Mon
	// Type "4" then Enter
	m = sendRune(m, '4')
	require.True(t, m.typing)
	m = sendKey(m, tea.KeyEnter)
	require.False(t, m.typing)
	require.InDelta(t, 4.0, m.rows[0].Hours.Mon, 0.001)
	require.True(t, m.dirty)
}

func TestModel_TypeSnaps(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyRight) // Mon
	// Type "1.3" → snaps to 1.5
	m = sendRune(m, '1')
	m = sendRune(m, '.')
	m = sendRune(m, '3')
	m = sendKey(m, tea.KeyEnter)
	require.InDelta(t, 1.5, m.rows[0].Hours.Mon, 0.001)
}

func TestModel_BackspaceClearsCell(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyRight) // Mon = 8.0
	m = sendKey(m, tea.KeyBackspace)
	require.InDelta(t, 0.0, m.rows[0].Hours.Mon, 0.001)
	require.True(t, m.dirty)
}

func TestModel_DirtyDetection(t *testing.T) {
	m := New("test", testRows())
	require.False(t, m.dirty)
	m = sendKey(m, tea.KeyRight) // Mon = 8.0
	m = sendRune(m, '+')         // 8.5
	require.True(t, m.dirty)
}

func TestModel_SaveExit(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyRight)
	m = sendRune(m, '+') // make dirty
	// Ctrl-S
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(Model)
	require.True(t, m.saved)
	require.True(t, m.quitting)
	require.NotNil(t, cmd) // tea.Quit
}

func TestModel_CancelClean(t *testing.T) {
	m := New("test", testRows())
	// Esc on clean model → quit immediately
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	require.True(t, m.quitting)
	require.False(t, m.saved)
	require.NotNil(t, cmd)
}

func TestModel_CancelDirtyPrompt(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyRight)
	m = sendRune(m, '+') // make dirty
	// Esc → shows confirm prompt
	m = sendKey(m, tea.KeyEsc)
	require.True(t, m.confirm)
	require.False(t, m.quitting)
	// 'y' → quit without saving
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(Model)
	require.True(t, m.quitting)
	require.False(t, m.saved)
	require.NotNil(t, cmd)
}

func TestModel_CancelDirtyDeny(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyRight)
	m = sendRune(m, '+') // make dirty
	m = sendKey(m, tea.KeyEsc)
	require.True(t, m.confirm)
	// 'n' → back to editing
	m = sendRune(m, 'n')
	require.False(t, m.confirm)
	require.False(t, m.quitting)
}

func TestModel_TabWraps(t *testing.T) {
	m := New("test", testRows())
	// Tab 7 times → should be on row 1, col 0
	for i := 0; i < 7; i++ {
		m = sendKey(m, tea.KeyTab)
	}
	require.Equal(t, 1, m.cursor.row)
	require.Equal(t, 0, m.cursor.col)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/tui/editor/ -run "TestModel" -count=1
```

Expected: compilation error — `New` function not defined.

- [ ] **Step 3: Implement the model**

Create `internal/tui/editor/model.go`:

```go
package editor

import (
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ipm/tdx/internal/domain"
)

// cursor tracks the active cell position.
type cursor struct {
	row int // 0-based index into rows
	col int // 0=Sun, 1=Mon, ..., 6=Sat
}

// weekday returns the time.Weekday for the current column.
func (c cursor) weekday() time.Weekday {
	return time.Weekday(c.col) // Sun=0 matches time.Sunday=0
}

// Model is the bubbletea model for the template editor.
type Model struct {
	name     string
	rows     []domain.TemplateRow
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

// New creates a new editor Model for the given template rows.
func New(name string, rows []domain.TemplateRow) Model {
	orig := make([]domain.WeekHours, len(rows))
	for i, r := range rows {
		orig[i] = r.Hours
	}
	return Model{
		name:     name,
		rows:     rows,
		original: orig,
	}
}

// Saved reports whether the user chose to save.
func (m Model) Saved() bool { return m.saved }

// Rows returns the (possibly edited) template rows.
func (m Model) Rows() []domain.TemplateRow { return m.rows }

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
		// Quit confirmation mode
		if m.confirm {
			return m.handleConfirm(msg)
		}

		// Typing mode: accumulate digits
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
				switch r {
				case 'q':
					if m.dirty {
						m.confirm = true
						return m, nil
					}
					m.quitting = true
					return m, tea.Quit
				case '+':
					m.nudgeCell(1)
					return m, nil
				case '-':
					m.nudgeCell(-1)
					return m, nil
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

func (m *Model) handleConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'y' {
		m.quitting = true
		return *m, tea.Quit
	}
	m.confirm = false
	return *m, nil
}

func (m *Model) handleTyping(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		m.commitTyping()
		m.moveCell(1)
		return *m, nil
	case tea.KeyEsc:
		m.typing = false
		m.input = ""
		return *m, nil
	case tea.KeyBackspace:
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
		if len(m.input) == 0 {
			m.typing = false
		}
		return *m, nil
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			r := msg.Runes[0]
			if r >= '0' && r <= '9' || r == '.' {
				m.input += string(r)
				return *m, nil
			}
		}
	}
	// Any other key commits and processes normally
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

func (m *Model) nudgeCell(dir int) {
	wd := m.cursor.weekday()
	current := m.rows[m.cursor.row].Hours.ForDay(wd)
	m.rows[m.cursor.row].Hours.SetDay(wd, nudge(current, dir))
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

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/tui/editor/ -run "TestModel" -count=1
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/editor/model.go internal/tui/editor/model_test.go
git commit -m "feat(tui): add editor model with navigation, editing, snap, dirty detection"
```

---

## Task 4: Editor view

- [ ] **Step 1: Implement the view**

Create `internal/tui/editor/view.go`:

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
)

var dayNames = [7]string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}

const cellWidth = 6 // "99.9" + padding

// View implements tea.Model.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Title
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

	// Compute label width
	labelWidth := len("  ROW")
	for _, r := range m.rows {
		label := "  " + m.rowLabel(r)
		if len(label) > labelWidth {
			labelWidth = len(label)
		}
	}

	// Header row
	header := padRight("  ROW", labelWidth)
	for _, d := range dayNames {
		header += "  " + padRight(d, cellWidth-1)
	}
	header += "  TOTAL"
	b.WriteString(header + "\n")

	// Separator
	sepLen := labelWidth + 7*(1+cellWidth) + 2 + 5
	b.WriteString(strings.Repeat("─", sepLen) + "\n")

	// Data rows
	var dayTotals [7]float64
	for ri, r := range m.rows {
		label := "  " + m.rowLabel(r)
		line := padRight(label, labelWidth)
		rowTotal := 0.0
		for ci := 0; ci < 7; ci++ {
			wd := time.Weekday(ci)
			hours := r.Hours.ForDay(wd)
			cell := m.formatCell(ri, ci, hours)
			line += "  " + cell
			dayTotals[ci] += hours
			rowTotal += hours
		}
		line += "  " + padRight(fmt.Sprintf("%.1f", rowTotal), cellWidth-1)
		b.WriteString(strings.TrimRight(line, " ") + "\n")
		// Sub-label
		b.WriteString("    └ " + r.TimeType.Name + "\n")
	}

	// Separator
	b.WriteString(strings.Repeat("─", sepLen) + "\n")

	// Day totals
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

	// Key hints
	b.WriteString("\n")
	hints := "  ←→/Tab: move  +/-/↑↓: adjust ±0.5  type: set value  Ctrl-S: save  Esc: cancel"
	b.WriteString(hintStyle.Render(hints))

	return b.String()
}

func (m Model) rowLabel(r domain.TemplateRow) string {
	label := r.Label
	if label == "" {
		label = r.Target.DisplayRef
	}
	return label + " (" + string(r.Target.Kind) + ")"
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

Note: This `padRight` is local to the `editor` package. The `render` package has its own — they are separate packages and don't share unexported functions.

- [ ] **Step 2: Verify build**

```bash
go build ./internal/tui/editor/
go vet ./internal/tui/editor/
```

Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/editor/view.go
git commit -m "feat(tui): add editor view with grid rendering and active cell highlight"
```

---

## Task 5: CLI command wiring

- [ ] **Step 1: Create the edit command**

Create `internal/cli/time/template/edit.go`:

```go
package template

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/ipm/tdx/internal/config"
	"github.com/ipm/tdx/internal/svc/tmplsvc"
	"github.com/ipm/tdx/internal/tui/editor"
)

func newEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit <name>",
		Short: "Edit template hours in an interactive grid",
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

			m := editor.New(tmpl.Name, tmpl.Rows)
			p := tea.NewProgram(m, tea.WithAltScreen())
			result, err := p.Run()
			if err != nil {
				return fmt.Errorf("editor: %w", err)
			}

			final := result.(editor.Model)
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
		},
	}
}
```

- [ ] **Step 2: Register the command**

Modify `internal/cli/time/template/template.go` — add `newEditCmd()`:

```go
cmd.AddCommand(newEditCmd())
```

Add it after the `newShowCmd()` line.

- [ ] **Step 3: Write a test for template not found**

Create `internal/cli/time/template/edit_test.go`:

```go
package template

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEditCmd_NotFound(t *testing.T) {
	_ = seedTemplateDir(t)
	root := newTestRoot()
	root.SetArgs([]string{"template", "edit", "nonexistent"})
	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}
```

Note: This test only validates the error path. The interactive TUI cannot be tested via Cobra execution — the model tests in Task 3 cover the TUI logic directly.

Check if `newTestRoot()` exists in the test helpers. If not, build the command tree manually:

```go
func TestEditCmd_NotFound(t *testing.T) {
	_ = seedTemplateDir(t)
	cmd := newEditCmd()
	cmd.SetArgs([]string{"nonexistent"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}
```

- [ ] **Step 4: Verify build and test**

```bash
go build ./cmd/tdx
go test ./internal/cli/time/template/ -run TestEditCmd -count=1
go vet ./...
```

Expected: build succeeds, test passes, vet clean.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/time/template/edit.go internal/cli/time/template/edit_test.go internal/cli/time/template/template.go
git commit -m "feat(cli): add tdx time template edit command"
```

---

## Task 6: Documentation updates

- [ ] **Step 1: Update README.md**

Add `template edit` back to the Templates command table. Insert after the `show` row:

```
| `tdx time template edit <name>` | Edit template hours in a grid editor | |
```

- [ ] **Step 2: Update docs/guide.md — workflow intro**

In the Templates section, change the workflow summary from:

> 1. **Derive** a template from a week with known good data
> 2. **Show** or **compare** to verify it looks right
> 3. **Apply** it to future weeks

to:

> 1. **Derive** a template from a week with known good data
> 2. **Edit** hours if needed (optional)
> 3. **Show** or **compare** to verify it looks right
> 4. **Apply** it to future weeks

- [ ] **Step 3: Update docs/guide.md — add editor section**

After the "Show a template" subsection, add:

```markdown
### Edit a template

Open the interactive grid editor to adjust hour values:

```bash
tdx time template edit my-week
```

The editor shows the template as a navigable grid. Use arrow keys or Tab
to move between cells, then adjust values:

| Key | Action |
|-----|--------|
| Arrow keys / Tab | Navigate between day cells |
| `+` / `-` | Increment or decrement by 0.5 hours |
| 0-9, `.` | Type a value directly (snaps to nearest 0.5) |
| Backspace | Clear cell to 0 |
| Enter | Confirm typed value and advance |
| Ctrl-S | Save and exit |
| Esc | Cancel (prompts if unsaved changes) |

Values are constrained to 0.5-hour increments between 0 and 24 hours.
Row totals and day totals update live as you edit.

This is useful for adjusting a derived template before applying it — for
example, reducing Friday hours for a short week, or zeroing out rows you
don't need this time.
```

- [ ] **Step 4: Commit**

```bash
git add README.md docs/guide.md
git commit -m "docs: add template edit to README and user guide"
```

---

## Task 7: Final verification

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

- [ ] **Step 3: Verify build and manual smoke test**

```bash
make build
./tdx time template edit --help
```

Expected: help text shows "Edit template hours in an interactive grid".

- [ ] **Step 4: Verify the editor launches**

If you have an existing template:

```bash
./tdx time template edit my-week
```

Verify: grid displays, navigation works, +/- nudges values, Ctrl-S saves, Esc cancels.

- [ ] **Step 5: Check commit log**

```bash
git log --oneline -7
```

Expected: 6 commits for this feature (deps, snap, model, view, cli, docs).
