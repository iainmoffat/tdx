package mcp

import "github.com/spf13/cobra"

// NewCmd returns the `tdx mcp` command group.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server for agent integration",
	}
	cmd.AddCommand(newServeCmd())
	return cmd
}
