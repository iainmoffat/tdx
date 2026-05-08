package ticketsvc

import (
	"context"
	"fmt"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

// GetTicket fetches a single ticket by ID. Returns IsFull=true on the result.
func (s *Service) GetTicket(ctx context.Context, profileName string, appID, id int) (domain.Ticket, error) {
	resolvedAppID, err := s.resolveAppID(profileName, appID)
	if err != nil {
		return domain.Ticket{}, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return domain.Ticket{}, err
	}
	var w wireTicket
	path := fmt.Sprintf("/TDWebApi/api/%d/tickets/%d", resolvedAppID, id)
	if err := client.DoJSON(ctx, "GET", path, nil, &w); err != nil {
		return domain.Ticket{}, fmt.Errorf("get ticket %d: %w", id, err)
	}
	return decodeTicket(w, true), nil
}

// SearchTickets calls POST /tickets/search. Returns partial records (IsFull=false).
func (s *Service) SearchTickets(ctx context.Context, profileName string, filter domain.TicketSearchFilter) ([]domain.Ticket, error) {
	resolvedAppID, err := s.resolveAppID(profileName, filter.AppID)
	if err != nil {
		return nil, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	req := wireTicketSearch{
		StatusIDs:          filter.StatusIDs,
		ResponsibilityUids: filter.AssigneeUIDs,
		RequestorUids:      filter.RequestorUIDs,
		AccountIDs:         filter.AccountIDs,
		SearchText:         filter.Text,
		MaxResults:         filter.Limit,
	}
	if !filter.IncludeClosed {
		t := true
		req.IsOpen = &t
	}
	if req.MaxResults == 0 {
		req.MaxResults = 50
	}
	var wire []wireTicket
	path := fmt.Sprintf("/TDWebApi/api/%d/tickets/search", resolvedAppID)
	if err := client.DoJSON(ctx, "POST", path, req, &wire); err != nil {
		return nil, fmt.Errorf("search tickets: %w", err)
	}
	out := make([]domain.Ticket, 0, len(wire))
	for _, w := range wire {
		out = append(out, decodeTicket(w, false))
	}
	return out, nil
}

// PatchTicket applies one or more JSON-Patch operations to a ticket.
// Returns the updated ticket (IsFull=true).
func (s *Service) PatchTicket(ctx context.Context, profileName string, appID, id int, ops []PatchOp) (domain.Ticket, error) {
	resolvedAppID, err := s.resolveAppID(profileName, appID)
	if err != nil {
		return domain.Ticket{}, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return domain.Ticket{}, err
	}
	var w wireTicket
	path := fmt.Sprintf("/TDWebApi/api/%d/tickets/%d", resolvedAppID, id)
	if err := client.DoJSON(ctx, "PATCH", path, ops, &w); err != nil {
		return domain.Ticket{}, fmt.Errorf("patch ticket %d: %w", id, err)
	}
	return decodeTicket(w, true), nil
}

func decodeTicket(w wireTicket, full bool) domain.Ticket {
	return domain.Ticket{
		ID:               w.ID,
		AppID:            w.AppID,
		Title:            w.Title,
		Description:      w.Description,
		StatusID:         w.StatusID,
		StatusName:       w.StatusName,
		TypeID:           w.TypeID,
		TypeName:         w.TypeName,
		PriorityID:       w.PriorityID,
		PriorityName:     w.PriorityName,
		AccountID:        w.AccountID,
		AccountName:      w.AccountName,
		ResponsibleUID:   w.ResponsibleUid,
		ResponsibleName:  w.ResponsibleFullName,
		RequestorUID:     w.RequestorUid,
		RequestorName:    w.RequestorName,
		CreatedDate:      parseTDTime(w.CreatedDate),
		ModifiedDate:     parseTDTime(w.ModifiedDate),
		EstimatedMinutes: w.EstimatedMinutes,
		ActualMinutes:    w.ActualMinutes,
		Tags:             w.Tags,
		IsFull:           full,
	}
}

// parseTDTime parses TD's date format. TD historically uses both
// `2006-01-02T15:04:05Z` and `2006-01-02T15:04:05.000-04:00`. Tolerate both.
// Returns zero time for empty strings or unparseable input.
func parseTDTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05Z"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
