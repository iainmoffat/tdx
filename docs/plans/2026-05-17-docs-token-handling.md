# Docs Token Handling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace three identical `TOKEN=$(grep "default:" ~/.config/tdx/credentials.yaml | awk '{print $2}')` snippets across two plan docs with the established `TDX_WALKTHROUGH_TOKEN` env-var pattern.

**Architecture:** Pure text edit. No code touched. Each occurrence becomes `TOKEN="${TDX_WALKTHROUGH_TOKEN:?set TDX_WALKTHROUGH_TOKEN to a valid TD bearer JWT first}"` — POSIX `:?word` syntax fails fast if the var is unset.

**Tech Stack:** None. Markdown only.

**Spec:** [`docs/specs/2026-05-17-docs-token-handling.md`](../specs/2026-05-17-docs-token-handling.md)

---

## File Structure

After this plan completes:

```
docs/
└── plans/
    ├── 2026-05-08-tdx-ticket-tasks.md           # MODIFY: 2 lines (2034, 2061)
    └── 2026-05-11-per-user-threshold-workable-hours.md  # MODIFY: 1 line (93)
```

## Branch + Versioning

- Branch: `docs-token-handling` (Task 0)
- Version: **v0.23.1** — patch, docs-only.

---

## Task 0: Create branch

**Files:** none

- [ ] **Step 1: Confirm clean tree on main**

```bash
git status
```

Expected: clean. Main is at `c456756` (the spec commit).

- [ ] **Step 2: Create branch**

```bash
git checkout -b docs-token-handling
```

Expected: `Switched to a new branch 'docs-token-handling'`.

---

## Task 1: Replace token-grep snippets

**Files:**
- Modify: `docs/plans/2026-05-08-tdx-ticket-tasks.md` (lines 2034 and 2061)
- Modify: `docs/plans/2026-05-11-per-user-threshold-workable-hours.md` (line 93)

- [ ] **Step 1: Confirm starting state**

```bash
git grep -n 'TOKEN=$(grep' docs/plans/
```

Expected output (3 matches):

```
docs/plans/2026-05-08-tdx-ticket-tasks.md:2034:TOKEN=$(grep "default:" ~/.config/tdx/credentials.yaml | awk '{print $2}')
docs/plans/2026-05-08-tdx-ticket-tasks.md:2061:TOKEN=$(grep "default:" ~/.config/tdx/credentials.yaml | awk '{print $2}')
docs/plans/2026-05-11-per-user-threshold-workable-hours.md:93:TOKEN=$(grep "default:" ~/.config/tdx/credentials.yaml | awk '{print $2}')
```

- [ ] **Step 2: Do the three replacements**

Use `sed -i ''` (macOS) or `sed -i` (Linux) to substitute. The exact replacement string is identical in all 3 spots:

```bash
sed -i '' \
  's|TOKEN=$(grep "default:" ~/.config/tdx/credentials.yaml | awk '\''{print $2}'\'')|TOKEN="${TDX_WALKTHROUGH_TOKEN:?set TDX_WALKTHROUGH_TOKEN to a valid TD bearer JWT first}"|g' \
  docs/plans/2026-05-08-tdx-ticket-tasks.md \
  docs/plans/2026-05-11-per-user-threshold-workable-hours.md
```

If the `sed` invocation gets thrown off by shell escaping of `$(...)` and pipes inside the search pattern, fall back to manual `Edit` tool calls on each occurrence. The replacement text is:

```
TOKEN="${TDX_WALKTHROUGH_TOKEN:?set TDX_WALKTHROUGH_TOKEN to a valid TD bearer JWT first}"
```

Source text (search pattern, must match verbatim):

```
TOKEN=$(grep "default:" ~/.config/tdx/credentials.yaml | awk '{print $2}')
```

- [ ] **Step 3: Verify the replacement**

```bash
git grep -n 'TOKEN=$(grep' docs/plans/
```

Expected: no output (zero matches).

