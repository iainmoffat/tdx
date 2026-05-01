package peoplesvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/iainmoffat/tdx/internal/domain"
)

// GetUser fetches a single user by UID.
func (s *Service) GetUser(ctx context.Context, profileName, uid string) (domain.User, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return domain.User{}, err
	}
	var w wireUser
	path := fmt.Sprintf("/TDWebApi/api/people/%s", uid)
	if err := client.DoJSON(ctx, "GET", path, nil, &w); err != nil {
		return domain.User{}, fmt.Errorf("get user %s: %w", uid, err)
	}
	return decodeUser(w), nil
}

// SearchUsers calls POST /api/people/search with the given filter.
// Default UserType="User", IsActive=true, MaxResults=100.
func (s *Service) SearchUsers(ctx context.Context, profileName string, filter domain.UserFilter) ([]domain.User, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	req := wireUserSearch{
		NameLike:   filter.NameLike,
		UserType:   filter.UserType,
		MaxResults: filter.Limit,
	}
	if req.UserType == "" {
		req.UserType = "User"
	}
	if req.MaxResults == 0 {
		req.MaxResults = 100
	}
	if filter.Active == nil {
		t := true
		req.IsActive = &t
	} else {
		req.IsActive = filter.Active
	}
	if filter.Employee != nil {
		req.IsEmployee = filter.Employee
	}
	if filter.AccountID > 0 {
		req.AccountIDs = []int{filter.AccountID}
	}

	var wire []wireUser
	if err := client.DoJSON(ctx, "POST", "/TDWebApi/api/people/search", req, &wire); err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}
	out := make([]domain.User, 0, len(wire))
	for _, w := range wire {
		out = append(out, decodeUser(w))
	}
	return out, nil
}

// decodeUser maps a wireUser into domain.User. Email falls back to
// AlternateEmail when PrimaryEmail is empty.
func decodeUser(w wireUser) domain.User {
	email := w.PrimaryEmail
	if email == "" {
		email = w.AlternateEmail
	}
	return domain.User{
		ID:               w.ID,
		UID:              w.UID,
		FullName:         w.FullName,
		Email:            email,
		Active:           w.IsActive,
		AccountName:      w.DefaultAccountName,
		ReportsToUID:     w.ReportsToUid,
		ReportsToID:      w.ReportsToId,
		ReportsToName:    w.ReportsToFullName,
		ReportsToEmail:   w.ReportsToEmail,
		ResourcePoolID:   w.ResourcePoolID,
		ResourcePoolName: strings.TrimSpace(w.ResourcePoolName),
	}
}
