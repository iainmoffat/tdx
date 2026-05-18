# Security Hardening — CI / Release Supply-Chain Hardening

**Date:** 2026-05-18
**Phase:** 6 of 7 (security hardening rollup; see [Phase 1 spec](2026-05-15-report-fanout-caps.md) for the full roadmap)
**Goal:** Lock the CI and release supply chain so a compromise of an upstream Action tag (or the GoReleaser `latest` tarball) cannot silently inject code into our build pipeline. Add a maintenance path so the locks don't go stale.

## Motivation

A security audit on 2026-05-15 identified CI/release supply-chain gaps as a Low-severity finding (`#6`):

- All actions in `ci.yml` and `release.yml` are **tag-pinned** (`@v4`, `@v5`, etc.). Tag refs are mutable — a compromised maintainer or upstream account can re-point `v4` to a malicious SHA after the fact, and our next workflow run silently executes the new code.
- `release.yml` uses `goreleaser version: latest`, which resolves at runtime to whatever GoReleaser cuts. A compromised future GoReleaser release would slip into our build of the next `v*` tag.
- `ci.yml` has no `permissions:` block; the `GITHUB_TOKEN` defaults to whatever the repo's settings allow (often read+write). CI does no writes today.

This phase fixes all three and adds Dependabot so the SHA pins don't rot.

## Threat model

- **Compromised upstream Action.** An attacker who controls a popular action's tag namespace (e.g. via maintainer-account takeover) can repoint `v4` to malicious code. Defended by SHA pinning.
- **Compromised GoReleaser release.** A bad release tag immediately propagates to every consumer using `version: latest`. Defended by pinning to a specific version.
- **Compromised CI step leveraging a broad token.** Even non-malicious actions can have bugs or transitive deps that read or write more than the workflow's intent. Defended by `permissions: contents: read`.
- **Bit-rot of pins.** SHA pins go stale; CVEs in pinned actions can sit unpatched. Defended by Dependabot weekly checks.

Out of scope: SLSA provenance signing (audit says "consider"; overkill for a single-contributor personal repo), gomod Dependabot (conflicts with the user's documented "don't run `go mod tidy` unless adding deps" preference).

## Scope

In scope:

- `.github/workflows/ci.yml`: SHA-pin 3 actions; add `permissions: contents: read`.
- `.github/workflows/release.yml`: SHA-pin 3 actions; pin GoReleaser CLI to `v2.15.4`.
- `.github/dependabot.yml` (new): weekly `github-actions` ecosystem updates.

Out of scope:

- SLSA / Sigstore provenance signing.
- Dependabot for the `gomod` ecosystem.
- Any changes to release artifacts, Homebrew tap, or the release tag flow.
- Finding #5 (keychain storage) — remains Phase 7.

## SHA-pin all Actions

Replace each `uses:` line with the `OWNER/REPO@SHA # vN` form. SHAs resolved on 2026-05-17 via `gh api repos/OWNER/REPO/git/ref/tags/vN`:

| Action | Tag | SHA |
|---|---|---|
| `actions/checkout` | v4 | `34e114876b0b11c390a56381ad16ebd13914f8d5` |
| `actions/setup-go` | v5 | `40f1582b2485089dde7abd97c1529aa768e1baff` |
| `golangci/golangci-lint-action` | v7 | `9fae48acfc02a90574d7c304a1758ef9895495fa` |
| `goreleaser/goreleaser-action` | v6 | `e435ccd777264be153ace6237001ef4d979d3a7a` |

**`ci.yml`** uses checkout, setup-go, golangci-lint-action.
**`release.yml`** uses checkout, setup-go, goreleaser-action.

The trailing `# vN` comment is the standard GitHub-recommended form. It keeps the human-readable version visible in `git blame` and PR diffs; Dependabot's `github-actions` ecosystem reads and updates these comments in lockstep with the SHA.

## Pin GoReleaser CLI version

In `release.yml`, change `version: latest` to `version: v2.15.4`. This is the current stable from `gh api repos/goreleaser/goreleaser/releases/latest` on 2026-05-17.

Two things this fixes:

- A compromised future GoReleaser release can't slip in silently.
- Reproducible builds: today's `latest` may differ from tomorrow's.

Dependabot does **not** track tool versions inside `with:` blocks, only the action `uses:` ref. Bumping GoReleaser to v2.16 etc. will be a manual edit — acceptable because GoReleaser version bumps occasionally have config-syntax changes worth pausing to review.

## `permissions: contents: read` on ci.yml

Add a top-level block to `ci.yml`:

```yaml
permissions:
  contents: read
```

CI today does no writes — no pushes, no comments, no releases. Default `GITHUB_TOKEN` scopes depend on repo settings and are often broader than needed. Clamping to `contents: read` limits blast radius if any step is ever compromised.

`release.yml` keeps its existing `permissions: contents: write` (needed to create the GitHub release).

## `.github/dependabot.yml`

New file:

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

**Behavior:**

- Once a week, scans `.github/workflows/*.yml` for `uses: OWNER/REPO@SHA # vN` lines.
- When the tag moves to a new SHA, opens a PR updating both the SHA and the `# vN` comment.
- Limit of 5 open PRs prevents Dependabot spam if multiple actions update in the same week.
- Commit prefix `chore(ci)` matches the project's existing commit-message style.

**Maintenance load:** ~1 PR per month in steady state. Read changelog, let the PR's own CI exercise the new SHA, squash-merge. About five minutes per PR.

`gomod` ecosystem is deliberately not configured: it would conflict with the user's documented preference of `// no go mod tidy without intent`.

## Verification

No code is touched, so no Go tests. Verification is shape-level:

```bash
# No mutable tag refs remain.
git grep '@v[0-9]' .github/workflows/
# expected: no matches

# Six SHA-pinned actions across two workflows.
git grep -E 'uses: [^@]+@[0-9a-f]{40} # v' .github/workflows/
# expected: 6 lines (3 in ci.yml + 3 in release.yml)

# GoReleaser pinned, not latest.
grep 'version:' .github/workflows/release.yml
# expected: version: v2.15.4

# Read-only permissions on ci.yml.
head -15 .github/workflows/ci.yml
# expected: contains "permissions:" block with "contents: read"

# Dependabot config exists.
test -f .github/dependabot.yml && echo ok
```

End-to-end: the next CI run after merge (and the next release after the following tag push) both pass. The release workflow's `goreleaser version` field now shows `v2.15.4` in the action log.

## Branch / version

- Branch: `ci-release-hardening`
- Version: **v0.23.2** — patch. CI/release infrastructure only; no code change, no behavior change for users.

## Open questions

None.
