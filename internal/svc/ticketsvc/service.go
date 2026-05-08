// Package ticketsvc owns operations against TeamDynamix's /api/{appId}/tickets/...
// endpoints. Mirrors the shape of peoplesvc.
package ticketsvc

import (
	"fmt"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/tdx"
)

// Service wraps the TD client for ticket endpoints.
type Service struct {
	paths       config.Paths
	profiles    *config.ProfileStore
	credentials *config.CredentialsStore
}

// New constructs a Service rooted at the given paths.
func New(paths config.Paths) *Service {
	return &Service{
		paths:       paths,
		profiles:    config.NewProfileStore(paths),
		credentials: config.NewCredentialsStore(paths),
	}
}

// clientFor builds an authenticated tdx.Client for the named profile.
func (s *Service) clientFor(profileName string) (*tdx.Client, error) {
	profile, err := s.profiles.GetProfile(profileName)
	if err != nil {
		return nil, err
	}
	token, err := s.credentials.GetToken(profileName)
	if err != nil {
		return nil, err
	}
	client, err := tdx.NewClient(profile.TenantBaseURL, token)
	if err != nil {
		return nil, fmt.Errorf("build client: %w", err)
	}
	return client, nil
}

// resolveAppID returns the appID to use: explicit > 0 wins; otherwise
// fall back to profile.TicketAppID; if both zero, error.
//
//nolint:unused // scaffolded for upcoming ticket-command tasks
func (s *Service) resolveAppID(profileName string, explicit int) (int, error) {
	if explicit > 0 {
		return explicit, nil
	}
	profile, err := s.profiles.GetProfile(profileName)
	if err != nil {
		return 0, err
	}
	if profile.TicketAppID == 0 {
		return 0, fmt.Errorf("no ticket app configured for profile %q (run `tdx ticket app list` then `tdx ticket app use <id>`, or pass --app <id>)", profileName)
	}
	return profile.TicketAppID, nil
}
