# tdx Ticket Tasks Spec (Phase D.3, v0.16.2)

**Date:** 2026-05-08
**Goal:** Add `tdx ticket task ...` sub-group covering daily ticket-task workflows: list, show detail, read feed, update progress, and log time worked. Ship as v0.16.2.

## Motivation

TD ticket tasks are sub-tasks of a ticket — used for breaking work into stages, tracking completion percent, and (separately) accumulating time. v0.16.0 added ticket support but skipped tasks. Users with task-heavy workflows (project tickets, longer ITSM work) currently can't see or update tasks from the CLI; they have to switch to the web UI. This patch closes that gap with a tight 5-command MVP.

## Decisions

Settled during brainstorming on 2026-05-08, after live-probing UFL:

1. **Five commands** in `tdx ticket task`: `list`, `show`, `feed`, `update`, `log`. No `create`, `delete`, or `assign` — those are admin or permission-gated workflows; defer.
2. **`update` is the feed-POST path; `log` is the time-entry path.** TD's web UI separates these because they have different semantic effects: `update` (POST `/tasks/{id}/feed`) records progress + an informational `HoursWorked` field that does NOT create a time entry; `log` creates a real time entry against `TargetTicketTask` via existing `timesvc.AddEntry`.
3. **`update --complete` is a shortcut** for `update --percent 100`. No separate `complete` command.
4. **Both `update` and `log` require `--yes`** (consistent with the existing mutating-command pattern).
5. **Reuse `domain.TicketFeedEntry`** for task feed (same shape — comments, system events).
6. **Reuse `domain.Target.TargetTicketTask`** for `tdx ticket task log` — already exists, no domain change for the time-entry path.
7. **MCP exposure:** 3 read + 2 mutating tools (5 new). Tool count: 57 → 62.
8. **No `delete` command.** DELETE on a task returned HTTP 403 on UFL during live probing — most users don't have delete permission; not worth shipping a command users can't use.

## Live-verified API surface (UFL, 2026-05-08)

| Endpoint | Verified shape |
|---|---|
| `GET /api/{appId}/tickets/{ticketID}/tasks` | flat array of tasks |
| `GET /api/{appId}/tickets/{ticketID}/tasks/{taskID}` | single task object |
| `POST /api/{appId}/tickets/{ticketID}/tasks/{taskID}/feed` | body `{Comments, PercentComplete, HoursWorked, Notify, IsPrivate}` — returns the new feed entry |
| `GET /api/{appId}/tickets/{ticketID}/tasks/{taskID}/feed` | flat array of feed entries (same wire shape as `wireFeedEntry`) |
| `DELETE /api/{appId}/tickets/{ticketID}/tasks/{taskID}` | 403 for typical users — out of scope |

Created a probe task (deleted via `IsActive=false` since DELETE is 403):

```json
{
  "ID": 28187, "TicketID": 542034, "Title": "...", "PercentComplete": 0,
  "ActualMinutes": 0, "EstimatedMinutes": 0,
  "ResponsibleUid": null, "ResponsibleGroupID": 0, ...
}
```

Confirmed `HoursWorked` on the feed POST does NOT update task `ActualMinutes` and does NOT create a time entry. Body text reads `"Changed Percent Complete from \"0 %\" to \"50 %\".<br>15 min on this"` — purely cosmetic.

## Command surface

### `tdx ticket task list <ticket-id>`

`GET /api/{appId}/tickets/{ticketID}/tasks`. Output table: `ID | TITLE | %COMPLETE | EST | ACT | RESPONSIBLE`. Hours columns format minutes as `1h30m` etc. JSON envelope `tdx.v1.ticketTaskList`.

Flags: `--app <id>`, `--json`, `--profile`.

### `tdx ticket task show <ticket-id> <task-id>`

`GET /api/{appId}/tickets/{ticketID}/tasks/{taskID}`. Pretty-printed sections: header (`#<task-id> — <title>`), Status (PercentComplete %), Responsible, Created/Modified, Estimated/Actual time, Description. JSON envelope `tdx.v1.ticketTask`.

