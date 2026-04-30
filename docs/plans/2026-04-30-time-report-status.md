# Time Status Report Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> Each task follows strict TDD where applicable. Never amend commits — always create new ones. Branch: `time-report-status` (already created off `main`, has the spec at `docs/specs/2026-04-30-time-report-status.md`).
>
> No `Co-Authored-By` trailer on commit messages. Subagent-driven execution recommended (multiple new packages + CLI tree).

**Design spec:** `docs/specs/2026-04-30-time-report-status.md`

**Goal:** Add `tdx time report status` and `get_time_status_report` MCP tool — a per-user weekly time-status roll-up with billable/non-billable/total hours and submission status. Five output formats: human table (default), JSON, CSV (stdout), XLSX (file).

**Architecture:** Domain types (`WeekStatusRow`, `TimeStatusReport`, extended `User`, `UserFilter`, `ErrPermission`). New per-user week endpoint in `timesvc.GetWeekReportForUser`. New `peoplesvc` package for user lookups. CLI orchestrator fans out per-(user, week) requests with `errgroup` (concurrency 5). Output renderers split by format. MCP exposes the report as a read-only tool.

**Tech Stack:** Go, cobra, `github.com/xuri/excelize/v2` (new), `golang.org/x/sync/errgroup` (new), stdlib `encoding/csv`.

**Affected packages:** `internal/domain`, `internal/svc/timesvc`, `internal/svc/peoplesvc` (new), `internal/cli/time/report` (new), `internal/mcp`, `internal/tdx`.

---

## Task 1: Add ErrPermission + UserFilter + WeekReport billable fields

**Files:**
- Modify: `internal/domain/errors.go`
- Modify: `internal/domain/user.go`
- Modify: `internal/domain/user_test.go`
- Create: `internal/domain/user_filter.go`
- Modify: `internal/domain/week.go`
- Modify: `internal/domain/week_test.go`

- [ ] **Step 1.1 — Failing test for ErrPermission**

Append to `internal/domain/errors_test.go`:

```go
func TestErrPermission_Wrappable(t *testing.T) {
	wrapped := fmt.Errorf("get report: %w", ErrPermission)
	require.ErrorIs(t, wrapped, ErrPermission)
}
```

Run: `go test ./internal/domain/ -run TestErrPermission_Wrappable -v`
Expected: FAIL with "undefined: ErrPermission".

- [ ] **Step 1.2 — Add ErrPermission**

In `internal/domain/errors.go`, add to the `var (...)` block:

```go
	// ErrPermission indicates the API rejected the request because the
	// caller lacks the necessary role/app/approver relationship.
	ErrPermission = errors.New("permission denied")
```

Run: `go test ./internal/domain/ -run TestErrPermission_Wrappable -v`
Expected: PASS.

- [ ] **Step 1.3 — Failing test for User extension**

Append to `internal/domain/user_test.go`:

```go
func TestUser_HasManagerFields(t *testing.T) {
	u := User{
		UID:            "user-1",
		FullName:       "Iain Moffat",
		Email:          "ipm@ufl.edu",
		Active:         true,
		AccountName:    "UFIT Operations",
		ReportsToUID:   "mgr-uid",
		ReportsToID:    42,
		ReportsToName:  "Manager Name",
		ReportsToEmail: "mgr@ufl.edu",
	}
	require.Equal(t, "user-1", u.UID)
	require.Equal(t, "mgr-uid", u.ReportsToUID)
	require.Equal(t, 42, u.ReportsToID)
	require.Equal(t, "Manager Name", u.ReportsToName)
	require.Equal(t, "mgr@ufl.edu", u.ReportsToEmail)
	require.True(t, u.Active)
	require.Equal(t, "UFIT Operations", u.AccountName)
}
```

Run: `go test ./internal/domain/ -run TestUser_HasManagerFields -v`
Expected: FAIL — fields don't exist yet.

- [ ] **Step 1.4 — Extend User**

In `internal/domain/user.go`, replace the User struct with:

```go
type User struct {
	ID             int    `json:"id,omitempty"             yaml:"id,omitempty"`
	UID            string `json:"uid,omitempty"            yaml:"uid,omitempty"`
	FullName       string `json:"fullName,omitempty"       yaml:"fullName,omitempty"`
	Email          string `json:"email,omitempty"          yaml:"email,omitempty"`
	Active         bool   `json:"active,omitempty"         yaml:"active,omitempty"`
	AccountName    string `json:"accountName,omitempty"    yaml:"accountName,omitempty"`
	ReportsToUID   string `json:"reportsToUID,omitempty"   yaml:"reportsToUID,omitempty"`
	ReportsToID    int    `json:"reportsToID,omitempty"    yaml:"reportsToID,omitempty"`
	ReportsToName  string `json:"reportsToName,omitempty"  yaml:"reportsToName,omitempty"`
	ReportsToEmail string `json:"reportsToEmail,omitempty" yaml:"reportsToEmail,omitempty"`
}
```

Run: `go test ./internal/domain/ -run TestUser_HasManagerFields -v`
Expected: PASS.

- [ ] **Step 1.5 — Create UserFilter**

Create `internal/domain/user_filter.go`:

```go
package domain

// UserFilter constrains a SearchUsers call. Zero-valued fields mean
// "no constraint". A pointer-typed bool (Active) distinguishes
// "no filter" (nil) from "explicitly false" (false).
type UserFilter struct {
	Active      *bool  // nil = no filter; non-nil = filter to this value
	UserType    string // default "User"
	AccountID   int    // 0 = no filter
	AccountName string // "" = no filter
	NameLike    string // "" = no filter
	Limit       int    // 0 = client default (100)
}
```

- [ ] **Step 1.6 — Failing test for WeekReport billable fields**

Append to `internal/domain/week_test.go`:

```go
func TestWeekReport_BillableHourMethods(t *testing.T) {
	r := WeekReport{
		MinutesBillable:    300,  // 5h
		MinutesNonBillable: 90,   // 1.5h
		TotalMinutes:       390,  // 6.5h
	}
	require.InDelta(t, 5.0, r.BillableHours(), 0.001)
	require.InDelta(t, 1.5, r.NonBillableHours(), 0.001)
	require.InDelta(t, 6.5, r.TotalHours(), 0.001)
}
```

Run: `go test ./internal/domain/ -run TestWeekReport_BillableHourMethods -v`
Expected: FAIL — fields/methods undefined.

- [ ] **Step 1.7 — Extend WeekReport**

In `internal/domain/week.go`, replace the WeekReport struct + add methods:

```go
type WeekReport struct {
	WeekRef            WeekRef      `json:"weekRef"`
	UserUID            string       `json:"userUID"`
	TotalMinutes       int          `json:"totalMinutes"`
	MinutesBillable    int          `json:"minutesBillable,omitempty"`
	MinutesNonBillable int          `json:"minutesNonBillable,omitempty"`
	Status             ReportStatus `json:"status"`
	Days               []DaySummary `json:"days"`
	Entries            []TimeEntry  `json:"entries"`
}

func (w WeekReport) TotalHours() float64        { return float64(w.TotalMinutes) / 60.0 }
func (w WeekReport) BillableHours() float64     { return float64(w.MinutesBillable) / 60.0 }
func (w WeekReport) NonBillableHours() float64  { return float64(w.MinutesNonBillable) / 60.0 }
```

(Existing `TotalHours()` stays. New methods are added.)

Run: `go test ./internal/domain/... -count=1`
Expected: PASS.

- [ ] **Step 1.8 — Commit**

```bash
git add internal/domain/errors.go internal/domain/errors_test.go internal/domain/user.go internal/domain/user_test.go internal/domain/user_filter.go internal/domain/week.go internal/domain/week_test.go
git commit -m "feat(domain): ErrPermission + User manager fields + WeekReport billable totals"
```

---

## Task 2: WeekStatusRow + TimeStatusReport types

**Files:**
- Create: `internal/domain/time_status_report.go`
- Create: `internal/domain/time_status_report_test.go`

- [ ] **Step 2.1 — Failing test**

Create `internal/domain/time_status_report_test.go`:

```go
package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWeekStatusRow_HourMethods(t *testing.T) {
	r := WeekStatusRow{
		BillableMin:    300,
		NonBillableMin: 60,
		TotalMin:       360,
	}
	require.InDelta(t, 5.0, r.BillableHours(), 0.001)
	require.InDelta(t, 1.0, r.NonBillableHours(), 0.001)
	require.InDelta(t, 6.0, r.TotalHours(), 0.001)
}

func TestTimeStatusReport_TotalsAcrossRows(t *testing.T) {
	week1 := WeekRef{
		StartDate: time.Date(2026, 4, 12, 0, 0, 0, 0, EasternTZ),
		EndDate:   time.Date(2026, 4, 18, 0, 0, 0, 0, EasternTZ),
	}
	rep := TimeStatusReport{
		From: week1,
		To:   week1,
		Rows: []WeekStatusRow{
			{WeekRef: week1, BillableMin: 240, NonBillableMin: 0, TotalMin: 240},
			{WeekRef: week1, BillableMin: 60, NonBillableMin: 60, TotalMin: 120},
		},
	}
	bill, nonBill, tot := rep.Totals()
	require.Equal(t, 300, bill)
	require.Equal(t, 60, nonBill)
	require.Equal(t, 360, tot)
}

func TestTimeStatusReport_RowsByWeek_GroupsAndOrders(t *testing.T) {
	w1 := WeekRef{StartDate: time.Date(2026, 4, 12, 0, 0, 0, 0, EasternTZ)}
	w2 := WeekRef{StartDate: time.Date(2026, 4, 19, 0, 0, 0, 0, EasternTZ)}
	rep := TimeStatusReport{
		Rows: []WeekStatusRow{
			{WeekRef: w2, User: User{FullName: "Bob"}},
			{WeekRef: w1, User: User{FullName: "Alice"}},
			{WeekRef: w1, User: User{FullName: "Charlie"}},
		},
	}
	groups := rep.RowsByWeek()
	require.Len(t, groups, 2)
	require.True(t, groups[0].Week.StartDate.Before(groups[1].Week.StartDate),
		"groups sorted by week ascending")
	require.Equal(t, w1, groups[0].Week)
	// Within week, order preserves input (caller responsibility).
	require.Len(t, groups[0].Rows, 2)
}
```

