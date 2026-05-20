package config

// tokenBackend is the per-process strategy for storing bearer tokens.
// Implementations: yamlBackend (file), keychainBackend (OS), memoryBackend
// (test-only).
type tokenBackend interface {
	// Get returns the token for the named profile, or ErrNoCredentials.
	Get(profile string) (string, error)

	// Set writes or replaces the token for the named profile.
	Set(profile, token string) error

	// Clear removes the token for the named profile. Missing is not an error.
	Clear(profile string) error

	// Name returns a stable identifier ("yaml", "keychain", "memory") for
	// diagnostics and migration logic.
	Name() string
}
