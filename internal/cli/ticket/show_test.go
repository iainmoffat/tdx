package ticket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

// stubDraftsvc satisfies draftsvcAPI; tests configure draft + err.
type stubDraftsvc struct {
	draft domain.WeekDraft
	err   error
}

func (s *stubDraftsvc) LoadDraft(_ string, _ time.Time, _ string) (domain.WeekDraft, error) {
	return s.draft, s.err
}

func TestRunTicketShowFull(t *testing.T) {
	stubT := &stubTicketsvc{ticket: domain.Ticket{
		ID:               12345,
		Title:            "Test ticket",
		StatusName:       "In Progress",
		TypeName:         "Incident",
		ResponsibleName:  "Alice",
		RequestorName:    "Bob",
		EstimatedMinutes: 240,
		ActualMinutes:    90,
		Description:      "Multi-line\ndescription",
		IsFull:           true,
	}}
	stubD := &stubDraftsvc{err: errors.New("no draft")}
	var buf bytes.Buffer
	if err := runTicketShow(context.Background(), &buf, stubT, stubD, "default", 31, 12345, false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"#12345", "Test ticket", "In Progress", "Alice", "Bob",
		"EST: 4h", "ACT: 1.5h", "this week: 0h", "Multi-line"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q\n%s", want, out)
		}
	}
}

func TestRunTicketShowWithThisWeek(t *testing.T) {
	stubT := &stubTicketsvc{ticket: domain.Ticket{ID: 12345, Title: "T", IsFull: true}}
	stubD := &stubDraftsvc{
		draft: domain.WeekDraft{
			Rows: []domain.DraftRow{
				{Target: domain.Target{Kind: domain.TargetTicket, ItemID: 12345}, Cells: []domain.DraftCell{
					{Hours: 0.5},
					{Hours: 1.0},
				}},
				{Target: domain.Target{Kind: domain.TargetTicket, ItemID: 99999}, Cells: []domain.DraftCell{
					{Hours: 4},
				}},
			},
		},
	}
	var buf bytes.Buffer
	if err := runTicketShow(context.Background(), &buf, stubT, stubD, "default", 31, 12345, false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "this week: 1.5h") {
		t.Errorf("expected this week: 1.5h, got: %s", out)
	}
	if !strings.Contains(out, "(2 entries)") {
		t.Errorf("expected (2 entries), got: %s", out)
	}
}

func TestRunTicketShowWithThisWeekSingular(t *testing.T) {
	stubT := &stubTicketsvc{ticket: domain.Ticket{ID: 1, Title: "T", IsFull: true}}
	stubD := &stubDraftsvc{
		draft: domain.WeekDraft{Rows: []domain.DraftRow{
			{Target: domain.Target{Kind: domain.TargetTicket, ItemID: 1}, Cells: []domain.DraftCell{{Hours: 2}}},
		}},
	}
	var buf bytes.Buffer
	_ = runTicketShow(context.Background(), &buf, stubT, stubD, "default", 31, 1, false)
	if !strings.Contains(buf.String(), "(1 entry)") {
		t.Errorf("expected singular '1 entry', got: %s", buf.String())
	}
}

func TestRunTicketShowNoDraftIsZero(t *testing.T) {
	stubT := &stubTicketsvc{ticket: domain.Ticket{ID: 1, Title: "T", IsFull: true}}
	stubD := &stubDraftsvc{err: errors.New("not found")}
	var buf bytes.Buffer
	if err := runTicketShow(context.Background(), &buf, stubT, stubD, "default", 31, 1, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "this week: 0h") {
		t.Errorf("expected this week: 0h: %s", buf.String())
	}
	// No "(N entries)" parenthetical when count is 0.
	if strings.Contains(buf.String(), "(0 entries)") {
		t.Errorf("should not show (0 entries): %s", buf.String())
	}
}

func TestRunTicketShowJSON(t *testing.T) {
	stubT := &stubTicketsvc{ticket: domain.Ticket{ID: 1, Title: "T", IsFull: true}}
	stubD := &stubDraftsvc{
		draft: domain.WeekDraft{Rows: []domain.DraftRow{
			{Target: domain.Target{Kind: domain.TargetTicket, ItemID: 1}, Cells: []domain.DraftCell{{Hours: 1.5}}},
		}},
	}
	var buf bytes.Buffer
	if err := runTicketShow(context.Background(), &buf, stubT, stubD, "default", 31, 1, true); err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["schema"] != "tdx.v1.ticket" {
		t.Fatalf("schema: %v", got["schema"])
	}
	tk, _ := got["ticket"].(map[string]interface{})
	tw, _ := tk["thisWeek"].(map[string]interface{})
	if tw["hours"] != 1.5 {
		t.Errorf("thisWeek.hours: %v", tw["hours"])
	}
	entries, _ := tw["entries"].(float64)
	if entries != 1 {
		t.Errorf("thisWeek.entries: %v", tw["entries"])
	}
}

func TestThisWeekForTicketIgnoresOtherTargets(t *testing.T) {
	stubD := &stubDraftsvc{
		draft: domain.WeekDraft{Rows: []domain.DraftRow{
			{Target: domain.Target{Kind: domain.TargetProject, ItemID: 1}, Cells: []domain.DraftCell{{Hours: 5}}},
			{Target: domain.Target{Kind: domain.TargetTicket, ItemID: 2}, Cells: []domain.DraftCell{{Hours: 3}}},
		}},
	}
	h, n := thisWeekForTicket(stubD, "default", 1) // looking for ticket 1; project 1 should NOT match
	if h != 0 || n != 0 {
		t.Errorf("project rows must not count: got (%v, %v), want (0, 0)", h, n)
	}
}
