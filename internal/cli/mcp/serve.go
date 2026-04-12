package mcp

import (
	"github.com/spf13/cobra"

	"github.com/ipm/tdx/internal/config"
	mcpsrv "github.com/ipm/tdx/internal/mcp"
	"github.com/ipm/tdx/internal/svc/authsvc"
	"github.com/ipm/tdx/internal/svc/timesvc"
	"github.com/ipm/tdx/internal/svc/tmplsvc"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func newServeCmd() *cobra.Command {
	var profileFlag string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the MCP server over stdio",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}

			auth := authsvc.New(paths)
			tsvc := timesvc.New(paths)
			tmsvc := tmplsvc.New(paths, tsvc)

			profileName, err := auth.ResolveProfile(profileFlag)
			if err != nil {
				return err
			}

			srv := mcpsrv.NewServer("0.1.0", mcpsrv.Services{
				Auth:     auth,
				Time:     tsvc,
				Template: tmsvc,
				Profile:  profileName,
			})

			return srv.Run(cmd.Context(), &sdkmcp.StdioTransport{})
		},
	}

	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name (defaults to active profile)")
	return cmd
}
