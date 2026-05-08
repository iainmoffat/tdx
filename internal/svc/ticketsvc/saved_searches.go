package ticketsvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/iainmoffat/tdx/internal/domain"
)

// ListSavedSearches returns saved searches visible to the current user
// for the given app.
func (s *Service) ListSavedSearches(ctx context.Context, profileName string, appID int) ([]domain.TicketSavedSearch, error) {
	resolvedAppID, err := s.resolveAppID(profileName, appID)
	if err != nil {
		return nil, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	var wire []wireSavedSearch
	path := fmt.Sprintf("/TDWebApi/api/%d/tickets/searches", resolvedAppID)
	if err := client.DoJSON(ctx, "GET", path, nil, &wire); err != nil {
		return nil, fmt.Errorf("list saved searches: %w", err)
	}
	out := make([]domain.TicketSavedSearch, 0, len(wire))
	for _, w := range wire {
		out = append(out, domain.TicketSavedSearch{
			ID:          w.ID,
			Name:        strings.TrimSpace(w.Name),
			OwnerUID:    w.OwnerUid,
			OwnerName:   w.OwnerFullName,
			Description: w.Description,
		})
	}
	return out, nil
}

// RunSavedSearch executes the saved search by ID and returns results.
// limit caps results (default 50 when limit <= 0).
// Returned tickets are partial records (IsFull=false).
func (s *Service) RunSavedSearch(ctx context.Context, profileName string, appID, searchID, limit int) ([]domain.Ticket, error) {
	resolvedAppID, err := s.resolveAppID(profileName, appID)
	if err != nil {
		return nil, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}
	req := wireSavedSearchOptions{MaxResults: limit}
	var wire []wireTicket
	path := fmt.Sprintf("/TDWebApi/api/%d/tickets/searches/%d/results", resolvedAppID, searchID)
	if err := client.DoJSON(ctx, "POST", path, req, &wire); err != nil {
		return nil, fmt.Errorf("run saved search %d: %w", searchID, err)
	}
	out := make([]domain.Ticket, 0, len(wire))
	for _, w := range wire {
		out = append(out, decodeTicket(w, false))
	}
	return out, nil
}

// ResolveSavedSearchByName finds a saved search by case-insensitive exact match.
func (s *Service) ResolveSavedSearchByName(ctx context.Context, profileName string, appID int, name string) (domain.TicketSavedSearch, error) {
	all, err := s.ListSavedSearches(ctx, profileName, appID)
	if err != nil {
		return domain.TicketSavedSearch{}, err
	}
	target := strings.ToLower(strings.TrimSpace(name))
	var matches []domain.TicketSavedSearch
	for _, ss := range all {
		if strings.ToLower(ss.Name) == target {
			matches = append(matches, ss)
		}
	}
	switch len(matches) {
	case 0:
		return domain.TicketSavedSearch{}, fmt.Errorf("no saved search matches %q", name)
	case 1:
		return matches[0], nil
	default:
		return domain.TicketSavedSearch{}, fmt.Errorf("multiple saved searches match %q (use --search-id <int> instead)", name)
	}
}
