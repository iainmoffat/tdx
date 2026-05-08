package ticketsvc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListStatusesDecodesAndComputesIsClosed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/TDWebApi/api/31/tickets/statuses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[
			{"ID": 1, "Name": "New", "IsActive": true, "Order": 1.0, "IsDefault": true, "StatusClass": 1},
			{"ID": 2, "Name": "In Progress", "IsActive": true, "Order": 2.0, "IsDefault": false, "StatusClass": 2},
			{"ID": 3, "Name": "  Closed  ", "IsActive": true, "Order": 3.0, "IsDefault": false, "StatusClass": 6},
			{"ID": 4, "Name": "Cancelled", "IsActive": true, "Order": 4.0, "IsDefault": false, "StatusClass": 6}
		]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	got, err := svc.ListStatuses(context.Background(), prof, 31)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("want 4, got %d", len(got))
	}
	if got[2].Name != "Closed" {
		t.Errorf("name not trimmed: %q", got[2].Name)
	}
	if !got[2].IsClosed {
		t.Errorf("StatusClass=6 should be IsClosed: %+v", got[2])
	}
	if got[0].IsClosed {
		t.Errorf("StatusClass=1 should NOT be IsClosed: %+v", got[0])
	}
	if !got[0].IsDefault {
		t.Errorf("first row should be IsDefault: %+v", got[0])
	}
}

func TestResolveStatusByNameSingleMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"ID": 1, "Name": "New", "StatusClass": 1, "IsActive": true, "Order": 1, "IsDefault": false},
			{"ID": 2, "Name": "In Progress", "StatusClass": 2, "IsActive": true, "Order": 2, "IsDefault": false},
			{"ID": 3, "Name": "Closed", "StatusClass": 6, "IsActive": true, "Order": 3, "IsDefault": false}
		]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	got, err := svc.ResolveStatusByName(context.Background(), prof, 31, "in progress")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 2 {
		t.Fatalf("want id=2, got %d", got.ID)
	}
}

func TestResolveStatusByNameNoMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"ID": 1, "Name": "New", "StatusClass": 1, "IsActive": true, "Order": 1, "IsDefault": false}]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	_, err := svc.ResolveStatusByName(context.Background(), prof, 31, "nonsense")
	if err == nil || !strings.Contains(err.Error(), "no ticket status matches") {
		t.Fatalf("want no-match error, got %v", err)
	}
}

func TestResolveStatusByNameAmbiguous(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"ID": 1, "Name": "Closed", "StatusClass": 6, "IsActive": true, "Order": 1, "IsDefault": false},
			{"ID": 2, "Name": "Closed", "StatusClass": 6, "IsActive": true, "Order": 2, "IsDefault": false}
		]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	_, err := svc.ResolveStatusByName(context.Background(), prof, 31, "Closed")
	if err == nil || !strings.Contains(err.Error(), "multiple statuses match") {
		t.Fatalf("want ambiguous error, got %v", err)
	}
	if !strings.Contains(err.Error(), "1 (Closed)") || !strings.Contains(err.Error(), "2 (Closed)") {
		t.Errorf("error should list candidates: %v", err)
	}
}

func TestListTypes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/tickets/types") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("isActive") != "true" {
			t.Errorf("expected isActive=true query param, got %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[
			{"ID": 10, "Name": "Incident", "Description": "Issue", "IsActive": true},
			{"ID": 11, "Name": "  Service Request  ", "Description": "", "IsActive": true}
		]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	got, err := svc.ListTypes(context.Background(), prof, 31)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].Name != "Service Request" {
		t.Fatalf("decode wrong: %+v", got)
	}
}

func TestResolveTypeByNameSingleMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"ID": 10, "Name": "Incident", "IsActive": true}]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	got, err := svc.ResolveTypeByName(context.Background(), prof, 31, "Incident")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 10 {
		t.Fatalf("want id=10, got %d", got.ID)
	}
}
