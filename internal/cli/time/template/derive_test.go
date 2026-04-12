package template

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ipm/tdx/internal/config"
	"github.com/ipm/tdx/internal/domain"
	"github.com/ipm/tdx/internal/svc/tmplsvc"
	"github.com/stretchr/testify/require"
)

func TestDeriveCmd_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"UID":"uid-abc","FullName":"Test","PrimaryEmail":"test@test.com","ReferenceID":1,"AlternateEmail":""}`))
		case "/TDWebApi/api/time/report/2026-04-05":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ID": 1,
				"PeriodStartDate": "2026-04-05T00:00:00Z",
				"PeriodEndDate": "2026-04-11T00:00:00Z",
				"Status": 0,
				"TimeReportUid": "uid-abc",
				"UserFullName": "Test",
				"MinutesBillable": 0,
				"MinutesNonBillable": 240,
				"MinutesTotal": 240,
				"TimeEntriesCount": 2,
				"Times": [
					{"TimeID":1,"Component":1,"ProjectID":54,"ItemID":54,"ItemTitle":"Proj","ProjectName":"Proj","TimeTypeID":5,"TimeTypeName":"Dev","TimeDate":"2026-04-06T00:00:00Z","Minutes":120,"Description":"work","Billable":false,"Uid":"uid-abc","Status":0,"StatusDate":"0001-01-01T00:00:00","CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00","AppID":0,"AppName":"","TicketID":0,"PlanID":0,"PortfolioID":0,"Limited":false,"FunctionalRoleId":0},
					{"TimeID":2,"Component":1,"ProjectID":54,"ItemID":54,"ItemTitle":"Proj","ProjectName":"Proj","TimeTypeID":5,"TimeTypeName":"Dev","TimeDate":"2026-04-07T00:00:00Z","Minutes":120,"Description":"work","Billable":false,"Uid":"uid-abc","Status":0,"StatusDate":"0001-01-01T00:00:00","CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00","AppID":0,"AppName":"","TicketID":0,"PlanID":0,"PortfolioID":0,"Limited":false,"FunctionalRoleId":0}
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

	// Build config dir pointing at the test server.
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)
	paths := config.Paths{
		Root:            dir,
		ConfigFile:      filepath.Join(dir, "config.yaml"),
		CredentialsFile: filepath.Join(dir, "credentials.yaml"),
		TemplatesDir:    filepath.Join(dir, "templates"),
	}
	ps := config.NewProfileStore(paths)
	require.NoError(t, ps.AddProfile(domain.Profile{Name: "default", TenantBaseURL: srv.URL}))
	cs := config.NewCredentialsStore(paths)
	require.NoError(t, cs.SetToken("default", "good-token"))

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"derive", "test-week", "--from-week", "2026-04-05"})
	require.NoError(t, cmd.Execute())

	got := out.String()
	require.Contains(t, got, `derived template "test-week"`)

	// Template file must exist on disk.
	store := tmplsvc.NewStore(paths)
	require.True(t, store.Exists("test-week"))
}

func TestDeriveCmd_MissingFromWeek(t *testing.T) {
	seedTemplateDir(t)

	cmd := NewCmd()
	cmd.SetArgs([]string{"derive", "test-week"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--from-week is required")
}
