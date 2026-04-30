package report

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
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
	users      map[string]domain.User
	search     []domain.User
	lastFilter domain.UserFilter
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
		manager:     "me",
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

func TestRunner_AccountSelectorSetsEmployeeFilter(t *testing.T) {
	people := &mockPeoplesvc{search: []domain.User{}}
	deps := runnerDeps{
		Time:   &mockTimesvc{},
		People: people,
		Auth:   &mockAuthsvc{},
	}
	_, err := assembleReport(context.Background(), deps, statusFlags{
		account:     "UFIT",
		week:        "2026-04-14",
		includeZero: true,
		limit:       100,
	})
	require.NoError(t, err)
	require.NotNil(t, people.lastFilter.Employee)
	require.True(t, *people.lastFilter.Employee)
	require.Equal(t, "UFIT", people.lastFilter.AccountName)
	require.Equal(t, employeeLimit, people.lastFilter.Limit)
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
