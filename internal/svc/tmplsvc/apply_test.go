package tmplsvc

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

// applyServer builds an httptest.Server that handles all endpoints needed by
// Apply: week report, locked days, time types, POST /time, and GET /time/{id}.
// reportJSON, lockedJSON, and typesJSON are served on their respective routes.
// postResponse is served for POST /TDWebApi/api/time.
// wireEntries maps entry ID → wire JSON for GET /TDWebApi/api/time/{id}.
func applyServer(t *testing.T, reportJSON, lockedJSON, typesJSON, postResponse string, wireEntries map[int]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/TDWebApi/api/time/report/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(reportJSON))

		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/time/locked":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(lockedJSON))

		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(typesJSON))

		case r.Method == http.MethodPost && r.URL.Path == "/TDWebApi/api/time":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(postResponse))

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/TDWebApi/api/time/"):
			// Extract entry ID from path.
			idStr := strings.TrimPrefix(r.URL.Path, "/TDWebApi/api/time/")
			var id int
			_, _ = fmt.Sscanf(idStr, "%d", &id)
			if body, ok := wireEntries[id]; ok {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(body))
			} else {
				http.NotFound(w, r)
			}

		default:
			t.Logf("applyServer: unhandled %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	}))
}

// monDayTemplate returns a Template with 1 row targeting project/54, type 5,
// Monday only, 2h.
func monDayTemplate() domain.Template {
	return domain.Template{
		SchemaVersion: 1,
		Name:          "mon-tmpl",
		Rows: []domain.TemplateRow{
			{
				ID:          "row-01",
				Target:      domain.Target{Kind: domain.TargetProject, ItemID: 54},
				TimeType:    domain.TimeType{ID: 5, Name: "Dev"},
				Hours:       domain.WeekHours{Mon: 2.0},
				Description: "work",
			},
		},
	}
}

// monTueDayTemplate returns a Template with 1 row targeting project/54, type 5,
// Mon and Tue, 2h each.
func monTueDayTemplate() domain.Template {
	return domain.Template{
		SchemaVersion: 1,
		Name:          "mon-tue-tmpl",
		Rows: []domain.TemplateRow{
			{
				ID:          "row-01",
				Target:      domain.Target{Kind: domain.TargetProject, ItemID: 54},
				TimeType:    domain.TimeType{ID: 5, Name: "Dev"},
				Hours:       domain.WeekHours{Mon: 2.0, Tue: 2.0},
				Description: "work",
			},
		},
	}
}

// wireEntry900 is a minimal wire JSON for entry ID 900.
const wireEntry900 = `{
	"TimeID":900,"ItemID":54,"ItemTitle":"Proj",
	"Uid":"test-uid","TimeTypeID":5,"TimeTypeName":"",
	"Billable":false,"AppID":0,"AppName":"None","Component":1,
	"TicketID":0,"ProjectID":54,"ProjectName":"Proj",
	"PlanID":0,"TimeDate":"2026-04-06T00:00:00Z",
	"Minutes":120.0,"Description":"work",
	"Status":0,"StatusDate":"0001-01-01T00:00:00",
	"PortfolioID":0,"Limited":false,"FunctionalRoleId":0,
	"CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00"
}`

// TestApply_CreatesEntries verifies that Apply executes a Create action and
// returns Created == 1 with no failures.
func TestApply_CreatesEntries(t *testing.T) {
	postResponse := `{"Succeeded":[{"Index":0,"ID":900}],"Failed":[]}`
	wireEntries := map[int]string{900: wireEntry900}

	srv := applyServer(t,
		emptyReport(0), "[]", typesJSON,
		postResponse, wireEntries,
	)
	defer srv.Close()

	paths, tsvc := tmplHarness(t, srv.URL)
	svc := New(paths, tsvc)

	tmpl := monDayTemplate()
	weekRef := testWeekRef() // Sun 2026-04-05 .. Sat 2026-04-11
	input := ReconcileInput{
		Template: tmpl,
		WeekRef:  weekRef,
		Mode:     domain.ModeAdd,
		UserUID:  "test-uid",
	}

	// Step 1: Reconcile to get the preview diff + hash.
	diff, err := svc.Reconcile(context.Background(), "default", input)
	require.NoError(t, err)
	require.NotEmpty(t, diff.DiffHash)
	creates, _, _ := diff.CountByKind()
	require.Equal(t, 1, creates, "expected 1 create action")

	// Step 2: Apply with the correct hash.
	result, err := svc.Apply(context.Background(), "default", input, diff.DiffHash)
	require.NoError(t, err)
	require.Equal(t, 1, result.Created)
	require.Equal(t, 0, result.Updated)
	require.Equal(t, 0, result.Skipped)
	require.Empty(t, result.Failed)
}

