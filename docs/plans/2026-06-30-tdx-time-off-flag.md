# `tdx time entry add --time-off` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use ultrapowers:subagent-driven-development (recommended) or ultrapowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users log a TeamDynamix time-off / leave entry from the CLI and MCP, closing the gap where the `timeoff` target was encodable but had no creation path.

**Architecture:** Add a `--time-off` target selector to `tdx time entry add` that resolves the tenant's time-off `ItemID` (auto-discovered from the user's recent leave entries, overridable via `--time-off-id`) and defaults `--type` to the tenant's time-off-flagged time type. The MCP `create_time_entry` tool already accepts `kind: "timeoff"` generically; it gains the same auto-discovery when `itemID` is omitted, plus an accurate `kind` schema description.

**Tech Stack:** Go 1.26.2, cobra CLI, `github.com/stretchr/testify/require`, `net/http/httptest` for service-level tests, modelcontextprotocol Go SDK.

**Spec:** `docs/specs/2026-06-30-tdx-time-off-flag-design.md`

## Global Constraints

- Go module path is `github.com/iainmoffat/tdx`. All internal imports use that prefix.
- Tests use `github.com/stretchr/testify/require` and `net/http/httptest`; CLI tests seed a profile with the existing `seedProfile(t, srv.URL)` helper in `internal/cli/time/entry/list_test.go:27`.
- Dates are parsed and compared in `domain.EasternTZ`. Never use `time.Local`.
- Never run `go mod tidy` (no new dependencies are introduced by this plan).
- No `Co-Authored-By` trailer on commits.
- Verification command for the whole repo: `go build ./... && go test ./... && go vet ./...`.
- Existing behavior must not change when `--time-off` is absent: `--type` stays required, and the three existing target selectors behave exactly as today.

## Pinned Decisions (rules the spec left open)

These are decided here so no implementer has to guess:

- **Discovery lookback window:** exactly 180 days, as `timeOffLookbackDays = 180`, counted back from "now" in `domain.EasternTZ`.
- **Discovery result cap:** `timeOffSearchLimit = 500` passed as `EntryFilter.Limit`.
- **"Most recent" tie-break:** sort candidate entries by `Date` descending; when two entries share the same `Date`, the higher `ID` wins. Deterministic via `sort.SliceStable`.
- **Multiple distinct ItemIDs in history:** take the `ItemID` of the single most-recent entry (per the sort above). Do not error, do not merge.
- **Discovery scope:** only the authenticated user's own entries (`EntryFilter.UserUID = user.UID`).
- **Candidate filter:** an entry qualifies only if `Target.Kind == domain.TargetTimeOff` **and** `Target.ItemID > 0`.
- **`DefaultTimeOffType` eligibility:** a time type qualifies only if `IsTimeOff && Active`. Exactly one match → use it; zero or two-or-more → error (caller then requires explicit `--type`).
- **`--type` matching case sensitivity:** unchanged — `domain.FindTimeTypeByName` already matches case-insensitively after trimming.
- **`--time-off-id 0`** is treated as "not supplied" (the override triggers only on `> 0`), consistent with every other int flag in `add.go`.

## File Structure

| File | Responsibility | Task |
|---|---|---|
| `internal/domain/timetype.go` | `TimeType.IsTimeOff` field + `DefaultTimeOffType` selector | 1 |
| `internal/domain/timetype_test.go` | tests for `DefaultTimeOffType` | 1 |
| `internal/svc/timesvc/timetypes.go` | populate `IsTimeOff` in `decodeTimeType` | 1 |
| `internal/domain/errors.go` | `ErrTimeOffIDUnknown` sentinel | 2 |
| `internal/svc/timesvc/timeoff.go` | **new** — `ResolveTimeOffItemID` + constants | 2 |
| `internal/svc/timesvc/timeoff_test.go` | **new** — override / discovery / not-found tests | 2 |
| `internal/cli/time/entry/add.go` | `--time-off` / `--time-off-id` flags, validation, resolution, target build | 3 |
| `internal/cli/time/entry/add_test.go` | CLI validation + success + dry-run tests | 3 |
| `internal/mcp/tools_entry.go` | `kind` schema text + auto-resolve `itemID` for timeoff | 4 |
| `internal/mcp/tools_entry_test.go` | MCP timeoff create test | 4 |
| `docs/guide/time.md`, `skills/tdx/SKILL.md` | user-facing docs | 5 |

---

