# tdx time entry add — auto-fallback appID Implementation Plan (v0.16.4)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When `tdx time entry add --ticket N` is invoked without `--app M`, fall back to the profile's `TicketAppID`. Update help text and docs.

**Architecture:** Tiny pure helper `resolveTicketAppID(explicit, profileDefault int) int` extracted for unit-testability. Cobra-side glue reads the profile via existing `config.NewProfileStore` and threads the result through the helper before the existing `--app required` check.

**Tech Stack:** Go 1.26.2; cobra. No service / domain / MCP changes.

**Spec:** [`docs/specs/2026-05-09-tdx-time-entry-ticket-app-fallback.md`](../specs/2026-05-09-tdx-time-entry-ticket-app-fallback.md)

---

## File Structure

After this plan completes:

```
internal/
└── cli/
    └── time/
        └── entry/
            ├── add.go         # MODIFY: extract resolveTicketAppID; replace bare error with fallback path; update help text
            └── add_test.go    # MODIFY: 3 new helper tests
docs/
└── guide/
    └── time.md                # MODIFY: note auto-fallback in tdx time entry add section
```

No new files. No README/tree changes.

## Branch + Versioning

- Branch: `entry-ticket-app-fallback` (Task 0)
- Version: v0.16.4 (no source change; tagged after merge)

---

## Task 0: Create branch

**Files:** none

- [ ] **Step 1: Confirm clean tree on main**

```bash
git status
```

Expected: `On branch main`, working tree clean.

- [ ] **Step 2: Create branch**

```bash
git checkout -b entry-ticket-app-fallback
```

Expected: `Switched to a new branch 'entry-ticket-app-fallback'`.

---

## Task 1: Extract `resolveTicketAppID` helper + tests

**Files:**
- Modify: `internal/cli/time/entry/add.go`
- Modify: `internal/cli/time/entry/add_test.go`

- [ ] **Step 1: Add the helper to `add.go`**

Append at the end of `internal/cli/time/entry/add.go` (after `runAdd` and before any other top-level declarations):

```go
// resolveTicketAppID returns the appID to use for a --ticket invocation:
//   - if explicit > 0, use that (caller-provided --app)
//   - else if profileTicketAppID > 0, use the profile default
//   - else 0 (caller treats as "no appID resolved" and errors out)
//
// Pure helper for testability. The cobra glue reads the profile from disk
// and passes prof.TicketAppID as the second argument; profile-load failures
// surface as profileTicketAppID=0 (no propagation), matching the silent-
// fallback pattern used elsewhere in the ticket commands.
func resolveTicketAppID(explicit, profileTicketAppID int) int {
	if explicit > 0 {
		return explicit
	}
	if profileTicketAppID > 0 {
		return profileTicketAppID
	}
	return 0
}
```

- [ ] **Step 2: Write three failing tests**

Append to `internal/cli/time/entry/add_test.go`:

```go
func TestResolveTicketAppIDExplicitWins(t *testing.T) {
	got := resolveTicketAppID(71, 34)
	if got != 71 {
		t.Errorf("explicit should win over profile default; got %d, want 71", got)
	}
}

func TestResolveTicketAppIDFallsBackToProfile(t *testing.T) {
	got := resolveTicketAppID(0, 34)
	if got != 34 {
		t.Errorf("should fall back to profile default; got %d, want 34", got)
	}
}

func TestResolveTicketAppIDZeroWhenNeitherSet(t *testing.T) {
	got := resolveTicketAppID(0, 0)
	if got != 0 {
		t.Errorf("should return 0 when neither is set; got %d", got)
	}
}
```

- [ ] **Step 3: Run tests to verify they pass**

```bash
go test ./internal/cli/time/entry/... -run TestResolveTicketAppID -v
```

Expected: 3 PASS.

- [ ] **Step 4: Verify full package + lint**

```bash
go build ./...
go test ./internal/cli/time/entry/...
go vet ./internal/cli/time/entry/...
gofmt -l internal/cli/time/entry/
golangci-lint run ./internal/cli/time/entry/...
```

