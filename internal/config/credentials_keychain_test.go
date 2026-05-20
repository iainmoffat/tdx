package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

// TestKeychainBackend_LiveRoundTrip exercises the real OS keychain end-to-end
// when one is available. Self-skips otherwise (e.g. CI Linux without D-Bus,
// or any environment where the probe fails). Uses a unique random account
// name to avoid colliding with the user's real tdx tokens; cleans up via
// t.Cleanup.
//
// To force-skip this test even on machines with a keychain, set
// TDX_TOKEN_BACKEND=yaml — the test honors that opt-out so a user running
// `go test ./...` doesn't see a Keychain Access permission prompt on macOS.
// (The package's TestMain defaults TDX_TOKEN_BACKEND=yaml so this test
// self-skips during normal `go test ./...` runs. Override with
// `TDX_TOKEN_BACKEND=auto go test ./internal/config/... -run LiveRoundTrip`
// to exercise it locally.)
func TestKeychainBackend_LiveRoundTrip(t *testing.T) {
	if os.Getenv("TDX_TOKEN_BACKEND") == "yaml" {
		t.Skip("TDX_TOKEN_BACKEND=yaml; skipping live keychain test")
	}
	if err := probeKeychain(); err != nil {
		t.Skipf("keychain not available: %v", err)
	}

	// Random account name so we don't collide with real tokens.
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		t.Fatal(err)
	}
	account := "__tdx_test_" + hex.EncodeToString(buf[:]) + "__"

	b := newKeychainBackend()
	t.Cleanup(func() { _ = b.Clear(account) })

	// Initial Get → ErrNoCredentials
	_, err := b.Get(account)
	require.ErrorIs(t, err, domain.ErrNoCredentials)

	// Set
	require.NoError(t, b.Set(account, "live-test-token"))

	// Get back the same value
	got, err := b.Get(account)
	require.NoError(t, err)
	require.Equal(t, "live-test-token", got)

	// Clear
	require.NoError(t, b.Clear(account))

	// Now Get → ErrNoCredentials again
	_, err = b.Get(account)
	require.ErrorIs(t, err, domain.ErrNoCredentials)
}
