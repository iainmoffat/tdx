# tdx Phase 2 — Read-only time operations

**Status:** design approved, ready for implementation plan
**Owner:** ipm
**Date:** 2026-04-10
**Preceded by:** [`2026-04-10-tdx-framework-design.md`](2026-04-10-tdx-framework-design.md) (framework spec) and [`2026-04-10-tdx-phase-1-auth-foundation.md`](../plans/2026-04-10-tdx-phase-1-auth-foundation.md) (Phase 1 plan, merged on 2026-04-10).

## 1. Goal

Ship the v1 shortlist of read-only time commands so tdx is already useful for daily "what did I log?" workflows before any write path exists. All commands must work end-to-end against the real UFL TeamDynamix tenant. No writes, no templates, no pagination beyond a `--limit` cap.

### Exit criteria

- `tdx time entry list` returns this week's entries for the signed-in user with zero flags.
- `tdx time week show` prints a Sun–Sat grid matching the layout of TD's web app week view.
- `tdx time type list` returns the tenant's visible time types.
- `tdx time type for ticket 12345 --app 42` returns the types valid for that work item.
- `tdx auth status` prints `user:` and `email:` lines when authenticated.
- A manual walkthrough at `docs/manual-tests/phase-2-read-ops-walkthrough.md` runs all commands against real UFL and passes.

## 2. Scope

### In scope

| Command | Purpose |
|---|---|
| `tdx time entry list` | List entries, default filter = this week + me |
| `tdx time entry show <id>` | Single entry detail |
| `tdx time week show [date]` | Row × Day grid for target week |
| `tdx time week locked [--from --to]` | Locked days in a range |
| `tdx time type list` | All visible time types |
| `tdx time type for <kind> <id> --app <app>` | Time types valid for a work item |
| `tdx auth status` (update) | Add `user:` and `email:` lines via whoami |

### Explicitly deferred

- `tdx time entry recent` — post-shortlist
- `tdx time type show <id>` — post-shortlist
- `tdx time week compare` — needs template domain types → Phase 4
- Full pagination of `/time/search` — add `--all` in a follow-up only if the verified request body supports it
- Fuzzy `--type` matching — exact case-insensitive only
- TD response caching beyond per-process whoami
- Entry write paths (Phase 3)

## 3. Architecture

Fits inside the framework spec §5.8 layering (`domain → tdx → svc → cli`). Every new artifact lands in one of those layers.

### 3.1 Domain types (new)

In `internal/domain/`, zero imports. Each type gets a `Validate()` method where applicable and a sentinel error if it has one.

```go
type User struct {
    ID       int
    UID      string   // TD GUID
    FullName string
    Email    string
}

type TargetKind string
const (
    TargetTicket       TargetKind = "ticket"
    TargetTicketTask   TargetKind = "ticketTask"
    TargetProjectTask  TargetKind = "projectTask"
    TargetProjectIssue TargetKind = "projectIssue"
    TargetWorkspace    TargetKind = "workspace"
    TargetRequest      TargetKind = "request"
    TargetTimeOff      TargetKind = "timeoff"
)

type Target struct {
    Kind        TargetKind
    AppID       int
    ItemID      int
    TaskID      int    // zero if not applicable
    DisplayName string
    DisplayRef  string // e.g. "#12345"
}

type TimeType struct {
    ID            int
    Name          string
    Description   string
    Billable      bool
    Limited       bool
    ComponentKind string
}

type ReportStatus string
const (
    ReportOpen      ReportStatus = "open"
    ReportSubmitted ReportStatus = "submitted"
    ReportApproved  ReportStatus = "approved"
)

type TimeEntry struct {
    ID           int
    UserUID      string
    Target       Target
    TimeType     TimeType
    // Date is the entry date. Stored as midnight in America/New_York so
    // equality and range comparisons across the codebase are consistent.
    // JSON marshals as "YYYY-MM-DD" via a custom MarshalJSON.
    Date         time.Time
    Minutes      int
    Description  string
    Billable     bool
    CreatedAt    time.Time // full instant, UTC from TD
    ModifiedAt   time.Time // full instant, UTC from TD
    ReportStatus ReportStatus
}

type DateRange struct {
    From time.Time // inclusive
    To   time.Time // inclusive
}

type EntryFilter struct {
    DateRange  DateRange
    UserUID    string
    Target     *Target
    TimeTypeID int
    Limit      int
}

type WeekRef struct {
    StartDate time.Time // Sunday of target week, America/New_York
    EndDate   time.Time // Saturday of target week, America/New_York
}

type DaySummary struct {
    Date    time.Time
    Minutes int
    Locked  bool
}

type WeekReport struct {
    WeekRef      WeekRef
    UserUID      string
    TotalMinutes int
    Status       ReportStatus
    Days         []DaySummary // always seven, Sun..Sat
    Entries      []TimeEntry
}

type LockedDay struct {
    Date   time.Time
    Reason string
}
```