Flags: `--app`, `--json`, `--profile`.

### `tdx ticket task feed <ticket-id> <task-id>`

`GET /api/{appId}/tickets/{ticketID}/tasks/{taskID}/feed`. Same chronological format as `tdx ticket feed`. JSON envelope `tdx.v1.ticketTaskFeed`.

Flags: `--app`, `--limit N` (0 = all), `--json`, `--profile`.

### `tdx ticket task update <ticket-id> <task-id>` (mutating)

`POST /api/{appId}/tickets/{ticketID}/tasks/{taskID}/feed` with body `{Comments, PercentComplete, HoursWorked, IsPrivate, Notify}`.

Flags:
- `--percent N` (0-100; sent as `PercentComplete` if provided)
- `--complete` (shortcut for `--percent 100`; mutually exclusive with `--percent`)
- `--comment "..."` (sent as `Comments`)
- `--hours-worked N` (sent as `HoursWorked` — informational only; does NOT create a time entry; warn in `--help` text)
- `--private` (sent as `IsPrivate=true`)
- `--notify <UID>` (repeatable; sent as `Notify[]`)
- `--yes` (required to send)

Validation: at least one of `--percent`/`--complete`/`--comment`/`--hours-worked` must be set (else nothing to update). `--percent` and `--complete` are mutually exclusive.

Output: `task #<task-id> updated: percent=<N>%, comment="<truncated>" (feed entry <id>)`.

### `tdx ticket task log <ticket-id> <task-id>` (mutating, time crossover)

Creates a real time entry via existing `timesvc.AddEntry` with `Target{Kind: TargetTicketTask, AppID, ItemID: ticketID, TaskID: taskID}`. Same flag shape as `tdx ticket log`:

- `--hours N` or `--minutes N` (one required)
- `--type "<name>"` or `--type-id <int>` (one required)
- `--date YYYY-MM-DD` (default: today)
- `--description "..."`
- `--billable / --no-billable` (default from time-type)
- `--app <id>`, `--profile`, `--yes`

Validation via `timesvc.TimeTypesForTarget` (same as `tdx ticket log`). On success: `logged 30m to ticket #<id> task #<task-id> (entry <id>, type "<name>")`.

## Domain types

```go
// internal/domain/ticket.go (additive)

// TicketTask is one task on a ticket.
type TicketTask struct {
    ID                  int
    TicketID            int
    Title               string
    Description         string
    Active              bool
    PercentComplete     int
    EstimatedMinutes    int
    ActualMinutes       int
    StartDate           time.Time
    EndDate             time.Time
    CreatedDate         time.Time
    CreatedName         string
    ModifiedDate        time.Time
    CompletedDate       time.Time
    CompletedName       string
    ResponsibleUID      string
    ResponsibleName     string
    ResponsibleGroupID  int
    ResponsibleGroupName string
    Order               int
}
```

`domain.TicketFeedEntry` reused for task feed (no new type needed).
`domain.Target` and `TargetTicketTask` already exist (no change).

## Service layer

New file `internal/svc/ticketsvc/tasks.go`:

```go
func (s *Service) ListTasks(ctx, profile, appID, ticketID int) ([]domain.TicketTask, error)
func (s *Service) GetTask(ctx, profile, appID, ticketID, taskID int) (domain.TicketTask, error)
func (s *Service) GetTaskFeed(ctx, profile, appID, ticketID, taskID int) ([]domain.TicketFeedEntry, error)
func (s *Service) UpdateTaskFeed(ctx, profile, appID, ticketID, taskID int, body string, percentComplete *int, hoursWorked float64, isPrivate bool, notify []string) (int, error)
```

`UpdateTaskFeed` returns the new feed entry ID. `percentComplete *int` because 0 is a valid value (means "set to 0% complete"); nil means "don't send PercentComplete in body".

Wire types added to `internal/svc/ticketsvc/types.go`:

```go
type wireTicketTask struct { /* per live-probed shape above */ }

type wireTaskFeedUpdate struct {
    Comments        string   `json:"Comments,omitempty"`
    PercentComplete *int     `json:"PercentComplete,omitempty"`
    HoursWorked     float64  `json:"HoursWorked,omitempty"`
    IsPrivate       bool     `json:"IsPrivate,omitempty"`
    Notify          []string `json:"Notify,omitempty"`
}
```

Date fields decoded via existing `parseTDTime`.

## CLI layer

New package contents:

- `internal/cli/ticket/task.go` — top-level `tdx ticket task` cobra cmd; registers 5 sub-commands
- `internal/cli/ticket/task_list.go` (or shared file `task.go`)
- `internal/cli/ticket/task_show.go`
- `internal/cli/ticket/task_feed.go`
- `internal/cli/ticket/task_update.go`
- `internal/cli/ticket/task_log.go`
- one `*_test.go` per file

Decision: keep all 5 sub-command implementations in **one file** `internal/cli/ticket/task.go` (~400 lines) since they share heavy plumbing (cobra factory, ticket+task ID parsing, app-id resolution). One test file `task_test.go`. Mirrors the (smaller) `app.go` pattern that keeps `list`/`use`/`show` together.

`ticketsvcAPI` interface (in `ticket.go`) widens with the four new methods.
`stubTicketsvc` (in `stub_test.go`) gains corresponding fields + methods.

`tdx ticket task list` and `show` rendering helpers can reuse the existing `formatHours` (defined in `show.go`) and `formatDate`/`truncate` (in `search.go`).

`tdx ticket task log` reuses `timesvcAPI` interface defined for `tdx ticket log` (see `log.go`). The two commands share `runTicketLog`-ish flow but with `Target.TaskID` set.

Wired in `internal/cli/ticket/ticket.go` `New()`:

```go
cmd.AddCommand(newTaskCmd(nil))
```

## MCP layer

`internal/mcp/tools_ticket.go` adds:

**Read-only tools (3 new):**
- `list_ticket_tasks` — `{profile?, appID?, ticketID}`; envelope `tdx.v1.ticketTaskList`
- `get_ticket_task` — `{profile?, appID?, ticketID, taskID}`; envelope `tdx.v1.ticketTask`
- `get_ticket_task_feed` — `{profile?, appID?, ticketID, taskID, limit?}`; envelope `tdx.v1.ticketTaskFeed`

**Mutating tools (2 new, require `confirm:true`):**
- `update_ticket_task` — `{profile?, appID?, ticketID, taskID, comment?, percentComplete?, hoursWorked?, isPrivate?, notify?, confirm}`
- `log_ticket_task_time` — same inputs as `log_ticket_time` plus `taskID`; mirrors that handler with the additional `taskID` populated on the `Target`

Tool count: 57 → 62.

## JSON envelopes (additive)

- `tdx.v1.ticketTaskList`
- `tdx.v1.ticketTask`
- `tdx.v1.ticketTaskFeed`

`update_ticket_task` and `log_ticket_task_time` return ad-hoc result objects (mirroring the existing `add_ticket_comment` / `log_ticket_time` patterns — no formal envelope).

## Documentation

- `docs/guide/ticket.md` — add a new top-level `## tdx ticket task` section after `## tdx ticket statuses`. Sub-sections per command. Update Contents.
- `docs/guide.md` ASCII tree — add `task → list / show / feed / update / log` to the `ticket` branch.
- `README.md` ASCII tree — same change (must stay byte-identical).
- `docs/guide/mcp.md` — add 5 new rows to "Tickets (Phase D — read-only)" and "Tickets (Phase D — mutating)" tables; bump tool count to 62 (read 12, mutating 6 — adjust real numbers based on actual current counts).

## Testing strategy

Per established patterns:

