package ticketsvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/iainmoffat/tdx/internal/domain"
)

// ListTasks fetches all tasks on a ticket via
// GET /api/{appId}/tickets/{ticketID}/tasks.
func (s *Service) ListTasks(ctx context.Context, profileName string, appID, ticketID int) ([]domain.TicketTask, error) {
	resolvedAppID, err := s.resolveAppID(profileName, appID)
	if err != nil {
		return nil, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	var wire []wireTicketTask
	path := fmt.Sprintf("/TDWebApi/api/%d/tickets/%d/tasks", resolvedAppID, ticketID)
	if err := client.DoJSON(ctx, "GET", path, nil, &wire); err != nil {
		return nil, fmt.Errorf("list ticket tasks %d: %w", ticketID, err)
	}
	out := make([]domain.TicketTask, 0, len(wire))
	for _, w := range wire {
		out = append(out, decodeTask(w))
	}
	return out, nil
}

// GetTask fetches a single task on a ticket.
func (s *Service) GetTask(ctx context.Context, profileName string, appID, ticketID, taskID int) (domain.TicketTask, error) {
	resolvedAppID, err := s.resolveAppID(profileName, appID)
	if err != nil {
		return domain.TicketTask{}, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return domain.TicketTask{}, err
	}
	var w wireTicketTask
	path := fmt.Sprintf("/TDWebApi/api/%d/tickets/%d/tasks/%d", resolvedAppID, ticketID, taskID)
	if err := client.DoJSON(ctx, "GET", path, nil, &w); err != nil {
		return domain.TicketTask{}, fmt.Errorf("get ticket task %d/%d: %w", ticketID, taskID, err)
	}
	return decodeTask(w), nil
}

// GetTaskFeed fetches feed entries for a ticket task.
func (s *Service) GetTaskFeed(ctx context.Context, profileName string, appID, ticketID, taskID int) ([]domain.TicketFeedEntry, error) {
	resolvedAppID, err := s.resolveAppID(profileName, appID)
	if err != nil {
		return nil, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	var wire []wireFeedEntry
	path := fmt.Sprintf("/TDWebApi/api/%d/tickets/%d/tasks/%d/feed", resolvedAppID, ticketID, taskID)
	if err := client.DoJSON(ctx, "GET", path, nil, &wire); err != nil {
		return nil, fmt.Errorf("get ticket task feed %d/%d: %w", ticketID, taskID, err)
	}
	out := make([]domain.TicketFeedEntry, 0, len(wire))
	for _, w := range wire {
		out = append(out, domain.TicketFeedEntry{
			ID:         w.ID,
			AuthorUID:  w.CreatedUid,
			AuthorName: w.CreatedFullName,
			CreatedAt:  parseTDTime(w.CreatedDate),
			Body:       w.Body,
			IsPrivate:  w.IsPrivate,
			EventKind:  classifyFeedKind(w.UpdateType),
		})
	}
	return out, nil
}

// UpdateTaskFeed posts a feed update to a ticket task. Returns the new
// feed entry's ID. percentComplete is a pointer because 0 is a valid
// value (set to 0%); pass nil to omit. hoursWorked is informational
// only — does NOT create a time entry or update task ActualMinutes.
func (s *Service) UpdateTaskFeed(ctx context.Context, profileName string, appID, ticketID, taskID int, body string, percentComplete *int, hoursWorked float64, isPrivate bool, notify []string) (int, error) {
	resolvedAppID, err := s.resolveAppID(profileName, appID)
	if err != nil {
		return 0, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return 0, err
	}
	req := wireTaskFeedUpdate{
		Comments:        body,
		PercentComplete: percentComplete,
		HoursWorked:     hoursWorked,
		IsPrivate:       isPrivate,
		Notify:          notify,
	}
	var resp wireFeedEntry
	path := fmt.Sprintf("/TDWebApi/api/%d/tickets/%d/tasks/%d/feed", resolvedAppID, ticketID, taskID)
	if err := client.DoJSON(ctx, "POST", path, req, &resp); err != nil {
		return 0, fmt.Errorf("update ticket task %d/%d: %w", ticketID, taskID, err)
	}
	return resp.ID, nil
}

// decodeTask maps a wireTicketTask to domain.TicketTask. Trims whitespace
// on Title; handles TD's "0001-01-01..." unset-date sentinel via
// parseTDTime returning zero time.
func decodeTask(w wireTicketTask) domain.TicketTask {
	return domain.TicketTask{
		ID:                   w.ID,
		TicketID:             w.TicketID,
		Title:                strings.TrimSpace(w.Title),
		Description:          w.Description,
		Active:               w.IsActive,
		PercentComplete:      w.PercentComplete,
		EstimatedMinutes:     w.EstimatedMinutes,
		ActualMinutes:        w.ActualMinutes,
		StartDate:            parseTDTime(w.StartDate),
		EndDate:              parseTDTime(w.EndDate),
		CreatedDate:          parseTDTime(w.CreatedDate),
		CreatedName:          w.CreatedFullName,
		ModifiedDate:         parseTDTime(w.ModifiedDate),
		CompletedDate:        parseTDTime(w.CompletedDate),
		CompletedName:        w.CompletedFullName,
		ResponsibleUID:       w.ResponsibleUid,
		ResponsibleName:      w.ResponsibleFullName,
		ResponsibleGroupID:   w.ResponsibleGroupID,
		ResponsibleGroupName: w.ResponsibleGroupName,
		Order:                w.Order,
	}
}
