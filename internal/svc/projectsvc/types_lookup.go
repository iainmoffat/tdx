package projectsvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/iainmoffat/tdx/internal/domain"
)

// ListProjectTypes fetches project types from GET /api/projects/types.
// If includeInactive is false (the default), passes isActive=true.
func (s *Service) ListProjectTypes(ctx context.Context, profile string, includeInactive bool) ([]domain.ProjectType, error) {
	c, err := s.clientFor(profile)
	if err != nil {
		return nil, err
	}
	path := "/TDWebApi/api/projects/types"
	if !includeInactive {
		path += "?isActive=true"
	}
	var rows []wireProjectType
	if err := c.DoJSON(ctx, "GET", path, nil, &rows); err != nil {
		return nil, fmt.Errorf("list project types: %w", err)
	}
	out := make([]domain.ProjectType, 0, len(rows))
	for _, w := range rows {
		out = append(out, domain.ProjectType{
			ID:       w.ID,
			Name:     strings.TrimSpace(w.Name),
			IsActive: w.IsActive,
		})
	}
	return out, nil
}

// ResolveTypeByName finds a project type by case-insensitive exact match.
// Returns an error listing candidates if zero or >1 match.
func (s *Service) ResolveTypeByName(ctx context.Context, profile string, name string) (domain.ProjectType, error) {
	types, err := s.ListProjectTypes(ctx, profile, false)
	if err != nil {
		return domain.ProjectType{}, err
	}
	target := strings.ToLower(strings.TrimSpace(name))
	var matches []domain.ProjectType
	for _, pt := range types {
		if strings.ToLower(pt.Name) == target {
			matches = append(matches, pt)
		}
	}
	switch len(matches) {
	case 0:
		names := make([]string, 0, len(types))
		for _, pt := range types {
			names = append(names, pt.Name)
		}
		return domain.ProjectType{}, fmt.Errorf("no project type matches %q among %d types (available: %s)",
			name, len(types), strings.Join(names, ", "))
	case 1:
		return matches[0], nil
	default:
		ids := make([]string, 0, len(matches))
		for _, m := range matches {
			ids = append(ids, fmt.Sprintf("%d (%s)", m.ID, m.Name))
		}
		return domain.ProjectType{}, fmt.Errorf("multiple project types match %q: %s — pass --type-id <int> instead",
			name, strings.Join(ids, ", "))
	}
}
