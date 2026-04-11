package render

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTable_BasicAlignment(t *testing.T) {
	var buf bytes.Buffer
	Table(&buf,
		[]string{"ID", "NAME", "ROLE"},
		[][]string{
			{"1", "Iain", "Owner"},
			{"17", "Other Person", "Dev"},
		},
		nil,
	)
	got := buf.String()
	// Column widths should be max(header, longest value) per column.
	// Header: "ID" (2), "NAME" (4), "ROLE" (4)
	// Longest: "17" (2), "Other Person" (12), "Owner" (5)
	// Result column widths: 2, 12, 5
	require.Contains(t, got, "ID  NAME          ROLE")
	require.Contains(t, got, "1   Iain          Owner")
	require.Contains(t, got, "17  Other Person  Dev")
}

func TestTable_WithSummary(t *testing.T) {
	var buf bytes.Buffer
	Table(&buf,
		[]string{"DATE", "HOURS"},
		[][]string{
			{"2026-04-06", "2.00"},
			{"2026-04-07", "1.50"},
		},
		[]string{"TOTAL", "3.50"},
	)
	got := buf.String()
	require.Contains(t, got, "DATE        HOURS")
	require.Contains(t, got, "2026-04-06  2.00")
	require.Contains(t, got, "────")
	require.Contains(t, got, "TOTAL       3.50")
}

func TestTable_NoRows(t *testing.T) {
	var buf bytes.Buffer
	Table(&buf, []string{"A", "B"}, nil, nil)
	got := buf.String()
	require.Contains(t, got, "A  B")
	// With no rows, we should still print the header and nothing else.
}

func TestTable_PreservesRowOrder(t *testing.T) {
	var buf bytes.Buffer
	Table(&buf,
		[]string{"X"},
		[][]string{{"z"}, {"a"}, {"m"}},
		nil,
	)
	lines := splitLines(buf.String())
	require.Contains(t, lines[1], "z")
	require.Contains(t, lines[2], "a")
	require.Contains(t, lines[3], "m")
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
