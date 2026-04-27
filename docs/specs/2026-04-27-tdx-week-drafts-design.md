# tdx — Local Week Drafts Design Spec

**Date:** 2026-04-27
**Status:** Approved (brainstorming complete)
**Owner:** ipm
**Targets:** Phase A (MVP) through Phase F (polish); see §15 for phasing.

---

## 0. Spec basis & decisions log

This spec extends the [framework design](2026-04-10-tdx-framework-design.md) and the read/write-ops phase specs. It introduces **week drafts** — first-class, locally-stored, dated week documents that complement (do not replace) templates.

Decisions made during brainstorming, locked here as inputs to every section below:

| # | Decision | Implication |
|---|---|---|
| Q1 | **Hybrid (C):** local week's on-disk shape is per-cell with optional metadata; default editing experience stays grid-of-numbers. | The C-hybrid escape hatch is real but invisible by default. |
| Q2 | **Reading X (revised):** the draft is the desired state for entries it has knowledge of. Cleared pulled cells = deletes on push. Untracked remote entries are out of scope for the push. | No separate tombstone affordance. `Backspace` on a pulled cell is the deletion gesture. |
| Q3 | **B+C combined:** identity is `(profile, weekStart, name)` with `name` defaulting to `default`; auto-snapshots are taken before every destructive op, with bounded retention and pinning. | Multiple alternate drafts per week are supported; an undo trail is automatic. |
| Q4 | **B now, C later:** per-profile YAML at `~/.config/tdx/profiles/<profile>/weeks/...`; cwd-scoped workspace mode (`.tdx/`) is deferred to Phase E. | Storage scope is per-profile starting in Phase A, including for templates. |
| — | **Templates-per-profile migration** lands in Phase A alongside the new draft storage layout. | One disruptive storage change rather than two. |
| — | **Documentation discipline:** every phase ships with `README.md`, `docs/guide.md`, and `docs/manual-tests/` deliverables. | Docs tasks are first-class line items in every phase plan. |

---

## 1. Product expansion brief

Today, `tdx` treats live weeks as transient API state: fetched, displayed, written through one entry at a time. The only locally-persisted artifact is a **template** — week-shape, dateless, intentionally lossy. Templates are excellent for *replay-this-pattern-weekly*, but they leave a real gap: there's no way to **hold a specific week in your hands, shape it, review it, and push it back atomically**.

This iteration adds **week drafts** — first-class, locally-stored, dated week documents that you can pull from TD, edit in the grid editor, validate, diff, preview, and push back, with full safety guarantees. Templates remain the right tool for *patterns*; week drafts are the right tool for *instances*.

### Beneficiaries

- **Human users** get a workspace they can plan, correct, and stage into. "Plan next week offline," "correct this week's Tuesday," "snapshot the week before I touch it" become single-command flows instead of sequences of `entry add/update/delete`.
- **AI agents (via MCP)** get a stable artifact to reason about — a dated, structured document with explicit dirty/clean/conflicted status, an `expectedDiffHash` for safe push, and a snapshot history for rollback. Agents can read a draft, propose edits, preview, and confirm — instead of reconstructing intent from individual entry queries.
- **Both audiences** gain WYSIWYG round-trip and deterministic diffs that today don't exist outside the template-apply path.

### Why this is better than templates + entry ops alone

- Templates collapse per-day variation; drafts preserve it (per-cell metadata when needed).
- Entry ops are imperative ("change entry #98765 to 3 hours"); drafts are declarative ("this is the week I want — make it so"). Declarative scales better for both editing UIs and agents.
- Drafts are durable between sessions: leave a partial plan for next week sitting on disk, return tomorrow, refresh against remote, finish the plan, push.

---

## 2. New capability inventory

Grouped by lifecycle stage. **Bold = MVP.** *Italic = strong stretch.* Plain = later phases.

### Acquiring a draft

- **Pull a live week into a draft** (`pull` from TD, default `name=default`)
- *Create a blank dated draft* (no rows; for staging next week from scratch)
- *Create a draft from a template* (seed rows from `<template>`, leave `weekStart` set)
- Create a draft by copying another draft (same date or shifted)
- Save a live week as a draft before editing (explicit "snapshot live to draft" alternate of pull)

### Living with drafts

- **List local drafts** (across dates and names; flag for "drafts with pending pushes")
- **Show a draft** (grid view; mirrors `tdx time week show`)
- **Edit a draft** (TUI grid editor in MVP; `--web` later)
- *Edit draft metadata* (description, tags, name rename)
- Annotate a cell (per-cell description, time-type, billable — the C-hybrid escape hatch)
- *Validate a draft against remote constraints*
- *Add notes to a draft* (free-form `notes` field, useful for human/agent context)

### Comparing & previewing

- **Diff a draft against live remote** (cell-level)
- Diff a draft against a template
- Diff a draft against another draft (or a snapshot)
- **Preview a push** (returns ReconcileDiff + `expectedDiffHash`, with a fourth `delete` action kind)

### Syncing

