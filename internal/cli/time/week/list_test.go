package week

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestListJSON_Schema(t *testing.T) {
	var buf bytes.Buffer
	if err := writeListJSON(&buf, []weekDraftListItem{}); err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["schema"] != "tdx.v1.weekDraftList" {
		t.Errorf("schema = %v", out["schema"])
	}
}

func TestListText_Empty(t *testing.T) {
	var buf bytes.Buffer
	writeListText(&buf, []weekDraftListItem{})
	if !strings.Contains(buf.String(), "No drafts found.") {
		t.Errorf("expected empty message, got %q", buf.String())
	}
}

func TestListText_RendersRows(t *testing.T) {
	items := []weekDraftListItem{
		{
			WeekStart:  "2026-05-03",
			Name:       "default",
			SyncState:  "clean",
			TotalHours: 40.0,
			PulledAt:   time.Date(2026, 4, 27, 9, 0, 0, 0, time.UTC).Format(time.RFC3339),
		},
	}
	var buf bytes.Buffer
	writeListText(&buf, items)
	out := buf.String()
	for _, want := range []string{"2026-05-03", "default", "clean", "40.0"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in: %q", want, out)
		}
	}
}

func TestFormatRFC3339OrEmpty(t *testing.T) {
	if got := formatRFC3339OrEmpty(time.Time{}); got != "" {
		t.Errorf("zero: got %q, want \"\"", got)
	}
	if got := formatRFC3339OrEmpty(time.Date(2026, 4, 27, 9, 0, 0, 0, time.UTC)); got == "" {
		t.Errorf("non-zero: got empty")
	}
}

func TestListText_GroupsByDate(t *testing.T) {
	items := []weekDraftListItem{
		{WeekStart: "2026-05-04", Name: "default", SyncState: "dirty", TotalHours: 18},
		{WeekStart: "2026-05-04", Name: "pristine", SyncState: "clean", TotalHours: 20},
		{WeekStart: "2026-04-12", Name: "default", SyncState: "clean", TotalHours: 20},
	}
	var buf bytes.Buffer
	writeListText(&buf, items)
	out := buf.String()
	if !strings.Contains(out, "2026-05-04") {
		t.Errorf("first date missing: %q", out)
	}
	lines := strings.Split(out, "\n")
	var pristineLine string
	for _, l := range lines {
		if strings.Contains(l, "pristine") {
			pristineLine = l
		}
	}
	if strings.HasPrefix(pristineLine, "2026-05-04") {
		t.Errorf("pristine line should have blank date column, got %q", pristineLine)
	}
}

func TestList_FilterArchived(t *testing.T) {
	items := []weekDraftListItem{
		{WeekStart: "2026-04-12", Name: "default", SyncState: "clean", Archived: false},
		{WeekStart: "2026-04-05", Name: "default", SyncState: "clean", Archived: true},
	}
	visible := filterArchived(items, false)
	if len(visible) != 1 || visible[0].WeekStart != "2026-04-12" {
		t.Errorf("filterArchived(false) wrong: %+v", visible)
	}
	all := filterArchived(items, true)
	if len(all) != 2 {
		t.Errorf("filterArchived(true) wrong: %+v", all)
	}
}
