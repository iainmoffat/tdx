# `tdx project` Phase 4 — Project-Time Review

**Date:** 2026-05-12
**Goal:** Close the project↔time crossover loop by adding the time-review side. Two new commands, one extended command, and a small new domain area for project resources. Answers "what time have I (or my team) logged on project X?" in one shot.

## Motivation

Today `tdx project log` writes time against a project task; `tdx project task list --mine` shows what's assigned. But neither answers "what did I actually work on for project X this week?" — that requires walking `tdx time entry list` output by eye.

Phase 4 closes this with three additions:

1. A **per-user project-time view** — `tdx project time <project-id>` (defaults to my entries, current week)
2. A **`--project N` selector on `tdx time entry list`** — for users who already think in time-entry terms
3. A **`--project N` selector on `tdx time report status`** — adds project as a peer of `--user`/`--manager`/`--account`/`--resource-pool`/`--all` so a project lead can ask "where did my team's hours go?" in one report

## Live wire-format findings (probed 2026-05-12)

- `POST /api/time/search` body field `ProjectID` is **silently ignored** on the test tenant (requested ProjectID=259, results spanned ProjectIDs 52, 54, 435734). Same pattern as the documented people-search filter ignores.
- Honored filters: `PersonUIDs`, `EntryDateFrom`/`To`, and (per existing usage) `TicketIDs`, `TimeTypeIDs`, `ApplicationIDs`.
- `GET /api/projects/{id}/resources` returns an array of `{UID, FullName, RoleID, RoleName, FunctionalRoles[]}` records. 31 resources on project 259. Field is `UID` (lowercase), not `UserUID`/`UserUid`.

**Filter approach:** since `ProjectID` is silently ignored, all project-time filtering is **client-side** after a date-range + per-user fetch. For per-user views this is one quick fetch + match. For team views, the report runner fans out as it does today — only the user-resolution path changes (project resources → user list).

## Decisions

Settled during planning on 2026-05-12:

1. **Both surfaces ship**: `tdx project time <project-id>` (project-namespace lens) AND `tdx time entry list --project N` (time-namespace lens). Small redundancy; both are natural discovery paths. The first wraps the second internally.
2. **Default for `tdx project time`**: my entries, this week. Tightest daily flow.
3. **`--project N` on `tdx time entry list`** is mutually exclusive with `--ticket N` (they're alternate target filters); paired with `--plan N`/`--task N` it narrows further inside the project (client-side, all-AND).
4. **`--project N` on `tdx time report status`** resolves the project's team via `/api/projects/{id}/resources`, then runs the existing fan-out. Mutually exclusive with the other selector types (same rule as today's `--user`/`--manager`/etc.).
5. **Single `--project` value** in Phase 4 (no `--project A --project B` repetition). Defer multi-value to a Phase 4.x when there's an ask.
6. **`projectsvc.ListResources`** is the new service method. Strictly additive to the existing service surface.

## Decisions deferred / out of scope

- **Cost / billable totals broken out by user.** Phase 4 keeps the existing report shape — billable/non-billable totals, no allocation/rate aggregation.
- **Multi-project filter** (`--project A --project B` AND across both, or union). Defer.
- **Plan-level report selector** (`--plan N` on `tdx time report status`). Defer; project granularity is enough for daily flow.
- **Functional-role aggregation** in the project resource fetch. We use only UID + name.

## Behavior

### `tdx project time <project-id>`

```
tdx project time <project-id>
  [--week YYYY-MM-DD | --from YYYY-MM-DD --to YYYY-MM-DD]
  [--user me|UID|email]... [--all-users]
  [--plan N] [--task N]
  [--limit N] [--json] [--profile P]
```

Defaults: `--user me`, `--week` = current week. `--all-users` is a Phase 4 ergonomic that resolves the project's team (via `/api/projects/{id}/resources`) into a per-user list, then fetches and merges client-side. `--all-users` is mutually exclusive with `--user`.

Output: table with DATE, USER, TYPE, KIND (project / projectTask / projectIssue), REF (planID/taskID), HOURS, DESCRIPTION. Footer shows total hours (and per-user totals if multi-user).

JSON envelope: `tdx.v1.projectTimeReview` with `{projectID, dateRange, users[], totalHours, billableHours, nonBillableHours, entries[]}`.

### `tdx time entry list --project N [--plan M] [--task K]`

Adds two flags on `tdx time entry list`:
- `--project N` — keep only entries where `Target.ItemID == N` (TargetProject), `Target.ProjectID == N` (TargetProjectTask / TargetProjectIssue). Client-side filter after fetch.
- `--plan M` and `--task K` — further narrow within `--project`; require `--project` to be set.

Mutually exclusive with `--ticket`. Otherwise composes with all existing filters.

### `tdx time report status --project N`

Adds `--project N` as a peer selector to `--user`/`--manager`/`--account`/`--resource-pool`/`--all`. Resolves the project's team (via `/api/projects/{id}/resources`), runs the existing user-fan-out, and post-filters each row's totals to time entries whose target matches the project. Same JSON envelope schema (`tdx.v1.timeStatusReport`); `filter.selector` becomes `"project"` and `filter.projectID` is echoed.

**Note on cost:** the existing runner fetches each user's full week-report via `GetWeekReportForUser`. Phase 4's `--project` selector ALSO post-filters those week-reports to project entries only. So the cost is the same as `--manager me` today — one week-report per resource on the project.

## MCP

Three changes — no new tools:

| Tool | Change |
|---|---|
| `list_time_entries` | gain `projectID? int`, `planID? int`, `taskID? int` inputs (mutually exclusive with `ticketID` for projectID) |
| `get_time_status_report` | gain `projectID? int` input; same mutex as the other selectors |
| (new) `get_project_time_review` | wraps `tdx project time` semantics — `{profile?, projectID, week?, from?, to?, userUIDs?[], allUsers?, planID?, taskID?, limit?}` |