Run: `go test ./internal/domain/ -run 'TestWeekStatusRow|TestTimeStatusReport' -v`
Expected: FAIL — types not defined.

- [ ] **Step 2.2 — Create the types**

Create `internal/domain/time_status_report.go`:

```go
package domain

import "sort"

// WeekStatusRow is one (user × week) row of a TimeStatusReport.
// Status is the user's weekly time-report submission status.
type WeekStatusRow struct {
	WeekRef        WeekRef      `json:"weekRef"`
	User           User         `json:"user"`
	Status         ReportStatus `json:"status"`
	BillableMin    int          `json:"billableMinutes"`
	NonBillableMin int          `json:"nonBillableMinutes"`
	TotalMin       int          `json:"totalMinutes"`
}

func (r WeekStatusRow) BillableHours() float64    { return float64(r.BillableMin) / 60.0 }
func (r WeekStatusRow) NonBillableHours() float64 { return float64(r.NonBillableMin) / 60.0 }
func (r WeekStatusRow) TotalHours() float64       { return float64(r.TotalMin) / 60.0 }

// TimeStatusReport is the assembled output of `tdx time report status`.
type TimeStatusReport struct {
	From WeekRef         `json:"from"`
	To   WeekRef         `json:"to"`
	Rows []WeekStatusRow `json:"rows"`
}

// Totals returns aggregated minutes across all rows.
func (r TimeStatusReport) Totals() (billable, nonBillable, total int) {
	for _, row := range r.Rows {
		billable += row.BillableMin
		nonBillable += row.NonBillableMin
		total += row.TotalMin
	}
	return
}

// WeekGroup bundles rows that fall within a single week.
type WeekGroup struct {
	Week WeekRef
	Rows []WeekStatusRow
}

// RowsByWeek groups rows by their WeekRef.StartDate, ordered ascending by
// week. Order within a week preserves input order.
func (r TimeStatusReport) RowsByWeek() []WeekGroup {
	idx := map[time.Time]int{} // map omitted to keep dep on time.Time minimal
	// We sort weeks by StartDate; rows within a week keep input order.
	weeks := []WeekRef{}
	groups := []WeekGroup{}
	for _, row := range r.Rows {
		key := row.WeekRef.StartDate
		if pos, ok := idx[key]; ok {
			groups[pos].Rows = append(groups[pos].Rows, row)
			continue
		}
		idx[key] = len(groups)
		groups = append(groups, WeekGroup{Week: row.WeekRef, Rows: []WeekStatusRow{row}})
		weeks = append(weeks, row.WeekRef)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		return groups[i].Week.StartDate.Before(groups[j].Week.StartDate)
	})
	return groups
}
```

(Note: the `idx map[time.Time]int` requires the file to import `time`. Add that to the import list.)

Replace the file contents with:

```go
package domain

import (
	"sort"
	"time"
)

// ... [same body, but with the time import declared]
```

- [ ] **Step 2.3 — Run tests**

Run: `go test ./internal/domain/ -run 'TestWeekStatusRow|TestTimeStatusReport' -v`
Expected: PASS.

- [ ] **Step 2.4 — Commit**

```bash
git add internal/domain/time_status_report.go internal/domain/time_status_report_test.go
git commit -m "feat(domain): WeekStatusRow + TimeStatusReport types"
```

---

## Task 3: timesvc.GetWeekReportForUser + ErrPermission mapping + billable plumbing

**Files:**
- Modify: `internal/svc/timesvc/week.go`
- Create: `internal/svc/timesvc/week_user.go`
- Create: `internal/svc/timesvc/week_user_test.go`
- Modify: `internal/svc/timesvc/week_test.go`

- [ ] **Step 3.1 — Failing test for billable plumbing in GetWeekReport**

Add to `internal/svc/timesvc/week_test.go`:

```go
func TestGetWeekReport_PopulatesBillableTotals(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"PeriodStartDate": "2026-04-12T00:00:00Z",
			"PeriodEndDate":   "2026-04-18T00:00:00Z",
			"Status":          1,
			"Times":           [],
			"TimeReportUid":   "user-1",
			"MinutesBillable": 300,
			"MinutesNonBillable": 90,
			"MinutesTotal":    390
		}`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	report, err := svc.GetWeekReport(context.Background(), profile, time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ))
	require.NoError(t, err)
	require.Equal(t, 300, report.MinutesBillable)
	require.Equal(t, 90, report.MinutesNonBillable)
	require.Equal(t, 390, report.TotalMinutes)
}
```

Run: `go test ./internal/svc/timesvc/ -run TestGetWeekReport_PopulatesBillableTotals -v`
Expected: FAIL — fields not propagated.

- [ ] **Step 3.2 — Plumb billable totals in GetWeekReport**

In `internal/svc/timesvc/week.go`, locate the return statement at the bottom of `GetWeekReport` and add the billable fields:

```go
	return domain.WeekReport{
		WeekRef:            ref,
		UserUID:             wire.TimeReportUid,
		TotalMinutes:        wire.MinutesTotal,
		MinutesBillable:     wire.MinutesBillable,
		MinutesNonBillable:  wire.MinutesNonBillable,
		Status:              decodeReportStatus(wire.Status),
		Days:                buildDaySummaries(ref, entries),
		Entries:             entries,
	}, nil
```

Run: `go test ./internal/svc/timesvc/ -run TestGetWeekReport_PopulatesBillableTotals -v`
Expected: PASS.

- [ ] **Step 3.3 — Failing tests for GetWeekReportForUser**

Create `internal/svc/timesvc/week_user_test.go`:

```go
package timesvc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestGetWeekReportForUser_HappyPath(t *testing.T) {
	var requestedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"PeriodStartDate": "2026-04-12T00:00:00Z",
			"PeriodEndDate":   "2026-04-18T00:00:00Z",
			"Status":          1,
			"Times":           [],
			"TimeReportUid":   "target-uid",
			"MinutesBillable": 240,
			"MinutesNonBillable": 0,
			"MinutesTotal":    240
		}`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	report, err := svc.GetWeekReportForUser(
		context.Background(), profile,
		time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ),
		"target-uid",
	)
	require.NoError(t, err)
	require.Equal(t, "/TDWebApi/api/time/report/2026-04-14/target-uid", requestedPath)
	require.Equal(t, "target-uid", report.UserUID)
	require.Equal(t, 240, report.MinutesBillable)
}

func TestGetWeekReportForUser_PermissionMappedFor401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`unauthorized`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	_, err := svc.GetWeekReportForUser(context.Background(), profile,
		time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ), "target-uid")
	require.Error(t, err)
	require.True(t, errors.Is(err, domain.ErrPermission), "401 should wrap ErrPermission")
}

func TestGetWeekReportForUser_PermissionMappedFor403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`forbidden`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	_, err := svc.GetWeekReportForUser(context.Background(), profile,
		time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ), "target-uid")
	require.Error(t, err)
	require.True(t, errors.Is(err, domain.ErrPermission), "403 should wrap ErrPermission")
}
```

Run: `go test ./internal/svc/timesvc/ -run TestGetWeekReportForUser_ -v`
Expected: FAIL — `GetWeekReportForUser` undefined.

- [ ] **Step 3.4 — Implement GetWeekReportForUser**

Create `internal/svc/timesvc/week_user.go`:

```go
package timesvc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

// GetWeekReportForUser fetches the week-of-`date` report for a specific
// user (uid). Mirrors GetWeekReport's decoding pipeline. TD 401/403
// responses are mapped to errors that wrap domain.ErrPermission so the
// CLI can distinguish "you can't see this user's report" from genuine
// failures.
func (s *Service) GetWeekReportForUser(ctx context.Context, profileName string, date time.Time, uid string) (domain.WeekReport, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return domain.WeekReport{}, err
	}

	day := date.In(domain.EasternTZ).Format("2006-01-02")
	path := fmt.Sprintf("/TDWebApi/api/time/report/%s/%s", day, uid)

	var wire wireTimeReport
	if err := client.DoJSON(ctx, "GET", path, nil, &wire); err != nil {
		if isPermissionErr(err) {
			return domain.WeekReport{}, fmt.Errorf("get week report for %s: %w", uid, domain.ErrPermission)
		}
		return domain.WeekReport{}, fmt.Errorf("get week report for %s: %w", uid, err)
	}

	periodStart := wire.PeriodStartDate.UTC()
	periodEnd := wire.PeriodEndDate.UTC()
	ref := domain.WeekRef{
		StartDate: time.Date(periodStart.Year(), periodStart.Month(), periodStart.Day(), 0, 0, 0, 0, domain.EasternTZ),
		EndDate:   time.Date(periodEnd.Year(), periodEnd.Month(), periodEnd.Day(), 0, 0, 0, 0, domain.EasternTZ),
	}

	entries := make([]domain.TimeEntry, 0, len(wire.Times))
	for _, t := range wire.Times {
		entry, err := decodeTimeEntry(t)
		if err != nil {
			return domain.WeekReport{}, err
		}
		entries = append(entries, entry)
	}
	if err := s.resolveTimeTypeNames(ctx, profileName, entries); err != nil {
		return domain.WeekReport{}, err
	}

	return domain.WeekReport{
		WeekRef:            ref,
		UserUID:            wire.TimeReportUid,
		TotalMinutes:       wire.MinutesTotal,
		MinutesBillable:    wire.MinutesBillable,
		MinutesNonBillable: wire.MinutesNonBillable,
		Status:             decodeReportStatus(wire.Status),
		Days:               buildDaySummaries(ref, entries),
		Entries:            entries,
	}, nil
}

// isPermissionErr matches the TD client's error format for 401/403. The
// client returns errors whose strings contain "401" or "403" alongside a
// status word; we look for the codes rather than the prose so a future
// formatter change doesn't silently break this match.
func isPermissionErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, domain.ErrPermission) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "401") || strings.Contains(msg, "403")
}
```

- [ ] **Step 3.5 — Run tests**

Run: `go test ./internal/svc/timesvc/ -run TestGetWeekReportForUser_ -v`
Expected: PASS for all three sub-tests.

- [ ] **Step 3.6 — Commit**

```bash
git add internal/svc/timesvc/week.go internal/svc/timesvc/week_test.go internal/svc/timesvc/week_user.go internal/svc/timesvc/week_user_test.go
git commit -m "feat(timesvc): GetWeekReportForUser + ErrPermission mapping + billable plumbing"
```

