package projectsvc

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestSearchPlans_DecodesPlanRows(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/TDWebApi/api/projects/259/plans/search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		_, _ = w.Write([]byte(`[
			{"ID": 1292, "Title": "FY2026 DR Waterfall", "ProjectID": 259, "ProjectName": "DR Project",
			 "PlanType": 1, "TaskCount": 10, "MyTaskCount": 2,
			 "StartDateUtc": "2025-07-01T00:00:00", "EndDateUtc": "2026-06-30T00:00:00"},
			{"ID": 1300, "Title": "DR Cardwall", "ProjectID": 259, "ProjectName": "DR Project",
			 "PlanType": 2, "TaskCount": 5, "MyTaskCount": 0}
		]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	plans, err := svc.SearchPlans(context.Background(), prof, 259, "", false)
	require.NoError(t, err)
	require.Len(t, plans, 2)
	require.Equal(t, 1292, plans[0].ID)
	require.Equal(t, domain.PlanWaterfall, plans[0].Type)
	require.Equal(t, 2, plans[0].MyTaskCount)
	require.Equal(t, domain.PlanCardwall, plans[1].Type)
}

func TestSearchPlans_SendsNameLike(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	_, err := svc.SearchPlans(context.Background(), prof, 259, "FY2026", false)
	require.NoError(t, err)
	require.True(t, strings.Contains(string(capturedBody), `"NameLike":"FY2026"`))
}
