package week

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func seedProfile(t *testing.T, tenantURL string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

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
}

func TestWeekShow_RendersGrid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/TDWebApi/api/time/report/2026-04-08", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"ID": 1,
			"PeriodStartDate": "2026-04-05T00:00:00Z",
			"PeriodEndDate": "2026-04-11T00:00:00Z",
			"Status": 0,
			"MinutesTotal": 480,
			"TimeEntriesCount": 2,
			"Times": [
				{"TimeID":1,"ItemID":12345,"AppID":42,"Component":9,"TicketID":12345,"TimeDate":"2026-04-06T00:00:00Z","Minutes":240,"TimeTypeID":1,"TimeTypeName":"Development","Status":0,"ItemTitle":"Ingest pipeline"},
				{"TimeID":2,"ItemID":12345,"AppID":42,"Component":9,"TicketID":12345,"TimeDate":"2026-04-07T00:00:00Z","Minutes":240,"TimeTypeID":1,"TimeTypeName":"Development","Status":0,"ItemTitle":"Ingest pipeline"}
			]
		}`))
	}))
	defer srv.Close()

	seedProfile(t, srv.URL)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"show", "2026-04-08"})
	require.NoError(t, cmd.Execute())

	got := out.String()
	require.Contains(t, got, "Week 2026-04-05 — 2026-04-11")
	require.Contains(t, got, "SUN")
	require.Contains(t, got, "SAT")
	require.Contains(t, got, "#12345")
	require.Contains(t, got, "DAY TOTAL")
}

func TestWeekLocked_RendersList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/TDWebApi/api/time/locked", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`["2026-04-06T00:00:00Z","2026-04-13T00:00:00Z"]`))
	}))
	defer srv.Close()

	seedProfile(t, srv.URL)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"locked", "--from", "2026-04-01", "--to", "2026-04-30"})
	require.NoError(t, cmd.Execute())

	got := out.String()
	require.Contains(t, got, "2026-04-06")
	require.Contains(t, got, "2026-04-13")
}

func TestWeekLocked_NoneInRange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	seedProfile(t, srv.URL)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"locked"})
	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "no locked days in range")
}
