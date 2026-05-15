package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCaps_Values(t *testing.T) {
	require.Equal(t, 52, MaxReportWeeks)
	require.Equal(t, 1000, MaxReportUsers)
}

func TestWeekSpan_SameDay(t *testing.T) {
	d := time.Date(2026, 4, 14, 0, 0, 0, 0, EasternTZ)
	require.Equal(t, 1, WeekSpan(d, d))
}

func TestWeekSpan_SameWeek(t *testing.T) {
	// Tuesday → Friday in the same Sun-anchored week
	from := time.Date(2026, 4, 14, 0, 0, 0, 0, EasternTZ) // Tue
	to := time.Date(2026, 4, 17, 0, 0, 0, 0, EasternTZ)   // Fri
	require.Equal(t, 1, WeekSpan(from, to))
}

func TestWeekSpan_AcrossWeeks(t *testing.T) {
	// Sun 2026-04-12 (week 1) → Sun 2026-04-19 (week 2)
	from := time.Date(2026, 4, 12, 0, 0, 0, 0, EasternTZ)
	to := time.Date(2026, 4, 19, 0, 0, 0, 0, EasternTZ)
	require.Equal(t, 2, WeekSpan(from, to))
}

func TestWeekSpan_DSTSpringForward(t *testing.T) {
	// Spring-forward 2026-03-08: the Saturday→Sunday transition loses 1 hour
	// but is still exactly 1 week. WeekSpan must not use Sub()/24 arithmetic.
	from := time.Date(2026, 3, 1, 0, 0, 0, 0, EasternTZ)  // Sun (before DST)
	to := time.Date(2026, 3, 15, 0, 0, 0, 0, EasternTZ)   // Sun (after DST)
	require.Equal(t, 3, WeekSpan(from, to))
}

func TestWeekSpan_MaxWeeks(t *testing.T) {
	// 52 weeks exactly: from Sun to the Sun 51 weeks later
	from := time.Date(2026, 1, 4, 0, 0, 0, 0, EasternTZ)
	to := from.AddDate(0, 0, 7*51)
	require.Equal(t, 52, WeekSpan(from, to))
}

func TestWeekSpan_ToBeforeFrom(t *testing.T) {
	// Negative span is invalid; helper returns 0.
	from := time.Date(2026, 4, 14, 0, 0, 0, 0, EasternTZ)
	to := time.Date(2026, 4, 7, 0, 0, 0, 0, EasternTZ)
	require.Equal(t, 0, WeekSpan(from, to))
}
