package render

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

const gridDayWidth = 5 // "99.9" + gutter
const gridEmptyCell = "."

// WeekGrid writes a Row × Day grid for the given week report. Rows are
// grouped by project name when available, with tasks indented underneath.
func WeekGrid(w io.Writer, report domain.WeekReport) {
	title := fmt.Sprintf("Week %s — %s  (%s)",
		report.WeekRef.StartDate.Format("2006-01-02"),
		report.WeekRef.EndDate.Format("2006-01-02"),
		report.Status)

	if len(report.Entries) == 0 {
		_, _ = fmt.Fprintln(w, title)
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, "  no entries in this week")
		return
	}

	data := WeekReportToGridData(report, title)
	Grid(w, data)
}

// WeekReportToGridData converts a domain.WeekReport into GridData for
// the shared Grid renderer.
func WeekReportToGridData(report domain.WeekReport, title string) GridData {
	type rowKey struct {
		ref      string
		name     string
		typeName string
	}
	type rowAcc struct {
		ref       string
		name      string
		groupName string
		typ       string
		byDay     [7]float64
	}
	rows := map[rowKey]*rowAcc{}
	order := []rowKey{}

	for _, e := range report.Entries {
		k := rowKey{ref: e.Target.DisplayRef, name: e.Target.DisplayName, typeName: e.TimeType.Name}
		row, ok := rows[k]
		if !ok {
			row = &rowAcc{
				ref:       e.Target.DisplayRef,
				name:      e.Target.DisplayName,
				groupName: e.Target.GroupName,
				typ:       e.TimeType.Name,
			}
			rows[k] = row
			order = append(order, k)
		}
		// DST-safe day bucketing.
		ey, em, ed := e.Date.Date()
		ry, rm, rd := report.WeekRef.StartDate.Date()
		entryDay := time.Date(ey, em, ed, 0, 0, 0, 0, time.UTC)
		refDay := time.Date(ry, rm, rd, 0, 0, 0, 0, time.UTC)
		dayIdx := int(entryDay.Sub(refDay).Hours() / 24)
		if dayIdx >= 0 && dayIdx < 7 {
			row.byDay[dayIdx] += float64(e.Minutes) / 60.0
		}
	}

	// Sort by GroupName, then DisplayName.
	sort.SliceStable(order, func(i, j int) bool {
		ri, rj := rows[order[i]], rows[order[j]]
		if ri.groupName != rj.groupName {
			return ri.groupName < rj.groupName
		}
		if ri.name != rj.name {
			return ri.name < rj.name
		}
		return ri.ref < rj.ref
	})

	gridRows := make([]GridRow, len(order))
	for i, k := range order {
		r := rows[k]
		var label string
		if r.groupName != "" {
			// Grouped under a project — show task name only.
			label = r.name
			if label == "" {
				label = r.ref
			}
		} else {
			// Ungrouped — show ref + name (e.g. "#12345 Ingest pipeline").
			label = r.ref
			if r.name != "" {
				label += " " + r.name
			}
		}
		gridRows[i] = GridRow{
			Label:  label,
			Detail: r.typ,
			Group:  r.groupName,
			Hours:  r.byDay,
		}
	}

	return GridData{Title: title, Rows: gridRows}
}

func writeGridHeader(w io.Writer, labelWidth int) {
	header := padRight("  ROW", labelWidth)
	for _, d := range []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"} {
		header += "  " + padRight(d, gridDayWidth-1)
	}
	header += "  TOTAL"
	_, _ = fmt.Fprintln(w, strings.TrimRight(header, " "))
}

func writeGridSeparator(w io.Writer, labelWidth int) {
	// Label + (7 days × (2 gutter + width)) + TOTAL column (7 chars including gutter).
	total := labelWidth + 7*(2+gridDayWidth-1) + 2 + len("TOTAL")
	_, _ = fmt.Fprintln(w, strings.Repeat("─", total))
}

// GridData is the abstract input for the grid renderer. Both templates and
// week reports can be converted to this format.
type GridData struct {
	Title    string
	Subtitle string
	Rows     []GridRow
}

// GridRow is one row in the grid.
type GridRow struct {
	Label   string
	Detail  string
	Group   string     // parent group name (e.g. project name); rows with the same Group are grouped together
	Ref     string     // e.g. "(project)" or "(ticket #12345)"
	Hours   [7]float64 // index 0=Sun .. 6=Sat
	Markers [7]string  // "", "+", "=", "~", "✗" — per-cell annotations
}