---

## Task 4: peoplesvc package

**Files:**
- Create: `internal/svc/peoplesvc/service.go`
- Create: `internal/svc/peoplesvc/types.go`
- Create: `internal/svc/peoplesvc/users.go`
- Create: `internal/svc/peoplesvc/users_test.go`

- [ ] **Step 4.1 — Create service.go**

Create `internal/svc/peoplesvc/service.go`:

```go
package peoplesvc

import (
	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/tdx"
)

// Service wraps the TD client for /api/people endpoints.
type Service struct {
	paths config.Paths
}

// New returns a Service rooted at paths.
func New(paths config.Paths) *Service {
	return &Service{paths: paths}
}

// clientFor returns an authenticated TD client for the named profile,
// using the same pattern as timesvc.Service.
func (s *Service) clientFor(profileName string) (*tdx.Client, error) {
	auth := authsvc.New(s.paths)
	resolved, err := auth.ResolveProfile(profileName)
	if err != nil {
		return nil, err
	}
	return auth.ClientFor(resolved)
}
```

If `auth.ClientFor` doesn't exist with that name, look at how `timesvc.Service.clientFor` works and mirror it. Quick search:

```bash
grep -nE "func .* clientFor|ClientFor" internal/svc/timesvc/service.go internal/svc/authsvc/*.go
```

Use whatever existing helper `timesvc` uses. (At time of writing, the pattern is `auth.NewClient(profile)` or similar — copy verbatim from timesvc/service.go.)

- [ ] **Step 4.2 — Create types.go**

Create `internal/svc/peoplesvc/types.go`:

```go
package peoplesvc

// wireUser matches GET /TDWebApi/api/people/{uid}.
type wireUser struct {
	UID                string `json:"UID"`
	ID                 int    `json:"ID"`
	FullName           string `json:"FullName"`
	PrimaryEmail       string `json:"PrimaryEmail"`
	AlternateEmail     string `json:"AlternateEmail"`
	IsActive           bool   `json:"IsActive"`
	DefaultAccountName string `json:"DefaultAccountName"`
	ReportsToUid       string `json:"ReportsToUid"`
	ReportsToId        int    `json:"ReportsToId"`
	ReportsToFullName  string `json:"ReportsToFullName"`
	ReportsToEmail     string `json:"ReportsToEmail"`
}

// wireUserSearch is the request body for POST /TDWebApi/api/people/search.
type wireUserSearch struct {
	NameLike   string `json:"NameLike,omitempty"`
	IsActive   *bool  `json:"IsActive,omitempty"`
	AccountIDs []int  `json:"AccountIDs,omitempty"`
	UserType   string `json:"UserType,omitempty"`
	MaxResults int    `json:"MaxResults,omitempty"`
}
```

- [ ] **Step 4.3 — Failing test for GetUser**

Create `internal/svc/peoplesvc/users_test.go`:

```go
package peoplesvc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/stretchr/testify/require"
)

// harness wires a fake TD server + a Service rooted at a temp config dir
// with a single profile pointed at the server.
func harness(t *testing.T, baseURL string) (*Service, string) {
	t.Helper()
	dir := t.TempDir()
	paths := config.Paths{
		Root:            dir,
		ConfigFile:      filepath.Join(dir, "config.yaml"),
		CredentialsFile: filepath.Join(dir, "credentials.yaml"),
	}
	auth := authsvc.New(paths)
	require.NoError(t, auth.Login(context.Background(), "ufl-test", baseURL, "fake-token"))
	return New(paths), "ufl-test"
}

func TestGetUser_Decodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/TDWebApi/api/people/target-uid", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"UID": "target-uid",
			"ID":  100,
			"FullName": "Iain Moffat",
			"PrimaryEmail": "ipm@ufl.edu",
			"IsActive": true,
			"DefaultAccountName": "UFIT",
			"ReportsToUid": "mgr-uid",
			"ReportsToId":  42,
			"ReportsToFullName": "Manager Name",
			"ReportsToEmail":   "mgr@ufl.edu"
		}`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	u, err := svc.GetUser(context.Background(), profile, "target-uid")
	require.NoError(t, err)
	require.Equal(t, domain.User{
		ID:             100,
		UID:            "target-uid",
		FullName:       "Iain Moffat",
		Email:          "ipm@ufl.edu",
		Active:         true,
		AccountName:    "UFIT",
		ReportsToUID:   "mgr-uid",
		ReportsToID:    42,
		ReportsToName:  "Manager Name",
		ReportsToEmail: "mgr@ufl.edu",
	}, u)
}

func TestGetUser_FallsBackToAlternateEmail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"UID": "target-uid",
			"FullName": "Test",
			"AlternateEmail": "alt@example.com"
		}`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	u, err := svc.GetUser(context.Background(), profile, "target-uid")
	require.NoError(t, err)
	require.Equal(t, "alt@example.com", u.Email)
}

func TestSearchUsers_DefaultsApplied(t *testing.T) {
	var receivedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/TDWebApi/api/people/search", r.URL.Path)
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		receivedBody = buf
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"UID":"u1","FullName":"User One","PrimaryEmail":"u1@x.com","IsActive":true}]`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	users, err := svc.SearchUsers(context.Background(), profile, domain.UserFilter{})
	require.NoError(t, err)
	require.Len(t, users, 1)
	require.Equal(t, "u1", users[0].UID)
	body := string(receivedBody)
	require.Contains(t, body, `"UserType":"User"`)
	require.Contains(t, body, `"IsActive":true`)
	require.Contains(t, body, `"MaxResults":100`)
}
```

NOTE: `harness` uses `auth.Login` to seed credentials. If that helper doesn't exist with that signature, look at how existing `timesvc` tests do it (e.g., `internal/svc/timesvc/entries_test.go`'s `harness`) and copy verbatim.

Run: `go test ./internal/svc/peoplesvc/ -v`
Expected: FAIL — service / methods not defined.

- [ ] **Step 4.4 — Implement users.go**

Create `internal/svc/peoplesvc/users.go`:

```go
package peoplesvc

import (
	"context"
	"fmt"

	"github.com/iainmoffat/tdx/internal/domain"
)

// GetUser fetches a single user by UID.
func (s *Service) GetUser(ctx context.Context, profileName, uid string) (domain.User, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return domain.User{}, err
	}
	var w wireUser
	path := fmt.Sprintf("/TDWebApi/api/people/%s", uid)
	if err := client.DoJSON(ctx, "GET", path, nil, &w); err != nil {
		return domain.User{}, fmt.Errorf("get user %s: %w", uid, err)
	}
	return decodeUser(w), nil
}

// SearchUsers calls POST /api/people/search with the given filter.
// Default UserType="User", IsActive=true, MaxResults=100.
func (s *Service) SearchUsers(ctx context.Context, profileName string, filter domain.UserFilter) ([]domain.User, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	req := wireUserSearch{
		NameLike:   filter.NameLike,
		UserType:   filter.UserType,
		MaxResults: filter.Limit,
	}
	if req.UserType == "" {
		req.UserType = "User"
	}
	if req.MaxResults == 0 {
		req.MaxResults = 100
	}
	if filter.Active == nil {
		t := true
		req.IsActive = &t
	} else {
		req.IsActive = filter.Active
	}
	if filter.AccountID > 0 {
		req.AccountIDs = []int{filter.AccountID}
	}

	var wire []wireUser
	if err := client.DoJSON(ctx, "POST", "/TDWebApi/api/people/search", req, &wire); err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}
	out := make([]domain.User, 0, len(wire))
	for _, w := range wire {
		u := decodeUser(w)
		// Optional client-side AccountName filter (TD's API doesn't
		// support filtering by account name directly).
		if filter.AccountName != "" && u.AccountName != filter.AccountName {
			continue
		}
		out = append(out, u)
	}
	return out, nil
}

// decodeUser maps a wireUser into domain.User. Email falls back to
// AlternateEmail when PrimaryEmail is empty.
func decodeUser(w wireUser) domain.User {
	email := w.PrimaryEmail
	if email == "" {
		email = w.AlternateEmail
	}
	return domain.User{
		ID:             w.ID,
		UID:            w.UID,
		FullName:       w.FullName,
		Email:          email,
		Active:         w.IsActive,
		AccountName:    w.DefaultAccountName,
		ReportsToUID:   w.ReportsToUid,
		ReportsToID:    w.ReportsToId,
		ReportsToName:  w.ReportsToFullName,
		ReportsToEmail: w.ReportsToEmail,
	}
}
```

- [ ] **Step 4.5 — Run tests**

Run: `go test ./internal/svc/peoplesvc/ -count=1 -v`
Expected: PASS for all three tests.

- [ ] **Step 4.6 — Commit**

```bash
git add internal/svc/peoplesvc/
git commit -m "feat(peoplesvc): GetUser + SearchUsers (POST /api/people/search)"
```

---

## Task 5: Add golang.org/x/sync + excelize deps

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 5.1 — Add deps**

Run:

```bash
go get golang.org/x/sync@latest github.com/xuri/excelize/v2@latest
```

- [ ] **Step 5.2 — Verify build**

Run: `go build ./...`
Expected: clean build.

- [ ] **Step 5.3 — Verify existing tests still pass**

Run: `go test ./... 2>&1 | grep -E "FAIL|ok\s" | tail -20`
Expected: every package green.

- [ ] **Step 5.4 — Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add golang.org/x/sync + xuri/excelize deps"
```

---

## Task 6: Retry-on-429 helper

**Files:**
- Modify: `internal/tdx/client.go`
- Modify: `internal/tdx/client_test.go`

- [ ] **Step 6.1 — Failing test for retry helper**

Append to `internal/tdx/client_test.go`:

```go
func TestClient_DoJSONWithRetry_RetriesOn429(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls < 2 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok": true}`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "tok")
	require.NoError(t, err)
	c.retryMaxJitterMillis = 1 // keep tests fast
	var out struct{ OK bool `json:"ok"` }
	err = c.DoJSONWithRetry(context.Background(), http.MethodGet, "/api/thing", nil, &out)
	require.NoError(t, err)
	require.True(t, out.OK)
	require.Equal(t, 2, calls)
}

