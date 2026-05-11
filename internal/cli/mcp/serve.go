package mcp

import (
	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	mcpsrv "github.com/iainmoffat/tdx/internal/mcp"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
	"github.com/iainmoffat/tdx/internal/svc/peoplesvc"
	"github.com/iainmoffat/tdx/internal/svc/projectsvc"
	"github.com/iainmoffat/tdx/internal/svc/ticketsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
	"github.com/iainmoffat/tdx/internal/svc/tmplsvc"

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
			drafts := draftsvc.NewService(paths, tsvc)
			people := peoplesvc.New(paths)
			tickets := ticketsvc.New(paths)
			projects := projectsvc.New(paths)

			profileName, err := auth.ResolveProfile(profileFlag)
			if err != nil {
				return err
			}

			srv := mcpsrv.NewServer("0.1.0", mcpsrv.Services{
				Auth:     auth,
				Time:     tsvc,
				Template: tmsvc,
				Drafts:   drafts,
				People:   people,
				Tickets:  tickets,
				Projects: projects,
				Profile:  profileName,
			})

			return srv.Run(cmd.Context(), &sdkmcp.StdioTransport{})
		},
	}

	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name (defaults to active profile)")
	return cmd
}
