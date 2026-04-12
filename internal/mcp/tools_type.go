package mcp

import (
	"context"
	"fmt"

	"github.com/ipm/tdx/internal/domain"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type listTypesArgs struct {
	Profile string `json:"profile,omitempty" jsonschema:"profile name"`
}

type typesForTargetArgs struct {
	Kind    string `json:"kind" jsonschema:"target kind (ticket/project/workspace/etc)"`
	ItemID  int    `json:"itemID" jsonschema:"work item ID"`
	AppID   int    `json:"appID,omitempty" jsonschema:"app ID (required for tickets)"`
	TaskID  int    `json:"taskID,omitempty" jsonschema:"task ID"`
	Profile string `json:"profile,omitempty" jsonschema:"profile name"`
}

// RegisterTypeTools registers time-type lookup MCP tools.
func RegisterTypeTools(srv *sdkmcp.Server, svcs Services) {
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "list_time_types",
		Description: "List all available time types.",
	}, listTypesHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "get_time_types_for_target",
		Description: "List valid time types for a specific work item (ticket, project, etc).",
	}, typesForTargetHandler(svcs))
}

func listTypesHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, listTypesArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args listTypesArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)

		types, err := svcs.Time.ListTimeTypes(ctx, profile)
		if err != nil {
			return errorResult(fmt.Sprintf("list time types: %v", err)), nil, nil
		}

		return jsonResult(types)
	}
}

func typesForTargetHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, typesForTargetArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args typesForTargetArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)

		target := domain.Target{
			Kind:   domain.TargetKind(args.Kind),
			AppID:  args.AppID,
			ItemID: args.ItemID,
			TaskID: args.TaskID,
		}

		types, err := svcs.Time.TimeTypesForTarget(ctx, profile, target)
		if err != nil {
			return errorResult(fmt.Sprintf("time types for target: %v", err)), nil, nil
		}

		return jsonResult(types)
	}
}
