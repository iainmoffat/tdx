# Week-oriented commands default to the current week

**Date:** 2026-04-28
**Status:** Approved (brainstorming complete)
**Owner:** ipm
**Type:** Focused UX fix (not a redesign)

---

## 0. Decisions log

| # | Decision |
|---|---|
| Q1 | Single-arg week commands (18 of them) get the date arg made optional. When omitted, default to the current week (Sunday containing `time.Now()` in EasternTZ). |
| Q2 | Multi-arg week commands (`set`, `rename`, `copy`) are out of scope. Their positional structure is load-bearing and the friction is small. Leave as-is. |
| Q3 | `tdx time week new` (with no positional date) defaults to creating a draft for the current week. The `--from-template`/`--from-draft`/`--name` flags continue to work. |
| Q4 | One shared resolver in `internal/cli/time/week/draft.go` (alongside the existing `ParseDraftRef`). All affected commands use it. |

---

## 1. Goal

`tdx time week pull` and the rest of the single-arg week commands should default to the current week when no date is provided. Today they all error with `accepts 1 arg(s), received 0`.

The fix should:
- Make the `<date>` (or `<date>[/<name>]`) positional optional on each affected command.
- Default to the Sunday of the current week in EasternTZ when omitted.
- Preserve all explicit-input semantics — if the user passes a ref, parse it the same way `ParseDraftRef` does today.
- Stay consistent across commands by routing through one shared helper.

---

## 2. Affected commands

### Already correct (no change)

- `show [date]` — `MaximumNArgs(1)`, falls back to `time.Now()`
- `locked` — no args, computes week from `time.Now()`
- `list` — `NoArgs`, lists all drafts (no week needed)

### In scope (18 commands)

Single-arg commands using `cobra.ExactArgs(1)` that need to become `cobra.MaximumNArgs(1)`:

`pull`, `status`, `edit`, `diff`, `preview`, `push`, `delete`, `note`, `history`, `reset`, `refresh`, `rebase`, `archive`, `unarchive`, `snapshot`, `restore`, `prune`, `new`.

### Out of scope

- `set <date>[/<name>] <row>:<day>=<hours>...` — multi-arg with positional structure
- `copy <src> <dst>` — both args are draft refs; no meaningful default
- `rename <date>[/<oldName>] <newName>` — first arg identifies the source draft

These keep their current `<date>` requirement.

---

## 3. Shared resolver

New function in `internal/cli/time/week/draft.go`, sibling to the existing `ParseDraftRef`:

```go
// ResolveWeekRef returns weekStart and name for a draft ref string. An
// empty ref defaults to the current week (Sunday containing time.Now() in
// EasternTZ) and name "default". A non-empty ref is parsed by ParseDraftRef.
func ResolveWeekRef(ref string) (time.Time, string, error) {
    if ref == "" {
        weekStart := domain.WeekRefContaining(time.Now()).StartDate
        return weekStart, "default", nil
    }
    return ParseDraftRef(ref)
}
```

The helper covers both shapes:
- `pull <date>` and `new <date>` — discard the returned name (always "default" or as-explicit; pull/new don't accept `/<name>` today).
- `cmd <date>[/<name>]` — use both returned values directly.

For `pull` and `new`, validating that the user didn't pass a `/<name>` slash in the date arg is preserved by reusing `ParseDraftRef`'s existing validation. (Today's pull/new accept any string and fail at `time.Parse` if it has a slash. The new behavior matches that.)

---

## 4. Per-command change pattern

For each in-scope command:

**Use string:** `cmd <date>[/<name>]` → `cmd [date[/name]]` (or `cmd [date]` for pull/new).

**Args:** `cobra.ExactArgs(1)` → `cobra.MaximumNArgs(1)`.

**Body:** Replace

```go
weekStart, name, err := ParseDraftRef(args[0])
if err != nil {
    return err
}
```

with

```go
ref := ""
if len(args) > 0 {
    ref = args[0]
}
weekStart, name, err := ResolveWeekRef(ref)
if err != nil {
    return err
}
```

**Short-doc text:** append "(defaults to the current week)" where it fits naturally.

---

## 5. Tests

- **Resolver test** (`internal/cli/time/week/draft_test.go`): empty ref → current week + "default"; bare date → that week + "default"; date/name → that week + name; invalid → error.
- **Smoke tests** for a representative sample (3 commands, including `pull`): verify `cobra.MaximumNArgs(1)` is set so no-arg invocation parses successfully. Don't write a per-command end-to-end test; the resolver covers the substantive logic.
- Existing per-command tests continue to work unchanged (they all pass an explicit date).

---

## 6. Docs

- **`README.md`** — Time Week Drafts table: change `<date>` to `[date]` for affected commands, with a single footnote like "Date is optional; omit to target the current week."
- **`docs/guide.md`** — search for command examples showing `<date>` and update where needed.

---

## 7. Out-of-scope explicitly

- `set`, `copy`, `rename` keep their current explicit week argument.
- No changes to `tdx time entry` commands (they have their own `--from`/`--to` flag-based scheme).
- No changes to MCP tools — they take structured `weekStart` JSON arguments where defaulting client-side is the agent's responsibility.
- No changes to draft watermark, snapshot retention, or anything domain-level.

---

## 8. Estimated work

~5 tasks: shared resolver + tests, batch-update the 18 commands, update `pull` test fixtures (one currently asserts the no-arg error), docs sweep, final verification + tag bump (likely v0.6.1 patch release).
