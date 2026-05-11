// Package projectsvc owns operations against TeamDynamix's /api/projects/...
// endpoints. Mirrors the shape of ticketsvc.
package projectsvc

import (
	"fmt"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/tdx"
)

// Service wraps the TD client for project endpoints.
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
