# Phase A — Manual Week Drafts Walkthrough

This document exercises the Phase A week-draft capability set against a real
TeamDynamix tenant. Run steps in order. Substitute `2026-04-20` (Sunday) with
any recent Sunday that has time entries in your tenant.

## Prerequisites

- A built `tdx` binary (`go build ./cmd/tdx`).
- Phase 1 and Phase 2 walkthroughs already passed on this machine.
- At least one week of logged time in the TD tenant to pull.

---

## Step 1: Setup — verify auth and choose a target week

Goal: confirm auth is live and identify a week with entries to work against.

```bash
./tdx auth status
./tdx time week show 2026-04-20
```

Verify: `auth status` shows `state: authenticated`. The week grid has at
least one non-zero cell. If the week is empty, pick a different Sunday.

---

## Step 2: Templates-per-profile migration

Goal: confirm the one-time migration fires on first Phase A use.

```bash
./tdx time week pull 2026-04-20
```

If this is the first Phase A run on this profile you will see:

```text
Migrating templates to per-profile storage… done.
```

On subsequent runs the line is absent. Either is correct.

Verify:

```bash
ls ~/.config/tdx/profiles/default/templates/
```

Expected: directory exists (may be empty).

---

## Step 3: Pull a recent week

Goal: create a local draft from live TD data.

```bash
./tdx time week pull 2026-04-20
```

```text
pulled week 2026-04-20 → ~/.config/tdx/profiles/default/drafts/2026-04-20.yaml
snapshot: pre-pull saved
```

Verify: `~/.config/tdx/profiles/default/drafts/2026-04-20.yaml` exists with
a `week:` header, `rows:` list, and `sourceHash:` field.

Notes: `pull` overwrites any existing draft after taking a `pre-pull`
snapshot. Do not pull a week you have already edited locally unless you
intend to discard local changes.

---

## Step 4: List and show

Goal: confirm the draft appears in list output and renders as a grid.

```bash
./tdx time week list
```

```text
WEEK        STATUS   ROWS
2026-04-20  clean    3
```

```bash
./tdx time week show 2026-04-20 --draft
```

Verify: grid renders from the local draft, cells match what was pulled.

---

## Step 5: Status — verify clean state

Goal: confirm the draft is unmodified.

```bash
./tdx time week status 2026-04-20
```

Verify: output shows `status: clean` and `hash: … (matches remote)`.

---

## Step 6: Edit a cell

Goal: modify a cell value and confirm the draft becomes dirty.

```bash
./tdx time week edit 2026-04-20
```

In your `$EDITOR`, change one cell value (e.g. `mon: 2.0` → `mon: 3.0`).
Save and quit.

```bash
./tdx time week status 2026-04-20
```

Verify: `status: dirty`.

---

## Step 7: Clear a pulled cell to schedule deletion

Goal: set a cell that has a `sourceEntryID` to 0 so the push deletes the
corresponding live entry.

```bash
./tdx time week edit 2026-04-20
```

Find a cell with `sourceEntryID:` set and change its hour value to `0`.
Save and quit.

Verify:

```bash
grep -A3 "sourceEntryID" ~/.config/tdx/profiles/default/drafts/2026-04-20.yaml | head -8
```

Expected: the cell adjacent to that `sourceEntryID` shows `0`.

Notes: a cell without a `sourceEntryID` zeroed out is a no-op (nothing to
delete). Only entries with a `sourceEntryID` and `0` hours trigger a delete.

---

## Step 8: Diff

Goal: review planned changes before pushing.

```bash
./tdx time week diff 2026-04-20
```

```text
row-01  mon   UPDATE  2.0 → 3.0   (#10001 "Project Alpha" / Regular)
row-02  wed   DELETE  1.5 → 0     (#10002 "Project Beta"  / Regular)
```

Verify: one UPDATE for Step 6, one DELETE for Step 7.

---

## Step 9: Preview

Goal: dry-run summary with a stable diff hash.

```bash
./tdx time week preview 2026-04-20
```

```text
week:     2026-04-20
actions:  1 update, 1 delete
hash:     d4e5f6a7
```

Verify: action counts match `diff`. Hash is stable across repeated calls
with no intervening edits.

---

## Step 10: Push without `--allow-deletes`

Goal: confirm the safety guard fires when deletes are present.

```bash
./tdx time week push 2026-04-20 --yes
```

```text
error: push plan contains 1 delete(s); re-run with --allow-deletes to confirm
```

Verify: non-zero exit, nothing written to TD.

---

## Step 11: Push with `--allow-deletes`

Goal: apply the draft to the live tenant.

```bash
./tdx time week push 2026-04-20 --yes --allow-deletes
```

```text
snapshot: pre-push saved
snapshot: pre-delete saved
pushing week 2026-04-20…
  UPDATE  row-01 mon  ✓
  DELETE  row-02 wed  ✓
push complete: 1 updated, 1 deleted
draft status: clean
```

Verify: both actions show `✓` and final line reads `draft status: clean`.

---

## Step 12: Verify in TD web UI

Goal: confirm live entries match the pushed draft.

Open `https://ufl.teamdynamix.com/` and navigate to your timesheet for the
week of 2026-04-20.

Verify:
- The updated cell shows the new hour value (3.0).
- The deleted entry is gone.

Notes: TD may cache the view briefly; hard-refresh if old values appear.

---

## Step 13: Auto-snapshot recovery

Goal: confirm snapshots are captured before destructive ops and the
`history` command surfaces them.

Snapshots fire only before destructive operations:

| Trigger | Op tag |
|---|---|
| `pull --force` overwriting a dirty draft | `pre-pull` |
| `push --yes` (taken before any writes) | `pre-push` |
| `delete --yes` | `pre-delete` |

