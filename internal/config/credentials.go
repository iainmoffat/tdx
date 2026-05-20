package config

// CredentialsStore persists bearer tokens per profile. It selects a backend
// at construction time based on TDX_TOKEN_BACKEND (auto / keychain / yaml).
// See credentials_backend.go for the tokenBackend interface and
// credentials_{yaml,keychain,memory}.go for implementations.
type CredentialsStore struct {
	backend tokenBackend
}

// NewCredentialsStore constructs a store with the YAML backend.
//
// TODO(Task 3): switch to selectBackend(paths) so the env var controls choice.
// TODO(Task 4): wire keychain backend.
// TODO(Task 5): wire auto-migration on Get miss.
func NewCredentialsStore(paths Paths) *CredentialsStore {
	return &CredentialsStore{backend: newYAMLBackend(paths)}
}

// GetToken returns the token for the named profile, or ErrNoCredentials.
func (s *CredentialsStore) GetToken(profile string) (string, error) {
	return s.backend.Get(profile)
}

// SetToken writes or replaces the token for the named profile.
func (s *CredentialsStore) SetToken(profile, token string) error {
	return s.backend.Set(profile, token)
}

// ClearToken removes the token for the named profile. Missing is not an error.
func (s *CredentialsStore) ClearToken(profile string) error {
	return s.backend.Clear(profile)
}
