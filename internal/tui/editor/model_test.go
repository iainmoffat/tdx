package editor

import (
	"testing"

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
	result, _ := updated.(Model)
	return result
}

func sendRune(m Model, r rune) Model {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	result, _ := updated.(Model)
	return result
}

// typeAndCommit types a value and presses Enter to commit it.
func typeAndCommit(m Model, val string) Model {
	for _, r := range val {
		m = sendRune(m, r)
	}
	m = sendKey(m, tea.KeyEnter)
	return m
}

func TestModel_InitialCursor(t *testing.T) {
	m := New("test", testRows())
	require.Equal(t, 0, m.cursor.row)
	require.Equal(t, 0, m.cursor.col)
	require.False(t, m.dirty)
}

func TestModel_NavigateRight(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyRight)
	require.Equal(t, 0, m.cursor.row)
	require.Equal(t, 1, m.cursor.col)
}

func TestModel_NavigateDown(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyDown)
	require.Equal(t, 1, m.cursor.row)
	require.Equal(t, 0, m.cursor.col)
}

func TestModel_WrapRight(t *testing.T) {
	m := New("test", testRows())
	for i := 0; i < 6; i++ {
		m = sendKey(m, tea.KeyRight)
	}
	require.Equal(t, 6, m.cursor.col)
	m = sendKey(m, tea.KeyRight)
	require.Equal(t, 1, m.cursor.row)
	require.Equal(t, 0, m.cursor.col)
}

func TestModel_WrapLeft(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyDown)
	require.Equal(t, 1, m.cursor.row)
	m = sendKey(m, tea.KeyLeft)
	require.Equal(t, 0, m.cursor.row)
	require.Equal(t, 6, m.cursor.col)
}

func TestModel_ClampTop(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyUp)
	require.Equal(t, 0, m.cursor.row)
}

func TestModel_ClampBottom(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyDown)
	m = sendKey(m, tea.KeyDown)
	require.Equal(t, 1, m.cursor.row)
}

func TestModel_TypeValue(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyRight) // Mon
	m = sendRune(m, '4')
	require.True(t, m.typing)
	m = sendKey(m, tea.KeyEnter)
	require.False(t, m.typing)
	require.InDelta(t, 4.0, m.rows[0].Hours.Mon, 0.001)
	require.True(t, m.dirty)
}

func TestModel_TypeReplacesExisting(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyRight) // Mon = 8.0
	// Type "2" — replaces, doesn't append to "8"
	m = sendRune(m, '2')
	m = sendKey(m, tea.KeyEnter)
	require.InDelta(t, 2.0, m.rows[0].Hours.Mon, 0.001)
}

func TestModel_TypeSnaps(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyRight) // Mon
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
	m = typeAndCommit(m, "4")    // change to 4.0
	require.True(t, m.dirty)
}

func TestModel_DirtyRevert(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyRight) // Mon = 8.0
	m = typeAndCommit(m, "4")    // change to 4.0
	require.True(t, m.dirty)
	// Navigate back and restore original value
	m = sendKey(m, tea.KeyLeft)
	m = typeAndCommit(m, "8")
	require.False(t, m.dirty)
}

func TestModel_SaveExit(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyRight)
	m = typeAndCommit(m, "4") // make dirty
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m, _ = updated.(Model)
	require.True(t, m.saved)
	require.True(t, m.quitting)
	require.NotNil(t, cmd)
}

func TestModel_CancelClean(t *testing.T) {
	m := New("test", testRows())
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m, _ = updated.(Model)
	require.True(t, m.quitting)
	require.False(t, m.saved)
	require.NotNil(t, cmd)
}

func TestModel_CancelDirtyPrompt(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyRight)
	m = typeAndCommit(m, "4") // make dirty
	m = sendKey(m, tea.KeyEsc)
	require.True(t, m.confirm)
	require.False(t, m.quitting)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m, _ = updated.(Model)
	require.True(t, m.quitting)
	require.False(t, m.saved)
	require.NotNil(t, cmd)
}

func TestModel_CancelDirtyDeny(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyRight)
	m = typeAndCommit(m, "4") // make dirty
	m = sendKey(m, tea.KeyEsc)
	require.True(t, m.confirm)
	m = sendRune(m, 'n')
	require.False(t, m.confirm)
	require.False(t, m.quitting)
}

