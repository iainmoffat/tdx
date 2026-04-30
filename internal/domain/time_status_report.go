package domain

import (
	"sort"
	"time"
)

// WeekStatusRow is one (user × week) row of a TimeStatusReport.
// Status is the user's weekly time-report submission status.
type WeekStatusRow struct {
	WeekRef        WeekRef      `json:"weekRef"`
	User           User         `json:"user"`
	Status         ReportStatus `json:"status"`
	BillableMin    int          `json:"billableMinutes"`
	NonBillableMin int          `json:"nonBillableMinutes"`
	TotalMin       int          `json:"totalMinutes"`
}

func (r WeekStatusRow) BillableHours() float64    { return float64(r.BillableMin) / 60.0 }
func (r WeekStatusRow) NonBillableHours() float64 { return float64(r.NonBillableMin) / 60.0 }
func (r WeekStatusRow) TotalHours() float64       { return float64(r.TotalMin) / 60.0 }

// TimeStatusReport is the assembled output of `tdx time report status`.
type TimeStatusReport struct {
	From WeekRef         `json:"from"`
	To   WeekRef         `json:"to"`
	Rows []WeekStatusRow `json:"rows"`
}

// Totals returns aggregated minutes across all rows.
func (r TimeStatusReport) Totals() (billable, nonBillable, total int) {
	for _, row := range r.Rows {
		billable += row.BillableMin
		nonBillable += row.NonBillableMin
		total += row.TotalMin
	}
	return
}

// WeekGroup bundles rows that fall within a single week.
type WeekGroup struct {
	Week WeekRef
	Rows []WeekStatusRow
}

// RowsByWeek groups rows by their WeekRef.StartDate, ordered ascending by
// week. Order within a week preserves input order.
func (r TimeStatusReport) RowsByWeek() []WeekGroup {
	idx := map[time.Time]int{}
	groups := []WeekGroup{}
	for _, row := range r.Rows {
		key := row.WeekRef.StartDate
		if pos, ok := idx[key]; ok {
			groups[pos].Rows = append(groups[pos].Rows, row)
			continue
		}
		idx[key] = len(groups)
		groups = append(groups, WeekGroup{Week: row.WeekRef, Rows: []WeekStatusRow{row}})
	}
	sort.SliceStable(groups, func(i, j int) bool {
		return groups[i].Week.StartDate.Before(groups[j].Week.StartDate)
	})
	return groups
}
