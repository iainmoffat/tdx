package ticketsvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/iainmoffat/tdx/internal/domain"
)

// ListApps fetches all platform applications and filters to ticketing apps.
// TD's tenant-level /api/applications endpoint returns every app type
// (tickets, projects, KB, assets, etc.); we filter via AppClass containing
// "Ticket" (case-insensitive).
func (s *Service) ListApps(ctx context.Context, profileName string) ([]domain.TicketApp, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	var wire []wireApp
	if err := client.DoJSON(ctx, "GET", "/TDWebApi/api/applications", nil, &wire); err != nil {
		return nil, fmt.Errorf("list applications: %w", err)
	}
	out := make([]domain.TicketApp, 0, len(wire))
	for _, w := range wire {
		if !isTicketApp(w) {
			continue
		}
		out = append(out, domain.TicketApp{
			ID:          w.ID,
			Name:        w.Name,
			Description: w.Description,
			Active:      w.Active,
			AppType:     w.Type,
		})
	}
	return out, nil
}

func isTicketApp(w wireApp) bool {
	return strings.Contains(strings.ToLower(w.AppClass), "ticket")
}
