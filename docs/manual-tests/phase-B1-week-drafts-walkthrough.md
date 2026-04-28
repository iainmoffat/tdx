# Phase B.1 — Manual Week Drafts Walkthrough

This document exercises the Phase B.1 week-draft capability set against a real
TeamDynamix tenant. Run steps in order. Uses `2026-04-12` (a past Sunday with
known entries) as the primary target week; `2026-05-31` for experiments that
must not touch live data.

## Prerequisites

- A built `tdx` binary (`go build ./cmd/tdx`).
- Phase A walkthrough already passed on this machine.
- At least one week of logged time in the TD tenant to pull.

---

## Step 1: Setup — verify auth and choose a target week

Goal: confirm auth is live before running any draft commands.

```bash
./tdx auth status
./tdx time week show 2026-04-12
```

Verify: `auth status` shows `state: authenticated`. The week grid has at
least one non-zero cell. If the week is empty, substitute a different recent
Sunday throughout this walkthrough.

---

## Step 2: Pull default draft and an alternate

Goal: create two local drafts for the same week under different names.

```bash
./tdx time week pull 2026-04-12
./tdx time week pull 2026-04-12 --name pristine
```

```text
pulled week 2026-04-12 → …/drafts/2026-04-12/default.yaml
pulled week 2026-04-12 → …/drafts/2026-04-12/pristine.yaml
```

### Verify

```bash
ls ~/.config/tdx/profiles/default/weeks/2026-04-12/
```

Expected: `default.yaml` and `pristine.yaml` both present.

### Notes

The `default` name is used when `--name` is omitted. Both drafts start as
clean copies of the same live week.

---

## Step 3: List shows alternates grouped

Goal: confirm both drafts appear under a single date entry.

```bash
./tdx time week list
```

```text
WEEK        NAME      STATUS   ROWS
2026-04-12  default   clean    3
            pristine  clean    3
```

### Verify

The `WEEK` column is blank for the second (and any subsequent) draft sharing
the same date — only the first row carries the date value. Row count may
differ from the example; the key check is both names appear.

---

## Step 4: Edit default with `set`, leave pristine untouched

Goal: make the `default` draft dirty while `pristine` stays clean.

```bash
./tdx time week set 2026-04-12 row-01:mon=6
./tdx time week status 2026-04-12
./tdx time week status 2026-04-12/pristine
```

### Verify

- `status 2026-04-12` (the default draft) shows `status: dirty`.
- `status 2026-04-12/pristine` shows `status: clean`.

### Notes

The `<date>/<name>` syntax selects a named draft. `<date>` alone always
resolves to `default`.

---

## Step 5: Create a blank new draft

Goal: create a draft with no rows pulled from TD.

```bash
./tdx time week new 2026-05-31 --name experiment
./tdx time week list
```

### Verify

- `list` shows `2026-05-31  experiment  blank  0` (or similar).
- No pull is issued to TD; the draft exists with an empty `rows:` list.

---

## Step 6: Clone a draft with `new --from-draft`

Goal: create a new draft by copying rows from an existing draft, shifted one
week forward.

```bash
./tdx time week new 2026-05-10 --from-draft 2026-04-12/default --shift 7d
./tdx time week show 2026-05-10 --draft
```

### Verify

- `show --draft` renders the same row labels as `2026-04-12/default`.
- `sourceEntryID` fields in the YAML are absent or cleared (new rows have no
  remote entries yet).
- Date fields on each cell are shifted by 7 days relative to the source week.

### Notes

`--shift` accepts `Nd` (days). Use it to advance a pulled week by one period
without re-pulling from TD.

---

## Step 7: Copy a draft

Goal: duplicate an existing draft within the same date.

```bash
./tdx time week copy 2026-04-12/default 2026-04-12/copy-test
./tdx time week list
```

### Verify

- `list` shows `copy-test` under `2026-04-12` with the same row count as
  `default`.
