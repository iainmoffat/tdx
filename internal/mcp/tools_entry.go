package mcp

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/ipm/tdx/internal/domain"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type listEntriesArgs struct {
	From     string `json:"from" jsonschema:"start date YYYY-MM-DD"`
	To       string `json:"to" jsonschema:"end date YYYY-MM-DD"`
	TicketID int    `json:"ticketID,omitempty" jsonschema:"filter by ticket ID"`
	AppID    int    `json:"appID,omitempty" jsonschema:"app ID (with ticketID)"`
	TypeID   int    `json:"typeID,omitempty" jsonschema:"filter by time type ID"`
	UserUID  string `json:"userUID,omitempty" jsonschema:"filter by user UID"`
	Limit    int    `json:"limit,omitempty" jsonschema:"max results (default 100)"`
	Profile  string `json:"profile,omitempty" jsonschema:"profile name"`
}

type getEntryArgs struct {
	ID      int    `json:"id" jsonschema:"time entry ID"`
	Profile string `json:"profile,omitempty" jsonschema:"profile name"`
}

type createEntryArgs struct {
	Date        string  `json:"date" jsonschema:"entry date YYYY-MM-DD"`
	Hours       float64 `json:"hours,omitempty" jsonschema:"duration in hours"`
	Minutes     int     `json:"minutes,omitempty" jsonschema:"duration in minutes"`
	TypeID      int     `json:"typeID" jsonschema:"time type ID"`
	Kind        string  `json:"kind" jsonschema:"target kind (ticket/project/workspace)"`
	ItemID      int     `json:"itemID" jsonschema:"work item ID"`
	AppID       int     `json:"appID,omitempty"`
	TaskID      int     `json:"taskID,omitempty"`
	Description string  `json:"description,omitempty"`
	Billable    bool    `json:"billable,omitempty"`
	Confirm     bool    `json:"confirm" jsonschema:"must be true to execute"`
	Profile     string  `json:"profile,omitempty"`
}

type updateEntryArgs struct {
	ID          int     `json:"id"`
	Date        string  `json:"date,omitempty"`
	Hours       float64 `json:"hours,omitempty"`
	Minutes     int     `json:"minutes,omitempty"`
	TypeID      int     `json:"typeID,omitempty"`
	Description string  `json:"description,omitempty"`
	Confirm     bool    `json:"confirm" jsonschema:"must be true to execute"`
	Profile     string  `json:"profile,omitempty"`
}

type deleteEntryArgs struct {
	ID      int    `json:"id"`
	Confirm bool   `json:"confirm" jsonschema:"must be true to execute"`
	Profile string `json:"profile,omitempty"`
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

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "create_time_entry",
		Description: "Create a new time entry. Requires confirm=true.",
	}, createEntryHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "update_time_entry",
		Description: "Update an existing time entry. Requires confirm=true.",
	}, updateEntryHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "delete_time_entry",
		Description: "Delete a time entry by ID. Requires confirm=true.",
	}, deleteEntryHandler(svcs))
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

// resolveMinutes converts hours/minutes args to total minutes.
// If both are provided, hours wins. If neither, returns an error.
func resolveMinutes(hours float64, minutes int) (int, error) {
	if hours > 0 {
		raw := hours * 60
		rounded := math.Round(raw)
		return int(rounded), nil
	}
	if minutes > 0 {
		return minutes, nil
	}
	return 0, fmt.Errorf("either hours or minutes must be provided")
}

func createEntryHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, createEntryArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args createEntryArgs) (*sdkmcp.CallToolResult, any, error) {
		if result, ok := confirmGate(args.Confirm, "Set confirm: true to create the time entry."); !ok {
			return result, nil, nil
		}

		profile := resolveProfile(svcs, args.Profile)

		date, err := time.ParseInLocation("2006-01-02", args.Date, domain.EasternTZ)
		if err != nil {
			return errorResult(fmt.Sprintf("invalid date: %v", err)), nil, nil
		}

		mins, err := resolveMinutes(args.Hours, args.Minutes)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}

		user, err := svcs.Auth.WhoAmI(ctx, profile)
		if err != nil {
			return errorResult(fmt.Sprintf("auth: %v", err)), nil, nil
		}

		target := domain.Target{
			Kind:   domain.TargetKind(args.Kind),
			ItemID: args.ItemID,
			AppID:  args.AppID,
			TaskID: args.TaskID,
		}

		input := domain.EntryInput{
			UserUID:     user.UID,
			Date:        date,
			Minutes:     mins,
			TimeTypeID:  args.TypeID,
			Billable:    args.Billable,
			Target:      target,
			Description: args.Description,
		}

		entry, err := svcs.Time.AddEntry(ctx, profile, input)
		if err != nil {
			return errorResult(fmt.Sprintf("create entry: %v", err)), nil, nil
		}

		return jsonResult(entry)
	}
}

func updateEntryHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, updateEntryArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args updateEntryArgs) (*sdkmcp.CallToolResult, any, error) {
		if result, ok := confirmGate(args.Confirm, "Set confirm: true to update the time entry."); !ok {
			return result, nil, nil
		}

		profile := resolveProfile(svcs, args.Profile)

		var update domain.EntryUpdate

		if args.Date != "" {
			d, err := time.ParseInLocation("2006-01-02", args.Date, domain.EasternTZ)
			if err != nil {
				return errorResult(fmt.Sprintf("invalid date: %v", err)), nil, nil
			}
			update.Date = &d
		}

		if args.Hours > 0 || args.Minutes > 0 {
			mins, err := resolveMinutes(args.Hours, args.Minutes)
			if err != nil {
				return errorResult(err.Error()), nil, nil
			}
			update.Minutes = &mins
		}

		if args.TypeID > 0 {
			update.TimeTypeID = &args.TypeID
		}

		if args.Description != "" {
			update.Description = &args.Description
		}

		entry, err := svcs.Time.UpdateEntry(ctx, profile, args.ID, update)
		if err != nil {
			return errorResult(fmt.Sprintf("update entry: %v", err)), nil, nil
		}

		return jsonResult(entry)
	}
}

func deleteEntryHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, deleteEntryArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, args deleteEntryArgs) (*sdkmcp.CallToolResult, any, error) {
		if result, ok := confirmGate(args.Confirm, "Set confirm: true to delete the time entry."); !ok {
			return result, nil, nil
		}

		profile := resolveProfile(svcs, args.Profile)

		err := svcs.Time.DeleteEntry(ctx, profile, args.ID)
		if err != nil {
			return errorResult(fmt.Sprintf("delete entry: %v", err)), nil, nil
		}

		return textResult(fmt.Sprintf("Time entry %d deleted.", args.ID))
	}
}
