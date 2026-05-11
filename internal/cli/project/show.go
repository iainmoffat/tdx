package project

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	week "github.com/iainmoffat/tdx/internal/cli/time/week"
	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
	"github.com/iainmoffat/tdx/internal/svc/projectsvc"
)

// draftsvcShowAPI is the tiny draftsvc interface used by project show.
type draftsvcShowAPI interface {
	LoadDraft(profile string, weekStart time.Time, name string) (domain.WeekDraft, error)
}

// draftShowStoreAdapter wraps *draftsvc.Store to satisfy draftsvcShowAPI.
type draftShowStoreAdapter struct {
	store *draftsvc.Store
}

func (a *draftShowStoreAdapter) LoadDraft(profile string, weekStart time.Time, name string) (domain.WeekDraft, error) {
	return a.store.Load(profile, weekStart, name)
}

func newShowCmd(svc projectsvcAPI) *cobra.Command {
	var (
		jsonFlag    bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show full detail for one project",
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
			drafts := &draftShowStoreAdapter{store: draftsvc.NewStore(paths)}
			return runProjectShow(cmd.Context(), cmd.OutOrStdout(), s, drafts, profile, id, jsonFlag)
		},
	}
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runProjectShow(ctx context.Context, w io.Writer, svc projectsvcAPI, drafts draftsvcShowAPI, profile string, id int, jsonOut bool) error {
	p, err := svc.Get(ctx, profile, id)
	if err != nil {
		return err
	}

	weekHours, weekEntries := thisWeekForProject(drafts, profile, id)

	if jsonOut {
		return printProjectShowJSON(w, p, weekHours, weekEntries)
	}
	return printProjectShowText(w, p, weekHours, weekEntries)
}

// thisWeekForProject returns (totalHours, entryCount) for the current week's
// draft rows that target the given project. Returns (0, 0) on any error.
func thisWeekForProject(drafts draftsvcShowAPI, profile string, projectID int) (float64, int) {
	if drafts == nil {
		return 0, 0
	}
	weekStart, name, err := week.ResolveWeekRef("")
	if err != nil {
		return 0, 0
	}
	d, err := drafts.LoadDraft(profile, weekStart, name)
	if err != nil {
		return 0, 0
	}
	var totalHours float64
	var count int
	for _, row := range d.Rows {
		if row.Target.Kind != domain.TargetProjectTask && row.Target.Kind != domain.TargetProject {
			continue
		}
		if row.Target.ProjectID != projectID && row.Target.ItemID != projectID {
			continue
		}
		for _, cell := range row.Cells {
			if cell.Hours > 0 {
				totalHours += cell.Hours
				count++
			}
		}
	}
	return totalHours, count
}

func printProjectShowText(w io.Writer, p domain.Project, weekHours float64, weekEntries int) error {
	_, _ = fmt.Fprintf(w, "PROJECT %d — %s\n", p.ID, p.Name)
	_, _ = fmt.Fprintln(w)
	if p.StatusName != "" {
		_, _ = fmt.Fprintf(w, "Status:      %s\n", p.StatusName)
	}
	if p.TypeName != "" {
		_, _ = fmt.Fprintf(w, "Type:        %s\n", p.TypeName)
	}
	if p.ManagerName != "" {
		_, _ = fmt.Fprintf(w, "Manager:     %s\n", p.ManagerName)
	}
	if p.SponsorName != "" {
		_, _ = fmt.Fprintf(w, "Sponsor:     %s\n", p.SponsorName)
	}
	if p.AccountName != "" {
		_, _ = fmt.Fprintf(w, "Account:     %s\n", p.AccountName)
	}
	activeStr := "no"
	if p.IsActive {
		activeStr = "yes"
	}
	_, _ = fmt.Fprintf(w, "Active:      %s\n", activeStr)
	_, _ = fmt.Fprintf(w, "%% Complete:  %.1f%%\n", p.PercentComplete)
	_, _ = fmt.Fprintf(w, "Hours:       actual=%.1f / estimated=%.1f\n", p.ActualHours, p.EstimatedHours)
	if !p.StartDate.IsZero() || !p.EndDate.IsZero() {
		_, _ = fmt.Fprintf(w, "Dates:       %s → %s\n", formatDate(p.StartDate), formatDate(p.EndDate))
	}
	if !p.ModifiedDate.IsZero() {
		_, _ = fmt.Fprintf(w, "Modified:    %s\n", formatDate(p.ModifiedDate))
	}
	if weekEntries > 0 {
		entry := "entries"
		if weekEntries == 1 {
			entry = "entry"
		}
		_, _ = fmt.Fprintf(w, "This week:   %.2fh (%d %s)\n", weekHours, weekEntries, entry)
	}
	return nil
}

func printProjectShowJSON(w io.Writer, p domain.Project, weekHours float64, weekEntries int) error {
	type thisWeekJSON struct {
		Hours   float64 `json:"hours"`
		Entries int     `json:"entries"`
	}
	type projectJSON struct {
		ID              int          `json:"id"`
		Name            string       `json:"name"`
		StatusID        int          `json:"statusID,omitempty"`
		StatusName      string       `json:"statusName,omitempty"`
		TypeID          int          `json:"typeID,omitempty"`
		TypeName        string       `json:"typeName,omitempty"`
		AccountID       int          `json:"accountID,omitempty"`
		AccountName     string       `json:"accountName,omitempty"`
		ManagerUID      string       `json:"managerUID,omitempty"`
		ManagerName     string       `json:"managerName,omitempty"`
		SponsorUID      string       `json:"sponsorUID,omitempty"`
		SponsorName     string       `json:"sponsorName,omitempty"`
		PercentComplete float64      `json:"percentComplete,omitempty"`
		EstimatedHours  float64      `json:"estimatedHours,omitempty"`
		ActualHours     float64      `json:"actualHours,omitempty"`
		StartDate       string       `json:"startDate,omitempty"`
		EndDate         string       `json:"endDate,omitempty"`
		ModifiedDate    string       `json:"modifiedDate,omitempty"`
		IsActive        bool         `json:"isActive,omitempty"`
		Description     string       `json:"description,omitempty"`
		ThisWeek        thisWeekJSON `json:"thisWeek"`
	}
	out := projectJSON{
		ID:              p.ID,
		Name:            p.Name,
		StatusID:        p.StatusID,
		StatusName:      p.StatusName,
		TypeID:          p.TypeID,
		TypeName:        p.TypeName,
		AccountID:       p.AccountID,
		AccountName:     p.AccountName,
		ManagerUID:      p.ManagerUID,
		ManagerName:     p.ManagerName,
		SponsorUID:      p.SponsorUID,
		SponsorName:     p.SponsorName,
		PercentComplete: p.PercentComplete,
		EstimatedHours:  p.EstimatedHours,
		ActualHours:     p.ActualHours,
		StartDate:       formatDate(p.StartDate),
		EndDate:         formatDate(p.EndDate),
		ModifiedDate:    formatDate(p.ModifiedDate),
		IsActive:        p.IsActive,
		Description:     p.Description,
		ThisWeek:        thisWeekJSON{Hours: weekHours, Entries: weekEntries},
	}
	return render.JSON(w, struct {
		Schema  string      `json:"schema"`
		Project projectJSON `json:"project"`
	}{Schema: "tdx.v1.project", Project: out})
}
