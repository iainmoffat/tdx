package project

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/peoplesvc"
)

// peoplesvcAPI is the subset of peoplesvc used by project helpers.
type peoplesvcAPI interface {
	LookupPeople(ctx context.Context, profile string, q string, limit int) ([]domain.User, error)
	SearchUsers(ctx context.Context, profile string, filter domain.UserFilter) ([]domain.User, error)
	ResolveAccountByName(ctx context.Context, profile, name string) (peoplesvc.Account, error)
}

// resolvePrincipal maps a CLI argument to a UID (mirrors ticket/helpers.go).
//
//   - "me" → authedUID (must be provided by caller; error if empty)
//   - looks like a UID (32+ chars with at least 4 dashes) → returned as-is
//   - otherwise → looked up via people.LookupPeople with limit=5
func resolvePrincipal(ctx context.Context, people peoplesvcAPI, profile, authedUID, arg string) (string, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "", fmt.Errorf("principal argument required")
	}
	if arg == "me" {
		if authedUID == "" {
			return "", fmt.Errorf("`me` requires an authenticated session — run `tdx auth status` to verify")
		}
		return authedUID, nil
	}
	// UID heuristic: 36-char hex+dashes (TD UIDs are GUID-shaped)
	if len(arg) >= 32 && strings.Count(arg, "-") >= 4 {
		return arg, nil
	}
	if people == nil {
		return "", fmt.Errorf("cannot resolve %q without a people-service", arg)
	}
	users, err := people.LookupPeople(ctx, profile, arg, 5)
	if err != nil {
		return "", fmt.Errorf("look up %q: %w", arg, err)
	}
	switch len(users) {
	case 0:
		return "", fmt.Errorf("no user matches %q", arg)
	case 1:
		return users[0].UID, nil
	default:
		labels := make([]string, 0, len(users))
		for _, u := range users {
			labels = append(labels, fmt.Sprintf("%s (%s)", u.FullName, u.Email))
		}
		return "", fmt.Errorf("multiple users match %q: %s — pass UID directly", arg, strings.Join(labels, ", "))
	}
}

// authedUIDFor returns the authenticated user's UID for the given profile.
func authedUIDFor(ctx context.Context, auth *authsvc.Service, profile string) (string, error) {
	user, err := auth.WhoAmI(ctx, profile)
	if err != nil {
		return "", fmt.Errorf("resolve authenticated user: %w", err)
	}
	if user.UID == "" {
		return "", fmt.Errorf("authenticated user has no UID — try re-running `tdx auth login`")
	}
	return user.UID, nil
}

// formatDate renders a time.Time as "YYYY-MM-DD" or "" if zero.
func formatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// truncate shortens s to at most n runes, appending "…" if truncated.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}

// printProjectList renders a project list as table or JSON.
func printProjectList(w io.Writer, projects []domain.Project, jsonOut bool, schema string) error {
	if jsonOut {
		type projectJSON struct {
			ID              int     `json:"id"`
			Name            string  `json:"name"`
			StatusName      string  `json:"statusName,omitempty"`
			ManagerName     string  `json:"managerName,omitempty"`
			TypeName        string  `json:"typeName,omitempty"`
			PercentComplete float64 `json:"percentComplete,omitempty"`
			StartDate       string  `json:"startDate,omitempty"`
			EndDate         string  `json:"endDate,omitempty"`
			IsActive        bool    `json:"isActive,omitempty"`
		}
		out := make([]projectJSON, 0, len(projects))
		for _, p := range projects {
			out = append(out, projectJSON{
				ID: p.ID, Name: p.Name, StatusName: p.StatusName,
				ManagerName: p.ManagerName, TypeName: p.TypeName,
				PercentComplete: p.PercentComplete,
				StartDate:       formatDate(p.StartDate), EndDate: formatDate(p.EndDate),
				IsActive: p.IsActive,
			})
		}
		return render.JSON(w, struct {
			Schema   string        `json:"schema"`
			Projects []projectJSON `json:"projects"`
		}{Schema: schema, Projects: out})
	}
	if len(projects) == 0 {
		_, _ = fmt.Fprintln(w, "no projects found")
		return nil
	}
	headers := []string{"ID", "NAME", "STATUS", "MANAGER", "TYPE", "% COMPLETE", "START", "END"}
	rows := make([][]string, 0, len(projects))
	for _, p := range projects {
		rows = append(rows, []string{
			strconv.Itoa(p.ID),
			truncate(p.Name, 50),
			p.StatusName,
			p.ManagerName,
			p.TypeName,
			fmt.Sprintf("%.1f%%", p.PercentComplete),
			formatDate(p.StartDate),
			formatDate(p.EndDate),
		})
	}
	render.Table(w, headers, rows, nil)
	return nil
}

