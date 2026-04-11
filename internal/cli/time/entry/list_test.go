package entry

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

// seedProfile writes a profile + token into TDX_CONFIG_HOME and returns
// the temp dir path so the test can pre-populate dependent state.
func seedProfile(t *testing.T, tenantURL string) string {
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
	return dir
}

func TestEntryList_WithExplicitRange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/TDWebApi/api/time/search":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{
					"TimeID": 1,
					"ItemID": 12345,
					"ItemTitle": "Ingest pipeline",
					"AppID": 42,
					"Component": 9,
					"TicketID": 12345,
					"TimeDate": "2026-04-06T00:00:00Z",
					"Minutes": 120,
					"Description": "Investigating the ingest bug",
					"TimeTypeID": 1,
					"TimeTypeName": "Development",
					"Status": 0
				}
			]`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	seedProfile(t, srv.URL)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"list", "--from", "2026-04-05", "--to", "2026-04-11", "--user", "abcd-1234"})
	require.NoError(t, cmd.Execute())

	got := out.String()
	require.Contains(t, got, "DATE")
	require.Contains(t, got, "HOURS")
	require.Contains(t, got, "2026-04-06")
	require.Contains(t, got, "2.00")
	require.Contains(t, got, "Development")
	require.Contains(t, got, "#12345")
	require.Contains(t, got, "TOTAL")
}

func TestEntryList_JSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[
			{"TimeID":1,"ItemID":12345,"AppID":42,"Component":9,"TicketID":12345,"TimeDate":"2026-04-06T00:00:00Z","Minutes":120,"TimeTypeID":1,"TimeTypeName":"Development","Status":0}
		]`))
	}))
	defer srv.Close()

	seedProfile(t, srv.URL)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"list", "--from", "2026-04-05", "--to", "2026-04-11", "--user", "abcd-1234", "--json"})
	require.NoError(t, cmd.Execute())

	got := out.String()
	require.Contains(t, got, `"schema": "tdx.v1.entryList"`)
	require.Contains(t, got, `"minutes": 120`)
}

func TestEntryList_TicketRequiresApp(t *testing.T) {
	seedProfile(t, "http://127.0.0.1/")

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"list", "--ticket", "12345"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error()+out.String(), "--ticket requires --app")
}

func TestEntryList_DefaultFilterUsesWhoami(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ReferenceID":42,"UID":"default-user","FullName":"Default User","PrimaryEmail":"me@ufl.edu"}`))
		case "/TDWebApi/api/time/search":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	seedProfile(t, srv.URL)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"list"})
	require.NoError(t, cmd.Execute())
	// Empty result body is acceptable; the important thing is that the
	// command completed without erroring, which means whoami resolved
	// and the default "this week, me" filter was built.
	require.Contains(t, out.String(), "TOTAL")
}
