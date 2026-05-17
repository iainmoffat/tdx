# Security Hardening — Replace Unsafe Token-Grep Snippets in Docs

**Date:** 2026-05-17
**Phase:** 5 of 7 (security hardening rollup; see [Phase 1 spec](2026-05-15-report-fanout-caps.md) for the full roadmap)
**Goal:** Replace every `TOKEN=$(grep "default:" ~/.config/tdx/credentials.yaml | awk '{print $2}')` line in tracked docs with the env-var pattern already established elsewhere in the project (`TDX_WALKTHROUGH_TOKEN`).

## Motivation

A security audit on 2026-05-15 identified the token-grep pattern as a Low-severity finding (`#7`). The snippet encourages risky shell habits:

- The user copy-pastes a command that reads the plaintext credentials file. The file path and contents end up in shell history, terminal scrollback, and screen-capture risks.
- On a shared dev box or CI machine, the same snippet may read a *different* user's credentials file with no warning.
- `TOKEN=$(...)` puts the subshell command line in `ps` briefly, exposing the file path.
- More importantly: it normalizes "extract the token from the credentials file" as a routine pattern. The right pattern is "the user supplies the token explicitly via environment".

The fix is a one-line replacement in three places.

## Scope

Three identical occurrences of:

```bash
TOKEN=$(grep "default:" ~/.config/tdx/credentials.yaml | awk '{print $2}')
```

| File | Line |
|---|---|
| `docs/plans/2026-05-08-tdx-ticket-tasks.md` | 2034 (audit-cited) |
| `docs/plans/2026-05-08-tdx-ticket-tasks.md` | 2061 |
| `docs/plans/2026-05-11-per-user-threshold-workable-hours.md` | 93 |

Out of scope:

- `docs/superpowers/` (legacy/gitignored — not edited in this PR).
- `docs/guide/auth.md` and `README.md` — already clean; only mention `credentials.yaml` in descriptive text, no grep extraction.
- New CLI surface (e.g. `tdx auth status --token`) — would add a permanent code feature for a one-off doc concern.
- Findings #5 (keychain storage) and #6 (CI hardening) — remain Phases 7 and 6.

## Replacement

Replace each occurrence verbatim with:

```bash
TOKEN="${TDX_WALKTHROUGH_TOKEN:?set TDX_WALKTHROUGH_TOKEN to a valid TD bearer JWT first}"
```

POSIX `:?word` syntax: if `TDX_WALKTHROUGH_TOKEN` is unset or empty, the shell exits with `bash: TDX_WALKTHROUGH_TOKEN: set TDX_WALKTHROUGH_TOKEN to a valid TD bearer JWT first`. The walkthrough fails fast with a clear message rather than running curl with an empty token.

The user runs:

```bash
export TDX_WALKTHROUGH_TOKEN='<paste-your-jwt-here>'
```

before working through the walkthrough.

## Why this pattern

- **Established elsewhere.** `docs/plans/2026-04-11-tdx-phase-2.5-sso-login.md:818,854` and `docs/plans/2026-04-11-tdx-phase-3-write-ops.md:3176` already use `TDX_WALKTHROUGH_TOKEN`. This PR makes the convention consistent.
- **No file read.** The credentials file is never opened; no path or token contents enter shell history.
- **No cross-user accident.** The token comes from the user's hands, not from whichever `credentials.yaml` happens to exist on the machine.
- **Fail-fast on missing token.** `:?` blocks the walkthrough rather than producing a confusing `401 Unauthorized` later.

## Tests

No code is touched, so no Go tests. Verification is by inspection:

```bash
git grep -n 'TOKEN=$(grep' docs/plans/
```

After the PR, this command returns no matches (outside `docs/superpowers/` which is gitignored).

Also:

```bash
git grep -n 'TDX_WALKTHROUGH_TOKEN:?' docs/plans/
```

Returns the three replacements.

## Walkthrough

No new manual walkthrough doc — the change is purely textual. The plans themselves remain walkthroughs; only the unsafe line is replaced.

## Branch / version

- Branch: `docs-token-handling`
- Version: **v0.23.1** — patch; docs-only, no code change, no behavior change.

## Open questions

None.
