package mcp

import (
	"context"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/iainmoffat/tdx/internal/cli/time/report"
)

type getTimeStatusReportArgs struct {
	Profile       string   `json:"profile,omitempty"`
	Week          string   `json:"week,omitempty"`
	From          string   `json:"from,omitempty"`
	To            string   `json:"to,omitempty"`
	UserUIDs      []string `json:"userUIDs,omitempty"`
	Managers      []string `json:"managers,omitempty"`
	Accounts      []string `json:"accounts,omitempty"`
	ResourcePools []string `json:"resourcePools,omitempty"`
	All           bool     `json:"all,omitempty"`
	IncludeZero   bool     `json:"includeZero,omitempty"`
	Incomplete    bool     `json:"incomplete,omitempty"`
	Threshold     float64  `json:"threshold,omitempty"`
	Limit         int      `json:"limit,omitempty"`
}

// RegisterReportTools registers the read-only Time Status Report tool.
func RegisterReportTools(srv *sdkmcp.Server, svcs Services) {
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name: "get_time_status_report",
		Description: `Generate a Time Status Report (per user, per week).

Selectors (exactly one): userUIDs, manager, account, resourcePool, all.
Date: week (single) or from/to (range).
Filters: incomplete=true keeps only rows below threshold. When threshold is omitted, each user's weekly threshold is computed as their TD WorkableHours × 5 (TD stores daily hours, e.g. 8.0 FT = 40h/week); falls back to 40h when WorkableHours is 0.
Permission-denied rows are dropped under the filter.
Read-only — no confirm required.`,
	}, getTimeStatusReportHandler(svcs))
}

func getTimeStatusReportHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, getTimeStatusReportArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args getTimeStatusReportArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)

		out, err := report.RunForMCP(ctx, report.MCPInputs{
			Profile:       profile,
			Week:          args.Week,
			From:          args.From,
			To:            args.To,
			Users:         args.UserUIDs,
			Managers:      args.Managers,
			Accounts:      args.Accounts,
			ResourcePools: args.ResourcePools,
			All:           args.All,
			IncludeZero:   args.IncludeZero,
			Incomplete:    args.Incomplete,
			Threshold:     args.Threshold,
			ThresholdSet:  args.Threshold > 0,
			Limit:         args.Limit,
			TimeSvc:       svcs.Time,
			PeopleSvc:     svcs.People,
			AuthSvc:       svcs.Auth,
		})
		if err != nil {
			return errorResult(fmt.Sprintf("time-status-report: %v", err)), nil, nil
		}
		return jsonResult(out)
	}
}
