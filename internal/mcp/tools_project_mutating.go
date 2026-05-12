package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/iainmoffat/tdx/internal/domain"
)

// addProjectCommentArgs is the argument type for add_project_comment.
type addProjectCommentArgs struct {
	Profile    string   `json:"profile,omitempty"`
	ProjectID  int      `json:"projectID"`
	Message    string   `json:"message"`
	NotifyUIDs []string `json:"notifyUIDs,omitempty"`
	IsPrivate  bool     `json:"isPrivate,omitempty"`
	Confirm    bool     `json:"confirm" jsonschema:"set true to actually post"`
}

// addProjectTaskCommentArgs is the argument type for add_project_task_comment.
type addProjectTaskCommentArgs struct {
	Profile    string   `json:"profile,omitempty"`
	ProjectID  int      `json:"projectID"`
	PlanID     int      `json:"planID"`
	TaskID     int      `json:"taskID"`
	Message    string   `json:"message"`
	NotifyUIDs []string `json:"notifyUIDs,omitempty"`
	IsPrivate  bool     `json:"isPrivate,omitempty"`
	Confirm    bool     `json:"confirm" jsonschema:"set true to actually post"`
}

// logProjectTaskTimeArgs is the argument type for log_project_task_time.
type logProjectTaskTimeArgs struct {
	Profile     string  `json:"profile,omitempty"`
	ProjectID   int     `json:"projectID"`
	PlanID      int     `json:"planID"`
	TaskID      int     `json:"taskID"`
	Hours       float64 `json:"hours,omitempty"`
	Minutes     int     `json:"minutes,omitempty"`
	TypeID      int     `json:"typeID,omitempty"`
	TypeName    string  `json:"typeName,omitempty"`
	Date        string  `json:"date,omitempty"` // YYYY-MM-DD; default today
	Description string  `json:"description,omitempty"`
	Billable    *bool   `json:"billable,omitempty"`
	Confirm     bool    `json:"confirm" jsonschema:"set true to actually log"`
}

// RegisterProjectMutatingTools registers the 3 mutating project MCP tools.
func RegisterProjectMutatingTools(srv *sdkmcp.Server, svcs Services) {
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "log_project_task_time",
		Description: "Log time worked against a project task (creates a time entry). Requires confirm=true.",
	}, logProjectTaskTimeHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "add_project_comment",
		Description: "Post a feed comment to a project. Requires confirm=true.",
	}, addProjectCommentHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "add_project_task_comment",
		Description: "Post a feed comment to a project task. Requires confirm=true.",
	}, addProjectTaskCommentHandler(svcs))
}

func logProjectTaskTimeHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, logProjectTaskTimeArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args logProjectTaskTimeArgs) (*sdkmcp.CallToolResult, any, error) {
		if errResp, ok := confirmGate(args.Confirm, "Set confirm: true to log time against the project task."); !ok {
			return errResp, nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)

		if args.ProjectID <= 0 || args.PlanID <= 0 || args.TaskID <= 0 {
			return errorResult("log_project_task_time: projectID, planID, and taskID are required"), nil, nil
		}

		// Resolve duration.
		mins, err := resolveMinutes(args.Hours, args.Minutes)
		if err != nil {
			return errorResult(fmt.Sprintf("log_project_task_time: %v", err)), nil, nil
		}

		// Resolve date (default today).
		var entryDate time.Time
		if args.Date == "" {
			entryDate = time.Now().In(domain.EasternTZ).Truncate(24 * time.Hour)
		} else {
			entryDate, err = time.ParseInLocation("2006-01-02", args.Date, domain.EasternTZ)
			if err != nil {
				return errorResult(fmt.Sprintf("log_project_task_time: invalid date %q: %v", args.Date, err)), nil, nil
			}
		}

		// Target convention for project tasks (matches timesvc/encode.go and
		// componentPathFor): ItemID=planID, TaskID=taskID, ProjectID=projectID.
		target := domain.Target{
			Kind:      domain.TargetProjectTask,
			ItemID:    args.PlanID,
			TaskID:    args.TaskID,
			ProjectID: args.ProjectID,
		}
		typeID := args.TypeID
		if typeID == 0 {
			if strings.TrimSpace(args.TypeName) == "" {
				return errorResult("log_project_task_time: provide typeID or typeName"), nil, nil
			}
			types, terr := svcs.Time.TimeTypesForTarget(ctx, profile, target)
			if terr != nil {
				return errorResult(fmt.Sprintf("log_project_task_time: resolve time types: %v", terr)), nil, nil
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
				return errorResult(fmt.Sprintf("log_project_task_time: no time type matches %q (available: %s)", args.TypeName, strings.Join(names, ", "))), nil, nil
			case 1:
				typeID = matched[0].ID
			default:
				return errorResult(fmt.Sprintf("log_project_task_time: multiple time types match %q — pass typeID instead", args.TypeName)), nil, nil
			}
		}

		// Resolve the caller's UID.
		user, err := svcs.Auth.WhoAmI(ctx, profile)
		if err != nil {
			return errorResult(fmt.Sprintf("log_project_task_time: auth: %v", err)), nil, nil
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
			return errorResult(fmt.Sprintf("log_project_task_time: %v", err)), nil, nil
		}

		return jsonResult(struct {
			Schema  string           `json:"schema"`
			Entry   domain.TimeEntry `json:"entry"`
			Message string           `json:"message"`
		}{
			Schema:  "tdx.v1.timeEntry",
			Entry:   entry,
			Message: fmt.Sprintf("Logged %.2fh against task %d on project %d (plan %d).", float64(mins)/60, args.TaskID, args.ProjectID, args.PlanID),
		})
	}
}

func addProjectCommentHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, addProjectCommentArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args addProjectCommentArgs) (*sdkmcp.CallToolResult, any, error) {
		if result, ok := confirmGate(args.Confirm, "Set confirm: true to post the comment."); !ok {
			return result, nil, nil
		}
		if strings.TrimSpace(args.Message) == "" {
			return errorResult("add_project_comment: message is required"), nil, nil
		}
		if args.ProjectID <= 0 {
			return errorResult("add_project_comment: projectID is required"), nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)
		entryID, err := svcs.Projects.AddFeed(ctx, profile, args.ProjectID, args.Message, args.IsPrivate, args.NotifyUIDs)
		if err != nil {
			return errorResult(fmt.Sprintf("add_project_comment: %v", err)), nil, nil
		}
		return jsonResult(struct {
			Schema  string `json:"schema"`
			OK      bool   `json:"ok"`
			EntryID int    `json:"entryID"`
			Message string `json:"message"`
		}{
			Schema:  "tdx.v1.projectFeedEntry",
			OK:      true,
			EntryID: entryID,
			Message: fmt.Sprintf("Comment posted to project %d (feed entry %d).", args.ProjectID, entryID),
		})
	}
}

func addProjectTaskCommentHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, addProjectTaskCommentArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args addProjectTaskCommentArgs) (*sdkmcp.CallToolResult, any, error) {
		if result, ok := confirmGate(args.Confirm, "Set confirm: true to post the comment."); !ok {
			return result, nil, nil
		}
		if strings.TrimSpace(args.Message) == "" {
			return errorResult("add_project_task_comment: message is required"), nil, nil
		}
		if args.ProjectID <= 0 || args.PlanID <= 0 || args.TaskID <= 0 {
			return errorResult("add_project_task_comment: projectID, planID, and taskID are all required"), nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)
		entryID, err := svcs.Projects.AddTaskFeed(ctx, profile, args.ProjectID, args.PlanID, args.TaskID, args.Message, args.IsPrivate, args.NotifyUIDs)
		if err != nil {
			return errorResult(fmt.Sprintf("add_project_task_comment: %v", err)), nil, nil
		}
		return jsonResult(struct {
			Schema  string `json:"schema"`
			OK      bool   `json:"ok"`
			EntryID int    `json:"entryID"`
			Message string `json:"message"`
		}{
			Schema:  "tdx.v1.projectTaskFeedEntry",
			OK:      true,
			EntryID: entryID,
			Message: fmt.Sprintf("Comment posted to task #%d / project %d / plan %d (feed entry %d).", args.TaskID, args.ProjectID, args.PlanID, entryID),
		})
	}
}
