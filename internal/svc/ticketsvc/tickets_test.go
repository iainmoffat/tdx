package ticketsvc

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
)

func TestGetTicketFull(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/TDWebApi/api/31/tickets/12345" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"ID": 12345, "AppID": 31, "Title": "Test", "Description": "body",
			"StatusID": 2, "StatusName": "In Progress",
			"TypeID": 10, "TypeName": "Incident",
			"ResponsibleUid": "uid-a", "ResponsibleFullName": "Alice",
			"RequestorUid": "uid-b", "RequestorName": "Bob",
			"CreatedDate": "2026-05-01T10:00:00Z",
			"ModifiedDate": "2026-05-08T14:30:00Z",
			"EstimatedMinutes": 240, "ActualMinutes": 90,
			"Tags": ["urgent", "vendor"]
		}`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	got, err := svc.GetTicket(context.Background(), prof, 31, 12345)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsFull {
		t.Error("IsFull should be true on GetTicket")
	}
	if got.Title != "Test" || got.StatusName != "In Progress" {
		t.Errorf("decode wrong: %+v", got)
	}
	if got.CreatedDate.IsZero() {
		t.Error("CreatedDate should parse")
	}
	if len(got.Tags) != 2 {
		t.Errorf("tags: %+v", got.Tags)
	}
}

func TestSearchTicketsDefaultsLimit(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	_, err := svc.SearchTickets(context.Background(), prof, domain.TicketSearchFilter{AppID: 31, Limit: 0})
	if err != nil {
		t.Fatal(err)
	}
	var sent map[string]interface{}
	if err := json.Unmarshal(capturedBody, &sent); err != nil {
		t.Fatal(err)
	}
	if mr, _ := sent["MaxResults"].(float64); mr != 50 {
		t.Errorf("default MaxResults should be 50, got %v", sent["MaxResults"])
	}
	if open, ok := sent["IsOpen"].(bool); !ok || !open {
		t.Errorf("IsOpen should be true (open-only default), got %v", sent["IsOpen"])
	}
}

func TestSearchTicketsIncludeClosedOmitsIsOpen(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	_, _ = svc.SearchTickets(context.Background(), prof, domain.TicketSearchFilter{AppID: 31, IncludeClosed: true})
	var sent map[string]interface{}
	_ = json.Unmarshal(capturedBody, &sent)
	if _, present := sent["IsOpen"]; present {
		t.Errorf("IsOpen should be omitted when IncludeClosed=true; body: %s", capturedBody)
	}
}

func TestSearchTicketsMapsAssigneeFilter(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	_, _ = svc.SearchTickets(context.Background(), prof, domain.TicketSearchFilter{
		AppID: 31, AssigneeUIDs: []string{"uid-a", "uid-b"},
	})
	if !strings.Contains(string(capturedBody), `"ResponsibilityUids":["uid-a","uid-b"]`) {
		t.Errorf("ResponsibilityUids not mapped from AssigneeUIDs; body: %s", capturedBody)
	}
}

func TestPatchTicketSendsOps(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		capturedBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"ID": 99, "StatusID": 5, "StatusName": "Closed"}`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	got, err := svc.PatchTicket(context.Background(), prof, 31, 99, []PatchOp{
		{Op: "replace", Path: "/StatusID", Value: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.StatusName != "Closed" {
		t.Errorf("decode after patch: %+v", got)
	}
	if !got.IsFull {
		t.Error("IsFull should be true")
	}
	if !bytes.Contains(capturedBody, []byte(`"op":"replace"`)) {
		t.Errorf("patch body missing op: %s", capturedBody)
	}
	if !bytes.Contains(capturedBody, []byte(`"path":"/StatusID"`)) {
		t.Errorf("patch body missing path: %s", capturedBody)
	}
}

func TestParseTDTimeMultipleFormats(t *testing.T) {
	cases := []string{
		"2026-05-01T10:00:00Z",
		"2026-05-01T10:00:00.123Z",
		"2026-05-01T10:00:00-04:00",
		"2026-05-01T10:00:00.123-04:00",
	}
	for _, s := range cases {
		got := parseTDTime(s)
		if got.IsZero() {
			t.Errorf("parseTDTime(%q) returned zero time", s)
		}
	}
	if !parseTDTime("").IsZero() {
		t.Error("empty should return zero")
	}
	if !parseTDTime("not a date").IsZero() {
		t.Error("garbage should return zero")
	}
}
