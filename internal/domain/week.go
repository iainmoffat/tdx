package domain

import "time"

// WeekRef is a canonical Sun–Sat week boundary, always in EasternTZ.
// StartDate is the Sunday at 00:00:00, EndDate is the Saturday at 00:00:00.
type WeekRef struct {
	StartDate time.Time `json:"startDate"`
	EndDate   time.Time `json:"endDate"`
}

// WeekRefContaining returns the Sun–Sat week (EasternTZ) that contains t.
// Input time zone is ignored; t is first converted into EasternTZ.
func WeekRefContaining(t time.Time) WeekRef {
	et := t.In(EasternTZ)
	// Go's Weekday: Sunday=0, Saturday=6. offset is days back to Sunday.
	offset := int(et.Weekday())
	start := time.Date(et.Year(), et.Month(), et.Day()-offset, 0, 0, 0, 0, EasternTZ)
	end := time.Date(et.Year(), et.Month(), et.Day()-offset+6, 0, 0, 0, 0, EasternTZ)
	return WeekRef{StartDate: start, EndDate: end}
}

// DaySummary is one day's totals inside a WeekReport. Computed client-side
// from the list of TimeEntry rows TD returns in /time/report/{date}.
type DaySummary struct {
	Date    time.Time `json:"date"`
	Minutes int       `json:"minutes"`
	Locked  bool      `json:"locked"`
}

// Hours is a render-convenience wrapper.
func (d DaySummary) Hours() float64 { return float64(d.Minutes) / 60.0 }

// WeekReport is the shape of /TDWebApi/api/time/report/{date}. The per-day
// breakdown (Days) is computed client-side — TD returns a flat entries
// array and totals, not a day-by-day summary.
type WeekReport struct {
	WeekRef      WeekRef      `json:"weekRef"`
	UserUID      string       `json:"userUID"`
	TotalMinutes int          `json:"totalMinutes"`
	Status       ReportStatus `json:"status"`
	Days         []DaySummary `json:"days"`
	Entries      []TimeEntry  `json:"entries"`
}

// TotalHours is a render-convenience wrapper.
func (w WeekReport) TotalHours() float64 { return float64(w.TotalMinutes) / 60.0 }

// LockedDay is one entry in the response of /TDWebApi/api/time/locked.
// TD returns a flat array of dates; Reason is always empty in Phase 2
// and kept as a field for forward compatibility.
type LockedDay struct {
	Date   time.Time `json:"date"`
	Reason string    `json:"reason,omitempty"`
}
