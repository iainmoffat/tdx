# `tdx time report status` — Time Status Report

**Date:** 2026-04-30
**Status:** Approved (brainstorming complete)
**Owner:** ipm
**Type:** New feature (Phase C — Reports kickoff)

---

## 0. Decisions log

| # | Decision |
|---|---|
| Q1 | Selectors are strictly exclusive: exactly one of `--user`, `--manager`, `--account`, `--all` is required. Any combination is a CLI validation error before any API call. (`--user` may be repeated to specify multiple UIDs in a single invocation; that's still "one selector".) |
| Q2 | Drop the spec's proposed `SummarizeBillable(report, types)` helper. TD's wire format already returns `MinutesBillable` and `MinutesNonBillable` per report; plumb those through `domain.WeekReport` directly. Avoids duplicating server logic and a class of client-side billable-bucketing bugs. |
| Q3 | Use `golang.org/x/sync/errgroup` for bounded-concurrency fan-out. Adds one well-known dep; net win in clarity vs. a hand-rolled semaphore. |
| Q4 | Permission errors (TD 401/403) map to a typed `domain.ErrPermission` returned by `timesvc.GetWeekReportForUser`. CLI surfaces them with an actionable message ("you need the Analysis app or to be the user/their approver"). |
| Q5 | `domain.User` is extended with `ReportsToUID`, `ReportsToID`, `ReportsToName`, `ReportsToEmail`. Existing `Email` field is reused. The new fields are optional (omit-empty JSON tags) so existing callers (auth/whoami) continue to work without populating them. |
| Q6 | Output formats: human table (default), `--json`, `--csv` (stdout), `--xlsx PATH` (file). Mutually exclusive — only one format flag at a time. CSV uses stdlib `encoding/csv`. XLSX adds `github.com/xuri/excelize/v2`. Subtotals are NOT injected as rows in either CSV or XLSX (let pivots/SUMIF handle it); they remain in the human/JSON outputs only. |

---

## 1. Goal

Add `tdx time report status`, a read-only CLI command (and matching MCP tool) that produces a roll-up of weekly time-report status across multiple users. Mirrors TeamDynamix's built-in **Work Management → Analysis → Standard Reports → Time Status Report**: rows of `(user, week)` with billable/non-billable/total hours and the report's submission status.

Non-goals: write paths (no "approve a timesheet" — TD doesn't expose it), per-entry detail (the report is a roll-up), TUI/grid rendering.

---

## 2. Affected files

### Modify

- `internal/domain/week.go` — add `MinutesBillable int` and `MinutesNonBillable int` to `WeekReport`. Add `BillableHours()` and `NonBillableHours()` methods.
- `internal/domain/user.go` — add `ReportsToUID`, `ReportsToID`, `ReportsToName`, `ReportsToEmail` fields. Update existing tests if they assert struct equality.
- `internal/domain/errors.go` — add `ErrPermission` sentinel error.
- `internal/svc/timesvc/week.go` — add `GetWeekReportForUser(ctx, profile, date, uid string)`. Extend `wireTimeReport` decoding to populate `MinutesBillable`/`MinutesNonBillable`.
- `internal/svc/timesvc/types.go` — verify `wireTimeReport` already has the billable fields (it does); no changes here unless missing.
- `internal/cli/time/time.go` — register the new `report` subcommand.
- `README.md` — add Time Reports subtable, MCP tool row, JSON schema "Phase C — Reports" line.
- `docs/guide.md` — new "Time Reports" section.
- `go.mod` / `go.sum` — add `golang.org/x/sync` and `github.com/xuri/excelize/v2`.

### Create

- `internal/domain/time_status_report.go` — `WeekStatusRow`, `TimeStatusReport` types.
- `internal/domain/time_status_report_test.go` — table-driven tests.
- `internal/domain/user_filter.go` — `UserFilter` type used by `peoplesvc.SearchUsers`.
- `internal/svc/timesvc/week_user.go` — `GetWeekReportForUser` (kept separate from `week.go` for focus). Permission-error mapping lives here.
- `internal/svc/timesvc/week_user_test.go` — httptest-based tests.
- `internal/svc/peoplesvc/service.go` — new package `peoplesvc` with `Service` struct, `New(paths)`, `clientFor(profileName)`.
- `internal/svc/peoplesvc/users.go` — `GetUser`, `SearchUsers`.
- `internal/svc/peoplesvc/users_test.go` — httptest tests.
- `internal/svc/peoplesvc/types.go` — wire types for `/api/people/{uid}` and `/api/people/search`.
- `internal/cli/time/report/report.go` — parent `report` cobra command.
- `internal/cli/time/report/status.go` — `tdx time report status` command + flag wiring.
- `internal/cli/time/report/status_test.go` — cobra tests.
- `internal/cli/time/report/print.go` — table + JSON renderers.
- `internal/cli/time/report/csv.go` — CSV writer (stdlib `encoding/csv`).
- `internal/cli/time/report/xlsx.go` — XLSX writer (excelize).
- `internal/cli/time/report/csv_test.go`, `internal/cli/time/report/xlsx_test.go` — table-driven tests.
- `internal/cli/time/report/runner.go` — orchestration: resolve users, fan out per-(user,week) calls with bounded concurrency, assemble `TimeStatusReport`.
- `internal/cli/time/report/runner_test.go` — orchestration tests with a mocked `timesvc`/`peoplesvc` interface.
- `internal/mcp/tools_report.go` — `get_time_status_report` registration + handler.
- `internal/mcp/tools_report_test.go` — handler tests.

---

## 3. Domain delta

### `WeekReport` extension

```go
// internal/domain/week.go
type WeekReport struct {
    WeekRef            WeekRef      `json:"weekRef"`
    UserUID            string       `json:"userUID"`
    TotalMinutes       int          `json:"totalMinutes"`
    MinutesBillable    int          `json:"minutesBillable,omitempty"`    // NEW
    MinutesNonBillable int          `json:"minutesNonBillable,omitempty"` // NEW
    Status             ReportStatus `json:"status"`
    Days               []DaySummary `json:"days"`
    Entries            []TimeEntry  `json:"entries"`
}

func (w WeekReport) BillableHours() float64    { return float64(w.MinutesBillable) / 60.0 }
func (w WeekReport) NonBillableHours() float64 { return float64(w.MinutesNonBillable) / 60.0 }
```

### `User` extension

```go
// internal/domain/user.go
type User struct {
    ID             int    `json:"id,omitempty"             yaml:"id,omitempty"`
    UID            string `json:"uid,omitempty"            yaml:"uid,omitempty"`
    FullName       string `json:"fullName,omitempty"       yaml:"fullName,omitempty"`
    Email          string `json:"email,omitempty"          yaml:"email,omitempty"`
    Active         bool   `json:"active,omitempty"         yaml:"active,omitempty"`           // NEW (used by SearchUsers)
    AccountName    string `json:"accountName,omitempty"    yaml:"accountName,omitempty"`      // NEW
    ReportsToUID   string `json:"reportsToUID,omitempty"   yaml:"reportsToUID,omitempty"`     // NEW
    ReportsToID    int    `json:"reportsToID,omitempty"    yaml:"reportsToID,omitempty"`      // NEW
    ReportsToName  string `json:"reportsToName,omitempty"  yaml:"reportsToName,omitempty"`    // NEW
    ReportsToEmail string `json:"reportsToEmail,omitempty" yaml:"reportsToEmail,omitempty"`   // NEW
}
```

### New types

```go
// internal/domain/time_status_report.go
type WeekStatusRow struct {
    WeekRef        WeekRef      `json:"weekRef"`
    User           User         `json:"user"`
    Status         ReportStatus `json:"status"`
    BillableMin    int          `json:"billableMinutes"`
    NonBillableMin int          `json:"nonBillableMinutes"`
    TotalMin       int          `json:"totalMinutes"`
}

func (r WeekStatusRow) BillableHours() float64    { /* ... */ }
func (r WeekStatusRow) NonBillableHours() float64 { /* ... */ }
func (r WeekStatusRow) TotalHours() float64       { /* ... */ }

type TimeStatusReport struct {
    From  WeekRef         `json:"from"`
    To    WeekRef         `json:"to"`
    Rows  []WeekStatusRow `json:"rows"`
}
```

```go
// internal/domain/user_filter.go
type UserFilter struct {
    Active      *bool  // nil = "no filter", true/false = filter
    UserType    string // default "User"
    AccountID   int    // 0 = no filter
    AccountName string // empty = no filter
    NameLike    string // empty = no filter
    Limit       int    // 0 = client default (100)
}
```

### Error sentinel

```go
// internal/domain/errors.go
var ErrPermission = errors.New("permission denied")
```

---

## 4. Service: timesvc

`GetWeekReportForUser(ctx, profile, date, uid)` mirrors `GetWeekReport` but hits `/api/time/report/{date}/{uid}`. Reuses `wireTimeReport` decoder, `decodeTimeEntry`, `decodeReportStatus`, `buildDaySummaries`, `resolveTimeTypeNames`, EasternTZ midnight normalization.

Permission mapping: when the TD client returns an error containing the strings `401` or `403`, wrap with `domain.ErrPermission` (use `errors.Is` at the call site).

`MinutesBillable`/`MinutesNonBillable` are read directly from `wireTimeReport.MinutesBillable` / `MinutesNonBillable` and assigned to the returned `WeekReport`. **Both `GetWeekReport` and `GetWeekReportForUser` populate them** — backfill the existing `GetWeekReport` so all callers benefit.

---

## 5. Service: peoplesvc

New package at `internal/svc/peoplesvc/`. Mirrors `timesvc`'s shape:

```go
type Service struct {
    paths config.Paths
}

func New(paths config.Paths) *Service { return &Service{paths: paths} }

func (s *Service) clientFor(profileName string) (*tdx.Client, error) { /* same pattern */ }

// GetUser fetches a single user by UID.
func (s *Service) GetUser(ctx context.Context, profileName, uid string) (domain.User, error)

// SearchUsers returns users matching the filter. Default Active=true, UserType="User", Limit=100.
func (s *Service) SearchUsers(ctx context.Context, profileName string, filter domain.UserFilter) ([]domain.User, error)
```

Wire types in `peoplesvc/types.go`:

```go
type wireUser struct {
    UID                 string `json:"UID"`
    ID                  int    `json:"ID"`
    FullName            string `json:"FullName"`
    PrimaryEmail        string `json:"PrimaryEmail"`
    AlternateEmail      string `json:"AlternateEmail"`
    IsActive            bool   `json:"IsActive"`
    DefaultAccountName  string `json:"DefaultAccountName"`
    ReportsToUid        string `json:"ReportsToUid"`
    ReportsToId         int    `json:"ReportsToId"`
    ReportsToFullName   string `json:"ReportsToFullName"`
    ReportsToEmail      string `json:"ReportsToEmail"`
}

type wireUserSearch struct {
    NameLike   string   `json:"NameLike,omitempty"`
    IsActive   *bool    `json:"IsActive,omitempty"`
    AccountIDs []int    `json:"AccountIDs,omitempty"`
    UserType   string   `json:"UserType,omitempty"`
    MaxResults int      `json:"MaxResults,omitempty"`
}
```

`decodeUser(w wireUser) domain.User` maps fields. `PrimaryEmail` falls back to `AlternateEmail` if empty.

---

## 6. CLI: `tdx time report status`

### Files

- `internal/cli/time/report/report.go` — parent `report` cobra command, single-line `Use: "report"`, `Short: "Generate time-related reports"`. AddCommand(newStatusCmd()).
- `internal/cli/time/report/status.go` — `newStatusCmd()` builds the cobra command with all flags and wires `RunE` to call into `runner.go`.
- `internal/cli/time/report/runner.go` — `runStatus(ctx, opts)` orchestration:
  1. Resolve profile.
  2. Validate selector exclusivity (return error if zero or >1 selectors).
  3. Resolve user list:
     - `--user uid1,uid2,...` → split, look up each via `peoplesvc.GetUser` (parallelized with errgroup, semaphore=5).
     - `--manager me` → `authsvc.WhoAmI` to get caller's UID, then `peoplesvc.SearchUsers(filter{...})` filtered client-side by `ReportsToUID == me.UID` (TD's `UserSearch` doesn't directly accept ReportsToUID; we filter the result).
     - `--manager <uid>` → same, filtered by that UID.
     - `--account <name>` → `peoplesvc.SearchUsers(filter{AccountName: name, Active: ptr(true)})`.
     - `--all` → `peoplesvc.SearchUsers(filter{Active: ptr(true), Limit: --limit or default 200})` plus `--yes` confirmation gate.
  4. Resolve week range: `--week` gives one week; `--from`/`--to` gives a range (normalized to whole Sunday–Saturday weeks covering the range).
  5. For each (user, week) pair, fan out `timesvc.GetWeekReportForUser` with errgroup semaphore=5. On `ErrPermission`, append a warning row with placeholder zeros + `Status: "permission-denied"` (the user can't see the report; we still acknowledge them in output). On any other error, fail the whole run.
  6. Apply `--include-zero` filter (default: include).
  7. Apply `--limit` cap.
  8. Sort: by week ascending, then by user name within week.
  9. Render via `print.go`.

- `internal/cli/time/report/print.go` — text + JSON renderers:
  - `printText(w io.Writer, report TimeStatusReport)` — for each week: header `WEEK YYYY-MM-DD — YYYY-MM-DD`, then a `render.Table` with columns `NAME | EMAIL | REPORTS TO | STATUS | BILL | NON-BILL | TOTAL`, then a per-week subtotal row. After all weeks, an overall totals block.
  - `printJSON(w io.Writer, report TimeStatusReport)` — emits the `tdx.v1.timeStatusReport` envelope.

- `internal/cli/time/report/csv.go` — `writeCSV(w io.Writer, report TimeStatusReport) error`. Header row: `weekStart,weekEnd,userUID,name,email,reportsToName,reportsToEmail,status,billableHours,nonBillableHours,totalHours`. One row per `WeekStatusRow`. No subtotal rows. Hours formatted to 2 decimal places. Uses stdlib `encoding/csv`.

- `internal/cli/time/report/xlsx.go` — `writeXLSX(path string, report TimeStatusReport) error`. Single sheet "Time Status Report". First row is a bold header (column names same as the CSV). One data row per `WeekStatusRow`. Frozen top row. Auto-width-ish columns (excelize doesn't auto-fit; set sensible static widths). Hours formatted as numbers (not strings) so users can sum/pivot. Uses `github.com/xuri/excelize/v2`.

### Flags

```
--profile string         profile name (default: active)
--week string            any date in target week (YYYY-MM-DD); mutually exclusive with --from/--to
--from string            range start (YYYY-MM-DD)
--to string              range end (YYYY-MM-DD)
--user stringSlice       user UIDs (repeatable / comma-separated)
--manager string         limit to direct reports of this UID; "me" = authenticated user
--account string         limit to users in this account/department by name
--all                    every active standard user (requires --yes)
--yes                    confirm --all (avoids accidental large queries)
--include-zero           include user-weeks with zero total minutes (default: include)
--limit int              cap user count (default: 200, hard cap: 1000)
--json                   emit JSON to stdout
--csv                    emit CSV to stdout
--xlsx string            write XLSX to this file path
```

### Selector validation

- Exactly one of `{--user, --manager, --account, --all}` must be provided. Zero or multiple → error.
- `--all` requires `--yes`. Without `--yes`, error: `--all is destructively large; pass --yes to confirm`.

### Format flag exclusivity

- At most one of `{--json, --csv, --xlsx}` may be set. Multiple → error before any API call. None set → human table output to stdout.

### JSON envelope

```go
type timeStatusReportJSON struct {
    Schema string                     `json:"schema"` // "tdx.v1.timeStatusReport"
    Filter timeStatusReportFilterJSON `json:"filter"`
    Weeks  []weekGroupJSON            `json:"weeks"`
    Totals weekTotalsJSON             `json:"totals"`
}

type timeStatusReportFilterJSON struct {
    Selector string   `json:"selector"`            // "user" | "manager" | "account" | "all"
    Users    []string `json:"users,omitempty"`
    Manager  string   `json:"manager,omitempty"`
    Account  string   `json:"account,omitempty"`
    From     string   `json:"from"`
    To       string   `json:"to"`
}

type weekGroupJSON struct {
    WeekStart string              `json:"weekStart"`
    WeekEnd   string              `json:"weekEnd"`
    Rows      []weekStatusRowJSON `json:"rows"`
    Subtotals weekTotalsJSON      `json:"subtotals"`
}

type weekStatusRowJSON struct {
    UserUID          string  `json:"userUID"`
    Name             string  `json:"name"`
    Email            string  `json:"email"`
    ReportsToName    string  `json:"reportsToName,omitempty"`
    ReportsToEmail   string  `json:"reportsToEmail,omitempty"`
    Status           string  `json:"status"` // ReportStatus value
    BillableHours    float64 `json:"billableHours"`
    NonBillableHours float64 `json:"nonBillableHours"`
    TotalHours       float64 `json:"totalHours"`
}

type weekTotalsJSON struct {
    BillableHours    float64 `json:"billableHours"`
    NonBillableHours float64 `json:"nonBillableHours"`
    TotalHours       float64 `json:"totalHours"`
}
```

---

## 7. MCP tool

`get_time_status_report` — read-only, no `confirm` gate.

```go
type getTimeStatusReportArgs struct {
    Profile     string   `json:"profile,omitempty"`
    Week        string   `json:"week,omitempty"`        // YYYY-MM-DD or empty
    From        string   `json:"from,omitempty"`
    To          string   `json:"to,omitempty"`
    UserUIDs    []string `json:"userUIDs,omitempty"`
    Manager     string   `json:"manager,omitempty"`     // UID or "me"
    Account     string   `json:"account,omitempty"`
    All         bool     `json:"all,omitempty"`
    IncludeZero bool     `json:"includeZero,omitempty"`
    Limit       int      `json:"limit,omitempty"`
}
```

Returns the same `tdx.v1.timeStatusReport` envelope. Tool count goes up by one (read-only); the implementer reads the README's current count and bumps both the prose number and the `wantCount` constant in `internal/mcp/server_test.go`.

---

## 8. Concurrency + rate limiting

- Use `golang.org/x/sync/errgroup` with semaphore size 5 for fan-out (per-user GetUser lookups, per-(user,week) report fetches).
- TD documents 60 req / 60 sec for `/time/report`. Five concurrent requests respects this with margin.
- Retry-on-429: a small generic helper in `internal/tdx/client.go` — if response is 429, sleep for `Retry-After` seconds (default 5s) plus a jittered 0–2s, retry up to 3 times. Retry helper is opt-in: `client.DoJSONWithRetry(...)` is a wrapper. `GetWeekReportForUser` uses it.
- Other endpoints (search, GetUser) keep using `DoJSON` for now — fix when/if rate limits become an issue.

---

## 9. Tests

### Domain (`internal/domain/time_status_report_test.go`)

Table-driven tests covering:
- Zero-row report → empty Rows + zero totals.
- Mixed billable/non-billable → BillableHours/NonBillableHours/TotalHours math.
- Status decoding (NoStatus, InProgress, Submitted, Rejected, Approved).
- DST week-boundary handling: Sunday→Saturday range stays correct on spring-forward / fall-back weeks.

### timesvc (`week_user_test.go`)

- Happy path: GET `/api/time/report/2026-04-12/{uid}` returns a normal `wireTimeReport` → decoded `WeekReport` has correct UserUID + billable totals.
- 401 → returns error wrapping `ErrPermission`.
- 403 → returns error wrapping `ErrPermission`.
- Verify `MinutesBillable` and `MinutesNonBillable` populated.

### peoplesvc (`users_test.go`)

- `GetUser`: GET `/api/people/{uid}` → decoded User with manager fields.
- `SearchUsers`: POST `/api/people/search` with body containing the filter; default UserType=User, Active=true, MaxResults=100.
- Email fallback: PrimaryEmail empty → uses AlternateEmail.

### CLI (`status_test.go`, `runner_test.go`, `csv_test.go`, `xlsx_test.go`)

- `runner_test.go` uses interface-based mocks for `timesvc`/`peoplesvc`/`authsvc` (define minimal interfaces in `runner.go` so tests can substitute). Cover:
  - Single user, single week.
  - Single user, multi-week range.
  - `--manager me` resolves authenticated UID and filters direct reports.
  - `--all` without `--yes` errors out.
  - Selector validation: zero / multiple selectors error.
  - Format flag exclusivity: `--json --csv` errors out.
  - Permission error → row appears with `Status: "permission-denied"` and zero hours; run continues.
  - JSON output schema matches.
- `csv_test.go`: write a small `TimeStatusReport`, parse the output back via `encoding/csv`, assert header row + per-row fields + 2-decimal hour formatting.
- `xlsx_test.go`: write a small report to a temp `.xlsx`, re-open with excelize, assert header is bold, sheet name, row count, and that hour cells are numeric (not string).
- `status_test.go` covers cobra flag registration only.

### MCP (`tools_report_test.go`)

- `get_time_status_report` no-confirm gate (read-only).
- Schema validation on output.

---

## 10. Docs

- **README.md** Time Reports subtable:
  ```
  | `tdx time report status` | Weekly time-status report (per user, per week) | `--week`, `--from`/`--to`, `--user`, `--manager`, `--account`, `--all`, `--include-zero`, `--limit`, `--json`, `--csv`, `--xlsx` |
  ```
- **README.md** MCP read-only tools table: add `get_time_status_report` row. Update count: "Read-only (N tools)" → +1.
- **README.md** JSON schema list: add new line `Schema names introduced in Phase C — Reports: tdx.v1.timeStatusReport.`
- **README.md** Quick Start: add example `tdx time report status --manager me --week 2026-04-12`.
- **docs/guide.md** new section "Time Reports" with subsection "Time Status Report" — column meanings, permission requirements (Analysis app or approver), example invocations, JSON shape.

---

## 11. Out of scope

- Write paths (no API for approving timesheets).
- TUI grid view; flat table only.
- Per-entry detail view (this is a roll-up).
- Caching / memoization of reports between invocations.
- Per-account aggregate roll-ups (e.g. "show totals by department"). Could be a follow-up.
- Historical data export. The CLI prints; `--json` is the export path.
- *(Both `--csv` and `--xlsx` are now in scope; see §6.)*

---

## 12. Estimated work

~15 commits across 5 themes (domain → timesvc → peoplesvc → cli → mcp + docs):

1. Domain: extend `User` + add `UserFilter`, `WeekStatusRow`, `TimeStatusReport`, `ErrPermission`. Add `MinutesBillable`/`MinutesNonBillable` to `WeekReport`.
2. timesvc: extend wire decoding for billable totals; add `GetWeekReportForUser` with permission-error mapping.
3. timesvc: retry-on-429 helper in `internal/tdx/client.go` (opt-in `DoJSONWithRetry`).
4. peoplesvc package: types + GetUser + SearchUsers + tests.
5. go.mod: add `golang.org/x/sync` and `github.com/xuri/excelize/v2`.
6. CLI: `report.go` + `status.go` (flag wiring + cobra); register in `time.go`.
7. CLI: `runner.go` — orchestration with errgroup fan-out.
8. CLI: `print.go` — table + JSON output.
9. CLI: `csv.go` + `csv_test.go`.
10. CLI: `xlsx.go` + `xlsx_test.go`.
11. CLI tests: `status_test.go`, `runner_test.go`.
12. MCP: `tools_report.go` + tests.
13. README + guide.md updates (include `--csv`/`--xlsx` examples).
14. Live verification against sample tenant (text, JSON, CSV, XLSX).
15. Final quality gate, push branch, open PR, tag v0.9.0 (minor bump — new public CLI surface + new MCP tool).

Subagent-driven execution recommended given the surface size (multiple new packages + CLI tree).