### Task 1: Domain — `IsTimeOff` on TimeType and `DefaultTimeOffType`

**Files:**
- Modify: `internal/domain/timetype.go`
- Modify: `internal/svc/timesvc/timetypes.go:29-39` (`decodeTimeType`)
- Test: `internal/domain/timetype_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces:
  - `domain.TimeType.IsTimeOff bool` (JSON/YAML key `isTimeOff`)
  - `func domain.DefaultTimeOffType(types []TimeType) (TimeType, error)`

- [ ] **Step 1: Write the failing test**

`internal/domain/timetype_test.go` already exists. **Append** these tests to it; do not overwrite the file. It is `package domain`; ensure its import block includes `strings` and `testing`.

```go

func TestDefaultTimeOffType_SingleMatch(t *testing.T) {
	types := []TimeType{
		{ID: 1, Name: "Standard Activities", Active: true},
		{ID: 3, Name: "Leave", Active: true, IsTimeOff: true},
	}
	got, err := DefaultTimeOffType(types)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != 3 {
		t.Errorf("ID = %d, want 3", got.ID)
	}
}

func TestDefaultTimeOffType_NoMatch(t *testing.T) {
	types := []TimeType{{ID: 1, Name: "Standard Activities", Active: true}}
	if _, err := DefaultTimeOffType(types); err == nil {
		t.Errorf("expected an error when no time-off type exists")
	}
}

func TestDefaultTimeOffType_MultipleMatches(t *testing.T) {
	types := []TimeType{
		{ID: 3, Name: "Leave", Active: true, IsTimeOff: true},
		{ID: 4, Name: "Holiday", Active: true, IsTimeOff: true},
	}
	_, err := DefaultTimeOffType(types)
	if err == nil {
		t.Fatalf("expected an error when multiple time-off types exist")
	}
	// The error must name the candidates so the user knows what to pass to --type.
	msg := err.Error()
	if !strings.Contains(msg, "Leave") || !strings.Contains(msg, "Holiday") {
		t.Errorf("error %q should name both candidates", msg)
	}
}

