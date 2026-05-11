package project

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/projectsvc"
)

func newListCmd(svc projectsvcAPI) *cobra.Command {
	var (
		limitFlag   int
		jsonFlag    bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List your participating projects (one row per plan)",
		RunE: func(cmd *cobra.Command, _ []string) error {
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
			if limitFlag < 1 {
				limitFlag = 50
			}
			if limitFlag > 500 {
				limitFlag = 500
			}
			return runProjectList(cmd.Context(), cmd.OutOrStdout(), s, profile, limitFlag, jsonFlag)
		},
	}
	cmd.Flags().IntVar(&limitFlag, "limit", 50, "max results (capped at 500)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runProjectList(ctx context.Context, w io.Writer, svc projectsvcAPI, profile string, limit int, jsonOut bool) error {
	plans, err := svc.ListMine(ctx, profile)
	if err != nil {
		return err
	}
	// Sort by project name, then plan title.
	sort.Slice(plans, func(i, j int) bool {
		if plans[i].ProjectName != plans[j].ProjectName {
			return plans[i].ProjectName < plans[j].ProjectName
		}
		return plans[i].Title < plans[j].Title
	})
	if len(plans) > limit {
		plans = plans[:limit]
	}
	if len(plans) == 0 {
		_, _ = fmt.Fprintln(w, "no projects found — check your TD project participation")
		return nil
	}
	return printPlanList(w, plans, jsonOut)
}
