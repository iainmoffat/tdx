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

func TestListTasks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/TDWebApi/api/31/tickets/100/tasks" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[
			{"ID": 1, "TicketID": 100, "Title": "  Step 1  ", "PercentComplete": 50, "EstimatedMinutes": 60, "ActualMinutes": 30, "IsActive": true, "ResponsibleUid": "uid-a", "ResponsibleFullName": "Alice", "Order": 1},
			{"ID": 2, "TicketID": 100, "Title": "Step 2", "PercentComplete": 0, "IsActive": true, "Order": 2}
		]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	tasks, err := svc.ListTasks(context.Background(), prof, 31, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("want 2, got %d", len(tasks))
	}
	if tasks[0].Title != "Step 1" {
		t.Errorf("title not trimmed: %q", tasks[0].Title)
	}
	if tasks[0].PercentComplete != 50 {
		t.Errorf("percent: %d", tasks[0].PercentComplete)
	}
	if tasks[0].ResponsibleName != "Alice" {
		t.Errorf("responsible: %q", tasks[0].ResponsibleName)
	}
}

func TestGetTask(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/TDWebApi/api/31/tickets/100/tasks/5" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"ID": 5, "TicketID": 100, "Title": "Investigate",
			"Description": "find the issue", "PercentComplete": 75,
			"EstimatedMinutes": 240, "ActualMinutes": 90,
			"IsActive": true, "Order": 3,
			"CreatedDate": "2026-05-01T10:00:00Z",
			"CompletedDate": "0001-01-01T00:00:00",
			"ResponsibleUid": "", "ResponsibleGroupID": 100, "ResponsibleGroupName": "Linux Team"
		}`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	got, err := svc.GetTask(context.Background(), prof, 31, 100, 5)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 5 || got.PercentComplete != 75 {
		t.Errorf("decode: %+v", got)
	}
	if !got.CompletedDate.IsZero() {
		t.Errorf("CompletedDate should be zero for sentinel input: %v", got.CompletedDate)
	}
	if got.ResponsibleGroupName != "Linux Team" {
		t.Errorf("group name: %q", got.ResponsibleGroupName)
	}
}

func TestGetTaskFeed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/TDWebApi/api/31/tickets/100/tasks/5/feed" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[
			{"ID": 200, "CreatedUid": "uid-a", "CreatedFullName": "Alice", "CreatedDate": "2026-05-01T10:00:00Z", "Body": "halfway", "UpdateType": 1}
		]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	entries, err := svc.GetTaskFeed(context.Background(), prof, 31, 100, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].EventKind != "comment" {
		t.Errorf("decode: %+v", entries)
	}
}

func TestUpdateTaskFeedSendsPercentAndHours(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		capturedBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"ID": 555}`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	pct := 50
	id, err := svc.UpdateTaskFeed(context.Background(), prof, 31, 100, 5, "halfway", &pct, 0.5, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if id != 555 {
		t.Errorf("feed id: %d", id)
	}
	var sent map[string]interface{}
	if err := json.Unmarshal(capturedBody, &sent); err != nil {
		t.Fatal(err)
	}
	if sent["Comments"] != "halfway" {
		t.Errorf("Comments: %v", sent["Comments"])
	}
	if pc, _ := sent["PercentComplete"].(float64); pc != 50 {
		t.Errorf("PercentComplete: %v", sent["PercentComplete"])
	}
	if hw, _ := sent["HoursWorked"].(float64); hw != 0.5 {
		t.Errorf("HoursWorked: %v", sent["HoursWorked"])
	}
}

func TestUpdateTaskFeedNilPercentOmits(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"ID": 1}`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	_, _ = svc.UpdateTaskFeed(context.Background(), prof, 31, 100, 5, "msg", nil, 0, false, nil)
	if strings.Contains(string(capturedBody), "PercentComplete") {
		t.Errorf("nil percent should be omitted; body: %s", capturedBody)
	}
}

func TestUpdateTaskFeedZeroPercentSent(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"ID": 1}`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	zero := 0
	_, _ = svc.UpdateTaskFeed(context.Background(), prof, 31, 100, 5, "reset", &zero, 0, false, nil)
	if !strings.Contains(string(capturedBody), `"PercentComplete":0`) {
		t.Errorf("explicit 0 should be sent; body: %s", capturedBody)
	}
}
