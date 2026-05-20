package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/zalando/go-keyring"
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

const keychainProbeAccount = "__probe__"

// probeKeychain returns nil if the OS keychain is usable in this process.
// Tries a Get on a sentinel account we never Set; both nil and ErrNotFound
// indicate the backend is reachable.
//
// Variable, not function, so tests can swap it out.
var probeKeychain = func() error {
	_, err := keyring.Get(keychainServiceName, keychainProbeAccount)
	if err == nil || errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

// selectBackend chooses a token backend based on the TDX_TOKEN_BACKEND
// environment variable.
//
//   - "" / "auto": try keychain; if probe fails, fall back to yaml with a
//     stderr notice so the user knows to set TDX_TOKEN_BACKEND=yaml to silence.
//   - "keychain": keychain only; fail closed if probe fails.
//   - "yaml": yaml only; no keychain probe.
//   - anything else: error.
func selectBackend(paths Paths) (tokenBackend, error) {
	switch os.Getenv("TDX_TOKEN_BACKEND") {
	case "", backendAuto:
		if err := probeKeychain(); err != nil {
			fmt.Fprintf(os.Stderr,
				"notice: keychain unavailable, using credentials.yaml (set TDX_TOKEN_BACKEND=yaml to silence)\n")
			return newYAMLBackend(paths), nil
		}
		return newKeychainBackend(), nil
	case backendKeychain:
		if err := probeKeychain(); err != nil {
			return nil, fmt.Errorf("TDX_TOKEN_BACKEND=keychain but keychain is unavailable: %w", err)
		}
		return newKeychainBackend(), nil
	case backendYAML:
		return newYAMLBackend(paths), nil
	default:
		return nil, fmt.Errorf("invalid TDX_TOKEN_BACKEND %q (want auto, keychain, or yaml)", os.Getenv("TDX_TOKEN_BACKEND"))
	}
}
