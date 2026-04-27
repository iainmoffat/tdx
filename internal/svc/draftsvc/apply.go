package draftsvc

import (
	"context"
	"fmt"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

// ApplyResult summarises the effect of an Apply run.
type ApplyResult struct {
	Created int
	Updated int
	Deleted int
	Skipped int
	Failed  []ApplyFailure
}

// ApplyFailure records one per-action failure during push.
type ApplyFailure struct {
	Kind    string
	RowID   string
	Date    string
	EntryID int
	Message string
}

// Apply executes the actions in the diff after re-verifying the hash.
// allowDeletes must be true if any ActionDelete actions are present, otherwise
// the entire push is refused. Auto-snapshots before any writes.
func (s *Service) Apply(ctx context.Context, profile string, weekStart time.Time, name string, expectedHash string, allowDeletes bool, userUID string) (ApplyResult, error) {
	draft, diff, err := s.Reconcile(ctx, profile, weekStart, name, userUID)
	if err != nil {
		return ApplyResult{}, err
	}

	if diff.DiffHash != expectedHash {
		return ApplyResult{}, fmt.Errorf("week changed since preview (hash mismatch)")
	}

	hasDeletes := false
	for _, a := range diff.Actions {
		if a.Kind == domain.ActionDelete {
			hasDeletes = true
			break
		}
	}
	if hasDeletes && !allowDeletes {
		return ApplyResult{}, fmt.Errorf("draft contains delete actions; pass --allow-deletes to confirm")
	}

	if _, err := s.snapshots.Take(draft, OpPrePush, ""); err != nil {
		return ApplyResult{}, fmt.Errorf("auto-snapshot pre-push: %w", err)
	}

	var result ApplyResult
	for _, a := range diff.Actions {
		switch a.Kind {
		case domain.ActionCreate:
			if _, err := s.tsvc.AddEntry(ctx, profile, a.Entry); err != nil {
				result.Failed = append(result.Failed, ApplyFailure{
					Kind: "create", RowID: a.RowID, Date: a.Date.Format("2006-01-02"), Message: err.Error(),
				})
			} else {
				result.Created++
			}
		case domain.ActionUpdate:
			if _, err := s.tsvc.UpdateEntry(ctx, profile, a.ExistingID, a.Patch); err != nil {
				result.Failed = append(result.Failed, ApplyFailure{
					Kind: "update", RowID: a.RowID, Date: a.Date.Format("2006-01-02"),
					EntryID: a.ExistingID, Message: err.Error(),
				})
			} else {
				result.Updated++
			}
		case domain.ActionDelete:
			if err := s.tsvc.DeleteEntry(ctx, profile, a.DeleteEntryID); err != nil {
				result.Failed = append(result.Failed, ApplyFailure{
					Kind: "delete", RowID: a.RowID, Date: a.Date.Format("2006-01-02"),
					EntryID: a.DeleteEntryID, Message: err.Error(),
				})
			} else {
				result.Deleted++
			}
		case domain.ActionSkip:
			result.Skipped++
		}
	}

	now := time.Now().UTC()
	draft.PushedAt = &now
	if err := s.store.Save(draft); err != nil {
		return result, err
	}
	return result, nil
}
