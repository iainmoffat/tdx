# Week Default Current Week Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> Each task follows strict TDD where applicable. Never amend commits — always create new ones. Branch: `fix-week-defaults` (already created off `main`, has the design spec at `docs/specs/2026-04-28-week-default-current-week.md`).
>
> Do NOT run `go mod tidy` — this fix introduces zero new dependencies.
>
> No `Co-Authored-By` trailer on commit messages.

**Design spec:** `docs/specs/2026-04-28-week-default-current-week.md`

**Goal:** Make the positional `<date>` argument optional on every single-arg `tdx time week` subcommand, defaulting to the current week (Sunday containing now in EasternTZ) when omitted.

**Architecture:** One shared resolver `ResolveWeekRef(ref string)` in `internal/cli/time/week/draft.go`, sibling to the existing `ParseDraftRef`. Eighteen command files switch from `cobra.ExactArgs(1)` to `cobra.MaximumNArgs(1)` and route an optional positional through the resolver.

**Tech Stack:** Go 1.24, cobra, gopkg.in/yaml.v3. No new deps.

**Affected commands (18):** `pull`, `status`, `edit`, `diff`, `preview`, `push`, `delete`, `note`, `history`, `reset`, `refresh`, `rebase`, `archive`, `unarchive`, `snapshot`, `restore`, `prune`, `new`. Out of scope: `set`, `copy`, `rename`, `show`, `locked`, `list`.

---

## Task 1: ResolveWeekRef helper + tests

**Files:**
- Modify: `internal/cli/time/week/draft.go`
- Create or extend: `internal/cli/time/week/draft_test.go`

- [ ] **Step 1.1 — Failing tests for ResolveWeekRef**

The file `internal/cli/time/week/draft_test.go` already exists (with `TestParseDraftRef`). Its current import block has only `"testing"`. Add `"time"`, `"github.com/iainmoffat/tdx/internal/domain"`, and `"github.com/stretchr/testify/require"` to the SAME import block (not a second one — Go forbids that). Then append the test functions below to the file.

```go
func TestResolveWeekRef_EmptyDefaultsToCurrentWeek(t *testing.T) {
	weekStart, name, err := ResolveWeekRef("")
	require.NoError(t, err)
	require.Equal(t, "default", name)
	expected := domain.WeekRefContaining(time.Now()).StartDate
	require.Equal(t, expected, weekStart)
}

func TestResolveWeekRef_BareDate(t *testing.T) {
	weekStart, name, err := ResolveWeekRef("2026-05-04")
	require.NoError(t, err)
	require.Equal(t, "default", name)
	// 2026-05-04 is a Monday; the containing week starts Sun 2026-05-03.
	expected, _ := time.ParseInLocation("2006-01-02", "2026-05-03", domain.EasternTZ)
	require.Equal(t, expected, weekStart)
}

func TestResolveWeekRef_DateWithName(t *testing.T) {
	weekStart, name, err := ResolveWeekRef("2026-05-04/pristine")
	require.NoError(t, err)
	require.Equal(t, "pristine", name)
	expected, _ := time.ParseInLocation("2006-01-02", "2026-05-03", domain.EasternTZ)
	require.Equal(t, expected, weekStart)
}

func TestResolveWeekRef_InvalidDate(t *testing.T) {
	_, _, err := ResolveWeekRef("not-a-date")
	require.Error(t, err)
}

func TestResolveWeekRef_EmptyNameAfterSlash(t *testing.T) {
	_, _, err := ResolveWeekRef("2026-05-04/")
	require.Error(t, err, "empty name after slash should fail (delegates to ParseDraftRef)")
}
```

- [ ] **Step 1.2 — Run tests to verify they fail**

Run: `go test ./internal/cli/time/week/ -run TestResolveWeekRef -v`
Expected: FAIL with "undefined: ResolveWeekRef".

- [ ] **Step 1.3 — Add ResolveWeekRef to draft.go**

Append to `internal/cli/time/week/draft.go`:

