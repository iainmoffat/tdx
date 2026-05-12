# `tdx project` Phase 4 — Project-Time Review Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development.

**Goal:** Ship Phase 4 of the `tdx project` namespace: `tdx project time`, `--project N` on `tdx time entry list`, `--project N` on `tdx time report status`, + small additions to the MCP layer.

**Spec:** `docs/specs/2026-05-12-tdx-project-time-review.md`

**Architecture:** Net-new `projectsvc.ListResources` + `internal/cli/project/time.go`. Additive `domain.ProjectResource`, `domain.EntryFilter.{ProjectID, PlanID, TaskID}`, and a project selector on the report runner. `timesvc.SearchEntries` gains client-side ProjectID/PlanID/TaskID post-filtering (TD silent-ignores these wire fields).

**Wire-format facts (confirmed live 2026-05-12):**

- `POST /api/time/search` honors `PersonUIDs`, `EntryDateFrom`/`To`. `ProjectID` body field is **silently ignored** — client-side filter required.
- `GET /api/projects/{id}/resources` returns `[{UID, FullName, RoleID, RoleName, FunctionalRoles[]}, ...]`. Field name is `UID` (lowercase), not `UserUID`. 31 resources on project 259. `PercentAllocated` etc. are null in the list shape — Phase 4 doesn't need them.

**Patterns to mirror:**
- `internal/svc/projectsvc/tasks.go` for `ListResources` (similar GET pattern)
- `internal/cli/time/report/runner.go` for the new `--project` selector branch (same shape as `--manager`/`--account` cases)
- `internal/cli/time/entry/list.go` for new flag wiring on the existing time-entry list

---

## File structure

```
internal/
├── domain/
│   ├── project.go                    # MODIFY: add ProjectResource
│   ├── project_test.go               # MODIFY: round-trip test for ProjectResource
│   ├── entry.go                      # MODIFY: EntryFilter gains ProjectID, PlanID, TaskID
│   └── entry_test.go                 # MODIFY: filter zero-value/round-trip
├── svc/
│   ├── projectsvc/
│   │   ├── resources.go              # NEW: ListResources
│   │   ├── resources_test.go         # NEW
│   │   └── types.go                  # MODIFY: wireProjectResource
│   └── timesvc/
│       ├── entries.go                # MODIFY: project filter wire + post-filter
│       └── entries_test.go           # MODIFY: project filter tests
├── cli/
│   ├── project/
│   │   ├── project.go                # MODIFY: register `time` subcommand; projectsvcAPI adds ListResources
│   │   ├── time.go                   # NEW: tdx project time
│   │   ├── time_test.go              # NEW
│   │   └── stub_test.go              # MODIFY: stub ListResources
│   └── time/
│       ├── entry/
│       │   ├── list.go               # MODIFY: --project / --plan / --task flags
│       │   └── list_test.go          # MODIFY
│       └── report/
│           ├── status.go             # MODIFY: --project flag + validation
│           ├── runner.go             # MODIFY: project selector branch
│           ├── runner_test.go        # MODIFY
│           └── print.go              # MODIFY: filter echo for projectID
└── mcp/
    ├── tools_project.go              # MODIFY: get_project_time_review (read)
    ├── tools_project_test.go         # MODIFY
    ├── tools_entry.go                # MODIFY: list_time_entries gains projectID/planID/taskID
    ├── tools_report.go               # MODIFY: get_time_status_report gains projectID
    ├── tools_report_test.go          # MODIFY
    └── server_test.go                # MODIFY: tool count 74 → 75
docs/
├── specs/2026-05-12-tdx-project-time-review.md
├── plans/2026-05-12-tdx-project-time-review.md  # this file
└── guide/
    ├── project.md                    # MODIFY: add `tdx project time` section + update MCP table
    └── time.md                       # MODIFY: --project flag docs on entry list + report status
```

---

## Tasks

### Task 1: Domain — ProjectResource + EntryFilter project fields

**Files:** `internal/domain/project.go`, `project_test.go`, `entry.go`, `entry_test.go`

- [ ] **Step 1.1: Add `ProjectResource` to `project.go`**

```go
// ProjectResource is one row from /api/projects/{id}/resources.
// Minimal Phase 4 shape — only the fields needed for the time-review
// "team" path. Wire UID field is lowercase ("UID", not "UserUID").
type ProjectResource struct {
    UID      string `json:"uid"                yaml:"uid"`
    FullName string `json:"fullName,omitempty" yaml:"fullName,omitempty"`
    RoleID   int    `json:"roleID,omitempty"   yaml:"roleID,omitempty"`
    RoleName string `json:"roleName,omitempty" yaml:"roleName,omitempty"`
    IsActive bool   `json:"isActive,omitempty" yaml:"isActive,omitempty"`
}
```

- [ ] **Step 1.2: Test** — small round-trip + zero-value test.

