# Phase B.2a — Week Drafts Refresh/Merge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> Each task follows strict TDD: failing test → verify failure → implement → verify pass → commit. Never amend commits — always create new ones. Branch: `phase-B2a-refresh-merge` (already created off `main`, has the design spec at `docs/specs/2026-04-28-tdx-phase-B2a-design.md`).
>
> Do NOT run `go mod tidy` — Phase B.2a adds zero new dependencies.
>
> No `Co-Authored-By` trailer on commit messages.

**Design spec:** `docs/specs/2026-04-28-tdx-phase-B2a-design.md`
**Builds on:** Phase B.1 (v0.5.0)

**Goal:** Add the three-way merge primitive — `tdx time week refresh / rebase` with `--strategy abort|ours|theirs` — so a user can safely refresh a draft against the latest remote without losing local edits or clobbering remote changes.

**Architecture**

```
CLI layer (internal/cli/time/week/)
  |-- refresh.go   newRefreshCmd  -- runRefresh
  |-- rebase.go    newRebaseCmd   -- alias of runRefresh
  v
Service layer (internal/svc/draftsvc/)
  |-- refresh.go   Strategy enum, MergeConflict, RefreshResult,
  |                 classify() pure function, Service.Refresh()
  v
Domain (internal/domain/) -- UNCHANGED. OpPreRefresh, CellConflict, SyncConflicted
                             already declared in Phase A; B.2a starts emitting
                             OpPreRefresh only.
```

**MCP:** 1 new mutating tool (`refresh_week_draft`). Tool count: 37 → 38. New schema: `tdx.v1.weekDraftRefreshResult`.

**Tech Stack:** Go 1.24, cobra, gopkg.in/yaml.v3, modelcontextprotocol/go-sdk. No new deps.

---

## Task 1: Strategy enum + MergeConflict + RefreshResult types

**Files:**
- Create: `internal/svc/draftsvc/refresh.go`
- Create: `internal/svc/draftsvc/refresh_test.go`

- [ ] **Step 1.1 — Failing test for Strategy.Validate**

Create `internal/svc/draftsvc/refresh_test.go`:

```go
package draftsvc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStrategy_Validate(t *testing.T) {
	cases := []struct {
		in      Strategy
		wantErr bool
	}{
		{StrategyAbort, false},
		{StrategyOurs, false},
		{StrategyTheirs, false},
		{Strategy(""), true},
		{Strategy("merge"), true},
	}
	for _, tc := range cases {
		err := tc.in.Validate()
		if tc.wantErr {
			require.Error(t, err, "expected error for %q", tc.in)
		} else {
			require.NoError(t, err, "unexpected error for %q", tc.in)
		}
	}
}

func TestRefreshResult_ZeroValueIsAbortFalse(t *testing.T) {
	var r RefreshResult
	require.False(t, r.Aborted)
	require.Empty(t, r.Conflicts)
	require.Equal(t, 0, r.Adopted)
	require.Equal(t, 0, r.Preserved)
	require.Equal(t, 0, r.Resolved)
	require.Equal(t, 0, r.ResolvedByStrategy)
}
```

- [ ] **Step 1.2 — Run test to verify it fails**

Run: `go test ./internal/svc/draftsvc/ -run 'TestStrategy_Validate|TestRefreshResult_ZeroValueIsAbortFalse' -v`
Expected: FAIL with "undefined: Strategy" / "undefined: RefreshResult".

- [ ] **Step 1.3 — Create refresh.go skeleton with types**

Create `internal/svc/draftsvc/refresh.go`:

```go
package draftsvc

import (
	"fmt"
)

// Strategy selects how Service.Refresh handles cell-level conflicts between
// local edits and remote changes. See docs/specs/2026-04-28-tdx-phase-B2a-design.md.
type Strategy string

const (
	// StrategyAbort refuses to mutate the draft if any conflicts exist; the
	// engine returns RefreshResult{Aborted: true, Conflicts: ...}.
	StrategyAbort Strategy = "abort"
	// StrategyOurs collapses every conflict by keeping the local cell.
	StrategyOurs Strategy = "ours"
	// StrategyTheirs collapses every conflict by taking the remote cell.
	StrategyTheirs Strategy = "theirs"
)

// Validate reports whether s is one of the three known strategies.
func (s Strategy) Validate() error {
	switch s {
	case StrategyAbort, StrategyOurs, StrategyTheirs:
		return nil
	default:
		return fmt.Errorf("unknown refresh strategy %q (want abort|ours|theirs)", string(s))
	}
}

// MergeConflict describes one cell where local and remote diverged in a way
// the engine cannot resolve without a strategy.
type MergeConflict struct {
	RowID             string
	Day               string // time.Weekday.String()
	LocalDescription  string // human-readable summary of local intent
	RemoteDescription string // human-readable summary of remote state
}

// RefreshResult reports what happened. Aborted=true with a non-empty Conflicts
// list means refresh refused to mutate anything.
type RefreshResult struct {
	Strategy           Strategy
	Adopted            int // cells whose remote changes were taken
	Preserved          int // cells where local edits survived
	Resolved           int // cells where both sides converged on the same value (no conflict)
	ResolvedByStrategy int // conflicts resolved by ours/theirs (always 0 under abort)
	Aborted            bool
	Conflicts          []MergeConflict
}
```

- [ ] **Step 1.4 — Run tests to verify pass**

Run: `go test ./internal/svc/draftsvc/ -run 'TestStrategy_Validate|TestRefreshResult_ZeroValueIsAbortFalse' -v`
Expected: PASS.

- [ ] **Step 1.5 — Commit**

```bash
git add internal/svc/draftsvc/refresh.go internal/svc/draftsvc/refresh_test.go
git commit -m "feat(refresh): Strategy enum + MergeConflict + RefreshResult types"
```

---

## Task 2: Cell classifier — happy paths

The classifier is a pure function over three cell maps keyed by `(rowKey, weekday)`. This task lays the foundation: the four non-conflict outcomes from §4 of the spec.

**Files:**
- Modify: `internal/svc/draftsvc/refresh.go`
- Modify: `internal/svc/draftsvc/refresh_test.go`

- [ ] **Step 2.1 — Failing tests for happy-path classifications**

Add `"time"` and `"github.com/iainmoffat/tdx/internal/domain"` to the existing import block of `internal/svc/draftsvc/refresh_test.go`, then append:

```go
func cell(hours float64, sourceID int) domain.DraftCell {
	return domain.DraftCell{Hours: hours, SourceEntryID: sourceID}
}

// classifyCellTC drives a single (rowKey, weekday) classification through
// classifyCell and asserts on the merged cell + outcome counter.
type classifyCellTC struct {
	name           string
	pulled         *domain.DraftCell // nil = absent
	local          *domain.DraftCell
	remote         *domain.DraftCell
	strategy       Strategy
	wantOutcome    cellOutcome
	wantMergeCell  *domain.DraftCell
	wantConflict   bool
}

func runClassifyCellTCs(t *testing.T, tcs []classifyCellTC) {
	t.Helper()
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			res := classifyCell(tc.pulled, tc.local, tc.remote, tc.strategy)
			require.Equal(t, tc.wantOutcome, res.outcome, "outcome mismatch")
			require.Equal(t, tc.wantConflict, res.conflict != nil, "conflict presence mismatch")
			if tc.wantMergeCell == nil {
				require.Nil(t, res.merged, "expected no merged cell")
			} else {
				require.NotNil(t, res.merged, "expected a merged cell")
				require.Equal(t, tc.wantMergeCell.Hours, res.merged.Hours, "merged hours")
				require.Equal(t, tc.wantMergeCell.SourceEntryID, res.merged.SourceEntryID, "merged sourceID")
			}
		})
	}
}

func TestClassifyCell_HappyPaths(t *testing.T) {
	c := cell(4, 100)
	c2 := cell(6, 100)
	cAdd := cell(3, 0)
	tcs := []classifyCellTC{
		{
			name:          "untouched: same on all three views",
			pulled:        &c, local: &c, remote: &c,
			strategy:      StrategyAbort,
			wantOutcome:   outcomeUntouched,
			wantMergeCell: &c,
		},
		{
			name:          "adopt remote: local unchanged, remote changed",
			pulled:        &c, local: &c, remote: &c2,
			strategy:      StrategyAbort,
			wantOutcome:   outcomeAdopted,
			wantMergeCell: &c2,
		},
		{
			name:          "preserve local: local changed, remote unchanged",
			pulled:        &c, local: &c2, remote: &c,
			strategy:      StrategyAbort,
			wantOutcome:   outcomePreserved,
			wantMergeCell: &c2,
		},
		{
			name:          "converged: both changed to same value",
			pulled:        &c, local: &c2, remote: &c2,
			strategy:      StrategyAbort,
			wantOutcome:   outcomeResolved,
			wantMergeCell: &c2,
		},
		{
			name:          "remote-only added (didn't exist locally)",
			pulled:        nil, local: nil, remote: &cAdd,
			strategy:      StrategyAbort,
			wantOutcome:   outcomeAdopted,
			wantMergeCell: &cAdd,
		},
		{
			name:          "local-only added (still not on remote)",
			pulled:        nil, local: &cAdd, remote: nil,
			strategy:      StrategyAbort,
			wantOutcome:   outcomePreserved,
			wantMergeCell: &cAdd,
		},
	}
	runClassifyCellTCs(t, tcs)
}
```

- [ ] **Step 2.2 — Run test to verify it fails**

Run: `go test ./internal/svc/draftsvc/ -run TestClassifyCell_HappyPaths -v`
Expected: FAIL with "undefined: classifyCell" and friends.

- [ ] **Step 2.3 — Implement happy-path classifier**

Add `"github.com/iainmoffat/tdx/internal/domain"` to the existing import block of `internal/svc/draftsvc/refresh.go`, then append the rest of this code below the existing types:

```go
// cellOutcome categorizes a single classifyCell result.
type cellOutcome int

const (
	outcomeNone               cellOutcome = iota
	outcomeUntouched                      // same on all three; pass through
	outcomeAdopted                        // remote-side change taken
	outcomePreserved                      // local-side change kept
	outcomeResolved                       // local and remote converged on same value
	outcomeResolvedByStrategy             // real conflict, collapsed by strategy
	outcomeDropped                        // cell drops out of merged set entirely
)

// cellClassification is what classifyCell returns for one (rowKey, weekday).
type cellClassification struct {
	outcome  cellOutcome
	merged   *domain.DraftCell // nil if cell drops out (deleted on both, or absent everywhere)
	conflict *MergeConflict    // non-nil only under StrategyAbort when we'd abort
}

// cellEqual reports whether two cells have the same hours+sourceID. Used by
// classifyCell to detect "unchanged from pull" and "converged" outcomes.
func cellEqual(a, b domain.DraftCell) bool {
	return a.Hours == b.Hours && a.SourceEntryID == b.SourceEntryID
}

// cellPresent reports whether ptr points at a cell that should be considered
// "present" (non-nil and not zero-cleared with no source ID).
func cellPresent(c *domain.DraftCell) bool {
	if c == nil {
		return false
	}
	return c.Hours != 0 || c.SourceEntryID != 0
}

// classifyCell is the pure-function core of the merge engine. It looks at one
// (rowKey, weekday) across the three views (at-pull-time, current-local,
// current-remote) and decides what the merged cell should be plus what
// counter to bump. Conflicts are reported as cellClassification.conflict;
// under StrategyOurs/StrategyTheirs the engine resolves them inline and
// outcome is outcomeResolvedByStrategy.
//
// Subsequent tasks extend this function with the conflict cases. Task 2
// covers only the non-conflict happy paths.
func classifyCell(pulled, local, remote *domain.DraftCell, strategy Strategy) cellClassification {
	pulledExists := cellPresent(pulled)
	localExists := cellPresent(local)
	remoteExists := cellPresent(remote)

	switch {
	// Cell exists in all three views.
	case pulledExists && localExists && remoteExists:
		localUnchanged := cellEqual(*pulled, *local)
		remoteUnchanged := cellEqual(*pulled, *remote)
		if localUnchanged && remoteUnchanged {
			merged := *local
			return cellClassification{outcome: outcomeUntouched, merged: &merged}
		}
		if localUnchanged && !remoteUnchanged {
			merged := *remote
			return cellClassification{outcome: outcomeAdopted, merged: &merged}
		}
		if !localUnchanged && remoteUnchanged {
			merged := *local
			return cellClassification{outcome: outcomePreserved, merged: &merged}
		}
		// Both sides changed.
		if cellEqual(*local, *remote) {
			merged := *local
			return cellClassification{outcome: outcomeResolved, merged: &merged}
		}
		// True conflict; later task fills in this branch.

	// Brand-new on remote only.
	case !pulledExists && !localExists && remoteExists:
		merged := *remote
		return cellClassification{outcome: outcomeAdopted, merged: &merged}

	// Brand-new on local only.
	case !pulledExists && localExists && !remoteExists:
		merged := *local
		return cellClassification{outcome: outcomePreserved, merged: &merged}

	// Absent everywhere (defensive — caller shouldn't pass this key).
	case !pulledExists && !localExists && !remoteExists:
		return cellClassification{outcome: outcomeDropped}
	}

	// Conflict + edge-case branches handled in later tasks.
	return cellClassification{outcome: outcomeNone}
}
```

- [ ] **Step 2.4 — Run tests to verify pass**

Run: `go test ./internal/svc/draftsvc/ -run TestClassifyCell_HappyPaths -v`
Expected: PASS (all 6 sub-tests).

- [ ] **Step 2.5 — Commit**

```bash
git add internal/svc/draftsvc/refresh.go internal/svc/draftsvc/refresh_test.go
git commit -m "feat(refresh): cell classifier — happy paths (untouched/adopt/preserve/resolve)"
```

---

## Task 3: Cell classifier — abort-conflict paths

Add the conflict branches: both-sides-changed-different, edit vs delete, cleared vs modified, conflicting adds.

**Files:**
- Modify: `internal/svc/draftsvc/refresh.go`
- Modify: `internal/svc/draftsvc/refresh_test.go`

- [ ] **Step 3.1 — Failing tests for abort conflicts**

Append to `internal/svc/draftsvc/refresh_test.go`:

```go
func TestClassifyCell_AbortConflicts(t *testing.T) {
	pulled := cell(4, 100)
	localEdit := cell(6, 100)
	remoteEdit := cell(8, 100)
	cleared := cell(0, 100) // hours=0 but sourceEntryID kept = "delete on push"
	addLocal := cell(3, 0)
	addRemoteSame := cell(3, 555) // remote added same target/day independently
	addRemoteDifferent := cell(5, 555)

	tcs := []classifyCellTC{
		{
			name:         "both changed, different values -> conflict",
			pulled:       &pulled, local: &localEdit, remote: &remoteEdit,
			strategy:     StrategyAbort,
			wantOutcome:  outcomeNone, // abort reports conflict, no merged cell
			wantConflict: true,
		},
		{
			name:         "local edited, remote deleted -> conflict",
			pulled:       &pulled, local: &localEdit, remote: nil,
			strategy:     StrategyAbort,
			wantOutcome:  outcomeNone,
			wantConflict: true,
		},
		{
			name:         "local cleared, remote modified -> conflict",
			pulled:       &pulled, local: &cleared, remote: &remoteEdit,
			strategy:     StrategyAbort,
			wantOutcome:  outcomeNone,
			wantConflict: true,
		},
		{
			name:         "both added different rows-on-same-key -> conflict",
			pulled:       nil, local: &addLocal, remote: &addRemoteDifferent,
			strategy:     StrategyAbort,
			wantOutcome:  outcomeNone,
			wantConflict: true,
		},
		{
			name:          "both added same hours -> resolved (converged add)",
			pulled:        nil, local: &addLocal, remote: &addRemoteSame,
			strategy:      StrategyAbort,
			wantOutcome:   outcomeResolved,
			wantMergeCell: &addRemoteSame, // adopt remote's sourceEntryID
		},
	}
	runClassifyCellTCs(t, tcs)
}
```

- [ ] **Step 3.2 — Run test to verify it fails**

Run: `go test ./internal/svc/draftsvc/ -run TestClassifyCell_AbortConflicts -v`
Expected: FAIL — current classifier returns `outcomeNone` with no conflict struct for these cases.

- [ ] **Step 3.3 — Add conflict branches to classifyCell**

Replace the classifyCell function in `internal/svc/draftsvc/refresh.go` with:

```go
func classifyCell(pulled, local, remote *domain.DraftCell, strategy Strategy) cellClassification {
	pulledExists := cellPresent(pulled)
	localExists := cellPresent(local)
	remoteExists := cellPresent(remote)
	pulledExistedRaw := pulled != nil && pulled.SourceEntryID != 0
	localCleared := pulledExistedRaw && local != nil && local.Hours == 0 && local.SourceEntryID != 0

	switch {
	// All three present.
	case pulledExists && localExists && remoteExists:
		localUnchanged := cellEqual(*pulled, *local)
		remoteUnchanged := cellEqual(*pulled, *remote)
		if localUnchanged && remoteUnchanged {
			merged := *local
			return cellClassification{outcome: outcomeUntouched, merged: &merged}
		}
		if localUnchanged && !remoteUnchanged {
			merged := *remote
			return cellClassification{outcome: outcomeAdopted, merged: &merged}
		}
		if !localUnchanged && remoteUnchanged {
			merged := *local
			return cellClassification{outcome: outcomePreserved, merged: &merged}
		}
		if cellEqual(*local, *remote) {
			merged := *local
			return cellClassification{outcome: outcomeResolved, merged: &merged}
		}
		return makeConflict(local, remote)

	// Local cleared (delete-on-push), remote modified.
	case localCleared && remoteExists && !cellEqual(*pulled, *remote):
		return makeConflict(local, remote)

	// Local edited (still has hours), remote deleted.
	case pulledExistedRaw && localExists && local.Hours > 0 && !remoteExists:
		if cellEqual(*pulled, *local) {
			// Local unchanged, remote deleted -> stale source. Task 5 handles this branch.
			return cellClassification{outcome: outcomeNone}
		}
		return makeConflict(local, remote)

	// Both sides added independently (no pulled cell).
	case !pulledExists && localExists && remoteExists:
		if cellEqual(*local, *remote) {
			merged := *remote // adopt remote: it has the real sourceEntryID
			return cellClassification{outcome: outcomeResolved, merged: &merged}
		}
		return makeConflict(local, remote)

	// Cell exists only on remote (Task 2 already covers this).
	case !pulledExists && !localExists && remoteExists:
		merged := *remote
		return cellClassification{outcome: outcomeAdopted, merged: &merged}

	// Cell exists only on local (Task 2 already covers this).
	case !pulledExists && localExists && !remoteExists:
		merged := *local
		return cellClassification{outcome: outcomePreserved, merged: &merged}

	case !pulledExists && !localExists && !remoteExists:
		return cellClassification{outcome: outcomeDropped}
	}

	return cellClassification{outcome: outcomeNone}
}

// makeConflict builds an abort-strategy conflict result from local + remote
// cell pointers. Description summaries are produced by describeIntent.
func makeConflict(local, remote *domain.DraftCell) cellClassification {
	return cellClassification{
		outcome: outcomeNone,
		conflict: &MergeConflict{
			LocalDescription:  describeIntent(local),
			RemoteDescription: describeIntent(remote),
			// RowID and Day are filled in by classify() (the per-row caller).
		},
	}
}

// describeIntent renders a one-line summary of a cell's role in the merge.
// nil = absent; hours==0 with sourceEntryID set = "cleared"; otherwise the
// hours value.
func describeIntent(c *domain.DraftCell) string {
	if c == nil {
		return "deleted on remote"
	}
	if c.Hours == 0 && c.SourceEntryID != 0 {
		return "cleared (delete on push)"
	}
	return fmt.Sprintf("updated to %.1fh", c.Hours)
}
```

- [ ] **Step 3.4 — Run tests to verify pass**

Run: `go test ./internal/svc/draftsvc/ -run 'TestClassifyCell_HappyPaths|TestClassifyCell_AbortConflicts' -v`
Expected: PASS for all 11 sub-tests across both top-level tests.

- [ ] **Step 3.5 — Commit**

```bash
git add internal/svc/draftsvc/refresh.go internal/svc/draftsvc/refresh_test.go
git commit -m "feat(refresh): classifier — abort-conflict branches + intent summaries"
```

---

## Task 4: Cell classifier — strategy ours/theirs resolution

Under `--strategy ours`, every conflict resolves to local. Under `--strategy theirs`, every conflict resolves to remote. The classifier needs to know its strategy at call time and emit `outcomeResolvedByStrategy` instead of returning a conflict struct.

**Files:**
- Modify: `internal/svc/draftsvc/refresh.go`
- Modify: `internal/svc/draftsvc/refresh_test.go`

- [ ] **Step 4.1 — Failing tests for ours/theirs resolution**

Append to `internal/svc/draftsvc/refresh_test.go`:

```go
func TestClassifyCell_StrategyOursResolves(t *testing.T) {
	pulled := cell(4, 100)
	localEdit := cell(6, 100)
	remoteEdit := cell(8, 100)

	res := classifyCell(&pulled, &localEdit, &remoteEdit, StrategyOurs)
	require.Equal(t, outcomeResolvedByStrategy, res.outcome)
	require.Nil(t, res.conflict)
	require.NotNil(t, res.merged)
	require.Equal(t, 6.0, res.merged.Hours, "ours keeps local")
}

func TestClassifyCell_StrategyTheirsResolves(t *testing.T) {
	pulled := cell(4, 100)
	localEdit := cell(6, 100)
	remoteEdit := cell(8, 100)

	res := classifyCell(&pulled, &localEdit, &remoteEdit, StrategyTheirs)
	require.Equal(t, outcomeResolvedByStrategy, res.outcome)
	require.Nil(t, res.conflict)
	require.NotNil(t, res.merged)
	require.Equal(t, 8.0, res.merged.Hours, "theirs takes remote")
}

func TestClassifyCell_StrategyOursOnEditVsDelete(t *testing.T) {
	pulled := cell(4, 100)
	localEdit := cell(6, 100)

	res := classifyCell(&pulled, &localEdit, nil, StrategyOurs)
	require.Equal(t, outcomeResolvedByStrategy, res.outcome)
	require.NotNil(t, res.merged)
	require.Equal(t, 6.0, res.merged.Hours)
}

func TestClassifyCell_StrategyTheirsOnEditVsDelete(t *testing.T) {
	pulled := cell(4, 100)
	localEdit := cell(6, 100)

	res := classifyCell(&pulled, &localEdit, nil, StrategyTheirs)
	require.Equal(t, outcomeResolvedByStrategy, res.outcome)
	require.Nil(t, res.merged, "theirs accepts the remote delete")
}

func TestClassifyCell_StrategyOursOnClearedVsModified(t *testing.T) {
	pulled := cell(4, 100)
	cleared := cell(0, 100)
	remoteEdit := cell(8, 100)

	res := classifyCell(&pulled, &cleared, &remoteEdit, StrategyOurs)
	require.Equal(t, outcomeResolvedByStrategy, res.outcome)
	require.NotNil(t, res.merged)
	require.Equal(t, 0.0, res.merged.Hours)
	require.Equal(t, 100, res.merged.SourceEntryID, "still flagged for delete")
}

func TestClassifyCell_StrategyTheirsOnClearedVsModified(t *testing.T) {
	pulled := cell(4, 100)
	cleared := cell(0, 100)
	remoteEdit := cell(8, 100)

	res := classifyCell(&pulled, &cleared, &remoteEdit, StrategyTheirs)
	require.Equal(t, outcomeResolvedByStrategy, res.outcome)
	require.NotNil(t, res.merged)
	require.Equal(t, 8.0, res.merged.Hours, "theirs takes the remote modification")
}
```

- [ ] **Step 4.2 — Run tests to verify they fail**

Run: `go test ./internal/svc/draftsvc/ -run 'TestClassifyCell_Strategy' -v`
Expected: FAIL — current code emits a conflict struct for these cases regardless of strategy.

- [ ] **Step 4.3 — Wire strategy into makeConflict**

Replace `makeConflict` in `internal/svc/draftsvc/refresh.go` with a strategy-aware version, and update the four call sites in `classifyCell` to pass `strategy` through:

```go
// makeConflict resolves a conflict according to strategy. Under StrategyAbort
// it returns a conflict struct (no merged cell). Under StrategyOurs it
// resolves to local. Under StrategyTheirs it resolves to remote.
func makeConflict(local, remote *domain.DraftCell, strategy Strategy) cellClassification {
	switch strategy {
	case StrategyOurs:
		var merged *domain.DraftCell
		if local != nil {
			c := *local
			merged = &c
		}
		return cellClassification{outcome: outcomeResolvedByStrategy, merged: merged}
	case StrategyTheirs:
		var merged *domain.DraftCell
		if remote != nil {
			c := *remote
			merged = &c
		}
		return cellClassification{outcome: outcomeResolvedByStrategy, merged: merged}
	default: // StrategyAbort or unset
		return cellClassification{
			outcome: outcomeNone,
			conflict: &MergeConflict{
				LocalDescription:  describeIntent(local),
				RemoteDescription: describeIntent(remote),
			},
		}
	}
}
```

Then update the four call sites in `classifyCell` from `return makeConflict(local, remote)` to `return makeConflict(local, remote, strategy)`.

- [ ] **Step 4.4 — Run tests to verify pass**

Run: `go test ./internal/svc/draftsvc/ -run 'TestClassifyCell_' -v`
Expected: PASS for all 17 sub-tests across all four `TestClassifyCell_*` tests.

- [ ] **Step 4.5 — Commit**

```bash
git add internal/svc/draftsvc/refresh.go internal/svc/draftsvc/refresh_test.go
git commit -m "feat(refresh): classifier — strategy ours/theirs resolve conflicts inline"
```

---

## Task 5: Cell classifier — row-level edges (stale source, cleared+deleted)

Two remaining rows from §4 of the spec: **stale source** (local unchanged, remote deleted) and **cleared+deleted** (local cleared, remote deleted = silent reality match — cell drops out).

**Files:**
- Modify: `internal/svc/draftsvc/refresh.go`
- Modify: `internal/svc/draftsvc/refresh_test.go`

- [ ] **Step 5.1 — Failing tests for stale source + cleared+deleted**

Append to `internal/svc/draftsvc/refresh_test.go`:

```go
func TestClassifyCell_StaleSource(t *testing.T) {
	pulled := cell(4, 100)
	local := cell(4, 100) // unchanged from pull
	res := classifyCell(&pulled, &local, nil, StrategyAbort)
	require.Equal(t, outcomeAdopted, res.outcome,
		"local unchanged + remote deleted = adopt remote (clear sourceID, mark as added)")
	require.NotNil(t, res.merged)
	require.Equal(t, 4.0, res.merged.Hours, "hours preserved")
	require.Equal(t, 0, res.merged.SourceEntryID, "sourceID cleared (re-add on next push)")
}

func TestClassifyCell_ClearedAndDeleted_DropsOut(t *testing.T) {
	pulled := cell(4, 100)
	cleared := cell(0, 100)
	res := classifyCell(&pulled, &cleared, nil, StrategyAbort)
	require.Equal(t, outcomeDropped, res.outcome,
		"local intent (delete) and remote reality (already deleted) match -> drop")
	require.Nil(t, res.merged)
	require.Nil(t, res.conflict)
}
```

- [ ] **Step 5.2 — Run tests to verify they fail**

Run: `go test ./internal/svc/draftsvc/ -run 'TestClassifyCell_StaleSource|TestClassifyCell_ClearedAndDeleted_DropsOut' -v`
Expected: FAIL — both currently return outcomeNone.

- [ ] **Step 5.3 — Add stale-source and cleared+deleted branches**

In `internal/svc/draftsvc/refresh.go`, replace the existing "Local edited (still has hours), remote deleted" case in `classifyCell` with the expanded version below, and add the new cleared+deleted case before it:

```go
	// Local cleared, remote already deleted -> reality matches local intent.
	case pulledExistedRaw && localCleared && !remoteExists:
		return cellClassification{outcome: outcomeDropped}

	// Pulled+local match (unchanged), remote deleted -> stale source.
	// Clear sourceEntryID; cell becomes a fresh local addition that will
	// re-Create on next push. Reconcile already does this; we mirror.
	case pulledExistedRaw && localExists && !remoteExists && cellEqual(*pulled, *local):
		merged := *local
		merged.SourceEntryID = 0
		return cellClassification{outcome: outcomeAdopted, merged: &merged}

	// Local edited (hours changed), remote deleted -> conflict.
	case pulledExistedRaw && localExists && !remoteExists && !cellEqual(*pulled, *local):
		return makeConflict(local, remote, strategy)
```

Remove the now-redundant standalone "Local edited (still has hours), remote deleted" block that this replaces.

- [ ] **Step 5.4 — Run tests to verify pass**

Run: `go test ./internal/svc/draftsvc/ -run 'TestClassifyCell_' -v`
Expected: PASS for all sub-tests including the two new ones.

- [ ] **Step 5.5 — Commit**

```bash
git add internal/svc/draftsvc/refresh.go internal/svc/draftsvc/refresh_test.go
git commit -m "feat(refresh): classifier — stale-source + cleared-and-deleted branches"
```

---

## Task 6: Per-row classifier wrapper

`classifyCell` operates on one cell. The merge engine needs a per-row driver that walks the union of weekday keys for one row and emits a slice of merged cells + per-row conflicts (with `RowID` and `Day` filled in).

**Files:**
- Modify: `internal/svc/draftsvc/refresh.go`
- Modify: `internal/svc/draftsvc/refresh_test.go`

- [ ] **Step 6.1 — Failing test for classifyRow**

Append to `internal/svc/draftsvc/refresh_test.go`:

```go
func TestClassifyRow_MixedOutcomesPerRow(t *testing.T) {
	row := domain.DraftRow{
		ID: "row-01",
		Cells: []domain.DraftCell{
			{Day: time.Monday, Hours: 6, SourceEntryID: 100},  // edited locally
			{Day: time.Tuesday, Hours: 4, SourceEntryID: 101}, // unchanged
		},
	}
	pulled := domain.DraftRow{
		ID: "row-01",
		Cells: []domain.DraftCell{
			{Day: time.Monday, Hours: 4, SourceEntryID: 100},
			{Day: time.Tuesday, Hours: 4, SourceEntryID: 101},
		},
	}
	remote := domain.DraftRow{
		ID: "row-01",
		Cells: []domain.DraftCell{
			{Day: time.Monday, Hours: 4, SourceEntryID: 100},   // unchanged
			{Day: time.Tuesday, Hours: 8, SourceEntryID: 101},  // edited remotely
			{Day: time.Wednesday, Hours: 3, SourceEntryID: 102}, // added remotely
		},
	}

	merged, counts, conflicts := classifyRow("row-01", &pulled, &row, &remote, StrategyAbort)
	require.Empty(t, conflicts)
	require.Equal(t, 1, counts.adopted, "Tue should be adopted from remote")
	require.Equal(t, 1, counts.preserved, "Mon should be preserved (local edit)")
	require.Equal(t, 0, counts.resolved)
	// Untouched cells aren't counted in the result struct, but they still
	// flow through merged.
	require.Len(t, merged, 3, "Mon (kept) + Tue (adopted) + Wed (added remote)")
}

func TestClassifyRow_ConflictCarriesRowAndDay(t *testing.T) {
	row := domain.DraftRow{
		ID: "row-01",
		Cells: []domain.DraftCell{
			{Day: time.Monday, Hours: 6, SourceEntryID: 100},
		},
	}
	pulled := domain.DraftRow{
		ID: "row-01",
		Cells: []domain.DraftCell{
			{Day: time.Monday, Hours: 4, SourceEntryID: 100},
		},
	}
	remote := domain.DraftRow{
		ID: "row-01",
		Cells: []domain.DraftCell{
			{Day: time.Monday, Hours: 8, SourceEntryID: 100},
		},
	}

	_, _, conflicts := classifyRow("row-01", &pulled, &row, &remote, StrategyAbort)
	require.Len(t, conflicts, 1)
	require.Equal(t, "row-01", conflicts[0].RowID)
	require.Equal(t, "Monday", conflicts[0].Day)
	require.Equal(t, "updated to 6.0h", conflicts[0].LocalDescription)
	require.Equal(t, "updated to 8.0h", conflicts[0].RemoteDescription)
}
```

- [ ] **Step 6.2 — Run test to verify it fails**

Run: `go test ./internal/svc/draftsvc/ -run 'TestClassifyRow_' -v`
Expected: FAIL with "undefined: classifyRow".

- [ ] **Step 6.3 — Implement classifyRow**

Append to `internal/svc/draftsvc/refresh.go`:

```go
// rowCounts accumulates outcome counts for one classifyRow call.
type rowCounts struct {
	adopted, preserved, resolved, resolvedByStrategy int
}

// classifyRow drives classifyCell across the union of weekday keys present
// in any of the three views for one rowID. Returns the merged cell slice
// (sorted by weekday), per-row counts, and any conflicts (with RowID and
// Day populated).
func classifyRow(rowID string, pulled, local, remote *domain.DraftRow, strategy Strategy) ([]domain.DraftCell, rowCounts, []MergeConflict) {
	pulledByDay := cellsByDay(pulled)
	localByDay := cellsByDay(local)
	remoteByDay := cellsByDay(remote)

	days := unionDays(pulledByDay, localByDay, remoteByDay)

	var merged []domain.DraftCell
	var counts rowCounts
	var conflicts []MergeConflict

	for _, day := range days {
		p := dayPtr(pulledByDay, day)
		l := dayPtr(localByDay, day)
		r := dayPtr(remoteByDay, day)
		res := classifyCell(p, l, r, strategy)
		switch res.outcome {
		case outcomeUntouched:
			// no counter; still part of merged
		case outcomeAdopted:
			counts.adopted++
		case outcomePreserved:
			counts.preserved++
		case outcomeResolved:
			counts.resolved++
		case outcomeResolvedByStrategy:
			counts.resolvedByStrategy++
		}
		if res.conflict != nil {
			c := *res.conflict
			c.RowID = rowID
			c.Day = day.String()
			conflicts = append(conflicts, c)
			continue
		}
		if res.merged != nil {
			cell := *res.merged
			cell.Day = day
			merged = append(merged, cell)
		}
	}
	return merged, counts, conflicts
}

func cellsByDay(r *domain.DraftRow) map[time.Weekday]domain.DraftCell {
	out := map[time.Weekday]domain.DraftCell{}
	if r == nil {
		return out
	}
	for _, c := range r.Cells {
		out[c.Day] = c
	}
	return out
}

func dayPtr(m map[time.Weekday]domain.DraftCell, day time.Weekday) *domain.DraftCell {
	if c, ok := m[day]; ok {
		return &c
	}
	return nil
}

func unionDays(a, b, c map[time.Weekday]domain.DraftCell) []time.Weekday {
	seen := map[time.Weekday]struct{}{}
	for d := range a {
		seen[d] = struct{}{}
	}
	for d := range b {
		seen[d] = struct{}{}
	}
	for d := range c {
		seen[d] = struct{}{}
	}
	out := make([]time.Weekday, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
```

Add `sort` and `time` to the import block of `internal/svc/draftsvc/refresh.go`:

```go
import (
	"fmt"
	"sort"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)
```

- [ ] **Step 6.4 — Run tests to verify pass**

Run: `go test ./internal/svc/draftsvc/ -run 'TestClassifyRow_' -v`
Expected: PASS for both sub-tests.

- [ ] **Step 6.5 — Commit**

```bash
git add internal/svc/draftsvc/refresh.go internal/svc/draftsvc/refresh_test.go
git commit -m "feat(refresh): classifyRow walks weekday union, populates conflict identity"
```

---

## Task 7: Whole-draft merge — classify()

`classifyRow` works on one row. `classify()` is the top-level pure function that aligns rows across the three views by row-key (Target+TimeType+Billable), invokes `classifyRow` on each, and assembles a new draft.

**Files:**
- Modify: `internal/svc/draftsvc/refresh.go`
- Modify: `internal/svc/draftsvc/refresh_test.go`

- [ ] **Step 7.1 — Failing tests for classify**

Append to `internal/svc/draftsvc/refresh_test.go`:

```go
// makeRow is a test helper for building DraftRow values with a target/type
// signature that can be matched across views by rowKey().
func makeRow(id string, kind domain.TargetKind, itemID, typeID int, cells ...domain.DraftCell) domain.DraftRow {
	return domain.DraftRow{
		ID:       id,
		Target:   domain.Target{Kind: kind, ItemID: itemID},
		TimeType: domain.TimeType{ID: typeID},
		Cells:    cells,
	}
}

func TestClassify_AdoptsNewRemoteRow(t *testing.T) {
	pulled := domain.WeekDraft{Profile: "p", Name: "default"}
	local := domain.WeekDraft{Profile: "p", Name: "default"}
	remote := domain.WeekDraft{
		Profile: "p", Name: "default",
		Rows: []domain.DraftRow{
			makeRow("row-01", domain.TargetTicket, 555, 17,
				domain.DraftCell{Day: time.Monday, Hours: 4, SourceEntryID: 900}),
		},
	}

	res := classify(pulled, local, remote, StrategyAbort)
	require.False(t, res.aborted)
	require.Empty(t, res.conflicts)
	require.Equal(t, 1, res.counts.adopted)
	require.Len(t, res.rows, 1, "new remote row joins merged set")
	require.Len(t, res.rows[0].Cells, 1)
}

func TestClassify_KeepsLocalOnlyRow(t *testing.T) {
	local := domain.WeekDraft{
		Profile: "p", Name: "default",
		Rows: []domain.DraftRow{
			makeRow("row-localnew", domain.TargetTicket, 777, 19,
				domain.DraftCell{Day: time.Tuesday, Hours: 2}),
		},
	}
	pulled := domain.WeekDraft{Profile: "p", Name: "default"}
	remote := domain.WeekDraft{Profile: "p", Name: "default"}

	res := classify(pulled, local, remote, StrategyAbort)
	require.False(t, res.aborted)
	require.Empty(t, res.conflicts)
	require.Equal(t, 1, res.counts.preserved)
	require.Len(t, res.rows, 1)
	require.Equal(t, "row-localnew", res.rows[0].ID, "local rowID preserved")
}

func TestClassify_AbortsOnConflict(t *testing.T) {
	row := makeRow("row-01", domain.TargetTicket, 555, 17)
	row.Cells = []domain.DraftCell{{Day: time.Monday, Hours: 6, SourceEntryID: 900}}
	rowPulled := makeRow("row-01", domain.TargetTicket, 555, 17)
	rowPulled.Cells = []domain.DraftCell{{Day: time.Monday, Hours: 4, SourceEntryID: 900}}
	rowRemote := makeRow("row-01", domain.TargetTicket, 555, 17)
	rowRemote.Cells = []domain.DraftCell{{Day: time.Monday, Hours: 8, SourceEntryID: 900}}

	res := classify(
		domain.WeekDraft{Rows: []domain.DraftRow{rowPulled}},
		domain.WeekDraft{Rows: []domain.DraftRow{row}},
		domain.WeekDraft{Rows: []domain.DraftRow{rowRemote}},
		StrategyAbort,
	)
	require.True(t, res.aborted)
	require.Len(t, res.conflicts, 1)
	require.Equal(t, "row-01", res.conflicts[0].RowID)
	require.Empty(t, res.rows, "abort means no merged rows")
}

func TestClassify_StrategyOursCollapsesConflict(t *testing.T) {
	rowPulled := makeRow("row-01", domain.TargetTicket, 555, 17,
		domain.DraftCell{Day: time.Monday, Hours: 4, SourceEntryID: 900})
	rowLocal := makeRow("row-01", domain.TargetTicket, 555, 17,
		domain.DraftCell{Day: time.Monday, Hours: 6, SourceEntryID: 900})
	rowRemote := makeRow("row-01", domain.TargetTicket, 555, 17,
		domain.DraftCell{Day: time.Monday, Hours: 8, SourceEntryID: 900})

	res := classify(
		domain.WeekDraft{Rows: []domain.DraftRow{rowPulled}},
		domain.WeekDraft{Rows: []domain.DraftRow{rowLocal}},
		domain.WeekDraft{Rows: []domain.DraftRow{rowRemote}},
		StrategyOurs,
	)
	require.False(t, res.aborted)
	require.Empty(t, res.conflicts)
	require.Equal(t, 1, res.counts.resolvedByStrategy)
	require.Len(t, res.rows, 1)
	require.Equal(t, 6.0, res.rows[0].Cells[0].Hours, "ours kept local 6.0h")
}
```

- [ ] **Step 7.2 — Run test to verify it fails**

Run: `go test ./internal/svc/draftsvc/ -run 'TestClassify_' -v`
Expected: FAIL with "undefined: classify".

- [ ] **Step 7.3 — Implement classify**

Append to `internal/svc/draftsvc/refresh.go`:

```go
// classifyResult bundles a top-level classify() output.
type classifyResult struct {
	rows      []domain.DraftRow
	counts    rowCounts
	conflicts []MergeConflict
	aborted   bool
}

// rowKey is the canonical alignment key across pulled/local/remote views.
// Rows match if and only if their (Target, TimeType, Billable) tuples match.
func rowKey(r domain.DraftRow) string {
	return fmt.Sprintf("%s:%d:%d:%d:%d:%t",
		r.Target.Kind, r.Target.AppID, r.Target.ItemID, r.Target.TaskID,
		r.TimeType.ID, r.Billable)
}

// classify performs the whole-draft three-way merge. It is pure: no I/O.
// Aborted=true with a non-empty conflicts list means "engine refuses; caller
// must not mutate". Under ours/theirs, conflicts is always empty.
func classify(pulled, local, remote domain.WeekDraft, strategy Strategy) classifyResult {
	pulledByKey := indexRows(pulled.Rows)
	localByKey := indexRows(local.Rows)
	remoteByKey := indexRows(remote.Rows)

	keys := unionKeys(pulledByKey, localByKey, remoteByKey)

	out := classifyResult{}
	for _, k := range keys {
		p := pulledByKey[k]
		l := localByKey[k]
		r := remoteByKey[k]

		// Pick a stable rowID: prefer local (user has been editing it),
		// then pulled (matches snapshot), then remote (new row).
		rowID, template := pickRowIdentity(l, p, r)

		merged, counts, conflicts := classifyRow(rowID, p, l, r, strategy)

		out.counts.adopted += counts.adopted
		out.counts.preserved += counts.preserved
		out.counts.resolved += counts.resolved
		out.counts.resolvedByStrategy += counts.resolvedByStrategy
		out.conflicts = append(out.conflicts, conflicts...)

		if len(merged) == 0 {
			continue // entire row drops out
		}
		row := template
		row.ID = rowID
		row.Cells = merged
		out.rows = append(out.rows, row)
	}

	if strategy == StrategyAbort && len(out.conflicts) > 0 {
		out.aborted = true
		out.rows = nil // engine must not produce a merged set under abort
	}
	return out
}

func indexRows(rows []domain.DraftRow) map[string]*domain.DraftRow {
	out := map[string]*domain.DraftRow{}
	for i := range rows {
		k := rowKey(rows[i])
		out[k] = &rows[i]
	}
	return out
}

func unionKeys(maps ...map[string]*domain.DraftRow) []string {
	seen := map[string]struct{}{}
	for _, m := range maps {
		for k := range m {
			seen[k] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// pickRowIdentity selects the rowID and row-template for a merged row.
// Local wins (user has been editing it); falls back to pulled (matches the
// at-pull-time snapshot) and finally remote (brand-new remote row).
func pickRowIdentity(local, pulled, remote *domain.DraftRow) (string, domain.DraftRow) {
	if local != nil {
		return local.ID, *local
	}
	if pulled != nil {
		return pulled.ID, *pulled
	}
	if remote != nil {
		return remote.ID, *remote
	}
	return "", domain.DraftRow{}
}
```

- [ ] **Step 7.4 — Run tests to verify pass**

Run: `go test ./internal/svc/draftsvc/ -run 'TestClassify_' -v`
Expected: PASS for all four sub-tests.

- [ ] **Step 7.5 — Commit**