`Session.User` (already defined in framework spec §5.1) is populated from the whoami response. Phase 1 left it zero-valued.

**New sentinel errors** in `internal/domain/errors.go`:

```go
var ErrEntryNotFound          = errors.New("time entry not found")
var ErrUnsupportedTargetKind  = errors.New("unsupported target kind")
```

### 3.2 Time zone

**All dates are computed in `America/New_York`**, regardless of laptop clock. This matches UFL's billing week.

```go
// internal/domain/tz.go
var EasternTZ *time.Location

func init() {
    loc, err := time.LoadLocation("America/New_York")
    if err != nil {
        // The embedded tzdata safety net in cmd/tdx/main.go guarantees
        // this branch never fires in the shipped binary. If we see this
        // panic, the import in main.go is missing or broken.
        panic("tdx: failed to load America/New_York: " + err.Error())
    }
    EasternTZ = loc
}
```

`cmd/tdx/main.go` adds `import _ "time/tzdata"` to embed the tz database into the binary, so `LoadLocation` works even on minimal container images without system tzdata. This is ~450 KB added to the binary and is worth it for the portability guarantee.

### 3.3 Week computation

Spec framework §5.4 said "Monday start"; Phase 2 amends this to "Sunday start" to match TD's web app layout. Recorded in §7 Decision Log.

```go
// WeekRefContaining returns the Sun..Sat week containing the given instant.
// Always computes in America/New_York.
func WeekRefContaining(t time.Time) WeekRef {
    easternT := t.In(EasternTZ)
    // Shift back to Sunday (weekday 0 in Go).
    offset := int(easternT.Weekday())
    start := time.Date(easternT.Year(), easternT.Month(), easternT.Day()-offset,
        0, 0, 0, 0, EasternTZ)
    end := start.AddDate(0, 0, 6)
    return WeekRef{StartDate: start, EndDate: end}
}
```

### 3.4 tdx client extension

`internal/tdx/client.go` gains one helper:

```go
// DoJSON performs an authenticated request. If body is non-nil it is JSON-encoded
// and sent with Content-Type: application/json. On 2xx, the response body is
// decoded into out (if non-nil). All error semantics (ErrUnauthorized, APIError,
// 429 retry) are inherited from Do.
func (c *Client) DoJSON(ctx context.Context, method, path string, body, out any) error
```

No typed domain methods on the client. The client stays thin.

### 3.5 timesvc (new package)

`internal/svc/timesvc/service.go`:

```go
package timesvc

type Service struct {
    paths    config.Paths
    profiles *config.ProfileStore
    creds    *config.CredentialsStore
}

func New(paths config.Paths) *Service

// Each read method resolves a fresh tdx.Client per call (picks up token changes).
func (s *Service) ListTimeTypes(ctx context.Context, profileName string) ([]domain.TimeType, error)
func (s *Service) TimeTypesForTarget(ctx context.Context, profileName string, target domain.Target) ([]domain.TimeType, error)
func (s *Service) SearchEntries(ctx context.Context, profileName string, filter domain.EntryFilter) ([]domain.TimeEntry, error)
func (s *Service) GetEntry(ctx context.Context, profileName string, id int) (domain.TimeEntry, error)
func (s *Service) GetWeekReport(ctx context.Context, profileName string, date time.Time) (domain.WeekReport, error)
func (s *Service) GetLockedDays(ctx context.Context, profileName string, from, to time.Time) ([]domain.LockedDay, error)
```