```go
// ResolveWeekRef returns the weekStart and draft name for an optional draft
// ref string. An empty ref defaults to the current week (Sunday containing
// time.Now() in EasternTZ) and the name "default". A non-empty ref is
// parsed by ParseDraftRef.
func ResolveWeekRef(ref string) (time.Time, string, error) {
	if ref == "" {
		weekStart := domain.WeekRefContaining(time.Now()).StartDate
		return weekStart, "default", nil
	}
	return ParseDraftRef(ref)
}
```

- [ ] **Step 1.4 — Run tests to verify they pass**

Run: `go test ./internal/cli/time/week/ -run TestResolveWeekRef -v`
Expected: PASS for all 5 sub-tests.

- [ ] **Step 1.5 — Commit**

```bash
git add internal/cli/time/week/draft.go internal/cli/time/week/draft_test.go
git commit -m "feat(week): ResolveWeekRef helper — empty input defaults to current week"
```

---

## Task 2: Migrate the 18 single-arg week commands

This task touches every command listed in the spec. Each edit follows the same three-line pattern (Use string + Args constraint + arg extraction). Land it as a single commit so the migration is atomic.

**Files (18 modify, no new):**
- `internal/cli/time/week/pull.go`
- `internal/cli/time/week/status.go`
- `internal/cli/time/week/edit.go`
- `internal/cli/time/week/diff.go`
- `internal/cli/time/week/preview.go`
- `internal/cli/time/week/push.go`
- `internal/cli/time/week/delete.go`
- `internal/cli/time/week/note.go`
- `internal/cli/time/week/history.go`
- `internal/cli/time/week/reset.go`
- `internal/cli/time/week/refresh.go`
- `internal/cli/time/week/rebase.go`
- `internal/cli/time/week/archive.go` (handles BOTH `archive` and `unarchive`)
- `internal/cli/time/week/snapshot.go`
- `internal/cli/time/week/restore.go`
- `internal/cli/time/week/prune.go`
- `internal/cli/time/week/new.go`