// TestApply_HashMismatch verifies that Apply returns an error when the
// expected hash does not match the current diff.
func TestApply_HashMismatch(t *testing.T) {
	srv := applyServer(t,
		emptyReport(0), "[]", typesJSON,
		`{"Succeeded":[],"Failed":[]}`, nil,
	)
	defer srv.Close()

	paths, tsvc := tmplHarness(t, srv.URL)
	svc := New(paths, tsvc)

	input := ReconcileInput{
		Template: monDayTemplate(),
		WeekRef:  testWeekRef(),
		Mode:     domain.ModeAdd,
		UserUID:  "test-uid",
	}

	// Pass a deliberately wrong hash.
	_, err := svc.Apply(context.Background(), "default", input, "wrong-hash-value")
	require.Error(t, err)
	require.True(t,
		strings.Contains(err.Error(), "hash mismatch") || strings.Contains(err.Error(), "changed since preview"),
		"error should mention hash mismatch, got: %v", err)
}

// TestApply_SkipsExisting verifies that Apply correctly counts skipped entries
// when one day already has a matching entry and one does not.
func TestApply_SkipsExisting(t *testing.T) {
	// Week report: Monday (2026-04-06) has a matching entry on project/54, type 5.
	reportWithMon := `{
		"ID": 1,
		"PeriodStartDate": "2026-04-05T00:00:00Z",
		"PeriodEndDate": "2026-04-11T00:00:00Z",
		"Status": 0,
		"TimeReportUid": "uid",
		"UserFullName": "User",
		"MinutesBillable": 0,
		"MinutesNonBillable": 0,
		"MinutesTotal": 120,
		"TimeEntriesCount": 1,
		"Times": [
			{"TimeID":99,"Component":1,"ProjectID":54,"ItemID":54,"ItemTitle":"Proj","ProjectName":"Proj","TimeTypeID":5,"TimeTypeName":"","TimeDate":"2026-04-06T00:00:00Z","Minutes":120,"Description":"existing","Billable":false,"Uid":"uid","Status":0,"StatusDate":"0001-01-01T00:00:00","CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00","AppID":0,"AppName":"","TicketID":0,"PlanID":0,"PortfolioID":0,"Limited":false,"FunctionalRoleId":0}
		]
	}`

	// Wire entry for the newly created Tuesday entry (ID 901).
	wireEntry901 := `{
		"TimeID":901,"ItemID":54,"ItemTitle":"Proj",
		"Uid":"test-uid","TimeTypeID":5,"TimeTypeName":"",
		"Billable":false,"AppID":0,"AppName":"None","Component":1,
		"TicketID":0,"ProjectID":54,"ProjectName":"Proj",
		"PlanID":0,"TimeDate":"2026-04-07T00:00:00Z",
		"Minutes":120.0,"Description":"work",
		"Status":0,"StatusDate":"0001-01-01T00:00:00",
		"PortfolioID":0,"Limited":false,"FunctionalRoleId":0,
		"CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00"
	}`

	postResponse := `{"Succeeded":[{"Index":0,"ID":901}],"Failed":[]}`
	wireEntries := map[int]string{901: wireEntry901}

	srv := applyServer(t, reportWithMon, "[]", typesJSON, postResponse, wireEntries)
	defer srv.Close()

	paths, tsvc := tmplHarness(t, srv.URL)
	svc := New(paths, tsvc)

	tmpl := monTueDayTemplate()
	weekRef := testWeekRef()
	input := ReconcileInput{
		Template: tmpl,
		WeekRef:  weekRef,
		Mode:     domain.ModeAdd,
		UserUID:  "test-uid",
	}

	// Reconcile to get the diff + hash.
	diff, err := svc.Reconcile(context.Background(), "default", input)
	require.NoError(t, err)
	creates, _, skips := diff.CountByKind()
	require.Equal(t, 1, creates, "expected 1 create (Tue)")
	require.Equal(t, 1, skips, "expected 1 skip (Mon)")

	// Apply.
	result, err := svc.Apply(context.Background(), "default", input, diff.DiffHash)
	require.NoError(t, err)
	require.Equal(t, 1, result.Created, "expected 1 created (Tue)")
	require.Equal(t, 1, result.Skipped, "expected 1 skipped (Mon)")
	require.Equal(t, 0, result.Updated)
	require.Empty(t, result.Failed)
}

