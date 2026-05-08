package ticket

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

func TestRunTicketFeedRendersAllEntries(t *testing.T) {
	stub := &stubTicketsvc{feed: []domain.TicketFeedEntry{
		{ID: 1, AuthorName: "Alice", CreatedAt: time.Date(2026, 5, 8, 14, 23, 0, 0, time.UTC), Body: "first comment", EventKind: "comment"},
		{ID: 2, AuthorName: "Bob", CreatedAt: time.Date(2026, 5, 8, 9, 12, 0, 0, time.UTC), Body: "status flipped", EventKind: "statusChange"},
	}}
	var buf bytes.Buffer
	if err := runTicketFeed(context.Background(), &buf, stub, "default", 31, 12345, 0, false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"Alice", "Bob", "comment", "statusChange", "first comment", "status flipped"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q\n%s", want, out)
		}
	}
}

func TestRunTicketFeedRespectsLimit(t *testing.T) {
	stub := &stubTicketsvc{feed: []domain.TicketFeedEntry{
		{ID: 1, AuthorName: "A", Body: "first", EventKind: "comment"},
		{ID: 2, AuthorName: "B", Body: "second", EventKind: "comment"},
		{ID: 3, AuthorName: "C", Body: "third", EventKind: "comment"},
	}}
	var buf bytes.Buffer
	_ = runTicketFeed(context.Background(), &buf, stub, "default", 31, 1, 2, false)
	if !strings.Contains(buf.String(), "first") || !strings.Contains(buf.String(), "second") {
		t.Errorf("first two should be present: %s", buf.String())
	}
	if strings.Contains(buf.String(), "third") {
		t.Errorf("third should be omitted with --limit 2: %s", buf.String())
	}
}

func TestRunTicketFeedEmpty(t *testing.T) {
	stub := &stubTicketsvc{feed: nil}
	var buf bytes.Buffer
	if err := runTicketFeed(context.Background(), &buf, stub, "default", 31, 1, 0, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "no feed entries") {
		t.Errorf("empty: %s", buf.String())
	}
}

func TestRunTicketFeedJSON(t *testing.T) {
	stub := &stubTicketsvc{feed: []domain.TicketFeedEntry{
		{ID: 1, AuthorName: "Alice", Body: "msg", EventKind: "comment", CreatedAt: time.Date(2026, 5, 8, 14, 0, 0, 0, time.UTC)},
	}}
	var buf bytes.Buffer
	if err := runTicketFeed(context.Background(), &buf, stub, "default", 31, 1, 0, true); err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["schema"] != "tdx.v1.ticketFeed" {
		t.Fatalf("schema: %v", got["schema"])
	}
	entries, _ := got["entries"].([]interface{})
	if len(entries) != 1 {
		t.Fatalf("entries: %v", entries)
	}
}
