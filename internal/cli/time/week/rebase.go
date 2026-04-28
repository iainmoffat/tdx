package week

import "github.com/spf13/cobra"

// newRebaseCmd is an alias of `tdx time week refresh`. Hardcore git users
// reach for `rebase` reflexively; we accept either name.
func newRebaseCmd() *cobra.Command {
	var f refreshFlags
	cmd := &cobra.Command{
		Use:   "rebase [date[/name]]",
		Short: "Alias of `refresh` (defaults to the current week)",
		Long:  `rebase is identical to refresh — same flags, same behavior. See ` + "`tdx time week refresh --help`" + ` for the full description.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := ""
			if len(args) > 0 {
				ref = args[0]
			}
			return runRefresh(cmd, f, ref)
		},
	}
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
	cmd.Flags().StringVar(&f.strategy, "strategy", "abort", "abort | ours | theirs")
	cmd.Flags().BoolVar(&f.json, "json", false, "JSON output")
	return cmd
}
