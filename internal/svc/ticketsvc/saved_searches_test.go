package ticketsvc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListSavedSearches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/TDWebApi/api/31/tickets/searches" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[
			{"ID": 100, "Name": "  My Open  ", "CreatedUID": "uid-a", "CreatedFullName": "Alice"},
			{"ID": 101, "Name": "Closed This Week", "CreatedUID": "uid-b", "CreatedFullName": "Bob"}
		]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	got, err := svc.ListSavedSearches(context.Background(), prof, 31)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	if got[0].Name != "My Open" {
		t.Errorf("name not trimmed: %q", got[0].Name)
	}
	if got[0].OwnerName != "Alice" {
		t.Errorf("owner: %+v", got[0])
	}
}

func TestRunSavedSearchSendsLimit(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/TDWebApi/api/31/tickets/searches/100/results" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		capturedBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"Data":[{"ID":1,"Title":"T1"},{"ID":2,"Title":"T2"}],"TotalCount":2,"CurrentPageIndex":0,"PageSize":25}`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	got, err := svc.RunSavedSearch(context.Background(), prof, 31, 100, 25)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	if got[0].IsFull {
		t.Error("saved-search results must be partial (IsFull=false)")
	}
	var sent map[string]interface{}
	if err := json.Unmarshal(capturedBody, &sent); err != nil {
		t.Fatal(err)
	}
	page, _ := sent["Page"].(map[string]interface{})
	if page == nil {
		t.Fatalf("Page object missing; body: %s", capturedBody)
	}
	if ps, _ := page["PageSize"].(float64); ps != 25 {
		t.Errorf("Page.PageSize: %v", page["PageSize"])
	}
}

func TestRunSavedSearchDefaultsLimit(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"Data":[],"TotalCount":0,"CurrentPageIndex":0,"PageSize":50}`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	_, _ = svc.RunSavedSearch(context.Background(), prof, 31, 100, 0)
	var sent map[string]interface{}
	_ = json.Unmarshal(capturedBody, &sent)
	page, _ := sent["Page"].(map[string]interface{})
	if page == nil {
		t.Fatalf("Page object missing; body: %s", capturedBody)
	}
	if ps, _ := page["PageSize"].(float64); ps != 50 {
		t.Errorf("default Page.PageSize should be 50, got %v", page["PageSize"])
	}
}

func TestResolveSavedSearchByNameSingleMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"ID": 100, "Name": "My Open", "OwnerUid": "a"},
			{"ID": 101, "Name": "Closed This Week", "OwnerUid": "b"}
		]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	got, err := svc.ResolveSavedSearchByName(context.Background(), prof, 31, "my open")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 100 {
		t.Fatalf("want id=100, got %d", got.ID)
	}
}

func TestResolveSavedSearchByNameNoMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"ID": 1, "Name": "Only"}]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	_, err := svc.ResolveSavedSearchByName(context.Background(), prof, 31, "nonsense")
	if err == nil || !strings.Contains(err.Error(), "no saved search matches") {
		t.Fatalf("want no-match error, got %v", err)
	}
}

func TestResolveSavedSearchByNameAmbiguous(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"ID": 1, "Name": "Open"},
			{"ID": 2, "Name": "Open"}
		]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	_, err := svc.ResolveSavedSearchByName(context.Background(), prof, 31, "Open")
	if err == nil || !strings.Contains(err.Error(), "multiple saved searches match") {
		t.Fatalf("want ambiguous error, got %v", err)
	}
}
