package timesvc

import (
	"context"
	"fmt"

	"github.com/ipm/tdx/internal/domain"
)

// AddEntry creates a new time entry and returns the full domain entry.
func (s *Service) AddEntry(ctx context.Context, profileName string, input domain.EntryInput) (domain.TimeEntry, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return domain.TimeEntry{}, err
	}

	w, err := encodeEntryWrite(input)
	if err != nil {
		return domain.TimeEntry{}, fmt.Errorf("encode entry: %w", err)
	}

	// POST expects a JSON array (batch of 1).
	payload := []wireTimeEntryWrite{w}
	var result wireBulkResult
	if err := client.DoJSON(ctx, "POST", "/TDWebApi/api/time", payload, &result); err != nil {
		return domain.TimeEntry{}, fmt.Errorf("create entry: %w", err)
	}

	if len(result.Failed) > 0 {
		return domain.TimeEntry{}, fmt.Errorf("create entry: %s", result.Failed[0].ErrorMessage)
	}
	if len(result.Succeeded) == 0 {
		return domain.TimeEntry{}, fmt.Errorf("create entry: no entry created")
	}

	// Fetch the created entry for the full domain object with resolved type names.
	return s.GetEntry(ctx, profileName, result.Succeeded[0].ID)
}
