package people

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/peoplesvc"
)

func newSearchCmd(svc peoplesvcAPI) *cobra.Command {
	var (
		limitFlag      int
		includeClients bool
		jsonFlag       bool
		profileFlag    string
	)
	cmd := &cobra.Command{
		Use:   "search QUERY",
		Short: "Find people by name, email, or ID (defaults to staff only)",
		Long: `Search for people via TD's autocomplete endpoint
(/api/people/lookup). Matches against name, email, and external IDs.

By default, only employees (IsEmployee=true) are shown — the typical
"find a coworker" workflow doesn't want portal clients in the result.
Pass --include-clients to add them back.`,
		Args: cobra.ExactArgs(1),
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
			people := svc
			if people == nil {
				people = peoplesvc.New(paths)
			}
			return runPeopleSearch(cmd.Context(), cmd.OutOrStdout(), people,
				profile, args[0], limitFlag, includeClients, jsonFlag)
		},
	}
	cmd.Flags().IntVar(&limitFlag, "limit", 25, "max results (capped at 100)")
	cmd.Flags().BoolVar(&includeClients, "include-clients", false, "include portal clients in results (default: staff only)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

// runPeopleSearch is the pure implementation. Tests call this directly
// to bypass config.ResolvePaths.
//
// Default (staff-only): /api/people/search with IsEmployee=true returns
// full per-user records (including the right Title/ReportsTo data); we
// substring-match on the query client-side.
//
// --include-clients: switches to /api/people/lookup, which is the only
// way to reach portal users via the people API. Lookup returns degraded
// records (IsEmployee always false, ReportsTo/Title null), so we mark
// unknown employee status accordingly. Best for "I just want to find
// this person" rather than rich detail.
func runPeopleSearch(ctx context.Context, w io.Writer, svc peoplesvcAPI,
	profile, query string, limit int, includeClients, jsonOut bool) error {
	var filtered []domain.User
	if includeClients {
		hits, err := svc.LookupPeople(ctx, profile, query, limit)
		if err != nil {
			return err
		}
		filtered = hits
	} else {
		trueVal := true
		all, err := svc.SearchUsers(ctx, profile, domain.UserFilter{
			Employee: &trueVal,
			Limit:    5000,
		})
		if err != nil {
			return err
		}
		filtered = matchUsersByQuery(all, query, limit)
	}

	if jsonOut {
		type personJSON struct {
			UID        string `json:"uid"`
			FullName   string `json:"fullName"`
			Email      string `json:"email,omitempty"`
			Account    string `json:"account,omitempty"`
			Manager    string `json:"manager,omitempty"`
			IsEmployee bool   `json:"isEmployee"`
			IsActive   bool   `json:"isActive"`
			Title      string `json:"title,omitempty"`
		}
		out := make([]personJSON, 0, len(filtered))
		for _, u := range filtered {
			out = append(out, personJSON{
				UID: u.UID, FullName: u.FullName, Email: u.Email,
				Account: u.AccountName, Manager: u.ReportsToName,
				IsEmployee: u.IsEmployee, IsActive: u.Active, Title: u.Title,
			})
		}
		return render.JSON(w, struct {
			Schema string       `json:"schema"`
			Query  string       `json:"query"`
			People []personJSON `json:"people"`
		}{
			Schema: "tdx.v1.peopleSearchResult",
			Query:  query,
			People: out,
		})
	}

	if len(filtered) == 0 {
		if !includeClients {
			_, _ = fmt.Fprintf(w, "no staff match %q (use --include-clients to broaden)\n", query)
		} else {
			_, _ = fmt.Fprintf(w, "no people match %q\n", query)
		}
		return nil
	}

	headers := []string{"UID", "NAME", "EMAIL", "ACCOUNT", "MANAGER", "TITLE"}
	rows := make([][]string, 0, len(filtered))
	for _, u := range filtered {
		rows = append(rows, []string{
			shortUID(u.UID),
			u.FullName,
			u.Email,
			u.AccountName,
			u.ReportsToName,
			u.Title,
		})
	}
	render.Table(w, headers, rows, nil)
	return nil
}

// shortUID returns the first 8 chars of a UID for compact tables.
// Empty input returns empty.
func shortUID(uid string) string {
	if len(uid) >= 8 {
		return uid[:8]
	}
	return uid
}

// matchUsersByQuery is a case-insensitive substring match against FullName
// and Email. Used by the staff-only path of `tdx people search` to scope
// the bulk SearchUsers result down to what the user typed. Caps to limit
// results.
func matchUsersByQuery(users []domain.User, query string, limit int) []domain.User {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	out := make([]domain.User, 0, limit)
	for _, u := range users {
		if !strings.Contains(strings.ToLower(u.FullName), q) &&
			!strings.Contains(strings.ToLower(u.Email), q) {
			continue
		}
		out = append(out, u)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}