```bash
git grep -n 'TDX_WALKTHROUGH_TOKEN:?' docs/plans/
```

Expected output (3 matches, one per replaced line):

```
docs/plans/2026-05-08-tdx-ticket-tasks.md:2034:TOKEN="${TDX_WALKTHROUGH_TOKEN:?set TDX_WALKTHROUGH_TOKEN to a valid TD bearer JWT first}"
docs/plans/2026-05-08-tdx-ticket-tasks.md:2061:TOKEN="${TDX_WALKTHROUGH_TOKEN:?set TDX_WALKTHROUGH_TOKEN to a valid TD bearer JWT first}"
docs/plans/2026-05-11-per-user-threshold-workable-hours.md:93:TOKEN="${TDX_WALKTHROUGH_TOKEN:?set TDX_WALKTHROUGH_TOKEN to a valid TD bearer JWT first}"
```

(Exact line numbers may shift by one if the replacement is longer than the original.)

- [ ] **Step 4: Sanity-check surrounding context**

The replacement should sit inside `bash` fenced code blocks next to `curl` commands. Spot-check that nothing was accidentally rewritten outside those code blocks:

```bash
git diff main..HEAD -- docs/plans/
```

Expected: exactly 3 hunks, each changing one `TOKEN=...` line. No other edits anywhere in the diff.

- [ ] **Step 5: Commit**

```bash
git add docs/plans/2026-05-08-tdx-ticket-tasks.md \
        docs/plans/2026-05-11-per-user-threshold-workable-hours.md
git commit -m "docs: replace token-grep snippets with TDX_WALKTHROUGH_TOKEN env-var pattern"
```

No `Co-Authored-By` trailer.

---

## Task 2: Push branch and create PR

**Files:** none

- [ ] **Step 1: Push branch**

```bash
git push -u origin docs-token-handling
```

- [ ] **Step 2: Create PR**

Write the body to `/tmp/pr-body-phase5.md`:

```markdown
## Summary

Phase 5 of the security hardening rollup. Addresses audit finding #7 (Low: docs include unsafe token-extraction shell snippets).

Replaces three identical `TOKEN=$(grep "default:" ~/.config/tdx/credentials.yaml | awk '{print $2}')` lines in two plan docs with the established env-var pattern:

```
TOKEN="${TDX_WALKTHROUGH_TOKEN:?set TDX_WALKTHROUGH_TOKEN to a valid TD bearer JWT first}"
```

The user supplies the token via `export TDX_WALKTHROUGH_TOKEN='<paste-jwt>'` before working through the walkthrough. POSIX `:?word` fails fast if the var is unset.

## Why this pattern

Already in use elsewhere (`docs/plans/2026-04-11-tdx-phase-2.5-sso-login.md`, `tdx-phase-3-write-ops.md`). This PR makes the convention consistent and removes the only doc-level encouragement of reading the credentials file directly.

## Test plan

- [x] `git grep 'TOKEN=$(grep' docs/plans/` returns no matches
- [x] `git grep 'TDX_WALKTHROUGH_TOKEN:?' docs/plans/` returns three matches
- [x] Diff scope is exactly three single-line replacements; no other edits

No code touched; CI runs `go test ./... -race` against an unchanged Go tree.

Closes: security audit finding #7.

Spec: `docs/specs/2026-05-17-docs-token-handling.md`
```

Then:

```bash
gh pr create --title "Replace unsafe token-grep snippets in docs (security hardening phase 5)" --body-file /tmp/pr-body-phase5.md
rm /tmp/pr-body-phase5.md
```

---

## Self-Review Notes

- [ ] Spec coverage:
  - 3 line replacements across 2 files — Task 1.
  - Verification commands (`git grep`) — Task 1 step 3.
  - PR creation — Task 2.
- [ ] No placeholders. Every step shows the exact commands and the exact replacement string.
- [ ] No type consistency concerns (no code touched).
- [ ] Each task is self-contained.
