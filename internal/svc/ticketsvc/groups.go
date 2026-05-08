package ticketsvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/iainmoffat/tdx/internal/domain"
)

// ListGroups returns all tenant groups via POST /api/groups/search.
// The endpoint may silently ignore body filter params (per the established
// TD silent-filter pattern); we send an empty body and let callers filter
// client-side. Groups are tenant-wide — no app-id needed.
func (s *Service) ListGroups(ctx context.Context, profileName string) ([]domain.TicketGroup, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	var wire []wireGroup
	if err := client.DoJSON(ctx, "POST", "/TDWebApi/api/groups/search", struct{}{}, &wire); err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	out := make([]domain.TicketGroup, 0, len(wire))
	for _, w := range wire {
		out = append(out, domain.TicketGroup{
			ID:     w.ID,
			Name:   strings.TrimSpace(w.Name),
			Active: w.IsActive,
		})
	}
	return out, nil
}

// ResolveGroupByName finds a group by case-insensitive exact match.
// Returns an error listing candidates if zero or >1 match.
func (s *Service) ResolveGroupByName(ctx context.Context, profileName string, name string) (domain.TicketGroup, error) {
	all, err := s.ListGroups(ctx, profileName)
	if err != nil {
		return domain.TicketGroup{}, err
	}
	target := strings.ToLower(strings.TrimSpace(name))
	var matches []domain.TicketGroup
	for _, g := range all {
		if strings.ToLower(g.Name) == target {
			matches = append(matches, g)
		}
	}
	switch len(matches) {
	case 0:
		return domain.TicketGroup{}, fmt.Errorf("no ticket group matches %q (use `tdx ticket groups list` to see options)", name)
	case 1:
		return matches[0], nil
	default:
		labels := make([]string, 0, len(matches))
		for _, m := range matches {
			labels = append(labels, fmt.Sprintf("%d (%s)", m.ID, m.Name))
		}
		return domain.TicketGroup{}, fmt.Errorf("multiple ticket groups match %q: %s — pass numeric id directly via --responsibility-group <int>", name, strings.Join(labels, ", "))
	}
}
