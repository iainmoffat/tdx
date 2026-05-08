package ticket

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/iainmoffat/tdx/internal/domain"
)

// peoplesvcAPI is the subset of peoplesvc used by ticket helpers.
// Mirrors the LookupPeople signature in internal/svc/peoplesvc.
type peoplesvcAPI interface {
	LookupPeople(ctx context.Context, profile string, q string, limit int) ([]domain.User, error)
	SearchUsers(ctx context.Context, profile string, filter domain.UserFilter) ([]domain.User, error)
}

// resolvePrincipal maps a CLI argument to a UID.
//
//   - "me" → authedUID (must be provided by caller; error if empty)
//   - looks like a UID (32+ chars with at least 4 dashes) → returned as-is
//   - otherwise → looked up via people.LookupPeople with limit=5; ambiguous
//     matches return a candidate-list error
func resolvePrincipal(ctx context.Context, people peoplesvcAPI, profile, authedUID, arg string) (string, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "", fmt.Errorf("principal argument required")
	}
	if arg == "me" {
		if authedUID == "" {
			return "", fmt.Errorf("`me` requires an authenticated session — run `tdx auth status` to verify")
		}
		return authedUID, nil
	}
	// UID heuristic: 36-char hex+dashes (TD UIDs are GUID-shaped)
	if len(arg) >= 32 && strings.Count(arg, "-") >= 4 {
		return arg, nil
	}
	if people == nil {
		return "", fmt.Errorf("cannot resolve %q without a people-service", arg)
	}
	users, err := people.LookupPeople(ctx, profile, arg, 5)
	if err != nil {
		return "", fmt.Errorf("look up %q: %w", arg, err)
	}
	switch len(users) {
	case 0:
		return "", fmt.Errorf("no user matches %q", arg)
	case 1:
		return users[0].UID, nil
	default:
		labels := make([]string, 0, len(users))
		for _, u := range users {
			labels = append(labels, fmt.Sprintf("%s (%s)", u.FullName, u.Email))
		}
		return "", fmt.Errorf("multiple users match %q: %s — pass UID directly", arg, strings.Join(labels, ", "))
	}
}

// parseStatusArg returns (statusID, statusName-or-empty). If arg is purely
// numeric, treat as ID; else, name (caller resolves via ResolveStatusByName).
func parseStatusArg(arg string) (int, string) {
	arg = strings.TrimSpace(arg)
	if id, err := strconv.Atoi(arg); err == nil {
		return id, ""
	}
	return 0, arg
}

// partialResultBanner returns a footer line warning about partial records.
// Used by `tdx ticket search` and `tdx ticket search saved` after rendering rows.
// Returns empty string for n == 0 (no banner needed when no rows).
func partialResultBanner(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("(%d row(s) — partial; use `tdx ticket show <id>` for full detail)", n)
}
