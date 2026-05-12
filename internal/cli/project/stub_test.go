//nolint:unused // all methods consumed by subcommand tests
package project

import (
	"context"

	"github.com/iainmoffat/tdx/internal/domain"
)

// stubProjectsvc is the shared test stub for projectsvcAPI.
// Tests configure only the fields/methods they exercise.
//
//nolint:unused // populated by subcommand tests
type stubProjectsvc struct {
	plans        []domain.ProjectPlan
	plan         domain.ProjectPlan
	projects     []domain.Project
	project      domain.Project
	tasks        []domain.ProjectTask
	task         domain.ProjectTask
	types        []domain.ProjectType
	resolvedType domain.ProjectType

	// Feed fields
	feed        []domain.ProjectFeedEntry
	feedAddedID int

	// Phase 4: resources
	resources []domain.ProjectResource

	// Capture last-call arguments for assertion.
	lastProjectID   int
	lastPlanID      int
	lastTaskID      int
	lastFilter      domain.ProjectSearchFilter
	lastNameLike    string
	lastFeedMessage string
	lastFeedPrivate bool
	lastFeedNotify  []string

	err error
}

func (s *stubProjectsvc) ListMine(_ context.Context, _ string) ([]domain.ProjectPlan, error) {
	return s.plans, s.err
}
func (s *stubProjectsvc) Search(_ context.Context, _ string, f domain.ProjectSearchFilter) ([]domain.Project, error) {
	s.lastFilter = f
	return s.projects, s.err
}
func (s *stubProjectsvc) Get(_ context.Context, _ string, id int) (domain.Project, error) {
	s.lastProjectID = id
	return s.project, s.err
}
func (s *stubProjectsvc) SearchPlans(_ context.Context, _ string, projectID int, nameLike string, _ bool) ([]domain.ProjectPlan, error) {
	s.lastProjectID = projectID
	s.lastNameLike = nameLike
	return s.plans, s.err
}
func (s *stubProjectsvc) ListTasks(_ context.Context, _ string, projectID, planID int) ([]domain.ProjectTask, error) {
	s.lastProjectID = projectID
	s.lastPlanID = planID
	return s.tasks, s.err
}
func (s *stubProjectsvc) GetTask(_ context.Context, _ string, projectID, planID, taskID int) (domain.ProjectTask, error) {
	s.lastProjectID = projectID
	s.lastPlanID = planID
	s.lastTaskID = taskID
	return s.task, s.err
}
func (s *stubProjectsvc) ListProjectTypes(_ context.Context, _ string, _ bool) ([]domain.ProjectType, error) {
	return s.types, s.err
}
func (s *stubProjectsvc) ResolveTypeByName(_ context.Context, _ string, _ string) (domain.ProjectType, error) {
	return s.resolvedType, s.err
}
func (s *stubProjectsvc) GetFeed(_ context.Context, _ string, projectID int) ([]domain.ProjectFeedEntry, error) {
	s.lastProjectID = projectID
	return s.feed, s.err
}
func (s *stubProjectsvc) AddFeed(_ context.Context, _ string, projectID int, message string, isPrivate bool, notify []string) (int, error) {
	s.lastProjectID = projectID
	s.lastFeedMessage = message
	s.lastFeedPrivate = isPrivate
	s.lastFeedNotify = notify
	return s.feedAddedID, s.err
}
func (s *stubProjectsvc) GetTaskFeed(_ context.Context, _ string, projectID, planID, taskID int) ([]domain.ProjectFeedEntry, error) {
	s.lastProjectID = projectID
	s.lastPlanID = planID
	s.lastTaskID = taskID
	return s.feed, s.err
}
func (s *stubProjectsvc) AddTaskFeed(_ context.Context, _ string, projectID, planID, taskID int, message string, isPrivate bool, notify []string) (int, error) {
	s.lastProjectID = projectID
	s.lastPlanID = planID
	s.lastTaskID = taskID
	s.lastFeedMessage = message
	s.lastFeedPrivate = isPrivate
	s.lastFeedNotify = notify
	return s.feedAddedID, s.err
}

func (s *stubProjectsvc) ListResources(_ context.Context, _ string, projectID int) ([]domain.ProjectResource, error) {
	s.lastProjectID = projectID
	return s.resources, s.err
}
