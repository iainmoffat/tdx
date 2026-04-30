package report

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestWriteXLSX_HeaderAndDataRow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.xlsx")
	require.NoError(t, writeXLSX(path, sampleReport()))

	f, err := excelize.OpenFile(path)
	require.NoError(t, err)
	defer f.Close()

	const sheet = "Time Status Report"
	rows, err := f.GetRows(sheet)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(rows), 2, "header + at least one data row")

	header := rows[0]
	require.Equal(t, "weekStart", header[0])
	require.Equal(t, "totalHours", header[len(header)-1])

	data := rows[1]
	require.Equal(t, "2026-04-12", data[0])
	require.Equal(t, "u1", data[2])
	require.Equal(t, "Alice", data[3])
}

func TestWriteXLSX_HeaderIsBold(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.xlsx")
	require.NoError(t, writeXLSX(path, sampleReport()))

	f, err := excelize.OpenFile(path)
	require.NoError(t, err)
	defer f.Close()

	styleID, err := f.GetCellStyle("Time Status Report", "A1")
	require.NoError(t, err)
	require.NotZero(t, styleID, "header cells must have a non-default style (bold)")
}

func TestWriteXLSX_HoursAreNumeric(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.xlsx")
	require.NoError(t, writeXLSX(path, sampleReport()))

	f, err := excelize.OpenFile(path)
	require.NoError(t, err)
	defer f.Close()

	// Cell I2 is the BillableHours value (4.0). After float64 SetCellValue,
	// excelize stores it as a number; reading it back should give "4" or "4.0"
	// (no quotes, no trailing zeros forced) — definitively NOT "4.00" which
	// would indicate it was set as a pre-formatted string.
	billCell, err := f.GetCellValue("Time Status Report", "I2")
	require.NoError(t, err)
	require.NotEqual(t, "4.00", billCell, "hours should be stored as numbers, not pre-formatted strings")
	// "4" is what excelize returns for the canonical numeric form.
	require.Contains(t, []string{"4", "4.0"}, billCell, "expected '4' or '4.0', got %q", billCell)
}