- **Push a draft to TD** (creates/updates/**deletes** per the preview; `--yes` + matching hash + `--allow-deletes` if any deletes)
- Refresh a draft from remote (re-pull preserving local edits — explicit "rebase" semantics)
- *Detect drift since pull* (`stale` flag on every read)
- Reset a draft to remote (discard local edits, re-pull)

### Snapshots & history

- **Auto-snapshot before destructive ops** (pull-overwrites-dirty, push, delete)
- List snapshots for a draft
- Restore a snapshot
- Manual snapshot (`--keep` to pin)
- Prune snapshots (configurable retention, default 10)

### Template ↔ draft interop

- Apply a template into a draft (modes: add / replace-matching / replace-mine; writes to the draft, not TD)
- Derive a template from a draft (skip remote round-trip)
- Compare a template to a draft

### Cleanup

- **Delete a draft** (with confirm + auto-snapshot)
- Archive a draft (out of `list` by default)

### Adjacent

- Shift a draft forward (copy this week's draft to next week, dates advanced)
- Partial-week push (the existing `--days` flag, lifted)
- Export/import a draft as a portable file
- Drafts as MCP-friendly planning artifacts (see §10)

---

## 3. Command surface (under `tdx time week …`)

All new verbs live under `tdx time week`. The two existing subcommands keep their meanings: `tdx time week show` shows live, `tdx time week locked` shows locked days.

**Phase column convention:** the phase listed for a command is the phase the command **first appears**. Flags listed alongside may be introduced incrementally — the authoritative per-phase scope is §14 (MVP) and §15 (phase plan). For example, `pull`'s `--name` flag is meaningful only once Phase B introduces multiple drafts per week.

### Acquisition

| Command | Role | Phase | Key flags |
|---|---|---|---|
| `tdx time week pull <date>` | Pull live week into a draft. Refuses to overwrite a dirty draft unless `--force`; auto-snapshots if it does. | A | `--name <slug>`, `--force`, `--profile` |
| `tdx time week new <date>` | Create a blank or seeded draft. | B | `--name`, `--from-template <name>`, `--from-draft <date>[/<name>]`, `--shift ±N` |
| `tdx time week copy <srcDate>[/<name>] <dstDate>` | Copy a draft (optionally shifted). | B | `--as <new-name>`, `--shift-only` |

### Inspection

| Command | Role | Phase | Key flags |
|---|---|---|---|
| `tdx time week list` | List drafts across all dates. | A | `--dirty`, `--ahead`, `--conflicted`, `--date <date>`, `--archived` (B), `--all-profiles` (E), `--json`, `--profile` |
| `tdx time week show <date>` | Existing command. Live by default; `--draft [name]` renders a draft. | A (extension) | `--draft [name]`, `--annotated`, `--json` |
| `tdx time week status <date>[/<name>]` | One-screen summary. | A | `--json`, `--profile` |
| `tdx time week history <date>[/<name>]` | List snapshots. | A (read), B (ops) | `--json`, `--limit N` |

### Editing

| Command | Role | Phase | Key flags |
|---|---|---|---|
| `tdx time week edit <date>[/<name>]` | Open the grid editor on a draft. | A (TUI), C (web) | `--web`, `--day <weekday>`, `--row <id>` |
| `tdx time week set <date>[/<name>] <row>:<day>=<hours>` | Non-interactive cell write. Repeatable. | A | `--description`, `--type`, `--billable` |
| `tdx time week note <date>[/<name>]` | Edit free-form notes. | A | `--append` |

### Compare / preview / push

| Command | Role | Phase | Key flags |
|---|---|---|---|
| `tdx time week diff <date>[/<name>]` | Diff vs remote / template / snapshot / draft. | A (vs remote), D (others) | `--against remote\|template <name>\|snapshot N\|draft <date>[/<name>]`, `--json` |
| `tdx time week validate <date>[/<name>]` | Run all push pre-checks. | A (light, in preview), C (standalone) | `--json` |
| `tdx time week preview <date>[/<name>]` | Compute the push reconcile diff. | A | `--days`, `--mode`, `--json` |
| `tdx time week push <date>[/<name>]` | Execute the push. Two-phase by default. | A | `--yes`, `--expected-diff-hash`, `--days`, `--mode`, `--allow-deletes` |

### Refresh / drift

| Command | Role | Phase | Key flags |
|---|---|---|---|
| `tdx time week refresh <date>[/<name>]` | Re-pull and replay local edits. Auto-snapshots first. | B | `--strategy ours\|theirs\|surface`, `--profile` |
| `tdx time week reset <date>[/<name>]` | Discard local, re-pull. Auto-snapshots first. | B | `--yes` |
| `tdx time week rebase <date>[/<name>]` | Alias for `refresh`. | B | (same as refresh) |

### Snapshots

| Command | Role | Phase | Key flags |
|---|---|---|---|
| `tdx time week snapshot <date>[/<name>]` | Manual snapshot. | B | `--keep`, `--note` |
| `tdx time week restore <date>[/<name>] --snapshot N` | Restore. Auto-snapshots first. | B | `--yes` |
| `tdx time week prune <date>[/<name>]` | Drop unpinned snapshots beyond retention. | B | `--all`, `--older-than 30d` |

### Template interop

| Command | Role | Phase |
|---|---|---|
| `tdx time week apply-template <date>[/<name>] <template>` | Apply template rows into a draft. Modes mirror live apply, but write to the draft. | D |
| `tdx time template derive <name> --from-draft <date>[/<dname>]` | Derive a template from a draft. | D |
| `tdx time template compare <name> --to-draft <date>[/<dname>]` | Diff a template against a draft. | D |

### Cleanup & portability

| Command | Role | Phase | Key flags |
|---|---|---|---|
| `tdx time week delete <date>[/<name>]` | Delete a draft (auto-snapshot first). | A | `--yes`, `--keep-snapshots` |
| `tdx time week archive <date>[/<name>]` | Move to archive (out of `list`). | B | `--restore` |
| `tdx time week export <date>[/<name>]` | Write draft YAML to stdout / file. | D | `--out <file>` |
| `tdx time week import <file>` | Read a draft YAML and save. | D | `--as <date>[/<name>]`, `--force` |

### Workspace mode (Phase E)

| Command | Role | Phase |
|---|---|---|
| `tdx time week init` | Create `.tdx/weeks/` in cwd; subsequent draft commands prefer this scope. | E |
| `tdx time week where` | Show active scope (global vs workspace). | E |

### Discoverability hooks

- `tdx time week show <date>` (no `--draft`) prints a one-line banner if a `default` draft exists for that date, pointing to `--draft`.
- After the first successful `pull`, the success message points users to `tdx time week --help` and the user guide.

---

## 4. Local week workspace model

Six product concepts. Names are canonical and used consistently across spec, CLI, MCP, and JSON.

### 4.1 Week draft

A single editable, dated, named document representing one user's intended state of one Sun–Sat week in one TD profile.

**Identity:** `(profile, weekStart, name)`. Default `name = default`.

**Carries:**
- Identity fields (`profile`, `weekStart`, `name`).
- **Provenance:** `blank | pulled (with watermark) | from-template <name> | from-draft <date>/<name> [shifted-by N]`.
- **Content:** ordered list of **draft rows**, each with target + type + billable + description + 7 **draft cells** (Sun..Sat).
- **Notes:** free-form `notes` field. Never sent to TD.
- **Metadata:** `createdAt`, `modifiedAt`, `pulledAt?`, `pushedAt?`, `tags[]`.
- **Sync state:** computed from cell states + watermark comparison; not stored.

A draft is the *user's representation of intent*. It is not the truth about TD's state — that's what `diff`/`preview`/`push` are for.

### 4.2 Draft row

Same shape as a `TemplateRow` today. Adds:
- **Row provenance:** `pulled | template | blank`. Pulled rows carry the set of source TD entry IDs that contributed to the row at pull time.

### 4.3 Draft cell

Atomic unit; one row × one weekday.

| Field | Description |
|---|---|
| `hours` | The cell's value (the editor primarily edits this). |
| `state` | Computed: `untouched | edited | added | conflict | invalid`. |
| `sourceEntryID?` | If pulled, the TD entry ID this cell originated from. Push uses this to update or delete. |
| `perCell?` | Optional `description`, `timeTypeID`, `billable` overriding the row's defaults for this cell only (Phase C surface; pull may auto-populate when remote can't be represented at the row level). |

**Per Reading X (Q2):** there is **no** `tombstoned` state. A cleared pulled cell (`hours=0` with a `sourceEntryID`) is the canonical delete-on-push gesture.

### 4.4 Snapshot

Immutable point-in-time copy of a draft, produced automatically before destructive ops and manually via `tdx time week snapshot`.

**Identity:** `(profile, weekStart, draftName, sequence)`.

**Carries:** the full draft document, an op tag (`pre-pull | pre-push | pre-refresh | pre-restore | pre-delete | manual`), an optional pin (`--keep`), and an optional note.

Snapshots are read-only. To "edit a snapshot," restore it (which auto-snapshots the current state first).

### 4.5 Pull watermark

A draft pulled from remote carries a watermark: the `pulledAt` time and a deterministic fingerprint of the remote `WeekReport` at pull time. Used for:
1. **Stale detection** — has remote changed since pull?
2. **Hash-protection input** — `expectedDiffHash` includes the watermark fingerprint, so a refresh between preview and push always invalidates the hash.

### 4.6 Sync state (the four-light model)

A draft is exactly one of these states. Visible across all output forms.

| State | Meaning |
|---|---|
| **clean** | Local cells exactly match the pull watermark. Push would be a no-op. |
| **dirty** | Local has uncommitted edits relative to the watermark. Push has work. |
| **stale** | Remote fingerprint has advanced since pull. Independent of dirty/clean. |
| **conflicted** | Refresh detected remote changes overlapping local edits; not auto-merged. |

**Special cases:** A `nascent` draft (created via `new` with no source) is reported as dirty until first push. After successful push, the draft becomes clean with a fresh watermark from the just-pushed state.

---

## 5. Pull / edit / push lifecycle

### 5.1 State diagram

```
          ┌──────────────────────────────┐
          │           ABSENT             │
          └──┬───────────────┬───────────┘
             │ pull <date>   │ new <date> [--from-template] [--from-draft]
             ▼               ▼
        ┌─────────────────────────────────┐
        │      DRAFT EXISTS (somewhere    │
        │  on the clean/dirty/stale/      │
        │       conflicted lattice)       │
        └─┬────┬────┬────┬────┬────┬──────┘
          │    │    │    │    │    └── delete → ABSENT (with snapshot)
          │    │    │    │    └────── archive → archived
          │    │    │    └─────── push --yes → clean + fresh watermark
          │    │    └─────── refresh / rebase → clean | dirty | conflicted
          │    └─────── edit ops → dirty
          └─────── snapshot / restore → may transition; auto-snapshots first
```

### 5.2 What the user sees

(Representative output samples; see brainstorming session for full set.)

**Pull (no existing draft):**
```
Pulling week 2026-05-04 from work (ufl.teamdynamix.com)... done
Created draft 2026-05-04/default (3 rows, 12 cells, 40h00m, status: open)

  tdx time week show 2026-05-04 --draft     # view the draft
  tdx time week edit 2026-05-04             # edit it
```

**Pull when a dirty draft exists:**
```
A dirty draft exists for 2026-05-04 (default, 3 cells edited).
Pulling would overwrite local edits.

  tdx time week refresh 2026-05-04          # re-pull and merge edits
  tdx time week pull 2026-05-04 --force     # auto-snapshot, then overwrite
  tdx time week pull 2026-05-04 --name temp # pull into a new named draft
```

**Status:**
```
2026-05-04 / default
  Profile:      work
  Pulled:       2026-04-27 09:12:14 (6h ago)
  Pushed:       never
  Sync state:   dirty (and STALE — remote advanced since pull)
  Cells:        12 untouched · 3 edited · 0 conflict
  Total hours:  41.5h (was 40.0h on pull)

  Action recommended:
    tdx time week refresh 2026-05-04
```

**Preview:**
```
Week 2026-05-04   (mode: replace-matching, draft: default)

  ROW              SUN    MON    TUE    WED    THU    FRI    SAT    TOTAL
  -----------------------------------------------------------------------
  ticket #123      .      8.0~   8.0=   8.0=   8.0=   4.0~   .      36.0
    L Work
  project 456      .      .      4.0=   .      .      −4.0   .      0.0
    L Planning
  -----------------------------------------------------------------------

  Symbols: + create  ~ update  = match (skip)  − delete  x blocked

  Summary: 0 creates · 2 updates · 1 delete · 4 skips · 0 blocked
  Diff hash: 8a7f...c2e1
```

**Push success:**
```
Pushing draft 2026-05-04/default (1 delete, 2 updates)... done in 1.2s
  Deleted entry #98765 (project 456 / Planning, Friday, 4.0h)
  Updated entry #98731 (ticket #123 / Work, Monday, 6.0h → 8.0h)
  Updated entry #98741 (ticket #123 / Work, Friday, 8.0h → 4.0h)

Draft 2026-05-04/default is now clean (3 rows, 11 cells, 36.0h).
```

**Push hash mismatch:**
```
Push aborted: remote week changed since preview.

  Re-run:  tdx time week preview 2026-05-04
  Or:      tdx time week refresh 2026-05-04

A snapshot of the draft was taken before the failed push:
  pre-push #4 at 2026-04-27T15:02:11Z
```

### 5.3 Locked, submitted, approved

- **Locked day:** per-cell `BlockerLocked`. Push proceeds for unblocked work; blockers reported.
- **Submitted week:** hard week-level blocker. `validate` and `preview` show a banner; `push` refuses with no override.
- **Approved week:** harder still. `push` refuses with no override.
- **Immutable remote fields:** validate runs probes; conflicts at push time produce per-action failures with field name + TD message; remaining actions proceed.

---

## 6. Week editing UX

### 6.1 Default experience

Same Sun..Sat × rows grid as today's template editor. Arrow keys / Tab navigate. Number keys edit. `Enter` / `Backspace` / `Ctrl-S` / `Esc` behave as today. **For pure hour edits, the experience is unchanged from `tdx time template edit`.**

What's new is the surrounding context:
- **Status bar:** sync state, dirty cell count, last pull/push age.
- **Cell annotations** (default-on, toggle with `^A`):
  - `untouched` — plain
  - `edited` — yellow / `*` suffix
  - `added` — green / `+` suffix
  - `conflict` — red warning + symbol; cursor jumps here on entry
  - `invalid` — red border + warning glyph
- **Live row & day totals.**
- **Validation in-place:** invalid type/target combos flagged in the gutter; editing not blocked, but push refuses until cleared.

### 6.2 Deletes via clearing (Reading X)

A cell that was pulled with a `sourceEntryID`, when cleared via `Backspace` (or set to 0), becomes a delete on push. There is no separate `D` key.

**Pre-save confirm:** if any cleared pulled cells exist when the user presses `Ctrl-S`, the editor displays a small confirm dialog naming the entry IDs that will be marked for deletion at push time. (Not deleted yet — push is still required.)

### 6.3 Per-cell metadata view (Phase C)

`^I` opens a cell detail panel showing description / time-type / billable, with override toggles. Cells with overrides show a `°` glyph in the grid.

### 6.4 Notes view

`^N` opens notes. Persisted to YAML, returned in MCP `get_week_draft`, never sent to TD.

### 6.5 Preview-from-editor

`^P` runs preview without leaving the editor; shows the same annotated grid in a modal pane. Repeatable as a gut-check during shaping. Does not push.

### 6.6 Save semantics

`Ctrl-S` writes the draft YAML to disk. **It does not push.** This is the single most important UX invariant of the editor.

### 6.7 Web variant parity (Phase C)

Everything above works in `--web` too — the existing browser editor extended with the status bar, pre-save confirm, cell detail panel, preview pane, notes pane.

---

## 7. Template ↔ draft interoperability

### 7.1 Wall

> **Templates are patterns; drafts are instances.** Use a template when you don't care which specific week it lands on. Use a draft when you do.

- Templates: dateless, idempotent across weeks, optimized for replay. Small, sharable.
- Drafts: dated, aware of specific TD entry IDs, optimized for editing-and-pushing-this-specific-week.

### 7.2 Workflow matrix

| Workflow | Command | Phase |
|---|---|---|
| Seed a draft from a template | `tdx time week new <date> --from-template <name>` | B |
| Apply a template into an existing draft | `tdx time week apply-template <date>[/<name>] <template> --mode …` | D |
| Compare a template to a draft | `tdx time template compare <name> --to-draft <date>[/<dname>]` | D |
| Derive a template from a draft | `tdx time template derive <name> --from-draft <date>[/<dname>]` | D |
| Pull → derive | `tdx time week pull <date> && tdx time template derive <name> --from-draft <date>` | A + D |

### 7.3 Confusion-prevention

- Template names and draft names share no namespace.
- `--from-template` and `--from-draft` are distinct flags. No magic resolution.
- `new --from-template` banner names the template + draft name + weekStart.

### 7.4 Provenance preservation

Drafts created from templates carry `provenance.fromTemplate.{name, appliedAt}`. Templates derived from drafts carry `derivedFrom.{draftName, weekStart}` (analogous to today's derive-from-week). Surfaced in `status`, `list --json`, MCP `get_week_draft`.

### 7.5 What templates do NOT gain

Out of scope: dating templates, embedding entry IDs in templates, per-cell metadata in templates. Templates stay simple, dateless, lossy. Anything needing fidelity is a draft.

---

## 8. Sync and reconciliation behavior

The explicit safety contract.

### 8.1 Pull never silently overwrites

| Existing state | `pull` behavior |
|---|---|
| No draft | Create. |
| Clean, fresh | No-op (banner: "draft is up-to-date"). |
| Clean, stale | Update watermark + adopt remote-changed rows. Banner explains. |
| Dirty | **Refuse.** Suggest `refresh` / `pull --force` / `pull --name <other>`. |
| Conflicted | **Refuse.** Suggest editor resolution or `refresh --strategy`. |

### 8.2 Refresh = re-pull + replay (Phase B)

`refresh` is the merge primitive. Always auto-snapshots first.

Per-cell merge rules:
- Untouched local + unchanged remote → carry forward.
- Untouched local + **changed remote** → adopt remote.
- Edited local + unchanged remote → keep local.
- Edited local + **changed remote** → **conflict**. Surfaced, not auto-resolved.
- A cleared pulled cell whose source entry no longer exists remotely → silently retire (goal achieved).

Strategies: `--strategy ours` / `theirs` skip prompting; default surfaces conflicts.

### 8.3 Diff is structural and bounded

`diff` returns deterministic cell-level differences across three axes:
- `--against remote` (default; recomputes watermark)
- `--against template <name>` (diff vs apply-this-template-into-fresh-draft)
- `--against snapshot <N>` (diff vs prior snapshot)
- `--against draft <date>[/<name>]` (diff vs another draft)

Each diff line: `(row, day, kind, before, after, sourceID?)`; kind ∈ `match | add | update | delete | conflict | invalid`.

### 8.4 Preview-only is the default for push

`push` without `--yes` is identical to `preview`. With `--yes`, re-reconciles, verifies hash, executes. Identical wording at decision time and execution time.

### 8.5 Push semantics (Reading X)

**The draft is the desired state for the entries it has knowledge of.**

| Cell shape | Action |
|---|---|
| `hours > 0`, no `sourceEntryID`, no remote match | `Create` |
| `hours > 0`, no `sourceEntryID`, remote match (per `--mode`) | `Skip` (`add`) / `Update` (`replace-matching`) / per-ownership (`replace-mine`) |
| `hours > 0`, with `sourceEntryID`, hours/metadata changed | `Update` |
| `hours > 0`, with `sourceEntryID`, no change | `Skip` (reason: `noChange`) |
| `hours == 0`, with `sourceEntryID` | **`Delete`** (cleared pulled cell) |
| `hours == 0`, no `sourceEntryID` | No action (cell exists but doesn't assert anything) |
| Untouched cell, with `sourceEntryID` | `Skip` (reason: `untouched`) |

**Untracked remote entries** (entries on remote that the draft has never referenced) are out of scope for push — never deleted, never updated. Drift is what `stale` and `refresh` are for.

### 8.6 `expectedDiffHash` covers deletes & watermark

Hash inputs (additive over today's set):
- `Create | Update | Skip | Delete` actions (with sourceEntryID + pre-delete fingerprint for deletes)
- Blockers
- Mode + Template/Draft name + WeekStart
- **Pull watermark fingerprint** — so a refresh between preview and push always invalidates the hash.

### 8.7 Drift detection on every read

Status / list / show --draft / MCP `get_week_draft` make one bounded remote probe to compute current week fingerprint and report `stale` if it diverges from the watermark. **No automatic refresh.** Disable with `--no-remote-check`; default is to probe.

### 8.8 Never silently destructive (gate matrix)

| Operation | Gate |
|---|---|
| Push (any) | `--yes` + matching `expectedDiffHash` |
| Push with deletes | `--yes` + matching `expectedDiffHash` + `--allow-deletes` |
| Push to submitted/approved | refused unconditionally |
| Pull overwriting dirty | `--force` + auto-snapshot |
| Reset (discard local) | `--yes` + auto-snapshot |
| Refresh auto-resolve | `--strategy ours\|theirs` (default surfaces conflicts) |
| Delete draft | `--yes` + auto-snapshot (unless `--no-snapshot`) |
| Restore snapshot | `--yes` + auto-snapshot of current first |
| Prune snapshots | `--yes` + refuses to drop pinned |

### 8.9 Batch / sync limits

- Per-push action ceiling, default 100, configurable. Exceeded → preview succeeds, push refuses with banner suggesting `--days` split.
- Network failures: fail-fast by default. `--continue-on-error` opts into best-effort.
- Per-action timing reported in push output (Phase F polish).

---

## 9. Adjacent use cases

### 9.1 Stage next week from a template (the headline planning workflow)

```bash
tdx time week new 2026-05-04 --from-template canonical
tdx time week edit 2026-05-04
tdx time week preview 2026-05-04
tdx time week push 2026-05-04 --yes
```

### 9.2 Mid-week correction

```bash
tdx time week pull 2026-04-27
tdx time week edit 2026-04-27
# fix Tuesday's hours; clear Wednesday's bogus entry
tdx time week push 2026-04-27 --yes --allow-deletes
```

### 9.3 Snapshot a live week before risky edits

```bash
tdx time week pull 2026-04-27 --name pristine
tdx time week pull 2026-04-27   # default draft
# (do edits)
tdx time week diff 2026-04-27 --against draft 2026-04-27/pristine
```

### 9.4 Shift a week forward (Phase B)

```bash
tdx time week copy 2026-04-27 2026-05-04 --shift-only
```

### 9.5 Review-only audit

```bash
tdx time week pull 2026-04-27 --name review
tdx time week show 2026-04-27 --draft review --annotated
tdx time week delete 2026-04-27/review --yes
```

### 9.6 Partial-week push

```bash
tdx time week push 2026-04-27 --days mon-fri --yes
```

### 9.7 Export / import (Phase D)

```bash
tdx time week export 2026-04-27 > my-week.yaml
tdx time week import my-week.yaml --as 2026-04-27/restored
```

### 9.8 Drafts as MCP planning artifacts

A draft is a stable handle for not-yet-confirmed planning work. An agent can pull or create a draft, edit it across multiple turns (each `update_week_draft` is small + idempotent), and request human confirmation before push. If the conversation ends without push, the draft persists for next session.

### 9.9 Excluded

Multi-week drafts (fortnight artifacts), cross-profile sync, per-cell comments beyond `description`/`notes` are explicit non-goals.

---

## 10. MCP expansion for week workspaces

Extends today's pattern: read-only tools open; mutating tools require `confirm: true`; apply-style tools require `expectedDiffHash` from a prior preview.

### 10.1 `list_week_drafts` (read)

- **Inputs:** `profile?`, filters (`dirty?`, `stale?`, `conflicted?`, `weekStart?`, `weekStartFrom?`, `weekStartTo?`, `name?`).
- **Outputs:** array of `{profile, weekStart, name, syncState, cellCounts, totalHours, pulledAt, pushedAt, provenance}`.

### 10.2 `get_week_draft` (read)

- **Inputs:** `profile?`, `weekStart`, `name?`.
- **Outputs:** full draft document + watermark + current-remote fingerprint (if probed) + `staleSince?`.

### 10.3 `pull_week_draft` (mutating; confirm:true)

- **Inputs:** `profile?`, `weekStart`, `name?`, `force?`, `confirm`.
- **Safety:** dirty-draft refusal returns structured remediation hints. Auto-snapshot before overwrite.

### 10.4 `create_week_draft` (mutating; confirm:true; Phase B)

- **Inputs:** `profile?`, `weekStart`, `name?`, `fromTemplate?` or `fromDraft?`, `confirm`.
- **Safety:** refuses to overwrite an existing draft at same identity.

### 10.5 `update_week_draft` (mutating; confirm:true)

- **Inputs:** `profile?`, `weekStart`, `name?`, `edits[] = {rowID, day, hours?, description?, timeTypeID?, billable?}`, `expectedModifiedAt?`, `confirm`.
- **Outputs:** updated draft + per-edit applied/skipped/rejected log.
- **Reading X:** no `tombstone` field. Setting `hours: 0` on a pulled cell is the deletion gesture.

### 10.6 `set_week_draft_notes` (mutating; confirm:true)

- **Inputs:** `profile?`, `weekStart`, `name?`, `notes`, `mode: replace | append`, `confirm`.

### 10.7 `apply_template_to_week_draft` (mutating; confirm:true; Phase D)

- **Inputs:** `profile?`, `weekStart`, `name?`, `template`, `mode`, `days?`, `overrides?`, `confirm`.

### 10.8 `derive_template_from_week_draft` (mutating; confirm:true; Phase D)

- **Inputs:** `profile?`, `weekStart`, `draftName?`, `templateName`, `description?`, `confirm`.

### 10.9 `diff_week_draft` (read)

- **Inputs:** `profile?`, `weekStart`, `name?`, `against: remote | template:<name> | snapshot:<N> | draft:<date>[/<name>]`.

### 10.10 `preview_push_week_draft` (read)

- **Inputs:** `profile?`, `weekStart`, `name?`, `mode?`, `days?`.
- **Outputs:** `{actions[], blockers[], creates, updates, deletes, skips, blockedCount, expectedDiffHash, pullWatermarkFingerprint}`.

### 10.11 `push_week_draft` (mutating; confirm:true; expectedDiffHash required)

- **Inputs:** `profile?`, `weekStart`, `name?`, `mode?`, `days?`, `expectedDiffHash`, `allowDeletes`, `confirm`.
- **Safety:** mirrors `apply_time_template_to_week`. Hash mismatch + deletes-without-allowDeletes both return structured errors with remediation.

### 10.12 `refresh_week_draft` (mutating; confirm:true; Phase B)

- **Inputs:** `profile?`, `weekStart`, `name?`, `strategy: ours | theirs | surface`, `confirm`.
- **Outputs:** `{draft, mergeResult, conflicts[]?}`.

### 10.13 `validate_week_draft` (read)

- **Inputs:** `profile?`, `weekStart`, `name?`.
- **Outputs:** `{valid, blockers[], invalidCells[], remoteFingerprintMatchesPull}`.

### 10.14 Snapshot / archive / delete tools

`list_week_draft_snapshots` (read), `restore_week_draft_snapshot` (mutating), `delete_week_draft` (mutating, MVP), `archive_week_draft` (mutating, Phase B). All mutating ops auto-snapshot first.

### 10.15 Agent reasoning recipe

Stated in tool descriptions where relevant:

1. Start a session with `get_week_draft` or `list_week_drafts`. Cache `modifiedAt`.
2. Before each `update_week_draft`, pass cached `modifiedAt` as `expectedModifiedAt`. On mismatch, re-read.
3. Before pushing, always call `preview_push_week_draft` and capture `expectedDiffHash`.
4. On `push_week_draft` hash mismatch: do not retry. Call `validate_week_draft` and `diff_week_draft --against remote`, then `refresh_week_draft`, then re-preview.
5. Treat `staleSince` as advisory; surface to user before pushing.

---

## 11. Output and display strategy

### 11.1 Three display tiers

- **Human terminal:** annotated grid (symbols `+ ~ = − x ? ! °`); compact `status` block; one-line actionable hints. Color opt-in / TTY-detected; symbols also work monochrome.
- **JSON (`tdx.v1.*` schemas):** new schemas — `tdx.v1.weekDraft`, `tdx.v1.weekDraftList`, `tdx.v1.weekDraftStatus`, `tdx.v1.weekDraftDiff`, `tdx.v1.weekDraftPreview`, `tdx.v1.weekDraftPushResult`, `tdx.v1.weekDraftSnapshot`, `tdx.v1.weekDraftSnapshotList`, `tdx.v1.weekDraftValidation`, `tdx.v1.weekDraftRefreshResult`. Additive-only within a schema; breaking changes get a new version.
- **MCP:** the same JSON envelopes, wrapped in MCP `CallToolResult.content`. Errors use the existing `errorResult` helper, with structured remediation hints.

### 11.2 The four-light state in output

- Human: `[clean]` / `[dirty 3]` / `[stale]` / `[conflicted 1]`.
- JSON: `"syncState": "dirty", "syncDetail": { "edited": 3, "added": 0, "conflict": 0, "stale": false }`.
- Combined states (e.g., `dirty + stale`) first-class: `[dirty 3 · stale]` in human; both flags set in JSON.

### 11.3 Diff/preview views

- **Grid view** (default): annotated grid renderer, extended with `−` for deletes.
- **Action list view** (`--list`): one line per action.
- **JSON** (`--json`).

### 11.4 Documentation commitment (cross-cutting)

Every new command, MCP tool, and concept introduced by this iteration ships with documentation in the same phase as the code, **not as follow-up**. Standing rules:

1. **`README.md`** — every new `tdx time week …` command in the command tables; new MCP tools listed; new JSON schema names listed.
2. **`docs/guide.md`** — gains a new top-level "Week drafts" section between "Templates" and "MCP Server" (concepts, lifecycle diagram, editor cheatsheet, push safety contract, worked examples for §9.1–§9.7). Plus subsections introduced by later phases (Refresh & rebase; Multiple drafts; Snapshots & history; Per-cell overrides; Export & import; Workspace mode; Performance & limits).
3. **`docs/manual-tests/phase-<x>-week-drafts-walkthrough.md`** — at least one new walkthrough per phase, runnable end-to-end against a real tenant.
4. **MCP tool descriptions** — carry the §10.15 reasoning recipe inline where relevant.
5. **Inline `--help`** — every flag has a one-line description; every command has a usage example.
6. **JSON schema docs** — `docs/guide.md`'s "JSON Output" section grows to list all new schemas with example payloads.
7. **Migration / upgrade note** — capture the templates-per-profile change in `docs/guide.md`'s "Storage layout" subsection. No silent migrations.

### 11.5 Discoverability

- `tdx time week --help` lists subcommands grouped by lifecycle.
- `tdx time week show <date>` (no `--draft`) banner if a default draft exists.
- First successful `pull` prints "Drafts live under `~/.config/tdx/profiles/<profile>/weeks/`. See `tdx time week --help` or the user guide."

---

## 12. Local storage / file model

### 12.1 Layout

```
~/.config/tdx/
├── config.yaml
├── credentials.yaml
└── profiles/
    └── work/
        ├── templates/                       # NEW: templates moved per-profile
        │   └── canonical.yaml
        └── weeks/
            ├── 2026-05-04/
            │   ├── default.yaml             # the draft itself
            │   ├── default.snapshots/
            │   │   ├── 0001-pre-pull-2026-04-27T091214Z.yaml
            │   │   ├── 0002-pre-push-2026-04-27T150211Z.yaml
            │   │   └── 0003-manual-pinned.yaml      # `--keep`
            │   ├── pristine.yaml            # named alternate (Phase B)
            │   └── pristine.snapshots/
            └── 2026-05-11/
                └── default.yaml
```

### 12.2 Templates-per-profile migration (Phase A)

- **Detect:** legacy `~/.config/tdx/templates/` exists on first run after upgrade.
- **Prompt:** offer to (a) migrate every template into the current default profile, (b) cancel and let the user move them by hand, (c) leave them at the legacy path during a deprecation window (read from both).
- **Auto-yes when only one profile exists.** Prompt only fires for multi-profile users.
- **Move:** atomically; leave `.migrated` marker in `~/.config/tdx/templates/` so the prompt doesn't re-fire.
- **Document:** new "Storage layout" subsection in `docs/guide.md`; one-line note in `README.md`.

### 12.3 Draft YAML shape (illustrative, not normative)

```yaml
schemaVersion: 1
profile: work
weekStart: 2026-05-04
name: default
notes: |
  Friday short week; conference Mon-Tue.

provenance:
  kind: pulled
  pulledAt: 2026-04-27T13:12:14Z
  remoteFingerprint: 8a7f...c2e1
  remoteStatus: open

createdAt:  2026-04-27T13:12:14Z
modifiedAt: 2026-04-27T15:01:32Z
pushedAt:   null

rows:
  - id: row-01
    target: { kind: ticket, appID: 42, itemID: 123, displayName: "Big Project" }
    timeType: { id: 7, name: "Work" }
    billable: true
    description: "API implementation"
    cells:
      - day: mon
        hours: 8.0
        sourceEntryID: 98731
        state: untouched
      - day: tue
        hours: 8.0
        sourceEntryID: 98732
        state: edited
      - day: fri
        hours: 4.0
        sourceEntryID: 98741
        state: edited
        perCell:
          description: "API implementation (short Friday)"
      - day: wed
        hours: 0
        sourceEntryID: 98740
        state: edited                 # cleared pulled cell → delete on push
```

### 12.4 Discovery & cleanup

- `tdx time week list` is the discovery surface.
- `tdx time week prune` removes unpinned snapshots beyond retention (default 10, configurable).
- `tdx time week archive <date>[/<name>]` moves to `weeks/_archive/<date>/<name>/`. Out of `list` by default.
- Wiping `~/.config/tdx/profiles/<profile>/weeks/` is safe — pure user data, not required for tdx to function.

### 12.5 Portability (Phase D)

- `tdx time week export 2026-05-04` → YAML to stdout / `--out file`.
- `tdx time week import <file>` → reads + saves; refuses to clobber unless `--force`.
- Round-trip test: `export | import` produces an identical draft.
- Stepping stone toward Phase E workspace mode.

### 12.6 Avoiding artifact clutter

- One canonical name (`default`); named alternates are advanced.
- Snapshots namespaced inside `<draftname>.snapshots/`; auto-prune.
- `list` defaults to per-profile + active drafts; archived/snapshot dirs not shown unless asked.

---

## 13. Safety, validation, constraints (consolidated)

| Concern | Behavior |
|---|---|
| Locked days | Per-cell `BlockerLocked`. Push proceeds for unblocked work; blockers reported. |
| Submitted week | Hard week-level blocker. Push refused unconditionally. |
| Approved week | Hardest. Push refused unconditionally. |
| Immutable / hard-to-change remote fields | `validate` probes; conflicts at push surface as per-action failures with field name + TD message. |
| Incomplete row mapping | `invalidCell` in `validate`; editor flags in gutter; push refuses. |
| Invalid time types | Caught at validate + preview. Editor cell-detail panel offers valid types for the target. |
| Duplicate / overlapping entries | Permitted but flagged in `validate` as warning. |
| Stale local copies | `stale` flag via watermark. Hash mismatch at push catches it; failure points to `refresh`. |
| Conflicting remote changes | Detected during `refresh`. Cell-level conflicts surfaced; draft enters `conflicted` state; push refuses. |
| Batch / sync limits | Per-push ceiling default 100 actions; configurable; suggests `--days` split. Network failures fail-fast unless `--continue-on-error`. |
| Permission-sensitive ops | Errors at push surface as per-action failures, no retry. (`tdx time auth probe` deferred to Phase F.) |
| Never destroy without consent | §8.8 gate matrix is canonical; no silent destructive paths. |

---

## 14. Recommended minimal slice (MVP — Phase A)

### 14.1 In MVP

| Capability | Command(s) |
|---|---|
| Pull a live week | `tdx time week pull <date>` (default name only; `--force`) |
| List drafts | `tdx time week list` |
| Show a draft | `tdx time week show <date> --draft` |
| Status | `tdx time week status <date>` |
| Edit (TUI) | `tdx time week edit <date>` |
| Diff vs remote | `tdx time week diff <date>` |
| Preview push | `tdx time week preview <date>` |
| Push | `tdx time week push <date> --yes [--allow-deletes]` |
| Delete draft | `tdx time week delete <date> --yes` |
| Auto-snapshots | `pre-pull-overwrite`, `pre-push`, `pre-delete` |
| MCP (7 tools) | `list_week_drafts`, `get_week_draft`, `pull_week_draft`, `update_week_draft`, `preview_push_week_draft`, `push_week_draft`, `delete_week_draft` |
| Templates-per-profile migration | one-shot prompt on first upgraded run |
| Docs | README, guide, walkthrough |

**SHOULD-tier (low cost, include if reasonable):** `tdx time week set`, `tdx time week note`, `tdx time week history` (read-only).

### 14.2 Deferred from MVP

Multiple named drafts, `new` / `copy` / `shift`, `refresh` / `rebase` / `reset`, per-cell metadata, `--web` for drafts, export/import, template ↔ draft interop, manual snapshot pinning / archive / prune, full standalone `validate`, workspace mode.

### 14.3 Why this slice

1. **Closes the headline workflow** — pull, edit, push end-to-end.
2. **Exercises every safety mechanism** — watermarks, hash protection, deletes, auto-snapshots, locked/submitted/approved blocking — in one shippable phase.
3. **Builds on what works** — reuses today's reconcile/apply engine (extended with `ActionDelete`), today's grid renderer, today's TUI editor, today's MCP confirm-gate pattern.
4. **Genuinely useful alone** — even without alternates / refresh, single-default-draft pull-edit-push is a real productivity win.
5. **Forward-compatible** — storage layout, engine extensions, MCP envelopes carry into Phases B–F unchanged.
6. **Folds in templates-per-profile** opportunistically — one user-visible storage change.

### 14.4 MVP-accepted constraints

- Only one draft per `(profile, weekStart)` in MVP. Schema supports `name` so Phase B is a CLI/UX change, not a model change.
- No merge — drift requires `pull --force` or `delete` + re-pull. Hash protection still prevents *unsafe* push.
- No editor UI for per-cell metadata — pulled cells with already-divergent metadata shown read-only with `°` glyph; editing requires hand-edit of YAML.

---

## 15. Phased plan

### Phase A — Local drafts MVP & per-profile storage

- **Code goals:** end-to-end pull → edit → push for a single default draft per week. Storage reorganization to per-profile. `ActionDelete` added to the reconcile engine. Auto-snapshots. 7 MCP tools.
- **User-visible deliverables:** §14.1.
- **Docs deliverables:**
  - `README.md` — Time Week Drafts command table; MCP additions; JSON schema additions; storage-layout note.
  - `docs/guide.md` — new "Week drafts" section (concepts, lifecycle, editor cheatsheet, push safety contract, worked examples for §9.1–§9.3, §9.5–§9.6); new "Storage layout" subsection.
  - `docs/manual-tests/phase-A-week-drafts-walkthrough.md` — runnable end-to-end against a real tenant.
  - Inline `--help` for every new command.
  - MCP tool descriptions including a shortened §10.15 recipe.
- **Open questions:**
  - Auto-snapshot retention default — proposed 10 unpinned per draft.
  - Templates-per-profile migration prompt: auto-yes when only one profile exists?
  - JSON schema name precision: `tdx.v1.weekDraft` vs `tdx.v1.timeWeekDraft`.
- **Risks:**
  - Reconcile-engine extension for deletes touches the safety-critical core; tests must cover `ActionDelete` × all three modes × all blocker types × hash-protection.
  - Templates-per-profile migration's prompt UX must be unambiguous.
  - Editor's "cleared pulled cell deletes the entry" UX is the most error-prone gesture; the pre-save confirm and `--allow-deletes` push gate must pull their weight.
- **Sequence:**
  1. Domain model + storage reorganization + templates migration script.
  2. Read-and-acquire CLI surface (`pull`, `list`, `show --draft`, `status`, `delete`).
  3. Engine: `ActionDelete` + watermark-aware reconcile.
  4. CLI `edit` (TUI), reusing existing editor module.
  5. CLI `diff`, `preview`, `push` with `--allow-deletes`.
  6. MCP — 7 tools mirroring CLI.
  7. Docs sweep + walkthrough.

### Phase B — Naming, alternates, refresh, snapshots polish

- **Code:** Q3's B+C surface in full. Multiple drafts per date. Merge primitive. Snapshot pinning, archive, prune. New + copy + shift.
- **User-visible:** `--name` everywhere; `new` (with `--from-template`, `--from-draft`, `--shift`); `copy`; `refresh / rebase / reset`; `snapshot --keep`; `restore`; `prune`; `archive`; ~5 more MCP tools.
- **Docs:** README additions; `docs/guide.md` "Refresh & rebase", "Multiple drafts per week", "Snapshots & history" subsections; phase-B walkthrough.
- **Open questions:** conflict resolution UX (editor vs separate `resolve` command); snapshot retention granularity (per-draft vs per-profile); `refresh` interactive vs flag-required.
- **Risks:** three-way merge is the most conceptually complex feature in this iteration; multi-draft `<date>[/<name>]` token must be consistent across all commands.

### Phase C — Editing depth, web editor, validation

- **Code:** the C-hybrid escape hatch for per-cell metadata; web editor parity for drafts; richer in-editor validation.
- **User-visible:** cell detail panel in TUI editor; `--web` for `tdx time week edit`; cell-level annotations for invalid type / locked day / submitted-week banners during editing; standalone `tdx time week validate`.
- **Docs:** "Per-cell overrides" subsection; web-editor walkthrough; validation reference; phase-C walkthrough.
- **Open questions:** auto-promote per-cell metadata on pull when row representation isn't faithful, vs. fold + warn?

### Phase D — Export/import and template ↔ draft interop

- **Code:** portability + bidirectional template/draft workflows.
- **User-visible:** `tdx time week export | import`; `tdx time week apply-template`; `tdx time template derive --from-draft`; `tdx time template compare --to-draft`; corresponding MCP tools; full `--against template|snapshot|draft` matrix in diff.
- **Docs:** "Export & import" subsection; expanded "Templates" section with §7.2 matrix; phase-D walkthrough.

### Phase E — Workspace mode (`.tdx/` in cwd)

- **Code:** Q4's deferred C — cwd-scoped drafts.
- **User-visible:** `tdx time week init`; workspace auto-detection; `tdx time week where`; `--workspace` / `--global` overrides.
- **Docs:** "Workspace mode" subsection; team-shared-weeks-in-a-repo walkthrough.
- **Open question:** does workspace mode see global drafts read-only, or fully isolated? Lean: isolated by default with `--include-global` listing flag.

### Phase F — Polish, observability, advanced safety

- **Code:** per-push action ceilings; `tdx time auth probe`; push timing reports; agent reasoning recipe expansions; perf and ergonomics sweep.
- **Docs:** "Performance & limits" subsection; final pass on every section based on Phase A–E real-world feedback.

### Cross-phase: documentation discipline

A phase is not done without docs. Each phase plan includes README, guide, walkthrough, `--help`, and MCP-description tasks as first-class items.

---

## 16. Planning artifacts and backlog scaffolding

### 16.1 Command map (consolidated, with phases)

See §3 — every command tagged with its phase.

### 16.2 Domain concepts

- **WeekDraft** — top-level dated, named editable artifact.
- **DraftRow** — `target + timeType + billable + description` plus 7 cells.
- **DraftCell** — atomic unit; `hours`, optional `sourceEntryID`, optional `perCellOverride`, computed `state`.
- **CellState** — enum: `untouched | edited | added | conflict | invalid` (no `tombstoned` per Reading X).
- **PerCellOverride** — `description? / timeTypeID? / billable?` (Phase C surface).
- **WeekDraftSnapshot** — immutable copy with op-tag, optional pin, optional note.
- **PullWatermark** — `pulledAt + remoteFingerprint + remoteStatus`.
- **DraftProvenance** — `kind: blank | pulled | from-template | from-draft` + sub-fields.
- **DraftSyncState** — `clean | dirty | stale | conflicted` + counts.
- **ActionDelete** — new kind in the reconcile engine alongside `Create | Update | Skip`.
- **DraftStore** — service-layer abstraction parallel to today's `tmplsvc.Store`, scoped per-profile.
- **DraftService** — service-layer surface for pull/preview/push/refresh/snapshot ops, parallel to `tmplsvc.Service`.

### 16.3 State model

A draft is at every moment exactly one combination of:
- **Lifecycle:** `nascent | live` (live = has a watermark).
- **Sync:** `clean | dirty | conflicted` (computed from cell states).
- **Drift:** `fresh | stale` (independent flag, computed from watermark vs current remote).

User-visible as the four-light model in §4.6 (`clean | dirty | stale | conflicted`); `nascent` is collapsed into "dirty" for reporting until first push.

### 16.4 Workflow list (canonical short names)

| Workflow | Phase first available | Audience |
|---|---|---|
| Pull-edit-push correction | A | human, agent |
| Stage next week from template | B | human, agent |
| Snapshot live week before edits | A | human, agent |
| Multiple alternative drafts | B | human |
| Refresh after remote drift | B | human, agent |
| Resolve conflicts in editor | B | human |
| Restore from auto-snapshot | A | human, agent |
| Per-cell description override | C | human, agent |
| Web-editor review | C | human (esp. non-CLI) |
| Export/import for sharing | D | human |
| Template ⇄ draft round-trip | D | human, agent |
| Workspace-mode team weeks | E | human |
| Long-running agent planning session | A | agent |

### 16.5 MVP scope (restated)

The Phase A slice in §14.1.

- **MUST:** pull, list, show --draft, status, edit (TUI), diff vs remote, preview, push (with `--allow-deletes`), delete, auto-snapshots, 7 MCP tools, templates-per-profile migration, full doc set.
- **SHOULD:** `tdx time week note`, `tdx time week set`, `tdx time week history` (read-only).
- **WON'T (this phase):** everything in §14.2.

### 16.6 Follow-on phases (one-line summaries)

- **B:** named alternates, refresh/merge, snapshots, new/copy/shift.
- **C:** per-cell metadata, web editor parity, full validate, drift-on-edit.
- **D:** export/import, template-draft interop, multi-target diff.
- **E:** workspace mode (`.tdx/` in cwd).
- **F:** polish, observability, advanced safety, push limits, auth probe.

### 16.7 Phase A backlog (issue-grain, ready to seed `writing-plans`)

**Storage & domain (foundation)**

1. Introduce `~/.config/tdx/profiles/<profile>/` paths layout in `internal/config/paths.go`; keep legacy template path readable until migration runs.
2. Introduce `domain.WeekDraft`, `DraftRow`, `DraftCell`, `CellState`, `DraftProvenance`, `PullWatermark`, `DraftSyncState`.
3. Introduce `internal/svc/draftsvc/store.go` with `Save / Load / List / Delete / Exists` parallel to `tmplsvc.Store`, scoped per-profile.
4. Introduce snapshot store: `<draftname>.snapshots/NNNN-<op>-<ts>.yaml` with op-tagging, pinning, bounded retention.
5. Templates-per-profile migration: detect legacy `~/.config/tdx/templates/`, prompt + move on first run, leave `.migrated` marker.

**Engine (the safety surface)**

6. Extend `domain.ActionKind` and `domain.ReconcileDiff` with `ActionDelete`.
7. Extend (or factor) reconcile to operate on a `WeekDraft`, computing actions for added/edited cells and `ActionDelete` for cleared pulled cells.
8. Extend `computeDiffHash` to include deletes and the pull watermark fingerprint.
9. Extend Apply to execute deletes via `timesvc.DeleteEntry`, with `--allow-deletes` honored at the service boundary.
10. Auto-snapshot helpers wired into all destructive ops.

**CLI**

11. `tdx time week pull <date>` (`--force`, dirty refusal, watermark capture).
12. `tdx time week list` (default + `--json`).
13. `tdx time week show <date> --draft [name]` and the existing-show banner.
14. `tdx time week status <date>` (sync state, cell counts, last pull/push, recommended action).
15. `tdx time week edit <date>` reusing TUI editor module, extended with §6.1 status bar and pre-save confirm for cleared pulled cells.
16. `tdx time week diff <date> --against remote`.
17. `tdx time week preview <date>` (annotated grid, `expectedDiffHash`).
18. `tdx time week push <date> --yes [--allow-deletes]` with hash protection.
19. `tdx time week delete <date> --yes` (auto-snapshot first).
20. `tdx time week set <date> <row>:<day>=<hours>` (SHOULD-tier).
21. `tdx time week note <date>` (SHOULD-tier).
22. `tdx time week history <date>` read-only (SHOULD-tier).

**MCP**

23. `list_week_drafts`.
24. `get_week_draft`.
25. `pull_week_draft` (mutating, confirm:true).
26. `update_week_draft` (mutating, confirm:true, `expectedModifiedAt`).
27. `preview_push_week_draft`.
28. `push_week_draft` (mutating, confirm:true, `expectedDiffHash`, `allowDeletes`).
29. `delete_week_draft` (mutating, confirm:true).

**Docs (each is a real task)**

30. `README.md` — week-draft command table; MCP additions; JSON schema additions; storage-layout note.
31. `docs/guide.md` — new "Week drafts" section (concepts, lifecycle, editor cheatsheet, push safety contract, worked examples).
32. `docs/guide.md` — new "Storage layout" subsection capturing per-profile reorganization.
33. `docs/manual-tests/phase-A-week-drafts-walkthrough.md` — runnable end-to-end against a real tenant.
34. Inline `--help` for every new command + MCP tool description text.

**Verification & release**

35. Reconcile-engine test pass: `ActionDelete` × all three modes × all blockers × hash-protection.
36. Manual walkthrough on `iainmoffat`'s real UFL tenant, signed off in the walkthrough doc.
37. README/release-note pass for the version bump.

(Expected to compress when written into a real plan; some items will fold during `writing-plans`.)
