package projectsvc

import (
	"context"
	"fmt"

	"github.com/iainmoffat/tdx/internal/domain"
)

// SearchPlans calls POST /api/projects/{projectID}/plans/search.
func (s *Service) SearchPlans(ctx context.Context, profile string, projectID int, nameLike string, includeEmpty bool) ([]domain.ProjectPlan, error) {
	c, err := s.clientFor(profile)
	if err != nil {
		return nil, err
	}
	body := wirePlanSearch{
		NameLike:     nameLike,
		IncludeEmpty: includeEmpty,
	}
	var rows []wirePlan
	path := fmt.Sprintf("/TDWebApi/api/projects/%d/plans/search", projectID)
	if err := c.DoJSON(ctx, "POST", path, body, &rows); err != nil {
		return nil, fmt.Errorf("search plans for project %d: %w", projectID, err)
	}
	out := make([]domain.ProjectPlan, 0, len(rows))
	for _, w := range rows {
		out = append(out, decodePlan(w))
	}
	return out, nil
}
