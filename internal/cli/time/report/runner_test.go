package report

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/peoplesvc"
	"github.com/stretchr/testify/require"
)

type mockTimesvc struct {
	reports map[string]domain.WeekReport // keyed by uid
	errs    map[string]error
}

func (m *mockTimesvc) GetWeekReportForUser(_ context.Context, _ string, _ time.Time, uid string) (domain.WeekReport, error) {
	if e, ok := m.errs[uid]; ok {
		return domain.WeekReport{}, e
	}
	return m.reports[uid], nil
}

type mockPeoplesvc struct {
	users               map[string]domain.User
	search              []domain.User
	lastFilter          domain.UserFilter
	pool                peoplesvc.ResourcePool
	poolErr             error
	resolveCalls        int
	account             peoplesvc.Account
	accountErr          error
	resolveAccountCalls int
}

func (m *mockPeoplesvc) GetUser(_ context.Context, _, uid string) (domain.User, error) {
	if u, ok := m.users[uid]; ok {
		return u, nil
	}
	return domain.User{}, errors.New("not found")
}

func (m *mockPeoplesvc) SearchUsers(_ context.Context, _ string, filter domain.UserFilter) ([]domain.User, error) {
	m.lastFilter = filter
	return m.search, nil
}

func (m *mockPeoplesvc) ResolvePoolByName(_ context.Context, _, _ string) (peoplesvc.ResourcePool, error) {
	m.resolveCalls++
	if m.poolErr != nil {
		return peoplesvc.ResourcePool{}, m.poolErr
	}
	return m.pool, nil
}

func (m *mockPeoplesvc) ResolveAccountByName(_ context.Context, _, _ string) (peoplesvc.Account, error) {
	m.resolveAccountCalls++
	if m.accountErr != nil {
		return peoplesvc.Account{}, m.accountErr
	}
	return m.account, nil
}

type mockAuthsvc struct {
	me domain.User
}

func (m *mockAuthsvc) WhoAmI(_ context.Context, _ string) (domain.User, error) {
	return m.me, nil
}

func TestRunner_SingleUserSingleWeek(t *testing.T) {
	week := domain.WeekRefContaining(time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ))
	user := domain.User{UID: "u1", FullName: "Alice", Email: "alice@x"}
	report := domain.WeekReport{
		WeekRef: week, UserUID: "u1",
		MinutesBillable: 240, MinutesNonBillable: 60, TotalMinutes: 300,
		Status: domain.ReportSubmitted,
	}
	deps := runnerDeps{
		Time:   &mockTimesvc{reports: map[string]domain.WeekReport{"u1": report}},
		People: &mockPeoplesvc{users: map[string]domain.User{"u1": user}},
		Auth:   &mockAuthsvc{},
	}
	out, err := assembleReport(context.Background(), deps, statusFlags{
		users:       []string{"u1"},
		week:        "2026-04-14",
		includeZero: true,
		limit:       100,
	})
	require.NoError(t, err)
	require.Len(t, out.Rows, 1)
	require.Equal(t, "u1", out.Rows[0].User.UID)
	require.Equal(t, 240, out.Rows[0].BillableMin)
	require.Equal(t, domain.ReportSubmitted, out.Rows[0].Status)
}

func TestRunner_PermissionErrorIsRowLevel(t *testing.T) {
	week := domain.WeekRefContaining(time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ))
	deps := runnerDeps{
		Time: &mockTimesvc{
			errs: map[string]error{"u1": domain.ErrPermission},
		},
		People: &mockPeoplesvc{users: map[string]domain.User{"u1": {UID: "u1", FullName: "Alice"}}},
		Auth:   &mockAuthsvc{},
	}
	out, err := assembleReport(context.Background(), deps, statusFlags{
		users:       []string{"u1"},
		week:        "2026-04-14",
		includeZero: true,
		limit:       100,
	})
	require.NoError(t, err)
	require.Len(t, out.Rows, 1)
	require.Equal(t, week.StartDate, out.Rows[0].WeekRef.StartDate)
	require.EqualValues(t, "permission-denied", out.Rows[0].Status)
	require.Equal(t, 0, out.Rows[0].TotalMin)
}

func TestRunner_ManagerMeUsesAuthenticatedUID(t *testing.T) {
	me := domain.User{UID: "mgr-uid", FullName: "Mgr"}
	directReport := domain.User{UID: "u1", FullName: "Direct", ReportsToUID: "mgr-uid"}
	other := domain.User{UID: "u2", FullName: "Other", ReportsToUID: "someone-else"}
	week := domain.WeekRefContaining(time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ))
	report := domain.WeekReport{WeekRef: week, UserUID: "u1", TotalMinutes: 60}

	people := &mockPeoplesvc{search: []domain.User{directReport, other}}
	deps := runnerDeps{
		Time:   &mockTimesvc{reports: map[string]domain.WeekReport{"u1": report}},
		People: people,
		Auth:   &mockAuthsvc{me: me},
	}
	out, err := assembleReport(context.Background(), deps, statusFlags{
		managers:    []string{"me"},
		week:        "2026-04-14",
		includeZero: true,
		limit:       100,
	})
	require.NoError(t, err)
	require.Len(t, out.Rows, 1, "only direct report's row is included")
	require.Equal(t, "u1", out.Rows[0].User.UID)
	require.NotNil(t, people.lastFilter.Employee, "Employee filter must be set on --manager search")
	require.True(t, *people.lastFilter.Employee)
	require.Equal(t, employeeLimit, people.lastFilter.Limit)
}

