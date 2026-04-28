package week

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
)

func TestCopyResultJSON_Schema(t *testing.T) {
	var buf bytes.Buffer
	if err := writeCopyResultJSON(&buf, domain.WeekDraft{Name: "pristine"}); err != nil {
		t.Fatal(err)
	}
	var resp map[string]any
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["schema"] != "tdx.v1.weekDraftCopyResult" {
		t.Errorf("schema = %v", resp["schema"])
	}
}
