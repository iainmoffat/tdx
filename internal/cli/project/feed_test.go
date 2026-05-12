package project

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

func TestFeed_Renders(t *testing.T) {
	stub := &stubProjectsvc{
		feed: []domain.ProjectFeedEntry{
			{
				ID: 1782210, CreatedByName: "Pat Manager",
				CreatedDate: time.Date(2026, 5, 7, 18, 35, 0, 0, time.UTC),
				UpdateType:  3, Body: "Changed Portfolio from A to B",
			},
			{
				ID: 1782180, CreatedByName: "Sample User",
				CreatedDate: time.Date(2026, 5, 6, 14, 12, 0, 0, time.UTC),
				UpdateType:  1, Body: "Backup config review complete.",
			},
		},
	}
	var buf bytes.Buffer
	if err := runProjectFeed(context.Background(), &buf, stub, "default", 259, 0, false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"1782210", "1782180", "Pat Manager", "Sample User", "system", "comment", "Changed Portfolio", "Backup config"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q\n%s", want, out)
		}
	}
}

func TestFeed_JSON(t *testing.T) {
	stub := &stubProjectsvc{
		feed: []domain.ProjectFeedEntry{
			{ID: 1782210, CreatedByUID: "uid-a", CreatedByName: "Pat Manager",
				CreatedDate: time.Date(2026, 5, 7, 18, 35, 0, 0, time.UTC),
				UpdateType:  3, Body: "Changed Portfolio"},
		},
	}
	var buf bytes.Buffer
	if err := runProjectFeed(context.Background(), &buf, stub, "default", 259, 0, true); err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["schema"] != "tdx.v1.projectFeed" {
		t.Fatalf("schema: %v", got["schema"])
	}
	if got["projectID"] != float64(259) {
		t.Fatalf("projectID: %v", got["projectID"])
	}
	entries, _ := got["entries"].([]interface{})
	if len(entries) != 1 {
		t.Fatalf("entries: %v", entries)
	}
}

func TestFeed_EmptyMessage(t *testing.T) {
	stub := &stubProjectsvc{feed: nil}
	var buf bytes.Buffer
	if err := runProjectFeed(context.Background(), &buf, stub, "default", 259, 0, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "no feed entries on project 259") {
		t.Errorf("empty: %s", buf.String())
	}
}

func TestFeed_RespectsLimit(t *testing.T) {
	stub := &stubProjectsvc{
		feed: []domain.ProjectFeedEntry{
			{ID: 1, Body: "first"},
			{ID: 2, Body: "second"},
			{ID: 3, Body: "third"},
		},
	}
	var buf bytes.Buffer
	if err := runProjectFeed(context.Background(), &buf, stub, "default", 259, 2, false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "first") || !strings.Contains(out, "second") {
		t.Errorf("first two should be present: %s", out)
	}
	if strings.Contains(out, "third") {
		t.Errorf("third should be omitted with limit=2: %s", out)
	}
}

func TestFeed_TruncatesBody(t *testing.T) {
	longBody := strings.Repeat("A", 100)
	stub := &stubProjectsvc{
		feed: []domain.ProjectFeedEntry{
			{ID: 1, Body: longBody, UpdateType: 1},
		},
	}
	var buf bytes.Buffer
	if err := runProjectFeed(context.Background(), &buf, stub, "default", 1, 0, false); err != nil {
		t.Fatal(err)
	}
	// Truncated body should be <= 80 chars + ellipsis; the full 100-char body should not appear
	if strings.Contains(buf.String(), longBody) {
		t.Error("body was not truncated")
	}
}