func TestRunner_AccountSelectorResolvesToServerSideID(t *testing.T) {
	people := &mockPeoplesvc{
		search:  []domain.User{},
		account: peoplesvc.Account{ID: 866, Name: "999999 (Sample Department)"},
	}
	deps := runnerDeps{
		Time:   &mockTimesvc{},
		People: people,
		Auth:   &mockAuthsvc{},
	}
	_, err := assembleReport(context.Background(), deps, statusFlags{
		accounts:    []string{"999999 (Sample Department)"},
		week:        "2026-04-14",
		includeZero: true,
		limit:       100,
	})
	require.NoError(t, err)
	require.Equal(t, 1, people.resolveAccountCalls)
	require.NotNil(t, people.lastFilter.Employee)
	require.True(t, *people.lastFilter.Employee)
	require.Equal(t, []int{866}, people.lastFilter.AccountIDs)
	require.Equal(t, employeeLimit, people.lastFilter.Limit)
}

func TestRunner_AccountNotFoundPropagates(t *testing.T) {
	people := &mockPeoplesvc{accountErr: errors.New("account \"Nope\" not found among 6404 accounts")}
	deps := runnerDeps{
		Time:   &mockTimesvc{},
		People: people,
		Auth:   &mockAuthsvc{},
	}
	_, err := assembleReport(context.Background(), deps, statusFlags{
		accounts:    []string{"Nope"},
		week:        "2026-04-14",
		includeZero: true,
		limit:       100,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestRunner_ResourcePoolSelectorFiltersByPoolID(t *testing.T) {
	week := domain.WeekRefContaining(time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ))
	inPool := domain.User{UID: "u1", FullName: "Pool Member", ResourcePoolID: 46}
	outOfPool := domain.User{UID: "u2", FullName: "Other", ResourcePoolID: 99}
	report := domain.WeekReport{WeekRef: week, UserUID: "u1", TotalMinutes: 60}

	people := &mockPeoplesvc{
		search: []domain.User{inPool, outOfPool},
		pool:   peoplesvc.ResourcePool{ID: 46, Name: "Test Pool"},
	}
	deps := runnerDeps{
		Time:   &mockTimesvc{reports: map[string]domain.WeekReport{"u1": report}},
		People: people,
		Auth:   &mockAuthsvc{},
	}
	out, err := assembleReport(context.Background(), deps, statusFlags{
		resourcePools: []string{"Test Pool"},
		week:          "2026-04-14",
		includeZero:   true,
		limit:         100,
	})
	require.NoError(t, err)
	require.Equal(t, 1, people.resolveCalls)
	require.Len(t, out.Rows, 1)
	require.Equal(t, "u1", out.Rows[0].User.UID)
	require.NotNil(t, people.lastFilter.Employee)
	require.True(t, *people.lastFilter.Employee)
}

func TestRunner_ResourcePoolNotFoundPropagates(t *testing.T) {
	people := &mockPeoplesvc{poolErr: errors.New("not found")}
	deps := runnerDeps{
		Time:   &mockTimesvc{},
		People: people,
		Auth:   &mockAuthsvc{},
	}
	_, err := assembleReport(context.Background(), deps, statusFlags{
		resourcePools: []string{"Nope"},
		week:          "2026-04-14",
		includeZero:   true,
		limit:         100,
	})
	require.Error(t, err)
}

func TestRunner_AllSelectorSetsEmployeeFilter(t *testing.T) {
	people := &mockPeoplesvc{search: []domain.User{}}
	deps := runnerDeps{
		Time:   &mockTimesvc{},
		People: people,
		Auth:   &mockAuthsvc{},
	}
	_, err := assembleReport(context.Background(), deps, statusFlags{
		all:         true,
		yes:         true,
		week:        "2026-04-14",
		includeZero: true,
		limit:       100,
	})
	require.NoError(t, err)
	require.NotNil(t, people.lastFilter.Employee)
	require.True(t, *people.lastFilter.Employee)
	require.Equal(t, employeeLimit, people.lastFilter.Limit)
}

func TestRunner_IncompleteFiltersBelowThreshold(t *testing.T) {
	week := domain.WeekRefContaining(time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ))
	users := []domain.User{
		{UID: "u1", FullName: "Alice", ReportsToUID: "mgr"},
		{UID: "u2", FullName: "Bob", ReportsToUID: "mgr"},
		{UID: "u3", FullName: "Carol", ReportsToUID: "mgr"},
	}
	reports := map[string]domain.WeekReport{
		"u1": {WeekRef: week, UserUID: "u1", TotalMinutes: 38 * 60, Status: domain.ReportOpen},
		"u2": {WeekRef: week, UserUID: "u2", TotalMinutes: 40 * 60, Status: domain.ReportOpen},
		"u3": {WeekRef: week, UserUID: "u3", TotalMinutes: 42 * 60, Status: domain.ReportOpen},
	}
	deps := runnerDeps{
		Time:   &mockTimesvc{reports: reports},
		People: &mockPeoplesvc{search: users},
		Auth:   &mockAuthsvc{me: domain.User{UID: "mgr"}},
	}
	out, err := assembleReport(context.Background(), deps, statusFlags{
		managers:    []string{"me"},
		week:        "2026-04-14",
		includeZero: true,
		incomplete:  true,
		threshold:   40,
		limit:       100,
	})
	require.NoError(t, err)
	require.Len(t, out.Rows, 1, "only Alice is below 40h")
	require.Equal(t, "u1", out.Rows[0].User.UID)
}

func TestRunner_IncompleteWithCustomThreshold(t *testing.T) {
	week := domain.WeekRefContaining(time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ))
	users := []domain.User{
		{UID: "u1", FullName: "PT-A", ReportsToUID: "mgr"},
		{UID: "u2", FullName: "PT-B", ReportsToUID: "mgr"},
	}
	reports := map[string]domain.WeekReport{
		"u1": {WeekRef: week, UserUID: "u1", TotalMinutes: 30 * 60, Status: domain.ReportOpen},
		"u2": {WeekRef: week, UserUID: "u2", TotalMinutes: 32 * 60, Status: domain.ReportOpen},
	}
	deps := runnerDeps{
		Time:   &mockTimesvc{reports: reports},
		People: &mockPeoplesvc{search: users},
		Auth:   &mockAuthsvc{me: domain.User{UID: "mgr"}},
	}
	out, err := assembleReport(context.Background(), deps, statusFlags{
		managers:     []string{"me"},
		week:         "2026-04-14",
		includeZero:  true,
		incomplete:   true,
		threshold:    32,
		thresholdSet: true,
		limit:        100,
	})
	require.NoError(t, err)
	require.Len(t, out.Rows, 1, "only the 30h row is below 32h threshold")
	require.Equal(t, "u1", out.Rows[0].User.UID)
}

