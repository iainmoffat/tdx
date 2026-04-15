package timesvc

import (
	"context"
	"fmt"

	"github.com/iainmoffat/tdx/internal/domain"
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

// UpdateEntry fetches the existing entry, applies the update, PUTs it back,
// and returns the updated domain entry.
func (s *Service) UpdateEntry(ctx context.Context, profileName string, id int, update domain.EntryUpdate) (domain.TimeEntry, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return domain.TimeEntry{}, err
	}

	// Fetch raw wire entry so we can re-submit all wire fields.
	var raw wireTimeEntry
	path := fmt.Sprintf("/TDWebApi/api/time/%d", id)
	if err := client.DoJSON(ctx, "GET", path, nil, &raw); err != nil {
		return domain.TimeEntry{}, fmt.Errorf("fetch entry %d: %w", id, err)
	}

	// Build a write struct from the raw entry, preserving all wire fields.
	w := wireTimeEntryWrite{
		TimeID:      raw.TimeID,
		Uid:         raw.Uid,
		TimeDate:    raw.TimeDate.Time.UTC().Format("2006-01-02") + "T00:00:00",
		Minutes:     raw.Minutes,
		TimeTypeID:  raw.TimeTypeID,
		Component:   raw.Component,
		TicketID:    raw.TicketID,
		ProjectID:   raw.ProjectID,
		PlanID:      raw.PlanID,
		PortfolioID: raw.PortfolioID,
		ItemID:      raw.ItemID,
		AppID:       raw.AppID,
		Description: raw.Description,
		Billable:    raw.Billable,
	}

	// Apply non-nil update fields.
	if update.Date != nil {
		w.TimeDate = update.Date.UTC().Format("2006-01-02") + "T00:00:00"
	}
	if update.Minutes != nil {
		w.Minutes = float64(*update.Minutes)
	}
	if update.TimeTypeID != nil {
		w.TimeTypeID = *update.TimeTypeID
	}
	if update.Billable != nil {
		w.Billable = *update.Billable
	}
	if update.Description != nil {
		w.Description = *update.Description
	}

	// PUT the modified entry.
	var updated wireTimeEntry
	if err := client.DoJSON(ctx, "PUT", path, w, &updated); err != nil {
		return domain.TimeEntry{}, fmt.Errorf("update entry %d: %w", id, err)
	}

	// Decode wire → domain and resolve type names.
	entry, err := decodeTimeEntry(updated)
	if err != nil {
		return domain.TimeEntry{}, err
	}
	single := []domain.TimeEntry{entry}
	if err := s.resolveTimeTypeNames(ctx, profileName, single); err != nil {
		return domain.TimeEntry{}, err
	}
	return single[0], nil
}

const maxBatchSize = 50

// DeleteEntry deletes a single time entry by ID.
func (s *Service) DeleteEntry(ctx context.Context, profileName string, id int) error {
	client, err := s.clientFor(profileName)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/TDWebApi/api/time/%d", id)
	if err := client.DoJSON(ctx, "DELETE", path, nil, nil); err != nil {
		return fmt.Errorf("delete entry %d: %w", id, err)
	}
	return nil
}

// DeleteEntries deletes multiple time entries in batches of 50.
// Returns a BatchResult with succeeded and failed IDs.
func (s *Service) DeleteEntries(ctx context.Context, profileName string, ids []int) (domain.BatchResult, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return domain.BatchResult{}, err
	}

	var result domain.BatchResult

	for i := 0; i < len(ids); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[i:end]

		var bulk wireBulkResult
		if err := client.DoJSON(ctx, "POST", "/TDWebApi/api/time/delete", chunk, &bulk); err != nil {
			for _, id := range chunk {
				result.Failed = append(result.Failed, domain.BatchFailure{
					ID:      id,
					Message: err.Error(),
				})
			}
			continue
		}

		for _, s := range bulk.Succeeded {
			result.Succeeded = append(result.Succeeded, s.ID)
		}
		for _, f := range bulk.Failed {
			result.Failed = append(result.Failed, domain.BatchFailure{
				ID:      f.TimeEntryID,
				Message: f.ErrorMessage,
			})
		}
	}

	return result, nil
}