```bash
git add internal/svc/draftsvc/refresh.go internal/svc/draftsvc/refresh_test.go
git commit -m "feat(refresh): classify() — top-level row alignment + abort gating"
```

---

## Task 8: Service.Refresh — abort path

Wire `classify()` into `Service` with a public `Refresh` method. This task implements the abort path: load the three views, call `classify()`, and on abort return without any disk mutation.

**Files:**
- Modify: `internal/svc/draftsvc/refresh.go`
- Modify: `internal/svc/draftsvc/refresh_test.go`

- [ ] **Step 8.1 — Failing test for Service.Refresh abort path**

Append to `internal/svc/draftsvc/refresh_test.go`. This test reuses helpers from existing service tests (look at `apply_test.go` and `pull_test.go` for the `mockTimeWriter` pattern):

```go
func TestService_Refresh_AbortPath_NoMutation(t *testing.T) {
	tmp := t.TempDir()
	paths := config.Paths{Root: tmp}
	mock := &mockTimeWriter{
		weekRpt: domain.WeekReport{
			WeekRef: domain.WeekRef{StartDate: weekStartTuesday0501()},
			Entries: []domain.TimeEntry{
				{
					ID: 900, Date: time.Date(2026, 5, 4, 0, 0, 0, 0, domain.EasternTZ),
					Minutes: 480, // 8h on remote
					Target:  domain.Target{Kind: domain.TargetTicket, ItemID: 555},
					TimeType: domain.TimeType{ID: 17},
				},
			},
		},
	}
	svc := newServiceWithTimeWriter(paths, mock)

	// Set up: pull (4h), then locally edit to 6h.
	weekStart := weekStartTuesday0501()
	pulledReport := domain.WeekReport{
		WeekRef: domain.WeekRef{StartDate: weekStart},
		Entries: []domain.TimeEntry{
			{
				ID: 900, Date: time.Date(2026, 5, 4, 0, 0, 0, 0, domain.EasternTZ),
				Minutes: 240, // 4h originally pulled
				Target:  domain.Target{Kind: domain.TargetTicket, ItemID: 555},
				TimeType: domain.TimeType{ID: 17},
			},
		},
	}
	mock.weekRpt = pulledReport
	_, err := svc.Pull(context.Background(), "p", weekStart, "default", false)
	require.NoError(t, err)

	// Edit local to 6h.
	d, err := svc.Store().Load("p", weekStart, "default")
	require.NoError(t, err)
	d.Rows[0].Cells[0].Hours = 6
	require.NoError(t, svc.Store().Save(d))

	// Now remote bumps to 8h; refresh under abort.
	mock.weekRpt.Entries[0].Minutes = 480

	res, err := svc.Refresh(context.Background(), "p", weekStart, "default", StrategyAbort)
	require.NoError(t, err)
	require.True(t, res.Aborted)
	require.Len(t, res.Conflicts, 1)
	require.Equal(t, StrategyAbort, res.Strategy)

	// Disk verification: local draft still has 6h.
	post, err := svc.Store().Load("p", weekStart, "default")
	require.NoError(t, err)
	require.Equal(t, 6.0, post.Rows[0].Cells[0].Hours, "abort must not mutate disk")
}

// weekStartTuesday0501 returns the Sunday containing 2026-05-04 (a Tuesday)
// in EasternTZ — i.e. 2026-05-03.
func weekStartTuesday0501() time.Time {
	return time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
}
```

You may need to add `"context"` and `"github.com/iainmoffat/tdx/internal/config"` to the test file's imports.

- [ ] **Step 8.2 — Run test to verify it fails**

Run: `go test ./internal/svc/draftsvc/ -run 'TestService_Refresh_AbortPath' -v`
Expected: FAIL — `svc.Refresh` undefined.

- [ ] **Step 8.3 — Implement Service.Refresh (abort branch only)**

Append to `internal/svc/draftsvc/refresh.go`:

```go
// Refresh fetches the current remote week, performs a three-way merge against
// the at-pull-time watermark and the current local draft, and either updates
// the draft (success / ours / theirs) or aborts (StrategyAbort with conflicts).
//
// On StrategyAbort with conflicts, no disk mutation occurs and the returned
// RefreshResult has Aborted=true.
func (s *Service) Refresh(ctx context.Context, profile string, weekStart time.Time, name string, strategy Strategy) (RefreshResult, error) {
	if name == "" {
		name = "default"
	}
	if err := strategy.Validate(); err != nil {
		return RefreshResult{}, err
	}

	draft, err := s.store.Load(profile, weekStart, name)
	if err != nil {
		return RefreshResult{}, err
	}
	pulled, err := s.store.LoadPulledSnapshot(profile, weekStart, name)
	if err != nil {
		return RefreshResult{}, fmt.Errorf("refresh: load pull watermark: %w (try `tdx time week pull --force` first)", err)
	}

	report, err := s.tsvc.GetWeekReport(ctx, profile, weekStart)
	if err != nil {
		return RefreshResult{}, fmt.Errorf("refresh: fetch remote: %w", err)
	}
	remoteDraft := buildDraftFromReport(profile, name, report)

	res := classify(pulled, draft, remoteDraft, strategy)

	if res.aborted {
		return RefreshResult{
			Strategy:  strategy,
			Aborted:   true,
			Conflicts: res.conflicts,
		}, nil
	}

	// Success branches added in Task 9.
	return RefreshResult{
		Strategy:           strategy,
		Adopted:            res.counts.adopted,
		Preserved:          res.counts.preserved,
		Resolved:           res.counts.resolved,
		ResolvedByStrategy: res.counts.resolvedByStrategy,
	}, nil
}
```

Add `context` to the imports if not already present.

- [ ] **Step 8.4 — Run tests to verify pass**

Run: `go test ./internal/svc/draftsvc/ -run 'TestService_Refresh_AbortPath' -v`
Expected: PASS.

- [ ] **Step 8.5 — Commit**

```bash
git add internal/svc/draftsvc/refresh.go internal/svc/draftsvc/refresh_test.go
git commit -m "feat(refresh): Service.Refresh — abort path returns without mutating"
```

---

## Task 9: Service.Refresh — success path with watermark write

On non-abort outcomes, persist the merged draft and rewrite `.pulled.yaml` to the now-current remote.

**Files:**
- Modify: `internal/svc/draftsvc/refresh.go`
- Modify: `internal/svc/draftsvc/refresh_test.go`

- [ ] **Step 9.1 — Failing tests for success + watermark**

Append to `internal/svc/draftsvc/refresh_test.go`:

```go
func TestService_Refresh_SuccessPath_WritesMergedDraftAndWatermark(t *testing.T) {
	tmp := t.TempDir()
	paths := config.Paths{Root: tmp}
	mock := &mockTimeWriter{}
	svc := newServiceWithTimeWriter(paths, mock)

	weekStart := weekStartTuesday0501()
	// Initial pull: Mon=4h, Tue=4h.
	mock.weekRpt = domain.WeekReport{
		WeekRef: domain.WeekRef{StartDate: weekStart},
		Entries: []domain.TimeEntry{
			{ID: 900, Date: time.Date(2026, 5, 4, 0, 0, 0, 0, domain.EasternTZ), Minutes: 240,
				Target: domain.Target{Kind: domain.TargetTicket, ItemID: 555},
				TimeType: domain.TimeType{ID: 17}},
			{ID: 901, Date: time.Date(2026, 5, 5, 0, 0, 0, 0, domain.EasternTZ), Minutes: 240,
				Target: domain.Target{Kind: domain.TargetTicket, ItemID: 555},
				TimeType: domain.TimeType{ID: 17}},
		},
	}
	_, err := svc.Pull(context.Background(), "p", weekStart, "default", false)
	require.NoError(t, err)

	// User edits Mon to 6h.
	d, _ := svc.Store().Load("p", weekStart, "default")
	for i := range d.Rows[0].Cells {
		if d.Rows[0].Cells[i].Day == time.Monday {
			d.Rows[0].Cells[i].Hours = 6
		}
	}
	require.NoError(t, svc.Store().Save(d))

	// Remote independently bumps Tue to 8h (no conflict, just adopt).
	mock.weekRpt.Entries[1].Minutes = 480

	res, err := svc.Refresh(context.Background(), "p", weekStart, "default", StrategyAbort)
	require.NoError(t, err)
	require.False(t, res.Aborted)
	require.Equal(t, 1, res.Adopted, "Tue adopted from remote")
	require.Equal(t, 1, res.Preserved, "Mon edit preserved")

	// Local draft now has Mon=6 + Tue=8.
	post, _ := svc.Store().Load("p", weekStart, "default")
	cells := map[time.Weekday]float64{}
	for _, c := range post.Rows[0].Cells {
		cells[c.Day] = c.Hours
	}
	require.Equal(t, 6.0, cells[time.Monday])
	require.Equal(t, 8.0, cells[time.Tuesday])

	// Watermark now matches the post-refresh remote: a second refresh is a no-op.
	res2, err := svc.Refresh(context.Background(), "p", weekStart, "default", StrategyAbort)
	require.NoError(t, err)
	require.False(t, res2.Aborted)
	require.Equal(t, 0, res2.Adopted, "second refresh should be a no-op")
	require.Equal(t, 1, res2.Preserved, "Mon edit still local-only relative to new watermark")
}
```

- [ ] **Step 9.2 — Run test to verify it fails**

Run: `go test ./internal/svc/draftsvc/ -run TestService_Refresh_SuccessPath -v`
Expected: FAIL — currently `Service.Refresh` doesn't write merged draft or watermark.

- [ ] **Step 9.3 — Add success-path mutation**

In `internal/svc/draftsvc/refresh.go`, replace the success-branch return at the end of `Refresh` with:

```go
	// Success: assemble the merged draft, save it, refresh the watermark.
	merged := draft
	merged.Rows = res.rows
	merged.Provenance = remoteDraft.Provenance // adopt the new pull-time/fingerprint/status
	merged.Provenance.Kind = draft.Provenance.Kind // preserve original Kind (e.g. ProvenanceFromTemplate)
	if merged.Provenance.Kind == "" {
		merged.Provenance.Kind = domain.ProvenancePulled
	}
	merged.ModifiedAt = time.Now().UTC()

	if err := s.store.Save(merged); err != nil {
		return RefreshResult{}, fmt.Errorf("refresh: save merged draft: %w", err)
	}
	if err := s.store.SavePulledSnapshot(remoteDraft); err != nil {
		return RefreshResult{}, fmt.Errorf("refresh: save watermark: %w", err)
	}
	return RefreshResult{
		Strategy:           strategy,
		Adopted:            res.counts.adopted,
		Preserved:          res.counts.preserved,
		Resolved:           res.counts.resolved,
		ResolvedByStrategy: res.counts.resolvedByStrategy,
	}, nil
```

- [ ] **Step 9.4 — Run tests to verify pass**

Run: `go test ./internal/svc/draftsvc/ -run 'TestService_Refresh' -v`
Expected: PASS.

- [ ] **Step 9.5 — Commit**

```bash
git add internal/svc/draftsvc/refresh.go internal/svc/draftsvc/refresh_test.go
git commit -m "feat(refresh): Service.Refresh — success path writes merged draft + watermark"
```

---

## Task 10: Service.Refresh — pre-refresh snapshot