func TestDefaultTimeOffType_IgnoresInactive(t *testing.T) {
	types := []TimeType{
		{ID: 3, Name: "Leave", Active: true, IsTimeOff: true},
		{ID: 9, Name: "Old Leave", Active: false, IsTimeOff: true},
	}
	got, err := DefaultTimeOffType(types)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != 3 {
		t.Errorf("ID = %d, want 3 (inactive type must be ignored)", got.ID)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/ -run TestDefaultTimeOffType -v`
Expected: FAIL — compile error `undefined: DefaultTimeOffType` and `unknown field IsTimeOff`.

- [ ] **Step 3: Add the field and the selector**

In `internal/domain/timetype.go`, add `IsTimeOff` to the struct (after `Active`):

```go
type TimeType struct {
	ID          int    `json:"id" yaml:"id"`
	Name        string `json:"name" yaml:"name"`
	Code        string `json:"code,omitempty" yaml:"code,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Billable    bool   `json:"billable" yaml:"billable"`
	Limited     bool   `json:"limited" yaml:"limited"`
	Active      bool   `json:"active" yaml:"active"`
	// IsTimeOff reports whether TD flags this type as a time-off/leave type
	// (wire field IsTimeOffTimeType). Used to default and validate --type for
	// time-off entries.
	IsTimeOff bool `json:"isTimeOff" yaml:"isTimeOff"`
}
```

Change the import line at the top of the same file from `import "strings"` to:

```go
import (
	"fmt"
	"strings"
)
```

Append to the end of `internal/domain/timetype.go`:

```go
// DefaultTimeOffType returns the tenant's single active time-off time type.
// It errors when there are zero or several candidates, because in those cases
// tdx cannot pick for the user and the caller must require an explicit --type.
func DefaultTimeOffType(types []TimeType) (TimeType, error) {
	var found []TimeType
	for _, t := range types {
		if t.IsTimeOff && t.Active {
			found = append(found, t)
		}
	}
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return TimeType{}, fmt.Errorf("no active time-off time type found")
	default:
		names := make([]string, 0, len(found))
		for _, t := range found {
			names = append(names, t.Name)
		}
		return TimeType{}, fmt.Errorf("multiple active time-off time types: %s",
			strings.Join(names, ", "))
	}
}
```

- [ ] **Step 4: Populate the field from the wire struct**

In `internal/svc/timesvc/timetypes.go`, add one line to `decodeTimeType`:

```go
func decodeTimeType(w wireTimeType) domain.TimeType {
	return domain.TimeType{
		ID:          w.ID,
		Name:        w.Name,
		Code:        w.Code,
		Description: w.HelpText,
		Billable:    w.IsBillable,
		Limited:     w.IsLimited,
		Active:      w.IsActive,
		IsTimeOff:   w.IsTimeOffTimeType,
	}
}
```

(`wireTimeType.IsTimeOffTimeType` already exists at `internal/svc/timesvc/types.go:97` — no wire change needed.)

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/domain/ ./internal/svc/timesvc/ -v -run 'TimeOff|TimeType'`
Expected: PASS (all four `TestDefaultTimeOffType_*` tests plus existing time-type tests).

- [ ] **Step 6: Commit**

```bash
git add internal/domain/timetype.go internal/domain/timetype_test.go internal/svc/timesvc/timetypes.go
git commit -m "Add IsTimeOff to TimeType and DefaultTimeOffType selector"
```

---

### Task 2: timesvc — `ResolveTimeOffItemID` discovery

**Files:**
- Modify: `internal/domain/errors.go`
- Create: `internal/svc/timesvc/timeoff.go`
- Test: `internal/svc/timesvc/timeoff_test.go`

**Interfaces:**
- Consumes: `domain.TargetTimeOff`, `domain.EntryFilter{DateRange, UserUID, Limit}`, `(*timesvc.Service).SearchEntries(ctx, profileName string, filter domain.EntryFilter) ([]domain.TimeEntry, error)`.
- Produces:
  - `domain.ErrTimeOffIDUnknown` (sentinel `error`)
  - `func (s *timesvc.Service) ResolveTimeOffItemID(ctx context.Context, profileName, userUID string, override int) (int, error)`

- [ ] **Step 1: Write the failing test**

Create `internal/svc/timesvc/timeoff_test.go`. It reuses the package's existing
fixture `harness(t, tenantURL) (*Service, string)` from
`internal/svc/timesvc/service_test.go:25`, which returns a `*Service` rooted at a
temp dir plus the profile name `"default"` — do not write a new setup helper.

```go
package timesvc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iainmoffat/tdx/internal/domain"
)

// timeOffSearchServer returns a stub TD server whose /time/search responds with
// the supplied JSON array body.
func timeOffSearchServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/TDWebApi/api/time/search" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
}

func TestResolveTimeOffItemID_OverrideWins(t *testing.T) {
	// The server must never be called when an override is supplied.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("override path must not call the API (got %s)", r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	svc, profile := harness(t, srv.URL)

	got, err := svc.ResolveTimeOffItemID(context.Background(), profile, "uid-abc", 52)
	require.NoError(t, err)
	require.Equal(t, 52, got)
}

func TestResolveTimeOffItemID_DiscoversMostRecent(t *testing.T) {
	// Component 17 = TimeOff. The 2026-05-14 entry is newer, so its ProjectID
	// (which decodes into Target.ItemID) must win over the older 2026-05-12 one.
	srv := timeOffSearchServer(t, `[
		{"TimeID":1,"Component":17,"ProjectID":40,"TimeDate":"2026-05-12T00:00:00Z","Minutes":60,"TimeTypeID":3,"Uid":"uid-abc","Status":0},
		{"TimeID":2,"Component":17,"ProjectID":52,"TimeDate":"2026-05-14T00:00:00Z","Minutes":60,"TimeTypeID":3,"Uid":"uid-abc","Status":0},
		{"TimeID":3,"Component":9,"TicketID":900,"ItemID":900,"TimeDate":"2026-05-20T00:00:00Z","Minutes":60,"TimeTypeID":1,"Uid":"uid-abc","Status":0}
	]`)
	defer srv.Close()
	svc, profile := harness(t, srv.URL)

	got, err := svc.ResolveTimeOffItemID(context.Background(), profile, "uid-abc", 0)
	require.NoError(t, err)
	require.Equal(t, 52, got)
}

func TestResolveTimeOffItemID_NotFound(t *testing.T) {
	// Only non-time-off entries: discovery must report the sentinel.
	srv := timeOffSearchServer(t, `[
		{"TimeID":3,"Component":9,"TicketID":900,"ItemID":900,"TimeDate":"2026-05-20T00:00:00Z","Minutes":60,"TimeTypeID":1,"Uid":"uid-abc","Status":0}
	]`)
	defer srv.Close()
	svc, profile := harness(t, srv.URL)

	_, err := svc.ResolveTimeOffItemID(context.Background(), profile, "uid-abc", 0)
	require.Error(t, err)
	require.True(t, errors.Is(err, domain.ErrTimeOffIDUnknown),
		"error %v should wrap domain.ErrTimeOffIDUnknown", err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/svc/timesvc/ -run TestResolveTimeOffItemID -v`
Expected: FAIL — compile error `svc.ResolveTimeOffItemID undefined` and `undefined: domain.ErrTimeOffIDUnknown`.

- [ ] **Step 3: Add the sentinel error**

In `internal/domain/errors.go`, add inside the existing `var (...)` block, after `ErrWeekSubmitted`:

```go
	// ErrTimeOffIDUnknown indicates tdx could not determine the tenant's
	// time-off ItemID from the user's recent entries and no override was given.
	ErrTimeOffIDUnknown = errors.New("time-off id unknown")
```

- [ ] **Step 4: Implement the resolver**

Create `internal/svc/timesvc/timeoff.go`:

```go
package timesvc

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

const (
	// timeOffLookbackDays bounds how far back discovery searches for a prior
	// time-off entry to copy the tenant's time-off ItemID from.
	timeOffLookbackDays = 180
	// timeOffSearchLimit bounds how many entries the discovery search requests.
	timeOffSearchLimit = 500
)

// ResolveTimeOffItemID returns the time-off ItemID to use for a new time-off
// entry.
//
// TD models time off as a pseudo-project whose ID is tenant-specific (52 on the
// UF tenant) and is not exposed on any time-type listing, so tdx discovers it
// from the user's own recent time-off entries. An override > 0 short-circuits
// the lookup entirely and performs no API call.
//
// Returns an error wrapping domain.ErrTimeOffIDUnknown when the user has no
// time-off entry in the lookback window; the caller should then tell the user
// to log one in the TD web UI or pass an explicit ID.
func (s *Service) ResolveTimeOffItemID(ctx context.Context, profileName, userUID string, override int) (int, error) {
	if override > 0 {
		return override, nil
	}

	now := time.Now().In(domain.EasternTZ)
	filter := domain.EntryFilter{
		DateRange: domain.DateRange{
			From: now.AddDate(0, 0, -timeOffLookbackDays),
			To:   now,
		},
		UserUID: userUID,
		Limit:   timeOffSearchLimit,
	}

	entries, err := s.SearchEntries(ctx, profileName, filter)
	if err != nil {
		return 0, fmt.Errorf("discover time-off id: %w", err)
	}

	candidates := make([]domain.TimeEntry, 0, len(entries))
	for _, e := range entries {
		if e.Target.Kind == domain.TargetTimeOff && e.Target.ItemID > 0 {
			candidates = append(candidates, e)
		}
	}
	if len(candidates) == 0 {
		return 0, fmt.Errorf("%w: no time-off entry in the last %d days",
			domain.ErrTimeOffIDUnknown, timeOffLookbackDays)
	}

	// Most recent wins; ties break on the higher entry ID so the result is
	// deterministic when several entries share a date.
	sort.SliceStable(candidates, func(i, j int) bool {
		if !candidates[i].Date.Equal(candidates[j].Date) {
			return candidates[i].Date.After(candidates[j].Date)
		}
		return candidates[i].ID > candidates[j].ID
	})
	return candidates[0].Target.ItemID, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/svc/timesvc/ -run TestResolveTimeOffItemID -v`
Expected: PASS — all three tests.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/errors.go internal/svc/timesvc/timeoff.go internal/svc/timesvc/timeoff_test.go
git commit -m "Add ResolveTimeOffItemID discovery to timesvc"
```

---

### Task 3: CLI — `--time-off` on `tdx time entry add`

**Files:**
- Modify: `internal/cli/time/entry/add.go`
- Test: `internal/cli/time/entry/add_test.go`

**Interfaces:**
- Consumes: `domain.DefaultTimeOffType` (Task 1), `domain.TimeType.IsTimeOff` (Task 1), `(*timesvc.Service).ResolveTimeOffItemID` and `domain.ErrTimeOffIDUnknown` (Task 2).
- Produces: user-facing flags `--time-off` (bool) and `--time-off-id` (int); a `domain.TargetTimeOff` case in `buildTarget` and `targetSummary`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/cli/time/entry/add_test.go`:

```go
func TestAddCmd_TimeOffRequiresOneTarget(t *testing.T) {
	// --time-off together with --ticket is two selectors: must be rejected
	// before any network call, so no server is needed.
	seedProfile(t, "http://127.0.0.1:1")

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"add", "--date", "2026-04-11", "--hours", "2",
		"--time-off", "--ticket", "100", "--app", "5",
	})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "exactly one of")
}

func TestAddCmd_TimeOffIDRequiresTimeOff(t *testing.T) {
	seedProfile(t, "http://127.0.0.1:1")

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"add", "--date", "2026-04-11", "--hours", "2",
		"--ticket", "100", "--app", "5", "--type", "Development",
		"--time-off-id", "52",
	})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--time-off-id requires --time-off")
}