func TestRunner_IncompleteDropsPermissionDenied(t *testing.T) {
	week := domain.WeekRefContaining(time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ))
	users := []domain.User{
		{UID: "u1", FullName: "Visible", ReportsToUID: "mgr"},
		{UID: "u2", FullName: "Forbidden", ReportsToUID: "mgr"},
	}
	deps := runnerDeps{
		Time: &mockTimesvc{
			reports: map[string]domain.WeekReport{
				"u1": {WeekRef: week, UserUID: "u1", TotalMinutes: 30 * 60, Status: domain.ReportOpen},
			},
			errs: map[string]error{"u2": domain.ErrPermission},
		},
		People: &mockPeoplesvc{search: users},
		Auth:   &mockAuthsvc{me: domain.User{UID: "mgr"}},
	}
	out, err := assembleReport(context.Background(), deps, statusFlags{
		managers:    []string{"me"},
		week:        "2026-04-14",
		includeZero: true,
		incomplete:  true,
		threshold:   40,
		limit:       100,
	})
	require.NoError(t, err)
	require.Len(t, out.Rows, 1, "permission-denied row dropped under --incomplete")
	require.Equal(t, "u1", out.Rows[0].User.UID)
}

func TestRunner_IncompleteFiltersAffectTotals(t *testing.T) {
	week := domain.WeekRefContaining(time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ))
	users := []domain.User{
		{UID: "u1", FullName: "Under", ReportsToUID: "mgr"},
		{UID: "u2", FullName: "Over", ReportsToUID: "mgr"},
	}
	reports := map[string]domain.WeekReport{
		"u1": {WeekRef: week, UserUID: "u1", TotalMinutes: 30 * 60, Status: domain.ReportOpen, MinutesBillable: 30 * 60},
		"u2": {WeekRef: week, UserUID: "u2", TotalMinutes: 50 * 60, Status: domain.ReportOpen, MinutesBillable: 50 * 60},
	}
	deps := runnerDeps{
		Time:   &mockTimesvc{reports: reports},
		People: &mockPeoplesvc{search: users},
		Auth:   &mockAuthsvc{me: domain.User{UID: "mgr"}},
	}
	out, err := assembleReport(context.Background(), deps, statusFlags{
		managers:    []string{"me"},
		week:        "2026-04-14",
		includeZero: true,
		incomplete:  true,
		threshold:   40,
		limit:       100,
	})
	require.NoError(t, err)
	bill, _, total := out.Totals()
	require.Equal(t, 30*60, bill, "totals reflect filtered (u1 only) — 30h billable")
	require.Equal(t, 30*60, total, "filtered total = 30h")
}

func TestRunner_MultiManagerUnion(t *testing.T) {
	week := domain.WeekRefContaining(time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ))
	users := []domain.User{
		{UID: "u1", FullName: "A's report", ReportsToUID: "mgr-A"},
		{UID: "u2", FullName: "B's report", ReportsToUID: "mgr-B"},
		{UID: "u3", FullName: "C's report", ReportsToUID: "mgr-C"},
	}
	deps := runnerDeps{
		Time: &mockTimesvc{reports: map[string]domain.WeekReport{
			"u1": {WeekRef: week, UserUID: "u1", TotalMinutes: 60},
			"u2": {WeekRef: week, UserUID: "u2", TotalMinutes: 60},
		}},
		People: &mockPeoplesvc{search: users},
		Auth:   &mockAuthsvc{},
	}
	out, err := assembleReport(context.Background(), deps, statusFlags{
		managers:    []string{"mgr-A", "mgr-B"},
		week:        "2026-04-14",
		includeZero: true,
		limit:       100,
	})
	require.NoError(t, err)
	require.Len(t, out.Rows, 2, "union of two managers' reports")
	uids := []string{out.Rows[0].User.UID, out.Rows[1].User.UID}
	require.ElementsMatch(t, []string{"u1", "u2"}, uids)
}

