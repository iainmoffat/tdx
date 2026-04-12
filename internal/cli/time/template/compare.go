package template

import "github.com/spf13/cobra"

func newCompareCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "compare <name>",
		Short: "Compare a template against a live week's entries",
		Args:  cobra.ExactArgs(1),
		RunE:  func(cmd *cobra.Command, args []string) error { return nil },
	}
}
