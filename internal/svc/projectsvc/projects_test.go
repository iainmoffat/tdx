package projectsvc

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestListMine_DecodesPlanRows(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/TDWebApi/api/projects/list" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[
			{"ID": 1292, "Title": "FY2026 DR Plan", "ProjectID": 259, "ProjectName": "Disaster Recovery",
			 "TaskCount": 12, "MyTaskCount": 3, "PlanType": 1,
			 "StartDateUtc": "2025-07-01T00:00:00", "EndDateUtc": "2026-06-30T00:00:00"}
		]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	plans, err := svc.ListMine(context.Background(), prof)
	require.NoError(t, err)
	require.Len(t, plans, 1)
	p := plans[0]
	require.Equal(t, 1292, p.ID)
	require.Equal(t, 259, p.ProjectID)
	require.Equal(t, "Disaster Recovery", p.ProjectName)
	require.Equal(t, "FY2026 DR Plan", p.Title)
	require.Equal(t, 3, p.MyTaskCount)
	require.Equal(t, domain.PlanWaterfall, p.Type)
	require.False(t, p.StartDate.IsZero())
}

func TestSearch_PostsExpectedBody(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		capturedBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	trueVal := true
	_, err := svc.Search(context.Background(), prof, domain.ProjectSearchFilter{
		NameLike: "Disaster",
		IsActive: &trueVal,
	})
	require.NoError(t, err)

	var sent map[string]interface{}
	require.NoError(t, json.Unmarshal(capturedBody, &sent))
	require.Equal(t, "Disaster", sent["NameLike"])
	require.Equal(t, true, sent["IsActive"])
}

func TestGet_DecodesProjectWithAdminAsManager(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/TDWebApi/api/projects/259" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"ID": 259, "Name": "Disaster Recovery",
			"StatusID": 3, "StatusName": "Executing",
			"AdminUID": "abc-manager-uid", "AdminName": "Charlotte Looney",
			"SponsorUID": "def-sponsor-uid", "SponsorName": "Elias Eldayrie",
			"PercentComplete": 96.0, "IsActive": true,
			"StartDate": "2025-07-01T00:00:00", "EndDate": "2026-06-30T00:00:00"
		}`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	p, err := svc.Get(context.Background(), prof, 259)
	require.NoError(t, err)
	require.Equal(t, 259, p.ID)
	require.Equal(t, "Disaster Recovery", p.Name)
	require.Equal(t, "abc-manager-uid", p.ManagerUID)
	require.Equal(t, "Charlotte Looney", p.ManagerName)
	require.Equal(t, "def-sponsor-uid", p.SponsorUID)
	require.Equal(t, "Executing", p.StatusName)
	require.InDelta(t, 96.0, p.PercentComplete, 0.001)
	require.True(t, p.IsActive)
}

func TestSearch_ClientSideManagerFilter(t *testing.T) {
	// Verify that when ManagerUID is set, it's included in the request body.
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`[
			{"ID": 100, "Name": "Project A", "AdminUID": "uid-manager"},
			{"ID": 101, "Name": "Project B", "AdminUID": "uid-other"}
		]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	results, err := svc.Search(context.Background(), prof, domain.ProjectSearchFilter{
		ManagerUID: "uid-manager",
	})
	require.NoError(t, err)
	require.Len(t, results, 2) // service returns all rows; CLI layer may filter
	require.Contains(t, string(capturedBody), `"ManagerUID":"uid-manager"`)
}

func TestParseTD_Formats(t *testing.T) {
	cases := []struct {
		in   string
		zero bool
	}{
		{"2026-05-01T10:00:00Z", false},
		{"2026-05-01T10:00:00.123Z", false},
		{"2026-05-01T10:00:00-04:00", false},
		{"2026-05-01T10:00:00", false},
		{"0001-01-01T00:00:00", true},
		{"1900-01-01T00:00:00", true},
		{"", true},
		{"not-a-date", true},
	}
	for _, tc := range cases {
		got := parseTD(tc.in)
		if tc.zero && !got.IsZero() {
			t.Errorf("parseTD(%q) should be zero, got %v", tc.in, got)
		}
		if !tc.zero && got.IsZero() {
			t.Errorf("parseTD(%q) should not be zero", tc.in)
		}
	}
}

// Suppress unused import for bytes used in other tests.
var _ = bytes.NewBuffer
