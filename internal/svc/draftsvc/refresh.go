package draftsvc

import (
	"fmt"

	"github.com/iainmoffat/tdx/internal/domain"
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
		if local.Hours == remote.Hours {
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
