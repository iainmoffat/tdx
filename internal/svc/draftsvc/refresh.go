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
