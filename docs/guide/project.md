# tdx project

`tdx project` exposes TeamDynamix project visibility: list the projects you're a resource on, search by name, see plan and task structure, find tasks assigned to you across all projects, and log time directly against a project task.

Phase 1 is read-mostly. The only mutating command is `tdx project log`, which is a thin wrapper around `tdx time entry add --project ... --plan ... --task ...` (and uses the same `--yes` confirmation gate).

All commands accept `--profile <name>` to override the active profile.

## Contents

- [tdx project list](#tdx-project-list)
- [tdx project search](#tdx-project-search)
- [tdx project show](#tdx-project-show)
- [tdx project plan list](#tdx-project-plan-list)
- [tdx project task list](#tdx-project-task-list)
- [tdx project task show](#tdx-project-task-show)
- [tdx project log](#tdx-project-log)

---

## tdx project list

List the projects you participate in. TD's `/api/projects/list` returns one row per **plan**, not per project — so a project with both a waterfall plan and a cardwall plan shows up as two rows. The `MY-TASKS` column tells you at a glance which plans have work assigned to you.

```bash
tdx project list
tdx project list --json --limit 100   # schema: tdx.v1.projectPlanList
```

**Flags:**

- `--json` — emit JSON instead of a table
- `--limit N` — max rows (default 50, cap 500)

Sample output:

```
PROJECT-ID  PROJECT                           PLAN-ID  PLAN                                  TYPE       MY-TASKS  TASKS  % COMPLETE  START       END
259         Fiscal Year 2026 Disaster Rec…  1292     Sample Recovery Project    waterfall  1         51     0.0%        2025-07-01  2026-06-30
54          Sample Operations and Support       2091     Operations                            waterfall  12        423    0.0%        2026-03-02  2029-12-31
```

---

## tdx project search

Server-side search via `POST /api/projects/search`. The optional positional argument is the `NameLike` filter.

```bash
tdx project search "Disaster"
tdx project search --manager me                     # projects you administer
tdx project search --type "IT Project" --include-inactive
```

**Flags:**

- `--manager me|UID|email` — filter by project manager (admin). Sample-verified: server-side `ManagerUID` filter is honored.
- `--type NAME|ID` — repeatable. Numeric values are used directly; names resolve via `/api/projects/types`.
- `--status ID` — repeatable. **Numeric IDs only** in Phase 1; project status names aren't surfaced as a lookup yet.
- `--active` / `--include-inactive` — default is `--active`. Adding `--include-inactive` flips to `IsActive=false`.
- `--limit N` — max rows (default 50, cap 1000)
- `--json` — emit JSON, schema `tdx.v1.projectList`

---

## tdx project show

Full project detail. Renders status, manager, sponsor, account, % complete, hours summary, and dates. If you have current-week time entries against this project, they're listed under "This week".

```bash
tdx project show 259
tdx project show 259 --json     # schema: tdx.v1.project
```

Sample output:

```
PROJECT 259 — Sample Recovery Project

Status:      Executing
Type:        Regulatory Project
Manager:     Pat Manager
Sponsor:     Sam Sponsor
Account:     999999 (Sample Department)
Active:      yes
% Complete:  96.0%
Hours:       actual=58.0 / estimated=320.0
Dates:       2025-07-01 → 2026-06-30
Modified:    2026-05-07
```

The "Manager" line is decoded from TD's `AdminUID`/`AdminName` fields. TD's API vocabulary calls this slot "Admin"; the web UI calls it "Project Manager".

---

## tdx project plan list

List the plans for a single project.

```bash
tdx project plan list 259
tdx project plan list 259 --name-like FY2026 --include-empty
tdx project plan list 259 --json     # schema: tdx.v1.projectPlanList
```

**Flags:**

- `--name-like SUBSTR` — server-side substring filter (NameLike)
- `--include-empty` — include plans with zero tasks (default omits)
- `--json` — emit JSON

---

## tdx project task list

Two modes:

### Single-plan mode

List the tasks for one plan within one project.

```bash
tdx project task list 259 --plan 1292
tdx project task list 259 --plan 1292 --limit 200 --json   # schema: tdx.v1.projectTaskList
```

If you pass a project ID without `--plan`, the command errors and points you at `tdx project plan list <id>` to find the plan ID.

### --mine mode (cross-project fanout)

Show all tasks assigned to you across every project you're on. TD has no "tasks assigned to me" endpoint, so this fans out:

1. Fetch your plans via `/api/projects/list`
2. Skip plans where `MyTaskCount == 0`
3. For each remaining plan, fetch tasks and filter to those that list you as a resource
4. Sort by end date (ascending; tasks without an end date last)

```bash
tdx project task list --mine
tdx project task list --mine --limit 50 --json
```

Mutually exclusive with `--plan` and with a positional project ID.

Sample output:

```
PROJECT  PLAN  TASK-ID  TITLE                                STATUS     %   EST  ACT      ASSIGNEES                       END
496      4907  4911     Determine first service to migrate   Overdue    0%  0.0  0.0      Sample User                     2026-03-13
259      1292  4938     Project Activities                   InProcess  0%  0.0  47.0     Resource A, Muralidhar…  2026-06-30
```

**Note on UID matching:** TD returns task resource UIDs in UPPERCASE GUID form while `tdx auth status` shows your UID in lowercase. The `--mine` filter handles this with a case-insensitive compare.

---

## tdx project task show

Full detail for a single task, including the assignment list.

```bash
tdx project task show 259 4938 --plan 1292
tdx project task show 259 4938 --plan 1292 --json   # schema: tdx.v1.projectTask
```

---

## tdx project log

Log time worked against a project task. Wraps `tdx time entry add` under the hood, so the entry shows up in `tdx time entry list`, week drafts, and reports.

```bash
tdx project log 259 4938 --plan 1292 --hours 0.5 --type "Project" --description "Backup config review" --yes
```

**Flags:**

- `--plan N` — plan ID (required)
- `--hours N` / `--minutes N` — duration (mutually exclusive, exactly one required)
- `--type NAME` / `--type-id N` — time type (mutually exclusive, exactly one required). Names resolve case-insensitively against TD's per-task type list.
- `--date YYYY-MM-DD` — entry date (default: today, in Eastern time)
- `--description "..."` — work description
- `--billable` — force billable; default is the type's billable flag
- `--yes` — **required** to actually log time. Without `--yes` the command refuses.

The new entry's ID is printed on success. To roll back: `tdx time entry delete <id> --yes`.

---

## MCP

The same operations are exposed as MCP tools for AI agents:

| Tool | Mutating? |
|---|---|
| `list_my_projects` | no |
| `search_projects` | no |
| `get_project` | no |
| `list_project_plans` | no |
| `list_project_tasks` (single-plan or `mine: true`) | no |
| `get_project_task` | no |
| `log_project_task_time` | yes — requires `confirm: true` |

The `mine: true` input on `list_project_tasks` is mutually exclusive with `projectID` + `planID`. `log_project_task_time` mirrors the CLI's mutex and `--yes` semantics; it returns an error envelope if `confirm` is absent.
