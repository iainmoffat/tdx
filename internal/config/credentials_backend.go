package config

import (
	"fmt"
	"os"
)

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

// backendName values for TDX_TOKEN_BACKEND.
const (
	backendAuto     = "auto"
	backendKeychain = "keychain"
	backendYAML     = "yaml"
)

// selectBackend chooses a token backend based on the TDX_TOKEN_BACKEND
// environment variable. Returns an error only when the env var holds an
// unrecognized value or keychain is requested but not yet wired.
//
// Task 3: only "yaml" and "auto" return a real backend. "auto" falls back
// to yamlBackend silently because keychain support arrives in Task 4.
func selectBackend(paths Paths) (tokenBackend, error) {
	switch os.Getenv("TDX_TOKEN_BACKEND") {
	case "", backendAuto:
		return newYAMLBackend(paths), nil
	case backendYAML:
		return newYAMLBackend(paths), nil
	case backendKeychain:
		return nil, fmt.Errorf("TDX_TOKEN_BACKEND=keychain not yet supported (will land in Task 4)")
	default:
		return nil, fmt.Errorf("invalid TDX_TOKEN_BACKEND %q (want auto, keychain, or yaml)", os.Getenv("TDX_TOKEN_BACKEND"))
	}
}
