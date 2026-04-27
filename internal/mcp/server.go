package mcp

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
	"github.com/iainmoffat/tdx/internal/svc/tmplsvc"
)

// Services bundles the service layer dependencies for MCP tool handlers.
type Services struct {
	Auth     *authsvc.Service
	Time     *timesvc.Service
	Template *tmplsvc.Service
	Drafts   *draftsvc.Service
	Profile  string // default profile name
}

// NewServer creates an MCP server with all tdx tools registered.
func NewServer(version string, svcs Services) *sdkmcp.Server {
	srv := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "tdx",
		Version: version,
	}, nil)

	RegisterAuthTools(srv, svcs)
	RegisterEntryTools(srv, svcs)
	RegisterWeekTools(srv, svcs)
	RegisterTypeTools(srv, svcs)
	RegisterTemplateTools(srv, svcs)
	RegisterApplyTools(srv, svcs)
	RegisterDraftTools(srv, svcs)          // read tools; mutating tools registered by Task 25
	RegisterDraftMutatingTools(srv, svcs) // Task 25

	return srv
}