func TestAddCmd_TimeOffRejectsCompanionFlags(t *testing.T) {
	seedProfile(t, "http://127.0.0.1:1")

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"add", "--date", "2026-04-11", "--hours", "2",
		"--time-off", "--app", "5",
	})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--time-off cannot be combined")
}

func TestAddCmd_TimeOffDryRunUsesOverrideAndDefaultsType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ReferenceID":42,"UID":"user-abc","FullName":"Test User","PrimaryEmail":"test@example.com"}`))

		case r.URL.Path == "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"ID":1,"Name":"Standard Activities","IsActive":true,"IsBillable":false},
				{"ID":3,"Name":"Leave","IsActive":true,"IsBillable":false,"IsTimeOffTimeType":true}
			]`))

		case r.URL.Path == "/TDWebApi/api/time/locked":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))

		case strings.HasPrefix(r.URL.Path, "/TDWebApi/api/time/report/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ID":1,"PeriodStartDate":"2026-04-05T00:00:00Z","PeriodEndDate":"2026-04-11T00:00:00Z","Status":0,"Times":[],"TimeReportUid":"user-abc","UserFullName":"Test User","MinutesBillable":0,"MinutesNonBillable":0,"MinutesTotal":0,"TimeEntriesCount":0}`))

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	seedProfile(t, srv.URL)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"add", "--date", "2026-04-11", "--hours", "2",
		"--time-off", "--time-off-id", "52", "--dry-run",
	})
	require.NoError(t, cmd.Execute())

	got := out.String()
	require.Contains(t, got, "time-off (id 52)")
	require.Contains(t, got, "kind=timeoff")
	// --type omitted: the single IsTimeOffTimeType type must be chosen.
	require.Contains(t, got, "Leave")
}

