package week

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

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
