package authsvc

import (
	"context"
	"fmt"

	"github.com/ipm/tdx/internal/domain"
	"github.com/ipm/tdx/internal/tdx"
)

// wireUser matches the JSON shape returned by GET /TDWebApi/api/auth/getuser.
// TD's actual response uses ReferenceID for the integer user ID (there is
// no top-level "ID" field) and AlternateEmail for the email fallback. These
// field names verified against the TD Web API on 2026-04-11.
type wireUser struct {
	ReferenceID    int    `json:"ReferenceID"`
	UID            string `json:"UID"`
	FullName       string `json:"FullName"`
	PrimaryEmail   string `json:"PrimaryEmail"`
	AlternateEmail string `json:"AlternateEmail"`
}

// WhoAmI returns the identity of the token owner for the given profile.
// The call is authenticated with the profile's stored credentials and
// makes one HTTP request to /TDWebApi/api/auth/getuser.
func (s *Service) WhoAmI(ctx context.Context, profileName string) (domain.User, error) {
	profile, err := s.profiles.GetProfile(profileName)
	if err != nil {
		return domain.User{}, err
	}
	token, err := s.credentials.GetToken(profileName)
	if err != nil {
		return domain.User{}, err
	}

	client, err := tdx.NewClient(profile.TenantBaseURL, token)
	if err != nil {
		return domain.User{}, fmt.Errorf("build client: %w", err)
	}

	var w wireUser
	if err := client.DoJSON(ctx, "GET", "/TDWebApi/api/auth/getuser", nil, &w); err != nil {
		return domain.User{}, fmt.Errorf("whoami: %w", err)
	}

	email := w.PrimaryEmail
	if email == "" {
		email = w.AlternateEmail
	}
	return domain.User{
		ID:       w.ReferenceID,
		UID:      w.UID,
		FullName: w.FullName,
		Email:    email,
	}, nil
}
