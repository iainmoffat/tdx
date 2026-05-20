package config

// CredentialsStore persists bearer tokens per profile. It selects a backend
// at construction time based on TDX_TOKEN_BACKEND (auto / keychain / yaml).
// See credentials_backend.go for the tokenBackend interface and
// credentials_{yaml,keychain,memory}.go for implementations.
//
// If TDX_TOKEN_BACKEND holds an invalid value, the store records the error
// and surfaces it on the first Get/Set/Clear call. This keeps the
// constructor signature single-valued so existing callers don't need
// updates.
type CredentialsStore struct {
	backend tokenBackend
	initErr error
}

// NewCredentialsStore constructs a store with the backend selected by
// TDX_TOKEN_BACKEND. Default (auto / unset) falls back to YAML — Task 4
// wires the keychain backend.
//
// TODO(Task 5): wire auto-migration on Get miss.
func NewCredentialsStore(paths Paths) *CredentialsStore {
	backend, err := selectBackend(paths)
	return &CredentialsStore{backend: backend, initErr: err}
}

// GetToken returns the token for the named profile, or ErrNoCredentials.
func (s *CredentialsStore) GetToken(profile string) (string, error) {
	if s.initErr != nil {
		return "", s.initErr
	}
	return s.backend.Get(profile)
}

// SetToken writes or replaces the token for the named profile.
func (s *CredentialsStore) SetToken(profile, token string) error {
	if s.initErr != nil {
		return s.initErr
	}
	return s.backend.Set(profile, token)
}

// ClearToken removes the token for the named profile. Missing is not an error.
func (s *CredentialsStore) ClearToken(profile string) error {
	if s.initErr != nil {
		return s.initErr
	}
	return s.backend.Clear(profile)
}