- [ ] **Step 1.3: Extend `EntryFilter`** in `entry.go`:

```go
type EntryFilter struct {
    DateRange  DateRange
    UserUID    string
    Target     *Target
    TimeTypeID int
    ProjectID  int   // NEW: client-side project filter (TD silent-ignores wire field)
    PlanID     int   // NEW: narrows project filter to a specific plan
    TaskID     int   // NEW: narrows project filter to a specific task
    Limit      int
}
```

- [ ] **Step 1.4: Test** — round-trip + zero-value.

- [ ] **Step 1.5: Verify + commit**

```
feat(domain): add ProjectResource; EntryFilter gains project/plan/task IDs
```

---

### Task 2: Service — `projectsvc.ListResources`

**Files:** `internal/svc/projectsvc/resources.go` + `resources_test.go`; modify `types.go`

- [ ] **Step 2.1: Wire type** — add to `types.go`:

```go
type wireProjectResource struct {
    UID      string `json:"UID"`
    FullName string `json:"FullName,omitempty"`
    RoleID   int    `json:"RoleID,omitempty"`
    RoleName string `json:"RoleName,omitempty"`
    IsActive bool   `json:"IsActive,omitempty"`
}
```

- [ ] **Step 2.2: Implement `resources.go`**

```go
// ListResources fetches the resource list for a project.
// GET /api/projects/{id}/resources. Returns lowercase UIDs and full names;
// null-valued optional fields are tolerated.
func (s *Service) ListResources(ctx context.Context, profile string, projectID int) ([]domain.ProjectResource, error) {
    client, err := s.clientFor(profile)
    if err != nil { return nil, err }
    var wire []wireProjectResource
    path := fmt.Sprintf("/TDWebApi/api/projects/%d/resources", projectID)
    if err := client.DoJSON(ctx, "GET", path, nil, &wire); err != nil {
        return nil, fmt.Errorf("list project %d resources: %w", projectID, err)
    }
    out := make([]domain.ProjectResource, 0, len(wire))
    for _, w := range wire {
        out = append(out, domain.ProjectResource{
            UID: w.UID, FullName: w.FullName,
            RoleID: w.RoleID, RoleName: w.RoleName,
            IsActive: w.IsActive,
        })
    }
    return out, nil
}
```

- [ ] **Step 2.3: Tests** — httptest fixture returns 3 resources (one with all fields, one with most fields null, one inactive). Verify decode + null tolerance.

- [ ] **Step 2.4: Verify + commit**

```
feat(projectsvc): add ListResources for project team membership
```

---

### Task 3: timesvc — project-aware SearchEntries

**Files:** `internal/svc/timesvc/entries.go` + `entries_test.go`

- [ ] **Step 3.1: Update `SearchEntries`**

After existing `filter.Target` block, add a defensive wire population for the new EntryFilter fields:

```go
if filter.ProjectID > 0 {
    req.ProjectID = filter.ProjectID
    if filter.PlanID > 0 { req.PlanID = filter.PlanID }
    if filter.TaskID > 0 { req.TaskIDs = []int{filter.TaskID} }  // if TaskIDs exists on wireTimeSearch; if not, ItemID
}
```

After decode, before the time-type-name resolution, add the client-side post-filter:

```go
if filter.ProjectID > 0 {
    out = filterEntriesByProject(out, filter.ProjectID, filter.PlanID, filter.TaskID)
}
```

`filterEntriesByProject`:
- Keep entry if:
  - `e.Target.Kind == TargetProject && e.Target.ItemID == projectID`, OR
  - `e.Target.Kind == TargetProjectTask && e.Target.ProjectID == projectID` (and if planID>0, also `e.Target.ItemID == planID`; if taskID>0, also `e.Target.TaskID == taskID`), OR
  - `e.Target.Kind == TargetProjectIssue && e.Target.ProjectID == projectID`
- Otherwise drop

- [ ] **Step 3.2: Tests** — fixture serves 5 mixed entries:
  - project 259 (TargetProject, ItemID=259)
  - project 259 task (TargetProjectTask, ProjectID=259, ItemID=planID, TaskID=N)
  - project 52 task (different project)
  - ticket (unrelated)
  - workspace (unrelated)

  Tests:
  - `TestSearchEntries_ProjectIDFiltersOnlyProjectKinds` — filter ProjectID=259 keeps 2 (project + projectTask)
  - `TestSearchEntries_PlanIDNarrowsWithinProject` — adds plan filter, keeps 1
  - `TestSearchEntries_TaskIDNarrowsWithinProject` — adds task filter, keeps 1
  - `TestSearchEntries_ProjectIDIgnoredWhenZero` — filter=0 returns all
  - `TestSearchEntries_PostsExpectedBodyWithProjectID` — body capture confirms wire field populated

- [ ] **Step 3.3: Verify + commit**

