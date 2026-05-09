package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/ticketsvc"
)

// --- Argument types ---

type addTicketCommentArgs struct {
	Profile   string   `json:"profile,omitempty"`
	AppID     int      `json:"appID,omitempty"`
	ID        int      `json:"id"`
	Comments  string   `json:"comments"`
	IsPrivate bool     `json:"isPrivate,omitempty"`
	Notify    []string `json:"notify,omitempty"`
	Confirm   bool     `json:"confirm" jsonschema:"set true to actually post"`
}

type updateTicketStatusArgs struct {
	Profile    string `json:"profile,omitempty"`
	AppID      int    `json:"appID,omitempty"`
	ID         int    `json:"id"`
	StatusID   int    `json:"statusID,omitempty"`
	StatusName string `json:"statusName,omitempty"`
	Comment    string `json:"comment,omitempty"`
	Confirm    bool   `json:"confirm" jsonschema:"set true to actually update"`
}

type updateTicketAssigneeArgs struct {
	Profile        string `json:"profile,omitempty"`
	AppID          int    `json:"appID,omitempty"`
	ID             int    `json:"id"`
	ResponsibleUID string `json:"responsibleUID"`
	Comment        string `json:"comment,omitempty"`
	Confirm        bool   `json:"confirm" jsonschema:"set true to actually update"`
}

type logTicketTimeArgs struct {
	Profile     string  `json:"profile,omitempty"`
	AppID       int     `json:"appID,omitempty"`
	ID          int     `json:"id"`
	Hours       float64 `json:"hours,omitempty"`
	Minutes     int     `json:"minutes,omitempty"`
	TypeID      int     `json:"typeID,omitempty"`
	TypeName    string  `json:"typeName,omitempty"`
	Date        string  `json:"date,omitempty"` // YYYY-MM-DD; default today
	Description string  `json:"description,omitempty"`
	Billable    *bool   `json:"billable,omitempty"` // pointer so we can detect "not set"
	Confirm     bool    `json:"confirm" jsonschema:"set true to actually log"`
}

type updateTicketTaskArgs struct {
	Profile         string   `json:"profile,omitempty"`
	AppID           int      `json:"appID,omitempty"`
	TicketID        int      `json:"ticketID"`
	TaskID          int      `json:"taskID"`
	Comment         string   `json:"comment,omitempty"`
	PercentComplete *int     `json:"percentComplete,omitempty"`
	HoursWorked     float64  `json:"hoursWorked,omitempty"`
	IsPrivate       bool     `json:"isPrivate,omitempty"`
	Notify          []string `json:"notify,omitempty"`
	Confirm         bool     `json:"confirm" jsonschema:"set true to actually update"`
}

type logTicketTaskTimeArgs struct {
	Profile     string  `json:"profile,omitempty"`
	AppID       int     `json:"appID,omitempty"`
	TicketID    int     `json:"ticketID"`
	TaskID      int     `json:"taskID"`
	Hours       float64 `json:"hours,omitempty"`
	Minutes     int     `json:"minutes,omitempty"`
	TypeID      int     `json:"typeID,omitempty"`
	TypeName    string  `json:"typeName,omitempty"`
	Date        string  `json:"date,omitempty"`
	Description string  `json:"description,omitempty"`
	Billable    *bool   `json:"billable,omitempty"`
	Confirm     bool    `json:"confirm" jsonschema:"set true to actually log"`
}

// RegisterTicketMutatingTools registers the 6 mutating ticket MCP tools.
func RegisterTicketMutatingTools(srv *sdkmcp.Server, svcs Services) {
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "add_ticket_comment",
		Description: "Post a feed comment to a ticket. Requires confirm=true.",
	}, addTicketCommentHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "update_ticket_status",
		Description: "Change a ticket's status. Pass statusID or statusName (resolved to ID automatically). Requires confirm=true.",
	}, updateTicketStatusHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "update_ticket_assignee",
		Description: "Change a ticket's assignee by UID. Requires confirm=true.",
	}, updateTicketAssigneeHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "log_ticket_time",
		Description: "Log time worked against a ticket (creates a time entry). Pass hours or minutes. Requires confirm=true.",
	}, logTicketTimeHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "update_ticket_task",
		Description: "Post a feed update to a ticket task with optional percentComplete/hoursWorked/comment. hoursWorked is informational; use log_ticket_task_time for real time entries. Requires confirm=true.",
	}, updateTicketTaskHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "log_ticket_task_time",
		Description: "Log time worked against a ticket task (creates a time entry). Requires confirm=true.",
	}, logTicketTaskTimeHandler(svcs))
}

