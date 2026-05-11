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
	return cmd
}
