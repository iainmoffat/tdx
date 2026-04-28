package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestListDraftsArgs_Schema(t *testing.T) {
	// Smoke test: ensure the args struct serializes the expected JSON keys.
	var a listDraftsArgs
	a.Profile = "work"
	a.Dirty = true
	data, _ := json.Marshal(a)
	s := string(data)
	if !strings.Contains(s, `"profile":"work"`) || !strings.Contains(s, `"dirty":true`) {
		t.Errorf("unexpected serialization: %s", s)
	}
}

// Smoke test: getDraftArgs requires WeekStart, profile and name optional.
func TestGetDraftArgs_Required(t *testing.T) {
	var a getDraftArgs
	a.WeekStart = "2026-05-04"
	data, _ := json.Marshal(a)
	if !strings.Contains(string(data), `"weekStart":"2026-05-04"`) {
		t.Errorf("got %s", string(data))
	}
}

func TestPullDraft_RequiresConfirm(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(stub.Close)

	svcs := mcpHarness(t, stub.URL)
	handler := pullDraftHandler(svcs)
	res, _, err := handler(context.Background(), &sdkmcp.CallToolRequest{}, pullDraftArgs{WeekStart: "2026-05-04", Confirm: false})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Errorf("expected error result for confirm=false, got non-error result")
	}
}

func TestCreateDraft_ConfirmGate(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(stub.Close)

	svcs := mcpHarness(t, stub.URL)
	handler := createDraftHandler(svcs)
	res, _, err := handler(context.Background(), &sdkmcp.CallToolRequest{}, createDraftArgs{WeekStart: "2026-05-04", Confirm: false})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Errorf("expected error result for confirm=false")
	}
}

func TestRefreshDraft_RequiresConfirm(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(stub.Close)

	svcs := mcpHarness(t, stub.URL)
	handler := refreshDraftHandler(svcs)
	res, _, err := handler(context.Background(), &sdkmcp.CallToolRequest{}, refreshDraftArgs{
		WeekStart: "2026-05-03",
		Strategy:  "abort",
		Confirm:   false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Errorf("expected error result for confirm=false, got non-error result")
	}
}

func TestRefreshDraft_RejectsUnknownStrategy(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(stub.Close)

	svcs := mcpHarness(t, stub.URL)
	handler := refreshDraftHandler(svcs)
	res, _, err := handler(context.Background(), &sdkmcp.CallToolRequest{}, refreshDraftArgs{
		WeekStart: "2026-05-03",
		Strategy:  "merge",
		Confirm:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Errorf("expected error result for unknown strategy, got non-error result")
	}
}
