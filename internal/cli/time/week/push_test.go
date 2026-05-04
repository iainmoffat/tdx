package week

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
)

func TestRenderPushResult_JSON(t *testing.T) {
	res := draftsvc.ApplyResult{Created: 2, Updated: 1, Deleted: 1, Skipped: 0}
	var buf bytes.Buffer
	if err := renderPushResult(&buf, res, true); err != nil {
		t.Fatal(err)
	}
	var resp weekDraftPushResp
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Schema != "tdx.v1.weekDraftPushResult" {
		t.Errorf("schema = %q", resp.Schema)
	}
	if resp.Created != 2 || resp.Updated != 1 || resp.Deleted != 1 {
		t.Errorf("counts = %+v", resp)
	}
}

func TestRenderPushResult_TextWithFailures(t *testing.T) {
	res := draftsvc.ApplyResult{
		Created: 1,
		Failed: []draftsvc.ApplyFailure{
			{Kind: "delete", RowID: "row-01", Date: "2026-05-04", EntryID: 98731, Message: "permission denied"},
		},
	}
	var buf bytes.Buffer
	if err := renderPushResult(&buf, res, false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"1 created", "Failures (1)", "98731", "permission denied"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in: %q", want, out)
		}
	}
}

func TestPushPreviewTrailer_NoDeletes(t *testing.T) {
	var buf bytes.Buffer
	weekStart := mustParseDate(t, "2026-04-26")
	writePushPreviewTrailer(&buf, weekStart, 5, 2, 0, 0, 0)
	out := buf.String()
	if !strings.Contains(out, "Preview only — no changes applied.") {
		t.Errorf("missing trailer banner: %q", out)
	}
	if !strings.Contains(out, "tdx time week push 2026-04-26 --yes") {
		t.Errorf("missing apply hint: %q", out)
	}
	if strings.Contains(out, "--allow-deletes") {
		t.Errorf("should not mention --allow-deletes when deletes=0: %q", out)
	}
}

func TestPushPreviewTrailer_WithDeletes(t *testing.T) {
	var buf bytes.Buffer
	weekStart := mustParseDate(t, "2026-04-26")
	writePushPreviewTrailer(&buf, weekStart, 1, 0, 3, 0, 0)
	out := buf.String()
	if !strings.Contains(out, "--allow-deletes") {
		t.Errorf("expected --allow-deletes hint when deletes>0: %q", out)
	}
	if !strings.Contains(out, "3 delete(s)") {
		t.Errorf("expected delete count: %q", out)
	}
}

func TestPushPreviewTrailer_NothingToPush(t *testing.T) {
	var buf bytes.Buffer
	weekStart := mustParseDate(t, "2026-04-26")
	writePushPreviewTrailer(&buf, weekStart, 0, 0, 0, 5, 0)
	out := buf.String()
	if !strings.Contains(out, "nothing to push") {
		t.Errorf("expected 'nothing to push' message: %q", out)
	}
	if strings.Contains(out, "--yes") {
		t.Errorf("should not nudge --yes when nothing to apply: %q", out)
	}
}

func TestPushPreviewTrailer_BlockersOnly(t *testing.T) {
	var buf bytes.Buffer
	weekStart := mustParseDate(t, "2026-04-26")
	writePushPreviewTrailer(&buf, weekStart, 0, 0, 0, 0, 2)
	out := buf.String()
	if !strings.Contains(out, "blocker") {
		t.Errorf("expected blocker mention: %q", out)
	}
	if strings.Contains(out, "--yes") {
		t.Errorf("blocker-only state should not suggest --yes: %q", out)
	}
}

func mustParseDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return d
}
