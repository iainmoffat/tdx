package ticket

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/peoplesvc"
	"github.com/iainmoffat/tdx/internal/svc/ticketsvc"
)

func newSearchCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		statusFlags    []string
		assigneeFlags  []string
		requestorFlags []string
		accountFlag    string // accepted but currently no-op; add support in a follow-up
		textFlag       string
		limitFlag      int
		includeClosed  bool
		appID          int
		jsonFlag       bool
		profileFlag    string
	)
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search tickets in the current app (default: my open)",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			auth := authsvc.New(paths)
			profile, err := auth.ResolveProfile(profileFlag)
			if err != nil {
				return err
			}

			s := svc
			if s == nil {
				s = ticketsvc.New(paths)
			}
			people := peoplesvc.New(paths)

			authedUID, err := authedUIDFor(cmd.Context(), auth, profile)
			if err != nil {
				return err
			}

			filter, err := buildSearchFilter(cmd.Context(), s, people, profile, authedUID, appID, statusFlags, assigneeFlags, requestorFlags, accountFlag, textFlag, limitFlag, includeClosed)
			if err != nil {
				return err
			}
			return runTicketSearch(cmd.Context(), cmd.OutOrStdout(), s, profile, filter, jsonFlag)
		},
	}
	cmd.Flags().StringSliceVar(&statusFlags, "status", nil, "filter by status name or id (repeatable)")
	cmd.Flags().StringSliceVar(&assigneeFlags, "assignee", nil, "assignee me|UID|email (repeatable; default = me)")
	cmd.Flags().StringSliceVar(&requestorFlags, "requestor", nil, "requestor me|UID|email (repeatable)")
	cmd.Flags().StringVar(&accountFlag, "account", "", "account/department name (currently informational)")
	cmd.Flags().StringVar(&textFlag, "text", "", "free-text search")
	cmd.Flags().IntVar(&limitFlag, "limit", 50, "max results (capped at 1000)")
	cmd.Flags().BoolVar(&includeClosed, "include-closed", false, "include closed tickets")
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id (overrides profile default)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	cmd.AddCommand(newSearchSavedCmd(svc))
	return cmd
}

// authedUIDFor returns the authenticated user's UID for the given profile.
// Uses authsvc.WhoAmI (which calls GET /TDWebApi/api/auth/getuser) — the same
// path used by `tdx auth status`. The UID is the GUID-shaped string from the
// User.UID field.
func authedUIDFor(ctx context.Context, auth *authsvc.Service, profile string) (string, error) {
	user, err := auth.WhoAmI(ctx, profile)
	if err != nil {
		return "", fmt.Errorf("resolve authenticated user: %w", err)
	}
	if user.UID == "" {
		return "", fmt.Errorf("authenticated user has no UID — try re-running `tdx auth login`")
	}
	return user.UID, nil
}

// buildSearchFilter constructs domain.TicketSearchFilter from CLI flags.
// Behavior:
//   - If no --assignee and no --requestor flags, default to assignee=authedUID
//   - Resolve each "me" in --assignee/--requestor to authedUID
//   - Resolve --status names → IDs via svc.ResolveStatusByName (numeric strings stay numeric)
//   - --account is currently informational (TD's POST /tickets/search has weak filter fidelity)
//   - --limit is clamped to [1, 1000]
func buildSearchFilter(ctx context.Context, svc ticketsvcAPI, people peoplesvcAPI, profile, authedUID string, appID int, statusFlags, assigneeFlags, requestorFlags []string, _ string, text string, limit int, includeClosed bool) (domain.TicketSearchFilter, error) {
	if limit < 1 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}

	statusIDs := make([]int, 0, len(statusFlags))
	for _, raw := range statusFlags {
		id, name := parseStatusArg(raw)
		if id > 0 {
			statusIDs = append(statusIDs, id)
			continue
		}
		st, err := svc.ResolveStatusByName(ctx, profile, appID, name)
		if err != nil {
			return domain.TicketSearchFilter{}, fmt.Errorf("--status %q: %w", raw, err)
		}
		statusIDs = append(statusIDs, st.ID)
	}

	resolveAll := func(args []string) ([]string, error) {
		out := make([]string, 0, len(args))
		for _, a := range args {
			uid, err := resolvePrincipal(ctx, people, profile, authedUID, a)
			if err != nil {
				return nil, err
			}
			out = append(out, uid)
		}
		return out, nil
	}

	assignees, err := resolveAll(assigneeFlags)
	if err != nil {
		return domain.TicketSearchFilter{}, fmt.Errorf("--assignee: %w", err)
	}
	requestors, err := resolveAll(requestorFlags)
	if err != nil {
		return domain.TicketSearchFilter{}, fmt.Errorf("--requestor: %w", err)
	}

	// Default: if user gave neither assignee nor requestor flags, default to assignee=me.
	if len(assignees) == 0 && len(requestors) == 0 {
		if authedUID == "" {
			return domain.TicketSearchFilter{}, fmt.Errorf("no filter specified and no authenticated UID — pass --assignee or --requestor explicitly")
		}
		assignees = []string{authedUID}
	}

	return domain.TicketSearchFilter{
		AppID:         appID,
		StatusIDs:     statusIDs,
		AssigneeUIDs:  assignees,
		RequestorUIDs: requestors,
		Text:          text,
		IncludeClosed: includeClosed,
		Limit:         limit,
	}, nil
}

