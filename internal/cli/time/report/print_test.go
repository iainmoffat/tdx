package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func sampleReport() domain.TimeStatusReport {
	week := domain.WeekRefContaining(time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ))
	return domain.TimeStatusReport{
		From: week, To: week,
		Rows: []domain.WeekStatusRow{
			{
				WeekRef: week,
				User: domain.User{
					UID: "u1", FullName: "Alice", Email: "a@x",
					ReportsToName: "Mgr", ReportsToEmail: "m@x",
				},
				Status:         domain.ReportSubmitted,
				BillableMin:    240,
				NonBillableMin: 60,
				TotalMin:       300,
			},
		},
	}
}

func TestPrintText_HumanReadable(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, printText(&buf, sampleReport(), statusFlags{users: []string{"u1"}, week: "2026-04-14"}))
	out := buf.String()
	require.Contains(t, out, "WEEK 2026-04-12")
	require.Contains(t, out, "Alice")
	require.Contains(t, out, "submitted")
	require.Contains(t, out, "4.00") // billable hours
	require.Contains(t, out, "5.00") // total hours
}

func TestPrintJSON_Schema(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, printJSON(&buf, sampleReport(), statusFlags{users: []string{"u1"}, week: "2026-04-14"}))
	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.Equal(t, "tdx.v1.timeStatusReport", got["schema"])
	weeks, ok := got["weeks"].([]any)
	require.True(t, ok)
	require.Len(t, weeks, 1)
	w, ok := weeks[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "2026-04-12", w["weekStart"])
	rows, ok := w["rows"].([]any)
	require.True(t, ok)
	require.Len(t, rows, 1)
	row, ok := rows[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "u1", row["userUID"])
	require.InDelta(t, 4.0, row["billableHours"], 0.001)
	require.InDelta(t, 5.0, row["totalHours"], 0.001)
}

func TestPrintJSON_FilterEcho(t *testing.T) {
	var buf bytes.Buffer
	f := statusFlags{users: []string{"u1", "u2"}, week: "2026-04-14"}
	require.NoError(t, printJSON(&buf, sampleReport(), f))
	out := buf.String()
	require.True(t, strings.Contains(out, `"selector": "user"`))
	require.True(t, strings.Contains(out, `"u1"`))
	require.True(t, strings.Contains(out, `"u2"`))
}

func TestPrintJSON_IncompleteEchoedWhenSet(t *testing.T) {
	var buf bytes.Buffer
	f := statusFlags{
		manager: "me", week: "2026-04-14",
		incomplete: true, threshold: 32,
	}
	require.NoError(t, printJSON(&buf, sampleReport(), f))
	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	filter, ok := got["filter"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, filter["incomplete"])
	require.InDelta(t, 32.0, filter["threshold"], 0.001)
}

func TestPrintJSON_IncompleteOmittedWhenUnset(t *testing.T) {
	var buf bytes.Buffer
	f := statusFlags{users: []string{"u1"}, week: "2026-04-14"}
	require.NoError(t, printJSON(&buf, sampleReport(), f))
	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	filter, ok := got["filter"].(map[string]any)
	require.True(t, ok)
	_, hasIncomplete := filter["incomplete"]
	require.False(t, hasIncomplete, "incomplete should be omitted when not set")
	_, hasThreshold := filter["threshold"]
	require.False(t, hasThreshold)
}
