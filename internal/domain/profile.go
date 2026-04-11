package domain

import (
	"fmt"
	"net/url"
	"strings"
)

// Profile names a TeamDynamix environment and its base URL.
type Profile struct {
	Name          string `yaml:"name"`
	TenantBaseURL string `yaml:"tenantBaseURL"`
}

// Validate returns nil if the profile is structurally sound.
func (p Profile) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidProfile)
	}
	if strings.ContainsAny(p.Name, "/\\ \t") {
		return fmt.Errorf("%w: name may not contain slashes or whitespace", ErrInvalidProfile)
	}
	if strings.TrimSpace(p.TenantBaseURL) == "" {
		return fmt.Errorf("%w: tenantBaseURL is required", ErrInvalidProfile)
	}
	u, err := url.Parse(p.TenantBaseURL)
	if err != nil {
		return fmt.Errorf("%w: tenantBaseURL: %v", ErrInvalidProfile, err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("%w: tenantBaseURL must use https", ErrInvalidProfile)
	}
	if u.Host == "" {
		return fmt.Errorf("%w: tenantBaseURL must include a host", ErrInvalidProfile)
	}
	return nil
}