func TestClient_DoJSONWithRetry_GivesUpAfterMaxRetries(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "tok")
	require.NoError(t, err)
	c.retryMaxJitterMillis = 1
	err = c.DoJSONWithRetry(context.Background(), http.MethodGet, "/api/thing", nil, nil)
	require.Error(t, err)
	require.Greater(t, calls, 1, "should have retried at least once")
}
```

Run: `go test ./internal/tdx/ -run TestClient_DoJSONWithRetry -v`
Expected: FAIL — `DoJSONWithRetry` undefined.

- [ ] **Step 6.2 — Implement retry helper**

In `internal/tdx/client.go`, add (likely near the existing `DoJSON`):

```go
// DoJSONWithRetry wraps DoJSON with bounded retries on HTTP 429.
// Honors the Retry-After header (seconds, default 5). Adds 0..retryMaxJitterMillis
// of jitter between attempts. Gives up after 3 retries.
func (c *Client) DoJSONWithRetry(ctx context.Context, method, path string, in, out any) error {
	const maxRetries = 3
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := c.DoJSON(ctx, method, path, in, out)
		if err == nil {
			return nil
		}
		lastErr = err
		if !isRateLimited(err) {
			return err
		}
		if attempt == maxRetries {
			break
		}
		wait := retryAfterFromError(err) + time.Duration(c.jitterMillis())*time.Millisecond
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return lastErr
}

// jitterMillis returns 0..retryMaxJitterMillis. Tests can override
// retryMaxJitterMillis for determinism.
func (c *Client) jitterMillis() int {
	max := c.retryMaxJitterMillis
	if max <= 0 {
		max = 2000
	}
	return rand.Intn(max + 1)
}

// retryAfterFromError returns the Retry-After duration encoded in an
// API error's message (the existing client format embeds the raw status
// + body); defaults to 5 seconds.
func retryAfterFromError(err error) time.Duration {
	// Conservative default. Real Retry-After parsing is best-effort
	// because the existing DoJSON doesn't surface response headers
	// today; we accept the worst case.
	return 5 * time.Second
}

func isRateLimited(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "429")
}
```

Add `"math/rand"` and `"strings"` to the imports if not already present, plus a `retryMaxJitterMillis int` field on `Client` (zero-value uses the 2000ms default).

NOTE: We can't read response headers with the current `DoJSON`-only abstraction without a small refactor. For now, accept the 5-second worst-case wait. (A future improvement could add a dedicated retry path that reads `Retry-After`.)

- [ ] **Step 6.3 — Run tests**

Run: `go test ./internal/tdx/ -run TestClient_DoJSONWithRetry -v`
Expected: PASS for both sub-tests.

- [ ] **Step 6.4 — Wire DoJSONWithRetry into GetWeekReportForUser**

In `internal/svc/timesvc/week_user.go`, change `client.DoJSON(...)` → `client.DoJSONWithRetry(...)`.

Run: `go test ./internal/svc/timesvc/ -count=1`
Expected: PASS.

- [ ] **Step 6.5 — Commit**

```bash
git add internal/tdx/client.go internal/tdx/client_test.go internal/svc/timesvc/week_user.go
git commit -m "feat(tdx): DoJSONWithRetry — bounded retries on 429"
```

---

## Task 7: CLI parent — `tdx time report`

**Files:**
- Create: `internal/cli/time/report/report.go`
- Modify: `internal/cli/time/time.go`

- [ ] **Step 7.1 — Create report.go**

Create `internal/cli/time/report/report.go`:

```go
// Package report provides the `tdx time report` command tree.
package report

import "github.com/spf13/cobra"

// NewCmd returns the `tdx time report` command tree.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate time-related reports",
	}
	cmd.AddCommand(newStatusCmd())
	return cmd
}
```

- [ ] **Step 7.2 — Wire into time.go**

In `internal/cli/time/time.go`, add the import and AddCommand:

```go
import (
	"github.com/iainmoffat/tdx/internal/cli/time/entry"
	"github.com/iainmoffat/tdx/internal/cli/time/report"
	"github.com/iainmoffat/tdx/internal/cli/time/template"
	"github.com/iainmoffat/tdx/internal/cli/time/timetype"
	"github.com/iainmoffat/tdx/internal/cli/time/week"
	"github.com/spf13/cobra"
)

// ... in NewCmd:
	cmd.AddCommand(entry.NewCmd())
	cmd.AddCommand(week.NewCmd())
	cmd.AddCommand(timetype.NewCmd())
	cmd.AddCommand(template.NewCmd())
	cmd.AddCommand(report.NewCmd())
```

- [ ] **Step 7.3 — Stub status command**

In `internal/cli/time/report/report.go`, add a temporary stub so the package compiles before Task 8 fills it in:

```go
func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Weekly time-status report (per user, per week)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cobra.ShowHelp(cmd)
		},
	}
}
```

- [ ] **Step 7.4 — Verify build**

Run: `go build ./... && /tmp/tdx-build/tdx time report --help 2>&1 | head -10` (build to a temp path so existing /tmp/tdx isn't clobbered):

```bash
go build -o /tmp/tdx-build/tdx ./cmd/tdx && /tmp/tdx-build/tdx time report --help
```

Expected: shows `report` parent with `status` subcommand.

- [ ] **Step 7.5 — Commit**

```bash
git add internal/cli/time/report/report.go internal/cli/time/time.go
git commit -m "feat(cli): tdx time report parent + status stub"
```

---

## Task 8: Status command flags + selector validation

**Files:**
- Create: `internal/cli/time/report/status.go`
- Create: `internal/cli/time/report/status_test.go`

- [ ] **Step 8.1 — Failing tests for selector + format validation**

Create `internal/cli/time/report/status_test.go`:

```go
package report

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func runStatusCmd(t *testing.T, args ...string) (stdout string, err error) {
	t.Helper()
	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), err
}

func TestStatus_SelectorRequired(t *testing.T) {
	_, err := runStatusCmd(t, "--week", "2026-04-12")
	require.Error(t, err)
	require.Contains(t, err.Error(), "selector")
}

func TestStatus_SelectorMutuallyExclusive(t *testing.T) {
	_, err := runStatusCmd(t,
		"--week", "2026-04-12",
		"--user", "u1",
		"--manager", "me",
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exactly one")
}

func TestStatus_FormatMutuallyExclusive(t *testing.T) {
	_, err := runStatusCmd(t,
		"--week", "2026-04-12",
		"--user", "u1",
		"--json", "--csv",
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "format")
}

func TestStatus_AllRequiresYes(t *testing.T) {
	_, err := runStatusCmd(t,
		"--week", "2026-04-12",
		"--all",
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "--yes")
}

func TestStatus_FlagsRegistered(t *testing.T) {
	cmd := newStatusCmd()
	for _, f := range []string{"week", "from", "to", "user", "manager", "account", "all", "yes", "include-zero", "limit", "json", "csv", "xlsx", "profile"} {
		require.NotNil(t, cmd.Flags().Lookup(f), "missing flag: %s", f)
	}
}
```

Run: `go test ./internal/cli/time/report/ -run TestStatus -v`
Expected: FAIL — flags not yet registered, validation not implemented.

- [ ] **Step 8.2 — Implement status.go (validation + flag wiring; runner stub for now)**

Create `internal/cli/time/report/status.go`:

```go
package report

import (
	"fmt"

	"github.com/spf13/cobra"
)

type statusFlags struct {
	profile     string
	week        string
	from        string
	to          string
	users       []string
	manager     string
	account     string
	all         bool
	yes         bool
	includeZero bool
	limit       int
	json        bool
	csv         bool
	xlsx        string
}

func newStatusCmd() *cobra.Command {
	var f statusFlags
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Weekly time-status report (per user, per week)",
		Long: `Render TeamDynamix's "Time Status Report" (Work Management → Analysis → Standard Reports).

For each (user, week) pair, prints submission status and billable / non-billable / total hours.

Selectors (exactly one required):
  --user UID    one or more user UIDs (repeatable / comma-separated)
  --manager UID limit to direct reports (use "me" for the authenticated user)
  --account NAME limit to users in this account/department by name
  --all         every active standard user (requires --yes)

Output formats (mutually exclusive; default: human table):
  --json       JSON envelope on stdout
  --csv        CSV on stdout (no subtotal rows; pivot in Excel)
  --xlsx PATH  write XLSX to PATH`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateStatusFlags(f); err != nil {
				return err
			}
			return runStatus(cmd, f)
		},
	}
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name (default: active)")
	cmd.Flags().StringVar(&f.week, "week", "", "any date inside the target week (YYYY-MM-DD)")
	cmd.Flags().StringVar(&f.from, "from", "", "range start (YYYY-MM-DD); requires --to")
	cmd.Flags().StringVar(&f.to, "to", "", "range end (YYYY-MM-DD); requires --from")
	cmd.Flags().StringSliceVar(&f.users, "user", nil, "user UIDs (repeatable / comma-separated)")
	cmd.Flags().StringVar(&f.manager, "manager", "", "limit to direct reports of this UID; \"me\" = authenticated user")
	cmd.Flags().StringVar(&f.account, "account", "", "limit to users in this account/department by name")
	cmd.Flags().BoolVar(&f.all, "all", false, "every active standard user (requires --yes)")
	cmd.Flags().BoolVar(&f.yes, "yes", false, "confirm --all")
	cmd.Flags().BoolVar(&f.includeZero, "include-zero", true, "include user-weeks with zero total minutes (default: include)")
	cmd.Flags().IntVar(&f.limit, "limit", 200, "cap user count (hard cap: 1000)")
	cmd.Flags().BoolVar(&f.json, "json", false, "emit JSON to stdout")
	cmd.Flags().BoolVar(&f.csv, "csv", false, "emit CSV to stdout")
	cmd.Flags().StringVar(&f.xlsx, "xlsx", "", "write XLSX to this file path")
	return cmd
}

