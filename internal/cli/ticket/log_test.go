package ticket

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
)

// stubTimesvc satisfies timesvcAPI.
type stubTimesvc struct {
	types      []domain.TimeType
	typesErr   error
	addedEntry domain.TimeEntry
	addErr     error
	lastInput  domain.EntryInput
}

func (s *stubTimesvc) TimeTypesForTarget(_ context.Context, _ string, _ domain.Target) ([]domain.TimeType, error) {
	return s.types, s.typesErr
}
func (s *stubTimesvc) AddEntry(_ context.Context, _ string, in domain.EntryInput) (domain.TimeEntry, error) {
	s.lastInput = in
	return s.addedEntry, s.addErr
}

func TestRunTicketLogHours(t *testing.T) {
	stub := &stubTimesvc{
		types:      []domain.TimeType{{ID: 7, Name: "Development", Billable: true}},
		addedEntry: domain.TimeEntry{ID: 9001},
	}
	var buf bytes.Buffer
	err := runTicketLog(context.Background(), &buf, stub, logRunArgs{
		profile: "default", authedUID: "uid-me", appID: 31, ticketID: 12345,
		hours: 1.5, typeName: "Development",
	})
	if err != nil {
		t.Fatal(err)
	}
	if stub.lastInput.Minutes != 90 {
		t.Errorf("Minutes: got %d, want 90", stub.lastInput.Minutes)
	}
	if stub.lastInput.TimeTypeID != 7 {
		t.Errorf("TypeID: got %d, want 7", stub.lastInput.TimeTypeID)
	}
	if stub.lastInput.Target.Kind != domain.TargetTicket {
		t.Errorf("Target.Kind: %s", stub.lastInput.Target.Kind)
	}
	if stub.lastInput.Target.ItemID != 12345 {
		t.Errorf("Target.ItemID: %d", stub.lastInput.Target.ItemID)
	}
	if stub.lastInput.Target.AppID != 31 {
		t.Errorf("Target.AppID: %d", stub.lastInput.Target.AppID)
	}
	if !stub.lastInput.Billable {
		t.Errorf("Billable should inherit from type (true)")
	}
	for _, want := range []string{"1h 30m", "#12345", "9001", "Development"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("missing %q\n%s", want, buf.String())
		}
	}
}

func TestRunTicketLogMinutes(t *testing.T) {
	stub := &stubTimesvc{
		types:      []domain.TimeType{{ID: 7, Name: "Development"}},
		addedEntry: domain.TimeEntry{ID: 100},
	}
	var buf bytes.Buffer
	_ = runTicketLog(context.Background(), &buf, stub, logRunArgs{
		profile: "default", authedUID: "u", appID: 31, ticketID: 1,
		minutes: 90, typeName: "Development",
	})
	if stub.lastInput.Minutes != 90 {
		t.Errorf("Minutes: %d", stub.lastInput.Minutes)
	}
}

func TestRunTicketLogTypeID(t *testing.T) {
	stub := &stubTimesvc{
		types: []domain.TimeType{
			{ID: 7, Name: "Development"},
			{ID: 9, Name: "Planning"},
		},
		addedEntry: domain.TimeEntry{ID: 1},
	}
	var buf bytes.Buffer
	if err := runTicketLog(context.Background(), &buf, stub, logRunArgs{
		profile: "default", authedUID: "u", appID: 31, ticketID: 1,
		minutes: 30, typeID: 9,
	}); err != nil {
		t.Fatal(err)
	}
	if stub.lastInput.TimeTypeID != 9 {
		t.Errorf("type-id: %d", stub.lastInput.TimeTypeID)
	}
}

func TestRunTicketLogTypeNotFound(t *testing.T) {
	stub := &stubTimesvc{types: []domain.TimeType{{ID: 7, Name: "Development"}}}
	err := runTicketLog(context.Background(), &bytes.Buffer{}, stub, logRunArgs{
		profile: "default", authedUID: "u", appID: 31, ticketID: 1,
		minutes: 30, typeName: "Coffee Break",
	})
	if err == nil || !strings.Contains(err.Error(), "no time type matches") {
		t.Fatalf("want no-match error, got %v", err)
	}
}

func TestRunTicketLogBillableOverride(t *testing.T) {
	stub := &stubTimesvc{
		types:      []domain.TimeType{{ID: 7, Name: "Development", Billable: true}},
		addedEntry: domain.TimeEntry{ID: 1},
	}
	_ = runTicketLog(context.Background(), &bytes.Buffer{}, stub, logRunArgs{
		profile: "default", authedUID: "u", appID: 31, ticketID: 1,
		minutes: 30, typeName: "Development", billableSet: true, billable: false,
	})
	if stub.lastInput.Billable {
		t.Error("--billable=false should override type default")
	}
}

func TestRunTicketLogPropagatesError(t *testing.T) {
	stub := &stubTimesvc{
		types:  []domain.TimeType{{ID: 7, Name: "Development"}},
		addErr: errors.New("td rejected"),
	}
	err := runTicketLog(context.Background(), &bytes.Buffer{}, stub, logRunArgs{
		profile: "default", authedUID: "u", appID: 31, ticketID: 1,
		minutes: 30, typeName: "Development",
	})
	if err == nil || !strings.Contains(err.Error(), "td rejected") {
		t.Fatalf("want propagated error, got %v", err)
	}
}

func TestNewLogCmdRequiresYes(t *testing.T) {
	cmd := newLogCmd(&stubTicketsvc{})
	cmd.SetArgs([]string{"1", "--minutes", "30", "--type", "Development"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("want --yes error, got %v", err)
	}
}

func TestNewLogCmdRejectsBothHoursAndMinutes(t *testing.T) {
	cmd := newLogCmd(&stubTicketsvc{})
	cmd.SetArgs([]string{"1", "--hours", "1", "--minutes", "30", "--type", "x", "--yes"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("want mutually-exclusive error, got %v", err)
	}
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		minutes int
		want    string
	}{
		{0, "0m"}, {30, "30m"}, {60, "1h"}, {90, "1h 30m"}, {125, "2h 5m"},
	}
	for _, c := range cases {
		got := formatDuration(c.minutes)
		if got != c.want {
			t.Errorf("formatDuration(%d) = %q, want %q", c.minutes, got, c.want)
		}
	}
}
