package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// TestNewServer_RegistersAllTools verifies that NewServer registers the expected
// number of tools without panicking, and that each tool is reachable via the
// MCP protocol using an in-memory transport pair.
func TestNewServer_RegistersAllTools(t *testing.T) {
	// The SDK validates struct tags at AddTool time, so we must supply a real
	// Services value (backed by a stub HTTP server) rather than a zero-value one.
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(stub.Close)

	svcs := mcpHarness(t, stub.URL)
	srv := NewServer("test", svcs)
	require.NotNil(t, srv)

	// Connect a client via an in-memory transport so we can call tools/list.
	ctx := context.Background()
	clientTransport, serverTransport := sdkmcp.NewInMemoryTransports()

	ss, err := srv.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ss.Close() })

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "0"}, nil)
	cs, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = cs.Close()
		_ = ss.Wait()
	})

	result, err := cs.ListTools(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Collect tool names for a helpful failure message.
	names := make([]string, len(result.Tools))
	for i, tool := range result.Tools {
		names[i] = tool.Name
	}

	const wantCount = 28
	require.Len(t, result.Tools, wantCount,
		"expected %d tools, got %d: %v", wantCount, len(result.Tools), names)
}
