package mcp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ipm/tdx/internal/domain"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type listEntriesArgs struct {
	From     string `json:"from" jsonschema:"required,description=start date YYYY-MM-DD"`
	To       string `json:"to" jsonschema:"required,description=end date YYYY-MM-DD"`
	TicketID int    `json:"ticketID,omitempty" jsonschema:"description=filter by ticket ID"`
	AppID    int    `json:"appID,omitempty" jsonschema:"description=app ID (with ticketID)"`
	TypeID   int    `json:"typeID,omitempty" jsonschema:"description=filter by time type ID"`
	UserUID  string `json:"userUID,omitempty" jsonschema:"description=filter by user UID"`
	Limit    int    `json:"limit,omitempty" jsonschema:"description=max results (default 100)"`
	Profile  string `json:"profile,omitempty" jsonschema:"description=profile name"`
}

type getEntryArgs struct {
	ID      int    `json:"id" jsonschema:"required,description=time entry ID"`
	Profile string `json:"profile,omitempty" jsonschema:"description=profile name"`
}

// RegisterEntryTools registers time entry MCP tools.
func RegisterEntryTools(srv *sdkmcp.Server, svcs Services) {
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "list_time_entries",
		Description: "Search time entries within a date range, with optional filters.",
	}, listEntriesHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "get_time_entry",
		Description: "Fetch a single time entry by ID.",
	}, getEntryHandler(svcs))
}

func listEntriesHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, listEntriesArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args listEntriesArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)

		from, err := time.ParseInLocation("2006-01-02", args.From, domain.EasternTZ)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid from date: %v", err)), nil, nil
		}
		to, err := time.ParseInLocation("2006-01-02", args.To, domain.EasternTZ)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid to date: %v", err)), nil, nil
		}

		filter := domain.EntryFilter{
			DateRange:  domain.DateRange{From: from, To: to},
			UserUID:    args.UserUID,
			TimeTypeID: args.TypeID,
			Limit:      args.Limit,
		}
		if filter.Limit == 0 {
			filter.Limit = 100
		}
		if args.TicketID > 0 {
			filter.Target = &domain.Target{
				Kind:   domain.TargetTicket,
				AppID:  args.AppID,
				ItemID: args.TicketID,
			}
		}

		entries, err := svcs.Time.SearchEntries(ctx, profile, filter)
		if err != nil {
			return errorResult(fmt.Sprintf("search entries: %v", err)), nil, nil
		}

		return jsonResult(entries)
	}
}

func getEntryHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, getEntryArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args getEntryArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)

		entry, err := svcs.Time.GetEntry(ctx, profile, args.ID)
		if err != nil {
			if errors.Is(err, domain.ErrEntryNotFound) {
				return errorResult(fmt.Sprintf("time entry %d not found", args.ID)), nil, nil
			}
			return errorResult(fmt.Sprintf("get entry: %v", err)), nil, nil
		}

		return jsonResult(entry)
	}
}
