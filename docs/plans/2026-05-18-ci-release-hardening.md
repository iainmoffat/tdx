# CI/Release Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** SHA-pin every Action in `ci.yml` and `release.yml`, pin GoReleaser CLI to `v2.15.4`, add `permissions: contents: read` to `ci.yml`, and add a Dependabot config so the pins don't rot.

**Architecture:** Pure YAML config changes — three workflow/config files in `.github/`. No Go code, no tests. Verification is shape-level (`git grep` patterns) plus the post-merge CI run itself.

**Tech Stack:** GitHub Actions workflow YAML; Dependabot config.

**Spec:** [`docs/specs/2026-05-18-ci-release-hardening.md`](../specs/2026-05-18-ci-release-hardening.md)

---

## File Structure

After this plan completes:

```
.github/
├── dependabot.yml         # CREATE: weekly github-actions ecosystem updates
└── workflows/
    ├── ci.yml             # MODIFY: SHA-pin 3 actions, add permissions: contents: read
    └── release.yml        # MODIFY: SHA-pin 3 actions, pin GoReleaser to v2.15.4
```

## Branch + Versioning

- Branch: `ci-release-hardening` (Task 0)
- Version: **v0.23.2** — patch; CI/release infrastructure only, no Go code change.

## Resolved SHAs (captured 2026-05-17)

| Action | Tag | SHA |
|---|---|---|
| `actions/checkout` | v4 | `34e114876b0b11c390a56381ad16ebd13914f8d5` |
| `actions/setup-go` | v5 | `40f1582b2485089dde7abd97c1529aa768e1baff` |
| `golangci/golangci-lint-action` | v7 | `9fae48acfc02a90574d7c304a1758ef9895495fa` |
| `goreleaser/goreleaser-action` | v6 | `e435ccd777264be153ace6237001ef4d979d3a7a` |

---

## Task 0: Create branch

**Files:** none

- [ ] **Step 1: Confirm clean tree on main**

```bash
git status
```

Expected: clean. Main is at `89c330b` (the spec commit).

- [ ] **Step 2: Create branch**

```bash
git checkout -b ci-release-hardening
```

Expected: `Switched to a new branch 'ci-release-hardening'`.

---

## Task 1: Harden `ci.yml`

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Write the full new file**

Replace the entire contents of `.github/workflows/ci.yml` with:

```yaml
name: CI

on:
  push:
    branches: ['*']
  pull_request:

permissions:
  contents: read

jobs:
  ci:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # v4

      - uses: actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff # v5
        with:
          go-version-file: go.mod

      - name: Vet
        run: go vet ./...

      - name: Lint
        uses: golangci/golangci-lint-action@9fae48acfc02a90574d7c304a1758ef9895495fa # v7
        with:
          version: v2.11.4

      - name: Test
        run: go test ./... -count=1 -race

      - name: Build
        run: go build ./cmd/tdx
```

Three changes from the pre-existing file:

1. New `permissions:` block (lines 8-9).
2. Each `uses:` for an action is now `OWNER/REPO@SHA # vN`.
3. Everything else (`name:`, `on:`, `jobs:`, the run steps) is byte-for-byte identical to today.

- [ ] **Step 2: Verify shape locally**

```bash
git diff .github/workflows/ci.yml
```

Expected: the new `permissions:` block appears as an addition; the three `uses:` lines change from `@vN` to `@SHA # vN`; nothing else changes.

- [ ] **Step 3: Sanity-check with grep**

```bash
git grep '@v[0-9]' .github/workflows/ci.yml
```

Expected: no matches.

```bash
git grep -E 'uses: [^@]+@[0-9a-f]{40} # v' .github/workflows/ci.yml
```

Expected: 3 lines.

```bash
grep -A 1 '^permissions:' .github/workflows/ci.yml
```

Expected: `permissions:` followed by `  contents: read`.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: SHA-pin actions and add contents:read permission"
```

No `Co-Authored-By` trailer.

---

## Task 2: Harden `release.yml`

**Files:**
- Modify: `.github/workflows/release.yml`

- [ ] **Step 1: Write the full new file**

Replace the entire contents of `.github/workflows/release.yml` with:

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff # v5
        with:
          go-version-file: go.mod

      - uses: goreleaser/goreleaser-action@e435ccd777264be153ace6237001ef4d979d3a7a # v6
        with:
          version: v2.15.4
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
```

Changes from the pre-existing file:

1. Three `uses:` lines change from `@vN` to `@SHA # vN`.
2. `version: latest` → `version: v2.15.4`.
3. The existing `permissions: contents: write` is preserved (release.yml needs write to create the GitHub release).
4. Everything else (`name:`, `on:`, `env:`, `args:`) is byte-for-byte identical.

- [ ] **Step 2: Sanity-check with grep**

```bash
git grep '@v[0-9]' .github/workflows/release.yml
```

Expected: no matches.

```bash
git grep -E 'uses: [^@]+@[0-9a-f]{40} # v' .github/workflows/release.yml
```

