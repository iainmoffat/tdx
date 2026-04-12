package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetWeekReport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/TDWebApi/api/time/report/2026-04-08" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ID": 1,
				"PeriodStartDate": "2026-04-05T00:00:00Z",
				"PeriodEndDate": "2026-04-11T00:00:00Z",
				"Status": 0,
				"TimeReportUid": "uid-abc",
				"UserFullName": "Test User",
				"MinutesBillable": 0,
				"MinutesNonBillable": 480,
				"MinutesTotal": 480,
				"TimeEntriesCount": 1,
				"Times": [
					{"TimeID":1,"ItemID":12345,"AppID":42,"Component":9,"TicketID":12345,"TimeDate":"2026-04-07T00:00:00Z","Minutes":480,"TimeTypeID":1,"TimeTypeName":"Development","Status":0}
				]
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svcs := mcpHarness(t, srv.URL)
	handler := weekReportHandler(svcs)
	result, _, err := handler(context.Background(), nil, weekReportArgs{Date: "2026-04-08"})
	require.NoError(t, err)
	require.False(t, result.IsError)

	text := extractText(t, result)
	var report map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &report))
	require.Contains(t, report, "weekRef")
	weekRef, ok := report["weekRef"].(map[string]any)
	require.True(t, ok, "weekRef should be a map")
	require.Contains(t, weekRef["startDate"], "2026-04-05")
}

func TestGetLockedDays(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/TDWebApi/api/time/locked" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				"2026-04-06T00:00:00Z",
				"2026-04-13T00:00:00Z"
			]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svcs := mcpHarness(t, srv.URL)
	handler := lockedDaysHandler(svcs)
	result, _, err := handler(context.Background(), nil, lockedDaysArgs{
		From: "2026-04-01",
		To:   "2026-04-30",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	text := extractText(t, result)
	var days []map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &days))
	require.Len(t, days, 2)
}
