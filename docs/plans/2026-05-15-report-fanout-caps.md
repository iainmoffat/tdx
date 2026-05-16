# Report Fan-Out Caps Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two hard caps (`MaxReportWeeks=52`, `MaxReportUsers=1000`) to every per-(user, week) API fan-out in tdx — so accidental or LLM-driven calls can't generate runaway TD API traffic. Covers `tdx time report status` (CLI + MCP) and `tdx project time` (CLI + MCP).

**Architecture:** New domain-level sentinel `ErrFanoutLimitExceeded`, package consts `MaxReportWeeks` / `MaxReportUsers`, and a small DST-safe `WeekSpan(from, to)` helper — all in `internal/domain`. Three independent call sites enforce the caps:
1. `internal/cli/time/report/runner.go` — `resolveWeeks` checks span; `assembleReport` checks resolved user count.
2. `internal/cli/project/time.go` — inline date parsing checks span; user resolution checks count.
3. `internal/mcp/tools_project.go` — same checks in the MCP handler.
4. `internal/cli/time/report/status.go` — existing `--limit > 1000` refusal upgrades to wrap the new sentinel.

**Tech Stack:** Go 1.26.2; cobra; testify/require. No new dependencies.

**Spec:** [`docs/specs/2026-05-15-report-fanout-caps.md`](../specs/2026-05-15-report-fanout-caps.md)

---

## File Structure

After this plan completes:

```
internal/
├── domain/
│   ├── caps.go                              # CREATE: MaxReportWeeks, MaxReportUsers, WeekSpan
│   ├── caps_test.go                         # CREATE: WeekSpan table tests (incl. DST boundary)
│   └── errors.go                            # MODIFY: add ErrFanoutLimitExceeded sentinel
├── cli/
│   ├── time/
│   │   └── report/
│   │       ├── runner.go                    # MODIFY: cap check in resolveWeeks; refuse over-limit users
│   │       ├── runner_test.go               # MODIFY: ~5 new tests
│   │       └── status.go                    # MODIFY: --limit > 1000 wraps sentinel; help text updated
│   └── project/
│       ├── time.go                          # MODIFY: cap checks in command RunE
│       └── time_test.go                     # MODIFY: ~4 new tests
└── mcp/
    └── tools_project.go                     # MODIFY: cap checks in get_project_time_review handler
docs/
└── manual-tests/
    └── 2026-05-15-fanout-caps-walkthrough.md  # CREATE
```

No README or top-level tree changes.

## Branch + Versioning

- Branch: `fanout-caps` (Task 0)
- Version: v0.20.0 (minor — additive guardrails, no breaking changes; tagged after merge)

---

## Task 0: Create branch

**Files:** none

- [ ] **Step 1: Confirm clean tree on main**

```bash
git status
```

Expected: clean. (Main is at `4714bfb`.)

- [ ] **Step 2: Create branch**

```bash
git checkout -b fanout-caps
```

Expected: `Switched to a new branch 'fanout-caps'`.

---

## Task 1: Domain sentinel, constants, and `WeekSpan` helper

**Files:**
- Modify: `internal/domain/errors.go`
- Create: `internal/domain/caps.go`
- Create: `internal/domain/caps_test.go`

- [ ] **Step 1: Add sentinel error**

In `internal/domain/errors.go`, add inside the existing `var (...)` block (just above the closing paren):

```go
	// ErrFanoutLimitExceeded indicates a per-user × per-week fan-out request
	// would exceed the hard caps defined in caps.go. Wrap with details about
	// which limit (weeks or users), the requested value, and the max.
	ErrFanoutLimitExceeded = errors.New("fanout_limit_exceeded")
```

- [ ] **Step 2: Write failing tests for caps + WeekSpan**

Create `internal/domain/caps_test.go`:

```go
package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCaps_Values(t *testing.T) {
	require.Equal(t, 52, MaxReportWeeks)
	require.Equal(t, 1000, MaxReportUsers)
}

func TestWeekSpan_SameDay(t *testing.T) {
	d := time.Date(2026, 4, 14, 0, 0, 0, 0, EasternTZ)
	require.Equal(t, 1, WeekSpan(d, d))
}

func TestWeekSpan_SameWeek(t *testing.T) {
	// Tuesday → Friday in the same Sun-anchored week
	from := time.Date(2026, 4, 14, 0, 0, 0, 0, EasternTZ) // Tue
	to := time.Date(2026, 4, 17, 0, 0, 0, 0, EasternTZ)   // Fri
	require.Equal(t, 1, WeekSpan(from, to))
}

func TestWeekSpan_AcrossWeeks(t *testing.T) {
	// Sun 2026-04-12 (week 1) → Sun 2026-04-19 (week 2)
	from := time.Date(2026, 4, 12, 0, 0, 0, 0, EasternTZ)
	to := time.Date(2026, 4, 19, 0, 0, 0, 0, EasternTZ)
	require.Equal(t, 2, WeekSpan(from, to))
}

func TestWeekSpan_DSTSpringForward(t *testing.T) {
	// Spring-forward 2026-03-08: the Saturday→Sunday transition loses 1 hour
	// but is still exactly 1 week. WeekSpan must not use Sub()/24 arithmetic.
	from := time.Date(2026, 3, 1, 0, 0, 0, 0, EasternTZ)  // Sun (before DST)
	to := time.Date(2026, 3, 15, 0, 0, 0, 0, EasternTZ)   // Sun (after DST)
	require.Equal(t, 3, WeekSpan(from, to))
}

func TestWeekSpan_MaxWeeks(t *testing.T) {
	// 52 weeks exactly: from Sun to the Sun 51 weeks later
	from := time.Date(2026, 1, 4, 0, 0, 0, 0, EasternTZ)
	to := from.AddDate(0, 0, 7*51)
	require.Equal(t, 52, WeekSpan(from, to))
}

func TestWeekSpan_ToBeforeFrom(t *testing.T) {
	// Negative span is invalid; helper returns 0.
	from := time.Date(2026, 4, 14, 0, 0, 0, 0, EasternTZ)
	to := time.Date(2026, 4, 7, 0, 0, 0, 0, EasternTZ)
	require.Equal(t, 0, WeekSpan(from, to))
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./internal/domain/... -run TestWeekSpan -v
```

Expected: FAIL — `WeekSpan` and `MaxReportWeeks`/`MaxReportUsers` are not defined yet.

- [ ] **Step 4: Implement caps.go**

Create `internal/domain/caps.go`:

```go
package domain

import "time"

const (
	// MaxReportWeeks bounds the number of Sunday-anchored weeks a single
	// time-report or project-time fan-out call may span. Set to 52 (one year)
	// so accidental wide-range calls (e.g. --from 2010-01-01 --to 2030-01-01)
	// are refused before any TD API request is issued.
	MaxReportWeeks = 52

	// MaxReportUsers bounds the resolved user set per fan-out call. Set to
	// 1000 to refuse pathologically-wide selector expansion (e.g. --all on
	// a multi-thousand-staff tenant) while still allowing typical
	// quarterly reports.
	MaxReportUsers = 1000
)

// WeekSpan returns the number of Sunday-anchored weeks (in EasternTZ) that
// the inclusive range [from, to] touches. Returns 0 when to < from.
//
// DST-safe: iterates by AddDate(0,0,7) on the week-start, never by
// time.Sub() / 24 — spring-forward / fall-back days have 23 / 25 hours.
func WeekSpan(from, to time.Time) int {
	if to.Before(from) {
		return 0
	}
	startWeek := WeekRefContaining(from)
	endWeek := WeekRefContaining(to)
	n := 0
	cur := startWeek
	for !cur.StartDate.After(endWeek.StartDate) {
		n++
		cur = WeekRefContaining(cur.StartDate.AddDate(0, 0, 7))
	}
	return n
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/domain/... -v
```

Expected: PASS for all `TestWeekSpan_*` and `TestCaps_Values`.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/errors.go internal/domain/caps.go internal/domain/caps_test.go
git commit -m "feat(domain): add MaxReportWeeks/MaxReportUsers caps and WeekSpan helper"
```

---

## Task 2: Week-span cap in report `resolveWeeks`

**Files:**
- Modify: `internal/cli/time/report/runner.go:287-315` (the `resolveWeeks` function)
- Modify: `internal/cli/time/report/runner_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/cli/time/report/runner_test.go`:

```go
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
	f := statusFlags{from: "2026-01-04", to: "2026-12-26"}
	weeks, err := resolveWeeks(f)
	require.NoError(t, err)
	require.Equal(t, 52, len(weeks))
}

