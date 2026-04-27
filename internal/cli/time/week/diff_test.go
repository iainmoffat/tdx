package week

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

func TestRenderDiff_JSON(t *testing.T) {
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	diff := domain.ReconcileDiff{
		Actions: []domain.Action{
			{Kind: domain.ActionCreate, RowID: "row-01", Date: week.AddDate(0, 0, 1),
				Entry: domain.EntryInput{Minutes: 240}},
			{Kind: domain.ActionDelete, RowID: "row-01", Date: week.AddDate(0, 0, 2),
				DeleteEntryID: 98731},
		},
	}
	var buf bytes.Buffer
	if err := renderDiff(&buf, diff, true); err != nil {
		t.Fatal(err)
	}
	var resp weekDraftDiffResp
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Schema != "tdx.v1.weekDraftDiff" {
		t.Errorf("schema = %q", resp.Schema)
	}
	if resp.Summary.Adds != 1 || resp.Summary.Deletes != 1 {
		t.Errorf("summary = %+v", resp.Summary)
	}
}

func TestRenderDiff_TextSkipsMatches(t *testing.T) {
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	diff := domain.ReconcileDiff{
		Actions: []domain.Action{
			{Kind: domain.ActionUpdate, RowID: "row-01", Date: week.AddDate(0, 0, 1),
				ExistingID: 100, Patch: domain.EntryUpdate{}}, // patch fields nil → After=0
			{Kind: domain.ActionSkip, RowID: "row-01", Date: week.AddDate(0, 0, 2),
				ExistingID: 200, SkipReason: "noChange"},
		},
	}
	var buf bytes.Buffer
	if err := renderDiff(&buf, diff, false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "update") {
		t.Errorf("text missing update line: %q", out)
	}
	// Match-kind entries must not appear as individual action lines.
	// (The summary may still say "N matches".)
	if strings.Contains(out, "  match  ") {
		t.Errorf("text should not list match action lines: %q", out)
	}
	if !strings.Contains(out, "1 updates") {
		t.Errorf("summary missing: %q", out)
	}
	if !strings.Contains(out, "1 matches") {
		t.Errorf("summary missing matches: %q", out)
	}
}
