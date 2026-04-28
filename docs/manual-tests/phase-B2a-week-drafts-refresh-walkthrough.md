# Phase B.2a Walkthrough — Refresh / Rebase Three-Way Merge

**Tenant:** UFL
**Profile:** `ufl` (or whatever profile is active for live-tenant tests)
**Goal:** End-to-end exercise of `tdx time week refresh` against the live UFL
TeamDynamix tenant.

> ⚠️ This walkthrough creates and modifies real time entries in TD. Use a week
> that you don't mind temporarily having garbage data in. The walkthrough
> cleans up at the end.

## 0. Setup

- [ ] Confirm `tdx` is on the v0.6.0 release candidate or newer.
- [ ] Run `tdx auth whoami` — confirm correct profile.
- [ ] Pick a target week (typically the current week if it's safe to dirty
  it, otherwise a future week). Note `WEEK=YYYY-MM-DD`.

## 1. No-op refresh (no remote changes)

- [ ] `tdx time week pull $WEEK` — pull the live week into a default draft.
- [ ] `tdx time week refresh $WEEK` — should print:

  ```
  Refresh complete.
    Adopted (remote -> draft):  0 cells
    Preserved (local edits):    0 cells
    Resolved (same on both):    0 cells
  ```

  Exit code 0.

- [ ] `tdx time week status $WEEK` — sync state should still be `clean`.

## 2. Auto-merge (local edit + non-conflicting remote add)

- [ ] Edit the local draft so one cell changes hours (e.g. Mon
  4.0h → 6.0h). Use `tdx time week edit` or directly edit the YAML.
- [ ] In the TD web UI, ADD a separate entry for a different day this week
  (e.g. Wed +2.0h) on a row that doesn't conflict with your local edit.
- [ ] `tdx time week refresh $WEEK` — should print non-zero `Adopted` AND
  `Preserved` counts. Exit 0.
- [ ] `tdx time week show $WEEK --draft` — confirms both edits coexist.

## 3. Conflict + abort (default strategy)

- [ ] Edit your local draft to change Mon 6.0h → 7.0h (or another value).
- [ ] In the TD web UI, change the SAME Mon entry to 9.0h.
- [ ] `tdx time week refresh $WEEK` — should:
  - Print `Refresh aborted: 1 cell(s) conflict ...`
  - List the conflict with `local: updated to 7.0h` / `remote: updated to 9.0h`
  - Exit non-zero.
- [ ] `tdx time week status $WEEK` — sync state UNCHANGED (still dirty,
  Mon still 7.0h).
- [ ] `ls $TDX_HOME/profiles/$PROFILE/weeks/$WEEK/default.snapshots/` — no
  new `pre-refresh-*` file (abort doesn't snapshot).

## 4. Strategy ours (resolves conflict, keeps local)

- [ ] `tdx time week refresh $WEEK --strategy ours` — should:
  - Print `Refresh complete (--strategy ours).`
  - Show `Resolved by --strategy: 1 cells (local won)`
  - Exit 0.
- [ ] `tdx time week show $WEEK --draft` — Mon shows 7.0h (local won).
- [ ] `ls $TDX_HOME/profiles/$PROFILE/weeks/$WEEK/default.snapshots/` —
  exactly one new `pre-refresh-*` snapshot is present.
- [ ] `tdx time week refresh $WEEK` — second consecutive refresh is a no-op
  (Adopted=0, Resolved=0, Preserved counter reflects unchanged local edits).

## 5. Strategy theirs (resolves conflict, takes remote)

- [ ] In the TD web UI, change the same Mon entry to 5.0h.
- [ ] Edit your local draft to change the same Mon to 11.0h.
- [ ] `tdx time week refresh $WEEK --strategy theirs` — should:
  - Print `Refresh complete (--strategy theirs).`
  - Show `Resolved by --strategy: 1 cells (remote won)`
  - Exit 0.
- [ ] `tdx time week show $WEEK --draft` — Mon shows 5.0h.

## 6. History view shows pre-refresh snapshots

- [ ] `tdx time week history $WEEK` — should list snapshots tagged
  `pre-refresh` for both the ours run (step 4) and theirs run (step 5).

## 7. Restore from a pre-refresh snapshot

- [ ] Pick a `pre-refresh` snapshot from step 6 — note its sequence number `N`.
- [ ] `tdx time week restore $WEEK --snapshot N --yes` — should restore the
  pre-refresh state.
- [ ] `tdx time week show $WEEK --draft` — Mon back to its pre-refresh hours.

## 8. JSON output

- [ ] `tdx time week refresh $WEEK --json` — emits a single JSON envelope
  with `schema = tdx.v1.weekDraftRefreshResult`, all counter fields, and
  `conflicts: []` (or populated if there are pending conflicts).

## 9. Cleanup

- [ ] `tdx time week reset $WEEK --yes` — discard local edits.
- [ ] In the TD web UI, restore Mon to its original value.
- [ ] `tdx time week prune $WEEK --yes` — sweep the test snapshots.

## Sign-off

| Step | Pass / Fail | Notes |
|---|---|---|
| 1 No-op | | |
| 2 Auto-merge | | |
| 3 Abort | | |
| 4 Ours | | |
| 5 Theirs | | |
| 6 History | | |
| 7 Restore | | |
| 8 JSON | | |
| 9 Cleanup | | |
