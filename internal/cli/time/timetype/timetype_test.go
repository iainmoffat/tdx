package timetype

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ipm/tdx/internal/config"
	"github.com/ipm/tdx/internal/domain"
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

func TestTypeList_RendersTable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/TDWebApi/api/time/types", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[
			{"ID":1,"Name":"Development","Code":"DEV","HelpText":"writing code","IsBillable":true,"IsLimited":false,"IsActive":true},
			{"ID":17,"Name":"General Admin","IsBillable":false,"IsLimited":false,"IsActive":true}
		]`))
	}))
	defer srv.Close()

	seedProfile(t, srv.URL)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"list"})
	require.NoError(t, cmd.Execute())

	got := out.String()
	require.Contains(t, got, "ID")
	require.Contains(t, got, "NAME")
	require.Contains(t, got, "Development")
	require.Contains(t, got, "General Admin")
	require.Contains(t, got, "true")  // billable
	require.Contains(t, got, "false") // limited
}

func TestTypeFor_TicketKind(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/TDWebApi/api/time/types/component/app/42/ticket/12345", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"ID":1,"Name":"Development","IsBillable":true,"IsActive":true}]`))
	}))
	defer srv.Close()

	seedProfile(t, srv.URL)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"for", "ticket", "12345", "--app", "42"})
	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "Development")
}

func TestTypeFor_UnknownKind(t *testing.T) {
	seedProfile(t, "http://127.0.0.1/")

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"for", "nonsense", "1", "--app", "42"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error()+out.String(), "unknown kind")
}

func TestTypeFor_TicketRequiresApp(t *testing.T) {
	seedProfile(t, "http://127.0.0.1/")

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"for", "ticket", "12345"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error()+out.String(), "--app")
}