func TestAddCmd_TimeOffRejectsNonTimeOffType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ReferenceID":42,"UID":"user-abc","FullName":"Test User","PrimaryEmail":"test@example.com"}`))
		case r.URL.Path == "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"ID":1,"Name":"Standard Activities","IsActive":true,"IsBillable":false},
				{"ID":3,"Name":"Leave","IsActive":true,"IsBillable":false,"IsTimeOffTimeType":true}
			]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	seedProfile(t, srv.URL)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"add", "--date", "2026-04-11", "--hours", "2",
		"--time-off", "--time-off-id", "52",
		"--type", "Standard Activities", "--dry-run",
	})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "is not a time-off type")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/time/entry/ -run TestAddCmd_TimeOff -v`
Expected: FAIL — `unknown flag: --time-off`.

- [ ] **Step 3: Add the flags**

In `internal/cli/time/entry/add.go`, add two fields to `addFlags` after `workspace int`:

```go
	timeOff     bool
	timeOffID   int
```

and register them in `newAddCmd`, immediately after the `--workspace` registration:

```go
	cmd.Flags().BoolVar(&f.timeOff, "time-off", false, "log time off / leave (time-off ID is auto-discovered from your recent leave entries)")
	cmd.Flags().IntVar(&f.timeOffID, "time-off-id", 0, "override the time-off item ID (requires --time-off)")
```

- [ ] **Step 4: Update validation**

In `runAdd`, change the `--type` requirement (currently `if f.typeName == ""`) to allow omission for time off:

```go
	// --type may be omitted for --time-off: the tenant's single time-off type
	// is used. Every other target still requires it.
	if f.typeName == "" && !f.timeOff {
		return fmt.Errorf("--type is required")
	}
