// Package timesvc owns every read operation against TeamDynamix's Time Web
// API. It composes the Phase 1 profile and credentials stores with the
// tdx.Client and exposes typed domain-shaped methods to CLI and MCP callers.
package timesvc

import (
	"fmt"

	"github.com/ipm/tdx/internal/config"
	"github.com/ipm/tdx/internal/tdx"
)

// Service is the read-only time operations facade. It holds references to
// the stores and resolves a fresh tdx.Client per call, so in-process token
// changes are always picked up.
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

// clientFor builds an authenticated tdx.Client for the named profile. It
// returns the domain sentinel errors directly so callers can errors.Is
// them without extra wrapping.
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
