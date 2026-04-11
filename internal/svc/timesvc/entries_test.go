package timesvc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ipm/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestSearchEntries_SendsCorrectRequestBody(t *testing.T) {
	var seenBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/TDWebApi/api/time/search", r.URL.Path)
		require.Equal(t, "POST", r.Method)
		b, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(b, &seenBody))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)

	from := time.Date(2026, 4, 5, 0, 0, 0, 0, domain.EasternTZ)
	to := time.Date(2026, 4, 11, 0, 0, 0, 0, domain.EasternTZ)
	_, err := svc.SearchEntries(context.Background(), profile, domain.EntryFilter{
		DateRange: domain.DateRange{From: from, To: to},
		UserUID:   "abcd-1234",
		Limit:     100,
	})
	require.NoError(t, err)

	require.Contains(t, seenBody, "EntryDateFrom")
	require.Contains(t, seenBody, "EntryDateTo")
	require.Equal(t, []any{"abcd-1234"}, seenBody["PersonUIDs"])
	require.Equal(t, float64(100), seenBody["MaxResults"])
}

func TestSearchEntries_DecodesTicketEntry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[
			{
				"TimeID": 987654,
				"ItemID": 12345,
				"ItemTitle": "Ingest pipeline",
				"AppID": 42,
				"AppName": "IT Help Desk",
				"Component": 9,
				"TicketID": 12345,
				"TimeDate": "2026-04-06T00:00:00Z",
				"Minutes": 120,
				"Description": "Investigating the ingest bug",
				"TimeTypeID": 1,
				"TimeTypeName": "Development",
				"Billable": false,
				"Uid": "abcd-1234",
				"Status": 0,
				"CreatedDate": "2026-04-06T15:30:00Z",
				"ModifiedDate": "2026-04-06T15:30:00Z"
			}
		]`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	entries, err := svc.SearchEntries(context.Background(), profile, domain.EntryFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 1)

	e := entries[0]
	require.Equal(t, 987654, e.ID)
	require.Equal(t, "abcd-1234", e.UserUID)
	require.Equal(t, domain.TargetTicket, e.Target.Kind)
	require.Equal(t, 42, e.Target.AppID)
	require.Equal(t, 12345, e.Target.ItemID)
	require.Equal(t, "Ingest pipeline", e.Target.DisplayName)
	require.Equal(t, "#12345", e.Target.DisplayRef)
	require.Equal(t, 1, e.TimeType.ID)
	require.Equal(t, "Development", e.TimeType.Name)
	require.Equal(t, 120, e.Minutes)
	require.Equal(t, "Investigating the ingest bug", e.Description)
	require.Equal(t, domain.ReportOpen, e.ReportStatus)
}

func TestSearchEntries_DecodesProjectEntry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[
			{
				"TimeID": 111,
				"ItemID": 9999,
				"ItemTitle": "Platform Services",
				"AppID": 5,
				"Component": 1,
				"ProjectID": 9999,
				"ProjectName": "Platform Services",
				"TimeDate": "2026-04-06T00:00:00Z",
				"Minutes": 90,
				"TimeTypeID": 17,
				"TimeTypeName": "General Admin",
				"Status": 1
			}
		]`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	entries, err := svc.SearchEntries(context.Background(), profile, domain.EntryFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, domain.TargetProject, entries[0].Target.Kind)
	require.Equal(t, 9999, entries[0].Target.ItemID)
	require.Equal(t, domain.ReportSubmitted, entries[0].ReportStatus)
}

func TestSearchEntries_DecodesTicketTaskEntry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[
			{
				"TimeID": 222,
				"ItemID": 7,
				"ItemTitle": "Sub-task",
				"AppID": 42,
				"Component": 25,
				"TicketID": 12345,
				"TimeTypeID": 1,
				"Status": 3
			}
		]`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	entries, err := svc.SearchEntries(context.Background(), profile, domain.EntryFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, domain.TargetTicketTask, entries[0].Target.Kind)
	require.Equal(t, 12345, entries[0].Target.ItemID) // ticket ID
	require.Equal(t, 7, entries[0].Target.TaskID)     // task ID
	require.Equal(t, domain.ReportApproved, entries[0].ReportStatus)
}

func TestSearchEntries_UnknownComponentReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"TimeID":1,"Component":999}]`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	_, err := svc.SearchEntries(context.Background(), profile, domain.EntryFilter{})
	require.ErrorIs(t, err, domain.ErrUnsupportedTargetKind)
}

func TestSearchEntries_NoUserFilterOmitsPersonUIDs(t *testing.T) {
	var seenBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &seenBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	_, err := svc.SearchEntries(context.Background(), profile, domain.EntryFilter{})
	require.NoError(t, err)
	_, hasPersonUIDs := seenBody["PersonUIDs"]
	require.False(t, hasPersonUIDs, "PersonUIDs should be omitted when UserUID is empty")
}