1. **Service-layer tests** (httptest fixtures): list, get, feed-get, feed-post (verify `HoursWorked` is sent, verify `PercentComplete *int` distinguishes nil from 0).
2. **CLI runner tests** (stub-based): list/show/feed/update happy paths, `--yes` enforcement on mutations, `--percent 100` vs `--complete` equivalence, `update` rejects all-empty input.
3. **MCP tests:** schema-name checks per tool; `confirm:true` enforcement on mutating tools.
4. **Live verification** before tag — see below.

## Live verification before tagging

Use test ticket 542034 (already exists from v0.16.0 verification). The implementer may need to create a fresh test task on it via raw API (since `tdx ticket task create` is out of scope) — that's fine; document the curl snippet in the live-verify task. Then exercise every command:

1. `tdx ticket task list 542034` → at least 1 row (the one created via raw API)
2. `tdx ticket task show 542034 <task-id>` → full detail
3. `tdx ticket task feed 542034 <task-id>` → empty or system events
4. `tdx ticket task update 542034 <task-id> --percent 50 --comment "halfway" --yes` → feed entry visible in TD UI
5. `tdx ticket task update 542034 <task-id> --complete --yes` → percent flips to 100
6. `tdx ticket task log 542034 <task-id> --minutes 1 --type "<valid-type>" --yes` → time entry visible in `tdx time entry list` AND in TD time UI; check that the time entry shows the task as target (not just the parent ticket)

Clean up the test task at the end (set IsActive=false via raw PUT — DELETE is 403 for typical users).

## Branch + versioning

- Branch: `ticket-tasks`
- Version: v0.16.2 (no source-level version string; tagged via git after merge)

## Risks and mitigations

- **`--hours-worked` confusion.** Users may expect this to create a time entry. Mitigation: `--help` text and the docs explicitly call out that `--hours-worked` is informational only; for real time entries, use `tdx ticket task log`.
- **Tasks may not exist on most tickets in app 34.** Live probing of 40+ recent tickets returned 0 tasks — IT Tickets (the canonical UFL ticket app) doesn't use sub-tasks heavily. The commands still work; they just return "no tasks found." This is fine.
- **Permission edge case for `update`.** TD may gate task updates by ticket assignment / responsibility. Mitigation: surface TD's error response verbatim ("Permission denied" → user knows what's up).
- **Date fields with `0001-01-01T00:00:00`.** TD returns this for unset `CompletedDate`. `parseTDTime` returns zero `time.Time` for it; CLI rendering hides zero-time fields.

## Acceptance criteria

1. All 5 commands in `tdx ticket task --help` are wired.
2. `tdx ticket task list <id>` returns rows on a ticket with tasks; "no tasks found" otherwise.
3. `tdx ticket task show` displays percent complete plus all relevant fields.
4. `tdx ticket task update --percent 50 --yes` flips percent and adds a feed entry visible via `tdx ticket task feed`.
5. `tdx ticket task update --complete --yes` is equivalent to `--percent 100 --yes`.
6. `tdx ticket task log --minutes 1 --type "..." --yes` creates a time entry visible in `tdx time entry list` with the task as target (not just the parent ticket).
7. Both mutating commands (`update`, `log`) require `--yes`; running without it fails fast.
8. 5 new MCP tools registered; mutating ones require `confirm:true`; tool count = 62.
9. Docs updated (guide/ticket.md + tree in guide.md + tree in README.md byte-identical + mcp.md tool tables).
10. `go test ./... && go vet ./... && gofmt -l . && golangci-lint run ./...` all green.
11. Live-verified against UFL on ticket 542034 with a real test task.
12. Released as v0.16.2 (PR + squash + tag + Goreleaser).

## Out of scope (explicitly deferred)

- `tdx ticket task create` — admin workflow; multiple required fields; defer
- `tdx ticket task delete` — 403 for typical users; defer
- `tdx ticket task assign <ticket-id> <task-id> <uid|me>` — could be `--assignee` on `update`; defer
- Predecessor management (`isEligiblePredecessor` query param)
- Bulk task operations (e.g. mark all complete on a ticket)
- Cross-ticket task search