```
feat(timesvc): SearchEntries gains project/plan/task filter (client-side)
```

---

### Task 4: CLI — `tdx time entry list --project N --plan M --task K`

**Files:** `internal/cli/time/entry/list.go` + `list_test.go`

- [ ] **Step 4.1: Add flags**

```go
cmd.Flags().IntVar(&projectFlag, "project", 0, "filter by project ID (mutually exclusive with --ticket)")
cmd.Flags().IntVar(&planFlag, "plan", 0, "narrow --project to a specific plan ID")
cmd.Flags().IntVar(&taskFlag, "task", 0, "narrow --project to a specific task ID")
```

- [ ] **Step 4.2: Validation (before config.ResolvePaths)**

```go
if projectFlag > 0 && ticketFlag > 0 {
    return fmt.Errorf("--project and --ticket are mutually exclusive")
}
if (planFlag > 0 || taskFlag > 0) && projectFlag <= 0 {
    return fmt.Errorf("--plan and --task require --project")
}
```

- [ ] **Step 4.3: Plumb into `EntryFilter`**

```go
filter.ProjectID = projectFlag
filter.PlanID = planFlag
filter.TaskID = taskFlag
```

- [ ] **Step 4.4: Tests** — flag mutex tests + happy path (stub returns 3 entries, 1 on project 259; filter --project 259 → 1).

- [ ] **Step 4.5: Verify + commit**

```
feat(cli/time): tdx time entry list gains --project/--plan/--task filters
```

---

### Task 5: CLI — `tdx project time`

**Files:** `internal/cli/project/time.go` + `time_test.go`; modify `project.go`, `stub_test.go`

- [ ] **Step 5.1: Extend `projectsvcAPI` interface** in `project.go`:

```go
ListResources(ctx context.Context, profile string, projectID int) ([]domain.ProjectResource, error)
```

Register the new `time` subcommand in `New()`.

- [ ] **Step 5.2: Stub** — extend `stub_test.go` to implement `ListResources`.

- [ ] **Step 5.3: Implement `time.go`**

```
tdx project time <project-id>
  [--week YYYY-MM-DD | --from YYYY-MM-DD --to YYYY-MM-DD]
  [--user me|UID|email]... [--all-users]
  [--plan N] [--task N]
  [--limit N] [--json] [--profile P]
```

Pre-config validation (mutex --user vs --all-users, project-id positive int).
Defaults: `--user me`, `--week` = current week (using `domain.WeekRefContaining(time.Now())`).
Date-range resolution: mirror `tdx time entry list` (`--week` OR `--from`+`--to`).
User resolution:
- If `--all-users`: call `svc.ListResources(ctx, profile, projectID)`; map to UIDs.
- Else: resolve each `--user` via `peoplesvc.resolvePrincipal` (or just `me` → authedUID); default to `[me]` when no `--user` flag.
For each user, call `timesvc.SearchEntries` with `{UserUID, DateRange, ProjectID, PlanID, TaskID}`.
Merge results, sort by date asc then user, render.

Output: table with DATE, USER, TYPE, KIND, REF, HOURS, DESCRIPTION (truncated 60). Footer: total hours; per-user totals if >1 user.
JSON envelope `tdx.v1.projectTimeReview`:

```json
{
  "schema": "tdx.v1.projectTimeReview",
  "projectID": 259,
  "dateRange": {"from": "2026-05-04", "to": "2026-05-10"},
  "users": [{"uid":"...","fullName":"..."}],
  "totalHours": 12.5,
  "billableHours": 10.0,
  "nonBillableHours": 2.5,
  "entries": [
    {"id":1234, "date":"2026-05-06", "userUID":"...", "userFullName":"...",
     "typeName":"Project", "kind":"projectTask", "projectID":259, "planID":1292,
     "taskID":4938, "hours":2.0, "billable":false, "description":"Backup review"}
  ]
}
```

- [ ] **Step 5.4: Tests**
  - `TestProjectTime_RequiresProjectID` — positional missing
  - `TestProjectTime_UserAndAllUsersMutex`
  - `TestProjectTime_DefaultsToMeThisWeek`
  - `TestProjectTime_AllUsersResolvesViaResources`
  - `TestProjectTime_DateRangeFromAndTo`
  - `TestProjectTime_JSONEnvelope`
  - `TestProjectTime_SumsHoursAcrossUsers`

- [ ] **Step 5.5: Commit**

```
feat(cli/project): tdx project time (single-user + --all-users)
```

---

### Task 6: CLI — `tdx time report status --project N`

**Files:** `internal/cli/time/report/status.go`, `runner.go`, `runner_test.go`, `print.go`

- [ ] **Step 6.1: Add `--project N` flag** in `status.go`. Mutually exclusive with `--user`/`--manager`/`--account`/`--resource-pool`/`--all` (same xor rule as today).

