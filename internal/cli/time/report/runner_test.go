package report

import (
	"context"
	"errors"
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
		account: peoplesvc.Account{ID: 866, Name: "14300000 (IT-ICT INFRA COMM TECHNOLOGY)"},
	}
	deps := runnerDeps{
		Time:   &mockTimesvc{},
		People: people,
		Auth:   &mockAuthsvc{},
	}
	_, err := assembleReport(context.Background(), deps, statusFlags{
		accounts:    []string{"14300000 (IT-ICT INFRA COMM TECHNOLOGY)"},
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
