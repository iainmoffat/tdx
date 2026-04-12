package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/ipm/tdx/internal/domain"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type weekReportArgs struct {
	Date    string `json:"date" jsonschema:"any date in the target week YYYY-MM-DD"`
	Profile string `json:"profile,omitempty" jsonschema:"profile name"`
}

type lockedDaysArgs struct {
	From    string `json:"from" jsonschema:"start date YYYY-MM-DD"`
	To      string `json:"to" jsonschema:"end date YYYY-MM-DD"`
	Profile string `json:"profile,omitempty" jsonschema:"profile name"`
}

// RegisterWeekTools registers week-view MCP tools.
func RegisterWeekTools(srv *sdkmcp.Server, svcs Services) {
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "get_week_report",
		Description: "Fetch the weekly time report for the week containing a given date.",
	}, weekReportHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "get_locked_days",
		Description: "List locked days within a date range.",
	}, lockedDaysHandler(svcs))
}

func weekReportHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, weekReportArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args weekReportArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)

		date, err := time.ParseInLocation("2006-01-02", args.Date, domain.EasternTZ)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid date: %v", err)), nil, nil
		}

		report, err := svcs.Time.GetWeekReport(ctx, profile, date)
		if err != nil {
			return errorResult(fmt.Sprintf("get week report: %v", err)), nil, nil
		}

		return jsonResult(report)
	}
}

func lockedDaysHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, lockedDaysArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args lockedDaysArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)

		from, err := time.ParseInLocation("2006-01-02", args.From, domain.EasternTZ)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid from date: %v", err)), nil, nil
		}
		to, err := time.ParseInLocation("2006-01-02", args.To, domain.EasternTZ)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid to date: %v", err)), nil, nil
		}

		days, err := svcs.Time.GetLockedDays(ctx, profile, from, to)
		if err != nil {
			return errorResult(fmt.Sprintf("get locked days: %v", err)), nil, nil
		}

		return jsonResult(days)
	}
}
