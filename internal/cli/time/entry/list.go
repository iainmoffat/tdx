package entry

import (
	"fmt"
	"time"

	"github.com/ipm/tdx/internal/config"
	"github.com/ipm/tdx/internal/domain"
	"github.com/ipm/tdx/internal/render"
	"github.com/ipm/tdx/internal/svc/authsvc"
	"github.com/ipm/tdx/internal/svc/timesvc"
	"github.com/spf13/cobra"
)

const defaultListLimit = 100

type entryListJSON struct {
	Schema       string              `json:"schema"`
	Filter       entryListFilterJSON `json:"filter"`
	TotalHours   float64             `json:"totalHours"`
	TotalMinutes int                 `json:"totalMinutes"`
	Entries      []domain.TimeEntry  `json:"entries"`
}

type entryListFilterJSON struct {
	From    string `json:"from"`
	To      string `json:"to"`
	UserUID string `json:"userUID,omitempty"`
	Limit   int    `json:"limit"`
}

func newListCmd() *cobra.Command {
	var (
		profileFlag string
		weekFlag    string
		fromFlag    string
		toFlag      string
		ticketFlag  int
		appFlag     int
		typeFlag    string
		userFlag    string
		limitFlag   int
		jsonFlag    bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List time entries, default this week for the current user",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			auth := authsvc.New(paths)
			tsvc := timesvc.New(paths)

			profileName, err := auth.ResolveProfile(profileFlag)
			if err != nil {
				return err
			}

			// Step 1: Resolve date range. (local, no network)
			var rng domain.DateRange
			switch {
			case weekFlag != "" && (fromFlag != "" || toFlag != ""):
				return fmt.Errorf("--week is mutually exclusive with --from/--to")
			case weekFlag != "":
				day, err := time.ParseInLocation("2006-01-02", weekFlag, domain.EasternTZ)
				if err != nil {
					return fmt.Errorf("invalid --week: %w", err)
				}
				w := domain.WeekRefContaining(day)
				rng = domain.DateRange{From: w.StartDate, To: w.EndDate}
			case fromFlag != "" || toFlag != "":
				if fromFlag == "" || toFlag == "" {
					return fmt.Errorf("--from and --to must be given together")
				}
				from, err := time.ParseInLocation("2006-01-02", fromFlag, domain.EasternTZ)
				if err != nil {
					return fmt.Errorf("invalid --from: %w", err)
				}
				to, err := time.ParseInLocation("2006-01-02", toFlag, domain.EasternTZ)
				if err != nil {
					return fmt.Errorf("invalid --to: %w", err)
				}
				rng = domain.DateRange{From: from, To: to}
			default:
				w := domain.WeekRefContaining(time.Now())
				rng = domain.DateRange{From: w.StartDate, To: w.EndDate}
			}

			// Step 2: Validate ticket/app pair. (local, no network — must come BEFORE WhoAmI)
			if ticketFlag > 0 && appFlag <= 0 {
				return fmt.Errorf("--ticket requires --app (use 'tdx config show' or pass --app <id>)")
			}

			// Step 3: Resolve user: explicit --user, or whoami. (network)
			userUID := userFlag
			if userUID == "" {
				user, err := auth.WhoAmI(cmd.Context(), profileName)
				if err != nil {
					return fmt.Errorf("could not resolve current user for default filter: %w", err)
				}
				userUID = user.UID
			}

			// Step 4: Build filter.
			filter := domain.EntryFilter{
				DateRange: rng,
				UserUID:   userUID,
				Limit:     limitFlag,
			}
			if ticketFlag > 0 {
				filter.Target = &domain.Target{
					Kind:   domain.TargetTicket,
					AppID:  appFlag,
					ItemID: ticketFlag,
				}
			}
			if typeFlag != "" {
				// Resolve type name → ID via a one-time lookup.
				types, err := tsvc.ListTimeTypes(cmd.Context(), profileName)
				if err != nil {
					return fmt.Errorf("lookup time type %q: %w", typeFlag, err)
				}
				match, ok := domain.FindTimeTypeByName(types, typeFlag)
				if !ok {
					return fmt.Errorf("no time type named %q", typeFlag)
				}
				filter.TimeTypeID = match.ID
			}

			entries, err := tsvc.SearchEntries(cmd.Context(), profileName, filter)
			if err != nil {
				return err
			}

			totalMin := 0
			for _, e := range entries {
				totalMin += e.Minutes
			}

			format := render.ResolveFormat(render.Flags{JSON: jsonFlag})
			if format == render.FormatJSON {
				return render.JSON(cmd.OutOrStdout(), entryListJSON{
					Schema: "tdx.v1.entryList",
					Filter: entryListFilterJSON{
						From:    rng.From.Format("2006-01-02"),
						To:      rng.To.Format("2006-01-02"),
						UserUID: userUID,
						Limit:   limitFlag,
					},
					TotalHours:   float64(totalMin) / 60.0,
					TotalMinutes: totalMin,
					Entries:      entries,
				})
			}

			// Human output: flat table.
			headers := []string{"DATE", "HOURS", "TYPE", "TARGET", "DESCRIPTION"}
			rows := make([][]string, 0, len(entries))
			for _, e := range entries {
				rows = append(rows, []string{
					e.Date.Format("2006-01-02"),
					fmt.Sprintf("%.2f", e.Hours()),
					e.TimeType.Name,
					targetLabel(e.Target),
					e.Description,
				})
			}
			summary := []string{"TOTAL", fmt.Sprintf("%.2f", float64(totalMin)/60.0), "", "", ""}
			render.Table(cmd.OutOrStdout(), headers, rows, summary)
			return nil
		},
	}

	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name (defaults to active profile)")
	cmd.Flags().StringVar(&weekFlag, "week", "", "any date inside the target week (YYYY-MM-DD)")
	cmd.Flags().StringVar(&fromFlag, "from", "", "range start (YYYY-MM-DD); requires --to")
	cmd.Flags().StringVar(&toFlag, "to", "", "range end (YYYY-MM-DD); requires --from")
	cmd.Flags().IntVar(&ticketFlag, "ticket", 0, "filter by ticket ID (requires --app)")
	cmd.Flags().IntVar(&appFlag, "app", 0, "application ID (required with --ticket)")
	cmd.Flags().StringVar(&typeFlag, "type", "", "filter by time type name (exact, case-insensitive)")
	cmd.Flags().StringVar(&userFlag, "user", "", "filter by user UID (defaults to whoami)")
	cmd.Flags().IntVar(&limitFlag, "limit", defaultListLimit, "maximum results")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	return cmd
}

func targetLabel(t domain.Target) string {
	if t.DisplayName != "" && t.DisplayRef != "" {
		return t.DisplayRef + " " + t.DisplayName
	}
	if t.DisplayRef != "" {
		return t.DisplayRef
	}
	return t.DisplayName
}
