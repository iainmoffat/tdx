# Docs Reorganization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restructure user-facing docs to mirror the `tdx` command tree: a thin `docs/guide.md` index plus per-top-level-command pages under `docs/guide/`, with the README's nine command tables replaced by a single ASCII command tree.

**Architecture:** Pure docs move. No code changes. Each task migrates one section (or one file) under the heading scheme defined in the spec. Verification per task is via `grep` against expected headings, line counts, and key prose snippets — there are no unit tests for docs.

**Tech Stack:** Markdown. Bash for verification. `git` for branching/commits.

**Spec:** [`docs/specs/2026-05-08-docs-reorganization.md`](../specs/2026-05-08-docs-reorganization.md)

---

## File Structure

After this plan completes:

```
docs/
├── guide.md            # ~250 lines: intro + ASCII tree + Concepts + JSON Output + Storage Layout + Configuration + Shell Completions + reference link list
└── guide/
    ├── auth.md         # ~120 lines: tdx auth (login/logout/status/profile)
    ├── time.md         # ~850 lines: tdx time (entry/template/type/week/report)
    ├── people.md       # ~100 lines: tdx people
    └── mcp.md          # ~200 lines: tdx mcp serve + tool catalog
README.md               # rewritten lines 60-255 (command tables → ASCII tree)
```

## Heading Scheme (apply consistently in every task)

- h1: top-level command group (one per file). Example: `# tdx auth`, `# tdx time`.
- h2: leaf command at the second level OR a sub-group. Example: `## tdx time entry`, `## tdx auth login`.
- h3: leaf command under a sub-group. Example: `### tdx time entry list`, `### tdx auth profile add`.
- h4: sub-topic within a leaf. Example: `#### Apply modes`, `#### Strategies`.

The h1 in each per-command file is its file root. README has its own `# tdx` h1.

## Canonical ASCII Command Tree

This block is byte-identical in `README.md` and `docs/guide.md`. Tasks 5 and 6 reference it; reproduce exactly:

```text
tdx
├── auth
│   ├── login / logout / status
│   └── profile          → list / add / remove / use
├── people
│   ├── search / show
│   ├── accounts         → list
│   └── pools            → list
├── time
│   ├── entry            → list / show / add / update / delete
│   ├── template         → derive / list / show / edit / clone / delete / apply / compare
│   ├── type             → list / for
│   ├── week             → show / locked / pull / list / status / edit / diff / preview / push / delete / set / note / history / new / copy / rename / reset / refresh / rebase / resolve / archive / unarchive / snapshot / restore / prune
│   └── report           → status
├── mcp                  → serve
├── config               → path / init / show
├── completion           → bash / zsh / fish
└── version
```

## Source-of-Truth Line Ranges

These are the line ranges in the **current** `docs/guide.md` (1314 lines total) that each task migrates from. Verified 2026-05-08:

| Section in current `guide.md` | Lines | Destination |
|---|---|---|
| Concepts | 28-43 | `guide.md` (kept, unchanged) |
| Authentication | 44-92 | `guide/auth.md` |
| Profiles | 93-137 | `guide/auth.md` (becomes `## tdx auth profile`) |
| Time Entries | 138-229 | `guide/time.md` (`## tdx time entry`) |
| Week View | 230-266 | `guide/time.md` (merged into `## tdx time week`) |
| Time Types | 267-290 | `guide/time.md` (`## tdx time type`) |
| Templates | 291-513 | `guide/time.md` (`## tdx time template`) |
| Week Drafts | 514-899 | `guide/time.md` (merged into `## tdx time week`) |
| Time Reports | 900-1020 | `guide/time.md` (`## tdx time report`) |
| People | 1021-1089 | `guide/people.md` |
| Storage Layout | 1090-1117 | `guide.md` (kept, unchanged) |
| MCP Server | 1118-1236 | `guide/mcp.md` |
| JSON Output | 1237-1274 | `guide.md` (kept, unchanged) |
| Shell Completions | 1275-1296 | `guide.md` (kept, unchanged) |
| Configuration | 1297-1314 | `guide.md` (kept, unchanged) |

README line ranges (current `README.md`, 317 lines):

| Section in current `README.md` | Lines | Destination |
|---|---|---|
| Commands (lines 60-176, all 9 tables) | 60-176 | DELETE; replaced by ASCII tree + pointer |
| MCP Integration intro + tool tables | 177-255 | Tool tables move to `guide/mcp.md`; intro setup snippet stays |

---

## Task 0: Create feature branch

**Files:** none (git operation only)

- [ ] **Step 1: Confirm clean working tree on `main`**

```bash
git status
```

