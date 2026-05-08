package ticketsvc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetFeedDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/TDWebApi/api/31/tickets/12345/feed" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[
			{"ID": 100, "CreatedUid": "uid-a", "CreatedFullName": "Alice", "CreatedDate": "2026-05-01T10:00:00Z", "Body": "first comment", "IsPrivate": false, "UpdateType": 1},
			{"ID": 101, "CreatedUid": "uid-b", "CreatedFullName": "Bob", "CreatedDate": "2026-05-02T11:00:00Z", "Body": "status changed", "IsPrivate": false, "UpdateType": 2}
		]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	entries, err := svc.GetFeed(context.Background(), prof, 31, 12345)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2, got %d", len(entries))
	}
	if entries[0].AuthorName != "Alice" || entries[0].EventKind != "comment" {
		t.Errorf("first entry decode: %+v", entries[0])
	}
	if entries[1].EventKind != "statusChange" {
		t.Errorf("second entry kind: %+v", entries[1])
	}
	if entries[0].CreatedAt.IsZero() {
		t.Error("CreatedAt should parse")
	}
}

func TestAddFeedSendsCommentBody(t *testing.T) {
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
	id, err := svc.AddFeed(context.Background(), prof, 31, 12345, "test message", true, []string{"notify-uid-1"})
	if err != nil {
		t.Fatal(err)
	}
	if id != 555 {
		t.Errorf("want id 555, got %d", id)
	}
	var sent map[string]interface{}
	if err := json.Unmarshal(capturedBody, &sent); err != nil {
		t.Fatal(err)
	}
	if sent["Comments"] != "test message" {
		t.Errorf("Comments: %v", sent["Comments"])
	}
	if sent["IsPrivate"] != true {
		t.Errorf("IsPrivate: %v", sent["IsPrivate"])
	}
	notify, ok := sent["Notify"].([]interface{})
	if !ok || len(notify) != 1 || notify[0] != "notify-uid-1" {
		t.Errorf("Notify: %v", sent["Notify"])
	}
}

func TestAddFeedOmitsEmptyNotify(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"ID": 1}`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	_, _ = svc.AddFeed(context.Background(), prof, 31, 12345, "msg", false, nil)
	var sent map[string]interface{}
	_ = json.Unmarshal(capturedBody, &sent)
	if _, present := sent["Notify"]; present {
		t.Errorf("Notify should be omitted when nil; body: %s", capturedBody)
	}
}

func TestClassifyFeedKind(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{1, "comment"}, {2, "statusChange"}, {3, "attachment"}, {4, "task"},
		{0, "event"}, {99, "event"},
	}
	for _, c := range cases {
		got := classifyFeedKind(c.in)
		if got != c.want {
			t.Errorf("classifyFeedKind(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
