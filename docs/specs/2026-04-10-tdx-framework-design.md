# `tdx` — Framework & v1 Product Design

- **Date:** 2026-04-10
- **Status:** Approved for implementation planning
- **Scope:** Product shape, capability surface, CLI/MCP design, domain model, project layout, phased build plan. This is a *framework-level* spec. Each numbered phase below will get its own follow-up brainstorm and implementation plan at execution time.
- **Target tenant:** `https://ufl.teamdynamix.com/` (initial); profile system keeps the tool tenant-agnostic.

---

## 1. Product brief

**`tdx`** is a Go CLI — with an embedded MCP server — for managing TeamDynamix time entries without touching the web UI. v1 is scoped strictly to time-entry management; the architecture is designed so future TD domains (`tdx ticket …`, `tdx project …`) slot in as sibling command namespaces without refactoring the existing code.

**Who it's for.** Individual TD users at UFL who log time regularly, and AI agents operating on behalf of those users via MCP. An agent needs to be able to do things like "log 2 hours against a ticket today" or "apply my default workweek to next week" as discrete, auditable, previewable actions.

**Problems solved.**

- The TD web time UI is slow, row-at-a-time, and doesn't compose with any scripting or agent workflow.
- There's no native concept of a reusable weekly template — most users mentally re-derive their week every week.
- There's no agent-friendly surface at all. An LLM assistant can't help you enter time today.
- No scriptable interface for bulk corrections, audits, or "show me this week's report and anything locked."

**Why CLI + MCP.** A CLI gives terminal-native speed and composability; MCP exposes the same capabilities to agents with structured inputs, stable schemas, and explicit preview→confirm handshakes for anything destructive. Both layers call the same Go service code, so behavior cannot drift between them.

---

## 2. v1 Capability Inventory

### 2.1 Auth & session

- Interactive browser SSO login (UFL TD SSO).
- Token persisted to a config file (`~/.config/tdx/credentials.yaml`, `0600`). No OS keychain in v1.
- `auth status` — active profile, tenant, identity, token expiry.
- `auth logout` — clear token for a profile.
- **Profiles**: named configurations for `(tenant, base URL, user)` so a sandbox or secondary tenant can be added later. Selected via `--profile` flag or `TDX_PROFILE` env var.

### 2.2 Read-only time operations

- List/search entries with filters (date range, user, ticket/project/task, time type). Default: "this week, me."
- Show a single entry by ID with full detail.
- Show weekly time report (`GET /api/time/report/{date}`).
- Show locked days in a range.
- List all visible time types; look up valid time types for a given work item.
- Recent-N affordance: "what did I log recently?"

### 2.3 Mutating time operations