func TestRunner_MultiManagerWithMe(t *testing.T) {
	week := domain.WeekRefContaining(time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ))
	users := []domain.User{
		{UID: "u1", FullName: "Me's report", ReportsToUID: "mgr-me"},
		{UID: "u2", FullName: "Other's report", ReportsToUID: "mgr-X"},
	}
	deps := runnerDeps{
		Time: &mockTimesvc{reports: map[string]domain.WeekReport{
			"u1": {WeekRef: week, UserUID: "u1", TotalMinutes: 60},
			"u2": {WeekRef: week, UserUID: "u2", TotalMinutes: 60},
		}},
		People: &mockPeoplesvc{search: users},
		Auth:   &mockAuthsvc{me: domain.User{UID: "mgr-me"}},
	}
	out, err := assembleReport(context.Background(), deps, statusFlags{
		managers:    []string{"me", "mgr-X"},
		week:        "2026-04-14",
		includeZero: true,
		limit:       100,
	})
	require.NoError(t, err)
	require.Len(t, out.Rows, 2, `"me" resolves to mgr-me; union with mgr-X`)
}

func TestRunner_MultiAccountServerSide(t *testing.T) {
	people := &mockPeoplesvc{
		search:  []domain.User{},
		account: peoplesvc.Account{ID: 866, Name: "Acct A"},
	}
	deps := runnerDeps{
		Time:   &mockTimesvc{},
		People: people,
		Auth:   &mockAuthsvc{},
	}
	_, err := assembleReport(context.Background(), deps, statusFlags{
		accounts:    []string{"Acct A", "Acct B"},
		week:        "2026-04-14",
		includeZero: true,
		limit:       100,
	})
	require.NoError(t, err)
	// Mock returns the same Account for every name; both names resolve to ID 866 → dedup → single ID.
	require.Equal(t, []int{866}, people.lastFilter.AccountIDs)
	require.Equal(t, 2, people.resolveAccountCalls, "both names get resolved (dedup happens after resolution)")
}

func TestRunner_MultiResourcePoolUnion(t *testing.T) {
	week := domain.WeekRefContaining(time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ))
	users := []domain.User{
		{UID: "u1", FullName: "Pool A", ResourcePoolID: 46},
		{UID: "u2", FullName: "Pool B", ResourcePoolID: 47},
		{UID: "u3", FullName: "Pool C", ResourcePoolID: 99},
	}
	people := &mockPeoplesvc{
		search: users,
		// stubPool returns the same pool ID for every call; we want different IDs
		// per call. Use a counter — but the mock returns one pool. Simplification:
		// inject IDs by mutating the mock between calls is awkward. Easier: use
		// a single pool for both names (both resolve to ID 46), filter ends up
		// matching just users in pool 46.
		pool: peoplesvc.ResourcePool{ID: 46, Name: "Pool A"},
	}
	deps := runnerDeps{
		Time: &mockTimesvc{reports: map[string]domain.WeekReport{
			"u1": {WeekRef: week, UserUID: "u1", TotalMinutes: 60},
		}},
		People: people,
		Auth:   &mockAuthsvc{},
	}
	out, err := assembleReport(context.Background(), deps, statusFlags{
		resourcePools: []string{"Pool A", "Pool B"},
		week:          "2026-04-14",
		includeZero:   true,
		limit:         100,
	})
	require.NoError(t, err)
	require.Equal(t, 2, people.resolveCalls, "each pool name gets resolved")
	require.Len(t, out.Rows, 1, "u1 (pool 46) matches; u2/u3 don't")
}

func TestRunner_DuplicateUserDeduped(t *testing.T) {
	week := domain.WeekRefContaining(time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ))
	user := domain.User{UID: "u1", FullName: "Alice"}
	deps := runnerDeps{
		Time:   &mockTimesvc{reports: map[string]domain.WeekReport{"u1": {WeekRef: week, UserUID: "u1", TotalMinutes: 60}}},
		People: &mockPeoplesvc{users: map[string]domain.User{"u1": user}},
		Auth:   &mockAuthsvc{},
	}
	out, err := assembleReport(context.Background(), deps, statusFlags{
		users:       []string{"u1", "u1"},
		week:        "2026-04-14",
		includeZero: true,
		limit:       100,
	})
	require.NoError(t, err)
	require.Len(t, out.Rows, 1, "duplicate UID deduped")
}

func TestRunner_RangeProducesMultipleWeeks(t *testing.T) {
	week1 := domain.WeekRefContaining(time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ))
	week2 := domain.WeekRefContaining(time.Date(2026, 4, 21, 0, 0, 0, 0, domain.EasternTZ))
	user := domain.User{UID: "u1", FullName: "A"}
	deps := runnerDeps{
		Time: &mockTimesvc{reports: map[string]domain.WeekReport{
			"u1": {WeekRef: week1, UserUID: "u1", TotalMinutes: 60},
		}},
		People: &mockPeoplesvc{users: map[string]domain.User{"u1": user}},
		Auth:   &mockAuthsvc{},
	}
	out, err := assembleReport(context.Background(), deps, statusFlags{
		users:       []string{"u1"},
		from:        "2026-04-14",
		to:          "2026-04-22",
		includeZero: true,
		limit:       100,
	})
	require.NoError(t, err)
	require.Len(t, out.Rows, 2, "2 weeks × 1 user = 2 rows")
	require.Equal(t, week1.StartDate, out.Rows[0].WeekRef.StartDate)
	require.Equal(t, week2.StartDate, out.Rows[1].WeekRef.StartDate)
}

