package render

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/ipm/tdx/internal/domain"
)

const gridDayWidth = 5 // "99.9" + gutter
const gridEmptyCell = "."

// WeekGrid writes a Row × Day grid for the given week report. Rows are
// grouped by (Target.DisplayRef, Target.DisplayName, TimeType.Name).
// Days are always Sun..Sat (seven columns). Empty cells render as "." so gaps scan cleanly.
func WeekGrid(w io.Writer, report domain.WeekReport) {
	fmt.Fprintf(w, "Week %s — %s  (%s)\n\n",
		report.WeekRef.StartDate.Format("2006-01-02"),
		report.WeekRef.EndDate.Format("2006-01-02"),
		report.Status)

	if len(report.Entries) == 0 {
		fmt.Fprintln(w, "  no entries in this week")
		return
	}

	type rowKey struct {
		ref      string
		name     string
		typeName string
	}
	type rowAcc struct {
		key   rowKey
		ref   string
		name  string
		typ   string
		byDay [7]int
		total int
	}
	rows := map[rowKey]*rowAcc{}
	order := []rowKey{}

	for _, e := range report.Entries {
		k := rowKey{ref: e.Target.DisplayRef, name: e.Target.DisplayName, typeName: e.TimeType.Name}
		row, ok := rows[k]
		if !ok {
			row = &rowAcc{
				key:  k,
				ref:  e.Target.DisplayRef,
				name: e.Target.DisplayName,
				typ:  e.TimeType.Name,
			}
			rows[k] = row
			order = append(order, k)
		}
		// DST-safe day bucketing: extract calendar dates and reconstruct
		// in UTC so the difference is always exactly 24 hours per day,
		// independent of any spring-forward/fall-back gap.
		ey, em, ed := e.Date.Date()
		ry, rm, rd := report.WeekRef.StartDate.Date()
		entryDay := time.Date(ey, em, ed, 0, 0, 0, 0, time.UTC)
		refDay := time.Date(ry, rm, rd, 0, 0, 0, 0, time.UTC)
		dayIdx := int(entryDay.Sub(refDay).Hours() / 24)
		if dayIdx >= 0 && dayIdx < 7 {
			row.byDay[dayIdx] += e.Minutes
			row.total += e.Minutes
		}
	}

	// Stable sort order: by DisplayRef, DisplayName, then TimeType.Name.
	sort.SliceStable(order, func(i, j int) bool {
		if order[i].ref != order[j].ref {
			return order[i].ref < order[j].ref
		}
		if order[i].name != order[j].name {
			return order[i].name < order[j].name
		}
		return order[i].typeName < order[j].typeName
	})

	// Compute row label width for alignment.
	labelWidth := len("  ROW")
	for _, k := range order {
		r := rows[k]
		label := fmt.Sprintf("  %s %s", r.ref, r.name)
		if len(label) > labelWidth {
			labelWidth = len(label)
		}
	}

	// Header.
	writeGridHeader(w, labelWidth)

	// Separator.
	writeGridSeparator(w, labelWidth)

	// Data rows.
	var dayTotals [7]int
	for _, k := range order {
		r := rows[k]
		label := fmt.Sprintf("  %s %s", r.ref, r.name)
		line := padRight(label, labelWidth)
		for i := 0; i < 7; i++ {
			line += "  " + formatCell(r.byDay[i])
			dayTotals[i] += r.byDay[i]
		}
		line += "  " + formatCell(r.total)
		fmt.Fprintln(w, strings.TrimRight(line, " "))
		// Sub-label row with type and app name.
		fmt.Fprintf(w, "    └ %s\n", r.typ)
	}

	// Separator before totals.
	writeGridSeparator(w, labelWidth)

	// Day-total row.
	totalLine := padRight("  DAY TOTAL", labelWidth)
	for i := 0; i < 7; i++ {
		totalLine += "  " + formatCell(dayTotals[i])
	}
	totalLine += "  " + formatCell(report.TotalMinutes)
	fmt.Fprintln(w, strings.TrimRight(totalLine, " "))
}

func writeGridHeader(w io.Writer, labelWidth int) {
	header := padRight("  ROW", labelWidth)
	for _, d := range []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"} {
		header += "  " + padRight(d, gridDayWidth-1)
	}
	header += "  TOTAL"
	fmt.Fprintln(w, strings.TrimRight(header, " "))
}

func writeGridSeparator(w io.Writer, labelWidth int) {
	// Label + (7 days × (2 gutter + width)) + TOTAL column (7 chars including gutter).
	total := labelWidth + 7*(2+gridDayWidth-1) + 2 + len("TOTAL")
	fmt.Fprintln(w, strings.Repeat("─", total))
}

func formatCell(minutes int) string {
	if minutes == 0 {
		return padRight(gridEmptyCell, gridDayWidth-1)
	}
	return padRight(fmt.Sprintf("%.1f", float64(minutes)/60.0), gridDayWidth-1)
}