**Endpoint mapping** (all under `/TDWebApi/api/`):

| Method | TD path | Notes |
|---|---|---|
| `ListTimeTypes` | `GET /time/types` | Body: `TimeType[]` |
| `TimeTypesForTarget` | `GET /time/types/component/app/{appID}/ticket/{ticketID}` (and siblings) | Exact path per kind verified from `ReferenceMaterial/Time` before implementation |
| `SearchEntries` | `POST /time/search` | Request body = `TimeEntryFilter`, exact shape verified from reference before implementation. `EntryFilter.Limit` becomes `MaxResults` in the request body if TD supports it, otherwise client truncates after receiving. Default 100 |
| `GetEntry` | `GET /time/{id}` | 404 → `ErrEntryNotFound` |
| `GetWeekReport` | `GET /time/report/{date}` | `date` is any day in target week, TD normalizes |
| `GetLockedDays` | `GET /time/locked?startDate=YYYY-MM-DD&endDate=YYYY-MM-DD` | Range inclusive |

Each service method uses `DoJSON` and wraps errors with context (`fmt.Errorf("list time types: %w", err)`).

### 3.6 authsvc.WhoAmI (new method)

`internal/svc/authsvc/service.go` grows:

```go
func (s *Service) WhoAmI(ctx context.Context, profileName string) (domain.User, error)
```

Hits `GET /TDWebApi/api/auth/getuser` via `DoJSON`. The exact struct that decodes the response is written after probing the live tenant with a real token (Step 0 of the owning task). Result is cached on an in-memory field of `Service` keyed by profile name — cleared when the process exits. Never persisted.

`authsvc.Status` is updated to call `WhoAmI` when `Authenticated && TokenValid` and populate a new `User domain.User` field on the returned `Status` struct. Whoami failure is non-fatal: `Status` returns success, populates `Status.User.Error` (new field) with the error string, and lets `tdx auth status` render it as a degraded line.

### 3.7 Renderers (new helpers in internal/render)

```go
// Table writes a left-aligned, column-padded table with a header row and optional summary row.
func Table(w io.Writer, headers []string, rows [][]string, summary []string)

// WeekGrid writes the Row × Day grid defined in framework spec §7.1.
// Always seven columns (Sun..Sat). Rows are grouped by (Target.DisplayRef, TimeType.Name).
func WeekGrid(w io.Writer, report domain.WeekReport)
```

No third-party dependencies. Phase 1's `JSON` and `Humanf` are unchanged and reused.

### 3.8 CLI subtree

New `internal/cli/time/`:

```
internal/cli/time/
├── time.go             // NewCmd() returns `time` parent
├── entry/
│   ├── entry.go        // `entry` parent
│   ├── list.go         // list.NewCmd()
│   └── show.go
├── week/
│   ├── week.go
│   ├── show.go
│   └── locked.go
└── type/
    ├── type.go
    ├── list.go
    └── for_target.go
```

`internal/cli/root.go` grows one import and one `AddCommand` line to wire the `time` subtree.

Every CLI command:
- Resolves the active profile via `authsvc.ResolveProfile`.
- Constructs a `timesvc.Service` with the current paths.
- Calls the relevant service method.
- Renders via `render.Table` / `render.WeekGrid` / `render.JSON` based on `--json` flag (routed through the Phase-1 `render.ResolveFormat`).

## 4. Per-command behavior

### 4.1 `tdx time entry list`

**Flags:**

| Flag | Purpose | Notes |
|---|---|---|
| `--week YYYY-MM-DD` | Any date inside target week | Mutually exclusive with `--from/--to` |
| `--from YYYY-MM-DD` | Range start (inclusive) | Requires `--to` |
| `--to YYYY-MM-DD` | Range end (inclusive) | Requires `--from` |
| `--ticket N` | Filter by ticket ID | Requires `--app` |
| `--app N` | Filter by application ID | Required if `--ticket` given |
| `--type NAME` | Filter by time type name (exact, case-insensitive) | Looked up via `ListTimeTypes`, passed as ID |
| `--user UID` | Override "me" | UID string |
| `--limit N` | Cap results | Default 100 |
| `--profile NAME` | Override active profile | Global flag from Phase 1 |
| `--json` | JSON output | Global flag |

