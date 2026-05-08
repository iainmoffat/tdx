package mcp

import (
	"context"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/iainmoffat/tdx/internal/domain"
)

// --- Argument types ---

type listTicketAppsArgs struct {
	Profile string `json:"profile,omitempty"`
}

type listTicketTypesArgs struct {
	Profile string `json:"profile,omitempty"`
	AppID   int    `json:"appID,omitempty"`
}

type listTicketStatusesArgs struct {
	Profile string `json:"profile,omitempty"`
	AppID   int    `json:"appID,omitempty"`
}

type listSavedSearchesArgs struct {
	Profile string `json:"profile,omitempty"`
	AppID   int    `json:"appID,omitempty"`
}

type searchTicketsArgs struct {
	Profile       string   `json:"profile,omitempty"`
	AppID         int      `json:"appID,omitempty"`
	StatusIDs     []int    `json:"statusIDs,omitempty"`
	AssigneeUIDs  []string `json:"assigneeUIDs,omitempty"`
	RequestorUIDs []string `json:"requestorUIDs,omitempty"`
	AccountIDs    []int    `json:"accountIDs,omitempty"`
	SearchText    string   `json:"searchText,omitempty"`
	IncludeClosed bool     `json:"includeClosed,omitempty"`
	MaxResults    int      `json:"maxResults,omitempty"`
}

type runSavedSearchArgs struct {
	Profile    string `json:"profile,omitempty"`
	AppID      int    `json:"appID,omitempty"`
	SearchID   int    `json:"searchID"`
	MaxResults int    `json:"maxResults,omitempty"`
}

type getTicketArgs struct {
	Profile string `json:"profile,omitempty"`
	AppID   int    `json:"appID,omitempty"`
	ID      int    `json:"id"`
}

type getTicketFeedArgs struct {
	Profile string `json:"profile,omitempty"`
	AppID   int    `json:"appID,omitempty"`
	ID      int    `json:"id"`
	Limit   int    `json:"limit,omitempty"`
}

// --- JSON row types (mirrors cli/ticket shapes) ---

type ticketAppJSON struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Active      bool   `json:"active"`
	AppType     string `json:"appType,omitempty"`
}

type ticketTypeJSON struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Active      bool   `json:"active"`
}

type ticketStatusJSON struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	IsClosed  bool    `json:"isClosed"`
	IsDefault bool    `json:"isDefault"`
	Order     float64 `json:"order,omitempty"`
}

type savedSearchJSON struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	OwnerName   string `json:"ownerName,omitempty"`
	Description string `json:"description,omitempty"`
}

type ticketRowJSON struct {
	ID            int    `json:"id"`
	Title         string `json:"title"`
	StatusName    string `json:"statusName"`
	TypeName      string `json:"typeName,omitempty"`
	AssigneeName  string `json:"assigneeName,omitempty"`
	RequestorName string `json:"requestorName,omitempty"`
	ModifiedDate  string `json:"modifiedDate,omitempty"`
}

type ticketDetailJSON struct {
	ID               int      `json:"id"`
	AppID            int      `json:"appID,omitempty"`
	Title            string   `json:"title"`
	Description      string   `json:"description,omitempty"`
	StatusName       string   `json:"statusName,omitempty"`
	TypeName         string   `json:"typeName,omitempty"`
	PriorityName     string   `json:"priorityName,omitempty"`
	AccountName      string   `json:"accountName,omitempty"`
	AssigneeUID      string   `json:"assigneeUID,omitempty"`
	AssigneeName     string   `json:"assigneeName,omitempty"`
	RequestorUID     string   `json:"requestorUID,omitempty"`
	RequestorName    string   `json:"requestorName,omitempty"`
	CreatedDate      string   `json:"createdDate,omitempty"`
	ModifiedDate     string   `json:"modifiedDate,omitempty"`
	EstimatedMinutes int      `json:"estimatedMinutes,omitempty"`
	ActualMinutes    int      `json:"actualMinutes,omitempty"`
	Tags             []string `json:"tags,omitempty"`
}

type feedEntryJSON struct {
	ID         int    `json:"id"`
	AuthorUID  string `json:"authorUID,omitempty"`
	AuthorName string `json:"authorName,omitempty"`
	CreatedAt  string `json:"createdAt,omitempty"`
	Body       string `json:"body,omitempty"`
	IsPrivate  bool   `json:"isPrivate"`
	EventKind  string `json:"eventKind,omitempty"`
}

