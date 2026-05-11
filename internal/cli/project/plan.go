package project

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/projectsvc"
)

func newPlanCmd(svc projectsvcAPI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Manage project plans",
	}
	cmd.AddCommand(newPlanListCmd(svc))
	return cmd
}

func newPlanListCmd(svc projectsvcAPI) *cobra.Command {
	var (
		nameLike     string
		includeEmpty bool
		jsonFlag     bool
		profileFlag  string
	)
	cmd := &cobra.Command{
		Use:   "list <project-id>",
		Short: "List plans for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return fmt.Errorf("project id must be a positive integer, got %q", args[0])
			}
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			auth := authsvc.New(paths)
			profile, err := auth.ResolveProfile(profileFlag)
			if err != nil {
				return err
			}
			s := svc
			if s == nil {
				s = projectsvc.New(paths)
			}
			return runPlanList(cmd.Context(), cmd.OutOrStdout(), s, profile, id, nameLike, includeEmpty, jsonFlag)
		},
	}
	cmd.Flags().StringVar(&nameLike, "name-like", "", "filter plans by substring")
	cmd.Flags().BoolVar(&includeEmpty, "include-empty", false, "include plans with no tasks")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runPlanList(ctx context.Context, w io.Writer, svc projectsvcAPI, profile string, projectID int, nameLike string, includeEmpty bool, jsonOut bool) error {
	plans, err := svc.SearchPlans(ctx, profile, projectID, nameLike, includeEmpty)
	if err != nil {
		return err
	}
	if len(plans) == 0 {
		_, _ = fmt.Fprintf(w, "no plans found for project %d\n", projectID)
		return nil
	}
	return printPlanList(w, plans, jsonOut)
}