- Add a single entry against a ticket/project/task with explicit time type.
- Edit an existing entry (within TD's post-creation immutability constraints).
- Delete an entry (confirmation required).
- Batch writes transparently respect the 50-item cap on `POST /api/time`; partial successes are surfaced explicitly.

### 2.4 Templates (local concept)

- Primary creation path: **derive a template from a live TD week**. Hand-crafting is secondary.
- List / show (always as grid) / edit / delete / clone.
- Preview apply (dry-run diff against a target week). Preview is **always on** before any real apply.
- Apply to a specified week with A1 reconciliation semantics (always preview + confirm).
- Import/export as portable files (the YAML file itself is the portable format; `cp` is functionally sufficient).
- At apply time: day-range narrowing (`--days mon-thu`) and per-day overrides (`--override fri=4h`).

### 2.5 Quality-of-life

- `--dry-run` on every mutating command (not only templates).
- `--json` everywhere, with a stable `tdx.v1` schema.
- Human output is the default in a TTY; JSON is automatic when piped.
- TD validation errors are parsed and presented per-row with actionable guidance.
- Exit codes: `0` full success, `2` partial success (some rows written, some failed), `1` total failure.

### 2.6 Explicit non-goals for v1

- Admin/service-account auth flows.
- Entering time on behalf of other users.
- Submitting/approving weekly reports.
- Ticket creation, project browsing, expense entry, or any non-time TD domain.

---

## 3. CLI command surface

The `time` namespace is first-class and everything time-related lives beneath it. Future domains are siblings to `time` and never pollute the root.

### 3.1 Command tree

```
tdx
├── auth
│   ├── login
│   ├── logout
│   ├── status
│   └── profile
│       ├── list
│       ├── add <name>
│       ├── remove <name>
│       └── use <name>
├── time
│   ├── entry
│   │   ├── list
│   │   ├── show <id>
│   │   ├── add
│   │   ├── update <id>
│   │   ├── delete <id>
│   │   └── recent
│   ├── week
│   │   ├── show [date]
│   │   ├── locked
│   │   └── compare
│   ├── template
│   │   ├── list
│   │   ├── show <name>
│   │   ├── derive <name>
│   │   ├── create <name>
│   │   ├── edit <name>
│   │   ├── clone <src> <dst>
│   │   ├── delete <name>
│   │   ├── import <path>
│   │   ├── export <name> [path]
│   │   └── apply <name>
│   └── type
│       ├── list
│       ├── show <id>
│       └── for <target>
├── mcp
│   └── serve
├── config
│   ├── show
│   ├── path
│   └── init
└── version
```

### 3.2 Global flag conventions

Every command accepts:

- `--profile <name>` / `TDX_PROFILE` — select the active profile.
- `--json` — force JSON output.
- `--human` — force human (table/grid) output.
- `--no-color` — disable ANSI styling.
- `--yes` / `-y` — skip interactive confirmation on destructive actions. The preview/diff still prints.
- `--dry-run` — on mutating commands, run the full pipeline and print what *would* happen without calling TD's write endpoints.
- `--verbose` / `-v` — more detail, including API call summaries.
- `--quiet` / `-q` — suppress non-essential output; rely on exit code.

### 3.3 Representative invocations

**Read:**

```
tdx time entry list
tdx time entry list --week 2026-03-22
tdx time entry list --from 2026-03-01 --to 2026-03-31
tdx time entry list --ticket 12345 --app 42
tdx time entry list --type "Development"
tdx time entry recent --limit 10

tdx time week show
tdx time week show 2026-03-22
tdx time week locked --from 2026-03-01 --to 2026-04-01

tdx time type list
tdx time type for ticket 12345 --app 42
```

**Write:**

```
tdx time entry add --ticket 12345 --app 42 \
    --hours 2 --type "Development" --date 2026-04-10 \
    --description "Investigating the ingest bug"

tdx time entry update 98765 --hours 2.5 --description "Updated description"
tdx time entry delete 98765
```

**Templates:**

```
tdx time template derive default-week --from-week 2026-03-15
tdx time template list
tdx time template show default-week
tdx time template edit default-week
tdx time template apply default-week --week 2026-04-12
tdx time template apply default-week --week 2026-04-12 --days mon-thu
tdx time template apply default-week --week 2026-04-12 \
    --override fri=4h --mode add --yes
tdx time template apply default-week --week 2026-04-12 --dry-run --json   # agent-friendly diff
tdx time template export default-week ./default-week.yaml
tdx time template import ./shared-template.yaml

tdx time week compare --template default-week --week 2026-04-12
```

**Auth:**

```
tdx auth login
tdx auth login --profile sandbox
tdx auth status
tdx auth profile list
tdx auth profile use sandbox
tdx auth logout
```

### 3.4 CLI vs agent ergonomics

Every command works identically for humans and agents. An agent appends `--json --yes` and gets machine-stable output. The preview-first flow means an agent typically runs `apply --dry-run --json` first, reasons about the diff, then runs `apply --yes --json` (or, via MCP, calls `apply_time_template_to_week` with `confirm: true` and an `expectedDiffHash`). The diff is the negotiation surface in both worlds.

**Non-interactive safety rule.** When `--json` is set (explicitly or by TTY auto-detection on a pipe) on a mutating command, the command refuses to prompt. It must be invoked with either `--dry-run` (print the diff and exit) or `--yes` (skip the prompt and apply). Running a mutating command with `--json` and neither flag exits non-zero with a clear error. This prevents agents and scripts from silently hanging on a prompt.

---

## 4. MCP tool surface

### 4.1 Delivery model

- **Transport:** stdio. `tdx mcp serve` runs the embedded server. The same binary is both CLI and MCP server.
- **Auth inheritance:** the MCP server reads `~/.config/tdx/` directly. If the user has run `tdx auth login` at any point, subsequent MCP sessions share that session. If the token is missing or expired at MCP startup, mutating tools return a structured "not authenticated — run `tdx auth login`" error. No mid-session interactive login (the host can't drive a browser).
- **Shared logic:** CLI commands and MCP tools are thin wrappers over the same `internal/svc/*` services. They cannot drift.

### 4.2 Read-only tools

| Tool | Purpose | Inputs | Output shape |
|---|---|---|---|
| `whoami` | Identity, profile, tenant, token expiry | — | `{ user, profile, tenant, expiresAt }` |
| `list_time_entries` | Search/list entries | `dateRange`, `ticketRef?`, `projectRef?`, `typeRef?`, `limit?` | `TimeEntry[]` |
| `get_time_entry` | Single entry detail | `id` | `TimeEntry` |
| `get_week_report` | Weekly report for a week | `date` (any day in target week) | `WeekReport` |
| `get_locked_days` | Locked days in a range | `from`, `to` | `LockedDay[]` |
| `list_time_types` | All visible time types | — | `TimeType[]` |
| `get_time_types_for_target` | Valid time types for a specific work item | `target` (ticket/task/project/…) | `TimeType[]` |
| `list_time_templates` | Local templates | — | `TemplateSummary[]` |
| `get_time_template` | Template detail | `name` | `Template` |
| `compare_template_to_week` | Diff a template vs a TD week | `templateName`, `week` | `ReconcileDiff` |
| `preview_apply_time_template` | Dry-run of an apply | `templateName`, `week`, `options` | `ReconcileDiff` |

### 4.3 Mutating tools

**Uniform rule:** every mutating tool requires `confirm: true`. Calling a mutating tool without `confirm: true` returns a structured error directing the agent to either call the read-only preview tool (for template applies) or retry with `confirm: true` (for single-entry operations, where there is no meaningful "diff" beyond the submitted payload). Rejection is explicit, not silent success.

| Tool | Safety model |
|---|---|
| `create_time_entry` | Requires `confirm: true`. Validation errors from TD (locked day, invalid target, limit exceeded) are returned as structured errors on the same call. |
| `update_time_entry` | Requires `confirm: true`. Rejects edits of TD-immutable fields with a clear structured error listing which fields are immutable. |
| `delete_time_entry` | Requires `confirm: true` on every call. No friendlier default. Tool description explicitly labels it destructive. |
| `apply_time_template_to_week` | Requires `confirm: true` **and** a caller-supplied `expectedDiffHash`. On confirm the tool re-runs the diff, compares against the expected hash, and refuses if they don't match. Prevents stale-diff races when an agent previews, then something changes in TD before the confirm. Agents always reach this tool via a prior `preview_apply_time_template` call. |
| `derive_time_template_from_week` | Local-only write. Requires `confirm: true`. |
| `create_time_template` | Local-only. Requires `confirm: true`. |
| `update_time_template` | Local-only. Requires `confirm: true`. |
| `delete_time_template` | Local-only. Requires `confirm: true`. |

---

## 5. Domain model

Types live in `internal/domain/` and carry no behavior beyond simple construction and validation.

### 5.1 Identity and session

- `Profile` — `{ name, tenantBaseURL, userHint }`
- `Session` — `{ profile, token, expiresAt, user }`
- `User` — `{ id, uid, fullName, email }`

### 5.2 TD work items

- `TargetKind` — enum: `ticket | ticketTask | projectTask | projectIssue | workspace | request | timeoff`
- `Target` — `{ kind, appID?, itemID, taskID?, displayName, displayRef }`. Captures everything needed for `POST /api/time` and for UI rendering.
- `TimeType` — `{ id, name, description, billable, limited, componentKind }`
- `TimeTypeLimit` — `{ timeTypeID, userID, fromDate, toDate, maxHours, hoursUsed }`

### 5.3 Time entries

- `TimeEntry` — `{ id, userID, target, timeType, date, minutes, description, billable, createdAt, modifiedAt, reportStatus }`
- `EntryDraft` — mutable builder with nilable fields (to distinguish "unset" from "set to zero")
- `EntryFilter` — `{ dateRange, userID?, target?, timeTypeID?, limit? }`
- `ReportStatus` — enum: `open | submitted | approved`

### 5.4 Weeks

- `WeekRef` — canonical `{ startDate, endDate }`, where `startDate` is Monday. All `--week` flags normalize to this.
- `WeekReport` — `{ weekRef, userID, totalMinutes, status, days: DaySummary[], entries: TimeEntry[] }`
- `DaySummary` — `{ date, minutes, locked }`
- `LockedDay` — `{ date, reason? }`

### 5.5 Templates

- `Template` — `{ name, description, tags, schemaVersion, createdAt, modifiedAt, derivedFrom?, rows: TemplateRow[], defaults? }`
- `TemplateRow` — `{ id, label, target, timeType, description, billable, dayHours, resolverHints }`
- `ResolverHints` — `{ targetDisplayName, targetAppName, timeTypeName, sourceWeek }` captured at derive time; used for drift detection.
- `TemplateDefaults` — optional row-level defaults.

### 5.6 Reconciliation

- `ApplyMode` — `add | replace-matching | replace-mine`. Default `add`.
- `ReconcileAction` — tagged union: `Create | Update | Skip | Conflict`.
- `ReconcileDiff` — `{ week, template, mode, actions: ReconcileAction[], blockers: Blocker[], summary, diffHash }`
- `Blocker` — `{ kind: locked | submitted | approved | forbidden | typeLimitExceeded | targetInvalid, date?, rowID?, detail }`
- `ReconcileResult` — `{ diff, successes, failures, partial: bool }`

### 5.7 Bulk ops & errors

- `BatchResult` — `{ submitted, succeeded, failed, failures: { index, error }[] }`. The TD client auto-splits at 50 items and aggregates.
- `APIError` — `{ httpStatus, code, message, itemIndex? }` parsed from TD failure arrays.
- `ValidationError` — presentation-friendly wrapper grouping per-row failures.

### 5.8 Layering

```
┌─────────────┐         ┌─────────────┐
│   cmd/cli/* │         │  mcp/tools  │
│ (CLI verbs) │         │ (MCP tools) │
└──────┬──────┘         └──────┬──────┘
       │                       │
       └───────────┬───────────┘
                   ▼
          ┌─────────────────┐
          │  internal/svc   │  authsvc, timesvc, tmplsvc, rendersvc
          └────────┬────────┘
                   ▼
          ┌─────────────────┐
          │  internal/tdx   │  thin typed HTTP client
          └────────┬────────┘
                   ▼
            TeamDynamix Web API
```

`domain/` has no imports. `tdx/` depends only on `domain/`. `svc/*` depend on `domain/` and `tdx/`. `cli/` and `mcp/` depend only on `svc/*` and `domain/`.

---

## 6. Template system (detailed)

### 6.1 On-disk layout

```
~/.config/tdx/
├── config.yaml                    # profiles, defaults, global prefs
├── credentials.yaml               # tokens, 0600
└── templates/
    ├── default-week.yaml          # one template per file, filename is the name
    ├── light-week.yaml
    └── training-week.yaml
```

Exporting a template is `cp templates/<name>.yaml <path>`. Importing is the reverse with schema validation. There is no separate export/import format; the on-disk YAML *is* the portable format.

### 6.2 Template file format

```yaml
schemaVersion: 1
name: default-week
description: Typical work week
tags: [default, fulltime]
createdAt: 2026-04-10T14:30:00-04:00
modifiedAt: 2026-04-10T14:30:00-04:00
derivedFrom:
  profile: ufl
  weekStart: 2026-03-15
  derivedAt: 2026-04-10T14:30:00-04:00

rows:
  - id: row-01
    label: "Platform team standup + admin"
    target:
      kind: project
      appId: 31
      itemId: 9821
    timeType:
      id: 412
      name: "General Admin"
    description: "Standing meetings and admin overhead"
    billable: false
    hours:
      mon: 1.0
      tue: 1.0
      wed: 1.0
      thu: 1.0
      fri: 1.0
    resolverHints:
      targetDisplayName: "Platform Services"
      targetAppName: "Projects"
      timeTypeName: "General Admin"

  - id: row-02
    label: "Ingest pipeline work"
    target:
      kind: ticket
      appId: 42
      itemId: 12345
    timeType:
      id: 311
      name: "Development"
    description: "Investigating and fixing the ingest bug"
    billable: true
    hours:
      mon: 3.0
      tue: 3.0
      wed: 3.0
      thu: 3.0
      fri: 3.0
    resolverHints:
      targetDisplayName: "Ingest pipeline drops rows > 10k"
      targetAppName: "IT Help Desk"
      timeTypeName: "Development"
```

**Format rules.**

- `schemaVersion` is mandatory; the loader migrates older versions on read.
- `derivedFrom` is present only for templates created via `derive`; hand-crafted templates omit it.
- Row `id` is locally unique and stable across edits.
- `resolverHints` implements the B2 decision: human-readable metadata for re-validation at apply time.
- `hours` is a day-keyed map (mon..sun). Missing days mean "no row for this day." Zero-hour days are omitted, not written as `0`.
- Decimal hours on disk; the reconciler converts to minutes. Fractions that don't resolve to integer minutes are rejected unless `--round` is passed.
- Hand-editing is a secondary path. The format is readable for debugging, git diffs, and the occasional power edit, but the primary editing paths are `derive`, `clone`, `apply --override`, and the (stretch) TUI grid editor.
- `tdx` writes canonical YAML (stable key order, consistent indentation) so round-trips produce predictable diffs.

### 6.3 Reconciliation algorithm

**Inputs:** `Template`, `WeekRef`, `ApplyMode`, optional `days` filter, optional `overrides`.

**Preview pipeline:**

1. **Fetch state.** `GET /api/time/report/{weekStart}` → `WeekReport`. `GET /api/time/locked?from=…&to=…` → locked days.
2. **Project the template onto the week.** For each `row × active day`, compute a `proposed entry` `(target, timeType, date, minutes, description, billable)`.
3. **Re-validate each row against live TD state.**
   - Is the target still visible/valid? Call the appropriate component endpoint.
   - Is the time type still valid for that target?
   - For limited time types: call the limits endpoint and check remaining capacity.
   - Any failure → `Blocker{kind: targetInvalid | typeLimitExceeded | forbidden, rowID}`.
4. **Detect locked / submitted / approved days.** Any proposed entry on a locked day, or any day in a submitted/approved report, becomes `Blocker{kind: locked | submitted | approved, date}`. Affected rows become `Skip` actions with explicit reasons and are never written.
5. **Classify non-blocked proposed entries against existing entries.** Match key: `(target, timeType, date)`.
6. **Apply mode:**
   - `add`: match → `Skip(reason: alreadyExists)`; no match → `Create`.
   - `replace-matching`: match → `Update(entryID, patch)`; no match → `Create`.
   - `replace-mine`: match **and** ownership-test passes → `Update`; match **and** ownership-test fails → `Skip(reason: notOwnedByTemplate)` reported as `Conflict`; no match → `Create`.
7. **Compute `diffHash`** over a canonicalized form of `(sorted actions, blockers, mode, template version, weekRef)`. This is the `expectedDiffHash` used by MCP race protection.
8. **Return `ReconcileDiff`.** Nothing is written.

**Apply step** (only on confirmation):

1. Re-run the preview pipeline from step 1.
2. Recompute `diffHash`; compare to the expected one. Mismatch → abort with "the week changed since preview."
3. Batch `Create` and `Update` actions into `POST /api/time` calls, respecting the 50-item cap. The client splits transparently.
4. Collect per-action results. Any failures populate `ReconcileResult.failures`. Partial success is explicit in the return and in the exit code.

### 6.4 Ownership tracking for `replace-mine`

Default: **description marker**. On any row written by a `tdx apply`, the reconciler appends a stable marker to the entry description:

```
<original description> [tdx:default-week#row-02]
```

The reconciler parses this back at match time to establish ownership. Trade-offs: robust to machine migration, survives reinstall, visible in TD, avoids a local source of truth that can silently get out of sync. Users can opt out via `ownership: journal` in `config.yaml`, which switches to a local journal file at `~/.config/tdx/state/applies.log`. Default is on because it's the safer robustness story.

This is revisitable in the Phase 4 brainstorm.

### 6.5 Template workflows

- "Derive from this week, apply to next week." → `tdx time template derive default-week --from-week 2026-03-15` → `tdx time template apply default-week --week 2026-03-22`.
- "Apply but shift Friday hours." → `… apply … --override fri=4h`.
- "Apply only Monday–Thursday." → `… apply … --days mon-thu`.
- "Apply but skip anything already logged." → `… apply … --mode add` (default).
- "Redo yesterday's (wrong) apply." → `… apply … --mode replace-mine`.
- "Compare without writing." → `tdx time week compare --template default-week --week 2026-03-22`.
- "Agent workflow." → `… apply … --dry-run --json`, then MCP `apply_time_template_to_week` with `confirm: true` and the diffHash.
- "Locked/submitted safety." → Any locked/submitted day is surfaced as a `Blocker` in every preview. Safety isn't a flag; it's the only mode.

### 6.6 Minimal first slice of the template feature

1. Local YAML storage + `list` / `show` (always grid view) / `delete` / `edit` (opens YAML in `$EDITOR`)
2. `derive <name> --from-week <date>`
3. `apply <name> --week <date>` in `add` mode only, with always-on preview + confirm
4. `compare` as a read-only diff
5. `week show` rendering a TD week with the same grid as templates
6. *Stretch for late v1:* interactive grid editor for templates and weeks (see §7)

---

## 7. Grid views and interactive editor

### 7.1 Grid as the default `show` rendering

`tdx time template show <name>` and `tdx time week show <date>` both render the same Row × Day ASCII grid:

```
default-week — Typical work week (derived from 2026-03-15)

  ROW                                  MON  TUE  WED  THU  FRI  TOTAL
  ────────────────────────────────────────────────────────────────────
  Platform standup + admin (project)    1.0  1.0  1.0  1.0  1.0    5.0
    └ General Admin · Platform Services
  Ingest pipeline (ticket #12345)        3.0  3.0  3.0  3.0  3.0   15.0
    └ Development · IT Help Desk
  ────────────────────────────────────────────────────────────────────
  DAY TOTAL                              4.0  4.0  4.0  4.0  4.0   20.0
```

`--json` is always available; for humans, the grid is the default and requires no flags.

### 7.2 Interactive grid editor (stretch)

`tdx time template edit <name>` (without `--yaml`) opens an interactive TUI grid. Arrow keys navigate Rows × Days. Hours go in cells. Rows can be added, removed, retargeted. Save on exit. The same widget is reused for `tdx time week edit <date>`, letting non-CLI-native users construct and push a week visually without ever opening YAML.

The TUI is a pure view over the template/week data structures produced by services. The reconciliation engine does not depend on it. We can land the non-TUI slice first and add the editor later without engine rework.

---

## 8. Go project layout

```
tdx/
├── cmd/
│   └── tdx/
│       └── main.go                 # wires cobra tree, owns no business logic
├── internal/
│   ├── cli/                        # cobra command definitions
│   │   ├── root.go
│   │   ├── auth/
│   │   ├── time/
│   │   │   ├── entry/
│   │   │   ├── week/
│   │   │   ├── template/
│   │   │   └── type/
│   │   ├── mcp/                    # tdx mcp serve wiring
│   │   └── config/
│   ├── svc/                        # domain services — the heart
│   │   ├── authsvc/
│   │   ├── timesvc/
│   │   ├── tmplsvc/                # template CRUD + reconciliation engine
│   │   └── rendersvc/              # shared table/grid/JSON renderers
│   ├── domain/                     # typed value objects, no behavior
│   │   ├── entry.go
│   │   ├── week.go
│   │   ├── template.go
│   │   ├── target.go
│   │   └── errors.go
│   ├── tdx/                        # TD Web API client
│   │   ├── client.go
│   │   ├── auth.go
│   │   ├── time.go
│   │   └── models.go
│   ├── config/                     # profiles + credentials file handling
│   ├── store/                      # template directory read/write
│   ├── tui/                        # interactive grid editor (stretch)
│   └── mcp/
│       ├── server.go
│       └── tools/                  # one file per tool group; thin shims over svc/
├── docs/
│   └── superpowers/specs/
├── scripts/
├── go.mod
└── README.md
```

**Principles.**

- `svc/` is the only orchestration layer. `cli/` and `mcp/` are thin frontends; `tdx/` is a dumb HTTP client; `domain/` holds value types.
- No circular imports. Dependency direction is one-way: `domain → tdx → svc → (cli|mcp)`.
- Adding a future TD domain requires: a new subpackage under `cli/`, a new service under `svc/`, possibly a new client file under `tdx/`. No existing code moves.

---

## 9. Output strategy

**Two formats, one rule.**

- **Human** (default in TTY): colored tables, Row × Day grids for week/template views, relative dates ("this Mon", "yesterday"), wrapped errors, confirmation prompts.
- **Machine** (`--json` or non-TTY stdout): stable schema versioned as `tdx.v1`, exact ISO-8601 dates, no color, no prompts. Documented and stable within a major version; `tdx --print-json-schema` emits it for agents.

**Format selection:**

```
if --json:              always JSON
elif --human:           always human
elif TDX_FORMAT env:    use that
elif stdout is TTY:     human
else:                   JSON
```

**Auto-detection matches `gh`.** `tdx ... | jq …` works without flags.

**MCP consistency.** Every MCP tool's output uses the same JSON schemas as `--json`. Transfer learning between CLI scraping and MCP calling is free for agents.

---

## 10. Error handling & TD constraints

| Constraint | UX response |
|---|---|
| 50-item batch cap on `POST /api/time` | Client auto-splits; caller sees an aggregated `BatchResult`. |
| Locked days | Detected on every mutating path via `GET /api/time/locked`; surfaced as `Blocker{kind: locked}` in previews. Mutating commands refuse to touch those dates. |
| Submitted/approved reports | Detected via `WeekReport` status; same `Blocker` treatment. |
| Post-creation immutability (target, owner) | `update` surfaces a clear error listing immutable fields and recommends `delete + add`. MCP returns a structured error with the same guidance. |
| Limited time accounts | Reconciler calls the limits endpoint during validation; caps surfaced proactively in previews. |
| Delete permission (self only in v1) | Permission errors translated to readable messages. |
| Partial success in batch writes | `BatchResult.failures` is always shown. Human output groups failures with row-level context. Exit: `0` full, `2` partial, `1` total failure. |
| Rate limiting | Client honors `Retry-After`; persistent limiting surfaces as a clearly labeled error rather than a raw 429. |

**Error presentation principle.** Per-item TD failures are grouped by row and printed with actionable summaries:

```
✗ 2 of 8 entries failed

  Row 5 (Ingest pipeline · Tue 2026-03-17)
    TD rejected: "Time account is not valid for the selected item"
    → The ticket may have been moved to a different app, or the time type
      is no longer valid for this ticket.
      Try: tdx time type for ticket 12345 --app 42

  Row 7 (Platform admin · Fri 2026-03-20)
    TD rejected: "Day is locked"
    → 2026-03-20 is in a locked period; entries cannot be added or edited.
```

---

## 11. Phased build plan

### Phase 0 — Research & framing *(complete)*

This document.

### Phase 1 — Auth & environment/profile foundation

- **Goals:** working `tdx auth login` against UFL SSO; persistent session; profile/config layer; TD client wiring with token threaded through.
- **Deliverables:** `tdx auth login | status | logout`, `tdx auth profile list | add | remove | use`, `internal/config/`, `internal/svc/authsvc/`, HTTP client scaffolding.
- **Risks/questions:** exact UFL SSO callback mechanism — loopback vs paste-token vs device-code polling. This deserves its own focused brainstorm at the start of the phase.
- **Milestone:** end-to-end login on a real UFL laptop; `auth status` shows identity + tenant + expiry; a manual API call succeeds using the stored token.

### Phase 2 — Read-only time operations

- **Goals:** everything non-mutating — entries list/show/search, week report, locked days, time types, component lookups, grid view of a week.
- **Deliverables:** `tdx time entry list | show | recent`, `tdx time week show | locked`, `tdx time type list | show | for`, read paths on `timesvc`, time-read methods on the TD client, table + grid renderers.
- **Risks/questions:** default filter ergonomics; UX for `--ticket` without `--app`.
- **Milestone:** tool is already useful for read-only workflows before any write path exists.

### Phase 3 — Mutating time-entry operations

- **Goals:** create/update/delete entries individually and in batches.
- **Deliverables:** `tdx time entry add | update | delete`, confirmation/preview flow, auto-splitting at 50-item boundary, partial-success reporting, error renderer, `--dry-run` across mutating commands.
- **Risks/questions:** exact update immutability rules; exact TD validation error shapes.
- **Milestone:** writing entries works end-to-end with safe defaults; locked/submitted days reliably blocked.

### Phase 3.5 — Interactive grid editor *(stretch)*

- **Goals:** make the tool usable for non-CLI-native users.
- **Deliverables:** `tdx time week edit`, `tdx time template edit --tui`, `internal/tui/` using bubbletea or similar.
- **Risks/questions:** scope creep; strict time-box recommended.
- **Milestone:** a non-technical user can enter and push a week without ever opening YAML or typing a flag.

### Phase 4 — Templates

- **Goals:** the reusable-week feature end to end.
- **Deliverables:** `tmplsvc` storage over `~/.config/tdx/templates/`, full command surface (`derive | list | show | edit | delete | clone | import | export`), reconciliation engine supporting `add`, `replace-matching`, `replace-mine`, preview-always-on semantics, ownership-marker logic behind a config toggle, `week compare`.
- **Risks/questions:** ownership tracking robustness when users also edit in the TD web UI; drift detection via `resolverHints`.
- **Milestone:** "derive default-week → apply next Monday" works end to end with safe previews, against real TD.

### Phase 5 — MCP surface

- **Goals:** expose the same capabilities as MCP tools with consistent preview→confirm safety.
- **Deliverables:** `tdx mcp serve`, tool definitions per §4, shared schemas with `--json`, `expectedDiffHash` race protection on `apply_time_template_to_week`.
- **Risks/questions:** Go MCP library choice; schema stability guarantees across versions.
- **Milestone:** an agent drives a full "derive → preview → apply" template workflow end to end against a real TD week.

### Phase 6 — Polish, docs, packaging, tests

- **Goals:** ship-readiness.
- **Deliverables:** README with full command reference, shell completions, `goreleaser` config, recorded demo/asciinema walkthrough, unit tests concentrated on `timesvc` and `tmplsvc` (the reconciler in particular), `make lint`, CI workflow.
- **Risks/questions:** distribution channel — Homebrew tap, `go install`, direct binaries.
- **Milestone:** a newcomer installs and uses the tool in under five minutes.

---

## 12. v1 command shortlist

The smallest set that makes `tdx` immediately useful.

| Command | Phase |
|---|---|
| `tdx auth login` | 1 |
| `tdx auth status` | 1 |
| `tdx auth logout` | 1 |
| `tdx time entry list` | 2 |
| `tdx time entry show <id>` | 2 |
| `tdx time week show [date]` | 2 |
| `tdx time week locked` | 2 |
| `tdx time type list` | 2 |
| `tdx time type for <target>` | 2 |
| `tdx time entry add` | 3 |
| `tdx time entry delete <id>` | 3 |
| `tdx time entry update <id>` | 3 |
| `tdx time template derive <name> --from-week <date>` | 4 (slice) |
| `tdx time template list` | 4 (slice) |
| `tdx time template show <name>` | 4 (slice) |
| `tdx time template apply <name> --week <date>` (preview + confirm, `add` mode) | 4 (slice) |

Everything else is post-v1.

---

## 13. Open questions

These are deliberately unresolved. Each will be addressed in the brainstorm for the phase that owns it.

1. **SSO callback mechanism.** Three plausible patterns:
   - *Loopback HTTP listener*: tdx spins up `http://127.0.0.1:<port>/callback`, opens the browser at the TD SSO endpoint with that URL as the redirect, TD returns a token after SSO. Best UX. Requires TD to support arbitrary localhost redirects.
   - *Paste-token fallback*: TD shows a token after login; user pastes into the CLI prompt. Ugly but bulletproof; should exist as a universal fallback regardless of primary mechanism.
   - *Polling endpoint*: the "every 60 seconds" URL fragment from the Auth docs hints at a device-code-style polling pattern. Would be ideal for CLI + MCP — no local server, no paste, survives headless environments.
   - **Action:** dedicated Phase 1 auth brainstorm to verify what UFL's instance actually supports before committing code.

2. **Native TD batch/weekly write model.** Confirmed: there is no timesheet-level write endpoint. `POST /api/time` is the batched add/edit endpoint with a 50-item cap and per-item result arrays. Templates are unambiguously a tdx-side abstraction that compiles to many `POST /api/time` rows. No changes to design.

3. **Limited time account behavior.** Exact shape of the limits response, whether running totals include pending entries, and how over-limit errors come back. Defer to Phase 3 brainstorm.

4. **Ownership marker format.** `[tdx:<template>#<rowID>]` is a reasonable default. Users may prefer no markers; `ownership: journal` in config.yaml switches to a local journal. Default is marker mode. Revisit in Phase 4 brainstorm.

5. **Target ergonomics without `--app`.** Some target kinds (tickets) require an `AppID`; others don't. The CLI flag ergonomics for `--ticket 12345` without an app will need a default-resolution step (use a configured default ticket app, or require `--app` explicitly). Defer the exact UX to Phase 2 brainstorm.

6. **Time granularity.** The API uses minutes. Templates store decimal hours. The engine rejects fractions that don't resolve to integer minutes by default; `--round` opts into nearest-minute rounding with an explicit notice in the preview.

7. **Profile-less first run.** On `tdx auth login` with no profile configured, auto-create a `default` profile pointing at `https://ufl.teamdynamix.com/`. First-run ergonomics should not require knowing the term "profile."

8. **Go MCP library choice.** Pin in Phase 5 brainstorm.

---

## 14. Decision log

Key decisions locked during brainstorming, for quick reference.

- **A1 — Apply-mode default: always preview + confirm.** Every `template apply` runs a dry-run first, prints a diff, requires confirmation. `--yes` skips the prompt but the diff still prints.
- **B2 — Template row identity: fixed refs + resolver hints + re-validation.** Templates store raw `(AppID, ItemID, TimeTypeID)` tuples plus human-readable hints; at apply time tdx re-validates each row and warns on drift.
- **Token storage: config file at `~/.config/tdx/credentials.yaml`, `0600`.** No OS keychain in v1.
- **Tenant: `https://ufl.teamdynamix.com/`** is the default, but profile system keeps the tool generic.
- **MCP race protection.** `apply_time_template_to_week` requires an `expectedDiffHash` on confirm; mismatch aborts.
- **Ownership tracking: description marker by default**, journal mode opt-in via config.
- **Output auto-detection: `gh`-style.** TTY → human; pipe → JSON.
- **Exit codes.** `0` full success, `2` partial success, `1` total failure.
- **Hand-editing templates is secondary.** Format is readable but not optimized for from-scratch authoring.
- **Grid view is the default `show` rendering** for both templates and weeks.
- **Interactive TUI editor is a Phase 3.5 stretch goal**, architecturally supported from day one.
- **Scope model: one framework-level spec now (this document), per-phase brainstorms at execution time.**
