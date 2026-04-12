package timesvc

import (
	"path/filepath"
	"testing"

	"github.com/ipm/tdx/internal/config"
	"github.com/ipm/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

// harness returns a timesvc.Service rooted at a temp dir with one profile
// and one stored token. Subsequent tests seed their own HTTP servers and
// call svc methods, reusing this fixture.
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

func TestService_UnknownProfileReturnsNotFound(t *testing.T) {
	svc, _ := harness(t, "http://localhost/")
	_, err := svc.clientFor("nope")
	require.ErrorIs(t, err, domain.ErrProfileNotFound)
}

func TestService_MissingTokenReturnsNoCredentials(t *testing.T) {
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
		TenantBaseURL: "http://localhost/",
	}))
	svc := New(paths)
	_, err := svc.clientFor("default")
	require.ErrorIs(t, err, domain.ErrNoCredentials)
}
