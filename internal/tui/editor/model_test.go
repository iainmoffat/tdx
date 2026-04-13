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
	return updated.(Model)
}

func sendRune(m Model, r rune) Model {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return updated.(Model)
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

func TestModel_NudgeUp(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyRight) // Mon = 8.0
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
	m = sendRune(m, '+')
	require.True(t, m.dirty)
}

func TestModel_SaveExit(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyRight)
	m = sendRune(m, '+')
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(Model)
	require.True(t, m.saved)
	require.True(t, m.quitting)
	require.NotNil(t, cmd)
}

func TestModel_CancelClean(t *testing.T) {
	m := New("test", testRows())
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	require.True(t, m.quitting)
	require.False(t, m.saved)
	require.NotNil(t, cmd)
}

func TestModel_CancelDirtyPrompt(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyRight)
	m = sendRune(m, '+')
	m = sendKey(m, tea.KeyEsc)
	require.True(t, m.confirm)
	require.False(t, m.quitting)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(Model)
	require.True(t, m.quitting)
	require.False(t, m.saved)
	require.NotNil(t, cmd)
}

func TestModel_CancelDirtyDeny(t *testing.T) {
	m := New("test", testRows())
	m = sendKey(m, tea.KeyRight)
	m = sendRune(m, '+')
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
