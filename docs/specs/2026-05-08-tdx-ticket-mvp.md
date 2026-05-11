# tdx Ticket MVP (Phase D.1) Spec

**Date:** 2026-05-08
**Goal:** Add first-class TeamDynamix ticket support to `tdx` under a new `tdx ticket ...` namespace. Cover daily read-heavy workflows, low-risk mutations (status / assignee / comment), saved-search execution, and one strong ticket↔time crossover (`tdx ticket log`). Ship as v0.16.0.

## Motivation

`tdx` already manages time entries and people discovery against TeamDynamix. Tickets are the most-asked-for adjacent surface: every time entry's target is a ticket (or project task), and the daily workflow of "look at the ticket, do the work, log the time, move it forward" currently requires a browser context-switch. Bringing tickets into `tdx` closes the loop and makes the time-logging crossover possible without the user leaving the terminal.

## Decisions

Settled during brainstorming on 2026-05-08:

1. **Namespace under `tdx ticket ...`.** Mirrors `tdx time`, `tdx auth`, `tdx people`. (Confirmed by user framing.)
2. **MVP slice = read-heavy + light mutations + saved searches + 1 crossover.** No `create`, no `update` beyond status/assignee, no tasks, no workflow approvals, no attachments/tags/contacts/assets. (Q3.)
3. **Per-profile default appId, plus `--app <id>` override.** Set with `tdx ticket app use <id>`; inspect with `tdx ticket app show`; discover with `tdx ticket app list`. (Q1.)
4. **`tdx ticket log` ships in MVP** as the time crossover. Thin wrapper over existing `timesvc.AddEntry`. (Q2.)
5. **Saved-search execution ships in MVP** as `tdx ticket search saved` (list) and `tdx ticket search saved <name>` (run). (Q3.)
6. **Heading scheme** in new `docs/guide/ticket.md` follows the established command-as-heading pattern: `## tdx ticket search`, `### tdx ticket app use`, etc.

## Command surface (Phase D.1)

11 commands across 4 sub-groups. Tree shape:

```
tdx ticket
├── app            → list / use / show              # appId discovery + persistence
├── search                                           # filter-driven search (default: my open tickets)
│   └── saved      → list / run                     # saved-search execution
├── show <id>                                        # full ticket detail (+ this-week time logged)
├── feed <id>                                        # read feed entries
├── comment <id>                                     # post feed comment (mutating)
├── status <id>                                      # change status (mutating)
├── assign <id>                                      # change assignee (mutating)
├── log <id>                                         # log time to a ticket (mutating, time crossover)
├── types          → list                            # metadata: ticket types in current app
└── statuses       → list                            # metadata: statuses in current app
```

### Per-command spec

#### `tdx ticket app list`

Lists ticket apps in the tenant. Calls `GET /api/applications` and filters to AppType containing "Ticket" / "TDNext" platform apps with ticket capability. Outputs `ID | NAME | DESCRIPTION | ACTIVE`. `--json` envelope `tdx.v1.ticketAppList`.

If `/api/applications` doesn't exist or returns nothing useful, fallback: print a hint pointing the user to TD admin for app IDs. (Verify live before locking.)

#### `tdx ticket app use <id>`

Persists `ticketAppID` on the current profile in `~/.config/tdx/config.yaml`. Writes a confirmation line. No network call beyond an optional sanity probe (e.g. `GET /api/{id}/tickets/statuses` to confirm the app accepts ticket calls).

#### `tdx ticket app show`

Prints the current default ticket app for the active profile, or a hint if unset.

#### `tdx ticket search`

Filter-driven search via `POST /api/{appId}/tickets/search`. Flags:
- `--status <name|id>` (repeatable; resolves names to IDs via cached `/statuses`)
- `--assignee me|<UID>|<email>` (repeatable; "me" → authenticated UID)
- `--requestor me|<UID>|<email>` (repeatable)
- `--account <name>` (resolves via existing peoplesvc)
- `--text <query>` (free text)
- `--limit N` (default 50, capped at 1000)
- `--include-closed` (default: open only — TD's search will skip closed if status filter not set; we'll set the default explicitly)
- `--app <id>` (override profile default for one call)
- `--json` (envelope `tdx.v1.ticketList`)

**Default with no flags:** open tickets where assignee = authenticated user. Equivalent to `--assignee me --include-closed=false`. The most common workflow.