All clean.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/time/entry/add.go internal/cli/time/entry/add_test.go
git commit -m "feat(cli/time/entry): add resolveTicketAppID helper"
```

**No `Co-Authored-By:` trailer** (per `feedback_no_coauthor.md` memory).

---

## Task 2: Wire helper into `runAdd`; update --ticket help text

**Files:**
- Modify: `internal/cli/time/entry/add.go`

- [ ] **Step 1: Replace the bare error path with a fallback**

Find the existing block in `runAdd` (currently lines ~117-120):

```go
// Companion flag validation.
if f.ticket > 0 && f.app <= 0 {
	return fmt.Errorf("--app is required with --ticket")
}
```

Replace with:

```go
// Companion flag validation.
if f.ticket > 0 && f.app <= 0 {
	// Fall back to the profile's default TicketAppID (set via
	// `tdx ticket app use <id>`) before erroring out.
	profileDefault := 0
	if paths, perr := config.ResolvePaths(); perr == nil {
		if prof, gerr := config.NewProfileStore(paths).GetProfile(f.profile); gerr == nil {
			profileDefault = prof.TicketAppID
		}
	}
	f.app = resolveTicketAppID(f.app, profileDefault)
	if f.app <= 0 {
		return fmt.Errorf("--app is required with --ticket (or run `tdx ticket app use <id>` to set a profile default)")
	}
}
```

Notes:
- `config.ResolvePaths()` and `config.NewProfileStore` are already accessible — `config` is imported in `add.go` (line 10 of the existing file).
- `f.profile` is the resolved profile flag (passed via `--profile` or empty). `GetProfile("")` may or may not error depending on profile-store semantics — we silently fall through to `profileDefault=0` either way, so the user gets the explicit `--app required` error rather than a confusing "no profile" error from the lookup.
- `config.NewProfileStore(paths).GetProfile(f.profile)` works for `f.profile == ""` only if the store treats empty-name as "default profile". If it doesn't, the GetProfile call fails silently and we surface the standard `--app required` error. **Verify by reading `internal/config/profiles.go` GetProfile behavior — adjust to call `auth.ResolveProfile(f.profile)` first if needed.**

If verification shows `GetProfile("")` doesn't auto-resolve, replace the inner block with:

```go
if paths, perr := config.ResolvePaths(); perr == nil {
	auth := authsvc.New(paths)
	if pname, rerr := auth.ResolveProfile(f.profile); rerr == nil {
		if prof, gerr := config.NewProfileStore(paths).GetProfile(pname); gerr == nil {
			profileDefault = prof.TicketAppID
		}
	}
}
```

(`authsvc` is already imported in `add.go` line 13.)

Pick whichever path correctly handles the "no `--profile` flag set; use the active profile" case. Prefer the `auth.ResolveProfile` form unless `GetProfile` already does that work.

- [ ] **Step 2: Update the `--ticket` flag help text**

Find the line declaring `--ticket` (currently around line 57):

```go
cmd.Flags().IntVar(&f.ticket, "ticket", 0, "ticket ID (requires --app)")
```

Replace with:

```go
cmd.Flags().IntVar(&f.ticket, "ticket", 0, "ticket ID (uses profile's default app if --app not set)")
```

- [ ] **Step 3: Verify**

```bash
go build ./...
go test ./internal/cli/time/entry/...
go vet ./internal/cli/time/entry/...
gofmt -l internal/cli/time/entry/
golangci-lint run ./internal/cli/time/entry/...
go run ./cmd/tdx time entry add --help | grep -i ticket
```

The help line should show the new wording. All tests pass.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/time/entry/add.go
git commit -m "feat(cli/time/entry): --ticket falls back to profile.TicketAppID when --app omitted"
```

**No `Co-Authored-By:` trailer.**

---

## Task 3: Documentation

**Files:**
- Modify: `docs/guide/time.md`

- [ ] **Step 1: Find the `## tdx time entry add` section**

```bash
grep -n "## tdx time entry add\|### tdx time entry add" docs/guide/time.md
```

Note the line range — somewhere around lines 60-100 in the existing file.

- [ ] **Step 2: Update the prose to mention the fallback**

In the section that documents the `--ticket` / `--app` flag pair, find the description that says (or implies) "--app is required with --ticket" and update.

