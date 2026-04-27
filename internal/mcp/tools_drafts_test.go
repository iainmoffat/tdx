package mcp

import (
	"encoding/json"
	"strings"
	"testing"
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