```

Then extend the target-selector block so `--time-off` is a fourth selector, and add the companion-flag guards. Replace the existing target-validation block with:

```go
	// Target validation: exactly one of --ticket, --project, --workspace, --time-off.
	targetCount := 0
	if f.ticket > 0 {
		targetCount++
	}
	if f.project > 0 {
		targetCount++
	}
	if f.workspace > 0 {
		targetCount++
	}
	if f.timeOff {
		targetCount++
	}
	if targetCount != 1 {
		return fmt.Errorf("exactly one of --ticket, --project, --workspace, or --time-off is required")
	}

	if f.timeOffID > 0 && !f.timeOff {
		return fmt.Errorf("--time-off-id requires --time-off")
	}
	if f.timeOff && (f.app > 0 || f.plan > 0 || f.task > 0 || f.issue > 0) {
		return fmt.Errorf("--time-off cannot be combined with --app, --plan, --task, or --issue")
	}
```

- [ ] **Step 5: Resolve the time type and the time-off ID**

Still in `runAdd`, replace the existing time-type lookup block (the
`tt, ok := domain.FindTimeTypeByName(...)` lines) with:

```go
	var tt domain.TimeType
	if f.typeName != "" {
		var ok bool
		tt, ok = domain.FindTimeTypeByName(types, f.typeName)
		if !ok {
			return fmt.Errorf("no time type named %q", f.typeName)
		}
		if f.timeOff && !tt.IsTimeOff {
			return fmt.Errorf("time type %q is not a time-off type", tt.Name)
		}
	} else {
		// Only reachable with --time-off (validated above).
		var derr error
		tt, derr = domain.DefaultTimeOffType(types)
		if derr != nil {
			return fmt.Errorf("--type is required (could not pick a default time-off type): %w", derr)
		}
	}

	if f.timeOff {
		itemID, rerr := tsvc.ResolveTimeOffItemID(cmd.Context(), profileName, user.UID, f.timeOffID)
		if rerr != nil {
			if errors.Is(rerr, domain.ErrTimeOffIDUnknown) {
				return fmt.Errorf("couldn't determine your time-off ID — log one leave entry in the TD web UI first, or pass --time-off-id N")
			}
			return rerr
		}
		f.timeOffID = itemID
	}
```

Add `"errors"` to the import block at the top of `add.go`.

- [ ] **Step 6: Build the target and render it**

In `buildTarget`, add a first case to the switch (before `case f.ticket > 0 && f.task > 0:`) — it must come first because `--time-off` is exclusive:

```go
	case f.timeOff:
		return domain.Target{
			Kind:   domain.TargetTimeOff,
			ItemID: f.timeOffID,
		}
```

In `targetSummary`, add before the `default:` case:

```go
	case domain.TargetTimeOff:
		return fmt.Sprintf("time-off (id %d)", t.ItemID)
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `go test ./internal/cli/time/entry/ -v`
Expected: PASS — the five new `TestAddCmd_TimeOff*` tests plus every pre-existing test in the package (regression check that non-time-off behavior is unchanged).

- [ ] **Step 8: Commit**

```bash
git add internal/cli/time/entry/add.go internal/cli/time/entry/add_test.go
git commit -m "Add --time-off flag to tdx time entry add"
```

---

### Task 4: MCP — `create_time_entry` supports `kind: "timeoff"`

**Files:**
- Modify: `internal/mcp/tools_entry.go` (`createEntryArgs` struct ~line 33; `createEntryHandler` target construction ~line 197)
- Test: `internal/mcp/tools_entry_test.go`

**Interfaces:**
- Consumes: `(*timesvc.Service).ResolveTimeOffItemID` (Task 2), reached via the concrete `svcs.Time` field (`Services.Time` is `*timesvc.Service`, so no interface change is needed).
- Produces: no new Go symbols — behavior change only.

- [ ] **Step 1: Write the failing test**

Append to `internal/mcp/tools_entry_test.go`:

