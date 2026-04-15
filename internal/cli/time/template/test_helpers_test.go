package template

import (
	"path/filepath"
	"testing"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/tmplsvc"
	"github.com/stretchr/testify/require"
)

// seedTemplateDir sets TDX_CONFIG_HOME to a temp dir with a profile and token,
// returning the dir so the caller can pre-populate templates.
func seedTemplateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

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
	cs := config.NewCredentialsStore(paths)
	require.NoError(t, cs.SetToken("default", "test-token"))
	return dir
}

// writeTestTemplate saves a template to disk in the given config home dir.
func writeTestTemplate(t *testing.T, dir string, tmpl domain.Template) {
	t.Helper()
	paths := config.Paths{TemplatesDir: filepath.Join(dir, "templates")}
	store := tmplsvc.NewStore(paths)
	require.NoError(t, store.Save(tmpl))
}
