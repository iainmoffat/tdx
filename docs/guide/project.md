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
- [tdx project feed](#tdx-project-feed)
- [tdx project comment](#tdx-project-comment)
- [tdx project task feed](#tdx-project-task-feed)
- [tdx project task comment](#tdx-project-task-comment)
- [tdx project log](#tdx-project-log)
- [tdx project time](#tdx-project-time)

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

## tdx project feed

Read the activity feed for a project. Shows system events (status changes, resource updates, percent-complete changes) and user comments interleaved by date.

```bash
tdx project feed 259
tdx project feed 259 --limit 100 --json   # schema: tdx.v1.projectFeed
```

**Flags:**

- `--limit N` — max rows (default 50, cap 500)
- `--json` — emit JSON

Sample output:

```
ID       DATE              BY            TYPE     BODY
1798321  2026-05-12 17:23  Sample User   comment  Backup config review complete — all checks passed.
1782210  2026-05-07 18:35  Pat Manager   system   Changed Portfolio(s) from "" to "Sample Portfolio".
```

The `TYPE` column shows `comment` for user-posted comments (TD's `UpdateType=1`) and `system` for everything else (status changes, edits, resource adds). Body is truncated at 80 chars in the human table; embedded newlines render as `↵`. Use `--json` for the full body.

---

## tdx project comment

Post a comment to a project feed. Requires `--yes`.

```bash
tdx project comment 259 --message "Backup config review complete — all checks passed." --yes
tdx project comment 259 --message "Private status update" --private --yes
tdx project comment 259 --message "Pinging the team" --notify me --notify some-uid --yes
```

**Flags:**

- `--message "..."` — comment text (required)
- `--notify me|UID|email` — repeatable; `me` resolves to your authed UID
- `--private` — mark the comment as private
- `--yes` — **required** to actually post

The new feed entry's ID is printed on success.

**TD wire-format note:** Project feed POST uses TD's `Body` field for the comment text (not `Comments` like ticket / task feed endpoints). This is transparent to users — both `tdx project comment` and `tdx project task comment` accept `--message` and produce the right wire shape internally.

---

## tdx project task feed

Read the activity feed for a specific project task. Same shape as `tdx project feed`.

```bash
tdx project task feed 259 4938 --plan 1292
tdx project task feed 259 4938 --plan 1292 --limit 50 --json   # schema: tdx.v1.projectTaskFeed
```

**Flags:**

- `--plan N` — plan ID (required)
- `--limit N` — max rows (default 50, cap 500)
- `--json` — emit JSON

---

## tdx project task comment

Post a comment to a project task feed. Requires `--yes`.

```bash
tdx project task comment 259 4938 --plan 1292 --message "Started on backup tests." --yes
```

**Flags:** same as `tdx project comment` plus `--plan N` (required).

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

## tdx project time

Show time entries logged against a project. Defaults to your entries for the current week; use `--all-users` for the team view, or `--user UID|email|me` (repeatable) for specific people. Filters down to specific plans/tasks within the project via `--plan` / `--task`.

```bash
tdx project time 259                               # my entries this week
tdx project time 259 --week 2026-05-04
tdx project time 259 --from 2026-04-01 --to 2026-04-30
tdx project time 259 --all-users --week 2026-05-04
tdx project time 259 --plan 1292 --task 4938 --week 2026-05-04
tdx project time 259 --user user1@example.com --user user2@example.com
tdx project time 259 --all-users --week 2026-05-04 --json   # schema: tdx.v1.projectTimeReview
```

**Flags:**

- `--week YYYY-MM-DD` — any date inside the target week (Sunday-Saturday, Eastern time)
- `--from / --to` — explicit date range (mutually exclusive with `--week`)
- `--user me|UID|email` — repeatable; default is `me`
- `--all-users` — resolve the project's team via TD's resources list (mutually exclusive with `--user`)
- `--plan N` — narrow to a specific plan
- `--task N` — narrow to a specific task
- `--limit N` — max rows (default 200)
- `--json` — emit JSON

**Implementation note:** TD's time-search endpoint silently ignores its `ProjectID` body field on the test tenant. tdx works around this with a client-side filter after fetching the user's entries in the date range. For per-user views this is one quick fetch; for `--all-users`, tdx fetches the project's resource list first, then one search per resource. Large project teams may bump against TD's 60-call-per-minute rate limit — back off and retry if you see a 429.

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
| `get_project_feed` | no |
| `get_project_task_feed` | no |
| `get_project_time_review` | no |
| `add_project_comment` | yes — requires `confirm: true` |
| `add_project_task_comment` | yes — requires `confirm: true` |
| `log_project_task_time` | yes — requires `confirm: true` |

Time-namespace tools also gained project inputs in this phase:

- `list_time_entries` now accepts `projectID? int`, `planID? int`, `taskID? int` (mutually exclusive with `ticketID`)
- `get_time_status_report` now accepts `projectID? int` as a peer of the existing `userUIDs?`/`managers?`/`accounts?`/`resourcePools?`/`all?` selectors

The `mine: true` input on `list_project_tasks` is mutually exclusive with `projectID` + `planID`. The `allUsers: true` input on `get_project_time_review` is mutually exclusive with `userUIDs`. All mutating tools mirror the CLI's `--yes` semantics — they return an error envelope if `confirm` is absent or `message` is empty.
