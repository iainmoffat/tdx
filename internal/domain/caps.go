package domain

import "time"

const (
	// MaxReportWeeks bounds the number of Sunday-anchored weeks a single
	// time-report or project-time fan-out call may span. Set to 52 (one year)
	// so accidental wide-range calls (e.g. --from 2010-01-01 --to 2030-01-01)
	// are refused before any TD API request is issued.
	MaxReportWeeks = 52

	// MaxReportUsers bounds the resolved user set per fan-out call. Set to
	// 1000 to refuse pathologically-wide selector expansion (e.g. --all on
	// a multi-thousand-staff tenant) while still allowing typical
	// quarterly reports.
	MaxReportUsers = 1000
)

// WeekSpan returns the number of Sunday-anchored weeks (in EasternTZ) that
// the inclusive range [from, to] touches. Returns 0 when to < from.
//
// DST-safe: iterates by AddDate(0,0,7) on the week-start, never by
// time.Sub() / 24 — spring-forward / fall-back days have 23 / 25 hours.
func WeekSpan(from, to time.Time) int {
	if to.Before(from) {
		return 0
	}
	startWeek := WeekRefContaining(from)
	endWeek := WeekRefContaining(to)
	n := 0
	cur := startWeek
	for !cur.StartDate.After(endWeek.StartDate) {
		n++
		cur = WeekRefContaining(cur.StartDate.AddDate(0, 0, 7))
	}
	return n
}
