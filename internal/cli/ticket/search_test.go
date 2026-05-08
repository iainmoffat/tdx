package ticket

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

func TestBuildSearchFilterDefaultsToMe(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{}
	filter, err := buildSearchFilter(context.Background(), stub, people, "default", "uid-of-me", 31, nil, nil, nil, "", "", 50, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(filter.AssigneeUIDs) != 1 || filter.AssigneeUIDs[0] != "uid-of-me" {
		t.Errorf("default assignee not me: %+v", filter.AssigneeUIDs)
	}
}

func TestBuildSearchFilterExplicitAssigneeOverridesDefault(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{}
	uid := "12345678-1234-1234-1234-123456789012"
	filter, err := buildSearchFilter(context.Background(), stub, people, "default", "uid-of-me", 31, nil, []string{uid}, nil, "", "", 50, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(filter.AssigneeUIDs) != 1 || filter.AssigneeUIDs[0] != uid {
		t.Errorf("explicit override not respected: %+v", filter.AssigneeUIDs)
	}
}

func TestBuildSearchFilterMeKeyword(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{}
	filter, err := buildSearchFilter(context.Background(), stub, people, "default", "uid-of-me", 31, nil, []string{"me"}, nil, "", "", 50, false)
	if err != nil {
		t.Fatal(err)
	}
	if filter.AssigneeUIDs[0] != "uid-of-me" {
		t.Errorf("'me' should resolve: %+v", filter.AssigneeUIDs)
	}
}

func TestBuildSearchFilterStatusByName(t *testing.T) {
	stub := &stubTicketsvc{resolvedStatus: domain.TicketStatus{ID: 7, Name: "In Progress"}}
	people := &stubPeoplesvc{}
	filter, err := buildSearchFilter(context.Background(), stub, people, "default", "me", 31, []string{"In Progress"}, nil, nil, "", "", 50, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(filter.StatusIDs) != 1 || filter.StatusIDs[0] != 7 {
		t.Errorf("status name not resolved: %+v", filter.StatusIDs)
	}
}

func TestBuildSearchFilterStatusByID(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{}
	filter, err := buildSearchFilter(context.Background(), stub, people, "default", "me", 31, []string{"5"}, nil, nil, "", "", 50, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(filter.StatusIDs) != 1 || filter.StatusIDs[0] != 5 {
		t.Errorf("numeric status not preserved: %+v", filter.StatusIDs)
	}
}

func TestBuildSearchFilterClampsLimit(t *testing.T) {
	stub := &stubTicketsvc{}
	people := &stubPeoplesvc{}
	for _, c := range []struct{ in, want int }{{0, 50}, {-1, 50}, {500, 500}, {2000, 1000}} {
		f, _ := buildSearchFilter(context.Background(), stub, people, "default", "me", 31, nil, []string{"me"}, nil, "", "", c.in, false)
		if f.Limit != c.want {
			t.Errorf("limit %d → %d, want %d", c.in, f.Limit, c.want)
		}
	}
}

func TestRunTicketSearchEmpty(t *testing.T) {
	stub := &stubTicketsvc{tickets: nil}
	var buf bytes.Buffer
	if err := runTicketSearch(context.Background(), &buf, stub, "default", domain.TicketSearchFilter{AppID: 31}, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "no tickets matched") {
		t.Errorf("empty msg: %s", buf.String())
	}
}

func TestRunTicketSearchTable(t *testing.T) {
	stub := &stubTicketsvc{tickets: []domain.Ticket{
		{ID: 100, Title: "Help me", StatusName: "Open", TypeName: "Incident",
			ResponsibleName: "Alice", RequestorName: "Bob",
			ModifiedDate: time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)},
	}}
	var buf bytes.Buffer
	if err := runTicketSearch(context.Background(), &buf, stub, "default", domain.TicketSearchFilter{AppID: 31}, false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"100", "Help me", "Open", "Alice", "Bob", "2026-05-08", "partial"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q\n%s", want, out)
		}
	}
}

func TestRunTicketSearchJSON(t *testing.T) {
	stub := &stubTicketsvc{tickets: []domain.Ticket{{ID: 100, Title: "T"}}}
	var buf bytes.Buffer
	if err := runTicketSearch(context.Background(), &buf, stub, "default", domain.TicketSearchFilter{AppID: 31}, true); err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["schema"] != "tdx.v1.ticketList" {
		t.Fatalf("schema: %v", got["schema"])
	}
}

func TestRunSavedSearchListEmpty(t *testing.T) {
	stub := &stubTicketsvc{savedSearches: nil}
	var buf bytes.Buffer
	if err := runSavedSearchList(context.Background(), &buf, stub, "default", 31, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "no saved searches found") {
		t.Errorf("empty: %s", buf.String())
	}
}

func TestRunSavedSearchListTable(t *testing.T) {
	stub := &stubTicketsvc{savedSearches: []domain.TicketSavedSearch{
		{ID: 7, Name: "My Open", OwnerName: "Alice"},
	}}
	var buf bytes.Buffer
	if err := runSavedSearchList(context.Background(), &buf, stub, "default", 31, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"7", "My Open", "Alice"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("missing %q\n%s", want, buf.String())
		}
	}
}

func TestRunSavedSearchRunByName(t *testing.T) {
	stub := &stubTicketsvc{
		resolvedSaved: domain.TicketSavedSearch{ID: 7, Name: "My Open"},
		savedResults:  []domain.Ticket{{ID: 1, Title: "X", StatusName: "Open"}},
	}
	var buf bytes.Buffer
	if err := runSavedSearchRun(context.Background(), &buf, stub, "default", 31, "My Open", 50, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "X") {
		t.Errorf("output: %s", buf.String())
	}
}
