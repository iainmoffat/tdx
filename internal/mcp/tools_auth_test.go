package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ipm/tdx/internal/config"
	"github.com/ipm/tdx/internal/domain"
	"github.com/ipm/tdx/internal/svc/authsvc"
	"github.com/ipm/tdx/internal/svc/timesvc"
	"github.com/ipm/tdx/internal/svc/tmplsvc"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// mcpHarness creates a Services struct backed by a test HTTP server.
func mcpHarness(t *testing.T, tenantURL string) Services {
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

	auth := authsvc.New(paths)
	tsvc := timesvc.New(paths)
	tmsvc := tmplsvc.New(paths, tsvc)

	return Services{
		Auth:     auth,
		Time:     tsvc,
		Template: tmsvc,
		Profile:  "default",
	}
}

func TestWhoami_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/TDWebApi/api/auth/getuser" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"UID":"uid-abc","FullName":"Test User","PrimaryEmail":"test@ufl.edu","ReferenceID":1,"AlternateEmail":""}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svcs := mcpHarness(t, srv.URL)
	handler := whoamiHandler(svcs)
	result, _, err := handler(context.Background(), nil, whoamiArgs{})
	require.NoError(t, err)
	require.False(t, result.IsError)

	require.Len(t, result.Content, 1)
	text := extractText(t, result)
	var resp whoamiResponse
	require.NoError(t, json.Unmarshal([]byte(text), &resp))
	require.Equal(t, "uid-abc", resp.UID)
	require.Equal(t, "Test User", resp.FullName)
}

func TestWhoami_AuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`"Invalid token"`))
	}))
	defer srv.Close()

	svcs := mcpHarness(t, srv.URL)
	handler := whoamiHandler(svcs)
	result, _, err := handler(context.Background(), nil, whoamiArgs{})
	require.NoError(t, err) // handler errors are in the result, not returned
	require.True(t, result.IsError)
}

// extractText gets the text string from the first TextContent in a result.
func extractText(t *testing.T, result *sdkmcp.CallToolResult) string {
	t.Helper()
	require.NotEmpty(t, result.Content)
	// *sdkmcp.TextContent implements Content and marshals as {"type":"text","text":"..."}
	tc, ok := result.Content[0].(*sdkmcp.TextContent)
	if ok {
		return tc.Text
	}
	// Fallback: marshal/unmarshal to extract text field.
	data, err := json.Marshal(result.Content[0])
	require.NoError(t, err)
	var wire struct {
		Text string `json:"text"`
	}
	require.NoError(t, json.Unmarshal(data, &wire))
	return wire.Text
}
