package ticketsvc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListGroups(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/TDWebApi/api/groups/search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		_, _ = w.Write([]byte(`[
			{"ID": 100, "Name": "  Linux Team  ", "IsActive": true, "Description": "ICT Linux", "ExternalID": ""},
			{"ID": 101, "Name": "Network Ops", "IsActive": true},
			{"ID": 102, "Name": "Archived Team", "IsActive": false}
		]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	groups, err := svc.ListGroups(context.Background(), prof)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 3 {
		t.Fatalf("want 3, got %d", len(groups))
	}
	if groups[0].Name != "Linux Team" {
		t.Errorf("name not trimmed: %q", groups[0].Name)
	}
	if !groups[0].Active || groups[2].Active {
		t.Errorf("Active mapping wrong: %+v", groups)
	}
}

func TestResolveGroupByNameSingleMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[
			{"ID": 100, "Name": "Linux Team", "IsActive": true},
			{"ID": 101, "Name": "Network Ops", "IsActive": true}
		]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	got, err := svc.ResolveGroupByName(context.Background(), prof, "linux team")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 100 {
		t.Fatalf("want id=100, got %d", got.ID)
	}
}

func TestResolveGroupByNameNoMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"ID": 1, "Name": "Only", "IsActive": true}]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	_, err := svc.ResolveGroupByName(context.Background(), prof, "nonsense")
	if err == nil || !strings.Contains(err.Error(), "no ticket group matches") {
		t.Fatalf("want no-match error, got %v", err)
	}
}

func TestResolveGroupByNameAmbiguous(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[
			{"ID": 1, "Name": "Network", "IsActive": true},
			{"ID": 2, "Name": "Network", "IsActive": true}
		]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	_, err := svc.ResolveGroupByName(context.Background(), prof, "Network")
	if err == nil || !strings.Contains(err.Error(), "multiple ticket groups match") {
		t.Fatalf("want ambiguous error, got %v", err)
	}
	if !strings.Contains(err.Error(), "1 (Network)") || !strings.Contains(err.Error(), "2 (Network)") {
		t.Errorf("error should list candidates: %v", err)
	}
}
