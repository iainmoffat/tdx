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
- [Week drafts](#week-drafts)
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

#### Browser editor

For a GUI experience, add `--web` to open the editor in your browser:

```bash
tdx time template edit --web my-week
```

This starts a local server and opens a spreadsheet-like grid. Click cells
to select, type to enter values, shift-click to fill across a row. Click
Save when done — the server exits automatically.

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

## Week drafts

Week drafts are first-class, locally-stored, dated week documents that let
you pull a live week from TeamDynamix, edit it offline, validate, diff,
preview, and push back with safety guarantees.

Templates are *patterns*; drafts are *instances*. Use a template when you
don't care which specific week it lands on. Use a draft when you do.

### Concepts

A **week draft** is identified by `(profile, weekStart, name)` where:
- `weekStart` is the Sunday of the target week (in EasternTZ)
- `name` defaults to `default`; multiple alternate names will be supported in Phase B

A **draft cell** holds hours plus optional metadata. Cells with a `sourceEntryID`
came from a TD pull; cells without one are local additions. Clearing a pulled
cell (`hours = 0`) marks it for deletion on push.

A **snapshot** is an immutable point-in-time copy of a draft, taken automatically
before destructive operations (pull-overwriting-dirty, push, delete). Bounded
retention (last 10 unpinned by default).

### Sync state

Every draft is in one of these states:

| State | Meaning |
|---|---|
| **clean** | Local cells match what was pulled. Push would be a no-op. |
| **dirty** | Local has uncommitted edits. Push has work to do. |
| **stale** | Remote fingerprint changed since pull (independent flag). |
| **conflicted** | Refresh detected divergent remote changes (Phase B). |

### Lifecycle

```
ABSENT --pull--> EXISTS (clean) --edit--> EXISTS (dirty)
                       |                       |
                  --refresh-->            --preview-->
                       |                       |
                       v                  --push --yes-->
                  EXISTS (clean,                 |
                  fresh watermark)               v
                                            EXISTS (clean, pushed)
```

### Editing

`tdx time week edit <date>` opens the draft YAML in `$EDITOR` (vi fallback).
On save, the YAML is validated against the draft schema before being written
back. Identity fields (profile, weekStart, name) are protected — changing
them is rejected with a clear error. A future phase will add a grid-aware
TUI editor for drafts.

For non-interactive cell writes, use `tdx time week set`:

```bash
tdx time week set 2026-05-04 row-01:mon=8 row-01:fri=4
```

### Push safety contract

Three layered guarantees:

1. **`--yes` required** — without it, push behaves as preview (renders the
   diff list + summary + hash, exits without writes).
2. **Hash protection** — push re-runs reconcile and verifies the computed
   `expectedDiffHash` matches the preview's. If remote changed, hash
   mismatches and push refuses, pointing you at `preview` or `pull --force`.
3. **`--allow-deletes` for any deletes** — if your draft contains cleared
   pulled cells (which produce delete actions on push), push refuses
   without the explicit `--allow-deletes` flag. This is an extra speed
   bump beyond `--yes` whenever destruction is involved.

### Worked examples

**Mid-week correction:**

```bash
tdx time week pull 2026-04-27
tdx time week edit 2026-04-27
# fix Tuesday's hours in the YAML; clear Wednesday's bogus entry
tdx time week preview 2026-04-27
tdx time week push 2026-04-27 --yes --allow-deletes
```

**Snapshot a live week before risky edits:**

```bash
tdx time week pull 2026-04-27 --name pristine
tdx time week pull 2026-04-27   # creates the default draft
tdx time week edit 2026-04-27
tdx time week diff 2026-04-27   # vs current remote
```

**Partial-week push** (defer weekend cells):

In your YAML, leave Sun/Sat cells unchanged or zeroed-but-unpulled; only
edited cells generate actions on push.

### Auto-snapshot history

Use `tdx time week history <date>` to list snapshots:

```
SEQ   OP            TAKEN                 PINNED  NOTE
1     pre-pull      2026-04-27 13:12:14
2     pre-push      2026-04-27 15:02:11
```

Snapshot retention: last 10 unpinned per draft. See "Snapshots & history"
below for manual pinning and pruning.

### Multiple drafts per week

A week draft is identified by `(profile, weekStart, name)`. The `name`
defaults to `default`. You can maintain any number of named alternates for
the same week — useful for staging, pristine references, or parallel edits.

**Creating alternates:**

```bash
# Pull into a named alternate (leaves the default draft untouched)
tdx time week pull 2026-04-27 --name pristine

# Create a blank draft for the week
tdx time week new 2026-04-27

# Seed from a template
tdx time week new 2026-04-27 --from-template my-week

# Clone from another draft (src and dst are full refs: date[/name])
tdx time week new 2026-05-04 --from-draft 2026-04-27 --shift 7d

# Clone to an explicit dst ref
tdx time week copy 2026-04-27/default 2026-04-27/backup
```

Cells are dimensionless (no absolute dates embedded), so `--from-draft` with
`--shift 7d` correctly advances every cell date to the target week.

**Renaming:**

```bash
tdx time week rename 2026-04-27/backup 2026-04-27/pristine
```

Renaming preserves the full snapshot history of the source draft.

**Listing alternates:**

```bash
tdx time week list
```

Alternates for the same date are grouped visually under the same week header:

```
2026-04-27
  default   dirty   3 edits
  pristine  clean
```

**Worked examples:**

Snapshot live week before a risky edit:

```bash
tdx time week pull 2026-04-27 --name pristine   # reference copy
tdx time week pull 2026-04-27                   # default draft to edit
tdx time week edit 2026-04-27
tdx time week diff 2026-04-27 --against 2026-04-27/pristine
```

Stage next week from this week:

```bash
tdx time week new 2026-05-04 --from-draft 2026-04-27 --shift 7d
tdx time week edit 2026-05-04      # adjust as needed
tdx time week preview 2026-05-04
tdx time week push 2026-05-04 --yes
```

### Snapshots & history

Snapshots are immutable point-in-time copies of a draft. tdx takes them
automatically before destructive operations:

| Trigger | Snapshot label |
|---------|---------------|
| `pull` overwriting a dirty draft | `pre-pull` |
| `push` | `pre-push` |
| `delete` | `pre-delete` |
| `rename` | `pre-rename` |
| `reset` | `pre-reset` |
| `restore` | `pre-restore` |

**Manual snapshots:**

```bash
# Take a snapshot (auto-prunable)
tdx time week snapshot 2026-04-27

# Pin the snapshot so it survives prune
tdx time week snapshot 2026-04-27 --keep --note "before risky edit"
```

Pinned snapshots are exempt from all automatic and manual prune operations.

**List snapshots:**

```bash
tdx time week history 2026-04-27
```

```
SEQ   OP            TAKEN                 PINNED  NOTE
1     pre-pull      2026-04-27 13:12:14
2     pre-push      2026-04-27 15:02:11
3     manual        2026-04-27 16:45:00   yes     before risky edit
```

**Restore from a snapshot:**

```bash
tdx time week restore 2026-04-27 --snapshot 2 --yes
```

tdx auto-snapshots the current draft (`pre-restore`) before overwriting it.

**Prune snapshots:**

```bash
# Drop unpinned snapshots older than 30 days
tdx time week prune 2026-04-27 --older-than 30d --yes

# Drop unpinned snapshots down to the retention cap (10 by default)
tdx time week prune 2026-04-27 --yes
```

Pinned snapshots survive both `--older-than` and bare `--yes` prunes.

### Archive & unarchive

Archiving is a soft hide: the draft YAML stays on disk with full git-diff and
`cat` parity; nothing moves.

```bash
# Hide a draft from default list output
tdx time week archive 2026-04-27/pristine

# Restore it to default list visibility
tdx time week unarchive 2026-04-27/pristine

# Show all drafts, including archived
tdx time week list --archived
```

`archive` sets `archived: true` in the draft YAML. `list` filters archived
drafts by default; pass `--archived` to include them. Because archiving is
just a YAML flag, there are no rename collisions and the draft remains fully
accessible to `show`, `diff`, `history`, and `cat`.

## Storage layout

Week drafts and templates live under per-profile directories:

```
~/.config/tdx/
├── config.yaml
├── credentials.yaml
└── profiles/
    └── <profile>/
        ├── templates/          # per-profile templates (Phase A migration)
        └── weeks/
            └── <YYYY-MM-DD>/
                ├── default.yaml             # the draft
                ├── default.pulled.yaml      # at-pull-time snapshot
                └── default.snapshots/       # auto-history
                    ├── 0001-pre-pull-...yaml
                    └── 0002-pre-push-...yaml
```

On first run after upgrading from a pre-Phase-A version, tdx detects any
templates in the legacy `~/.config/tdx/templates/` directory and offers to
migrate them into the active profile. Single-profile users see the
migration run silently; multi-profile users get a one-time prompt naming
the target profile.

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

The MCP server exposes 37 tools (17 read-only, 20 mutating). All mutating
tools require `confirm: true`.

**Read-only (11 tools — core):**

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

**Read-only (5 tools — week drafts):**

| Tool | Description |
|------|-------------|
| `list_week_drafts` | List local drafts with sync state |
| `get_week_draft` | Load a single draft + sync state |
| `preview_push_week_draft` | Preview push and capture diffHash |
| `diff_week_draft` | Cell-level diff vs remote |
| `list_week_draft_snapshots` | List snapshots for a draft |

**Mutating (8 tools — core):**

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

**Mutating (12 tools — week drafts):**

| Tool | Description |
|------|-------------|
| `pull_week_draft` | Pull live week into a local draft |
| `update_week_draft` | Apply per-cell edits |
| `delete_week_draft` | Delete a draft (auto-snapshots) |
| `push_week_draft` | Push to TD; requires `expectedDiffHash` |
| `create_week_draft` | Create a draft: blank, template-seeded, or cloned |
| `copy_week_draft` | Clone a draft to a new ref |
| `rename_week_draft` | Rename a draft (preserves snapshot history) |
| `reset_week_draft` | Discard local edits and re-pull |
| `archive_week_draft` | Hide a draft from default list output |
| `unarchive_week_draft` | Show a previously archived draft |
| `snapshot_week_draft` | Take a manual snapshot; optional pin |
| `restore_week_draft_snapshot` | Restore from a snapshot by sequence number |
| `prune_week_draft_snapshots` | Drop unpinned snapshots |

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

| Path | Contents |
|------|----------|
| `config.yaml` | Profiles and default profile |
| `credentials.yaml` | Authentication tokens (per profile) |
| `templates/` | Legacy templates (migrated to per-profile on upgrade) |
| `profiles/<profile>/templates/` | Per-profile templates |
| `profiles/<profile>/weeks/<YYYY-MM-DD>/<name>.yaml` | Local week drafts |
| `profiles/<profile>/weeks/<YYYY-MM-DD>/<name>.snapshots/` | Per-draft auto-snapshots |

Override the config directory with `TDX_CONFIG_HOME`:

```bash
export TDX_CONFIG_HOME=/path/to/custom/config
```
