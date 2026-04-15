package template

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
	"github.com/iainmoffat/tdx/internal/svc/tmplsvc"
)

func newCompareCmd() *cobra.Command {
	var (
		weekFlag    string
		modeFlag    string
		daysFlag    string
		overrides   []string
		roundFlag   bool
		jsonFlag    bool
		profileFlag string
	)

	cmd := &cobra.Command{
		Use:   "compare <name>",
		Short: "Compare a template against a live week's entries",
		Long: `Compare shows what apply would do without making any changes.

It runs the reconciliation engine and prints the annotated grid preview,
exactly like 'apply --dry-run', but is an explicit read-only command with
no --yes or --dry-run flags.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// --week is required.
			if weekFlag == "" {
				return fmt.Errorf("--week is required")
			}
			weekDate, err := time.ParseInLocation("2006-01-02", weekFlag, domain.EasternTZ)
			if err != nil {
				return fmt.Errorf("invalid --week: %w", err)
			}

			// Parse --mode.
			mode, err := domain.ParseApplyMode(modeFlag)
			if err != nil {
				return err
			}

			// Parse --days.
			var daysFilter []time.Weekday
			if daysFlag != "" {
				daysFilter, err = parseDays(daysFlag)
				if err != nil {
					return fmt.Errorf("invalid --days: %w", err)
				}
			}

			// Parse --override flags.
			var parsedOverrides []tmplsvc.Override
			for _, o := range overrides {
				ov, err := parseOverride(o)
				if err != nil {
					return fmt.Errorf("invalid --override %q: %w", o, err)
				}
				parsedOverrides = append(parsedOverrides, ov)
			}

			// Wire up services.
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			auth := authsvc.New(paths)
			tsvc := timesvc.New(paths)
			svc := tmplsvc.New(paths, tsvc)

			profileName, err := auth.ResolveProfile(profileFlag)
			if err != nil {
				return err
			}

			// Load template.
			tmpl, err := svc.Store().Load(name)
			if err != nil {
				return err
			}

			// Resolve week.
			weekRef := domain.WeekRefContaining(weekDate)

			// Get user identity.
			user, err := auth.WhoAmI(cmd.Context(), profileName)
			if err != nil {
				return err
			}

			// Build reconcile input.
			input := tmplsvc.ReconcileInput{
				Template:   tmpl,
				WeekRef:    weekRef,
				Mode:       mode,
				DaysFilter: daysFilter,
				Overrides:  parsedOverrides,
				Round:      roundFlag,
				UserUID:    user.UID,
			}

			// Run reconciliation preview (read-only — never writes).
			diff, err := svc.Reconcile(cmd.Context(), profileName, input)
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()
			creates, updates, skips := diff.CountByKind()
			blockers := len(diff.Blockers)

			format := render.ResolveFormat(render.Flags{JSON: jsonFlag})

			if format == render.FormatJSON {
				return render.JSON(w, struct {
					Schema      string           `json:"schema"`
					DiffHash    string           `json:"diffHash"`
					Creates     int              `json:"creates"`
					Updates     int              `json:"updates"`
					Skips       int              `json:"skips"`
					Blockers    int              `json:"blockers"`
					Actions     []domain.Action  `json:"actions"`
					BlockerList []domain.Blocker `json:"blockerList,omitempty"`
				}{
					Schema:      "tdx.v1.templateComparePreview",
					DiffHash:    diff.DiffHash,
					Creates:     creates,
					Updates:     updates,
					Skips:       skips,
					Blockers:    blockers,
					Actions:     diff.Actions,
					BlockerList: diff.Blockers,
				})
			}

			// Render annotated grid preview.
			render.Grid(w, diffToGridData(tmpl, diff))

			// Summary line.
			_, _ = fmt.Fprintf(w, "\n%d to create, %d to update, %d skipped, %d blocked\n",
				creates, updates, skips, blockers)

			return nil
		},
	}

	cmd.Flags().StringVar(&weekFlag, "week", "", "target week date (YYYY-MM-DD, required)")
	cmd.Flags().StringVar(&modeFlag, "mode", "add", "apply mode: add, replace-matching, replace-mine")
	cmd.Flags().StringVar(&daysFlag, "days", "", "filter days: range (mon-fri) or list (mon,wed,fri)")
	cmd.Flags().StringArrayVar(&overrides, "override", nil, "override row hours: row-id:day=hours (repeatable)")
	cmd.Flags().BoolVar(&roundFlag, "round", false, "round fractional minutes to nearest whole minute")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}
