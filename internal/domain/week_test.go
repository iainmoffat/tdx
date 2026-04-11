package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 12, 0, 0, 0, EasternTZ)
}

func TestWeekRefContaining_SundayInput(t *testing.T) {
	// 2026-04-05 is a Sunday.
	w := WeekRefContaining(d(2026, 4, 5))
	require.Equal(t, time.Sunday, w.StartDate.Weekday())
	require.Equal(t, "2026-04-05", w.StartDate.Format("2006-01-02"))
	require.Equal(t, "2026-04-11", w.EndDate.Format("2006-01-02"))
}

func TestWeekRefContaining_SaturdayInput(t *testing.T) {
	// 2026-04-11 is a Saturday.
	w := WeekRefContaining(d(2026, 4, 11))
	require.Equal(t, "2026-04-05", w.StartDate.Format("2006-01-02"))
	require.Equal(t, "2026-04-11", w.EndDate.Format("2006-01-02"))
}

func TestWeekRefContaining_MidWeek(t *testing.T) {
	// 2026-04-08 is a Wednesday.
	w := WeekRefContaining(d(2026, 4, 8))
	require.Equal(t, "2026-04-05", w.StartDate.Format("2006-01-02"))
	require.Equal(t, "2026-04-11", w.EndDate.Format("2006-01-02"))
}

func TestWeekRefContaining_SpringForward(t *testing.T) {
	// DST starts 2026-03-08 at 02:00 EST → 03:00 EDT. The week containing
	// that Sunday must still have StartDate = 2026-03-08 and EndDate = 2026-03-14.
	w := WeekRefContaining(d(2026, 3, 10)) // Tuesday after DST start
	require.Equal(t, "2026-03-08", w.StartDate.Format("2006-01-02"))
	require.Equal(t, "2026-03-14", w.EndDate.Format("2006-01-02"))
}

func TestWeekRefContaining_FallBack(t *testing.T) {
	// DST ends 2026-11-01 at 02:00 EDT → 01:00 EST.
	w := WeekRefContaining(d(2026, 11, 4)) // Wednesday after fall back
	require.Equal(t, "2026-11-01", w.StartDate.Format("2006-01-02"))
	require.Equal(t, "2026-11-07", w.EndDate.Format("2006-01-02"))
}

func TestWeekRefContaining_YearBoundary(t *testing.T) {
	// 2026-01-01 is a Thursday; the week containing it is 2025-12-28..2026-01-03.
	w := WeekRefContaining(d(2026, 1, 1))
	require.Equal(t, "2025-12-28", w.StartDate.Format("2006-01-02"))
	require.Equal(t, "2026-01-03", w.EndDate.Format("2006-01-02"))
}

func TestWeekRefContaining_UTCInputConvertedToEastern(t *testing.T) {
	// 2026-04-06 01:00 UTC is 2026-04-05 21:00 EDT (Sunday). So the week
	// is 2026-04-05..2026-04-11, not 2026-04-06..2026-04-12.
	utc := time.Date(2026, 4, 6, 1, 0, 0, 0, time.UTC)
	w := WeekRefContaining(utc)
	require.Equal(t, "2026-04-05", w.StartDate.Format("2006-01-02"))
}

func TestWeekReport_TotalHours(t *testing.T) {
	wr := WeekReport{
		TotalMinutes: 1470, // 24.5 hours
	}
	require.InDelta(t, 24.5, wr.TotalHours(), 0.0001)
}

func TestLockedDay_Empty(t *testing.T) {
	ld := LockedDay{Date: d(2026, 4, 6)}
	require.Empty(t, ld.Reason)
}
