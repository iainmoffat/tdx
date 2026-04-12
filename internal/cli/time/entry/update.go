package entry

import "github.com/spf13/cobra"

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <id>",
		Short: "Update an existing time entry",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil // stub -- implemented in Task 10
		},
	}
}