Expected: `On branch main` and `nothing to commit, working tree clean`. If the working tree is dirty, stop and consult the user.

- [ ] **Step 2: Create and switch to feature branch**

```bash
git checkout -b docs-restructure
```

Expected: `Switched to a new branch 'docs-restructure'`.

---

## Task 1: Build `docs/guide/people.md`

**Files:**
- Create: `docs/guide/people.md`
- Read-only source: `docs/guide.md` lines 1021-1089

This is the smallest per-command file — good warmup.

- [ ] **Step 1: Create the directory and file with heading skeleton**

Create `docs/guide/people.md` with this exact structure:

```markdown
# tdx people

Discovery commands for finding TD users, accounts, and resource pools. Read-only.

## Contents

- [tdx people search](#tdx-people-search)
- [tdx people show](#tdx-people-show)
- [tdx people accounts](#tdx-people-accounts)
- [tdx people pools](#tdx-people-pools)

---

## tdx people search

[migrate prose from `docs/guide.md` lines 1027-1048: "Search by name, email, or ID"]

## tdx people show

[migrate prose from `docs/guide.md` lines 1049-1060: "Show full details for one user"]

## tdx people accounts

### tdx people accounts list

[migrate prose from `docs/guide.md` lines 1072-1081: "List accounts"]

#### How `--account` resolves names

[migrate prose from `docs/guide.md` lines 1082-1089: "How `--account` resolves names" — moved here because it's the natural home; it explains the same lookup used by both the `--account` flag in `tdx time report status` and the `accounts list` command]

## tdx people pools

### tdx people pools list

[migrate prose from `docs/guide.md` lines 1061-1071: "List resource pools"]
```

Read each source line range from current `docs/guide.md`, copy the prose under the previous heading style, and place under the new heading. Drop the old `### Search by name, email, or ID` style heading — the section's heading IS now `## tdx people search`. Keep all code blocks, bullet lists, schema callouts, and inline tables verbatim.

- [ ] **Step 2: Verify headings**

```bash
grep -nE '^#' docs/guide/people.md
```

Expected output (exact):
```
1:# tdx people
5:## Contents
14:## tdx people search
18:## tdx people show
22:## tdx people accounts
24:### tdx people accounts list
28:#### How `--account` resolves names
32:## tdx people pools
34:### tdx people pools list
```

(Line numbers may differ slightly based on prose length; the heading sequence is what matters.)

- [ ] **Step 3: Verify content presence**

```bash
grep -c "tdx.v1.peopleSearchResult" docs/guide/people.md
grep -c "include-clients" docs/guide/people.md
grep -c "TrimSpace" docs/guide/people.md || true
```

Expected: at least 1 hit on the schema name and at least 1 hit on `include-clients`. (TrimSpace check is informational — it appears in the source if present; not a hard requirement.)

- [ ] **Step 4: Commit**

```bash
git add docs/guide/people.md
git commit -m "docs(guide): add guide/people.md (tdx people reference)"
```

---

## Task 2: Build `docs/guide/auth.md`

**Files:**
- Create: `docs/guide/auth.md`
- Read-only source: `docs/guide.md` lines 44-137 (Authentication 44-92, Profiles 93-137)

- [ ] **Step 1: Create the file with heading skeleton**

Create `docs/guide/auth.md`:

```markdown
# tdx auth

Authenticate against a TeamDynamix tenant and manage profiles for multiple tenants.

## Contents

- [tdx auth login](#tdx-auth-login)
- [tdx auth status](#tdx-auth-status)
- [tdx auth logout](#tdx-auth-logout)
- [tdx auth profile](#tdx-auth-profile)

---

## tdx auth login

[merge prose from `docs/guide.md` lines 46-73 covering "First login" (46-54), "SSO login" (55-66), "Scripted login" (67-73). Keep the three flows as `### First login`, `### SSO login`, `### Scripted login` h3 sub-sections under this h2.]

### First login
[lines 46-54]

### SSO login
[lines 55-66]

### Scripted login
[lines 67-73]

## tdx auth status

[migrate prose from `docs/guide.md` lines 75-82, removing the old `### Check your session` heading — the h2 now serves that role]

## tdx auth logout

[migrate prose from `docs/guide.md` lines 83-92, removing the old `### Log out` heading]

## tdx auth profile

Profiles let one tdx install talk to multiple TD tenants.

[migrate the intro paragraph(s) from `docs/guide.md` lines 93-97]

### tdx auth profile add

[migrate prose from `docs/guide.md` lines 98-106]

### tdx auth profile list

[migrate prose from `docs/guide.md` lines 113-120]

### tdx auth profile use

This is the "switch the default" command, plus the `--profile` per-invocation flag.

