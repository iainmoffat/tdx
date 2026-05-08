package ticketsvc

import (
	"path/filepath"
	"testing"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

// harness returns a ticketsvc.Service rooted at a temp dir with one profile
// and one stored token. Mirrors peoplesvc's harness shape.
func harness(t *testing.T, tenantURL string) (*Service, string) {
	t.Helper()
	dir := t.TempDir()
	paths := config.Paths{
		Root:            dir,
		ConfigFile:      filepath.Join(dir, "config.yaml"),
		CredentialsFile: filepath.Join(dir, "credentials.yaml"),
		TemplatesDir:    filepath.Join(dir, "templates"),
	}
	ps := config.NewProfileStore(paths)
	require.NoError(t, ps.AddProfile(domain.Profile{
		Name:          "default",
		TenantBaseURL: tenantURL,
	}))
	cs := config.NewCredentialsStore(paths)
	require.NoError(t, cs.SetToken("default", "good-token"))

	return New(paths), "default"
}
