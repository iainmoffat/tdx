package timesvc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestGetWeekReportForUser_HappyPath(t *testing.T) {
	var requestedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"PeriodStartDate": "2026-04-12T00:00:00Z",
			"PeriodEndDate":   "2026-04-18T00:00:00Z",
			"Status":          1,
			"Times":           [],
			"TimeReportUid":   "target-uid",
			"MinutesBillable": 240,
			"MinutesNonBillable": 0,
			"MinutesTotal":    240
		}`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	report, err := svc.GetWeekReportForUser(
		context.Background(), profile,
		time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ),
		"target-uid",
	)
	require.NoError(t, err)
	require.Equal(t, "/TDWebApi/api/time/report/2026-04-14/target-uid", requestedPath)
	require.Equal(t, "target-uid", report.UserUID)
	require.Equal(t, 240, report.MinutesBillable)
}

func TestGetWeekReportForUser_PermissionMappedFor401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`unauthorized`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	_, err := svc.GetWeekReportForUser(context.Background(), profile,
		time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ), "target-uid")
	require.Error(t, err)
	require.True(t, errors.Is(err, domain.ErrPermission), "401 should wrap ErrPermission")
}

func TestGetWeekReportForUser_PermissionMappedFor403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`forbidden`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	_, err := svc.GetWeekReportForUser(context.Background(), profile,
		time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ), "target-uid")
	require.Error(t, err)
	require.True(t, errors.Is(err, domain.ErrPermission), "403 should wrap ErrPermission")
}