- The snapshot history of the source is not copied; `copy-test` starts with a
  clean snapshot log containing only the `post-copy` snapshot.

```bash
./tdx time week history 2026-04-12/copy-test
```

Expected: one entry, op `post-copy` (or similar), recent timestamp.

---

## Step 8: Rename a draft

Goal: rename `copy-test` to `trial` and confirm snapshot history follows.

```bash
./tdx time week rename 2026-04-12/copy-test trial
./tdx time week list
```

### Verify

- `list` shows `trial` under `2026-04-12`; `copy-test` is gone.

```bash
./tdx time week history 2026-04-12/trial
```

Expected: the same `post-copy` snapshot that existed under `copy-test` now
appears under `trial`. Rename does not break snapshot continuity.

---

## Step 9: Manual snapshot with `--keep`

Goal: pin a snapshot before a risky edit so it survives automatic pruning.

```bash
./tdx time week snapshot 2026-04-12/default --keep --note "before risky edit"
./tdx time week history 2026-04-12/default
```

```text
SEQ   OP        TAKEN                 PINNED  NOTE
1     manual    2026-04-27 10:00:00   yes     before risky edit
```

### Verify

- The snapshot appears in `history` with `PINNED: yes`.
- Note the `SEQ` number — you will need it in Step 10.

---

## Step 10: Edit, then restore from snapshot

Goal: make a bad edit, then restore to the pinned snapshot from Step 9.

```bash
./tdx time week set 2026-04-12 row-01:fri=99
./tdx time week status 2026-04-12
```

Verify: `status` shows `dirty`.

Now restore. Replace `N` with the SEQ from Step 9:

```bash
./tdx time week restore 2026-04-12 --snapshot N --yes
./tdx time week status 2026-04-12
./tdx time week history 2026-04-12/default
```

### Verify

- `status` returns to `clean`.
- `fri` value in the draft is back to the pre-edit value (not `99`).
- `history` shows a new `pre-restore` snapshot was taken automatically before
  the restore was applied.

---

## Step 11: Prune snapshots

Goal: exercise both prune paths (age filter and retention cap).

**Age filter (expect zero pruned — nothing is 30 days old yet):**

```bash
./tdx time week prune 2026-04-12/default --older-than 30d --yes
```

```text
Pruned 0 snapshot(s) (0 pinned skipped).
```

**Retention-cap path (prunes non-pinned beyond cap, sparing pinned):**

```bash
./tdx time week prune 2026-04-12/default --yes
```

### Verify

- First command reports `Pruned 0`.
- Second command may prune auto-snapshots beyond the configured retention cap;
  the pinned snapshot from Step 9 must survive.

```bash
./tdx time week history 2026-04-12/default
```

Expected: pinned row (`before risky edit`) still present.

---

## Step 12: Archive and unarchive

Goal: hide a draft from default `list` and restore it.

```bash
./tdx time week archive 2026-04-12/trial
./tdx time week list
```

### Verify

- `trial` is absent from the default listing.

```bash
./tdx time week list --archived
```

- `trial` appears with an `archived` indicator.

```bash
./tdx time week unarchive 2026-04-12/trial
./tdx time week list
```

- `trial` is visible again in the default listing.

---

## Step 13: Reset a draft

Goal: discard local edits by re-pulling from TD, with a safety snapshot first.

```bash
./tdx time week reset 2026-04-12/default --yes
```

```text
snapshot: pre-reset saved
pulling 2026-04-12 from TD…
draft reset to remote state.
```

```bash
./tdx time week status 2026-04-12
./tdx time week history 2026-04-12/default
```

### Verify

- `status` shows `clean`.
- `history` contains a `pre-reset` snapshot taken immediately before the pull.

### Notes

`reset` is equivalent to `pull --force` but explicit about the snapshot
semantics: a safety snapshot is always taken regardless of whether the draft
was clean or dirty.

---

## Step 14: MCP smoke test

