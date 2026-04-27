package week

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

func TestRenderPreview_JSON(t *testing.T) {
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	diff := domain.ReconcileDiff{
		Actions: []domain.Action{
			{Kind: domain.ActionCreate, RowID: "row-01", Date: week.AddDate(0, 0, 1),
				Entry: domain.EntryInput{Minutes: 240}},
			{Kind: domain.ActionDelete, RowID: "row-01", Date: week.AddDate(0, 0, 2),
				DeleteEntryID: 98731},
		},
		DiffHash: "abc123def456",
	}
	var buf bytes.Buffer
	if err := renderPreview(&buf, diff, 1, 0, 1, 0, true); err != nil {
		t.Fatal(err)
	}
	var resp weekDraftPreviewResp
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Schema != "tdx.v1.weekDraftPreview" {
		t.Errorf("schema = %q", resp.Schema)
	}
	if resp.ExpectedDiffHash != "abc123def456" {
		t.Errorf("hash = %q", resp.ExpectedDiffHash)
	}
	if resp.Creates != 1 || resp.Deletes != 1 {
		t.Errorf("counts wrong: %+v", resp)
	}
}

func TestRenderPreview_TextIncludesHash(t *testing.T) {
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	diff := domain.ReconcileDiff{
		Actions: []domain.Action{
			{Kind: domain.ActionCreate, RowID: "row-01", Date: week.AddDate(0, 0, 1),
				Entry: domain.EntryInput{Minutes: 480}},
		},
		DiffHash: "abc1234567890def",
	}
	var buf bytes.Buffer
	if err := renderPreview(&buf, diff, 1, 0, 0, 0, false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Summary: 1 creates") {
		t.Errorf("summary missing: %q", out)
	}
	if !strings.Contains(out, "Diff hash: abc1234567890def...") {
		t.Errorf("hash missing: %q", out)
	}
}
