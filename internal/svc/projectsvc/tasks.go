package projectsvc

import (
	"context"
	"fmt"

	"github.com/iainmoffat/tdx/internal/domain"
)

// ListTasks fetches all tasks for a plan: GET /api/projects/{projectID}/plans/{planID}/tasks.
func (s *Service) ListTasks(ctx context.Context, profile string, projectID, planID int) ([]domain.ProjectTask, error) {
	c, err := s.clientFor(profile)
	if err != nil {
		return nil, err
	}
	var rows []wireTask
	path := fmt.Sprintf("/TDWebApi/api/projects/%d/plans/%d/tasks", projectID, planID)
	if err := c.DoJSON(ctx, "GET", path, nil, &rows); err != nil {
		return nil, fmt.Errorf("list tasks for project %d plan %d: %w", projectID, planID, err)
	}
	out := make([]domain.ProjectTask, 0, len(rows))
	for _, w := range rows {
		out = append(out, decodeTask(w, projectID, planID))
	}
	return out, nil
}

// GetTask fetches a single task: GET /api/projects/{projectID}/plans/{planID}/tasks/{taskID}.
func (s *Service) GetTask(ctx context.Context, profile string, projectID, planID, taskID int) (domain.ProjectTask, error) {
	c, err := s.clientFor(profile)
	if err != nil {
		return domain.ProjectTask{}, err
	}
	var w wireTask
	path := fmt.Sprintf("/TDWebApi/api/projects/%d/plans/%d/tasks/%d", projectID, planID, taskID)
	if err := c.DoJSON(ctx, "GET", path, nil, &w); err != nil {
		return domain.ProjectTask{}, fmt.Errorf("get task %d (project %d plan %d): %w", taskID, projectID, planID, err)
	}
	return decodeTask(w, projectID, planID), nil
}

func decodeTask(w wireTask, projectID, planID int) domain.ProjectTask {
	// Use wire values if present; fall back to caller-supplied IDs (defensive).
	pid := w.ProjectID
	if pid == 0 {
		pid = projectID
	}
	plid := w.PlanID
	if plid == 0 {
		plid = planID
	}

	resources := make([]domain.ProjectTaskResource, 0, len(w.Resources))
	for _, r := range w.Resources {
		resources = append(resources, domain.ProjectTaskResource{
			UID:             r.ResourceUID,
			FullName:        r.ResourceFullName,
			PercentAssigned: r.PercentAssignedWhole,
			RoleID:          r.ResourceRoleID,
			RoleName:        r.ResourceRoleName,
		})
	}

	return domain.ProjectTask{
		ProjectID:       pid,
		PlanID:          plid,
		PlanName:        w.PlanName,
		ID:              w.ID,
		Title:           w.Title,
		Status:          w.Status,
		StatusID:        w.StatusID,
		PercentComplete: w.PercentComplete,
		EstimatedHours:  w.EstimatedHours,
		ActualHours:     w.ActualHours,
		RemainingHours:  w.RemainingHours,
		StartDate:       parseTD(w.StartDateUtc),
		EndDate:         parseTD(w.EndDateUtc),
		ModifiedDate:    parseTD(w.ModifiedDate),
		IsParent:        w.IsParent,
		IndentLevel:     w.IndentLevel,
		ParentID:        w.ParentID,
		OutlineNumber:   w.OutlineNumber,
		Description:     w.Description,
		Resources:       resources,
		TicketAppID:     w.TicketAppID,
		TicketID:        w.TicketID,
	}
}
