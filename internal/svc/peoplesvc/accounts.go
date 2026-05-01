package peoplesvc

import (
	"context"
	"fmt"
	"strings"
)

// Account is a TD account/department as returned by /api/accounts/search.
type Account struct {
	ID              int
	Name            string
	IsActive        bool
	ParentID        int
	ParentName      string
	Code            string
	ManagerUID      string
	ManagerFullName string
}

// SearchAccounts lists all accounts visible to the profile. TD's
// /api/accounts/search ignores filter inputs; we always send {} and
// decode the full list.
func (s *Service) SearchAccounts(ctx context.Context, profileName string) ([]Account, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	var wire []wireAccount
	if err := client.DoJSON(ctx, "POST", "/TDWebApi/api/accounts/search", struct{}{}, &wire); err != nil {
		return nil, fmt.Errorf("search accounts: %w", err)
	}
	out := make([]Account, 0, len(wire))
	for _, w := range wire {
		out = append(out, Account{
			ID:              w.ID,
			Name:            strings.TrimSpace(w.Name),
			IsActive:        w.IsActive,
			ParentID:        w.ParentID,
			ParentName:      strings.TrimSpace(w.ParentName),
			Code:            w.Code,
			ManagerUID:      w.ManagerUID,
			ManagerFullName: w.ManagerFullName,
		})
	}
	return out, nil
}

// ResolveAccountByName looks up a single account by case-insensitive name
// (after trimming whitespace). Returns an error when no account matches or
// when multiple accounts share the name.
func (s *Service) ResolveAccountByName(ctx context.Context, profileName, name string) (Account, error) {
	target := strings.ToLower(strings.TrimSpace(name))
	if target == "" {
		return Account{}, fmt.Errorf("account name is empty")
	}
	accounts, err := s.SearchAccounts(ctx, profileName)
	if err != nil {
		return Account{}, err
	}
	matches := []Account{}
	for _, a := range accounts {
		if strings.ToLower(a.Name) == target {
			matches = append(matches, a)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return Account{}, fmt.Errorf("account %q not found among %d accounts", name, len(accounts))
	default:
		ids := make([]string, 0, len(matches))
		for _, m := range matches {
			ids = append(ids, fmt.Sprintf("%d", m.ID))
		}
		return Account{}, fmt.Errorf("account %q is ambiguous (matched IDs %s)", name, strings.Join(ids, ", "))
	}
}
