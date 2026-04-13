package editor

import (
	"sort"
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
	return time.Weekday(c.col)
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
// Rows are sorted into display order (grouped by GroupName, then by Label)
// so that cursor navigation matches the visual layout.
func New(name string, rows []domain.TemplateRow) Model {
	// Sort into display order: grouped rows by GroupName then Label,
	// ungrouped rows by Label. This matches the view's grouping logic.
	sorted := make([]domain.TemplateRow, len(rows))
	copy(sorted, rows)
	sort.SliceStable(sorted, func(i, j int) bool {
		gi, gj := sorted[i].Target.GroupName, sorted[j].Target.GroupName
		if gi != gj {
			return gi < gj
		}
		li, lj := rowSortLabel(sorted[i]), rowSortLabel(sorted[j])
		return li < lj
	})

	orig := make([]domain.WeekHours, len(sorted))
	for i, r := range sorted {
		orig[i] = r.Hours
	}
	return Model{
		name:     name,
		rows:     sorted,
		original: orig,
	}
}

func rowSortLabel(r domain.TemplateRow) string {
	if r.Label != "" {
		return r.Label
	}
	return r.Target.DisplayRef
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
