package tmplsvc

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ipm/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

// reconcileServer builds an httptest.Server that serves the endpoints needed by
// Reconcile: GET /api/time/report/{date}, GET /api/time/locked, and GET
// /api/time/types. The caller supplies the week report JSON, locked-days JSON,
// and time-types JSON; the server routes accordingly.
func reconcileServer(t *testing.T, reportJSON, lockedJSON, typesJSON string) *httptest.Server {
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

		default:
			t.Logf("reconcileServer: unhandled %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	}))
}

// emptyReport returns a week report JSON for a Sun 2026-04-05 .. Sat 2026-04-11
// week with no time entries and the given status code.
func emptyReport(statusCode int) string {
	return fmt.Sprintf(`{
		"ID": 1,
		"PeriodStartDate": "2026-04-05T00:00:00Z",
		"PeriodEndDate": "2026-04-11T00:00:00Z",
		"Status": %d,
		"TimeReportUid": "uid",
		"UserFullName": "User",
		"MinutesBillable": 0,
		"MinutesNonBillable": 0,
		"MinutesTotal": 0,
		"TimeEntriesCount": 0,
		"Times": []
	}`, statusCode)
}

// typesJSON is a minimal time-types response with one active type (ID=5).
const typesJSON = `[{"ID":5,"Name":"Dev","IsActive":true}]`

// testTemplate returns a template with one row targeting project/54, type 5,
// Mon-Fri 2h each.
func testTemplate() domain.Template {
	return domain.Template{
		SchemaVersion: 1,
		Name:          "test-tmpl",
		Rows: []domain.TemplateRow{
			{
				ID:       "row-01",
				Target:   domain.Target{Kind: domain.TargetProject, ItemID: 54},
				TimeType: domain.TimeType{ID: 5, Name: "Dev"},
				Hours: domain.WeekHours{
					Mon: 2.0, Tue: 2.0, Wed: 2.0, Thu: 2.0, Fri: 2.0,
				},
				Description: "work",
			},
		},
	}
}

// weekRef returns a WeekRef for the Sun 2026-04-05 .. Sat 2026-04-11 week.
func testWeekRef() domain.WeekRef {
	return domain.WeekRef{
		StartDate: time.Date(2026, 4, 5, 0, 0, 0, 0, domain.EasternTZ),
		EndDate:   time.Date(2026, 4, 11, 0, 0, 0, 0, domain.EasternTZ),
	}
}

func TestReconcile_AddMode_AllCreate(t *testing.T) {
	srv := reconcileServer(t, emptyReport(0), "[]", typesJSON)
	defer srv.Close()

	paths, tsvc := tmplHarness(t, srv.URL)
	svc := New(paths, tsvc)

	diff, err := svc.Reconcile(context.Background(), "default", ReconcileInput{
		Template: testTemplate(),
		WeekRef:  testWeekRef(),
		Mode:     domain.ModeAdd,
		UserUID:  "test-uid",
	})
	require.NoError(t, err)

	// 5 Create actions for Mon-Fri, 0 blockers, 0 skips.
	creates, updates, skips := diff.CountByKind()
	require.Equal(t, 5, creates, "expected 5 creates")
	require.Equal(t, 0, updates, "expected 0 updates")
	require.Equal(t, 0, skips, "expected 0 skips")
	require.Empty(t, diff.Blockers)
	require.Len(t, diff.Actions, 5)

	// Actions should be sorted by (RowID, Date).
	for i := 1; i < len(diff.Actions); i++ {
		prev := diff.Actions[i-1]
		curr := diff.Actions[i]
		require.True(t, prev.RowID < curr.RowID ||
			(prev.RowID == curr.RowID && !prev.Date.After(curr.Date)),
			"actions should be sorted by (RowID, Date)")
	}

	// Each create action should have the correct EntryInput.
	for _, a := range diff.Actions {
		require.Equal(t, domain.ActionCreate, a.Kind)
		require.Equal(t, "row-01", a.RowID)
		require.Equal(t, "test-uid", a.Entry.UserUID)
		require.Equal(t, 120, a.Entry.Minutes) // 2h = 120m
		require.Equal(t, 5, a.Entry.TimeTypeID)
		require.Equal(t, domain.TargetProject, a.Entry.Target.Kind)
		require.Equal(t, 54, a.Entry.Target.ItemID)
		require.Equal(t, "work", a.Entry.Description)
	}

	// DiffHash must be non-empty.
	require.NotEmpty(t, diff.DiffHash)
}

func TestReconcile_AddMode_ExistingSkipped(t *testing.T) {
	// Week report has one matching entry on Monday (project/54, type 5).
	reportJSON := `{
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
	srv := reconcileServer(t, reportJSON, "[]", typesJSON)
	defer srv.Close()

	paths, tsvc := tmplHarness(t, srv.URL)
	svc := New(paths, tsvc)

	diff, err := svc.Reconcile(context.Background(), "default", ReconcileInput{
		Template: testTemplate(),
		WeekRef:  testWeekRef(),
		Mode:     domain.ModeAdd,
		UserUID:  "test-uid",
	})
	require.NoError(t, err)

	creates, _, skips := diff.CountByKind()
	require.Equal(t, 4, creates, "expected 4 creates (Tue-Fri)")
	require.Equal(t, 1, skips, "expected 1 skip (Mon)")
	require.Empty(t, diff.Blockers)

	// Find the skip action and verify reason.
	var skipAction domain.Action
	for _, a := range diff.Actions {
		if a.Kind == domain.ActionSkip {
			skipAction = a
			break
		}
	}
	require.Equal(t, "alreadyExists", skipAction.SkipReason)
	// Monday is 2026-04-06.
	require.Equal(t, time.Monday, skipAction.Date.Weekday())
}

func TestReconcile_LockedDayBlocker(t *testing.T) {
	// Monday 2026-04-06 is locked.
	lockedJSON := `["2026-04-06T00:00:00Z"]`
	srv := reconcileServer(t, emptyReport(0), lockedJSON, typesJSON)
	defer srv.Close()

	paths, tsvc := tmplHarness(t, srv.URL)
	svc := New(paths, tsvc)

	diff, err := svc.Reconcile(context.Background(), "default", ReconcileInput{
		Template: testTemplate(),
		WeekRef:  testWeekRef(),
		Mode:     domain.ModeAdd,
		UserUID:  "test-uid",
	})
	require.NoError(t, err)

	creates, _, _ := diff.CountByKind()
	require.Equal(t, 4, creates, "expected 4 creates (Tue-Fri)")
	require.Len(t, diff.Blockers, 1)
	require.Equal(t, domain.BlockerLocked, diff.Blockers[0].Kind)
	require.Equal(t, time.Monday, diff.Blockers[0].Date.Weekday())
	require.Equal(t, "row-01", diff.Blockers[0].RowID)
}

func TestReconcile_SubmittedWeekBlocker(t *testing.T) {
	// Status=1 means submitted.
	srv := reconcileServer(t, emptyReport(1), "[]", typesJSON)
	defer srv.Close()

	paths, tsvc := tmplHarness(t, srv.URL)
	svc := New(paths, tsvc)

	diff, err := svc.Reconcile(context.Background(), "default", ReconcileInput{
		Template: testTemplate(),
		WeekRef:  testWeekRef(),
		Mode:     domain.ModeAdd,
		UserUID:  "test-uid",
	})
	require.NoError(t, err)

	// All 5 days should be blockers (submitted), 0 actions.
	require.Empty(t, diff.Actions)
	require.Len(t, diff.Blockers, 5)
	for _, b := range diff.Blockers {
		require.Equal(t, domain.BlockerSubmitted, b.Kind)
	}
}

func TestReconcile_DaysFilter(t *testing.T) {
	srv := reconcileServer(t, emptyReport(0), "[]", typesJSON)
	defer srv.Close()

	paths, tsvc := tmplHarness(t, srv.URL)
	svc := New(paths, tsvc)

	diff, err := svc.Reconcile(context.Background(), "default", ReconcileInput{
		Template:   testTemplate(),
		WeekRef:    testWeekRef(),
		Mode:       domain.ModeAdd,
		DaysFilter: []time.Weekday{time.Monday, time.Tuesday, time.Wednesday},
		UserUID:    "test-uid",
	})
	require.NoError(t, err)

	creates, _, _ := diff.CountByKind()
	require.Equal(t, 3, creates, "expected 3 creates (Mon-Wed)")
	require.Empty(t, diff.Blockers)
	require.Len(t, diff.Actions, 3)

	// Verify only Mon, Tue, Wed are present.
	days := make(map[time.Weekday]bool)
	for _, a := range diff.Actions {
		days[a.Date.Weekday()] = true
	}
	require.True(t, days[time.Monday])
	require.True(t, days[time.Tuesday])
	require.True(t, days[time.Wednesday])
	require.False(t, days[time.Thursday])
	require.False(t, days[time.Friday])
}

func TestReconcile_ReplaceMatchingMode_UpdatesExisting(t *testing.T) {
	// Week has a matching entry on Monday (project/54, type 5) with 60 minutes
	// instead of the template's 120 minutes. Template has Mon-Tue 2h.
	reportJSON := `{
		"ID": 1,
		"PeriodStartDate": "2026-04-05T00:00:00Z",
		"PeriodEndDate": "2026-04-11T00:00:00Z",
		"Status": 0,
		"TimeReportUid": "uid",
		"UserFullName": "User",
		"MinutesBillable": 0,
		"MinutesNonBillable": 0,
		"MinutesTotal": 60,
		"TimeEntriesCount": 1,
		"Times": [
			{"TimeID":99,"Component":1,"ProjectID":54,"ItemID":54,"ItemTitle":"Proj","ProjectName":"Proj","TimeTypeID":5,"TimeTypeName":"","TimeDate":"2026-04-06T00:00:00Z","Minutes":60,"Description":"work","Billable":false,"Uid":"uid","Status":0,"StatusDate":"0001-01-01T00:00:00","CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00","AppID":0,"AppName":"","TicketID":0,"PlanID":0,"PortfolioID":0,"Limited":false,"FunctionalRoleId":0}
		]
	}`
	srv := reconcileServer(t, reportJSON, "[]", typesJSON)
	defer srv.Close()

	// Template with only Mon-Tue 2h.
	tmpl := domain.Template{
		SchemaVersion: 1,
		Name:          "test-tmpl",
		Rows: []domain.TemplateRow{
			{
				ID:       "row-01",
				Target:   domain.Target{Kind: domain.TargetProject, ItemID: 54},
				TimeType: domain.TimeType{ID: 5, Name: "Dev"},
				Hours: domain.WeekHours{
					Mon: 2.0, Tue: 2.0,
				},
				Description: "work",
			},
		},
	}

	paths, tsvc := tmplHarness(t, srv.URL)
	svc := New(paths, tsvc)

	diff, err := svc.Reconcile(context.Background(), "default", ReconcileInput{
		Template: tmpl,
		WeekRef:  testWeekRef(),
		Mode:     domain.ModeReplaceMatching,
		UserUID:  "test-uid",
	})
	require.NoError(t, err)

	creates, updates, skips := diff.CountByKind()
	require.Equal(t, 1, creates, "expected 1 create (Tue)")
	require.Equal(t, 1, updates, "expected 1 update (Mon)")
	require.Equal(t, 0, skips, "expected 0 skips")
	require.Empty(t, diff.Blockers)

	// Find the update action and verify it targets Monday with a minutes patch.
	var updateAction domain.Action
	for _, a := range diff.Actions {
		if a.Kind == domain.ActionUpdate {
			updateAction = a
			break
		}
	}
	require.Equal(t, time.Monday, updateAction.Date.Weekday())
	require.Equal(t, 99, updateAction.ExistingID)
	require.NotNil(t, updateAction.Patch.Minutes, "expected Patch.Minutes to be set")
	require.Equal(t, 120, *updateAction.Patch.Minutes)

	// Find the create action and verify it targets Tuesday.
	var createAction domain.Action
	for _, a := range diff.Actions {
		if a.Kind == domain.ActionCreate {
			createAction = a
			break
		}
	}
	require.Equal(t, time.Tuesday, createAction.Date.Weekday())
}

func TestReconcile_ReplaceMatchingMode_AlreadyMatches(t *testing.T) {
	// Week has a matching entry on Monday with SAME values (120 minutes, same
	// description). Mode: ModeReplaceMatching. Expect ActionSkip("alreadyMatches").
	reportJSON := `{
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
			{"TimeID":99,"Component":1,"ProjectID":54,"ItemID":54,"ItemTitle":"Proj","ProjectName":"Proj","TimeTypeID":5,"TimeTypeName":"","TimeDate":"2026-04-06T00:00:00Z","Minutes":120,"Description":"work","Billable":false,"Uid":"uid","Status":0,"StatusDate":"0001-01-01T00:00:00","CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00","AppID":0,"AppName":"","TicketID":0,"PlanID":0,"PortfolioID":0,"Limited":false,"FunctionalRoleId":0}
		]
	}`
	srv := reconcileServer(t, reportJSON, "[]", typesJSON)
	defer srv.Close()

	// Template with only Mon 2h.
	tmpl := domain.Template{
		SchemaVersion: 1,
		Name:          "test-tmpl",
		Rows: []domain.TemplateRow{
			{
				ID:       "row-01",
				Target:   domain.Target{Kind: domain.TargetProject, ItemID: 54},
				TimeType: domain.TimeType{ID: 5, Name: "Dev"},
				Hours: domain.WeekHours{
					Mon: 2.0,
				},
				Description: "work",
			},
		},
	}

	paths, tsvc := tmplHarness(t, srv.URL)
	svc := New(paths, tsvc)

	diff, err := svc.Reconcile(context.Background(), "default", ReconcileInput{
		Template: tmpl,
		WeekRef:  testWeekRef(),
		Mode:     domain.ModeReplaceMatching,
		UserUID:  "test-uid",
	})
	require.NoError(t, err)

	creates, updates, skips := diff.CountByKind()
	require.Equal(t, 0, creates, "expected 0 creates")
	require.Equal(t, 0, updates, "expected 0 updates")
	require.Equal(t, 1, skips, "expected 1 skip")
	require.Empty(t, diff.Blockers)
	require.Len(t, diff.Actions, 1)

	skipAction := diff.Actions[0]
	require.Equal(t, domain.ActionSkip, skipAction.Kind)
	require.Equal(t, "alreadyMatches", skipAction.SkipReason)
	require.Equal(t, time.Monday, skipAction.Date.Weekday())
}

func TestReconcile_ReplaceMatchingMode_CreatesNew(t *testing.T) {
	// Empty week. Template Mon-Fri 2h. Mode: ModeReplaceMatching.
	// Expect 5 ActionCreate — same as add mode when nothing exists.
	srv := reconcileServer(t, emptyReport(0), "[]", typesJSON)
	defer srv.Close()

	paths, tsvc := tmplHarness(t, srv.URL)
	svc := New(paths, tsvc)

	diff, err := svc.Reconcile(context.Background(), "default", ReconcileInput{
		Template: testTemplate(),
		WeekRef:  testWeekRef(),
		Mode:     domain.ModeReplaceMatching,
		UserUID:  "test-uid",
	})
	require.NoError(t, err)

	creates, updates, skips := diff.CountByKind()
	require.Equal(t, 5, creates, "expected 5 creates (Mon-Fri)")
	require.Equal(t, 0, updates, "expected 0 updates")
	require.Equal(t, 0, skips, "expected 0 skips")
	require.Empty(t, diff.Blockers)
	require.Len(t, diff.Actions, 5)

	for _, a := range diff.Actions {
		require.Equal(t, domain.ActionCreate, a.Kind)
		require.Equal(t, 120, a.Entry.Minutes)
		require.Equal(t, 5, a.Entry.TimeTypeID)
		require.Equal(t, domain.TargetProject, a.Entry.Target.Kind)
		require.Equal(t, 54, a.Entry.Target.ItemID)
	}
}

func TestReconcile_ReplaceMineMode_OwnedUpdated(t *testing.T) {
	// Week has a matching entry on Monday whose description contains the
	// ownership marker for test-tmpl/row-01, but with 60 minutes instead of
	// the template's 120. Checker = &MarkerChecker{}. Mode = ModeReplaceMine.
	// Expect: 1 ActionUpdate (owned entry gets updated), 4 ActionCreate (Tue-Fri).
	reportJSON := `{
		"ID": 1,
		"PeriodStartDate": "2026-04-05T00:00:00Z",
		"PeriodEndDate": "2026-04-11T00:00:00Z",
		"Status": 0,
		"TimeReportUid": "uid",
		"UserFullName": "User",
		"MinutesBillable": 0,
		"MinutesNonBillable": 0,
		"MinutesTotal": 60,
		"TimeEntriesCount": 1,
		"Times": [
			{"TimeID":99,"Component":1,"ProjectID":54,"ItemID":54,"ItemTitle":"Proj","ProjectName":"Proj","TimeTypeID":5,"TimeTypeName":"","TimeDate":"2026-04-06T00:00:00Z","Minutes":60,"Description":"work [tdx:test-tmpl#row-01]","Billable":false,"Uid":"uid","Status":0,"StatusDate":"0001-01-01T00:00:00","CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00","AppID":0,"AppName":"","TicketID":0,"PlanID":0,"PortfolioID":0,"Limited":false,"FunctionalRoleId":0}
		]
	}`
	srv := reconcileServer(t, reportJSON, "[]", typesJSON)
	defer srv.Close()

	paths, tsvc := tmplHarness(t, srv.URL)
	svc := New(paths, tsvc)

	diff, err := svc.Reconcile(context.Background(), "default", ReconcileInput{
		Template: testTemplate(),
		WeekRef:  testWeekRef(),
		Mode:     domain.ModeReplaceMine,
		Checker:  &MarkerChecker{},
		UserUID:  "test-uid",
	})
	require.NoError(t, err)

	creates, updates, skips := diff.CountByKind()
	require.Equal(t, 4, creates, "expected 4 creates (Tue-Fri)")
	require.Equal(t, 1, updates, "expected 1 update (owned Mon entry)")
	require.Equal(t, 0, skips, "expected 0 skips")
	require.Empty(t, diff.Blockers)

	var updateAction domain.Action
	for _, a := range diff.Actions {
		if a.Kind == domain.ActionUpdate {
			updateAction = a
			break
		}
	}
	require.Equal(t, time.Monday, updateAction.Date.Weekday())
	require.Equal(t, 99, updateAction.ExistingID)
	require.NotNil(t, updateAction.Patch.Minutes, "expected Patch.Minutes to be set")
	require.Equal(t, 120, *updateAction.Patch.Minutes)
}

func TestReconcile_ReplaceMineMode_NotOwnedSkipped(t *testing.T) {
	// Week has a matching entry on Monday whose description is plain text with
	// no ownership marker. Checker = &MarkerChecker{}. Mode = ModeReplaceMine.
	// Expect: 1 ActionSkip("notOwnedByTemplate") on Mon, 4 ActionCreate (Tue-Fri).
	reportJSON := `{
		"ID": 1,
		"PeriodStartDate": "2026-04-05T00:00:00Z",
		"PeriodEndDate": "2026-04-11T00:00:00Z",
		"Status": 0,
		"TimeReportUid": "uid",
		"UserFullName": "User",
		"MinutesBillable": 0,
		"MinutesNonBillable": 0,
		"MinutesTotal": 60,
		"TimeEntriesCount": 1,
		"Times": [
			{"TimeID":99,"Component":1,"ProjectID":54,"ItemID":54,"ItemTitle":"Proj","ProjectName":"Proj","TimeTypeID":5,"TimeTypeName":"","TimeDate":"2026-04-06T00:00:00Z","Minutes":60,"Description":"someone else's work","Billable":false,"Uid":"uid","Status":0,"StatusDate":"0001-01-01T00:00:00","CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00","AppID":0,"AppName":"","TicketID":0,"PlanID":0,"PortfolioID":0,"Limited":false,"FunctionalRoleId":0}
		]
	}`
	srv := reconcileServer(t, reportJSON, "[]", typesJSON)
	defer srv.Close()

	paths, tsvc := tmplHarness(t, srv.URL)
	svc := New(paths, tsvc)

	diff, err := svc.Reconcile(context.Background(), "default", ReconcileInput{
		Template: testTemplate(),
		WeekRef:  testWeekRef(),
		Mode:     domain.ModeReplaceMine,
		Checker:  &MarkerChecker{},
		UserUID:  "test-uid",
	})
	require.NoError(t, err)

	creates, updates, skips := diff.CountByKind()
	require.Equal(t, 4, creates, "expected 4 creates (Tue-Fri)")
	require.Equal(t, 0, updates, "expected 0 updates")
	require.Equal(t, 1, skips, "expected 1 skip (not owned)")
	require.Empty(t, diff.Blockers)

	var skipAction domain.Action
	for _, a := range diff.Actions {
		if a.Kind == domain.ActionSkip {
			skipAction = a
			break
		}
	}
	require.Equal(t, "notOwnedByTemplate", skipAction.SkipReason)
	require.Equal(t, time.Monday, skipAction.Date.Weekday())
}

func TestReconcile_ReplaceMineMode_NilChecker(t *testing.T) {
	// Mode = ModeReplaceMine with Checker = nil and a matching entry present.
	// Expect: error containing "requires an ownership checker".
	reportJSON := `{
		"ID": 1,
		"PeriodStartDate": "2026-04-05T00:00:00Z",
		"PeriodEndDate": "2026-04-11T00:00:00Z",
		"Status": 0,
		"TimeReportUid": "uid",
		"UserFullName": "User",
		"MinutesBillable": 0,
		"MinutesNonBillable": 0,
		"MinutesTotal": 60,
		"TimeEntriesCount": 1,
		"Times": [
			{"TimeID":99,"Component":1,"ProjectID":54,"ItemID":54,"ItemTitle":"Proj","ProjectName":"Proj","TimeTypeID":5,"TimeTypeName":"","TimeDate":"2026-04-06T00:00:00Z","Minutes":60,"Description":"work","Billable":false,"Uid":"uid","Status":0,"StatusDate":"0001-01-01T00:00:00","CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00","AppID":0,"AppName":"","TicketID":0,"PlanID":0,"PortfolioID":0,"Limited":false,"FunctionalRoleId":0}
		]
	}`
	srv := reconcileServer(t, reportJSON, "[]", typesJSON)
	defer srv.Close()

	paths, tsvc := tmplHarness(t, srv.URL)
	svc := New(paths, tsvc)

	_, err := svc.Reconcile(context.Background(), "default", ReconcileInput{
		Template: testTemplate(),
		WeekRef:  testWeekRef(),
		Mode:     domain.ModeReplaceMine,
		Checker:  nil,
		UserUID:  "test-uid",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires an ownership checker")
}

func TestReconcile_DiffHashStable(t *testing.T) {
	srv := reconcileServer(t, emptyReport(0), "[]", typesJSON)
	defer srv.Close()

	paths, tsvc := tmplHarness(t, srv.URL)
	svc := New(paths, tsvc)

	input := ReconcileInput{
		Template: testTemplate(),
		WeekRef:  testWeekRef(),
		Mode:     domain.ModeAdd,
		UserUID:  "test-uid",
	}

	diff1, err := svc.Reconcile(context.Background(), "default", input)
	require.NoError(t, err)

	diff2, err := svc.Reconcile(context.Background(), "default", input)
	require.NoError(t, err)

	require.Equal(t, diff1.DiffHash, diff2.DiffHash, "DiffHash should be deterministic")
	require.NotEmpty(t, diff1.DiffHash)
}

func TestReconcile_ZeroHourRowSkipped(t *testing.T) {
	// Template row with all zero hours should produce no actions.
	srv := reconcileServer(t, emptyReport(0), "[]", typesJSON)
	defer srv.Close()

	tmpl := domain.Template{
		SchemaVersion: 1,
		Name:          "zero-hours",
		Rows: []domain.TemplateRow{
			{
				ID:       "row-zero",
				Target:   domain.Target{Kind: domain.TargetProject, ItemID: 54},
				TimeType: domain.TimeType{ID: 5, Name: "Dev"},
				Hours:    domain.WeekHours{}, // all zeros
			},
		},
	}

	paths, tsvc := tmplHarness(t, srv.URL)
	svc := New(paths, tsvc)

	diff, err := svc.Reconcile(context.Background(), "default", ReconcileInput{
		Template: tmpl,
		WeekRef:  testWeekRef(),
		Mode:     domain.ModeAdd,
		UserUID:  "test-uid",
	})
	require.NoError(t, err)
	require.Empty(t, diff.Actions, "zero-hour row should produce no actions")
	require.Empty(t, diff.Blockers)
}

func TestReconcile_OverrideToZeroSkips(t *testing.T) {
	// Template has Mon=2h, but override sets Mon=0 → Mon is skipped.
	srv := reconcileServer(t, emptyReport(0), "[]", typesJSON)
	defer srv.Close()

	// Template with only Mon 2h.
	tmpl := domain.Template{
		SchemaVersion: 1,
		Name:          "override-zero",
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

	paths, tsvc := tmplHarness(t, srv.URL)
	svc := New(paths, tsvc)

	diff, err := svc.Reconcile(context.Background(), "default", ReconcileInput{
		Template: tmpl,
		WeekRef:  testWeekRef(),
		Mode:     domain.ModeAdd,
		UserUID:  "test-uid",
		Overrides: []Override{
			{RowID: "row-01", Day: time.Monday, Hours: 0},
		},
	})
	require.NoError(t, err)
	require.Empty(t, diff.Actions, "override to zero should produce no actions")
	require.Empty(t, diff.Blockers)
}

func TestReconcile_RoundingAllowed(t *testing.T) {
	// Template with 1.333h (non-integer minutes) and Round=true → rounds to 80 min.
	srv := reconcileServer(t, emptyReport(0), "[]", typesJSON)
	defer srv.Close()

	tmpl := domain.Template{
		SchemaVersion: 1,
		Name:          "rounding",
		Rows: []domain.TemplateRow{
			{
				ID:          "row-round",
				Target:      domain.Target{Kind: domain.TargetProject, ItemID: 54},
				TimeType:    domain.TimeType{ID: 5, Name: "Dev"},
				Hours:       domain.WeekHours{Mon: 1.333},
				Description: "fractional",
			},
		},
	}

	paths, tsvc := tmplHarness(t, srv.URL)
	svc := New(paths, tsvc)

	diff, err := svc.Reconcile(context.Background(), "default", ReconcileInput{
		Template: tmpl,
		WeekRef:  testWeekRef(),
		Mode:     domain.ModeAdd,
		UserUID:  "test-uid",
		Round:    true,
	})
	require.NoError(t, err)
	require.Len(t, diff.Actions, 1)
	// 1.333 * 60 = 79.98, math.Round → 80
	require.Equal(t, 80, diff.Actions[0].Entry.Minutes)
}

func TestReconcile_RoundingRejected(t *testing.T) {
	// Same template with Round=false → error.
	srv := reconcileServer(t, emptyReport(0), "[]", typesJSON)
	defer srv.Close()

	tmpl := domain.Template{
		SchemaVersion: 1,
		Name:          "rounding",
		Rows: []domain.TemplateRow{
			{
				ID:          "row-round",
				Target:      domain.Target{Kind: domain.TargetProject, ItemID: 54},
				TimeType:    domain.TimeType{ID: 5, Name: "Dev"},
				Hours:       domain.WeekHours{Mon: 1.333},
				Description: "fractional",
			},
		},
	}

	paths, tsvc := tmplHarness(t, srv.URL)
	svc := New(paths, tsvc)

	_, err := svc.Reconcile(context.Background(), "default", ReconcileInput{
		Template: tmpl,
		WeekRef:  testWeekRef(),
		Mode:     domain.ModeAdd,
		UserUID:  "test-uid",
		Round:    false,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "non-integer minutes")
	require.Contains(t, err.Error(), "--round")
}
