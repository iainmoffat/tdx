package auth

import (
	"testing"

	"github.com/ipm/tdx/internal/config"
	"github.com/stretchr/testify/require"
)

// setTokenForTest writes a token directly to the credentials store.
func setTokenForTest(t *testing.T, profile, token string) {
	t.Helper()
	p, err := config.ResolvePaths()
	require.NoError(t, err)
	store := config.NewCredentialsStore(p)
	require.NoError(t, store.SetToken(profile, token))
}