Snapshot the local draft as `OpPreRefresh` before any disk mutation. Snapshot fires only on the success path (no point snapshotting on abort — nothing's changing).

**Files:**
- Modify: `internal/svc/draftsvc/refresh.go`
- Modify: `internal/svc/draftsvc/refresh_test.go`

- [ ] **Step 10.1 — Failing test for pre-refresh snapshot**

Append to `internal/svc/draftsvc/refresh_test.go`:

```go
func TestService_Refresh_TakesPreRefreshSnapshot(t *testing.T) {
	tmp := t.TempDir()
	paths := config.Paths{Root: tmp}
	mock := &mockTimeWriter{}
	svc := newServiceWithTimeWriter(paths, mock)

	weekStart := weekStartTuesday0501()
	mock.weekRpt = domain.WeekReport{
		WeekRef: domain.WeekRef{StartDate: weekStart},
		Entries: []domain.TimeEntry{
			{ID: 900, Date: time.Date(2026, 5, 4, 0, 0, 0, 0, domain.EasternTZ), Minutes: 240,
				Target: domain.Target{Kind: domain.TargetTicket, ItemID: 555},
				TimeType: domain.TimeType{ID: 17}},
		},
	}
	_, err := svc.Pull(context.Background(), "p", weekStart, "default", false)
	require.NoError(t, err)

	// Edit local to 6h, remote bumps to 8h, refresh ours.
	d, _ := svc.Store().Load("p", weekStart, "default")
	d.Rows[0].Cells[0].Hours = 6
	require.NoError(t, svc.Store().Save(d))
	mock.weekRpt.Entries[0].Minutes = 480

	_, err = svc.Refresh(context.Background(), "p", weekStart, "default", StrategyOurs)
	require.NoError(t, err)

	snaps, err := svc.Snapshots().List("p", weekStart, "default")
	require.NoError(t, err)
	var preRefresh []SnapshotInfo
	for _, s := range snaps {
		if s.Op == OpPreRefresh {
			preRefresh = append(preRefresh, s)
		}
	}
	require.Len(t, preRefresh, 1, "exactly one pre-refresh snapshot")
}

func TestService_Refresh_AbortPath_NoSnapshot(t *testing.T) {
	tmp := t.TempDir()
	paths := config.Paths{Root: tmp}
	mock := &mockTimeWriter{}
	svc := newServiceWithTimeWriter(paths, mock)

	weekStart := weekStartTuesday0501()
	mock.weekRpt = domain.WeekReport{
		WeekRef: domain.WeekRef{StartDate: weekStart},
		Entries: []domain.TimeEntry{
			{ID: 900, Date: time.Date(2026, 5, 4, 0, 0, 0, 0, domain.EasternTZ), Minutes: 240,
				Target: domain.Target{Kind: domain.TargetTicket, ItemID: 555},
				TimeType: domain.TimeType{ID: 17}},
		},
	}
	_, err := svc.Pull(context.Background(), "p", weekStart, "default", false)
	require.NoError(t, err)

	d, _ := svc.Store().Load("p", weekStart, "default")
	d.Rows[0].Cells[0].Hours = 6
	require.NoError(t, svc.Store().Save(d))
	mock.weekRpt.Entries[0].Minutes = 480

	_, err = svc.Refresh(context.Background(), "p", weekStart, "default", StrategyAbort)
	require.NoError(t, err)

	snaps, err := svc.Snapshots().List("p", weekStart, "default")
	require.NoError(t, err)
	for _, s := range snaps {
		require.NotEqual(t, OpPreRefresh, s.Op, "abort must not snapshot")
	}
}
```

- [ ] **Step 10.2 — Run tests to verify they fail**

Run: `go test ./internal/svc/draftsvc/ -run 'TestService_Refresh_TakesPreRefreshSnapshot|TestService_Refresh_AbortPath_NoSnapshot' -v`
Expected: TestService_Refresh_TakesPreRefreshSnapshot FAILs (no snapshot taken yet); TestService_Refresh_AbortPath_NoSnapshot PASSes (no snapshot is taken because we haven't added the call).

- [ ] **Step 10.3 — Add snapshot call before mutation**

In `internal/svc/draftsvc/refresh.go`, immediately before the `s.store.Save(merged)` line, add:

```go
	if _, err := s.snapshots.Take(draft, OpPreRefresh, ""); err != nil {
		return RefreshResult{}, fmt.Errorf("refresh: pre-refresh snapshot: %w", err)
	}
```

- [ ] **Step 10.4 — Run tests to verify pass**

Run: `go test ./internal/svc/draftsvc/ -run 'TestService_Refresh' -v`
Expected: PASS for all four `TestService_Refresh*` tests.

- [ ] **Step 10.5 — Commit**

```bash
git add internal/svc/draftsvc/refresh.go internal/svc/draftsvc/refresh_test.go
git commit -m "feat(refresh): take OpPreRefresh snapshot before mutation"
```

---

## Task 11: `tdx time week refresh` CLI — text output for all three paths

**Files:**
- Create: `internal/cli/time/week/refresh.go`
- Create: `internal/cli/time/week/refresh_test.go`
- Modify: `internal/cli/time/week/week.go`

- [ ] **Step 11.1 — Failing test for refresh CLI command structure**

Create `internal/cli/time/week/refresh_test.go`:

```go
package week

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRefreshCmd_FlagsRegistered(t *testing.T) {
	cmd := newRefreshCmd()
	require.NotNil(t, cmd)
	require.Equal(t, "refresh <date>[/<name>]", cmd.Use)
	require.NotNil(t, cmd.Flags().Lookup("strategy"), "--strategy flag missing")
	require.NotNil(t, cmd.Flags().Lookup("profile"), "--profile flag missing")
	require.NotNil(t, cmd.Flags().Lookup("json"), "--json flag missing")
	def, err := cmd.Flags().GetString("strategy")
	require.NoError(t, err)
	require.Equal(t, "abort", def, "default strategy must be abort")
}
```

- [ ] **Step 11.2 — Run test to verify it fails**

Run: `go test ./internal/cli/time/week/ -run TestNewRefreshCmd_FlagsRegistered -v`
Expected: FAIL — `newRefreshCmd` undefined.

- [ ] **Step 11.3 — Create refresh.go with full text-output behavior**

Create `internal/cli/time/week/refresh.go`:

```go
package week

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

type refreshFlags struct {
	profile  string
	strategy string
	json     bool
}

func newRefreshCmd() *cobra.Command {
	var f refreshFlags
	cmd := &cobra.Command{
		Use:   "refresh <date>[/<name>]",
		Short: "Three-way merge a draft against the latest remote",
		Long: `Refresh re-fetches the live week and merges remote changes into the local draft.

  --strategy abort    (default) refuse to mutate if any cell-level conflict
  --strategy ours     on conflict, keep local
  --strategy theirs   on conflict, take remote

On --strategy abort with conflicts, refresh exits non-zero and prints the
list of conflicts. The local draft is unchanged. Re-run with --strategy ours
or --strategy theirs to proceed, or 'tdx time week reset --yes' to discard
local edits and re-pull.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRefresh(cmd, f, args[0])
		},
	}
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
	cmd.Flags().StringVar(&f.strategy, "strategy", "abort", "abort | ours | theirs")
	cmd.Flags().BoolVar(&f.json, "json", false, "JSON output")
	return cmd
}

func runRefresh(cmd *cobra.Command, f refreshFlags, ref string) error {
	weekStart, name, err := ParseDraftRef(ref)
	if err != nil {
		return err
	}
	strategy := draftsvc.Strategy(f.strategy)
	if err := strategy.Validate(); err != nil {
		return err
	}

	paths, err := config.ResolvePaths()
	if err != nil {
		return err
	}
	auth := authsvc.New(paths)
	tsvc := timesvc.New(paths)
	drafts := draftsvc.NewService(paths, tsvc)

	profileName, err := auth.ResolveProfile(f.profile)
	if err != nil {
		return err
	}

	res, err := drafts.Refresh(cmd.Context(), profileName, weekStart, name, strategy)
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	if f.json {
		return writeRefreshJSON(w, weekStart, name, res)
	}
	if res.Aborted {
		writeRefreshAbortText(w, weekStart, res)
		return fmt.Errorf("refresh aborted: %d conflict(s)", len(res.Conflicts))
	}
	writeRefreshSuccessText(w, res)
	return nil
}

func writeRefreshSuccessText(w io.Writer, res draftsvc.RefreshResult) {
	suffix := ""
	if res.Strategy != draftsvc.StrategyAbort {
		suffix = fmt.Sprintf(" (--strategy %s)", res.Strategy)
	}
	_, _ = fmt.Fprintf(w, "Refresh complete%s.\n", suffix)
	_, _ = fmt.Fprintf(w, "  Adopted (remote -> draft):  %d cells\n", res.Adopted)
	_, _ = fmt.Fprintf(w, "  Preserved (local edits):    %d cells\n", res.Preserved)
	_, _ = fmt.Fprintf(w, "  Resolved (same on both):    %d cells\n", res.Resolved)
	if res.ResolvedByStrategy > 0 {
		who := "local won"
		if res.Strategy == draftsvc.StrategyTheirs {
			who = "remote won"
		}
		_, _ = fmt.Fprintf(w, "  Resolved by --strategy:     %d cells (%s)\n",
			res.ResolvedByStrategy, who)
	}
}

func writeRefreshAbortText(w io.Writer, weekStart time.Time, res draftsvc.RefreshResult) {
	_, _ = fmt.Fprintf(w, "Refresh aborted: %d cell(s) conflict between local edits and remote changes.\n\n",
		len(res.Conflicts))
	for _, c := range res.Conflicts {
		_, _ = fmt.Fprintf(w, "  %s  %s  conflict\n", c.RowID, c.Day[:3])
		_, _ = fmt.Fprintf(w, "    local:   %s\n", c.LocalDescription)
		_, _ = fmt.Fprintf(w, "    remote:  %s\n", c.RemoteDescription)
		_, _ = fmt.Fprintln(w)
	}
	_, _ = fmt.Fprintln(w, "Choose one:")
	_, _ = fmt.Fprintln(w, "  --strategy ours        (keep local for all conflicts; refresh succeeds)")
	_, _ = fmt.Fprintln(w, "  --strategy theirs      (take remote for all conflicts; refresh succeeds)")
	_, _ = fmt.Fprintf(w, "  tdx time week reset %s --yes  (give up local edits entirely, re-pull fresh)\n",
		weekStart.Format("2006-01-02"))
}

func writeRefreshJSON(w io.Writer, weekStart time.Time, name string, res draftsvc.RefreshResult) error {
	// Implemented in Task 13.
	_ = weekStart
	_ = name
	_ = res
	_, _ = io.WriteString(w, "{}\n")
	return nil
}
```

The two call sites in `runRefresh` invoke them as:

```go
		return writeRefreshJSON(w, weekStart, name, res)
		// ...
		writeRefreshAbortText(w, weekStart, res)
```

- [ ] **Step 11.4 — Register refresh command in week.go**

Modify `internal/cli/time/week/week.go` to add the new subcommand. Locate the existing `cmd.AddCommand(newPruneCmd())` line and add immediately after it:

```go
	cmd.AddCommand(newRefreshCmd())