If there's a sentence/bullet like:

```markdown
- `--ticket <id>` — ticket ID (requires `--app`)
- `--app <id>` — application ID (required with `--ticket`)
```

Change to:

```markdown
- `--ticket <id>` — ticket ID. If `--app` is not provided, the profile's default ticket app (from `tdx ticket app use <id>`) is used.
- `--app <id>` — application ID. Required with `--ticket` only if no profile default is set; explicit `--app` always overrides the profile default.
```

If the existing prose differs, adapt the wording — the goal is for a reader to understand that `--app` is now optional when a profile default exists.

Add a one-line example below if it fits the style of the surrounding section:

```markdown
```bash
# With profile.TicketAppID=34, --app is no longer needed:
tdx time entry add --ticket 12345 --hours 1 --type "Development" --date 2026-05-09
```
```

- [ ] **Step 3: Verify**

```bash
grep -n "profile.s default ticket app\|profile default" docs/guide/time.md | head
```

Should return at least one match in the section you edited.

- [ ] **Step 4: Commit**

```bash
git add docs/guide/time.md
git commit -m "docs: note --ticket auto-fallback to profile.TicketAppID in tdx time entry add"
```

**No `Co-Authored-By:` trailer.**

---

## Task 4: Live verification + PR + release

**Files:** none modified — verification + git operations only.

### Live verification

- [ ] **Step 1: Pre-flight**

```bash
go build ./... && go test ./... && go vet ./... && gofmt -l . && golangci-lint run ./...
```

All green required.

- [ ] **Step 2: Build local binary + check auth**

```bash
go build -o tdx ./cmd/tdx
./tdx auth status | head -5
./tdx ticket app show
```

Confirm `state: authenticated` / `token: valid` and that the profile has a ticket app set (e.g. app 34).

- [ ] **Step 3: Verify the fallback works**

```bash
# With profile default set to 34, this should NOT error on --app:
./tdx time entry add --ticket 542034 --minutes 1 --type "<some valid type>" --date 2026-05-09 --description "v0.16.4 verify" --dry-run
```

Note: ticket 542034 lives in IT Tickets app 34 which doesn't have time accounts wired (verified during v0.16.0 testing — `Time account N was not found for use with the item time entry 0`). Use `--dry-run` to skip the actual submit; the KEY check is that the command gets past the "--app is required" gate and into the type-resolution / dry-run output. If dry-run isn't supported in this command, omit it and accept that TD may reject the entry — what matters is the command exits with a TD-side error, not the local "--app required" error.

Alternative: do a `--dry-run`-equivalent check by passing an invalid type and confirming the command reaches the type-resolution step (which would only happen if we got past the appID gate). Or use a ticket in a different app where time accounts are wired.

- [ ] **Step 4: Verify explicit --app still wins**

```bash
./tdx time entry add --ticket 542034 --app 34 --minutes 1 --type "<some valid type>" --date 2026-05-09 --description "explicit"
```

Should behave the same as Step 3 (since --app and the profile default both = 34 in this test).

- [ ] **Step 5: Verify the new error message**

Temporarily set `TicketAppID: 0` in `~/.config/tdx/config.yaml` (or pick a profile without it set):

```bash
./tdx --profile <some-profile-without-ticketAppID> time entry add --ticket 542034 --minutes 1 --type "..." --date 2026-05-09
```

Should error with text mentioning `tdx ticket app use`. Restore the original config when done.

If you can't easily clear the profile default, this step is acceptable to skip — the CLI logic is unit-tested by `TestResolveTicketAppIDZeroWhenNeitherSet`.

### Push + PR + merge + tag

- [ ] **Step 6: Push branch**

```bash
rm tdx 2>/dev/null
git push -u origin entry-ticket-app-fallback
```

- [ ] **Step 7: Open PR**

```bash
gh pr create --title "v0.16.4: tdx time entry add --ticket falls back to profile default app" --body-file /tmp/pr-body-v0.16.4.md
```

Body:

```markdown
## Summary

Small QoL fix: when `tdx time entry add --ticket N` is invoked without `--app M`, fall back to the profile's `TicketAppID` (the same per-profile default that `tdx ticket *` commands have used since v0.16.0). Saves users from typing `--app 34` on every invocation.

### Before / after

```bash
# Before:
$ tdx time entry add --ticket 12345 --hours 1 --type "Dev" --date 2026-05-09
Error: --app is required with --ticket

# After (with `tdx ticket app use 34` configured):
$ tdx time entry add --ticket 12345 --hours 1 --type "Dev" --date 2026-05-09
# Works — uses app 34
```

### What changed

- New pure helper `resolveTicketAppID(explicit, profileDefault int) int` in `internal/cli/time/entry/add.go` (3 unit tests)
- `runAdd` reads `profile.TicketAppID` and threads it through the helper before erroring on missing `--app`
- `--ticket` flag help text updated; `--app` is no longer described as "required"
- Error message when neither flag nor profile default is set now points at `tdx ticket app use`
- Doc note in `docs/guide/time.md`

### Out of scope

- Other target types (`--project`, `--workspace`) — they don't have a per-profile default concept
- New `--target ticket://N` URL-style shorthand — bigger scope
- MCP `add_time_entry` — not exposed today

Spec: `docs/specs/2026-05-09-tdx-time-entry-ticket-app-fallback.md`
Plan: `docs/plans/2026-05-09-tdx-time-entry-ticket-app-fallback.md`

## Test plan

- [x] `resolveTicketAppID` unit tests (3): explicit-wins, profile-fallback, zero-when-neither
- [x] `--app` explicit override still works
- [x] Help text reflects the new behavior
- [x] `go test ./... && go vet ./... && gofmt -l . && golangci-lint run ./...` all green
- [x] Live-verified on UFL: --ticket 542034 without --app now reaches the type-resolution step (was previously rejected at the "--app required" gate)

After merge, tag `v0.16.4` to trigger Goreleaser.
```

(Write the body to `/tmp/pr-body-v0.16.4.md` first to avoid heredoc-escaping issues.)

- [ ] **Step 8: Wait for CI; merge**

```bash
# Poll CI:
gh run list --branch entry-ticket-app-fallback --limit 1 --json status,conclusion

# Once status:completed, conclusion:success:
gh pr merge <PR#> --squash --admin --delete-branch
```

- [ ] **Step 9: Reset main, tag, push tag**

```bash
git checkout main
git fetch origin
git reset --hard origin/main
git tag v0.16.4
git push origin v0.16.4
```

Goreleaser publishes the release.

- [ ] **Step 10: Update memory**

Edit `MEMORY.md` index line for "current state" → v0.16.4. Add a "Latest release" block to `project_tdx_current_state.md`. Mark this item as shipped in `project_tdx_backlog.md`.

---

## Self-Review

**1. Spec coverage:**
- Spec § Behavior (fall back to profile.TicketAppID) → Task 2
- Spec § Implementation outline (extract resolveTicketAppID helper) → Task 1
- Spec § Test cases (3 helper tests) → Task 1 step 2
- Spec § Help text update → Task 2 step 2
- Spec § Error message update → Task 2 step 1
- Spec § Documentation update → Task 3
- Spec § Live verification → Task 4 steps 3-5
- Spec § Acceptance criteria 1-7 → Tasks 2 (acc 1, 2, 3, 4) + Task 1 (acc 5: tests pass) + Task 4 (acc 6, 7)

All requirements have a task.

**2. Placeholder scan:**
- Task 2 step 1 has a "verify by reading internal/config/profiles.go" instruction with two concrete fallback code paths. That's a probe directive for the implementer, not a placeholder.
- Task 4 step 3 has a "use --dry-run if supported, else accept TD's error" instruction. Concrete contingency logic, not a vague TODO.
- No "TBD" / "fill in details" anywhere.

**3. Type consistency:**
- `resolveTicketAppID(explicit, profileTicketAppID int) int` — defined in Task 1, called in Task 2 with `(f.app, profileDefault)`. Consistent.
- `f.app`, `f.ticket`, `f.profile` — existing struct fields on `addFlags`, not redefined.
- `config.NewProfileStore(paths).GetProfile(...)` — matches the established pattern from v0.16.0 (`tdx ticket app use`).

All consistent.