func TestResolveWeeks_Refuses53Weeks(t *testing.T) {
	// 53 weeks — one over the cap.
	f := statusFlags{from: "2026-01-04", to: "2027-01-02"}
	_, err := resolveWeeks(f)
	require.ErrorIs(t, err, domain.ErrFanoutLimitExceeded)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/cli/time/report/... -run TestResolveWeeks -v
```

Expected: FAIL — no cap check exists.

- [ ] **Step 3: Add cap check to `resolveWeeks`**

In `internal/cli/time/report/runner.go`, replace the existing `resolveWeeks` function (line 287 onward) with this version. The only changes are: (a) compute `domain.WeekSpan` before iterating, and (b) refuse if it exceeds the cap.

```go
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
	if span := domain.WeekSpan(from, to); span > domain.MaxReportWeeks {
		return nil, fmt.Errorf("%w: weeks=%d max=%d; narrow the --from/--to range",
			domain.ErrFanoutLimitExceeded, span, domain.MaxReportWeeks)
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
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/cli/time/report/... -v
```

Expected: all tests PASS, including the new `TestResolveWeeks_*` and all existing tests unaffected.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/time/report/runner.go internal/cli/time/report/runner_test.go
git commit -m "feat(report): refuse report ranges over MaxReportWeeks (52)"
```

---

## Task 3: Resolved-user cap in report `assembleReport`

**Files:**
- Modify: `internal/cli/time/report/runner.go:66-84` (the `assembleReport` function — the limit-clamp block at lines 77-84)
- Modify: `internal/cli/time/report/runner_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/cli/time/report/runner_test.go`:

```go
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
```

You will also need to add `"fmt"` to the test file's imports if not already present (it likely already is).

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/cli/time/report/... -run 'TestRunner_(Refuses|Allows)' -v
```

Expected: FAIL — `TestRunner_RefusesOverMaxUsers` fails because today's code silently truncates rather than refusing.

- [ ] **Step 3: Replace the limit-clamp block with a refusal**

In `internal/cli/time/report/runner.go`, replace the block at lines 77-84 (the `// Apply --limit cap.` block) with:

```go
	// Refuse if resolved user set exceeds the hard cap. This catches wide-
	// selector cases (e.g. --all on a multi-thousand-staff tenant) that today
	// would silently truncate. --limit N (user-explicit narrowing where
	// N ≤ MaxReportUsers) is checked separately in validateStatusFlags.
	if len(users) > domain.MaxReportUsers {
		return domain.TimeStatusReport{}, fmt.Errorf("%w: users=%d max=%d; narrow with --resource-pool, --account, or --manager",
			domain.ErrFanoutLimitExceeded, len(users), domain.MaxReportUsers)
	}

	// Apply --limit cap (user-explicit narrowing).
	cap := f.limit
	if cap <= 0 {
		cap = domain.MaxReportUsers
	}
	if len(users) > cap {
		users = users[:cap]
	}
```

Note: the inner `cap > hardLimit` check from the original is gone — `validateStatusFlags` already refuses `--limit > 1000`. The local `hardLimit` const (line 57 of runner.go) is still referenced by other code? Search and remove if unused:

```bash
grep -n "hardLimit" /Users/ipm/code/tdx/internal/cli/time/report/*.go
```

If `hardLimit` has no other callers, remove the const from runner.go:55-58.

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/cli/time/report/... -v
```

Expected: all tests PASS, including new `TestRunner_RefusesOverMaxUsers` and `TestRunner_Allows1000Users`, plus all pre-existing runner tests.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/time/report/runner.go internal/cli/time/report/runner_test.go
git commit -m "feat(report): refuse resolved user sets over MaxReportUsers (1000)"
```

---

## Task 4: Upgrade `--limit > 1000` error to wrap the sentinel

**Files:**
- Modify: `internal/cli/time/report/status.go:144-146` and `:87`
- Modify: `internal/cli/time/report/status_test.go`

- [ ] **Step 1: Write failing test in `status_test.go`**

Validation tests for `validateStatusFlags` live in `internal/cli/time/report/status_test.go` (named `TestStatus_*`). Append:

```go
func TestStatus_LimitOver1000WrapsSentinel(t *testing.T) {
	f := statusFlags{week: "2026-04-12", users: []string{"u1"}, limit: 1500}
	err := validateStatusFlags(f)
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrFanoutLimitExceeded)
	require.Contains(t, err.Error(), "limit=1500")
	require.Contains(t, err.Error(), "max=1000")
}
```

Add `"github.com/iainmoffat/tdx/internal/domain"` to status_test.go's imports if not already present.

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/cli/time/report/... -run TestStatus_LimitOver1000WrapsSentinel -v
```

Expected: FAIL — current error is a plain `fmt.Errorf` with no sentinel.

- [ ] **Step 3: Upgrade the error**

In `internal/cli/time/report/status.go`, replace lines 144-146:

```go
	// Limit cap
	if f.limit > 1000 {
		return fmt.Errorf("--limit cannot exceed 1000")
	}
```

With:

```go
	// Limit cap (wraps domain.ErrFanoutLimitExceeded so MCP / tests can
	// detect via errors.Is alongside the runtime resolved-user check).
	if f.limit > domain.MaxReportUsers {
		return fmt.Errorf("%w: limit=%d max=%d; --limit cannot exceed %d",
			domain.ErrFanoutLimitExceeded, f.limit, domain.MaxReportUsers, domain.MaxReportUsers)
	}
```

Add `"github.com/iainmoffat/tdx/internal/domain"` to the imports if not already present.

- [ ] **Step 4: Run tests to verify**

```bash
go test ./internal/cli/time/report/... -v
```

Expected: PASS, all tests.

- [ ] **Step 5: Update help text on --limit flag**

In `internal/cli/time/report/status.go:87`, update the flag help text:

```go
	cmd.Flags().IntVar(&f.limit, "limit", 200, "cap user count (hard cap: 1000; resolved sets over 1000 are also refused)")
```

- [ ] **Step 6: Commit**

```bash
git add internal/cli/time/report/status.go internal/cli/time/report/status_test.go
git commit -m "feat(report): wrap --limit>1000 error in ErrFanoutLimitExceeded"
```

---

## Task 5: Add caps to `tdx project time` CLI

**Files:**
- Modify: `internal/cli/project/time.go`
- Modify: `internal/cli/project/time_test.go`

- [ ] **Step 1: Read the current command structure**

```bash
sed -n '50,120p' /Users/ipm/code/tdx/internal/cli/project/time.go
```

Note the date-parsing block (around lines 80-105) and user-resolution block (around lines 117-156). The new checks slot in between user resolution and the call to `runProjectTimeRender`.

- [ ] **Step 2: Write failing tests**

Append to `internal/cli/project/time_test.go`:

```go
func TestProjectTime_RefusesOverMaxWeekSpan(t *testing.T) {
	cmd := newTimeCmd(nil, nil)
	cmd.SetArgs([]string{"259", "--from", "2020-01-01", "--to", "2030-01-01"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrFanoutLimitExceeded)
	require.Contains(t, err.Error(), "weeks=")
}

func TestProjectTime_RefusesOverMaxUsers(t *testing.T) {
	// Build a project-resource list of 1001 synthetic resources.
	resources := make([]domain.ProjectResource, domain.MaxReportUsers+1)
	for i := range resources {
		resources[i] = domain.ProjectResource{
			UID:      fmt.Sprintf("u%04d", i),
			FullName: fmt.Sprintf("user %d", i),
		}
	}
	psvc := &stubProjectsvc{resources: resources}
	tsvc := &stubTimesvcTime{}
	cmd := newTimeCmd(psvc, tsvc)
	cmd.SetArgs([]string{"259", "--week", "2026-04-12", "--all-users"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrFanoutLimitExceeded)
	require.Contains(t, err.Error(), "users=1001")
}
```

Verified stub names: `stubProjectsvc` lives in `internal/cli/project/stub_test.go` (shared across project tests; has a `resources []domain.ProjectResource` field). `stubTimesvcTime` is local to `internal/cli/project/time_test.go`. Tests will need `"io"` and `"fmt"` imports if not already present.

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./internal/cli/project/... -run 'TestProjectTime_(Refuses)' -v
```

Expected: FAIL.

- [ ] **Step 4: Add cap checks in `newTimeCmd`'s `RunE`**

In `internal/cli/project/time.go`, add the week-span check immediately after the date-range resolution block (just after the `switch { case weekFlag != "": ... }` block ends, around line 106), before the `// Resolve services.` comment:

```go
	// Refuse over-cap week ranges before any IO.
	if span := domain.WeekSpan(rng.From, rng.To); span > domain.MaxReportWeeks {
		return fmt.Errorf("%w: weeks=%d max=%d; narrow the --from/--to range",
			domain.ErrFanoutLimitExceeded, span, domain.MaxReportWeeks)
	}
```

Then after the user-resolution `switch` block ends (around line 156, just before the `return runProjectTimeRender(...)` line), add:

```go
	// Refuse over-cap resolved user sets.
	if len(users) > domain.MaxReportUsers {
		return fmt.Errorf("%w: users=%d max=%d; narrow with --user or a smaller team",
			domain.ErrFanoutLimitExceeded, len(users), domain.MaxReportUsers)
	}
```

Confirm `domain` is already imported (it should be; the file already references `domain.WeekRefContaining`, etc.).

- [ ] **Step 5: Run tests to verify pass**

```bash
go test ./internal/cli/project/... -v
```

Expected: PASS, all tests.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/project/time.go internal/cli/project/time_test.go
git commit -m "feat(project): apply fan-out caps to tdx project time"
```

---

## Task 6: Add caps to `get_project_time_review` MCP handler

**Files:**
- Modify: `internal/mcp/tools_project.go` (around lines 530-570 — the project-time-review handler)

**Testing note:** The MCP handler is registered as a closure inside `RegisterProjectTools` and is not externally addressable for direct unit testing; the existing `tools_project_test.go` exercises tools via the MCP SDK with `httptest.NewServer` mocks. That setup is heavy for what's essentially a string-prefix check on a pre-IO refusal. The cap logic itself is covered by Task 1 (`WeekSpan` tests) and Task 5 (project CLI tests). The MCP handler's only job is to call the helper and forward the error to `errorResult`. We verify end-to-end in the Task 8 walkthrough rather than building MCP-handler test scaffolding.

- [ ] **Step 1: Locate the handler's date and user resolution**

```bash
sed -n '500,600p' /Users/ipm/code/tdx/internal/mcp/tools_project.go
```

Identify where (a) the date range `rng` is finalized, and (b) the `users` slice is finalized. These are the insertion points for the two cap checks.

- [ ] **Step 2: Add cap checks**

In `internal/mcp/tools_project.go`, after the date-range resolution (after the inline `time.ParseInLocation` block for `from`/`to`), add:

```go
		if span := domain.WeekSpan(rng.From, rng.To); span > domain.MaxReportWeeks {
			return errorResult(fmt.Sprintf("%v: weeks=%d max=%d; narrow the from/to range",
				domain.ErrFanoutLimitExceeded, span, domain.MaxReportWeeks)), nil, nil
		}
```

After user resolution (just before the call to the render/fetch helper), add:

```go
		if len(users) > domain.MaxReportUsers {
			return errorResult(fmt.Sprintf("%v: users=%d max=%d; narrow with userUIDs or a smaller team",
				domain.ErrFanoutLimitExceeded, len(users), domain.MaxReportUsers)), nil, nil
		}
```

Note: we use `%v` not `%w` because `errorResult` consumes a string, not an error. The string includes `fanout_limit_exceeded:` (from the sentinel's `Error()`) so an LLM can still pattern-match.

Confirm `domain` and `fmt` are imported.

- [ ] **Step 3: Run all MCP tests**

```bash
go test ./internal/mcp/... -v
```

Expected: PASS (no new tests, but existing tests should still pass after the additions).

- [ ] **Step 4: Commit**

```bash
git add internal/mcp/tools_project.go
git commit -m "feat(mcp): apply fan-out caps to get_project_time_review"
```

---

## Task 7: Full test + lint sweep

**Files:** none

- [ ] **Step 1: Run the full suite**

```bash
go test ./... && go vet ./... && gofmt -l . && golangci-lint run ./...
```

Expected: all green. No gofmt output. No vet warnings. No lint warnings.

- [ ] **Step 2: If failures appear, fix in place and commit per-issue**

Common gotchas:
- `errcheck` on type assertions in tests → add a `_ =` or `, ok := ; require.True(t, ok)` pattern
- Unused `hardLimit` const if Task 3 left it dangling → remove
- Missing imports → add

- [ ] **Step 3: Confirm green**

```bash
go test ./... -race
```

Expected: PASS with `-race` (matches CI).

---

## Task 8: Manual walkthrough doc

**Files:**
- Create: `docs/manual-tests/2026-05-15-fanout-caps-walkthrough.md`

- [ ] **Step 1: Write walkthrough**

Create `docs/manual-tests/2026-05-15-fanout-caps-walkthrough.md`:

```markdown
# Fan-Out Caps Walkthrough (v0.20.0)

Spec: [`docs/specs/2026-05-15-report-fanout-caps.md`](../specs/2026-05-15-report-fanout-caps.md)

## Step 1: `tdx time report status` week cap

```
tdx time report status --user me --from 2020-01-01 --to 2030-01-01
```

Expected:
- Exit 1
- Stderr contains `fanout_limit_exceeded: weeks=` and `max=52`

## Step 2: `tdx time report status` --limit cap

```
tdx time report status --user me --week 2026-04-12 --limit 5000
```

Expected: Exit 1; stderr contains `fanout_limit_exceeded: limit=5000 max=1000`.

## Step 3: `tdx time report status` boundary (52 weeks ✓)

```
tdx time report status --user me --from 2026-01-04 --to 2026-12-26 --json
```

Expected: Exit 0; JSON envelope has 52 weeks × 1 user = 52 rows (or fewer with `--include-zero=false`).

## Step 4: `tdx project time` week cap

```
tdx project time 259 --user me --from 2020-01-01 --to 2030-01-01
```

Expected: Exit 1; stderr `fanout_limit_exceeded: weeks=...`.

## Step 5: `tdx project time --all-users` user cap

This step needs a project with >1000 resources, which may not exist on the test tenant. If so, skip and note in this doc.

```
tdx project time <big-project-id> --all-users --week 2026-04-12
```

Expected (if applicable): Exit 1; stderr `fanout_limit_exceeded: users=...`.

## Step 6: MCP error shape

Via Claude or any MCP client, call `get_time_status_report` with a 10-year `from`/`to` range. The tool result should be an error containing `fanout_limit_exceeded: weeks=...`. The LLM should be able to retry with a narrower range.

Same for `get_project_time_review` with a 10-year range.

## Step 7: Within-cap calls unaffected

```
tdx time report status --manager me --week 2026-04-12 --json
```

Expected: Exit 0; output unchanged from v0.19.0 behavior.
```

- [ ] **Step 2: Commit**

```bash
git add docs/manual-tests/2026-05-15-fanout-caps-walkthrough.md
git commit -m "docs: walkthrough for fan-out caps (v0.20.0)"
```

---

## Task 9: PR

**Files:** none

- [ ] **Step 1: Push branch**

```bash
git push -u origin fanout-caps
```

- [ ] **Step 2: Create PR**

```bash
gh pr create --title "Fan-out caps for report and project-time (security hardening phase 1)" --body "$(cat <<'EOF'
## Summary

Phase 1 of a multi-phase security hardening rollup (see spec for the full roadmap).

Adds two hard caps to every per-(user, week) API fan-out:

- `MaxReportWeeks = 52`
- `MaxReportUsers = 1000`

Refusal at flag-validation or pre-fan-out time wraps the new `domain.ErrFanoutLimitExceeded` sentinel. Covers:

- `tdx time report status` (CLI + MCP via shared runner)
- `tdx project time` (CLI)
- `get_project_time_review` (MCP)

## Test plan

- [ ] `go test ./... -race` green
- [ ] `go vet ./... && gofmt -l . && golangci-lint run ./...` green
- [ ] Manual walkthrough at `docs/manual-tests/2026-05-15-fanout-caps-walkthrough.md`

Closes: security audit finding #4.
EOF
)"
```

---

## Self-Review Notes

After completing the plan, run through this checklist:

- [ ] Spec coverage:
  - Week cap (52) — Task 2 (report), Task 5 (project CLI), Task 6 (project MCP).
  - User cap (1000) — Task 3 (report), Task 5 (project CLI), Task 6 (project MCP), Task 4 (explicit `--limit` flag).
  - `ErrFanoutLimitExceeded` sentinel — Task 1.
  - `WeekSpan` helper — Task 1.
  - MCP error shape — Tasks 6 (handler-level) + 4 (flag-level via wrapping).
  - Roadmap phases — referenced from the spec, not implemented here.
- [ ] No placeholders.
- [ ] Type consistency: `MaxReportWeeks` / `MaxReportUsers` / `ErrFanoutLimitExceeded` / `WeekSpan` names match across tasks.
- [ ] Each task is self-contained — tasks can be reviewed and committed independently.
