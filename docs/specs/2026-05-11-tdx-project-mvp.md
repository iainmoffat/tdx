# `tdx project` MVP (Phase 1) — Project Visibility + Time Crossover

**Date:** 2026-05-11
**Goal:** Ship the first slice of `tdx project ...` — daily project visibility (my projects, search, show, plans, tasks, my-tasks) plus the natural time crossover (`tdx project log` against a project task). Read-mostly; one mutating command (time log) that reuses existing `timesvc` plumbing. Mirrors the proven `tdx ticket` shape.

## Motivation

Today `tdx` covers Time and Tickets well. The data layer already understands projects — `domain.Target` has `TargetProject`/`TargetProjectTask`/`TargetProjectIssue` and `timesvc/encode.go` correctly routes them — but there is no *discovery* path. A user can log time to a project task only if they already know the project ID, plan ID, and task ID. The Web API has the discovery endpoints; we just haven't exposed them.

This spec adds the read commands that turn "I know my project numbers" into "show me my projects, my plans, my tasks, then let me log time against one." It also closes the agent-side loop: an MCP client can now propose time entries from observed tasks instead of needing the human to look up identifiers.

## Decisions

Settled during planning on 2026-05-11:

1. **Namespace**: all new commands live under `tdx project ...` to match the existing `tdx time` / `tdx ticket` / `tdx people` taxonomy.
2. **Read-first MVP**: 6 read commands + 1 time-crossover write. No project/task mutation in Phase 1; defer feed/comment to Phase 2 to keep the shippable surface tight.
3. **Mirror `tdx ticket` conventions**: principal resolution, `--profile`/`--json`/`--limit` flags, partial-result banners, JSON envelope schema names (`tdx.v1.project*`), MCP tool naming (`list_my_projects`, etc.). New patterns only where the project domain genuinely differs.
4. **No new profile field**: unlike Tickets (which use `profile.TicketAppID`), Projects in TD aren't app-scoped — paths are `/api/projects/...` not `/api/{appId}/projects/...`. Nothing to default.
5. **Plan-type normalization**: domain enum `PlanWaterfall`/`PlanCardwall` (mapped from wire `1`/`2`); unknown values render as `unknown(N)`.
6. **`--mine` task view is a fanout**: TD has no task-search/assigned-to-me endpoint. We walk `/api/projects/list` → filter to plans with `MyTaskCount > 0` → fetch tasks per plan → client-side filter to assignee=me. Capped at 50 projects to avoid unbounded fanout; honors `--limit`.
7. **Time crossover via `tdx project log`**: mirrors `tdx ticket log`. Calls existing `timesvc.AddEntry` with `Target{Kind:TargetProjectTask, ItemID:taskID, TaskID:taskID (legacy), ProjectID, PlanID(via ItemID encoding chain)}` — wire encoding is already implemented and tested. Requires `--yes` and `--type`/`--type-id`.
8. **JSON envelopes are additive**: new schemas `tdx.v1.projectList`, `tdx.v1.project`, `tdx.v1.projectPlanList`, `tdx.v1.projectTaskList`, `tdx.v1.projectTask`. Schema name unchanged on existing envelopes.

## Decisions deferred / out of scope (Phase 1)

- **`tdx project feed` / `tdx project comment`** (read + add project feed entries). Phase 2.
- **`tdx project task feed` / `tdx project task comment`**. Phase 2.
- **`tdx project resources <id>`** (list project resources with allocations). Phase 3.
- **`tdx project plan show <id> <plan-id>`** (full plan w/ tasks + resources + relationships). Phase 3.
- **Project-time crossover beyond `log`** (`tdx project time <id>` powered by `POST /api/time/search`). Phase 4 — adds the most leverage once basics are in.
- **Task update** (percent, status, assignee). Phase 5; involves plan checkout/draft dance.
- **Project create**. Phase 5; permission-gated like `tdx ticket create`.
- **Functional roles / resource pools surfacing**. Resource pools are already exposed via `tdx people pools list`; no new commands needed in Phase 1.
- **Cardwall lists endpoint** (`/api/projects/{id}/boards/{boardId}/lists`). Useful for board-only projects; defer to whichever phase first justifies a real board UX.

## Behavior

### Commands

#### `tdx project list`

List the caller's participating projects.

```
tdx project list [--json] [--limit N] [--profile P]
```

Calls `GET /api/projects/list`. Renders ID, NAME, STATUS, MANAGER, MY-TASKS (count from `MyTaskCount` if present), START, END. Sorted by name. Default limit 50; cap 500.

