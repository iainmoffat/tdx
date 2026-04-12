package mcp

import (
	"context"
	"fmt"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type listTemplatesArgs struct {
	Profile string `json:"profile,omitempty" jsonschema:"description=profile name"`
}

type getTemplateArgs struct {
	Name    string `json:"name" jsonschema:"required,description=template name"`
	Profile string `json:"profile,omitempty" jsonschema:"description=profile name"`
}

// RegisterTemplateTools registers template MCP tools.
func RegisterTemplateTools(srv *sdkmcp.Server, svcs Services) {
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "list_time_templates",
		Description: "List all saved time templates.",
	}, listTemplatesHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "get_time_template",
		Description: "Load a time template by name.",
	}, getTemplateHandler(svcs))
}

func listTemplatesHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, listTemplatesArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args listTemplatesArgs) (*sdkmcp.CallToolResult, any, error) {
		templates, err := svcs.Template.Store().List()
		if err != nil {
			return errorResult(fmt.Sprintf("list templates: %v", err)), nil, nil
		}

		return jsonResult(templates)
	}
}

func getTemplateHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, getTemplateArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args getTemplateArgs) (*sdkmcp.CallToolResult, any, error) {
		tmpl, err := svcs.Template.Store().Load(args.Name)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return errorResult(fmt.Sprintf("template %q not found", args.Name)), nil, nil
			}
			return errorResult(fmt.Sprintf("load template: %v", err)), nil, nil
		}

		return jsonResult(tmpl)
	}
}