Returns partial records (TD limitation — no description/tasks/attributes). Output table: `ID | TITLE | STATUS | TYPE | ASSIGNEE | REQUESTOR | MODIFIED`.

#### `tdx ticket search saved`

`tdx ticket search saved` (no args) — calls `GET /api/{appId}/tickets/searches` and lists saved searches: `ID | NAME | OWNER | DESCRIPTION`.

`tdx ticket search saved <name>` — resolves name to ID (case-insensitive exact match; ambiguous → error with candidate list), then calls `POST /api/{appId}/tickets/searches/{searchId}/results`. Same output table as `tdx ticket search`. `--limit` and `--json` honored.

Rate-limit handling: TD limits to 60 calls/min/IP for both endpoints. On HTTP 429, surface a clear "rate limited; wait 60s" error.

#### `tdx ticket show <id>`

`GET /api/{appId}/tickets/{id}`. Pretty-printed sections:
- Header: `#12345 — <Title>`
- Status, Type, Priority, Account, Assignee, Requestor, Created, Modified
- Description (markdown-ish; print as-is, indented)
- Estimated/Actual: `EST: 4h  ACT: 1.5h (TD)  |  this week: 0.5h (1 entry)`
- Tags (if any)

The "this week" line is computed locally from the user's current week draft (if any) by scanning entries with `Target.Type=Ticket AND Target.ID=<id>`. Never makes a TD API call for this — it's pure local enrichment.

`--json` envelope `tdx.v1.ticket`.

#### `tdx ticket feed <id>`