- [ ] **Step 6.2: Extend `statusFlags` + `MCPInputs`** with `projectID int`.

- [ ] **Step 6.3: Extend `runner.resolveUsers`** with a new branch:

```go
case f.projectID > 0:
    resources, err := deps.Project.ListResources(ctx, deps.Profile, f.projectID)
    if err != nil { return nil, err }
    out := make([]domain.User, 0, len(resources))
    for _, r := range resources {
        out = append(out, domain.User{UID: r.UID, FullName: r.FullName})
    }
    return out, nil
```

`runnerDeps` gains a `Project projectsvcAPI` interface field (small subset: `ListResources`).

- [ ] **Step 6.4: Add per-row project-filter post-processing**

After `g.Wait()` and before the existing `--include-zero` filter, when `f.projectID > 0`:

For each row, re-derive `BillableMin`/`NonBillableMin`/`TotalMin` by fetching the user's week entries via `timesvc.SearchEntries(filter{UserUID, DateRange, ProjectID})` and summing.

Implementation note: `GetWeekReportForUser` doesn't expose per-target breakdowns, so the cleanest path is a separate `SearchEntries` call per row. Add `timesvcEntriesAPI` to `runnerDeps` (small extension of `timesvcAPI`).

Concurrency: same `errgroup` pool (limit 5).

- [ ] **Step 6.5: Update `print.go` filter echo**

Add `selector: "project"`, `projectID: N` to the JSON envelope's `filter` block when `--project` is in use. Update the human header line.

- [ ] **Step 6.6: Tests** — new `TestRunner_ProjectSelectorResolvesViaResources` + `TestRunner_ProjectFilterScopesTotalsToProjectEntries`.

- [ ] **Step 6.7: Verify + commit**

```
feat(time/report): tdx time report status --project N selector
```

---

### Task 7: MCP — 1 new tool + 2 extended

**Files:** `internal/mcp/tools_project.go`, `tools_project_test.go`, `tools_entry.go`, `tools_report.go`, `tools_report_test.go`, `server_test.go`

- [ ] **Step 7.1: `get_project_time_review`** — new read tool wrapping `tdx project time`. Inputs: `{profile?, projectID, week?, from?, to?, userUIDs?[], allUsers?, planID?, taskID?, limit?}`. Validates `userUIDs xor allUsers`.

- [ ] **Step 7.2: Extend `list_time_entries`** — add `projectID?`, `planID?`, `taskID?` inputs. Same mutex (`ticketID xor projectID`).

- [ ] **Step 7.3: Extend `get_time_status_report`** — add `projectID? int` input; same xor rule with the other selectors.

- [ ] **Step 7.4: Tool-count assertion** — `server_test.go` from 74 → 75.

- [ ] **Step 7.5: Tests** — input validation + happy path for each.

- [ ] **Step 7.6: Commit**

```
feat(mcp): project-time review tool + project filter on list/report (74 → 75)
```

---

### Task 8: Live verification

- [ ] Build, run each command against the test tenant:
  1. `tdx project time 259` — my entries this week (likely 0-1)
  2. `tdx project time 259 --week 2026-05-04 --all-users --json | jq '{users: .users | length, total: .totalHours, billable: .billableHours}'`
  3. `tdx time entry list --project 259 --week 2026-05-04` — verify only project-259 entries
  4. `tdx time entry list --project 259 --plan 1292 --week 2026-05-04` — narrows to one plan
  5. `tdx time report status --project 259 --week 2026-05-04 --json | jq '.filter, (.weeks[0].rows | length)'`
  6. Sanity-check totals against the web UI's project time tab.

---

### Task 9: Docs

- [ ] **Step 9.1: `docs/guide/project.md`** — add a "tdx project time" section + extend the MCP tools table (74 → 75).
- [ ] **Step 9.2: `docs/guide/time.md`** — document the new `--project`/`--plan`/`--task` flags on `time entry list` and the new `--project` selector on `time report status`.
- [ ] **Step 9.3: Index updates** in `docs/guide.md` command tree.
- [ ] **Step 9.4: README ASCII tree** — same update.
- [ ] **Step 9.5: Commit**

```
docs: tdx project time + --project selector docs
```

---

### Task 10: Release

- [ ] **Step 10.1: Final lint/test sweep** — green.
- [ ] **Step 10.2: PR titled `v0.19.0: tdx project time review (Phase 4)`**.
- [ ] **Step 10.3: After CI green, squash-merge with `--admin` + delete branch.**
- [ ] **Step 10.4: Tag `v0.19.0` and push.**
- [ ] **Step 10.5: Update memory** — current_state bump, backlog ticks off Phase 4, append wire-format finding to `reference_td_project_api_quirks.md` (`/api/projects/{id}/resources` shape + `/api/time/search` ProjectID silent-ignore).
