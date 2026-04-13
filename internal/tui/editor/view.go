package editor

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/ipm/tdx/internal/domain"
)

var (
	activeStyle = lipgloss.NewStyle().Reverse(true)
	headerStyle = lipgloss.NewStyle().Bold(true)
	hintStyle   = lipgloss.NewStyle().Faint(true)
	groupStyle  = lipgloss.NewStyle().Bold(true)
)

var dayNames = [7]string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}

const cellWidth = 6

// View implements tea.Model.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Title
	title := "Editing: " + m.name
	if m.dirty {
		title += " [modified]"
	}
	b.WriteString(headerStyle.Render(title))
	b.WriteString("\n\n")

	if m.confirm {
		b.WriteString("  Unsaved changes. Quit? (y/n)")
		return b.String()
	}

	// Build ordered groups for display (preserving row indices for cursor mapping).
	type indexedRow struct {
		flatIdx int
		row     domain.TemplateRow
	}
	type group struct {
		name string
		rows []indexedRow
	}
	var groups []group
	groupIdx := map[string]int{}

	for i, r := range m.rows {
		gn := r.Target.GroupName
		if gn == "" {
			groups = append(groups, group{name: "", rows: []indexedRow{{i, r}}})
			continue
		}
		if idx, ok := groupIdx[gn]; ok {
			groups[idx].rows = append(groups[idx].rows, indexedRow{i, r})
		} else {
			groupIdx[gn] = len(groups)
			groups = append(groups, group{name: gn, rows: []indexedRow{{i, r}}})
		}
	}

	// Compute label width.
	labelWidth := len("  ROW")
	for _, g := range groups {
		if g.name != "" {
			label := "  " + g.name
			if len(label) > labelWidth {
				labelWidth = len(label)
			}
		}
		for _, ir := range g.rows {
			prefix := "  "
			if g.name != "" {
				prefix = "    + "
			}
			label := prefix + m.rowLabel(ir.row)
			if len(label) > labelWidth {
				labelWidth = len(label)
			}
		}
	}

	// Header row
	header := padRight("  ROW", labelWidth)
	for _, d := range dayNames {
		header += "  " + padRight(d, cellWidth-1)
	}
	header += "  TOTAL"
	b.WriteString(header + "\n")

	// Separator
	sepLen := labelWidth + 7*(1+cellWidth) + 2 + 5
	b.WriteString(strings.Repeat("─", sepLen) + "\n")

	// Data rows grouped hierarchically.
	var dayTotals [7]float64
	for _, g := range groups {
		if g.name != "" {
			// Group header with aggregated hours.
			var gDays [7]float64
			for _, ir := range g.rows {
				for ci := 0; ci < 7; ci++ {
					gDays[ci] += ir.row.Hours.ForDay(time.Weekday(ci))
				}
			}
			gTotal := 0.0
			for ci := 0; ci < 7; ci++ {
				gTotal += gDays[ci]
			}
			headerLine := padRight("  "+g.name, labelWidth)
			for ci := 0; ci < 7; ci++ {
				if gDays[ci] == 0 {
					headerLine += "  " + padRight(" ", cellWidth-1)
				} else {
					headerLine += "  " + padRight(fmt.Sprintf("%.1f", gDays[ci]), cellWidth-1)
				}
			}
			if gTotal > 0 {
				headerLine += "  " + padRight(fmt.Sprintf("%.1f", gTotal), cellWidth-1)
			}
			b.WriteString(groupStyle.Render(strings.TrimRight(headerLine, " ")) + "\n")

			// Task rows indented under group.
			for _, ir := range g.rows {
				label := "    + " + m.rowLabel(ir.row)
				line := padRight(label, labelWidth)
				rowTotal := 0.0
				for ci := 0; ci < 7; ci++ {
					wd := time.Weekday(ci)
					hours := ir.row.Hours.ForDay(wd)
					cell := m.formatCell(ir.flatIdx, ci, hours)
					line += "  " + cell
					dayTotals[ci] += hours
					rowTotal += hours
				}
				line += "  " + padRight(fmt.Sprintf("%.1f", rowTotal), cellWidth-1)
				b.WriteString(strings.TrimRight(line, " ") + "\n")
				if ir.row.TimeType.Name != "" {
					b.WriteString("        " + ir.row.TimeType.Name + "\n")
				}
			}
		} else {
			// Ungrouped row.
			ir := g.rows[0]
			label := "  " + m.rowLabel(ir.row)
			line := padRight(label, labelWidth)
			rowTotal := 0.0
			for ci := 0; ci < 7; ci++ {
				wd := time.Weekday(ci)
				hours := ir.row.Hours.ForDay(wd)
				cell := m.formatCell(ir.flatIdx, ci, hours)
				line += "  " + cell
				dayTotals[ci] += hours
				rowTotal += hours
			}
			line += "  " + padRight(fmt.Sprintf("%.1f", rowTotal), cellWidth-1)
			b.WriteString(strings.TrimRight(line, " ") + "\n")
			if ir.row.TimeType.Name != "" {
				b.WriteString("    └ " + ir.row.TimeType.Name + "\n")
			}
		}
	}

	// Separator
	b.WriteString(strings.Repeat("─", sepLen) + "\n")

	// Day totals
	totalLine := padRight("  DAY TOTAL", labelWidth)
	grandTotal := 0.0
	for ci := 0; ci < 7; ci++ {
		v := fmt.Sprintf("%.1f", dayTotals[ci])
		if dayTotals[ci] == 0 {
			v = "."
		}
		totalLine += "  " + padRight(v, cellWidth-1)
		grandTotal += dayTotals[ci]
	}
	totalLine += "  " + padRight(fmt.Sprintf("%.1f", grandTotal), cellWidth-1)
	b.WriteString(strings.TrimRight(totalLine, " ") + "\n")

	// Key hints
	b.WriteString("\n")
	hints := "  ←→↑↓/Tab: navigate  type: set value  Backspace: clear  Ctrl-S: save  Esc: cancel"
	b.WriteString(hintStyle.Render(hints))

	return b.String()
}

func (m Model) rowLabel(r domain.TemplateRow) string {
	label := r.Label
	if label == "" {
		label = r.Target.DisplayRef
	}
	if r.Target.GroupName != "" {
		return label
	}
	return label + " (" + string(r.Target.Kind) + ")"
}

func (m Model) formatCell(row, col int, hours float64) string {
	isActive := m.cursor.row == row && m.cursor.col == col

	var text string
	if isActive && m.typing {
		text = m.input + "_"
	} else if hours == 0 {
		text = "."
	} else {
		text = fmt.Sprintf("%.1f", hours)
	}

	padded := padRight(text, cellWidth-1)
	if isActive {
		return activeStyle.Render(padded)
	}
	return padded
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
