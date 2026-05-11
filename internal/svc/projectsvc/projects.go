package projectsvc

import (
	"context"
	"fmt"

	"github.com/iainmoffat/tdx/internal/domain"
)

// ListMine returns plan-shaped rows for projects the caller participates in.
// Note: despite the endpoint name, /api/projects/list returns plan objects
// (one row per plan). Use this for "what am I on"; use Search for
// project-level search.
func (s *Service) ListMine(ctx context.Context, profile string) ([]domain.ProjectPlan, error) {
	c, err := s.clientFor(profile)
	if err != nil {
		return nil, err
	}
	var rows []wirePlan
	if err := c.DoJSON(ctx, "GET", "/TDWebApi/api/projects/list", nil, &rows); err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	out := make([]domain.ProjectPlan, 0, len(rows))
	for _, w := range rows {
		out = append(out, decodePlan(w))
	}
	return out, nil
}

// Search calls POST /api/projects/search. Returns project-level records.
func (s *Service) Search(ctx context.Context, profile string, f domain.ProjectSearchFilter) ([]domain.Project, error) {
	c, err := s.clientFor(profile)
	if err != nil {
		return nil, err
	}
	body := wireProjectSearch{
		NameLike:   f.NameLike,
		ManagerUID: f.ManagerUID,
		StatusIDs:  f.StatusIDs,
		TypeIDs:    f.TypeIDs,
		IsActive:   f.IsActive,
		IsOpen:     f.IsOpen,
		MaxResults: f.MaxResults,
	}
	var rows []wireProject
	if err := c.DoJSON(ctx, "POST", "/TDWebApi/api/projects/search", body, &rows); err != nil {
		return nil, fmt.Errorf("search projects: %w", err)
	}
	out := make([]domain.Project, 0, len(rows))
	for _, w := range rows {
		out = append(out, decodeProject(w))
	}
	return out, nil
}

// Get fetches a single project by ID.
func (s *Service) Get(ctx context.Context, profile string, id int) (domain.Project, error) {
	c, err := s.clientFor(profile)
	if err != nil {
		return domain.Project{}, err
	}
	var w wireProject
	if err := c.DoJSON(ctx, "GET", fmt.Sprintf("/TDWebApi/api/projects/%d", id), nil, &w); err != nil {
		return domain.Project{}, fmt.Errorf("get project %d: %w", id, err)
	}
	return decodeProject(w), nil
}

func decodeProject(w wireProject) domain.Project {
	return domain.Project{
		ID:              w.ID,
		Name:            w.Name,
		StatusID:        w.StatusID,
		StatusName:      w.StatusName,
		TypeID:          w.TypeID,
		TypeName:        w.TypeName,
		AccountID:       w.AccountID,
		AccountName:     w.AccountName,
		ManagerUID:      w.AdminUID,
		ManagerName:     w.AdminName,
		SponsorUID:      w.SponsorUID,
		SponsorName:     w.SponsorName,
		PercentComplete: w.PercentComplete,
		EstimatedHours:  w.EstimatedHours,
		ActualHours:     w.ActualHours,
		StartDate:       parseTD(w.StartDate),
		EndDate:         parseTD(w.EndDate),
		ModifiedDate:    parseTD(w.ModifiedDate),
		IsActive:        w.IsActive,
		Description:     w.Description,
	}
}

func decodePlan(w wirePlan) domain.ProjectPlan {
	return domain.ProjectPlan{
		ID:              w.ID,
		ProjectID:       w.ProjectID,
		ProjectName:     w.ProjectName,
		Title:           w.Title,
		Type:            domain.ProjectPlanType(w.PlanType),
		TaskCount:       w.TaskCount,
		MyTaskCount:     w.MyTaskCount,
		PercentComplete: w.PercentComplete,
		EstimatedHours:  w.EstimatedHours,
		ActualHours:     w.ActualHours,
		StartDate:       parseTD(w.StartDateUtc),
		EndDate:         parseTD(w.EndDateUtc),
		ModifiedDate:    parseTD(w.ModifiedDate),
		IsCheckedOut:    w.IsCheckedOut,
	}
}
