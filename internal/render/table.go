package render

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// visualWidth returns the rune-count of s, which approximates the column
// width a typical fixed-width terminal will allocate. Treats every rune as
// width 1 — adequate for ASCII plus the truncation ellipsis "…" we use; not
// CJK-correct (wide characters need wcwidth). Good enough for tdx today.
func visualWidth(s string) int { return utf8.RuneCountInString(s) }

// Table writes a left-aligned column-padded table to w. Column widths are
// computed as max(header-width, longest-value-width) per column, with a
// two-space gutter between columns. If summary is non-nil, a thin separator
// line is written before the summary row.
//
// Widths and padding are measured in runes (visual cells), not bytes, so a
// truncation marker like "…" (3 bytes / 1 visible char) doesn't throw off
// column alignment.
//
// Table does not wrap, truncate, or color. Callers that need those
// treatments should preprocess the cell strings.
func Table(w io.Writer, headers []string, rows [][]string, summary []string) {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = visualWidth(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i >= len(widths) {
				continue
			}
			if vw := visualWidth(cell); vw > widths[i] {
				widths[i] = vw
			}
		}
	}
	for i, cell := range summary {
		if i >= len(widths) {
			continue
		}
		if vw := visualWidth(cell); vw > widths[i] {
			widths[i] = vw
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
	vw := visualWidth(s)
	if vw >= width {
		return s
	}
	return s + strings.Repeat(" ", width-vw)
}
