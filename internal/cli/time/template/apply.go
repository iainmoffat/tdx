package template

import "github.com/spf13/cobra"

func newApplyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "apply <name>",
		Short: "Apply a template to create time entries for a week",
		Args:  cobra.ExactArgs(1),
		RunE:  func(cmd *cobra.Command, args []string) error { return nil },
	}
}