// validateStatusFlags enforces selector + format exclusivity rules
// before any API calls happen.
func validateStatusFlags(f statusFlags) error {
	// Selectors
	selectors := 0
	if len(f.users) > 0 {
		selectors++
	}
	if f.manager != "" {
		selectors++
	}
	if f.account != "" {
		selectors++
	}
	if f.all {
		selectors++
	}
	switch {
	case selectors == 0:
		return fmt.Errorf("a selector is required: --user, --manager, --account, or --all")
	case selectors > 1:
		return fmt.Errorf("exactly one of --user, --manager, --account, --all may be set")
	}

	if f.all && !f.yes {
		return fmt.Errorf("--all is destructively large; pass --yes to confirm")
	}

	// Formats
	formats := 0
	if f.json {
		formats++
	}
	if f.csv {
		formats++
	}
	if f.xlsx != "" {
		formats++
	}
	if formats > 1 {
		return fmt.Errorf("only one output format flag may be set (--json, --csv, --xlsx)")
	}

	// Limit cap
	if f.limit > 1000 {
		return fmt.Errorf("--limit cannot exceed 1000")
	}

	// Date range
	if f.week != "" && (f.from != "" || f.to != "") {
		return fmt.Errorf("--week is mutually exclusive with --from/--to")
	}
	if (f.from != "") != (f.to != "") {
		return fmt.Errorf("--from and --to must be given together")
	}
	if f.week == "" && f.from == "" {
		return fmt.Errorf("--week or --from/--to is required")
	}

	return nil
}

// runStatus is implemented in runner.go (Task 9). Stub here for compilation.
func runStatus(cmd *cobra.Command, f statusFlags) error {
	return fmt.Errorf("not yet implemented")
}
```

- [ ] **Step 8.3 — Run validation tests**

Run: `go test ./internal/cli/time/report/ -run TestStatus -v`
Expected: PASS for all 5 sub-tests.

- [ ] **Step 8.4 — Commit**

```bash
git add internal/cli/time/report/status.go internal/cli/time/report/status_test.go
git commit -m "feat(cli): tdx time report status — flag wiring + selector/format validation"
```

---

## Task 9: Status runner (orchestration with errgroup)

**Files:**
- Create: `internal/cli/time/report/runner.go`
- Create: `internal/cli/time/report/runner_test.go`

- [ ] **Step 9.1 — Define mockable interfaces in runner.go (without runner logic yet)**

Create `internal/cli/time/report/runner.go` with just the interfaces:

```go
package report

import (
	"context"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

// timesvcAPI is the subset of timesvc.Service the runner needs.
type timesvcAPI interface {
	GetWeekReportForUser(ctx context.Context, profile string, date time.Time, uid string) (domain.WeekReport, error)
}

// peoplesvcAPI is the subset of peoplesvc.Service the runner needs.
type peoplesvcAPI interface {
	GetUser(ctx context.Context, profile, uid string) (domain.User, error)
	SearchUsers(ctx context.Context, profile string, filter domain.UserFilter) ([]domain.User, error)
}

// authsvcAPI is the subset of authsvc.Service the runner needs.
type authsvcAPI interface {
	WhoAmI(ctx context.Context, profile string) (domain.User, error)
}
```

- [ ] **Step 9.2 — Failing test for runner**

Create `internal/cli/time/report/runner_test.go`:

```go
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
	users  map[string]domain.User
	search []domain.User
}

func (m *mockPeoplesvc) GetUser(_ context.Context, _, uid string) (domain.User, error) {
	if u, ok := m.users[uid]; ok {
		return u, nil
	}
	return domain.User{}, errors.New("not found")
}

func (m *mockPeoplesvc) SearchUsers(_ context.Context, _ string, _ domain.UserFilter) ([]domain.User, error) {
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
		users: []string{"u1"},
		week:  "2026-04-14",
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
		users: []string{"u1"},
		week:  "2026-04-14",
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

	deps := runnerDeps{
		Time:   &mockTimesvc{reports: map[string]domain.WeekReport{"u1": report}},
		People: &mockPeoplesvc{search: []domain.User{directReport, other}},
		Auth:   &mockAuthsvc{me: me},
	}
	out, err := assembleReport(context.Background(), deps, statusFlags{
		manager: "me",
		week:    "2026-04-14",
	})
	require.NoError(t, err)
	require.Len(t, out.Rows, 1, "only direct report's row is included")
	require.Equal(t, "u1", out.Rows[0].User.UID)
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
		users: []string{"u1"},
		from:  "2026-04-14", to: "2026-04-22",
	})
	require.NoError(t, err)
	require.Len(t, out.Rows, 2, "2 weeks × 1 user = 2 rows")
	require.Equal(t, week1.StartDate, out.Rows[0].WeekRef.StartDate)
	require.Equal(t, week2.StartDate, out.Rows[1].WeekRef.StartDate)
}
```

Run: `go test ./internal/cli/time/report/ -run TestRunner -v`
Expected: FAIL — runner not implemented.

- [ ] **Step 9.3 — Implement assembleReport in runner.go**

Append to `internal/cli/time/report/runner.go`:

```go
import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/iainmoffat/tdx/internal/domain"
	"golang.org/x/sync/errgroup"
)

// runnerDeps bundles the service dependencies for assembleReport so tests
// can inject mocks via the interfaces above.
type runnerDeps struct {
	Time    timesvcAPI
	People  peoplesvcAPI
	Auth    authsvcAPI
	Profile string
}

const (
	maxConcurrency = 5
	hardLimit      = 1000
)

// assembleReport orchestrates the per-(user, week) fan-out and returns
// the assembled TimeStatusReport. Permission errors become rows with
// Status="permission-denied" and zero hours; any other error fails the run.
func assembleReport(ctx context.Context, deps runnerDeps, f statusFlags) (domain.TimeStatusReport, error) {
	weeks, err := resolveWeeks(f)
	if err != nil {
		return domain.TimeStatusReport{}, err
	}

	users, err := resolveUsers(ctx, deps, f)
	if err != nil {
		return domain.TimeStatusReport{}, err
	}

	// Apply --limit cap.
	cap := f.limit
	if cap <= 0 || cap > hardLimit {
		cap = hardLimit
	}
	if len(users) > cap {
		users = users[:cap]
	}

	// Index users by UID for quick lookup.
	userByUID := make(map[string]domain.User, len(users))
	for _, u := range users {
		userByUID[u.UID] = u
	}

	// Fan out per-(user, week).
	type result struct {
		row domain.WeekStatusRow
	}
	resultsMu := sync.Mutex{}
	results := make([]domain.WeekStatusRow, 0, len(users)*len(weeks))

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrency)

	for _, week := range weeks {
		week := week
		for _, u := range users {
			u := u
			g.Go(func() error {
				rep, err := deps.Time.GetWeekReportForUser(gctx, deps.Profile, week.StartDate, u.UID)
				row := domain.WeekStatusRow{
					WeekRef: week,
					User:    u,
				}
				switch {
				case err == nil:
					row.Status = rep.Status
					row.BillableMin = rep.MinutesBillable
					row.NonBillableMin = rep.MinutesNonBillable
					row.TotalMin = rep.TotalMinutes
				case errors.Is(err, domain.ErrPermission):
					row.Status = domain.ReportStatus("permission-denied")
				default:
					return fmt.Errorf("get report for %s/%s: %w", u.UID, week.StartDate.Format("2006-01-02"), err)
				}
				resultsMu.Lock()
				results = append(results, row)
				resultsMu.Unlock()
				return nil
			})
		}
	}

	if err := g.Wait(); err != nil {
		return domain.TimeStatusReport{}, err
	}

	// Apply --include-zero filter.
	if !f.includeZero {
		filtered := results[:0]
		for _, r := range results {
			if r.TotalMin > 0 {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	// Sort: by week ascending, then by user FullName.
	sort.SliceStable(results, func(i, j int) bool {
		if !results[i].WeekRef.StartDate.Equal(results[j].WeekRef.StartDate) {
			return results[i].WeekRef.StartDate.Before(results[j].WeekRef.StartDate)
		}
		return results[i].User.FullName < results[j].User.FullName
	})

	first, last := weeks[0], weeks[len(weeks)-1]
	return domain.TimeStatusReport{From: first, To: last, Rows: results}, nil
}

// resolveWeeks converts statusFlags' --week / --from/--to into a list of
// WeekRefs (Sunday→Saturday in EasternTZ).
func resolveWeeks(f statusFlags) ([]domain.WeekRef, error) {
	if f.week != "" {
		t, err := time.ParseInLocation("2006-01-02", f.week, domain.EasternTZ)
		if err != nil {
			return nil, fmt.Errorf("invalid --week: %w", err)
		}
		return []domain.WeekRef{domain.WeekRefContaining(t)}, nil
	}
	from, err := time.ParseInLocation("2006-01-02", f.from, domain.EasternTZ)
	if err != nil {
		return nil, fmt.Errorf("invalid --from: %w", err)
	}
	to, err := time.ParseInLocation("2006-01-02", f.to, domain.EasternTZ)
	if err != nil {
		return nil, fmt.Errorf("invalid --to: %w", err)
	}
	if to.Before(from) {
		return nil, fmt.Errorf("--to (%s) before --from (%s)", to.Format("2006-01-02"), from.Format("2006-01-02"))
	}
	startWeek := domain.WeekRefContaining(from)
	endWeek := domain.WeekRefContaining(to)
	weeks := []domain.WeekRef{}
	cur := startWeek
	for !cur.StartDate.After(endWeek.StartDate) {
		weeks = append(weeks, cur)
		cur = domain.WeekRefContaining(cur.StartDate.AddDate(0, 0, 7))
	}
	return weeks, nil
}

// resolveUsers maps the selector flags to a concrete user list.
// Pre-validated: exactly one of --user/--manager/--account/--all is set.
func resolveUsers(ctx context.Context, deps runnerDeps, f statusFlags) ([]domain.User, error) {
	switch {
	case len(f.users) > 0:
		out := make([]domain.User, 0, len(f.users))
		for _, uid := range f.users {
			u, err := deps.People.GetUser(ctx, deps.Profile, uid)
			if err != nil {
				return nil, fmt.Errorf("get user %s: %w", uid, err)
			}
			out = append(out, u)
		}
		return out, nil

	case f.manager != "":
		mgrUID := f.manager
		if mgrUID == "me" {
			me, err := deps.Auth.WhoAmI(ctx, deps.Profile)
			if err != nil {
				return nil, fmt.Errorf("whoami: %w", err)
			}
			mgrUID = me.UID
		}
		all, err := deps.People.SearchUsers(ctx, deps.Profile, domain.UserFilter{Limit: hardLimit})
		if err != nil {
			return nil, err
		}
		out := []domain.User{}
		for _, u := range all {
			if u.ReportsToUID == mgrUID {
				out = append(out, u)
			}
		}
		return out, nil

	case f.account != "":
		return deps.People.SearchUsers(ctx, deps.Profile, domain.UserFilter{
			AccountName: f.account,
			Limit:       hardLimit,
		})

	case f.all:
		return deps.People.SearchUsers(ctx, deps.Profile, domain.UserFilter{Limit: hardLimit})
	}
	return nil, fmt.Errorf("no selector (validation should have caught this)")
}
```

Add `"context"` and `"time"` to imports if not already present.

- [ ] **Step 9.4 — Run runner tests**

Run: `go test ./internal/cli/time/report/ -run TestRunner -v`
Expected: PASS for all 4 sub-tests.

- [ ] **Step 9.5 — Commit**

```bash
git add internal/cli/time/report/runner.go internal/cli/time/report/runner_test.go
git commit -m "feat(cli): report runner — errgroup fan-out + selector resolution"
```

---

## Task 10: Text + JSON output

**Files:**
- Create: `internal/cli/time/report/print.go`
- Create: `internal/cli/time/report/print_test.go`

- [ ] **Step 10.1 — Failing tests**

Create `internal/cli/time/report/print_test.go`:

```go
package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func sampleReport() domain.TimeStatusReport {
	week := domain.WeekRefContaining(time.Date(2026, 4, 14, 0, 0, 0, 0, domain.EasternTZ))
	return domain.TimeStatusReport{
		From: week, To: week,
		Rows: []domain.WeekStatusRow{
			{
				WeekRef: week,
				User: domain.User{UID: "u1", FullName: "Alice", Email: "a@x", ReportsToName: "Mgr", ReportsToEmail: "m@x"},
				Status: domain.ReportSubmitted,
				BillableMin: 240, NonBillableMin: 60, TotalMin: 300,
			},
		},
	}
}

func TestPrintText_HumanReadable(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, printText(&buf, sampleReport(), statusFlags{users: []string{"u1"}, week: "2026-04-14"}))
	out := buf.String()
	require.Contains(t, out, "WEEK 2026-04-12")
	require.Contains(t, out, "Alice")
	require.Contains(t, out, "submitted")
	require.Contains(t, out, "4.00")  // billable hours
	require.Contains(t, out, "5.00")  // total hours
}

func TestPrintJSON_Schema(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, printJSON(&buf, sampleReport(), statusFlags{users: []string{"u1"}, week: "2026-04-14"}))
	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.Equal(t, "tdx.v1.timeStatusReport", got["schema"])
	weeks := got["weeks"].([]any)
	require.Len(t, weeks, 1)
	w := weeks[0].(map[string]any)
	require.Equal(t, "2026-04-12", w["weekStart"])
	rows := w["rows"].([]any)
	require.Len(t, rows, 1)
	row := rows[0].(map[string]any)
	require.Equal(t, "u1", row["userUID"])
	require.InDelta(t, 4.0, row["billableHours"], 0.001)
	require.InDelta(t, 5.0, row["totalHours"], 0.001)
}