func toTicketRowJSON(t domain.Ticket) ticketRowJSON {
	md := ""
	if !t.ModifiedDate.IsZero() {
		md = t.ModifiedDate.Format("2006-01-02")
	}
	return ticketRowJSON{
		ID: t.ID, Title: t.Title, StatusName: t.StatusName, TypeName: t.TypeName,
		AssigneeName: t.ResponsibleName, RequestorName: t.RequestorName, ModifiedDate: md,
	}
}

func toTicketDetailJSON(t domain.Ticket) ticketDetailJSON {
	return ticketDetailJSON{
		ID:               t.ID,
		AppID:            t.AppID,
		Title:            t.Title,
		Description:      t.Description,
		StatusName:       t.StatusName,
		TypeName:         t.TypeName,
		PriorityName:     t.PriorityName,
		AccountName:      t.AccountName,
		AssigneeUID:      t.ResponsibleUID,
		AssigneeName:     t.ResponsibleName,
		RequestorUID:     t.RequestorUID,
		RequestorName:    t.RequestorName,
		CreatedDate:      formatTicketTime(t.CreatedDate),
		ModifiedDate:     formatTicketTime(t.ModifiedDate),
		EstimatedMinutes: t.EstimatedMinutes,
		ActualMinutes:    t.ActualMinutes,
		Tags:             t.Tags,
	}
}

func formatTicketTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// --- Registration ---

// RegisterTicketTools registers the 8 read-only ticket MCP tools.
func RegisterTicketTools(srv *sdkmcp.Server, svcs Services) {
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "list_ticket_apps",
		Description: "List ticket apps in the tenant. Read-only — no confirm required.",
	}, listTicketAppsHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "list_ticket_types",
		Description: "List ticket types in an app (default: profile's TicketAppID). Read-only.",
	}, listTicketTypesHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "list_ticket_statuses",
		Description: "List ticket statuses in an app. Read-only.",
	}, listTicketStatusesHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "list_saved_searches",
		Description: "List saved searches visible to the user. Read-only.",
	}, listSavedSearchesHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "search_tickets",
		Description: "Search tickets with filters (returns partial records). Read-only.",
	}, searchTicketsHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "run_saved_search",
		Description: "Execute a saved search by ID (rate-limited to 60 calls/min/IP). Read-only.",
	}, runSavedSearchHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name: "get_ticket",
		Description: "Get full detail for one ticket by ID. Read-only. " +
			"Note: this-week-time enrichment is not available via MCP; use the CLI (`tdx ticket show`) for that.",
	}, getTicketHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "get_ticket_feed",
		Description: "Read feed entries for a ticket. Read-only.",
	}, getTicketFeedHandler(svcs))
}

// --- Handlers ---

func listTicketAppsHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, listTicketAppsArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args listTicketAppsArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)
		apps, err := svcs.Tickets.ListApps(ctx, profile)
		if err != nil {
			return errorResult("list_ticket_apps: " + err.Error()), nil, nil
		}
		out := make([]ticketAppJSON, 0, len(apps))
		for _, a := range apps {
			out = append(out, ticketAppJSON{
				ID: a.ID, Name: a.Name, Description: a.Description,
				Active: a.Active, AppType: a.AppType,
			})
		}
		return jsonResult(struct {
			Schema string          `json:"schema"`
			Apps   []ticketAppJSON `json:"apps"`
		}{Schema: "tdx.v1.ticketAppList", Apps: out})
	}
}

func listTicketTypesHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, listTicketTypesArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args listTicketTypesArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)
		types, err := svcs.Tickets.ListTypes(ctx, profile, args.AppID)
		if err != nil {
			return errorResult("list_ticket_types: " + err.Error()), nil, nil
		}
		out := make([]ticketTypeJSON, 0, len(types))
		for _, t := range types {
			out = append(out, ticketTypeJSON{
				ID: t.ID, Name: t.Name, Description: t.Description, Active: t.Active,
			})
		}
		return jsonResult(struct {
			Schema string           `json:"schema"`
			Types  []ticketTypeJSON `json:"types"`
		}{Schema: "tdx.v1.ticketTypeList", Types: out})
	}
}

func listTicketStatusesHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, listTicketStatusesArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args listTicketStatusesArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)
		statuses, err := svcs.Tickets.ListStatuses(ctx, profile, args.AppID)
		if err != nil {
			return errorResult("list_ticket_statuses: " + err.Error()), nil, nil
		}
		out := make([]ticketStatusJSON, 0, len(statuses))
		for _, s := range statuses {
			out = append(out, ticketStatusJSON{
				ID: s.ID, Name: s.Name, IsClosed: s.IsClosed, IsDefault: s.IsDefault, Order: s.Order,
			})
		}
		return jsonResult(struct {
			Schema   string             `json:"schema"`
			Statuses []ticketStatusJSON `json:"statuses"`
		}{Schema: "tdx.v1.ticketStatusList", Statuses: out})
	}
}

func listSavedSearchesHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, listSavedSearchesArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args listSavedSearchesArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)
		searches, err := svcs.Tickets.ListSavedSearches(ctx, profile, args.AppID)
		if err != nil {
			return errorResult("list_saved_searches: " + err.Error()), nil, nil
		}
		out := make([]savedSearchJSON, 0, len(searches))
		for _, s := range searches {
			out = append(out, savedSearchJSON{
				ID: s.ID, Name: s.Name, OwnerName: s.OwnerName, Description: s.Description,
			})
		}
		return jsonResult(struct {
			Schema   string            `json:"schema"`
			Searches []savedSearchJSON `json:"savedSearches"`
		}{Schema: "tdx.v1.ticketSavedSearchList", Searches: out})
	}
}

func searchTicketsHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, searchTicketsArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args searchTicketsArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)
		filter := domain.TicketSearchFilter{
			AppID:         args.AppID,
			StatusIDs:     args.StatusIDs,
			AssigneeUIDs:  args.AssigneeUIDs,
			RequestorUIDs: args.RequestorUIDs,
			AccountIDs:    args.AccountIDs,
			Text:          args.SearchText,
			IncludeClosed: args.IncludeClosed,
			Limit:         args.MaxResults,
		}
		tickets, err := svcs.Tickets.SearchTickets(ctx, profile, filter)
		if err != nil {
			return errorResult("search_tickets: " + err.Error()), nil, nil
		}
		out := make([]ticketRowJSON, 0, len(tickets))
		for _, t := range tickets {
			out = append(out, toTicketRowJSON(t))
		}
		return jsonResult(struct {
			Schema  string          `json:"schema"`
			Tickets []ticketRowJSON `json:"tickets"`
		}{Schema: "tdx.v1.ticketList", Tickets: out})
	}
}

func runSavedSearchHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, runSavedSearchArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args runSavedSearchArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)
		tickets, err := svcs.Tickets.RunSavedSearch(ctx, profile, args.AppID, args.SearchID, args.MaxResults)
		if err != nil {
			return errorResult("run_saved_search: " + err.Error()), nil, nil
		}
		out := make([]ticketRowJSON, 0, len(tickets))
		for _, t := range tickets {
			out = append(out, toTicketRowJSON(t))
		}
		return jsonResult(struct {
			Schema  string          `json:"schema"`
			Tickets []ticketRowJSON `json:"tickets"`
		}{Schema: "tdx.v1.ticketList", Tickets: out})
	}
}

func getTicketHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, getTicketArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args getTicketArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)
		t, err := svcs.Tickets.GetTicket(ctx, profile, args.AppID, args.ID)
		if err != nil {
			return errorResult("get_ticket: " + err.Error()), nil, nil
		}
		return jsonResult(struct {
			Schema string           `json:"schema"`
			Ticket ticketDetailJSON `json:"ticket"`
		}{Schema: "tdx.v1.ticket", Ticket: toTicketDetailJSON(t)})
	}
}

func getTicketFeedHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, getTicketFeedArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args getTicketFeedArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)
		entries, err := svcs.Tickets.GetFeed(ctx, profile, args.AppID, args.ID)
		if err != nil {
			return errorResult("get_ticket_feed: " + err.Error()), nil, nil
		}
		// Apply optional limit.
		if args.Limit > 0 && len(entries) > args.Limit {
			entries = entries[:args.Limit]
		}
		out := make([]feedEntryJSON, 0, len(entries))
		for _, e := range entries {
			ts := ""
			if !e.CreatedAt.IsZero() {
				ts = e.CreatedAt.Format(time.RFC3339)
			}
			out = append(out, feedEntryJSON{
				ID: e.ID, AuthorUID: e.AuthorUID, AuthorName: e.AuthorName,
				CreatedAt: ts, Body: e.Body, IsPrivate: e.IsPrivate, EventKind: e.EventKind,
			})
		}
		return jsonResult(struct {
			Schema  string          `json:"schema"`
			Entries []feedEntryJSON `json:"entries"`
		}{Schema: "tdx.v1.ticketFeed", Entries: out})
	}
}
