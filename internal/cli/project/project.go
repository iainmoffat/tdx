package project

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/domain"
)

// projectsvcAPI is the minimal interface CLI commands depend on.
// Defined here so tests can stub easily.
//
//nolint:unused // consumed by subcommands
type projectsvcAPI interface {
	ListMine(ctx context.Context, profile string) ([]domain.ProjectPlan, error)
	Search(ctx context.Context, profile string, filter domain.ProjectSearchFilter) ([]domain.Project, error)
	Get(ctx context.Context, profile string, id int) (domain.Project, error)
	SearchPlans(ctx context.Context, profile string, projectID int, nameLike string, includeEmpty bool) ([]domain.ProjectPlan, error)
	ListTasks(ctx context.Context, profile string, projectID, planID int) ([]domain.ProjectTask, error)
	GetTask(ctx context.Context, profile string, projectID, planID, taskID int) (domain.ProjectTask, error)
	ListProjectTypes(ctx context.Context, profile string, includeInactive bool) ([]domain.ProjectType, error)
	ResolveTypeByName(ctx context.Context, profile string, name string) (domain.ProjectType, error)
	// Feed / comment methods (Phase 2)
	GetFeed(ctx context.Context, profile string, projectID int) ([]domain.ProjectFeedEntry, error)
	AddFeed(ctx context.Context, profile string, projectID int, message string, isPrivate bool, notify []string) (int, error)
	GetTaskFeed(ctx context.Context, profile string, projectID, planID, taskID int) ([]domain.ProjectFeedEntry, error)
	AddTaskFeed(ctx context.Context, profile string, projectID, planID, taskID int, message string, isPrivate bool, notify []string) (int, error)
	// Phase 4: project team membership
	ListResources(ctx context.Context, profile string, projectID int) ([]domain.ProjectResource, error)
}

// New returns the top-level `tdx project` command.
func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Inspect TeamDynamix projects, plans, and tasks",
	}
	cmd.AddCommand(newListCmd(nil))
	cmd.AddCommand(newSearchCmd(nil))
	cmd.AddCommand(newShowCmd(nil))
	cmd.AddCommand(newPlanCmd(nil))
	cmd.AddCommand(newTaskCmd(nil))
	cmd.AddCommand(newLogCmd(nil))
	cmd.AddCommand(newFeedCmd(nil))
	cmd.AddCommand(newCommentCmd(nil))
	cmd.AddCommand(newTimeCmd(nil, nil))
	return cmd
}