[migrate prose from `docs/guide.md` lines 107-129 — covers both "Switch the default" (107-112) and "Use a specific profile for one command" (121-129); merge under this single h3 since they're describing the same `tdx auth profile use` command and the `--profile` flag respectively]

### tdx auth profile remove

[migrate prose from `docs/guide.md` lines 130-137]
```

Notes:
- The current guide orders the profile commands as `add → switch (use) → list → use-once → remove`. The natural CLI order is `add → list → use → remove`. Reorder to match the CLI.
- "Use a specific profile for one command" describes the global `--profile` flag, not the `use` command. Keep its prose under `### tdx auth profile use` because that's where readers looking up "how do I use a profile" will land. Add a one-line note at the top of that subsection: "The `tdx auth profile use <name>` command sets the default; the `--profile <name>` global flag overrides for one invocation."

- [ ] **Step 2: Verify headings**

```bash
grep -nE '^#' docs/guide/auth.md
```

Expected sequence:
```
# tdx auth
## Contents
## tdx auth login
### First login
### SSO login
### Scripted login
## tdx auth status
## tdx auth logout
## tdx auth profile
### tdx auth profile add
### tdx auth profile list
### tdx auth profile use
### tdx auth profile remove
```

- [ ] **Step 3: Verify content presence**

```bash
grep -c "loginsso\|--sso" docs/guide/auth.md
grep -c "TDX_TOKEN\|--token" docs/guide/auth.md
grep -c "credentials.yaml" docs/guide/auth.md
```

Expected: each ≥ 1.

- [ ] **Step 4: Commit**

```bash
git add docs/guide/auth.md
git commit -m "docs(guide): add guide/auth.md (tdx auth reference)"
```

---

## Task 3: Build `docs/guide/mcp.md`

**Files:**
- Create: `docs/guide/mcp.md`
- Read-only sources:
  - `docs/guide.md` lines 1118-1236 (MCP Server section)
  - `README.md` lines 196-254 (per-phase tool tables — these MOVE here, not duplicate)

- [ ] **Step 1: Create the file**

```markdown
# tdx mcp

MCP server for AI agent integration. Exposes tdx as 44 tools (22 read-only, 22 mutating).

## Contents

- [tdx mcp serve](#tdx-mcp-serve)
- [Available tools](#available-tools)
- [Safety model](#safety-model)

---

## tdx mcp serve

[migrate prose from `docs/guide.md` lines 1118-1161 — covers "Start the server" (1123-1131) and "Configure in your AI tool" (1132-1161). Drop the `## MCP Server` h2 and merge the two h3s as h3 children of this h2.]

### Start the server
[lines 1123-1131]

### Configure in your AI tool
[lines 1132-1161]

## Available tools

[migrate prose from `docs/guide.md` lines 1162-1224, AND import the per-phase tool tables from `README.md` lines 196-254. The README's tables are more detailed; prefer them. Result: one section listing all 44 tools, organized by phase.]

[Specifically, copy from README:
- Week drafts (Phase A — read-only, 4 tools) — lines 196-204
- Week drafts (Phase A — mutating, 4 tools) — lines 205-213
- Week drafts (Phase B.1 — read-only, 1 tool) — lines 214-219
- Week drafts (Phase B.1 — mutating, 8 tools) — lines 220-232
- Week drafts (Phase B.2a — mutating) — lines 234-240
- Time Reports (Phase C — read-only, 1 tool) — lines 241-246
- People (read-only, 4 tools) — lines 247-254]

## Safety model

[migrate prose from `docs/guide.md` lines 1225-1236]
```

- [ ] **Step 2: Verify headings**

```bash
grep -nE '^#' docs/guide/mcp.md
```

Expected:
```
# tdx mcp
## Contents
## tdx mcp serve
### Start the server
### Configure in your AI tool
## Available tools
## Safety model
```

- [ ] **Step 3: Verify the 7 phase-tool subsections are present**

```bash
grep -c "Phase A — read-only\|Phase A — mutating\|Phase B.1 — read-only\|Phase B.1 — mutating\|Phase B.2a\|Phase C\|People (read-only" docs/guide/mcp.md
```

Expected: ≥ 6 (some phases may be combined; minimum 6 distinct phase headings/labels).

- [ ] **Step 4: Verify a representative tool name from each phase is present**

```bash
for tool in list_week_drafts pull_week_draft list_week_draft_snapshots create_week_draft refresh_week_draft get_time_status_report search_people; do
  grep -q "$tool" docs/guide/mcp.md && echo "OK: $tool" || echo "MISSING: $tool"
done
```

Expected: all `OK`.

- [ ] **Step 5: Commit**

```bash
git add docs/guide/mcp.md
git commit -m "docs(guide): add guide/mcp.md (tdx mcp reference + tool catalog)"
```

---

## Task 4: Build `docs/guide/time.md` (the big one)

**Files:**
- Create: `docs/guide/time.md`
- Read-only source: `docs/guide.md` lines 138-1020

This is the largest task: ~850 lines after migration, covering five sub-groups (`entry`, `template`, `type`, `week`, `report`). Done in one task so the commit is atomic and the heading scheme is internally consistent.

- [ ] **Step 1: Create the file with full skeleton**

```markdown
# tdx time

Read and manage TeamDynamix time entries, weekly drafts, templates, and reports.

## Contents

- [tdx time entry](#tdx-time-entry) — individual time entries
- [tdx time template](#tdx-time-template) — reusable weekly templates
- [tdx time type](#tdx-time-type) — TD time type lookup
- [tdx time week](#tdx-time-week) — week views, drafts, refresh, snapshots
- [tdx time report](#tdx-time-report) — weekly time-status reports

---

## tdx time entry

### tdx time entry list
[migrate `docs/guide.md` lines 140-161 (List entries)]

### tdx time entry list (filters)
[migrate `docs/guide.md` lines 162-175 (Filter entries) — keep as a fold under list. Use `#### Filters` instead of a new h3 if it fits better; engineer's call.]

Actually, keep it simpler: merge "List entries" + "Filter entries" under a single `### tdx time entry list` h3 with `#### Filters` as an h4 sub-topic.

### tdx time entry show
[migrate lines 176-181]

### tdx time entry add
[migrate lines 182-205]

### tdx time entry update
[migrate lines 206-213]

### tdx time entry delete
[migrate lines 214-229]

## tdx time template

[migrate intro from lines 291-299]

### tdx time template derive
[migrate lines 300-310]

### tdx time template list
[migrate lines 311-316]

### tdx time template show
[migrate lines 317-325]

### tdx time template edit
[migrate lines 326-364, including the `#### Browser editor` h4 (lines 353-364)]

### tdx time template clone
[migrate lines 365-372]

### tdx time template delete
[migrate lines 373-378]

### tdx time template compare
[migrate lines 379-395]

### tdx time template apply
[migrate lines 396-421 (intro + basic apply)]

#### Apply modes
[migrate lines 422-453]

#### Ownership markers
[migrate lines 454-468]

#### Day filtering
[migrate lines 469-483]

#### Hour overrides
[migrate lines 484-501]

#### Rounding
[migrate lines 502-513]

## tdx time type

### tdx time type list
[migrate lines 269-274]

### tdx time type for
[migrate lines 275-281]

#### How type matching works
[migrate lines 282-290]

## tdx time week

[migrate intro from lines 230-256 (Week View intro) + 514-522 (Week Drafts intro). Combine into a single overview that covers: weekly grid view, draft concept, sync state, lifecycle.]

#### Concepts
[migrate lines 523-536 (Week Drafts → Concepts)]

#### Sync state
[migrate lines 537-547]

#### Lifecycle
[migrate lines 548-560]

#### Editing
[migrate lines 561-581]

#### Push safety contract
[migrate lines 582-595]

#### Worked examples
[migrate lines 596-621]

#### Auto-snapshot history
[migrate lines 622-634]

#### Multiple drafts per week
[migrate lines 635-704]

### tdx time week show
[migrate lines 232-256 (Week View show prose) — the existing `## Week View` body covers this command]

### tdx time week locked
[migrate lines 257-266 (Locked days)]

### tdx time week pull
[the pull command is described inline in lines 514-704 (Week Drafts) — extract the parts specific to `pull`]

### tdx time week list
[extract from lines 514-704 — the parts specific to `list`]

### tdx time week status
[extract — `status`-specific prose]

### tdx time week edit
[extract — `edit`-specific prose]

### tdx time week diff
[extract — `diff`-specific prose]

### tdx time week preview
[extract — `preview`-specific prose]

### tdx time week push
[extract — `push`-specific prose, including the safety contract reference]

### tdx time week delete
[extract]

### tdx time week set
[extract]

### tdx time week note
[extract]

### tdx time week history
[migrate lines 705-763 (Snapshots & history) intro + history-specific parts]

### tdx time week new
[extract from "Multiple drafts per week" — the `new` command]

### tdx time week copy
[extract — the `copy` command]

### tdx time week rename
[extract — the `rename` command]

### tdx time week reset
[extract — the `reset` command, includes the auto-snapshot behavior]

### tdx time week refresh
[migrate lines 785-808]

#### Strategies
[migrate lines 809-818]

#### Worked example
[migrate lines 819-852]

#### Surface strategy + tdx time week resolve
[migrate lines 853-899]

### tdx time week rebase
Alias of `tdx time week refresh`. See above.

### tdx time week resolve
[the `resolve` command's own prose — it's described inline in lines 853-899; lift out a self-contained section here]

### tdx time week archive
[migrate lines 764-778 (Archive & unarchive — archive part)]

### tdx time week unarchive
[migrate lines 764-784 (unarchive part)]

### tdx time week snapshot
[migrate from "Snapshots & history" — the `snapshot` command]

### tdx time week restore
[migrate from "Snapshots & history" — the `restore` command]

### tdx time week prune
[migrate from "Snapshots & history" — the `prune` command]

## tdx time report

### tdx time report status
[migrate lines 902-907]

#### Columns
[migrate lines 908-918]

#### TD endpoints used
[migrate lines 919-933]

#### Permissions
[migrate lines 934-943]

#### Selectors (exactly one required)
[migrate lines 944-957]

#### Date range
[migrate lines 958-964]

#### Filters
[migrate lines 965-979]

#### Output formats (mutually exclusive)
[migrate lines 980-988]

#### Examples
[migrate lines 989-1020]
```

**Migration notes for the engineer:**

1. The `## tdx time week` section is the trickiest. The current guide has command details scattered through "Week View" (lines 230-266) and "Week Drafts" (lines 514-899). For each leaf command (`pull`, `push`, `refresh`, etc.), find the prose in the source that describes that command and lift it under the new `### tdx time week <cmd>` heading. The command-detail prose tends to live in the per-feature `###` sections (e.g. "Editing", "Multiple drafts per week", "Snapshots & history") — read each `###` and split its contents by command.

2. The conceptual material (Concepts, Sync state, Lifecycle, Editing, Push safety contract, Worked examples, Auto-snapshot history) stays as `####` h4 sub-topics under `## tdx time week` BEFORE the leaf-command h3s. This puts orientation material at the top of the `tdx time week` section.

3. If a leaf command has no dedicated prose in the current guide (some are only described in tables in README), write a one-line description matching the README, no more. Do not invent new content.

4. The `### tdx time week rebase` section is intentionally a one-line alias note — `rebase` is a literal alias of `refresh` in the code (`internal/cli/time/week/rebase.go`).

5. The "Multiple drafts per week" section (lines 635-704) covers `new`, `copy`, `pull --name`, and clone semantics. Split by command. Where the source has a single example block covering multiple commands, replicate the relevant lines under each command's section.

- [ ] **Step 2: Verify file size and h2 sequence**

```bash
wc -l docs/guide/time.md
grep -nE '^## ' docs/guide/time.md
```

Expected:
- File length: 600-1000 lines (target 850).
- h2 sequence (exact, in this order):
```
## Contents
## tdx time entry
## tdx time template
## tdx time type
## tdx time week
## tdx time report
```

- [ ] **Step 3: Verify all leaf commands have a heading**

```bash
for cmd in entry-list entry-show entry-add entry-update entry-delete \
          template-derive template-list template-show template-edit template-clone template-delete template-apply template-compare \
          type-list type-for \
          week-show week-locked week-pull week-list week-status week-edit week-diff week-preview week-push week-delete week-set week-note week-history week-new week-copy week-rename week-reset week-refresh week-rebase week-resolve week-archive week-unarchive week-snapshot week-restore week-prune \
          report-status; do
  human=$(echo "$cmd" | sed 's/-/ /')
  grep -q "tdx time $human" docs/guide/time.md && echo "OK:  tdx time $human" || echo "MISSING: tdx time $human"
done
```

Expected: every line says `OK`. If any are MISSING, find the relevant prose in the source and add a section for that command (or write a one-line description if the source has none).

- [ ] **Step 4: Verify key h4 sub-topics are present**

```bash
for h4 in "Apply modes" "Ownership markers" "Day filtering" "Hour overrides" "Rounding" \
         "How type matching works" "Sync state" "Lifecycle" "Push safety contract" \
         "Strategies" "Surface strategy + tdx time week resolve" \
         "Selectors (exactly one required)" "Output formats (mutually exclusive)"; do
  grep -q "^#### $h4\b\|^### $h4\b" docs/guide/time.md && echo "OK: $h4" || echo "MISSING: $h4"
done
```

Expected: all `OK`.

- [ ] **Step 5: Commit**

```bash
git add docs/guide/time.md
git commit -m "docs(guide): add guide/time.md (tdx time reference)"
```

---

## Task 5: Rewrite `docs/guide.md` as the index

**Files:**
- Modify (rewrite): `docs/guide.md`

After this task, `docs/guide.md` is the entry point — intro, ASCII command tree, link list to per-command pages, then the kept cross-cutting sections.

- [ ] **Step 1: Read kept content for splice-back**

```bash
grep -nE '^## ' docs/guide.md
```

Confirm the current line ranges of the four kept sections are still:
- Concepts: starts at line 28
- Storage Layout: starts at line 1090
- JSON Output: starts at line 1237
- Shell Completions: starts at line 1275
- Configuration: starts at line 1297

If they've drifted (e.g. someone edited the file), use the new line numbers in the splice.

- [ ] **Step 2: Rewrite the file**

Overwrite `docs/guide.md` with this exact structure:

```markdown
# tdx User Guide

This guide covers all tdx features in depth. For a quick reference of
commands and flags, see the [README](../README.md).

---

## Table of Contents

- [Command tree](#command-tree)
- [Reference](#reference)
- [Concepts](#concepts)
- [Storage Layout](#storage-layout)
- [JSON Output](#json-output)
- [Shell Completions](#shell-completions)
- [Configuration](#configuration)

---

## Command tree

```text
tdx
├── auth
│   ├── login / logout / status
│   └── profile          → list / add / remove / use
├── people
│   ├── search / show
│   ├── accounts         → list
│   └── pools            → list
├── time
│   ├── entry            → list / show / add / update / delete
│   ├── template         → derive / list / show / edit / clone / delete / apply / compare
│   ├── type             → list / for
│   ├── week             → show / locked / pull / list / status / edit / diff / preview / push / delete / set / note / history / new / copy / rename / reset / refresh / rebase / resolve / archive / unarchive / snapshot / restore / prune
│   └── report           → status
├── mcp                  → serve
├── config               → path / init / show
├── completion           → bash / zsh / fish
└── version
```

## Reference

- [tdx auth](guide/auth.md) — authenticate and manage profiles
- [tdx time](guide/time.md) — time entries, week drafts, templates, reports
- [tdx people](guide/people.md) — find users, accounts, resource pools
- [tdx mcp](guide/mcp.md) — MCP server for AI agents

---

## Concepts

[verbatim copy of current `docs/guide.md` lines 28-43]

---

## Storage Layout

[verbatim copy of current `docs/guide.md` lines 1090-1117]

---

## JSON Output

[verbatim copy of current `docs/guide.md` lines 1237-1274]

---

## Shell Completions

[verbatim copy of current `docs/guide.md` lines 1275-1296]

---

## Configuration

[verbatim copy of current `docs/guide.md` lines 1297-1314]
```

**Important:** the four kept sections must be byte-identical to the current source. Use `Read` on the source line ranges and copy without alteration.

- [ ] **Step 3: Verify structure**

```bash
grep -nE '^## ' docs/guide.md
```

Expected exactly:
```
## Table of Contents
## Command tree
## Reference
## Concepts
## Storage Layout
## JSON Output
## Shell Completions
## Configuration
```

- [ ] **Step 4: Verify the four per-command page links resolve**

```bash
for f in guide/auth.md guide/time.md guide/people.md guide/mcp.md; do
  test -f "docs/$f" && echo "OK: docs/$f" || echo "MISSING: docs/$f"
done
```

Expected: all `OK`. (If any missing, the corresponding prior task didn't complete — stop and fix.)

- [ ] **Step 5: Verify no removed sections leaked back in**

```bash
for h in "## Authentication" "## Profiles" "## Time Entries" "## Week View" \
         "## Time Types" "## Templates" "## Week Drafts" "## Time Reports" \
         "## People" "## MCP Server"; do
  grep -q "^$h\$" docs/guide.md && echo "LEAK: $h" || echo "ok (gone): $h"
done
```

Expected: every line says `ok (gone)`. Any `LEAK` means the splice missed a deletion.

- [ ] **Step 6: Verify file size**

```bash
wc -l docs/guide.md
```

Expected: 200-300 lines. If significantly larger, prose from removed sections leaked through; review and fix.

- [ ] **Step 7: Commit**

```bash
git add docs/guide.md
git commit -m "docs(guide): rewrite guide.md as index (intro + tree + links + cross-cutting)"
```

---

## Task 6: Rewrite `README.md`

**Files:**
- Modify: `README.md`

Replace the nine command tables (lines 60-176) with the canonical ASCII tree + pointer. Replace the per-phase MCP tool tables (lines 196-254) with a teaser sentence pointing to `docs/guide/mcp.md`. Keep everything else.

- [ ] **Step 1: Replace the Commands section (lines 60-176)**

Use `Edit` to replace lines 60-176 of `README.md`. The replacement is:

```markdown
## Commands

```text
tdx
├── auth
│   ├── login / logout / status
│   └── profile          → list / add / remove / use
├── people
│   ├── search / show
│   ├── accounts         → list
│   └── pools            → list
├── time
│   ├── entry            → list / show / add / update / delete
│   ├── template         → derive / list / show / edit / clone / delete / apply / compare
│   ├── type             → list / for
│   ├── week             → show / locked / pull / list / status / edit / diff / preview / push / delete / set / note / history / new / copy / rename / reset / refresh / rebase / resolve / archive / unarchive / snapshot / restore / prune
│   └── report           → status
├── mcp                  → serve
├── config               → path / init / show
├── completion           → bash / zsh / fish
└── version
```

Full reference: [User Guide](docs/guide.md). The guide is split per top-level command:

- [tdx auth](docs/guide/auth.md)
- [tdx time](docs/guide/time.md)
- [tdx people](docs/guide/people.md)
- [tdx mcp](docs/guide/mcp.md)
```

The `Edit` call's `old_string` must match the current README content from `## Commands` (line 60) through the end of the `### Other` table block (line 176). Use a `Read` to grab those lines first if needed.

- [ ] **Step 2: Replace the per-phase MCP tool tables (current lines ~196-254 — find them after Step 1)**

After Step 1, line numbers in README will have shifted. Re-grep:

```bash
grep -nE '^\*\*Week drafts \(Phase|^\*\*Time Reports \(Phase|^\*\*People \(read-only' README.md
```

The block starts at the first `**Week drafts (Phase A — read-only` line and ends at the closing `|` row of the People table. Replace the entire block with this single sentence:

```markdown
The MCP server exposes 44 tools (22 read-only, 22 mutating). All mutating
tools require `confirm: true`. Template applies require an `expectedDiffHash`
from a prior preview call for race protection. See [tdx mcp](docs/guide/mcp.md)
for the full tool catalog and safety model.
```

Note: the existing two-paragraph intro that currently lives at README lines 192-194 ("The MCP server exposes 44 tools...") becomes the *only* MCP descriptive content in the README. The replacement above is a superset of that intro plus the pointer; replace from line 192 (the start of "The MCP server exposes") through the end of the People table.

- [ ] **Step 3: Verify README structure**

```bash
grep -nE '^## |^### ' README.md
```

Expected h2 sequence:
```
## Install
## Quick Start
## Commands
## MCP Integration
## JSON Output
## Shell Completions
## Configuration
## Development
## License
```

Expected: NO `### Auth`, `### Time Entries`, `### Time Week`, `### Time Week Drafts`, `### Time Types`, `### Templates`, `### Time Reports`, `### People`, `### MCP`, `### Other` h3 entries (those were the deleted command tables).

- [ ] **Step 4: Verify ASCII tree byte-identical with `docs/guide.md`**

```bash
diff <(awk '/^```text$/,/^```$/' docs/guide.md) <(awk '/^```text$/,/^```$/' README.md)
```

Expected: empty diff (the tree blocks match).

- [ ] **Step 5: Verify file size**

```bash
wc -l README.md
```

Expected: 150-200 lines (down from 317).

- [ ] **Step 6: Commit**

```bash
git add README.md
git commit -m "docs(readme): replace command tables with ASCII tree + guide links"
```

---

## Task 7: Verify acceptance criteria and open PR

**Files:** none modified — verification only.

Walk through each acceptance criterion from the spec.

- [ ] **Step 1: Acceptance criterion 1 — `docs/guide.md` is the index**

```bash
grep -nE '^## ' docs/guide.md
wc -l docs/guide.md
```

Expected sections (in order): Table of Contents, Command tree, Reference, Concepts, Storage Layout, JSON Output, Shell Completions, Configuration. File length 200-300 lines.

- [ ] **Step 2: Acceptance criterion 2 — per-command files exist**

```bash
ls -la docs/guide/auth.md docs/guide/time.md docs/guide/people.md docs/guide/mcp.md
```

Expected: all four files present, each non-empty.

- [ ] **Step 3: Acceptance criterion 3 — heading scheme is consistent**

```bash
for f in docs/guide/auth.md docs/guide/time.md docs/guide/people.md docs/guide/mcp.md; do
  echo "=== $f ==="
  head -1 "$f"
done
```

Expected: each file's first line is `# tdx <command>` (where `<command>` is auth, time, people, mcp respectively). No file starts with anything else.

- [ ] **Step 4: Acceptance criterion 4 — README has no command tables**

```bash
grep -E '^### (Auth|Time Entries|Time Week|Time Week Drafts|Time Types|Templates|Time Reports|People|MCP|Other)' README.md
```

Expected: empty output (no matches).

- [ ] **Step 5: Acceptance criterion 5 — ASCII trees byte-identical**

```bash
diff <(awk '/^```text$/,/^```$/' docs/guide.md) <(awk '/^```text$/,/^```$/' README.md)
```

Expected: empty output.

- [ ] **Step 6: Acceptance criterion 6 — no broken intra-doc anchor links**

```bash
grep -rn "guide.md#" docs/ README.md 2>/dev/null
```

Expected: empty output. (If anything appears, verify the anchor exists in the new `docs/guide.md`. Update the link to point to the new per-command file's anchor, e.g. `guide/auth.md#tdx-auth-login`, if the section moved.)

- [ ] **Step 7: Acceptance criterion 7 — only docs files changed**

```bash
git diff --stat main..HEAD
```

Expected: file list is exactly:
```
README.md
docs/guide.md
docs/guide/auth.md          (new)
docs/guide/mcp.md           (new)
docs/guide/people.md        (new)
docs/guide/time.md          (new)
docs/specs/2026-05-08-docs-reorganization.md  (already committed pre-branch)
docs/plans/2026-05-08-docs-reorganization.md  (this plan)
```

No `internal/`, `cmd/`, or any code files. If anything else appears, stop and investigate.

- [ ] **Step 8: Render check (manual)**

Open each new/modified file in a markdown viewer (or `gh pr view --web` after Step 9) and visually confirm:
- ASCII trees render in fenced code blocks (not as collapsed text).
- Tables render as tables.
- The TOC links work (click each link in `docs/guide.md` and each per-command file).
- No raw `[migrate prose from ...]` placeholders remain in any file.

```bash
grep -rn '\[migrate' docs/guide.md docs/guide/
```

Expected: empty output. (Any match means migration prose wasn't pasted in.)

- [ ] **Step 9: Push and open PR**

```bash
git push -u origin docs-restructure
gh pr create --title "docs: restructure guide by top-level command (+ ASCII tree)" --body "$(cat <<'EOF'
## Summary

- Split `docs/guide.md` into a thin index plus per-top-level-command pages under `docs/guide/` (auth, time, people, mcp).
- Replace README's nine command tables with a single 2-level ASCII command tree.
- Move per-phase MCP tool tables from README into `docs/guide/mcp.md`.
- Heading scheme: command-as-heading (`## tdx auth login`, `### tdx time week refresh`) for deterministic anchors.

Spec: `docs/specs/2026-05-08-docs-reorganization.md`
Plan: `docs/plans/2026-05-08-docs-reorganization.md`

## Test plan

- [ ] All four per-command files exist with correct h1.
- [ ] ASCII tree byte-identical between README and guide.md.
- [ ] No broken intra-doc anchor links (`grep -rn "guide.md#" docs/ README.md` empty).
- [ ] No `[migrate ...]` placeholders left in any file.
- [ ] Git diff covers only docs files.
- [ ] Visual render check on GitHub PR view.
EOF
)"
```

Expected: PR URL printed. Wait for CI (none expected to fail — no code changed). Confirm with user before merging.

---

## Self-Review (run by plan author after writing)

**1. Spec coverage:**
- Decision 1 (split per top-level command) → Tasks 1-4
- Decision 2 (single file per top-level command) → Tasks 1-4 (no further split)
- Decision 3 (ASCII tree, two levels deep) → Task 5 step 2 + Task 6 step 1 (canonical block)
- Decision 4 (README keeps tree, drops tables) → Task 6
- Decision 5 (command-as-heading) → every per-command file's heading skeleton uses this
- Decision 6 (cross-cutting in guide.md) → Task 5 keeps Concepts, JSON Output, Storage Layout, Configuration, Shell Completions
- Acceptance criteria 1-7 → Task 7 walks through each

**2. Placeholder scan:**
The plan deliberately uses `[migrate prose from ...]` as a placeholder INSIDE the file skeletons. These are not plan placeholders — they are explicit instructions to the implementer ("paste content from these source lines here"). Task 7 step 8 verifies they don't survive into the final files. This is fine; flagged for awareness.

No "TBD", "TODO", "fill in details" elsewhere.

**3. Type consistency:**
- File names consistent throughout: `docs/guide/auth.md`, `docs/guide/time.md`, `docs/guide/people.md`, `docs/guide/mcp.md`.
- Heading scheme consistent across tasks (h1 = `# tdx <cmd>`, h2 = leaf or subgroup, h3 = leaf-under-subgroup, h4 = sub-topic).
- ASCII tree block reproduced identically in Task 5 and Task 6.
- Branch name (`docs-restructure`) consistent in Task 0 and Task 7.

All consistent.
