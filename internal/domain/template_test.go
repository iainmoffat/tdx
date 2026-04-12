package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// WeekHours tests
// ---------------------------------------------------------------------------

func TestWeekHours_Total(t *testing.T) {
	var wh WeekHours
	wh.SetDay(time.Monday, 3.0)
	wh.SetDay(time.Tuesday, 3.0)
	wh.SetDay(time.Wednesday, 3.0)
	wh.SetDay(time.Thursday, 3.0)
	wh.SetDay(time.Friday, 3.0)
	require.InDelta(t, 15.0, wh.Total(), 1e-9)
}

func TestWeekHours_Total_AllDays(t *testing.T) {
	var wh WeekHours
	wh.SetDay(time.Sunday, 1.0)
	wh.SetDay(time.Monday, 2.0)
	wh.SetDay(time.Tuesday, 3.0)
	wh.SetDay(time.Wednesday, 4.0)
	wh.SetDay(time.Thursday, 5.0)
	wh.SetDay(time.Friday, 6.0)
	wh.SetDay(time.Saturday, 7.0)
	require.InDelta(t, 28.0, wh.Total(), 1e-9)
}

func TestWeekHours_ForDay(t *testing.T) {
	var wh WeekHours
	wh.SetDay(time.Sunday, 0.5)
	wh.SetDay(time.Monday, 1.0)
	wh.SetDay(time.Tuesday, 1.5)
	wh.SetDay(time.Wednesday, 2.0)
	wh.SetDay(time.Thursday, 2.5)
	wh.SetDay(time.Friday, 3.0)
	wh.SetDay(time.Saturday, 3.5)

	require.InDelta(t, 0.5, wh.ForDay(time.Sunday), 1e-9)
	require.InDelta(t, 1.0, wh.ForDay(time.Monday), 1e-9)
	require.InDelta(t, 1.5, wh.ForDay(time.Tuesday), 1e-9)
	require.InDelta(t, 2.0, wh.ForDay(time.Wednesday), 1e-9)
	require.InDelta(t, 2.5, wh.ForDay(time.Thursday), 1e-9)
	require.InDelta(t, 3.0, wh.ForDay(time.Friday), 1e-9)
	require.InDelta(t, 3.5, wh.ForDay(time.Saturday), 1e-9)
}

func TestWeekHours_SetDay(t *testing.T) {
	var wh WeekHours
	wh.SetDay(time.Monday, 2.5)
	wh.SetDay(time.Thursday, 4.0)

	require.InDelta(t, 2.5, wh.ForDay(time.Monday), 1e-9)
	require.InDelta(t, 4.0, wh.ForDay(time.Thursday), 1e-9)
	require.InDelta(t, 6.5, wh.Total(), 1e-9)
}

func TestWeekHours_ToMinutesExact(t *testing.T) {
	var wh WeekHours
	wh.SetDay(time.Monday, 1.5)
	wh.SetDay(time.Tuesday, 0.25)

	mins, ok := wh.ToMinutesExact(time.Monday)
	require.True(t, ok)
	require.Equal(t, 90, mins)

	mins, ok = wh.ToMinutesExact(time.Tuesday)
	require.True(t, ok)
	require.Equal(t, 15, mins)
}

func TestWeekHours_ToMinutesExact_NonInteger(t *testing.T) {
	var wh WeekHours
	wh.SetDay(time.Wednesday, 0.333)

	_, ok := wh.ToMinutesExact(time.Wednesday)
	require.False(t, ok)
}

// ---------------------------------------------------------------------------
// Template.Validate tests
// ---------------------------------------------------------------------------

func TestTemplate_Validate(t *testing.T) {
	makeRow := func(id string) TemplateRow {
		return TemplateRow{ID: id}
	}

	t.Run("valid", func(t *testing.T) {
		tmpl := Template{
			Name: "My Template",
			Rows: []TemplateRow{makeRow("row-1"), makeRow("row-2")},
		}
		require.NoError(t, tmpl.Validate())
	})

	t.Run("empty name", func(t *testing.T) {
		tmpl := Template{
			Name: "",
			Rows: []TemplateRow{makeRow("row-1")},
		}
		require.Error(t, tmpl.Validate())
	})

	t.Run("no rows", func(t *testing.T) {
		tmpl := Template{
			Name: "My Template",
			Rows: nil,
		}
		require.Error(t, tmpl.Validate())
	})

	t.Run("duplicate row IDs", func(t *testing.T) {
		tmpl := Template{
			Name: "My Template",
			Rows: []TemplateRow{makeRow("row-1"), makeRow("row-1")},
		}
		require.Error(t, tmpl.Validate())
	})

	t.Run("empty row ID", func(t *testing.T) {
		tmpl := Template{
			Name: "My Template",
			Rows: []TemplateRow{makeRow("")},
		}
		require.Error(t, tmpl.Validate())
	})
}

// ---------------------------------------------------------------------------
// ApplyMode tests
// ---------------------------------------------------------------------------

func TestApplyMode_ParseAndString(t *testing.T) {
	cases := []struct {
		name string
		mode ApplyMode
	}{
		{"add", ModeAdd},
		{"replace-matching", ModeReplaceMatching},
		{"replace-mine", ModeReplaceMine},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// String → Parse round-trip
			parsed, err := ParseApplyMode(tc.mode.String())
			require.NoError(t, err)
			require.Equal(t, tc.mode, parsed)
		})
	}

	t.Run("bogus string errors", func(t *testing.T) {
		_, err := ParseApplyMode("bogus")
		require.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// ReconcileDiff tests
// ---------------------------------------------------------------------------

func TestReconcileDiff_CountByKind(t *testing.T) {
	diff := ReconcileDiff{
		Actions: []Action{
			{Kind: ActionCreate},
			{Kind: ActionCreate},
			{Kind: ActionUpdate},
			{Kind: ActionSkip},
			{Kind: ActionSkip},
			{Kind: ActionSkip},
		},
	}
	creates, updates, skips := diff.CountByKind()
	require.Equal(t, 2, creates)
	require.Equal(t, 1, updates)
	require.Equal(t, 3, skips)
}
