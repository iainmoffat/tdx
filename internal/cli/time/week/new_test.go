package week

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
)

func TestNewResultJSON_Schema(t *testing.T) {
	var buf bytes.Buffer
	err := writeNewResultJSON(&buf, domain.WeekDraft{Name: "default"})
	if err != nil {
		t.Fatal(err)
	}
	var resp map[string]any
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["schema"] != "tdx.v1.weekDraftCreateResult" {
		t.Errorf("schema = %v", resp["schema"])
	}
}

func TestParseShift(t *testing.T) {
	cases := []struct {
		in   string
		want int // days
		bad  bool
	}{
		{"7d", 7, false},
		{"-7d", -7, false},
		{"14d", 14, false},
		{"7", 0, true},
		{"7days", 0, true},
		{"abc", 0, true},
	}
	for _, c := range cases {
		d, err := parseShift(c.in)
		if (err != nil) != c.bad {
			t.Errorf("parseShift(%q) err=%v bad=%v", c.in, err, c.bad)
			continue
		}
		if c.bad {
			continue
		}
		gotDays := int(d.Hours() / 24)
		if gotDays != c.want {
			t.Errorf("parseShift(%q) days=%d, want %d", c.in, gotDays, c.want)
		}
	}
}
