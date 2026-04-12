package entry

import "github.com/spf13/cobra"

func newAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "Add a new time entry",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil // stub -- implemented in Task 9
		},
	}
}