func TestPrintJSON_FilterEcho(t *testing.T) {
	var buf bytes.Buffer
	f := statusFlags{users: []string{"u1", "u2"}, week: "2026-04-14"}
	require.NoError(t, printJSON(&buf, sampleReport(), f))
	out := buf.String()
	require.True(t, strings.Contains(out, `"selector":"user"`))
	require.True(t, strings.Contains(out, `"u1"`))
	require.True(t, strings.Contains(out, `"u2"`))
}
```

Run: `go test ./internal/cli/time/report/ -run TestPrint -v`
Expected: FAIL — printText/printJSON undefined.

- [ ] **Step 10.2 — Implement print.go**

Create `internal/cli/time/report/print.go`:

```go
package report

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/render"
)

// printText renders a human-readable Time Status Report.
func printText(w io.Writer, rep domain.TimeStatusReport, f statusFlags) error {
	groups := rep.RowsByWeek()
	for _, g := range groups {
		fmt.Fprintf(w, "WEEK %s — %s\n",
			g.Week.StartDate.Format("2006-01-02"),
			g.Week.EndDate.Format("2006-01-02"))

		headers := []string{"NAME", "EMAIL", "REPORTS TO", "STATUS", "BILL", "NON-BILL", "TOTAL"}
		rows := make([][]string, 0, len(g.Rows))
		var billSum, nonBillSum, totalSum int
		for _, r := range g.Rows {
			rows = append(rows, []string{
				r.User.FullName,
				r.User.Email,
				r.User.ReportsToName,
				string(r.Status),
				fmt.Sprintf("%.2f", r.BillableHours()),
				fmt.Sprintf("%.2f", r.NonBillableHours()),
				fmt.Sprintf("%.2f", r.TotalHours()),
			})
			billSum += r.BillableMin
			nonBillSum += r.NonBillableMin
			totalSum += r.TotalMin
		}
		summary := []string{"TOTAL", "", "", "",
			fmt.Sprintf("%.2f", float64(billSum)/60.0),
			fmt.Sprintf("%.2f", float64(nonBillSum)/60.0),
			fmt.Sprintf("%.2f", float64(totalSum)/60.0),
		}
		render.Table(w, headers, rows, summary)
		fmt.Fprintln(w)
	}
	bill, nonBill, total := rep.Totals()
	fmt.Fprintf(w, "OVERALL: %.2f bill · %.2f non-bill · %.2f total\n",
		float64(bill)/60.0, float64(nonBill)/60.0, float64(total)/60.0)
	return nil
}

// printJSON emits the tdx.v1.timeStatusReport envelope.
func printJSON(w io.Writer, rep domain.TimeStatusReport, f statusFlags) error {
	type filterJSON struct {
		Selector string   `json:"selector"`
		Users    []string `json:"users,omitempty"`
		Manager  string   `json:"manager,omitempty"`
		Account  string   `json:"account,omitempty"`
		From     string   `json:"from"`
		To       string   `json:"to"`
	}
	type rowJSON struct {
		UserUID          string  `json:"userUID"`
		Name             string  `json:"name"`
		Email            string  `json:"email"`
		ReportsToName    string  `json:"reportsToName,omitempty"`
		ReportsToEmail   string  `json:"reportsToEmail,omitempty"`
		Status           string  `json:"status"`
		BillableHours    float64 `json:"billableHours"`
		NonBillableHours float64 `json:"nonBillableHours"`
		TotalHours       float64 `json:"totalHours"`
	}
	type totalsJSON struct {
		BillableHours    float64 `json:"billableHours"`
		NonBillableHours float64 `json:"nonBillableHours"`
		TotalHours       float64 `json:"totalHours"`
	}
	type weekJSON struct {
		WeekStart string     `json:"weekStart"`
		WeekEnd   string     `json:"weekEnd"`
		Rows      []rowJSON  `json:"rows"`
		Subtotals totalsJSON `json:"subtotals"`
	}

	selector := selectorOf(f)
	filter := filterJSON{
		Selector: selector,
		From:     rep.From.StartDate.Format("2006-01-02"),
		To:       rep.To.EndDate.Format("2006-01-02"),
	}
	switch selector {
	case "user":
		filter.Users = f.users
	case "manager":
		filter.Manager = f.manager
	case "account":
		filter.Account = f.account
	}

	weeks := []weekJSON{}
	for _, g := range rep.RowsByWeek() {
		var bill, nonBill, total int
		rows := make([]rowJSON, 0, len(g.Rows))
		for _, r := range g.Rows {
			rows = append(rows, rowJSON{
				UserUID: r.User.UID, Name: r.User.FullName, Email: r.User.Email,
				ReportsToName: r.User.ReportsToName, ReportsToEmail: r.User.ReportsToEmail,
				Status: string(r.Status),
				BillableHours: r.BillableHours(), NonBillableHours: r.NonBillableHours(), TotalHours: r.TotalHours(),
			})
			bill += r.BillableMin
			nonBill += r.NonBillableMin
			total += r.TotalMin
		}
		weeks = append(weeks, weekJSON{
			WeekStart: g.Week.StartDate.Format("2006-01-02"),
			WeekEnd:   g.Week.EndDate.Format("2006-01-02"),
			Rows:      rows,
			Subtotals: totalsJSON{
				BillableHours: float64(bill) / 60.0, NonBillableHours: float64(nonBill) / 60.0, TotalHours: float64(total) / 60.0,
			},
		})
	}

	bill, nonBill, total := rep.Totals()
	envelope := struct {
		Schema string     `json:"schema"`
		Filter filterJSON `json:"filter"`
		Weeks  []weekJSON `json:"weeks"`
		Totals totalsJSON `json:"totals"`
	}{
		Schema: "tdx.v1.timeStatusReport",
		Filter: filter,
		Weeks:  weeks,
		Totals: totalsJSON{
			BillableHours: float64(bill) / 60.0, NonBillableHours: float64(nonBill) / 60.0, TotalHours: float64(total) / 60.0,
		},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}

func selectorOf(f statusFlags) string {
	switch {
	case len(f.users) > 0:
		return "user"
	case f.manager != "":
		return "manager"
	case f.account != "":
		return "account"
	case f.all:
		return "all"
	}
	return ""
}
```

- [ ] **Step 10.3 — Run print tests**

Run: `go test ./internal/cli/time/report/ -run TestPrint -v`
Expected: PASS.

- [ ] **Step 10.4 — Commit**

```bash
git add internal/cli/time/report/print.go internal/cli/time/report/print_test.go
git commit -m "feat(cli): report — text + JSON renderers"
```

---

## Task 11: CSV output

**Files:**
- Create: `internal/cli/time/report/csv.go`
- Create: `internal/cli/time/report/csv_test.go`

- [ ] **Step 11.1 — Failing test**

Create `internal/cli/time/report/csv_test.go`:

```go
package report

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteCSV_Headers(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, writeCSV(&buf, sampleReport()))
	r := csv.NewReader(strings.NewReader(buf.String()))
	rows, err := r.ReadAll()
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	require.Equal(t,
		[]string{"weekStart", "weekEnd", "userUID", "name", "email", "reportsToName", "reportsToEmail", "status", "billableHours", "nonBillableHours", "totalHours"},
		rows[0])
}

