package draftsvc

import (
	"context"
	"fmt"
	"sort"
	"time"

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
		return makeConflict(local, remote, strategy)

	// Local cleared (delete-on-push), remote modified.
	case localCleared && remoteExists && !cellEqual(*pulled, *remote):
		return makeConflict(local, remote, strategy)

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

	// Both sides added independently (no pulled cell).
	case !pulledExists && localExists && remoteExists:
		if local.Hours == remote.Hours {
			merged := *remote // adopt remote: it has the real sourceEntryID
			return cellClassification{outcome: outcomeResolved, merged: &merged}
		}
		return makeConflict(local, remote, strategy)

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
				// RowID and Day are filled in by classify() (the per-row caller).
			},
		}
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
}
