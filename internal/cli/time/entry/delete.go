package entry

import "github.com/spf13/cobra"

func newDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id> [<id>...]",
		Short: "Delete one or more time entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil // stub -- implemented in Task 11
		},
	}
}
