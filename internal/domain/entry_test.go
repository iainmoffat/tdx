package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReportStatus_IsTerminal(t *testing.T) {
	require.False(t, ReportOpen.IsTerminal())
	require.False(t, ReportSubmitted.IsTerminal())
	require.True(t, ReportApproved.IsTerminal())
	require.True(t, ReportRejected.IsTerminal())
}

func TestDateRange_Contains(t *testing.T) {
	d := func(y int, m time.Month, day int) time.Time {
		return time.Date(y, m, day, 0, 0, 0, 0, EasternTZ)
	}
	r := DateRange{From: d(2026, 4, 5), To: d(2026, 4, 11)}
	require.True(t, r.Contains(d(2026, 4, 5)))   // boundary start
	require.True(t, r.Contains(d(2026, 4, 11)))  // boundary end
	require.True(t, r.Contains(d(2026, 4, 8)))   // middle
	require.False(t, r.Contains(d(2026, 4, 4)))  // before
	require.False(t, r.Contains(d(2026, 4, 12))) // after
}

func TestTimeEntry_Hours(t *testing.T) {
	e := TimeEntry{Minutes: 90}
	require.InDelta(t, 1.5, e.Hours(), 0.0001)

	e2 := TimeEntry{Minutes: 0}
	require.Equal(t, 0.0, e2.Hours())
}

func TestTimeEntry_DateJSON(t *testing.T) {
	e := TimeEntry{
		ID:      123,
		Date:    time.Date(2026, 4, 6, 0, 0, 0, 0, EasternTZ),
		Minutes: 120,
	}
	blob, err := json.Marshal(e)
	require.NoError(t, err)
	require.Contains(t, string(blob), `"date":"2026-04-06"`)
	require.NotContains(t, string(blob), "T00:00:00")
}

func TestEntryFilter_DefaultLimit(t *testing.T) {
	f := EntryFilter{}
	require.Equal(t, 0, f.Limit, "zero means unset; caller decides default")
}
