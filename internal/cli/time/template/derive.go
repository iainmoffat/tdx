package template

import "github.com/spf13/cobra"

func newDeriveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "derive <name>",
		Short: "Create a template from a live week's entries",
		Args:  cobra.ExactArgs(1),
		RunE:  func(cmd *cobra.Command, args []string) error { return nil },
	}
}