**Probe risk:** docs claim this returns Plan objects, not Projects. We will live-probe and either (a) decode the actual fields into a partial `Project` (preferred) or (b) introduce a `ProjectListEntry` domain type if the shape really is plan-flavored. The CLI output stays the same either way — only the internal decoder changes.

#### `tdx project search`

Server-side search via `POST /api/projects/search`.

```
tdx project search [QUERY] [--manager me|UID|email] [--status NAME|ID]... [--type NAME|ID]... [--active|--include-inactive] [--limit N] [--json] [--profile P]
```

`QUERY` maps to `NameLike`. Default: `IsActive=true`, `IsOpen=true`. `--manager me` resolves via `auth.WhoAmI`. Other filters resolved via list-then-match (no new server endpoints needed for Phase 1 — `TypeID` comes from `GET /api/projects/types`; status names from the project's status list if cheap, else accept numeric only and document).

**Filter fidelity:** TD search endpoints have a documented history of silently ignoring filter params. We will live-probe each filter on UFL and document any that are ignored in `reference_td_search_silent_filters.md`. Client-side post-filter as a fallback for fields TD ignores.

#### `tdx project show <id>`

Full project detail.

```
tdx project show <id> [--json] [--profile P]
```

Renders header (ID, name, status, manager, % complete, dates, type, account, hours summary) + a "This week" line listing time entries from the current week draft whose `Target.ItemID == projectID` (mirroring `tdx ticket show`'s thisWeek crossover). JSON envelope `tdx.v1.project`.

#### `tdx project plan list <project-id>`

```
tdx project plan list <project-id> [--include-empty] [--name-like SUBSTR] [--json] [--profile P]
```

Calls `POST /api/projects/{id}/plans/search`. Renders ID, NAME, TYPE (waterfall|cardwall|unknown(N)), STATUS, MY-TASKS (from `MyTaskCount`), TASKS, % COMPLETE, START, END.

#### `tdx project task list <project-id> [--plan N]` (single-plan mode)

```
tdx project task list <project-id> --plan <plan-id> [--limit N] [--json] [--profile P]
```

Calls `GET /api/projects/{project-id}/plans/{plan-id}/tasks`. Renders task list. When `--plan` omitted with a project-id, error and suggest `tdx project plan list <id>` to find one.

#### `tdx project task list --mine` (cross-project mode)

```
tdx project task list --mine [--limit N] [--json] [--profile P]
```

Fanout:
1. `GET /api/projects/list` → my projects (capped at 50)
2. For each, `POST /api/projects/{id}/plans/search` → filter plans where `MyTaskCount > 0`
3. For each kept plan, `GET /api/projects/{id}/plans/{plan-id}/tasks` → keep tasks where `ResponsibleUID == authedUID` (or `Resources[].UID == authedUID` if needed — probe live)
4. Sort by `EndDate ASC NULLS LAST, ModifiedDate DESC`, cap at `--limit` (default 50, max 200)

Columns: PROJECT, PLAN, TASK-ID, TITLE, STATUS, %, EST, ACT, END.

`--mine` is mutually exclusive with `--plan` and with positional `<project-id>`.

If TD's `/api/projects/list` returns 0 projects, suggest the user check their TD project participation.

#### `tdx project task show <project-id> <task-id> --plan <plan-id>`

```
tdx project task show <project-id> <task-id> --plan <plan-id> [--json] [--profile P]
```

`GET /api/projects/{project-id}/plans/{plan-id}/tasks/{task-id}`. Renders header + Resources + Hours summary + optional first lines of description.

#### `tdx project log <project-id> <task-id>` (mutating; time crossover)

```
tdx project log <project-id> <task-id> --plan <plan-id> --hours N|--minutes N --type NAME|--type-id N [--date YYYY-MM-DD] [--description "..."] [--billable] [--yes] [--profile P]
```

Mirror of `tdx ticket log`. Builds `domain.Target{Kind:TargetProjectTask, ItemID:taskID, ProjectID:projectID}` (PlanID lives implicitly in the wire path; encode helper already handles it via the existing `ProjectID+PlanID+ItemID` triplet). Validates the time-type via `timesvc.TimeTypesForTarget` and accepts a name or ID. Requires `--yes`. Successful add prints the created entry ID.

**Wire-encoding note:** `domain.Target` does not currently carry a separate `PlanID` field. Either (a) extend `Target` to add `PlanID int` (preferred — it's already implicit in the time wire format), or (b) plumb plan-id through `EntryInput` as a parallel field. We will choose (a) during plan — it's strictly additive on `Target` and matches how `ProjectID` is already plumbed.

### JSON envelopes

```
tdx.v1.projectList         { projects: [{id, name, statusName, managerName, myTaskCount?, start, end}] }
tdx.v1.project             { project: {full detail}, thisWeek?: [...] }
tdx.v1.projectPlanList     { plans: [{id, name, typeName, statusName, myTaskCount, taskCount, percentComplete, start, end}] }
tdx.v1.projectTaskList     { tasks: [{projectID, planID, id, title, statusName, percentComplete, est, act, end, responsibleName?}] }
tdx.v1.projectTask         { task: {full detail incl. resources[]} }
```

### MCP tools (read-only unless noted)

| Tool | Verb | Inputs |
|---|---|---|
| `list_my_projects` | read | `profile?`, `limit?` |
| `search_projects` | read | `query?`, `managerUID?`, `statusIDs?`, `typeIDs?`, `isActive?`, `limit?`, `profile?` |
| `get_project` | read | `id`, `profile?` |
| `list_project_plans` | read | `projectID`, `nameLike?`, `includeEmpty?`, `profile?` |
| `list_project_tasks` | read | `projectID?`, `planID?`, `mine?`, `limit?`, `profile?` (validates: `mine` xor (`projectID`+`planID`)) |
| `get_project_task` | read | `projectID`, `planID`, `taskID`, `profile?` |
| `log_project_task_time` | mutating, `confirm:true` | `projectID`, `planID`, `taskID`, `hours`|`minutes`, `typeName`|`typeID`, `date?`, `description?`, `billable?`, `profile?`, `confirm` |

Tool count: 63 → 70.

## Wire format

`/TDWebApi/` prefix on every call. All known endpoints:

- `GET /api/projects/list` — my projects. **LIVE-PROBE first** to confirm shape.
- `POST /api/projects/search` — body `ProjectSearch{ NameLike, IsActive, IsOpen, ManagerUID, StatusIDs, TypeIDs, … }`. Live-probe filter fidelity per field.
- `GET /api/projects/{id}` — full project.
- `POST /api/projects/{id}/plans/search` — body `PlanSearch{ NameLike?, IncludeEmpty? }`. Returns plans w/ `PlanType` enum (1=waterfall, 2=cardwall) and `MyTaskCount`.
- `GET /api/projects/{projectID}/plans/{planID}/tasks` — task list.
- `GET /api/projects/{projectID}/plans/{planID}/tasks/{taskID}` — task detail.
- `GET /api/projects/types` — for `--type` resolution (cached per profile).

Time entry creation reuses the existing `POST /api/time` path via `timesvc.AddEntry`. Wire format **already implemented and tested** (`timesvc/encode.go`): `Component=2 (TaskTime)`, `ItemID=taskID`, `ProjectID=projectID`, `PlanID=planID`. No new wire surface for the mutating path.

## Domain types

`internal/domain/project.go` (new):

```go
type Project struct {
    ID                int
    Name              string
    StatusID          int
    StatusName        string
    TypeID            int
    TypeName          string
    AccountID         int
    AccountName       string
    ManagerUID        string
    ManagerName       string
    PercentComplete   float64
    EstimatedHours    float64
    BudgetHours       float64
    StartDate         time.Time
    EndDate           time.Time
    ModifiedDate      time.Time
    IsActive          bool
    Description       string
    MyTaskCount       int   // populated from /list and /plans/search
}

type ProjectSearchFilter struct {
    NameLike    string
    ManagerUID  string
    StatusIDs   []int
    TypeIDs     []int
    IsActive    *bool
    IsOpen      *bool
    Limit       int
}

type ProjectPlan struct {
    ID              int
    ProjectID       int
    Name            string
    Type            ProjectPlanType  // PlanWaterfall|PlanCardwall|PlanUnknown
    StatusName      string
    TaskCount       int
    MyTaskCount     int
    PercentComplete float64
    StartDate       time.Time
    EndDate         time.Time
}

type ProjectPlanType int
const (
    PlanWaterfall ProjectPlanType = 1
    PlanCardwall  ProjectPlanType = 2
)
func (t ProjectPlanType) String() string { /* "waterfall" | "cardwall" | "unknown(N)" */ }

type ProjectTask struct {
    ProjectID       int
    PlanID          int
    ID              int
    Title           string
    StatusName      string
    PercentComplete float64
    EstimatedHours  float64
    ActualHours     float64
    RemainingHours  float64
    StartDate       time.Time
    EndDate         time.Time
    ResponsibleUID  string
    ResponsibleName string
    IsParent        bool
    Description     string
    Resources       []ProjectTaskResource
}

type ProjectTaskResource struct {
    UID       string
    FullName  string
    PercentAllocation float64
}
```

`domain.Target` gains additive `PlanID int` field with `yaml:"planID,omitempty"` and `json:"planID,omitempty"` tags. Existing serialization compatible (zero value omits).

## Service layer

`internal/svc/projectsvc/` (new package), mirroring ticketsvc:
- `service.go` — `Service{paths, profiles, credentials}` + `clientFor`
- `types.go` — `wireProject`, `wireProjectSearch`, `wirePlan`, `wirePlanSearch`, `wireTask`
- `projects.go` — `ListMine`, `Search`, `Get`
- `plans.go` — `SearchPlans`
- `tasks.go` — `ListTasks(projectID, planID)`, `GetTask(projectID, planID, taskID)`
- `types_lookup.go` — `ListProjectTypes`, `ResolveTypeByName` (for `--type` flag on search)

Decoders normalize PlanType → enum, parse timestamps via existing `wireDateTime` helper.

## CLI layer

`internal/cli/project/`:
- `project.go` — top-level + `projectsvcAPI` interface
- `list.go`/`list_test.go`
- `search.go`/`search_test.go`
- `show.go`/`show_test.go`
- `plan.go`/`plan_test.go` (registers `plan list`)
- `task.go`/`task_test.go` (registers `task list` w/ `--mine` + `task show`)
- `log.go`/`log_test.go` (registers `log`)
- `helpers.go`/`helpers_test.go` (`resolvePrincipal` proxy, `printProjectList` helper)

Wire `tdx project` into root in `internal/cli/cli.go` alongside `tdx ticket` and `tdx people`.

## Testing strategy

Mirrors the patterns from `internal/cli/ticket/*_test.go`:
- Stub `projectsvcAPI` in each CLI test file
- httptest-based service-layer tests for each endpoint (`projectsvc/projects_test.go` etc.)
- One end-to-end-ish "fanout" test for `task list --mine`: stub returns 3 projects, 2 plans per project (one w/ `MyTaskCount=0`, one with N>0), assert only my tasks bubble up

Plus the integration smoke I always do: `go test ./... && go vet ./... && gofmt -l . && golangci-lint run ./...` clean.

## Live verification (mandatory before tag)

1. `tdx project list` returns >= 1 project on UFL for the authed user.
2. Decode the actual `/api/projects/list` response and reconcile against the planned domain shape. If TD really returns Plan-shaped objects, adjust the decoder; either keep `Project` as the CLI-facing type (preferred) or introduce `ProjectListEntry`.
3. `tdx project search "<known project name fragment>"` returns matching projects.
4. Each `ProjectSearch` filter live-probed; any silently-ignored field documented in `reference_td_search_silent_filters.md`.
5. `tdx project show <id>` matches what the web UI shows (status, manager, dates, % complete).
6. `tdx project plan list <id>` returns plans with correct type labels and `MyTaskCount`.
7. `tdx project task list <id> --plan <p>` returns tasks; columns sensible.
8. `tdx project task list --mine` returns *only* my tasks; cross-project fanout works.
9. `tdx project log <project-id> <task-id> --plan <p> --hours 0.25 --type "<known type>" --description "tdx live test" --yes` creates a real time entry visible in `tdx time entry list`. Roll back via `tdx time entry delete`.

## Risks / mitigations

- **`/api/projects/list` shape ambiguity** → live-probe before locking decoder.
- **Search filter fidelity** → live-probe per filter; document silent-ignores.
- **`MyTaskCount` may be absent on UFL** → fall back to fanout without the pruning step; `--mine` still works, just slower.
- **`--mine` fanout latency** → cap projects at 50, run plan/task fetches with bounded concurrency (use `golang.org/x/sync/errgroup` w/ limit 5, same pattern as `time/report`).
- **`Target.PlanID` addition** → strictly additive, but every place that constructs/serializes `Target` needs a quick read to confirm no test fixture breaks. Already mostly covered by `time entry add` paths.

## Definition of done

1. All 7 commands above behave per the Behavior section, both human and `--json`.
2. All 7 MCP tools register and respond correctly to mock inputs.
3. Live verification (#1–9 above) passes on UFL.
4. `docs/guide.md` index gets a one-line entry for `tdx project`; `docs/guide/project.md` (new) documents each command with examples.
5. README mentions `tdx project` in the command tree.
6. Tests + vet + gofmt + golangci-lint clean.
7. Spec post-implementation note added if any wire-format surprises during live probe.