Expected: 3 lines.

```bash
grep '^\s*version:' .github/workflows/release.yml
```

Expected: `          version: v2.15.4` (not `latest`).

```bash
grep -A 1 '^permissions:' .github/workflows/release.yml
```

Expected: `permissions:` followed by `  contents: write` (write, not read — release needs this).

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "ci(release): SHA-pin actions and pin GoReleaser CLI to v2.15.4"
```

No `Co-Authored-By` trailer.

---

## Task 3: Add Dependabot config

**Files:**
- Create: `.github/dependabot.yml`

- [ ] **Step 1: Write the file**

Create `.github/dependabot.yml` with exactly:

```yaml
version: 2
updates:
  - package-ecosystem: github-actions
    directory: /
    schedule:
      interval: weekly
    open-pull-requests-limit: 5
    commit-message:
      prefix: "chore(ci)"
```

- [ ] **Step 2: Validate the file is syntactically reasonable**

```bash
test -f .github/dependabot.yml && echo "exists"
python3 -c "import yaml; yaml.safe_load(open('.github/dependabot.yml'))" && echo "parses"
```

Expected: both `exists` and `parses` print. GitHub will validate the schema on push; if it rejects, fix and recommit.

- [ ] **Step 3: Commit**

```bash
git add .github/dependabot.yml
git commit -m "chore(ci): add Dependabot config for weekly github-actions updates"
```

No `Co-Authored-By` trailer.

---

## Task 4: Verification sweep

**Files:** none

- [ ] **Step 1: Confirm no mutable tag refs remain in workflows**

```bash
git grep '@v[0-9]' .github/workflows/
```

Expected: no matches.

- [ ] **Step 2: Confirm exactly 6 SHA-pinned action references**

```bash
git grep -cE 'uses: [^@]+@[0-9a-f]{40} # v' .github/workflows/ci.yml .github/workflows/release.yml
```

Expected:

```
.github/workflows/ci.yml:3
.github/workflows/release.yml:3
```

- [ ] **Step 3: Confirm Dependabot file**

```bash
test -f .github/dependabot.yml && cat .github/dependabot.yml
```

Expected: the file content from Task 3 step 1.

- [ ] **Step 4: Confirm permissions blocks**

```bash
grep -B 0 -A 1 '^permissions:' .github/workflows/ci.yml .github/workflows/release.yml
```

Expected: `ci.yml` shows `contents: read`; `release.yml` shows `contents: write`.

- [ ] **Step 5: Confirm GoReleaser pinned**

```bash
grep 'version: v' .github/workflows/release.yml
```

Expected: `          version: v2.15.4`.

If any check fails, fix the offending file and re-run the relevant verify step.

---

## Task 5: Push branch and create PR

**Files:** none

- [ ] **Step 1: Push branch**

```bash
git push -u origin ci-release-hardening
```

- [ ] **Step 2: Create PR**

Write the body to `/tmp/pr-body-phase6.md`:

```markdown
## Summary

Phase 6 of the security hardening rollup. Addresses audit finding #6 (Low: CI/release supply-chain hardening gaps).

- SHA-pin every Action in `ci.yml` and `release.yml` (4 distinct actions, 6 `uses:` lines).
- Pin GoReleaser CLI to `v2.15.4` (was `latest`).
- Add `permissions: contents: read` to `ci.yml` (release.yml keeps `contents: write` — needs it to create releases).
- Add `.github/dependabot.yml` for weekly `github-actions` ecosystem updates so the SHA pins don't rot.

No SLSA provenance; no gomod Dependabot.

## Test plan

- [x] `git grep '@v[0-9]' .github/workflows/` returns no matches
- [x] `git grep -E 'uses: [^@]+@[0-9a-f]{40} # v' .github/workflows/` returns 6 lines
- [x] `permissions:` blocks correct on each workflow
- [x] GoReleaser pinned to `v2.15.4`
- [ ] Next CI run after merge passes (proves SHAs resolve and lint/test still work)
- [ ] Next `v*` tag triggers release.yml successfully

Closes: security audit finding #6.

Spec: `docs/specs/2026-05-18-ci-release-hardening.md`
```

Then:

```bash
gh pr create --title "CI/release hardening (security hardening phase 6)" --body-file /tmp/pr-body-phase6.md
rm /tmp/pr-body-phase6.md
```

---

## Self-Review Notes

- [ ] Spec coverage:
  - SHA pin all 4 actions in 2 workflows — Tasks 1, 2.
  - GoReleaser CLI pin — Task 2.
  - `permissions: contents: read` on ci.yml — Task 1.
  - Dependabot config — Task 3.
  - Verification — Task 4.
  - PR creation — Task 5.
- [ ] No placeholders. Each step shows exact YAML or exact commands.
- [ ] Type consistency: SHAs and version strings are byte-identical across tasks. Each `# vN` comment matches its tag.
- [ ] Each task is self-contained and commits independently.