func TestWriteCSV_DataRow(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, writeCSV(&buf, sampleReport()))
	r := csv.NewReader(strings.NewReader(buf.String()))
	rows, err := r.ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 2, "header + 1 data row")
	require.Equal(t, "2026-04-12", rows[1][0])
	require.Equal(t, "u1", rows[1][2])
	require.Equal(t, "Alice", rows[1][3])
	require.Equal(t, "submitted", rows[1][7])
	require.Equal(t, "4.00", rows[1][8])
	require.Equal(t, "5.00", rows[1][10])
}
```

(`sampleReport` is defined in print_test.go in the same package, so it's available.)

Run: `go test ./internal/cli/time/report/ -run TestWriteCSV -v`
Expected: FAIL.

- [ ] **Step 11.2 — Implement csv.go**

Create `internal/cli/time/report/csv.go`:

```go
package report

import (
	"encoding/csv"
	"fmt"
	"io"

	"github.com/iainmoffat/tdx/internal/domain"
)

// writeCSV emits a flat CSV (one row per WeekStatusRow). No subtotal
// rows — Excel pivots/SUMIF do the per-week math better than fixed rows.
func writeCSV(w io.Writer, rep domain.TimeStatusReport) error {
	cw := csv.NewWriter(w)
	header := []string{
		"weekStart", "weekEnd", "userUID", "name", "email",
		"reportsToName", "reportsToEmail", "status",
		"billableHours", "nonBillableHours", "totalHours",
	}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, r := range rep.Rows {
		if err := cw.Write([]string{
			r.WeekRef.StartDate.Format("2006-01-02"),
			r.WeekRef.EndDate.Format("2006-01-02"),
			r.User.UID,
			r.User.FullName,
			r.User.Email,
			r.User.ReportsToName,
			r.User.ReportsToEmail,
			string(r.Status),
			fmt.Sprintf("%.2f", r.BillableHours()),
			fmt.Sprintf("%.2f", r.NonBillableHours()),
			fmt.Sprintf("%.2f", r.TotalHours()),
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
```

- [ ] **Step 11.3 — Run tests**

Run: `go test ./internal/cli/time/report/ -run TestWriteCSV -v`
Expected: PASS.

- [ ] **Step 11.4 — Commit**

```bash
git add internal/cli/time/report/csv.go internal/cli/time/report/csv_test.go
git commit -m "feat(cli): report --csv (flat, no subtotal rows)"
```

---

## Task 12: XLSX output

**Files:**
- Create: `internal/cli/time/report/xlsx.go`
- Create: `internal/cli/time/report/xlsx_test.go`

- [ ] **Step 12.1 — Failing test**

Create `internal/cli/time/report/xlsx_test.go`:

```go
package report

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestWriteXLSX_HeaderAndDataRow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.xlsx")
	require.NoError(t, writeXLSX(path, sampleReport()))

	f, err := excelize.OpenFile(path)
	require.NoError(t, err)
	defer f.Close()

	const sheet = "Time Status Report"
	rows, err := f.GetRows(sheet)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(rows), 2, "header + at least one data row")

	header := rows[0]
	require.Equal(t, "weekStart", header[0])
	require.Equal(t, "totalHours", header[len(header)-1])

	data := rows[1]
	require.Equal(t, "2026-04-12", data[0])
	require.Equal(t, "u1", data[2])
	require.Equal(t, "Alice", data[3])

	// Hours columns are numeric (not strings).
	billCell, err := f.GetCellValue(sheet, "I2")
	require.NoError(t, err)
	require.Equal(t, "4", billCell)  // excelize returns "4" for 4.0 numeric (no trailing zeros)
}

func TestWriteXLSX_HeaderIsBold(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.xlsx")
	require.NoError(t, writeXLSX(path, sampleReport()))

	f, err := excelize.OpenFile(path)
	require.NoError(t, err)
	defer f.Close()

	styleID, err := f.GetCellStyle("Time Status Report", "A1")
	require.NoError(t, err)
	require.NotZero(t, styleID, "header cells must have a non-default style (bold)")
}
```

Run: `go test ./internal/cli/time/report/ -run TestWriteXLSX -v`
Expected: FAIL — `writeXLSX` undefined.

- [ ] **Step 12.2 — Implement xlsx.go**

Create `internal/cli/time/report/xlsx.go`:

```go
package report

import (
	"fmt"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/xuri/excelize/v2"
)

// writeXLSX writes a TimeStatusReport to path as an .xlsx file.
// Single sheet "Time Status Report" with bold frozen header row.
// Hours are stored as numbers (not strings) so Excel can pivot/sum.
func writeXLSX(path string, rep domain.TimeStatusReport) error {
	f := excelize.NewFile()
	defer f.Close()

	const sheet = "Time Status Report"
	idx, err := f.NewSheet(sheet)
	if err != nil {
		return fmt.Errorf("new sheet: %w", err)
	}
	f.SetActiveSheet(idx)
	if err := f.DeleteSheet("Sheet1"); err != nil {
		// Non-fatal: leave the default sheet if delete fails.
		_ = err
	}

	headers := []string{
		"weekStart", "weekEnd", "userUID", "name", "email",
		"reportsToName", "reportsToEmail", "status",
		"billableHours", "nonBillableHours", "totalHours",
	}

	// Bold style for header.
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	})
	if err != nil {
		return fmt.Errorf("header style: %w", err)
	}

	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return err
		}
		if err := f.SetCellStyle(sheet, cell, cell, headerStyle); err != nil {
			return err
		}
	}

	for ri, r := range rep.Rows {
		row := ri + 2 // 1-indexed; header takes row 1
		stringVals := []any{
			r.WeekRef.StartDate.Format("2006-01-02"),
			r.WeekRef.EndDate.Format("2006-01-02"),
			r.User.UID,
			r.User.FullName,
			r.User.Email,
			r.User.ReportsToName,
			r.User.ReportsToEmail,
			string(r.Status),
		}
		for i, v := range stringVals {
			cell, _ := excelize.CoordinatesToCellName(i+1, row)
			if err := f.SetCellValue(sheet, cell, v); err != nil {
				return err
			}
		}
		// Numeric columns
		numericVals := []float64{r.BillableHours(), r.NonBillableHours(), r.TotalHours()}
		for i, v := range numericVals {
			cell, _ := excelize.CoordinatesToCellName(len(stringVals)+1+i, row)
			if err := f.SetCellValue(sheet, cell, v); err != nil {
				return err
			}
		}
	}

	// Sensible static column widths.
	widths := map[string]float64{
		"A": 12, "B": 12, "C": 16, "D": 24, "E": 28, "F": 24, "G": 28,
		"H": 14, "I": 12, "J": 14, "K": 12,
	}
	for col, w := range widths {
		_ = f.SetColWidth(sheet, col, col, w)
	}

	// Freeze the top row.
	_ = f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})

	return f.SaveAs(path)
}
```

- [ ] **Step 12.3 — Run tests**

Run: `go test ./internal/cli/time/report/ -run TestWriteXLSX -v`
Expected: PASS.

- [ ] **Step 12.4 — Commit**

```bash
git add internal/cli/time/report/xlsx.go internal/cli/time/report/xlsx_test.go
git commit -m "feat(cli): report --xlsx (excelize, single sheet, bold header, frozen row)"
```

---

## Task 13: Wire runStatus into status.go

**Files:**
- Modify: `internal/cli/time/report/status.go`

- [ ] **Step 13.1 — Replace runStatus stub with the real implementation**

In `internal/cli/time/report/status.go`, replace the stub `runStatus` with:

```go
func runStatus(cmd *cobra.Command, f statusFlags) error {
	paths, err := config.ResolvePaths()
	if err != nil {
		return err
	}
	auth := authsvc.New(paths)
	profile, err := auth.ResolveProfile(f.profile)
	if err != nil {
		return err
	}
	tsvc := timesvc.New(paths)
	psvc := peoplesvc.New(paths)

	deps := runnerDeps{
		Time:    tsvc,
		People:  psvc,
		Auth:    auth,
		Profile: profile,
	}

	rep, err := assembleReport(cmd.Context(), deps, f)
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	switch {
	case f.json:
		return printJSON(w, rep, f)
	case f.csv:
		return writeCSV(w, rep)
	case f.xlsx != "":
		return writeXLSX(f.xlsx, rep)
	default:
		return printText(w, rep, f)
	}
}
```

Add the imports:

```go
"github.com/iainmoffat/tdx/internal/config"
"github.com/iainmoffat/tdx/internal/svc/authsvc"
"github.com/iainmoffat/tdx/internal/svc/peoplesvc"
"github.com/iainmoffat/tdx/internal/svc/timesvc"
```

If `authsvc.Service` doesn't satisfy the `authsvcAPI` interface (e.g. method shape is `(profile string)` not `(ctx, profile)`), define a small wrapper struct in `runner.go` that adapts it. Verify by running `go build`.

- [ ] **Step 13.2 — Verify build**

Run: `go build ./...`
Expected: clean. If type mismatches, adjust the interfaces or wrappers.

- [ ] **Step 13.3 — Run all report package tests**

Run: `go test ./internal/cli/time/report/ -count=1`
Expected: PASS.

- [ ] **Step 13.4 — Commit**

```bash
git add internal/cli/time/report/status.go
git commit -m "feat(cli): wire runStatus to runner + format dispatch"
```

---

## Task 14: MCP tool

**Files:**
- Create: `internal/mcp/tools_report.go`
- Create: `internal/mcp/tools_report_test.go`
- Modify: `internal/mcp/server.go`
- Modify: `internal/mcp/server_test.go`

- [ ] **Step 14.1 — Bump wantCount in server_test.go**

Find the line `const wantCount = 38` in `internal/mcp/server_test.go` and change to `const wantCount = 39`.

- [ ] **Step 14.2 — Failing test for MCP handler**

Create `internal/mcp/tools_report_test.go`:

```go
package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestGetTimeStatusReport_RejectsMissingSelector(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(stub.Close)

	svcs := mcpHarness(t, stub.URL)
	handler := getTimeStatusReportHandler(svcs)
	res, _, err := handler(context.Background(), &sdkmcp.CallToolRequest{}, getTimeStatusReportArgs{
		Week: "2026-04-14",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Errorf("expected error result for missing selector")
	}
}
```

Run: `go test ./internal/mcp/ -run TestGetTimeStatusReport -v`
Expected: FAIL — handler undefined.

- [ ] **Step 14.3 — Implement tools_report.go**

Create `internal/mcp/tools_report.go`:

```go
package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/iainmoffat/tdx/internal/cli/time/report"
)

