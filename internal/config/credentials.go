package config

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/iainmoffat/tdx/internal/domain"
)

// CredentialsStore persists bearer tokens per profile. It selects a backend
// at construction time based on TDX_TOKEN_BACKEND (auto / keychain / yaml).
// On Get miss under a non-YAML backend, the store attempts a one-time
// migration from credentials.yaml — see credentials_migration_test.go.
type CredentialsStore struct {
	backend tokenBackend
	yaml    *yamlBackend // always set, used for auto-migration

	initErr error

	// migrationNoticed records profiles already announced as migrated this
	// process so the stderr notice prints at most once per profile per run.
	mu               sync.Mutex
	migrationNoticed map[string]bool
}

// NewCredentialsStore constructs a store. If TDX_TOKEN_BACKEND holds an
// invalid value the store stores the error and surfaces it on the first
// Get/Set/Clear call.
func NewCredentialsStore(paths Paths) *CredentialsStore {
	backend, err := selectBackend(paths)
	return &CredentialsStore{
		backend:          backend,
		yaml:             newYAMLBackend(paths),
		initErr:          err,
		migrationNoticed: map[string]bool{},
	}
}

// GetToken returns the token for the named profile, or ErrNoCredentials.
// Under a non-YAML backend, a YAML token found here is auto-migrated.
func (s *CredentialsStore) GetToken(profile string) (string, error) {
	if s.initErr != nil {
		return "", s.initErr
	}
	token, err := s.backend.Get(profile)
	if err == nil {
		return token, nil
	}
	if !errors.Is(err, domain.ErrNoCredentials) {
		return "", err
	}
	// Backend miss. If the active backend isn't yaml AND yaml has a token,
	// migrate.
	if s.backend.Name() == "yaml" {
		return "", err
	}
	yamlToken, yerr := s.yaml.Get(profile)
	if yerr != nil {
		// No YAML token either — propagate the original miss.
		return "", err
	}
	return s.migrate(profile, yamlToken)
}

// migrate moves a token from yaml to the active backend with rollback-safe
// semantics. See the spec's "Migration semantics" section for invariants.
func (s *CredentialsStore) migrate(profile, yamlToken string) (string, error) {
	// 1. Write to active backend.
	if err := s.backend.Set(profile, yamlToken); err != nil {
		// Leave YAML untouched; bubble the error.
		return "", fmt.Errorf("migrate %q to %s: %w", profile, s.backend.Name(), err)
	}
	// 2. Read back; if it differs, defensive abort.
	readBack, err := s.backend.Get(profile)
	if err != nil || readBack != yamlToken {
		return "", fmt.Errorf("migrate %q to %s: round-trip mismatch", profile, s.backend.Name())
	}
	// 3. Clear from YAML.
	if cerr := s.yaml.Clear(profile); cerr != nil {
		// Token now in BOTH places. Warn but return the token so the
		// command can proceed.
		fmt.Fprintf(os.Stderr,
			"warning: token migrated to %s but YAML cleanup failed: %v (re-run will retry)\n",
			s.backend.Name(), cerr)
		return yamlToken, nil
	}
	// 4. Print one-time notice per profile per process.
	s.mu.Lock()
	already := s.migrationNoticed[profile]
	s.migrationNoticed[profile] = true
	s.mu.Unlock()
	if !already {
		fmt.Fprintf(os.Stderr,
			"notice: migrated token for profile %q from credentials.yaml to OS keychain\n",
			profile)
	}
	return yamlToken, nil
}

// SetToken writes or replaces the token via the active backend.
func (s *CredentialsStore) SetToken(profile, token string) error {
	if s.initErr != nil {
		return s.initErr
	}
	return s.backend.Set(profile, token)
}

// ClearToken removes the token from the active backend AND from yaml. Missing
// entries on either side are ignored so logout always produces a clean state.
func (s *CredentialsStore) ClearToken(profile string) error {
	if s.initErr != nil {
		return s.initErr
	}
	// Clear active backend first.
	bErr := s.backend.Clear(profile)
	// Always also clear YAML — even when it IS the active backend, this is a
	// safe no-op (yamlBackend.Clear handles missing entries).
	yErr := s.yaml.Clear(profile)
	if bErr != nil && !errors.Is(bErr, domain.ErrNoCredentials) {
		return bErr
	}
	if yErr != nil && !errors.Is(yErr, domain.ErrNoCredentials) {
		return yErr
	}
	return nil
}