**Default filter (no flags):** `DateRange = WeekRefContaining(time.Now())`, `UserUID = whoami.UID`, `Limit = 100`.

**Whoami dependency:** default filter calls `authsvc.WhoAmI` on every invocation with no `--user`. Whoami failure = hard error with clear message.

**Human output (flat table):**

```
DATE        HOURS  TYPE                TARGET                       DESCRIPTION
2026-04-06   2.00  Development         #12345 Ingest pipeline       Investigating the ingest bug
2026-04-06   1.50  General Admin       Platform Services (project)  Team standup + email triage
2026-04-07   3.00  Development         #12345 Ingest pipeline       Fixed the batch lag
────────────────────────────────────────────────────────────────────────────────────────
TOTAL       24.50
```

Description is truncated with `…` if it exceeds the remaining terminal width (fall back to 60 cols if `term.GetSize` fails).

**JSON output:**

```json
{
  "schema": "tdx.v1.entryList",
  "filter": {
    "from": "2026-04-05",
    "to": "2026-04-11",
    "userUID": "abcd-...",
    "limit": 100
  },
  "totalHours": 24.50,
  "totalMinutes": 1470,
  "entries": [
    {
      "id": 987654,
      "date": "2026-04-06",
      "minutes": 120,
      "hours": 2.0,
      "userUID": "abcd-...",
      "target": {
        "kind": "ticket",
        "appID": 42,
        "itemID": 12345,
        "displayRef": "#12345",
        "displayName": "Ingest pipeline"
      },
      "timeType": { "id": 17, "name": "Development" },
      "description": "Investigating the ingest bug",
      "billable": false,
      "reportStatus": "open"
    }
  ]
}
```

### 4.2 `tdx time entry show <id>`

- Positional `<id>` required (int).
- `timesvc.GetEntry` call.
- 404 → `domain.ErrEntryNotFound` → CLI prints `entry <id> not found` and exits 1.
- Human: detail block, one field per line. JSON: `{ "schema": "tdx.v1.entry", "entry": {...} }`.

### 4.3 `tdx time week show [date]`

- Positional `[date]` optional, defaults to today (eastern TZ).
- Resolves to `WeekRef` containing that date.
- `timesvc.GetWeekReport` call.
- Human: `render.WeekGrid` — always seven columns (Sun..Sat), Row grouping by `(Target.DisplayRef, TimeType.Name)`, row label is target line 1 + type line 2 indented with `└`.
- JSON: `{ "schema": "tdx.v1.weekReport", "weekRef": {...}, "totalHours": N, "totalMinutes": N, "status": "open", "days": [...], "entries": [...] }`.

**Grid example (always seven columns):**

```
Week 2026-04-05 — 2026-04-11  (open)

  ROW                                  SUN  MON  TUE  WED  THU  FRI  SAT  TOTAL
  ────────────────────────────────────────────────────────────────────────────────
  #12345 Ingest pipeline                 .    2    3    3    3    2    .    13.0
    └ Development · IT Help Desk
  Platform Services (project)            .    2    1    1    1    2    .     7.0
    └ General Admin · Platform Services
  ────────────────────────────────────────────────────────────────────────────────
  DAY TOTAL                              .    4    4    4    4    4    .    20.0
```

Empty cells render as `.` (not zero), so gaps scan cleanly.

### 4.4 `tdx time week locked [--from --to]`

- Default range: current week (Sun..Sat eastern).
- `--from/--to` override (both required if either given).
- `timesvc.GetLockedDays` call.
- Human: one line per locked day (`2026-04-06  submitted`). If empty, prints `no locked days in range`. JSON: `{ "schema": "tdx.v1.lockedDays", "from": "...", "to": "...", "days": [...] }`.

### 4.5 `tdx time type list`

- No flags beyond globals.
- `timesvc.ListTimeTypes` call.
- Human: `render.Table` with columns `ID  NAME  BILLABLE  LIMITED  DESCRIPTION`. JSON: `{ "schema": "tdx.v1.timeTypes", "types": [...] }`.

### 4.6 `tdx time type for <kind> <id> --app <appID>`