A fresh pull (no existing draft) does not produce a snapshot. The first
snapshot you'll see in this walkthrough is the `pre-push` from Step 11,
followed by `pre-delete` after Step 16.

```bash
ls ~/.config/tdx/profiles/default/weeks/2026-04-20/default.snapshots/
```

Expected (after Step 11 push): at least one `NNNN-pre-push-<ts>.yaml`.

```bash
./tdx time week history 2026-04-20
```

```text
SEQ   OP            TAKEN                 PINNED  NOTE
1     pre-push      2026-04-27 10:05:00
```

Verify: at least one entry with the matching op tag and a recent timestamp.
After Step 16 (delete), a `pre-delete` row joins the list.

---

## Step 14: Non-interactive cell write

Goal: set a cell value without opening the editor.

```bash
./tdx time week set 2026-04-20 row-01:thu=4
```

Verify: output shows `set row-01 thu → 4.0`. `grep thu` on the draft YAML
shows `thu: 4`. Status becomes dirty.

Notes: format is `row-label:day=hours`. Day is the three-letter abbreviation
(sun, mon, tue, wed, thu, fri, sat).

---

## Step 15: Notes

Goal: append a free-text note to the draft.

```bash
./tdx time week note 2026-04-20 --append "Pre-conference week"
```

Verify: `grep notes` on the draft YAML shows a `notes:` field containing
`"Pre-conference week"`. Use `--set` instead of `--append` to overwrite.

---

## Step 16: Delete the draft

Goal: remove the local draft file while retaining snapshots.

```bash
./tdx time week delete 2026-04-20 --yes
```

Verify: `2026-04-20.yaml` is absent from `drafts/` and `tdx time week list`
no longer shows the week. `ls snapshots/2026-04-20/` still shows the
snapshot files from earlier steps.

---

## Step 17: MCP smoke test

Goal: exercise the MCP server tools against the draft workflow.

Pull a fresh draft first if Step 16 deleted the only one:

```bash
./tdx time week pull 2026-04-20
```

Start the server:

```bash
./tdx mcp serve
```

In a second terminal (or via an MCP client), call each tool:

**list_week_drafts**

```json
{ "tool": "list_week_drafts", "arguments": {} }
```

Expected: JSON array containing the `2026-04-20` draft.

**pull_week_draft** (with confirm)

```json
{ "tool": "pull_week_draft", "arguments": { "week": "2026-04-20", "confirm": true } }
```

Expected: success; `confirm: false` returns a preview without writing.

**get_week_draft**

```json
{ "tool": "get_week_draft", "arguments": { "week": "2026-04-20" } }
```

Expected: JSON representation of the draft YAML.

**preview_push_week_draft**

```json
{ "tool": "preview_push_week_draft", "arguments": { "week": "2026-04-20" } }
```

Expected: `updates`, `deletes`, `hash` fields in the response.

Stop the server with Ctrl-C.

---

## Step 18: Stale draft — hash mismatch

Goal: confirm an out-of-date draft is refused at push time.

1. Pull the week:

   ```bash
   ./tdx time week pull 2026-04-20
   ```

2. Edit locally:

   ```bash
   ./tdx time week set 2026-04-20 row-01:fri=1
   ```

3. In the TD web UI, add or change an entry for the same week (e.g. log
   0.5 hours on any ticket for Friday). This advances the remote hash.

4. Attempt to push:

   ```bash
   ./tdx time week push 2026-04-20 --yes
   ```

   ```text
   error: remote week has changed since last pull (hash mismatch)
   local hash:  a1b2c3d4
   remote hash: e5f6a7b8
   run 'tdx time week pull 2026-04-20' to refresh, or
       'tdx time week diff 2026-04-20' to review divergence
   ```

Verify: non-zero exit, no entries written to TD.

---

## Step 19: Sign-off

Fill in after completing all steps.

```
Date run:     2026-04-27
Tenant:       https://ufl.teamdynamix.com/
tdx version:  0.1.0-dev (branch phase-A-week-drafts @ bf2f78d)
Tester:       Claude (subagent-driven dispatcher) on behalf of ipm
Week used:    2026-04-12 (open, 24 entries, 20h, UFIT Administration row only)

Passed steps: 1 2 3 4 5 6 7 8 9 10 13 14 15 16 17 18
Variations:   - Step 6 / 7 used `tdx time week set` instead of the interactive
                editor (no TTY in subagent dispatch).
              - Step 11 was exercised against live data with a tiny update
                (Mon 4.0 -> 4.5h on entry #22085) and immediately restored
                (4.5 -> 4.0h on the same entry). The --allow-deletes path
                with actual delete actions was NOT exercised against live
                data (would change entry IDs); the refusal gate was
                verified in Step 10, and the delete code path itself is
                covered by TestApply_AllowDeletesGate (mocked tsvc).
              - Step 12 (TD web UI verification) was substituted with
                `tdx time week show 2026-04-12 --json | jq .entries[] |
                select(.id == 22085)` to confirm the remote state changed
                and was restored.
              - Step 18 was simulated via `--expected-diff-hash deadbeef…`
                rather than mid-test remote tampering; same code path,
                no remote write needed.

Failed steps: none

Bugs caught and fixed during the walkthrough:
  - tmplsvc per-profile regression  -> commit d99d419
  - show --json banner corruption  -> commit 9ab5d1c
  - Zero-entry placeholders + diff before-value polish  -> commit db94438
  - Walkthrough doc snapshot expectation (this file)  -> commit bf2f78d

Notes: live-tenant entry #22085 was modified twice during Step 11 and is
back to its original state (240 minutes / 4.0h on Mon 2026-04-13).
Entry #22089 (Friday) was set to 0h locally during Step 18 but no push
occurred (hash refused), so remote was untouched.
```
