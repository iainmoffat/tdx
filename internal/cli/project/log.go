package project

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
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

// timesvcLogAPI is the subset of timesvc the log command needs.
type timesvcLogAPI interface {
	TimeTypesForTarget(ctx context.Context, profile string, target domain.Target) ([]domain.TimeType, error)
	AddEntry(ctx context.Context, profile string, in domain.EntryInput) (domain.TimeEntry, error)
}

func newLogCmd(svc projectsvcAPI) *cobra.Command {
	var (
		planIDFlag  int
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
		Use:   "log <project-id> <task-id>",
		Short: "Log time worked against a project task (--yes required)",
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
				return fmt.Errorf("--plan is required")
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
			tsvc := timesvc.New(paths)
			billableSet := cmd.Flags().Changed("billable")
			return runProjectLog(cmd.Context(), cmd.OutOrStdout(), tsvc, profile, authedUID,
				projectID, planIDFlag, taskID, hoursFlag, minutesFlag, typeName, typeID,
				dateFlag, descFlag, billableSet, billable)
		},
	}
	cmd.Flags().IntVar(&planIDFlag, "plan", 0, "plan ID (required)")
	cmd.Flags().Float64Var(&hoursFlag, "hours", 0, "hours (mutually exclusive with --minutes)")
	cmd.Flags().IntVar(&minutesFlag, "minutes", 0, "minutes (mutually exclusive with --hours)")
	cmd.Flags().StringVar(&typeName, "type", "", "time type name (case-insensitive; --type or --type-id required)")
	cmd.Flags().IntVar(&typeID, "type-id", 0, "time type id (alternative to --type)")
	cmd.Flags().StringVar(&dateFlag, "date", "", "entry date YYYY-MM-DD (default: today)")
	cmd.Flags().StringVar(&descFlag, "description", "", "description of work performed")
	cmd.Flags().BoolVar(&billable, "billable", false, "force billable (default: type's billable flag)")
	cmd.Flags().BoolVar(&yesFlag, "yes", false, "required to log time")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	_ = svc // projectsvcAPI not needed for log; kept for interface consistency
	return cmd
}

func runProjectLog(ctx context.Context, w io.Writer, svc timesvcLogAPI, profile, authedUID string,
	projectID, planID, taskID int, hoursF float64, minutesI int,
	typeName string, typeID int, dateStr, description string,
	billableSet, billable bool) error {

	// Validate mutual exclusions (also enforced in cobra RunE; checked here for testability).
	if (hoursF > 0) && (minutesI > 0) {
		return fmt.Errorf("--hours and --minutes are mutually exclusive")
	}
	if typeName != "" && typeID != 0 {
		return fmt.Errorf("--type and --type-id are mutually exclusive")
	}

	// Resolve date.
	date := time.Now().In(domain.EasternTZ)
	if dateStr != "" {
		d, err := time.ParseInLocation("2006-01-02", dateStr, domain.EasternTZ)
		if err != nil {
			return fmt.Errorf("invalid --date: %w", err)
		}
		date = d
	}

	// Resolve minutes.
	minutes := minutesI
	if hoursF > 0 {
		minutes = int(math.Round(hoursF * 60))
	}
	if minutes <= 0 {
		return fmt.Errorf("duration must be positive")
	}

	// Target convention for project tasks (matches timesvc/encode.go and
	// componentPathFor — verified against UFL): ItemID carries the plan ID,
	// TaskID carries the task ID, ProjectID carries the project ID.
	target := domain.Target{
		Kind:      domain.TargetProjectTask,
		ItemID:    planID,
		TaskID:    taskID,
		ProjectID: projectID,
	}

	// Resolve time type.
	chosen, err := resolveProjectTimeType(ctx, svc, profile, target, typeID, typeName)
	if err != nil {
		return err
	}

	// Determine billable.
	effectiveBillable := chosen.Billable
	if billableSet {
		effectiveBillable = billable
	}

	in := domain.EntryInput{
		UserUID:     authedUID,
		Date:        date,
		Minutes:     minutes,
		TimeTypeID:  chosen.ID,
		Billable:    effectiveBillable,
		Target:      target,
		Description: description,
	}
	entry, err := svc.AddEntry(ctx, profile, in)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(w, "logged %s to task #%d on project %d (entry %d, type %q)\n",
		formatLogDuration(minutes), taskID, projectID, entry.ID, chosen.Name)
	return nil
}

func resolveProjectTimeType(ctx context.Context, svc timesvcLogAPI, profile string, target domain.Target, typeID int, typeName string) (domain.TimeType, error) {
	types, err := svc.TimeTypesForTarget(ctx, profile, target)
	if err != nil {
		return domain.TimeType{}, fmt.Errorf("look up time types: %w", err)
	}
	if typeID > 0 {
		for _, tt := range types {
			if tt.ID == typeID {
				return tt, nil
			}
		}
		return domain.TimeType{}, fmt.Errorf("time type id %d is not valid for this project task (run `tdx time type for projectTask <task-id>` to see allowed types)", typeID)
	}
	needle := strings.ToLower(strings.TrimSpace(typeName))
	var matches []domain.TimeType
	for _, tt := range types {
		if strings.ToLower(strings.TrimSpace(tt.Name)) == needle {
			matches = append(matches, tt)
		}
	}
	switch len(matches) {
	case 0:
		names := make([]string, 0, len(types))
		for _, tt := range types {
			names = append(names, tt.Name)
		}
		return domain.TimeType{}, fmt.Errorf("no time type matches %q — allowed: %s", typeName, strings.Join(names, ", "))
	case 1:
		return matches[0], nil
	default:
		return domain.TimeType{}, fmt.Errorf("multiple time types match %q (use --type-id instead)", typeName)
	}
}

// formatLogDuration prints minutes as "Nh", "Nm", or "Nh Mm".
func formatLogDuration(minutes int) string {
	h := minutes / 60
	m := minutes % 60
	switch {
	case h == 0:
		return fmt.Sprintf("%dm", m)
	case m == 0:
		return fmt.Sprintf("%dh", h)
	default:
		return fmt.Sprintf("%dh %dm", h, m)
	}
}