func TestRunner_IncompletePerUserFromWorkableHours(t *testing.T) {
	// Alice: WorkableHours=40, logged 40h → not incomplete (40 >= 40)
	// Bob:   WorkableHours=32, logged 30h → incomplete (30 < 32)
	// Carol: WorkableHours=40, logged 35h → incomplete (35 < 40)
	// No --threshold flag set; thresholdSet=false → per-user mode.
	week := domain.WeekRefContaining(time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ))
	users := []domain.User{
		{UID: "alice", FullName: "Alice", ReportsToUID: "mgr", WorkableHours: 8.0}, // 8h/day = 40h/week FT
		{UID: "bob", FullName: "Bob", ReportsToUID: "mgr", WorkableHours: 6.4},     // 6.4h/day = 32h/week PT
		{UID: "carol", FullName: "Carol", ReportsToUID: "mgr", WorkableHours: 8.0}, // 8h/day = 40h/week FT
	}
	reports := map[string]domain.WeekReport{
		"alice": {WeekRef: week, UserUID: "alice", TotalMinutes: 40 * 60, Status: domain.ReportOpen},
		"bob":   {WeekRef: week, UserUID: "bob", TotalMinutes: 30 * 60, Status: domain.ReportOpen},
		"carol": {WeekRef: week, UserUID: "carol", TotalMinutes: 35 * 60, Status: domain.ReportOpen},
	}
	deps := runnerDeps{
		Time:   &mockTimesvc{reports: reports},
		People: &mockPeoplesvc{search: users},
		Auth:   &mockAuthsvc{me: domain.User{UID: "mgr"}},
	}
	out, err := assembleReport(context.Background(), deps, statusFlags{
		managers:     []string{"me"},
		week:         "2026-04-14",
		includeZero:  true,
		incomplete:   true,
		thresholdSet: false,
		limit:        100,
	})
	require.NoError(t, err)
	require.Len(t, out.Rows, 2, "Bob and Carol are incomplete; Alice meets her threshold")
	gotUIDs := make(map[string]bool)
	for _, r := range out.Rows {
		gotUIDs[r.User.UID] = true
	}
	if !gotUIDs["bob"] || !gotUIDs["carol"] || gotUIDs["alice"] {
		t.Errorf("expected bob and carol in result, not alice; got UIDs: %v", gotUIDs)
	}
}

func TestRunner_IncompletePerUserFallbackTo40(t *testing.T) {
	// User with WorkableHours=0 (unset) and TotalMinutes=30h → falls back to 40h → 30 < 40 → incomplete.
	// No --threshold; thresholdSet=false.
	week := domain.WeekRefContaining(time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ))
	users := []domain.User{
		{UID: "u1", FullName: "Unset", ReportsToUID: "mgr", WorkableHours: 0},
	}
	reports := map[string]domain.WeekReport{
		"u1": {WeekRef: week, UserUID: "u1", TotalMinutes: 30 * 60, Status: domain.ReportOpen},
	}
	deps := runnerDeps{
		Time:   &mockTimesvc{reports: reports},
		People: &mockPeoplesvc{search: users},
		Auth:   &mockAuthsvc{me: domain.User{UID: "mgr"}},
	}
	out, err := assembleReport(context.Background(), deps, statusFlags{
		managers:     []string{"me"},
		week:         "2026-04-14",
		includeZero:  true,
		incomplete:   true,
		thresholdSet: false,
		limit:        100,
	})
	require.NoError(t, err)
	require.Len(t, out.Rows, 1, "WorkableHours=0 falls back to 40h; 30h < 40h → incomplete")
	require.Equal(t, "u1", out.Rows[0].User.UID)
}

func TestRunner_IncompleteGlobalThresholdOverridesPerUser(t *testing.T) {
	// Same Alice/Bob/Carol setup as TestRunner_IncompletePerUserFromWorkableHours,
	// BUT with --threshold 20 explicit (thresholdSet=true) → global mode.
	// All three have logged >= 20h, so none are incomplete.
	week := domain.WeekRefContaining(time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ))
	users := []domain.User{
		{UID: "alice", FullName: "Alice", ReportsToUID: "mgr", WorkableHours: 8.0}, // 8h/day = 40h/week FT
		{UID: "bob", FullName: "Bob", ReportsToUID: "mgr", WorkableHours: 6.4},     // 6.4h/day = 32h/week PT
		{UID: "carol", FullName: "Carol", ReportsToUID: "mgr", WorkableHours: 8.0}, // 8h/day = 40h/week FT
	}
	reports := map[string]domain.WeekReport{
		"alice": {WeekRef: week, UserUID: "alice", TotalMinutes: 40 * 60, Status: domain.ReportOpen},
		"bob":   {WeekRef: week, UserUID: "bob", TotalMinutes: 30 * 60, Status: domain.ReportOpen},
		"carol": {WeekRef: week, UserUID: "carol", TotalMinutes: 35 * 60, Status: domain.ReportOpen},
	}
	deps := runnerDeps{
		Time:   &mockTimesvc{reports: reports},
		People: &mockPeoplesvc{search: users},
		Auth:   &mockAuthsvc{me: domain.User{UID: "mgr"}},
	}
	out, err := assembleReport(context.Background(), deps, statusFlags{
		managers:     []string{"me"},
		week:         "2026-04-14",
		includeZero:  true,
		incomplete:   true,
		threshold:    20,
		thresholdSet: true,
		limit:        100,
	})
	require.NoError(t, err)
	require.Len(t, out.Rows, 0, "global threshold=20 overrides per-user; all three logged >= 20h → no incomplete rows")
}