- Positional `<kind>` and `<id>`, both required.
- `<kind>` is one of `ticket | ticketTask | projectTask | projectIssue`. These are the subset of `domain.TargetKind` values that the TD `/time/types/component/...` tree supports. Other kinds (`workspace`, `request`, `timeoff`) are valid domain types for entry filtering but do not have a component-lookup endpoint, so `type for` rejects them with `ErrUnsupportedTargetKind`.
- `--app <appID>` required for all supported kinds. `--task <taskID>` flag required for `ticketTask` and `projectTask`.
- Target construction happens in the CLI layer; the service just takes a `domain.Target`.
- `timesvc.TimeTypesForTarget` call.
- Human: `render.Table`. JSON: `{ "schema": "tdx.v1.timeTypesForTarget", "target": {...}, "types": [...] }`.

### 4.7 `tdx auth status` (update)

Phase 1's output:

```
profile:  default
tenant:   https://ufl.teamdynamix.com/
state:    authenticated
token:    valid
```

Phase 2 adds two lines when `Authenticated && TokenValid`:

```
profile:  default
tenant:   https://ufl.teamdynamix.com/
state:    authenticated
token:    valid
user:     Iain Moffat
email:    ipm@ufl.edu
```

Whoami failure is non-fatal:

```
profile:  default
tenant:   https://ufl.teamdynamix.com/
state:    authenticated
token:    valid
user:     (lookup failed: <short error>)
```

JSON output gains `user.fullName`, `user.email`, and `user.error` (omitempty).

## 5. Error handling

- **Generic 4xx/5xx on any time call** → `tdx.APIError` (already exists). CLI prints `tdx api: <status>: <message>` and exits 1.
- **401 on any call** → `tdx.ErrUnauthorized` → CLI prints `not authenticated — run 'tdx auth login'` and exits 1.
- **404 on `/time/{id}`** → `domain.ErrEntryNotFound` → CLI prints `entry <id> not found` and exits 1.
- **`--ticket` without `--app`** → CLI-layer validation error → usage message, exit 2.
- **Unknown `<kind>` on `type for`** → `domain.ErrUnsupportedTargetKind` → CLI prints supported kinds list, exit 2.
- **Whoami failure during `auth status`** → non-fatal, degraded `user:` line.
- **Whoami failure during default-filter `entry list`** → fatal: `could not resolve current user for default filter: <err>`, exit 1.

## 6. Testing strategy

Same TDD pattern as Phase 1.

- **Unit tests per timesvc method**, using `httptest.NewServer` with canned response bodies. Request-body-bearing methods (`SearchEntries`) assert the outgoing JSON shape, not just response decoding.
- **Unit tests per renderer** using golden-string comparison against `bytes.Buffer`. The `WeekGrid` test asserts exact column alignment with a known fixture.
- **Integration tests per CLI command**, using the `cobra.Execute` pattern + `TDX_CONFIG_HOME=t.TempDir()` + an httptest-backed TD stub. Same pattern Phase 1 used for `auth status` / `auth login`.
- **Domain tests** for `WeekRefContaining` covering: Sunday input, Saturday input, mid-week input, DST-boundary input (2026-03-08 spring forward in NY), year-boundary input.
- **Manual walkthrough** at `docs/manual-tests/phase-2-read-ops-walkthrough.md` — runs all six commands against real UFL, verifies human and `--json` output shapes. This is the Phase 2 equivalent of the Phase 1 walkthrough.

Coverage targets: `internal/domain/` ≥ 90%, `internal/svc/timesvc/` ≥ 85%, `internal/cli/time/*` ≥ 70%.

## 7. Decision log

