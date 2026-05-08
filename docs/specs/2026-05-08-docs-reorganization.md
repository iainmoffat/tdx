# Docs Reorganization Spec

**Date:** 2026-05-08
**Goal:** Restructure user-facing docs to mirror the `tdx` command tree, replacing the flat 1300-line `docs/guide.md` with a thin index plus per-top-level-command files. Replace the README's duplicated command tables with a single ASCII command tree.

## Motivation

`docs/guide.md` has grown to ~1300 lines with a flat 15-entry TOC mixing concept sections (Concepts, JSON Output, Configuration) with subcommand groups (Auth, Profiles, Time Entries, Week View, Templates, Week Drafts, Time Reports, People, MCP). The TOC has drifted twice in the last week (v0.15.1 patched four missing entries plus title-case outliers). The README duplicates much of the same surface in nine separate command tables, producing three places to update for every command change (`README.md`, `docs/guide.md`, in-source `--help`). The next phase will expand tdx into additional TD API surface, which under the current structure means pasting more sections into the same monolith.

## Decisions

These were settled during brainstorming on 2026-05-08:

1. **Split per top-level command.** `docs/guide.md` becomes a thin index; each top-level command (`auth`, `time`, `people`, `mcp`) gets its own page in `docs/guide/`. (Question Q1, option b.)
2. **Single file per top-level command.** No further per-second-level splits today (e.g. `tdx time` stays in one `time.md`). Defer further splitting until a file is genuinely cluttered. (Q2, option a.)
3. **ASCII tree, two levels deep, in `docs/guide.md`.** Top-level commands plus their immediate children (e.g. `tdx time week → pull / push / refresh / ...`). Hand-maintained — no auto-generation. (Q3.)
4. **README keeps the ASCII tree, drops all command tables.** README's job is "what is this and what can it do?" The tree answers that; the guide owns the detailed reference. (Q4, option c.)
5. **Command-as-heading inside per-command files.** `## tdx auth login`, `### tdx time week refresh`. Anchors are deterministic and grep-friendly. Friendlier prose-style headings (e.g. "Strategies", "Apply modes") survive as `####` sub-topics within a leaf-command section. (Q5.)
6. **Cross-cutting sections live in `docs/guide.md`.** Concepts, JSON Output, Storage Layout, Configuration, Shell Completions stay at the entry point — they're short, cross-cutting, and orient the reader before they navigate to a command page.

## File structure

```
docs/
├── guide.md            # entry-point: intro + ASCII tree + cross-cutting sections + links
└── guide/
    ├── auth.md         # tdx auth (+ profile subgroup)
    ├── time.md         # tdx time (entry + template + type + week + report)
    ├── people.md       # tdx people
    └── mcp.md          # tdx mcp serve + tool catalog
```

Approximate sizes after migration:
- `docs/guide.md` — ~250 lines
- `docs/guide/auth.md` — ~120 lines
- `docs/guide/time.md` — ~850 lines (largest; defers further split)
- `docs/guide/people.md` — ~100 lines
- `docs/guide/mcp.md` — ~200 lines

## Heading scheme

Each per-command file uses a single h1 for its top-level command, h2 for second-level groups or leaves, h3 for leaves under a subgroup, h4 for sub-topics within a leaf.

**Auth example:**
```markdown
# tdx auth

## tdx auth login
## tdx auth logout
## tdx auth status
## tdx auth profile
### tdx auth profile list
### tdx auth profile add
### tdx auth profile remove
### tdx auth profile use
```

