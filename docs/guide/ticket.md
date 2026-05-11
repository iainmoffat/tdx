# tdx ticket

`tdx ticket` exposes TeamDynamix ticket management: search and browse tickets, read feed activity, post comments, change status, reassign, and log time worked — all from the terminal or via MCP.

Every ticket command targets a specific **ticket app** (the TD concept that partitions tickets by team/system). Set a per-profile default with `tdx ticket app use <id>`; that value is stored in `~/.config/tdx/config.yaml` as `ticketAppID`. Override it for any single call with `--app <id>`. Commands that talk to TD error out if no app is resolved.

**Partial vs full records.** Search results and saved-search results are partial: they contain status, assignee, type, and dates but omit description, tasks, and time detail. Use `tdx ticket show <id>` to fetch a full record.

**Mutations require `--yes`.** `tdx ticket comment`, `tdx ticket status`, `tdx ticket assign`, and `tdx ticket log` all require `--yes` before writing to TD. Omitting `--yes` prints a dry-run preview and exits.

All commands accept `--profile <name>` to override the active profile.

## Contents

- [tdx ticket app](#tdx-ticket-app)
- [tdx ticket search](#tdx-ticket-search)
- [tdx ticket show](#tdx-ticket-show)
- [tdx ticket feed](#tdx-ticket-feed)
- [tdx ticket comment](#tdx-ticket-comment)
- [tdx ticket status](#tdx-ticket-status)
- [tdx ticket assign](#tdx-ticket-assign)
- [tdx ticket update](#tdx-ticket-update)
- [tdx ticket log](#tdx-ticket-log)
- [tdx ticket types](#tdx-ticket-types)
- [tdx ticket statuses](#tdx-ticket-statuses)
- [tdx ticket groups](#tdx-ticket-groups)
- [tdx ticket task](#tdx-ticket-task)

---

## tdx ticket app

Manage the per-profile default ticket app. Resolution order: `--app <id>` flag > `profile.ticketAppID` in config > error. Running `tdx ticket app use <id>` writes the default into the active profile's config entry; you only need to do this once per profile.

### tdx ticket app list

List all ticket apps visible to the authenticated user. Use this to discover the app ID to pass to `tdx ticket app use`.

```bash
tdx ticket app list
tdx ticket app list --json     # schema: tdx.v1.ticketAppList
```

**Flags:**

- `--json` — emit JSON instead of a table

### tdx ticket app use

Set the default ticket app for the active profile. Writes `ticketAppID` to `~/.config/tdx/config.yaml`.

```bash
tdx ticket app use 42
```

No flags beyond `--profile`.

### tdx ticket app show

Show the currently configured default ticket app for the active profile. Useful to confirm what app is active before running a search.

```bash
tdx ticket app show
```

No flags beyond `--profile`. Exits with a clear error if no default is set.

---

## tdx ticket search

Search for tickets. Without any flags, returns your open assigned tickets (equivalent to `--assignee me --include-closed=false`).

```bash
# My open tickets (default)
tdx ticket search

# Tickets assigned to me or a colleague, any status
tdx ticket search --assignee me --assignee aaaaaaaa-1234-5678-9abc-000000000001 --include-closed

# Full-text search, cap at 20
tdx ticket search --text "database migration" --limit 20

# Tickets requested by a specific user, in a different app
tdx ticket search --requestor alice@example.com --app 99 --json

# Tickets assigned to my team (direct reports), open only
tdx ticket search --manager me

# Tickets assigned to a specific group
tdx ticket search --responsibility-group "Linux Platform Services"

# Mixed: my open tickets PLUS Linux Team's
tdx ticket search --assignee me --responsibility-group "Linux Platform Services"
```

Output is a partial-record table: ID, TITLE, STATUS, ASSIGNEE, REQUESTOR, MODIFIED. The records omit description and time data; use `tdx ticket show <id>` for full detail.

**Flags:**

- `--status <name|id>` — filter by status name or ID; repeatable (OR logic); case-insensitive
- `--assignee <me|UID|email>` — filter by assignee; repeatable; defaults to `me` when no assignee/requestor flag is given
- `--requestor <me|UID|email>` — filter by requestor; repeatable
- `--responsibility-group <name|id>` — filter to tickets assigned to a TD responsibility group (team). Repeatable. Numeric arg = ID; non-numeric = case-insensitive exact name match (errors with candidate list on ambiguity). Use `tdx ticket groups list` to discover groups.
- `--manager me|UID|email` — expand to "tickets assigned to direct reports of this person." Repeatable. `me` = the authenticated user. Resolution: fetches employees (~1k staff on the test tenant) and filters client-side by `ReportsToUID`, then injects matching UIDs as assignees. Direct reports only (no transitive walk).
- `--account <name>` — informational label only (not a TD-side filter); name-resolves for display
- `--text <q>` — full-text search string
- `--limit N` — max results (default 50, max 1000)
- `--include-closed` — include closed/resolved tickets (default: open only)
- `--app <id>` — override the profile default ticket app
- `--json` — emit JSON; schema: `tdx.v1.ticketList` (partial records)

### tdx ticket search saved

Two modes depending on whether a NAME argument is supplied:

- **No argument**: list all saved searches visible to the user.
- **With NAME**: run the named saved search and return results.

```bash
# List available saved searches
tdx ticket search saved

# Run a saved search by name
tdx ticket search saved "My Team Open Tickets"

# Cap saved-search results and emit JSON
tdx ticket search saved "My Team Open Tickets" --limit 100 --json
```

Saved searches return **partial records** (same shape as `tdx ticket search`). A banner is printed below the table reminding you to use `tdx ticket show <id>` for full detail.

TD rate-limits saved-search execution to 60 calls/min/IP. If you hit the limit, tdx surfaces the 429 clearly and suggests waiting before retrying.

**Flags:**

- `--app <id>` — override the profile default ticket app
- `--limit N` — cap result count
- `--json` — list mode: schema `tdx.v1.ticketSavedSearchList`; run mode: schema `tdx.v1.ticketList`

---

## tdx ticket show

Fetch the full record for a single ticket. Includes description, tasks, and a locally-computed **this-week time block** that shows hours and entry count for the current week draft without an extra API call.

```bash
tdx ticket show 12345
tdx ticket show 12345 --app 42 --json
```

The table output includes TD's EstimatedMinutes and ActualMinutes plus a "this week" line (e.g., `this week: 2.5h (3 entries)`) derived from the active profile's current week draft. Project rows are excluded from that count.

**JSON schema:** `tdx.v1.ticket` — full record with a `thisWeek` block (`hours`, `entries`).

**Flags:**

- `--app <id>` — override the profile default ticket app
- `--json` — emit full-record JSON

---

## tdx ticket feed

Read the activity feed for a ticket: comments, status changes, and system events.

```bash
# Last 10 feed entries (default)
tdx ticket feed 12345

# All entries
tdx ticket feed 12345 --limit 0

# JSON
tdx ticket feed 12345 --limit 25 --json
```

**Flags:**

- `--app <id>` — override the profile default ticket app
- `--limit N` — max entries to fetch; `0` returns all (default 10)
- `--json` — emit JSON; schema: `tdx.v1.ticketFeed`

---

## tdx ticket comment

Post a comment to a ticket's feed. Requires `--yes`.

```bash
# Post a public comment (dry-run without --yes)
tdx ticket comment 12345 "Deployed the fix to staging." --yes

# Private comment, notify two colleagues
tdx ticket comment 12345 "Escalating — see notes." \
  --private \
  --notify aaaaaaaa-1234-5678-9abc-000000000001 \
  --notify aaaaaaaa-1234-5678-9abc-000000000002 \
  --yes
```

Without `--yes`, tdx prints what it would post and exits. This applies to all mutating ticket commands.

**Flags:**

- `--app <id>` — override the profile default ticket app
- `--private` — mark the comment as private (visible to staff only)
- `--notify <UID>` — UID of a user to notify; repeatable
- `--yes` — **required** to write; omit for a dry-run preview

---

## tdx ticket status

Change a ticket's status. Requires `--yes`.

Status names are resolved case-insensitively. If the name matches multiple statuses, tdx errors with a candidate list; use `--status-id <int>` to disambiguate.

```bash
# Change by name
tdx ticket status 12345 "In Progress" --yes

# Change with a comment
tdx ticket status 12345 "Resolved" --comment "Fixed in build 4.2.1." --yes

# Disambiguate by ID
tdx ticket status 12345 "Closed" --status-id 17 --yes
```

**Flags:**

- `--app <id>` — override the profile default ticket app
- `--status-id <int>` — disambiguate when multiple statuses share the same name
- `--comment <text>` — optional comment to post with the status change
- `--yes` — **required** to write

---

## tdx ticket assign

Reassign a ticket to a different user. Requires `--yes`.

The principal argument accepts a TD UID, an email address, or the literal `me`.

```bash
# Assign to yourself
tdx ticket assign 12345 me --yes

# Assign by email
tdx ticket assign 12345 alice@example.com --yes

# Assign by UID with a comment
tdx ticket assign 12345 aaaaaaaa-1234-5678-9abc-000000000001 \
  --comment "Taking this over from Alice." \
  --yes
```

**Flags:**

- `--app <id>` — override the profile default ticket app
- `--comment <text>` — optional comment posted with the assignment change
- `--yes` — **required** to write

---

## tdx ticket update

Update editable ticket fields. **Mutating** — `--yes` required. Excludes status (use `tdx ticket status`) and assignee (use `tdx ticket assign`).

```bash
tdx ticket update <id> --title "New title" --yes
tdx ticket update <id> --description "..." --comment "rolled back" --yes
tdx ticket update <id> --type "Incident" --priority-id 3 --yes
tdx ticket update <id> --requestor alice@uf.edu --yes
tdx ticket update <id> --responsibility-group "Linux Team" --yes
```

Flags:

- `--title "<text>"` — replace ticket title
- `--description "<text>"` — replace ticket description (full replacement; `--description ""` clears it)
- `--type <name|id>` — set ticket type by name (case-insensitive exact match) or numeric id
- `--account <name|id>` — set account by name (resolved via `tdx people accounts list`) or numeric id
- `--requestor <uid|email|me>` — set requestor; `me` = the authenticated user
- `--responsibility-group <name|id>` — set responsibility group by name or id (use `tdx ticket groups list` to discover names)
- `--priority-id <int>` — set priority by numeric id (priority-name resolution not supported in v0.16.3)
- `--comment "<text>"` — optional accompanying feed comment posted after the PATCH succeeds; comment-only invocations (no field flags) are valid and just post the comment

At least one of the field flags or `--comment` must be set. The PATCH is sent as a single TD call; if any field is rejected, nothing applies. JSON output is the full updated ticket (`tdx.v1.ticket` envelope).

---

## tdx ticket log

Log time worked against a ticket. Requires `--yes`.

This is a thin wrapper over `tdx time entry add` that creates a `Target{Kind: Ticket, AppID, ItemID}` time entry. The time type must be allowed for the ticket; tdx validates this via `TimeTypesForTarget` before writing.

`--billable` inherits the time type's default flag when omitted. Pass `--billable=false` to override a type that is billable by default.

```bash
# Log 1.5 hours today, resolving type by name (dry-run)
tdx ticket log 12345 --hours 1.5 --type "Development"

# Confirm the write
tdx ticket log 12345 --hours 1.5 --type "Development" --yes

# Log 90 minutes, specific date, explicit billable override
tdx ticket log 12345 --minutes 90 --type-id 7 \
  --date 2026-05-06 \
  --description "Root-cause analysis" \
  --billable=false \
  --yes
```

**Flags:**

- `--app <id>` — override the profile default ticket app
- `--hours N` — time to log in hours (mutually exclusive with `--minutes`)
- `--minutes N` — time to log in minutes (mutually exclusive with `--hours`); one of the two is required
- `--type <name>` — time type by name (case-insensitive); mutually exclusive with `--type-id`
- `--type-id <int>` — time type by numeric ID; mutually exclusive with `--type`; one of the two is required
- `--date YYYY-MM-DD` — date for the entry (default: today in America/New_York)
- `--description <text>` — free-text description for the time entry
- `--billable` — mark the entry billable (default: inherit from the time type)
- `--yes` — **required** to write

---

## tdx ticket types

List the ticket types defined in an app. Use this to discover the type names used in ticket creation or filtering.

### tdx ticket types list

```bash
tdx ticket types list                  # uses profile default app
tdx ticket types list --app 42
tdx ticket types list --app 42 --json  # schema: tdx.v1.ticketTypeList
```

**Flags:**

- `--app <id>` — override the profile default ticket app
- `--json` — emit JSON; schema: `tdx.v1.ticketTypeList`

---

## tdx ticket statuses

List the ticket statuses defined in an app. Use this to discover valid status names for `tdx ticket search --status` and `tdx ticket status`.

### tdx ticket statuses list

```bash
tdx ticket statuses list                  # uses profile default app
tdx ticket statuses list --app 42
tdx ticket statuses list --app 42 --json  # schema: tdx.v1.ticketStatusList
```

**Flags:**

- `--app <id>` — override the profile default ticket app
- `--json` — emit JSON; schema: `tdx.v1.ticketStatusList`

---

## tdx ticket groups

Inspect tenant-wide ticket responsibility groups (teams that tickets can be assigned to as a group rather than to an individual).

### tdx ticket groups list

```bash
tdx ticket groups list
tdx ticket groups list --json
```

Output: `ID | NAME | ACTIVE` table. JSON envelope: `tdx.v1.ticketGroupList`.

Groups are tenant-wide — the same group can serve multiple ticket apps. Use `--responsibility-group <name>` on `tdx ticket search` once you know the group you care about.

**Flags:**

- `--json` — emit JSON; schema: `tdx.v1.ticketGroupList`

---

## tdx ticket task

Manage tasks on a ticket: list, view detail, read feed, update progress, and log time worked.

### tdx ticket task list

```bash
tdx ticket task list <ticket-id>
tdx ticket task list <ticket-id> --json
```

Output: `ID | TITLE | %COMPLETE | EST | ACT | RESPONSIBLE` table. JSON envelope: `tdx.v1.ticketTaskList`.

### tdx ticket task show

```bash
tdx ticket task show <ticket-id> <task-id>
```

Pretty-printed sections: header, progress, responsible person/group, dates, time, description. JSON envelope: `tdx.v1.ticketTask`.

### tdx ticket task feed

```bash
tdx ticket task feed <ticket-id> <task-id>
tdx ticket task feed <ticket-id> <task-id> --limit 5
```

Reads task feed entries (comments + system events). JSON envelope: `tdx.v1.ticketTaskFeed`.

### tdx ticket task update

```bash
tdx ticket task update <ticket-id> <task-id> --percent 50 --comment "halfway" --yes
tdx ticket task update <ticket-id> <task-id> --complete --yes
```

Posts a feed update to the task. **Mutating** — `--yes` required.

Flags:

- `--percent N` (0-100) or `--complete` (shortcut for `--percent 100`); mutually exclusive
- `--comment "..."` — comment body
- `--hours-worked N` — informational only; does NOT create a time entry. Use `tdx ticket task log` for real time tracking.
- `--private` — internal note (not visible to requestor)
- `--notify <UID>` — additional notify recipients (repeatable)

### tdx ticket task log

```bash
tdx ticket task log <ticket-id> <task-id> --hours 1.5 --type "Development" --yes
tdx ticket task log <ticket-id> <task-id> --minutes 30 --type-id 7 --description "fixed bug" --yes
```

Creates a real time entry against the task. **Mutating** — `--yes` required. Wraps `tdx time entry add`. Same flag shape as `tdx ticket log`.
