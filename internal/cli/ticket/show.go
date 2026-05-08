package ticket

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
	"github.com/iainmoffat/tdx/internal/svc/ticketsvc"
)

// draftsvcAPI is the tiny subset of draftsvc used here for computing the
// "this week" time crossover. Defined as an interface so tests can stub.
type draftsvcAPI interface {
	LoadDraft(profile string, weekStart time.Time, name string) (domain.WeekDraft, error)
}

// draftStoreAdapter wraps *draftsvc.Store to satisfy draftsvcAPI.
type draftStoreAdapter struct {
	store *draftsvc.Store
}

func (a *draftStoreAdapter) LoadDraft(profile string, weekStart time.Time, name string) (domain.WeekDraft, error) {
	return a.store.Load(profile, weekStart, name)
}

func newShowCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		appID       int
		jsonFlag    bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show full detail for one ticket",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return fmt.Errorf("ticket id must be a positive integer, got %q", args[0])
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
				s = ticketsvc.New(paths)
			}

			drafts := &draftStoreAdapter{store: draftsvc.NewStore(paths)}

			return runTicketShow(cmd.Context(), cmd.OutOrStdout(), s, drafts, profile, appID, id, jsonFlag)
		},
	}
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id (overrides profile default)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runTicketShow(ctx context.Context, w io.Writer, svc ticketsvcAPI, drafts draftsvcAPI, profile string, appID, id int, jsonOut bool) error {
	t, err := svc.GetTicket(ctx, profile, appID, id)
	if err != nil {
		return err
	}

	weekHours, weekEntries := thisWeekForTicket(drafts, profile, id)

	if jsonOut {
		return printTicketShowJSON(w, t, weekHours, weekEntries)
	}
	return printTicketShowText(w, t, weekHours, weekEntries)
}

// thisWeekForTicket returns (totalHours, entryCount) for cells in the
// current week's draft that target the given ticket. Returns (0, 0) on
// any error (no draft, missing draft, etc.) — never propagates.
func thisWeekForTicket(drafts draftsvcAPI, profile string, ticketID int) (float64, int) {
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
		if row.Target.Kind != domain.TargetTicket || row.Target.ItemID != ticketID {
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

func printTicketShowText(w io.Writer, t domain.Ticket, weekHours float64, weekEntries int) error {
	_, _ = fmt.Fprintf(w, "#%d — %s\n", t.ID, t.Title)
	_, _ = fmt.Fprintln(w)
	if t.StatusName != "" {
		_, _ = fmt.Fprintf(w, "Status:    %s\n", t.StatusName)
	}
	if t.TypeName != "" {
		_, _ = fmt.Fprintf(w, "Type:      %s\n", t.TypeName)
	}
	if t.PriorityName != "" {
		_, _ = fmt.Fprintf(w, "Priority:  %s\n", t.PriorityName)
	}
	if t.AccountName != "" {
		_, _ = fmt.Fprintf(w, "Account:   %s\n", t.AccountName)
	}
	if t.ResponsibleName != "" {
		_, _ = fmt.Fprintf(w, "Assignee:  %s\n", t.ResponsibleName)
	}
	if t.RequestorName != "" {
		_, _ = fmt.Fprintf(w, "Requestor: %s\n", t.RequestorName)
	}
	if !t.CreatedDate.IsZero() {
		_, _ = fmt.Fprintf(w, "Created:   %s\n", t.CreatedDate.Format("2006-01-02 15:04"))
	}
	if !t.ModifiedDate.IsZero() {
		_, _ = fmt.Fprintf(w, "Modified:  %s\n", t.ModifiedDate.Format("2006-01-02 15:04"))
	}

	// Time line: TD's est/act + this week (local)
	estStr := formatHours(float64(t.EstimatedMinutes) / 60)
	actStr := formatHours(float64(t.ActualMinutes) / 60)
	thisWeekStr := formatHours(weekHours)
	entriesPart := ""
	if weekEntries > 0 {
		entry := "entries"
		if weekEntries == 1 {
			entry = "entry"
		}
		entriesPart = fmt.Sprintf(" (%d %s)", weekEntries, entry)
	}
	_, _ = fmt.Fprintf(w, "Time:      EST: %s  ACT: %s (TD)  |  this week: %s%s\n", estStr, actStr, thisWeekStr, entriesPart)

	if len(t.Tags) > 0 {
		_, _ = fmt.Fprintf(w, "Tags:      %v\n", t.Tags)
	}

	if t.Description != "" {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, "Description:")
		for _, line := range splitLines(t.Description) {
			_, _ = fmt.Fprintln(w, "  "+line)
		}
	}
	return nil
}

func printTicketShowJSON(w io.Writer, t domain.Ticket, weekHours float64, weekEntries int) error {
	type thisWeekJSON struct {
		Hours   float64 `json:"hours"`
		Entries int     `json:"entries"`
	}
	type ticketJSON struct {
		ID               int          `json:"id"`
		AppID            int          `json:"appID,omitempty"`
		Title            string       `json:"title"`
		Description      string       `json:"description,omitempty"`
		StatusName       string       `json:"statusName,omitempty"`
		TypeName         string       `json:"typeName,omitempty"`
		PriorityName     string       `json:"priorityName,omitempty"`
		AccountName      string       `json:"accountName,omitempty"`
		AssigneeUID      string       `json:"assigneeUID,omitempty"`
		AssigneeName     string       `json:"assigneeName,omitempty"`
		RequestorUID     string       `json:"requestorUID,omitempty"`
		RequestorName    string       `json:"requestorName,omitempty"`
		CreatedDate      string       `json:"createdDate,omitempty"`
		ModifiedDate     string       `json:"modifiedDate,omitempty"`
		EstimatedMinutes int          `json:"estimatedMinutes,omitempty"`
		ActualMinutes    int          `json:"actualMinutes,omitempty"`
		Tags             []string     `json:"tags,omitempty"`
		ThisWeek         thisWeekJSON `json:"thisWeek"`
	}
	out := ticketJSON{
		ID:               t.ID,
		AppID:            t.AppID,
		Title:            t.Title,
		Description:      t.Description,
		StatusName:       t.StatusName,
		TypeName:         t.TypeName,
		PriorityName:     t.PriorityName,
		AccountName:      t.AccountName,
		AssigneeUID:      t.ResponsibleUID,
		AssigneeName:     t.ResponsibleName,
		RequestorUID:     t.RequestorUID,
		RequestorName:    t.RequestorName,
		CreatedDate:      formatRFC3339(t.CreatedDate),
		ModifiedDate:     formatRFC3339(t.ModifiedDate),
		EstimatedMinutes: t.EstimatedMinutes,
		ActualMinutes:    t.ActualMinutes,
		Tags:             t.Tags,
		ThisWeek:         thisWeekJSON{Hours: weekHours, Entries: weekEntries},
	}
	return render.JSON(w, struct {
		Schema string     `json:"schema"`
		Ticket ticketJSON `json:"ticket"`
	}{Schema: "tdx.v1.ticket", Ticket: out})
}

func formatHours(h float64) string {
	if h == 0 {
		return "0h"
	}
	if h == float64(int(h)) {
		return fmt.Sprintf("%dh", int(h))
	}
	return fmt.Sprintf("%gh", h)
}

func formatRFC3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func splitLines(s string) []string {
	out := []string{}
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