**Time example (showing the deepest case):**
```markdown
# tdx time

## tdx time entry
### tdx time entry list
### tdx time entry show
### tdx time entry add
### tdx time entry update
### tdx time entry delete

## tdx time template
### tdx time template derive
### tdx time template list
### tdx time template show
### tdx time template edit
### tdx time template clone
### tdx time template delete
### tdx time template apply
#### Apply modes
#### Ownership markers
#### Day filtering
#### Hour overrides
#### Rounding
### tdx time template compare

## tdx time type
### tdx time type list
### tdx time type for

## tdx time week
[overview prose: drafts concept, sync state, lifecycle]
### tdx time week show
### tdx time week locked
### tdx time week pull
### tdx time week list
### tdx time week status
### tdx time week edit
### tdx time week diff
### tdx time week preview
### tdx time week push
### tdx time week delete
### tdx time week set
### tdx time week note
### tdx time week history
### tdx time week new
### tdx time week copy
### tdx time week rename
### tdx time week reset
### tdx time week refresh
#### Strategies
#### Worked example
#### Surface strategy + tdx time week resolve
### tdx time week rebase
### tdx time week resolve
### tdx time week archive
### tdx time week unarchive
### tdx time week snapshot
### tdx time week restore
### tdx time week prune

## tdx time report
### tdx time report status
#### Columns
#### TD endpoints used
#### Permissions
#### Selectors (exactly one required)
#### Date range
#### Filters
#### Output formats (mutually exclusive)
#### Examples
```

**People example:**
```markdown
# tdx people

## tdx people search
## tdx people show
## tdx people accounts
### tdx people accounts list
## tdx people pools
### tdx people pools list
#### How `--account` resolves names
```

**MCP example:**
```markdown
# tdx mcp

## tdx mcp serve
### Configure in your AI tool
### Available tools
[per-phase tool tables migrated from README]
### Safety model
```

Each per-command file gets its own TOC at the top listing h2s. `time.md` lists h3s as well because of its size.

## `docs/guide.md` contents (the index)

```markdown
# tdx User Guide

This guide covers all tdx features in depth. For a quick reference of
commands and flags, see the [README](../README.md).

---

## Command tree

[ASCII tree — see "Command tree" section below for exact rendering]

## Reference

- [tdx auth](guide/auth.md) — authentication and profiles
- [tdx time](guide/time.md) — time entries, week drafts, templates, reports
- [tdx people](guide/people.md) — find users, accounts, resource pools
- [tdx mcp](guide/mcp.md) — MCP server for AI agents

---

## Concepts
[unchanged from current guide.md]

## JSON Output
[unchanged]

## Storage Layout
[unchanged]

## Configuration
[unchanged]

## Shell Completions
[unchanged]
```

## Command tree (canonical rendering)

Single hand-maintained block, copied identically into `README.md` and `docs/guide.md`:

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

The `tdx time week` row is long but kept on a single visual line (rendered as a wide row, line-wraps in narrow viewers); this captures the full second-level surface without listing 25 separate child rows.

Tree is regenerated by hand on doc-touching releases. No automation in this pass — YAGNI.

## `README.md` changes

**Keep unchanged:** intro, Install, Quick Start, MCP Integration JSON snippet, JSON Output paragraph, Configuration table, Development, License.

**Replace** lines ~60-175 (the nine command tables: Auth, Time Entries, Time Week, Time Week Drafts, Time Types, Templates, Time Reports, People, MCP, Other) with:
1. The ASCII command tree (same block as in `docs/guide.md`).
2. A single line: "Full reference: [User Guide](docs/guide.md)."

**Replace** lines ~196-254 (per-phase MCP tool tables: Phase A read-only, Phase A mutating, Phase B.1 read-only, Phase B.1 mutating, Phase B.2a, Time Reports Phase C, People) with a teaser sentence: "44 tools total (22 read-only, 22 mutating). All mutating tools require `confirm: true`. See [tdx mcp](docs/guide/mcp.md) for the full tool catalog and safety model."

## Content migration map

Pure cut-and-paste from current `docs/guide.md` into the new files. No prose rewriting in this pass.