```

- [ ] **Step 11.5 — Run unit test to verify it passes**

Run: `go test ./internal/cli/time/week/ -run TestNewRefreshCmd_FlagsRegistered -v`
Expected: PASS.

- [ ] **Step 11.6 — Run go vet to check the new file compiles**

Run: `go vet ./...`
Expected: clean — no errors.

- [ ] **Step 11.7 — Commit**

```bash
git add internal/cli/time/week/refresh.go internal/cli/time/week/refresh_test.go internal/cli/time/week/week.go
git commit -m "feat(cli): tdx time week refresh — text output for all three strategy paths"
```

---

## Task 12: `tdx time week rebase` alias

`rebase` is a separate cobra subcommand that calls the same `runRefresh`. Identical flags, identical behavior, different name.

**Files:**
- Create: `internal/cli/time/week/rebase.go`
- Modify: `internal/cli/time/week/refresh_test.go`
- Modify: `internal/cli/time/week/week.go`

- [ ] **Step 12.1 — Failing test for rebase alias**

Append to `internal/cli/time/week/refresh_test.go`:

```go
func TestNewRebaseCmd_IsAliasOfRefresh(t *testing.T) {
	cmd := newRebaseCmd()
	require.NotNil(t, cmd)
	require.Equal(t, "rebase <date>[/<name>]", cmd.Use)
	require.NotNil(t, cmd.Flags().Lookup("strategy"))
	require.NotNil(t, cmd.Flags().Lookup("profile"))
	require.NotNil(t, cmd.Flags().Lookup("json"))
	def, err := cmd.Flags().GetString("strategy")
	require.NoError(t, err)
	require.Equal(t, "abort", def)
}
```

- [ ] **Step 12.2 — Run test to verify it fails**

Run: `go test ./internal/cli/time/week/ -run TestNewRebaseCmd_IsAliasOfRefresh -v`
Expected: FAIL — `newRebaseCmd` undefined.

- [ ] **Step 12.3 — Create rebase.go**

Create `internal/cli/time/week/rebase.go`:

```go
package week

import "github.com/spf13/cobra"

