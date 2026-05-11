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
	m = sendKey(m, tea.KeyRight) // Mon
	m = typeAndCommit(m, "5")    // Mon=5, cursor advances to Tue
	require.True(t, m.dirty)
	m = sendKey(m, tea.KeyLeft) // back to Mon
	m = typeAndCommit(m, "8")   // Mon=8 (original), dirty clears
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
			{ID: "row-01", Label: "Admin Task", GroupName: "Sample Department Administration", TypeName: "Standard", Hours: domain.WeekHours{Mon: 8.0}},
			{ID: "row-03", Label: "Prof Dev", GroupName: "Sample Department Administration", TypeName: "Training"},
			{ID: "row-04", Label: "Docker", GroupName: "Sample Operations", TypeName: "Standard", Hours: domain.WeekHours{Tue: 1.0}},
			{ID: "row-02", Label: "Linux", GroupName: "Sample Operations", TypeName: "Standard", Hours: domain.WeekHours{Mon: 1.0}},
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
