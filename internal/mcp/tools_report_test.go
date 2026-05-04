package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestGetTimeStatusReport_RejectsMissingSelector(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(stub.Close)

	svcs := mcpHarness(t, stub.URL)
	handler := getTimeStatusReportHandler(svcs)
	res, _, err := handler(context.Background(), &sdkmcp.CallToolRequest{}, getTimeStatusReportArgs{
		Week: "2026-04-14",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Errorf("expected error result for missing selector")
	}
}

func TestGetTimeStatusReport_RejectsBothFormats(t *testing.T) {
	// Sanity: validation rules from the CLI flow apply via RunForMCP.
	// Note: --json/--csv/--xlsx flags don't exist in the MCP args, so
	// this test instead checks that selector exclusivity still fires.
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(stub.Close)

	svcs := mcpHarness(t, stub.URL)
	handler := getTimeStatusReportHandler(svcs)
	res, _, err := handler(context.Background(), &sdkmcp.CallToolRequest{}, getTimeStatusReportArgs{
		Week:     "2026-04-14",
		UserUIDs: []string{"u1"},
		Managers: []string{"me"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Errorf("expected error result for multiple selectors")
	}
}