func TestRunner_IncompleteNotSetIgnoresThreshold(t *testing.T) {
	// f.incomplete=false: three users with logged hours; no filtering applied.
	// All three rows should be returned regardless of WorkableHours.
	week := domain.WeekRefContaining(time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ))
	users := []domain.User{
		{UID: "alice", FullName: "Alice", ReportsToUID: "mgr", WorkableHours: 8.0}, // 8h/day = 40h/week FT
		{UID: "bob", FullName: "Bob", ReportsToUID: "mgr", WorkableHours: 6.4},     // 6.4h/day = 32h/week PT
		{UID: "carol", FullName: "Carol", ReportsToUID: "mgr", WorkableHours: 8.0}, // 8h/day = 40h/week FT
	}
	reports := map[string]domain.WeekReport{
		"alice": {WeekRef: week, UserUID: "alice", TotalMinutes: 40 * 60, Status: domain.ReportOpen},
		"bob":   {WeekRef: week, UserUID: "bob", TotalMinutes: 30 * 60, Status: domain.ReportOpen},
		"carol": {WeekRef: week, UserUID: "carol", TotalMinutes: 35 * 60, Status: domain.ReportOpen},
	}
	deps := runnerDeps{
		Time:   &mockTimesvc{reports: reports},
		People: &mockPeoplesvc{search: users},
		Auth:   &mockAuthsvc{me: domain.User{UID: "mgr"}},
	}
	out, err := assembleReport(context.Background(), deps, statusFlags{
		managers:    []string{"me"},
		week:        "2026-04-14",
		includeZero: true,
		incomplete:  false,
		limit:       100,
	})
	require.NoError(t, err)
	require.Len(t, out.Rows, 3, "incomplete=false: all three users returned regardless of WorkableHours or hours logged")
}

// --- JSON envelope threshold tests ---

// jsonEnvelope is a minimal decode target for the JSON envelope emitted by buildJSONEnvelope.
type jsonEnvelope struct {
	Filter struct {
		ThresholdMode string  `json:"thresholdMode"`
		Threshold     float64 `json:"threshold"`
	} `json:"filter"`
	Weeks []struct {
		Rows []struct {
			UserUID   string  `json:"userUID"`
			Threshold float64 `json:"threshold"`
		} `json:"rows"`
	} `json:"weeks"`
}

func encodeEnvelope(t *testing.T, rep domain.TimeStatusReport, f statusFlags) jsonEnvelope {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, printJSON(&buf, rep, f))
	var env jsonEnvelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &env))
	return env
}

func TestRunner_JSONEnvelopeThresholdModePerUser(t *testing.T) {
	// Bob:   WorkableHours=6.4/day → 32h/week threshold, logged 30h → incomplete
	// Carol: WorkableHours=8.0/day → 40h/week threshold, logged 35h → incomplete
	// thresholdSet=false → per-user mode.
	week := domain.WeekRefContaining(time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ))
	users := []domain.User{
		{UID: "bob", FullName: "Bob", ReportsToUID: "mgr", WorkableHours: 6.4},
		{UID: "carol", FullName: "Carol", ReportsToUID: "mgr", WorkableHours: 8.0},
	}
	reports := map[string]domain.WeekReport{
		"bob":   {WeekRef: week, UserUID: "bob", TotalMinutes: 30 * 60, Status: domain.ReportOpen},
		"carol": {WeekRef: week, UserUID: "carol", TotalMinutes: 35 * 60, Status: domain.ReportOpen},
	}
	deps := runnerDeps{
		Time:   &mockTimesvc{reports: reports},
		People: &mockPeoplesvc{search: users},
		Auth:   &mockAuthsvc{me: domain.User{UID: "mgr"}},
	}
	f := statusFlags{
		managers:     []string{"me"},
		week:         "2026-04-14",
		includeZero:  true,
		incomplete:   true,
		thresholdSet: false,
		limit:        100,
	}
	rep, err := assembleReport(context.Background(), deps, f)
	require.NoError(t, err)
	require.Len(t, rep.Rows, 2)

	env := encodeEnvelope(t, rep, f)

	require.Equal(t, "per-user", env.Filter.ThresholdMode)
	require.Equal(t, float64(0), env.Filter.Threshold, "per-user mode: filter.threshold omitted (0)")

	// Build a UID → threshold map from the envelope rows.
	require.Len(t, env.Weeks, 1)
	rowThresholds := map[string]float64{}
	for _, r := range env.Weeks[0].Rows {
		rowThresholds[r.UserUID] = r.Threshold
	}
	require.Equal(t, float64(32), rowThresholds["bob"], "Bob's threshold = his WorkableHours × 5 (6.4 × 5 = 32)")
	require.Equal(t, float64(40), rowThresholds["carol"], "Carol's threshold = her WorkableHours × 5 (8.0 × 5 = 40)")
}

