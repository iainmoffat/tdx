# tdx — Phase B.1 Design Spec

**Date:** 2026-04-27
**Status:** Approved (brainstorming complete)
**Owner:** ipm
**Builds on:** Phase A (v0.4.0, shipped 2026-04-27)
**Splits Phase B from framework spec into:** B.1 (this doc) + B.2 (deferred)

---

## 0. Decisions log

| # | Decision | Implication |
|---|---|---|
| Q1 | **Split framework spec's Phase B into B.1 + B.2.** B.1 ships naming, alternates, acquisition verbs, and the snapshot-management surface. B.2 ships the three-way merge primitive (`refresh` / `rebase`) plus conflict-resolution UX. | B.1 is a CLI/UX layer on top of Phase A's foundation with zero new domain logic. B.2 is the conceptually densest work in the whole iteration and gets its own brainstorm. |
| Q2 | **Soft archive via `archived: true` flag** on the draft YAML. `list` skips archived drafts by default; `--archived` includes them. `unarchive` flips the flag. | No file motion, no rename collisions, no half-moved state. Matches the existing on-disk philosophy (everything is plain YAML you can `cat` / `git diff` / hand-edit). |
| Q3 | **Snapshot retention stays per-draft (Phase A default).** No change to the bounded-retention scope; the default of 10 unpinned per draft is fine. | Pruning, pinning, and retention all stay localized to each named draft. Storage is KB-scale; the worst-case math is megabytes. |
| Q4 | **Add `rename` to the verb set.** Not in the framework spec but a natural addition once alternate names exist. Auto-snapshots before renaming. | Closes the snapshot-loss gap of `copy + delete`. Small surface (~half a task) and composes cleanly with `archive`. |

---

## 1. Goal & scope

### What B.1 adds

Phase A made the `<date>[/<name>]` token parseable everywhere and made auto-snapshots fire before destructive ops, but **only `default` is usable as a name and snapshots have no user-visible management surface.** B.1 closes those gaps:

- **Multiple drafts per `(profile, weekStart)`** — `--name` actually works.
- **Acquisition verbs** — `new` (blank / from-template / from-draft) and `copy` (clone within or across weeks).
- **`reset`** — discard local edits + re-pull (the no-merge sibling of refresh).
- **Snapshot management** — `snapshot --keep`, `restore --snapshot N`, `prune`.
- **`archive` / `unarchive`** — soft hide via `archived: true`.
- **`rename`** — preserves snapshot history.
- **9 MCP tool additions** (8 mutating + 1 read) plus one filter extension on the existing `list_week_drafts`.
- **Docs:** new `guide.md` subsections + a Phase B.1 walkthrough.

### What B.1 explicitly does NOT add (deferred to B.2)

- `refresh` / `rebase` — the three-way merge engine.
- `CellConflict` / `SyncConflicted` state production (declared as enums in Phase A; never emitted yet).
- In-editor or `resolve`-command conflict-resolution UX.
- The `refresh_week_draft` MCP tool.

These get their own brainstorming pass when B.1 has shipped and we have real-world miles on the alternates / snapshot UX.

---

## 2. Domain model delta

A single new field on `WeekDraft`:

```go
type WeekDraft struct {
    // ... all Phase A fields unchanged ...
    Archived bool `yaml:"archived,omitempty" json:"archived,omitempty"`
}
```

That's it. Cells are already dimensionless `(weekday, hours)` so `copy 2026-04-27 2026-05-04` Just Works without any cell-rewriting. `--shift Nd` is sugar over `copy <src> <src+Nd>`.

No changes to `DraftSyncState` or `CellState` enums (their `conflict` variants remain declared but unproduced; B.2 will start emitting them).

---

## 3. Command surface

All under `tdx time week`. Verbs marked **NEW** are introduced by B.1; **extended** verbs already exist from Phase A and gain new behavior.

| Verb | Status | Sketch |
|---|---|---|
| `pull <date>` | extended | `--name <slug>` actually creates alternates; existing dirty-refusal logic unchanged |
| `new <date>` | NEW | `--name`, `--from-template <name>`, `--from-draft <date>[/<name>]`, `--shift ±N` (only with `--from-draft`) |
| `copy <src> <dst>` | NEW | `<src>` and `<dst>` are full `<date>[/<name>]` refs; cells are dimensionless so cross-week copy works without extra logic |
| `rename <date>[/<old>] <new>` | NEW | renames YAML + `.pulled.yaml` + `.snapshots/`; auto-snapshots before; refuses on collision |
| `reset <date>[/<name>]` | NEW | `--yes`-gated; auto-snapshots; deletes local + re-pulls fresh |
| `archive <date>[/<name>]` | NEW | sets `archived: true`; reversible via `unarchive` |
| `unarchive <date>[/<name>]` | NEW | clears `archived: true` |
| `snapshot <date>[/<name>]` | NEW | manual snapshot; `--keep` (pin), `--note <s>` |
| `restore <date>[/<name>] --snapshot N` | NEW | `--yes`-gated; auto-snapshots current first |
| `prune <date>[/<name>]` | NEW | drops unpinned snapshots beyond retention; `--all`, `--older-than 30d` |
| `list` | extended | `--archived` shows hidden drafts; default output groups alternates under same date |
| `history <date>[/<name>]` | extended | adds `PINNED` column to text output (data already in store) |

