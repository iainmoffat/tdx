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

	// Compute label width
	labelWidth := len("  ROW")
	for _, r := range m.rows {
		label := "  " + m.rowLabel(r)
		if len(label) > labelWidth {
			labelWidth = len(label)
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

	// Data rows
	var dayTotals [7]float64
	for ri, r := range m.rows {
		label := "  " + m.rowLabel(r)
		line := padRight(label, labelWidth)
		rowTotal := 0.0
		for ci := 0; ci < 7; ci++ {
			wd := time.Weekday(ci)
			hours := r.Hours.ForDay(wd)
			cell := m.formatCell(ri, ci, hours)
			line += "  " + cell
			dayTotals[ci] += hours
			rowTotal += hours
		}
		line += "  " + padRight(fmt.Sprintf("%.1f", rowTotal), cellWidth-1)
		b.WriteString(strings.TrimRight(line, " ") + "\n")
		// Sub-label
		b.WriteString("    └ " + r.TimeType.Name + "\n")
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
