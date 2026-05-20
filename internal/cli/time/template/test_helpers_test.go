package template

import (
	"os"
	"testing"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/tmplsvc"
	"github.com/stretchr/testify/require"
)

// TestMain defaults TDX_TOKEN_BACKEND=yaml for this package so tests never
// touch the dev's real OS keychain.
func TestMain(m *testing.M) {
	if os.Getenv("TDX_TOKEN_BACKEND") == "" {
		os.Setenv("TDX_TOKEN_BACKEND", "yaml")
	}
	os.Exit(m.Run())
}

// seedTemplateDir sets TDX_CONFIG_HOME to a temp dir with a profile and token,
// returning the dir so the caller can pre-populate templates.
func seedTemplateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	paths, err := config.ResolvePaths()
	require.NoError(t, err)
	ps := config.NewProfileStore(paths)
	require.NoError(t, ps.AddProfile(domain.Profile{
		Name:          "default",
		TenantBaseURL: "http://localhost/",
	}))
	cs := config.NewCredentialsStore(paths)
	require.NoError(t, cs.SetToken("default", "test-token"))
	return dir
}

// writeTestTemplate saves a template to disk under the "default" profile.
func writeTestTemplate(t *testing.T, _ string, tmpl domain.Template) {
	t.Helper()
	paths, err := config.ResolvePaths()
	require.NoError(t, err)
	store := tmplsvc.NewStore(paths)
	require.NoError(t, store.Save("default", tmpl))
}