Tool count: 74 → 75.

## Domain

`internal/domain/project.go` gains:

```go
// ProjectResource is a row from /api/projects/{id}/resources.
// Minimal Phase 4 shape — only what's needed for the report runner's
// user list. Field names follow the API: UID (lowercase), FullName.
type ProjectResource struct {
    UID       string `json:"uid"`
    FullName  string `json:"fullName,omitempty"`
    RoleID    int    `json:"roleID,omitempty"`
    RoleName  string `json:"roleName,omitempty"`
    IsActive  bool   `json:"isActive,omitempty"`
}
```

`domain.EntryFilter` gains optional `ProjectID int`, `PlanID int`, `TaskID int` fields. Zero means "no project filter". Implementation: `SearchEntries` sets the corresponding `wireTimeSearch` fields (even though TD ignores them — defensive) AND does a client-side post-filter on the decoded entries.

`domain.TimeStatusReportFilter` (if present) or the equivalent in the runner gains a `ProjectID int` field for the new `--project` selector. Existing `TimeStatusReport` schema fields unchanged.

## Service layer

- `internal/svc/projectsvc/resources.go` (new): `ListResources(ctx, profile, projectID int) ([]domain.ProjectResource, error)`. Wires GET `/api/projects/{id}/resources`. Decoder handles `UID`/`FullName` and silently drops `null`-valued optional fields.
- `internal/svc/timesvc/entries.go`: extend `SearchEntries` to populate `req.ProjectID`/`PlanID` (defensive — TD ignores them, but we send for forward-compat) and to apply a client-side post-filter on the decoded `entries[].Target` when the filter has a project ID. Match rules:
  - `TargetProject`: entry's `Target.Kind == TargetProject && Target.ItemID == projectID`
  - `TargetProjectTask`: entry's `Target.Kind == TargetProjectTask && Target.ProjectID == projectID` (and if `filter.PlanID > 0`, also `Target.ItemID == planID`; if `filter.TaskID > 0`, also `Target.TaskID == taskID`)
  - `TargetProjectIssue`: entry's `Target.Kind == TargetProjectIssue && Target.ProjectID == projectID`
- No new service methods on `timesvc`.

## CLI layer

- `internal/cli/project/time.go` + `_test.go` (new): `tdx project time <project-id>` runner. Composes:
  1. Resolve users: `--user me/UID/email` (resolved via `peoplesvc`) or `--all-users` (fetched via new `projectsvc.ListResources`)
  2. Compute date range from `--week`/`--from`/`--to`
  3. For each user, `timesvc.SearchEntries` with `EntryDateRange + UserUID + ProjectID` filter
  4. Merge, sort by date asc, render
- `internal/cli/time/entry/list.go`: add `--project N`, `--plan N`, `--task N` flags; validation as described; plumb into `EntryFilter`.
- `internal/cli/time/report/status.go` + `runner.go`: add `--project N` flag; new `resolveUsers` branch that calls `projectsvc.ListResources` and uses the UIDs as the user list; runner post-filters week totals via a new helper that walks each row's underlying entries against the project filter.

`projectsvcAPI` (CLI-side interface) gains `ListResources`.

## Testing

- Domain: `ProjectResource` zero-value + round-trip (small).
- `projectsvc.ListResources`: httptest fixture returns 3 resources; decode + null-field tolerance.
- `timesvc.SearchEntries` project filter: fixture returns 5 mixed entries (2 on project 259, 1 on project 52, 1 ticket, 1 workspace); filter for projectID=259 keeps only the 2. Add `PlanID`/`TaskID` narrowing tests too.
- CLI: stub-based tests for `tdx project time` covering single-user/multi-user/all-users/date-range/mutex (user vs all-users; project-id positive int).
- Report runner: a new test verifying `--project N` resolves users via the stub `projectsvcAPI` and feeds them into the existing fan-out.
- MCP: three new tests for the input shapes; tool-count assertion bumped 74 → 75.

## Live verification

1. `tdx project time 259` returns my project-259 entries this week.
2. `tdx project time 259 --week 2026-05-04 --all-users` returns the team's entries for that week, summed correctly.
3. `tdx time entry list --project 259 --week 2026-05-04` matches the project-time-review output (same entries).
4. `tdx time report status --project 259 --week 2026-05-04` returns one row per resource on the project with project-only totals.
5. `tdx time report status --project 259 --week 2026-05-04 --json | jq '.filter'` shows `selector: "project"` and `projectID: 259`.

## Risks / mitigations

- **TD might honor `ProjectID` after all on some tenants.** Setting the wire field is defensive — it would just speed up the post-filter (which would then be a no-op match).
- **Project with many resources (>50).** UFL's project 259 has 31; reasonable. Existing report runner caps user fan-out at 5 concurrent; we inherit that.
- **`/api/projects/{id}/resources` permission gating.** Per the resource probe, this endpoint returns the resource list for any project the caller can see. If a tenant restricts this, the `--all-users` and `--project` selector both fail with a clean permission error; the per-user views still work.

## Definition of done

1. `tdx project time` works in human + `--json`, single-user and `--all-users` modes.
2. `tdx time entry list --project N` works with `--plan`/`--task` narrowing.
3. `tdx time report status --project N` works as a peer selector; JSON `filter` echoes correctly.
4. 1 new MCP tool + 2 extended tool inputs; tool count 74 → 75.
5. Live verification 1-5 above pass.
6. Tests, vet, gofmt, lint clean.
7. `docs/guide/project.md` and `docs/guide/time.md` updated.
