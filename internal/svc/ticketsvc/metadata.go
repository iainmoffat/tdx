package ticketsvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/iainmoffat/tdx/internal/domain"
)

// ListStatuses returns all statuses for the given app.
// appID == 0 falls back to the profile's TicketAppID via resolveAppID.
func (s *Service) ListStatuses(ctx context.Context, profileName string, appID int) ([]domain.TicketStatus, error) {
	id, err := s.resolveAppID(profileName, appID)
	if err != nil {
		return nil, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	var wire []wireTicketStatus
	path := fmt.Sprintf("/TDWebApi/api/%d/tickets/statuses", id)
	if err := client.DoJSON(ctx, "GET", path, nil, &wire); err != nil {
		return nil, fmt.Errorf("list ticket statuses: %w", err)
	}
	out := make([]domain.TicketStatus, 0, len(wire))
	for _, w := range wire {
		out = append(out, domain.TicketStatus{
			ID:        w.ID,
			Name:      strings.TrimSpace(w.Name),
			IsClosed:  w.StatusClass == 6,
			IsDefault: w.IsDefault,
			Order:     w.Order,
		})
	}
	return out, nil
}

// ListTypes returns active ticket types for the given app.
func (s *Service) ListTypes(ctx context.Context, profileName string, appID int) ([]domain.TicketType, error) {
	id, err := s.resolveAppID(profileName, appID)
	if err != nil {
		return nil, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	var wire []wireTicketType
	path := fmt.Sprintf("/TDWebApi/api/%d/tickets/types?isActive=true", id)
	if err := client.DoJSON(ctx, "GET", path, nil, &wire); err != nil {
		return nil, fmt.Errorf("list ticket types: %w", err)
	}
	out := make([]domain.TicketType, 0, len(wire))
	for _, w := range wire {
		out = append(out, domain.TicketType{
			ID:          w.ID,
			Name:        strings.TrimSpace(w.Name),
			Description: w.Description,
			Active:      w.IsActive,
		})
	}
	return out, nil
}

// ResolveStatusByName finds a status by case-insensitive exact match.
// Returns an error listing candidates if zero or >1 match.
func (s *Service) ResolveStatusByName(ctx context.Context, profileName string, appID int, name string) (domain.TicketStatus, error) {
	statuses, err := s.ListStatuses(ctx, profileName, appID)
	if err != nil {
		return domain.TicketStatus{}, err
	}
	target := strings.ToLower(strings.TrimSpace(name))
	var matches []domain.TicketStatus
	for _, st := range statuses {
		if strings.ToLower(st.Name) == target {
			matches = append(matches, st)
		}
	}
	switch len(matches) {
	case 0:
		return domain.TicketStatus{}, fmt.Errorf("no ticket status matches %q (use `tdx ticket statuses list` to see options)", name)
	case 1:
		return matches[0], nil
	default:
		names := make([]string, 0, len(matches))
		for _, m := range matches {
			names = append(names, fmt.Sprintf("%d (%s)", m.ID, m.Name))
		}
		return domain.TicketStatus{}, fmt.Errorf("multiple statuses match %q: %s — pass --status-id <int> instead", name, strings.Join(names, ", "))
	}
}

// ResolveTypeByName finds a ticket type by case-insensitive exact match.
func (s *Service) ResolveTypeByName(ctx context.Context, profileName string, appID int, name string) (domain.TicketType, error) {
	types, err := s.ListTypes(ctx, profileName, appID)
	if err != nil {
		return domain.TicketType{}, err
	}
	target := strings.ToLower(strings.TrimSpace(name))
	var matches []domain.TicketType
	for _, tt := range types {
		if strings.ToLower(tt.Name) == target {
			matches = append(matches, tt)
		}
	}
	switch len(matches) {
	case 0:
		return domain.TicketType{}, fmt.Errorf("no ticket type matches %q", name)
	case 1:
		return matches[0], nil
	default:
		return domain.TicketType{}, fmt.Errorf("multiple ticket types match %q (use --type-id instead)", name)
	}
}