type getTimeStatusReportArgs struct {
	Profile     string   `json:"profile,omitempty"`
	Week        string   `json:"week,omitempty"`
	From        string   `json:"from,omitempty"`
	To          string   `json:"to,omitempty"`
	UserUIDs    []string `json:"userUIDs,omitempty"`
	Manager     string   `json:"manager,omitempty"`
	Account     string   `json:"account,omitempty"`
	All         bool     `json:"all,omitempty"`
	IncludeZero bool     `json:"includeZero,omitempty"`
	Limit       int      `json:"limit,omitempty"`
}

// RegisterReportTools registers the read-only Time Status Report tool.
func RegisterReportTools(srv *sdkmcp.Server, svcs Services) {
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name: "get_time_status_report",
		Description: `Generate a Time Status Report (per user, per week).

Selectors (exactly one): userUIDs, manager, account, all.
Date: week (single) or from/to (range).
Read-only — no confirm required.`,
	}, getTimeStatusReportHandler(svcs))
}

func getTimeStatusReportHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, getTimeStatusReportArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args getTimeStatusReportArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)

		// Bridge to runner.assembleReport via the report package's exported
		// Run helper. Since assembleReport + statusFlags are unexported,
		// expose a single Run function from the report package for MCP use.
		out, err := report.RunForMCP(ctx, report.MCPInputs{
			Profile:     profile,
			Week:        args.Week,
			From:        args.From,
			To:          args.To,
			Users:       args.UserUIDs,
			Manager:     args.Manager,
			Account:     args.Account,
			All:         args.All,
			IncludeZero: args.IncludeZero,
			Limit:       args.Limit,
			TimeSvc:     svcs.Time,
			PeopleSvc:   svcs.People,
			AuthSvc:     svcs.Auth,
		})
		if err != nil {
			return errorResult(fmt.Sprintf("time-status-report: %v", err)), nil, nil
		}

		body, err := json.Marshal(out)
		if err != nil {
			return errorResult(fmt.Sprintf("encode: %v", err)), nil, nil
		}
		return jsonResult(json.RawMessage(body))
	}
}
```

- [ ] **Step 14.4 — Add Services.People; expose MCP entry point in report package**

In `internal/mcp/server.go`, add `People *peoplesvc.Service` to the `Services` struct, and add `RegisterReportTools(srv, svcs)` to NewServer.

In `internal/cli/time/report/runner.go`, add an exported `RunForMCP` helper (and an `MCPInputs` struct) that:
1. Constructs a `statusFlags` from the inputs.
2. Calls `assembleReport`.
3. Returns the JSON envelope as `map[string]any` (or a typed struct re-using `printJSON`'s envelope).

Sketch:

```go
type MCPInputs struct {
	Profile     string
	Week        string
	From, To    string
	Users       []string
	Manager     string
	Account     string
	All         bool
	IncludeZero bool
	Limit       int
	TimeSvc     timesvcAPI
	PeopleSvc   peoplesvcAPI
	AuthSvc     authsvcAPI
}

func RunForMCP(ctx context.Context, in MCPInputs) (interface{}, error) {
	f := statusFlags{
		profile: in.Profile, week: in.Week, from: in.From, to: in.To,
		users: in.Users, manager: in.Manager, account: in.Account, all: in.All,
		yes: in.All, // bypass --yes guard for MCP (agent already opted in)
		includeZero: in.IncludeZero,
		limit:       in.Limit,
	}
	if err := validateStatusFlags(f); err != nil {
		return nil, err
	}
	deps := runnerDeps{Time: in.TimeSvc, People: in.PeopleSvc, Auth: in.AuthSvc, Profile: in.Profile}
	rep, err := assembleReport(ctx, deps, f)
	if err != nil {
		return nil, err
	}
	// Use the same envelope as printJSON; build it inline.
	return buildJSONEnvelope(rep, f), nil
}
```

The `buildJSONEnvelope(rep, f)` function should be the body of printJSON refactored to return the envelope value rather than write it. Refactor `printJSON` to call `buildJSONEnvelope` then encode.

- [ ] **Step 14.5 — Register the tool**

Add `RegisterReportTools(srv, svcs)` to `internal/mcp/server.go`'s `NewServer`.

- [ ] **Step 14.6 — Verify**

Run: `go test ./internal/mcp/ -count=1 -v 2>&1 | tail -20`
Expected: PASS — `wantCount=39` matches; the new handler test passes.

- [ ] **Step 14.7 — Commit**

```bash
git add internal/mcp/tools_report.go internal/mcp/tools_report_test.go internal/mcp/server.go internal/mcp/server_test.go internal/cli/time/report/runner.go
git commit -m "feat(mcp): get_time_status_report tool (38 -> 39)"
```

---

## Task 15: Docs + verification + version bump + PR

**Files:**
- Modify: `README.md`
- Modify: `docs/guide.md`

- [ ] **Step 15.1 — README updates**

In `README.md`:

1. Add a Time Reports subtable (after the Time Templates subtable):

```markdown
### Time Reports

| Command | Description | Key Flags |
|---|---|---|
| `tdx time report status` | Weekly time-status report (per user, per week) | `--week`, `--from`/`--to`, `--user`, `--manager`, `--account`, `--all`, `--include-zero`, `--limit`, `--json`, `--csv`, `--xlsx` |
```

2. Add to the MCP read-only tools table:

```markdown
| `get_time_status_report` | Generate a Time Status Report; read-only |
```

3. Update the count of read-only MCP tools by +1.

4. Add to the JSON-schema list:

```markdown
Schema names introduced in Phase C — Reports: `tdx.v1.timeStatusReport`.
```

5. Add a Quick Start example:

```bash
# Weekly time-status report for your direct reports
tdx time report status --manager me --week 2026-04-12
```

- [ ] **Step 15.2 — guide.md updates**

In `docs/guide.md`, add a new top-level section "## Time Reports" with subsection "### Time Status Report" explaining:
- Column meanings (Name, Email, Reports To, Reports To Email, Status, Bill Hrs, Non-Bill Hrs, Total Hrs).
- Underlying TD endpoints used (`/api/time/report/{date}/{uid}`, `/api/people/{uid}`, `/api/people/search`).
- Permission requirements (Analysis app role OR be the user / their approver).
- Selector flags + format flags + example invocations for human, JSON, CSV, XLSX.

- [ ] **Step 15.3 — Live verification**

```bash
go build -o /tmp/tdx ./cmd/tdx
/tmp/tdx time report status --manager me --week 2026-04-12 --include-zero=false 2>&1 | head -20
/tmp/tdx time report status --user $(/tmp/tdx auth status --json 2>&1 | jq -r '.user.uid') --week 2026-04-12 --csv > /tmp/test-report.csv && head /tmp/test-report.csv
```

Expected: human output + CSV output both render correctly.

- [ ] **Step 15.4 — Full quality gate**

```bash
go test ./... && go vet ./... && gofmt -l . && golangci-lint run ./...
```

Expected: all green; gofmt empty; lint 0 issues.

- [ ] **Step 15.5 — Commit docs**

```bash
git add README.md docs/guide.md
git commit -m "docs: tdx time report status (Phase C — Reports)"
```

- [ ] **Step 15.6 — Push, open PR**

```bash
git push -u origin time-report-status
```

PR body in `/tmp/pr-body.md`:

```
## Summary
- New `tdx time report status` command + `get_time_status_report` MCP tool
- Per-user weekly time-status report mirroring TD's "Time Status Report"
- Output formats: human table (default), `--json`, `--csv` (stdout), `--xlsx PATH`
- Selectors: `--user`, `--manager` (incl. "me"), `--account`, `--all` (with `--yes`)
- Date: `--week` or `--from`/`--to`

## Spec
docs/specs/2026-04-30-time-report-status.md

## New packages
- `internal/svc/peoplesvc` (GetUser, SearchUsers)
- `internal/cli/time/report` (parent + status command)

## Domain additions
- `WeekStatusRow`, `TimeStatusReport`, `UserFilter`
- `User` extended with manager fields
- `WeekReport` extended with billable/non-billable totals
- `ErrPermission` sentinel
- `tdx.v1.timeStatusReport` JSON schema

## Test plan
- [x] go test ./...
- [x] go vet ./...
- [x] gofmt -l .
- [x] golangci-lint run ./... (0 issues)
- [x] Live verification on UFL: --manager me --week ...

## Demo
[paste sample human output here]
[paste sample --json output here]
```

```bash
gh pr create --title "feat(time): tdx time report status (Time Status Report) + MCP tool" --body-file /tmp/pr-body.md
```

- [ ] **Step 15.7 — After merge: tag v0.9.0**

Minor bump (new public CLI surface + new MCP tool + new dep + new domain types).

```bash
git checkout main
git pull
git tag -a v0.9.0 -m "feat(time): tdx time report status (Time Status Report)"
git push origin v0.9.0
```

---

## Notes for the implementer

- **Subagent-driven recommended.** Multi-package surface; isolated subagent contexts keep each task focused.
- **Verify `decodeReportStatus`** maps wire status codes to the right `domain.ReportStatus` values. This is reused by `GetWeekReportForUser` — no changes needed, but the runner relies on the existing mapping.
- **`wireTimeReport.MinutesBillable` / `MinutesNonBillable`** already exist in `internal/svc/timesvc/types.go`. We're just plumbing them through.
- **`auth.ClientFor(profile)`** — verify this is the actual helper name. If the existing pattern in `timesvc/service.go` uses a different helper, mirror that exactly.
- **`timesvc.Service` and `peoplesvc.Service`** must satisfy `timesvcAPI`/`peoplesvcAPI` interfaces. If `WhoAmI` on `authsvc.Service` doesn't have the `(ctx, profile)` shape the interface expects, write a tiny adapter struct in `runner.go`.
- **Live tenant** has the auth token still loaded. After Task 13, exercise `--user`, `--manager me`, `--csv`, and `--xlsx` against your own UID before opening the PR.
- Tag is **v0.9.0** (minor) — new feature surface that doesn't break any existing CLI. v1.0.0 stays available for a bigger milestone.
