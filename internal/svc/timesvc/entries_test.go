package timesvc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
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
				"TimeTypeName": "Development",
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
	_, hasMaxResults := seenBody["MaxResults"]
	require.False(t, hasMaxResults, "MaxResults should be omitted when Limit is 0")
}

func TestGetEntry_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/TDWebApi/api/time/987654", r.URL.Path)
		require.Equal(t, "GET", r.Method)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"TimeID": 987654,
			"ItemID": 12345,
			"ItemTitle": "Ingest pipeline",
			"AppID": 42,
			"Component": 9,
			"TicketID": 12345,
			"TimeDate": "2026-04-06T00:00:00Z",
			"Minutes": 120,
			"TimeTypeID": 1,
			"TimeTypeName": "Development",
			"Status": 0
		}`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	entry, err := svc.GetEntry(context.Background(), profile, 987654)
	require.NoError(t, err)
	require.Equal(t, 987654, entry.ID)
	require.Equal(t, domain.TargetTicket, entry.Target.Kind)
}

func TestGetEntry_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`entry not found`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	_, err := svc.GetEntry(context.Background(), profile, 999)
	require.ErrorIs(t, err, domain.ErrEntryNotFound)
}

func TestGetEntry_ResolvesTimeTypeName(t *testing.T) {
	// Parallel of TestSearchEntries_ResolvesTimeTypeNamesViaListTimeTypes
	// for the GetEntry single-entry path. TD's /api/time/{id} response
	// has the same TimeTypeID-without-TimeTypeName issue as the search
	// endpoint, so the side-join must fire here too.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/TDWebApi/api/time/987654":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"TimeID": 987654,
				"ItemID": 12345,
				"AppID": 42,
				"Component": 9,
				"TicketID": 12345,
				"TimeDate": "2026-04-06T00:00:00Z",
				"Minutes": 60,
				"TimeTypeID": 5,
				"Status": 0
			}`))
		case "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"ID":5,"Name":"Standard Activities","Code":"Standard","IsBillable":false,"IsActive":true}]`))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	entry, err := svc.GetEntry(context.Background(), profile, 987654)
	require.NoError(t, err)
	require.Equal(t, 987654, entry.ID)
	require.Equal(t, 5, entry.TimeType.ID)
	require.Equal(t, "Standard Activities", entry.TimeType.Name)
	require.Equal(t, "Standard", entry.TimeType.Code)
}

func TestSearchEntries_ResolvesTimeTypeNamesViaListTimeTypes(t *testing.T) {
	// TD's /api/time/search returns TimeTypeID without TimeTypeName.
	// Verify that timesvc side-joins with /api/time/types to populate
	// the missing names so callers see fully-populated TimeType objects.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/TDWebApi/api/time/search":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"TimeID":1,"ItemID":12345,"AppID":42,"Component":9,"TicketID":12345,"TimeDate":"2026-04-06T00:00:00Z","Minutes":120,"TimeTypeID":5,"Status":0},
				{"TimeID":2,"ItemID":12345,"AppID":42,"Component":9,"TicketID":12345,"TimeDate":"2026-04-07T00:00:00Z","Minutes":60,"TimeTypeID":3,"Status":0}
			]`))
		case "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"ID":3,"Name":"Leave","Code":"Leave","IsBillable":false,"IsActive":true},
				{"ID":5,"Name":"Standard Activities","Code":"Standard","IsBillable":false,"IsActive":true}
			]`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	entries, err := svc.SearchEntries(context.Background(), profile, domain.EntryFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 2)

	// First entry: TimeTypeID 5 → "Standard Activities"
	require.Equal(t, 5, entries[0].TimeType.ID)
	require.Equal(t, "Standard Activities", entries[0].TimeType.Name)
	require.Equal(t, "Standard", entries[0].TimeType.Code)

	// Second entry: TimeTypeID 3 → "Leave"
	require.Equal(t, 3, entries[1].TimeType.ID)
	require.Equal(t, "Leave", entries[1].TimeType.Name)
}

func TestSearchEntries_ResolveSkippedWhenNamesAlreadyPresent(t *testing.T) {
	// If the wire response already has TimeTypeName populated (e.g. a future
	// TD endpoint or a test fixture), the resolve helper must NOT make a
	// second API call to /api/time/types. The test server only registers
	// /api/time/search; if /types is hit, the test fails.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/TDWebApi/api/time/search":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"TimeID":1,"ItemID":12345,"AppID":42,"Component":9,"TicketID":12345,"TimeDate":"2026-04-06T00:00:00Z","Minutes":60,"TimeTypeID":5,"TimeTypeName":"Already Set","Status":0}]`))
		case "/TDWebApi/api/time/types":
			// If the resolve early-out is ever accidentally broken, this 500 will
			// propagate up through ListTimeTypes → resolveTimeTypeNames →
			// SearchEntries, surfacing as an error that require.NoError catches
			// from the test goroutine. (t.Fatalf from a goroutine other than the
			// test goroutine doesn't actually fail the test.)
			w.WriteHeader(http.StatusInternalServerError)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	entries, err := svc.SearchEntries(context.Background(), profile, domain.EntryFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "Already Set", entries[0].TimeType.Name)
}
