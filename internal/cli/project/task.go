package project

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"sync"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/projectsvc"
)

const maxPlanFanout = 50
const taskFanoutConcurrency = 5

func newTaskCmd(svc projectsvcAPI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage project tasks",
	}
	cmd.AddCommand(newTaskListCmd(svc))
	cmd.AddCommand(newTaskShowCmd(svc))
	return cmd
}

func newTaskListCmd(svc projectsvcAPI) *cobra.Command {
	var (
		planIDFlag  int
		mineFlag    bool
		limitFlag   int
		jsonFlag    bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "list [<project-id>]",
		Short: "List project tasks (--plan required when project-id given; --mine for cross-project)",
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

			if mineFlag && (len(args) > 0 || planIDFlag != 0) {
				return fmt.Errorf("--mine is mutually exclusive with <project-id> and --plan")
			}

			if limitFlag < 1 {
				limitFlag = 50
			}
			if limitFlag > 200 {
				limitFlag = 200
			}

			if mineFlag {
				authedUID, err := authedUIDFor(cmd.Context(), auth, profile)
				if err != nil {
					return err
				}
				return runTaskListMine(cmd.Context(), cmd.OutOrStdout(), s, profile, authedUID, limitFlag, jsonFlag)
			}

			// Single-plan mode: project-id + --plan required.
			if len(args) == 0 {
				return fmt.Errorf("provide <project-id> --plan <plan-id>, or use --mine for your tasks across all projects")
			}
			projectID, err := strconv.Atoi(args[0])
			if err != nil || projectID <= 0 {
				return fmt.Errorf("project id must be a positive integer, got %q", args[0])
			}
			if planIDFlag == 0 {
				return fmt.Errorf("--plan is required when specifying a project-id (run `tdx project plan list %d` to see available plans)", projectID)
			}
			return runTaskListSinglePlan(cmd.Context(), cmd.OutOrStdout(), s, profile, projectID, planIDFlag, limitFlag, jsonFlag)
		},
	}
	cmd.Flags().IntVar(&planIDFlag, "plan", 0, "plan ID (required when project-id is given)")
	cmd.Flags().BoolVar(&mineFlag, "mine", false, "list my tasks across all projects (mutually exclusive with project-id and --plan)")
	cmd.Flags().IntVar(&limitFlag, "limit", 50, "max results (default 50, max 200)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runTaskListSinglePlan(ctx context.Context, w io.Writer, svc projectsvcAPI, profile string, projectID, planID, limit int, jsonOut bool) error {
	tasks, err := svc.ListTasks(ctx, profile, projectID, planID)
	if err != nil {
		return err
	}
	if len(tasks) > limit {
		tasks = tasks[:limit]
	}
	return printTaskList(w, tasks, jsonOut)
}