// --- Handlers ---

func addTicketCommentHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, addTicketCommentArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args addTicketCommentArgs) (*sdkmcp.CallToolResult, any, error) {
		if result, ok := confirmGate(args.Confirm, "Set confirm: true to post the comment."); !ok {
			return result, nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)
		entryID, err := svcs.Tickets.AddFeed(ctx, profile, args.AppID, args.ID, args.Comments, args.IsPrivate, args.Notify)
		if err != nil {
			return errorResult(fmt.Sprintf("add_ticket_comment: %v", err)), nil, nil
		}
		return jsonResult(struct {
			Schema  string `json:"schema"`
			EntryID int    `json:"entryID"`
			Message string `json:"message"`
		}{
			Schema:  "tdx.v1.ticketFeedEntry",
			EntryID: entryID,
			Message: fmt.Sprintf("Comment posted to ticket %d (feed entry %d).", args.ID, entryID),
		})
	}
}

func updateTicketStatusHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, updateTicketStatusArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args updateTicketStatusArgs) (*sdkmcp.CallToolResult, any, error) {
		if result, ok := confirmGate(args.Confirm, "Set confirm: true to update the ticket status."); !ok {
			return result, nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)

		statusID := args.StatusID
		if statusID == 0 {
			if strings.TrimSpace(args.StatusName) == "" {
				return errorResult("update_ticket_status: provide statusID or statusName"), nil, nil
			}
			st, err := svcs.Tickets.ResolveStatusByName(ctx, profile, args.AppID, args.StatusName)
			if err != nil {
				return errorResult(fmt.Sprintf("update_ticket_status: %v", err)), nil, nil
			}
			statusID = st.ID
		}

		ops := []ticketsvc.PatchOp{
			{Op: "replace", Path: "/StatusID", Value: statusID},
		}
		t, err := svcs.Tickets.PatchTicket(ctx, profile, args.AppID, args.ID, ops)
		if err != nil {
			return errorResult(fmt.Sprintf("update_ticket_status: %v", err)), nil, nil
		}

		// Optionally post a comment.
		if args.Comment != "" {
			if _, cerr := svcs.Tickets.AddFeed(ctx, profile, args.AppID, args.ID, args.Comment, false, nil); cerr != nil {
				// Non-fatal: status was already updated; surface as warning in result.
				return jsonResult(struct {
					Schema  string           `json:"schema"`
					Ticket  ticketDetailJSON `json:"ticket"`
					Warning string           `json:"warning,omitempty"`
				}{
					Schema:  "tdx.v1.ticket",
					Ticket:  toTicketDetailJSON(t),
					Warning: fmt.Sprintf("status updated but comment failed: %v", cerr),
				})
			}
		}

		return jsonResult(struct {
			Schema string           `json:"schema"`
			Ticket ticketDetailJSON `json:"ticket"`
		}{Schema: "tdx.v1.ticket", Ticket: toTicketDetailJSON(t)})
	}
}

func updateTicketAssigneeHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, updateTicketAssigneeArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args updateTicketAssigneeArgs) (*sdkmcp.CallToolResult, any, error) {
		if result, ok := confirmGate(args.Confirm, "Set confirm: true to update the ticket assignee."); !ok {
			return result, nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)

		ops := []ticketsvc.PatchOp{
			{Op: "replace", Path: "/ResponsibleUid", Value: args.ResponsibleUID},
		}
		t, err := svcs.Tickets.PatchTicket(ctx, profile, args.AppID, args.ID, ops)
		if err != nil {
			return errorResult(fmt.Sprintf("update_ticket_assignee: %v", err)), nil, nil
		}

		// Optionally post a comment.
		if args.Comment != "" {
			if _, cerr := svcs.Tickets.AddFeed(ctx, profile, args.AppID, args.ID, args.Comment, false, nil); cerr != nil {
				return jsonResult(struct {
					Schema  string           `json:"schema"`
					Ticket  ticketDetailJSON `json:"ticket"`
					Warning string           `json:"warning,omitempty"`
				}{
					Schema:  "tdx.v1.ticket",
					Ticket:  toTicketDetailJSON(t),
					Warning: fmt.Sprintf("assignee updated but comment failed: %v", cerr),
				})
			}
		}

		return jsonResult(struct {
			Schema string           `json:"schema"`
			Ticket ticketDetailJSON `json:"ticket"`
		}{Schema: "tdx.v1.ticket", Ticket: toTicketDetailJSON(t)})
	}
}

func logTicketTimeHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, logTicketTimeArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args logTicketTimeArgs) (*sdkmcp.CallToolResult, any, error) {
		if result, ok := confirmGate(args.Confirm, "Set confirm: true to log time against the ticket."); !ok {
			return result, nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)

		// Resolve duration.
		mins, err := resolveMinutes(args.Hours, args.Minutes)
		if err != nil {
			return errorResult(fmt.Sprintf("log_ticket_time: %v", err)), nil, nil
		}

		// Resolve date (default today).
		var entryDate time.Time
		if args.Date == "" {
			entryDate = time.Now().In(domain.EasternTZ).Truncate(24 * time.Hour)
		} else {
			entryDate, err = time.ParseInLocation("2006-01-02", args.Date, domain.EasternTZ)
			if err != nil {
				return errorResult(fmt.Sprintf("log_ticket_time: invalid date %q: %v", args.Date, err)), nil, nil
			}
		}

		// Resolve time type.
		typeID := args.TypeID
		if typeID == 0 {
			if strings.TrimSpace(args.TypeName) == "" {
				return errorResult("log_ticket_time: provide typeID or typeName"), nil, nil
			}
			target := domain.Target{
				Kind:   domain.TargetTicket,
				AppID:  args.AppID,
				ItemID: args.ID,
			}
			types, terr := svcs.Time.TimeTypesForTarget(ctx, profile, target)
			if terr != nil {
				return errorResult(fmt.Sprintf("log_ticket_time: resolve time types: %v", terr)), nil, nil
			}
			needle := strings.ToLower(strings.TrimSpace(args.TypeName))
			var matched []domain.TimeType
			for _, tt := range types {
				if strings.ToLower(tt.Name) == needle {
					matched = append(matched, tt)
				}
			}
			switch len(matched) {
			case 0:
				names := make([]string, 0, len(types))
				for _, tt := range types {
					names = append(names, tt.Name)
				}
				return errorResult(fmt.Sprintf("log_ticket_time: no time type matches %q (available: %s)", args.TypeName, strings.Join(names, ", "))), nil, nil
			case 1:
				typeID = matched[0].ID
			default:
				return errorResult(fmt.Sprintf("log_ticket_time: multiple time types match %q — pass typeID instead", args.TypeName)), nil, nil
			}
		}

		// Resolve the caller's UID.
		user, err := svcs.Auth.WhoAmI(ctx, profile)
		if err != nil {
			return errorResult(fmt.Sprintf("log_ticket_time: auth: %v", err)), nil, nil
		}

		billable := false
		if args.Billable != nil {
			billable = *args.Billable
		}

		input := domain.EntryInput{
			UserUID:    user.UID,
			Date:       entryDate,
			Minutes:    mins,
			TimeTypeID: typeID,
			Billable:   billable,
			Target: domain.Target{
				Kind:   domain.TargetTicket,
				AppID:  args.AppID,
				ItemID: args.ID,
			},
			Description: args.Description,
		}

		entry, err := svcs.Time.AddEntry(ctx, profile, input)
		if err != nil {
			return errorResult(fmt.Sprintf("log_ticket_time: %v", err)), nil, nil
		}

		return jsonResult(struct {
			Schema  string           `json:"schema"`
			Entry   domain.TimeEntry `json:"entry"`
			Message string           `json:"message"`
		}{
			Schema:  "tdx.v1.timeEntry",
			Entry:   entry,
			Message: fmt.Sprintf("Logged %.2fh against ticket %d.", float64(mins)/60, args.ID),
		})
	}
}

func updateTicketTaskHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, updateTicketTaskArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args updateTicketTaskArgs) (*sdkmcp.CallToolResult, any, error) {
		if errResp, ok := confirmGate(args.Confirm, "set confirm=true to update the ticket task"); !ok {
			return errResp, nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)
		feedID, err := svcs.Tickets.UpdateTaskFeed(ctx, profile, args.AppID, args.TicketID, args.TaskID, args.Comment, args.PercentComplete, args.HoursWorked, args.IsPrivate, args.Notify)
		if err != nil {
			return errorResult(fmt.Sprintf("update_ticket_task: %v", err)), nil, nil
		}
		return jsonResult(struct {
			Schema   string `json:"schema"`
			TicketID int    `json:"ticketID"`
			TaskID   int    `json:"taskID"`
			FeedID   int    `json:"feedID"`
		}{Schema: "tdx.v1.ticketTaskUpdateResult", TicketID: args.TicketID, TaskID: args.TaskID, FeedID: feedID})
	}
}

func logTicketTaskTimeHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, logTicketTaskTimeArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args logTicketTaskTimeArgs) (*sdkmcp.CallToolResult, any, error) {
		if errResp, ok := confirmGate(args.Confirm, "set confirm=true to log time"); !ok {
			return errResp, nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)

		// Resolve duration.
		mins, err := resolveMinutes(args.Hours, args.Minutes)
		if err != nil {
			return errorResult(fmt.Sprintf("log_ticket_task_time: %v", err)), nil, nil
		}

		// Resolve date (default today).
		var entryDate time.Time
		if args.Date == "" {
			entryDate = time.Now().In(domain.EasternTZ).Truncate(24 * time.Hour)
		} else {
			entryDate, err = time.ParseInLocation("2006-01-02", args.Date, domain.EasternTZ)
			if err != nil {
				return errorResult(fmt.Sprintf("log_ticket_task_time: invalid date %q: %v", args.Date, err)), nil, nil
			}
		}

		// Resolve time type.
		target := domain.Target{
			Kind:   domain.TargetTicketTask,
			AppID:  args.AppID,
			ItemID: args.TicketID,
			TaskID: args.TaskID,
		}
		typeID := args.TypeID
		if typeID == 0 {
			if strings.TrimSpace(args.TypeName) == "" {
				return errorResult("log_ticket_task_time: provide typeID or typeName"), nil, nil
			}
			types, terr := svcs.Time.TimeTypesForTarget(ctx, profile, target)
			if terr != nil {
				return errorResult(fmt.Sprintf("log_ticket_task_time: resolve time types: %v", terr)), nil, nil
			}
			needle := strings.ToLower(strings.TrimSpace(args.TypeName))
			var matched []domain.TimeType
			for _, tt := range types {
				if strings.ToLower(tt.Name) == needle {
					matched = append(matched, tt)
				}
			}
			switch len(matched) {
			case 0:
				names := make([]string, 0, len(types))
				for _, tt := range types {
					names = append(names, tt.Name)
				}
				return errorResult(fmt.Sprintf("log_ticket_task_time: no time type matches %q (available: %s)", args.TypeName, strings.Join(names, ", "))), nil, nil
			case 1:
				typeID = matched[0].ID
			default:
				return errorResult(fmt.Sprintf("log_ticket_task_time: multiple time types match %q — pass typeID instead", args.TypeName)), nil, nil
			}
		}

		// Resolve the caller's UID.
		user, err := svcs.Auth.WhoAmI(ctx, profile)
		if err != nil {
			return errorResult(fmt.Sprintf("log_ticket_task_time: auth: %v", err)), nil, nil
		}

		billable := false
		if args.Billable != nil {
			billable = *args.Billable
		}

		entry, err := svcs.Time.AddEntry(ctx, profile, domain.EntryInput{
			UserUID:     user.UID,
			Date:        entryDate,
			Minutes:     mins,
			TimeTypeID:  typeID,
			Billable:    billable,
			Target:      target,
			Description: args.Description,
		})
		if err != nil {
			return errorResult(fmt.Sprintf("log_ticket_task_time: %v", err)), nil, nil
		}

		return jsonResult(struct {
			Schema  string           `json:"schema"`
			Entry   domain.TimeEntry `json:"entry"`
			Message string           `json:"message"`
		}{
			Schema:  "tdx.v1.timeEntry",
			Entry:   entry,
			Message: fmt.Sprintf("Logged %.2fh against task %d on ticket %d.", float64(mins)/60, args.TaskID, args.TicketID),
		})
	}
}