| Current section in `guide.md` | New location |
|---|---|
| Concepts | `docs/guide.md` (unchanged) |
| Authentication | `docs/guide/auth.md` → `## tdx auth login`, `## tdx auth status`, `## tdx auth logout` |
| Profiles | `docs/guide/auth.md` → `## tdx auth profile` (with h3 leaves: list, add, remove, use) |
| Time Entries | `docs/guide/time.md` → `## tdx time entry` |
| Week View (`show`, `locked`) | `docs/guide/time.md` → `## tdx time week` (merged with Week Drafts) |
| Time Types | `docs/guide/time.md` → `## tdx time type` |
| Templates | `docs/guide/time.md` → `## tdx time template` |
| Week Drafts | `docs/guide/time.md` → `## tdx time week` (merged with Week View) |
| Time Reports | `docs/guide/time.md` → `## tdx time report` |
| People | `docs/guide/people.md` |
| Storage Layout | `docs/guide.md` (unchanged) |
| MCP Server | `docs/guide/mcp.md` (plus per-phase tool tables migrated from README) |
| JSON Output | `docs/guide.md` (unchanged) |
| Shell Completions | `docs/guide.md` (unchanged) |
| Configuration | `docs/guide.md` (unchanged) |

The "Week View" + "Week Drafts" merge under `## tdx time week` reflects the actual command tree — `tdx time week show`, `tdx time week locked`, and the draft commands all live under the same `tdx time week` group; today's split into two top-level guide sections is artificial.

## Self-review pass

After all content is moved:
- Read each new file end-to-end once. Verify each leaf command's prose is intact.
- Search every new file for inbound `[text](#anchor)` markdown links. Repoint any that crossed file boundaries to `guide/<file>.md#anchor`.
- Verify each per-command file has a working top-of-file TOC.
- Verify `docs/guide.md` lists all four per-command pages.
- Verify the ASCII tree in `docs/guide.md` and `README.md` are byte-identical.
- Run any existing markdown tooling (none today, but check) and visually confirm the GitHub-rendered output looks right.

No new content. No prose rewrites. If a section is found to have outdated info during the move, note it for a separate follow-up — do not fix in this PR.

## Out of scope

- No regeneration of `--help` text or auto-generated command docs.
- No prose rewriting — pure structural move. If existing wording is awkward, it stays awkward in this pass.
- No splitting of `time.md` further (deferred; will revisit if a future addition pushes any single file past ~1500 lines).
- No changes to `docs/specs/`, `docs/plans/`, or `docs/manual-tests/`.
- No PR template, contributor docs, or new top-level docs (e.g. CONTRIBUTING.md).
- No automation for generating the ASCII tree — hand-maintained for now.
- No changes to `internal/...` or any code path. Strictly docs.

## Risk and rollback

- **Inbound anchor links to `guide.md#section`:** verified via `grep -rn "guide.md#"` on 2026-05-08 — none exist outside the file. Restructure is a clean break for external linkers.
- **Search engine / Google links to `guide.md` anchors:** the file remains at `docs/guide.md` so the URL resolves. Old anchors won't match new content but readers land on the index, see the tree, and click through. Acceptable.
- **CI:** no doc-checking CI today. Manual review suffices.
- **Rollback:** single PR; revert is `git revert <sha>`.

## Acceptance criteria

1. `docs/guide.md` exists, contains: intro, command tree, reference link list, Concepts, JSON Output, Storage Layout, Configuration, Shell Completions. ~250 lines.
2. `docs/guide/auth.md`, `docs/guide/time.md`, `docs/guide/people.md`, `docs/guide/mcp.md` exist with content from the migration map.
3. Each per-command file uses the heading scheme defined above; anchors are deterministic.
4. `README.md` no longer contains the nine command tables or the per-phase MCP tool tables. ASCII tree appears once, with a pointer to the guide. MCP tool teaser points to `docs/guide/mcp.md`.
5. `README.md` and `docs/guide.md` ASCII trees are byte-identical.
6. `grep -rn "guide.md#" docs/ README.md` returns zero results (no broken intra-doc anchor links).
7. No code changed. `git diff --stat` covers only `README.md`, `docs/guide.md`, and the four files under `docs/guide/`. (Plus this spec.)
