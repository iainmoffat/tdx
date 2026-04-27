package week

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

func TestPullResultJSON(t *testing.T) {
	draft := domain.WeekDraft{
		SchemaVersion: 1, Profile: "work", Name: "default",
		WeekStart: time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ),
		Rows:      []domain.DraftRow{{ID: "row-01"}},
	}

	var buf bytes.Buffer
	if err := writePullResultJSON(&buf, draft); err != nil {
		t.Fatal(err)
	}

	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["schema"] != "tdx.v1.weekDraftPullResult" {
		t.Errorf("schema = %v", out["schema"])
	}
	if out["draft"] == nil {
		t.Errorf("draft missing")
	}
}

func TestPullResultText_RendersSummary(t *testing.T) {
	draft := domain.WeekDraft{
		SchemaVersion: 1, Profile: "work", Name: "default",
		WeekStart: time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ),
		Rows: []domain.DraftRow{{
			ID:    "row-01",
			Cells: []domain.DraftCell{{Day: time.Monday, Hours: 8.0}},
		}},
	}

	var buf bytes.Buffer
	writePullResultText(&buf, draft)
	out := buf.String()

	if !strings.Contains(out, "2026-05-03") {
		t.Errorf("date missing: %q", out)
	}
	if !strings.Contains(out, "default") {
		t.Errorf("name missing: %q", out)
	}
}