```go
func TestCreateEntry_TimeOffAutoDiscoversItemID(t *testing.T) {
	var postedComponent, postedProjectID int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ReferenceID":42,"UID":"uid-abc","FullName":"Test User","PrimaryEmail":"test@example.com"}`))

		// Discovery: one prior time-off entry (Component 17) carrying ProjectID 52.
		case r.URL.Path == "/TDWebApi/api/time/search":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"TimeID":7,"Component":17,"ProjectID":52,"TimeDate":"2026-05-14T00:00:00Z","Minutes":120,"TimeTypeID":3,"Uid":"uid-abc","Status":0}
			]`))

		case r.URL.Path == "/TDWebApi/api/time" && r.Method == "POST":
			var body []map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Len(t, body, 1)
			postedComponent = int(body[0]["Component"].(float64))
			postedProjectID = int(body[0]["ProjectID"].(float64))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Succeeded":[{"Index":0,"ID":777}],"Failed":[]}`))

		case r.URL.Path == "/TDWebApi/api/time/777" && r.Method == "GET":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"TimeID":777,"Component":17,"ProjectID":52,"TimeDate":"2026-06-11T00:00:00Z","Minutes":120,"Description":"PTO","TimeTypeID":3,"TimeTypeName":"Leave","Status":0,"Billable":false}`))

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	svcs := mcpHarness(t, srv.URL)
	handler := createEntryHandler(svcs)
	result, _, err := handler(context.Background(), nil, createEntryArgs{
		Date:    "2026-06-11",
		Hours:   2.0,
		TypeID:  3,
		Kind:    "timeoff",
		// ItemID deliberately omitted: it must be auto-discovered.
		Confirm: true,
	})
	require.NoError(t, err)
	require.False(t, result.IsError, "expected success, got: %v", extractText(t, result))

	// TimeOff encodes as Component 17 with the item ID carried in ProjectID.
	require.Equal(t, 17, postedComponent)
	require.Equal(t, 52, postedProjectID, "itemID should have been auto-discovered as 52")
}
```

**Test conventions in this package:** `mcpHarness(t, tenantURL) Services` is
defined in `internal/mcp/tools_auth_test.go`; handlers are invoked directly as
`handler(context.Background(), nil, <args struct>)` and their text payload read
with `extractText(t, result)` — see `TestCreateEntry_WithConfirm`
(`internal/mcp/tools_entry_test.go:115`) for the canonical shape. Ensure the test
file's imports include `context` and `encoding/json`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp/ -run TestCreateEntry_TimeOff -v`
Expected: FAIL — `postedProjectID` is `0`, not `52` (the handler passes `itemID: 0` straight through today).

- [ ] **Step 3: Document the supported kinds**

In `internal/mcp/tools_entry.go`, update the two `createEntryArgs` fields:

```go
	Kind   string `json:"kind" jsonschema:"target kind: ticket, ticketTask, project, projectTask, projectIssue, workspace, or timeoff"`
	ItemID int    `json:"itemID,omitempty" jsonschema:"work item ID; optional for kind=timeoff (auto-discovered from your recent leave entries when omitted)"`
```

- [ ] **Step 4: Auto-resolve the time-off item ID**

In `createEntryHandler`, replace the `target := domain.Target{...}` construction with:

```go
		itemID := args.ItemID
		if domain.TargetKind(args.Kind) == domain.TargetTimeOff && itemID == 0 {
			resolved, rerr := svcs.Time.ResolveTimeOffItemID(ctx, profile, user.UID, 0)
			if rerr != nil {
				return errorResult("couldn't determine the time-off ID — log one leave entry in the TD web UI first, or pass itemID explicitly."), nil, nil
			}
			itemID = resolved
		}

		target := domain.Target{
			Kind:      domain.TargetKind(args.Kind),
			ItemID:    itemID,
			AppID:     args.AppID,
			TaskID:    args.TaskID,
			ProjectID: args.ProjectID,
		}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/mcp/ -v`
Expected: PASS — the new test plus every pre-existing MCP test.

- [ ] **Step 6: Commit**

```bash
git add internal/mcp/tools_entry.go internal/mcp/tools_entry_test.go
git commit -m "Support kind=timeoff with auto-discovered itemID in create_time_entry"
```

---

### Task 5: Documentation

**Files:**
- Modify: `docs/guide/time.md` (the `tdx time entry add` reference section)
- Modify: `skills/tdx/SKILL.md` (decision tree)

**Interfaces:**
- Consumes: the finished CLI and MCP behavior from Tasks 3 and 4.
- Produces: no code symbols.

- [ ] **Step 1: Locate the entry-add reference section**

Run: `grep -n "tdx time entry add" docs/guide/time.md | head`
Read the surrounding section so the new text matches the file's existing heading level and prose style.

- [ ] **Step 2: Document the flag in the user guide**

Add to the `tdx time entry add` section of `docs/guide/time.md`:

````markdown
#### Logging time off

Use `--time-off` instead of a work target to log leave / PTO:

```bash
tdx time entry add --time-off --date 2026-06-11 --hours 2
```

