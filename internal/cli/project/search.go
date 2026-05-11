package project

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/peoplesvc"
	"github.com/iainmoffat/tdx/internal/svc/projectsvc"
)

func newSearchCmd(svc projectsvcAPI) *cobra.Command {
	var (
		managerFlag     string
		statusFlags     []string
		typeFlags       []string
		includeInactive bool
		limitFlag       int
		jsonFlag        bool
		profileFlag     string
	)
	cmd := &cobra.Command{
		Use:   "search [QUERY]",
		Short: "Search for projects by name and filters",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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
			people := peoplesvc.New(paths)

			authedUID, err := authedUIDFor(cmd.Context(), auth, profile)
			if err != nil {
				return err
			}

			var query string
			if len(args) > 0 {
				query = args[0]
			}

			if limitFlag < 1 {
				limitFlag = 50
			}
			if limitFlag > 500 {
				limitFlag = 500
			}

			return runProjectSearch(cmd.Context(), cmd.OutOrStdout(), s, people, profile, authedUID,
				query, managerFlag, statusFlags, typeFlags, includeInactive, limitFlag, jsonFlag)
		},
	}
	// --status accepts numeric IDs only for Phase 1 (no project-status resolution endpoint in scope).
	// --type accepts both names (resolved via ResolveTypeByName) and numeric IDs.
	cmd.Flags().StringVar(&managerFlag, "manager", "", "filter by manager me|UID|email")
	cmd.Flags().StringSliceVar(&statusFlags, "status", nil, "filter by status ID (numeric only; repeatable)")
	cmd.Flags().StringSliceVar(&typeFlags, "type", nil, "filter by type name or numeric ID (repeatable; name resolved via /api/projects/types)")
	cmd.Flags().BoolVar(&includeInactive, "include-inactive", false, "include inactive projects (default: active only)")
	cmd.Flags().IntVar(&limitFlag, "limit", 50, "max results (capped at 500)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runProjectSearch(ctx context.Context, w io.Writer, svc projectsvcAPI, people peoplesvcAPI,
	profile, authedUID, query, managerArg string, statusArgs, typeArgs []string,
	includeInactive bool, limit int, jsonOut bool) error {

	// Resolve --manager arg to UID.
	var managerUID string
	if managerArg != "" {
		uid, err := resolvePrincipal(ctx, people, profile, authedUID, managerArg)
		if err != nil {
			return fmt.Errorf("--manager: %w", err)
		}
		managerUID = uid
	}

	// Parse status IDs (numeric only per Phase 1 decision).
	var statusIDs []int
	for _, raw := range statusArgs {
		id, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("--status %q: only numeric IDs accepted in Phase 1 (no project-status name resolution endpoint)", raw)
		}
		statusIDs = append(statusIDs, id)
	}

	// Resolve type flags: numeric as-is; name → ResolveTypeByName.
	var typeIDs []int
	for _, raw := range typeArgs {
		if id, err := strconv.Atoi(raw); err == nil {
			typeIDs = append(typeIDs, id)
			continue
		}
		pt, err := svc.ResolveTypeByName(ctx, profile, raw)
		if err != nil {
			return fmt.Errorf("--type %q: %w", raw, err)
		}
		typeIDs = append(typeIDs, pt.ID)
	}

	isActive := true
	var isActivePtr *bool
	if !includeInactive {
		isActivePtr = &isActive
	}

	filter := domain.ProjectSearchFilter{
		NameLike:   query,
		ManagerUID: managerUID,
		StatusIDs:  statusIDs,
		TypeIDs:    typeIDs,
		IsActive:   isActivePtr,
		MaxResults: limit,
	}

	projects, err := svc.Search(ctx, profile, filter)
	if err != nil {
		return err
	}

	// Client-side manager filter as fallback (TD may silently ignore ManagerUID
	// in the request body — live-probe will verify; defensive post-filter never hurts).
	if managerUID != "" {
		filtered := projects[:0]
		for _, p := range projects {
			if p.ManagerUID == managerUID {
				filtered = append(filtered, p)
			}
		}
		projects = filtered
	}

	if len(projects) > limit {
		projects = projects[:limit]
	}

	return printProjectList(w, projects, jsonOut, "tdx.v1.projectList")
}
