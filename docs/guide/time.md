# tdx time

`tdx time` is the core of tdx: it covers individual time entries, reusable weekly templates, week views and local drafts with a full pull-edit-push lifecycle, time type lookup, and team-wide time-status reports.

## Contents

- [tdx time entry](#tdx-time-entry) — individual time entries
- [tdx time template](#tdx-time-template) — reusable weekly templates
- [tdx time type](#tdx-time-type) — TD time type lookup
- [tdx time week](#tdx-time-week) — week views, drafts, refresh, snapshots
- [tdx time report](#tdx-time-report) — weekly time-status reports

---

## tdx time entry

### tdx time entry list

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

#### Filters

Filter by time type name (case-insensitive substring match):

```bash
tdx time entry list --type development
```

Filter by ticket (requires `--app` for the TD application ID):

```bash
tdx time entry list --ticket 12345 --app 42
```

### tdx time entry show

```bash
tdx time entry show 98765
```

### tdx time entry add

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
- `--ticket <id>` and optionally `--app <id>` (if no profile default app is set, `--app` is required; see `tdx ticket app use`)
- `--project <id>` (optionally with `--plan <id> --task <id>`)
- `--workspace <id> --app <id>`

With a profile default ticket app (set via `tdx ticket app use <id>`), `--app` is no longer needed:

```bash
tdx time entry add --date 2026-04-07 --hours 2 --type "Development" --ticket 12345 -d "ticket work"
```

Preview without creating:

```bash
tdx time entry add --date 2026-04-07 --hours 2 --type Dev --project 54 --dry-run
```

### tdx time entry update

```bash
tdx time entry update 98765 --hours 3 -d "updated description"
```

Only the flags you pass are changed; everything else stays the same.

### tdx time entry delete

```bash
# Preview first
tdx time entry delete 98765 --dry-run

# Apply (--yes is required)
tdx time entry delete 98765 --yes
tdx time entry delete 98765 98766 98767 --yes   # multiple IDs
```

Without `--yes` the command refuses to delete and points you at `--dry-run`,
matching the safety pattern used by `tdx time week push --yes` and friends.

---

## tdx time template

Templates are the core feature of tdx. The workflow is:

1. **Derive** a template from a week with known good data
2. **Edit** hours if needed (optional)
3. **Show** or **compare** to verify it looks right
4. **Apply** it to future weeks

### tdx time template derive

```bash
tdx time template derive my-week --from-week 2026-04-07
```

This fetches all entries from the week containing April 7 and groups them
into template rows. Entries with the same target, time type, and billable
flag are folded into one row with accumulated hours per day. The most common
description across grouped entries is used for the row.

### tdx time template list

```bash
tdx time template list
```

### tdx time template show

```bash
tdx time template show my-week
```

Displays the template as a grid, similar to the week view but showing the
template's hour pattern rather than live data.

### tdx time template edit

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

### tdx time template clone

```bash
tdx time template clone my-week my-week-v2
```

Creates a copy under a new name. Useful for making variations.

### tdx time template delete

```bash
tdx time template delete my-week
```

### tdx time template compare

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

### tdx time template apply

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

#### Apply modes

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

#### Ownership markers

When tdx creates an entry in `replace-mine` mode, it appends a marker to
the description:

```
API implementation [tdx:my-week#row-01]
```

The marker format is `[tdx:<template-name>#<row-id>]`. On future applies,
tdx uses this marker to identify which entries it owns. Entries without the
marker (created manually or by other tools) are never modified.

The marker is stripped from display output but preserved in the stored entry.

#### Day filtering

Restrict an apply to specific days:

```bash
# Range: Monday through Friday
tdx time template apply my-week --week 2026-04-14 --days mon-fri --yes

# Specific days
tdx time template apply my-week --week 2026-04-14 --days mon,wed,fri --yes
```

Day names are three-letter abbreviations (case-insensitive): `sun`, `mon`,
`tue`, `wed`, `thu`, `fri`, `sat`.

#### Hour overrides

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

#### Rounding

If a template has fractional hours that produce non-integer minutes (e.g.
1.333 hours = 79.98 minutes), tdx errors by default. Pass `--round` to
allow rounding to the nearest whole minute:

```bash
tdx time template apply my-week --week 2026-04-14 --round --yes
```

---

## tdx time type

### tdx time type list

```bash
tdx time type list
```

### tdx time type for

```bash
tdx time type for ticket 12345 --app 42
tdx time type for project 54
```

#### How type matching works

When you use `--type` in entry commands, tdx matches by name
(case-insensitive). For example, `--type dev` matches a type named
"Development". If no match is found, the command errors with the available
type names.

---

## tdx time week

The week grid shows all entries for a week in a compact table. Beyond read-only
views, `tdx time week` provides a full local-draft model: pull a live week
into a named draft, edit it offline, validate with diff and preview, then push
back with multi-layer safety guarantees. Templates are *patterns*; drafts are
*instances* — use a draft when you care about a specific calendar week.

Week drafts are first-class, locally-stored, dated week documents that let
you pull a live week from TeamDynamix, edit it offline, validate, diff,
preview, and push back with safety guarantees.

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

The interactive grid editor (same as `tdx time template edit`) lets you
navigate cells with arrow keys or Tab and type new values. Use `--web` to
open the browser-based grid instead of the terminal editor.

The grid edits hours within existing rows. Identity fields (profile,
weekStart, name) and per-cell metadata (SourceEntryID, PerCell) are
preserved across edits — zeroing a pulled cell becomes a delete-on-push;
zeroing a local-only addition drops the cell entirely.

To add or remove rows, use `tdx time week new --from-template` or
`tdx time week set`.

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
# fix Tuesday's hours in the grid; clear Wednesday's bogus entry (becomes a delete-on-push)
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

In the grid, leave Sun/Sat cells at their pulled value (or zero if they
weren't pulled); only edited cells generate actions on push.

### Auto-snapshot history

Use `tdx time week history <date>` to list snapshots:

```
SEQ   OP            TAKEN                 PINNED  NOTE
1     pre-pull      2026-04-27 13:12:14
2     pre-push      2026-04-27 15:02:11
```

Snapshot retention: last 10 unpinned per draft. See [tdx time week snapshot](#tdx-time-week-snapshot),
[tdx time week restore](#tdx-time-week-restore), and [tdx time week prune](#tdx-time-week-prune)
for manual pinning and pruning.

### tdx time week show

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

### tdx time week locked

Some days may be administratively locked (no edits allowed):

```bash
tdx time week locked
```

### tdx time week pull

Pulls a live week from TeamDynamix into a local draft. The draft is identified
by `(profile, weekStart, name)`; `name` defaults to `default`.

```bash
tdx time week pull 2026-04-27

# Pull into a named alternate (leaves the default draft untouched)
tdx time week pull 2026-04-27 --name pristine
```

Key flags: `--name` to create a named alternate; `--force` to overwrite a
dirty draft (auto-snapshots first); `--json` for machine-readable output.

If the draft already exists and is dirty, pull refuses unless you pass
`--force`. A `pre-pull` snapshot is taken automatically before overwriting.

### tdx time week list

Lists all local drafts across all weeks:

```bash
tdx time week list
```

Alternates for the same date are grouped visually under the same week header:

```
2026-04-27
  default   dirty   3 edits
  pristine  clean
```

Flags: `--dirty` to show only drafts with uncommitted edits; `--conflicted`
to show only drafts in the conflicted state; `--date <YYYY-MM-DD>` to filter
by week; `--archived` to include archived drafts (hidden by default).
Use `--no-remote-check` to skip the live-week fingerprint check, and `--json`
for machine-readable output.

### tdx time week status

Prints a one-screen status summary for a draft — sync state, entry count,
pending actions, and the draft's watermark date.

```bash
tdx time week status 2026-04-27
tdx time week status 2026-04-27 --json
```

Pass `--no-remote-check` to skip the remote fingerprint comparison (faster
when offline or when you only care about local state).

### tdx time week edit

Opens the draft in the interactive grid editor:

```bash
tdx time week edit 2026-04-27
tdx time week edit              # edits the current week's default draft
```

Use `--web` to open the editor in your browser instead of the terminal.
Date is optional; omit it to edit the current week's draft.

The grid edits hours within existing rows. Zeroing a pulled cell becomes a
delete-on-push; zeroing a local-only addition drops the cell entirely.

To add or remove rows, use `tdx time week new --from-template` or
`tdx time week set`. While a draft is conflicted (`SyncConflicted`), `edit`
refuses — resolve conflicts first with `tdx time week resolve`.

### tdx time week diff

Shows the diff between the local draft and the remote (live TD week):

```bash
tdx time week diff 2026-04-27
tdx time week diff 2026-04-27 --json
```

Diff is cell-level. Default compares the draft against the live remote week.
Pass `--against remote` to make this explicit (e.g. when piping output where
defaults are unclear).

### tdx time week preview

Previews the push — shows the reconciled diff list, summary, and hash
without writing anything:

```bash
tdx time week preview 2026-04-27
```

Equivalent to running `push` without `--yes`. Use this to confirm actions
before committing.

### tdx time week push

Pushes local draft changes back to TeamDynamix. See [Push safety contract](#push-safety-contract)
for the full guarantee set.

```bash
tdx time week push 2026-04-27 --yes
tdx time week push 2026-04-27 --yes --allow-deletes   # required if draft has cleared cells
```

A `pre-push` snapshot is taken automatically before any writes. Without
`--yes`, behaves as preview. While a draft is conflicted (`SyncConflicted`),
`push` refuses — resolve first.

### tdx time week delete

Deletes a local draft. Auto-snapshots the current state (`pre-delete`) before
removing it, so recovery is possible via `tdx time week restore`.

```bash
tdx time week delete 2026-04-27 --yes
tdx time week delete 2026-04-27 --yes --keep-snapshots   # retain snapshot history
```

`--yes` is required.

### tdx time week set

Non-interactive cell writes using `<row>:<day>=<hours>` syntax:

```bash
tdx time week set 2026-05-04 row-01:mon=8 row-01:fri=4
```

Multiple assignments can be given in one invocation. Use `tdx time week
show --json` or `tdx time template show my-week --json` to find row IDs.
Setting hours to `0` clears the cell.

### tdx time week note

Attaches a free-form note to a draft:

```bash
tdx time week note 2026-04-27 "Worked from home all week"
tdx time week note 2026-04-27 "extra context" --append   # append to existing note
tdx time week note 2026-04-27 --clear                    # remove the note
```

Notes are stored in the draft YAML and are not pushed to TD.

### tdx time week history

Lists snapshots for a draft:

```bash
tdx time week history 2026-04-27
```

```
SEQ   OP            TAKEN                 PINNED  NOTE
1     pre-pull      2026-04-27 13:12:14
2     pre-push      2026-04-27 15:02:11
3     manual        2026-04-27 16:45:00   yes     before risky edit
```

Use `--json` for machine-readable output; `--limit N` to cap the number of snapshots shown.

### tdx time week new

Creates a new draft for a week. Multiple options for seeding:

```bash
# Create a blank draft for the week
tdx time week new 2026-04-27

# Seed from a template
tdx time week new 2026-04-27 --from-template my-week

# Clone from another draft (advances cell dates by --shift)
tdx time week new 2026-05-04 --from-draft 2026-04-27 --shift 7d
```

Cells are dimensionless (no absolute dates embedded), so `--from-draft` with
`--shift 7d` correctly advances every cell date to the target week.

Use `--name` to create a named alternate instead of the default:

```bash
tdx time week new 2026-04-27 --name staging --from-template my-week
```

### tdx time week copy

Clones a draft to a new ref without removing the source:

```bash
tdx time week copy 2026-04-27/default 2026-04-27/backup
```

### tdx time week rename

Renames a draft, preserving its full snapshot history:

```bash
tdx time week rename 2026-04-27/backup 2026-04-27/pristine
```

A `pre-rename` snapshot is taken automatically before the rename.

### tdx time week reset

Discards all local edits and re-pulls a fresh copy from TD. Auto-snapshots
the current state (`pre-reset`) before overwriting, so the edit history is
recoverable.

```bash
tdx time week reset 2026-05-03 --yes
```

`--yes` is required.

### tdx time week refresh

`tdx time week refresh [date[/name]]` re-fetches the live week (defaults to
the current week if no date is given) and merges remote changes into the local
draft using a three-way merge between three views:

- **at-pull-time** — what the live week looked like when the draft was
  created (from the `.pulled.yaml` watermark).
- **current-local** — what the draft contains right now.
- **current-remote** — what the live week contains right now.

Each cell is classified per the merge rules and one of three things happens:

- **Auto-merge** — local-only and remote-only changes both apply.
- **Conflict + strategy** — both sides changed the same cell; behavior
  depends on `--strategy`.
- **Reality match** — local intent (e.g. cleared) and remote state
  (already deleted) agree; the cell drops out silently.

The `rebase` command is a verbatim alias of `refresh` for git-muscle-memory
users — same flags, same behavior.

#### Strategies

```
--strategy abort    (default) refuse to mutate if any cell-level conflict
--strategy ours     on conflict, keep local
--strategy theirs   on conflict, take remote
--strategy surface  on conflict, save both candidates side-by-side and let
                    'tdx time week resolve' pick winners cell-by-cell
```

#### Worked example

You pulled the week, edited Monday from 4.0h to 6.0h. Meanwhile a coworker
edited the same row's Monday on TD to 8.0h:

```
$ tdx time week refresh 2026-05-03
Refresh aborted: 1 cell(s) conflict between local edits and remote changes.

  row-01  Mon  conflict
    local:   updated to 6.0h
    remote:  updated to 8.0h

Choose one:
  --strategy ours        (keep local for all conflicts; refresh succeeds)
  --strategy theirs      (take remote for all conflicts; refresh succeeds)
  tdx time week reset 2026-05-03 --yes  (give up local edits entirely, re-pull fresh)
```

Decide what you want and re-run with the strategy:

```
$ tdx time week refresh 2026-05-03 --strategy ours
Refresh complete (--strategy ours).
  Adopted (remote -> draft):  0 cells
  Preserved (local edits):    0 cells
  Resolved (same on both):    0 cells
  Resolved by --strategy:     1 cells (local won)
```

A snapshot tagged `pre-refresh` is taken before any disk mutation. To roll
back: `tdx time week history 2026-05-03` and
`tdx time week restore 2026-05-03 --snapshot N --yes`.

#### Surface strategy + tdx time week resolve

`--strategy surface` is for the case where neither "all local wins" nor
"all remote wins" is right — e.g. some conflicts you want local for, others
remote. The merge writes both candidates per conflicted cell into the
draft, then `tdx time week resolve` lets you pick winners cell by cell:

```
$ tdx time week refresh 2026-05-03 --strategy surface
Refresh complete (--strategy surface).
  Adopted (remote -> draft):  0 cells
  Preserved (local edits):    0 cells
  Resolved (same on both):    0 cells
  Surfaced (pick winners):    2 cells (tdx time week resolve)

$ tdx time week resolve 2026-05-03
WEEK 2026-05-03  conflicts: 2
ROW       DAY      LOCAL    REMOTE   PULLED
row-01    Monday   6.0      8.0      4.0
row-01    Tuesday  6.0      (deleted) 4.0

Pick:
  tdx time week resolve 2026-05-03 --row row-01 --day Monday --pick remote
  tdx time week resolve 2026-05-03 --all-local
  tdx time week resolve 2026-05-03 --all-remote --yes
```

### tdx time week rebase

`rebase` is a literal alias of `refresh`. Same flags, same behavior. See [tdx time week refresh](#tdx-time-week-refresh) above.

### tdx time week resolve

After running `tdx time week refresh --strategy surface`, use `resolve` to
pick winners cell by cell:

```bash
tdx time week resolve 2026-05-03
```

Apply forms:

```
--all-local            keep local for every conflict
--all-remote           take remote for every conflict
--row ID --day NAME    per-cell pick; require --pick local|remote
--pick local|remote
--yes                  required when --pick remote would drop a cell
                       (remote candidate is "deleted")
--json                 machine-readable output for both status and apply
```

While a draft is conflicted (`SyncConflicted`), `push` and `edit` both
refuse — `push` would commit unresolved divergences, and the grid editor
doesn't display the alternate candidates so editing could silently lose
remote intent. Resolve first, then push or edit normally.

A snapshot tagged `pre-resolve` is taken before each apply, so picks are
revertible via `tdx time week history` / `restore`.

### tdx time week archive

Archiving is a soft hide: the draft YAML stays on disk with full git-diff and
`cat` parity; nothing moves.

```bash
# Hide a draft from default list output
tdx time week archive 2026-04-27/pristine
```

`archive` sets `archived: true` in the draft YAML. Archived drafts are
filtered from `list` output by default; pass `--archived` to include them.
Because archiving is just a YAML flag, there are no rename collisions and
the draft remains fully accessible to `show`, `diff`, `history`, and `cat`.

### tdx time week unarchive

```bash
# Restore a draft to default list visibility
tdx time week unarchive 2026-04-27/pristine
```

### tdx time week snapshot

Takes a manual snapshot of a draft:

```bash
# Take a snapshot (auto-prunable)
tdx time week snapshot 2026-04-27

# Pin the snapshot so it survives prune
tdx time week snapshot 2026-04-27 --keep --note "before risky edit"
```

Pinned snapshots are exempt from all automatic and manual prune operations.

tdx also takes snapshots automatically before destructive operations:

| Trigger | Snapshot label |
|---------|---------------|
| `pull` overwriting a dirty draft | `pre-pull` |
| `push` | `pre-push` |
| `delete` | `pre-delete` |
| `rename` | `pre-rename` |
| `reset` | `pre-reset` |
| `restore` | `pre-restore` |

### tdx time week restore

Restores a draft from a snapshot. tdx auto-snapshots the current draft
(`pre-restore`) before overwriting it.

```bash
tdx time week restore 2026-04-27 --snapshot 2 --yes
```

`--yes` is required. Use `tdx time week history 2026-04-27` to find snapshot
sequence numbers.

### tdx time week prune

Prunes unpinned snapshots for a draft. Pinned snapshots survive both
`--older-than` and bare `--yes` prunes.

```bash
# Drop unpinned snapshots older than 30 days
tdx time week prune 2026-04-27 --older-than 30d --yes

# Drop unpinned snapshots down to the retention cap (10 by default)
tdx time week prune 2026-04-27 --yes
```

---

## tdx time report

### tdx time report status

`tdx time report status` reproduces TeamDynamix's built-in **Work Management →
Analysis → Standard Reports → Time Status Report**: one row per `(user, week)`
with billable / non-billable / total hours and the user's submission status.

#### Columns

- **Name** / **Email** — from `/api/people/{uid}`.
- **Reports To** / **Reports To Email** — manager pulled from the same endpoint.
- **Status** — TD's weekly time-report status: `open`, `submitted`, `rejected`,
  `approved`. A row whose report you can't see (no Analysis app, not the user,
  not their approver) shows `permission-denied` and zero hours; the run
  continues so partial reports are still useful.
- **Bill Hrs** / **Non-Bill Hrs** / **Total Hrs** — pulled from the wire
  response's `MinutesBillable` / `MinutesNonBillable` / `MinutesTotal`.

#### TD endpoints used

- `GET /TDWebApi/api/time/report/{date}/{uid}` — per-user weekly report.
- `GET /TDWebApi/api/people/{uid}` — user profile (name, email, manager).
- `POST /TDWebApi/api/people/search` — enumerate the user population for
  `--manager`, `--account`, `--resource-pool`, and `--all`. Sent with
  `IsEmployee=true` to narrow the response to time-reporting staff
  (TD's "User" type also includes portal Clients). For `--account`,
  also passes `AccountIDs` so TD filters server-side.
- `POST /TDWebApi/api/resourcepools/search` — list resource pools so
  `--resource-pool NAME` can resolve a name to an ID for client-side
  filtering on `ResourcePoolID`. Also powers `tdx people pools list`.
- `POST /TDWebApi/api/accounts/search` — list accounts so `--account NAME`
  can resolve a name to an ID for server-side `AccountIDs` filtering.

#### Permissions

Reading another user's report requires one of:
- The Analysis application role.
- Being the user themselves.
- Being an approver on their timesheet.

When permission is denied for a user, that row's `Status` becomes
`permission-denied` and the rest of the run continues.

#### Selectors (exactly one required)

```
--user UID            one or more user UIDs (repeatable / comma-separated)
--manager UID         direct reports of one or more managers; "me" = authenticated user
--account NAME        users in one or more accounts/departments by name
--resource-pool NAME  users in one or more TD resource pools by name
--all                 every active employee (requires --yes)

Each selector flag is repeatable or comma-separated; values within a
flag are unioned. Selector *types* remain mutually exclusive — you
cannot mix `--manager X --resource-pool Y` in a single run.
```

#### Date range

```
--week DATE      any date in the target week (Sun–Sat in EasternTZ)
--from / --to    multi-week range; normalized to whole weeks
```

#### Filters

```
--include-zero        include user-weeks with zero total minutes (default true)
--incomplete          keep only user-weeks below --threshold (drops permission-denied)
--threshold N         explicit global threshold (hours); requires --incomplete
```

`--incomplete` filters to user-weeks below the threshold. When `--threshold` is omitted,
each user's TD `WorkableHours` (e.g. 40 for FT, 32 for PT) is used as their individual
threshold; if `WorkableHours` is unset in TD, falls back to 40. Pass `--threshold N`
to override with a global threshold for all rows.

`--incomplete` is independent of submission status — a `submitted` 38h holiday week
still shows up if below the threshold. Permission-denied rows drop out under
`--incomplete` (zero hours we couldn't read aren't informative). Subtotals and
OVERALL totals reflect the filtered set, mirroring how `--include-zero=false` already
works.

#### Output formats (mutually exclusive)

```
default          human table
--json           JSON envelope (tdx.v1.timeStatusReport schema)
--csv            CSV on stdout (no subtotal rows; pivot in Excel)
--xlsx PATH      write XLSX file (single sheet, bold frozen header)
```

When `--json` is used with `--incomplete`:
- Each row's `threshold` field shows the threshold value used for that row
  (e.g. the user's `WorkableHours`, or the `--threshold` override if provided).
- The JSON envelope's `filter.thresholdMode` is either `"per-user"` (when `--threshold`
  is omitted, using `WorkableHours`) or `"global"` (when `--threshold N` is specified).

#### Examples

```bash
# Your direct reports' weekly status
tdx time report status --manager me --week 2026-04-12

# Multi-week range for one user, JSON
tdx time report status --user $UID --from 2026-04-12 --to 2026-04-26 --json

# Whole department in CSV
tdx time report status --account "UFIT Operations" --week 2026-04-12 --csv > status.csv

# A specific TD resource pool (use the exact name from the TD UI)
tdx time report status --resource-pool "ICT - DBP - Linux Platform Services LPS" --week 2026-04-12

# Whole org in XLSX (requires --yes)
tdx time report status --all --yes --week 2026-04-12 --xlsx all.xlsx

# Direct reports below their individual WorkableHours (per-user threshold)
tdx time report status --manager me --week 2026-04-19 --incomplete

# Override with a global threshold of 20 hours for everyone
tdx time report status --manager me --week 2026-04-19 --incomplete --threshold 20

# Restore pre-v0.16.5 behavior (global 40 for all users)
tdx time report status --manager me --week 2026-04-19 --incomplete --threshold 40

# Multiple managers — union of direct reports
tdx time report status --manager me --manager other-uid --week 2026-04-19

# Multiple resource pools (or comma-separated)
tdx time report status --resource-pool "Pool A" --resource-pool "Pool B" --week 2026-04-19
tdx time report status --resource-pool "Pool A,Pool B" --week 2026-04-19
```
