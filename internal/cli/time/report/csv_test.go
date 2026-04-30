package report

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteCSV_Headers(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, writeCSV(&buf, sampleReport()))
	r := csv.NewReader(strings.NewReader(buf.String()))
	rows, err := r.ReadAll()
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	require.Equal(t,
		[]string{"weekStart", "weekEnd", "userUID", "name", "email", "reportsToName", "reportsToEmail", "status", "billableHours", "nonBillableHours", "totalHours"},
		rows[0])
}

func TestWriteCSV_DataRow(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, writeCSV(&buf, sampleReport()))
	r := csv.NewReader(strings.NewReader(buf.String()))
	rows, err := r.ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 2, "header + 1 data row")
	require.Equal(t, "2026-04-12", rows[1][0])
	require.Equal(t, "u1", rows[1][2])
	require.Equal(t, "Alice", rows[1][3])
	require.Equal(t, "submitted", rows[1][7])
	require.Equal(t, "4.00", rows[1][8])
	require.Equal(t, "5.00", rows[1][10])
}