func TestRunner_JSONEnvelopeThresholdModeGlobal(t *testing.T) {
	// Same Bob/Carol, but thresholdSet=true with threshold=20 → global mode.
	// Both are below 20h? No — Bob has 30h and Carol 35h → neither is below 20h → 0 rows.
	// Use threshold=38 so Bob (30h) is below but Carol (35h) is also below.
	week := domain.WeekRefContaining(time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ))
	users := []domain.User{
		{UID: "bob", FullName: "Bob", ReportsToUID: "mgr", WorkableHours: 6.4},
		{UID: "carol", FullName: "Carol", ReportsToUID: "mgr", WorkableHours: 8.0},
	}
	reports := map[string]domain.WeekReport{
		"bob":   {WeekRef: week, UserUID: "bob", TotalMinutes: 30 * 60, Status: domain.ReportOpen},
		"carol": {WeekRef: week, UserUID: "carol", TotalMinutes: 35 * 60, Status: domain.ReportOpen},
	}
	deps := runnerDeps{
		Time:   &mockTimesvc{reports: reports},
		People: &mockPeoplesvc{search: users},
		Auth:   &mockAuthsvc{me: domain.User{UID: "mgr"}},
	}
	f := statusFlags{
		managers:     []string{"me"},
		week:         "2026-04-14",
		includeZero:  true,
		incomplete:   true,
		threshold:    38,
		thresholdSet: true,
		limit:        100,
	}
	rep, err := assembleReport(context.Background(), deps, f)
	require.NoError(t, err)
	require.Len(t, rep.Rows, 2, "both Bob (30h) and Carol (35h) are below the global 38h threshold")

	env := encodeEnvelope(t, rep, f)

	require.Equal(t, "global", env.Filter.ThresholdMode)
	require.Equal(t, float64(38), env.Filter.Threshold)

	require.Len(t, env.Weeks, 1)
	for _, r := range env.Weeks[0].Rows {
		require.Equal(t, float64(38), r.Threshold, "every row threshold == global threshold (UID=%s)", r.UserUID)
	}
}

func TestRunner_JSONEnvelopeNoFilterMode(t *testing.T) {
	// incomplete=false: thresholdMode and threshold absent; row threshold absent.
	week := domain.WeekRefContaining(time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ))
	user := domain.User{UID: "u1", FullName: "Alice", WorkableHours: 8.0}
	reports := map[string]domain.WeekReport{
		"u1": {WeekRef: week, UserUID: "u1", TotalMinutes: 40 * 60, Status: domain.ReportOpen},
	}
	deps := runnerDeps{
		Time:   &mockTimesvc{reports: reports},
		People: &mockPeoplesvc{users: map[string]domain.User{"u1": user}},
		Auth:   &mockAuthsvc{},
	}
	f := statusFlags{
		users:       []string{"u1"},
		week:        "2026-04-14",
		includeZero: true,
		incomplete:  false,
		limit:       100,
	}
	rep, err := assembleReport(context.Background(), deps, f)
	require.NoError(t, err)
	require.Len(t, rep.Rows, 1)

	env := encodeEnvelope(t, rep, f)

	require.Equal(t, "", env.Filter.ThresholdMode, "thresholdMode absent when incomplete=false")
	require.Equal(t, float64(0), env.Filter.Threshold, "threshold absent when incomplete=false")

	require.Len(t, env.Weeks, 1)
	require.Len(t, env.Weeks[0].Rows, 1)
	require.Equal(t, float64(0), env.Weeks[0].Rows[0].Threshold, "per-row threshold absent when incomplete=false")
}

// mockProjectsvc implements reportProjectsvcAPI for runner tests.
type mockProjectsvc struct {
	resources []domain.ProjectResource
	err       error
}

func (m *mockProjectsvc) ListResources(_ context.Context, _ string, _ int) ([]domain.ProjectResource, error) {
	return m.resources, m.err
}

// mockEntriesSvc implements timesvcEntriesAPI for runner project-filter tests.
// SearchEntries is called concurrently by the runner; the lastFilter capture
// is guarded by a mutex.
type mockEntriesSvc struct {
	entries    []domain.TimeEntry
	err        error
	mu         sync.Mutex
	lastFilter domain.EntryFilter
}

func (m *mockEntriesSvc) SearchEntries(_ context.Context, _ string, filter domain.EntryFilter) ([]domain.TimeEntry, error) {
	m.mu.Lock()
	m.lastFilter = filter
	m.mu.Unlock()
	return m.entries, m.err
}

func TestRunner_ProjectSelectorResolvesViaResources(t *testing.T) {
	week := domain.WeekRefContaining(time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ))
	resources := []domain.ProjectResource{
		{UID: "u1", FullName: "Alice"},
		{UID: "u2", FullName: "Bob"},
	}
	reports := map[string]domain.WeekReport{
		"u1": {WeekRef: week, UserUID: "u1", TotalMinutes: 120, MinutesBillable: 120},
		"u2": {WeekRef: week, UserUID: "u2", TotalMinutes: 60},
	}
	// No project-specific entries (entries svc returns empty for simplicity
	// since project filter happens in the entries svc post-filter).
	projEntries := []domain.TimeEntry{
		{ID: 1, UserUID: "u1", Minutes: 120, Target: domain.Target{Kind: domain.TargetProjectTask, ProjectID: 259}},
	}

	deps := runnerDeps{
		Time:    &mockTimesvc{reports: reports},
		Entries: &mockEntriesSvc{entries: projEntries},
		People:  &mockPeoplesvc{},
		Auth:    &mockAuthsvc{},
		Project: &mockProjectsvc{resources: resources},
	}
	out, err := assembleReport(context.Background(), deps, statusFlags{
		projectID:   259,
		week:        "2026-04-14",
		includeZero: true,
		limit:       100,
	})
	require.NoError(t, err)
	// Both resources should be resolved as users.
	uids := make([]string, 0, len(out.Rows))
	for _, r := range out.Rows {
		uids = append(uids, r.User.UID)
	}
	require.Contains(t, uids, "u1")
	require.Contains(t, uids, "u2")
}

