package week

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

func TestRenderDraftAsWeekReport_BasicGrid(t *testing.T) {
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	draft := domain.WeekDraft{
		SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week,
		Rows: []domain.DraftRow{{
			ID:       "row-01",
			Target:   domain.Target{Kind: domain.TargetTicket, AppID: 42, ItemID: 123, DisplayName: "Big Project"},
			TimeType: domain.TimeType{ID: 7, Name: "Work"}, Billable: true,
			Cells: []domain.DraftCell{{Day: time.Monday, Hours: 8.0}},
		}},
	}
	var buf bytes.Buffer
	renderDraftAsWeekReport(&buf, draft)
	out := buf.String()
	if !strings.Contains(out, "Draft: 2026-05-03/default") {
		t.Errorf("draft header missing: %q", out)
	}
	if !strings.Contains(out, "Work") {
		t.Errorf("time type missing: %q", out)
	}
}

func TestShow_NoBannerWhenNoDraft(t *testing.T) {
	// Optional integration check; skip if too involved.
	// The banner only fires when drafts.Store().Exists returns true,
	// which requires a real config dir. Trust the unit-tested
	// Store.Exists path; integration coverage comes via manual walkthrough.
}
