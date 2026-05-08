package ticket

import (
	"context"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/ticketsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

// newTaskCmd assembles the `tdx ticket task` sub-tree.
func newTaskCmd(svc ticketsvcAPI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage ticket tasks (list, show, feed, update, log time)",
	}
	cmd.AddCommand(newTaskListCmd(svc))
	cmd.AddCommand(newTaskShowCmd(svc))
	cmd.AddCommand(newTaskFeedCmd(svc))
	cmd.AddCommand(newTaskUpdateCmd(svc))
	cmd.AddCommand(newTaskLogCmd(svc))
	return cmd
}

// --- task list ---

func newTaskListCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		appID       int
		jsonFlag    bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "list <ticket-id>",
		Short: "List tasks on a ticket",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ticketID, err := strconv.Atoi(args[0])
			if err != nil || ticketID <= 0 {
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
			return runTaskList(cmd.Context(), cmd.OutOrStdout(), s, profile, appID, ticketID, jsonFlag)
		},
	}
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id (overrides profile default)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runTaskList(ctx context.Context, w io.Writer, svc ticketsvcAPI, profile string, appID, ticketID int, jsonOut bool) error {
	tasks, err := svc.ListTasks(ctx, profile, appID, ticketID)
	if err != nil {
		return err
	}
	if jsonOut {
		type taskJSON struct {
			ID               int    `json:"id"`
			TicketID         int    `json:"ticketID"`
			Title            string `json:"title"`
			PercentComplete  int    `json:"percentComplete"`
			EstimatedMinutes int    `json:"estimatedMinutes,omitempty"`
			ActualMinutes    int    `json:"actualMinutes,omitempty"`
			ResponsibleName  string `json:"responsibleName,omitempty"`
			Order            int    `json:"order"`
		}
		out := make([]taskJSON, 0, len(tasks))
		for _, t := range tasks {
			resp := t.ResponsibleName
			if resp == "" && t.ResponsibleGroupName != "" {
				resp = t.ResponsibleGroupName + " (group)"
			}
			out = append(out, taskJSON{
				ID: t.ID, TicketID: t.TicketID, Title: t.Title,
				PercentComplete:  t.PercentComplete,
				EstimatedMinutes: t.EstimatedMinutes, ActualMinutes: t.ActualMinutes,
				ResponsibleName: resp, Order: t.Order,
			})
		}
		return render.JSON(w, struct {
			Schema string     `json:"schema"`
			Tasks  []taskJSON `json:"tasks"`
		}{Schema: "tdx.v1.ticketTaskList", Tasks: out})
	}
	if len(tasks) == 0 {
		_, _ = fmt.Fprintln(w, "no tasks found on this ticket")
		return nil
	}
	headers := []string{"ID", "TITLE", "%COMPLETE", "EST", "ACT", "RESPONSIBLE"}
	rows := make([][]string, 0, len(tasks))
	for _, t := range tasks {
		resp := t.ResponsibleName
		if resp == "" && t.ResponsibleGroupName != "" {
			resp = t.ResponsibleGroupName + " (group)"
		}
		rows = append(rows, []string{
			strconv.Itoa(t.ID),
			truncate(t.Title, 50),
			fmt.Sprintf("%d%%", t.PercentComplete),
			formatDuration(t.EstimatedMinutes),
			formatDuration(t.ActualMinutes),
			resp,
		})
	}
	render.Table(w, headers, rows, nil)
	return nil
}

// --- task show ---

func newTaskShowCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		appID       int
		jsonFlag    bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "show <ticket-id> <task-id>",
		Short: "Show full detail for one ticket task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ticketID, taskID, err := parseTaskIDs(args)
			if err != nil {
				return err
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
			return runTaskShow(cmd.Context(), cmd.OutOrStdout(), s, profile, appID, ticketID, taskID, jsonFlag)
		},
	}
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runTaskShow(ctx context.Context, w io.Writer, svc ticketsvcAPI, profile string, appID, ticketID, taskID int, jsonOut bool) error {
	t, err := svc.GetTask(ctx, profile, appID, ticketID, taskID)
	if err != nil {
		return err
	}
	if jsonOut {
		return render.JSON(w, struct {
			Schema string            `json:"schema"`
			Task   domain.TicketTask `json:"task"`
		}{Schema: "tdx.v1.ticketTask", Task: t})
	}
	_, _ = fmt.Fprintf(w, "#%d / task #%d — %s\n\n", t.TicketID, t.ID, t.Title)
	_, _ = fmt.Fprintf(w, "Progress:    %d%%\n", t.PercentComplete)
	if !t.Active {
		_, _ = fmt.Fprintln(w, "Status:      INACTIVE")
	}
	if t.ResponsibleName != "" {
		_, _ = fmt.Fprintf(w, "Responsible: %s\n", t.ResponsibleName)
	} else if t.ResponsibleGroupName != "" {
		_, _ = fmt.Fprintf(w, "Responsible: %s (group)\n", t.ResponsibleGroupName)
	}
	if !t.CreatedDate.IsZero() {
		_, _ = fmt.Fprintf(w, "Created:     %s by %s\n", t.CreatedDate.Format("2006-01-02 15:04"), t.CreatedName)
	}
	if !t.ModifiedDate.IsZero() {
		_, _ = fmt.Fprintf(w, "Modified:    %s\n", t.ModifiedDate.Format("2006-01-02 15:04"))
	}
	if !t.CompletedDate.IsZero() {
		_, _ = fmt.Fprintf(w, "Completed:   %s by %s\n", t.CompletedDate.Format("2006-01-02 15:04"), t.CompletedName)
	}
	_, _ = fmt.Fprintf(w, "Time:        EST: %s  ACT: %s\n", formatDuration(t.EstimatedMinutes), formatDuration(t.ActualMinutes))
	if t.Description != "" {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, "Description:")
		for _, line := range splitLines(t.Description) {
			_, _ = fmt.Fprintln(w, "  "+line)
		}
	}
	return nil
}

// --- task feed ---

func newTaskFeedCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		appID       int
		limit       int
		jsonFlag    bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "feed <ticket-id> <task-id>",
		Short: "Read the feed for a ticket task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ticketID, taskID, err := parseTaskIDs(args)
			if err != nil {
				return err
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
			return runTaskFeed(cmd.Context(), cmd.OutOrStdout(), s, profile, appID, ticketID, taskID, limit, jsonFlag)
		},
	}
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id")
	cmd.Flags().IntVar(&limit, "limit", 0, "max entries (0 = all)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runTaskFeed(ctx context.Context, w io.Writer, svc ticketsvcAPI, profile string, appID, ticketID, taskID, limit int, jsonOut bool) error {
	entries, err := svc.GetTaskFeed(ctx, profile, appID, ticketID, taskID)
	if err != nil {
		return err
	}
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	if jsonOut {
		type entryJSON struct {
			ID         int    `json:"id"`
			AuthorName string `json:"authorName,omitempty"`
			CreatedAt  string `json:"createdAt,omitempty"`
			Body       string `json:"body,omitempty"`
			IsPrivate  bool   `json:"isPrivate"`
			EventKind  string `json:"eventKind,omitempty"`
		}
		out := make([]entryJSON, 0, len(entries))
		for _, e := range entries {
			ts := ""
			if !e.CreatedAt.IsZero() {
				ts = e.CreatedAt.Format(time.RFC3339)
			}
			out = append(out, entryJSON{ID: e.ID, AuthorName: e.AuthorName, CreatedAt: ts, Body: e.Body, IsPrivate: e.IsPrivate, EventKind: e.EventKind})
		}
		return render.JSON(w, struct {
			Schema  string      `json:"schema"`
			Entries []entryJSON `json:"entries"`
		}{Schema: "tdx.v1.ticketTaskFeed", Entries: out})
	}
	if len(entries) == 0 {
		_, _ = fmt.Fprintln(w, "no feed entries")
		return nil
	}
	for i, e := range entries {
		when := ""
		if !e.CreatedAt.IsZero() {
			when = e.CreatedAt.Format("2006-01-02 15:04")
		}
		_, _ = fmt.Fprintf(w, "[%s] %s — %s\n", when, e.AuthorName, e.EventKind)
		if e.Body != "" {
			for _, line := range splitLines(e.Body) {
				_, _ = fmt.Fprintln(w, "  "+line)
			}
		}
		if i < len(entries)-1 {
			_, _ = fmt.Fprintln(w)
		}
	}
	return nil
}

