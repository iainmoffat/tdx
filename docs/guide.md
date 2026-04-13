# tdx User Guide

This guide covers all tdx features in depth. For a quick reference of
commands and flags, see the [README](../README.md).

---

## Table of Contents

- [Concepts](#concepts)
- [Authentication](#authentication)
- [Profiles](#profiles)
- [Time Entries](#time-entries)
- [Week View](#week-view)
- [Time Types](#time-types)
- [Templates](#templates)
- [MCP Server](#mcp-server)
- [JSON Output](#json-output)
- [Shell Completions](#shell-completions)

---

## Concepts

tdx manages **time entries** in [TeamDynamix](https://www.teamdynamix.com/).
Each entry records hours worked on a specific date against a **target**
(ticket, project, or workspace) with a **time type** (e.g. "Development",
"Planning").

A **week** in TD runs Sunday through Saturday. All dates are interpreted in
Eastern time (America/New_York), matching how TD computes billing periods.

A **template** is a saved weekly pattern of time entries that can be applied
to any future week. Templates are the core productivity feature of tdx:
derive one from a representative week, then replay it whenever you need to.

---

## Authentication

### First login

```bash
tdx auth login --url https://yourorg.teamdynamix.com/
```

You'll be prompted to paste a TD API token. To get one, log in to
TeamDynamix in your browser and navigate to your profile's API token view.

### SSO login

If your organization uses SSO, the `--sso` flag opens the TD SSO endpoint
in your browser. Complete the SSO flow, then copy the token displayed:

```bash
tdx auth login --sso
```

The SSO endpoint issues a fresh 24-hour JWT each time it's called with a
valid TD session cookie.

### Scripted login

For CI or scripts, pipe the token via stdin:

```bash
echo "$TOKEN" | tdx auth login --token-stdin --profile default --url https://yourorg.teamdynamix.com/
```

### Check your session

```bash
tdx auth status
```

Shows your user identity, tenant URL, and profile name.

### Log out

```bash
tdx auth logout
```

Removes the stored token and profile.

---

## Profiles

Profiles let you manage multiple TD tenants or accounts. Each profile stores
a tenant URL and authentication token independently.

### Add a profile

```bash
tdx auth profile add work --url https://work.teamdynamix.com/
tdx auth profile add personal --url https://personal.teamdynamix.com/
```

The first profile added automatically becomes the default.

### Switch the default

```bash
tdx auth profile use personal
```

### List profiles

```bash
tdx auth profile list
```

The default profile is marked with `*`.

### Use a specific profile for one command

Every command accepts `--profile`:

```bash
tdx time entry list --profile work
tdx time entry list --profile personal
```

### Remove a profile

```bash
tdx auth profile remove personal
```

---

## Time Entries

### List entries

By default, lists the current week's entries:

```bash
tdx time entry list
```

List a specific week (any date within the week works):

```bash
tdx time entry list --week 2026-04-07
```

Use an explicit date range:

```bash
tdx time entry list --from 2026-04-01 --to 2026-04-30
```

`--week` and `--from`/`--to` are mutually exclusive.

### Filter entries

Filter by time type name (case-insensitive substring match):

```bash
tdx time entry list --type development
```

Filter by ticket (requires `--app` for the TD application ID):

```bash
tdx time entry list --ticket 12345 --app 42
```

### Show a single entry

```bash
tdx time entry show 98765
```

### Create an entry

```bash
tdx time entry add \
  --date 2026-04-07 \
  --hours 2 \
  --type "Development" \
  --project 54 \
  -d "API implementation"
```

**Duration** must be exactly one of `--hours` or `--minutes`.

**Target** must be exactly one of:
- `--ticket <id> --app <id>` (ticket requires app)
- `--project <id>` (optionally with `--plan <id> --task <id>`)
- `--workspace <id> --app <id>`

Preview without creating:

```bash
tdx time entry add --date 2026-04-07 --hours 2 --type Dev --project 54 --dry-run
```

### Update an entry

```bash
tdx time entry update 98765 --hours 3 -d "updated description"
```

Only the flags you pass are changed; everything else stays the same.

### Delete entries

```bash
tdx time entry delete 98765
tdx time entry delete 98765 98766 98767   # multiple IDs
```

---

## Week View

The week grid shows all entries for a week in a compact table:

```bash
tdx time week show
tdx time week show 2026-04-07
```

Output:

```
Week 2026-04-05 - 2026-04-11  (open)

  ROW              SUN    MON    TUE    WED    THU    FRI    SAT    TOTAL
  -----------------------------------------------------------------------
  ticket #123      .      8.0    8.0    8.0    8.0    8.0    .      40.0
    L Work
  project 456      .      .      4.0    .      .      4.0    .      8.0
    L Planning
  -----------------------------------------------------------------------
  DAY TOTAL        .      8.0    12.0   8.0    8.0    12.0   .      48.0
```

Empty cells show `.` for clean scanning. Rows are grouped by target and
time type.

### Locked days

Some days may be administratively locked (no edits allowed):

```bash
tdx time week locked
```

---

## Time Types

### List all types

```bash
tdx time type list
```

### Find valid types for a work item

```bash
tdx time type for ticket 12345 --app 42
tdx time type for project 54
```

### How type matching works

When you use `--type` in entry commands, tdx matches by name
(case-insensitive). For example, `--type dev` matches a type named
"Development". If no match is found, the command errors with the available
type names.

---

## Templates

Templates are the core feature of tdx. The workflow is:

1. **Derive** a template from a week with known good data
2. **Edit** hours if needed (optional)
3. **Show** or **compare** to verify it looks right
4. **Apply** it to future weeks

### Derive a template

```bash
tdx time template derive my-week --from-week 2026-04-07
```

This fetches all entries from the week containing April 7 and groups them
into template rows. Entries with the same target, time type, and billable
flag are folded into one row with accumulated hours per day. The most common
description across grouped entries is used for the row.

### List templates

```bash
tdx time template list
```

### Show a template

```bash
tdx time template show my-week
```

Displays the template as a grid, similar to the week view but showing the
template's hour pattern rather than live data.

### Edit a template

Open the interactive grid editor to adjust hour values:

```bash
tdx time template edit my-week
```

The editor shows the template as a navigable grid. Use arrow keys or Tab
to move between cells, then adjust values:

| Key | Action |
|-----|--------|
| Arrow keys / Tab | Navigate between cells |
| 0-9, `.` | Type a value (snaps to nearest 0.5 on Enter) |
| Enter | Confirm typed value and advance to next cell |
| Backspace | Clear cell to 0 |
| Ctrl-S | Save and exit |
| Esc | Cancel (prompts if unsaved changes) |

Values are constrained to 0.5-hour increments between 0 and 24 hours.
Row totals and day totals update live as you edit.

This is useful for adjusting a derived template before applying it — for
example, reducing Friday hours for a short week, or zeroing out rows you
don't need this time.

### Clone a template

```bash
tdx time template clone my-week my-week-v2
```

Creates a copy under a new name. Useful for making variations.

### Delete a template

```bash
tdx time template delete my-week
```

### Compare template vs live week

Before applying, see what would change:

```bash
tdx time template compare my-week --week 2026-04-14
```

The output shows the template grid annotated with action markers:

| Marker | Meaning |
|--------|---------|
| `+` | Entry will be created |
| `~` | Existing entry will be updated |
| `=` | Existing entry matches, will be skipped |
| `x` | Day is blocked (locked or submitted) |

### Apply a template

Apply is a two-step process for safety: preview first, then confirm.

**Preview (dry run):**

```bash
tdx time template apply my-week --week 2026-04-14 --dry-run
```

This shows the same annotated grid as `compare`, plus a summary of actions.

**Apply:**

```bash
tdx time template apply my-week --week 2026-04-14 --yes
```

The `--yes` flag is required to actually write changes. Without it, tdx
shows the preview and exits.

**Race protection:** When you pass `--yes`, tdx re-computes the diff before
applying and verifies it matches the preview. If someone else modified the
week between your preview and apply, the hashes won't match and the apply
is rejected. This prevents accidental overwrites.

### Apply modes

The `--mode` flag controls how existing entries are handled:

**`add` (default):**
Creates new entries for each template row/day. If a matching entry already
exists (same target, type, and date), it's skipped — no duplicates are
created, but existing entries are never modified.

```bash
tdx time template apply my-week --week 2026-04-14 --mode add --yes
```

**`replace-matching`:**
Like `add`, but if a matching entry exists with different values (e.g.
different hours), it's updated to match the template. Entries that already
match exactly are skipped.

```bash
tdx time template apply my-week --week 2026-04-14 --mode replace-matching --yes
```

**`replace-mine`:**
Only updates entries that tdx previously created from this template. Uses
ownership markers (described below) embedded in entry descriptions to track
provenance. Entries not created by tdx are left untouched, even if they
match by target and type.

```bash
tdx time template apply my-week --week 2026-04-14 --mode replace-mine --yes
```

### Ownership markers

When tdx creates an entry in `replace-mine` mode, it appends a marker to
the description:

```
API implementation [tdx:my-week#row-01]
```

The marker format is `[tdx:<template-name>#<row-id>]`. On future applies,
tdx uses this marker to identify which entries it owns. Entries without the
marker (created manually or by other tools) are never modified.

The marker is stripped from display output but preserved in the stored entry.

### Day filtering

Restrict an apply to specific days:

```bash
# Range: Monday through Friday
tdx time template apply my-week --week 2026-04-14 --days mon-fri --yes

# Specific days
tdx time template apply my-week --week 2026-04-14 --days mon,wed,fri --yes
```

Day names are three-letter abbreviations (case-insensitive): `sun`, `mon`,
`tue`, `wed`, `thu`, `fri`, `sat`.

### Hour overrides

Override specific row/day hours for a single apply without changing the
saved template:

```bash
tdx time template apply my-week --week 2026-04-14 \
  --override row-01:fri=4 \
  --override row-02:mon=0 \
  --yes
```

The syntax is `--override <row-id>:<day>=<hours>`. Use `tdx time template
show my-week --json` to find row IDs.

Setting hours to `0` skips that cell entirely. This is useful for holidays
or partial weeks.

### Rounding

If a template has fractional hours that produce non-integer minutes (e.g.
1.333 hours = 79.98 minutes), tdx errors by default. Pass `--round` to
allow rounding to the nearest whole minute:

```bash
tdx time template apply my-week --week 2026-04-14 --round --yes
```

---

## MCP Server

tdx includes an MCP (Model Context Protocol) server that exposes all
functionality to AI agents like Claude.

### Start the server

```bash
tdx mcp serve
```

The server runs over stdio and speaks the MCP protocol. It's designed to be
launched by AI tools, not run manually.

### Configure in your AI tool

Add tdx to your AI tool's MCP configuration:

**Claude Code** (`~/.claude/settings.json`):

```json
{
  "mcpServers": {
    "tdx": {
      "command": "tdx",
      "args": ["mcp", "serve"]
    }
  }
}
```

**Cursor** (`.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "tdx": {
      "command": "tdx",
      "args": ["mcp", "serve"]
    }
  }
}
```

### Available tools

The MCP server exposes 20 tools:

**Read-only (12 tools):**

| Tool | Description |
|------|-------------|
| `whoami` | Current user identity |
| `list_time_entries` | Search entries by date range and filters |
| `get_time_entry` | Fetch single entry by ID |
| `get_week_report` | Weekly report with entries and status |
| `get_locked_days` | Locked days in a date range |
| `list_time_types` | All available time types |
| `get_time_types_for_target` | Valid types for a work item |
| `list_time_templates` | All saved templates |
| `get_time_template` | Load template by name |
| `compare_template_to_week` | Compare template vs live week |
| `preview_apply_time_template` | Preview apply with diffHash |

**Mutating (8 tools):**

| Tool | Description |
|------|-------------|
| `create_time_entry` | Create a new entry |
| `update_time_entry` | Update an existing entry |
| `delete_time_entry` | Delete an entry |
| `create_time_template` | Create template from JSON rows |
| `update_time_template` | Update template description |
| `delete_time_template` | Delete a template |
| `derive_time_template` | Derive template from live week |
| `apply_time_template_to_week` | Apply template to a week |

### Safety model

All mutating tools require `confirm: true` in the request. This ensures the
AI agent explicitly confirms each write operation with the user.

Template applies have an additional safety layer: the agent must first call
`preview_apply_time_template` to get a `diffHash`, then pass that hash to
`apply_time_template_to_week`. If the week changed between preview and
apply, the hash won't match and the apply is rejected.

---

## JSON Output

All commands support `--json` for machine-readable output:

```bash
tdx time entry list --json
tdx time entry list --json | jq '.entries[].id'
```

You can also set the `TDX_FORMAT` environment variable:

```bash
export TDX_FORMAT=json
tdx time entry list   # outputs JSON without --json flag
```

### Schema envelopes

JSON output uses stable `tdx.v1.*` schemas. Every response has a top-level
`"schema"` field:

```json
{
  "schema": "tdx.v1.entryList",
  "filter": { "from": "2026-04-05", "to": "2026-04-11" },
  "totalHours": 40.0,
  "totalMinutes": 2400,
  "entries": [...]
}
```

Schema names include: `tdx.v1.entryList`, `tdx.v1.entryAdd`,
`tdx.v1.weekReport`, `tdx.v1.timeTypes`, `tdx.v1.template`,
`tdx.v1.templateList`, `tdx.v1.templateDerive`,
`tdx.v1.templateApplyPreview`, `tdx.v1.timeTypesForTarget`.

---

## Shell Completions

### Bash

```bash
echo 'eval "$(tdx completion bash)"' >> ~/.bashrc
```

### Zsh

```bash
tdx completion zsh > "${fpath[1]}/_tdx"
```

### Fish

```bash
tdx completion fish | source
```

---

## Configuration

tdx stores configuration in `~/.config/tdx/`:

| File | Contents |
|------|----------|
| `config.yaml` | Profiles and default profile |
| `credentials.yaml` | Authentication tokens (per profile) |
| `templates/` | Saved template YAML files |

Override the config directory with `TDX_CONFIG_HOME`:

```bash
export TDX_CONFIG_HOME=/path/to/custom/config
```
