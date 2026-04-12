package entry

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/ipm/tdx/internal/config"
	"github.com/ipm/tdx/internal/domain"
	"github.com/ipm/tdx/internal/render"
	"github.com/ipm/tdx/internal/svc/authsvc"
	"github.com/ipm/tdx/internal/svc/timesvc"
	"github.com/spf13/cobra"
)

type entryShowJSON struct {
	Schema string           `json:"schema"`
	Entry  domain.TimeEntry `json:"entry"`
}

func newShowCmd() *cobra.Command {
	var (
		profileFlag string
		jsonFlag    bool
	)

	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show a single time entry by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid entry id %q: %w", args[0], err)
			}
			if id <= 0 {
				return fmt.Errorf("entry id must be a positive integer, got %d", id)
			}

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

			entry, err := tsvc.GetEntry(cmd.Context(), profileName, id)
			if err != nil {
				if errors.Is(err, domain.ErrEntryNotFound) {
					return fmt.Errorf("entry %d not found", id)
				}
				return err
			}

			format := render.ResolveFormat(render.Flags{JSON: jsonFlag})
			if format == render.FormatJSON {
				return render.JSON(cmd.OutOrStdout(), entryShowJSON{
					Schema: "tdx.v1.entry",
					Entry:  entry,
				})
			}

			printEntry(cmd.OutOrStdout(), entry)
			return nil
		},
	}

	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name (defaults to active profile)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	return cmd
}
