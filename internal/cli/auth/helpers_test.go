package auth

import (
	"os"
	"testing"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/stretchr/testify/require"
)

// TestMain defaults TDX_TOKEN_BACKEND=yaml for this package so tests never
// touch the dev's real OS keychain.
func TestMain(m *testing.M) {
	if os.Getenv("TDX_TOKEN_BACKEND") == "" {
		_ = os.Setenv("TDX_TOKEN_BACKEND", "yaml")
	}
	os.Exit(m.Run())
}

// setTokenForTest writes a token directly to the credentials store.
func setTokenForTest(t *testing.T, profile, token string) {
	t.Helper()
	p, err := config.ResolvePaths()
	require.NoError(t, err)
	store := config.NewCredentialsStore(p)
	require.NoError(t, store.SetToken(profile, token))
}