// Grid writes a Row × Day grid to w from abstract GridData. Used for
// template show, apply preview, and compare output.
//
// When rows have a Group field, they are collected under group headers.
// Rows without a Group render as top-level entries.
func Grid(w io.Writer, data GridData) {
	_, _ = fmt.Fprintln(w, data.Title)
	if data.Subtitle != "" {
		_, _ = fmt.Fprintln(w, data.Subtitle)
	}
	_, _ = fmt.Fprintln(w)

	if len(data.Rows) == 0 {
		_, _ = fmt.Fprintln(w, "  no rows")
		return
	}

	// Build ordered groups: preserve first-seen order, collect rows per group.
	type group struct {
		name string
		rows []GridRow
	}
	var groups []group
	groupIdx := map[string]int{} // group name → index in groups slice

	for _, r := range data.Rows {
		gn := r.Group
		if gn == "" {
			// Ungrouped rows get their own singleton group using the label.
			groups = append(groups, group{name: "", rows: []GridRow{r}})
			continue
		}
		if idx, ok := groupIdx[gn]; ok {
			groups[idx].rows = append(groups[idx].rows, r)
		} else {
			groupIdx[gn] = len(groups)
			groups = append(groups, group{name: gn, rows: []GridRow{r}})
		}
	}

	// Compute label column width.
	labelWidth := len("  ROW")
	for _, g := range groups {
		if g.name != "" {
			label := "  " + g.name
			if len(label) > labelWidth {
				labelWidth = len(label)
			}
		}
		for _, r := range g.rows {
			prefix := "  "
			if g.name != "" {
				prefix = "    + " // indented task
			}
			label := prefix + r.Label
			if len(label) > labelWidth {
				labelWidth = len(label)
			}
		}
	}

	// Header + separator.
	writeGridHeader(w, labelWidth)
	writeGridSeparator(w, labelWidth)

	// Data rows grouped hierarchically.
	var dayTotals [7]float64
	for _, g := range groups {
		if g.name != "" {
			// Group header row: show group name with aggregated hours.
			var groupDays [7]float64
			for _, r := range g.rows {
				for i := 0; i < 7; i++ {
					groupDays[i] += r.Hours[i]
				}
			}
			groupTotal := 0.0
			for i := 0; i < 7; i++ {
				groupTotal += groupDays[i]
			}
			headerLine := padRight("  "+g.name, labelWidth)
			for i := 0; i < 7; i++ {
				headerLine += "  " + formatGroupCell(groupDays[i])
			}
			if groupTotal > 0 {
				headerLine += "  " + padRight(fmt.Sprintf("%.1f", groupTotal), gridDayWidth-1)
			}
			_, _ = fmt.Fprintln(w, strings.TrimRight(headerLine, " "))

			// Task rows indented under the group.
			for _, r := range g.rows {
				label := "    + " + r.Label
				line := padRight(label, labelWidth)
				rowTotal := 0.0
				for i := 0; i < 7; i++ {
					cell := formatGridCell(r.Hours[i], r.Markers[i])
					line += "  " + cell
					dayTotals[i] += r.Hours[i]
					rowTotal += r.Hours[i]
				}
				line += "  " + padRight(fmt.Sprintf("%.1f", rowTotal), gridDayWidth-1)
				_, _ = fmt.Fprintln(w, strings.TrimRight(line, " "))
				if r.Detail != "" {
					_, _ = fmt.Fprintf(w, "      %s\n", r.Detail)
				}
			}
		} else {
			// Ungrouped row (singleton).
			r := g.rows[0]
			label := "  " + r.Label
			if r.Ref != "" {
				label += " " + r.Ref
			}
			line := padRight(label, labelWidth)
			rowTotal := 0.0
			for i := 0; i < 7; i++ {
				cell := formatGridCell(r.Hours[i], r.Markers[i])
				line += "  " + cell
				dayTotals[i] += r.Hours[i]
				rowTotal += r.Hours[i]
			}
			line += "  " + padRight(fmt.Sprintf("%.1f", rowTotal), gridDayWidth-1)
			_, _ = fmt.Fprintln(w, strings.TrimRight(line, " "))
			if r.Detail != "" {
				_, _ = fmt.Fprintf(w, "    └ %s\n", r.Detail)
			}
		}
	}

	// Separator + day totals.
	writeGridSeparator(w, labelWidth)
	totalLine := padRight("  DAY TOTAL", labelWidth)
	grandTotal := 0.0
	for i := 0; i < 7; i++ {
		totalLine += "  " + formatGridCell(dayTotals[i], "")
		grandTotal += dayTotals[i]
	}
	totalLine += "  " + padRight(fmt.Sprintf("%.1f", grandTotal), gridDayWidth-1)
	_, _ = fmt.Fprintln(w, strings.TrimRight(totalLine, " "))
}

// formatGroupCell formats a group header cell (no markers).
func formatGroupCell(hours float64) string {
	if hours == 0 {
		return padRight(" ", gridDayWidth-1)
	}
	return padRight(fmt.Sprintf("%.1f", hours), gridDayWidth-1)
}

// formatGridCell formats a single cell with optional marker prefix.
// Used by Grid() — the older WeekGrid() uses formatCell() instead.
func formatGridCell(hours float64, marker string) string {
	if hours == 0 && marker == "" {
		return padRight(gridEmptyCell, gridDayWidth-1)
	}
	if hours == 0 && marker != "" {
		return padRight(marker, gridDayWidth-1)
	}
	val := fmt.Sprintf("%.1f", hours)
	if marker != "" {
		val = marker + val
	}
	return padRight(val, gridDayWidth-1)
}
