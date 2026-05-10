# tdx time entry add — auto-fallback appID for --ticket (v0.16.4)

**Date:** 2026-05-09
**Goal:** When `tdx time entry add --ticket N` is invoked without `--app M`, fall back to the profile's `TicketAppID` (same default `tdx ticket *` commands use). Saves users from typing `--app 34` on every invocation when they have a default ticket app configured.

## Motivation

Since v0.16.0 (`tdx ticket app use <id>`), profiles can carry a default ticket app ID. The `tdx ticket *` commands all read this default automatically — `tdx ticket show 12345`, `tdx ticket log 12345 --hours 1 --type "..." --yes`, etc.

`tdx time entry add --ticket 12345` should work the same way. Today it doesn't:

```
$ tdx time entry add --ticket 12345 --hours 1 --type "Dev" --date 2026-05-09
Error: --app is required with --ticket
```

The user has to add `--app 34` even though the profile already knows the default. This is a small daily-flow friction. The fix is a few lines.

## Decisions

Settled during brainstorming on 2026-05-09:

1. **Fall back to `profile.TicketAppID`** when `--ticket N` is set but `--app M` is omitted. `--app` still overrides when both are passed.
2. **Error message updated** to mention `tdx ticket app use` as the way to set a profile default. Keeps the failure self-descriptive.
3. **Help text updated** for `--ticket`: drop "(requires --app)" since it's now optional.
4. **Scope: `--ticket` only.** Other target types (`--project`, `--workspace`) don't have a per-profile default concept; this patch doesn't touch them.
5. **No new flag.** Don't add `--target ticket://N` URL-style shorthand — bigger scope, defer to a future minor if asked.
6. **No MCP change.** `add_time_entry` isn't exposed via MCP today; if/when it is, it'll inherit the same logic naturally.

## Behavior

### Before (current)

`internal/cli/time/entry/add.go` lines 117-120:

```go
if f.ticket > 0 && f.app <= 0 {
    return fmt.Errorf("--app is required with --ticket")
}
```

### After

```go
if f.ticket > 0 && f.app <= 0 {
    // Try profile default before erroring.
    if id := lookupProfileTicketAppID(f.profile); id > 0 {
        f.app = id
    } else {
        return fmt.Errorf("--app is required with --ticket (or run `tdx ticket app use <id>` to set a profile default)")
    }
}
```

Where `lookupProfileTicketAppID(profile string) int` is a small helper that reads the profile from disk via `config.NewProfileStore(paths).GetProfile(profile)` and returns `prof.TicketAppID` (or `0` if anything fails — failure is silent, the caller treats `0` as "no default").

## Examples

```bash
# Profile has TicketAppID=34 (set via `tdx ticket app use 34`)
tdx time entry add --ticket 12345 --hours 1 --type "Development" --date 2026-05-09
# Works — appID 34 picked from profile.

# Override:
tdx time entry add --ticket 12345 --app 71 --hours 1 --type "..." --date 2026-05-09
# Works — appID 71 (explicit beats default).

# Profile has no TicketAppID set:
tdx time entry add --ticket 12345 --hours 1 --type "..." --date 2026-05-09
# Error: --app is required with --ticket (or run `tdx ticket app use <id>` to set a profile default)
```

## Implementation outline

### File changes

- Modify: `internal/cli/time/entry/add.go` — replace the bare error with a fall-back path; update the `--ticket` flag help text.
- Modify: `internal/cli/time/entry/add_test.go` — three new test cases.
- Modify: `docs/guide/time.md` — update the `## tdx time entry add` section to note the fallback behavior.

No service / domain / MCP changes.

### Helper

Extract a tiny pure helper for testability:

```go
// resolveTicketAppID returns appID for a --ticket invocation:
//   explicit > 0  → use it
//   else profile.TicketAppID > 0  → use it
//   else 0 (caller errors)
func resolveTicketAppID(explicit int, profileTicketAppID int) int {
    if explicit > 0 {
        return explicit
    }
    if profileTicketAppID > 0 {
        return profileTicketAppID
    }
    return 0
}
```

The cobra-side glue reads the profile from `config.NewProfileStore(paths).GetProfile(profileFlag)` and passes `prof.TicketAppID` into this helper. The helper itself is trivial to unit-test.

### Test cases

In `internal/cli/time/entry/add_test.go`:

1. **`TestResolveTicketAppIDExplicitWins`** — explicit=71, profileDefault=34 → returns 71.
2. **`TestResolveTicketAppIDFallsBackToProfile`** — explicit=0, profileDefault=34 → returns 34.
3. **`TestResolveTicketAppIDZeroWhenNeitherSet`** — explicit=0, profileDefault=0 → returns 0.

The cobra-level integration (does `runAdd` actually CALL `resolveTicketAppID` and gate on its result?) is harder to unit-test cleanly because `runAdd` does heavy `config.ResolvePaths()` work; we don't add a cobra-level test for that — live verification covers it.

## Live verification

1. `tdx auth status` → token valid.
2. `tdx ticket app show` → confirms default app is set.
3. `tdx time entry add --ticket 542034 --minutes 1 --type "<some valid type>" --date 2026-05-09 --description "v0.16.4 verify"` → should succeed without `--app`. (Note: 542034 is in IT Tickets app 34 which doesn't have time accounts wired — expect TD's error about missing time account if applicable. The KEY check is whether we get past the "--app is required" gate, not whether the time entry actually persists.)
4. Temporarily clear the profile's TicketAppID (`tdx ticket app use 0` if supported, or edit `~/.config/tdx/config.yaml` directly) → re-run step 3 → expect the new error message mentioning `tdx ticket app use`.
5. Restore profile default.

## Out of scope

- `tdx time entry add --task` (same fallback would apply, but `--task` already requires `--ticket`, which provides the appID context).
- `--target ticket://N` URL-style shorthand — bigger scope.
- MCP `add_time_entry` — not exposed today.
- Other target types' equivalents.

## Acceptance criteria

1. `tdx time entry add --ticket N --hours H --type "..." --date D` succeeds when no `--app` is given and the profile has `TicketAppID > 0`.
2. `tdx time entry add --ticket N --app M ...` still uses the explicit `--app` even if a profile default exists.
3. `tdx time entry add --ticket N` without `--app` and without a profile `TicketAppID` returns an error whose message mentions `tdx ticket app use`.
4. `tdx time entry add --ticket --help` text drops "(requires --app)" or notes the fallback.
5. All existing tests pass; 3 new helper tests pass; `go vet` and `golangci-lint` clean.
6. Live-verified on UFL.
7. Released as v0.16.4 (PR + squash + tag + Goreleaser).

## Risks

- **Profile-load failures should be silent.** If `GetProfile` errors (e.g. corrupted config), we fall through to the "explicit --app required" error. Don't propagate the load error — it'd confuse users who never asked for a profile lookup. This is the same defensive behavior `tdx ticket *` commands use.
- **No regression for users without profile defaults.** Anyone whose profile doesn't have `TicketAppID` set gets the same behavior as before, just with a slightly more helpful error message.