| # | Decision | Rationale |
|---|---|---|
| 1 | v1 shortlist only (6 commands + whoami) | "Useful for read-only workflows" milestone from framework spec §11 Phase 2 |
| 2 | Whoami included in Phase 2 (not Phase 1.5) | People API access lands here anyway, and it fills the `auth status` identity gap |
| 3 | Default `entry list` filter is `this week, me` | Matches framework spec §2.2; requires whoami on default path |
| 4 | `--ticket` requires `--app` (no auto-inference) | Simplest, avoids extra API calls. Deferred config-based default |
| 5 | `entry list` always renders flat table, never grid | Clean separation: `entry list` = details, `week show` = summary |
| 6 | Typed TD methods live in `timesvc`, not `tdx.Client` | `tdx` stays a "thin HTTP client" per framework spec §5.8 |
| 7 | All dates computed in `America/New_York` | UFL billing zone; travels with laptop clock ignored |
| 8 | Week columns are Sun..Sat, always seven | Matches TD web app layout. **Amends framework spec §5.4** which said "Monday start" |
| 9 | `entry list` default `--limit 100` | Safe cap until pagination semantics verified |
| 10 | Whoami failure is non-fatal for `auth status`, fatal for default-filter `entry list` | `status` is a degradable display; `list` needs a concrete user ID |
| 11 | `tdx.Client` gains one helper `DoJSON`, no typed methods | Keeps `internal/tdx` thin |
| 12 | No TD response caching except per-process whoami | Simpler Phase 2; caching is a separate concern |
| 13 | `--type NAME` uses exact case-insensitive match via a per-call `ListTimeTypes` lookup | Fuzzy matching is fragile and not needed for v1 |

## 8. Unknowns resolved during implementation

Each item below is a **Step 0** on its owning task — verify first, implement second.

1. **`/auth/getuser` response shape.** Probe with real UFL token; write the decode struct against the actual JSON. Owning task: authsvc.WhoAmI.
2. **`POST /time/search` request body shape (`TimeEntryFilter`).** Read from `ReferenceMaterial/Time`. Owning task: timesvc.SearchEntries.
3. **TD pagination on `/time/search`.** Check reference. If paginated, add a `--all` flag in a follow-up task, not Phase 2.
4. **Component-target endpoint URLs per `TargetKind`.** Read from `ReferenceMaterial/Time`. Unsupported kinds return `ErrUnsupportedTargetKind`. Owning task: timesvc.TimeTypesForTarget.

## 9. Non-goals

- Writes of any kind (Phase 3).
- Templates (Phase 4).
- Pagination beyond `--limit` (Phase 2.5 if needed).
- Fuzzy or partial type-name matching.
- Offline mode, replay, persistent caching, cross-tenant joins.
- `entry recent`, `type show`, `week compare` (post-shortlist or later phases).
- MCP tools for read ops (Phase 5 exposes the same service methods).
- Binary distribution, CI, shell completions (Phase 6).

## 10. Plan structure (estimate)

Rough decomposition for the implementation plan, subject to `writing-plans` refinement:

1. Domain types batch 1: `User`, `Target`, `TargetKind`, `TimeType` + tests
2. Domain types batch 2: `TimeEntry`, `EntryFilter`, `ReportStatus`, `DateRange`, `WeekRef`, `DaySummary`, `WeekReport`, `LockedDay` + tests
3. `WeekRefContaining` + eastern TZ helper + DST / year-boundary tests
4. `tdx.Client.DoJSON` helper + tests
5. New sentinel errors (`ErrEntryNotFound`, `ErrUnsupportedTargetKind`)
6. `authsvc.WhoAmI` — Step 0 probes real tenant, then struct + method + test
7. `tdx auth status` integration — add `user:`/`email:` lines + tests
8. `timesvc` skeleton + `ListTimeTypes` method + test
9. `timesvc.TimeTypesForTarget` — Step 0 reads reference, then impl + tests
10. `timesvc.SearchEntries` — Step 0 reads reference for request body, then impl + tests
11. `timesvc.GetEntry` + 404 mapping test
12. `timesvc.GetWeekReport`
13. `timesvc.GetLockedDays`
14. `render.Table` helper + tests
15. `render.WeekGrid` helper + golden-string tests
16. `tdx time` parent wiring + `entry list` CLI + tests
17. `tdx time entry show` CLI + tests
18. `tdx time week show` + `week locked` CLI + tests
19. `tdx time type list` + `type for` CLI + tests
20. Wire `time` subtree into `cli/root.go`
21. `docs/manual-tests/phase-2-read-ops-walkthrough.md` + final exit-criteria check

Estimated total: ~21 TDD tasks. `writing-plans` may merge or split once it works through the actual test sequencing.