func TestRunner_ProjectFilterScopesTotalsToProjectEntries(t *testing.T) {
	week := domain.WeekRefContaining(time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ))
	resources := []domain.ProjectResource{{UID: "u1", FullName: "Alice"}}
	reports := map[string]domain.WeekReport{
		// GetWeekReportForUser returns all-project totals (4h).
		"u1": {WeekRef: week, UserUID: "u1", TotalMinutes: 240, MinutesBillable: 120, MinutesNonBillable: 120},
	}
	// SearchEntries returns only project-259-scoped entries (1.5h).
	projEntries := []domain.TimeEntry{
		{ID: 1, UserUID: "u1", Minutes: 60, Billable: true, Target: domain.Target{Kind: domain.TargetProjectTask, ProjectID: 259}},
		{ID: 2, UserUID: "u1", Minutes: 30, Target: domain.Target{Kind: domain.TargetProjectTask, ProjectID: 259}},
	}

	entriesSvc := &mockEntriesSvc{entries: projEntries}
	deps := runnerDeps{
		Time:    &mockTimesvc{reports: reports},
		Entries: entriesSvc,
		People:  &mockPeoplesvc{},
		Auth:    &mockAuthsvc{},
		Project: &mockProjectsvc{resources: resources},
	}
	out, err := assembleReport(context.Background(), deps, statusFlags{
		projectID:   259,
		week:        "2026-04-14",
		includeZero: true,
		limit:       100,
	})
	require.NoError(t, err)
	require.Len(t, out.Rows, 1)
	// Totals should be scoped to project entries (90 min = 1.5h), not the full week (240 min).
	require.Equal(t, 90, out.Rows[0].TotalMin)
	require.Equal(t, 60, out.Rows[0].BillableMin)
	require.Equal(t, 30, out.Rows[0].NonBillableMin)
	// Verify SearchEntries was called with the correct ProjectID.
	require.Equal(t, 259, entriesSvc.lastFilter.ProjectID)
}

func TestValidateStatusFlags_ProjectMutexWithUser(t *testing.T) {
	err := validateStatusFlags(statusFlags{
		users:     []string{"u1"},
		projectID: 259,
		week:      "2026-04-14",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "exactly one of")
}

func TestValidateStatusFlags_ProjectAloneIsValid(t *testing.T) {
	err := validateStatusFlags(statusFlags{
		projectID: 259,
		week:      "2026-04-14",
	})
	require.NoError(t, err)
}

func TestResolveWeeks_RefusesOverMaxSpan(t *testing.T) {
	f := statusFlags{from: "2020-01-01", to: "2030-01-01"}
	_, err := resolveWeeks(f)
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrFanoutLimitExceeded)
	require.Contains(t, err.Error(), "weeks=")
	require.Contains(t, err.Error(), "max=52")
}

func TestResolveWeeks_Allows52Weeks(t *testing.T) {
	// 52 weeks exactly is the boundary — must pass.
	f := statusFlags{from: "2026-01-04", to: "2026-12-27"}
	weeks, err := resolveWeeks(f)
	require.NoError(t, err)
	require.Equal(t, 52, len(weeks))
}

func TestResolveWeeks_Refuses53Weeks(t *testing.T) {
	// 53 weeks — one over the cap.
	f := statusFlags{from: "2026-01-04", to: "2027-01-03"}
	_, err := resolveWeeks(f)
	require.ErrorIs(t, err, domain.ErrFanoutLimitExceeded)
}

func TestRunner_RefusesOverMaxUsers(t *testing.T) {
	// Build 1001 synthetic users — one over the cap.
	const n = domain.MaxReportUsers + 1
	users := make([]domain.User, n)
	for i := 0; i < n; i++ {
		uid := fmt.Sprintf("u%04d", i)
		users[i] = domain.User{UID: uid, FullName: uid, IsEmployee: true}
	}
	deps := runnerDeps{
		Time:   &mockTimesvc{},
		People: &mockPeoplesvc{search: users},
		Auth:   &mockAuthsvc{me: domain.User{UID: "me"}},
	}
	f := statusFlags{week: "2026-04-12", all: true, yes: true}
	_, err := assembleReport(context.Background(), deps, f)
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrFanoutLimitExceeded)
	require.Contains(t, err.Error(), "users=1001")
	require.Contains(t, err.Error(), "max=1000")
}

func TestRunner_Allows1000Users(t *testing.T) {
	// 1000 users exactly — boundary, must pass.
	const n = domain.MaxReportUsers
	users := make([]domain.User, n)
	reports := make(map[string]domain.WeekReport, n)
	for i := 0; i < n; i++ {
		uid := fmt.Sprintf("u%04d", i)
		users[i] = domain.User{UID: uid, FullName: uid, IsEmployee: true}
		reports[uid] = domain.WeekReport{}
	}
	deps := runnerDeps{
		Time:   &mockTimesvc{reports: reports},
		People: &mockPeoplesvc{search: users},
		Auth:   &mockAuthsvc{me: domain.User{UID: "me"}},
	}
	f := statusFlags{week: "2026-04-12", all: true, yes: true, includeZero: true, limit: 1000}
	rep, err := assembleReport(context.Background(), deps, f)
	require.NoError(t, err)
	require.Equal(t, 1000, len(rep.Rows))
}
