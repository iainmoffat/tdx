package tmplsvc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
	"github.com/stretchr/testify/require"
)

// tmplHarness returns (config.Paths, *timesvc.Service) rooted at a temp dir
// with one "default" profile pointing at tenantURL and a stored token.
func tmplHarness(t *testing.T, tenantURL string) (config.Paths, *timesvc.Service) {
	t.Helper()
	dir := t.TempDir()
	paths := config.Paths{
		Root:            dir,
		ConfigFile:      filepath.Join(dir, "config.yaml"),
		CredentialsFile: filepath.Join(dir, "credentials.yaml"),
		TemplatesDir:    filepath.Join(dir, "templates"),
	}
	ps := config.NewProfileStore(paths)
	require.NoError(t, ps.AddProfile(domain.Profile{
		Name:          "default",
		TenantBaseURL: tenantURL,
	}))
	cs := config.NewCredentialsStore(paths)
	require.NoError(t, cs.SetToken("default", "good-token"))
	return paths, timesvc.New(paths)
}

// TestDerive_GroupsEntriesByTargetAndType verifies that Derive correctly
// groups 3 entries into 2 rows:
//   - Entries 1+2 share (project/54, type 5) → one row with 4.0 total hours
//   - Entry 3 has (ticket/100, type 5) → a separate row with 2.0 hours
//
// Rows are sorted by total hours descending, so the project row comes first.
func TestDerive_GroupsEntriesByTargetAndType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/TDWebApi/api/time/report/2026-04-06":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ID": 1,
				"PeriodStartDate": "2026-04-05T00:00:00Z",
				"PeriodEndDate": "2026-04-11T00:00:00Z",
				"Status": 0,
				"TimeReportUid": "uid",
				"UserFullName": "User",
				"MinutesBillable": 0,
				"MinutesNonBillable": 0,
				"MinutesTotal": 360,
				"TimeEntriesCount": 3,
				"Times": [
					{"TimeID":1,"Component":1,"ProjectID":54,"ItemID":54,"ItemTitle":"Proj","ProjectName":"Proj","TimeTypeID":5,"TimeTypeName":"","TimeDate":"2026-04-07T00:00:00Z","Minutes":120,"Description":"work","Billable":false,"Uid":"uid","Status":0,"StatusDate":"0001-01-01T00:00:00","CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00","AppID":0,"AppName":"","TicketID":0,"PlanID":0,"PortfolioID":0,"Limited":false,"FunctionalRoleId":0},
					{"TimeID":2,"Component":1,"ProjectID":54,"ItemID":54,"ItemTitle":"Proj","ProjectName":"Proj","TimeTypeID":5,"TimeTypeName":"","TimeDate":"2026-04-08T00:00:00Z","Minutes":120,"Description":"work","Billable":false,"Uid":"uid","Status":0,"StatusDate":"0001-01-01T00:00:00","CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00","AppID":0,"AppName":"","TicketID":0,"PlanID":0,"PortfolioID":0,"Limited":false,"FunctionalRoleId":0},
					{"TimeID":3,"Component":9,"ProjectID":0,"ItemID":100,"ItemTitle":"Bug","ProjectName":"","TimeTypeID":5,"TimeTypeName":"","TimeDate":"2026-04-07T00:00:00Z","Minutes":120,"Description":"fix","Billable":false,"Uid":"uid","Status":0,"StatusDate":"0001-01-01T00:00:00","CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00","AppID":42,"AppName":"","TicketID":100,"PlanID":0,"PortfolioID":0,"Limited":false,"FunctionalRoleId":0}
				]
			}`))
		case "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"ID":5,"Name":"Dev","IsActive":true}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	paths, tsvc := tmplHarness(t, srv.URL)
	svc := New(paths, tsvc)

	weekDate := time.Date(2026, 4, 6, 0, 0, 0, 0, domain.EasternTZ)
	tmpl, err := svc.Derive(context.Background(), "default", "my-week", weekDate)
	require.NoError(t, err)

	require.Equal(t, "my-week", tmpl.Name)
	require.Len(t, tmpl.Rows, 2)

	// Rows sorted by total hours descending: project row (4h) before ticket row (2h).
	require.Equal(t, "row-01", tmpl.Rows[0].ID)
	require.Equal(t, "row-02", tmpl.Rows[1].ID)

	require.InDelta(t, 4.0, tmpl.Rows[0].Hours.Total(), 0.001)
	require.InDelta(t, 2.0, tmpl.Rows[1].Hours.Total(), 0.001)

	// First row is the project entry (two days, 2h each).
	require.Equal(t, domain.TargetProject, tmpl.Rows[0].Target.Kind)
	require.Equal(t, 54, tmpl.Rows[0].Target.ItemID)

	// Second row is the ticket entry.
	require.Equal(t, domain.TargetTicket, tmpl.Rows[1].Target.Kind)
	require.Equal(t, 100, tmpl.Rows[1].Target.ItemID)

	// DerivedFrom metadata must be populated.
	require.NotNil(t, tmpl.DerivedFrom)
	require.Equal(t, "default", tmpl.DerivedFrom.Profile)
	require.Equal(t, "2026-04-05", tmpl.DerivedFrom.WeekStart.Format("2006-01-02"))

	// Template should be persisted.
	require.True(t, svc.store.Exists("my-week"))
}

// TestDerive_AlreadyExists verifies that Derive returns an error when a
// template with the same name already exists.
func TestDerive_AlreadyExists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	paths, tsvc := tmplHarness(t, srv.URL)
	svc := New(paths, tsvc)

	// Seed an existing template.
	existing := sampleTemplate()
	existing.Name = "conflict"
	require.NoError(t, svc.store.Save(existing))

	weekDate := time.Date(2026, 4, 6, 0, 0, 0, 0, domain.EasternTZ)
	_, err := svc.Derive(context.Background(), "default", "conflict", weekDate)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists")
}

// TestDerive_EmptyWeek verifies that Derive returns an error when there are
// no entries in the specified week.
func TestDerive_EmptyWeek(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/TDWebApi/api/time/report/2026-04-06":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ID": 1,
				"PeriodStartDate": "2026-04-05T00:00:00Z",
				"PeriodEndDate": "2026-04-11T00:00:00Z",
				"Status": 0,
				"TimeReportUid": "uid",
				"UserFullName": "User",
				"MinutesBillable": 0,
				"MinutesNonBillable": 0,
				"MinutesTotal": 0,
				"TimeEntriesCount": 0,
				"Times": []
			}`))
		case "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"ID":5,"Name":"Dev","IsActive":true}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	paths, tsvc := tmplHarness(t, srv.URL)
	svc := New(paths, tsvc)

	weekDate := time.Date(2026, 4, 6, 0, 0, 0, 0, domain.EasternTZ)
	_, err := svc.Derive(context.Background(), "default", "empty-week", weekDate)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no entries")
}
