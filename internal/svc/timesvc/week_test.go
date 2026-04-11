package timesvc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ipm/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestGetWeekReport_DecodesAndComputesDays(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/TDWebApi/api/time/report/2026-04-08", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"ID": 1,
			"PeriodStartDate": "2026-04-05T00:00:00Z",
			"PeriodEndDate": "2026-04-11T00:00:00Z",
			"Status": 0,
			"TimeReportUid": "abcd-1234",
			"UserFullName": "Iain Moffat",
			"MinutesBillable": 0,
			"MinutesNonBillable": 1200,
			"MinutesTotal": 1200,
			"TimeEntriesCount": 3,
			"Times": [
				{"TimeID":1,"ItemID":12345,"AppID":42,"Component":9,"TicketID":12345,"TimeDate":"2026-04-06T00:00:00Z","Minutes":240,"TimeTypeID":1,"TimeTypeName":"Development","Status":0},
				{"TimeID":2,"ItemID":12345,"AppID":42,"Component":9,"TicketID":12345,"TimeDate":"2026-04-07T00:00:00Z","Minutes":480,"TimeTypeID":1,"TimeTypeName":"Development","Status":0},
				{"TimeID":3,"ItemID":12345,"AppID":42,"Component":9,"TicketID":12345,"TimeDate":"2026-04-08T00:00:00Z","Minutes":480,"TimeTypeID":1,"TimeTypeName":"Development","Status":0}
			]
		}`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	// Any date in the target week works.
	day := time.Date(2026, 4, 8, 0, 0, 0, 0, domain.EasternTZ)
	report, err := svc.GetWeekReport(context.Background(), profile, day)
	require.NoError(t, err)

	require.Equal(t, "2026-04-05", report.WeekRef.StartDate.Format("2006-01-02"))
	require.Equal(t, "2026-04-11", report.WeekRef.EndDate.Format("2006-01-02"))
	require.Equal(t, 1200, report.TotalMinutes)
	require.InDelta(t, 20.0, report.TotalHours(), 0.0001)
	require.Equal(t, domain.ReportOpen, report.Status)
	require.Len(t, report.Entries, 3)

	// Days must always be seven, Sun..Sat.
	require.Len(t, report.Days, 7)
	require.Equal(t, time.Sunday, report.Days[0].Date.Weekday())
	require.Equal(t, time.Saturday, report.Days[6].Date.Weekday())

	// Per-day totals computed from entries.
	require.Equal(t, 0, report.Days[0].Minutes)   // Sun
	require.Equal(t, 240, report.Days[1].Minutes) // Mon
	require.Equal(t, 480, report.Days[2].Minutes) // Tue
	require.Equal(t, 480, report.Days[3].Minutes) // Wed
	require.Equal(t, 0, report.Days[4].Minutes)   // Thu
	require.Equal(t, 0, report.Days[5].Minutes)   // Fri
	require.Equal(t, 0, report.Days[6].Minutes)   // Sat
}

func TestGetWeekReport_BucketsAcrossSpringForward(t *testing.T) {
	// Spring-forward DST starts 2026-03-08 02:00 EST → 03:00 EDT.
	// The week containing March 8 is 2026-03-08 (Sun) to 2026-03-14 (Sat).
	// Entries dated Mon..Sat must land in their correct day buckets.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"ID": 7,
			"PeriodStartDate": "2026-03-08T00:00:00Z",
			"PeriodEndDate": "2026-03-14T00:00:00Z",
			"Status": 0,
			"TimeReportUid": "abcd-1234",
			"MinutesBillable": 0,
			"MinutesNonBillable": 360,
			"MinutesTotal": 360,
			"TimeEntriesCount": 3,
			"Times": [
				{"TimeID":1,"ItemID":12345,"AppID":42,"Component":9,"TicketID":12345,"TimeDate":"2026-03-09T00:00:00Z","Minutes":60,"TimeTypeID":1,"TimeTypeName":"Development","Status":0},
				{"TimeID":2,"ItemID":12345,"AppID":42,"Component":9,"TicketID":12345,"TimeDate":"2026-03-11T00:00:00Z","Minutes":120,"TimeTypeID":1,"TimeTypeName":"Development","Status":0},
				{"TimeID":3,"ItemID":12345,"AppID":42,"Component":9,"TicketID":12345,"TimeDate":"2026-03-14T00:00:00Z","Minutes":180,"TimeTypeID":1,"TimeTypeName":"Development","Status":0}
			]
		}`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	day := time.Date(2026, 3, 10, 0, 0, 0, 0, domain.EasternTZ)
	report, err := svc.GetWeekReport(context.Background(), profile, day)
	require.NoError(t, err)

	require.Equal(t, "2026-03-08", report.WeekRef.StartDate.Format("2006-01-02"))
	require.Equal(t, "2026-03-14", report.WeekRef.EndDate.Format("2006-01-02"))
	require.Len(t, report.Days, 7)

	// Per-day buckets must survive the DST gap.
	require.Equal(t, 0, report.Days[0].Minutes, "Sun")
	require.Equal(t, 60, report.Days[1].Minutes, "Mon")
	require.Equal(t, 0, report.Days[2].Minutes, "Tue")
	require.Equal(t, 120, report.Days[3].Minutes, "Wed")
	require.Equal(t, 0, report.Days[4].Minutes, "Thu")
	require.Equal(t, 0, report.Days[5].Minutes, "Fri")
	require.Equal(t, 180, report.Days[6].Minutes, "Sat")
}

func TestGetLockedDays_DecodesDateArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/TDWebApi/api/time/locked", r.URL.Path)
		require.Equal(t, "2026-04-01", r.URL.Query().Get("startDate"))
		require.Equal(t, "2026-04-30", r.URL.Query().Get("endDate"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[
			"2026-04-06T00:00:00Z",
			"2026-04-13T00:00:00Z"
		]`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, domain.EasternTZ)
	to := time.Date(2026, 4, 30, 0, 0, 0, 0, domain.EasternTZ)
	days, err := svc.GetLockedDays(context.Background(), profile, from, to)
	require.NoError(t, err)
	require.Len(t, days, 2)
	require.Equal(t, "2026-04-06", days[0].Date.Format("2006-01-02"))
	require.Equal(t, "2026-04-13", days[1].Date.Format("2006-01-02"))
	require.Empty(t, days[0].Reason)
}

func TestGetLockedDays_EmptyRange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	from := time.Date(2026, 5, 1, 0, 0, 0, 0, domain.EasternTZ)
	to := time.Date(2026, 5, 31, 0, 0, 0, 0, domain.EasternTZ)
	days, err := svc.GetLockedDays(context.Background(), profile, from, to)
	require.NoError(t, err)
	require.Empty(t, days)
}