func TestModel_TabWraps(t *testing.T) {
	m := New("test", testRows())
	for i := 0; i < 7; i++ {
		m = sendKey(m, tea.KeyTab)
	}
	require.Equal(t, 1, m.cursor.row)
	require.Equal(t, 0, m.cursor.col)
}

func TestModel_CtrlS_WhileTyping(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyRight) // Mon
	m = sendRune(m, '4')         // start typing
	require.True(t, m.typing)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m, _ = updated.(Model)
	require.True(t, m.saved)
	require.True(t, m.quitting)
	require.InDelta(t, 4.0, m.rows[0].Hours.Mon, 0.001)
	require.NotNil(t, cmd)
}

func TestModel_Esc_WhileTyping(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyRight) // Mon = 8.0
	m = sendRune(m, '3')         // start typing
	require.True(t, m.typing)
	m = sendKey(m, tea.KeyEsc) // discard
	require.False(t, m.typing)
	require.InDelta(t, 8.0, m.rows[0].Hours.Mon, 0.001) // unchanged
	require.False(t, m.dirty)
}

func TestModel_QuitKey_Clean(t *testing.T) {
	m := New("test", testRows())
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m, _ = updated.(Model)
	require.True(t, m.quitting)
	require.False(t, m.saved)
	require.NotNil(t, cmd)
}

func TestModel_QuitKey_Dirty(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyRight)
	m = typeAndCommit(m, "4") // make dirty
	m = sendRune(m, 'q')
	require.True(t, m.confirm)
	require.False(t, m.quitting)
}

// testGroupedRows returns rows from multiple groups in a deliberately
// scrambled order, simulating what derive produces (sorted by total hours
// descending, not by group).
func testGroupedRows() []domain.TemplateRow {
	return []domain.TemplateRow{
		{
			ID:       "row-01",
			Label:    "Admin Task",
			Target:   domain.Target{GroupName: "UFIT Administration"},
			TimeType: domain.TimeType{ID: 5, Name: "Standard"},
			Hours:    domain.WeekHours{Mon: 8.0},
		},
		{
			ID:       "row-02",
			Label:    "Linux",
			Target:   domain.Target{GroupName: "UFIT Operations"},
			TimeType: domain.TimeType{ID: 5, Name: "Standard"},
			Hours:    domain.WeekHours{Mon: 1.0},
		},
		{
			ID:       "row-03",
			Label:    "Prof Dev",
			Target:   domain.Target{GroupName: "UFIT Administration"},
			TimeType: domain.TimeType{ID: 6, Name: "Training"},
		},
		{
			ID:       "row-04",
			Label:    "Docker",
			Target:   domain.Target{GroupName: "UFIT Operations"},
			TimeType: domain.TimeType{ID: 5, Name: "Standard"},
			Hours:    domain.WeekHours{Tue: 1.0},
		},
	}
}

func TestModel_GroupedNavigation_DownVisitsDisplayOrder(t *testing.T) {
	m := New("test", testGroupedRows())

	// After sorting, rows should be grouped:
	// UFIT Administration: Admin Task, Prof Dev
	// UFIT Operations: Docker, Linux
	// Verify by navigating down and collecting row IDs.
	var visited []string
	for i := 0; i < len(m.rows); i++ {
		visited = append(visited, m.rows[m.cursor.row].ID)
		m = sendKey(m, tea.KeyDown)
	}

	// Expected: grouped by GroupName, then sorted by Label within each group.
	require.Equal(t, []string{"row-01", "row-03", "row-04", "row-02"}, visited,
		"navigation should follow display order: UFIT Administration (Admin Task, Prof Dev), UFIT Operations (Docker, Linux)")
}

func TestModel_GroupedRows_PreservesAfterEdit(t *testing.T) {
	m := New("test", testGroupedRows())

	// First row should be Admin Task (Mon=8.0)
	require.Equal(t, "row-01", m.rows[0].ID)

	// Edit it
	m = sendKey(m, tea.KeyRight) // Mon
	m = typeAndCommit(m, "4")
	require.InDelta(t, 4.0, m.rows[0].Hours.Mon, 0.001)

	// Save returns rows in display order
	rows := m.Rows()
	require.Equal(t, "row-01", rows[0].ID)
	require.InDelta(t, 4.0, rows[0].Hours.Mon, 0.001)
}