Goal: exercise Phase B.1 MCP tools over JSON-RPC stdio.

Pull a fresh draft if needed, then start the server:

```bash
./tdx time week pull 2026-04-12
./tdx mcp serve
```

In a second terminal (or MCP client), call each tool in turn:

**list_week_drafts (including archived)**

```json
{ "tool": "list_week_drafts", "arguments": { "archived": true } }
```

Expected: JSON array containing `default`, `pristine`, `experiment`,
`2026-05-10` (unnamed or `default`), and `trial` entries. Archived drafts
appear if `archived: true`.

**create_week_draft (blank)**

```json
{ "tool": "create_week_draft", "arguments": { "week": "2026-06-01", "name": "mcp-blank", "from": "blank", "confirm": true } }
```

Expected: success; `confirm: false` returns a preview without writing.

**copy_week_draft**

```json
{ "tool": "copy_week_draft", "arguments": { "src": "2026-04-12/default", "dst": "2026-04-12/mcp-copy" } }
```

Expected: success; `list_week_drafts` now shows `mcp-copy` under
`2026-04-12`.

**archive_week_draft / unarchive_week_draft**

```json
{ "tool": "archive_week_draft", "arguments": { "draft": "2026-04-12/mcp-copy" } }
```

```json
{ "tool": "unarchive_week_draft", "arguments": { "draft": "2026-04-12/mcp-copy" } }
```

Expected: archive hides from default list; unarchive restores visibility.

**snapshot_week_draft (pinned)**

```json
{ "tool": "snapshot_week_draft", "arguments": { "draft": "2026-04-12/default", "keep": true, "note": "mcp-test" } }
```

Expected: success; snapshot created with `pinned: true`.

**list_week_draft_snapshots**

```json
{ "tool": "list_week_draft_snapshots", "arguments": { "draft": "2026-04-12/default" } }
```

Expected: JSON array including the pinned `mcp-test` snapshot.

**restore_week_draft_snapshot**

Use the `seq` from the `list_week_draft_snapshots` response:

```json
{ "tool": "restore_week_draft_snapshot", "arguments": { "draft": "2026-04-12/default", "seq": 1, "confirm": true } }
```

Expected: success; draft content reverts; a `pre-restore` snapshot is
recorded automatically.

**prune_week_draft_snapshots**

```json
{ "tool": "prune_week_draft_snapshots", "arguments": { "draft": "2026-04-12/default", "olderThanDays": 30 } }
```

Expected: `pruned: 0` (nothing is 30 days old in this walkthrough).

Stop the server with Ctrl-C.

---

## Step 15: Cleanup

Goal: delete all walkthrough-created test drafts so the working state is clean.

```bash
./tdx time week delete 2026-04-12/pristine  --yes
./tdx time week delete 2026-04-12/copy-test --yes  # already renamed; skip if absent
./tdx time week delete 2026-04-12/trial     --yes
./tdx time week delete 2026-04-12/mcp-copy  --yes
./tdx time week delete 2026-05-31/experiment --yes
./tdx time week delete 2026-05-10           --yes
./tdx time week delete 2026-06-01/mcp-blank --yes
./tdx time week list
```

### Verify

Only `2026-04-12/default` (or whichever drafts you had before starting this
walkthrough) remain. All `*-test`, `experiment`, `pristine`, `trial`,
`mcp-*` names are gone.

### Notes

`delete` removes the draft YAML but retains the snapshot directory.
Snapshots can be inspected after deletion via the raw filesystem path
`~/.config/tdx/profiles/default/weeks/<date>/<name>.snapshots/`.

---

## Step 16: Sign-off

Fill in after completing all steps.

```
Date run:     _______________
Tenant:       https://ufl.teamdynamix.com/
tdx version:  _______________
Tester:       _______________
Week used:    _______________

Passed steps: 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15
Failed steps: (none / list any that failed with a note)

Notes:
```
