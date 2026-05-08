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
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

// timesvcAPI is the subset of timesvc the log command needs.
type timesvcAPI interface {
	TimeTypesForTarget(ctx context.Context, profile string, target domain.Target) ([]domain.TimeType, error)
	AddEntry(ctx context.Context, profile string, in domain.EntryInput) (domain.TimeEntry, error)
}

// logRunArgs bundles the runner's parameters for clarity.
type logRunArgs struct {
	profile     string
	authedUID   string
	appID       int
	ticketID    int
	hours       float64
	minutes     int
	typeName    string
	typeID      int
	dateStr     string
	description string
	billableSet bool
	billable    bool
}

func newLogCmd(svc ticketsvcAPI) *cobra.Command {
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
		Use:   "log <id>",
		Short: "Log time worked against a ticket (--yes required)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return fmt.Errorf("ticket id must be a positive integer, got %q", args[0])
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
			// resolve appID via profile fallback if not explicitly set
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
			return runTicketLog(cmd.Context(), cmd.OutOrStdout(), tsvc, logRunArgs{
				profile: profile, authedUID: authedUID, appID: effectiveAppID, ticketID: id,
				hours: hoursFlag, minutes: minutesFlag, typeName: typeName, typeID: typeID,
				dateStr: dateFlag, description: descFlag, billableSet: billableSet, billable: billable,
			})
		},
	}
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id (overrides profile default)")
	cmd.Flags().Float64Var(&hoursFlag, "hours", 0, "hours (mutually exclusive with --minutes)")
	cmd.Flags().IntVar(&minutesFlag, "minutes", 0, "minutes (mutually exclusive with --hours)")
	cmd.Flags().StringVar(&typeName, "type", "", "time type name (case-insensitive; --type or --type-id required)")
	cmd.Flags().IntVar(&typeID, "type-id", 0, "time type id (alternative to --type)")
	cmd.Flags().StringVar(&dateFlag, "date", "", "entry date YYYY-MM-DD (default: today)")
	cmd.Flags().StringVar(&descFlag, "description", "", "description of work performed")
	cmd.Flags().BoolVar(&billable, "billable", false, "force billable (default: type's billable flag)")
	cmd.Flags().BoolVar(&yesFlag, "yes", false, "required to log time")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runTicketLog(ctx context.Context, w io.Writer, svc timesvcAPI, args logRunArgs) error {
	// Resolve date
	date := time.Now().In(domain.EasternTZ)
	if args.dateStr != "" {
		d, err := time.ParseInLocation("2006-01-02", args.dateStr, domain.EasternTZ)
		if err != nil {
			return fmt.Errorf("invalid --date: %w", err)
		}
		date = d
	}
	// Resolve minutes
	minutes := args.minutes
	if args.hours > 0 {
		minutes = int(math.Round(args.hours * 60))
	}
	if minutes <= 0 {
		return fmt.Errorf("duration must be positive")
	}

	target := domain.Target{
		Kind:   domain.TargetTicket,
		AppID:  args.appID,
		ItemID: args.ticketID,
	}

	// Resolve type
	chosen, err := resolveTimeType(ctx, svc, args.profile, target, args.typeID, args.typeName)
	if err != nil {
		return err
	}

	// Determine billable: explicit flag wins; else inherit from chosen type.
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
	_, _ = fmt.Fprintf(w, "logged %s to ticket #%d (entry %d, type %q)\n",
		formatDuration(minutes), args.ticketID, entry.ID, chosen.Name)
	return nil
}

func resolveTimeType(ctx context.Context, svc timesvcAPI, profile string, target domain.Target, typeID int, typeName string) (domain.TimeType, error) {
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
		return domain.TimeType{}, fmt.Errorf("time type id %d is not valid for ticket app %d (run `tdx time type for ticket <ticket-id>` to see allowed types)", typeID, target.AppID)
	}
	target1 := strings.ToLower(strings.TrimSpace(typeName))
	var matches []domain.TimeType
	for _, tt := range types {
		if strings.ToLower(strings.TrimSpace(tt.Name)) == target1 {
			matches = append(matches, tt)
		}
	}
	switch len(matches) {
	case 0:
		names := make([]string, 0, len(types))
		for _, tt := range types {
			names = append(names, tt.Name)
		}
		return domain.TimeType{}, fmt.Errorf("no time type matches %q for ticket — allowed: %s", typeName, strings.Join(names, ", "))
	case 1:
		return matches[0], nil
	default:
		return domain.TimeType{}, fmt.Errorf("multiple time types match %q (use --type-id instead)", typeName)
	}
}

// formatDuration prints minutes as "Nh", "Nm", or "Nh Mm".
func formatDuration(minutes int) string {
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
