package tmplsvc

import (
	"context"
	"fmt"

	"github.com/iainmoffat/tdx/internal/domain"
)

// ApplyResult reports what happened during apply.
type ApplyResult struct {
	Created int
	Updated int
	Skipped int
	Failed  []ApplyFailure
}

// ApplyFailure records a single failed action.
type ApplyFailure struct {
	RowID   string
	Date    string
	Message string
}

// Apply executes the actions from a reconciliation preview.
// It re-runs reconciliation to verify the diffHash hasn't changed,
// then executes Create and Update actions via timesvc.
func (s *Service) Apply(ctx context.Context, profileName string, input ReconcileInput, expectedHash string) (ApplyResult, error) {
	// Re-run reconciliation to detect concurrent changes.
	diff, err := s.Reconcile(ctx, profileName, input)
	if err != nil {
		return ApplyResult{}, err
	}
	if diff.DiffHash != expectedHash {
		return ApplyResult{}, fmt.Errorf("week changed since preview (hash mismatch)")
	}

	var result ApplyResult
	for _, a := range diff.Actions {
		switch a.Kind {
		case domain.ActionCreate:
			entry := a.Entry
			// Apply ownership marker if checker is provided.
			if input.Checker != nil {
				entry.Description = input.Checker.Mark(entry.Description, input.Template.Name, a.RowID)
			}
			_, err := s.tsvc.AddEntry(ctx, profileName, entry)
			if err != nil {
				result.Failed = append(result.Failed, ApplyFailure{
					RowID:   a.RowID,
					Date:    a.Date.Format("2006-01-02"),
					Message: err.Error(),
				})
			} else {
				result.Created++
			}

		case domain.ActionUpdate:
			_, err := s.tsvc.UpdateEntry(ctx, profileName, a.ExistingID, a.Patch)
			if err != nil {
				result.Failed = append(result.Failed, ApplyFailure{
					RowID:   a.RowID,
					Date:    a.Date.Format("2006-01-02"),
					Message: err.Error(),
				})
			} else {
				result.Updated++
			}

		case domain.ActionSkip:
			result.Skipped++
		}
	}
	return result, nil
}
