package week

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
)

func TestRenderHistory_JSON_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := renderHistory(&buf, nil, true); err != nil {
		t.Fatal(err)
	}
	var resp weekDraftSnapshotListResp
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Schema != "tdx.v1.weekDraftSnapshotList" {
		t.Errorf("schema = %q", resp.Schema)
	}
}

func TestRenderHistory_TextEmpty(t *testing.T) {
	var buf bytes.Buffer
	_ = renderHistory(&buf, nil, false)
	if !strings.Contains(buf.String(), "No snapshots.") {
		t.Errorf("got %q", buf.String())
	}
}

func TestRenderHistory_TextRows(t *testing.T) {
	list := []draftsvc.SnapshotInfo{
		{Sequence: 1, Op: draftsvc.OpPrePull, Taken: time.Date(2026, 4, 27, 9, 0, 0, 0, time.UTC), Note: ""},
		{Sequence: 2, Op: draftsvc.OpManual, Taken: time.Date(2026, 4, 28, 14, 0, 0, 0, time.UTC), Pinned: true, Note: "important"},
	}
	var buf bytes.Buffer
	_ = renderHistory(&buf, list, false)
	out := buf.String()
	for _, want := range []string{"SEQ", "OP", "pre-pull", "manual", "yes", "important"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in: %q", want, out)
		}
	}
}