`GET /api/{appId}/tickets/{id}/feed`. Output chronologically (TD typically returns reverse-chron; we'll display oldest-first or newest-first based on convention check):

```
[2026-05-07 14:23] alice@uf.edu — comment
  Working on it — vendor reply due Friday.

[2026-05-07 09:12] bob@uf.edu — status changed to "In Progress"
```

Flags: `--limit N`, `--json` (envelope `tdx.v1.ticketFeed`).

#### `tdx ticket comment <id> "message"`

`POST /api/{appId}/tickets/{id}/feed` with the body's `Comments` field. Flags:
- `--private` — `IsPrivate=true` (internal note, not visible to requestor)
- `--notify <UID,UID...>` — adds `Notify` recipients
- `--yes` — required to send

No `--dry-run` (low-stakes operation; `--yes` plus the explicit message argument are sufficient gates).

#### `tdx ticket status <id> <name>`

Resolves `<name>` to a status ID via cached `GET /tickets/statuses`. If ambiguous (multiple statuses match the name), error with the candidates. Calls `PATCH /api/{appId}/tickets/{id}` with a single `replace` op on `StatusID`.

Flags: `--yes` (required); `--comment "..."` (optional accompanying feed comment, posted via separate POST after the status patch succeeds).

#### `tdx ticket assign <id> <uid|email|me>`

Resolves the principal:
- `me` → authenticated UID (from `tdx auth status`)
- email or partial name → reuse `peoplesvc.SearchUsers`/`LookupPeople` resolution; ambiguous → error
- raw UID → use directly

Calls `PATCH /api/{appId}/tickets/{id}` with `replace` on `ResponsibleUid`. Flags: `--yes` (required); `--comment "..."` optional.

#### `tdx ticket log <id>` (time crossover)

Logs a time entry against the ticket. Thin wrapper over existing `timesvc.AddEntry`. Flags:
- `--hours N` or `--minutes N` (one required)
- `--type "Development"` or `--type-id <int>` (one required, mirrors `tdx time entry add`)
- `--date YYYY-MM-DD` (default: today)
- `--description "..."` (optional)
- `--billable / --no-billable` (default: type-driven)
- `--yes` (required)

Internally constructs a `domain.EntryInput` with `Target = {Type: Ticket, ID: <id>}` and calls the same code path as `tdx time entry add`. On success, prints the same confirmation as the time command, plus a "(logged to ticket #<id>)" tag.

**Validation:** before submit, calls `TimeTypesForTarget` to confirm the type is valid for this ticket — same guard `tdx time entry add` uses.

#### `tdx ticket types list`

`GET /api/{appId}/tickets/types?isActive=true`. Table: `ID | NAME | DESCRIPTION | ACTIVE`. `--json` envelope `tdx.v1.ticketTypeList`.

#### `tdx ticket statuses list`

`GET /api/{appId}/tickets/statuses`. Table: `ID | NAME | IS-CLOSED | IS-DEFAULT | ORDER`. `--json` envelope `tdx.v1.ticketStatusList`.

## MCP tools (Phase D.1)

12 new tools. Naming follows the existing convention (`<verb>_<noun>`).

**Read-only (8 tools, no `confirm` required):**
- `list_ticket_apps`
- `list_ticket_types` (input: `appId?` — optional, falls back to profile default)
- `list_ticket_statuses` (input: `appId?`)
- `list_saved_searches` (input: `appId?`)
- `search_tickets` (inputs: filters mirroring CLI flags; `appId?`)
- `run_saved_search` (inputs: `searchId`, `appId?`, `limit?`)
- `get_ticket` (inputs: `id`, `appId?`)
- `get_ticket_feed` (inputs: `id`, `appId?`, `limit?`)

**Mutating (4 tools, all require `confirm: true`):**
- `add_ticket_comment` (inputs: `id`, `comments`, `isPrivate?`, `notify?`, `appId?`)
- `update_ticket_status` (inputs: `id`, `statusName` or `statusId`, `comment?`, `appId?`)
- `update_ticket_assignee` (inputs: `id`, `responsibleUid` or `responsibleEmail`, `comment?`, `appId?`)
- `log_ticket_time` (inputs: `id`, `hours` or `minutes`, `typeName` or `typeId`, `date?`, `description?`, `billable?`, `appId?`)

Tool count: 44 → 56.

`appId?` is optional in MCP inputs. Default resolution mirrors the CLI: profile-level `ticketAppID` first; explicit input overrides. If neither is set, return a clear error pointing to `tdx ticket app use`.

## Domain types

New file `internal/domain/ticket.go`:

```go
type TicketApp struct {
    ID          int
    Name        string
    Description string
    Active      bool
    AppType     string  // "TDNext", etc. — informational
}

type TicketStatus struct {
    ID        int
    Name      string
    IsClosed  bool
    IsDefault bool
    Order     float64
}

type TicketType struct {
    ID          int
    Name        string
    Description string
    Active      bool
}

type Ticket struct {
    ID                int
    AppID             int
    Title             string
    Description       string
    StatusID          int
    StatusName        string
    TypeID            int
    TypeName          string
    PriorityID        int
    PriorityName      string
    AccountID         int
    AccountName       string
    ResponsibleUID    string
    ResponsibleName   string
    RequestorUID      string
    RequestorName     string
    CreatedDate       time.Time
    ModifiedDate      time.Time
    EstimatedMinutes  int
    ActualMinutes     int
    Tags              []string
    // partial-record signal: set false on results from search/saved-search,
    // true on results from GET /{id}
    IsFull            bool
}

type TicketFeedEntry struct {
    ID         int
    AuthorUID  string
    AuthorName string
    CreatedAt  time.Time
    Body       string         // the comment text
    IsPrivate  bool
    EventKind  string         // "comment" | "statusChange" | "assignment" | "task" | ...
}

type TicketSearchFilter struct {
    AppID         int
    StatusIDs     []int
    AssigneeUIDs  []string
    RequestorUIDs []string
    AccountIDs    []int
    Text          string
    IncludeClosed bool
    Limit         int
}

type TicketSavedSearch struct {
    ID          int
    Name        string
    OwnerUID    string
    OwnerName   string
    Description string
}
```

Existing `domain.Target` already supports `TargetType=Ticket` (used by `tdx time entry add` today).

## Service layer

New package `internal/svc/ticketsvc/`:

- `ticketsvc/service.go` — constructor + shared helpers
- `ticketsvc/types.go` — wire types matching TD's JSON
- `ticketsvc/apps.go` — `ListApps`
- `ticketsvc/tickets.go` — `GetTicket`, `SearchTickets`, `PatchTicket`
- `ticketsvc/feed.go` — `GetFeed`, `AddFeed`
- `ticketsvc/saved_searches.go` — `ListSavedSearches`, `RunSavedSearch`
- `ticketsvc/metadata.go` — `ListStatuses`, `ListTypes` + name resolvers (`ResolveStatusByName`, `ResolveTypeByName`)

Same patterns as `peoplesvc`: thin wire structs, decoder functions to map to `domain.*`, `client.DoJSON` for transport.

`appId` resolution: services accept `(profileName, appID)` in every call. If `appID == 0`, the service reads the profile's `ticketAppID`; if still zero, returns a clear error. CLI layer is responsible for `--app` override; service layer does the fallback.

## CLI layer

New package `internal/cli/ticket/`:

- `ticket.go` — top-level `tdx ticket` cobra command + sub-command registration
- `app.go` — `app list/use/show`
- `search.go` — `search` + `search saved list/run`
- `show.go` — `show`
- `feed.go` — `feed`
- `comment.go` — `comment`
- `status.go` — `status`
- `assign.go` — `assign`
- `log.go` — `log` (time crossover)
- `types.go` — `types list`
- `statuses.go` — `statuses list`
- `helpers.go` — shared `resolveAppID`, `resolvePrincipal`, `resolveStatus` etc.

Pattern: each file exposes a runner function (`runTicketShow`, `runTicketSearch`, etc.) plus a `newXCmd(svc ticketsvcAPI) *cobra.Command` factory. Tests target the runners directly. Mirrors `internal/cli/people/` exactly.

## Config layer

Extend profile config:

```yaml
profiles:
  default:
    url: https://yourorg.teamdynamix.com/
    ticketAppID: 31    # NEW — optional
```

Existing `internal/domain/profile.go` (or wherever) gets a `TicketAppID int` field, JSON/YAML tag `ticketAppID,omitempty`. Backwards compatible — existing profiles without the field work unchanged for `tdx auth`/`tdx time`/`tdx people`.

`tdx ticket app use <id>` writes via the existing config-write path. Sanity probe before persisting: `GET /api/{id}/tickets/statuses` — if 200, accept; if 404/403, refuse with explanation.

## JSON envelopes (additive)

New schema names, all under the `tdx.v1.*` family:

- `tdx.v1.ticketAppList` — output of `app list`
- `tdx.v1.ticketTypeList`
- `tdx.v1.ticketStatusList`
- `tdx.v1.ticketSavedSearchList`
- `tdx.v1.ticketList` — search results (partial records)
- `tdx.v1.ticket` — single ticket (full)
- `tdx.v1.ticketFeed` — feed list
- `tdx.v1.ticketCommentResult` — comment-add result
- `tdx.v1.ticketUpdateResult` — status/assign update result
- `tdx.v1.ticketTimeLogResult` — time-log result (mirrors timesvc's existing `tdx.v1.timeEntry`)

## Documentation

New file: `docs/guide/ticket.md`. Heading scheme:

```markdown
# tdx ticket

[intro]

## Contents
[TOC]

## tdx ticket app
### tdx ticket app list
### tdx ticket app use
### tdx ticket app show

## tdx ticket search
[main search]
### tdx ticket search saved
#### tdx ticket search saved list
#### tdx ticket search saved run

## tdx ticket show
## tdx ticket feed
## tdx ticket comment
## tdx ticket status
## tdx ticket assign
## tdx ticket log

## tdx ticket types
### tdx ticket types list

## tdx ticket statuses
### tdx ticket statuses list
```

Updates:
- `docs/guide.md` ASCII command tree: add the `ticket` branch (2 levels deep, matching existing format)
- `README.md` ASCII command tree: same change (must stay byte-identical)
- `docs/guide/mcp.md`: add a new heading `#### Tickets (Phase D — read-only, 8 tools)` and `#### Tickets (Phase D — mutating, 4 tools)` with the 12 new tools listed
- Reference list in `docs/guide.md`: add `[tdx ticket](guide/ticket.md)` line

## Testing strategy

Per established patterns:

1. **Unit tests on pure runner functions** (mocked `ticketsvcAPI` interface).
2. **Service-layer tests** with `httptest.Server` fixtures. Fixtures captured from live Sample responses (per `feedback_probe_wire_formats_early.md` — never trust the docs alone).
3. **CLI tests** via runner-function calls; cobra layer not exercised in tests.
4. **MCP tool tests** in `internal/mcp/tools_ticket_test.go` mirroring `tools_people_test.go`.

**Live probe before locking fixtures:**
- Pick a real Sample ticket app (one user has access to)
- Capture: `/applications` (apps list), `/{appId}/tickets/statuses`, `/{appId}/tickets/types`, `/{appId}/tickets/searches`, `POST /{appId}/tickets/search` (small filter), `GET /{appId}/tickets/{id}`, `GET /{appId}/tickets/{id}/feed`
- Save sanitized JSON snippets into `internal/svc/ticketsvc/testdata/`
- Verify wire field names and time-zone handling on dates

## Live verification before tagging

Before opening the PR for review:

1. `tdx ticket app list` — at least one app appears
2. `tdx ticket app use <id>` — config persists; sanity probe passes
3. `tdx ticket types list` and `tdx ticket statuses list` return real metadata
4. `tdx ticket search` (default = my open) returns reasonable result set
5. `tdx ticket search saved` lists at least one saved search; `tdx ticket search saved <name>` runs
6. `tdx ticket show <real-id>` displays full detail; "this week" line accurate against current draft
7. `tdx ticket feed <id>` matches what TD's web UI shows
8. `tdx ticket comment <id> "test from tdx" --yes` posts and is visible in TD UI
9. `tdx ticket status <id> "<status>" --yes` flips status; visible in UI
10. `tdx ticket assign <id> me --yes` reassigns; visible in UI
11. `tdx ticket log <id> --minutes 1 --type "<type>" --yes` creates a 1-minute time entry against the ticket; visible in `tdx time entry list` and TD time entry UI

Each of the 4 mutating commands tested on a low-stakes test ticket. Roll back any test changes (delete test comments, restore original status/assignee) after verification.

## Out of scope (explicitly deferred)

- `tdx ticket create` — needs form selection; ticket forms have many required fields; defer to D.2 with proper interactive form fill
- `tdx ticket update` — generic field updates beyond status/assignee; safer to ship one-at-a-time mutators in MVP, generalize later if needed
- `tdx ticket task ...` — full task subtree (list/create/complete/log time); defer to D.3
- `tdx ticket workflow ...` — workflow approve/reassign/actions; defer to D.3
- Attachments, tags, contacts, assets, CIs, classification, SLA — all defer
- Inverse crossover sugar `tdx time entry add ticket://12345 ...` — defer to D.2; current target flag works
- MCP `create_ticket`, `update_ticket_field`, etc. — track with the deferred CLI commands

## Risks and mitigations

- **`/api/applications` may not exist or may return apps unrelated to tickets.** Mitigation: live-probe first; if needed, fallback is to print a hint pointing the user to the TD admin UI for app IDs. Worst case: `app list` becomes a stub with a help message; everything else still works once the user knows their app ID.
- **Saved searches return partial records that omit important fields.** Mitigation: `tdx ticket search saved <name>` output explicitly notes "partial — use `tdx ticket show <id>` for full detail" in the table footer when results are partial.
- **`PATCH /tickets/{id}` may have non-obvious failure modes (read-only fields, workflow-locked statuses).** Mitigation: surface TD's error response verbatim; document common cases in `docs/guide/ticket.md`.
- **Status-name resolution is ambiguous in some apps** (e.g. multiple "Closed" variants). Mitigation: ambiguous-match returns a candidate list error; users can pass `--status-id <int>` for unambiguous selection.
- **TD search-endpoint silent-filter pattern** (per `reference_td_search_silent_filters.md`). Mitigation: probe live which `TicketSearch` fields the sample tenant honors before relying on them; fall back to client-side filtering for any silently-ignored fields.
- **Rate limit on saved searches (60/min/IP).** Mitigation: surface 429 errors with a clear message; CLI doesn't auto-retry.

## Acceptance criteria

1. All 11 commands in the tree shape above are wired and discoverable via `tdx ticket --help`.
2. Default (no-flag) `tdx ticket search` returns the user's open tickets without further setup, given that `tdx ticket app use <id>` has been run.
3. `tdx ticket show <id>` displays both TD's `ActualMinutes` and the locally-computed "this week" hours line; the local line is correct for at least one verified ticket-and-week pair.
4. `tdx ticket log <id>` creates a time entry visible in `tdx time entry list` AND TD's time-entry UI for that ticket.
5. All 4 mutating commands require `--yes`; running without it fails fast with a clear preview/instruction.
6. All 12 MCP tools are registered; mutating ones require `confirm: true`; tool count reaches 56.
7. New `docs/guide/ticket.md` exists with full per-command reference; ASCII tree updated in `guide.md` and `README.md` (byte-identical); MCP tool tables in `guide/mcp.md` updated.
8. `go test ./... && go vet ./... && gofmt -l . && golangci-lint run ./...` all green.
9. Live-verified against the test tenant on at least one real ticket app for every command.
10. Released as v0.16.0 (PR + squash-merge + tag + Goreleaser run).
