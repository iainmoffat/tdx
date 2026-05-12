package projectsvc

import (
	"context"
	"fmt"

	"github.com/iainmoffat/tdx/internal/domain"
)

// ListResources fetches the resource list for a project.
// GET /api/projects/{id}/resources. Returns lowercase UIDs and full names;
// null-valued optional fields are tolerated.
func (s *Service) ListResources(ctx context.Context, profile string, projectID int) ([]domain.ProjectResource, error) {
	client, err := s.clientFor(profile)
	if err != nil {
		return nil, err
	}
	var wire []wireProjectResource
	path := fmt.Sprintf("/TDWebApi/api/projects/%d/resources", projectID)
	if err := client.DoJSON(ctx, "GET", path, nil, &wire); err != nil {
		return nil, fmt.Errorf("list project %d resources: %w", projectID, err)
	}
	out := make([]domain.ProjectResource, 0, len(wire))
	for _, w := range wire {
		out = append(out, domain.ProjectResource{
			UID:      w.UID,
			FullName: w.FullName,
			RoleID:   w.RoleID,
			RoleName: w.RoleName,
			IsActive: w.IsActive,
		})
	}
	return out, nil
}
