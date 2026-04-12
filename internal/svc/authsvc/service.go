package authsvc

import (
	"context"
	"errors"
	"fmt"

	"github.com/ipm/tdx/internal/config"
	"github.com/ipm/tdx/internal/domain"
	"github.com/ipm/tdx/internal/tdx"
)

// Service orchestrates the auth-related operations.
// It composes the profile store, credentials store, and TD client.
type Service struct {
	paths       config.Paths
	profiles    *config.ProfileStore
	credentials *config.CredentialsStore
}

// New constructs an auth service rooted at the given paths.
func New(paths config.Paths) *Service {
	return &Service{
		paths:       paths,
		profiles:    config.NewProfileStore(paths),
		credentials: config.NewCredentialsStore(paths),
	}
}

// LoginInput describes a paste-token login attempt.
type LoginInput struct {
	ProfileName   string
	TenantBaseURL string
	Token         string
}

// Login validates the token against TD and persists it on success.
// It upserts the profile: creating it if new, updating tenant URL if existing.
func (s *Service) Login(ctx context.Context, in LoginInput) (domain.Session, error) {
	profile := domain.Profile{
		Name:          in.ProfileName,
		TenantBaseURL: in.TenantBaseURL,
	}
	if err := profile.Validate(); err != nil {
		return domain.Session{}, err
	}
	if in.Token == "" {
		return domain.Session{}, fmt.Errorf("%w: empty token", domain.ErrInvalidToken)
	}

	client, err := tdx.NewClient(profile.TenantBaseURL, in.Token)
	if err != nil {
		return domain.Session{}, err
	}
	if err := client.Ping(ctx); err != nil {
		if errors.Is(err, tdx.ErrUnauthorized) {
			return domain.Session{}, fmt.Errorf("%w: server rejected token", domain.ErrInvalidToken)
		}
		return domain.Session{}, fmt.Errorf("validate token: %w", err)
	}

	// Upsert profile.
	if existing, err := s.profiles.GetProfile(profile.Name); err == nil {
		if existing.TenantBaseURL != profile.TenantBaseURL {
			if err := s.profiles.RemoveProfile(profile.Name); err != nil {
				return domain.Session{}, err
			}
			if err := s.profiles.AddProfile(profile); err != nil {
				return domain.Session{}, err
			}
		}
	} else if errors.Is(err, domain.ErrProfileNotFound) {
		if err := s.profiles.AddProfile(profile); err != nil {
			return domain.Session{}, err
		}
	} else {
		return domain.Session{}, err
	}

	if err := s.credentials.SetToken(profile.Name, in.Token); err != nil {
		return domain.Session{}, err
	}

	return domain.Session{Profile: profile, Token: in.Token}, nil
}

// Logout clears the token for a profile. The profile itself is preserved.
// Missing credentials are not an error.
func (s *Service) Logout(profileName string) error {
	return s.credentials.ClearToken(profileName)
}

// Status describes the current state of an auth profile.
type Status struct {
	Profile       domain.Profile
	Authenticated bool // a token is stored
	TokenValid    bool // the stored token was accepted by the server (only set if Authenticated)
	ValidationErr string
	User          domain.User `json:"user,omitempty"`
	UserErr       string      `json:"userErr,omitempty"`
}

// Status reports the state of a profile's credentials.
// If no token is stored, Authenticated is false and the server is not contacted.
// If a token is stored, the service makes a cheap Ping to verify it.
func (s *Service) Status(ctx context.Context, profileName string) (Status, error) {
	profile, err := s.profiles.GetProfile(profileName)
	if err != nil {
		return Status{}, err
	}
	status := Status{Profile: profile}

	token, err := s.credentials.GetToken(profileName)
	if errors.Is(err, domain.ErrNoCredentials) {
		return status, nil
	}
	if err != nil {
		return status, err
	}
	status.Authenticated = true

	client, err := tdx.NewClient(profile.TenantBaseURL, token)
	if err != nil {
		status.ValidationErr = err.Error()
		return status, nil
	}
	if err := client.Ping(ctx); err != nil {
		status.ValidationErr = err.Error()
		return status, nil
	}
	status.TokenValid = true

	// Identity lookup is additive: a failure here must not fail Status.
	user, err := s.WhoAmI(ctx, profileName)
	if err != nil {
		status.UserErr = err.Error()
		return status, nil
	}
	status.User = user
	return status, nil
}

// ResolveProfile picks a profile name from an explicit flag or the configured default.
// Used by commands that accept --profile.
func (s *Service) ResolveProfile(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	cfg, err := s.profiles.Load()
	if err != nil {
		return "", err
	}
	if cfg.DefaultProfile == "" {
		return "", fmt.Errorf("%w: no default profile configured", domain.ErrProfileNotFound)
	}
	return cfg.DefaultProfile, nil
}

// Profiles returns the underlying profile store (used by CLI commands that CRUD profiles).
func (s *Service) Profiles() *config.ProfileStore {
	return s.profiles
}

// Paths returns the resolved paths (used by the config command).
func (s *Service) Paths() config.Paths {
	return s.paths
}