func runTaskListMine(ctx context.Context, w io.Writer, svc projectsvcAPI, profile, authedUID string, limit int, jsonOut bool) error {
	// Step 1: Get my plans.
	plans, err := svc.ListMine(ctx, profile)
	if err != nil {
		return err
	}

	// Step 2: Filter to plans with MyTaskCount > 0.
	var candidates []domain.ProjectPlan
	for _, p := range plans {
		if p.MyTaskCount > 0 {
			candidates = append(candidates, p)
		}
	}

	if len(candidates) == 0 {
		_, _ = fmt.Fprintln(w, "no projects/plans assigned to you on this tenant")
		return nil
	}

	// Step 3: Cap at maxPlanFanout.
	if len(candidates) > maxPlanFanout {
		candidates = candidates[:maxPlanFanout]
	}

	// Step 4: Fetch tasks in parallel (errgroup with bounded concurrency).
	var mu sync.Mutex
	var allTasks []domain.ProjectTask

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(taskFanoutConcurrency)

	for _, plan := range candidates {
		p := plan // capture
		g.Go(func() error {
			tasks, err := svc.ListTasks(gctx, profile, p.ProjectID, p.ID)
			if err != nil {
				return err
			}
			// Step 5: Filter to tasks assigned to authedUID.
			for _, t := range tasks {
				if t.AssignedTo(authedUID) {
					// Enrich plan context for display.
					if t.PlanID == 0 {
						t.PlanID = p.ID
					}
					if t.ProjectID == 0 {
						t.ProjectID = p.ProjectID
					}
					mu.Lock()
					allTasks = append(allTasks, t)
					mu.Unlock()
				}
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	if len(allTasks) == 0 {
		_, _ = fmt.Fprintf(w, "no tasks assigned to you across %d plans\n", len(candidates))
		return nil
	}

	// Step 6: Sort by EndDate ASC (zero last), then ModifiedDate DESC.
	sort.Slice(allTasks, func(i, j int) bool {
		ei := allTasks[i].EndDate
		ej := allTasks[j].EndDate
		if ei.IsZero() != ej.IsZero() {
			return !ei.IsZero() // non-zero before zero
		}
		if !ei.Equal(ej) {
			return ei.Before(ej)
		}
		return allTasks[i].ModifiedDate.After(allTasks[j].ModifiedDate)
	})

	// Step 7: Cap at limit.
	if len(allTasks) > limit {
		allTasks = allTasks[:limit]
	}

	return printTaskList(w, allTasks, jsonOut)
}

func newTaskShowCmd(svc projectsvcAPI) *cobra.Command {
	var (
		planIDFlag  int
		jsonFlag    bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "show <project-id> <task-id>",
		Short: "Show full detail for one project task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID, err := strconv.Atoi(args[0])
			if err != nil || projectID <= 0 {
				return fmt.Errorf("project id must be a positive integer, got %q", args[0])
			}
			taskID, err := strconv.Atoi(args[1])
			if err != nil || taskID <= 0 {
				return fmt.Errorf("task id must be a positive integer, got %q", args[1])
			}
			if planIDFlag == 0 {
				return fmt.Errorf("--plan is required (run `tdx project plan list %d` to find plan IDs)", projectID)
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
			return runTaskShow(cmd.Context(), cmd.OutOrStdout(), s, profile, projectID, planIDFlag, taskID, jsonFlag)
		},
	}
	cmd.Flags().IntVar(&planIDFlag, "plan", 0, "plan ID (required)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runTaskShow(ctx context.Context, w io.Writer, svc projectsvcAPI, profile string, projectID, planID, taskID int, jsonOut bool) error {
	task, err := svc.GetTask(ctx, profile, projectID, planID, taskID)
	if err != nil {
		return err
	}
	if jsonOut {
		return printTaskShowJSON(w, task)
	}
	return printTaskShowText(w, task)
}

func printTaskShowText(w io.Writer, t domain.ProjectTask) error {
	_, _ = fmt.Fprintf(w, "TASK %d — %s\n", t.ID, t.Title)
	_, _ = fmt.Fprintln(w)
	if t.Status != "" {
		_, _ = fmt.Fprintf(w, "Status:      %s\n", t.Status)
	}
	_, _ = fmt.Fprintf(w, "%% Complete:  %.1f%%\n", t.PercentComplete)
	_, _ = fmt.Fprintf(w, "Hours:       actual=%.1f / estimated=%.1f / remaining=%.1f\n",
		t.ActualHours, t.EstimatedHours, t.RemainingHours)
	if !t.StartDate.IsZero() || !t.EndDate.IsZero() {
		_, _ = fmt.Fprintf(w, "Dates:       %s → %s\n", formatDate(t.StartDate), formatDate(t.EndDate))
	}
	if !t.ModifiedDate.IsZero() {
		_, _ = fmt.Fprintf(w, "Modified:    %s\n", formatDate(t.ModifiedDate))
	}
	if t.OutlineNumber != "" {
		_, _ = fmt.Fprintf(w, "Outline:     %s (indent=%d)\n", t.OutlineNumber, t.IndentLevel)
	}
	if len(t.Resources) > 0 {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, "Resources:")
		headers := []string{"UID", "NAME", "ROLE", "%"}
		rows := make([][]string, 0, len(t.Resources))
		for _, r := range t.Resources {
			rows = append(rows, []string{
				r.UID, r.FullName, r.RoleName,
				fmt.Sprintf("%.0f%%", r.PercentAssigned),
			})
		}
		render.Table(w, headers, rows, nil)
	}
	if t.Description != "" {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, "Description:")
		lines := splitDescription(t.Description)
		if len(lines) > 10 {
			lines = lines[:10]
		}
		for _, line := range lines {
			_, _ = fmt.Fprintln(w, "  "+line)
		}
	}
	return nil
}

func printTaskShowJSON(w io.Writer, t domain.ProjectTask) error {
	type resourceJSON struct {
		UID             string  `json:"uid"`
		FullName        string  `json:"fullName"`
		RoleName        string  `json:"roleName,omitempty"`
		PercentAssigned float64 `json:"percentAssigned,omitempty"`
	}
	type taskJSON struct {
		ProjectID       int            `json:"projectID"`
		PlanID          int            `json:"planID"`
		TaskID          int            `json:"taskID"`
		Title           string         `json:"title"`
		Status          string         `json:"status,omitempty"`
		StatusID        int            `json:"statusID,omitempty"`
		PercentComplete float64        `json:"percentComplete,omitempty"`
		EstimatedHours  float64        `json:"estimatedHours,omitempty"`
		ActualHours     float64        `json:"actualHours,omitempty"`
		RemainingHours  float64        `json:"remainingHours,omitempty"`
		StartDate       string         `json:"startDate,omitempty"`
		EndDate         string         `json:"endDate,omitempty"`
		ModifiedDate    string         `json:"modifiedDate,omitempty"`
		IsParent        bool           `json:"isParent,omitempty"`
		IndentLevel     int            `json:"indentLevel,omitempty"`
		OutlineNumber   string         `json:"outlineNumber,omitempty"`
		Description     string         `json:"description,omitempty"`
		Resources       []resourceJSON `json:"resources,omitempty"`
	}
	resources := make([]resourceJSON, 0, len(t.Resources))
	for _, r := range t.Resources {
		resources = append(resources, resourceJSON{
			UID: r.UID, FullName: r.FullName, RoleName: r.RoleName,
			PercentAssigned: r.PercentAssigned,
		})
	}
	out := taskJSON{
		ProjectID: t.ProjectID, PlanID: t.PlanID, TaskID: t.ID,
		Title: t.Title, Status: t.Status, StatusID: t.StatusID,
		PercentComplete: t.PercentComplete, EstimatedHours: t.EstimatedHours,
		ActualHours: t.ActualHours, RemainingHours: t.RemainingHours,
		StartDate: formatDate(t.StartDate), EndDate: formatDate(t.EndDate),
		ModifiedDate: formatDate(t.ModifiedDate),
		IsParent:     t.IsParent, IndentLevel: t.IndentLevel, OutlineNumber: t.OutlineNumber,
		Description: t.Description, Resources: resources,
	}
	return render.JSON(w, struct {
		Schema string   `json:"schema"`
		Task   taskJSON `json:"task"`
	}{Schema: "tdx.v1.projectTask", Task: out})
}

func splitDescription(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == '\n' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	out = append(out, cur)
	return out
}
