package ticketsvc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListAppsFiltersToTicketApps(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/TDWebApi/api/applications" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"AppID": 31, "Name": "Service Desk", "Description": "Help desk", "Active": true, "Type": "TDNext", "AppClass": "TDTickets"},
			{"AppID": 50, "Name": "Knowledge", "Description": "KB", "Active": true, "Type": "TDNext", "AppClass": "TDKnowledgeBase"},
			{"AppID": 71, "Name": "Project Tickets", "Description": "PM", "Active": true, "Type": "TDNext", "AppClass": "TDTicketsProjects"}
		]`))
	}))
	defer srv.Close()

	svc, prof := harness(t, srv.URL)
	apps, err := svc.ListApps(context.Background(), prof)
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 2 {
		t.Fatalf("want 2 ticket apps, got %d: %+v", len(apps), apps)
	}
	if apps[0].ID != 31 || apps[1].ID != 71 {
		t.Fatalf("filtered IDs wrong: %+v", apps)
	}
}

func TestIsTicketApp(t *testing.T) {
	cases := []struct {
		class string
		want  bool
	}{
		{"TDTickets", true},
		{"tdtickets", true},
		{"TDTicketsProjects", true},
		{"TDKnowledgeBase", false},
		{"", false},
	}
	for _, c := range cases {
		got := isTicketApp(wireApp{AppClass: c.class})
		if got != c.want {
			t.Errorf("isTicketApp(%q) = %v, want %v", c.class, got, c.want)
		}
	}
}