`refresh` / `rebase` are deliberately absent — they belong to B.2.

---

## 4. `list` UX with alternates

Today's `list` shows one row per draft. With alternates that gets noisy. Two design tweaks:

- **Sort:** `weekStart desc, name asc` (already implemented in Phase A's `Store.List`).
- **Alternate-name visual grouping:** when a date has multiple drafts, the second+ rows visually align under the first (the date column is blank for follow-ups). Compact text view; JSON output unchanged.
- **`--archived`** flag toggles inclusion; default still hides archived.

Example text output:

```
WEEK         NAME       STATE     HOURS  PULLED
2026-05-04   default    dirty 3    18.0  2026-04-27T19:17Z
             pristine   clean      20.0  2026-04-27T19:17Z
2026-04-12   default    clean      20.0  2026-04-27T19:17Z
```

JSON output unchanged from Phase A's `tdx.v1.weekDraftList` — it's an array; consumers handle grouping.

---

## 5. Acquisition verbs

### `new <date>`

Creates a draft. Three modes:

- `tdx time week new 2026-05-04` — blank draft (no rows). Provenance kind: `blank`. Pushing it produces only `Create` actions for whatever rows the user adds.
- `tdx time week new 2026-05-04 --from-template canonical` — seeds rows from the named template. Provenance: `from-template <name>`.
- `tdx time week new 2026-05-04 --from-draft 2026-04-27` — clones rows from another draft. Provenance: `from-draft <ref>`.
- `tdx time week new 2026-05-04 --from-draft 2026-04-27 --shift 7d` — shorthand for "use 2026-04-27's content for 2026-05-04." Since cells are dimensionless, `--shift` is purely cosmetic notation; computed as a time-delta from the source's weekStart to validate sanity.

`--name <slug>` works on all variants.

`--shift` requires `--from-draft` (no semantic meaning with `--from-template` or blank). Validation rejects invalid combinations.

### `copy <src> <dst>`

Both refs are full `<date>[/<name>]` tokens. Behavior:

- Loads the source draft.
- Constructs a new draft for `<dst>` with the source's rows (copied), provenance recorded as `from-draft <src> [shifted-by Nd]`.
- Saves alongside source's `.pulled.yaml` sibling **only if `<dst>` is the same date as `<src>`** (otherwise the watermark is meaningless on a different week).
- Refuses if `<dst>` already exists; user must `delete <dst>` first or pick a different name.

### `reset <date>[/<name>]`

Discard local edits + re-pull. Requires `--yes`. Auto-snapshots the existing draft as `pre-reset`. Deletes the draft + its `.pulled.yaml`, then runs `Pull(force=false)` on the same `(date, name)`. The `.snapshots/` history is retained.

### Why `reset` instead of `pull --force`

Both achieve "drop local edits, get a fresh pull." `reset` is the explicit verb; `pull --force` is the imperative that happens to do the same thing. They route to the same service-layer flow but `reset` is what shows up in `--help` and what we recommend in `status` output.

---

## 6. `archive` / `unarchive`

### `archive <date>[/<name>]`

Sets `archived: true` on the draft YAML. Saves. No file motion. Idempotent (archiving an already-archived draft is a no-op with a one-line confirmation).

### `unarchive <date>[/<name>]`

Symmetric inverse. Clears `archived: true`.

### Behavior changes

- `tdx time week list` — filters out drafts with `archived: true` by default. `--archived` includes them. JSON consumers see all drafts (filter is text-mode only by default; `--archived=false` could be added later if needed).
- `tdx time week show <date> --draft` — works on archived drafts identically; archived doesn't mean "read-only," just "hidden by default."
- `tdx time week status` — surfaces archive state in the human output: `Archived: yes (2026-04-15)` if true.
- `tdx time week pull` — refuses to overwrite an archived draft unless `--force`, with a "this draft is archived; unarchive first or use --force" error.

### Why a flag instead of a separate dir

Per Q2: zero file-motion bugs, full `cat` / `git diff` parity with active drafts, no rename collisions on unarchive. Disk clutter argument is theoretical at tdx's data scale.

---

## 7. `rename` semantics

Identity of a draft is `(profile, weekStart, name)`. Renaming changes only the third component.

### What renames

```
profiles/<p>/weeks/<date>/<old>.yaml          → <new>.yaml
profiles/<p>/weeks/<date>/<old>.pulled.yaml   → <new>.pulled.yaml  (if exists)
profiles/<p>/weeks/<date>/<old>.snapshots/    → <new>.snapshots/   (if exists)
```

Plus: the `name:` field inside the YAML is updated (the YAML's `name` and its filename must stay in sync).

### Safety

- **Pre-flight check:** stats all three target paths; refuses if any of them exist.
- **Auto-snapshot first:** the original draft gets a `pre-rename` snapshot so `restore` can roll back the rename if regretted.
- **Atomic-ish ordering:** YAML first (the canonical artifact), then `.pulled.yaml`, then `.snapshots/`. Failure between steps leaves the draft loadable from the new YAML; the orphaned `.pulled.yaml` or `.snapshots/` directory becomes a benign leftover (`tdx time week list` and `history` will tolerate them).
- **No cross-date rename.** `tdx time week rename 2026-04-27/foo bar` works (changes `foo` → `bar` within `2026-04-27`); changing the date is `copy` + `delete`. This keeps the verb crisp.

---

## 8. Snapshot management UX

Phase A's `SnapshotStore` already implements take, list, load, pin, prune. B.1 surfaces them as CLI verbs.

### `snapshot <date>[/<name>] [--keep] [--note <s>]`

Creates a manual snapshot.

- Op tag: `manual`.
- `--keep` flag → pinned. Pinned snapshots survive auto-prune.
- `--note <s>` → stored in the filename suffix per Phase A's existing convention.

### `restore <date>[/<name>] --snapshot N --yes`

Replaces the current draft with snapshot `N`'s contents. Auto-snapshots the existing draft as `pre-restore` first. Snapshot N's pinned status is preserved.

### `prune <date>[/<name>] [--all] [--older-than DUR] --yes`

Drops snapshots beyond retention.

- Default (no flags): removes unpinned snapshots beyond the per-draft retention cap (10).
- `--all`: removes all unpinned snapshots regardless of cap.
- `--older-than 30d`: removes unpinned snapshots older than the duration. Composes with the cap (whichever is more aggressive).

### `history <date>[/<name>]`

Existing command from Phase A. B.1 extends the text output to render the existing `Pinned` field as a column:

```
SEQ   OP            TAKEN                 PINNED  NOTE
1     pre-pull      2026-04-27 13:12:14
2     manual        2026-04-27 14:45:00   yes     baseline-before-experiment
3     pre-push      2026-04-27 15:02:11
```

JSON output unchanged (the `pinned` field already exists in Phase A's `tdx.v1.weekDraftSnapshotList`).

---

## 9. MCP additions

8 mutating tools (all require `confirm: true`). One read-only tool addition. One filter extension on the existing `list_week_drafts`.

| Tool | Purpose |
|---|---|
| `create_week_draft` | Mutating. `from: blank | template:<n> | draft:<date>[/<n>]`. Optional `name`, `shiftDays`. |
| `copy_week_draft` | Mutating. `srcRef`, `dstRef`. |
| `rename_week_draft` | Mutating. Auto-snapshots first. |
| `reset_week_draft` | Mutating. `--yes`-equivalent gate. |
| `archive_week_draft` | Mutating. Sets `archived: true`. |
| `unarchive_week_draft` | Mutating. Clears `archived: true`. |
| `snapshot_week_draft` | Mutating. Take a manual snapshot; `keep` and `note` optional. |
| `restore_week_draft_snapshot` | Mutating. Restore by sequence. Auto-snapshots first. |
| `prune_week_draft_snapshots` | Mutating. Drop unpinned beyond retention. |

`list_week_drafts` (existing) gains an `archived: bool` filter argument.

`list_week_draft_snapshots` (which Phase A's CLI `history` reads via `Snapshots().List()`) becomes a first-class read-only MCP tool here. Read-only (no confirm needed).

Total MCP tool count: 27 → ~36.

---

## 10. JSON schemas

Additive only; no breaking changes to existing envelopes.

- `tdx.v1.weekDraftCopyResult`
- `tdx.v1.weekDraftRenameResult`
- `tdx.v1.weekDraftArchiveResult` (used for both archive and unarchive)
- `tdx.v1.weekDraftSnapshot` (single-snapshot return for `snapshot --json` and `restore --json`)
- `tdx.v1.weekDraftSnapshotList` — already from Phase A; unchanged

The existing `tdx.v1.weekDraft` envelope gains the `archived` field; consumers that don't read it remain compatible.

---

## 11. Docs deliverables

Per the Phase A documentation discipline ("a phase is not done without docs"):

1. **`README.md`** — extend the Time Week Drafts table with the 7 new commands. Expand MCP tool table.
2. **`docs/guide.md`** — three new subsections in the "Week drafts" section:
   - "Multiple drafts per week" — alternate names, `--name`, `list` grouping, `copy --as`, `rename`.
   - "Snapshots & history" — `snapshot --keep`, `restore`, `prune`, retention semantics, pinning.
   - "Archive & unarchive" — soft-archive behavior, the `--archived` filter.
3. **`docs/manual-tests/phase-B1-week-drafts-walkthrough.md`** — runnable end-to-end against the live UFL tenant. Exercises:
   - Create alternate via `pull --name pristine`.
   - `new --from-draft` with `--shift`.
   - `copy` across weeks.
   - `rename` an alternate; verify snapshots followed.
   - Archive/unarchive cycle.
   - `snapshot --keep`, `restore`, `prune`.
   - MCP smoke test exercising the 9 new/extended tools.
4. **Inline `--help`** for every new command and flag.
5. **MCP tool descriptions** carrying enough behavior detail for an agent to use them correctly without reading the guide.

---

## 12. Recommended slice + sequence

B.1 is one phase, ~22 tasks, ~30 commits. Suggested order:

1. Domain delta — add `Archived` field + tests.
2. `pull --name` plumbing — verify alternate-name pulls work end-to-end.
3. `new` — blank, `--from-template`, `--from-draft`, `--shift`.
4. `copy <src> <dst>`.
5. `rename` — including auto-snapshot pre-flight.
6. `reset`.
7. `archive` / `unarchive` — domain + CLI.
8. `snapshot` (manual) + `restore` + `prune`.
9. `list` UX — alternate-name grouping + `--archived` flag.
10. `history` text output — `PINNED` column.
11. MCP — 8 mutating tools + `list_week_draft_snapshots` read tool + `archived` filter on `list_week_drafts`.
12. Docs — README, guide, walkthrough, inline `--help`.
13. Verification — full-suite + tag v0.5.0 + release.

---

## 13. Open questions and risks

### Open questions

- **Default retention number**: 10 unpinned per draft from Phase A. Stays at 10 unless walkthrough surfaces a need to bump.
- **`copy` cross-week + `.pulled.yaml`**: spec'd as "copy `.pulled.yaml` only when src and dst are same date." Verify this matches what users expect on the walkthrough.
- **`archive` and `pull`**: the rule "archived drafts refuse pull-overwrite without --force" is consistent with the existing dirty-pull guard but is one extra rule; confirm UX during walkthrough.

### Risks

- **`rename` failure mode** — if step 2 of 3 fails after step 1 succeeded, we have a draft loadable from `<new>.yaml` with an orphaned `<old>.pulled.yaml`. Mitigation: pre-flight check + treat orphans as benign in `list`/`history`. Document in the walkthrough.
- **Multi-draft `<date>[/<name>]` token consistency** — every command needs to accept the token correctly. Phase A's `ParseDraftRef` is shared; B.1 just stresses every call site. CLI tests should cover both forms (with and without `/<name>`) for every new command.
- **`list` text grouping** — easy to break visually with edge cases (very long names, zero-row drafts). Walkthrough should include at least one cluttered-list scenario.

---

## 14. After B.1: what B.2 will need

Recorded here so the B.1 plan doesn't paint into a corner:

- The three-way merge engine consumes the `at-pull-time` cells (already saved as `.pulled.yaml` siblings since Phase A) plus the current local + current remote. B.1's new rows on existing drafts (`copy`, `new --from-draft`) need to make sure the `.pulled.yaml` sibling is created or omitted correctly so refresh doesn't think untouched cells are conflicts.
- B.1's `reset` is the no-merge version of refresh; refresh's design must be a strict superset.
- The `CellConflict` and `SyncConflicted` enum values declared in Phase A's domain code remain unproduced after B.1. B.2 will start emitting them.

---

## 15. Summary

B.1 ships in one focused PR: alternates, acquisition verbs, snapshot management, archive, rename. Zero new domain logic; pure CLI/UX surface on top of Phase A's foundation. ~22 tasks, ~30 commits. Tag v0.5.0 on completion. B.2 (refresh/merge) follows in a separate brainstorm + spec + plan cycle.