// TestApply_CheckerMarksDescription verifies that Apply calls Checker.Mark on
// the description before creating an entry when a Checker is provided.
func TestApply_CheckerMarksDescription(t *testing.T) {
	postResponse := `{"Succeeded":[{"Index":0,"ID":900}],"Failed":[]}`
	wireEntries := map[int]string{900: wireEntry900}

	srv := applyServer(t,
		emptyReport(0), "[]", typesJSON,
		postResponse, wireEntries,
	)
	defer srv.Close()

	paths, tsvc := tmplHarness(t, srv.URL)
	svc := New(paths, tsvc)

	// Minimal OwnershipChecker that prepends a marker.
	checker := &stubChecker{marker: "[tdx:mon-tmpl:row-01]"}

	tmpl := monDayTemplate()
	input := ReconcileInput{
		Template: tmpl,
		WeekRef:  testWeekRef(),
		Mode:     domain.ModeAdd,
		UserUID:  "test-uid",
		Checker:  checker,
	}

	diff, err := svc.Reconcile(context.Background(), "default", input)
	require.NoError(t, err)

	result, err := svc.Apply(context.Background(), "default", input, diff.DiffHash)
	require.NoError(t, err)
	require.Equal(t, 1, result.Created)
	require.True(t, checker.markCalled, "Checker.Mark should have been called")
}

// stubChecker is a minimal OwnershipChecker for testing.
type stubChecker struct {
	marker     string
	markCalled bool
}

func (c *stubChecker) IsOwned(entry domain.TimeEntry, templateName, rowID string) bool {
	return strings.Contains(entry.Description, c.marker)
}

func (c *stubChecker) Mark(description, templateName, rowID string) string {
	c.markCalled = true
	return c.marker + " " + description
}

func (c *stubChecker) Unmark(description string) string {
	return strings.TrimPrefix(description, c.marker+" ")
}

// Verify stubChecker satisfies domain.OwnershipChecker at compile time.
var _ domain.OwnershipChecker = (*stubChecker)(nil)

// TestApply_CollectsFailures verifies that Apply collects failures from the
// server rather than returning early on first error.
func TestApply_CollectsFailures(t *testing.T) {
	// Server returns a failure for POST /time.
	postResponse := `{"Succeeded":[],"Failed":[{"Index":0,"TimeEntryID":0,"ErrorMessage":"Day is locked","ErrorCode":40,"ErrorCodeName":"DayLocked"}]}`

	srv := applyServer(t,
		emptyReport(0), "[]", typesJSON,
		postResponse, nil,
	)
	defer srv.Close()

	paths, tsvc := tmplHarness(t, srv.URL)
	svc := New(paths, tsvc)

	tmpl := monDayTemplate()
	input := ReconcileInput{
		Template: tmpl,
		WeekRef:  testWeekRef(),
		Mode:     domain.ModeAdd,
		UserUID:  "test-uid",
	}

	diff, err := svc.Reconcile(context.Background(), "default", input)
	require.NoError(t, err)

	result, err := svc.Apply(context.Background(), "default", input, diff.DiffHash)
	require.NoError(t, err, "Apply itself should not error; failures are collected in result")
	require.Equal(t, 0, result.Created)
	require.Len(t, result.Failed, 1)
	require.Contains(t, result.Failed[0].Message, "Day is locked")
	require.Equal(t, "row-01", result.Failed[0].RowID)
}

// TestApply_WeekdayForMonday sanity-checks which date is Monday in testWeekRef.
// testWeekRef start is Sun 2026-04-05; Mon is 2026-04-06.
func TestApply_WeekdayForMonday(t *testing.T) {
	wr := testWeekRef()
	mon := wr.StartDate.AddDate(0, 0, 1)
	require.Equal(t, time.Monday, mon.Weekday())
}