(That's 16 files covering 18 commands — `archive.go` defines both `archive` and `unarchive`.)

### The standard edit pattern

For each command, three changes:

**A. Use string** — change the positional from required to optional:
- `<date>[/<name>]` → `[date[/name]]`
- `<date>` (pull, new) → `[date]`
- `<date>[/<name>] --snapshot N --yes` (restore) → `[date[/name]] --snapshot N --yes`

**B. Args constraint** — `cobra.ExactArgs(1)` → `cobra.MaximumNArgs(1)`.

**C. Arg extraction in RunE** — replace
```go
RunE: func(cmd *cobra.Command, args []string) error {
    return runFoo(cmd, f, args[0])
},
```
with
```go
RunE: func(cmd *cobra.Command, args []string) error {
    ref := ""
    if len(args) > 0 {
        ref = args[0]
    }
    return runFoo(cmd, f, ref)
},
```

Then inside `runFoo`, replace `weekStart, name, err := ParseDraftRef(ref)` with `weekStart, name, err := ResolveWeekRef(ref)`.

For `archive.go` which uses a shared inner builder for both archive/unarchive: edit the shared builder once.

- [ ] **Step 2.1 — Edit pull.go**

In `internal/cli/time/week/pull.go`:

```go
// BEFORE
cmd := &cobra.Command{
    Use:   "pull <date>",
    Short: "Pull a live week into a local draft",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        return runPull(cmd, f, args[0])
    },
}
```

```go
// AFTER
cmd := &cobra.Command{
    Use:   "pull [date]",
    Short: "Pull a live week into a local draft (defaults to the current week)",
    Args:  cobra.MaximumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        ref := ""
        if len(args) > 0 {
            ref = args[0]
        }
        return runPull(cmd, f, ref)
    },
}
```

In the same file, inside `runPull`, change:
```go
weekStart, name, err := ParseDraftRef(ref)
```
to:
```go
weekStart, name, err := ResolveWeekRef(ref)
```

- [ ] **Step 2.2 — Edit status.go**

In `internal/cli/time/week/status.go`:
- `Use: "status <date>[/<name>]"` → `Use: "status [date[/name]]"`
- `Args: cobra.ExactArgs(1)` → `Args: cobra.MaximumNArgs(1)`
- Short doc: append "(defaults to the current week)" if it isn't there
- RunE: extract `ref := ""` if-len pattern shown above
- In the runner, change `ParseDraftRef(ref)` → `ResolveWeekRef(ref)`

- [ ] **Step 2.3 — Edit edit.go**

Same three-part edit:
- `Use: "edit <date>[/<name>]"` → `Use: "edit [date[/name]]"`
- `Args: cobra.ExactArgs(1)` → `Args: cobra.MaximumNArgs(1)`
- RunE: extract `ref := ""` if-len pattern
- Runner: `ParseDraftRef(ref)` → `ResolveWeekRef(ref)`

- [ ] **Step 2.4 — Edit diff.go**

Same edit pattern. `Use: "diff <date>[/<name>]"` → `Use: "diff [date[/name]]"`. Args + RunE + runner change.

- [ ] **Step 2.5 — Edit preview.go**

Same edit pattern. `Use: "preview <date>[/<name>]"` → `Use: "preview [date[/name]]"`. Args + RunE + runner change.

- [ ] **Step 2.6 — Edit push.go**

Same edit pattern. `Use: "push <date>[/<name>]"` → `Use: "push [date[/name]]"`. Args + RunE + runner change.

- [ ] **Step 2.7 — Edit delete.go**

Same edit pattern. `Use: "delete <date>[/<name>]"` → `Use: "delete [date[/name]]"`. Args + RunE + runner change.

- [ ] **Step 2.8 — Edit note.go**

Same edit pattern. `Use: "note <date>[/<name>]"` → `Use: "note [date[/name]]"`. Args + RunE + runner change.

- [ ] **Step 2.9 — Edit history.go**

Same edit pattern. `Use: "history <date>[/<name>]"` → `Use: "history [date[/name]]"`. Args + RunE + runner change.

- [ ] **Step 2.10 — Edit reset.go**

Same edit pattern. `Use: "reset <date>[/<name>]"` → `Use: "reset [date[/name]]"`. Args + RunE + runner change.

- [ ] **Step 2.11 — Edit refresh.go**

Same edit pattern. `Use: "refresh <date>[/<name>]"` → `Use: "refresh [date[/name]]"`. Args + RunE + runner change.

- [ ] **Step 2.12 — Edit rebase.go**

Same edit pattern. `Use: "rebase <date>[/<name>]"` → `Use: "rebase [date[/name]]"`. Args + RunE + runner change.

- [ ] **Step 2.13 — Edit archive.go (covers both archive and unarchive)**

`archive.go` has two `cobra.Command` definitions sharing a builder. For each:
- `Use: "archive <date>[/<name>]"` → `Use: "archive [date[/name]]"`
- `Use: "unarchive <date>[/<name>]"` → `Use: "unarchive [date[/name]]"`
- `Args: cobra.ExactArgs(1)` → `Args: cobra.MaximumNArgs(1)` (both)
- RunE extraction pattern (both)
- Wherever `ParseDraftRef(ref)` is used in the runner, switch to `ResolveWeekRef(ref)`

- [ ] **Step 2.14 — Edit snapshot.go**

Same edit pattern. `Use: "snapshot <date>[/<name>]"` → `Use: "snapshot [date[/name]]"`. Args + RunE + runner change.

- [ ] **Step 2.15 — Edit restore.go**

Same edit pattern. `Use: "restore <date>[/<name>] --snapshot N --yes"` → `Use: "restore [date[/name]] --snapshot N --yes"`. Args + RunE + runner change.

- [ ] **Step 2.16 — Edit prune.go**

Same edit pattern. `Use: "prune <date>[/<name>]"` → `Use: "prune [date[/name]]"`. Args + RunE + runner change.

- [ ] **Step 2.17 — Edit new.go**

Same edit pattern. `Use: "new <date>"` → `Use: "new [date]"`. Args + RunE + runner change. Note: in `runNew`, the call site is `weekStart, name, err := ParseDraftRef(dateRef)` — change to `ResolveWeekRef(dateRef)`. The `f.fromDraft` parsing inside `runNew` keeps `ParseDraftRef` (it's not optional; it's a flag value).

- [ ] **Step 2.18 — Add cobra-arg smoke tests**

Append to `internal/cli/time/week/draft_test.go`. Add `"github.com/spf13/cobra"` to the existing import block (which by Task 1 already has `testing`, `time`, `domain`, `require`).

```go
func TestSingleArgWeekCommands_AcceptZeroArgs(t *testing.T) {
	cases := []struct {
		name string
		cmd  *cobra.Command
	}{
		{"pull", newPullCmd()},
		{"status", newStatusCmd()},
		{"refresh", newRefreshCmd()},
		{"new", newNewCmd()},
		{"history", newHistoryCmd()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.NotNil(t, tc.cmd)
			require.NoError(t, tc.cmd.Args(tc.cmd, []string{}),
				"%s should accept zero args after migration", tc.name)
			require.NoError(t, tc.cmd.Args(tc.cmd, []string{"2026-05-04"}),
				"%s should still accept one arg", tc.name)
		})
	}
}
```

Add `"github.com/spf13/cobra"` to the imports if not already there.

- [ ] **Step 2.19 — Run all tests in the week package**

Run: `go test ./internal/cli/time/week/ -v -run 'TestResolveWeekRef|TestSingleArgWeekCommands' 2>&1 | tail -40`
Expected: PASS for all 5 ResolveWeekRef sub-tests + 5 SingleArgWeekCommands sub-tests.

Run the full week package to confirm no regressions:

Run: `go test ./internal/cli/time/week/ -count=1`
Expected: ok.

- [ ] **Step 2.20 — Run go vet to verify all 16 files compile**

Run: `go vet ./...`
Expected: clean.

- [ ] **Step 2.21 — Commit**

```bash
git add internal/cli/time/week/
git commit -m "feat(cli): week commands default to current week when date is omitted"
```

---

## Task 3: Update README and guide.md

**Files:**
- Modify: `README.md`
- Modify: `docs/guide.md`

- [ ] **Step 3.1 — README: change `<date>` to `[date]` in the Time Week Drafts table**

Open `README.md`. Find the Time Week Drafts table (around lines 84–105). For each row covering an in-scope command, change the syntax column:

| Before | After |
|---|---|
| `tdx time week pull <date>` | `tdx time week pull [date]` |
| `tdx time week show <date> --draft [name]` | (already correct, leave) |
| `tdx time week status <date>[/<name>]` | `tdx time week status [date[/name]]` |
| `tdx time week edit <date>[/<name>]` | `tdx time week edit [date[/name]]` |
| `tdx time week diff <date>[/<name>]` | `tdx time week diff [date[/name]]` |
| `tdx time week preview <date>[/<name>]` | `tdx time week preview [date[/name]]` |
| `tdx time week push <date>[/<name>] --yes` | `tdx time week push [date[/name]] --yes` |
| `tdx time week delete <date>[/<name>] --yes` | `tdx time week delete [date[/name]] --yes` |
| `tdx time week note <date>[/<name>]` | `tdx time week note [date[/name]]` |
| `tdx time week history <date>[/<name>]` | `tdx time week history [date[/name]]` |
| `tdx time week new <date>` | `tdx time week new [date]` |
| `tdx time week reset <date>[/<name>] --yes` | `tdx time week reset [date[/name]] --yes` |
| `tdx time week refresh <date>[/<name>]` | `tdx time week refresh [date[/name]]` |
| `tdx time week rebase <date>[/<name>]` | `tdx time week rebase [date[/name]]` |
| `tdx time week archive <date>[/<name>]` | `tdx time week archive [date[/name]]` |
| `tdx time week unarchive <date>[/<name>]` | `tdx time week unarchive [date[/name]]` |
| `tdx time week snapshot <date>[/<name>]` | `tdx time week snapshot [date[/name]]` |
| `tdx time week restore <date>[/<name>] --snapshot N --yes` | `tdx time week restore [date[/name]] --snapshot N --yes` |
| `tdx time week prune <date>[/<name>] --yes` | `tdx time week prune [date[/name]] --yes` |

`set <date>[/<name>] <row>:<day>=<h>`, `copy <src> <dst>`, `rename <date>[/<old>] <new>` stay as-is — out of scope.

Add a one-line note immediately above or below the table:

```markdown
> Commands taking `[date]` or `[date[/name]]` default to the current week if omitted.
```

- [ ] **Step 3.2 — guide.md: search for command examples and update where necessary**

In `docs/guide.md`, search for any occurrence of `<date>` or `<date>[/<name>]` in command-syntax lines or code blocks. Where found in an in-scope command context, change to `[date]` or `[date[/name]]`. Inline narrative like "for week `<date>`" can stay if it's prose, not command syntax.

Likely-affected sections (verify presence, edit if found):
- "Week drafts" section command summaries
- "Pull" / "Edit" / "Push" subsections that show full command lines
- "Refresh & rebase" subsection (added in B.2a)

For each section that documents a command's invocation, append a sentence noting the default behavior, e.g. "If you omit the date, the current week is used."

- [ ] **Step 3.3 — Commit**

```bash
git add README.md docs/guide.md
git commit -m "docs: README + guide.md — week commands default to current week"
```

---

## Task 4: Final verification + version bump + PR

**Files:**
- (Maybe) `cmd/tdx/main.go` if version is hardcoded for dev; otherwise no source change.

- [ ] **Step 4.1 — Full quality gate**

Run:
```bash
go test ./... && go vet ./... && gofmt -l . && golangci-lint run ./...
```

Expected: all green. `gofmt -l .` should print nothing. If `golangci-lint` flags anything in the new code, fix inline.

- [ ] **Step 4.2 — Manual sanity check**

Build and try the no-arg form:

```bash
go build -o /tmp/tdx ./cmd/tdx
/tmp/tdx time week pull --help | head -3
/tmp/tdx time week refresh --help | head -3
```

Expected: usage lines show `[date]` or `[date[/name]]`. The `pull` line should read `tdx time week pull [date]`.

You can also test that the no-arg call resolves correctly without actually pulling:

```bash
/tmp/tdx time week show 2>&1 | head -2  # already worked, sanity confirm
```

(Don't run `tdx time week pull` in the sanity check — it would create a real draft. Help text inspection is enough.)

- [ ] **Step 4.3 — Push branch + open PR**

```bash
git push -u origin fix-week-defaults
gh pr create --title "fix(cli): week commands default to current week when date is omitted" --body "$(cat <<'EOF'
## Summary
- 18 single-arg \`tdx time week\` commands now accept zero positional args
- When date is omitted, defaults to the Sunday of the current week (EasternTZ)
- Shared resolver \`ResolveWeekRef\` in \`internal/cli/time/week/draft.go\`
- README + guide.md updated to reflect optional positional

## Spec
\`docs/specs/2026-04-28-week-default-current-week.md\`

## Out of scope
\`set\`, \`copy\`, \`rename\` — multi-arg commands keep their explicit week ref.

## Test plan
- [x] \`go test ./...\` — green
- [x] \`go vet ./...\` — clean
- [x] \`gofmt -l .\` — clean
- [x] \`golangci-lint run ./...\` — 0 issues
- [x] \`tdx time week pull --help\` shows \`[date]\` (not \`<date>\`)
EOF
)"
```

- [ ] **Step 4.4 — After PR merges: tag and release**

This is a CLI UX patch — no engine changes — so a patch-version tag is appropriate.

```bash
git checkout main
git pull
git tag -a v0.6.1 -m "fix(cli): week commands default to current week when date is omitted"
git push origin v0.6.1
```

Goreleaser auto-publishes the new release; Homebrew tap auto-updates per the existing pipeline.

---

## Notes for the implementer

- The migration is mechanical. The most likely failure mode is missing one of the 16 files. After Step 2.20 finishes, double-check by grepping: `grep -l 'cobra.ExactArgs(1)' internal/cli/time/week/*.go` should NOT list any of the in-scope files (it should still list `set.go`, `copy.go`, `rename.go`, and possibly nothing else).
- If the grep above lists an in-scope file, you missed it. Edit it and add to the existing commit (or make a fix-up commit — never amend the published commit).
- Don't add per-command no-arg integration tests. The resolver covers the substantive logic; the cobra-arg smoke tests in Step 2.18 prove the constraint flipped correctly. A bigger test surface here just adds maintenance burden.
- Don't change `set`, `copy`, `rename` even though they take a week-shaped first arg. Per the spec, they're out of scope.
- Don't change MCP tool args. Agents are responsible for choosing weekStart client-side.