// newRebaseCmd is an alias of `tdx time week refresh`. Hardcore git users
// reach for `rebase` reflexively; we accept either name.
func newRebaseCmd() *cobra.Command {
	var f refreshFlags
	cmd := &cobra.Command{
		Use:   "rebase <date>[/<name>]",
		Short: "Alias of `refresh`",
		Long:  `rebase is identical to refresh — same flags, same behavior. See ` + "`tdx time week refresh --help`" + ` for the full description.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRefresh(cmd, f, args[0])
		},
	}
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
	cmd.Flags().StringVar(&f.strategy, "strategy", "abort", "abort | ours | theirs")
	cmd.Flags().BoolVar(&f.json, "json", false, "JSON output")
	return cmd
}
```

- [ ] **Step 12.4 — Register rebase in week.go**

In `internal/cli/time/week/week.go`, add immediately after `cmd.AddCommand(newRefreshCmd())`:

```go
	cmd.AddCommand(newRebaseCmd())
```

- [ ] **Step 12.5 — Run tests to verify pass**

Run: `go test ./internal/cli/time/week/ -run 'TestNewRefreshCmd_FlagsRegistered|TestNewRebaseCmd_IsAliasOfRefresh' -v`
Expected: PASS for both.

- [ ] **Step 12.6 — Commit**

```bash
git add internal/cli/time/week/rebase.go internal/cli/time/week/refresh_test.go internal/cli/time/week/week.go
git commit -m "feat(cli): tdx time week rebase — alias of refresh"
```

---

## Task 13: Refresh `--json` output

Replace the stub `writeRefreshJSON` with the real implementation per spec §5 / §7.

**Files:**
- Modify: `internal/cli/time/week/refresh.go`
- Modify: `internal/cli/time/week/refresh_test.go`

- [ ] **Step 13.1 — Failing test for JSON shape**

Add `"bytes"`, `"encoding/json"`, `"time"`, and `"github.com/iainmoffat/tdx/internal/svc/draftsvc"` to the existing import block of `internal/cli/time/week/refresh_test.go` (do NOT start a second import block — Go forbids that). Then append:

```go
func TestWriteRefreshJSON_AbortShape(t *testing.T) {
	var buf bytes.Buffer
	res := draftsvc.RefreshResult{
		Strategy:  draftsvc.StrategyAbort,
		Aborted:   true,
		Conflicts: []draftsvc.MergeConflict{
			{RowID: "row-01", Day: "Monday", LocalDescription: "updated to 6.0h", RemoteDescription: "updated to 8.0h"},
		},
	}
	err := writeRefreshJSON(&buf, time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC), "default", res)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.Equal(t, "tdx.v1.weekDraftRefreshResult", got["schema"])
	require.Equal(t, "abort", got["strategy"])
	require.Equal(t, true, got["aborted"])
	require.Equal(t, float64(0), got["adopted"])
	require.Equal(t, float64(0), got["preserved"])
	require.Equal(t, float64(0), got["resolved"])
	require.Equal(t, float64(0), got["resolvedByStrategy"])
	conflicts := got["conflicts"].([]any)
	require.Len(t, conflicts, 1)
	c := conflicts[0].(map[string]any)
	require.Equal(t, "row-01", c["row"])
	require.Equal(t, "Monday", c["day"])
	require.Equal(t, "updated to 6.0h", c["local"])
	require.Equal(t, "updated to 8.0h", c["remote"])
}

func TestWriteRefreshJSON_SuccessShape(t *testing.T) {
	var buf bytes.Buffer
	res := draftsvc.RefreshResult{
		Strategy:           draftsvc.StrategyOurs,
		Adopted:            3,
		Preserved:          5,
		Resolved:           1,
		ResolvedByStrategy: 2,
	}
	err := writeRefreshJSON(&buf, time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC), "default", res)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.Equal(t, "ours", got["strategy"])
	require.Equal(t, false, got["aborted"])
	require.Equal(t, float64(3), got["adopted"])
	require.Equal(t, float64(5), got["preserved"])
	require.Equal(t, float64(1), got["resolved"])
	require.Equal(t, float64(2), got["resolvedByStrategy"])
	conflicts, ok := got["conflicts"].([]any)
	require.True(t, ok || got["conflicts"] == nil, "conflicts is array or absent")
	require.Empty(t, conflicts)
}
```

- [ ] **Step 13.2 — Run tests to verify they fail**

Run: `go test ./internal/cli/time/week/ -run 'TestWriteRefreshJSON_' -v`
Expected: FAIL — stub returns `{}`.

- [ ] **Step 13.3 — Replace writeRefreshJSON with real impl**

In `internal/cli/time/week/refresh.go`, replace the stub `writeRefreshJSON` with:

```go
func writeRefreshJSON(w io.Writer, _ time.Time, _ string, res draftsvc.RefreshResult) error {
	type conflictJSON struct {
		Row    string `json:"row"`
		Day    string `json:"day"`
		Local  string `json:"local"`
		Remote string `json:"remote"`
	}
	conflicts := make([]conflictJSON, 0, len(res.Conflicts))
	for _, c := range res.Conflicts {
		conflicts = append(conflicts, conflictJSON{
			Row: c.RowID, Day: c.Day,
			Local: c.LocalDescription, Remote: c.RemoteDescription,
		})
	}
	envelope := struct {
		Schema             string         `json:"schema"`
		Strategy           string         `json:"strategy"`
		Aborted            bool           `json:"aborted"`
		Adopted            int            `json:"adopted"`
		Preserved          int            `json:"preserved"`
		Resolved           int            `json:"resolved"`
		ResolvedByStrategy int            `json:"resolvedByStrategy"`
		Conflicts          []conflictJSON `json:"conflicts"`
	}{
		Schema:             "tdx.v1.weekDraftRefreshResult",
		Strategy:           string(res.Strategy),
		Aborted:            res.Aborted,
		Adopted:            res.Adopted,
		Preserved:          res.Preserved,
		Resolved:           res.Resolved,
		ResolvedByStrategy: res.ResolvedByStrategy,
		Conflicts:          conflicts,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}
```

Add `"encoding/json"` to imports of `refresh.go`.

- [ ] **Step 13.4 — Run tests to verify pass**

Run: `go test ./internal/cli/time/week/ -run 'TestWriteRefreshJSON_' -v`
Expected: PASS for both.

- [ ] **Step 13.5 — Commit**

```bash
git add internal/cli/time/week/refresh.go internal/cli/time/week/refresh_test.go
git commit -m "feat(cli): refresh --json emits tdx.v1.weekDraftRefreshResult envelope"
```

---

## Task 14: MCP `refresh_week_draft` tool

**Files:**
- Modify: `internal/mcp/tools_drafts.go`
- Modify: `internal/mcp/tools_drafts_test.go`
- Modify: `internal/mcp/server_test.go`

- [ ] **Step 14.1 — Failing test: bump tool count to 38**

In `internal/mcp/server_test.go`, change the `wantCount` constant:

```go
const wantCount = 38
```

Run: `go test ./internal/mcp/ -run TestServer -v`
Expected: FAIL — server registers 37 tools, test wants 38.

- [ ] **Step 14.2 — Failing test for refresh_week_draft handler**

Append to `internal/mcp/tools_drafts_test.go` (use the existing test infrastructure as a model — look at `TestResetDraftHandler_RequiresConfirm` if present, otherwise the closest mutating-tool test):

```go
func TestRefreshDraftHandler_RequiresConfirm(t *testing.T) {
	srv, _ := newTestServerWithDrafts(t)
	res, err := callTool(t, srv, "refresh_week_draft", map[string]any{
		"weekStart": "2026-05-03",
		"strategy":  "abort",
		// confirm omitted intentionally
	})
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, textOf(res), "confirm=true")
}

func TestRefreshDraftHandler_RejectsUnknownStrategy(t *testing.T) {
	srv, _ := newTestServerWithDrafts(t)
	res, err := callTool(t, srv, "refresh_week_draft", map[string]any{
		"weekStart": "2026-05-03",
		"strategy":  "merge",
		"confirm":   true,
	})
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, textOf(res), "unknown refresh strategy")
}
```

(If helper names differ in `tools_drafts_test.go`, mirror the existing patterns. The intent is: confirm gate + strategy validation gate.)

- [ ] **Step 14.3 — Run tests to verify they fail**

Run: `go test ./internal/mcp/ -run 'TestRefreshDraft' -v`
Expected: FAIL — `refresh_week_draft` not registered.

- [ ] **Step 14.4 — Add the tool registration**

In `internal/mcp/tools_drafts.go`, add a new args struct near the other mutating-tool arg types (e.g. just below `resetDraftArgs`):

```go
type refreshDraftArgs struct {
	Profile   string `json:"profile,omitempty"`
	WeekStart string `json:"weekStart"`
	Name      string `json:"name,omitempty"`
	Strategy  string `json:"strategy,omitempty" jsonschema:"abort | ours | theirs (default abort)"`
	Confirm   bool   `json:"confirm"`
}
```

Inside `RegisterDraftMutatingTools`, add the registration block immediately after `resetDraftHandler`:

```go
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name: "refresh_week_draft",
		Description: `Refresh a draft against the current remote. Three-way merge between
at-pull-time / current-local / current-remote.

strategy: abort (default) - refuse to mutate if any cell-level conflict
          ours              - on conflict, keep local
          theirs            - on conflict, take remote

On strategy=abort with conflicts, returns a successful tool result with
aborted=true and a conflicts[] list. Agent can re-call with strategy=ours
or strategy=theirs after surfacing the conflicts to the user.

Requires confirm=true.`,
	}, refreshDraftHandler(svcs))
```

Then add the handler at the bottom of the file:

```go
func refreshDraftHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, refreshDraftArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args refreshDraftArgs) (*sdkmcp.CallToolResult, any, error) {
		if r, ok := confirmGate(args.Confirm, "Set confirm=true to refresh the draft."); !ok {
			return r, nil, nil
		}
		strategy := draftsvc.Strategy(args.Strategy)
		if strategy == "" {
			strategy = draftsvc.StrategyAbort
		}
		if err := strategy.Validate(); err != nil {
			return errorResult(err.Error()), nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)
		weekStart, err := parseWeekStart(args.WeekStart)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid weekStart: %v", err)), nil, nil
		}
		name := args.Name
		if name == "" {
			name = "default"
		}
		res, err := svcs.Drafts.Refresh(ctx, profile, weekStart, name, strategy)
		if err != nil {
			return errorResult(fmt.Sprintf("refresh: %v", err)), nil, nil
		}
		type conflictJSON struct {
			Row    string `json:"row"`
			Day    string `json:"day"`
			Local  string `json:"local"`
			Remote string `json:"remote"`
		}
		conflicts := make([]conflictJSON, 0, len(res.Conflicts))
		for _, c := range res.Conflicts {
			conflicts = append(conflicts, conflictJSON{
				Row: c.RowID, Day: c.Day,
				Local: c.LocalDescription, Remote: c.RemoteDescription,
			})
		}
		return jsonResult(struct {
			Schema             string         `json:"schema"`
			Strategy           string         `json:"strategy"`
			Aborted            bool           `json:"aborted"`
			Adopted            int            `json:"adopted"`
			Preserved          int            `json:"preserved"`
			Resolved           int            `json:"resolved"`
			ResolvedByStrategy int            `json:"resolvedByStrategy"`
			Conflicts          []conflictJSON `json:"conflicts"`
		}{
			Schema:             "tdx.v1.weekDraftRefreshResult",
			Strategy:           string(res.Strategy),
			Aborted:            res.Aborted,
			Adopted:            res.Adopted,
			Preserved:          res.Preserved,
			Resolved:           res.Resolved,
			ResolvedByStrategy: res.ResolvedByStrategy,
			Conflicts:          conflicts,
		})
	}
}
```

- [ ] **Step 14.5 — Run tests to verify pass**

Run: `go test ./internal/mcp/ -v`
Expected: PASS — `wantCount=38` is now correct, `refresh_week_draft` confirm-gate and strategy-validation tests both pass.

- [ ] **Step 14.6 — Commit**

```bash
git add internal/mcp/tools_drafts.go internal/mcp/tools_drafts_test.go internal/mcp/server_test.go
git commit -m "feat(mcp): refresh_week_draft tool (37 -> 38 tools)"
```

---

## Task 15: README + guide.md

**Files:**
- Modify: `README.md`
- Modify: `docs/guide.md`

- [ ] **Step 15.1 — Add refresh + rebase to README's Time Week Drafts table**

Locate the Time Week Drafts table in `README.md`. After the row for `tdx time week reset` (or wherever the existing draft commands are listed), add two rows:

```
| `tdx time week refresh <date>[/<name>]` | three-way merge against current remote (--strategy abort\|ours\|theirs) |
| `tdx time week rebase <date>[/<name>]` | alias of refresh (same flags, same behavior) |
```

If the README has an MCP tool table, locate it and add:

```
| `refresh_week_draft` | mutating | three-way merge a draft against current remote; supports strategy abort\|ours\|theirs |
```

If the README has a JSON-schema list, add `tdx.v1.weekDraftRefreshResult` to it.

- [ ] **Step 15.2 — Add Refresh & rebase subsection to docs/guide.md**

In `docs/guide.md`, locate the "Week drafts" section (or its closest equivalent — search for "draft"). After the "Reset" subsection, append:

````markdown
### Refresh & rebase

`tdx time week refresh <date>[/<name>]` re-fetches the live week and merges
remote changes into the local draft using a three-way merge between three
views:

- **at-pull-time** — what the live week looked like when the draft was
  created (from the `.pulled.yaml` watermark).
- **current-local** — what the draft contains right now.
- **current-remote** — what the live week contains right now.

Each cell is classified per the [merge rules](../specs/2026-04-28-tdx-phase-B2a-design.md#4-cell-level-merge-rules)
and one of three things happens:

- **Auto-merge** — local-only and remote-only changes both apply.
- **Conflict + strategy** — both sides changed the same cell; behavior
  depends on `--strategy`.
- **Reality match** — local intent (e.g. cleared) and remote state
  (already deleted) agree; the cell drops out silently.

#### Strategies

```
--strategy abort    (default) refuse to mutate if any cell-level conflict
--strategy ours     on conflict, keep local
--strategy theirs   on conflict, take remote
```

The `rebase` command is a verbatim alias of `refresh` for hardcore git
users — same flags, same behavior.

#### Worked example

You pulled the week, edited Monday from 4.0h to 6.0h. Meanwhile a coworker
edited the same row's Monday on TD to 8.0h:

```
$ tdx time week refresh 2026-05-03
Refresh aborted: 1 cell(s) conflict between local edits and remote changes.

  row-01  Mon  conflict
    local:   updated to 6.0h
    remote:  updated to 8.0h

Choose one:
  --strategy ours        (keep local for all conflicts; refresh succeeds)
  --strategy theirs      (take remote for all conflicts; refresh succeeds)
  tdx time week reset 2026-05-03 --yes  (give up local edits entirely, re-pull fresh)
```

Decide what you want and re-run with the strategy:

```
$ tdx time week refresh 2026-05-03 --strategy ours
Refresh complete (--strategy ours).
  Adopted (remote -> draft):  0 cells
  Preserved (local edits):    0 cells
  Resolved (same on both):    0 cells
  Resolved by --strategy:     1 cells (local won)
```

A snapshot tagged `pre-refresh` is taken before any disk mutation. To roll
back: `tdx time week history 2026-05-03` and `tdx time week restore --snapshot N --yes`.
````

- [ ] **Step 15.3 — Sanity check by building and running --help**

Run:

```bash
go build -o /tmp/tdx ./cmd/tdx && /tmp/tdx time week refresh --help
```

Expected: usage text appears with `--strategy abort | ours | theirs` flag visible.

- [ ] **Step 15.4 — Commit**

```bash
git add README.md docs/guide.md
git commit -m "docs: README + guide.md — refresh / rebase + tdx.v1.weekDraftRefreshResult"
```

---

## Task 16: Manual walkthrough doc

**Files:**
- Create: `docs/manual-tests/phase-B2a-week-drafts-refresh-walkthrough.md`

- [ ] **Step 16.1 — Create the walkthrough doc**

Write `docs/manual-tests/phase-B2a-week-drafts-refresh-walkthrough.md`:

````markdown
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
````

- [ ] **Step 16.2 — Commit**

```bash
git add docs/manual-tests/phase-B2a-week-drafts-refresh-walkthrough.md
git commit -m "docs: Phase B.2a manual walkthrough"
```

---

## Task 17: Final verification + version bump + tag

**Files:**
- Modify: `internal/version/version.go` (or wherever the version constant lives — verify with `grep -rn 'Version =' internal/`)

- [ ] **Step 17.1 — Run full quality gate**

```bash
go test ./... && go vet ./... && gofmt -l . && golangci-lint run ./...
```

Expected: all green. `gofmt -l .` should print nothing. If `golangci-lint` flags anything, fix inline before continuing.

- [ ] **Step 17.2 — Bump version to v0.6.0**

Find the version constant. Most likely it's in `internal/version/version.go`. If the file exists, change:

```go
const Version = "0.5.0"
```

to:

```go
const Version = "0.6.0"
```

If the file doesn't exist, search for `0.5.0` or `0.5` strings:

```bash
grep -rn "0\.5\.0" --include="*.go"
```

and update wherever the version is defined. Commit:

```bash
git add internal/version/version.go  # adjust path as needed
git commit -m "chore: bump version to 0.6.0"
```

- [ ] **Step 17.3 — Push branch + open PR**

```bash
git push -u origin phase-B2a-refresh-merge
gh pr create --title "Phase B.2a — Week Drafts refresh/rebase merge engine" --body "$(cat <<'EOF'
## Summary
- Three-way merge engine between at-pull-time / current-local / current-remote
- New CLI: tdx time week refresh / rebase --strategy abort | ours | theirs (default abort)
- New MCP tool: refresh_week_draft (37 -> 38 tools)
- New JSON schema: tdx.v1.weekDraftRefreshResult
- Pre-refresh snapshot via existing OpPreRefresh tag
- Surface strategy + conflicted state machine deferred to B.2b

## Test plan
- [ ] `go test ./... && go vet ./... && golangci-lint run ./...`
- [ ] Manual walkthrough: `docs/manual-tests/phase-B2a-week-drafts-refresh-walkthrough.md`
EOF
)"
```

- [ ] **Step 17.4 — After PR merged: tag and release**

Once the squash-merge lands on main:

```bash
git checkout main
git pull
git tag -a v0.6.0 -m "Phase B.2a — week-draft refresh/rebase merge engine"
git push origin v0.6.0
```

Goreleaser triggers the release; Homebrew tap updates automatically per the
project's existing release pipeline. Verify the GitHub release page shows
binaries for darwin/linux/windows × amd64/arm64.

---

## Notes for the implementer

- **Strict TDD throughout.** Every Task has a test step that must fail
  before the implementation step.
- **Never amend commits.** If a commit is wrong, make a new fix-up commit
  and let the PR squash-merge collapse them.
- **Don't run `go mod tidy`.** This phase introduces zero new deps.
- **Watermark consistency is load-bearing.** Task 9's "second refresh is a
  no-op" assertion is the canary for the watermark write — if you
  accidentally drop the `s.store.SavePulledSnapshot(remoteDraft)` line,
  the next refresh will see all the just-merged remote changes as
  fresh remote diffs and either re-adopt them or abort. The test catches
  this; do not weaken or skip it.
- **Pre-refresh snapshot fires only on success.** Task 10's
  `TestService_Refresh_AbortPath_NoSnapshot` test enforces this. Aborts
  must not leave stray snapshots — they'd make `tdx time week history`
  noisy with no-op entries.
- **The classifier is the densest piece of code in the iteration.** If a
  test from Tasks 2–7 fails in a way that's hard to localize, the issue
  is almost always in the `classifyCell` switch ordering. Read the §4
  table in the spec carefully; map each row to its classifier branch.
- **Task 14's MCP test helpers (`callTool`, `textOf`, `newTestServerWithDrafts`)
  may have different names** in the actual codebase. Mirror whatever
  pattern existing mutating-tool tests (e.g. for `reset_week_draft`) use.
