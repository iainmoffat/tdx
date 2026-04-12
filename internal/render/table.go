package render

import (
	"fmt"
	"io"
	"strings"
)

// Table writes a left-aligned column-padded table to w. Column widths are
// computed as max(header-width, longest-value-width) per column, with a
// two-space gutter between columns. If summary is non-nil, a thin separator
// line is written before the summary row.
//
// Table does not wrap, truncate, or color. Callers that need those
// treatments should preprocess the cell strings.
func Table(w io.Writer, headers []string, rows [][]string, summary []string) {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i >= len(widths) {
				continue
			}
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	for i, cell := range summary {
		if i >= len(widths) {
			continue
		}
		if len(cell) > widths[i] {
			widths[i] = len(cell)
		}
	}

	writeRow(w, headers, widths)
	for _, row := range rows {
		writeRow(w, row, widths)
	}
	if len(summary) > 0 {
		writeSeparator(w, widths)
		writeRow(w, summary, widths)
	}
}

func writeRow(w io.Writer, cells []string, widths []int) {
	parts := make([]string, len(widths))
	for i := range widths {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		parts[i] = padRight(cell, widths[i])
	}
	// Trim trailing spaces so callers can do exact-line equality assertions
	// without worrying about column padding on the last cell.
	_, _ = fmt.Fprintln(w, strings.TrimRight(strings.Join(parts, "  "), " "))
}

func writeSeparator(w io.Writer, widths []int) {
	total := 0
	for _, width := range widths {
		total += width
	}
	// Account for "  " gutters between columns.
	if len(widths) > 1 {
		total += 2 * (len(widths) - 1)
	}
	_, _ = fmt.Fprintln(w, strings.Repeat("─", total))
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