// printPlanList renders a plan list as table or JSON.
func printPlanList(w io.Writer, plans []domain.ProjectPlan, jsonOut bool) error {
	if jsonOut {
		type planJSON struct {
			PlanID          int     `json:"planID"`
			ProjectID       int     `json:"projectID"`
			ProjectName     string  `json:"projectName"`
			Title           string  `json:"title"`
			Type            string  `json:"type"`
			MyTaskCount     int     `json:"myTaskCount,omitempty"`
			TaskCount       int     `json:"taskCount,omitempty"`
			PercentComplete float64 `json:"percentComplete,omitempty"`
			StartDate       string  `json:"startDate,omitempty"`
			EndDate         string  `json:"endDate,omitempty"`
		}
		out := make([]planJSON, 0, len(plans))
		for _, p := range plans {
			out = append(out, planJSON{
				PlanID: p.ID, ProjectID: p.ProjectID, ProjectName: p.ProjectName,
				Title: p.Title, Type: p.Type.String(),
				MyTaskCount: p.MyTaskCount, TaskCount: p.TaskCount,
				PercentComplete: p.PercentComplete,
				StartDate:       formatDate(p.StartDate), EndDate: formatDate(p.EndDate),
			})
		}
		return render.JSON(w, struct {
			Schema string     `json:"schema"`
			Plans  []planJSON `json:"plans"`
		}{Schema: "tdx.v1.projectPlanList", Plans: out})
	}
	if len(plans) == 0 {
		_, _ = fmt.Fprintln(w, "no plans found")
		return nil
	}
	headers := []string{"PROJECT-ID", "PROJECT", "PLAN-ID", "PLAN", "TYPE", "MY-TASKS", "TASKS", "% COMPLETE", "START", "END"}
	rows := make([][]string, 0, len(plans))
	for _, p := range plans {
		rows = append(rows, []string{
			strconv.Itoa(p.ProjectID),
			truncate(p.ProjectName, 30),
			strconv.Itoa(p.ID),
			truncate(p.Title, 40),
			p.Type.String(),
			strconv.Itoa(p.MyTaskCount),
			strconv.Itoa(p.TaskCount),
			fmt.Sprintf("%.1f%%", p.PercentComplete),
			formatDate(p.StartDate),
			formatDate(p.EndDate),
		})
	}
	render.Table(w, headers, rows, nil)
	return nil
}

// printTaskList renders a task list as table or JSON.
func printTaskList(w io.Writer, tasks []domain.ProjectTask, jsonOut bool) error {
	if jsonOut {
		type taskJSON struct {
			ProjectID       int     `json:"projectID"`
			PlanID          int     `json:"planID"`
			TaskID          int     `json:"taskID"`
			Title           string  `json:"title"`
			Status          string  `json:"status,omitempty"`
			PercentComplete float64 `json:"percentComplete,omitempty"`
			EstimatedHours  float64 `json:"estimatedHours,omitempty"`
			ActualHours     float64 `json:"actualHours,omitempty"`
			EndDate         string  `json:"endDate,omitempty"`
			Assignees       string  `json:"assignees,omitempty"`
		}
		out := make([]taskJSON, 0, len(tasks))
		for _, t := range tasks {
			out = append(out, taskJSON{
				ProjectID: t.ProjectID, PlanID: t.PlanID, TaskID: t.ID,
				Title: t.Title, Status: t.Status,
				PercentComplete: t.PercentComplete,
				EstimatedHours:  t.EstimatedHours, ActualHours: t.ActualHours,
				EndDate:   formatDate(t.EndDate),
				Assignees: assigneeNames(t.Resources),
			})
		}
		return render.JSON(w, struct {
			Schema string     `json:"schema"`
			Tasks  []taskJSON `json:"tasks"`
		}{Schema: "tdx.v1.projectTaskList", Tasks: out})
	}
	if len(tasks) == 0 {
		_, _ = fmt.Fprintln(w, "no tasks found")
		return nil
	}
	headers := []string{"PROJECT", "PLAN", "TASK-ID", "TITLE", "STATUS", "%", "EST", "ACT", "ASSIGNEES", "END"}
	rows := make([][]string, 0, len(tasks))
	for _, t := range tasks {
		rows = append(rows, []string{
			strconv.Itoa(t.ProjectID),
			strconv.Itoa(t.PlanID),
			strconv.Itoa(t.ID),
			truncate(t.Title, 40),
			t.Status,
			fmt.Sprintf("%.0f%%", t.PercentComplete),
			fmt.Sprintf("%.1f", t.EstimatedHours),
			fmt.Sprintf("%.1f", t.ActualHours),
			truncate(assigneeNames(t.Resources), 30),
			formatDate(t.EndDate),
		})
	}
	render.Table(w, headers, rows, nil)
	return nil
}

// assigneeNames returns a comma-separated list of assignee full names.
func assigneeNames(resources []domain.ProjectTaskResource) string {
	names := make([]string, 0, len(resources))
	for _, r := range resources {
		if r.FullName != "" {
			names = append(names, r.FullName)
		}
	}
	return strings.Join(names, ", ")
}