`--time-off` is mutually exclusive with `--ticket`, `--project`, and
`--workspace`, and cannot be combined with `--app`, `--plan`, `--task`, or
`--issue`.

Two things are resolved for you:

- **The time type.** Omit `--type` and tdx picks the tenant's single active
  time-off type (on UF that is `Leave`). Pass `--type` explicitly if your tenant
  has more than one; passing a non-time-off type is rejected.
- **The time-off ID.** TD models time off as a tenant-specific pseudo-project.
  tdx discovers yours from your most recent leave entry in the last 180 days.
  If you have never logged leave, tdx cannot guess it — log one entry in the TD
  web UI, or pass it directly:

```bash
tdx time entry add --time-off --time-off-id 52 --date 2026-06-11 --hours 2
```

Preview first with `--dry-run`, as with any other target.
````

- [ ] **Step 3: Add it to the skill decision tree**

In `skills/tdx/SKILL.md`, add this bullet to the Decision Tree immediately after
the "I forgot to log a few hours" entry:

```markdown
- "log PTO / vacation / sick leave" / "I took time off Thursday"
  → `tdx time entry add --time-off --date <YYYY-MM-DD> --hours N` — the time-off
    ID and the Leave time type are auto-resolved (`--time-off-id N` overrides the
    ID; `--type` overrides the type). Preview with `--dry-run`.
```

- [ ] **Step 4: Verify the docs match reality**

Run: `go build -o /tmp/tdx-doccheck ./cmd/tdx && /tmp/tdx-doccheck time entry add --help`
Expected: the help output lists `--time-off` and `--time-off-id` with the wording used in the docs. Fix any drift in the docs, not the help text.

- [ ] **Step 5: Commit**

```bash
git add docs/guide/time.md skills/tdx/SKILL.md
git commit -m "Document tdx time entry add --time-off"
```

---

## Final Verification

- [ ] **Full build, test, vet, lint**

```bash
go build ./... && go test ./... && go vet ./... && gofmt -l .
golangci-lint run ./...
```
Expected: tests pass, `gofmt -l .` prints nothing, golangci-lint reports `0 issues`.

- [ ] **Live acceptance check (UF tenant, requires a valid token)**

This is the real gate — unit tests cannot prove the TD wire format is right.

```bash
tdx auth status --json                     # confirm tokenValid: true

# 1. Dry run: must print a time-off target with a discovered id.
tdx time entry add --time-off --date 2026-06-11 --hours 1 --dry-run

# 2. Create for real, then confirm it landed as Leave.
tdx time entry add --time-off --date 2026-06-11 --hours 1 -d "plan verification"
tdx time entry list --week 2026-06-07 --json | \
  python3 -c "import json,sys; [print(e['id'], e['date'][:10], e['timeType']['name']) for e in json.load(sys.stdin)['entries'] if 'leave' in e['timeType']['name'].lower()]"

# 3. Roll it back.
tdx time entry delete <id-from-step-2> --yes
```
Expected: step 1 shows `time-off (id 52)` and `kind=timeoff`; step 2 lists the new
entry with type `Leave`; step 3 removes it.

- [ ] **Finish the branch**

Use ultrapowers:finishing-a-development-branch. Branch is `feat/time-off-flag`;
the spec commit is already on it.

## Self-Review Notes

Checked against `docs/specs/2026-06-30-tdx-time-off-flag-design.md`:

- Spec §1 (CLI flags, 4-way mutex, buildTarget, targetSummary) → Task 3.
- Spec §2 (ResolveTimeOffItemID, override, 180-day lookback, sentinel error) → Task 2.
- Spec §3 (IsTimeOff field, DefaultTimeOffType, `--type` validation/defaulting) → Tasks 1 and 3.
- Spec §4 (MCP parity, corrected to `create_time_entry`) → Task 4.
- Spec §5 error table → covered by Task 3 Steps 4–5 and Task 4 Step 4; each row has a test in Task 3 Step 1 except the two guarded by existing shared code (`ErrDayLocked`, `ErrWeekSubmitted`), which are untouched by this change.
- Spec §6 data flow → the order in Task 3 Steps 4–6 matches it exactly.
- Spec "Testing" section → Tasks 1–4 test steps, plus the live acceptance check above.
- Spec "Out of scope" (week `add-row`, profile caching) → no task touches either.

Ambiguities the spec left open are pinned in **Pinned Decisions** above rather
than inherited silently.