func runTicketSearch(ctx context.Context, w io.Writer, svc ticketsvcAPI, profile string, filter domain.TicketSearchFilter, jsonOut bool) error {
	tickets, err := svc.SearchTickets(ctx, profile, filter)
	if err != nil {
		return err
	}
	return printTicketList(w, tickets, jsonOut, "tdx.v1.ticketList")
}

// printTicketList renders rows or JSON. Reused by search and search-saved.
func printTicketList(w io.Writer, tickets []domain.Ticket, jsonOut bool, schema string) error {
	if jsonOut {
		type ticketJSON struct {
			ID            int    `json:"id"`
			Title         string `json:"title"`
			StatusName    string `json:"statusName"`
			TypeName      string `json:"typeName,omitempty"`
			AssigneeName  string `json:"assigneeName,omitempty"`
			RequestorName string `json:"requestorName,omitempty"`
			ModifiedDate  string `json:"modifiedDate,omitempty"`
		}
		out := make([]ticketJSON, 0, len(tickets))
		for _, t := range tickets {
			out = append(out, ticketJSON{
				ID: t.ID, Title: t.Title, StatusName: t.StatusName, TypeName: t.TypeName,
				AssigneeName: t.ResponsibleName, RequestorName: t.RequestorName,
				ModifiedDate: formatDate(t.ModifiedDate),
			})
		}
		return render.JSON(w, struct {
			Schema  string       `json:"schema"`
			Tickets []ticketJSON `json:"tickets"`
		}{Schema: schema, Tickets: out})
	}
	if len(tickets) == 0 {
		_, _ = fmt.Fprintln(w, "no tickets matched")
		return nil
	}
	headers := []string{"ID", "TITLE", "STATUS", "TYPE", "ASSIGNEE", "REQUESTOR", "MODIFIED"}
	rows := make([][]string, 0, len(tickets))
	for _, t := range tickets {
		rows = append(rows, []string{
			strconv.Itoa(t.ID),
			truncate(t.Title, 60),
			t.StatusName,
			t.TypeName,
			t.ResponsibleName,
			t.RequestorName,
			formatDate(t.ModifiedDate),
		})
	}
	render.Table(w, headers, rows, nil)
	if banner := partialResultBanner(len(tickets)); banner != "" {
		_, _ = fmt.Fprintln(w, banner)
	}
	return nil
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func newSearchSavedCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		appID       int
		jsonFlag    bool
		limitFlag   int
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "saved [NAME]",
		Short: "List saved searches; with NAME, run that saved search",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			auth := authsvc.New(paths)
			profile, err := auth.ResolveProfile(profileFlag)
			if err != nil {
				return err
			}
			s := svc
			if s == nil {
				s = ticketsvc.New(paths)
			}
			if len(args) == 0 {
				return runSavedSearchList(cmd.Context(), cmd.OutOrStdout(), s, profile, appID, jsonFlag)
			}
			return runSavedSearchRun(cmd.Context(), cmd.OutOrStdout(), s, profile, appID, args[0], limitFlag, jsonFlag)
		},
	}
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id (overrides profile default)")
	cmd.Flags().IntVar(&limitFlag, "limit", 50, "max results when running")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runSavedSearchList(ctx context.Context, w io.Writer, svc ticketsvcAPI, profile string, appID int, jsonOut bool) error {
	searches, err := svc.ListSavedSearches(ctx, profile, appID)
	if err != nil {
		return err
	}
	if jsonOut {
		type savedJSON struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			OwnerName   string `json:"ownerName,omitempty"`
			Description string `json:"description,omitempty"`
		}
		out := make([]savedJSON, 0, len(searches))
		for _, s := range searches {
			out = append(out, savedJSON{ID: s.ID, Name: s.Name, OwnerName: s.OwnerName, Description: s.Description})
		}
		return render.JSON(w, struct {
			Schema   string      `json:"schema"`
			Searches []savedJSON `json:"savedSearches"`
		}{Schema: "tdx.v1.ticketSavedSearchList", Searches: out})
	}
	if len(searches) == 0 {
		_, _ = fmt.Fprintln(w, "no saved searches found")
		return nil
	}
	headers := []string{"ID", "NAME", "OWNER", "DESCRIPTION"}
	rows := make([][]string, 0, len(searches))
	for _, s := range searches {
		rows = append(rows, []string{strconv.Itoa(s.ID), s.Name, s.OwnerName, s.Description})
	}
	render.Table(w, headers, rows, nil)
	return nil
}

func runSavedSearchRun(ctx context.Context, w io.Writer, svc ticketsvcAPI, profile string, appID int, name string, limit int, jsonOut bool) error {
	saved, err := svc.ResolveSavedSearchByName(ctx, profile, appID, name)
	if err != nil {
		return err
	}
	tickets, err := svc.RunSavedSearch(ctx, profile, appID, saved.ID, limit)
	if err != nil {
		// Surface 429 specifically if the underlying error indicates rate limiting.
		if strings.Contains(err.Error(), "429") || strings.Contains(strings.ToLower(err.Error()), "rate limit") {
			return fmt.Errorf("rate limited by TD (60 calls/min/IP) — wait and retry: %w", err)
		}
		return err
	}
	return printTicketList(w, tickets, jsonOut, "tdx.v1.ticketList")
}
