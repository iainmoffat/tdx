package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ipm/tdx/internal/domain"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type listTemplatesArgs struct {
	Profile string `json:"profile,omitempty" jsonschema:"profile name"`
}

type getTemplateArgs struct {
	Name    string `json:"name" jsonschema:"template name"`
	Profile string `json:"profile,omitempty" jsonschema:"profile name"`
}

type createTemplateArgs struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Rows        string `json:"rows" jsonschema:"JSON array of template rows"`
	Confirm     bool   `json:"confirm" jsonschema:"must be true to execute"`
}

type updateTemplateArgs struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Confirm     bool   `json:"confirm" jsonschema:"must be true to execute"`
}

type deleteTemplateArgs struct {
	Name    string `json:"name"`
	Confirm bool   `json:"confirm" jsonschema:"must be true to execute"`
}

type deriveTemplateArgs struct {
	Name        string `json:"name"`
	FromWeek    string `json:"fromWeek" jsonschema:"source week YYYY-MM-DD"`
	Description string `json:"description,omitempty"`
	Confirm     bool   `json:"confirm" jsonschema:"must be true to execute"`
	Profile     string `json:"profile,omitempty"`
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

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "create_time_template",
		Description: "Create a new time template from a JSON row specification. Requires confirm=true.",
	}, createTemplateHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "update_time_template",
		Description: "Update a template's description. Requires confirm=true.",
	}, updateTemplateHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "delete_time_template",
		Description: "Delete a template by name. Requires confirm=true.",
	}, deleteTemplateHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "derive_time_template",
		Description: "Derive a new template from an existing week's time entries. Requires confirm=true.",
	}, deriveTemplateHandler(svcs))
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

func createTemplateHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, createTemplateArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args createTemplateArgs) (*sdkmcp.CallToolResult, any, error) {
		if result, ok := confirmGate(args.Confirm, "Set confirm: true to create the template."); !ok {
			return result, nil, nil
		}

		var rows []domain.TemplateRow
		if err := json.Unmarshal([]byte(args.Rows), &rows); err != nil {
			return errorResult(fmt.Sprintf("invalid rows JSON: %v", err)), nil, nil
		}

		now := time.Now().UTC()
		tmpl := domain.Template{
			SchemaVersion: 1,
			Name:          args.Name,
			Description:   args.Description,
			CreatedAt:     now,
			ModifiedAt:    now,
			Rows:          rows,
		}

		if err := tmpl.Validate(); err != nil {
			return errorResult(fmt.Sprintf("invalid template: %v", err)), nil, nil
		}

		if err := svcs.Template.Store().Save(tmpl); err != nil {
			return errorResult(fmt.Sprintf("save template: %v", err)), nil, nil
		}

		return jsonResult(tmpl)
	}
}

func updateTemplateHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, updateTemplateArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args updateTemplateArgs) (*sdkmcp.CallToolResult, any, error) {
		if result, ok := confirmGate(args.Confirm, "Set confirm: true to update the template."); !ok {
			return result, nil, nil
		}

		tmpl, err := svcs.Template.Store().Load(args.Name)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return errorResult(fmt.Sprintf("template %q not found", args.Name)), nil, nil
			}
			return errorResult(fmt.Sprintf("load template: %v", err)), nil, nil
		}

		tmpl.Description = args.Description
		tmpl.ModifiedAt = time.Now().UTC()

		if err := svcs.Template.Store().Save(tmpl); err != nil {
			return errorResult(fmt.Sprintf("save template: %v", err)), nil, nil
		}

		return jsonResult(tmpl)
	}
}

func deleteTemplateHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, deleteTemplateArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args deleteTemplateArgs) (*sdkmcp.CallToolResult, any, error) {
		if result, ok := confirmGate(args.Confirm, "Set confirm: true to delete the template."); !ok {
			return result, nil, nil
		}

		if err := svcs.Template.Store().Delete(args.Name); err != nil {
			if strings.Contains(err.Error(), "not found") {
				return errorResult(fmt.Sprintf("template %q not found", args.Name)), nil, nil
			}
			return errorResult(fmt.Sprintf("delete template: %v", err)), nil, nil
		}

		return textResult(fmt.Sprintf("Template %q deleted.", args.Name))
	}
}

func deriveTemplateHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, deriveTemplateArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args deriveTemplateArgs) (*sdkmcp.CallToolResult, any, error) {
		if result, ok := confirmGate(args.Confirm, "Set confirm: true to derive the template from the week."); !ok {
			return result, nil, nil
		}

		profile := resolveProfile(svcs, args.Profile)

		weekDate, err := time.ParseInLocation("2006-01-02", args.FromWeek, domain.EasternTZ)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid week date: %v", err)), nil, nil
		}

		tmpl, err := svcs.Template.Derive(ctx, profile, args.Name, weekDate)
		if err != nil {
			return errorResult(fmt.Sprintf("derive template: %v", err)), nil, nil
		}

		if args.Description != "" {
			tmpl.Description = args.Description
			if saveErr := svcs.Template.Store().Save(tmpl); saveErr != nil {
				return errorResult(fmt.Sprintf("save description: %v", saveErr)), nil, nil
			}
		}

		return jsonResult(tmpl)
	}
}
