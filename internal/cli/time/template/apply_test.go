package template

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ipm/tdx/internal/config"
	"github.com/ipm/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

// testTemplate returns a minimal template for apply tests.
// One row: project/54 with Dev(ID=5), Mon–Fri 2h each.
func testTemplate() domain.Template {
	return domain.Template{
		SchemaVersion: 1,
		Name:          "test-tmpl",
		Description:   "Test template",
		Rows: []domain.TemplateRow{
			{
				ID:    "row-01",
				Label: "Project Alpha",
				Target: domain.Target{
					Kind:   domain.TargetProject,
					ItemID: 54,
				},
				TimeType: domain.TimeType{
					ID:   5,
					Name: "Dev",
				},
				Hours: domain.WeekHours{Mon: 2, Tue: 2, Wed: 2, Thu: 2, Fri: 2},
				ResolverHints: domain.ResolverHints{
					TargetDisplayName: "Proj",
				},
			},
		},
	}
}

// setupApplyServer creates a test server for apply tests.
// It serves:
//   - GET /TDWebApi/api/auth/getuser → user identity
//   - GET /TDWebApi/api/time/report/2026-04-05 → empty week report
//   - GET /TDWebApi/api/time/locked → no locked days
//   - GET /TDWebApi/api/time/types → time type list
//
// If handlePost is true, it also handles:
//   - POST /TDWebApi/api/time → bulk create (success)
//   - GET /TDWebApi/api/time/100 → get created entry
func setupApplyServer(t *testing.T, handlePost bool) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	postCount := &atomic.Int32{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/TDWebApi/api/auth/getuser" && r.Method == "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"UID":"uid-abc","FullName":"Test User","PrimaryEmail":"test@test.com","ReferenceID":1,"AlternateEmail":""}`))

		case r.URL.Path == "/TDWebApi/api/time/report/2026-04-05" && r.Method == "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ID": 1,
				"PeriodStartDate": "2026-04-05T00:00:00Z",
				"PeriodEndDate": "2026-04-11T00:00:00Z",
				"Status": 0,
				"TimeReportUid": "uid-abc",
				"UserFullName": "Test User",
				"MinutesBillable": 0,
				"MinutesNonBillable": 0,
				"MinutesTotal": 0,
				"TimeEntriesCount": 0,
				"Times": []
			}`))

		case r.URL.Path == "/TDWebApi/api/time/locked" && r.Method == "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))

		case r.URL.Path == "/TDWebApi/api/time/types" && r.Method == "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"ID":5,"Name":"Dev","IsActive":true}]`))

		case handlePost && r.URL.Path == "/TDWebApi/api/time" && r.Method == "POST":
			postCount.Add(1)
			// Parse the batch; return one success per entry.
			var entries []json.RawMessage
			if err := json.NewDecoder(r.Body).Decode(&entries); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			var succeeded []map[string]int
			for i := range entries {
				succeeded = append(succeeded, map[string]int{"Index": i, "ID": 100 + i})
			}
			resp := map[string]any{"Succeeded": succeeded, "Failed": []any{}}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)

		case handlePost && strings.HasPrefix(r.URL.Path, "/TDWebApi/api/time/") && r.Method == "GET":
			// GET /TDWebApi/api/time/{id} — return a valid entry for
			// the re-fetch after create.
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"TimeID": 100,
				"Component": 1,
				"ProjectID": 54,
				"ItemID": 54,
				"ItemTitle": "Proj",
				"ProjectName": "Proj",
				"TimeTypeID": 5,
				"TimeTypeName": "Dev",
				"TimeDate": "2026-04-06T00:00:00Z",
				"Minutes": 120,
				"Description": "",
				"Billable": false,
				"Uid": "uid-abc",
				"Status": 0,
				"StatusDate": "0001-01-01T00:00:00",
				"CreatedDate": "0001-01-01T00:00:00",
				"ModifiedDate": "0001-01-01T00:00:00",
				"AppID": 0,
				"AppName": "",
				"TicketID": 0,
				"PlanID": 0,
				"PortfolioID": 0,
				"Limited": false,
				"FunctionalRoleId": 0
			}`))

		default:
			t.Logf("unhandled request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	return srv, postCount
}

// setupApplyEnv creates a temp config dir pointing at srv, seeds a template,
// and returns the config dir path.
func setupApplyEnv(t *testing.T, srvURL string) string {
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
	require.NoError(t, ps.AddProfile(domain.Profile{Name: "default", TenantBaseURL: srvURL}))
	cs := config.NewCredentialsStore(paths)
	require.NoError(t, cs.SetToken("default", "test-token"))

	writeTestTemplate(t, dir, testTemplate())
	return dir
}

func TestApplyCmd_DryRun(t *testing.T) {
	srv, postCount := setupApplyServer(t, false)
	defer srv.Close()

	setupApplyEnv(t, srv.URL)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"apply", "test-tmpl", "--week", "2026-04-05", "--dry-run"})
	require.NoError(t, cmd.Execute())

	got := out.String()
	// Dry-run must show the preview with create markers.
	require.Contains(t, got, "create", "dry-run output should mention creates")
	// Dry-run must NOT issue any POST requests.
	require.Equal(t, int32(0), postCount.Load(), "dry-run must not POST time entries")
}

func TestApplyCmd_YesFlag(t *testing.T) {
	srv, postCount := setupApplyServer(t, true)
	defer srv.Close()

	setupApplyEnv(t, srv.URL)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"apply", "test-tmpl", "--week", "2026-04-05", "--yes"})
	require.NoError(t, cmd.Execute())

	got := out.String()
	// Should show the result summary.
	require.Contains(t, got, "applied", "output should confirm entries were applied")
	// POST must have been called (reconcile runs twice: once for preview, once inside Apply).
	require.Greater(t, postCount.Load(), int32(0), "--yes must trigger POST to create entries")
}

func TestApplyCmd_MissingWeek(t *testing.T) {
	_ = seedTemplateDir(t)

	cmd := NewCmd()
	cmd.SetArgs([]string{"apply", "test-tmpl"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--week is required")
}

func TestApplyCmd_NoConfirm(t *testing.T) {
	srv, postCount := setupApplyServer(t, false)
	defer srv.Close()

	setupApplyEnv(t, srv.URL)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"apply", "test-tmpl", "--week", "2026-04-05"})
	require.NoError(t, cmd.Execute())

	got := out.String()
	// Should show the preview and a message about --yes.
	require.Contains(t, got, "--yes", "should instruct user to use --yes")
	require.Equal(t, int32(0), postCount.Load(), "no --yes means no POST")
}

func TestParseDays_Range(t *testing.T) {
	days, err := parseDays("mon-thu")
	require.NoError(t, err)
	require.Len(t, days, 4)
	// Monday=1, Tuesday=2, Wednesday=3, Thursday=4
	for i, expected := range []int{1, 2, 3, 4} {
		require.Equal(t, expected, int(days[i]), "day %d", i)
	}
}

func TestParseDays_List(t *testing.T) {
	days, err := parseDays("mon,wed,fri")
	require.NoError(t, err)
	require.Len(t, days, 3)
	require.Equal(t, 1, int(days[0])) // Monday
	require.Equal(t, 3, int(days[1])) // Wednesday
	require.Equal(t, 5, int(days[2])) // Friday
}

func TestParseDays_Invalid(t *testing.T) {
	_, err := parseDays("foo")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown day")
}

func TestParseOverride(t *testing.T) {
	ov, err := parseOverride("row-01:fri=4")
	require.NoError(t, err)
	require.Equal(t, "row-01", ov.RowID)
	require.Equal(t, 5, int(ov.Day)) // Friday
	require.InDelta(t, 4.0, ov.Hours, 0.001)
}

func TestParseOverride_Invalid(t *testing.T) {
	_, err := parseOverride("bad-format")
	require.Error(t, err)
}

func TestApplyCmd_JSONFlag(t *testing.T) {
	srv, _ := setupApplyServer(t, false)
	defer srv.Close()

	setupApplyEnv(t, srv.URL)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"apply", "test-tmpl", "--week", "2026-04-05", "--json"})
	require.NoError(t, cmd.Execute())

	got := out.String()
	// Should contain JSON with diffHash.
	require.Contains(t, got, "diffHash", "JSON output should include diffHash")
}

func TestApplyCmd_DaysFilter(t *testing.T) {
	srv, _ := setupApplyServer(t, false)
	defer srv.Close()

	setupApplyEnv(t, srv.URL)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"apply", "test-tmpl", "--week", "2026-04-05", "--days", "mon,tue", "--dry-run"})
	require.NoError(t, cmd.Execute())

	got := out.String()
	// Should show the preview with only Monday and Tuesday creates.
	require.Contains(t, got, "2 to create", "should create 2 entries for mon,tue")
}
