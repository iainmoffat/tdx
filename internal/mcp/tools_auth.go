package mcp

import (
	"context"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type whoamiArgs struct {
	Profile string `json:"profile,omitempty" jsonschema:"profile name (defaults to active profile)"`
}

type whoamiResponse struct {
	UID      string `json:"uid"`
	FullName string `json:"fullName"`
	Email    string `json:"email"`
	Profile  string `json:"profile"`
}

// RegisterAuthTools registers authentication-related MCP tools.
func RegisterAuthTools(srv *sdkmcp.Server, svcs Services) {
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "whoami",
		Description: "Returns the authenticated user's identity, profile, and tenant.",
	}, whoamiHandler(svcs))
}

func whoamiHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, whoamiArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args whoamiArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)

		user, err := svcs.Auth.WhoAmI(ctx, profile)
		if err != nil {
			return errorResult(fmt.Sprintf("Not authenticated: %v. Run `tdx auth login` first.", err)), nil, nil
		}

		return jsonResult(whoamiResponse{
			UID:      user.UID,
			FullName: user.FullName,
			Email:    user.Email,
			Profile:  profile,
		})
	}
}
