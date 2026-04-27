package week

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

func TestRecommendedAction(t *testing.T) {
	cases := []struct {
		sync         domain.SyncState
		stale        bool
		wantContains string
	}{
		{domain.SyncConflicted, false, "edit to resolve"},
		{domain.SyncDirty, true, "remote drifted"},
		{domain.SyncDirty, false, "preview"},
		{domain.SyncClean, true, "adopt remote"},
		{domain.SyncClean, false, "no action"},
	}
	for _, c := range cases {
		got := recommendedAction(c.sync, c.stale)
		if !strings.Contains(got, c.wantContains) {
			t.Errorf("sync=%s stale=%v: got %q, want contains %q", c.sync, c.stale, got, c.wantContains)
		}
	}
}

func TestRenderStatusText_Basic(t *testing.T) {
	d := domain.WeekDraft{
		SchemaVersion: 1, Profile: "work", Name: "default",
		WeekStart: time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ),
	}
	state := domain.DraftSyncState{Sync: domain.SyncClean}
	var buf bytes.Buffer
	if err := renderStatus(&buf, d, state, "no action recommended", false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"work", "default", "clean", "no action"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in: %q", want, out)
		}
	}
}