// --- task update (mutating) ---

func newTaskUpdateCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		appID        int
		percentFlag  int
		completeFlag bool
		commentFlag  string
		hoursWorked  float64
		privateFlag  bool
		notifyFlag   []string
		yesFlag      bool
		profileFlag  string
	)
	cmd := &cobra.Command{
		Use:   "update <ticket-id> <task-id>",
		Short: "Post a feed update to a ticket task (--yes required; --hours-worked is informational only — use `tdx ticket task log` for real time entries)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ticketID, taskID, err := parseTaskIDs(args)
			if err != nil {
				return err
			}
			percentSet := cmd.Flags().Changed("percent")
			if !yesFlag {
				return fmt.Errorf("pass --yes to update the task")
			}
			if percentSet && completeFlag {
				return fmt.Errorf("--percent and --complete are mutually exclusive")
			}
			if !percentSet && !completeFlag && commentFlag == "" && hoursWorked == 0 {
				return fmt.Errorf("nothing to update — pass at least one of --percent / --complete / --comment / --hours-worked")
			}
			var pc *int
			if completeFlag {
				v := 100
				pc = &v
			} else if percentSet {
				if percentFlag < 0 || percentFlag > 100 {
					return fmt.Errorf("--percent must be 0-100, got %d", percentFlag)
				}
				v := percentFlag
				pc = &v
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
			return runTaskUpdate(cmd.Context(), cmd.OutOrStdout(), s, profile, appID, ticketID, taskID, commentFlag, pc, hoursWorked, privateFlag, notifyFlag)
		},
	}
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id")
	cmd.Flags().IntVar(&percentFlag, "percent", 0, "percent complete (0-100)")
	cmd.Flags().BoolVar(&completeFlag, "complete", false, "shortcut for --percent 100")
	cmd.Flags().StringVar(&commentFlag, "comment", "", "comment body")
	cmd.Flags().Float64Var(&hoursWorked, "hours-worked", 0, "hours worked (informational only — does NOT create a time entry)")
	cmd.Flags().BoolVar(&privateFlag, "private", false, "internal note (not visible to requestor)")
	cmd.Flags().StringSliceVar(&notifyFlag, "notify", nil, "additional notify recipients by UID (repeatable)")
	cmd.Flags().BoolVar(&yesFlag, "yes", false, "required to send")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runTaskUpdate(ctx context.Context, w io.Writer, svc ticketsvcAPI, profile string, appID, ticketID, taskID int, body string, pc *int, hoursWorked float64, isPrivate bool, notify []string) error {
	feedID, err := svc.UpdateTaskFeed(ctx, profile, appID, ticketID, taskID, body, pc, hoursWorked, isPrivate, notify)
	if err != nil {
		return err
	}
	parts := []string{}
	if pc != nil {
		parts = append(parts, fmt.Sprintf("percent=%d%%", *pc))
	}
	if body != "" {
		parts = append(parts, fmt.Sprintf("comment=%q", truncate(body, 40)))
	}
	if hoursWorked > 0 {
		parts = append(parts, fmt.Sprintf("hours-worked=%g (informational)", hoursWorked))
	}
	summary := strings.Join(parts, ", ")
	_, _ = fmt.Fprintf(w, "task #%d/#%d updated: %s (feed entry %d)\n", ticketID, taskID, summary, feedID)
	return nil
}

// --- task log (mutating, time crossover) ---

type taskLogArgs struct {
	profile     string
	authedUID   string
	appID       int
	ticketID    int
	taskID      int
	hours       float64
	minutes     int
	typeName    string
	typeID      int
	dateStr     string
	description string
	billableSet bool
	billable    bool
}

func newTaskLogCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		appID       int
		hoursFlag   float64
		minutesFlag int
		typeName    string
		typeID      int
		dateFlag    string
		descFlag    string
		billable    bool
		yesFlag     bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "log <ticket-id> <task-id>",
		Short: "Log time worked against a ticket task (--yes required); creates a real time entry",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ticketID, taskID, err := parseTaskIDs(args)
			if err != nil {
				return err
			}
			if !yesFlag {
				return fmt.Errorf("pass --yes to log time")
			}
			if (hoursFlag > 0) == (minutesFlag > 0) {
				if hoursFlag == 0 && minutesFlag == 0 {
					return fmt.Errorf("specify either --hours or --minutes")
				}
				return fmt.Errorf("--hours and --minutes are mutually exclusive")
			}
			if (typeName == "") == (typeID == 0) {
				if typeName == "" && typeID == 0 {
					return fmt.Errorf("specify either --type or --type-id")
				}
				return fmt.Errorf("--type and --type-id are mutually exclusive")
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
			authedUID, err := authedUIDFor(cmd.Context(), auth, profile)
			if err != nil {
				return err
			}
			effectiveAppID := appID
			if effectiveAppID == 0 {
				prof, perr := config.NewProfileStore(paths).GetProfile(profile)
				if perr != nil {
					return perr
				}
				if prof.TicketAppID == 0 {
					return fmt.Errorf("no ticket app configured for profile %q (run `tdx ticket app use <id>` or pass --app <id>)", profile)
				}
				effectiveAppID = prof.TicketAppID
			}
			tsvc := timesvc.New(paths)
			billableSet := cmd.Flags().Changed("billable")
			return runTaskLog(cmd.Context(), cmd.OutOrStdout(), tsvc, taskLogArgs{
				profile: profile, authedUID: authedUID, appID: effectiveAppID, ticketID: ticketID, taskID: taskID,
				hours: hoursFlag, minutes: minutesFlag, typeName: typeName, typeID: typeID,
				dateStr: dateFlag, description: descFlag, billableSet: billableSet, billable: billable,
			})
		},
	}
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id")
	cmd.Flags().Float64Var(&hoursFlag, "hours", 0, "hours")
	cmd.Flags().IntVar(&minutesFlag, "minutes", 0, "minutes")
	cmd.Flags().StringVar(&typeName, "type", "", "time type name")
	cmd.Flags().IntVar(&typeID, "type-id", 0, "time type id")
	cmd.Flags().StringVar(&dateFlag, "date", "", "entry date YYYY-MM-DD (default: today)")
	cmd.Flags().StringVar(&descFlag, "description", "", "description of work performed")
	cmd.Flags().BoolVar(&billable, "billable", false, "force billable (default: type's billable flag)")
	cmd.Flags().BoolVar(&yesFlag, "yes", false, "required")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runTaskLog(ctx context.Context, w io.Writer, svc timesvcAPI, args taskLogArgs) error {
	date := time.Now().In(domain.EasternTZ)
	if args.dateStr != "" {
		d, err := time.ParseInLocation("2006-01-02", args.dateStr, domain.EasternTZ)
		if err != nil {
			return fmt.Errorf("invalid --date: %w", err)
		}
		date = d
	}
	minutes := args.minutes
	if args.hours > 0 {
		minutes = int(math.Round(args.hours * 60))
	}
	if minutes <= 0 {
		return fmt.Errorf("duration must be positive")
	}
	target := domain.Target{
		Kind:   domain.TargetTicketTask,
		AppID:  args.appID,
		ItemID: args.ticketID,
		TaskID: args.taskID,
	}
	chosen, err := resolveTimeType(ctx, svc, args.profile, target, args.typeID, args.typeName)
	if err != nil {
		return err
	}
	billable := chosen.Billable
	if args.billableSet {
		billable = args.billable
	}
	in := domain.EntryInput{
		UserUID:     args.authedUID,
		Date:        date,
		Minutes:     minutes,
		TimeTypeID:  chosen.ID,
		Billable:    billable,
		Target:      target,
		Description: args.description,
	}
	entry, err := svc.AddEntry(ctx, args.profile, in)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(w, "logged %s to ticket #%d task #%d (entry %d, type %q)\n",
		formatDuration(minutes), args.ticketID, args.taskID, entry.ID, chosen.Name)
	return nil
}

// --- shared helpers ---

func parseTaskIDs(args []string) (int, int, error) {
	ticketID, err := strconv.Atoi(args[0])
	if err != nil || ticketID <= 0 {
		return 0, 0, fmt.Errorf("ticket id must be a positive integer, got %q", args[0])
	}
	taskID, err := strconv.Atoi(args[1])
	if err != nil || taskID <= 0 {
		return 0, 0, fmt.Errorf("task id must be a positive integer, got %q", args[1])
	}
	return ticketID, taskID, nil
}
